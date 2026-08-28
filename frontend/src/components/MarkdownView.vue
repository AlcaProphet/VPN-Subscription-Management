<!-- MarkdownView.vue：MD 内容渲染（R10-06）——markdown-it html:false（原始 HTML 按文本转义，防存储型 XSS）；
     管理员面板配置的公告/页脚等展示内容经此渲染，输出样式收紧（链接换行、无外边距容器） -->
<script setup lang="ts">
import { computed } from 'vue'
import MarkdownIt from 'markdown-it'

// 单例：html:false 禁原始 HTML；linkify 关闭（防误链）；默认 validateLink 已拦截 javascript: 等危险协议
const md = new MarkdownIt({ html: false, linkify: false })

const props = defineProps<{ source: string }>()
const rendered = computed(() => md.render(props.source ?? ''))
</script>

<template>
  <!-- v-html 渲染 markdown-it 白名单输出（html:false 下原始 HTML 已转义，无脚本注入面） -->
  <div class="markdown-body text-sm leading-relaxed" v-html="rendered" />
</template>

<style scoped>
/* 收紧 MD 渲染样式：文本色跟随主题（继承容器），链接使用主色，段落间距紧凑 */
.markdown-body :deep(p) {
  margin: 0.25rem 0;
}
.markdown-body :deep(a) {
  color: var(--ui-primary);
  word-break: break-all;
}
.markdown-body :deep(h1),
.markdown-body :deep(h2),
.markdown-body :deep(h3) {
  margin: 0.5rem 0 0.25rem;
  font-weight: 600;
}
.markdown-body :deep(ul),
.markdown-body :deep(ol) {
  padding-left: 1.25rem;
  margin: 0.25rem 0;
}
.markdown-body :deep(li) {
  margin: 0.125rem 0;
}
.markdown-body :deep(code) {
  background: rgba(128, 128, 128, 0.15);
  padding: 0.1rem 0.3rem;
  border-radius: 4px;
  font-size: 0.875em;
}
.markdown-body :deep(blockquote) {
  border-left: 3px solid rgba(128, 128, 128, 0.4);
  padding-left: 0.75rem;
  margin: 0.25rem 0;
  color: rgba(128, 128, 128, 0.9);
}
</style>
