<script lang="ts">
  import { flip } from 'svelte/animate';
  import Cover from '../components/Cover.svelte';
  import Photo from '../components/Photo.svelte';
  import RecipeTile from '../components/RecipeTile.svelte';
  import ShareSheet from '../components/ShareSheet.svelte';
  import Sheet from '../components/Sheet.svelte';
  import { del, get, put } from '../lib/api';
  import { app, go, loadCookbooks, oops, toast } from '../lib/state.svelte';
  import { ArrowDown, ArrowLeft, ArrowUp, Check, Pencil, Plus, Printer, Search, Share, Trash, X } from '../lib/icons';
  import type { Card, Cookbook } from '../lib/types';

  let { id }: { id: number } = $props();
  let book = $state<Cookbook | null>(null);
  let arranging = $state(false);
  let picking = $state(false);
  let editing = $state(false);
  let sharing = $state(false);
  let q = $state('');
  let chaptersText = $state('');
  const colors = ['bay', 'tomato', 'butter', 'ocean', 'plum', 'charcoal', 'paper'];

  const load = () => get<Cookbook>(`/cookbooks/${id}`).then((b) => { b.shareToken ??= ''; book = b; }).catch(oops);
  load();

  const cards = $derived(new Map(app.recipes.map((r) => [r.id, r])));
  const sections = $derived.by(() => {
    if (!book) return [];
    const order = [...book.chapters];
    for (const e of book.entries ?? []) if (e.chapter && !order.includes(e.chapter)) order.push(e.chapter);
    const groups = [{ name: '', items: [] as Card[] }, ...order.map((name) => ({ name, items: [] as Card[] }))];
    for (const e of book.entries ?? []) {
      const c = cards.get(e.recipeId);
      if (c) (groups.find((g) => g.name === e.chapter) ?? groups[0]).items.push(c);
    }
    return groups.filter((g) => g.items.length || (arranging && g.name));
  });
  const inBook = $derived(new Set((book?.entries ?? []).map((e) => e.recipeId)));
  const pickList = $derived(app.recipes.filter((r) => !q.trim() || r.title.toLowerCase().includes(q.trim().toLowerCase())).slice(0, 200));

  async function save(patch: Partial<Cookbook> = {}) {
    if (!book) return;
    Object.assign(book, patch);
    try {
      const saved = await put<Cookbook>(`/cookbooks/${id}`, book);
      saved.shareToken ??= '';
      book = saved;
      loadCookbooks();
    } catch (e) {
      oops(e);
    }
  }
  function toggle(rid: number) {
    if (!book) return;
    book.entries = inBook.has(rid) ? book.entries!.filter((e) => e.recipeId !== rid) : [...(book.entries ?? []), { recipeId: rid, chapter: '' }];
    save();
  }
  function move(rid: number, d: number) {
    if (!book?.entries) return;
    const e = book.entries;
    const i = e.findIndex((x) => x.recipeId === rid);
    // Swap with the neighbour in the same chapter.
    let j = i + d;
    while (j >= 0 && j < e.length && e[j].chapter !== e[i].chapter) j += d;
    if (j < 0 || j >= e.length) return;
    [e[i], e[j]] = [e[j], e[i]];
    save();
  }
  function setChapter(rid: number, chapter: string) {
    const e = book?.entries?.find((x) => x.recipeId === rid);
    if (e) {
      e.chapter = chapter;
      save();
    }
  }
  function openEdit() {
    chaptersText = book!.chapters.join('\n');
    editing = true;
  }
  async function saveDetails(e: SubmitEvent) {
    e.preventDefault();
    await save({ chapters: chaptersText.split('\n').map((c) => c.trim()).filter(Boolean) });
    editing = false;
  }
  async function remove() {
    if (!book || !confirm(`Delete the cookbook “${book.title}”? Its recipes stay in your library.`)) return;
    await del(`/cookbooks/${id}`).catch(oops);
    await loadCookbooks();
    go('/books', { replace: true });
    toast('Cookbook deleted');
  }
</script>

{#if book}
  <div class="page">
    <a class="btn quiet sm" href="/books" style="margin:0 0 10px -10px"><ArrowLeft />Cookbooks</a>
    <header class="head">
      <div class="cov"><Cover title={book.title} subtitle={book.subtitle} color={book.color} seed={book.id} /></div>
      <div class="stack" style:gap="10px">
        <h1 class="page-title">{book.title}</h1>
        {#if book.subtitle}<p class="serif sub">{book.subtitle}</p>{/if}
        {#if book.description}<p class="muted" style:max-width="60ch" style:white-space="pre-line">{book.description}</p>{/if}
        <p class="faint small num">{book.entries?.length ?? 0} recipes{book.chapters.length ? `, ${book.chapters.length} chapters` : ''}</p>
        <div class="row wrap" style:margin-top="6px">
          <button class="btn primary" onclick={() => (picking = true)}><Plus />Add recipes</button>
          <button class="btn" class:primary={arranging} onclick={() => (arranging = !arranging)}>{arranging ? 'Done arranging' : 'Arrange'}</button>
          <button class="icon-btn" class:on={!!book.shareToken} onclick={() => (sharing = true)} aria-label="Share" title="Share"><Share /></button>
          <a class="icon-btn" href={`/books/${id}/print`} aria-label="Print" title="Print"><Printer /></a>
          <button class="icon-btn" onclick={openEdit} aria-label="Edit details" title="Edit details"><Pencil /></button>
          <button class="icon-btn" onclick={remove} aria-label="Delete cookbook" title="Delete"><Trash /></button>
        </div>
      </div>
    </header>

    {#if (book.entries?.length ?? 0) === 0}
      <div class="empty"><h2>An empty book</h2><p>Add recipes from your library. They stay in the library too, and can be in other cookbooks at the same time.</p></div>
    {/if}

    {#each sections as g (g.name)}
      <section class="chapter">
        {#if g.name}<h2 class="serif">{g.name}</h2>{/if}
        {#if arranging}
          <ul class="rows">
            {#each g.items as r (r.id)}
              <li animate:flip={{ duration: 220 }}>
                <div class="thumb"><Photo image={r.image} color={r.color} title={r.title} tags={r.tags} /></div>
                <span class="grow title">{r.title}</span>
                {#if book.chapters.length}
                  <select class="input ch" value={book.entries?.find((e) => e.recipeId === r.id)?.chapter ?? ''} onchange={(e) => setChapter(r.id, e.currentTarget.value)} aria-label="Chapter">
                    <option value="">No chapter</option>
                    {#each book.chapters as c}<option value={c}>{c}</option>{/each}
                  </select>
                {/if}
                <button class="icon-btn" onclick={() => move(r.id, -1)} aria-label="Move up"><ArrowUp /></button>
                <button class="icon-btn" onclick={() => move(r.id, 1)} aria-label="Move down"><ArrowDown /></button>
                <button class="icon-btn" onclick={() => toggle(r.id)} aria-label="Remove from cookbook"><X /></button>
              </li>
            {/each}
            {#if g.items.length === 0}<li class="faint small" style:padding="12px">No recipes in this chapter yet.</li>{/if}
          </ul>
        {:else}
          <div class="grid">{#each g.items as r, i (r.id)}<RecipeTile {r} {i} />{/each}</div>
        {/if}
      </section>
    {/each}
    {#if arranging && book.chapters.length === 0}<p class="hint">Want sections like “Starters” and “Mains”? Add chapters under <button class="linklike" onclick={openEdit}>Edit details</button>.</p>{/if}
  </div>

  <ShareSheet bind:open={sharing} kind="cookbook" {id} title={book.title} bind:token={book.shareToken as string} />

  <Sheet bind:open={picking} title="Add recipes">
    <div class="stack">
      <div class="search"><Search /><input class="input" bind:value={q} placeholder="Search your recipes" aria-label="Search your recipes" /></div>
      <div class="picks">
        {#each pickList as r (r.id)}
          <button class="pick" onclick={() => toggle(r.id)} aria-pressed={inBook.has(r.id)}>
            <div class="thumb"><Photo image={r.image} color={r.color} title={r.title} tags={r.tags} /></div>
            <span class="grow title">{r.title}</span>
            <span class="box" class:checked={inBook.has(r.id)}><Check /></span>
          </button>
        {/each}
      </div>
      <button class="btn primary" onclick={() => (picking = false)}>Done</button>
    </div>
  </Sheet>

  <Sheet bind:open={editing} title="Cookbook details">
    <form class="stack" onsubmit={saveDetails}>
      <label class="field"><span>Title</span><input class="input" bind:value={book.title} required /></label>
      <label class="field"><span>Subtitle</span><input class="input" bind:value={book.subtitle} /></label>
      <label class="field"><span>Introduction</span><textarea class="input" rows="3" bind:value={book.description} placeholder="Shown on the contents page"></textarea></label>
      <label class="field"><span>Chapters</span><textarea class="input" rows="4" bind:value={chaptersText} placeholder={'Starters\nMains\nDesserts'}></textarea><span class="hint">One per line, in reading order.</span></label>
      <div class="field">
        <span>Cover</span>
        <div class="row wrap">{#each colors as c}<button type="button" class={`swatch cover ${c}`} class:on={book.color === c} onclick={() => (book!.color = c)} aria-label={c}></button>{/each}</div>
      </div>
      <div class="field">
        <span>Page style when shared or printed</span>
        <div class="seg" style:width="fit-content">
          {#each [['classic', 'Classic'], ['modern', 'Modern'], ['card', 'Compact']] as [v, label]}<button type="button" class:on={book.style === v} onclick={() => (book!.style = v)}>{label}</button>{/each}
        </div>
      </div>
      <button class="btn primary">Save details</button>
    </form>
  </Sheet>
{/if}

<style>
  .head { display: grid; grid-template-columns: 170px minmax(0, 1fr); gap: clamp(20px, 4vw, 44px); align-items: center; margin-bottom: 36px; }
  .cov :global(.cover) { box-shadow: inset 0.7em 0 0.9em -0.6em rgb(0 0 0 / 0.45), 0 14px 34px rgb(20 30 25 / 0.22); }
  .sub { font-style: italic; font-size: 1.2rem; color: var(--ink-2); }
  .chapter { margin-bottom: 40px; }
  .chapter h2 { font-size: 1.6rem; font-weight: 600; font-style: italic; margin-bottom: 16px; padding-bottom: 8px; border-bottom: 1px solid var(--line); }
  .rows li, .pick { display: flex; align-items: center; gap: 12px; padding: 8px 4px; border-bottom: 1px solid var(--line); text-align: left; width: 100%; }
  .thumb { position: relative; width: 56px; height: 42px; flex: none; border-radius: 7px; overflow: hidden; background: var(--sunk); isolation: isolate; }
  .thumb :global(img) { width: 100%; height: 100%; object-fit: cover; }
  .title { font: 600 1.02rem/1.25 var(--serif); }
  .ch { width: auto; max-width: 150px; min-height: 36px; height: 36px; padding: 0 30px 0 10px; font-size: 0.85rem; }
  .picks { display: grid; max-height: 50dvh; overflow: auto; }
  .box { flex: none; width: 22px; height: 22px; border-radius: 50%; display: grid; place-items: center; box-shadow: inset 0 0 0 1.5px var(--line-2); color: transparent; transition: background 0.2s, transform 0.25s var(--spring); }
  .box :global(svg) { width: 13px; height: 13px; stroke-width: 3; }
  .box.checked { background: var(--bay); box-shadow: none; color: var(--on-bay); transform: scale(1.06); }
  .search { position: relative; }
  .search :global(svg) { position: absolute; left: 13px; top: 13px; width: 18px; height: 18px; color: var(--ink-3); }
  .search .input { padding-left: 40px; }
  .linklike { color: var(--bay); font-weight: 600; text-decoration: underline; }
  @media (max-width: 640px) { .head { grid-template-columns: 104px minmax(0, 1fr); align-items: start; } .rows .icon-btn { width: 34px; } .ch { max-width: 104px; } }
</style>
