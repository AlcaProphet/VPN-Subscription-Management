<!-- AppSelect.vue：全局单浮层管理包装的 AntD Select（Build12 Step 2） -->
<script setup lang="ts">
import { onBeforeUnmount, ref } from 'vue'
import { Select } from 'ant-design-vue'
import { nextOverlayId, registerOverlay, closeNonModalOverlays } from '@/utils/overlayManager'

const emit = defineEmits<{
  'update:open': [open: boolean]
  'dropdownVisibleChange': [open: boolean]
  'openChange': [open: boolean]
}>()

const open = ref(false)
const selectRef = ref<any>(null)
const overlayId = nextOverlayId('select')
let unregister: (() => void) | null = null

function focusTrigger() {
  const input = selectRef.value?.$el?.querySelector?.('input') as HTMLElement | undefined
  input?.focus?.({ preventScroll: true })
}

function setOpen(value: boolean) {
  open.value = value
  emit('update:open', value)
  emit('dropdownVisibleChange', value)
  emit('openChange', value)
  if (value) {
    closeNonModalOverlays()
    unregister = registerOverlay({
      id: overlayId,
      type: 'select',
      close: () => setOpen(false),
      focusTrigger,
    })
  } else {
    unregister?.()
    unregister = null
  }
}

onBeforeUnmount(() => {
  unregister?.()
})
</script>

<template>
  <Select ref="selectRef" v-bind="$attrs" :open="open" @dropdown-visible-change="setOpen">
    <slot />
  </Select>
</template>
