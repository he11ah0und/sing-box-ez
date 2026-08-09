<script lang="ts">
  // ActivityTimeline renders member connection spans of a connection group
  // as a waterfall: one horizontal track per connection, bar from its start
  // to its end (or to "now" while it is alive). Open tracks use the success
  // color, closed tracks the primary one.
  // With interactive=true the view is navigable: wheel zooms around the
  // cursor, dragging pans, double-click resets to the full window. The
  // miniature embedded in the group dialog is non-interactive and acts as a
  // button that opens the interactive detail dialog.
  import type { ConnSpan } from '../../../bindings/sing-box-ez/internal/core/state/models.js';
  import { RotateCcw } from '@lucide/svelte';
  import { formatDuration } from '../utils/format.js';
  import { useLocale } from '../stores/locale.svelte.js';

  let { spans = [], interactive = true }: { spans?: ConnSpan[]; interactive?: boolean } = $props();

  const L = useLocale(['main.connections.timeline_reset']);

  // Ticks once per second so open tracks grow while the dialog is open.
  let now = $state(Date.now());
  $effect(() => {
    const t = setInterval(() => {
      now = Date.now();
    }, 1000);
    return () => clearInterval(t);
  });

  interface Track {
    id: string;
    start: number;
    end: number; // equals `now` while the connection is alive
    open: boolean;
  }

  const tracks = $derived.by((): Track[] => {
    const list = (spans ?? [])
      .map((s) => {
        const start = new Date(s.start).getTime();
        const end = s.end ? new Date(s.end).getTime() : 0;
        if (!start || Number.isNaN(start)) return null;
        const open = !end || Number.isNaN(end);
        return { id: s.id ?? '', start, end: open ? now : end, open };
      })
      .filter((t): t is Track => t != null)
      .sort((a, b) => a.start - b.start);
    return list;
  });

  // The full window spans the first track start to now, with a 1s floor and
  // a small right margin so the latest bar never hugs the edge.
  const fullStart = $derived(tracks.length ? Math.min(...tracks.map((t) => t.start)) : now - 1000);
  const fullEnd = $derived(Math.max(now, ...tracks.map((t) => t.end)) + 500);
  const fullLen = $derived(Math.max(1000, fullEnd - fullStart));

  // A null view means "follow the full window"; once the user zooms or pans
  // the view stays put until the double-click reset.
  let viewStart = $state<number | null>(null);
  let viewEnd = $state<number | null>(null);
  const windowStart = $derived(viewStart ?? fullStart);
  const windowEnd = $derived(viewEnd ?? fullEnd);
  const windowLen = $derived(Math.max(1, windowEnd - windowStart));

  function clampView(start: number, end: number): [number, number] {
    const len = Math.max(500, end - start);
    const maxLen = fullLen;
    const l = Math.min(len, maxLen);
    let s = start;
    if (s < fullStart) s = fullStart;
    if (s + l > fullEnd) s = fullEnd - l;
    if (s < fullStart) s = fullStart;
    return [s, s + l];
  }

  let container: HTMLDivElement | null = $state(null);

  // Wheel zoom must be non-passive to suppress page scroll.
  $effect(() => {
    const el = container;
    if (!el || !interactive) return;
    const onWheel = (e: WheelEvent) => {
      e.preventDefault();
      const rect = el.getBoundingClientRect();
      const frac = rect.width > 0 ? (e.clientX - rect.left) / rect.width : 0.5;
      const s = viewStart ?? fullStart;
      const en = viewEnd ?? fullEnd;
      const len = Math.max(1, en - s);
      const factor = e.deltaY > 0 ? 1.25 : 0.8;
      const newLen = Math.min(Math.max(500, len * factor), fullLen);
      const anchor = s + len * frac;
      const ns = anchor - newLen * frac;
      const [cs, ce] = clampView(ns, ns + newLen);
      viewStart = cs;
      viewEnd = ce;
    };
    el.addEventListener('wheel', onWheel, { passive: false });
    return () => el.removeEventListener('wheel', onWheel);
  });

  // Drag panning. touch-action: pan-y on the container leaves vertical
  // drags to the scroller (mobile) and only horizontal ones reach this
  // handler; the axis lock below is the same guard for mouse input, so a
  // mostly-vertical gesture can never push the tracks out of view.
  let panning = $state(false);
  let axisLocked = false;
  let panX = 0;
  let panY = 0;
  let panStart = 0;
  let panEnd = 0;

  function onPointerDown(e: PointerEvent) {
    if (!container || !interactive) return;
    axisLocked = false;
    panning = false;
    panX = e.clientX;
    panY = e.clientY;
    panStart = viewStart ?? fullStart;
    panEnd = viewEnd ?? fullEnd;
    container.setPointerCapture(e.pointerId);
  }
  function onPointerMove(e: PointerEvent) {
    if (!container) return;
    const dx = e.clientX - panX;
    if (!panning) {
      if (axisLocked || (dx === 0 && e.clientY === panY)) return;
      const dy = e.clientY - panY;
      if (Math.abs(dx) < 4 && Math.abs(dy) < 4) return;
      axisLocked = true;
      if (Math.abs(dy) > Math.abs(dx)) return; // vertical gesture: not ours
      panning = true;
    }
    const rect = container.getBoundingClientRect();
    if (rect.width <= 0) return;
    const len = panEnd - panStart;
    const shift = (-dx / rect.width) * len;
    const [cs, ce] = clampView(panStart + shift, panEnd + shift);
    viewStart = cs;
    viewEnd = ce;
  }
  function onPointerUp() {
    panning = false;
  }
  function onReset() {
    viewStart = null;
    viewEnd = null;
  }

  function leftPct(t: Track): number {
    return ((t.start - windowStart) / windowLen) * 100;
  }
  function widthPct(t: Track): number {
    // 0.5% floor so instant connections stay visible as a tick.
    return Math.max(0.5, ((t.end - t.start) / windowLen) * 100);
  }
  function fmt(ms: number): string {
    return new Date(ms).toLocaleTimeString();
  }
</script>

{#if tracks.length}
  <div class="space-y-1">
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div
      bind:this={container}
      class="space-y-0.5 select-none {interactive ? (panning ? 'cursor-grabbing' : 'cursor-grab') : ''}"
      style={interactive ? 'touch-action: pan-y' : ''}
      onpointerdown={onPointerDown}
      onpointermove={onPointerMove}
      onpointerup={onPointerUp}
      onpointercancel={onPointerUp}
      ondblclick={onReset}
    >
      {#each tracks as t (t.id + t.start)}
        {@const left = leftPct(t)}
        {@const width = widthPct(t)}
        {#if left < 100 && left + width > 0}
          <!-- Each track: the span bar plus a start-time · duration label. -->
          <div class="flex items-center gap-2">
            <div class="relative h-2.5 flex-1 rounded bg-accent/40" title="{fmt(t.start)} - {t.open ? '' : fmt(t.end)}">
              <div
                class="absolute inset-y-0 rounded {t.open ? 'bg-[var(--color-success)]' : 'bg-primary/70'}"
                style="left: {Math.max(0, left)}%; width: {Math.min(100 - Math.max(0, left), width)}%"
              ></div>
            </div>
            <span class="shrink-0 whitespace-nowrap text-[10px] text-muted-foreground tabular-nums">
              {fmt(t.start)} · {formatDuration(t.end - t.start)}
            </span>
          </div>
        {/if}
      {/each}
    </div>
    <div class="flex justify-between items-center text-[10px] text-muted-foreground">
      <span>{fmt(windowStart)}</span>
      <!-- Visible reset: on touch there is no double-click, and a panned
           view otherwise sticks until the dialog is reopened. -->
      {#if interactive && viewStart !== null}
        <button
          class="p-1 rounded hover:bg-accent"
          aria-label={L.mainConnectionsTimeline_reset}
          title={L.mainConnectionsTimeline_reset}
          onclick={onReset}
        >
          <RotateCcw size={12} />
        </button>
      {/if}
      <span>{fmt(windowEnd)}</span>
    </div>
  </div>
{/if}
