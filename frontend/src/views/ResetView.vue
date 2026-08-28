<!-- ResetView.vue：重置密码（/reset#token=...）——新密码 + 确认密码 → 提交 → 成功跳 /login -->
<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Form, Input, Button, Alert, Result } from 'ant-design-vue'
import { resetPassword, validateResetToken, type ResetTokenStatus } from '@/api/auth'
import { useTheme } from '@/theme'

const route = useRoute()
const router = useRouter()
const { dark, toggle } = useTheme()
const form = reactive({ password: '', confirm: '' })
const submitting = ref(false)
const errorMsg = ref('')
const done = ref(false)
const tokenStatus = ref<ResetTokenStatus | 'checking' | 'network_error'>('checking')

// 新格式：token 放 URL fragment，不进入服务器/反代访问日志。读取后立即清除 hash。
const token = new URLSearchParams(route.hash.slice(1)).get('token') ?? ''
if (token) history.replaceState(null, '', route.path)

const invalidState = computed(() => {
  switch (tokenStatus.value) {
    case 'missing': return { title: '重置链接无效', detail: '链接中未找到重置令牌，请重新申请密码重置。' }
    case 'used': return { title: '重置链接已使用', detail: '该链接已完成过密码重置，请使用新密码登录。' }
    case 'expired': return { title: '重置链接已过期', detail: '为保护账号安全，请重新申请密码重置链接。' }
    default: return null
  }
})

async function validateToken() {
  if (!token) {
    tokenStatus.value = 'missing'
    return
  }
  tokenStatus.value = 'checking'
  try {
    tokenStatus.value = (await validateResetToken(token)).status
  } catch (err) {
    errorMsg.value = (err as Error).message
    tokenStatus.value = 'network_error'
  }
}

onMounted(() => { void validateToken() })

const rules = {
  password: [{ required: true, min: 8, message: '密码至少 8 个字符', trigger: 'blur' as const }],
  confirm: [
    { required: true, message: '请再次输入密码', trigger: 'blur' as const },
    {
      validator: (_: unknown, value: string) =>
        value === form.password ? Promise.resolve() : Promise.reject(new Error('两次输入的密码不一致')),
      trigger: 'blur' as const,
    },
  ],
}

async function onSubmit() {
  submitting.value = true
  errorMsg.value = ''
  try {
    await resetPassword({ token, password: form.password })
    done.value = true
  } catch (err) {
    errorMsg.value = (err as Error).message
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="w-full max-w-md">
    <div class="bg-white dark:bg-gray-800 dark:text-gray-100 rounded-lg shadow p-8">
      <Result v-if="done" status="success" title="密码已重置" sub-title="请使用新密码登录">
        <template #extra><Button type="primary" @click="router.push('/login')">前往登录</Button></template>
      </Result>
      <Result v-else-if="tokenStatus === 'checking'" status="info" title="正在验证重置链接" />
      <Result v-else-if="tokenStatus === 'network_error'" status="warning" title="暂时无法验证重置链接" :sub-title="errorMsg || '请检查网络后重试'">
        <template #extra>
          <Button type="primary" @click="validateToken">重试</Button>
          <Button @click="router.push('/login')">返回登录</Button>
        </template>
      </Result>
      <Result v-else-if="tokenStatus !== 'valid'" status="error" :title="invalidState?.title" :sub-title="invalidState?.detail">
        <template #extra>
          <Button type="primary" @click="router.push('/forgot')">重新申请</Button>
          <Button @click="router.push('/login')">返回登录</Button>
        </template>
      </Result>
      <template v-else>
        <h1 class="text-xl font-semibold mb-6">重置密码</h1>
        <Form layout="vertical" :model="form" :rules="rules" @finish="onSubmit">
          <Form.Item label="新密码" name="password">
            <Input.Password v-model:value="form.password" autocomplete="new-password" />
          </Form.Item>
          <Form.Item label="确认密码" name="confirm">
            <Input.Password v-model:value="form.confirm" autocomplete="new-password" />
          </Form.Item>
          <Alert v-if="errorMsg" type="error" :message="errorMsg" class="mb-4" />
          <Button type="primary" html-type="submit" block :loading="submitting">确认重置</Button>
        </Form>
      </template>
    </div>
    <div class="text-right mt-3"><Button size="small" @click="toggle">{{ dark ? '浅色模式' : '暗色模式' }}</Button></div>
  </div>
</template>
