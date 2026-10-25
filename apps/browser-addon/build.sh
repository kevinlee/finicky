#!/bin/bash
# Create fresh archives from only the extension's public runtime files.
set -euo pipefail
cd "$(dirname "$0")"
mkdir -p build
staging="$(mktemp -d "$(pwd)/build/.waystile-extension.XXXXXX")"
trap 'rm -rf "$staging"' EXIT
for browser in chrome firefox; do
  folder="$staging/$browser"
  mkdir -p "$folder"
  cp background.js contentScript.js icon16.png icon48.png icon128.png icon256.png "$folder/"
  cp "manifest-$browser.json" "$folder/manifest.json"
  cp ../../LICENSE "$folder/LICENSE"
  (cd "$folder" && zip -q "$staging/$browser.zip" background.js contentScript.js icon16.png icon48.png icon128.png icon256.png manifest.json LICENSE)
  mv -f "$staging/$browser.zip" "build/$browser.zip"
done
printf 'Built fresh Waystile browser extensions in %s/build\n' "$(pwd)"
