<!-- FormSection.vue：复杂表单内的字段分段与帮助文本，桌面与手机保持相同阅读顺序。 -->
<script setup lang="ts">
defineProps<{ title: string; help?: string }>()
</script>

<template>
  <section class="form-section">
    <div class="mb-3">
      <h2 class="m-0 text-base font-semibold text-text">{{ title }}</h2>
      <p v-if="help" class="m-0 mt-1 text-xs text-text-secondary">{{ help }}</p>
    </div>
    <slot />
  </section>
</template>

<style scoped>
.form-section + .form-section { border-top: 1px solid rgb(229 231 235); padding-top: 1rem; }
</style>
