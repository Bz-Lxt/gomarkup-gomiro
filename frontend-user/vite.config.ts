import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    host: '127.0.0.1',
    port: 5173,
    proxy: {
      '/api': { target: 'http://127.0.0.1:18432', changeOrigin: true },
      '/ws': { target: 'ws://127.0.0.1:18432', ws: true, changeOrigin: true },
      '/healthz': { target: 'http://127.0.0.1:18432', changeOrigin: true },
      '/readyz': { target: 'http://127.0.0.1:18432', changeOrigin: true },
      '/metrics': { target: 'http://127.0.0.1:18432', changeOrigin: true },
    },
  },
})
