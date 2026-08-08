export function formatBytes(b: unknown): string {
  const n = Number(b) || 0;
  if (n === 0) return '0 B';
  if (n < 1024) return `${n} B`;
  const units = ['kB', 'MB', 'GB', 'TB'];
  let f = n / 1024;
  let i = 0;
  while (f >= 1024 && i < units.length - 1) {
    f /= 1024;
    i++;
  }
  return `${f.toFixed(1)} ${units[i]}`;
}

export function formatSpeed(v: unknown): string {
  const n = Number(v) || 0;
  if (n === 0) return '0 B/s';
  if (n < 1024) return `${Math.round(n)} B/s`;
  const units = ['kB/s', 'MB/s', 'GB/s'];
  let f = n / 1024;
  let i = 0;
  while (f >= 1024 && i < units.length - 1) {
    f /= 1024;
    i++;
  }
  return `${f.toFixed(1)} ${units[i]}`;
}

export function formatTime(date: string | number | Date | null | undefined): string {
  if (!date) return '';
  const d = new Date(date);
  return d.toLocaleString();
}

export interface HostPort {
  host: string;
  port: string;
  isIP: boolean;
}

export function splitHostPort(addr: string | null | undefined): HostPort {
  if (!addr) return { host: '', port: '', isIP: false };
  // IPv6 bracketed form
  const bracketMatch = addr.match(/^\[(.+)]:(\d+)$/);
  if (bracketMatch) {
    const host = bracketMatch[1];
    return { host, port: bracketMatch[2], isIP: isIP(host) };
  }
  // bare IPv6 with trailing port (e.g. 2606:4700::1:443)
  const lastColon = addr.lastIndexOf(':');
  if (lastColon > 0) {
    const hostPart = addr.slice(0, lastColon);
    const portPart = addr.slice(lastColon + 1);
    if (/^\d+$/.test(portPart) && portPart.length <= 5) {
      return { host: hostPart, port: portPart, isIP: isIP(hostPart) };
    }
  }
  if (isIP(addr)) return { host: addr, port: '', isIP: true };
  return { host: addr, port: '', isIP: false };
}

function isIP(s: string): boolean {
  if (!s) return false;
  // very rough IPv4/v6 check
  return /^\d{1,3}(\.\d{1,3}){3}$/.test(s) || /^[0-9a-fA-F:]+$/.test(s);
}

export function ipVersionLabel(destination: string | null | undefined): string {
  const { host, isIP: isIp } = splitHostPort(destination);
  if (!isIp || !host) return '';
  if (/^\d{1,3}(\.\d{1,3}){3}$/.test(host)) return 'IPv4';
  return 'IPv6';
}
