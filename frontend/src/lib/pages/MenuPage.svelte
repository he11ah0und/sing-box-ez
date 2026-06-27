<script>
  import { pushLevel, currentLevel } from '../stores/navigation.js';
  import { locale, tValue } from '../stores/locale.js';

  const items = [
    { id: 'core', key: 'tab.core' },
    { id: 'settings', key: 'tab.settings' },
    { id: 'logs', key: 'tab.logs' },
    { id: 'about', key: 'tab.about' }
  ];

  function navigate(id) {
    pushLevel({ type: 'page', id });
  }
</script>

<div class="p-4">
  <h2 class="text-2xl font-bold mb-4">{tValue($locale, 'tab.menu', 'Menu')}</h2>
  <div class="flex flex-col gap-2">
    {#each items as item}
      <button
        class="w-full text-left px-4 py-4 rounded-lg bg-[var(--color-surface)] border border-[var(--color-border)] hover:bg-[var(--color-surface-variant)] transition-colors"
        class:text-[var(--color-primary)]={$currentLevel.id === item.id}
        onclick={() => navigate(item.id)}
      >
        {tValue($locale, item.key, item.id)}
      </button>
    {/each}
  </div>
</div>
