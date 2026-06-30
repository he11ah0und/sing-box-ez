import { writable, get } from 'svelte/store';

const defaultColors = {
  bg: '#0f172a',
  surface: '#1e293b',
  'surface-variant': '#334155',
  border: '#334155',
  text: '#f8fafc',
  'text-muted': '#94a3b8',
  primary: '#3b82f6',
  'primary-hover': '#2563eb',
  danger: '#ef4444',
  success: '#22c55e',
  warning: '#f59e0b',
  info: '#06b6d4'
};

export const theme = writable({
  name: 'default',
  mode: 'system',
  colors: defaultColors
});

function isSystemDark() {
  if (typeof window === 'undefined') return true;
  return window.matchMedia('(prefers-color-scheme: dark)').matches;
}

function resolveEffectiveColors(themeData) {
  const { colors, darkColors, lightColors, mode } = themeData || {};
  if (mode === 'system' && darkColors && lightColors) {
    return isSystemDark() ? darkColors : lightColors;
  }
  return colors || {};
}

export function applyTheme(themeData) {
  const root = document.documentElement;
  const effective = resolveEffectiveColors(themeData || {});
  const merged = { ...defaultColors, ...effective };

  Object.entries(merged).forEach(([key, value]) => {
    root.style.setProperty(`--color-${key}`, value);
  });

  const { mode } = themeData || {};
  let scheme = 'dark';
  if (mode === 'light') {
    scheme = 'light';
  } else if (mode === 'system') {
    scheme = isSystemDark() ? 'dark' : 'light';
  } else if (mode === 'dark') {
    scheme = 'dark';
  }
  root.style.colorScheme = scheme;
}

if (typeof window !== 'undefined') {
  const mql = window.matchMedia('(prefers-color-scheme: dark)');
  mql.addEventListener('change', () => {
    applyTheme(get(theme));
  });
}
