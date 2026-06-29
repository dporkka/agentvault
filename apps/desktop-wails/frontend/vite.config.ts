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
    // The markdown editor bundles @codemirror/lang-markdown and its Lezer
    // parser. After trimming unused codeLanguages, the chunk is still ~605 kB
    // minified, so we intentionally budget it rather than fighting the warning.
    chunkSizeWarningLimit: 650,
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (id.includes('node_modules')) {
            if (id.includes('react-markdown')) {
              return 'markdown-vendor';
            }
            if (id.includes('react') || id.includes('react-dom')) {
              return 'react-vendor';
            }
            if (
              id.includes('@codemirror') ||
              id.includes('codemirror') ||
              id.includes('@lezer') ||
              id.includes('@uiw/react-codemirror')
            ) {
              return 'codemirror-vendor';
            }
            return 'vendor';
          }
        },
      },
    },
  },
})
