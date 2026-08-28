<!-- OidcCallbackView.vue：OIDC 回调中转页 /login/callback——用 HttpOnly ticket 换 token → 存 store → 跳 / -->
<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Button, Result, Space, Alert } from 'ant-design-vue'
import { exchangeOidc } from '@/api/oidc'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const auth = useAuthStore()
const loading = ref(true)
const errorMessage = ref('')
const contactVisible = ref(false)

function startOidcLogin() {
  window.location.href = '/api/auth/oidc/login'
}

onMounted(async () => {
  try {
    const res = await exchangeOidc()
    auth.setSession(res.token)
    await router.replace('/')
  } catch (err) {
    errorMessage.value = (err as Error).message || 'OIDC 登录未能完成，请稍后重试。'
  } finally {
    loading.value = false
  }
})
</script>
<template>
  <div class="w-full max-w-md py-20 mx-auto">
    <div class="bg-white dark:bg-gray-800 dark:text-gray-100 rounded-lg shadow p-8">
      <div v-if="loading" class="text-center text-gray-500">正在登录…</div>
      <Result v-else-if="errorMessage" status="error" title="无法完成 OIDC 登录" :sub-title="errorMessage">
        <template #extra>
          <Space wrap>
            <Button type="primary" @click="startOidcLogin">重新使用 OIDC 登录</Button>
            <Button @click="router.replace('/login')">使用本地账号</Button>
            <Button @click="contactVisible = !contactVisible">联系管理员</Button>
          </Space>
          <Alert v-if="contactVisible" type="info" class="mt-4 text-left" show-icon
                 message="请联系系统管理员，并说明 OIDC 登录未能完成。" />
        </template>
      </Result>
    </div>
  </div>
</template>
