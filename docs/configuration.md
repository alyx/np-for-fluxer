# Configuration and authentication

## Run manually

Install Go 1.22 or newer, then build in this directory:

```sh
go build -o bin/np-for-fluxer .
```

On Windows use `go build -o bin/np-for-fluxer.exe .`.

On macOS use `bash scripts/build-macos.sh`, then run `./bin/NpForFluxer.app/Contents/MacOS/np-for-fluxer` in place of `./bin/np-for-fluxer` below. The bundle includes the usage description required for direct Apple Events access to Spotify. The macOS installer also supplies this bundle metadata when installing a prebuilt executable.

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

