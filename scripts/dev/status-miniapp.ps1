param()

$ErrorActionPreference = "Stop"
. (Join-Path $PSScriptRoot "_miniapp-common.ps1")

$root = Get-RepoRoot
$runtime = Get-MiniAppRuntimeDir -Root $root

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

$apiHealthUrl = "http://127.0.0.1:8080/health"
$viteUrl = "http://127.0.0.1:5173"
$workerHealthUrl = "http://127.0.0.1:9090/healthz"

Write-Host "VK Mini App status"
Write-Host "Repo:          $root"
Write-Host "Runtime:       $runtime"
Write-Host "API process:   $(Test-ProcessId -Name "api")"
Write-Host "Worker:        $(Test-ProcessId -Name "worker")"
Write-Host "Vite:          $(Test-ProcessId -Name "vite")"
Write-Host "Tunnel:        $(Test-ProcessId -Name "tunnel")"
Write-Host "API health:    $(Test-HttpStatus -Url $apiHealthUrl)"
Write-Host "Vite health:   $(Test-HttpStatus -Url $viteUrl)"
Write-Host "Worker health: $(Test-HttpStatus -Url $workerHealthUrl)"

$frontendFile = Join-Path $runtime "frontend-url.txt"
if (Test-Path -LiteralPath $frontendFile) {
    $frontendUrl = Get-Content -LiteralPath $frontendFile -ErrorAction SilentlyContinue | Select-Object -First 1
    Write-Host "Frontend URL:  $frontendUrl"
} else {
    $tunnelUrl = Get-LocalhostRunUrlFromLogs -RuntimeDir $runtime
    if (-not [string]::IsNullOrWhiteSpace($tunnelUrl)) {
        Write-Host "Frontend URL:  $tunnelUrl"
    } else {
        Write-Host "Frontend URL:  not found"
    }
}

$launchShortcut = Join-Path $runtime "miniapp-launch.url"
if (Test-Path -LiteralPath $launchShortcut) {
    Write-Host "Browser test:  $launchShortcut"
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
