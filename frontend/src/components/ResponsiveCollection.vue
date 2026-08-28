<!-- ResponsiveCollection.vue：列表页面统一桌面表格与手机卡片的呈现选择。 -->
<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'

const isMobile = ref(false)
function syncViewport() {
  isMobile.value = window.matchMedia('(max-width: 767px)').matches
}
onMounted(() => {
  syncViewport()
  window.addEventListener('resize', syncViewport)
})
onUnmounted(() => window.removeEventListener('resize', syncViewport))
</script>

<template>
  <slot v-if="!isMobile" name="table" />
  <slot v-else name="cards" />
</template>
