<!-- AssemblyView.vue：订阅装配占位页（UI §5.0，第一次构建唯一落地形态）——无任何表单与接口调用 -->
<script setup lang="ts">
import { Result, Tag } from 'ant-design-vue'
// 暂缓项详见 DesignOnHold.md；本页无任何表单与接口调用
</script>

<template>
  <Result status="info" title="订阅装配">
    <template #subTitle>
      <p>勾选模块动态拼接双语法订阅（Clash YAML / Shadowrocket .conf）</p>
      <Tag color="processing" class="mt-2">即将推出</Tag>
      <p class="text-gray-400 text-sm mt-3">该功能已完成设计、暂缓开发，恢复时间见项目规划</p>
    </template>
  </Result>
</template>
