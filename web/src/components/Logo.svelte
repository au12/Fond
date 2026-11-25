<script lang="ts">
  let { size = 28 }: { size?: number } = $props();
</script>

<!-- A skillet seen from above, with a pat of butter melting in it. -->
<svg width={size} height={size} viewBox="0 0 32 32" fill="none" aria-hidden="true">
  <path d="M21.5 13.5h8" stroke="currentColor" stroke-width="3.6" stroke-linecap="round" />
  <circle cx="12.5" cy="13.5" r="10" stroke="currentColor" stroke-width="2.4" />
  <rect x="8.7" y="9.9" width="7.6" height="7.2" rx="1.6" transform="rotate(-14 12.5 13.5)" fill="var(--butter)" />
</svg>
