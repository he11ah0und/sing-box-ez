<script lang="ts">
  import { onMount, onDestroy } from 'svelte';

  let {
    data = [],
    times = [],
    span = 60,
    color = 'var(--color-primary)',
    height = 80,
    fill = false
  }: {
    data?: number[];
    // times holds per-sample timestamps (ms), aligned with data.
    times?: number[];
    // span is the visible time window in seconds.
    span?: number;
    color?: string;
    height?: number;
    fill?: boolean;
  } = $props();

  // Fixed internal coordinate space; the SVG stretches to the container.
  const W = 300;

  // now ticks every animation frame so points drift left smoothly between
  // data updates. Only real samples are drawn — points enter at the right
  // edge and slide left, there is no synthetic seed point.
  let now = $state(Date.now());
  let raf: number | null = null;

  function frame() {
    now = Date.now();
    raf = requestAnimationFrame(frame);
  }

  onMount(() => {
    raf = requestAnimationFrame(frame);
  });
  onDestroy(() => {
    if (raf !== null) cancelAnimationFrame(raf);
  });

  function buildPath(values: number[], ts: number[], h: number, nowMs: number, close: boolean): string {
    if (values.length < 2 || values.length !== ts.length) return '';
    const spanMs = Math.max(1, span) * 1000;
    const visible: { x: number; y: number }[] = [];
    let max = 1;
    for (let i = 0; i < values.length; i++) {
      if (nowMs - ts[i] <= spanMs && values[i] > max) max = values[i];
    }
    for (let i = 0; i < values.length; i++) {
      const age = nowMs - ts[i];
      if (age < 0 || age > spanMs) continue;
      visible.push({ x: W - (age / spanMs) * W, y: h - (values[i] / max) * h });
    }
    if (visible.length < 2) return '';
    let d = '';
    visible.forEach((p, i) => {
      d += `${i === 0 ? 'M' : 'L'} ${p.x.toFixed(1)} ${p.y.toFixed(1)}`;
    });
    if (close) {
      const first = visible[0];
      const last = visible[visible.length - 1];
      d += ` L ${last.x.toFixed(1)} ${h} L ${first.x.toFixed(1)} ${h} Z`;
    }
    return d;
  }

  const linePath = $derived(buildPath(data, times, height, now, false));
  const areaPath = $derived(fill ? buildPath(data, times, height, now, true) : '');
</script>

<svg class="w-full" style="height: {height}px" preserveAspectRatio="none" viewBox="0 0 {W} {height}">
  {#if fill && areaPath}
    <path d={areaPath} fill={color} opacity="0.15" />
  {/if}
  {#if linePath}
    <path d={linePath} fill="none" stroke={color} stroke-width="2" vector-effect="non-scaling-stroke" />
  {/if}
</svg>
