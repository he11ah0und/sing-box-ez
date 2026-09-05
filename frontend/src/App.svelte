<script lang="ts">
  import { onMount, tick } from 'svelte';
  import type { Component } from 'svelte';
  import { fly } from 'svelte/transition';
  import { RefreshCw } from '@lucide/svelte';
  import { initWailsEvents } from '$lib/wails/bridge.js';
  import { theme, applyTheme, colorScheme, fromThemePayload } from '$lib/stores/theme.js';
  import { signalLocaleReady, useLocale, useLocaleRecord, format } from '@he11ah0und/localengine-web';
  import { GetTheme, SetFallbackType, QuitApp, GetOrphanedConfigs, DeleteOrphanedConfigs } from '../bindings/sing-box-ez/internal/gui/wails/bindings.js';
  import type { OrphanedConfig } from '../bindings/sing-box-ez/internal/core/models.js';
  import { currentLevel, enterSubNav, setSubTab, setRootPage } from '$lib/stores/navigation.js';
  import { appState, type StyleCheckState } from '$lib/stores/appState.js';
  import Shell from '$lib/components/Shell.svelte';
  import StartupPage from '$lib/pages/StartupPage.svelte';
  import { loadPage, pageRegistry } from '$lib/pages/index.js';
  import { Toaster } from '$lib/components/ui/sonner/index.js';
  import * as Dialog from '$lib/components/ui/dialog/index.js';
  import * as AlertDialog from '$lib/components/ui/alert-dialog/index.js';
  import { Button } from '$lib/components/ui/button/index.js';
  import * as Tooltip from '$lib/components/ui/tooltip/index.js';

  let ActivePage = $state<Component | null>(null);
  let loadToken = 0;
  // Cached config files left behind by deleted profiles; shown once per
  // launch in a dialog offering to remove them.
  let orphanedConfigs = $state<OrphanedConfig[]>([]);
  let orphanDeleting = $state(false);

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
      // One-shot startup scan for config cache leftovers; the dialog offers
      // to delete or keep them.
      GetOrphanedConfigs()
        .then((list) => {
          if (list && list.length > 0) orphanedConfigs = list;
        })
        .catch(() => {});
      tick().then(() => setTimeout(signalLocaleReady, 0));
    } catch (err) {
      console.error('Failed to init Wails events:', err);
      appState.update((s) => ({ ...s, ready: true }));
    }
  });

  function closeDialog() {
    appState.update((s) => ({ ...s, dialog: null }));
  }

  async function deleteOrphanedConfigs() {
    orphanDeleting = true;
    try {
      await DeleteOrphanedConfigs(orphanedConfigs.map((o) => o.name));
      orphanedConfigs = [];
    } catch {
      // The backend already reported the failure with a toast.
    } finally {
      orphanDeleting = false;
    }
  }

  function orphanDate(o: OrphanedConfig): string {
    if (!o.modTime) return '—';
    const d = new Date(o.modTime);
    return isNaN(d.getTime()) ? '—' : d.toLocaleString();
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
    'common.quit',
    'configs.btn.delete',
    'dialog.config_style.btn.ignore',
    'dialog.config_style.btn.to_client',
    'dialog.privileges_required.title',
    'dialog.privileges_required.body',
    'dialog.privileges_required.btn_settings',
    'dialog.channel_error.title',
    'dialog.channel_error.body',
    'dialog.channel_error.btn_continue',
    'dialog.orphan_configs.title',
    'dialog.orphan_configs.body',
    'dialog.orphan_configs.btn_keep'
  ]);

  // The channel-error dialog is not dismissible by click-away/Escape: the
  // user must pick Quit or Continue anyway explicitly.
  function closeChannelError() {
    appState.update((s) => ({ ...s, updateChannelError: null }));
  }

  function closePrivilegesDialog() {
    appState.update((s) => ({ ...s, privilegesRequired: false }));
  }

  // Leads to the settings tab that fixes the missing privilege (setcap on
  // Linux, restart-as-admin on Windows).
  function openPrivilegeSettings() {
    closePrivilegesDialog();
    const settings = pageRegistry.find((p) => p.id === 'settings');
    enterSubNav('settings', settings?.tabs ?? []);
    setSubTab('system');
    setRootPage('settings');
  }
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

{#if $appState.privilegesRequired}
  <AlertDialog.Root open={true} onOpenChange={(open) => { if (!open) closePrivilegesDialog(); }}>
    <AlertDialog.Content>
      <AlertDialog.Header>
        <AlertDialog.Title>{L.dialogPrivileges_requiredTitle}</AlertDialog.Title>
        <AlertDialog.Description>
          {L.dialogPrivileges_requiredBody}
        </AlertDialog.Description>
      </AlertDialog.Header>
      <AlertDialog.Footer>
        <Button variant="outline" onclick={closePrivilegesDialog}>
          {L.commonCancel}
        </Button>
        <Button onclick={openPrivilegeSettings}>
          {L.dialogPrivileges_requiredBtn_settings}
        </Button>
      </AlertDialog.Footer>
    </AlertDialog.Content>
  </AlertDialog.Root>
{/if}

{#if $appState.updateChannelError}
  <AlertDialog.Root open={true}>
    <AlertDialog.Content>
      <AlertDialog.Header>
        <AlertDialog.Title class="text-destructive">
          {L.dialogChannel_errorTitle}
        </AlertDialog.Title>
        <AlertDialog.Description>
          {format(L.dialogChannel_errorBody, {
            channel: $appState.updateChannelError.channel,
            error: $appState.updateChannelError.error
          })}
        </AlertDialog.Description>
      </AlertDialog.Header>
      <AlertDialog.Footer>
        <Button variant="outline" onclick={closeChannelError}>
          {L.dialogChannel_errorBtn_continue}
        </Button>
        <Button variant="destructive" onclick={() => QuitApp()}>
          {L.commonQuit}
        </Button>
      </AlertDialog.Footer>
    </AlertDialog.Content>
  </AlertDialog.Root>
{/if}

{#if orphanedConfigs.length > 0}
  <AlertDialog.Root open={true} onOpenChange={(open) => { if (!open) orphanedConfigs = []; }}>
    <AlertDialog.Content>
      <AlertDialog.Header>
        <AlertDialog.Title>{L.dialogOrphan_configsTitle}</AlertDialog.Title>
        <AlertDialog.Description>
          {format(L.dialogOrphan_configsBody, { count: orphanedConfigs.length })}
        </AlertDialog.Description>
      </AlertDialog.Header>
      <ul class="max-h-60 overflow-auto divide-y divide-border rounded-xl border border-border">
        {#each orphanedConfigs as orphan (orphan.name)}
          <li class="flex items-center justify-between gap-3 px-3 py-2 text-sm">
            <span class="font-medium truncate">{orphan.name}</span>
            <span class="text-muted-foreground shrink-0">{orphanDate(orphan)}</span>
          </li>
        {/each}
      </ul>
      <AlertDialog.Footer>
        <Button variant="outline" onclick={() => (orphanedConfigs = [])}>
          {L.dialogOrphan_configsBtn_keep}
        </Button>
        <Button variant="destructive" disabled={orphanDeleting} onclick={deleteOrphanedConfigs}>
          {L.configsBtnDelete}
        </Button>
      </AlertDialog.Footer>
    </AlertDialog.Content>
  </AlertDialog.Root>
{/if}

<Toaster position="top-right" theme={$colorScheme} />
