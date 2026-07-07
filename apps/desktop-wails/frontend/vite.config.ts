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
    // Desktop assets are embedded, so a ~550 kB vendor chunk is fine; raise
    // the warning threshold rather than splitting into many tiny chunks.
    chunkSizeWarningLimit: 700,
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
            // Split CodeMirror into smaller chunks so no single embedded vendor
            // chunk trips Vite's default 500 kB warning.
            if (id.includes('@codemirror/view')) {
              return 'codemirror-view';
            }
            if (id.includes('@codemirror/state')) {
              return 'codemirror-state';
            }
            if (id.includes('@uiw/react-codemirror')) {
              return 'codemirror-uiw';
            }
            if (id.includes('@codemirror/lang-markdown') || id.includes('@lezer')) {
              return 'codemirror-lang';
            }
            if (id.includes('@codemirror') || id.includes('codemirror')) {
              return 'codemirror-core';
            }
            return 'vendor';
          }
        },
      },
    },
  },
})
