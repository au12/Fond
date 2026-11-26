<script lang="ts">
  import Cover from '../components/Cover.svelte';
  import Sheet from '../components/Sheet.svelte';
  import { post } from '../lib/api';
  import { COVER_COLORS, newLeather } from '../lib/leather';
  import { app, go, loadCookbooks, oops } from '../lib/state.svelte';
  import { Plus, RefreshCw } from '../lib/icons';

  let creating = $state(false);
  let title = $state('');
  let subtitle = $state('');
  let color = $state('bay');
  // Every new book is dealt a hide of its own. It is saved with the book and stays until changed by hand.
  let leather = $state(newLeather());
  let loaded = $state(app.cookbooks.length > 0);
  loadCookbooks().finally(() => (loaded = true));

  function start() {
    leather = newLeather();
    creating = true;
  }

  async function create(e: SubmitEvent) {
    e.preventDefault();
    try {
      const b = await post('/cookbooks', { title, subtitle, color, leather });
      await loadCookbooks();
      creating = false;
      title = subtitle = '';
      go(`/books/${b.id}`);
    } catch (err) {
      oops(err);
    }
  }
</script>

<div class="page">
  <div class="spread" style:margin-bottom="24px">
    <h1 class="page-title">Cookbooks</h1>
    <button class="btn" onclick={start}><Plus />New cookbook</button>
  </div>
  {#if !loaded}
    <div></div>
  {:else if app.cookbooks.length === 0}
    <div class="empty">
      <div style="width:150px"><Cover title="Sunday Suppers" subtitle="Slow food for a full table" color="bay" leather={1979} /></div>
      <h2>Gather recipes into cookbooks</h2>
      <p>Arrange recipes into chapters, share the whole book with a link, or print it. A recipe can live in as many cookbooks as you like.</p>
      <button class="btn primary" onclick={start}><Plus />Make your first cookbook</button>
    </div>
  {:else}
    <div class="shelf">
      {#each app.cookbooks as b (b.id)}
        <a class="book" href={`/books/${b.id}`}>
          <Cover title={b.title} subtitle={b.subtitle} color={b.color} leather={b.leather} />
          <span class="faint small num">{b.count} recipe{b.count === 1 ? '' : 's'}</span>
        </a>
      {/each}
    </div>
  {/if}
</div>

<Sheet bind:open={creating} title="New cookbook">
  <form class="stack" onsubmit={create}>
    <div class="make">
      <div style="width:120px;flex:none"><Cover title={title || 'Title'} {subtitle} {color} {leather} /></div>
      <div class="stack grow">
        <!-- svelte-ignore a11y_autofocus -->
        <label class="field"><span>Title</span><input class="input" bind:value={title} required autofocus placeholder="Weeknight Dinners" /></label>
        <label class="field"><span>Subtitle</span><input class="input" bind:value={subtitle} placeholder="Optional" /></label>
      </div>
    </div>
    <div class="field">
      <span>Cover</span>
      <div class="row wrap">{#each COVER_COLORS as c}<button type="button" class={`swatch cover ${c}`} class:on={color === c} onclick={() => (color = c)} aria-label={c} aria-pressed={color === c}></button>{/each}</div>
      <div class="row wrap" style:margin-top="4px">
        <button type="button" class="btn sm" onclick={() => (leather = newLeather())}><RefreshCw />New leather</button>
        <span class="hint">No two hides are alike. Once the book is made, its cover stays as it is.</span>
      </div>
    </div>
    <button class="btn primary" disabled={!title.trim()}>Create cookbook</button>
  </form>
</Sheet>

<style>
  .shelf { display: grid; grid-template-columns: repeat(auto-fill, minmax(min(170px, 42vw), 1fr)); gap: 34px 26px; }
  .book { display: grid; gap: 10px; justify-items: center; transition: transform 0.3s var(--ease); }
  .book:hover { transform: translateY(-5px) rotate(-1deg); }
  .book :global(.cover) { width: 100%; box-shadow: inset 0.7em 0 0.9em -0.6em rgb(0 0 0 / 0.45), 0 10px 26px rgb(20 30 25 / 0.2); }
  .make { display: flex; gap: 18px; align-items: center; }
  :global(.swatch.cover) { width: 36px; height: 36px; aspect-ratio: auto; padding: 0; border-radius: 50%; box-shadow: none; }
  :global(.swatch.cover)::before, :global(.swatch.cover)::after { display: none; }
  :global(.swatch.cover.on) { outline: 2px solid var(--ink); outline-offset: 3px; }
</style>
