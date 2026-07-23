<template>
  <div class="min-h-screen bg-gray-50 dark:bg-gray-900">
    <!-- Top bar -->
    <header class="flex items-center justify-between px-6 py-3 bg-white dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700">
      <div class="flex items-center gap-4">
        <button
          class="text-gray-600 dark:text-gray-400 hover:text-gray-800 dark:hover:text-gray-200 text-sm flex items-center gap-1"
          @click="router.push('/')"
        >
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 19l-7-7m0 0l7-7m-7 7h18"/>
          </svg>
          首页
        </button>
        <h1 class="m-0 text-xl font-bold text-gray-900 dark:text-white">分流规则</h1>
      </div>
    </header>

    <main class="p-6">
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

      <!-- Card Grid -->
      <div v-else class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-5">
        <div
          v-for="rule in rules"
          :key="rule.id"
          class="bg-white dark:bg-gray-800 rounded-lg shadow-md overflow-hidden"
        >
          <div class="px-4 py-3 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between gap-2">
            <span class="text-sm font-semibold text-gray-900 dark:text-white truncate">{{ rule.name }}</span>
            <span class="rounded-full px-2 py-0.5 text-xs font-medium shrink-0 bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300">
              {{ rule.client_type || 'Shadowrocket' }}
            </span>
          </div>
          <div class="p-4">
            <div class="text-sm text-gray-500 dark:text-gray-400 mb-3 space-y-1">
              <div class="flex items-center gap-2">
                <span class="text-gray-400 dark:text-gray-500">当前版本:</span>
                <span v-if="currentVersion(rule) !== null" class="text-gray-700 dark:text-gray-300">v{{ currentVersion(rule) }}</span>
                <span v-else class="text-gray-400 dark:text-gray-500 italic">—</span>
              </div>
              <div v-if="currentUpdatedAt(rule)" class="flex items-center gap-2">
                <span class="text-gray-400 dark:text-gray-500">更新于:</span>
                <span class="text-gray-700 dark:text-gray-300">{{ formatTime(currentUpdatedAt(rule)) }}</span>
              </div>
            </div>
            <div class="flex justify-end">
              <a
                v-if="rule.token"
                :href="getRuleDownloadUrl(rule.id, rule.token)"
                class="no-underline"
              >
                <button class="bg-blue-600 hover:bg-blue-700 text-white rounded-md px-3 py-1.5 text-sm">
                  下载当前版本
                </button>
              </a>
              <button
                v-else
                class="bg-blue-600 text-white rounded-md px-3 py-1.5 text-sm opacity-50 cursor-not-allowed"
                disabled
                title="请联系管理员获取下载链接"
              >
                下载当前版本
              </button>
            </div>
          </div>
        </div>
      </div>
    </main>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useToast } from '@/composables/useToast'
import { publicApi } from '@/services/api'

const router = useRouter()
const { error: toastError } = useToast()

const loading = ref(true)
const rules = ref([])

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
    toastError('加载规则失败')
  } finally {
    loading.value = false
  }
})
</script>
