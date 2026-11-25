<script lang="ts">
  import Sheet from './Sheet.svelte';
  import QR from './QR.svelte';
  import { put } from '../lib/api';
  import { app, oops, toast } from '../lib/state.svelte';
  import { Copy, ExternalLink, Share } from '../lib/icons';

  let { open = $bindable(false), kind, id, title, token = $bindable('') }: { open: boolean; kind: 'recipe' | 'cookbook'; id: number; title: string; token: string } = $props();
  let busy = $state(false);
  const url = $derived(token ? `${app.origin || location.origin}/${kind === 'recipe' ? 's' : 'c'}/${token}` : '');

  async function toggle() {
    busy = true;
    try {
      const res = await put(`/${kind === 'recipe' ? 'recipes' : 'cookbooks'}/${id}/share`, { on: !token });
      token = res.shareToken;
    } catch (e) {
      oops(e);
    } finally {
      busy = false;
    }
  }
  async function copy() {
    await navigator.clipboard.writeText(url);
    toast('Link copied');
  }
</script>

<Sheet bind:open title={`Share this ${kind}`}>
  <div class="stack">
    <div class="spread">
      <div>
        <div style:font-weight="600">Public link</div>
        <div class="hint">Anyone with the link can read {kind === 'recipe' ? 'this recipe' : 'this cookbook and its recipes'}, without an account. Turn it off to stop sharing.</div>
      </div>
      <button class="switch" role="switch" aria-checked={!!token} aria-label="Public link" disabled={busy} onclick={toggle}></button>
    </div>
    {#if url}
      <div class="link">
        <input class="input" readonly value={url} onfocus={(e) => e.currentTarget.select()} aria-label="Public link" />
      </div>
      <div class="row wrap">
        <button class="btn primary" onclick={copy}><Copy />Copy link</button>
        {#if navigator.share}<button class="btn" onclick={() => navigator.share({ title, url }).catch(() => {})}><Share />Send</button>{/if}
        <a class="btn quiet" href={url} target="_blank" rel="noopener"><ExternalLink />Preview</a>
      </div>
      <div class="qr"><QR value={url} /><span class="hint">Scan to open on a phone.</span></div>
    {/if}
  </div>
</Sheet>

<style>
  .qr { display: flex; align-items: center; gap: 16px; padding-top: 6px; }
  .qr :global(svg) { border-radius: 8px; box-shadow: inset 0 0 0 1px var(--line); }
</style>
