# Spotify now playing for Fluxer

A small Go command-line app that reads **local Spotify desktop playback** and sets your Fluxer custom status to `🎵 Song — Artist`. No Spotify developer account, API key, or Premium API access is needed. Linux playback uses the godbus Go library, compiled into the executable; no playerctl installation is required.

## Build and run

Install Go 1.22 or newer, then build in this directory:

```sh
go build -o bin/np-for-fluxer .
```

On Windows use `go build -o bin/np-for-fluxer.exe .`.

First test playback detection without credentials or Fluxer writes:

```sh
./bin/np-for-fluxer -dry-run
```

Windows PowerShell:

```powershell
.\bin\np-for-fluxer.exe -dry-run
$env:FLUXER_TOKEN = Read-Host 'Fluxer user session token' -MaskInput
.\bin\np-for-fluxer.exe
```

`-MaskInput` requires PowerShell 7.1+. On Windows PowerShell 5.1, set the environment variable through your preferred secret-management tool instead.

macOS / Linux (Bash):

```bash
read -rsp 'Fluxer user session token: ' FLUXER_TOKEN; echo
export FLUXER_TOKEN
./bin/np-for-fluxer
unset FLUXER_TOKEN
```

The token is a **user session token**, sent unchanged in `Authorization`. Fluxer's documented login/session exchange issues these tokens; bot tokens and ordinary OAuth bearer tokens cannot update a user's settings. Supply a token obtained through an authorized Fluxer client/session workflow. The app does not extract tokens from your installed client or request your password. The background installers save the token in a local file so the job can restart; running the app with `FLUXER_TOKEN` does not save it. Treat the token like your password; never paste it into an issue or commit it.

For a background job, pass `-token-file /path/to/token` instead of setting a persistent environment variable. On macOS/Linux the app refuses a token file that is readable by other users (`chmod 600`). The installers below create this file and restrict its permissions. If `-token-file` is given, it takes priority over `FLUXER_TOKEN`.

## Start automatically in your desktop session

Use the matching binary from `bin/`, or build the native executable for your computer before installing. Run the installer **as your regular desktop user**, not with `sudo` or Administrator. It copies the binary to a stable user directory and asks for your Fluxer user session token if one is not already in `FLUXER_TOKEN` or an existing token file. Re-running an installer updates the binary and restarts the job. Set `FLUXER_TOKEN` first to replace an existing token. Uninstallers stop the job and remove its startup registration; they leave token files and logs for you to delete if desired.

Windows uses a per-user Task Scheduler job at login, which can read Spotify's desktop media session. A traditional Windows service in Session 0 cannot reliably read the signed-in user's media session. PowerShell:

```powershell
go build -o bin/np-for-fluxer.exe .
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\install-windows.ps1
Get-ScheduledTask -TaskName NpForFluxer
Get-Content "$env:LOCALAPPDATA\NpForFluxer\np-for-fluxer.log" -Tail 30 -Wait
# To remove startup:
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\uninstall-windows.ps1
```

Linux uses a `systemd --user` unit, which runs in the same login session as Spotify and its MPRIS D-Bus interface. The executable reads MPRIS directly through the session bus. The unit starts on login and stops on logout; lingering is unnecessary for a desktop media watcher.

```sh
go build -o bin/np-for-fluxer .
bash scripts/install-linux.sh
systemctl --user status np-for-fluxer.service
journalctl --user -u np-for-fluxer.service -f
# To remove startup:
bash scripts/uninstall-linux.sh
```

macOS uses a user LaunchAgent, which starts at login and can request Automation permission to read Spotify. Grant the permission if macOS prompts; if permission was previously denied, check System Settings → Privacy & Security → Automation.

```sh
go build -o bin/np-for-fluxer .
bash scripts/install-macos.sh
tail -f "$HOME/Library/Application Support/NpForFluxer/np-for-fluxer.log"
# To remove startup:
bash scripts/uninstall-macos.sh
```

Use only one copy of the app for a Fluxer account at a time. A manually running copy and a background job can overwrite each other's custom status.

## Platform requirements and investigation

| Platform | Implemented local reader | Requirements / limitations |
| --- | --- | --- |
| Windows | Windows `GlobalSystemMediaTransportControlsSessionManager`, via bundled Windows PowerShell and WinRT | Windows 10 1809+ / Windows 11. Spotify must expose its media session. Only sessions whose app ID contains Spotify are read. |
| macOS | Spotify's scripting interface through JavaScript for Automation (`osascript`) | Spotify desktop installed. Allow the terminal/app to control Spotify when macOS requests Automation permission. Does not launch Spotify when it is closed. |
| Linux | Spotify MPRIS metadata and playback state via direct D-Bus (godbus) | Run in the same desktop/session D-Bus as Spotify. Sandboxes must expose Spotify's MPRIS service. |

The browser player is not supported. Remote Spotify Connect playback is only reflected if the local desktop client's media interface reports it. Ads or missing titles may not provide usable song metadata. A Spotify app update or OS media-access setting can affect detection; use `-dry-run` to diagnose it.

The alternative is Spotify's Web API `GET /v1/me/player/currently-playing` with OAuth scope `user-read-currently-playing`. That supports account-level playback but introduces developer-app setup, token refresh, and Spotify access restrictions. This application deliberately implements local desktop readers instead.

## Behavior

- Polls every five seconds; `-interval 10s` is supported (2–30 seconds).
- Writes only on a song change, plus a refresh once per minute. Status expires after two minutes if the process crashes, loses connectivity, or stops reading playback.
- Clears the status it has set when Spotify pauses/stops/closes, or on Ctrl+C/SIGINT. Reader errors preserve the last status until it expires. Does not clear an existing status before it has published a song. A forced task stop may not deliver SIGINT, so expiry is the fallback.
- **Replaces your existing custom status. It does not restore the previous one.** Avoid changing your custom status manually while this app runs; it owns that field while monitoring.
- Truncates text to 128 Unicode code points. Does not change online/idle/DND status or any other settings.
- Backs off on API failures and respects `Retry-After` / JSON `retry_after`; stops on authentication or permission failures.
- Uses HTTPS. Discovery follows validated, credential-free redirects (including the hosted Fluxer instance's 308 redirect). Status updates refuse redirects. HTTP is accepted only for loopback development instances. Credentials are never logged.

Self-hosted Fluxer:

```sh
./bin/np-for-fluxer -instance https://chat.example.com
# Explicit override if discovery is unavailable:
./bin/np-for-fluxer -api-base https://chat.example.com/api/v1
```

Discovery uses `/.well-known/fluxer` → `endpoints.api_public` and ensures `/v1`. Updates use `PATCH /v1/users/@me/settings` with only `custom_status`. Fluxer republishes presence when settings change, so a Gateway connection is unnecessary.

## Validation

```sh
go test ./...
go vet ./...
```

HTTP tests use a local mock server and never contact a real account. Playback readers require manual testing with Spotify on each target platform; cross-compilation alone does not verify OS integration.

## Sources

- [Fluxer HTTP API](https://docs.fluxer.app/http-api/)
- [Fluxer user settings and custom status schema](https://docs.fluxer.app/http-api/users/settings/)
- [Fluxer authentication](https://docs.fluxer.app/authentication/)
- [Windows media session manager](https://learn.microsoft.com/en-us/uwp/api/windows.media.control.globalsystemmediatransportcontrolssessionmanager)
- [Windows scheduled task settings](https://learn.microsoft.com/en-us/powershell/module/scheduledtasks/new-scheduledtasksettingsset)
- [systemd user services](https://www.freedesktop.org/software/systemd/man/latest/systemd.special.html)
- [Apple LaunchAgents](https://developer.apple.com/library/archive/documentation/MacOSX/Conceptual/BPSystemStartup/Chapters/CreatingLaunchdJobs.html)
- [Apple JavaScript for Automation](https://developer.apple.com/library/archive/releasenotes/InterapplicationCommunication/RN-JavaScriptForAutomation/)
- [MPRIS player specification](https://specifications.freedesktop.org/mpris/latest/Player_Interface.html)
- [godbus Go D-Bus library](https://github.com/godbus/dbus)
- [Spotify currently playing API](https://developer.spotify.com/documentation/web-api/reference/get-the-users-currently-playing-track)
