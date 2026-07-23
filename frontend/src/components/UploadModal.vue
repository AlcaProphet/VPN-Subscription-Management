<template>
  <el-dialog
    :model-value="visible"
    title="上传版本"
    :width="isMobile ? '90%' : '520px'"
    :fullscreen="isMobile"
    :close-on-click-modal="false"
    :append-to-body="true"
    :modal-append-to-body="true"
    @update:model-value="$emit('update:visible', $event)"
  >
    <UploadTabs
      ref="uploadTabsRef"
      v-model="activeTab"
      v-model:textContent="textContent"
      :accept="accept"
      :maxSize="maxSize"
      @file-change="onFileChange"
    />

    <template #footer>
      <div class="flex justify-end gap-2">
        <button
          class="bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-600 rounded-md px-4 py-2 text-sm"
          @click="$emit('update:visible', false)"
        >
          取消
        </button>
        <button
          v-if="activeTab === 'file'"
          class="bg-blue-600 hover:bg-blue-700 text-white rounded-md px-4 py-2 text-sm disabled:opacity-50 disabled:cursor-not-allowed"
          :disabled="!selectedFile"
          @click="onFileUpload"
        >
          上传文件
        </button>
        <button
          v-else
          class="bg-blue-600 hover:bg-blue-700 text-white rounded-md px-4 py-2 text-sm disabled:opacity-50 disabled:cursor-not-allowed"
          :disabled="!textContent.trim()"
          @click="onTextSave"
        >
          保存文本
        </button>
      </div>
    </template>
  </el-dialog>
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import { useIsMobile } from '@/composables/useIsMobile'
import UploadTabs from '@/components/UploadTabs.vue'

const isMobile = useIsMobile()

const props = defineProps({
  visible: { type: Boolean, required: true },
  accept: { type: String, default: '.conf,.yaml,.yml,.txt' },
  maxSize: { type: Number, default: 50 },
  initialContent: { type: String, default: '' }
})

const emit = defineEmits(['update:visible', 'upload', 'textSave'])

const maxSizeMB = computed(() => props.maxSize)

const activeTab = ref('file')
const selectedFile = ref(null)
const textContent = ref('')
const uploadTabsRef = ref(null)

// Pre-fill text editor when opening with initial content (e.g. editing a
// previewed version).
watch(() => props.visible, (isVisible) => {
  if (isVisible && props.initialContent) {
    textContent.value = props.initialContent
    activeTab.value = 'text'
  }
})

function onFileChange(file) {
  selectedFile.value = file
}

function onFileUpload() {
  if (selectedFile.value) {
    emit('upload', selectedFile.value)
    emit('update:visible', false)
    resetForm()
  }
}

function onTextSave() {
  if (textContent.value.trim()) {
    emit('textSave', textContent.value)
    emit('update:visible', false)
    resetForm()
  }
}

function resetForm() {
  selectedFile.value = null
  textContent.value = ''
  activeTab.value = 'file'
  uploadTabsRef.value?.clearFile()
}
</script>
