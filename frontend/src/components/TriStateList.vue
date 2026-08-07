<!-- TriStateList.vue：加载中/空/列表三态封装（UI §7.5） -->
<script setup lang="ts">
import { Skeleton, Result, Empty, Button } from 'ant-design-vue'

defineProps<{
  loading: boolean
  empty: boolean          // 数据为空
  error?: string          // 错误信息（存在时展示错误态 + 重试）
  emptyText?: string      // 空状态引导文案（UI §7.5）
}>()
const emit = defineEmits<{ retry: [] }>()
</script>

<template>
  <Skeleton v-if="loading" active />
  <Result v-else-if="error" status="error" :sub-title="error">
    <template #extra><Button @click="emit('retry')">重试</Button></template>
  </Result>
  <Empty v-else-if="empty" :description="emptyText ?? '暂无数据'" />
  <slot v-else />
</template>
