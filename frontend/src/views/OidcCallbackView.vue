<!-- OidcCallbackView.vue：OIDC 回调中转页 /login/callback——提取 token → 存 store → 立即清空 URL → 跳 / -->
<script setup lang="ts">
import { onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()

onMounted(() => {
  const token = route.query.token as string | undefined
  if (token) {
    auth.setSession(token)
    history.replaceState(null, '', '/login/callback') // 立即清空 URL（含 token），Design1 §3.2
  }
  router.replace('/')
})
</script>
<template><div class="text-center text-gray-500 py-20">正在登录…</div></template>
