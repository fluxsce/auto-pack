import { defineConfig, type Plugin } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'
import { niumaUiHost } from '@niuma/ui/vite-plugins/niuma-ui-host'
import { createRequire } from 'node:module'
import { dirname, resolve } from 'node:path'

const root = import.meta.dirname
const require = createRequire(import.meta.url)
const WEB_BASE = '/autopack/'
const UI_ROOT = dirname(require.resolve('@niuma/ui/package.json'))

function redirectToWebBase(): Plugin {
  return {
    name: 'redirect-to-web-base',
    configureServer(server) {
      server.middlewares.use((req, res, next) => {
        const path = (req.url ?? '').split('?')[0]
        if (path === '/' || path === '') {
          res.statusCode = 302
          res.setHeader('Location', WEB_BASE)
          res.end()
          return
        }
        if (path === '/favicon.ico') {
          req.url = `${WEB_BASE}favicon.png`
        }
        next()
      })
    },
  }
}

export default defineConfig({
  base: WEB_BASE,
  plugins: [redirectToWebBase(), vue(), tailwindcss(), ...niumaUiHost()],
  resolve: {
    alias: { '@': resolve(root, 'src') },
  },
  server: {
    port: 5173,
    fs: { allow: [root, UI_ROOT, resolve(root, '..')] },
    proxy: {
      '/autopack/api': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: 'server/web',
    emptyOutDir: true,
    target: 'es2022',
  },
})
