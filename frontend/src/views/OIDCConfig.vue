<template>
  <div class="oidc-container" v-loading="loading">
    <h2>OIDC 配置</h2>

    <!-- OIDC Config Section -->
    <el-card class="config-card">
      <template #header>
        <div class="card-header-row">
          <span>OIDC 提供商配置</span>
          <el-tag :type="providerTagType" size="small">{{ providerLabel }}</el-tag>
        </div>
      </template>

      <el-form
        ref="oidcFormRef"
        :model="oidcForm"
        :rules="oidcRules"
        label-position="top"
      >
        <el-form-item>
          <el-button type="warning" plain @click="showSwitchDialog = true">
            切换提供商
          </el-button>
        </el-form-item>

        <!-- Keycloak fields -->
        <template v-if="oidcForm.provider_type === 'keycloak'">
          <el-form-item label="Keycloak Base URL" prop="keycloak_base_url">
            <el-input v-model="oidcForm.keycloak_base_url" placeholder="https://keycloak.example.com" />
          </el-form-item>
          <el-form-item label="Keycloak Realm" prop="keycloak_realm">
            <el-input v-model="oidcForm.keycloak_realm" placeholder="my-realm" />
          </el-form-item>
        </template>

        <!-- Auth0 fields -->
        <template v-if="oidcForm.provider_type === 'auth0'">
          <el-form-item label="Auth0 Domain" prop="auth0_domain">
            <el-input v-model="oidcForm.auth0_domain" placeholder="your-tenant.auth0.com" />
          </el-form-item>
        </template>

        <!-- Generic OIDC fields -->
        <template v-if="oidcForm.provider_type === 'generic'">
          <el-form-item label="Issuer URL" prop="generic_issuer">
            <el-input v-model="oidcForm.generic_issuer" placeholder="https://oidc.example.com" />
          </el-form-item>
        </template>

        <!-- Common fields -->
        <el-form-item label="Client ID" prop="client_id">
          <el-input v-model="oidcForm.client_id" placeholder="your-client-id" />
        </el-form-item>
        <el-form-item label="Client Secret" prop="client_secret">
          <el-input v-model="oidcForm.client_secret" type="password" show-password placeholder="留空则不修改" />
          <div class="form-tip">已存储的 Client Secret 已加密，回显为 ••••••。输入新值将覆盖</div>
        </el-form-item>
        <el-form-item label="回调地址 (Redirect URI)" prop="redirect_uri">
          <el-input v-model="oidcForm.redirect_uri" placeholder="https://vpn.example.com/api/v1/auth/callback" />
        </el-form-item>
        <el-form-item label="前端地址 (Frontend URL)" prop="frontend_url">
          <el-input v-model="oidcForm.frontend_url" placeholder="https://vpn.example.com" />
        </el-form-item>

        <el-form-item>
          <el-button :loading="testing" @click="handleTest">测试连接</el-button>
          <el-button type="primary" :loading="saving" @click="handleSave">保存配置</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- Rate Limit Section -->
    <el-card class="config-card">
      <template #header>
        <span>速率限制配置</span>
      </template>

      <el-form
        ref="rateFormRef"
        :model="rateForm"
        :rules="rateRules"
        label-position="top"
      >
        <el-row :gutter="20">
          <el-col :span="12">
            <el-form-item label="登录 API 限制 (次/分钟)" prop="rate_limit_login">
              <el-input-number
                v-model="rateForm.rate_limit_login"
                :min="1"
                :max="1000"
                style="width: 100%"
              />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="下载 API 限制 (次/分钟)" prop="rate_limit_download">
              <el-input-number
                v-model="rateForm.rate_limit_download"
                :min="1"
                :max="1000"
                style="width: 100%"
              />
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item>
          <el-button type="primary" :loading="rateSaving" @click="handleRateSave">
            保存速率限制
          </el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- Provider Switch Dialog -->
    <OIDCSwitchDialog
      v-model:visible="showSwitchDialog"
      :current-provider="oidcForm.provider_type"
      @switch="handleProviderSwitch"
    />
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { useToast } from '@/composables/useToast'
const { success: toastSuccess, error: toastError } = useToast()
import { adminApi } from '@/services/api'
import OIDCSwitchDialog from '@/components/OIDCSwitchDialog.vue'

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

const providerTagType = computed(() => {
  const types = { keycloak: '', auth0: 'success', generic: 'warning' }
  return types[oidcForm.provider_type] || ''
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
// Lifecycle
// ==========================================================================
onMounted(async () => {
  loading.value = true
  await loadConfig()
  loading.value = false
})
</script>

<style scoped>
.oidc-container h2 {
  margin: 0 0 20px 0;
  font-size: 20px;
  font-weight: 600;
}

.config-card {
  margin-bottom: 20px;
}

.card-header-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.form-tip {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-top: 4px;
}
</style>
