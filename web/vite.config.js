import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [vue()],
  server: {
    port: 5173,
    // 开发时 /api 代理到 Go 后端，联调无需跨域
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:18765',
        changeOrigin: true,
      },
    },
  },
})
