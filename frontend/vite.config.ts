import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// 构建产物直接落到后端的 embed 目录，go build 时会被打进单二进制。
export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) },
  },
  build: {
    outDir: '../backend/internal/web/dist',
    emptyOutDir: true,
    // 离线图标集（icons-*.js，约 3MB）必然超过这个阈值并触发一条警告。
    // 那是**故意**的：它是按需加载的独立 chunk，不进首屏（决策 019）。
    // 别为了消掉警告把它并回主包，也别因此就不做这个 chunk。
    chunkSizeWarningLimit: 1200,
    rollupOptions: {
      output: {
        // 组件库单独成块，改业务代码时它的缓存不会失效。
        manualChunks: { 'naive-ui': ['naive-ui'] },
      },
    },
  },
  server: {
    // 默认 5173（和 CLAUDE.md 里写的一致）；被别的进程占了就用 PORT 临时换一个。
    port: Number(process.env.PORT) || 5173,
    // 开发时把接口和上传文件都代理到后端，浏览器只跟 5173 打交道，
    // 会话 Cookie 因此是同源的，不用为跨域单独放宽 CORS。
    proxy: {
      '/api': { target: 'http://127.0.0.1:8080', changeOrigin: false },
      '/uploads': { target: 'http://127.0.0.1:8080', changeOrigin: false },
    },
  },
})
