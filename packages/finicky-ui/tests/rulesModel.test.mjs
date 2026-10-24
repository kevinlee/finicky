import assert from "node:assert/strict";
import test from "node:test";
import { normalizeRulesFile } from "../src/lib/rulesModel.ts";

test("normalizes each native match encoding while preserving destinations and options", () => {
  const native = {
    defaultBrowser: "Google Chrome",
    defaultProfileDirectory: "Profile 2",
    path: "/Users/example/.config/finicky/rules.json",
    options: { keepRunning: false, logRequests: true },
    profiles: [
      {
        id: "work",
        name: "Work",
        browser: "Google Chrome",
        profileDirectory: "Profile 2",
      },
    ],
    rules: [
      {
        match: null,
        sourceApps: ["com.example.app"],
        browser: "Google Chrome",
        profileDirectory: "Profile 2",
        enabled: false,
      },
      { match: "*example.edu/*", browser: "Safari", name: "University" },
      {
        match: ["*example.com/*", "*example.org/*"],
        browser: "Firefox",
        profile: "Personal",
      },
    ],
  };
  const model = normalizeRulesFile(native);
  assert.deepEqual(
    model.rules.map((rule) => rule.match),
    [[], ["*example.edu/*"], ["*example.com/*", "*example.org/*"]],
  );
  assert.equal(model.rules[0].enabled, false);
  assert.equal(model.rules[0].profileDirectory, "Profile 2");
  assert.deepEqual(model.rules[0].sourceApps, ["com.example.app"]);
  assert.equal(model.rules[1].name, "University");
  assert.equal(model.rules[2].profile, "Personal");
  assert.deepEqual(model.options, native.options);
  assert.deepEqual(model.profiles, native.profiles);
  assert.equal(model.defaultProfileDirectory, "Profile 2");
  assert.equal(model.path, native.path);
  assert.equal(native.rules[0].match, null);
  assert.equal(native.rules[1].match, "*example.edu/*");
});

test("an empty native file always provides safe collections", () => {
  for (const native of [null, undefined, {}, { rules: null, profiles: null }]) {
    const model = normalizeRulesFile(native);
    assert.deepEqual(model.rules, []);
    assert.deepEqual(model.profiles, []);
    assert.equal(model.defaultBrowser, "");
  }
});

test("array-valued edits survive a native save-and-reload round trip", () => {
  const draft = normalizeRulesFile({
    rules: [{ match: "*example.com/*", browser: "Safari" }],
  });
  draft.rules[0].match.push("*example.org/*");
  const reloaded = normalizeRulesFile(JSON.parse(JSON.stringify(draft)));
  assert.deepEqual(reloaded, draft);
  assert.equal(
    reloaded.rules[0].match.some((pattern) => pattern.includes("example.org")),
    true,
  );
  assert.equal(
    reloaded.rules[0].match.join("\n"),
    "*example.com/*\n*example.org/*",
  );
});
