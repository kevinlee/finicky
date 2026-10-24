#!/usr/bin/env bash
set -euo pipefail

# Read-only check. A valid ad hoc seal proves bundle integrity, but does not
# provide the developer identity used by macOS system-integration handshakes.
require_team=false
if [[ "${1:-}" == "--require-team" ]]; then
  require_team=true
  shift
fi
app_path="${1:-}"
if [[ -z "$app_path" || ! -d "$app_path" ]]; then
  echo "Usage: $0 [--require-team] '/path/to/Finicky Actions.app'" >&2
  exit 2
fi

codesign --verify --deep --strict "$app_path"
bundle_team() {
  codesign -dv --verbose=4 "$1" 2>&1 | sed -n 's/^TeamIdentifier=//p'
}
team="$(bundle_team "$app_path")"
if [[ -z "$team" || "$team" == "not set" ]]; then
  echo "Finicky Actions has no signing Team ID. Bundle integrity passes, but native Shortcuts execution is not validated." >&2
  echo "On the tested Mac, the ad hoc companion failed to connect with LNConnectionErrorDomain 1200 and a missing-team-ID diagnostic, including after registration cleanup." >&2
  echo "Use an existing Apple Development or Developer ID identity and repeat the runtime acceptance checks." >&2
  if "$require_team"; then exit 1; fi
  exit 0
fi

for extension in "$app_path"/Contents/PlugIns/*.appex "$app_path"/Contents/Extensions/*.appex; do
  [[ -d "$extension" ]] || continue
  extension_team="$(bundle_team "$extension")"
  if [[ "$extension_team" != "$team" ]]; then
    echo "Signing Team ID differs for $(basename "$extension"). Sign the companion and every extension with the same identity." >&2
    exit 1
  fi
done
echo "Companion and embedded extensions have matching signing Team IDs. Runtime Shortcuts/Share/Focus/Safari checks are still required."
