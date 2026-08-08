<script lang="ts">
  import { onMount, tick } from 'svelte';
  import type { Component } from 'svelte';
  import { initWailsEvents } from '$lib/wails/bridge.js';
  import { theme, applyTheme, colorScheme, fromThemePayload } from '$lib/stores/theme.js';
  import { signalLocaleReady, locale, tValue, tValues } from '$lib/stores/locale.js';
  import { GetTheme, SetFallbackType } from '../bindings/sing-box-ez/internal/gui/wails/bindings.js';
  import { currentLevel } from '$lib/stores/navigation.js';
  import { appState, type StyleCheckState } from '$lib/stores/appState.js';
  import Shell from '$lib/components/Shell.svelte';
  import StartupPage from '$lib/pages/StartupPage.svelte';
  import { loadPage } from '$lib/pages/index.js';
  import { Toaster } from '$lib/components/ui/sonner/index.js';
  import * as Dialog from '$lib/components/ui/dialog/index.js';
  import * as AlertDialog from '$lib/components/ui/alert-dialog/index.js';
  import { Button } from '$lib/components/ui/button/index.js';
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

  function clearStyleCheck() {
    appState.update((s) => ({ ...s, styleCheck: null }));
  }

  async function resolveStyleCheck(check: StyleCheckState, fallbackType: 'ignore' | 'to_client') {
    clearStyleCheck();
    try {
      await SetFallbackType(check.config, fallbackType);
    } catch (err) {
      console.error('SetFallbackType failed:', err);
    }
  }

  const styleCheckTitle = $derived(
    $appState.styleCheck
      ? tValues($locale, [
          `dialog.config_style.${$appState.styleCheck.style}_title`,
          'dialog.config_style.unknown_title'
        ])
      : ''
  );
  const styleCheckBody = $derived(
    $appState.styleCheck
      ? tValues($locale, [
          `dialog.config_style.${$appState.styleCheck.style}_body`,
          'dialog.config_style.unknown_body'
        ])
      : ''
  );
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

{#if $appState.styleCheck}
  <AlertDialog.Root open={true} onOpenChange={(open) => { if (!open) clearStyleCheck(); }}>
    <AlertDialog.Content>
      <AlertDialog.Header>
        <AlertDialog.Title>{styleCheckTitle}</AlertDialog.Title>
        <AlertDialog.Description>
          {$appState.styleCheck.config} — {styleCheckBody}
        </AlertDialog.Description>
      </AlertDialog.Header>
      <AlertDialog.Footer>
        <Button variant="outline" onclick={clearStyleCheck}>
          {tValue($locale, 'dialog.config_style.btn.cancel')}
        </Button>
        <Button variant="outline" onclick={() => resolveStyleCheck($appState.styleCheck!, 'ignore')}>
          {tValue($locale, 'dialog.config_style.btn.ignore')}
        </Button>
        <Button onclick={() => resolveStyleCheck($appState.styleCheck!, 'to_client')}>
          {tValue($locale, 'dialog.config_style.btn.to_client')}
        </Button>
      </AlertDialog.Footer>
    </AlertDialog.Content>
  </AlertDialog.Root>
{/if}

<Toaster position="top-right" theme={$colorScheme} />
