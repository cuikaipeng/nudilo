param([string]$Platform = "windows/amd64", [switch]$Signed)
$ErrorActionPreference = "Stop"
Set-Location (Join-Path $PSScriptRoot "..")
$packageArgs = @("scripts/release.py", "package", "--platform", $Platform)
if ($Signed) { $packageArgs += "--signed" }
python @packageArgs
if ($LASTEXITCODE -ne 0) { throw "Packaging failed" }
