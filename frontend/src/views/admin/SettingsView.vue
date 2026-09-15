<!-- SettingsView.vue：面板配置（UI §5.8，Design1 §3.4.8）——左侧锚点 + 右侧分区卡片；<768 锚点转顶部 Select -->
<script setup lang="ts">
import { computed, nextTick, onMounted, reactive, ref, watch } from 'vue'
import { onBeforeRouteLeave, useRouter } from 'vue-router'
import dayjs from 'dayjs'
import {
  Alert, Button, Card, Checkbox, Input, InputNumber, Modal, Radio, Space, Switch, Tag, Upload,
} from 'ant-design-vue'
import {
  getOidc, saveOidc, disableOidc, clearOidc, testOidc, getOidcRules, saveOidcRules, getLocalAuth, saveLocalAuth,
  getCaptcha, saveCaptcha, getSMTP, saveSMTP, testSMTP, getSite, saveSite, deleteSiteIcon,
  getRateLimit, saveRateLimit, getLogLevel, saveLogLevel, getAnnouncement, saveAnnouncement,
  getDebug, saveDebug, exportConfig, importConfig, clearAll, downloadBackup,
  getAdvancedSettings, saveAdvancedSettings, getAdminTask,
  type OidcSettings, type WhitelistConfig, type LocalAuthSettings,
  type CaptchaSettings, type SMTPSettings, type RateLimitSettings, type SiteInfo, type NoticeSettings,
  type AdvancedSettings,
} from '@/api/settings'
import { pollTask } from '@/api/request'
import { useSystemStore } from '@/stores/system'
import { useAuthStore } from '@/stores/auth'
import { ApiError } from '@/api/request'
import { importFileError } from '@/utils/fileLimits'
import ConfirmModal from '@/components/ConfirmModal.vue'
import PageHeader from '@/components/PageHeader.vue'
import { Notify } from '@/components/Notify'

const system = useSystemStore()
const auth = useAuthStore()
const router = useRouter()
const isProd = computed(() => system.status?.app_mode === 'prod')

// --- 六大设置分组：桌面左侧导航，手机顶部 Select。 ---
const settingGroups = [
  { key: 'identity', title: '身份与访问', description: 'OIDC、本地认证与验证码' },
  { key: 'notifications', title: '通知', description: '邮件发送设置与测试' },
  { key: 'content', title: '外观与内容', description: '站点信息、公告与页脚' },
  { key: 'runtime', title: '运行与安全', description: '运行模式、高级模式、限流与日志' },
  { key: 'data', title: '数据管理', description: '导入导出与备份' },
  { key: 'danger', title: '危险操作', description: '不可恢复的清空操作' },
] as const
type SettingGroupKey = (typeof settingGroups)[number]['key']
const activeGroup = ref<SettingGroupKey>('identity')
const groupOptions = settingGroups.map((item) => ({ value: item.key, label: item.title }))
const dirtyParts = ref(new Set<string>())
const settingsLoaded = ref(false)
const suppressDirty = ref(false)
const dangerExpanded = ref(false)
const dirtyCount = computed(() => dirtyParts.value.size)
function isGroupVisible(key: SettingGroupKey) { return activeGroup.value === key }
function markDirty(key: string) {
  if (!settingsLoaded.value || suppressDirty.value) return
  dirtyParts.value = new Set(dirtyParts.value).add(key)
}
function markSaved(key: string) {
  const next = new Set(dirtyParts.value)
  next.delete(key)
  dirtyParts.value = next
}
async function reloadClean(key: string, loader: () => Promise<void>) {
  suppressDirty.value = true
  try {
    await loader()
    await nextTick()
  } finally {
    suppressDirty.value = false
    markSaved(key)
  }
}

// --- OIDC 配置 ---
const oidc = reactive<OidcSettings>({ provider_type: 'generic', base_url: '', realm: '', client_id: '', client_secret: '', client_secret_configured: false, frontend_url: '', callback_url: '', params_state: undefined, params_damaged: false, params_warning: '' })
const oidcEnabled = ref(false) // 后端当前生效启用状态；与下拉草稿分开
const oidcSelection = ref('off') // 下拉选择：'off' 或具体 provider；选择 off 时仍保留 oidc.provider_type 供展示
const oidcSaving = ref(false)
const oidcTargetLoading = ref(false)
const oidcTest = ref<{ ok: boolean; message: string; warnings?: string[] } | null>(null)
const clearCallbackURL = ref(false)
const oidcCallbackPath = '/api/auth/oidc/callback'
// 未设置独立回调时的推导值；独立值优先，供管理员核对实际 redirect_uri。
const derivedCallbackURL = computed(() => {
  const base = (oidc.frontend_url || '').replace(/\/+$/, '')
  return base ? base + oidcCallbackPath : ''
})
const effectiveCallbackURL = computed(() => oidc.callback_url || derivedCallbackURL.value)
// 独立 callback_url 与前端地址 host/scheme 不一致时提示 state Cookie 边界，不阻断保存。
const callbackHostWarning = computed(() => {
  if (!oidc.callback_url) return ''
  try {
    const callback = new URL(oidc.callback_url)
    const frontend = oidc.frontend_url ? new URL(oidc.frontend_url) : null
    if (!frontend) return ''
    const hostMismatch = callback.host !== frontend.host
    const schemeDowngrade = frontend.protocol === 'https:' && callback.protocol !== 'https:'
    if (hostMismatch || schemeDowngrade) {
      const reason = hostMismatch
        ? `host 不一致（${callback.host} ≠ ${frontend.host}）`
        : '回调 scheme 非 HTTPS 而前端地址为 HTTPS'
      return `独立回调地址与前端地址 ${reason}：state Cookie 仅限发起登录的 host，HTTPS 前端下的 Secure Cookie 也不会随 HTTP 回调发送；实际登录必须从回调地址所在 host/协议发起，否则回调会因 state 不匹配失败。`
    }
  } catch {
    return ''
  }
  return ''
})
const providerOptions = computed(() => {
  const opts = [
    { label: '暂未启用（本地账号模式）', value: 'off' }, // R10-08：off 为前端显示值，映射 provider_type 空串（未配置）
    { label: 'Keycloak', value: 'keycloak' },
    { label: 'Auth0', value: 'auth0' },
    { label: 'Generic OIDC', value: 'generic' },
  ]
  // R31-06：Production 下 Mock 只读保留已有选择，不再作为可选启用项。
  if (!isProd.value || oidc.provider_type === 'mock') {
    opts.push({
      label: isProd.value ? 'Mock（生产不可用）' : 'Mock（仅 Dev）',
      value: 'mock',
      disabled: isProd.value,
    } as any)
  }
  return opts
})
// 当前提供商参数基线：用于判断“切离时是否有该提供商的未保存草稿”。
// frontend_url / callback_url 是站点级字段，不进入该基线、切换时保留。
let oidcBaseline = { provider_type: '', base_url: '', realm: '', client_id: '' }
let oidcLoadSeq = 0
function captureOidcBaseline() {
  oidcBaseline = {
    provider_type: oidc.provider_type,
    base_url: oidc.base_url,
    realm: oidc.realm,
    client_id: oidc.client_id,
  }
}
function hasProviderDraft() {
  return oidc.client_secret !== ''
    || oidc.provider_type !== oidcBaseline.provider_type
    || oidc.base_url !== oidcBaseline.base_url
    || oidc.realm !== oidcBaseline.realm
    || oidc.client_id !== oidcBaseline.client_id
}
async function loadOidc() {
  try {
    const res = await getOidc()
    oidcEnabled.value = res.enabled !== false // 兼容未返回 enabled 的旧响应/缓存，仅显式 false 视为停用
    // 停用时仍保留 provider/参数；下拉回到 off，但页面用 oidc.provider_type 展示保留的提供商。
    oidcSelection.value = oidcEnabled.value ? (res.provider_type || 'off') : 'off'
    Object.assign(oidc, res)
    // Secret 输入值始终为空；只有 client_secret_configured 表示当前提供商已存可用 Secret。
    oidc.client_secret = ''
    oidc.client_secret_configured = res.client_secret_configured === true
    // 当前生效提供商 GET 不返回目标损坏字段；显式归零，避免残留上一次目标读取的提示。
    oidc.params_state = res.params_state
    oidc.params_damaged = res.params_damaged === true
    oidc.params_warning = res.params_warning || ''
    clearCallbackURL.value = false
    captureOidcBaseline()
  } catch (err) {
    Notify.error((err as Error).message)
  }
}
// OIDC 字段随提供商动态显隐（R10-02）：Auth0 用 Domain 标识；Realm 仅 Keycloak 适用；Mock 无参数
const isMockProvider = computed(() => oidc.provider_type === 'mock')
// R31-06：Production 下历史 mock 配置只读保留，禁止保存/测试并在界面提示切换真实提供商。
const mockBlocked = computed(() => isProd.value && isMockProvider.value)
const urlLabel = computed(() => (oidc.provider_type === 'auth0' ? 'Domain' : 'Base URL'))
const urlPlaceholder = computed(() => (oidc.provider_type === 'auth0' ? 'your-tenant.auth0.com' : 'https://idp.example.com'))
const showRealm = computed(() => oidc.provider_type === 'keycloak')
// 兼容未返回 params_state 的旧响应：按损坏标记/已配置状态降级推断，避免旧测试与缓存响应失效。
const oidcState = computed(() => oidc.params_state
  || (oidc.params_damaged ? 'secret_damaged' : (oidc.client_secret_configured ? 'usable' : 'not_configured')))
const oidcKeyFault = computed(() => oidcState.value === 'signing_key_fault')
const oidcStateAlert = computed(() => oidc.params_warning || (() => {
  switch (oidcState.value) {
    case 'not_configured': return '当前提供商尚未保存 OIDC 参数，请填写必要参数并输入新的 Client Secret 后保存'
    case 'missing_secret': return '已存 OIDC 参数尚未配置可用 Client Secret，请填写新的 Client Secret 后保存'
    case 'json_damaged': return '已存 OIDC 参数 JSON 无法解析，请重新填写必要的 Base URL/Realm/Client ID 并输入新的 Client Secret 后保存'
    case 'secret_damaged': return '已存 Client Secret 损坏或为脱敏占位符，请输入新的 Client Secret 后保存'
    case 'signing_key_fault': return '系统签名密钥缺失或不可读取，当前无法校验或保存 OIDC 凭据；请通过备份恢复或应急初始化处理，不要在此重填 Secret'
    default: return ''
  }
})())
const secretHint = computed(() => {
  switch (oidcState.value) {
    case 'usable':
      return '已保存 Secret 不会回显；留空仅在目标提供商的 Base URL/Realm/Client ID 与已存配置完全一致且已有可用 Secret 时保持原值，否则保存会被拒绝，需输入新 Secret。'
    case 'missing_secret':
      return '该提供商尚未配置可用 Client Secret，必须输入新的 Client Secret 后保存。'
    case 'json_damaged':
      return '已存参数 JSON 损坏，请重新填写必要的 Base URL/Realm/Client ID 并输入新的 Client Secret 后保存。'
    case 'secret_damaged':
      return '已存 Client Secret 损坏或为脱敏占位符，必须输入新的 Client Secret 后保存。'
    case 'signing_key_fault':
      return '系统签名密钥不可用，OIDC 配置无法校验或保存；请先按备份恢复或应急初始化处理，不要通过重填 Secret 尝试恢复。'
    default:
      return '尚未保存 OIDC 参数，请填写必要参数并输入新的 Client Secret 后保存。'
  }
})
const secretPlaceholder = computed(() => {
  if (oidcKeyFault.value) return '系统签名密钥不可用，已阻断重填'
  return oidcState.value === 'usable'
    ? '留空保持原值；输入新 Secret 后保存替换'
    : '必须输入新的 Client Secret'
})
// R31-07：选择“暂未启用”只形成页面草稿；保存停用走独立入口，不清空 oidc.provider_type，
// 以便停用态仍展示保留的提供商。
// 目标提供商读取成功后整体替换提供商字段，避免异步返回期间出现“目标 provider + 源字段”的中间态。
function applyOidcTarget(providerType: string, res: OidcSettings) {
  oidc.provider_type = providerType
  oidcSelection.value = providerType
  oidc.base_url = res.base_url || ''
  oidc.realm = res.realm || ''
  oidc.client_id = res.client_id || ''
  oidc.client_secret = ''
  oidc.client_secret_configured = res.client_secret_configured === true
  oidc.params_state = res.params_state
  oidc.params_damaged = res.params_damaged === true
  oidc.params_warning = res.params_warning || ''
  clearCallbackURL.value = false
  oidcTest.value = null
  captureOidcBaseline()
}
async function loadOidcTarget(providerType: string) {
  const seq = ++oidcLoadSeq
  oidcTargetLoading.value = true
  try {
    const res = await getOidc(providerType)
    if (seq !== oidcLoadSeq) return // 过期响应丢弃
    applyOidcTarget(providerType, res)
  } catch (err) {
    if (seq === oidcLoadSeq) Notify.error((err as Error).message)
  } finally {
    if (seq === oidcLoadSeq) oidcTargetLoading.value = false
  }
}
// onProviderChange：'off'（暂未启用）只改页面草稿；启用态之间切换需确认；停用态选择 provider 仅加载目标字段，保存成功后才重新启用。
function onProviderChange(v: any) {
  const target = v === 'off' ? '' : v
  if (target === oidcSelection.value) return
  if (oidcSelection.value === 'off') {
    if (target !== '') void loadOidcTarget(target)
    return
  }
  const draftWarning = hasProviderDraft()
    ? '当前提供商的未保存参数草稿将被丢弃（前端地址/回调地址保留）。'
    : ''
  Modal.confirm({
    title: target === '' ? '保存停用草稿' : '切换提供商类型',
    content: target === ''
      ? `选择“暂未启用”只形成页面草稿，不会立即停用；点击“保存停用”后 OIDC 登录与绑定才会关闭。提供商参数与已有绑定会保留，现有会话继续有效。${draftWarning}`
      : `已绑定旧提供商 OIDC 身份的用户在新提供商下登录将失效，建议先为相关管理员设置本地密码。切换后将读取目标提供商自己的 Base URL/Realm/Client ID；目标未配置则清空。Client Secret 输入框始终清空，留空仅在目标字段与已存配置完全一致且已有可用 Secret 时可复用，否则必须输入新 Secret。${draftWarning}`,
    okText: target === '' ? '保留草稿' : '继续切换',
    cancelText: '取消',
    onOk: async () => {
      if (target === '') {
        oidcSelection.value = 'off'
        markDirty('oidc')
        return
      }
      await loadOidcTarget(target)
    },
  })
}
// markClearCallback：显式清除独立回调地址；空 callback_url 本身仍表示不修改，必须由该标记驱动。
function markClearCallback() {
  Modal.confirm({
    title: '恢复跟随前端地址',
    content: '保存后将清除已保存的独立回调地址，改用“前端地址 + /api/auth/oidc/callback”推导。进行中的 OIDC 授权仍使用原地址。',
    okText: '标记清除',
    cancelText: '取消',
    onOk: () => {
      oidc.callback_url = ''
      clearCallbackURL.value = true
      markDirty('oidc')
    },
  })
}

// 手动编辑独立回调地址即取消待清除标记。
function onCallbackInput() {
  clearCallbackURL.value = false
}

async function doSaveOidc() {
  if (oidcSelection.value === 'off') {
    oidcSaving.value = true
    try {
      await disableOidc()
      Notify.success('OIDC 已停用，提供商参数与已有绑定已保留；现有会话继续有效')
      await reloadClean('oidc', loadOidc)
      await system.fetchStatus(true)
    } catch (err) {
      Notify.error((err as Error).message)
    } finally {
      oidcSaving.value = false
    }
    return
  }
  if (mockBlocked.value) {
    Notify.error('生产模式不支持模拟 OIDC，请切换到真实提供商')
    return
  }
  if (oidcTargetLoading.value || oidcKeyFault.value) return
  oidcSaving.value = true
  try {
    await saveOidc({ ...oidc, clear_callback_url: clearCallbackURL.value })
    Notify.success('OIDC 配置已保存，地址即时生效')
    clearCallbackURL.value = false
    await reloadClean('oidc', loadOidc)
    await system.fetchStatus(true)
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    oidcSaving.value = false
  }
}
async function doTestOidc() {
  if (oidcSelection.value === 'off') return
  if (mockBlocked.value) {
    Notify.error('生产模式不支持模拟 OIDC，请切换到真实提供商')
    return
  }
  if (oidcTargetLoading.value || oidcKeyFault.value) return
  oidcTest.value = null
  try {
    oidcTest.value = await testOidc({ ...oidc })
  } catch (err) {
    Notify.error((err as Error).message)
  }
}
const clearOidcOpen = ref(false)
async function confirmClearOidc() {
  try {
    await clearOidc()
    Notify.success('OIDC 配置已清空')
    clearOidcOpen.value = false
    await reloadClean('oidc', loadOidc)
    await system.fetchStatus(true)
  } catch (err) {
    Notify.error((err as Error).message) // 本地登录已关时提示死锁
  }
}

// --- OIDC 启用规则 ---
const oidcRules = reactive({ approval_on: false, whitelist: { role_claim_path: 'realm_access.roles', role_values: [] as string[], group_claim_path: 'groups', group_values: [] as string[] } as WhitelistConfig })
const rulesSaving = ref(false)
async function loadOidcRules() {
  try {
    const res = await getOidcRules()
    oidcRules.approval_on = res.approval_on
    // 归一化零值（R10-03）：未配置时后端返回 claim_path=""、values=null；
    // Object.assign 会覆盖预设默认值并让 AntD Select 把空字符串渲染为空 tag（视觉空格）
    oidcRules.whitelist.role_claim_path = res.whitelist.role_claim_path || 'realm_access.roles'
    oidcRules.whitelist.group_claim_path = res.whitelist.group_claim_path || 'groups'
    oidcRules.whitelist.role_values = res.whitelist.role_values ?? []
    oidcRules.whitelist.group_values = res.whitelist.group_values ?? []
  } catch (err) {
    Notify.error((err as Error).message)
  }
}
async function doSaveOidcRules() {
  rulesSaving.value = true
  try {
    const res = await saveOidcRules({ approval_on: oidcRules.approval_on, whitelist: oidcRules.whitelist })
    Notify.success('OIDC 启用规则已保存')
    markSaved('oidc-rules')
    if (res.warning) Modal.warning({ title: '注意', content: res.warning })
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    rulesSaving.value = false
  }
}

// --- 本地认证 ---
const localAuth = reactive<LocalAuthSettings>({ allow_local_login: true, allow_selfreg: false, selfreg_approval: false })
const localSaving = ref(false)
async function loadLocalAuth() {
  try {
    Object.assign(localAuth, await getLocalAuth())
  } catch (err) {
    Notify.error((err as Error).message)
  }
}
async function doSaveLocalAuth() {
  localSaving.value = true
  try {
    await saveLocalAuth({ ...localAuth })
    Notify.success('本地认证配置已保存')
    markSaved('local-auth')
  } catch (err) {
    Notify.error((err as Error).message) // 死锁防护提示
  } finally {
    localSaving.value = false
  }
}

// --- 验证码 ---
const captcha = reactive<CaptchaSettings>({ provider: 'off', site_key: '', secret_key: '', pages: [] })
const captchaSaving = ref(false)
const captchaPages = [
  { label: '注册页', value: 'register' },
  { label: '登录页', value: 'login' },
  { label: '找回密码', value: 'forgot' },
]
async function loadCaptcha() {
  try {
    const res = await getCaptcha()
    // 归一化零值（R10-04）：未配置时后端返回 provider=""，Object.assign 覆盖预设 'off' 导致 Radio 无勾选
    captcha.provider = res.provider || 'off'
    captcha.site_key = res.site_key
    captcha.secret_key = res.secret_key
    captcha.pages = res.pages ?? []
  } catch (err) {
    Notify.error((err as Error).message)
  }
}
async function doSaveCaptcha() {
  captchaSaving.value = true
  try {
    await saveCaptcha({ ...captcha })
    Notify.success('验证码配置已保存')
    await reloadClean('captcha', loadCaptcha)
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    captchaSaving.value = false
  }
}

// --- SMTP ---
const smtp = reactive<SMTPSettings>({ host: '', port: '587', user: '', password: '', password_configured: false, from: '', security: 'starttls', auth_required: true, configured: false, scopes: [] })
const smtpSaving = ref(false)
const smtpTesting = ref(false)
const smtpTestTo = ref('')
const smtpSecurityOptions = [
  { label: 'STARTTLS（必须升级）', value: 'starttls' },
  { label: '连接即 TLS', value: 'implicit_tls' },
  { label: '无加密（仅本地无认证中继）', value: 'plain' },
]
const scopeOptions = [
  { label: '密码重置邮件', value: 'password_reset' },
  { label: '审批结果通知', value: 'approval_notify' },
  { label: '欢迎邮件', value: 'welcome' },
]
async function loadSMTP() {
  try {
    Object.assign(smtp, await getSMTP())
  } catch (err) {
    Notify.error((err as Error).message)
  }
}
async function doSaveSMTP() {
  smtpSaving.value = true
  try {
    await saveSMTP({ ...smtp })
    Notify.success('SMTP 配置已保存')
    await reloadClean('smtp', loadSMTP)
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    smtpSaving.value = false
  }
}
watch(() => smtp.security, (security) => {
  if (security === 'plain') smtp.auth_required = false
})
async function doTestSMTP() {
  smtpTesting.value = true
  try {
    const result = await testSMTP(smtpTestTo.value.trim())
    Notify.success(`测试邮件已发送至 ${result.to}，请查收`)
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    smtpTesting.value = false
  }
}

// --- 站点信息 ---
const site = reactive<SiteInfo>({ site_name: '', icon_url: '' })
const siteSaving = ref(false)
const siteFile = ref<File | null>(null)
async function loadSite() {
  try {
    Object.assign(site, await getSite())
  } catch (err) {
    Notify.error((err as Error).message)
  }
}
function onSiteFile(file: File) {
  siteFile.value = file
  return false
}
async function doSaveSite() {
  if (!site.site_name.trim()) {
    Notify.error('站点名称不能为空')
    return
  }
  siteSaving.value = true
  try {
    const fd = new FormData()
    fd.append('site_name', site.site_name.trim())
    if (siteFile.value) fd.append('icon', siteFile.value)
    Object.assign(site, await saveSite(fd))
    siteFile.value = null
    Notify.success('站点信息已保存')
    markSaved('site')
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    siteSaving.value = false
  }
}
const iconDeleteOpen = ref(false)
async function confirmIconDelete() {
  try {
    await deleteSiteIcon()
    site.icon_url = ''
    Notify.success('已恢复默认 ICON')
    markSaved('site')
    iconDeleteOpen.value = false
  } catch (err) {
    Notify.error((err as Error).message)
  }
}

// --- 速率限制 ---
const rate = reactive<RateLimitSettings>({
  login: 10, register: 5, forgot: 5, download: 20, reset_validate: 10,
  http_read_header_timeout_sec: 5, http_read_timeout_sec: 60,
  http_write_timeout_sec: 300, http_idle_timeout_sec: 120, http_max_body_mb: 4,
})
const trustProxy = ref('auto')
const trustProxyCidrs = ref('')
const rateSaving = ref(false)
async function loadRateLimit() {
  try {
    const res = await getRateLimit()
    Object.assign(rate, res.settings)
    trustProxy.value = res.trust_proxy
    trustProxyCidrs.value = res.trust_proxy_cidrs ?? ''
  } catch (err) {
    Notify.error((err as Error).message)
  }
}
async function doSaveRateLimit() {
  rateSaving.value = true
  try {
    await saveRateLimit({ ...rate })
    Notify.success('速率限制已保存；连接超时字段需重启容器后生效')
    markSaved('ratelimit')
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    rateSaving.value = false
  }
}

// --- 日志级别 ---
const logLevel = ref('info')
async function loadLogLevel() {
  try {
    logLevel.value = (await getLogLevel()).level
  } catch (err) {
    Notify.error((err as Error).message)
  }
}
async function doSaveLogLevel() {
  try {
    await saveLogLevel(logLevel.value)
    Notify.success('日志级别已切换并立即生效')
    markSaved('log-level')
  } catch (err) {
    Notify.error((err as Error).message)
  }
}

// --- 公告与页脚（R10-07：首页公告 / 登录页公告 / 登录页页脚三份独立配置）---
const announcement = reactive<NoticeSettings>({ home_announcement: '', login_announcement: '', login_footer: '' })
const announcementSaving = ref(false)
async function loadAnnouncement() {
  try {
    Object.assign(announcement, await getAnnouncement())
  } catch (err) {
    Notify.error((err as Error).message)
  }
}
async function doSaveAnnouncement() {
  announcementSaving.value = true
  try {
    await saveAnnouncement({ ...announcement })
    Notify.success('公告与页脚已保存')
    markSaved('announcement')
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    announcementSaving.value = false
  }
}

// --- 调试模式 ---
const debugOn = ref(false)
const debugSaving = ref(false)
async function loadDebug() {
  try {
    debugOn.value = (await getDebug()).on
  } catch (err) {
    Notify.error((err as Error).message)
  }
}
async function doSaveDebug() {
  debugSaving.value = true
  try {
    await saveDebug(debugOn.value)
    Notify.success('调试模式已更新')
    markSaved('debug')
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    debugSaving.value = false
  }
}

// --- 高级模式（Build7 Step2/Step4） ---
const advanced = ref<AdvancedSettings>({ advanced_mode: false, collect_interval_minutes: 10, traffic_card_enabled: true })
const advancedSaving = ref(false)
const advancedConfirmOpen = ref(false)
async function loadAdvanced() {
  try {
    Object.assign(advanced.value, await getAdvancedSettings())
  } catch (err) {
    Notify.error((err as Error).message)
  }
}
async function doSaveAdvanced() {
  advancedSaving.value = true
  try {
    const data = { ...advanced.value }
    if (!data.advanced_mode) {
      advancedConfirmOpen.value = true
      return
    }
    await saveAdvancedSettings(data)
    Notify.success('高级模式已开启')
    markSaved('advanced')
    await system.fetchStatus(true)
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    advancedSaving.value = false
  }
}
async function confirmDisableAdvanced() {
  advancedSaving.value = true
  try {
    const res = await saveAdvancedSettings({ ...advanced.value, confirm_word: 'DISABLE' })
    if (res.task_id) {
      await pollTask({
        submit: () => Promise.resolve(),
        query: () => getAdminTask(res.task_id!),
        isDone: (t) => t.status === 'succeeded' || t.status === 'failed',
      }).run()
    }
    advancedConfirmOpen.value = false
    Notify.success('高级模式已关闭，数据已清空')
    markSaved('advanced')
    await system.fetchStatus(true)
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    advancedSaving.value = false
  }
}


// --- 配置导入/导出（仅 Production；Dev 显示说明文案） ---
const exportPwd = ref('')
const exporting = ref(false)
const importOpen = ref(false)
const disableImportOpen = ref(false)
const importForm = reactive({ file: null as File | null, password: '' })
const importing = ref(false)
const importProtectError = ref('')

// downloadBlob 触发浏览器下载
function downloadBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  a.click()
  URL.revokeObjectURL(url)
}

// asBlobError 将 JSON 错误 Blob 转为 ApiError（responseType=blob 时错误体被包装为 Blob）
async function asBlobError(data: Blob): Promise<never> {
  const text = await data.text()
  try {
    const body = JSON.parse(text)
    throw new ApiError(body.code ?? 500, body.message ?? '操作失败')
  } catch (e) {
    if (e instanceof ApiError) throw e
    throw new ApiError(500, '操作失败')
  }
}

async function doExport() {
  if (exportPwd.value.length < 8) {
    Notify.error('导出密码至少 8 字符')
    return
  }
  exporting.value = true
  try {
    const blob = await exportConfig(exportPwd.value)
    if (blob instanceof Blob && blob.type.includes('json')) await asBlobError(blob) // 错误响应
    downloadBlob(blob, `vpn-sub-config-${dayjs().format('YYYYMMDD')}.enc`)
    Notify.success('配置已导出')
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    exporting.value = false
  }
}

function onImportFile(file: File) {
  const err = importFileError(file)
  if (err) {
    importForm.file = null
    Notify.error(err)
    return false
  }
  importForm.file = file
  return false
}

// 第一步确认 IMPORT 后先按普通导入提交；仅当后端要求 DISABLE（无实例/账号且高级关闭）时再弹第二步。
function doImport() {
  if (!importForm.file || !importForm.password) {
    Notify.error('请选择文件并输入导出密码')
    return
  }
  importOpen.value = false
  void submitImport(false)
}

async function submitImport(withDisable = true) {
  if (!importForm.file || !importForm.password) {
    Notify.error('请选择文件并输入导出密码')
    return
  }
  importing.value = true
  try {
    const fd = new FormData()
    fd.append('file', importForm.file)
    fd.append('password', importForm.password)
    // 确认词由 ConfirmModal 按钮禁用保证输入正确（okDisabled），后端二次校验兜底；与 SetupView 硬编码模式统一（R08-02）
    fd.append('confirm_word', 'IMPORT')
    fd.append('disable_confirm_word', withDisable ? 'DISABLE' : '')
    const res = await importConfig(fd)
    disableImportOpen.value = false
    const importAftercare = '地址与 OIDC 配置即时生效；签名密钥替换后全部会话失效，请重新登录；如导入文件包含不同的日志级别/HTTP 超时等启动参数，重启容器后完全生效。'
    let importHintText = '配置已整体覆盖（导出文件中不存在的配置键已清除）。' + importAftercare
    if (res.task_id) {
      const task = await pollTask({
        submit: () => Promise.resolve(),
        query: () => getAdminTask(res.task_id!),
        isDone: (t) => t.status === 'succeeded' || t.status === 'failed',
      }).run()
      if (task.status === 'failed') {
        throw new Error(task.error || '导入任务失败')
      }
      const result = (task.result ?? {}) as { hints?: string[] }
      const hints = result.hints ?? []
      importHintText = hints.length > 0
        ? '配置已导入并完成异步处理，完成提示：\n' + hints.join('\n') + '\n' + importAftercare
        : '配置已导入并完成异步处理，请刷新页面后确认高级模式状态。' + importAftercare
    }
    Modal.warning({
      title: '导入完成',
      content: importHintText,
      okText: '退出登录',
      onOk: async () => {
        await auth.logoutAction()
        void router.push('/login')
      },
    })
  } catch (err) {
    const msg = (err as Error).message
    // 首次普通提交被要求 DISABLE 时，静默进入第二步确认弹窗。
    if (!withDisable && msg.includes('DISABLE')) {
      disableImportOpen.value = true
      return
    }
    if (msg.includes('signing_key') || msg.includes('签名密钥') || msg.includes('配置导入仅适用全新部署')) {
      importProtectError.value = msg
    }
    Notify.error(msg) // 确认词/密码错误提示
  } finally {
    importing.value = false
  }
}

// --- 备份下载（ConfirmModal 二次确认 → tar.gz） ---
const backupOpen = ref(false)
const backingUp = ref(false)
async function doBackup() {
  backingUp.value = true
  try {
    const blob = await downloadBackup()
    if (blob instanceof Blob && blob.type.includes('json')) await asBlobError(blob)
    downloadBlob(blob, `vpn-sub-backup-${dayjs().format('YYYYMMDD-HHmmss')}.tar.gz`)
    Notify.success('备份已下载')
    backupOpen.value = false
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    backingUp.value = false
  }
}

// --- 危险操作区：一键清空所有数据（RESET 确认词 + 二次确认） ---
const clearOpen = ref(false)
const clearing = ref(false)
function openClear() {
  clearOpen.value = true
}
async function doClearAll() {
  clearing.value = true
  try {
    // 确认词由 ConfirmModal 按钮禁用保证输入正确（okDisabled），后端二次校验兜底；与 SetupView 硬编码模式统一（R08-01）
    await clearAll('RESET')
    Notify.success('系统已重置')
    clearOpen.value = false
    // 签名密钥已轮换，旧会话全部失效：立即清除本地凭据（R07-08，防残留失效 token 触发首页 me() 401 全局提示）
    await auth.logoutAction()
    await system.fetchStatus(true) // configured=false → 守卫自动跳 /setup
    void router.push('/setup')
  } catch (err) {
    Notify.error((err as Error).message) // 确认词不正确
  } finally {
    clearing.value = false
  }
}

// 初始加载完成前不记录脏状态；之后每个独立保存分区都准确参与离开确认。
watch(oidc, () => markDirty('oidc'), { deep: true })
watch(oidcSelection, () => markDirty('oidc'))
watch(oidcRules, () => markDirty('oidc-rules'), { deep: true })
watch(localAuth, () => markDirty('local-auth'), { deep: true })
watch(captcha, () => markDirty('captcha'), { deep: true })
watch(smtp, () => markDirty('smtp'), { deep: true })
watch(site, () => markDirty('site'), { deep: true })
watch(rate, () => markDirty('ratelimit'), { deep: true })
watch(logLevel, () => markDirty('log-level'))
watch(announcement, () => markDirty('announcement'), { deep: true })
watch(debugOn, () => markDirty('debug'))
watch(advanced, () => markDirty('advanced'), { deep: true })

onBeforeRouteLeave(() => {
  if (dirtyCount.value === 0) return true
  return window.confirm(`当前有 ${dirtyCount.value} 个分区尚未保存，确定离开吗？`)
})

onMounted(async () => {
  await Promise.all([loadOidc(), loadOidcRules(), loadLocalAuth(), loadCaptcha(), loadSMTP(),
    loadSite(), loadRateLimit(), loadLogLevel(), loadAnnouncement(), loadDebug(), loadAdvanced()])
  settingsLoaded.value = true
  void system.fetchStatus()
})
</script>

<template>
  <div>
    <PageHeader title="面板配置" subtitle="按场景分组管理配置；每个分区独立保存。">
      <template #actions>
        <Tag v-if="dirtyCount" color="orange">{{ dirtyCount }} 个分区有未保存更改</Tag>
      </template>
    </PageHeader>

    <!-- 手机只保留一个分组选择器，避免长锚点列表挤占可视区域。 -->
    <div class="mb-4 md:hidden">
      <AppSelect v-model:value="activeGroup" :options="groupOptions" class="w-full" aria-label="选择设置分组" />
    </div>

    <div class="flex gap-6">
      <!-- 桌面分组导航：组内卡片在右侧连续展示，定位比旧版 15 项锚点更快。 -->
      <aside class="hidden md:block w-44 shrink-0">
        <div class="sticky top-20 space-y-1">
          <button v-for="group in settingGroups" :key="group.key" type="button" class="settings-group-nav"
                  :class="{ active: activeGroup === group.key }" @click="activeGroup = group.key">
            <span class="block font-medium">{{ group.title }}</span>
            <span class="block mt-0.5 text-xs opacity-70">{{ group.description }}</span>
          </button>
        </div>
      </aside>

      <div class="flex-1 space-y-4 min-w-0">
        <!-- OIDC 配置 -->
        <Card v-show="isGroupVisible('identity')" id="oidc" title="OIDC 配置" size="small">
          <div class="space-y-3 max-w-xl">
            <div class="flex items-center gap-3">
              <span class="w-24 text-sm">提供商类型</span>
              <AppSelect class="flex-1" :value="oidcSelection" :options="providerOptions" :disabled="oidcTargetLoading" @change="onProviderChange" />
            </div>
            <!-- 暂未启用（off）只折叠参数区；未保存草稿与已停用状态在下方单独展示。 -->
            <template v-if="oidcSelection !== 'off'">
              <Alert v-if="!oidcEnabled" type="info" show-icon
                     message="保存后将重新启用 OIDC。请核对目标提供商参数与 Secret 状态后保存。" />
              <template v-if="!isMockProvider">
                <div class="flex items-center gap-3">
                  <span class="w-24 text-sm">{{ urlLabel }}</span>
                  <Input v-model:value="oidc.base_url" :placeholder="urlPlaceholder" :disabled="oidcKeyFault" />
                </div>
                <div v-if="showRealm" class="flex items-center gap-3">
                  <span class="w-24 text-sm">Realm</span>
                  <Input v-model:value="oidc.realm" placeholder="Keycloak 专用，如 master" :disabled="oidcKeyFault" />
                </div>
                <div class="flex items-center gap-3">
                  <span class="w-24 text-sm">Client ID</span>
                  <Input v-model:value="oidc.client_id" placeholder="客户端标识" :disabled="oidcKeyFault" />
                </div>
                <div class="flex items-center gap-3">
                  <span class="w-24 text-sm">Client Secret <Tag v-if="oidc.client_secret_configured" color="success">已配置</Tag><Tag v-else>未配置</Tag></span>
                  <Input.Password v-model:value="oidc.client_secret" autocomplete="new-password" :placeholder="secretPlaceholder" :disabled="oidcKeyFault" />
                </div>
                <Alert v-if="oidcState !== 'usable'" :type="oidcKeyFault ? 'error' : 'warning'" show-icon
                       :message="oidcStateAlert || '已存 OIDC 配置需要处理，请按提示重填后保存'" />
                  <p class="text-xs text-text-secondary">{{ secretHint }}</p>
              </template>
              <Alert v-else :type="mockBlocked ? 'error' : 'info'" show-icon
                     :message="mockBlocked ? '生产模式不支持模拟 OIDC：历史参数已保留但登录入口已停用，请切换到真实提供商' : '模拟 OIDC：无需参数，登录页将显示 Dev 模拟登录表单'" />
              <Alert type="info" show-icon message="接入提示" description="OIDC 回调要求公网可达的 HTTPS 域名，局域网直连模式可能无法完成回调" />
              <div class="flex items-center gap-3">
                <span class="w-24 text-sm">前端地址</span>
                <Input v-model:value="oidc.frontend_url" placeholder="https://app.example.com" />
              </div>
              <div class="flex items-start gap-3">
                <span class="w-24 text-sm pt-1">回调地址</span>
                <div class="flex-1 min-w-0 space-y-1">
                  <div class="flex items-center gap-2">
                    <Input v-model:value="oidc.callback_url" :placeholder="derivedCallbackURL || 'https://app.example.com/api/auth/oidc/callback'" @input="onCallbackInput" />
                    <Button size="small" :disabled="!oidc.callback_url || oidcKeyFault" @click="markClearCallback">恢复推导</Button>
                  </div>
                  <p class="text-xs text-text-tertiary">
                    当前生效回调：{{ effectiveCallbackURL || '未配置（请填写前端地址或独立回调地址）' }}<span v-if="clearCallbackURL">；已标记清除独立回调，保存后回退推导</span>
                  </p>
                </div>
              </div>
              <Alert type="info" show-icon message="前端地址与回调地址保存后即时生效" />
              <Alert v-if="callbackHostWarning" type="warning" show-icon :message="callbackHostWarning" />
              <Alert v-if="oidcTest" :type="oidcTest.ok ? 'success' : 'error'" show-icon :message="oidcTest.message" :description="oidcTest.warnings?.length ? oidcTest.warnings.join('；') : undefined" />
              <Space>
                <Button type="primary" :loading="oidcSaving" :disabled="oidcTargetLoading || oidcKeyFault || mockBlocked" @click="doSaveOidc">保存</Button>
                <Button :disabled="oidcTargetLoading || oidcKeyFault || mockBlocked" @click="doTestOidc">测试连接</Button>
                <Button danger :disabled="oidcTargetLoading" @click="clearOidcOpen = true">清空 OIDC 配置</Button>
              </Space>
            </template>
            <template v-else>
              <Alert v-if="oidcEnabled" type="warning" show-icon
                     message="尚未保存停用：点击“保存停用”后，新的 OIDC 登录与绑定将关闭；提供商参数与已有绑定保留，现有会话继续有效。" />
              <Alert v-else type="info" show-icon
                     :message="oidc.provider_type
                       ? `OIDC 已停用。已保留提供商：${oidc.provider_type}；选择提供商并保存后重新启用。`
                       : '暂未启用 OIDC：保持本地账号模式，或选择上方提供商开始配置。'" />
              <div v-if="oidcEnabled" class="flex items-center gap-2">
                <Button type="primary" :loading="oidcSaving" :disabled="oidcTargetLoading" @click="doSaveOidc">保存停用</Button>
                <Button :disabled="oidcTargetLoading" @click="oidcSelection = oidc.provider_type || 'off'">取消草稿</Button>
              </div>
              <Space v-else>
                <Button v-if="oidc.provider_type" danger :disabled="oidcTargetLoading" @click="clearOidcOpen = true">清空 OIDC 配置</Button>
              </Space>
            </template>
          </div>
        </Card>

        <!-- OIDC 启用规则 -->
        <Card v-show="isGroupVisible('identity')" id="oidc-rules" title="OIDC 启用规则" size="small">
          <div class="space-y-3 max-w-xl">
            <div class="flex items-center gap-3">
              <span class="w-24 text-sm">审批开关</span>
              <Switch v-model:checked="oidcRules.approval_on" />
              <span class="text-xs text-text-tertiary">开启后新 OIDC 用户按白名单判定，未命中进入审批中心</span>
            </div>
            <Alert v-if="oidcRules.approval_on && !oidcRules.whitelist.role_values.length && !oidcRules.whitelist.group_values.length"
                   type="warning" show-icon message="白名单为空，新用户将全部直接激活" />
            <div class="flex items-center gap-3">
              <span class="w-24 text-sm">Role 声明路径</span>
              <AppSelect v-model:value="oidcRules.whitelist.role_claim_path" class="flex-1"
                      :options="[{ value: 'realm_access.roles', label: 'realm_access.roles' }, { value: 'roles', label: 'roles' }]"
                      allow-clear placeholder="点分路径" />
            </div>
            <div class="flex items-start gap-3">
              <span class="w-24 text-sm">Role 白名单</span>
              <AppSelect v-model:value="oidcRules.whitelist.role_values" mode="tags" class="flex-1" placeholder="输入值后回车（如 admin）" />
            </div>
            <div class="flex items-center gap-3">
              <span class="w-24 text-sm">Group 声明路径</span>
              <AppSelect v-model:value="oidcRules.whitelist.group_claim_path" class="flex-1"
                      :options="[{ value: 'groups', label: 'groups' }, { value: 'roles', label: 'roles' }]"
                      allow-clear placeholder="点分路径" />
            </div>
            <div class="flex items-start gap-3">
              <span class="w-24 text-sm">Group 白名单</span>
              <AppSelect v-model:value="oidcRules.whitelist.group_values" mode="tags" class="flex-1" placeholder="输入值后回车（如 vpn-users）" />
            </div>
            <Button type="primary" :loading="rulesSaving" @click="doSaveOidcRules">保存</Button>
          </div>
        </Card>

        <!-- 本地认证 -->
        <Card v-show="isGroupVisible('identity')" id="local-auth" title="本地认证" size="small">
          <div class="space-y-3 max-w-xl">
            <div class="flex items-center gap-3">
              <span class="w-40 text-sm">允许本地登录</span>
              <Switch v-model:checked="localAuth.allow_local_login" />
            </div>
            <div class="flex items-center gap-3">
              <span class="w-40 text-sm">允许自注册</span>
              <Switch v-model:checked="localAuth.allow_selfreg" />
            </div>
            <div class="flex items-center gap-3">
              <span class="w-40 text-sm">自注册审批</span>
              <Switch v-model:checked="localAuth.selfreg_approval" />
              <span class="text-xs text-text-tertiary">开启后自注册用户进入审批中心</span>
            </div>
            <Alert v-if="!localAuth.allow_local_login" type="warning" show-icon
                   message="本地登录关闭且 OIDC 不可用时将被禁止保存（防认证死锁）" />
            <Button type="primary" :loading="localSaving" @click="doSaveLocalAuth">保存</Button>
          </div>
        </Card>

        <!-- 验证码 -->
        <Card v-show="isGroupVisible('identity')" id="captcha" title="验证码" size="small">
          <div class="space-y-3 max-w-xl">
            <div class="flex items-center gap-3">
              <span class="w-24 text-sm">提供商</span>
              <Radio.Group v-model:value="captcha.provider">
                <Radio value="off">关闭</Radio>
                <Radio value="recaptcha">reCAPTCHA</Radio>
                <Radio value="turnstile">Cloudflare Turnstile</Radio>
              </Radio.Group>
            </div>
            <div class="flex items-center gap-3">
              <span class="w-24 text-sm">站点密钥</span>
              <Input v-model:value="captcha.site_key" placeholder="reCAPTCHA / Turnstile 站点密钥（明文回显）" />
            </div>
            <div class="flex items-center gap-3">
              <span class="w-24 text-sm">服务端密钥</span>
              <Input.Password v-model:value="captcha.secret_key" placeholder="reCAPTCHA / Turnstile 服务端密钥（明文回显）" />
            </div>
            <div class="flex items-center gap-3">
              <span class="w-24 text-sm">启用页面</span>
              <Checkbox.Group v-model:value="captcha.pages" :options="captchaPages" />
            </div>
            <Alert type="info" show-icon message="验证码依赖外部网络，局域网直连部署不建议启用" />
            <Button type="primary" :loading="captchaSaving" @click="doSaveCaptcha">保存</Button>
          </div>
        </Card>

        <!-- SMTP -->
        <Card v-show="isGroupVisible('notifications')" id="smtp" title="邮件发送 · 通用 SMTP" size="small">
          <div class="max-w-2xl space-y-5">
            <p class="text-sm text-text-secondary">填写邮件服务商提供的 SMTP 参数。测试邮件使用已保存的配置；修改后请先保存。</p>
            <Alert v-if="smtp.host && !smtp.configured" type="warning" show-icon message="当前 SMTP 设置尚未符合新连接方式要求，请重新选择、保存并发送测试邮件。" />
            <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <label class="space-y-1 text-sm"><span>SMTP 服务器</span><Input v-model:value="smtp.host" placeholder="smtp.example.com；本地中继填 127.0.0.1" /></label>
              <label class="space-y-1 text-sm"><span>端口</span><Input v-model:value="smtp.port" placeholder="STARTTLS 常见 587；连接即 TLS 常见 465" /></label>
              <label class="space-y-1 text-sm"><span>发件邮箱</span><Input v-model:value="smtp.from" placeholder="sender@example.com" /></label>
            </div>
            <div class="space-y-1 text-sm">
              <span>连接方式</span>
              <AppSelect v-model:value="smtp.security" :options="smtpSecurityOptions" class="w-full" />
              <p class="text-xs text-text-secondary">按服务商要求选择，端口不会自动决定连接方式。STARTTLS 必须升级成功才发送；“连接即 TLS”从建立连接时加密。无加密只允许本机回环地址上的无认证中继。</p>
            </div>
            <div class="space-y-2 text-sm">
              <div class="flex items-center gap-3"><span>需要 SMTP 认证</span><Switch v-model:checked="smtp.auth_required" :disabled="smtp.security === 'plain'" /></div>
              <p v-if="smtp.security === 'plain'" class="text-xs text-text-secondary">本地中继不发送账号或密码。</p>
              <p v-else-if="!smtp.auth_required" class="text-xs text-text-secondary">此连接将不发送 SMTP 账号或密码。</p>
            </div>
            <template v-if="smtp.auth_required">
              <label class="block space-y-1 text-sm"><span>登录账号</span><Input v-model:value="smtp.user" placeholder="服务商提供的 SMTP 账号" /></label>
              <div class="space-y-1 text-sm">
                <span>SMTP 专用密码 <Tag v-if="smtp.password_configured" color="success">已配置</Tag><Tag v-else>未配置</Tag></span>
                <Input.Password v-model:value="smtp.password" autocomplete="new-password" placeholder="留空保持原密码；输入新密码后保存" />
                <p class="text-xs text-text-secondary">已保存密码不会回显。若曾被占位符覆盖，请重新填写服务商提供的专用密码。</p>
              </div>
            </template>
            <div class="space-y-1 text-sm">
              <span>启用业务邮件</span>
              <Checkbox.Group v-model:value="smtp.scopes" :options="scopeOptions" class="flex flex-wrap gap-2" />
              <p class="text-xs text-text-secondary">测试邮件不受此范围限制；密码重置、审批通知与欢迎邮件分别按勾选项发送。</p>
            </div>
            <Button type="primary" :loading="smtpSaving" @click="doSaveSMTP">保存 SMTP 设置</Button>
            <div class="border-t border-border pt-4 space-y-2">
              <div class="text-sm font-medium">发送测试邮件</div>
              <p class="text-xs text-text-secondary">留空发送至当前管理员邮箱{{ auth.user?.email ? `（${auth.user.email}）` : '' }}；填写后只发送至指定地址。发送成功仅表示服务商接受该邮件，请另行检查收件箱。</p>
              <div class="flex flex-col gap-2 sm:flex-row">
                <Input v-model:value="smtpTestTo" type="email" placeholder="测试收件邮箱（可留空）" aria-label="测试收件邮箱" />
                <Button :loading="smtpTesting" class="sm:flex-none" @click="doTestSMTP">发送测试邮件</Button>
              </div>
            </div>
          </div>
        </Card>

        <!-- 站点信息 -->
        <Card v-show="isGroupVisible('content')" id="site" title="站点信息" size="small">
          <div class="space-y-3 max-w-xl">
            <div class="flex items-center gap-3">
              <span class="w-24 text-sm">站点名称</span>
              <Input v-model:value="site.site_name" :maxlength="50" show-count placeholder="≤50 字符" />
            </div>
            <div class="flex items-center gap-3">
              <span class="w-24 text-sm flex-none">站点 ICON</span>
              <Upload :before-upload="onSiteFile" :max-count="1" accept=".png,.jpeg,.jpg,.webp,.ico">
                <Button>选择文件（≤2MB）</Button>
              </Upload>
              <!-- flex-1 + min-w-0 + truncate：完整 URL 超长时省略号截断不溢出（title 提示完整路径） -->
              <span v-if="site.icon_url" class="text-xs text-text-tertiary flex-1 min-w-0 truncate"
                    :title="site.icon_url">当前：{{ site.icon_url }}</span>
            </div>
            <Space>
              <Button type="primary" :loading="siteSaving" @click="doSaveSite">保存</Button>
              <Button danger @click="iconDeleteOpen = true">删除恢复默认 ICON</Button>
            </Space>
          </div>
        </Card>

        <!-- 运行模式信息 -->
        <Card v-show="isGroupVisible('runtime')" id="mode" title="运行模式信息" size="small">
          <div class="flex items-center gap-3">
            <span class="text-sm">当前模式：</span>
            <Tag :color="isProd ? 'green' : 'orange'">{{ isProd ? 'Production' : 'Dev' }}</Tag>
            <span class="text-xs text-text-tertiary">由启动环境变量决定，修改需重启容器</span>
          </div>
        </Card>

          <!-- 高级模式 -->
          <Card v-show="isGroupVisible('runtime')" id="advanced" title="高级模式" size="small">
            <div class="space-y-3 max-w-xl">
              <div class="flex items-center gap-3">
                <span class="w-28 text-sm">高级模式</span>
                <Switch v-model:checked="advanced.advanced_mode" />
                <Tag :color="advanced.advanced_mode ? 'green' : 'default'">{{ advanced.advanced_mode ? '已开启' : '未开启' }}</Tag>
              </div>
              <div v-if="advanced.advanced_mode" class="flex items-center gap-3">
                <span class="w-28 text-sm">采集间隔（分钟）</span>
                <InputNumber v-model:value="advanced.collect_interval_minutes" :min="1" class="w-32" />
              </div>
              <div class="flex items-center gap-3">
                <span class="w-28 text-sm">流量卡片</span>
                <Switch v-model:checked="advanced.traffic_card_enabled" />
              </div>
              <Space>
                <Button type="primary" :loading="advancedSaving" @click="doSaveAdvanced">保存</Button>
              </Space>
              <Alert v-if="!advanced.advanced_mode" type="info" show-icon
                     message="开启高级模式后将解锁用户组节点分配、Xray 实例与独立账号管理。关闭高级模式会清空 Xray 相关数据，需输入确认词 DISABLE。" />
            </div>
          </Card>

          <ConfirmModal :open="advancedConfirmOpen" title="关闭高级模式" danger confirm-word="DISABLE" :loading="advancedSaving"
                        content="将移除全部 Xray 实例、Xray 节点、组分配、独立账号、流量记录与用户凭据；保留 proxy_groups、用户组与装配蓝图。此操作不可恢复。请输入 DISABLE 确认。"
                        @confirm="confirmDisableAdvanced" @update:open="advancedConfirmOpen = false" />


        <!-- 速率限制 -->
        <Card v-show="isGroupVisible('runtime')" id="ratelimit" title="速率限制与连接防护" size="small">
          <div class="space-y-3 max-w-xl">
            <div class="grid grid-cols-2 gap-3">
              <div><span class="text-sm">登录（次/分钟）</span><InputNumber v-model:value="rate.login" class="w-full mt-1" :min="1" /></div>
              <div><span class="text-sm">注册（次/分钟）</span><InputNumber v-model:value="rate.register" class="w-full mt-1" :min="1" /></div>
              <div><span class="text-sm">找回密码（次/分钟）</span><InputNumber v-model:value="rate.forgot" class="w-full mt-1" :min="1" /></div>
              <div><span class="text-sm">下载（次/分钟）</span><InputNumber v-model:value="rate.download" class="w-full mt-1" :min="1" /></div>
              <div><span class="text-sm">重置校验（次/分钟）</span><InputNumber v-model:value="rate.reset_validate" class="w-full mt-1" :min="1" /></div>
            </div>
            <div class="grid grid-cols-2 gap-3 border-t pt-3">
              <div><span class="text-sm">读头超时（秒）</span><InputNumber v-model:value="rate.http_read_header_timeout_sec" class="w-full mt-1" :min="1" :max="60" /></div>
              <div><span class="text-sm">读取超时（秒）</span><InputNumber v-model:value="rate.http_read_timeout_sec" class="w-full mt-1" :min="1" :max="3600" /></div>
              <div><span class="text-sm">写出超时（秒）</span><InputNumber v-model:value="rate.http_write_timeout_sec" class="w-full mt-1" :min="1" :max="3600" /></div>
              <div><span class="text-sm">空闲超时（秒）</span><InputNumber v-model:value="rate.http_idle_timeout_sec" class="w-full mt-1" :min="1" :max="3600" /></div>
              <div><span class="text-sm">API 请求体上限（MB）</span><InputNumber v-model:value="rate.http_max_body_mb" class="w-full mt-1" :min="1" :max="320" /></div>
            </div>
            <div class="text-sm">当前 IP 解析策略（TRUST_PROXY）：<Tag>{{ trustProxy }}</Tag></div>
            <div v-if="trustProxyCidrs" class="text-sm">TRUST_PROXY_CIDRS：<Tag>{{ trustProxyCidrs }}</Tag></div>
            <Alert v-if="trustProxy === 'auto'" type="warning" show-icon
                   message="auto 模式伪造风险：局域网直连的客户端可构造转发头伪造 IP 绕过限流，建议直连部署设 TRUST_PROXY=off" />
            <Alert type="info" show-icon message="超时字段在服务启动时读取，保存后需重启容器生效；API 请求体上限即时生效。" />
            <Button type="primary" :loading="rateSaving" @click="doSaveRateLimit">保存</Button>
          </div>
        </Card>

        <!-- 日志级别 -->
        <Card v-show="isGroupVisible('runtime')" id="log-level" title="日志级别" size="small">
          <div class="space-y-3 max-w-xl">
            <Radio.Group v-model:value="logLevel" button-style="solid">
              <Radio.Button value="debug">debug</Radio.Button>
              <Radio.Button value="info">info</Radio.Button>
              <Radio.Button value="warn">warn</Radio.Button>
              <Radio.Button value="error">error</Radio.Button>
            </Radio.Group>
            <div>
              <Button type="primary" @click="doSaveLogLevel">保存（立即生效）</Button>
            </div>
          </div>
        </Card>

        <!-- 公告与页脚（R10-07：三份独立配置，支持 Markdown） -->
        <Card v-show="isGroupVisible('content')" id="announcement" title="公告与页脚" size="small">
          <div class="space-y-3 max-w-xl">
            <Alert type="warning" show-icon message="公告与页脚内容接口公开可见（未登录可获取），请勿写入内部信息" />
            <div>
              <div class="mb-1 text-sm">首页公告</div>
              <Input.TextArea v-model:value="announcement.home_announcement" :rows="3" :maxlength="2000" show-count
                              placeholder="登录后首页顶部展示（支持 Markdown，≤2000 字符）" />
            </div>
            <div>
              <div class="mb-1 text-sm">登录页公告</div>
              <Input.TextArea v-model:value="announcement.login_announcement" :rows="3" :maxlength="2000" show-count
                              placeholder="登录卡片上方展示（支持 Markdown，≤2000 字符）" />
            </div>
            <div>
              <div class="mb-1 text-sm">登录页页脚</div>
              <Input.TextArea v-model:value="announcement.login_footer" :rows="3" :maxlength="2000" show-count
                              placeholder="登录卡片下方展示（支持 Markdown，≤2000 字符）" />
            </div>
            <Button type="primary" :loading="announcementSaving" @click="doSaveAnnouncement">保存</Button>
          </div>
        </Card>

        <!-- 调试模式 -->
        <Card v-show="isGroupVisible('runtime')" id="debug" title="调试模式" size="small">
          <div class="space-y-3 max-w-xl">
            <div class="flex items-center gap-3">
              <Switch v-model:checked="debugOn" />
              <span class="text-sm">开启后 5xx 错误响应返回详细内部信息，生产环境请保持关闭</span>
            </div>
            <Button type="primary" :loading="debugSaving" @click="doSaveDebug">保存</Button>
          </div>
        </Card>

        <!-- 配置导入/导出（仅 Production 渲染；Dev 显示说明文案） -->
        <Card v-show="isGroupVisible('data')" id="import-export" title="配置导入/导出" size="small">
          <Alert v-if="!isProd" type="info" show-icon message="Dev 模式不提供配置导入导出"
                 description="避免模拟 OIDC 等调试配置外流，同时免除模拟配置流入生产的拦截需求" />
          <div v-else class="space-y-4 max-w-xl">
            <div>
              <div class="mb-1 text-sm font-medium">导出配置</div>
              <div class="flex items-center gap-2">
                <Input.Password v-model:value="exportPwd" placeholder="设置导出密码（≥8 字符）" style="max-width: 260px" />
                <Button type="primary" :loading="exporting" @click="doExport">导出并下载</Button>
              </div>
              <div class="text-xs text-text-tertiary mt-1">内容：全部系统配置（含签名密钥与敏感密文）+ 站点信息（ICON 内嵌）；v2 额外包含 Xray 实例清单（含节点显示名映射）与独立账号推送目标/超限标记；不含业务数据与日志</div>
            </div>
            <div>
              <div class="mb-1 text-sm font-medium">导入配置（整体覆盖）</div>
              <Space>
                <Upload :before-upload="onImportFile" :max-count="1">
                  <Button>选择文件</Button>
                </Upload>
                <Input.Password v-model:value="importForm.password" placeholder="导出密码（≥8 字符）" style="max-width: 220px" />
                <Button danger @click="importOpen = true">导入</Button>
              </Space>
              <Alert v-if="importProtectError" type="error" show-icon class="mt-2" :message="importProtectError" />
              <div class="text-xs text-text-tertiary mt-1">导入将整体覆盖全部配置（导出文件中不存在的键一并清除）；v2 导入会整体覆盖 Xray 实例、组节点分配将被级联清空；带实例/账号导入且高级模式关闭时将自动开启高级模式；地址与 OIDC 配置即时生效，签名密钥替换后请重新登录，日志级别/HTTP 超时等启动参数重启后生效</div>
            </div>
          </div>
        </Card>

        <!-- 备份下载 -->
        <Card v-show="isGroupVisible('data')" id="backup" title="备份下载" size="small">
          <div class="space-y-2 max-w-xl">
            <Button :loading="backingUp" @click="backupOpen = true">下载备份（tar.gz）</Button>
            <div class="text-xs text-text-tertiary">含数据库一致性快照 + 全部内容文件；恢复方式见部署文档（手动解包到数据卷）</div>
          </div>
        </Card>

        <!-- 危险操作区（红色边框卡片） -->
        <div v-show="isGroupVisible('danger')" class="rounded-lg border border-red-200 bg-red-50/60 p-3 dark:border-red-900 dark:bg-red-950/20">
          <div class="flex items-center justify-between gap-3">
            <div><p class="m-0 font-medium text-red-700 dark:text-red-300">危险操作</p><p class="m-0 mt-1 text-xs text-red-600 dark:text-red-400">此区域默认收起，操作不可恢复。</p></div>
            <Button danger @click="dangerExpanded = !dangerExpanded">{{ dangerExpanded ? '收起' : '展开' }}</Button>
          </div>
        </div>
        <Card v-show="isGroupVisible('danger') && dangerExpanded" id="danger" title="危险操作区" size="small" class="border-red-300">
          <div class="space-y-2 max-w-xl">
            <Alert type="error" show-icon message="一键清空所有数据"
                   description="清空全部业务数据与系统配置（含签名密钥），删除全部内容文件，系统回到未配置状态（无需重启）；需输入确认词 RESET + 二次确认" />
            <Button danger :loading="clearing" @click="openClear">一键清空所有数据</Button>
          </div>
        </Card>
      </div>
    </div>

    <!-- 清空 OIDC 配置确认 -->
    <ConfirmModal :open="clearOidcOpen" title="清空 OIDC 配置" danger
                  content="将清空全部提供商参数与配置状态，本地登录未开启时将被拒绝。确定继续？"
                  @confirm="confirmClearOidc" @update:open="clearOidcOpen = false" />
    <!-- 删除 ICON 确认 -->
    <ConfirmModal :open="iconDeleteOpen" title="删除站点 ICON" danger
                  content="将删除已上传的站点 ICON 并恢复默认。确定继续？"
                  @confirm="confirmIconDelete" @update:open="iconDeleteOpen = false" />
    <!-- 导入确认（IMPORT 确认词） -->
    <ConfirmModal :open="importOpen" title="导入配置（整体覆盖）" danger confirm-word="IMPORT" :loading="importing"
                  content="导入将整体覆盖全部配置：导出文件中不存在的配置键一并清除；v2 导入会整体覆盖 Xray 实例、组节点分配将被级联清空；带实例/账号导入且高级模式关闭时将自动开启高级模式；签名密钥替换后全部会话立即失效（含当前管理员），请重新登录；地址与 OIDC 配置即时生效，日志级别/HTTP 超时等启动参数重启后生效。"
                  @confirm="doImport" @update:open="importOpen = false" />
      <!-- v2 导入第二确认（DISABLE；仅当导入会清空高级模式数据时后端强制校验） -->
      <ConfirmModal :open="disableImportOpen" title="确认清空高级模式数据" danger confirm-word="DISABLE" :loading="importing"
                    content="该导入文件不包含 Xray 实例/独立账号且高级模式为关闭状态，将按 OFF 清空口径移除旧高级数据（用户凭据、配额、流量记录、Xray 表等）。请输入 DISABLE 继续。"
                    @confirm="submitImport(true)" @update:open="disableImportOpen = false" />
    <!-- 备份确认 -->
    <ConfirmModal :open="backupOpen" title="下载备份"
                  content="将生成数据库一致性快照并打包全部内容文件下载（tar.gz），确定继续？"
                  @confirm="doBackup" @update:open="backupOpen = false" />
    <!-- 一键清空确认（RESET 确认词 + 二次确认） -->
    <ConfirmModal :open="clearOpen" title="一键清空所有数据" danger confirm-word="RESET" :loading="clearing"
                  content="将清空全部业务数据与系统配置、删除全部内容文件，系统回到未配置状态并进入首次配置。此操作不可恢复！"
                  @confirm="doClearAll" @update:open="clearOpen = false" />
  </div>
</template>

<style scoped>
.settings-group-nav { width: 100%; border: 0; border-radius: .5rem; padding: .65rem .75rem; text-align: left; color: rgb(75 85 99); background: transparent; cursor: pointer; }
.settings-group-nav:hover { background: rgb(243 244 246); }
.settings-group-nav.active { color: rgb(30 64 175); background: rgb(239 246 255); }
:global(.dark) .settings-group-nav { color: rgb(203 213 225); }
:global(.dark) .settings-group-nav:hover { background: rgb(31 41 55); }
:global(.dark) .settings-group-nav.active { color: rgb(147 197 253); background: rgb(30 58 138 / .35); }
</style>
