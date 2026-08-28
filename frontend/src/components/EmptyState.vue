<!-- EmptyState.vue：统一空状态，保留标题、下一步说明与最多两个明确操作。 -->
<script setup lang="ts">
import { Empty } from 'ant-design-vue'

withDefaults(defineProps<{
  title?: string
  description?: string
  compact?: boolean
}>(), {
  title: '暂无数据',
  description: '',
  compact: false,
})
</script>

<template>
  <div class="empty-state text-center" :class="compact ? 'py-6' : 'py-12'">
    <div>
      <Empty :description="false" />
      <div class="space-y-1">
        <p class="m-0 font-medium text-text">{{ title }}</p>
        <p v-if="description" class="m-0 text-sm text-text-secondary">{{ description }}</p>
      </div>
      <div v-if="$slots.actions" class="mt-4 flex flex-wrap justify-center gap-2">
        <slot name="actions" />
      </div>
    </div>
  </div>
</template>

<style scoped>
.empty-state { min-height: 11rem; display: grid; place-items: center; }
</style>
