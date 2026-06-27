<script>
  import { locale, tValue } from '../stores/locale.js';
</script>

<div class="p-4">
  <h2 class="text-2xl font-bold mb-4">{tValue($locale, 'tab.logs', 'Logs')}</h2>
  <p class="text-[var(--color-text-muted)]">Logs viewer will be here.</p>
</div>
