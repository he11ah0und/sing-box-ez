import type { Component } from 'svelte';
import {
  Home,
  List,
  Settings as SettingsIcon,
  Bug,
  Info,
  Cpu,
  Menu,
  ScrollText,
  Palette,
  RefreshCw,
  Cog,
  Gauge,
  Waypoints,
  ArrowLeftRight
} from '@lucide/svelte';

export interface PageTab {
  id: string;
  key: string;
  icon?: Component;
}

export interface PageMeta {
  id: string;
  key: string;
  icon?: Component;
  nav?: boolean;
  bottomNav?: boolean;
  order?: number;
  tabs?: PageTab[];
  // tabsRequiresCore marks a page whose tabs exist only while the core API
  // is connected (the main page: overview/groups/connections). Without a
  // connection the page renders its standalone content (the start button).
  tabsRequiresCore?: boolean;
}

// tabsVisible reports whether a page's tabs should be offered right now.
export function tabsVisible(page: PageMeta | undefined, coreConnected: boolean): boolean {
  return !!page?.tabs?.length && (!page.tabsRequiresCore || coreConnected);
}

interface PageModule {
  default: Component;
}

// Static page metadata, kept here (not in module scripts) so page components
// can be code-split and loaded lazily via import.meta.glob.
// bottomNav is true only for the three mobile bottom-bar entries
// (main / configs / menu); secondary pages stay reachable via the menu page.
export const pageRegistry: PageMeta[] = [
  {
    id: 'main',
    key: 'tab.main',
    icon: Home,
    nav: true,
    bottomNav: true,
    order: 0,
    // Main's sub-pages appear only while the core is connected; MainPage
    // renders them per $subNav.activeTab (they are not standalone pages).
    tabs: [
      { id: 'overview', key: 'main.tabs.overview', icon: Gauge },
      { id: 'groups', key: 'tab.groups', icon: Waypoints },
      { id: 'connections', key: 'main.api.connections', icon: ArrowLeftRight }
    ],
    tabsRequiresCore: true
  },
  { id: 'configs', key: 'tab.configs', icon: List, nav: true, bottomNav: true, order: 1 },
  {
    id: 'settings',
    key: 'tab.settings',
    icon: SettingsIcon,
    nav: true,
    order: 3,
    tabs: [
      // core/system reuse the existing generic keys (same labels) to avoid
      // duplicate locale values; the rest use the settings.tab.* convention.
      { id: 'core', key: 'tab.core', icon: Cpu },
      { id: 'log', key: 'settings.tab.log', icon: ScrollText },
      { id: 'ui', key: 'settings.tab.ui', icon: Palette },
      { id: 'updates', key: 'settings.tab.updates', icon: RefreshCw },
      { id: 'system', key: 'common.system', icon: Cog }
    ]
  },
  {
    id: 'debug',
    key: 'common.debug',
    icon: Bug,
    nav: true,
    order: 4,
    tabs: [
      { id: 'app', key: 'log.tab.app', icon: ScrollText },
      { id: 'core', key: 'tab.core', icon: Cpu }
    ]
  },
  {
    id: 'about',
    key: 'tab.about',
    icon: Info,
    nav: true,
    order: 5,
    tabs: [
      { id: 'info', key: 'common.info', icon: Info },
      { id: 'updates', key: 'settings.tab.updates', icon: RefreshCw }
    ]
  },
  { id: 'core', key: 'tab.core', icon: Cpu, nav: false, bottomNav: false },
  { id: 'menu', key: 'tab.menu', icon: Menu, nav: false, bottomNav: true, order: 2 }
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
