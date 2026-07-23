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
        :class="currentProvider === option.value
          ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/20'
          : 'border-gray-200 dark:border-gray-600 hover:border-gray-300 dark:hover:border-gray-500'"
      >
        <input
          type="radio"
          :value="option.value"
          :checked="currentProvider === option.value"
          class="w-4 h-4 text-blue-600"
          @change="onSelect(option.value)"
        />
        <span class="text-sm text-gray-700 dark:text-gray-300">{{ option.label }}</span>
      </label>
    </div>
    <template #footer>
      <div class="flex justify-end">
        <button
          class="bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-600 rounded-md px-4 py-2 text-sm"
          @click="$emit('update:visible', false)"
        >
          取消
        </button>
      </div>
    </template>
  </el-dialog>
</template>

<script setup>
import { nextTick } from 'vue'
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

function onSelect(provider) {
  emit('switch', provider)
  // Defer closing until Vue finishes processing the reactive update triggered
  // by the parent's handleProviderSwitch (which may swap v-if template blocks).
  // Without nextTick, the dialog closing animation races with DOM mutations,
  // causing the popup to be interrupted.
  nextTick(() => {
    emit('update:visible', false)
  })
}
</script>
