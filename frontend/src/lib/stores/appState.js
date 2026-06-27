import { writable } from 'svelte/store';

export const appState = writable({
  status: { running: false, processing: false },
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
  logs: { app: [], core: [] },
  traffic: { up: 0, down: 0 },
  notifications: [],
  dialog: null,
  startup: { show: false, options: [], selected: null }
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
