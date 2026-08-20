<!-- AssemblyView.vue：订阅装配（Design2-UI §5）——四类装配器 + 预览 diff + 重新编辑 -->
<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Alert, Button, Card, Checkbox, Input, Modal, Radio, Result, Segmented, Select, Space, Steps, Tabs, Tag, message } from 'ant-design-vue'
import PageHeader from '@/components/PageHeader.vue'
import PoolTab from './assembly/PoolTab.vue'
import DiffView from '@/components/DiffView.vue'
import {
  getAssemblyContext, previewAssembly, generateAssembly, getBlueprint,
  type AssemblyContext, type GenerateInput, type TargetSyntax, type PoolSelection, type RuleLine,
} from '@/api/assembly'
import { Notify } from '@/components/Notify'
import { listSubscriptions } from '@/api/subscription'
import { versionApi } from '@/api/version'

const route = useRoute()
const router = useRouter()
const TAB_KEYS = ['pool', 'clash-yaml', 'sr-subs', 'generic-subs', 'sr-conf'] as const
function normalizeTab(v: unknown): string {
  const s = String(v ?? 'pool')
  return (TAB_KEYS as readonly string[]).includes(s) ? s : 'pool'
}
const activeTab = ref<string>(normalizeTab(route.query.tab))
watch(() => route.query.tab, () => { activeTab.value = normalizeTab(route.query.tab) })
function onTabChange(key: string | number) {
  void router.replace({ query: { ...route.query, tab: String(key) } })
}

const RULE_TYPES = ['DOMAIN', 'DOMAIN-SUFFIX', 'DOMAIN-KEYWORD', 'IP-CIDR', 'IP-CIDR6', 'PROCESS-NAME', 'PROCESS-NAME-REGEX', 'USER-AGENT']
const CLASH_RULE_TYPES = RULE_TYPES.filter((t) => t !== 'USER-AGENT')
const FORCE_GROUPS = ['🚀直接连接', '🌎国外流量', '🛟无法归属的流量']
const DEFAULT_HEADERS: Record<TargetSyntax, string> = {
  'clash-yaml': JSON.stringify({ port: 7890, mode: 'rule', 'log-level': 'info', 'allow-lan': false, 'external-controller': '127.0.0.1:9090' }, null, 2),
  'sr-subs': JSON.stringify({ status: '2026/01/01 Version', remarks: 'VPN Subscription' }, null, 2),
  'generic-subs': '{}',
  'sr-conf': JSON.stringify({ loglevel: 'warning' }, null, 2),
}

const context = ref<AssemblyContext | null>(null)
const loadingContext = ref(false)
const previewing = ref(false)
const generating = ref(false)
const previewText = ref('')
const previewSkipped = ref<any[]>([])
const previewWarnings = ref<string[]>([])
const showDiff = ref(false)
const diffOld = ref('')
const diffMissing = ref(false)
const editVersionId = ref<number | null>(null)
const invalidRefs = ref<Array<{ kind: string; name: string }>>([])
const nameChanged = ref<Record<string, string>>({})
const layoutMode = ref<'step' | 'page'>(localStorage.getItem('assembly_layout_mode') === 'page' ? 'page' : 'step')
const currentStep = ref(0)
const headerConfirmOpen = ref(false)
const diffLoading = ref(false)
const generateResult = ref<{ version_id: number; version_no: number; auto_activated: boolean; skipped: any[]; warnings: string[] } | null>(null)

const form = reactive({
  platform_id: undefined as number | undefined,
  rule_id: undefined as number | undefined,
  fixed_params_text: '{}',
  node_names: [] as string[],
  group_names: [] as string[],
  overseas_members: [] as string[],
  pools: [] as PoolSelection[],
  custom_rules: [] as RuleLine[],
  final_direction: 'PROXY',
})

const targetSyntax = computed<TargetSyntax>(() => {
  const t = activeTab.value
  return (['clash-yaml', 'sr-subs', 'generic-subs', 'sr-conf'] as TargetSyntax[]).includes(t as TargetSyntax)
    ? (t as TargetSyntax)
    : 'clash-yaml'
})
const isSrConf = computed(() => targetSyntax.value === 'sr-conf')
const filteredPlatforms = computed(() => {
  if (!context.value) return []
  const map: Record<TargetSyntax, string> = { 'clash-yaml': 'yaml', 'sr-subs': 'subs', 'generic-subs': 'generic-subs', 'sr-conf': '' }
  const want = map[targetSyntax.value]
  return context.value.platforms.filter((p) => !want || p.product_type === want)
})

watch(layoutMode, (v) => localStorage.setItem('assembly_layout_mode', v))
watch(targetSyntax, () => { currentStep.value = 0 })

const stepDefs = computed<Array<{ key: string; title: string }>>(() => {
  if (targetSyntax.value === 'clash-yaml') {
    return [
      { key: 'target', title: '类型与目标' },
      { key: 'header', title: '头部表单' },
      { key: 'nodes', title: '节点与代理组' },
      { key: 'rules', title: '规则素材' },
      { key: 'preview', title: '预览' },
      { key: 'generate', title: '确认生成' },
    ]
  }
  if (targetSyntax.value === 'sr-subs' || targetSyntax.value === 'generic-subs') {
    return [
      { key: 'target', title: '类型与目标' },
      { key: 'header', title: '头部表单' },
      { key: 'nodes', title: '节点勾选' },
      { key: 'preview', title: '预览' },
      { key: 'generate', title: '确认生成' },
    ]
  }
  return [
    { key: 'target', title: '类型与目标' },
    { key: 'header', title: '头部表单' },
    { key: 'rules', title: '规则素材' },
    { key: 'preview', title: '预览' },
    { key: 'generate', title: '确认生成' },
  ]
})
const currentStepKey = computed(() => stepDefs.value[currentStep.value]?.key ?? 'target')
const hasHeaderStep = computed(() => stepDefs.value.some((s) => s.key === 'header'))
const hasNodesStep = computed(() => stepDefs.value.some((s) => s.key === 'nodes'))
const hasRulesStep = computed(() => stepDefs.value.some((s) => s.key === 'rules'))
const manualNodes = computed(() => (context.value?.nodes ?? []).filter((n) => n.source === 'manual' && !n.missing))
const xrayNodes = computed(() => (context.value?.nodes ?? []).filter((n) => n.source === 'xray' && !n.missing))
const presetGroups = computed(() => context.value?.proxy_groups.filter((g) => g.type === 'preset') ?? [])
const customGroups = computed(() => context.value?.proxy_groups.filter((g) => g.type === 'custom') ?? [])
const ruleTypeOptions = computed(() => targetSyntax.value === 'clash-yaml' ? CLASH_RULE_TYPES : RULE_TYPES)

async function loadContext() {
  loadingContext.value = true
  try {
    context.value = await getAssemblyContext()
    // 支持从订阅/规则页带目标参数进入装配
    const platformId = Number(route.query.platform_id ?? 0)
    if (platformId > 0) form.platform_id = platformId
    const ruleId = Number(route.query.rule_id ?? 0)
    if (ruleId > 0) form.rule_id = ruleId
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    loadingContext.value = false
  }
}
onMounted(() => {
  void loadContext()
  void loadEditIfAny()
})

async function loadEditIfAny() {
  const id = Number(route.query.edit_version_id ?? 0)
  if (!id) return
  editVersionId.value = id
  try {
    const data = await getBlueprint(id)
    const bp = data.blueprint
    activeTab.value = bp.target_syntax
    form.platform_id = bp.platform_id ?? undefined
    form.rule_id = bp.rule_id ?? undefined
    form.node_names = bp.selection?.node_names ?? []
    form.group_names = bp.selection?.group_names ?? []
    form.overseas_members = bp.selection?.overseas_members ?? []
    form.pools = bp.selection?.pools ?? []
    form.final_direction = bp.selection?.final_direction ?? 'PROXY'
    form.fixed_params_text = JSON.stringify(bp.fixed_params ?? {}, null, 2)
    form.custom_rules = bp.custom_rules ?? []
    invalidRefs.value = data.invalid_refs ?? []
    nameChanged.value = data.name_changed ?? {}
    Notify.info('正在重新编辑版本，请检查失效引用')
  } catch (err) {
    Notify.error((err as Error).message)
  }
}

function parseFixedParams(): Record<string, unknown> {
  try {
    const v = JSON.parse(form.fixed_params_text || '{}')
    return v && typeof v === 'object' ? v : {}
  } catch {
    message.warning('头部 JSON 格式错误')
    return {}
  }
}
function parseCustomRules(): RuleLine[] {
  return form.custom_rules.map((r) => ({
    rule_type: r.rule_type,
    match_value: r.match_value.trim(),
    target: r.target.trim(),
  }))
}
function buildInput(): GenerateInput {
  return {
    target_syntax: targetSyntax.value,
    platform_id: isSrConf.value ? undefined : form.platform_id,
    rule_id: isSrConf.value ? form.rule_id : undefined,
    fixed_params: parseFixedParams(),
    node_names: form.node_names,
    group_names: form.group_names,
    overseas_members: form.overseas_members,
    pools: form.pools,
    custom_rules: parseCustomRules(),
    final_direction: form.final_direction,
  }
}

async function doPreview() {
  previewing.value = true
  try {
    const res = await previewAssembly(buildInput())
    previewText.value = res.content
    previewSkipped.value = res.skipped
    previewWarnings.value = res.warnings
    showDiff.value = false
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    previewing.value = false
  }
}
function toggleNode(name: string) {
  form.node_names = form.node_names.includes(name)
    ? form.node_names.filter((n) => n !== name)
    : [...form.node_names, name]
}
function toggleGroup(name: string) {
  form.group_names = form.group_names.includes(name)
    ? form.group_names.filter((n) => n !== name)
    : [...form.group_names, name]
}
function toggleOverseas(name: string) {
  form.overseas_members = form.overseas_members.includes(name)
    ? form.overseas_members.filter((n) => n !== name)
    : [...form.overseas_members, name]
}
function addPool() {
  form.pools.push({ pool_id: context.value?.pools?.[0]?.id ?? 0, target: outputGroups.value[0] ?? '' })
}
function movePool(from: number, to: number) {
  if (from < 0 || from >= form.pools.length || to < 0 || to >= form.pools.length || from === to) return
  const next = [...form.pools]
  const [item] = next.splice(from, 1)
  next.splice(to, 0, item)
  form.pools = next
}
function addRule() {
  form.custom_rules.push({ rule_type: targetSyntax.value === 'clash-yaml' ? 'DOMAIN-SUFFIX' : 'DOMAIN-SUFFIX', match_value: '', target: '' })
}
function removeRule(idx: number) {
  form.custom_rules.splice(idx, 1)
}

function applyDefaultHeader() {
  form.fixed_params_text = DEFAULT_HEADERS[targetSyntax.value]
  headerConfirmOpen.value = false
  Notify.success('已采用默认头部')
}

function targetReady(): boolean {
  return isSrConf.value ? !!form.rule_id : !!form.platform_id
}
function prevStep() {
  if (currentStep.value > 0) currentStep.value -= 1
}
function nextStep() {
  if (currentStepKey.value === 'target' && !targetReady()) {
    Notify.warning('请先选择目标')
    return
  }
  if (currentStepKey.value === 'nodes' && targetSyntax.value === 'clash-yaml' && form.overseas_members.length === 0) {
    Notify.warning('「🌎国外流量」组未包含任何节点')
    return
  }
  if (currentStep.value < stepDefs.value.length - 1) currentStep.value += 1
}

async function fetchCurrentActive(): Promise<{ text: string; missing: boolean } | null> {
  if (isSrConf.value) {
    if (!form.rule_id) { Notify.warning('请先选择规则实体'); return null }
    const versions = await versionApi('/admin/rules').list(form.rule_id)
    const current = versions.find((v) => v.current)
    if (!current) return { text: '', missing: true }
    const text = await versionApi('/admin/rules').preview(form.rule_id, current.version_no)
    return { text, missing: false }
  }
  if (!form.platform_id) { Notify.warning('请先选择目标平台'); return null }
  const subs = await listSubscriptions()
  const sub = subs.find((s) => s.platform_id === form.platform_id)
  if (!sub) return { text: '', missing: true }
  const versions = await versionApi('/admin/subscriptions').list(sub.id)
  const current = versions.find((v) => v.current)
  if (!current) return { text: '', missing: true }
  const text = await versionApi('/admin/subscriptions').preview(sub.id, current.version_no)
  return { text, missing: false }
}
async function toggleDiff() {
  if (!previewText.value) { Notify.warning('请先预览产物'); return }
  if (!showDiff.value) {
    diffLoading.value = true
    try {
      const data = await fetchCurrentActive()
      if (!data) return
      diffOld.value = data.text
      diffMissing.value = data.missing
      showDiff.value = true
    } catch (err) {
      Notify.error((err as Error).message)
    } finally {
      diffLoading.value = false
    }
  } else {
    showDiff.value = false
  }
}

function invalidLabel(kind: string) {
  return kind === 'node' ? '节点' : kind === 'group' ? '代理组' : '素材池'
}
function removeInvalidRef(ref: { kind: string; name: string }) {
  if (ref.kind === 'node') form.node_names = form.node_names.filter((n) => n !== ref.name)
  if (ref.kind === 'group') form.group_names = form.group_names.filter((n) => n !== ref.name)
  if (ref.kind === 'pool') form.pools = form.pools.filter((p) => String(p.pool_id) !== ref.name)
}
function removeAllInvalidRefs() {
  invalidRefs.value.forEach(removeInvalidRef)
  invalidRefs.value = []
}

async function doGenerate() {
  if (invalidRefs.value.length > 0) {
    Notify.error('存在失效引用，请先剔除后生成')
    return
  }
  if (!targetReady()) {
    Notify.error('请先选择目标')
    return
  }
  if (targetSyntax.value === 'clash-yaml' && form.overseas_members.length === 0) {
    Notify.error('「🌎国外流量」组未包含任何节点，空组将导致 Clash 加载失败')
    return
  }
  generating.value = true
  try {
    const res = await generateAssembly(buildInput())
    generateResult.value = res
    Notify.success(res.auto_activated ? '首个版本已自动激活' : '已入池未生效，请激活')
    editVersionId.value = null
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    generating.value = false
  }
}
function continueAssembly() {
  generateResult.value = null
  currentStep.value = 0
}

const outputGroups = computed(() => {
  const set = new Set<string>(['🚀直接连接', '🌎国外流量', '🛟无法归属的流量'])
  form.group_names.forEach((g) => set.add(g))
  return Array.from(set)
})
</script>

<template>
  <div>
    <PageHeader title="订阅装配" subtitle="四类装配器：选择目标 → 填头部 → 勾选节点/组 → 规则 → 预览 → 生成" />
    <Alert v-if="editVersionId" type="info" show-icon class="mb-4" message="正在重新编辑版本，请检查失效引用后生成新版本" />
    <Alert v-if="invalidRefs.length" type="error" show-icon class="mb-2"
           :message="`${invalidRefs.length} 项引用已失效，请剔除或替换后生成`">
      <template #action>
        <Button size="small" danger @click="removeAllInvalidRefs">一键剔除全部失效项</Button>
      </template>
    </Alert>
    <Alert v-for="ref in invalidRefs" :key="`${ref.kind}-${ref.name}`" type="error" show-icon class="mb-2"
           :message="`失效${invalidLabel(ref.kind)}：${ref.name}`" />
    <Alert v-if="Object.keys(nameChanged).length" type="warning" show-icon class="mb-2"
           message="以下节点显示名已变化，生成时将按当前显示名渲染" :description="Object.entries(nameChanged).map(([k,v]) => `${k} → ${v}`).join('；')" />
    <Tabs v-model:activeKey="activeTab" @change="onTabChange">
      <Tabs.TabPane key="pool" tab="规则素材池">
        <PoolTab />
      </Tabs.TabPane>
      <Tabs.TabPane v-for="tab in (['clash-yaml','sr-subs','generic-subs','sr-conf'] as const)" :key="tab" :tab="tab">
        <div v-if="loadingContext" class="py-12 text-center text-gray-400">加载装配上下文中…</div>
        <div v-else class="space-y-4">
          <div class="flex items-center justify-between flex-wrap gap-2">
            <Segmented v-model:value="layoutMode" :options="[{ label: '分步', value: 'step' }, { label: '单页', value: 'page' }]" />
            <span class="text-xs text-gray-400">四类装配器共用同一表单状态，切换不丢失</span>
          </div>

          <Steps v-if="layoutMode === 'step'" :current="currentStep" size="small" class="mb-4">
            <Steps.Step v-for="s in stepDefs" :key="s.key" :title="s.title" />
          </Steps>

          <!-- ① 类型与目标 -->
          <div v-show="layoutMode === 'page' || currentStepKey === 'target'">
            <Card v-if="layoutMode === 'page'" title="① 类型与目标" size="small" class="mb-3">
              <div class="grid md:grid-cols-2 gap-4">
                <div>
                  <div class="text-sm mb-1">{{ isSrConf ? '规则实体' : '目标平台' }}</div>
                  <Select v-if="isSrConf" v-model:value="form.rule_id" placeholder="选择规则实体" class="w-full">
                    <Select.Option v-for="r in context?.rules ?? []" :key="r.id" :value="r.id">{{ r.name }}<span v-if="r.current_version <= 0" class="text-xs text-gray-400">（空实体）</span></Select.Option>
                  </Select>
                  <Select v-else v-model:value="form.platform_id" placeholder="选择平台" class="w-full">
                    <Select.Option v-for="p in filteredPlatforms" :key="p.id" :value="p.id">{{ p.name }}（{{ p.product_type }}）</Select.Option>
                  </Select>
                </div>
                <div v-if="isSrConf">
                  <div class="text-sm mb-1">FINAL 方向</div>
                  <Radio.Group v-model:value="form.final_direction" class="w-full">
                    <Radio value="PROXY">PROXY</Radio>
                    <Radio value="DIRECT">DIRECT</Radio>
                  </Radio.Group>
                </div>
              </div>
            </Card>
            <div v-else>
              <div class="grid md:grid-cols-2 gap-4">
                <div>
                  <div class="text-sm mb-1">{{ isSrConf ? '规则实体' : '目标平台' }}</div>
                  <Select v-if="isSrConf" v-model:value="form.rule_id" placeholder="选择规则实体" class="w-full">
                    <Select.Option v-for="r in context?.rules ?? []" :key="r.id" :value="r.id">{{ r.name }}<span v-if="r.current_version <= 0" class="text-xs text-gray-400">（空实体）</span></Select.Option>
                  </Select>
                  <Select v-else v-model:value="form.platform_id" placeholder="选择平台" class="w-full">
                    <Select.Option v-for="p in filteredPlatforms" :key="p.id" :value="p.id">{{ p.name }}（{{ p.product_type }}）</Select.Option>
                  </Select>
                </div>
                <div v-if="isSrConf">
                  <div class="text-sm mb-1">FINAL 方向</div>
                  <Radio.Group v-model:value="form.final_direction" class="w-full">
                    <Radio value="PROXY">PROXY</Radio>
                    <Radio value="DIRECT">DIRECT</Radio>
                  </Radio.Group>
                </div>
              </div>
            </div>
          </div>

          <!-- ② 头部表单 -->
          <div v-if="hasHeaderStep" v-show="layoutMode === 'page' || currentStepKey === 'header'">
            <Card v-if="layoutMode === 'page'" title="② 头部表单" size="small" class="mb-3">
              <div class="space-y-2">
                <div v-if="targetSyntax === 'generic-subs'" class="text-sm text-gray-400">通用节点订阅不输出 STATUS/REMARKS 头部，本步无需填写。</div>
                <div v-else>
                  <div class="flex items-center justify-between mb-1">
                    <span class="text-sm">头部参数（JSON）</span>
                    <Button size="small" @click="headerConfirmOpen = true">一键采用默认值</Button>
                  </div>
                  <Input.TextArea v-model:value="form.fixed_params_text" :rows="4" placeholder='{"port":7890,"mode":"rule"}' />
                </div>
              </div>
            </Card>
            <div v-else>
              <div class="space-y-2">
                <div v-if="targetSyntax === 'generic-subs'" class="text-sm text-gray-400">通用节点订阅不输出 STATUS/REMARKS 头部，本步无需填写。</div>
                <div v-else>
                  <div class="flex items-center justify-between mb-1">
                    <span class="text-sm">头部参数（JSON）</span>
                    <Button size="small" @click="headerConfirmOpen = true">一键采用默认值</Button>
                  </div>
                  <Input.TextArea v-model:value="form.fixed_params_text" :rows="4" placeholder='{"port":7890,"mode":"rule"}' />
                </div>
              </div>
            </div>
          </div>

          <!-- ③ 节点与代理组 -->
          <div v-if="hasNodesStep" v-show="layoutMode === 'page' || currentStepKey === 'nodes'">
            <Card v-if="layoutMode === 'page'" title="③ 节点与代理组" size="small" class="mb-3">
              <div class="space-y-3">
                <div>
                  <div class="text-sm font-medium mb-1">manual 节点</div>
                  <div class="grid md:grid-cols-3 gap-2">
                    <Checkbox v-for="n in manualNodes" :key="n.name" :checked="form.node_names.includes(n.name)" @change="toggleNode(n.name)">
                      <span>{{ n.render_name }}</span><Tag class="ml-1">{{ n.protocol }}</Tag>
                      <Tag v-if="invalidRefs.some((r) => r.kind === 'node' && r.name === n.name)" color="red">已失效</Tag>
                    </Checkbox>
                    <div v-if="manualNodes.length === 0" class="text-xs text-gray-400">暂无 manual 节点</div>
                  </div>
                </div>
                <div>
                  <div class="text-sm font-medium mb-1">xray 节点</div>
                  <div class="grid md:grid-cols-3 gap-2">
                    <Checkbox v-for="n in xrayNodes" :key="n.name" :checked="form.node_names.includes(n.name)"
                              :disabled="!n.allocatable || n.enabled === false" @change="toggleNode(n.name)">
                      <span>{{ n.render_name }}</span>
                      <span v-if="n.display_name" class="block text-xs text-gray-400 font-mono">{{ n.name }}</span>
                      <Tag v-if="!n.allocatable || n.enabled === false" class="ml-1">不可用</Tag>
                    </Checkbox>
                    <div v-if="xrayNodes.length === 0" class="text-xs text-gray-400">未检测到 Xray 节点（高级模式录入实例后刷新节点发现）</div>
                  </div>
                </div>
                <div v-if="targetSyntax === 'clash-yaml'">
                  <div class="text-sm font-medium mb-1">代理组</div>
                  <div class="grid md:grid-cols-3 gap-2">
                    <Checkbox v-for="g in FORCE_GROUPS" :key="g" :checked="true" disabled>{{ g }}<Tag class="ml-1">强制</Tag></Checkbox>
                    <Checkbox v-for="g in presetGroups" :key="g.name" :checked="form.group_names.includes(g.name)" :disabled="!g.enabled" @change="toggleGroup(g.name)">
                      {{ g.name }}<Tag class="ml-1">preset</Tag>
                    </Checkbox>
                    <Checkbox v-for="g in customGroups" :key="g.name" :checked="form.group_names.includes(g.name)" @change="toggleGroup(g.name)">
                      {{ g.name }}<Tag class="ml-1">自建</Tag>
                    </Checkbox>
                  </div>
                </div>
                <div v-if="targetSyntax === 'clash-yaml'">
                  <div class="text-sm font-medium mb-1">🌎国外流量成员（仅节点）</div>
                  <div class="grid md:grid-cols-3 gap-2">
                    <Checkbox v-for="n in manualNodes.concat(xrayNodes)" :key="n.name" :checked="form.overseas_members.includes(n.name)"
                              :disabled="n.source === 'xray' && (!n.allocatable || n.enabled === false)" @change="toggleOverseas(n.name)">
                      {{ n.render_name }}
                    </Checkbox>
                  </div>
                </div>
              </div>
            </Card>
            <div v-else>
              <div class="space-y-3">
                <div>
                  <div class="text-sm font-medium mb-1">manual 节点</div>
                  <div class="grid md:grid-cols-3 gap-2">
                    <Checkbox v-for="n in manualNodes" :key="n.name" :checked="form.node_names.includes(n.name)" @change="toggleNode(n.name)">
                      <span>{{ n.render_name }}</span><Tag class="ml-1">{{ n.protocol }}</Tag>
                      <Tag v-if="invalidRefs.some((r) => r.kind === 'node' && r.name === n.name)" color="red">已失效</Tag>
                    </Checkbox>
                    <div v-if="manualNodes.length === 0" class="text-xs text-gray-400">暂无 manual 节点</div>
                  </div>
                </div>
                <div>
                  <div class="text-sm font-medium mb-1">xray 节点</div>
                  <div class="grid md:grid-cols-3 gap-2">
                    <Checkbox v-for="n in xrayNodes" :key="n.name" :checked="form.node_names.includes(n.name)"
                              :disabled="!n.allocatable || n.enabled === false" @change="toggleNode(n.name)">
                      <span>{{ n.render_name }}</span>
                      <span v-if="n.display_name" class="block text-xs text-gray-400 font-mono">{{ n.name }}</span>
                      <Tag v-if="!n.allocatable || n.enabled === false" class="ml-1">不可用</Tag>
                    </Checkbox>
                    <div v-if="xrayNodes.length === 0" class="text-xs text-gray-400">未检测到 Xray 节点（高级模式录入实例后刷新节点发现）</div>
                  </div>
                </div>
                <div v-if="targetSyntax === 'clash-yaml'">
                  <div class="text-sm font-medium mb-1">代理组</div>
                  <div class="grid md:grid-cols-3 gap-2">
                    <Checkbox v-for="g in FORCE_GROUPS" :key="g" :checked="true" disabled>{{ g }}<Tag class="ml-1">强制</Tag></Checkbox>
                    <Checkbox v-for="g in presetGroups" :key="g.name" :checked="form.group_names.includes(g.name)" :disabled="!g.enabled" @change="toggleGroup(g.name)">
                      {{ g.name }}<Tag class="ml-1">preset</Tag>
                    </Checkbox>
                    <Checkbox v-for="g in customGroups" :key="g.name" :checked="form.group_names.includes(g.name)" @change="toggleGroup(g.name)">
                      {{ g.name }}<Tag class="ml-1">自建</Tag>
                    </Checkbox>
                  </div>
                </div>
                <div v-if="targetSyntax === 'clash-yaml'">
                  <div class="text-sm font-medium mb-1">🌎国外流量成员（仅节点）</div>
                  <div class="grid md:grid-cols-3 gap-2">
                    <Checkbox v-for="n in manualNodes.concat(xrayNodes)" :key="n.name" :checked="form.overseas_members.includes(n.name)"
                              :disabled="n.source === 'xray' && (!n.allocatable || n.enabled === false)" @change="toggleOverseas(n.name)">
                      {{ n.render_name }}
                    </Checkbox>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <!-- ④ 规则素材 -->
          <div v-if="hasRulesStep" v-show="layoutMode === 'page' || currentStepKey === 'rules'">
            <Card v-if="layoutMode === 'page'" title="④ 规则素材" size="small" class="mb-3">
              <div class="space-y-3">
                <div>
                  <div class="text-sm font-medium mb-1">已勾选素材池（有序）</div>
                  <div v-if="form.pools.length === 0" class="text-xs text-gray-400">尚未添加素材池</div>
                  <div v-for="(p, idx) in form.pools" :key="idx" class="flex items-center gap-2 mb-2">
                    <Select :value="p.pool_id" class="w-48" @change="(v: any) => { form.pools[idx] = { ...form.pools[idx], pool_id: Number(v) } }">
                      <Select.Option v-for="pool in context?.pools ?? []" :key="pool.id" :value="pool.id">{{ pool.name }}</Select.Option>
                    </Select>
                    <Select v-if="targetSyntax !== 'sr-conf'" :value="p.target" class="flex-1" @change="(v: any) => { form.pools[idx] = { ...form.pools[idx], target: String(v) } }">
                      <Select.Option v-for="g in outputGroups" :key="g" :value="g">{{ g }}</Select.Option>
                    </Select>
                    <Radio.Group v-else :value="p.target" @change="(e: any) => { form.pools[idx] = { ...form.pools[idx], target: String(e.target.value) } }">
                      <Radio value="PROXY">PROXY</Radio>
                      <Radio value="DIRECT">DIRECT</Radio>
                    </Radio.Group>
                    <Button size="small" :disabled="idx === 0" @click="movePool(idx, idx - 1)">上移</Button>
                    <Button size="small" :disabled="idx === form.pools.length - 1" @click="movePool(idx, idx + 1)">下移</Button>
                    <Button size="small" danger @click="form.pools.splice(idx, 1)">移除</Button>
                  </div>
                  <Button size="small" @click="addPool">添加素材池</Button>
                </div>
                <div>
                  <div class="text-sm font-medium mb-1">手动规则行</div>
                  <div v-for="(r, idx) in form.custom_rules" :key="idx" class="flex items-center gap-2 mb-2">
                    <Select v-model:value="r.rule_type" class="w-48" :options="ruleTypeOptions.map((t) => ({ label: t, value: t }))" />
                    <Input v-model:value="r.match_value" placeholder="匹配值" class="flex-1" />
                    <Select v-if="targetSyntax !== 'sr-conf'" v-model:value="r.target" class="w-40" placeholder="目标组">
                      <Select.Option v-for="g in outputGroups" :key="g" :value="g">{{ g }}</Select.Option>
                    </Select>
                    <Radio.Group v-else v-model:value="r.target">
                      <Radio value="PROXY">PROXY</Radio>
                      <Radio value="DIRECT">DIRECT</Radio>
                    </Radio.Group>
                    <Button size="small" danger @click="removeRule(idx)">删除</Button>
                  </div>
                  <Button size="small" @click="addRule">添加规则行</Button>
                </div>
                <div v-if="targetSyntax === 'clash-yaml'" class="text-xs text-gray-400">USER-AGENT 规则在 Clash 中不支持，已从类型下拉排除。</div>
              </div>
            </Card>
            <div v-else>
              <div class="space-y-3">
                <div>
                  <div class="text-sm font-medium mb-1">已勾选素材池（有序）</div>
                  <div v-if="form.pools.length === 0" class="text-xs text-gray-400">尚未添加素材池</div>
                  <div v-for="(p, idx) in form.pools" :key="idx" class="flex items-center gap-2 mb-2">
                    <Select :value="p.pool_id" class="w-48" @change="(v: any) => { form.pools[idx] = { ...form.pools[idx], pool_id: Number(v) } }">
                      <Select.Option v-for="pool in context?.pools ?? []" :key="pool.id" :value="pool.id">{{ pool.name }}</Select.Option>
                    </Select>
                    <Select v-if="targetSyntax !== 'sr-conf'" :value="p.target" class="flex-1" @change="(v: any) => { form.pools[idx] = { ...form.pools[idx], target: String(v) } }">
                      <Select.Option v-for="g in outputGroups" :key="g" :value="g">{{ g }}</Select.Option>
                    </Select>
                    <Radio.Group v-else :value="p.target" @change="(e: any) => { form.pools[idx] = { ...form.pools[idx], target: String(e.target.value) } }">
                      <Radio value="PROXY">PROXY</Radio>
                      <Radio value="DIRECT">DIRECT</Radio>
                    </Radio.Group>
                    <Button size="small" :disabled="idx === 0" @click="movePool(idx, idx - 1)">上移</Button>
                    <Button size="small" :disabled="idx === form.pools.length - 1" @click="movePool(idx, idx + 1)">下移</Button>
                    <Button size="small" danger @click="form.pools.splice(idx, 1)">移除</Button>
                  </div>
                  <Button size="small" @click="addPool">添加素材池</Button>
                </div>
                <div>
                  <div class="text-sm font-medium mb-1">手动规则行</div>
                  <div v-for="(r, idx) in form.custom_rules" :key="idx" class="flex items-center gap-2 mb-2">
                    <Select v-model:value="r.rule_type" class="w-48" :options="ruleTypeOptions.map((t) => ({ label: t, value: t }))" />
                    <Input v-model:value="r.match_value" placeholder="匹配值" class="flex-1" />
                    <Select v-if="targetSyntax !== 'sr-conf'" v-model:value="r.target" class="w-40" placeholder="目标组">
                      <Select.Option v-for="g in outputGroups" :key="g" :value="g">{{ g }}</Select.Option>
                    </Select>
                    <Radio.Group v-else v-model:value="r.target">
                      <Radio value="PROXY">PROXY</Radio>
                      <Radio value="DIRECT">DIRECT</Radio>
                    </Radio.Group>
                    <Button size="small" danger @click="removeRule(idx)">删除</Button>
                  </div>
                  <Button size="small" @click="addRule">添加规则行</Button>
                </div>
                <div v-if="targetSyntax === 'clash-yaml'" class="text-xs text-gray-400">USER-AGENT 规则在 Clash 中不支持，已从类型下拉排除。</div>
              </div>
            </div>
          </div>

          <!-- ⑤ 预览 -->
          <div v-show="layoutMode === 'page' || currentStepKey === 'preview'">
            <Card v-if="layoutMode === 'page'" title="⑤ 预览" size="small" class="mb-3">
              <div class="space-y-3">
                <Space>
                  <Button type="primary" :loading="previewing" @click="doPreview">预览产物</Button>
                  <Button :loading="diffLoading" @click="toggleDiff">与当前激活版本对比</Button>
                </Space>
                <Alert v-for="(w, i) in previewWarnings" :key="i" type="warning" show-icon :message="w" />
                <Alert v-for="(s, i) in previewSkipped" :key="'s'+i" type="warning" show-icon :message="`跳过 ${s.name}：${s.reason}`" />
                <div v-if="previewText">
                  <h3 class="font-semibold mb-2">预览</h3>
                  <pre class="bg-gray-50 dark:bg-gray-900 rounded p-3 text-xs overflow-auto max-h-[50vh] whitespace-pre-wrap">{{ previewText }}</pre>
                </div>
                <DiffView v-if="showDiff && previewText" :old-text="diffOld" :new-text="previewText" :target-missing="diffMissing" />
              </div>
            </Card>
            <div v-else>
              <div class="space-y-3">
                <Space>
                  <Button type="primary" :loading="previewing" @click="doPreview">预览产物</Button>
                  <Button :loading="diffLoading" @click="toggleDiff">与当前激活版本对比</Button>
                </Space>
                <Alert v-for="(w, i) in previewWarnings" :key="i" type="warning" show-icon :message="w" />
                <Alert v-for="(s, i) in previewSkipped" :key="'s'+i" type="warning" show-icon :message="`跳过 ${s.name}：${s.reason}`" />
                <div v-if="previewText">
                  <h3 class="font-semibold mb-2">预览</h3>
                  <pre class="bg-gray-50 dark:bg-gray-900 rounded p-3 text-xs overflow-auto max-h-[50vh] whitespace-pre-wrap">{{ previewText }}</pre>
                </div>
                <DiffView v-if="showDiff && previewText" :old-text="diffOld" :new-text="previewText" :target-missing="diffMissing" />
              </div>
            </div>
          </div>

          <!-- ⑥ 确认生成 -->
          <div v-show="layoutMode === 'page' || currentStepKey === 'generate'">
            <Card v-if="layoutMode === 'page'" title="⑥ 确认生成" size="small" class="mb-3">
              <div class="space-y-3">
                <Alert v-if="invalidRefs.length" type="error" show-icon message="存在失效引用，请先剔除后生成" />
                <Alert v-if="targetSyntax === 'clash-yaml' && form.overseas_members.length === 0" type="error" show-icon message="「🌎国外流量」组未包含任何节点" />
                <Button type="primary" :loading="generating" @click="doGenerate">生成版本</Button>
              </div>
            </Card>
            <div v-else>
              <div class="space-y-3">
                <Alert v-if="invalidRefs.length" type="error" show-icon message="存在失效引用，请先剔除后生成" />
                <Alert v-if="targetSyntax === 'clash-yaml' && form.overseas_members.length === 0" type="error" show-icon message="「🌎国外流量」组未包含任何节点" />
                <Button type="primary" :loading="generating" @click="doGenerate">生成版本</Button>
              </div>
            </div>
          </div>

          <div v-if="layoutMode === 'step'" class="flex items-center justify-between">
            <Button :disabled="currentStep === 0" @click="prevStep">上一步</Button>
            <Button v-if="currentStep < stepDefs.length - 1" type="primary" @click="nextStep">下一步</Button>
            <Button v-else type="primary" :loading="generating" @click="doGenerate">生成版本</Button>
          </div>

          <Result v-if="generateResult" status="success"
                  :title="generateResult.auto_activated ? '首个版本已自动激活' : '已入池未生效，请激活'"
                  :sub-title="`版本 v${generateResult.version_no} 已入池${generateResult.auto_activated ? '并激活' : '，请到版本管理激活'}`">
            <template #extra>
              <Space>
                <Button type="primary" @click="router.push(isSrConf ? '/admin/rules' : '/admin/subscriptions')">去版本管理激活</Button>
                <Button @click="continueAssembly">继续装配</Button>
              </Space>
            </template>
          </Result>
        </div>
      </Tabs.TabPane>
    </Tabs>

    <Modal :open="headerConfirmOpen" title="采用默认头部" :footer="null" :width="420" @cancel="headerConfirmOpen = false">
      <p>将覆盖当前已填头部字段，确定采用默认值吗？</p>
      <div class="flex justify-end gap-2">
        <Button @click="headerConfirmOpen = false">取消</Button>
        <Button type="primary" @click="applyDefaultHeader">确认采用</Button>
      </div>
    </Modal>
  </div>
</template>
