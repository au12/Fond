<script lang="ts">
  // A small simmering pot: shown while Fond reads a page.
  let { done = false }: { done?: boolean } = $props();
</script>

<svg class="pot" class:done viewBox="0 0 120 110" width="120" height="110" aria-hidden="true">
  <g class="steam" stroke="var(--ink-3)" stroke-width="3" stroke-linecap="round" fill="none">
    <path d="M44 34c-5-6 5-10 0-17" />
    <path d="M60 30c-5-6 5-10 0-17" />
    <path d="M76 34c-5-6 5-10 0-17" />
  </g>
  <g class="lid">
    <rect x="52" y="38" width="16" height="6" rx="3" fill="var(--bay)" />
    <path d="M28 52c0-7 14-10 32-10s32 3 32 10z" fill="var(--bay)" />
  </g>
  <path d="M26 56h68v24a16 16 0 0 1-16 16H42a16 16 0 0 1-16-16z" fill="var(--bay)" />
  <path d="M26 62h-9a5 5 0 0 0 0 10h9M94 62h9a5 5 0 0 1 0 10h-9" stroke="var(--bay)" stroke-width="5" fill="none" stroke-linecap="round" />
  <path class="tick" d="M48 76l9 9 16-18" stroke="var(--butter)" stroke-width="6" fill="none" stroke-linecap="round" stroke-linejoin="round" />
</svg>

<style>
  .lid { transform-origin: 60px 52px; animation: rattle 0.9s ease-in-out infinite; }
  .steam path { stroke-dasharray: 26; animation: steam 1.8s linear infinite; }
  .steam path:nth-child(2) { animation-delay: 0.3s; }
  .steam path:nth-child(3) { animation-delay: 0.6s; }
  .tick { stroke-dasharray: 40; stroke-dashoffset: 40; }
  .done .lid, .done .steam path { animation: none; }
  .done .steam { opacity: 0; transition: opacity 0.3s; }
  .done .tick { stroke-dashoffset: 0; transition: stroke-dashoffset 0.4s 0.1s cubic-bezier(0.2, 0.8, 0.2, 1); }
  @keyframes rattle { 0%, 100% { transform: rotate(0) translateY(0); } 25% { transform: rotate(-3deg) translateY(-2px); } 75% { transform: rotate(3deg) translateY(-1px); } }
  @keyframes steam { 0% { stroke-dashoffset: 26; opacity: 0; } 30% { opacity: 1; } 100% { stroke-dashoffset: -26; opacity: 0; } }
</style>
