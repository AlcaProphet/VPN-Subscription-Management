<!-- AssemblyView.vue：订阅装配（Design2-UI §5）——四类装配器 + 预览 diff + 重新编辑 -->
<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Alert, Button, Modal, Result, Space, Tabs, message } from 'ant-design-vue'
import PageHeader from '@/components/PageHeader.vue'
import PoolTab from './assembly/PoolTab.vue'
import AssemblerShell from './assembly/AssemblerShell.vue'
import TypeTargetStep from './assembly/TypeTargetStep.vue'
import HeaderStep from './assembly/HeaderStep.vue'
import NodesGroupsStep from './assembly/NodesGroupsStep.vue'
import RulesStep from './assembly/RulesStep.vue'
import PreviewStep from './assembly/PreviewStep.vue'
import GenerateStep from './assembly/GenerateStep.vue'
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
const editVersionNo = ref<number | null>(null)
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
  fixed_params_text: DEFAULT_HEADERS['clash-yaml'],
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
    editVersionNo.value = bp.version_no ?? null
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
    Notify.info(editVersionNo.value ? `正在重新编辑版本 v${editVersionNo.value}，请检查失效引用` : `正在重新编辑版本 #${editVersionId.value}，请检查失效引用`)
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
    if (!res.auto_activated && !isSrConf.value && form.platform_id) {
      // R14-09：装配生成入池后与上传流一致，标记订阅行「已入池未生效」
      try {
        const subs = await listSubscriptions()
        const sub = subs.find((s) => s.platform_id === form.platform_id)
        if (sub) sessionStorage.setItem(`pooled_sub_${sub.id}`, '1')
      } catch {
        // 标记失败不阻断生成回执
      }
    }
    editVersionId.value = null
    editVersionNo.value = null
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

async function goActivation() {
  if (isSrConf.value) {
    if (form.rule_id) void router.push(`/admin/rules/${form.rule_id}/versions`)
    else void router.push('/admin/rules')
    return
  }
  if (!form.platform_id) {
    void router.push('/admin/subscriptions')
    return
  }
  try {
    const subs = await listSubscriptions()
    const sub = subs.find((s) => s.platform_id === form.platform_id)
    if (sub) {
      void router.push(`/admin/subscriptions/${sub.id}/versions`)
    } else {
      void router.push('/admin/subscriptions')
    }
  } catch (err) {
    Notify.error((err as Error).message)
    void router.push('/admin/subscriptions')
  }
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
    <Alert v-if="editVersionId" type="info" show-icon class="mb-4"
           :message="editVersionNo ? `正在重新编辑版本 v${editVersionNo}，请检查失效引用后生成新版本` : `正在重新编辑版本 #${editVersionId}，请检查失效引用后生成新版本`" />
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
        <div v-else>
          <AssemblerShell
            :layout-mode="layoutMode"
            :step-defs="stepDefs"
            :current-step="currentStep"
            :current-step-key="currentStepKey"
            :has-header-step="hasHeaderStep"
            :has-nodes-step="hasNodesStep"
            :has-rules-step="hasRulesStep"
            :generating="generating"
            @update:layout-mode="(v: string) => layoutMode = (v === 'page' ? 'page' : 'step')"
            @update:current-step="(v: number) => currentStep = v"
            @next="nextStep"
            @prev="prevStep"
            @generate="doGenerate"
          >
            <template #target>
              <TypeTargetStep :form="form" :context="context" :is-sr-conf="isSrConf" :filtered-platforms="filteredPlatforms" />
            </template>
            <template #header>
              <HeaderStep :form="form" :target-syntax="targetSyntax" @apply-default="headerConfirmOpen = true" />
            </template>
            <template #nodes>
              <NodesGroupsStep :form="form" :context="context" :target-syntax="targetSyntax" :invalid-refs="invalidRefs"
                               :manual-nodes="manualNodes" :xray-nodes="xrayNodes" :preset-groups="presetGroups" :custom-groups="customGroups"
                               @toggle-node="toggleNode" @toggle-group="toggleGroup" @toggle-overseas="toggleOverseas" />
            </template>
            <template #rules>
              <RulesStep :form="form" :context="context" :target-syntax="targetSyntax" :output-groups="outputGroups" :rule-type-options="ruleTypeOptions"
                         @add-pool="addPool" @move-pool="movePool" @remove-pool="(i: number) => form.pools.splice(i, 1)"
                         @add-rule="addRule" @remove-rule="removeRule" />
            </template>
            <template #preview>
              <PreviewStep :previewing="previewing" :preview-warnings="previewWarnings" :preview-skipped="previewSkipped"
                           :preview-text="previewText" :show-diff="showDiff" :diff-old="diffOld" :diff-missing="diffMissing" :diff-loading="diffLoading"
                           @preview="doPreview" @toggle-diff="toggleDiff" />
            </template>
            <template #generate>
              <GenerateStep :invalid-count="invalidRefs.length" :clash-empty-overseas="targetSyntax === 'clash-yaml' && form.overseas_members.length === 0"
                            :generating="generating" @generate="doGenerate" />
            </template>
          </AssemblerShell>

          <Result v-if="generateResult" status="success"
                  :title="generateResult.auto_activated ? '首个版本已自动激活' : '已入池未生效，请激活'"
                  :sub-title="`版本 v${generateResult.version_no} 已入池${generateResult.auto_activated ? '并激活' : '，请到版本管理激活'}`">
            <template #extra>
              <Space>
                <Button type="primary" @click="goActivation">去版本管理激活</Button>
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
