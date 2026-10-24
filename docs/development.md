# Platform implementation and development

## Platform support

| Platform | Implemented local reader | Requirements / limitations |
| --- | --- | --- |
| Windows | Windows `GlobalSystemMediaTransportControlsSessionManager`, via direct WinRT calls using Go bindings | Windows 10 1809+ / Windows 11. Spotify must expose its media session. Only sessions whose app ID contains Spotify are read. |
| macOS | Direct Apple Events through Foundation's `NSAppleEventDescriptor`, called from Go using `purego` | Spotify desktop installed. Build with `bash scripts/build-macos.sh` to include the Automation usage description. Allow the terminal/app to control Spotify when macOS requests Automation permission. Targets Spotify's running process ID and does not launch it when closed. |
| Linux | Spotify MPRIS metadata and playback state via direct D-Bus (godbus) | Run in the same desktop/session D-Bus as Spotify. Sandboxes must expose Spotify's MPRIS service. |

The browser player is not supported. Remote Spotify Connect playback is only reflected if the local desktop client's media interface reports it. Ads or missing titles may not provide usable song metadata. A Spotify app update or OS media-access setting can affect detection; use `-dry-run` to diagnose it.

The macOS reader sends `core/getd` Apple Events for player state (`pPlS`), current track name (`pnam` of `pTrk`), and artist (`pArt` of `pTrk`). It uses an autorelease pool per poll. Each synchronous send waits at most two seconds, shortened to the remaining context deadline; cancellation is checked between sends and may wait for an in-flight send to finish. Automation denial is reported as an error, preserving the existing status until expiry. No AppleScript or JavaScript is compiled or executed.

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

On macOS, the regular tests also validate native descriptor construction and Unicode conversion. Opt into live Spotify reads with:

```sh
NP_TEST_MAC_MEDIA=1 go test -run TestMacMediaIntegration -v
```

Run this with Spotify playing, paused, and closed, and check `-dry-run` from the built app bundle as well. Verify the Automation prompt and denial behavior for the installed LaunchAgent; its permission identity can differ from Terminal. Native reads and permission prompts must be verified on macOS; cross-compiling from Windows does not exercise them.

## Sources

- [Fluxer HTTP API](https://docs.fluxer.app/http-api/)
- [Fluxer user settings and custom status schema](https://docs.fluxer.app/http-api/users/settings/)
- [Fluxer authentication](https://docs.fluxer.app/authentication/)
- [WinRT Go bindings](https://github.com/saltosystems/winrt-go)
- [Windows media session manager](https://learn.microsoft.com/en-us/uwp/api/windows.media.control.globalsystemmediatransportcontrolssessionmanager)
- [Windows scheduled task settings](https://learn.microsoft.com/en-us/powershell/module/scheduledtasks/new-scheduledtasksettingsset)
- [systemd user services](https://www.freedesktop.org/software/systemd/man/latest/systemd.special.html)
- [Apple LaunchAgents](https://developer.apple.com/library/archive/documentation/MacOSX/Conceptual/BPSystemStartup/Chapters/CreatingLaunchdJobs.html)
- [Apple Event descriptors](https://developer.apple.com/documentation/foundation/nsappleeventdescriptor)
- [Apple Events Automation usage description](https://developer.apple.com/documentation/bundleresources/information-property-list/nsappleeventsusagedescription)
- [purego native calls](https://github.com/ebitengine/purego)
- [MPRIS player specification](https://specifications.freedesktop.org/mpris/latest/Player_Interface.html)
- [godbus Go D-Bus library](https://github.com/godbus/dbus)
- [Spotify currently playing API](https://developer.spotify.com/documentation/web-api/reference/get-the-users-currently-playing-track)
