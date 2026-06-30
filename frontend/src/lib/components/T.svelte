<script>
  import { locale, registerLocaleKey } from '../stores/locale.js';

  let { id, fallback = '' } = $props();

  registerLocaleKey(id);
</script>

{$locale.values[id] ?? fallback}
