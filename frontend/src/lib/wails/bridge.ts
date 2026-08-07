import { Events } from '@wailsio/runtime';
import { toast } from 'svelte-sonner';
import { appState, appendAppLog, appendCoreLog, type ConfigRecord } from '../stores/appState.js';
import { locale, setLocaleValues } from '../stores/locale.js';
import { theme, applyTheme, type ThemeData } from '../stores/theme.js';
import type { SelfUpdateInfo, Settings } from '../../../bindings/sing-box-ez/internal/gui/wails/models.js';

interface WailsEvent<T> {
  data: T;
}

interface StatusChangedPayload {
  running?: boolean;
  pid?: number;
}

interface ConfigsChangedPayload {
  configs?: ConfigRecord[];
  active?: ConfigRecord | null;
}

interface CoreVersionPayload {
  installed?: string;
  latest?: string;
}

interface LogLinePayload {
  line?: string;
}

interface TrafficPayload {
  up?: number;
  down?: number;
  upTotal?: number;
  downTotal?: number;
  upRate?: string;
  downRate?: string;
  connected?: boolean;
  backend?: string;
  version?: string;
  connections?: number;
}

interface LocaleChangedPayload {
  language?: string;
  values?: Record<string, string>;
}

interface NotificationPayload {
  title?: string;
  body?: string;
  message?: string;
  type?: string;
}

interface UpdateProgressPayload {
  total?: number;
  downloaded?: number;
}

interface StyleCheckPayload {
  config?: string;
  style?: string;
}

function showNotification(data: NotificationPayload) {
  const title = data.title ?? '';
  const description = data.body ?? data.message;
  const options = description ? { description } : undefined;
  switch (data.type) {
    case 'success':
      toast.success(title, options);
      break;
    case 'error':
      toast.error(title, options);
      break;
    case 'warning':
      toast.warning(title, options);
      break;
    default:
      toast(title, options);
  }
}

export function initWailsEvents() {
  Events.On('status:changed', (event: WailsEvent<StatusChangedPayload>) => {
    const data = event.data ?? {};
    appState.update((s) => ({
      ...s,
      status: { ...s.status, running: data.running ?? false, pid: data.pid ?? 0 }
    }));
  });

  Events.On('configs:changed', (event: WailsEvent<ConfigsChangedPayload>) => {
    const data = event.data ?? {};
    appState.update((s) => ({
      ...s,
      configs: data.configs ?? s.configs,
      activeConfig: data.active ?? s.activeConfig
    }));
  });

  Events.On('config:style_check', (event: WailsEvent<StyleCheckPayload>) => {
    const data = event.data ?? {};
    if (!data.config) return;
    appState.update((s) => ({
      ...s,
      styleCheck: { config: data.config ?? '', style: data.style ?? 'undefined' }
    }));
  });

  Events.On('selfupdate:available', (event: WailsEvent<SelfUpdateInfo>) => {
    if (!event.data) return;
    appState.update((s) => ({ ...s, selfUpdateInfo: event.data }));
  });

  Events.On('settings:changed', (event: WailsEvent<Settings>) => {
    appState.update((s) => ({ ...s, settings: event.data ?? {} as Settings }));
  });

  Events.On('core:version', (event: WailsEvent<CoreVersionPayload>) => {
    const data = event.data ?? {};
    appState.update((s) => ({
      ...s,
      coreInfo: {
        ...s.coreInfo,
        ...(data.installed !== undefined && { installedVersion: data.installed }),
        ...(data.latest !== undefined && { latestVersion: data.latest })
      }
    }));
  });

  Events.On('log:app', (event: WailsEvent<LogLinePayload>) => {
    if (event.data?.line !== undefined) appendAppLog(event.data.line);
  });
  Events.On('log:core', (event: WailsEvent<LogLinePayload>) => {
    if (event.data?.line !== undefined) appendCoreLog(event.data.line);
  });

  Events.On('traffic:updated', (event: WailsEvent<TrafficPayload>) => {
    const data = event.data ?? {};
    appState.update((s) => {
      const maxPoints = 60;
      const upHistory = [...s.traffic.history.up, data.up ?? 0].slice(-maxPoints);
      const downHistory = [...s.traffic.history.down, data.down ?? 0].slice(-maxPoints);
      return {
        ...s,
        traffic: {
          ...s.traffic,
          up: data.up ?? 0,
          down: data.down ?? 0,
          upTotal: data.upTotal ?? 0,
          downTotal: data.downTotal ?? 0,
          upRate: data.upRate ?? '0 B/s',
          downRate: data.downRate ?? '0 B/s',
          connected: data.connected ?? false,
          backend: data.backend ?? '',
          version: data.version ?? '',
          connections: data.connections ?? 0,
          history: { up: upHistory, down: downHistory }
        }
      };
    });
  });

  Events.On('locale:changed', (event: WailsEvent<LocaleChangedPayload>) => {
    const data = event.data ?? {};
    locale.set({ language: data.language ?? 'en', values: data.values ?? {} });
  });

  Events.On('locale:keys_changed', (event: WailsEvent<Record<string, string>>) => {
    setLocaleValues(event.data);
  });

  Events.On('theme:changed', (event: WailsEvent<ThemeData>) => {
    theme.set(event.data);
    applyTheme(event.data);
  });

  Events.On('dialog:show', (event: WailsEvent<{ title?: string; body?: string }>) => {
    appState.update((s) => ({ ...s, dialog: event.data }));
  });

  Events.On('dialog:hide', () => {
    appState.update((s) => ({ ...s, dialog: null }));
  });

  Events.On('notification', (event: WailsEvent<NotificationPayload>) => {
    showNotification(event.data ?? {});
  });

  Events.On('update:progress', (event: WailsEvent<UpdateProgressPayload>) => {
    const data = event.data ?? {};
    const total = data.total ?? 0;
    const downloaded = data.downloaded ?? 0;
    appState.update((s) => ({
      ...s,
      coreInfo: {
        ...s.coreInfo,
        downloading: total > 0 && downloaded < total,
        downloadProgress: total > 0 ? downloaded / total : 0
      }
    }));
  });

  Events.On('selfupdate:progress', (event: WailsEvent<UpdateProgressPayload>) => {
    const data = event.data ?? {};
    const total = data.total ?? 0;
    const downloaded = data.downloaded ?? 0;
    appState.update((s) => ({
      ...s,
      selfUpdate: {
        ...s.selfUpdate,
        downloading: total > 0 && downloaded < total,
        downloadProgress: total > 0 ? downloaded / total : 0
      }
    }));
  });
}
