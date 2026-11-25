<script lang="ts">
  let { value, size = 132 }: { value: string; size?: number } = $props();
  let path = $state('');
  let n = $state(0);

  $effect(() => {
    const v = value;
    import('qrcode-generator').then(({ default: qrcode }) => {
      const qr = qrcode(0, 'M');
      qr.addData(v);
      qr.make();
      n = qr.getModuleCount();
      let d = '';
      for (let r = 0; r < n; r++) for (let c = 0; c < n; c++) if (qr.isDark(r, c)) d += `M${c} ${r}h1v1h-1z`;
      path = d;
    });
  });
</script>

{#if path}
  <svg class="qr" width={size} height={size} viewBox={`-2 -2 ${n + 4} ${n + 4}`} shape-rendering="crispEdges" role="img" aria-label="QR code">
    <rect x="-2" y="-2" width={n + 4} height={n + 4} fill="#fff" />
    <path d={path} fill="#111" />
  </svg>
{/if}
