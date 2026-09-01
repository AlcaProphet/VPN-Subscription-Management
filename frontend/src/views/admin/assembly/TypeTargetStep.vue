<!-- TypeTargetStep.vue：装配顶部常驻目标选择（R22-03/R22-08；原“类型与目标”步骤已移除） -->
<script setup lang="ts">
import { Input, Radio, Select } from 'ant-design-vue'
import type { AssemblyContext } from '@/api/assembly'
import type { PlatformItem } from '@/api/platform'

defineProps<{
  form: {
    platform_id?: number
    rule_id?: number
    rule_name: string
    sr_rule_mode: 'existing' | 'new'
    final_direction: string
  }
  context: AssemblyContext | null
  isSrConf: boolean
  filteredPlatforms: PlatformItem[]
}>()
</script>

<template>
  <div class="grid md:grid-cols-2 gap-4">
    <div>
      <div class="text-sm mb-1">{{ isSrConf ? 'SR 分流规则' : '目标平台' }}</div>
      <template v-if="isSrConf">
        <Radio.Group v-model:value="form.sr_rule_mode" button-style="solid" class="mb-2">
          <Radio.Button value="new">新建规则</Radio.Button>
          <Radio.Button value="existing">选择已有规则</Radio.Button>
        </Radio.Group>
        <Input v-if="form.sr_rule_mode === 'new'" v-model:value="form.rule_name" placeholder="请输入新规则名称" class="w-full" allow-clear />
        <AppSelect v-else :value="form.rule_id" placeholder="选择已有规则实体" class="w-full" @change="(v: any) => form.rule_id = Number(v)">
          <Select.Option v-for="r in context?.rules ?? []" :key="r.id" :value="r.id">
            {{ r.name }}<span v-if="r.current_version <= 0" class="text-xs text-text-tertiary">（空实体）</span>
          </Select.Option>
        </AppSelect>
        <div v-if="form.sr_rule_mode === 'existing' && (context?.rules ?? []).length === 0" class="text-xs text-text-secondary mt-2">
          暂无规则实体，可切换到“新建规则”直接创建
        </div>
      </template>
      <template v-else>
        <AppSelect :value="form.platform_id" placeholder="选择平台" class="w-full" @change="(v: any) => form.platform_id = Number(v)">
          <Select.Option v-for="p in filteredPlatforms" :key="p.id" :value="p.id">{{ p.name }}（{{ p.product_type }}）</Select.Option>
        </AppSelect>
        <div v-if="filteredPlatforms.length === 0" class="text-xs text-text-secondary mt-2">
          暂无匹配格式的平台，请先创建对应格式平台：<a href="/admin/platforms">前往平台管理</a>
        </div>
      </template>
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
