# Background services

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

