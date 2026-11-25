// Coverage over a corpus of real ingredient lines. Not run by default: point
// FOND_LINES at a JSON file holding an array of lines (or of [site, language,
// line] rows) and run
//   FOND_LINES=lines.json npx vitest run corpus --silent=false
// to see how many measured lines Fond can weigh and which names it misses most.

import { readFileSync } from 'node:fs';
import { expect, it } from 'vitest';
import foods from '../data/foods.json';
import { loadFoods } from './foods';
import { lookup, parseIngredient, weigh } from './ingredients';

loadFoods(foods as any);

const path = process.env.FOND_LINES;

it.skipIf(!path)('weighs most of what real recipes measure by volume', () => {
  const data: (string | [string, string, string])[] = JSON.parse(readFileSync(path!, 'utf8'));
  const lines = data.flatMap((row) => (typeof row === 'string' ? [row] : /^en/i.test(row[1] ?? '') ? [row[2]] : []));
  const n: Record<string, number> = {};
  const add = (k: string) => (n[k] = (n[k] ?? 0) + 1);
  const missed = new Map<string, number>();
  for (const line of lines) {
    const p = parseIngredient(line);
    if (p.qty == null) { add('unmeasured'); continue; }
    if (p.unit?.kind === 'weight') { add('by weight'); continue; }
    const m = lookup(p);
    if (m.skip) { add('not weighed'); continue; }
    const kind = p.unit?.kind === 'volume' ? (p.unit.key === 'tsp' || p.unit.key === 'tbsp' ? 'spoons' : 'cups') : 'counts';
    const w = weigh(p);
    add(kind);
    if (w) {
      add(kind + ' weighed');
      if (w.approx) add(kind + ' estimated');
    } else if (kind !== 'counts') {
      const name = m.phrase || p.rest;
      missed.set(name, (missed.get(name) ?? 0) + 1);
    }
  }
  const pct = (k: string) => `${n[k + ' weighed'] ?? 0}/${n[k] ?? 0} (${(((n[k + ' weighed'] ?? 0) / (n[k] || 1)) * 100).toFixed(1)} %, ${n[k + ' estimated'] ?? 0} as estimates)`;
  const top = [...missed].sort((a, b) => b[1] - a[1]).slice(0, 60).map(([k, v]) => `${k} ×${v}`).join(', ');
  console.log(
    [
      `${lines.length} lines: ${n['unmeasured'] ?? 0} without an amount, ${n['by weight'] ?? 0} already by weight, ${n['not weighed'] ?? 0} not worth weighing`,
      `cups and millilitres  ${pct('cups')}`,
      `spoons                ${pct('spoons')}`,
      `counted items         ${pct('counts')}`,
      `missed most often: ${top}`,
    ].join('\n'),
  );
  // The bar the dictionary was built to: nineteen in twenty cup measures.
  expect((n['cups weighed'] ?? 0) / (n['cups'] || 1)).toBeGreaterThan(0.95);
});
