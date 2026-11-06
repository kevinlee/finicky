#!/usr/bin/env bash
# Build universal Waystile + Waystile Actions, sign with Apple Development/Developer ID
# and Hardened Runtime, install locally on this Mac, and publish the verified update
# package + release.json to iCloud Drive so other Macs receive a one-click update option.
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."

SIGNING_IDENTITY="${FINICKY_SIGNING_IDENTITY:-}"
DEVELOPMENT_TEAM="${FINICKY_DEVELOPMENT_TEAM:-}"

if [[ -z "$SIGNING_IDENTITY" ]]; then
  SIGNING_IDENTITY="$(security find-identity -v -p codesigning 2>/dev/null | sed -n 's/.*"\(Developer ID Application: [^"]*\)".*/\1/p' | head -n 1)"
fi
if [[ -z "$SIGNING_IDENTITY" ]]; then
  SIGNING_IDENTITY="$(security find-identity -v -p codesigning 2>/dev/null | sed -n 's/.*"\(Apple Development: [^"]*\)".*/\1/p' | head -n 1)"
fi
: "${SIGNING_IDENTITY:?Set FINICKY_SIGNING_IDENTITY to an Apple Development or Developer ID Application identity}"

echo "1/4 Building universal Waystile.app and browser extensions..."
BUILD_UNIVERSAL=1 ./scripts/build-fork.sh
codesign --force --options runtime --sign "$SIGNING_IDENTITY" "apps/finicky/build/Waystile.app"
codesign --verify --deep --strict "apps/finicky/build/Waystile.app"

if [[ -z "$DEVELOPMENT_TEAM" ]]; then
  DEVELOPMENT_TEAM="$(codesign -dv --verbose=4 "apps/finicky/build/Waystile.app" 2>&1 | sed -n 's/^TeamIdentifier=//p' | head -n 1)"
fi
: "${DEVELOPMENT_TEAM:?Set FINICKY_DEVELOPMENT_TEAM to your 10-character Apple Team ID}"

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
