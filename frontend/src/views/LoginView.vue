<!-- LoginView.vue：本地登录区块 + 注册入口可见性 + 表空提示 + OIDC 区块（UI §2.2） -->
<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Form, Input, Button, Checkbox, Alert, Divider, Switch } from 'ant-design-vue'
import { useAuthStore } from '@/stores/auth'
import { useSystemStore } from '@/stores/system'
import { useTheme } from '@/theme'
import { mockLogin } from '@/api/oidc'
import { getPublicAnnouncement } from '@/api/settings'
import CaptchaWidget from '@/components/CaptchaWidget.vue'
import MarkdownView from '@/components/MarkdownView.vue'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const system = useSystemStore()
const { dark, toggle } = useTheme()
const form = reactive({ email: '', password: '', remember: false, captcha_token: '' })
const submitting = ref(false)
const errorMsg = ref('')

// OIDC 回调错误统一映射为可恢复的中文说明，内部枚举不直接暴露给用户。
const oidcError = ref('')
const oidcErrorMessages: Record<string, string> = {
  state_mismatch: 'OIDC 登录请求校验失败，请重新发起登录。',
  state_expired: 'OIDC 登录请求已过期，请重新发起登录。',
  exchange_failed: 'OIDC 身份验证交换失败，请稍后重试。',
  resolve_failed: '无法解析 OIDC 用户信息，请联系管理员检查身份提供商配置。',
  issue_failed: '无法创建登录会话，请稍后重试或联系管理员。',
}
watch(() => route.query.oidc_error, (v) => {
  const value = typeof v === 'string' ? v : ''
  oidcError.value = value ? (oidcErrorMessages[value] ?? `OIDC 登录失败：${value}`) : ''
}, { immediate: true })

// 模拟登录表单（UI §2.2：role/group 附加属性，勾选后输入，R07-07）
const mockForm = reactive({ email: '', username: '', email_verified: true, with_role: false, role: '', with_group: false, group: '' })
const mockSubmitting = ref(false)

async function onMockLogin() {
  mockSubmitting.value = true
  errorMsg.value = ''
  try {
    const res = await mockLogin({
      email: mockForm.email,
      username: mockForm.username || undefined,
      email_verified: mockForm.email_verified,
      // 附加属性：勾选且输入值才透传，空则 undefined（保持「可留空」语义）
      roles: mockForm.with_role && mockForm.role ? [mockForm.role] : undefined,
      groups: mockForm.with_group && mockForm.group ? [mockForm.group] : undefined,
    })
    if (res.status === 'pending') {
      errorMsg.value = res.message ?? '账号待审批，请等待管理员审核'
      return
    }
    if (!res.token) {
      errorMsg.value = '模拟登录未返回会话令牌'
      return
    }
    auth.setSession(res.token)
    await router.push('/')
  } catch (err) {
    errorMsg.value = (err as Error).message
  } finally {
    mockSubmitting.value = false
  }
}

// 注册入口可见性：依赖本地登录开启，且 allow_selfreg 开启或用户表为空（Design1 §3.2/5.2）
const showRegister = computed(() => {
  const st = system.status
  return !!st && st.allow_local_login !== false && (st.allow_selfreg || st.user_table_empty)
})
// 本地登录区块可见性：仅当 allow_local_login 开启时显示（Design1 §3.2）
const showLocalLogin = computed(() => system.status?.allow_local_login !== false)
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
// 登录页公告/页脚（R10-07：登录页公告 + 登录页页脚独立配置，公开端点获取，MD 渲染；有内容才显示）
const notice = reactive({ login_announcement: '', login_footer: '' })

onMounted(async () => {
  // 公开请求失败不阻断页面渲染（R07-08 防 unhandled rejection 噪音）
  await system.fetchStatus(true).catch(() => {})
  void system.fetchSiteInfo(true).catch(() => {})
  try {
    const n = await getPublicAnnouncement()
    notice.login_announcement = n.login_announcement ?? ''
    notice.login_footer = n.login_footer ?? ''
  } catch {
    // 公告/页脚获取失败不阻断登录页
  }
  if (auth.token) router.replace('/') // 已登录访问自动跳 /
})
</script>

<template>
  <div class="w-full max-w-md">
    <!-- 自定义登录页公告：登录 card 上方，MD 渲染；容器与登录 card 同款样式（R10-07 边框阴影统一） -->
    <div v-if="notice.login_announcement"
         class="bg-surface rounded-lg shadow p-4 mb-4">
      <MarkdownView :source="notice.login_announcement" />
    </div>
    <div class="bg-surface rounded-lg shadow p-8">
      <!-- 顶部：Logo 垂直布局（ICON 上、站点标题下，标题更大；R10-06） -->
      <div class="flex flex-col items-center gap-3 mb-6">
        <img v-if="system.siteIconUrl" :src="system.siteIconUrl" alt="站点 ICON" class="h-16 w-16 object-contain" />
        <span class="text-2xl font-semibold">{{ system.siteName }}</span>
      </div>
      <h1 class="text-xl font-semibold mb-6">登录</h1>
      <!-- 本地登录区块：仅当 allow_local_login 开启时显示（Design1 §3.2） -->
      <template v-if="showLocalLogin">
        <!-- 表空提示：系统尚未配置管理员，首个注册用户将成为管理员 -->
        <Alert v-if="tableEmpty" type="info" show-icon class="mb-4"
               message="系统尚未配置管理员，首个注册用户将成为管理员" />
        <Form layout="vertical" :model="form" @finish="onSubmit">
          <Form.Item label="邮箱" name="email" :rules="[{ required: true, type: 'email', trigger: 'blur' }]">
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
          <CaptchaWidget page="login" @update:token="(t: string) => (form.captcha_token = t)" />
          <Button type="primary" html-type="submit" block :loading="submitting">登录</Button>
        </Form>
      </template>
      <!-- OIDC 区块：oidc_configured 时渲染（Step 6 填充） -->
      <template v-if="system.status?.oidc_configured">
        <Divider plain>或</Divider>
        <!-- mock 提供商（仅 Dev）：模拟登录表单，标题标注「Dev 模拟登录」 -->
        <Form v-if="system.status.oidc_provider_type === 'mock'" layout="vertical" :model="mockForm" @finish="onMockLogin">
          <div class="text-sm text-text-tertiary mb-2">Dev 模拟登录</div>
          <Form.Item label="邮箱" name="email" :rules="[{ required: true, type: 'email', trigger: 'blur' }]">
            <Input v-model:value="mockForm.email" />
          </Form.Item>
          <Form.Item label="用户名（可留空，默认取邮箱前缀）"><Input v-model:value="mockForm.username" /></Form.Item>
          <Checkbox v-model:checked="mockForm.email_verified" class="mb-3">email_verified（默认勾选）</Checkbox>
          <!-- 附加属性（UI §2.2，R07-07）：勾选后显示输入，供测试 Role/Group 白名单与审批逻辑 -->
          <Checkbox v-model:checked="mockForm.with_role" class="mb-1">附加 role</Checkbox>
          <Form.Item v-if="mockForm.with_role" label="role 值">
            <Input v-model:value="mockForm.role" placeholder="如 user / admin" />
          </Form.Item>
          <Checkbox v-model:checked="mockForm.with_group" class="mb-1">附加 group</Checkbox>
          <Form.Item v-if="mockForm.with_group" label="group 值">
            <Input v-model:value="mockForm.group" placeholder="如 default" />
          </Form.Item>
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
    <!-- 自定义登录页页脚：登录 card 下方，MD 渲染；容器与登录 card 同款样式（R10-07 边框阴影统一） -->
    <div v-if="notice.login_footer"
         class="bg-surface rounded-lg shadow p-4 mt-4">
      <MarkdownView :source="notice.login_footer" />
    </div>
    <div class="text-right mt-3"><Switch :checked="dark" checked-children="🌙" un-checked-children="☀️" size="small" title="切换暗色/浅色模式" @change="toggle" /></div>
  </div>
</template>
