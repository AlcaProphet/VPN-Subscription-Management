<!-- AssemblyView.vue：订阅装配（Design2-UI §5）——四类装配器 + 预览 diff + 重新编辑 -->
<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Alert, Button, Checkbox, Form, Input, Select, Space, Tabs, Tag, message } from 'ant-design-vue'
import PageHeader from '@/components/PageHeader.vue'
import PoolTab from './assembly/PoolTab.vue'
import DiffView from '@/components/DiffView.vue'
import {
  getAssemblyContext, previewAssembly, generateAssembly, getBlueprint,
  type AssemblyContext, type GenerateInput, type TargetSyntax, type PoolSelection, type RuleLine,
} from '@/api/assembly'
import { Notify } from '@/components/Notify'

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

const form = reactive({
  platform_id: undefined as number | undefined,
  rule_id: undefined as number | undefined,
  fixed_params_text: '{}',
  node_names: [] as string[],
  group_names: [] as string[],
  overseas_members: [] as string[],
  pools: [] as PoolSelection[],
  custom_rules_text: '',
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
    form.custom_rules_text = (bp.custom_rules ?? []).map((r: RuleLine) => `${r.rule_type},${r.match_value},${r.target}`).join('\n')
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
  return form.custom_rules_text.split('\n').filter(Boolean).map((line) => {
    const parts = line.split(',').map((s) => s.trim())
    return { rule_type: parts[0] ?? '', match_value: parts[1] ?? '', target: parts[2] ?? '' }
  })
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
async function doGenerate() {
  generating.value = true
  try {
    const res = await generateAssembly(buildInput())
    Notify.success(res.auto_activated ? '首个版本已自动激活' : '已入池未生效，请激活')
    editVersionId.value = null
    void router.replace({ query: { tab: activeTab.value } })
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    generating.value = false
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
    <Alert v-if="editVersionId" type="info" show-icon class="mb-4" message="正在重新编辑版本，请检查失效引用后生成新版本" />
    <Alert v-for="ref in invalidRefs" :key="`${ref.kind}-${ref.name}`" type="error" show-icon class="mb-2"
           :message="`失效${ref.kind === 'node' ? '节点' : ref.kind === 'group' ? '代理组' : '素材池'}：${ref.name}`" />
    <Alert v-if="Object.keys(nameChanged).length" type="warning" show-icon class="mb-2"
           message="以下节点显示名已变化，生成时将按当前显示名渲染" :description="Object.entries(nameChanged).map(([k,v]) => `${k} → ${v}`).join('；')" />
    <Tabs v-model:activeKey="activeTab" @change="onTabChange">
      <Tabs.TabPane key="pool" tab="规则素材池">
        <PoolTab />
      </Tabs.TabPane>
      <Tabs.TabPane v-for="tab in (['clash-yaml','sr-subs','generic-subs','sr-conf'] as const)" :key="tab" :tab="tab">
        <div class="space-y-4">
          <Form layout="vertical">
            <div class="grid md:grid-cols-2 gap-4">
              <Form.Item :label="isSrConf ? '规则实体' : '目标平台'">
                <Select v-if="isSrConf" v-model:value="form.rule_id" placeholder="选择规则实体" class="w-full">
                  <Select.Option v-for="r in context?.rules ?? []" :key="r.id" :value="r.id">{{ r.name }}</Select.Option>
                </Select>
                <Select v-else v-model:value="form.platform_id" placeholder="选择平台" class="w-full">
                  <Select.Option v-for="p in filteredPlatforms" :key="p.id" :value="p.id">{{ p.name }}（{{ p.product_type }}）</Select.Option>
                </Select>
              </Form.Item>
              <Form.Item v-if="isSrConf" label="FINAL 方向">
                <Select v-model:value="form.final_direction" class="w-full">
                  <Select.Option value="PROXY">PROXY</Select.Option>
                  <Select.Option value="DIRECT">DIRECT</Select.Option>
                </Select>
              </Form.Item>
            </div>
            <Form.Item v-if="targetSyntax !== 'generic-subs'" label="头部参数（JSON）">
              <Input.TextArea v-model:value="form.fixed_params_text" :rows="4" placeholder='{"port":7890,"mode":"rule"}' />
            </Form.Item>
            <Form.Item label="节点">
              <div class="grid md:grid-cols-3 gap-2">
                <Checkbox v-for="n in context?.nodes ?? []" :key="n.name" :checked="form.node_names.includes(n.name)"
                  @change="(e: any) => { const v = e.target.checked; form.node_names = v ? [...form.node_names, n.name] : form.node_names.filter((x) => x !== n.name) }">
                  {{ n.render_name }}<Tag v-if="n.source === 'xray'" class="ml-1">xray</Tag>
                </Checkbox>
              </div>
            </Form.Item>
            <Form.Item label="代理组">
              <div class="grid md:grid-cols-3 gap-2">
                <Checkbox v-for="g in context?.proxy_groups ?? []" :key="g.name" :checked="form.group_names.includes(g.name)"
                  @change="(e: any) => { const v = e.target.checked; form.group_names = v ? [...form.group_names, g.name] : form.group_names.filter((x) => x !== g.name) }">
                  {{ g.name }}<Tag v-if="g.type === 'preset'" class="ml-1">preset</Tag>
                </Checkbox>
              </div>
            </Form.Item>
            <Form.Item v-if="targetSyntax === 'clash-yaml'" label="🌎国外流量成员">
              <Select mode="multiple" v-model:value="form.overseas_members" class="w-full" placeholder="选择节点">
                <Select.Option v-for="n in context?.nodes ?? []" :key="n.name" :value="n.name">{{ n.render_name }}</Select.Option>
              </Select>
            </Form.Item>
            <Form.Item label="规则素材池（有序 + 目标）">
              <div class="space-y-2">
                <div v-for="(p, idx) in form.pools" :key="idx" class="flex gap-2">
                  <Select :value="p.pool_id" class="w-48" @change="(v: any) => { form.pools[idx] = { ...form.pools[idx], pool_id: Number(v) } }">
                    <Select.Option v-for="pool in context?.pools ?? []" :key="pool.id" :value="pool.id">{{ pool.name }}</Select.Option>
                  </Select>
                  <Select :value="p.target" class="flex-1" @change="(v: any) => { form.pools[idx] = { ...form.pools[idx], target: String(v) } }">
                    <Select.Option v-for="g in outputGroups" :key="g" :value="g">{{ g }}</Select.Option>
                    <Select.Option v-if="isSrConf" value="PROXY">PROXY</Select.Option>
                    <Select.Option v-if="isSrConf" value="DIRECT">DIRECT</Select.Option>
                  </Select>
                  <Button danger @click="form.pools.splice(idx, 1)">移除</Button>
                </div>
                <Button @click="form.pools.push({ pool_id: context?.pools?.[0]?.id ?? 0, target: outputGroups[0] ?? '' })">添加素材池</Button>
              </div>
            </Form.Item>
            <Form.Item label="手动规则行（每行：类型,匹配值,目标）">
              <Input.TextArea v-model:value="form.custom_rules_text" :rows="3" placeholder="DOMAIN-SUFFIX,example.com,组A" />
            </Form.Item>
          </Form>

          <Space>
            <Button type="primary" :loading="previewing" @click="doPreview">预览产物</Button>
            <Button type="primary" ghost :loading="generating" @click="doGenerate">生成版本</Button>
            <Button @click="showDiff = !showDiff">与当前激活版本对比</Button>
          </Space>

          <Alert v-for="(w, i) in previewWarnings" :key="i" type="warning" show-icon :message="w" />
          <Alert v-for="(s, i) in previewSkipped" :key="'s'+i" type="warning" show-icon :message="`跳过 ${s.name}：${s.reason}`" />

          <div v-if="previewText">
            <h3 class="font-semibold mb-2">预览</h3>
            <pre class="bg-gray-50 dark:bg-gray-900 rounded p-3 text-xs overflow-auto max-h-[50vh] whitespace-pre-wrap">{{ previewText }}</pre>
          </div>
          <DiffView v-if="showDiff && previewText" :old-text="diffOld" :new-text="previewText" :target-missing="diffMissing" />
        </div>
      </Tabs.TabPane>
    </Tabs>
  </div>
</template>
