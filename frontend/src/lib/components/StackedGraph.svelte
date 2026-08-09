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

  // buildPaths stacks the upload area on top of the download area; the scale
  // is the max of up+down over the visible window.
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
        const total = (downVals[i] || 0) + (upVals[i] || 0);
        if (total > max) max = total;
      }
    }
    const pts: { x: number; yDown: number; yTotal: number }[] = [];
    for (let i = 0; i < n; i++) {
      const age = nowMs - ts[i];
      if (age < 0 || age > spanMs) continue;
      const x = W - (age / spanMs) * W;
      const yDown = h - ((downVals[i] || 0) / max) * h;
      const yTotal = h - (((downVals[i] || 0) + (upVals[i] || 0)) / max) * h;
      pts.push({ x, yDown, yTotal });
    }
    if (pts.length < 2) return empty;

    const line = (y: (p: (typeof pts)[number]) => number): string =>
      pts.map((p, i) => `${i === 0 ? 'M' : 'L'} ${p.x.toFixed(1)} ${y(p).toFixed(1)}`).join(' ');

    const downLine = line((p) => p.yDown);
    const upLine = line((p) => p.yTotal);
    const first = pts[0];
    const last = pts[pts.length - 1];
    const downArea =
      `${downLine} L ${last.x.toFixed(1)} ${h} L ${first.x.toFixed(1)} ${h} Z`;
    // The upload band closes back along the download top edge in reverse.
    const back = pts
      .slice()
      .reverse()
      .map((p) => `L ${p.x.toFixed(1)} ${p.yDown.toFixed(1)}`)
      .join(' ');
    const upArea = `${upLine} ${back} Z`;
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
