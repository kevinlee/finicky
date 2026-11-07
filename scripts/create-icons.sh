#!/usr/bin/env bash
# Regenerate the original Waystile app, menu-bar, and browser-extension icons.
set -euo pipefail
root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
swift "$root_dir/scripts/create-waystile-icons.swift" "$root_dir"
