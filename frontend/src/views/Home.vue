<template>
  <div class="home-container">
    <!-- Top bar -->
    <header class="home-header">
      <div class="header-left">
        <h1 class="header-title">VPN 订阅</h1>
        <span class="header-update" v-if="updateTime">
          更新于 {{ formatTime(updateTime) }}
        </span>
        <span class="header-update header-no-update" v-else>
          暂无更新
        </span>
      </div>
      <div class="header-right">
        <el-button
          v-if="userStore.isAdmin"
          type="primary"
          size="small"
          @click="router.push('/admin')"
        >
          管理面板
        </el-button>
        <span class="header-username">{{ userStore.user?.username }}</span>
        <el-tag
          :type="roleTagType"
          size="small"
        >
          {{ roleLabel }}
        </el-tag>
        <el-button size="small" @click="handleLogout">
          退出
        </el-button>
        <el-button
          :icon="isDark ? Sunny : Moon"
          circle
          size="small"
          @click="toggleTheme"
        />
      </div>
    </header>

    <!-- Platform cards -->
    <main class="home-main" v-loading="loading">
      <el-empty
        v-if="!loading && platforms.length === 0"
        description="暂无平台，请联系管理员"
      />

      <el-row v-else :gutter="20">
        <el-col
          v-for="p in platforms"
          :key="p.id"
          :xs="24"
          :md="12"
          :lg="8"
        >
          <el-card class="platform-card" shadow="hover">
            <template #header>
              <div class="card-header">
                <span class="platform-name">{{ p.name }}</span>
              </div>
            </template>

            <p class="platform-desc">{{ p.description }}</p>

            <!-- Subscription sections -->
            <div class="sub-sections">
              <!-- Custom subscription (replaces default/advanced) -->
              <template v-if="p.has_custom_sub">
                <div class="sub-section">
                  <el-tag type="warning" size="small" class="sub-label">
                    已被分配自定义订阅
                  </el-tag>
                  <div class="sub-buttons">
                    <el-button
                      type="primary"
                      size="small"
                      :disabled="!p.download_token"
                      @click="handleImport(p, p.download_token)"
                    >
                      一键导入
                    </el-button>
                    <el-button
                      size="small"
                      :disabled="!p.download_token"
                      @click="showCopyDialog(p, p.download_token, 'custom')"
                    >
                      复制链接
                    </el-button>
                    <el-button
                      type="warning"
                      text
                      size="small"
                      :disabled="!p.download_token"
                      :loading="refreshing[p.id + '-custom']"
                      @click="handleRefresh(p, 'custom')"
                    >
                      刷新链接
                    </el-button>
                  </div>
                </div>
              </template>

              <!-- Default subscription (non-custom users or admin preview) -->
              <template v-if="!p.has_custom_sub && p.sub_type === 'default'">
                <!-- Normal user with default -->
                <div class="sub-section">
                  <el-tag type="info" size="small" class="sub-label">
                    默认订阅
                  </el-tag>
                  <div class="sub-buttons" v-if="p.default_configured">
                    <el-button
                      type="primary"
                      size="small"
                      :disabled="!p.download_token"
                      @click="handleImport(p, p.download_token)"
                    >
                      一键导入
                    </el-button>
                    <el-button
                      size="small"
                      :disabled="!p.download_token"
                      @click="showCopyDialog(p, p.download_token, 'default')"
                    >
                      复制链接
                    </el-button>
                    <el-button
                      type="warning"
                      text
                      size="small"
                      :disabled="!p.download_token"
                      :loading="refreshing[p.id + '-default']"
                      @click="handleRefresh(p, 'default')"
                    >
                      刷新链接
                    </el-button>
                  </div>
                  <p v-else class="sub-unconfigured">
                    默认订阅未配置，请联系管理员
                  </p>
                </div>
              </template>

              <template v-if="!p.has_custom_sub && p.sub_type === 'advanced'">
                <!-- Advanced user with advanced -->
                <div class="sub-section">
                  <el-tag type="warning" size="small" class="sub-label">
                    高级订阅
                  </el-tag>
                  <div class="sub-buttons" v-if="p.advanced_configured">
                    <el-button
                      type="primary"
                      size="small"
                      :disabled="!p.download_token"
                      @click="handleImport(p, p.download_token)"
                    >
                      一键导入
                    </el-button>
                    <el-button
                      size="small"
                      :disabled="!p.download_token"
                      @click="showCopyDialog(p, p.download_token, 'advanced')"
                    >
                      复制链接
                    </el-button>
                    <el-button
                      type="warning"
                      text
                      size="small"
                      :disabled="!p.download_token"
                      :loading="refreshing[p.id + '-advanced']"
                      @click="handleRefresh(p, 'advanced')"
                    >
                      刷新链接
                    </el-button>
                  </div>
                  <p v-else class="sub-unconfigured">
                    高级订阅未配置，请联系管理员
                  </p>
                </div>
              </template>

              <!-- Admin preview: default subscription (when admin's primary is advanced) -->
              <template v-if="userStore.isAdmin && !p.has_custom_sub && p.sub_type === 'advanced' && p.default_configured">
                <div class="sub-section admin-preview">
                  <el-tag type="info" size="small" class="sub-label">
                    默认订阅（预览）
                  </el-tag>
                  <div class="sub-buttons">
                    <el-button
                      type="primary"
                      size="small"
                      :disabled="!p.preview_token"
                      @click="handleImport(p, p.preview_token)"
                    >
                      一键导入
                    </el-button>
                    <el-button
                      size="small"
                      :disabled="!p.preview_token"
                      @click="showCopyDialog(p, p.preview_token, 'default')"
                    >
                      复制链接
                    </el-button>
                    <el-button
                      type="warning"
                      text
                      size="small"
                      :disabled="!p.preview_token"
                      :loading="refreshing[p.id + '-default']"
                      @click="handleRefresh(p, 'default')"
                    >
                      刷新链接
                    </el-button>
                  </div>
                </div>
              </template>

              <!-- Admin preview: advanced subscription (when admin's primary is default — rare) -->
              <template v-if="userStore.isAdmin && !p.has_custom_sub && p.sub_type === 'default' && p.advanced_configured">
                <div class="sub-section admin-preview">
                  <el-tag type="warning" size="small" class="sub-label">
                    高级订阅（预览）
                  </el-tag>
                  <div class="sub-buttons">
                    <el-button
                      type="primary"
                      size="small"
                      :disabled="!p.preview_token"
                      @click="handleImport(p, p.preview_token)"
                    >
                      一键导入
                    </el-button>
                    <el-button
                      size="small"
                      :disabled="!p.preview_token"
                      @click="showCopyDialog(p, p.preview_token, 'advanced')"
                    >
                      复制链接
                    </el-button>
                    <el-button
                      type="warning"
                      text
                      size="small"
                      :disabled="!p.preview_token"
                      :loading="refreshing[p.id + '-advanced']"
                      @click="handleRefresh(p, 'advanced')"
                    >
                      刷新链接
                    </el-button>
                  </div>
                </div>
              </template>

              <!-- Admin: unconfigured preview labels -->
              <template v-if="userStore.isAdmin && !p.has_custom_sub && p.sub_type === 'advanced' && !p.default_configured">
                <p class="sub-unconfigured">默认订阅未配置</p>
              </template>
              <template v-if="userStore.isAdmin && !p.has_custom_sub && p.sub_type === 'default' && !p.advanced_configured">
                <p class="sub-unconfigured">高级订阅未配置</p>
              </template>

              <!-- Admin with custom sub: also show default & advanced preview (non-functional if no tokens) -->
              <template v-if="userStore.isAdmin && p.has_custom_sub">
                <div class="sub-section admin-preview" v-if="p.default_configured">
                  <el-tag type="info" size="small" class="sub-label">
                    默认订阅（预览）
                  </el-tag>
                  <p class="sub-unconfigured">自定义订阅已激活，预览不可用</p>
                </div>
                <div class="sub-section admin-preview" v-if="p.advanced_configured">
                  <el-tag type="warning" size="small" class="sub-label">
                    高级订阅（预览）
                  </el-tag>
                  <p class="sub-unconfigured">自定义订阅已激活，预览不可用</p>
                </div>
                <p class="sub-unconfigured" v-if="!p.default_configured">默认订阅未配置</p>
                <p class="sub-unconfigured" v-if="!p.advanced_configured">高级订阅未配置</p>
              </template>
            </div>

            <!-- Download client button -->
            <div class="card-footer" v-if="p.download_url">
              <a
                :href="p.download_url"
                target="_blank"
                class="download-client-link"
              >
                下载客户端
              </a>
            </div>
          </el-card>
        </el-col>
      </el-row>
    </main>

    <!-- Copy link dialog -->
    <el-dialog
      v-model="copyDialogVisible"
      title="复制订阅链接"
      width="500px"
    >
      <el-input
        ref="copyInputRef"
        :model-value="copyUrl"
        readonly
        @click="handleCopyToClipboard"
      />
      <template #footer>
        <el-button @click="copyDialogVisible = false">关闭</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Sunny, Moon } from '@element-plus/icons-vue'
import { useUserStore } from '@/stores/user'
import { useTheme } from '@/composables/useTheme'
import { userApi, downloadApi } from '@/services/api'

const router = useRouter()
const userStore = useUserStore()
const { isDark, toggle: toggleTheme } = useTheme()

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
const roleTagType = computed(() => {
  if (userStore.isAdmin) return 'danger'
  if (userStore.isAdvanced) return 'warning'
  return 'info'
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
    ElMessage.error('获取平台列表失败')
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
    ElMessage.success('已复制到剪贴板')
  } catch (e) {
    // Fallback: access native input element inside el-input component
    const inputEl = copyInputRef.value?.$el?.querySelector('input')
    if (inputEl) {
      inputEl.select()
      document.execCommand('copy')
      ElMessage.success('已复制到剪贴板')
    }
  }
}

async function handleRefresh(platform, subType) {
  const key = platform.id + '-' + subType
  refreshing[key] = true
  try {
    const typeParam = subType === 'custom' ? platform.sub_type || 'default' : subType
    await userApi.refreshToken(platform.id, typeParam)
    ElMessage.success('链接已刷新')
    // Refetch platforms to get new tokens
    await fetchPlatforms()
  } catch (e) {
    ElMessage.error('刷新失败')
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

<style scoped>
.home-container {
  min-height: 100vh;
  background: var(--el-bg-color-page);
}

/* Header */
.home-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 24px;
  background: var(--el-bg-color);
  border-bottom: 1px solid var(--el-border-color-light);
  flex-wrap: wrap;
  gap: 8px;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 16px;
}

.header-title {
  margin: 0;
  font-size: 20px;
  font-weight: 700;
  color: var(--el-text-color-primary);
}

.header-update {
  font-size: 13px;
  color: var(--el-text-color-secondary);
}

.header-no-update {
  font-style: italic;
}

.header-right {
  display: flex;
  align-items: center;
  gap: 8px;
}

.header-username {
  font-size: 14px;
  color: var(--el-text-color-primary);
}

/* Main */
.home-main {
  padding: 24px;
}

/* Platform cards */
.platform-card {
  margin-bottom: 20px;
}

.card-header {
  display: flex;
  align-items: center;
}

.platform-name {
  font-size: 16px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.platform-desc {
  margin: 0 0 16px;
  font-size: 13px;
  color: var(--el-text-color-secondary);
  line-height: 1.5;
}

/* Subscription sections */
.sub-sections {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.sub-section {
  padding: 12px;
  border-radius: 8px;
  background: var(--el-fill-color-light);
}

.sub-section.admin-preview {
  background: var(--el-fill-color-lighter);
  border: 1px dashed var(--el-border-color);
}

.sub-label {
  margin-bottom: 8px;
}

.sub-buttons {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  margin-top: 8px;
}

.sub-unconfigured {
  margin: 0;
  font-size: 13px;
  color: var(--el-text-color-secondary);
  font-style: italic;
}

/* Card footer */
.card-footer {
  margin-top: 16px;
  padding-top: 12px;
  border-top: 1px solid var(--el-border-color-lighter);
  text-align: center;
}

.download-client-link {
  font-size: 13px;
  color: var(--el-color-primary);
  text-decoration: none;
}

.download-client-link:hover {
  text-decoration: underline;
}

/* Responsive */
@media (max-width: 768px) {
  .home-header {
    padding: 10px 16px;
  }

  .header-left {
    flex-wrap: wrap;
    gap: 8px;
  }

  .home-main {
    padding: 16px;
  }
}
</style>
