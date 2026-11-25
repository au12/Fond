<script lang="ts">
  import RecipeTile from '../components/RecipeTile.svelte';
  import Pot from '../components/Pot.svelte';
  import { get } from '../lib/api';
  import { app, go, openAdd, recipePath } from '../lib/state.svelte';
  import { ArrowUpDown, Dices, Heart, Plus, Search, Timer, X } from '../lib/icons';

  type Sort = 'new' | 'az' | 'cooked' | 'quick';
  let q = $state('');
  let fav = $state(false);
  let quick = $state(false);
  let tag = $state('');
  let sort = $state<Sort>((localStorage.getItem('fond:sort') as Sort) || 'new');
  let hits = $state<number[] | null>(null);
  let searchEl: HTMLInputElement;
  let limit = $state(60);

  const tags = $derived.by(() => {
    const counts = new Map<string, number>();
    for (const r of app.recipes) for (const t of r.tags) counts.set(t, (counts.get(t) ?? 0) + 1);
    return [...counts].sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0])).slice(0, 14).map((x) => x[0]);
  });

  // Titles and tags match instantly; the server's full-text index then adds
  // matches from ingredients, steps and notes.
  let searchSeq = 0;
  $effect(() => {
    const term = q.trim();
    hits = null;
    if (term.length < 2) return;
    const seq = ++searchSeq;
    const t = setTimeout(async () => {
      try {
        const ids = await get<number[]>('/search?q=' + encodeURIComponent(term));
        if (seq === searchSeq) hits = ids;
      } catch {
        /* local results still show */
      }
    }, 160);
    return () => clearTimeout(t);
  });

  const shown = $derived.by(() => {
    let list = app.recipes;
    if (fav) list = list.filter((r) => r.fav);
    if (quick) list = list.filter((r) => r.total > 0 && r.total <= 30);
    if (tag) list = list.filter((r) => r.tags.includes(tag));
    const term = q.trim().toLowerCase();
    if (term) {
      const words = term.split(/\s+/);
      const local = (r: (typeof list)[number]) => words.every((w) => r.title.toLowerCase().includes(w) || r.tags.some((t) => t.toLowerCase().includes(w)));
      const rank = new Map((hits ?? []).map((id, i) => [id, i]));
      list = list.filter((r) => local(r) || rank.has(r.id));
      return [...list].sort((a, b) => Number(local(b)) - Number(local(a)) || (rank.get(a.id) ?? 1e6) - (rank.get(b.id) ?? 1e6));
    }
    const by: Record<Sort, (a: any, b: any) => number> = {
      new: (a, b) => b.created - a.created || b.id - a.id,
      az: (a, b) => a.title.localeCompare(b.title),
      cooked: (a, b) => b.lastCooked - a.lastCooked || b.created - a.created,
      quick: (a, b) => (a.total || 1e6) - (b.total || 1e6),
    };
    return [...list].sort(by[sort]);
  });

  $effect(() => {
    localStorage.setItem('fond:sort', sort);
  });
  $effect(() => {
    void [q, fav, quick, tag, sort];
    limit = 60;
  });

  function surprise() {
    const pool = shown.length ? shown : app.recipes;
    if (pool.length) go(recipePath(pool[Math.floor(Math.random() * pool.length)]));
  }

  function onKey(e: KeyboardEvent) {
    const typing = /INPUT|TEXTAREA|SELECT/.test((e.target as Element).tagName);
    if (e.key === '/' && !typing) {
      e.preventDefault();
      searchEl?.focus();
    } else if (e.key === 'n' && !typing && !e.metaKey && !e.ctrlKey) openAdd();
  }

  // Pasting a link anywhere on the library starts an import.
  function onPaste(e: ClipboardEvent) {
    if (/INPUT|TEXTAREA/.test((e.target as Element).tagName)) return;
    const t = e.clipboardData?.getData('text')?.trim() ?? '';
    if (/^https?:\/\/\S+$/i.test(t)) openAdd(t);
  }

  function more(node: HTMLElement) {
    const io = new IntersectionObserver((es) => es[0].isIntersecting && (limit += 60), { rootMargin: '600px' });
    io.observe(node);
    return { destroy: () => io.disconnect() };
  }
</script>

<svelte:window onkeydown={onKey} onpaste={onPaste} />

<div class="page">
  {#if app.recipesLoaded && app.recipes.length === 0}
    <div class="empty">
      <Pot />
      <h2>Nothing cooking yet</h2>
      <p>Save your first recipe from any website, or bring your whole collection over from Mealie.</p>
      <div class="row wrap" style:justify-content="center">
        <button class="btn primary" onclick={() => openAdd()}><Plus />Add a recipe</button>
        {#if app.user?.role === 'admin'}<a class="btn" href="/settings/import">Move from Mealie</a>{/if}
      </div>
    </div>
  {:else}
    <div class="findrow">
      <div class="find">
        <Search />
        <input bind:this={searchEl} bind:value={q} type="search" placeholder={app.recipes.length ? `Search ${app.recipes.length} recipes` : 'Search recipes'} aria-label="Search recipes" autocomplete="off" enterkeyhint="search" />
        {#if q}<button class="icon-btn" onclick={() => { q = ''; searchEl.focus(); }} aria-label="Clear search"><X /></button>{/if}
      </div>
      <button class="icon-btn tool" onclick={surprise} aria-label="Pick a random recipe" title="Pick one for me"><Dices /></button>
      <label class="sortwrap" title="Sort">
        <ArrowUpDown />
        <select class="input sort" bind:value={sort} aria-label="Sort recipes" disabled={!!q.trim()}>
          <option value="new">Newest</option>
          <option value="az">A to Z</option>
          <option value="cooked">Cooked recently</option>
          <option value="quick">Quickest</option>
        </select>
      </label>
    </div>

    <div class="chips">
      <button class="chip" class:on={fav} onclick={() => (fav = !fav)} aria-pressed={fav}><Heart />Favorites</button>
      <button class="chip" class:on={quick} onclick={() => (quick = !quick)} aria-pressed={quick}><Timer />30 minutes or less</button>
      {#each tags as t}
        <button class="chip" class:on={tag === t} onclick={() => (tag = tag === t ? '' : t)} aria-pressed={tag === t}>{t}</button>
      {/each}
    </div>

    {#if !app.recipesLoaded}
      <div class="grid">{#each Array(8) as _}<div class="tile"><div class="photo skeleton"></div></div>{/each}</div>
    {:else if shown.length === 0}
      <div class="empty">
        <h2>No recipes match</h2>
        <p>Try fewer words, or clear the filters.</p>
        <button class="btn" onclick={() => { q = ''; fav = false; quick = false; tag = ''; }}>Clear search and filters</button>
      </div>
    {:else}
      <div class="grid">
        {#each shown.slice(0, limit) as r, i (r.id)}
          <RecipeTile {r} {i} />
        {/each}
      </div>
      {#if shown.length > limit}<div use:more style:height="1px"></div>{/if}
    {/if}
  {/if}
</div>

<style>
  .findrow { display: flex; align-items: center; gap: 8px; margin-bottom: 14px; }
  .find { position: relative; flex: 1; min-width: 0; display: flex; align-items: center; }
  .find > :global(svg) { position: absolute; left: 18px; width: 20px; height: 20px; color: var(--ink-3); pointer-events: none; }
  .find input { width: 100%; height: 54px; padding: 0 50px 0 50px; border-radius: 999px; border: 1px solid var(--line); background: var(--surface); font-size: 1.05rem; outline: none; transition: border-color 0.15s, box-shadow 0.15s; }
  .find input:focus { border-color: var(--bay); box-shadow: 0 0 0 4px color-mix(in srgb, var(--bay) 14%, transparent); }
  .find input::-webkit-search-cancel-button { display: none; }
  .find .icon-btn { position: absolute; right: 6px; }
  .tool { flex: none; width: 48px; height: 48px; }
  .sortwrap { position: relative; flex: none; display: flex; align-items: center; }
  .sortwrap > :global(svg) { display: none; }
  .sort { width: auto; min-height: 48px; height: 48px; padding: 0 38px 0 18px; border-radius: 999px; font-size: 0.95rem; font-weight: 500; border-color: var(--line); }
  .chips { display: flex; gap: 8px; overflow-x: auto; scrollbar-width: none; padding: 2px; margin: -2px -2px 26px; mask-image: linear-gradient(to right, #000 calc(100% - 28px), transparent); }
  .chips::-webkit-scrollbar { display: none; }
  .skeleton { animation: pulse 1.4s ease-in-out infinite; }
  @keyframes pulse { 50% { opacity: 0.5; } }
  @media (max-width: 600px) {
    .findrow { gap: 2px; }
    .find { margin-right: 4px; }
    .find input { height: 48px; font-size: 1rem; padding-left: 46px; }
    .find > :global(svg) { left: 16px; }
    .tool { width: 42px; height: 42px; }
    .sortwrap { width: 42px; height: 42px; justify-content: center; border-radius: 50%; color: var(--ink-2); }
    .sortwrap > :global(svg) { display: block; width: 20px; height: 20px; }
    .sort { position: absolute; inset: 0; width: 100%; opacity: 0; padding: 0; }
    .chips { margin-bottom: 18px; }
  }
</style>
