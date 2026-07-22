<template>
  <div class="logs-container">
    <div class="page-header">
      <h2>日志查看</h2>
      <el-date-picker
        v-model="selectedDate"
        type="date"
        placeholder="选择日期"
        format="YYYY-MM-DD"
        value-format="YYYY-MM-DD"
        @change="fetchLogs"
      />
    </div>

    <el-empty
      v-if="!loading && logs.length === 0"
      description="暂无日志记录"
    />

    <el-table
      v-else
      :data="logs"
      stripe
      class="logs-table"
      v-loading="loading"
    >
      <el-table-column label="时间" width="180">
        <template #default="{ row }">
          {{ formatTime(row.created_at) }}
        </template>
      </el-table-column>
      <el-table-column label="下载类型" width="130">
        <template #default="{ row }">
          <el-tag size="small" :type="typeTagType(row.download_type)">
            {{ downloadTypeLabel(row.download_type) }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="用户 ID" width="140" show-overflow-tooltip>
        <template #default="{ row }">
          {{ row.user_id || '—' }}
        </template>
      </el-table-column>
      <el-table-column label="平台" width="130">
        <template #default="{ row }">
          {{ row.platform || '—' }}
        </template>
      </el-table-column>
      <el-table-column label="状态" width="80">
        <template #default="{ row }">
          <el-tag :type="row.status === 'success' ? 'success' : 'danger'" size="small">
            {{ row.status === 'success' ? '成功' : '失败' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="失败原因" min-width="160" show-overflow-tooltip>
        <template #default="{ row }">
          <span v-if="row.error_reason" class="error-reason">{{ row.error_reason }}</span>
          <span v-else class="no-data">—</span>
        </template>
      </el-table-column>
      <el-table-column label="IP" width="150">
        <template #default="{ row }">
          {{ row.ip }}
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useToast } from '@/composables/useToast'
const { success: toastSuccess, error: toastError } = useToast()
import { adminApi } from '@/services/api'

// ==========================================================================
// Data
// ==========================================================================
const loading = ref(true)
const logs = ref([])

// Default to today
const today = new Date()
const selectedDate = ref(
  today.getFullYear() + '-' +
  String(today.getMonth() + 1).padStart(2, '0') + '-' +
  String(today.getDate()).padStart(2, '0')
)

// ==========================================================================
// Helpers
// ==========================================================================
function formatTime(t) {
  if (!t) return ''
  return new Date(t).toLocaleString()
}

function downloadTypeLabel(type) {
  const labels = {
    subscription: '订阅下载',
    share: '分享下载',
    custom: '自定义订阅',
    rule: '规则下载'
  }
  return labels[type] || type || '—'
}

function typeTagType(type) {
  const types = {
    subscription: '',
    share: 'success',
    custom: 'warning',
    rule: 'info'
  }
  return types[type] || ''
}

// ==========================================================================
// Data Loading
// ==========================================================================
async function fetchLogs() {
  loading.value = true
  try {
    const res = await adminApi.logs.getLogs(selectedDate.value)
    logs.value = res.data.logs || []
  } catch (e) {
    toastError('加载日志失败')
    logs.value = []
  } finally {
    loading.value = false
  }
}

// ==========================================================================
// Lifecycle
// ==========================================================================
onMounted(() => {
  fetchLogs()
})
</script>

<style scoped>
.logs-container {
  padding: 0;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
  flex-wrap: wrap;
  gap: 12px;
}

.page-header h2 {
  margin: 0;
  font-size: 20px;
  font-weight: 600;
}

.logs-table {
  width: 100%;
}

.error-reason {
  color: var(--el-color-danger);
}

.no-data {
  color: var(--el-text-color-secondary);
}
</style>
