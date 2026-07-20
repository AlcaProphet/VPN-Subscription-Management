<template>
  <div class="login-container">
    <div class="login-card">
      <h1 class="login-title">VPN 订阅管理</h1>
      <p class="login-subtitle">请通过 OIDC 认证登录</p>

      <el-button
        class="login-btn"
        type="primary"
        size="large"
        @click="handleLogin"
      >
        通过 OIDC 登录
      </el-button>

      <el-button
        class="switch-account-btn"
        type="default"
        size="default"
        text
        @click="handleSwitchAccount"
      >
        使用其他账号登录
      </el-button>

      <div class="theme-toggle">
        <el-button
          :icon="isDark ? Sunny : Moon"
          circle
          @click="toggleTheme"
        />
      </div>
    </div>
  </div>
</template>

<script setup>
import { onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { Sunny, Moon } from '@element-plus/icons-vue'
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

<style scoped>
.login-container {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 20px;
  background: var(--el-bg-color-page);
}

.login-card {
  text-align: center;
  padding: 48px 40px;
  border-radius: 12px;
  background: var(--el-bg-color);
  box-shadow: var(--el-box-shadow-light);
  max-width: 400px;
  width: 100%;
}

.login-title {
  margin: 0 0 8px;
  font-size: 28px;
  font-weight: 700;
  color: var(--el-text-color-primary);
}

.login-subtitle {
  margin: 0 0 32px;
  font-size: 15px;
  color: var(--el-text-color-secondary);
}

.login-btn {
  width: 100%;
  height: 48px;
  font-size: 16px;
}

.switch-account-btn {
  margin-top: 12px;
  color: var(--el-text-color-secondary);
}

.theme-toggle {
  margin-top: 24px;
}
</style>
