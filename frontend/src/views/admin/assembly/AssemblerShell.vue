<!-- AssemblerShell.vue：装配器分步/单页双形态外壳（Design2-UI §5.3.0） -->
<script setup lang="ts">
import { Button, Card, Segmented, Steps } from 'ant-design-vue'

defineProps<{
  layoutMode: 'step' | 'page'
  stepDefs: Array<{ key: string; title: string }>
  currentStep: number
  currentStepKey: string
  hasHeaderStep: boolean
  hasNodesStep: boolean
  hasRulesStep: boolean
  generating: boolean
}>()

const emit = defineEmits<{
  'update:layoutMode': [value: string]
  'update:currentStep': [value: number]
  next: []
  prev: []
  generate: []
}>()

const stepTitles: Record<string, string> = {
  header: '① 头部表单',
  nodes: '② 节点与代理组',
  rules: '③ 规则素材',
  preview: '④ 预览',
}
</script>

<template>
  <div class="space-y-4">
    <div class="flex items-center justify-between flex-wrap gap-2">
      <Segmented
        :value="layoutMode"
        :options="[{ label: '分步', value: 'step' }, { label: '单页', value: 'page' }]"
        @change="(v: any) => emit('update:layoutMode', String(v))"
      />
      <span class="text-xs text-gray-400">四类装配器共用同一表单状态，切换不丢失</span>
    </div>

    <Steps v-if="layoutMode === 'step'" :current="currentStep" size="small" class="mb-4">
      <Steps.Step v-for="s in stepDefs" :key="s.key" :title="s.title" />
    </Steps>

    <div v-if="hasHeaderStep" v-show="layoutMode === 'page' || currentStepKey === 'header'">
      <Card v-if="layoutMode === 'page'" :title="stepTitles.header" size="small" class="mb-3">
        <slot name="header" />
      </Card>
      <div v-else><slot name="header" /></div>
    </div>

    <div v-if="hasNodesStep" v-show="layoutMode === 'page' || currentStepKey === 'nodes'">
      <Card v-if="layoutMode === 'page'" :title="stepTitles.nodes" size="small" class="mb-3">
        <slot name="nodes" />
      </Card>
      <div v-else><slot name="nodes" /></div>
    </div>

    <div v-if="hasRulesStep" v-show="layoutMode === 'page' || currentStepKey === 'rules'">
      <Card v-if="layoutMode === 'page'" :title="stepTitles.rules" size="small" class="mb-3">
        <slot name="rules" />
      </Card>
      <div v-else><slot name="rules" /></div>
    </div>

    <div v-show="layoutMode === 'page' || currentStepKey === 'preview'">
      <Card v-if="layoutMode === 'page'" :title="stepTitles.preview" size="small" class="mb-3">
        <slot name="preview" />
      </Card>
      <div v-else><slot name="preview" /></div>
    </div>

    <div v-if="layoutMode === 'step'" class="flex items-center justify-between">
      <Button :disabled="currentStep === 0" @click="emit('prev')">上一步</Button>
      <Button v-if="currentStep < stepDefs.length - 1" type="primary" @click="emit('next')">下一步</Button>
      <Button v-else type="primary" :loading="generating" @click="emit('generate')">确认生成</Button>
    </div>
    <div v-if="layoutMode === 'page'" class="flex justify-end">
      <Button type="primary" :loading="generating" @click="emit('generate')">确认生成</Button>
    </div>
  </div>
</template>
