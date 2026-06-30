<script>
  import { setRootPage, currentLevel } from '../stores/navigation.js';
  import { tValue, locale } from '../stores/locale.js';
  import { pageRegistry } from '../pages/index.js';

  const items = pageRegistry.filter((page) => page.bottomNav);

  function navigate(id) {
    setRootPage(id);
  }
</script>

<nav class="h-16 border-t border-[var(--color-border)] bg-[var(--color-surface)] flex items-center justify-around px-2 md:hidden shrink-0">
  {#each items as item}
    {@const Icon = item.icon}
    <button
      class="flex flex-col items-center p-2 rounded-lg hover:bg-[var(--color-surface-variant)] transition-colors"
      class:text-[var(--color-primary)]={$currentLevel.id === item.id}
      onclick={() => navigate(item.id)}
    >
      <Icon size={22} />
      <span class="text-xs mt-1">{tValue($locale, item.key, item.id)}</span>
    </button>
  {/each}
</nav>
