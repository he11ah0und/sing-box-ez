<script module>
  import { Cpu } from '@lucide/svelte';
  export const pageMeta = {
    id: 'core',
    key: 'tab.core',
    icon: Cpu,
    nav: false,
    bottomNav: false
  };
</script>

<script>
  import { Download, RefreshCw } from '@lucide/svelte';
  import { appState } from '../stores/appState.js';
  import { locale, tValue } from '../stores/locale.js';
  import Page from '../components/Page.svelte';
  import { GetCoreInfo, DownloadCore } from '../../../bindings/sing-box-ez/internal/gui/wails/bindings.js';

  let processing = $state(false);
  let message = $state('');

  async function load() {
    try {
      const info = await GetCoreInfo();
      appState.update((s) => ({ ...s, coreInfo: { ...s.coreInfo, ...info } }));
    } catch (err) {
      message = String(err);
    }
  }

  async function download() {
    processing = true;
    message = '';
    try {
      await DownloadCore();
      await load();
    } catch (err) {
      message = String(err);
    } finally {
      processing = false;
    }
  }
</script>

<Page
  title={tValue($locale, 'tab.core', 'Core')}
  onLoad={load}
>
  <section class="rounded-2xl border border-[var(--color-border)] bg-[var(--color-surface)] p-5 shadow-sm space-y-4">
    <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
      <div class="rounded-xl bg-[var(--color-bg)] p-4">
        <p class="text-sm text-[var(--color-text-muted)]">{tValue($locale, 'core.installed', 'Installed version')}</p>
        <p class="text-lg font-medium">{$appState.coreInfo.installedVersion || '—'}</p>
      </div>
      <div class="rounded-xl bg-[var(--color-bg)] p-4">
        <p class="text-sm text-[var(--color-text-muted)]">{tValue($locale, 'core.latest', 'Latest version')}</p>
        <p class="text-lg font-medium">{$appState.coreInfo.latestVersion || '—'}</p>
      </div>
    </div>

    {#if $appState.coreInfo.downloading}
      <div class="space-y-1">
        <div class="flex justify-between text-sm">
          <span>{tValue($locale, 'core.update.downloading', 'Downloading…')}</span>
          <span>{Math.round(($appState.coreInfo.downloadProgress ?? 0) * 100)}%</span>
        </div>
        <div class="h-2 rounded-full bg-[var(--color-border)] overflow-hidden">
          <div
            class="h-full bg-[var(--color-primary)] transition-all"
            style="width: {Math.round(($appState.coreInfo.downloadProgress ?? 0) * 100)}%"
          ></div>
        </div>
      </div>
    {/if}

    <div class="flex flex-wrap gap-3">
      <button
        class="flex items-center gap-2 px-4 py-2 rounded-xl bg-[var(--color-primary)] text-white disabled:opacity-50 hover:opacity-90 transition"
        disabled={processing || $appState.coreInfo.downloading}
        onclick={download}
      >
        <Download size={18} />
        {tValue($locale, 'core.btn.download', 'Download / update core')}
      </button>
      <button
        class="flex items-center gap-2 px-4 py-2 rounded-xl border border-[var(--color-border)] bg-[var(--color-surface-variant)] hover:bg-[var(--color-border)] transition"
        onclick={load}
      >
        <RefreshCw size={18} />
        {tValue($locale, 'common.refresh', 'Refresh')}
      </button>
    </div>

    {#if message}
      <p class="text-sm text-[var(--color-danger)]">{message}</p>
    {/if}
  </section>
</Page>
