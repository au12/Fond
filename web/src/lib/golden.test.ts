// A golden set: several hundred real ingredient lines, each with the food Fond
// should recognise and the weight it should give. The lines come from recipes
// in the wild (and a block of staples and awkward cases at the end); the
// answers were read through by eye against the source data.
//
// When the dictionary changes on purpose, refresh the answers with
//   FOND_GOLDEN_UPDATE=1 npm test -- golden
// and read the diff before committing it.

import { readFileSync, writeFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import foods from '../data/foods.json';
import { loadFoods } from './foods';
import { lookup, parseIngredient, weigh } from './ingredients';

loadFoods(foods as any);

type Row = [line: string, food: string, grams: number | null, approx: 0 | 1];
const file = fileURLToPath(new URL('./golden.json', import.meta.url));
const rows: Row[] = JSON.parse(readFileSync(file, 'utf8'));

function answer(line: string): Row {
  const p = parseIngredient(line);
  const m = lookup(p);
  const w = weigh(p);
  const food = m.skip ? 'skip' : (w?.match?.food?.id ?? m.food?.id ?? '');
  return [line, food, w ? Math.round(w.g * 10) / 10 : null, w?.approx ? 1 : 0];
}

describe('golden set', () => {
  if (process.env.FOND_GOLDEN_UPDATE) {
    it('rewrites the answers', () => {
      const next = rows.map(([line]) => answer(line));
      writeFileSync(file, '[\n' + next.map((r) => '  ' + JSON.stringify(r)).join(',\n') + '\n]\n');
    });
    return;
  }
  it('has enough lines to mean something', () => {
    expect(rows.length).toBeGreaterThan(400);
  });
  it('recognises and weighs every line as recorded', () => {
    const wrong: string[] = [];
    for (const row of rows) {
      const got = answer(row[0]);
      const sameGrams = row[2] == null || got[2] == null ? row[2] === got[2] : Math.abs(row[2] - got[2]) <= Math.max(0.1, row[2] * 0.005);
      if (got[1] !== row[1] || !sameGrams || got[3] !== row[3]) wrong.push(`${row[0]}\n      expected ${row[1] || '∅'} ${row[2]}${row[3] ? ' ≈' : ''}\n      got      ${got[1] || '∅'} ${got[2]}${got[3] ? ' ≈' : ''}`);
    }
    expect(wrong, `\n${wrong.join('\n')}\n`).toEqual([]);
  });
});
