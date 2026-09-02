<!-- HeaderStep.vue：装配步骤②头部表单；Clash 按实际配置分区，保留分区级高级 JSON。 -->
<script setup lang="ts">
import { computed, nextTick, reactive, ref, watch } from 'vue'
import { Button, Collapse, Input, InputNumber, Select, Space, Switch } from 'ant-design-vue'
import type { TargetSyntax } from '@/api/assembly'
import { CLASH_HEADER_DEFAULTS } from './clashHeaderDefaults'

const props = defineProps<{
  form: { fixed_params_text: string }
  targetSyntax: TargetSyntax
}>()

const emit = defineEmits<{ 'apply-default': [] }>()

interface HeaderField {
  name: string
  label: string
  type: 'text' | 'number' | 'bool'
  default?: unknown
}

type ClashSectionID = 'ports' | 'geodata' | 'dns' | 'more'
const PORT_KEYS = ['port', 'socks-port', 'redir-port', 'tproxy-port', 'mixed-port'] as const
const PORT_FIELDS = [
  { name: 'port', label: 'port', description: 'HTTP/HTTPS 代理监听端口' },
  { name: 'socks-port', label: 'socks-port', description: 'SOCKS5 代理监听端口' },
  { name: 'redir-port', label: 'redir-port', description: 'Linux/macOS Redir 透明代理端口' },
  { name: 'tproxy-port', label: 'tproxy-port', description: 'Linux TProxy TCP/UDP 透明代理端口' },
  { name: 'mixed-port', label: 'Mixed Port（可选）', description: 'HTTP 与 SOCKS 共用监听端口（可选）' },
] as const
const GEO_KEYS = ['geox-url', 'geo-auto-update', 'geo-update-interval'] as const
const MORE_KEYS = ['allow-lan', 'find-process-mode', 'mode', 'log-level', 'ipv6', 'ntp'] as const
const controlledKeys = new Set<string>([...PORT_KEYS, ...GEO_KEYS, 'dns'])
const NTP_PRESETS = [
  { value: 'ntp.aliyun.com', label: '阿里云（ntp.aliyun.com）' },
  { value: 'ntp.tencent.com', label: '腾讯云（ntp.tencent.com）' },
  { value: 'ntp.ntsc.ac.cn', label: '国家授时中心（ntp.ntsc.ac.cn）' },
  { value: 'time.cloudflare.com', label: 'Cloudflare（time.cloudflare.com）' },
] as const

const headerFields = computed<HeaderField[]>(() => {
  switch (props.targetSyntax) {
    case 'sr-subs':
      return [
        { name: 'status', label: 'STATUS', type: 'text', default: '2026/01/01 Version' },
        { name: 'remarks', label: 'REMARKS', type: 'text', default: 'VPN Subscription' },
      ]
    case 'sr-conf':
      return [
        { name: 'loglevel', label: 'Log Level', type: 'text', default: 'warning' },
        { name: 'ipv6', label: 'IPv6', type: 'bool', default: false },
        { name: 'dns-server', label: 'DNS Server', type: 'text', default: '223.6.6.6, 119.29.29.29' },
      ]
    default:
      return []
  }
})
const inputFields = computed(() => headerFields.value.filter((field) => field.type !== 'bool'))
const switchFields = computed(() => headerFields.value.filter((field) => field.type === 'bool'))

const advancedWhole = ref(false)
const advancedSections = reactive<Record<ClashSectionID, boolean>>({ ports: false, geodata: false, dns: false, more: false })
const sectionJSON = reactive<Record<ClashSectionID, string>>({ ports: '', geodata: '', dns: '', more: '' })
const sectionErrors = reactive<Record<ClashSectionID, string>>({ ports: '', geodata: '', dns: '', more: '' })
const applyingSection = ref<ClashSectionID | null>(null)

function isRecord(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === 'object' && !Array.isArray(value)
}

function cloneDefault<T>(value: T): T {
  return JSON.parse(JSON.stringify(value)) as T
}

function defaults(): Record<string, unknown> {
  return cloneDefault(CLASH_HEADER_DEFAULTS) as Record<string, unknown>
}

function parseObject(): Record<string, unknown> {
  try {
    const value = JSON.parse(props.form.fixed_params_text || '{}')
    return isRecord(value) ? value : {}
  } catch {
    return {}
  }
}

function updateRoot(next: Record<string, unknown>, source?: ClashSectionID) {
  applyingSection.value = source ?? null
  props.form.fixed_params_text = JSON.stringify(next, null, 2)
  void nextTick(() => { applyingSection.value = null })
}

function rootValue(name: string): unknown {
  const root = parseObject()
  return root[name] === undefined ? defaults()[name] : root[name]
}

function stringValue(value: unknown, fallback = ''): string {
  return typeof value === 'string' ? value : fallback
}

function numberValue(value: unknown, fallback = 0): number {
  return typeof value === 'number' && Number.isFinite(value) ? value : fallback
}

function boolValue(value: unknown, fallback = false): boolean {
  return typeof value === 'boolean' ? value : fallback
}

function setRootValue(name: string, value: unknown) {
  const root = parseObject()
  if (value === undefined) delete root[name]
  else root[name] = value
  updateRoot(root)
}

function portValue(name: typeof PORT_KEYS[number]): number | undefined {
  const value = parseObject()[name]
  if (typeof value === 'number' && Number.isFinite(value)) return value
  const fallback = defaults()[name]
  return typeof fallback === 'number' ? fallback : undefined
}

function setPort(name: typeof PORT_KEYS[number], value: number | null) {
  setRootValue(name, value === null && name === 'mixed-port' ? undefined : value ?? 0)
}

function geoxValue(name: 'geoip' | 'geosite' | 'mmdb'): string {
  const geox = rootValue('geox-url')
  const fallback = (defaults()['geox-url'] as Record<string, unknown>)[name]
  return stringValue(isRecord(geox) ? geox[name] : undefined, stringValue(fallback))
}

function setGeoxValue(name: 'geoip' | 'geosite' | 'mmdb', value: string) {
  const current = rootValue('geox-url')
  const geox = isRecord(current) ? { ...current } : {}
  geox[name] = value
  setRootValue('geox-url', geox)
}

function dnsConfig(): Record<string, unknown> {
  const current = parseObject().dns
  return isRecord(current) ? { ...current } : cloneDefault(CLASH_HEADER_DEFAULTS.dns) as Record<string, unknown>
}

function dnsValue(name: string): unknown {
  const current = dnsConfig()[name]
  const fallback = (CLASH_HEADER_DEFAULTS.dns as Record<string, unknown>)[name]
  return current === undefined ? cloneDefault(fallback) : current
}

function setDNSValue(name: string, value: unknown) {
  const dns = dnsConfig()
  dns[name] = value
  setRootValue('dns', dns)
}

function fallbackFilter(): Record<string, unknown> {
  const current = dnsValue('fallback-filter')
  return isRecord(current) ? { ...current } : cloneDefault(CLASH_HEADER_DEFAULTS.dns['fallback-filter']) as Record<string, unknown>
}

function fallbackFilterValue(name: string): unknown {
  const current = fallbackFilter()[name]
  const defaultsFilter = CLASH_HEADER_DEFAULTS.dns['fallback-filter'] as Record<string, unknown>
  return current === undefined ? cloneDefault(defaultsFilter[name]) : current
}

function setFallbackFilterValue(name: string, value: unknown) {
  setDNSValue('fallback-filter', { ...fallbackFilter(), [name]: value })
}

function stringList(value: unknown, fallback: readonly string[]): string[] {
  return Array.isArray(value) ? value.map((item) => String(item)) : [...fallback]
}

function dnsList(name: 'default-nameserver' | 'fallback' | 'fake-ip-filter'): string[] {
  return stringList(dnsValue(name), CLASH_HEADER_DEFAULTS.dns[name])
}

function setDNSList(name: 'default-nameserver' | 'fallback' | 'fake-ip-filter', values: string[]) {
  setDNSValue(name, values)
}

function filterList(name: 'geosite' | 'ipcidr'): string[] {
  return stringList(fallbackFilterValue(name), CLASH_HEADER_DEFAULTS.dns['fallback-filter'][name])
}

function setFilterList(name: 'geosite' | 'ipcidr', values: string[]) {
  setFallbackFilterValue(name, values)
}

function replaceList(values: string[], index: number, value: string): string[] {
  const next = [...values]
  next[index] = value
  return next
}

function removeListItem(values: string[], index: number): string[] {
  const next = [...values]
  next.splice(index, 1)
  return next
}

function ntpValue(name: 'server' | 'port' | 'interval' | 'enable' | 'write-to-system'): unknown {
  const current = rootValue('ntp')
  const fallback = (CLASH_HEADER_DEFAULTS.ntp as Record<string, unknown>)[name]
  return isRecord(current) && current[name] !== undefined ? current[name] : fallback
}

function setNTPValue(name: 'server' | 'port' | 'interval' | 'enable' | 'write-to-system', value: unknown) {
  const current = rootValue('ntp')
  const ntp = isRecord(current) ? { ...current } : {}
  ntp[name] = value
  setRootValue('ntp', ntp)
}

function ntpPresetValue(): string | undefined {
  const server = stringValue(ntpValue('server'), 'ntp.aliyun.com')
  return NTP_PRESETS.some((preset) => preset.value === server) ? server : undefined
}

function customNTPServer(): string {
  const server = stringValue(ntpValue('server'), 'ntp.aliyun.com')
  return NTP_PRESETS.some((preset) => preset.value === server) ? '' : server
}

function setCustomNTPServer(value: string) {
  const server = value.trim()
  if (server) {
    setNTPValue('server', server)
    return
  }
  if (!ntpPresetValue()) setNTPValue('server', CLASH_HEADER_DEFAULTS.ntp.server)
}

function fieldValue(name: string): unknown {
  const value = parseObject()[name]
  if (value !== undefined) return value
  return headerFields.value.find((field) => field.name === name)?.default
}

function setField(name: string, value: unknown) {
  setRootValue(name, value)
}

function sectionPayload(id: ClashSectionID): Record<string, unknown> {
  const root = parseObject()
  const fallback = defaults()
  if (id === 'ports' || id === 'geodata') {
    const keys = id === 'ports' ? PORT_KEYS : GEO_KEYS
    const payload: Record<string, unknown> = {}
    for (const key of keys) {
      if (root[key] !== undefined) payload[key] = root[key]
      else if (fallback[key] !== undefined) payload[key] = fallback[key]
    }
    return payload
  }
  if (id === 'dns') return dnsConfig()

  const payload: Record<string, unknown> = {}
  for (const key of MORE_KEYS) payload[key] = root[key] === undefined ? fallback[key] : root[key]
  for (const [key, value] of Object.entries(root)) {
    if (!controlledKeys.has(key) && !(key in payload)) payload[key] = value
  }
  return payload
}

function refreshSectionJSON(id: ClashSectionID) {
  sectionJSON[id] = JSON.stringify(sectionPayload(id), null, 2)
  sectionErrors[id] = ''
}

function setSectionAdvanced(id: ClashSectionID, enabled: boolean) {
  if (!enabled && sectionErrors[id]) return
  advancedSections[id] = enabled
  if (enabled) refreshSectionJSON(id)
}

function updateSectionJSON(id: ClashSectionID, text: string) {
  sectionJSON[id] = text
  try {
    const parsed = JSON.parse(text || '{}')
    if (!isRecord(parsed)) throw new Error('shape')
    const root = parseObject()
    if (id === 'ports' || id === 'geodata') {
      const keys = id === 'ports' ? PORT_KEYS : GEO_KEYS
      const invalid = Object.keys(parsed).find((key) => !keys.includes(key as never))
      if (invalid) throw new Error(`参数“${invalid}”不属于此分区`)
      for (const key of keys) delete root[key]
      for (const key of keys) if (parsed[key] !== undefined) root[key] = parsed[key]
    } else if (id === 'dns') {
      root.dns = parsed
    } else {
      const invalid = Object.keys(parsed).find((key) => controlledKeys.has(key))
      if (invalid) throw new Error(`参数“${invalid}”请在对应分区填写`)
      for (const key of Object.keys(root)) if (!controlledKeys.has(key)) delete root[key]
      Object.assign(root, parsed)
    }
    sectionErrors[id] = ''
    updateRoot(root, id)
  } catch (error) {
    sectionErrors[id] = error instanceof Error && error.message !== 'shape'
      ? error.message
      : '请输入 JSON 对象'
  }
}

watch(() => props.form.fixed_params_text, () => {
  for (const id of Object.keys(advancedSections) as ClashSectionID[]) {
    if (advancedSections[id] && applyingSection.value !== id) refreshSectionJSON(id)
  }
})
</script>

<template>
  <div class="space-y-2">
    <div v-if="targetSyntax === 'generic-subs'" class="text-sm text-text-tertiary">
      通用节点订阅不输出 STATUS/REMARKS 头部，本步无需填写。
    </div>
    <div v-else>
      <div class="flex items-center justify-between gap-3 mb-1">
        <span class="text-sm">{{ targetSyntax === 'clash-yaml' ? 'Clash 头部参数' : (advancedWhole ? '头部参数（JSON）' : '头部参数') }}</span>
        <Space>
          <Button size="small" danger @click="emit('apply-default')">使用默认值</Button>
          <label v-if="targetSyntax !== 'clash-yaml'" class="flex items-center gap-2 text-sm text-text-secondary whitespace-nowrap">
            <Switch v-model:checked="advancedWhole" size="small" />
            <span>高级 JSON</span>
          </label>
        </Space>
      </div>

      <template v-if="targetSyntax === 'clash-yaml'">
        <div class="clash-header-hint">
          <span class="clash-header-hint-title">按需展开编辑</span>
          <span>所有分区均已按默认模板预填；不确定参数用途时请保持默认值。结构化表格和高级 JSON 会写入同一份 Clash 头部配置。</span>
        </div>
        <Collapse class="clash-header-sections">
          <Collapse.Panel key="ports">
            <template #header><div class="collapse-panel-title"><span>端口配置</span><span>默认已预填</span></div></template>
            <div class="clash-header-section" data-section="ports">
              <div class="section-editor-toolbar"><span>HTTP、SOCKS 与透明代理监听端口</span><label class="flex items-center gap-2 text-xs text-text-secondary whitespace-nowrap"><Switch :checked="advancedSections.ports" size="small" @change="(value: any) => setSectionAdvanced('ports', Boolean(value))" /><span>高级 JSON</span></label></div>
              <div v-if="advancedSections.ports"><Input.TextArea class="section-json-editor" :value="sectionJSON.ports" :rows="7" @input="(event: any) => updateSectionJSON('ports', event.target.value)" /><div v-if="sectionErrors.ports" class="section-json-error">{{ sectionErrors.ports }}</div></div>
              <div v-else class="header-input-fields grid grid-cols-1 md:grid-cols-2 gap-3"><div v-for="field in PORT_FIELDS" :key="field.name"><label class="text-sm text-text-secondary">{{ field.label }}</label><div class="field-description">{{ field.description }}</div><InputNumber :value="portValue(field.name)" class="w-full" :placeholder="field.name === 'mixed-port' ? '未启用' : undefined" @change="(value: any) => setPort(field.name, value)" /></div></div>
            </div>
          </Collapse.Panel>

          <Collapse.Panel key="geodata">
            <template #header><div class="collapse-panel-title"><span>Geo 数据</span><span>默认已预填</span></div></template>
            <div class="clash-header-section" data-section="geodata">
              <div class="section-editor-toolbar"><span>GeoIP、GeoSite 与 MMDB 下载地址</span><label class="flex items-center gap-2 text-xs text-text-secondary whitespace-nowrap"><Switch :checked="advancedSections.geodata" size="small" @change="(value: any) => setSectionAdvanced('geodata', Boolean(value))" /><span>高级 JSON</span></label></div>
              <div v-if="advancedSections.geodata"><Input.TextArea class="section-json-editor" :value="sectionJSON.geodata" :rows="9" @input="(event: any) => updateSectionJSON('geodata', event.target.value)" /><div v-if="sectionErrors.geodata" class="section-json-error">{{ sectionErrors.geodata }}</div></div>
              <template v-else>
                <div class="header-input-fields grid grid-cols-1 md:grid-cols-2 gap-3"><div v-for="name in ['geoip', 'geosite', 'mmdb'] as const" :key="name"><label class="text-sm text-text-secondary">{{ name }}</label><Input :value="geoxValue(name)" @change="(event: any) => setGeoxValue(name, event.target.value)" /></div><div><label class="text-sm text-text-secondary">更新间隔（小时）</label><InputNumber :value="numberValue(rootValue('geo-update-interval'), 168)" class="w-full" @change="(value: any) => setRootValue('geo-update-interval', value ?? 0)" /></div></div>
                <section class="header-switch-fields section-switch-fields"><div class="text-sm font-medium">开关参数</div><div class="section-switch-grid"><label class="section-switch-option"><span>自动更新 Geo 数据</span><Switch :checked="boolValue(rootValue('geo-auto-update'), true)" @change="(value: any) => setRootValue('geo-auto-update', Boolean(value))" /></label></div></section>
              </template>
            </div>
          </Collapse.Panel>

          <Collapse.Panel key="dns">
            <template #header><div class="collapse-panel-title"><span>DNS 配置</span><span>默认已预填</span></div></template>
            <div class="clash-header-section" data-section="dns">
              <div class="section-editor-toolbar"><span>DNS 服务、Fake-IP 与回退过滤策略</span><label class="flex items-center gap-2 text-xs text-text-secondary whitespace-nowrap"><Switch :checked="advancedSections.dns" size="small" @change="(value: any) => setSectionAdvanced('dns', Boolean(value))" /><span>高级 JSON</span></label></div>
              <div v-if="advancedSections.dns"><Input.TextArea class="section-json-editor" :value="sectionJSON.dns" :rows="16" @input="(event: any) => updateSectionJSON('dns', event.target.value)" /><div v-if="sectionErrors.dns" class="section-json-error">{{ sectionErrors.dns }}</div></div>
              <template v-else>
                <div class="header-input-fields grid grid-cols-1 md:grid-cols-2 gap-3"><div><label class="text-sm text-text-secondary">监听地址</label><Input :value="stringValue(dnsValue('listen'))" @change="(event: any) => setDNSValue('listen', event.target.value)" /></div><div><label class="text-sm text-text-secondary">增强模式</label><Select class="w-full" :value="stringValue(dnsValue('enhanced-mode'), 'fake-ip')" @change="(value: any) => setDNSValue('enhanced-mode', value)"><Select.Option value="fake-ip">fake-ip</Select.Option><Select.Option value="redir-host">redir-host</Select.Option></Select></div><div><label class="text-sm text-text-secondary">Fake-IP 范围</label><Input :value="stringValue(dnsValue('fake-ip-range'))" @change="(event: any) => setDNSValue('fake-ip-range', event.target.value)" /></div><div><label class="text-sm text-text-secondary">回退 GeoIP 代码</label><Input :value="stringValue(fallbackFilterValue('geoip-code'), 'CN')" @change="(event: any) => setFallbackFilterValue('geoip-code', event.target.value)" /></div></div>
                <section class="header-switch-fields section-switch-fields"><div class="text-sm font-medium">开关参数</div><div class="section-switch-grid"><label class="section-switch-option"><span>启用 DNS</span><Switch :checked="boolValue(dnsValue('enable'), true)" @change="(value: any) => setDNSValue('enable', Boolean(value))" /></label><template v-if="boolValue(dnsValue('enable'), true)"><label class="section-switch-option"><span>DNS IPv6</span><Switch :checked="boolValue(dnsValue('ipv6'), false)" @change="(value: any) => setDNSValue('ipv6', Boolean(value))" /></label><label class="section-switch-option"><span>回退 GeoIP 过滤</span><Switch :checked="boolValue(fallbackFilterValue('geoip'), true)" @change="(value: any) => setFallbackFilterValue('geoip', Boolean(value))" /></label></template></div></section>
                <div class="dns-list-grid"><section v-for="name in ['default-nameserver', 'fallback'] as const" :key="name" class="parameter-list"><div class="parameter-list-title">{{ name }}</div><div v-for="(item, index) in dnsList(name)" :key="`${name}-${index}`" class="parameter-list-row"><Input :value="item" @change="(event: any) => setDNSList(name, replaceList(dnsList(name), index, event.target.value))" /><Button size="small" danger @click="setDNSList(name, removeListItem(dnsList(name), index))">删除</Button></div><Button size="small" @click="setDNSList(name, [...dnsList(name), ''])">新增条目</Button></section><section v-for="name in ['geosite', 'ipcidr'] as const" :key="name" class="parameter-list"><div class="parameter-list-title">fallback-filter.{{ name }}</div><div v-for="(item, index) in filterList(name)" :key="`${name}-${index}`" class="parameter-list-row"><Input :value="item" @change="(event: any) => setFilterList(name, replaceList(filterList(name), index, event.target.value))" /><Button size="small" danger @click="setFilterList(name, removeListItem(filterList(name), index))">删除</Button></div><Button size="small" @click="setFilterList(name, [...filterList(name), ''])">新增条目</Button></section></div>
                <Collapse class="dns-filter-collapse"><Collapse.Panel key="fake-ip-filter" :header="`fake-ip-filter（${dnsList('fake-ip-filter').length} 项，默认已预填）`"><section class="parameter-list"><div v-for="(item, index) in dnsList('fake-ip-filter')" :key="`fake-ip-filter-${index}`" class="parameter-list-row"><Input :value="item" @change="(event: any) => setDNSList('fake-ip-filter', replaceList(dnsList('fake-ip-filter'), index, event.target.value))" /><Button size="small" danger @click="setDNSList('fake-ip-filter', removeListItem(dnsList('fake-ip-filter'), index))">删除</Button></div><Button size="small" @click="setDNSList('fake-ip-filter', [...dnsList('fake-ip-filter'), ''])">新增条目</Button></section></Collapse.Panel></Collapse>
              </template>
            </div>
          </Collapse.Panel>

          <Collapse.Panel key="more">
            <template #header><div class="collapse-panel-title"><span>更多参数</span><span>默认已预填</span></div></template>
            <div class="clash-header-section" data-section="more">
              <div class="section-editor-toolbar"><span>运行模式、日志、NTP 和其他未分区的顶层参数</span><label class="flex items-center gap-2 text-xs text-text-secondary whitespace-nowrap"><Switch :checked="advancedSections.more" size="small" @change="(value: any) => setSectionAdvanced('more', Boolean(value))" /><span>高级 JSON</span></label></div>
              <div v-if="advancedSections.more"><Input.TextArea class="section-json-editor" :value="sectionJSON.more" :rows="14" @input="(event: any) => updateSectionJSON('more', event.target.value)" /><div v-if="sectionErrors.more" class="section-json-error">{{ sectionErrors.more }}</div></div>
              <template v-else>
                <div class="header-input-fields grid grid-cols-1 md:grid-cols-2 gap-3"><div><label class="text-sm text-text-secondary">进程匹配模式</label><Select class="w-full" :value="stringValue(rootValue('find-process-mode'), 'strict')" @change="(value: any) => setRootValue('find-process-mode', value)"><Select.Option value="always">always</Select.Option><Select.Option value="strict">strict</Select.Option><Select.Option value="off">off</Select.Option></Select></div><div><label class="text-sm text-text-secondary">工作模式</label><Select class="w-full" :value="stringValue(rootValue('mode'), 'rule')" @change="(value: any) => setRootValue('mode', value)"><Select.Option value="rule">rule</Select.Option><Select.Option value="global">global</Select.Option><Select.Option value="direct">direct</Select.Option></Select></div><div><label class="text-sm text-text-secondary">日志级别</label><Select class="w-full" :value="stringValue(rootValue('log-level'), 'warning')" @change="(value: any) => setRootValue('log-level', value)"><Select.Option value="silent">silent</Select.Option><Select.Option value="error">error</Select.Option><Select.Option value="warning">warning</Select.Option><Select.Option value="info">info</Select.Option><Select.Option value="debug">debug</Select.Option></Select></div><div><label class="text-sm text-text-secondary">常用 NTP 服务器</label><Select class="w-full" :value="ntpPresetValue()" placeholder="选择常用 NTP 服务" @change="(value: any) => setNTPValue('server', value)"><Select.Option v-for="preset in NTP_PRESETS" :key="preset.value" :value="preset.value">{{ preset.label }}</Select.Option></Select></div><div><label class="text-sm text-text-secondary">自定义 NTP 服务器（可选）</label><Input :value="customNTPServer()" placeholder="填写后优先使用" @change="(event: any) => setCustomNTPServer(event.target.value)" /></div><div><label class="text-sm text-text-secondary">NTP 端口</label><InputNumber :value="numberValue(ntpValue('port'), 123)" class="w-full" @change="(value: any) => setNTPValue('port', value ?? 0)" /></div><div><label class="text-sm text-text-secondary">NTP 间隔（分钟）</label><InputNumber :value="numberValue(ntpValue('interval'), 30)" class="w-full" @change="(value: any) => setNTPValue('interval', value ?? 0)" /></div></div>
                <section class="header-switch-fields section-switch-fields"><div class="text-sm font-medium">开关参数</div><div class="section-switch-grid"><label class="section-switch-option"><span>Allow LAN</span><Switch :checked="boolValue(rootValue('allow-lan'), false)" @change="(value: any) => setRootValue('allow-lan', Boolean(value))" /></label><label class="section-switch-option"><span>IPv6</span><Switch :checked="boolValue(rootValue('ipv6'), false)" @change="(value: any) => setRootValue('ipv6', Boolean(value))" /></label><label class="section-switch-option"><span>启用 NTP</span><Switch :checked="boolValue(ntpValue('enable'), true)" @change="(value: any) => setNTPValue('enable', Boolean(value))" /></label><label class="section-switch-option"><span>写入系统时间</span><Switch :checked="boolValue(ntpValue('write-to-system'), false)" @change="(value: any) => setNTPValue('write-to-system', Boolean(value))" /></label></div></section>
              </template>
            </div>
          </Collapse.Panel>
        </Collapse>
      </template>

      <template v-else>
        <Input.TextArea v-if="advancedWhole" v-model:value="form.fixed_params_text" :rows="4" placeholder='{"port":7890,"mode":"rule"}' />
        <div v-else class="space-y-4">
          <div v-if="inputFields.length" class="header-input-fields grid grid-cols-1 md:grid-cols-2 gap-3"><div v-for="field in inputFields" :key="field.name"><label class="text-sm text-text-secondary">{{ field.label }}</label><InputNumber v-if="field.type === 'number'" :value="Number(fieldValue(field.name) ?? field.default ?? 0)" class="w-full" @change="(value: any) => setField(field.name, value ?? 0)" /><Input v-else :value="String(fieldValue(field.name) ?? '')" @change="(event: any) => setField(field.name, event.target.value)" /></div></div>
          <section v-if="switchFields.length" class="header-switch-fields border rounded-lg p-3"><div class="text-sm font-medium mb-2">开关参数</div><div class="grid grid-cols-1 sm:grid-cols-2 gap-3"><label v-for="field in switchFields" :key="field.name" class="flex items-center justify-between gap-3 text-sm text-text-secondary"><span>{{ field.label }}</span><Switch :checked="Boolean(fieldValue(field.name) ?? field.default ?? false)" @change="(value: any) => setField(field.name, value)" /></label></div></section>
        </div>
      </template>
    </div>
  </div>
</template>

<style scoped>
.section-editor-toolbar { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-bottom: 16px; font-size: 12px; color: var(--ui-text-tertiary); }
.clash-header-hint { display: flex; align-items: baseline; flex-wrap: wrap; gap: 4px 8px; margin-bottom: 12px; padding: 9px 12px; border: 1px solid color-mix(in srgb, var(--ui-primary) 30%, var(--ui-border)); border-radius: 8px; background: color-mix(in srgb, var(--ui-primary) 5%, transparent); font-size: 12px; color: var(--ui-text-tertiary); }
.clash-header-hint-title { color: var(--ui-text); font-weight: 600; }
.collapse-panel-title { display: flex; align-items: center; gap: 8px; }
.collapse-panel-title span:last-child { padding: 1px 6px; border-radius: 999px; background: var(--ui-fill); color: var(--ui-text-tertiary); font-size: 11px; font-weight: 400; }
.field-description { margin: 2px 0 4px; font-size: 12px; color: var(--ui-text-tertiary); }
.section-switch-fields { margin-top: 16px; border: 1px solid var(--ui-border); border-radius: 8px; padding: 12px; display: grid; gap: 10px; }
.section-switch-grid { display: flex; flex-wrap: wrap; gap: 10px 24px; }
.section-switch-option { display: inline-flex; align-items: center; gap: 8px; color: var(--ui-text-secondary); font-size: 14px; }
.section-json-editor { font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace; }
.section-json-error { margin-top: 4px; font-size: 12px; color: var(--ui-danger); }
.dns-list-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 16px; margin-top: 16px; }
.dns-filter-collapse { margin-top: 16px; }
.parameter-list { border: 1px solid var(--ui-border); border-radius: 8px; padding: 12px; }
.parameter-list-title { margin-bottom: 10px; font-size: 13px; font-weight: 500; }
.parameter-list-row { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 8px; margin-bottom: 8px; }
@media (max-width: 767px) { .section-editor-toolbar { align-items: flex-start; flex-direction: column; } .dns-list-grid { grid-template-columns: minmax(0, 1fr); } .clash-header-hint { align-items: flex-start; flex-direction: column; gap: 2px; } }
</style>
