<script lang="ts">
  import { ArrowLeft } from '@lucide/svelte';
  import type { Snippet } from 'svelte';
  import { Button } from '$lib/components/ui/button/index.js';
  import TopBar from './TopBar.svelte';
  import BottomNav from './BottomNav.svelte';
  import { currentLevel, subNav, setRootPage, setSubTab, enterSubNav, exitSubNav, goHome } from '../stores/navigation.js';
  import { useLocale, useLocaleRecord } from '../stores/locale.svelte.js';
  import { pageRegistry } from '../pages/index.js';

  let { children }: { children?: Snippet } = $props();

  const L = useLocale(['common.back', 'app.title']);

  const mainNavItems = pageRegistry.filter((page) => page.nav);
  const currentPage = $derived(pageRegistry.find((i) => i.id === $currentLevel.id));
  const inSubNav = $derived(!!($subNav.pageId && $subNav.pageId === currentPage?.id && currentPage?.tabs?.length));
  const subTabs = $derived(inSubNav ? (currentPage?.tabs ?? []) : []);

  // Every nav label key is known upfront from the page registry.
  const R = useLocaleRecord(
    pageRegistry.flatMap((page) => [page.key, ...(page.tabs ?? []).map((tab) => tab.key)])
  );
  const title = $derived(currentPage ? R[currentPage.key] : $currentLevel.id);

  function navigate(id: string) {
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

  function selectTab(id: string) {
    setSubTab(id);
  }
</script>

<div class="flex h-full w-full bg-background text-foreground">
  <!-- Desktop side rail -->
  <aside class="hidden md:flex w-56 flex-col border-r border-border bg-card">
    <div class="h-14 flex items-center px-4 font-semibold border-b border-border">
      {L.appTitle}
    </div>
    <nav class="flex-1 overflow-auto py-2">
      {#if inSubNav}
        <button
          class="w-full text-left px-4 py-3 hover:bg-accent flex items-center gap-3 text-muted-foreground"
          onclick={back}
          aria-label={L.commonBack}
        >
          <ArrowLeft size={20} />
          <span>{L.commonBack}</span>
        </button>
        {#each subTabs as tab}
          <button
            class="w-full text-left px-4 py-3 hover:bg-accent flex items-center gap-3"
            class:bg-secondary={$subNav.activeTab === tab.id}
            onclick={() => selectTab(tab.id)}
          >
            <span>{R[tab.key]}</span>
          </button>
        {/each}
      {:else}
        {#each mainNavItems as item}
          {@const Icon = item.icon}
          <button
            class="w-full text-left px-4 py-3 hover:bg-accent flex items-center gap-3"
            class:bg-secondary={$currentLevel.id === item.id}
            onclick={() => navigate(item.id)}
          >
            {#if Icon}<Icon size={20} />{/if}
            <span>{R[item.key]}</span>
          </button>
        {/each}
      {/if}
    </nav>
  </aside>

  <!-- Main area -->
  <div class="flex flex-col flex-1 min-w-0">
    <TopBar
      {title}
      showBack={inSubNav}
      onBack={back}
    />

    <!-- Mobile tab bar for sub-pages -->
    {#if inSubNav}
      <div class="flex md:hidden gap-2 px-4 py-2 border-b border-border bg-card">
        {#each subTabs as tab}
          <button
            class="px-3 py-1.5 rounded-lg text-sm font-medium transition"
            class:bg-primary={$subNav.activeTab === tab.id}
            class:text-primary-foreground={$subNav.activeTab === tab.id}
            class:bg-secondary={$subNav.activeTab !== tab.id}
            onclick={() => selectTab(tab.id)}
          >
            {R[tab.key]}
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
