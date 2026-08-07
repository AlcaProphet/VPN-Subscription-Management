<!-- HomeView.vue：登录成功占位页（完整首页在 Build2） -->
<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Card, Button, Tag, Descriptions } from 'ant-design-vue'
import { useAuthStore } from '@/stores/auth'
import { me } from '@/api/auth'

const router = useRouter()
const auth = useAuthStore()
const loading = ref(true)

onMounted(async () => {
  try {
    if (!auth.user) auth.user = await me()
  } catch {
    // 401 已由拦截器处理跳转
  } finally {
    loading.value = false
  }
})

async function onLogout() {
  await auth.logoutAction()
  router.push('/login')
}
</script>

<template>
  <div class="max-w-2xl mx-auto py-16 px-4">
    <Card :loading="loading">
      <template #title>
        登录成功
        <Tag :color="auth.user?.role === 'admin' ? 'gold' : 'blue'" class="ml-2">
          {{ auth.user?.role === 'admin' ? '管理员' : '用户' }}
        </Tag>
      </template>
      <Descriptions v-if="auth.user" :column="1" size="small">
        <Descriptions.Item label="用户名">{{ auth.user.username }}</Descriptions.Item>
        <Descriptions.Item label="邮箱">{{ auth.user.email || '—' }}</Descriptions.Item>
        <Descriptions.Item label="状态">{{ auth.user.status }}</Descriptions.Item>
        <Descriptions.Item label="来源">{{ auth.user.user_source }}</Descriptions.Item>
      </Descriptions>
      <Button type="primary" danger class="mt-4" @click="onLogout">退出登录</Button>
    </Card>
  </div>
</template>
