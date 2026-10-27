$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot
$binaryDirectory = Join-Path $projectRoot '.tools\pg\node_modules\@embedded-postgres\windows-x64\native\bin'
$dataDirectory = [IO.Path]::GetFullPath((Join-Path $projectRoot '.tools\postgres-r2-data'))
$expectedParent = [IO.Path]::GetFullPath((Join-Path $projectRoot '.tools'))
if ((Split-Path -Parent $dataDirectory) -ne $expectedParent) { throw 'Unexpected test database directory' }
$passwordFile = Join-Path $projectRoot '.secrets\qa-postgres-password.txt'
if (!(Test-Path -LiteralPath $passwordFile)) {
    $random = [byte[]]::new(32)
    [Security.Cryptography.RandomNumberGenerator]::Fill($random)
    [IO.File]::WriteAllText($passwordFile, [Convert]::ToHexString($random).ToLowerInvariant(), [Text.UTF8Encoding]::new($false))
}
if (!(Test-Path -LiteralPath (Join-Path $dataDirectory 'PG_VERSION'))) {
    & (Join-Path $binaryDirectory 'initdb.exe') -D $dataDirectory -U couplelink -A scram-sha-256 --pwfile=$passwordFile --encoding=UTF8 --locale=C
    if ($LASTEXITCODE -ne 0) { throw 'Test PostgreSQL initialization failed' }
}
& (Join-Path $binaryDirectory 'pg_ctl.exe') -D $dataDirectory status
if ($LASTEXITCODE -ne 0) {
    & (Join-Path $binaryDirectory 'pg_ctl.exe') -D $dataDirectory -l (Join-Path $projectRoot '.tools\postgres-r2.log') -o '-h 127.0.0.1 -p 55433' -w -t 30 start
    if ($LASTEXITCODE -ne 0) { throw 'Test PostgreSQL failed to start' }
}
$env:PGPASSWORD = [IO.File]::ReadAllText($passwordFile).Trim()
$env:COUPLELINK_TEST_DATABASE_URL = "postgresql://couplelink:$($env:PGPASSWORD)@127.0.0.1:55433/couplelink_test?sslmode=disable"
$env:COUPLELINK_TEST_POSTGRES_ROOT = $projectRoot
node (Join-Path $PSScriptRoot 'prepare-test-db.cjs')
if ($LASTEXITCODE -ne 0) { throw 'Test database creation failed' }
Remove-Item Env:PGPASSWORD
Remove-Item Env:COUPLELINK_TEST_POSTGRES_ROOT
Write-Output 'Project-local PostgreSQL is ready at 127.0.0.1:55433; no Docker or global database service is used.'
