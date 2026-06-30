<script>
  let { title = '' } = $props();
</script>

<header class="h-14 flex items-center px-4 border-b border-[var(--color-border)] bg-[var(--color-surface)] shrink-0">
  <h1 class="text-lg font-semibold truncate">{title}</h1>
</header>
