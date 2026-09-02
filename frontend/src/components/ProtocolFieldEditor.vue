<!-- ProtocolFieldEditor.vue：协议字段递归编辑器；对象默认结构化，保留对象级高级 JSON。 -->
<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { Button, Input, InputNumber, Select, Switch } from 'ant-design-vue'
import EditableCombobox from '@/components/EditableCombobox.vue'
import type { FieldSchema } from '@/api/node'

const props = withDefaults(defineProps<{
  field: FieldSchema
  modelValue?: unknown
  sensitivePaths?: string[]
  credentialState?: 'unset' | 'saved' | 'replacing' | 'cleared'
  path?: string
}>(), {
  modelValue: undefined,
  sensitivePaths: () => [],
  credentialState: undefined,
  path: '',
})

const emit = defineEmits<{
  'update:modelValue': [value: unknown]
  'validity-change': [payload: { path: string; valid: boolean }]
  'json-dirty-change': [payload: { path: string; dirty: boolean }]
}>()

const fieldPath = computed(() => props.path || props.field.name)
const advanced = ref(false)
const jsonText = ref('')
const jsonError = ref('')
const jsonDirty = ref(false)
const mapErrors = reactive<Record<string, string>>({})

const objectValue = computed<Record<string, unknown>>(() => {
  if (props.modelValue && typeof props.modelValue === 'object' && !Array.isArray(props.modelValue)) {
    return props.modelValue as Record<string, unknown>
  }
  return {}
})

const listValue = computed<unknown[]>(() => Array.isArray(props.modelValue) ? props.modelValue : [])
const knownNames = computed(() => new Set((props.field.properties ?? []).map((item) => item.name)))
const unknownCount = computed(() => Object.keys(objectValue.value).filter((key) => !knownNames.value.has(key)).length)
const mapEntries = computed(() => Object.entries(objectValue.value))
const sensitive = computed(() => props.field.type === 'password' || props.sensitivePaths.includes(fieldPath.value))
const shownCredentialState = computed(() => {
  if (props.credentialState) return props.credentialState
  return props.sensitivePaths.includes(fieldPath.value) ? 'saved' : 'unset'
})

watch(() => props.modelValue, (value) => {
  jsonDirty.value = false
  emitJsonDirty(false)
  if (!advanced.value) jsonText.value = JSON.stringify(value ?? emptyObjectValue(), null, 2)
}, { immediate: true, deep: true })

function emptyObjectValue(): Record<string, unknown> | unknown[] {
  return props.field.object_kind === 'list' ? [] : {}
}

function update(value: unknown) {
  emit('update:modelValue', value)
}

function forwardValidity(payload: { path: string; valid: boolean }) {
  emit('validity-change', payload)
}

function forwardJsonDirty(payload: { path: string; dirty: boolean }) {
  emit('json-dirty-change', payload)
}

function emitJsonDirty(dirty: boolean) {
  emit('json-dirty-change', { path: fieldPath.value, dirty })
}

function setAdvanced(next: boolean) {
  if (!next && jsonError.value) return
  if (next) {
    jsonText.value = JSON.stringify(props.modelValue ?? emptyObjectValue(), null, 2)
    jsonDirty.value = false
    emitJsonDirty(false)
    advanced.value = true
    return
  }
  // 离开高级 JSON 时放弃未应用草稿，恢复结构化编辑当前有效值。
  jsonDirty.value = false
  emitJsonDirty(false)
  jsonError.value = ''
  jsonText.value = JSON.stringify(props.modelValue ?? emptyObjectValue(), null, 2)
  forwardValidity({ path: fieldPath.value, valid: true })
  advanced.value = false
}

function parseJSONText(): { parsed: unknown; error: string } {
  try {
    const parsed = JSON.parse(jsonText.value || (props.field.object_kind === 'list' ? '[]' : '{}'))
    const validShape = props.field.object_kind === 'list'
      ? Array.isArray(parsed)
      : parsed !== null && typeof parsed === 'object' && !Array.isArray(parsed)
    if (!validShape) throw new Error('shape')
    return { parsed, error: '' }
  } catch {
    return { parsed: null, error: props.field.object_kind === 'list' ? '请输入 JSON 对象数组' : '请输入 JSON 对象' }
  }
}

function updateJSON(value: string) {
  jsonText.value = value
  jsonDirty.value = true
  emitJsonDirty(true)
  const result = parseJSONText()
  jsonError.value = result.error
  forwardValidity({ path: fieldPath.value, valid: result.error === '' })
}

function applyJSON() {
  const result = parseJSONText()
  jsonError.value = result.error
  if (result.error) {
    forwardValidity({ path: fieldPath.value, valid: false })
    return
  }
  jsonDirty.value = false
  emitJsonDirty(false)
  forwardValidity({ path: fieldPath.value, valid: true })
  update(result.parsed)
}

function discardJSON() {
  jsonText.value = JSON.stringify(props.modelValue ?? emptyObjectValue(), null, 2)
  jsonDirty.value = false
  emitJsonDirty(false)
  jsonError.value = ''
  forwardValidity({ path: fieldPath.value, valid: true })
}

function childValue(name: string): unknown {
  return objectValue.value[name]
}

function setChild(name: string, value: unknown) {
  update({ ...objectValue.value, [name]: value })
}

function addMapEntry() {
  let index = mapEntries.value.length + 1
  let key = `参数${index}`
  while (key in objectValue.value) {
    index += 1
    key = `参数${index}`
  }
  update({ ...objectValue.value, [key]: '' })
}

function renameMapKey(oldKey: string, newKey: string) {
  if (!newKey || (newKey !== oldKey && newKey in objectValue.value)) return
  const next: Record<string, unknown> = {}
  for (const [key, value] of mapEntries.value) next[key === oldKey ? newKey : key] = value
  update(next)
}

function setMapValue(key: string, value: unknown) {
  update({ ...objectValue.value, [key]: value })
}

function setComplexMapValue(key: string, value: string) {
  try {
    const parsed = JSON.parse(value)
    delete mapErrors[key]
    forwardValidity({ path: `${fieldPath.value}.${key}`, valid: true })
    setMapValue(key, parsed)
  } catch {
    mapErrors[key] = '复杂值 JSON 格式错误'
    forwardValidity({ path: `${fieldPath.value}.${key}`, valid: false })
  }
}

function removeMapEntry(key: string) {
  const next = { ...objectValue.value }
  delete next[key]
  delete mapErrors[key]
  forwardValidity({ path: `${fieldPath.value}.${key}`, valid: true })
  update(next)
}

function addListItem() {
  update([...listValue.value, {}])
}

function setListChild(index: number, name: string, value: unknown) {
  const next = [...listValue.value]
  const current = next[index]
  const object = current && typeof current === 'object' && !Array.isArray(current)
    ? current as Record<string, unknown>
    : {}
  next[index] = { ...object, [name]: value }
  update(next)
}

function removeListItem(index: number) {
  const next = [...listValue.value]
  next.splice(index, 1)
  update(next)
}

function textListValue(value: unknown): string {
  return Array.isArray(value) ? value.join(', ') : String(value ?? '')
}

function isComplex(value: unknown): boolean {
  return value !== null && typeof value === 'object'
}
</script>

<template>
  <div v-if="field.type === 'object'" class="protocol-object-field rounded-lg border p-3">
    <div class="flex items-center justify-between gap-3 mb-3">
      <div>
        <div class="text-sm font-medium text-text">{{ field.label }}</div>
        <div v-if="field.help" class="text-xs text-text-tertiary mt-1">{{ field.help }}</div>
      </div>
      <label class="flex items-center gap-2 text-xs text-text-secondary whitespace-nowrap">
        <Switch :checked="advanced" size="small" @change="(value: any) => setAdvanced(Boolean(value))" />
        <span>{{ advanced ? '高级 JSON' : '结构化编辑' }}</span>
      </label>
    </div>

    <div v-if="advanced">
      <Input.TextArea :value="jsonText" :rows="6" class="font-mono" @input="(event: any) => updateJSON(event.target.value)" />
      <div v-if="jsonError" class="text-xs text-red-500 mt-1">{{ jsonError }}</div>
      <div class="mt-2 flex items-center gap-2">
        <Button size="small" type="primary" :disabled="!!jsonError" @click="applyJSON">应用</Button>
        <Button size="small" @click="discardJSON">放弃</Button>
        <span v-if="jsonDirty" class="text-xs text-text-tertiary">JSON 草稿未应用</span>
      </div>
    </div>

    <template v-else-if="field.object_kind === 'map'">
      <div v-if="mapEntries.length" class="space-y-2">
        <div v-for="([key, value]) in mapEntries" :key="key" class="grid grid-cols-1 md:grid-cols-[minmax(140px,0.7fr)_minmax(180px,1fr)_auto] gap-2 items-start">
          <Input :value="key" aria-label="参数名" @change="(event: any) => renameMapKey(key, event.target.value)" />
          <Switch v-if="typeof value === 'boolean'" :checked="value" @change="(next: any) => setMapValue(key, Boolean(next))" />
          <InputNumber v-else-if="typeof value === 'number'" :value="value" class="w-full" @change="(next: any) => setMapValue(key, next ?? 0)" />
          <Input.TextArea v-else-if="isComplex(value)" :value="JSON.stringify(value)" :rows="2" @blur="(event: any) => setComplexMapValue(key, event.target.value)" />
          <Input v-else :value="String(value ?? '')" @change="(event: any) => setMapValue(key, event.target.value)" />
          <Button danger @click="removeMapEntry(key)">删除</Button>
          <div v-if="mapErrors[key]" class="md:col-start-2 text-xs text-red-500">{{ mapErrors[key] }}</div>
        </div>
      </div>
      <div v-else class="text-xs text-text-tertiary mb-2">暂无参数</div>
      <Button size="small" @click="addMapEntry">新增参数</Button>
    </template>

    <template v-else-if="field.object_kind === 'list'">
      <div v-if="listValue.length" class="space-y-3">
        <div v-for="(item, index) in listValue" :key="index" class="rounded-lg border p-3">
          <div class="flex items-center justify-between mb-3">
            <span class="text-sm font-medium">第 {{ index + 1 }} 项</span>
            <Button size="small" danger @click="removeListItem(index)">删除</Button>
          </div>
          <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
            <ProtocolFieldEditor
              v-for="property in field.properties ?? []"
              :key="property.name"
              :field="property"
              :model-value="item && typeof item === 'object' && !Array.isArray(item) ? (item as Record<string, unknown>)[property.name] : undefined"
              :sensitive-paths="sensitivePaths"
              :path="`${fieldPath}[${index}].${property.name}`"
              @update:model-value="(value: unknown) => setListChild(index, property.name, value)"
              @validity-change="forwardValidity"
              @json-dirty-change="forwardJsonDirty"
            />
          </div>
        </div>
      </div>
      <div v-else class="text-xs text-text-tertiary mb-2">暂无条目</div>
      <Button size="small" @click="addListItem">新增条目</Button>
    </template>

    <template v-else>
      <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
        <ProtocolFieldEditor
          v-for="property in field.properties ?? []"
          :key="property.name"
          :field="property"
          :model-value="childValue(property.name)"
          :sensitive-paths="sensitivePaths"
          :path="`${fieldPath}.${property.name}`"
          :class="property.type === 'object' ? 'md:col-span-2' : ''"
          @update:model-value="(value: unknown) => setChild(property.name, value)"
          @validity-change="forwardValidity"
          @json-dirty-change="forwardJsonDirty"
        />
      </div>
      <div v-if="unknownCount" class="text-xs text-text-tertiary mt-3">
        已保留 {{ unknownCount }} 个未识别参数，可在高级 JSON 中查看和编辑。
      </div>
    </template>
  </div>

  <label v-else-if="field.type === 'bool'" class="protocol-switch-field flex items-center justify-between gap-3 rounded-md border px-3 py-2 text-sm text-text-secondary">
    <span>{{ field.label }}<span v-if="field.required" class="text-red-500"> *</span></span>
    <Switch :checked="Boolean(modelValue ?? field.default ?? false)" @change="(value: any) => update(Boolean(value))" />
  </label>

  <div v-else class="protocol-scalar-field">
    <label class="text-sm text-text-secondary">{{ field.label }}<span v-if="field.required" class="text-red-500"> *</span></label>
    <Input.Password v-if="sensitive" :value="String(modelValue ?? '')" :placeholder="shownCredentialState === 'saved' ? '已保存（留空保留）' : '未配置'" @change="(event: any) => update(event.target.value)" />
    <div v-if="sensitive && shownCredentialState === 'saved'" class="text-xs text-text-tertiary mt-1">已保存（留空保留）</div>
    <InputNumber v-else-if="field.type === 'number'" :value="Number(modelValue ?? field.default ?? 0)" class="w-full" @change="(value: any) => update(value ?? 0)" />
    <EditableCombobox v-else-if="field.type === 'select' && field.option_items" :value="String(modelValue ?? field.default ?? '')" :items="field.option_items" :allow-custom="field.allow_custom !== false" class="w-full" @update:model-value="(value: string) => update(value)" />
    <AppSelect v-else-if="field.type === 'select'" :value="String(modelValue ?? field.default ?? '')" class="w-full" @change="(value: any) => update(value)">
      <Select.Option v-for="option in field.options" :key="option" :value="option">{{ option }}</Select.Option>
    </AppSelect>
    <Input v-else-if="field.type === 'text-list' || field.type === 'int-list'" :value="textListValue(modelValue)" @change="(event: any) => update(event.target.value)" />
    <Input v-else :value="String(modelValue ?? '')" @change="(event: any) => update(event.target.value)" />
    <div v-if="field.type === 'text-list' || field.type === 'int-list'" class="text-xs text-text-tertiary mt-1">逗号分隔</div>
    <div v-else-if="field.help" class="text-xs text-text-tertiary mt-1">{{ field.help }}</div>
  </div>
</template>
