<template>
  <!-- 桌面端：内联按钮 -->
  <div class="hidden md:flex md:items-center md:gap-1">
    <slot />
  </div>
  <!-- 移动端：下拉菜单 -->
  <div class="md:hidden relative" ref="menuRef">
    <button
      @click.stop="toggle"
      class="flex items-center justify-center w-8 h-8 rounded-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-600"
      title="更多操作"
    >
      <svg class="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
        <path d="M10 6a2 2 0 110-4 2 2 0 010 4zM10 12a2 2 0 110-4 2 2 0 010 4zM10 18a2 2 0 110-4 2 2 0 010 4z"/>
      </svg>
    </button>
    <!-- 下拉菜单 -->
    <div
      v-if="open"
      class="absolute right-0 z-[300] mt-1 w-44 rounded-md shadow-lg bg-white dark:bg-gray-700 ring-1 ring-black/5 dark:ring-white/10 py-1"
      @click.stop="close"
    >
      <slot name="menu" />
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'

const open = ref(false)
const menuRef = ref(null)

function toggle() {
  open.value = !open.value
}

function close() {
  open.value = false
}

function onClickOutside(e) {
  if (menuRef.value && !menuRef.value.contains(e.target)) {
    open.value = false
  }
}

onMounted(() => {
  document.addEventListener('click', onClickOutside)
})

onUnmounted(() => {
  document.removeEventListener('click', onClickOutside)
})
</script>
