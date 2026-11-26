<script lang="ts">
  import { untrack } from 'svelte';
  import Logo from './Logo.svelte';
  import { leatherUrl } from '../lib/leather';

  // leather is the seed the book's hide is drawn from. large asks for the big drawing: a cover shown big, or printed.
  let { title, subtitle = '', color = 'bay', leather = 0, large = false }: { title: string; subtitle?: string; color?: string; leather?: number; large?: boolean } = $props();

  const wanted = $derived(leather ? leatherUrl(leather, color, large) : '');
  let shown = $state('');
  // A cover's first hide loads in place over the plain colour. A change of hide waits until the new one
  // has arrived, so the cover goes from one leather to the next and never blinks bare in between.
  $effect(() => {
    const url = wanted;
    if (!url || !untrack(() => shown)) {
      shown = url;
      return;
    }
    let current = true;
    const img = new Image();
    img.onload = img.onerror = () => {
      if (current) shown = url;
    };
    img.src = url;
    return () => {
      current = false;
    };
  });
</script>

<div class={`cover ${color}`} style:--hide={shown ? `url(${shown})` : null}>
  <b>{title}</b>
  {#if subtitle}<i>{subtitle}</i>{/if}
  <span style="--butter: currentColor; color: inherit"><Logo size={32} /></span>
</div>
