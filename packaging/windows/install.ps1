# shikA installer for Windows (PowerShell).
#
#   irm https://raw.githubusercontent.com/braymix/shika/main/packaging/windows/install.ps1 | iex
#
# Downloads shikad.exe, installs it under %LOCALAPPDATA%\shikA, and registers a
# per-user scheduled task that starts it at logon (no admin required). For a true
# system service, install NSSM and point it at the exe instead.

param(
  [string]$Repo    = $(if ($env:SHIKA_REPO)    { $env:SHIKA_REPO }    else { "braymix/shika" }),
  [string]$Version = $(if ($env:SHIKA_VERSION) { $env:SHIKA_VERSION } else { "latest" })
)

$ErrorActionPreference = "Stop"
$arch  = if ([Environment]::Is64BitOperatingSystem) { "amd64" } else { throw "shikA needs 64-bit Windows" }
$asset = "shikad-windows-$arch.exe"

if ($Version -eq "latest") {
  $url = "https://github.com/$Repo/releases/latest/download/$asset"
} else {
  $url = "https://github.com/$Repo/releases/download/$Version/$asset"
}

$dir = Join-Path $env:LOCALAPPDATA "shikA"
New-Item -ItemType Directory -Force -Path $dir | Out-Null
$exe = Join-Path $dir "shikad.exe"

Write-Host "shikA: downloading $asset ($Version)" -ForegroundColor Cyan
Invoke-WebRequest -Uri $url -OutFile $exe -UseBasicParsing

# Register a logon scheduled task so shikad runs in the background.
$taskName = "shikA orchestrator"
$action   = New-ScheduledTaskAction -Execute $exe
$trigger  = New-ScheduledTaskTrigger -AtLogOn
$settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -RestartCount 3 -RestartInterval (New-TimeSpan -Minutes 1)
Register-ScheduledTask -TaskName $taskName -Action $action -Trigger $trigger -Settings $settings -Force | Out-Null
Start-ScheduledTask -TaskName $taskName

Write-Host "shikA: installed to $exe and started." -ForegroundColor Green
Write-Host "shikA: dashboard on http://localhost:8977" -ForegroundColor Green
