<script lang="ts">
  import { app } from '../lib/state.svelte';
  import { X } from '../lib/icons';

  let { tags = $bindable([]) }: { tags: string[] } = $props();
  let draft = $state('');
  let focused = $state(false);

  const known = $derived.by(() => {
    const counts = new Map<string, number>();
    for (const r of app.recipes) for (const t of r.tags) counts.set(t, (counts.get(t) ?? 0) + 1);
    return [...counts].sort((a, b) => b[1] - a[1]).map((x) => x[0]);
  });
  const suggestions = $derived(
    known.filter((t) => !tags.some((x) => x.toLowerCase() === t.toLowerCase()) && t.toLowerCase().includes(draft.trim().toLowerCase())).slice(0, 8),
  );

  function add(t: string) {
    t = t.trim().replace(/,+$/, '');
    if (t && !tags.some((x) => x.toLowerCase() === t.toLowerCase())) tags = [...tags, t];
    draft = '';
  }
  function key(e: KeyboardEvent) {
    if (e.key === 'Enter' || e.key === ',') {
      e.preventDefault();
      add(draft);
    } else if (e.key === 'Backspace' && !draft && tags.length) tags = tags.slice(0, -1);
  }
</script>

<div class="tags input" class:focused>
  {#each tags as t}
    <span class="tag">{t}<button type="button" onclick={() => (tags = tags.filter((x) => x !== t))} aria-label={`Remove ${t}`}><X /></button></span>
  {/each}
  <input bind:value={draft} onkeydown={key} onfocus={() => (focused = true)} onblur={() => { setTimeout(() => (focused = false), 150); add(draft); }} placeholder={tags.length ? '' : 'Dinner, Italian, Weeknight…'} aria-label="Add a tag" />
</div>
{#if focused && suggestions.length}
  <div class="sugg">{#each suggestions as s}<button type="button" class="chip" onmousedown={(e) => { e.preventDefault(); add(s); }}>{s}</button>{/each}</div>
{/if}

<style>
  .tags { display: flex; flex-wrap: wrap; gap: 6px; padding: 7px 10px; cursor: text; }
  .tags.focused { border-color: var(--bay); box-shadow: 0 0 0 3px color-mix(in srgb, var(--bay) 18%, transparent); }
  .tag { display: inline-flex; align-items: center; gap: 2px; height: 28px; padding: 0 4px 0 11px; border-radius: 999px; background: var(--wash); font-size: 0.875rem; font-weight: 600; color: var(--bay); }
  .tag button { width: 22px; height: 22px; display: grid; place-items: center; border-radius: 50%; color: inherit; opacity: 0.7; }
  .tag button:hover { opacity: 1; background: color-mix(in srgb, var(--bay) 14%, transparent); }
  .tag :global(svg) { width: 13px; height: 13px; }
  input { flex: 1; min-width: 120px; height: 28px; border: 0; outline: 0; background: none; }
  .sugg { display: flex; flex-wrap: wrap; gap: 6px; margin-top: 8px; }
  .sugg .chip { height: 28px; font-size: 0.8rem; }
</style>
