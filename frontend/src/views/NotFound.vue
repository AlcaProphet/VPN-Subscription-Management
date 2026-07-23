<template>
  <div class="flex items-center justify-center min-h-screen bg-gray-50 dark:bg-gray-900">
    <div class="text-center p-12 rounded-xl bg-white dark:bg-gray-800 shadow-lg max-w-sm w-full">
      <h1 class="m-0 mb-2 text-6xl font-bold text-gray-300 dark:text-gray-600">404</h1>
      <p class="m-0 mb-8 text-base text-gray-500 dark:text-gray-400">页面不存在</p>
      <button
        class="bg-blue-600 hover:bg-blue-700 text-white rounded-md px-4 py-2 text-sm"
        @click="$router.push('/')"
      >
        返回首页
      </button>
    </div>
  </div>
</template>
