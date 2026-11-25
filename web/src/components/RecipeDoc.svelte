<script lang="ts">
  // The typeset recipe: used for public pages, printing and cookbooks.
  import QR from './QR.svelte';
  import Rich from './Rich.svelte';
  import { img } from '../lib/api';
  import { formatNumber, parseIngredient, show, type DisplayOptions } from '../lib/ingredients';
  import { minutesLabel, tokenize } from '../lib/steps';
  import type { Recipe } from '../lib/types';
  import { dict, weighOptions } from '../lib/weigh.svelte';

  let {
    r, look = 'classic', opts, temp = 'original', photo = true, notes = true, qr = '', number = 0, chapter = '',
  }: { r: Recipe; look?: string; opts: DisplayOptions; temp?: 'original' | 'C' | 'F'; photo?: boolean; notes?: boolean; qr?: string; number?: number; chapter?: string } = $props();

  const yieldText = $derived(
    !r.yield ? '' : opts.scale === 1 || !r.servings ? r.yield : r.yield.replace(/\d+(?:[.,]\d+)?/, formatNumber(r.servings * opts.scale, true)),
  );
  const facts = $derived(([['Prep', minutesLabel(r.prepMin)], ['Cook', minutesLabel(r.cookMin)], ['Total', minutesLabel(r.totalMin)], ['Makes', yieldText]] as const).filter((f) => f[1]));
  const stepNo = $derived.by(() => {
    let n = 0;
    return r.steps.map((s) => (s.h ? 0 : ++n));
  });
  const tempMode = $derived(temp === 'original' && opts.system !== 'original' ? (opts.system === 'metric' ? 'C' : 'F') : temp);
  const clean = (s: string) => s.replace(/\*\*|__/g, '');
  // Each recipe is weighed with its own corrections and its own salt.
  const o = $derived((dict.rev, { ...opts, ...weighOptions(r, { flour: opts.flour ?? 'spoon' }) }));
</script>

<article class={`doc ${look}`} id={number ? `recipe-${number}` : undefined}>
  <header class="doc-head">
    {#if number || chapter}<p class="doc-kicker">{#if number}<span class="doc-no">No. {number}</span>{/if}{#if chapter}<span>{chapter}</span>{/if}</p>{/if}
    <h1>{r.title}</h1>
    {#if r.description}<p class="doc-desc">{r.description}</p>{/if}
    {#if facts.length}
      <dl class="doc-facts">{#each facts as [k, v]}<div><dt>{k}</dt><dd>{v}</dd></div>{/each}</dl>
    {/if}
  </header>
  {#if photo && r.image}
    <figure class="doc-photo"><img src={img(r.image)} alt="" style:background={r.imageColor} /></figure>
  {/if}
  <div class="doc-body">
    <section class="doc-ing">
      <h2>Ingredients</h2>
      <ul>
        {#each r.ingredients as line}
          {#if line.h}
            <li class="doc-group">{line.text}</li>
          {:else}
            {@const s = show(parseIngredient(line.text), o)}
            <li>{#if s.measure}<span class="q">{s.estimated && s.approx ? '≈ ' : ''}{s.measure}</span>{' '}{/if}{s.rest}{#if s.also}<span class="also"> ({s.approx ? '≈ ' : ''}{s.also})</span>{/if}</li>
          {/if}
        {/each}
      </ul>
    </section>
    <section class="doc-method">
      <h2>Method</h2>
      <ol>
        {#each r.steps as step, i}
          {#if step.h}
            <li class="doc-group">{step.text}</li>
          {:else}
            <li class="doc-step"><span class="n">{stepNo[i]}</span><p>{#each tokenize(clean(step.text), tempMode) as tok}{tok.t === 'temp' ? tok.shown : tok.v}{/each}</p></li>
          {/if}
        {/each}
      </ol>
      {#if notes && r.notes}
        <section class="doc-notes"><h2>Notes</h2><Rich text={r.notes} /></section>
      {/if}
    </section>
  </div>
  {#if r.sourceUrl || r.author || qr}
    <footer class="doc-foot">
      <p>
        {#if r.author}<span>{r.author}</span>{/if}
        {#if r.sourceUrl}<a href={r.sourceUrl} target="_blank" rel="noopener noreferrer nofollow">{r.sourceName || r.sourceUrl.replace(/^https?:\/\/(www\.)?/, '').split('/')[0]}</a>{/if}
      </p>
      {#if qr}<div class="doc-qr"><QR value={qr} size={64} /></div>{/if}
    </footer>
  {/if}
</article>
