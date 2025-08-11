import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import netlify from '@netlify/vite-plugin';
import { viteStaticCopy } from 'vite-plugin-static-copy';
import path from 'path';
import { InitialState } from './src/types';

// https://vitejs.dev/config/
export default defineConfig(() => {
  return {
    plugins: [
      {
        name: 'gomplate-html-transform',
        transformIndexHtml(html) {
          return templateHTML(html);

        },
      },
      react(),
      // netlify middleware to emulate netlify functions in local dev
      netlify(),
      viteStaticCopy({
        targets: [
          // When building, vite removes README.md from the dist directory which makes the git tree dirty as it is in the .gitignore.
          // This is needed because otherwise the go build fails with "pattern dist: no matching files found".
          // This copies README.md back into the dist directory.
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


// templateHTML templated fields in our `index.html` file which is production is handled by our go server with JSON.stringify(values.key).
function templateHTML(html: string) {
  const values = {
    Title: 'My Dev App',
    InitialState: {
      icon: 'does-not-exist.png',
      title: 'mock execution',
      installTarget: 'linux',
    } as InitialState,
  };

  // Quick way to replace {{ .key }} with JSON.stringify(values.key). Given how simple our templates are, this is sufficient for now.
  const transformed = html.replace(/\{\{\s*\.(\w+)\s*\}\}/g, (_, key: string) => {
    if (key in values) {
      return JSON.stringify(values[key as keyof typeof values]);
    }
    // Return empty string if key does not exist in values
    return ''
  });
  return transformed;

}
