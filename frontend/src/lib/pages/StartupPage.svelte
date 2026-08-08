<script lang="ts">
  import { appState } from '../stores/appState.js';
  import { useLocale } from '../stores/locale.svelte.js';
  import * as Card from '$lib/components/ui/card/index.js';
  import { Button } from '$lib/components/ui/button/index.js';
  import { cn } from '$lib/utils.js';

  const L = useLocale(['app.title', 'startup.subtitle', 'startup.continue']);

  function selectMode(mode: string) {
    appState.update((s) => ({ ...s, startup: { ...s.startup, selected: mode } }));
  }

  function continueStartup() {
    appState.update((s) => ({ ...s, startup: { ...s.startup, show: false } }));
  }
</script>

<div class="h-full flex flex-col items-center justify-center p-6 bg-background">
  <Card.Root class="w-full max-w-md shadow-xl">
    <Card.Header>
      <Card.Title class="text-2xl">{L.appTitle}</Card.Title>
      <Card.Description>{L.startupSubtitle}</Card.Description>
    </Card.Header>
    <Card.Content class="flex flex-col gap-3">
      {#each $appState.startup.options as option (option.id)}
        <button
          class={cn(
            'px-4 py-3 rounded-lg border border-border text-left transition-colors hover:bg-accent',
            $appState.startup.selected === option.id && 'bg-primary text-primary-foreground hover:bg-primary/90 border-transparent'
          )}
          onclick={() => selectMode(option.id)}
        >
          {option.label}
        </button>
      {/each}
    </Card.Content>
    <Card.Footer>
      <Button class="w-full" size="lg" onclick={continueStartup}>
        {L.startupContinue}
      </Button>
    </Card.Footer>
  </Card.Root>
</div>
