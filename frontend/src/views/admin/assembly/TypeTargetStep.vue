<!-- TypeTargetStep.vue：装配步骤① 类型与目标（Design2-UI §5.3.0/5.3.4） -->
<script setup lang="ts">
import { Radio, Select } from 'ant-design-vue'
import type { AssemblyContext } from '@/api/assembly'
import type { PlatformItem } from '@/api/platform'

defineProps<{
  form: { platform_id?: number; rule_id?: number; final_direction: string }
  context: AssemblyContext | null
  isSrConf: boolean
  filteredPlatforms: PlatformItem[]
}>()
</script>

<template>
  <div class="grid md:grid-cols-2 gap-4">
    <div>
      <div class="text-sm mb-1">{{ isSrConf ? '规则实体' : '目标平台' }}</div>
      <Select v-if="isSrConf" :value="form.rule_id" placeholder="选择规则实体" class="w-full" @change="(v: any) => form.rule_id = Number(v)">
        <Select.Option v-for="r in context?.rules ?? []" :key="r.id" :value="r.id">
          {{ r.name }}<span v-if="r.current_version <= 0" class="text-xs text-gray-400">（空实体）</span>
        </Select.Option>
      </Select>
      <Select v-else :value="form.platform_id" placeholder="选择平台" class="w-full" @change="(v: any) => form.platform_id = Number(v)">
        <Select.Option v-for="p in filteredPlatforms" :key="p.id" :value="p.id">{{ p.name }}（{{ p.product_type }}）</Select.Option>
      </Select>
    </div>
    <div v-if="isSrConf">
      <div class="text-sm mb-1">FINAL 方向</div>
      <Radio.Group :value="form.final_direction" class="w-full" @change="(e: any) => form.final_direction = String(e.target.value)">
        <Radio value="PROXY">PROXY</Radio>
        <Radio value="DIRECT">DIRECT</Radio>
      </Radio.Group>
    </div>
  </div>
</template>
