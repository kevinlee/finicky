import type {
  ConfigOptions,
  Features,
  Rule,
  RulesFile,
  SavedProfile,
} from "../types";

type Destination = {
  browser: string;
  profile?: string;
  profileDirectory?: string;
  private?: boolean;
};

/** Private and regular favorites are different destinations. */
export function matchesFavorite(
  profile: Destination,
  target: Destination,
): boolean {
  return (
    profile.browser === target.browser &&
    (profile.profileDirectory || "") === (target.profileDirectory || "") &&
    (profile.profileDirectory
      ? true
      : (profile.profile || "") === (target.profile || "")) &&
    Boolean(profile.private) === Boolean(target.private)
  );
}

export function defaultDestinationValue(
  model: RulesFile,
  profiles: SavedProfile[],
): string {
  const target = {
    browser: model.defaultBrowser,
    profile: model.defaultProfile,
    profileDirectory: model.defaultProfileDirectory,
    private: model.defaultPrivate,
  };
  const saved = profiles.find((profile) => matchesFavorite(profile, target));
  return saved
    ? `profile:${saved.id}`
    : model.defaultProfile || model.defaultProfileDirectory
      ? "custom"
      : `browser:${model.defaultBrowser}`;
}

/** Keep an explicitly selected custom editor open even if it matches a favorite. */
export function alternativeDestinationValue(
  features: Partial<Features>,
  profiles: SavedProfile[],
  editingCustom = false,
): string {
  if (!features.alternativeBrowser) return "none";
  if (editingCustom) return "custom";
  const favorite = profiles.find((profile) =>
    matchesFavorite(profile, {
      browser: features.alternativeBrowser!,
      profile: features.alternativeProfile,
      profileDirectory: features.alternativeProfileDirectory,
      private: features.alternativePrivate,
    }),
  );
  return favorite ? `profile:${favorite.id}` : "custom";
}

/** Update only rules using this favorite; their own launch options stay intact. */
export function updateRulesForProfile(
  rules: Rule[],
  previous: SavedProfile,
  next: SavedProfile,
): Rule[] {
  return rules.map((rule) => {
    if (!matchesFavorite(previous, rule)) return rule;
    const updated = { ...rule, browser: next.browser };
    delete updated.profile;
    delete updated.profileDirectory;
    if (next.profile) updated.profile = next.profile;
    if (next.profileDirectory) updated.profileDirectory = next.profileDirectory;
    return updated;
  });
}

/** Saved options override the active JavaScript value, including explicit false. */
export function effectiveOption(
  saved: Partial<ConfigOptions> | undefined,
  configured: Partial<ConfigOptions> | undefined,
  key: keyof ConfigOptions,
  fallback: boolean,
): boolean {
  return saved?.[key] ?? configured?.[key] ?? fallback;
}
