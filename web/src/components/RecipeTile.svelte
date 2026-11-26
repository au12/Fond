<script lang="ts">
  import Photo from './Photo.svelte';
  import { put } from '../lib/api';
  import { oops, recipePath } from '../lib/state.svelte';
  import { Heart } from '../lib/icons';
  import { minutesLabel } from '../lib/steps';
  import type { Card } from '../lib/types';

  let { r, i = 0 }: { r: Card; i?: number } = $props();
  let popping = $state(false);

  async function fav(e: MouseEvent) {
    e.preventDefault();
    e.stopPropagation();
    r.fav = !r.fav;
    popping = r.fav;
    try {
      await put(`/recipes/${r.id}/favorite`, { on: r.fav });
    } catch (err) {
      r.fav = !r.fav;
      oops(err);
    }
  }
</script>

<a class="tile" href={recipePath(r)}>
  <div class="photo"><Photo image={r.image} color={r.color} title={r.title} tags={r.tags} eager={i < 8} /></div>
  <button class="fav" class:on={r.fav} class:pop={popping} onanimationend={() => (popping = false)} onclick={fav} aria-label={r.fav ? 'Remove from favorites' : 'Add to favorites'} aria-pressed={r.fav}><Heart /></button>
  <h3>{r.title}</h3>
  <div class="meta">
    {#if r.total}<span>{minutesLabel(r.total)}</span>{/if}
    {#if r.tags[0]}<span>{r.tags[0]}</span>{/if}
  </div>
</a>
