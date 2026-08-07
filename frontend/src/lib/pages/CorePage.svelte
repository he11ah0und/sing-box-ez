<script module lang="ts">
  import { Cpu } from '@lucide/svelte';
  export const pageMeta = {
    id: 'core',
    key: 'tab.core',
    icon: Cpu,
    nav: false,
    bottomNav: false
  };
</script>

<script lang="ts">
  import { Download, RefreshCw } from '@lucide/svelte';
  import { toast } from 'svelte-sonner';
  import { appState } from '../stores/appState.js';
  import { locale, tValue } from '../stores/locale.js';
  import Page from '../components/Page.svelte';
  import * as Card from '$lib/components/ui/card/index.js';
  import { Button } from '$lib/components/ui/button/index.js';
  import { Progress } from '$lib/components/ui/progress/index.js';
  import { Skeleton } from '$lib/components/ui/skeleton/index.js';
  import { GetCoreInfo, DownloadCore } from '../../../bindings/sing-box-ez/internal/gui/wails/bindings.js';

  let processing = $state(false);
  let loaded = $state(false);

  async function load() {
    try {
      const info = await GetCoreInfo();
      appState.update((s) => ({ ...s, coreInfo: { ...s.coreInfo, ...info } }));
    } catch (err) {
      toast.error(String(err));
    } finally {
      loaded = true;
    }
  }

  async function download() {
    processing = true;
    try {
      await DownloadCore();
      await load();
    } catch (err) {
      toast.error(String(err));
    } finally {
      processing = false;
    }
  }
</script>

<Page
  title={tValue($locale, 'tab.core', 'Core')}
  onLoad={load}
>
  <Card.Root>
    <Card.Content class="space-y-4">
      {#if !loaded}
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <Skeleton class="h-[76px] w-full rounded-xl" />
          <Skeleton class="h-[76px] w-full rounded-xl" />
        </div>
      {:else}
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <div class="rounded-xl bg-background border border-border p-4">
            <p class="text-sm text-muted-foreground">{tValue($locale, 'core.installed', 'Installed version')}</p>
            <p class="text-lg font-medium">{$appState.coreInfo.installedVersion || '—'}</p>
          </div>
          <div class="rounded-xl bg-background border border-border p-4">
            <p class="text-sm text-muted-foreground">{tValue($locale, 'core.latest', 'Latest version')}</p>
            <p class="text-lg font-medium">{$appState.coreInfo.latestVersion || '—'}</p>
          </div>
        </div>
      {/if}

      {#if $appState.coreInfo.downloading}
        <div class="space-y-1">
          <div class="flex justify-between text-sm">
            <span>{tValue($locale, 'core.update.downloading', 'Downloading…')}</span>
            <span>{Math.round(($appState.coreInfo.downloadProgress ?? 0) * 100)}%</span>
          </div>
          <Progress value={($appState.coreInfo.downloadProgress ?? 0) * 100} max={100} />
        </div>
      {/if}

      <div class="flex flex-wrap gap-3">
        <Button
          disabled={processing || $appState.coreInfo.downloading}
          onclick={download}
        >
          <Download size={18} />
          {tValue($locale, 'core.btn.download', 'Download / update core')}
        </Button>
        <Button variant="outline" onclick={load}>
          <RefreshCw size={18} />
          {tValue($locale, 'common.refresh', 'Refresh')}
        </Button>
      </div>
    </Card.Content>
  </Card.Root>
</Page>
