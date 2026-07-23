<template>
  <div>
    <div v-if="loading" class="flex items-center justify-center py-12">
      <svg class="animate-spin h-5 w-5 mr-2 text-blue-600" fill="none" viewBox="0 0 24 24">
        <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
        <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/>
      </svg>
      <span class="text-gray-500 dark:text-gray-400">加载中...</span>
    </div>

    <template v-else>
      <h2 class="m-0 mb-5 text-xl font-semibold text-gray-900 dark:text-white">面板配置</h2>

      <!-- OIDC Config Card -->
      <div class="bg-white dark:bg-gray-800 rounded-lg shadow-md mb-5">
        <div class="px-6 py-4 border-b border-gray-200 dark:border-gray-700 flex justify-between items-center">
          <span class="font-medium text-gray-900 dark:text-white">OIDC 提供商配置</span>
          <span class="rounded-full px-2 py-0.5 text-xs font-medium"
            :class="providerTagClass">{{ providerLabel }}</span>
        </div>

        <div class="p-6">
          <el-form ref="oidcFormRef" :model="oidcForm" :rules="oidcRules" label-position="top">
            <el-form-item>
              <button class="bg-orange-50 dark:bg-orange-900/20 border border-orange-300 dark:border-orange-700 text-orange-700 dark:text-orange-300 hover:bg-orange-100 dark:hover:bg-orange-900/30 rounded-md px-3 py-1 text-sm" @click="showSwitchDialog = true">切换提供商</button>
            </el-form-item>

            <template v-if="oidcForm.provider_type === 'keycloak'">
              <el-form-item label="Keycloak Base URL" prop="keycloak_base_url">
                <input v-model="oidcForm.keycloak_base_url" placeholder="https://keycloak.example.com" class="w-full rounded-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-3 py-2 text-base text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none" @blur="oidcFormRef.validateField('keycloak_base_url')" />
              </el-form-item>
              <el-form-item label="Keycloak Realm" prop="keycloak_realm">
                <input v-model="oidcForm.keycloak_realm" placeholder="my-realm" class="w-full rounded-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-3 py-2 text-base text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none" @blur="oidcFormRef.validateField('keycloak_realm')" />
              </el-form-item>
            </template>
            <template v-if="oidcForm.provider_type === 'auth0'">
              <el-form-item label="Auth0 Domain" prop="auth0_domain">
                <input v-model="oidcForm.auth0_domain" placeholder="your-tenant.auth0.com" class="w-full rounded-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-3 py-2 text-base text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none" @blur="oidcFormRef.validateField('auth0_domain')" />
              </el-form-item>
            </template>
            <template v-if="oidcForm.provider_type === 'generic'">
              <el-form-item label="Issuer URL" prop="generic_issuer">
                <input v-model="oidcForm.generic_issuer" placeholder="https://oidc.example.com" class="w-full rounded-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-3 py-2 text-base text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none" @blur="oidcFormRef.validateField('generic_issuer')" />
              </el-form-item>
            </template>

            <el-form-item label="Client ID" prop="client_id">
              <input v-model="oidcForm.client_id" placeholder="your-client-id" class="w-full rounded-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-3 py-2 text-base text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none" @blur="oidcFormRef.validateField('client_id')" />
            </el-form-item>
            <el-form-item label="Client Secret" prop="client_secret">
              <input v-model="oidcForm.client_secret" type="password" placeholder="留空则不修改" class="w-full rounded-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-3 py-2 text-base text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none" />
              <div class="text-xs text-gray-400 dark:text-gray-500 mt-1">已存储的 Client Secret 已加密，回显为 ••••••。输入新值将覆盖</div>
            </el-form-item>
            <el-form-item label="回调地址 (Redirect URI)" prop="redirect_uri">
              <input v-model="oidcForm.redirect_uri" placeholder="https://vpn.example.com/api/v1/auth/callback" class="w-full rounded-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-3 py-2 text-base text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none" @blur="oidcFormRef.validateField('redirect_uri')" />
            </el-form-item>
            <el-form-item label="前端地址 (Frontend URL)" prop="frontend_url">
              <input v-model="oidcForm.frontend_url" placeholder="https://vpn.example.com" class="w-full rounded-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-3 py-2 text-base text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none" @blur="oidcFormRef.validateField('frontend_url')" />
            </el-form-item>

            <el-form-item>
              <div class="flex gap-3">
                <button class="bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-600 rounded-md px-4 py-2 text-sm disabled:opacity-50" :disabled="saving" @click="handleTest">
                  <svg v-if="testing" class="animate-spin -ml-1 mr-2 h-4 w-4 inline-block text-gray-700" fill="none" viewBox="0 0 24 24">
                    <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
                    <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/>
                  </svg>
                  测试连接
                </button>
                <button class="bg-blue-600 hover:bg-blue-700 text-white rounded-md px-4 py-2 text-sm disabled:opacity-50" :disabled="testing" @click="handleSave">
                  <svg v-if="saving" class="animate-spin -ml-1 mr-2 h-4 w-4 inline-block text-white" fill="none" viewBox="0 0 24 24">
                    <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
                    <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/>
                  </svg>
                  保存配置
                </button>
              </div>
            </el-form-item>
          </el-form>
        </div>
      </div>

      <!-- Rate Limit Card -->
      <div class="bg-white dark:bg-gray-800 rounded-lg shadow-md mb-5">
        <div class="px-6 py-4 border-b border-gray-200 dark:border-gray-700">
          <span class="font-medium text-gray-900 dark:text-white">速率限制配置</span>
        </div>

        <div class="p-6">
          <el-form ref="rateFormRef" :model="rateForm" :rules="rateRules" label-position="top">
            <div class="grid grid-cols-1 md:grid-cols-2 gap-5">
              <el-form-item label="登录 API 限制 (次/分钟)" prop="rate_limit_login">
                <input v-model.number="rateForm.rate_limit_login" type="number" min="1" max="1000" class="w-full rounded-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-3 py-2 text-base text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none" @blur="rateFormRef.validateField('rate_limit_login')" />
              </el-form-item>
              <el-form-item label="下载 API 限制 (次/分钟)" prop="rate_limit_download">
                <input v-model.number="rateForm.rate_limit_download" type="number" min="1" max="1000" class="w-full rounded-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-3 py-2 text-base text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none" @blur="rateFormRef.validateField('rate_limit_download')" />
              </el-form-item>
            </div>
            <el-form-item>
              <button class="bg-blue-600 hover:bg-blue-700 text-white rounded-md px-4 py-2 text-sm disabled:opacity-50" :disabled="rateSaving" @click="handleRateSave">保存速率限制</button>
            </el-form-item>
          </el-form>
        </div>
      </div>

      <!-- Announcement Card -->
      <div class="bg-white dark:bg-gray-800 rounded-lg shadow-md mb-5">
        <div class="px-6 py-4 border-b border-gray-200 dark:border-gray-700">
          <span class="font-medium text-gray-900 dark:text-white">公告栏</span>
        </div>
        <div class="p-6">
          <el-form label-position="top">
            <el-form-item label="公告内容（支持多行文本，留空则不显示）">
              <textarea v-model="announcementContent" :rows="4" placeholder="输入公告内容..."
                class="w-full rounded-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-3 py-2 text-base text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none resize-y"></textarea>
            </el-form-item>
            <el-form-item>
              <button class="bg-blue-600 hover:bg-blue-700 text-white rounded-md px-4 py-2 text-sm disabled:opacity-50" :disabled="announcementSaving" @click="handleAnnouncementSave">保存公告</button>
            </el-form-item>
          </el-form>
        </div>
      </div>
    </template>

    <OIDCSwitchDialog v-model:visible="showSwitchDialog" :current-provider="oidcForm.provider_type" @switch="handleProviderSwitch" />
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { useToast } from '@/composables/useToast'
import { adminApi } from '@/services/api'
import OIDCSwitchDialog from '@/components/OIDCSwitchDialog.vue'

const { success: toastSuccess, error: toastError } = useToast()

// ==========================================================================
// Data
// ==========================================================================
const loading = ref(true)
const showSwitchDialog = ref(false)

// OIDC
const oidcFormRef = ref(null)
const testing = ref(false)
const saving = ref(false)
const oidcForm = reactive({
  provider_type: 'keycloak',
  keycloak_base_url: '',
  keycloak_realm: '',
  auth0_domain: '',
  generic_issuer: '',
  client_id: '',
  client_secret: '',
  redirect_uri: '',
  frontend_url: ''
})

const oidcRules = computed(() => {
  const base = {
    client_id: [{ required: true, message: '请输入 Client ID', trigger: 'blur' }],
    redirect_uri: [{ required: true, message: '请输入回调地址', trigger: 'blur' }],
    frontend_url: [{ required: true, message: '请输入前端地址', trigger: 'blur' }]
  }
  if (oidcForm.provider_type === 'keycloak') {
    return {
      ...base,
      keycloak_base_url: [{ required: true, message: '请输入 Keycloak Base URL', trigger: 'blur' }],
      keycloak_realm: [{ required: true, message: '请输入 Keycloak Realm', trigger: 'blur' }]
    }
  } else if (oidcForm.provider_type === 'auth0') {
    return {
      ...base,
      auth0_domain: [{ required: true, message: '请输入 Auth0 Domain', trigger: 'blur' }]
    }
  } else {
    return {
      ...base,
      generic_issuer: [{ required: true, message: '请输入 Issuer URL', trigger: 'blur' }]
    }
  }
})

// Rate limit
const rateFormRef = ref(null)
const rateSaving = ref(false)
const rateForm = reactive({
  rate_limit_login: 10,
  rate_limit_download: 20
})

// Announcement
const announcementContent = ref('')
const announcementSaving = ref(false)

const rateRules = {
  rate_limit_login: [{ required: true, message: '请输入登录限制', trigger: 'blur' }],
  rate_limit_download: [{ required: true, message: '请输入下载限制', trigger: 'blur' }]
}

// ==========================================================================
// Computed
// ==========================================================================
const providerLabel = computed(() => {
  const labels = { keycloak: 'Keycloak', auth0: 'Auth0', generic: '通用 OIDC' }
  return labels[oidcForm.provider_type] || 'Keycloak'
})

const providerTagClass = computed(() => {
  const classes = { keycloak: 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300', auth0: 'bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-300', generic: 'bg-yellow-100 dark:bg-yellow-900/30 text-yellow-700 dark:text-yellow-300' }
  return classes[oidcForm.provider_type] || ''
})

// ==========================================================================
// Data Loading
// ==========================================================================
async function loadConfig() {
  try {
    const [oidcRes, rateRes] = await Promise.all([
      adminApi.system.getOIDCConfig(),
      adminApi.system.getRateLimit()
    ])

    // Parse OIDC config
    const cfg = oidcRes.data.config || {}
    oidcForm.provider_type = cfg.provider_type || 'keycloak'
    oidcForm.keycloak_base_url = cfg.keycloak_base_url || ''
    oidcForm.keycloak_realm = cfg.keycloak_realm || ''
    oidcForm.auth0_domain = cfg.auth0_domain || ''
    oidcForm.generic_issuer = cfg.generic_issuer || ''
    oidcForm.client_id = cfg.client_id || ''
    oidcForm.client_secret = cfg.client_secret || ''  // backend returns masked value or empty
    oidcForm.redirect_uri = cfg.redirect_uri || ''
    oidcForm.frontend_url = cfg.frontend_url || ''

    // Parse rate limit
    rateForm.rate_limit_login = rateRes.data.rate_limit_login || 10
    rateForm.rate_limit_download = rateRes.data.rate_limit_download || 20
  } catch (e) {
    toastError('加载配置失败')
  }
}

// ==========================================================================
// OIDC Operations
// ==========================================================================
function buildOIDCPayload() {
  const payload = {
    provider_type: oidcForm.provider_type,
    client_id: oidcForm.client_id,
    client_secret: oidcForm.client_secret,
    redirect_uri: oidcForm.redirect_uri,
    frontend_url: oidcForm.frontend_url
  }
  if (oidcForm.provider_type === 'keycloak') {
    payload.keycloak_base_url = oidcForm.keycloak_base_url
    payload.keycloak_realm = oidcForm.keycloak_realm
  } else if (oidcForm.provider_type === 'auth0') {
    payload.auth0_domain = oidcForm.auth0_domain
  } else {
    payload.generic_issuer = oidcForm.generic_issuer
  }
  return payload
}

async function handleTest() {
  const valid = await oidcFormRef.value.validate().catch(() => false)
  if (!valid) return

  testing.value = true
  try {
    const payload = buildOIDCPayload()
    // Don't send masked or empty secret if not changed
    if (payload.client_secret === '••••••' || payload.client_secret === '***' || payload.client_secret === '') {
      delete payload.client_secret
    }
    await adminApi.system.testOIDC(payload)
    toastSuccess('连接测试成功')
  } catch (e) {
    const msg = e.response?.data?.error || '连接测试失败'
    toastError('连接测试失败：' + msg)
  } finally {
    testing.value = false
  }
}

async function handleSave() {
  const valid = await oidcFormRef.value.validate().catch(() => false)
  if (!valid) return

  saving.value = true
  try {
    const payload = buildOIDCPayload()
    // Don't send masked or empty secret if not changed
    if (payload.client_secret === '••••••' || payload.client_secret === '***' || payload.client_secret === '') {
      delete payload.client_secret
    }
    await adminApi.system.configure(payload)
    toastSuccess('OIDC 配置已保存')
  } catch (e) {
    const msg = e.response?.data?.error || '保存失败'
    toastError(msg)
  } finally {
    saving.value = false
  }
}

async function handleProviderSwitch(provider) {
  try {
    await adminApi.system.switchProvider({ provider_type: provider })
    oidcForm.provider_type = provider
    showSwitchDialog.value = false
    toastSuccess('提供商已切换')
    // Reload config to get switched provider's saved values
    await loadConfig()
  } catch (e) {
    const msg = e.response?.data?.error || '切换失败'
    toastError(msg)
  }
}

// ==========================================================================
// Rate Limit Operations
// ==========================================================================
async function handleRateSave() {
  const valid = await rateFormRef.value.validate().catch(() => false)
  if (!valid) return

  rateSaving.value = true
  try {
    await adminApi.system.updateRateLimit({
      rate_limit_login: rateForm.rate_limit_login,
      rate_limit_download: rateForm.rate_limit_download
    })
    toastSuccess('速率限制已更新')
  } catch (e) {
    const msg = e.response?.data?.error || '更新失败'
    toastError(msg)
  } finally {
    rateSaving.value = false
  }
}

// ==========================================================================
// Announcement
// ==========================================================================
async function handleAnnouncementSave() {
  announcementSaving.value = true
  try {
    await adminApi.system.updateAnnouncement({ content: announcementContent.value })
    toastSuccess('公告已保存')
  } catch (e) {
    const msg = e.response?.data?.error || '保存失败'
    toastError(msg)
  } finally {
    announcementSaving.value = false
  }
}

async function loadAnnouncement() {
  try {
    const res = await adminApi.system.getAnnouncement()
    announcementContent.value = res.data.content || ''
  } catch (e) {
    // Non-critical
  }
}

// ==========================================================================
// Lifecycle
// ==========================================================================
onMounted(async () => {
  loading.value = true
  await Promise.all([loadConfig(), loadAnnouncement()])
  loading.value = false
})
</script>
