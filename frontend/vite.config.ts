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
    },
  },
  build: {
    rollupOptions: {
      output: {
        manualChunks: { antd: ['ant-design-vue'], vendor: ['vue', 'vue-router', 'pinia', 'axios', 'dayjs'] },
      },
    },
  },
})
