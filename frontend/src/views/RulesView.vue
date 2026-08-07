<!-- RulesView.vue：规则浏览页（UI §4.2）——规则卡片网格 + 下载/一键导入 -->
<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { Alert, Button, Card, Modal, Tag, TypographyText } from 'ant-design-vue'
import { userRules, previewRule, type RuleItem } from '@/api/rule'
import { buildImportUrl } from '@/utils/importUrl'
import { Notify } from '@/components/Notify'

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

// 一键导入（全局规则 Token）：schemes 首项 + encodeURIComponent(下载 URL)
function oneClickImport(r: RuleItem) {
  const scheme = r.schemes?.[0]
  if (!scheme) {
    Notify.warning('该规则未配置导入 scheme')
    return
  }
  const url = `${location.origin}/rules/${r.slug}/download?token=${r.token}`
  window.location.href = buildImportUrl(scheme, url)
  setTimeout(() => Notify.info('请确认已安装对应客户端'), 3000)
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
  <div class="min-h-screen bg-gray-50 dark:bg-gray-900">
    <main class="max-w-6xl mx-auto p-4">
      <div class="flex items-center justify-between mb-4">
        <h2 class="text-lg font-semibold m-0">分流规则</h2>
      </div>

      <Alert type="warning" show-icon class="mb-4"
             message="规则内容公开，链接请谨慎分发，请勿外发" />

      <div v-if="loading" class="text-center py-16 text-gray-400">加载中…</div>
      <div v-else-if="rules.length === 0" class="text-center py-16 text-gray-400">还没有规则</div>

      <div v-else class="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
        <Card v-for="r in rules" :key="r.id" class="shadow-sm">
          <div class="flex items-center justify-between">
            <span class="font-medium">{{ r.name }}</span>
            <Tag color="blue">{{ r.client_type }}</Tag>
          </div>
          <TypographyText code class="text-xs text-gray-400">{{ r.slug }}</TypographyText>
          <div class="text-xs text-gray-500 mt-1">当前版本 v{{ r.current_version || '—' }}</div>
          <div class="mt-3 flex gap-2">
            <Button size="small" @click="openPreview(r)">下载</Button>
            <Button size="small" type="primary" :disabled="!r.schemes?.length" @click="oneClickImport(r)">一键导入</Button>
          </div>
        </Card>
      </div>
    </main>

    <!-- 预览弹窗（会话凭据，纯文本） -->
    <Modal :open="previewOpen" :title="`${previewName} 内容`" width="80%" :footer="null" @cancel="previewOpen = false">
      <pre class="text-xs overflow-auto max-h-[70vh] whitespace-pre-wrap break-all">{{ previewContent }}</pre>
    </Modal>
  </div>
</template>
