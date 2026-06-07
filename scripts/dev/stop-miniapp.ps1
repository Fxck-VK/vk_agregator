param(
    [switch]$StopDocker,
    [switch]$Quiet
)

$ErrorActionPreference = "Stop"
. (Join-Path $PSScriptRoot "_miniapp-common.ps1")

$root = Get-RepoRoot
$runtime = Get-MiniAppRuntimeDir -Root $root
$miniAppPort = 5173

if (-not $Quiet) {
    Write-Host "Stopping local VK Mini App processes..."
}

Stop-PidFileProcess -PidFile (Join-Path $runtime "api.pid")
Stop-PidFileProcess -PidFile (Join-Path $runtime "worker.pid")
Stop-PidFileProcess -PidFile (Join-Path $runtime "vite.pid")
Stop-PidFileProcess -PidFile (Join-Path $runtime "tunnel.pid")

$httpAddr = Get-ConfigValue -Root $root -Name "HTTP_ADDR" -Default ":8080"
$workerMetricsAddr = Get-ConfigValue -Root $root -Name "WORKER_METRICS_ADDR" -Default ":9090"

$httpPort = Get-PortFromListenAddr -Addr $httpAddr
if ($null -ne $httpPort) {
    Stop-ListenerOnPort -Port $httpPort
}

$workerMetricsPort = Get-PortFromListenAddr -Addr $workerMetricsAddr
if ($null -ne $workerMetricsPort) {
    Stop-ListenerOnPort -Port $workerMetricsPort
}

Stop-ListenerOnPort -Port $miniAppPort
Stop-MiniAppCommandLineProcesses -Root $root -MiniAppPort $miniAppPort
Stop-LocalhostRunTunnel

if ($StopDocker) {
    if (-not $Quiet) {
        Write-Host "Stopping Docker dependencies..."
    }
    Push-Location $root
    try {
        docker compose stop postgres redis minio | Out-Host
    } finally {
        Pop-Location
    }
}

if (-not $Quiet) {
    Write-Host "VK Mini App stopped."
}
