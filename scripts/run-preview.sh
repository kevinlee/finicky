#!/bin/bash
# Run an isolated local copy; never replace or register the installed app.
set -euo pipefail
cd "$(dirname "$0")/.."

prepare_only=false
if [ "${1:-}" = "--prepare-only" ]; then
  prepare_only=true
  shift
fi

case "$(uname -m)" in
  arm64) target_arch=arm64 ;;
  x86_64) target_arch=amd64 ;;
  *) echo "Unsupported architecture" >&2; exit 1 ;;
esac

source_app="$(pwd)/apps/finicky/build/Finicky-${target_arch}.app"
preview_app="$(pwd)/apps/finicky/build/Waystile Preview.app"
preview_rules="${1:-$(pwd)/apps/finicky/build/preview-rules.json}"

if [ ! -x "$source_app/Contents/MacOS/Finicky" ]; then
  echo "Build the local app first with ./scripts/build-preview.sh" >&2
  exit 1
fi

if [ ! -e "$preview_rules" ]; then
  mkdir -p "$(dirname "$preview_rules")"
  cat > "$preview_rules" <<'JSON'
{
  "defaultBrowser": "Safari",
  "rules": [],
  "profiles": [],
  "options": { "checkForUpdates": false, "logRequests": false, "keepRunning": true }
}
JSON
fi

ditto "$source_app" "$preview_app"
# The upstream asset copy includes this build-time source beside Info.plist.
# Resources belong under Resources; codesign treats other Contents files as code.
if [ -f "$preview_app/Contents/template-icon.pdf" ]; then
  mv -f "$preview_app/Contents/template-icon.pdf" "$preview_app/Contents/Resources/template-icon.pdf"
fi
preview_plist="$preview_app/Contents/Info.plist"
/usr/libexec/PlistBuddy -c "Set :CFBundleIdentifier com.kevinlee.finicky.preview" "$preview_plist"
/usr/libexec/PlistBuddy -c "Set :CFBundleName Waystile Preview" "$preview_plist"
/usr/libexec/PlistBuddy -c "Set :CFBundleDisplayName Waystile Preview" "$preview_plist"
/usr/libexec/PlistBuddy -c "Delete :CFBundleURLTypes" "$preview_plist" 2>/dev/null || true
/usr/libexec/PlistBuddy -c "Delete :CFBundleDocumentTypes" "$preview_plist" 2>/dev/null || true
/usr/libexec/PlistBuddy -c "Delete :NSUserActivityTypes" "$preview_plist" 2>/dev/null || true
codesign --force --sign - "$preview_app"

if [ "$prepare_only" = true ]; then
  echo "Prepared preview app: $preview_app"
  echo "Preview settings: $preview_rules"
  exit 0
fi

exec "$preview_app/Contents/MacOS/Finicky" --window --no-config --rules "$preview_rules" --preview
