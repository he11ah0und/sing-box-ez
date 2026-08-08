<script lang="ts">
  import { Trash2, RefreshCw, Copy } from '@lucide/svelte';
  import { appState, clearLogs } from '../stores/appState.js';
  import { useLocale } from '../stores/locale.js';
  import { subNav } from '../stores/navigation.js';
  import Page from '../components/Page.svelte';
  import { Button } from '$lib/components/ui/button/index.js';
  import { ScrollArea } from '$lib/components/ui/scroll-area/index.js';
  import { parseANSILine, parseAppLogLine, parseCoreLogLine, type AnsiPart } from '../utils/ansi.js';
  import {
    GetAppLogs,
    GetCoreLogs,
    ClearAppLogs,
    ClearCoreLogs,
    CopyLogs
  } from '../../../bindings/sing-box-ez/internal/gui/wails/bindings.js';

  let processing = $state(false);

  const tabDebug = useLocale('tab.debug');
  const logBtnCopy = useLocale('log.btn.copy');
  const commonRefresh = useLocale('common.refresh');
  const commonClear = useLocale('common.clear');
  const logEmpty = useLocale('log.empty');

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

  async function copy() {
    // The backend copies the buffer as plain text and reports it with a toast.
    await CopyLogs($subNav.activeTab === 'core' ? 'core' : 'app').catch(() => {});
  }
</script>

<Page
  title={$tabDebug}
  onLoad={load}
  fullHeight={true}
  extraClass="space-y-4"
>
  {#snippet actions()}
    <Button variant="outline" onclick={copy}>
      <Copy size={16} />
      {$logBtnCopy}
    </Button>
    <Button variant="outline" onclick={load}>
      <RefreshCw size={16} />
      {$commonRefresh}
    </Button>
    <Button variant="destructive" disabled={processing} onclick={clear}>
      <Trash2 size={16} />
      {$commonClear}
    </Button>
  {/snippet}

  <ScrollArea class="flex-1 min-h-0 rounded-2xl border border-border bg-card">
    <div class="p-4 font-mono text-sm">
      {#if $subNav.activeTab === 'core'}
        {#if $appState.logs.core.length === 0}
          <p class="text-muted-foreground">{$logEmpty}</p>
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
          <p class="text-muted-foreground">{$logEmpty}</p>
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
