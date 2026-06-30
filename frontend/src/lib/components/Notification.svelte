<script>
  import { appState } from '../stores/appState.js';
  import { X, Info } from '@lucide/svelte';

  function remove(index) {
    appState.update((s) => ({
      ...s,
      notifications: s.notifications.filter((_, i) => i !== index)
    }));
  }
</script>

<div class="fixed top-4 right-4 z-50 flex flex-col gap-2 w-80 pointer-events-none">
  {#each $appState.notifications as n, i (i)}
    <div
      class="pointer-events-auto rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)] p-4 shadow-lg flex items-start gap-3"
    >
      <Info size={20} class="shrink-0 text-[var(--color-primary)] mt-0.5" />
      <div class="flex-1 min-w-0">
        <p class="font-medium text-sm">{n.title}</p>
        <p class="text-sm text-[var(--color-text-muted)] break-words">{n.body}</p>
      </div>
      <button class="p-1 rounded-lg hover:bg-[var(--color-surface-variant)] shrink-0" onclick={() => remove(i)}>
        <X size={16} />
      </button>
    </div>
  {/each}
</div>
