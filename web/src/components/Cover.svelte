<script lang="ts">
  import Logo from './Logo.svelte';
  let { title, subtitle = '', color = 'bay' }: { title: string; subtitle?: string; color?: string } = $props();
</script>

<div class={`cover ${color}`}>
  <b>{title}</b>
  {#if subtitle}<i>{subtitle}</i>{/if}
  <span style="--butter: currentColor; color: inherit"><Logo size={32} /></span>
</div>
