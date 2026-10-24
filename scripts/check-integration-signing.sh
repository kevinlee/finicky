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

bundle_team() {
  local bundle_path="$1"
  local expected_identifier="$2"
  local metadata identifier plist_identifier
  if [[ ! -d "$bundle_path" ]]; then
    echo "Required integration bundle is missing: $bundle_path" >&2
    return 1
  fi
  if ! metadata="$(codesign -dv --verbose=4 "$bundle_path" 2>&1)"; then
    printf '%s\n' "$metadata" >&2
    return 1
  fi
  identifier="$(printf '%s\n' "$metadata" | sed -n 's/^Identifier=//p')"
  if ! plist_identifier="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleIdentifier' "$bundle_path/Contents/Info.plist")"; then
    echo "Could not read the bundle identifier for $bundle_path" >&2
    return 1
  fi
  if [[ "$identifier" != "$expected_identifier" || "$plist_identifier" != "$expected_identifier" ]]; then
    echo "Unexpected bundle or signing identifier for $bundle_path; expected $expected_identifier." >&2
    return 1
  fi
  printf '%s\n' "$metadata" | sed -n 's/^TeamIdentifier=//p'
}

team="$(bundle_team "$app_path" 'com.kevinlee.finicky.actions')"
expected_team="${FINICKY_DEVELOPMENT_TEAM:-}"
if [[ -n "$expected_team" && "$team" != "$expected_team" ]]; then
  echo "Finicky Actions does not have the requested signing Team ID ($expected_team)." >&2
  exit 1
fi

extension_paths=(
  "$app_path/Contents/PlugIns/Finicky Safari.appex"
  "$app_path/Contents/PlugIns/Open with Finicky Plus.appex"
  "$app_path/Contents/Extensions/Finicky Focus.appex"
)
extension_identifiers=(
  'com.kevinlee.finicky.actions.safari'
  'com.kevinlee.finicky.actions.share'
  'com.kevinlee.finicky.actions.focus'
)
for index in "${!extension_paths[@]}"; do
  extension="${extension_paths[$index]}"
  extension_team="$(bundle_team "$extension" "${extension_identifiers[$index]}")"
  if [[ "$extension_team" != "$team" ]]; then
    echo "Signing Team ID differs for $(basename "$extension"). Sign the companion and every extension with the same identity." >&2
    exit 1
  fi
done
for extension in "$app_path"/Contents/PlugIns/*.appex "$app_path"/Contents/Extensions/*.appex; do
  [[ -d "$extension" ]] || continue
  case "$extension" in
    "${extension_paths[0]}"|"${extension_paths[1]}"|"${extension_paths[2]}") ;;
    *) echo "Unexpected integration extension: $extension" >&2; exit 1 ;;
  esac
done

codesign --verify --deep --strict "$app_path"
if [[ -z "$team" || "$team" == "not set" ]]; then
  echo "Finicky Actions has no signing Team ID. Bundle integrity passes, but native Shortcuts execution is not validated." >&2
  echo "On the tested Mac, the ad hoc companion failed to connect with LNConnectionErrorDomain 1200 and a missing-team-ID diagnostic, including after registration cleanup." >&2
  echo "Use an existing Apple Development or Developer ID identity and repeat the runtime acceptance checks." >&2
  if "$require_team"; then exit 1; fi
  exit 0
fi
echo "Companion and embedded extensions have matching signing Team IDs. Runtime Shortcuts/Share/Focus/Safari checks are still required."
