<script lang="ts">
  import { img } from '../lib/api';
  import { foodIcon, tint } from '../lib/icons';

  let { image, color = '', title = '', tags = [], thumb = true, eager = false }: { image: string; color?: string; title?: string; tags?: string[]; thumb?: boolean; eager?: boolean } = $props();
  let loaded = $state(false);
  const Icon = $derived(foodIcon(title, tags));
  const dark = $derived(document.documentElement.dataset.theme === 'dark');
</script>

{#if image}
  <img
    src={img(image, thumb)}
    alt=""
    loading={eager ? 'eager' : 'lazy'}
    decoding="async"
    style:background={color || 'var(--sunk)'}
    style:opacity={loaded ? 1 : 0.001}
    onload={() => (loaded = true)}
  />
  {#if !loaded}<div style:position="absolute" style:inset="0" style:background={color || 'var(--sunk)'} style:z-index="-1"></div>{/if}
{:else}
  <div class="blank" style:background={tint(title, dark)} style:color="var(--ink-2)"><Icon /></div>
{/if}
