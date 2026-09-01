<!-- PageHeader.vue：管理面板页面头（Design2-UI §1.3）——唯一 h1、说明与右侧操作区。 -->
<script setup lang="ts">
defineProps<{ title: string; subtitle?: string }>()
</script>

<template>
  <div class="flex items-start justify-between gap-3 mb-4 flex-wrap">
    <div class="min-w-0">
      <h1 class="text-2xl leading-8 font-semibold m-0 text-text">{{ title }}</h1>
      <div v-if="subtitle" class="text-sm text-text-tertiary mt-1">{{ subtitle }}</div>
    </div>
    <div class="mobile-actions flex items-center gap-2 flex-shrink-0">
      <slot name="actions" />
    </div>
  </div>
</template>
