<!-- NotFoundView.vue：a-result 404 + 「返回首页」 -->
<script setup lang="ts">
import { Result, Button } from 'ant-design-vue'
</script>

<template>
  <Result status="404" title="404" sub-title="页面不存在">
    <template #extra><Button type="primary" @click="$router.push('/')">返回首页</Button></template>
  </Result>
</template>
