// Type shims for generated Wails bindings (frontend/bindings/ is gitignored JS,
// excluded from tsconfig). Extend as more bindings are consumed from TS code.
declare module '*/bindings/sing-box-ez/internal/gui/wails/bindings.js' {
  import type { ThemeData } from '$lib/stores/theme.js';
  import type { ConfigRecord } from '$lib/stores/appState.js';

  export interface Settings {
    language: string;
    theme: string;
    themeMode: string;
    autoStartCore: boolean;
    autoRestart: boolean;
    runAsAdmin: boolean;
    logLimit: number;
    desktopNotifications: boolean;
    autoCheckCore: boolean;
    autoCheckSelf: boolean;
    defaultIntervalHours: number;
  }

  export interface Status {
    running: boolean;
    pid: number;
  }

  export interface CoreInfoPayload {
    installedVersion: string;
    latestVersion: string;
    downloading: boolean;
    downloadProgress: number;
  }

  export interface LanguageOption {
    code: string;
    name: string;
  }

  export interface UpdateChannel {
    id: string;
    name: string;
  }

  export interface SelfUpdateInfo {
    current: string;
    latest: string;
    hasUpdate: boolean;
    isDevBuild: boolean;
    body: string;
    latestDate: string;
    releaseCount: number;
  }

  export interface VersionInfo {
    branch: string;
    commit: string;
    commitDate: string;
    buildDate: string;
    buildFlags: string;
    isDev: boolean;
    humanCommit: string;
    humanBuild: string;
  }

  export interface APIStatus {
    version: string;
    uptime: string;
    memory: number;
    goroutines: number;
    connectionsIn: number;
    connectionsOut: number;
    trafficAvailable: boolean;
    uplink: number;
    downlink: number;
    uplinkTotal: number;
    downlinkTotal: number;
  }

  export interface APIInfo {
    backend: string;
    host: string;
    port: number;
    addr: string;
  }

  export interface APINode {
    tag: string;
    type: string;
    delay: number;
    delayValid: boolean;
    delayAt: string;
  }

  export interface APIGroup {
    tag: string;
    type: string;
    selected: string;
    nodes: APINode[] | null;
    delay: number;
    delayValid: boolean;
  }

  export interface APIProcessInfo {
    processID: number;
    userID: number;
    userName: string;
    processPath: string;
    packageNames: string[] | null;
  }

  export interface APIConnection {
    id: string;
    inbound: string;
    inboundType: string;
    network: string;
    source: string;
    destination: string;
    domain: string;
    protocol: string;
    user: string;
    outbound: string;
    outboundType: string;
    chain: string[] | null;
    uplink: number;
    downlink: number;
    uplinkTotal: number;
    downlinkTotal: number;
    rule: string;
    createdAt: string;
    closedAt: string;
    processInfo: APIProcessInfo;
    metadata: Record<string, unknown> | null;
  }

  export interface URLTestResult {
    results: Record<string, number> | null;
    average: number;
    count: number;
  }

  export interface LocalePayload {
    language: string;
    values: Record<string, string> | null;
  }

  export function ActivateConfig(name: string): Promise<void>;
  export function AddConfig(rec: ConfigRecord): Promise<void>;
  export function CheckSelfUpdate(branch: string): Promise<SelfUpdateInfo>;
  export function ClearAppLogs(): Promise<void>;
  export function ClearCoreLogs(): Promise<void>;
  export function CloseAPIConnection(id: string): Promise<void>;
  export function CloseAPIConnections(): Promise<void>;
  export function DeleteConfig(name: string): Promise<void>;
  export function DownloadCore(): Promise<void>;
  export function EditConfig(oldName: string, rec: ConfigRecord): Promise<void>;
  export function GetAPIConnections(): Promise<APIConnection[] | null>;
  export function GetAPIGroups(): Promise<APIGroup[] | null>;
  export function GetAPIInfo(): Promise<APIInfo | null>;
  export function GetAPIMode(): Promise<string>;
  export function GetAPIStatus(): Promise<APIStatus>;
  export function GetActiveConfig(): Promise<ConfigRecord | null>;
  export function GetAppLogs(): Promise<string[] | null>;
  export function GetAvailableLanguages(): Promise<LanguageOption[] | null>;
  export function GetBranches(): Promise<UpdateChannel[] | null>;
  export function GetConfigs(): Promise<ConfigRecord[] | null>;
  export function GetCoreInfo(): Promise<CoreInfoPayload>;
  export function GetCoreLogs(): Promise<string[] | null>;
  export function GetReleaseNotes(ver: string): Promise<string>;
  export function GetSettings(): Promise<Settings>;
  export function GetStatus(): Promise<Status>;
  export function GetTheme(): Promise<ThemeData | null>;
  export function GetThemeNames(): Promise<string[] | null>;
  export function GetVersionInfo(): Promise<VersionInfo>;
  export function InstallSelfUpdate(branch: string): Promise<void>;
  export function LocaleReady(): Promise<void>;
  export function OpenDataDir(): Promise<void>;
  export function OpenURL(url: string): Promise<void>;
  export function RegisterLocaleKeys(keys: string[]): Promise<Record<string, string> | null>;
  export function Restart(): Promise<void>;
  export function SaveSettings(s: Settings): Promise<void>;
  export function SelectAPINode(group: string, node: string): Promise<void>;
  export function SetAPIMode(mode: string): Promise<void>;
  export function SetLanguage(code: string): Promise<LocalePayload>;
  export function Start(): Promise<void>;
  export function Stop(): Promise<void>;
  export function URLTestAPIGroup(group: string): Promise<URLTestResult>;
}
