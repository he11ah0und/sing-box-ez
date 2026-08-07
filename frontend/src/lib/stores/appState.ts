import { writable } from 'svelte/store';

export interface CoreStatus {
  running: boolean;
  processing: boolean;
  pid: number;
}

export interface ConfigRecord {
  name: string;
  url?: string;
  type?: string;
  update_interval_hours?: number;
  last_update?: string | null;
  parent?: string;
  auto_update?: boolean;
  hash?: string;
  fallback_type?: string | null;
}

export interface CoreInfo {
  installedVersion: string;
  latestVersion: string;
  downloading: boolean;
  downloadProgress: number;
}

export interface TrafficHistory {
  up: number[];
  down: number[];
}

export interface TrafficState {
  up: number;
  down: number;
  upTotal: number;
  downTotal: number;
  upRate: string;
  downRate: string;
  connected: boolean;
  backend: string;
  version: string;
  connections: number;
  history: TrafficHistory;
}

export interface LogsState {
  app: string[];
  core: string[];
}

export interface DialogPayload {
  title?: string;
  body?: string;
}

export interface StartupOption {
  id: string;
  label: string;
}

export interface StartupState {
  show: boolean;
  options: StartupOption[];
  selected: string | null;
}

export interface SelfUpdateState {
  downloading: boolean;
  downloadProgress: number;
}

export interface AppState {
  status: CoreStatus;
  configs: ConfigRecord[];
  activeConfig: ConfigRecord | null;
  settings: Record<string, unknown>;
  coreInfo: CoreInfo;
  apiStatus: unknown;
  apiMode: string;
  apiGroups: unknown[];
  apiConnections: unknown[];
  logs: LogsState;
  traffic: TrafficState;
  dialog: DialogPayload | null;
  startup: StartupState;
  selfUpdate: SelfUpdateState;
}

export const appState = writable<AppState>({
  status: { running: false, processing: false, pid: 0 },
  configs: [],
  activeConfig: null,
  settings: {},
  coreInfo: {
    installedVersion: '',
    latestVersion: '',
    downloading: false,
    downloadProgress: 0
  },
  apiStatus: null,
  apiMode: '',
  apiGroups: [],
  apiConnections: [],
  logs: { app: [], core: [] },
  traffic: {
    up: 0,
    down: 0,
    upTotal: 0,
    downTotal: 0,
    upRate: '0 B/s',
    downRate: '0 B/s',
    connected: false,
    backend: '',
    version: '',
    connections: 0,
    history: { up: [], down: [] }
  },
  dialog: null,
  startup: { show: false, options: [], selected: null },
  selfUpdate: { downloading: false, downloadProgress: 0 }
});

export function appendAppLog(line: string) {
  appState.update((s) => ({
    ...s,
    logs: { ...s.logs, app: [...s.logs.app, line].slice(-1000) }
  }));
}

export function appendCoreLog(line: string) {
  appState.update((s) => ({
    ...s,
    logs: { ...s.logs, core: [...s.logs.core, line].slice(-1000) }
  }));
}

export function clearLogs() {
  appState.update((s) => ({ ...s, logs: { app: [], core: [] } }));
}
