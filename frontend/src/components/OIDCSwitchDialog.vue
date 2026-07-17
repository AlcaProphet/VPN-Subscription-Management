<template>
  <el-dialog
    :model-value="visible"
    title="选择 OIDC 提供商"
    width="400px"
    :close-on-click-modal="false"
    @update:model-value="$emit('update:visible', $event)"
  >
    <el-radio-group :model-value="currentProvider" @update:model-value="onSelect">
      <el-radio value="keycloak" class="provider-radio">Keycloak</el-radio>
      <el-radio value="auth0" class="provider-radio">Auth0</el-radio>
      <el-radio value="generic" class="provider-radio">通用 OIDC</el-radio>
    </el-radio-group>
    <template #footer>
      <el-button @click="$emit('update:visible', false)">取消</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
const props = defineProps({
  visible: { type: Boolean, required: true },
  currentProvider: { type: String, default: 'keycloak' }
})

const emit = defineEmits(['update:visible', 'switch'])

function onSelect(provider) {
  emit('switch', provider)
  emit('update:visible', false)
}
</script>

<style scoped>
.provider-radio {
  display: block;
  margin-bottom: 12px;
}
</style>
