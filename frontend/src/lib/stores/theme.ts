import { writable, get } from 'svelte/store';
import type { ThemePayload } from '../../../bindings/sing-box-ez/internal/gui/wails/models.js';

export type ThemeMode = 'dark' | 'light' | 'system';

export interface ThemeData {
  name: string;
  mode: ThemeMode | string;
  colors?: Record<string, string>;
  darkColors?: Record<string, string>;
  lightColors?: Record<string, string>;
}

// fromThemePayload converts a generated Wails theme payload into the frontend ThemeData shape.
export function fromThemePayload(payload: ThemePayload): ThemeData {
  return {
    name: payload.name,
    mode: payload.mode,
    colors: (payload.colors ?? undefined) as Record<string, string> | undefined
  };
}

const defaultColors: Record<string, string> = {
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

export const theme = writable<ThemeData>({
  name: 'default',
  mode: 'system',
  colors: defaultColors
});

// colorScheme reflects the last applied effective scheme ('dark' | 'light'),
// e.g. for syncing the sonner Toaster theme.
export const colorScheme = writable<'dark' | 'light'>('dark');

function isSystemDark(): boolean {
  if (typeof window === 'undefined') return true;
  return window.matchMedia('(prefers-color-scheme: dark)').matches;
}

function resolveEffectiveColors(themeData: ThemeData): Record<string, string> {
  const { colors, darkColors, lightColors, mode } = themeData || {};
  if (mode === 'system' && darkColors && lightColors) {
    return isSystemDark() ? darkColors : lightColors;
  }
  return colors || {};
}

// Foreground colors with guaranteed contrast on primary/destructive backgrounds.
const FG_LIGHT = 'oklch(0.985 0 0)';
const FG_DARK = 'oklch(0.205 0 0)';

function isLightColor(hex: string): boolean {
  const m = hex.trim().match(/^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})/);
  if (!m) return false;
  let h = m[1];
  if (h.length === 3) h = h.split('').map((c) => c + c).join('');
  const r = parseInt(h.slice(0, 2), 16) / 255;
  const g = parseInt(h.slice(2, 4), 16) / 255;
  const b = parseInt(h.slice(4, 6), 16) / 255;
  const lum = 0.2126 * r + 0.7152 * g + 0.0722 * b;
  return lum > 0.5;
}

function contrastForeground(color: string): string {
  return isLightColor(color) ? FG_DARK : FG_LIGHT;
}

// Maps backend theme colors (hex) onto shadcn CSS tokens.
// Keys absent from the payload are left untouched (fall back to style.css).
function applyColors(root: HTMLElement, colors: Record<string, string>) {
  const set = (token: string, value: string) => root.style.setProperty(token, value);

  if (colors.bg) set('--background', colors.bg);
  if (colors.surface) {
    set('--card', colors.surface);
    set('--popover', colors.surface);
  }
  if (colors['surface-variant']) {
    set('--secondary', colors['surface-variant']);
    set('--muted', colors['surface-variant']);
    set('--accent', colors['surface-variant']);
  }
  if (colors.border) {
    set('--border', colors.border);
    set('--input', colors.border);
  }
  if (colors.text) {
    set('--foreground', colors.text);
    set('--card-foreground', colors.text);
    set('--popover-foreground', colors.text);
  }
  if (colors['text-muted']) set('--muted-foreground', colors['text-muted']);
  if (colors.primary) {
    set('--primary', colors.primary);
    set('--ring', colors.primary);
    set('--primary-foreground', contrastForeground(colors.primary));
  }
  if (colors.danger) {
    set('--destructive', colors.danger);
    set('--destructive-foreground', contrastForeground(colors.danger));
  }
  // Extra status colors — exposed as plain variables for inline use.
  if (colors.success) set('--color-success', colors.success);
  if (colors.warning) set('--color-warning', colors.warning);
  if (colors.info) set('--color-info', colors.info);
  if (colors['status-ok']) set('--color-status-ok', colors['status-ok']);
  if (colors['status-warning']) set('--color-status-warning', colors['status-warning']);
  // Config card state colors (cache state × auto-update, legacy gio palette).
  if (colors['card-cached']) set('--color-card-cached', colors['card-cached']);
  if (colors['card-uncached']) set('--color-card-uncached', colors['card-uncached']);
  if (colors['card-cached-no-auto-update'])
    set('--color-card-cached-no-auto-update', colors['card-cached-no-auto-update']);
  if (colors['card-uncached-no-auto-update'])
    set('--color-card-uncached-no-auto-update', colors['card-uncached-no-auto-update']);
}

export function applyTheme(themeData: ThemeData | null | undefined) {
  const root = document.documentElement;

  const { mode } = themeData || {};
  let scheme: 'dark' | 'light' = 'dark';
  if (mode === 'light') {
    scheme = 'light';
  } else if (mode === 'system') {
    scheme = isSystemDark() ? 'dark' : 'light';
  } else if (mode === 'dark') {
    scheme = 'dark';
  }

  root.classList.toggle('dark', scheme === 'dark');
  root.style.colorScheme = scheme;
  colorScheme.set(scheme);

  const effective = resolveEffectiveColors(themeData || { name: 'default', mode: 'system' });
  applyColors(root, effective);
}

if (typeof window !== 'undefined') {
  const mql = window.matchMedia('(prefers-color-scheme: dark)');
  mql.addEventListener('change', () => {
    applyTheme(get(theme));
  });
}
