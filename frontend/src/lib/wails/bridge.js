import { Events } from '@wailsio/runtime'
import { appState, appendAppLog, appendCoreLog } from '../stores/appState.js'
import { locale, setLocaleValues } from '../stores/locale.js'
import { theme, applyTheme } from '../stores/theme.js'

export function initWailsEvents() {
  Events.On('status:changed', (event) => {
    const data = event.data ?? {}
    appState.update((s) => ({
      ...s,
      status: { ...s.status, running: data.running ?? false, pid: data.pid ?? 0 }
    }))
  })

  Events.On('configs:changed', (event) => {
    const data = event.data ?? {}
    appState.update((s) => ({
      ...s,
      configs: data.configs ?? s.configs,
      activeConfig: data.active ?? s.activeConfig
    }))
  })

  Events.On('settings:changed', (event) => {
    appState.update((s) => ({ ...s, settings: event.data ?? {} }))
  })

  Events.On('core:version', (event) => {
    const data = event.data ?? {}
    appState.update((s) => ({
      ...s,
      coreInfo: {
        ...s.coreInfo,
        ...(data.installed !== undefined && { installedVersion: data.installed }),
        ...(data.latest !== undefined && { latestVersion: data.latest })
      }
    }))
  })

  Events.On('log:app', (event) => appendAppLog(event.data?.line))
  Events.On('log:core', (event) => appendCoreLog(event.data?.line))

  Events.On('traffic:updated', (event) => {
    const data = event.data ?? {}
    appState.update((s) => {
      const maxPoints = 60
      const upHistory = [...s.traffic.history.up, data.up ?? 0].slice(-maxPoints)
      const downHistory = [...s.traffic.history.down, data.down ?? 0].slice(-maxPoints)
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
      }
    })
  })

  Events.On('locale:changed', (event) => {
    const data = event.data ?? {}
    locale.set({ language: data.language ?? 'en', values: data.values ?? {} })
  })

  Events.On('locale:keys_changed', (event) => {
    setLocaleValues(event.data)
  })

  Events.On('theme:changed', (event) => {
    theme.set(event.data)
    applyTheme(event.data)
  })

  Events.On('dialog:show', (event) => {
    appState.update((s) => ({ ...s, dialog: event.data }))
  })

  Events.On('dialog:hide', () => {
    appState.update((s) => ({ ...s, dialog: null }))
  })

  Events.On('notification', (event) => {
    appState.update((s) => ({
      ...s,
      notifications: [...s.notifications.slice(-4), event.data]
    }))
  })

  Events.On('update:progress', (event) => {
    const data = event.data ?? {}
    appState.update((s) => ({
      ...s,
      coreInfo: {
        ...s.coreInfo,
        downloading: data.total > 0 && data.downloaded < data.total,
        downloadProgress: data.total > 0 ? data.downloaded / data.total : 0
      }
    }))
  })

  Events.On('selfupdate:progress', (event) => {
    const data = event.data ?? {}
    appState.update((s) => ({
      ...s,
      selfUpdate: {
        ...(s.selfUpdate ?? {}),
        downloading: data.total > 0 && data.downloaded < data.total,
        downloadProgress: data.total > 0 ? data.downloaded / data.total : 0
      }
    }))
  })
}
