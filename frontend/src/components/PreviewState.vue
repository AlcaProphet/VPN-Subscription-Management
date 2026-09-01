<!-- PreviewState.vue：装配预览的四种状态统一文案与语义颜色。 -->
<script setup lang="ts">
import { Alert, Spin, Tag } from 'ant-design-vue'

const props = withDefaults(defineProps<{
  state: 'idle' | 'generating' | 'fresh' | 'stale'
  fingerprint?: string
}>(), { fingerprint: '' })

const labels = {
  idle: '尚未生成预览',
  generating: '正在生成预览',
  fresh: '预览为最新状态',
  stale: '预览已过期，请重新生成',
} as const
</script>

<template>
  <div class="flex items-center gap-2">
    <Spin v-if="props.state === 'generating'" size="small" />
    <Tag :color="props.state === 'fresh' ? 'green' : props.state === 'stale' ? 'orange' : 'default'">
      {{ labels[props.state] }}
    </Tag>
    <span v-if="fingerprint" class="text-xs text-text-tertiary">{{ fingerprint }}</span>
  </div>
  <Alert v-if="props.state === 'stale'" class="mt-2" type="warning" show-icon message="配置已变更，生成前请更新预览" />
</template>
