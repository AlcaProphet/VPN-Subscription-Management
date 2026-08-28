<!-- ConfirmModal.vue：删除/危险操作统一确认对话框（禁止浏览器原生 confirm） -->
<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { Modal, Input } from 'ant-design-vue'
import { nextOverlayId, registerOverlay, focusFirstInContainer } from '@/utils/overlayManager'

const props = defineProps<{
  open: boolean
  title: string
  content?: string        // 影响提示文案（支持插槽扩展）
  danger?: boolean        // 危险操作红色确认按钮
  confirmWord?: string    // 需输入确认词时传入（如 RESET/IMPORT，Build3 使用）
  loading?: boolean
}>()
const emit = defineEmits<{ confirm: []; cancel: []; 'update:open': [boolean] }>()

const word = ref('')
// 确认词不正确时确认按钮禁用
const okDisabled = computed(() => !!props.confirmWord && word.value !== props.confirmWord)

const overlayId = nextOverlayId('confirm-modal')
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
      const container = document.querySelector<HTMLElement>('.ant-modal-content')
      if (container) focusFirstInContainer(container)
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
  <Modal :open="open" :title="title" :ok-button-props="{ danger, disabled: okDisabled, loading }"
         @ok="emit('confirm')" @cancel="emit('update:open', false); emit('cancel')">
    <p v-if="content">{{ content }}</p>
    <slot />
    <Input v-if="confirmWord" v-model:value="word" :placeholder="`请输入 ${confirmWord} 以确认`" class="mt-3" />
  </Modal>
</template>
