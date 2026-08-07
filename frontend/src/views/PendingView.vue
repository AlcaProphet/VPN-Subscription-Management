<!-- PendingView.vue：账号待审批提示页（无凭据独立页） -->
<script setup lang="ts">
import { Result, Button } from 'ant-design-vue'
</script>

<template>
  <Result status="info" title="账号待审批" sub-title="等待管理员激活，激活后即可登录">
    <template #extra><Button type="primary" @click="$router.push('/login')">返回登录</Button></template>
  </Result>
</template>
