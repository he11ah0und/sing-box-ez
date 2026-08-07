import { writable, derived, get, type Readable } from 'svelte/store';
import {
  RegisterLocaleKeys,
  LocaleReady
} from '../../../bindings/sing-box-ez/internal/gui/wails/bindings.js';

export interface LocaleState {
  language: string;
  values: Record<string, string>;
}

// locale holds the active language and a flat map of registered translation values.
export const locale = writable<LocaleState>({
  language: 'en',
  values: {}
});

const pendingKeys = new Set<string>();
let ready = false;
let flushScheduled = false;

function flush() {
  flushScheduled = false;
  if (pendingKeys.size === 0) return;
  const keys = Array.from(pendingKeys);
  pendingKeys.clear();
  RegisterLocaleKeys(keys)
    .then((values: Record<string, string> | null) => {
      locale.update((l) => ({ ...l, values: { ...l.values, ...(values || {}) } }));
    })
    .catch((err: unknown) => {
      console.warn('RegisterLocaleKeys failed:', err);
    });
}

export function registerLocaleKey(key: string) {
  if (!key || pendingKeys.has(key)) return;
  pendingKeys.add(key);
  if (!flushScheduled) {
    flushScheduled = true;
    queueMicrotask(flush);
  }
}

export function registerLocaleKeys(keys: string[] | null | undefined) {
  if (!keys) return;
  keys.forEach(registerLocaleKey);
}

export async function signalLocaleReady() {
  await Promise.resolve();
  ready = true;
  flush();
  try {
    await LocaleReady();
  } catch (err) {
    console.warn('LocaleReady failed:', err);
  }
}

export function setLocaleValues(values: Record<string, string> | null | undefined) {
  locale.update((l) => ({ ...l, values: { ...l.values, ...(values || {}) } }));
}

export function setLocale(language: string, values?: Record<string, string> | null) {
  locale.set({ language, values: values || {} });
}

// tValue reads a registered key from a locale snapshot.
// Calling it automatically registers the key for backend delivery.
export function tValue(localeData: LocaleState | null | undefined, key: string, fallback = ''): string {
  if (key) registerLocaleKey(key);
  return localeData?.values?.[key] ?? fallback;
}

// t returns a Svelte store with the translation for a registered key.
export function t(key: string, fallback = ''): Readable<string> {
  return derived(locale, ($locale) => $locale.values[key] ?? fallback);
}

// useLocale registers a key and returns a reactive store for its translation.
export function useLocale(key: string, fallback = ''): Readable<string> {
  registerLocaleKey(key);
  return t(key, fallback);
}

// getLocaleString registers a key and returns its current translation value.
export function getLocaleString(key: string, fallback = ''): string {
  return get(useLocale(key, fallback));
}
