<!-- DiffView.vue：文本行级 Diff 预览（jsdiff diffLines，三色高亮） -->
<script setup lang="ts">
import { computed } from 'vue'
import { diffLines } from '@/lib/diff'

const props = withDefaults(defineProps<{
  oldText?: string
  newText: string
  targetMissing?: boolean
}>(), {
  oldText: '',
  targetMissing: false,
})

const changes = computed<any[]>(() => {
  if (props.targetMissing) {
    return [{ value: props.newText, added: true }]
  }
  return diffLines(props.oldText ?? '', props.newText)
})
</script>

<template>
  <pre class="diff-view max-h-[60vh] overflow-auto font-mono text-xs bg-gray-50 dark:bg-gray-900 rounded p-3 whitespace-pre-wrap">
    <div v-if="targetMissing" class="text-blue-600 mb-2">目标尚无激活版本，本次对比为整体新增</div>
    <div v-for="(c, i) in changes" :key="i" :class="{
      'bg-green-100 dark:bg-green-900/40': c.added,
      'bg-red-100 dark:bg-red-900/40': c.removed,
      'text-gray-700 dark:text-gray-300': !c.added && !c.removed,
    }">
      <template v-for="(line, idx) in c.value.split('\n')" :key="idx">
        <div v-if="line !== '' || idx < c.value.split('\n').length - 1">
          <span class="select-none mr-2">{{ c.added ? '+' : c.removed ? '-' : ' ' }}</span>{{ line }}
        </div>
      </template>
    </div>
  </pre>
</template>
