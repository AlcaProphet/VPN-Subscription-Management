<!-- OpenVPNImportPanel.vue：Build32 Step 19 的 `.ovpn` 粘贴解析面板。
  只读解析：解析结果与原文都只存在内存，永不写入 localStorage；应用时只覆盖 parser 明确产出的字段，
  未产出的现有草稿字段不会被空响应静默删除；关闭面板或切换协议时立即丢弃原文与未应用草稿。
-->
<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Alert, Button, Input } from 'ant-design-vue'
import { parseOpenVPN } from '@/api/node'
import type { OpenVPNParseDiagnostic, OpenVPNParseResult } from '@/api/node'
import { ApiError } from '@/api/request'

const props = defineProps<{
  open: boolean
  sensitivePaths: string[]
}>()

const emit = defineEmits<{
  apply: [payload: { host: string; port: number; protocol_json: Record<string, unknown>; selectors: Record<string, string> }]
  'draft-dirty-change': [payload: { path: string; dirty: boolean }]
}>()

// 页面级阻断使用的稳定路径；与 JSON／自定义值草稿共用同一套阻断集合。
const DRAFT_PATH = 'openvpn-import'

const text = ref('')
const parsing = ref(false)
const result = ref<OpenVPNParseResult | null>(null)
const diagnostics = ref<OpenVPNParseDiagnostic[]>([])
const error = ref('')

const hasDraft = computed(() => text.value.trim() !== '')
const canApply = computed(() => result.value !== null)

watch(hasDraft, (dirty) => emit('draft-dirty-change', { path: DRAFT_PATH, dirty }))
// 关闭面板即丢弃原文与未应用结果，阻断随之解除。
watch(() => props.open, (open) => { if (!open) reset() })

function reset() {
  text.value = ''
  result.value = null
  diagnostics.value = []
  error.value = ''
  emit('draft-dirty-change', { path: DRAFT_PATH, dirty: false })
}

function isSensitive(name: string): boolean {
  return props.sensitivePaths.some((path) => path === name || path.startsWith(`${name}.`) || path.startsWith(`${name}[`))
}

// 脱敏结构：只展示字段名、来源行号与安全摘要，敏感字段不显示取值。
const maskedStructure = computed(() => {
  const parsed = result.value
  if (!parsed) return [] as { name: string; line?: number; preview: string; sensitive: boolean }[]
  return Object.entries(parsed.protocol_json ?? {}).map(([name, value]) => {
    const sensitive = isSensitive(name)
    let preview = '已解析'
    if (sensitive) {
      preview = '已解析（敏感值不展示，应用后按凭据流程处理）'
    } else if (typeof value === 'string') {
      const text = value.replace(/\s+/g, ' ').trim()
      preview = text.length > 60 ? `${text.slice(0, 60)}…` : text
    } else {
      preview = JSON.stringify(value)
    }
    return { name, line: parsed.field_sources?.[name], preview, sensitive }
  })
})

async function run() {
  if (!hasDraft.value) {
    error.value = '请先粘贴 `.ovpn` 内容'
    return
  }
  parsing.value = true
  error.value = ''
  result.value = null
  diagnostics.value = []
  try {
    const parsed = await parseOpenVPN(text.value)
    result.value = parsed
    diagnostics.value = parsed.diagnostics ?? []
  } catch (err) {
    if (err instanceof ApiError && err.status === 400) {
      const details = err.details as { error_code?: string; diagnostics?: OpenVPNParseDiagnostic[] } | undefined
      diagnostics.value = details?.diagnostics ?? []
      error.value = details?.error_code
        ? `解析被阻断（${details.error_code}），请按行号修正后重试`
        : err.message
    } else {
      error.value = (err as Error).message
    }
  } finally {
    parsing.value = false
  }
}

function apply() {
  const parsed = result.value
  if (!parsed) return
  emit('apply', {
    host: parsed.host ?? '',
    port: parsed.port ?? 0,
    protocol_json: parsed.protocol_json ?? {},
    selectors: parsed.selectors ?? {},
  })
  reset()
}
</script>

<template>
  <div class="openvpn-import-panel rounded-lg border p-3">
    <div class="mb-2">
      <div class="text-sm font-medium text-text">粘贴 `.ovpn` 解析</div>
      <div class="text-xs text-text-tertiary">
        只读解析：不读取外部文件、不执行脚本／hook／include、不落库。上限 256 KiB。
      </div>
    </div>

    <Input.TextArea
      v-model:value="text"
      data-field-path="openvpn-import-text"
      :rows="6"
      :disabled="!open"
      placeholder="粘贴 .ovpn 文本（含 <ca>／<cert>／<key> 等内嵌块）"
      class="font-mono"
    />

    <div class="mt-2 flex items-center gap-2">
      <Button size="small" :loading="parsing" :disabled="!open" @click="run">解析</Button>
      <Button size="small" type="primary" :disabled="!canApply" @click="apply">应用解析结果</Button>
      <Button size="small" :disabled="!hasDraft && !canApply" @click="reset">取消</Button>
    </div>

    <Alert v-if="error" type="error" show-icon class="mt-2" :message="error" />
    <div v-if="diagnostics.length" class="mt-2 space-y-1">
      <div v-for="(diagnostic, index) in diagnostics" :key="index" class="text-xs">
        <span class="font-mono text-text-tertiary">{{ diagnostic.code }}</span>
        <span v-if="diagnostic.line" class="ml-1 font-mono text-text-tertiary">行 {{ diagnostic.line }}</span>
        <span class="mx-1">·</span>
        <span :class="diagnostic.severity === 'error' ? 'text-danger' : 'text-text'">{{ diagnostic.message }}</span>
      </div>
    </div>

    <div v-if="result" class="mt-3 rounded-md border p-2">
      <div class="text-xs font-medium text-text">已解析结构（脱敏）</div>
      <div class="mt-1 space-y-1">
        <div v-for="item in maskedStructure" :key="item.name" class="text-xs">
          <span class="font-mono text-text-secondary">{{ item.name }}</span>
          <span v-if="item.line" class="ml-1 font-mono text-text-tertiary">行 {{ item.line }}</span>
          <span class="mx-1">·</span>
          <span class="text-text-tertiary">{{ item.preview }}</span>
        </div>
      </div>
      <p class="mt-2 text-xs text-text-tertiary">
        应用只覆盖本次解析明确产出的字段；其它现有草稿字段保持不变，最终仍需普通保存与检查。
      </p>
    </div>

    <p v-if="hasDraft && !result" class="mt-2 text-xs text-text-tertiary">
      存在未应用的 `.ovpn` 原文，保存与检查前必须应用或取消。
    </p>
  </div>
</template>
