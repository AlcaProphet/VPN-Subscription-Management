<template>
  <el-dialog
    :model-value="visible"
    :title="title"
    :width="dialogWidth"
    :close-on-click-modal="false"
    :append-to-body="true"
    @update:model-value="$emit('update:visible', $event)"
  >
    <p class="text-gray-700 dark:text-gray-300">{{ message }}</p>
    <template #footer>
      <div class="flex justify-end gap-2">
        <button
          class="bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-600 rounded-md px-4 py-2 text-sm"
          @click="$emit('cancel'); $emit('update:visible', false)"
        >
          {{ cancelText }}
        </button>
        <button
          class="bg-red-600 hover:bg-red-700 text-white rounded-md px-4 py-2 text-sm"
          @click="$emit('confirm'); $emit('update:visible', false)"
        >
          {{ confirmText }}
        </button>
      </div>
    </template>
  </el-dialog>
</template>

<script setup>
import { useDialogWidth } from '@/composables/useDialogWidth'
const dialogWidth = useDialogWidth('420px')
defineProps({
  visible: { type: Boolean, required: true },
  title: { type: String, default: '确认操作' },
  message: { type: String, default: '确定要执行此操作吗？' },
  confirmText: { type: String, default: '确认' },
  cancelText: { type: String, default: '取消' }
})

defineEmits(['update:visible', 'confirm', 'cancel'])
</script>
