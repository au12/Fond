<script lang="ts">
  import { flip } from 'svelte/animate';
  import { fade, fly, slide } from 'svelte/transition';
  import { get, post, put } from '../lib/api';
  import { oops, prefs, toast } from '../lib/state.svelte';
  import { AISLES, combine, type ShopItem, type ShopLine } from '../lib/shopping';
  import { dict, ensureFoods } from '../lib/weigh.svelte';
  import { Check, ChevronDown, ListChecks, Merge, Plus, ShoppingBasket, Split, Trash, X } from '../lib/icons';

  let items = $state<ShopItem[]>([]);
  let loaded = $state(false);
  let draft = $state('');
  let open = $state<Record<string, boolean>>({});
  let selecting = $state(false);
  let picked = $state<Set<string>>(new Set());
  const pf = prefs();
  ensureFoods();

  get<ShopItem[]>('/shopping').then((x) => { items = x; loaded = true; }).catch(oops);

  const lines = $derived.by(() => {
    dict.rev;
    return combine(items, { system: pf.system, fractions: pf.fractions, weigh: pf.weigh, flour: pf.flour });
  });
  const todo = $derived(lines.filter((l) => !l.checked));
  const done = $derived(lines.filter((l) => l.checked));
  const aisles = $derived(AISLES.map((name) => ({ name, lines: todo.filter((l) => l.aisle === name) })).filter((a) => a.lines.length));
  const pickedLines = $derived(lines.filter((l) => picked.has(l.key)));

  async function add(e: SubmitEvent) {
    e.preventDefault();
    const texts = draft.split(/[\n,]/).map((s) => s.trim()).filter(Boolean);
    if (!texts.length) return;
    draft = '';
    try {
      items = await post('/shopping', { items: texts.map((text) => ({ text })) });
    } catch (err) {
      oops(err);
    }
  }
  async function bulk(ids: number[], body: Record<string, unknown>) {
    try {
      items = await post('/shopping/bulk', { ids, ...body });
    } catch (err) {
      oops(err);
    }
  }
  function toggle(l: ShopLine) {
    const to = !l.checked;
    for (const it of l.items) it.checked = to;
    navigator.vibrate?.(8);
    bulk(l.items.map((i) => i.id), { checked: to });
  }
  /** Deletes lines, with a moment to take it back. */
  function remove(gone: ShopItem[], what: string) {
    const ids = new Set(gone.map((i) => i.id));
    items = items.filter((i) => !ids.has(i.id));
    bulk([...ids], { delete: true });
    toast(what, {
      action: {
        label: 'Undo',
        run: async () => {
          try {
            const back = await post<ShopItem[]>('/shopping', { items: gone.map(({ text, recipeId, recipeTitle, group }) => ({ text, recipeId, recipeTitle, group })) });
            // Ticks come back too.
            const ticked = back.filter((b) => gone.some((g) => g.checked && g.text === b.text && !b.checked)).map((b) => b.id);
            items = ticked.length ? await post('/shopping/bulk', { ids: ticked, checked: true }) : back;
          } catch (err) {
            oops(err);
          }
        },
      },
    });
  }
  async function group(ids: number[], g: string) {
    try {
      items = await post('/shopping/group', { ids, group: g });
    } catch (err) {
      oops(err);
    }
  }
  function mergePicked() {
    const ids = pickedLines.flatMap((l) => l.items.map((i) => i.id));
    group(ids, 'm:' + Math.random().toString(36).slice(2, 10));
    stopSelecting();
    toast('Merged into one line');
  }
  function deletePicked() {
    const gone = pickedLines.flatMap((l) => l.items);
    remove(gone, `${gone.length} line${gone.length === 1 ? '' : 's'} deleted`);
    stopSelecting();
  }
  /** Takes a merged line apart: each line stands on its own and Fond no longer joins it to others. */
  function separate(l: ShopLine) {
    Promise.all(l.items.map((it) => put(`/shopping/${it.id}`, { group: 'solo:' + it.id }))).then(() => get<ShopItem[]>('/shopping')).then((x) => (items = x)).catch(oops);
    open[l.key] = false;
  }
  function pick(l: ShopLine) {
    const next = new Set(picked);
    if (next.has(l.key)) next.delete(l.key);
    else next.add(l.key);
    picked = next;
  }
  function stopSelecting() {
    selecting = false;
    picked = new Set();
  }
  async function clear(all: boolean) {
    if (all && !confirm('Clear the whole shopping list?')) return;
    items = await post('/shopping/clear', { all });
    toast(all ? 'Shopping list cleared' : 'Ticked items removed');
  }
  const count = (n: number, one: string) => `${n} ${one}${n === 1 ? '' : 's'}`;
</script>

{#snippet row(l: ShopLine)}
  {@const merged = l.items.length > 1}
  <div class="line" class:done={l.checked} class:picked={picked.has(l.key)}>
    {#if selecting}
      <button class="item" onclick={() => pick(l)} aria-pressed={picked.has(l.key)}>
        <span class="box sq" class:checked={picked.has(l.key)}><Check /></span>
        <span class="grow txt">{#if l.amount}<b>{l.approx ? '≈ ' : ''}{l.amount}</b>{' '}{/if}{l.name}</span>
      </button>
    {:else}
      <button class="item" onclick={() => toggle(l)} aria-pressed={l.checked}>
        <span class="box" class:checked={l.checked}><Check /></span>
        <span class="grow txt">
          {#if l.amount}<b>{l.approx ? '≈ ' : ''}{l.amount}</b>{' '}{/if}{l.name}
          {#if l.from.length || merged}<small>{merged ? `${l.items.length} lines${l.manual ? ', merged by hand' : ''}` : ''}{merged && l.from.length ? ' · ' : ''}{l.from.join(', ')}</small>{/if}
        </span>
      </button>
      {#if merged}
        <button class="icon-btn more" class:open={open[l.key]} onclick={() => (open[l.key] = !open[l.key])} aria-expanded={!!open[l.key]} aria-label="Show the lines"><ChevronDown /></button>
      {/if}
      <button class="icon-btn rm" onclick={() => remove(l.items, merged ? `${l.items.length} lines deleted` : `“${l.text}” deleted`)} aria-label="Delete"><X /></button>
    {/if}
  </div>
  {#if merged && open[l.key] && !selecting}
    <ul class="parts" transition:slide={{ duration: 180 }}>
      {#each l.items as it (it.id)}
        <li>
          <span class="grow">{it.text}{#if it.recipeTitle}<small>{it.recipeTitle}</small>{/if}</span>
          <button class="icon-btn rm" onclick={() => remove([it], `“${it.text}” deleted`)} aria-label={`Delete ${it.text}`}><X /></button>
        </li>
      {/each}
      <li class="sep"><button class="btn sm quiet" onclick={() => separate(l)}><Split />Keep these separate</button></li>
    </ul>
  {/if}
{/snippet}

<div class="page narrow shop">
  <div class="spread head">
    <div>
      <h1 class="page-title">Shopping list</h1>
      {#if lines.length}<p class="faint small num">{count(todo.length, 'thing')} to get{done.length ? `, ${done.length} in the basket` : ''}</p>{/if}
    </div>
    {#if lines.length}
      {#if selecting}
        <button class="btn sm" onclick={stopSelecting}>Done</button>
      {:else}
        <button class="btn sm" onclick={() => (selecting = true)}><ListChecks />Select</button>
      {/if}
    {/if}
  </div>
  {#if !selecting}
    <form class="add" onsubmit={add}>
      <input class="input" bind:value={draft} placeholder="Add an item, or several separated by commas" aria-label="Add an item" enterkeyhint="done" />
      <button class="btn primary" disabled={!draft.trim()} aria-label="Add"><Plus /></button>
    </form>
  {:else}
    <p class="hint pickhint" in:fade={{ duration: 150 }}>Pick lines to merge into one, or to delete.</p>
  {/if}

  {#if loaded && items.length === 0}
    <div class="empty">
      <span style="color:var(--bay)"><ShoppingBasket size={56} strokeWidth={1.25} /></span>
      <h2>Nothing to buy</h2>
      <p>Add items here, open a recipe and choose “Add to shopping list”, or add a week of meals from the plan. The same ingredient from different recipes is added up into one line.</p>
    </div>
  {/if}

  {#each aisles as a (a.name)}
    <section>
      <h2>{a.name}</h2>
      <ul>
        {#each a.lines as l (l.key)}
          <li animate:flip={{ duration: 220 }} out:fade={{ duration: 120 }}>{@render row(l)}</li>
        {/each}
      </ul>
    </section>
  {/each}

  {#if done.length}
    <section class="got">
      <div class="spread"><h2>In the basket</h2>{#if !selecting}<button class="btn quiet sm" onclick={() => clear(false)}>Remove ticked</button>{/if}</div>
      <ul>
        {#each done as l (l.key)}
          <li animate:flip={{ duration: 220 }}>{@render row(l)}</li>
        {/each}
      </ul>
    </section>
  {/if}
  {#if lines.length && !selecting}
    <div class="foot"><button class="btn sm quiet danger" onclick={() => clear(true)}><Trash />Clear the whole list</button></div>
  {/if}
</div>

{#if selecting}
  <div class="seldock" transition:fly={{ y: 30, duration: 200 }}><div class="selbar">
    <span class="num">{picked.size} picked</span>
    <div class="grow"></div>
    <button class="btn sm" disabled={pickedLines.length < 2} onclick={mergePicked}><Merge />Merge</button>
    <button class="btn sm danger" disabled={!pickedLines.length} onclick={deletePicked}><Trash />Delete</button>
    <button class="icon-btn" onclick={stopSelecting} aria-label="Stop selecting"><X /></button>
  </div></div>
{/if}

<style>
  .shop { padding-bottom: 120px; }
  .head { margin-bottom: 18px; align-items: end; }
  .head p { margin-top: 4px; }
  .add { display: flex; gap: 8px; margin-bottom: 26px; }
  .add .input { height: 50px; border-radius: 999px; padding: 0 20px; }
  .add .btn { width: 50px; height: 50px; padding: 0; flex: none; }
  .pickhint { margin: 0 0 22px; padding: 14px 16px; border-radius: var(--r-m); background: var(--butter-soft); color: var(--ink-2); }
  section { margin-bottom: 24px; }
  h2 { font: italic 600 1.15rem var(--serif); color: var(--ink-2); margin-bottom: 4px; }
  li { border-bottom: 1px solid var(--line); }
  .line { display: flex; align-items: center; border-radius: 10px; transition: background 0.15s; }
  .line.picked { background: var(--wash); }
  .item { flex: 1; min-width: 0; display: flex; align-items: center; gap: 14px; padding: 13px 4px; text-align: left; font-size: 1.05rem; line-height: 1.3; }
  .txt b { font-weight: 650; }
  .txt small { display: block; font-size: 0.78rem; color: var(--ink-3); margin-top: 2px; }
  .box { flex: none; width: 24px; height: 24px; border-radius: 50%; display: grid; place-items: center; box-shadow: inset 0 0 0 1.5px var(--line-2); color: transparent; transition: background 0.2s, transform 0.25s var(--spring); }
  .box.sq { border-radius: 7px; }
  .box :global(svg) { width: 14px; height: 14px; stroke-width: 3; }
  .box.checked { background: var(--bay); box-shadow: none; color: var(--on-bay); transform: scale(1.04); }
  .line.done .txt { color: var(--ink-3); text-decoration: line-through; }
  .rm { opacity: 0.45; flex: none; }
  .rm:hover { opacity: 1; color: var(--tomato); }
  .more { flex: none; color: var(--ink-3); }
  .more :global(svg) { transition: transform 0.25s var(--ease); }
  .more.open :global(svg) { transform: rotate(180deg); }
  .parts { margin: -2px 0 10px 38px; padding: 4px 0 2px 14px; border-left: 2px solid var(--line); }
  .parts li { display: flex; align-items: center; gap: 8px; border: 0; padding: 2px 0; font-size: 0.94rem; color: var(--ink-2); }
  .parts small { display: block; font-size: 0.75rem; color: var(--ink-3); }
  .parts .icon-btn { width: 34px; height: 34px; }
  .parts .sep { padding-top: 4px; }
  .got { margin-top: 34px; }
  .foot { display: flex; justify-content: center; margin-top: 30px; }
  .head h1 { white-space: nowrap; }
  .seldock { position: fixed; z-index: 45; left: 0; right: 0; bottom: calc(var(--nav-h) + var(--safe-b) + 14px); display: flex; justify-content: center; pointer-events: none; }
  .selbar {
    pointer-events: auto; width: min(560px, calc(100vw - 24px)); display: flex; align-items: center; gap: 8px; padding: 8px 8px 8px 18px;
    border-radius: 999px; background: var(--surface); box-shadow: var(--shadow-lg), inset 0 0 0 1px var(--line); font-weight: 600;
  }
</style>
