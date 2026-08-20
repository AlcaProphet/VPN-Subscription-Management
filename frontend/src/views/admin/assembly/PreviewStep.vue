<!-- PreviewStep.vue：装配步骤⑤ 预览与 Diff（Design2-UI §5.3.5） -->
<script setup lang="ts">
import { Alert, Button, Space } from 'ant-design-vue'
import DiffView from '@/components/DiffView.vue'

defineProps<{
  previewing: boolean
  previewWarnings: string[]
  previewSkipped: Array<{ name: string; reason: string }>
  previewText: string
  showDiff: boolean
  diffOld: string
  diffMissing: boolean
  diffLoading: boolean
}>()

const emit = defineEmits<{ preview: []; 'toggle-diff': [] }>()
</script>

<template>
  <div class="space-y-3">
    <Space>
      <Button type="primary" :loading="previewing" @click="emit('preview')">预览产物</Button>
      <Button :loading="diffLoading" @click="emit('toggle-diff')">与当前激活版本对比</Button>
    </Space>
    <Alert v-for="(w, i) in previewWarnings" :key="i" type="warning" show-icon :message="w" />
    <Alert v-for="(s, i) in previewSkipped" :key="'s'+i" type="warning" show-icon :message="`跳过 ${s.name}：${s.reason}`" />
    <div v-if="previewText">
      <h3 class="font-semibold mb-2">预览</h3>
      <pre class="bg-gray-50 dark:bg-gray-900 rounded p-3 text-xs overflow-auto max-h-[50vh] whitespace-pre-wrap">{{ previewText }}</pre>
    </div>
    <DiffView v-if="showDiff && previewText" :old-text="diffOld" :new-text="previewText" :target-missing="diffMissing" />
  </div>
</template>
