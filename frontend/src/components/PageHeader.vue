<!-- PageHeader.vue：管理面板页面头（Design2-UI §1.3）——标题 + 副标题 + 右侧操作区 -->
<script setup lang="ts">
defineProps<{ title: string; subtitle?: string }>()
</script>

<template>
  <div class="flex items-start justify-between gap-3 mb-4 flex-wrap">
    <div class="min-w-0">
      <h2 class="text-lg font-semibold m-0 text-gray-900 dark:text-gray-100">{{ title }}</h2>
      <div v-if="subtitle" class="text-sm text-gray-400 mt-1">{{ subtitle }}</div>
    </div>
    <div class="mobile-actions flex items-center gap-2 flex-shrink-0">
      <slot name="actions" />
    </div>
  </div>
</template>
