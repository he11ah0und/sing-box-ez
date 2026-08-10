<script lang="ts">
  import { untrack } from 'svelte';
  import { ChevronDown, ChevronUp, Filter, ArrowUpDown, CircleX, Check } from '@lucide/svelte';
  import { toast } from 'svelte-sonner';
  import { appState } from '../stores/appState.js';
  import { useLocale } from '../stores/locale.svelte.js';

  import Page from '../components/Page.svelte';
  import StackedGraph from '../components/StackedGraph.svelte';
  import GraphDetailDialog from '../components/GraphDetailDialog.svelte';
  import ActivityTimeline from '../components/ActivityTimeline.svelte';
  import TimelineDetailDialog from '../components/TimelineDetailDialog.svelte';
  import * as Dialog from '$lib/components/ui/dialog/index.js';
  import * as Card from '$lib/components/ui/card/index.js';
  import { Button } from '$lib/components/ui/button/index.js';
  import { Badge } from '$lib/components/ui/badge/index.js';
  import { Input } from '$lib/components/ui/input/index.js';
  import {
    CloseAPIConnections,
    CloseAPIConnection,
    CloseAPIConnectionGroup,
    GetConnectionTrafficHistory,
    GetConnectionGroupTrafficHistory,
    GetConnectionsSort,
    SetConnectionsSort
  } from '../../../bindings/sing-box-ez/internal/gui/wails/bindings.js';
  import type {
    Connection as APIConnection,
    ConnectionGroup as APIConnectionGroup
  } from '../../../bindings/sing-box-ez/internal/core/state/models.js';
  import { formatBytes, formatSpeed, formatTime, splitHostPort, ipVersionLabel } from '../utils/format.js';

  // Property names are derived from the keys: dots camelize, underscores
  // stay ("connection_details.title" → L.connection_detailsTitle).
  const L = useLocale([
    'main.graph.details_title',
    'main.api.connections',
    'main.api.close_connections',
    'main.connections.empty',
    'main.connections.inactive',
    'main.connections.active',
    'main.connections.timeline',
    'plugins.info.status',
    'main.connections.total_connections',
    'main.connections.search',
    'main.connections.filter',
    'main.connections.sort',
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
    'connection_details.close_connection'
  ]);

  // Core API state arrives from the backend via api:state events; the page
  // never polls.
  const apiInfo = $derived($appState.api.info);
  const apiConnections = $derived($appState.api.connections);
  const apiConnGroups = $derived($appState.api.connGroups ?? []);
  let expandedConnGroups = $state<Set<string>>(new Set());
  // Connection search filters groups by target substring (IP or domain);
  // it lives in the filter dialog so the header stays compact on mobile.
  let connSearch = $state('');
  let showConnFilter = $state(false);
  let showConnSort = $state(false);
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

  let selectedConnId = $state<string | null>(null);
  let selectedConnSnapshot = $state<APIConnection | null>(null);
  // The open dialog follows the live connection from the store so traffic
  // counters refresh while it is open; the snapshot remains as a fallback
  // once the connection is gone from the list (closed).
  const selectedConn = $derived(
    (selectedConnId != null && apiConnections.find((c) => c.id === selectedConnId)) ||
      selectedConnSnapshot
  );

  async function loadInitial() {
    try {
      const sort = await GetConnectionsSort();
      if (connSortModes.includes(sort as ConnSortMode)) connSort = sort as ConnSortMode;
    } catch (err) {
      toast.error(String(err));
    }
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

  const graphSpan = $derived(Math.max(1, $appState.settings?.['core.traffic_graph_history'] || 60));

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

<Page onLoad={loadInitial}>
  <Card.Root>
    <Card.Header>
      <!-- flex-wrap lets the controls drop below the title on narrow screens. -->
      <div class="flex flex-wrap items-center justify-between gap-2">
        <Card.Title>
          {L.mainApiConnections} ({apiConnections.length})
        </Card.Title>
        <div class="flex items-center gap-2">
          <!-- Icon-only on narrow screens; labels appear from sm up. -->
          <Button
            variant="outline"
            size="sm"
            class={connSearch.trim() ? 'text-primary' : ''}
            onclick={() => (showConnFilter = true)}
            aria-label={L.mainConnectionsFilter}
          >
            <Filter size={16} />
            <span class="hidden sm:inline">{L.mainConnectionsFilter}</span>
          </Button>
          <Button
            variant="outline"
            size="sm"
            onclick={() => (showConnSort = true)}
            aria-label={L.mainConnectionsSort}
          >
            <ArrowUpDown size={16} />
            <span class="hidden sm:inline">{connSortLabels[connSort]}</span>
          </Button>
          <Button
            variant="outline"
            size="sm"
            onclick={closeConnections}
            aria-label={L.mainApiClose_connections}
          >
            <CircleX size={16} />
            <span class="hidden sm:inline">{L.mainApiClose_connections}</span>
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

<!-- Filter dialog: currently holds only the target search; new filter
     variables go here as they appear. -->
<Dialog.Root open={showConnFilter} onOpenChange={(open) => { if (!open) showConnFilter = false; }}>
  <Dialog.Content>
    <Dialog.Header>
      <Dialog.Title>{L.mainConnectionsFilter}</Dialog.Title>
    </Dialog.Header>
    <Input
      placeholder={L.mainConnectionsSearch}
      bind:value={connSearch}
      autofocus
    />
  </Dialog.Content>
</Dialog.Root>

<!-- Sort dialog: one option per sort mode; the active one is checked. -->
<Dialog.Root open={showConnSort} onOpenChange={(open) => { if (!open) showConnSort = false; }}>
  <Dialog.Content>
    <Dialog.Header>
      <Dialog.Title>{L.mainConnectionsSort}</Dialog.Title>
    </Dialog.Header>
    <div class="flex flex-col gap-2">
      {#each connSortModes as mode (mode)}
        <button
          class="w-full text-left px-4 py-3 rounded-xl border border-border bg-secondary hover:bg-accent transition flex items-center justify-between {mode === connSort ? 'text-primary' : ''}"
          onclick={() => {
            setConnSort(mode);
            showConnSort = false;
          }}
        >
          {connSortLabels[mode]}
          {#if mode === connSort}<Check size={16} />{/if}
        </button>
      {/each}
    </div>
  </Dialog.Content>
</Dialog.Root>
