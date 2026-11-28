<script lang="ts">
  // A cookbook to read like a book: the leather cover opens, pages turn, a photograph faces each recipe.
  // Two pages at a time on a wide screen, one at a time on a phone.
  import { tick, untrack } from 'svelte';
  import { fitsIn } from './measure';
  import BookPage from './BookPage.svelte';
  import { layout, partsOf, sectionsOf, singles, spreads, folio, type Page, type Part } from '../../lib/book';
  import type { System } from '../../lib/ingredients';
  import { recipePath } from '../../lib/state.svelte';
  import { dict, ensureFoods } from '../../lib/weigh.svelte';
  import type { Cookbook, Recipe } from '../../lib/types';
  import { ArrowLeft, BookOpen, ChevronLeft, ChevronRight, ExternalLink, ListTree, Printer, Scale, X } from '../../lib/icons';

  let {
    book, recipes, system: startSystem = 'original', weigh: startWeigh = false, fractions = true, flour = 'spoon', temp = 'original', canOpen = false, onclose, onprint,
  }: {
    book: Cookbook; recipes: Recipe[]; system?: System; weigh?: boolean; fractions?: boolean; flour?: 'spoon' | 'dip'; temp?: 'original' | 'C' | 'F';
    canOpen?: boolean; onclose?: () => void; onprint?: () => void;
  } = $props();

  let system = $state<System>(untrack(() => startSystem));
  let weigh = $state(untrack(() => startWeigh));
  const opts = $derived({ scale: 1, system, weigh, fractions, flour });
  ensureFoods();

  // ---------------------------------------------------------------- pages
  let pages = $state<Page[]>([]);
  let measuring = $state(true);
  let probe = $state<HTMLElement | null>(null);
  const sections = $derived(sectionsOf(book, recipes));

  async function paginate() {
    await tick();
    if (!probe) return;
    await document.fonts?.ready;
    const fits = fitsIn(probe, { book, opts, temp });
    const parts = new Map<number, Part[]>();
    for (const g of sections) for (const it of g.items) parts.set(it.r.id, partsOf(it.r, g.name, it.n, fits));
    const keep = pages.length ? pageAt() : -1;
    pages = layout(book, sections, parts);
    measuring = false;
    if (keep >= 0) showPage(Math.min(keep, pages.length - 1), false);
    else {
      // The first time, back to where the reader left off.
      let saved = 0;
      try {
        saved = parseInt(localStorage.getItem(KEY) ?? '0', 10) || 0;
      } catch {
        /* private mode */
      }
      if (saved > 0 && saved < pages.length - 1) at = viewOf(saved);
    }
  }
  // Lay the book out again when what the pages say changes: units, weights, the dictionary arriving.
  $effect(() => {
    opts;
    dict.rev;
    sections;
    untrack(() => paginate());
  });

  // ---------------------------------------------------------------- spreads and the reader's place
  let vw = $state(innerWidth);
  let vh = $state(innerHeight);
  const wide = $derived(vw >= 900 && vw > vh * 1.15);
  const views = $derived(wide ? spreads(pages) : singles(pages).map((i) => [i, null] as [number | null, number | null]));
  let at = $state(0);
  const view = $derived(views[Math.min(at, views.length - 1)] ?? [null, null]);
  const KEY = `fond:book:${untrack(() => book.id)}`;

  function pageAt(): number {
    const v = views[at];
    return v ? (v[1] ?? v[0] ?? 0) : 0;
  }
  function viewOf(page: number): number {
    const i = views.findIndex(([l, r]) => l === page || r === page);
    if (i >= 0) return i;
    // A blank page on a phone: the next page that is shown.
    const j = views.findIndex(([l]) => (l ?? -1) > page);
    return j >= 0 ? j : views.length - 1;
  }
  function showPage(page: number, animate = true) {
    go(viewOf(page), animate);
  }
  // Switching between one page and two keeps the same page in front of the reader.
  let lastWide = untrack(() => wide);
  $effect(() => {
    const w = wide;
    if (w === lastWide) return;
    const page = untrack(() => {
      const v = (w ? singles(pages).map((i) => [i, null]) : spreads(pages))[at] as [number | null, number | null] | undefined;
      return v ? (v[1] ?? v[0] ?? 0) : 0;
    });
    lastWide = w;
    untrack(() => (at = viewOf(page)));
  });
  $effect(() => {
    if (measuring) return;
    try {
      localStorage.setItem(KEY, String(pageAt()));
    } catch {
      /* private mode */
    }
  });
  // ---------------------------------------------------------------- turning
  type Turn = { dir: 1 | -1; from: number; to: number };
  let turn = $state<Turn | null>(null);
  let slideDir = $state<1 | -1>(1);
  const TURN_MS = 720;

  function go(to: number, animate = true) {
    to = Math.max(0, Math.min(views.length - 1, to));
    if (to === at) return;
    if (turn) {
      // A turn already under way finishes at once; the next starts from where it would land.
      at = turn.to;
      turn = null;
    }
    const reduce = matchMedia('(prefers-reduced-motion: reduce)').matches;
    if (!wide || !animate || reduce || Math.abs(to - at) > 1) {
      slideDir = to > at ? 1 : -1;
      at = to;
      return;
    }
    turn = { dir: to > at ? 1 : -1, from: at, to };
    setTimeout(() => {
      if (turn && turn.to === to) {
        at = to;
        turn = null;
      }
    }, TURN_MS);
  }
  const next = () => go((turn?.to ?? at) + 1);
  const prev = () => go((turn?.to ?? at) - 1);

  // During a turn: what lies under the turning leaf, and what is on either side of it.
  const under = $derived.by((): [number | null, number | null] => {
    if (!turn) return view;
    const a = views[turn.from], b = views[turn.to];
    return turn.dir > 0 ? [a[0], b[1]] : [b[0], a[1]];
  });
  const leaf = $derived.by(() => {
    if (!turn) return null;
    const a = views[turn.from], b = views[turn.to];
    return turn.dir > 0 ? { front: a[1], back: b[0] } : { front: a[0], back: b[1] };
  });
  // A closed book sits in the middle of the table, not off to one side.
  const shift = $derived.by(() => {
    if (!wide) return 0;
    const v = turn ? views[turn.to] : view;
    if (v[0] == null && v[1] != null) return -25;
    if (v[1] == null && v[0] != null) return 25;
    return 0;
  });

  // ---------------------------------------------------------------- size
  const chromeH = 128;
  const pw = $derived(wide ? Math.min((vw - 96) / 2, (vh - chromeH) / 1.5, 620) : Math.min(vw - 20, (vh - chromeH + 30) / 1.5, 560));
  const ph = $derived(pw * 1.5);

  // ---------------------------------------------------------------- what is open
  const shown = $derived([view[0], view[1]].filter((i): i is number => i != null).map((i) => pages[i]));
  const openRecipe = $derived.by(() => {
    for (const p of [...shown].reverse()) if (p && (p.kind === 'recipe' || p.kind === 'photo')) return p.r;
    return null;
  });
  const tocPage = $derived(pages.findIndex((p) => p.kind === 'toc'));
  const label = $derived.by(() => {
    const nums = [view[0], view[1]].filter((i): i is number => i != null && folio(i) > 0 && pages[i]?.kind !== 'endpaper' && pages[i]?.kind !== 'back').map(folio);
    if (!nums.length) return view[1] === 0 ? 'Cover' : pages[view[0] ?? 0]?.kind === 'back' ? 'Back cover' : '';
    return nums.length === 2 ? `Pages ${nums[0]}–${nums[1]}` : `Page ${nums[0]}`;
  });

  // ---------------------------------------------------------------- touch, keys and clicks
  let dragX = $state(0);
  let dragging = false;
  let startX = 0, startY = 0, moved = false;
  function down(e: PointerEvent) {
    if (e.pointerType === 'mouse') return;
    dragging = true;
    moved = false;
    startX = e.clientX;
    startY = e.clientY;
  }
  function move(e: PointerEvent) {
    if (!dragging) return;
    const dx = e.clientX - startX, dy = e.clientY - startY;
    if (!moved && Math.abs(dy) > Math.abs(dx)) {
      dragging = false;
      return;
    }
    if (Math.abs(dx) > 6) moved = true;
    if (!wide) dragX = dx;
  }
  function up(e: PointerEvent) {
    if (!dragging) return;
    dragging = false;
    const dx = e.clientX - startX;
    dragX = 0;
    if (Math.abs(dx) > 50) dx < 0 ? next() : prev();
  }
  function pageClick(e: MouseEvent, side: 'left' | 'right') {
    if (moved) {
      moved = false;
      return;
    }
    if ((e.target as Element).closest('button, a')) return;
    if (wide) side === 'right' ? next() : prev();
    else {
      // On a phone, the right two thirds turn forward and the left third back.
      const box = (e.currentTarget as HTMLElement).getBoundingClientRect();
      e.clientX - box.left > box.width / 3 ? next() : prev();
    }
  }
  function key(e: KeyboardEvent) {
    if ((e.target as Element)?.closest?.('input, select, textarea')) return;
    if (e.key === 'ArrowRight' || e.key === 'PageDown' || e.key === ' ') { e.preventDefault(); next(); }
    else if (e.key === 'ArrowLeft' || e.key === 'PageUp') { e.preventDefault(); prev(); }
    else if (e.key === 'Home') go(0, false);
    else if (e.key === 'End') go(views.length - 1, false);
    else if (e.key === 'Escape') onclose?.();
  }
  let menu = $state(false);
  // Pages remaining on each side, for the edges of the paper block.
  const leftEdge = $derived(Math.min(5, Math.ceil((at / Math.max(1, views.length - 1)) * 5)));
  const rightEdge = $derived(Math.min(5, Math.ceil(((views.length - 1 - at) / Math.max(1, views.length - 1)) * 5)));
</script>

<svelte:window bind:innerWidth={vw} bind:innerHeight={vh} onkeydown={key} onclick={() => (menu = false)} />

<div class="reader" role="dialog" aria-modal="true" aria-label={book.title} style:--pw={`${pw}px`} style:--ph={`${ph}px`}>
  <header class="chrome">
    {#if onclose}<button class="icon-btn light" onclick={() => onclose?.()} aria-label="Close the book"><ArrowLeft /></button>{:else}<span style="width:42px"></span>{/if}
    <div class="what">
      <b>{book.title}</b>
      <span class="num">{label}</span>
    </div>
    <div class="tools">
      {#if canOpen && openRecipe}
        <a class="btn sm openin high" href={recipePath(openRecipe)}><ExternalLink />Open in Fond</a>
      {/if}
      {#if tocPage > 0}<button class="icon-btn light" onclick={() => showPage(tocPage)} aria-label="Contents" title="Contents"><ListTree /></button>{/if}
      <div style:position="relative">
        <button class="icon-btn light" class:on={weigh || system !== 'original'} onclick={(e) => { e.stopPropagation(); menu = !menu; }} aria-label="Units" aria-expanded={menu} title="Units"><Scale /></button>
        {#if menu}
          <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
          <div class="menu units" onclick={(e) => e.stopPropagation()}>
            <div class="seg">
              {#each [['original', 'As written'], ['metric', 'Metric'], ['us', 'US']] as [v, l]}<button class:on={system === v} onclick={() => (system = v as System)}>{l}</button>{/each}
            </div>
            <label class="row small"><button class="switch" role="switch" aria-checked={weigh} aria-label="Weigh" onclick={() => (weigh = !weigh)}></button>Show cups and spoons as weights</label>
          </div>
        {/if}
      </div>
      {#if onprint}<button class="icon-btn light" onclick={() => onprint?.()} aria-label="Print" title="Print"><Printer /></button>{/if}
    </div>
  </header>

  <div class="table" onpointerdown={down} onpointermove={move} onpointerup={up} onpointercancel={up}>
    {#if measuring || !pages.length}
      <div class="loading"><BookOpen /><span>Opening the book…</span></div>
    {:else if wide}
      <button class="turn prevb" onclick={prev} disabled={at === 0 && !turn} aria-label="Previous page"><ChevronLeft /></button>
      <div class="book" style:transform={`translateX(${shift}%)`} class:closed={shift !== 0}>
        <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
        <div class="half left" class:empty={under[0] == null} style:--edge={leftEdge} onclick={(e) => pageClick(e, 'left')}>
          {#if under[0] != null}{#key under[0]}<BookPage page={pages[under[0]]} index={under[0]} {book} {opts} {temp} side="left" onjump={showPage} />{/key}{/if}
        </div>
        <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
        <div class="half right" class:empty={under[1] == null} style:--edge={rightEdge} onclick={(e) => pageClick(e, 'right')}>
          {#if under[1] != null}{#key under[1]}<BookPage page={pages[under[1]]} index={under[1]} {book} {opts} {temp} side="right" onjump={showPage} />{/key}{/if}
        </div>
        {#if turn && leaf}
          <div class="leaf" class:fwd={turn.dir > 0} class:back={turn.dir < 0} style:--ms={`${TURN_MS}ms`}>
            <div class="face front">{#if leaf.front != null}<BookPage page={pages[leaf.front]} index={leaf.front} {book} {opts} {temp} side={turn.dir > 0 ? 'right' : 'left'} />{/if}<i class="shade"></i></div>
            <div class="face rear">{#if leaf.back != null}<BookPage page={pages[leaf.back]} index={leaf.back} {book} {opts} {temp} side={turn.dir > 0 ? 'left' : 'right'} />{/if}<i class="shade"></i></div>
          </div>
        {/if}
      </div>
      <button class="turn nextb" onclick={next} disabled={at === views.length - 1 && !turn} aria-label="Next page"><ChevronRight /></button>
    {:else}
      {@const idx = view[0]}
      <div class="single" style:transform={dragX ? `translateX(${dragX}px) rotate(${dragX / 60}deg)` : null} class:dragging={!!dragX}>
        {#key idx}
          <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
          <div class="sheet-page" class:closed={pages[idx ?? 0]?.kind === 'cover' || pages[idx ?? 0]?.kind === 'back'} style:--from={`${slideDir * 40}%`} onclick={(e) => pageClick(e, 'right')}>
            {#if idx != null}<BookPage page={pages[idx]} index={idx} {book} {opts} {temp} side={pages[idx].kind === 'photo' ? 'left' : 'right'} onjump={showPage} />{/if}
          </div>
        {/key}
      </div>
    {/if}
  </div>

  {#if !measuring && views.length > 1}
    <footer class="scrub">
      {#if canOpen && openRecipe}<a class="btn sm openin low" href={recipePath(openRecipe)}><ExternalLink />Open in Fond</a>{/if}
      <input type="range" min="0" max={views.length - 1} value={at} oninput={(e) => go(+e.currentTarget.value, false)} aria-label="Page" />
    </footer>
  {/if}
  <!-- Pages are measured here, out of sight, at a fixed size. -->
  <div class="probe" bind:this={probe} aria-hidden="true" inert></div>
</div>

<style>
  .reader {
    position: fixed; inset: 0; z-index: 70; display: flex; flex-direction: column; color: #efe8dc; overflow: hidden;
    background: radial-gradient(120% 90% at 50% 40%, #3a2f26 0%, #241d18 55%, #16120f 100%);
    touch-action: pan-y; user-select: none; -webkit-user-select: none;
  }
  .reader::before { content: ''; position: absolute; inset: 0; pointer-events: none; opacity: 0.35; background: repeating-linear-gradient(92deg, rgb(255 255 255 / 0.015) 0 2px, transparent 2px 7px), repeating-linear-gradient(88deg, rgb(0 0 0 / 0.05) 0 1px, transparent 1px 11px); }
  .chrome { position: relative; z-index: 4; display: flex; align-items: center; gap: 10px; height: 60px; padding: 0 clamp(8px, 2vw, 22px); }
  .what { flex: 1; min-width: 0; display: grid; text-align: center; }
  .what b { font: italic 600 1.08rem/1.2 var(--serif); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .what span { font-size: 0.76rem; color: #b9ad9b; min-height: 1.1em; }
  .tools { display: flex; align-items: center; gap: 2px; }
  .icon-btn.light { color: #d9cfbf; }
  .icon-btn.light:hover, .icon-btn.light.on { background: rgb(255 255 255 / 0.1); color: #fff; }
  .openin { background: rgb(255 255 255 / 0.12); color: #fff; box-shadow: none; margin-right: 4px; }
  .openin:hover { background: rgb(255 255 255 / 0.2); box-shadow: none; }
  .units { right: 0; top: 48px; display: grid; gap: 12px; padding: 14px; color: var(--ink); min-width: 270px; }
  .units .row { gap: 10px; padding: 0 2px; }

  .table { position: relative; flex: 1; display: flex; align-items: center; justify-content: center; gap: 18px; min-height: 0; }
  .loading { display: grid; justify-items: center; gap: 10px; color: #b9ad9b; animation: rise 0.4s var(--ease) both; }
  .loading :global(svg) { width: 34px; height: 34px; opacity: 0.8; }

  .book { position: relative; display: flex; width: calc(var(--pw) * 2); height: var(--ph); perspective: calc(var(--pw) * 8); transition: transform 0.72s cubic-bezier(0.45, 0.05, 0.25, 1); }
  .half { position: relative; width: var(--pw); height: var(--ph); }
  .half:not(.empty) { box-shadow: 0 2px 4px rgb(0 0 0 / 0.25), 0 26px 60px rgb(0 0 0 / 0.45); }
  /* The edges of the pages still to come, and of those already read. */
  .half.right:not(.empty) { box-shadow: calc(var(--edge) * 1px) 0 0 -0.5px #e9e1d1, calc(var(--edge) * 2px) calc(var(--edge) * 0.5px) 0 -1px #d8cebb, 0 2px 4px rgb(0 0 0 / 0.25), 0 26px 60px rgb(0 0 0 / 0.45); border-radius: 0 3px 3px 0; }
  .half.left:not(.empty) { box-shadow: calc(var(--edge) * -1px) 0 0 -0.5px #e9e1d1, calc(var(--edge) * -2px) calc(var(--edge) * 0.5px) 0 -1px #d8cebb, 0 2px 4px rgb(0 0 0 / 0.25), 0 26px 60px rgb(0 0 0 / 0.45); border-radius: 3px 0 0 3px; }
  .half :global(.bp) { border-radius: inherit; }
  .half { cursor: pointer; }

  .leaf { position: absolute; top: 0; width: var(--pw); height: var(--ph); transform-style: preserve-3d; z-index: 5; pointer-events: none; }
  .leaf.fwd { left: var(--pw); transform-origin: left center; animation: fwd var(--ms) cubic-bezier(0.45, 0.05, 0.3, 1) forwards; }
  .leaf.back { left: 0; transform-origin: right center; animation: bwd var(--ms) cubic-bezier(0.45, 0.05, 0.3, 1) forwards; }
  .face { position: absolute; inset: 0; backface-visibility: hidden; -webkit-backface-visibility: hidden; overflow: hidden; }
  .face.rear { transform: rotateY(180deg); }
  .shade { position: absolute; inset: 0; pointer-events: none; z-index: 6; }
  .fwd .front .shade { background: linear-gradient(to left, rgb(0 0 0 / 0.0), rgb(0 0 0 / 0.25)); animation: shadeIn var(--ms) ease-in forwards; }
  .fwd .rear .shade { background: linear-gradient(to left, rgb(0 0 0 / 0.3), transparent 60%); animation: shadeOut var(--ms) ease-out forwards; }
  .back .front .shade { background: linear-gradient(to right, rgb(0 0 0 / 0.0), rgb(0 0 0 / 0.25)); animation: shadeIn var(--ms) ease-in forwards; }
  .back .rear .shade { background: linear-gradient(to right, rgb(0 0 0 / 0.3), transparent 60%); animation: shadeOut var(--ms) ease-out forwards; }
  @keyframes fwd { from { transform: rotateY(0deg); } 50% { transform: rotateY(-90deg) translateZ(2px); } to { transform: rotateY(-180deg); } }
  @keyframes bwd { from { transform: rotateY(0deg); } 50% { transform: rotateY(90deg) translateZ(2px); } to { transform: rotateY(180deg); } }
  @keyframes shadeIn { from { opacity: 0; } to { opacity: 1; } }
  @keyframes shadeOut { from { opacity: 1; } to { opacity: 0; } }

  .turn { width: 46px; height: 46px; flex: none; border-radius: 50%; display: grid; place-items: center; color: #d9cfbf; background: rgb(255 255 255 / 0.06); transition: background 0.15s, opacity 0.2s; z-index: 2; }
  .turn:hover { background: rgb(255 255 255 / 0.14); color: #fff; }
  .turn:disabled { opacity: 0; pointer-events: none; }
  .turn :global(svg) { width: 22px; height: 22px; }

  .single { position: relative; width: var(--pw); height: var(--ph); transition: transform 0.25s var(--ease); }
  .single.dragging { transition: none; }
  .sheet-page { position: absolute; inset: 0; box-shadow: 0 2px 4px rgb(0 0 0 / 0.25), 0 20px 50px rgb(0 0 0 / 0.5); border-radius: 2px 5px 5px 2px; overflow: hidden; animation: pageIn 0.42s cubic-bezier(0.2, 0.8, 0.2, 1) both; }
  .sheet-page.closed { border-radius: 4px 10px 10px 4px; }
  @keyframes pageIn { from { opacity: 0; transform: translateX(var(--from)) rotateY(calc(var(--from) * -0.25deg)); } to { opacity: 1; transform: none; } }

  .scrub { position: relative; z-index: 3; display: flex; flex-direction: column; align-items: center; gap: 10px; padding: 10px 20px calc(14px + env(safe-area-inset-bottom, 0px)); }
  .openin.low { display: none; }
  @media (max-width: 700px) { .openin.high { display: none; } .openin.low { display: inline-flex; margin: 0; } }
  .scrub input { width: min(520px, 100%); accent-color: #d8c592; }
  .probe { position: absolute; left: -9999px; top: 0; width: 480px; height: 720px; visibility: hidden; pointer-events: none; }
  @media (max-width: 899px) {
    .chrome { height: 54px; }
    .openin { padding: 0 11px; }
    .scrub { padding-top: 6px; }
  }
  @media (prefers-reduced-motion: reduce) { .sheet-page { animation: none; } }
</style>
