import { defineConfig } from 'vitest/config';

export default defineConfig({
  test: {
    typecheck: {
      enabled: true,
      include: ['**/*.test.ts', '**/*.test.tsx', '**/*.spec.ts', '**/*.spec.tsx'],
      tsconfig: './tsconfig.test.json',
    },
    environment: 'jsdom',
    setupFiles: ['./vitest.setup.ts'],
  },
}); 
