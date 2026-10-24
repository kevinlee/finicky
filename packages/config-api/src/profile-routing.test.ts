import { describe, expect, it } from "vitest";
import { openUrl } from "./index";

const slack = { name: "Slack", bundleId: "com.tinyspeck.slackmacgap", path: "/Applications/Slack.app" };
const cursor = { name: "Cursor", bundleId: "com.todesktop.230313mzl4w4u92", path: "/Applications/Cursor.app" };
const work = { name: "Google Chrome", profile: "Paramount", profileDirectory: "Profile 2" };

describe("app and profile routing", () => {
  it("requires both app and URL conditions, and handles unknown sources", () => {
    const config = {
      defaultBrowser: "Safari",
      handlers: [{ match: "example.com/*", sourceApps: [slack.bundleId, cursor.bundleId], browser: work }],
    };
    expect(openUrl("https://example.com/a", slack, null, config).browser).toMatchObject(work);
    expect(openUrl("https://example.com/a", cursor, null, config).browser).toMatchObject(work);
    expect(openUrl("https://elsewhere.com", slack, null, config).browser.name).toBe("Safari");
    expect(openUrl("https://example.com/a", null, null, config).browser.name).toBe("Safari");
    expect(openUrl("https://example.com/a", { ...slack, bundleId: "other.app" }, null, config).browser.name).toBe("Safari");
  });

  it("skips disabled rules and retains first-match ordering", () => {
    const config = {
      defaultBrowser: "Safari",
      handlers: [
        { match: "*", sourceApps: [slack.bundleId], browser: "Firefox", enabled: false },
        { match: "*", sourceApps: [slack.bundleId], browser: work },
        { match: "*", browser: "Safari" },
      ],
    };
    expect(openUrl("https://anywhere.example", slack, null, config).browser).toMatchObject(work);
  });

  it("rejects unsafe explicit profile directory targets", () => {
    for (const profileDirectory of ["..", ".", "../Profile 2", "a/b", "a\\b", "\n", "\u0000"]) {
      expect(() => openUrl("https://example.com", null, null, {
        defaultBrowser: { ...work, profileDirectory },
      })).toThrow();
    }
  });

  it("detects bundle IDs in profile targets while respecting explicit app types", () => {
    const result = openUrl("https://example.com", slack, null, {
      defaultBrowser: { name: "com.google.Chrome", profileDirectory: "Profile 2" },
    });
    expect(result.browser.appType).toBe("bundleId");
    const explicit = openUrl("https://example.com", slack, null, {
      defaultBrowser: { name: "com.google.Chrome", appType: "appName" },
    });
    expect(explicit.browser.appType).toBe("appName");
    const legacy = openUrl("https://example.com", slack, null, {
      defaultBrowser: { name: "Google Chrome.app" },
    });
    expect(legacy.browser.appType).toBe("appName");
  });
});
