<script lang="ts">
  // Puts a meal on the plan, or changes one already there: which day, which meal, which recipe
  // (or a note like "Leftovers"), and for how many.
  import { untrack } from 'svelte';
  import Photo from './Photo.svelte';
  import Sheet from './Sheet.svelte';
  import { del, post, put } from '../lib/api';
  import { addDays, iso, MEALS, parseDay, type Meal, type PlanEntry } from '../lib/plan';
  import { app, go, oops, recipePath, toast } from '../lib/state.svelte';
  import { formatNumber } from '../lib/ingredients';
  import { ChevronLeft, ChevronRight, Dices, ExternalLink, Minus, Plus, Search, StickyNote, Trash } from '../lib/icons';
  import type { Card } from '../lib/types';

  let {
    open = $bindable(false), day = iso(new Date()), meal = 'dinner', recipeId = null, servings = 0, entry = null, onsaved,
  }: {
    open: boolean; day?: string; meal?: Meal; recipeId?: number | null; servings?: number; entry?: PlanEntry | null; onsaved?: (e: PlanEntry | null) => void;
  } = $props();

  let pickDay = $state('');
  let pickMeal = $state<Meal>('dinner');
  let pickRecipe = $state<number | null>(null);
  let note = $state('');
  let count = $state(0);
  let q = $state('');
  let writing = $state(false);
  let busy = $state(false);
  // The strip of days starts at the day asked for, or today, and can be paged.
  let stripStart = $state(new Date());

  // Each time the sheet opens it starts from what it was given.
  $effect(() => {
    if (open) untrack(reset);
  });
  function reset() {
    pickDay = entry?.day ?? day;
    pickMeal = (entry?.meal ?? meal) as Meal;
    pickRecipe = entry ? entry.recipeId : recipeId;
    note = entry?.text ?? '';
    writing = !!entry && !entry.recipeId;
    count = entry?.servings ?? servings;
    q = '';
    const d = parseDay(pickDay);
    const today = parseDay(iso(new Date()));
    // Show the chosen day, with the days around it, and today when it is near.
    stripStart = d < today ? d : addDays(d, -Math.min(3, Math.round((d.getTime() - today.getTime()) / 86400000)));
  }

  let stripEl = $state<HTMLElement | null>(null);
  // The chosen day stays in view as the strip moves.
  $effect(() => {
    pickDay;
    stripStart;
    const el = stripEl;
    if (!el) return;
    requestAnimationFrame(() => {
      const on = el.querySelector<HTMLElement>('.day.on');
      if (on) el.scrollTo({ left: Math.max(0, on.offsetLeft - el.offsetLeft - 8), behavior: 'auto' });
      else el.scrollTo({ left: 0 });
    });
  });
  const card = $derived(pickRecipe ? app.recipes.find((r) => r.id === pickRecipe) ?? null : null);
  const list = $derived.by(() => {
    const t = q.trim().toLowerCase();
    const all = t ? app.recipes.filter((r) => r.title.toLowerCase().includes(t) || r.tags.some((g) => g.toLowerCase().includes(t))) : app.recipes;
    return all.slice(0, 80);
  });
  const days = $derived(Array.from({ length: 14 }, (_, i) => addDays(stripStart, i)));
  const todayIso = iso(new Date());
  const dayName = (d: Date) => {
    const k = iso(d);
    if (k === todayIso) return 'Today';
    if (k === iso(addDays(new Date(), 1))) return 'Tomorrow';
    return d.toLocaleDateString(undefined, { weekday: 'short' });
  };

  function surprise() {
    // Something not made lately, if there is one.
    const pool = app.recipes.filter((r) => !r.lastCooked || Date.now() / 1000 - r.lastCooked > 21 * 86400);
    const from = pool.length ? pool : app.recipes;
    if (!from.length) return;
    choose(from[Math.floor(Math.random() * from.length)]);
  }
  function choose(r: Card) {
    pickRecipe = r.id;
    writing = false;
  }
  function step(d: number) {
    count = Math.max(0, Math.round((count || 0) + d));
  }

  async function save() {
    if (busy) return;
    if (!pickRecipe && !note.trim()) return;
    busy = true;
    try {
      const body = { day: pickDay, meal: pickMeal, servings: count, text: pickRecipe ? '' : note.trim(), recipeId: pickRecipe };
      let saved: PlanEntry;
      if (entry) {
        // A recipe can't be swapped on an entry; replace the entry instead.
        if ((entry.recipeId ?? null) !== (pickRecipe ?? null)) {
          await del(`/plan/${entry.id}`);
          saved = await post('/plan', body);
        } else saved = await put(`/plan/${entry.id}`, body);
      } else saved = await post('/plan', body);
      open = false;
      onsaved?.(saved);
      if (!entry) {
        const d = parseDay(saved.day);
        toast(`${card?.title ?? note.trim()} · ${MEALS.find(([k]) => k === saved.meal)?.[1]} on ${saved.day === todayIso ? 'today' : d.toLocaleDateString(undefined, { weekday: 'long', month: 'short', day: 'numeric' })}`, { action: app.path === '/plan' ? undefined : { label: 'View plan', run: () => go(`/plan?week=${saved.day}`) } });
      }
    } catch (e) {
      oops(e);
    } finally {
      busy = false;
    }
  }
  async function remove() {
    if (!entry) return;
    try {
      await del(`/plan/${entry.id}`);
      open = false;
      onsaved?.(null);
    } catch (e) {
      oops(e);
    }
  }
</script>

<Sheet bind:open title={entry ? 'Planned meal' : 'Add to the plan'}>
  <div class="stack">
    <div class="field">
      <span>Day</span>
      <div class="strip">
        <button class="icon-btn nav" onclick={() => (stripStart = addDays(stripStart, -7))} aria-label="Earlier"><ChevronLeft /></button>
        <div class="days" bind:this={stripEl}>
          {#each days as d (iso(d))}
            <button class="day" class:on={pickDay === iso(d)} class:today={iso(d) === todayIso} onclick={() => (pickDay = iso(d))} aria-pressed={pickDay === iso(d)}>
              <small>{dayName(d)}</small><b class="num">{d.getDate()}</b>
            </button>
          {/each}
        </div>
        <button class="icon-btn nav" onclick={() => (stripStart = addDays(stripStart, 7))} aria-label="Later"><ChevronRight /></button>
      </div>
    </div>

    <div class="field">
      <span>Meal</span>
      <div class="seg meals">{#each MEALS as [k, l]}<button class:on={pickMeal === k} onclick={() => (pickMeal = k)}>{l}</button>{/each}</div>
    </div>

    {#if card}
      <div class="chosen">
        <div class="thumb"><Photo image={card.image} color={card.color} title={card.title} tags={card.tags} /></div>
        <div class="grow"><b>{card.title}</b>{#if entry}<a class="open" href={recipePath(card)} onclick={() => (open = false)}>Open recipe <ExternalLink /></a>{/if}</div>
        {#if !recipeId || entry}<button class="btn sm quiet" onclick={() => (pickRecipe = null)}>Change</button>{/if}
      </div>
    {:else if writing}
      <label class="field">
        <span>Note</span>
        <!-- svelte-ignore a11y_autofocus -->
        <input class="input" bind:value={note} placeholder="Leftovers, eating out, takeaway…" maxlength="200" autofocus />
      </label>
      <button class="btn sm quiet" style:width="fit-content" onclick={() => (writing = false)}>Pick a recipe instead</button>
    {:else}
      <div class="row">
        <div class="search grow"><Search /><input class="input" bind:value={q} placeholder="Search your recipes" aria-label="Search your recipes" /></div>
        <button class="icon-btn" onclick={surprise} aria-label="Surprise me" title="Surprise me"><Dices /></button>
      </div>
      <div class="picks">
        {#each list as r (r.id)}
          <button class="pick" onclick={() => choose(r)}>
            <div class="thumb"><Photo image={r.image} color={r.color} title={r.title} tags={r.tags} /></div>
            <span class="grow title">{r.title}</span>
          </button>
        {:else}
          <p class="hint" style:padding="12px 4px">No recipes match.</p>
        {/each}
      </div>
      <button class="btn sm quiet" style:width="fit-content" onclick={() => (writing = true)}><StickyNote />Write a note instead</button>
    {/if}

    {#if card}
      <div class="opt">
        <div><b>Servings</b><span class="hint">{count ? 'The shopping list is scaled to this.' : 'As the recipe is written.'}</span></div>
        <div class="stepper">
          <button onclick={() => step(-1)} aria-label="Fewer" disabled={!count}><Minus /></button>
          <span class="num">{count ? formatNumber(count, true) : 'As written'}</span>
          <button onclick={() => step(1)} aria-label="More"><Plus /></button>
        </div>
      </div>
    {/if}

    <div class="row">
      {#if entry}<button class="icon-btn danger" onclick={remove} aria-label="Remove from the plan" title="Remove from the plan"><Trash /></button>{/if}
      <button class="btn primary grow" disabled={busy || (!pickRecipe && !note.trim())} onclick={save}>{entry ? 'Save' : 'Add to plan'}</button>
    </div>
  </div>
</Sheet>

<style>
  .strip { display: flex; align-items: center; gap: 4px; min-width: 0; }
  .nav { width: 34px; height: 34px; flex: none; }
  .days { flex: 1; min-width: 0; display: flex; gap: 6px; overflow-x: auto; scrollbar-width: none; scroll-snap-type: x proximity; padding: 2px; }
  .day { flex: none; width: 56px; display: grid; justify-items: center; gap: 1px; padding: 7px 0 6px; border-radius: 12px; background: var(--surface); box-shadow: inset 0 0 0 1px var(--line); scroll-snap-align: start; transition: background 0.15s, box-shadow 0.15s; }
  .day small { font-size: 0.7rem; font-weight: 600; color: var(--ink-3); }
  .day b { font: 600 1.15rem/1.1 var(--serif); }
  .day.today small { color: var(--tomato); }
  .day.on { background: var(--bay); box-shadow: none; color: var(--on-bay); }
  .day.on small { color: color-mix(in srgb, var(--on-bay) 75%, transparent); }
  .meals { width: 100%; display: grid; grid-template-columns: repeat(4, 1fr); }
  .chosen { display: flex; align-items: center; gap: 12px; padding: 10px; border-radius: var(--r-m); background: var(--wash); }
  .chosen b { display: block; font: 600 1.05rem/1.25 var(--serif); }
  .open { display: inline-flex; align-items: center; gap: 4px; margin-top: 2px; font-size: 0.83rem; font-weight: 600; color: var(--bay); }
  .open :global(svg) { width: 13px; height: 13px; }
  .thumb { position: relative; width: 56px; height: 42px; flex: none; border-radius: 7px; overflow: hidden; background: var(--sunk); isolation: isolate; }
  .thumb :global(img) { width: 100%; height: 100%; object-fit: cover; }
  .search { position: relative; }
  .search :global(svg) { position: absolute; left: 13px; top: 13px; width: 18px; height: 18px; color: var(--ink-3); }
  .search .input { padding-left: 40px; }
  .picks { display: grid; max-height: min(38dvh, 340px); overflow: auto; margin: 0 -4px; }
  .pick { display: flex; align-items: center; gap: 12px; padding: 7px 4px; border-bottom: 1px solid var(--line); text-align: left; }
  .pick:hover { background: var(--sunk); }
  .title { font: 600 1rem/1.25 var(--serif); }
  .opt { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
  .opt > div:first-child { display: grid; }
  .stepper { display: inline-flex; align-items: center; height: 38px; border-radius: 999px; background: var(--surface); box-shadow: inset 0 0 0 1px var(--line-2); flex: none; }
  .stepper button { width: 38px; height: 38px; display: grid; place-items: center; border-radius: 50%; color: var(--bay); }
  .stepper button:disabled { opacity: 0.35; }
  .stepper :global(svg) { width: 16px; height: 16px; }
  .stepper span { min-width: 84px; text-align: center; font-weight: 700; font-size: 0.9rem; }
  .danger { color: var(--tomato); }
</style>
