import { writable } from 'svelte/store';

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
  warning: '#f59e0b'
};

export const theme = writable({
  name: 'default',
  mode: 'system',
  colors: defaultColors
});

export function applyTheme(colors) {
  const root = document.documentElement;
  const effective = { ...defaultColors, ...(colors || {}) };
  Object.entries(effective).forEach(([key, value]) => {
    root.style.setProperty(`--color-${key}`, value);
  });
}
