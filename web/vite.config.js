import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { fileURLToPath, URL } from 'node:url'

// 两个入口：员工 H5（index.html，Vant）与管理后台（admin.html，Element Plus）。
// 生产环境由 Go 服务 go:embed 托管；开发环境用下面的中间件模拟 SPA 回退。
function spaFallback() {
  return {
    name: 'spa-fallback',
    configureServer(server) {
      server.middlewares.use((req, _res, next) => {
        const url = (req.url || '/').split('?')[0]
        const isAsset =
          url.startsWith('/@') ||
          url.startsWith('/src/') ||
          url.startsWith('/node_modules/') ||
          url.startsWith('/api/') ||
          url.startsWith('/assets/') ||
          /\.[a-z0-9]+$/i.test(url)
        if (isAsset) return next()
        if (url === '/admin' || url.startsWith('/admin/')) req.url = '/admin.html'
        else if (url !== '/' && req.url !== '/') req.url = '/index.html'
        next()
      })
    },
  }
}

export default defineConfig({
  plugins: [vue(), spaFallback()],
  base: '/',
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    chunkSizeWarningLimit: 1500,
    rollupOptions: {
      input: {
        employee: fileURLToPath(new URL('./index.html', import.meta.url)),
        admin: fileURLToPath(new URL('./admin.html', import.meta.url)),
      },
    },
  },
  server: {
    port: 5173,
    proxy: {
      '/api': { target: 'http://127.0.0.1:8080', changeOrigin: true },
      '/healthz': { target: 'http://127.0.0.1:8080', changeOrigin: true },
    },
  },
})
