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
        <h2 class="m-0 text-xl font-semibold text-gray-900 dark:text-white">用户管理</h2>
        <span class="text-sm text-gray-400 dark:text-gray-500">用户通过 OIDC 登录自动创建，不可手动添加</span>
      </div>

      <div v-if="users.length === 0" class="text-center py-12 text-gray-400 dark:text-gray-500">暂无用户</div>

      <!-- Card Grid -->
      <div v-else class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-5">
        <div
          v-for="u in users"
          :key="u.user_id"
          class="bg-white dark:bg-gray-800 rounded-lg shadow-md overflow-hidden"
        >
          <div class="px-4 py-3 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between gap-2">
            <span class="text-base font-semibold text-gray-900 dark:text-white truncate">{{ u.username }}</span>
            <span class="rounded-full px-2 py-0.5 text-xs font-medium shrink-0" :class="u.role === 'admin' ? 'bg-purple-100 dark:bg-purple-900/30 text-purple-700 dark:text-purple-300' : 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300'">{{ u.role === 'admin' ? '管理员' : '普通用户' }}</span>
          </div>
          <div class="p-4">
            <div class="text-sm text-gray-500 dark:text-gray-400 mb-3 space-y-1">
              <div class="flex items-center gap-2">
                <span class="text-gray-400 dark:text-gray-500">邮箱:</span>
                <span class="text-gray-700 dark:text-gray-300 truncate">{{ u.email }}</span>
              </div>
              <div class="flex items-center gap-2">
                <span class="text-gray-400 dark:text-gray-500">级别:</span>
                <span class="rounded-full px-2 py-0.5 text-xs font-medium" :class="u.is_advanced ? 'bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300' : 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300'">{{ u.is_advanced ? '高级' : '普通' }}</span>
              </div>
              <div class="flex items-center gap-2">
                <span class="text-gray-400 dark:text-gray-500 shrink-0">自定义:</span>
                <template v-if="u.has_custom_sub">
                  <span v-for="p in u.custom_sub_platforms" :key="p" class="inline-block rounded-full px-2 py-0.5 text-xs font-medium bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-300 mr-1">{{ p }}</span>
                </template>
                <span v-else class="text-gray-400 dark:text-gray-500 italic">—</span>
              </div>
            </div>
            <div class="flex flex-wrap gap-1">
              <button class="bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-600 rounded-md px-3 py-1.5 text-sm" @click="openEditDialog(u)">编辑</button>
              <button class="bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-600 rounded-md px-3 py-1.5 text-sm" @click="openUploadDialog(u)">上传自定义订阅</button>
              <button v-if="u.has_custom_sub" class="bg-orange-600 hover:bg-orange-700 text-white rounded-md px-3 py-1.5 text-sm" @click="openDeleteCustomDialog(u)">删除自定义订阅</button>
              <button class="bg-red-600 hover:bg-red-700 text-white rounded-md px-3 py-1.5 text-sm" @click="confirmRevoke(u)">吊销 Token</button>
              <button class="bg-red-600 hover:bg-red-700 text-white rounded-md px-3 py-1.5 text-sm" @click="confirmDeleteUser(u)">删除用户</button>
            </div>
          </div>
        </div>
      </div>
    </template>

    <!-- Edit Dialog -->
    <el-dialog v-model="editVisible" title="编辑用户" :width="editDialogWidth" :close-on-click-modal="false" :append-to-body="true">
      <el-form v-if="editUser" label-position="top">
        <el-form-item label="用户名">
          <input :value="editUser.username" disabled class="w-full rounded-md border border-gray-300 dark:border-gray-600 bg-gray-100 dark:bg-gray-600 px-3 py-2 text-base text-gray-500 dark:text-gray-400 cursor-not-allowed" />
        </el-form-item>
        <el-form-item label="邮箱">
          <input :value="editUser.email" disabled class="w-full rounded-md border border-gray-300 dark:border-gray-600 bg-gray-100 dark:bg-gray-600 px-3 py-2 text-base text-gray-500 dark:text-gray-400 cursor-not-allowed" />
        </el-form-item>
        <el-form-item label="角色">
          <span class="rounded-full px-2 py-0.5 text-xs font-medium" :class="editUser.role === 'admin' ? 'bg-purple-100 dark:bg-purple-900/30 text-purple-700 dark:text-purple-300' : 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300'">{{ editUser.role === 'admin' ? '管理员' : '普通用户' }}</span>
        </el-form-item>
        <el-form-item label="订阅级别">
          <button
            role="switch"
            :aria-checked="editIsAdvanced"
            @click="editIsAdvanced = !editIsAdvanced"
            @keydown.space.prevent="editIsAdvanced = !editIsAdvanced"
            @keydown.enter.prevent="editIsAdvanced = !editIsAdvanced"
            :disabled="isSelf(editUser) && editUser.role === 'admin'"
            class="relative inline-flex h-6 w-11 items-center rounded-full transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
            :class="editIsAdvanced ? 'bg-blue-600' : 'bg-gray-300 dark:bg-gray-600'"
          >
            <span class="inline-block h-4 w-4 transform rounded-full bg-white transition-transform" :class="editIsAdvanced ? 'translate-x-6' : 'translate-x-1'"/>
          </button>
          <span class="ml-2 text-sm" :class="editIsAdvanced ? 'text-blue-600' : 'text-gray-500'">{{ editIsAdvanced ? '高级' : '普通' }}</span>
          <div v-if="isSelf(editUser) && editUser.role === 'admin'" class="text-xs text-gray-400 dark:text-gray-500 mt-1">管理员始终为高级用户</div>
        </el-form-item>
        <el-form-item v-if="editUser.groups && editUser.groups.length > 0" label="Groups">
          <span v-for="g in editUser.groups" :key="g" class="inline-block rounded-full px-2 py-0.5 text-xs font-medium bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 mr-1 mb-1">{{ g }}</span>
        </el-form-item>
      </el-form>
      <template #footer>
        <div class="flex justify-end gap-2">
          <button class="bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-600 rounded-md px-4 py-2 text-sm" @click="editVisible = false">取消</button>
          <button class="bg-blue-600 hover:bg-blue-700 text-white rounded-md px-4 py-2 text-sm disabled:opacity-50" :disabled="editSubmitting" @click="handleEditSave">保存</button>
        </div>
      </template>
    </el-dialog>

    <!-- Upload Custom Subscription Dialog -->
    <el-dialog v-model="uploadVisible" title="上传自定义订阅" :width="uploadDialogWidth" :close-on-click-modal="false" :append-to-body="true" @closed="resetUploadForm">
      <el-form ref="uploadFormRef" :model="uploadForm" :rules="uploadRules" label-position="top">
        <el-form-item label="适用平台" prop="platform">
          <select v-model="uploadForm.platform" class="w-full rounded-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-3 py-2 text-base text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none" @change="uploadFormRef.validateField('platform')">
            <option value="" disabled>请选择平台</option>
            <option v-for="p in platforms" :key="p.id" :value="p.id">{{ p.name }}</option>
          </select>
        </el-form-item>
        <el-form-item label="订阅文件">
          <el-upload ref="uploadRef" :auto-upload="false" :limit="1" accept=".conf,.yaml,.yml,.txt" :on-change="onUploadFileChange" :before-upload="beforeUploadCheck" drag>
            <div class="flex flex-col items-center py-4">
              <svg class="w-10 h-10 text-gray-400 mb-2" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12"/></svg>
              <div class="text-sm text-gray-600 dark:text-gray-400">将文件拖到此处，或<em class="text-blue-600 not-italic">点击上传</em></div>
              <div class="text-xs text-gray-400 dark:text-gray-500 mt-1">文件大小不超过 50MB。同一平台再次上传将覆盖已有自定义订阅</div>
            </div>
          </el-upload>
        </el-form-item>
      </el-form>
      <template #footer>
        <div class="flex justify-end gap-2">
          <button class="bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-600 rounded-md px-4 py-2 text-sm" @click="uploadVisible = false">取消</button>
          <button class="bg-blue-600 hover:bg-blue-700 text-white rounded-md px-4 py-2 text-sm disabled:opacity-50" :disabled="!uploadFile" @click="handleUpload">上传</button>
        </div>
      </template>
    </el-dialog>

    <!-- Delete Custom Subscription Dialog -->
    <el-dialog v-model="deleteCustomVisible" title="删除自定义订阅" :width="deleteDialogWidth" :close-on-click-modal="false" :append-to-body="true">
      <p v-if="deleteCustomUser" class="text-gray-700 dark:text-gray-300">请选择要删除的自定义订阅平台：</p>
      <select v-if="deleteCustomUser" v-model="deleteCustomPlatform" class="w-full rounded-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-3 py-2 text-base text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none mt-2">
        <option value="" disabled>请选择平台</option>
        <option v-for="p in deleteCustomUser.custom_sub_platforms" :key="p" :value="p">{{ p }}</option>
      </select>
      <template #footer>
        <div class="flex justify-end gap-2">
          <button class="bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-600 rounded-md px-4 py-2 text-sm" @click="deleteCustomVisible = false">取消</button>
          <button class="bg-red-600 hover:bg-red-700 text-white rounded-md px-4 py-2 text-sm disabled:opacity-50" :disabled="!deleteCustomPlatform" @click="handleDeleteCustom">删除</button>
        </div>
      </template>
    </el-dialog>

    <ConfirmDialog v-model:visible="revokeVisible" title="吊销下载 Token" message="确定吊销该用户所有下载链接？吊销后用户需重新获取。" @confirm="handleRevoke" />
    <ConfirmDialog v-model:visible="deleteUserVisible" title="删除用户" message="确定删除该用户？将级联删除其所有下载 Token 和自定义订阅。此操作不可恢复！" @confirm="handleDeleteUser" />
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { useToast } from '@/composables/useToast'
import { useDialogWidth } from '@/composables/useDialogWidth'
import { adminApi } from '@/services/api'
import { useUserStore } from '@/stores/user'
import ConfirmDialog from '@/components/ConfirmDialog.vue'

const editDialogWidth = useDialogWidth('460px')
const uploadDialogWidth = useDialogWidth('480px')
const deleteDialogWidth = useDialogWidth('440px')

const userStore = useUserStore()
const { success: toastSuccess, error: toastError } = useToast()

// ==========================================================================
// Data
// ==========================================================================
const loading = ref(true)
const users = ref([])
const platforms = ref([])

// Edit
const editVisible = ref(false)
const editUser = ref(null)
const editIsAdvanced = ref(false)
const editSubmitting = ref(false)

// Upload custom sub
const uploadVisible = ref(false)
const uploadUser = ref(null)
const uploadFile = ref(null)
const uploadSubmitting = ref(false)
const uploadFormRef = ref(null)
const uploadRef = ref(null)
const uploadForm = reactive({ platform: '' })
const uploadRules = {
  platform: [{ required: true, message: '请选择平台', trigger: 'change' }]
}

// Delete custom sub
const deleteCustomVisible = ref(false)
const deleteCustomUser = ref(null)
const deleteCustomPlatform = ref('')
const deleteCustomSubmitting = ref(false)

// Revoke
const revokeVisible = ref(false)
const revokeTarget = ref(null)

// Delete user
const deleteUserVisible = ref(false)
const deleteUserTarget = ref(null)

// ==========================================================================
// Helpers
// ==========================================================================
function isSelf(user) {
  return userStore.user && userStore.user.user_id === user.user_id
}

// ==========================================================================
// Data Loading
// ==========================================================================
async function fetchUsers() {
  try {
    const res = await adminApi.users.list()
    users.value = res.data.users || []
  } catch (e) {
    toastError('加载用户列表失败')
  }
}

async function fetchPlatforms() {
  try {
    const res = await adminApi.platforms.list()
    platforms.value = res.data.platforms || []
  } catch (e) {
    // Non-critical
  }
}

// ==========================================================================
// Edit
// ==========================================================================
function openEditDialog(row) {
  editUser.value = { ...row }
  editIsAdvanced.value = row.is_advanced
  editVisible.value = true
}

async function handleEditSave() {
  if (!editUser.value) return
  editSubmitting.value = true
  try {
    await adminApi.users.update(editUser.value.user_id, {
      is_advanced: editIsAdvanced.value
    })
    toastSuccess('用户已更新')
    editVisible.value = false
    await fetchUsers()
  } catch (e) {
    const msg = e.response?.data?.error || '更新失败'
    toastError(msg)
  } finally {
    editSubmitting.value = false
  }
}

// ==========================================================================
// Upload Custom Subscription
// ==========================================================================
function openUploadDialog(row) {
  uploadUser.value = row
  resetUploadForm()
  uploadVisible.value = true
}

function resetUploadForm() {
  uploadForm.platform = ''
  uploadFile.value = null
  uploadFormRef.value?.clearValidate()
  uploadRef.value?.clearFiles()
}

function onUploadFileChange(file) {
  uploadFile.value = file.raw
}

function beforeUploadCheck(file) {
  const maxBytes = 50 * 1024 * 1024
  if (file.size > maxBytes) {
    toastError('文件大小不能超过 50MB')
    return false
  }
  return true
}

async function handleUpload() {
  const valid = await uploadFormRef.value.validate().catch(() => false)
  if (!valid || !uploadFile.value || !uploadUser.value) return

  uploadSubmitting.value = true
  try {
    await adminApi.users.uploadCustomSub(
      uploadUser.value.user_id,
      uploadForm.platform,
      uploadFile.value
    )
    toastSuccess('自定义订阅已上传')
    uploadVisible.value = false
    await fetchUsers()
  } catch (e) {
    const msg = e.response?.data?.error || '上传失败'
    toastError(msg)
  } finally {
    uploadSubmitting.value = false
  }
}

// ==========================================================================
// Delete Custom Subscription
// ==========================================================================
function openDeleteCustomDialog(row) {
  deleteCustomUser.value = row
  deleteCustomPlatform.value = ''
  deleteCustomVisible.value = true
}

async function handleDeleteCustom() {
  if (!deleteCustomUser.value || !deleteCustomPlatform.value) return
  deleteCustomSubmitting.value = true
  try {
    await adminApi.users.deleteCustomSub(
      deleteCustomUser.value.user_id,
      deleteCustomPlatform.value
    )
    toastSuccess('自定义订阅已删除，用户恢复默认/高级自动分配')
    deleteCustomVisible.value = false
    await fetchUsers()
  } catch (e) {
    const msg = e.response?.data?.error || '删除失败'
    toastError(msg)
  } finally {
    deleteCustomSubmitting.value = false
  }
}

// ==========================================================================
// Revoke Tokens
// ==========================================================================
function confirmRevoke(row) {
  revokeTarget.value = row
  revokeVisible.value = true
}

async function handleRevoke() {
  if (!revokeTarget.value) return
  try {
    await adminApi.users.revokeTokens(revokeTarget.value.user_id)
    toastSuccess('用户下载 Token 已全部吊销')
    revokeTarget.value = null
  } catch (e) {
    const msg = e.response?.data?.error || '操作失败'
    toastError(msg)
  }
}

// ==========================================================================
// Delete User
// ==========================================================================
function confirmDeleteUser(row) {
  deleteUserTarget.value = row
  deleteUserVisible.value = true
}

async function handleDeleteUser() {
  if (!deleteUserTarget.value) return
  try {
    await adminApi.users.delete(deleteUserTarget.value.user_id)
    toastSuccess('用户已删除')
    deleteUserTarget.value = null
    await fetchUsers()
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
  await Promise.all([fetchUsers(), fetchPlatforms()])
  loading.value = false
})
</script>

<style scoped>
</style>
