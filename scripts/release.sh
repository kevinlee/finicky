#!/bin/bash
# Sign and notarize a prepared app with existing credentials; never installs it.
set -euo pipefail
phase=${1:-}
app=${2:-}
release_dir=${3:-}
if [[ "$phase" != sign && "$phase" != sign-companion && "$phase" != notarize ]] || [[ ! -d "$app/Contents" ]] || [[ -z "$release_dir" ]]; then
  echo 'Usage: SIGNING_IDENTITY=... release.sh sign|sign-companion APP RELEASE_DIR' >&2
  echo '       NOTARY_KEYCHAIN_PROFILE=... release.sh notarize APP RELEASE_DIR' >&2
  exit 2
fi
umask 077
mkdir -p "$release_dir"
app=$(cd "$(dirname "$app")" && pwd)/$(basename "$app")
release_dir=$(cd "$release_dir" && pwd)
verify_architectures() {
  local bundle="$1" executable arches
  executable=$(/usr/libexec/PlistBuddy -c 'Print :CFBundleExecutable' "$bundle/Contents/Info.plist")
  arches=$(lipo -archs "$bundle/Contents/MacOS/$executable")
  [[ " $arches " == *' arm64 '* && " $arches " == *' x86_64 '* ]] || { echo "A universal binary is required: $bundle" >&2; exit 1; }
}
verify_architectures "$app"
while IFS= read -r -d '' extension; do verify_architectures "$extension"; done < <(find "$app/Contents" -type d -name '*.appex' -print0)
identifier=$(/usr/libexec/PlistBuddy -c 'Print :CFBundleIdentifier' "$app/Contents/Info.plist")
if [[ "$phase" == sign && "$identifier" != com.kevinlee.finicky ]] || [[ "$phase" == sign-companion && "$identifier" != com.kevinlee.finicky.actions ]]; then
  echo 'The signing phase does not match this app identity' >&2; exit 1
fi
name=$(basename "$app" .app)
version=$(/usr/libexec/PlistBuddy -c 'Print :CFBundleShortVersionString' "$app/Contents/Info.plist")
archive="$release_dir/${name// /-}-${version}-beta.1-universal.zip"
if [[ "$phase" == sign || "$phase" == sign-companion ]]; then
  : "${SIGNING_IDENTITY:?Set an existing Developer ID Application identity}"
  if [[ "$phase" == sign-companion ]]; then
    source_root=$(cd "$(dirname "$0")/.." && pwd)
    codesign --force --options runtime --timestamp --sign "$SIGNING_IDENTITY" \
      --entitlements "$source_root/apps/integrations/Safari/FinickySafari.entitlements" "$app/Contents/PlugIns/Finicky Safari.appex"
    codesign --force --options runtime --timestamp --sign "$SIGNING_IDENTITY" \
      --entitlements "$source_root/apps/integrations/Share/FinickyShare.entitlements" "$app/Contents/PlugIns/Open with Finicky Plus.appex"
    codesign --force --options runtime --timestamp --sign "$SIGNING_IDENTITY" \
      --entitlements "$source_root/apps/integrations/Focus/FinickyFocus.entitlements" "$app/Contents/Extensions/Finicky Focus.appex"
    codesign --force --options runtime --timestamp --sign "$SIGNING_IDENTITY" \
      --entitlements "$source_root/apps/integrations/App/FinickyActions.entitlements" "$app"
    "$source_root/scripts/check-integration-signing.sh" --require-team "$app"
  else
    [[ ! -d "$app/Contents/PlugIns" && ! -d "$app/Contents/Extensions" ]] || { echo 'Use sign-companion for embedded extensions' >&2; exit 1; }
    codesign --force --options runtime --timestamp --sign "$SIGNING_IDENTITY" "$app"
  fi
  codesign --verify --strict --deep --verbose=2 "$app"
  details=$(codesign -dv --verbose=4 "$app" 2>&1)
  [[ "$details" == *'Authority=Developer ID Application:'* && "$details" == *'runtime'* ]] || { echo 'Developer ID / hardened runtime verification failed' >&2; exit 1; }
  if codesign -d --entitlements :- "$app" 2>/dev/null | /usr/bin/grep -q 'get-task-allow'; then
    echo 'Development entitlement is not allowed in a public release' >&2; exit 1
  fi
  printf 'Signed: %s\n' "$app"
  exit 0
fi
: "${NOTARY_KEYCHAIN_PROFILE:?Set an existing notarytool profile name}"
[[ ! -e "$archive" ]] || { echo 'Refusing to overwrite a release archive' >&2; exit 1; }
codesign --verify --strict --deep "$app"
details=$(codesign -dv --verbose=4 "$app" 2>&1)
[[ "$details" == *'Authority=Developer ID Application:'* && "$details" == *'runtime'* ]] || { echo 'Notarization requires a Developer ID release build' >&2; exit 1; }
submission="$release_dir/.notary-upload.zip"
[[ ! -e "$submission" && ! -e "$release_dir/notary-result.json" ]] || { echo 'Inspect the existing submission before retrying' >&2; exit 1; }
ditto -c -k --keepParent "$app" "$submission"
notary_args=(--keychain-profile "$NOTARY_KEYCHAIN_PROFILE")
if [[ -n "${NOTARY_KEYCHAIN:-}" ]]; then notary_args+=(--keychain "$NOTARY_KEYCHAIN"); fi
xcrun notarytool submit "$submission" "${notary_args[@]}" --wait --output-format json > "$release_dir/notary-result.json"
status=$(/usr/bin/plutil -extract status raw -o - "$release_dir/notary-result.json")
[[ "$status" == Accepted ]] || { echo 'Apple did not accept the submission; inspect the private result' >&2; exit 1; }
xcrun stapler staple "$app"
xcrun stapler validate "$app"
spctl --assess --type execute --verbose=2 "$app"
ditto -c -k --keepParent "$app" "$archive"
(cd "$release_dir" && shasum -a 256 "$(basename "$archive")" > "$(basename "$archive").sha256")
printf 'Release: %s\n' "$archive"
