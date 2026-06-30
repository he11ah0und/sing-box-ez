import { writable, derived } from 'svelte/store';

export const navigationStack = writable([{ type: 'page', id: 'main' }]);

export const currentLevel = derived(
  navigationStack,
  ($stack) => $stack[$stack.length - 1]
);

export const canGoBack = derived(
  navigationStack,
  ($stack) => $stack.length > 1
);

export const subNav = writable({ pageId: null, activeTab: null });

export function pushLevel(level) {
  navigationStack.update((stack) => [...stack, level]);
}

export function popLevel() {
  navigationStack.update((stack) => {
    if (stack.length <= 1) return stack;
    return stack.slice(0, -1);
  });
}

export function replaceTop(level) {
  navigationStack.update((stack) => {
    if (stack.length === 0) return [level];
    return [...stack.slice(0, -1), level];
  });
}

export function goHome() {
  navigationStack.set([{ type: 'page', id: 'main' }]);
}

export function setRootPage(id) {
  navigationStack.set([{ type: 'page', id }]);
}

export function enterSubNav(pageId, tabs = []) {
  subNav.set({
    pageId,
    activeTab: tabs[0]?.id ?? null
  });
}

export function setSubTab(id) {
  subNav.update((s) => ({ ...s, activeTab: id }));
}

export function exitSubNav() {
  subNav.set({ pageId: null, activeTab: null });
}
