#!/usr/bin/env bash
set -euo pipefail
umask 077
if [[ $(uname -s) != Darwin ]]; then echo 'Run this on macOS.' >&2; exit 1; fi
root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
source_binary="$root/bin/np-for-fluxer"
if [[ ! -f $source_binary ]]; then
  case $(uname -m) in
    x86_64) arch=amd64 ;;
    arm64) arch=arm64 ;;
    *) echo 'Unsupported prebuilt architecture; run: go build -o bin/np-for-fluxer .' >&2; exit 1 ;;
  esac
  source_binary="$root/bin/np-for-fluxer-darwin-$arch"
fi
if [[ ! -f $source_binary ]]; then echo 'Build first: go build -o bin/np-for-fluxer .' >&2; exit 1; fi
app_dir="$HOME/Library/Application Support/NpForFluxer"
agent_dir="$HOME/Library/LaunchAgents"
mkdir -p -- "$app_dir" "$agent_dir"
chmod 700 -- "$app_dir"
temp_binary=$(mktemp "$app_dir/.np-for-fluxer.XXXXXX")
trap 'rm -f -- "$temp_binary"' EXIT
cp "$source_binary" "$temp_binary"
chmod 700 "$temp_binary"
mv -f "$temp_binary" "$app_dir/np-for-fluxer"
token_file="$app_dir/token"
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
xml_escape() { printf '%s' "$1" | sed -e 's/\&/\&amp;/g' -e 's/</\&lt;/g' -e 's/>/\&gt;/g' -e 's/"/\&quot;/g' -e "s/'/\&apos;/g"; }
binary_xml=$(xml_escape "$app_dir/np-for-fluxer")
token_xml=$(xml_escape "$token_file")
log_xml=$(xml_escape "$app_dir/np-for-fluxer.log")
plist="$agent_dir/app.np-for-fluxer.plist"
cat > "$plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>Label</key><string>app.np-for-fluxer</string>
  <key>ProgramArguments</key><array><string>$binary_xml</string><string>-token-file</string><string>$token_xml</string></array>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
  <key>ThrottleInterval</key><integer>30</integer>
  <key>StandardOutPath</key><string>$log_xml</string>
  <key>StandardErrorPath</key><string>$log_xml</string>
</dict></plist>
EOF
launchctl bootout "gui/$UID" "$plist" 2>/dev/null || true
launchctl bootstrap "gui/$UID" "$plist"
echo "Installed and started. Logs: $app_dir/np-for-fluxer.log"
