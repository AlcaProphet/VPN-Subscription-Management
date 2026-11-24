<!-- ProviderTrafficSettings.vue：基础模式服务器剩余流量管理设置（Build41 Step 5 / Design7 §三）。 -->
<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { Alert, Button, Card, Input, Switch, Tag } from 'ant-design-vue'
import dayjs from 'dayjs'
import ConfirmModal from '@/components/ConfirmModal.vue'
import { Notify } from '@/components/Notify'
import { ApiError } from '@/api/request'
import { useSystemStore } from '@/stores/system'
import {
  deleteProviderTrafficBinding,
  deleteProviderTrafficHost,
  deleteProviderTrafficVPSAccount,
  getProviderTrafficSettings,
  refreshProviderTraffic,
  refreshProviderTrafficHost,
  saveProviderTrafficBinding,
  saveProviderTrafficHost,
  saveProviderTrafficVPSAccount,
  saveProviderTrafficEnabled,
  PROVIDER_TRAFFIC_LABELS,
  PROVIDER_TRAFFIC_PROVIDERS,
  type Provider,
  type ProviderTrafficBinding,
  type ProviderTrafficAccount,
  type ProviderTrafficBindingPayload,
  type ProviderTrafficSettings as ProviderTrafficSettingsData,
} from '@/api/providerTraffic'

const emit = defineEmits<{
  (e: 'dirty-change', dirty: boolean): void
}>()

const system = useSystemStore()

const loading = ref(false)
const loaded = ref(false)
const loadError = ref('')
const available = ref(false)
const savedEnabled = ref(false)
const enabledDraft = ref(false)
const totalSaving = ref(false)
const totalError = ref('')
const deleteTarget = ref<{ key: string; name: string; revision: string } | null>(null)
const supportsHosts = ref(false)
const account = ref<ProviderTrafficAccount>({ configured: false, revision: '' })
const accountBaseRevision = ref('')
const accountCredential = ref('')
const accountSaving = ref(false)
const accountError = ref('')
const accountConfirmRevision = ref('')
const accountConflict = computed(() => accountBaseRevision.value !== account.value.revision)
let loadVersion = 0
let reloadAfterWrite = false

interface ProviderDraft {
  name: string
  halfRemaining: boolean
  enabled: boolean
  instanceId: string
  credential: string
}

interface Binding extends ProviderTrafficBinding {
  id?: string
  name?: string
  half_remaining?: boolean
}

interface ProviderState {
  saved: Binding
  draftBase: Binding
  draft: ProviderDraft
  saving: boolean
  deleting: boolean
  refreshing: boolean
  actionError: string
}

function blankBinding(provider: Provider): ProviderTrafficBinding {
  return {
    provider,
    enabled: false,
    instance_id: null,
    revision: null,
    credential_configured: false,
    result: null,
  }
}

function blankState(provider: Provider): ProviderState {
  const saved = blankBinding(provider)
  return {
    saved,
    draftBase: saved,
    draft: { name: '', halfRemaining: false, enabled: false, instanceId: '', credential: '' },
    saving: false,
    deleting: false,
    refreshing: false,
    actionError: '',
  }
}

const states = reactive<Record<string, ProviderState>>({
  kiwivm: blankState('kiwivm'),
  vps: blankState('vps'),
})

const rows = computed(() => PROVIDER_TRAFFIC_PROVIDERS.flatMap((provider) =>
  Object.entries(states)
    .filter(([key, state]) => state.saved.provider === provider
      && (supportsHosts.value ? key !== provider : key === provider))
    .map(([key, state]) => ({ key, provider, state })),
))
const vpsHasHosts = computed(() => rows.value.some(({ state }) => state.saved.provider === 'vps' && state.saved.revision))

// 服务端返回的 available 是权威值，也要处理在同页保存的高级模式变更。
const effectiveAvailable = computed(() => available.value && system.status?.advanced_mode !== true)

function providerDirty(state: ProviderState): boolean {
  return state.draft.enabled !== state.draftBase.enabled
    || state.draft.instanceId !== (state.draftBase.instance_id ?? '')
    || state.draft.name !== (state.draftBase.name ?? '')
    || state.draft.halfRemaining !== (state.draftBase.half_remaining ?? false)
    || state.draft.credential !== ''
}

function bindingConflict(state: ProviderState): boolean {
  return state.draftBase.revision !== state.saved.revision
}

function resetDraft(state: ProviderState) {
  const { enabled, instance_id, name, half_remaining } = state.saved
  state.draftBase = state.saved
  state.draft = { enabled, instanceId: instance_id ?? '', name: name ?? '', halfRemaining: half_remaining ?? false, credential: '' }
  state.actionError = ''
}

function useLatest(key: string) {
  const state = states[key]
  if (supportsHosts.value && !state.saved.revision && !key.startsWith('new-')) delete states[key]
  else resetDraft(state)
}

function addHost(provider: Provider) {
  const key = `new-${provider}`
  if (!canEdit.value || states[key]) return
  states[key] = blankState(provider)
  states[key].draft.enabled = true
}

function resetAccountDraft() {
  accountCredential.value = ''
  accountBaseRevision.value = account.value.revision
  accountError.value = ''
}

const totalDirty = computed(() => enabledDraft.value !== savedEnabled.value)
const dirty = computed(() => totalDirty.value || accountCredential.value !== ''
  || rows.value.some(({ state }) => providerDirty(state)))
const writing = computed(() => totalSaving.value || accountSaving.value
  || rows.value.some(({ state }) => state.saving || state.deleting))
const canEdit = computed(() => effectiveAvailable.value && !loading.value && !loadError.value && !writing.value)

const deleteLoading = computed(() => deleteTarget.value
  ? (states[deleteTarget.value.key]?.deleting ?? false) : accountSaving.value)
const deleteContent = computed(() => {
  if (accountConfirmRevision.value) return '将清除本站保存的 V.PS Refresh Token。确定继续？'
  const target = deleteTarget.value
  if (!target) return ''
  return `将解除 ${target.name} 的服务器流量绑定，并清除该实例的设置和最近采集结果。确定继续？`
})

function closeConfirmation() {
  deleteTarget.value = null
  accountConfirmRevision.value = ''
}

watch(dirty, (value) => {
  if (loaded.value) emit('dirty-change', value)
})

// 基础模式恢复后等待本地写入完成再重读，避免旧读取覆盖写入响应。
watch(() => system.status?.advanced_mode, (advanced, previous) => {
  if (advanced === true) {
    closeConfirmation()
    reloadAfterWrite = false
  } else if (previous === true) {
    if (writing.value) reloadAfterWrite = true
    else void loadSettings()
  }
})
watch(writing, (busy) => {
  if (busy || !reloadAfterWrite) return
  reloadAfterWrite = false
  void loadSettings()
})

interface ApplySettingsOptions {
  submittedTotal?: boolean
  changedKey?: string
  savedAccount?: boolean
}

// 合并服务端快照时，保留其他主机上尚未保存的草稿及其编辑基准。
function applySettings(data: ProviderTrafficSettingsData, options: ApplySettingsOptions = {}) {
  available.value = data.available
  if (!data.available) {
    closeConfirmation()
    return
  }
  const wasTotalDirty = totalDirty.value
  savedEnabled.value = data.enabled
  if (options.submittedTotal || !wasTotalDirty) enabledDraft.value = savedEnabled.value

  supportsHosts.value = data.hosts !== undefined
  const incoming: [string, Binding][] = data.hosts
    ? data.hosts.map((item) => [`host-${item.id}`, item])
    : PROVIDER_TRAFFIC_PROVIDERS.map((provider) => [
      provider, data.providers.find((item) => item.provider === provider) ?? blankBinding(provider),
    ])
  const keys = new Set(incoming.map(([key]) => key))
  for (const [key, item] of incoming) {
    const state = states[key] ?? (states[key] = blankState(item.provider))
    const wasDirty = providerDirty(state)
    state.saved = item
    if (options.changedKey === key || !wasDirty) resetDraft(state)
  }
  for (const [key, state] of Object.entries(states)) {
    if (!key.startsWith('host-') || keys.has(key)) continue
    if (options.changedKey === key || !providerDirty(state)) delete states[key]
    else state.saved = blankBinding(state.saved.provider)
  }
  if (options.changedKey?.startsWith('new-')) delete states[options.changedKey]

  account.value = data.vps_account ?? { configured: false, revision: '' }
  if (options.savedAccount || accountCredential.value === '') resetAccountDraft()
  if (accountConfirmRevision.value && (accountConfirmRevision.value !== account.value.revision || vpsHasHosts.value)) {
    accountConfirmRevision.value = ''
    accountError.value = '账号设置已变化，请重新确认'
  }
  const target = deleteTarget.value
  if (target && states[target.key]?.saved.revision !== target.revision) {
    deleteTarget.value = null
    const state = states[target.key]
    if (state) state.actionError = '绑定已更改，请重新确认解除绑定'
  }
}

async function loadSettings() {
  const version = ++loadVersion
  loading.value = true
  loadError.value = ''
  try {
    const data = await getProviderTrafficSettings()
    if (version !== loadVersion) return
    applySettings(data)
    loaded.value = true
    emit('dirty-change', dirty.value)
  } catch (err) {
    if (version === loadVersion) {
      loadError.value = (err as Error).message || '服务器剩余流量设置读取失败'
    }
  } finally {
    if (version === loadVersion) loading.value = false
  }
}

function retryLoad() {
  void loadSettings()
}

onMounted(() => {
  void loadSettings()
})

function friendlyError(err: unknown): string {
  const message = err instanceof Error && err.message ? err.message : '操作失败'
  if (!(err instanceof ApiError)) return message
  if (err.status === 409) {
    if (message.includes('已添加') || message.includes('V.PS')) return message
    if (message.includes('正在采集')) return '正在采集，请稍后再试'
    if (message.includes('修订')) return '绑定已在其他页面更改，请刷新页面后重试'
    if (message.includes('未启用')) return '当前绑定未启用，请先保存并启用'
    if (message.includes('已丢弃')) return '本轮采集结果已丢弃，请刷新页面后重试'
    return '当前状态已变化，请刷新页面后重试'
  }
  if (err.status === 403) return message.includes('模式') ? '当前模式不可用，请刷新页面后重试' : '权限不足'
  if (err.status === 0) return '网络异常，请稍后重试'
  return message
}

const INSTANCE_ID_PATTERN = /^[1-9][0-9]*$/

function instanceLabel(provider: Provider): string {
  return provider === 'kiwivm' ? 'VEID' : 'Service ID'
}

function credentialLabel(provider: Provider): string {
  return provider === 'kiwivm' ? 'API Key' : 'Refresh Token'
}

function instancePlaceholder(): string {
  return '正十进制数字实例 ID'
}

function instanceChanged(state: ProviderState): boolean {
  return state.draft.instanceId.trim() !== (state.draftBase.instance_id ?? '')
}

function credentialPlaceholder(state: ProviderState): string {
  if (state.draft.instanceId.trim() !== '' && instanceChanged(state)) return '更换实例必须填写新凭据'
  if (state.saved.credential_configured) return '留空保留已保存凭据；输入新值将替换'
  return '首次绑定必须填写凭据'
}

function credentialHint(state: ProviderState): string {
  if (state.saved.credential_configured) {
    return '已保存凭据不会回显。同一实例留空按省略提交；更换实例必须输入新凭据。'
  }
  return '首次绑定必须填写凭据；保存成功后输入框会清空。'
}

function formatRemaining(value?: number): string {
  return typeof value === 'number' && Number.isFinite(value) ? value.toFixed(2) : '—'
}

function formatSampledAt(value?: string): string {
  return value ? dayjs(value).format('MM-DD HH:mm') : '—'
}

function emptyResultText(state: ProviderState): string {
  if (!state.saved.instance_id || !state.saved.revision) return '未绑定'
  if (!state.saved.enabled) return '平台已停用'
  if (!savedEnabled.value) return '总开关关闭，暂不采集'
  return '等待采集'
}

function canRefresh(state: ProviderState): boolean {
  return canEdit.value
    && !state.refreshing
    && state.saved.enabled
    && state.saved.instance_id !== null
    && state.saved.revision !== null
    && !totalDirty.value
    && savedEnabled.value
    && !providerDirty(state)
    && !bindingConflict(state)
}

async function saveEnabled() {
  if (!canEdit.value) return
  totalError.value = ''
  totalSaving.value = true
  try {
    const data = await saveProviderTrafficEnabled(enabledDraft.value)
    applySettings(data, { submittedTotal: true })
    Notify.success('服务器剩余流量总开关已保存')
  } catch (err) {
    totalError.value = friendlyError(err)
    Notify.error(totalError.value)
  } finally {
    totalSaving.value = false
  }
}

async function saveHost(key: string, state: ProviderState) {
  if (!canEdit.value || bindingConflict(state)) return
  const provider = state.saved.provider
  const usesHosts = supportsHosts.value
  const needsHostCredential = !usesHosts || provider === 'kiwivm'
  const name = state.draft.name.trim()
  if (usesHosts && (!name || Array.from(name).length > 64)) {
    state.actionError = '主机名称必须为 1 到 64 个字符'
    return
  }
  state.actionError = ''
  const instanceId = state.draft.instanceId.trim()
  if (!INSTANCE_ID_PATTERN.test(instanceId)) {
    state.actionError = '实例 ID 必须为正十进制数字'
    Notify.error(state.actionError)
    return
  }
  const credential = state.draft.credential.trim()
  const credentialOmitted = credential === ''
  const revision = state.draftBase.revision
  if (needsHostCredential && revision === null && credentialOmitted) {
    state.actionError = '首次绑定必须填写凭据'
    Notify.error(state.actionError)
    return
  }
  if (needsHostCredential && revision !== null && instanceChanged(state) && credentialOmitted) {
    state.actionError = '更换实例必须填写新凭据'
    Notify.error(state.actionError)
    return
  }
  const payload: ProviderTrafficBindingPayload = {
    enabled: state.draft.enabled,
    instance_id: instanceId,
    revision,
  }
  // 同一实例留空时省略 credential，避免用空串或 null 覆盖已保存凭据。
  if (needsHostCredential && !credentialOmitted) payload.credential = credential
  state.saving = true
  try {
    const data = usesHosts
      ? await saveProviderTrafficHost(provider, state.saved.id ?? '', { ...payload, name, half_remaining: state.draft.halfRemaining })
      : await saveProviderTrafficBinding(provider, payload)
    applySettings(data, { changedKey: key })
    Notify.success('服务器流量设置已保存')
  } catch (err) {
    state.actionError = friendlyError(err)
    Notify.error(state.actionError)
  } finally {
    state.saving = false
  }
}

async function refresh(state: ProviderState) {
  const { provider, id } = state.saved
  const revisionAtStart = state.saved.revision
  if (!revisionAtStart || !canRefresh(state)) return
  state.actionError = ''
  state.refreshing = true
  try {
    const result = id
      ? await refreshProviderTrafficHost(provider, id, revisionAtStart)
      : await refreshProviderTraffic(provider, revisionAtStart)
    // 采集途中若绑定修订已变化，旧结果不得挂到新绑定上。
    if (effectiveAvailable.value && savedEnabled.value && state.saved.enabled && state.saved.revision === revisionAtStart) {
      state.saved = { ...state.saved, result }
    }
  } catch (err) {
    if (effectiveAvailable.value && state.saved.revision === revisionAtStart) {
      state.actionError = friendlyError(err)
      Notify.error(state.actionError)
    }
  } finally {
    state.refreshing = false
  }
}

function requestDelete(key: string, state: ProviderState) {
  const { provider, name, instance_id, revision } = state.saved
  if (!canEdit.value || state.refreshing || !instance_id || !revision) return
  deleteTarget.value = { key, name: `${name || PROVIDER_TRAFFIC_LABELS[provider]}（实例 ${instance_id}）`, revision }
}

async function confirmDelete() {
  if (accountConfirmRevision.value) {
    await saveAccount(true)
    return
  }
  const target = deleteTarget.value
  if (!target || !canEdit.value) return
  const { key, revision } = target
  const state = states[key]
  if (!state || state.refreshing || state.saved.revision !== revision) return
  const { provider, id } = state.saved
  state.deleting = true
  state.actionError = ''
  try {
    const data = id ? await deleteProviderTrafficHost(provider, id, revision) : await deleteProviderTrafficBinding(provider, revision)
    closeConfirmation()
    applySettings(data, { changedKey: key })
    Notify.success('已解除服务器流量绑定')
  } catch (err) {
    state.actionError = friendlyError(err)
    Notify.error(state.actionError)
    closeConfirmation()
  } finally {
    state.deleting = false
  }
}

async function saveAccount(remove = false) {
  if (!canEdit.value || accountConflict.value) return
  const revision = remove ? accountConfirmRevision.value : accountBaseRevision.value
  const credential = accountCredential.value.trim()
  if (remove ? !revision || revision !== account.value.revision || vpsHasHosts.value : !credential) return
  accountSaving.value = true
  accountError.value = ''
  try {
    const data = remove
      ? await deleteProviderTrafficVPSAccount(revision)
      : await saveProviderTrafficVPSAccount(revision, credential)
    closeConfirmation()
    applySettings(data, { savedAccount: true })
    Notify.success(remove ? '已解除 V.PS 账号绑定' : '已保存 V.PS 账号')
  } catch (err) {
    accountError.value = friendlyError(err)
    closeConfirmation()
  } finally {
    accountSaving.value = false
  }
}

</script>

<template>
  <div>
    <Card id="provider-traffic" title="服务器剩余流量" size="small">
      <div class="flex max-w-2xl flex-col gap-3">
        <div v-if="loading" class="text-sm text-text-tertiary">加载中…</div>
        <template v-else-if="loadError">
          <Alert type="error" show-icon :message="loadError" />
          <Button size="small" class="self-start" @click="retryLoad">重试</Button>
        </template>
        <Alert v-else-if="!effectiveAvailable" type="info" show-icon message="仅在基础模式下可用" />
        <template v-else>
          <div class="provider-traffic-total flex flex-wrap items-center gap-3">
            <Switch v-model:checked="enabledDraft" aria-label="启用服务器剩余流量" :disabled="totalSaving"
                    @change="totalError = ''" />
            <span class="text-sm">启用服务器剩余流量</span>
            <Button type="primary" size="small" :loading="totalSaving" :disabled="!canEdit" @click="saveEnabled">保存</Button>
          </div>
          <p class="text-xs text-text-secondary">开启后自动采集，基础模式的登录用户可在首页查看。关闭后停止发起新请求，保留各主机的绑定和开关。</p>
          <Alert v-if="totalError" type="error" show-icon :message="totalError" />

          <template v-for="provider in PROVIDER_TRAFFIC_PROVIDERS" :key="provider">
            <div v-if="supportsHosts" class="flex flex-wrap items-center justify-between gap-2 border-t border-border-subtle pt-3">
              <span class="font-medium">{{ PROVIDER_TRAFFIC_LABELS[provider] }}</span>
              <Button size="small" :disabled="!canEdit || Boolean(states[`new-${provider}`]) || (provider === 'vps' && !account.configured)"
                      @click="addHost(provider)">添加主机</Button>
            </div>

            <div v-if="supportsHosts && provider === 'vps'" class="flex flex-col gap-2 rounded bg-surface-subtle p-3">
              <div class="flex flex-wrap items-center gap-2">
                <span class="text-sm font-medium">V.PS 账号</span>
                <Tag v-if="account.configured" color="success">已保存</Tag>
                <Tag v-else>未配置</Tag>
              </div>
              <p class="text-xs text-text-secondary">同一账号下的主机共用一个 Refresh Token。保存账号后可添加主机。</p>
              <Input.Password v-model:value="accountCredential" aria-label="V.PS 账号 Refresh Token"
                              placeholder="Refresh Token" autocomplete="new-password" :disabled="writing" />
              <div class="flex flex-wrap items-center gap-2">
                <Button type="primary" size="small" :loading="accountSaving && !accountConfirmRevision"
                        :disabled="!canEdit || !accountCredential.trim() || accountConflict" @click="saveAccount()">保存账号</Button>
                <Button v-if="account.configured" danger size="small" :disabled="!canEdit || vpsHasHosts || accountConflict"
                        @click="accountConfirmRevision = account.revision">解除账号绑定</Button>
                <Button v-if="accountConflict" size="small" @click="resetAccountDraft">使用最新账号设置</Button>
              </div>
              <Alert v-if="accountError" type="error" show-icon :message="accountError" />
            </div>

          <div v-for="row in rows.filter((item) => item.provider === provider)" :key="row.key" class="provider-traffic-row space-y-3 border-t border-border-subtle pt-3"
               :data-provider="row.provider" :data-host="row.state.saved.id">
            <div class="flex flex-wrap items-center gap-2">
              <span class="font-medium break-all">{{ supportsHosts ? row.state.saved.name || '新主机' : PROVIDER_TRAFFIC_LABELS[row.provider] }}</span>
              <Switch v-model:checked="row.state.draft.enabled"
                      :aria-label="`采集${PROVIDER_TRAFFIC_LABELS[row.provider]}`"
                      :disabled="writing" @change="row.state.actionError = ''" />
              <span class="text-sm">采集此实例</span>
              <Tag v-if="!row.state.saved.revision" color="default">未绑定</Tag>
              <Tag v-else-if="!row.state.saved.enabled" color="default">已停用</Tag>
              <Tag v-else color="success">已启用</Tag>
            </div>

            <div v-if="supportsHosts" class="flex flex-col gap-1 text-sm sm:flex-row sm:items-center sm:gap-3">
              <span class="text-text-secondary sm:w-24">主机名称</span>
              <Input v-model:value="row.state.draft.name" aria-label="主机名称"
                     placeholder="例如 JP01" :disabled="writing" />
            </div>
            <div v-if="supportsHosts" class="flex items-center gap-2 text-sm">
              <Switch v-model:checked="row.state.draft.halfRemaining" aria-label="剩余流量折半显示" :disabled="writing" />
              <span>剩余流量折半显示</span>
            </div>
            <p v-if="supportsHosts" class="text-xs text-text-tertiary">按服务商剩余流量的 50% 显示，适用于按双向流量计费的主机。</p>

            <div class="flex flex-col gap-1 text-sm sm:flex-row sm:items-center sm:gap-3">
              <span class="text-text-secondary sm:w-24">{{ instanceLabel(row.provider) }}</span>
              <Input v-model:value="row.state.draft.instanceId"
                     :aria-label="`${PROVIDER_TRAFFIC_LABELS[row.provider]} 实例 ID`"
                     inputmode="numeric" :placeholder="instancePlaceholder()" :disabled="writing"
                     @input="row.state.actionError = ''" />
            </div>

            <div v-if="!supportsHosts || row.provider === 'kiwivm'" class="flex flex-col gap-1 text-sm sm:flex-row sm:items-center sm:gap-3">
              <span class="text-text-secondary sm:w-24">{{ credentialLabel(row.provider) }}</span>
              <div class="flex flex-1 flex-col gap-1 sm:flex-row sm:items-center">
                <Input.Password v-model:value="row.state.draft.credential"
                                :aria-label="`${PROVIDER_TRAFFIC_LABELS[row.provider]} 凭据`"
                                autocomplete="new-password"
                                :placeholder="credentialPlaceholder(row.state)"
                                :disabled="writing" @input="row.state.actionError = ''" />
                <Tag v-if="row.state.saved.credential_configured" color="success" class="shrink-0">已保存</Tag>
                <Tag v-else class="shrink-0">未配置</Tag>
              </div>
            </div>
            <p v-if="!supportsHosts || row.provider === 'kiwivm'" class="text-xs text-text-tertiary">{{ credentialHint(row.state) }}</p>

            <div v-if="bindingConflict(row.state)" class="flex flex-col items-start gap-2">
              <Alert type="warning" show-icon message="绑定已在其他页面更改。请使用最新绑定重新编辑，本家草稿将清除。" />
              <Button size="small" :disabled="!canEdit" @click="useLatest(row.key)">使用最新绑定</Button>
            </div>

            <div class="flex flex-wrap items-center gap-2">
              <Button type="primary" size="small" :loading="row.state.saving" :disabled="!canEdit || bindingConflict(row.state)"
                      @click="saveHost(row.key, row.state)">保存</Button>
              <Button size="small" :loading="row.state.refreshing" :disabled="!canRefresh(row.state)"
                      @click="refresh(row.state)">立即采集</Button>
              <Button v-if="row.key.startsWith('new-')" size="small" :disabled="writing" @click="delete states[row.key]">取消添加</Button>
              <Button v-else danger size="small" :loading="row.state.deleting"
                      :disabled="!row.state.saved.revision || !canEdit || row.state.refreshing"
                      @click="requestDelete(row.key, row.state)">解除绑定</Button>
            </div>

            <div class="rounded bg-surface-subtle p-2 text-sm" aria-live="polite">
              <template v-if="row.state.saved.result">
                <template v-if="row.state.saved.result.status === 'ok'">
                  <span class="text-text-secondary">最近结果：</span>
                  剩余 <span class="font-medium">{{ formatRemaining(row.state.saved.result.remaining) }}</span>
                  {{ row.state.saved.result.unit }}
                  <span v-if="row.state.saved.result.half_remaining" class="text-text-tertiary"> · 按 50% 显示</span>
                  <span class="text-text-tertiary"> · 采集于 {{ formatSampledAt(row.state.saved.result.sampled_at) }}</span>
                </template>
                <template v-else-if="row.state.saved.result.status === 'error'">
                  <span class="text-text-secondary">最近结果：</span>
                  <span class="text-danger">采集失败</span>
                  <span class="text-text-secondary"> · {{ row.state.saved.result.message }}</span>
                </template>
                <template v-else>
                  <span class="text-text-tertiary">等待采集</span>
                </template>
              </template>
              <span v-else class="text-text-tertiary">{{ emptyResultText(row.state) }}</span>
            </div>

            <Alert v-if="row.state.saved.result?.status === 'error' && row.state.saved.result.hint"
                   type="warning" show-icon :message="row.state.saved.result.hint" />
            <Alert v-if="row.state.actionError" type="error" show-icon :message="row.state.actionError" />
          </div>
          </template>
        </template>
      </div>
    </Card>

    <ConfirmModal :open="deleteTarget !== null || accountConfirmRevision !== ''"
                  :title="accountConfirmRevision ? '解除 V.PS 账号绑定' : '解除服务器流量绑定'" danger :loading="deleteLoading"
                  :content="deleteContent" @confirm="confirmDelete" @update:open="closeConfirmation" />
  </div>
</template>
