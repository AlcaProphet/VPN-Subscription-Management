<!-- EditableCombobox.vue：标准单选下拉 + 显式自定义值草稿。 -->
<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { Button, Input, Select } from 'ant-design-vue'
import AppSelect from '@/components/AppSelect.vue'
import type { OptionItem } from '@/api/node'

const props = withDefaults(defineProps<{
  value?: string
  items?: OptionItem[]
  allowCustom?: boolean
  placeholder?: string
  disabled?: boolean
}>(), {
  value: '',
  items: () => [],
  allowCustom: true,
  placeholder: '',
  disabled: false,
})

const emit = defineEmits<{
  'update:modelValue': [value: string]
  'draft-dirty-change': [dirty: boolean]
}>()

const CUSTOM_VALUE = '__vpn_sub_custom_value__'
const EMPTY_VALUE = '__vpn_sub_empty_value__'
const customMode = ref(false)
const customDraft = ref('')
const customDirty = ref(false)
const customError = ref('')

const groupLabelMap: Record<string, string> = {
  common: '常用',
  extended: '扩展',
  legacy: '旧版兼容',
  pending: '待验证',
  unverified: '待验证',
}

function isKnownValue(value: string): boolean {
  return props.items.some((item) => item.value === value)
}

function isCustomValue(value: string): boolean {
  return props.allowCustom && value !== '' && !isKnownValue(value)
}

function groupLabel(group?: string): string {
  return (group && groupLabelMap[group]) || group || ''
}

function verifiedLabel(verified?: string): string {
  if (!verified) return ''
  return verified.replace(/^mihomo-/i, 'Mihomo ')
}

function internalValue(value: string): string | undefined {
  if (isCustomValue(value)) return CUSTOM_VALUE
  if (value === '') return isKnownValue('') ? EMPTY_VALUE : undefined
  return value
}

const selectedValue = computed(() => customMode.value ? CUSTOM_VALUE : internalValue(props.value))

function setDirty(dirty: boolean) {
  if (customDirty.value === dirty) return
  customDirty.value = dirty
  emit('draft-dirty-change', dirty)
}

function syncFromValue(value: string) {
  const custom = isCustomValue(value)
  customMode.value = custom
  customDraft.value = custom ? value : ''
  customError.value = ''
  setDirty(false)
}

watch([() => props.value, () => props.items, () => props.allowCustom], () => syncFromValue(props.value), { immediate: true, deep: true })

function selectValue(value: unknown) {
  const selected = String(value)
  customError.value = ''
  if (selected === CUSTOM_VALUE) {
    if (!props.allowCustom) return
    customMode.value = true
    customDraft.value = isCustomValue(props.value) ? props.value : ''
    setDirty(!isCustomValue(props.value))
    return
  }
  customMode.value = false
  customDraft.value = ''
  setDirty(false)
  emit('update:modelValue', selected === EMPTY_VALUE ? '' : selected)
}

function updateCustomDraft(value: string) {
  customDraft.value = value
  customError.value = ''
  setDirty(!isCustomValue(props.value) || value.trim() !== props.value)
}

function applyCustom() {
  const value = customDraft.value.trim()
  if (!value) {
    customError.value = '请输入自定义值'
    setDirty(true)
    return
  }
  customDraft.value = value
  setDirty(false)
  emit('update:modelValue', value)
}

function cancelCustom() {
  syncFromValue(props.value)
}

function onCustomKeydown(event: KeyboardEvent) {
  if (event.key !== 'Enter' || event.isComposing) return
  event.preventDefault()
  applyCustom()
}

onBeforeUnmount(() => {
  if (customDirty.value) emit('draft-dirty-change', false)
})
</script>

<template>
  <div class="editable-combobox">
    <AppSelect
      :value="selectedValue"
      :disabled="disabled"
      :placeholder="placeholder"
      class="w-full"
      @change="selectValue"
    >
      <Select.Option
        v-for="item in items"
        :key="item.value === '' ? EMPTY_VALUE : item.value"
        :value="item.value === '' ? EMPTY_VALUE : item.value"
        :label="[item.label || item.value, item.value, verifiedLabel(item.verified), groupLabel(item.group)].filter(Boolean).join(' ')"
      >
        <span class="text-text">{{ item.label || item.value }}</span>
        <span v-if="item.verified || item.group" class="ml-2 text-xs text-text-tertiary">
          <span v-if="item.verified">{{ verifiedLabel(item.verified) }}</span><span v-if="item.verified && item.group"> · </span><span v-if="item.group">{{ groupLabel(item.group) }}</span>
        </span>
      </Select.Option>
      <Select.Option v-if="allowCustom" :value="CUSTOM_VALUE" label="其他 自定义">其他（自定义）</Select.Option>
    </AppSelect>

    <div v-if="customMode" class="custom-value-editor mt-2 rounded-md border p-3">
      <label class="mb-1 block text-sm text-text-secondary">自定义值 <span class="text-red-500">*</span></label>
      <Input
        :value="customDraft"
        :disabled="disabled"
        placeholder="请输入列表外的自定义值"
        :status="customError ? 'error' : undefined"
        @input="(event: any) => updateCustomDraft(event.target.value)"
        @keydown="onCustomKeydown"
      />
      <div v-if="customError" class="mt-1 text-xs text-red-500">{{ customError }}</div>
      <div class="mt-2 flex flex-wrap items-center gap-2">
        <Button type="primary" size="small" :disabled="disabled" @click="applyCustom">应用自定义值</Button>
        <Button size="small" :disabled="disabled" @click="cancelCustom">取消</Button>
        <span v-if="customDirty" class="text-xs text-text-tertiary">自定义值草稿未应用</span>
      </div>
    </div>
  </div>
</template>
