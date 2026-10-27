param(
    [Parameter(Mandatory)][string]$BindAddress,
    [ValidateRange(1,65535)][int]$Port = 18080
)
$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot
New-Item -ItemType Directory -Path (Join-Path $projectRoot '.tools') -Force | Out-Null
$resultPath = Join-Path $projectRoot '.tools\firewall-result.txt'
try {
    if (!(Get-NetIPAddress -AddressFamily IPv4 | Where-Object IPAddress -eq $BindAddress) -or $BindAddress -eq '0.0.0.0') {
        throw 'Choose one IPv4 address owned by this computer'
    }
    $ruleName = "CoupleLink-Local-TCP-$Port"
    $existing = Get-NetFirewallRule -Name $ruleName -ErrorAction SilentlyContinue
    if ($existing) {
        if ($existing.Group -ne 'CoupleLink Local Server') { throw 'Existing rule is not owned by this launcher' }
        Set-NetFirewallRule -Name $ruleName -Enabled True -Direction Inbound -Action Allow -Protocol TCP -LocalAddress $BindAddress -LocalPort $Port -RemoteAddress LocalSubnet -Profile Any | Out-Null
    } else {
        New-NetFirewallRule -Name $ruleName -DisplayName "CoupleLink local API TCP $Port" -Group 'CoupleLink Local Server' -Enabled True -Direction Inbound -Action Allow -Protocol TCP -LocalAddress $BindAddress -LocalPort $Port -RemoteAddress LocalSubnet -Profile Any | Out-Null
    }
    [IO.File]::WriteAllText($resultPath, "Allowed only TCP ${BindAddress}:$Port from LocalSubnet")
} catch {
    [IO.File]::WriteAllText($resultPath, "Firewall configuration failed: $($_.Exception.Message)")
    throw
}
