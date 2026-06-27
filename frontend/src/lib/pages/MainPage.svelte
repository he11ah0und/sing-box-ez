<script>
  import { appState } from '../stores/appState.js';
  import { locale, tValue } from '../stores/locale.js';
</script>

<div class="p-4">
  <h2 class="text-2xl font-bold mb-4">{tValue($locale, 'tab.main', 'Main')}</h2>
  <p class="text-[var(--color-text-muted)]">Running: {$appState.status.running}</p>
</div>
