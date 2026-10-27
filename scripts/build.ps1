param(
    [string]$FlutterDirectory,
    [string]$JavaDirectory = 'D:\toolchains\jdk17',
    [string]$AndroidSdkDirectory,
    [string]$GoCommand = 'go',
    [string]$LocalHttpHosts = '',
    [switch]$Preview
)
$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot
if (!$FlutterDirectory) { $FlutterDirectory = Join-Path $projectRoot '.tools\flutter' }
if (!$AndroidSdkDirectory) { $AndroidSdkDirectory = Join-Path $projectRoot '.tools\android-sdk' }
$env:PUB_CACHE = Join-Path $projectRoot '.tools\pub-cache'
$env:GRADLE_USER_HOME = Join-Path $projectRoot '.tools\gradle'
$env:ANDROID_USER_HOME = Join-Path $projectRoot '.tools\android-home'
$env:APPDATA = Join-Path $projectRoot '.tools\appdata'
$env:LOCALAPPDATA = Join-Path $projectRoot '.tools\localappdata'
$env:npm_config_cache = Join-Path $projectRoot '.tools\npm-cache'
$env:JAVA_HOME = $JavaDirectory
$env:ANDROID_HOME = $AndroidSdkDirectory
$networkDefines = @("--dart-define=COUPLELINK_LOCAL_HTTP_HOSTS=$LocalHttpHosts")
$flutterCommand = Join-Path $FlutterDirectory 'bin\flutter.bat'
function Assert-Exit([string]$label) { if ($LASTEXITCODE -ne 0) { throw "$label failed with exit code $LASTEXITCODE" } }
Push-Location $projectRoot
try {
    node scripts/generate-assets.mjs
    Assert-Exit 'Assets'
    node scripts/check-contracts.mjs
    Assert-Exit 'Contracts'
    & "$PSScriptRoot\prepare-signing.ps1" -JavaDirectory $JavaDirectory
    Push-Location apps/mobile
    try {
        & $flutterCommand pub get
        Assert-Exit 'Flutter dependencies'
        & $flutterCommand analyze --no-pub
        Assert-Exit 'Static analysis'
        & $flutterCommand test --no-pub
        Assert-Exit 'Flutter tests'
        & $flutterCommand build apk --release --no-pub @networkDefines
        Assert-Exit 'Android APK'
        if ($Preview) {
            & $flutterCommand build web --no-pub --no-wasm-dry-run --no-web-resources-cdn @networkDefines
            Assert-Exit 'Web preview'
        }
    } finally { Pop-Location }
    npm.cmd --prefix apps/band-vela ci --no-audit --no-fund
    Assert-Exit 'Vela dependencies'
    npm.cmd --prefix apps/band-vela run release
    Assert-Exit 'Vela RPK'
    & "$PSScriptRoot\build-server.ps1" -GoCommand $GoCommand
    node scripts/package-artifacts.mjs
    Assert-Exit 'Artifact packaging'
} finally { Pop-Location }
