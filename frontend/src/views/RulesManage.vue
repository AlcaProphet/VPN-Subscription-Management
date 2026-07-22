<template>
  <div>
    <div v-if="loading" class="flex items-center justify-center py-12">
      <svg class="animate-spin h-5 w-5 mr-2 text-blue-600" fill="none" viewBox="0 0 24 24">
        <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
        <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/>
      </svg>
      <span class="text-gray-500 dark:text-gray-400">加载中...</span>
    </div>

    <template v-else>
      <div class="flex justify-between items-center mb-5 flex-wrap gap-3">
        <h2 class="m-0 text-xl font-semibold text-gray-900 dark:text-white">规则管理</h2>
        <button class="bg-blue-600 hover:bg-blue-700 text-white rounded-md px-3 py-1.5 text-sm flex items-center gap-1" @click="openCreateDialog">
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4"/></svg>
          创建规则
        </button>
      </div>

      <div v-if="rules.length === 0" class="text-center py-12 text-gray-400 dark:text-gray-500">暂无规则，请创建</div>

      <el-table v-else :data="rules" stripe>
        <el-table-column prop="name" label="规则名称" min-width="160" />
        <el-table-column label="客户端类型" width="130">
          <template #default="{ row }">
            <span class="rounded-full px-2 py-0.5 text-xs font-medium bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300">{{ row.client_type || 'Shadowrocket' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="当前版本" width="100">
          <template #default="{ row }">
            <span v-if="currentVersion(row) !== null">v{{ currentVersion(row) }}</span>
            <span v-else class="text-gray-400 dark:text-gray-500 italic">—</span>
          </template>
        </el-table-column>
        <el-table-column v-if="!isMobile" label="更新时间" width="180">
          <template #default="{ row }">
            <span v-if="currentUpdatedAt(row)">{{ formatTime(currentUpdatedAt(row)) }}</span>
            <span v-else class="text-gray-400 dark:text-gray-500 italic">—</span>
          </template>
        </el-table-column>
        <el-table-column v-if="!isMobile" label="Token" width="140">
          <template #default="{ row }">
            <span v-if="row.token" class="text-xs text-gray-500 dark:text-gray-400">{{ maskToken(row.token) }}</span>
            <span v-else class="text-gray-400 dark:text-gray-500 italic">—</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="80" min-width="80" fixed="right">
          <template #default="{ row }">
            <ActionMenu>
              <template #default>
                <button class="bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-600 rounded-md px-3 py-1.5 text-xs" @click="goVersions(row)">版本管理</button>
                <button class="bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-600 rounded-md px-3 py-1.5 text-xs ml-1 disabled:opacity-50 disabled:cursor-not-allowed" :disabled="!row.token" @click="copyDownloadLink(row)">复制下载链接</button>
                <button class="bg-orange-500 hover:bg-orange-600 text-white rounded-md px-3 py-1.5 text-xs ml-1" @click="confirmRotateToken(row)">轮替 Token</button>
                <button class="bg-red-600 hover:bg-red-700 text-white rounded-md px-3 py-1.5 text-xs ml-1" @click="confirmDelete(row)">删除</button>
              </template>
              <template #menu>
                <button class="block w-full text-left px-4 py-2 text-sm text-gray-700 dark:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-600" @click="goVersions(row)">版本管理</button>
                <button class="block w-full text-left px-4 py-2 text-sm text-gray-700 dark:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-600" :disabled="!row.token" @click="copyDownloadLink(row)">复制下载链接</button>
                <button class="block w-full text-left px-4 py-2 text-sm text-orange-600 hover:bg-gray-100 dark:hover:bg-gray-600" @click="confirmRotateToken(row)">轮替 Token</button>
                <button class="block w-full text-left px-4 py-2 text-sm text-red-600 hover:bg-gray-100 dark:hover:bg-gray-600" @click="confirmDelete(row)">删除</button>
              </template>
            </ActionMenu>
          </template>
        </el-table-column>
      </el-table>
    </template>

    <!-- Create Dialog -->
    <el-dialog v-model="createVisible" title="创建规则" width="520px" :close-on-click-modal="false" @closed="resetCreateForm">
      <el-form ref="createFileFormRef" :model="createForm" :rules="createRules" label-position="top">
        <el-form-item label="ID" prop="id">
          <input v-model="createForm.id" placeholder="小写字母、数字和连字符" class="w-full rounded-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-3 py-2 text-sm text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none" @blur="createFileFormRef.validateField('id')" />
        </el-form-item>
        <el-form-item label="名称" prop="name">
          <input v-model="createForm.name" placeholder="规则名称" class="w-full rounded-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-3 py-2 text-sm text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none" @blur="createFileFormRef.validateField('name')" />
        </el-form-item>
        <el-form-item label="客户端类型" prop="client_type">
          <select v-model="createForm.client_type" class="w-full rounded-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-3 py-2 text-sm text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none" @change="createFileFormRef.validateField('client_type')">
            <option value="shadowrocket">Shadowrocket</option>
          </select>
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

    <ConfirmDialog v-model:visible="rotateVisible" title="轮替 Token" message="轮替后旧链接立即失效，确定？" @confirm="handleRotateToken" />
    <ConfirmDialog v-model:visible="deleteVisible" title="删除规则" message="确定删除？将级联删除所有版本文件和 Token。" @confirm="handleDelete" />
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useToast } from '@/composables/useToast'
import { adminApi, publicApi } from '@/services/api'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import UploadTabs from '@/components/UploadTabs.vue'
import ActionMenu from '@/components/ActionMenu.vue'
import { useIsMobile } from '@/composables/useIsMobile'

const router = useRouter()
const { success: toastSuccess, error: toastError, info: toastInfo, warning: toastWarning } = useToast()
const isMobile = useIsMobile()

// ==========================================================================
// Data
// ==========================================================================
const loading = ref(true)
const rules = ref([])

// Create
const createVisible = ref(false)
const createTab = ref('file')
const submitting = ref(false)
const createFileSelected = ref(false)
const createFile = ref(null)
const createForm = reactive({ id: '', name: '', client_type: 'shadowrocket', content: '' })
const createFileFormRef = ref(null)
const uploadTabsRef = ref(null)

const createRules = {
  id: [
    { required: true, message: '请输入 ID', trigger: 'blur' },
    { pattern: /^[a-z0-9-]+$/, message: 'ID 只能包含小写字母、数字和连字符', trigger: 'blur' }
  ],
  name: [{ required: true, message: '请输入名称', trigger: 'blur' }]
}

// Actions
const rotateVisible = ref(false)
const deleteVisible = ref(false)
const actionTarget = ref(null)

// ==========================================================================
// Helpers
// ==========================================================================
function currentVersion(rule) {
  if (!rule.versions || rule.versions.length === 0) return null
  const sorted = [...rule.versions].sort(
    (a, b) => new Date(b.updated_at || 0) - new Date(a.updated_at || 0)
  )
  return sorted[0]?.version ?? null
}

function currentUpdatedAt(rule) {
  if (!rule.versions || rule.versions.length === 0) return null
  const sorted = [...rule.versions].sort(
    (a, b) => new Date(b.updated_at || 0) - new Date(a.updated_at || 0)
  )
  return sorted[0]?.updated_at ?? null
}

function maskToken(token) {
  if (!token || token.length <= 8) return token
  return token.substring(0, 8) + '...'
}

function formatTime(t) {
  if (!t) return ''
  return new Date(t).toLocaleString()
}

// ==========================================================================
// Data Loading
// ==========================================================================
async function fetchRules() {
  try {
    const res = await adminApi.rules.list()
    rules.value = res.data.rules || []
  } catch (e) {
    toastError('加载规则列表失败')
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
  createForm.id = ''
  createForm.name = ''
  createForm.client_type = 'shadowrocket'
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
    fd.append('id', createForm.id)
    fd.append('name', createForm.name)
    fd.append('client_type', createForm.client_type)
    const res = await adminApi.rules.create(fd)
    toastSuccess('规则已创建')
    if (res.data.token) {
      toastInfo('Token: ' + res.data.token)
    }
    createVisible.value = false
    await fetchRules()
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
    const res = await adminApi.rules.create({
      id: createForm.id,
      name: createForm.name,
      client_type: createForm.client_type,
      content: createForm.content
    })
    toastSuccess('规则已创建')
    if (res.data.token) {
      toastInfo('Token: ' + res.data.token)
    }
    createVisible.value = false
    await fetchRules()
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
  router.push('/admin/rules/' + row.id + '/versions')
}

function copyDownloadLink(row) {
  if (!row.token) return
  const url = publicApi.getRuleDownloadUrl(row.id, row.token)
  const fullUrl = window.location.origin + url
  navigator.clipboard.writeText(fullUrl).then(() => {
    toastSuccess('已复制到剪贴板')
  }).catch(() => {
    toastError('复制失败，请手动复制')
  })
}

function confirmRotateToken(row) {
  actionTarget.value = row
  rotateVisible.value = true
}

async function handleRotateToken() {
  if (!actionTarget.value) return
  try {
    const res = await adminApi.rules.refreshToken(actionTarget.value.id)
    if (res.data.token) {
      actionTarget.value.token = res.data.token
    }
    toastSuccess('Token 已轮替，旧链接立即失效')
    actionTarget.value = null
    await fetchRules()
  } catch (e) {
    const msg = e.response?.data?.error || '轮替失败'
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
    await adminApi.rules.delete(actionTarget.value.id)
    toastSuccess('规则已删除')
    actionTarget.value = null
    await fetchRules()
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
  await fetchRules()
  loading.value = false
})
</script>

<style scoped>
</style>
