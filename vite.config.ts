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
const requireUi = createRequire(resolve(UI_ROOT, 'package.json'))

/** 发布桶 exclude `@niuma/ui` 后，dayjs CJS/UMD 没有 ESM default。 */
function dayjsEsmAliases(): { find: RegExp; replacement: string }[] {
  try {
    return [
      {
        find: /^dayjs\/plugin\/customParseFormat(?:\.js)?$/,
        replacement: requireUi.resolve('dayjs/esm/plugin/customParseFormat'),
      },
      {
        find: /^dayjs$/,
        replacement: requireUi.resolve('dayjs/esm'),
      },
    ]
  } catch {
    return []
  }
}

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
    alias: [{ find: '@', replacement: resolve(root, 'src') }, ...dayjsEsmAliases()],
  },
  // 发布包 niuma-ui 没有 src 时，niumaUiHost 不会排除整桶。
  // esbuild optimizeDeps 会扫到 dist/monaco/setup.js 的 `?worker`（Vite 查询串），直接失败。
  optimizeDeps: {
    exclude: ['@niuma/ui', 'niuma-ui', 'monaco-editor'],
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
