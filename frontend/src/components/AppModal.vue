<!-- AppModal.vue：统一焦点/浮层管理的 AntD Modal 包装（Build12 Step 2） -->
<script setup lang="ts">
import { nextTick, ref, watch } from 'vue'
import { Modal } from 'ant-design-vue'
import { nextOverlayId, registerOverlay, focusFirstInContainer } from '@/utils/overlayManager'

const props = withDefaults(defineProps<{
  open: boolean
  title?: string
  width?: string | number
  footer?: any
  centered?: boolean
  destroyOnClose?: boolean
}>(), {
  title: '',
  width: 640,
  footer: null,
  centered: false,
  destroyOnClose: false,
})

const emit = defineEmits<{
  'update:open': [open: boolean]
  cancel: []
}>()

const overlayId = nextOverlayId('app-modal')
let overlayUnregister: (() => void) | null = null
const lastFocused = ref<HTMLElement | null>(null)

watch(() => props.open, async (open) => {
  if (open) {
    lastFocused.value = document.activeElement as HTMLElement | null
    overlayUnregister?.()
    overlayUnregister = registerOverlay({
      id: overlayId,
      type: 'modal',
      close: () => emit('update:open', false),
      focusTrigger: () => lastFocused.value?.focus?.(),
    })
    await nextTick()
    setTimeout(() => {
      const el = document.querySelector<HTMLElement>('.ant-modal-content')
      if (el) focusFirstInContainer(el)
    }, 0)
  } else {
    lastFocused.value?.focus?.()
    lastFocused.value = null
    overlayUnregister?.()
    overlayUnregister = null
  }
}, { immediate: true })
</script>

<template>
  <Modal v-bind="$attrs" :open="props.open" :title="props.title" :width="props.width"
         :footer="props.footer" :centered="props.centered" :destroy-on-close="props.destroyOnClose"
         @cancel="emit('update:open', false); emit('cancel')">
    <slot />
  </Modal>
</template>
