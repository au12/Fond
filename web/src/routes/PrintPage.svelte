<script lang="ts">
  import BookDoc from '../components/BookDoc.svelte';
  import RecipeDoc from '../components/RecipeDoc.svelte';
  import { get } from '../lib/api';
  import { cookState } from '../lib/cook.svelte';
  import { app, oops, prefs, savePrefs } from '../lib/state.svelte';
  import { ArrowLeft, Printer } from '../lib/icons';
  import type { Cookbook, Recipe } from '../lib/types';

  let { kind, id }: { kind: 'recipe' | 'cookbook'; id: number } = $props();
  const pf = prefs();
  let recipe = $state<Recipe | null>(null);
  let book = $state<Cookbook | null>(null);
  let recipes = $state<Recipe[]>([]);
  let look = $state<string>(pf.docStyle);
  let paper = $state<string>(pf.paper);
  let photo = $state(true);
  let notes = $state(true);
  let qr = $state(true);
  let size = $state(11);
  const scale = kind === 'recipe' ? cookState(id).scale : 1;
  const opts = $derived({ scale, system: pf.system, weigh: pf.weigh, fractions: pf.fractions });

  if (kind === 'recipe') get<Recipe>(`/recipes/${id}`).then((r) => (recipe = r)).catch(oops);
  else get<{ cookbook: Cookbook; recipes: Recipe[] }>(`/cookbooks/${id}?full=1`).then((x) => { book = x.cookbook; recipes = x.recipes; look = x.cookbook.style; }).catch(oops);

  const PAPER: Record<string, { css: string; label: string; w: number }> = {
    letter: { css: 'letter', label: 'Letter', w: 816 },
    a4: { css: 'A4', label: 'A4', w: 794 },
    half: { css: '5.5in 8.5in', label: 'Half letter', w: 528 },
    card: { css: '6in 4in', label: '4 × 6 card', w: 576 },
  };
  const qrLink = $derived(recipe ? (recipe.shareToken ? `${app.origin}/s/${recipe.shareToken}` : recipe.sourceUrl) : '');

  $effect(() => {
    const small = paper === 'card' || paper === 'half';
    const el = document.createElement('style');
    el.textContent = `@page { size: ${PAPER[paper].css}; margin: ${small ? '0.35in' : '0.75in 0.8in'}; } @media print { .doc { --print-size: ${small ? Math.min(size, 9) : size}pt; } }`;
    document.head.append(el);
    document.documentElement.classList.add('printing');
    return () => {
      el.remove();
      document.documentElement.classList.remove('printing');
    };
  });

  function print() {
    if (kind === 'recipe' && (look === 'classic' || look === 'modern' || look === 'card')) savePrefs({ docStyle: look as any, paper: (paper === 'a4' ? 'a4' : 'letter') as any });
    window.print();
  }
</script>

<div class="printbar noprint">
  <button class="btn quiet sm" onclick={() => history.back()}><ArrowLeft />Back</button>
  <div class="opts">
    <div class="seg" role="group" aria-label="Style">
      {#each [['classic', 'Classic'], ['modern', 'Modern'], ['card', 'Compact']] as [v, label]}
        <button class:on={look === v} onclick={() => { look = v; if (book) book.style = v; }}>{label}</button>
      {/each}
    </div>
    <select class="input mini" bind:value={paper} aria-label="Paper size">{#each Object.entries(PAPER) as [k, v]}<option value={k}>{v.label}</option>{/each}</select>
    <select class="input mini" bind:value={size} aria-label="Text size"><option value={10}>Small text</option><option value={11}>Medium text</option><option value={12.5}>Large text</option></select>
    <button class="chip" class:on={photo} onclick={() => (photo = !photo)} aria-pressed={photo}>Photo</button>
    <button class="chip" class:on={notes} onclick={() => (notes = !notes)} aria-pressed={notes}>Notes</button>
    {#if kind === 'recipe' && qrLink}<button class="chip" class:on={qr} onclick={() => (qr = !qr)} aria-pressed={qr}>QR code</button>{/if}
  </div>
  <button class="btn primary" onclick={print}><Printer />Print</button>
</div>

<div class="stage" style:--w={`${PAPER[paper].w}px`} style:font-size={`${size * 1.28}px`}>
  {#if recipe}
    <div class="sheet-paper"><RecipeDoc r={recipe} {look} {opts} temp={pf.temp} {photo} {notes} qr={qr ? qrLink : ''} /></div>
  {:else if book}
    <BookDoc {book} {recipes} {opts} temp={pf.temp} {photo} {notes} />
  {/if}
</div>

<style>
  .printbar { position: sticky; top: 0; z-index: 30; display: flex; align-items: center; gap: 12px; padding: 10px clamp(12px, 3vw, 28px); background: color-mix(in srgb, var(--paper) 90%, transparent); backdrop-filter: blur(14px); -webkit-backdrop-filter: blur(14px); border-bottom: 1px solid var(--line); }
  .opts { flex: 1; display: flex; align-items: center; gap: 8px; overflow-x: auto; scrollbar-width: none; justify-content: center; }
  .mini { width: auto; min-height: 36px; height: 36px; padding: 0 32px 0 12px; border-radius: 999px; font-size: 0.86rem; flex: none; }
  .stage { display: grid; gap: 28px; justify-content: center; padding: 28px 14px 80px; background: var(--sunk); min-height: 100dvh; }
  .stage > :global(.sheet-paper) { width: min(var(--w), calc(100vw - 28px)); }
  .stage > :global(.book-cover) { padding: 0; background: none; box-shadow: none; }
  @media (max-width: 700px) { .opts { justify-content: flex-start; } .stage { font-size: 13px !important; } }
  @media print {
    .stage { display: block; padding: 0; background: none; min-height: 0; font-size: inherit !important; }
  }
</style>
