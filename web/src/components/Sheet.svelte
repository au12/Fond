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

  // On a phone the keyboard covers the bottom of the screen without changing 100dvh, which left a sheet
  // behind the keyboard with its top out of sight. The scrim follows the part of the screen still visible.
  let vvH = $state(0);
  let vvTop = $state(0);
  $effect(() => {
    const vv = window.visualViewport;
    if (!open || !vv) return;
    const sync = () => {
      const covered = window.innerHeight - vv.height > 80;
      vvH = covered ? vv.height : 0;
      vvTop = covered ? vv.offsetTop : 0;
    };
    sync();
    vv.addEventListener('resize', sync);
    vv.addEventListener('scroll', sync);
    return () => {
      vv.removeEventListener('resize', sync);
      vv.removeEventListener('scroll', sync);
      vvH = vvTop = 0;
    };
  });
</script>

<svelte:window onkeydown={key} />

{#if open}
  <div class="scrim" transition:fade={{ duration: 160 }} onclick={(e) => e.target === e.currentTarget && close()} role="presentation"
    style:height={vvH ? `${vvH}px` : null} style:top={vvH ? `${vvTop}px` : null} style:bottom={vvH ? 'auto' : null}>
    <div class="sheet" class:wide style:max-height={vvH ? `${Math.round(vvH * 0.94)}px` : null} role="dialog" aria-modal="true" aria-label={title} transition:fly={{ y: 24, duration: 260, easing: cubicOut }}>
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
