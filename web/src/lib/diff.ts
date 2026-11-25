// A small line-and-word diff for the recipe history: what was added, what was
// taken out, and inside a line that was only edited, which words changed.

import type { Line } from './types';

export interface Seg {
  t: 'same' | 'add' | 'del';
  v: string;
}

export interface Row {
  kind: 'same' | 'add' | 'del' | 'change';
  /** A section heading rather than an ingredient or a step. */
  h: boolean;
  /** The line as it stands after the change (or as it was, for a removal). */
  text: string;
  /** For an edited line: the words, marked as kept, added or removed. */
  segs?: Seg[];
}

/** Longest common subsequence, as index pairs. */
function lcs<T>(a: T[], b: T[], same: (x: T, y: T) => boolean): [number, number][] {
  const n = a.length;
  const m = b.length;
  const dp: Uint16Array[] = Array.from({ length: n + 1 }, () => new Uint16Array(m + 1));
  for (let i = n - 1; i >= 0; i--) {
    for (let j = m - 1; j >= 0; j--) dp[i][j] = same(a[i], b[j]) ? dp[i + 1][j + 1] + 1 : Math.max(dp[i + 1][j], dp[i][j + 1]);
  }
  const out: [number, number][] = [];
  for (let i = 0, j = 0; i < n && j < m; ) {
    if (same(a[i], b[j])) out.push([i++, j++]);
    else if (dp[i + 1][j] >= dp[i][j + 1]) i++;
    else j++;
  }
  return out;
}

const tokens = (s: string) => s.split(/(\s+)/).filter((t) => t !== '');

/** The words of one line against another. */
export function diffWords(before: string, after: string): Seg[] {
  const a = tokens(before);
  const b = tokens(after);
  const pairs = lcs(a, b, (x, y) => x === y);
  const out: Seg[] = [];
  const push = (t: Seg['t'], v: string) => {
    const last = out[out.length - 1];
    if (last && last.t === t) last.v += v;
    else out.push({ t, v });
  };
  let i = 0;
  let j = 0;
  for (const [pi, pj] of [...pairs, [a.length, b.length] as [number, number]]) {
    while (i < pi) push('del', a[i++]);
    while (j < pj) push('add', b[j++]);
    if (pi < a.length) push('same', a[pi]);
    i = pi + 1;
    j = pj + 1;
  }
  return gather(out.filter((s) => s.v !== ''));
}

/**
 * "45 minutes" → "1 hour" comes out of the diff as two swaps around a space.
 * It reads better as one: gather changes that only whitespace separates.
 */
function gather(segs: Seg[]): Seg[] {
  const out: Seg[] = [];
  let del = '';
  let add = '';
  const flush = () => {
    if (del) out.push({ t: 'del', v: del });
    if (add) out.push({ t: 'add', v: add });
    del = add = '';
  };
  segs.forEach((seg, i) => {
    if (seg.t === 'del') del += seg.v;
    else if (seg.t === 'add') add += seg.v;
    else if ((del || add) && /^\s+$/.test(seg.v) && segs[i + 1] && segs[i + 1].t !== 'same') {
      // A space inside a run of changes belongs to both the old words and the new.
      del += seg.v;
      add += seg.v;
    } else {
      flush();
      out.push(seg);
    }
  });
  flush();
  return out;
}

/** How alike two lines are, 0 to 1, by the words they share (amounts aside). */
function alike(a: string, b: string): number {
  const words = (s: string) => new Set((s.toLowerCase().match(/[a-z\u00c0-\u024f]+/g) ?? []).map((w) => (w.length > 3 ? w.replace(/s$/, '') : w)));
  const x = words(a);
  const y = words(b);
  if (!x.size || !y.size) return 0;
  let both = 0;
  for (const w of x) if (y.has(w)) both++;
  return (2 * both) / (x.size + y.size);
}

/** What changed between two lists of ingredients or steps. */
export function diffLines(before: Line[], after: Line[]): Row[] {
  const key = (l: Line) => (l.h ? '#' : '') + l.text.replace(/\s+/g, ' ').trim();
  const pairs = lcs(before, after, (x, y) => key(x) === key(y));
  const rows: Row[] = [];
  let i = 0;
  let j = 0;
  for (const [pi, pj] of [...pairs, [before.length, after.length] as [number, number]]) {
    const gone = before.slice(i, pi);
    const come = after.slice(j, pj);
    // A removal beside an addition is usually one line, edited: pair them where they resemble each other.
    const used = new Set<number>();
    for (const old of gone) {
      let best = -1;
      let score = 0.4;
      come.forEach((c, k) => {
        if (used.has(k) || !!c.h !== !!old.h) return;
        const s = alike(old.text, c.text);
        if (s > score) {
          score = s;
          best = k;
        }
      });
      if (best < 0) rows.push({ kind: 'del', h: !!old.h, text: old.text });
      else {
        // Lines that come before their partner are plain additions.
        for (let k = 0; k < best; k++) {
          if (!used.has(k)) {
            used.add(k);
            rows.push({ kind: 'add', h: !!come[k].h, text: come[k].text });
          }
        }
        used.add(best);
        rows.push({ kind: 'change', h: !!come[best].h, text: come[best].text, segs: diffWords(old.text, come[best].text) });
      }
    }
    come.forEach((c, k) => {
      if (!used.has(k)) rows.push({ kind: 'add', h: !!c.h, text: c.text });
    });
    if (pi < before.length) rows.push({ kind: 'same', h: !!after[pj].h, text: after[pj].text });
    i = pi + 1;
    j = pj + 1;
  }
  return rows;
}

export interface Tally {
  added: number;
  removed: number;
  changed: number;
}

export function tally(rows: Row[]): Tally {
  const t = { added: 0, removed: 0, changed: 0 };
  for (const r of rows) {
    if (r.kind === 'add') t.added++;
    else if (r.kind === 'del') t.removed++;
    else if (r.kind === 'change') t.changed++;
  }
  return t;
}

/** "2 changed, 1 added" */
export function tallyText(t: Tally): string {
  const parts: string[] = [];
  if (t.changed) parts.push(`${t.changed} changed`);
  if (t.added) parts.push(`${t.added} added`);
  if (t.removed) parts.push(`${t.removed} removed`);
  return parts.join(', ');
}
