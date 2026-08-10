<script lang="ts">
  import { onDestroy } from 'svelte';
  import { RefreshCw, ChevronDown, ChevronUp, Zap, Filter } from '@lucide/svelte';
  import { appState } from '../stores/appState.js';
  import { useLocale } from '../stores/locale.svelte.js';

  import Page from '../components/Page.svelte';
  import * as Card from '$lib/components/ui/card/index.js';
  import * as Dialog from '$lib/components/ui/dialog/index.js';
  import { Button } from '$lib/components/ui/button/index.js';
  import { Badge } from '$lib/components/ui/badge/index.js';
  import { Input } from '$lib/components/ui/input/index.js';
  import { Skeleton } from '$lib/components/ui/skeleton/index.js';
  import {
    SelectAPINode,
    URLTestAPIGroup,
    SetAPIGroupFilter
  } from '../../../bindings/sing-box-ez/internal/gui/wails/bindings.js';
  import type { Node as APINode } from '../../../bindings/sing-box-ez/internal/core/state/models.js';

  // Property names are derived from the keys: dots camelize, underscores
  // stay ("main.groups.empty" → L.mainGroupsEmpty).
  const L = useLocale([
    'tab.groups',
    'main.groups.empty',
    'main.connections.search',
    'main.connections.filter'
  ]);

  // Core API state arrives from the backend via api:state events; the page
  // never polls.
  const apiStatus = $derived($appState.api.status);
  const apiGroups = $derived($appState.api.groups);

  let expandedGroups = $state<Set<string>>(new Set());
  let testingGroups = $state<Set<string>>(new Set());
  let groupDelays = $state<Record<string, Record<string, number>>>({});

  // Group search is applied by the backend: the query is debounced into
  // SetAPIGroupFilter and the filtered list arrives with the next api:state
  // push. The page never filters the store locally.
  let groupSearch = $state('');
  // The search lives in the filter dialog so the header stays compact.
  let showGroupFilter = $state(false);
  $effect(() => {
    const q = groupSearch.trim();
    const timer = setTimeout(() => {
      SetAPIGroupFilter(q).catch(() => {
        // The backend already reported the failure with a toast.
      });
    }, 300);
    return () => clearTimeout(timer);
  });
  // The filter lives in the backend poller; clear it when leaving so other
  // consumers of the group list (e.g. the overview outbound chain) see all
  // groups again.
  onDestroy(() => {
    SetAPIGroupFilter('').catch(() => {});
  });

  const visibleGroups = $derived(
    apiGroups.filter((g) => g.type !== 'Fallback' && g.type !== 'LoadBalance')
  );

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
</script>

<Page>
  <Card.Root>
    <Card.Header>
      <div class="flex items-center justify-between gap-2">
        <Card.Title>{L.tabGroups}</Card.Title>
        <Button
          variant="outline"
          size="sm"
          class={groupSearch.trim() ? 'text-primary' : ''}
          onclick={() => (showGroupFilter = true)}
          aria-label={L.mainConnectionsFilter}
        >
          <Filter size={16} />
          <span class="hidden sm:inline">{L.mainConnectionsFilter}</span>
        </Button>
      </div>
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
                  <Badge variant="outline">{`${group.delay} ms`}</Badge>
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
                  <!-- Class interpolation stays inside the plain class
                       attribute: the i18n hardcode-scan flags multi-word
                       literals in cn() calls. -->
                  <button
                    class="w-full px-4 py-2 flex items-center justify-between hover:bg-accent transition {node.tag === group.selected ? 'bg-primary text-primary-foreground hover:bg-primary/90' : ''}"
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
</Page>

<!-- Filter dialog: currently holds only the group-name search (applied by
     the backend via SetAPIGroupFilter); new filter variables go here. -->
<Dialog.Root open={showGroupFilter} onOpenChange={(open) => { if (!open) showGroupFilter = false; }}>
  <Dialog.Content>
    <Dialog.Header>
      <Dialog.Title>{L.mainConnectionsFilter}</Dialog.Title>
    </Dialog.Header>
    <Input
      placeholder={L.mainConnectionsSearch}
      bind:value={groupSearch}
      autofocus
    />
  </Dialog.Content>
</Dialog.Root>
