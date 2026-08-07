<script module lang="ts">
  import { Home } from '@lucide/svelte';
  export const pageMeta = {
    id: 'main',
    key: 'tab.main',
    icon: Home,
    nav: true,
    bottomNav: true,
    order: 0
  };
</script>

<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { Square, RefreshCw, ChevronDown, ChevronUp, Zap } from '@lucide/svelte';
  import { appState } from '../stores/appState.js';
  import { locale, tValue } from '../stores/locale.js';

  import Page from '../components/Page.svelte';
  import Sparkline from '../components/Sparkline.svelte';
  import * as Dialog from '$lib/components/ui/dialog/index.js';
  import * as Card from '$lib/components/ui/card/index.js';
  import * as Tabs from '$lib/components/ui/tabs/index.js';
  import * as Select from '$lib/components/ui/select/index.js';
  import { Button } from '$lib/components/ui/button/index.js';
  import { Badge } from '$lib/components/ui/badge/index.js';
  import { cn } from '$lib/utils.js';
  import {
    Start,
    Stop,
    Restart,
    GetStatus,
    GetConfigs,
    GetActiveConfig,
    GetCoreInfo,
    GetSettings,
    ActivateConfig,
    GetAPIMode,
    SetAPIMode,
    GetAPIGroups,
    GetAPIConnections,
    GetAPIStatus,
    GetAPIInfo,
    SelectAPINode,
    URLTestAPIGroup,
    CloseAPIConnections,
    CloseAPIConnection,
    type APIStatus,
    type APIInfo,
    type APIGroup,
    type APINode,
    type APIConnection
  } from '../../../bindings/sing-box-ez/internal/gui/wails/bindings.js';
  import { formatBytes, formatSpeed, formatRelative, formatTime, splitHostPort, ipVersionLabel } from '../utils/format.js';

  const tabs = [
    { id: 'overview', key: 'main.tabs.overview' },
    { id: 'groups', key: 'main.tabs.groups' },
    { id: 'connections', key: 'main.tabs.connections' }
  ];
  const modes = ['rule', 'global', 'direct'];

  let activeTab = $state('overview');
  let processing = $state(false);
  let message = $state('');
  let apiStatus = $state<APIStatus | null>(null);
  let apiInfo = $state<APIInfo | null>(null);
  let apiMode = $state('');
  let apiGroups = $state<APIGroup[]>([]);
  let apiConnections = $state<APIConnection[]>([]);
  let expandedGroups = $state<Set<string>>(new Set());
  let testingGroups = $state<Set<string>>(new Set());
  let groupDelays = $state<Record<string, Record<string, number>>>({});
  let selectedConn = $state<APIConnection | null>(null);
  let pollTimer = $state<ReturnType<typeof setInterval> | null>(null);
  let connectedAt = $state<number | null>(null);

  const statusLabel = $derived(
    $appState.status.running
      ? tValue($locale, 'main.running', 'Running')
      : tValue($locale, 'main.stopped', 'Stopped')
  );
  const activeName = $derived($appState.activeConfig?.name ?? '—');
  const activeBadge = $derived($appState.activeConfig?.type ?? '');
  const activeUpdated = $derived(formatRelative($appState.activeConfig?.last_update));

  const configSelectLabel = $derived(
    $appState.activeConfig?.name
      ?? ($appState.configs.length
        ? tValue($locale, 'main.active.placeholder', 'Select active config')
        : tValue($locale, 'configs.empty', 'No configs'))
  );
  const modeLabel = $derived(tValue($locale, `main.api.mode_${apiMode}`, apiMode));

  const visibleGroups = $derived(
    apiGroups.filter((g) => g.type !== 'Fallback' && g.type !== 'LoadBalance')
  );

  onMount(() => {
    pollTimer = setInterval(pollAPI, 2000);
  });

  onDestroy(() => {
    if (pollTimer) clearInterval(pollTimer);
  });

  async function loadInitial() {
    try {
      const [status, configs, active, coreInfo, settings] = await Promise.all([
        GetStatus(),
        GetConfigs(),
        GetActiveConfig(),
        GetCoreInfo(),
        GetSettings()
      ]);
      appState.update((s) => ({
        ...s,
        status: { ...s.status, running: status.running, pid: status.pid },
        configs: configs ?? [],
        activeConfig: active,
        coreInfo: { ...s.coreInfo, ...coreInfo },
        settings: settings as unknown as Record<string, unknown>
      }));
      if (status.running) await pollAPI();
    } catch (err) {
      message = String(err);
    }
  }

  async function pollAPI() {
    if (!$appState.status.running) {
      apiStatus = null;
      apiInfo = null;
      apiGroups = [];
      apiConnections = [];
      return;
    }
    try {
      const [status, info, mode, groups, conns] = await Promise.all([
        GetAPIStatus().catch(() => null),
        GetAPIInfo().catch(() => null),
        GetAPIMode().catch(() => ''),
        GetAPIGroups().catch(() => []),
        GetAPIConnections().catch(() => [])
      ]);
      if (status?.version) {
        if (!connectedAt) connectedAt = Date.now();
      } else {
        connectedAt = null;
      }
      apiStatus = status;
      apiInfo = info;
      apiMode = mode;
      apiGroups = groups ?? [];
      apiConnections = conns ?? [];
    } catch {
      // Suppress repeated polling errors; they are visible when the API is down.
    }
  }

  async function callBinding(promise: Promise<unknown>, action: string) {
    processing = true;
    message = '';
    try {
      await promise;
      await loadInitial();
    } catch (err) {
      message = `${action}: ${err}`;
    } finally {
      processing = false;
    }
  }

  function handleStart() {
    callBinding(Start(), tValue($locale, 'main.btn.start', 'Start'));
  }
  function handleStop() {
    callBinding(Stop(), tValue($locale, 'main.btn.stop', 'Stop'));
  }
  function handleRestart() {
    callBinding(Restart(), tValue($locale, 'main.btn.restart', 'Restart'));
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
      await pollAPI();
    } catch (err) {
      message = String(err);
    } finally {
      processing = false;
    }
  }

  async function setMode(mode: string) {
    try {
      await SetAPIMode(mode);
      await pollAPI();
    } catch (err) {
      message = String(err);
    }
  }

  async function selectNode(group: string, node: string) {
    try {
      await SelectAPINode(group, node);
      await pollAPI();
    } catch (err) {
      message = String(err);
    }
  }

  async function testGroup(tag: string) {
    testingGroups = new Set(testingGroups).add(tag);
    try {
      const res = await URLTestAPIGroup(tag);
      groupDelays = { ...groupDelays, [tag]: res.results ?? {} };
    } catch (err) {
      message = String(err);
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
      await pollAPI();
    } catch (err) {
      message = String(err);
    }
  }

  async function closeConnection(id: string) {
    try {
      await CloseAPIConnection(id);
      selectedConn = null;
      await pollAPI();
    } catch (err) {
      message = String(err);
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
    if (conn.createdAt) sub += ` · ${formatRelative(conn.createdAt)}`;
    if (outbound) sub += ` · ${outbound}`;
    return sub;
  }
</script>

<Page onLoad={loadInitial}>
  {#if !$appState.status.running}
    <!-- Stopped state -->
    <div class="flex flex-col items-center justify-center min-h-[40vh] gap-8">
      <button
        class="w-32 h-32 rounded-full bg-primary text-primary-foreground text-lg font-semibold shadow-lg disabled:opacity-50 hover:opacity-90 transition flex items-center justify-center"
        disabled={processing}
        onclick={handleStart}
      >
        {#if processing}
          <RefreshCw size={32} class="animate-spin" />
        {:else}
          {tValue($locale, 'main.btn.start', 'Start')}
        {/if}
      </button>
      <p class="text-muted-foreground">{statusLabel}</p>
    </div>

    <Card.Root>
      <Card.Content class="space-y-2">
        <p class="text-sm text-muted-foreground">{tValue($locale, 'main.active.label', 'Active config')}</p>
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
      </Card.Content>
    </Card.Root>
  {:else}
    <!-- Running state -->
    <Tabs.Root bind:value={activeTab} class="space-y-6">
      <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
        <Tabs.List>
          {#each tabs as tab (tab.id)}
            <Tabs.Trigger value={tab.id}>{tValue($locale, tab.key, tab.id)}</Tabs.Trigger>
          {/each}
        </Tabs.List>
        <div class="text-right text-sm text-muted-foreground">
          {#if apiStatus}
            <p>{apiInfo?.backend ?? ''} {apiStatus.version}</p>
            {#if connectedAt}
              <p>{formatTime(new Date(connectedAt))}</p>
            {/if}
          {:else}
            <p>{tValue($locale, 'main.api.connecting', 'Connecting…')}</p>
          {/if}
        </div>
      </div>

      <Tabs.Content value="overview" class="space-y-6">
        <!-- Traffic graphs -->
        <Card.Root>
          <Card.Content>
            <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div class="rounded-xl bg-background border border-border p-4 space-y-2">
                <div class="flex items-center justify-between">
                  <span class="text-sm text-muted-foreground">{tValue($locale, 'main.dashboard.upload', 'Upload')}</span>
                  <span class="text-sm font-medium">{$appState.traffic.upRate}</span>
                </div>
                <Sparkline data={$appState.traffic.history.up} color="var(--color-success)" fill />
                <p class="text-xs text-muted-foreground">
                  {tValue($locale, 'main.dashboard.min', 'Min')}: {formatSpeed(upStats.min)}
                  &nbsp;{tValue($locale, 'main.dashboard.max', 'Max')}: {formatSpeed(upStats.max)}
                  &nbsp;{tValue($locale, 'main.dashboard.avg', 'Avg')}: {formatSpeed(upStats.avg)}
                </p>
              </div>
              <div class="rounded-xl bg-background border border-border p-4 space-y-2">
                <div class="flex items-center justify-between">
                  <span class="text-sm text-muted-foreground">{tValue($locale, 'main.dashboard.download', 'Download')}</span>
                  <span class="text-sm font-medium">{$appState.traffic.downRate}</span>
                </div>
                <Sparkline data={$appState.traffic.history.down} color="var(--color-primary)" fill />
                <p class="text-xs text-muted-foreground">
                  {tValue($locale, 'main.dashboard.min', 'Min')}: {formatSpeed(downStats.min)}
                  &nbsp;{tValue($locale, 'main.dashboard.max', 'Max')}: {formatSpeed(downStats.max)}
                  &nbsp;{tValue($locale, 'main.dashboard.avg', 'Avg')}: {formatSpeed(downStats.avg)}
                </p>
              </div>
            </div>
          </Card.Content>
        </Card.Root>

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
            <p class="text-sm text-muted-foreground">{tValue($locale, 'main.api.mode', 'Mode')}</p>
            <Select.Root type="single" value={apiMode} onValueChange={(v) => { if (v) setMode(v); }}>
              <Select.Trigger class="w-full">{modeLabel}</Select.Trigger>
              <Select.Content>
                {#each modes as m (m)}
                  <Select.Item value={m} label={tValue($locale, `main.api.mode_${m}`, m)} />
                {/each}
              </Select.Content>
            </Select.Root>
          </Card.Content>
        </Card.Root>

        <!-- Profile card -->
        <Card.Root>
          <Card.Content class="space-y-2">
            <div class="flex items-center justify-between">
              <span class="text-sm text-muted-foreground">{tValue($locale, 'main.dashboard.profile', 'Profile')}</span>
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

        <!-- Stop / Restart -->
        <div class="flex flex-wrap gap-3">
          <Button variant="destructive" size="lg" class="flex-1" disabled={processing} onclick={handleStop}>
            <Square size={18} />
            {tValue($locale, 'main.btn.stop', 'Stop')}
          </Button>
          <Button variant="secondary" size="lg" class="flex-1" disabled={processing} onclick={handleRestart}>
            <RefreshCw size={18} />
            {tValue($locale, 'main.btn.restart', 'Restart')}
          </Button>
        </div>
      </Tabs.Content>

      <Tabs.Content value="groups">
        <Card.Root>
          <Card.Header>
            <Card.Title>{tValue($locale, 'main.groups.title', 'Groups')}</Card.Title>
          </Card.Header>
          <Card.Content class="space-y-4">
            {#if !visibleGroups.length}
              <p class="text-muted-foreground">{tValue($locale, 'main.groups.empty', 'No groups')}</p>
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

      <Tabs.Content value="connections">
        <Card.Root>
          <Card.Header>
            <div class="flex items-center justify-between">
              <Card.Title>
                {tValue($locale, 'main.api.connections', 'Connections')} ({apiConnections.length})
              </Card.Title>
              <Button variant="outline" size="sm" onclick={closeConnections}>
                {tValue($locale, 'main.api.close_connections', 'Close all')}
              </Button>
            </div>
          </Card.Header>
          <Card.Content>
            {#if !apiConnections.length}
              <p class="text-muted-foreground">{tValue($locale, 'main.connections.empty', 'No active connections')}</p>
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

  {#if message}
    <p class="text-sm text-destructive">{message}</p>
  {/if}
</Page>

<Dialog.Root open={selectedConn != null} onOpenChange={(open) => { if (!open) selectedConn = null; }}>
  <Dialog.Content class="sm:max-w-lg">
    <Dialog.Header>
      <Dialog.Title>{tValue($locale, 'connection_details.title', 'Connection')}</Dialog.Title>
    </Dialog.Header>
    {#if selectedConn}
      {@const c = selectedConn}
      {@const inbound = c.inbound || c.inboundType || '—'}
      {@const outbound = c.outbound || c.outboundType || '—'}
      <div class="space-y-3 text-sm">
        {@render DetailRow('ID', c.id)}
        {@render DetailRow(tValue($locale, 'connection_details.inbound', 'Inbound'), inbound)}
        {@render DetailRow(tValue($locale, 'connection_details.network', 'Network'), c.network)}
        {@render DetailRow(tValue($locale, 'connection_details.source', 'Source'), c.source)}
        {@render DetailRow(tValue($locale, 'connection_details.destination', 'Destination'), c.destination)}
        {@render DetailRow(tValue($locale, 'connection_details.domain', 'Domain'), c.domain)}
        {@render DetailRow(tValue($locale, 'connection_details.rule', 'Rule'), c.rule)}
        {@render DetailRow(tValue($locale, 'connection_details.outbound', 'Outbound'), outbound)}
        {@render DetailRow(tValue($locale, 'connection_details.chain', 'Chain'), c.chain?.join(' → '))}
        {@render DetailRow(tValue($locale, 'connection_details.uplink', 'Uplink'), `${formatSpeed(c.uplink)} (${formatBytes(c.uplinkTotal)})`)}
        {@render DetailRow(tValue($locale, 'connection_details.downlink', 'Downlink'), `${formatSpeed(c.downlink)} (${formatBytes(c.downlinkTotal)})`)}
        {@render DetailRow(tValue($locale, 'connection_details.created', 'Created'), formatTime(c.createdAt))}
        {#if c.processInfo?.userName}
          {@render DetailRow(tValue($locale, 'connection_details.user', 'User'), c.processInfo.userName)}
          {@render DetailRow(tValue($locale, 'connection_details.process', 'Process'), c.processInfo.processPath)}
        {/if}
      </div>
      <Dialog.Footer>
        <Button variant="destructive" onclick={() => closeConnection(c.id)}>
          {tValue($locale, 'connection_details.close_connection', 'Close connection')}
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
