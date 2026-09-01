<!-- PlatformHeadersEditor.vue：平台附加响应头结构化/JSON 双模式编辑器（R25-02） -->
<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Alert, Button, Input, Switch } from 'ant-design-vue'

const props = withDefaults(defineProps<{
  modelValue?: Record<string, string>
  advancedMode?: boolean
}>(), {
  modelValue: () => ({}),
  advancedMode: false,
})

const emit = defineEmits<{
  'validity-change': [valid: boolean]
}>()

const mode = ref<'structured' | 'json'>('structured')
const jsonText = ref('')
const jsonError = ref('')
const formError = ref('')

const filenameEnabled = ref(false)
const filename = ref('')
const profileUpdateEnabled = ref(false)
const profileUpdateValue = ref('')
const profileWebEnabled = ref(false)
const profileWebValue = ref('')
const userinfoEnabled = ref(false)
const userinfoValue = ref('')
const customHeaders = ref<{ key: string; value: string }[]>([])

const knownHeaders = ['Content-Disposition', 'profile-update-interval', 'profile-web-page-url', 'subscription-userinfo']
const managedHeaders = ['profile-update-interval', 'profile-web-page-url', 'subscription-userinfo']
const ctrlRe = /[\x00-\x1f\x7f]/
const headerNameRe = /^[!#$%&'*+\-.^_`|~0-9A-Za-z]+$/

function findKeyCI(map: Record<string, string>, key: string): string | undefined {
  return Object.keys(map).find((k) => k.toLowerCase() === key.toLowerCase())
}

function getCI(map: Record<string, string>, key: string): string {
  const found = findKeyCI(map, key)
  return found ? map[found] : ''
}

function parseContentDispositionFilename(value: string): string | null {
  const star = /filename\*\s*=\s*(?:UTF-8|utf-8)''([^;]+)/i.exec(value)
  if (star) {
    try { return decodeURIComponent(star[1].trim()) } catch { return star[1].trim() }
  }
  const plain = /filename\s*=\s*"([^"]*)"/i.exec(value) || /filename\s*=\s*([^;]+)/i.exec(value)
  return plain ? plain[1].trim() : null
}

function buildContentDisposition(name: string): string {
  return `attachment; filename*=UTF-8''${encodeURIComponent(name)}`
}

function isKnownHeader(key: string): boolean {
  const lower = key.trim().toLowerCase()
  return knownHeaders.some((k) => k.toLowerCase() === lower)
}

function isManagedHeader(key: string): boolean {
  const lower = key.trim().toLowerCase()
  return managedHeaders.some((k) => k.toLowerCase() === lower)
}

function syncFromModel() {
  const map = props.modelValue ?? {}
  const cd = findKeyCI(map, 'Content-Disposition')
  filenameEnabled.value = !!cd
  filename.value = cd ? parseContentDispositionFilename(map[cd]) ?? map[cd] : ''

  const pu = findKeyCI(map, 'profile-update-interval')
  profileUpdateEnabled.value = !!pu
  profileUpdateValue.value = pu ? map[pu] : ''
  const pw = findKeyCI(map, 'profile-web-page-url')
  profileWebEnabled.value = !!pw
  profileWebValue.value = pw ? map[pw] : ''
  const ui = findKeyCI(map, 'subscription-userinfo')
  userinfoEnabled.value = !!ui
  userinfoValue.value = ui ? map[ui] : ''

  customHeaders.value = Object.entries(map)
    .filter(([key]) => !isKnownHeader(key))
    .map(([key, value]) => ({ key, value }))
  if (!customHeaders.value.length) customHeaders.value = [{ key: '', value: '' }]

  if (mode.value === 'json') {
    jsonText.value = JSON.stringify(editableMap(), null, 2)
  }
}

function editableMap(): Record<string, string> {
  const map = props.modelValue ?? {}
  const out: Record<string, string> = {}
  for (const [k, v] of Object.entries(map)) {
    if (!isManagedHeader(k)) out[k] = v
  }
  return out
}

watch(() => props.modelValue, syncFromModel, { immediate: true, deep: true })

function currentValue(): Record<string, string> {
  const map: Record<string, string> = {}
  if (filenameEnabled.value && filename.value.trim()) {
    map['Content-Disposition'] = buildContentDisposition(filename.value.trim())
  }
  if (profileUpdateEnabled.value && profileUpdateValue.value.trim()) {
    map['profile-update-interval'] = profileUpdateValue.value.trim()
  }
  if (profileWebEnabled.value && profileWebValue.value.trim()) {
    map['profile-web-page-url'] = profileWebValue.value.trim()
  }
  if (userinfoEnabled.value && userinfoValue.value.trim()) {
    map['subscription-userinfo'] = userinfoValue.value.trim()
  }
  for (const row of customHeaders.value) {
    if (!row.key.trim()) continue
    map[row.key.trim()] = row.value
  }
  return map
}

function validateStructured(): string {
  if (filenameEnabled.value) {
    const name = filename.value.trim()
    if (!name) return '手动覆盖下载文件名不能为空'
    if (ctrlRe.test(name)) return '下载文件名含控制字符'
    if (/[\\/]/.test(name) || name.includes('..')) return '下载文件名不能包含路径分隔符或路径穿越片段'
    if (name.includes('"')) return '下载文件名不能包含双引号'
  }
  for (const row of customHeaders.value) {
    if (row.key.trim() && isKnownHeader(row.key)) {
      return `请使用上方专用字段编辑 ${row.key.trim()}`
    }
  }
  const seen: Record<string, string> = {}
  const compact = currentValue()
  for (const [k, v] of Object.entries(compact)) {
    const lower = k.toLowerCase()
    if (seen[lower]) return `响应头 ${k} 与 ${seen[lower]} 大小写语义重复`
    seen[lower] = k
    if (!headerNameRe.test(k)) return `响应头键 ${k} 不符合 HTTP 头名规范`
    if (ctrlRe.test(k) || ctrlRe.test(v)) return `响应头 ${k} 含控制字符`
    if (lower === 'profile-update-interval' && !/^\d+$/.test(v.trim())) return '更新周期必须是非负整数小时'
    if (lower === 'profile-web-page-url' && v.trim() !== '{frontend_url}') {
      try {
        const u = new URL(v.trim())
        if (!['http:', 'https:'].includes(u.protocol) || !u.host) return '主页须为 http/https 地址或 {frontend_url}'
      } catch {
        return '主页须为 http/https 地址或 {frontend_url}'
      }
    }
    if (lower === 'subscription-userinfo') {
      const valid = v.split(';').every((part) => {
        const [name, amount, ...rest] = part.trim().split('=')
        return !!name && rest.length === 0 && /^\d+$/.test(amount ?? '')
      })
      if (!valid) return '流量信息须为 key=非负整数; ...'
    }
  }
  return ''
}

function validateJson(parse = false): { ok: boolean; error: string; value?: Record<string, string> } {
  let parsed: unknown
  try {
    parsed = JSON.parse(jsonText.value || '{}')
  } catch {
    return { ok: false, error: 'JSON 格式错误' }
  }
  if (parsed === null || typeof parsed !== 'object' || Array.isArray(parsed)) {
    return { ok: false, error: 'JSON 根节点必须是对象' }
  }
  const obj = parsed as Record<string, unknown>
  const out: Record<string, string> = {}
  const seen: Record<string, string> = {}
  for (const [k, v] of Object.entries(obj)) {
    const lower = k.toLowerCase()
    if (seen[lower]) return { ok: false, error: `存在大小写语义重复的头名：${k} / ${seen[lower]}` }
    seen[lower] = k
    if (typeof v !== 'string') return { ok: false, error: `响应头 ${k} 的值必须是字符串` }
    if (!headerNameRe.test(k)) return { ok: false, error: `响应头键 ${k} 不符合 HTTP 头名规范` }
    if (ctrlRe.test(k) || ctrlRe.test(v)) return { ok: false, error: `响应头 ${k} 含控制字符` }
    if (lower === 'content-disposition') {
      const name = parseContentDispositionFilename(v)
      if (!name) return { ok: false, error: 'Content-Disposition 格式无效或缺少文件名' }
      if (/[\\/]/.test(name) || name.includes('..') || name.includes('"')) {
        return { ok: false, error: 'Content-Disposition 文件名包含非法字符' }
      }
    }
    if (props.advancedMode && isManagedHeader(k)) {
      return { ok: false, error: `高级模式下 ${k} 由系统管理，不可编辑` }
    }
    out[k] = v
  }
  if (!parse) return { ok: true, error: '', value: out }
  return { ok: true, error: '', value: out }
}

function emitValidity() {
  const valid = mode.value === 'json' ? validateJson().ok : validateStructured() === ''
  emit('validity-change', valid)
}

function setAdvanced(next: boolean) {
  if (!next) {
    // 从 JSON 切回结构化：先校验 JSON
    const result = validateJson()
    if (!result.ok) {
      jsonError.value = result.error
      emitValidity()
      return
    }
    jsonError.value = ''
    // 将当前 JSON 应用到结构化草稿；高级模式下补回系统接管字段的原始只读值。
    const map: Record<string, string> = { ...(result.value ?? {}) }
    if (props.advancedMode) {
      for (const key of managedHeaders) {
        const found = findKeyCI(props.modelValue ?? {}, key)
        if (found) map[found] = (props.modelValue ?? {})[found]
      }
    }
    const cd = findKeyCI(map, 'Content-Disposition')
    filenameEnabled.value = !!cd
    filename.value = cd ? parseContentDispositionFilename(map[cd]) ?? map[cd] : ''
    const pu = findKeyCI(map, 'profile-update-interval')
    profileUpdateEnabled.value = !!pu
    profileUpdateValue.value = pu ? map[pu] : ''
    const pw = findKeyCI(map, 'profile-web-page-url')
    profileWebEnabled.value = !!pw
    profileWebValue.value = pw ? map[pw] : ''
    const ui = findKeyCI(map, 'subscription-userinfo')
    userinfoEnabled.value = !!ui
    userinfoValue.value = ui ? map[ui] : ''
    customHeaders.value = Object.entries(map).filter(([k]) => !isKnownHeader(k)).map(([key, value]) => ({ key, value }))
    if (!customHeaders.value.length) customHeaders.value = [{ key: '', value: '' }]
  } else {
    jsonError.value = ''
    jsonText.value = JSON.stringify(props.advancedMode ? editableMap() : currentValue(), null, 2)
  }
  mode.value = next ? 'json' : 'structured'
  emitValidity()
}

function onJsonInput(value: string) {
  jsonText.value = value
  const result = validateJson()
  jsonError.value = result.ok ? '' : result.error
  emitValidity()
}

function addCustom() {
  customHeaders.value.push({ key: '', value: '' })
}

function removeCustom(index: number) {
  customHeaders.value.splice(index, 1)
  if (!customHeaders.value.length) customHeaders.value.push({ key: '', value: '' })
}

function getValue(): Record<string, string> | null {
  if (mode.value === 'json') {
    const result = validateJson()
    if (!result.ok) {
      jsonError.value = result.error
      emitValidity()
      return null
    }
    const merged: Record<string, string> = {}
    // 高级模式下仅保留系统接管字段的原始平台值，其余以 JSON 内容为准。
    if (props.advancedMode) {
      for (const key of managedHeaders) {
        const found = findKeyCI(props.modelValue ?? {}, key)
        if (found) merged[found] = (props.modelValue ?? {})[found]
      }
    }
    for (const [k, v] of Object.entries(result.value ?? {})) {
      merged[k] = v
    }
    return merged
  }
  const err = validateStructured()
  if (err) {
    formError.value = err
    emitValidity()
    return null
  }
  formError.value = ''
  const out = currentValue()
  // 高级模式下若结构化未编辑三个系统字段，也必须保留原值。
  if (props.advancedMode) {
    for (const existing of Object.keys(out)) {
      if (isManagedHeader(existing)) delete out[existing]
    }
    for (const key of managedHeaders) {
      const found = findKeyCI(props.modelValue ?? {}, key)
      if (found) out[found] = (props.modelValue ?? {})[found]
    }
  }
  return out
}

function onStructuredChange() {
  formError.value = ''
  emitValidity()
}

defineExpose({ getValue })

const customRows = computed(() => customHeaders.value)
</script>

<template>
  <div class="border rounded p-3 space-y-3">
    <div class="flex items-center justify-between gap-3">
      <div class="text-sm font-medium">附加响应头</div>
      <label class="flex items-center gap-2 text-xs text-text-secondary whitespace-nowrap">
        <Switch :checked="mode === 'json'" size="small" @change="(v: any) => setAdvanced(Boolean(v))" />
        <span>{{ mode === 'json' ? '高级 JSON' : '结构化编辑' }}</span>
      </label>
    </div>

    <Alert v-if="formError" type="error" show-icon class="mb-2" :message="formError" />
    <Alert v-if="jsonError" type="error" show-icon class="mb-2" :message="jsonError" />

    <!-- JSON 模式 -->
    <template v-if="mode === 'json'">
      <div v-if="props.advancedMode" class="space-y-2">
        <div v-for="key in managedHeaders" :key="key" class="text-xs text-text-secondary">
          <span class="font-medium">{{ key }}</span>
          <span class="ml-1 text-text-tertiary">（高级模式下由系统管理，只读保留）</span>
          <div class="font-mono bg-surface-subtle rounded px-2 py-1">{{ getCI(props.modelValue ?? {}, key) || '—' }}</div>
        </div>
      </div>
      <Input.TextArea :value="jsonText" :rows="8" class="font-mono" placeholder='{"X-Custom":"value"}' @input="(e: any) => onJsonInput(e.target.value)" />
    </template>

    <!-- 结构化模式 -->
    <template v-else>
      <div class="space-y-3">
        <div>
          <div class="text-sm font-medium mb-2">手动覆盖下载文件名</div>
          <div class="flex items-center gap-2">
            <Switch :checked="filenameEnabled" @change="(v: any) => { filenameEnabled = Boolean(v); onStructuredChange() }" />
            <Input v-model:value="filename" class="flex-1" placeholder="完整文件名，如 Luneflare.yaml；留空恢复系统默认" :disabled="!filenameEnabled" @input="onStructuredChange" />
          </div>
        </div>

        <div>
          <div class="text-sm font-medium mb-2">客户端兼容字段</div>
          <div class="space-y-2">
            <div class="flex items-center gap-2">
              <Switch :checked="profileUpdateEnabled" :disabled="props.advancedMode" @change="(v: any) => { profileUpdateEnabled = Boolean(v); onStructuredChange() }" />
              <span class="text-sm w-44">自动更新周期（小时）</span>
              <Input v-model:value="profileUpdateValue" class="flex-1" placeholder="0 = 不自动更新" :disabled="props.advancedMode || !profileUpdateEnabled" @input="onStructuredChange" />
            </div>
            <div class="flex items-center gap-2">
              <Switch :checked="profileWebEnabled" :disabled="props.advancedMode" @change="(v: any) => { profileWebEnabled = Boolean(v); onStructuredChange() }" />
              <span class="text-sm w-44">订阅主页</span>
              <Input v-model:value="profileWebValue" class="flex-1" placeholder="https://… 或 {frontend_url}" :disabled="props.advancedMode || !profileWebEnabled" @input="onStructuredChange" />
            </div>
            <div class="flex items-center gap-2">
              <Switch :checked="userinfoEnabled" :disabled="props.advancedMode" @change="(v: any) => { userinfoEnabled = Boolean(v); onStructuredChange() }" />
              <span class="text-sm w-44">订阅流量信息</span>
              <Input v-model:value="userinfoValue" class="flex-1" placeholder="upload=0; download=0; total=0; expire=…" :disabled="props.advancedMode || !userinfoEnabled" @input="onStructuredChange" />
            </div>
            <div v-if="props.advancedMode" class="text-xs text-text-tertiary">
              高级模式下上述三个字段由系统固定接管：更新周期固定为 6 小时，主页使用当前系统前端地址，流量信息按下载用户动态生成。
            </div>
          </div>
        </div>

        <div>
          <div class="text-sm font-medium mb-2">普通自定义响应头</div>
          <div v-for="(row, i) in customRows" :key="i" class="flex gap-2 mb-2 items-start">
            <Input v-model:value="row.key" class="w-40" placeholder="头名，如 X-Custom" @input="onStructuredChange" />
            <Input v-model:value="row.value" class="flex-1" placeholder="头值，如 {frontend_url}" @input="onStructuredChange" />
            <Button size="small" danger :disabled="customRows.length <= 1" @click="removeCustom(i)">删除</Button>
          </div>
          <Button size="small" @click="addCustom">添加响应头</Button>
        </div>
      </div>
    </template>
  </div>
</template>
