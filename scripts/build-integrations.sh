#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
integration_root="$repo_root/apps/integrations"
build_root="$integration_root/.build"
output_root="${FINICKY_INTEGRATIONS_OUTPUT:-$repo_root/dist/integrations}"
output_app="$output_root/Waystile Actions.app"
configuration="${CONFIGURATION:-Release}"
signing_identity="${FINICKY_SIGNING_IDENTITY:--}"
require_runtime_signing="${FINICKY_REQUIRE_RUNTIME_SIGNING:-0}"

if [[ "$require_runtime_signing" == "1" && ( "$signing_identity" == "-" || -z "${FINICKY_DEVELOPMENT_TEAM:-}" ) ]]; then
  echo "Runtime integration validation requires FINICKY_SIGNING_IDENTITY and FINICKY_DEVELOPMENT_TEAM for an existing Apple-issued identity." >&2
  echo "Ad hoc signing can build the companion but does not establish a signing Team ID for App Intents communication." >&2
  exit 1
fi

command -v xcodegen >/dev/null || { echo "XcodeGen is required. Install it with Homebrew: brew install xcodegen" >&2; exit 1; }
mkdir -p "$build_root" "$output_root"
xcodegen generate --spec "$integration_root/project.yml" --project "$integration_root"
# A signing-identity change can re-sign standalone extensions without refreshing
# their embedded copies. Start clean so the app receives the new signatures.
xcodebuild \
  -project "$integration_root/FinickyIntegrations.xcodeproj" \
  -scheme FinickyActions \
  -configuration "$configuration" \
  -derivedDataPath "$build_root/DistributionDerivedData" \
  -destination 'platform=macOS' \
  CODE_SIGN_STYLE=Manual \
  CODE_SIGN_IDENTITY="$signing_identity" \
  DEVELOPMENT_TEAM="${FINICKY_DEVELOPMENT_TEAM:-}" \
  ARCHS="arm64 x86_64" \
  ONLY_ACTIVE_ARCH=NO \
  ENABLE_HARDENED_RUNTIME=YES \
  CODE_SIGN_INJECT_BASE_ENTITLEMENTS=NO \
  clean build

app_path="$build_root/DistributionDerivedData/Build/Products/$configuration/Finicky Actions.app"
# Do not merge a new release into an old bundle: stale embedded extensions or
# test-only resources can otherwise survive. Preserve the previous output.
if [[ -e "$output_app" ]]; then
  previous_output="$(mktemp -d "$output_root/.previous-integrations.XXXXXX")"
  mv "$output_app" "$previous_output/Waystile Actions.app"
fi
ditto "$app_path" "$output_app"
codesign --verify --deep --strict "$output_app"
if [[ "$require_runtime_signing" == "1" ]]; then
  bash "$repo_root/scripts/check-integration-signing.sh" --require-team "$output_app"
else
  # macOS's system Bash 3.2 treats an empty array as unset under nounset.
  bash "$repo_root/scripts/check-integration-signing.sh" "$output_app"
fi
echo "Built $output_app"
echo "This companion has not been installed or enabled. See apps/integrations/README.md for setup."
