<!-- HeaderStep.vue：装配步骤② 头部表单（Design2-UI §5.3.1/5.3.2/5.3.3） -->
<script setup lang="ts">
import { Button, Input } from 'ant-design-vue'
import type { TargetSyntax } from '@/api/assembly'

defineProps<{
  form: { fixed_params_text: string }
  targetSyntax: TargetSyntax
}>()

const emit = defineEmits<{ 'apply-default': [] }>()
</script>

<template>
  <div class="space-y-2">
    <div v-if="targetSyntax === 'generic-subs'" class="text-sm text-gray-400">
      通用节点订阅不输出 STATUS/REMARKS 头部，本步无需填写。
    </div>
    <div v-else>
      <div class="flex items-center justify-between mb-1">
        <span class="text-sm">头部参数（JSON）</span>
        <Button size="small" @click="emit('apply-default')">一键采用默认值</Button>
      </div>
      <Input.TextArea v-model:value="form.fixed_params_text" :rows="4" placeholder='{"port":7890,"mode":"rule"}' />
    </div>
  </div>
</template>
