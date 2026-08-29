# Package a built prima.cpp checkout into shika-engine-<platform>.zip on Windows,
# with llama-server.exe / llama-cli.exe and their DLLs at the top level (the
# layout internal/engine expects).
param(
  [Parameter(Mandatory=$true)][string]$Prima,
  [Parameter(Mandatory=$true)][string]$Platform
)
$ErrorActionPreference = "Stop"

$out = Join-Path (Get-Location) "shika-engine-$Platform.zip"
$stage = Join-Path $env:RUNNER_TEMP "engine-stage"
if (Test-Path $stage) { Remove-Item -Recurse -Force $stage }
New-Item -ItemType Directory -Path $stage | Out-Null

foreach ($bin in @("llama-server.exe", "llama-cli.exe")) {
  $src = Get-ChildItem -Path (Join-Path $Prima "build") -Recurse -Filter $bin | Select-Object -First 1
  if (-not $src) { throw "package-engine: could not find $bin under $Prima\build" }
  Copy-Item $src.FullName (Join-Path $stage $bin)
  # Bundle DLLs sitting next to the binary.
  Get-ChildItem -Path $src.Directory.FullName -Filter *.dll | ForEach-Object {
    Copy-Item $_.FullName (Join-Path $stage $_.Name)
  }
}

if (Test-Path $out) { Remove-Item $out }
Compress-Archive -Path (Join-Path $stage '*') -DestinationPath $out
Write-Host "package-engine: wrote $out"
