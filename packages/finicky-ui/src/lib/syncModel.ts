import type { SavedProfile, SyncBinding, SyncSnapshot, SyncStatus } from "../types";

const record = (value: unknown): value is Record<string, unknown> =>
  value !== null && typeof value === "object" && !Array.isArray(value);
const text = (value: unknown): string | undefined =>
  typeof value === "string" && value.trim() ? value.trim() : undefined;
const strings = (value: unknown): string[] => Array.isArray(value)
  ? value.filter((item): item is string => typeof item === "string") : [];

export const syncStatusLabels: Record<SyncStatus, string> = {
  disabled: "Off", synced: "Up to date", pending: "Waiting for iCloud",
  offline: "iCloud unavailable", conflict: "Choose which settings to keep",
  awaitingMapping: "Match profiles on this Mac", localChanges: "Changes waiting to sync", error: "Needs attention",
};

export function normalizeSyncSnapshot(value: unknown): SyncSnapshot | null {
  if (!record(value) || typeof value.enabled !== "boolean" ||
      typeof value.state !== "string" || !Object.prototype.hasOwnProperty.call(syncStatusLabels, value.state) ||
      !Array.isArray(value.profiles) || !Array.isArray(value.localProfiles) || !record(value.bindings)) return null;
  const profiles = value.profiles.filter(record).filter((entry) => text(entry.id) && text(entry.name))
    .map((entry) => ({ id: String(entry.id), name: String(entry.name), color: text(entry.color), private: entry.private === true, hidden: entry.hidden === true, mapped: entry.mapped === true }));
  if (new Set(profiles.map((profile) => profile.id)).size !== profiles.length) return null;
  const bindings = Object.fromEntries(Object.entries(value.bindings).filter(([, binding]) => record(binding) && text(binding.browser))
    .map(([id, value]) => {
      const binding = value as Record<string, unknown>;
      return [id, { browser: String(binding.browser), ...(text(binding.profile) ? { profile: String(binding.profile) } : {}), ...(text(binding.profileDirectory) ? { profileDirectory: String(binding.profileDirectory) } : {}) }];
    }));
  const localProfiles: SavedProfile[] = value.localProfiles.filter(record)
    .filter((entry) => text(entry.id) && text(entry.name) && text(entry.browser))
    .map((entry) => ({ id: String(entry.id), name: String(entry.name), browser: String(entry.browser), profile: text(entry.profile), profileDirectory: text(entry.profileDirectory), private: entry.private === true, hidden: entry.hidden === true }));
  return {
    enabled: value.enabled, state: value.state as SyncStatus, profiles, bindings, localProfiles,
    message: text(value.message), deviceId: text(value.deviceId), appliedRevision: text(value.appliedRevision),
    heads: strings(value.heads), missingProfileIds: strings(value.missingProfileIds),
    headDetails: Array.isArray(value.headDetails) ? value.headDetails.filter(record).filter((head) => text(head.id)).map((head) => ({ id: String(head.id), deviceId: text(head.deviceId), deviceName: text(head.deviceName), createdAt: text(head.createdAt) })) : [],
    cloudPath: text(value.cloudPath), blockedReason: text(value.blockedReason),
    lastSyncedAt: text(value.lastSyncedAt), limitations: strings(value.limitations),
  };
}

export function isChromiumBrowser(browser: string): boolean {
  return /chrome|chromium|brave|edgemac|microsoft edge|vivaldi|opera|arc|company\.thebrowser/i.test(browser);
}

// Browser names are edited character by character. Keep both explicit profile
// values in the draft so intermediate text cannot erase the user's destination.
export function updateSyncBindingDraft(binding: SyncBinding | undefined, patch: Partial<SyncBinding>): SyncBinding {
  return { browser: "", ...binding, ...patch };
}

export function syncBindingForSave(binding: SyncBinding, manual = false): SyncBinding {
  const browser = binding.browser.trim();
  const directory = binding.profileDirectory?.trim();
  const profile = binding.profile?.trim();
  if (manual && !isChromiumBrowser(browser)) {
    return { browser, ...(profile ? { profile } : {}) };
  }
  return { browser, ...(directory ? { profileDirectory: directory } : !manual && profile ? { profile } : {}) };
}

export function syncBindingError(binding?: SyncBinding): string {
  if (!binding?.browser.trim()) return "Choose a local destination.";
  if (binding.browser === "prompt") return "Choose a browser instead of the picker.";
  if (/[\x00-\x1f]/.test(binding.browser)) return "Enter a valid browser name.";
  const directory = binding.profileDirectory?.trim() || "";
  if (directory && (directory === "." || directory === ".." || /[/\\\x00-\x1f]/.test(directory) || directory.length > 200))
    return "Use one Chrome folder name, such as Default or Profile 2, without a path.";
  if (isChromiumBrowser(binding.browser) && !directory)
    return "Enter this Mac’s profile directory so links open the intended profile.";
  if (binding.profile && (/[/\\\x00-\x1f]/.test(binding.profile) || binding.profile.length > 200)) return "Enter a profile name, not a folder path.";
  return "";
}

export function syncMappingErrors(snapshot: SyncSnapshot, bindings: Record<string, SyncBinding>): Record<string, string> {
  return Object.fromEntries(snapshot.profiles.map((profile) => [profile.id, syncBindingError(bindings[profile.id])]).filter(([, error]) => error));
}

export function bindingForFavorite(profile: SavedProfile): SyncBinding {
  return { browser: profile.browser, ...(profile.profileDirectory ? { profileDirectory: profile.profileDirectory } : profile.profile ? { profile: profile.profile } : {}) };
}

export function equalBindings(a: Record<string, SyncBinding>, b: Record<string, SyncBinding>): boolean {
  const aliases: Record<string, string> = {
    "Google Chrome": "com.google.Chrome", "Google Chrome Canary": "com.google.Chrome.canary",
    Chromium: "org.chromium.Chromium", Safari: "com.apple.Safari",
    "Safari Technology Preview": "com.apple.SafariTechnologyPreview", Firefox: "org.mozilla.firefox",
    "Microsoft Edge": "com.microsoft.edgemac", "Brave Browser": "com.brave.Browser",
    Arc: "company.thebrowser.Browser", Vivaldi: "com.vivaldi.Vivaldi", Opera: "com.operasoftware.Opera",
  };
  const canonical = (bindings: Record<string, SyncBinding>) => JSON.stringify(Object.keys(bindings).sort().map((id) => {
    const binding = bindings[id];
    const browser = binding.browser.trim();
    const directory = binding.profileDirectory?.trim() || "";
    return [id, aliases[browser] || browser, directory ? "" : binding.profile?.trim() || "", directory];
  }));
  return canonical(a) === canonical(b);
}

export function reconcileSyncMappings(
  draft: Record<string, SyncBinding>,
  baseline: Record<string, SyncBinding>,
  incoming: Record<string, SyncBinding>,
): { draft: Record<string, SyncBinding>; baseline: Record<string, SyncBinding>; conflict: boolean } {
  if (equalBindings(draft, baseline) || equalBindings(draft, incoming))
    return { draft: structuredClone(incoming), baseline: structuredClone(incoming), conflict: false };
  return { draft, baseline, conflict: !equalBindings(baseline, incoming) };
}

export function syncResolution(snapshot: SyncSnapshot, choice: unknown, revision?: string): { type: "resolveSync"; choice: "local" | "remote"; revision?: string } | null {
  if (!snapshot.enabled || snapshot.state !== "conflict") return null;
  if (choice === "local") return { type: "resolveSync", choice };
  if (choice !== "remote") return null;
  const selected = snapshot.heads.length === 1 ? snapshot.heads[0] : revision;
  return selected && snapshot.heads.includes(selected) ? { type: "resolveSync", choice, revision: selected } : null;
}

export interface SyncViewState {
  snapshot: SyncSnapshot | null;
  pending: string;
  sawSnapshot: boolean;
  resultSucceeded: boolean;
  error: string;
}

export const emptySyncView = (): SyncViewState => ({ snapshot: null, pending: "", sawSnapshot: false, resultSucceeded: false, error: "" });
export const beginSyncRequest = (view: SyncViewState, operation: string): SyncViewState => ({ ...view, pending: operation, sawSnapshot: false, resultSucceeded: false });

export function receiveSyncSnapshot(view: SyncViewState, raw: unknown): SyncViewState {
  const snapshot = normalizeSyncSnapshot(raw);
  if (!snapshot) return { ...view, pending: "", error: "Waystile returned incomplete sync information. Refresh to try again." };
  const complete = view.pending === "getSync" || Boolean(view.pending && view.resultSucceeded);
  return { ...view, snapshot, sawSnapshot: true, pending: complete ? "" : view.pending, error: complete ? "" : view.error };
}

export function receiveSyncResult(view: SyncViewState, raw: unknown): SyncViewState {
  if (!record(raw) || !view.pending || (text(raw.operation) && raw.operation !== view.pending)) return view;
  if (raw.success !== true) return { ...view, pending: "", error: text(raw.error) || "The sync request could not be completed." };
  return { ...view, resultSucceeded: true, pending: view.sawSnapshot ? "" : view.pending, error: view.sawSnapshot ? "" : view.error };
}

export function syncMutationBlocked(input: { loaded: boolean; dirty: boolean; saving: boolean; importing: boolean; externalChange: boolean; otherBusy: boolean }): boolean {
  return !input.loaded || input.dirty || input.saving || input.importing || input.externalChange || input.otherBusy;
}
