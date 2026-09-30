import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

const target = process.env.HALCON_BACKEND_URL || 'http://127.0.0.1:3300'
// Preserve Host and Origin together: backend WebSocket origin checks remain enabled.
const proxy = { target, changeOrigin: false, ws: true }
export default defineConfig({ plugins: [react()], server: { port: 5173, strictPort: true, proxy: { '/api': proxy, '/location': proxy, '/dashboard': proxy } } })
