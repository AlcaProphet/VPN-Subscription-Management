<template>
  <div class="flex min-h-screen bg-gray-50 dark:bg-gray-900">
    <!-- Mobile hamburger button -->
    <div class="hidden max-md:flex fixed top-0 left-0 right-0 z-[100] h-12 items-center gap-3 px-4 bg-white dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700">
      <button class="flex items-center justify-center w-6 h-6" @click="toggleSidebar">
        <svg v-if="!sidebarVisible" class="w-6 h-6 text-gray-700 dark:text-gray-300" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16M4 18h16"/>
        </svg>
        <svg v-else class="w-6 h-6 text-gray-700 dark:text-gray-300" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/>
        </svg>
      </button>
      <span class="text-base font-semibold text-gray-900 dark:text-white flex-1">管理面板</span>
      <button class="text-blue-600 hover:text-blue-700 text-sm" @click="goHome">
        <svg class="w-5 h-5" fill="currentColor" viewBox="0 0 24 24">
          <path d="M10 20v-6h4v6h5v-8h3L12 3 2 12h3v8z"/>
        </svg>
      </button>
    </div>

    <!-- Sidebar overlay for mobile -->
    <div
      v-if="sidebarVisible"
      class="hidden max-md:block fixed inset-0 z-[199] bg-black/40"
      @click="toggleSidebar"
    />

    <!-- Sidebar -->
    <aside
      :class="['fixed top-0 left-0 bottom-0 z-[200] bg-white dark:bg-gray-800 border-r border-gray-200 dark:border-gray-700 overflow-y-auto flex flex-col w-[200px] shrink-0',
        sidebarVisible ? 'max-md:translate-x-0' : 'max-md:-translate-x-[200px]',
        'max-md:transition-transform max-md:duration-300']"
    >
      <div class="p-5 pb-3">
        <div class="flex items-center justify-between gap-2">
          <h3 class="m-0 text-lg text-gray-900 dark:text-white whitespace-nowrap">管理面板</h3>
          <button
            class="bg-blue-600 hover:bg-blue-700 text-white rounded-md px-2 py-1 text-xs flex items-center gap-1"
            @click="goHome"
          >
            <svg class="w-3.5 h-3.5" fill="currentColor" viewBox="0 0 24 24">
              <path d="M10 20v-6h4v6h5v-8h3L12 3 2 12h3v8z"/>
            </svg>
            首页
          </button>
        </div>
      </div>
      <el-menu
        :router="true"
        :default-active="activeMenu"
        class="sidebar-menu flex-1 border-r-0!"
        @select="onMenuSelect"
      >
        <el-menu-item index="/admin/subscriptions">
          <svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"/>
          </svg>
          <span>订阅管理</span>
        </el-menu-item>
        <el-menu-item index="/admin/shares">
          <svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8.684 13.342C8.886 12.938 9 12.482 9 12c0-.482-.114-.938-.316-1.342m0 2.684a3 3 0 110-2.684m0 2.684l6.632 3.316m-6.632-6l6.632-3.316m0 0a3 3 0 105.367-2.684 3 3 0 00-5.367 2.684zm0 9.316a3 3 0 105.368 2.684 3 3 0 00-5.368-2.684z"/>
          </svg>
          <span>分享订阅</span>
        </el-menu-item>
        <el-menu-item index="/admin/platforms">
          <svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"/>
          </svg>
          <span>平台管理</span>
        </el-menu-item>
        <el-menu-item index="/admin/users">
          <svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z"/>
          </svg>
          <span>用户管理</span>
        </el-menu-item>
        <el-menu-item index="/admin/rules">
          <svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 10h16M4 14h16M4 18h16"/>
          </svg>
          <span>规则管理</span>
        </el-menu-item>
        <el-menu-item index="/admin/system">
          <svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.066 2.573c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.573 1.066c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.066-2.573c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z"/>
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"/>
          </svg>
          <span>面板配置</span>
        </el-menu-item>
        <el-menu-item index="/admin/logs">
          <svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-3 7h3m-3 4h3m-6-4h.01M9 16h.01"/>
          </svg>
          <span>日志查看</span>
        </el-menu-item>
      </el-menu>
    </aside>

    <!-- Main content -->
    <main class="ml-[200px] max-md:ml-0 flex-1 p-6 max-md:pt-16 min-h-screen box-border min-w-0">
      <router-view />
    </main>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'

const route = useRoute()
const router = useRouter()

const sidebarVisible = ref(false)

const activeMenu = computed(() => {
  const path = route.path
  if (path.startsWith('/admin/subscriptions')) return '/admin/subscriptions'
  if (path.startsWith('/admin/shares')) return '/admin/shares'
  if (path.startsWith('/admin/platforms')) return '/admin/platforms'
  if (path.startsWith('/admin/users')) return '/admin/users'
  if (path.startsWith('/admin/rules')) return '/admin/rules'
  if (path.startsWith('/admin/system')) return '/admin/system'
  if (path.startsWith('/admin/logs')) return '/admin/logs'
  return '/admin/subscriptions'
})

function toggleSidebar() {
  sidebarVisible.value = !sidebarVisible.value
}

function onMenuSelect() {
  sidebarVisible.value = false
}

function goHome() {
  sidebarVisible.value = false
  router.push('/')
}
</script>

<style scoped>
:deep(.sidebar-menu .el-menu-item.is-active) {
  background: linear-gradient(135deg, #667eea, #764ba2);
  color: #fff;
}
:deep(.sidebar-menu .el-menu-item.is-active svg) {
  color: #fff;
}
</style>
