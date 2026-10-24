$ErrorActionPreference = 'Stop'
$taskName = 'NpForFluxer'
$appDir = Join-Path $env:LOCALAPPDATA 'NpForFluxer'
$source = Join-Path $PSScriptRoot '..\bin\np-for-fluxer.exe'
$binary = Join-Path $appDir 'np-for-fluxer.exe'
$tokenFile = Join-Path $appDir 'token'
$runner = Join-Path $appDir 'run.ps1'
$user = [Security.Principal.WindowsIdentity]::GetCurrent().Name

if (-not (Test-Path -LiteralPath $source)) { throw "Build the Windows binary first: go build -o bin/np-for-fluxer.exe ." }
New-Item -ItemType Directory -Force -Path $appDir | Out-Null

$existing = Get-ScheduledTask -TaskName $taskName -ErrorAction SilentlyContinue
if ($existing) { Stop-ScheduledTask -TaskName $taskName -ErrorAction SilentlyContinue }
for ($i = 0; $i -lt 20; $i++) {
    $running = Get-Process -Name 'np-for-fluxer' -ErrorAction SilentlyContinue | Where-Object { $_.Path -eq $binary }
    if (-not $running) { break }
    Start-Sleep -Milliseconds 500
}
if ($running) { throw "The old background process is still using $binary. Stop it before updating." }

if ($env:FLUXER_TOKEN) {
    $token = $env:FLUXER_TOKEN.Trim()
} elseif (-not (Test-Path -LiteralPath $tokenFile)) {
    $secret = Read-Host 'Fluxer user session token' -AsSecureString
    $ptr = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($secret)
    try { $token = [Runtime.InteropServices.Marshal]::PtrToStringBSTR($ptr).Trim() }
    finally { [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($ptr) }
}
if ($null -ne $token) {
    if (-not $token) { throw 'Token cannot be empty' }
    [System.IO.File]::WriteAllText($tokenFile, $token, [System.Text.UTF8Encoding]::new($false))
    $token = $null
}
if (-not (Test-Path -LiteralPath $tokenFile)) { throw 'Token file was not created' }
& icacls.exe $tokenFile /inheritance:r /grant:r "${user}:(R,W)" | Out-Null
if ($LASTEXITCODE -ne 0) { throw 'Could not restrict token file access' }

Copy-Item -LiteralPath $source -Destination $binary -Force
@'
$ErrorActionPreference = 'Stop'
& "$PSScriptRoot\np-for-fluxer.exe" -token-file "$PSScriptRoot\token" *>> "$PSScriptRoot\np-for-fluxer.log"
'@ | Set-Content -LiteralPath $runner -Encoding UTF8

$action = New-ScheduledTaskAction -Execute 'powershell.exe' -Argument ('-NoLogo -NoProfile -NonInteractive -WindowStyle Hidden -ExecutionPolicy Bypass -File "{0}"' -f $runner)
$trigger = New-ScheduledTaskTrigger -AtLogOn -User $user
$principal = New-ScheduledTaskPrincipal -UserId $user -LogonType Interactive -RunLevel Limited
$settings = New-ScheduledTaskSettingsSet -ExecutionTimeLimit (New-TimeSpan -Seconds 0) -RestartCount 3 -RestartInterval (New-TimeSpan -Minutes 1) -MultipleInstances IgnoreNew -StartWhenAvailable
Register-ScheduledTask -TaskName $taskName -Action $action -Trigger $trigger -Principal $principal -Settings $settings -Force | Out-Null
Start-ScheduledTask -TaskName $taskName
Write-Host "Installed and started $taskName. Logs: $appDir\np-for-fluxer.log"
