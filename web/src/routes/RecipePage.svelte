<script lang="ts">
  import { onDestroy } from 'svelte';
  import { slide } from 'svelte/transition';
  import Photo from '../components/Photo.svelte';
  import Rich from '../components/Rich.svelte';
  import Sheet from '../components/Sheet.svelte';
  import ShareSheet from '../components/ShareSheet.svelte';
  import { del, get, post, put } from '../lib/api';
  import { cookState, resetCook, saveCook } from '../lib/cook.svelte';
  import { bakers, formatNumber, parseIngredient, show, type System } from '../lib/ingredients';
  import { app, go, loadCookbooks, loadRecipes, oops, prefs, recipePath, timeAgo, toast } from '../lib/state.svelte';
  import { clock, minutesLabel, tokenize } from '../lib/steps';
  import { left, startTimer, timerFor, toggleTimer } from '../lib/timers.svelte';
  import { keepAwake } from '../lib/wake';
  import { ArrowLeft, BookPlus, Check, Ellipsis, ExternalLink, Heart, ListPlus, Minus, Pencil, Percent, Plus, Printer, RotateCcw, Share, Star, Timer, Trash, Wheat } from '../lib/icons';
  import type { Recipe } from '../lib/types';

  let { id }: { id: number } = $props();
  let r = $state<Recipe | null>(null);
  let missing = $state(false);
  let system = $state<System>(prefs().system);
  let baker = $state(false);
  let menu = $state(false);
  let sharing = $state(false);
  let booksOpen = $state(false);
  let shopOpen = $state(false);
  let shopPick = $state<boolean[]>([]);
  let heartPop = $state(false);
  let newBook = $state('');
  let celebrated = false;

  const cook = cookState(id);
  const pf = $derived(prefs());

  get<Recipe>(`/recipes/${id}`)
    .then((x) => {
      x.shareToken ??= '';
      r = x;
      document.title = `${x.title} · Fond`;
      baker = pf.baker && !!x.extra?.baker;
    })
    .catch(() => (missing = true));

  $effect(() => keepAwake(pf.awake && !!r));
  onDestroy(() => {
    keepAwake(false);
    document.title = 'Fond';
  });

  const opts = $derived({ scale: cook.scale, system, weigh: pf.weigh, fractions: pf.fractions });
  const parsed = $derived((r?.ingredients ?? []).map((l) => (l.h ? null : parseIngredient(l.text))));
  const items = $derived(parsed.filter((p) => p != null));
  const bake = $derived(bakers(items, r?.extra?.flour ?? {}));
  const canBake = $derived(bake.flour > 0 && bake.rows.filter((x) => x.grams != null).length >= 2);
  const stepNumbers = $derived.by(() => {
    let n = 0;
    return (r?.steps ?? []).map((s) => (s.h ? 0 : ++n));
  });
  const stepTotal = $derived(stepNumbers.filter(Boolean).length);
  const stepsDone = $derived(stepNumbers.filter((n, i) => n && cook.steps[i]).length);
  const progress = $derived(Object.values(cook.ing).some(Boolean) || stepsDone > 0 || cook.scale !== 1);
  const servings = $derived(r && r.servings > 0 ? r.servings * cook.scale : 0);
  const yieldText = $derived.by(() => {
    if (!r?.yield) return '';
    if (cook.scale === 1 || !r.servings) return r.yield;
    return r.yield.replace(/\d+(?:[.,]\d+)?/, formatNumber(r.servings * cook.scale, true));
  });

  function setScale(s: number) {
    cook.scale = Math.min(Math.max(s, 0.125), 50);
    saveCook(id);
  }
  function stepServings(d: number) {
    if (!r) return;
    if (r.servings > 0) {
      const step = r.servings >= 12 ? Math.max(1, Math.round(r.servings / 4)) : 1;
      setScale(Math.max(step, Math.round(servings + d * step)) / r.servings);
    } else {
      const ladder = [0.25, 0.5, 1, 1.5, 2, 3, 4, 6, 8];
      const i = ladder.findIndex((x) => Math.abs(x - cook.scale) < 0.01);
      setScale(ladder[Math.min(Math.max((i < 0 ? 2 : i) + d, 0), ladder.length - 1)]);
    }
  }
  function setFlour(g: number) {
    if (g > 0 && bake.flour > 0) setScale(g / bake.flour);
  }

  function tickIng(i: number) {
    cook.ing[i] = !cook.ing[i];
    saveCook(id);
  }
  function tickStep(i: number) {
    cook.steps[i] = !cook.steps[i];
    saveCook(id);
  }
  $effect(() => {
    if (stepTotal > 0 && stepsDone === stepTotal && !celebrated) {
      celebrated = true;
      toast('Every step done. Enjoy it!', { action: { label: 'I made this', run: () => cooked() } });
    } else if (stepsDone < stepTotal) celebrated = false;
  });

  function timer(stepIndex: number, label: string, seconds: number) {
    if (!r) return;
    const existing = timerFor(id, stepIndex, label);
    if (existing) return toggleTimer(existing.id);
    startTimer({ label, recipeId: id, recipeTitle: r.title, recipePath: recipePath(r), step: stepIndex, total: seconds });
  }

  async function fav() {
    if (!r) return;
    r.favorite = !r.favorite;
    heartPop = r.favorite;
    const card = app.recipes.find((c) => c.id === id);
    if (card) card.fav = r.favorite;
    put(`/recipes/${id}/favorite`, { on: r.favorite }).catch(oops);
  }
  async function cooked(undo = false) {
    if (!r) return;
    try {
      const res = await post(`/recipes/${id}/cooked`, { undo });
      r.cookedCount = res.cookedCount;
      r.lastCooked = res.lastCooked;
      if (!undo) toast('Added to your cooking log', { action: { label: 'Undo', run: () => cooked(true) } });
      loadRecipes();
    } catch (e) {
      oops(e);
    }
  }
  async function rate(n: number) {
    if (!r) return;
    r.rating = r.rating === n ? 0 : n;
    put(`/recipes/${id}/rating`, { rating: r.rating }).catch(oops);
  }
  async function remove() {
    if (!r || !confirm(`Delete “${r.title}”? This can’t be undone.`)) return;
    try {
      await del(`/recipes/${id}`);
      await loadRecipes();
      go('/', { replace: true });
      toast('Recipe deleted');
    } catch (e) {
      oops(e);
    }
  }
  async function toggleBook(bookId: number) {
    if (!r) return;
    r.cookbooks = r.cookbooks.includes(bookId) ? r.cookbooks.filter((x) => x !== bookId) : [...r.cookbooks, bookId];
    try {
      await put(`/recipes/${id}/cookbooks`, { ids: r.cookbooks });
      loadCookbooks();
    } catch (e) {
      oops(e);
    }
  }
  async function createBook(e: SubmitEvent) {
    e.preventDefault();
    if (!newBook.trim() || !r) return;
    try {
      const b = await post('/cookbooks', { title: newBook.trim() });
      newBook = '';
      await loadCookbooks();
      toggleBook(b.id);
    } catch (err) {
      oops(err);
    }
  }
  function openShop() {
    shopPick = (r?.ingredients ?? []).map((l, i) => !l.h && !cook.ing[i]);
    shopOpen = true;
  }
  async function addToList() {
    if (!r) return;
    const items = r.ingredients
      .map((l, i) => ({ l, i }))
      .filter(({ l, i }) => !l.h && shopPick[i])
      .map(({ i }) => {
        const s = show(parsed[i]!, opts);
        return { text: (s.measure + ' ' + s.rest).trim(), recipeId: id, recipeTitle: r!.title };
      });
    try {
      await post('/shopping', { items });
      shopOpen = false;
      toast(`${items.length} item${items.length === 1 ? '' : 's'} added to your shopping list`, { action: { label: 'View', run: () => go('/list') } });
    } catch (e) {
      oops(e);
    }
  }
  async function toggleFlour(raw: string, on: boolean) {
    if (!r) return;
    r.extra = { ...(r.extra ?? {}), flour: { ...(r.extra?.flour ?? {}), [raw]: on } };
    put(`/recipes/${id}`, r).catch(oops);
  }
  function toggleBaker() {
    if (!r) return;
    baker = !baker;
    r.extra = { ...(r.extra ?? {}), baker };
    put(`/recipes/${id}`, r).catch(oops);
  }
  const clean = (s: string) => s.replace(/\*\*|__/g, '');
  const facts = $derived(
    r ? ([['Prep', minutesLabel(r.prepMin)], ['Cook', minutesLabel(r.cookMin)], ['Total', minutesLabel(r.totalMin)], ['Makes', yieldText]] as const).filter((f) => f[1]) : [],
  );
</script>

<svelte:window onclick={() => (menu = false)} />

{#if missing}
  <div class="page"><div class="empty"><h2>This recipe isn’t here</h2><p>It may have been deleted.</p><a class="btn primary" href="/">Back to recipes</a></div></div>
{:else if r}
  <article class="page recipe">
    <div class="bar">
      <button class="btn quiet sm back" onclick={() => (history.length > 1 ? history.back() : go('/'))}><ArrowLeft />Recipes</button>
      <div class="grow"></div>
      <button class="icon-btn heart" class:on={r.favorite} class:pop={heartPop} onanimationend={() => (heartPop = false)} onclick={fav} aria-label="Favorite" aria-pressed={r.favorite}><Heart /></button>
      <button class="icon-btn" class:on={r.cookbooks.length > 0} onclick={() => (booksOpen = true)} aria-label="Add to a cookbook" title="Cookbooks"><BookPlus /></button>
      <button class="icon-btn" class:on={!!r.shareToken} onclick={() => (sharing = true)} aria-label="Share" title="Share"><Share /></button>
      <a class="icon-btn deskonly" href={`${recipePath(r)}/print`} aria-label="Print" title="Print"><Printer /></a>
      <a class="icon-btn" href={`${recipePath(r)}/edit`} aria-label="Edit" title="Edit"><Pencil /></a>
      <div style:position="relative">
        <button class="icon-btn" onclick={(e) => { e.stopPropagation(); menu = !menu; }} aria-label="More" aria-expanded={menu}><Ellipsis /></button>
        {#if menu}
          <div class="menu" style="right:0;top:46px" transition:slide={{ duration: 140 }}>
            <a href={`${recipePath(r)}/print`}><Printer />Print or save as PDF</a>
            <button onclick={openShop}><ListPlus />Add to shopping list</button>
            {#if progress}<button onclick={() => resetCook(id)}><RotateCcw />Clear my progress</button>{/if}
            <button class="danger" onclick={remove}><Trash />Delete recipe</button>
          </div>
        {/if}
      </div>
    </div>

    <header class="head" class:nophoto={!r.image}>
      <div class="intro">
        <h1>{r.title}</h1>
        {#if r.description}<p class="desc">{r.description}</p>{/if}
        {#if facts.length}
          <dl class="facts">{#each facts as [k, v]}<div><dt>{k}</dt><dd class="num">{v}</dd></div>{/each}</dl>
        {/if}
        {#if r.sourceUrl || r.author}
          <p class="source">
            {#if r.sourceUrl}From <a href={r.sourceUrl} target="_blank" rel="noopener noreferrer">{r.sourceName || 'the original page'}<ExternalLink /></a>{/if}
            {#if r.author && r.author !== r.sourceName}<span>{r.sourceUrl ? 'by' : 'By'} {r.author}</span>{/if}
          </p>
        {/if}
        {#if r.tags.length}<div class="tags">{#each r.tags as t}<span class="chip">{t}</span>{/each}</div>{/if}
      </div>
      {#if r.image}
        <div class="hero" style:view-transition-name={`photo-${r.id}`}><Photo image={r.image} color={r.imageColor} thumb={false} eager /></div>
      {/if}
    </header>

    <div class="controls">
      <div class="stepper" role="group" aria-label="Scale recipe">
        <button onclick={() => stepServings(-1)} aria-label="Make less"><Minus /></button>
        <span class="num">{#if servings}{formatNumber(servings, true)} <small>{servings === 1 ? 'serving' : 'servings'}</small>{:else}{formatNumber(cook.scale, true)}× <small>recipe</small>{/if}</span>
        <button onclick={() => stepServings(1)} aria-label="Make more"><Plus /></button>
      </div>
      {#if cook.scale !== 1}<button class="btn sm quiet" onclick={() => setScale(1)}><RotateCcw />Original</button>{/if}
      <div class="grow"></div>
      {#if canBake}<button class="chip" class:on={baker} onclick={toggleBaker} aria-pressed={baker}><Percent />Baker’s %</button>{/if}
      <div class="seg" role="group" aria-label="Units">
        <button class:on={system === 'original'} onclick={() => (system = 'original')}>As written</button>
        <button class:on={system === 'metric'} onclick={() => (system = 'metric')}>Metric</button>
        <button class:on={system === 'us'} onclick={() => (system = 'us')}>US</button>
      </div>
    </div>

    <div class="cols">
      <section class="ingredients" aria-label="Ingredients">
        <h2>Ingredients</h2>
        {#if baker && canBake}
          <div class="bakerbox" transition:slide={{ duration: 200 }}>
            <label><span>Flour</span><input class="num" type="number" inputmode="decimal" min="1" value={Math.round(bake.flour * cook.scale)} onchange={(e) => setFlour(+e.currentTarget.value)} aria-label="Total flour in grams" /> g</label>
            {#if bake.hydration != null}<div><span>Hydration</span><b class="num">{Math.round(bake.hydration)}%</b></div>{/if}
            <div><span>Total</span><b class="num">{Math.round(bake.total * cook.scale)} g</b></div>
          </div>
        {/if}
        <ul>
          {#each r.ingredients as line, i}
            {#if line.h}
              <li class="group">{line.text}</li>
            {:else}
              {@const p = parsed[i]!}
              {@const s = show(p, opts)}
              {@const row = bake.rows[items.indexOf(p)]}
              <li class:done={cook.ing[i]}>
                <button class="ing" onclick={() => tickIng(i)} aria-pressed={!!cook.ing[i]}>
                  <span class="box"><Check /></span>
                  <span class="txt">{#if s.measure}<b class="num" class:conv={s.converted} title={s.converted ? `Converted from ${s.from}${s.estimated ? ' using a standard density' : ''}` : undefined}>{s.measure}</b>{' '}{/if}{s.rest}</span>
                </button>
                {#if baker && canBake}
                  <button class="pct num" class:flour={row.flour} onclick={() => toggleFlour(p.raw, !row.flour)} title={row.flour ? 'Counted as flour. Tap to change.' : 'Tap to count this as flour'}>
                    {#if row.flour}<Wheat />{/if}{row.percent != null ? (row.percent < 10 ? +row.percent.toFixed(1) : Math.round(row.percent)) + '%' : '–'}
                  </button>
                {/if}
              </li>
            {/if}
          {/each}
        </ul>
        <button class="btn sm quiet shop" onclick={openShop}><ListPlus />Add to shopping list</button>
      </section>

      <section class="method" aria-label="Method">
        <div class="spread">
          <h2>Method</h2>
          {#if stepsDone > 0}<span class="faint small num">{stepsDone} of {stepTotal} done</span>{/if}
        </div>
        {#if cook.scale !== 1}<p class="note">Amounts written inside the steps are for the original recipe.</p>{/if}
        <ol>
          {#each r.steps as step, i}
            {#if step.h}
              <li class="group">{step.text}</li>
            {:else}
              <li class="step" class:done={cook.steps[i]}>
                <button class="n" onclick={() => tickStep(i)} aria-pressed={!!cook.steps[i]} aria-label={`Step ${stepNumbers[i]} ${cook.steps[i] ? 'done' : 'not done'}`}>
                  <span class="digit serif">{stepNumbers[i]}</span><span class="tick"><Check /></span>
                </button>
                <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_noninteractive_element_interactions -->
                <p onclick={(e) => !(e.target as Element).closest('button,a') && tickStep(i)}>
                  {#each tokenize(clean(step.text), pf.temp === 'original' && system !== 'original' ? (system === 'metric' ? 'C' : 'F') : pf.temp) as tok}{#if tok.t === 'timer'}{@const running = timerFor(id, i, tok.v)}<button class="timer" class:running={running && !running.done} class:finished={running?.done} onclick={() => timer(i, tok.v, tok.seconds)} title="Start a timer"><Timer />{#if running && !running.done}<span class="num">{clock(left(running))}</span>{:else}{tok.v}{/if}</button>{:else if tok.t === 'temp'}<span class="temp" class:conv={tok.converted} title={tok.converted ? `Written as ${tok.v}` : undefined}>{tok.shown}</span>{:else}{tok.v}{/if}{/each}
                </p>
              </li>
            {/if}
          {/each}
        </ol>

        {#if r.notes}
          <section class="notes"><h3>Notes</h3><Rich text={r.notes} /></section>
        {/if}
        {#if r.nutrition && Object.keys(r.nutrition).length}
          <section class="notes">
            <h3>Nutrition <span class="faint">per serving</span></h3>
            <dl class="nutri">{#each Object.entries(r.nutrition) as [k, v]}<div><dt>{k.replace(/([A-Z])/g, ' $1').toLowerCase()}</dt><dd class="num">{v}</dd></div>{/each}</dl>
          </section>
        {/if}

        <footer class="log">
          <button class="btn" onclick={() => cooked()}><Check />I made this</button>
          <span class="muted small">{#if r.cookedCount}Made {r.cookedCount === 1 ? 'once' : `${r.cookedCount} times`}, last {timeAgo(r.lastCooked)}{:else}Not made yet{/if}</span>
          <div class="grow"></div>
          <div class="stars" role="group" aria-label="Rating">
            {#each [1, 2, 3, 4, 5] as n}
              <button class:on={r.rating >= n} onclick={() => rate(n)} aria-label={`${n} star${n > 1 ? 's' : ''}`} aria-pressed={r.rating >= n}><Star /></button>
            {/each}
          </div>
        </footer>
      </section>
    </div>
  </article>

  <ShareSheet bind:open={sharing} kind="recipe" {id} title={r.title} bind:token={r.shareToken as string} />

  <Sheet bind:open={booksOpen} title="Cookbooks">
    <div class="stack">
      {#if app.cookbooks.length === 0}<p class="muted">A cookbook is a collection you arrange yourself. A recipe can sit in as many as you like.</p>{/if}
      <div class="books">
        {#each app.cookbooks as b (b.id)}
          <button class="bookrow" onclick={() => toggleBook(b.id)} aria-pressed={r.cookbooks.includes(b.id)}>
            <span class="box" class:checked={r.cookbooks.includes(b.id)}><Check /></span>
            <span class="grow">{b.title}</span>
            <span class="faint small num">{b.count}</span>
          </button>
        {/each}
      </div>
      <form class="row" onsubmit={createBook}>
        <input class="input" bind:value={newBook} placeholder="New cookbook name" aria-label="New cookbook name" />
        <button class="btn" disabled={!newBook.trim()}><Plus />Create</button>
      </form>
    </div>
  </Sheet>

  <Sheet bind:open={shopOpen} title="Add to shopping list">
    <div class="stack">
      <div class="books">
        {#each r.ingredients as line, i}
          {#if line.h}
            <div class="label" style:padding="8px 4px 2px">{line.text}</div>
          {:else}
            {@const s = show(parsed[i]!, opts)}
            <button class="bookrow" onclick={() => (shopPick[i] = !shopPick[i])} aria-pressed={shopPick[i]}>
              <span class="box" class:checked={shopPick[i]}><Check /></span>
              <span class="grow">{s.measure} {s.rest}</span>
            </button>
          {/if}
        {/each}
      </div>
      <button class="btn primary" disabled={!shopPick.some(Boolean)} onclick={addToList}>Add {shopPick.filter(Boolean).length} to list</button>
    </div>
  </Sheet>
{:else}
  <div class="page recipe"><div class="skel"></div></div>
{/if}

<style>
  .recipe { padding-top: 10px; }
  .bar { display: flex; align-items: center; gap: 2px; margin: 0 -8px 14px; }
  .back { padding-left: 10px; }
  .heart.on { color: var(--tomato); }
  .heart.on :global(svg) { fill: currentColor; }
  .head { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 0.92fr); gap: clamp(24px, 4vw, 56px); align-items: center; }
  .head.nophoto { grid-template-columns: minmax(0, 760px); }
  h1 { font: 600 clamp(2rem, 4.4vw, 3.2rem)/1.06 var(--serif); letter-spacing: -0.02em; text-wrap: balance; }
  .desc { margin-top: 14px; font: 400 1.14rem/1.55 var(--serif); color: var(--ink-2); max-width: 58ch; white-space: pre-line; }
  .facts { display: flex; flex-wrap: wrap; gap: 8px 30px; margin-top: 22px; }
  .facts dt { font-size: 0.8rem; color: var(--ink-3); }
  .facts dd { font-weight: 600; font-size: 1.02rem; }
  .source { margin-top: 16px; font-size: 0.92rem; color: var(--ink-2); display: flex; flex-wrap: wrap; gap: 6px; }
  .source a { display: inline-flex; align-items: center; gap: 4px; font-weight: 600; color: var(--bay); }
  .source :global(svg) { width: 13px; height: 13px; }
  .tags { display: flex; flex-wrap: wrap; gap: 6px; margin-top: 16px; }
  .tags .chip { height: 28px; font-size: 0.8rem; pointer-events: none; }
  .hero { position: relative; aspect-ratio: 4 / 3; border-radius: var(--r-l); overflow: hidden; background: var(--sunk); isolation: isolate; }
  .hero :global(img) { width: 100%; height: 100%; object-fit: cover; transition: opacity 0.4s; }

  .controls { position: sticky; top: 64px; z-index: 20; display: flex; align-items: center; flex-wrap: wrap; gap: 10px; margin: 30px 0 6px; padding: 10px 0; background: color-mix(in srgb, var(--paper) 92%, transparent); backdrop-filter: blur(12px); -webkit-backdrop-filter: blur(12px); border-bottom: 1px solid var(--line); }
  .stepper { display: inline-flex; align-items: center; height: 40px; border-radius: 999px; background: var(--surface); box-shadow: inset 0 0 0 1px var(--line-2); }
  .stepper button { width: 40px; height: 40px; display: grid; place-items: center; border-radius: 50%; color: var(--bay); }
  .stepper button:hover { background: var(--wash); }
  .stepper button:active { transform: scale(0.9); }
  .stepper :global(svg) { width: 17px; height: 17px; }
  .stepper span { min-width: 104px; text-align: center; font-weight: 700; }
  .stepper small { font-weight: 500; color: var(--ink-2); font-size: 0.85rem; }

  .cols { display: grid; grid-template-columns: minmax(260px, 0.72fr) minmax(0, 1.28fr); gap: clamp(28px, 5vw, 72px); margin-top: 22px; align-items: start; }
  h2 { font: 600 1.5rem/1.2 var(--serif); letter-spacing: -0.01em; margin-bottom: 12px; }
  h3 { font: 600 1.15rem/1.2 var(--serif); margin-bottom: 8px; }
  .ingredients { position: sticky; top: 132px; max-height: calc(100dvh - 150px); overflow: auto; padding-right: 6px; scrollbar-width: thin; }
  .group { margin: 20px 0 6px; font: 600 1.02rem/1.3 var(--serif); font-style: italic; color: var(--ink-2); }
  .ingredients li:first-child.group { margin-top: 0; }
  .ingredients li:not(.group) { display: flex; align-items: stretch; border-bottom: 1px solid var(--line); }
  .ing { flex: 1; display: flex; gap: 12px; align-items: flex-start; text-align: left; padding: 10px 2px; line-height: 1.4; }
  .ing b { font-weight: 700; }
  .box { flex: none; width: 21px; height: 21px; margin-top: 1px; border-radius: 50%; display: grid; place-items: center; box-shadow: inset 0 0 0 1.5px var(--line-2); color: transparent; transition: background 0.2s, box-shadow 0.2s, transform 0.25s var(--spring); }
  .box :global(svg) { width: 13px; height: 13px; stroke-width: 3; }
  .done .box, .box.checked { background: var(--bay); box-shadow: none; color: var(--on-bay); transform: scale(1.06); }
  .txt { text-decoration: line-through transparent 1.5px; transition: text-decoration-color 0.3s, color 0.3s; }
  .done .txt { color: var(--ink-3); text-decoration-color: currentColor; }
  .done .conv { text-decoration: inherit; }
  .conv { text-decoration: underline dotted var(--ink-3); text-underline-offset: 4px; cursor: help; }
  .pct { flex: none; display: inline-flex; align-items: center; justify-content: flex-end; gap: 4px; min-width: 66px; padding: 0 2px 0 8px; font-size: 0.9rem; font-weight: 600; color: var(--ink-2); }
  .pct.flour { color: var(--bay); }
  .pct :global(svg) { width: 14px; height: 14px; }
  .bakerbox { display: flex; flex-wrap: wrap; gap: 6px 18px; padding: 12px 14px; margin-bottom: 8px; border-radius: var(--r-m); background: var(--butter-soft); font-size: 0.9rem; }
  .bakerbox span { color: var(--ink-2); margin-right: 6px; }
  .bakerbox input { width: 72px; height: 30px; padding: 0 8px; border-radius: 8px; border: 1px solid var(--line-2); background: var(--surface); font-weight: 700; text-align: right; }
  .shop { margin: 12px 0 0 -8px; color: var(--bay); }

  .note { margin: -4px 0 12px; font-size: 0.86rem; color: var(--ink-3); }
  .step { display: grid; grid-template-columns: 46px minmax(0, 1fr); gap: 14px; padding: 14px 0; border-bottom: 1px solid var(--line); transition: opacity 0.3s; }
  .step p { font-size: 1.07rem; line-height: 1.68; max-width: 66ch; cursor: pointer; }
  .n { position: relative; width: 42px; height: 42px; border-radius: 50%; display: grid; place-items: center; background: var(--wash); color: var(--bay); transition: background 0.25s, transform 0.3s var(--spring); }
  .n:hover { transform: scale(1.06); }
  .digit { font-size: 1.3rem; font-weight: 600; font-variant-numeric: lining-nums; transition: opacity 0.2s, transform 0.3s var(--spring); }
  .tick { position: absolute; inset: 0; display: grid; place-items: center; opacity: 0; transform: scale(0.4) rotate(-30deg); transition: opacity 0.2s, transform 0.35s var(--spring); }
  .tick :global(svg) { width: 20px; height: 20px; stroke-width: 3; }
  .step.done .n { background: var(--bay); color: var(--on-bay); }
  .step.done .digit { opacity: 0; transform: scale(0.4); }
  .step.done .tick { opacity: 1; transform: none; }
  .step.done p { color: var(--ink-3); }
  .timer { display: inline-flex; align-items: center; gap: 5px; margin: 0 1px; padding: 1px 10px 1px 7px; border-radius: 999px; background: var(--butter-soft); color: var(--ink); font-weight: 600; font-size: 0.95em; line-height: 1.5; vertical-align: baseline; box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--butter) 70%, transparent); transition: background 0.2s, transform 0.15s var(--ease); }
  .timer:hover { background: var(--butter); color: #2a2508; }
  .timer:active { transform: scale(0.95); }
  .timer :global(svg) { width: 14px; height: 14px; }
  .timer.running { background: var(--ink); color: var(--paper); box-shadow: none; }
  .timer.running :global(svg) { animation: tickpulse 1s ease-in-out infinite; }
  .timer.finished { background: var(--bay); color: var(--on-bay); box-shadow: none; }
  @keyframes tickpulse { 50% { opacity: 0.35; } }
  .temp { font-weight: 600; white-space: nowrap; }
  .notes { margin-top: 30px; padding: 18px 20px; border-radius: var(--r-l); background: var(--sunk); line-height: 1.6; }
  .nutri { display: flex; flex-wrap: wrap; gap: 8px 24px; }
  .nutri dt { font-size: 0.78rem; color: var(--ink-3); }
  .nutri dd { font-weight: 600; }
  .log { display: flex; align-items: center; flex-wrap: wrap; gap: 12px; margin-top: 30px; }
  .stars { display: flex; }
  .stars button { width: 32px; height: 36px; display: grid; place-items: center; color: var(--line-2); transition: color 0.15s, transform 0.2s var(--spring); }
  .stars button:hover { transform: scale(1.15); }
  .stars button.on { color: var(--butter); }
  .stars button.on :global(svg) { fill: currentColor; stroke: color-mix(in srgb, var(--butter) 70%, #6b5a10); }
  .stars :global(svg) { width: 21px; height: 21px; }
  .books { display: grid; max-height: 46dvh; overflow: auto; }
  .bookrow { display: flex; align-items: center; gap: 12px; padding: 11px 6px; text-align: left; border-bottom: 1px solid var(--line); line-height: 1.35; }
  .skel { height: 60dvh; border-radius: var(--r-l); background: var(--sunk); animation: pulse 1.4s ease-in-out infinite; }
  @keyframes pulse { 50% { opacity: 0.5; } }

  @media (max-width: 820px) {
    .head { grid-template-columns: 1fr; gap: 18px; }
    .hero { order: -1; margin: 0 calc(-1 * clamp(16px, 4vw, 40px)); border-radius: 0; aspect-ratio: 3 / 2; }
    .bar { margin-bottom: 6px; }
    .controls { position: static; margin-top: 20px; gap: 8px; }
    .controls .grow { display: none; }
    .seg { margin-left: auto; }
    .seg button { padding: 0 10px; }
    .stepper span { min-width: 92px; }
    .cols { grid-template-columns: 1fr; gap: 34px; }
    .ingredients { position: static; max-height: none; overflow: visible; padding: 0; }
    .step { grid-template-columns: 40px minmax(0, 1fr); gap: 12px; }
    .n { width: 38px; height: 38px; }
  }
</style>
