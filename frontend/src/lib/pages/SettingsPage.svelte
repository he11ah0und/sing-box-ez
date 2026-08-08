<script lang="ts">
  import { Save, RotateCcw, ShieldCheck, Trash2 } from '@lucide/svelte';
  import { toast } from 'svelte-sonner';
  import { appState } from '../stores/appState.js';
  import { locale, useLocale, useLocaleRecord } from '../stores/locale.svelte.js';
  import { theme, applyTheme, fromThemePayload } from '../stores/theme.js';
  import { subNav } from '../stores/navigation.js';
  import Page from '../components/Page.svelte';
  import * as Card from '$lib/components/ui/card/index.js';
  import { Button } from '$lib/components/ui/button/index.js';
  import { Input } from '$lib/components/ui/input/index.js';
  import { Label } from '$lib/components/ui/label/index.js';
  import * as Select from '$lib/components/ui/select/index.js';
  import { Switch } from '$lib/components/ui/switch/index.js';
  import { Separator } from '$lib/components/ui/separator/index.js';
  import { Skeleton } from '$lib/components/ui/skeleton/index.js';
  import * as AlertDialog from '$lib/components/ui/alert-dialog/index.js';
  import {
    GetSettings,
    SaveSettings,
    GetTheme,
    GetThemeNames,
    GetAvailableLanguages,
    SetLanguage,
    GetPrivilegeTabState,
    RestartAsAdmin,
    ApplySetcap,
    ResetData
  } from '../../../bindings/sing-box-ez/internal/gui/wails/bindings.js';
  import type {
    Settings,
    LanguageOption,
    PrivilegeTabState
  } from '../../../bindings/sing-box-ez/internal/gui/wails/models.js';

  let processing = $state(false);

  // The form seeds from the cached settings snapshot (fetched once by the
  // bridge) so revisits render instantly; GetSettings runs again only when
  // the snapshot is missing or the user hits Reset.
  const defaultSettings: Settings = {
    language: 'en',
    theme: 'default',
    themeMode: 'system',
    autoStartCore: false,
    autoRestart: false,
    runAsAdmin: false,
    logLimit: 100,
    desktopNotifications: true,
    autoCheckCore: true,
    autoCheckSelf: true,
    defaultIntervalHours: 24,
    proxyEnabled: false,
    urlTestURL: '',
    coreLogLevel: 'info',
    trafficGraphHistory: 60,
    showLogs: false,
    autoUpdateConfigs: false,
    autoUpdateConfigsIntervalHours: 24,
    autoUpdateOnHashMismatch: false,
    autoRestartOnConfigUpdate: false,
    backgroundUpdateCheckIntervalHours: 2
  };
  let form = $state<Settings>({ ...defaultSettings, ...$appState.settings });

  let themeNames = $state<string[]>([]);
  let languages = $state<LanguageOption[]>([]);
  let privState = $state<PrivilegeTabState | null>(null);
  let privProcessing = $state(false);
  let confirmRestartAdmin = $state(false);
  let confirmSetcap = $state(false);
  let confirmReset = $state(false);

  // Theme mode "system" maps to the system_based locale key; the config
  // value itself stays "system". Log levels debug/info live in common.*.
  const logLevelKeys: Record<string, string> = {
    debug: 'common.debug',
    info: 'common.info',
    warn: 'settings.log_level.warn',
    error: 'settings.log_level.error'
  };

  const R = useLocaleRecord([
    'settings.theme_mode.system_based',
    'settings.theme_mode.dark',
    'settings.theme_mode.light',
    ...Object.values(logLevelKeys)
  ]);

  const themeModeLabel = $derived(
    R[`settings.theme_mode.${form.themeMode === 'system' ? 'system_based' : form.themeMode}`]
  );
  const coreLogLevelLabel = $derived(
    R[logLevelKeys[form.coreLogLevel] ?? `settings.log_level.${form.coreLogLevel}`]
  );

  const L = useLocale([
    'tab.settings',
    'common.reset',
    'common.save',
    'common.cancel',
    'dialog.btn.confirm',
    'core.start_on_launch',
    'core.auto_restart',
    'core.proxy.enabled',
    'core.url_test_url.label',
    'core.log.level',
    'core.graph_history.label',
    'core.privileges.title',
    'core.btn.restart_admin',
    'core.btn.apply_setcap',
    'core.mode.setcap_prompt',
    'settings.runAsAdmin',
    'common.debug',
    'common.info',
    'settings.log_level.warn',
    'settings.log_level.error',
    'settings.system.mode',
    'settings.system.restart_admin_confirm',
    'settings.reset.title',
    'settings.reset.btn',
    'settings.reset.confirm_title',
    'settings.reset.confirm_msg',
    'settings.language.title',
    'settings.theme.title',
    'settings.theme_mode.title',
    'settings.theme_mode.system_based',
    'settings.theme_mode.dark',
    'settings.theme_mode.light',
    'settings.log_limit.label',
    'settings.default_interval.label',
    'settings.config_update.interval',
    'settings.config_update.background_interval',
    'settings.desktop_notifications',
    'settings.show_logs',
    'settings.update_check.core',
    'settings.update_check.self',
    'settings.config_update.auto',
    'settings.config_update.hash_mismatch',
    'settings.config_update.auto_restart'
  ]);

  function colorStyle(color: string): string {
    if (color === 'green') return 'color: var(--color-success)';
    if (color === 'yellow') return 'color: var(--color-warning)';
    return '';
  }

  async function loadPrivileges() {
    try {
      privState = await GetPrivilegeTabState();
    } catch (err) {
      toast.error(String(err));
    }
  }

  $effect(() => {
    if ($subNav.activeTab === 'system') loadPrivileges();
  });

  async function load(force = false) {
    try {
      const needSettings = force || !$appState.settingsLoaded;
      const [s, names, langs] = await Promise.all([
        needSettings ? GetSettings() : Promise.resolve(null),
        GetThemeNames(),
        GetAvailableLanguages()
      ]);
      if (s) {
        form = { ...form, ...s };
        appState.update((state) => ({ ...state, settings: s, settingsLoaded: true }));
      }
      themeNames = names && names.length > 0 ? names : ['default'];
      languages = langs && langs.length > 0 ? langs : [{ code: 'en', name: 'English' }];
    } catch (err) {
      toast.error(String(err));
    }
  }

  async function save() {
    processing = true;
    try {
      await SaveSettings({ ...form });
      appState.update((state) => ({ ...state, settings: { ...form } }));
      const [t, l] = await Promise.all([GetTheme(), SetLanguage(form.language)]);
      if (t) {
        const data = fromThemePayload(t);
        theme.set(data);
        applyTheme(data);
      }
      if (l) {
        locale.set({ language: l.language, values: (l.values ?? {}) as Record<string, string> });
      }
    } catch {
      // The backend already reported the result with a toast.
    } finally {
      processing = false;
    }
  }

  function reset() {
    load(true);
  }

  async function restartAsAdmin() {
    privProcessing = true;
    try {
      await RestartAsAdmin();
    } catch {
      // The backend already reported the failure with a toast.
    } finally {
      privProcessing = false;
    }
  }

  async function applySetcap() {
    privProcessing = true;
    try {
      await ApplySetcap();
      await loadPrivileges();
    } catch {
      // The backend already reported the failure with a toast.
    } finally {
      privProcessing = false;
    }
  }

  async function resetData() {
    // ResetData deletes the data files and quits the app; the call may never resolve.
    await ResetData().catch(() => {});
  }
</script>

<Page
  title={L.tabSettings}
  onLoad={load}
>
  {#snippet actions()}
    <Button variant="outline" onclick={reset}>
      <RotateCcw size={16} />
      {L.commonReset}
    </Button>
    <Button disabled={processing} onclick={save}>
      <Save size={16} />
      {L.commonSave}
    </Button>
  {/snippet}

  <Card.Root>
    <Card.Content class="space-y-5">
      {#if $subNav.activeTab === 'core'}
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <Label class="flex items-center gap-3 rounded-xl bg-background border border-border p-3 cursor-pointer">
            <Switch bind:checked={form.autoStartCore} />
            <span>{L.coreStart_on_launch}</span>
          </Label>
          <Label class="flex items-center gap-3 rounded-xl bg-background border border-border p-3 cursor-pointer">
            <Switch bind:checked={form.autoRestart} />
            <span>{L.coreAuto_restart}</span>
          </Label>
          <Label class="flex items-center gap-3 rounded-xl bg-background border border-border p-3 cursor-pointer">
            <Switch bind:checked={form.runAsAdmin} />
            <span>{L.settingsRunAsAdmin}</span>
          </Label>
          <Label class="flex items-center gap-3 rounded-xl bg-background border border-border p-3 cursor-pointer">
            <Switch bind:checked={form.proxyEnabled} />
            <span>{L.coreProxyEnabled}</span>
          </Label>
        </div>

        <div class="grid grid-cols-1 sm:grid-cols-2 gap-5">
          <div class="space-y-1 sm:col-span-2">
            <Label for="settings-url-test">{L.coreUrl_test_urlLabel}</Label>
            <Input id="settings-url-test" bind:value={form.urlTestURL} />
          </div>

          <div class="space-y-1">
            <Label>{L.coreLogLevel}</Label>
            <Select.Root type="single" bind:value={form.coreLogLevel}>
              <Select.Trigger class="w-full">{coreLogLevelLabel}</Select.Trigger>
              <Select.Content>
                <Select.Item value="debug" label={L.commonDebug} />
                <Select.Item value="info" label={L.commonInfo} />
                <Select.Item value="warn" label={L.settingsLog_levelWarn} />
                <Select.Item value="error" label={L.settingsLog_levelError} />
              </Select.Content>
            </Select.Root>
          </div>

          <div class="space-y-1">
            <Label>{L.coreGraph_historyLabel}</Label>
            <Select.Root
              type="single"
              value={String(form.trafficGraphHistory)}
              onValueChange={(v) => (form.trafficGraphHistory = Number(v))}
            >
              <Select.Trigger class="w-full">{form.trafficGraphHistory}</Select.Trigger>
              <Select.Content>
                {#each [30, 60, 120, 300] as n (n)}
                  <Select.Item value={String(n)} label={String(n)} />
                {/each}
              </Select.Content>
            </Select.Root>
          </div>
        </div>
      {:else if $subNav.activeTab === 'system'}
        <div class="space-y-3">
          <p class="font-medium flex items-center gap-2">
            <ShieldCheck size={16} />
            {L.corePrivilegesTitle}
          </p>
          {#if privState}
            <p class="text-sm text-muted-foreground">
              {L.settingsSystemMode}: {privState.mode}
            </p>
            {#if privState.adminStatusText}
              <p class="text-sm" style={colorStyle(privState.adminStatusColor)}>{privState.adminStatusText}</p>
            {/if}
            {#if privState.privilegeText}
              <p class="text-sm" style={colorStyle(privState.privilegeColor)}>{privState.privilegeText}</p>
            {/if}
            {#if privState.adminLabel}
              <p class="text-sm text-muted-foreground">{privState.adminLabel}</p>
            {/if}
            {#if privState.showRestartAdminBtn || privState.showSetcapBtn}
              <div class="flex flex-wrap gap-2 pt-1">
                {#if privState.showRestartAdminBtn}
                  <Button variant="outline" disabled={privProcessing} onclick={() => (confirmRestartAdmin = true)}>
                    {L.coreBtnRestart_admin}
                  </Button>
                {/if}
                {#if privState.showSetcapBtn}
                  <Button variant="outline" disabled={privProcessing} onclick={() => (confirmSetcap = true)}>
                    {L.coreBtnApply_setcap}
                  </Button>
                {/if}
              </div>
            {/if}
          {:else}
            <Skeleton class="h-20 w-full rounded-xl" />
          {/if}
        </div>

        <Separator />

        <div class="space-y-3">
          <p class="font-medium text-destructive">{L.settingsResetTitle}</p>
          <Button variant="destructive" onclick={() => (confirmReset = true)}>
            <Trash2 size={16} />
            {L.settingsResetBtn}
          </Button>
        </div>
      {:else}
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-5">
          <div class="space-y-1">
            <Label>{L.settingsLanguageTitle}</Label>
            <Select.Root type="single" bind:value={form.language}>
              <Select.Trigger class="w-full">
                {languages.find((l) => l.code === form.language)?.name ?? form.language}
              </Select.Trigger>
              <Select.Content>
                {#each languages as lang (lang.code)}
                  <Select.Item value={lang.code} label={lang.name} />
                {/each}
              </Select.Content>
            </Select.Root>
          </div>

          <div class="space-y-1">
            <Label>{L.settingsThemeTitle}</Label>
            <Select.Root type="single" bind:value={form.theme}>
              <Select.Trigger class="w-full">{form.theme}</Select.Trigger>
              <Select.Content>
                {#each themeNames as name (name)}
                  <Select.Item value={name} label={name} />
                {/each}
              </Select.Content>
            </Select.Root>
          </div>

          <div class="space-y-1">
            <Label>{L.settingsTheme_modeTitle}</Label>
            <Select.Root type="single" bind:value={form.themeMode}>
              <Select.Trigger class="w-full">{themeModeLabel}</Select.Trigger>
              <Select.Content>
                <Select.Item value="system" label={L.settingsTheme_modeSystem_based} />
                <Select.Item value="dark" label={L.settingsTheme_modeDark} />
                <Select.Item value="light" label={L.settingsTheme_modeLight} />
              </Select.Content>
            </Select.Root>
          </div>

          <div class="space-y-1">
            <Label for="settings-log-limit">{L.settingsLog_limitLabel}</Label>
            <Input id="settings-log-limit" type="number" min="10" bind:value={form.logLimit} />
          </div>

          <div class="space-y-1">
            <Label for="settings-interval">{L.settingsDefault_intervalLabel}</Label>
            <Input id="settings-interval" type="number" min="1" bind:value={form.defaultIntervalHours} />
          </div>

          <div class="space-y-1">
            <Label for="settings-config-interval">{L.settingsConfig_updateInterval}</Label>
            <Input id="settings-config-interval" type="number" min="1" bind:value={form.autoUpdateConfigsIntervalHours} />
          </div>

          <div class="space-y-1">
            <Label for="settings-bg-interval">{L.settingsConfig_updateBackground_interval}</Label>
            <Input id="settings-bg-interval" type="number" min="1" bind:value={form.backgroundUpdateCheckIntervalHours} />
          </div>
        </div>

        <Separator />

        <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <Label class="flex items-center gap-3 rounded-xl bg-background border border-border p-3 cursor-pointer">
            <Switch bind:checked={form.desktopNotifications} />
            <span>{L.settingsDesktop_notifications}</span>
          </Label>
          <Label class="flex items-center gap-3 rounded-xl bg-background border border-border p-3 cursor-pointer">
            <Switch bind:checked={form.showLogs} />
            <span>{L.settingsShow_logs}</span>
          </Label>
          <Label class="flex items-center gap-3 rounded-xl bg-background border border-border p-3 cursor-pointer">
            <Switch bind:checked={form.autoCheckCore} />
            <span>{L.settingsUpdate_checkCore}</span>
          </Label>
          <Label class="flex items-center gap-3 rounded-xl bg-background border border-border p-3 cursor-pointer">
            <Switch bind:checked={form.autoCheckSelf} />
            <span>{L.settingsUpdate_checkSelf}</span>
          </Label>
          <Label class="flex items-center gap-3 rounded-xl bg-background border border-border p-3 cursor-pointer">
            <Switch bind:checked={form.autoUpdateConfigs} />
            <span>{L.settingsConfig_updateAuto}</span>
          </Label>
          <Label class="flex items-center gap-3 rounded-xl bg-background border border-border p-3 cursor-pointer">
            <Switch bind:checked={form.autoUpdateOnHashMismatch} />
            <span>{L.settingsConfig_updateHash_mismatch}</span>
          </Label>
          <Label class="flex items-center gap-3 rounded-xl bg-background border border-border p-3 cursor-pointer">
            <Switch bind:checked={form.autoRestartOnConfigUpdate} />
            <span>{L.settingsConfig_updateAuto_restart}</span>
          </Label>
        </div>
      {/if}
    </Card.Content>
  </Card.Root>

  <AlertDialog.Root bind:open={confirmRestartAdmin}>
    <AlertDialog.Content>
      <AlertDialog.Header>
        <AlertDialog.Title>{L.coreBtnRestart_admin}</AlertDialog.Title>
        <AlertDialog.Description>
          {L.settingsSystemRestart_admin_confirm}
        </AlertDialog.Description>
      </AlertDialog.Header>
      <AlertDialog.Footer>
        <AlertDialog.Cancel>{L.commonCancel}</AlertDialog.Cancel>
        <AlertDialog.Action onclick={restartAsAdmin}>
          {L.dialogBtnConfirm}
        </AlertDialog.Action>
      </AlertDialog.Footer>
    </AlertDialog.Content>
  </AlertDialog.Root>

  <AlertDialog.Root bind:open={confirmSetcap}>
    <AlertDialog.Content>
      <AlertDialog.Header>
        <AlertDialog.Title>{L.coreBtnApply_setcap}</AlertDialog.Title>
        <AlertDialog.Description>
          {L.coreModeSetcap_prompt}
        </AlertDialog.Description>
      </AlertDialog.Header>
      <AlertDialog.Footer>
        <AlertDialog.Cancel>{L.commonCancel}</AlertDialog.Cancel>
        <AlertDialog.Action onclick={applySetcap}>
          {L.dialogBtnConfirm}
        </AlertDialog.Action>
      </AlertDialog.Footer>
    </AlertDialog.Content>
  </AlertDialog.Root>

  <AlertDialog.Root bind:open={confirmReset}>
    <AlertDialog.Content>
      <AlertDialog.Header>
        <AlertDialog.Title>{L.settingsResetConfirm_title}</AlertDialog.Title>
        <AlertDialog.Description class="whitespace-pre-wrap">
          {L.settingsResetConfirm_msg}
        </AlertDialog.Description>
      </AlertDialog.Header>
      <AlertDialog.Footer>
        <AlertDialog.Cancel>{L.commonCancel}</AlertDialog.Cancel>
        <AlertDialog.Action
          class="bg-destructive text-white hover:bg-destructive/90"
          onclick={resetData}
        >
          {L.settingsResetBtn}
        </AlertDialog.Action>
      </AlertDialog.Footer>
    </AlertDialog.Content>
  </AlertDialog.Root>
</Page>
