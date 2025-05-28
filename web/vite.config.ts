import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import { viteStaticCopy } from 'vite-plugin-static-copy';
import path from 'path';

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [
    react(),
    // When building, vite removes README.md from the dist directory which makes the git tree dirty as it is in the .gitignore.
    // This is needed because otherwise the go build fails with "pattern dist: no matching files found".
    // This copies README.md back into the dist directory.
    viteStaticCopy({
      targets: [
        {
          src: path.resolve(__dirname, './README.md'),
          dest: './',
        },
      ],
    }),
  ],
  optimizeDeps: {
    exclude: ['lucide-react'],
  },
});
