import { writable } from 'svelte/store';
import type { ConfigRecord } from '../../../bindings/sing-box-ez/internal/config/models.js';
import type {
  ActiveConfig,
  CoreInfo,
  SelfUpdateInfo,
  Settings
} from '../../../bindings/sing-box-ez/internal/gui/wails/models.js';
import type {
  Connection as APIConnection,
  Group as APIGroup,
  Info as APIInfo,
  Status as APIStatus
} from '../../../bindings/sing-box-ez/internal/core/state/models.js';

// Re-export generated Wails models so existing imports from this module keep working.
export type { ConfigRecord, CoreInfo, SelfUpdateInfo };

export interface CoreStatus {
  running: boolean;
  processing: boolean;
  pid: number;
}

export interface TrafficHistory {
  // times holds per-sample timestamps (ms); up/down are rate samples aligned
  // with it. Points enter at the right edge of the graph and drift left.
  times: number[];
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

export interface StyleCheckState {
  config: string;
  style: string;
}

// ApiState is the latest core API snapshot pushed by the backend via
// api:state events. Pages read it instead of polling.
// phase is one of: stopped | starting | waiting_api | connected.
export interface ApiState {
  phase: string;
  status: APIStatus | null;
  info: APIInfo | null;
  mode: string;
  groups: APIGroup[];
  connections: APIConnection[];
}

export interface AppState {
  status: CoreStatus;
  configs: ConfigRecord[];
  activeConfig: ActiveConfig | null;
  settings: Settings;
  coreInfo: CoreInfo;
  logs: LogsState;
  traffic: TrafficState;
  dialog: DialogPayload | null;
  startup: StartupState;
  selfUpdate: SelfUpdateState;
  styleCheck: StyleCheckState | null;
  selfUpdateInfo: SelfUpdateInfo | null;
  api: ApiState;
}

export const appState = writable<AppState>({
  status: { running: false, processing: false, pid: 0 },
  configs: [],
  activeConfig: null,
  settings: {} as Settings,
  coreInfo: {
    installedVersion: '',
    latestVersion: '',
    downloading: false,
    downloadProgress: 0
  },
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
    history: { times: [], up: [], down: [] }
  },
  dialog: null,
  startup: { show: false, options: [], selected: null },
  selfUpdate: { downloading: false, downloadProgress: 0 },
  styleCheck: null,
  selfUpdateInfo: null,
  api: { phase: 'stopped', status: null, info: null, mode: '', groups: [], connections: [] }
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
