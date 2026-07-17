<template>
  <div class="manage-layout">
    <!-- Mobile hamburger button -->
    <div class="mobile-header" @click="toggleSidebar">
      <el-icon :size="24">
        <Expand v-if="!sidebarVisible" />
        <Fold v-if="sidebarVisible" />
      </el-icon>
      <span class="mobile-title">管理面板</span>
    </div>

    <!-- Sidebar overlay for mobile -->
    <div
      v-if="sidebarVisible"
      class="sidebar-overlay"
      @click="toggleSidebar"
    />

    <!-- Sidebar -->
    <el-aside
      :class="['manage-sidebar', { 'sidebar-visible': sidebarVisible }]"
      width="200px"
    >
      <div class="sidebar-header">
        <h3>管理面板</h3>
      </div>
      <el-menu
        :router="true"
        :default-active="activeMenu"
        class="sidebar-menu"
        @select="onMenuSelect"
      >
        <el-menu-item index="/admin/subscriptions">
          <el-icon><Document /></el-icon>
          <span>订阅管理</span>
        </el-menu-item>
        <el-menu-item index="/admin/shares">
          <el-icon><Share /></el-icon>
          <span>分享订阅</span>
        </el-menu-item>
        <el-menu-item index="/admin/platforms">
          <el-icon><Monitor /></el-icon>
          <span>平台管理</span>
        </el-menu-item>
        <el-menu-item index="/admin/users">
          <el-icon><User /></el-icon>
          <span>用户管理</span>
        </el-menu-item>
        <el-menu-item index="/admin/rules">
          <el-icon><List /></el-icon>
          <span>规则管理</span>
        </el-menu-item>
        <el-menu-item index="/admin/oidc">
          <el-icon><Setting /></el-icon>
          <span>OIDC 配置</span>
        </el-menu-item>
        <el-menu-item index="/admin/logs">
          <el-icon><Tickets /></el-icon>
          <span>日志查看</span>
        </el-menu-item>
      </el-menu>
    </el-aside>

    <!-- Main content -->
    <el-main class="manage-main">
      <router-view />
    </el-main>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  Document,
  Share,
  Monitor,
  User,
  List,
  Setting,
  Tickets,
  Expand,
  Fold
} from '@element-plus/icons-vue'

const route = useRoute()
const router = useRouter()

const sidebarVisible = ref(false)

const activeMenu = computed(() => {
  // Match the current route to a menu item
  const path = route.path
  if (path.startsWith('/admin/subscriptions')) return '/admin/subscriptions'
  if (path.startsWith('/admin/shares')) return '/admin/shares'
  if (path.startsWith('/admin/platforms')) return '/admin/platforms'
  if (path.startsWith('/admin/users')) return '/admin/users'
  if (path.startsWith('/admin/rules')) return '/admin/rules'
  if (path.startsWith('/admin/oidc')) return '/admin/oidc'
  if (path.startsWith('/admin/logs')) return '/admin/logs'
  return '/admin/subscriptions'
})

function toggleSidebar() {
  sidebarVisible.value = !sidebarVisible.value
}

function onMenuSelect() {
  // Close sidebar on mobile after navigation
  sidebarVisible.value = false
}
</script>

<style scoped>
.manage-layout {
  display: flex;
  min-height: 100vh;
  background: var(--el-bg-color-page);
}

/* Mobile header */
.mobile-header {
  display: none;
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  z-index: 100;
  height: 48px;
  align-items: center;
  gap: 12px;
  padding: 0 16px;
  background: var(--el-bg-color);
  border-bottom: 1px solid var(--el-border-color-light);
  cursor: pointer;
}

.mobile-title {
  font-size: 16px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

/* Sidebar overlay */
.sidebar-overlay {
  display: none;
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  z-index: 199;
  background: rgba(0, 0, 0, 0.4);
}

/* Sidebar */
.manage-sidebar {
  position: fixed;
  top: 0;
  left: 0;
  bottom: 0;
  z-index: 200;
  background: var(--el-bg-color);
  border-right: 1px solid var(--el-border-color-light);
  overflow-y: auto;
  display: flex;
  flex-direction: column;
}

.sidebar-header {
  padding: 20px 20px 12px;
}

.sidebar-header h3 {
  margin: 0;
  font-size: 17px;
  color: var(--el-text-color-primary);
}

.sidebar-menu {
  flex: 1;
  border-right: none !important;
}

.sidebar-menu .el-menu-item.is-active {
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  color: #fff;
}

.sidebar-menu .el-menu-item.is-active .el-icon {
  color: #fff;
}

/* Main content */
.manage-main {
  margin-left: 200px;
  flex: 1;
  padding: 24px;
  min-height: 100vh;
  box-sizing: border-box;
}

/* Mobile responsive */
@media (max-width: 768px) {
  .mobile-header {
    display: flex;
  }

  .sidebar-overlay {
    display: block;
  }

  .manage-sidebar {
    transform: translateX(-200px);
    transition: transform 0.3s ease;
  }

  .manage-sidebar.sidebar-visible {
    transform: translateX(0);
  }

  .manage-main {
    margin-left: 0;
    padding-top: 64px;
  }
}
</style>
