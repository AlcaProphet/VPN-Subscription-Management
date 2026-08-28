<!-- StateContainer.vue：统一加载、错误、空与内容四态，页面只保留业务内容。 -->
<script setup lang="ts">
import { Button, Result, Skeleton } from 'ant-design-vue'
import EmptyState from './EmptyState.vue'

withDefaults(defineProps<{
  loading?: boolean
  error?: string
  empty?: boolean
  emptyTitle?: string
  emptyText?: string
}>(), {
  loading: false,
  error: '',
  empty: false,
  emptyTitle: '暂无数据',
  emptyText: '',
})

const emit = defineEmits<{ retry: [] }>()
</script>

<template>
  <Skeleton v-if="loading" active />
  <Result v-else-if="error" status="error" title="加载失败" :sub-title="error">
    <template #extra>
      <Button type="primary" @click="emit('retry')">重试</Button>
    </template>
  </Result>
  <template v-else-if="empty">
    <slot name="empty">
      <EmptyState :title="emptyTitle" :description="emptyText">
        <template v-if="$slots['empty-actions']" #actions><slot name="empty-actions" /></template>
      </EmptyState>
    </slot>
  </template>
  <slot v-else />
</template>
