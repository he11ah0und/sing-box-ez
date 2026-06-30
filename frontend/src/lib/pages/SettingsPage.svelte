<script module>
  import { Settings } from '@lucide/svelte';
  export const pageMeta = {
    id: 'settings',
    key: 'tab.settings',
    icon: Settings,
    nav: true,
    bottomNav: true,
    order: 2
  };
</script>

<script>
  import { Save, RotateCcw } from '@lucide/svelte';
  import { appState } from '../stores/appState.js';
  import { locale, tValue } from '../stores/locale.js';
  import { theme, applyTheme } from '../stores/theme.js';
  import Page from '../components/Page.svelte';
  import { GetSettings, SaveSettings, GetTheme, GetThemeNames, GetAvailableLanguages, SetLanguage } from '../../../bindings/sing-box-ez/internal/gui/wails/bindings.js';

  let processing = $state(false);
  let message = $state('');
  let saved = $state(false);

  let form = $state({
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

  let themeNames = $state([]);
  let languages = $state([]);

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
        locale.set(l);
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
    <button
      class="flex items-center gap-2 px-3 py-2 rounded-xl border border-[var(--color-border)] bg-[var(--color-surface-variant)] hover:bg-[var(--color-border)] transition"
      onclick={reset}
    >
      <RotateCcw size={16} />
      {tValue($locale, 'common.reset', 'Reset')}
    </button>
    <button
      class="flex items-center gap-2 px-3 py-2 rounded-xl bg-[var(--color-primary)] text-white disabled:opacity-50 hover:opacity-90 transition"
      disabled={processing}
      onclick={save}
    >
      <Save size={16} />
      {tValue($locale, 'common.save', 'Save')}
    </button>
  {/snippet}

  <section class="rounded-2xl border border-[var(--color-border)] bg-[var(--color-surface)] p-5 shadow-sm space-y-5">
    <div class="grid grid-cols-1 sm:grid-cols-2 gap-5">
      <label class="block space-y-1">
        <span class="text-sm text-[var(--color-text-muted)]">{tValue($locale, 'settings.language.title', 'Language')}</span>
        <select bind:value={form.language} class="w-full rounded-xl border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2">
          {#each languages as lang}
            <option value={lang.code}>{lang.name}</option>
          {/each}
        </select>
      </label>

      <label class="block space-y-1">
        <span class="text-sm text-[var(--color-text-muted)]">{tValue($locale, 'settings.theme.title', 'Theme')}</span>
        <select bind:value={form.theme} class="w-full rounded-xl border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2">
          {#each themeNames as name}
            <option value={name}>{name}</option>
          {/each}
        </select>
      </label>

      <label class="block space-y-1">
        <span class="text-sm text-[var(--color-text-muted)]">{tValue($locale, 'settings.theme_mode.title', 'Theme mode')}</span>
        <select bind:value={form.themeMode} class="w-full rounded-xl border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2">
          <option value="system">{tValue($locale, 'settings.theme_mode.system', 'System')}</option>
          <option value="dark">{tValue($locale, 'settings.theme_mode.dark', 'Dark')}</option>
          <option value="light">{tValue($locale, 'settings.theme_mode.light', 'Light')}</option>
        </select>
      </label>

      <label class="block space-y-1">
        <span class="text-sm text-[var(--color-text-muted)]">{tValue($locale, 'settings.log_limit.label', 'Log limit')}</span>
        <input type="number" bind:value={form.logLimit} min="10" class="w-full rounded-xl border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2" />
      </label>

      <label class="block space-y-1">
        <span class="text-sm text-[var(--color-text-muted)]">{tValue($locale, 'settings.default_interval.label', 'Default update interval (h)')}</span>
        <input type="number" bind:value={form.defaultIntervalHours} min="1" class="w-full rounded-xl border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2" />
      </label>
    </div>

    <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
      <label class="flex items-center gap-3 rounded-xl bg-[var(--color-bg)] p-3 cursor-pointer">
        <input type="checkbox" bind:checked={form.autoStartCore} class="w-5 h-5 accent-[var(--color-primary)]" />
        <span>{tValue($locale, 'core.start_on_launch', 'Start core on app launch')}</span>
      </label>
      <label class="flex items-center gap-3 rounded-xl bg-[var(--color-bg)] p-3 cursor-pointer">
        <input type="checkbox" bind:checked={form.autoRestart} class="w-5 h-5 accent-[var(--color-primary)]" />
        <span>{tValue($locale, 'core.auto_restart', 'Auto restart core')}</span>
      </label>
      <label class="flex items-center gap-3 rounded-xl bg-[var(--color-bg)] p-3 cursor-pointer">
        <input type="checkbox" bind:checked={form.runAsAdmin} class="w-5 h-5 accent-[var(--color-primary)]" />
        <span>{tValue($locale, 'settings.runAsAdmin', 'Run core as admin')}</span>
      </label>
      <label class="flex items-center gap-3 rounded-xl bg-[var(--color-bg)] p-3 cursor-pointer">
        <input type="checkbox" bind:checked={form.desktopNotifications} class="w-5 h-5 accent-[var(--color-primary)]" />
        <span>{tValue($locale, 'settings.desktop_notifications', 'Desktop notifications')}</span>
      </label>
      <label class="flex items-center gap-3 rounded-xl bg-[var(--color-bg)] p-3 cursor-pointer">
        <input type="checkbox" bind:checked={form.autoCheckCore} class="w-5 h-5 accent-[var(--color-primary)]" />
        <span>{tValue($locale, 'settings.update_check.core', 'Auto-check core updates')}</span>
      </label>
      <label class="flex items-center gap-3 rounded-xl bg-[var(--color-bg)] p-3 cursor-pointer">
        <input type="checkbox" bind:checked={form.autoCheckSelf} class="w-5 h-5 accent-[var(--color-primary)]" />
        <span>{tValue($locale, 'settings.update_check.self', 'Auto-check app updates')}</span>
      </label>
    </div>

    {#if saved}
      <p class="text-sm text-green-500">{tValue($locale, 'settings.saved', 'Settings saved')}</p>
    {/if}
    {#if message}
      <p class="text-sm text-[var(--color-danger)]">{message}</p>
    {/if}
  </section>
</Page>
