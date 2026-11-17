import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// Vite config for the ATMOS React frontend (served on :5173, proxies /api to the Go backend on :8080).
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      '/api': { target: 'http://localhost:8081', changeOrigin: true },
      '/tiles': {
        target: 'https://tile.openstreetmap.org',
        changeOrigin: true,
        rewrite: (p) => p.replace(/^\/tiles/, ''),
      },
    },
  },
})
