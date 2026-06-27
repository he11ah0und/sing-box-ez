<script>
  import { ArrowLeft, Menu } from '@lucide/svelte';

  let { title = '', showBack = false, onaction } = $props();
</script>

<header class="h-14 flex items-center px-4 border-b border-[var(--color-border)] bg-[var(--color-surface)] shrink-0">
  <button
    class="p-2 -ml-2 rounded-lg hover:bg-[var(--color-surface-variant)] transition-colors md:hidden"
    onclick={() => onaction?.()}
  >
    {#if showBack}
      <ArrowLeft size={24} />
    {:else}
      <Menu size={24} />
    {/if}
  </button>

  <h1 class="ml-3 text-lg font-semibold truncate">{title}</h1>
</header>
