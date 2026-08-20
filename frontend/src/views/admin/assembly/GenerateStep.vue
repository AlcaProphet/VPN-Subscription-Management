<!-- GenerateStep.vue：装配步骤⑥ 确认生成（Design2-UI §5.3.0） -->
<script setup lang="ts">
import { Alert, Button } from 'ant-design-vue'

defineProps<{
  invalidCount: number
  clashEmptyOverseas: boolean
  generating: boolean
}>()

const emit = defineEmits<{ generate: [] }>()
</script>

<template>
  <div class="space-y-3">
    <Alert v-if="invalidCount > 0" type="error" show-icon message="存在失效引用，请先剔除后生成" />
    <Alert v-if="clashEmptyOverseas" type="error" show-icon message="「🌎国外流量」组未包含任何节点" />
    <Button type="primary" :loading="generating" @click="emit('generate')">生成版本</Button>
  </div>
</template>
