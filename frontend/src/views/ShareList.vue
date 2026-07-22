<template>
  <div>
    <div class="flex justify-between items-center mb-5 flex-wrap gap-3">
      <h2 class="m-0 text-xl font-semibold text-gray-900 dark:text-white">分享订阅</h2>
      <button class="bg-blue-600 hover:bg-blue-700 text-white rounded-md px-3 py-1.5 text-sm flex items-center gap-1" @click="openCreateDialog">
        <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4"/></svg>
        创建分享订阅
      </button>
    </div>

    <div v-if="!loading && shares.length === 0" class="text-center py-12 text-gray-400 dark:text-gray-500">暂无分享订阅，请创建</div>

    <el-table v-else :data="shares" stripe>
      <el-table-column prop="name" label="名称" min-width="160" />
      <el-table-column label="创建时间" width="180"><template #default="{ row }">{{ formatTime(row.created_at) }}</template></el-table-column>
      <el-table-column label="当前版本" width="100">
        <template #default="{ row }">
          <span v-if="currentVersion(row) !== null">v{{ currentVersion(row) }}</span>
          <span v-else class="text-gray-400 dark:text-gray-500 italic">—</span>
        </template>
      </el-table-column>
      <el-table-column label="Token 状态" width="110">
        <template #default="{ row }">
          <span class="rounded-full px-2 py-0.5 text-xs font-medium" :class="row.has_token ? 'bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-300' : 'bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-300'">{{ row.has_token ? '有效' : '已吊销' }}</span>
        </template>
      </el-table-column>
      <el-table-column label="操作" min-width="320" fixed="right">
        <template #default="{ row }">
          <button class="bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-600 rounded-md px-3 py-1.5 text-xs" @click="goVersions(row)">版本管理</button>
          <button class="bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-600 rounded-md px-3 py-1.5 text-xs ml-1 disabled:opacity-50 disabled:cursor-not-allowed" :disabled="!row.has_token" @click="copyShareLink(row)">复制分享链接</button>
          <button class="bg-orange-500 hover:bg-orange-600 text-white rounded-md px-3 py-1.5 text-xs ml-1" @click="confirmRefreshToken(row)">刷新 Token</button>
          <button class="bg-red-600 hover:bg-red-700 text-white rounded-md px-3 py-1.5 text-xs ml-1 disabled:opacity-50 disabled:cursor-not-allowed" :disabled="!row.has_token" @click="confirmRevokeToken(row)">吊销 Token</button>
          <button class="bg-red-600 hover:bg-red-700 text-white rounded-md px-3 py-1.5 text-xs ml-1" @click="confirmDelete(row)">删除</button>
        </template>
      </el-table-column>
    </el-table>

    <!-- Create Dialog -->
    <el-dialog v-model="createVisible" title="创建分享订阅" width="520px" :close-on-click-modal="false" @closed="resetCreateForm">
      <el-form ref="createFileFormRef" :model="createForm" :rules="createRules" label-position="top">
        <el-form-item label="名称" prop="name">
          <input v-model="createForm.name" placeholder="分享订阅名称" class="w-full rounded-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-3 py-2 text-sm text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none" @blur="createFileFormRef.validateField('name')" />
        </el-form-item>
      </el-form>
      <UploadTabs
        ref="uploadTabsRef"
        v-model="createTab"
        v-model:textContent="createForm.content"
        @file-change="onCreateFileChange"
        @clear-file="createFileSelected = false; createFile = null"
      />
      <template #footer>
        <div class="flex justify-end gap-2">
          <button class="bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-600 rounded-md px-4 py-2 text-sm" @click="createVisible = false">取消</button>
          <button v-if="createTab === 'file'" class="bg-blue-600 hover:bg-blue-700 text-white rounded-md px-4 py-2 text-sm disabled:opacity-50" :disabled="!createFileSelected" @click="handleCreateFile">创建并上传</button>
          <button v-else class="bg-blue-600 hover:bg-blue-700 text-white rounded-md px-4 py-2 text-sm disabled:opacity-50" :disabled="!createForm.content.trim()" @click="handleCreateText">创建</button>
        </div>
      </template>
    </el-dialog>

    <ConfirmDialog v-model:visible="refreshVisible" title="刷新 Token" message="刷新后旧链接立即失效，确定？" @confirm="handleRefreshToken" />
    <ConfirmDialog v-model:visible="revokeVisible" title="吊销 Token" message="吊销后该分享链接立即不可用，订阅文件保留。确定？" @confirm="handleRevokeToken" />
    <ConfirmDialog v-model:visible="deleteVisible" title="删除分享订阅" message="确定删除？将级联删除所有版本文件和 Token。" @confirm="handleDelete" />
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useToast } from '@/composables/useToast'
import { adminApi, downloadApi } from '@/services/api'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import UploadTabs from '@/components/UploadTabs.vue'

const router = useRouter()
const { success: toastSuccess, error: toastError, info: toastInfo, warning: toastWarning } = useToast()

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
const uploadTabsRef = ref(null)

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
    toastError('加载分享订阅列表失败')
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
  createFileFormRef.value?.clearValidate()
  uploadTabsRef.value?.clearFile()
}

function onCreateFileChange(file) {
  createFile.value = file
  createFileSelected.value = true
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
    toastSuccess('分享订阅已创建')
    if (res.data.token) {
      toastInfo('Token: ' + res.data.token)
    }
    createVisible.value = false
    await fetchShares()
  } catch (e) {
    const msg = e.response?.data?.error || '创建失败'
    toastError(msg)
  } finally {
    submitting.value = false
  }
}

async function handleCreateText() {
  const valid = await createFileFormRef.value.validate().catch(() => false)
  if (!valid || !createForm.content.trim()) return

  submitting.value = true
  try {
    const res = await adminApi.shares.create({
      name: createForm.name,
      content: createForm.content
    })
    toastSuccess('分享订阅已创建')
    if (res.data.token) {
      toastInfo('Token: ' + res.data.token)
    }
    createVisible.value = false
    await fetchShares()
  } catch (e) {
    const msg = e.response?.data?.error || '创建失败'
    toastError(msg)
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
    toastWarning('无法获取 Token')
    return
  }
  const url = downloadApi.shareDownloadUrl(row.id, token)
  // Build full URL from current origin
  const fullUrl = window.location.origin + url
  navigator.clipboard.writeText(fullUrl).then(() => {
    toastSuccess('已复制到剪贴板')
  }).catch(() => {
    toastError('复制失败，请手动复制')
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
    toastSuccess('Token 已刷新，旧链接立即失效')
    actionTarget.value = null
    await fetchShares()
  } catch (e) {
    const msg = e.response?.data?.error || '刷新失败'
    toastError(msg)
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
    toastSuccess('Token 已吊销，链接立即不可用')
    actionTarget.value = null
    await fetchShares()
  } catch (e) {
    const msg = e.response?.data?.error || '吊销失败'
    toastError(msg)
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
    toastSuccess('分享订阅已删除')
    actionTarget.value = null
    await fetchShares()
  } catch (e) {
    const msg = e.response?.data?.error || '删除失败'
    toastError(msg)
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
  display: block;
  min-width: 100%;
  width: 100%;
  padding: 0;
  box-sizing: border-box;
  overflow: hidden;
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
