#!/usr/bin/env bash
set -euo pipefail
umask 077
if [[ $(uname -s) != Linux ]]; then echo 'Run this on Linux.' >&2; exit 1; fi
root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
source_binary="$root/bin/np-for-fluxer"
if [[ ! -f $source_binary ]]; then
  case $(uname -m) in
    x86_64) arch=amd64 ;;
    aarch64|arm64) arch=arm64 ;;
    *) echo 'Unsupported prebuilt architecture; run: go build -o bin/np-for-fluxer .' >&2; exit 1 ;;
  esac
  source_binary="$root/bin/np-for-fluxer-linux-$arch"
fi
if [[ ! -f $source_binary ]]; then echo 'Build first: go build -o bin/np-for-fluxer .' >&2; exit 1; fi
bin_dir="$HOME/.local/bin"
config_dir="$HOME/.config/np-for-fluxer"
unit_dir="$HOME/.config/systemd/user"
mkdir -p -- "$bin_dir" "$config_dir" "$unit_dir"
chmod 700 -- "$config_dir"
temp_binary=$(mktemp "$bin_dir/.np-for-fluxer.XXXXXX")
trap 'rm -f -- "$temp_binary"' EXIT
install -m 700 -- "$source_binary" "$temp_binary"
mv -f -- "$temp_binary" "$bin_dir/np-for-fluxer"
token_file="$config_dir/token"
if [[ -n ${FLUXER_TOKEN:-} ]]; then
  printf '%s' "$FLUXER_TOKEN" > "$token_file"
elif [[ ! -f $token_file ]]; then
  read -r -s -p 'Fluxer user session token: ' token
  printf '\n'
  [[ -n $token ]] || { echo 'Token cannot be empty' >&2; exit 1; }
  printf '%s' "$token" > "$token_file"
  unset token
fi
chmod 600 -- "$token_file"
unit="$unit_dir/np-for-fluxer.service"
# Percent signs have special meaning in systemd unit files.
service_binary=${bin_dir//%/%%}/np-for-fluxer
service_token=${token_file//%/%%}
cat > "$unit" <<EOF
[Unit]
Description=Spotify now playing for Fluxer
After=graphical-session.target
StartLimitIntervalSec=60
StartLimitBurst=3

[Service]
Type=simple
ExecStart="$service_binary" -token-file "$service_token"
Restart=on-failure
RestartSec=10
KillSignal=SIGINT
TimeoutStopSec=10

[Install]
WantedBy=default.target
EOF
systemctl --user daemon-reload
systemctl --user enable np-for-fluxer.service
systemctl --user restart np-for-fluxer.service
echo 'Installed. Logs: journalctl --user -u np-for-fluxer.service -f'
