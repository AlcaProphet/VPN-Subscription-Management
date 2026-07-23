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
              <button
                class="bg-blue-600 hover:bg-blue-700 text-white rounded-md px-3 py-1.5 text-sm disabled:opacity-50 disabled:cursor-not-allowed"
                :disabled="fetchingLink === rule.id"
                @click="fetchDownloadLink(rule)"
              >
                <span v-if="fetchingLink === rule.id" class="flex items-center gap-1">
                  <svg class="animate-spin h-3 w-3" fill="none" viewBox="0 0 24 24">
                    <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
                    <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/>
                  </svg>
                  获取中...
                </span>
                <span v-else>获取下载链接</span>
              </button>
            </div>
          </div>
        </div>
      </div>

      <!-- Download link dialog -->
      <Teleport to="body">
        <div
          v-if="showDialog"
          class="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
          @click.self="showDialog = false"
        >
          <div class="bg-white dark:bg-gray-800 rounded-xl shadow-xl w-full max-w-md mx-4 p-5">
            <div class="flex items-center justify-between mb-4">
              <h3 class="text-base font-semibold text-gray-900 dark:text-white">
                {{ dialogRuleName }} — 下载链接
              </h3>
              <button
                class="text-gray-400 hover:text-gray-600 dark:hover:text-gray-200"
                @click="showDialog = false"
              >
                <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/>
                </svg>
              </button>
            </div>
            <div class="flex items-center gap-2 mb-4">
              <input
                ref="urlInputRef"
                :value="dialogUrl"
                readonly
                class="flex-1 px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-gray-50 dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:outline-none"
                @focus="$event.target.select()"
              />
              <button
                class="shrink-0 bg-blue-600 hover:bg-blue-700 text-white rounded-md px-3 py-2 text-sm"
                @click="copyUrl"
              >
                {{ copied ? '已复制' : '复制' }}
              </button>
            </div>
            <p class="text-xs text-gray-400 dark:text-gray-500">
              将链接粘贴到支持订阅的客户端中即可使用
            </p>
          </div>
        </div>
      </Teleport>
    </main>
  </div>
</template>

<script setup>
import { ref, onMounted, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { useToast } from '@/composables/useToast'
import { userApi } from '@/services/api'

const router = useRouter()
const { error: toastError, success: toastSuccess } = useToast()

const loading = ref(true)
const rules = ref([])

// Download link dialog state
const showDialog = ref(false)
const dialogUrl = ref('')
const dialogRuleName = ref('')
const fetchingLink = ref(null) // rule.id that is currently being fetched
const copied = ref(false)
const urlInputRef = ref(null)

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

async function fetchDownloadLink(rule) {
  fetchingLink.value = rule.id
  try {
    const res = await userApi.getRuleDownloadLink(rule.id)
    dialogUrl.value = res.data.url
    dialogRuleName.value = res.data.rule_name || rule.name
    copied.value = false
    showDialog.value = true
    // Auto-select the URL in the input for easy copying
    await nextTick()
    urlInputRef.value?.select()
  } catch (e) {
    toastError('获取下载链接失败')
  } finally {
    fetchingLink.value = null
  }
}

async function copyUrl() {
  try {
    await navigator.clipboard.writeText(dialogUrl.value)
    copied.value = true
    toastSuccess('已复制到剪贴板')
    setTimeout(() => { copied.value = false }, 2000)
  } catch {
    // Fallback for non-HTTPS environments
    urlInputRef.value?.select()
    document.execCommand('copy')
    copied.value = true
    toastSuccess('已复制到剪贴板')
    setTimeout(() => { copied.value = false }, 2000)
  }
}

function formatTime(t) {
  if (!t) return ''
  return new Date(t).toLocaleString()
}

onMounted(async () => {
  try {
    const res = await userApi.getRules()
    rules.value = res.data.rules || []
  } catch (e) {
    toastError('加载规则失败')
  } finally {
    loading.value = false
  }
})
</script>
