import { fileURLToPath, URL } from 'node:url'

import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import vueDevTools from 'vite-plugin-vue-devtools'

// The Go API server listens on 127.0.0.1:8080 (see backend/configs/config.yaml).
const API_TARGET = 'http://127.0.0.1:8080'

// https://vite.dev/config/
export default defineConfig({
  plugins: [vue(), vueDevTools()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    port: 5173,
    // Proxying keeps the browser on a single origin during development, so the
    // frontend behaves exactly as it does in production, where the Go server
    // serves dist/ itself and no CORS is involved at all.
    proxy: {
      '/api': { target: API_TARGET, changeOrigin: true },
      // Uploaded media is served by the Go server from data/uploads.
      '/uploads': { target: API_TARGET, changeOrigin: true },
    },
  },
  build: {
    // Article bodies are stored as HTML, so the JS payload is the only real
    // cost; splitting the heavy editor out keeps the public pages light.
    chunkSizeWarningLimit: 900,
    rollupOptions: {
      output: {
        // Written as a function rather than the `{ chunk: [pkg] }` map form:
        // the map form fails overload resolution against Vite 8's Rollup
        // output types.
        manualChunks(id: string) {
          if (
            id.includes('node_modules/@tiptap') ||
            id.includes('node_modules/lowlight') ||
            id.includes('node_modules/dompurify')
          ) {
            return 'editor'
          }
          if (
            id.includes('node_modules/vue-router') ||
            /node_modules[\\/]@?vue[\\/]/.test(id)
          ) {
            return 'vendor'
          }
          return undefined
        },
      },
    },
  },
})
