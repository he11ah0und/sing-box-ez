<script lang="ts">
  import { onMount, tick } from 'svelte';
  import { initWailsEvents } from '$lib/wails/bridge.js';
  import { theme, applyTheme, colorScheme } from '$lib/stores/theme.js';
  import { signalLocaleReady } from '$lib/stores/locale.js';
  import { GetTheme } from '../bindings/sing-box-ez/internal/gui/wails/bindings.js';
  import { currentLevel } from '$lib/stores/navigation.js';
  import { appState } from '$lib/stores/appState.js';
  import Shell from '$lib/components/Shell.svelte';
  import StartupPage from '$lib/pages/StartupPage.svelte';
  import { pageComponents } from '$lib/pages/index.js';
  import { Toaster } from '$lib/components/ui/sonner/index.js';
  import * as Dialog from '$lib/components/ui/dialog/index.js';

  let ActivePage = $derived(pageComponents[$currentLevel.id] ?? pageComponents.main);

  onMount(() => {
    console.log('App mounted');
    try {
      initWailsEvents();
      GetTheme()
        .then((t) => {
          if (t) {
            theme.set(t);
            applyTheme(t);
          } else {
            applyTheme($theme);
          }
        })
        .catch((err) => {
          console.warn('Failed to fetch initial theme:', err);
          applyTheme($theme);
        });
      tick().then(() => setTimeout(signalLocaleReady, 0));
      console.log('Wails events initialized');
    } catch (err) {
      console.error('Failed to init Wails events:', err);
    }
  });

  function closeDialog() {
    appState.update((s) => ({ ...s, dialog: null }));
  }
</script>

{#if $appState.startup.show}
  <StartupPage />
{:else}
  <Shell>
    <ActivePage />
  </Shell>
{/if}

<Dialog.Root open={$appState.dialog != null} onOpenChange={(open) => { if (!open) closeDialog(); }}>
  <Dialog.Content>
    <Dialog.Header>
      <Dialog.Title>{$appState.dialog?.title ?? ''}</Dialog.Title>
    </Dialog.Header>
    <p class="text-sm text-muted-foreground">{$appState.dialog?.body ?? ''}</p>
  </Dialog.Content>
</Dialog.Root>

<Toaster position="top-right" theme={$colorScheme} />
