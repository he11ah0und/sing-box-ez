<script module lang="ts">
  import { Bug } from '@lucide/svelte';
  export const pageMeta = {
    id: 'debug',
    key: 'tab.debug',
    icon: Bug,
    nav: true,
    bottomNav: true,
    order: 3,
    tabs: [
      { id: 'app', key: 'log.tab.app' },
      { id: 'core', key: 'log.tab.core' }
    ]
  };
</script>

<script lang="ts">
  import { Trash2, RefreshCw } from '@lucide/svelte';
  import { appState, clearLogs } from '../stores/appState.js';
  import { locale, tValue } from '../stores/locale.js';
  import { subNav } from '../stores/navigation.js';
  import Page from '../components/Page.svelte';
  import { Button } from '$lib/components/ui/button/index.js';
  import { ScrollArea } from '$lib/components/ui/scroll-area/index.js';
  import { parseANSILine, parseAppLogLine, parseCoreLogLine, type AnsiPart } from '../utils/ansi.js';
  import {
    GetAppLogs,
    GetCoreLogs,
    ClearAppLogs,
    ClearCoreLogs
  } from '../../../bindings/sing-box-ez/internal/gui/wails/bindings.js';

  let processing = $state(false);

  function colorizeCore(line: string): AnsiPart[] {
    const ansi = parseANSILine(line);
    if (ansi.some((p) => p.style.includes('color'))) {
      return ansi;
    }
    return parseCoreLogLine(line);
  }

  async function load() {
    try {
      const [app, core] = await Promise.all([GetAppLogs(), GetCoreLogs()]);
      appState.update((s) => ({
        ...s,
        logs: { app: app ?? [], core: core ?? [] }
      }));
    } catch {
      // ignore
    }
  }

  async function clear() {
    processing = true;
    try {
      if ($subNav.activeTab === 'core') {
        await ClearCoreLogs();
      } else {
        await ClearAppLogs();
      }
      clearLogs();
      await load();
    } finally {
      processing = false;
    }
  }
</script>

<Page
  title={tValue($locale, 'tab.debug', 'Debug')}
  onLoad={load}
  fullHeight={true}
  extraClass="space-y-4"
>
  {#snippet actions()}
    <Button variant="outline" onclick={load}>
      <RefreshCw size={16} />
      {tValue($locale, 'common.refresh', 'Refresh')}
    </Button>
    <Button variant="destructive" disabled={processing} onclick={clear}>
      <Trash2 size={16} />
      {tValue($locale, 'common.clear', 'Clear')}
    </Button>
  {/snippet}

  <ScrollArea class="flex-1 min-h-0 rounded-2xl border border-border bg-card">
    <div class="p-4 font-mono text-sm">
      {#if $subNav.activeTab === 'core'}
        {#if $appState.logs.core.length === 0}
          <p class="text-muted-foreground">{tValue($locale, 'log.empty', 'No logs yet.')}</p>
        {:else}
          {#each $appState.logs.core as line}
            <div class="whitespace-pre-wrap break-words py-0.5">
              {#each colorizeCore(line) as part}
                <span style={part.style}>{part.text}</span>
              {/each}
            </div>
          {/each}
        {/if}
      {:else}
        {#if $appState.logs.app.length === 0}
          <p class="text-muted-foreground">{tValue($locale, 'log.empty', 'No logs yet.')}</p>
        {:else}
          {#each $appState.logs.app as line}
            <div class="whitespace-pre-wrap break-words py-0.5">
              {#each parseAppLogLine(line) as part}
                <span style={part.style}>{part.text}</span>
              {/each}
            </div>
          {/each}
        {/if}
      {/if}
    </div>
  </ScrollArea>
</Page>
