<!-- AdminLayout.vue：管理面板布局（UI §5.0）——Sider 220/64 + <768 汉堡 Drawer；9 模块 + 1 预留 -->
<script setup lang="ts">
import { computed, h, onMounted, onUnmounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Layout, Menu, Drawer, Button, type MenuProps } from 'ant-design-vue'
import {
  CloudUploadOutlined, TeamOutlined, ShareAltOutlined, AppstoreOutlined,
  BranchesOutlined, BlockOutlined, HomeOutlined, MenuFoldOutlined, MenuUnfoldOutlined,
} from '@ant-design/icons-vue'
import { useTheme } from '@/theme'

const route = useRoute()
const router = useRouter()
const { dark, toggle } = useTheme()

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

// 侧边栏菜单：9 模块 + 1 预留，平铺不分组（图标+文字）；
// 「用户/审批中心/面板配置/日志」在 Build3 实现，本 Step 隐藏（Build3 补充显示）
const menuItems = computed(() => [
  { key: '/admin/subscriptions', icon: () => h(CloudUploadOutlined), label: '订阅' },
  { key: '/admin/groups', icon: () => h(TeamOutlined), label: '用户组' },
  { key: '/admin/shares', icon: () => h(ShareAltOutlined), label: '分享' },
  { key: '/admin/platforms', icon: () => h(AppstoreOutlined), label: '平台' },
  // { key: '/admin/users', icon: () => h(UserOutlined), label: '用户' },          // Build3 显示
  // { key: '/admin/approvals', icon: () => h(AuditOutlined), label: '审批中心' }, // Build3 显示
  { key: '/admin/rules', icon: () => h(BranchesOutlined), label: '规则' },
  // { key: '/admin/settings', icon: () => h(SettingOutlined), label: '面板配置' }, // Build3 显示
  // { key: '/admin/logs', icon: () => h(FileTextOutlined), label: '日志' },        // Build3 显示
  { key: '/admin/assembly', icon: () => h(BlockOutlined), label: '订阅装配' }, // 预留占位页
] as MenuProps['items'])

const selectedKeys = computed(() => {
  // 版本管理子路由高亮对应父菜单（/admin/subscriptions/:id/versions → 订阅）
  const seg = route.path.split('/')[2]
  return seg ? [`/admin/${seg}`] : []
})

function onMenuClick(key: string) {
  drawerOpen.value = false
  router.push(key)
}

// 退出登录
async function onLogout() {
  const { useAuthStore } = await import('@/stores/auth')
  const auth = useAuthStore()
  await auth.logoutAction()
  router.push('/login')
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
         trigger 置空避免默认折叠按钮与底部操作区重叠（自定义折叠按钮见下） -->
    <Layout.Sider v-if="!isMobile" v-model:collapsed="collapsed" theme="light"
                  :width="220" :collapsed-width="64" collapsible :trigger="null">
      <div class="h-16 flex items-center justify-center font-semibold truncate">
        <span v-if="!collapsed">管理面板</span>
        <span v-else>管</span>
      </div>
      <div class="flex flex-col h-[calc(100vh-4rem)]">
        <Menu mode="inline" :selected-keys="selectedKeys" :items="menuItems"
              @click="(e: any) => onMenuClick(e.key)" />
        <!-- 底部固定操作区：返回主界面 + 收起菜单（自定义折叠按钮，替代默认 trigger） -->
        <div class="mt-auto p-2 space-y-1">
          <Button block type="text" @click="goHome">
            <template #icon><HomeOutlined /></template>
            <span v-if="!collapsed">返回主界面</span>
          </Button>
          <Button block type="text" @click="collapsed = !collapsed">
            <template #icon><MenuFoldOutlined v-if="!collapsed" /><MenuUnfoldOutlined v-else /></template>
            <span v-if="!collapsed">收起菜单</span>
          </Button>
        </div>
      </div>
    </Layout.Sider>
    <!-- <768：汉堡按钮唤出 Drawer 抽屉式菜单 -->
    <Drawer v-else :open="drawerOpen" placement="left" :width="220" @close="drawerOpen = false">
      <div class="h-8 font-semibold mb-2">管理面板</div>
      <Menu mode="inline" :selected-keys="selectedKeys" :items="menuItems"
            @click="(e: any) => onMenuClick(e.key)" />
      <!-- 返回主界面：Drawer 底部固定入口 -->
      <div class="absolute bottom-4 left-0 right-0 px-2">
        <Button block type="text" @click="goHome">
          <template #icon><HomeOutlined /></template>
          返回主界面
        </Button>
      </div>
    </Drawer>

    <Layout>
      <Layout.Header v-if="isMobile" class="bg-white dark:bg-gray-800 flex items-center justify-between px-3">
        <Button type="text" @click="drawerOpen = true">☰</Button>
        <Button type="text" size="small" @click="toggle()">{{ dark ? '浅色' : '暗色' }}</Button>
      </Layout.Header>
      <!-- 右侧内容区：白底卡片容器（24px 内边距） -->
      <Layout.Content class="p-6">
        <div class="bg-white dark:bg-gray-800 rounded-lg p-6 min-h-full">
          <div class="flex justify-end mb-2">
            <Button size="small" type="text" @click="toggle()">{{ dark ? '切换到浅色' : '切换到暗色' }}</Button>
            <Button size="small" type="text" danger @click="onLogout">退出</Button>
          </div>
          <RouterView />
        </div>
      </Layout.Content>
    </Layout>
  </Layout>
</template>
