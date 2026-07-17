<template>
  <div class="user-container" v-loading="loading">
    <div class="page-header">
      <h2>用户管理</h2>
      <span class="header-tip">用户通过 OIDC 登录自动创建，不可手动添加</span>
    </div>

    <el-empty
      v-if="!loading && users.length === 0"
      description="暂无用户"
    />

    <el-table
      v-else
      :data="users"
      stripe
      class="user-table"
    >
      <el-table-column prop="username" label="用户名" min-width="120" />
      <el-table-column prop="email" label="邮箱" min-width="180" show-overflow-tooltip />
      <el-table-column label="角色" width="90">
        <template #default="{ row }">
          <el-tag
            :type="row.role === 'admin' ? 'danger' : 'info'"
            size="small"
          >
            {{ row.role === 'admin' ? '管理员' : '普通用户' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="订阅级别" width="90">
        <template #default="{ row }">
          <el-tag
            :type="row.is_advanced ? 'warning' : 'info'"
            size="small"
          >
            {{ row.is_advanced ? '高级' : '普通' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="自定义订阅" min-width="140">
        <template #default="{ row }">
          <template v-if="row.has_custom_sub">
            <el-tag
              v-for="p in row.custom_sub_platforms"
              :key="p"
              size="small"
              type="success"
              class="platform-tag"
            >
              {{ p }}
            </el-tag>
          </template>
          <span v-else class="no-data">—</span>
        </template>
      </el-table-column>
      <el-table-column label="操作" min-width="340" fixed="right">
        <template #default="{ row }">
          <el-button size="small" @click="openEditDialog(row)">编辑</el-button>
          <el-button size="small" @click="openUploadDialog(row)">上传自定义订阅</el-button>
          <el-button
            v-if="row.has_custom_sub"
            size="small"
            type="warning"
            @click="openDeleteCustomDialog(row)"
          >
            删除自定义订阅
          </el-button>
          <el-button size="small" type="danger" @click="confirmRevoke(row)">
            吊销 Token
          </el-button>
          <el-button size="small" type="danger" @click="confirmDeleteUser(row)">
            删除用户
          </el-button>
        </template>
      </el-table-column>
    </el-table>

    <!-- Edit Dialog -->
    <el-dialog
      v-model="editVisible"
      title="编辑用户"
      width="460px"
      :close-on-click-modal="false"
    >
      <el-form v-if="editUser" label-position="top">
        <el-form-item label="用户名">
          <el-input :model-value="editUser.username" disabled />
        </el-form-item>
        <el-form-item label="邮箱">
          <el-input :model-value="editUser.email" disabled />
        </el-form-item>
        <el-form-item label="角色">
          <el-tag :type="editUser.role === 'admin' ? 'danger' : 'info'" size="small">
            {{ editUser.role === 'admin' ? '管理员' : '普通用户' }}
          </el-tag>
        </el-form-item>
        <el-form-item label="订阅级别">
          <el-switch
            v-model="editIsAdvanced"
            :disabled="isSelf(editUser) && editUser.role === 'admin'"
            active-text="高级"
            inactive-text="普通"
          />
          <div v-if="isSelf(editUser) && editUser.role === 'admin'" class="form-tip">
            管理员始终为高级用户
          </div>
        </el-form-item>
        <el-form-item v-if="editUser.groups && editUser.groups.length > 0" label="Groups">
          <el-tag v-for="g in editUser.groups" :key="g" size="small" class="group-tag">
            {{ g }}
          </el-tag>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editVisible = false">取消</el-button>
        <el-button type="primary" :loading="editSubmitting" @click="handleEditSave">
          保存
        </el-button>
      </template>
    </el-dialog>

    <!-- Upload Custom Subscription Dialog -->
    <el-dialog
      v-model="uploadVisible"
      title="上传自定义订阅"
      width="480px"
      :close-on-click-modal="false"
      @closed="resetUploadForm"
    >
      <el-form ref="uploadFormRef" :model="uploadForm" :rules="uploadRules" label-position="top">
        <el-form-item label="适用平台" prop="platform">
          <el-select v-model="uploadForm.platform" style="width: 100%" placeholder="请选择平台">
            <el-option
              v-for="p in platforms"
              :key="p.id"
              :label="p.name"
              :value="p.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="订阅文件">
          <el-upload
            ref="uploadRef"
            :auto-upload="false"
            :limit="1"
            accept=".conf,.yaml,.yml,.txt"
            :on-change="onUploadFileChange"
            :before-upload="beforeUploadCheck"
            drag
          >
            <el-icon class="el-icon--upload"><UploadFilled /></el-icon>
            <div class="el-upload__text">
              将文件拖到此处，或<em>点击上传</em>
            </div>
            <template #tip>
              <div class="el-upload__tip">
                文件大小不超过 50MB。同一平台再次上传将覆盖已有自定义订阅
              </div>
            </template>
          </el-upload>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="uploadVisible = false">取消</el-button>
        <el-button type="primary" :loading="uploadSubmitting" :disabled="!uploadFile" @click="handleUpload">
          上传
        </el-button>
      </template>
    </el-dialog>

    <!-- Delete Custom Subscription Dialog -->
    <el-dialog
      v-model="deleteCustomVisible"
      title="删除自定义订阅"
      width="440px"
      :close-on-click-modal="false"
    >
      <p v-if="deleteCustomUser">
        请选择要删除的自定义订阅平台：
      </p>
      <el-select
        v-if="deleteCustomUser"
        v-model="deleteCustomPlatform"
        style="width: 100%"
        placeholder="请选择平台"
      >
        <el-option
          v-for="p in deleteCustomUser.custom_sub_platforms"
          :key="p"
          :label="p"
          :value="p"
        />
      </el-select>
      <template #footer>
        <el-button @click="deleteCustomVisible = false">取消</el-button>
        <el-button
          type="danger"
          :loading="deleteCustomSubmitting"
          :disabled="!deleteCustomPlatform"
          @click="handleDeleteCustom"
        >
          删除
        </el-button>
      </template>
    </el-dialog>

    <!-- Revoke Tokens Confirm -->
    <ConfirmDialog
      v-model:visible="revokeVisible"
      title="吊销下载 Token"
      message="确定吊销该用户所有下载链接？吊销后用户需重新获取。"
      @confirm="handleRevoke"
    />

    <!-- Delete User Confirm -->
    <ConfirmDialog
      v-model:visible="deleteUserVisible"
      title="删除用户"
      message="确定删除该用户？将级联删除其所有下载 Token 和自定义订阅。此操作不可恢复！"
      @confirm="handleDeleteUser"
    />
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { UploadFilled } from '@element-plus/icons-vue'
import { adminApi } from '@/services/api'
import { useUserStore } from '@/stores/user'
import ConfirmDialog from '@/components/ConfirmDialog.vue'

const userStore = useUserStore()

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
    ElMessage.error('加载用户列表失败')
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
    ElMessage.success('用户已更新')
    editVisible.value = false
    await fetchUsers()
  } catch (e) {
    const msg = e.response?.data?.error || '更新失败'
    ElMessage.error(msg)
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
    ElMessage.error('文件大小不能超过 50MB')
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
    ElMessage.success('自定义订阅已上传')
    uploadVisible.value = false
    await fetchUsers()
  } catch (e) {
    const msg = e.response?.data?.error || '上传失败'
    ElMessage.error(msg)
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
    ElMessage.success('自定义订阅已删除，用户恢复默认/高级自动分配')
    deleteCustomVisible.value = false
    await fetchUsers()
  } catch (e) {
    const msg = e.response?.data?.error || '删除失败'
    ElMessage.error(msg)
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
    ElMessage.success('用户下载 Token 已全部吊销')
    revokeTarget.value = null
  } catch (e) {
    const msg = e.response?.data?.error || '操作失败'
    ElMessage.error(msg)
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
    ElMessage.success('用户已删除')
    deleteUserTarget.value = null
    await fetchUsers()
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
  await Promise.all([fetchUsers(), fetchPlatforms()])
  loading.value = false
})
</script>

<style scoped>
.user-container {
  padding: 0;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
  flex-wrap: wrap;
  gap: 8px;
}

.page-header h2 {
  margin: 0;
  font-size: 20px;
  font-weight: 600;
}

.header-tip {
  font-size: 13px;
  color: var(--el-text-color-secondary);
}

.user-table {
  width: 100%;
}

.platform-tag {
  margin-right: 4px;
  margin-bottom: 4px;
}

.no-data {
  color: var(--el-text-color-secondary);
}

.form-tip {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-top: 4px;
}

.group-tag {
  margin-right: 4px;
}
</style>
