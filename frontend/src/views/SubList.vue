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
      <div class="flex justify-between items-center mb-5 flex-wrap gap-3">
        <h2 class="m-0 text-xl font-semibold text-gray-900 dark:text-white">订阅管理</h2>
        <button class="bg-blue-600 hover:bg-blue-700 text-white rounded-md px-3 py-1.5 text-sm flex items-center gap-1" @click="openCreateDialog">
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4"/>
          </svg>
          创建订阅
        </button>
      </div>

      <!-- Empty -->
      <div v-if="subscriptions.length === 0" class="text-center py-12 text-gray-400 dark:text-gray-500">
        暂无订阅，请创建
      </div>

      <!-- Table -->
      <el-table v-else :data="sortedSubscriptions" stripe>
        <el-table-column prop="name" label="名称" min-width="160" />
        <el-table-column prop="platform" label="平台" width="140" />
        <el-table-column label="类型" width="100">
          <template #default="{ row }">
            <span class="rounded-full px-2 py-0.5 text-xs font-medium"
              :class="row.type === 'default' ? 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300' : 'bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300'">
              {{ row.type === 'default' ? '默认' : '高级' }}
            </span>
          </template>
        </el-table-column>
        <el-table-column label="当前版本" width="100">
          <template #default="{ row }">
            <span v-if="currentVersion(row) !== null">v{{ currentVersion(row) }}</span>
            <span v-else class="text-gray-400 dark:text-gray-500 italic">—</span>
          </template>
        </el-table-column>
        <el-table-column label="更新时间" width="180">
          <template #default="{ row }">
            <span v-if="currentUpdatedAt(row)">{{ formatTime(currentUpdatedAt(row)) }}</span>
            <span v-else class="text-gray-400 dark:text-gray-500 italic">—</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="260" fixed="right">
          <template #default="{ row }">
            <button class="bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-600 rounded-md px-3 py-1.5 text-xs" @click="goVersions(row)">版本管理</button>
            <button class="bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-600 rounded-md px-3 py-1.5 text-xs ml-1" @click="openEditDialog(row)">编辑</button>
            <button class="bg-red-600 hover:bg-red-700 text-white rounded-md px-3 py-1.5 text-xs ml-1" @click="confirmDelete(row)">删除</button>
          </template>
        </el-table-column>
      </el-table>
    </template>

    <!-- Create / Edit Dialog -->
    <el-dialog
      v-model="dialogVisible"
      :title="isEditing ? '编辑订阅' : '创建订阅'"
      width="480px"
      :close-on-click-modal="false"
      @closed="resetForm"
    >
      <el-form ref="formRef" :model="form" :rules="formRules" label-position="top">
        <el-form-item label="名称" prop="name">
          <input v-model="form.name" placeholder="订阅名称"
            class="w-full rounded-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-3 py-2 text-sm text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none"
            @blur="formRef.validateField('name')" />
        </el-form-item>
        <el-form-item label="类型" prop="type">
          <select v-model="form.type"
            class="w-full rounded-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-3 py-2 text-sm text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none"
            @change="formRef.validateField('type')">
            <option value="default">默认 (default)</option>
            <option value="advanced">高级 (advanced)</option>
          </select>
        </el-form-item>
        <el-form-item label="平台" prop="platform">
          <select v-model="form.platform"
            class="w-full rounded-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-3 py-2 text-sm text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none"
            @change="formRef.validateField('platform')">
            <option v-for="p in platforms" :key="p.id" :value="p.id">{{ p.name }}</option>
          </select>
        </el-form-item>
      </el-form>
      <template #footer>
        <div class="flex justify-end gap-2">
          <button class="bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-600 rounded-md px-4 py-2 text-sm" @click="dialogVisible = false">取消</button>
          <button class="bg-blue-600 hover:bg-blue-700 text-white rounded-md px-4 py-2 text-sm disabled:opacity-50" :disabled="submitting" @click="handleSubmit">
            {{ isEditing ? '保存' : '创建' }}
          </button>
        </div>
      </template>
    </el-dialog>

    <!-- Delete Confirm -->
    <ConfirmDialog
      v-model:visible="deleteVisible"
      title="删除订阅"
      message="确定删除该订阅？将级联删除所有版本文件和下载 Token。"
      @confirm="handleDelete"
    />
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useToast } from '@/composables/useToast'
import { adminApi } from '@/services/api'
import ConfirmDialog from '@/components/ConfirmDialog.vue'

const router = useRouter()
const { success: toastSuccess, error: toastError } = useToast()

// ==========================================================================
// Data
// ==========================================================================
const loading = ref(true)
const subscriptions = ref([])
const platforms = ref([])

const dialogVisible = ref(false)
const isEditing = ref(false)
const editingId = ref('')
const submitting = ref(false)
const formRef = ref(null)

const deleteVisible = ref(false)
const deleteTarget = ref(null)

const form = reactive({
  id: '',
  name: '',
  type: 'default',
  platform: ''
})

// ==========================================================================
// Validation Rules
// ==========================================================================
const formRules = computed(() => ({
  name: [{ required: true, message: '请输入名称', trigger: 'blur' }],
  type: [{ required: true, message: '请选择类型', trigger: 'change' }],
  platform: [{ required: true, message: '请选择平台', trigger: 'change' }]
}))

// ==========================================================================
// Computed
// ==========================================================================
const sortedSubscriptions = computed(() => {
  return [...subscriptions.value].sort((a, b) => {
    if (a.platform !== b.platform) return a.platform.localeCompare(b.platform)
    if (a.type === 'default' && b.type === 'advanced') return -1
    if (a.type === 'advanced' && b.type === 'default') return 1
    return 0
  })
})

// ==========================================================================
// Helpers
// ==========================================================================
function currentVersion(sub) {
  if (!sub.versions || sub.versions.length === 0) return null
  const sorted = [...sub.versions].sort(
    (a, b) => new Date(b.updated_at || 0) - new Date(a.updated_at || 0)
  )
  return sorted[0]?.version ?? null
}

function currentUpdatedAt(sub) {
  if (!sub.versions || sub.versions.length === 0) return null
  const sorted = [...sub.versions].sort(
    (a, b) => new Date(b.updated_at || 0) - new Date(a.updated_at || 0)
  )
  return sorted[0]?.updated_at ?? null
}

function formatTime(t) {
  if (!t) return ''
  return new Date(t).toLocaleString()
}

// ==========================================================================
// Data Loading
// ==========================================================================
async function fetchSubscriptions() {
  try {
    const res = await adminApi.subscriptions.list()
    subscriptions.value = res.data.subscriptions || []
  } catch (e) {
    toastError('加载订阅列表失败')
  }
}

async function fetchPlatforms() {
  try {
    const res = await adminApi.platforms.list()
    platforms.value = res.data.platforms || []
  } catch (e) {
    // Platforms dropdown failure is non-critical
  }
}

// ==========================================================================
// Create / Edit
// ==========================================================================
function openCreateDialog() {
  isEditing.value = false
  editingId.value = ''
  resetForm()
  dialogVisible.value = true
}

function openEditDialog(row) {
  isEditing.value = true
  editingId.value = row.id
  form.id = row.id
  form.name = row.name
  form.type = row.type
  form.platform = row.platform
  dialogVisible.value = true
}

function resetForm() {
  form.id = ''
  form.name = ''
  form.type = 'default'
  form.platform = ''
  formRef.value?.clearValidate()
}

async function handleSubmit() {
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) return

  submitting.value = true
  try {
    if (isEditing.value) {
      await adminApi.subscriptions.update(editingId.value, {
        name: form.name,
        platform: form.platform,
        type: form.type
      })
      toastSuccess('订阅已更新')
    } else {
      await adminApi.subscriptions.create({
        name: form.name,
        type: form.type,
        platform: form.platform
      })
      toastSuccess('订阅已创建')
    }
    dialogVisible.value = false
    await fetchSubscriptions()
  } catch (e) {
    const msg = e.response?.data?.error || '操作失败'
    toastError(msg)
  } finally {
    submitting.value = false
  }
}

// ==========================================================================
// Delete
// ==========================================================================
function confirmDelete(row) {
  deleteTarget.value = row
  deleteVisible.value = true
}

async function handleDelete() {
  if (!deleteTarget.value) return
  try {
    await adminApi.subscriptions.delete(deleteTarget.value.id)
    toastSuccess('订阅已删除')
    deleteTarget.value = null
    await fetchSubscriptions()
  } catch (e) {
    const msg = e.response?.data?.error || '删除失败'
    toastError(msg)
  }
}

// ==========================================================================
// Navigation
// ==========================================================================
function goVersions(row) {
  router.push('/admin/subscriptions/' + row.id + '/versions')
}

// ==========================================================================
// Lifecycle
// ==========================================================================
onMounted(async () => {
  loading.value = true
  await Promise.all([fetchSubscriptions(), fetchPlatforms()])
  loading.value = false
})
</script>

<style scoped>
.sub-list-container {
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

.sub-table {
  width: 100%;
}

.no-version {
  color: var(--el-text-color-secondary);
}
</style>
