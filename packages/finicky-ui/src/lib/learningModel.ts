import type {
  LearningDestination,
  LearningObservation,
  LearningSnapshot,
  RulesFile,
} from "../types";

const record = (value: unknown): value is Record<string, unknown> =>
  Boolean(value) && typeof value === "object" && !Array.isArray(value);
const text = (value: unknown): string | undefined =>
  typeof value === "string" && value.trim() ? value.trim() : undefined;
const count = (value: unknown): value is number =>
  Number.isSafeInteger(value) && Number(value) >= 0;

/** Learning saves immediately; keep pending routing and preference edits intact. */
export function withLearningEnabled(settings: RulesFile, enabled: boolean): RulesFile {
  return {
    ...settings,
    features: {
      ...settings.features,
      learningEnabled: enabled,
      // Go always emits this field; absent and null both select all services.
      nativeAppServices: settings.features?.nativeAppServices ?? null,
    },
  };
}

interface LearningRulesState {
  draft: RulesFile;
  baseline: RulesFile;
  incoming: RulesFile | null;
  externalChange: boolean;
}

function stableJSON(value: unknown): string {
  if (Array.isArray(value)) return `[${value.map(stableJSON).join(",")}]`;
  if (record(value)) return `{${Object.keys(value).sort()
    .filter((key) => value[key] !== undefined)
    .map((key) => `${JSON.stringify(key)}:${stableJSON(value[key])}`).join(",")}}`;
  return JSON.stringify(value);
}

/** Native learning saves emit rules before the dedicated learning snapshot. */
export function reconcileLearningRules(
  state: LearningRulesState,
  next: RulesFile,
): LearningRulesState | null {
  // getRules adds the display path, while saveAndApply sends a bare RulesFile.
  // It is transport metadata, not an externally changed routing preference.
  const { path: previousPath, ...baselineSettings } = state.baseline;
  const { path: nextPath, ...nextSettings } = next;
  if (stableJSON(withLearningEnabled(baselineSettings, false)) !==
      stableJSON(withLearningEnabled(nextSettings, false))) return null;
  const enabled = next.features?.learningEnabled === true;
  const path = nextPath ?? previousPath;
  return {
    ...state,
    draft: withLearningEnabled({ ...state.draft, ...(path ? { path } : {}) }, enabled),
    // Retain local property order: the editor's dirty check compares JSON text.
    baseline: withLearningEnabled({ ...state.baseline, ...(path ? { path } : {}) }, enabled),
    incoming: state.incoming ? withLearningEnabled(state.incoming, enabled) : null,
  };
}

export function learningSourceName(source: string): string {
  const names: Record<string, string> = {
    "com.google.Chrome": "Google Chrome",
    "com.google.Chrome.canary": "Chrome Canary",
    "org.chromium.Chromium": "Chromium",
    "com.apple.Safari": "Safari",
    "com.apple.SafariTechnologyPreview": "Safari Technology Preview",
    "org.mozilla.firefox": "Firefox",
    "com.microsoft.edgemac": "Microsoft Edge",
    "com.brave.Browser": "Brave",
    "company.thebrowser.Browser": "Arc",
  };
  return names[source] || source;
}

/** The learning view accepts hostnames, never full links or URL components. */
export function learningDomain(value: unknown): string | null {
  const domain = text(value);
  if (!domain || domain.length > 253) return null;
  if (/^[a-z\d](?:[a-z\d.-]*[a-z\d])?\.?$/i.test(domain)) return domain;
  if (/^\[[a-f\d:]+\]$/i.test(domain)) return domain;
  return null;
}

export function normalizeLearningSnapshot(value: unknown): LearningSnapshot | null {
  if (!record(value) || typeof value.enabled !== "boolean" ||
      !Array.isArray(value.observations) || !Array.isArray(value.destinations) ||
      !count(value.feedbackCount) || !record(value.evaluation)) return null;
  const { evaluated, predicted, correct, abstained } = value.evaluation;
  if (![evaluated, predicted, correct, abstained].every(count) ||
      Number(predicted) + Number(abstained) !== evaluated ||
      Number(correct) > Number(predicted) || Number(evaluated) > value.feedbackCount)
    return null;
  const destinations: LearningDestination[] = [];
  for (const candidate of value.destinations) {
    if (!record(candidate)) continue;
    const id = text(candidate.id), name = text(candidate.name);
    if (id && name && !destinations.some((destination) => destination.id === id))
      destinations.push({ id, name });
  }
  const observations: LearningObservation[] = [];
  for (const candidate of value.observations) {
    if (!record(candidate)) continue;
    const id = text(candidate.id), domain = learningDomain(candidate.domain);
    const observedAt = text(candidate.observedAt);
    if (!id || !domain || !observedAt || !Number.isFinite(Date.parse(observedAt)) ||
        observations.some((observation) => observation.id === id)) continue;
    observations.push({
      id, domain, observedAt,
      sourceApp: text(candidate.sourceApp),
      confirmedDestinationId: text(candidate.confirmedDestinationId),
      confirmedDestinationName: text(candidate.confirmedDestinationName),
      predictedDestinationId: text(candidate.predictedDestinationId),
      predictedDestinationName: text(candidate.predictedDestinationName),
      reason: text(candidate.reason),
      feedbackKind: text(candidate.feedbackKind),
    });
  }
  return {
    enabled: value.enabled,
    destinations,
    observations: observations.sort((a, b) => Date.parse(b.observedAt) - Date.parse(a.observedAt)),
    feedbackCount: value.feedbackCount,
    evaluation: {
      evaluated: Number(evaluated), predicted: Number(predicted),
      correct: Number(correct), abstained: Number(abstained),
    },
    limitations: Array.isArray(value.limitations)
      ? value.limitations.filter((item): item is string => Boolean(text(item)))
      : [],
  };
}

export function learningPrediction(observation: LearningObservation): string {
  if (!observation.predictedDestinationId) return "Insufficient data";
  return observation.predictedDestinationName || "Unavailable destination";
}

export function learningEvaluation(snapshot: LearningSnapshot): string {
  const { evaluated, predicted, correct, abstained } = snapshot.evaluation;
  if (!evaluated) return "No confirmed choices have been evaluated yet.";
  if (!predicted)
    return `All ${evaluated} confirmed ${evaluated === 1 ? "choice needed" : "choices needed"} more data for a prediction.`;
  return `${correct} of ${predicted} predictions matched the initial chooser selection. ${abstained} ${abstained === 1 ? "choice had" : "choices had"} insufficient data.`;
}

export function learningCorrection(
  snapshot: LearningSnapshot,
  requestId: string,
  destinationId: string,
): { type: "correctLearning"; requestId: string; destinationId: string } | null {
  const observation = snapshot.observations.find((entry) => entry.id === requestId);
  if (!observation?.confirmedDestinationId ||
      observation.confirmedDestinationId === destinationId ||
      !snapshot.destinations.some((destination) => destination.id === destinationId))
    return null;
  return { type: "correctLearning", requestId, destinationId };
}
