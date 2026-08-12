<script lang="ts">
  import { locale, registerLocaleKey } from '@he11ah0und/localengine-web';

  let { id, fallback = '' }: { id: string; fallback?: string } = $props();

  $effect(() => registerLocaleKey(id));
</script>

{$locale.values[id] ?? fallback}
