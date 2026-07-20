<template>
  <div class="setup-container">
    <el-card class="setup-card">
      <template #header>
        <div class="setup-header">
          <h2>VPN 订阅管理系统 — 首次配置</h2>
          <p class="setup-desc">请配置 OIDC 认证提供商以完成系统初始化</p>
        </div>
      </template>

      <el-form
        ref="formRef"
        :model="form"
        :rules="rules"
        label-position="top"
        size="large"
      >
        <!-- Provider Type -->
        <el-form-item label="OIDC 提供商">
          <div class="provider-row">
            <el-tag :type="providerTagType" size="large">{{ providerLabel }}</el-tag>
            <el-button type="warning" plain @click="showSwitchDialog = true">
              切换提供商
            </el-button>
          </div>
        </el-form-item>

        <!-- Keycloak-specific fields -->
        <template v-if="form.provider_type === 'keycloak'">
          <el-form-item label="Keycloak Base URL" prop="keycloak_base_url">
            <el-input v-model="form.keycloak_base_url" placeholder="https://keycloak.example.com" />
          </el-form-item>
          <el-form-item label="Keycloak Realm" prop="keycloak_realm">
            <el-input v-model="form.keycloak_realm" placeholder="my-realm" />
          </el-form-item>
        </template>

        <!-- Auth0-specific fields -->
        <template v-if="form.provider_type === 'auth0'">
          <el-form-item label="Auth0 Domain" prop="auth0_domain">
            <el-input v-model="form.auth0_domain" placeholder="your-tenant.auth0.com" />
          </el-form-item>
        </template>

        <!-- Generic OIDC-specific fields -->
        <template v-if="form.provider_type === 'generic'">
          <el-form-item label="Issuer URL" prop="generic_issuer">
            <el-input v-model="form.generic_issuer" placeholder="https://oidc.example.com" />
          </el-form-item>
        </template>

        <!-- Common fields -->
        <el-form-item label="Client ID" prop="client_id">
          <el-input v-model="form.client_id" placeholder="your-client-id" />
        </el-form-item>
        <el-form-item label="Client Secret" prop="client_secret">
          <el-input v-model="form.client_secret" type="password" show-password placeholder="your-client-secret" />
        </el-form-item>
        <el-form-item label="回调地址 (Redirect URI)" prop="redirect_uri">
          <el-input v-model="form.redirect_uri" placeholder="https://vpn.example.com/api/v1/auth/callback" />
        </el-form-item>
        <el-form-item label="前端地址 (Frontend URL)" prop="frontend_url">
          <el-input v-model="form.frontend_url" placeholder="https://vpn.example.com" />
        </el-form-item>

        <!-- Actions -->
        <el-form-item>
          <div class="setup-actions">
            <el-button
              :loading="testing"
              :disabled="saving"
              @click="handleTest"
            >
              测试连接
            </el-button>
            <el-button
              type="primary"
              :loading="saving"
              :disabled="testing"
              @click="handleSubmit"
            >
              完成配置
            </el-button>
          </div>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- Provider switch dialog -->
    <OIDCSwitchDialog
      v-model:visible="showSwitchDialog"
      :current-provider="form.provider_type"
      @switch="handleProviderSwitch"
    />
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { publicApi, adminApi } from '@/services/api'
import OIDCSwitchDialog from '@/components/OIDCSwitchDialog.vue'

const router = useRouter()
const formRef = ref(null)

const form = reactive({
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

const showSwitchDialog = ref(false)
const testing = ref(false)
const saving = ref(false)

const providerLabel = computed(() => {
  const labels = { keycloak: 'Keycloak', auth0: 'Auth0', generic: '通用 OIDC' }
  return labels[form.provider_type] || 'Keycloak'
})

const providerTagType = computed(() => {
  const types = { keycloak: '', auth0: 'success', generic: 'warning' }
  return types[form.provider_type] || ''
})

const rules = computed(() => {
  const base = {
    client_id: [{ required: true, message: '请输入 Client ID', trigger: 'blur' }],
    client_secret: [{ required: true, message: '请输入 Client Secret', trigger: 'blur' }],
    redirect_uri: [{ required: true, message: '请输入回调地址', trigger: 'blur' }],
    frontend_url: [{ required: true, message: '请输入前端地址', trigger: 'blur' }]
  }
  if (form.provider_type === 'keycloak') {
    return {
      ...base,
      keycloak_base_url: [{ required: true, message: '请输入 Keycloak Base URL', trigger: 'blur' }],
      keycloak_realm: [{ required: true, message: '请输入 Keycloak Realm', trigger: 'blur' }]
    }
  } else if (form.provider_type === 'auth0') {
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

function buildPayload() {
  const payload = {
    provider_type: form.provider_type,
    client_id: form.client_id,
    client_secret: form.client_secret,
    redirect_uri: form.redirect_uri,
    frontend_url: form.frontend_url
  }
  if (form.provider_type === 'keycloak') {
    payload.keycloak_base_url = form.keycloak_base_url
    payload.keycloak_realm = form.keycloak_realm
  } else if (form.provider_type === 'auth0') {
    payload.auth0_domain = form.auth0_domain
  } else {
    payload.generic_issuer = form.generic_issuer
  }
  return payload
}

function handleProviderSwitch(newProvider) {
  form.provider_type = newProvider
  formRef.value?.clearValidate()
}

async function handleTest() {
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) return

  testing.value = true
  try {
    await adminApi.system.testOIDC(buildPayload())
    ElMessage.success('连接测试成功')
  } catch (e) {
    const msg = e.response?.data?.error || '连接测试失败'
    ElMessage.error('连接测试失败：' + msg)
  } finally {
    testing.value = false
  }
}

async function handleSubmit() {
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) return

  saving.value = true
  try {
    await adminApi.system.configure(buildPayload())
    ElMessage.success('配置完成，即将跳转到登录页')
    setTimeout(() => {
      router.push('/login')
    }, 1500)
  } catch (e) {
    const msg = e.response?.data?.error || '配置失败'
    ElMessage.error(msg)
  } finally {
    saving.value = false
  }
}

onMounted(async () => {
  // Auto-detect callback URL and frontend URL from current browser origin.
  // Only fills empty fields so that manual edits are preserved on re-render.
  const origin = window.location.origin
  if (!form.redirect_uri) {
    form.redirect_uri = origin + '/api/v1/auth/callback'
  }
  if (!form.frontend_url) {
    form.frontend_url = origin
  }

  try {
    const res = await publicApi.getSystemStatus()
    if (res.data.configured) {
      router.push('/login')
    }
  } catch (e) {
    // System status unavailable — stay on setup page
  }
})
</script>

<style scoped>
.setup-container {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 20px;
  background: var(--el-bg-color-page);
}

.setup-card {
  width: 100%;
  max-width: 560px;
}

.setup-header {
  text-align: center;
}

.setup-header h2 {
  margin: 0 0 8px;
  font-size: 22px;
  color: var(--el-text-color-primary);
}

.setup-desc {
  margin: 0;
  font-size: 14px;
  color: var(--el-text-color-secondary);
}

.provider-row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.setup-actions {
  display: flex;
  gap: 12px;
  justify-content: flex-end;
  width: 100%;
}
</style>
