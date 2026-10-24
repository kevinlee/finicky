import assert from "node:assert/strict";
import test from "node:test";
import {
  defaultDestinationValue,
  matchesFavorite,
  effectiveOption,
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
