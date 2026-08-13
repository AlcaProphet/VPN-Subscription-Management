<!-- ForgotView.vue：忘记密码——邮箱输入 → 提交 → a-result 统一提示（防枚举） -->
<script setup lang="ts">
import { reactive, ref } from 'vue'
import { Form, Input, Button, Result } from 'ant-design-vue'
import { forgot } from '@/api/auth'
import { Notify } from '@/components/Notify'
import CaptchaWidget from '@/components/CaptchaWidget.vue'
import { useTheme } from '@/theme'

const { dark, toggle } = useTheme()
const form = reactive({ email: '', captcha_token: '' })
const submitted = ref(false)
const submitting = ref(false)

async function onSubmit() {
  submitting.value = true
  try {
    await forgot({ email: form.email, captcha_token: form.captcha_token })
    submitted.value = true // 无论邮箱是否存在，展示同一提示（防枚举）
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="w-full max-w-md">
    <div class="bg-white dark:bg-gray-800 dark:text-gray-100 rounded-lg shadow p-8">
      <Result v-if="submitted" status="success" title="若该邮箱已注册，重置链接已发送"
              sub-title="请查收邮件（1 小时内有效）">
        <template #extra><Button type="primary" @click="$router.push('/login')">返回登录</Button></template>
      </Result>
      <template v-else>
        <h1 class="text-xl font-semibold mb-6">找回密码</h1>
        <Form layout="vertical" :model="form" @finish="onSubmit">
          <Form.Item label="邮箱" name="email" :rules="[{ required: true, type: 'email', trigger: 'blur' }]">
            <Input v-model:value="form.email" autocomplete="email" />
          </Form.Item>
          <CaptchaWidget page="forgot" @update:token="(t: string) => (form.captcha_token = t)" />
          <Button type="primary" html-type="submit" block :loading="submitting">发送重置链接</Button>
        </Form>
        <div class="text-center mt-4">
          <RouterLink to="/login" class="text-sm">返回登录</RouterLink>
        </div>
      </template>
    </div>
    <div class="text-right mt-3"><Button size="small" @click="toggle">{{ dark ? '浅色模式' : '暗色模式' }}</Button></div>
  </div>
</template>
