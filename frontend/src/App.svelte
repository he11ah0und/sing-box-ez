<script>
  import { onMount } from 'svelte';
  import { initWailsEvents } from './lib/wails/bridge.js';
  import { theme, applyTheme } from './lib/stores/theme.js';
  import { currentLevel } from './lib/stores/navigation.js';
  import { appState } from './lib/stores/appState.js';
  import Shell from './lib/components/Shell.svelte';
  import Modal from './lib/components/Modal.svelte';

  import MainPage from './lib/pages/MainPage.svelte';
  import ConfigsPage from './lib/pages/ConfigsPage.svelte';
  import CorePage from './lib/pages/CorePage.svelte';
  import SettingsPage from './lib/pages/SettingsPage.svelte';
  import LogsPage from './lib/pages/LogsPage.svelte';
  import AboutPage from './lib/pages/AboutPage.svelte';
  import MenuPage from './lib/pages/MenuPage.svelte';
  import StartupPage from './lib/pages/StartupPage.svelte';

  const pages = {
    main: MainPage,
    configs: ConfigsPage,
    core: CorePage,
    settings: SettingsPage,
    logs: LogsPage,
    about: AboutPage,
    menu: MenuPage
  };

  let activePage = $derived(pages[$currentLevel.id] ?? MainPage);

  onMount(() => {
    console.log('App mounted');
    try {
      initWailsEvents();
      applyTheme($theme.colors);
      console.log('Wails events initialized');
    } catch (err) {
      console.error('Failed to init Wails events:', err);
    }
  });
</script>

{#if $appState.startup.show}
  <StartupPage />
{:else}
  <Shell>
    <activePage></activePage>
  </Shell>
{/if}

{#if $appState.dialog}
  <Modal title={$appState.dialog.title} onclose={() => appState.update((s) => ({ ...s, dialog: null }))}>
    <p>{$appState.dialog.body}</p>
  </Modal>
{/if}
