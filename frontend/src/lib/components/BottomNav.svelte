<script>
  import { Home, List, Menu } from '@lucide/svelte';
  import { pushLevel, currentLevel } from '../stores/navigation.js';
  import { tValue, locale } from '../stores/locale.js';

  const items = [
    { id: 'main', key: 'tab.main', icon: Home },
    { id: 'configs', key: 'tab.configs', icon: List }
  ];

  function navigate(id) {
    pushLevel({ type: 'page', id });
  }
</script>

<nav class="h-16 border-t border-[var(--color-border)] bg-[var(--color-surface)] flex items-center justify-around px-2 md:hidden shrink-0">
  {#each items as item}
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
  <button
    class="flex flex-col items-center p-2 rounded-lg hover:bg-[var(--color-surface-variant)] transition-colors"
    class:text-[var(--color-primary)]={$currentLevel.id === 'menu'}
    onclick={() => navigate('menu')}
  >
    <Menu size={22} />
    <span class="text-xs mt-1">{tValue($locale, 'tab.menu', 'Menu')}</span>
  </button>
</nav>
