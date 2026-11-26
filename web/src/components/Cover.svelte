<script lang="ts">
  import Logo from './Logo.svelte';
  // Each book shows its own patch of wear, picked by `seed` (the book's id), so a shelf is not a row of twins.
  let { title, subtitle = '', color = 'bay', seed = 0 }: { title: string; subtitle?: string; color?: string; seed?: number } = $props();
  const at = $derived([(seed * 37) % 101, (seed * 59 + 23) % 101]);
</script>

<div class={`cover ${color}`} style:--wx={`${at[0]}%`} style:--wy={`${at[1]}%`}>
  <b>{title}</b>
  {#if subtitle}<i>{subtitle}</i>{/if}
  <span style="--butter: currentColor; color: inherit"><Logo size={32} /></span>
</div>
