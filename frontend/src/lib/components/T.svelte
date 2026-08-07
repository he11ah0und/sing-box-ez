<script lang="ts">
  import { locale, registerLocaleKey } from '../stores/locale.js';

  let { id, fallback = '' }: { id: string; fallback?: string } = $props();

  $effect(() => registerLocaleKey(id));
</script>

{$locale.values[id] ?? fallback}
