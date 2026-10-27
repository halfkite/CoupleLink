param(
    [string]$JavaDirectory = 'D:\toolchains\jdk17',
    [string]$OpenSsl = 'C:\Program Files\Git\usr\bin\openssl.exe'
)
$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot
$secretDirectory = Join-Path $projectRoot '.secrets'
$propertiesPath = Join-Path $secretDirectory 'signing.properties'
$keystorePath = Join-Path $secretDirectory 'couplelink.jks'
New-Item -ItemType Directory -Force -Path $secretDirectory | Out-Null
if (!(Test-Path -LiteralPath $propertiesPath)) {
    if (Test-Path -LiteralPath $keystorePath) { throw 'Existing signing key has no properties. Restore its matching credentials; do not replace the key.' }
    $randomBytes = [byte[]]::new(32)
    [System.Security.Cryptography.RandomNumberGenerator]::Fill($randomBytes)
    $password = [Convert]::ToBase64String($randomBytes)
    $alias = 'couplelink'
    $env:COUPLELINK_SIGN_PASSWORD = $password
    & (Join-Path $JavaDirectory 'bin\keytool.exe') -genkeypair -keystore $keystorePath -storetype JKS -storepass:env COUPLELINK_SIGN_PASSWORD -keypass:env COUPLELINK_SIGN_PASSWORD -alias $alias -keyalg RSA -keysize 2048 -validity 10000 -dname 'CN=CoupleLink, OU=Personal, O=CoupleLink, C=CN'
    if ($LASTEXITCODE -ne 0) { throw 'Signing key generation failed.' }
    $content = "storeFile=../../../.secrets/couplelink.jks`nstorePassword=$password`nkeyAlias=$alias`nkeyPassword=$password`n"
    [IO.File]::WriteAllText($propertiesPath, $content, [Text.UTF8Encoding]::new($false))
    Remove-Item Env:COUPLELINK_SIGN_PASSWORD
}
$values = @{}
Get-Content -LiteralPath $propertiesPath | ForEach-Object { $parts = $_.Split('=',2); if ($parts.Length -eq 2) { $values[$parts[0]] = $parts[1] } }
$env:COUPLELINK_SIGN_PASSWORD = $values['storePassword']
$p12Path = Join-Path $secretDirectory 'couplelink.p12'
if (!(Test-Path -LiteralPath $p12Path)) {
    & (Join-Path $JavaDirectory 'bin\keytool.exe') -importkeystore -srckeystore $keystorePath -srcstoretype JKS -srcstorepass:env COUPLELINK_SIGN_PASSWORD -srckeypass:env COUPLELINK_SIGN_PASSWORD -srcalias $values['keyAlias'] -destkeystore $p12Path -deststoretype PKCS12 -deststorepass:env COUPLELINK_SIGN_PASSWORD -destkeypass:env COUPLELINK_SIGN_PASSWORD -noprompt
    if ($LASTEXITCODE -ne 0) { throw 'Signing key conversion failed.' }
}
$releaseDirectory = Join-Path $projectRoot 'apps\band-vela\sign\release'
New-Item -ItemType Directory -Force -Path $releaseDirectory | Out-Null
& $OpenSsl pkcs12 -in $p12Path -passin env:COUPLELINK_SIGN_PASSWORD -nocerts -nodes -out (Join-Path $releaseDirectory 'private.pem')
if ($LASTEXITCODE -ne 0) { throw 'Vela private key extraction failed.' }
& $OpenSsl pkcs12 -in $p12Path -passin env:COUPLELINK_SIGN_PASSWORD -clcerts -nokeys -out (Join-Path $releaseDirectory 'certificate.pem')
if ($LASTEXITCODE -ne 0) { throw 'Vela certificate extraction failed.' }
Remove-Item Env:COUPLELINK_SIGN_PASSWORD
Write-Output 'Local signing prepared. APK and RPK use the same certificate. Back up .secrets privately.'
