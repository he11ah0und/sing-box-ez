<script lang="ts">
  // TimelineDetailDialog shows a connection group's activity timeline at
  // full size with navigation (wheel zoom, drag pan, double-click reset).
  // The group dialog embeds a non-interactive miniature that opens this.
  import ActivityTimeline from './ActivityTimeline.svelte';
  import * as Dialog from '$lib/components/ui/dialog/index.js';
  import type { ConnSpan } from '../../../bindings/sing-box-ez/internal/core/state/models.js';

  let {
    open = false,
    onclose,
    title,
    spans = []
  }: {
    open?: boolean;
    onclose?: () => void;
    title: string;
    spans?: ConnSpan[];
  } = $props();
</script>

<Dialog.Root {open} onOpenChange={(o) => { if (!o) onclose?.(); }}>
  <Dialog.Content class="sm:max-w-3xl max-h-[85vh] overflow-y-auto">
    <Dialog.Header>
      <Dialog.Title>{title}</Dialog.Title>
    </Dialog.Header>
    <ActivityTimeline {spans} />
  </Dialog.Content>
</Dialog.Root>
