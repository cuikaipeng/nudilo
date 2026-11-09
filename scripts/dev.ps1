$ErrorActionPreference = "Stop"
Set-Location (Join-Path $PSScriptRoot "..")
go run ./cmd/callbell-assets
if ($LASTEXITCODE -ne 0) { throw "Asset preparation failed" }
$devOS = go env GOOS
if ($LASTEXITCODE -ne 0) { throw "Cannot determine the development platform" }
$devArch = go env GOARCH
if ($LASTEXITCODE -ne 0) { throw "Cannot determine the development architecture" }
$helperName = "callbell-hook"
if ($devOS -eq "windows") { $helperName += ".exe" }
$helperDir = Join-Path "resources" "$devOS-$devArch/bin"
New-Item -ItemType Directory -Force -Path $helperDir | Out-Null
$previousCGOEnabled = $env:CGO_ENABLED
try {
    $env:CGO_ENABLED = "0"
    go build -trimpath -o (Join-Path $helperDir $helperName) ./cmd/callbell-hook
    if ($LASTEXITCODE -ne 0) { throw "Event helper build failed" }
} finally {
    $env:CGO_ENABLED = $previousCGOEnabled
}
go tool mygo dev @args
if ($LASTEXITCODE -ne 0) { throw "Development build failed" }
