<!-- OpenVPNImportPanel.vue：Build32 Step 19 的 `.ovpn` 粘贴解析面板。
  只读解析：解析结果与原文都只存在内存，永不写入 localStorage；应用时只覆盖 parser 明确产出的字段，
  未产出的现有草稿字段不会被空响应静默删除；关闭面板或切换协议时立即丢弃原文与未应用草稿。
-->
<script setup lang="ts">
import { computed, onBeforeUnmount, ref, useId, watch } from 'vue'
import { Alert, Button, Input } from 'ant-design-vue'
import { parseOpenVPN } from '@/api/node'
import type { OpenVPNParseDiagnostic, OpenVPNParseResult } from '@/api/node'
import { ApiError } from '@/api/request'

const props = withDefaults(defineProps<{
  open: boolean
  applyVersion?: number
  sensitivePaths: string[]
}>(), { applyVersion: 0 })

const emit = defineEmits<{
  apply: [payload: { host: string; port: number; protocol_json: Record<string, unknown>; selectors: Record<string, string> }]
  'draft-dirty-change': [payload: { path: string; dirty: boolean }]
}>()

// 页面级阻断使用的稳定路径；与 JSON／自定义值草稿共用同一套阻断集合。
const DRAFT_PATH = 'openvpn-import'
const panel = ref<HTMLElement | null>(null)
const sourceId = useId()

const text = ref('')
const parsing = ref(false)
const result = ref<OpenVPNParseResult | null>(null)
const diagnostics = ref<OpenVPNParseDiagnostic[]>([])
const error = ref('')
const responseCurrent = ref(false)
const stale = ref(false)

let requestVersion = 0
const hasText = computed(() => text.value.trim() !== '')
const hasDraft = computed(() => hasText.value || result.value !== null)
const canApply = computed(() => props.open && responseCurrent.value && result.value !== null && !parsing.value)

// 原文编辑立即使旧请求与行号失效；保留未处理结果，以免清空原文绕过页面预检。
watch(text, () => {
  stale.value ||= parsing.value || responseCurrent.value || result.value !== null
  requestVersion += 1
  parsing.value = false
  responseCurrent.value = false
  diagnostics.value = []
  error.value = ''
}, { flush: 'sync' })

// textarea 的 API 值统一使用 LF；选区偏移按 JS 字符串代码单元计数，不按 UTF-8 字节或字符数。
const sourceValue = computed(() => text.value.replace(/\r\n?/g, '\n'))
const lineStarts = computed(() => {
  const starts = [0]
  const value = sourceValue.value
  for (let index = 0; index < value.length; index += 1) {
    if (value[index] === '\n') starts.push(index + 1)
  }
  return starts
})

function lineRange(line?: number): { start: number; end: number } | undefined {
  if (!props.open || !responseCurrent.value || parsing.value || typeof line !== 'number'
      || !Number.isInteger(line) || line < 1 || line > lineStarts.value.length) return
  return {
    start: lineStarts.value[line - 1],
    end: line < lineStarts.value.length ? lineStarts.value[line] - 1 : sourceValue.value.length,
  }
}

function locateLine(line?: number) {
  const range = lineRange(line)
  const textarea = panel.value?.querySelector<HTMLTextAreaElement>('textarea[data-field-path="openvpn-import-text"]')
  if (!range || !textarea) return
  textarea.focus({ preventScroll: true })
  textarea.setSelectionRange(range.start, range.end, 'backward')

  // 逻辑行可能折成多行，不能用行号乘行高；仅在定位时测量当前排版，不保留原文副本。
  const style = getComputedStyle(textarea)
  const measure = document.createElement('div')
  measure.dataset.openvpnLineMeasure = ''
  measure.setAttribute('aria-hidden', 'true')
  Object.assign(measure.style, {
    position: 'fixed', left: '0', top: '0', visibility: 'hidden', pointerEvents: 'none',
    boxSizing: 'border-box', width: `${textarea.clientWidth}px`,
  })
  for (const property of [
    'font-family', 'font-size', 'font-weight', 'font-style', 'font-variant', 'font-stretch',
    'line-height', 'letter-spacing', 'word-spacing', 'tab-size', 'text-indent', 'text-transform',
    'text-align', 'direction', 'white-space', 'overflow-wrap', 'word-break',
    'padding-top', 'padding-right', 'padding-bottom', 'padding-left',
  ]) measure.style.setProperty(property, style.getPropertyValue(property))
  measure.append(document.createTextNode(textarea.value.slice(0, range.start)))
  const marker = document.createElement('span')
  marker.textContent = '\u200b'
  measure.append(marker)
  document.body.append(measure)
  try {
    const rect = marker.getBoundingClientRect()
    const top = rect.top - measure.getBoundingClientRect().top
    textarea.scrollTop = Math.max(0, top - (textarea.clientHeight - rect.height) / 2)
    textarea.scrollLeft = 0
  } finally {
    measure.remove()
  }
  textarea.scrollIntoView?.({ block: 'center', behavior: 'auto' })
}

watch(hasDraft, (dirty) => emit('draft-dirty-change', { path: DRAFT_PATH, dirty }))
// 关闭面板即丢弃原文与未应用结果，阻断随之解除。
watch(() => props.open, (open) => { if (!open) reset() })
// 父页面完成预检并真正应用后才清空；取消预检时保留原文和解析结果。
watch(() => props.applyVersion, () => reset())

function reset() {
  requestVersion += 1
  parsing.value = false
  text.value = ''
  result.value = null
  diagnostics.value = []
  error.value = ''
  responseCurrent.value = false
  stale.value = false
  emit('draft-dirty-change', { path: DRAFT_PATH, dirty: false })
}
onBeforeUnmount(reset)

function isSensitive(name: string): boolean {
  return props.sensitivePaths.some((path) => path === name || path.startsWith(`${name}.`) || path.startsWith(`${name}[`))
}

// 脱敏结构：只展示字段名、来源行号与安全摘要，敏感字段不显示取值。
const maskedStructure = computed(() => {
  const parsed = result.value
  if (!parsed || !responseCurrent.value) return [] as { name: string; line?: number; preview: string; sensitive: boolean }[]
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
  if (!props.open || !hasText.value) {
    error.value = '请先粘贴 `.ovpn` 内容'
    return
  }
  parsing.value = true
  const version = ++requestVersion
  error.value = ''
  result.value = null
  diagnostics.value = []
  responseCurrent.value = false
  stale.value = false
  try {
    const parsed = await parseOpenVPN(sourceValue.value)
    if (version !== requestVersion) return
    responseCurrent.value = true
    result.value = parsed
    diagnostics.value = parsed.diagnostics ?? []
  } catch (err) {
    if (version !== requestVersion) return
    responseCurrent.value = true
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
    if (version === requestVersion) parsing.value = false
  }
}

function apply() {
  const parsed = result.value
  if (!parsed || !canApply.value) return
  emit('apply', {
    host: parsed.host ?? '',
    port: parsed.port ?? 0,
    protocol_json: parsed.protocol_json ?? {},
    selectors: parsed.selectors ?? {},
  })
}
</script>

<template>
  <div ref="panel" :data-field-path="DRAFT_PATH" class="openvpn-import-panel rounded-lg border p-3">
    <div class="mb-2">
      <div class="text-sm font-medium text-text">粘贴 `.ovpn` 解析</div>
      <div class="text-xs text-text-tertiary">
        只读解析：不读取外部文件、不执行脚本／hook／include、不落库。上限 256 KiB。
      </div>
    </div>

    <Input.TextArea
      :id="sourceId"
      v-model:value="text"
      aria-label="OpenVPN 原文"
      data-field-path="openvpn-import-text"
      :rows="6"
      :disabled="!open"
      placeholder="粘贴 .ovpn 文本（含 <ca>／<cert>／<key> 等内嵌块）"
      class="font-mono"
    />

    <div class="mt-2 flex items-center gap-2">
      <Button size="small" :loading="parsing" :disabled="!open || !hasText" @click="run">解析</Button>
      <Button size="small" type="primary" :disabled="!canApply" @click="apply">应用解析结果</Button>
      <Button size="small" :disabled="!hasDraft && !parsing" @click="reset">取消</Button>
    </div>

    <Alert v-if="error" type="error" show-icon class="mt-2" :message="error" />
    <Alert v-if="stale" type="warning" show-icon class="mt-2" message="原文已更改，请重新解析或取消；旧解析结果不可应用或定位。" />
    <div v-if="diagnostics.length" class="mt-2 space-y-1">
      <div v-for="(diagnostic, index) in diagnostics" :key="index" class="text-xs">
        <span class="font-mono text-text-tertiary">{{ diagnostic.code }}</span>
        <Button v-if="lineRange(diagnostic.line)" type="link" html-type="button" size="small"
          class="openvpn-source-link ml-1 font-mono" :aria-label="`定位原文第 ${diagnostic.line} 行`"
          :aria-controls="sourceId" @click="locateLine(diagnostic.line)">行 {{ diagnostic.line }}</Button>
        <span v-else-if="diagnostic.line" class="ml-1 font-mono text-text-tertiary">行 {{ diagnostic.line }}</span>
        <span class="mx-1">·</span>
        <span :class="diagnostic.severity === 'error' ? 'text-danger' : 'text-text'">{{ diagnostic.message }}</span>
      </div>
    </div>

    <div v-if="result && responseCurrent" class="mt-3 rounded-md border p-2">
      <div class="text-xs font-medium text-text">已解析结构（脱敏）</div>
      <div class="mt-1 space-y-1">
        <div v-for="item in maskedStructure" :key="item.name" class="text-xs">
          <span class="font-mono text-text-secondary">{{ item.name }}</span>
          <Button v-if="lineRange(item.line)" type="link" html-type="button" size="small"
            class="openvpn-source-link ml-1 font-mono" :aria-label="`定位原文第 ${item.line} 行`"
            :aria-controls="sourceId" @click="locateLine(item.line)">行 {{ item.line }}</Button>
          <span v-else-if="item.line" class="ml-1 font-mono text-text-tertiary">行 {{ item.line }}</span>
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

<style scoped>
.openvpn-source-link {
  height: auto;
  padding: 0;
  vertical-align: baseline;
  font-size: inherit;
}
</style>
