import { writable, derived, type Readable } from 'svelte/store';
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

// registeredKeys remembers every key ever sent to the backend so a key
// is registered exactly once per session, no matter how many components
// subscribe to it.
const registeredKeys = new Set<string>();
const pendingKeys = new Set<string>();
let ready = false;
let flushScheduled = false;

function flush() {
  flushScheduled = false;
  if (pendingKeys.size === 0) return;
  const keys = Array.from(pendingKeys);
  pendingKeys.clear();
  RegisterLocaleKeys(keys)
    .then((values) => {
      setLocaleValues(values as Record<string, string> | null);
    })
    .catch((err: unknown) => {
      console.warn('RegisterLocaleKeys failed:', err);
    });
}

export function registerLocaleKey(key: string) {
  if (!key || registeredKeys.has(key)) return;
  registeredKeys.add(key);
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

// tValue reads a key from a locale snapshot. Calling it automatically
// registers the key for backend delivery. Use it only for dynamic keys
// (computed at render time); static keys must be declared once per
// component via useLocale and subscribed as {$cell}.
// A missing value renders the key itself — English fallbacks in code are
// forbidden (the hardcode-scan guard flags them); the single source of
// truth is internal/app/locales/*.yaml.
export function tValue(localeData: LocaleState | null | undefined, key: string): string {
  if (key) registerLocaleKey(key);
  return (key && localeData?.values?.[key]) || key;
}

// tValues tries keys in order and returns the first resolved value; all
// keys are registered. When nothing resolves, the last key is returned.
export function tValues(localeData: LocaleState | null | undefined, keys: string[]): string {
  for (const key of keys) {
    if (key) registerLocaleKey(key);
  }
  for (const key of keys) {
    const v = key && localeData?.values?.[key];
    if (v) return v;
  }
  return keys[keys.length - 1] ?? '';
}

// t returns a Svelte store with the translation for a registered key.
export function t(key: string): Readable<string> {
  return derived(locale, ($locale) => $locale.values[key] ?? key);
}

// cells memoizes one derived store per key so repeated useLocale calls
// for the same key share a single subscription.
const cells = new Map<string, Readable<string>>();

// useLocale registers a key and returns its memoized reactive cell.
// Declare cells once at component init and subscribe in markup as {$cell};
// values flow in via the locale:keys_changed / locale:changed events and
// the RegisterLocaleKeys response.
export function useLocale(key: string): Readable<string> {
  registerLocaleKey(key);
  let cell = cells.get(key);
  if (!cell) {
    cell = t(key);
    cells.set(key, cell);
  }
  return cell;
}
