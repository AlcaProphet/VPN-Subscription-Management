<template>
  <div class="flex items-center justify-center min-h-screen p-5 bg-gray-50 dark:bg-gray-900">
    <div class="text-center p-12 rounded-xl bg-white dark:bg-gray-800 shadow-lg max-w-sm w-full">
      <h1 class="m-0 mb-2 text-3xl font-bold text-gray-900 dark:text-white">VPN 订阅管理</h1>
      <p class="m-0 mb-8 text-base text-gray-500 dark:text-gray-400">请通过 OIDC 认证登录</p>

      <button
        class="w-full h-12 text-base bg-blue-600 hover:bg-blue-700 text-white rounded-md mb-3"
        @click="handleLogin"
      >
        通过 OIDC 登录
      </button>

      <button
        class="text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300 text-sm bg-transparent border-none cursor-pointer"
        @click="handleSwitchAccount"
      >
        使用其他账号登录
      </button>

      <div class="mt-6">
        <button
          class="w-9 h-9 rounded-full border border-gray-300 dark:border-gray-600 flex items-center justify-center bg-white dark:bg-gray-700 hover:bg-gray-50 dark:hover:bg-gray-600 mx-auto"
          @click="toggleTheme"
        >
          <!-- Sun icon for dark mode, Moon icon for light mode -->
          <svg v-if="isDark" class="w-5 h-5 text-yellow-500" fill="currentColor" viewBox="0 0 24 24">
            <path d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z"/>
          </svg>
          <svg v-else class="w-5 h-5 text-gray-600" fill="currentColor" viewBox="0 0 24 24">
            <path d="M21.752 15.002A9.718 9.718 0 0118 15.75c-5.385 0-9.75-4.365-9.75-9.75 0-1.33.266-2.597.748-3.752A9.753 9.753 0 003 11.25C3 16.635 7.365 21 12.75 21a9.753 9.753 0 009.002-5.998z"/>
          </svg>
        </button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useUserStore } from '@/stores/user'
import { useTheme } from '@/composables/useTheme'

const router = useRouter()
const userStore = useUserStore()
const { isDark, toggle: toggleTheme } = useTheme()

function handleLogin() {
  window.location.href = '/api/v1/auth/login'
}

function handleSwitchAccount() {
  window.location.href = '/api/v1/auth/login?prompt=login'
}

onMounted(() => {
  if (userStore.token) {
    router.push('/')
  }
})
</script>
