param([string]$GoCommand = 'go')
$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot
$destination = Join-Path $projectRoot '.tools\server-release'
$webDist = Join-Path $projectRoot 'apps\mobile-web\dist'
$embedDist = Join-Path $projectRoot 'server\internal\webdist\dist'
if (Test-Path $webDist) {
    if (Test-Path $embedDist) { Remove-Item $embedDist -Recurse -Force }
    New-Item -ItemType Directory -Path $embedDist -Force | Out-Null
    Copy-Item (Join-Path $webDist '*') $embedDist -Recurse -Force
}
New-Item -ItemType File -Path (Join-Path $embedDist '.keep') -Force | Out-Null
New-Item -ItemType Directory -Path $destination -Force | Out-Null
$previousOS = $env:GOOS
$previousArch = $env:GOARCH
$previousCGO = $env:CGO_ENABLED
Push-Location (Join-Path $projectRoot 'server')
try {
    & $GoCommand vet ./...
    if ($LASTEXITCODE -ne 0) { throw 'Go static verification failed' }
    & $GoCommand test ./...
    if ($LASTEXITCODE -ne 0) { throw 'Go tests failed' }
    $env:CGO_ENABLED = '0'
    $env:GOARCH = 'amd64'
    foreach ($platform in @('windows','linux')) {
        $env:GOOS = $platform
        $name = if ($platform -eq 'windows') { 'couplelink-windows-amd64.exe' } else { 'couplelink-linux-amd64' }
        & $GoCommand build -trimpath -ldflags='-s -w' -o (Join-Path $destination $name) ./cmd/couplelink
        if ($LASTEXITCODE -ne 0) { throw "Server build failed: $platform" }
    }
} finally {
    $env:GOOS = $previousOS
    $env:GOARCH = $previousArch
    $env:CGO_ENABLED = $previousCGO
    Pop-Location
}
