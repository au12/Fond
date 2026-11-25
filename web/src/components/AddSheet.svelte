<script lang="ts">
  import Sheet from './Sheet.svelte';
  import Pot from './Pot.svelte';
  import { ApiError, api, del, post } from '../lib/api';
  import { app, go, loadRecipes, recipePath, toast } from '../lib/state.svelte';
  import { Camera, ClipboardPaste, FileText, Link, Pencil } from '../lib/icons';

  type Mode = 'menu' | 'working' | 'done' | 'text' | 'failed' | 'duplicate';
  let mode = $state<Mode>('menu');
  let url = $state('');
  let text = $state('');
  let error = $state('');
  let draft = $state<Record<string, string>>({});
  let dup = $state<{ id: number; slug: string; title: string } | null>(null);
  let phase = $state(0);
  let fileInput: HTMLInputElement;
  const phases = ['Opening the page', 'Finding the ingredients', 'Reading the steps', 'Fetching the photo'];
  let phaseTimer = 0;

  $effect(() => {
    if (app.adding) {
      mode = 'menu';
      error = '';
      text = '';
      url = app.addUrl;
      if (/^https?:\/\/\S+$/i.test(url.trim())) importLink();
    }
  });

  function working() {
    mode = 'working';
    phase = 0;
    clearInterval(phaseTimer);
    phaseTimer = window.setInterval(() => (phase = Math.min(phase + 1, phases.length - 1)), 1400);
  }

  function finish(recipe: any) {
    clearInterval(phaseTimer);
    mode = 'done';
    loadRecipes();
    setTimeout(() => {
      app.adding = false;
      go(recipePath(recipe));
      toast('Saved to your recipes', {
        action: { label: 'Undo', run: async () => { await del(`/recipes/${recipe.id}`).catch(() => {}); loadRecipes(); go('/'); } },
      });
    }, 650);
  }

  function failed(e: unknown) {
    clearInterval(phaseTimer);
    if (e instanceof ApiError && e.data?.draft) draft = e.data.draft;
    else draft = url ? { sourceUrl: url } : {};
    error = e instanceof Error ? e.message : 'That didn’t work';
    mode = 'failed';
  }

  async function importLink(force = false) {
    if (!url.trim()) return;
    working();
    try {
      const res = await post('/import/url', { url: url.trim(), force });
      if (res.duplicate) {
        clearInterval(phaseTimer);
        dup = res.recipe;
        mode = 'duplicate';
      } else finish(res.recipe);
    } catch (e) {
      failed(e);
    }
  }

  async function importText() {
    working();
    try {
      finish((await post('/import/text', { text })).recipe);
    } catch (e) {
      failed(e);
    }
  }

  async function importPhoto(e: Event) {
    const f = (e.target as HTMLInputElement).files?.[0];
    if (!f) return;
    working();
    const fd = new FormData();
    fd.append('file', f);
    try {
      finish((await api('POST', '/import/photo', fd)).recipe);
    } catch (err) {
      failed(err);
    }
  }

  async function paste() {
    try {
      url = (await navigator.clipboard.readText()).trim();
    } catch {
      toast('Fond isn’t allowed to read the clipboard here. Paste with your keyboard instead.');
    }
  }

  function startBlank(params: Record<string, string> = {}) {
    app.adding = false;
    const q = new URLSearchParams(params).toString();
    go('/new' + (q ? '?' + q : ''));
  }
</script>

<Sheet bind:open={app.adding} title={mode === 'menu' ? 'Add a recipe' : mode === 'text' ? 'Paste a recipe' : ''}>
  {#if mode === 'menu'}
    <form class="stack" onsubmit={(e) => { e.preventDefault(); importLink(); }}>
      <label class="field">
        <span>From a link</span>
        <div class="linkrow">
          <Link />
          <!-- svelte-ignore a11y_autofocus -->
          <input class="input" bind:value={url} type="url" inputmode="url" placeholder="https://…" autocomplete="off" autofocus />
          {#if !url && navigator.clipboard?.readText}<button type="button" class="btn sm quiet" onclick={paste}><ClipboardPaste />Paste</button>{/if}
        </div>
      </label>
      <button class="btn primary" disabled={!url.trim()}>Save recipe</button>
    </form>
    <div class="or">or start another way</div>
    <div class="options">
      <button onclick={() => startBlank()}><Pencil /><span>Write it yourself</span></button>
      <button onclick={() => (mode = 'text')}><FileText /><span>Paste the text</span></button>
      {#if app.features.ai}<button onclick={() => fileInput.click()}><Camera /><span>Read a photo</span></button>{/if}
    </div>
    <input type="file" accept="image/*" hidden bind:this={fileInput} onchange={importPhoto} />
  {:else if mode === 'text'}
    <div class="stack">
      <p class="muted small">Paste a whole recipe. Put the title on the first line; Fond splits the rest at the “Ingredients” and “Instructions” headings.</p>
      <textarea class="input" rows="12" bind:value={text} placeholder={'Weeknight Dal\n\nIngredients\n1 cup red lentils\n…\n\nInstructions\nRinse the lentils…'}></textarea>
      <div class="row" style:justify-content="flex-end">
        <button class="btn quiet" onclick={() => (mode = 'menu')}>Back</button>
        <button class="btn primary" disabled={text.trim().length < 20} onclick={importText}>Save recipe</button>
      </div>
    </div>
  {:else if mode === 'working' || mode === 'done'}
    <div class="center">
      <Pot done={mode === 'done'} />
      <p class="serif status">{mode === 'done' ? 'Saved' : phases[phase]}</p>
    </div>
  {:else if mode === 'duplicate' && dup}
    <div class="center">
      <p class="serif status">You already have this one</p>
      <p class="muted">“{dup.title}” was saved from the same link.</p>
      <div class="row wrap" style:justify-content="center">
        <button class="btn primary" onclick={() => { app.adding = false; go(recipePath(dup!)); }}>Open it</button>
        <button class="btn" onclick={() => importLink(true)}>Save a second copy</button>
      </div>
    </div>
  {:else}
    <div class="center">
      <p class="serif status">No recipe found</p>
      <p class="muted">{error}</p>
      <div class="row wrap" style:justify-content="center">
        <button class="btn primary" onclick={() => startBlank(draft)}>Fill it in myself</button>
        <button class="btn" onclick={() => (mode = 'menu')}>Try another link</button>
      </div>
    </div>
  {/if}
</Sheet>

<style>
  .linkrow { position: relative; display: flex; align-items: center; }
  .linkrow > :global(svg) { position: absolute; left: 13px; width: 18px; height: 18px; color: var(--ink-3); pointer-events: none; }
  .linkrow .input { padding-left: 40px; height: 48px; }
  .linkrow .btn { position: absolute; right: 6px; }
  .or { margin: 22px 0 12px; font-size: 0.85rem; color: var(--ink-3); text-align: center; }
  .options { display: grid; grid-template-columns: repeat(auto-fit, minmax(130px, 1fr)); gap: 10px; }
  .options button { display: grid; justify-items: center; gap: 8px; padding: 16px 10px; border-radius: 14px; background: var(--sunk); font-weight: 600; font-size: 0.9rem; transition: background 0.15s, transform 0.15s var(--ease); }
  .options button:hover { background: var(--wash); }
  .options button:active { transform: scale(0.97); }
  .options :global(svg) { width: 22px; height: 22px; color: var(--bay); }
  .center { display: grid; justify-items: center; text-align: center; gap: 12px; padding: 18px 0 8px; }
  .status { font-size: 1.45rem; font-weight: 600; min-height: 1.4em; }
</style>
