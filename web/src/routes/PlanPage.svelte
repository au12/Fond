<script lang="ts">
  import { flip } from 'svelte/animate';
  import { fade, scale } from 'svelte/transition';
  import Photo from '../components/Photo.svelte';
  import PlanSheet from '../components/PlanSheet.svelte';
  import Sheet from '../components/Sheet.svelte';
  import { put } from '../lib/api';
  import { formatNumber } from '../lib/ingredients';
  import { addDays, iso, loadPlan, mealLabel, MEALS, parseDay, shopFor, weekStart, type Meal, type PlanEntry } from '../lib/plan';
  import { app, go, oops, prefs, recipePath, toast } from '../lib/state.svelte';
  import { CalendarDays, Check, ChevronLeft, ChevronRight, ListPlus, Plus, ShoppingBasket, StickyNote } from '../lib/icons';

  const pf = prefs();
  const asked = new URLSearchParams(app.query).get('week');
  let start = $state(weekStart(asked && /^\d{4}-\d{2}-\d{2}$/.test(asked) ? parseDay(asked) : new Date()));
  let entries = $state<PlanEntry[]>([]);
  let loaded = $state(false);
  // The add/edit sheet.
  let sheet = $state(false);
  let sheetDay = $state(iso(new Date()));
  let sheetMeal = $state<Meal>('dinner');
  let editing = $state<PlanEntry | null>(null);
  // Adding the week to the shopping list.
  let shopping = $state(false);
  let shopPick = $state<Record<number, boolean>>({});
  let busy = $state(false);
  // Dragging on a desktop.
  let dragging = $state<number | null>(null);
  let over = $state('');

  const days = $derived(Array.from({ length: 7 }, (_, i) => addDays(start, i)));
  const from = $derived(iso(days[0]));
  const to = $derived(iso(days[6]));
  const todayIso = iso(new Date());
  const thisWeek = $derived(iso(weekStart(new Date())) === from);
  const cards = $derived(new Map(app.recipes.map((r) => [r.id, r])));
  const ORDER: Record<string, number> = { breakfast: 0, lunch: 1, dinner: 2, snack: 3 };
  const byDay = $derived.by(() => {
    const m = new Map<string, PlanEntry[]>();
    for (const d of days) m.set(iso(d), []);
    for (const e of entries) m.get(e.day)?.push(e);
    for (const list of m.values()) list.sort((a, b) => ORDER[a.meal] - ORDER[b.meal] || a.position - b.position || a.id - b.id);
    return m;
  });
  const planned = $derived(entries.filter((e) => e.recipeId && cards.has(e.recipeId)));
  const range = $derived.by(() => {
    const a = days[0], b = days[6];
    const sameMonth = a.getMonth() === b.getMonth();
    const left = a.toLocaleDateString(undefined, { month: 'long', day: 'numeric' });
    const right = b.toLocaleDateString(undefined, sameMonth ? { day: 'numeric' } : { month: 'long', day: 'numeric' });
    return `${left} – ${right}${b.getFullYear() !== new Date().getFullYear() ? `, ${b.getFullYear()}` : ''}`;
  });

  async function load() {
    const f = from, t = to;
    try {
      const got = await loadPlan(f, t);
      if (f === from) entries = got;
    } catch (e) {
      oops(e);
    } finally {
      loaded = true;
    }
  }
  $effect(() => {
    from;
    load();
  });
  function week(d: number) {
    start = d === 0 ? weekStart(new Date()) : addDays(start, 7 * d);
    history.replaceState(history.state, '', d === 0 ? '/plan' : `/plan?week=${iso(start)}`);
  }
  function add(day: string, meal: Meal = 'dinner') {
    editing = null;
    sheetDay = day;
    sheetMeal = meal;
    sheet = true;
  }
  function edit(e: PlanEntry) {
    editing = e;
    sheet = true;
  }
  function saved(e: PlanEntry | null) {
    if (editing) entries = entries.filter((x) => x.id !== editing!.id);
    if (e && e.day >= from && e.day <= to) entries = [...entries.filter((x) => x.id !== e.id), e];
  }

  // ---- drag a meal to another day
  function drop(day: string) {
    const id = dragging;
    dragging = null;
    over = '';
    const e = entries.find((x) => x.id === id);
    if (!e || e.day === day) return;
    const was = e.day;
    e.day = day;
    e.position = 1000 + Date.now() % 1000;
    put(`/plan/${e.id}`, { day, position: e.position }).catch((err) => {
      e.day = was;
      oops(err);
    });
  }

  function openShop() {
    shopPick = Object.fromEntries(planned.map((e) => [e.id, e.day >= todayIso]));
    if (!Object.values(shopPick).some(Boolean)) shopPick = Object.fromEntries(planned.map((e) => [e.id, true]));
    shopping = true;
  }
  async function shop() {
    busy = true;
    try {
      const n = await shopFor(planned.filter((e) => shopPick[e.id]), pf.flour);
      shopping = false;
      toast(`${n} ingredient${n === 1 ? '' : 's'} added to your shopping list`, { action: { label: 'View', run: () => go('/list') } });
    } catch (e) {
      oops(e);
    } finally {
      busy = false;
    }
  }
  const short = (d: Date) => d.toLocaleDateString(undefined, { weekday: 'short' });
  const long = (d: Date) => d.toLocaleDateString(undefined, { weekday: 'long' });
</script>

{#snippet entry(e: PlanEntry)}
  {@const c = e.recipeId ? cards.get(e.recipeId) : null}
  <button
    class="meal"
    class:note={!c}
    class:lifted={dragging === e.id}
    draggable="true"
    ondragstart={(ev) => { dragging = e.id; ev.dataTransfer?.setData('text/plain', String(e.id)); if (ev.dataTransfer) ev.dataTransfer.effectAllowed = 'move'; }}
    ondragend={() => { dragging = null; over = ''; }}
    onclick={() => edit(e)}
  >
    {#if c}
      <div class="ph"><Photo image={c.image} color={c.color} title={c.title} tags={c.tags} /></div>
      <div class="words">
        <span class="what">{mealLabel(e.meal)}{e.servings ? ` · ${formatNumber(e.servings, true)} servings` : ''}</span>
        <b>{c.title}</b>
      </div>
    {:else}
      <div class="words">
        <span class="what"><StickyNote />{mealLabel(e.meal)}</span>
        <b>{e.text || 'A recipe that was deleted'}</b>
      </div>
    {/if}
  </button>
{/snippet}

<div class="page plan">
  <header class="top">
    <div>
      <h1 class="page-title">Meal plan</h1>
      <div class="weekbar">
        <button class="icon-btn" onclick={() => week(-1)} aria-label="Previous week"><ChevronLeft /></button>
        <span class="range serif">{range}</span>
        <button class="icon-btn" onclick={() => week(1)} aria-label="Next week"><ChevronRight /></button>
        {#if !thisWeek}<button class="btn sm quiet" onclick={() => week(0)} transition:fade={{ duration: 120 }}><CalendarDays />This week</button>{/if}
      </div>
    </div>
    <button class="btn" class:primary={planned.length > 0} disabled={!planned.length} onclick={openShop}><ListPlus />Shop for this week</button>
  </header>

  <div class="week" class:empty-week={loaded && !entries.length}>
    {#each days as d (iso(d))}
      {@const k = iso(d)}
      {@const list = byDay.get(k) ?? []}
      <section
        class="day"
        class:today={k === todayIso}
        class:past={k < todayIso}
        class:over={over === k}
        aria-label={d.toLocaleDateString(undefined, { weekday: 'long', month: 'long', day: 'numeric' })}
        ondragover={(ev) => { if (dragging) { ev.preventDefault(); over = k; } }}
        ondragleave={(ev) => { if (!(ev.currentTarget as HTMLElement).contains(ev.relatedTarget as Node)) over = ''; }}
        ondrop={(ev) => { ev.preventDefault(); drop(k); }}
      >
        <header>
          <span class="dname">{short(d)}</span>
          <b class="dnum num">{d.getDate()}</b>
          {#if k === todayIso}<span class="todaytag">Today</span>{/if}
        </header>
        <div class="meals">
          {#each list as e (e.id)}
            <div animate:flip={{ duration: 240 }} in:scale={{ start: 0.94, duration: 200 }}>{@render entry(e)}</div>
          {/each}
        </div>
        <button class="add" onclick={() => add(k, list.length ? 'dinner' : 'dinner')} aria-label={`Add a meal on ${long(d)}`}><Plus /><span>Add</span></button>
      </section>
    {/each}
  </div>
  {#if loaded && !entries.length}
    <p class="hint center" in:fade>Nothing planned this week yet. Add a recipe to a day, or a note like “Leftovers”. {app.recipes.length ? 'On a recipe, the calendar button puts it straight on the plan.' : ''}</p>
  {/if}
</div>

<PlanSheet bind:open={sheet} day={sheetDay} meal={sheetMeal} entry={editing} onsaved={saved} />

<Sheet bind:open={shopping} title="Shop for this week">
  <div class="stack">
    <p class="muted">The ingredients of the meals you pick go on the shopping list, scaled to the servings planned. The same ingredient from different meals is added up.</p>
    <div class="picklist">
      {#each planned as e (e.id)}
        {@const c = cards.get(e.recipeId!)!}
        <button class="pickrow" onclick={() => (shopPick[e.id] = !shopPick[e.id])} aria-pressed={!!shopPick[e.id]}>
          <span class="box" class:checked={shopPick[e.id]}><Check /></span>
          <span class="grow"><b>{c.title}</b><small>{parseDay(e.day).toLocaleDateString(undefined, { weekday: 'long' })} · {mealLabel(e.meal)}{e.servings ? ` · ${formatNumber(e.servings, true)} servings` : ''}</small></span>
        </button>
      {/each}
    </div>
    <button class="btn primary" disabled={busy || !Object.values(shopPick).some(Boolean)} onclick={shop}><ShoppingBasket />Add {Object.values(shopPick).filter(Boolean).length} to the shopping list</button>
  </div>
</Sheet>

<style>
  .plan { width: min(1400px, 100%); }
  .top { display: flex; align-items: end; justify-content: space-between; gap: 16px; flex-wrap: wrap; margin-bottom: 22px; }
  .weekbar { display: flex; align-items: center; gap: 2px; margin: 6px 0 0 -10px; }
  .range { font-size: 1.15rem; font-weight: 600; color: var(--ink-2); min-width: 0; padding: 0 4px; }
  .week { display: grid; grid-template-columns: repeat(7, minmax(0, 1fr)); gap: 12px; align-items: stretch; }
  .day { display: flex; flex-direction: column; gap: 10px; min-height: 260px; padding: 12px; border-radius: var(--r-l); background: var(--surface); box-shadow: inset 0 0 0 1px var(--line); transition: box-shadow 0.15s, background 0.15s; }
  .day.today { box-shadow: inset 0 0 0 2px var(--bay); }
  .day.past { background: color-mix(in srgb, var(--surface) 55%, var(--sunk)); }
  .day.over { background: var(--wash); box-shadow: inset 0 0 0 2px var(--bay); }
  .day header { display: flex; align-items: baseline; gap: 8px; }
  .dname { font-size: 0.78rem; font-weight: 700; letter-spacing: 0.08em; text-transform: uppercase; color: var(--ink-3); }
  .dnum { font: 600 1.5rem/1 var(--serif); }
  .today .dname { color: var(--bay); }
  .todaytag { margin-left: auto; padding: 2px 8px; border-radius: 999px; background: var(--bay); color: var(--on-bay); font-size: 0.7rem; font-weight: 700; }
  .meals { display: grid; gap: 8px; }
  .meal { display: block; width: 100%; text-align: left; border-radius: 12px; overflow: hidden; background: var(--paper); box-shadow: inset 0 0 0 1px var(--line); transition: transform 0.2s var(--ease), box-shadow 0.2s, opacity 0.2s; cursor: grab; }
  .meal:hover { transform: translateY(-2px); box-shadow: inset 0 0 0 1px var(--line-2), var(--shadow); }
  .meal.lifted { opacity: 0.4; }
  .meal.note { background: var(--butter-soft); box-shadow: none; }
  .ph { position: relative; aspect-ratio: 16 / 10; overflow: hidden; background: var(--sunk); isolation: isolate; }
  .ph :global(img) { width: 100%; height: 100%; object-fit: cover; }
  .words { display: grid; gap: 2px; padding: 8px 10px 10px; }
  .what { display: inline-flex; align-items: center; gap: 4px; font-size: 0.68rem; font-weight: 700; letter-spacing: 0.06em; text-transform: uppercase; color: var(--ink-3); }
  .what :global(svg) { width: 12px; height: 12px; }
  .words b { font: 600 0.98rem/1.22 var(--serif); text-wrap: balance; }
  .add { margin-top: auto; display: flex; align-items: center; justify-content: center; gap: 6px; height: 38px; border-radius: 10px; color: var(--ink-3); font-size: 0.85rem; font-weight: 600; box-shadow: inset 0 0 0 1.5px var(--line); border: 0; transition: color 0.15s, box-shadow 0.15s, background 0.15s; }
  .add :global(svg) { width: 16px; height: 16px; }
  .add:hover { color: var(--bay); background: var(--wash); box-shadow: inset 0 0 0 1.5px var(--bay); }
  .center { text-align: center; margin-top: 22px; }
  .picklist { display: grid; max-height: 48dvh; overflow: auto; }
  .pickrow { display: flex; align-items: center; gap: 12px; padding: 10px 2px; border-bottom: 1px solid var(--line); text-align: left; }
  .pickrow b { display: block; font: 600 1rem/1.25 var(--serif); }
  .pickrow small { display: block; font-size: 0.8rem; color: var(--ink-3); }
  .box { flex: none; width: 22px; height: 22px; border-radius: 50%; display: grid; place-items: center; box-shadow: inset 0 0 0 1.5px var(--line-2); color: transparent; transition: background 0.2s; }
  .box :global(svg) { width: 13px; height: 13px; stroke-width: 3; }
  .box.checked { background: var(--bay); box-shadow: none; color: var(--on-bay); }

  /* A narrow screen reads the week as a list of days, one under the other. */
  @media (max-width: 1100px) and (min-width: 821px) {
    .week { grid-template-columns: repeat(4, minmax(0, 1fr)); }
  }
  @media (max-width: 820px) {
    .top .btn { width: 100%; }
    .week { grid-template-columns: 1fr; gap: 10px; }
    .day { min-height: 0; display: grid; grid-template-columns: 64px minmax(0, 1fr); grid-template-areas: 'h m' 'h a'; gap: 8px 12px; padding: 12px 12px 12px 14px; }
    .day header { grid-area: h; display: grid; align-content: start; justify-items: start; gap: 2px; }
    .dname { font-size: 0.7rem; }
    .dnum { font-size: 1.7rem; }
    .todaytag { margin: 4px 0 0; }
    .meals { grid-area: m; }
    .meals:empty { display: none; }
    .add { grid-area: a; margin: 0; height: 36px; justify-content: flex-start; padding: 0 12px; box-shadow: none; background: var(--sunk); }
    .meal { display: flex; align-items: center; gap: 0; }
    .ph { width: 74px; flex: none; aspect-ratio: 4 / 3; }
    .words { padding: 8px 12px; }
    .day.past:not(:has(.meal)) { opacity: 0.6; }
  }
</style>
