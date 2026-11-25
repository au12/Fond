<script lang="ts">
  import BookDoc from '../components/BookDoc.svelte';
  import Logo from '../components/Logo.svelte';
  import RecipeDoc from '../components/RecipeDoc.svelte';
  import { get } from '../lib/api';
  import { formatNumber, type System } from '../lib/ingredients';
  import { keepAwake } from '../lib/wake';
  import { Minus, Plus, Printer } from '../lib/icons';
  import type { Cookbook, Recipe } from '../lib/types';

  let { kind, token }: { kind: 'recipe' | 'cookbook'; token: string } = $props();
  let recipe = $state<Recipe | null>(null);
  let book = $state<Cookbook | null>(null);
  let recipes = $state<Recipe[]>([]);
  let gone = $state(false);
  let scale = $state(1);
  let system = $state<System>('original');
  const opts = $derived({ scale, system, weigh: false, fractions: true });

  if (kind === 'recipe') get<Recipe>(`/public/recipe/${token}`).then((r) => { recipe = r; document.title = r.title; keepAwake(true); }).catch(() => (gone = true));
  else get<{ cookbook: Cookbook; recipes: Recipe[] }>(`/public/cookbook/${token}`).then((x) => { book = x.cookbook; recipes = x.recipes; document.title = x.cookbook.title; }).catch(() => (gone = true));

  function step(d: number) {
    if (recipe && recipe.servings > 0) scale = Math.max(1, Math.round(recipe.servings * scale + d)) / recipe.servings;
    else {
      const ladder = [0.5, 1, 1.5, 2, 3, 4];
      const i = ladder.findIndex((x) => Math.abs(x - scale) < 0.01);
      scale = ladder[Math.min(Math.max((i < 0 ? 1 : i) + d, 0), ladder.length - 1)];
    }
  }
</script>

<div class="pub">
  <header class="noprint">
    <span class="wordmark" style="font-size:1.25rem"><Logo size={22} />Fond</span>
    <div class="grow"></div>
    {#if recipe}
      <div class="stepper" role="group" aria-label="Scale recipe">
        <button onclick={() => step(-1)} aria-label="Make less"><Minus /></button>
        <span class="num">{recipe.servings > 0 ? formatNumber(recipe.servings * scale, true) + ' servings' : formatNumber(scale, true) + '×'}</span>
        <button onclick={() => step(1)} aria-label="Make more"><Plus /></button>
      </div>
    {/if}
    <div class="seg" role="group" aria-label="Units">
      <button class:on={system === 'original'} onclick={() => (system = 'original')}>As written</button>
      <button class:on={system === 'metric'} onclick={() => (system = 'metric')}>Metric</button>
      <button class:on={system === 'us'} onclick={() => (system = 'us')}>US</button>
    </div>
    <button class="icon-btn" onclick={() => window.print()} aria-label="Print"><Printer /></button>
  </header>

  <main>
    {#if gone}
      <div class="empty"><h2>This link is no longer shared</h2><p>The owner may have turned sharing off.</p></div>
    {:else if recipe}
      <div class="sheet-paper"><RecipeDoc r={recipe} {opts} /></div>
    {:else if book}
      <BookDoc {book} {recipes} {opts} />
    {/if}
  </main>
</div>

<style>
  .pub { min-height: 100dvh; background: var(--sunk); }
  header { position: sticky; top: 0; z-index: 20; display: flex; align-items: center; flex-wrap: wrap; gap: 10px; padding: 10px clamp(12px, 4vw, 32px); background: color-mix(in srgb, var(--paper) 90%, transparent); backdrop-filter: blur(14px); -webkit-backdrop-filter: blur(14px); border-bottom: 1px solid var(--line); }
  main { display: grid; gap: 26px; justify-content: center; padding: clamp(14px, 4vw, 40px) 12px 80px; font-size: clamp(15px, 1.5vw, 17px); }
  main > :global(.sheet-paper) { width: min(860px, calc(100vw - 24px)); }
  main > :global(.book-cover) { width: min(460px, calc(100vw - 24px)); justify-self: center; padding: 0; background: none; box-shadow: none; }
  .stepper { display: inline-flex; align-items: center; height: 38px; border-radius: 999px; background: var(--surface); box-shadow: inset 0 0 0 1px var(--line-2); }
  .stepper button { width: 38px; height: 38px; display: grid; place-items: center; border-radius: 50%; color: var(--bay); }
  .stepper :global(svg) { width: 16px; height: 16px; }
  .stepper span { min-width: 92px; text-align: center; font-weight: 700; font-size: 0.92rem; }
  @media (max-width: 560px) { .seg button { padding: 0 9px; font-size: 0.8rem; } header .grow { display: none; } .seg { margin-left: auto; } }
  @media print { .pub { background: none; } main { display: block; padding: 0; font-size: inherit; } main > :global(.book-cover) { width: auto; } }
</style>
