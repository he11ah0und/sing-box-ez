import { setTransport } from '@wailsio/runtime'

const isDesktop =
  typeof window !== 'undefined' &&
  (!!window.chrome?.webview?.postMessage ||
    !!window.webkit?.messageHandlers?.external?.postMessage ||
    !!window.wails?.invoke)

if (!isDesktop) {
  const WS_PORT = import.meta.env?.VITE_WS_PORT || '34115'
  const WS_URL = `ws://127.0.0.1:${WS_PORT}/wails/ws`
  const pending = new Map()
  const queue = []
  let open = false
  let ws = null

  function generateID() {
    return typeof crypto !== 'undefined' && crypto.randomUUID
      ? crypto.randomUUID()
      : `${Date.now()}-${Math.random()}`
  }

  setTransport({
    call(objectID, method, windowName, args) {
      return new Promise((resolve, reject) => {
        if (open) {
          sendCall(objectID, method, windowName, args, resolve, reject)
        } else {
          queue.push({ objectID, method, windowName, args, resolve, reject })
        }
      })
    }
  })

  function sendCall(objectID, method, windowName, args, resolve, reject) {
    const id = generateID()
    pending.set(id, { objectID, method, windowName, args, resolve, reject })
    ws.send(JSON.stringify({
      type: 'call',
      id,
      object: objectID,
      method,
      windowName,
      args
    }))
  }

  function flushQueue() {
    while (queue.length) {
      const { objectID, method, windowName, args, resolve, reject } = queue.shift()
      sendCall(objectID, method, windowName, args, resolve, reject)
    }
  }

  function connect() {
    open = false
    ws = new WebSocket(WS_URL)

    ws.addEventListener('open', () => {
      open = true
      flushQueue()
    })

    ws.addEventListener('message', (event) => {
      let msg
      try {
        msg = JSON.parse(event.data)
      } catch {
        return
      }

      if (msg.type === 'call') {
        const p = pending.get(msg.id)
        if (!p) return
        pending.delete(msg.id)
        if (msg.error) {
          p.reject(new Error(msg.error))
        } else {
          p.resolve(msg.result)
        }
      } else if (msg.type === 'event') {
        if (window._wails?.dispatchWailsEvent) {
          window._wails.dispatchWailsEvent({ name: msg.name, data: msg.data })
        }
      }
    })

    ws.addEventListener('close', () => {
      open = false
      // Re-queue pending calls so they are retried after reconnect.
      for (const p of pending.values()) {
        queue.push(p)
      }
      pending.clear()
      setTimeout(connect, 1000)
    })

    ws.addEventListener('error', (err) => {
      console.warn('WebSocket IPC error:', err)
    })
  }

  connect()
}
