import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { authApi, publicApi } from '@/services/api'

export const useUserStore = defineStore('user', () => {
  // State
  const user = ref(null)
  const token = ref(localStorage.getItem('jwt') || null)
  const isConfigured = ref(null) // null = not yet checked

  // Getters
  const isLoggedIn = computed(() => !!token.value)
  const isAdmin = computed(() => user.value?.role === 'admin')
  const isAdvanced = computed(() => user.value?.is_advanced === true)

  // Actions
  async function checkSystemStatus() {
    if (isConfigured.value !== null) return // already cached
    try {
      const res = await publicApi.getSystemStatus()
      isConfigured.value = res.data.configured
    } catch (e) {
      // Don't cache errors — keep null so the guard retries on next navigation
    }
  }

  async function fetchUser() {
    if (!token.value) return
    try {
      const res = await authApi.getMe()
      user.value = res.data
    } catch (e) {
      // Token invalid or expired
      user.value = null
      token.value = null
      localStorage.removeItem('jwt')
    }
  }

  function login(jwt) {
    token.value = jwt
    localStorage.setItem('jwt', jwt)
  }

  function logout(router) {
    user.value = null
    token.value = null
    localStorage.removeItem('jwt')
    if (router) {
      router.push('/login')
    }
  }

  return {
    user,
    token,
    isConfigured,
    isLoggedIn,
    isAdmin,
    isAdvanced,
    checkSystemStatus,
    fetchUser,
    login,
    logout
  }
})
