import { ref, watchEffect } from 'vue'

const THEME_KEY = 'vpn-theme'

const isDark = ref(false)

function initTheme() {
  const stored = localStorage.getItem(THEME_KEY)
  if (stored === 'dark') {
    isDark.value = true
  } else if (stored === 'light') {
    isDark.value = false
  } else {
    // Use system preference
    isDark.value = window.matchMedia('(prefers-color-scheme: dark)').matches
  }
}

function toggle() {
  isDark.value = !isDark.value
  localStorage.setItem(THEME_KEY, isDark.value ? 'dark' : 'light')
}

// Sync DOM class on change
watchEffect(() => {
  document.documentElement.classList.toggle('dark', isDark.value)
})

// Initialize on load
initTheme()

export function useTheme() {
  return {
    isDark,
    toggle
  }
}
