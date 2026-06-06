param()

$ErrorActionPreference = "Stop"
. (Join-Path $PSScriptRoot "_bot-common.ps1")

$root = Get-RepoRoot
$runtime = Get-BotRuntimeDir -Root $root

function Test-ProcessId {
    param([string]$Name)

    $pidFile = Join-Path $runtime "$Name.pid"
    if (-not (Test-Path -LiteralPath $pidFile)) {
        return "not tracked"
    }
    $raw = Get-Content -LiteralPath $pidFile -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($raw -notmatch '^\d+$') {
        return "invalid pid"
    }
    try {
        $proc = Get-Process -Id ([int]$raw) -ErrorAction Stop
        return "running pid=$($proc.Id)"
    } catch {
        return "not running pid=$raw"
    }
}

function Test-HttpStatus {
    param([string]$Url)

    try {
        $response = Invoke-WebRequest -Uri $Url -UseBasicParsing -TimeoutSec 5
        return "ok status=$($response.StatusCode)"
    } catch {
        return "failed $($_.Exception.Message)"
    }
}

$httpAddr = Get-ConfigValue -Root $root -Name "HTTP_ADDR" -Default ":8080"
$workerMetricsAddr = Get-ConfigValue -Root $root -Name "WORKER_METRICS_ADDR" -Default ":9090"
$apiHealthUrl = Convert-ListenAddrToLocalUrl -Addr $httpAddr -Path "/health"

Write-Host "VK bot status"
Write-Host "Repo:          $root"
Write-Host "Runtime:       $runtime"
Write-Host "API process:   $(Test-ProcessId -Name "api")"
Write-Host "Worker:        $(Test-ProcessId -Name "worker")"
Write-Host "Cloudflared:   $(Test-ProcessId -Name "cloudflared")"
Write-Host "API health:    $(Test-HttpStatus -Url $apiHealthUrl)"

if (-not [string]::IsNullOrWhiteSpace($workerMetricsAddr)) {
    $workerHealthUrl = Convert-ListenAddrToLocalUrl -Addr $workerMetricsAddr -Path "/healthz"
    Write-Host "Worker health: $(Test-HttpStatus -Url $workerHealthUrl)"
}

$callbackFile = Join-Path $runtime "callback-url.txt"
if (Test-Path -LiteralPath $callbackFile) {
    $callbackUrl = Get-Content -LiteralPath $callbackFile -ErrorAction SilentlyContinue | Select-Object -First 1
    Write-Host "VK Callback:   $callbackUrl"
} else {
    $tunnelUrl = Get-TunnelUrlFromLogs -RuntimeDir $runtime
    if (-not [string]::IsNullOrWhiteSpace($tunnelUrl)) {
        Write-Host "VK Callback:   $tunnelUrl/webhooks/vk"
    } else {
        Write-Host "VK Callback:   not found"
    }
}

Write-Host ""
Write-Host "Docker dependencies:"
Push-Location $root
try {
    $oldPreference = $ErrorActionPreference
    $ErrorActionPreference = "SilentlyContinue"
    docker info 1>$null 2>$null
    $dockerAvailable = ($LASTEXITCODE -eq 0)
    $ErrorActionPreference = $oldPreference

    if (-not $dockerAvailable) {
        Write-Host "Docker Engine is not running."
    } else {
        docker compose ps postgres redis minio
    }
} catch {
    Write-Host "Docker status failed: $($_.Exception.Message)"
} finally {
    Pop-Location
}
