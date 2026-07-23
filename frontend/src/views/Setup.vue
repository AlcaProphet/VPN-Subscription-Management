<template>
  <div class="max-w-lg mx-auto py-8 px-4">
    <div class="bg-white dark:bg-gray-800 rounded-lg shadow-md overflow-hidden">
      <!-- Header -->
      <div class="px-6 py-5 border-b border-gray-200 dark:border-gray-700">
        <h2 class="m-0 text-xl font-semibold text-gray-900 dark:text-white">VPN 订阅管理系统 — 首次配置</h2>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">请配置 OIDC 认证提供商以完成系统初始化</p>
      </div>

      <!-- Body -->
      <div class="p-6">
        <el-form
          ref="formRef"
          :model="form"
          :rules="rules"
          label-position="top"
          size="large"
        >
          <!-- Provider Type -->
          <el-form-item label="OIDC 提供商">
            <div class="flex items-center gap-3">
              <span class="rounded-full px-3 py-1 text-xs font-medium"
                :class="providerTagClass">
                {{ providerLabel }}
              </span>
              <button
                class="bg-orange-50 dark:bg-orange-900/20 border border-orange-300 dark:border-orange-700 text-orange-700 dark:text-orange-300 hover:bg-orange-100 dark:hover:bg-orange-900/30 rounded-md px-3 py-1 text-sm"
                @click="showSwitchDialog = true"
              >
                切换提供商
              </button>
            </div>
          </el-form-item>

          <!-- Keycloak-specific fields -->
          <template v-if="form.provider_type === 'keycloak'">
            <el-form-item label="Keycloak Base URL" prop="keycloak_base_url">
              <input v-model="form.keycloak_base_url" placeholder="https://keycloak.example.com"
                class="w-full rounded-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-3 py-2 text-base text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none"
                @blur="formRef.validateField('keycloak_base_url')" />
            </el-form-item>
            <el-form-item label="Keycloak Realm" prop="keycloak_realm">
              <input v-model="form.keycloak_realm" placeholder="my-realm"
                class="w-full rounded-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-3 py-2 text-base text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none"
                @blur="formRef.validateField('keycloak_realm')" />
            </el-form-item>
          </template>

          <!-- Auth0-specific fields -->
          <template v-if="form.provider_type === 'auth0'">
            <el-form-item label="Auth0 Domain" prop="auth0_domain">
              <input v-model="form.auth0_domain" placeholder="your-tenant.auth0.com"
                class="w-full rounded-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-3 py-2 text-base text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none"
                @blur="formRef.validateField('auth0_domain')" />
            </el-form-item>
          </template>

          <!-- Generic OIDC-specific fields -->
          <template v-if="form.provider_type === 'generic'">
            <el-form-item label="Issuer URL" prop="generic_issuer">
              <input v-model="form.generic_issuer" placeholder="https://oidc.example.com"
                class="w-full rounded-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-3 py-2 text-base text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none"
                @blur="formRef.validateField('generic_issuer')" />
            </el-form-item>
          </template>

          <!-- Common fields -->
          <el-form-item label="Client ID" prop="client_id">
            <input v-model="form.client_id" placeholder="your-client-id"
              class="w-full rounded-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-3 py-2 text-base text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none"
              @blur="formRef.validateField('client_id')" />
          </el-form-item>
          <el-form-item label="Client Secret" prop="client_secret">
            <input v-model="form.client_secret" type="password" placeholder="your-client-secret"
              class="w-full rounded-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-3 py-2 text-base text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none"
              @blur="formRef.validateField('client_secret')" />
          </el-form-item>
          <el-form-item label="回调地址 (Redirect URI)" prop="redirect_uri">
            <input v-model="form.redirect_uri" placeholder="https://vpn.example.com/api/v1/auth/callback"
              class="w-full rounded-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-3 py-2 text-base text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none"
              @blur="formRef.validateField('redirect_uri')" />
          </el-form-item>
          <el-form-item label="前端地址 (Frontend URL)" prop="frontend_url">
            <input v-model="form.frontend_url" placeholder="https://vpn.example.com"
              class="w-full rounded-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-3 py-2 text-base text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none"
              @blur="formRef.validateField('frontend_url')" />
          </el-form-item>

          <!-- Actions -->
          <el-form-item>
            <div class="flex gap-3">
              <button
                class="bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-600 rounded-md px-4 py-2 text-sm disabled:opacity-50"
                :disabled="saving"
                @click="handleTest"
              >
                <svg v-if="testing" class="animate-spin -ml-1 mr-2 h-4 w-4 inline-block text-gray-700" fill="none" viewBox="0 0 24 24">
                  <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
                  <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/>
                </svg>
                测试连接
              </button>
              <button
                class="bg-blue-600 hover:bg-blue-700 text-white rounded-md px-4 py-2 text-sm disabled:opacity-50"
                :disabled="testing"
                @click="handleSubmit"
              >
                <svg v-if="saving" class="animate-spin -ml-1 mr-2 h-4 w-4 inline-block text-white" fill="none" viewBox="0 0 24 24">
                  <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
                  <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/>
                </svg>
                完成配置
              </button>
            </div>
          </el-form-item>
        </el-form>
      </div>
    </div>

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
import { publicApi, adminApi } from '@/services/api'
import { useToast } from '@/composables/useToast'
import OIDCSwitchDialog from '@/components/OIDCSwitchDialog.vue'

const router = useRouter()
const formRef = ref(null)
const { success: toastSuccess, error: toastError } = useToast()

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
const setupConfirmed = ref(false) // true only after getSystemStatus confirms configured=false

const providerLabel = computed(() => {
  const labels = { keycloak: 'Keycloak', auth0: 'Auth0', generic: '通用 OIDC' }
  return labels[form.provider_type] || 'Keycloak'
})

const providerTagClass = computed(() => {
  const classes = {
    keycloak: 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300',
    auth0: 'bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-300',
    generic: 'bg-yellow-100 dark:bg-yellow-900/30 text-yellow-700 dark:text-yellow-300'
  }
  return classes[form.provider_type] || ''
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
  if (!valid) {
    toastError('请填写所有必填字段后再测试连接')
    return
  }

  // Re-verify system is still in setup mode before making admin API calls.
  // If the status check initially failed but the system is actually configured,
  // the admin API would return 401 → axios interceptor → hard redirect to /login,
  // clearing the form and dismissing any toast. This check prevents that.
  if (!setupConfirmed.value) {
    try {
      const res = await publicApi.getSystemStatus()
      if (res.data.configured) {
        router.push('/login')
        return
      }
      setupConfirmed.value = true
    } catch {
      toastError('无法验证系统状态，请检查网络后刷新页面重试')
      return
    }
  }

  testing.value = true
  try {
    await adminApi.system.testOIDC(buildPayload())
    toastSuccess('连接测试成功')
  } catch (e) {
    const msg = e.response?.data?.error || '连接测试失败'
    toastError('连接测试失败：' + msg)
  } finally {
    testing.value = false
  }
}

async function handleSubmit() {
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) {
    toastError('请填写所有必填字段后再提交')
    return
  }

  // Same re-verification as handleTest (see comment above)
  if (!setupConfirmed.value) {
    try {
      const res = await publicApi.getSystemStatus()
      if (res.data.configured) {
        router.push('/login')
        return
      }
      setupConfirmed.value = true
    } catch {
      toastError('无法验证系统状态，请检查网络后刷新页面重试')
      return
    }
  }

  saving.value = true
  try {
    await adminApi.system.configure(buildPayload())
    toastSuccess('配置完成，即将跳转到登录页')
    setTimeout(() => {
      router.push('/login')
    }, 1500)
  } catch (e) {
    const msg = e.response?.data?.error || '配置失败'
    toastError(msg)
  } finally {
    saving.value = false
  }
}

onMounted(async () => {
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
      return
    }
    setupConfirmed.value = true
  } catch (e) {
    // System status unavailable — stay on setup page.
    // setupConfirmed stays false; handleTest/handleSubmit will re-check.
  }
})
</script>
