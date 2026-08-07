<script lang="ts">
  import { ExternalLink, FolderOpen, GitBranch, Download, FileText, RefreshCw } from '@lucide/svelte';
  import { toast } from 'svelte-sonner';
  import { appState } from '../stores/appState.js';
  import { locale, tValue } from '../stores/locale.js';
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
    OpenURL,
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

  async function checkUpdate() {
    checking = true;
    try {
      selfUpdate = await CheckSelfUpdate(currentBranch);
      if (selfUpdate?.hasUpdate) {
        toast.success(tValue($locale, 'about.update.available', 'Update available'), {
          description: `${tValue($locale, 'about.update.current_version', 'Current:')} ${selfUpdate.current} → ${tValue($locale, 'about.update.latest', 'Latest:')} ${selfUpdate.latest}`
        });
      } else if (selfUpdate?.isDevBuild) {
        toast.warning(tValue($locale, 'about.update.dev_build', 'Development build'));
      } else if (selfUpdate) {
        toast(tValue($locale, 'about.update.up_to_date', 'Up to date'));
      }
    } catch (err) {
      toast.error(String(err));
      selfUpdate = null;
    } finally {
      checking = false;
    }
  }

  async function installUpdate() {
    installing = true;
    try {
      await InstallSelfUpdate(currentBranch);
      toast.success(tValue($locale, 'about.update.installed', 'Update installed'));
    } catch (err) {
      toast.error(String(err));
    } finally {
      installing = false;
    }
  }

  async function openRepo() {
    await OpenURL('https://github.com/he11ah0und/sing-box-ez');
  }

  async function openDataDir() {
    await OpenDataDir();
  }

  async function fetchReleaseNotes() {
    try {
      const body = await GetReleaseNotes(version?.commit || 'latest');
      releaseNotes = body;
      showNotes = true;
    } catch (err) {
      toast.error(String(err));
    }
  }

  async function openReleaseNotes() {
    const tag = version?.commit || 'latest';
    await OpenURL(`https://github.com/he11ah0und/sing-box-ez/releases/tag/${tag}`);
  }

  function selectBranch(name: string) {
    currentBranch = name;
    showBranchPicker = false;
    selfUpdate = null;
    checkUpdate();
  }
</script>

<Page
  title={tValue($locale, 'tab.about', 'About')}
  onLoad={load}
>
  {#if version}
    <Card.Root>
      <Card.Header>
        <Card.Title>{tValue($locale, 'about.system.title', 'System')}</Card.Title>
        <Card.Description>{version.buildFlags}</Card.Description>
      </Card.Header>
      <Card.Content class="space-y-3">
        <p class="text-sm text-muted-foreground">
          {tValue($locale, 'about.commit_info.prefix', 'Commit:')} {version.branch}
          {#if version.commit}, {version.commit}{/if}
          {#if version.commitDate}, {version.commitDate}{/if}
        </p>
        <p class="text-sm text-muted-foreground">
          {tValue($locale, 'about.build_info.prefix', 'Build:')} {version.buildDate || '—'}
        </p>
        {#if version.isDev}
          <p class="text-sm text-primary">{tValue($locale, 'about.dev_build.label', 'Development build')}</p>
        {/if}

        <div class="flex flex-wrap gap-3 pt-2">
          <Button onclick={openRepo}>
            <ExternalLink size={16} />
            {tValue($locale, 'about.btn.open_repo', 'Open repo')}
          </Button>
          {#if !version.isDev}
            <Button variant="outline" onclick={fetchReleaseNotes}>
              <FileText size={16} />
              {tValue($locale, 'about.btn.release_notes', 'Release notes')}
            </Button>
            <Button variant="outline" onclick={openReleaseNotes}>
              <ExternalLink size={16} />
              {tValue($locale, 'about.btn.open_release_notes', 'Open release notes')}
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
          <Card.Title>{tValue($locale, 'about.update.title', 'App update')}</Card.Title>
          <Card.Description>{tValue($locale, 'about.branch.label', 'Branch:')} {currentBranch}</Card.Description>
        </div>
        <Button variant="outline" onclick={() => showBranchPicker = true}>
          <GitBranch size={16} />
          {tValue($locale, 'about.btn.switch_branch', 'Switch branch')}
        </Button>
      </div>
    </Card.Header>
    <Card.Content class="space-y-4">
      {#if $appState.selfUpdate?.downloading}
        <div class="space-y-1">
          <div class="flex justify-between text-sm">
            <span>{tValue($locale, 'about.update.downloading', 'Downloading…')}</span>
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
          {tValue($locale, 'about.btn.check_update', 'Check update')}
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
                  {tValue($locale, 'about.btn.install_update', 'Install update')}
                </Button>
              {/snippet}
            </AlertDialog.Trigger>
            <AlertDialog.Content>
              <AlertDialog.Header>
                <AlertDialog.Title>{tValue($locale, 'about.btn.install_update', 'Install update')}</AlertDialog.Title>
                <AlertDialog.Description>
                  {tValue($locale, 'about.update.confirm', 'Install update?')}
                </AlertDialog.Description>
              </AlertDialog.Header>
              <AlertDialog.Footer>
                <AlertDialog.Cancel>{tValue($locale, 'common.cancel', 'Cancel')}</AlertDialog.Cancel>
                <AlertDialog.Action onclick={installUpdate}>
                  {tValue($locale, 'startup.continue', 'Continue')}
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
        {tValue($locale, 'about.btn.open_data', 'Open data folder')}
      </Button>
    </Card.Content>
  </Card.Root>
</Page>

<Dialog.Root open={showNotes} onOpenChange={(open) => { if (!open) showNotes = false; }}>
  <Dialog.Content class="sm:max-w-2xl">
    <Dialog.Header>
      <Dialog.Title>{tValue($locale, 'about.release_notes.title', 'Release notes')}</Dialog.Title>
    </Dialog.Header>
    <div class="max-w-none whitespace-pre-wrap overflow-auto max-h-[70vh] text-sm">{releaseNotes}</div>
  </Dialog.Content>
</Dialog.Root>

<Dialog.Root open={showBranchPicker} onOpenChange={(open) => { if (!open) showBranchPicker = false; }}>
  <Dialog.Content>
    <Dialog.Header>
      <Dialog.Title>{tValue($locale, 'about.btn.switch_branch', 'Switch branch')}</Dialog.Title>
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
