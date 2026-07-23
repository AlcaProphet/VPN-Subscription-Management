<template>
  <el-dialog
    :model-value="visible"
    title="选择 OIDC 提供商"
    :width="dialogWidth"
    :close-on-click-modal="false"
    :append-to-body="true"
    @update:model-value="$emit('update:visible', $event)"
  >
    <div class="space-y-3">
      <label
        v-for="option in providers"
        :key="option.value"
        class="flex items-center gap-2 p-3 rounded-md border cursor-pointer transition-colors"
        :class="selected === option.value
          ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/20'
          : 'border-gray-200 dark:border-gray-600 hover:border-gray-300 dark:hover:border-gray-500'"
      >
        <input
          type="radio"
          :value="option.value"
          :checked="selected === option.value"
          class="w-4 h-4 text-blue-600"
          @change="selected = option.value"
        />
        <span class="text-sm text-gray-700 dark:text-gray-300">{{ option.label }}</span>
      </label>
    </div>
    <template #footer>
      <div class="flex justify-end gap-3">
        <button
          type="button"
          class="bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-600 rounded-md px-4 py-2 text-sm"
          @click="$emit('update:visible', false)"
        >
          取消
        </button>
        <button
          type="button"
          class="bg-blue-600 hover:bg-blue-700 text-white rounded-md px-4 py-2 text-sm disabled:opacity-50"
          :disabled="selected === currentProvider"
          @click="confirm"
        >
          确认切换
        </button>
      </div>
    </template>
  </el-dialog>
</template>

<script setup>
import { ref, watch } from 'vue'
import { useDialogWidth } from '@/composables/useDialogWidth'
const dialogWidth = useDialogWidth('400px')

const props = defineProps({
  visible: { type: Boolean, required: true },
  currentProvider: { type: String, default: 'keycloak' }
})

const emit = defineEmits(['update:visible', 'switch'])

const providers = [
  { value: 'keycloak', label: 'Keycloak' },
  { value: 'auth0', label: 'Auth0' },
  { value: 'generic', label: '通用 OIDC' }
]

const selected = ref(props.currentProvider)

// Reset selection to current provider each time the dialog opens
watch(() => props.visible, (v) => {
  if (v) selected.value = props.currentProvider
})

function confirm() {
  emit('switch', selected.value)
  emit('update:visible', false)
}
</script>
