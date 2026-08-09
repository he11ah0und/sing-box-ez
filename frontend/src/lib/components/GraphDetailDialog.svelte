<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import StackedGraph from './StackedGraph.svelte';
  import * as Dialog from '$lib/components/ui/dialog/index.js';
  import { useLocale } from '../stores/locale.svelte.js';
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
    'main.graph.p95',
    'main.graph.current'
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
</script>

<Dialog.Root {open} onOpenChange={(o) => { if (!o) onclose?.(); }}>
  <Dialog.Content class="sm:max-w-2xl">
    <Dialog.Header>
      <Dialog.Title>{title}</Dialog.Title>
    </Dialog.Header>

    <div class="space-y-4">
      <StackedGraph {up} {down} {times} {span} height={200} />

      <!-- Current speeds -->
      <div class="space-y-1">
        <p class="text-sm text-muted-foreground">{L.mainGraphCurrent}</p>
        <div class="grid grid-cols-3 gap-4 text-center">
        <div>
          <p class="text-sm text-muted-foreground">{L.mainDashboardUpload}</p>
          <p class="text-xl font-semibold text-[var(--color-success)]">{formatSpeed(currentUp)}</p>
        </div>
        <div>
          <p class="text-sm text-muted-foreground">{L.mainDashboardDownload}</p>
          <p class="text-xl font-semibold text-[var(--color-primary)]">{formatSpeed(currentDown)}</p>
        </div>
        <div>
          <p class="text-sm text-muted-foreground">{L.mainGraphTotal}</p>
          <p class="text-xl font-semibold">{formatSpeed(currentUp + currentDown)}</p>
        </div>
        </div>
      </div>

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
