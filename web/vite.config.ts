import { svelte } from '@sveltejs/vite-plugin-svelte';
import { defineConfig } from 'vite';

export default defineConfig({
  plugins: [svelte()],
  build: { target: 'es2022', assetsInlineLimit: 0, cssCodeSplit: false, chunkSizeWarningLimit: 600 },
  server: { proxy: { '/api': 'http://127.0.0.1:8080', '/img': 'http://127.0.0.1:8080' } },
  test: { include: ['src/**/*.test.ts'] },
});
