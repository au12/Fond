import { svelte } from '@sveltejs/vite-plugin-svelte';
import { defineConfig } from 'vite';

// Phones turn a tap into a :hover that sticks until the next tap somewhere else, so a tapped button kept its
// hover background and a tapped tile stayed zoomed. Every :hover rule is built behind @media (hover: hover),
// so hover styles only reach devices with a real pointer. Selectors without :hover in the same rule stay put.
const hoverOnlyWithPointer = {
  postcssPlugin: 'hover-only-with-pointer',
  Rule(rule: any, { AtRule }: any) {
    if (!rule.selector.includes(':hover')) return;
    if (rule.parent?.type === 'atrule' && rule.parent.name === 'media' && rule.parent.params === '(hover: hover)') return;
    const hover = rule.selectors.filter((s: string) => s.includes(':hover'));
    const rest = rule.selectors.filter((s: string) => !s.includes(':hover'));
    rule.after(new AtRule({ name: 'media', params: '(hover: hover)' }).append(rule.clone({ selectors: hover })));
    if (rest.length) rule.selectors = rest;
    else rule.remove();
  },
};

export default defineConfig({
  plugins: [svelte()],
  css: { postcss: { plugins: [hoverOnlyWithPointer] } },
  build: { target: 'es2022', assetsInlineLimit: 0, cssCodeSplit: false, chunkSizeWarningLimit: 600 },
  server: { proxy: { '/api': 'http://127.0.0.1:8080', '/img': 'http://127.0.0.1:8080' } },
  test: { include: ['src/**/*.test.ts'] },
});
