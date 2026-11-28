// A cookbook as a book: the pages in order, and how they pair into spreads.
//
// Every size on a page is a fraction of the page's width, so a page laid out once at any size
// breaks the same way at every size. Pagination measures each recipe once, out of sight, and
// the result holds on a phone and on a desktop alike.

import type { Cookbook, Recipe } from './types';

export type Page =
  | { kind: 'cover' }
  | { kind: 'endpaper' }
  | { kind: 'title' }
  | { kind: 'toc'; part: number; parts: number; rows: TocRow[] }
  | { kind: 'intro' }
  | { kind: 'chapter'; name: string; n: number }
  | { kind: 'chapterlist'; name: string; rows: TocRow[] }
  | { kind: 'photo'; r: Recipe }
  | { kind: 'recipe'; r: Recipe; n: number; chapter: string; part: Part }
  | { kind: 'blank' }
  | { kind: 'back' };

/** One page's share of a recipe: the steps from..to, at a tightness, and whether it starts or continues. */
export interface Part {
  from: number;
  to: number;
  fit: number;
  first: boolean;
  notes: boolean;
  /** The last page of the recipe, which carries its footnotes (source, QR code, nutrition). */
  last?: boolean;
  /** A photograph across the top of the first page, for a recipe printed on its own. */
  photo?: boolean;
}

export type TocRow = { kind: 'chapter'; name: string } | { kind: 'recipe'; title: string; n: number; page: number };

export interface Section {
  name: string;
  items: { r: Recipe; n: number }[];
}

/** Recipes in reading order, grouped by chapter and numbered through the book. */
export function sectionsOf(book: Cookbook, recipes: Recipe[]): Section[] {
  const byId = new Map(recipes.map((r) => [r.id, r]));
  const order = [...book.chapters];
  for (const e of book.entries ?? []) if (e.chapter && !order.includes(e.chapter)) order.push(e.chapter);
  const groups: Section[] = [{ name: '', items: [] }, ...order.map((name) => ({ name, items: [] as Section['items'] }))];
  for (const e of book.entries ?? []) {
    const r = byId.get(e.recipeId);
    if (r) (groups.find((g) => g.name === e.chapter) ?? groups[0]).items.push({ r, n: 0 });
  }
  let n = 0;
  const out = groups.filter((g) => g.items.length);
  for (const g of out) for (const it of g.items) it.n = ++n;
  return out;
}

/** Does this share of the recipe fit on one page? Supplied by the viewer, which can measure. */
export type Fits = (r: Recipe, part: Part, chapter: string, n: number) => boolean;

const FITS = [0, 1, 2, 3];

/** Splits a recipe over as few pages as it needs: one, at the roomiest setting that fits, or more. */
export function partsOf(r: Recipe, chapter: string, n: number, fits: Fits, o: { photo?: boolean; notes?: boolean } = {}): Part[] {
  const steps = r.steps.length;
  const hasNotes = !!r.notes?.trim() && o.notes !== false;
  const photo = !!o.photo && !!r.image;
  for (const fit of FITS) {
    const whole = { from: 0, to: steps, fit, first: true, notes: hasNotes, last: true, photo };
    if (fits(r, whole, chapter, n)) return [whole];
  }
  const fit = FITS[FITS.length - 1];
  const parts: Part[] = [];
  // The first page holds the ingredients and as many steps as fit beside them.
  let k = largest(0, steps, (to) => fits(r, { from: 0, to, fit, first: true, notes: false, photo }, chapter, n));
  parts.push({ from: 0, to: k, fit, first: true, notes: false, photo });
  // Continuation pages hold the rest of the method, then the notes.
  let guard = 0;
  while (k < steps && guard++ < 40) {
    const to = Math.max(k + 1, largest(k, steps, (t) => fits(r, { from: k, to: t, fit, first: false, notes: false }, chapter, n)));
    const withNotes = to === steps && hasNotes && fits(r, { from: k, to, fit, first: false, notes: true, last: true }, chapter, n);
    parts.push({ from: k, to, fit, first: false, notes: withNotes });
    k = to;
  }
  const last = parts[parts.length - 1];
  if (hasNotes && !last.notes) {
    if (fits(r, { ...last, notes: true, last: true }, chapter, n)) last.notes = true;
    else parts.push({ from: steps, to: steps, fit, first: false, notes: true });
  }
  const end = parts[parts.length - 1];
  end.last = true;
  // The footnotes need room on the last page too; when they don't fit there, they get a page of their own.
  if (!fits(r, end, chapter, n) && (end.to > end.from || end.notes)) {
    end.last = false;
    parts.push({ from: steps, to: steps, fit, first: false, notes: false, last: true });
  }
  return parts;
}

/** The largest `to` in (lo, hi] for which ok(to) holds; lo when none does. ok is assumed to hold up to a point. */
function largest(lo: number, hi: number, ok: (to: number) => boolean): number {
  let a = lo, b = hi;
  while (a < b) {
    const mid = Math.ceil((a + b + 1) / 2);
    if (ok(mid)) a = mid;
    else b = mid - 1;
  }
  return a;
}

const TOC_ROWS = 19;

/**
 * Lays the book out. A photograph always faces its recipe, on the left of a spread; a chapter
 * opens on a right-hand page. The cover stands alone, and so does the back.
 */
export function layout(book: Cookbook, sections: Section[], parts: Map<number, Part[]>): Page[] {
  // Contents first, with page numbers filled in once the rest is placed.
  const rows: TocRow[] = [];
  for (const g of sections) {
    if (g.name) rows.push({ kind: 'chapter', name: g.name });
    for (const it of g.items) rows.push({ kind: 'recipe', title: it.r.title, n: it.n, page: 0 });
  }
  const tocParts = Math.max(1, Math.ceil(rows.reduce((s, r) => s + (r.kind === 'chapter' ? 2 : 1), 0) / TOC_ROWS));
  const tocs: TocRow[][] = [];
  let cur: TocRow[] = [], used = 0;
  for (const row of rows) {
    const w = row.kind === 'chapter' ? 2 : 1;
    if (used + w > TOC_ROWS && cur.length) {
      tocs.push(cur);
      cur = [];
      used = 0;
    }
    cur.push(row);
    used += w;
  }
  tocs.push(cur);

  const p: Page[] = [{ kind: 'cover' }, { kind: 'endpaper' }, { kind: 'title' }];
  const right = () => p.length % 2 === 0; // even indexes are right-hand pages
  const onRight = () => { if (!right()) p.push({ kind: 'blank' }); };
  const onLeft = () => { if (right()) p.push({ kind: 'blank' }); };
  if (book.description?.trim()) p.push({ kind: 'intro' });
  onRight();
  tocs.forEach((t, i) => p.push({ kind: 'toc', part: i, parts: tocs.length, rows: t }));
  const first = new Map<number, number>(); // recipe number → its first page
  const lists: TocRow[][] = [];
  sections.forEach((g, gi) => {
    if (g.name) {
      // A chapter opens across a spread: its name on the left, what is in it on the right.
      onLeft();
      p.push({ kind: 'chapter', name: g.name, n: gi + (sections[0].name ? 1 : 0) });
      const rows: TocRow[] = g.items.slice(0, TOC_ROWS).map((it) => ({ kind: 'recipe', title: it.r.title, n: it.n, page: 0 }));
      lists.push(rows);
      p.push({ kind: 'chapterlist', name: g.name, rows });
    }
    for (const it of g.items) {
      if (it.r.image) {
        onLeft();
        first.set(it.n, p.length);
        p.push({ kind: 'photo', r: it.r });
      } else first.set(it.n, p.length);
      for (const part of parts.get(it.r.id) ?? [{ from: 0, to: it.r.steps.length, fit: 0, first: true, notes: true }]) {
        p.push({ kind: 'recipe', r: it.r, n: it.n, chapter: g.name, part });
      }
    }
  });
  // The back endpaper on a right-hand page, then the back cover on its own.
  onRight();
  p.push({ kind: 'endpaper' });
  p.push({ kind: 'back' });
  for (const t of [...tocs, ...lists]) for (const row of t) if (row.kind === 'recipe') row.page = folio(first.get(row.n) ?? 0);
  return p;
}

/** The number printed at the foot of a page. The cover and endpapers carry none, and the title page is page 1. */
export const folio = (index: number) => Math.max(0, index - 1);

/** Pairs pages into what a reader sees at once: [left, right], with the covers alone. */
export function spreads(pages: Page[]): [number | null, number | null][] {
  const out: [number | null, number | null][] = [[null, 0]];
  for (let i = 1; i < pages.length - 1; i += 2) out.push([i, i + 1 < pages.length - 1 ? i + 1 : null]);
  out.push([pages.length - 1, null]);
  return out;
}

/** On a phone, one page at a time, with the blank pages left out. */
export function singles(pages: Page[]): number[] {
  return pages.map((_, i) => i).filter((i) => pages[i].kind !== 'blank');
}
