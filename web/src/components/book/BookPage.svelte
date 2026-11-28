<script lang="ts">
  // One page of a cookbook as a book. Every size is a fraction of the page's width (cqw), so a page
  // looks, and breaks, the same at any size.
  import Cover from '../Cover.svelte';
  import Rich from '../Rich.svelte';
  import Logo from '../Logo.svelte';
  import { img } from '../../lib/api';
  import { folio, type Page } from '../../lib/book';
  import { formatNumber, parseIngredient, show, type DisplayOptions } from '../../lib/ingredients';
  import { leatherUrl } from '../../lib/leather';
  import { minutesLabel, tokenize } from '../../lib/steps';
  import type { Cookbook } from '../../lib/types';
  import { dict, weighOptions } from '../../lib/weigh.svelte';

  let {
    page, index, book, opts, temp = 'original', side = 'right', onjump,
  }: {
    page: Page; index: number; book: Cookbook; opts: DisplayOptions; temp?: 'original' | 'C' | 'F'; side?: 'left' | 'right'; onjump?: (page: number) => void;
  } = $props();

  const look = $derived(book.style === 'modern' ? 'modern' : 'classic');
  const clean = (s: string) => s.replace(/\*\*|__/g, '');
  const tempMode = $derived(temp === 'original' && opts.system !== 'original' ? (opts.system === 'metric' ? 'C' : 'F') : temp);
  const ROMAN = ['', 'I', 'II', 'III', 'IV', 'V', 'VI', 'VII', 'VIII', 'IX', 'X', 'XI', 'XII', 'XIII', 'XIV', 'XV', 'XVI', 'XVII', 'XVIII', 'XIX', 'XX'];

  function makes(r: { yield: string }) {
    const y = r.yield;
    const m = y.match(/^(\d[\d\s.,/½¼¾–-]*(?:to\s+\d+\s*)?)\s*(servings?|people|persons?|portions?)?$/i);
    return !y ? '' : m ? `Serves ${m[1].trim()}` : /^(serves|makes|yields?)\b/i.test(y) ? y : `Makes ${y}`;
  }
  const facts = (r: { prepMin: number; cookMin: number; totalMin: number }) =>
    ([['Prep', minutesLabel(r.prepMin)], ['Cook', minutesLabel(r.cookMin)], ['Total', minutesLabel(r.totalMin)]] as const).filter((f) => f[1]);
  /** Step numbers through the whole recipe, so a continued method carries on counting. */
  const numbers = (steps: { h?: boolean }[]) => {
    let n = 0;
    return steps.map((s) => (s.h ? 0 : ++n));
  };
</script>

<div class={`bp ${side} ${page.kind} ${look}`} data-page={index}>
  {#if page.kind === 'cover'}
    <div class="full"><Cover title={book.title} subtitle={book.subtitle} color={book.color} leather={book.leather} large /></div>
  {:else if page.kind === 'back'}
    <div class={`full back cover ${book.color}`} style:--hide={book.leather ? `url(${leatherUrl(book.leather, book.color, true)})` : null}></div>
  {:else if page.kind === 'endpaper'}
    <div class={`full endpaper ep-${book.color}`}></div>
  {:else if page.kind === 'photo'}
    <div class="full photo"><img src={img(page.r.image)} alt="" style:background={page.r.imageColor} loading="lazy" decoding="async" /></div>
  {:else}
    <div class="in">
      {#if page.kind === 'title'}
        <div class="titlepage">
          <div class="orn" aria-hidden="true"></div>
          <h1>{book.title}</h1>
          {#if book.subtitle}<p class="sub">{book.subtitle}</p>{/if}
          <div class="orn" aria-hidden="true"></div>
          <div class="mark"><Logo size={22} /><span>Fond</span></div>
        </div>
      {:else if page.kind === 'intro'}
        <div class="intro">
          <p class="run">Introduction</p>
          <div class="introtext">{book.description}</div>
        </div>
      {:else if page.kind === 'toc'}
        <div class="toc">
          <h2>{page.part === 0 ? 'Contents' : 'Contents, continued'}</h2>
          <ol>
            {#each page.rows as row}
              {#if row.kind === 'chapter'}
                <li class="ch">{row.name}</li>
              {:else}
                <li><button onclick={(e) => { e.stopPropagation(); onjump?.(row.page + 1); }}><span class="t">{row.title}</span><span class="dots"></span><span class="pg num">{row.page}</span></button></li>
              {/if}
            {/each}
          </ol>
        </div>
      {:else if page.kind === 'chapterlist'}
        <div class="toc inch">
          <p class="run">In this chapter</p>
          <ol>
            {#each page.rows as row}
              {#if row.kind === 'recipe'}
                <li><button onclick={(e) => { e.stopPropagation(); onjump?.(row.page + 1); }}><span class="t">{row.title}</span><span class="dots"></span><span class="pg num">{row.page}</span></button></li>
              {/if}
            {/each}
          </ol>
        </div>
      {:else if page.kind === 'chapter'}
        <div class="chapter">
          {#if page.n}<p class="num-word">{ROMAN[page.n] ?? page.n}</p>{/if}
          <h2>{page.name}</h2>
          <div class="orn" aria-hidden="true"></div>
        </div>
      {:else if page.kind === 'recipe'}
        {@const r = page.r}
        {@const part = page.part}
        {@const o = (dict.rev, { ...opts, ...weighOptions(r, { flour: opts.flour ?? 'spoon' }) })}
        {@const nums = numbers(r.steps)}
        <article class={`rp fit-${part.fit}`} class:cont={!part.first}>
          <p class="run">{part.first ? page.chapter || book.title : `${r.title}, continued`}</p>
          {#if part.first}
            <header>
              <h1>{r.title}</h1>
              {#if r.description}<p class="desc">{r.description}</p>{/if}
            </header>
          {/if}
          <div class="body" class:single={!part.first}>
            {#if part.first}
              <section class="ing">
                {#if makes(r) || facts(r).length}
                  <dl class="facts">
                    {#if makes(r)}<div><dd>{makes(r)}</dd></div>{/if}
                    {#each facts(r) as [k, v]}<div><dt>{k}</dt><dd>{v}</dd></div>{/each}
                  </dl>
                {/if}
                <ul>
                  {#each r.ingredients as line}
                    {#if line.h}
                      <li class="grp">{line.text}</li>
                    {:else}
                      {@const s = show(parseIngredient(line.text), o)}
                      <li>{#if s.measure}<b>{s.estimated && s.approx ? '≈ ' : ''}{s.measure}</b>{' '}{/if}{s.rest}{#if s.also}{' '}<span class="also">({s.approx ? '≈ ' : ''}{s.also})</span>{/if}</li>
                    {/if}
                  {/each}
                </ul>
              </section>
            {/if}
            {#if part.to > part.from}
              <ol class="method">
                {#each r.steps.slice(part.from, part.to) as step, i}
                  {#if step.h}
                    <li class="grp">{step.text}</li>
                  {:else}
                    <li class="step"><span class="n">{nums[part.from + i]}</span><p>{#each tokenize(clean(step.text), tempMode) as tok}{tok.t === 'temp' ? tok.shown : tok.v}{/each}</p></li>
                  {/if}
                {/each}
              </ol>
            {/if}
          </div>
          {#if part.notes && r.notes}
            <aside class="notes"><h3>Notes</h3><div><Rich text={r.notes} /></div></aside>
          {/if}
        </article>
      {/if}
      {#if page.kind === 'recipe' || page.kind === 'toc' || page.kind === 'chapter' || page.kind === 'chapterlist' || page.kind === 'intro'}
        <footer class="folio num">{folio(index)}</footer>
      {/if}
    </div>
  {/if}
</div>

<style>
  .bp {
    --paper: #fbf8f1; --ink: #23201b; --soft: #6f6a60; --rule: #d9d2c3; --tint: #f1ebdf; --accent: #2c4a3a;
    position: relative; width: 100%; height: 100%; overflow: hidden; background: var(--paper); color: var(--ink);
    container-type: size; font-family: var(--serif); font-variant-numeric: oldstyle-nums proportional-nums;
    print-color-adjust: exact; -webkit-print-color-adjust: exact;
  }
  /* The fold: pages darken a little toward the spine, and the paper has a faint warmth at its edge. */
  .bp::after { content: ''; position: absolute; inset: 0; pointer-events: none; z-index: 3; }
  .bp.left::after { background: linear-gradient(to left, rgb(60 40 20 / 0.16), rgb(60 40 20 / 0.05) 5%, transparent 12%), linear-gradient(to right, rgb(120 90 50 / 0.06), transparent 3%); }
  .bp.right::after { background: linear-gradient(to right, rgb(60 40 20 / 0.18), rgb(60 40 20 / 0.05) 5%, transparent 12%), linear-gradient(to left, rgb(120 90 50 / 0.06), transparent 3%); }
  .bp.cover::after, .bp.back::after { display: none; }
  .bp.photo::after { background: linear-gradient(to left, rgb(0 0 0 / 0.28), transparent 9%); }
  .bp.photo.right::after { background: linear-gradient(to right, rgb(0 0 0 / 0.28), transparent 9%); }

  .full { position: absolute; inset: 0; }
  .full :global(.cover) { width: 100%; height: 100%; aspect-ratio: auto; border-radius: 0 1.2cqw 1.2cqw 0; font-size: 4cqw; }
  .back.cover { border-radius: 1.2cqw 0 0 1.2cqw; transform: scaleX(-1); }
  .photo img { width: 100%; height: 100%; object-fit: cover; }
  .endpaper { --ep: #c9bf9f; --ep2: #b5aa86; background-color: var(--ep); background-image:
    radial-gradient(circle at 25% 25%, var(--ep2) 0 6%, transparent 7%), radial-gradient(circle at 75% 75%, var(--ep2) 0 6%, transparent 7%),
    radial-gradient(circle at 75% 25%, color-mix(in srgb, var(--ep2) 50%, transparent) 0 2.5%, transparent 3.5%), radial-gradient(circle at 25% 75%, color-mix(in srgb, var(--ep2) 50%, transparent) 0 2.5%, transparent 3.5%);
    background-size: 9cqw 9cqw; }
  .ep-bay { --ep: #c3cbb6; --ep2: #a9b49b; } .ep-tomato { --ep: #dcc0b1; --ep2: #c9a592; } .ep-chestnut { --ep: #d8c3a6; --ep2: #c3a983; }
  .ep-ocean { --ep: #bfcbd0; --ep2: #a3b3ba; } .ep-plum { --ep: #cdbec8; --ep2: #b7a3b0; } .ep-charcoal { --ep: #c6c4bd; --ep2: #aeaba2; }
  .ep-butter { --ep: #e2d3a6; --ep2: #d0bd83; } .ep-paper { --ep: #ddd3bc; --ep2: #c9bc9d; }

  .in { position: absolute; inset: 0; padding: 10cqw 9cqw 9cqw; font-size: 2.7cqw; line-height: 1.55; display: flex; flex-direction: column; }
  .left .in { padding-left: 8cqw; padding-right: 10cqw; }
  .right .in { padding-left: 10cqw; padding-right: 8cqw; }
  .folio { position: absolute; bottom: 4.5cqw; left: 0; right: 0; text-align: center; font: 500 2.3cqw/1 var(--sans); letter-spacing: 0.12em; color: var(--soft); font-variant-numeric: lining-nums; }
  .run { font: 600 1.7cqw/1.4 var(--sans); letter-spacing: 0.3em; text-transform: uppercase; color: var(--soft); text-align: center; margin-bottom: 5cqw; }
  .orn { width: 14cqw; height: 1.4cqw; margin: 0 auto; background: radial-gradient(circle, var(--soft) 0 0.45cqw, transparent 0.55cqw) center / 1.4cqw 1.4cqw no-repeat, linear-gradient(var(--rule), var(--rule)) center / 100% 1px no-repeat; }

  .titlepage { margin: auto 0; display: grid; justify-items: center; gap: 5cqw; text-align: center; padding-bottom: 8cqw; }
  .titlepage h1 { font: 400 8cqw/1.15 var(--serif); letter-spacing: 0.06em; text-transform: uppercase; text-wrap: balance; font-variant-numeric: lining-nums; }
  .titlepage .sub { font-style: italic; font-size: 3.6cqw; color: var(--soft); text-wrap: balance; }
  .mark { position: absolute; bottom: 9cqw; left: 0; right: 0; display: flex; justify-content: center; align-items: center; gap: 1.5cqw; font: italic 600 3cqw/1 var(--serif); color: var(--accent); }
  .mark :global(svg) { width: 4.5cqw; height: 4.5cqw; }

  .intro { margin: auto 0; }
  .introtext { white-space: pre-line; text-align: justify; hyphens: auto; -webkit-hyphens: auto; font-size: 3cqw; line-height: 1.7; text-wrap: pretty; }
  .introtext::first-letter { float: left; font-size: 3.4em; line-height: 0.9; padding: 0.04em 0.08em 0 0; color: var(--accent); }

  .toc h2 { font: 400 4cqw/1.3 var(--serif); letter-spacing: 0.18em; text-transform: uppercase; text-align: center; margin-bottom: 6cqw; }
  .toc ol { display: grid; }
  .toc .ch { margin: 3.2cqw 0 1cqw; font: 600 1.75cqw/1.5 var(--sans); letter-spacing: 0.24em; text-transform: uppercase; color: var(--soft); }
  .toc .ch:first-child { margin-top: 0; }
  .toc button { display: flex; align-items: baseline; gap: 1.2cqw; width: 100%; padding: 0.55cqw 0; font: inherit; font-size: 2.9cqw; line-height: 1.45; text-align: left; color: inherit; border-radius: 0.6cqw; }
  .toc button:hover .t { color: var(--accent); text-decoration: underline; text-underline-offset: 0.4em; text-decoration-thickness: 1px; }
  .toc .dots { flex: 1; border-bottom: 1.5px dotted #b7ae9c; transform: translateY(-0.3em); min-width: 4cqw; }
  .toc .pg { font: 500 2.4cqw var(--sans); color: var(--soft); font-variant-numeric: lining-nums tabular-nums; }

  .inch { margin: auto 0; padding-bottom: 6cqw; }
  .inch button { font-size: 3.2cqw; padding: 1cqw 0; }
  .chapter { margin: auto 0; display: grid; justify-items: center; gap: 4cqw; text-align: center; padding-bottom: 10cqw; }
  .num-word { font: 500 3.2cqw/1 var(--serif); letter-spacing: 0.08em; color: var(--soft); }
  .chapter h2 { font: italic 400 9cqw/1.1 var(--serif); text-wrap: balance; }

  /* A recipe: the cookbook spread Fond prints, laid out for a page of its own. */
  .rp { --k: 1; display: flex; flex-direction: column; font-size: calc(1em * var(--k)); min-height: 0; }
  .rp.fit-1 { --k: 0.95; } .rp.fit-2 { --k: 0.9; } .rp.fit-3 { --k: 0.85; line-height: 1.48; }
  .rp header { padding-bottom: 1.3em; border-bottom: 1px solid var(--rule); }
  .rp h1 { font: 400 1.6em/1.28 var(--serif); letter-spacing: 0.15em; text-transform: uppercase; text-align: center; text-wrap: balance; font-variant-numeric: lining-nums; }
  .desc { margin-top: 0.9em; text-align: justify; hyphens: auto; -webkit-hyphens: auto; text-wrap: pretty; line-height: 1.6; white-space: pre-line; }
  .body { display: grid; grid-template-columns: minmax(0, 0.34fr) minmax(0, 0.66fr); gap: 2.3em; margin-top: 1.4em; align-items: start; }
  .body.single { grid-template-columns: 1fr; margin-top: 0; }
  .rp.cont .run { margin-bottom: 4cqw; }
  .facts { font: 600 0.64em/1.6 var(--sans); letter-spacing: 0.18em; text-transform: uppercase; color: var(--soft); font-variant-numeric: lining-nums; }
  .facts div { display: flex; gap: 0.6em; }
  .facts dt { opacity: 0.8; }
  .facts::after { content: ''; display: block; width: 1.8em; margin: 1.1em 0 1.2em; border-top: 1px solid currentColor; opacity: 0.6; }
  .ing ul { font: 400 0.84em/1.36 var(--sans); font-variant-numeric: lining-nums; }
  .ing li { padding: 0.22em 0 0.22em 0.9em; text-indent: -0.9em; }
  .ing b { font-weight: 600; }
  .also { color: var(--soft); white-space: nowrap; }
  .grp { margin-top: 1.3em; padding-bottom: 0.35em; font: 600 0.64em/1.5 var(--sans); letter-spacing: 0.18em; text-transform: uppercase; color: var(--soft); text-indent: 0 !important; padding-left: 0 !important; }
  .grp:first-child { margin-top: 0; }
  .method { display: grid; }
  .step { position: relative; padding: 0 0 0.85em 1.6em; }
  .step .n { position: absolute; left: 0; top: 0.42em; font: 600 0.68em/1.5 var(--sans); color: var(--soft); font-variant-numeric: lining-nums; }
  .step:last-child { padding-bottom: 0; }
  .notes { display: flex; gap: 1.1em; margin-top: 1.6em; padding: 1em 1.3em 1em 1em; background: var(--tint); font: 400 0.82em/1.5 var(--sans); }
  .notes h3 { flex: none; writing-mode: vertical-rl; transform: rotate(180deg); padding-left: 0.8em; border-left: 1px solid var(--rule); font: italic 400 1.1em/1 var(--serif); color: var(--soft); text-align: center; }
  .notes > div { align-self: center; min-width: 0; }

  /* Modern: sans headings, flush left, a tinted ingredient panel. */
  .modern .rp h1 { font: 800 2.1em/1.06 var(--sans); letter-spacing: -0.03em; text-transform: none; text-align: left; }
  .modern .rp header { border: 0; padding-bottom: 0; }
  .modern .run { text-align: left; font-weight: 700; }
  .modern .ing { padding: 1em 1.1em; border-radius: 1.6cqw; background: var(--tint); }
  .modern .step .n { color: var(--accent); font-size: 0.8em; font-weight: 800; top: 0.26em; }
  .modern .notes { border-radius: 1.6cqw; }
  .modern .titlepage h1 { font: 800 8.5cqw/1.04 var(--sans); letter-spacing: -0.03em; text-transform: none; }
  .modern .chapter h2 { font: 800 8cqw/1.05 var(--sans); font-style: normal; letter-spacing: -0.03em; }
</style>
