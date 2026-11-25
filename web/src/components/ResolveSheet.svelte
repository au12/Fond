<script lang="ts">
  // "How should Fond weigh this?" One ingredient line: what Fond makes of it,
  // and the ways to put it right, for the whole household or for one recipe.
  import Sheet from './Sheet.svelte';
  import { post } from '../lib/api';
  import { allFoods, searchFoods, type Food, type Rule } from '../lib/foods';
  import { amountOf, lookup, parseIngredient, show, weigh, type Fix, type WeighOptions } from '../lib/ingredients';
  import { app, oops, toast } from '../lib/state.svelte';
  import { dict, removeRule, saveRule } from '../lib/weigh.svelte';
  import { Ban, Check, Loader, Scale, Search, Undo, Wand } from '../lib/icons';

  let {
    open = $bindable(false), line, options = {}, canScope = false, onfix,
  }: {
    open: boolean;
    /** The ingredient line as written. */
    line: string;
    options?: WeighOptions;
    /** Opened from a recipe, so "only this recipe" is on offer. */
    canScope?: boolean;
    /** Sets (or with null, clears) this recipe's own correction for the line. */
    onfix?: (raw: string, fix: Fix | null) => void | Promise<void>;
  } = $props();

  let query = $state('');
  let scope = $state<'house' | 'recipe'>('house');
  let grams = $state('');
  let busy = $state(false);
  let asking = $state(false);
  let aiPick = $state('');

  const p = $derived(parseIngredient(line));
  const m = $derived((dict.rev, lookup(p, options)));
  const w = $derived((dict.rev, weigh(p, options)));
  const shown = $derived((dict.rev, show(p, { ...options, scale: 1, system: 'metric', weigh: true, fractions: true })));
  const amount = $derived(amountOf(p));
  const fix = $derived(options.fixes?.[p.raw] ?? null);
  const rule = $derived(m.rule && m.rule.id ? m.rule : null);
  const name = $derived(m.name || p.rest);
  // Rules are kept under the name as written; Fond matches them without the describing words.
  const phrase = $derived(m.phrase ? name.toLowerCase().replace(/\([^)]*\)/g, ' ').replace(/\s+/g, ' ').trim() : '');
  const measure = $derived(p.head || (amount && 'count' in amount ? String(amount.count) : ''));

  // Start each line afresh.
  $effect(() => {
    line;
    if (open) {
      query = '';
      grams = '';
      aiPick = '';
      scope = canScope && !phrase ? 'recipe' : 'house';
    }
  });

  const found = $derived.by(() => {
    dict.rev;
    const list = searchFoods(query || name, 6);
    const ai = aiPick ? allFoods().find((f) => f.id === aiPick) : null;
    return ai ? [ai, ...list.filter((f) => f.id !== ai.id)].slice(0, 6) : list;
  });

  /** What this line would weigh as another food. */
  function as(food: Food): string {
    const r = weigh(p, { ...options, fixes: { ...(options.fixes ?? {}), [p.raw]: { foodId: food.id } } });
    if (!r) return '';
    const g = r.g >= 10 ? Math.round(r.g) : +r.g.toFixed(1);
    return `${r.approx || food.k === 'B' ? '≈ ' : ''}${g} g`;
  }

  async function apply(change: { foodId?: string; gPerMl?: number; gEach?: number; skip?: boolean }, source: 'user' | 'ai' = 'user') {
    if (busy) return;
    busy = true;
    try {
      if (scope === 'recipe' || !phrase) {
        await onfix?.(p.raw, change);
      } else {
        const r: Rule = change.skip
          ? { phrase, action: 'skip', source }
          : change.foodId
            ? { phrase, action: 'map', foodId: change.foodId, source }
            : { phrase, action: 'custom', gPerMl: change.gPerMl ?? null, gEach: change.gEach ?? null, source };
        await saveRule(r);
        // A recipe's own correction would hide the household's; clear it.
        if (fix) await onfix?.(p.raw, null);
        toast(`Fond will ${change.skip ? 'leave' : 'weigh'} “${phrase}” ${change.skip ? 'alone' : 'this way'} in every recipe`);
      }
      open = false;
    } catch (e) {
      oops(e);
    } finally {
      busy = false;
    }
  }

  function applyGrams(e: SubmitEvent) {
    e.preventDefault();
    const g = parseFloat(grams.replace(',', '.'));
    if (!(g > 0) || !amount) return;
    if ('ml' in amount) apply({ gPerMl: +(g / amount.ml).toFixed(4) });
    else apply({ gEach: +(g / amount.count).toFixed(2) });
  }

  async function clear() {
    busy = true;
    try {
      if (fix) await onfix?.(p.raw, null);
      else if (rule) await removeRule(rule);
      open = false;
    } catch (e) {
      oops(e);
    } finally {
      busy = false;
    }
  }

  async function askAI() {
    asking = true;
    try {
      const res = await post<{ suggestions: Record<string, string> }>('/food-rules/suggest', {
        phrases: [name],
        foods: allFoods().filter((f) => !f.z).map((f) => [f.id, f.n]),
      });
      aiPick = res.suggestions[name] ?? '';
      if (!aiPick) toast('The model found nothing close enough');
    } catch (e) {
      oops(e);
    } finally {
      asking = false;
    }
  }
</script>

<Sheet bind:open title="How to weigh this">
  <div class="stack">
    <p class="line serif">{line}</p>

    <div class="now" class:unknown={!w && !m.skip}>
      <Scale />
      <div>
        {#if m.skip}
          <b>Left as written</b>
          <span>{fix?.skip || rule ? 'You asked Fond not to weigh this.' : 'This is not something a scale helps with.'}</span>
        {:else if w}
          <b>{shown.approx ? '≈ ' : ''}{w.g >= 10 ? Math.round(w.g) : +w.g.toFixed(2)} g{w.match?.food ? ` as ${w.match.food.n}` : ''}</b>
          <span>
            {#if fix}Changed for this recipe.{:else if rule}Your household’s correction.{:else if w.how === 'author'}The recipe gives this weight itself.{:else if shown.basis}{shown.basis}{/if}
            {#if shown.approx && !fix && !rule} An estimate: sources disagree, or the amount depends on how it is cut or packed.{/if}
          </span>
        {:else}
          <b>Fond doesn’t know what this weighs</b>
          <span>Pick the closest food below, or weigh it once and tell Fond.</span>
        {/if}
      </div>
    </div>

    {#if fix || rule}
      <button class="btn sm quiet undo" onclick={clear} disabled={busy}><Undo />Remove {fix ? 'this recipe’s' : 'the household'} correction</button>
    {/if}

    {#if canScope && phrase}
      <div class="scope">
        <span class="label">Apply to</span>
        <div class="seg">
          <button class:on={scope === 'house'} onclick={() => (scope = 'house')}>Every recipe</button>
          <button class:on={scope === 'recipe'} onclick={() => (scope = 'recipe')}>Only this one</button>
        </div>
      </div>
    {/if}

    <section>
      <h3>Weigh it like…</h3>
      <label class="find">
        <Search />
        <input class="input" bind:value={query} placeholder="Search foods, like brown sugar" aria-label="Search foods" autocomplete="off" />
      </label>
      <div class="foods">
        {#each found as food (food.id)}
          <button class="food" onclick={() => apply({ foodId: food.id }, food.id === aiPick ? 'ai' : 'user')} disabled={busy} class:current={m.food?.id === food.id}>
            <span class="grow">{food.n}{#if food.id === aiPick}<small class="ai"><Wand />suggested</small>{/if}</span>
            <span class="num faint">{as(food)}</span>
            {#if m.food?.id === food.id}<Check />{/if}
          </button>
        {:else}
          <p class="hint">{query ? 'Nothing in Fond’s list is called that. Try a plainer name.' : 'Nothing in Fond’s list sounds like this. Search for the closest thing: chocolate chips, say, for toffee bits.'}</p>
        {/each}
      </div>
      {#if app.features.ai}
        <button class="btn sm quiet" onclick={askAI} disabled={asking}>{#if asking}<span class="spin" style:display="inline-grid"><Loader /></span>{:else}<Wand />{/if}Ask the AI model for the closest match</button>
      {/if}
    </section>

    {#if amount}
      <section>
        <h3>Or weigh it yourself</h3>
        <form class="mine" onsubmit={applyGrams}>
          <span>{measure || 'This much'} weighs</span>
          <input class="input num" bind:value={grams} inputmode="decimal" placeholder="0" aria-label="Weight in grams" />
          <span>g</span>
          <button class="btn sm" disabled={busy || !(parseFloat(grams.replace(',', '.')) > 0)}>Use this</button>
        </form>
      </section>
    {/if}

    {#if !m.skip}
      <button class="btn sm quiet leave" onclick={() => apply({ skip: true })} disabled={busy}><Ban />Don’t weigh this</button>
    {/if}
  </div>
</Sheet>

<style>
  .line { font-size: 1.15rem; line-height: 1.4; font-style: italic; color: var(--ink-2); margin-top: -6px; }
  .now { display: flex; gap: 12px; padding: 14px 16px; border-radius: var(--r-m); background: var(--wash); }
  .now.unknown { background: var(--butter-soft); }
  .now :global(svg) { flex: none; width: 20px; height: 20px; margin-top: 2px; color: var(--bay); }
  .now div { display: grid; gap: 2px; }
  .now span { font-size: 0.875rem; color: var(--ink-2); }
  .undo { justify-self: start; margin: -4px 0 0 -8px; color: var(--bay); }
  .scope { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
  section { display: grid; gap: 10px; }
  h3 { font: 600 1.1rem/1.2 var(--serif); }
  .find { position: relative; display: block; }
  .find :global(svg) { position: absolute; left: 13px; top: 50%; width: 17px; height: 17px; transform: translateY(-50%); color: var(--ink-3); pointer-events: none; }
  .find .input { padding-left: 38px; }
  .foods { display: grid; }
  .food { display: flex; align-items: center; gap: 12px; padding: 11px 4px; text-align: left; border-bottom: 1px solid var(--line); line-height: 1.3; transition: background 0.15s; }
  .food:hover { background: var(--sunk); }
  .food.current { font-weight: 600; }
  .food :global(svg) { width: 16px; height: 16px; color: var(--bay); }
  .ai { display: inline-flex; align-items: center; gap: 4px; margin-left: 8px; padding: 1px 8px; border-radius: 999px; background: var(--butter-soft); font-size: 0.75rem; font-weight: 600; color: var(--ink-2); }
  .ai :global(svg) { width: 12px; height: 12px; color: inherit; }
  .mine { display: flex; align-items: center; flex-wrap: wrap; gap: 8px; }
  .mine .input { width: 96px; min-height: 38px; padding: 6px 10px; text-align: right; font-weight: 600; }
  .leave { justify-self: start; margin-left: -8px; color: var(--ink-2); }
</style>
