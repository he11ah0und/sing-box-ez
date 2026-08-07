import type { Component } from 'svelte';

export interface PageTab {
  id: string;
  key: string;
}

export interface PageMeta {
  id: string;
  key: string;
  icon?: Component;
  nav?: boolean;
  bottomNav?: boolean;
  order?: number;
  tabs?: PageTab[];
}

export interface PageEntry extends PageMeta {
  component: Component;
}

interface PageModule {
  default: Component;
  pageMeta?: PageMeta;
}

const modules = import.meta.glob<PageModule>('./*.svelte', { eager: true });

export const pageRegistry: PageEntry[] = Object.values(modules)
  .map((mod) => ({ ...mod.pageMeta, component: mod.default }) as PageEntry)
  .filter((page) => page?.id)
  .sort((a, b) => (a.order ?? Infinity) - (b.order ?? Infinity));

export const pageComponents: Record<string, Component> = Object.fromEntries(
  pageRegistry.map((page) => [page.id, page.component])
);
