import type { Features, SavedProfile } from "../types";

export const featureDefaults: Features = {
  menuIcon: "default",
  promptMode: "never",
  promptModifier: "fn",
  stripTracking: false,
  unwrapRedirects: false,
  expandShortURLs: true,
  extraShorteners: [],
  historyEnabled: false,
  learningEnabled: false,
  nativeApps: false,
};

export function effectiveFeatures(features?: Partial<Features>): Features {
  return { ...featureDefaults, ...features };
}

export function parseShorteners(value: string): string[] {
  return [
    ...new Set(
      value
        .split(/[\n,]/)
        .map((host) => host.trim().toLowerCase())
        .filter(Boolean),
    ),
  ];
}

export function shortenerError(hosts: string[]): string {
  return hosts.some(
    (host) => !/^(?:[a-z0-9](?:[a-z0-9-]*[a-z0-9])?\.)+[a-z]{2,}$/i.test(host),
  )
    ? "Enter hostnames only, such as go.example.com, with no paths or https://."
    : "";
}

export function shortcutError(
  shortcut: string,
  profiles: SavedProfile[],
  id: string,
): string {
  if (shortcut && !/^[a-z]$/i.test(shortcut))
    return "Use one letter (A–Z) for the picker shortcut.";
  if (
    shortcut &&
    profiles.some(
      (profile) =>
        profile.id !== id &&
        profile.shortcut?.toLowerCase() === shortcut.toLowerCase(),
    )
  ) {
    return "Another profile already uses this picker shortcut.";
  }
  return "";
}

export function integrationURL(
  scheme: string,
  action: string,
  values: Record<string, string> = {},
): string {
  const protocol = /^[a-z][a-z0-9+.-]*$/i.test(scheme)
    ? scheme
    : "finicky-plus";
  const query = new URLSearchParams(values).toString();
  return `${protocol}://${action}${query ? `?${query}` : ""}`;
}

export const nativeServiceChoices: {
  id: string;
  name: string;
  hint?: string;
}[] = [
  { id: "zoom", name: "Zoom meetings" },
  { id: "teams", name: "Microsoft Teams" },
  { id: "spotify", name: "Spotify" },
  { id: "figma", name: "Figma" },
  { id: "linear", name: "Linear" },
  { id: "obsidian", name: "Obsidian" },
  { id: "googleMeet", name: "Google Meet → Chrome" },
  { id: "clickup", name: "ClickUp" },
  {
    id: "slack",
    name: "Slack",
    hint: "Links with explicit workspace and channel IDs",
  },
  {
    id: "mastodon",
    name: "Mastodon → Ivory",
    hint: "Profiles and posts on mastodon.social and mastodon.online",
  },
];

/** An absent/null selection enables all adapters; an empty array enables none. */
export function selectedNativeServices(services?: string[] | null): string[] {
  return services ?? nativeServiceChoices.map((service) => service.id);
}

export function toggleNativeService(
  services: string[] | null | undefined,
  id: string,
  enabled: boolean,
): string[] {
  const next = new Set(selectedNativeServices(services));
  if (enabled) next.add(id);
  else next.delete(id);
  return [...next];
}
