<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { Alert, Button, Card, Input, Modal, Radio, Tag } from 'ant-design-vue'
import { Notify } from '@/components/Notify'
import {
  getMailTemplates, saveMailTemplate, restoreMailTemplate, previewMailTemplate,
  type MailTemplateKind, type MailTemplatePreview, type MailTemplateState, type MailTemplateView,
} from '@/api/settings'

const emit = defineEmits<{
  (e: 'dirty-change', dirty: boolean): void
}>()

const templates = ref<MailTemplateView[]>([])
const limits = reactive({ subject: 200, body: 10000 })
const selectedKind = ref<MailTemplateKind>('password_reset')
const draft = reactive({ subject: '', body: '' })
const baseline = reactive({ subject: '', body: '' })
const state = ref<MailTemplateState>('default')
const warning = ref('')
const subjectVariables = ref<string[]>([])
const bodyVariables = ref<string[]>([])
const requiredBodyVariables = ref<string[]>([])
const loading = ref(false)
const loadError = ref('')
const saving = ref(false)
const restoring = ref(false)
const preview = ref<MailTemplatePreview | null>(null)
const previewMode = ref<'html' | 'text'>('html')
const previewError = ref('')
const lastFocusedField = ref<'subject' | 'body' | ''>('')
const subjectRef = ref<any>(null)
const bodyRef = ref<any>(null)
const ready = ref(false)

let previewTimer: ReturnType<typeof setTimeout> | undefined
let previewSeq = 0
let applying = false

const templateOptions = computed(() => templates.value.map((item) => ({ value: item.id, label: item.label })))
const dirty = computed(() => ready.value && (draft.subject !== baseline.subject || draft.body !== baseline.body))
const subjectLength = computed(() => Array.from(draft.subject || '').length)
const bodyLength = computed(() => Array.from(draft.body || '').length)
const requiredSet = computed(() => new Set(requiredBodyVariables.value))

watch(dirty, (value) => {
  if (ready.value) emit('dirty-change', value)
})

watch(() => [draft.subject, draft.body], () => {
  if (!ready.value || applying) return
  previewError.value = ''
  schedulePreview()
})

function clearPreviewTimer() {
  if (previewTimer !== undefined) {
    clearTimeout(previewTimer)
    previewTimer = undefined
  }
}

function schedulePreview() {
  clearPreviewTimer()
  previewTimer = setTimeout(() => {
    previewTimer = undefined
    void runPreview()
  }, 300)
}

async function runPreview() {
  if (!ready.value || applying) return
  clearPreviewTimer()
  const kindAtRequest = selectedKind.value
  const snapshot = { subject: draft.subject, body: draft.body }
  const seq = ++previewSeq
  preview.value = null
  previewError.value = ''
  try {
    const result = await previewMailTemplate(kindAtRequest, snapshot)
    if (seq !== previewSeq || kindAtRequest !== selectedKind.value) return
    if (snapshot.subject !== draft.subject || snapshot.body !== draft.body) return
    preview.value = result
  } catch (err) {
    if (seq === previewSeq) {
      preview.value = null
      previewError.value = (err as Error)?.message || '邮件内容预览失败'
    }
  }
}

function refreshPreview() {
  clearPreviewTimer()
  void runPreview()
}

function refreshPreviewAfterApply() {
  void nextTick(() => {
    refreshPreview()
  })
}

function applyTemplate(template: MailTemplateView) {
  applying = true
  selectedKind.value = template.id
  baseline.subject = template.subject
  baseline.body = template.body
  draft.subject = template.subject
  draft.body = template.body
  state.value = template.state
  warning.value = template.warning || ''
  subjectVariables.value = [...(template.subject_variables || [])]
  bodyVariables.value = [...(template.body_variables || [])]
  requiredBodyVariables.value = [...(template.required_body_variables || [])]
  preview.value = null
  previewError.value = ''
  void nextTick(() => {
    applying = false
  })
}

function applyView(view: MailTemplateView) {
  const index = templates.value.findIndex((item) => item.id === view.id)
  if (index >= 0) templates.value[index] = view
  else templates.value.push(view)
  applyTemplate(view)
  ready.value = true
  emit('dirty-change', false)
}

async function loadTemplates() {
  loading.value = true
  loadError.value = ''
  try {
    const result = await getMailTemplates()
    templates.value = result.templates || []
    limits.subject = result.limits?.subject ?? 200
    limits.body = result.limits?.body ?? 10000
    const first = templates.value.find((item) => item.id === selectedKind.value) || templates.value[0]
    if (!first) throw new Error('未返回邮件模板')
    applyTemplate(first)
    ready.value = true
    emit('dirty-change', false)
    refreshPreviewAfterApply()
  } catch (err) {
    loadError.value = (err as Error)?.message || '加载邮件模板失败'
  } finally {
    loading.value = false
  }
}

function switchTemplate(kind: MailTemplateKind) {
  if (kind === selectedKind.value) return
  const target = templates.value.find((item) => item.id === kind)
  if (!target) return
  if (dirty.value) {
    Modal.confirm({
      title: '丢弃未保存的邮件内容？',
      content: '切换分支会丢弃当前分支尚未保存的草稿。',
      okText: '丢弃并切换',
      cancelText: '继续编辑',
      onOk: () => {
        applyTemplate(target)
        refreshPreviewAfterApply()
      },
    })
    return
  }
  applyTemplate(target)
  refreshPreviewAfterApply()
}

async function save() {
  if (saving.value) return
  clearPreviewTimer()
  previewSeq++
  saving.value = true
  try {
    const view = await saveMailTemplate(selectedKind.value, { subject: draft.subject, body: draft.body })
    applyView(view)
    Notify.success('邮件模板已保存')
    refreshPreviewAfterApply()
  } catch (err) {
    Notify.error((err as Error)?.message || '邮件模板保存失败')
  } finally {
    saving.value = false
  }
}

function confirmRestore() {
  if (state.value === 'default' || restoring.value) return
  Modal.confirm({
    title: '恢复默认邮件内容？',
    content: '将删除当前分支的自定义内容并恢复内置默认文案。',
    okText: '恢复默认',
    cancelText: '取消',
    onOk: async () => {
      restoring.value = true
      try {
        const view = await restoreMailTemplate(selectedKind.value)
        applyView(view)
        Notify.success('已恢复默认邮件内容')
        refreshPreviewAfterApply()
      } catch (err) {
        Notify.error((err as Error)?.message || '恢复默认失败')
      } finally {
        restoring.value = false
      }
    },
  })
}

function getSubjectInput(): HTMLInputElement | null {
  const component: any = subjectRef.value
  return component?.input ?? component?.$el?.querySelector?.('input') ?? null
}

function getBodyTextarea(): HTMLTextAreaElement | null {
  const component: any = bodyRef.value
  return component?.resizableTextArea?.textArea ?? component?.$el?.querySelector?.('textarea') ?? null
}

function variableToken(variable: string) {
  return `{{${variable}}}`
}

function insertVariable(field: 'subject' | 'body', variable: string) {
  const token = variableToken(variable)
  const current = field === 'subject' ? draft.subject : draft.body
  const element = field === 'subject' ? getSubjectInput() : getBodyTextarea()
  let start = current.length
  let end = current.length
  if (element && lastFocusedField.value === field) {
    const selectionStart = (element as HTMLInputElement).selectionStart
    const selectionEnd = (element as HTMLInputElement).selectionEnd
    if (typeof selectionStart === 'number' && typeof selectionEnd === 'number') {
      start = selectionStart
      end = selectionEnd
    }
  }
  const next = current.slice(0, start) + token + current.slice(end)
  if (field === 'subject') draft.subject = next
  else draft.body = next
  void nextTick(() => {
    if (!element) return
    element.focus()
    if (typeof element.setSelectionRange === 'function') {
      const cursor = start + token.length
      element.setSelectionRange(cursor, cursor)
    }
  })
}

onMounted(() => {
  void loadTemplates()
})

onBeforeUnmount(() => {
  clearPreviewTimer()
  previewSeq++
})

defineExpose({
  loadTemplates,
  switchTemplate,
  insertVariable,
  runPreview,
  save,
  confirmRestore,
  draft,
  selectedKind,
  dirty,
  preview,
})
</script>

<template>
  <Card id="mail-templates" title="邮件内容" size="small">
    <div v-if="loading && !ready" class="py-6 text-center text-sm text-text-secondary">正在加载邮件模板…</div>
    <div v-else-if="loadError" class="space-y-2">
      <Alert type="error" show-icon :message="loadError" />
      <Button size="small" @click="loadTemplates">重试</Button>
    </div>
    <div v-else class="space-y-4">
      <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div class="flex flex-wrap items-center gap-2">
          <span class="text-sm">模板分支</span>
          <AppSelect :value="selectedKind" :options="templateOptions" class="min-w-[180px]" @change="switchTemplate" />
          <Tag v-if="state === 'customized'" color="blue">已自定义</Tag>
          <Tag v-else-if="state === 'damaged'" color="warning">配置损坏</Tag>
          <Tag v-else>默认文案</Tag>
        </div>
        <div class="flex items-center gap-2">
          <Button type="primary" :loading="saving" @click="save">保存当前模板</Button>
          <Button danger :disabled="state === 'default'" :loading="restoring" @click="confirmRestore">恢复默认</Button>
        </div>
      </div>

      <Alert v-if="state === 'damaged'" type="warning" show-icon :message="warning || '模板配置损坏，已回退内置默认文案。'" />

      <div class="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <div class="space-y-3">
          <div class="space-y-1 text-sm">
            <div class="flex items-center justify-between">
              <span>主题</span>
              <span class="text-xs text-text-tertiary">{{ subjectLength }}/{{ limits.subject }}</span>
            </div>
            <Input ref="subjectRef" v-model:value="draft.subject" aria-label="邮件主题" :maxlength="limits.subject" @focus="lastFocusedField = 'subject'" />
            <div v-if="subjectVariables.length" class="flex flex-wrap items-center gap-1 pt-1">
              <span class="text-xs text-text-tertiary">主题变量：</span>
              <Button v-for="variable in subjectVariables" :key="variable" size="small" @click="insertVariable('subject', variable)">
                {{ variableToken(variable) }}
              </Button>
            </div>
          </div>

          <div class="space-y-1 text-sm">
            <div class="flex items-center justify-between">
              <span>正文</span>
              <span class="text-xs text-text-tertiary">{{ bodyLength }}/{{ limits.body }}</span>
            </div>
            <Input.TextArea ref="bodyRef" v-model:value="draft.body" aria-label="邮件正文" :rows="10" :maxlength="limits.body" @focus="lastFocusedField = 'body'" />
            <div v-if="bodyVariables.length" class="flex flex-wrap items-center gap-1 pt-1">
              <span class="text-xs text-text-tertiary">正文变量：</span>
              <Button v-for="variable in bodyVariables" :key="variable" size="small" @click="insertVariable('body', variable)">
                {{ variableToken(variable) }}<span v-if="requiredSet.has(variable)" class="text-red-500"> *</span>
              </Button>
            </div>
          </div>
        </div>

        <div class="space-y-2 rounded-lg border border-border p-3">
          <div class="flex items-center justify-between">
            <span class="text-sm font-medium">即时预览</span>
            <Radio.Group v-model:value="previewMode" size="small" button-style="solid">
              <Radio.Button value="html">HTML</Radio.Button>
              <Radio.Button value="text">纯文本</Radio.Button>
            </Radio.Group>
          </div>
          <div v-if="preview" class="space-y-2">
            <div class="text-sm"><span class="text-text-tertiary">主题：</span>{{ preview.subject }}</div>
            <iframe
              v-if="previewMode === 'html'"
              sandbox=""
              :srcdoc="preview.html_body"
              title="HTML 邮件预览"
              class="h-80 w-full rounded border border-border bg-surface"
            ></iframe>
            <pre v-else class="max-h-80 overflow-auto whitespace-pre-wrap break-all rounded bg-surface-subtle p-2 text-xs">{{ preview.text_body }}</pre>
          </div>
          <Alert v-else-if="previewError" type="error" show-icon :message="previewError" />
          <div v-else class="text-xs text-text-tertiary">编辑主题或正文后将在 300ms 内生成预览。</div>
          <div class="text-xs text-text-tertiary">示意预览，最终显示取决于邮件客户端；完整 URL 不做省略。</div>
        </div>
      </div>

      <Alert type="info" show-icon>
        <template #message>使用说明</template>
        <template #description>
          <ul class="m-0 list-disc pl-4">
            <li>SMTP 测试邮件不使用这些模板，仍发送固定文案。</li>
            <li>审批通过由 approval_notify 控制，不会再叠发欢迎邮件。</li>
            <li>密码重置链接实际有效期 1 小时。</li>
            <li>若要自动生成链接，请将完整 URL 独立成行。</li>
          </ul>
        </template>
      </Alert>
    </div>
  </Card>
</template>
