<!-- NodeCheckPanel.vue：节点目标检查面板（Build19 Step 5）
  请求使用当前草稿；草稿变化会使旧结果失效，迟到响应不覆盖新状态。
-->
<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Alert, Button, Tag } from 'ant-design-vue'
import { checkNode } from '@/api/node'
import type { NodeCheckRequest, NodeCheckResponse, TargetCheckResult } from '@/api/node'
import { ApiError } from '@/api/request'

const props = defineProps<{
  request: NodeCheckRequest
}>()

const emit = defineEmits<{
  conflict: []
}>()

const checking = ref(false)
const result = ref<NodeCheckResponse | null>(null)
const error = ref('')
const seq = ref(0)

watch(() => props.request, () => {
  result.value = null
  error.value = ''
  seq.value += 1
}, { deep: true, immediate: true })

async function run() {
  checking.value = true
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
</script>

<template>
  <div class="node-check-panel rounded-lg border p-3">
    <div class="mb-2 flex items-center justify-between gap-3">
      <div>
        <div class="text-sm font-medium text-text">目标检查</div>
        <div class="text-xs text-text-tertiary">按当前草稿检查去敏输出与诊断；检查不写库，也不保存节点。</div>
      </div>
      <Button type="primary" size="small" :loading="checking" @click="run">检查当前节点</Button>
    </div>

    <Alert v-if="error" type="error" show-icon class="mb-2" :message="error" />

    <div v-if="checking && !result" class="text-xs text-text-tertiary">正在检查…</div>

    <div v-else-if="result" class="space-y-3">
      <div v-for="key in targetKeys" :key="key" class="rounded-md border p-3">
        <div class="flex items-center justify-between gap-2">
          <span class="text-sm font-medium text-text">{{ key }}</span>
          <Tag :color="statusColor(result.targets[key].status)">{{ result.targets[key].status }}</Tag>
        </div>
        <div v-if="diagnosticsFor(result.targets[key]).length" class="mt-2 space-y-1">
          <div v-for="(diag, index) in sortedDiagnostics(result.targets[key])" :key="index" class="text-xs">
            <span class="font-mono text-text-tertiary">{{ diag.code }}</span>
            <span class="mx-1">·</span>
            <span class="text-text">{{ diag.message }}</span>
            <span v-if="diag.field_path" class="ml-1 font-mono text-text-tertiary">[{{ diag.field_path }}]</span>
          </div>
        </div>
        <div v-else class="text-xs text-text-tertiary mt-2">未发现诊断</div>
        <details v-if="result.targets[key].preview" class="mt-2">
          <summary class="cursor-pointer text-xs text-text-secondary">脱敏产物（不能直接连接）</summary>
          <pre class="mt-1 overflow-auto rounded bg-gray-50 p-2 text-xs">{{ result.targets[key].preview }}</pre>
        </details>
      </div>
      <p class="text-xs text-text-tertiary">检查结果仅用于提示，不自动保存节点。</p>
    </div>
  </div>
</template>
