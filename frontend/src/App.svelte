<script>
  import { onMount, tick } from 'svelte';
  import { initWailsEvents } from './lib/wails/bridge.js';
  import { theme, applyTheme } from './lib/stores/theme.js';
  import { signalLocaleReady } from './lib/stores/locale.js';
  import { GetTheme } from '../bindings/sing-box-ez/internal/gui/wails/bindings.js';
  import { currentLevel } from './lib/stores/navigation.js';
  import { appState } from './lib/stores/appState.js';
  import Shell from './lib/components/Shell.svelte';
  import Modal from './lib/components/Modal.svelte';
  import Notification from './lib/components/Notification.svelte';
  import StartupPage from './lib/pages/StartupPage.svelte';
  import { pageComponents } from './lib/pages/index.js';

  let ActivePage = $derived(pageComponents[$currentLevel.id] ?? pageComponents.main);

  onMount(() => {
    console.log('App mounted');
    try {
      initWailsEvents();
      GetTheme()
        .then((t) => {
          if (t) {
            theme.set(t);
            applyTheme(t);
          } else {
            applyTheme($theme);
          }
        })
        .catch((err) => {
          console.warn('Failed to fetch initial theme:', err);
          applyTheme($theme);
        });
      tick().then(() => setTimeout(signalLocaleReady, 0));
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
    <ActivePage />
  </Shell>
{/if}

{#if $appState.dialog}
  <Modal title={$appState.dialog.title} onclose={() => appState.update((s) => ({ ...s, dialog: null }))}>
    <p>{$appState.dialog.body}</p>
  </Modal>
{/if}

<Notification />
