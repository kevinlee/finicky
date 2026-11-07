#!/bin/bash
# Build a separate, locally signed app without replacing the installed router.
set -euo pipefail
cd "$(dirname "$0")/.."
case "$(uname -m)" in
  arm64) target_arch=arm64 ;;
  x86_64) target_arch=amd64 ;;
  *) echo "Unsupported architecture" >&2; exit 1 ;;
esac
if [ "${BUILD_UNIVERSAL:-0}" = "1" ]; then
  BUILD_UNIVERSAL=1 ./scripts/build.sh
  source_app="apps/finicky/build/Finicky.app"
else
  BUILD_TARGET_ARCH="$target_arch" ./scripts/build.sh
  source_app="apps/finicky/build/Finicky-${target_arch}.app"
fi
fork_app="apps/finicky/build/Waystile.app"
# Build outputs are disposable; never merge stale bundle resources into a release.
rm -rf "$fork_app"
ditto "$source_app" "$fork_app"
if [ -f "$fork_app/Contents/template-icon.pdf" ]; then
  mv "$fork_app/Contents/template-icon.pdf" "$fork_app/Contents/Resources/template-icon.pdf"
fi
codesign --force --deep --sign - "$fork_app"
(cd apps/browser-addon && bash build.sh)
if [ "${1:-}" = "--integrations" ]; then bash scripts/build-integrations.sh; fi
printf 'Built: %s/%s\n' "$(pwd)" "$fork_app"
