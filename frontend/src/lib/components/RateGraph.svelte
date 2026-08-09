<script lang="ts">
  import { onMount, onDestroy } from 'svelte';

  export interface RateMarker {
    // value is the rate (bytes/s) the horizontal line is drawn at.
    value: number;
    color: string;
    // dash is the SVG stroke-dasharray; empty means a solid line.
    dash?: string;
  }

  let {
    data = [],
    times = [],
    span = 60,
    color = 'var(--color-primary)',
    height = 120,
    markers = []
  }: {
    data?: number[];
    // times holds per-sample timestamps (ms), aligned with data.
    times?: number[];
    // span is the visible time window in seconds.
    span?: number;
    color?: string;
    height?: number;
    // markers are horizontal reference lines (min/max/median/p95) drawn
    // across the graph at their rate value.
    markers?: RateMarker[];
  } = $props();

  // Fixed internal coordinate space; the SVG stretches to the container.
  const W = 300;

  // now ticks every animation frame so points drift left smoothly between
  // data updates, same as Sparkline/StackedGraph.
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

  // scaleMax covers both the visible samples and the marker lines.
  function computeMax(values: number[], ts: number[], nowMs: number): number {
    const spanMs = Math.max(1, span) * 1000;
    let max = 1;
    for (let i = 0; i < values.length; i++) {
      if (nowMs - ts[i] <= spanMs && values[i] > max) max = values[i];
    }
    for (const m of markers) {
      if (m.value > max) max = m.value;
    }
    return max;
  }

  function buildPath(values: number[], ts: number[], h: number, max: number, nowMs: number, close: boolean): string {
    if (values.length < 2 || values.length !== ts.length) return '';
    const spanMs = Math.max(1, span) * 1000;
    const visible: { x: number; y: number }[] = [];
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

  const scaleMax = $derived(computeMax(data, times, now));
  const linePath = $derived(buildPath(data, times, height, scaleMax, now, false));
  const areaPath = $derived(buildPath(data, times, height, scaleMax, now, true));
  // markerY positions each reference line; lines sit 1px above the bottom at
  // a zero value so they stay visible.
  const markerLines = $derived(
    markers
      .filter((m) => m.value >= 0)
      .map((m) => ({ ...m, y: Math.max(1, height - (m.value / scaleMax) * height) }))
  );
</script>

<svg class="w-full" style="height: {height}px" preserveAspectRatio="none" viewBox="0 0 {W} {height}">
  {#if areaPath}
    <path d={areaPath} fill={color} opacity="0.15" />
  {/if}
  {#if linePath}
    <path d={linePath} fill="none" stroke={color} stroke-width="2" vector-effect="non-scaling-stroke" />
  {/if}
  {#each markerLines as m (m.color + m.value)}
    <line
      x1="0"
      y1={m.y}
      x2={W}
      y2={m.y}
      stroke={m.color}
      stroke-width="1"
      stroke-dasharray={m.dash ?? ''}
      vector-effect="non-scaling-stroke"
    />
  {/each}
</svg>
