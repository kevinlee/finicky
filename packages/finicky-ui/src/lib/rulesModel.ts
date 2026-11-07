import type { Rule, RulesFile, SavedProfile } from "../types";

function record(value: unknown): Record<string, unknown> {
  return value !== null && typeof value === "object" && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : {};
}

/** Native Rule.MarshalJSON uses a string for one URL pattern and null for none. */
export function normalizeRulesFile(value: unknown): RulesFile {
  const source = record(value);
  const rules = Array.isArray(source.rules) ? source.rules : [];
  const profiles = Array.isArray(source.profiles) ? source.profiles : [];

  return {
    ...source,
    defaultBrowser:
      typeof source.defaultBrowser === "string" ? source.defaultBrowser : "",
    profiles: profiles.map(
      (profile) => ({ ...record(profile) }) as unknown as SavedProfile,
    ),
    rules: rules.map((value) => {
      const rule = record(value);
      const match = Array.isArray(rule.match)
        ? rule.match.filter(
            (pattern): pattern is string => typeof pattern === "string",
          )
        : typeof rule.match === "string"
          ? [rule.match]
          : [];
      return {
        ...rule,
        browser: typeof rule.browser === "string" ? rule.browser : "",
        match,
        sourceApps: Array.isArray(rule.sourceApps)
          ? rule.sourceApps.filter(
              (app): app is string => typeof app === "string",
            )
          : [],
      } as Rule;
    }),
  } as RulesFile;
}
