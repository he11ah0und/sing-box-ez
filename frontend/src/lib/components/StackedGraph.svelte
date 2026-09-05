<script lang="ts">
  import { onMount, onDestroy } from 'svelte';

  let {
    up = [],
    down = [],
    times = [],
    span = 60,
    upColor = 'var(--color-success)',
    downColor = 'var(--color-primary)',
    height = 80
  }: {
    up?: number[];
    down?: number[];
    // times holds per-sample timestamps (ms), aligned with up/down.
    times?: number[];
    // span is the visible time window in seconds.
    span?: number;
    upColor?: string;
    downColor?: string;
    height?: number;
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

  interface StackedPaths {
    downArea: string;
    downLine: string;
    upArea: string;
    upLine: string;
  }

  // buildPaths draws upload and download as independent overlapping series;
  // the scale is the max of either series over the visible window. (An
  // earlier stacked layout plotted up on top of down, making the compact
  // graph disagree with the per-series detail view.)
  function buildPaths(
    upVals: number[],
    downVals: number[],
    ts: number[],
    h: number,
    nowMs: number
  ): StackedPaths {
    const empty = { downArea: '', downLine: '', upArea: '', upLine: '' };
    const n = Math.min(upVals.length, downVals.length, ts.length);
    if (n < 2) return empty;
    const spanMs = Math.max(1, span) * 1000;
    let max = 1;
    for (let i = 0; i < n; i++) {
      const age = nowMs - ts[i];
      if (age >= 0 && age <= spanMs) {
        if ((downVals[i] || 0) > max) max = downVals[i] || 0;
        if ((upVals[i] || 0) > max) max = upVals[i] || 0;
      }
    }
    const pts: { x: number; yDown: number; yUp: number }[] = [];
    for (let i = 0; i < n; i++) {
      const age = nowMs - ts[i];
      if (age < 0 || age > spanMs) continue;
      const x = W - (age / spanMs) * W;
      const yDown = h - ((downVals[i] || 0) / max) * h;
      const yUp = h - ((upVals[i] || 0) / max) * h;
      pts.push({ x, yDown, yUp });
    }
    if (pts.length < 2) return empty;

    const line = (y: (p: (typeof pts)[number]) => number): string =>
      pts.map((p, i) => `${i === 0 ? 'M' : 'L'} ${p.x.toFixed(1)} ${y(p).toFixed(1)}`).join(' ');

    const downLine = line((p) => p.yDown);
    const upLine = line((p) => p.yUp);
    const first = pts[0];
    const last = pts[pts.length - 1];
    const downArea =
      `${downLine} L ${last.x.toFixed(1)} ${h} L ${first.x.toFixed(1)} ${h} Z`;
    const upArea =
      `${upLine} L ${last.x.toFixed(1)} ${h} L ${first.x.toFixed(1)} ${h} Z`;
    return { downArea, downLine, upArea, upLine };
  }

  const paths = $derived(buildPaths(up, down, times, height, now));
</script>

<svg class="w-full" style="height: {height}px" preserveAspectRatio="none" viewBox="0 0 {W} {height}">
  {#if paths.downArea}
    <path d={paths.downArea} fill={downColor} opacity="0.35" />
  {/if}
  {#if paths.upArea}
    <path d={paths.upArea} fill={upColor} opacity="0.35" />
  {/if}
  {#if paths.downLine}
    <path d={paths.downLine} fill="none" stroke={downColor} stroke-width="1.5" vector-effect="non-scaling-stroke" />
  {/if}
  {#if paths.upLine}
    <path d={paths.upLine} fill="none" stroke={upColor} stroke-width="1.5" vector-effect="non-scaling-stroke" />
  {/if}
</svg>
