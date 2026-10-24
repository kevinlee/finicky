#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
integration_root="$repo_root/apps/integrations"
build_root="$integration_root/.build"
output_root="${FINICKY_INTEGRATIONS_OUTPUT:-$repo_root/dist/integrations}"
configuration="${CONFIGURATION:-Release}"
signing_identity="${FINICKY_SIGNING_IDENTITY:--}"

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
echo "Built $output_root/Finicky Actions.app"
echo "This companion has not been installed or enabled. See apps/integrations/README.md for setup."
