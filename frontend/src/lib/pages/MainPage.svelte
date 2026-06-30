<script module>
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

<script>
  import { onMount, onDestroy } from 'svelte';
  import { Play, Square, RefreshCw, ChevronDown, ChevronUp, Zap, X } from '@lucide/svelte';
  import { appState } from '../stores/appState.js';
  import { locale, tValue } from '../stores/locale.js';

  import Page from '../components/Page.svelte';
  import Sparkline from '../components/Sparkline.svelte';
  import Modal from '../components/Modal.svelte';
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
    CloseAPIConnection
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
  let apiStatus = $state(null);
  let apiInfo = $state(null);
  let apiMode = $state('');
  let apiGroups = $state([]);
  let apiConnections = $state([]);
  let expandedGroups = $state(new Set());
  let testingGroups = $state(new Set());
  let groupDelays = $state({});
  let selectedConn = $state(null);
  let pollTimer = $state(null);
  let connectedAt = $state(null);

  const statusLabel = $derived(
    $appState.status.running
      ? tValue($locale, 'main.running', 'Running')
      : tValue($locale, 'main.stopped', 'Stopped')
  );
  const activeName = $derived($appState.activeConfig?.name ?? '—');
  const activeBadge = $derived($appState.activeConfig?.type ?? '');
  const activeUpdated = $derived(formatRelative($appState.activeConfig?.last_update));

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
        settings
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
    } catch (err) {
      // Suppress repeated polling errors; they are visible when the API is down.
    }
  }

  async function callBinding(promise, action) {
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

  async function activateConfig(name) {
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

  async function setMode(mode) {
    try {
      await SetAPIMode(mode);
      await pollAPI();
    } catch (err) {
      message = String(err);
    }
  }

  async function selectNode(group, node) {
    try {
      await SelectAPINode(group, node);
      await pollAPI();
    } catch (err) {
      message = String(err);
    }
  }

  async function testGroup(tag) {
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

  function toggleGroup(tag) {
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

  async function closeConnection(id) {
    try {
      await CloseAPIConnection(id);
      selectedConn = null;
      await pollAPI();
    } catch (err) {
      message = String(err);
    }
  }

  function formatNodeDelay(n, groupTag) {
    if (n.delayValid) return `${n.delay} ms`;
    const d = groupDelays[groupTag]?.[n.tag];
    if (d != null) return `${d} ms`;
    return '—';
  }

  function delayColor(n, groupTag) {
    let d = -1;
    if (n.delayValid) d = n.delay;
    else if (groupDelays[groupTag]?.[n.tag] != null) d = groupDelays[groupTag][n.tag];
    if (d < 0) return '';
    if (d < 300) return 'text-green-500';
    if (d < 800) return 'text-amber-500';
    return 'text-red-500';
  }

  function sparkStats(data) {
    if (!data?.length) return { min: 0, max: 0, avg: 0 };
    return {
      min: Math.min(...data),
      max: Math.max(...data),
      avg: data.reduce((a, b) => a + b, 0) / data.length
    };
  }

  function buildOutboundChain(groups) {
    if (!groups?.length) return { chain: '', delay: '' };
    const groupMap = new Map(groups.map((g) => [g.tag, g]));
    const selectedAsNode = new Map();
    for (const g of groups) {
      for (const n of g.nodes) {
        selectedAsNode.set(n.tag, (selectedAsNode.get(n.tag) ?? 0) + 1);
      }
    }
    const roots = groups.filter((g) => !selectedAsNode.has(g.tag));
    if (!roots.length) return { chain: '', delay: '' };
    roots.sort((a, b) => a.tag.localeCompare(b.tag));
    let g = roots[0];
    const parts = [];
    const visited = new Set();
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
      const node = g.nodes.find((n) => n.tag === g.selected);
      parts.push(`${g.selected} (${node?.type ?? 'node'})`);
      if (node?.delayValid) delay = `${node.delay} ms`;
      break;
    }
    return { chain: parts.join(' → '), delay };
  }

  const outboundChain = $derived(buildOutboundChain(apiGroups));
  const upStats = $derived(sparkStats($appState.traffic.history.up));
  const downStats = $derived(sparkStats($appState.traffic.history.down));

  function formatConnectionTarget(conn) {
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

  function formatConnectionSub(conn) {
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
        class="w-32 h-32 rounded-full bg-[var(--color-primary)] text-white text-lg font-semibold shadow-lg disabled:opacity-50 hover:opacity-90 transition flex items-center justify-center"
        disabled={processing}
        onclick={handleStart}
      >
        {#if processing}
          <RefreshCw size={32} class="animate-spin" />
        {:else}
          {tValue($locale, 'main.btn.start', 'Start')}
        {/if}
      </button>
      <p class="text-[var(--color-text-muted)]">{statusLabel}</p>
    </div>

    <section class="rounded-2xl border border-[var(--color-border)] bg-[var(--color-surface)] p-5 shadow-sm">
      <label class="block space-y-2">
        <span class="text-sm text-[var(--color-text-muted)]">{tValue($locale, 'main.active.label', 'Active config')}</span>
        <select
          class="w-full rounded-xl border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2"
          value={$appState.activeConfig?.name ?? ''}
          onchange={(e) => activateConfig(e.target.value)}
        >
          <option value="" disabled>{$appState.configs.length ? tValue($locale, 'main.active.placeholder', 'Select active config') : tValue($locale, 'configs.empty', 'No configs')}</option>
          {#each $appState.configs as cfg}
            <option value={cfg.name}>{cfg.name}</option>
          {/each}
        </select>
      </label>
    </section>
  {:else}
    <!-- Running state -->
    <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
      <div class="flex gap-2">
        {#each tabs as tab}
          <button
            class="px-4 py-2 rounded-xl text-sm font-medium transition border"
            class:bg-[var(--color-primary)]={activeTab === tab.id}
            class:text-white={activeTab === tab.id}
            class:border-transparent={activeTab === tab.id}
            class:bg-[var(--color-surface-variant)]={activeTab !== tab.id}
            class:border-[var(--color-border)]={activeTab !== tab.id}
            onclick={() => activeTab = tab.id}
          >
            {tValue($locale, tab.key, tab.id)}
          </button>
        {/each}
      </div>
      <div class="text-right text-sm text-[var(--color-text-muted)]">
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

    {#if activeTab === 'overview'}
      <!-- Traffic graphs -->
      <section class="rounded-2xl border border-[var(--color-border)] bg-[var(--color-surface)] p-5 shadow-sm space-y-4">
        <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div class="rounded-xl bg-[var(--color-bg)] p-4 space-y-2">
            <div class="flex items-center justify-between">
              <span class="text-sm text-[var(--color-text-muted)]">{tValue($locale, 'main.dashboard.upload', 'Upload')}</span>
              <span class="text-sm font-medium">{$appState.traffic.upRate}</span>
            </div>
            <Sparkline data={$appState.traffic.history.up} color="var(--color-success)" fill />
            <p class="text-xs text-[var(--color-text-muted)]">
              {tValue($locale, 'main.dashboard.min', 'Min')}: {formatSpeed(upStats.min)}
              &nbsp;{tValue($locale, 'main.dashboard.max', 'Max')}: {formatSpeed(upStats.max)}
              &nbsp;{tValue($locale, 'main.dashboard.avg', 'Avg')}: {formatSpeed(upStats.avg)}
            </p>
          </div>
          <div class="rounded-xl bg-[var(--color-bg)] p-4 space-y-2">
            <div class="flex items-center justify-between">
              <span class="text-sm text-[var(--color-text-muted)]">{tValue($locale, 'main.dashboard.download', 'Download')}</span>
              <span class="text-sm font-medium">{$appState.traffic.downRate}</span>
            </div>
            <Sparkline data={$appState.traffic.history.down} color="var(--color-primary)" fill />
            <p class="text-xs text-[var(--color-text-muted)]">
              {tValue($locale, 'main.dashboard.min', 'Min')}: {formatSpeed(downStats.min)}
              &nbsp;{tValue($locale, 'main.dashboard.max', 'Max')}: {formatSpeed(downStats.max)}
              &nbsp;{tValue($locale, 'main.dashboard.avg', 'Avg')}: {formatSpeed(downStats.avg)}
            </p>
          </div>
        </div>
      </section>

      <!-- Outbound chain -->
      {#if outboundChain.chain}
        <section class="rounded-2xl border border-[var(--color-border)] bg-[var(--color-surface)] p-5 shadow-sm">
          <div class="flex items-center justify-between gap-4">
            <p class="text-sm break-all">{outboundChain.chain}</p>
            {#if outboundChain.delay}
              <span class="text-green-500 text-sm whitespace-nowrap">{outboundChain.delay}</span>
            {/if}
          </div>
        </section>
      {/if}

      <!-- Mode selector -->
      <section class="rounded-2xl border border-[var(--color-border)] bg-[var(--color-surface)] p-5 shadow-sm">
        <label class="block space-y-2">
          <span class="text-sm text-[var(--color-text-muted)]">{tValue($locale, 'main.api.mode', 'Mode')}</span>
          <select
            class="w-full rounded-xl border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2"
            value={apiMode}
            onchange={(e) => setMode(e.target.value)}
          >
            {#each modes as m}
              <option value={m}>{tValue($locale, `main.api.mode_${m}`, m)}</option>
            {/each}
          </select>
        </label>
      </section>

      <!-- Profile card -->
      <section class="rounded-2xl border border-[var(--color-border)] bg-[var(--color-surface)] p-5 shadow-sm">
        <div class="flex items-center justify-between mb-4">
          <span class="text-sm text-[var(--color-text-muted)]">{tValue($locale, 'main.dashboard.profile', 'Profile')}</span>
          {#if activeBadge}
            <span class="px-2 py-1 rounded-lg text-xs border border-[var(--color-border)] bg-[var(--color-surface-variant)]">
              {activeBadge}
            </span>
          {/if}
        </div>
        <label class="block space-y-2">
          <select
            class="w-full rounded-xl border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2"
            value={$appState.activeConfig?.name ?? ''}
            onchange={(e) => activateConfig(e.target.value)}
            disabled={!$appState.configs.length}
          >
            <option value="" disabled>{$appState.configs.length ? tValue($locale, 'main.active.placeholder', 'Select active config') : tValue($locale, 'configs.empty', 'No configs')}</option>
            {#each $appState.configs as cfg}
              <option value={cfg.name}>{cfg.name}</option>
            {/each}
          </select>
          {#if activeUpdated}
            <p class="text-sm text-[var(--color-text-muted)]">{activeUpdated}</p>
          {/if}
        </label>
      </section>

      <!-- Stop / Restart -->
      <div class="flex flex-wrap gap-3">
        <button
          class="flex-1 flex items-center justify-center gap-2 px-4 py-3 rounded-xl bg-[var(--color-danger)] text-white disabled:opacity-50 hover:opacity-90 transition"
          disabled={processing}
          onclick={handleStop}
        >
          <Square size={18} />
          {tValue($locale, 'main.btn.stop', 'Stop')}
        </button>
        <button
          class="flex-1 flex items-center justify-center gap-2 px-4 py-3 rounded-xl border border-[var(--color-border)] bg-[var(--color-surface-variant)] disabled:opacity-50 hover:bg-[var(--color-border)] transition"
          disabled={processing}
          onclick={handleRestart}
        >
          <RefreshCw size={18} />
          {tValue($locale, 'main.btn.restart', 'Restart')}
        </button>
      </div>
    {:else if activeTab === 'groups'}
      <section class="rounded-2xl border border-[var(--color-border)] bg-[var(--color-surface)] p-5 shadow-sm space-y-4">
        <h3 class="text-lg font-semibold">{tValue($locale, 'main.groups.title', 'Groups')}</h3>
        {#if !visibleGroups.length}
          <p class="text-[var(--color-text-muted)]">{tValue($locale, 'main.groups.empty', 'No groups')}</p>
        {:else}
          {#each visibleGroups as group (group.tag)}
            <div class="rounded-xl border border-[var(--color-border)] bg-[var(--color-bg)] overflow-hidden">
              <div
                class="w-full px-4 py-3 flex items-center justify-between hover:bg-[var(--color-surface-variant)] transition cursor-pointer"
                onclick={() => toggleGroup(group.tag)}
                role="button"
                tabindex="0"
                onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') toggleGroup(group.tag); }}
              >
                <div class="text-left">
                  <p class="font-medium">{group.tag}</p>
                  <p class="text-sm text-[var(--color-text-muted)]">{group.selected || '—'}</p>
                </div>
                <div class="flex items-center gap-2">
                  <span class="px-2 py-1 rounded-lg text-xs border border-[var(--color-border)] bg-[var(--color-surface-variant)]">{group.type}</span>
                  {#if group.delayValid}
                    <span class="px-2 py-1 rounded-lg text-xs border border-[var(--color-border)] bg-[var(--color-surface-variant)]">{group.delay} ms</span>
                  {/if}
                  {#if group.type !== 'URLTest' && group.tag !== 'GLOBAL'}
                    <button
                      class="p-1.5 rounded-lg hover:bg-[var(--color-surface-variant)]"
                      disabled={testingGroups.has(group.tag)}
                      onclick={(e) => { e.stopPropagation(); testGroup(group.tag); }}
                    >
                      {#if testingGroups.has(group.tag)}
                        <RefreshCw size={16} class="animate-spin" />
                      {:else}
                        <Zap size={16} />
                      {/if}
                    </button>
                  {/if}
                  {#if expandedGroups.has(group.tag)}
                    <ChevronUp size={18} />
                  {:else}
                    <ChevronDown size={18} />
                  {/if}
                </div>
              </div>
              {#if expandedGroups.has(group.tag)}
                <div class="divide-y divide-[var(--color-border)]">
                  {#each group.nodes as node (node.tag)}
                    <button
                      class="w-full px-4 py-2 flex items-center justify-between hover:bg-[var(--color-surface-variant)] transition"
                      class:bg-[var(--color-primary)]={node.tag === group.selected}
                      class:text-white={node.tag === group.selected}
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
      </section>
    {:else if activeTab === 'connections'}
      <section class="rounded-2xl border border-[var(--color-border)] bg-[var(--color-surface)] p-5 shadow-sm space-y-4">
        <div class="flex items-center justify-between">
          <h3 class="text-lg font-semibold">
            {tValue($locale, 'main.api.connections', 'Connections')} ({apiConnections.length})
          </h3>
          <button
            class="px-3 py-1.5 rounded-lg text-sm border border-[var(--color-border)] bg-[var(--color-surface-variant)] hover:bg-[var(--color-border)] transition"
            onclick={closeConnections}
          >
            {tValue($locale, 'main.api.close_connections', 'Close all')}
          </button>
        </div>
        {#if !apiConnections.length}
          <p class="text-[var(--color-text-muted)]">{tValue($locale, 'main.connections.empty', 'No active connections')}</p>
        {:else}
          <div class="space-y-2">
            {#each apiConnections as conn (conn.id)}
              <button
                class="w-full text-left rounded-xl border border-[var(--color-border)] bg-[var(--color-bg)] p-3 hover:bg-[var(--color-surface-variant)] transition"
                onclick={() => selectedConn = conn}
              >
                <div class="flex items-center justify-between gap-3">
                  <span class="font-medium truncate">{formatConnectionTarget(conn)}</span>
                  {#if ipVersionLabel(conn.destination)}
                    <span class="px-2 py-0.5 rounded text-xs border border-[var(--color-border)] bg-[var(--color-surface-variant)] whitespace-nowrap">
                      {ipVersionLabel(conn.destination)}
                    </span>
                  {/if}
                </div>
                <p class="text-sm text-[var(--color-text-muted)] truncate mt-1">{formatConnectionSub(conn)}</p>
              </button>
            {/each}
          </div>
        {/if}
      </section>
    {/if}
  {/if}

  {#if message}
    <p class="text-sm text-[var(--color-danger)]">{message}</p>
  {/if}
</Page>

{#if selectedConn}
  <Modal
    title={tValue($locale, 'connection_details.title', 'Connection')}
    onclose={() => selectedConn = null}
  >
    {@const c = selectedConn}
    {@const inbound = c.inbound || c.inboundType || '—'}
    {@const outbound = c.outbound || c.outboundType || '—'}
    <div class="space-y-3 text-sm">
      <DetailRow label="ID" value={c.id} />
      <DetailRow label={tValue($locale, 'connection_details.inbound', 'Inbound')} value={inbound} />
      <DetailRow label={tValue($locale, 'connection_details.network', 'Network')} value={c.network} />
      <DetailRow label={tValue($locale, 'connection_details.source', 'Source')} value={c.source} />
      <DetailRow label={tValue($locale, 'connection_details.destination', 'Destination')} value={c.destination} />
      <DetailRow label={tValue($locale, 'connection_details.domain', 'Domain')} value={c.domain} />
      <DetailRow label={tValue($locale, 'connection_details.rule', 'Rule')} value={c.rule} />
      <DetailRow label={tValue($locale, 'connection_details.outbound', 'Outbound')} value={outbound} />
      <DetailRow label={tValue($locale, 'connection_details.chain', 'Chain')} value={c.chain?.join(' → ')} />
      <DetailRow label={tValue($locale, 'connection_details.uplink', 'Uplink')} value={`${formatSpeed(c.uplink)} (${formatBytes(c.uplinkTotal)})`} />
      <DetailRow label={tValue($locale, 'connection_details.downlink', 'Downlink')} value={`${formatSpeed(c.downlink)} (${formatBytes(c.downlinkTotal)})`} />
      <DetailRow label={tValue($locale, 'connection_details.created', 'Created')} value={formatTime(c.createdAt)} />
      {#if c.processInfo?.userName}
        <DetailRow label={tValue($locale, 'connection_details.user', 'User')} value={c.processInfo.userName} />
        <DetailRow label={tValue($locale, 'connection_details.process', 'Process')} value={c.processInfo.processPath} />
      {/if}
    </div>
    {#snippet footer()}
      <button
        class="px-4 py-2 rounded-xl bg-[var(--color-danger)] text-white hover:opacity-90 transition"
        onclick={() => closeConnection(c.id)}
      >
        {tValue($locale, 'connection_details.close_connection', 'Close connection')}
      </button>
    {/snippet}
  </Modal>
{/if}

{#snippet DetailRow(label, value)}
  {#if value}
    <div class="flex items-start justify-between gap-4">
      <span class="text-[var(--color-text-muted)] whitespace-nowrap">{label}</span>
      <span class="break-all text-right">{value}</span>
    </div>
  {/if}
{/snippet}
