import assert from "node:assert/strict";
import test from "node:test";
import {
  effectiveFeatures,
  parseShorteners,
  shortenerError,
  shortcutError,
  integrationURL,
  nativeServiceChoices,
  selectedNativeServices,
  toggleNativeService,
} from "../src/lib/featuresModel.ts";
import { normalizeRulesFile } from "../src/lib/rulesModel.ts";

test("missing feature preferences preserve existing short-link expansion behavior", () => {
  assert.equal(effectiveFeatures().expandShortURLs, true);
  assert.equal(effectiveFeatures().menuIcon, "default");
  assert.equal(effectiveFeatures().promptMode, "never");
  assert.equal(effectiveFeatures().historyEnabled, false);
  assert.equal(
    effectiveFeatures({ expandShortURLs: false }).expandShortURLs,
    false,
  );
});

test("additional shorteners are deduplicated and reject full URLs or paths", () => {
  assert.deepEqual(
    parseShorteners("Go.Example.com\n bit.ly, go.example.com\n"),
    ["go.example.com", "bit.ly"],
  );
  assert.equal(shortenerError(["go.example.com", "t.co"]), "");
  for (const invalid of [
    "https://go.example.com",
    "go.example.com/path",
    "localhost",
    "a..com",
    "-go.example.com",
  ]) {
    assert.notEqual(shortenerError([invalid]), "");
  }
});

test("picker shortcuts prevent ambiguous case-insensitive duplicate choices", () => {
  const profiles = [
    { id: "work", name: "Work", browser: "Chrome", shortcut: "W" },
  ];
  assert.equal(shortcutError("w", profiles, "work"), "");
  assert.notEqual(shortcutError("w", profiles, "personal"), "");
  assert.equal(shortcutError("", profiles, "personal"), "");
  assert.notEqual(shortcutError("2", profiles, "personal"), "");
  assert.notEqual(shortcutError("cmd+w", profiles, "personal"), "");
});

test("automation URLs encode nested URL query parameters without treating them as routing options", () => {
  const destination = "https://example.com/?q=hello world&private=true#section";
  const url = new URL(
    integrationURL("finicky-plus", "open", {
      url: destination,
      prompt: "true",
      profileDirectory: "Profile 2",
    }),
  );
  assert.equal(url.protocol, "finicky-plus:");
  assert.equal(url.searchParams.get("url"), destination);
  assert.equal(url.searchParams.get("profileDirectory"), "Profile 2");
  assert.equal(url.searchParams.has("private"), false);
  assert.equal(
    integrationURL("invalid://scheme", "clipboard"),
    "finicky-plus://clipboard",
  );
});

test("settings round trips preserve features, picker metadata, and rule launch flags", () => {
  const original = {
    defaultBrowser: "prompt",
    features: {
      promptMode: "unmatched",
      stripTracking: true,
      extraShorteners: ["go.example.com"],
    },
    profiles: [
      {
        id: "work",
        name: "Work",
        browser: "Chrome",
        shortcut: "w",
        hidden: true,
        private: true,
      },
    ],
    rules: [
      {
        match: "*example.com/*",
        browser: "Chrome",
        private: true,
        newWindow: true,
      },
    ],
  };
  const normalized = normalizeRulesFile(original);
  assert.deepEqual(normalized.features, original.features);
  assert.deepEqual(normalized.profiles, original.profiles);
  assert.equal(normalized.rules[0].private, true);
  assert.equal(normalized.rules[0].newWindow, true);
  assert.deepEqual(
    normalizeRulesFile(JSON.parse(JSON.stringify(normalized))),
    normalized,
  );
});

test("native service selection preserves all versus none and toggles without duplicates", () => {
  const all = nativeServiceChoices.map((service) => service.id);
  assert.deepEqual(selectedNativeServices(), all);
  assert.deepEqual(selectedNativeServices(null), all);
  assert.deepEqual(selectedNativeServices([]), []);
  assert.deepEqual(toggleNativeService([], "zoom", true), ["zoom"]);
  assert.deepEqual(toggleNativeService(["zoom"], "zoom", true), ["zoom"]);
  assert.deepEqual(toggleNativeService(["zoom"], "zoom", false), []);
  assert.equal(
    toggleNativeService(undefined, "zoom", false).includes("zoom"),
    false,
  );
});

test("URL scripts and destination privacy survive settings normalization and JSON export", () => {
  const ruleScript = "return url.replace('old.example', 'new.example');";
  const settings = normalizeRulesFile({
    defaultBrowser: "Google Chrome",
    defaultPrivate: true,
    features: {
      transformScript: "return url;",
      nativeAppServices: [],
      alternativePrivate: true,
      alternativeBrowser: "Firefox",
    },
    rules: [
      {
        browser: "Google Chrome",
        match: "*example.com/*",
        transformScript: ruleScript,
      },
    ],
  });
  const restored = normalizeRulesFile(JSON.parse(JSON.stringify(settings)));
  assert.equal(restored.defaultPrivate, true);
  assert.equal(restored.features.alternativePrivate, true);
  assert.deepEqual(restored.features.nativeAppServices, []);
  assert.equal(restored.features.transformScript, "return url;");
  assert.equal(restored.rules[0].transformScript, ruleScript);
});
