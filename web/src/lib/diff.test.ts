import { describe, expect, it } from 'vitest';
import { diffLines, diffWords, tally, tallyText } from './diff';

const L = (...texts: string[]) => texts.map((text) => (text.startsWith('# ') ? { text: text.slice(2), h: true } : { text }));
const kinds = (a: string[], b: string[]) => diffLines(L(...a), L(...b)).map((r) => `${r.kind}:${r.text}`);

describe('line diff', () => {
  it('finds nothing in identical lists', () => {
    expect(kinds(['1 cup flour', '1 tsp salt'], ['1 cup flour', '1 tsp salt'])).toEqual(['same:1 cup flour', 'same:1 tsp salt']);
    expect(kinds(['1 cup  flour'], ['1 cup flour'])).toEqual(['same:1 cup flour']);
  });
  it('sees additions and removals in place', () => {
    expect(kinds(['flour', 'salt'], ['flour', 'yeast', 'salt'])).toEqual(['same:flour', 'add:yeast', 'same:salt']);
    expect(kinds(['flour', 'yeast', 'salt'], ['flour', 'salt'])).toEqual(['same:flour', 'del:yeast', 'same:salt']);
    expect(kinds([], ['flour'])).toEqual(['add:flour']);
    expect(kinds(['flour'], [])).toEqual(['del:flour']);
  });
  it('treats an edited line as one change, not a removal and an addition', () => {
    const rows = diffLines(L('3 cups flour', '1 tsp salt'), L('3 cups (360 g) flour', '1 tsp salt'));
    expect(rows.map((r) => r.kind)).toEqual(['change', 'same']);
    expect(rows[0].segs).toEqual([{ t: 'same', v: '3 cups ' }, { t: 'add', v: '(360 g) ' }, { t: 'same', v: 'flour' }]);
  });
  it('does not pair lines that have nothing in common', () => {
    expect(kinds(['1 cup milk'], ['2 eggs'])).toEqual(['del:1 cup milk', 'add:2 eggs']);
  });
  it('keeps headings apart from ingredients', () => {
    const rows = diffLines(L('# Dough', 'flour'), L('# For the dough', 'flour'));
    expect(rows[0]).toMatchObject({ kind: 'change', h: true });
    expect(kinds(['Dough'], ['# Dough'])).toEqual(['del:Dough', 'add:Dough']);
  });
  it('handles a reordering', () => {
    const rows = kinds(['a b c', 'd e f', 'g h i'], ['g h i', 'a b c', 'd e f']);
    expect(rows.filter((r) => r.startsWith('same')).length).toBe(2);
    expect(rows).toContain('add:g h i');
    expect(rows).toContain('del:g h i');
  });
  it('counts', () => {
    const t = tally(diffLines(L('1 cup flour', 'salt', 'water'), L('2 cups flour', 'water', 'yeast')));
    expect(t).toEqual({ added: 1, removed: 1, changed: 1 });
    expect(tallyText(t)).toBe('1 changed, 1 added, 1 removed');
    expect(tallyText({ added: 0, removed: 0, changed: 0 })).toBe('');
  });
});

describe('word diff', () => {
  it('marks swapped words', () => {
    expect(diffWords('Bake for 30 minutes.', 'Bake for 35 minutes.')).toEqual([
      { t: 'same', v: 'Bake for ' }, { t: 'del', v: '30' }, { t: 'add', v: '35' }, { t: 'same', v: ' minutes.' },
    ]);
  });
  it('gathers neighbouring changes into one', () => {
    expect(diffWords('Rest for 45 minutes.', 'Rest for 1 hour.')).toEqual([{ t: 'same', v: 'Rest for ' }, { t: 'del', v: '45 minutes.' }, { t: 'add', v: '1 hour.' }]);
  });
  it('rebuilds both sides exactly', () => {
    const a = 'Mix the flour and salt, then rest.';
    const b = 'Whisk the flour, sugar and salt.';
    const segs = diffWords(a, b);
    expect(segs.filter((s) => s.t !== 'add').map((s) => s.v).join('')).toBe(a);
    expect(segs.filter((s) => s.t !== 'del').map((s) => s.v).join('')).toBe(b);
  });
});
