<template>
  <div class="platform-container" v-loading="loading">
    <div class="page-header">
      <h2>平台管理</h2>
      <el-button type="primary" @click="openCreateDialog">
        <el-icon><Plus /></el-icon>
        创建平台
      </el-button>
    </div>

    <el-empty
      v-if="!loading && platforms.length === 0"
      description="暂无平台"
    />

    <el-table
      v-else
      :data="platforms"
      stripe
      class="platform-table"
    >
      <el-table-column prop="id" label="ID" width="140" />
      <el-table-column prop="name" label="名称" min-width="140" />
      <el-table-column prop="description" label="描述" min-width="160" show-overflow-tooltip />
      <el-table-column label="Client Schemes" min-width="200">
        <template #default="{ row }">
          <el-tag
            v-for="(scheme, idx) in row.client_schemes"
            :key="idx"
            size="small"
            class="scheme-tag"
          >
            {{ scheme }}
          </el-tag>
          <span v-if="!row.client_schemes || row.client_schemes.length === 0" class="no-data">—</span>
        </template>
      </el-table-column>
      <el-table-column label="下载链接" min-width="160" show-overflow-tooltip>
        <template #default="{ row }">
          <span v-if="row.download_url">{{ row.download_url }}</span>
          <span v-else class="no-data">—</span>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="160" fixed="right">
        <template #default="{ row }">
          <el-button size="small" @click="openEditDialog(row)">编辑</el-button>
          <el-button size="small" type="danger" @click="confirmDelete(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <!-- Create / Edit Dialog -->
    <el-dialog
      v-model="dialogVisible"
      :title="isEditing ? '编辑平台' : '创建平台'"
      width="540px"
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
          <el-input v-model="form.name" placeholder="平台名称" />
        </el-form-item>
        <el-form-item label="描述" prop="description">
          <el-input
            v-model="form.description"
            type="textarea"
            :rows="2"
            placeholder="平台描述"
          />
        </el-form-item>
        <el-form-item label="Client Schemes" prop="schemesText">
          <el-input
            v-model="form.schemesText"
            type="textarea"
            :rows="4"
            placeholder="每行一个 scheme，例如：&#10;clash://install-config?url=&#10;clash-verge://install-config?url="
          />
          <div class="form-tip">每行一个 Client Scheme，一键导入时使用第一个</div>
        </el-form-item>
        <el-form-item label="下载链接" prop="download_url">
          <el-input v-model="form.download_url" placeholder="https://example.com/download（可选）" />
          <div class="form-tip">可选，配置后在首页显示「下载客户端」按钮</div>
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
      title="删除平台"
      message="确定删除该平台？将级联删除该平台的所有订阅、下载 Token 和自定义订阅。此操作不可恢复！"
      @confirm="handleDelete"
    />
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import { adminApi } from '@/services/api'
import ConfirmDialog from '@/components/ConfirmDialog.vue'

// ==========================================================================
// Data
// ==========================================================================
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
  id: [
    { required: true, message: '请输入 ID', trigger: 'blur' },
    {
      pattern: /^[a-z0-9-]+$/,
      message: 'ID 只能包含小写字母、数字和连字符',
      trigger: 'blur'
    }
  ],
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
    ElMessage.error('加载平台列表失败')
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
      ElMessage.success('平台已更新')
    } else {
      payload.id = form.id
      await adminApi.platforms.create(payload)
      ElMessage.success('平台已创建')
    }
    dialogVisible.value = false
    await fetchPlatforms()
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
    await adminApi.platforms.delete(deleteTarget.value.id)
    ElMessage.success('平台已删除')
    deleteTarget.value = null
    await fetchPlatforms()
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
  await fetchPlatforms()
  loading.value = false
})
</script>

<style scoped>
.platform-container {
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

.platform-table {
  width: 100%;
}

.scheme-tag {
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
</style>
