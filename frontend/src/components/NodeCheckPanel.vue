<!-- NodeCheckPanel.vue：节点目标检查面板（Build19 Step 5）
  请求使用当前草稿；草稿变化会使旧结果失效，迟到响应不覆盖新状态。
-->
<script setup lang="ts">
import { computed, ref, useId, watch } from 'vue'
import { Alert, Button, Tag } from 'ant-design-vue'
import { checkNode } from '@/api/node'
import type { NodeCheckRequest, NodeCheckResponse, TargetCheckResult } from '@/api/node'
import { ApiError } from '@/api/request'

const props = defineProps<{
  request: NodeCheckRequest
  blockedReason?: string
}>()

const emit = defineEmits<{
  conflict: []
  locate: []
}>()

const titleId = useId()
const checking = ref(false)
const result = ref<NodeCheckResponse | null>(null)
const error = ref('')
const seq = ref(0)

watch(() => props.request, () => {
  checking.value = false
  result.value = null
  error.value = ''
  seq.value += 1
}, { deep: true, immediate: true })

async function run() {
  if (props.blockedReason) return
  checking.value = true
  result.value = null
  error.value = ''
  const currentSeq = ++seq.value
  try {
    const res = await checkNode({ ...props.request })
    if (currentSeq !== seq.value) return
    result.value = res
  } catch (err) {
    if (currentSeq !== seq.value) return
    if (err instanceof ApiError && err.status === 409) {
      error.value = '节点已被其他编辑更新，请重新加载后重试'
      emit('conflict')
    } else {
      error.value = (err as Error).message
    }
  } finally {
    if (currentSeq === seq.value) checking.value = false
  }
}

function statusColor(status: string): string {
  switch (status) {
    case 'ok': return 'green'
    case 'warn': return 'orange'
    case 'error': return 'red'
    default: return 'default'
  }
}

function sortedDiagnostics(target: TargetCheckResult): TargetCheckResult['diagnostics'] {
  const order: Record<string, number> = { error: 0, warn: 1, info: 2 }
  return [...diagnosticsFor(target)].sort((a, b) => (order[a.severity] ?? 3) - (order[b.severity] ?? 3))
}

function diagnosticsFor(target: TargetCheckResult): TargetCheckResult['diagnostics'] {
  return target.diagnostics ?? []
}

const targetKeys = computed(() => result.value ? Object.keys(result.value.targets) : [])
const resultSummary = computed(() => {
  if (!result.value) return ''
  const targets = Object.values(result.value.targets)
  const counts = new Map<string, number>()
  for (const { status } of targets) {
    counts.set(status, (counts.get(status) ?? 0) + 1)
  }
  return `检查结果（${targets.length} 个目标） · 通过 ${counts.get('ok') ?? 0} · 警告 ${counts.get('warn') ?? 0} · 错误 ${counts.get('error') ?? 0} · 跳过 ${counts.get('skip') ?? 0}`
})
</script>

<template>
  <section :aria-labelledby="titleId" class="node-check-panel min-w-0 rounded-lg border p-3">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h2 :id="titleId" class="m-0 text-sm font-medium text-text">目标检查</h2>
        <p class="m-0 mt-1 text-xs text-text-tertiary">按当前草稿检查脱敏输出与诊断；检查不写库，也不保存节点。</p>
      </div>
      <div class="flex flex-wrap items-center gap-2">
        <Button v-if="blockedReason" size="small" @click="emit('locate')">定位草稿</Button>
        <Button type="primary" size="small" :loading="checking" :disabled="!!blockedReason" @click="run">检查当前节点</Button>
      </div>
    </div>

    <Alert v-if="blockedReason" type="warning" show-icon class="mt-2" :message="blockedReason" />
    <Alert v-else-if="error" type="error" show-icon class="mt-2" :message="error" />

    <div role="status" :class="checking ? 'mt-2 text-xs text-text-tertiary' : 'sr-only'">
      {{ checking ? '正在检查…' : resultSummary }}
    </div>

    <details v-if="result" class="mt-2 border-t">
      <summary class="cursor-pointer rounded-sm py-3 text-sm font-medium text-text focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary md:py-1">
        {{ resultSummary }}
      </summary>
      <div class="mt-2 space-y-3">
        <div v-for="key in targetKeys" :key="key" class="rounded-md border p-3">
          <div class="flex items-center justify-between gap-2">
            <span class="text-sm font-medium text-text">{{ key }}</span>
            <Tag :color="statusColor(result.targets[key].status)">{{ result.targets[key].status }}</Tag>
          </div>
          <div v-if="diagnosticsFor(result.targets[key]).length" class="mt-2 space-y-1">
            <div v-for="(diag, index) in sortedDiagnostics(result.targets[key])" :key="index" class="text-xs wrap-anywhere">
              <span class="font-mono text-text-tertiary">{{ diag.code }}</span>
              <span class="mx-1">·</span>
              <span class="text-text">{{ diag.message }}</span>
              <span v-if="diag.field_path" class="ml-1 font-mono text-text-tertiary">[{{ diag.field_path }}]</span>
            </div>
          </div>
          <div v-else class="mt-2 text-xs text-text-tertiary">未发现诊断</div>
          <details v-if="result.targets[key].preview" class="mt-2">
            <summary class="cursor-pointer text-xs text-text-secondary">脱敏产物（不能直接连接）</summary>
            <pre class="mt-1 overflow-auto rounded bg-surface-subtle p-2 text-xs">{{ result.targets[key].preview }}</pre>
          </details>
        </div>
      </div>
    </details>
  </section>
</template>
