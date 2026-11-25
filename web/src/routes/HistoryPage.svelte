<script lang="ts">
  // A recipe's history: every version of its ingredients and method, what
  // changed in each, and a way back to any of them.
  import { onDestroy } from 'svelte';
  import { get, post } from '../lib/api';
  import { diffLines, tally, tallyText, type Row } from '../lib/diff';
  import { go, oops, recipePath, timeAgo, toast } from '../lib/state.svelte';
  import { ArrowLeft, History, RotateCcw } from '../lib/icons';
  import type { Recipe, Version } from '../lib/types';

  let { id }: { id: number } = $props();
  let r = $state<Recipe | null>(null);
  let versions = $state<Version[]>([]);
  let picked = $state(0);
  let whole = $state(false);
  let missing = $state(false);
  let busy = $state(false);

  async function load() {
    try {
      const [rec, hist] = await Promise.all([get<Recipe>(`/recipes/${id}`), get<{ version: number; versions: Version[] }>(`/recipes/${id}/versions`)]);
      r = rec;
      versions = hist.versions;
      picked = hist.versions[0]?.version ?? 0;
      document.title = `History · ${rec.title} · Fond`;
    } catch {
      missing = true;
    }
  }
  load();
  onDestroy(() => (document.title = 'Fond'));

  interface Entry {
    v: Version;
    /** The version this one is compared with; none for the first. */
    from: Version | null;
    ingredients: Row[];
    steps: Row[];
    summary: string;
  }
  const entries = $derived(
    versions.map((v, i): Entry => {
      const from = versions[i + 1] ?? null;
      const ingredients = diffLines(from?.ingredients ?? v.ingredients, v.ingredients);
      const steps = diffLines(from?.steps ?? v.steps, v.steps);
      const parts = from ? [[tallyText(tally(ingredients)), 'ingredients'], [tallyText(tally(steps)), 'steps']].filter(([t]) => t).map(([t, what]) => `${what}: ${t}`) : [];
      return { v, from, ingredients, steps, summary: parts.join(' · ') };
    }),
  );
  const current = $derived(versions[0]?.version ?? 0);
  const entry = $derived(entries.find((e) => e.v.version === picked) ?? null);
  const when = (at: number) => new Date(at * 1000).toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' });
  const stepNo = (rows: Row[]) => {
    let n = 0;
    return rows.map((row) => (row.h || row.kind === 'del' ? 0 : ++n));
  };

  async function restore(v: Version) {
    if (busy || !r) return;
    if (!confirm(`Bring back version ${v.version}? Its ingredients, method and yield return as version ${current + 1}. The title, notes and photo stay as they are, and nothing in the history is lost.`)) return;
    busy = true;
    try {
      const saved = await post<Recipe>(`/recipes/${id}/versions/${v.version}/restore`);
      toast(saved.version === current ? 'The recipe already matches that version' : `Version ${v.version} is back, as version ${saved.version}`);
      go(recipePath(saved));
    } catch (e) {
      oops(e);
    } finally {
      busy = false;
    }
  }
</script>

{#if missing}
  <div class="page"><div class="empty"><h2>This recipe isn’t here</h2><p>It may have been deleted.</p><a class="btn primary" href="/">Back to recipes</a></div></div>
{:else if r}
  <div class="page history">
    <a class="btn quiet sm back" href={recipePath(r)}><ArrowLeft />{r.title}</a>
    <header>
      <h1 class="page-title">History</h1>
      <p class="muted">Each time the ingredients or the method change, Fond keeps the recipe as it was. Notes, photos and tags are not versions.</p>
    </header>

    <div class="layout">
      <ol class="rail" aria-label="Versions">
        {#each entries as e (e.v.version)}
          <li>
            <button class="stop" class:on={picked === e.v.version} onclick={() => (picked = e.v.version)} aria-current={picked === e.v.version}>
              <span class="dot" class:now={e.v.version === current}></span>
              <span class="what">
                <b>Version {e.v.version}{#if e.v.version === current}<i class="tag">Current</i>{/if}</b>
                <span class="num">{when(e.v.at)}{e.v.user ? ` · ${e.v.user}` : ''}</span>
                {#if e.v.note}<em>{e.v.note}</em>{/if}
                {#if e.summary}<small>{e.summary}</small>{:else if !e.from}<small>Where this recipe started</small>{/if}
              </span>
            </button>
          </li>
        {/each}
      </ol>

      {#if entry}
        {#key entry.v.version}
          <section class="detail" aria-label={`Version ${entry.v.version}`}>
            <div class="top">
              <div>
                <h2>Version {entry.v.version}</h2>
                <p class="muted small">
                  {#if entry.from}Compared with version {entry.from.version}, saved {timeAgo(entry.from.at)}.{:else}The first version Fond kept.{/if}
                  {#if entry.from && entry.from.yield !== entry.v.yield}<br />Makes <s>{entry.from.yield || 'not stated'}</s> → <b>{entry.v.yield || 'not stated'}</b>{/if}
                </p>
              </div>
              <div class="grow"></div>
              {#if entry.from}
                <div class="seg" role="group" aria-label="What to show">
                  <button class:on={!whole} onclick={() => (whole = false)}>Changes</button>
                  <button class:on={whole} onclick={() => (whole = true)}>Whole recipe</button>
                </div>
              {/if}
            </div>

            {#each [['Ingredients', entry.ingredients], ['Method', entry.steps]] as [title, rows]}
              {@const list = rows as Row[]}
              {@const numbers = stepNo(list)}
              {@const quiet = whole || !entry.from}
              <h3>{title}{#if !quiet && !list.some((x) => x.kind !== 'same')}<span class="still">unchanged</span>{/if}</h3>
              <ul class="lines" class:method={title === 'Method'}>
                {#each list as row, i}
                  {#if !(quiet && row.kind === 'del')}
                    <li class={quiet ? 'same' : row.kind} class:group={row.h}>
                      <span class="mark" aria-hidden="true">{quiet ? '' : row.kind === 'add' ? '+' : row.kind === 'del' ? '−' : row.kind === 'change' ? '~' : ''}</span>
                      {#if title === 'Method' && !row.h}<span class="n num">{numbers[i] || ''}</span>{/if}
                      <span class="t">
                        {#if row.kind === 'change' && row.segs && !quiet}
                          {#each row.segs as seg}{#if seg.t === 'add'}<ins>{seg.v}</ins>{:else if seg.t === 'del'}<del>{seg.v}</del>{:else}{seg.v}{/if}{/each}
                        {:else}{row.text}{/if}
                      </span>
                      <span class="sr">{quiet ? '' : row.kind === 'add' ? 'added' : row.kind === 'del' ? 'removed' : row.kind === 'change' ? 'changed' : ''}</span>
                    </li>
                  {/if}
                {:else}
                  <li class="same"><span class="mark"></span><span class="t faint">Nothing here.</span></li>
                {/each}
              </ul>
            {/each}

            {#if entry.v.version !== current}
              <div class="restore">
                <button class="btn primary" onclick={() => restore(entry.v)} disabled={busy}><RotateCcw />Restore version {entry.v.version}</button>
                <span class="hint">Brings these ingredients and this method back as version {current + 1}. Nothing is lost.</span>
              </div>
            {/if}
          </section>
        {/key}
      {/if}
    </div>
    {#if versions.length === 1}
      <p class="lonely"><History />This recipe hasn’t changed since it was saved. Edit the ingredients or the method and the earlier version will wait here.</p>
    {/if}
  </div>
{:else}
  <div class="page"><div class="skel"></div></div>
{/if}

<style>
  .history { width: min(1080px, 100%); padding-top: 10px; }
  .back { margin-left: -10px; max-width: 100%; overflow: hidden; text-overflow: ellipsis; justify-content: flex-start; }
  header { display: grid; gap: 6px; margin: 10px 0 26px; }
  header p { max-width: 62ch; }
  .layout { display: grid; grid-template-columns: 290px minmax(0, 1fr); gap: clamp(24px, 4vw, 52px); align-items: start; }

  .rail { position: sticky; top: 84px; max-height: calc(100dvh - 110px); overflow: auto; padding: 2px 4px 2px 0; scrollbar-width: thin; }
  .rail li { position: relative; }
  /* The line that joins one version to the next. */
  .rail li:not(:last-child)::before { content: ''; position: absolute; left: 18px; top: 30px; bottom: -6px; width: 2px; background: var(--line); }
  .stop { position: relative; display: flex; gap: 12px; width: 100%; padding: 10px 10px 12px 10px; border-radius: var(--r-m); text-align: left; transition: background 0.15s; }
  .stop:hover { background: var(--sunk); }
  .stop.on { background: var(--wash); }
  .dot { flex: none; width: 18px; height: 18px; margin-top: 2px; border-radius: 50%; background: var(--paper); box-shadow: inset 0 0 0 2px var(--line-2); transition: box-shadow 0.2s, background 0.2s, transform 0.25s var(--spring); z-index: 1; }
  .stop.on .dot { box-shadow: inset 0 0 0 5px var(--bay); transform: scale(1.08); }
  .dot.now { background: var(--butter); box-shadow: inset 0 0 0 2px color-mix(in srgb, var(--butter) 60%, #6b5a10); }
  .stop.on .dot.now { background: var(--butter); box-shadow: inset 0 0 0 2px var(--bay), 0 0 0 3px var(--wash); }
  .what { display: grid; gap: 1px; min-width: 0; }
  .what b { display: flex; align-items: center; gap: 8px; font: 600 1.06rem/1.3 var(--serif); }
  .tag { padding: 1px 8px; border-radius: 999px; background: var(--butter); color: #2a2508; font: 600 0.7rem/1.5 var(--sans); font-style: normal; letter-spacing: 0.02em; }
  .what span { font-size: 0.82rem; color: var(--ink-2); }
  .what em { font-size: 0.88rem; color: var(--ink); }
  .what small { font-size: 0.78rem; color: var(--ink-3); }

  .detail { min-width: 0; padding: clamp(18px, 3vw, 30px); border-radius: var(--r-l); background: var(--surface); box-shadow: inset 0 0 0 1px var(--line); animation: rise 0.25s var(--ease); }
  .top { display: flex; align-items: flex-start; flex-wrap: wrap; gap: 12px; margin-bottom: 6px; }
  h2 { font: 600 1.6rem/1.15 var(--serif); letter-spacing: -0.01em; }
  h3 { margin: 22px 0 8px; font: 600 1.15rem/1.2 var(--serif); }
  .still { margin-left: 10px; font: 500 0.82rem var(--sans); color: var(--ink-3); }
  /* Old-style figures make a 1 look like an I; versions are numbers. */
  h2, .what b, .n { font-variant-numeric: lining-nums; }
  .top s { color: var(--ink-3); }
  .lines { display: grid; gap: 2px; }
  .lines li { display: grid; grid-template-columns: 22px minmax(0, 1fr); align-items: baseline; padding: 6px 10px 6px 0; border-radius: 8px; line-height: 1.5; }
  .lines.method li:not(.group) { grid-template-columns: 22px 26px minmax(0, 1fr); padding-top: 8px; padding-bottom: 8px; }
  .mark { text-align: center; font-weight: 700; color: var(--ink-3); user-select: none; }
  .n { font: 600 0.95rem var(--serif); color: var(--ink-3); }
  .same .t { color: var(--ink-2); }
  .group .t { font: italic 600 1rem/1.4 var(--serif); color: var(--ink-2); }
  .add { background: var(--wash); }
  .add .mark, .add .n { color: var(--bay); }
  .add .t { color: var(--ink); font-weight: 500; }
  .del { background: var(--tomato-soft); }
  .del .mark { color: var(--tomato); }
  .del .t { color: var(--ink-2); text-decoration: line-through; text-decoration-color: color-mix(in srgb, var(--tomato) 60%, transparent); }
  .change { background: color-mix(in srgb, var(--butter-soft) 55%, transparent); }
  .change .mark { color: color-mix(in srgb, var(--butter) 55%, var(--ink)); }
  ins { text-decoration: none; padding: 1px 2px; border-radius: 4px; background: color-mix(in srgb, var(--bay) 18%, transparent); color: var(--ink); font-weight: 600; white-space: pre-wrap; }
  del { padding: 1px 2px; border-radius: 4px; background: var(--tomato-soft); color: var(--ink-3); text-decoration-color: color-mix(in srgb, var(--tomato) 70%, transparent); white-space: pre-wrap; }
  .restore { display: flex; align-items: center; flex-wrap: wrap; gap: 10px 14px; margin-top: 26px; padding-top: 20px; border-top: 1px solid var(--line); }
  .lonely { display: flex; align-items: center; gap: 10px; margin-top: 22px; padding: 14px 16px; border-radius: var(--r-m); background: var(--sunk); font-size: 0.92rem; color: var(--ink-2); }
  .lonely :global(svg) { flex: none; width: 18px; height: 18px; color: var(--bay); }
  .skel { height: 60dvh; border-radius: var(--r-l); background: var(--sunk); animation: pulse 1.4s ease-in-out infinite; }
  @keyframes pulse { 50% { opacity: 0.5; } }

  @media (max-width: 860px) {
    .layout { grid-template-columns: 1fr; gap: 16px; }
    /* On a phone the versions run across the top, newest first. */
    .rail { position: static; display: flex; gap: 8px; max-height: none; margin: 0 calc(-1 * clamp(16px, 4vw, 40px)); padding: 2px clamp(16px, 4vw, 40px) 6px; overflow-x: auto; scrollbar-width: none; scroll-snap-type: x proximity; }
    .rail li { flex: none; width: min(74vw, 270px); scroll-snap-align: start; }
    .rail li:not(:last-child)::before { display: none; }
    .stop { height: 100%; box-shadow: inset 0 0 0 1px var(--line); background: var(--surface); }
    .stop.on { box-shadow: inset 0 0 0 2px var(--bay); background: var(--surface); }
    .detail { padding: 16px 14px; }
    .top .seg { order: 3; }
  }
</style>
