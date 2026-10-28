param(
    [string]$BindAddress = '127.0.0.1',
    [ValidateRange(1,65535)][int]$Port = 18080,
    [ValidatePattern('^[a-z0-9][a-z0-9_-]*$')][string]$ProjectName = 'couplelink-local'
)
$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot
$parsed = $null
if (![Net.IPAddress]::TryParse($BindAddress, [ref]$parsed) -or $parsed.AddressFamily -ne [Net.Sockets.AddressFamily]::InterNetwork) {
    throw 'Choose an explicit IPv4 address owned by this computer'
}
if ($BindAddress -ne '127.0.0.1' -and !(Get-NetIPAddress -AddressFamily IPv4 | Where-Object IPAddress -eq $BindAddress)) {
    throw 'Bind address does not belong to this computer'
}
$privateDirectory = Join-Path $projectRoot '.secrets'
New-Item -ItemType Directory -Path $privateDirectory -Force | Out-Null
$settingsName = if ($ProjectName -eq 'couplelink-local') { 'local-server.env' } else { "$ProjectName.env" }
$settingsPath = Join-Path $privateDirectory $settingsName
if (!(Test-Path -LiteralPath $settingsPath)) {
    $random = [byte[]]::new(32)
    [Security.Cryptography.RandomNumberGenerator]::Fill($random)
    $password = [Convert]::ToHexString($random).ToLowerInvariant()
    [IO.File]::WriteAllText($settingsPath, "POSTGRES_PASSWORD=$password`n", [Text.UTF8Encoding]::new($false))
}
$passwordLine = [IO.File]::ReadAllLines($settingsPath) | Where-Object { $_ -match '^POSTGRES_PASSWORD=[a-f0-9]{64}$' }
if (@($passwordLine).Count -ne 1) { throw 'Existing private database settings are invalid; do not overwrite them' }
$realtimeLines = @()
foreach ($key in @('COUPLELINK_REALTIME_API_KEY','COUPLELINK_REALTIME_SECRET')) {
    $existing = @([IO.File]::ReadAllLines($settingsPath) | Where-Object { $_.StartsWith("$key=") })
    if ($existing.Count -gt 1 -or ($existing.Count -eq 1 -and $existing[0] -notmatch "^$key=[a-f0-9]{64}$")) { throw 'Existing realtime key is invalid; it will not be overwritten' }
    if ($existing.Count -eq 1) { $realtimeLines += $existing[0] } else {
        $keyBytes = [byte[]]::new(32)
        [Security.Cryptography.RandomNumberGenerator]::Fill($keyBytes)
        $realtimeLines += "$key=$([Convert]::ToHexString($keyBytes).ToLowerInvariant())"
    }
}
[IO.File]::WriteAllText($settingsPath, "$passwordLine`n$($realtimeLines -join "`n")`nCOUPLELINK_BIND_IP=$BindAddress`nCOUPLELINK_PORT=$Port`nCOUPLELINK_ALLOWED_ORIGINS=http://127.0.0.1:5178`n", [Text.UTF8Encoding]::new($false))
& docker compose --env-file $settingsPath -p $ProjectName -f (Join-Path $projectRoot 'deploy\compose.yaml') up -d --build
if ($LASTEXITCODE -ne 0) { throw 'Local server failed to start' }
$ready = $false
$lastHealthError = 'API did not report ready'
for ($attempt = 0; $attempt -lt 10; $attempt++) {
    try {
        $health = Invoke-RestMethod -Uri "http://${BindAddress}:$Port/healthz" -NoProxy -TimeoutSec 3
        if ($health.status -eq 'ok') { $ready = $true; break }
    } catch [System.Net.Http.HttpRequestException] {
        $lastHealthError = $_.Exception.Message
    }
    if ($attempt -lt 9) { Start-Sleep -Seconds 1 }
}
if (!$ready) { throw "Containers started but API health check did not pass: $lastHealthError" }
Write-Output "CoupleLink $($health.version) is ready: http://${BindAddress}:$Port"
Write-Output 'No registration setup code is required. The existing databases of other Compose projects are not used.'
