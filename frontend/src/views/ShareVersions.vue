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
      <div class="flex items-center gap-4 mb-5 flex-wrap">
        <button class="text-gray-600 dark:text-gray-400 hover:text-gray-800 dark:hover:text-gray-200 text-sm flex items-center gap-1" @click="goBack">
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 19l-7-7m0 0l7-7m-7 7h18"/></svg>
          返回分享列表
        </button>
        <h2 v-if="share" class="m-0 text-xl font-semibold text-gray-900 dark:text-white">{{ share.name }} — 版本管理</h2>
        <button class="bg-blue-600 hover:bg-blue-700 text-white rounded-md px-3 py-1.5 text-sm flex items-center gap-1 ml-auto" @click="uploadVisible = true">
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4"/></svg>
          新建版本
        </button>
      </div>

      <div v-if="versions.length === 0" class="text-center py-12 text-gray-400 dark:text-gray-500">暂无版本</div>

      <el-table v-else :data="sortedVersions" stripe>
        <el-table-column label="版本号" width="100"><template #default="{ row }">v{{ row.version }}</template></el-table-column>
        <el-table-column v-if="!isMobile" label="创建时间" width="180"><template #default="{ row }">{{ formatTime(row.created_at) }}</template></el-table-column>
        <el-table-column label="更新时间" width="180"><template #default="{ row }">{{ formatTime(row.updated_at) }}</template></el-table-column>
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <span v-if="isCurrent(row)" class="rounded-full px-2 py-0.5 text-xs font-medium bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-300">当前</span>
            <span v-else class="text-gray-400 dark:text-gray-500">—</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="80" min-width="80" fixed="right">
          <template #default="{ row }">
            <ActionMenu>
              <template #default>
                <button v-if="!isCurrent(row)" class="bg-blue-600 hover:bg-blue-700 text-white rounded-md px-3 py-1.5 text-xs" @click="handleSwitch(row)">设为当前</button>
                <button class="bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-600 rounded-md px-3 py-1.5 text-xs ml-1" @click="handlePreview(row)">预览</button>
                <button class="bg-red-600 hover:bg-red-700 text-white rounded-md px-3 py-1.5 text-xs ml-1 disabled:opacity-50 disabled:cursor-not-allowed" :disabled="isCurrent(row) || versions.length <= 1" @click="confirmDeleteVersion(row)">删除</button>
              </template>
              <template #menu>
                <button v-if="!isCurrent(row)" class="block w-full text-left px-4 py-2 text-sm text-blue-600 hover:bg-gray-100 dark:hover:bg-gray-600" @click="handleSwitch(row)">设为当前</button>
                <button class="block w-full text-left px-4 py-2 text-sm text-gray-700 dark:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-600" @click="handlePreview(row)">预览</button>
                <button class="block w-full text-left px-4 py-2 text-sm text-red-600 hover:bg-gray-100 dark:hover:bg-gray-600 disabled:opacity-50" :disabled="isCurrent(row) || versions.length <= 1" @click="confirmDeleteVersion(row)">删除</button>
              </template>
            </ActionMenu>
          </template>
        </el-table-column>
      </el-table>
    </template>

    <UploadModal v-model:visible="uploadVisible" :initial-content="editContent" @upload="onFileUpload" @textSave="onTextSave" />

    <el-dialog v-model="previewVisible" title="版本预览" width="640px" :close-on-click-modal="false">
      <pre class="bg-gray-100 dark:bg-gray-800 p-4 rounded-md text-sm overflow-auto max-h-96 text-gray-900 dark:text-gray-100">{{ previewContent }}</pre>
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
import { adminApi } from '@/services/api'
import UploadModal from '@/components/UploadModal.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import ActionMenu from '@/components/ActionMenu.vue'
import { useIsMobile } from '@/composables/useIsMobile'

const route = useRoute()
const router = useRouter()
const { success: toastSuccess, error: toastError } = useToast()
const isMobile = useIsMobile()


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
    toastError('缺少分享订阅 ID')
    router.push('/admin/shares')
    return
  }
  try {
    const res = await adminApi.shares.get(id)
    share.value = res.data.share
  } catch (e) {
    toastError('加载分享订阅信息失败')
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
    toastSuccess('版本已上传')
    await fetchShare()
  } catch (e) {
    const msg = e.response?.data?.error || '上传失败'
    toastError(msg)
  }
}

async function onTextSave(content) {
  const id = route.params.id
  try {
    await adminApi.shares.createVersionFromText(id, content)
    toastSuccess('新版本已创建')
    await fetchShare()
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
    await adminApi.shares.switchVersion(id, v.version)
    toastSuccess('已切换当前版本')
    await fetchShare()
  } catch (e) {
    const msg = e.response?.data?.error || '切换失败'
    toastError(msg)
  }
}

async function handlePreview(v) {
  const id = route.params.id
  try {
    const res = await adminApi.shares.getVersion(id, v.version)
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
    await adminApi.shares.deleteVersion(id, deleteVersionTarget.value.version)
    toastSuccess('版本已删除')
    deleteVersionTarget.value = null
    await fetchShare()
  } catch (e) {
    const msg = e.response?.data?.error || '删除失败'
    toastError(msg)
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
</style>
