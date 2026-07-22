<template>
  <router-view />
  <!-- Toast 通知容器 (右下角) -->
  <div class="fixed bottom-4 right-4 z-50 flex flex-col gap-2">
    <div v-for="t in toasts" :key="t.id"
         class="px-4 py-2 rounded-lg shadow-lg text-white text-sm transition-all"
         :class="toastClass(t.type)">
      {{ t.message }}
    </div>
  </div>
</template>

<script setup>
import { useTheme } from '@/composables/useTheme'
import { useToast } from '@/composables/useToast'
const { toasts } = useToast()
const toastClass = (type) => ({
  success: 'bg-green-600',
  error: 'bg-red-600',
  info: 'bg-gray-700',
  warning: 'bg-yellow-600',
})[type] || 'bg-gray-700'
useTheme()
</script>
