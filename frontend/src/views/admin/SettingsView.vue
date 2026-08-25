<!-- SettingsView.vue：面板配置（UI §5.8，Design1 §3.4.8）——左侧锚点 + 右侧分区卡片；<768 锚点转顶部 Select -->
<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import dayjs from 'dayjs'
import {
  Alert, Anchor, Button, Card, Checkbox, Input, InputNumber, Modal, Radio, Select, Space, Switch, Tag, Upload,
} from 'ant-design-vue'
import {
  getOidc, saveOidc, clearOidc, testOidc, getOidcRules, saveOidcRules, getLocalAuth, saveLocalAuth,
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
import ConfirmModal from '@/components/ConfirmModal.vue'
import PageHeader from '@/components/PageHeader.vue'
import { Notify } from '@/components/Notify'

const system = useSystemStore()
const auth = useAuthStore()
const router = useRouter()
const isProd = ref(system.status?.app_mode === 'prod')

// --- 分区锚点 ---
const sections = [
  { key: 'oidc', title: 'OIDC 配置' },
  { key: 'oidc-rules', title: 'OIDC 启用规则' },
  { key: 'local-auth', title: '本地认证' },
  { key: 'captcha', title: '验证码' },
  { key: 'smtp', title: 'SMTP' },
  { key: 'site', title: '站点信息' },
  { key: 'mode', title: '运行模式信息' },
  { key: 'advanced', title: '高级模式' },
  { key: 'ratelimit', title: '速率限制' },
  { key: 'log-level', title: '日志级别' },
  { key: 'announcement', title: '公告与页脚' },
  { key: 'debug', title: '调试模式' },
  { key: 'import-export', title: '配置导入/导出' },
  { key: 'backup', title: '备份下载' },
  { key: 'danger', title: '危险操作区' },
]

// --- OIDC 配置 ---
const oidc = reactive<OidcSettings>({ provider_type: 'generic', base_url: '', realm: '', client_id: '', client_secret: '', frontend_url: '', callback_url: '' })
const oidcSaving = ref(false)
const oidcTest = ref<{ ok: boolean; message: string } | null>(null)
const providerOptions = [
  { label: '暂未启用（本地账号模式）', value: 'off' }, // R10-08：off 为前端显示值，映射 provider_type 空串（未配置）
  { label: 'Keycloak', value: 'keycloak' },
  { label: 'Auth0', value: 'auth0' },
  { label: 'Generic OIDC', value: 'generic' },
  { label: 'Mock（仅 Dev）', value: 'mock' },
]
async function loadOidc() {
  try {
    Object.assign(oidc, await getOidc())
  } catch (err) {
    Notify.error((err as Error).message)
  }
}
// OIDC 字段随提供商动态显隐（R10-02）：Auth0 用 Domain 标识；Realm 仅 Keycloak 适用；Mock 无参数
const isMockProvider = computed(() => oidc.provider_type === 'mock')
const urlLabel = computed(() => (oidc.provider_type === 'auth0' ? 'Domain' : 'Base URL'))
const urlPlaceholder = computed(() => (oidc.provider_type === 'auth0' ? 'your-tenant.auth0.com' : 'https://idp.example.com'))
const showRealm = computed(() => oidc.provider_type === 'keycloak')
// 切换提供商：写入类型并清空不适用字段（realm 仅 Keycloak 适用；残留会导致 Auth0/通用发现文档 URL 拼接错误）
function applyProvider(v: string) {
  oidc.provider_type = v
  if (v !== 'keycloak') oidc.realm = ''
}
// onProviderChange：'off'（暂未启用）映射为空串；首次配置直接生效，启用态之间切换需确认（含切到暂未启用——R10-08）
function onProviderChange(v: any) {
  const target = v === 'off' ? '' : v
  if (target === oidc.provider_type) return
  if (oidc.provider_type === '') {
    applyProvider(target) // 首次配置：直接生效
    return
  }
  Modal.confirm({
    title: '切换提供商类型',
    content: target === ''
      ? '切换为暂未启用将停用 OIDC 登录，已绑定 OIDC 身份的账号将无法通过 OIDC 登录（本地密码登录不受影响）。确定？'
      : '已绑定旧提供商 OIDC 身份的用户在新提供商下登录将失效，建议先为相关管理员设置本地密码。切换后通用字段（地址/Client ID/Secret）保留；Realm 为 Keycloak 专用，切换后自动清空。',
    okText: '继续切换',
    cancelText: '取消',
    onOk: () => applyProvider(target),
  })
}
async function doSaveOidc() {
  oidcSaving.value = true
  try {
    const res = await saveOidc({ ...oidc })
    Notify.success('OIDC 配置已保存')
    if (res.need_restart) {
      Modal.warning({ title: '需重启容器生效', content: '前端地址与回调地址修改后需重启容器生效，请同步核对两字段' })
    }
    await loadOidc()
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    oidcSaving.value = false
  }
}
async function doTestOidc() {
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
    await loadOidc()
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
    await loadCaptcha()
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    captchaSaving.value = false
  }
}

// --- SMTP ---
const smtp = reactive<SMTPSettings>({ host: '', port: '587', user: '', password: '', from: '', tls: false, scopes: [] })
const smtpSaving = ref(false)
const smtpTesting = ref(false)
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
    await loadSMTP()
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    smtpSaving.value = false
  }
}
async function doTestSMTP() {
  smtpTesting.value = true
  try {
    await testSMTP()
    Notify.success('测试邮件已发送，请查收')
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
    iconDeleteOpen.value = false
  } catch (err) {
    Notify.error((err as Error).message)
  }
}

// --- 速率限制 ---
const rate = reactive<RateLimitSettings>({
  login: 10, register: 5, forgot: 5, download: 20,
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
    let importHintText = '配置已整体覆盖（导出文件中不存在的配置键已清除）；前端地址与回调地址已按导出值覆盖，若域名/端口有变化请先核对修改（修改后需重启生效）。请立即重启容器后再重新登录。'
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
        ? '配置已导入并完成异步处理，完成提示：\n' + hints.join('\n')
        : '配置已导入并完成异步处理，请刷新页面后确认高级模式状态。'
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

onMounted(() => {
  void Promise.all([loadOidc(), loadOidcRules(), loadLocalAuth(), loadCaptcha(), loadSMTP(),
    loadSite(), loadRateLimit(), loadLogLevel(), loadAnnouncement(), loadDebug(), loadAdvanced()])
  void system.fetchStatus()
})
</script>

<template>
  <div>
    <PageHeader title="面板配置" />

    <div class="flex gap-6">
      <!-- 左侧锚点导航（<768 隐藏，改用顶部 Select）；
           sticky 吸顶（顶栏 64px 下） + 限高独立滚动条，不随页面滚动出屏 -->
      <div class="hidden md:block w-40 shrink-0">
        <div class="sticky top-20 max-h-[calc(100vh-6rem)] overflow-y-auto">
          <!-- offset-top=80：点击锚点滚动目标时预留 sticky 顶栏（64px）+ 间距，避免目标被顶栏遮挡 -->
          <Anchor :items="sections.map((s) => ({ key: s.key, href: `#${s.key}`, title: s.title }))" :offset-top="80" />
        </div>
      </div>

      <div class="flex-1 space-y-4 min-w-0 settings-scroll">
        <!-- OIDC 配置 -->
        <Card id="oidc" title="OIDC 配置" size="small">
          <div class="space-y-3 max-w-xl">
            <div class="flex items-center gap-3">
              <span class="w-24 text-sm">提供商类型</span>
              <Select class="flex-1" :value="oidc.provider_type || 'off'" :options="providerOptions" @change="onProviderChange" />
            </div>
            <!-- 未配置（暂未启用）：折叠全部配置框（R10-08） -->
            <template v-if="oidc.provider_type">
              <template v-if="!isMockProvider">
                <div class="flex items-center gap-3">
                  <span class="w-24 text-sm">{{ urlLabel }}</span>
                  <Input v-model:value="oidc.base_url" :placeholder="urlPlaceholder" />
                </div>
                <div v-if="showRealm" class="flex items-center gap-3">
                  <span class="w-24 text-sm">Realm</span>
                  <Input v-model:value="oidc.realm" placeholder="Keycloak 专用，如 master" />
                </div>
                <div class="flex items-center gap-3">
                  <span class="w-24 text-sm">Client ID</span>
                  <Input v-model:value="oidc.client_id" placeholder="客户端标识" />
                </div>
                <div class="flex items-center gap-3">
                  <span class="w-24 text-sm">Client Secret</span>
                  <Input.Password v-model:value="oidc.client_secret" placeholder="已配置时留空不修改" />
                </div>
              </template>
              <Alert v-else type="info" show-icon message="模拟 OIDC：无需参数，登录页将显示 Dev 模拟登录表单" />
              <Alert type="info" show-icon message="接入提示" description="OIDC 回调要求公网可达的 HTTPS 域名，局域网直连模式可能无法完成回调" />
              <div class="flex items-center gap-3">
                <span class="w-24 text-sm">前端地址</span>
                <Input v-model:value="oidc.frontend_url" placeholder="https://app.example.com" />
              </div>
              <div class="flex items-center gap-3">
                <span class="w-24 text-sm">回调地址</span>
                <Input v-model:value="oidc.callback_url" placeholder="https://app.example.com/api/auth/oidc/callback" />
              </div>
              <Alert v-if="oidc.frontend_url || oidc.callback_url" type="warning" show-icon message="前端地址/回调地址修改后需重启容器生效" />
              <Alert v-if="oidcTest" :type="oidcTest.ok ? 'success' : 'error'" show-icon :message="oidcTest.message" />
              <Space>
                <Button type="primary" :loading="oidcSaving" @click="doSaveOidc">保存</Button>
                <Button @click="doTestOidc">测试连接</Button>
                <Button danger @click="clearOidcOpen = true">清空 OIDC 配置</Button>
              </Space>
            </template>
            <Alert v-else type="info" show-icon message="暂未启用 OIDC：保持本地账号模式，或选择上方提供商开始配置" />
          </div>
        </Card>

        <!-- OIDC 启用规则 -->
        <Card id="oidc-rules" title="OIDC 启用规则" size="small">
          <div class="space-y-3 max-w-xl">
            <div class="flex items-center gap-3">
              <span class="w-24 text-sm">审批开关</span>
              <Switch v-model:checked="oidcRules.approval_on" />
              <span class="text-xs text-gray-400">开启后新 OIDC 用户按白名单判定，未命中进入审批中心</span>
            </div>
            <Alert v-if="oidcRules.approval_on && !oidcRules.whitelist.role_values.length && !oidcRules.whitelist.group_values.length"
                   type="warning" show-icon message="白名单为空，新用户将全部直接激活" />
            <div class="flex items-center gap-3">
              <span class="w-24 text-sm">Role 声明路径</span>
              <Select v-model:value="oidcRules.whitelist.role_claim_path" class="flex-1"
                      :options="[{ value: 'realm_access.roles', label: 'realm_access.roles' }, { value: 'roles', label: 'roles' }]"
                      allow-clear placeholder="点分路径" />
            </div>
            <div class="flex items-start gap-3">
              <span class="w-24 text-sm">Role 白名单</span>
              <Select v-model:value="oidcRules.whitelist.role_values" mode="tags" class="flex-1" placeholder="输入值后回车（如 admin）" />
            </div>
            <div class="flex items-center gap-3">
              <span class="w-24 text-sm">Group 声明路径</span>
              <Select v-model:value="oidcRules.whitelist.group_claim_path" class="flex-1"
                      :options="[{ value: 'groups', label: 'groups' }, { value: 'roles', label: 'roles' }]"
                      allow-clear placeholder="点分路径" />
            </div>
            <div class="flex items-start gap-3">
              <span class="w-24 text-sm">Group 白名单</span>
              <Select v-model:value="oidcRules.whitelist.group_values" mode="tags" class="flex-1" placeholder="输入值后回车（如 vpn-users）" />
            </div>
            <Button type="primary" :loading="rulesSaving" @click="doSaveOidcRules">保存</Button>
          </div>
        </Card>

        <!-- 本地认证 -->
        <Card id="local-auth" title="本地认证" size="small">
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
              <span class="text-xs text-gray-400">开启后自注册用户进入审批中心</span>
            </div>
            <Alert v-if="!localAuth.allow_local_login" type="warning" show-icon
                   message="本地登录关闭且 OIDC 不可用时将被禁止保存（防认证死锁）" />
            <Button type="primary" :loading="localSaving" @click="doSaveLocalAuth">保存</Button>
          </div>
        </Card>

        <!-- 验证码 -->
        <Card id="captcha" title="验证码" size="small">
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
        <Card id="smtp" title="SMTP" size="small">
          <div class="space-y-3 max-w-xl">
            <div class="flex items-center gap-3">
              <span class="w-24 text-sm">服务器</span>
              <Input v-model:value="smtp.host" placeholder="smtp.example.com" />
            </div>
            <div class="flex items-center gap-3">
              <span class="w-24 text-sm">端口</span>
              <Input v-model:value="smtp.port" placeholder="587" />
            </div>
            <div class="flex items-center gap-3">
              <span class="w-24 text-sm">账号</span>
              <Input v-model:value="smtp.user" placeholder="登录账号" />
            </div>
            <div class="flex items-center gap-3">
              <span class="w-24 text-sm">密码</span>
              <Input.Password v-model:value="smtp.password" placeholder="已配置时留空不修改" />
            </div>
            <div class="flex items-center gap-3">
              <span class="w-24 text-sm">发件人</span>
              <Input v-model:value="smtp.from" placeholder="缺省取账号" />
            </div>
            <div class="flex items-center gap-3">
              <span class="w-24 text-sm">TLS</span>
              <Switch v-model:checked="smtp.tls" />
            </div>
            <div class="flex items-center gap-3">
              <span class="w-24 text-sm">启用范围</span>
              <Checkbox.Group v-model:value="smtp.scopes" :options="scopeOptions" />
            </div>
            <Space>
              <Button type="primary" :loading="smtpSaving" @click="doSaveSMTP">保存</Button>
              <Button :loading="smtpTesting" @click="doTestSMTP">发送测试邮件</Button>
            </Space>
          </div>
        </Card>

        <!-- 站点信息 -->
        <Card id="site" title="站点信息" size="small">
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
              <span v-if="site.icon_url" class="text-xs text-gray-400 flex-1 min-w-0 truncate"
                    :title="site.icon_url">当前：{{ site.icon_url }}</span>
            </div>
            <Space>
              <Button type="primary" :loading="siteSaving" @click="doSaveSite">保存</Button>
              <Button danger @click="iconDeleteOpen = true">删除恢复默认 ICON</Button>
            </Space>
          </div>
        </Card>

        <!-- 运行模式信息 -->
        <Card id="mode" title="运行模式信息" size="small">
          <div class="flex items-center gap-3">
            <span class="text-sm">当前模式：</span>
            <Tag :color="isProd ? 'green' : 'orange'">{{ isProd ? 'Production' : 'Dev' }}</Tag>
            <span class="text-xs text-gray-400">由启动环境变量决定，修改需重启容器</span>
          </div>
        </Card>

          <!-- 高级模式 -->
          <Card id="advanced" title="高级模式" size="small">
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
        <Card id="ratelimit" title="速率限制与连接防护" size="small">
          <div class="space-y-3 max-w-xl">
            <div class="grid grid-cols-2 gap-3">
              <div><span class="text-sm">登录（次/分钟）</span><InputNumber v-model:value="rate.login" class="w-full mt-1" :min="1" /></div>
              <div><span class="text-sm">注册（次/分钟）</span><InputNumber v-model:value="rate.register" class="w-full mt-1" :min="1" /></div>
              <div><span class="text-sm">找回密码（次/分钟）</span><InputNumber v-model:value="rate.forgot" class="w-full mt-1" :min="1" /></div>
              <div><span class="text-sm">下载（次/分钟）</span><InputNumber v-model:value="rate.download" class="w-full mt-1" :min="1" /></div>
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
        <Card id="log-level" title="日志级别" size="small">
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
        <Card id="announcement" title="公告与页脚" size="small">
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
        <Card id="debug" title="调试模式" size="small">
          <div class="space-y-3 max-w-xl">
            <div class="flex items-center gap-3">
              <Switch v-model:checked="debugOn" />
              <span class="text-sm">开启后 5xx 错误响应返回详细内部信息，生产环境请保持关闭</span>
            </div>
            <Button type="primary" :loading="debugSaving" @click="doSaveDebug">保存</Button>
          </div>
        </Card>

        <!-- 配置导入/导出（仅 Production 渲染；Dev 显示说明文案） -->
        <Card id="import-export" title="配置导入/导出" size="small">
          <Alert v-if="!isProd" type="info" show-icon message="Dev 模式不提供配置导入导出"
                 description="避免模拟 OIDC 等调试配置外流，同时免除模拟配置流入生产的拦截需求" />
          <div v-else class="space-y-4 max-w-xl">
            <div>
              <div class="mb-1 text-sm font-medium">导出配置</div>
              <div class="flex items-center gap-2">
                <Input.Password v-model:value="exportPwd" placeholder="设置导出密码（≥8 字符）" style="max-width: 260px" />
                <Button type="primary" :loading="exporting" @click="doExport">导出并下载</Button>
              </div>
              <div class="text-xs text-gray-400 mt-1">内容：全部系统配置（含签名密钥与敏感密文）+ 站点信息（ICON 内嵌）；v2 额外包含 Xray 实例清单（含节点显示名映射）与独立账号推送目标/超限标记；不含业务数据与日志</div>
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
              <div class="text-xs text-gray-400 mt-1">导入将整体覆盖全部配置（导出文件中不存在的键一并清除）；v2 导入会整体覆盖 Xray 实例、组节点分配将被级联清空；带实例/账号导入且高级模式关闭时将自动开启高级模式；完成后需重启容器并重新登录</div>
            </div>
          </div>
        </Card>

        <!-- 备份下载 -->
        <Card id="backup" title="备份下载" size="small">
          <div class="space-y-2 max-w-xl">
            <Button :loading="backingUp" @click="backupOpen = true">下载备份（tar.gz）</Button>
            <div class="text-xs text-gray-400">含数据库一致性快照 + 全部内容文件；恢复方式见部署文档（手动解包到数据卷）</div>
          </div>
        </Card>

        <!-- 危险操作区（红色边框卡片） -->
        <Card id="danger" title="危险操作区" size="small" class="border-red-300">
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
                  content="导入将整体覆盖全部配置：导出文件中不存在的配置键一并清除；v2 导入会整体覆盖 Xray 实例、组节点分配将被级联清空；带实例/账号导入且高级模式关闭时将自动开启高级模式；签名密钥替换后全部会话立即失效（含当前管理员）；导入完成后请立即重启容器再重新登录。"
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
/* 锚点滚动目标预留 sticky 顶栏高度（64px + 16px 间距）：
   覆盖浏览器原生 #hash 直链跳转场景（AntD Anchor offset-top 仅作用于点击路径） */
.settings-scroll :deep([id]) {
  scroll-margin-top: 80px;
}
</style>
