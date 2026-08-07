<!-- LoginView.vue：本地登录区块 + 注册入口可见性 + 表空提示 + OIDC 区块（UI §2.2） -->
<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Form, Input, Button, Checkbox, Alert, Divider } from 'ant-design-vue'
import { useAuthStore } from '@/stores/auth'
import { useSystemStore } from '@/stores/system'
import { useTheme } from '@/theme'
import { mockLogin } from '@/api/oidc'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const system = useSystemStore()
const { dark, toggle } = useTheme()
const form = reactive({ email: '', password: '', remember: false })
const submitting = ref(false)
const errorMsg = ref('')

// OIDC 区块：oidc_error 冲突文案从 route.query 读取展示
const oidcError = ref('')
watch(() => route.query.oidc_error, (v) => { if (v) oidcError.value = String(v) }, { immediate: true })

// 模拟登录表单
const mockForm = reactive({ email: '', username: '', email_verified: true })
const mockSubmitting = ref(false)

async function onMockLogin() {
  mockSubmitting.value = true
  errorMsg.value = ''
  try {
    await mockLogin({ email: mockForm.email, username: mockForm.username || undefined, email_verified: mockForm.email_verified })
    await router.push('/')
  } catch (err) {
    errorMsg.value = (err as Error).message
  } finally {
    mockSubmitting.value = false
  }
}

// 注册入口可见性：allow_selfreg 开启，或 user_table_empty 且 allow_local_login（始终显示）
const showRegister = computed(() => {
  const st = system.status
  return !!st && (st.allow_selfreg || (st.user_table_empty && st.allow_local_login))
})
const tableEmpty = computed(() => system.status?.user_table_empty ?? false)

// 真实提供商登录：跳转后端发起授权
function startOidcLogin() {
  window.location.href = '/api/auth/oidc/login'
}

async function onSubmit() {
  submitting.value = true
  errorMsg.value = ''
  try {
    await auth.loginAction(form)
    await router.push('/')
  } catch (err) {
    errorMsg.value = (err as Error).message // 后端统一措辞直接展示
  } finally {
    submitting.value = false
  }
}
onMounted(async () => {
  await system.fetchStatus(true)
  if (auth.token) router.replace('/') // 已登录访问自动跳 /
})
</script>

<template>
  <div class="w-full max-w-md">
    <div class="bg-white dark:bg-gray-800 rounded-lg shadow p-8">
      <h1 class="text-xl font-semibold mb-6">登录</h1>
      <!-- 表空提示：系统尚未配置管理员，首个注册用户将成为管理员 -->
      <Alert v-if="tableEmpty" type="info" show-icon class="mb-4"
             message="系统尚未配置管理员，首个注册用户将成为管理员" />
      <Form layout="vertical" @finish="onSubmit">
        <Form.Item label="邮箱" name="email" :rules="[{ required: true, type: 'email' }]">
          <Input v-model:value="form.email" autocomplete="email" />
        </Form.Item>
        <Form.Item label="密码" name="password" :rules="[{ required: true }]">
          <Input.Password v-model:value="form.password" autocomplete="current-password" />
        </Form.Item>
        <div class="flex items-center justify-between mb-4">
          <Checkbox v-model:checked="form.remember">记住我</Checkbox>
          <RouterLink to="/forgot" class="text-sm">忘记密码？</RouterLink>
        </div>
        <Alert v-if="errorMsg" type="error" :message="errorMsg" class="mb-4" />
        <Button type="primary" html-type="submit" block :loading="submitting">登录</Button>
      </Form>
      <!-- OIDC 区块：oidc_configured 时渲染（Step 6 填充） -->
      <template v-if="system.status?.oidc_configured">
        <Divider plain>或</Divider>
        <!-- mock 提供商（仅 Dev）：模拟登录表单，标题标注「Dev 模拟登录」 -->
        <Form v-if="system.status.oidc_provider_type === 'mock'" layout="vertical" @finish="onMockLogin">
          <div class="text-sm text-gray-400 mb-2">Dev 模拟登录</div>
          <Form.Item label="邮箱" name="mock_email" :rules="[{ required: true, type: 'email' }]">
            <Input v-model:value="mockForm.email" />
          </Form.Item>
          <Form.Item label="用户名（可留空，默认取邮箱前缀）"><Input v-model:value="mockForm.username" /></Form.Item>
          <Checkbox v-model:checked="mockForm.email_verified" class="mb-3">email_verified（默认勾选）</Checkbox>
          <Button block :loading="mockSubmitting" html-type="submit">模拟登录</Button>
        </Form>
        <!-- 真实提供商：主按钮直接跳后端发起授权 -->
        <Button v-else block type="primary" ghost @click="startOidcLogin">
          使用 OIDC 登录
        </Button>
      </template>
      <!-- 回调错误（oidc_error/冲突文案）展示 -->
      <Alert v-if="oidcError" type="error" :message="oidcError" class="mt-4" />
      <Divider v-if="showRegister" plain />
      <div v-if="showRegister" class="text-center">
        还没有账号？<RouterLink to="/register">立即注册</RouterLink>
      </div>
    </div>
    <div class="text-right mt-3"><Button size="small" @click="toggle">{{ dark ? '浅色模式' : '暗色模式' }}</Button></div>
  </div>
</template>
