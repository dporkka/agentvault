import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'path'

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
      '@agentvault/contract': path.resolve(__dirname, '../../../packages/contract/src'),
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
})
