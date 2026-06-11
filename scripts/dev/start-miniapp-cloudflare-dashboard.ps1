param(
    [switch]$NoRestart,
    [switch]$NoWait,
    [switch]$OpenBrowser,
    [int]$ApiPort = 8080,
    [int]$MiniAppPort = 5173,
    [int]$WorkerMetricsPort = 9090,
    [int]$VkUserID = 777,
    [int]$TimeoutSeconds = 120,
    [string]$Hostname = "app.neiirohub.ru",
    [ValidateSet("auto", "quic", "http2")]
    [string]$Protocol = "http2",
    [switch]$RememberToken
)

$ErrorActionPreference = "Stop"
. (Join-Path $PSScriptRoot "_miniapp-common.ps1")

function Get-TunnelToken {
    param([Parameter(Mandatory = $true)][string]$TokenFile)

    if (-not [string]::IsNullOrWhiteSpace($env:CLOUDFLARE_TUNNEL_TOKEN)) {
        return $env:CLOUDFLARE_TUNNEL_TOKEN.Trim()
    }

    if (Test-Path -LiteralPath $TokenFile) {
        $saved = Get-Content -LiteralPath $TokenFile -ErrorAction Stop | Select-Object -First 1
        if (-not [string]::IsNullOrWhiteSpace($saved)) {
            return $saved.Trim()
        }
    }

    $secure = Read-Host "Paste Cloudflare tunnel token" -AsSecureString
    $bstr = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($secure)
    try {
        return [Runtime.InteropServices.Marshal]::PtrToStringBSTR($bstr)
    } finally {
        if ($bstr -ne [IntPtr]::Zero) {
            [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($bstr)
        }
    }
}

function Wait-CloudflaredLog {
    param(
        [Parameter(Mandatory = $true)][string]$LogPath,
        [Parameter(Mandatory = $true)][string]$ErrPath,
        [int]$TimeoutSeconds = 60
    )

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    do {
        foreach ($path in @($LogPath, $ErrPath)) {
            if (-not (Test-Path -LiteralPath $path)) {
                continue
            }
            $text = Get-Content -LiteralPath $path -Raw -ErrorAction SilentlyContinue
            if ($text -match "Registered tunnel connection|Connection .* registered|Connected to") {
                return $true
            }
            if ($text -match "failed|error|invalid|unauthorized") {
                return $false
            }
        }
        Start-Sleep -Milliseconds 500
    } while ((Get-Date) -lt $deadline)

    return $false
}

function Test-PublicMetricsExposure {
    param([Parameter(Mandatory = $true)][string]$ProbeHostname)

    $url = "https://$ProbeHostname/metrics"
    $status = 0
    $contentType = ""
    $exposed = $false
    try {
        $resp = Invoke-WebRequest -Uri $url -UseBasicParsing -TimeoutSec 10 -MaximumRedirection 3
        $status = [int]$resp.StatusCode
        $contentType = [string]$resp.Headers["Content-Type"]
        $body = [string]$resp.Content
        $exposed = ($status -ge 200 -and $status -lt 300 -and $body -match "(?m)^# HELP (vkagg_|payment_|go_|process_)")
    } catch [System.Net.WebException] {
        if ($_.Exception.Response -ne $null) {
            $status = [int]$_.Exception.Response.StatusCode
            $contentType = [string]$_.Exception.Response.Headers["Content-Type"]
        }
    } catch {
        return [pscustomobject]@{
            Url = $url
            Status = "check_failed"
            ContentType = ""
            Exposed = $false
        }
    }

    return [pscustomobject]@{
        Url = $url
        Status = $status
        ContentType = $contentType
        Exposed = $exposed
    }
}

$root = Get-RepoRoot
$runtime = Get-MiniAppRuntimeDir -Root $root
Ensure-Directory -Path $runtime
Import-MiniAppDevEnv -Root $root
$tokenFile = Join-Path $runtime "cloudflare-token.txt"

$cloudflared = Get-Command cloudflared -ErrorAction SilentlyContinue | Select-Object -First 1
if ($null -eq $cloudflared) {
    throw "cloudflared is not installed or not available in PATH."
}

$existingCloudflared = Get-Process cloudflared -ErrorAction SilentlyContinue
if ($existingCloudflared) {
    $ids = ($existingCloudflared | ForEach-Object { $_.Id }) -join ", "
    Write-Warning "Existing cloudflared process(es) detected: $ids. Stop stale connectors first if this tunnel returns 502."
}

$cloudflaredService = Get-Service cloudflared -ErrorAction SilentlyContinue
if ($cloudflaredService -and $cloudflaredService.Status -eq "Running") {
    Write-Warning "Windows service 'cloudflared' is running. If it uses the same tunnel, Cloudflare may route traffic to a stale connector."
}

$token = Get-TunnelToken -TokenFile $tokenFile
if ([string]::IsNullOrWhiteSpace($token)) {
    throw "Cloudflare tunnel token is empty."
}
if ($RememberToken -and -not (Test-Path -LiteralPath $tokenFile)) {
    Set-Content -LiteralPath $tokenFile -Value $token -Encoding ASCII
}

Push-Location $root
try {
    & (Join-Path $PSScriptRoot "start-miniapp.ps1") `
        -NoTunnel `
        -NoWait `
        -NoRestart:$NoRestart `
        -ApiPort $ApiPort `
        -MiniAppPort $MiniAppPort `
        -WorkerMetricsPort $WorkerMetricsPort `
        -VkUserID $VkUserID `
        -TimeoutSeconds $TimeoutSeconds

    Import-MiniAppDevEnv -Root $root
    $launchParams = New-MiniAppLaunchParams -UserID $VkUserID
    $publicUrl = "https://$Hostname"
    $launchUrl = "$publicUrl/?$launchParams"

    $stdout = Join-Path $runtime "cloudflared-dashboard-live.log"
    $stderr = Join-Path $runtime "cloudflared-dashboard-live.err"

    Write-Host "Starting Cloudflare dashboard-managed tunnel for $publicUrl..."
    $cloudflaredArgs = @("tunnel", "run")
    if ($Protocol -ne "auto") {
        $cloudflaredArgs = @("tunnel", "--protocol", $Protocol, "run")
    }

    $oldCloudflareTunnelToken = $env:CLOUDFLARE_TUNNEL_TOKEN
    $oldTunnelToken = $env:TUNNEL_TOKEN
    Remove-Item Env:\CLOUDFLARE_TUNNEL_TOKEN -ErrorAction SilentlyContinue
    $env:TUNNEL_TOKEN = $token
    try {
        $tunnelProc = Start-Process -FilePath $cloudflared.Source `
            -ArgumentList $cloudflaredArgs `
            -WorkingDirectory $root `
            -WindowStyle Hidden `
            -RedirectStandardOutput $stdout `
            -RedirectStandardError $stderr `
            -PassThru
    } finally {
        if (-not [string]::IsNullOrWhiteSpace($oldCloudflareTunnelToken)) {
            $env:CLOUDFLARE_TUNNEL_TOKEN = $oldCloudflareTunnelToken
        }
        if (-not [string]::IsNullOrWhiteSpace($oldTunnelToken)) {
            $env:TUNNEL_TOKEN = $oldTunnelToken
        } else {
            Remove-Item Env:\TUNNEL_TOKEN -ErrorAction SilentlyContinue
        }
    }
    Set-Content -Path (Join-Path $runtime "tunnel.pid") -Value $tunnelProc.Id -Encoding ASCII
    Set-Content -Path (Join-Path $runtime "frontend-url.txt") -Value $publicUrl -Encoding ASCII
    Set-Content -Path (Join-Path $runtime "miniapp-cloudflare-launch.url") -Encoding ASCII -Value "[InternetShortcut]`r`nURL=$launchUrl`r`n"

    $connected = Wait-CloudflaredLog -LogPath $stdout -ErrPath $stderr -TimeoutSeconds 60

    Write-Host ""
    Write-Host "VK Mini App Cloudflare stack is running."
    Write-Host "Public URL:    $publicUrl"
    Write-Host "Launch URL:    $launchUrl"
    Write-Host "API health:    http://127.0.0.1:$ApiPort/health"
    Write-Host "Vite local:    http://127.0.0.1:$MiniAppPort"
    Write-Host "Worker health: http://127.0.0.1:$WorkerMetricsPort/healthz"
    Write-Host "Logs:          $runtime"
    Write-Host "Tunnel log:    $stdout"
    if ($connected) {
        Write-Host "Tunnel status: connected or registering"
    } else {
        Write-Host "Tunnel status: not confirmed; inspect tunnel logs and Cloudflare Dashboard"
    }
    $metricsProbe = Test-PublicMetricsExposure -ProbeHostname $Hostname
    if ($metricsProbe.Exposed) {
        Write-Warning "Security check failed: public /metrics is exposed at $($metricsProbe.Url) (status $($metricsProbe.Status)). Close it in Cloudflare Public Hostname/WAF before sharing the Mini App."
    } else {
        Write-Host "Public /metrics: not exposed (status $($metricsProbe.Status))"
    }

    if ($OpenBrowser) {
        Start-Process $launchUrl
    }

    if ($NoWait) {
        exit 0
    }

    Read-Host "Press Enter to stop API, worker, Vite and Cloudflare tunnel"
    & (Join-Path $PSScriptRoot "stop-miniapp.ps1")
} finally {
    Pop-Location
}
