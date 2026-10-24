#!/bin/bash
# Build this checkout without replacing the installed Finicky app.
set -euo pipefail
cd "$(dirname "$0")/.."

case "$(uname -m)" in
  arm64) target_arch=arm64 ;;
  x86_64) target_arch=amd64 ;;
  *) echo "Unsupported architecture" >&2; exit 1 ;;
esac

BUILD_TARGET_ARCH="$target_arch" ./scripts/build.sh
echo "Local app: $(pwd)/apps/finicky/build/Finicky-${target_arch}.app"
