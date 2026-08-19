import { fileURLToPath, URL } from 'node:url'

import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import vueDevTools from 'vite-plugin-vue-devtools'

// URL_PREFIX: sub-path for reverse proxy deployments (no trailing slash)
const urlPrefix = (process.env.URL_PREFIX || '').replace(/\/+$/, '')

// https://vite.dev/config/
export default defineConfig({
  base: urlPrefix ? `${urlPrefix}/` : '/',

  plugins: [
    vue(),
    vueDevTools(),
  ],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url))
    },
  },
  server: {
    proxy: {
      '/api': {
        target: 'http://localhost:3001',
        changeOrigin: true,
      },
      '/a': {
        target: 'http://localhost:3001',
        changeOrigin: true,
      },
      '/b': {
        target: 'http://localhost:3001',
        changeOrigin: true,
      },
      '/health': {
        target: 'http://localhost:3001',
        changeOrigin: true,
      },
    },
  },
})
