<!-- AppHeader.vue：用户端/管理端通用顶栏（UI §4.1/§5.0）——站点名 + 更新时间（可选）+ 管理面板按钮（可选）+ 组名标签 + 用户名按钮 Dropdown（点击展开）+ 暗色 emoji+开关 -->
<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { Button, Dropdown, Space, Switch, Tag } from 'ant-design-vue'
import { useAuthStore } from '@/stores/auth'
import { useSystemStore } from '@/stores/system'
import { useTheme } from '@/theme'

// 模板中直接使用 props 名（updatedAt/manageBtn/burger）与 $emit，无需赋值解包
defineProps<{
  updatedAt?: string | null // 更新时间戳文案（如「订阅更新于 …」，不传不显示）
  manageBtn?: boolean       // 显示「管理面板」入口按钮（仅用户端管理员）
  burger?: boolean          // 显示 ☰ 汉堡按钮（管理面板移动端唤出 Drawer）
}>()
defineEmits<{ 'open-drawer': [] }>()

const router = useRouter()
const auth = useAuthStore()
const system = useSystemStore()
const { dark, toggle } = useTheme()

const isAdmin = computed(() => auth.user?.role === 'admin')
const groupName = computed(() => auth.user?.group_name ?? '')

async function onLogout() {
  await auth.logoutAction()
  router.push('/login')
}
</script>

<template>
  <!-- 自定义 header 元素（非 AntD Layout.Header——避免其默认深色背景覆盖 Tailwind 底色，R08-UI07） -->
  <header class="sticky top-0 z-10 bg-white dark:bg-gray-800 shadow-sm h-16 flex items-center px-4 gap-3">
    <Button v-if="burger" type="text" @click="$emit('open-drawer')">☰</Button>
    <div class="flex-1 min-w-0">
      <span class="font-semibold text-lg dark:text-gray-100 truncate">{{ system.siteName }}</span>
      <span v-if="updatedAt" class="ml-3 text-xs text-gray-500 dark:text-gray-400 hidden md:inline">{{ updatedAt }}</span>
    </div>
    <div class="flex items-center gap-2 flex-shrink-0">
      <Button v-if="manageBtn && isAdmin" type="primary" size="small" @click="router.push('/admin/subscriptions')">管理面板</Button>
      <Tag v-if="groupName" color="cyan" class="m-0 max-w-[120px] truncate hidden sm:inline-flex">{{ groupName }}</Tag>
      <Dropdown :trigger="['click']">
        <Button size="small">{{ auth.user?.username }}</Button>
        <template #overlay>
          <div class="bg-white dark:bg-gray-800 p-3 shadow rounded w-48">
            <Space :wrap="true">
              <Tag :color="isAdmin ? 'blue' : 'default'">{{ isAdmin ? '管理员' : '用户' }}</Tag>
            </Space>
            <div class="mt-2 flex flex-col gap-1">
              <Button size="small" type="text" @click="router.push('/profile')">个人中心</Button>
              <Button size="small" type="text" danger @click="onLogout">退出登录</Button>
            </div>
          </div>
        </template>
      </Dropdown>
      <Switch :checked="dark" checked-children="🌙" un-checked-children="☀️" size="small"
              title="切换暗色/浅色模式" @change="toggle" />
    </div>
  </header>
</template>
