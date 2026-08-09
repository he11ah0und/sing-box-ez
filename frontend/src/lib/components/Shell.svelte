<script lang="ts">
  import { ArrowLeft, ChevronsLeft, ChevronsRight } from '@lucide/svelte';
  import type { Component, Snippet } from 'svelte';
  import * as Tooltip from '$lib/components/ui/tooltip/index.js';
  import TopBar from './TopBar.svelte';
  import BottomNav from './BottomNav.svelte';
  import { currentLevel, subNav, setRootPage, setRootPageSilent, backPage, setSubTab, enterSubNav, exitSubNav, goHome, menuContext, setMenuContext } from '../stores/navigation.js';
  import { sidebarCollapsed, toggleSidebar } from '../stores/sidebar.js';
  import { useLocale, useLocaleRecord } from '../stores/locale.svelte.js';
  import { pageRegistry } from '../pages/index.js';

  let { children }: { children?: Snippet } = $props();

  const L = useLocale(['common.back', 'app.title']);

  const mainNavItems = pageRegistry.filter((page) => page.nav);
  const currentPage = $derived(pageRegistry.find((i) => i.id === $currentLevel.id));
  const inSubNav = $derived(!!($subNav.pageId && $subNav.pageId === currentPage?.id && currentPage?.tabs?.length));
  const subTabs = $derived(inSubNav ? (currentPage?.tabs ?? []) : []);
  const activeTab = $derived(subTabs.find((tab) => tab.id === $subNav.activeTab));
  // Secondary pages (settings/debug/about) are children of the menu page on mobile.
  const isSecondaryPage = $derived(
    !!(currentPage?.nav && !currentPage?.bottomNav && currentPage.id !== 'menu')
  );
  // Menu page in sub-pages mode: opened from a tabbed page, shows its tabs.
  const menuContextPage = $derived(
    $menuContext ? pageRegistry.find((p) => p.id === $menuContext) : undefined
  );
  const menuSubPages = $derived(
    $currentLevel.id === 'menu' && !!menuContextPage?.tabs?.length
  );
  const showBack = $derived(inSubNav || isSecondaryPage || menuSubPages);

  // The menu page exists only for the mobile bottom nav. When the viewport
  // widens to desktop while the menu (or its sub-pages mode) is open, the
  // desktop rail already exposes those pages, so bounce back to the page
  // the user came from (or main), without recording the redirect itself
  // in the back history.
  $effect(() => {
    const mql = window.matchMedia('(min-width: 640px)');
    const onChange = () => {
      if (mql.matches && $currentLevel.id === 'menu') {
        if (!backPage()) setRootPageSilent('main');
      }
    };
    mql.addEventListener('change', onChange);
    onChange();
    return () => mql.removeEventListener('change', onChange);
  });

  // Every nav label key is known upfront from the page registry.
  const R = useLocaleRecord(
    pageRegistry.flatMap((page) => [page.key, ...(page.tabs ?? []).map((tab) => tab.key)])
  );
  const title = $derived(
    menuSubPages && menuContextPage
      ? R[menuContextPage.key]
      : currentPage
        ? inSubNav && activeTab
          ? `${R[currentPage.key]} · ${R[activeTab.key]}`
          : R[currentPage.key]
        : $currentLevel.id
  );

  function navigate(id: string) {
    const page = pageRegistry.find((p) => p.id === id);
    if (page?.tabs?.length) {
      enterSubNav(id, page.tabs);
    } else {
      exitSubNav();
    }
    setRootPage(id);
  }

  // Desktop rail back: leave the sub-nav and return to the previous page.
  function back() {
    exitSubNav();
    if (!backPage()) goHome();
  }

  // Mobile top-bar back: secondary pages belong to the menu page; from the
  // menu's sub-pages mode it returns to the full menu list.
  function mobileBack() {
    exitSubNav();
    setMenuContext(null);
    setRootPage('menu');
  }

  function selectTab(id: string) {
    setSubTab(id);
  }
</script>

{#snippet railButton(label: string, Icon: Component | undefined, active: boolean, muted: boolean, onclick: () => void)}
  <Tooltip.Root>
    <Tooltip.Trigger>
      {#snippet child({ props })}
        <button
          {...props}
          class="w-full text-left py-3 hover:bg-accent flex items-center gap-3"
          class:px-4={!$sidebarCollapsed}
          class:justify-center={$sidebarCollapsed}
          class:bg-secondary={active}
          class:text-muted-foreground={muted}
          {onclick}
          aria-label={label}
        >
          {#if Icon}<Icon size={20} />{/if}
          {#if !$sidebarCollapsed}<span>{label}</span>{/if}
        </button>
      {/snippet}
    </Tooltip.Trigger>
    {#if $sidebarCollapsed}
      <Tooltip.Content>{label}</Tooltip.Content>
    {/if}
  </Tooltip.Root>
{/snippet}

<div class="flex h-full w-full bg-background text-foreground">
  <!-- Desktop side rail -->
  <aside
    class="hidden md:flex flex-col border-r border-border bg-card transition-[width] duration-200"
    class:w-56={!$sidebarCollapsed}
    class:w-16={$sidebarCollapsed}
  >
    <div
      class="h-14 flex items-center font-semibold border-b border-border"
      class:px-4={!$sidebarCollapsed}
      class:justify-center={$sidebarCollapsed}
    >
      {#if $sidebarCollapsed}
        {L.appTitle.charAt(0)}
      {:else}
        {L.appTitle}
      {/if}
    </div>
    <nav class="flex-1 overflow-auto py-2">
      {#if inSubNav}
        {@render railButton(L.commonBack, ArrowLeft, false, true, back)}
        {#each subTabs as tab (tab.id)}
          {@render railButton(R[tab.key], tab.icon, $subNav.activeTab === tab.id, false, () => selectTab(tab.id))}
        {/each}
      {:else}
        {#each mainNavItems as item (item.id)}
          {@render railButton(R[item.key], item.icon, $currentLevel.id === item.id, false, () => navigate(item.id))}
        {/each}
      {/if}
    </nav>
    <div
      class="border-t border-border p-2 flex"
      class:justify-end={!$sidebarCollapsed}
      class:justify-center={$sidebarCollapsed}
    >
      <button
        class="p-2 rounded-lg hover:bg-accent text-muted-foreground"
        onclick={toggleSidebar}
      >
        {#if $sidebarCollapsed}
          <ChevronsRight size={20} />
        {:else}
          <ChevronsLeft size={20} />
        {/if}
      </button>
    </div>
  </aside>

  <!-- Main area -->
  <div class="flex flex-col flex-1 min-w-0">
    <TopBar
      {title}
      {showBack}
      onBack={mobileBack}
    />

    <main class="flex-1 overflow-auto">
      {@render children?.()}
    </main>

    <!-- Mobile bottom nav -->
    <BottomNav />
  </div>
</div>
