param(
    [switch]$StatusOnly,
    [switch]$StopOnly,
    [switch]$StopDocker,
    [switch]$NoCloudflare,
    [switch]$NoRestart,
    [switch]$NoWait,
    [switch]$OpenGrafana,
    [switch]$OpenMiniApp,
    [switch]$OpenAdmin,
    [switch]$UseEnvProviders,
    [switch]$RealVkDelivery,
    [switch]$ResetDevData,
    [ValidateSet("mock", "yookassa")]
    [string]$PaymentProvider = "mock",
    [ValidateSet("auto", "quic", "http2")]
    [string]$CloudflareProtocol = "http2",
    [string[]]$PublicHostnames = @("local-app.neiirohub.ru", "local-vk.neiirohub.ru", "local-pay.neiirohub.ru"),
    [int]$ApiPort = 8080,
    [int]$WorkerMetricsPort = 9090,
    [int]$ProviderWebhookPort = 8082,
    [int]$MiniAppPort = 5173,
    [int]$AdminPort = 5175,
    [int]$GrafanaPort = 3000,
    [int]$PrometheusPort = 9091,
    [int]$AlertmanagerPort = 9093,
    [int]$OtelGrpcPort = 4317,
    [int]$VkUserID = 777,
    [int]$TimeoutSeconds = 180
)

$ErrorActionPreference = "Stop"
. (Join-Path $PSScriptRoot "_miniapp-common.ps1")

function Get-AllRuntimeDir {
    param([Parameter(Mandatory = $true)][string]$Root)

    return (Join-Path $Root ".runtime\start-all")
}

function Invoke-DevScript {
    param(
        [Parameter(Mandatory = $true)][string]$ScriptPath,
        [Parameter(Mandatory = $true)][string[]]$Arguments
    )

    $powershell = (Get-Command powershell.exe -ErrorAction Stop | Select-Object -First 1).Source
    & $powershell @("-NoProfile", "-ExecutionPolicy", "Bypass", "-File", $ScriptPath) @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "$ScriptPath failed with exit code $LASTEXITCODE."
    }
}

function Get-PidFileState {
    param([Parameter(Mandatory = $true)][string]$PidFile)

    if (-not (Test-Path -LiteralPath $PidFile)) {
        return "not running"
    }

    $raw = Get-Content -LiteralPath $PidFile -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($raw -notmatch '^\d+$') {
        return "invalid pid file"
    }

    try {
        $proc = Get-Process -Id ([int]$raw) -ErrorAction Stop
        return "running pid=$($proc.Id)"
    } catch {
        return "not running stale_pid=$raw"
    }
}

function Wait-CloudflaredReady {
    param(
        [Parameter(Mandatory = $true)][string]$StdoutPath,
        [Parameter(Mandatory = $true)][string]$StderrPath,
        [int]$TimeoutSeconds = 60
    )

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    do {
        foreach ($path in @($StdoutPath, $StderrPath)) {
            if (-not (Test-Path -LiteralPath $path)) {
                continue
            }
            $text = Get-Content -LiteralPath $path -Raw -ErrorAction SilentlyContinue
            if ($text -match "Registered tunnel connection|Connection .* registered|Connected to") {
                return $true
            }
            if ($text -match "(?i)invalid token|unauthorized|failed to authenticate") {
                return $false
            }
        }
        Start-Sleep -Milliseconds 500
    } while ((Get-Date) -lt $deadline)

    return $false
}

function Stop-AllCloudflare {
    param([Parameter(Mandatory = $true)][string]$RuntimeDir)

    Stop-PidFileProcess -PidFile (Join-Path $RuntimeDir "cloudflared.pid")
}

function Start-AllCloudflare {
    param(
        [Parameter(Mandatory = $true)][string]$Root,
        [Parameter(Mandatory = $true)][string]$RuntimeDir,
        [ValidateSet("auto", "quic", "http2")]
        [string]$Protocol = "http2",
        [int]$TimeoutSeconds = 60
    )

    Import-MiniAppDevEnv -Root $Root
    $token = [Environment]::GetEnvironmentVariable("CLOUDFLARED_TUNNEL_TOKEN", "Process")
    if ([string]::IsNullOrWhiteSpace($token)) {
        throw "CLOUDFLARED_TUNNEL_TOKEN is empty. Put the dashboard tunnel token into .env or run with -NoCloudflare."
    }

    $cloudflared = Get-Command cloudflared -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($null -eq $cloudflared) {
        throw "cloudflared is not installed or not available in PATH."
    }

    Stop-AllCloudflare -RuntimeDir $RuntimeDir

    $stdout = Join-Path $RuntimeDir "cloudflared-live.log"
    $stderr = Join-Path $RuntimeDir "cloudflared-live.err"
    Remove-Item -LiteralPath $stdout, $stderr -Force -ErrorAction SilentlyContinue

    $args = @("tunnel", "run")
    if ($Protocol -ne "auto") {
        $args = @("tunnel", "--protocol", $Protocol, "run")
    }

    $oldTunnelToken = [Environment]::GetEnvironmentVariable("TUNNEL_TOKEN", "Process")
    $oldCloudflareTunnelToken = [Environment]::GetEnvironmentVariable("CLOUDFLARE_TUNNEL_TOKEN", "Process")
    $oldCloudflaredTunnelToken = [Environment]::GetEnvironmentVariable("CLOUDFLARED_TUNNEL_TOKEN", "Process")
    Set-Item -Path Env:TUNNEL_TOKEN -Value $token
    Remove-Item Env:\CLOUDFLARE_TUNNEL_TOKEN -ErrorAction SilentlyContinue
    Remove-Item Env:\CLOUDFLARED_TUNNEL_TOKEN -ErrorAction SilentlyContinue
    try {
        $proc = Start-Process -FilePath $cloudflared.Source `
            -ArgumentList $args `
            -WorkingDirectory $Root `
            -WindowStyle Hidden `
            -RedirectStandardOutput $stdout `
            -RedirectStandardError $stderr `
            -PassThru
    } finally {
        if ([string]::IsNullOrWhiteSpace($oldTunnelToken)) {
            Remove-Item Env:\TUNNEL_TOKEN -ErrorAction SilentlyContinue
        } else {
            Set-Item -Path Env:TUNNEL_TOKEN -Value $oldTunnelToken
        }
        if ([string]::IsNullOrWhiteSpace($oldCloudflareTunnelToken)) {
            Remove-Item Env:\CLOUDFLARE_TUNNEL_TOKEN -ErrorAction SilentlyContinue
        } else {
            Set-Item -Path Env:CLOUDFLARE_TUNNEL_TOKEN -Value $oldCloudflareTunnelToken
        }
        if ([string]::IsNullOrWhiteSpace($oldCloudflaredTunnelToken)) {
            Remove-Item Env:\CLOUDFLARED_TUNNEL_TOKEN -ErrorAction SilentlyContinue
        } else {
            Set-Item -Path Env:CLOUDFLARED_TUNNEL_TOKEN -Value $oldCloudflaredTunnelToken
        }
    }

    Set-Content -Path (Join-Path $RuntimeDir "cloudflared.pid") -Value $proc.Id -Encoding ASCII
    $connected = Wait-CloudflaredReady -StdoutPath $stdout -StderrPath $stderr -TimeoutSeconds $TimeoutSeconds

    return [pscustomobject]@{
        Connected = $connected
        LogPath   = $stdout
        ErrorPath = $stderr
        Pid       = $proc.Id
    }
}

$root = Get-RepoRoot
$runtime = Get-AllRuntimeDir -Root $root
$observabilityScript = Join-Path $PSScriptRoot "start-observability.ps1"
Ensure-Directory -Path $runtime

$observabilityArgs = @(
    "-ApiPort", [string]$ApiPort,
    "-WorkerMetricsPort", [string]$WorkerMetricsPort,
    "-ProviderWebhookPort", [string]$ProviderWebhookPort,
    "-MiniAppPort", [string]$MiniAppPort,
    "-AdminPort", [string]$AdminPort,
    "-GrafanaPort", [string]$GrafanaPort,
    "-PrometheusPort", [string]$PrometheusPort,
    "-AlertmanagerPort", [string]$AlertmanagerPort,
    "-OtelGrpcPort", [string]$OtelGrpcPort,
    "-VkUserID", [string]$VkUserID,
    "-TimeoutSeconds", [string]$TimeoutSeconds
)

if ($StatusOnly) {
    Invoke-DevScript -ScriptPath $observabilityScript -Arguments (@("-StatusOnly") + $observabilityArgs)
    Write-Host "Cloudflare:         $(Get-PidFileState -PidFile (Join-Path $runtime "cloudflared.pid"))"
    Write-Host "Cloudflare logs:    $runtime"
    exit 0
}

if ($StopOnly) {
    Stop-AllCloudflare -RuntimeDir $runtime
    $stopArgs = @("-StopOnly") + $observabilityArgs
    if ($StopDocker) {
        $stopArgs += "-StopDocker"
    }
    Invoke-DevScript -ScriptPath $observabilityScript -Arguments $stopArgs
    Write-Host "All local dev services stopped."
    exit 0
}

if ($NoRestart) {
    $observabilityArgs += "-NoRestart"
}
if ($OpenGrafana) {
    $observabilityArgs += "-OpenGrafana"
}
if ($OpenMiniApp) {
    $observabilityArgs += "-OpenMiniApp"
}
if ($OpenAdmin) {
    $observabilityArgs += "-OpenAdmin"
}
if (-not $UseEnvProviders) {
    $observabilityArgs += "-MockProviders"
}
if ($RealVkDelivery) {
    $observabilityArgs += "-RealVkDelivery"
}
if ($ResetDevData) {
    $observabilityArgs += "-ResetDevData"
}
$observabilityArgs += @("-PaymentProvider", $PaymentProvider, "-NoWait")

Invoke-DevScript -ScriptPath $observabilityScript -Arguments $observabilityArgs

$cloudflareState = "disabled"
if (-not $NoCloudflare) {
    Write-Host "Starting Cloudflare dashboard-managed tunnel..."
    $cloudflare = Start-AllCloudflare -Root $root -RuntimeDir $runtime -Protocol $CloudflareProtocol -TimeoutSeconds 60
    if ($cloudflare.Connected) {
        $cloudflareState = "connected pid=$($cloudflare.Pid)"
    } else {
        $cloudflareState = "started but not confirmed pid=$($cloudflare.Pid)"
    }
}

Write-Host ""
Write-Host "All local dev services are running."
Write-Host "API health:        http://127.0.0.1:$ApiPort/healthz"
Write-Host "Worker health:     http://127.0.0.1:$WorkerMetricsPort/healthz"
Write-Host "Provider webhook:  http://127.0.0.1:$ProviderWebhookPort/readyz"
Write-Host "Mini App:          http://127.0.0.1:$MiniAppPort"
Write-Host "Admin UI:          http://127.0.0.1:$AdminPort"
Write-Host "Grafana:           http://127.0.0.1:$GrafanaPort"
Write-Host "Prometheus:        http://127.0.0.1:$PrometheusPort"
Write-Host "Alertmanager:      http://127.0.0.1:$AlertmanagerPort"
Write-Host "Cloudflare:        $cloudflareState"
if (-not $NoCloudflare) {
    Write-Host "Public Mini App:   https://local-app.neiirohub.ru"
    Write-Host "Public VK API:     https://local-vk.neiirohub.ru"
    Write-Host "Public payment:    https://local-pay.neiirohub.ru"
}
Write-Host "Status:            .\scripts\dev\start-all.ps1 -StatusOnly"
Write-Host "Stop apps:         .\scripts\dev\start-all.ps1 -StopOnly"
Write-Host "Stop everything:   .\scripts\dev\start-all.ps1 -StopOnly -StopDocker"
Write-Host "Logs:              $runtime"

if ($NoWait) {
    exit 0
}

Read-Host "Press Enter to stop local app processes and Cloudflare tunnel"
Stop-AllCloudflare -RuntimeDir $runtime
Invoke-DevScript -ScriptPath $observabilityScript -Arguments (@("-StopOnly") + $observabilityArgs)
