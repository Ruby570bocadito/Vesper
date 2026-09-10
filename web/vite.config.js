import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

const API_PORT = process.env.VESPER_API_PORT || '8443'
const WS_PORT = process.env.VESPER_WS_PORT || API_PORT

export default defineConfig({
  plugins: [vue()],
  server: {
    port: 3000,
    proxy: {
      '/api': {
        target: `http://localhost:${API_PORT}`,
        changeOrigin: true,
      },
      '/ws': {
        target: `ws://localhost:${WS_PORT}`,
        ws: true,
        changeOrigin: true,
      },
    },
  },
})
