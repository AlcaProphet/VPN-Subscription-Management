<!-- PageShell.vue：管理页统一骨架。旧页面可继续使用 PageHeader，新增页面由本组件收口标题与状态。 -->
<script setup lang="ts">
import PageHeader from './PageHeader.vue'
import StateContainer from './StateContainer.vue'

withDefaults(defineProps<{
  title: string
  description?: string
  loading?: boolean
  error?: string
  empty?: boolean
  emptyTitle?: string
  emptyText?: string
}>(), {
  description: '',
  loading: false,
  error: '',
  empty: false,
  emptyTitle: '暂无数据',
  emptyText: '',
})

const emit = defineEmits<{ retry: [] }>()
</script>

<template>
  <section>
    <PageHeader :title="title" :subtitle="description">
      <template v-if="$slots.actions" #actions><slot name="actions" /></template>
    </PageHeader>
    <StateContainer :loading="loading" :error="error" :empty="empty" :empty-title="emptyTitle" :empty-text="emptyText"
                    @retry="emit('retry')">
      <template v-if="$slots.empty" #empty><slot name="empty" /></template>
      <template v-if="$slots['empty-actions']" #empty-actions><slot name="empty-actions" /></template>
      <slot />
    </StateContainer>
  </section>
</template>
