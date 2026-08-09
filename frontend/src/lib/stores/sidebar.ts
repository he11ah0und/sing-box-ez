import { writable } from 'svelte/store';

// Desktop side-rail collapse state, persisted in localStorage.
const STORAGE_KEY = 'sidebar-collapsed';

function load(): boolean {
  if (typeof localStorage === 'undefined') return false;
  try {
    return localStorage.getItem(STORAGE_KEY) === '1';
  } catch {
    return false;
  }
}

export const sidebarCollapsed = writable<boolean>(load());

if (typeof localStorage !== 'undefined') {
  sidebarCollapsed.subscribe((collapsed) => {
    try {
      localStorage.setItem(STORAGE_KEY, collapsed ? '1' : '0');
    } catch {
      // storage unavailable (private mode, quota) — keep session-only state
    }
  });
}

export function toggleSidebar() {
  sidebarCollapsed.update((collapsed) => !collapsed);
}
