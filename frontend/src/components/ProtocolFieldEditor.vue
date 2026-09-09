<!-- ProtocolFieldEditor.vue：协议字段递归编辑器；对象默认结构化，保留对象级高级 JSON。 -->
<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { Button, Input, InputNumber, Select, Switch } from 'ant-design-vue'
import EditableCombobox from '@/components/EditableCombobox.vue'
import type { CurrentState, FieldSchema } from '@/api/node'
import { matchesSensitivePath, pathContains } from '@/utils/nodeFeatures'
import { hasConfiguredValue, matchesCondition } from '@/utils/nodeFormLayout'

const props = withDefaults(defineProps<{
  field: FieldSchema
  modelValue?: unknown
  sensitivePaths?: string[]
  savedSensitivePaths?: string[]
  invalidatedSensitivePaths?: string[]
  credentialState?: 'unset' | 'saved' | 'replacing' | 'cleared'
  path?: string
  currentState?: CurrentState
  jsonResetVersions?: Record<string, number>
  centralizedSwitches?: boolean
}>(), {
  modelValue: undefined,
  sensitivePaths: () => [],
  savedSensitivePaths: () => [],
  invalidatedSensitivePaths: () => [],
  credentialState: undefined,
  path: '',
  currentState: undefined,
  jsonResetVersions: () => ({}),
  centralizedSwitches: false,
})

const emit = defineEmits<{
  'update:modelValue': [value: unknown]
  'validity-change': [payload: { path: string; valid: boolean }]
  'json-dirty-change': [payload: { path: string; dirty: boolean }]
  'credential-change': [payload: { path: string; value: string }]
}>()

const fieldPath = computed(() => props.path || props.field.name)
const advanced = ref(false)
const jsonText = ref('')
const jsonError = ref('')
const jsonDirty = ref(false)
const jsonInvalidPath = ref('')
const mapErrors = reactive<Record<string, string>>({})
const mapKeyErrors = reactive<Record<string, string>>({})
const mapRowIDs = reactive(new Map<string, string>())
let nextMapRowID = 0

const objectValue = computed<Record<string, unknown>>(() => {
  if (props.modelValue && typeof props.modelValue === 'object' && !Array.isArray(props.modelValue)) {
    return props.modelValue as Record<string, unknown>
  }
  return {}
})

const listValue = computed<unknown[]>(() => Array.isArray(props.modelValue) ? props.modelValue : [])
const knownNames = computed(() => new Set((props.field.properties ?? []).map((item) => item.name)))
const unknownCount = computed(() => Object.keys(objectValue.value).filter((key) => !knownNames.value.has(key)).length)
const mapEntries = computed(() => Object.entries(objectValue.value).map(([key, value]) => ({
  id: mapRowIDs.get(key) ?? `map-key:${key}`,
  key,
  value,
})))
const sensitive = computed(() => props.field.type === 'password' || props.sensitivePaths.some((path) => matchesSensitivePath(path, fieldPath.value)))

function visibleProperties(properties?: FieldSchema[]): FieldSchema[] {
  return (properties ?? []).filter((property) => matchesCondition(property.when, props.currentState))
}
const structuredProperties = computed(() => visibleProperties(props.field.properties)
  .filter((field) => !props.centralizedSwitches || field.type !== 'bool'))
const advancedCount = computed(() => structuredProperties.value.filter((field) => field.advanced && hasConfiguredValue(childValue(field.name))).length)
const shownCredentialState = computed(() => {
  if (props.credentialState) return props.credentialState
  if (typeof props.modelValue === 'string' && props.modelValue !== '') return 'replacing'
  if (props.invalidatedSensitivePaths.includes(fieldPath.value)) return 'cleared'
  return props.savedSensitivePaths.includes(fieldPath.value) ? 'saved' : 'unset'
})

watch(() => props.modelValue, (value) => {
  syncMapRowIDs(value)
  jsonDirty.value = false
  emitJsonDirty(false)
  if (!advanced.value) jsonText.value = JSON.stringify(value ?? emptyObjectValue(), null, 2)
}, { immediate: true, deep: true })

// 只丢弃与重置范围重叠的局部草稿，关闭子功能也会使覆盖它的父 JSON 草稿失效。
watch(() => Object.entries(props.jsonResetVersions)
  .filter(([path]) => pathContains(path, fieldPath.value) || pathContains(fieldPath.value, path))
  .reduce((version, [, count]) => version + count, 0), () => discardJSON(), { flush: 'post' })

function emptyObjectValue(): Record<string, unknown> | unknown[] {
  return props.field.object_kind === 'list' ? [] : {}
}

function createMapRowID(): string {
  nextMapRowID += 1
  return `map-row-${nextMapRowID}`
}

// 参数名属于业务数据，不能同时充当 Vue 行身份；改名时需保留输入框实例、焦点与输入法状态。
function syncMapRowIDs(value: unknown) {
  if (props.field.object_kind !== 'map') return
  const keys = value && typeof value === 'object' && !Array.isArray(value) ? Object.keys(value) : []
  const currentKeys = new Set(keys)
  for (const key of mapRowIDs.keys()) {
    if (!currentKeys.has(key)) mapRowIDs.delete(key)
  }
  for (const key of keys) {
    if (!mapRowIDs.has(key)) mapRowIDs.set(key, createMapRowID())
  }
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

function forwardCredentialChange(payload: { path: string; value: string }) {
  emit('credential-change', payload)
}

function updateCredential(value: string) {
  update(value)
  emit('credential-change', { path: fieldPath.value, value })
}

function emitJsonDirty(dirty: boolean) {
  emit('json-dirty-change', { path: fieldPath.value, dirty })
}

function setJSONValidity(path: string) {
  if (jsonInvalidPath.value && jsonInvalidPath.value !== path) {
    forwardValidity({ path: jsonInvalidPath.value, valid: true })
  }
  if (path) forwardValidity({ path, valid: false })
  else if (jsonInvalidPath.value) forwardValidity({ path: jsonInvalidPath.value, valid: true })
  jsonInvalidPath.value = path
}

function validateStringMap(value: Record<string, unknown>): { error: string; path: string } {
  for (const [key, item] of Object.entries(value)) {
    if (key === '') return { error: `${fieldPath.value} 参数名不能为空`, path: fieldPath.value }
    if (typeof item !== 'string') {
      const path = `${fieldPath.value}.${key}`
      return { error: `${path} 的值必须为字符串`, path }
    }
  }
  return { error: '', path: '' }
}

function discardMapKeyDrafts() {
  for (const key of Object.keys(mapKeyErrors)) {
    delete mapKeyErrors[key]
    forwardValidity({ path: key ? `${fieldPath.value}.${key}` : fieldPath.value, valid: true })
  }
}

function setAdvanced(next: boolean) {
  if (next === advanced.value) return
  if (!next && jsonError.value) return
  if (next) {
    // 非法改名从未进入模型；切到 JSON 时丢弃其输入草稿并恢复当前有效键。
    discardMapKeyDrafts()
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
  setJSONValidity('')
  advanced.value = false
}

function parseJSONText(): { parsed: unknown; error: string; path: string } {
  try {
    const parsed = JSON.parse(jsonText.value || (props.field.object_kind === 'list' ? '[]' : '{}'))
    const validShape = props.field.object_kind === 'list'
      ? Array.isArray(parsed)
      : parsed !== null && typeof parsed === 'object' && !Array.isArray(parsed)
    if (!validShape) {
      return { parsed: null, error: props.field.object_kind === 'list' ? '请输入 JSON 对象数组' : '请输入 JSON 对象', path: fieldPath.value }
    }
    if (props.field.object_kind === 'map' && props.field.map_value_type === 'string') {
      const validation = validateStringMap(parsed as Record<string, unknown>)
      if (validation.error) return { parsed: null, ...validation }
    }
    return { parsed, error: '', path: '' }
  } catch {
    return { parsed: null, error: props.field.object_kind === 'list' ? '请输入 JSON 对象数组' : '请输入 JSON 对象', path: fieldPath.value }
  }
}

function updateJSON(value: string) {
  jsonText.value = value
  jsonDirty.value = true
  emitJsonDirty(true)
  const result = parseJSONText()
  jsonError.value = result.error
  setJSONValidity(result.path)
}

function applyJSON() {
  const result = parseJSONText()
  jsonError.value = result.error
  if (result.error) {
    setJSONValidity(result.path)
    return
  }
  jsonDirty.value = false
  emitJsonDirty(false)
  setJSONValidity('')
  update(result.parsed)
}

function discardJSON() {
  jsonText.value = JSON.stringify(props.modelValue ?? emptyObjectValue(), null, 2)
  jsonDirty.value = false
  emitJsonDirty(false)
  jsonError.value = ''
  setJSONValidity('')
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
  mapRowIDs.set(key, createMapRowID())
  update({ ...objectValue.value, [key]: '' })
}

function renameMapKey(oldKey: string, newKey: string) {
  const error = newKey === '' ? '参数名不能为空'
    : newKey !== oldKey && newKey in objectValue.value ? '参数名重复' : ''
  const errorPath = oldKey ? `${fieldPath.value}.${oldKey}` : fieldPath.value
  if (error) {
    mapKeyErrors[oldKey] = error
    forwardValidity({ path: errorPath, valid: false })
    return
  }
  if (mapKeyErrors[oldKey]) {
    delete mapKeyErrors[oldKey]
    forwardValidity({ path: errorPath, valid: true })
  }
  if (newKey === oldKey) return
  const rowID = mapRowIDs.get(oldKey) ?? createMapRowID()
  // 在父级同步回写前同时保留旧/新键映射，避免当前 render 周期提前卸载输入框。
  mapRowIDs.set(oldKey, rowID)
  mapRowIDs.set(newKey, rowID)
  if (mapErrors[oldKey]) mapErrors[newKey] = mapErrors[oldKey]
  const next: Record<string, unknown> = {}
  for (const { key, value } of mapEntries.value) next[key === oldKey ? newKey : key] = value
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
  delete mapKeyErrors[key]
  forwardValidity({ path: `${fieldPath.value}.${key}`, valid: true })
  if (key === '') forwardValidity({ path: fieldPath.value, valid: true })
  update(next)
}

function addListItem() {
  const item: Record<string, unknown> = {}
  if (props.field.item_id_field) item[props.field.item_id_field] = crypto.randomUUID()
  update([...listValue.value, item])
}

function listItemID(item: unknown, index: number): string {
  if (item && typeof item === 'object' && !Array.isArray(item) && props.field.item_id_field) {
    const id = (item as Record<string, unknown>)[props.field.item_id_field]
    if (typeof id === 'string' && id) return id
  }
  return String(index)
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

const scalarListItems = computed<string[]>(() => {
  const value = props.modelValue
  if (Array.isArray(value)) return value.map((item) => String(item))
  if (typeof value === 'string' && value.trim() !== '') {
    return value.split(',').map((item) => item.trim()).filter((item) => item !== '')
  }
  return []
})

function emitScalarList(items: string[]) {
  if (props.field.type === 'int-list') {
    const numbers: number[] = []
    for (const item of items) {
      const num = Number(item)
      if (!Number.isNaN(num)) numbers.push(num)
    }
    update(numbers)
    return
  }
  update(items)
}

function addScalarListItem() {
  emitScalarList([...scalarListItems.value, ''])
}

function removeScalarListItem(index: number) {
  const next = [...scalarListItems.value]
  next.splice(index, 1)
  emitScalarList(next)
}

function setScalarListItem(index: number, value: string) {
  const next = [...scalarListItems.value]
  next[index] = value
  emitScalarList(next)
}

function isLongText(field: FieldSchema): boolean {
  return field.type === 'text' && ['client-config', 'certificate', 'ca', 'ca-str', 'host-key', 'restls-script'].includes(field.name)
}

function isComplex(value: unknown): boolean {
  return value !== null && typeof value === 'object'
}
</script>

<template>
  <div v-if="field.type === 'object'" :data-field-path="fieldPath" class="protocol-object-field rounded-lg border p-3">
    <div class="protocol-object-toolbar flex items-center justify-between gap-3 mb-3">
      <div>
        <div class="text-sm font-medium text-text">{{ field.label }}</div>
        <div v-if="field.help" class="text-xs text-text-tertiary mt-1">{{ field.help }}</div>
      </div>
      <div class="protocol-editor-mode flex shrink-0" role="group" :aria-label="`${field.label}编辑模式`">
        <Button :type="!advanced ? 'primary' : 'default'" :aria-pressed="!advanced" :disabled="!!jsonError" @click="setAdvanced(false)">结构化编辑</Button>
        <Button :type="advanced ? 'primary' : 'default'" :aria-pressed="advanced" @click="setAdvanced(true)">高级 JSON</Button>
      </div>
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
        <div v-for="entry in mapEntries" :key="entry.id" class="protocol-map-entry grid grid-cols-1 md:grid-cols-[minmax(140px,0.7fr)_minmax(180px,1fr)_auto] gap-2 items-start">
          <div class="min-w-0">
            <Input :value="entry.key" aria-label="参数名" :aria-invalid="!!mapKeyErrors[entry.key]" @change="(event: any) => renameMapKey(entry.key, event.target.value)" />
            <div v-if="mapKeyErrors[entry.key]" class="map-key-error text-xs text-red-500 mt-1">{{ mapKeyErrors[entry.key] }}</div>
          </div>
          <div class="min-w-0">
            <Switch v-if="field.map_value_type !== 'string' && typeof entry.value === 'boolean'" :checked="entry.value" @change="(next: any) => setMapValue(entry.key, Boolean(next))" />
            <InputNumber v-else-if="field.map_value_type !== 'string' && typeof entry.value === 'number'" :value="entry.value" class="w-full" @change="(next: any) => setMapValue(entry.key, next ?? 0)" />
            <Input.TextArea v-else-if="field.map_value_type !== 'string' && isComplex(entry.value)" :value="JSON.stringify(entry.value)" :rows="2" @blur="(event: any) => setComplexMapValue(entry.key, event.target.value)" />
            <Input v-else :value="String(entry.value ?? '')" @change="(event: any) => setMapValue(entry.key, event.target.value)" />
            <div v-if="mapErrors[entry.key]" class="text-xs text-red-500 mt-1">{{ mapErrors[entry.key] }}</div>
          </div>
          <Button danger @click="removeMapEntry(entry.key)">删除</Button>
        </div>
      </div>
      <div v-else class="text-xs text-text-tertiary mb-2">暂无参数</div>
      <Button size="small" @click="addMapEntry">新增参数</Button>
    </template>

    <template v-else-if="field.object_kind === 'list'">
      <div v-if="listValue.length" class="space-y-3">
        <div v-for="(item, index) in listValue" :key="listItemID(item, index)" class="rounded-lg border p-3">
          <div class="flex items-center justify-between mb-3">
            <span class="text-sm font-medium">第 {{ index + 1 }} 项</span>
            <Button size="small" danger @click="removeListItem(index)">删除</Button>
          </div>
          <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
            <ProtocolFieldEditor
              v-for="property in visibleProperties(field.properties)"
              :key="property.name"
              :field="property"
              :model-value="item && typeof item === 'object' && !Array.isArray(item) ? (item as Record<string, unknown>)[property.name] : undefined"
              :sensitive-paths="sensitivePaths"
              :saved-sensitive-paths="savedSensitivePaths"
              :invalidated-sensitive-paths="invalidatedSensitivePaths"
              :path="`${fieldPath}[${listItemID(item, index)}].${property.name}`"
              :current-state="currentState"
              :json-reset-versions="jsonResetVersions"
              @update:model-value="(value: unknown) => setListChild(index, property.name, value)"
              @validity-change="forwardValidity"
              @json-dirty-change="forwardJsonDirty"
              @credential-change="forwardCredentialChange"
            />
          </div>
        </div>
      </div>
      <div v-else class="text-xs text-text-tertiary mb-2">暂无条目</div>
      <Button size="small" @click="addListItem">新增条目</Button>
    </template>

    <template v-else>
      <component :is="tier ? 'details' : 'div'" v-for="tier in [false, true]" :key="String(tier)"
        v-show="structuredProperties.some((property) => !!property.advanced === tier)"
        :class="tier ? 'protocol-object-advanced rounded-md border p-3 mt-3' : ''">
        <summary v-if="tier" class="cursor-pointer text-sm font-medium">高级参数（已配置 {{ advancedCount }} 项）</summary>
        <div class="protocol-fields-grid grid grid-cols-1 md:grid-cols-2 gap-3 items-start" :class="tier ? 'mt-3' : ''">
        <ProtocolFieldEditor
          v-for="property in structuredProperties.filter((property) => !!property.advanced === tier)"
          :key="property.name"
          :field="property"
          :model-value="childValue(property.name)"
          :sensitive-paths="sensitivePaths"
          :saved-sensitive-paths="savedSensitivePaths"
          :invalidated-sensitive-paths="invalidatedSensitivePaths"
          :path="`${fieldPath}.${property.name}`"
          :current-state="currentState"
          :json-reset-versions="jsonResetVersions"
          :centralized-switches="centralizedSwitches"
          :class="property.type === 'object' ? 'md:col-span-2' : ''"
          @update:model-value="(value: unknown) => setChild(property.name, value)"
          @validity-change="forwardValidity"
          @json-dirty-change="forwardJsonDirty"
          @credential-change="forwardCredentialChange"
        />
        </div>
      </component>
      <div v-if="centralizedSwitches && visibleProperties(field.properties).some((property) => property.type === 'bool')" class="text-xs text-text-tertiary mt-2">运行开关位于“独立开关”区域。</div>
      <div v-if="unknownCount" class="text-xs text-text-tertiary mt-3">
        已保留 {{ unknownCount }} 个未识别参数，可在高级 JSON 中查看和编辑。
      </div>
    </template>
  </div>

  <label v-else-if="field.type === 'bool'" :data-field-path="fieldPath" class="protocol-switch-field flex items-center justify-between gap-3 rounded-md border px-3 py-2 text-sm text-text-secondary">
    <span>{{ field.label }}<span v-if="field.required" class="text-red-500"> *</span><span v-if="field.help" class="block text-xs text-text-tertiary">{{ field.help }}</span></span>
    <Switch :aria-label="field.label" :checked="Boolean(modelValue ?? field.default ?? false)" @change="(value: any) => update(Boolean(value))" />
  </label>

  <div v-else :data-field-path="fieldPath" class="protocol-scalar-field">
    <label class="text-sm text-text-secondary">{{ field.label }}<span v-if="field.required" class="text-red-500"> *</span></label>
    <template v-if="sensitive">
      <Input.Password :value="String(modelValue ?? '')" :placeholder="shownCredentialState === 'saved' ? '已保存（留空保留）' : '未配置'" @change="(event: any) => updateCredential(event.target.value)" />
      <div class="text-xs text-text-tertiary mt-1">
        {{ shownCredentialState === 'saved' ? '已保存（留空保留）' : shownCredentialState === 'replacing' ? '待替换' : '未配置' }}
      </div>
    </template>
    <InputNumber v-else-if="field.type === 'number'" :value="Number(modelValue ?? field.default ?? 0)" class="w-full" @change="(value: any) => update(value ?? 0)" />
    <EditableCombobox v-else-if="(field.type === 'select' || field.type === 'text') && field.option_items" :value="String(modelValue ?? field.default ?? '')" :items="field.option_items" :allow-custom="field.allow_custom === true" class="w-full" @update:model-value="(value: string) => update(value)" />
    <AppSelect v-else-if="field.type === 'select'" :value="String(modelValue ?? field.default ?? '')" class="w-full" @change="(value: any) => update(value)">
      <Select.Option v-for="option in field.options" :key="option" :value="option">{{ option }}</Select.Option>
    </AppSelect>
    <div v-else-if="field.type === 'text-list' || field.type === 'int-list'" class="protocol-list-editor space-y-2">
      <div v-for="(item, index) in scalarListItems" :key="index" class="flex items-center gap-2">
        <Input :value="item" :placeholder="field.type === 'int-list' ? '数字' : '条目'" @change="(event: any) => setScalarListItem(index, event.target.value)" />
        <Button danger @click="removeScalarListItem(index)">删除</Button>
      </div>
      <div v-if="scalarListItems.length === 0" class="text-xs text-text-tertiary">暂无条目</div>
      <Button size="small" @click="addScalarListItem">新增条目</Button>
      <div v-if="field.option_items?.length" class="text-xs text-text-tertiary">
        推荐：{{ field.option_items.map((item) => item.label || item.value).join('、') }}
      </div>
    </div>
    <Input.TextArea v-else-if="isLongText(field)" :value="String(modelValue ?? '')" :rows="4" @change="(event: any) => update(event.target.value)" />
    <Input v-else :value="String(modelValue ?? '')" @change="(event: any) => update(event.target.value)" />
    <div v-if="field.help" class="text-xs text-text-tertiary mt-1">{{ field.help }}</div>
  </div>
</template>

<style scoped>
.protocol-fields-grid { align-items: start; }
.protocol-switch-field { min-height: 44px; align-self: start; }
.protocol-switch-field :deep(.ant-switch) { flex-shrink: 0; min-height: 22px; height: 22px; }
.protocol-scalar-field { min-width: 0; }
.protocol-scalar-field > label { display: block; margin-bottom: 6px; }
.protocol-editor-mode .ant-btn { border-radius: 0; }
.protocol-editor-mode .ant-btn:first-child { border-radius: 6px 0 0 6px; }
.protocol-editor-mode .ant-btn:last-child { border-radius: 0 6px 6px 0; }
@media (max-width: 767px) {
  .protocol-object-toolbar { align-items: flex-start; flex-direction: column; }
  .protocol-object-field :deep(.ant-btn), .protocol-list-editor :deep(.ant-btn) { min-height: 44px; }
}
</style>
