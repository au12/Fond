<script lang="ts">
  // Safe, tiny formatting for notes and descriptions: paragraphs, **bold**,
  // and clickable links. Never uses innerHTML.
  let { text }: { text: string } = $props();

  type Tok = { t: 'text' | 'bold' | 'link'; v: string };
  function inline(s: string): Tok[] {
    const out: Tok[] = [];
    const re = /\*\*(.+?)\*\*|(https?:\/\/[^\s<>()]+[^\s<>().,;:!?])/g;
    let pos = 0;
    for (const m of s.matchAll(re)) {
      if (m.index! > pos) out.push({ t: 'text', v: s.slice(pos, m.index) });
      out.push(m[1] ? { t: 'bold', v: m[1] } : { t: 'link', v: m[2] });
      pos = m.index! + m[0].length;
    }
    if (pos < s.length) out.push({ t: 'text', v: s.slice(pos) });
    return out;
  }
  const paras = $derived(text.split(/\n{2,}/).map((p) => p.trim()).filter(Boolean));
</script>

{#each paras as para}
  <p>
    {#each para.split('\n') as line, i}
      {#if i}<br />{/if}{#each inline(line) as tok}{#if tok.t === 'bold'}<strong>{tok.v}</strong>{:else if tok.t === 'link'}<a href={tok.v} target="_blank" rel="noopener noreferrer nofollow">{tok.v.replace(/^https?:\/\/(www\.)?/, '').slice(0, 48)}</a>{:else}{tok.v}{/if}{/each}
    {/each}
  </p>
{/each}

<style>
  p + p { margin-top: 0.8em; }
  a { text-decoration: underline; text-decoration-color: var(--line-2); text-underline-offset: 3px; }
  a:hover { text-decoration-color: currentColor; }
</style>
