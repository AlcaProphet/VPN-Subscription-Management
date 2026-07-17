<template>
  <div class="share-list-container" v-loading="loading">
    <div class="page-header">
      <h2>分享订阅</h2>
      <el-button type="primary" @click="openCreateDialog">
        <el-icon><Plus /></el-icon>
        创建分享订阅
      </el-button>
    </div>

    <el-empty
      v-if="!loading && shares.length === 0"
      description="暂无分享订阅，请创建"
    />

    <el-table
      v-else
      :data="shares"
      stripe
      class="share-table"
    >
      <el-table-column prop="name" label="名称" min-width="160" />
      <el-table-column label="创建时间" width="180">
        <template #default="{ row }">
          {{ formatTime(row.created_at) }}
        </template>
      </el-table-column>
      <el-table-column label="当前版本" width="100">
        <template #default="{ row }">
          <span v-if="currentVersion(row) !== null">v{{ currentVersion(row) }}</span>
          <span v-else class="no-version">—</span>
        </template>
      </el-table-column>
      <el-table-column label="Token 状态" width="110">
        <template #default="{ row }">
          <el-tag v-if="row.has_token" type="success" size="small">有效</el-tag>
          <el-tag v-else type="danger" size="small">已吊销</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" min-width="320" fixed="right">
        <template #default="{ row }">
          <el-button size="small" @click="goVersions(row)">
            版本管理
          </el-button>
          <el-button
            size="small"
            :disabled="!row.has_token"
            @click="copyShareLink(row)"
          >
            复制分享链接
          </el-button>
          <el-button
            size="small"
            type="warning"
            @click="confirmRefreshToken(row)"
          >
            刷新 Token
          </el-button>
          <el-button
            size="small"
            type="danger"
            :disabled="!row.has_token"
            @click="confirmRevokeToken(row)"
          >
            吊销 Token
          </el-button>
          <el-button
            size="small"
            type="danger"
            @click="confirmDelete(row)"
          >
            删除
          </el-button>
        </template>
      </el-table-column>
    </el-table>

    <!-- Create Dialog -->
    <el-dialog
      v-model="createVisible"
      title="创建分享订阅"
      width="520px"
      :close-on-click-modal="false"
      @closed="resetCreateForm"
    >
      <el-tabs v-model="createTab">
        <el-tab-pane label="文件上传" name="file">
          <el-form ref="createFileFormRef" :model="createForm" :rules="createRules" label-position="top">
            <el-form-item label="名称" prop="name">
              <el-input v-model="createForm.name" placeholder="分享订阅名称" />
            </el-form-item>
            <el-form-item label="订阅文件">
              <el-upload
                ref="createUploadRef"
                :auto-upload="false"
                :limit="1"
                accept=".conf,.yaml,.yml,.txt"
                :on-change="onCreateFileChange"
                :before-upload="beforeCreateUpload"
                drag
              >
                <el-icon class="el-icon--upload"><UploadFilled /></el-icon>
                <div class="el-upload__text">
                  将文件拖到此处，或<em>点击上传</em>
                </div>
                <template #tip>
                  <div class="el-upload__tip">
                    文件大小不超过 50MB
                  </div>
                </template>
              </el-upload>
            </el-form-item>
          </el-form>
          <div style="margin-top: 12px; text-align: right">
            <el-button @click="createVisible = false">取消</el-button>
            <el-button type="primary" :loading="submitting" :disabled="!createFileSelected" @click="handleCreateFile">
              创建并上传
            </el-button>
          </div>
        </el-tab-pane>
        <el-tab-pane label="文本编辑" name="text">
          <el-form ref="createTextFormRef" :model="createForm" :rules="createRules" label-position="top">
            <el-form-item label="名称" prop="name">
              <el-input v-model="createForm.name" placeholder="分享订阅名称" />
            </el-form-item>
            <el-form-item label="订阅内容" prop="content">
              <el-input
                v-model="createForm.content"
                type="textarea"
                :rows="10"
                placeholder="在此粘贴订阅配置文本..."
              />
            </el-form-item>
          </el-form>
          <div style="margin-top: 12px; text-align: right">
            <el-button @click="createVisible = false">取消</el-button>
            <el-button type="primary" :loading="submitting" :disabled="!createForm.content.trim()" @click="handleCreateText">
              创建
            </el-button>
          </div>
        </el-tab-pane>
      </el-tabs>
    </el-dialog>

    <!-- Refresh Token Confirm -->
    <ConfirmDialog
      v-model:visible="refreshVisible"
      title="刷新 Token"
      message="刷新后旧链接立即失效，确定？"
      @confirm="handleRefreshToken"
    />

    <!-- Revoke Token Confirm -->
    <ConfirmDialog
      v-model:visible="revokeVisible"
      title="吊销 Token"
      message="吊销后该分享链接立即不可用，订阅文件保留。确定？"
      @confirm="handleRevokeToken"
    />

    <!-- Delete Confirm -->
    <ConfirmDialog
      v-model:visible="deleteVisible"
      title="删除分享订阅"
      message="确定删除？将级联删除所有版本文件和 Token。"
      @confirm="handleDelete"
    />
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Plus, UploadFilled } from '@element-plus/icons-vue'
import { adminApi, downloadApi } from '@/services/api'
import ConfirmDialog from '@/components/ConfirmDialog.vue'

const router = useRouter()

// ==========================================================================
// Data
// ==========================================================================
const loading = ref(true)
const shares = ref([])

// Create
const createVisible = ref(false)
const createTab = ref('file')
const submitting = ref(false)
const createFileSelected = ref(false)
const createFile = ref(null)
const createForm = reactive({ name: '', content: '' })
const createFileFormRef = ref(null)
const createTextFormRef = ref(null)
const createUploadRef = ref(null)

const createRules = {
  name: [{ required: true, message: '请输入名称', trigger: 'blur' }]
}

// Actions
const refreshVisible = ref(false)
const revokeVisible = ref(false)
const deleteVisible = ref(false)
const actionTarget = ref(null)

// ==========================================================================
// Helpers
// ==========================================================================
function currentVersion(share) {
  if (!share.versions || share.versions.length === 0) return null
  const sorted = [...share.versions].sort(
    (a, b) => new Date(b.updated_at || 0) - new Date(a.updated_at || 0)
  )
  return sorted[0]?.version ?? null
}

function formatTime(t) {
  if (!t) return ''
  return new Date(t).toLocaleString()
}

// ==========================================================================
// Data Loading
// ==========================================================================
async function fetchShares() {
  try {
    const res = await adminApi.shares.list()
    shares.value = res.data.shares || []
  } catch (e) {
    ElMessage.error('加载分享订阅列表失败')
  }
}

// ==========================================================================
// Create
// ==========================================================================
function openCreateDialog() {
  resetCreateForm()
  createVisible.value = true
}

function resetCreateForm() {
  createForm.name = ''
  createForm.content = ''
  createFile.value = null
  createFileSelected.value = false
  createTab.value = 'file'
  createUploadRef.value?.clearFiles()
  createFileFormRef.value?.clearValidate()
  createTextFormRef.value?.clearValidate()
}

function onCreateFileChange(file) {
  createFile.value = file.raw
  createFileSelected.value = true
}

function beforeCreateUpload(file) {
  const maxBytes = 50 * 1024 * 1024
  if (file.size > maxBytes) {
    ElMessage.error('文件大小不能超过 50MB')
    return false
  }
  return true
}

async function handleCreateFile() {
  const valid = await createFileFormRef.value.validate().catch(() => false)
  if (!valid || !createFile.value) return

  submitting.value = true
  try {
    const fd = new FormData()
    fd.append('file', createFile.value)
    fd.append('name', createForm.name)
    const res = await adminApi.shares.create(fd)
    ElMessage.success('分享订阅已创建')
    if (res.data.token) {
      ElMessage.info('Token: ' + res.data.token)
    }
    createVisible.value = false
    await fetchShares()
  } catch (e) {
    const msg = e.response?.data?.error || '创建失败'
    ElMessage.error(msg)
  } finally {
    submitting.value = false
  }
}

async function handleCreateText() {
  const valid = await createTextFormRef.value.validate().catch(() => false)
  if (!valid || !createForm.content.trim()) return

  submitting.value = true
  try {
    const res = await adminApi.shares.create({
      name: createForm.name,
      content: createForm.content
    })
    ElMessage.success('分享订阅已创建')
    if (res.data.token) {
      ElMessage.info('Token: ' + res.data.token)
    }
    createVisible.value = false
    await fetchShares()
  } catch (e) {
    const msg = e.response?.data?.error || '创建失败'
    ElMessage.error(msg)
  } finally {
    submitting.value = false
  }
}

// ==========================================================================
// Actions
// ==========================================================================
function goVersions(row) {
  router.push('/admin/shares/' + row.id + '/versions')
}

function copyShareLink(row) {
  const token = row.token
  if (!token) {
    ElMessage.warning('无法获取 Token')
    return
  }
  const url = downloadApi.shareDownloadUrl(row.id, token)
  // Build full URL from current origin
  const fullUrl = window.location.origin + url
  navigator.clipboard.writeText(fullUrl).then(() => {
    ElMessage.success('已复制到剪贴板')
  }).catch(() => {
    ElMessage.error('复制失败，请手动复制')
  })
}

function confirmRefreshToken(row) {
  actionTarget.value = row
  refreshVisible.value = true
}

async function handleRefreshToken() {
  if (!actionTarget.value) return
  try {
    const res = await adminApi.shares.refreshToken(actionTarget.value.id)
    // Update the token in local data
    if (res.data.token) {
      actionTarget.value.token = res.data.token
      actionTarget.value.has_token = true
    }
    ElMessage.success('Token 已刷新，旧链接立即失效')
    actionTarget.value = null
    await fetchShares()
  } catch (e) {
    const msg = e.response?.data?.error || '刷新失败'
    ElMessage.error(msg)
  }
}

function confirmRevokeToken(row) {
  actionTarget.value = row
  revokeVisible.value = true
}

async function handleRevokeToken() {
  if (!actionTarget.value) return
  try {
    await adminApi.shares.revokeToken(actionTarget.value.id)
    ElMessage.success('Token 已吊销，链接立即不可用')
    actionTarget.value = null
    await fetchShares()
  } catch (e) {
    const msg = e.response?.data?.error || '吊销失败'
    ElMessage.error(msg)
  }
}

function confirmDelete(row) {
  actionTarget.value = row
  deleteVisible.value = true
}

async function handleDelete() {
  if (!actionTarget.value) return
  try {
    await adminApi.shares.delete(actionTarget.value.id)
    ElMessage.success('分享订阅已删除')
    actionTarget.value = null
    await fetchShares()
  } catch (e) {
    const msg = e.response?.data?.error || '删除失败'
    ElMessage.error(msg)
  }
}

// ==========================================================================
// Lifecycle
// ==========================================================================
onMounted(async () => {
  loading.value = true
  await fetchShares()
  loading.value = false
})
</script>

<style scoped>
.share-list-container {
  padding: 0;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.page-header h2 {
  margin: 0;
  font-size: 20px;
  font-weight: 600;
}

.share-table {
  width: 100%;
}

.no-version {
  color: var(--el-text-color-secondary);
}
</style>
