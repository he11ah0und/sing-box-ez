<script>
  import { X } from '@lucide/svelte';

  let { title = '', showClose = true, onclose, children, footer } = $props();
</script>

<div class="fixed inset-0 z-50 flex items-end sm:items-center justify-center bg-black/60 p-0 sm:p-4">
  <div class="w-full h-full sm:h-auto sm:max-h-[90vh] sm:max-w-lg bg-[var(--color-surface)] sm:rounded-xl shadow-2xl flex flex-col">
    {#if title || showClose}
      <div class="flex items-center justify-between px-4 py-3 border-b border-[var(--color-border)]">
        {#if title}
          <h2 class="text-lg font-semibold">{title}</h2>
        {:else}
          <span></span>
        {/if}
        {#if showClose}
          <button class="p-1 rounded-lg hover:bg-[var(--color-surface-variant)] transition-colors" onclick={() => onclose?.()}>
            <X size={20} />
          </button>
        {/if}
      </div>
    {/if}
    <div class="flex-1 overflow-auto p-4">
      {@render children?.()}
    </div>
    {#if footer}
      <div class="px-4 py-3 border-t border-[var(--color-border)] flex justify-end gap-2">
        {@render footer()}
      </div>
    {/if}
  </div>
</div>
