<!-- TriStateList.vue：兼容旧页面的状态容器，统一委托给四态 StateContainer。 -->
<script setup lang="ts">
import StateContainer from './StateContainer.vue'

defineProps<{
  loading: boolean
  empty: boolean          // 数据为空
  error?: string          // 错误信息（存在时展示错误态 + 重试）
  emptyText?: string      // 空状态引导文案（UI §7.5）
}>()
const emit = defineEmits<{ retry: [] }>()
</script>

<template>
  <StateContainer :loading="loading" :empty="empty" :error="error ?? ''" :empty-text="emptyText ?? ''" @retry="emit('retry')">
    <template v-if="$slots.empty" #empty><slot name="empty" /></template>
    <template v-if="$slots['empty-actions']" #empty-actions><slot name="empty-actions" /></template>
    <slot />
  </StateContainer>
</template>
