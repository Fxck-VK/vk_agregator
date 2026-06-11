param(
    [string]$TunnelName = "neiirohub-vk-bot",
    [string]$Hostname = "app.neiirohub.ru",
    [string]$LocalUrl = "http://localhost:5173",
    [string]$TunnelConfigPath = "",
    [switch]$SkipDnsRoute
)

$ErrorActionPreference = "Stop"
. (Join-Path $PSScriptRoot "_bot-common.ps1")

function Invoke-Cloudflared {
    param(
        [Parameter(Mandatory = $true)][string[]]$Arguments
    )

    $cmd = Get-Command cloudflared -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($null -eq $cmd) {
        throw "cloudflared is not installed or not available in PATH."
    }
    & $cmd.Source @Arguments | Out-Host
}

function Add-OrUpdate-IngressRoute {
    param(
        [Parameter(Mandatory = $true)][string]$ConfigPath,
        [Parameter(Mandatory = $true)][string]$RouteHostname,
        [Parameter(Mandatory = $true)][string]$RouteService
    )

    if (-not (Test-Path -LiteralPath $ConfigPath)) {
        throw "Cloudflare tunnel config was not found: $ConfigPath. Run .\scripts\dev\setup-cloudflare-tunnel.ps1 -Login first."
    }

    $lines = [System.Collections.Generic.List[string]]::new()
    foreach ($line in Get-Content -LiteralPath $ConfigPath) {
        $lines.Add($line)
    }

    $existingIndex = -1
    for ($i = 0; $i -lt $lines.Count; $i++) {
        if ($lines[$i].Trim() -eq "- hostname: $RouteHostname") {
            $existingIndex = $i
            break
        }
    }

    if ($existingIndex -ge 0) {
        for ($i = $existingIndex + 1; $i -lt [Math]::Min($existingIndex + 4, $lines.Count); $i++) {
            if ($lines[$i].Trim().StartsWith("service:")) {
                $lines[$i] = "    service: $RouteService"
                Set-Content -LiteralPath $ConfigPath -Value $lines -Encoding ASCII
                return
            }
        }
        throw "Found route for $RouteHostname, but could not find its service line."
    }

    $catchAllIndex = -1
    for ($i = 0; $i -lt $lines.Count; $i++) {
        if ($lines[$i].Trim() -eq "- service: http_status:404") {
            $catchAllIndex = $i
            break
        }
    }
    if ($catchAllIndex -lt 0) {
        throw "Could not find Cloudflare tunnel catch-all route '- service: http_status:404'."
    }

    $routeLines = [string[]]@(
        "  - hostname: $RouteHostname",
        "    service: $RouteService"
    )
    $lines.InsertRange($catchAllIndex, $routeLines)
    Set-Content -LiteralPath $ConfigPath -Value $lines -Encoding ASCII
}

function Add-OrUpdate-MetricsDenyRoute {
    param(
        [Parameter(Mandatory = $true)][string]$ConfigPath,
        [Parameter(Mandatory = $true)][string]$RouteHostname
    )

    if (-not (Test-Path -LiteralPath $ConfigPath)) {
        throw "Cloudflare tunnel config was not found: $ConfigPath. Run .\scripts\dev\setup-cloudflare-tunnel.ps1 -Login first."
    }

    $original = [System.Collections.Generic.List[string]]::new()
    foreach ($line in Get-Content -LiteralPath $ConfigPath) {
        $original.Add($line)
    }

    $lines = [System.Collections.Generic.List[string]]::new()
    for ($i = 0; $i -lt $original.Count; ) {
        if ($original[$i].Trim() -eq "- hostname: $RouteHostname") {
            $end = $i + 1
            $isMetricsDeny = $false
            while ($end -lt $original.Count -and -not $original[$end].TrimStart().StartsWith("- ")) {
                if ($original[$end].Trim().StartsWith("path: ^/metrics")) {
                    $isMetricsDeny = $true
                }
                $end++
            }
            if ($isMetricsDeny) {
                $i = $end
                continue
            }
        }
        $lines.Add($original[$i])
        $i++
    }

    $insertIndex = -1
    for ($i = 0; $i -lt $lines.Count; $i++) {
        if ($lines[$i].Trim() -eq "- hostname: $RouteHostname") {
            $insertIndex = $i
            break
        }
    }
    if ($insertIndex -lt 0) {
        for ($i = 0; $i -lt $lines.Count; $i++) {
            if ($lines[$i].Trim() -eq "- service: http_status:404") {
                $insertIndex = $i
                break
            }
        }
    }
    if ($insertIndex -lt 0) {
        throw "Could not find Cloudflare tunnel route insertion point."
    }

    $routeLines = [string[]]@(
        "  - hostname: $RouteHostname",
        '    path: ^/metrics(?:$|/)',
        "    service: http_status:404"
    )
    $lines.InsertRange($insertIndex, $routeLines)
    Set-Content -LiteralPath $ConfigPath -Value $lines -Encoding ASCII
}

$root = Get-RepoRoot
$runtime = Get-BotRuntimeDir -Root $root
if ([string]::IsNullOrWhiteSpace($TunnelConfigPath)) {
    $TunnelConfigPath = Get-ConfigValue -Root $root -Name "VK_BOT_TUNNEL_CONFIG" -Default (Get-NamedTunnelConfigPath -RuntimeDir $runtime)
}

Add-OrUpdate-IngressRoute -ConfigPath $TunnelConfigPath -RouteHostname $Hostname -RouteService $LocalUrl
Add-OrUpdate-MetricsDenyRoute -ConfigPath $TunnelConfigPath -RouteHostname $Hostname

if (-not $SkipDnsRoute) {
    Write-Host "Creating/updating DNS route: $Hostname -> $TunnelName"
    Invoke-Cloudflared -Arguments @("tunnel", "route", "dns", $TunnelName, $Hostname)
}

Write-Host ""
Write-Host "Mini App Cloudflare route is configured."
Write-Host "Hostname:      https://$Hostname"
Write-Host "Local service: $LocalUrl"
Write-Host "Config:        $TunnelConfigPath"
