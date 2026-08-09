<script lang="ts">
  import { onMount, tick } from 'svelte';
  import type { Component } from 'svelte';
  import { fly } from 'svelte/transition';
  import { RefreshCw } from '@lucide/svelte';
  import { initWailsEvents } from '$lib/wails/bridge.js';
  import { theme, applyTheme, colorScheme, fromThemePayload } from '$lib/stores/theme.js';
  import { signalLocaleReady, useLocale, useLocaleRecord } from '$lib/stores/locale.svelte.js';
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
      const seeds = initWailsEvents();
      const themeSeed = GetTheme()
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
      // The shell stays behind a loading state until the initial backend
      // data (traffic history, API state, settings, theme) has settled.
      Promise.allSettled([seeds, themeSeed]).then(() => {
        appState.update((s) => ({ ...s, ready: true }));
      });
      tick().then(() => setTimeout(signalLocaleReady, 0));
    } catch (err) {
      console.error('Failed to init Wails events:', err);
      appState.update((s) => ({ ...s, ready: true }));
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

  // Config style texts: the style set is fixed (server/undefined), anything
  // else falls back to the unknown_* keys.
  const R = useLocaleRecord([
    'dialog.config_style.server_title',
    'dialog.config_style.server_body',
    'dialog.config_style.undefined_title',
    'dialog.config_style.undefined_body',
    'dialog.config_style.unknown_title',
    'dialog.config_style.unknown_body'
  ]);

  function styleText(suffix: 'title' | 'body'): string {
    const check = $appState.styleCheck;
    if (!check) return '';
    const specific = `dialog.config_style.${check.style}_${suffix}`;
    const value = R[specific];
    return value !== undefined && value !== specific
      ? value
      : R[`dialog.config_style.unknown_${suffix}`];
  }

  const styleCheckTitle = $derived.by(() => styleText('title'));
  const styleCheckBody = $derived.by(() => styleText('body'));

  const L = useLocale([
    'common.cancel',
    'dialog.config_style.btn.ignore',
    'dialog.config_style.btn.to_client'
  ]);
</script>

<Tooltip.Provider>
  {#if $appState.startup.show}
    <StartupPage />
  {:else if !$appState.ready}
    <!-- Initial backend seeds are in flight. -->
    <div class="flex h-full w-full items-center justify-center bg-background">
      <RefreshCw size={32} class="animate-spin text-muted-foreground" />
    </div>
  {:else}
    <Shell>
      <!-- Light page transition: a short rise-and-fade per root page. -->
      {#key $currentLevel.id}
        <div class="h-full min-h-0" in:fly={{ y: 8, duration: 150 }}>
          {#if ActivePage}
            <ActivePage />
          {/if}
        </div>
      {/key}
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
          {L.commonCancel}
        </Button>
        <Button variant="outline" onclick={() => resolveStyleCheck($appState.styleCheck!, 'ignore')}>
          {L.dialogConfig_styleBtnIgnore}
        </Button>
        <Button onclick={() => resolveStyleCheck($appState.styleCheck!, 'to_client')}>
          {L.dialogConfig_styleBtnTo_client}
        </Button>
      </AlertDialog.Footer>
    </AlertDialog.Content>
  </AlertDialog.Root>
{/if}

<Toaster position="top-right" theme={$colorScheme} />
