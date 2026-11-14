<script module lang="ts">
  import type {
    VersionInfo,
    UpdateChannel,
    UpdaterEntry
  } from '../../../bindings/sing-box-ez/internal/gui/wails/models.js';
  import type { ChannelInfo } from '../../../bindings/sing-box-ez/internal/framework/updater/models.js';

  // Module-level snapshot cache: revisiting About renders the last data
  // instantly (no skeleton flash) while load() refreshes it in the
  // background.
  const aboutCache: {
    version: VersionInfo | null;
    branches: UpdateChannel[];
    updaters: UpdaterEntry[];
    updateChannel: ChannelInfo | null;
    coreLoaded: boolean;
  } = { version: null, branches: [], updaters: [], updateChannel: null, coreLoaded: false };
</script>

<script lang="ts">
  import { ExternalLink, FolderOpen, GitBranch, Download, FileText, RefreshCw } from '@lucide/svelte';
  import { toast } from 'svelte-sonner';
  import { fly } from 'svelte/transition';
  import { appState } from '../stores/appState.js';
  import { useLocale } from '@he11ah0und/localengine-web';
  import { subNav } from '../stores/navigation.js';
  import Page from '../components/Page.svelte';
  import UpdateCard from '../components/UpdateCard.svelte';
  import * as Card from '$lib/components/ui/card/index.js';
  import { Button } from '$lib/components/ui/button/index.js';
  import { Progress } from '$lib/components/ui/progress/index.js';
  import { Skeleton } from '$lib/components/ui/skeleton/index.js';
  import { Badge } from '$lib/components/ui/badge/index.js';
  import * as Dialog from '$lib/components/ui/dialog/index.js';
  import * as AlertDialog from '$lib/components/ui/alert-dialog/index.js';
  import { cn } from '$lib/utils.js';
  import {
    GetVersionInfo,
    GetBranches,
    GetUpdaters,
    GetUpdateChannel,
    CheckSelfUpdate,
    InstallSelfUpdate,
    GetCoreInfo,
    DownloadCore,
    DownloadCoreVersion,
    ListCoreVersions,
    IsCoreManaged,
    SetCoreSourceMode,
    SetConfigValues,
    CancelUpdate,
    OpenDataDir,
    OpenUpdaterProjectURL,
    OpenReleaseURL,
    GetReleaseNotes,
    CheckExternalUpdate,
    GetLatestCoreVersion
  } from '../../../bindings/sing-box-ez/internal/gui/wails/bindings.js';
  import type { ExternalUpdateInfo } from '../../../bindings/sing-box-ez/internal/gui/wails/models.js';
  import type { SelfUpdateInfo } from '../../../bindings/sing-box-ez/internal/gui/wails/models.js';
  import type { CoreRelease } from '../../../bindings/sing-box-ez/internal/core/models.js';

  let version = $state<VersionInfo | null>(aboutCache.version);
  let branches = $state<UpdateChannel[]>(aboutCache.branches);
  let updaters = $state<UpdaterEntry[]>(aboutCache.updaters);
  let updateChannel = $state<ChannelInfo | null>(aboutCache.updateChannel);
  let externalInfo = $state<ExternalUpdateInfo | null>(null);
  let checkingExternal = $state(false);
  let checkingCore = $state(false);
  let appUpdateOpen = $state(false);
  let coreUpdateOpen = $state(false);
  let currentBranch = $state(aboutCache.version?.branch ?? '');
  let selfUpdate = $state<SelfUpdateInfo | null>(null);
  let checking = $state(false);
  let installing = $state(false);
  let showInstallConfirm = $state(false);
  let releaseNotes = $state('');
  let showNotes = $state(false);
  let showBranchPicker = $state(false);
  let coreLoaded = $state(aboutCache.coreLoaded);
  let coreProcessing = $state(false);
  let coreManaged = $state(true);
  let showVersionPicker = $state(false);
  let showSourcePicker = $state(false);
  let sourceBusy = $state(false);
  let showChannelPicker = $state(false);
  let channelBusy = $state(false);
  let coreVersions = $state<CoreRelease[]>([]);
  let versionsLoading = $state(false);
  // Downloads run in a modal: set to the updater whose transfer is active.
  let updateModal = $state<'app' | 'core' | null>(null);
  let cancelling = $state(false);

  async function cancelDownload() {
    if (!updateModal || cancelling) return;
    cancelling = true;
    try {
      await CancelUpdate(updateModal);
    } finally {
      cancelling = false;
    }
  }

  const L = useLocale([
    'tab.about',
    'tab.core',
    'common.system',
    'common.cancel',
    'log.tab.app',
    'about.commit_info.prefix',
    'about.build_info.prefix',
    'about.dev_build.label',
    'about.btn.open_repo',
    'about.btn.release_notes',
    'about.btn.open_release_notes',
    'about.update.title',
    'about.branch.label',
    'about.btn.switch_branch',
    'about.update.downloading',
    'about.btn.check_update',
    'about.btn.install_update',
    'about.update.confirm',
    'startup.continue',
    'about.btn.open_data',
    'about.release_notes.title',
    'about.updaters.repository',
    'about.updaters.backend',
    'about.backend_name.github',
    'about.channel.label',
    'about.channel.auto',
    'core.installed',
    'core.latest',
    'core.btn.download',
    'core.update.downloading',
    'core.update.installing',
    'core.pick_version',
    'core.change_source',
    'core.change_channel',
    'core.prerelease',
    'core.external_managed',
    'settings.core.source.mode.label',
    'settings.core.source.mode.official',
    'settings.core.source.mode.system',
    'settings.core.source.mode.custom',
    'settings.core.source.channel.label',
    'settings.core.source.channel.stable',
    'settings.core.source.channel.beta',
    'about.channel.external_managed',
    'about.channel.external_hint',
    'about.update.latest',
    'about.update.up_to_date',
    'configs.dialog.btn.copy'
  ]);

  // The updates tab lists every updater declared in the project spec; the
  // specialized bodies below are keyed by the updater's spec name.
  const updaterTitles = $derived<Record<string, string>>({
    updater: L.logTabApp,
    'core-updater': L.tabCore
  });

  async function load() {
    // Version info is compile-time data: show the System card instantly
    // instead of holding it back behind the slower core/channel calls.
    try {
      const v = await GetVersionInfo();
      version = v;
      currentBranch = v?.branch ?? 'main';
      aboutCache.version = v;
    } catch (err) {
      toast.error(String(err));
    }
    try {
      const ch = await GetUpdateChannel();
      updateChannel = ch ?? null;
      const external = updateChannel?.external ?? false;
      // On external channels there is no branch list to fetch at all, so the
      // pointless GetBranches call (and its guard error) is skipped.
      const [u, core, managed, br, ext] = await Promise.all([
        GetUpdaters(),
        GetCoreInfo(),
        IsCoreManaged(),
        external ? Promise.resolve([]) : GetBranches().catch(() => []),
        external ? CheckExternalUpdate().catch(() => null) : Promise.resolve(null)
      ]);
      coreManaged = managed;
      updateChannel = ch ?? null;
      updaters = u ?? [];
      branches = br ?? [];
      externalInfo = ext;
      aboutCache.branches = branches;
      aboutCache.updaters = updaters;
      aboutCache.updateChannel = updateChannel;
      appState.update((s) => ({ ...s, coreInfo: { ...s.coreInfo, ...core } }));
    } catch (err) {
      toast.error(String(err));
    } finally {
      coreLoaded = true;
      aboutCache.coreLoaded = true;
    }
  }

  // Pick up a background update check result delivered via selfupdate:available,
  // both on mount and if the event arrives while the page is open.
  $effect(() => {
    if (!selfUpdate && $appState.selfUpdateInfo) {
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

  async function checkExternalUpdate() {
    checkingExternal = true;
    try {
      externalInfo = await CheckExternalUpdate();
      // A found update is presented in a dialog with the update command.
      if (externalInfo?.outdated) appUpdateOpen = true;
    } catch {
      externalInfo = null;
    } finally {
      checkingExternal = false;
    }
  }

  // Manual core version check, presented through the same UpdateCard dialog.
  function copyUpdateCommand() {
    // External channels update via the package manager, so the dialog offers
    // to copy the command instead of downloading anything in-app.
    if (externalInfo?.hint) void navigator.clipboard.writeText(externalInfo.hint);
  }

  async function checkCoreVersion() {
    checkingCore = true;
    try {
      const v = await GetLatestCoreVersion();
      appState.update((s) => ({ ...s, coreInfo: { ...s.coreInfo, latestVersion: v } }));
      if (v && $appState.coreInfo.installedVersion && v !== $appState.coreInfo.installedVersion) {
        coreUpdateOpen = true;
      }
    } catch (err) {
      toast.error(String(err));
    } finally {
      checkingCore = false;
    }
  }

  async function installUpdate() {
    installing = true;
    updateModal = 'app';
    try {
      await InstallSelfUpdate(currentBranch);
    } catch {
      // The backend already reported the failure with a toast.
    } finally {
      installing = false;
      // No completion progress event arrives on cancel/failure; reset the
      // flag so the auto-close effect below can fire.
      appState.update((s) => ({ ...s, selfUpdate: { ...s.selfUpdate, downloading: false } }));
    }
  }

  async function downloadCore() {
    coreProcessing = true;
    updateModal = 'core';
    try {
      await DownloadCore();
      const core = await GetCoreInfo();
      appState.update((s) => ({ ...s, coreInfo: { ...s.coreInfo, ...core } }));
    } catch {
      // The backend already reported the failure with a toast.
    } finally {
      coreProcessing = false;
      appState.update((s) => ({
        ...s,
        coreInfo: { ...s.coreInfo, downloading: false, installing: false }
      }));
    }
  }

  async function openVersionPicker() {
    showVersionPicker = true;
    if (coreVersions.length > 0 || versionsLoading) return;
    versionsLoading = true;
    try {
      coreVersions = (await ListCoreVersions()) ?? [];
    } catch {
      // The backend already reported the failure with a toast.
    } finally {
      versionsLoading = false;
    }
  }

  async function downloadCoreVersion(ver: string) {
    showVersionPicker = false;
    coreProcessing = true;
    updateModal = 'core';
    try {
      await DownloadCoreVersion(ver);
      const core = await GetCoreInfo();
      appState.update((s) => ({ ...s, coreInfo: { ...s.coreInfo, ...core } }));
    } catch {
      // The backend already reported the failure with a toast.
    } finally {
      coreProcessing = false;
      appState.update((s) => ({
        ...s,
        coreInfo: { ...s.coreInfo, downloading: false, installing: false }
      }));
    }
  }

  // The download modal closes itself once the transfer (or the failed
  // attempt) is over; closing it by hand leaves the download running in
  // the background — the backend reports the result with a toast.
  $effect(() => {
    if (updateModal === 'app' && !installing && !$appState.selfUpdate.downloading) {
      updateModal = null;
    }
    if (updateModal === 'core' && !coreProcessing && !$appState.coreInfo.downloading) {
      updateModal = null;
    }
  });

  const modalIsApp = $derived(updateModal === 'app');
  const modalDownloading = $derived(
    modalIsApp ? $appState.selfUpdate.downloading : $appState.coreInfo.downloading
  );
  const modalProgress = $derived(
    (modalIsApp ? $appState.selfUpdate.downloadProgress : $appState.coreInfo.downloadProgress) ?? 0
  );

  async function openUpdaterRepo(name: string) {
    await OpenUpdaterProjectURL(name);
  }

  async function openDataDir() {
    await OpenDataDir();
  }

  async function fetchReleaseNotes() {
    try {
      const body = await GetReleaseNotes(version?.versionTag || version?.commit || 'latest');
      releaseNotes = body;
      showNotes = true;
    } catch {
      // The backend already reported the failure with a toast.
    }
  }

  async function openReleaseNotes() {
    const tag = version?.versionTag || version?.commit || 'latest';
    await OpenReleaseURL(tag);
  }

  function selectBranch(name: string) {
    currentBranch = name;
    showBranchPicker = false;
    selfUpdate = null;
    void checkUpdate();
  }

  // Update backends are chosen at compile time; known ids get a display name.
  function backendName(id: string): string {
    return id === 'github' ? L.aboutBackend_nameGithub : id;
  }

  // The core source lives in the config (core.source.mode); the updates tab
  // switches it like the app branch picker switches channels.
  const coreSourceMode = $derived(($appState.settings?.['core.source.mode'] as string) || 'official');
  const coreSourceModes = ['official', 'system', 'custom'] as const;

  function coreSourceLabel(mode: string): string {
    switch (mode) {
      case 'system': return L.settingsCoreSourceModeSystem;
      case 'custom': return L.settingsCoreSourceModeCustom;
      default: return L.settingsCoreSourceModeOfficial;
    }
  }

  async function setCoreSource(mode: string) {
    if (sourceBusy || mode === coreSourceMode) return;
    sourceBusy = true;
    try {
      await SetCoreSourceMode(mode);
      // The mode swap changes the binary, its version and managed status.
      coreManaged = await IsCoreManaged();
      const core = await GetCoreInfo();
      appState.update((s) => ({ ...s, coreInfo: { ...s.coreInfo, ...core } }));
      showSourcePicker = false;
    } catch {
      // The backend already reported the failure with a toast.
    } finally {
      sourceBusy = false;
    }
  }

  // The core release channel (core.source.channel) only applies to the
  // managed core; it lives in the config like the source mode does.
  const coreChannel = $derived(($appState.settings?.['core.source.channel'] as string) || 'stable');
  const coreChannels = ['stable', 'beta'] as const;

  function coreChannelLabel(ch: string): string {
    return ch === 'beta' ? L.settingsCoreSourceChannelBeta : L.settingsCoreSourceChannelStable;
  }

  async function setCoreChannel(ch: string) {
    if (channelBusy || ch === coreChannel) return;
    channelBusy = true;
    try {
      await SetConfigValues({ 'core.source.channel': ch });
      // Re-resolve the latest version for the new channel.
      const core = await GetCoreInfo();
      appState.update((s) => ({ ...s, coreInfo: { ...s.coreInfo, ...core } }));
      showChannelPicker = false;
    } catch {
      // The backend already reported the failure with a toast.
    } finally {
      channelBusy = false;
    }
  }
</script>

{#snippet appUpdaterBody(u: UpdaterEntry)}
  <!-- App self-updater: channel picker, check, install, progress. -->
  <div class="space-y-4">
    <p class="text-sm text-muted-foreground">
      {L.aboutChannelLabel}{updateChannel?.name ?? ''} ({L.aboutChannelAuto})
    </p>
    <div class="flex items-center justify-between gap-2">
      <p class="text-sm text-muted-foreground">{L.aboutBranchLabel} {currentBranch}</p>
      <Button variant="outline" size="sm" onclick={() => showBranchPicker = true}>
        <GitBranch size={16} />
        {L.aboutBtnSwitch_branch}
      </Button>
    </div>
    {#if version?.isDev}
      <p class="text-sm text-primary">{L.aboutDev_buildLabel}</p>
    {/if}
    <UpdateCard
      label={`${L.aboutUpdatersBackend}: ${backendName(u.sourceBackend)}`}
      current={selfUpdate?.current || version?.version || ''}
      latest={selfUpdate?.latest || ''}
      {checking}
      currentLabel={L.coreInstalled}
      latestLabel={L.coreLatest}
      checkLabel={L.aboutBtnCheck_update}
      updateTitle={L.aboutUpdateTitle}
      closeLabel={L.commonCancel}
      onCheck={checkUpdate}
    />
    <div class="flex flex-wrap gap-3">
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
                {L.aboutBtnInstall_update}
              </Button>
            {/snippet}
          </AlertDialog.Trigger>
          <AlertDialog.Content>
            <AlertDialog.Header>
              <AlertDialog.Title>{L.aboutBtnInstall_update}</AlertDialog.Title>
              <AlertDialog.Description>
                {L.aboutUpdateConfirm}
              </AlertDialog.Description>
            </AlertDialog.Header>
            <AlertDialog.Footer>
              <AlertDialog.Cancel>{L.commonCancel}</AlertDialog.Cancel>
              <AlertDialog.Action onclick={installUpdate}>
                {L.startupContinue}
              </AlertDialog.Action>
            </AlertDialog.Footer>
          </AlertDialog.Content>
        </AlertDialog.Root>
      {/if}
    </div>
  </div>
{/snippet}

{#snippet externalUpdaterBody()}
  <!-- Externally managed builds check updates through the package channel, not the self-update backend. -->
  <UpdateCard
    label={`${L.aboutUpdatersBackend}: ${updateChannel?.name ?? ''}`}
    current={version?.version ?? ''}
    latest={externalInfo?.latest ?? ''}
    checking={checkingExternal}
    currentLabel={L.coreInstalled}
    latestLabel={L.coreLatest}
    checkLabel={L.aboutBtnCheck_update}
    updateTitle={L.aboutUpdateTitle}
    closeLabel={L.commonCancel}
    actionLabel={L.configsDialogBtnCopy}
    onAction={copyUpdateCommand}
    bind:open={appUpdateOpen}
    onCheck={checkExternalUpdate}
  />
{/snippet}

{#snippet coreUpdaterBody(u: UpdaterEntry)}
  <!-- Core updater: installed/latest versions and the download action. -->
  <div class="space-y-4">
    <div class="flex items-center justify-between gap-2">
      <p class="text-sm text-muted-foreground">
        {L.settingsCoreSourceModeLabel}: {coreSourceLabel(coreSourceMode)}
      </p>
      <Button variant="outline" size="sm" onclick={() => showSourcePicker = true}>
        {L.coreChange_source}
      </Button>
    </div>
    {#if coreManaged}
      <!-- The channel only matters while the app manages the core. -->
      <div class="flex items-center justify-between gap-2">
        <p class="text-sm text-muted-foreground">
          {L.settingsCoreSourceChannelLabel}: {coreChannelLabel(coreChannel)}
        </p>
        <Button variant="outline" size="sm" onclick={() => showChannelPicker = true}>
          {L.coreChange_channel}
        </Button>
      </div>
    {/if}
    {#if !coreLoaded}
      <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <Skeleton class="h-[76px] w-full rounded-xl" />
        <Skeleton class="h-[76px] w-full rounded-xl" />
      </div>
    {:else}
      <UpdateCard
        label={`${L.aboutUpdatersBackend}: ${backendName(u.sourceBackend)}`}
        current={$appState.coreInfo.installedVersion || ''}
        latest={$appState.coreInfo.latestVersion || ''}
        checking={checkingCore}
        currentLabel={L.coreInstalled}
        latestLabel={L.coreLatest}
        checkLabel={L.aboutBtnCheck_update}
        updateTitle={L.tabCore}
        closeLabel={L.commonCancel}
        actionLabel={L.coreBtnDownload}
        onAction={downloadCore}
        bind:open={coreUpdateOpen}
        onCheck={checkCoreVersion}
      />
    {/if}
    {#if coreManaged}
      <div class="flex flex-wrap gap-3">
        {#if !$appState.coreInfo.installedVersion || $appState.coreInfo.latestVersion !== $appState.coreInfo.installedVersion}
          <Button disabled={coreProcessing} onclick={downloadCore}>
            <Download size={16} class={coreProcessing ? 'animate-bounce' : ''} />
            {L.coreBtnDownload}
          </Button>
        {/if}
        <Button variant="outline" disabled={coreProcessing} onclick={openVersionPicker}>
          {L.corePick_version}
        </Button>
      </div>
    {:else}
      <!-- System/custom core: updates happen outside the app. -->
      <p class="text-sm text-muted-foreground">{L.coreExternal_managed}</p>
    {/if}
  </div>
{/snippet}

<Page
  title={L.tabAbout}
  onLoad={load}
>
  <!-- Tab switches animate like the root pages do. -->
  {#key $subNav.activeTab}
    <div class="space-y-6" in:fly={{ y: 8, duration: 150 }}>
  {#if ($subNav.activeTab ?? 'info') === 'updates'}
    <!-- Every updater declared in the project spec gets a card; known
         updaters get a specialized body, unknown ones a generic one. -->
    <div class="space-y-6">
      {#each updaters as u (u.name)}
        <Card.Root>
          <Card.Header>
            <Card.Title>{updaterTitles[u.name] ?? u.name}</Card.Title>
            <Card.Description>{L.aboutUpdatersRepository}: {u.owner}/{u.repo}</Card.Description>
            <Card.Action>
              <Button variant="outline" size="sm" onclick={() => openUpdaterRepo(u.name)}>
                <ExternalLink size={16} />
                {L.aboutBtnOpen_repo}
              </Button>
            </Card.Action>
          </Card.Header>
          <Card.Content>
            {#if u.name === 'updater'}
              {#if updateChannel?.external}
                {@render externalUpdaterBody()}
              {:else}
                {@render appUpdaterBody(u)}
              {/if}
            {:else if u.name === 'core-updater'}
              {@render coreUpdaterBody(u)}
            {:else}
              <p class="text-sm text-muted-foreground">{u.sourceBackend} → {u.applyBackend}</p>
            {/if}
          </Card.Content>
        </Card.Root>
      {/each}
    </div>
  {:else}
    {#if version}
      <Card.Root>
        <Card.Header>
          <Card.Title>{L.commonSystem}</Card.Title>
          <Card.Description>{version.buildFlags}</Card.Description>
        </Card.Header>
        <Card.Content class="space-y-3">
          <p class="text-sm text-muted-foreground">
            {#if version.version}v{version.version} · {/if}
            {L.aboutCommit_infoPrefix} {version.branch}
            {#if version.commit}, {version.commit}{/if}
            {#if version.commitDate}, {version.commitDate}{/if}
            {#if version.humanCommit} ({version.humanCommit}){/if}
          </p>
          <p class="text-sm text-muted-foreground">
            {L.aboutBuild_infoPrefix} {version.buildDate || '—'}
            {#if version.humanBuild} ({version.humanBuild}){/if}
          </p>
          {#if version.isDev}
            <p class="text-sm text-primary">{L.aboutDev_buildLabel}</p>
          {/if}

          <div class="flex flex-wrap gap-3 pt-2">
            {#if !version.isDev}
              <Button variant="outline" onclick={fetchReleaseNotes}>
                <FileText size={16} />
                {L.aboutBtnRelease_notes}
              </Button>
              <Button variant="outline" onclick={openReleaseNotes}>
                <ExternalLink size={16} />
                {L.aboutBtnOpen_release_notes}
              </Button>
            {/if}
          </div>
        </Card.Content>
      </Card.Root>
    {:else}
      <!-- Version info is still on its way (first run): shimmer placeholder. -->
      <Card.Root>
        <Card.Header>
          <Card.Title>{L.commonSystem}</Card.Title>
        </Card.Header>
        <Card.Content class="space-y-3">
          <Skeleton class="h-4 w-3/4" />
          <Skeleton class="h-4 w-1/2" />
          <Skeleton class="h-9 w-40 rounded-xl" />
        </Card.Content>
      </Card.Root>
    {/if}

    <Card.Root>
      <Card.Content>
        <Button variant="outline" onclick={openDataDir}>
          <FolderOpen size={16} />
          {L.aboutBtnOpen_data}
        </Button>
      </Card.Content>
    </Card.Root>
  {/if}
    </div>
  {/key}
</Page>

<!-- Active download/update runs in a modal; it auto-closes on completion
     or cancel. The X button is hidden and overlay/Escape closes are
     prevented — only the cancel button stops the transfer. -->
<Dialog.Root open={updateModal !== null} onOpenChange={() => {}}>
  <Dialog.Content
    showCloseButton={false}
    onInteractOutside={(e: Event) => { e.preventDefault(); }}
    onEscapeKeydown={(e: Event) => { e.preventDefault(); }}
  >
    <Dialog.Header>
      <Dialog.Title>{modalIsApp ? L.aboutUpdateTitle : L.tabCore}</Dialog.Title>
    </Dialog.Header>
    <div class="space-y-2">
      <div class="flex justify-between text-sm">
        <span>
          {modalIsApp
            ? L.aboutUpdateDownloading
            : $appState.coreInfo.installing
              ? L.coreUpdateInstalling
              : L.coreUpdateDownloading}
        </span>
        {#if modalDownloading}
          <span>{Math.round(modalProgress * 100)}%</span>
        {/if}
      </div>
      {#if modalDownloading}
        <Progress value={modalProgress * 100} max={100} />
      {:else}
        <!-- No byte total yet (or the apply phase): indeterminate state. -->
        <div class="flex justify-center py-2">
          <RefreshCw size={20} class="animate-spin text-muted-foreground" />
        </div>
      {/if}
      <div class="flex justify-end pt-2">
        <Button variant="outline" disabled={cancelling} onclick={cancelDownload}>
          {L.commonCancel}
        </Button>
      </div>
    </div>
  </Dialog.Content>
</Dialog.Root>

<Dialog.Root open={showNotes} onOpenChange={(open) => { if (!open) showNotes = false; }}>
  <Dialog.Content class="sm:max-w-2xl">
    <Dialog.Header>
      <Dialog.Title>{L.aboutRelease_notesTitle}</Dialog.Title>
    </Dialog.Header>
    <div class="max-w-none whitespace-pre-wrap overflow-auto max-h-[70vh] text-sm">{releaseNotes}</div>
  </Dialog.Content>
</Dialog.Root>

<Dialog.Root open={showBranchPicker} onOpenChange={(open) => { if (!open) showBranchPicker = false; }}>
  <Dialog.Content>
    <Dialog.Header>
      <Dialog.Title>{L.aboutBtnSwitch_branch}</Dialog.Title>
    </Dialog.Header>
    <div class="flex flex-col gap-2">
      {#each branches as branch (branch.id)}
        <button
          class={cn(
            'w-full text-left px-4 py-3 rounded-xl border border-border bg-secondary hover:bg-accent transition',
            branch.name === currentBranch && 'text-primary'
          )}
          onclick={() => { selectBranch(branch.name); }}
        >
          {branch.name} {#if branch.name === currentBranch}✓{/if}
        </button>
      {/each}
    </div>
  </Dialog.Content>
</Dialog.Root>

<!-- Official core version picker: all GitHub releases, pre-releases flagged. -->
<Dialog.Root open={showVersionPicker} onOpenChange={(open) => { if (!open) showVersionPicker = false; }}>
  <Dialog.Content>
    <Dialog.Header>
      <Dialog.Title>{L.corePick_version}</Dialog.Title>
    </Dialog.Header>
    {#if versionsLoading}
      <div class="space-y-2">
        <Skeleton class="h-10 w-full rounded-xl" />
        <Skeleton class="h-10 w-full rounded-xl" />
        <Skeleton class="h-10 w-full rounded-xl" />
      </div>
    {:else}
      <ul class="max-h-72 overflow-auto divide-y divide-border rounded-xl border border-border">
        {#each coreVersions as rel (rel.version)}
          <li>
            <button
              class="w-full text-left px-4 py-3 hover:bg-accent transition flex items-center justify-between gap-3"
              onclick={() => downloadCoreVersion(rel.version)}
            >
              <span class="font-medium">{rel.version}</span>
              <span class="flex items-center gap-2 text-sm text-muted-foreground">
                {#if rel.prerelease}
                  <Badge variant="secondary">{L.corePrerelease}</Badge>
                {/if}
                {rel.publishedAt}
              </span>
            </button>
          </li>
        {/each}
      </ul>
    {/if}
  </Dialog.Content>
</Dialog.Root>

<!-- Core source picker: official (managed download), system binary or a
     custom path. Custom asks for a file when none is configured. -->
<Dialog.Root open={showSourcePicker} onOpenChange={(open) => { if (!open) showSourcePicker = false; }}>
  <Dialog.Content>
    <Dialog.Header>
      <Dialog.Title>{L.coreChange_source}</Dialog.Title>
    </Dialog.Header>
    <div class="flex flex-col gap-2">
      {#each coreSourceModes as mode (mode)}
        <button
          class={cn(
            'w-full text-left px-4 py-3 rounded-xl border border-border bg-secondary hover:bg-accent transition',
            mode === coreSourceMode && 'text-primary',
            sourceBusy && 'opacity-50 pointer-events-none'
          )}
          disabled={sourceBusy}
          onclick={() => setCoreSource(mode)}
        >
          {coreSourceLabel(mode)} {#if mode === coreSourceMode}✓{/if}
        </button>
      {/each}
    </div>
  </Dialog.Content>
</Dialog.Root>

<!-- Core release channel picker: stable tracks the latest stable release,
     beta also considers pre-releases. Managed (official) cores only. -->
<Dialog.Root open={showChannelPicker} onOpenChange={(open) => { if (!open) showChannelPicker = false; }}>
  <Dialog.Content>
    <Dialog.Header>
      <Dialog.Title>{L.settingsCoreSourceChannelLabel}</Dialog.Title>
    </Dialog.Header>
    <div class="flex flex-col gap-2">
      {#each coreChannels as ch (ch)}
        <button
          class={cn(
            'w-full text-left px-4 py-3 rounded-xl border border-border bg-secondary hover:bg-accent transition',
            ch === coreChannel && 'text-primary',
            channelBusy && 'opacity-50 pointer-events-none'
          )}
          disabled={channelBusy}
          onclick={() => setCoreChannel(ch)}
        >
          {coreChannelLabel(ch)} {#if ch === coreChannel}✓{/if}
        </button>
      {/each}
    </div>
  </Dialog.Content>
</Dialog.Root>
