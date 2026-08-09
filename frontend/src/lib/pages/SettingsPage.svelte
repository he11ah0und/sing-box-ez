<script lang="ts">
  import { Save, RotateCcw, ShieldCheck, Trash2 } from '@lucide/svelte';
  import { toast } from 'svelte-sonner';
  import { appState } from '../stores/appState.js';
  import { locale, useLocale, useLocaleRecord } from '../stores/locale.svelte.js';
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
    GetConfigSpec,
    GetConfigValues,
    SetConfigValues,
    GetTheme,
    GetThemeNames,
    SetTheme,
    GetAvailableLanguages,
    SetLanguage,
    GetPrivilegeTabState,
    RestartAsAdmin,
    ApplySetcap,
    ResetData
  } from '../../../bindings/sing-box-ez/internal/gui/wails/bindings.js';
  import type {
    ConfigSpecEntry,
    LanguageOption,
    PrivilegeTabState
  } from '../../../bindings/sing-box-ez/internal/gui/wails/models.js';

  let processing = $state(false);

  // The form seeds from the cached values snapshot (fetched once by the
  // bridge) so revisits render instantly; GetConfigValues runs again only
  // when the snapshot is missing or the user hits Reset.
  let form = $state<Record<string, any>>({ ...$appState.settings });
  let spec = $state<ConfigSpecEntry[]>([]);

  let languages = $state<LanguageOption[]>([]);
  let themeName = $state('');
  let themeNames = $state<string[]>([]);
  let privState = $state<PrivilegeTabState | null>(null);
  let privProcessing = $state(false);
  let confirmRestartAdmin = $state(false);
  let confirmSetcap = $state(false);
  let confirmReset = $state(false);

  // Dynamic labels come from wildcard namespaces: tab labels, control labels
  // and select option labels all follow the settings.<path>[.label|.<option>]
  // convention; common.* backs the debug/info option fallback.
  const R = useLocaleRecord(['settings.*', 'common.*']);

  const L = useLocale([
    'tab.settings',
    'common.reset',
    'common.save',
    'common.cancel',
    'dialog.btn.confirm',
    'core.privileges.title',
    'core.btn.restart_admin',
    'core.btn.apply_setcap',
    'core.mode.setcap_prompt',
    'settings.system.mode',
    'settings.system.restart_admin_confirm',
    'settings.reset.title',
    'settings.reset.btn',
    'settings.reset.confirm_title',
    'settings.reset.confirm_msg',
    'settings.language.title',
    'settings.theme.title'
  ]);

  // The system tab is custom; privileges.* entries fold into it (there is no
  // settings.tab.privileges key), rendered above the privileges block.
  function tabOf(path: string): string {
    const seg = path.split('.')[0];
    return seg === 'privileges' ? 'system' : seg;
  }

  function entriesFor(tab: string): ConfigSpecEntry[] {
    return spec.filter((e) => tabOf(e.path) === tab);
  }

  function entryLabel(entry: ConfigSpecEntry): string {
    // Selects with translated option labels are maps and label via
    // settings.<path>.label; plain keys label via settings.<path> directly.
    if (entry.control === 'select') {
      const withLabel = R.settings[`${entry.path}.label`];
      if (withLabel && withLabel !== `settings.${entry.path}.label`) return withLabel;
    }
    return R.settings[entry.path];
  }

  function optionLabel(entry: ConfigSpecEntry, option: unknown): string {
    const opt = String(option);
    const key = `${entry.path}.${opt}`;
    const label = R.settings[key];
    // A missing convention key falls back to common.<option> (debug/info),
    // then to the raw value itself (numeric options like 30/60/120/300).
    // Tolerate both missing-leaf forms: the proxy's full-key fallback and
    // a plain undefined.
    if (label && label !== `settings.${key}`) return label;
    const common = R.common[opt];
    return common && common !== `common.${opt}` ? common : opt;
  }

  function inputId(path: string): string {
    return `settings-${path.replaceAll('.', '-')}`;
  }

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
      const needValues = force || !$appState.settingsLoaded;
      const [s, v, langs, t, names] = await Promise.all([
        GetConfigSpec(),
        needValues ? GetConfigValues() : Promise.resolve(null),
        GetAvailableLanguages(),
        GetTheme(),
        GetThemeNames()
      ]);
      spec = s ?? [];
      if (v) {
        form = { ...v };
        appState.update((state) => ({ ...state, settings: { ...v }, settingsLoaded: true }));
      } else {
        form = { ...$appState.settings };
      }
      languages = langs && langs.length > 0 ? langs : [{ code: 'en', name: 'English' }];
      themeName = t?.name ?? '';
      themeNames = names && names.length > 0 ? names : [themeName || 'default'];
    } catch (err) {
      toast.error(String(err));
    }
  }

  async function save() {
    processing = true;
    try {
      await SetConfigValues({ ...form });
      appState.update((state) => ({ ...state, settings: { ...form } }));
    } catch {
      // The backend already reported the result with a toast.
    } finally {
      processing = false;
    }
  }

  function reset() {
    load(true);
  }

  // SetLanguage persists ui.language itself and returns the new locale; the
  // backend also emits locale:changed, so this just applies the result.
  async function changeLanguage(code: string) {
    try {
      const l = await SetLanguage(code);
      if (l) {
        locale.set({ language: l.language, values: (l.values ?? {}) as Record<string, string> });
      }
    } catch (err) {
      toast.error(String(err));
    }
  }

  // SetTheme persists ui.theme and emits theme:changed, which the bridge
  // applies to the document; this just keeps the select in sync.
  async function changeTheme(name: string) {
    try {
      const t = await SetTheme(name);
      if (t) themeName = t.name;
    } catch (err) {
      toast.error(String(err));
    }
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

{#snippet generatedControls(tab: string)}
  {@const switches = entriesFor(tab).filter((e) => e.control === 'switch')}
  {@const fields = entriesFor(tab).filter((e) => e.control !== 'switch')}
  {#if switches.length > 0}
    <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
      {#each switches as entry (entry.path)}
        <Label class="flex items-center gap-3 rounded-xl bg-background border border-border p-3 cursor-pointer">
          <Switch bind:checked={form[entry.path]} />
          <span>{entryLabel(entry)}</span>
        </Label>
      {/each}
    </div>
  {/if}
  {#if fields.length > 0}
    <div class="grid grid-cols-1 sm:grid-cols-2 gap-5">
      {#each fields as entry (entry.path)}
        <div class="space-y-1">
          <Label for={inputId(entry.path)}>{entryLabel(entry)}</Label>
          {#if entry.control === 'select'}
            <Select.Root
              type="single"
              value={String(form[entry.path])}
              onValueChange={(v) => (form[entry.path] = entry.type === 'int' ? Number(v) : v)}
            >
              <Select.Trigger class="w-full">{optionLabel(entry, form[entry.path])}</Select.Trigger>
              <Select.Content>
                {#each entry.options ?? [] as opt (String(opt))}
                  <Select.Item value={String(opt)} label={optionLabel(entry, opt)} />
                {/each}
              </Select.Content>
            </Select.Root>
          {:else if entry.control === 'number'}
            <Input
              id={inputId(entry.path)}
              type="number"
              min={entry.min ?? undefined}
              max={entry.max ?? undefined}
              bind:value={form[entry.path]}
            />
          {:else}
            <Input id={inputId(entry.path)} bind:value={form[entry.path]} />
          {/if}
        </div>
      {/each}
    </div>
  {/if}
{/snippet}

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
      {#if $subNav.activeTab === 'system'}
        {@render generatedControls('system')}

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
        {#if $subNav.activeTab === 'ui'}
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-5">
            <div class="space-y-1">
              <Label>{L.settingsLanguageTitle}</Label>
              <Select.Root type="single" value={$locale.language} onValueChange={changeLanguage}>
                <Select.Trigger class="w-full">
                  {languages.find((l) => l.code === $locale.language)?.name ?? $locale.language}
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
              <Select.Root type="single" value={themeName} onValueChange={changeTheme}>
                <Select.Trigger class="w-full">{themeName}</Select.Trigger>
                <Select.Content>
                  {#each themeNames as name (name)}
                    <Select.Item value={name} label={name} />
                  {/each}
                </Select.Content>
              </Select.Root>
            </div>
          </div>
        {/if}

        {@render generatedControls($subNav.activeTab ?? '')}
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
