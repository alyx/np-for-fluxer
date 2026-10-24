#!/usr/bin/env bash
set -euo pipefail
plist="$HOME/Library/LaunchAgents/app.np-for-fluxer.plist"
launchctl bootout "gui/$UID" "$plist" 2>/dev/null || true
rm -f -- "$plist"
echo "LaunchAgent removed. Token, binary, and logs remain under $HOME/Library/Application Support/NpForFluxer; remove them when no longer needed."
