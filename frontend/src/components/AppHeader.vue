<!-- AppHeader.vue：用户端/管理端通用顶栏，手机仅保留导航、站点名和账户入口。 -->
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
// 所属组标签仅高级模式展示（基础模式全面隐藏组概念，Design2 第一章）
const groupName = computed(() =>
  system.status?.advanced_mode === true ? (auth.user?.group_name ?? '') : '',
)

async function onLogout() {
  await auth.logoutAction()
  router.push('/login')
}
</script>

<template>
  <!-- 自定义 header 元素（非 AntD Layout.Header——避免其默认深色背景覆盖 Tailwind 底色，R08-UI07） -->
  <header class="sticky top-0 z-10 bg-white dark:bg-gray-800 shadow-sm h-16 flex items-center px-3 md:px-4 gap-2 md:gap-3">
    <Button v-if="burger" type="text" class="touch-target flex items-center justify-center flex-shrink-0"
            aria-label="打开管理导航" @click="$emit('open-drawer')">
      <template #icon><MenuOutlined class="text-xl" /></template>
    </Button>
    <div class="flex-1 min-w-0 flex items-center gap-2">
      <!-- 窄视口隐藏图标，优先给账户入口保留不可压缩的操作空间。 -->
      <img v-if="system.siteIconUrl" :src="system.siteIconUrl" alt="站点 ICON" class="hidden sm:block h-8 w-8 object-contain" />
      <span class="font-semibold text-lg dark:text-gray-100 truncate">{{ system.siteName }}</span>
      <span v-if="updatedAt" class="ml-3 text-xs text-gray-500 dark:text-gray-400 hidden md:inline">{{ updatedAt }}</span>
    </div>
    <div class="flex items-center gap-2 flex-shrink-0">
      <Button v-if="manageBtn && isAdmin" type="primary" class="hidden md:inline-flex" @click="router.push('/admin')">管理面板</Button>
      <Tag v-if="groupName" color="cyan" class="m-0 max-w-[120px] truncate hidden md:inline-flex">{{ groupName }}</Tag>
      <Dropdown :trigger="['click']">
        <Button class="touch-target max-w-28 md:max-w-none" aria-label="账户菜单">{{ auth.user?.username }}</Button>
        <template #overlay>
          <!-- 边框 + 增强阴影：暗色模式 gray-800 背景与顶栏同色，靠边框/阴影增强区分（R09-13） -->
          <div class="bg-white dark:bg-gray-800 p-3 shadow-lg rounded border border-gray-200 dark:border-gray-600 w-auto min-w-32">
            <!-- 角色标签仅管理员展示（R11：普通用户不再显示「用户」标签） -->
            <div v-if="isAdmin" class="flex justify-center">
              <Tag color="blue" class="m-0">管理员</Tag>
            </div>
            <div class="mt-2 flex flex-col gap-1">
              <Button type="text" class="touch-target" @click="router.push('/profile')">个人中心</Button>
              <Button type="text" danger class="touch-target" @click="onLogout">退出登录</Button>
              <div class="md:hidden mt-1 pt-2 border-t border-gray-200 dark:border-gray-600 flex items-center justify-between gap-3">
                <span class="text-sm text-gray-600 dark:text-gray-300">深色模式</span>
                <Switch :checked="dark" checked-children="🌙" un-checked-children="☀️"
                        title="切换暗色/浅色模式" @change="toggle" />
              </div>
            </div>
          </div>
        </template>
      </Dropdown>
      <Switch :checked="dark" checked-children="🌙" un-checked-children="☀️" class="hidden md:inline-flex"
              title="切换暗色/浅色模式" @change="toggle" />
    </div>
  </header>
</template>
