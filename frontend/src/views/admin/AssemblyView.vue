<!-- AssemblyView.vue：订阅装配（Design2-UI §5.1）——五页签；本 Build 实现规则素材池页签，其余为 Build5 占位 -->
<script setup lang="ts">
import { ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Result, Tabs } from 'ant-design-vue'
import PageHeader from '@/components/PageHeader.vue'
import PoolTab from './assembly/PoolTab.vue'

const route = useRoute()
const router = useRouter()
const validTabs = ['pool', 'clash-yaml', 'sr-subs', 'generic-subs', 'sr-conf']

function tabFromQuery(): string {
  const q = String(route.query.tab ?? 'pool')
  return validTabs.includes(q) ? q : 'pool'
}
const activeTab = ref(tabFromQuery())
watch(() => route.query.tab, () => { activeTab.value = tabFromQuery() })
function onTabChange(key: string | number) {
  void router.replace({ query: { ...route.query, tab: String(key) } })
}
</script>

<template>
  <div>
    <PageHeader title="订阅装配" subtitle="维护规则素材池；Clash YAML / SR 节点订阅 / 通用节点订阅 / SR 分流规则装配器将在下一轮实现" />
    <Tabs v-model:activeKey="activeTab" @change="onTabChange">
      <Tabs.TabPane key="pool" tab="规则素材池">
        <PoolTab />
      </Tabs.TabPane>
      <Tabs.TabPane key="clash-yaml" tab="Clash YAML">
        <Result status="info" title="Clash YAML 装配器" sub-title="将在下一轮构建实现" />
      </Tabs.TabPane>
      <Tabs.TabPane key="sr-subs" tab="SR 节点订阅">
        <Result status="info" title="Shadowrocket 节点订阅装配器" sub-title="将在下一轮构建实现" />
      </Tabs.TabPane>
      <Tabs.TabPane key="generic-subs" tab="通用节点订阅">
        <Result status="info" title="通用节点订阅装配器" sub-title="将在下一轮构建实现" />
      </Tabs.TabPane>
      <Tabs.TabPane key="sr-conf" tab="SR 分流规则">
        <Result status="info" title="Shadowrocket 分流规则装配器" sub-title="将在下一轮构建实现" />
      </Tabs.TabPane>
    </Tabs>
  </div>
</template>
