export interface NativeTestResult {
  url?: string;
  action?: string;
  browser?: string;
  profile?: string;
  profileDirectory?: string;
  private?: boolean;
  newWindow?: boolean;
  args?: string[] | null;
  error?: string;
  openInBackground?: boolean;
}

export function isPromptResult(result: NativeTestResult | null): boolean {
  return result?.action === "prompt" || result?.browser === "prompt";
}

/** Older JS configurations select Chrome profiles through open's arguments. */
export function testResultTarget(result: NativeTestResult | null) {
  let directory = result?.profileDirectory;
  const args = Array.isArray(result?.args) ? result.args : [];
  if (!directory) {
    for (let index = 0; index < args.length; index += 1) {
      const argument = args[index];
      if (argument.startsWith("--profile-directory=")) {
        directory = argument.slice("--profile-directory=".length) || undefined;
      } else if (argument === "--profile-directory" && args[index + 1]) {
        directory = args[index + 1];
      }
      if (directory) break;
    }
  }
  return {
    browser: isPromptResult(result) ? "prompt" : result?.browser || "",
    profile: result?.profile,
    profileDirectory: directory,
    private: result?.private,
  };
}

export function testProfileDescription(
  result: NativeTestResult | null,
): string {
  if (isPromptResult(result)) return "Choose browser / profile";
  const target = testResultTarget(result);
  const description =
    target.profileDirectory ||
    target.profile ||
    (result?.args?.length ? "Custom launch arguments" : "Last used profile");
  return [
    description,
    result?.private ? "Private browsing" : "",
    result?.newWindow ? "New window" : "",
  ]
    .filter(Boolean)
    .join(" · ");
}
