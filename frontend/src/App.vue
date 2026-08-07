<!-- App.vue：ConfigProvider 全局中文/主题 + 按 meta.layout 切换布局 -->
<script setup lang="ts">
import { ConfigProvider } from 'ant-design-vue'
import { useRoute } from 'vue-router'
import { useTheme, antdLocale } from '@/theme'
import BlankLayout from '@/layouts/BlankLayout.vue'
import HomeLayout from '@/layouts/HomeLayout.vue'

const { antdTheme } = useTheme()
const route = useRoute()
const layouts: Record<string, unknown> = { blank: BlankLayout, home: HomeLayout }
</script>

<template>
  <ConfigProvider :locale="antdLocale" :theme="antdTheme">
    <component :is="layouts[(route.meta.layout as string) ?? 'home']">
      <RouterView />
    </component>
  </ConfigProvider>
</template>
