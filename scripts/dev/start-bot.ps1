param(
    [switch]$NoTunnel,
    [switch]$SkipDocker,
    [switch]$SkipMigrate,
    [switch]$NoRestart,
    [string]$TunnelMode = "",
    [string]$TunnelHostname = "",
    [string]$TunnelName = "",
    [string]$TunnelConfigPath = "",
    [ValidateSet("http2", "quic")][string]$TunnelProtocol = "http2",
    [int]$TimeoutSeconds = 120
)

$ErrorActionPreference = "Stop"
. (Join-Path $PSScriptRoot "_bot-common.ps1")

$root = Get-RepoRoot
$runtime = Get-BotRuntimeDir -Root $root
$bin = Get-BotBinDir -Root $root

Ensure-Directory -Path $runtime
Ensure-Directory -Path $bin

Push-Location $root
try {
    if (-not $NoRestart) {
        & (Join-Path $PSScriptRoot "stop-bot.ps1") -Quiet
    }

    if (-not $SkipDocker) {
        Write-Host "Starting Docker dependencies: postgres, redis, minio..."
        Ensure-DockerRunning -TimeoutSeconds $TimeoutSeconds
        docker compose up -d postgres redis minio | Out-Host
        Wait-BotDockerDependencies -Root $root -TimeoutSeconds $TimeoutSeconds
    }

    if (-not $SkipMigrate) {
        Write-Host "Applying database migrations..."
        go run ./cmd/migrate up | Out-Host
    }

    Write-Host "Building bot binaries..."
    $apiExe = Join-Path $bin "api.exe"
    $workerExe = Join-Path $bin "worker.exe"
    go build -o $apiExe ./cmd/api
    go build -o $workerExe ./cmd/worker

    Write-Host "Starting API webhook..."
    $apiProc = Start-BotExecutable `
        -Root $root `
        -ExePath $apiExe `
        -StdoutPath (Join-Path $runtime "api-live.log") `
        -StderrPath (Join-Path $runtime "api-live.err")
    Set-Content -Path (Join-Path $runtime "api.pid") -Value $apiProc.Id -Encoding ASCII

    Write-Host "Starting worker..."
    $workerProc = Start-BotExecutable `
        -Root $root `
        -ExePath $workerExe `
        -StdoutPath (Join-Path $runtime "worker-live.log") `
        -StderrPath (Join-Path $runtime "worker-live.err")
    Set-Content -Path (Join-Path $runtime "worker.pid") -Value $workerProc.Id -Encoding ASCII

    $httpAddr = Get-ConfigValue -Root $root -Name "HTTP_ADDR" -Default ":8080"
    $workerMetricsAddr = Get-ConfigValue -Root $root -Name "WORKER_METRICS_ADDR" -Default ":9090"
    $apiHealthUrl = Convert-ListenAddrToLocalUrl -Addr $httpAddr -Path "/health"
    $apiBaseUrl = Convert-ListenAddrToLocalUrl -Addr $httpAddr -Path ""

    Write-Host "Waiting for API health: $apiHealthUrl"
    Wait-Http -Url $apiHealthUrl -TimeoutSeconds $TimeoutSeconds | Out-Null

    if (-not [string]::IsNullOrWhiteSpace($workerMetricsAddr)) {
        $workerHealthUrl = Convert-ListenAddrToLocalUrl -Addr $workerMetricsAddr -Path "/healthz"
        Write-Host "Waiting for worker health: $workerHealthUrl"
        Wait-Http -Url $workerHealthUrl -TimeoutSeconds $TimeoutSeconds | Out-Null
    }

    $callbackUrl = ""
    if (-not $NoTunnel) {
        if ([string]::IsNullOrWhiteSpace($TunnelMode)) {
            $TunnelMode = Get-ConfigValue -Root $root -Name "VK_BOT_TUNNEL_MODE" -Default "quick"
        }
        if ([string]::IsNullOrWhiteSpace($TunnelHostname)) {
            $TunnelHostname = Get-ConfigValue -Root $root -Name "VK_BOT_TUNNEL_HOSTNAME" -Default "vk.neiirohub.ru"
        }
        if ([string]::IsNullOrWhiteSpace($TunnelName)) {
            $TunnelName = Get-ConfigValue -Root $root -Name "VK_BOT_TUNNEL_NAME" -Default "neiirohub-vk-bot"
        }
        if ([string]::IsNullOrWhiteSpace($TunnelConfigPath)) {
            $TunnelConfigPath = Get-ConfigValue -Root $root -Name "VK_BOT_TUNNEL_CONFIG" -Default (Get-NamedTunnelConfigPath -RuntimeDir $runtime)
        }

        if ($TunnelMode -eq "quick") {
            Write-Host "Starting Cloudflare quick tunnel..."
            $cloudflaredProc = Start-CloudflaredTunnel -RuntimeDir $runtime -LocalUrl $apiBaseUrl -Protocol $TunnelProtocol
            Set-Content -Path (Join-Path $runtime "cloudflared.pid") -Value $cloudflaredProc.Id -Encoding ASCII

            $tunnelUrl = Wait-TunnelUrl -RuntimeDir $runtime -TimeoutSeconds $TimeoutSeconds
            $publicHealthUrl = "$tunnelUrl/health"
            Write-Host "Waiting for public tunnel health: $publicHealthUrl"
            Wait-Http -Url $publicHealthUrl -TimeoutSeconds $TimeoutSeconds | Out-Null
            $callbackUrl = "$tunnelUrl/webhooks/vk"
        } elseif ($TunnelMode -eq "named") {
            Write-Host "Starting Cloudflare named tunnel: $TunnelName -> $TunnelHostname"
            $cloudflaredProc = Start-CloudflaredNamedTunnel -RuntimeDir $runtime -ConfigPath $TunnelConfigPath -TunnelName $TunnelName
            Set-Content -Path (Join-Path $runtime "cloudflared.pid") -Value $cloudflaredProc.Id -Encoding ASCII

            $publicHealthUrl = "https://$TunnelHostname/health"
            Write-Host "Waiting for public named tunnel health: $publicHealthUrl"
            Wait-Http -Url $publicHealthUrl -TimeoutSeconds $TimeoutSeconds | Out-Null
            $callbackUrl = Get-NamedTunnelCallbackUrl -Hostname $TunnelHostname
        } else {
            throw "Unsupported TunnelMode '$TunnelMode'. Use 'quick' or 'named'."
        }

        Set-Content -Path (Join-Path $runtime "callback-url.txt") -Value $callbackUrl -Encoding ASCII
    }

    Write-Host ""
    Write-Host "VK bot is running."
    Write-Host "API health:    $apiHealthUrl"
    if (-not [string]::IsNullOrWhiteSpace($workerMetricsAddr)) {
        Write-Host "Worker health: $workerHealthUrl"
    }
    if ($callbackUrl -ne "") {
        Write-Host "VK Callback:   $callbackUrl"
        Write-Host "Put this URL into VK Callback API settings and confirm the server."
    } else {
        Write-Host "Tunnel:        disabled by -NoTunnel"
    }
    Write-Host "Logs:          $runtime"
} finally {
    Pop-Location
}
