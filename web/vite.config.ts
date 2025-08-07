import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import netlify from '@netlify/vite-plugin';
import { viteStaticCopy } from 'vite-plugin-static-copy';
import path from 'path';
import { InitialState } from './src/types';

// https://vitejs.dev/config/
export default defineConfig(({ mode }) => {
  const isDev = mode === 'development';
  return {
    // Treat html files as assets to be copied
    // assetsInclude: ['**/*.html'],
    plugins: [
      {
        name: 'gomplate-html-transform',
        transformIndexHtml(html) {
          if (!isDev) return html;

          return templateHTML(html);

        },
      },
      react(),
      netlify(),
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
  }
});


function templateHTML(html: string) {
  const values = {
    Title: 'My Dev App',
    InitialState: {
      icon: 'does-not-exist.png',
      title: 'mock execution',
      installTarget: 'linux',
    } as InitialState,
  };

  const transformed = html.replace(/\{\{\s*\.(\w+)\s*\}\}/g, (_, key) => {
    return JSON.stringify(values[key] || '');
  });
  return transformed;

}
