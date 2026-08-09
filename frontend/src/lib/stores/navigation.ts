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
  pageHistory.length = 0;
  menuContext.set(null);
  navigationStack.set([{ type: 'page', id: 'main' }]);
}

// Root-page back history: setRootPage records the page being left so the
// desktop back button can return to it instead of always landing on main.
const pageHistory: string[] = [];

export function setRootPage(id: string) {
  if (id !== 'menu') menuContext.set(null);
  navigationStack.update((stack) => {
    const cur = stack[stack.length - 1];
    if (cur?.type === 'page' && cur.id !== id) pageHistory.push(cur.id);
    return [{ type: 'page', id }];
  });
}

// setRootPageSilent switches the root page without touching the back
// history — for viewport-driven redirects the user never navigated to.
export function setRootPageSilent(id: string) {
  if (id !== 'menu') menuContext.set(null);
  navigationStack.set([{ type: 'page', id }]);
}

// backPage returns to the previous root page; false when there is none.
export function backPage(): boolean {
  const prev = pageHistory.pop();
  if (!prev) return false;
  if (prev !== 'menu') menuContext.set(null);
  navigationStack.set([{ type: 'page', id: prev }]);
  return true;
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
