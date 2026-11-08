#!/bin/bash
# Package an existing bundle, preserving any release signature.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
APP="${1:-$ROOT/bin/dym-bridge.app}"
OUTPUT="${2:-$ROOT/bin/dym-bridge.dmg}"
PYTHON="${DMG_PYTHON:-$ROOT/bin/dmg-venv/bin/python}"

if [[ "$(uname -s)" != Darwin ]]; then
    echo "DMG packaging requires macOS." >&2
    exit 1
fi
if [[ ! -d "$APP/Contents" ]]; then
    echo "App bundle not found: $APP (run make bundle first)." >&2
    exit 1
fi
if [[ ! -x "$PYTHON" ]]; then
    echo "Run make dmg-deps first, or set DMG_PYTHON to a Python with dmgbuild and Pillow." >&2
    exit 1
fi

mkdir -p "$(dirname "$OUTPUT")"
WORK="$(mktemp -d "${TMPDIR:-/tmp}/bridge-dmg.XXXXXX")"
trap 'rm -rf "$WORK"' EXIT
"$PYTHON" "$ROOT/packaging/macos/dmg/background.py" "$WORK/background.png"
# dmgbuild writes Finder metadata directly, so CI needs no Finder automation.
"$PYTHON" -m dmgbuild -s "$ROOT/packaging/macos/dmg/settings.py" \
    -D "app=$APP" -D "background=$WORK/background.png" \
    "Dym Bridge" "$WORK/dym-bridge.dmg"
# Leave any previous output intact if packaging fails.
mv -f "$WORK/dym-bridge.dmg" "$OUTPUT"
echo "Created $OUTPUT"
