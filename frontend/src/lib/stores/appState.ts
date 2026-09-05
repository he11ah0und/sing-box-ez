import { writable } from 'svelte/store';
import type { ConfigRecord } from '../../../bindings/sing-box-ez/internal/config/models.js';
import type {
  ActiveConfig,
  CoreInfo,
  SelfUpdateInfo
} from '../../../bindings/sing-box-ez/internal/gui/wails/models.js';
import type {
  Connection as APIConnection,
  ConnectionGroup as APIConnectionGroup,
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
  // modeList holds the clash modes the running core accepts; empty when the
  // core does not report a list (older sing-box / plain Clash API).
  modeList: string[];
  groups: APIGroup[];
  connections: APIConnection[];
  // connGroups aggregates connections by target; inactive groups are kept
  // until the configured retention elapses.
  connGroups: APIConnectionGroup[];
  // session counts core API connection sessions; a change means the core
  // was (re)started and per-session data must reset.
  session: number;
}

export interface AppState {
  status: CoreStatus;
  configs: ConfigRecord[];
  activeConfig: ActiveConfig | null;
  settings: Record<string, any>;
  // settingsLoaded marks that settings were fetched at least once, so pages
  // render from the store instead of re-fetching on every mount.
  settingsLoaded: boolean;
  // ready flips once the initial backend seeds (traffic history, API state,
  // settings) have settled; the shell shows a loading state until then.
  ready: boolean;
  // coreInfo is the backend model plus the frontend-only installing flag:
  // set by the update:phase event while the downloaded binary is being
  // installed (the Go model only covers the download phase).
  coreInfo: CoreInfo & { installing: boolean };
  logs: LogsState;
  traffic: TrafficState;
  dialog: DialogPayload | null;
  // privilegesRequired is set by the privileges:required event: a start was
  // refused because the config needs setcap/admin; App shows a dialog that
  // leads to the privilege settings.
  privilegesRequired: boolean;
  startup: StartupState;
  selfUpdate: SelfUpdateState;
  styleCheck: StyleCheckState | null;
  selfUpdateInfo: SelfUpdateInfo | null;
  // updateChannelError is set by the update:channel_error event: an
  // externally managed build (AUR, ...) failed to verify its own version —
  // App shows a blocking danger dialog with Quit / Continue anyway.
  updateChannelError: { channel: string; error: string } | null;
  // configsUpdateFailed is set by the configs:update_failed event: every
  // config due for a background refresh failed to download — App shows a
  // dialog listing the configs with the classified failure reason.
  configsUpdateFailed: { name: string; kind: string }[] | null;
  // configDownloadFailed is set by the config:download_failed event: the
  // start flow could not download the active config and used the cached
  // copy — App offers a retry through the running core's proxy.
  configDownloadFailed: { name: string; kind: string } | null;
  // updateCheckFailed is set by the updatecheck:failed event: a background
  // update check ("app" or "core") failed on connectivity — App offers a
  // retry through the running core's proxy.
  updateCheckFailed: { target: string; kind: string } | null;
  api: ApiState;
}

export const appState = writable<AppState>({
  status: { running: false, processing: false, pid: 0 },
  configs: [],
  activeConfig: null,
  settings: {},
  settingsLoaded: false,
  ready: false,
  coreInfo: {
    installedVersion: '',
    latestVersion: '',
    downloading: false,
    downloadProgress: 0,
    installing: false
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
  privilegesRequired: false,
  startup: { show: false, options: [], selected: null },
  selfUpdate: { downloading: false, downloadProgress: 0 },
  styleCheck: null,
  selfUpdateInfo: null,
  updateChannelError: null,
  configsUpdateFailed: null,
  configDownloadFailed: null,
  updateCheckFailed: null,
  api: { phase: 'stopped', status: null, info: null, mode: '', modeList: [], groups: [], connections: [], connGroups: [], session: 0 }
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
