<script lang="ts">
  import { Square, RefreshCw } from '@lucide/svelte';
  import { appState } from '../stores/appState.js';
  import { useLocale, useLocaleRecord } from '@he11ah0und/localengine-web';

  import Page from '../components/Page.svelte';
  import StackedGraph from '../components/StackedGraph.svelte';
  import GraphDetailDialog from '../components/GraphDetailDialog.svelte';
  import * as Card from '$lib/components/ui/card/index.js';
  import * as Select from '$lib/components/ui/select/index.js';
  import { Button } from '$lib/components/ui/button/index.js';
  import { Badge } from '$lib/components/ui/badge/index.js';
  import { Skeleton } from '$lib/components/ui/skeleton/index.js';
  import {
    Stop,
    Restart,
    GetConfigs,
    GetActiveConfig,
    ActivateConfig,
    SetAPIMode
  } from '../../../bindings/sing-box-ez/internal/gui/wails/bindings.js';
  import type { Group as APIGroup } from '../../../bindings/sing-box-ez/internal/core/state/models.js';
  import { formatSpeed, formatTime } from '../utils/format.js';

  // Mode options come from the running core (sing-box reports its accepted
  // clash modes via GetClashModeStatus); the static list is the fallback for
  // cores that do not report one.
  const modes = $derived(
    $appState.api.modeList.length > 0 ? $appState.api.modeList : ['rule', 'global', 'direct']
  );
  // Proxy mode labels: "rule" is shared with connection details via
  // common.rule; the rest keep their own keys.
  const modeKeys: Record<string, string> = {
    rule: 'common.rule',
    global: 'main.api.mode_global',
    direct: 'main.api.mode_direct'
  };

  function modeKey(mode: string): string {
    return modeKeys[mode] ?? `main.api.mode_${mode}`;
  }

  // Property names are derived from the keys: dots camelize, underscores
  // stay ("main.dashboard.upload" → L.mainDashboardUpload).
  const L = useLocale([
    'main.active.placeholder',
    'configs.empty',
    'main.btn.stop',
    'main.btn.restart',
    'main.dashboard.upload',
    'main.dashboard.download',
    'main.dashboard.profile',
    'main.graph.details_title',
    'main.graph.total',
    'main.api.mode',
    'configs.type.remote',
    'configs.type.local'
  ]);

  // Dynamic-key access (mode selector) reads through a record.
  const R = useLocaleRecord([
    'common.rule',
    'main.api.mode_global',
    'main.api.mode_direct'
  ]);

  let processing = $state(false);
  // Core API state arrives from the backend via api:state events; the page
  // never polls.
  const apiPhase = $derived($appState.api.phase);
  const apiStatus = $derived($appState.api.status);
  const apiInfo = $derived($appState.api.info);
  const apiMode = $derived($appState.api.mode);
  const apiGroups = $derived($appState.api.groups);

  // processing covers the window between a stop/restart click and the first
  // phase event; a phase transition (or a binding failure) releases it.
  let lastPhase = $state('');
  $effect(() => {
    if (apiPhase !== lastPhase) {
      lastPhase = apiPhase;
      processing = false;
    }
  });

  // Must stay derived: L.* still holds the raw keys until the locale values
  // arrive, and a plain const would freeze those keys into the badge.
  const configTypeLabels = $derived<Record<string, string>>({
    remote: L.configsTypeRemote,
    local: L.configsTypeLocal
  });
  const activeBadge = $derived(
    $appState.activeConfig ? (configTypeLabels[$appState.activeConfig.type] ?? '') : ''
  );
  const activeUpdated = $derived($appState.activeConfig?.lastUpdateAgo ?? '');

  const configSelectLabel = $derived(
    $appState.activeConfig?.name
      ?? ($appState.configs.length
        ? L.mainActivePlaceholder
        : L.configsEmpty)
  );
  const modeLabel = $derived(R[modeKey(apiMode)]);

  // Refreshes only what the profile card shows; the heavier initial seed
  // (status/settings) is owned by MainPage and the bridge.
  async function reloadConfigs() {
    const [configs, active] = await Promise.all([GetConfigs(), GetActiveConfig()]);
    appState.update((s) => ({ ...s, configs: configs ?? [], activeConfig: active }));
  }

  // Errors are reported by the backend with localized toasts. On success the
  // processing flag is released by the next phase transition (see $effect).
  async function callBinding(promise: Promise<unknown>) {
    processing = true;
    try {
      await promise;
    } catch {
      processing = false;
      // The backend already reported the failure with a toast.
    }
  }

  function handleStop() {
    callBinding(Stop());
  }
  function handleRestart() {
    callBinding(Restart());
  }

  async function activateConfig(name: string) {
    if (!name || name === $appState.activeConfig?.name) return;
    processing = true;
    try {
      await ActivateConfig(name);
      if ($appState.status.running) {
        await Restart();
      }
      await reloadConfigs();
    } catch {
      // The backend already reported the failure with a toast.
    } finally {
      processing = false;
    }
  }

  async function setMode(mode: string) {
    try {
      await SetAPIMode(mode);
    } catch {
      // The backend already reported the failure with a toast.
    }
  }

  function buildOutboundChain(groups: APIGroup[]): { chain: string; delay: string } {
    if (!groups?.length) return { chain: '', delay: '' };
    const groupMap = new Map(groups.map((g) => [g.tag, g]));
    const selectedAsNode = new Map<string, number>();
    for (const g of groups) {
      for (const n of g.nodes ?? []) {
        selectedAsNode.set(n.tag, (selectedAsNode.get(n.tag) ?? 0) + 1);
      }
    }
    const roots = groups.filter((g) => !selectedAsNode.has(g.tag));
    if (!roots.length) return { chain: '', delay: '' };
    roots.sort((a, b) => a.tag.localeCompare(b.tag));
    let g: APIGroup | undefined = roots[0];
    const parts: string[] = [];
    const visited = new Set<string>();
    let delay = '';
    while (g) {
      if (visited.has(g.tag)) break;
      visited.add(g.tag);
      if (g.tag !== 'GLOBAL') parts.push(`${g.tag} (${g.type})`);
      if (!g.selected) break;
      const next = groupMap.get(g.selected);
      if (next) {
        g = next;
        continue;
      }
      const node = (g.nodes ?? []).find((n) => n.tag === g!.selected);
      parts.push(`${g.selected} (${node?.type ?? 'node'})`);
      if (node?.delayValid) delay = `${node.delay} ms`;
      break;
    }
    return { chain: parts.join(' → '), delay };
  }

  const outboundChain = $derived(buildOutboundChain(apiGroups));
  const graphSpan = $derived(Math.max(1, $appState.settings?.['core.traffic_graph_history'] || 60));

  // Overview graph detail dialog; fed the live store arrays so it keeps
  // updating while open.
  let mainGraphOpen = $state(false);
</script>

<Page>
  <!-- API info: right-aligned on desktop, left-aligned above the content on
       narrow screens. -->
  <div class="text-left sm:text-right text-sm text-muted-foreground">
    {#if apiStatus}
      <p>{apiInfo?.backend ?? ''} {apiStatus.version}</p>
      {#if apiStatus.connectedAt}
        <p>{formatTime(new Date(apiStatus.connectedAt))}</p>
        {#if apiStatus.connectedAgo}
          <p>{apiStatus.connectedAgo}</p>
        {/if}
      {/if}
    {/if}
  </div>

  <!-- Traffic graph: stacked upload/download, clickable for details -->
  <Card.Root>
    <Card.Content>
      <button
        class="w-full text-left rounded-xl bg-background border border-border p-4 space-y-2 cursor-pointer hover:bg-accent transition"
        onclick={() => (mainGraphOpen = true)}
      >
        <div class="flex flex-wrap items-center justify-between gap-x-6 gap-y-1">
          <span class="text-sm text-muted-foreground">
            {L.mainDashboardUpload}
            <span class="font-medium text-[var(--color-success)]">{$appState.traffic.upRate}</span>
          </span>
          <span class="text-sm text-muted-foreground">
            {L.mainDashboardDownload}
            <span class="font-medium text-[var(--color-primary)]">{$appState.traffic.downRate}</span>
          </span>
          <span class="text-sm text-muted-foreground">
            {L.mainGraphTotal}
            <span class="font-medium">{formatSpeed($appState.traffic.up + $appState.traffic.down)}</span>
          </span>
        </div>
        <StackedGraph
          up={$appState.traffic.history.up}
          down={$appState.traffic.history.down}
          times={$appState.traffic.history.times}
          span={graphSpan}
        />
      </button>
    </Card.Content>
  </Card.Root>

  <!-- API-dependent blocks: skeletons while the API status is loading -->
  {#if !apiStatus}
    <Card.Root>
      <Card.Content class="space-y-3">
        <Skeleton class="h-5 w-1/3" />
        <Skeleton class="h-10 w-full rounded-xl" />
        <Skeleton class="h-10 w-full rounded-xl" />
      </Card.Content>
    </Card.Root>
  {:else}
    <!-- Outbound chain -->
    {#if outboundChain.chain}
      <Card.Root>
        <Card.Content>
          <div class="flex items-center justify-between gap-4">
            <p class="text-sm break-all">{outboundChain.chain}</p>
            {#if outboundChain.delay}
              <span class="text-[var(--color-success)] text-sm whitespace-nowrap">{outboundChain.delay}</span>
            {/if}
          </div>
        </Card.Content>
      </Card.Root>
    {/if}

    <!-- Mode selector -->
    <Card.Root>
      <Card.Content class="space-y-2">
        <p class="text-sm text-muted-foreground">{L.mainApiMode}</p>
        <Select.Root type="single" value={apiMode} onValueChange={(v) => { if (v) setMode(v); }}>
          <Select.Trigger class="w-full">{modeLabel}</Select.Trigger>
          <Select.Content>
            {#each modes as m (m)}
              <Select.Item value={m} label={R[modeKey(m)]} />
            {/each}
          </Select.Content>
        </Select.Root>
      </Card.Content>
    </Card.Root>

    <!-- Profile card -->
    <Card.Root>
      <Card.Content class="space-y-2">
        <div class="flex items-center justify-between">
          <span class="text-sm text-muted-foreground">{L.mainDashboardProfile}</span>
          {#if activeBadge}
            <Badge variant="outline">{activeBadge}</Badge>
          {/if}
        </div>
        <Select.Root
          type="single"
          value={$appState.activeConfig?.name ?? ''}
          onValueChange={(v) => { if (v) activateConfig(v); }}
          disabled={!$appState.configs.length}
        >
          <Select.Trigger class="w-full">{configSelectLabel}</Select.Trigger>
          <Select.Content>
            {#each $appState.configs as cfg (cfg.name)}
              <Select.Item value={cfg.name} label={cfg.name} />
            {/each}
          </Select.Content>
        </Select.Root>
        {#if activeUpdated}
          <p class="text-sm text-muted-foreground">{activeUpdated}</p>
        {/if}
      </Card.Content>
    </Card.Root>
  {/if}

  <!-- Stop / Restart -->
  <div class="flex flex-wrap gap-3">
    <Button variant="destructive" size="lg" class="flex-1" disabled={processing} onclick={handleStop}>
      <Square size={18} />
      {L.mainBtnStop}
    </Button>
    <Button variant="secondary" size="lg" class="flex-1" disabled={processing} onclick={handleRestart}>
      <RefreshCw size={18} />
      {L.mainBtnRestart}
    </Button>
  </div>
</Page>

<!-- Mounted only while open: the dialog runs a rAF clock, and an
     always-mounted instance would tick 60 times a second in the background. -->
{#if mainGraphOpen}
  <GraphDetailDialog
    open={mainGraphOpen}
    onclose={() => (mainGraphOpen = false)}
    title={L.mainGraphDetails_title}
    up={$appState.traffic.history.up}
    down={$appState.traffic.history.down}
    times={$appState.traffic.history.times}
    span={graphSpan}
  />
{/if}
