<script module>
  import { Menu } from '@lucide/svelte';
  export const pageMeta = {
    id: 'menu',
    key: 'tab.menu',
    icon: Menu,
    nav: false,
    bottomNav: false
  };
</script>

<script>
  import { setRootPage, currentLevel } from '../stores/navigation.js';
  import { locale, tValue } from '../stores/locale.js';
  import { pageRegistry } from '../pages/index.js';

  const items = pageRegistry.filter((page) => page.nav && !page.bottomNav);

  function navigate(id) {
    setRootPage(id);
  }
</script>

<div class="p-4">
  <h2 class="text-2xl font-bold mb-4">{tValue($locale, 'tab.menu', 'Menu')}</h2>
  <div class="flex flex-col gap-2">
    {#each items as item}
      {@const Icon = item.icon}
      <button
        class="w-full text-left px-4 py-4 rounded-lg bg-[var(--color-surface)] border border-[var(--color-border)] hover:bg-[var(--color-surface-variant)] transition-colors flex items-center gap-3"
        class:text-[var(--color-primary)]={$currentLevel.id === item.id}
        onclick={() => navigate(item.id)}
      >
        <Icon size={20} />
        {tValue($locale, item.key, item.id)}
      </button>
    {/each}
  </div>
</div>
