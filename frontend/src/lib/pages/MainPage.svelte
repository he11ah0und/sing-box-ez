<script lang="ts">
  import { Square, RefreshCw, ChevronDown, ChevronUp, Zap } from '@lucide/svelte';
  import { toast } from 'svelte-sonner';
  import { appState } from '../stores/appState.js';
  import { useLocale, useLocaleRecord } from '../stores/locale.svelte.js';

  import Page from '../components/Page.svelte';
  import Sparkline from '../components/Sparkline.svelte';
  import * as Dialog from '$lib/components/ui/dialog/index.js';
  import * as Card from '$lib/components/ui/card/index.js';
  import * as Tabs from '$lib/components/ui/tabs/index.js';
  import * as Select from '$lib/components/ui/select/index.js';
  import { Button } from '$lib/components/ui/button/index.js';
  import { Badge } from '$lib/components/ui/badge/index.js';
  import { Skeleton } from '$lib/components/ui/skeleton/index.js';
  import { cn } from '$lib/utils.js';
  import {
    Start,
    Stop,
    Restart,
    GetStatus,
    GetConfigs,
    GetActiveConfig,
    GetCoreInfo,
    GetConfigValues,
    ActivateConfig,
    SetAPIMode,
    SelectAPINode,
    URLTestAPIGroup,
    CloseAPIConnections,
    CloseAPIConnection
  } from '../../../bindings/sing-box-ez/internal/gui/wails/bindings.js';
  import type {
    Status as APIStatus,
    Info as APIInfo,
    Group as APIGroup,
    Node as APINode,
    Connection as APIConnection
  } from '../../../bindings/sing-box-ez/internal/core/state/models.js';
  import { formatBytes, formatSpeed, formatTime, splitHostPort, ipVersionLabel } from '../utils/format.js';

  const tabs = [
    { id: 'overview', key: 'main.tabs.overview' },
    { id: 'groups', key: 'tab.groups' },
    { id: 'connections', key: 'tab.connections' }
  ];
  const modes = ['rule', 'global', 'direct'];
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
  // stay ("connection_details.title" → L.connection_detailsTitle).
  const L = useLocale([
    'main.active.placeholder',
    'configs.empty',
    'main.btn.start',
    'main.btn.stop',
    'main.btn.restart',
    'main.active.label',
    'main.dashboard.upload',
    'main.dashboard.download',
    'main.dashboard.min',
    'main.dashboard.max',
    'main.dashboard.avg',
    'main.dashboard.profile',
    'main.api.mode',
    'tab.groups',
    'main.groups.empty',
    'main.api.connections',
    'main.api.close_connections',
    'main.connections.empty',
    'connection_details.title',
    'connection_details.inbound',
    'connection_details.network',
    'connection_details.source',
    'connection_details.destination',
    'connection_details.domain',
    'common.rule',
    'connection_details.outbound',
    'connection_details.chain',
    'connection_details.uplink',
    'connection_details.downlink',
    'connection_details.created',
    'connection_details.user',
    'connection_details.process',
    'connection_details.close_connection'
  ]);

  // Dynamic-key access (tab triggers, mode selector, phase label) reads
  // through a record; the phase namespace is registered via a wildcard.
  const R = useLocaleRecord([
    'main.tabs.overview',
    'tab.groups',
    'tab.connections',
    'common.rule',
    'main.api.mode_global',
    'main.api.mode_direct',
    'main.phase.*'
  ]);

  let activeTab = $state('overview');
  let processing = $state(false);
  // Core API state arrives from the backend via api:state events; the page
  // never polls. phase: stopped | preparing_config | checking_config |
  // starting | stopping | waiting_api | connected.
  const apiPhase = $derived($appState.api.phase);
  // processing covers the window between the button click and the first
  // phase event; a phase transition (or a binding failure) releases it.
  let lastPhase = $state('');
  $effect(() => {
    if (apiPhase !== lastPhase) {
      lastPhase = apiPhase;
      processing = false;
    }
  });
  const apiStatus = $derived($appState.api.status);
  const apiInfo = $derived($appState.api.info);
  const apiMode = $derived($appState.api.mode);
  const apiGroups = $derived($appState.api.groups);
  const apiConnections = $derived($appState.api.connections);
  let expandedGroups = $state<Set<string>>(new Set());
  let testingGroups = $state<Set<string>>(new Set());
  let groupDelays = $state<Record<string, Record<string, number>>>({});
  let selectedConn = $state<APIConnection | null>(null);

  const activeBadge = $derived($appState.activeConfig?.type ?? '');
  const activeUpdated = $derived($appState.activeConfig?.lastUpdateAgo ?? '');

  const configSelectLabel = $derived(
    $appState.activeConfig?.name
      ?? ($appState.configs.length
        ? L.mainActivePlaceholder
        : L.configsEmpty)
  );
  const modeLabel = $derived(R[modeKey(apiMode)]);

  const visibleGroups = $derived(
    apiGroups.filter((g) => g.type !== 'Fallback' && g.type !== 'LoadBalance')
  );

  async function loadInitial() {
    try {
      const [status, configs, active, coreInfo, settings] = await Promise.all([
        GetStatus(),
        GetConfigs(),
        GetActiveConfig(),
        GetCoreInfo(),
        GetConfigValues()
      ]);
      appState.update((s) => ({
        ...s,
        status: { ...s.status, running: status.running, pid: status.pid },
        configs: configs ?? [],
        activeConfig: active,
        coreInfo: { ...s.coreInfo, ...coreInfo },
        settings: settings ?? s.settings,
        settingsLoaded: true
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
      await loadInitial();
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

  async function selectNode(group: string, node: string) {
    try {
      await SelectAPINode(group, node);
    } catch {
      // The backend already reported the failure with a toast.
    }
  }

  async function testGroup(tag: string) {
    testingGroups = new Set(testingGroups).add(tag);
    try {
      const res = await URLTestAPIGroup(tag);
      groupDelays = { ...groupDelays, [tag]: (res.results ?? {}) as Record<string, number> };
    } catch {
      // The backend already reported the failure with a toast.
    } finally {
      const next = new Set(testingGroups);
      next.delete(tag);
      testingGroups = next;
    }
  }

  function toggleGroup(tag: string) {
    const next = new Set(expandedGroups);
    if (next.has(tag)) next.delete(tag);
    else next.add(tag);
    expandedGroups = next;
  }

  async function closeConnections() {
    try {
      await CloseAPIConnections();
    } catch {
      // The backend already reported the failure with a toast.
    }
  }

  async function closeConnection(id: string) {
    try {
      await CloseAPIConnection(id);
      selectedConn = null;
    } catch {
      // The backend already reported the failure with a toast.
    }
  }

  function formatNodeDelay(n: APINode, groupTag: string): string {
    if (n.delayValid) return `${n.delay} ms`;
    const d = groupDelays[groupTag]?.[n.tag];
    if (d != null) return `${d} ms`;
    return '—';
  }

  function delayColor(n: APINode, groupTag: string): string {
    let d = -1;
    if (n.delayValid) d = n.delay;
    else if (groupDelays[groupTag]?.[n.tag] != null) d = groupDelays[groupTag][n.tag];
    if (d < 0) return '';
    if (d < 300) return 'text-[var(--color-success)]';
    if (d < 800) return 'text-amber-500';
    return 'text-red-500';
  }

  function sparkStats(data: number[] | undefined) {
    if (!data?.length) return { min: 0, max: 0, avg: 0 };
    return {
      min: Math.min(...data),
      max: Math.max(...data),
      avg: data.reduce((a, b) => a + b, 0) / data.length
    };
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
  const upStats = $derived(sparkStats($appState.traffic.history.up));
  const downStats = $derived(sparkStats($appState.traffic.history.down));
  const graphSpan = $derived(Math.max(1, $appState.settings?.['core.traffic_graph_history'] || 60));

  function formatConnectionTarget(conn: APIConnection): string {
    if (conn.domain) {
      const { port } = splitHostPort(conn.destination);
      return port ? `${conn.domain}:${port}` : conn.domain;
    }
    const { host, port, isIP } = splitHostPort(conn.destination);
    if (host) {
      if (port) return isIP ? `[${host}]:${port}` : `${host}:${port}`;
      return host;
    }
    return conn.id?.slice(0, 8) ?? '';
  }

  function formatConnectionSub(conn: APIConnection): string {
    const outbound = conn.outbound || conn.chain?.[conn.chain.length - 1] || '';
    const info = apiInfo;
    let sub;
    if (info?.backend !== 'clash' && (conn.uplink > 0 || conn.downlink > 0)) {
      sub = `↑${formatSpeed(conn.uplink)} ↓${formatSpeed(conn.downlink)} · ${formatBytes(conn.uplinkTotal + conn.downlinkTotal)}`;
    } else {
      sub = `↑${formatBytes(conn.uplinkTotal)} ↓${formatBytes(conn.downlinkTotal)}`;
    }
    if (conn.createdAgo) sub += ` · ${conn.createdAgo}`;
    if (outbound) sub += ` · ${outbound}`;
    return sub;
  }
</script>

<Page onLoad={loadInitial} fullHeight>
  {#if apiPhase !== 'connected'}
    <!-- Button frame: stopped shows an active start button; the transition
         phases (preparing/checking/starting/stopping/waiting) keep the same
         frame with the button busy and the phase label below. The active
         config picker is pinned to the bottom and locked during transitions. -->
    {@const busy = apiPhase !== 'stopped'}
    <div class="flex flex-col flex-1 min-h-0 gap-6">
      <div class="flex flex-1 flex-col items-center justify-center gap-4">
        <button
          class="w-32 h-32 rounded-full bg-primary text-primary-foreground text-lg font-semibold shadow-lg disabled:opacity-50 hover:opacity-90 transition flex items-center justify-center"
          disabled={processing || busy}
          onclick={handleStart}
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
  {:else}
    <!-- Connected state -->
    <Tabs.Root bind:value={activeTab} class="flex flex-col flex-1 min-h-0 gap-6">
      <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 shrink-0">
        <Tabs.List>
          {#each tabs as tab (tab.id)}
            <Tabs.Trigger value={tab.id}>{R[tab.key]}</Tabs.Trigger>
          {/each}
        </Tabs.List>
        <div class="text-right text-sm text-muted-foreground">
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
      </div>

      <Tabs.Content value="overview" class="flex-1 min-h-0 overflow-y-auto space-y-6">
        <!-- Traffic graphs -->
        <Card.Root>
          <Card.Content>
            <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div class="rounded-xl bg-background border border-border p-4 space-y-2">
                <div class="flex items-center justify-between">
                  <span class="text-sm text-muted-foreground">{L.mainDashboardUpload}</span>
                  <span class="text-sm font-medium">{$appState.traffic.upRate}</span>
                </div>
                <Sparkline
                  data={$appState.traffic.history.up}
                  times={$appState.traffic.history.times}
                  span={graphSpan}
                  color="var(--color-success)"
                  fill
                />
                <p class="text-xs text-muted-foreground">
                  {L.mainDashboardMin}: {formatSpeed(upStats.min)}
                  &nbsp;{L.mainDashboardMax}: {formatSpeed(upStats.max)}
                  &nbsp;{L.mainDashboardAvg}: {formatSpeed(upStats.avg)}
                </p>
              </div>
              <div class="rounded-xl bg-background border border-border p-4 space-y-2">
                <div class="flex items-center justify-between">
                  <span class="text-sm text-muted-foreground">{L.mainDashboardDownload}</span>
                  <span class="text-sm font-medium">{$appState.traffic.downRate}</span>
                </div>
                <Sparkline
                  data={$appState.traffic.history.down}
                  times={$appState.traffic.history.times}
                  span={graphSpan}
                  color="var(--color-primary)"
                  fill
                />
                <p class="text-xs text-muted-foreground">
                  {L.mainDashboardMin}: {formatSpeed(downStats.min)}
                  &nbsp;{L.mainDashboardMax}: {formatSpeed(downStats.max)}
                  &nbsp;{L.mainDashboardAvg}: {formatSpeed(downStats.avg)}
                </p>
              </div>
            </div>
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
      </Tabs.Content>

      <Tabs.Content value="groups" class="flex-1 min-h-0 overflow-y-auto">
        <Card.Root>
          <Card.Header>
            <Card.Title>{L.tabGroups}</Card.Title>
          </Card.Header>
          <Card.Content class="space-y-4">
            {#if !apiStatus}
              <div class="space-y-3">
                <Skeleton class="h-16 w-full rounded-xl" />
                <Skeleton class="h-16 w-full rounded-xl" />
                <Skeleton class="h-16 w-full rounded-xl" />
              </div>
            {:else if !visibleGroups.length}
              <p class="text-muted-foreground">{L.mainGroupsEmpty}</p>
            {:else}
              {#each visibleGroups as group (group.tag)}
                <div class="rounded-xl border border-border bg-background overflow-hidden">
                  <div
                    class="w-full px-4 py-3 flex items-center justify-between hover:bg-accent transition cursor-pointer"
                    onclick={() => toggleGroup(group.tag)}
                    role="button"
                    tabindex="0"
                    onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') toggleGroup(group.tag); }}
                  >
                    <div class="text-left">
                      <p class="font-medium">{group.tag}</p>
                      <p class="text-sm text-muted-foreground">{group.selected || '—'}</p>
                    </div>
                    <div class="flex items-center gap-2">
                      <Badge variant="outline">{group.type}</Badge>
                      {#if group.delayValid}
                        <Badge variant="outline">{group.delay} ms</Badge>
                      {/if}
                      {#if group.type !== 'URLTest' && group.tag !== 'GLOBAL'}
                        <Button
                          variant="ghost"
                          size="icon-sm"
                          disabled={testingGroups.has(group.tag)}
                          onclick={(e) => { e.stopPropagation(); testGroup(group.tag); }}
                        >
                          {#if testingGroups.has(group.tag)}
                            <RefreshCw size={16} class="animate-spin" />
                          {:else}
                            <Zap size={16} />
                          {/if}
                        </Button>
                      {/if}
                      {#if expandedGroups.has(group.tag)}
                        <ChevronUp size={18} />
                      {:else}
                        <ChevronDown size={18} />
                      {/if}
                    </div>
                  </div>
                  {#if expandedGroups.has(group.tag)}
                    <div class="divide-y divide-border">
                      {#each group.nodes ?? [] as node (node.tag)}
                        <button
                          class={cn(
                            'w-full px-4 py-2 flex items-center justify-between hover:bg-accent transition',
                            node.tag === group.selected && 'bg-primary text-primary-foreground hover:bg-primary/90'
                          )}
                          disabled={group.type !== 'Selector'}
                          onclick={() => selectNode(group.tag, node.tag)}
                        >
                          <span class="truncate">{node.tag}</span>
                          <span class="text-sm whitespace-nowrap {delayColor(node, group.tag)}">{formatNodeDelay(node, group.tag)}</span>
                        </button>
                      {/each}
                    </div>
                  {/if}
                </div>
              {/each}
            {/if}
          </Card.Content>
        </Card.Root>
      </Tabs.Content>

      <Tabs.Content value="connections" class="flex-1 min-h-0 overflow-y-auto">
        <Card.Root>
          <Card.Header>
            <div class="flex items-center justify-between">
              <Card.Title>
                {L.mainApiConnections} ({apiConnections.length})
              </Card.Title>
              <Button variant="outline" size="sm" onclick={closeConnections}>
                {L.mainApiClose_connections}
              </Button>
            </div>
          </Card.Header>
          <Card.Content>
            {#if !apiConnections.length}
              <p class="text-muted-foreground">{L.mainConnectionsEmpty}</p>
            {:else}
              <div class="space-y-2">
                {#each apiConnections as conn (conn.id)}
                  <button
                    class="w-full text-left rounded-xl border border-border bg-background p-3 hover:bg-accent transition"
                    onclick={() => selectedConn = conn}
                  >
                    <div class="flex items-center justify-between gap-3">
                      <span class="font-medium truncate">{formatConnectionTarget(conn)}</span>
                      {#if ipVersionLabel(conn.destination)}
                        <Badge variant="outline" class="whitespace-nowrap">{ipVersionLabel(conn.destination)}</Badge>
                      {/if}
                    </div>
                    <p class="text-sm text-muted-foreground truncate mt-1">{formatConnectionSub(conn)}</p>
                  </button>
                {/each}
              </div>
            {/if}
          </Card.Content>
        </Card.Root>
      </Tabs.Content>
    </Tabs.Root>
  {/if}

</Page>

<Dialog.Root open={selectedConn != null} onOpenChange={(open) => { if (!open) selectedConn = null; }}>
  <Dialog.Content class="sm:max-w-lg">
    <Dialog.Header>
      <Dialog.Title>{L.connection_detailsTitle}</Dialog.Title>
    </Dialog.Header>
    {#if selectedConn}
      {@const c = selectedConn}
      {@const inbound = c.inbound || c.inboundType || '—'}
      {@const outbound = c.outbound || c.outboundType || '—'}
      <div class="space-y-3 text-sm">
        {@render DetailRow('ID', c.id)}
        {@render DetailRow(L.connection_detailsInbound, inbound)}
        {@render DetailRow(L.connection_detailsNetwork, c.network)}
        {@render DetailRow(L.connection_detailsSource, c.source)}
        {@render DetailRow(L.connection_detailsDestination, c.destination)}
        {@render DetailRow(L.connection_detailsDomain, c.domain)}
        {@render DetailRow(L.commonRule, c.rule)}
        {@render DetailRow(L.connection_detailsOutbound, outbound)}
        {@render DetailRow(L.connection_detailsChain, c.chain?.join(' → '))}
        {@render DetailRow(L.connection_detailsUplink, `${formatSpeed(c.uplink)} (${formatBytes(c.uplinkTotal)})`)}
        {@render DetailRow(L.connection_detailsDownlink, `${formatSpeed(c.downlink)} (${formatBytes(c.downlinkTotal)})`)}
        {@render DetailRow(L.connection_detailsCreated, formatTime(c.createdAt) + (c.createdAgo ? ` (${c.createdAgo})` : ''))}
        {#if c.processInfo?.userName}
          {@render DetailRow(L.connection_detailsUser, c.processInfo.userName)}
          {@render DetailRow(L.connection_detailsProcess, c.processInfo.processPath)}
        {/if}
      </div>
      <Dialog.Footer>
        <Button variant="destructive" onclick={() => closeConnection(c.id)}>
          {L.connection_detailsClose_connection}
        </Button>
      </Dialog.Footer>
    {/if}
  </Dialog.Content>
</Dialog.Root>

{#snippet DetailRow(label: string, value: string | null | undefined)}
  {#if value}
    <div class="flex items-start justify-between gap-4">
      <span class="text-muted-foreground whitespace-nowrap">{label}</span>
      <span class="break-all text-right">{value}</span>
    </div>
  {/if}
{/snippet}
