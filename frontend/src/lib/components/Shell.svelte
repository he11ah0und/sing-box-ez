<script>
  import { ArrowLeft } from '@lucide/svelte';
  import TopBar from './TopBar.svelte';
  import BottomNav from './BottomNav.svelte';
  import { currentLevel, subNav, setRootPage, setSubTab, enterSubNav, exitSubNav, goHome } from '../stores/navigation.js';
  import { tValue, locale } from '../stores/locale.js';
  import { pageRegistry } from '../pages/index.js';

  let { children } = $props();

  const mainNavItems = pageRegistry.filter((page) => page.nav);
  const currentPage = $derived(pageRegistry.find((i) => i.id === $currentLevel.id));
  const inSubNav = $derived(!!($subNav.pageId && $subNav.pageId === currentPage?.id && currentPage?.tabs?.length));
  const subTabs = $derived(inSubNav ? currentPage.tabs : []);

  function navigate(id) {
    const page = pageRegistry.find((p) => p.id === id);
    if (page?.tabs?.length) {
      enterSubNav(id, page.tabs);
    } else {
      exitSubNav();
    }
    setRootPage(id);
  }

  function back() {
    exitSubNav();
    goHome();
  }

  function selectTab(id) {
    setSubTab(id);
  }
</script>

<div class="flex h-full w-full bg-[var(--color-bg)] text-[var(--color-text)]">
  <!-- Desktop side rail -->
  <aside class="hidden md:flex w-56 flex-col border-r border-[var(--color-border)] bg-[var(--color-surface)]">
    <div class="h-14 flex items-center px-4 font-semibold border-b border-[var(--color-border)]">
      sing-box-ez
    </div>
    <nav class="flex-1 overflow-auto py-2">
      {#if inSubNav}
        <button
          class="w-full text-left px-4 py-3 hover:bg-[var(--color-surface-variant)] flex items-center gap-3 text-[var(--color-text-muted)]"
          onclick={back}
        >
          <ArrowLeft size={20} />
          <span>{tValue($locale, 'common.back', 'Back')}</span>
        </button>
        {#each subTabs as tab}
          <button
            class="w-full text-left px-4 py-3 hover:bg-[var(--color-surface-variant)] flex items-center gap-3"
            class:bg-[var(--color-surface-variant)]={$subNav.activeTab === tab.id}
            onclick={() => selectTab(tab.id)}
          >
            <span>{tValue($locale, tab.key, tab.id)}</span>
          </button>
        {/each}
      {:else}
        {#each mainNavItems as item}
          {@const Icon = item.icon}
          <button
            class="w-full text-left px-4 py-3 hover:bg-[var(--color-surface-variant)] flex items-center gap-3"
            class:bg-[var(--color-surface-variant)]={$currentLevel.id === item.id}
            onclick={() => navigate(item.id)}
          >
            <Icon size={20} />
            <span>{tValue($locale, item.key, item.id)}</span>
          </button>
        {/each}
      {/if}
    </nav>
  </aside>

  <!-- Main area -->
  <div class="flex flex-col flex-1 min-w-0">
    <TopBar
      title={tValue($locale, currentPage?.key ?? $currentLevel.id, $currentLevel.id)}
      showBack={inSubNav}
      onBack={back}
    />

    <!-- Mobile tab bar for sub-pages -->
    {#if inSubNav}
      <div class="flex md:hidden gap-2 px-4 py-2 border-b border-[var(--color-border)] bg-[var(--color-surface)]">
        {#each subTabs as tab}
          <button
            class="px-3 py-1.5 rounded-lg text-sm font-medium transition"
            class:bg-[var(--color-primary)]={$subNav.activeTab === tab.id}
            class:text-white={$subNav.activeTab === tab.id}
            class:bg-[var(--color-surface-variant)]={$subNav.activeTab !== tab.id}
            onclick={() => selectTab(tab.id)}
          >
            {tValue($locale, tab.key, tab.id)}
          </button>
        {/each}
      </div>
    {/if}

    <main class="flex-1 overflow-auto">
      {@render children?.()}
    </main>

    <!-- Mobile bottom nav -->
    <BottomNav />
  </div>
</div>
