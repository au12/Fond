<script lang="ts">
  import BookDoc from '../components/BookDoc.svelte';
  import RecipeDoc from '../components/RecipeDoc.svelte';
  import { get } from '../lib/api';
  import { cookState } from '../lib/cook.svelte';
  import { app, oops, prefs, savePrefs } from '../lib/state.svelte';
  import { ArrowLeft, Printer } from '../lib/icons';
  import type { Cookbook, Recipe } from '../lib/types';
  import { ensureFoods } from '../lib/weigh.svelte';

  let { kind, id }: { kind: 'recipe' | 'cookbook'; id: number } = $props();
  const pf = prefs();
  let recipe = $state<Recipe | null>(null);
  let book = $state<Cookbook | null>(null);
  let recipes = $state<Recipe[]>([]);
  let look = $state<string>(pf.docStyle);
  let paper = $state<string>(pf.paper);
  let photo = $state(true);
  let notes = $state(true);
  let qr = $state(true);
  let size = $state(11);
  // Print what the cook was looking at: the recipe page passes its Weigh setting along.
  let weigh = $state(new URLSearchParams(app.query).has('weigh') ? new URLSearchParams(app.query).get('weigh') === '1' : pf.weigh);
  const scale = kind === 'recipe' ? cookState(id).scale : 1;
  const opts = $derived({ scale, system: pf.system, weigh, fractions: pf.fractions, flour: pf.flour });
  ensureFoods();

  if (kind === 'recipe') get<Recipe>(`/recipes/${id}`).then((r) => (recipe = r)).catch(oops);
  else get<{ cookbook: Cookbook; recipes: Recipe[] }>(`/cookbooks/${id}?full=1`).then((x) => { book = x.cookbook; recipes = x.recipes; look = x.cookbook.style; }).catch(oops);

  // Sizes in points. The preview is a scale model of the sheet, so what fits here fits on paper.
  const PAPER: Record<string, { css: string; label: string; w: number; h: number }> = {
    letter: { css: 'letter', label: 'Letter', w: 612, h: 792 },
    a4: { css: 'A4', label: 'A4', w: 595.3, h: 841.9 },
    half: { css: '5.5in 8.5in', label: 'Half letter', w: 396, h: 612 },
    card: { css: '6in 4in', label: '4 × 6 card', w: 432, h: 288 },
  };
  const qrLink = $derived(recipe ? (recipe.shareToken ? `${app.origin}/s/${recipe.shareToken}` : recipe.sourceUrl) : '');
  let vw = $state(window.innerWidth);
  const page = $derived.by(() => {
    const small = paper === 'card' || paper === 'half';
    const { w, h, css } = PAPER[paper];
    const mx = small ? 25.2 : 57.6;
    const my = small ? 25.2 : 54;
    return { w, h, css, mx, my, pt: small ? Math.min(size, 9) : size, px: Math.min(4 / 3, (vw - 28) / w) };
  });

  // One sheet when it can be: the recipe is set at each tightness out of sight, and the roomiest that fits wins.
  // Nothing goes below about 10 pt, and a recipe too long for one sheet is only tightened until its ingredients stay on the first.
  const FITS = [0, 1, 2, 3, 4];
  let heights = $state<number[]>([0, 0, 0, 0, 0]);
  const room = $derived((page.h - 2 * page.my) * page.px - 3);
  let probes = $state<HTMLElement[]>([]);
  // Where the ingredient list ends at each tightness, so a two-sheet recipe still keeps its whole list on the first.
  const listEnds = $derived(FITS.map((f) => {
    const el = heights[f] > 0 ? probes[f] : null;
    const ing = el?.querySelector('.doc-ing');
    return el && ing ? ing.getBoundingClientRect().bottom - el.getBoundingClientRect().top : 0;
  }));
  const fit = $derived.by(() => {
    if (!recipe || look === 'card' || paper === 'card') return 0;
    const whole = FITS.findIndex((f) => heights[f] > 0 && heights[f] <= room);
    if (whole >= 0) return whole;
    const list = FITS.findIndex((f) => listEnds[f] > 0 && listEnds[f] <= room);
    return list < 0 ? 0 : list;
  });
  const sheets = $derived(recipe && heights[fit] > 0 ? Math.ceil(heights[fit] / room) : 0);

  $effect(() => {
    const p = page;
    const u = (n: number) => `${(n * p.px).toFixed(2)}px`;
    const el = document.createElement('style');
    el.textContent =
      `@page { size: ${p.css}; margin: ${p.my}pt ${p.mx}pt; } @page plate { margin: 0; }` +
      `@media screen { .stage { font-size: ${u(p.pt)}; --page-w: ${u(p.w)}; --page-h: ${u(p.h)}; --page-mx: ${u(p.mx)}; --page-my: ${u(p.my)}; --page-ch: ${u(p.h - 2 * p.my)}; } }` +
      `@media print { .doc { --print-size: ${p.pt}pt; } .stage { --page-w: ${p.w}pt; --page-h: ${p.h}pt; --page-ch: ${p.h - 2 * p.my}pt; } }`;
    document.head.append(el);
    document.documentElement.classList.add('printing');
    return () => {
      el.remove();
      document.documentElement.classList.remove('printing');
    };
  });

  function print() {
    if (kind === 'recipe' && (look === 'classic' || look === 'modern' || look === 'card')) savePrefs({ docStyle: look as any, paper: (paper === 'a4' ? 'a4' : 'letter') as any });
    window.print();
  }
</script>

<div class="printbar noprint">
  <button class="btn quiet sm" onclick={() => history.back()}><ArrowLeft />Back</button>
  <div class="opts">
    <div class="seg" role="group" aria-label="Style">
      {#each [['classic', 'Classic'], ['modern', 'Modern'], ['card', 'Compact']] as [v, label]}
        <button class:on={look === v} onclick={() => { look = v; if (book) book.style = v; }}>{label}</button>
      {/each}
    </div>
    <select class="input mini" bind:value={paper} aria-label="Paper size">{#each Object.entries(PAPER) as [k, v]}<option value={k}>{v.label}</option>{/each}</select>
    <select class="input mini" bind:value={size} aria-label="Text size"><option value={10}>Small text</option><option value={11}>Medium text</option><option value={12.5}>Large text</option></select>
    <button class="chip" class:on={weigh} onclick={() => (weigh = !weigh)} aria-pressed={weigh}>Weights</button>
    <button class="chip" class:on={photo} onclick={() => (photo = !photo)} aria-pressed={photo}>Photo</button>
    <button class="chip" class:on={notes} onclick={() => (notes = !notes)} aria-pressed={notes}>Notes</button>
    {#if kind === 'recipe' && qrLink}<button class="chip" class:on={qr} onclick={() => (qr = !qr)} aria-pressed={qr}>QR code</button>{/if}
    {#if sheets}<span class="pages faint small num">{sheets === 1 ? '1 page' : `${sheets} pages`}</span>{/if}
  </div>
  <button class="btn primary" onclick={print}><Printer />Print</button>
</div>

<svelte:window bind:innerWidth={vw} />

<div class="stage">
  {#if recipe}
    <div class="sheet-paper"><RecipeDoc r={recipe} {look} {opts} temp={pf.temp} {photo} {notes} qr={qr ? qrLink : ''} paged {fit} /></div>
    <div class="measure noprint" aria-hidden="true" inert>
      {#each FITS as f}
        <div bind:this={probes[f]} bind:clientHeight={heights[f]}><RecipeDoc r={recipe} {look} {opts} temp={pf.temp} {photo} {notes} qr={qr ? qrLink : ''} paged measure fit={f} /></div>
      {/each}
    </div>
  {:else if book}
    <BookDoc {book} {recipes} {opts} temp={pf.temp} {photo} {notes} paged />
  {/if}
</div>

<style>
  .printbar { position: sticky; top: 0; z-index: 30; display: flex; align-items: center; gap: 12px; padding: 10px clamp(12px, 3vw, 28px); background: color-mix(in srgb, var(--paper) 90%, transparent); -webkit-backdrop-filter: blur(14px); backdrop-filter: blur(14px); border-bottom: 1px solid var(--line); }
  .opts { flex: 1; display: flex; align-items: center; gap: 8px; overflow-x: auto; scrollbar-width: none; justify-content: center; }
  .mini { width: auto; min-height: 36px; height: 36px; padding: 0 32px 0 12px; border-radius: 999px; font-size: 0.86rem; flex: none; }
  .pages { flex: none; padding: 0 4px; }
  .stage { position: relative; display: grid; gap: 28px; justify-content: center; padding: 28px 14px 80px; background: var(--sunk); min-height: 100dvh; overflow: hidden; }
  .stage > :global(.sheet-paper) { width: var(--page-w); min-height: var(--page-h); padding: var(--page-my) var(--page-mx); }
  .stage > :global(.book-cover) { padding: 0; min-height: 0; background: none; box-shadow: none; }
  .stage > :global(.book-cover .cover) { aspect-ratio: auto; height: var(--page-h); }
  /* A cookbook spread is shown as its two sheets: the photograph to the edges, then the recipe. */
  .stage > :global(.sheet-paper.bleed) { padding: 0; min-height: 0; background: none; box-shadow: none; overflow: visible; }
  .stage :global(.doc.facing .doc-plate) { height: var(--page-h); margin-bottom: 28px; box-shadow: var(--shadow-lg); border-radius: 4px; overflow: hidden; }
  .stage :global(.doc.facing .doc-plate img) { height: 100%; aspect-ratio: auto; }
  .stage :global(.doc.facing .doc-page) { box-sizing: border-box; min-height: var(--page-h); padding: var(--page-my) var(--page-mx); background: #fff; box-shadow: var(--shadow-lg); border-radius: 4px; }
  .measure { position: absolute; left: 0; top: 0; width: calc(var(--page-w) - 2 * var(--page-mx)); visibility: hidden; pointer-events: none; }
  @media (max-width: 700px) { .opts { justify-content: flex-start; } }
  @media print {
    .stage { display: block; padding: 0; background: none; min-height: 0; overflow: visible; }
    .stage :global(.doc.facing .doc-plate) { margin: 0; box-shadow: none; border-radius: 0; }
    .stage :global(.doc.facing .doc-page) { min-height: calc(var(--page-ch) - 4pt); padding: 0; box-shadow: none; }
    .stage > :global(.book-cover .cover) { height: 100vh; }
  }
</style>
