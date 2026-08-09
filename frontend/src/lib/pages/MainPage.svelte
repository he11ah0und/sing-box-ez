<script lang="ts">
  import { RefreshCw } from '@lucide/svelte';
  import { toast } from 'svelte-sonner';
  import { fly } from 'svelte/transition';
  import { appState } from '../stores/appState.js';
  import { subNav } from '../stores/navigation.js';
  import { useLocale, useLocaleRecord } from '../stores/locale.svelte.js';

  import Page from '../components/Page.svelte';
  import * as Card from '$lib/components/ui/card/index.js';
  import * as Select from '$lib/components/ui/select/index.js';
  import OverviewPage from './OverviewPage.svelte';
  import GroupsPage from './GroupsPage.svelte';
  import ConnectionsPage from './ConnectionsPage.svelte';
  import {
    Start,
    Restart,
    GetStatus,
    GetConfigs,
    GetActiveConfig,
    ActivateConfig
  } from '../../../bindings/sing-box-ez/internal/gui/wails/bindings.js';

  // Property names are derived from the keys: dots camelize, underscores
  // stay ("main.btn.start" → L.mainBtnStart).
  const L = useLocale([
    'main.active.placeholder',
    'configs.empty',
    'main.btn.start',
    'main.active.label'
  ]);

  // The phase namespace is registered via a wildcard; the label below the
  // button reads R.mainPhase[apiPhase].
  const R = useLocaleRecord(['main.phase.*']);

  let processing = $state(false);
  // Core API state arrives from the backend via api:state events; the page
  // never polls. phase: stopped | preparing_config | checking_config |
  // starting | stopping | waiting_api | connected.
  const apiPhase = $derived($appState.api.phase);
  const connected = $derived(apiPhase === 'connected');
  // While connected, the main page is a tab container: the active sub-nav
  // tab picks which sub-page renders (overview by default).
  const activeTab = $derived($subNav.activeTab ?? 'overview');
  // processing covers the window between the button click and the first
  // phase event; a phase transition (or a binding failure) releases it.
  let lastPhase = $state('');
  $effect(() => {
    if (apiPhase !== lastPhase) {
      lastPhase = apiPhase;
      processing = false;
    }
  });

  const configSelectLabel = $derived(
    $appState.activeConfig?.name
      ?? ($appState.configs.length
        ? L.mainActivePlaceholder
        : L.configsEmpty)
  );

  async function loadInitial() {
    try {
      const [status, configs, active] = await Promise.all([
        GetStatus(),
        GetConfigs(),
        GetActiveConfig()
      ]);
      appState.update((s) => ({
        ...s,
        status: { ...s.status, running: status.running, pid: status.pid },
        configs: configs ?? [],
        activeConfig: active
      }));
    } catch (err) {
      toast.error(String(err));
    }
  }

  // Errors are reported by the backend with localized toasts. On success the
  // processing flag is released by the next phase transition (see $effect).
  async function callBinding(promise: Promise<unknown>) {
    processing = true;
    try {
      await promise;
      await loadInitial();
    } catch {
      processing = false;
      // The backend already reported the failure with a toast.
    }
  }

  function handleStart() {
    callBinding(Start());
  }

  async function activateConfig(name: string) {
    if (!name || name === $appState.activeConfig?.name) return;
    processing = true;
    try {
      await ActivateConfig(name);
      if ($appState.status.running) {
        await Restart();
      }
      await loadInitial();
    } catch {
      // The backend already reported the failure with a toast.
    } finally {
      processing = false;
    }
  }
</script>

{#if connected}
  <!-- Connected: main acts as a tab container; each tab is a full sub-page
       with its own Page wrapper, swapped with the same rise-and-fade
       transition as the root pages. -->
  {#key activeTab}
    <div class="h-full min-h-0" in:fly={{ y: 8, duration: 150 }}>
      {#if activeTab === 'groups'}
        <GroupsPage />
      {:else if activeTab === 'connections'}
        <ConnectionsPage />
      {:else}
        <OverviewPage />
      {/if}
    </div>
  {/key}
{:else}
<Page onLoad={loadInitial} fullHeight>
  <!-- Button frame: stopped shows an active start button; the transition
       phases (preparing/checking/starting/stopping/waiting) keep the same
       frame with the button busy and the phase label below. The active
       config picker is pinned to the bottom and locked during
       transitions. -->
  {@const busy = apiPhase !== 'stopped'}
  <div class="flex flex-col flex-1 min-h-0 gap-6">
    <div class="flex flex-1 flex-col items-center justify-center gap-4">
      <!-- Class interpolation stays inside the plain class attribute (the
           i18n hardcode-scan flags multi-word literals in cn() calls). -->
      <button
        class="w-32 h-32 rounded-full text-lg font-semibold shadow-lg disabled:opacity-50 hover:opacity-90 transition flex items-center justify-center bg-primary text-primary-foreground"
        disabled={processing || busy}
        onclick={handleStart}
        aria-label={L.mainBtnStart}
      >
        {#if processing || busy}
          <RefreshCw size={32} class="animate-spin" />
        {:else}
          {L.mainBtnStart}
        {/if}
      </button>
      {#if busy}
        <p class="text-sm text-muted-foreground">{R.mainPhase[apiPhase]}</p>
      {/if}
    </div>

    <Card.Root class="shrink-0">
      <Card.Content class="space-y-2">
        <p class="text-sm text-muted-foreground">{L.mainActiveLabel}</p>
        <Select.Root
          type="single"
          value={$appState.activeConfig?.name ?? ''}
          onValueChange={(v) => { if (v) activateConfig(v); }}
          disabled={!$appState.configs.length || busy}
        >
          <Select.Trigger class="w-full">{configSelectLabel}</Select.Trigger>
          <Select.Content>
            {#each $appState.configs as cfg (cfg.name)}
              <Select.Item value={cfg.name} label={cfg.name} />
            {/each}
          </Select.Content>
        </Select.Root>
      </Card.Content>
    </Card.Root>
  </div>
</Page>
{/if}
