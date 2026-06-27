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
