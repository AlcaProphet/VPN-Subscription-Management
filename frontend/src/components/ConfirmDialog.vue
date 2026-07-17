<template>
  <el-dialog
    :model-value="visible"
    :title="title"
    width="420px"
    :close-on-click-modal="false"
    @update:model-value="$emit('update:visible', $event)"
  >
    <p>{{ message }}</p>
    <template #footer>
      <el-button @click="$emit('cancel'); $emit('update:visible', false)">
        {{ cancelText }}
      </el-button>
      <el-button type="danger" @click="$emit('confirm'); $emit('update:visible', false)">
        {{ confirmText }}
      </el-button>
    </template>
  </el-dialog>
</template>

<script setup>
defineProps({
  visible: { type: Boolean, required: true },
  title: { type: String, default: '确认操作' },
  message: { type: String, default: '确定要执行此操作吗？' },
  confirmText: { type: String, default: '确认' },
  cancelText: { type: String, default: '取消' }
})

defineEmits(['update:visible', 'confirm', 'cancel'])
</script>
