<script lang="ts">
  import type { Snippet } from 'svelte';
  import { fade, fly } from 'svelte/transition';
  import { cubicOut } from 'svelte/easing';
  import { X } from '../lib/icons';

  let { open = $bindable(false), title = '', wide = false, children, onclose }: { open: boolean; title?: string; wide?: boolean; children: Snippet; onclose?: () => void } = $props();

  function close() {
    open = false;
    onclose?.();
  }
  function key(e: KeyboardEvent) {
    if (e.key === 'Escape' && open) close();
  }
</script>

<svelte:window onkeydown={key} />

{#if open}
  <div class="scrim" transition:fade={{ duration: 160 }} onclick={(e) => e.target === e.currentTarget && close()} role="presentation">
    <div class="sheet" class:wide role="dialog" aria-modal="true" aria-label={title} transition:fly={{ y: 24, duration: 260, easing: cubicOut }}>
      {#if title}
        <div class="spread" style:margin-bottom="16px">
          <h2>{title}</h2>
          <button class="icon-btn" onclick={close} aria-label="Close"><X /></button>
        </div>
      {/if}
      {@render children()}
    </div>
  </div>
{/if}
