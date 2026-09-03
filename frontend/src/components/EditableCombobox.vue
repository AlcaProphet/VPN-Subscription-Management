<!-- EditableCombobox.vue：可编辑下拉/推荐选项组件（Build19 Step 2）
  行为：
  - 打开可浏览候选，输入过滤（大小写不敏感，匹配规范值和显示名）。
  - 无匹配且允许自定义时显示“使用自定义值”。
  - 只有选择候选或明确点击自定义值才回写 modelValue；失焦/Escape 不自动改写。
  - 旧值不在候选时仍回显原值并可覆盖。
  - 接入全局单浮层管理，Escape 关闭并回到触发元素。
-->
<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { nextOverlayId, registerOverlay } from '@/utils/overlayManager'
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
}>()

const open = ref(false)
const text = ref(labelFor(props.value))
const activeIndex = ref(-1)
const overlayId = nextOverlayId('editable-combobox')
let unregister: (() => void) | null = null
const inputEl = ref<HTMLInputElement | null>(null)

const groupLabelMap: Record<string, string> = {
  common: '常用',
  extended: '扩展',
  legacy: '旧版兼容',
  pending: '待验证',
  unverified: '待验证',
}

function labelFor(value: string): string {
  const item = props.items.find((item) => item.value === value)
  return item?.label || value
}

function groupLabel(group?: string): string {
  return (group && groupLabelMap[group]) || group || ''
}

watch(() => props.value, (value) => {
  if (!open.value) text.value = labelFor(value)
}, { immediate: true })

interface DisplayItem {
  kind: 'option' | 'custom'
  value: string
  label: string
  group?: string
  verified?: string
}

const filteredItems = computed<OptionItem[]>(() => {
  const query = text.value.trim().toLowerCase()
  if (!query) return props.items
  return props.items.filter((item) => {
    const value = (item.value || '').toLowerCase()
    const label = (item.label || '').toLowerCase()
    return value.includes(query) || label.includes(query)
  })
})

const showCustom = computed(() => {
  if (!props.allowCustom) return false
  const query = text.value.trim()
  if (!query) return false
  const itemValues = filteredItems.value.map((item) => item.value)
  if (itemValues.includes(query) || itemValues.some((item) => item.toLowerCase() === query.toLowerCase())) return false
  return true
})

const displayItems = computed<DisplayItem[]>(() => {
  const items: DisplayItem[] = []
  const emptyOption = props.items.find((item) => item.value === '')
  // 空值候选（如“无/不使用插件”）始终可点击，不依赖用户清空搜索词。
  if (emptyOption) {
    items.push({
      kind: 'option',
      value: '',
      label: emptyOption.label || '无',
      group: emptyOption.group,
      verified: emptyOption.verified,
    })
  }
  const shownFiltered = emptyOption
    ? filteredItems.value.filter((item) => item.value !== '')
    : filteredItems.value
  items.push(...shownFiltered.map((item) => ({
    kind: 'option' as const,
    value: item.value,
    label: item.label || item.value,
    group: item.group,
    verified: item.verified,
  })))
  if (showCustom.value) {
    items.push({ kind: 'custom', value: text.value, label: `使用自定义值：${text.value}` })
  }
  return items
})

function openDropdown() {
  if (props.disabled || open.value) return
  // 打开时清空搜索态：当前选中值仅用于关闭后的回显，不作为打开时的过滤词。
  text.value = ''
  activeIndex.value = -1
  open.value = true
  unregister = registerOverlay({
    id: overlayId,
    type: 'select',
    close: closeDropdown,
    focusTrigger: () => inputEl.value?.focus(),
  })
}

function closeDropdown() {
  if (!open.value) return
  open.value = false
  text.value = labelFor(props.value)
  activeIndex.value = -1
  unregister?.()
  unregister = null
}

function selectItem(item: DisplayItem) {
  emit('update:modelValue', item.value)
  closeDropdown()
}

function onInput(event: Event) {
  const next = (event.target as HTMLInputElement).value
  text.value = next
  activeIndex.value = -1
}

function onKeydown(event: KeyboardEvent) {
  if (!open.value) {
    if (event.key === 'ArrowDown' || event.key === 'Enter') {
      event.preventDefault()
      openDropdown()
    }
    return
  }
  if (event.key === 'Escape') {
    event.preventDefault()
    closeDropdown()
    return
  }
  if (event.key === 'ArrowDown') {
    event.preventDefault()
    activeIndex.value = displayItems.value.length === 0 ? -1 : (activeIndex.value + 1) % displayItems.value.length
    return
  }
  if (event.key === 'ArrowUp') {
    event.preventDefault()
    activeIndex.value = displayItems.value.length === 0 ? -1 : (activeIndex.value <= 0 ? displayItems.value.length - 1 : activeIndex.value - 1)
    return
  }
  if (event.key === 'Enter') {
    event.preventDefault()
    if (activeIndex.value >= 0 && activeIndex.value < displayItems.value.length) {
      selectItem(displayItems.value[activeIndex.value])
      return
    }
    if (showCustom.value && text.value.trim()) {
      selectItem({ kind: 'custom', value: text.value, label: text.value })
    }
    return
  }
  if (event.key === 'Tab') {
    closeDropdown()
  }
}

function onBlur() {
  // 延迟关闭，允许点击下拉项；mousedown 已阻止默认时不会触发此路径。
  window.setTimeout(() => {
    if (open.value) closeDropdown()
  }, 120)
}

function onMousedownItem(event: MouseEvent) {
  event.preventDefault()
}

onBeforeUnmount(() => {
  unregister?.()
})
</script>

<template>
  <div class="editable-combobox relative">
    <input
      ref="inputEl"
      :value="text"
      :disabled="disabled"
      :placeholder="placeholder"
      class="w-full rounded-md border border-gray-300 px-3 py-1.5 text-sm outline-none focus:border-primary focus:ring-1 focus:ring-primary disabled:cursor-not-allowed disabled:bg-gray-100"
      :aria-expanded="open"
      role="combobox"
      aria-autocomplete="list"
      @focus="openDropdown"
      @input="onInput"
      @keydown="onKeydown"
      @blur="onBlur"
    />
    <div v-if="open && displayItems.length" class="absolute z-30 mt-1 max-h-56 w-full overflow-auto rounded-lg border bg-white py-1 shadow-lg">
      <button
        v-for="(item, index) in displayItems"
        :key="item.kind === 'custom' ? '__custom__' : item.value"
        type="button"
        class="block w-full px-3 py-1.5 text-left text-sm hover:bg-gray-100"
        :class="{ 'bg-primary/5': activeIndex === index }"
        :data-kind="item.kind"
        @mousedown="onMousedownItem"
        @click="selectItem(item)"
      >
        <span class="text-text">{{ item.label }}</span>
        <span v-if="item.verified" class="ml-2 text-xs text-text-tertiary">{{ item.verified }}</span>
        <span v-if="item.group && item.kind === 'option'" class="ml-2 text-xs text-text-tertiary">{{ groupLabel(item.group) }}</span>
      </button>
    </div>
    <div v-else-if="open && !filteredItems.length && !showCustom" class="absolute z-30 mt-1 w-full rounded-lg border bg-white px-3 py-2 text-sm text-text-tertiary shadow-lg">
      无匹配选项
    </div>
  </div>
</template>
