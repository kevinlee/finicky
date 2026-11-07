import { featureDefaults } from "./featuresModel";
import { normalizeRulesFile } from "./rulesModel";
import { syncBindingError } from "./syncModel";
import type { HistoryEntry, LearningObservation, LearningSnapshot, SyncBinding, SyncSnapshot, SyncStatus } from "../types";
// A browser-only development bridge. Native builds never run this branch.
// All data is illustrative; saving here changes localStorage, not Finicky.
if (import.meta.env.DEV && !window.webkit?.messageHandlers?.finicky) {
  const previewWindow = window as Window & { finickyPreview?: boolean };
  previewWindow.finickyPreview = true;
  const storageKey = "finicky-plus-preview-v2";
  const defaults = {
    defaultBrowser: "Google Chrome",
    profiles: [
      {
        id: "personal",
        name: "Personal",
        browser: "Google Chrome",
        profileDirectory: "Default",
        color: "#39836b",
        shortcut: "p",
      },
      {
        id: "work",
        name: "Work",
        browser: "Google Chrome",
        profileDirectory: "Profile 2",
        color: "#4569db",
        shortcut: "w",
      },
      {
        id: "academic",
        name: "Academic",
        browser: "Google Chrome",
        profileDirectory: "Profile 3",
        color: "#a4454b",
        shortcut: "a",
      },
    ],
    rules: [
      ...[
        ["Slack", "com.tinyspeck.slackmacgap"],
        ["ChatGPT", "com.openai.codex"],
        ["Claude", "com.anthropic.claudefordesktop"],
        ["Cursor", "com.todesktop.230313mzl4w4u92"],
      ].map(([name, app]) => ({
        name: `${name} links`,
        sourceApps: [app],
        match: [],
        browser: "Google Chrome",
        profileDirectory: "Profile 2",
        enabled: true,
      })),
      {
        name: "Academic websites",
        sourceApps: [],
        match: ["*example.edu/*"],
        browser: "Google Chrome",
        profileDirectory: "Profile 3",
        enabled: false,
      },
    ],
    features: { ...featureDefaults },
    options: {
      keepRunning: true,
      hideIcon: false,
      logRequests: false,
      checkForUpdates: true,
    },
  };
  let rules: any = structuredClone(defaults);
  try {
    rules = JSON.parse(localStorage.getItem(storageKey) || "null") || rules;
  } catch {
    /* First launch. */
  }
  // Optional local preview scenarios exercise the UI without accessing iCloud.
  const syncScenario = new URLSearchParams(window.location.search).get("syncPreview");
  let previewSyncEnabled = Boolean(syncScenario);
  let previewSyncState: SyncStatus = syncScenario === "conflict" ? "conflict"
    : syncScenario === "offline" ? "offline" : syncScenario === "pending" ? "pending"
      : syncScenario ? "awaitingMapping" : "disabled";
  let previewBindings: Record<string, SyncBinding> = {};
  let previewSyncedAt = "";
  const sharedProfiles = defaults.profiles.map((profile) => ({ id: profile.id, name: profile.name, color: profile.color }));
  const syncSnapshot = (): SyncSnapshot => ({
    enabled: previewSyncEnabled,
    state: previewSyncEnabled ? previewSyncState : "disabled",
    message: previewSyncState === "conflict" ? "The example shared settings changed on another Mac."
      : previewSyncState === "offline" ? "This preview simulates iCloud being unavailable."
        : previewSyncState === "awaitingMapping" ? "Choose where shared profiles open on this Mac."
          : "Development preview. No iCloud files are read or written.",
    profiles: sharedProfiles.map((profile) => ({ ...profile, mapped: Boolean(previewBindings[profile.id]) })),
    bindings: structuredClone(previewBindings),
    localProfiles: structuredClone(rules.profiles || []),
    missingProfileIds: sharedProfiles.filter((profile) => !previewBindings[profile.id]).map((profile) => profile.id),
    heads: previewSyncState === "conflict" ? ["preview-local", "preview-remote"] : [],
    headDetails: previewSyncState === "conflict" ? [
      { id: "preview-local", deviceId: "preview-mac-a", deviceName: "Example MacBook", createdAt: new Date(Date.now() - 60_000).toISOString() },
      { id: "preview-remote", deviceId: "preview-mac-b", deviceName: "Example iMac", createdAt: new Date().toISOString() },
    ] : [],
    cloudPath: "iCloud Drive / Finicky Plus Sync (preview)",
    lastSyncedAt: previewSyncedAt || undefined,
    limitations: ["This preview uses synthetic shared settings and never contacts iCloud."],
  });
  let history: HistoryEntry[] = [
    {
      id: "preview-1",
      url: "https://example.com/meeting",
      originalUrl: "https://example.com/meeting?utm_source=email",
      browser: "Google Chrome",
      profileDirectory: "Profile 2",
      sourceApp: "com.tinyspeck.slackmacgap",
      timestamp: new Date().toISOString(),
    },
    {
      id: "preview-2",
      url: "https://example.edu/",
      browser: "Google Chrome",
      profileDirectory: "Profile 3",
      timestamp: new Date(Date.now() - 3_600_000).toISOString(),
    },
  ];
  // Synthetic examples only; browsing activity is never read by this preview.
  let learningObservations: LearningObservation[] = [
    {
      id: "learning-preview-1",
      domain: "example.com",
      sourceApp: "Google Chrome",
      observedAt: new Date().toISOString(),
      confirmedDestinationId: "work",
      confirmedDestinationName: "Work",
      predictedDestinationId: "work",
      predictedDestinationName: "Work",
      feedbackKind: "chooser",
    },
    {
      id: "learning-preview-2",
      domain: "example.edu",
      sourceApp: "Google Chrome",
      observedAt: new Date(Date.now() - 3_600_000).toISOString(),
      confirmedDestinationId: "academic",
      confirmedDestinationName: "Academic",
      feedbackKind: "chooser",
    },
    {
      id: "learning-preview-3",
      domain: "example.org",
      sourceApp: "Google Chrome",
      observedAt: new Date(Date.now() - 7_200_000).toISOString(),
    },
  ];
  const initialLearningChoices = new Map(learningObservations.map((entry) => [entry.id, entry.confirmedDestinationId]));
  const learningSnapshot = (): LearningSnapshot => {
    const labeled = learningObservations.filter((entry) => entry.confirmedDestinationId);
    const predicted = labeled.filter((entry) => entry.predictedDestinationId);
    return {
      enabled: Boolean(rules.features?.learningEnabled),
      observations: learningObservations,
      feedbackCount: labeled.length,
      destinations: (rules.profiles || [])
        .filter((profile: any) => !profile.hidden && !profile.private)
        .map((profile: any) => ({ id: profile.id, name: profile.name })),
      evaluation: {
        evaluated: labeled.length,
        predicted: predicted.length,
        correct: predicted.filter((entry) => entry.predictedDestinationId === initialLearningChoices.get(entry.id)).length,
        abstained: labeled.length - predicted.length,
      },
      limitations: ["Development preview: these examples are illustrative and no links are collected."],
    };
  };
  const send = (type: string, message: unknown) =>
    window.finicky.receiveMessage({ type, message });
  // Mirror Go's wire format, including single-string and null match values.
  // This keeps the browser preview useful for native bridge regression checks.
  const rulesMessage = () => ({
    ...rules,
    path: "Preview / rules.json",
    rules: rules.rules.map((rule: any) => ({
      ...rule,
      match:
        rule.match?.length === 1
          ? rule.match[0]
          : rule.match?.length
            ? rule.match
            : null,
    })),
  });
  const receive = (raw: string) => {
    const msg = JSON.parse(raw);
    queueMicrotask(() => {
      switch (msg.type) {
        case "getSync":
          send("sync", syncSnapshot());
          break;
        case "enableSync":
          previewSyncEnabled = msg.enabled === true;
          previewSyncState = previewSyncEnabled
            ? sharedProfiles.some((profile) => !previewBindings[profile.id]) ? "awaitingMapping" : "synced"
            : "disabled";
          send("syncResult", { success: true, operation: msg.type });
          send("sync", syncSnapshot());
          break;
        case "saveSyncBindings":
          if (sharedProfiles.some((profile) => syncBindingError(msg.bindings?.[profile.id]))) {
            send("syncResult", { success: false, operation: msg.type, error: "Choose a valid destination for every shared profile." });
            break;
          }
          previewBindings = structuredClone(msg.bindings);
          if (previewSyncState !== "conflict") previewSyncState = previewSyncEnabled ? "synced" : "disabled";
          send("sync", syncSnapshot());
          send("syncResult", { success: true, operation: msg.type });
          break;
        case "syncNow":
          if (previewSyncState === "offline") {
            send("syncResult", { success: false, operation: msg.type, error: "iCloud is unavailable in this preview scenario. Your local settings are unchanged." });
            break;
          }
          previewSyncState = sharedProfiles.some((profile) => !previewBindings[profile.id]) ? "awaitingMapping" : "synced";
          previewSyncedAt = new Date().toISOString();
          send("syncResult", { success: true, operation: msg.type });
          send("sync", syncSnapshot());
          break;
        case "resolveSync":
          if (previewSyncState !== "conflict" || !["local", "remote"].includes(msg.choice) || (msg.choice === "remote" && !syncSnapshot().heads.includes(msg.revision))) {
            send("syncResult", { success: false, operation: msg.type, error: "No matching conflict is waiting for this choice." });
            break;
          }
          previewSyncState = sharedProfiles.some((profile) => !previewBindings[profile.id]) ? "awaitingMapping" : "synced";
          send("syncResult", { success: true, operation: msg.type });
          send("sync", syncSnapshot());
          break;
        case "getRules":
          send("rules", rulesMessage());
          break;
        case "getFeatures":
          send("features", { ...featureDefaults, ...rules.features });
          break;
        case "saveFeatures":
          rules.features = msg.features;
          localStorage.setItem(storageKey, JSON.stringify(rules));
          send("featuresSaved", { success: true });
          break;
        case "getHistory":
          send("history", history);
          break;
        case "getLearning":
          send("learning", learningSnapshot());
          break;
        case "setLearningEnabled": {
          rules.features = { ...rules.features, learningEnabled: msg.enabled === true };
          localStorage.setItem(storageKey, JSON.stringify(rules));
          // Native saveAndApply emits no display path and serializes nil services as null.
          const { path: _path, ...savedRules } = rulesMessage();
          send("rules", { ...savedRules, features: { ...rules.features, nativeAppServices: rules.features.nativeAppServices ?? null } });
          send("features", rules.features);
          send("learningResult", { success: true, operation: msg.type });
          send("learning", learningSnapshot());
          break;
        }
        case "clearLearning":
          learningObservations = [];
          initialLearningChoices.clear();
          send("learningResult", { success: true, operation: msg.type });
          send("learning", learningSnapshot());
          break;
        case "deleteLearning":
          learningObservations = learningObservations.filter((entry) => entry.id !== msg.requestId);
          initialLearningChoices.delete(msg.requestId);
          send("learningResult", { success: true, operation: msg.type });
          send("learning", learningSnapshot());
          break;
        case "correctLearning": {
          const entry = learningObservations.find((entry) => entry.id === msg.requestId);
          const destination = learningSnapshot().destinations.find((entry) => entry.id === msg.destinationId);
          if (!entry?.confirmedDestinationId || !destination) {
            send("learningResult", { success: false, operation: msg.type, error: "This example or destination is no longer available." });
            break;
          }
          entry.confirmedDestinationId = destination.id;
          entry.confirmedDestinationName = destination.name;
          entry.feedbackKind = "correction";
          send("learningResult", { success: true, operation: msg.type });
          send("learning", learningSnapshot());
          break;
        }
        case "clearHistory":
          history = [];
          send("historyCleared", { success: true });
          break;
        case "grantBrowserAccess":
          send("browserAccessResult", { success: true });
          break;
        case "getIntegrations":
          send("integrations", {
            appPath: "/Applications/Waystile.app",
            bundleId: "com.kevinlee.finicky",
            scheme: "finicky-plus",
          });
          break;
        case "openPicker":
          send("openPickerResult", { success: true });
          break;
        case "openClipboard":
          send("openClipboardResult", { success: true });
          break;
        case "exportSettings":
          send("settingsExport", { json: JSON.stringify(rules, null, 2) });
          break;
        case "importSettings":
          try {
            const imported = JSON.parse(msg.json);
            if (!Array.isArray(imported.rules))
              throw new Error("Settings must contain a rules array.");
            rules = normalizeRulesFile(imported);
            localStorage.setItem(storageKey, JSON.stringify(rules));
            send("rules", rulesMessage());
            send("settingsImported", { success: true });
          } catch (error) {
            send("settingsImported", { success: false, error: String(error) });
          }
          break;
        case "saveRules":
          rules = structuredClone(msg.payload);
          localStorage.setItem(storageKey, JSON.stringify(rules));
          send("rules", rulesMessage());
          send("saveRulesSuccess", { success: true });
          break;
        case "getInstalledBrowsers":
          send("installedBrowsers", ["Google Chrome", "Safari", "Firefox"]);
          break;
        case "getBrowserProfiles":
          send("browserProfiles", {
            browser: msg.browser,
            profileDetails:
              msg.browser === "Google Chrome"
                ? [
                    {
                      id: "Default",
                      name: "Personal",
                      profile: "Personal",
                      profileDirectory: "Default",
                    },
                    {
                      id: "Profile 2",
                      name: "Work",
                      profile: "Work",
                      profileDirectory: "Profile 2",
                    },
                    {
                      id: "Profile 3",
                      name: "Work",
                      profile: "Work",
                      profileDirectory: "Profile 3",
                    },
                  ]
                : [],
            profiles:
              msg.browser === "Google Chrome"
                ? ["Personal", "Work", "Work"]
                : [],
          });
          break;
        case "testUrl": {
          try {
            const url = new URL(msg.url);
            const found = rules.rules.find((rule: any) => {
              if (rule.enabled === false) return false;
              if (
                rule.sourceApps?.length &&
                !rule.sourceApps.includes(msg.openerBundleId || "")
              )
                return false;
              if (!rule.match?.length) return !!rule.sourceApps?.length;
              return rule.match.some((pattern: string) => {
                const expression = pattern
                  .replace(/[.+?^${}()|[\]\\]/g, "\\$&")
                  .replace(/\*/g, ".*");
                return (
                  new RegExp(`^${expression}$`, "i").test(url.href) ||
                  new RegExp(`^${expression}$`, "i").test(
                    url.href.replace(/^https?:\/\//, ""),
                  )
                );
              });
            });
            if (
              rules.features?.transformScript?.trim() ||
              found?.transformScript?.trim()
            ) {
              send("testUrlResult", {
                error:
                  "URL transformation scripts run in the native app. Open Waystile to test this configuration.",
              });
              break;
            }
            const destination = found
              ? found.browser
              : rules.defaultBrowser || "Safari";
            send("testUrlResult", {
              action:
                destination === "prompt" ||
                rules.features?.promptMode === "always" ||
                (!found && rules.features?.promptMode === "unmatched")
                  ? "prompt"
                  : "open",
              browser: found ? found.browser : rules.defaultBrowser || "Safari",
              profile: found ? found.profile || "" : rules.defaultProfile || "",
              profileDirectory: found
                ? found.profileDirectory || ""
                : rules.defaultProfileDirectory || "",
              openInBackground: false,
              url: url.href,
            });
          } catch {
            send("testUrlResult", { error: "Enter a valid web address." });
          }
          break;
        }
      }
    });
  };
  window.finicky = {
    _queue: [],
    receiveMessage(msg) {
      this._queue?.push(msg);
    },
    sendMessage() {},
  };
  window.webkit = { messageHandlers: { finicky: { postMessage: receive } } };
  window.finicky._queue?.push(
    { type: "version", message: "Development preview" },
    {
      type: "config",
      message: {
        configPath: "Preview / rules.json",
        isJSConfig: false,
        options: rules.options,
      },
    },
  );
}

export {};
