<script>
  import { ArrowLeft } from '@lucide/svelte';
  import { setRootPage, currentLevel, subNav, enterSubNav, exitSubNav, goHome } from '../stores/navigation.js';
  import { tValue, locale } from '../stores/locale.js';
  import { pageRegistry } from '../pages/index.js';

  const mainItems = pageRegistry.filter((page) => page.bottomNav);
  const currentPage = $derived(pageRegistry.find((p) => p.id === $currentLevel.id));
  const inSubNav = $derived(!!($subNav.pageId && $subNav.pageId === currentPage?.id && currentPage?.tabs?.length));

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
</script>

<nav class="h-16 border-t border-[var(--color-border)] bg-[var(--color-surface)] flex items-center justify-around px-2 md:hidden shrink-0">
  {#if inSubNav}
    <button
      class="flex items-center gap-2 px-4 py-2 rounded-lg hover:bg-[var(--color-surface-variant)] transition-colors"
      onclick={back}
    >
      <ArrowLeft size={22} />
      <span class="text-sm">{tValue($locale, 'common.back', 'Back')}</span>
    </button>
  {:else}
    {#each mainItems as item}
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
  {/if}
</nav>
