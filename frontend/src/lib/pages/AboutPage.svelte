<script module>
  import { Info } from '@lucide/svelte';
  export const pageMeta = {
    id: 'about',
    key: 'tab.about',
    icon: Info,
    nav: true,
    bottomNav: true,
    order: 4
  };
</script>

<script>
  import { ExternalLink, FolderOpen, GitBranch, Download, FileText, RefreshCw } from '@lucide/svelte';
  import { appState } from '../stores/appState.js';
  import { locale, tValue } from '../stores/locale.js';
  import Page from '../components/Page.svelte';
  import Modal from '../components/Modal.svelte';
  import {
    GetVersionInfo,
    GetBranches,
    CheckSelfUpdate,
    InstallSelfUpdate,
    OpenDataDir,
    OpenURL,
    GetReleaseNotes
  } from '../../../bindings/sing-box-ez/internal/gui/wails/bindings.js';

  let version = $state(null);
  let branches = $state([]);
  let currentBranch = $state('');
  let selfUpdate = $state(null);
  let checking = $state(false);
  let installing = $state(false);
  let message = $state('');
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
      message = String(err);
    }
  }

  async function checkUpdate() {
    checking = true;
    message = '';
    try {
      selfUpdate = await CheckSelfUpdate(currentBranch);
    } catch (err) {
      message = String(err);
      selfUpdate = null;
    } finally {
      checking = false;
    }
  }

  async function installUpdate() {
    if (!confirm(tValue($locale, 'about.update.confirm', 'Install update?'))) return;
    installing = true;
    try {
      await InstallSelfUpdate(currentBranch);
      message = tValue($locale, 'about.update.installed', 'Update installed');
    } catch (err) {
      message = String(err);
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
      message = String(err);
    }
  }

  async function openReleaseNotes() {
    const tag = version?.commit || 'latest';
    await OpenURL(`https://github.com/he11ah0und/sing-box-ez/releases/tag/${tag}`);
  }

  function selectBranch(name) {
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
    <section class="rounded-2xl border border-[var(--color-border)] bg-[var(--color-surface)] p-5 shadow-sm space-y-3">
      <h3 class="font-medium">{tValue($locale, 'about.system.title', 'System')}</h3>
      <p class="text-sm text-[var(--color-text-muted)]">{version.buildFlags}</p>
      <p class="text-sm text-[var(--color-text-muted)]">
        {tValue($locale, 'about.commit_info.prefix', 'Commit:')} {version.branch}
        {#if version.commit}, {version.commit}{/if}
        {#if version.commitDate}, {version.commitDate}{/if}
      </p>
      <p class="text-sm text-[var(--color-text-muted)]">
        {tValue($locale, 'about.build_info.prefix', 'Build:')} {version.buildDate || '—'}
      </p>
      {#if version.isDev}
        <p class="text-sm text-[var(--color-primary)]">{tValue($locale, 'about.dev_build.label', 'Development build')}</p>
      {/if}

      <div class="flex flex-wrap gap-3 pt-2">
        <button class="flex items-center gap-2 px-4 py-2 rounded-xl bg-[var(--color-primary)] text-white hover:opacity-90 transition" onclick={openRepo}>
          <ExternalLink size={16} />
          {tValue($locale, 'about.btn.open_repo', 'Open repo')}
        </button>
        {#if !version.isDev}
          <button class="flex items-center gap-2 px-4 py-2 rounded-xl border border-[var(--color-border)] bg-[var(--color-surface-variant)] hover:bg-[var(--color-border)] transition" onclick={fetchReleaseNotes}>
            <FileText size={16} />
            {tValue($locale, 'about.btn.release_notes', 'Release notes')}
          </button>
          <button class="flex items-center gap-2 px-4 py-2 rounded-xl border border-[var(--color-border)] bg-[var(--color-surface-variant)] hover:bg-[var(--color-border)] transition" onclick={openReleaseNotes}>
            <ExternalLink size={16} />
            {tValue($locale, 'about.btn.open_release_notes', 'Open release notes')}
          </button>
        {/if}
      </div>
    </section>
  {/if}

  <section class="rounded-2xl border border-[var(--color-border)] bg-[var(--color-surface)] p-5 shadow-sm space-y-4">
    <div class="flex items-center justify-between">
      <div>
        <h3 class="font-medium">{tValue($locale, 'about.update.title', 'App update')}</h3>
        <p class="text-sm text-[var(--color-text-muted)]">{tValue($locale, 'about.branch.label', 'Branch:')} {currentBranch}</p>
      </div>
      <button
        class="flex items-center gap-2 px-3 py-2 rounded-xl border border-[var(--color-border)] bg-[var(--color-surface-variant)] hover:bg-[var(--color-border)] transition"
        onclick={() => showBranchPicker = true}
      >
        <GitBranch size={16} />
        {tValue($locale, 'about.btn.switch_branch', 'Switch branch')}
      </button>
    </div>

    {#if selfUpdate}
      <div class="rounded-xl bg-[var(--color-bg)] p-4 space-y-2">
        <p class="text-sm">{tValue($locale, 'about.update.current_version', 'Current:')} {selfUpdate.current}</p>
        <p class="text-sm">{tValue($locale, 'about.update.latest', 'Latest:')} {selfUpdate.latest}</p>
        {#if selfUpdate.hasUpdate}
          <p class="text-sm text-green-500">{tValue($locale, 'about.update.available', 'Update available')}</p>
        {:else if selfUpdate.isDevBuild}
          <p class="text-sm text-[var(--color-warning)]">{tValue($locale, 'about.update.dev_build', 'Development build')}</p>
        {:else}
          <p class="text-sm text-[var(--color-text-muted)]">{tValue($locale, 'about.update.up_to_date', 'Up to date')}</p>
        {/if}
      </div>
    {/if}

    {#if $appState.selfUpdate?.downloading}
      <div class="space-y-1">
        <div class="flex justify-between text-sm">
          <span>{tValue($locale, 'about.update.downloading', 'Downloading…')}</span>
          <span>{Math.round(($appState.selfUpdate.downloadProgress ?? 0) * 100)}%</span>
        </div>
        <div class="h-2 rounded-full bg-[var(--color-border)] overflow-hidden">
          <div class="h-full bg-[var(--color-primary)] transition-all" style="width: {Math.round(($appState.selfUpdate.downloadProgress ?? 0) * 100)}%"></div>
        </div>
      </div>
    {/if}

    <div class="flex flex-wrap gap-3">
      <button
        class="flex items-center gap-2 px-4 py-2 rounded-xl bg-[var(--color-primary)] text-white disabled:opacity-50 hover:opacity-90 transition"
        disabled={checking || installing}
        onclick={checkUpdate}
      >
        {#if checking}
          <RefreshCw size={16} class="animate-spin" />
        {:else}
          <RefreshCw size={16} />
        {/if}
        {tValue($locale, 'about.btn.check_update', 'Check update')}
      </button>
      {#if selfUpdate?.hasUpdate}
        <button
          class="flex items-center gap-2 px-4 py-2 rounded-xl bg-[var(--color-success)] text-white disabled:opacity-50 hover:opacity-90 transition"
          disabled={installing}
          onclick={installUpdate}
        >
          <Download size={16} />
          {tValue($locale, 'about.btn.install_update', 'Install update')}
        </button>
      {/if}
    </div>
  </section>

  <section class="rounded-2xl border border-[var(--color-border)] bg-[var(--color-surface)] p-5 shadow-sm">
    <button
      class="flex items-center gap-2 px-4 py-2 rounded-xl border border-[var(--color-border)] bg-[var(--color-surface-variant)] hover:bg-[var(--color-border)] transition"
      onclick={openDataDir}
    >
      <FolderOpen size={16} />
      {tValue($locale, 'about.btn.open_data', 'Open data folder')}
    </button>
  </section>

  {#if message}
    <p class="text-sm text-[var(--color-danger)]">{message}</p>
  {/if}
</Page>

{#if showNotes}
  <Modal title={tValue($locale, 'about.release_notes.title', 'Release notes')} onclose={() => showNotes = false}>
    <div class="prose prose-invert max-w-none whitespace-pre-wrap">{releaseNotes}</div>
  </Modal>
{/if}

{#if showBranchPicker}
  <Modal title={tValue($locale, 'about.btn.switch_branch', 'Switch branch')} onclose={() => showBranchPicker = false}>
    <div class="flex flex-col gap-2">
      {#each branches as branch (branch.id)}
        <button
          class="w-full text-left px-4 py-3 rounded-xl border border-[var(--color-border)] bg-[var(--color-surface-variant)] hover:bg-[var(--color-border)] transition"
          class:text-[var(--color-primary)]={branch.name === currentBranch}
          onclick={() => selectBranch(branch.name)}
        >
          {branch.name} {#if branch.name === currentBranch}✓{/if}
        </button>
      {/each}
    </div>
  </Modal>
{/if}
