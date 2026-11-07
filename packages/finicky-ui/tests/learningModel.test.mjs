import assert from "node:assert/strict";
import test from "node:test";
import {
  learningCorrection,
  learningDomain,
  learningEvaluation,
  learningPrediction,
  normalizeLearningSnapshot,
  reconcileLearningRules,
  withLearningEnabled,
} from "../src/lib/learningModel.ts";
import { effectiveFeatures } from "../src/lib/featuresModel.ts";
import { normalizeRulesFile } from "../src/lib/rulesModel.ts";

const snapshot = (patch = {}) => ({
  enabled: false,
  observations: [],
  destinations: [{ id: "work", name: "Work" }, { id: "personal", name: "Personal" }],
  feedbackCount: 0,
  evaluation: { evaluated: 0, predicted: 0, correct: 0, abstained: 0 },
  ...patch,
});
const observation = (patch = {}) => ({
  id: "request-1",
  domain: "example.com",
  observedAt: "2026-10-25T12:00:00Z",
  ...patch,
});

test("learning starts off and survives a preferences round trip independently of history", () => {
  assert.equal(effectiveFeatures().learningEnabled, false);
  const preferences = effectiveFeatures({ learningEnabled: true, historyEnabled: false });
  const saved = normalizeRulesFile({ defaultBrowser: "Chrome", rules: [], features: preferences });
  assert.equal(effectiveFeatures(saved.features).learningEnabled, true);
  assert.equal(effectiveFeatures(saved.features).historyEnabled, false);
});

test("learning updates preserve unsaved routing and preference edits without changing their baseline", () => {
  const baseline = { defaultBrowser: "Chrome", rules: [], features: { historyEnabled: false } };
  const draft = { ...baseline, defaultBrowser: "Safari", features: { historyEnabled: true } };
  const savedLearning = withLearningEnabled(baseline, true);
  const pendingEdits = withLearningEnabled(draft, true);
  assert.equal(savedLearning.defaultBrowser, "Chrome");
  assert.equal(savedLearning.features.historyEnabled, false);
  assert.equal(pendingEdits.defaultBrowser, "Safari");
  assert.equal(pendingEdits.features.historyEnabled, true);
  assert.equal(pendingEdits.features.learningEnabled, true);
  assert.equal(baseline.features.learningEnabled, undefined);
  assert.notDeepEqual(savedLearning, pendingEdits);
});

test("native rules before learning confirmation do not conflict with unsaved edits", () => {
  const baseline = normalizeRulesFile({ defaultBrowser: "Chrome", rules: [], features: { learningEnabled: false } });
  const draft = { ...baseline, defaultBrowser: "Safari", features: { ...baseline.features, historyEnabled: true } };
  const next = normalizeRulesFile({ ...baseline, features: { learningEnabled: true } });
  const state = reconcileLearningRules({ draft, baseline, incoming: null, externalChange: false }, next);
  assert.equal(state.externalChange, false);
  assert.equal(state.incoming, null);
  assert.equal(state.draft.defaultBrowser, "Safari");
  assert.equal(state.draft.features.historyEnabled, true);
  assert.equal(state.draft.features.learningEnabled, true);
  assert.equal(state.baseline.defaultBrowser, "Chrome");
  assert.equal(state.baseline.features.learningEnabled, true);
  // The subsequent features/snapshot message is idempotent, with edits still pending.
  assert.deepEqual(withLearningEnabled(state.draft, true), state.draft);
  assert.notDeepEqual(state.draft, state.baseline);
});

test("bare native save notifications preserve the getRules path and unsaved private-default edit", () => {
  const baseline = normalizeRulesFile({
    defaultBrowser: "Google Chrome",
    defaultProfileDirectory: "Default",
    path: "~/preview/rules.json",
    options: { keepRunning: true, logRequests: false, checkForUpdates: false },
    rules: [],
    profiles: [
      { id: "work", name: "Work", browser: "Google Chrome", profileDirectory: "Profile 2" },
      { id: "personal", name: "Personal", browser: "Google Chrome", profileDirectory: "Default" },
      { id: "academic", name: "Academic", browser: "Google Chrome", profileDirectory: "Profile 3" },
    ],
    features: { learningEnabled: false },
  });
  const draft = { ...baseline, defaultPrivate: true };
  // saveAndApply omits path, and Go omits a false learningEnabled value.
  const { path, ...nativeSaved } = baseline;
  const next = { ...nativeSaved, features: { learningEnabled: true, nativeAppServices: null } };
  const state = reconcileLearningRules({ draft, baseline, incoming: null, externalChange: false }, next);
  assert.notEqual(state, null);
  assert.equal(state.externalChange, false);
  assert.equal(state.draft.defaultPrivate, true);
  assert.equal(state.baseline.defaultPrivate, undefined);
  assert.equal(state.draft.path, path);
  assert.equal(state.baseline.path, path);
  assert.equal(state.draft.features.learningEnabled, true);
  assert.notDeepEqual(state.draft, state.baseline);
});

test("first learning save normalizes native null defaults without inventing pending edits", () => {
  const baseline = normalizeRulesFile({ defaultBrowser: "Chrome", rules: [], path: "~/rules.json" });
  const next = normalizeRulesFile({ defaultBrowser: "Chrome", rules: [], features: { learningEnabled: true, nativeAppServices: null } });
  const state = reconcileLearningRules({ draft: baseline, baseline, incoming: null, externalChange: false }, next);
  assert.notEqual(state, null);
  assert.equal(state.externalChange, false);
  assert.deepEqual(state.draft, state.baseline);
  assert.equal(JSON.stringify(state.draft), JSON.stringify(state.baseline));
  // An explicit empty service list means none, so it remains a genuine difference.
  assert.equal(reconcileLearningRules({ draft: baseline, baseline, incoming: null, externalChange: false }, {
    ...next, features: { learningEnabled: true, nativeAppServices: [] },
  }), null);
});

test("a clean editor stays clean when native learning saves reorder object keys", () => {
  const baseline = normalizeRulesFile({
    path: "~/rules.json",
    features: { nativeAppServices: null, learningEnabled: false },
    profiles: [],
    rules: [],
    defaultBrowser: "Chrome",
  });
  const next = normalizeRulesFile({
    defaultBrowser: "Chrome",
    rules: [],
    profiles: [],
    features: { learningEnabled: true, nativeAppServices: null },
  });
  const state = reconcileLearningRules({ draft: baseline, baseline, incoming: null, externalChange: false }, next);
  assert.notEqual(state, null);
  assert.equal(state.externalChange, false);
  assert.equal(state.draft.features.learningEnabled, true);
  assert.equal(JSON.stringify(state.draft), JSON.stringify(state.baseline));
});

test("learning updates preserve genuine conflicts and reject unrelated incoming changes", () => {
  const baseline = normalizeRulesFile({ defaultBrowser: "Chrome", rules: [] });
  const draft = { ...baseline, defaultBrowser: "Safari" };
  const incoming = { ...baseline, defaultBrowser: "Firefox" };
  const next = { ...baseline, features: { learningEnabled: true } };
  const state = reconcileLearningRules({ draft, baseline, incoming, externalChange: true }, next);
  assert.equal(state.externalChange, true);
  assert.equal(state.incoming.defaultBrowser, "Firefox");
  assert.equal(state.incoming.features.learningEnabled, true);
  assert.equal(reconcileLearningRules({ draft, baseline, incoming: null, externalChange: false }, { ...next, defaultBrowser: "Firefox" }), null);
});

test("learning never renders a raw URL or path from a malformed domain field", () => {
  for (const domain of ["https://example.com/private?token=abc", "example.com/private", "example.com?q=secret", "user:pass@example.com", "<script>alert(1)</script>", "example.com\nsecret"]) {
    assert.equal(learningDomain(domain), null);
    assert.deepEqual(normalizeLearningSnapshot(snapshot({ observations: [observation({ domain })] })).observations, []);
  }
  assert.equal(learningDomain("example.com"), "example.com");
  assert.equal(learningDomain("xn--bcher-kva.example"), "xn--bcher-kva.example");
});

test("malformed evaluation cannot turn unlabeled requests into apparent successes", () => {
  for (const evaluation of [
    { evaluated: 1, predicted: 1, correct: 1, abstained: 0 },
    { evaluated: 0, predicted: 1, correct: 0, abstained: 0 },
    { evaluated: 0, predicted: 0, correct: 1, abstained: 0 },
    { evaluated: 0, predicted: -1, correct: 0, abstained: 1 },
  ]) assert.equal(normalizeLearningSnapshot(snapshot({ evaluation })), null);
  const data = normalizeLearningSnapshot(snapshot({ observations: [observation({ predictedDestinationId: "work", predictedDestinationName: "Work" })] }));
  assert.equal(data.feedbackCount, 0);
  assert.equal(data.evaluation.correct, 0);
  assert.equal(learningEvaluation(data), "No confirmed choices have been evaluated yet.");
});

test("reported match counts keep abstentions out of prediction denominator", () => {
  const data = normalizeLearningSnapshot(snapshot({
    feedbackCount: 12,
    evaluation: { evaluated: 12, predicted: 4, correct: 3, abstained: 8 },
  }));
  assert.equal(learningEvaluation(data), "3 of 4 predictions matched the initial chooser selection. 8 choices had insufficient data.");
  assert.equal(learningEvaluation(snapshot({ feedbackCount: 1, evaluation: { evaluated: 1, predicted: 0, correct: 0, abstained: 1 } })), "All 1 confirmed choice needed more data for a prediction.");
});

test("correction requires an explicit changed destination and an existing confirmed example", () => {
  const data = snapshot({ observations: [observation({ confirmedDestinationId: "work" })] });
  for (const destination of ["", "work", "deleted-profile"]) assert.equal(learningCorrection(data, "request-1", destination), null);
  assert.equal(learningCorrection(data, "missing-request", "personal"), null);
  assert.equal(learningCorrection(snapshot({ observations: [observation()] }), "request-1", "personal"), null);
  assert.deepEqual(learningCorrection(data, "request-1", "personal"), {
    type: "correctLearning", requestId: "request-1", destinationId: "personal",
  });
});

test("predictions remain separate from confirmed choices and absent predictions abstain", () => {
  const corrected = observation({ confirmedDestinationId: "personal", confirmedDestinationName: "Personal", predictedDestinationId: "work", predictedDestinationName: "Work", feedbackKind: "correction" });
  assert.equal(learningPrediction(corrected), "Work");
  assert.equal(learningPrediction(observation({ confirmedDestinationId: "personal", confirmedDestinationName: "Personal" })), "Insufficient data");
  assert.equal(learningPrediction(observation({ predictedDestinationId: "removed-profile" })), "Unavailable destination");
});

test("native samples are sanitized, deduplicated and displayed newest first", () => {
  const data = normalizeLearningSnapshot(snapshot({
    observations: [
      observation({ id: "older", observedAt: "2026-10-24T12:00:00Z", url: "https://example.com/secret" }),
      observation({ id: "newer", observedAt: "2026-10-25T12:00:00Z" }),
      observation({ id: "bad", observedAt: "invalid" }),
      observation({ id: "older" }),
    ],
  }));
  assert.deepEqual(data.observations.map((entry) => entry.id), ["newer", "older"]);
  assert.equal(JSON.stringify(data).includes("secret"), false);
});
