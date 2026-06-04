import { fileURLToPath, URL } from 'node:url'

import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import vueDevTools from 'vite-plugin-vue-devtools'

const appBase = '/autopack/'

// https://vite.dev/config/
export default defineConfig({
  base: appBase,
  plugins: [vue(), vueDevTools()],
  server: {
    proxy: {
      '/autopack/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  css: {
    preprocessorOptions: {
      scss: {
        additionalData: `@use "@/styles/abstracts" as *;\n`,
      },
    },
  },
})
