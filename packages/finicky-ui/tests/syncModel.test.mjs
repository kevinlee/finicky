import assert from "node:assert/strict";
import test from "node:test";
import {
  beginSyncRequest, bindingForFavorite, emptySyncView, equalBindings, normalizeSyncSnapshot,
  receiveSyncResult, receiveSyncSnapshot, reconcileSyncMappings, syncBindingError,
  syncMappingErrors, syncMutationBlocked, syncResolution, syncBindingForSave, updateSyncBindingDraft,
} from "../src/lib/syncModel.ts";

const snapshot = (patch = {}) => ({
  enabled: false, state: "disabled", heads: [], missingProfileIds: ["work", "personal", "academic"],
  profiles: [{ id: "work", name: "Work" }, { id: "personal", name: "Personal" }, { id: "academic", name: "Academic" }],
  bindings: {},
  localProfiles: [{ id: "work", name: "Work", browser: "Google Chrome", profileDirectory: "Profile 7" }],
  ...patch,
});

test("joining a Mac never infers directories from matching shared and local names", () => {
  const incoming = normalizeSyncSnapshot(snapshot());
  assert.deepEqual(incoming.bindings, {});
  assert.deepEqual(Object.keys(syncMappingErrors(incoming, {})), ["work", "personal", "academic"]);
  const explicitChoice = bindingForFavorite(incoming.localProfiles[0]);
  assert.deepEqual(explicitChoice, { browser: "Google Chrome", profileDirectory: "Profile 7" });
  assert.equal(syncBindingError(explicitChoice), "");
});

test("directory mapping requires a stable Chrome target and rejects paths", () => {
  for (const profileDirectory of ["Default", "Profile 2", "Custom Profile"]) {
    assert.equal(syncBindingError({ browser: "Google Chrome", profileDirectory }), "");
  }
  for (const profileDirectory of ["", "../Profile 2", "/Users/me/Profile 2", "..", "Profile\\2", "Profile\n2"]) {
    assert.notEqual(syncBindingError({ browser: "com.google.Chrome", profileDirectory }), "");
  }
  assert.notEqual(syncBindingError({ browser: "Google Chrome", profile: "Work" }), "");
  assert.notEqual(syncBindingError({ browser: "prompt" }), "");
  assert.notEqual(syncBindingError({ browser: "Firefox", profile: "/tmp/profile" }), "");
  assert.equal(syncBindingError({ browser: "Safari" }), "");
});

test("native bundle identifiers match local favorites without losing directory identity", () => {
  const native = { work: { browser: "com.google.Chrome", profile: "Old display name", profileDirectory: "Profile 2" } };
  assert.equal(equalBindings(native, { work: { browser: "Google Chrome", profileDirectory: "Profile 2" } }), true);
  assert.equal(equalBindings(native, { work: { browser: "Google Chrome", profileDirectory: "Profile 7" } }), false);
  assert.equal(equalBindings(native, { work: { browser: "Unknown Browser", profileDirectory: "Profile 2" } }), false);
});

test("sync snapshots fail closed for unknown states and duplicate shared identities", () => {
  assert.equal(normalizeSyncSnapshot(snapshot({ state: "toString" })), null);
  assert.equal(normalizeSyncSnapshot(snapshot({ enabled: "true" })), null);
  assert.equal(normalizeSyncSnapshot(snapshot({ profiles: [{ id: "work", name: "Work" }, { id: "work", name: "Another" }] })), null);
});

test("result before snapshot does not optimistically enable sync", () => {
  let view = receiveSyncSnapshot(emptySyncView(), snapshot());
  view = beginSyncRequest(view, "enableSync");
  view = receiveSyncResult(view, { success: true, operation: "enableSync" });
  assert.equal(view.pending, "enableSync");
  assert.equal(view.snapshot.enabled, false);
  view = receiveSyncSnapshot(view, snapshot({ enabled: true, state: "awaitingMapping" }));
  assert.equal(view.pending, "");
  assert.equal(view.snapshot.enabled, true);
});

test("snapshot before result stays pending until its matching operation succeeds", () => {
  let view = beginSyncRequest(receiveSyncSnapshot(emptySyncView(), snapshot()), "saveSyncBindings");
  view = receiveSyncSnapshot(view, snapshot({ bindings: { work: { browser: "Chrome", profileDirectory: "Profile 2" } } }));
  assert.equal(view.pending, "saveSyncBindings");
  view = receiveSyncResult(view, { success: true, operation: "syncNow" });
  assert.equal(view.pending, "saveSyncBindings");
  view = receiveSyncResult(view, { success: true, operation: "saveSyncBindings" });
  assert.equal(view.pending, "");
});

test("failed sync preserves actual state and its error across background status refreshes", () => {
  let view = beginSyncRequest(receiveSyncSnapshot(emptySyncView(), snapshot()), "enableSync");
  view = receiveSyncResult(view, { success: false, operation: "enableSync", error: "iCloud is offline" });
  assert.equal(view.pending, "");
  assert.equal(view.snapshot.enabled, false);
  view = receiveSyncSnapshot(view, snapshot());
  assert.equal(view.error, "iCloud is offline");
  view = receiveSyncSnapshot(beginSyncRequest(view, "getSync"), snapshot());
  assert.equal(view.error, "");
  assert.equal(view.pending, "");
});

test("mapping polls preserve local edits and surface genuine concurrent changes", () => {
  const baseline = { work: { browser: "Chrome", profileDirectory: "Profile 2" } };
  const draft = { work: { browser: "Chrome", profileDirectory: "Profile 7" } };
  let state = reconcileSyncMappings(draft, baseline, structuredClone(baseline));
  assert.equal(state.conflict, false);
  assert.deepEqual(state.draft, draft);
  state = reconcileSyncMappings(draft, baseline, { work: { browser: "Chrome", profileDirectory: "Profile 4" } });
  assert.equal(state.conflict, true);
  assert.deepEqual(state.draft, draft);
  state = reconcileSyncMappings(draft, baseline, structuredClone(draft));
  assert.equal(state.conflict, false);
  assert.equal(equalBindings(state.draft, state.baseline), true);
});

test("conflict resolution requires an explicit local or remote choice for a current conflict", () => {
  const conflict = snapshot({ enabled: true, state: "conflict", heads: ["revision-a"] });
  for (const choice of ["", "auto", undefined]) assert.equal(syncResolution(conflict, choice), null);
  assert.deepEqual(syncResolution(conflict, "local"), { type: "resolveSync", choice: "local" });
  assert.deepEqual(syncResolution(conflict, "remote"), { type: "resolveSync", choice: "remote", revision: "revision-a" });
  assert.equal(syncResolution(snapshot({ enabled: true, state: "synced" }), "remote"), null);
  assert.equal(syncResolution(snapshot({ enabled: false, state: "conflict" }), "local"), null);
});

test("multiple shared revisions require explicit selection and reject stale choices", () => {
  const conflict = snapshot({ enabled: true, state: "conflict", heads: ["older", "newer"] });
  assert.equal(syncResolution(conflict, "remote"), null);
  assert.equal(syncResolution(conflict, "remote", "gone"), null);
  assert.deepEqual(syncResolution(conflict, "remote", "older"), { type: "resolveSync", choice: "remote", revision: "older" });
  assert.deepEqual(syncResolution(conflict, "local"), { type: "resolveSync", choice: "local" });
});

test("sync mutations are blocked by unapplied edits, save/import activity and external conflicts", () => {
  const clean = { loaded: true, dirty: false, saving: false, importing: false, externalChange: false, otherBusy: false };
  assert.equal(syncMutationBlocked(clean), false);
  for (const key of ["dirty", "saving", "importing", "externalChange", "otherBusy"]) assert.equal(syncMutationBlocked({ ...clean, [key]: true }), true);
  assert.equal(syncMutationBlocked({ ...clean, loaded: false }), true);
});

test("sync state events remain independent of unsaved main settings", () => {
  const draft = { defaultBrowser: "Safari", features: { learningEnabled: true }, rules: [] };
  const before = structuredClone(draft);
  let view = beginSyncRequest(emptySyncView(), "getSync");
  view = receiveSyncSnapshot(view, snapshot({ enabled: true, state: "conflict" }));
  assert.deepEqual(draft, before);
  assert.equal(view.snapshot.state, "conflict");
  assert.equal("draft" in view, false);
});


test("typing a manual browser name preserves the explicit Chrome directory through intermediate text", () => {
  let draft = { browser: "Google Chrome", profileDirectory: "Profile 7" };
  for (const browser of ["", "G", "Google", "Google Chrome Canary"]) {
    draft = updateSyncBindingDraft(draft, { browser });
    assert.equal(draft.profileDirectory, "Profile 7");
  }
  assert.deepEqual(syncBindingForSave(draft, true), { browser: "Google Chrome Canary", profileDirectory: "Profile 7" });
  assert.equal(syncBindingError(syncBindingForSave(draft, true)), "");
});

test("manual save uses only the final browser's explicit profile field without mutating the draft", () => {
  const original = { browser: "Firefox", profile: "Personal", profileDirectory: "Profile 7" };
  const firefox = syncBindingForSave(original, true);
  assert.deepEqual(firefox, { browser: "Firefox", profile: "Personal" });
  assert.equal(original.profileDirectory, "Profile 7");
  const chromeDraft = updateSyncBindingDraft(original, { browser: "com.google.Chrome" });
  assert.deepEqual(syncBindingForSave(chromeDraft, true), { browser: "com.google.Chrome", profileDirectory: "Profile 7" });
  const unmappedChrome = syncBindingForSave({ browser: "Chrome", profile: "Work" }, true);
  assert.deepEqual(unmappedChrome, { browser: "Chrome" });
  assert.notEqual(syncBindingError(unmappedChrome), "");
  assert.deepEqual(syncBindingForSave(original), { browser: "Firefox", profileDirectory: "Profile 7" });
});


test("normalized manual save remains recoverable on failure and clean after its acknowledgement", () => {
  const baseline = { work: { browser: "Google Chrome", profileDirectory: "Profile 7" } };
  const edited = updateSyncBindingDraft(baseline.work, { browser: "Firefox", profile: "Personal" });
  // The editor replaces its draft with exactly the normalized submitted values,
  // retaining its baseline until a native snapshot confirms the save.
  const submitted = { work: syncBindingForSave(edited, true) };
  assert.deepEqual(submitted, { work: { browser: "Firefox", profile: "Personal" } });
  const failedSave = reconcileSyncMappings(submitted, baseline, structuredClone(baseline));
  assert.equal(failedSave.conflict, false);
  assert.deepEqual(failedSave.draft, submitted);
  assert.equal(equalBindings(failedSave.draft, failedSave.baseline), false);
  const successfulRetry = reconcileSyncMappings(failedSave.draft, failedSave.baseline, structuredClone(submitted));
  assert.equal(successfulRetry.conflict, false);
  assert.equal(equalBindings(successfulRetry.draft, successfulRetry.baseline), true);
  assert.deepEqual(successfulRetry.draft, submitted);
  assert.deepEqual(baseline, { work: { browser: "Google Chrome", profileDirectory: "Profile 7" } });
});
