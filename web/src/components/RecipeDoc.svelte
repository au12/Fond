<script lang="ts">
  // The typeset recipe: used for public pages, printing and cookbooks.
  import QR from './QR.svelte';
  import Rich from './Rich.svelte';
  import { img } from '../lib/api';
  import { formatNumber, parseIngredient, show, type DisplayOptions } from '../lib/ingredients';
  import { minutesLabel, tokenize } from '../lib/steps';
  import type { Recipe } from '../lib/types';
  import { dict, weighOptions } from '../lib/weigh.svelte';

  // spread: the photo is a full page of its own, facing the recipe. paged: laid out for paper.
  // fit: 0 (roomy) to 3 (tight), chosen by the print page so a recipe stays on one sheet when it can.
  let {
    r, look = 'classic', opts, temp = 'original', photo = true, notes = true, qr = '', number = 0, chapter = '', spread = false, paged = false, fit = 0, measure = false,
  }: {
    r: Recipe; look?: string; opts: DisplayOptions; temp?: 'original' | 'C' | 'F'; photo?: boolean; notes?: boolean; qr?: string; number?: number; chapter?: string;
    spread?: boolean; paged?: boolean; fit?: number; measure?: boolean;
  } = $props();

  const yieldText = $derived(
    !r.yield ? '' : opts.scale === 1 || !r.servings ? r.yield : r.yield.replace(/\d+(?:[.,]\d+)?/, formatNumber(r.servings * opts.scale, true)),
  );
  const facts = $derived(([['Prep', minutesLabel(r.prepMin)], ['Cook', minutesLabel(r.cookMin)], ['Total', minutesLabel(r.totalMin)]] as const).filter((f) => f[1]));
  // "4 servings" reads better as "Serves 4"; anything else is what the recipe makes.
  const makes = $derived.by(() => {
    const m = yieldText.match(/^(\d[\d\s.,/½¼¾–-]*(?:to\s+\d+\s*)?)\s*(servings?|people|persons?|portions?)?$/i);
    return !yieldText ? '' : m ? `Serves ${m[1].trim()}` : /^(serves|makes|yields?)\b/i.test(yieldText) ? yieldText : `Makes ${yieldText}`;
  });
  const hasPhoto = $derived(photo && !!r.image);
  const stepNo = $derived.by(() => {
    let n = 0;
    return r.steps.map((s) => (s.h ? 0 : ++n));
  });
  const tempMode = $derived(temp === 'original' && opts.system !== 'original' ? (opts.system === 'metric' ? 'C' : 'F') : temp);
  const clean = (s: string) => s.replace(/\*\*|__/g, '');
  // Each recipe is weighed with its own corrections and its own salt.
  const o = $derived((dict.rev, { ...opts, ...weighOptions(r, { flour: opts.flour ?? 'spoon' }) }));
</script>

<article class={`doc ${look} fit-${fit}`} class:facing={spread} class:paged class:measure class:has-photo={hasPhoto} id={number ? `recipe-${number}` : undefined}>
  {#if hasPhoto}
    <figure class="doc-plate"><img src={img(r.image)} alt="" style:background={r.imageColor} /></figure>
  {/if}
  <div class="doc-page">
    {#if chapter}<p class="doc-run">{chapter}</p>{/if}
    <header class="doc-head">
      <h1>{r.title}</h1>
      {#if r.description}<p class="doc-desc">{r.description}</p>{/if}
    </header>
    <div class="doc-body">
      <section class="doc-ing">
        {#if makes || facts.length}
          <dl class="doc-facts">
            {#if makes}<div class="makes"><dd>{makes}</dd></div>{/if}
            {#each facts as [k, v]}<div><dt>{k}</dt><dd>{v}</dd></div>{/each}
          </dl>
        {/if}
        <h2 class="sr">Ingredients</h2>
        <ul>
          {#each r.ingredients as line}
            {#if line.h}
              <li class="doc-group">{line.text}</li>
            {:else}
              {@const s = show(parseIngredient(line.text), o)}
              <li>{#if s.measure}<span class="q">{s.estimated && s.approx ? '≈ ' : ''}{s.measure}</span>{' '}{/if}{s.rest}{#if s.also}{' '}<span class="also">({s.approx ? '≈ ' : ''}{s.also})</span>{/if}</li>
            {/if}
          {/each}
        </ul>
      </section>
      <section class="doc-method">
        <h2 class="sr">Method</h2>
        <ol>
          {#each r.steps as step, i}
            {#if step.h}
              <li class="doc-group">{step.text}</li>
            {:else}
              <li class="doc-step"><span class="n">{stepNo[i]}</span><p>{#each tokenize(clean(step.text), tempMode) as tok}{tok.t === 'temp' ? tok.shown : tok.v}{/each}</p></li>
            {/if}
          {/each}
        </ol>
      </section>
    </div>
    {#if notes && r.notes}
      <aside class="doc-notes"><h2>Notes</h2><div><Rich text={r.notes} /></div></aside>
    {/if}
    {#if r.sourceUrl || r.author || qr || number}
      <footer class="doc-foot">
        <p>
          {#if r.author}<span>{r.author}</span>{/if}
          {#if r.sourceUrl}<a href={r.sourceUrl} target="_blank" rel="noopener noreferrer nofollow">{r.sourceName || r.sourceUrl.replace(/^https?:\/\/(www\.)?/, '').split('/')[0]}</a>{/if}
        </p>
        <span class="doc-folio">{number || ''}</span>
        <div class="doc-qr">{#if qr}<QR value={qr} size={64} />{/if}</div>
      </footer>
    {/if}
  </div>
</article>
