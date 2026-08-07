<!-- ProfileView.vue：个人中心（UI §4.3）——a-tabs 三页签：基本信息 / 密码管理 / OIDC 绑定 -->
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Alert, Button, Card, Descriptions, Form, Input, Tabs, Tag } from 'ant-design-vue'
import { updateUsername, updateEmail, updatePassword } from '@/api/profile'
import { me } from '@/api/auth'
import { http } from '@/api/request'
import { useAuthStore } from '@/stores/auth'
import { useSystemStore } from '@/stores/system'
import { Notify } from '@/components/Notify'

const router = useRouter()
const auth = useAuthStore()
const system = useSystemStore()

const loading = ref(true)
const oidcConfigured = computed(() => system.status?.oidc_configured === true)

onMounted(async () => {
  try {
    auth.user = await me()
  } catch {
    // 401 已由拦截器处理
  } finally {
    loading.value = false
  }
})

// --- 基本信息：修改用户名/邮箱行内编辑 ---
const editUsername = ref(false)
const usernameVal = ref('')
const editEmail = ref(false)
const emailVal = ref('')
const saving = ref(false)

async function saveUsername() {
  if (!usernameVal.value.trim()) return
  saving.value = true
  try {
    await updateUsername(usernameVal.value.trim())
    Notify.success('用户名已更新')
    editUsername.value = false
    auth.user = await me()
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    saving.value = false
  }
}

async function saveEmail() {
  if (!emailVal.value) return
  saving.value = true
  try {
    await updateEmail(emailVal.value)
    // 邮箱修改成功 → 所有设备会话已失效 → 清凭据跳登录
    Notify.success('邮箱已修改，所有设备会话已失效，请重新登录')
    await auth.logoutAction()
    router.push('/login')
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    saving.value = false
  }
}

// --- 密码管理 ---
const pwdForm = ref({ current_password: '', new_password: '' })
const savingPwd = ref(false)

async function savePassword() {
  if (pwdForm.value.new_password.length < 8) {
    Notify.error('密码长度至少 8 个字符')
    return
  }
  savingPwd.value = true
  try {
    const res = await updatePassword(pwdForm.value)
    Notify.success(res.message ?? '密码已更新，请重新登录')
    await auth.logoutAction()
    router.push('/login')
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    savingPwd.value = false
  }
}

// --- OIDC 绑定 ---
const subject = computed(() => auth.user?.email ?? '')
async function startBind() {
  // 未绑定 → 调绑定端点拿授权 URL（Bearer 会话经拦截器携带）→ 跳转授权
  try {
    const res = await http.post<any, { auth_url: string }>('/auth/oidc/bind')
    window.location.href = res.auth_url
  } catch (err) {
    Notify.error((err as Error).message)
  }
}
</script>

<template>
  <div class="min-h-screen bg-gray-50 dark:bg-gray-900">
    <main class="max-w-2xl mx-auto p-4">
      <h2 class="text-lg font-semibold mb-4">个人中心</h2>
      <Card :loading="loading" class="shadow-sm">
        <Tabs>
          <!-- 基本信息 -->
          <Tabs.TabPane key="basic" tab="基本信息">
            <Descriptions :column="1" size="middle">
              <Descriptions.Item label="用户名">
                <template v-if="!editUsername">
                  {{ auth.user?.username }}
                  <Button size="small" type="link" @click="editUsername = true; usernameVal = auth.user?.username ?? ''">修改</Button>
                </template>
                <template v-else>
                  <Input v-model:value="usernameVal" :maxlength="64" class="w-56" @press-enter="saveUsername" />
                  <Button size="small" type="primary" class="ml-2" :loading="saving" @click="saveUsername">保存</Button>
                </template>
              </Descriptions.Item>
              <Descriptions.Item label="邮箱">
                <template v-if="!editEmail">
                  {{ auth.user?.email || '—' }}
                  <Button size="small" type="link" @click="editEmail = true; emailVal = auth.user?.email ?? ''">修改</Button>
                </template>
                <template v-else>
                  <Input v-model:value="emailVal" class="w-56" @press-enter="saveEmail" />
                  <Button size="small" type="primary" class="ml-2" :loading="saving" @click="saveEmail">保存</Button>
                  <Alert type="warning" class="mt-2" message="修改邮箱后所有设备会话将立即失效，需重新登录" />
                </template>
              </Descriptions.Item>
              <Descriptions.Item label="角色">
                <Tag :color="auth.user?.role === 'admin' ? 'blue' : 'default'">
                  {{ auth.user?.role === 'admin' ? '管理员' : '用户' }}
                </Tag>
              </Descriptions.Item>
              <Descriptions.Item label="所属组">
                {{ auth.user?.group_name || (auth.user?.group_id ? `#${auth.user.group_id}` : '—') }}
              </Descriptions.Item>
            </Descriptions>
          </Tabs.TabPane>

          <!-- 密码管理 -->
          <Tabs.TabPane key="password" tab="密码管理">
            <Alert type="info" show-icon class="mb-3"
                   message="修改密码需验证当前密码；OIDC 用户首次设置可留空当前密码" />
            <Form layout="vertical" @submit.prevent="savePassword">
              <Form.Item label="当前密码">
                <Input.Password v-model:value="pwdForm.current_password" placeholder="已设密码时必填；OIDC 首次设置可留空" />
              </Form.Item>
              <Form.Item label="新密码" required>
                <Input.Password v-model:value="pwdForm.new_password" placeholder="至少 8 个字符" />
              </Form.Item>
              <Button type="primary" :loading="savingPwd" @click="savePassword">保存密码</Button>
            </Form>
          </Tabs.TabPane>

          <!-- OIDC 绑定（仅 OIDC 已配置时显示） -->
          <Tabs.TabPane v-if="oidcConfigured" key="oidc" tab="OIDC 绑定">
            <template v-if="auth.user?.user_source === 'oidc'">
              <Descriptions :column="1" size="middle">
                <Descriptions.Item label="绑定状态">
                  <Tag color="green">已绑定 OIDC</Tag>
                </Descriptions.Item>
                <Descriptions.Item label="身份标识">
                  {{ subject }}
                </Descriptions.Item>
              </Descriptions>
            </template>
            <template v-else>
              <Alert type="info" show-icon class="mb-3"
                     message="绑定后可使用 OIDC 提供商账号登录本系统" />
              <Button type="primary" @click="startBind">绑定 OIDC</Button>
            </template>
          </Tabs.TabPane>
        </Tabs>
      </Card>
    </main>
  </div>
</template>
