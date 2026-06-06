param(
    [switch]$StopDocker,
    [switch]$Quiet
)

$ErrorActionPreference = "Stop"
. (Join-Path $PSScriptRoot "_bot-common.ps1")

$root = Get-RepoRoot
$runtime = Get-BotRuntimeDir -Root $root

if (-not $Quiet) {
    Write-Host "Stopping local VK bot processes..."
}

Stop-PidFileProcess -PidFile (Join-Path $runtime "api.pid")
Stop-PidFileProcess -PidFile (Join-Path $runtime "worker.pid")
Stop-PidFileProcess -PidFile (Join-Path $runtime "cloudflared.pid")

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

Stop-BotCommandLineProcesses -Root $root

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
    Write-Host "VK bot stopped."
}
