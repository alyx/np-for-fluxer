# Platform implementation and development

## Platform support

| Platform | Implemented local reader | Requirements / limitations |
| --- | --- | --- |
| Windows | Windows `GlobalSystemMediaTransportControlsSessionManager`, via direct WinRT calls using Go bindings | Windows 10 1809+ / Windows 11. Spotify must expose its media session. Only sessions whose app ID contains Spotify are read. |
| macOS | Spotify's scripting interface through JavaScript for Automation (`osascript`) | Spotify desktop installed. Allow the terminal/app to control Spotify when macOS requests Automation permission. Does not launch Spotify when it is closed. |
| Linux | Spotify MPRIS metadata and playback state via direct D-Bus (godbus) | Run in the same desktop/session D-Bus as Spotify. Sandboxes must expose Spotify's MPRIS service. |

The browser player is not supported. Remote Spotify Connect playback is only reflected if the local desktop client's media interface reports it. Ads or missing titles may not provide usable song metadata. A Spotify app update or OS media-access setting can affect detection; use `-dry-run` to diagnose it.

## Validation

```sh
go test ./...
go vet ./...
```

HTTP tests use a local mock server and never contact a real account. Playback readers require manual testing with Spotify on each target platform; cross-compilation alone does not verify OS integration.

On Windows, opt into a read-only integration test of the current desktop media sessions:

```powershell
$env:NP_TEST_WINDOWS_MEDIA = '1'
go test -run TestWindowsMediaIntegration -v
Remove-Item Env:NP_TEST_WINDOWS_MEDIA
```

Run it with Spotify playing, paused, and closed to check each state. It reads three snapshots without controlling playback or contacting Fluxer. The regular polling interval remains five seconds (`-interval` overrides it).

## Sources

- [Fluxer HTTP API](https://docs.fluxer.app/http-api/)
- [Fluxer user settings and custom status schema](https://docs.fluxer.app/http-api/users/settings/)
- [Fluxer authentication](https://docs.fluxer.app/authentication/)
- [WinRT Go bindings](https://github.com/saltosystems/winrt-go)
- [Windows media session manager](https://learn.microsoft.com/en-us/uwp/api/windows.media.control.globalsystemmediatransportcontrolssessionmanager)
- [Windows scheduled task settings](https://learn.microsoft.com/en-us/powershell/module/scheduledtasks/new-scheduledtasksettingsset)
- [systemd user services](https://www.freedesktop.org/software/systemd/man/latest/systemd.special.html)
- [Apple LaunchAgents](https://developer.apple.com/library/archive/documentation/MacOSX/Conceptual/BPSystemStartup/Chapters/CreatingLaunchdJobs.html)
- [Apple JavaScript for Automation](https://developer.apple.com/library/archive/releasenotes/InterapplicationCommunication/RN-JavaScriptForAutomation/)
- [MPRIS player specification](https://specifications.freedesktop.org/mpris/latest/Player_Interface.html)
- [godbus Go D-Bus library](https://github.com/godbus/dbus)
- [Spotify currently playing API](https://developer.spotify.com/documentation/web-api/reference/get-the-users-currently-playing-track)
