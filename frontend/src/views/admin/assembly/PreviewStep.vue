<!-- PreviewStep.vue：装配步骤⑤ 预览与 Diff（Design2-UI §5.3.5） -->
<script setup lang="ts">
import { computed } from 'vue'
import { Alert, Button, Space } from 'ant-design-vue'
import DiffView from '@/components/DiffView.vue'
import type { TargetSyntax } from '@/api/assembly'

const props = defineProps<{
  previewing: boolean
  previewWarnings: string[]
  previewSkipped: Array<{ name: string; reason: string }>
  previewText: string
  previewStale: boolean
  previewedAt: number | null
  previewedTargetSyntax: TargetSyntax | null
  showDiff: boolean
  diffOld: string
  diffMissing: boolean
  diffLoading: boolean
}>()

const emit = defineEmits<{ preview: []; 'toggle-diff': [] }>()

const targetLabels: Record<TargetSyntax, string> = {
  'clash-yaml': 'Clash YAML',
  'sr-subs': 'SR 节点订阅',
  'generic-subs': '通用节点订阅',
  'sr-conf': 'SR 分流规则',
}
const previewMeta = computed(() => {
  if (!props.previewedAt || !props.previewedTargetSyntax) return ''
  return `最近预览：${new Date(props.previewedAt).toLocaleString('zh-CN')} · ${targetLabels[props.previewedTargetSyntax]}`
})
</script>

<template>
  <div class="space-y-3">
    <Space>
      <Button type="primary" :loading="previewing" @click="emit('preview')">刷新预览</Button>
      <Button :loading="diffLoading" @click="emit('toggle-diff')">与当前激活版本对比</Button>
    </Space>
    <Alert v-if="previewText && previewStale" type="warning" show-icon message="配置已变化，请重新预览" />
    <Alert v-else-if="previewText && previewMeta" type="success" show-icon :message="previewMeta" />
    <Alert v-for="(w, i) in previewWarnings" :key="i" type="warning" show-icon :message="w" />
    <Alert v-for="(s, i) in previewSkipped" :key="'s'+i" type="warning" show-icon :message="`跳过 ${s.name}：${s.reason}`" />
    <Alert v-if="previewText.includes('# {{xray_nodes}}')" type="info" show-icon
           message="# {{xray_nodes}} 占位将在下载时按用户分配节点动态注入" />
    <div v-if="previewText && !previewStale">
      <h3 class="font-semibold mb-2">预览</h3>
      <pre class="bg-gray-50 dark:bg-gray-900 rounded p-3 text-xs overflow-auto max-h-[50vh] whitespace-pre-wrap">{{ previewText }}</pre>
    </div>
    <details v-else-if="previewText" class="rounded border border-dashed border-gray-300 p-3 opacity-70">
      <summary class="cursor-pointer text-sm">查看已过期的预览</summary>
      <pre class="mt-3 bg-gray-50 dark:bg-gray-900 rounded p-3 text-xs overflow-auto max-h-[50vh] whitespace-pre-wrap">{{ previewText }}</pre>
    </details>
    <DiffView v-if="showDiff && previewText && !previewStale" :old-text="diffOld" :new-text="previewText" :target-missing="diffMissing" />
  </div>
</template>
