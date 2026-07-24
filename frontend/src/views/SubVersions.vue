<template>
  <div>
    <!-- Loading -->
    <div v-if="loading" class="flex items-center justify-center py-12">
      <svg class="animate-spin h-5 w-5 mr-2 text-blue-600" fill="none" viewBox="0 0 24 24">
        <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
        <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/>
      </svg>
      <span class="text-gray-500 dark:text-gray-400">加载中...</span>
    </div>

    <template v-else>
      <div class="flex items-center gap-4 mb-5 flex-wrap">
        <button class="text-gray-600 dark:text-gray-400 hover:text-gray-800 dark:hover:text-gray-200 text-sm flex items-center gap-1" @click="goBack">
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 19l-7-7m0 0l7-7m-7 7h18"/></svg>
          返回订阅列表
        </button>
        <h2 v-if="subscription" class="m-0 text-xl font-semibold text-gray-900 dark:text-white">{{ subscription.name }} — 版本管理</h2>
        <button class="bg-blue-600 hover:bg-blue-700 text-white rounded-md px-3 py-1.5 text-sm flex items-center gap-1 ml-auto" @click="uploadVisible = true">
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4"/></svg>
          新建版本
        </button>
      </div>

      <div v-if="versions.length === 0" class="text-center py-12 text-gray-400 dark:text-gray-500">暂无版本</div>

      <div v-else class="grid grid-cols-1 md:grid-cols-2 gap-4">
        <div v-for="v in sortedVersions" :key="v.version"
             class="bg-white dark:bg-gray-800 rounded-lg shadow-md overflow-hidden"
             :class="{ 'border-l-4 border-l-green-500': isCurrent(v) }">
          <div class="px-4 py-3 flex items-center justify-between border-b border-gray-200 dark:border-gray-700">
            <span class="font-semibold text-gray-900 dark:text-white">v{{ v.version }}</span>
            <span v-if="isCurrent(v)" class="bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-300 text-xs rounded-full px-2 py-0.5">当前</span>
          </div>
          <div class="px-4 py-2 text-sm text-gray-500 dark:text-gray-400 space-y-1">
            <div>创建: {{ formatTime(v.created_at) }}</div>
            <div>更新: {{ formatTime(v.updated_at) }}</div>
          </div>
          <div class="px-4 py-3 border-t border-gray-200 dark:border-gray-700 flex gap-2 justify-end flex-wrap">
            <button v-if="!isCurrent(v)" @click="handleSwitch(v)" class="bg-blue-600 hover:bg-blue-700 text-white rounded-md px-3 py-1.5 text-sm">设为当前</button>
            <button @click="handlePreview(v)" class="bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-200 rounded-md px-3 py-1.5 text-sm">预览</button>
            <button @click="confirmDeleteVersion(v)" :disabled="isCurrent(v) || versions.length <= 1" class="bg-red-600 hover:bg-red-700 text-white rounded-md px-3 py-1.5 text-sm disabled:opacity-50 disabled:cursor-not-allowed">删除</button>
          </div>
        </div>
      </div>
    </template>

    <UploadModal v-model:visible="uploadVisible" :initial-content="editContent" @upload="onFileUpload" @textSave="onTextSave" />

    <el-dialog v-model="previewVisible" title="版本预览" :width="previewDialogWidth" :close-on-click-modal="false" :append-to-body="true">
      <pre class="bg-gray-100 dark:bg-gray-800 p-4 rounded-md text-sm overflow-auto max-h-96 max-md:max-h-[calc(100vh-120px)] text-gray-900 dark:text-gray-100 whitespace-pre-wrap break-all">{{ previewContent }}</pre>
      <template #footer>
        <div class="flex justify-end gap-2">
          <button class="bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-600 rounded-md px-4 py-2 text-sm" @click="previewVisible = false">关闭</button>
          <button class="bg-blue-600 hover:bg-blue-700 text-white rounded-md px-4 py-2 text-sm" @click="handleEditFromPreview">基于此版本编辑</button>
        </div>
      </template>
    </el-dialog>

    <ConfirmDialog v-model:visible="deleteVersionVisible" title="删除版本" :message="'确定删除版本 v' + (deleteVersionTarget?.version || '') + '？'" @confirm="handleDeleteVersion" />
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useToast } from '@/composables/useToast'
import { useDialogWidth } from '@/composables/useDialogWidth'
import { adminApi } from '@/services/api'
import UploadModal from '@/components/UploadModal.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'

const route = useRoute()
const router = useRouter()
const { success: toastSuccess, error: toastError } = useToast()
const previewDialogWidth = useDialogWidth('640px')

// ==========================================================================
// Data
// ==========================================================================
const loading = ref(true)
const subscription = ref(null)
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
  return subscription.value?.versions || []
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
async function fetchSubscription() {
  const id = route.params.id
  if (!id) {
    toastError('缺少订阅 ID')
    router.push('/admin/subscriptions')
    return
  }
  try {
    const res = await adminApi.subscriptions.get(id)
    subscription.value = res.data.subscription
  } catch (e) {
    toastError('加载订阅信息失败')
    router.push('/admin/subscriptions')
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
    await adminApi.subscriptions.uploadVersion(id, fd)
    toastSuccess('版本已上传')
    await fetchSubscription()
  } catch (e) {
    const msg = e.response?.data?.error || '上传失败'
    toastError(msg)
  }
}

async function onTextSave(content) {
  const id = route.params.id
  try {
    await adminApi.subscriptions.createVersionFromText(id, content)
    toastSuccess('新版本已创建')
    await fetchSubscription()
  } catch (e) {
    const msg = e.response?.data?.error || '创建失败'
    toastError(msg)
  }
}

// ==========================================================================
// Version Operations
// ==========================================================================
async function handleSwitch(v) {
  const id = route.params.id
  try {
    await adminApi.subscriptions.switchVersion(id, v.version)
    toastSuccess('已切换当前版本')
    await fetchSubscription()
  } catch (e) {
    const msg = e.response?.data?.error || '切换失败'
    toastError(msg)
  }
}

async function handlePreview(v) {
  const id = route.params.id
  try {
    const res = await adminApi.subscriptions.getVersion(id, v.version)
    previewContent.value = res.data.content || ''
    previewVisible.value = true
  } catch (e) {
    toastError('加载版本内容失败')
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
    await adminApi.subscriptions.deleteVersion(id, deleteVersionTarget.value.version)
    toastSuccess('版本已删除')
    deleteVersionTarget.value = null
    await fetchSubscription()
  } catch (e) {
    const msg = e.response?.data?.error || '删除失败'
    toastError(msg)
  }
}

// ==========================================================================
// Navigation
// ==========================================================================
function goBack() {
  router.push('/admin/subscriptions')
}

// ==========================================================================
// Lifecycle
// ==========================================================================
onMounted(async () => {
  loading.value = true
  await fetchSubscription()
  loading.value = false
})
</script>

<style scoped>
</style>
