<template>
  <div>
    <div class="flex justify-between items-center mb-5 flex-wrap gap-3">
      <h2 class="m-0 text-xl font-semibold text-gray-900 dark:text-white">日志查看</h2>
      <input
        type="date"
        :value="selectedDate"
        @change="selectedDate = $event.target.value; fetchLogs()"
        class="rounded-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-3 py-2 text-sm text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none"
      />
    </div>

    <div v-if="loading" class="flex items-center justify-center py-12">
      <svg class="animate-spin h-5 w-5 mr-2 text-blue-600" fill="none" viewBox="0 0 24 24">
        <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
        <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/>
      </svg>
      <span class="text-gray-500 dark:text-gray-400">加载中...</span>
    </div>

    <div v-else-if="logs.length === 0" class="text-center py-12 text-gray-400 dark:text-gray-500">
      暂无日志记录
    </div>

    <div v-else class="w-full overflow-x-auto">
      <el-table :data="logs" stripe>
      <el-table-column label="时间" width="180"><template #default="{ row }">{{ formatTime(row.created_at) }}</template></el-table-column>
      <el-table-column label="下载类型" width="130">
        <template #default="{ row }">
          <span class="rounded-full px-2 py-0.5 text-xs font-medium" :class="typeTagClass(row.download_type)">{{ downloadTypeLabel(row.download_type) }}</span>
        </template>
      </el-table-column>
      <el-table-column v-if="!isMobile" label="用户 ID" width="140" show-overflow-tooltip><template #default="{ row }">{{ row.user_id || '—' }}</template></el-table-column>
      <el-table-column v-if="!isMobile" label="平台" width="130"><template #default="{ row }">{{ row.platform || '—' }}</template></el-table-column>
      <el-table-column label="状态" width="80">
        <template #default="{ row }">
          <span class="rounded-full px-2 py-0.5 text-xs font-medium" :class="row.status === 'success' ? 'bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-300' : 'bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-300'">{{ row.status === 'success' ? '成功' : '失败' }}</span>
        </template>
      </el-table-column>
      <el-table-column label="失败原因" min-width="160" show-overflow-tooltip>
        <template #default="{ row }">
          <span v-if="row.error_reason" class="text-red-600 dark:text-red-400">{{ row.error_reason }}</span>
          <span v-else class="text-gray-400 dark:text-gray-500">—</span>
        </template>
      </el-table-column>
      <el-table-column label="IP" width="150"><template #default="{ row }">{{ row.ip }}</template></el-table-column>
    </el-table>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useToast } from '@/composables/useToast'
import { adminApi } from '@/services/api'
import { useIsMobile } from '@/composables/useIsMobile'

const { error: toastError } = useToast()
const isMobile = useIsMobile()

const loading = ref(true)
const logs = ref([])

const today = new Date()
const selectedDate = ref(
  today.getFullYear() + '-' +
  String(today.getMonth() + 1).padStart(2, '0') + '-' +
  String(today.getDate()).padStart(2, '0')
)

function formatTime(t) {
  if (!t) return ''
  return new Date(t).toLocaleString()
}

function downloadTypeLabel(type) {
  const labels = { subscription: '订阅下载', share: '分享下载', custom: '自定义订阅', rule: '规则下载' }
  return labels[type] || type || '—'
}

function typeTagClass(type) {
  const classes = {
    subscription: 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300',
    share: 'bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-300',
    custom: 'bg-orange-100 dark:bg-orange-900/30 text-orange-700 dark:text-orange-300',
    rule: 'bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300'
  }
  return classes[type] || 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300'
}

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

onMounted(() => { fetchLogs() })
</script>
