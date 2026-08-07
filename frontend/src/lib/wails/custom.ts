import { setTransport } from '@wailsio/runtime';

declare global {
  interface Window {
    chrome?: { webview?: { postMessage?: unknown } };
    webkit?: { messageHandlers?: { external?: { postMessage?: unknown } } };
    wails?: { invoke?: unknown };
    _wails?: {
      dispatchWailsEvent?: (event: { name: string; data: unknown }) => void;
    };
  }
}

interface PendingCall {
  objectID: number;
  method: number;
  windowName: string;
  args: unknown;
  resolve: (value: unknown) => void;
  reject: (reason?: unknown) => void;
}

const isDesktop =
  typeof window !== 'undefined' &&
  (!!window.chrome?.webview?.postMessage ||
    !!window.webkit?.messageHandlers?.external?.postMessage ||
    !!window.wails?.invoke);

if (!isDesktop) {
  const WS_PORT = import.meta.env?.VITE_WS_PORT || '34115';
  const WS_URL = `ws://127.0.0.1:${WS_PORT}/wails/ws`;
  const pending = new Map<string, PendingCall>();
  const queue: PendingCall[] = [];
  let open = false;
  let ws: WebSocket | null = null;

  function generateID(): string {
    return typeof crypto !== 'undefined' && crypto.randomUUID
      ? crypto.randomUUID()
      : `${Date.now()}-${Math.random()}`;
  }

  setTransport({
    call(objectID: number, method: number, windowName: string, args: unknown) {
      return new Promise((resolve, reject) => {
        const pendingCall: PendingCall = { objectID, method, windowName, args, resolve, reject };
        if (open) {
          sendCall(pendingCall);
        } else {
          queue.push(pendingCall);
        }
      });
    }
  });

  function sendCall(p: PendingCall) {
    const id = generateID();
    pending.set(id, p);
    ws?.send(JSON.stringify({
      type: 'call',
      id,
      object: p.objectID,
      method: p.method,
      windowName: p.windowName,
      args: p.args
    }));
  }

  function flushQueue() {
    while (queue.length) {
      const p = queue.shift();
      if (p) sendCall(p);
    }
  }

  function connect() {
    open = false;
    ws = new WebSocket(WS_URL);

    ws.addEventListener('open', () => {
      open = true;
      flushQueue();
    });

    ws.addEventListener('message', (event) => {
      let msg: any;
      try {
        msg = JSON.parse(event.data);
      } catch {
        return;
      }

      if (msg.type === 'call') {
        const p = pending.get(msg.id);
        if (!p) return;
        pending.delete(msg.id);
        if (msg.error) {
          p.reject(new Error(msg.error));
        } else {
          p.resolve(msg.result);
        }
      } else if (msg.type === 'event') {
        if (window._wails?.dispatchWailsEvent) {
          window._wails.dispatchWailsEvent({ name: msg.name, data: msg.data });
        }
      }
    });

    ws.addEventListener('close', () => {
      open = false;
      // Re-queue pending calls so they are retried after reconnect.
      for (const p of pending.values()) {
        queue.push(p);
      }
      pending.clear();
      setTimeout(connect, 1000);
    });

    ws.addEventListener('error', (err) => {
      console.warn('WebSocket IPC error:', err);
    });
  }

  connect();
}

export {};
