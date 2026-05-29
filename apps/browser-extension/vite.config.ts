import { defineConfig } from 'vite';
import { resolve } from 'path';

export default defineConfig({
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    rollupOptions: {
      input: {
        popup: resolve(__dirname, 'popup/index.html'),
        background: resolve(__dirname, 'src/background/service-worker.ts'),
        content: resolve(__dirname, 'src/content/content-script.ts'),
      },
      output: {
        entryFileNames: (chunk) => {
          if (chunk.name === 'background') return 'background/service-worker.js';
          if (chunk.name === 'content') return 'content/content-script.js';
          return 'popup/[name].js';
        },
        chunkFileNames: 'shared/[name].js',
        assetFileNames: (assetInfo) => {
          const info = assetInfo.name || '';
          if (info.endsWith('.css')) return 'popup/[name][extname]';
          if (info.endsWith('.html')) return 'popup/[name][extname]';
          return 'assets/[name][extname]';
        },
      },
    },
  },
  resolve: {
    alias: {
      '@shared': resolve(__dirname, 'src/shared'),
    },
  },
  publicDir: 'public',
});
