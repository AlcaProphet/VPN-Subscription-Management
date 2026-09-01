<!-- AppDropdown.vue：全局单浮层管理包装的 Dropdown（Build12 Step 2） -->
<script setup lang="ts">
import { onBeforeUnmount, ref } from 'vue'
import { Dropdown } from 'ant-design-vue'
import { nextOverlayId, registerOverlay, closeNonModalOverlays } from '@/utils/overlayManager'

const props = defineProps<{
  overlayId?: string
}>()
const emit = defineEmits<{ 'openChange': [open: boolean]; 'update:open': [open: boolean] }>()

const id = props.overlayId || nextOverlayId('dropdown')
const open = ref(false)
let unregister: (() => void) | null = null

function setOpen(value: boolean) {
  open.value = value
  emit('update:open', value)
  emit('openChange', value)
  if (value) {
    closeNonModalOverlays()
    unregister = registerOverlay({
      id,
      type: 'dropdown',
      close: () => setOpen(false),
    })
  } else {
    unregister?.()
    unregister = null
  }
}

function onOpenChange(value: boolean) {
  setOpen(value)
}

onBeforeUnmount(() => {
  unregister?.()
})
</script>

<template>
  <Dropdown v-bind="$attrs" :open="open" @open-change="onOpenChange">
    <slot />
    <template v-if="$slots.overlay" #overlay>
      <slot name="overlay" />
    </template>
  </Dropdown>
</template>
