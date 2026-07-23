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
        <h2 class="m-0 text-xl font-semibold text-gray-900 dark:text-white">平台管理</h2>
        <button class="bg-blue-600 hover:bg-blue-700 text-white rounded-md px-3 py-1.5 text-sm flex items-center gap-1" @click="openCreateDialog">
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4"/></svg>
          创建平台
        </button>
      </div>

      <div v-if="platforms.length === 0" class="text-center py-12 text-gray-400 dark:text-gray-500">暂无平台</div>

      <!-- Card Grid -->
      <div v-else class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-5">
        <div
          v-for="p in platforms"
          :key="p.id"
          class="bg-white dark:bg-gray-800 rounded-lg shadow-md overflow-hidden"
        >
          <div class="px-4 py-3 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between gap-2">
            <span class="text-base font-semibold text-gray-900 dark:text-white truncate">{{ p.name }}</span>
            <span class="rounded-full px-2 py-0.5 text-xs font-medium shrink-0 bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300">{{ p.id }}</span>
          </div>
          <div class="p-4">
            <div class="text-sm text-gray-500 dark:text-gray-400 mb-3 space-y-2">
              <p v-if="p.description" class="m-0 text-gray-700 dark:text-gray-300">{{ p.description }}</p>
              <div v-if="p.client_schemes && p.client_schemes.length > 0" class="flex flex-wrap gap-1">
                <span v-for="(scheme, idx) in p.client_schemes" :key="idx" class="inline-block rounded-full px-2 py-0.5 text-xs font-medium bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300">{{ scheme }}</span>
              </div>
              <div v-if="p.download_url" class="flex items-center gap-2">
                <span class="text-gray-400 dark:text-gray-500 shrink-0">下载:</span>
                <span class="text-gray-700 dark:text-gray-300 truncate text-xs">{{ p.download_url }}</span>
              </div>
            </div>
            <div class="flex flex-wrap gap-1">
              <button class="bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-600 rounded-md px-3 py-1.5 text-sm" @click="openEditDialog(p)">编辑</button>
              <button class="bg-red-600 hover:bg-red-700 text-white rounded-md px-3 py-1.5 text-sm" @click="confirmDelete(p)">删除</button>
            </div>
          </div>
        </div>
      </div>
    </template>

    <el-dialog v-model="dialogVisible" :title="isEditing ? '编辑平台' : '创建平台'" :width="dialogWidth" :close-on-click-modal="false" :append-to-body="true" @closed="resetForm">
      <div :class="{ 'max-h-[calc(100vh-200px)] overflow-y-auto': isMobile }">
        <el-form ref="formRef" :model="form" :rules="formRules" label-position="top">
        <el-form-item v-if="isEditing" label="ID">
          <span class="text-sm text-gray-500 dark:text-gray-400">{{ form.id }}</span>
        </el-form-item>
        <el-form-item label="名称" prop="name">
          <input v-model="form.name" placeholder="平台名称"
            class="w-full rounded-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-3 py-2 text-base text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none"
            @blur="formRef.validateField('name')" />
        </el-form-item>
        <el-form-item label="描述" prop="description">
          <textarea v-model="form.description" :rows="2" placeholder="平台描述"
            class="w-full rounded-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-3 py-2 text-base text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none resize-y"></textarea>
        </el-form-item>
        <el-form-item label="Client Schemes" prop="schemesText">
          <textarea v-model="form.schemesText" :rows="4" placeholder="每行一个 scheme"
            class="w-full rounded-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-3 py-2 text-base text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none resize-y"></textarea>
          <div class="text-xs text-gray-400 dark:text-gray-500 mt-1">每行一个 Client Scheme，一键导入时使用第一个</div>
        </el-form-item>
        <el-form-item label="下载链接" prop="download_url">
          <input v-model="form.download_url" placeholder="https://example.com/download（可选）"
            class="w-full rounded-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-3 py-2 text-base text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none" />
          <div class="text-xs text-gray-400 dark:text-gray-500 mt-1">可选，配置后在首页显示「下载客户端」按钮</div>
        </el-form-item>
      </el-form>
      </div>
      <template #footer>
        <div class="flex justify-end gap-2">
          <button class="bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-600 rounded-md px-4 py-2 text-sm" @click="dialogVisible = false">取消</button>
          <button class="bg-blue-600 hover:bg-blue-700 text-white rounded-md px-4 py-2 text-sm disabled:opacity-50" :disabled="submitting" @click="handleSubmit">{{ isEditing ? '保存' : '创建' }}</button>
        </div>
      </template>
    </el-dialog>

    <ConfirmDialog v-model:visible="deleteVisible" title="删除平台" message="确定删除该平台？将级联删除该平台的所有订阅、下载 Token 和自定义订阅。此操作不可恢复！" @confirm="handleDelete" />
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { useToast } from '@/composables/useToast'
import { useDialogWidth } from '@/composables/useDialogWidth'
import { useIsMobile } from '@/composables/useIsMobile'
import { adminApi } from '@/services/api'
import ConfirmDialog from '@/components/ConfirmDialog.vue'

const dialogWidth = useDialogWidth('540px')
const isMobile = useIsMobile()

// ==========================================================================
// Data
// ==========================================================================
const { success: toastSuccess, error: toastError } = useToast()
const loading = ref(true)
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
  description: '',
  schemesText: '',
  download_url: ''
})

// ==========================================================================
// Validation Rules
// ==========================================================================
const formRules = computed(() => ({
  name: [{ required: true, message: '请输入名称', trigger: 'blur' }]
}))

// ==========================================================================
// Data Loading
// ==========================================================================
async function fetchPlatforms() {
  try {
    const res = await adminApi.platforms.list()
    platforms.value = res.data.platforms || []
  } catch (e) {
    toastError('加载平台列表失败')
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
  form.description = row.description || ''
  form.schemesText = (row.client_schemes || []).join('\n')
  form.download_url = row.download_url || ''
  dialogVisible.value = true
}

function resetForm() {
  form.id = ''
  form.name = ''
  form.description = ''
  form.schemesText = ''
  form.download_url = ''
  formRef.value?.clearValidate()
}

async function handleSubmit() {
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) return

  // Parse schemes text to array
  const schemes = form.schemesText
    .split('\n')
    .map(s => s.trim())
    .filter(Boolean)

  submitting.value = true
  try {
    const payload = {
      name: form.name,
      description: form.description,
      client_schemes: schemes,
      download_url: form.download_url || ''
    }

    if (isEditing.value) {
      await adminApi.platforms.update(editingId.value, payload)
      toastSuccess('平台已更新')
    } else {
      await adminApi.platforms.create(payload)
      toastSuccess('平台已创建')
    }
    dialogVisible.value = false
    await fetchPlatforms()
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
    await adminApi.platforms.delete(deleteTarget.value.id)
    toastSuccess('平台已删除')
    deleteTarget.value = null
    await fetchPlatforms()
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
  await fetchPlatforms()
  loading.value = false
})
</script>

<style scoped>
</style>
