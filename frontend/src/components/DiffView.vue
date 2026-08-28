<!-- DiffView.vue：文本行级 Diff 预览（jsdiff diffLines，三色高亮）
     防过载机制：新旧总行数超阈值时不自动计算，需手动点击「启动行级对比」；
     计算时启用 jsdiff timeout，超时返回粗粒度结果并提示。
     新增/删除计数与定位、错误行定位。 -->
<script setup lang="ts">
import { computed, ref } from 'vue'
import { Button } from 'ant-design-vue'
import { diffLines, type Change } from 'diff'

// 行数安全阈值：新旧总行数超过该值时要求手动启动，防止超大产物拖垮浏览器
const SAFE_LINE_LIMIT = 5000
// jsdiff 超时（毫秒）：超时后返回已算出的粗粒度结果，不阻塞页面
const DIFF_TIMEOUT_MS = 2000

const props = withDefaults(defineProps<{
  oldText?: string
  newText: string
  targetMissing?: boolean
}>(), {
  oldText: '',
  targetMissing: false,
})

// 规模预检：总行数 = 旧文本行数 + 新文本行数
const totalLines = computed(() =>
  (props.oldText ? props.oldText.split('\n').length : 0) + props.newText.split('\n').length)
const oversized = computed(() => !props.targetMissing && totalLines.value > SAFE_LINE_LIMIT)

// 手动启动开关：超大文本必须由用户显式点击后才执行行级 diff
const forced = ref(false)
const running = ref(false)
const slowHint = ref(false)
const forcedChanges = ref<Change[] | null>(null)

// 常规规模文本：直接同步计算（带 timeout 保护）；targetMissing 整体视为新增
const changes = computed<Change[] | null>(() => {
  if (props.targetMissing) {
    return [{ value: props.newText, added: true, removed: false, count: props.newText.split('\n').length }]
  }
  if (oversized.value) {
    return forced.value ? forcedChanges.value : null // 未手动启动时不计算
  }
  return diffLines(props.oldText ?? '', props.newText, { timeout: DIFF_TIMEOUT_MS }) ?? []
})

const addedCount = computed(() => {
  let n = 0
  for (const c of changes.value ?? []) if (c.added) n += renderLines(c.value).length
  return n
})
const removedCount = computed(() => {
  let n = 0
  for (const c of changes.value ?? []) if (c.removed) n += renderLines(c.value).length
  return n
})

// 手动启动行级对比（超大文本入口）；延迟一帧让 loading 先渲染，避免长计算卡住交互
function startDiff() {
  running.value = true
  slowHint.value = false
  setTimeout(() => {
    const t0 = performance.now()
    const result = diffLines(props.oldText ?? '', props.newText, { timeout: DIFF_TIMEOUT_MS }) ?? []
    slowHint.value = performance.now() - t0 >= DIFF_TIMEOUT_MS
    forcedChanges.value = result
    forced.value = true
    running.value = false
  }, 0)
}

function renderLines(value: string): string[] {
  const lines = value.split('\n')
  // 末尾空串是结尾换行符的产物，不渲染为空行
  if (lines.length > 1 && lines[lines.length - 1] === '') lines.pop()
  return lines
}

function jumpTo(kind: 'added' | 'removed' | 'error') {
  if (kind === 'error') {
    const el = Array.from(document.querySelectorAll<HTMLElement>('.diff-view .diff-line'))
      .find((node) => /error|错误|失败|invalid/i.test(node.textContent ?? ''))
    el?.scrollIntoView({ behavior: 'smooth', block: 'center' })
    return
  }
  const el = document.querySelector<HTMLElement>(`.diff-view [data-diff-kind="${kind}"]`)
  el?.scrollIntoView({ behavior: 'smooth', block: 'center' })
}
</script>

<template>
  <div class="diff-view max-h-[60vh] overflow-auto font-mono text-xs bg-surface-subtle rounded p-3 whitespace-pre-wrap">
    <div v-if="targetMissing" class="text-info mb-2">目标尚无激活版本，本次对比为整体新增</div>
    <div v-else-if="oversized && !forced" class="text-warning flex items-center gap-3 flex-wrap">
      <span>内容共 {{ totalLines }} 行，超过安全阈值 {{ SAFE_LINE_LIMIT }} 行，行级对比可能导致页面卡顿。</span>
      <Button size="small" :loading="running" @click="startDiff">仍要启动行级对比</Button>
    </div>
    <div v-if="slowHint" class="text-warning mb-2">对比超出时限，已返回粗粒度部分结果</div>

    <div v-if="changes" class="flex flex-wrap items-center gap-2 mb-2">
      <span class="text-success">+{{ addedCount }}</span>
      <span class="text-danger">-{{ removedCount }}</span>
      <Button size="small" :disabled="addedCount === 0" @click="jumpTo('added')">定位新增</Button>
      <Button size="small" :disabled="removedCount === 0" @click="jumpTo('removed')">定位删除</Button>
      <Button size="small" @click="jumpTo('error')">定位错误行</Button>
    </div>

    <template v-if="changes">
      <div v-for="(c, i) in changes" :key="i" :class="{
        'bg-green-100 dark:bg-green-900/40 bg-success-soft': c.added,
        'bg-red-100 dark:bg-red-900/40 bg-danger-soft': c.removed,
        'text-text': !c.added && !c.removed,
      }">
        <div v-for="(line, idx) in renderLines(c.value)" :key="idx" class="diff-line"
             :data-diff-kind="c.added ? 'added' : c.removed ? 'removed' : 'context'">
          <span class="select-none mr-2 inline-block w-4 text-right">{{ c.added ? '+' : c.removed ? '-' : ' ' }}</span>{{ line }}
        </div>
      </div>
    </template>
  </div>
</template>
