<!-- RegisterView.vue：用户名 + 邮箱 + 密码 + 确认密码（UI §2.2） -->
<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Form, Input, Button, Alert } from 'ant-design-vue'
import { useAuthStore } from '@/stores/auth'
import { useTheme } from '@/theme'

const router = useRouter()
const auth = useAuthStore()
const { dark, toggle } = useTheme()
const form = reactive({ username: '', email: '', password: '', confirm: '' })
const submitting = ref(false)
const errorMsg = ref('')

// 表单规则：密码 ≥8 字符；确认密码与密码一致
const rules = {
  username: [{ required: true, type: 'string' as const, min: 1, max: 64, message: '请输入用户名', trigger: 'blur' as const }],
  email: [{ required: true, type: 'email' as const, message: '请输入有效邮箱', trigger: 'blur' as const }],
  password: [{ required: true, type: 'string' as const, min: 8, message: '密码至少 8 个字符', trigger: 'blur' as const }],
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
    const data = await auth.registerAction({ username: form.username, email: form.email, password: form.password })
    if (data.status === 'active') await router.push('/') // 直接激活：token 已存 → 首页
    else await router.push('/pending') // 待审批
  } catch (err) {
    errorMsg.value = (err as Error).message
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="w-full max-w-md">
    <div class="bg-white dark:bg-gray-800 rounded-lg shadow p-8">
      <h1 class="text-xl font-semibold mb-6">注册</h1>
      <Form layout="vertical" :rules="rules" @finish="onSubmit">
        <Form.Item label="用户名" name="username">
          <Input v-model:value="form.username" :maxlength="64" />
        </Form.Item>
        <Form.Item label="邮箱" name="email">
          <Input v-model:value="form.email" autocomplete="email" />
        </Form.Item>
        <Form.Item label="密码" name="password">
          <Input.Password v-model:value="form.password" autocomplete="new-password" />
        </Form.Item>
        <Form.Item label="确认密码" name="confirm">
          <Input.Password v-model:value="form.confirm" autocomplete="new-password" />
        </Form.Item>
        <Alert v-if="errorMsg" type="error" :message="errorMsg" class="mb-4" />
        <Button type="primary" html-type="submit" block :loading="submitting">注册</Button>
      </Form>
      <div class="text-center mt-4">
        已有账号？<RouterLink to="/login">返回登录</RouterLink>
      </div>
    </div>
    <div class="text-right mt-3"><Button size="small" @click="toggle">{{ dark ? '浅色模式' : '暗色模式' }}</Button></div>
  </div>
</template>
