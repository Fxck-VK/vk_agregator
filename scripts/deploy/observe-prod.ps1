[CmdletBinding()]
param(
    [string]$EnvFile = ".env",
    [int]$ReverseProxyPort = 8088,
    [int]$Tail = 80,
    [switch]$ShowLogs
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..")
Set-Location $repoRoot

$compose = @("--env-file", $EnvFile, "-f", "docker-compose.prod.yml")

function Invoke-Compose {
    param([Parameter(Mandatory = $true)][string[]]$ComposeArgs)
    docker compose @compose @ComposeArgs
}

function Test-Http {
    param(
        [Parameter(Mandatory = $true)][string]$Name,
        [Parameter(Mandatory = $true)][string]$Url
    )

    try {
        $response = Invoke-WebRequest -UseBasicParsing -Method Get -Uri $Url -TimeoutSec 10
        $status = [int]$response.StatusCode
        if ($status -lt 200 -or $status -gt 299) {
            throw "$Name returned HTTP $status"
        }
        Write-Host "[OK] $Name -> HTTP $status"
    } catch {
        Write-Host "[FAIL] $Name -> $($_.Exception.Message)" -ForegroundColor Red
        throw
    }
}

function Show-MetricLines {
    param(
        [Parameter(Mandatory = $true)][string]$Name,
        [Parameter(Mandatory = $true)][string]$Url,
        [Parameter(Mandatory = $true)][string]$Pattern
    )

    try {
        $response = Invoke-WebRequest -UseBasicParsing -Method Get -Uri $Url -TimeoutSec 10
        $lines = @(
            ($response.Content -split "`n") |
                Where-Object { $_ -notmatch "^\s*#" -and $_ -match $Pattern } |
                Select-Object -First 40
        )
        Write-Host "== $Name metrics =="
        if ($lines.Count -eq 0) {
            Write-Host "(no matching metric samples)"
        } else {
            $lines | ForEach-Object { Write-Host $_ }
        }
    } catch {
        Write-Host "[WARN] $Name metrics unavailable: $($_.Exception.Message)" -ForegroundColor Yellow
    }
}

Write-Host "== Production container status =="
Invoke-Compose -Args @("ps")

Write-Host "== Health endpoints =="
Test-Http -Name "api /health" -Url "http://127.0.0.1:8080/health"
Test-Http -Name "api /healthz" -Url "http://127.0.0.1:8080/healthz"
Test-Http -Name "worker /healthz" -Url "http://127.0.0.1:9090/healthz"
Test-Http -Name "provider-webhook /health" -Url "http://127.0.0.1:8082/health"
Test-Http -Name "provider-webhook /readyz" -Url "http://127.0.0.1:8082/readyz"
Test-Http -Name "reverse-proxy /proxy-health" -Url "http://127.0.0.1:$ReverseProxyPort/proxy-health"

Write-Host "== Private metrics endpoints =="
Test-Http -Name "api /metrics" -Url "http://127.0.0.1:8080/metrics"
Test-Http -Name "worker /metrics" -Url "http://127.0.0.1:9090/metrics"
Test-Http -Name "provider-webhook /metrics" -Url "http://127.0.0.1:8082/metrics"
Show-MetricLines -Name "worker queue/DLQ" -Url "http://127.0.0.1:9090/metrics" -Pattern "^(vkagg_queue_depth|vkagg_queue_oldest_age_seconds|vkagg_queue_consumer_lag|vkagg_dlq_routed_total)"
Show-MetricLines -Name "payment webhook" -Url "http://127.0.0.1:8082/metrics" -Pattern "^(payment_webhook_unprocessed_events|payment_webhook_oldest_unprocessed_age_seconds|payment_webhook_processing_errors_total|payment_provider_errors_total|payment_reconciliation_mismatches|payment_webhooks_total)"

Write-Host "== Redis stream lengths =="
$streams = @(
    "stream:jobs:text",
    "stream:jobs:image",
    "stream:jobs:video",
    "stream:jobs:delivery",
    "stream:jobs:provider_poll",
    "stream:jobs:dlq"
)
foreach ($stream in $streams) {
    $length = Invoke-Compose -Args @("exec", "-T", "redis", "redis-cli", "XLEN", $stream)
    Write-Host "$stream length=$($length.Trim())"
}

if ($ShowLogs) {
    Write-Host "== Recent container logs =="
    Invoke-Compose -Args @("logs", "--tail=$Tail", "api", "worker", "provider-webhook", "reverse-proxy")
} else {
    Write-Host "Logs are not printed by default. Use -ShowLogs or run:"
    Write-Host "docker compose --env-file $EnvFile -f docker-compose.prod.yml logs --tail=$Tail api worker provider-webhook reverse-proxy"
}

Write-Host "production observability check OK"
