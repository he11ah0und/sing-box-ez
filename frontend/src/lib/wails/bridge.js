import {Events} from '@wailsio/runtime'
import {appState, appendAppLog, appendCoreLog} from '../stores/appState.js'
import {locale} from '../stores/locale.js'
import {theme, applyTheme} from '../stores/theme.js'

export function initWailsEvents() {
  Events.On('status:changed', (event) => {
    const data = event.data ?? {}
    appState.update((s) => ({
      ...s,
      status: {...s.status, running: data.running}
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
    const data = event.data ?? {}
    appState.update((s) => ({...s, settings: data.settings ?? {}}))
  })

  Events.On('log:app', (event) => appendAppLog(event.data?.line))
  Events.On('log:core', (event) => appendCoreLog(event.data?.line))

  Events.On('traffic:updated', (event) => {
    appState.update((s) => ({...s, traffic: event.data}))
  })

  Events.On('locale:changed', (event) => {
    locale.set(event.data)
  })

  Events.On('theme:changed', (event) => {
    theme.set(event.data)
    applyTheme(event.data?.colors)
  })

  Events.On('dialog:show', (event) => {
    appState.update((s) => ({...s, dialog: event.data}))
  })

  Events.On('dialog:hide', () => {
    appState.update((s) => ({...s, dialog: null}))
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
}
