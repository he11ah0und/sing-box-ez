<script lang="ts">
  import { onMount, tick } from 'svelte';
  import type { Component } from 'svelte';
  import { initWailsEvents } from '$lib/wails/bridge.js';
  import { theme, applyTheme, colorScheme, fromThemePayload } from '$lib/stores/theme.js';
  import { signalLocaleReady } from '$lib/stores/locale.js';
  import { GetTheme } from '../bindings/sing-box-ez/internal/gui/wails/bindings.js';
  import { currentLevel } from '$lib/stores/navigation.js';
  import { appState } from '$lib/stores/appState.js';
  import Shell from '$lib/components/Shell.svelte';
  import StartupPage from '$lib/pages/StartupPage.svelte';
  import { loadPage } from '$lib/pages/index.js';
  import { Toaster } from '$lib/components/ui/sonner/index.js';
  import * as Dialog from '$lib/components/ui/dialog/index.js';
  import * as Tooltip from '$lib/components/ui/tooltip/index.js';

  let ActivePage = $state<Component | null>(null);
  let loadToken = 0;

  $effect(() => {
    const id = $currentLevel.id;
    const token = ++loadToken;
    loadPage(id).then((component) => {
      if (token === loadToken) ActivePage = component;
    });
  });

  onMount(() => {
    try {
      initWailsEvents();
      GetTheme()
        .then((t) => {
          if (t) {
            const data = fromThemePayload(t);
            theme.set(data);
            applyTheme(data);
          } else {
            applyTheme($theme);
          }
        })
        .catch((err) => {
          console.warn('Failed to fetch initial theme:', err);
          applyTheme($theme);
        });
      tick().then(() => setTimeout(signalLocaleReady, 0));
    } catch (err) {
      console.error('Failed to init Wails events:', err);
    }
  });

  function closeDialog() {
    appState.update((s) => ({ ...s, dialog: null }));
  }
</script>

<Tooltip.Provider>
  {#if $appState.startup.show}
    <StartupPage />
  {:else}
    <Shell>
      {#if ActivePage}
        <ActivePage />
      {/if}
    </Shell>
  {/if}
</Tooltip.Provider>

<Dialog.Root open={$appState.dialog != null} onOpenChange={(open) => { if (!open) closeDialog(); }}>
  <Dialog.Content>
    <Dialog.Header>
      <Dialog.Title>{$appState.dialog?.title ?? ''}</Dialog.Title>
    </Dialog.Header>
    <p class="text-sm text-muted-foreground">{$appState.dialog?.body ?? ''}</p>
  </Dialog.Content>
</Dialog.Root>

<Toaster position="top-right" theme={$colorScheme} />
