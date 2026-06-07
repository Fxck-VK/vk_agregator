param(
    [switch]$NoTunnel,
    [switch]$SkipDocker,
    [switch]$SkipMigrate,
    [switch]$NoRestart,
    [switch]$NoWait,
    [switch]$OpenBrowser,
    [int]$ApiPort = 8080,
    [int]$MiniAppPort = 5173,
    [int]$WorkerMetricsPort = 9090,
    [int]$VkUserID = 777,
    [int]$TimeoutSeconds = 120
)

$ErrorActionPreference = "Stop"
. (Join-Path $PSScriptRoot "_miniapp-common.ps1")

$root = Get-RepoRoot
$runtime = Get-MiniAppRuntimeDir -Root $root
$bin = Get-MiniAppBinDir -Root $root

Ensure-Directory -Path $runtime
Ensure-Directory -Path $bin

Push-Location $root
try {
    if (-not $NoRestart) {
        & (Join-Path $PSScriptRoot "stop-miniapp.ps1") -Quiet
    }

    Import-MiniAppDevEnv -Root $root
    Set-MiniAppDevDefaults -ApiPort $ApiPort -WorkerMetricsPort $WorkerMetricsPort

    Write-Host "DEEPINFRA_API_KEY: $(Get-MiniAppEnvState -Name 'DEEPINFRA_API_KEY')"
    Write-Host "DEEPINFRA_TEXT_MODEL: $(Get-MiniAppEnvState -Name 'DEEPINFRA_TEXT_MODEL')"
    Write-Host "DATABASE_URL: $(Get-MiniAppEnvState -Name 'DATABASE_URL')"
    Write-Host "REDIS_ADDR: $(Get-MiniAppEnvState -Name 'REDIS_ADDR')"
    Write-Host "S3_ENDPOINT: $(Get-MiniAppEnvState -Name 'S3_ENDPOINT')"
    Write-Host "S3_BUCKET: $(Get-MiniAppEnvState -Name 'S3_BUCKET')"

    Assert-MiniAppDevRequirements -RequireTunnel:(-not $NoTunnel)

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

    Write-Host "Building Mini App API and worker binaries..."
    $apiExe = Join-Path $bin "api.exe"
    $workerExe = Join-Path $bin "worker.exe"
    go build -o $apiExe ./cmd/api
    go build -o $workerExe ./cmd/worker

    Write-Host "Starting API, worker and Vite dev server..."
    $apiProc = Start-MiniAppExecutable `
        -Root $root `
        -ExePath $apiExe `
        -StdoutPath (Join-Path $runtime "api-live.log") `
        -StderrPath (Join-Path $runtime "api-live.err") `
        -ApiPort $ApiPort `
        -WorkerMetricsPort $WorkerMetricsPort
    Set-Content -Path (Join-Path $runtime "api.pid") -Value $apiProc.Id -Encoding ASCII

    $workerProc = Start-MiniAppExecutable `
        -Root $root `
        -ExePath $workerExe `
        -StdoutPath (Join-Path $runtime "worker-live.log") `
        -StderrPath (Join-Path $runtime "worker-live.err") `
        -ApiPort $ApiPort `
        -WorkerMetricsPort $WorkerMetricsPort
    Set-Content -Path (Join-Path $runtime "worker.pid") -Value $workerProc.Id -Encoding ASCII

    $devLaunchParams = New-MiniAppLaunchParams -UserID $VkUserID
    $viteProc = Start-ViteDevServer `
        -Root $root `
        -RuntimeDir $runtime `
        -MiniAppPort $MiniAppPort `
        -LaunchParams $devLaunchParams
    Set-Content -Path (Join-Path $runtime "vite.pid") -Value $viteProc.Id -Encoding ASCII

    $apiHealthUrl = "http://127.0.0.1:$ApiPort/health"
    $viteUrl = "http://127.0.0.1:$MiniAppPort"
    $workerHealthUrl = "http://127.0.0.1:$WorkerMetricsPort/healthz"

    Write-Host "Waiting for API health: $apiHealthUrl"
    Wait-Http -Url $apiHealthUrl -TimeoutSeconds $TimeoutSeconds | Out-Null
    Write-Host "Waiting for Vite dev server: $viteUrl"
    Wait-Http -Url $viteUrl -TimeoutSeconds $TimeoutSeconds | Out-Null
    Write-Host "Waiting for worker health: $workerHealthUrl"
    Wait-Http -Url $workerHealthUrl -TimeoutSeconds $TimeoutSeconds | Out-Null

    $publicUrl = $viteUrl
    if (-not $NoTunnel) {
        Write-Host "Starting localhost.run HTTPS tunnel (*.lhr.life)..."
        $tunnelProc = Start-LocalhostRunTunnel -RuntimeDir $runtime -MiniAppPort $MiniAppPort
        Set-Content -Path (Join-Path $runtime "tunnel.pid") -Value $tunnelProc.Id -Encoding ASCII
        $publicUrl = Wait-LocalhostRunUrl -RuntimeDir $runtime -TimeoutSeconds $TimeoutSeconds
        Set-Content -Path (Join-Path $runtime "frontend-url.txt") -Value $publicUrl -Encoding ASCII
    }

    $launchUrl = "$publicUrl/?$devLaunchParams"
    $launchShortcut = Join-Path $runtime "miniapp-launch.url"
    Set-Content -Path $launchShortcut -Encoding ASCII -Value "[InternetShortcut]`r`nURL=$launchUrl`r`n"

    Write-Host ""
    Write-Host "VK Mini App dev stack is running."
    Write-Host "API health:    $apiHealthUrl"
    Write-Host "Vite local:    $viteUrl"
    Write-Host "Worker health: $workerHealthUrl"
    if (-not $NoTunnel) {
        Write-Host "Frontend URL:  $publicUrl"
        Write-Host "VK dev URL:    paste the Frontend URL into dev.vk.com (https://*.lhr.life)."
    } else {
        Write-Host "Tunnel:        disabled by -NoTunnel"
    }
    Write-Host "Browser test:  $launchShortcut"
    Write-Host "Logs:          $runtime"

    if ($OpenBrowser) {
        Start-Process $launchUrl
    }

    if ($NoWait) {
        exit 0
    }

    Read-Host "Press Enter to stop API, worker, Vite and tunnel"
    & (Join-Path $PSScriptRoot "stop-miniapp.ps1")
} finally {
    Pop-Location
}
