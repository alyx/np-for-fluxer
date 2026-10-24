#!/usr/bin/env bash
set -euo pipefail
if [[ $(uname -s) != Darwin ]]; then echo 'Run this on macOS.' >&2; exit 1; fi
root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
bundle="$root/bin/NpForFluxer.app"
mkdir -p -- "$bundle/Contents/MacOS"
cd -- "$root"
CGO_ENABLED=0 go build -o "$bundle/Contents/MacOS/np-for-fluxer" .
cp "$root/scripts/macos-Info.plist" "$bundle/Contents/Info.plist"
codesign --force --sign - "$bundle"
echo "Built $bundle"
