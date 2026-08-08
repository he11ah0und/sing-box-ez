<script lang="ts">
  import { Download, RefreshCw } from '@lucide/svelte';
  import { toast } from 'svelte-sonner';
  import { appState } from '../stores/appState.js';
  import { useLocale } from '../stores/locale.js';
  import Page from '../components/Page.svelte';
  import * as Card from '$lib/components/ui/card/index.js';
  import { Button } from '$lib/components/ui/button/index.js';
  import { Progress } from '$lib/components/ui/progress/index.js';
  import { Skeleton } from '$lib/components/ui/skeleton/index.js';
  import { GetCoreInfo, DownloadCore } from '../../../bindings/sing-box-ez/internal/gui/wails/bindings.js';

  let processing = $state(false);
  let loaded = $state(false);

  const tabCore = useLocale('tab.core');
  const coreInstalled = useLocale('core.installed');
  const coreLatest = useLocale('core.latest');
  const coreUpdateDownloading = useLocale('core.update.downloading');
  const coreBtnDownload = useLocale('core.btn.download');
  const commonRefresh = useLocale('common.refresh');

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
  title={$tabCore}
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
            <p class="text-sm text-muted-foreground">{$coreInstalled}</p>
            <p class="text-lg font-medium">{$appState.coreInfo.installedVersion || '—'}</p>
          </div>
          <div class="rounded-xl bg-background border border-border p-4">
            <p class="text-sm text-muted-foreground">{$coreLatest}</p>
            <p class="text-lg font-medium">{$appState.coreInfo.latestVersion || '—'}</p>
          </div>
        </div>
      {/if}

      {#if $appState.coreInfo.downloading}
        <div class="space-y-1">
          <div class="flex justify-between text-sm">
            <span>{$coreUpdateDownloading}</span>
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
          {$coreBtnDownload}
        </Button>
        <Button variant="outline" onclick={load}>
          <RefreshCw size={18} />
          {$commonRefresh}
        </Button>
      </div>
    </Card.Content>
  </Card.Root>
</Page>
