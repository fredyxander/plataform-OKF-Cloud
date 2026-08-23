import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/auth':      'http://localhost:8080',
      '/documents': 'http://localhost:8080',
      '/jobs':      'http://localhost:8080',
    }
  }
})