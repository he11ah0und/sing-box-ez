import type { Component } from 'svelte';
import { Home, List, Settings as SettingsIcon, Bug, Info, Cpu, Menu } from '@lucide/svelte';

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

interface PageModule {
  default: Component;
}

// Static page metadata, kept here (not in module scripts) so page components
// can be code-split and loaded lazily via import.meta.glob.
export const pageRegistry: PageMeta[] = [
  { id: 'main', key: 'tab.main', icon: Home, nav: true, bottomNav: true, order: 0 },
  { id: 'configs', key: 'tab.configs', icon: List, nav: true, bottomNav: true, order: 1 },
  {
    id: 'settings',
    key: 'tab.settings',
    icon: SettingsIcon,
    nav: true,
    bottomNav: true,
    order: 2,
    tabs: [
      { id: 'core', key: 'settings.tab.core' },
      { id: 'log', key: 'settings.tab.log' },
      { id: 'ui', key: 'settings.tab.ui' },
      { id: 'updates', key: 'settings.tab.updates' },
      { id: 'system', key: 'settings.tab.system' }
    ]
  },
  {
    id: 'debug',
    key: 'common.debug',
    icon: Bug,
    nav: true,
    bottomNav: true,
    order: 3,
    tabs: [
      { id: 'app', key: 'log.tab.app' },
      { id: 'core', key: 'tab.core' }
    ]
  },
  { id: 'about', key: 'tab.about', icon: Info, nav: true, bottomNav: true, order: 4 },
  { id: 'core', key: 'tab.core', icon: Cpu, nav: false, bottomNav: false },
  { id: 'menu', key: 'tab.menu', icon: Menu, nav: false, bottomNav: false }
];

const pageFiles: Record<string, string> = {
  main: './MainPage.svelte',
  configs: './ConfigsPage.svelte',
  settings: './SettingsPage.svelte',
  debug: './DebugPage.svelte',
  about: './AboutPage.svelte',
  core: './CorePage.svelte',
  menu: './MenuPage.svelte'
};

const modules = import.meta.glob<PageModule>('./*.svelte');

export async function loadPage(id: string): Promise<Component> {
  const file = pageFiles[id] ?? pageFiles.main;
  const mod = await modules[file]();
  return mod.default;
}
