<script lang="ts">
  import { ArrowLeft } from '@lucide/svelte';
  import { Button } from '$lib/components/ui/button/index.js';
  import * as Tooltip from '$lib/components/ui/tooltip/index.js';
  import { locale, tValue } from '../stores/locale.js';

  let {
    title = '',
    showBack = false,
    onBack = null
  }: {
    title?: string;
    showBack?: boolean;
    onBack?: (() => void) | null;
  } = $props();
</script>

<header class="h-14 flex items-center gap-3 px-4 border-b border-border bg-card shrink-0">
  {#if showBack}
    <Tooltip.Root>
      <Tooltip.Trigger>
        {#snippet child({ props })}
          <Button
            {...props}
            variant="ghost"
            size="icon"
            class="md:hidden -ml-1"
            onclick={() => onBack?.()}
            aria-label={tValue($locale, 'common.back')}
          >
            <ArrowLeft size={20} />
          </Button>
        {/snippet}
      </Tooltip.Trigger>
      <Tooltip.Content>{tValue($locale, 'common.back')}</Tooltip.Content>
    </Tooltip.Root>
  {/if}
  <h1 class="text-lg font-semibold truncate">{title}</h1>
</header>
