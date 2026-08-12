<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import RateGraph, { type RateMarker } from './RateGraph.svelte';
  import * as Dialog from '$lib/components/ui/dialog/index.js';
  import { useLocale } from '@he11ah0und/localengine-web';
  import { formatSpeed } from '../utils/format.js';

  let {
    open = false,
    onclose,
    title,
    up = [],
    down = [],
    times = [],
    span = 60
  }: {
    open?: boolean;
    onclose?: () => void;
    title: string;
    up?: number[];
    down?: number[];
    // times holds per-sample timestamps (ms), aligned with up/down.
    times?: number[];
    // span is the visible time window in seconds.
    span?: number;
  } = $props();

  const L = useLocale([
    'main.dashboard.upload',
    'main.dashboard.download',
    'main.dashboard.min',
    'main.dashboard.max',
    'main.dashboard.avg',
    'main.graph.total',
    'main.graph.median',
    'main.graph.p95'
  ]);

  // now ticks every animation frame so the stats window drifts with the graph.
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

  interface SeriesStats {
    min: number;
    max: number;
    avg: number;
    median: number;
    p95: number;
  }

  function percentile(sorted: number[], q: number): number {
    if (!sorted.length) return 0;
    const idx = Math.min(sorted.length - 1, Math.max(0, Math.ceil(q * sorted.length) - 1));
    return sorted[idx];
  }

  function calcStats(values: number[]): SeriesStats {
    if (!values.length) return { min: 0, max: 0, avg: 0, median: 0, p95: 0 };
    const sorted = [...values].sort((a, b) => a - b);
    return {
      min: sorted[0],
      max: sorted[sorted.length - 1],
      avg: values.reduce((a, b) => a + b, 0) / values.length,
      median: percentile(sorted, 0.5),
      p95: percentile(sorted, 0.95)
    };
  }

  // visiblePairs filters the samples to the current span window (same drift
  // logic as the graph) and pairs up/down per sample.
  const visiblePairs = $derived.by(() => {
    const n = Math.min(up.length, down.length, times.length);
    const spanMs = Math.max(1, span) * 1000;
    const pairs: { up: number; down: number }[] = [];
    for (let i = 0; i < n; i++) {
      const age = now - times[i];
      if (age < 0 || age > spanMs) continue;
      pairs.push({ up: up[i] || 0, down: down[i] || 0 });
    }
    return pairs;
  });

  const totalStats = $derived(calcStats(visiblePairs.map((p) => p.up + p.down)));
  const downStats = $derived(calcStats(visiblePairs.map((p) => p.down)));
  const upStats = $derived(calcStats(visiblePairs.map((p) => p.up)));

  const currentUp = $derived(up.length ? up[up.length - 1] || 0 : 0);
  const currentDown = $derived(down.length ? down[down.length - 1] || 0 : 0);

  // Marker line styles: distinct dash patterns so the lines stay separable
  // even where colors are close.
  const markerStyle = {
    min: { color: 'var(--color-muted-foreground)', dash: '2 3' },
    max: { color: 'var(--color-destructive)', dash: '6 3' },
    median: { color: 'var(--color-info)', dash: '' },
    p95: { color: 'var(--color-warning)', dash: '4 2' }
  };

  function markersOf(s: SeriesStats): RateMarker[] {
    return [
      { value: s.min, ...markerStyle.min },
      { value: s.max, ...markerStyle.max },
      { value: s.median, ...markerStyle.median },
      { value: s.p95, ...markerStyle.p95 }
    ];
  }

  const downMarkers = $derived(markersOf(downStats));
  const upMarkers = $derived(markersOf(upStats));
</script>

<Dialog.Root {open} onOpenChange={(o) => { if (!o) onclose?.(); }}>
  <Dialog.Content class="sm:max-w-2xl max-h-[85vh] overflow-y-auto">
    <Dialog.Header>
      <Dialog.Title>{title}</Dialog.Title>
    </Dialog.Header>

    <div class="space-y-4">
      <!-- Two independent graphs; current speeds live in the headers. -->
      {@render SeriesGraph(L.mainDashboardDownload, down, currentDown, downStats, downMarkers, 'var(--color-primary)')}
      {@render SeriesGraph(L.mainDashboardUpload, up, currentUp, upStats, upMarkers, 'var(--color-success)')}

      <!-- Stats over the visible window -->
      <div class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="text-muted-foreground">
              <th class="text-left font-medium py-1"></th>
              <th class="text-right font-medium py-1">{L.mainDashboardMin}</th>
              <th class="text-right font-medium py-1">{L.mainDashboardMax}</th>
              <th class="text-right font-medium py-1">{L.mainDashboardAvg}</th>
              <th class="text-right font-medium py-1">{L.mainGraphMedian}</th>
              <th class="text-right font-medium py-1">{L.mainGraphP95}</th>
            </tr>
          </thead>
          <tbody>
            {@render StatsRow(L.mainGraphTotal, totalStats, '')}
            {@render StatsRow(L.mainDashboardDownload, downStats, 'text-[var(--color-primary)]')}
            {@render StatsRow(L.mainDashboardUpload, upStats, 'text-[var(--color-success)]')}
          </tbody>
        </table>
      </div>
    </div>
  </Dialog.Content>
</Dialog.Root>

{#snippet SeriesGraph(
  label: string,
  data: number[],
  current: number,
  stats: SeriesStats,
  markers: RateMarker[],
  color: string
)}
  <div class="space-y-1">
    <div class="flex items-baseline justify-between">
      <p class="text-sm text-muted-foreground">{label}</p>
      <p class="text-lg font-semibold" style="color: {color}">{formatSpeed(current)}</p>
    </div>
    <RateGraph {data} {times} {span} {color} {markers} height={140} />
    <div class="flex flex-wrap gap-x-4 gap-y-1 text-xs">
      {@render LegendItem(L.mainDashboardMin, stats.min, markerStyle.min)}
      {@render LegendItem(L.mainDashboardMax, stats.max, markerStyle.max)}
      {@render LegendItem(L.mainGraphMedian, stats.median, markerStyle.median)}
      {@render LegendItem(L.mainGraphP95, stats.p95, markerStyle.p95)}
    </div>
  </div>
{/snippet}

{#snippet LegendItem(label: string, value: number, style: { color: string; dash: string })}
  <span class="flex items-center gap-1.5">
    <svg width="18" height="4" viewBox="0 0 18 4">
      <line
        x1="0"
        y1="2"
        x2="18"
        y2="2"
        stroke={style.color}
        stroke-width="1.5"
        stroke-dasharray={style.dash}
      />
    </svg>
    <span class="text-muted-foreground">{label}</span>
    <span>{formatSpeed(value)}</span>
  </span>
{/snippet}

{#snippet StatsRow(label: string, s: SeriesStats, cls: string)}
  <tr class="border-t border-border">
    <td class="py-1 {cls}">{label}</td>
    <td class="text-right py-1">{formatSpeed(s.min)}</td>
    <td class="text-right py-1">{formatSpeed(s.max)}</td>
    <td class="text-right py-1">{formatSpeed(s.avg)}</td>
    <td class="text-right py-1">{formatSpeed(s.median)}</td>
    <td class="text-right py-1">{formatSpeed(s.p95)}</td>
  </tr>
{/snippet}
