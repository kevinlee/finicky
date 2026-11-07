import assert from "node:assert/strict";
import test from "node:test";
import {
  defaultDestinationValue,
  matchesFavorite,
  effectiveOption,
  alternativeDestinationValue,
  updateRulesForProfile,
} from "../src/lib/settingsModel.ts";

const regular = {
  id: "work",
  name: "Work",
  browser: "Google Chrome",
  profileDirectory: "Profile 2",
};
const privateProfile = {
  ...regular,
  id: "work-private",
  name: "Work private",
  private: true,
};

test("default selection distinguishes private and regular favorites sharing one profile", () => {
  const model = {
    defaultBrowser: "Google Chrome",
    defaultProfileDirectory: "Profile 2",
    rules: [],
  };
  assert.equal(
    defaultDestinationValue(model, [regular, privateProfile]),
    "profile:work",
  );
  assert.equal(
    defaultDestinationValue({ ...model, defaultPrivate: true }, [
      regular,
      privateProfile,
    ]),
    "profile:work-private",
  );
  assert.equal(
    defaultDestinationValue({ defaultBrowser: "", rules: [] }, [regular]),
    "browser:",
  );
});

test("switching a matching alternative favorite to custom keeps the editor available", () => {
  const features = {
    alternativeBrowser: regular.browser,
    alternativeProfileDirectory: regular.profileDirectory,
  };
  const favorites = [regular, privateProfile];
  assert.equal(
    alternativeDestinationValue(features, favorites),
    "profile:work",
  );
  assert.equal(
    alternativeDestinationValue(features, favorites, true),
    "custom",
  );
  assert.equal(
    alternativeDestinationValue(
      { ...features, alternativePrivate: true },
      favorites,
      true,
    ),
    "custom",
  );
  assert.equal(
    alternativeDestinationValue(
      { ...features, alternativePrivate: true },
      favorites,
      false,
    ),
    "profile:work-private",
  );
  assert.equal(alternativeDestinationValue({}, favorites, true), "none");
});

test("editing a private favorite updates its rules without rewriting regular routes", () => {
  const regularRule = {
    ...regular,
    name: "Regular route",
    match: ["example.com/*"],
    newWindow: true,
  };
  const privateRule = {
    ...privateProfile,
    name: "Private route",
    match: [],
    sourceApps: ["com.example.app"],
    newWindow: true,
  };
  const moved = { ...privateProfile, profileDirectory: "Profile 3" };
  const updated = updateRulesForProfile(
    [regularRule, privateRule],
    privateProfile,
    moved,
  );
  assert.deepEqual(updated[0], regularRule);
  assert.equal(updated[1].profileDirectory, "Profile 3");
  assert.equal(updated[1].private, true);
  assert.equal(updated[1].newWindow, true);
  assert.deepEqual(updated[1].sourceApps, ["com.example.app"]);
  assert.equal(privateRule.profileDirectory, "Profile 2");

  const movedRegular = updateRulesForProfile(
    [regularRule, privateRule],
    regular,
    { ...regular, profileDirectory: "Profile 4" },
  );
  assert.equal(movedRegular[0].profileDirectory, "Profile 4");
  assert.deepEqual(movedRegular[1], privateRule);
});

test("favorite matching includes privacy while directory identity takes precedence over display name", () => {
  assert.equal(matchesFavorite(regular, privateProfile), false);
  assert.equal(
    matchesFavorite(privateProfile, { ...regular, private: true }),
    true,
  );
  assert.equal(
    matchesFavorite(regular, {
      ...regular,
      private: false,
      profile: "Different display name",
    }),
    true,
  );
  assert.equal(
    matchesFavorite(
      { browser: "Firefox", profile: "One" },
      { browser: "Firefox", profile: "Two" },
    ),
    false,
  );
});

test("saved preferences override JavaScript options and inherited toggles invert their effective value", () => {
  assert.equal(
    effectiveOption(
      { logRequests: false },
      { logRequests: true },
      "logRequests",
      false,
    ),
    false,
  );
  assert.equal(
    effectiveOption(
      { keepRunning: true },
      { keepRunning: false },
      "keepRunning",
      false,
    ),
    true,
  );
  assert.equal(
    effectiveOption(undefined, { logRequests: true }, "logRequests", false),
    true,
  );
  const toggled = !effectiveOption(
    undefined,
    { logRequests: true },
    "logRequests",
    false,
  );
  assert.equal(toggled, false);
  assert.equal(
    effectiveOption(undefined, undefined, "keepRunning", true),
    true,
  );
});
