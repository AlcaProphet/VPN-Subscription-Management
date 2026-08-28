<!-- RulesStep.vue：装配步骤④ 规则素材（Design2-UI §5.3.1/5.3.3） -->
<script setup lang="ts">
import { ref } from 'vue'
import { Button, Input, Radio, Select } from 'ant-design-vue'
import type { AssemblyContext, TargetSyntax } from '@/api/assembly'

defineProps<{
  form: {
    pools: Array<{ pool_id: number; target: string }>
    custom_rules: Array<{ rule_type: string; match_value: string; target: string }>
  }
  context: AssemblyContext | null
  targetSyntax: TargetSyntax
  outputGroups: string[]
  ruleTypeOptions: string[]
}>()

const emit = defineEmits<{
  'add-pool': []
  'move-pool': [from: number, to: number]
  'remove-pool': [index: number]
  'add-rule': []
  'remove-rule': [index: number]
}>()

// 素材池有序列表桌面拖拽（R14-15）：使用原生 HTML5 DnD，移动端仍走按钮
const dragIndex = ref<number | null>(null)
function onDragStart(idx: number) {
  dragIndex.value = idx
}
function onDrop(idx: number) {
  if (dragIndex.value !== null && dragIndex.value !== idx) {
    emit('move-pool', dragIndex.value, idx)
  }
  dragIndex.value = null
}
</script>

<template>
  <div class="space-y-3">
    <div>
      <div class="text-sm font-medium mb-1">已勾选素材池（有序）</div>
      <div v-if="form.pools.length === 0" class="text-xs text-text-tertiary">尚未添加素材池</div>
      <div v-for="(p, idx) in form.pools" :key="idx" :draggable="true" class="flex items-center gap-2 mb-2 cursor-move"
           @dragstart="onDragStart(idx)" @dragover.prevent @drop="onDrop(idx)">
        <AppSelect :value="p.pool_id" class="w-48" @change="(v: any) => form.pools[idx] = { ...form.pools[idx], pool_id: Number(v) }">
          <Select.Option v-for="pool in context?.pools ?? []" :key="pool.id" :value="pool.id">{{ pool.name }}</Select.Option>
        </AppSelect>
        <AppSelect v-if="targetSyntax !== 'sr-conf'" :value="p.target" class="flex-1" @change="(v: any) => form.pools[idx] = { ...form.pools[idx], target: String(v) }">
          <Select.Option v-for="g in outputGroups" :key="g" :value="g">{{ g }}</Select.Option>
        </AppSelect>
        <Radio.Group v-else :value="p.target" @change="(e: any) => form.pools[idx] = { ...form.pools[idx], target: String(e.target.value) }">
          <Radio value="PROXY">PROXY</Radio>
          <Radio value="DIRECT">DIRECT</Radio>
        </Radio.Group>
        <Button size="small" :disabled="idx === 0" @click="emit('move-pool', idx, idx - 1)">上移</Button>
        <Button size="small" :disabled="idx === form.pools.length - 1" @click="emit('move-pool', idx, idx + 1)">下移</Button>
        <Button size="small" danger @click="emit('remove-pool', idx)">移除</Button>
      </div>
      <Button size="small" @click="emit('add-pool')">添加素材池</Button>
    </div>
    <div>
      <div class="text-sm font-medium mb-1">手动规则行</div>
      <div v-for="(r, idx) in form.custom_rules" :key="idx" class="flex items-center gap-2 mb-2">
        <AppSelect v-model:value="r.rule_type" class="w-48" :options="ruleTypeOptions.map((t) => ({ label: t, value: t }))" />
        <Input v-model:value="r.match_value" placeholder="匹配值" class="flex-1" />
        <AppSelect v-if="targetSyntax !== 'sr-conf'" v-model:value="r.target" class="w-40" placeholder="目标组">
          <Select.Option v-for="g in outputGroups" :key="g" :value="g">{{ g }}</Select.Option>
        </AppSelect>
        <Radio.Group v-else v-model:value="r.target">
          <Radio value="PROXY">PROXY</Radio>
          <Radio value="DIRECT">DIRECT</Radio>
        </Radio.Group>
        <Button size="small" danger @click="emit('remove-rule', idx)">删除</Button>
      </div>
      <Button size="small" @click="emit('add-rule')">添加规则行</Button>
    </div>
    <div v-if="targetSyntax === 'clash-yaml'" class="text-xs text-text-tertiary">USER-AGENT 规则在 Clash 中不支持，已从类型下拉排除。</div>
  </div>
</template>
