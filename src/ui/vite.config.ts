import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// The dev server proxies /v1 to the Go server, so the UI needs no CORS.
export default defineConfig({
  plugins: [react()],
  server: {
    port: Number(process.env.PORT) || 5173,
    proxy: { '/v1': process.env.API_URL ?? 'http://localhost:8080' },
  },
})
