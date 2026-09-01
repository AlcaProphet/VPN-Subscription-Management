<!-- ContextBar.vue：跨页任务上下文提示（Build11 Step 2） -->
<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Alert, Button } from 'ant-design-vue'
import { readAssemblyDraft, type AssemblyDraft } from '@/utils/assemblyDraft'

const route = useRoute()
const router = useRouter()
const draft = ref<AssemblyDraft | null>(null)

function refresh() {
  draft.value = readAssemblyDraft()
}

function returnToAssembly() {
  if (!draft.value) return
  void router.push(draft.value.returnPath)
}

onMounted(refresh)
// AdminLayout 不会在管理路由切换时重新挂载，需随路径刷新 sessionStorage 中的新草稿。
watch(() => route.fullPath, refresh, { immediate: true })
</script>

<template>
  <Alert v-if="draft && route.path !== draft.returnPath" type="info" show-icon class="mb-4"
         :message="`正在补充构建前置条件：${draft.sourceLabel}`"
         description="已保存当前订阅装配草稿，完成后可返回继续。">
    <template #action>
      <Button size="small" type="primary" @click="returnToAssembly">返回装配</Button>
    </template>
  </Alert>
</template>
