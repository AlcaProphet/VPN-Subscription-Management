<template>
  <el-config-provider>
    <router-view />
  </el-config-provider>
</template>

<script setup>
import { useTheme } from '@/composables/useTheme'
useTheme()
</script>
