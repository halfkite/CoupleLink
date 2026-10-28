param([string]$JavaHome = $env:JAVA_HOME, [ValidateSet('Release','Debug')][string]$Configuration = 'Release')
$ErrorActionPreference = 'Stop'
$taskRoot = Split-Path -Parent $PSScriptRoot
if (-not $JavaHome) { throw 'Set JAVA_HOME to a JDK 21 installation.' }
$env:JAVA_HOME = $JavaHome
Push-Location (Join-Path $taskRoot 'apps/mobile-web')
try {
    & npm.cmd run build
    if ($LASTEXITCODE -ne 0) { throw 'Web build failed' }
    & npm.cmd run sync
    if ($LASTEXITCODE -ne 0) { throw 'Android asset sync failed' }
    Push-Location android
    try {
        & ./gradlew.bat "assemble$Configuration" --console=plain
        if ($LASTEXITCODE -ne 0) { throw 'Android build failed' }
    } finally { Pop-Location }
} finally { Pop-Location }
