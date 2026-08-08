<script lang="ts">
  import { ExternalLink, FolderOpen, GitBranch, Download, FileText, RefreshCw } from '@lucide/svelte';
  import { toast } from 'svelte-sonner';
  import { appState } from '../stores/appState.js';
  import { useLocale } from '../stores/locale.js';
  import Page from '../components/Page.svelte';
  import * as Card from '$lib/components/ui/card/index.js';
  import { Button } from '$lib/components/ui/button/index.js';
  import { Progress } from '$lib/components/ui/progress/index.js';
  import * as Dialog from '$lib/components/ui/dialog/index.js';
  import * as AlertDialog from '$lib/components/ui/alert-dialog/index.js';
  import { cn } from '$lib/utils.js';
  import {
    GetVersionInfo,
    GetBranches,
    CheckSelfUpdate,
    InstallSelfUpdate,
    OpenDataDir,
    OpenProjectURL,
    OpenReleaseURL,
    GetReleaseNotes
  } from '../../../bindings/sing-box-ez/internal/gui/wails/bindings.js';
  import type {
    VersionInfo,
    UpdateChannel,
    SelfUpdateInfo
  } from '../../../bindings/sing-box-ez/internal/gui/wails/models.js';

  let version = $state<VersionInfo | null>(null);
  let branches = $state<UpdateChannel[]>([]);
  let currentBranch = $state('');
  let selfUpdate = $state<SelfUpdateInfo | null>(null);
  let checking = $state(false);
  let installing = $state(false);
  let showInstallConfirm = $state(false);
  let releaseNotes = $state('');
  let showNotes = $state(false);
  let showBranchPicker = $state(false);

  const tabAbout = useLocale('tab.about');
  const aboutSystemTitle = useLocale('common.system');
  const aboutCommitInfoPrefix = useLocale('about.commit_info.prefix');
  const aboutBuildInfoPrefix = useLocale('about.build_info.prefix');
  const aboutDevBuildLabel = useLocale('about.dev_build.label');
  const aboutBtnOpenRepo = useLocale('about.btn.open_repo');
  const aboutBtnReleaseNotes = useLocale('about.btn.release_notes');
  const aboutBtnOpenReleaseNotes = useLocale('about.btn.open_release_notes');
  const aboutUpdateTitle = useLocale('about.update.title');
  const aboutBranchLabel = useLocale('about.branch.label');
  const aboutBtnSwitchBranch = useLocale('about.btn.switch_branch');
  const aboutUpdateDownloading = useLocale('about.update.downloading');
  const aboutBtnCheckUpdate = useLocale('about.btn.check_update');
  const aboutBtnInstallUpdate = useLocale('about.btn.install_update');
  const aboutUpdateConfirm = useLocale('about.update.confirm');
  const commonCancel = useLocale('common.cancel');
  const startupContinue = useLocale('startup.continue');
  const aboutBtnOpenData = useLocale('about.btn.open_data');
  const aboutReleaseNotesTitle = useLocale('about.release_notes.title');

  async function load() {
    try {
      version = await GetVersionInfo();
      currentBranch = version?.branch ?? 'main';
      const list = await GetBranches();
      branches = list ?? [];
    } catch (err) {
      toast.error(String(err));
    }
  }

  // Pick up a background update check result delivered via selfupdate:available,
  // both on mount and if the event arrives while the page is open.
  $effect(() => {
    if (!selfUpdate && $appState.selfUpdateInfo?.hasUpdate) {
      selfUpdate = $appState.selfUpdateInfo;
    }
  });

  async function checkUpdate() {
    checking = true;
    try {
      // Result and error toasts are emitted by the backend.
      selfUpdate = await CheckSelfUpdate(currentBranch);
    } catch {
      selfUpdate = null;
    } finally {
      checking = false;
    }
  }

  async function installUpdate() {
    installing = true;
    try {
      await InstallSelfUpdate(currentBranch);
    } catch {
      // The backend already reported the failure with a toast.
    } finally {
      installing = false;
    }
  }

  async function openRepo() {
    await OpenProjectURL();
  }

  async function openDataDir() {
    await OpenDataDir();
  }

  async function fetchReleaseNotes() {
    try {
      const body = await GetReleaseNotes(version?.commit || 'latest');
      releaseNotes = body;
      showNotes = true;
    } catch {
      // The backend already reported the failure with a toast.
    }
  }

  async function openReleaseNotes() {
    const tag = version?.commit || 'latest';
    await OpenReleaseURL(tag);
  }

  function selectBranch(name: string) {
    currentBranch = name;
    showBranchPicker = false;
    selfUpdate = null;
    checkUpdate();
  }
</script>

<Page
  title={$tabAbout}
  onLoad={load}
>
  {#if version}
    <Card.Root>
      <Card.Header>
        <Card.Title>{$aboutSystemTitle}</Card.Title>
        <Card.Description>{version.buildFlags}</Card.Description>
      </Card.Header>
      <Card.Content class="space-y-3">
        <p class="text-sm text-muted-foreground">
          {$aboutCommitInfoPrefix} {version.branch}
          {#if version.commit}, {version.commit}{/if}
          {#if version.commitDate}, {version.commitDate}{/if}
        </p>
        <p class="text-sm text-muted-foreground">
          {$aboutBuildInfoPrefix} {version.buildDate || '—'}
        </p>
        {#if version.isDev}
          <p class="text-sm text-primary">{$aboutDevBuildLabel}</p>
        {/if}

        <div class="flex flex-wrap gap-3 pt-2">
          <Button onclick={openRepo}>
            <ExternalLink size={16} />
            {$aboutBtnOpenRepo}
          </Button>
          {#if !version.isDev}
            <Button variant="outline" onclick={fetchReleaseNotes}>
              <FileText size={16} />
              {$aboutBtnReleaseNotes}
            </Button>
            <Button variant="outline" onclick={openReleaseNotes}>
              <ExternalLink size={16} />
              {$aboutBtnOpenReleaseNotes}
            </Button>
          {/if}
        </div>
      </Card.Content>
    </Card.Root>
  {/if}

  <Card.Root>
    <Card.Header>
      <div class="flex items-center justify-between">
        <div>
          <Card.Title>{$aboutUpdateTitle}</Card.Title>
          <Card.Description>{$aboutBranchLabel} {currentBranch}</Card.Description>
        </div>
        <Button variant="outline" onclick={() => showBranchPicker = true}>
          <GitBranch size={16} />
          {$aboutBtnSwitchBranch}
        </Button>
      </div>
    </Card.Header>
    <Card.Content class="space-y-4">
      {#if $appState.selfUpdate?.downloading}
        <div class="space-y-1">
          <div class="flex justify-between text-sm">
            <span>{$aboutUpdateDownloading}</span>
            <span>{Math.round(($appState.selfUpdate.downloadProgress ?? 0) * 100)}%</span>
          </div>
          <Progress value={($appState.selfUpdate.downloadProgress ?? 0) * 100} max={100} />
        </div>
      {/if}

      <div class="flex flex-wrap gap-3">
        <Button
          disabled={checking || installing}
          onclick={checkUpdate}
        >
          <RefreshCw size={16} class={checking ? 'animate-spin' : ''} />
          {$aboutBtnCheckUpdate}
        </Button>
        {#if selfUpdate?.hasUpdate}
          <AlertDialog.Root bind:open={showInstallConfirm}>
            <AlertDialog.Trigger>
              {#snippet child({ props })}
                <Button
                  {...props}
                  class="bg-[var(--color-success)] text-white hover:opacity-90"
                  disabled={installing}
                >
                  <Download size={16} />
                  {$aboutBtnInstallUpdate}
                </Button>
              {/snippet}
            </AlertDialog.Trigger>
            <AlertDialog.Content>
              <AlertDialog.Header>
                <AlertDialog.Title>{$aboutBtnInstallUpdate}</AlertDialog.Title>
                <AlertDialog.Description>
                  {$aboutUpdateConfirm}
                </AlertDialog.Description>
              </AlertDialog.Header>
              <AlertDialog.Footer>
                <AlertDialog.Cancel>{$commonCancel}</AlertDialog.Cancel>
                <AlertDialog.Action onclick={installUpdate}>
                  {$startupContinue}
                </AlertDialog.Action>
              </AlertDialog.Footer>
            </AlertDialog.Content>
          </AlertDialog.Root>
        {/if}
      </div>
    </Card.Content>
  </Card.Root>

  <Card.Root>
    <Card.Content>
      <Button variant="outline" onclick={openDataDir}>
        <FolderOpen size={16} />
        {$aboutBtnOpenData}
      </Button>
    </Card.Content>
  </Card.Root>
</Page>

<Dialog.Root open={showNotes} onOpenChange={(open) => { if (!open) showNotes = false; }}>
  <Dialog.Content class="sm:max-w-2xl">
    <Dialog.Header>
      <Dialog.Title>{$aboutReleaseNotesTitle}</Dialog.Title>
    </Dialog.Header>
    <div class="max-w-none whitespace-pre-wrap overflow-auto max-h-[70vh] text-sm">{releaseNotes}</div>
  </Dialog.Content>
</Dialog.Root>

<Dialog.Root open={showBranchPicker} onOpenChange={(open) => { if (!open) showBranchPicker = false; }}>
  <Dialog.Content>
    <Dialog.Header>
      <Dialog.Title>{$aboutBtnSwitchBranch}</Dialog.Title>
    </Dialog.Header>
    <div class="flex flex-col gap-2">
      {#each branches as branch (branch.id)}
        <button
          class={cn(
            'w-full text-left px-4 py-3 rounded-xl border border-border bg-secondary hover:bg-accent transition',
            branch.name === currentBranch && 'text-primary'
          )}
          onclick={() => selectBranch(branch.name)}
        >
          {branch.name} {#if branch.name === currentBranch}✓{/if}
        </button>
      {/each}
    </div>
  </Dialog.Content>
</Dialog.Root>
