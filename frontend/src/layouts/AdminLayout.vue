<!-- AdminLayout.vue：管理面板布局（UI §5.0）——Sider 220/64 + <768 汉堡 Drawer；9 模块 + 1 预留 -->
<script setup lang="ts">
import { computed, h, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Layout, Menu, Drawer, Button, Tooltip, type MenuProps } from 'ant-design-vue'
import {
  CloudUploadOutlined, TeamOutlined, ShareAltOutlined, AppstoreOutlined,
  BranchesOutlined, BlockOutlined, HomeOutlined, MenuFoldOutlined, MenuUnfoldOutlined, UserOutlined, AuditOutlined, SettingOutlined, FileTextOutlined,
  ApartmentOutlined, CloudServerOutlined, CloseOutlined,
} from '@ant-design/icons-vue'
import { useSystemStore } from '@/stores/system'
import AppHeader from '@/components/AppHeader.vue'
import ContextBar from '@/components/ContextBar.vue'

const route = useRoute()
const router = useRouter()
const system = useSystemStore()

// isMobile：matchMedia('(max-width: 767px)') 响应式布尔（与三档断点 <768 对齐）；
// 必须监听窗口缩放，窗口跨越 768px 断点时侧边栏/Drawer 响应式切换
const isMobile = ref(false)
const collapsed = ref(false) // <768 默认收起（onMounted 中按 isMobile 初始化）
const drawerOpen = ref(false)

function checkMobile() {
  isMobile.value = window.matchMedia('(max-width: 767px)').matches
  if (isMobile.value) collapsed.value = true // 移动端默认收起
}
onMounted(() => {
  checkMobile()
  window.addEventListener('resize', checkMobile)
})
onUnmounted(() => {
  window.removeEventListener('resize', checkMobile)
})

// 移动端 Drawer 打开时锁背景滚动（scroll chaining 穿透：抽屉内滚动/触摸不应带动被遮挡的背景内容）；
// html + body 双锁（iOS Safari 上仅 body overflow:hidden 可能失效）；overscroll-behavior 阻断边缘回弹穿透
watch(drawerOpen, (open) => {
  const doc = document.documentElement
  doc.style.overflow = open ? 'hidden' : ''
  doc.style.overscrollBehavior = open ? 'contain' : ''
  document.body.style.overflow = open ? 'hidden' : ''
})

function menuIcon(icon: any, label: string) {
  return () => h(Tooltip, { title: label, placement: 'right' }, {
    default: () => h(icon, { 'aria-label': label }),
  })
}

// 侧边栏菜单按业务分组；「用户组」与「Xray 实例」仅高级模式可见。
const advanced = computed(() => system.status?.advanced_mode === true)
const menuItems = computed(() => {
  const distribution = [
    { key: '/admin/subscriptions', icon: menuIcon(CloudUploadOutlined, '订阅'), label: '订阅' },
    { key: '/admin/shares', icon: menuIcon(ShareAltOutlined, '分享'), label: '分享' },
    { key: '/admin/platforms', icon: menuIcon(AppstoreOutlined, '平台'), label: '平台' },
    { key: '/admin/rules', icon: menuIcon(BranchesOutlined, '规则'), label: '规则' },
  ]
  const assembly = [
    { key: '/admin/assembly', icon: menuIcon(BlockOutlined, '订阅装配'), label: '订阅装配' },
    { key: '/admin/nodes', icon: menuIcon(ApartmentOutlined, '节点'), label: '节点' },
    ...(advanced.value ? [{ key: '/admin/xray', icon: menuIcon(CloudServerOutlined, 'Xray 实例'), label: 'Xray 实例' }] : []),
  ]
  const members = [
    { key: '/admin/users', icon: menuIcon(UserOutlined, '用户'), label: '用户' },
    { key: '/admin/approvals', icon: menuIcon(AuditOutlined, '审批中心'), label: '审批中心' },
    ...(advanced.value ? [{ key: '/admin/groups', icon: menuIcon(TeamOutlined, '用户组'), label: '用户组' }] : []),
  ]
  return [
    { key: '/admin', icon: menuIcon(HomeOutlined, '概览'), label: '概览' },
    { type: 'group', label: '分发', children: distribution },
    { type: 'group', label: '装配', children: assembly },
    { type: 'group', label: '成员', children: members },
    { type: 'group', label: '系统', children: [
      { key: '/admin/settings', icon: menuIcon(SettingOutlined, '面板配置'), label: '面板配置' },
      { key: '/admin/logs', icon: menuIcon(FileTextOutlined, '日志'), label: '日志' },
    ] },
  ] as MenuProps['items']
})

const selectedKeys = computed(() => {
  if (route.path === '/admin') return ['/admin']
  // 版本管理子路由高亮对应父菜单（/admin/subscriptions/:id/versions → 订阅）
  const seg = route.path.split('/')[2]
  return seg ? [`/admin/${seg}`] : []
})

const currentPageName = computed(() => {
  const names: Record<string, string> = {
    admin: '概览',
    subscriptions: '订阅', shares: '分享', platforms: '平台', users: '用户', approvals: '审批中心',
    rules: '规则', settings: '面板配置', logs: '日志', assembly: '订阅装配', nodes: '节点',
    groups: '用户组', xray: 'Xray 实例',
  }
  return names[route.path.split('/')[2] ?? ''] ?? '管理面板'
})

function onMenuClick(key: string) {
  drawerOpen.value = false
  router.push(key)
}

// 返回主界面（用户端首页）
function goHome() {
  drawerOpen.value = false
  void router.push('/')
}
</script>

<template>
  <Layout class="min-h-screen">
    <!-- ≥768：固定 Sider（展开 220px / 收起 64px，浅色主题，当前路由高亮）；
         trigger 置空避免默认折叠按钮与底部操作区重叠（自定义折叠按钮见下）；
         内联 position:sticky 吸顶（AntD 默认 position:relative 会与 Tailwind sticky 类同特异性冲突，内联样式优先级最高） -->
    <Layout.Sider v-if="!isMobile" v-model:collapsed="collapsed" theme="light"
                  :width="220" :collapsed-width="64" collapsible :trigger="null"
                  :style="{ position: 'sticky', top: 0, height: '100vh', overflowY: 'auto' }">
      <div class="h-16 flex items-center justify-center font-semibold truncate text-text">
        <span v-if="!collapsed">管理面板</span>
        <Tooltip v-else title="管理面板"><SettingOutlined class="text-lg" aria-label="管理面板" /></Tooltip>
      </div>
      <div class="flex flex-col h-[calc(100vh-4rem)]">
        <Menu mode="inline" :selected-keys="selectedKeys" :items="menuItems"
              @click="(e: any) => onMenuClick(e.key)" />
        <!-- 底部固定操作区：返回主界面 + 收起菜单（自定义折叠按钮，替代默认 trigger） -->
        <div class="mt-auto p-2 space-y-1">
          <Button block type="text" aria-label="返回主界面" @click="goHome">
            <template #icon><HomeOutlined /></template>
            <span v-if="!collapsed">返回主界面</span>
          </Button>
          <Button block type="text" aria-label="收起或展开菜单" @click="collapsed = !collapsed">
            <template #icon><MenuFoldOutlined v-if="!collapsed" /><MenuUnfoldOutlined v-else /></template>
            <span v-if="!collapsed">收起菜单</span>
          </Button>
        </div>
      </div>
    </Layout.Sider>
    <!-- <768：汉堡按钮唤出 Drawer 抽屉式菜单 -->
    <Drawer v-else :open="drawerOpen" placement="left" :width="280" :closable="false" @close="drawerOpen = false">
      <template #title>
        <div class="flex items-center justify-between gap-3">
          <span>{{ currentPageName }}</span>
          <Button type="text" class="touch-target" aria-label="关闭管理导航" @click="drawerOpen = false">
            <template #icon><CloseOutlined /></template>
          </Button>
        </div>
      </template>
      <div class="h-full flex flex-col min-h-0">
        <Menu class="flex-1 overflow-y-auto" mode="inline" :selected-keys="selectedKeys" :items="menuItems"
              @click="(e: any) => onMenuClick(e.key)" />
        <div class="mt-3 pt-3 border-t border-subtle border">
          <Button block type="text" class="touch-target" aria-label="返回主界面" @click="goHome">
            <template #icon><HomeOutlined /></template>
            返回主界面
          </Button>
        </div>
      </div>
    </Drawer>

    <Layout class="min-w-0">
      <!-- 通用顶栏（问题 R08-UI05：管理面板显示主界面式 header；
           R08-UI07：自定义 header 元素替代 AntD Layout.Header，避免其默认深色背景覆盖 Tailwind 底色） -->
      <AppHeader :burger="isMobile" @open-drawer="drawerOpen = true" />
      <!-- 右侧内容区：白底卡片容器（24px 内边距） -->
      <Layout.Content class="min-w-0 p-3 sm:p-4 md:p-6">
        <div class="min-w-0 bg-surface rounded-lg p-3 md:p-6 min-h-full">
          <ContextBar />
          <RouterView />
        </div>
      </Layout.Content>
    </Layout>
  </Layout>
</template>
