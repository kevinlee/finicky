export interface Rule {
  match: string[];
  browser: string;
  profile?: string;
  profileDirectory?: string;
  sourceApps?: string[];
  enabled?: boolean;
  private?: boolean;
  newWindow?: boolean;
  transformScript?: string;
  name?: string;
}

export interface SavedProfile {
  id: string;
  name: string;
  browser: string;
  profile?: string;
  profileDirectory?: string;
  color?: string;
  shortcut?: string;
  hidden?: boolean;
  private?: boolean;
}

export interface ConfigOptions {
  keepRunning: boolean;
  hideIcon: boolean;
  logRequests: boolean;
  checkForUpdates: boolean;
}

export interface Features {
  menuIcon: "default" | "browser" | "globe" | "arrow";
  promptMode: "never" | "always" | "unmatched";
  promptModifier: "fn" | "option" | "shift" | "control" | "command" | "none";
  stripTracking: boolean;
  unwrapRedirects: boolean;
  expandShortURLs: boolean;
  extraShorteners: string[];
  historyEnabled: boolean;
  nativeApps: boolean;
  nativeAppServices?: string[] | null;
  transformScript?: string;
  alternativePrivate?: boolean;
  alternativeBrowser?: string;
  alternativeProfile?: string;
  alternativeProfileDirectory?: string;
}

export interface HistoryEntry {
  id: string;
  url: string;
  originalUrl?: string;
  browser?: string;
  profile?: string;
  profileDirectory?: string;
  sourceApp?: string;
  matchedRule?: number;
  timestamp: string;
  error?: string;
}

export interface IntegrationInfo {
  appPath: string;
  bundleId: string;
  scheme: string;
}

export interface RulesFile {
  defaultPrivate?: boolean;
  defaultBrowser: string;
  defaultProfile?: string;
  defaultProfileDirectory?: string;
  profiles?: SavedProfile[];
  options?: Partial<ConfigOptions>;
  features?: Partial<Features>;
  rules: Rule[];
  path?: string;
}

export interface LogEntry {
  level: string;
  msg: string;
  time: string;
  error?: string;
  [key: string]: any; // Allow for additional dynamic fields
}

declare global {
  interface Window {
    finickyPreview?: boolean;
    finicky: {
      sendMessage: (msg: any) => void;
      receiveMessage: (msg: any) => void;
      /** Stub queue populated by the WKUserScript before the Svelte app is ready */
      _queue?: any[];
    };
    webkit?: {
      messageHandlers?: {
        finicky?: {
          postMessage: (msg: string) => void;
        };
      };
    };
  }
}

export interface UpdateInfo {
  version: string;
  hasUpdate: boolean;
  updateCheckEnabled: boolean;
  downloadUrl: string;
  releaseUrl: string;
}

export interface ConfigInfo {
  configPath: string;
  isJSConfig?: boolean;
  handlers?: number;
  rewrites?: number;
  defaultBrowser?: string;
  options?: Partial<ConfigOptions>;
}
