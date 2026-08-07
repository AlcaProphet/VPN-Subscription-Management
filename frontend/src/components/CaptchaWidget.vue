<!-- CaptchaWidget.vue：按 provider/pages 渲染 reCAPTCHA/Turnstile；脚本加载失败显示明确错误文案（不静默卡死） -->
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Alert } from 'ant-design-vue'
import { useSystemStore } from '@/stores/system'

const props = defineProps<{ page: 'register' | 'login' | 'forgot' }>()
const emit = defineEmits<{ 'update:token': [string] }>()
const system = useSystemStore()
const loadError = ref(false)

// 启用判定与后端 Enforced 对齐：provider 非 off 且页面在 captcha_pages
const enabled = computed(() => {
  const st = system.status
  return !!st && !!st.captcha_provider && st.captcha_provider !== 'off'
    && (st.captcha_pages ?? []).includes(props.page) && !!st.captcha_site_key
})

// 提供商脚本地址
function providerScriptURL(provider: string, siteKey: string): string {
  if (provider === 'turnstile') {
    return `https://challenges.cloudflare.com/turnstile/v0/api.js?onload=onCaptchaLoad&render=explicit`
  }
  return `https://www.google.com/recaptcha/api.js?onload=onCaptchaLoad&render=${siteKey}`
}

// 全局回调（脚本加载完成后渲染）
declare global {
  interface Window {
    onCaptchaLoad?: () => void
    grecaptcha?: { render: (el: string | HTMLElement, opts: Record<string, unknown>) => void }
    turnstile?: { render: (el: string | HTMLElement, opts: Record<string, unknown>) => string }
  }
}

function renderWidget() {
  const st = system.status
  if (!st) return
  const el = document.getElementById('captcha-container')
  if (!el) return
  if (st.captcha_provider === 'turnstile' && window.turnstile) {
    window.turnstile.render(el, {
      sitekey: st.captcha_site_key,
      callback: (token: string) => emit('update:token', token),
    })
    return
  }
  if (st.captcha_provider === 'recaptcha' && window.grecaptcha) {
    window.grecaptcha.render(el, {
      sitekey: st.captcha_site_key,
      callback: (token: string) => emit('update:token', token),
    })
  }
}

onMounted(() => {
  if (!enabled.value) return
  const st = system.status!
  // 动态加载提供商脚本，onerror → loadError=true
  const script = document.createElement('script')
  script.src = providerScriptURL(st.captcha_provider!, st.captcha_site_key!)
  script.async = true
  script.onerror = () => { loadError.value = true }
  window.onCaptchaLoad = () => renderWidget()
  script.onload = () => { /* onCaptchaLoad 由脚本内部触发 */ }
  document.head.appendChild(script)
})
</script>

<template>
  <div v-if="enabled" class="mb-4">
    <Alert v-if="loadError" type="error" message="验证码加载失败，请检查网络后刷新重试" />
    <div v-else id="captcha-container" />
  </div>
</template>
