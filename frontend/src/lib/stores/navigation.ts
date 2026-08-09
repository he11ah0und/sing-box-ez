import { writable, derived } from 'svelte/store';

export interface Level {
  type: 'page' | 'subnav';
  id: string;
}

export interface SubNavTab {
  id: string;
  key: string;
}

export interface SubNavState {
  pageId: string | null;
  activeTab: string | null;
}

export const navigationStack = writable<Level[]>([{ type: 'page', id: 'main' }]);

export const currentLevel = derived(
  navigationStack,
  ($stack) => $stack[$stack.length - 1]
);

export const canGoBack = derived(
  navigationStack,
  ($stack) => $stack.length > 1
);

export const subNav = writable<SubNavState>({ pageId: null, activeTab: null });

// Mobile menu-page mode: when the bottom-bar menu button is tapped from a
// tabbed page (settings/debug), the menu shows that page's tabs instead of
// the full secondary-pages list. Cleared on any navigation to a non-menu
// page and when the mobile back button is pressed.
export const menuContext = writable<string | null>(null);

export function setMenuContext(pageId: string | null) {
  menuContext.set(pageId);
}

export function pushLevel(level: Level) {
  navigationStack.update((stack) => [...stack, level]);
}

export function popLevel() {
  navigationStack.update((stack) => {
    if (stack.length <= 1) return stack;
    return stack.slice(0, -1);
  });
}

export function replaceTop(level: Level) {
  navigationStack.update((stack) => {
    if (stack.length === 0) return [level];
    return [...stack.slice(0, -1), level];
  });
}

export function goHome() {
  navigationStack.set([{ type: 'page', id: 'main' }]);
}

export function setRootPage(id: string) {
  if (id !== 'menu') menuContext.set(null);
  navigationStack.set([{ type: 'page', id }]);
}

export function enterSubNav(pageId: string, tabs: SubNavTab[] = []) {
  subNav.set({
    pageId,
    activeTab: tabs[0]?.id ?? null
  });
}

export function setSubTab(id: string) {
  subNav.update((s) => ({ ...s, activeTab: id }));
}

export function exitSubNav() {
  subNav.set({ pageId: null, activeTab: null });
}
