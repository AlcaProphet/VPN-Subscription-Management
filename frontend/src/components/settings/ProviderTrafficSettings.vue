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
  getProviderTrafficSettings,
  refreshProviderTraffic,
  saveProviderTrafficBinding,
  saveProviderTrafficEnabled,
  PROVIDER_TRAFFIC_LABELS,
  PROVIDER_TRAFFIC_PROVIDERS,
  type Provider,
  type ProviderTrafficBinding,
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
const deleteTarget = ref<{ provider: Provider; instanceId: string; revision: string } | null>(null)
let loadVersion = 0
let reloadAfterWrite = false

interface ProviderDraft {
  enabled: boolean
  instanceId: string
  credential: string
}

interface ProviderState {
  saved: ProviderTrafficBinding
  draftBase: Pick<ProviderTrafficBinding, 'enabled' | 'instance_id' | 'revision'>
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
    draft: { enabled: false, instanceId: '', credential: '' },
    saving: false,
    deleting: false,
    refreshing: false,
    actionError: '',
  }
}

const states = reactive<Record<Provider, ProviderState>>({
  kiwivm: blankState('kiwivm'),
  vps: blankState('vps'),
})

const rows = computed(() => PROVIDER_TRAFFIC_PROVIDERS.map((provider) => ({
  provider,
  state: states[provider],
})))

// 服务端返回的 available 是权威值；同时兼容设置页内刚刚保存高级模式、系统状态先更新的场景。
const effectiveAvailable = computed(() => available.value && system.status?.advanced_mode !== true)

function providerDirty(provider: Provider): boolean {
  const state = states[provider]
  return state.draft.enabled !== state.draftBase.enabled
    || state.draft.instanceId !== (state.draftBase.instance_id ?? '')
    || state.draft.credential !== ''
}

function bindingConflict(provider: Provider): boolean {
  const state = states[provider]
  return state.draftBase.revision !== state.saved.revision
}

function resetDraft(state: ProviderState) {
  const { enabled, instance_id, revision } = state.saved
  state.draftBase = { enabled, instance_id, revision }
  state.draft = { enabled, instanceId: instance_id ?? '', credential: '' }
  state.actionError = ''
}

const totalDirty = computed(() => enabledDraft.value !== savedEnabled.value)
const dirty = computed(() => totalDirty.value
  || PROVIDER_TRAFFIC_PROVIDERS.some((provider) => providerDirty(provider)))
const writing = computed(() => totalSaving.value
  || PROVIDER_TRAFFIC_PROVIDERS.some((provider) => states[provider].saving || states[provider].deleting))
const canEdit = computed(() => effectiveAvailable.value && !loading.value && !loadError.value && !writing.value)

const deleteLoading = computed(() => deleteTarget.value ? states[deleteTarget.value.provider].deleting : false)
const deleteContent = computed(() => {
  const target = deleteTarget.value
  if (!target) return ''
  return `将解除 ${PROVIDER_TRAFFIC_LABELS[target.provider]} 实例 ${target.instanceId} 的服务器流量绑定，并清除该实例的凭据、开关及最近采集结果。确定继续？`
})

watch(dirty, (value) => {
  if (loaded.value) emit('dirty-change', value)
})

// 基础模式恢复后等待本地写入完成再重读，避免旧读取覆盖写入响应。
watch(() => system.status?.advanced_mode, (advanced, previous) => {
  if (advanced === true) {
    deleteTarget.value = null
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
  changedProvider?: Provider
}

// applySettings 合并服务端快照：只强制更新本次提交的分区，其他脏草稿保持原样。
function applySettings(data: ProviderTrafficSettingsData, options: ApplySettingsOptions = {}) {
  available.value = data.available
  if (!data.available) {
    deleteTarget.value = null
    return
  }
  const wasTotalDirty = totalDirty.value
  const incomingProviders = new Map(data.providers.map((item) => [item.provider, item]))
  savedEnabled.value = data.enabled
  if (options.submittedTotal || !wasTotalDirty) {
    enabledDraft.value = savedEnabled.value
  }
  for (const provider of PROVIDER_TRAFFIC_PROVIDERS) {
    const state = states[provider]
    const incoming = incomingProviders.get(provider) ?? blankBinding(provider)
    const wasDirty = providerDirty(provider)
    state.saved = incoming
    if (options.changedProvider === provider || !wasDirty) resetDraft(state)
  }
  const target = deleteTarget.value
  if (target && states[target.provider].saved.revision !== target.revision) {
    deleteTarget.value = null
    states[target.provider].actionError = '绑定已更改，请重新确认解除绑定'
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

function instanceChanged(provider: Provider): boolean {
  const state = states[provider]
  return state.draft.instanceId.trim() !== (state.draftBase.instance_id ?? '')
}

function credentialPlaceholder(provider: Provider): string {
  const state = states[provider]
  if (state.draft.instanceId.trim() !== '' && instanceChanged(provider)) return '更换实例必须填写新凭据'
  if (state.saved.credential_configured) return '留空保留已保存凭据；输入新值将替换'
  return '首次绑定必须填写凭据'
}

function credentialHint(provider: Provider): string {
  const state = states[provider]
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

function canRefresh(provider: Provider): boolean {
  const state = states[provider]
  return canEdit.value
    && !state.refreshing
    && state.saved.enabled
    && state.saved.instance_id !== null
    && state.saved.revision !== null
    && !totalDirty.value
    && savedEnabled.value
    && !providerDirty(provider)
    && !bindingConflict(provider)
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

async function saveProvider(provider: Provider) {
  const state = states[provider]
  if (!canEdit.value || bindingConflict(provider)) return
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
  if (revision === null && credentialOmitted) {
    state.actionError = '首次绑定必须填写凭据'
    Notify.error(state.actionError)
    return
  }
  if (revision !== null && instanceId !== (state.draftBase.instance_id ?? '') && credentialOmitted) {
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
  if (!credentialOmitted) payload.credential = credential
  state.saving = true
  try {
    const data = await saveProviderTrafficBinding(provider, payload)
    applySettings(data, { changedProvider: provider })
    Notify.success(`${PROVIDER_TRAFFIC_LABELS[provider]}设置已保存`)
  } catch (err) {
    state.actionError = friendlyError(err)
    Notify.error(state.actionError)
  } finally {
    state.saving = false
  }
}

async function refresh(provider: Provider) {
  const state = states[provider]
  const revisionAtStart = state.saved.revision
  if (!revisionAtStart || !canRefresh(provider)) return
  state.actionError = ''
  state.refreshing = true
  try {
    const result = await refreshProviderTraffic(provider, revisionAtStart)
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

function requestDelete(provider: Provider) {
  const state = states[provider]
  const { instance_id, revision } = state.saved
  if (!canEdit.value || state.refreshing || !instance_id || !revision) return
  deleteTarget.value = { provider, instanceId: instance_id, revision }
}

async function confirmDelete() {
  const target = deleteTarget.value
  if (!target || !canEdit.value) return
  const { provider, revision } = target
  const state = states[provider]
  if (state.refreshing || state.saved.revision !== revision) return
  state.deleting = true
  state.actionError = ''
  try {
    const data = await deleteProviderTrafficBinding(provider, revision)
    deleteTarget.value = null
    applySettings(data, { changedProvider: provider })
    Notify.success(`已解除${PROVIDER_TRAFFIC_LABELS[provider]}绑定`)
  } catch (err) {
    state.actionError = friendlyError(err)
    Notify.error(state.actionError)
    deleteTarget.value = null
  } finally {
    state.deleting = false
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
          <p class="text-xs text-text-secondary">开启后自动采集，基础模式的登录用户可在首页查看。关闭后停止发起新请求，保留两家绑定及各自开关。</p>
          <Alert v-if="totalError" type="error" show-icon :message="totalError" />

          <div v-for="row in rows" :key="row.provider" class="provider-traffic-row space-y-3 border-t border-border-subtle pt-3"
               :data-provider="row.provider">
            <div class="flex flex-wrap items-center gap-2">
              <span class="font-medium">{{ PROVIDER_TRAFFIC_LABELS[row.provider] }}</span>
              <Switch v-model:checked="row.state.draft.enabled"
                      :aria-label="`采集${PROVIDER_TRAFFIC_LABELS[row.provider]}`"
                      :disabled="writing" @change="row.state.actionError = ''" />
              <span class="text-sm">采集此实例</span>
              <Tag v-if="!row.state.saved.revision" color="default">未绑定</Tag>
              <Tag v-else-if="!row.state.saved.enabled" color="default">已停用</Tag>
              <Tag v-else color="success">已启用</Tag>
            </div>

            <div class="flex flex-col gap-1 text-sm sm:flex-row sm:items-center sm:gap-3">
              <span class="text-text-secondary sm:w-24">{{ instanceLabel(row.provider) }}</span>
              <Input v-model:value="row.state.draft.instanceId"
                     :aria-label="`${PROVIDER_TRAFFIC_LABELS[row.provider]} 实例 ID`"
                     inputmode="numeric" :placeholder="instancePlaceholder()" :disabled="writing"
                     @input="row.state.actionError = ''" />
            </div>

            <div class="flex flex-col gap-1 text-sm sm:flex-row sm:items-center sm:gap-3">
              <span class="text-text-secondary sm:w-24">{{ credentialLabel(row.provider) }}</span>
              <div class="flex flex-1 flex-col gap-1 sm:flex-row sm:items-center">
                <Input.Password v-model:value="row.state.draft.credential"
                                :aria-label="`${PROVIDER_TRAFFIC_LABELS[row.provider]} 凭据`"
                                autocomplete="new-password"
                                :placeholder="credentialPlaceholder(row.provider)"
                                :disabled="writing" @input="row.state.actionError = ''" />
                <Tag v-if="row.state.saved.credential_configured" color="success" class="shrink-0">已保存</Tag>
                <Tag v-else class="shrink-0">未配置</Tag>
              </div>
            </div>
            <p class="text-xs text-text-tertiary">{{ credentialHint(row.provider) }}</p>

            <div v-if="bindingConflict(row.provider)" class="flex flex-col items-start gap-2">
              <Alert type="warning" show-icon message="绑定已在其他页面更改。请使用最新绑定重新编辑，本家草稿将清除。" />
              <Button size="small" :disabled="!canEdit" @click="resetDraft(row.state)">使用最新绑定</Button>
            </div>

            <div class="flex flex-wrap items-center gap-2">
              <Button type="primary" size="small" :loading="row.state.saving" :disabled="!canEdit || bindingConflict(row.provider)"
                      @click="saveProvider(row.provider)">保存</Button>
              <Button size="small" :loading="row.state.refreshing" :disabled="!canRefresh(row.provider)"
                      @click="refresh(row.provider)">立即采集</Button>
              <Button danger size="small" :loading="row.state.deleting"
                      :disabled="!row.state.saved.revision || !canEdit || row.state.refreshing"
                      @click="requestDelete(row.provider)">解除绑定</Button>
            </div>

            <div class="rounded bg-surface-subtle p-2 text-sm" aria-live="polite">
              <template v-if="row.state.saved.result">
                <template v-if="row.state.saved.result.status === 'ok'">
                  <span class="text-text-secondary">最近结果：</span>
                  剩余 <span class="font-medium">{{ formatRemaining(row.state.saved.result.remaining) }}</span>
                  {{ row.state.saved.result.unit }}
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
      </div>
    </Card>

    <ConfirmModal :open="deleteTarget !== null" title="解除服务器流量绑定" danger :loading="deleteLoading"
                  :content="deleteContent" @confirm="confirmDelete" @update:open="deleteTarget = null" />
  </div>
</template>
