<!-- AppHeader.vue：用户端/管理端通用顶栏（UI §4.1/§5.0）——站点名 + 更新时间（可选）+ 管理面板按钮（可选）+ 组名标签 + 用户名按钮 Dropdown（点击展开）+ 暗色 emoji+开关 -->
<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { Button, Dropdown, Switch, Tag } from 'ant-design-vue'
import { MenuOutlined } from '@ant-design/icons-vue'
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
    <!-- 汉堡按钮（手机端管理面板）：图标 text-2xl 放大 + 44px 触控区（移动端触控目标规范） -->
    <Button v-if="burger" type="text" class="!w-11 !h-11 flex items-center justify-center flex-shrink-0"
            @click="$emit('open-drawer')">
      <template #icon><MenuOutlined class="text-2xl" /></template>
    </Button>
    <div class="flex-1 min-w-0 flex items-center gap-2">
      <!-- 站点 ICON（R10-06）：顶栏站点名左侧，尺寸调大；未设置不显示 -->
      <img v-if="system.siteIconUrl" :src="system.siteIconUrl" alt="站点 ICON" class="h-8 w-8 object-contain" />
      <span class="font-semibold text-lg dark:text-gray-100 truncate">{{ system.siteName }}</span>
      <span v-if="updatedAt" class="ml-3 text-xs text-gray-500 dark:text-gray-400 hidden md:inline">{{ updatedAt }}</span>
    </div>
    <div class="flex items-center gap-2 flex-shrink-0">
      <Button v-if="manageBtn && isAdmin" type="primary" size="small" @click="router.push('/admin/subscriptions')">管理面板</Button>
      <Tag v-if="groupName" color="cyan" class="m-0 max-w-[120px] truncate hidden sm:inline-flex">{{ groupName }}</Tag>
      <Dropdown :trigger="['click']">
        <Button size="small">{{ auth.user?.username }}</Button>
        <template #overlay>
          <!-- 边框 + 增强阴影：暗色模式 gray-800 背景与顶栏同色，靠边框/阴影增强区分（R09-13） -->
          <div class="bg-white dark:bg-gray-800 p-3 shadow-lg rounded border border-gray-200 dark:border-gray-600 w-auto min-w-32">
            <!-- 角色标签水平居中（R09-11；m-0 消除 AntD Tag 默认右 margin 导致的视觉偏移） -->
            <div class="flex justify-center">
              <Tag :color="isAdmin ? 'blue' : 'default'" class="m-0">{{ isAdmin ? '管理员' : '用户' }}</Tag>
            </div>
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
