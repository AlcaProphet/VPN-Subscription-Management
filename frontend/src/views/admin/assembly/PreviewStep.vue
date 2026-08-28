<!-- PreviewStep.vue：装配步骤⑤ 预览与 Diff（Design2-UI §5.3.5）
     支持行号、搜索高亮、复制、换行切换，预览与 Diff 通过 Tabs 切换。 -->
<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Alert, Button, Input, Space, Tabs } from 'ant-design-vue'
import DiffView from '@/components/DiffView.vue'
import { Notify } from '@/components/Notify'
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

// 预览视图工具：行号 / 搜索高亮 / 换行 / 复制
const activeView = ref<'preview' | 'diff'>('preview')
const wrap = ref(true)
const searchQuery = ref('')
watch(() => props.showDiff, (v) => {
  if (v) activeView.value = 'diff'
}, { immediate: true })

const previewLines = computed(() => (props.previewText || '').split('\n'))
const matchingLines = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  if (!q) return 0
  return previewLines.value.filter((line) => line.toLowerCase().includes(q)).length
})

function splitLine(line: string): Array<{ text: string; hit: boolean }> {
  const q = searchQuery.value.trim().toLowerCase()
  if (!q) return [{ text: line, hit: false }]
  const lower = line.toLowerCase()
  const parts: Array<{ text: string; hit: boolean }> = []
  let idx = 0
  while (idx <= line.length) {
    const i = lower.indexOf(q, idx)
    if (i < 0) {
      if (line.slice(idx)) parts.push({ text: line.slice(idx), hit: false })
      break
    }
    if (i > idx) parts.push({ text: line.slice(idx, i), hit: false })
    parts.push({ text: line.slice(i, i + q.length), hit: true })
    idx = i + q.length
  }
  return parts.length ? parts : [{ text: line, hit: false }]
}

async function copyPreview() {
  try {
    await navigator.clipboard.writeText(props.previewText || '')
    Notify.success('预览已复制')
  } catch {
    Notify.error('复制失败，请手动复制')
  }
}
</script>

<template>
  <div class="space-y-3">
    <Space wrap>
      <Button type="primary" :loading="previewing" @click="emit('preview')">刷新预览</Button>
      <Button :loading="diffLoading" @click="emit('toggle-diff')">与当前激活版本对比</Button>
      <Button v-if="previewText" @click="copyPreview">复制</Button>
      <Button @click="wrap = !wrap">{{ wrap ? '取消换行' : '自动换行' }}</Button>
    </Space>
    <Alert v-if="previewText && previewStale" type="warning" show-icon message="配置已变化，请重新预览" />
    <Alert v-else-if="previewText && previewMeta" type="success" show-icon :message="previewMeta" />
    <Alert v-for="(w, i) in previewWarnings" :key="i" type="warning" show-icon :message="w" />
    <Alert v-for="(s, i) in previewSkipped" :key="'s'+i" type="warning" show-icon :message="`跳过 ${s.name}：${s.reason}`" />
    <Alert v-if="previewText.includes('# {{xray_nodes}}')" type="info" show-icon
           message="# {{xray_nodes}} 占位将在下载时按用户分配节点动态注入" />

    <Tabs v-if="previewText" :active-key="activeView" @change="(k: any) => activeView = k">
      <Tabs.TabPane key="preview" tab="预览">
        <Input.Search v-model:value="searchQuery" allow-clear placeholder="搜索预览内容"
                      class="mb-2 max-w-sm" enter-button="搜索" />
        <div v-if="searchQuery.trim()" class="text-xs mb-1 text-text-secondary">
          匹配 {{ matchingLines }} 行
        </div>
        <details v-if="previewStale" class="rounded border border-dashed border p-3 opacity-70">
          <summary class="cursor-pointer text-sm">查看已过期的预览</summary>
          <div class="mt-3 bg-surface-subtle rounded p-3 text-xs overflow-auto max-h-[50vh] font-mono"
               :class="wrap ? 'whitespace-pre-wrap' : 'whitespace-pre'">
            <div v-for="(line, i) in previewLines" :key="i" class="flex">
              <span class="select-none w-8 flex-none pr-2 text-right text-text-tertiary">{{ i + 1 }}</span>
              <span class="flex-1 min-w-0">
                <template v-for="(part, pi) in splitLine(line)" :key="pi">
                  <mark v-if="part.hit" class="bg-yellow-200 dark:bg-yellow-900/50 rounded px-0.5">{{ part.text }}</mark>
                  <span v-else>{{ part.text }}</span>
                </template>
                <span v-if="line === ''"> </span>
              </span>
            </div>
          </div>
        </details>
        <div v-else class="bg-surface-subtle rounded p-3 text-xs overflow-auto max-h-[50vh] font-mono"
             :class="wrap ? 'whitespace-pre-wrap' : 'whitespace-pre'">
          <div v-for="(line, i) in previewLines" :key="i" class="flex">
            <span class="select-none w-8 flex-none pr-2 text-right text-text-tertiary">{{ i + 1 }}</span>
            <span class="flex-1 min-w-0">
              <template v-for="(part, pi) in splitLine(line)" :key="pi">
                <mark v-if="part.hit" class="bg-yellow-200 dark:bg-yellow-900/50 rounded px-0.5">{{ part.text }}</mark>
                <span v-else>{{ part.text }}</span>
              </template>
              <span v-if="line === ''"> </span>
            </span>
          </div>
        </div>
      </Tabs.TabPane>
      <Tabs.TabPane key="diff" tab="差异对比" :disabled="!showDiff">
        <DiffView v-if="showDiff && previewText && !previewStale" :old-text="diffOld" :new-text="previewText" :target-missing="diffMissing" />
        <Alert v-else type="info" show-icon message="请点击“与当前激活版本对比”加载差异数据。" />
      </Tabs.TabPane>
    </Tabs>
    <div v-else class="text-sm text-text-secondary">尚未生成预览，请先点击“刷新预览”。</div>
  </div>
</template>
