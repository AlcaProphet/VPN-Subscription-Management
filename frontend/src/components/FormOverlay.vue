<!-- FormOverlay.vue：表单统一载体，桌面使用弹窗，手机使用底部全屏抽屉。 -->
<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { Button, Drawer, Modal } from 'ant-design-vue'
import { CloseOutlined } from '@ant-design/icons-vue'

const props = withDefaults(defineProps<{
  open: boolean
  title: string
  width?: string | number
  loading?: boolean
  destroyOnClose?: boolean
}>(), {
  width: 520,
  loading: false,
  destroyOnClose: false,
})

const emit = defineEmits<{
  'update:open': [open: boolean]
  cancel: []
  submit: []
}>()

const isMobile = ref(false)
let mediaQuery: MediaQueryList | null = null

function syncViewport() {
  isMobile.value = mediaQuery?.matches ?? false
}

function close() {
  emit('update:open', false)
  emit('cancel')
}

function submit() {
  emit('submit')
}

onMounted(() => {
  mediaQuery = window.matchMedia('(max-width: 767px)')
  syncViewport()
  mediaQuery.addEventListener('change', syncViewport)
})

onUnmounted(() => {
  mediaQuery?.removeEventListener('change', syncViewport)
})
</script>

<template>
  <Modal v-if="!isMobile" :open="props.open" :width="props.width" :closable="false"
         :confirm-loading="props.loading" :destroy-on-close="props.destroyOnClose" @cancel="close">
    <template #title>
      <div class="form-overlay__title">
        <span>{{ props.title }}</span>
        <Button type="text" class="touch-target" :aria-label="`关闭${props.title}`" @click="close">
          <template #icon><CloseOutlined /></template>
        </Button>
      </div>
    </template>

    <slot />

    <template #footer>
      <div class="form-overlay__footer">
        <slot name="footer" :close="close" :submit="submit">
          <Button class="touch-target" @click="close">取消</Button>
          <Button type="primary" class="touch-target" :loading="props.loading" @click="submit">保存</Button>
        </slot>
      </div>
    </template>
  </Modal>

  <Drawer v-else :open="props.open" placement="bottom" height="100dvh" :closable="false"
          :destroy-on-close="props.destroyOnClose" root-class-name="form-overlay-drawer" @close="close">
    <template #title>
      <div class="form-overlay__title">
        <span>{{ props.title }}</span>
        <Button type="text" class="touch-target" :aria-label="`关闭${props.title}`" @click="close">
          <template #icon><CloseOutlined /></template>
        </Button>
      </div>
    </template>

    <slot />

    <template #footer>
      <div class="form-overlay__footer">
        <slot name="footer" :close="close" :submit="submit">
          <Button class="touch-target" @click="close">取消</Button>
          <Button type="primary" class="touch-target" :loading="props.loading" @click="submit">保存</Button>
        </slot>
      </div>
    </template>
  </Drawer>
</template>

<style>
.form-overlay__title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-width: 0;
}

.form-overlay__title > span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.form-overlay__footer {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
  padding-bottom: env(safe-area-inset-bottom);
}

.form-overlay-drawer .ant-drawer-body {
  overflow-y: auto;
  overscroll-behavior: contain;
  padding: 20px 16px;
}

.form-overlay-drawer .ant-drawer-footer {
  padding: 12px 16px 0;
}

@media (max-width: 767px) {
  .form-overlay-drawer .ant-btn,
  .form-overlay-drawer .ant-switch {
    min-width: 44px;
    min-height: 44px;
  }
}
</style>
