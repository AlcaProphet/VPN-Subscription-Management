<!-- RulesView.vue：规则浏览页（UI §4.2）——规则卡片网格 + 下载/复制链接 -->
<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ArrowLeftOutlined } from '@ant-design/icons-vue'
import { Alert, Button, Card, Tag, TypographyText } from 'ant-design-vue'
import AppModal from '@/components/AppModal.vue'
import { userRules, previewRule, type RuleItem } from '@/api/rule'
import { Notify } from '@/components/Notify'

const router = useRouter()

const loading = ref(true)
const rules = ref<RuleItem[]>([])
async function load() {
  loading.value = true
  try {
    rules.value = await userRules()
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    loading.value = false
  }
}
onMounted(load)

// 复制链接（全局规则 Token，内容公开；Shadowrocket App 移除 scheme 唤起后一键导入 UI 下线，后端保留）
async function copyLink(r: RuleItem) {
  if (!r.token) return
  const url = `${location.origin}/rules/${r.slug}/download?token=${r.token}`
  try {
    await navigator.clipboard.writeText(url)
    Notify.success('链接已复制（规则内容公开，请谨慎分发）')
  } catch {
    Notify.error('复制失败，请手动复制')
  }
}

// 预览弹窗
const previewOpen = ref(false)
const previewContent = ref('')
const previewName = ref('')
async function openPreview(r: RuleItem) {
  previewName.value = r.name
  previewOpen.value = true
  try {
    previewContent.value = await previewRule(r.id)
  } catch (err) {
    Notify.error((err as Error).message)
  }
}
</script>

<template>
  <div class="min-h-screen bg-page">
    <main class="max-w-6xl mx-auto p-4">
      <div class="flex items-center gap-2 mb-4">
        <Button type="text" @click="router.push('/')">
          <template #icon><ArrowLeftOutlined /></template>
          返回主界面
        </Button>
        <h2 class="text-lg font-semibold m-0">分流规则</h2>
      </div>

      <Alert type="warning" show-icon class="mb-4"
             message="规则内容公开，链接请谨慎分发，请勿外发" />

      <div v-if="loading" class="text-center py-16 text-text-tertiary">加载中…</div>
      <div v-else-if="rules.length === 0" class="text-center py-16 text-text-tertiary">还没有规则</div>

      <div v-else class="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
        <Card v-for="r in rules" :key="r.id" class="shadow-sm">
          <div class="flex items-center justify-between">
            <span class="font-medium">{{ r.name }}</span>
            <Tag color="blue">{{ r.client_type }}</Tag>
          </div>
          <TypographyText code class="text-xs text-text-tertiary">{{ r.slug }}</TypographyText>
          <div class="text-xs text-text-secondary mt-1">当前版本 v{{ r.current_version || '—' }}</div>
          <div class="mt-3 flex gap-2">
            <Button size="small" @click="openPreview(r)">下载</Button>
            <Button size="small" type="primary" :disabled="!r.token" @click="copyLink(r)">复制链接</Button>
          </div>
        </Card>
      </div>
    </main>

    <!-- 预览弹窗（会话凭据，纯文本） -->
    <AppModal :open="previewOpen" :title="`${previewName} 内容`" width="80%" :footer="null" @update:open="previewOpen = $event">
      <pre class="text-xs overflow-auto max-h-[70vh] whitespace-pre-wrap break-all">{{ previewContent }}</pre>
    </AppModal>
  </div>
</template>
