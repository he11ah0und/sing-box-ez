<script>
  import { ArrowLeft } from '@lucide/svelte';

  let { title = '', showBack = false, onBack = null } = $props();
</script>

<header class="h-14 flex items-center gap-3 px-4 border-b border-[var(--color-border)] bg-[var(--color-surface)] shrink-0">
  {#if showBack}
    <button
      class="md:hidden p-1 -ml-1 rounded-lg hover:bg-[var(--color-surface-variant)] transition"
      onclick={onBack}
      aria-label="Back"
    >
      <ArrowLeft size={20} />
    </button>
  {/if}
  <h1 class="text-lg font-semibold truncate">{title}</h1>
</header>
