<template>
  <el-dialog
    :model-value="visible"
    title="上传版本"
    width="520px"
    :close-on-click-modal="false"
    @update:model-value="$emit('update:visible', $event)"
  >
    <el-tabs v-model="activeTab">
      <el-tab-pane label="文件上传" name="file">
        <el-upload
          ref="uploadRef"
          :auto-upload="false"
          :limit="1"
          :accept="accept"
          :on-change="onFileChange"
          :before-upload="beforeUpload"
          drag
        >
          <el-icon class="el-icon--upload"><upload-filled /></el-icon>
          <div class="el-upload__text">
            将文件拖到此处，或<em>点击上传</em>
          </div>
          <template #tip>
            <div class="el-upload__tip">
              文件大小不超过 {{ maxSizeMB }}MB，支持 {{ accept }} 格式
            </div>
          </template>
        </el-upload>
        <div style="margin-top: 12px; text-align: right">
          <el-button @click="$emit('update:visible', false)">取消</el-button>
          <el-button type="primary" :disabled="!selectedFile" @click="onFileUpload">
            上传文件
          </el-button>
        </div>
      </el-tab-pane>
      <el-tab-pane label="文本编辑" name="text">
        <el-input
          v-model="textContent"
          type="textarea"
          :rows="12"
          placeholder="在此粘贴订阅配置文本..."
        />
        <div style="margin-top: 12px; text-align: right">
          <el-button @click="$emit('update:visible', false)">取消</el-button>
          <el-button type="primary" :disabled="!textContent.trim()" @click="onTextSave">
            保存文本
          </el-button>
        </div>
      </el-tab-pane>
    </el-tabs>
  </el-dialog>
</template>

<script setup>
import { ref, computed } from 'vue'
import { UploadFilled } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'

const props = defineProps({
  visible: { type: Boolean, required: true },
  accept: { type: String, default: '.conf,.yaml,.yml,.txt' },
  maxSize: { type: Number, default: 50 }
})

const emit = defineEmits(['update:visible', 'upload', 'textSave'])

const maxSizeMB = computed(() => props.maxSize)

const activeTab = ref('file')
const selectedFile = ref(null)
const textContent = ref('')
const uploadRef = ref(null)

function beforeUpload(file) {
  const maxBytes = props.maxSize * 1024 * 1024
  if (file.size > maxBytes) {
    ElMessage.error(`文件大小不能超过 ${props.maxSize}MB`)
    return false
  }
  return true
}

// Note: cannot use `import { ElMessage }` at top level without a UI context,
// but ElMessage is globally available via ElementPlus.
function onFileChange(file) {
  selectedFile.value = file.raw
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
  uploadRef.value?.clearFiles()
}
</script>
