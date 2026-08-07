<script lang="ts">
  import { onMount, type Snippet } from 'svelte';

  let {
    title = '',
    onLoad = null,
    fullHeight = false,
    extraClass = '',
    actions = null,
    children
  }: {
    title?: string;
    onLoad?: (() => void) | null;
    fullHeight?: boolean;
    extraClass?: string;
    actions?: Snippet | null;
    children?: Snippet;
  } = $props();

  onMount(() => {
    if (onLoad) onLoad();
  });
</script>

<div class="p-4 md:p-6 {fullHeight ? 'h-full flex flex-col' : 'space-y-6'} {extraClass}">
  {#if actions}
    <div class="flex items-center justify-end gap-4">
      <div class="flex items-center gap-2 shrink-0">
        {@render actions()}
      </div>
    </div>
  {/if}

  {@render children?.()}
</div>
