<script lang="ts">
  import Cover from './Cover.svelte';
  import RecipeDoc from './RecipeDoc.svelte';
  import type { DisplayOptions } from '../lib/ingredients';
  import type { Cookbook, Recipe } from '../lib/types';

  let { book, recipes, opts, temp = 'original', photo = true, notes = true }: { book: Cookbook; recipes: Recipe[]; opts: DisplayOptions; temp?: 'original' | 'C' | 'F'; photo?: boolean; notes?: boolean } = $props();

  // Recipes in reading order, grouped by chapter, numbered through the book.
  const sections = $derived.by(() => {
    const byId = new Map(recipes.map((r) => [r.id, r]));
    const order = [...book.chapters];
    for (const e of book.entries ?? []) if (e.chapter && !order.includes(e.chapter)) order.push(e.chapter);
    const groups = [{ name: '', items: [] as { r: Recipe; n: number }[] }, ...order.map((name) => ({ name, items: [] as { r: Recipe; n: number }[] }))];
    for (const e of book.entries ?? []) {
      const r = byId.get(e.recipeId);
      if (r) (groups.find((g) => g.name === e.chapter) ?? groups[0]).items.push({ r, n: 0 });
    }
    let n = 0;
    const out = groups.filter((g) => g.items.length);
    for (const g of out) for (const it of g.items) it.n = ++n;
    return out;
  });
</script>

<div class="sheet-paper book-cover"><Cover title={book.title} subtitle={book.subtitle} color={book.color} /></div>

<div class="sheet-paper page-break">
  <div class="toc">
    <h1 style="font:600 2.2em/1.1 var(--serif);text-align:center">Contents</h1>
    {#if book.description}<p style="margin-top:0.8em;text-align:center;font-style:italic;color:#5a655e;white-space:pre-line">{book.description}</p>{/if}
    {#each sections as g}
      {#if g.name}<h2>{g.name}</h2>{:else}<div style="height:1.2em"></div>{/if}
      {#each g.items as it}
        <a href={`#recipe-${it.n}`} onclick={(e) => { e.preventDefault(); e.stopPropagation(); document.getElementById(`recipe-${it.n}`)?.scrollIntoView({ behavior: 'smooth' }); }}><span>{it.r.title}</span><span>{it.n}</span></a>
      {/each}
    {/each}
  </div>
</div>

{#each sections as g}
  {#each g.items as it (it.r.id)}
    <div class="sheet-paper page-break"><RecipeDoc r={it.r} look={book.style} {opts} {temp} {photo} {notes} number={it.n} chapter={g.name} /></div>
  {/each}
{/each}
