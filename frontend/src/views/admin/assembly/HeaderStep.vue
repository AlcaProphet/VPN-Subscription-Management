<!-- HeaderStep.vue：装配步骤② 头部表单（Design2-UI §5.3.1/5.3.2/5.3.3）
     按平台类型渲染结构化字段；保留“高级 JSON”切换供高级用户微调。 -->
<script setup lang="ts">
import { computed, ref } from 'vue'
import { Button, Input, InputNumber, Space, Switch } from 'ant-design-vue'
import type { TargetSyntax } from '@/api/assembly'

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

const headerFields = computed<HeaderField[]>(() => {
  switch (props.targetSyntax) {
    case 'clash-yaml':
      return [
        { name: 'port', label: 'Port', type: 'number', default: 7890 },
        { name: 'socks-port', label: 'SOCKS Port', type: 'number', default: 7891 },
        { name: 'allow-lan', label: 'Allow LAN', type: 'bool', default: false },
        { name: 'mode', label: 'Mode', type: 'text', default: 'rule' },
        { name: 'log-level', label: 'Log Level', type: 'text', default: 'info' },
      ]
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

const advanced = ref(false)

function parseObject(): Record<string, unknown> {
  try {
    const v = JSON.parse(props.form.fixed_params_text || '{}')
    return v && typeof v === 'object' ? v : {}
  } catch {
    return {}
  }
}

function fieldValue(name: string): unknown {
  const v = parseObject()[name]
  if (v !== undefined) return v
  return headerFields.value.find((f) => f.name === name)?.default
}

function setField(name: string, val: unknown) {
  const obj = parseObject()
  obj[name] = val
  props.form.fixed_params_text = JSON.stringify(obj, null, 2)
}
</script>

<template>
  <div class="space-y-2">
    <div v-if="targetSyntax === 'generic-subs'" class="text-sm text-text-tertiary">
      通用节点订阅不输出 STATUS/REMARKS 头部，本步无需填写。
    </div>
    <div v-else>
      <div class="flex items-center justify-between mb-1">
        <span class="text-sm">{{ advanced ? '头部参数（JSON）' : '头部参数' }}</span>
        <Space>
          <Button size="small" @click="emit('apply-default')">一键采用默认值</Button>
          <Button size="small" @click="advanced = !advanced">{{ advanced ? '结构化表单' : '高级 JSON' }}</Button>
        </Space>
      </div>

      <Input.TextArea v-if="advanced" v-model:value="form.fixed_params_text" :rows="4" placeholder='{"port":7890,"mode":"rule"}' />

      <div v-else class="space-y-4">
        <div v-if="inputFields.length" class="header-input-fields grid grid-cols-1 md:grid-cols-2 gap-3">
          <div v-for="f in inputFields" :key="f.name">
            <label class="text-sm text-text-secondary">{{ f.label }}</label>
            <InputNumber v-if="f.type === 'number'" :value="Number(fieldValue(f.name) ?? f.default ?? 0)" class="w-full"
                         @change="(v: any) => setField(f.name, v ?? 0)" />
            <Input v-else :value="String(fieldValue(f.name) ?? '')" @change="(e: any) => setField(f.name, e.target.value)" />
          </div>
        </div>
        <section v-if="switchFields.length" class="header-switch-fields border rounded-lg p-3">
          <div class="text-sm font-medium mb-2">开关参数</div>
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <label v-for="f in switchFields" :key="f.name" class="flex items-center justify-between gap-3 text-sm text-text-secondary">
              <span>{{ f.label }}</span>
              <Switch :checked="Boolean(fieldValue(f.name) ?? f.default ?? false)" @change="(v: any) => setField(f.name, v)" />
            </label>
          </div>
        </section>
      </div>
    </div>
  </div>
</template>
