<script lang="ts">
  import { setRootPage, currentLevel, enterSubNav, exitSubNav, setSubTab, menuContext } from '../stores/navigation.js';
  import { appState } from '../stores/appState.js';
  import { useLocaleRecord } from '@he11ah0und/localengine-web';
  import { pageRegistry, tabsVisible, type PageMeta } from '../pages/index.js';
  import { cn } from '$lib/utils.js';

  const coreConnected = $derived($appState.api.phase === 'connected');
  const items = pageRegistry.filter((page) => page.nav && !page.bottomNav);
  const R = useLocaleRecord(
    pageRegistry.flatMap((page) => [page.key, ...(page.tabs ?? []).map((tab) => tab.key)])
  );

  // Sub-pages mode: the menu was opened from a tabbed page and shows only
  // that page's tabs. Otherwise the full secondary-pages list is shown.
  const contextPage = $derived(
    $menuContext ? pageRegistry.find((p) => p.id === $menuContext) : undefined
  );
  const subPagesMode = $derived(tabsVisible(contextPage, coreConnected));

  function navigate(id: string) {
    const page = pageRegistry.find((p) => p.id === id);
    if (tabsVisible(page, coreConnected)) {
      enterSubNav(id, page?.tabs ?? []);
    } else {
      exitSubNav();
    }
    setRootPage(id);
  }

  function openTab(page: PageMeta, tabId: string) {
    enterSubNav(page.id, page.tabs ?? []);
    setSubTab(tabId);
    setRootPage(page.id);
  }
</script>

<div class="p-4">
  {#if subPagesMode && contextPage}
    <div class="flex flex-col gap-2">
      {#each contextPage.tabs as tab (tab.id)}
        {@const TabIcon = tab.icon}
        <button
          class="w-full text-left px-4 py-4 rounded-lg bg-card border border-border hover:bg-accent transition-colors flex items-center gap-3"
          onclick={() => contextPage && openTab(contextPage, tab.id)}
        >
          {#if TabIcon}<TabIcon size={20} />{/if}
          {R[tab.key]}
        </button>
      {/each}
    </div>
  {:else}
    <div class="flex flex-col gap-2">
      {#each items as item (item.id)}
        {@const Icon = item.icon}
        <button
          class={cn(
            'w-full text-left px-4 py-4 rounded-lg bg-card border border-border hover:bg-accent transition-colors flex items-center gap-3',
            $currentLevel.id === item.id && 'text-primary'
          )}
          onclick={() => navigate(item.id)}
        >
          {#if Icon}<Icon size={20} />{/if}
          {R[item.key]}
        </button>
      {/each}
    </div>
  {/if}
</div>
