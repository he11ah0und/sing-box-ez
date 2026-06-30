import { writable } from 'svelte/store';

export const appState = writable({
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
  notifications: [],
  dialog: null,
  startup: { show: false, options: [], selected: null },
  selfUpdate: { downloading: false, downloadProgress: 0 }
});

export function appendAppLog(line) {
  appState.update((s) => ({
    ...s,
    logs: { ...s.logs, app: [...s.logs.app, line].slice(-1000) }
  }));
}

export function appendCoreLog(line) {
  appState.update((s) => ({
    ...s,
    logs: { ...s.logs, core: [...s.logs.core, line].slice(-1000) }
  }));
}

export function clearLogs() {
  appState.update((s) => ({ ...s, logs: { app: [], core: [] } }));
}
