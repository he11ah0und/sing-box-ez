<script lang="ts">
  import { untrack } from 'svelte';
  import { Square, RefreshCw, ChevronDown, ChevronUp, Zap } from '@lucide/svelte';
  import { toast } from 'svelte-sonner';
  import { appState } from '../stores/appState.js';
  import { useLocale, useLocaleRecord } from '../stores/locale.svelte.js';

  import Page from '../components/Page.svelte';
  import StackedGraph from '../components/StackedGraph.svelte';
  import GraphDetailDialog from '../components/GraphDetailDialog.svelte';
  import ActivityTimeline from '../components/ActivityTimeline.svelte';
  import TimelineDetailDialog from '../components/TimelineDetailDialog.svelte';
  import * as Dialog from '$lib/components/ui/dialog/index.js';
  import * as Card from '$lib/components/ui/card/index.js';
  import * as Tabs from '$lib/components/ui/tabs/index.js';
  import * as Select from '$lib/components/ui/select/index.js';
  import { Button } from '$lib/components/ui/button/index.js';
  import { Badge } from '$lib/components/ui/badge/index.js';
  import { Input } from '$lib/components/ui/input/index.js';
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
    CloseAPIConnection,
    CloseAPIConnectionGroup,
    GetConnectionTrafficHistory,
    GetConnectionGroupTrafficHistory,
    GetConnectionsSort,
    SetConnectionsSort
  } from '../../../bindings/sing-box-ez/internal/gui/wails/bindings.js';
  import type {
    Status as APIStatus,
    Info as APIInfo,
    Group as APIGroup,
    Node as APINode,
    Connection as APIConnection,
    ConnectionGroup as APIConnectionGroup
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
    'main.graph.details_title',
    'main.graph.total',
    'main.api.mode',
    'tab.groups',
    'main.groups.empty',
    'main.api.connections',
    'main.api.close_connections',
    'main.connections.empty',
    'main.connections.inactive',
    'main.connections.active',
    'main.connections.timeline',
    'plugins.info.status',
    'main.connections.total_connections',
    'main.connections.search',
    'main.connections.close_group',
    'main.connections.sort_date',
    'main.connections.sort_traffic',
    'main.connections.sort_total',
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
    'connection_details.close_connection',
    'configs.type.remote',
    'configs.type.local'
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
  const apiConnGroups = $derived($appState.api.connGroups ?? []);
  let expandedConnGroups = $state<Set<string>>(new Set());
  // Connection search filters groups by target substring (IP or domain).
  let connSearch = $state('');
  const visibleConnGroups = $derived.by(() => {
    const q = connSearch.trim().toLowerCase();
    if (!q) return apiConnGroups;
    return apiConnGroups.filter((g) => g.target.toLowerCase().includes(q));
  });
  // Group details dialog (right-click on a group row) follows the live group
  // from the store so the timeline keeps ticking while it is open.
  let selectedGroupKey = $state<string | null>(null);
  const selectedGroup = $derived(apiConnGroups.find((g) => g.key === selectedGroupKey) ?? null);
  // The group dialog's timeline miniature opens this full interactive view.
  let timelineOpen = $state(false);

  // Connection group sorting. The mode is persisted in the config
  // (ui.connections_sort) and applied by the backend; the entry has no
  // settings-UI control, so it round-trips through dedicated bindings
  // (like language/theme) instead of SetConfigValues.
  type ConnSortMode = 'date' | 'traffic' | 'total';
  const connSortModes: ConnSortMode[] = ['date', 'traffic', 'total'];
  let connSort = $state<ConnSortMode>('date');
  const connSortLabels = $derived<Record<ConnSortMode, string>>({
    date: L.mainConnectionsSort_date,
    traffic: L.mainConnectionsSort_traffic,
    total: L.mainConnectionsSort_total
  });

  async function setConnSort(mode: ConnSortMode) {
    connSort = mode;
    try {
      await SetConnectionsSort(mode);
    } catch (err) {
      toast.error(String(err));
    }
  }

  // Group rows: left click toggles the member list, right-click or a
  // long-press (mobile) opens the group details dialog.
  let lpTimer: ReturnType<typeof setTimeout> | null = null;
  let lpFired = false;

  function lpCancel() {
    if (lpTimer !== null) {
      clearTimeout(lpTimer);
      lpTimer = null;
    }
  }
  function lpStart(key: string) {
    lpCancel();
    lpTimer = setTimeout(() => {
      lpTimer = null;
      lpFired = true;
      selectedGroupKey = key;
    }, 500);
  }
  function onGroupClick(g: APIConnectionGroup) {
    // The click following a long-press must not toggle the expansion.
    if (lpFired) {
      lpFired = false;
      return;
    }
    // Inactive groups have no live members to expand; a left click opens
    // the details dialog directly (same as right-click / long-press).
    if (!g.active) {
      selectedGroupKey = g.key;
      return;
    }
    toggleConnGroup(g.key);
  }

  // IPv badges of a group: the literal version for IP targets, otherwise
  // per-version member counts (last known for inactive groups).
  function groupIPBadges(g: APIConnectionGroup): string[] {
    const direct = ipVersionLabel(g.target);
    if (direct) return [direct];
    const badges: string[] = [];
    if (g.ipv4 > 0) badges.push(g.ipv4 > 1 ? `IPv4×${g.ipv4}` : 'IPv4');
    if (g.ipv6 > 0) badges.push(g.ipv6 > 1 ? `IPv6×${g.ipv6}` : 'IPv6');
    return badges;
  }
  let expandedGroups = $state<Set<string>>(new Set());
  let testingGroups = $state<Set<string>>(new Set());
  let groupDelays = $state<Record<string, Record<string, number>>>({});
  let selectedConnId = $state<string | null>(null);
  let selectedConnSnapshot = $state<APIConnection | null>(null);
  // The open dialog follows the live connection from the store so traffic
  // counters refresh while it is open; the snapshot remains as a fallback
  // once the connection is gone from the list (closed).
  const selectedConn = $derived(
    (selectedConnId != null && apiConnections.find((c) => c.id === selectedConnId)) ||
      selectedConnSnapshot
  );

  const configTypeLabels: Record<string, string> = {
    remote: L.configsTypeRemote,
    local: L.configsTypeLocal
  };
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

  const visibleGroups = $derived(
    apiGroups.filter((g) => g.type !== 'Fallback' && g.type !== 'LoadBalance')
  );

  async function loadInitial() {
    try {
      const [status, configs, active, coreInfo, settings, sort] = await Promise.all([
        GetStatus(),
        GetConfigs(),
        GetActiveConfig(),
        GetCoreInfo(),
        GetConfigValues(),
        GetConnectionsSort()
      ]);
      if (connSortModes.includes(sort as ConnSortMode)) connSort = sort as ConnSortMode;
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

  function toggleConnGroup(key: string) {
    const next = new Set(expandedConnGroups);
    if (next.has(key)) next.delete(key);
    else next.add(key);
    expandedConnGroups = next;
  }

  // Live member connections of a group, resolved from the store snapshot.
  function groupMembers(g: APIConnectionGroup): APIConnection[] {
    const ids = new Set(g.connIDs ?? []);
    return apiConnections.filter((c) => ids.has(c.id));
  }

  function formatGroupSub(g: APIConnectionGroup): string {
    let sub =
      g.active && (g.upRate > 0 || g.downRate > 0)
        ? `↑${formatSpeed(g.upRate)} ↓${formatSpeed(g.downRate)} · ${formatBytes(g.upTotal + g.downTotal)}`
        : `↑${formatBytes(g.upTotal)} ↓${formatBytes(g.downTotal)}`;
    if (g.lastAgo) sub += ` · ${g.lastAgo}`;
    return sub;
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
      selectedConnId = null;
      selectedConnSnapshot = null;
    } catch {
      // The backend already reported the failure with a toast.
    }
  }

  // Closes every live member of a connection group; the group itself turns
  // inactive on the next snapshot and is kept for its retention window.
  async function closeConnectionGroup(g: APIConnectionGroup) {
    try {
      await CloseAPIConnectionGroup(g.connIDs ?? []);
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

  // Per-connection graph history: seeded once from the backend when the
  // connection dialog opens, then extended with live samples from the
  // api:state snapshots. connGraphId pins the buffer to one connection so a
  // late seed response cannot overwrite a newer dialog's data.
  let connGraphOpen = $state(false);
  let connGraphId = $state<string | null>(null);
  let connGraphSeeded = $state(false);
  let connGraphHistory = $state<{ up: number[]; down: number[]; times: number[] }>({
    up: [],
    down: [],
    times: []
  });

  // Per-group graph history, same seed-then-extend pattern as connGraph.
  let groupGraphOpen = $state(false);
  let groupGraphKey = $state<string | null>(null);
  let groupGraphSeeded = $state(false);
  let groupGraphHistory = $state<{ up: number[]; down: number[]; times: number[] }>({
    up: [],
    down: [],
    times: []
  });

  $effect(() => {
    const id = selectedConnId;
    if (!id) {
      connGraphId = null;
      connGraphSeeded = false;
      return;
    }
    connGraphId = id;
    connGraphSeeded = false;
    connGraphHistory = { up: [], down: [], times: [] };
    GetConnectionTrafficHistory(id)
      .then((h) => {
        if (connGraphId !== id) return;
        const pts = h?.points ?? [];
        connGraphHistory = {
          up: pts.map((p) => p.up ?? 0),
          down: pts.map((p) => p.down ?? 0),
          times: pts.map((p) => new Date(p.at).getTime())
        };
      })
      .catch(() => {})
      .finally(() => {
        if (connGraphId === id) connGraphSeeded = true;
      });
  });

  $effect(() => {
    const key = selectedGroupKey;
    if (!key) {
      groupGraphKey = null;
      groupGraphSeeded = false;
      return;
    }
    groupGraphKey = key;
    groupGraphSeeded = false;
    groupGraphHistory = { up: [], down: [], times: [] };
    GetConnectionGroupTrafficHistory(key)
      .then((h) => {
        if (groupGraphKey !== key) return;
        const pts = h?.points ?? [];
        groupGraphHistory = {
          up: pts.map((p) => p.up ?? 0),
          down: pts.map((p) => p.down ?? 0),
          times: pts.map((p) => new Date(p.at).getTime())
        };
      })
      .catch(() => {})
      .finally(() => {
        if (groupGraphKey === key) groupGraphSeeded = true;
      });
  });

  $effect(() => {
    const key = selectedGroupKey;
    const groups = apiConnGroups;
    if (!key || groupGraphKey !== key || !groupGraphSeeded) return;
    const g = groups.find((gr) => gr.key === key);
    if (!g || !g.active) return;
    // Append on every api:state push (~1/s), zero rate included; untracked
    // for the same retrigger reason as the connection graph above.
    untrack(() => {
      const maxPoints = Math.max(2, graphSpan * 2);
      groupGraphHistory = {
        up: [...groupGraphHistory.up, g.upRate ?? 0].slice(-maxPoints),
        down: [...groupGraphHistory.down, g.downRate ?? 0].slice(-maxPoints),
        times: [...groupGraphHistory.times, Date.now()].slice(-maxPoints)
      };
    });
  });

  $effect(() => {
    const id = selectedConnId;
    const conns = apiConnections;
    if (!id || connGraphId !== id || !connGraphSeeded) return;
    const conn = conns.find((c) => c.id === id);
    if (!conn) return;
    // Append on every api:state push (~1/s), zero rate included: sparse
    // points would interpolate into triangular artifacts on the graph.
    // The history reads are untracked so the write cannot retrigger this
    // effect into an infinite loop.
    untrack(() => {
      const maxPoints = Math.max(2, graphSpan * 2);
      connGraphHistory = {
        up: [...connGraphHistory.up, conn.upRate ?? 0].slice(-maxPoints),
        down: [...connGraphHistory.down, conn.downRate ?? 0].slice(-maxPoints),
        times: [...connGraphHistory.times, Date.now()].slice(-maxPoints)
      };
    });
  });

  const connGraphTitle = $derived(
    selectedConn
      ? `${L.mainGraphDetails_title} — ${formatConnectionTarget(selectedConn)}`
      : L.mainGraphDetails_title
  );

  const groupGraphTitle = $derived(
    selectedGroup
      ? `${L.mainGraphDetails_title} — ${selectedGroup.target}`
      : L.mainGraphDetails_title
  );

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
            <div class="flex items-center justify-between gap-2">
              <Card.Title>
                {L.mainApiConnections} ({apiConnections.length})
              </Card.Title>
              <div class="flex items-center gap-2">
                <Input
                  class="w-36 sm:w-48 h-8 text-sm"
                  placeholder={L.mainConnectionsSearch}
                  bind:value={connSearch}
                />
                <Select.Root
                  type="single"
                  value={connSort}
                  onValueChange={(v) => { if (v) setConnSort(v as ConnSortMode); }}
                >
                  <Select.Trigger class="w-32 h-8 text-sm">{connSortLabels[connSort]}</Select.Trigger>
                  <Select.Content>
                    {#each connSortModes as mode (mode)}
                      <Select.Item value={mode} label={connSortLabels[mode]} />
                    {/each}
                  </Select.Content>
                </Select.Root>
                <Button variant="outline" size="sm" onclick={closeConnections}>
                  {L.mainApiClose_connections}
                </Button>
              </div>
            </div>
          </Card.Header>
          <Card.Content>
            {#if !visibleConnGroups.length}
              <p class="text-muted-foreground">{L.mainConnectionsEmpty}</p>
            {:else}
              <div class="space-y-2">
                {#each visibleConnGroups as group (group.key)}
                  {@const expanded = group.active && expandedConnGroups.has(group.key)}
                  <div
                    class="w-full rounded-xl border border-border bg-background {group.active ? '' : 'opacity-60'}"
                  >
                    <button
                      class="w-full text-left p-3 hover:bg-accent transition rounded-xl"
                      onclick={() => onGroupClick(group)}
                      oncontextmenu={(e) => {
                        e.preventDefault();
                        selectedGroupKey = group.key;
                      }}
                      ontouchstart={() => lpStart(group.key)}
                      ontouchend={lpCancel}
                      ontouchmove={lpCancel}
                    >
                      <div class="flex items-center justify-between gap-3">
                        <span class="font-medium truncate">{group.target}</span>
                        <span class="flex items-center gap-2 shrink-0">
                          {#if group.connCount > 1}
                            <Badge variant="outline" class="whitespace-nowrap">×{group.connCount}</Badge>
                          {/if}
                          {#if !group.active}
                            <Badge variant="outline" class="whitespace-nowrap">{L.mainConnectionsInactive}</Badge>
                          {/if}
                          {#each groupIPBadges(group) as ipb (ipb)}
                            <Badge variant="outline" class="whitespace-nowrap">{ipb}</Badge>
                          {/each}
                          {#if group.active}
                            {#if expanded}
                              <ChevronUp size={16} class="text-muted-foreground" />
                            {:else}
                              <ChevronDown size={16} class="text-muted-foreground" />
                            {/if}
                          {/if}
                        </span>
                      </div>
                      <p class="text-sm text-muted-foreground truncate mt-1">{formatGroupSub(group)}</p>
                    </button>
                    {#if expanded}
                      {@const members = groupMembers(group)}
                      <div class="px-3 pb-3 space-y-2 border-t border-border pt-2">
                        {#each members as conn (conn.id)}
                          <button
                            class="w-full text-left rounded-lg border border-border bg-background p-2 hover:bg-accent transition"
                            onclick={() => {
                              selectedConnId = conn.id;
                              selectedConnSnapshot = conn;
                            }}
                          >
                            <div class="flex items-center justify-between gap-3">
                              <span class="text-sm font-medium truncate">{formatConnectionTarget(conn)}</span>
                              {#if ipVersionLabel(conn.destination)}
                                <Badge variant="outline" class="whitespace-nowrap">{ipVersionLabel(conn.destination)}</Badge>
                              {/if}
                            </div>
                            <p class="text-xs text-muted-foreground truncate mt-0.5">{formatConnectionSub(conn)}</p>
                          </button>
                        {/each}
                      </div>
                    {/if}
                  </div>
                {/each}
              </div>
            {/if}
          </Card.Content>
        </Card.Root>
      </Tabs.Content>
    </Tabs.Root>
  {/if}

</Page>

<Dialog.Root open={selectedConn != null} onOpenChange={(open) => { if (!open) { selectedConnId = null; selectedConnSnapshot = null; } }}>
  <Dialog.Content class="sm:max-w-lg max-h-[85vh] overflow-y-auto">
    <Dialog.Header>
      <Dialog.Title>{L.connection_detailsTitle}</Dialog.Title>
    </Dialog.Header>
    {#if selectedConn}
      {@const c = selectedConn}
      <!-- Route-level fields (inbound/network/source/...) live in the group
           details dialog: members of a group share them. -->
      <div class="space-y-3 text-sm">
        {@render DetailRow('ID', c.id)}
        {@render DetailRow(L.pluginsInfoStatus, apiConnections.some((x) => x.id === c.id) ? L.mainConnectionsActive : L.mainConnectionsInactive)}
        {@render DetailRow(L.connection_detailsUplink, `${formatSpeed(c.uplink)} (${formatBytes(c.uplinkTotal)})`)}
        {@render DetailRow(L.connection_detailsDownlink, `${formatSpeed(c.downlink)} (${formatBytes(c.downlinkTotal)})`)}
        {@render DetailRow(L.connection_detailsCreated, formatTime(c.createdAt) + (c.createdAgo ? ` (${c.createdAgo})` : ''))}
        {#if c.processInfo?.userName}
          {@render DetailRow(L.connection_detailsUser, c.processInfo.userName)}
          {@render DetailRow(L.connection_detailsProcess, c.processInfo.processPath)}
        {/if}
        {#if connGraphId === c.id}
          <div class="space-y-1 pt-1">
            <p class="text-muted-foreground">{L.mainGraphDetails_title}</p>
            <button
              class="w-full rounded-xl border border-border bg-background p-2 cursor-pointer hover:bg-accent transition"
              onclick={() => (connGraphOpen = true)}
            >
              <StackedGraph
                up={connGraphHistory.up}
                down={connGraphHistory.down}
                times={connGraphHistory.times}
                span={graphSpan}
                height={80}
              />
            </button>
          </div>
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

<!-- Connection group details: opened by right-click on a group row. Shows
     the activity timeline and aggregated counters; follows store updates. -->
<Dialog.Root open={selectedGroup != null} onOpenChange={(open) => { if (!open) selectedGroupKey = null; }}>
  <Dialog.Content class="sm:max-w-lg max-h-[85vh] overflow-y-auto">
    <Dialog.Header>
      <Dialog.Title>{selectedGroup?.target ?? ''}</Dialog.Title>
    </Dialog.Header>
    {#if selectedGroup}
      {@const g = selectedGroup}
      <!-- Route-level fields are shared by the group's members; take them
           from the first live one. Inactive groups have no live members,
           so these rows hide (the history does not store them). -->
      {@const m = groupMembers(g)[0]}
      <div class="space-y-3 text-sm">
        <div class="flex items-center gap-2">
          {#if g.active}
            <Badge variant="outline">×{g.connCount}</Badge>
          {:else}
            <Badge variant="outline">{L.mainConnectionsInactive}</Badge>
          {/if}
          {#each groupIPBadges(g) as ipb (ipb)}
            <Badge variant="outline">{ipb}</Badge>
          {/each}
        </div>
        {@render DetailRow(L.pluginsInfoStatus, g.active ? `${L.mainConnectionsActive} ×${g.connCount}` : L.mainConnectionsInactive)}
        {@render DetailRow(L.mainConnectionsTotal_connections, String((g.spans ?? []).length))}
        {@render DetailRow(L.connection_detailsNetwork, g.network)}
        {#if m}
          {@render DetailRow(L.connection_detailsInbound, m.inbound || m.inboundType)}
          {@render DetailRow(L.connection_detailsSource, m.source)}
          {@render DetailRow(L.connection_detailsDestination, m.destination)}
          {@render DetailRow(L.connection_detailsDomain, m.domain)}
          {@render DetailRow(L.commonRule, m.rule)}
          {@render DetailRow(L.connection_detailsOutbound, m.outbound || m.outboundType)}
          {@render DetailRow(L.connection_detailsChain, m.chain?.join(' → '))}
        {/if}
        {@render DetailRow(L.connection_detailsCreated, formatTime(g.firstSeen))}
        {@render DetailRow(L.connection_detailsUplink, `${formatSpeed(g.upRate)} (${formatBytes(g.upTotal)})`)}
        {@render DetailRow(L.connection_detailsDownlink, `${formatSpeed(g.downRate)} (${formatBytes(g.downTotal)})`)}
        {#if groupGraphKey === g.key}
          <div class="space-y-1 pt-1">
            <p class="text-muted-foreground">{L.mainGraphDetails_title}</p>
            <button
              class="w-full rounded-xl border border-border bg-background p-2 cursor-pointer hover:bg-accent transition"
              onclick={() => (groupGraphOpen = true)}
            >
              <StackedGraph
                up={groupGraphHistory.up}
                down={groupGraphHistory.down}
                times={groupGraphHistory.times}
                span={graphSpan}
                height={80}
              />
            </button>
          </div>
        {/if}
        {#if (g.spans ?? []).length}
          <div class="space-y-1 pt-1">
            <p class="text-muted-foreground">{L.mainConnectionsTimeline}</p>
            <!-- Miniature: click opens the full interactive timeline. -->
            <button
              class="w-full rounded-xl border border-border bg-background p-2 cursor-pointer hover:bg-accent transition"
              onclick={() => (timelineOpen = true)}
            >
              <div class="max-h-32 overflow-hidden pointer-events-none">
                <ActivityTimeline spans={g.spans ?? []} interactive={false} />
              </div>
            </button>
          </div>
        {/if}
      </div>
      {#if g.active && (g.connIDs ?? []).length}
        <Dialog.Footer>
          <Button variant="destructive" onclick={() => closeConnectionGroup(g)}>
            {L.mainConnectionsClose_group}
          </Button>
        </Dialog.Footer>
      {/if}
    {/if}
  </Dialog.Content>
</Dialog.Root>

<TimelineDetailDialog
  open={timelineOpen && selectedGroup != null}
  onclose={() => (timelineOpen = false)}
  title={selectedGroup ? `${L.mainConnectionsTimeline} — ${selectedGroup.target}` : ''}
  spans={selectedGroup?.spans ?? []}
/>

<GraphDetailDialog
  open={mainGraphOpen}
  onclose={() => (mainGraphOpen = false)}
  title={L.mainGraphDetails_title}
  up={$appState.traffic.history.up}
  down={$appState.traffic.history.down}
  times={$appState.traffic.history.times}
  span={graphSpan}
/>

<GraphDetailDialog
  open={connGraphOpen}
  onclose={() => (connGraphOpen = false)}
  title={connGraphTitle}
  up={connGraphHistory.up}
  down={connGraphHistory.down}
  times={connGraphHistory.times}
  span={graphSpan}
/>

<GraphDetailDialog
  open={groupGraphOpen}
  onclose={() => (groupGraphOpen = false)}
  title={groupGraphTitle}
  up={groupGraphHistory.up}
  down={groupGraphHistory.down}
  times={groupGraphHistory.times}
  span={graphSpan}
/>
