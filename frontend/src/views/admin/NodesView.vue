<!-- NodesView.vue：节点管理页（Design2-UI §6）——manual/xray 双态列表 + 动态表单 -->
<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { Alert, Button, Form, Input, InputNumber, Select, Space, Switch, Table, Tag, Tooltip } from 'ant-design-vue'
import { listNodes, getProtocols, createNode, updateNode, deleteNode, toggleNode, setNodeDisplayName, importNodes, type NodeItem, type ProtocolInfo, type NodeForm, type NodeCheckRequest, type ImportLineResult, type FieldSchema, type CurrentState, type ConditionRule, type ExtensionOp, type ExtensionSummary, type ExtensionInput } from '@/api/node'
import ConfirmModal from '@/components/ConfirmModal.vue'
import FormOverlay from '@/components/FormOverlay.vue'
import FormSection from '@/components/FormSection.vue'
import NodeCheckPanel from '@/components/NodeCheckPanel.vue'
import PageHeader from '@/components/PageHeader.vue'
import ProtocolFieldEditor from '@/components/ProtocolFieldEditor.vue'
import TriStateList from '@/components/TriStateList.vue'
import { Notify } from '@/components/Notify'
import { ApiError } from '@/api/request'

const loading = ref(false)
const nodes = ref<NodeItem[]>([])
const protocols = ref<ProtocolInfo[]>([])
const editing = ref<NodeItem | null>(null)
const creating = ref(false)
const naming = ref<NodeItem | null>(null)
const namingError = ref('')
const displayNameInput = ref('')
const toDelete = ref<NodeItem | null>(null)
const deleting = ref(false)
const disableTarget = ref<NodeItem | null>(null)
const disabling = ref(false)
const publicTarget = ref<NodeItem | null>(null)
const publicChanging = ref(false)
const saving = ref(false)
const conflictError = ref('')
const importOpen = ref(false)
const importText = ref('')
const importResults = ref<ImportLineResult[]>([])
const importing = ref(false)

const form = reactive<NodeForm>({
  name: '', protocol: 'vless', host: '', port: 443, protocol_json: {},
})
const nameReadonly = computed(() => editing.value !== null)
const nameSpaceError = computed(() => {
  if (!creating.value || nameReadonly.value) return ''
  return form.name.includes(' ') ? '名称禁止空格' : ''
})
const invalidProtocolPaths = reactive(new Set<string>())
const resetScopes = new Set<string>()
const clearedSensitivePaths = new Set<string>()
const unappliedJsonPaths = reactive(new Set<string>())
const extensionOps = ref<ExtensionOp[]>([])
const extensionDraft = reactive({
  open: false,
  mode: 'add' as 'add' | 'replace',
  editingId: '',
  scope: 'node',
  targets: '',
  label: '',
  payload: '',
})
const sourceLabel: Record<string, string> = { manual: '手动添加', xray: 'Xray' }

async function load() {
  loading.value = true
  try {
    nodes.value = await listNodes()
    if (protocols.value.length === 0) protocols.value = await getProtocols()
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    loading.value = false
  }
}
onMounted(() => void load())

function currentSchema(): ProtocolInfo | null {
  return protocols.value.find((p) => p.protocol === form.protocol) ?? null
}
function hasSchemaField(name: string): boolean {
  return currentSchema()?.form_schema.some((field) => field.name === name) ?? false
}
function matchesCondition(rule: ConditionRule | undefined, state: CurrentState, target?: string): boolean {
  if (!rule) return true
  if (rule.network && rule.network.length > 0 && !rule.network.includes(state.network ?? '')) return false
  if (rule.security && rule.security.length > 0 && !rule.security.includes(state.security ?? '')) return false
  const plugin = state.plugin ?? ''
  if (rule.plugin && rule.plugin.length > 0 && !rule.plugin.includes(plugin)) return false
  if (rule.features && rule.features.length > 0 && !rule.features.some((item) => (state.features ?? []).includes(item))) return false
  if (target && rule.targets && rule.targets.length > 0 && !rule.targets.includes(target)) return false
  return true
}
const currentState = computed<CurrentState>(() => {
  const params = form.protocol_json as Record<string, any>
  const state: CurrentState = {}
  const network = typeof params.network === 'string' ? params.network
    : typeof params.transport === 'string' ? params.transport
      : hasSchemaField('network') ? 'tcp' : undefined
  if (network) state.network = network
  if (typeof params.security === 'string' && params.security) {
    state.security = params.security
  } else if (params['reality-opts'] && typeof params['reality-opts'] === 'object' && Object.keys(params['reality-opts']).length > 0) {
    state.security = 'reality'
  } else if (params.tls === true) {
    state.security = 'tls'
  } else if (form.protocol === 'trojan') {
    state.security = 'tls'
  } else {
    state.security = 'none'
  }
  const plugin = typeof params.plugin === 'string' && params.plugin ? params.plugin : null
  state.plugin = plugin
  const features: string[] = []
  if (params['udp-over-tcp'] === true) features.push('udp-over-tcp')
  if (params['udp-over-stream'] === true) features.push('udp-over-stream')
  if (params.xudp === true) features.push('xudp')
  if (params.smux && typeof params.smux === 'object' && (params.smux as Record<string, any>).enabled === true) features.push('smux')
  if (typeof params.multiplexing === 'string' && params.multiplexing && params.multiplexing !== 'MULTIPLEXING_OFF') features.push('multiplexing')
  state.features = features
  return state
})
function fieldVisible(field: FieldSchema): boolean {
  return matchesCondition(field.when, currentState.value)
}
function fieldGroup(field: FieldSchema): string {
  if (field.group) return field.group
  switch (field.section) {
    case 'auth': return 'auth'
    case 'transport': return 'connection'
    case 'security': return 'connection'
    case 'switches': return 'switches'
    default: return 'advanced'
  }
}
function groupFields(group: string): FieldSchema[] {
  return currentSchema()?.form_schema.filter((field) => fieldGroup(field) === group && fieldVisible(field)) ?? []
}
function fieldByName(name: string): FieldSchema | undefined {
  return currentSchema()?.form_schema.find((field) => field.name === name)
}
function optionLabel(fieldName: string, value: string): string {
  if (!value) return ''
  const field = fieldByName(fieldName)
  const item = field?.option_items?.find((option) => option.value === value)
  return item?.label ?? value
}
function hasEffectiveValue(value: unknown): boolean {
  if (value === undefined || value === null) return false
  if (typeof value === 'string') return value.trim() !== ''
  if (Array.isArray(value)) return value.length > 0
  if (typeof value === 'object') return Object.keys(value as Record<string, unknown>).length > 0
  return true
}
function configuredAdvancedCount(): number {
  return groupFields('advanced').filter((field) => hasEffectiveValue(fieldValue(field.name))).length
}
const currentCombo = computed(() => {
  const protocolLabel = currentSchema()?.label ?? form.protocol
  const networkLabel = currentState.value.network && (hasSchemaField('network') || hasSchemaField('transport'))
    ? optionLabel('network', currentState.value.network)
    : ''
  const securityLabel = currentState.value.security && (hasSchemaField('security') || form.protocol === 'trojan')
    ? optionLabel('security', currentState.value.security)
    : ''
  return [protocolLabel, networkLabel, securityLabel].filter(Boolean).join(' · ')
})
const extensionRows = computed<Array<{ key: string; id?: string; scope: string; targets: string[]; label: string; op?: ExtensionOp; isNew: boolean }>>(() => {
  const rows: Array<{ key: string; id?: string; scope: string; targets: string[]; label: string; op?: ExtensionOp; isNew: boolean }> = (editing.value?.extensions ?? []).map((ext) => ({
    key: ext.id,
    id: ext.id,
    scope: ext.scope,
    targets: ext.targets ?? [],
    label: ext.label ?? '',
    op: extensionOpFor(ext.id),
    isNew: false,
  }))
  const addCount = extensionOps.value.filter((op) => op.op === 'add').length
  let newIndex = 0
  for (const op of extensionOps.value) {
    if (op.op !== 'add') continue
    rows.push({
      key: `new-${newIndex++}-${addCount}`,
      scope: op.scope ?? '',
      targets: op.targets ?? [],
      label: op.label ?? '',
      op,
      isNew: true,
    })
  }
  return rows
})
const checkRequest = computed<NodeCheckRequest>(() => {
  const credentialOps = Array.from(clearedSensitivePaths).map((path) => ({ path, op: 'clear' as const }))
  const request: NodeCheckRequest = {
    node_id: editing.value?.id,
    base_revision: editing.value?.edit_revision,
    protocol: form.protocol,
    host: form.host,
    port: form.port,
    protocol_json: { ...form.protocol_json },
    current_state: { ...currentState.value },
    reset_scopes: resetScopesArray(),
    credential_ops: credentialOps.length > 0 ? credentialOps : undefined,
    targets: ['clash-yaml', 'sr-subs', 'generic-subs'],
  }
  if (editing.value) {
    if (extensionOps.value.length > 0) request.extension_ops = [...extensionOps.value]
  } else if (extensionOps.value.length > 0) {
    request.extensions = extensionInputsForCreate()
  }
  return request
})
function resetScopesArray(): string[] {
  return [...resetScopes]
}
function extensionInputsForCreate(): ExtensionInput[] {
  return extensionOps.value
    .filter((op) => op.op === 'add')
    .map((op) => ({
      scope: op.scope ?? '',
      targets: op.targets ?? [],
      label: op.label ?? '',
      payload: op.payload ?? '',
    }))
}
function credentialStateForPath(path: string): 'unset' | 'saved' | 'replacing' | 'cleared' {
  if (!editing.value) return 'unset'
  if (clearedSensitivePaths.has(path)) return 'cleared'
  return 'saved'
}
function handleJsonDirty(payload: { path: string; dirty: boolean }) {
  if (payload.dirty) unappliedJsonPaths.add(payload.path)
  else unappliedJsonPaths.delete(payload.path)
}
function warnResetScope(scope: string) {
  const messages: Record<string, string> = {
    protocol: '切换协议将清空当前协议参数与凭据，切回需重新填写',
    network: '切换传输将清空该分区参数，切回需重新填写',
    security: '切换安全方式将清空该分区参数，切回需重新填写',
    plugin: '切换或取消插件将清空插件参数与扩展，切回需重新填写',
  }
  if (scope.startsWith('feature.')) {
    Notify.warning('关闭该功能将清空其子参数与扩展，重新开启不会恢复')
    return
  }
  const message = messages[scope] ?? `切换将清空该分区参数，切回需重新填写`
  Notify.warning(message)
}
function resetExtensionDraft() {
  extensionDraft.open = false
  extensionDraft.mode = 'add'
  extensionDraft.editingId = ''
  extensionDraft.scope = 'node'
  extensionDraft.targets = ''
  extensionDraft.label = ''
  extensionDraft.payload = ''
}
function scopeOptions() {
  const state = currentState.value
  const options = [{ value: 'node', label: 'node（节点公共）' }]
  if (state.network) options.push({ value: `transport.${state.network}`, label: `transport.${state.network}` })
  if (state.security && state.security !== 'none') options.push({ value: `security.${state.security}`, label: `security.${state.security}` })
  if (state.plugin) options.push({ value: `plugin.${state.plugin}`, label: `plugin.${state.plugin}` })
  for (const feature of state.features ?? []) options.push({ value: `feature.${feature}`, label: `feature.${feature}` })
  return options
}
function openExtensionAdd() {
  resetExtensionDraft()
  extensionDraft.open = true
}
function openExtensionReplace(ext: Pick<ExtensionSummary, 'id' | 'scope' | 'targets' | 'label'>) {
  resetExtensionDraft()
  extensionDraft.mode = 'replace'
  extensionDraft.editingId = ext.id
  extensionDraft.scope = ext.scope
  extensionDraft.targets = (ext.targets ?? []).join(', ')
  extensionDraft.label = ext.label ?? ''
  extensionDraft.open = true
}
function removeExtension(ext: Pick<ExtensionSummary, 'id' | 'scope' | 'targets' | 'label'>) {
  extensionOps.value = extensionOps.value.filter((op) => !(op.id === ext.id && op.op !== 'add'))
  extensionOps.value.push({ op: 'clear', id: ext.id })
}
function removePendingAdd(op: ExtensionOp) {
  extensionOps.value = extensionOps.value.filter((item) => item !== op)
}
function commitExtensionDraft() {
  const targets = extensionDraft.targets.split(',').map((item) => item.trim()).filter(Boolean)
  if (!extensionDraft.scope.trim()) {
    Notify.warning('请填写扩展所属范围')
    return
  }
  if (!extensionDraft.payload.trim()) {
    Notify.warning('请填写扩展负载内容')
    return
  }
  if (extensionDraft.mode === 'replace') {
    if (!extensionDraft.editingId) {
      Notify.warning('替换扩展缺少原扩展 ID')
      return
    }
    const op: ExtensionOp = {
      op: 'replace',
      id: extensionDraft.editingId,
      scope: extensionDraft.scope.trim(),
      targets,
      label: extensionDraft.label,
      payload: extensionDraft.payload,
    }
    extensionOps.value = extensionOps.value.filter((item) => !(item.id === op.id && item.op !== 'add'))
    extensionOps.value.push(op)
  } else {
    extensionOps.value.push({
      op: 'add',
      scope: extensionDraft.scope.trim(),
      targets,
      label: extensionDraft.label,
      payload: extensionDraft.payload,
    })
  }
  resetExtensionDraft()
}
function extensionOpFor(id: string): ExtensionOp | undefined {
  return extensionOps.value.find((op) => op.id === id && op.op !== 'add')
}
function scopeResetsExtension(extensionScope: string, resetScope: string): boolean {
  if (resetScope === 'protocol') return true
  if (resetScope === 'network') return extensionScope.startsWith('transport.')
  if (resetScope === 'security') return extensionScope.startsWith('security.')
  if (resetScope === 'plugin') return extensionScope.startsWith('plugin.')
  if (resetScope.startsWith('feature.')) return extensionScope === resetScope
  return false
}
function clearUnappliedJsonForTopNames(topNames: Set<string>) {
  for (const path of Array.from(unappliedJsonPaths)) {
    const top = path.split('.')[0]
    if (topNames.has(top)) unappliedJsonPaths.delete(path)
  }
}
function clearScopedFields(scope: string) {
  const schema = currentSchema()
  if (!schema) return
  const next = { ...form.protocol_json }
  let changed = false
  const topNames = new Set<string>()
  for (const field of schema.form_schema) {
    if (!field.reset_on?.includes(scope)) continue
    topNames.add(field.name)
    if (field.name in next) {
      delete next[field.name]
      changed = true
    }
    for (const path of Array.from(invalidProtocolPaths)) {
      if (path === field.name || path.startsWith(`${field.name}.`) || path.startsWith(`${field.name}[`)) {
        invalidProtocolPaths.delete(path)
      }
    }
  }
  for (const sensitivePath of schema.sensitive_fields ?? []) {
    const top = sensitivePath.split('.')[0]
    if (topNames.has(top)) clearedSensitivePaths.add(sensitivePath)
  }
  clearUnappliedJsonForTopNames(topNames)
  extensionOps.value = extensionOps.value.filter((op) => !scopeResetsExtension(op.scope ?? '', scope))
  if (changed) form.protocol_json = next
}
function applyResetScope(scope: string, changed: boolean) {
  if (!changed) return
  resetScopes.add(scope)
  clearScopedFields(scope)
  warnResetScope(scope)
}
function resetAllEditScopes() {
  resetScopes.clear()
  clearedSensitivePaths.clear()
  unappliedJsonPaths.clear()
  extensionOps.value = []
  resetExtensionDraft()
}
function updateProtocol(protocol: string) {
  if (form.protocol === protocol) return
  warnResetScope('protocol')
  form.protocol = protocol
  form.protocol_json = {}
  invalidProtocolPaths.clear()
  unappliedJsonPaths.clear()
  extensionOps.value = []
  resetExtensionDraft()
  resetScopes.add('protocol')
  clearedSensitivePaths.clear()
  for (const path of currentSchema()?.sensitive_fields ?? []) clearedSensitivePaths.add(path)
}
function openCreate() {
  creating.value = true
  editing.value = null
  resetAllEditScopes()
  conflictError.value = ''
  form.name = ''
  form.protocol = 'vless'
  form.host = ''
  form.port = 443
  form.protocol_json = {}
  invalidProtocolPaths.clear()
}
function openImport() {
  importOpen.value = true
  importText.value = ''
  importResults.value = []
}
async function doImport() {
  if (!importText.value.trim()) {
    Notify.warning('请先粘贴节点 URI')
    return
  }
  importing.value = true
  try {
    const res = await importNodes(importText.value)
    importResults.value = res.list
    await load()
    Notify.success(`导入完成：${res.list.filter((r) => r.ok).length} 成功，${res.list.filter((r) => !r.ok).length} 跳过`)
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    importing.value = false
  }
}

function openEdit(n: NodeItem) {
  if (n.source !== 'manual') return
  editing.value = n
  creating.value = true
  resetAllEditScopes()
  conflictError.value = ''
  form.name = n.name
  form.protocol = n.protocol
  form.host = n.host
  form.port = n.port
  const protocolJson = { ...(n.protocol_json ?? {}) }
  // security 是表单层虚拟字段，数据库只保留 tls/reality-opts/current_state；
  // 编辑时必须从 current_state 回填，避免安全下拉错显示为默认 none。
  if (hasSchemaField('security') && n.current_state?.security) {
    protocolJson.security = n.current_state.security
  }
  if (hasSchemaField('network') && n.current_state?.network) {
    protocolJson.network = n.current_state.network
  }
  if (hasSchemaField('plugin') && n.current_state?.plugin) {
    protocolJson.plugin = n.current_state.plugin
  }
  form.protocol_json = protocolJson
  invalidProtocolPaths.clear()
}
async function save() {
  if (invalidProtocolPaths.size > 0) {
    Notify.warning('请先修正协议对象参数中的格式错误')
    return
  }
  if (unappliedJsonPaths.size > 0) {
    Notify.warning('存在未应用的 JSON 草稿，请先应用或放弃后再保存')
    return
  }
  saving.value = true
  try {
    const credentialOps = Array.from(clearedSensitivePaths).map((path) => ({ path, op: 'clear' as const }))
    const payload: NodeForm = {
      ...form,
      current_state: { ...currentState.value },
      reset_scopes: resetScopesArray(),
      credential_ops: credentialOps.length > 0 ? credentialOps : undefined,
      base_revision: editing.value?.edit_revision,
    }
    if (editing.value) {
      if (extensionOps.value.length > 0) payload.extension_ops = [...extensionOps.value]
    } else if (extensionOps.value.length > 0) {
      payload.extensions = extensionInputsForCreate()
    }
    if (editing.value) {
      await updateNode(editing.value.id, payload)
    } else {
      await createNode(payload)
    }
    Notify.success(editing.value ? '节点已更新' : '节点已创建')
    creating.value = false
    conflictError.value = ''
    resetAllEditScopes()
    await load()
  } catch (err) {
    if (err instanceof ApiError && err.status === 409) {
      conflictError.value = '节点已被其他编辑更新，请重新加载后重试'
      Notify.warning(conflictError.value)
    } else {
      Notify.error((err as Error).message)
    }
  } finally {
    saving.value = false
  }
}

async function reloadAfterConflict() {
  const id = editing.value?.id
  await load()
  const fresh = id ? nodes.value.find((n) => n.id === id) : undefined
  if (fresh) openEdit(fresh)
  conflictError.value = ''
}

async function onToggleEnabled(n: NodeItem, enabled: boolean) {
  if (!enabled) {
    disableTarget.value = n
    return
  }
  try {
    await toggleNode(n.id, { enabled })
    n.enabled = enabled
  } catch (err) {
    Notify.error((err as Error).message)
    await load()
  }
}
async function confirmDisable() {
  if (!disableTarget.value) return
  disabling.value = true
  try {
    await toggleNode(disableTarget.value.id, { enabled: false })
    disableTarget.value.enabled = false
    Notify.success('节点已停用')
    disableTarget.value = null
  } catch (err) {
    Notify.error((err as Error).message)
    await load()
  } finally {
    disabling.value = false
  }
}
async function onTogglePublic(n: NodeItem, isPublic: boolean) {
  publicTarget.value = { ...n, is_public: isPublic }
}
async function confirmPublic() {
  if (!publicTarget.value) return
  publicChanging.value = true
  try {
    const target = publicTarget.value
    await toggleNode(target.id, { is_public: target.is_public })
    const row = nodes.value.find((x) => x.id === target.id)
    if (row) row.is_public = target.is_public
    Notify.success(target.is_public ? '已设为公共节点' : '已取消公共节点')
    publicTarget.value = null
  } catch (err) {
    Notify.error((err as Error).message)
    await load()
  } finally {
    publicChanging.value = false
  }
}
function openNaming(n: NodeItem) {
  naming.value = { ...n }
  displayNameInput.value = n.display_name ?? ''
  namingError.value = ''
}
async function saveDisplayName() {
  if (!naming.value) return
  saving.value = true
  namingError.value = ''
  try {
    await setNodeDisplayName(naming.value.id, displayNameInput.value)
    Notify.success('显示名已更新')
    naming.value = null
    await load()
  } catch (err) {
    if (err instanceof ApiError && err.status === 409) {
      namingError.value = err.message // R14-14：显示名冲突字段级提示
    } else {
      Notify.error((err as Error).message)
    }
  } finally {
    saving.value = false
  }
}
async function confirmDelete() {
  if (!toDelete.value) return
  deleting.value = true
  try {
    await deleteNode(toDelete.value.id)
    Notify.success('节点已删除')
    toDelete.value = null
    await load()
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    deleting.value = false
  }
}

const deleteContent = computed(() => {
  const n = toDelete.value
  if (!n) return ''
  return `将删除节点「${n.render_name}」。装配快照与代理组定义中的引用将按悬空容错处理。\n删除后不可恢复。`
})

function fieldValue(key: string): unknown {
  return form.protocol_json[key]
}
function setField(key: string, val: unknown) {
  const oldValue = form.protocol_json[key]
  const oldNetwork = currentState.value.network
  const oldSecurity = currentState.value.security
  const oldPlugin = currentState.value.plugin
  if (key === 'network' || key === 'transport') {
    applyResetScope('network', val !== oldNetwork)
  } else if (key === 'security') {
    applyResetScope('security', val !== oldSecurity)
  } else if (key === 'plugin') {
    applyResetScope('plugin', val !== oldPlugin)
  } else if ((key === 'udp-over-tcp' || key === 'udp-over-stream' || key === 'xudp') && oldValue === true && val === false) {
    applyResetScope(`feature.${key}`, true)
  }
  form.protocol_json = { ...form.protocol_json, [key]: val }
  if (hasEffectiveValue(val) && (currentSchema()?.sensitive_fields ?? []).includes(key)) {
    clearedSensitivePaths.delete(key)
  }
}
function handleFieldValidity(payload: { path: string; valid: boolean }) {
  if (payload.valid) invalidProtocolPaths.delete(payload.path)
  else invalidProtocolPaths.add(payload.path)
}
</script>

<template>
  <div>
    <PageHeader title="节点管理">
      <template #actions>
        <Button @click="openImport">批量导入</Button>
        <Button type="primary" @click="openCreate">新建节点</Button>
      </template>
    </PageHeader>

    <TriStateList :loading="loading" :empty="nodes.length === 0" empty-text="暂无节点">
      <Table :data-source="nodes" row-key="id" :pagination="false" class="hidden md:block">
        <Table.Column key="render_name" title="节点">
          <template #default="{ record }">
            <div class="font-medium">{{ record.render_name }}</div>
            <div v-if="record.source === 'xray' && record.display_name" class="text-xs text-text-secondary">{{ record.name }}</div>
          </template>
        </Table.Column>
        <Table.Column key="source" title="来源" width="100">
          <template #default="{ record }">
            <Tag :color="record.source === 'manual' ? 'default' : 'purple'">{{ sourceLabel[record.source] ?? record.source }}</Tag>
          </template>
        </Table.Column>
        <Table.Column key="protocol" title="协议" data-index="protocol" width="110" />
        <Table.Column key="addr" title="地址">
          <template #default="{ record }">{{ record.host }}:{{ record.port }}</template>
        </Table.Column>
        <Table.Column key="enabled" title="启用" width="90">
          <template #default="{ record }">
            <Switch :checked="record.enabled" @change="(v: any) => onToggleEnabled(record, v)" />
          </template>
        </Table.Column>
        <Table.Column key="public" title="公共" width="90">
          <template #default="{ record }">
            <Switch v-if="record.source === 'xray' && record.allocatable && !record.missing" :checked="record.is_public" @change="(v: any) => onTogglePublic(record, v)" />
          </template>
        </Table.Column>
        <Table.Column key="actions" title="操作" width="200">
          <template #default="{ record }">
            <Space>
              <Button v-if="record.source === 'manual'" size="small" @click="openEdit(record)">编辑</Button>
              <Button v-if="record.source === 'xray'" size="small" @click="openNaming(record)">命名</Button>
              <Tooltip :title="record.source === 'xray' && !record.missing ? '该入站仍存在于 Xray 实例，请先删除 Xray 入站并刷新节点检测' : ''">
                <Button size="small" danger :disabled="record.source === 'xray' && !record.missing" @click="toDelete = record">删除</Button>
              </Tooltip>
            </Space>
          </template>
        </Table.Column>
      </Table>

      <div class="grid grid-cols-1 gap-3 md:hidden">
        <div v-for="n in nodes" :key="n.id" class="border rounded-lg p-3">
          <div class="flex items-center justify-between">
            <span class="font-medium">{{ n.render_name }}</span>
            <Tag :color="n.source === 'manual' ? 'default' : 'purple'">{{ sourceLabel[n.source] ?? n.source }}</Tag>
          </div>
          <div v-if="n.source === 'xray' && n.display_name" class="text-xs text-text-secondary">{{ n.name }}</div>
          <div class="text-xs text-text-secondary mt-1">{{ n.protocol }} · {{ n.host }}:{{ n.port }}</div>
          <div class="mobile-actions mt-2 flex items-center gap-2 flex-wrap">
            <label class="switch-hit">
              <Switch :checked="n.enabled" size="small" @change="(v: any) => onToggleEnabled(n, v)" />
            </label>
            <Button v-if="n.source === 'manual'" size="small" @click="openEdit(n)">编辑</Button>
            <Button v-if="n.source === 'xray'" size="small" @click="openNaming(n)">命名</Button>
            <Tooltip :title="n.source === 'xray' && !n.missing ? '该入站仍存在于 Xray 实例，请先删除 Xray 入站并刷新节点检测' : ''">
              <Button size="small" danger :disabled="n.source === 'xray' && !n.missing" @click="toDelete = n">删除</Button>
            </Tooltip>
          </div>
        </div>
      </div>
    </TriStateList>

    <FormOverlay :open="creating" :title="editing ? '编辑节点' : '新建节点'" :width="920" :loading="saving" destroy-on-close
                 @submit="save" @update:open="creating = false">
      <Form layout="vertical" class="node-protocol-form space-y-5">
        <Alert v-if="conflictError" type="warning" show-icon class="mb-2" :message="conflictError">
          <template #action>
            <Button size="small" @click="reloadAfterConflict">重新加载</Button>
          </template>
        </Alert>
        <FormSection title="基本信息" help="选择协议并填写节点的稳定名称与连接地址。">
          <div class="grid grid-cols-1 md:grid-cols-2 gap-x-3">
            <Form.Item label="协议" required>
              <AppSelect :value="form.protocol" @change="(value: any) => updateProtocol(String(value))">
                <Select.Option v-for="p in protocols" :key="p.protocol" :value="p.protocol">{{ p.label }}</Select.Option>
              </AppSelect>
            </Form.Item>
            <Form.Item label="名称" required :validate-status="nameSpaceError ? 'error' : undefined" :help="nameSpaceError || undefined">
              <Input v-model:value="form.name" :disabled="nameReadonly" placeholder="禁止空格/逗号，支持中文与 emoji" />
            </Form.Item>
            <Form.Item label="服务器" required>
              <Input v-model:value="form.host" placeholder="域名或 IP" />
            </Form.Item>
            <Form.Item label="端口" required>
              <InputNumber v-model:value="form.port" :min="1" :max="65535" class="w-full" />
            </Form.Item>
          </div>
        </FormSection>

        <div v-if="currentCombo" class="rounded-lg border px-3 py-2 text-sm text-text-secondary">
          当前组合：{{ currentCombo }}
        </div>

        <FormSection v-if="groupFields('auth').length" title="认证与密钥" help="凭据编辑时留空将保留原值；新建时必须填写。">
          <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
            <ProtocolFieldEditor v-for="field in groupFields('auth')" :key="field.name" :field="field"
              :model-value="fieldValue(field.name)" :sensitive-paths="currentSchema()?.sensitive_fields ?? []" :credential-state="credentialStateForPath(field.name)" :current-state="currentState"
              :class="field.type === 'object' ? 'md:col-span-2' : ''"
              @update:model-value="(value: unknown) => setField(field.name, value)" @validity-change="handleFieldValidity" @json-dirty-change="handleJsonDirty" />
          </div>
        </FormSection>

        <FormSection v-if="groupFields('connection').length" title="连接方式与当前参数" help="按当前协议、传输、安全与插件组合动态展示；切换分支时清空旧分支参数。">
          <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
            <ProtocolFieldEditor v-for="field in groupFields('connection')" :key="field.name" :field="field"
              :model-value="fieldValue(field.name)" :sensitive-paths="currentSchema()?.sensitive_fields ?? []" :credential-state="credentialStateForPath(field.name)" :current-state="currentState"
              :class="field.type === 'object' ? 'md:col-span-2' : ''"
              @update:model-value="(value: unknown) => setField(field.name, value)" @validity-change="handleFieldValidity" @json-dirty-change="handleJsonDirty" />
          </div>
        </FormSection>

        <FormSection v-if="groupFields('switches').length" title="独立开关" help="当前组合适用的布尔开关；常用开关直接展示，更多开关默认折叠。">
          <div class="node-switch-fields grid grid-cols-1 md:grid-cols-2 gap-3">
            <ProtocolFieldEditor v-for="field in groupFields('switches').filter((item) => !item.advanced)" :key="field.name" :field="field"
              :model-value="fieldValue(field.name)" :sensitive-paths="currentSchema()?.sensitive_fields ?? []" :credential-state="credentialStateForPath(field.name)" :current-state="currentState"
              @update:model-value="(value: unknown) => setField(field.name, value)" @validity-change="handleFieldValidity" @json-dirty-change="handleJsonDirty" />
          </div>
          <details v-if="groupFields('switches').some((item) => item.advanced)" class="mt-3 rounded-lg border p-3">
            <summary class="cursor-pointer text-sm font-medium">更多开关</summary>
            <div class="grid grid-cols-1 md:grid-cols-2 gap-3 mt-3">
              <ProtocolFieldEditor v-for="field in groupFields('switches').filter((item) => item.advanced)" :key="field.name" :field="field"
                :model-value="fieldValue(field.name)" :sensitive-paths="currentSchema()?.sensitive_fields ?? []" :credential-state="credentialStateForPath(field.name)" :current-state="currentState"
                @update:model-value="(value: unknown) => setField(field.name, value)" @validity-change="handleFieldValidity" @json-dirty-change="handleJsonDirty" />
            </div>
          </details>
        </FormSection>

        <details v-if="groupFields('advanced').length" class="node-advanced-fields rounded-lg border p-3">
          <summary class="cursor-pointer font-medium">更多功能 / 高级参数（已配置 {{ configuredAdvancedCount() }} 项）</summary>
          <p class="text-xs text-text-secondary mt-2">性能、路由、调优与兼容参数；默认折叠。</p>
          <div class="grid grid-cols-1 md:grid-cols-2 gap-3 mt-3">
            <ProtocolFieldEditor v-for="field in groupFields('advanced')" :key="field.name" :field="field"
              :model-value="fieldValue(field.name)" :sensitive-paths="currentSchema()?.sensitive_fields ?? []" :credential-state="credentialStateForPath(field.name)" :current-state="currentState"
              :class="field.type === 'object' ? 'md:col-span-2' : ''"
              @update:model-value="(value: unknown) => setField(field.name, value)" @validity-change="handleFieldValidity" @json-dirty-change="handleJsonDirty" />
          </div>
        </details>

        <details class="node-advanced-data rounded-lg border p-3">
          <summary class="cursor-pointer font-medium">高级数据与目标检查</summary>
          <p class="text-xs text-text-secondary mt-2">未知扩展摘要、局部 JSON 草稿与脱敏目标检查；默认折叠。</p>

          <div class="mt-4 border-t pt-3">
            <div class="flex items-center justify-between gap-3">
              <div>
                <div class="text-sm font-medium">未知扩展</div>
                <div class="text-xs text-text-tertiary">扩展负载加密保存；当前仅保存并用于诊断，不会自动进入任何输出产物。</div>
              </div>
              <Button size="small" @click="openExtensionAdd">新增扩展</Button>
            </div>
            <div v-if="extensionRows.length" class="mt-2 space-y-2">
              <div v-for="row in extensionRows" :key="row.key" class="rounded-md border p-2">
                <div class="flex items-start justify-between gap-2">
                  <div class="min-w-0">
                    <div class="text-sm font-medium">{{ row.label || row.scope }}</div>
                    <div class="text-xs font-mono text-text-tertiary">{{ row.scope }}<span v-if="row.targets.length"> · {{ row.targets.join(', ') }}</span></div>
                    <div class="text-xs text-text-tertiary">{{ row.isNew ? '待新增' : row.op?.op === 'replace' ? '待替换' : row.op?.op === 'clear' ? '待清除' : '已配置' }}</div>
                  </div>
                  <Space>
                    <Button v-if="!row.isNew" size="small" @click="openExtensionReplace({ id: row.id!, scope: row.scope, targets: row.targets, label: row.label })">替换</Button>
                    <Button size="small" danger @click="row.isNew && row.op ? removePendingAdd(row.op) : removeExtension({ id: row.id!, scope: row.scope, targets: row.targets, label: row.label })">删除</Button>
                  </Space>
                </div>
                <div v-if="row.op?.op === 'clear'" class="mt-1 text-xs text-red-500">保存后将从节点中移除该扩展。</div>
                <div v-else-if="row.op?.op === 'replace'" class="mt-1 text-xs text-orange-500">保存后将使用新负载替换该扩展。</div>
              </div>
            </div>
            <div v-else class="text-xs text-text-tertiary mt-2">暂无未知扩展</div>

            <div v-if="extensionDraft.open" class="mt-3 space-y-2 rounded-md border p-3">
              <div class="text-sm font-medium">{{ extensionDraft.mode === 'replace' ? '替换未知扩展' : '新增未知扩展' }}</div>
              <Form.Item label="所属范围" required>
                <AppSelect :value="extensionDraft.scope" @change="(value: any) => extensionDraft.scope = String(value)">
                  <Select.Option v-for="opt in scopeOptions()" :key="opt.value" :value="opt.value">{{ opt.label }}</Select.Option>
                </AppSelect>
              </Form.Item>
              <Form.Item label="目标">
                <Input v-model:value="extensionDraft.targets" placeholder="clash-yaml, sr-subs, generic-subs" />
              </Form.Item>
              <Form.Item label="标签">
                <Input v-model:value="extensionDraft.label" placeholder="可选，例如 WebSocket 未知扩展" />
              </Form.Item>
              <Form.Item label="负载内容" required>
                <Input.TextArea v-model:value="extensionDraft.payload" :rows="4" placeholder="扩展负载将整体加密保存；检查/输出仅返回摘要和诊断。" />
              </Form.Item>
              <div class="flex gap-2">
                <Button type="primary" size="small" @click="commitExtensionDraft">应用扩展</Button>
                <Button size="small" @click="resetExtensionDraft">取消</Button>
              </div>
            </div>
          </div>

          <div class="mt-4 border-t pt-3">
            <NodeCheckPanel :request="checkRequest" @conflict="conflictError = '节点已被其他编辑更新，请重新加载后重试'" />
          </div>
        </details>
      </Form>
    </FormOverlay>

    <FormOverlay :open="naming !== null" title="节点显示名" :loading="saving"
                 @submit="saveDisplayName" @update:open="naming = null">
      <p class="text-sm text-text-secondary">仅 Xray 节点可设置显示名；留空保存将清空并恢复系统名。</p>
      <Alert v-if="namingError" type="error" show-icon class="mb-2" :message="namingError" />
      <Input v-model:value="displayNameInput" placeholder="系统名" />
    </FormOverlay>

    <ConfirmModal :open="disableTarget !== null" title="停用节点" danger :loading="disabling"
                  content="停用该节点将移除受影响用户的 Xray 账号（重新启用后需重新分配）。确定停用吗？"
                  @confirm="confirmDisable" @update:open="disableTarget = null" />

    <ConfirmModal :open="publicTarget !== null" title="切换公共标记" :loading="publicChanging"
                  :content="`确定${publicTarget?.is_public ? '设为' : '取消'}公共节点吗？公共节点将进入候选集供用户分配。`"
                  @confirm="confirmPublic" @update:open="publicTarget = null" />

    <ConfirmModal :open="toDelete !== null" title="删除节点" danger :loading="deleting" :content="deleteContent" @confirm="confirmDelete" @update:open="toDelete = null" />

    <FormOverlay :open="importOpen" title="批量导入节点" :width="760" :loading="importing"
                 @submit="doImport" @update:open="importOpen = false">
      <p class="text-sm text-text-secondary mb-2">支持 ss / vmess / vless / trojan / hysteria2 / hysteria / tuic / wireguard / anytls / http(s) / socks5，也可粘贴 Base64 订阅文本。</p>
      <Input.TextArea v-model:value="importText" :rows="8" placeholder="每行一条节点 URI" />
      <div v-if="importResults.length" class="mt-3">
        <div class="text-sm font-medium mb-2">导入回执（{{ importResults.filter((r) => r.ok).length }} 成功 / {{ importResults.filter((r) => !r.ok).length }} 跳过）</div>
        <Table :data-source="importResults" row-key="(r: any) => r.line + r.raw" :pagination="false" size="small">
          <Table.Column key="line" title="行" data-index="line" width="60" />
          <Table.Column key="name" title="名称" data-index="name" width="160" />
          <Table.Column key="result" title="结果" width="90">
            <template #default="{ record }">
              <Tag :color="record.ok ? 'success' : 'warning'">{{ record.ok ? '成功' : '跳过' }}</Tag>
            </template>
          </Table.Column>
          <Table.Column key="reason" title="说明">
            <template #default="{ record }">{{ record.reason || record.raw }}</template>
          </Table.Column>
        </Table>
      </div>
      <template #footer>
        <Button class="touch-target" @click="importOpen = false">取消</Button>
        <Button type="primary" class="touch-target" :loading="importing" @click="doImport">开始导入</Button>
      </template>
    </FormOverlay>

  </div>
</template>
