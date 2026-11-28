<script lang="ts">
  // Print and save as PDF. What is shown is what prints: the pages themselves, laid out at the size of the
  // paper, with every page break where it will fall.
  import { tick, untrack } from 'svelte';
  import BookPage from '../components/book/BookPage.svelte';
  import { fitsIn } from '../components/book/measure';
  import Sheet from '../components/Sheet.svelte';
  import { get } from '../lib/api';
  import type { Page } from '../lib/book';
  import { cookState } from '../lib/cook.svelte';
  import { asJsonLd, asMarkdown, asText, bookPages, download, PAPERS, paperOf, recipePages, typeScale } from '../lib/printing';
  import { app, oops, prefs, savePrefs } from '../lib/state.svelte';
  import { ArrowLeft, ChevronDown, Download, FileDown, Printer, Settings } from '../lib/icons';
  import type { Cookbook, Prefs, Recipe } from '../lib/types';
  import { dict, ensureFoods } from '../lib/weigh.svelte';

  // given: what to print when it is already loaded, as on a shared link (where there is no account to fetch with).
  let { kind, id = 0, given = null, onback }: {
    kind: 'recipe' | 'cookbook'; id?: number; given?: { recipe?: Recipe; book?: Cookbook; recipes?: Recipe[] } | null; onback?: () => void;
  } = $props();
  const pf = prefs();
  const saved = { ...(pf.print ?? {}) };
  let recipe = $state<Recipe | null>(null);
  let book = $state<Cookbook | null>(null);
  let recipes = $state<Recipe[]>([]);

  // ---- options, remembered between visits
  let paper = $state(saved.paper ?? (pf.paper === 'a4' ? 'a4' : 'letter'));
  let size = $state<0 | 1 | 2>(saved.size ?? 1);
  let style = $state<'classic' | 'modern'>(saved.style ?? (pf.docStyle === 'modern' ? 'modern' : 'classic'));
  let photo = $state<'top' | 'page' | 'off'>(saved.photo ?? 'top');
  let notes = $state(saved.notes ?? true);
  let qr = $state(saved.qr ?? true);
  let source = $state(saved.source ?? true);
  let nutrition = $state(pf.nutrition && (saved.nutrition ?? false));
  // Cookbooks
  let photos = $state(saved.photos ?? true);
  let cover = $state(saved.cover ?? true);
  let endpapers = $state(saved.endpapers ?? false);
  let duplex = $state(saved.duplex ?? true);
  let folios = $state(saved.folios ?? true);
  // Print what the cook was looking at: the recipe page passes its Weigh setting along.
  const q = new URLSearchParams(app.query);
  let weigh = $state(q.has('weigh') ? q.get('weigh') === '1' : pf.weigh);
  const scale = untrack(() => (kind === 'recipe' && id ? cookState(id).scale : 1));
  const opts = $derived({ scale, system: pf.system, weigh, fractions: pf.fractions, flour: pf.flour });
  ensureFoods();

  $effect(() => {
    // A cookbook's style is the book's own; only a recipe's style is remembered for next time.
    const print: Prefs['print'] = { paper, size, style: kind === 'recipe' ? style : saved.style, photo, notes, qr, source, nutrition, photos, cover, endpapers, duplex, folios };
    untrack(() => {
      if (JSON.stringify(print) !== JSON.stringify(prefs().print)) savePrefs({ print });
    });
  });

  if (given) {
    untrack(() => {
      if (given!.recipe) recipe = given!.recipe;
      if (given!.book) {
        book = given!.book;
        recipes = given!.recipes ?? [];
        style = given!.book.style === 'modern' ? 'modern' : 'classic';
      }
    });
  } else if (kind === 'recipe') get<Recipe>(`/recipes/${id}`).then((r) => { recipe = r; document.title = `${r.title} · Fond`; }).catch(oops);
  else get<{ cookbook: Cookbook; recipes: Recipe[] }>(`/cookbooks/${id}?full=1`).then((x) => { book = x.cookbook; recipes = x.recipes; style = x.cookbook.style === 'modern' ? 'modern' : 'classic'; document.title = `${x.cookbook.title} · Fond`; }).catch(oops);

  const P = $derived(paperOf(paper));
  const fz = $derived(typeScale(P, size));
  const qrLink = $derived(recipe ? (recipe.shareToken ? `${app.origin || location.origin}/s/${recipe.shareToken}` : given ? location.href.split('?')[0] : recipe.sourceUrl) : '');
  // A recipe printed on its own is set as a page of a book of one.
  const asBook = $derived<Cookbook | null>(
    book ? { ...book, style } : recipe ? { id: 0, title: '', subtitle: '', description: '', color: 'bay', style, chapters: [], leather: 0, count: 1, covers: [] } : null,
  );
  const extras = $derived(kind === 'recipe' ? { qr: qr ? qrLink : '', source, nutrition } : { nutrition });

  // ---- pagination, out of sight at the paper's proportions
  let probe = $state<HTMLElement | null>(null);
  let pages = $state<{ page: Page; index: number }[]>([]);
  let busy = $state(true);
  let run = 0;
  async function paginate() {
    const mine = ++run;
    busy = true;
    await tick();
    await document.fonts?.ready;
    if (!probe || !asBook || mine !== run) return;
    const fits = fitsIn(probe, { book: asBook, opts, temp: pf.temp, fz, mg: P.mg, style, extras, folios: true });
    let out: { page: Page; index: number }[] = [];
    if (recipe) out = recipePages(recipe, fits, { photo, notes }).map((page, i) => ({ page, index: i + 2 }));
    else if (book) out = bookPages(asBook, recipes, fits, { photos, notes, cover, endpapers, duplex });
    if (mine !== run) return;
    pages = out;
    busy = false;
  }
  $effect(() => {
    [recipe, book, recipes, paper, size, style, photo, notes, qr, source, nutrition, photos, cover, endpapers, duplex, folios, weigh, dict.rev];
    untrack(() => paginate());
  });

  // ---- the preview is a scale model of the sheet
  let stageW = $state(800);
  const pxPerPt = $derived(Math.min(4 / 3, (stageW - 32) / P.w));
  const probeW = 600;

  $effect(() => {
    const el = document.createElement('style');
    el.textContent = `@page { size: ${P.css}; margin: 0; } @media print { .pp { width: ${P.w}pt !important; height: ${P.h}pt !important; } }`;
    document.head.append(el);
    document.documentElement.classList.add('printing');
    return () => {
      el.remove();
      document.documentElement.classList.remove('printing');
    };
  });

  let options = $state(false);
  let exportMenu = $state(false);
  function print() {
    options = false;
    window.print();
  }
  function exportAs(kind: 'md' | 'txt' | 'json') {
    exportMenu = false;
    if (!recipe) return;
    const base = recipe.slug || 'recipe';
    if (kind === 'md') download(`${base}.md`, asMarkdown(recipe), 'text/markdown');
    else if (kind === 'txt') download(`${base}.txt`, asText(recipe), 'text/plain');
    else download(`${base}.json`, asJsonLd(recipe, recipe.image ? `${app.origin}/img/${recipe.image}.jpg` : ''), 'application/ld+json');
  }
  const groups = ['Sheets', 'Book pages', 'Cards'] as const;
  const sheets = $derived(pages.length);
  // Pages printed back to back get mirrored margins, wider at the spine; a loose sheet is set evenly.
  const side = (i: number, p: Page): 'left' | 'right' | 'single' => (kind === 'cookbook' && duplex ? (i % 2 === 1 ? 'left' : 'right') : 'single');
  const showFolios = $derived(kind === 'cookbook' ? folios : pages.length > 1);
</script>

<svelte:window onclick={() => (exportMenu = false)} />

{#snippet controls()}
  <div class="group">
    <span class="label">Paper</span>
    <select class="input" bind:value={paper} aria-label="Paper size">
      {#each groups as g}<optgroup label={g}>{#each PAPERS.filter((p) => p.group === g) as p}<option value={p.key}>{p.label}</option>{/each}</optgroup>{/each}
    </select>
  </div>
  <div class="group">
    <span class="label">Style</span>
    <div class="seg full">{#each [['classic', 'Classic'], ['modern', 'Modern']] as [v, l]}<button class:on={style === v} onclick={() => (style = v as any)}>{l}</button>{/each}</div>
  </div>
  <div class="group">
    <span class="label">Text size <span class="faint">{P.sizes[size]} pt</span></span>
    <div class="seg full">{#each ['Small', 'Medium', 'Large'] as l, i}<button class:on={size === i} onclick={() => (size = i as 0 | 1 | 2)}>{l}</button>{/each}</div>
  </div>
  {#if kind === 'recipe'}
    {#if recipe?.image}
      <div class="group">
        <span class="label">Photo</span>
        <div class="seg full">{#each [['top', 'On top'], ['page', 'Own page'], ['off', 'Off']] as [v, l]}<button class:on={photo === v} onclick={() => (photo = v as any)}>{l}</button>{/each}</div>
      </div>
    {/if}
  {/if}
  <div class="group toggles">
    <span class="label">Include</span>
    <label><button class="switch" role="switch" aria-checked={weigh} onclick={() => (weigh = !weigh)} aria-label="Weights"></button>Weights for cups and spoons</label>
    <label><button class="switch" role="switch" aria-checked={notes} onclick={() => (notes = !notes)} aria-label="Notes"></button>Notes</label>
    {#if pf.nutrition}<label><button class="switch" role="switch" aria-checked={nutrition} onclick={() => (nutrition = !nutrition)} aria-label="Nutrition"></button>Nutrition</label>{/if}
    {#if kind === 'recipe'}
      <label><button class="switch" role="switch" aria-checked={source} onclick={() => (source = !source)} aria-label="Source"></button>Source and author</label>
      {#if qrLink}<label><button class="switch" role="switch" aria-checked={qr} onclick={() => (qr = !qr)} aria-label="QR code"></button>QR code to the recipe</label>{/if}
    {:else}
      <label><button class="switch" role="switch" aria-checked={photos} onclick={() => (photos = !photos)} aria-label="Photographs"></button>Photographs</label>
      <label><button class="switch" role="switch" aria-checked={cover} onclick={() => (cover = !cover)} aria-label="Covers"></button>Front and back cover</label>
      <label><button class="switch" role="switch" aria-checked={endpapers} onclick={() => (endpapers = !endpapers)} aria-label="Endpapers"></button>Endpapers</label>
      <label><button class="switch" role="switch" aria-checked={folios} onclick={() => (folios = !folios)} aria-label="Page numbers"></button>Page numbers</label>
      <label><button class="switch" role="switch" aria-checked={duplex} onclick={() => (duplex = !duplex)} aria-label="Both sides"></button>Printing on both sides</label>
      <p class="hint">{duplex ? 'Blank pages are kept so every photograph faces its recipe when the sheets are printed back to back.' : 'Blank pages are left out.'}</p>
    {/if}
  </div>
{/snippet}

<div class="printing-page">
  <header class="bar noprint">
    <button class="btn quiet sm back" onclick={() => (onback ? onback() : history.back())} aria-label="Back"><ArrowLeft /><span class="deskonly">Back</span></button>
    <div class="grow title">
      <b>{recipe?.title ?? book?.title ?? ''}</b>
      <span class="faint small num">{busy ? 'Laying out…' : `${sheets} page${sheets === 1 ? '' : 's'} · ${P.label}`}</span>
    </div>
    <button class="btn sm mob" onclick={() => (options = true)}><Settings />Options</button>
    {#if kind === 'recipe'}
      <div style:position="relative" class="deskonly">
        <button class="btn sm" onclick={(e) => { e.stopPropagation(); exportMenu = !exportMenu; }} aria-expanded={exportMenu}><Download />Export<ChevronDown /></button>
        {#if exportMenu}
          <div class="menu" style="right:0;top:42px">
            <button onclick={() => exportAs('md')}><FileDown />Markdown (.md)</button>
            <button onclick={() => exportAs('txt')}><FileDown />Plain text (.txt)</button>
            <button onclick={() => exportAs('json')}><FileDown />Recipe data (schema.org JSON)</button>
          </div>
        {/if}
      </div>
    {/if}
    <button class="btn primary" onclick={print} disabled={busy}><Printer /><span class="deskonly">Print or save as PDF</span><span class="mob">Print</span></button>
  </header>

  <div class="layout">
    <aside class="panel noprint deskonly">{@render controls()}</aside>
    <div class="stage" bind:clientWidth={stageW}>
      {#if asBook}
        {#each pages as { page, index }, i (i + ':' + index + ':' + page.kind)}
          <div class="pp" style:width={`${P.w * pxPerPt}px`} style:height={`${P.h * pxPerPt}px`}>
            <BookPage {page} {index} book={asBook} {opts} temp={pf.temp} side={side(index, page)} {fz} mg={P.mg} {style} {extras} folios={showFolios} />
          </div>
        {/each}
      {/if}
      {#if busy && !pages.length}<div class="pp ghost noprint" style:width={`${P.w * pxPerPt}px`} style:height={`${P.h * pxPerPt}px`}></div>{/if}
    </div>
  </div>
  <div class="probe" bind:this={probe} style:width={`${probeW}px`} style:height={`${(probeW * P.h) / P.w}px`} aria-hidden="true" inert></div>
</div>

<Sheet bind:open={options} title="Print options">
  <div class="stack">
    {@render controls()}
    {#if kind === 'recipe'}
      <div class="group">
        <span class="label">Export</span>
        <div class="row wrap">
          <button class="btn sm" onclick={() => exportAs('md')}><FileDown />Markdown</button>
          <button class="btn sm" onclick={() => exportAs('txt')}><FileDown />Text</button>
          <button class="btn sm" onclick={() => exportAs('json')}><FileDown />JSON</button>
        </div>
      </div>
    {/if}
    <button class="btn primary" onclick={print}><Printer />Print or save as PDF</button>
  </div>
</Sheet>

<style>
  .printing-page { min-height: 100dvh; background: var(--sunk); }
  .bar { position: sticky; top: 0; z-index: 30; display: flex; align-items: center; gap: 10px; height: 62px; padding: 0 clamp(10px, 2.5vw, 24px); background: color-mix(in srgb, var(--paper) 92%, transparent); -webkit-backdrop-filter: blur(14px); backdrop-filter: blur(14px); border-bottom: 1px solid var(--line); }
  .title { display: grid; text-align: center; min-width: 0; }
  .title b { font: 600 1.05rem/1.2 var(--serif); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .layout { display: grid; grid-template-columns: 300px minmax(0, 1fr); align-items: start; }
  .panel { position: sticky; top: 62px; height: calc(100dvh - 62px); overflow: auto; display: grid; align-content: start; gap: 20px; padding: 22px 22px 40px; background: var(--paper); border-right: 1px solid var(--line); }
  .group { display: grid; gap: 8px; }
  .label { font-size: 0.8rem; font-weight: 700; letter-spacing: 0.04em; text-transform: uppercase; color: var(--ink-3); display: flex; justify-content: space-between; }
  .label .faint { text-transform: none; letter-spacing: 0; font-weight: 500; }
  .seg.full { display: grid; grid-auto-flow: column; grid-auto-columns: 1fr; }
  .seg.full button { padding: 0 6px; }
  .toggles label { display: flex; align-items: center; gap: 12px; font-size: 0.93rem; padding: 3px 0; cursor: pointer; }
  .toggles .switch { flex: none; }
  .stage { display: grid; justify-content: center; align-content: start; gap: 24px; padding: 28px 16px 90px; min-width: 0; }
  .pp { position: relative; background: #fbf8f1; box-shadow: var(--shadow-lg); border-radius: 2px; overflow: hidden; }
  .pp :global(.bp::after) { display: none; }
  /* On paper the page is the paper: white, with the tints kept light. */
  .pp :global(.bp) { --paper: #fff; --tint: #f3efe7; }
  .ghost { background: color-mix(in srgb, #fbf8f1 60%, transparent); animation: pulse 1.2s ease-in-out infinite alternate; }
  @keyframes pulse { to { opacity: 0.5; } }
  .probe { position: absolute; left: -9999px; top: 0; visibility: hidden; pointer-events: none; }
  @media (max-width: 820px) {
    .layout { grid-template-columns: 1fr; }
    .stage { padding: 16px 10px 80px; gap: 16px; }
    .bar .btn.primary { padding: 0 14px; }
    .bar { gap: 6px; }
    .title b { font-size: 0.95rem; }
    .back { padding: 0 8px; }
  }
  @media print {
    :global(html), :global(body) { background: #fff !important; }
    .noprint, .probe, .ghost { display: none !important; }
    .printing-page { background: none; min-height: 0; }
    .layout { display: block; }
    .stage { display: block; padding: 0; }
    .pp { box-shadow: none; border-radius: 0; break-after: page; page-break-after: always; overflow: hidden; }
    .pp:last-child { break-after: auto; page-break-after: auto; }
    .pp :global(.cover) { border-radius: 0 !important; }
  }
</style>
