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

      <!-- Card Grid -->
      <div v-else class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-5">
        <div
          v-for="rule in rules"
          :key="rule.id"
          class="bg-white dark:bg-gray-800 rounded-lg shadow-md overflow-hidden"
        >
          <div class="px-4 py-3 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between gap-2">
            <span class="text-base font-semibold text-gray-900 dark:text-white truncate">{{ rule.name }}</span>
            <span class="rounded-full px-2 py-0.5 text-xs font-medium shrink-0 bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300">{{ rule.client_type || 'Shadowrocket' }}</span>
          </div>
          <div class="p-4">
            <div class="text-sm text-gray-500 dark:text-gray-400 mb-3 space-y-1">
              <div class="flex items-center gap-2">
                <span class="text-gray-400 dark:text-gray-500">当前版本:</span>
                <span v-if="currentVersion(rule) !== null" class="text-gray-700 dark:text-gray-300">v{{ currentVersion(rule) }}</span>
                <span v-else class="text-gray-400 dark:text-gray-500 italic">—</span>
              </div>
              <div v-if="currentUpdatedAt(rule)" class="flex items-center gap-2">
                <span class="text-gray-400 dark:text-gray-500">更新于:</span>
                <span class="text-gray-700 dark:text-gray-300">{{ formatTime(currentUpdatedAt(rule)) }}</span>
              </div>

            </div>
            <div class="flex flex-wrap gap-1">
              <button class="bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-600 rounded-md px-3 py-1.5 text-sm" @click="goVersions(rule)">版本管理</button>
              <button class="bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-600 rounded-md px-3 py-1.5 text-sm disabled:opacity-50 disabled:cursor-not-allowed" :disabled="!rule.token" @click="copyDownloadLink(rule)">复制下载链接</button>
              <button class="bg-orange-500 hover:bg-orange-600 text-white rounded-md px-3 py-1.5 text-sm" @click="confirmRotateToken(rule)">轮替 Token</button>
              <button class="bg-red-600 hover:bg-red-700 text-white rounded-md px-3 py-1.5 text-sm" @click="confirmDelete(rule)">删除</button>
            </div>
          </div>
        </div>
      </div>
    </template>

    <!-- Create Dialog -->
    <el-dialog v-model="createVisible" title="创建规则" :width="dialogWidth" :close-on-click-modal="false" :append-to-body="true" @closed="resetCreateForm">
      <el-form ref="createFileFormRef" :model="createForm" :rules="createRules" label-position="top">
        <el-form-item label="名称" prop="name">
          <input v-model="createForm.name" placeholder="规则名称" class="w-full rounded-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-3 py-2 text-base text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none" @blur="createFileFormRef.validateField('name')" />
        </el-form-item>
        <el-form-item label="客户端类型" prop="client_type">
          <el-select v-model="createForm.client_type" class="w-full" placeholder="选择客户端类型" @change="onClientTypeChange">
            <el-option value="shadowrocket" label="Shadowrocket" />
          </el-select>
        </el-form-item>
        <el-form-item label="URL Schemes">
          <textarea v-model="createForm.schemesText" :rows="3" placeholder="每行一个 scheme，一键导入时使用第一个"
            class="w-full rounded-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-3 py-2 text-base text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none resize-y"></textarea>
          <div class="text-xs text-gray-400 dark:text-gray-500 mt-1">每行一个 URL Scheme，一键导入时使用第一个</div>
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
import { useDialogWidth } from '@/composables/useDialogWidth'
import { adminApi, publicApi } from '@/services/api'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import UploadTabs from '@/components/UploadTabs.vue'

const dialogWidth = useDialogWidth('520px')

const router = useRouter()
const { success: toastSuccess, error: toastError, warning: toastWarning } = useToast()

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
const createForm = reactive({ name: '', client_type: 'shadowrocket', schemesText: 'shadowrocket://config/add/', content: '' })
const createFileFormRef = ref(null)
const uploadTabsRef = ref(null)

const createRules = {
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
  createForm.name = ''
  createForm.client_type = 'shadowrocket'
  createForm.schemesText = 'shadowrocket://config/add/'
  createForm.content = ''
  createFile.value = null
  createFileSelected.value = false
  createTab.value = 'file'
  createFileFormRef.value?.clearValidate()
  uploadTabsRef.value?.clearFile()
}

function onClientTypeChange() {
  // Set default schemes based on client type
  if (createForm.client_type === 'shadowrocket') {
    createForm.schemesText = 'shadowrocket://config/add/'
  }
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
    fd.append('client_type', createForm.client_type)
    // Client schemes are passed as JSON for multipart; backend will parse
    const schemes = createForm.schemesText.split('\n').map(s => s.trim()).filter(Boolean)
    // For multipart, we pass schemes via JSON body approach — but since CreateRule
    // uses createRuleWithFirstVersion which accepts nil schemes, the file upload
    // path defaults to ["shadowrocket://config/add/"]. To customize, admin should
    // use the JSON text creation path or edit the rule after creation.
    const res = await adminApi.rules.create(fd)
    toastSuccess('规则已创建')
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
    const schemes = createForm.schemesText.split('\n').map(s => s.trim()).filter(Boolean)
    const res = await adminApi.rules.create({
      name: createForm.name,
      client_type: createForm.client_type,
      client_schemes: schemes,
      content: createForm.content
    })
    toastSuccess('规则已创建')
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
