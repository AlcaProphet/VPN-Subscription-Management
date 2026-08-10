<!-- App.vue：ConfigProvider 全局中文/主题 + 按 meta.layout 切换布局 -->
<script setup lang="ts">
import { ConfigProvider } from 'ant-design-vue'
import { onMounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useTheme, antdLocale } from '@/theme'
import { useSystemStore } from '@/stores/system'
import BlankLayout from '@/layouts/BlankLayout.vue'
import HomeLayout from '@/layouts/HomeLayout.vue'
import AdminLayout from '@/layouts/AdminLayout.vue'

const { antdTheme } = useTheme()
const route = useRoute()
const layouts: Record<string, unknown> = { blank: BlankLayout, home: HomeLayout, admin: AdminLayout }

// 站点名称 → 浏览器标题（Design1 §3.4.8）；未设置时回退默认标题
const system = useSystemStore()
onMounted(() => {
  void system.fetchSiteInfo()
})
watch(() => system.siteName, (v) => {
  document.title = v
}, { immediate: true })
// 站点 ICON → favicon（R10-04）：未设置回退默认 /favicon.svg；带版本参数防浏览器缓存旧图
watch(() => system.siteIconUrl, (v) => {
  let link = document.querySelector<HTMLLinkElement>('link[rel="icon"]')
  if (!link) {
    link = document.createElement('link')
    link.rel = 'icon'
    document.head.appendChild(link)
  }
  link.href = v || '/favicon.svg'
}, { immediate: true })
</script>

<template>
  <ConfigProvider :locale="antdLocale" :theme="antdTheme">
    <component :is="layouts[(route.meta.layout as string) ?? 'home']">
      <RouterView />
    </component>
  </ConfigProvider>
</template>
