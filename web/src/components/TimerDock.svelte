<script lang="ts">
  import { fly } from 'svelte/transition';
  import { flip } from 'svelte/animate';
  import { addMinute, left, removeTimer, timers, toggleTimer } from '../lib/timers.svelte';
  import { clock } from '../lib/steps';
  import { Bell, Pause, Play, X } from '../lib/icons';
</script>

{#if timers.list.length}
  <div class="dock" aria-label="Timers">
    {#each timers.list as t (t.id)}
      {@const remaining = left(t)}
      <div class="timer" class:done={t.done} animate:flip={{ duration: 220 }} in:fly={{ y: 30, duration: 260 }} out:fly={{ y: 20, duration: 180 }}>
        <div class="bar" style:transform={`scaleX(${t.done ? 1 : 1 - remaining / t.total})`}></div>
        <a href={t.recipePath} class="info">
          <span class="time num">{#if t.done}<Bell />Done{:else}{clock(remaining)}{/if}</span>
          <span class="what">{t.recipeTitle}{t.step >= 0 ? `, step ${t.step + 1}` : ''}</span>
        </a>
        <button class="mini" onclick={() => addMinute(t.id)} aria-label="Add one minute">+1</button>
        {#if !t.done}
          <button class="mini" onclick={() => toggleTimer(t.id)} aria-label={t.pausedLeft != null ? 'Resume' : 'Pause'}>{#if t.pausedLeft != null}<Play />{:else}<Pause />{/if}</button>
        {/if}
        <button class="mini" onclick={() => removeTimer(t.id)} aria-label="Dismiss timer"><X /></button>
      </div>
    {/each}
  </div>
{/if}

<style>
  .dock { position: fixed; z-index: 45; right: 16px; bottom: calc(var(--nav-h) + var(--safe-b) + 14px); display: grid; gap: 8px; width: min(340px, calc(100vw - 32px)); }
  .timer { position: relative; overflow: hidden; display: flex; align-items: center; gap: 4px; padding: 8px 8px 8px 16px; border-radius: 16px; background: var(--ink); color: var(--paper); box-shadow: var(--shadow-lg); }
  .bar { position: absolute; inset: 0; background: color-mix(in srgb, var(--butter) 26%, transparent); transform-origin: left; transition: transform 0.25s linear; pointer-events: none; }
  .info { position: relative; flex: 1; min-width: 0; display: grid; }
  .time { display: flex; align-items: center; gap: 6px; font-size: 1.25rem; font-weight: 700; line-height: 1.2; }
  .time :global(svg) { width: 18px; height: 18px; }
  .what { font-size: 0.78rem; opacity: 0.72; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .mini { position: relative; width: 36px; height: 36px; border-radius: 50%; display: grid; place-items: center; font-size: 0.8rem; font-weight: 700; color: inherit; opacity: 0.85; }
  .mini:hover { background: rgb(255 255 255 / 0.14); opacity: 1; }
  .mini :global(svg) { width: 16px; height: 16px; }
  .done { background: var(--tomato); color: #fff; animation: ring 0.6s ease 3; }
  .done .bar { display: none; }
  @keyframes ring { 20% { transform: rotate(-1.5deg) scale(1.02); } 60% { transform: rotate(1.5deg) scale(1.02); } }
  @media print { .dock { display: none; } }
</style>
