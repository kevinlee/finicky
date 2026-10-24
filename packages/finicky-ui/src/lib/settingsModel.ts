import type { ConfigOptions, RulesFile, SavedProfile } from "../types";

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

/** Saved options override the active JavaScript value, including explicit false. */
export function effectiveOption(
  saved: Partial<ConfigOptions> | undefined,
  configured: Partial<ConfigOptions> | undefined,
  key: keyof ConfigOptions,
  fallback: boolean,
): boolean {
  return saved?.[key] ?? configured?.[key] ?? fallback;
}
