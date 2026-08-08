<script lang="ts">
  import { setRootPage, currentLevel } from '../stores/navigation.js';
  import { locale, tValue, useLocale } from '../stores/locale.js';
  import { pageRegistry } from '../pages/index.js';
  import { cn } from '$lib/utils.js';

  const tabMenu = useLocale('tab.menu');

  const items = pageRegistry.filter((page) => page.nav && !page.bottomNav);

  function navigate(id: string) {
    setRootPage(id);
  }
</script>

<div class="p-4">
  <h2 class="text-2xl font-bold mb-4">{$tabMenu}</h2>
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
        {tValue($locale, item.key)}
      </button>
    {/each}
  </div>
</div>
