<script>
  import TopBar from './TopBar.svelte';
  import BottomNav from './BottomNav.svelte';
  import { currentLevel, canGoBack, popLevel, pushLevel } from '../stores/navigation.js';
  import { tValue, locale } from '../stores/locale.js';

  let { children } = $props();

  const navItems = [
    { id: 'main', key: 'tab.main' },
    { id: 'configs', key: 'tab.configs' },
    { id: 'core', key: 'tab.core' },
    { id: 'settings', key: 'tab.settings' },
    { id: 'logs', key: 'tab.logs' },
    { id: 'about', key: 'tab.about' }
  ];

  function handleTopButton() {
    if ($canGoBack) {
      popLevel();
    } else {
      pushLevel({ type: 'page', id: 'menu' });
    }
  }

  function navigate(id) {
    if (id === 'menu') {
      pushLevel({ type: 'page', id: 'menu' });
    } else {
      pushLevel({ type: 'page', id });
    }
  }

  let title = $derived(tValue($locale, navItems.find(i => i.id === $currentLevel.id)?.key ?? $currentLevel.id, $currentLevel.id));
</script>

<div class="flex h-full w-full bg-[var(--color-bg)] text-[var(--color-text)]">
  <!-- Desktop side rail -->
  <aside class="hidden md:flex w-56 flex-col border-r border-[var(--color-border)] bg-[var(--color-surface)]">
    <div class="h-14 flex items-center px-4 font-semibold border-b border-[var(--color-border)]">
      sing-box-ez
    </div>
    <nav class="flex-1 overflow-auto py-2">
      {#each navItems as item}
        <button
          class="w-full text-left px-4 py-3 hover:bg-[var(--color-surface-variant)] flex items-center gap-3"
          class:bg-[var(--color-surface-variant)]={$currentLevel.id === item.id}
          onclick={() => navigate(item.id)}
        >
          <span>{tValue($locale, item.key, item.id)}</span>
        </button>
      {/each}
    </nav>
  </aside>

  <!-- Main area -->
  <div class="flex flex-col flex-1 min-w-0">
    <TopBar {title} showBack={$canGoBack} onaction={handleTopButton} />

    <main class="flex-1 overflow-auto">
      {@render children?.()}
    </main>

    <!-- Mobile bottom nav -->
    <BottomNav />
  </div>
</div>
