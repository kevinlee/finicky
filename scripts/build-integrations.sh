#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
integration_root="$repo_root/apps/integrations"
build_root="$integration_root/.build"
output_root="${FINICKY_INTEGRATIONS_OUTPUT:-$repo_root/dist/integrations}"
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
xcodebuild \
  -project "$integration_root/FinickyIntegrations.xcodeproj" \
  -scheme FinickyActions \
  -configuration "$configuration" \
  -derivedDataPath "$build_root/DerivedData" \
  -destination 'platform=macOS' \
  CODE_SIGN_STYLE=Manual \
  CODE_SIGN_IDENTITY="$signing_identity" \
  DEVELOPMENT_TEAM="${FINICKY_DEVELOPMENT_TEAM:-}" \
  build

app_path="$build_root/DerivedData/Build/Products/$configuration/Finicky Actions.app"
ditto "$app_path" "$output_root/Finicky Actions.app"
codesign --verify --deep --strict "$output_root/Finicky Actions.app"
signing_check_args=()
if [[ "$require_runtime_signing" == "1" ]]; then signing_check_args+=(--require-team); fi
bash "$repo_root/scripts/check-integration-signing.sh" "${signing_check_args[@]}" "$output_root/Finicky Actions.app"
echo "Built $output_root/Finicky Actions.app"
echo "This companion has not been installed or enabled. See apps/integrations/README.md for setup."
