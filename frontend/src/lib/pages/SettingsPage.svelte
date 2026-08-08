<script lang="ts">
  import { Save, RotateCcw, ShieldCheck, Trash2 } from '@lucide/svelte';
  import { toast } from 'svelte-sonner';
  import { appState } from '../stores/appState.js';
  import { locale, tValue } from '../stores/locale.js';
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
      toast.success(tValue($locale, 'settings.saved'));
    } catch (err) {
      toast.error(String(err));
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
    } catch (err) {
      toast.error(String(err));
    } finally {
      privProcessing = false;
    }
  }

  async function applySetcap() {
    privProcessing = true;
    try {
      await ApplySetcap();
      await loadPrivileges();
    } catch (err) {
      toast.error(String(err));
    } finally {
      privProcessing = false;
    }
  }

  async function resetData() {
    try {
      // ResetData deletes the data files and quits the app; the call may never resolve.
      await ResetData();
    } catch (err) {
      toast.error(String(err));
    }
  }
</script>

<Page
  title={tValue($locale, 'tab.settings')}
  onLoad={load}
>
  {#snippet actions()}
    <Button variant="outline" onclick={reset}>
      <RotateCcw size={16} />
      {tValue($locale, 'common.reset')}
    </Button>
    <Button disabled={processing} onclick={save}>
      <Save size={16} />
      {tValue($locale, 'common.save')}
    </Button>
  {/snippet}

  <Card.Root>
    <Card.Content class="space-y-5">
      {#if $subNav.activeTab === 'core'}
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <Label class="flex items-center gap-3 rounded-xl bg-background border border-border p-3 cursor-pointer">
            <Switch bind:checked={form.autoStartCore} />
            <span>{tValue($locale, 'core.start_on_launch')}</span>
          </Label>
          <Label class="flex items-center gap-3 rounded-xl bg-background border border-border p-3 cursor-pointer">
            <Switch bind:checked={form.autoRestart} />
            <span>{tValue($locale, 'core.auto_restart')}</span>
          </Label>
          <Label class="flex items-center gap-3 rounded-xl bg-background border border-border p-3 cursor-pointer">
            <Switch bind:checked={form.runAsAdmin} />
            <span>{tValue($locale, 'settings.runAsAdmin')}</span>
          </Label>
          <Label class="flex items-center gap-3 rounded-xl bg-background border border-border p-3 cursor-pointer">
            <Switch bind:checked={form.proxyEnabled} />
            <span>{tValue($locale, 'core.proxy.enabled')}</span>
          </Label>
        </div>

        <div class="grid grid-cols-1 sm:grid-cols-2 gap-5">
          <div class="space-y-1 sm:col-span-2">
            <Label for="settings-url-test">{tValue($locale, 'core.url_test_url.label')}</Label>
            <Input id="settings-url-test" bind:value={form.urlTestURL} />
          </div>

          <div class="space-y-1">
            <Label>{tValue($locale, 'core.log.level')}</Label>
            <Select.Root type="single" bind:value={form.coreLogLevel}>
              <Select.Trigger class="w-full">{coreLogLevelLabel}</Select.Trigger>
              <Select.Content>
                <Select.Item value="debug" label={tValue($locale, 'settings.log_level.debug')} />
                <Select.Item value="info" label={tValue($locale, 'settings.log_level.info')} />
                <Select.Item value="warn" label={tValue($locale, 'settings.log_level.warn')} />
                <Select.Item value="error" label={tValue($locale, 'settings.log_level.error')} />
              </Select.Content>
            </Select.Root>
          </div>

          <div class="space-y-1">
            <Label>{tValue($locale, 'core.graph_history.label')}</Label>
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
            {tValue($locale, 'core.privileges.title')}
          </p>
          {#if privState}
            <p class="text-sm text-muted-foreground">
              {tValue($locale, 'settings.system.mode')}: {privState.mode}
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
                    {tValue($locale, 'core.btn.restart_admin')}
                  </Button>
                {/if}
                {#if privState.showSetcapBtn}
                  <Button variant="outline" disabled={privProcessing} onclick={() => (confirmSetcap = true)}>
                    {tValue($locale, 'core.btn.apply_setcap')}
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
          <p class="font-medium text-destructive">{tValue($locale, 'settings.reset.title')}</p>
          <Button variant="destructive" onclick={() => (confirmReset = true)}>
            <Trash2 size={16} />
            {tValue($locale, 'settings.reset.btn')}
          </Button>
        </div>
      {:else}
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-5">
          <div class="space-y-1">
            <Label>{tValue($locale, 'settings.language.title')}</Label>
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
            <Label>{tValue($locale, 'settings.theme.title')}</Label>
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
            <Label>{tValue($locale, 'settings.theme_mode.title')}</Label>
            <Select.Root type="single" bind:value={form.themeMode}>
              <Select.Trigger class="w-full">{themeModeLabel}</Select.Trigger>
              <Select.Content>
                <Select.Item value="system" label={tValue($locale, 'settings.theme_mode.system')} />
                <Select.Item value="dark" label={tValue($locale, 'settings.theme_mode.dark')} />
                <Select.Item value="light" label={tValue($locale, 'settings.theme_mode.light')} />
              </Select.Content>
            </Select.Root>
          </div>

          <div class="space-y-1">
            <Label for="settings-log-limit">{tValue($locale, 'settings.log_limit.label')}</Label>
            <Input id="settings-log-limit" type="number" min="10" bind:value={form.logLimit} />
          </div>

          <div class="space-y-1">
            <Label for="settings-interval">{tValue($locale, 'settings.default_interval.label')}</Label>
            <Input id="settings-interval" type="number" min="1" bind:value={form.defaultIntervalHours} />
          </div>

          <div class="space-y-1">
            <Label for="settings-config-interval">{tValue($locale, 'settings.config_update.interval')}</Label>
            <Input id="settings-config-interval" type="number" min="1" bind:value={form.autoUpdateConfigsIntervalHours} />
          </div>

          <div class="space-y-1">
            <Label for="settings-bg-interval">{tValue($locale, 'settings.config_update.background_interval')}</Label>
            <Input id="settings-bg-interval" type="number" min="1" bind:value={form.backgroundUpdateCheckIntervalHours} />
          </div>
        </div>

        <Separator />

        <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <Label class="flex items-center gap-3 rounded-xl bg-background border border-border p-3 cursor-pointer">
            <Switch bind:checked={form.desktopNotifications} />
            <span>{tValue($locale, 'settings.desktop_notifications')}</span>
          </Label>
          <Label class="flex items-center gap-3 rounded-xl bg-background border border-border p-3 cursor-pointer">
            <Switch bind:checked={form.showLogs} />
            <span>{tValue($locale, 'settings.show_logs')}</span>
          </Label>
          <Label class="flex items-center gap-3 rounded-xl bg-background border border-border p-3 cursor-pointer">
            <Switch bind:checked={form.autoCheckCore} />
            <span>{tValue($locale, 'settings.update_check.core')}</span>
          </Label>
          <Label class="flex items-center gap-3 rounded-xl bg-background border border-border p-3 cursor-pointer">
            <Switch bind:checked={form.autoCheckSelf} />
            <span>{tValue($locale, 'settings.update_check.self')}</span>
          </Label>
          <Label class="flex items-center gap-3 rounded-xl bg-background border border-border p-3 cursor-pointer">
            <Switch bind:checked={form.autoUpdateConfigs} />
            <span>{tValue($locale, 'settings.config_update.auto')}</span>
          </Label>
          <Label class="flex items-center gap-3 rounded-xl bg-background border border-border p-3 cursor-pointer">
            <Switch bind:checked={form.autoUpdateOnHashMismatch} />
            <span>{tValue($locale, 'settings.config_update.hash_mismatch')}</span>
          </Label>
          <Label class="flex items-center gap-3 rounded-xl bg-background border border-border p-3 cursor-pointer">
            <Switch bind:checked={form.autoRestartOnConfigUpdate} />
            <span>{tValue($locale, 'settings.config_update.auto_restart')}</span>
          </Label>
        </div>
      {/if}
    </Card.Content>
  </Card.Root>

  <AlertDialog.Root bind:open={confirmRestartAdmin}>
    <AlertDialog.Content>
      <AlertDialog.Header>
        <AlertDialog.Title>{tValue($locale, 'core.btn.restart_admin')}</AlertDialog.Title>
        <AlertDialog.Description>
          {tValue($locale, 'settings.system.restart_admin_confirm')}
        </AlertDialog.Description>
      </AlertDialog.Header>
      <AlertDialog.Footer>
        <AlertDialog.Cancel>{tValue($locale, 'common.cancel')}</AlertDialog.Cancel>
        <AlertDialog.Action onclick={restartAsAdmin}>
          {tValue($locale, 'dialog.btn.confirm')}
        </AlertDialog.Action>
      </AlertDialog.Footer>
    </AlertDialog.Content>
  </AlertDialog.Root>

  <AlertDialog.Root bind:open={confirmSetcap}>
    <AlertDialog.Content>
      <AlertDialog.Header>
        <AlertDialog.Title>{tValue($locale, 'core.btn.apply_setcap')}</AlertDialog.Title>
        <AlertDialog.Description>
          {tValue($locale, 'core.mode.setcap_prompt')}
        </AlertDialog.Description>
      </AlertDialog.Header>
      <AlertDialog.Footer>
        <AlertDialog.Cancel>{tValue($locale, 'common.cancel')}</AlertDialog.Cancel>
        <AlertDialog.Action onclick={applySetcap}>
          {tValue($locale, 'dialog.btn.confirm')}
        </AlertDialog.Action>
      </AlertDialog.Footer>
    </AlertDialog.Content>
  </AlertDialog.Root>

  <AlertDialog.Root bind:open={confirmReset}>
    <AlertDialog.Content>
      <AlertDialog.Header>
        <AlertDialog.Title>{tValue($locale, 'settings.reset.confirm_title')}</AlertDialog.Title>
        <AlertDialog.Description class="whitespace-pre-wrap">
          {tValue($locale, 'settings.reset.confirm_msg')}
        </AlertDialog.Description>
      </AlertDialog.Header>
      <AlertDialog.Footer>
        <AlertDialog.Cancel>{tValue($locale, 'common.cancel')}</AlertDialog.Cancel>
        <AlertDialog.Action
          class="bg-destructive text-white hover:bg-destructive/90"
          onclick={resetData}
        >
          {tValue($locale, 'settings.reset.btn')}
        </AlertDialog.Action>
      </AlertDialog.Footer>
    </AlertDialog.Content>
  </AlertDialog.Root>
</Page>
