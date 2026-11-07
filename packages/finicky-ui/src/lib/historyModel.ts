import type { HistoryEntry } from "../types";

/** Native app URLs must return to their web source before routing again. */
export function historyOpenURL(
  entry: Pick<HistoryEntry, "url" | "originalUrl">,
): string | null {
  for (const candidate of [entry.url, entry.originalUrl]) {
    if (!candidate) continue;
    try {
      if (["http:", "https:", "file:"].includes(new URL(candidate).protocol))
        return candidate;
    } catch {
      /* A malformed saved URL is not a routable destination. */
    }
  }
  return null;
}
