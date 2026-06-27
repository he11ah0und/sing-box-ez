import { writable, derived } from 'svelte/store';

export const locale = writable({
  language: 'en',
  translations: {}
});

export function t(path, fallback = '') {
  return derived(locale, ($locale) => {
    const keys = path.split('.');
    let current = $locale.translations;
    for (const key of keys) {
      if (current == null || typeof current !== 'object') return fallback;
      current = current[key];
    }
    return current ?? fallback;
  });
}

export function tValue(localeData, path, fallback = '') {
  const keys = path.split('.');
  let current = localeData.translations;
  for (const key of keys) {
    if (current == null || typeof current !== 'object') return fallback;
    current = current[key];
  }
  return current ?? fallback;
}
