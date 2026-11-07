import assert from "node:assert/strict";
import test from "node:test";
import {
  testProfileDescription,
  isPromptResult,
  testResultTarget,
} from "../src/lib/testResult.ts";
import { matchesFavorite } from "../src/lib/settingsModel.ts";

test("recognizes the profile directory in an existing JavaScript launch configuration", () => {
  const result = {
    browser: "Google Chrome",
    args: [
      "-n",
      "--args",
      "--profile-directory=Profile 2",
      "https://example.com",
    ],
  };
  assert.deepEqual(testResultTarget(result), {
    browser: "Google Chrome",
    profile: undefined,
    profileDirectory: "Profile 2",
    private: undefined,
  });
  assert.equal(testProfileDescription(result), "Profile 2");
});

test("private test results select the private favorite and describe launch flags", () => {
  const regular = {
    id: "regular",
    browser: "Google Chrome",
    profileDirectory: "Profile 2",
  };
  const privateFavorite = { ...regular, id: "private", private: true };
  const result = {
    browser: "Google Chrome",
    profileDirectory: "Profile 2",
    private: true,
    newWindow: true,
  };
  const target = testResultTarget(result);
  assert.equal(
    [regular, privateFavorite].find((profile) =>
      matchesFavorite(profile, target),
    )?.id,
    "private",
  );
  assert.equal(
    testProfileDescription(result),
    "Profile 2 · Private browsing · New window",
  );
  assert.equal(
    testProfileDescription({ ...result, private: false, newWindow: false }),
    "Profile 2",
  );
});

test("supports a separate profile argument and preserves explicit resolved profiles", () => {
  assert.equal(
    testResultTarget({ args: ["--profile-directory", "Default"] })
      .profileDirectory,
    "Default",
  );
  assert.equal(
    testResultTarget({
      profileDirectory: "Profile 3",
      args: ["--profile-directory=Profile 2"],
    }).profileDirectory,
    "Profile 3",
  );
  assert.equal(testProfileDescription({ profile: "Personal" }), "Personal");
});

test("custom launch arguments never claim the last used profile", () => {
  assert.equal(
    testProfileDescription({ args: ["--incognito"] }),
    "Custom launch arguments",
  );
  assert.equal(
    testProfileDescription({ args: ["--profile-directory"] }),
    "Custom launch arguments",
  );
  assert.equal(testProfileDescription({ args: [] }), "Last used profile");
  assert.equal(testProfileDescription({ args: null }), "Last used profile");
});

test("typed prompt actions are displayed even when no browser is returned", () => {
  assert.equal(isPromptResult({ action: "prompt" }), true);
  assert.equal(testResultTarget({ action: "prompt" }).browser, "prompt");
  assert.equal(
    testProfileDescription({ action: "prompt" }),
    "Choose browser / profile",
  );
  assert.equal(isPromptResult({ browser: "prompt" }), true);
  assert.equal(isPromptResult({ browser: "Safari", action: "open" }), false);
});
