<!-- AppPopover.vue：全局单浮层管理包装的 Popover（Build12 Step 2） -->
<script setup lang="ts">
import { onBeforeUnmount, ref } from 'vue'
import { Popover } from 'ant-design-vue'
import { nextOverlayId, registerOverlay, closeNonModalOverlays } from '@/utils/overlayManager'

const props = defineProps<{
  overlayId?: string
}>()
const emit = defineEmits<{ 'openChange': [open: boolean]; 'update:open': [open: boolean] }>()

const id = props.overlayId || nextOverlayId('popover')
const open = ref(false)
let unregister: (() => void) | null = null

function setOpen(value: boolean) {
  open.value = value
  emit('update:open', value)
  emit('openChange', value)
  if (value) {
    closeNonModalOverlays()
    unregister = registerOverlay({ id, type: 'popover', close: () => setOpen(false) })
  } else {
    unregister?.()
    unregister = null
  }
}

onBeforeUnmount(() => unregister?.())
</script>

<template>
  <Popover v-bind="$attrs" :open="open" @open-change="setOpen">
    <slot />
    <template v-if="$slots.content" #content>
      <slot name="content" />
    </template>
  </Popover>
</template>
