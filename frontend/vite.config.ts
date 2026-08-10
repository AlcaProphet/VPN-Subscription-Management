import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { fileURLToPath, URL } from 'node:url'

export default defineConfig({
  plugins: [vue()],
  resolve: { alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) } },
  server: {
    proxy: {
      '/api': 'http://127.0.0.1:8080',
      '/health': 'http://127.0.0.1:8080',
      // 站点 ICON/安装包等可缓存资源（R10-04）：不代理会被 SPA fallback 吞掉返回 HTML 导致图片加载失败
      '/public': 'http://127.0.0.1:8080',
    },
  },
  build: {
    // 不配置 manualChunks：ant-design-vue 4.x 内部存在模块循环依赖，
    // 手动拆出 antd/vendor chunk 会产生跨 chunk 循环引用，浏览器报
    // `Cannot access 'X' before initialization` 导致白屏；交由 rollup 自动分割（已验证）
    rollupOptions: {
      output: {},
    },
  },
})
