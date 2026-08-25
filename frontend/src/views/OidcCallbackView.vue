<!-- OidcCallbackView.vue：OIDC 回调中转页 /login/callback——用 HttpOnly ticket 换 token → 存 store → 跳 / -->
<script setup lang="ts">
import { onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { exchangeOidc } from '@/api/oidc'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const auth = useAuthStore()

onMounted(async () => {
  try {
    const res = await exchangeOidc()
    auth.setSession(res.token)
    await router.replace('/')
  } catch {
    await router.replace('/login?oidc_error=exchange_failed')
  }
})
</script>
<template><div class="text-center text-gray-500 py-20">正在登录…</div></template>
