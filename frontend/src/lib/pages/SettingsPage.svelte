<script lang="ts">
  import { Save, RotateCcw, ShieldCheck, Trash2 } from '@lucide/svelte';
  import { toast } from 'svelte-sonner';
  import { appState } from '../stores/appState.js';
  import { locale, tValue, useLocale } from '../stores/locale.js';
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

  let form = $state<Settings>({
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
  });

  let themeNames = $state<string[]>([]);
  let languages = $state<LanguageOption[]>([]);
  let privState = $state<PrivilegeTabState | null>(null);
  let privProcessing = $state(false);
  let confirmRestartAdmin = $state(false);
  let confirmSetcap = $state(false);
  let confirmReset = $state(false);

  const themeModeLabel = $derived(tValue($locale, `settings.theme_mode.${form.themeMode}`));
  const coreLogLevelLabel = $derived(tValue($locale, `settings.log_level.${form.coreLogLevel}`));

  const tabSettings = useLocale('tab.settings');
  const commonReset = useLocale('common.reset');
  const commonSave = useLocale('common.save');
  const commonCancel = useLocale('common.cancel');
  const dialogBtnConfirm = useLocale('dialog.btn.confirm');
  const coreStartOnLaunch = useLocale('core.start_on_launch');
  const coreAutoRestart = useLocale('core.auto_restart');
  const coreProxyEnabled = useLocale('core.proxy.enabled');
  const coreUrlTestUrlLabel = useLocale('core.url_test_url.label');
  const coreLogLevel = useLocale('core.log.level');
  const coreGraphHistoryLabel = useLocale('core.graph_history.label');
  const corePrivilegesTitle = useLocale('core.privileges.title');
  const coreBtnRestartAdmin = useLocale('core.btn.restart_admin');
  const coreBtnApplySetcap = useLocale('core.btn.apply_setcap');
  const coreModeSetcapPrompt = useLocale('core.mode.setcap_prompt');
  const settingsRunAsAdmin = useLocale('settings.runAsAdmin');
  const settingsLogLevelDebug = useLocale('settings.log_level.debug');
  const settingsLogLevelInfo = useLocale('settings.log_level.info');
  const settingsLogLevelWarn = useLocale('settings.log_level.warn');
  const settingsLogLevelError = useLocale('settings.log_level.error');
  const settingsSystemMode = useLocale('settings.system.mode');
  const settingsSystemRestartAdminConfirm = useLocale('settings.system.restart_admin_confirm');
  const settingsResetTitle = useLocale('settings.reset.title');
  const settingsResetBtn = useLocale('settings.reset.btn');
  const settingsResetConfirmTitle = useLocale('settings.reset.confirm_title');
  const settingsResetConfirmMsg = useLocale('settings.reset.confirm_msg');
  const settingsLanguageTitle = useLocale('settings.language.title');
  const settingsThemeTitle = useLocale('settings.theme.title');
  const settingsThemeModeTitle = useLocale('settings.theme_mode.title');
  const settingsThemeModeSystem = useLocale('settings.theme_mode.system');
  const settingsThemeModeDark = useLocale('settings.theme_mode.dark');
  const settingsThemeModeLight = useLocale('settings.theme_mode.light');
  const settingsLogLimitLabel = useLocale('settings.log_limit.label');
  const settingsDefaultIntervalLabel = useLocale('settings.default_interval.label');
  const settingsConfigUpdateInterval = useLocale('settings.config_update.interval');
  const settingsConfigUpdateBackgroundInterval = useLocale('settings.config_update.background_interval');
  const settingsDesktopNotifications = useLocale('settings.desktop_notifications');
  const settingsShowLogs = useLocale('settings.show_logs');
  const settingsUpdateCheckCore = useLocale('settings.update_check.core');
  const settingsUpdateCheckSelf = useLocale('settings.update_check.self');
  const settingsConfigUpdateAuto = useLocale('settings.config_update.auto');
  const settingsConfigUpdateHashMismatch = useLocale('settings.config_update.hash_mismatch');
  const settingsConfigUpdateAutoRestart = useLocale('settings.config_update.auto_restart');

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

  async function load() {
    try {
      const [s, names, langs] = await Promise.all([
        GetSettings(),
        GetThemeNames(),
        GetAvailableLanguages()
      ]);
      form = { ...form, ...s };
      themeNames = names && names.length > 0 ? names : ['default'];
      languages = langs && langs.length > 0 ? langs : [{ code: 'en', name: 'English' }];
      appState.update((state) => ({ ...state, settings: s }));
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
    load();
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
  title={$tabSettings}
  onLoad={load}
>
  {#snippet actions()}
    <Button variant="outline" onclick={reset}>
      <RotateCcw size={16} />
      {$commonReset}
    </Button>
    <Button disabled={processing} onclick={save}>
      <Save size={16} />
      {$commonSave}
    </Button>
  {/snippet}

  <Card.Root>
    <Card.Content class="space-y-5">
      {#if $subNav.activeTab === 'core'}
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <Label class="flex items-center gap-3 rounded-xl bg-background border border-border p-3 cursor-pointer">
            <Switch bind:checked={form.autoStartCore} />
            <span>{$coreStartOnLaunch}</span>
          </Label>
          <Label class="flex items-center gap-3 rounded-xl bg-background border border-border p-3 cursor-pointer">
            <Switch bind:checked={form.autoRestart} />
            <span>{$coreAutoRestart}</span>
          </Label>
          <Label class="flex items-center gap-3 rounded-xl bg-background border border-border p-3 cursor-pointer">
            <Switch bind:checked={form.runAsAdmin} />
            <span>{$settingsRunAsAdmin}</span>
          </Label>
          <Label class="flex items-center gap-3 rounded-xl bg-background border border-border p-3 cursor-pointer">
            <Switch bind:checked={form.proxyEnabled} />
            <span>{$coreProxyEnabled}</span>
          </Label>
        </div>

        <div class="grid grid-cols-1 sm:grid-cols-2 gap-5">
          <div class="space-y-1 sm:col-span-2">
            <Label for="settings-url-test">{$coreUrlTestUrlLabel}</Label>
            <Input id="settings-url-test" bind:value={form.urlTestURL} />
          </div>

          <div class="space-y-1">
            <Label>{$coreLogLevel}</Label>
            <Select.Root type="single" bind:value={form.coreLogLevel}>
              <Select.Trigger class="w-full">{coreLogLevelLabel}</Select.Trigger>
              <Select.Content>
                <Select.Item value="debug" label={$settingsLogLevelDebug} />
                <Select.Item value="info" label={$settingsLogLevelInfo} />
                <Select.Item value="warn" label={$settingsLogLevelWarn} />
                <Select.Item value="error" label={$settingsLogLevelError} />
              </Select.Content>
            </Select.Root>
          </div>

          <div class="space-y-1">
            <Label>{$coreGraphHistoryLabel}</Label>
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
            {$corePrivilegesTitle}
          </p>
          {#if privState}
            <p class="text-sm text-muted-foreground">
              {$settingsSystemMode}: {privState.mode}
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
                    {$coreBtnRestartAdmin}
                  </Button>
                {/if}
                {#if privState.showSetcapBtn}
                  <Button variant="outline" disabled={privProcessing} onclick={() => (confirmSetcap = true)}>
                    {$coreBtnApplySetcap}
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
          <p class="font-medium text-destructive">{$settingsResetTitle}</p>
          <Button variant="destructive" onclick={() => (confirmReset = true)}>
            <Trash2 size={16} />
            {$settingsResetBtn}
          </Button>
        </div>
      {:else}
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-5">
          <div class="space-y-1">
            <Label>{$settingsLanguageTitle}</Label>
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
            <Label>{$settingsThemeTitle}</Label>
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
            <Label>{$settingsThemeModeTitle}</Label>
            <Select.Root type="single" bind:value={form.themeMode}>
              <Select.Trigger class="w-full">{themeModeLabel}</Select.Trigger>
              <Select.Content>
                <Select.Item value="system" label={$settingsThemeModeSystem} />
                <Select.Item value="dark" label={$settingsThemeModeDark} />
                <Select.Item value="light" label={$settingsThemeModeLight} />
              </Select.Content>
            </Select.Root>
          </div>

          <div class="space-y-1">
            <Label for="settings-log-limit">{$settingsLogLimitLabel}</Label>
            <Input id="settings-log-limit" type="number" min="10" bind:value={form.logLimit} />
          </div>

          <div class="space-y-1">
            <Label for="settings-interval">{$settingsDefaultIntervalLabel}</Label>
            <Input id="settings-interval" type="number" min="1" bind:value={form.defaultIntervalHours} />
          </div>

          <div class="space-y-1">
            <Label for="settings-config-interval">{$settingsConfigUpdateInterval}</Label>
            <Input id="settings-config-interval" type="number" min="1" bind:value={form.autoUpdateConfigsIntervalHours} />
          </div>

          <div class="space-y-1">
            <Label for="settings-bg-interval">{$settingsConfigUpdateBackgroundInterval}</Label>
            <Input id="settings-bg-interval" type="number" min="1" bind:value={form.backgroundUpdateCheckIntervalHours} />
          </div>
        </div>

        <Separator />

        <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <Label class="flex items-center gap-3 rounded-xl bg-background border border-border p-3 cursor-pointer">
            <Switch bind:checked={form.desktopNotifications} />
            <span>{$settingsDesktopNotifications}</span>
          </Label>
          <Label class="flex items-center gap-3 rounded-xl bg-background border border-border p-3 cursor-pointer">
            <Switch bind:checked={form.showLogs} />
            <span>{$settingsShowLogs}</span>
          </Label>
          <Label class="flex items-center gap-3 rounded-xl bg-background border border-border p-3 cursor-pointer">
            <Switch bind:checked={form.autoCheckCore} />
            <span>{$settingsUpdateCheckCore}</span>
          </Label>
          <Label class="flex items-center gap-3 rounded-xl bg-background border border-border p-3 cursor-pointer">
            <Switch bind:checked={form.autoCheckSelf} />
            <span>{$settingsUpdateCheckSelf}</span>
          </Label>
          <Label class="flex items-center gap-3 rounded-xl bg-background border border-border p-3 cursor-pointer">
            <Switch bind:checked={form.autoUpdateConfigs} />
            <span>{$settingsConfigUpdateAuto}</span>
          </Label>
          <Label class="flex items-center gap-3 rounded-xl bg-background border border-border p-3 cursor-pointer">
            <Switch bind:checked={form.autoUpdateOnHashMismatch} />
            <span>{$settingsConfigUpdateHashMismatch}</span>
          </Label>
          <Label class="flex items-center gap-3 rounded-xl bg-background border border-border p-3 cursor-pointer">
            <Switch bind:checked={form.autoRestartOnConfigUpdate} />
            <span>{$settingsConfigUpdateAutoRestart}</span>
          </Label>
        </div>
      {/if}
    </Card.Content>
  </Card.Root>

  <AlertDialog.Root bind:open={confirmRestartAdmin}>
    <AlertDialog.Content>
      <AlertDialog.Header>
        <AlertDialog.Title>{$coreBtnRestartAdmin}</AlertDialog.Title>
        <AlertDialog.Description>
          {$settingsSystemRestartAdminConfirm}
        </AlertDialog.Description>
      </AlertDialog.Header>
      <AlertDialog.Footer>
        <AlertDialog.Cancel>{$commonCancel}</AlertDialog.Cancel>
        <AlertDialog.Action onclick={restartAsAdmin}>
          {$dialogBtnConfirm}
        </AlertDialog.Action>
      </AlertDialog.Footer>
    </AlertDialog.Content>
  </AlertDialog.Root>

  <AlertDialog.Root bind:open={confirmSetcap}>
    <AlertDialog.Content>
      <AlertDialog.Header>
        <AlertDialog.Title>{$coreBtnApplySetcap}</AlertDialog.Title>
        <AlertDialog.Description>
          {$coreModeSetcapPrompt}
        </AlertDialog.Description>
      </AlertDialog.Header>
      <AlertDialog.Footer>
        <AlertDialog.Cancel>{$commonCancel}</AlertDialog.Cancel>
        <AlertDialog.Action onclick={applySetcap}>
          {$dialogBtnConfirm}
        </AlertDialog.Action>
      </AlertDialog.Footer>
    </AlertDialog.Content>
  </AlertDialog.Root>

  <AlertDialog.Root bind:open={confirmReset}>
    <AlertDialog.Content>
      <AlertDialog.Header>
        <AlertDialog.Title>{$settingsResetConfirmTitle}</AlertDialog.Title>
        <AlertDialog.Description class="whitespace-pre-wrap">
          {$settingsResetConfirmMsg}
        </AlertDialog.Description>
      </AlertDialog.Header>
      <AlertDialog.Footer>
        <AlertDialog.Cancel>{$commonCancel}</AlertDialog.Cancel>
        <AlertDialog.Action
          class="bg-destructive text-white hover:bg-destructive/90"
          onclick={resetData}
        >
          {$settingsResetBtn}
        </AlertDialog.Action>
      </AlertDialog.Footer>
    </AlertDialog.Content>
  </AlertDialog.Root>
</Page>
