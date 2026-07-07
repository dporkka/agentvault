import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'path'

// Helper to extract the npm package name from a node_modules path so chunks
// are assigned consistently regardless of monorepo nesting.
function pkgName(id: string): string {
  const m = id.match(/node_modules\/(@[^/]+\/[^/]+|[^/]+)/);
  return m ? m[1] : '';
}

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
    // The editor is lazy-loaded, but CodeMirror's core view code is still
    // ~560 kB after minification. We split out the language support, React
    // wrapper, and markdown renderer, then budget the remaining core chunk
    // because desktop assets are embedded and a single monolithic editor
    // chunk is preferable to many tiny chunks at runtime.
    chunkSizeWarningLimit: 600,
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (!id.includes('node_modules')) {
            return;
          }
          const name = pkgName(id);

          // Markdown renderer used by the preview pane.
          if (name === 'react-markdown') {
            return 'markdown-vendor';
          }

          // React runtime.
          if (name === 'react' || name === 'react-dom') {
            return 'react-vendor';
          }

          // CodeMirror language support (lazy-loaded in EditorView).
          if (name === '@codemirror/lang-markdown' || name.startsWith('@lezer/')) {
            return 'codemirror-lang';
          }

          // The React wrapper around CodeMirror.
          if (name === '@uiw/react-codemirror') {
            return 'codemirror-uiw';
          }

          // Core CodeMirror packages. Split the two largest so no single chunk
          // exceeds the default warning threshold.
          if (name === '@codemirror/view') {
            return 'codemirror-view';
          }
          if (name === '@codemirror/state') {
            return 'codemirror-state';
          }
          if (name.startsWith('@codemirror/') || name === 'codemirror') {
            return 'codemirror-core';
          }

          return 'vendor';
        },
      },
    },
  },
})
