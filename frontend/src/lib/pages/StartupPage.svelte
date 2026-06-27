<script>
  import { appState } from '../stores/appState.js';
  import { locale, tValue } from '../stores/locale.js';

  function selectMode(mode) {
    appState.update((s) => ({ ...s, startup: { ...s.startup, selected: mode } }));
  }

  function continueStartup() {
    appState.update((s) => ({ ...s, startup: { ...s.startup, show: false } }));
  }
</script>

<div class="h-full flex flex-col items-center justify-center p-6 bg-[var(--color-bg)]">
  <div class="w-full max-w-md bg-[var(--color-surface)] rounded-xl border border-[var(--color-border)] p-6 shadow-xl">
    <h1 class="text-2xl font-bold mb-2">{tValue($locale, 'startup.title', 'Startup')}</h1>
    <p class="text-[var(--color-text-muted)] mb-6">{tValue($locale, 'startup.subtitle', 'Choose connection mode')}</p>

    <div class="flex flex-col gap-3 mb-6">
      {#each $appState.startup.options as option}
        <button
          class="px-4 py-3 rounded-lg border border-[var(--color-border)] text-left transition-colors"
          class:bg-[var(--color-primary)]={$appState.startup.selected === option.id}
          class:text-white={$appState.startup.selected === option.id}
          onclick={() => selectMode(option.id)}
        >
          {option.label}
        </button>
      {/each}
    </div>

    <button
      class="w-full py-3 rounded-lg bg-[var(--color-primary)] hover:bg-[var(--color-primary-hover)] text-white font-semibold transition-colors"
      onclick={continueStartup}
    >
      {tValue($locale, 'startup.continue', 'Continue')}
    </button>
  </div>
</div>
