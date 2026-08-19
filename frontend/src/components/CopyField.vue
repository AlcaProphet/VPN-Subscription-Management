<!-- CopyField.vue：Token/链接/标识展示 + 一键复制（Design2-UI §1.3） -->
<script setup lang="ts">
import { Button, Input, Tooltip } from 'ant-design-vue'
import { CopyOutlined } from '@ant-design/icons-vue'
import { Notify } from '@/components/Notify'

const props = withDefaults(defineProps<{
  value: string
  label?: string
  warning?: string // 敏感链接复制警示（如「该链接与您的账号绑定，请勿分享」）
}>(), { label: '', warning: '' })

async function copy() {
  try {
    await navigator.clipboard.writeText(props.value)
    Notify.success(props.warning ? `已复制（${props.warning}）` : '已复制')
  } catch {
    Notify.error('复制失败，请手动复制')
  }
}
</script>

<template>
  <div class="flex items-center gap-1">
    <Input :value="value" readonly size="small" :addon-before="label || undefined" class="!w-auto min-w-40">
      <template #suffix>
        <Tooltip title="复制">
          <Button type="text" size="small" @click="copy">
            <template #icon><CopyOutlined /></template>
          </Button>
        </Tooltip>
      </template>
    </Input>
  </div>
</template>
