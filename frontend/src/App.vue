<template>
  <!-- Route loading progress bar -->
  <div v-if="isNavigating" class="fixed top-0 left-0 right-0 z-[9999] h-0.5 bg-blue-600 animate-pulse" />
  <router-view />
  <!-- Toast 通知容器 (右下角) -->
  <TransitionGroup name="toast" tag="div" class="fixed bottom-4 right-4 z-50 flex flex-col gap-2">
    <div v-for="t in toasts" :key="t.id"
         class="px-4 py-2 rounded-lg shadow-lg text-white text-sm"
         :class="toastClass(t.type)">
      {{ t.message }}
    </div>
  </TransitionGroup>
</template>

<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useTheme } from '@/composables/useTheme'
import { useToast } from '@/composables/useToast'

const router = useRouter()
const isNavigating = ref(false)

router.beforeEach(() => {
  isNavigating.value = true
})
router.afterEach(() => {
  isNavigating.value = false
})

const { toasts } = useToast()
const toastClass = (type) => ({
  success: 'bg-green-600',
  error: 'bg-red-600',
  info: 'bg-gray-700',
  warning: 'bg-yellow-600',
})[type] || 'bg-gray-700'
useTheme()
</script>
