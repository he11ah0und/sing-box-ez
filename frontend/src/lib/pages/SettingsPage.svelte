<script lang="ts">
  import { Save, RotateCcw, ShieldCheck, Trash2, LoaderCircle } from '@lucide/svelte';
  import { toast } from 'svelte-sonner';
  import { fly } from 'svelte/transition';
  import { Events } from '@wailsio/runtime';
  import { appState } from '../stores/appState.js';
  import { locale, useLocale, useLocaleRecord } from '@he11ah0und/localengine-web';
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
    RunConfigAction,
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
  let confirmAction = $state<ConfigSpecEntry | null>(null);
  let confirmActionOpen = $state(false);
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
    'core.privileges.setcap_hint',
    'settings.system.mode',
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

  // Action labels come from settings.<path>; privileges.setcap is the one
  // special case: it is a toggle, so the label and confirm text follow the
  // current capability state.
  function actionLabel(entry: ConfigSpecEntry): string {
    if (entry.path === 'privileges.setcap') {
      return privState?.hasSetcap
        ? R.settings['privileges.setcap_remove']
        : R.settings['privileges.setcap'];
    }
    return R.settings[entry.path];
  }

  function actionConfirmText(entry: ConfigSpecEntry): string {
    if (entry.path === 'privileges.setcap') {
      return privState?.hasSetcap
        ? R.settings['privileges.setcap_remove_confirm']
        : R.settings['privileges.setcap_confirm'];
    }
    return R.settings[`${entry.path}_confirm`];
  }

  function clickAction(entry: ConfigSpecEntry) {
    if (entry.confirm) {
      confirmAction = entry;
      confirmActionOpen = true;
    } else {
      runAction(entry);
    }
  }

  async function runAction(entry: ConfigSpecEntry) {
    privProcessing = true;
    try {
      await RunConfigAction(entry.path);
      await loadPrivileges();
      // Success: the backend shows a result toast; close the confirm dialog.
      confirmActionOpen = false;
    } catch {
      // Keep the dialog open on failure; the backend already toasted the error.
    } finally {
      privProcessing = false;
    }
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

  // A core update replaces the binary and drops its setcap state; the
  // backend signals it so an open settings page does not show stale data.
  $effect(() => {
    const off = Events.On('privileges:changed', () => loadPrivileges());
    return () => off();
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

  async function resetData() {
    // ResetData deletes the data files and quits the app; the call may never resolve.
    await ResetData().catch(() => {});
  }
</script>

{#snippet generatedControls(tab: string)}
  {@const switches = entriesFor(tab).filter((e) => e.control === 'switch')}
  {@const fields = entriesFor(tab).filter((e) => e.control !== 'switch' && e.control !== 'action')}
  {@const actions = entriesFor(tab).filter((e) => e.control === 'action')}
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
  {#if actions.length > 0}
    <div class="flex flex-wrap gap-2">
      {#each actions as entry (entry.path)}
        <Button variant="outline" disabled={privProcessing} onclick={() => clickAction(entry)}>
          {actionLabel(entry)}
        </Button>
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

  <!-- Tab switches animate like the root pages do. -->
  {#key $subNav.activeTab}
    <div in:fly={{ y: 8, duration: 150 }}>
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
            {#if privState.mode === 'linux'}
              <p class="text-xs text-muted-foreground">{L.corePrivilegesSetcap_hint}</p>
            {/if}
            {#if privState.adminLabel}
              <p class="text-sm text-muted-foreground">{privState.adminLabel}</p>
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
    </div>
  {/key}

  <AlertDialog.Root
    open={confirmActionOpen}
    onOpenChange={(open) => {
      // While the action runs the dialog stays open: close requests are
      // ignored so the spinner remains visible until success or failure.
      if (privProcessing && !open) return;
      confirmActionOpen = open;
    }}
  >
    <AlertDialog.Content>
      <AlertDialog.Header>
        <AlertDialog.Title>{confirmAction ? actionLabel(confirmAction) : ''}</AlertDialog.Title>
        <AlertDialog.Description>
          {confirmAction ? actionConfirmText(confirmAction) : ''}
        </AlertDialog.Description>
      </AlertDialog.Header>
      <AlertDialog.Footer>
        <AlertDialog.Cancel disabled={privProcessing}>{L.commonCancel}</AlertDialog.Cancel>
        <AlertDialog.Action
          disabled={privProcessing}
          onclick={(e) => {
            // Prevent the default close: the dialog closes on success in
            // runAction and stays open on failure.
            e.preventDefault();
            if (confirmAction) runAction(confirmAction);
          }}
        >
          {#if privProcessing}
            <LoaderCircle size={16} class="animate-spin" />
          {:else}
            {L.dialogBtnConfirm}
          {/if}
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
