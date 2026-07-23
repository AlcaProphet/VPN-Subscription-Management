<template>
  <div>
    <!-- Tab 头 -->
    <div class="flex mb-4 border-b border-gray-200 dark:border-gray-700">
      <button
        class="px-4 py-2 text-sm rounded-t-md transition-colors"
        :class="activeTab === 'file'
          ? 'bg-blue-600 text-white'
          : 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-200 hover:bg-gray-200 dark:hover:bg-gray-600'"
        @click="activeTab = 'file'"
      >
        文件上传
      </button>
      <button
        class="px-4 py-2 text-sm rounded-t-md transition-colors"
        :class="activeTab === 'text'
          ? 'bg-blue-600 text-white'
          : 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-200 hover:bg-gray-200 dark:hover:bg-gray-600'"
        @click="activeTab = 'text'"
      >
        文本编辑
      </button>
    </div>

    <!-- 文件 Tab -->
    <div v-show="activeTab === 'file'">
      <el-upload
        ref="uploadRef"
        :auto-upload="false"
        :limit="1"
        :accept="accept"
        :on-change="onFileChange"
        :before-upload="beforeUpload"
        drag
      >
        <div class="flex flex-col items-center py-4">
          <svg class="w-10 h-10 text-gray-400 mb-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12" />
          </svg>
          <div class="text-sm text-gray-600 dark:text-gray-400">
            将文件拖到此处，或<em class="text-blue-600 not-italic">点击上传</em>
          </div>
          <div class="text-xs text-gray-400 dark:text-gray-500 mt-1">
            文件大小不超过 {{ maxSizeMB }}MB，支持 {{ accept }} 格式
          </div>
        </div>
      </el-upload>
      <!-- 已选文件提示 -->
      <div v-if="selectedFileName" class="mt-2 flex items-center gap-2 text-sm text-gray-600 dark:text-gray-400">
        <svg class="w-4 h-4 text-green-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
        </svg>
        <span>{{ selectedFileName }}</span>
        <button
          class="text-red-500 hover:text-red-600 text-xs"
          @click="clearFile"
        >
          清除
        </button>
      </div>
    </div>

    <!-- 文本 Tab -->
    <div v-show="activeTab === 'text'">
      <textarea
        :value="textContent"
        @input="$emit('update:textContent', $event.target.value)"
        class="w-full h-48 p-3 rounded-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-sm text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none resize-y"
        placeholder="在此粘贴订阅配置文本..."
      ></textarea>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import { useToast } from '@/composables/useToast'

const props = defineProps({
  modelValue: { type: String, default: 'file' },
  accept: { type: String, default: '.conf,.yaml,.yml,.txt' },
  maxSize: { type: Number, default: 50 },
  textContent: { type: String, default: '' }
})

const emit = defineEmits(['update:modelValue', 'update:textContent', 'file-change', 'upload', 'clear-file'])

const { error: toastError } = useToast()

const maxSizeMB = computed(() => props.maxSize)
const activeTab = ref(props.modelValue)
const selectedFileName = ref('')
const uploadRef = ref(null)

// Sync activeTab with parent modelValue
watch(() => props.modelValue, (val) => { activeTab.value = val })
watch(activeTab, (val) => { emit('update:modelValue', val) })

function beforeUpload(file) {
  const maxBytes = props.maxSize * 1024 * 1024
  if (file.size > maxBytes) {
    toastError(`文件大小不能超过 ${props.maxSize}MB`)
    return false
  }
  return true
}

function onFileChange(file) {
  selectedFileName.value = file.name
  emit('file-change', file.raw)
}

function clearFile() {
  selectedFileName.value = ''
  uploadRef.value?.clearFiles()
  emit('clear-file')
}

// Expose methods for parent
defineExpose({ uploadRef, clearFile })
</script>
