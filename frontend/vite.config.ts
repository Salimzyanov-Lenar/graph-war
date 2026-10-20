import react from '@vitejs/plugin-react';
import { defineConfig } from 'vitest/config';

const backend = process.env.GRAPHWAR_BACKEND ?? '127.0.0.1:8080';

export default defineConfig({
  plugins: [react()],
  server: {
    // Единый origin с бэкендом (cookie, Origin allowlist): /api и /ws идут на локальный сервер.
    proxy: {
      '/api': `http://${backend}`,
      '/healthz': `http://${backend}`,
      '/readyz': `http://${backend}`,
      '/ws': { target: `ws://${backend}`, ws: true },
    },
  },
  test: {
    environment: 'jsdom',
    include: ['src/**/*.test.{ts,tsx}'],
  },
});
