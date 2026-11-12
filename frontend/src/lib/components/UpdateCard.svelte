<script lang="ts">
  import { RefreshCw } from '@lucide/svelte';
  import { Button } from '$lib/components/ui/button/index.js';
  import * as AlertDialog from '$lib/components/ui/alert-dialog/index.js';

  // Shared update card: installed/latest version boxes, a primary check button
  // and an update dialog that opens only when a newer version exists.
  let {
    current = '',
    latest = '',
    checking = false,
    currentLabel = '',
    latestLabel = '',
    checkLabel = '',
    updateTitle = '',
    closeLabel = '',
    hint = '',
    label = '',
    actionLabel = '',
    open = $bindable(false),
    onCheck,
    onAction
  }: {
    current?: string;
    latest?: string;
    checking?: boolean;
    currentLabel?: string;
    latestLabel?: string;
    checkLabel?: string;
    updateTitle?: string;
    closeLabel?: string;
    hint?: string;
    label?: string;
    actionLabel?: string;
    open?: boolean;
    onCheck: () => void;
    onAction?: () => void;
  } = $props();
</script>

<div class="space-y-4">
<div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
  {#if label}
    <p class="col-span-full text-sm text-muted-foreground">{label}</p>
  {/if}
  <div class="rounded-xl bg-background border border-border p-4">
    <p class="text-sm text-muted-foreground">{currentLabel}</p>
    <p class="text-lg font-medium">{current || '—'}</p>
  </div>
  <div class="rounded-xl bg-background border border-border p-4">
    <p class="text-sm text-muted-foreground">{latestLabel}</p>
    <p class="text-lg font-medium">{latest || '—'}</p>
  </div>
</div>
<div class="flex flex-wrap items-center gap-3">
  <Button disabled={checking} onclick={onCheck}>
    <RefreshCw size={16} class={checking ? 'animate-spin' : ''} />
    {checkLabel}
  </Button>
</div>

</div>

<AlertDialog.Root bind:open>
  <AlertDialog.Content>
    <AlertDialog.Header>
      <AlertDialog.Title>{updateTitle}</AlertDialog.Title>
      <AlertDialog.Description>
        {latestLabel}: {latest}{#if hint} — <code>{hint}</code>{/if}
      </AlertDialog.Description>
    </AlertDialog.Header>
    <AlertDialog.Footer>
      {#if onAction && actionLabel}
        <AlertDialog.Action onclick={onAction}>{actionLabel}</AlertDialog.Action>
      {/if}
      <AlertDialog.Cancel>{closeLabel}</AlertDialog.Cancel>
    </AlertDialog.Footer>
  </AlertDialog.Content>
</AlertDialog.Root>
