<template>
  <div class="min-h-screen bg-gray-50 dark:bg-gray-900">
    <header class="px-6 pt-6 pb-0 sm:px-6 sm:pt-6">
      <h1 class="m-0 mb-1 text-2xl font-bold text-gray-900 dark:text-white">分流规则</h1>
      <p class="m-0 text-sm text-gray-500 dark:text-gray-400">浏览和下载可用的分流规则配置</p>
    </header>

    <main class="px-6 py-6">
      <!-- Loading -->
      <div v-if="loading" class="flex items-center justify-center py-12">
        <svg class="animate-spin h-5 w-5 mr-2 text-blue-600" fill="none" viewBox="0 0 24 24">
          <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
          <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/>
        </svg>
        <span class="text-gray-500 dark:text-gray-400">加载中...</span>
      </div>

      <!-- Empty -->
      <div v-else-if="rules.length === 0" class="text-center py-12 text-gray-400 dark:text-gray-500">
        暂无可用规则
      </div>

      <!-- Table -->
      <el-table
        v-else
        :data="rules"
        stripe
        class="w-full"
      >
        <el-table-column prop="name" label="规则名称" min-width="180" />
        <el-table-column prop="client_type" label="客户端类型" width="140">
          <template #default="{ row }">
            <span class="rounded-full px-2 py-0.5 text-xs font-medium bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300">
              {{ row.client_type || 'Shadowrocket' }}
            </span>
          </template>
        </el-table-column>
        <el-table-column label="当前版本" width="120">
          <template #default="{ row }">
            <span v-if="currentVersion(row) !== null">
              v{{ currentVersion(row) }}
            </span>
            <span v-else class="text-gray-400 dark:text-gray-500 italic">—</span>
          </template>
        </el-table-column>
        <el-table-column label="更新时间" width="180">
          <template #default="{ row }">
            <span v-if="currentUpdatedAt(row)">
              {{ formatTime(currentUpdatedAt(row)) }}
            </span>
            <span v-else class="text-gray-400 dark:text-gray-500 italic">—</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="160" fixed="right">
          <template #default="{ row }">
            <a
              v-if="row.token"
              :href="getRuleDownloadUrl(row.id, row.token)"
              class="no-underline"
            >
              <button class="bg-blue-600 hover:bg-blue-700 text-white rounded-md px-3 py-1.5 text-xs">
                下载当前版本
              </button>
            </a>
            <button
              v-else
              class="bg-blue-600 text-white rounded-md px-3 py-1.5 text-xs opacity-50 cursor-not-allowed"
              disabled
              title="请联系管理员获取下载链接"
            >
              下载当前版本
            </button>
          </template>
        </el-table-column>
      </el-table>
    </main>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { publicApi } from '@/services/api'

const loading = ref(true)
const rules = ref([])

function currentVersion(rule) {
  if (!rule.versions || rule.versions.length === 0) return null
  // Find the version with the highest updated_at (current version)
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

function getRuleDownloadUrl(ruleId, token) {
  return publicApi.getRuleDownloadUrl(ruleId, token)
}

function formatTime(t) {
  if (!t) return ''
  return new Date(t).toLocaleString()
}

onMounted(async () => {
  try {
    const res = await publicApi.getRules()
    rules.value = res.data.rules || []
  } catch (e) {
    // Silently fail — rules page is informational
  } finally {
    loading.value = false
  }
})
</script>
