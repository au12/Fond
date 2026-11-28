// Printing: paper sizes, and how a recipe or a cookbook becomes pages for them.
//
// A printed page is the same page the book viewer shows, laid out at the size of the paper. Type and
// margins are set for the sheet: 11 pt on a letter page, a little smaller on a card.

import { layout, partsOf, sectionsOf, type Fits, type Page, type Part } from './book';
import type { Cookbook, Recipe } from './types';

export interface Paper {
  key: string;
  label: string;
  group: 'Sheets' | 'Book pages' | 'Cards';
  /** Points. */
  w: number;
  h: number;
  /** Body text in points at Small, Medium and Large. */
  sizes: [number, number, number];
  /** Margins against the book proportions: smaller on a card. */
  mg: number;
  /** CSS @page size. */
  css: string;
}

const IN = 72;
const MM = 72 / 25.4;
export const PAPERS: Paper[] = [
  { key: 'letter', label: 'Letter', group: 'Sheets', w: 8.5 * IN, h: 11 * IN, sizes: [10, 11, 12.5], mg: 0.9, css: 'letter' },
  { key: 'a4', label: 'A4', group: 'Sheets', w: 210 * MM, h: 297 * MM, sizes: [10, 11, 12.5], mg: 0.9, css: 'A4' },
  { key: 'legal', label: 'Legal', group: 'Sheets', w: 8.5 * IN, h: 14 * IN, sizes: [10, 11, 12.5], mg: 0.9, css: 'legal' },
  { key: 'half', label: 'Half letter', group: 'Sheets', w: 5.5 * IN, h: 8.5 * IN, sizes: [9, 10, 11], mg: 0.85, css: '5.5in 8.5in' },
  { key: 'a5', label: 'A5', group: 'Sheets', w: 148 * MM, h: 210 * MM, sizes: [9, 10, 11], mg: 0.85, css: 'A5' },
  { key: 'book6x9', label: '6 × 9 in', group: 'Book pages', w: 6 * IN, h: 9 * IN, sizes: [9.5, 10.5, 11.5], mg: 1, css: '6in 9in' },
  { key: 'book7x10', label: '7 × 10 in', group: 'Book pages', w: 7 * IN, h: 10 * IN, sizes: [10, 11, 12], mg: 1, css: '7in 10in' },
  { key: 'book8x10', label: '8 × 10 in', group: 'Book pages', w: 8 * IN, h: 10 * IN, sizes: [10, 11, 12], mg: 0.95, css: '8in 10in' },
  { key: 'card5x7', label: '5 × 7 card', group: 'Cards', w: 5 * IN, h: 7 * IN, sizes: [8, 9, 10], mg: 0.65, css: '5in 7in' },
  { key: 'card4x6', label: '4 × 6 card', group: 'Cards', w: 6 * IN, h: 4 * IN, sizes: [7.5, 8.5, 9.5], mg: 0.55, css: '6in 4in' },
  { key: 'card3x5', label: '3 × 5 card', group: 'Cards', w: 5 * IN, h: 3 * IN, sizes: [6.5, 7.5, 8.5], mg: 0.5, css: '5in 3in' },
];
export const paperOf = (key: string) => PAPERS.find((p) => p.key === key) ?? PAPERS[0];

/** The page viewer's type is 2.7% of the page's width; this is how far to turn it for a body size in points. */
export const typeScale = (paper: Paper, size: 0 | 1 | 2) => paper.sizes[size] / (0.027 * paper.w);

export interface PrintOptions {
  photo: 'top' | 'page' | 'off';
  notes: boolean;
}

/** A recipe printed on its own: the photograph across the top (or on a page of its own), then as many pages as it needs. */
export function recipePages(r: Recipe, fits: Fits, o: PrintOptions): Page[] {
  const parts = partsOf(r, '', 0, fits, { photo: o.photo === 'top', notes: o.notes });
  const pages: Page[] = parts.map((part: Part) => ({ kind: 'recipe', r, n: 0, chapter: '', part }));
  if (o.photo === 'page' && r.image) pages.unshift({ kind: 'photo', r });
  return pages;
}

export interface BookOptions {
  photos: boolean;
  notes: boolean;
  cover: boolean;
  endpapers: boolean;
  /** Printing on both sides: blank pages stay, so every photograph faces its recipe. */
  duplex: boolean;
}

/** A whole cookbook: the book's own pages, less what the reader chose to leave out. */
export function bookPages(book: Cookbook, recipes: Recipe[], fits: Fits, o: BookOptions): { page: Page; index: number }[] {
  const sections = sectionsOf(book, o.photos ? recipes : recipes.map((r) => ({ ...r, image: '' })));
  const parts = new Map<number, Part[]>();
  for (const g of sections) for (const it of g.items) parts.set(it.r.id, partsOf(it.r, g.name, it.n, fits, { notes: o.notes }));
  return layout(book, sections, parts)
    .map((page, index) => ({ page, index }))
    .filter(({ page }) =>
      page.kind === 'cover' || page.kind === 'back' ? o.cover : page.kind === 'endpaper' ? o.endpapers : page.kind === 'blank' ? o.duplex : true,
    );
}

// ---------------------------------------------------------------- exporting a recipe as text

import { minutesLabel } from './steps';

function facts(r: Recipe): string[] {
  return [r.yield && `Makes: ${r.yield}`, r.prepMin && `Prep: ${minutesLabel(r.prepMin)}`, r.cookMin && `Cook: ${minutesLabel(r.cookMin)}`, r.totalMin && `Total: ${minutesLabel(r.totalMin)}`].filter(Boolean) as string[];
}

export function asMarkdown(r: Recipe): string {
  const out = [`# ${r.title}`, ''];
  if (r.description) out.push(r.description, '');
  const f = facts(r);
  if (f.length) out.push(f.join(' · '), '');
  out.push('## Ingredients', '');
  for (const l of r.ingredients) out.push(l.h ? `\n### ${l.text}\n` : `- ${l.text}`);
  out.push('', '## Method', '');
  let n = 0;
  for (const l of r.steps) out.push(l.h ? `\n### ${l.text}\n` : `${++n}. ${l.text}`);
  if (r.notes) out.push('', '## Notes', '', r.notes);
  if (r.sourceUrl || r.author) out.push('', `Source: ${[r.author, r.sourceName || r.sourceUrl].filter(Boolean).join(', ')}${r.sourceUrl ? ` <${r.sourceUrl}>` : ''}`);
  if (r.tags.length) out.push('', `Tags: ${r.tags.join(', ')}`);
  return out.join('\n').replace(/\n{3,}/g, '\n\n') + '\n';
}

export function asText(r: Recipe): string {
  const rule = '-'.repeat(Math.min(60, Math.max(12, r.title.length)));
  const out = [r.title.toUpperCase(), rule];
  if (r.description) out.push(r.description);
  const f = facts(r);
  if (f.length) out.push('', f.join('   '));
  out.push('', 'INGREDIENTS');
  for (const l of r.ingredients) out.push(l.h ? `\n${l.text}:` : `  ${l.text}`);
  out.push('', 'METHOD');
  let n = 0;
  for (const l of r.steps) out.push(l.h ? `\n${l.text}:` : `  ${++n}. ${l.text}`);
  if (r.notes) out.push('', 'NOTES', r.notes);
  if (r.sourceUrl) out.push('', r.sourceUrl);
  return out.join('\n') + '\n';
}

/** schema.org Recipe, which other recipe apps import. */
export function asJsonLd(r: Recipe, image = ''): string {
  const iso = (m: number) => (m > 0 ? `PT${m}M` : undefined);
  const ld: Record<string, unknown> = {
    '@context': 'https://schema.org', '@type': 'Recipe', name: r.title, description: r.description || undefined,
    image: image || undefined, recipeYield: r.yield || undefined, prepTime: iso(r.prepMin), cookTime: iso(r.cookMin), totalTime: iso(r.totalMin),
    author: r.author ? { '@type': 'Person', name: r.author } : undefined, isBasedOn: r.sourceUrl || undefined,
    keywords: r.tags.length ? r.tags.join(', ') : undefined,
    recipeIngredient: r.ingredients.filter((l) => !l.h).map((l) => l.text),
    recipeInstructions: r.steps.filter((l) => !l.h).map((l) => ({ '@type': 'HowToStep', text: l.text })),
    comment: r.notes ? [{ '@type': 'Comment', text: r.notes }] : undefined,
  };
  return JSON.stringify(ld, null, 2) + '\n';
}

export function download(name: string, text: string, type: string) {
  const a = document.createElement('a');
  a.href = URL.createObjectURL(new Blob([text], { type }));
  a.download = name;
  a.click();
  setTimeout(() => URL.revokeObjectURL(a.href), 1000);
}
