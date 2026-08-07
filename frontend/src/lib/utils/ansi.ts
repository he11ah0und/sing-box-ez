// ANSI SGR parser for colored log rendering.

export interface AnsiPart {
  text: string;
  style: string;
}

interface AnsiState {
  bold: boolean;
  fg: string;
  skip?: number;
}

const BASIC_COLORS = [
  '#000000', '#cd3131', '#0dbc79', '#e5e510', '#2472c8', '#bc3fbc', '#11a8cd', '#e5e5e5'
];
const BRIGHT_COLORS = [
  '#666666', '#f14c4c', '#23d18b', '#f5f543', '#3b8eea', '#d670d6', '#29b8db', '#e5e5e5'
];

function clamp(v: number, min: number, max: number): number {
  return Math.max(min, Math.min(max, v));
}

function ansi256(n: number): string {
  if (n < 16) return n < 8 ? BASIC_COLORS[n] : BRIGHT_COLORS[n - 8];
  if (n < 232) {
    const c = n - 16;
    const r = Math.floor(c / 36);
    const g = Math.floor((c % 36) / 6);
    const b = c % 6;
    const to8 = (x: number) => clamp(55 + x * 40, 0, 255);
    return `rgb(${to8(r)}, ${to8(g)}, ${to8(b)})`;
  }
  const gray = clamp(8 + (n - 232) * 10, 0, 255);
  return `rgb(${gray}, ${gray}, ${gray})`;
}

function applyCode(state: AnsiState, code: number, params: string[], i: number): AnsiState {
  if (code === 0) return { bold: false, fg: '' };
  if (code === 1) return { ...state, bold: true };
  if (code === 22) return { ...state, bold: false };
  if (code === 39) return { ...state, fg: '' };
  if (code >= 30 && code <= 37) return { ...state, fg: BASIC_COLORS[code - 30] };
  if (code >= 90 && code <= 97) return { ...state, fg: BRIGHT_COLORS[code - 90] };
  if (code === 38) {
    const mode = Number(params[i + 1]);
    if (mode === 5 && params[i + 2] !== undefined) {
      return { ...state, fg: ansi256(Number(params[i + 2])), skip: 2 };
    }
    if (mode === 2 && params[i + 4] !== undefined) {
      const r = Number(params[i + 2]);
      const g = Number(params[i + 3]);
      const b = Number(params[i + 4]);
      return { ...state, fg: `rgb(${r}, ${g}, ${b})`, skip: 4 };
    }
  }
  return state;
}

function styleFor(state: AnsiState): string {
  const parts: string[] = [];
  if (state.fg) parts.push(`color: ${state.fg}`);
  if (state.bold) parts.push('font-weight: 700');
  return parts.join('; ');
}

/**
 * Parse a string containing ANSI SGR escape sequences into styled spans.
 * Returns an array of { text, style } objects. Non-SGR sequences are stripped.
 */
export function parseANSILine(line: string): AnsiPart[] {
  const parts: AnsiPart[] = [];
  let state: AnsiState = { bold: false, fg: '' };
  let start = 0;

  for (let i = 0; i < line.length; i++) {
    if (line.charCodeAt(i) !== 0x1b) continue;

    if (start < i) {
      parts.push({ text: line.slice(start, i), style: styleFor(state) });
    }

    if (i + 1 >= line.length || line[i + 1] !== '[') {
      start = i + 1;
      continue;
    }

    let seqEnd = i + 2;
    while (seqEnd < line.length) {
      const b = line.charCodeAt(seqEnd);
      if (b >= 0x40 && b <= 0x7e) break;
      seqEnd++;
    }
    if (seqEnd >= line.length) break;

    const final = line[seqEnd];
    const seq = line.slice(i + 2, seqEnd);
    if (final === 'm') {
      const raw = seq.startsWith('[') ? seq.slice(1) : seq;
      const params = raw === '' ? [] : raw.split(';');
      for (let pi = 0; pi < params.length; pi++) {
        const code = Number(params[pi]);
        const next = applyCode(state, code, params, pi);
        if (next.skip) pi += next.skip;
        state = next;
      }
    }

    i = seqEnd;
    start = seqEnd + 1;
  }

  if (start < line.length) {
    parts.push({ text: line.slice(start), style: styleFor(state) });
  }
  if (parts.length === 0) {
    parts.push({ text: '', style: '' });
  }
  return parts;
}

const LEVEL_COLORS: Record<string, string> = {
  DBG: '#888888',
  INF: '#3b8eea',
  WRN: '#f59e0b',
  ERR: '#ef4444'
};

const CORE_LEVEL_COLORS: Record<string, string> = {
  DEBUG: '#888888',
  INFO: '#3b8eea',
  WARN: '#f59e0b',
  WARNING: '#f59e0b',
  ERROR: '#ef4444',
  ERR: '#ef4444',
  FATAL: '#ef4444'
};

/**
 * Colorize an app log line with the format [HH:MM:SS] [LEVEL] source -> message.
 */
export function parseAppLogLine(line: string): AnsiPart[] {
  const arrowIdx = line.indexOf(' -> ');
  if (arrowIdx < 0) {
    return [{ text: line, style: 'color: #f5f543' }];
  }
  const parts: AnsiPart[] = [];
  let header = line.slice(0, arrowIdx);
  const message = line.slice(arrowIdx + 4);

  // Timestamp: first ]
  const tsEnd = header.indexOf('] ');
  if (tsEnd >= 0) {
    parts.push({ text: header.slice(0, tsEnd + 1) + ' ', style: 'color: #888888' });
    header = header.slice(tsEnd + 2).trimStart();
  } else {
    parts.push({ text: header + ' ', style: 'color: #888888' });
    header = '';
  }

  // Level: next ]
  let levelColor = LEVEL_COLORS.DBG;
  const lvlEnd = header.indexOf('] ');
  if (lvlEnd >= 0) {
    const level = header.slice(0, lvlEnd + 1);
    levelColor = LEVEL_COLORS[level.replace(/[\[\]]/g, '')] || LEVEL_COLORS.DBG;
    parts.push({ text: level + ' ', style: `color: ${levelColor}` });
    header = header.slice(lvlEnd + 2).trimStart();
  } else if (header) {
    levelColor = LEVEL_COLORS[header.replace(/[\[\]]/g, '')] || LEVEL_COLORS.DBG;
    parts.push({ text: header + ' ', style: `color: ${levelColor}` });
    header = '';
  }

  // Source.
  if (header) {
    parts.push({ text: header + ' ', style: 'color: #29b8db' });
  }

  parts.push({ text: '-> ', style: 'color: #bc3fbc' });
  parts.push({ text: message, style: `color: ${levelColor}` });
  return parts;
}

/**
 * Colorize a raw core log line by highlighting the level token.
 */
export function parseCoreLogLine(line: string): AnsiPart[] {
  const levels = ['DEBUG', 'INFO', 'WARN', 'WARNING', 'ERROR', 'ERR', 'FATAL'];
  const lower = line.toLowerCase();
  for (const lvl of levels) {
    const target = lvl.toLowerCase();
    let idx = 0;
    while (idx < lower.length) {
      const pos = lower.indexOf(target, idx);
      if (pos < 0) break;
      const end = pos + target.length;
      const before = pos === 0 || !/\w/.test(lower[pos - 1]);
      const after = end >= lower.length || !/\w/.test(lower[end]);
      if (before && after) {
        const color = CORE_LEVEL_COLORS[lvl] || '#888888';
        const parts: AnsiPart[] = [];
        if (pos > 0) parts.push({ text: line.slice(0, pos), style: 'color: #888888' });
        parts.push({ text: line.slice(pos, end), style: `color: ${color}` });
        if (end < line.length) parts.push({ text: line.slice(end), style: 'color: #888888' });
        return parts;
      }
      idx = end;
    }
  }
  return [{ text: line, style: 'color: #888888' }];
}
