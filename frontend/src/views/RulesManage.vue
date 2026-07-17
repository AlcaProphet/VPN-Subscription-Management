<template>
  <div class="rules-container" v-loading="loading">
    <div class="page-header">
      <h2>规则管理</h2>
      <el-button type="primary" @click="openCreateDialog">
        <el-icon><Plus /></el-icon>
        创建规则
      </el-button>
    </div>

    <el-empty
      v-if="!loading && rules.length === 0"
      description="暂无规则，请创建"
    />

    <el-table
      v-else
      :data="rules"
      stripe
      class="rules-table"
    >
      <el-table-column prop="name" label="规则名称" min-width="160" />
      <el-table-column label="客户端类型" width="130">
        <template #default="{ row }">
          <el-tag size="small">{{ row.client_type || 'Shadowrocket' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="当前版本" width="100">
        <template #default="{ row }">
          <span v-if="currentVersion(row) !== null">v{{ currentVersion(row) }}</span>
          <span v-else class="no-data">—</span>
        </template>
      </el-table-column>
      <el-table-column label="更新时间" width="180">
        <template #default="{ row }">
          <span v-if="currentUpdatedAt(row)">{{ formatTime(currentUpdatedAt(row)) }}</span>
          <span v-else class="no-data">—</span>
        </template>
      </el-table-column>
      <el-table-column label="Token" width="140">
        <template #default="{ row }">
          <span v-if="row.token" class="token-text">{{ maskToken(row.token) }}</span>
          <span v-else class="no-data">—</span>
        </template>
      </el-table-column>
      <el-table-column label="操作" min-width="300" fixed="right">
        <template #default="{ row }">
          <el-button size="small" @click="goVersions(row)">
            版本管理
          </el-button>
          <el-button
            size="small"
            :disabled="!row.token"
            @click="copyDownloadLink(row)"
          >
            复制下载链接
          </el-button>
          <el-button size="small" type="warning" @click="confirmRotateToken(row)">
            轮替 Token
          </el-button>
          <el-button size="small" type="danger" @click="confirmDelete(row)">
            删除
          </el-button>
        </template>
      </el-table-column>
    </el-table>

    <!-- Create Dialog -->
    <el-dialog
      v-model="createVisible"
      title="创建规则"
      width="520px"
      :close-on-click-modal="false"
      @closed="resetCreateForm"
    >
      <el-tabs v-model="createTab">
        <el-tab-pane label="文件上传" name="file">
          <el-form ref="createFileFormRef" :model="createForm" :rules="createRules" label-position="top">
            <el-form-item label="ID" prop="id">
              <el-input v-model="createForm.id" placeholder="小写字母、数字和连字符" />
            </el-form-item>
            <el-form-item label="名称" prop="name">
              <el-input v-model="createForm.name" placeholder="规则名称" />
            </el-form-item>
            <el-form-item label="客户端类型" prop="client_type">
              <el-select v-model="createForm.client_type" style="width: 100%">
                <el-option label="Shadowrocket" value="shadowrocket" />
              </el-select>
            </el-form-item>
            <el-form-item label="规则文件">
              <el-upload
                ref="createUploadRef"
                :auto-upload="false"
                :limit="1"
                accept=".conf,.yaml,.yml,.txt"
                :on-change="onCreateFileChange"
                :before-upload="beforeCreateUpload"
                drag
              >
                <el-icon class="el-icon--upload"><UploadFilled /></el-icon>
                <div class="el-upload__text">
                  将文件拖到此处，或<em>点击上传</em>
                </div>
                <template #tip>
                  <div class="el-upload__tip">文件大小不超过 50MB</div>
                </template>
              </el-upload>
            </el-form-item>
          </el-form>
          <div style="margin-top: 12px; text-align: right">
            <el-button @click="createVisible = false">取消</el-button>
            <el-button type="primary" :loading="submitting" :disabled="!createFileSelected" @click="handleCreateFile">
              创建并上传
            </el-button>
          </div>
        </el-tab-pane>
        <el-tab-pane label="文本编辑" name="text">
          <el-form ref="createTextFormRef" :model="createForm" :rules="createRules" label-position="top">
            <el-form-item label="ID" prop="id">
              <el-input v-model="createForm.id" placeholder="小写字母、数字和连字符" />
            </el-form-item>
            <el-form-item label="名称" prop="name">
              <el-input v-model="createForm.name" placeholder="规则名称" />
            </el-form-item>
            <el-form-item label="客户端类型" prop="client_type">
              <el-select v-model="createForm.client_type" style="width: 100%">
                <el-option label="Shadowrocket" value="shadowrocket" />
              </el-select>
            </el-form-item>
            <el-form-item label="规则内容" prop="content">
              <el-input
                v-model="createForm.content"
                type="textarea"
                :rows="10"
                placeholder="在此粘贴规则配置文本..."
              />
            </el-form-item>
          </el-form>
          <div style="margin-top: 12px; text-align: right">
            <el-button @click="createVisible = false">取消</el-button>
            <el-button type="primary" :loading="submitting" :disabled="!createForm.content.trim()" @click="handleCreateText">
              创建
            </el-button>
          </div>
        </el-tab-pane>
      </el-tabs>
    </el-dialog>

    <!-- Rotate Token Confirm -->
    <ConfirmDialog
      v-model:visible="rotateVisible"
      title="轮替 Token"
      message="轮替后旧链接立即失效，确定？"
      @confirm="handleRotateToken"
    />

    <!-- Delete Confirm -->
    <ConfirmDialog
      v-model:visible="deleteVisible"
      title="删除规则"
      message="确定删除？将级联删除所有版本文件和 Token。"
      @confirm="handleDelete"
    />
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Plus, UploadFilled } from '@element-plus/icons-vue'
import { adminApi, publicApi } from '@/services/api'
import ConfirmDialog from '@/components/ConfirmDialog.vue'

const router = useRouter()

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
const createTextFormRef = ref(null)
const createUploadRef = ref(null)

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
    ElMessage.error('加载规则列表失败')
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
  createUploadRef.value?.clearFiles()
  createFileFormRef.value?.clearValidate()
  createTextFormRef.value?.clearValidate()
}

function onCreateFileChange(file) {
  createFile.value = file.raw
  createFileSelected.value = true
}

function beforeCreateUpload(file) {
  const maxBytes = 50 * 1024 * 1024
  if (file.size > maxBytes) {
    ElMessage.error('文件大小不能超过 50MB')
    return false
  }
  return true
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
    ElMessage.success('规则已创建')
    if (res.data.token) {
      ElMessage.info('Token: ' + res.data.token)
    }
    createVisible.value = false
    await fetchRules()
  } catch (e) {
    const msg = e.response?.data?.error || '创建失败'
    ElMessage.error(msg)
  } finally {
    submitting.value = false
  }
}

async function handleCreateText() {
  const valid = await createTextFormRef.value.validate().catch(() => false)
  if (!valid || !createForm.content.trim()) return

  submitting.value = true
  try {
    const res = await adminApi.rules.create({
      id: createForm.id,
      name: createForm.name,
      client_type: createForm.client_type,
      content: createForm.content
    })
    ElMessage.success('规则已创建')
    if (res.data.token) {
      ElMessage.info('Token: ' + res.data.token)
    }
    createVisible.value = false
    await fetchRules()
  } catch (e) {
    const msg = e.response?.data?.error || '创建失败'
    ElMessage.error(msg)
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
    ElMessage.success('已复制到剪贴板')
  }).catch(() => {
    ElMessage.error('复制失败，请手动复制')
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
    ElMessage.success('Token 已轮替，旧链接立即失效')
    actionTarget.value = null
    await fetchRules()
  } catch (e) {
    const msg = e.response?.data?.error || '轮替失败'
    ElMessage.error(msg)
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
    ElMessage.success('规则已删除')
    actionTarget.value = null
    await fetchRules()
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
  await fetchRules()
  loading.value = false
})
</script>

<style scoped>
.rules-container {
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

.rules-table {
  width: 100%;
}

.token-text {
  font-family: monospace;
  font-size: 13px;
}

.no-data {
  color: var(--el-text-color-secondary);
}
</style>
