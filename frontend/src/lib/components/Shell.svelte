<script>
  import TopBar from './TopBar.svelte';
  import BottomNav from './BottomNav.svelte';
  import { currentLevel, setRootPage } from '../stores/navigation.js';
  import { tValue, locale } from '../stores/locale.js';
  import { pageRegistry } from '../pages/index.js';

  let { children } = $props();

  const navItems = pageRegistry.filter((page) => page.nav);

  function navigate(id) {
    setRootPage(id);
  }

  let title = $derived(tValue($locale, pageRegistry.find(i => i.id === $currentLevel.id)?.key ?? $currentLevel.id, $currentLevel.id));
</script>

<div class="flex h-full w-full bg-[var(--color-bg)] text-[var(--color-text)]">
  <!-- Desktop side rail -->
  <aside class="hidden md:flex w-56 flex-col border-r border-[var(--color-border)] bg-[var(--color-surface)]">
    <div class="h-14 flex items-center px-4 font-semibold border-b border-[var(--color-border)]">
      sing-box-ez
    </div>
    <nav class="flex-1 overflow-auto py-2">
      {#each navItems as item}
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
    </nav>
  </aside>

  <!-- Main area -->
  <div class="flex flex-col flex-1 min-w-0">
    <TopBar {title} />

    <main class="flex-1 overflow-auto">
      {@render children?.()}
    </main>

    <!-- Mobile bottom nav -->
    <BottomNav />
  </div>
</div>
