<script lang="ts">
  import { ArrowLeft } from '@lucide/svelte';
  import { Button } from '$lib/components/ui/button/index.js';
  import { setRootPage, currentLevel, subNav, enterSubNav, exitSubNav, goHome } from '../stores/navigation.js';
  import { useLocale, useLocaleRecord } from '../stores/locale.svelte.js';
  import { pageRegistry } from '../pages/index.js';

  const mainItems = pageRegistry.filter((page) => page.bottomNav);
  const currentPage = $derived(pageRegistry.find((i) => i.id === $currentLevel.id));
  const inSubNav = $derived(!!($subNav.pageId && $subNav.pageId === currentPage?.id && currentPage?.tabs?.length));

  const L = useLocale(['common.back']);
  const R = useLocaleRecord(mainItems.map((item) => item.key));

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
</script>

<nav class="h-16 border-t border-border bg-card flex items-center justify-around px-2 md:hidden shrink-0">
  {#if inSubNav}
    <Button variant="ghost" class="gap-2" onclick={back}>
      <ArrowLeft size={22} />
      <span class="text-sm">{L.commonBack}</span>
    </Button>
  {:else}
    {#each mainItems as item}
      {@const Icon = item.icon}
      <button
        class="flex flex-col items-center p-2 rounded-lg hover:bg-accent transition-colors"
        class:text-primary={$currentLevel.id === item.id}
        onclick={() => navigate(item.id)}
      >
        {#if Icon}<Icon size={22} />{/if}
        <span class="text-xs mt-1">{R[item.key]}</span>
      </button>
    {/each}
  {/if}
</nav>
