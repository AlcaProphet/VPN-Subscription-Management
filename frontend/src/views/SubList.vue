<template>
  <div class="sub-list-container" v-loading="loading">
    <div class="page-header">
      <h2>订阅管理</h2>
      <el-button type="primary" @click="openCreateDialog">
        <el-icon><Plus /></el-icon>
        创建订阅
      </el-button>
    </div>

    <el-empty
      v-if="!loading && subscriptions.length === 0"
      description="暂无订阅，请创建"
    />

    <el-table
      v-else
      :data="sortedSubscriptions"
      stripe
      class="sub-table"
    >
      <el-table-column prop="name" label="名称" min-width="160" />
      <el-table-column prop="platform" label="平台" width="140" />
      <el-table-column label="类型" width="100">
        <template #default="{ row }">
          <el-tag
            :type="row.type === 'default' ? 'info' : 'warning'"
            size="small"
          >
            {{ row.type === 'default' ? '默认' : '高级' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="当前版本" width="100">
        <template #default="{ row }">
          <span v-if="currentVersion(row) !== null">v{{ currentVersion(row) }}</span>
          <span v-else class="no-version">—</span>
        </template>
      </el-table-column>
      <el-table-column label="更新时间" width="180">
        <template #default="{ row }">
          <span v-if="currentUpdatedAt(row)">{{ formatTime(currentUpdatedAt(row)) }}</span>
          <span v-else class="no-version">—</span>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="260" fixed="right">
        <template #default="{ row }">
          <el-button size="small" @click="goVersions(row)">
            版本管理
          </el-button>
          <el-button size="small" @click="openEditDialog(row)">
            编辑
          </el-button>
          <el-button size="small" type="danger" @click="confirmDelete(row)">
            删除
          </el-button>
        </template>
      </el-table-column>
    </el-table>

    <!-- Create / Edit Dialog -->
    <el-dialog
      v-model="dialogVisible"
      :title="isEditing ? '编辑订阅' : '创建订阅'"
      width="480px"
      :close-on-click-modal="false"
      @closed="resetForm"
    >
      <el-form
        ref="formRef"
        :model="form"
        :rules="formRules"
        label-position="top"
      >
        <el-form-item label="ID" prop="id">
          <el-input
            v-model="form.id"
            :disabled="isEditing"
            placeholder="小写字母、数字和连字符"
          />
        </el-form-item>
        <el-form-item label="名称" prop="name">
          <el-input v-model="form.name" placeholder="订阅名称" />
        </el-form-item>
        <el-form-item label="类型" prop="type">
          <el-select v-model="form.type" style="width: 100%">
            <el-option label="默认 (default)" value="default" />
            <el-option label="高级 (advanced)" value="advanced" />
          </el-select>
        </el-form-item>
        <el-form-item label="平台" prop="platform">
          <el-select v-model="form.platform" style="width: 100%">
            <el-option
              v-for="p in platforms"
              :key="p.id"
              :label="p.name"
              :value="p.id"
            />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="handleSubmit">
          {{ isEditing ? '保存' : '创建' }}
        </el-button>
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
import { ElMessage } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import { adminApi } from '@/services/api'
import ConfirmDialog from '@/components/ConfirmDialog.vue'

const router = useRouter()

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
  id: [
    { required: true, message: '请输入 ID', trigger: 'blur' },
    {
      pattern: /^[a-z0-9-]+$/,
      message: 'ID 只能包含小写字母、数字和连字符',
      trigger: 'blur'
    }
  ],
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
    ElMessage.error('加载订阅列表失败')
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
      ElMessage.success('订阅已更新')
    } else {
      await adminApi.subscriptions.create({
        id: form.id,
        name: form.name,
        type: form.type,
        platform: form.platform
      })
      ElMessage.success('订阅已创建')
    }
    dialogVisible.value = false
    await fetchSubscriptions()
  } catch (e) {
    const msg = e.response?.data?.error || '操作失败'
    ElMessage.error(msg)
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
    ElMessage.success('订阅已删除')
    deleteTarget.value = null
    await fetchSubscriptions()
  } catch (e) {
    const msg = e.response?.data?.error || '删除失败'
    ElMessage.error(msg)
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
