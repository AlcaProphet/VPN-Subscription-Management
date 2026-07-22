<template>
  <div class="min-h-screen bg-gray-50 dark:bg-gray-900">
    <!-- Top bar -->
    <header class="flex items-center justify-between px-6 py-3 bg-white dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700 flex-wrap gap-2">
      <div class="flex items-center gap-4">
        <h1 class="m-0 text-xl font-bold text-gray-900 dark:text-white">VPN 订阅</h1>
        <span v-if="updateTime" class="text-sm text-gray-500 dark:text-gray-400">
          更新于 {{ formatTime(updateTime) }}
        </span>
        <span v-else class="text-sm text-gray-400 dark:text-gray-500 italic">
          暂无更新
        </span>
      </div>
      <div class="flex items-center gap-2">
        <button
          v-if="userStore.isAdmin"
          class="bg-blue-600 hover:bg-blue-700 text-white rounded-md px-3 py-1.5 text-xs"
          @click="router.push('/admin')"
        >
          管理面板
        </button>
        <span class="text-sm text-gray-700 dark:text-gray-300">{{ userStore.user?.username }}</span>
        <span class="rounded-full px-2 py-0.5 text-xs font-medium"
          :class="roleTagClass">
          {{ roleLabel }}
        </span>
        <button
          class="bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-600 rounded-md px-3 py-1.5 text-xs"
          @click="handleLogout"
        >
          退出
        </button>
        <button
          class="w-8 h-8 rounded-full border border-gray-300 dark:border-gray-600 flex items-center justify-center bg-white dark:bg-gray-700 hover:bg-gray-50 dark:hover:bg-gray-600"
          @click="toggleTheme"
        >
          <svg v-if="isDark" class="w-4 h-4 text-yellow-500" fill="currentColor" viewBox="0 0 24 24">
            <path d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z"/>
          </svg>
          <svg v-else class="w-4 h-4 text-gray-600" fill="currentColor" viewBox="0 0 24 24">
            <path d="M21.752 15.002A9.718 9.718 0 0118 15.75c-5.385 0-9.75-4.365-9.75-9.75 0-1.33.266-2.597.748-3.752A9.753 9.753 0 003 11.25C3 16.635 7.365 21 12.75 21a9.753 9.753 0 009.002-5.998z"/>
          </svg>
        </button>
      </div>
    </header>

    <!-- Platform cards -->
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
      <div v-else-if="platforms.length === 0" class="text-center py-12 text-gray-400 dark:text-gray-500">
        暂无平台，请联系管理员
      </div>

      <!-- Platform cards grid -->
      <div v-else class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-5">
        <div
          v-for="p in platforms"
          :key="p.id"
          class="bg-white dark:bg-gray-800 rounded-lg shadow-md overflow-hidden"
        >
          <!-- Card header -->
          <div class="px-4 py-3 border-b border-gray-200 dark:border-gray-700">
            <span class="text-base font-semibold text-gray-900 dark:text-white">{{ p.name }}</span>
          </div>

          <!-- Card body -->
          <div class="p-4">
            <p class="m-0 mb-4 text-sm text-gray-500 dark:text-gray-400 leading-relaxed">{{ p.description }}</p>

            <!-- Subscription sections -->
            <div class="flex flex-col gap-3">
              <!-- Branch 1: Custom subscription (replaces default/advanced) -->
              <template v-if="p.has_custom_sub">
                <div class="p-3 rounded-lg bg-gray-100 dark:bg-gray-700/50">
                  <span class="rounded-full px-2 py-0.5 text-xs font-medium bg-orange-100 dark:bg-orange-900/30 text-orange-700 dark:text-orange-300 mb-2 inline-block">
                    已被分配自定义订阅
                  </span>
                  <div class="flex items-center gap-2 flex-wrap mt-2">
                    <button
                      class="bg-blue-600 hover:bg-blue-700 text-white rounded-md px-3 py-1.5 text-xs disabled:opacity-50 disabled:cursor-not-allowed"
                      :disabled="!p.download_token"
                      @click="handleImport(p, p.download_token)"
                    >
                      一键导入
                    </button>
                    <button
                      class="bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-600 rounded-md px-3 py-1.5 text-xs disabled:opacity-50 disabled:cursor-not-allowed"
                      :disabled="!p.download_token"
                      @click="showCopyDialog(p, p.download_token, 'custom')"
                    >
                      复制链接
                    </button>
                    <button
                      class="text-yellow-600 hover:text-yellow-700 text-xs disabled:opacity-50 disabled:cursor-not-allowed"
                      :disabled="!p.download_token"
                      @click="handleRefresh(p, 'custom')"
                    >
                      <svg v-if="refreshing[p.id + '-custom']" class="animate-spin h-3 w-3 inline-block mr-1" fill="none" viewBox="0 0 24 24">
                        <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
                        <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/>
                      </svg>
                      刷新链接
                    </button>
                  </div>
                </div>
              </template>

              <!-- Branch 2: Normal user, default subscription -->
              <template v-if="!p.has_custom_sub && p.sub_type === 'default'">
                <div class="p-3 rounded-lg bg-gray-100 dark:bg-gray-700/50">
                  <span class="rounded-full px-2 py-0.5 text-xs font-medium bg-gray-200 dark:bg-gray-600 text-gray-700 dark:text-gray-300 mb-2 inline-block">
                    默认订阅
                  </span>
                  <div v-if="p.default_configured" class="flex items-center gap-2 flex-wrap mt-2">
                    <button
                      class="bg-blue-600 hover:bg-blue-700 text-white rounded-md px-3 py-1.5 text-xs disabled:opacity-50 disabled:cursor-not-allowed"
                      :disabled="!p.download_token"
                      @click="handleImport(p, p.download_token)"
                    >
                      一键导入
                    </button>
                    <button
                      class="bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-600 rounded-md px-3 py-1.5 text-xs disabled:opacity-50 disabled:cursor-not-allowed"
                      :disabled="!p.download_token"
                      @click="showCopyDialog(p, p.download_token, 'default')"
                    >
                      复制链接
                    </button>
                    <button
                      class="text-yellow-600 hover:text-yellow-700 text-xs disabled:opacity-50 disabled:cursor-not-allowed"
                      :disabled="!p.download_token"
                      @click="handleRefresh(p, 'default')"
                    >
                      <svg v-if="refreshing[p.id + '-default']" class="animate-spin h-3 w-3 inline-block mr-1" fill="none" viewBox="0 0 24 24">
                        <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
                        <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/>
                      </svg>
                      刷新链接
                    </button>
                  </div>
                  <p v-else class="m-0 text-sm text-gray-400 dark:text-gray-500 italic">
                    默认订阅未配置，请联系管理员
                  </p>
                </div>
              </template>

              <!-- Branch 3: Advanced user, advanced subscription -->
              <template v-if="!p.has_custom_sub && p.sub_type === 'advanced'">
                <div class="p-3 rounded-lg bg-gray-100 dark:bg-gray-700/50">
                  <span class="rounded-full px-2 py-0.5 text-xs font-medium bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300 mb-2 inline-block">
                    高级订阅
                  </span>
                  <div v-if="p.advanced_configured" class="flex items-center gap-2 flex-wrap mt-2">
                    <button
                      class="bg-blue-600 hover:bg-blue-700 text-white rounded-md px-3 py-1.5 text-xs disabled:opacity-50 disabled:cursor-not-allowed"
                      :disabled="!p.download_token"
                      @click="handleImport(p, p.download_token)"
                    >
                      一键导入
                    </button>
                    <button
                      class="bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-600 rounded-md px-3 py-1.5 text-xs disabled:opacity-50 disabled:cursor-not-allowed"
                      :disabled="!p.download_token"
                      @click="showCopyDialog(p, p.download_token, 'advanced')"
                    >
                      复制链接
                    </button>
                    <button
                      class="text-yellow-600 hover:text-yellow-700 text-xs disabled:opacity-50 disabled:cursor-not-allowed"
                      :disabled="!p.download_token"
                      @click="handleRefresh(p, 'advanced')"
                    >
                      <svg v-if="refreshing[p.id + '-advanced']" class="animate-spin h-3 w-3 inline-block mr-1" fill="none" viewBox="0 0 24 24">
                        <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
                        <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/>
                      </svg>
                      刷新链接
                    </button>
                  </div>
                  <p v-else class="m-0 text-sm text-gray-400 dark:text-gray-500 italic">
                    高级订阅未配置，请联系管理员
                  </p>
                </div>
              </template>

              <!-- Branch 4: Admin preview default (when admin's primary is advanced) -->
              <template v-if="userStore.isAdmin && !p.has_custom_sub && p.sub_type === 'advanced' && p.default_configured">
                <div class="p-3 rounded-lg bg-gray-50 dark:bg-gray-700/30 border border-dashed border-gray-300 dark:border-gray-600">
                  <span class="rounded-full px-2 py-0.5 text-xs font-medium bg-gray-200 dark:bg-gray-600 text-gray-700 dark:text-gray-300 mb-2 inline-block">
                    默认订阅（预览）
                  </span>
                  <div class="flex items-center gap-2 flex-wrap mt-2">
                    <button
                      class="bg-blue-600 hover:bg-blue-700 text-white rounded-md px-3 py-1.5 text-xs disabled:opacity-50 disabled:cursor-not-allowed"
                      :disabled="!p.preview_token"
                      @click="handleImport(p, p.preview_token)"
                    >
                      一键导入
                    </button>
                    <button
                      class="bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-600 rounded-md px-3 py-1.5 text-xs disabled:opacity-50 disabled:cursor-not-allowed"
                      :disabled="!p.preview_token"
                      @click="showCopyDialog(p, p.preview_token, 'default')"
                    >
                      复制链接
                    </button>
                    <button
                      class="text-yellow-600 hover:text-yellow-700 text-xs disabled:opacity-50 disabled:cursor-not-allowed"
                      :disabled="!p.preview_token"
                      @click="handleRefresh(p, 'default')"
                    >
                      <svg v-if="refreshing[p.id + '-default']" class="animate-spin h-3 w-3 inline-block mr-1" fill="none" viewBox="0 0 24 24">
                        <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
                        <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/>
                      </svg>
                      刷新链接
                    </button>
                  </div>
                </div>
              </template>

              <!-- Branch 5: Admin preview advanced (when admin's primary is default) -->
              <template v-if="userStore.isAdmin && !p.has_custom_sub && p.sub_type === 'default' && p.advanced_configured">
                <div class="p-3 rounded-lg bg-gray-50 dark:bg-gray-700/30 border border-dashed border-gray-300 dark:border-gray-600">
                  <span class="rounded-full px-2 py-0.5 text-xs font-medium bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300 mb-2 inline-block">
                    高级订阅（预览）
                  </span>
                  <div class="flex items-center gap-2 flex-wrap mt-2">
                    <button
                      class="bg-blue-600 hover:bg-blue-700 text-white rounded-md px-3 py-1.5 text-xs disabled:opacity-50 disabled:cursor-not-allowed"
                      :disabled="!p.preview_token"
                      @click="handleImport(p, p.preview_token)"
                    >
                      一键导入
                    </button>
                    <button
                      class="bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-600 rounded-md px-3 py-1.5 text-xs disabled:opacity-50 disabled:cursor-not-allowed"
                      :disabled="!p.preview_token"
                      @click="showCopyDialog(p, p.preview_token, 'advanced')"
                    >
                      复制链接
                    </button>
                    <button
                      class="text-yellow-600 hover:text-yellow-700 text-xs disabled:opacity-50 disabled:cursor-not-allowed"
                      :disabled="!p.preview_token"
                      @click="handleRefresh(p, 'advanced')"
                    >
                      <svg v-if="refreshing[p.id + '-advanced']" class="animate-spin h-3 w-3 inline-block mr-1" fill="none" viewBox="0 0 24 24">
                        <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
                        <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/>
                      </svg>
                      刷新链接
                    </button>
                  </div>
                </div>
              </template>

              <!-- Branch 6: Admin, unconfigured default -->
              <template v-if="userStore.isAdmin && !p.has_custom_sub && p.sub_type === 'advanced' && !p.default_configured">
                <p class="m-0 text-sm text-gray-400 dark:text-gray-500 italic">默认订阅未配置</p>
              </template>

              <!-- Branch 7: Admin, unconfigured advanced -->
              <template v-if="userStore.isAdmin && !p.has_custom_sub && p.sub_type === 'default' && !p.advanced_configured">
                <p class="m-0 text-sm text-gray-400 dark:text-gray-500 italic">高级订阅未配置</p>
              </template>

              <!-- Branch 8: Admin with custom sub, show preview for default & advanced -->
              <template v-if="userStore.isAdmin && p.has_custom_sub">
                <div v-if="p.preview_token && p.preview_sub_type === 'default'" class="p-3 rounded-lg bg-gray-50 dark:bg-gray-700/30 border border-dashed border-gray-300 dark:border-gray-600">
                  <span class="rounded-full px-2 py-0.5 text-xs font-medium bg-gray-200 dark:bg-gray-600 text-gray-700 dark:text-gray-300 mb-2 inline-block">
                    默认订阅（预览）
                  </span>
                  <div class="flex items-center gap-2 flex-wrap mt-2">
                    <button
                      class="bg-blue-600 hover:bg-blue-700 text-white rounded-md px-3 py-1.5 text-xs disabled:opacity-50 disabled:cursor-not-allowed"
                      :disabled="!p.preview_token"
                      @click="handleImport(p, p.preview_token)"
                    >
                      一键导入
                    </button>
                    <button
                      class="bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-600 rounded-md px-3 py-1.5 text-xs disabled:opacity-50 disabled:cursor-not-allowed"
                      :disabled="!p.preview_token"
                      @click="showCopyDialog(p, p.preview_token, 'default')"
                    >
                      复制链接
                    </button>
                  </div>
                </div>
                <div v-if="p.preview_token2 && p.preview_sub_type2 === 'advanced'" class="p-3 rounded-lg bg-gray-50 dark:bg-gray-700/30 border border-dashed border-gray-300 dark:border-gray-600">
                  <span class="rounded-full px-2 py-0.5 text-xs font-medium bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300 mb-2 inline-block">
                    高级订阅（预览）
                  </span>
                  <div class="flex items-center gap-2 flex-wrap mt-2">
                    <button
                      class="bg-blue-600 hover:bg-blue-700 text-white rounded-md px-3 py-1.5 text-xs disabled:opacity-50 disabled:cursor-not-allowed"
                      :disabled="!p.preview_token2"
                      @click="handleImport(p, p.preview_token2)"
                    >
                      一键导入
                    </button>
                    <button
                      class="bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-600 rounded-md px-3 py-1.5 text-xs disabled:opacity-50 disabled:cursor-not-allowed"
                      :disabled="!p.preview_token2"
                      @click="showCopyDialog(p, p.preview_token2, 'advanced')"
                    >
                      复制链接
                    </button>
                  </div>
                </div>
                <p v-if="!p.default_configured" class="m-0 text-sm text-gray-400 dark:text-gray-500 italic">默认订阅未配置</p>
                <p v-if="!p.advanced_configured" class="m-0 text-sm text-gray-400 dark:text-gray-500 italic">高级订阅未配置</p>
              </template>
            </div>

            <!-- Download client button -->
            <div v-if="p.download_url" class="mt-4 pt-3 border-t border-gray-100 dark:border-gray-700 text-center">
              <a
                :href="p.download_url"
                target="_blank"
                class="text-sm text-blue-600 hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300 no-underline hover:underline"
              >
                下载客户端
              </a>
            </div>
          </div>
        </div>
      </div>
    </main>

    <!-- Copy link dialog -->
    <el-dialog
      v-model="copyDialogVisible"
      title="复制订阅链接"
      width="500px"
      :append-to-body="true"
    >
      <input
        ref="copyInputRef"
        :value="copyUrl"
        readonly
        class="w-full rounded-md border border-gray-300 dark:border-gray-600 bg-gray-50 dark:bg-gray-700 px-3 py-2 text-sm text-gray-700 dark:text-gray-300 cursor-pointer"
        @click="handleCopyToClipboard"
      />
      <template #footer>
        <div class="flex justify-end">
          <button
            class="bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-600 rounded-md px-4 py-2 text-sm"
            @click="copyDialogVisible = false"
          >
            关闭
          </button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { useUserStore } from '@/stores/user'
import { useTheme } from '@/composables/useTheme'
import { useToast } from '@/composables/useToast'
import { userApi, downloadApi } from '@/services/api'

const router = useRouter()
const userStore = useUserStore()
const { isDark, toggle: toggleTheme } = useTheme()
const { success: toastSuccess, error: toastError } = useToast()

// State
const loading = ref(true)
const platforms = ref([])
const updateTime = ref('')
const refreshing = reactive({})

// Copy dialog state
const copyDialogVisible = ref(false)
const copyUrl = ref('')
const copyInputRef = ref(null)

// Role display
const roleTagClass = computed(() => {
  if (userStore.isAdmin) return 'bg-purple-100 dark:bg-purple-900/30 text-purple-700 dark:text-purple-300'
  if (userStore.isAdvanced) return 'bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300'
  return 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300'
})

const roleLabel = computed(() => {
  if (userStore.isAdmin) return '管理员'
  if (userStore.isAdvanced) return '高级用户'
  return '普通用户'
})

function formatTime(t) {
  if (!t) return ''
  return new Date(t).toLocaleString()
}

async function fetchPlatforms() {
  loading.value = true
  try {
    const res = await userApi.getUserPlatforms()
    platforms.value = res.data.platforms || []
  } catch (e) {
    toastError('获取平台列表失败')
  } finally {
    loading.value = false
  }
}

async function fetchUpdateTime() {
  try {
    const res = await userApi.getUpdateTime()
    updateTime.value = res.data.update_time || ''
  } catch (e) {
    // Silently fail — update time is non-critical
  }
}

function buildDownloadUrl(platform, token) {
  return downloadApi.downloadByTokenUrl(platform.id, token)
}

function buildImportUrl(platform, token) {
  const scheme = platform.client_schemes?.[0]
  if (!scheme) return '#'
  const downloadUrl = buildDownloadUrl(platform, token)
  // Build absolute URL using window.location.origin
  const fullDownloadUrl = window.location.origin + downloadUrl
  return scheme + encodeURIComponent(fullDownloadUrl)
}

function handleImport(platform, token) {
  if (!token) return
  const url = buildImportUrl(platform, token)
  window.location.href = url
}

function showCopyDialog(platform, token, subType) {
  if (!token) return
  copyUrl.value = window.location.origin + buildDownloadUrl(platform, token)
  copyDialogVisible.value = true
  nextTick(() => {
    handleCopyToClipboard()
  })
}

async function handleCopyToClipboard() {
  try {
    await navigator.clipboard.writeText(copyUrl.value)
    toastSuccess('已复制到剪贴板')
  } catch (e) {
    // Fallback: select the native input element
    if (copyInputRef.value) {
      copyInputRef.value.select()
      document.execCommand('copy')
      toastSuccess('已复制到剪贴板')
    }
  }
}

async function handleRefresh(platform, subType) {
  const key = platform.id + '-' + subType
  refreshing[key] = true
  try {
    const typeParam = subType === 'custom' ? 'custom' : subType
    await userApi.refreshToken(platform.id, typeParam)
    toastSuccess('链接已刷新')
    await fetchPlatforms()
  } catch (e) {
    toastError('刷新失败')
  } finally {
    refreshing[key] = false
  }
}

function handleLogout() {
  userStore.logout(router)
}

// Lifecycle
onMounted(async () => {
  await Promise.all([fetchPlatforms(), fetchUpdateTime()])
})
</script>
