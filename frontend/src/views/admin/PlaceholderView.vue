<!-- PlaceholderView.vue：Build4 阶段的后续页面占位（Build5/7 替换为真实页面） -->
<script setup lang="ts">
import { Result } from 'ant-design-vue'

defineProps<{ title: string; description?: string }>()
</script>

<template>
  <Result status="info" :title="title" :sub-title="description ?? '将在后续构建轮次实现'">
    <template #extra />
  </Result>
</template>
