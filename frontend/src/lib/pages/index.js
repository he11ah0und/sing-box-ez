const modules = import.meta.glob('./*.svelte', { eager: true });

export const pageRegistry = Object.values(modules)
  .map((mod) => ({ ...mod.pageMeta, component: mod.default }))
  .filter((page) => page?.id)
  .sort((a, b) => (a.order ?? Infinity) - (b.order ?? Infinity));

export const pageComponents = Object.fromEntries(
  pageRegistry.map((page) => [page.id, page.component])
);
