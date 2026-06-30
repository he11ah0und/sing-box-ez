<script module>
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

<script>
  import { Trash2, RefreshCw } from '@lucide/svelte';
  import { appState, clearLogs } from '../stores/appState.js';
  import { locale, tValue } from '../stores/locale.js';
  import { subNav } from '../stores/navigation.js';
  import Page from '../components/Page.svelte';
  import { parseANSILine, parseAppLogLine, parseCoreLogLine } from '../utils/ansi.js';
  import {
    GetAppLogs,
    GetCoreLogs,
    ClearAppLogs,
    ClearCoreLogs
  } from '../../../bindings/sing-box-ez/internal/gui/wails/bindings.js';

  let processing = $state(false);

  function colorizeCore(line) {
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
    } catch (err) {
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
    <button
      class="flex items-center gap-2 px-3 py-2 rounded-xl border border-[var(--color-border)] bg-[var(--color-surface-variant)] hover:bg-[var(--color-border)] transition"
      onclick={load}
    >
      <RefreshCw size={16} />
      {tValue($locale, 'common.refresh', 'Refresh')}
    </button>
    <button
      class="flex items-center gap-2 px-3 py-2 rounded-xl bg-[var(--color-danger)] text-white hover:opacity-90 transition"
      disabled={processing}
      onclick={clear}
    >
      <Trash2 size={16} />
      {tValue($locale, 'common.clear', 'Clear')}
    </button>
  {/snippet}

  <div class="flex-1 min-h-0 rounded-2xl border border-[var(--color-border)] bg-[var(--color-surface)] p-4 overflow-auto font-mono text-sm">
    {#if $subNav.activeTab === 'core'}
      {#if $appState.logs.core.length === 0}
        <p class="text-[var(--color-text-muted)]">{tValue($locale, 'log.empty', 'No logs yet.')}</p>
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
        <p class="text-[var(--color-text-muted)]">{tValue($locale, 'log.empty', 'No logs yet.')}</p>
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
</Page>
