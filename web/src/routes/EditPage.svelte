<script lang="ts">
  import ResolveSheet from '../components/ResolveSheet.svelte';
  import TagInput from '../components/TagInput.svelte';
  import { api, del, get, img, post, put } from '../lib/api';
  import { grow } from '../lib/grow';
  import { parseIngredient, show } from '../lib/ingredients';
  import { prefs } from '../lib/state.svelte';
  import { dict, ensureFoods, weighOptions } from '../lib/weigh.svelte';
  import { app, go, loadRecipes, oops, recipePath, toast } from '../lib/state.svelte';
  import { minutesLabel } from '../lib/steps';
  import { ImagePlus, Loader, Trash } from '../lib/icons';
  import type { Line, Recipe } from '../lib/types';

  let { id }: { id: number } = $props();
  const q = new URLSearchParams(app.query);
  let loaded = $state(id === 0);
  let busy = $state(false);
  let title = $state(q.get('title') ?? '');
  let description = $state(q.get('description') ?? '');
  let sourceUrl = $state(q.get('sourceUrl') ?? '');
  let author = $state('');
  let yieldText = $state('');
  let servings = $state('');
  let prep = $state('');
  let cook = $state('');
  let total = $state('');
  let tags = $state<string[]>([]);
  let ingredients = $state('');
  let steps = $state('');
  let notes = $state('');
  let image = $state('');
  let file = $state<File | null>(null);
  let preview = $state('');
  let removeImage = $state(false);
  let original: Recipe | null = null;
  let fileInput: HTMLInputElement;
  let dirty = $state(false);
  let versionNote = $state('');
  let resolving = $state(false);
  let resolveLine = $state('');

  const toText = (lines: Line[], sep: string) => lines.map((l) => (l.h ? '# ' + l.text : l.text)).join(sep);

  if (id) {
    get<Recipe>(`/recipes/${id}`)
      .then((r) => {
        original = r;
        title = r.title;
        description = r.description;
        sourceUrl = r.sourceUrl;
        author = r.author;
        yieldText = r.yield;
        servings = r.servings ? String(r.servings) : '';
        prep = minutesLabel(r.prepMin);
        cook = minutesLabel(r.cookMin);
        total = minutesLabel(r.totalMin);
        tags = r.tags;
        ingredients = toText(r.ingredients, '\n');
        steps = toText(r.steps, '\n\n');
        notes = r.notes;
        image = r.image;
        loaded = true;
      })
      .catch(oops);
  }

  function minutes(s: string): number {
    s = s.trim().toLowerCase();
    if (!s) return 0;
    if (/^\d+$/.test(s)) return +s;
    let m = 0;
    for (const x of s.matchAll(/(\d+(?:\.\d+)?)\s*(h|m|d)/g)) m += +x[1] * (x[2] === 'h' ? 60 : x[2] === 'd' ? 1440 : 1);
    return Math.round(m);
  }

  function ingredientLines(): Line[] {
    return ingredients.split('\n').map((l) => l.trim()).filter(Boolean).map((l) => {
      const h = l.startsWith('#') || (/:$/.test(l) && !/\d/.test(l) && l.length < 60);
      return { text: l.replace(/^#+\s*/, '').replace(/:$/, h ? '' : ':'), h };
    });
  }
  function stepLines(): Line[] {
    const blocks = /\n\s*\n/.test(steps.trim()) ? steps.split(/\n\s*\n/) : steps.split('\n');
    return blocks.map((b) => b.trim().replace(/\s*\n\s*/g, ' ')).filter(Boolean).map((b) => {
      const h = b.startsWith('#');
      return { text: h ? b.replace(/^#+\s*/, '') : b.replace(/^(?:step\s*)?\d{1,2}[.):]\s+/i, ''), h };
    });
  }
  // Alongside each line, the weight Fond would give it, so a name it cannot place shows up while typing.
  ensureFoods();
  const parsedPreview = $derived.by(() => {
    dict.rev;
    const lines = ingredientLines();
    const o = { scale: 1, system: 'metric' as const, weigh: true, fractions: true, ...weighOptions({ title, tags, ingredients: lines, extra: original?.extra, sourceUrl }, prefs()) };
    return lines.map((l) => {
      const p = l.h ? null : parseIngredient(l.text);
      const w = p && dict.ready ? show(p, o) : null;
      const grams = w?.grams != null && (w.estimated || w.also) ? `${w.approx ? '≈ ' : ''}${w.grams >= 10 ? Math.round(w.grams) : +w.grams.toFixed(1)} g` : '';
      return { l, p, grams, unknown: !!w && w.measurable && !w.weighed };
    });
  });
  const unknown = $derived(parsedPreview.filter((x) => x.unknown).length);
  // Saving different ingredients or a different method starts a new version; other edits do not.
  const flat = (lines: Line[]) => lines.map((l) => (l.h ? '#' : '') + l.text.replace(/\s+/g, ' ').trim()).join('\n');
  const newVersion = $derived(!!original && loaded && (flat(ingredientLines()) !== flat(original.ingredients) || flat(stepLines()) !== flat(original.steps)));

  function pick(f: File | null | undefined) {
    if (!f || !f.type.startsWith('image/')) return;
    file = f;
    removeImage = false;
    if (preview) URL.revokeObjectURL(preview);
    preview = URL.createObjectURL(f);
    dirty = true;
  }

  async function save(e?: Event) {
    e?.preventDefault();
    if (busy) return;
    if (!title.trim()) return toast('Give the recipe a title first', { bad: true });
    busy = true;
    const body = {
      ...(original ?? {}),
      title, description, sourceUrl, author, tags, notes,
      sourceName: sourceUrl === original?.sourceUrl ? original?.sourceName ?? '' : (() => { try { return new URL(sourceUrl).hostname.replace(/^www\./, ''); } catch { return ''; } })(),
      yield: yieldText || (servings ? `${servings} servings` : ''),
      servings: parseFloat(servings) || parseFloat(yieldText) || 0,
      prepMin: minutes(prep), cookMin: minutes(cook), totalMin: minutes(total),
      ingredients: ingredientLines(), steps: stepLines(),
      versionNote: newVersion ? versionNote.trim() : '',
    };
    try {
      const saved = id ? await put<Recipe>(`/recipes/${id}`, body) : await post<Recipe>('/recipes', body);
      if (file) {
        const fd = new FormData();
        fd.append('file', file);
        await api('POST', `/recipes/${saved.id}/image`, fd);
      } else if (removeImage && image) await del(`/recipes/${saved.id}/image`);
      dirty = false;
      await loadRecipes();
      go(recipePath(saved), { replace: true });
      toast(id ? 'Changes saved' : 'Recipe saved');
    } catch (err) {
      oops(err);
    } finally {
      busy = false;
    }
  }

  function key(e: KeyboardEvent) {
    if ((e.metaKey || e.ctrlKey) && e.key === 's') save(e);
  }
  function paste(e: ClipboardEvent) {
    const f = [...(e.clipboardData?.files ?? [])].find((x) => x.type.startsWith('image/'));
    if (f) pick(f);
  }
</script>

<svelte:window onkeydown={key} onpaste={paste} onbeforeunload={(e) => dirty && e.preventDefault()} />

<form class="page edit" onsubmit={save} oninput={() => (dirty = true)}>
  <div class="spread top">
    <h1 class="page-title">{id ? 'Edit recipe' : 'New recipe'}</h1>
    <div class="row">
      <button type="button" class="btn quiet" onclick={() => history.back()}>Cancel</button>
      <button class="btn primary" disabled={busy || !loaded}>{#if busy}<span class="spin" style="display:grid"><Loader /></span>{/if}Save</button>
    </div>
  </div>

  {#if loaded}
    <div class="grid2">
      <div class="stack">
        <label class="field"><span>Title</span><input class="input big serif" bind:value={title} placeholder="Grandma’s apple pie" required /></label>
        <label class="field"><span>Description</span><textarea class="input" rows="2" use:grow={description} bind:value={description} placeholder="A line or two about the dish"></textarea></label>
        <div class="four">
          <label class="field"><span>Makes</span><input class="input" bind:value={yieldText} placeholder="12 cookies" /></label>
          <label class="field"><span>Prep</span><input class="input" bind:value={prep} placeholder="20 min" /></label>
          <label class="field"><span>Cook</span><input class="input" bind:value={cook} placeholder="1 hr 10 min" /></label>
          <label class="field"><span>Total</span><input class="input" bind:value={total} placeholder="optional" /></label>
        </div>
        <div class="field"><span>Tags</span><TagInput bind:tags /></div>
      </div>
      <div class="photo" class:has={preview || (image && !removeImage)} ondragover={(e) => e.preventDefault()} ondrop={(e) => { e.preventDefault(); pick(e.dataTransfer?.files[0]); }} role="presentation">
        {#if preview || (image && !removeImage)}
          <img src={preview || img(image)} alt="" />
          <div class="photo-actions">
            <button type="button" class="btn sm" onclick={() => fileInput.click()}><ImagePlus />Replace</button>
            <button type="button" class="btn sm danger" onclick={() => { file = null; preview = ''; removeImage = true; dirty = true; }}><Trash />Remove</button>
          </div>
        {:else}
          <button type="button" class="drop" onclick={() => fileInput.click()}><ImagePlus /><span>Add a photo</span><small>Choose, drop or paste an image</small></button>
        {/if}
        <input type="file" accept="image/*" hidden bind:this={fileInput} onchange={(e) => pick(e.currentTarget.files?.[0])} />
      </div>
    </div>

    <div class="grid2 even">
      <label class="field">
        <span>Ingredients</span>
        <textarea class="input mono" rows="8" use:grow={ingredients} bind:value={ingredients} spellcheck="false" placeholder={'2 1/2 cups flour, sifted\n1 tsp salt\n# For the filling\n6 apples, peeled'}></textarea>
        <span class="hint">One per line, written the way you’d say it. Start a line with # for a section heading.</span>
      </label>
      <div class="field">
        <span>How Fond reads them</span>
        <ul class="parsed" aria-live="polite">
          {#each parsedPreview as { l, p, grams, unknown }}
            {#if l.h}<li class="h">{l.text}</li>
            {:else if p}<li>{#if p.head}<b class="num">{p.prefix}{p.head}</b>{' '}{/if}{p.rest}{#if p.alt}<i> also {p.alt.qty} {p.alt.unit.one}</i>{/if}{#if grams}<span class="wt num">{grams}</span>{:else if unknown}<button type="button" class="wt none" onclick={() => { resolveLine = l.text; resolving = true; }} title="Fond doesn’t know what this weighs. Tell it how.">no weight</button>{/if}</li>{/if}
          {:else}
            <li class="faint">Quantities and units are picked out automatically, so scaling and unit conversion just work.</li>
          {/each}
        </ul>
        {#if unknown}<span class="hint">{unknown === 1 ? 'One line has' : `${unknown} lines have`} no weight yet. Tap <b>no weight</b> to teach Fond what it is, or write the weight in yourself, like “1 cup (140 g)”.</span>{/if}
      </div>
    </div>

    <label class="field">
      <span>Method</span>
      <textarea class="input" rows="8" use:grow={steps} bind:value={steps} placeholder={'Heat the oven to 375°F.\n\nMix the flour and salt, then rub in the butter.\n\nBake for 45 minutes.'}></textarea>
      <span class="hint">Leave a blank line between steps. Times like “45 minutes” become tappable timers. Start a line with # for a section heading.</span>
    </label>
    {#if newVersion && original}
      <label class="field vnote">
        <span>What changed? <i>Optional. This save becomes version {original.version + 1}, and version {original.version} stays in the history.</i></span>
        <input class="input" bind:value={versionNote} maxlength="120" placeholder="Less sugar, longer bake…" />
      </label>
    {/if}
    <label class="field"><span>Notes</span><textarea class="input" rows="3" use:grow={notes} bind:value={notes} placeholder="Substitutions, what to serve it with, what you’d change next time"></textarea></label>
    <div class="grid2 even">
      <label class="field"><span>Original link</span><input class="input" type="url" bind:value={sourceUrl} placeholder="https://…" /></label>
      <label class="field"><span>Author</span><input class="input" bind:value={author} /></label>
    </div>
  {/if}
</form>

<ResolveSheet bind:open={resolving} line={resolveLine} options={{ flour: prefs().flour }} />

<style>
  .edit { width: min(1000px, 100%); display: grid; gap: 22px; }
  .top { position: sticky; top: 64px; z-index: 10; padding: 8px 0; background: color-mix(in srgb, var(--paper) 92%, transparent); backdrop-filter: blur(12px); -webkit-backdrop-filter: blur(12px); }
  .grid2 { display: grid; grid-template-columns: minmax(0, 1fr) 300px; gap: 22px; align-items: start; }
  .grid2.even { grid-template-columns: 1fr 1fr; }
  .four { display: grid; grid-template-columns: 1.4fr 1fr 1fr 1fr; gap: 12px; }
  .big { font-size: 1.45rem; font-weight: 600; height: 54px; }
  .mono { font-variant-numeric: tabular-nums; }
  .photo { position: relative; aspect-ratio: 4 / 3; border-radius: var(--r-l); overflow: hidden; background: var(--sunk); }
  .photo img { width: 100%; height: 100%; object-fit: cover; }
  .photo-actions { position: absolute; left: 10px; right: 10px; bottom: 10px; display: flex; gap: 8px; }
  .drop { width: 100%; height: 100%; display: grid; place-content: center; justify-items: center; gap: 6px; border: 1.5px dashed var(--line-2); border-radius: var(--r-l); color: var(--ink-2); font-weight: 600; transition: border-color 0.15s, background 0.15s; }
  .drop:hover { border-color: var(--bay); background: var(--wash); }
  .drop :global(svg) { width: 28px; height: 28px; color: var(--bay); }
  .drop small { font-weight: 400; color: var(--ink-3); }
  .parsed { min-height: 120px; padding: 10px 14px; border-radius: var(--r-m); background: var(--sunk); line-height: 1.45; font-size: 0.95rem; }
  .parsed li { padding: 3px 0; }
  .vnote i { font-weight: 400; font-style: normal; color: var(--ink-3); }
  .wt { float: right; margin-left: 10px; font-size: 0.82rem; color: var(--ink-3); }
  .wt.none { padding: 0 8px; border-radius: 999px; background: var(--butter-soft); color: var(--ink-2); font-weight: 600; transition: background 0.15s; }
  .wt.none:hover { background: var(--butter); color: #2a2508; }
  .parsed li.h { font: italic 600 1rem var(--serif); color: var(--ink-2); padding-top: 8px; }
  .parsed b { color: var(--bay); }
  .parsed i { color: var(--ink-3); font-size: 0.85em; }
  @media (max-width: 820px) {
    .top { top: 54px; }
    .grid2, .grid2.even { grid-template-columns: 1fr; }
    .four { grid-template-columns: 1fr 1fr; }
    .photo { order: -1; }
  }
</style>
