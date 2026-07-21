<template>
  <div class="versions-container" v-loading="loading">
    <!-- Header -->
    <div class="page-header">
      <div class="header-left">
        <el-button @click="goBack" text>
          <el-icon><ArrowLeft /></el-icon>
          返回分享列表
        </el-button>
        <h2 v-if="share">{{ share.name }} — 版本管理</h2>
      </div>
      <el-button type="primary" @click="uploadVisible = true">
        <el-icon><Plus /></el-icon>
        新建版本
      </el-button>
    </div>

    <!-- Version List -->
    <el-empty
      v-if="!loading && versions.length === 0"
      description="暂无版本"
    />

    <el-table
      v-else
      :data="sortedVersions"
      stripe
      class="versions-table"
    >
      <el-table-column label="版本号" width="100">
        <template #default="{ row }">
          v{{ row.version }}
        </template>
      </el-table-column>
      <el-table-column label="创建时间" width="180">
        <template #default="{ row }">
          {{ formatTime(row.created_at) }}
        </template>
      </el-table-column>
      <el-table-column label="更新时间" width="180">
        <template #default="{ row }">
          {{ formatTime(row.updated_at) }}
        </template>
      </el-table-column>
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-tag v-if="isCurrent(row)" type="success" size="small">
            当前
          </el-tag>
          <span v-else class="no-tag">—</span>
        </template>
      </el-table-column>
      <el-table-column label="操作" min-width="240" fixed="right">
        <template #default="{ row }">
          <el-button
            v-if="!isCurrent(row)"
            size="small"
            type="primary"
            @click="handleSwitch(row)"
          >
            设为当前
          </el-button>
          <el-button size="small" @click="handlePreview(row)">
            预览
          </el-button>
          <el-button
            size="small"
            type="danger"
            :disabled="isCurrent(row) || versions.length <= 1"
            @click="confirmDeleteVersion(row)"
          >
            删除
          </el-button>
        </template>
      </el-table-column>
    </el-table>

    <!-- Upload Modal -->
    <UploadModal
      v-model:visible="uploadVisible"
      :initial-content="editContent"
      @upload="onFileUpload"
      @textSave="onTextSave"
    />

    <!-- Preview Dialog -->
    <el-dialog
      v-model="previewVisible"
      title="版本预览"
      width="640px"
      :close-on-click-modal="false"
    >
      <pre class="preview-content">{{ previewContent }}</pre>
      <template #footer>
        <el-button @click="previewVisible = false">关闭</el-button>
        <el-button type="primary" @click="handleEditFromPreview">
          基于此版本编辑
        </el-button>
      </template>
    </el-dialog>

    <!-- Delete Version Confirm -->
    <ConfirmDialog
      v-model:visible="deleteVersionVisible"
      title="删除版本"
      :message="'确定删除版本 v' + (deleteVersionTarget?.version || '') + '？'"
      @confirm="handleDeleteVersion"
    />
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { ArrowLeft, Plus } from '@element-plus/icons-vue'
import { adminApi } from '@/services/api'
import UploadModal from '@/components/UploadModal.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'

const route = useRoute()
const router = useRouter()

// ==========================================================================
// Data
// ==========================================================================
const loading = ref(true)
const share = ref(null)
const uploadVisible = ref(false)

const previewVisible = ref(false)
const previewContent = ref('')
const editContent = ref('')

const deleteVersionVisible = ref(false)
const deleteVersionTarget = ref(null)

// ==========================================================================
// Computed
// ==========================================================================
const versions = computed(() => {
  return share.value?.versions || []
})

const sortedVersions = computed(() => {
  return [...versions.value].sort((a, b) => b.version - a.version)
})

const currentVersionNum = computed(() => {
  if (versions.value.length === 0) return null
  const sorted = [...versions.value].sort(
    (a, b) => new Date(b.updated_at || 0) - new Date(a.updated_at || 0)
  )
  return sorted[0]?.version ?? null
})

// ==========================================================================
// Helpers
// ==========================================================================
function isCurrent(v) {
  return v.version === currentVersionNum.value
}

function formatTime(t) {
  if (!t) return ''
  return new Date(t).toLocaleString()
}

// ==========================================================================
// Data Loading
// ==========================================================================
async function fetchShare() {
  const id = route.params.id
  if (!id) {
    ElMessage.error('缺少分享订阅 ID')
    router.push('/admin/shares')
    return
  }
  try {
    const res = await adminApi.shares.get(id)
    share.value = res.data.share
  } catch (e) {
    ElMessage.error('加载分享订阅信息失败')
    router.push('/admin/shares')
  }
}

// ==========================================================================
// Upload
// ==========================================================================
async function onFileUpload(file) {
  const id = route.params.id
  const fd = new FormData()
  fd.append('file', file)
  try {
    await adminApi.shares.uploadVersion(id, fd)
    ElMessage.success('版本已上传')
    await fetchShare()
  } catch (e) {
    const msg = e.response?.data?.error || '上传失败'
    ElMessage.error(msg)
  }
}

async function onTextSave(content) {
  const id = route.params.id
  try {
    await adminApi.shares.createVersionFromText(id, content)
    ElMessage.success('新版本已创建')
    await fetchShare()
  } catch (e) {
    const msg = e.response?.data?.error || '创建失败'
    ElMessage.error(msg)
  }
}

// ==========================================================================
// Version Operations
// ==========================================================================
async function handleSwitch(v) {
  const id = route.params.id
  try {
    await adminApi.shares.switchVersion(id, v.version)
    ElMessage.success('已切换当前版本')
    await fetchShare()
  } catch (e) {
    const msg = e.response?.data?.error || '切换失败'
    ElMessage.error(msg)
  }
}

async function handlePreview(v) {
  const id = route.params.id
  try {
    const res = await adminApi.shares.getVersion(id, v.version)
    previewContent.value = res.data.content || ''
    previewVisible.value = true
  } catch (e) {
    ElMessage.error('加载版本内容失败')
  }
}

function handleEditFromPreview() {
  editContent.value = previewContent.value
  previewVisible.value = false
  uploadVisible.value = true
}

function confirmDeleteVersion(v) {
  deleteVersionTarget.value = v
  deleteVersionVisible.value = true
}

async function handleDeleteVersion() {
  if (!deleteVersionTarget.value) return
  const id = route.params.id
  try {
    await adminApi.shares.deleteVersion(id, deleteVersionTarget.value.version)
    ElMessage.success('版本已删除')
    deleteVersionTarget.value = null
    await fetchShare()
  } catch (e) {
    const msg = e.response?.data?.error || '删除失败'
    ElMessage.error(msg)
  }
}

// ==========================================================================
// Navigation
// ==========================================================================
function goBack() {
  router.push('/admin/shares')
}

// ==========================================================================
// Lifecycle
// ==========================================================================
onMounted(async () => {
  loading.value = true
  await fetchShare()
  loading.value = false
})
</script>

<style scoped>
.versions-container {
  padding: 0;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
  flex-wrap: wrap;
  gap: 12px;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 12px;
}

.header-left h2 {
  margin: 0;
  font-size: 20px;
  font-weight: 600;
}

.versions-table {
  width: 100%;
}

.preview-content {
  max-height: 400px;
  overflow: auto;
  background: var(--el-fill-color-light);
  padding: 16px;
  border-radius: 4px;
  font-size: 13px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-all;
  margin: 0;
}

.no-tag {
  color: var(--el-text-color-secondary);
}
</style>
