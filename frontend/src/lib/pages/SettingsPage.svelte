<script module lang="ts">
  import { Settings as SettingsIcon } from '@lucide/svelte';
  export const pageMeta = {
    id: 'settings',
    key: 'tab.settings',
    icon: SettingsIcon,
    nav: true,
    bottomNav: true,
    order: 2,
    tabs: [
      { id: 'general', key: 'settings.tab.general' },
      { id: 'core', key: 'settings.tab.core' }
    ]
  };
</script>

<script lang="ts">
  import { Save, RotateCcw } from '@lucide/svelte';
  import { appState } from '../stores/appState.js';
  import { locale, tValue } from '../stores/locale.js';
  import { theme, applyTheme } from '../stores/theme.js';
  import { subNav } from '../stores/navigation.js';
  import Page from '../components/Page.svelte';
  import * as Card from '$lib/components/ui/card/index.js';
  import { Button } from '$lib/components/ui/button/index.js';
  import { Input } from '$lib/components/ui/input/index.js';
  import { Label } from '$lib/components/ui/label/index.js';
  import * as Select from '$lib/components/ui/select/index.js';
  import { Switch } from '$lib/components/ui/switch/index.js';
  import {
    GetSettings,
    SaveSettings,
    GetTheme,
    GetThemeNames,
    GetAvailableLanguages,
    SetLanguage,
    type Settings,
    type LanguageOption
  } from '../../../bindings/sing-box-ez/internal/gui/wails/bindings.js';

  let processing = $state(false);
  let message = $state('');
  let saved = $state(false);

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
    defaultIntervalHours: 24
  });

  let themeNames = $state<string[]>([]);
  let languages = $state<LanguageOption[]>([]);

  const themeModeLabel = $derived(tValue($locale, `settings.theme_mode.${form.themeMode}`, form.themeMode));

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
      appState.update((state) => ({ ...state, settings: s as unknown as Record<string, unknown> }));
    } catch (err) {
      message = String(err);
    }
  }

  async function save() {
    processing = true;
    message = '';
    saved = false;
    try {
      await SaveSettings({ ...form });
      appState.update((state) => ({ ...state, settings: { ...form } }));
      const [t, l] = await Promise.all([GetTheme(), SetLanguage(form.language)]);
      if (t) {
        theme.set(t);
        applyTheme(t);
      }
      if (l) {
        locale.set({ language: l.language, values: l.values ?? {} });
      }
      saved = true;
    } catch (err) {
      message = String(err);
    } finally {
      processing = false;
    }
  }

  function reset() {
    load();
  }
</script>

<Page
  title={tValue($locale, 'tab.settings', 'Settings')}
  onLoad={load}
>
  {#snippet actions()}
    <Button variant="outline" onclick={reset}>
      <RotateCcw size={16} />
      {tValue($locale, 'common.reset', 'Reset')}
    </Button>
    <Button disabled={processing} onclick={save}>
      <Save size={16} />
      {tValue($locale, 'common.save', 'Save')}
    </Button>
  {/snippet}

  <Card.Root>
    <Card.Content class="space-y-5">
      {#if $subNav.activeTab === 'core'}
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <Label class="flex items-center gap-3 rounded-xl bg-background border border-border p-3 cursor-pointer">
            <Switch bind:checked={form.autoStartCore} />
            <span>{tValue($locale, 'core.start_on_launch', 'Start core on app launch')}</span>
          </Label>
          <Label class="flex items-center gap-3 rounded-xl bg-background border border-border p-3 cursor-pointer">
            <Switch bind:checked={form.autoRestart} />
            <span>{tValue($locale, 'core.auto_restart', 'Auto restart core')}</span>
          </Label>
          <Label class="flex items-center gap-3 rounded-xl bg-background border border-border p-3 cursor-pointer">
            <Switch bind:checked={form.runAsAdmin} />
            <span>{tValue($locale, 'settings.runAsAdmin', 'Run core as admin')}</span>
          </Label>
        </div>
      {:else}
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-5">
          <div class="space-y-1">
            <Label>{tValue($locale, 'settings.language.title', 'Language')}</Label>
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
            <Label>{tValue($locale, 'settings.theme.title', 'Theme')}</Label>
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
            <Label>{tValue($locale, 'settings.theme_mode.title', 'Theme mode')}</Label>
            <Select.Root type="single" bind:value={form.themeMode}>
              <Select.Trigger class="w-full">{themeModeLabel}</Select.Trigger>
              <Select.Content>
                <Select.Item value="system" label={tValue($locale, 'settings.theme_mode.system', 'System')} />
                <Select.Item value="dark" label={tValue($locale, 'settings.theme_mode.dark', 'Dark')} />
                <Select.Item value="light" label={tValue($locale, 'settings.theme_mode.light', 'Light')} />
              </Select.Content>
            </Select.Root>
          </div>

          <div class="space-y-1">
            <Label for="settings-log-limit">{tValue($locale, 'settings.log_limit.label', 'Log limit')}</Label>
            <Input id="settings-log-limit" type="number" min="10" bind:value={form.logLimit} />
          </div>

          <div class="space-y-1">
            <Label for="settings-interval">{tValue($locale, 'settings.default_interval.label', 'Default update interval (h)')}</Label>
            <Input id="settings-interval" type="number" min="1" bind:value={form.defaultIntervalHours} />
          </div>
        </div>

        <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <Label class="flex items-center gap-3 rounded-xl bg-background border border-border p-3 cursor-pointer">
            <Switch bind:checked={form.desktopNotifications} />
            <span>{tValue($locale, 'settings.desktop_notifications', 'Desktop notifications')}</span>
          </Label>
          <Label class="flex items-center gap-3 rounded-xl bg-background border border-border p-3 cursor-pointer">
            <Switch bind:checked={form.autoCheckCore} />
            <span>{tValue($locale, 'settings.update_check.core', 'Auto-check core updates')}</span>
          </Label>
          <Label class="flex items-center gap-3 rounded-xl bg-background border border-border p-3 cursor-pointer">
            <Switch bind:checked={form.autoCheckSelf} />
            <span>{tValue($locale, 'settings.update_check.self', 'Auto-check app updates')}</span>
          </Label>
        </div>
      {/if}

      {#if saved}
        <p class="text-sm text-[var(--color-success)]">{tValue($locale, 'settings.saved', 'Settings saved')}</p>
      {/if}
      {#if message}
        <p class="text-sm text-destructive">{message}</p>
      {/if}
    </Card.Content>
  </Card.Root>
</Page>
