param(
    [string]$TunnelName = "neiirohub-vk-bot",
    [string]$Hostname = "vk.neiirohub.ru",
    [string]$LocalUrl = "",
    [ValidateSet("http2", "quic")][string]$TunnelProtocol = "http2",
    [switch]$Login,
    [switch]$SkipDnsRoute
)

$ErrorActionPreference = "Stop"
. (Join-Path $PSScriptRoot "_bot-common.ps1")

function Invoke-Cloudflared {
    param(
        [Parameter(Mandatory = $true)][string[]]$Arguments,
        [switch]$Capture,
        [switch]$SuppressError
    )

    $oldPreference = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    try {
        if ($Capture) {
            if ($SuppressError) {
                return (& $script:CloudflaredPath @Arguments 2>$null)
            }
            return (& $script:CloudflaredPath @Arguments)
        }

        if ($SuppressError) {
            & $script:CloudflaredPath @Arguments 2>$null | Out-Host
        } else {
            & $script:CloudflaredPath @Arguments | Out-Host
        }
    } finally {
        $ErrorActionPreference = $oldPreference
    }
}

function Get-CloudflaredTunnel {
    param([Parameter(Mandatory = $true)][string]$Name)

    $raw = Invoke-Cloudflared -Arguments @("tunnel", "list", "--output", "json") -Capture -SuppressError
    if ($LASTEXITCODE -ne 0) {
        return $null
    }
    if ([string]::IsNullOrWhiteSpace(($raw | Out-String))) {
        return $null
    }

    $items = $raw | ConvertFrom-Json
    foreach ($item in $items) {
        if ([string]$item.name -eq $Name) {
            return $item
        }
    }
    return $null
}

$root = Get-RepoRoot
$runtime = Get-BotRuntimeDir -Root $root
$configPath = Get-NamedTunnelConfigPath -RuntimeDir $runtime
$cloudflaredHome = Join-Path $HOME ".cloudflared"
$originCert = Join-Path $cloudflaredHome "cert.pem"

Ensure-Directory -Path $runtime
Ensure-Directory -Path (Split-Path -Parent $configPath)

$cmd = Get-Command cloudflared -ErrorAction SilentlyContinue | Select-Object -First 1
if ($null -eq $cmd) {
    throw "cloudflared is not installed or not available in PATH."
}
$cmdPreferred = Get-Command cloudflared.cmd -ErrorAction SilentlyContinue | Select-Object -First 1
if ($null -ne $cmdPreferred) {
    $script:CloudflaredPath = $cmdPreferred.Source
} else {
    $script:CloudflaredPath = $cmd.Source
}

if ($Login -or -not (Test-Path -LiteralPath $originCert)) {
    Write-Host "Opening Cloudflare login for tunnel authorization..."
    Invoke-Cloudflared -Arguments @("tunnel", "login")
}

if (-not (Test-Path -LiteralPath $originCert)) {
    throw "Cloudflare origin certificate was not created: $originCert"
}

if ([string]::IsNullOrWhiteSpace($LocalUrl)) {
    $httpAddr = Get-ConfigValue -Root $root -Name "HTTP_ADDR" -Default ":8080"
    $LocalUrl = Convert-ListenAddrToLocalUrl -Addr $httpAddr -Path ""
}

$tunnel = Get-CloudflaredTunnel -Name $TunnelName
if ($null -eq $tunnel) {
    Write-Host "Creating Cloudflare tunnel: $TunnelName"
    Invoke-Cloudflared -Arguments @("tunnel", "create", $TunnelName)
    $tunnel = Get-CloudflaredTunnel -Name $TunnelName
}

if ($null -eq $tunnel) {
    throw "Cloudflare tunnel '$TunnelName' was not found after creation."
}

$tunnelID = [string]$tunnel.id
if ([string]::IsNullOrWhiteSpace($tunnelID)) {
    throw "Cloudflare tunnel '$TunnelName' does not expose an id in cloudflared output."
}

$credentialsFile = Join-Path $cloudflaredHome "$tunnelID.json"
if (-not (Test-Path -LiteralPath $credentialsFile)) {
    $found = Get-ChildItem -Path $cloudflaredHome -Filter "$tunnelID.json" -Recurse -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($null -ne $found) {
        $credentialsFile = $found.FullName
    }
}
if (-not (Test-Path -LiteralPath $credentialsFile)) {
    throw "Cloudflare tunnel credentials file was not found for tunnel id $tunnelID."
}

$credentialsYaml = $credentialsFile.Replace("\", "/")
$configYaml = @"
tunnel: $tunnelID
credentials-file: $credentialsYaml
protocol: $TunnelProtocol

ingress:
  - hostname: $Hostname
    service: $LocalUrl
  - service: http_status:404
"@

Set-Content -Path $configPath -Value $configYaml -Encoding ASCII
Set-Content -Path (Join-Path $runtime "cloudflared-tunnel-name.txt") -Value $TunnelName -Encoding ASCII
Set-Content -Path (Join-Path $runtime "cloudflared-tunnel-hostname.txt") -Value $Hostname -Encoding ASCII
Set-Content -Path (Join-Path $runtime "callback-url.txt") -Value (Get-NamedTunnelCallbackUrl -Hostname $Hostname) -Encoding ASCII

if (-not $SkipDnsRoute) {
    Write-Host "Creating/updating DNS route: $Hostname -> $TunnelName"
    Invoke-Cloudflared -Arguments @("tunnel", "route", "dns", $TunnelName, $Hostname)
}

Write-Host ""
Write-Host "Cloudflare named tunnel is configured."
Write-Host "Tunnel name:   $TunnelName"
Write-Host "Tunnel id:     $tunnelID"
Write-Host "Hostname:      https://$Hostname"
Write-Host "Local service: $LocalUrl"
Write-Host "Config:        $configPath"
Write-Host "VK Callback:   $(Get-NamedTunnelCallbackUrl -Hostname $Hostname)"
Write-Host ""
Write-Host "Start with:"
Write-Host ".\scripts\dev\start-bot.ps1 -TunnelMode named"
