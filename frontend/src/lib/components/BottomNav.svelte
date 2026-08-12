<script lang="ts">
  import { setRootPage, currentLevel, enterSubNav, exitSubNav, setMenuContext } from '../stores/navigation.js';
  import { appState } from '../stores/appState.js';
  import { useLocaleRecord } from '@he11ah0und/localengine-web';
  import { pageRegistry, tabsVisible } from '../pages/index.js';

  // Always the same three entries (main / configs / menu) — registry order.
  const mainItems = pageRegistry.filter((page) => page.bottomNav);
  const R = useLocaleRecord(mainItems.map((item) => item.key));

  // The menu button stands in for every secondary page (settings/debug/about/core),
  // so it stays highlighted while any non-primary page is open.
  function isActive(id: string): boolean {
    if (id === 'menu') {
      return !mainItems.some((item) => item.id !== 'menu' && item.id === $currentLevel.id);
    }
    return $currentLevel.id === id;
  }

  function navigate(id: string) {
    const page = pageRegistry.find((p) => p.id === id);
    const connected = $appState.api.phase === 'connected';
    if (id === 'menu') {
      // Opening the menu from a page with visible tabs shows that page's
      // tabs; from a primary page (or the menu itself) it resets to the
      // full list.
      const from = pageRegistry.find((p) => p.id === $currentLevel.id);
      setMenuContext(tabsVisible(from, connected) ? (from?.id ?? null) : null);
    }
    if (tabsVisible(page, connected)) {
      enterSubNav(id, page?.tabs ?? []);
    } else {
      exitSubNav();
    }
    setRootPage(id);
  }
</script>

<nav class="h-16 border-t border-border bg-card flex items-center justify-around px-2 md:hidden shrink-0">
  {#each mainItems as item (item.id)}
    {@const Icon = item.icon}
    <button
      class="flex flex-col items-center p-2 rounded-lg hover:bg-accent transition-colors"
      class:text-primary={isActive(item.id)}
      onclick={() => navigate(item.id)}
    >
      {#if Icon}<Icon size={22} />{/if}
      <span class="text-xs mt-1">{R[item.key]}</span>
    </button>
  {/each}
</nav>
