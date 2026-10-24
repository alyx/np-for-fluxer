#!/usr/bin/env bash
set -euo pipefail
systemctl --user disable --now np-for-fluxer.service
rm -f -- "$HOME/.config/systemd/user/np-for-fluxer.service"
systemctl --user daemon-reload
echo "Service removed. Token and binary remain under $HOME/.config/np-for-fluxer and $HOME/.local/bin; remove them when no longer needed."
