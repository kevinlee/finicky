#!/usr/bin/env bash
# Build universal Waystile + Waystile Actions, sign with Apple Development/Developer ID,
# install locally on this Mac, and publish the verified update package + release.json
# to iCloud Drive so other Macs receive a one-click "Update & Relaunch" option.
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."

SIGNING_IDENTITY="${FINICKY_SIGNING_IDENTITY:-Apple Development: Kevin Lee (7YU96FM828)}"
DEVELOPMENT_TEAM="${FINICKY_DEVELOPMENT_TEAM:-4F8Q446767}"

echo "1/4 Building universal Waystile.app and browser extensions..."
BUILD_UNIVERSAL=1 ./scripts/build-fork.sh
codesign --force --deep --sign "$SIGNING_IDENTITY" "apps/finicky/build/Waystile.app"
codesign --verify --deep --strict "apps/finicky/build/Waystile.app"

echo "2/4 Building and signing universal Waystile Actions.app..."
FINICKY_SIGNING_IDENTITY="$SIGNING_IDENTITY" \
FINICKY_DEVELOPMENT_TEAM="$DEVELOPMENT_TEAM" \
FINICKY_REQUIRE_RUNTIME_SIGNING=1 \
./scripts/build-integrations.sh

echo "3/4 Installing to /Applications and updating unpacked Chrome Extension..."
killall "Finicky" "Finicky Actions" 2>/dev/null || true
ditto "apps/finicky/build/Waystile.app" "/Applications/Waystile.app"
ditto "dist/integrations/Waystile Actions.app" "/Applications/Waystile Actions.app"
mkdir -p "$HOME/Library/Application Support/Finicky Plus/Chrome Extension"
unzip -qo "apps/browser-addon/build/chrome.zip" -d "$HOME/Library/Application Support/Finicky Plus/Chrome Extension"

LSREGISTER="/System/Library/Frameworks/CoreServices.framework/Versions/Current/Frameworks/LaunchServices.framework/Versions/Current/Support/lsregister"
for b in apps/finicky/build/*.app dist/integrations/*.app; do
  if [ -d "$b" ]; then
    "$LSREGISTER" -u "$b" 2>/dev/null || true
  fi
done
"$LSREGISTER" -f -R -trusted "/Applications/Waystile.app"
"$LSREGISTER" -f -R -trusted "/Applications/Waystile Actions.app"

echo "4/4 Publishing verified update bundle & release.json to iCloud Drive..."
"/Applications/Waystile.app/Contents/MacOS/Finicky" --publish-update
