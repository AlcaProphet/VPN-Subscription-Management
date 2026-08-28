<!-- AppRangePicker.vue：全局单浮层管理包装的 AntD RangePicker（Build12 Step 2） -->
<script setup lang="ts">
import { onBeforeUnmount, ref } from 'vue'
import { DatePicker } from 'ant-design-vue'
import { nextOverlayId, registerOverlay, closeNonModalOverlays } from '@/utils/overlayManager'

const emit = defineEmits<{ 'update:open': [open: boolean]; 'openChange': [open: boolean] }>()
const open = ref(false)
const overlayId = nextOverlayId('range-picker')
let unregister: (() => void) | null = null

function setOpen(value: boolean) {
  open.value = value
  emit('update:open', value)
  emit('openChange', value)
  if (value) {
    closeNonModalOverlays()
    unregister = registerOverlay({ id: overlayId, type: 'picker', close: () => setOpen(false) })
  } else {
    unregister?.()
    unregister = null
  }
}

onBeforeUnmount(() => unregister?.())
</script>

<template>
  <DatePicker.RangePicker v-bind="$attrs" :open="open" @open-change="setOpen" @dropdown-visible-change="setOpen">
    <slot />
  </DatePicker.RangePicker>
</template>
