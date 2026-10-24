import { featureDefaults } from "./featuresModel";
import { normalizeRulesFile } from "./rulesModel";
import type { HistoryEntry } from "../types";
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
        case "clearHistory":
          history = [];
          send("historyCleared", { success: true });
          break;
        case "grantBrowserAccess":
          send("browserAccessResult", { success: true });
          break;
        case "getIntegrations":
          send("integrations", {
            appPath: "/Applications/Finicky Plus.app",
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
                  "URL transformation scripts run in the native app. Open Finicky Plus to test this configuration.",
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
