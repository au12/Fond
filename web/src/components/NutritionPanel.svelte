<script lang="ts">
  // Nutrition for a recipe, estimated from its ingredients. Shown per serving when the recipe says how
  // many it serves, with the energy split into protein, fat and carbohydrate, and an honest footnote.
  import { slide } from 'svelte/transition';
  import { NUTRIENTS, fmt, nutritionOf } from '../lib/nutrition';
  import type { WeighOptions } from '../lib/ingredients';
  import type { Recipe } from '../lib/types';
  import { ChevronDown } from '../lib/icons';

  let { r, o, scale = 1 }: { r: Recipe; o: WeighOptions; scale?: number } = $props();

  const n = $derived(nutritionOf(r, o));
  let whole = $state(false);
  let why = $state(false);
  const perServing = $derived(!!n.each && !whole);
  const v = $derived(perServing ? n.each! : n.total.map((x) => x * scale));
  const val = (key: string) => v[NUTRIENTS.findIndex((x) => x.key === key)];
  // Where the energy comes from: 4 kcal a gram for protein and carbohydrate, 9 for fat.
  const split = $derived.by(() => {
    const p = val('protein') * 4, f = val('fat') * 9, c = val('carbs') * 4;
    const t = p + f + c;
    return t > 0 ? [{ k: 'Protein', pct: p / t, c: 'p' }, { k: 'Fat', pct: f / t, c: 'f' }, { k: 'Carbs', pct: c / t, c: 'c' }] : [];
  });
  const shown = $derived(n.counted > 0 && n.coverage >= 0.5);
  const theirs = $derived(Object.entries(r.nutrition ?? {}).filter(([, x]) => x));
  const pct = (x: number) => `${Math.round(x * 100)}%`;
</script>

{#if shown || theirs.length}
  <section class="nutrition" aria-label="Nutrition">
    <div class="top">
      <h3>Nutrition</h3>
      {#if shown && n.each}
        <div class="seg sm" role="group" aria-label="Show nutrition for">
          <button class:on={!whole} onclick={() => (whole = false)}>Per serving</button>
          <button class:on={whole} onclick={() => (whole = true)}>{scale === 1 ? 'Whole recipe' : 'All you’re making'}</button>
        </div>
      {/if}
    </div>
    {#if shown}
      <div class="body">
        <div class="kcal">
          <b class="num">{n.approx || n.coverage < 0.97 ? '≈ ' : ''}{fmt(val('kcal'), 'kcal')}</b>
          <span>calories{perServing ? ` a serving, of ${n.servings % 1 ? n.servings.toFixed(1) : n.servings}` : !n.each ? ' in the recipe' : ''}</span>
          {#if split.length}
            <div class="bar" aria-hidden="true">{#each split as s}<i class={s.c} style:flex-grow={s.pct}></i>{/each}</div>
            <div class="legend">{#each split as s}<span><i class={s.c}></i>{s.k} {pct(s.pct)}</span>{/each}</div>
          {/if}
        </div>
        <dl class="grid">
          {#each NUTRIENTS.slice(1) as x}
            <div class:sub={x.key === 'satfat' || x.key === 'sugar' || x.key === 'fiber'}><dt>{x.label}</dt><dd class="num">{fmt(val(x.key), x.key)}<small>{x.unit}</small></dd></div>
          {/each}
        </dl>
      </div>
      <button class="why" onclick={() => (why = !why)} aria-expanded={why}>
        Estimated from USDA FoodData Central for {n.counted} of {n.counted + n.missed.length} measured ingredients{n.coverage < 0.97 ? `, about ${pct(n.coverage)} of the recipe by weight` : ''}.
        <ChevronDown />
      </button>
      {#if why}
        <div class="detail" transition:slide={{ duration: 180 }}>
          {#if n.missed.length}<p><b>Not counted:</b> {n.missed.join(' · ')}</p>{/if}
          {#if n.unmeasured}<p>{n.unmeasured} ingredient{n.unmeasured === 1 ? ' has' : 's have'} no amount (“salt to taste”) and {n.unmeasured === 1 ? 'is' : 'are'} left out, as is fat for frying and flour for dusting.</p>{/if}
          <p>Figures are for raw ingredients as written. Cooking, draining and what stays in the pan all change what you eat, so treat this as a guide.</p>
          {#if !n.servings}<p>Add the number of servings to the recipe to see figures per serving.</p>{/if}
        </div>
      {/if}
    {/if}
    {#if theirs.length}
      <p class="theirs"><b>{shown ? 'The recipe’s own figures' : 'Per serving, from the recipe'}:</b> {theirs.map(([k, x]) => `${k.replace(/([A-Z])/g, ' $1').toLowerCase()} ${x}`).join(' · ')}</p>
    {/if}
  </section>
{/if}

<style>
  .nutrition { margin-top: 30px; padding: 20px 22px; border-radius: var(--r-l); background: var(--sunk); display: grid; gap: 14px; }
  .top { display: flex; align-items: center; justify-content: space-between; gap: 12px; flex-wrap: wrap; }
  h3 { font: 600 1.2rem/1.2 var(--serif); }
  .seg.sm button { height: 28px; padding: 0 11px; font-size: 0.8rem; }
  .body { display: grid; grid-template-columns: minmax(150px, 0.8fr) minmax(0, 1.4fr); gap: 22px; align-items: start; }
  .kcal { display: grid; gap: 2px; }
  .kcal b { font: 600 2.6rem/1 var(--serif); letter-spacing: -0.02em; color: var(--ink); }
  .kcal > span { font-size: 0.85rem; color: var(--ink-2); }
  .bar { display: flex; gap: 2px; height: 8px; margin-top: 12px; border-radius: 999px; overflow: hidden; }
  .bar i, .legend i { display: block; background: var(--c); }
  .bar i { flex-basis: 0; min-width: 3px; }
  .legend { display: flex; flex-wrap: wrap; gap: 4px 12px; margin-top: 7px; font-size: 0.76rem; color: var(--ink-2); }
  .legend span { display: inline-flex; align-items: center; gap: 5px; }
  .legend i { width: 8px; height: 8px; border-radius: 50%; }
  .p { --c: var(--bay); } .f { --c: var(--butter); } .c { --c: color-mix(in srgb, var(--tomato) 70%, var(--butter)); }
  .grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 0 22px; }
  .grid div { display: flex; justify-content: space-between; align-items: baseline; gap: 10px; padding: 6px 0; border-bottom: 1px solid var(--line); }
  .grid dt { font-size: 0.88rem; color: var(--ink-2); }
  .grid .sub dt { padding-left: 12px; font-size: 0.82rem; color: var(--ink-3); }
  .grid dd { font-weight: 650; font-size: 0.95rem; }
  .grid small { margin-left: 2px; font-weight: 500; color: var(--ink-3); font-size: 0.78rem; }
  .why { display: flex; align-items: flex-start; gap: 6px; text-align: left; font-size: 0.8rem; color: var(--ink-3); line-height: 1.45; }
  .why :global(svg) { width: 15px; height: 15px; flex: none; margin-top: 1px; transition: transform 0.2s var(--ease); }
  .why[aria-expanded='true'] :global(svg) { transform: rotate(180deg); }
  .detail { display: grid; gap: 6px; font-size: 0.83rem; color: var(--ink-2); line-height: 1.5; }
  .theirs { font-size: 0.83rem; color: var(--ink-2); line-height: 1.5; }
  @media (max-width: 560px) { .body { grid-template-columns: 1fr; gap: 16px; } }
</style>
