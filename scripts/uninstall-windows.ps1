$ErrorActionPreference = 'Stop'
$taskName = 'NpForFluxer'
$appDir = Join-Path $env:LOCALAPPDATA 'NpForFluxer'
if (Get-ScheduledTask -TaskName $taskName -ErrorAction SilentlyContinue) {
    Stop-ScheduledTask -TaskName $taskName -ErrorAction SilentlyContinue
    Unregister-ScheduledTask -TaskName $taskName -Confirm:$false
}
Write-Host "Removed scheduled task $taskName. Token, binary, and logs remain in $appDir; delete that folder when no longer needed."
