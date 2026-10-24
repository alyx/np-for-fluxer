# Spotify now playing for Fluxer

Sets your Fluxer custom status to `🎵 Song — Artist` from local Spotify desktop playback on Windows, macOS, or Linux. No Spotify API key or developer account required.

Playback is read directly through WinRT on Windows, Apple Events on macOS, and D-Bus on Linux. Polling stays at five seconds by default, with no playback helper subprocesses.

## Quick start

Install Go 1.22+, open Spotify desktop, and run from this directory.

**Windows (PowerShell 7.1+):**

```powershell
go build -o bin/np-for-fluxer.exe .
.\bin\np-for-fluxer.exe -dry-run
$env:FLUXER_TOKEN = Read-Host 'Fluxer user session token' -MaskInput
.\bin\np-for-fluxer.exe
```

**macOS / Linux (Bash):**

```bash
if [[ $(uname -s) == Darwin ]]; then
  bash scripts/build-macos.sh
  binary=./bin/NpForFluxer.app/Contents/MacOS/np-for-fluxer
else
  go build -o bin/np-for-fluxer .
  binary=./bin/np-for-fluxer
fi
"$binary" -dry-run
read -rsp 'Fluxer user session token: ' FLUXER_TOKEN; echo
export FLUXER_TOKEN
"$binary"
```

`-dry-run` checks playback without contacting Fluxer. Use a **Fluxer user session token**, not a bot token; keep it private. See [configuration](docs/configuration.md) for token files, self-hosted instances, and polling options.

The app replaces your custom status and clears it when playback stops; it does not restore your previous status. Run one copy per account. Spotify's browser player is unsupported.

## Deploy as a background service

Build as above, then run the matching installer **as your regular desktop user** (no sudo or Administrator). It prompts for a token, saves it with restricted permissions, and starts the app now and at login.

| Platform | Install | Runs as |
| --- | --- | --- |
| Windows | `powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\install-windows.ps1` | Task Scheduler job |
| Linux | `bash scripts/install-linux.sh` | systemd user service |
| macOS | `bash scripts/install-macos.sh` | LaunchAgent |

The app needs your desktop session to read Spotify. On Windows, use the scheduled task rather than a system service. On macOS, allow Automation access to Spotify when prompted. The build script and installer create an app bundle with the required Automation usage description; Go calls the native APIs through `purego`, without cgo or Xcode build tools.

Rebuild and rerun the installer to update. Use the matching `scripts/uninstall-*` script to remove startup registration; token files and logs remain.

[Service management and logs](docs/services.md) · [Configuration and authentication](docs/configuration.md) · [Platform details and testing](docs/development.md)
