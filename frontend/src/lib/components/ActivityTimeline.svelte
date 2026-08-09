<script lang="ts">
  // ActivityTimeline renders member connection spans of a connection group
  // as a waterfall: one horizontal track per connection, bar from its start
  // to its end (or to "now" while it is alive). Open tracks use the success
  // color, closed tracks the muted one.
  import type { ConnSpan } from '../../../bindings/sing-box-ez/internal/core/state/models.js';

  let { spans = [] }: { spans?: ConnSpan[] } = $props();

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

  // The window spans the first track start to now, with a 1s floor and a
  // small right margin so the latest bar never hugs the edge.
  const windowStart = $derived(tracks.length ? Math.min(...tracks.map((t) => t.start)) : now - 1000);
  const windowEnd = $derived(Math.max(now, ...tracks.map((t) => t.end)) + 500);
  const windowLen = $derived(Math.max(1000, windowEnd - windowStart));

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
    <div class="space-y-0.5">
      {#each tracks as t (t.id + t.start)}
        <div class="relative h-2.5 rounded bg-accent/40" title="{fmt(t.start)} - {t.open ? '' : fmt(t.end)}">
          <div
            class="absolute inset-y-0 rounded {t.open ? 'bg-[var(--color-success)]' : 'bg-primary/70'}"
            style="left: {leftPct(t)}%; width: {widthPct(t)}%"
          ></div>
        </div>
      {/each}
    </div>
    <div class="flex justify-between text-[10px] text-muted-foreground">
      <span>{fmt(windowStart)}</span>
      <span>{fmt(windowEnd - 500)}</span>
    </div>
  </div>
{/if}
