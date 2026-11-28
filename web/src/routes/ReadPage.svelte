<script lang="ts">
  import BookViewer from '../components/book/BookViewer.svelte';
  import { get } from '../lib/api';
  import { go, oops, prefs } from '../lib/state.svelte';
  import type { Cookbook, Recipe } from '../lib/types';

  let { id }: { id: number } = $props();
  const pf = prefs();
  let book = $state<Cookbook | null>(null);
  let recipes = $state<Recipe[]>([]);
  get<{ cookbook: Cookbook; recipes: Recipe[] }>(`/cookbooks/${id}?full=1`)
    .then((x) => { book = x.cookbook; recipes = x.recipes; document.title = `${x.cookbook.title} · Fond`; })
    .catch(oops);
  const close = () => (history.state?.k ? history.back() : go(`/books/${id}`, { replace: true }));
</script>

{#if book}
  <BookViewer {book} {recipes} system={pf.system} weigh={pf.weigh} fractions={pf.fractions} flour={pf.flour} temp={pf.temp} canOpen onclose={close} onprint={() => go(`/books/${id}/print`)} />
{:else}
  <div style="position:fixed;inset:0;background:#241d18"></div>
{/if}
