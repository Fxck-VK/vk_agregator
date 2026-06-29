param(
    [string]$EnvFile = ".env.loadtest",
    [string]$BaseUrl = "",
    [string]$WebhookBaseUrl = "",
    [string]$Duration = "",
    [int]$Rate = 0,
    [string]$ProductCode = "",
    [string]$ReportDir = "",
    [switch]$UseDockerCompose,
    [switch]$SkipPostgres,
    [switch]$SkipRedis,
    [switch]$SkipDockerStats,
    [switch]$AllowDLQ,
    [switch]$ContinueOnFailure
)

$ErrorActionPreference = "Stop"

function Read-EnvFile {
    param([string]$Path)

    $values = @{}
    if (-not (Test-Path $Path)) {
        return $values
    }

    foreach ($line in Get-Content $Path) {
        $trimmed = $line.Trim()
        if ($trimmed -eq "" -or $trimmed.StartsWith("#")) {
            continue
        }

        $idx = $trimmed.IndexOf("=")
        if ($idx -le 0) {
            continue
        }

        $name = $trimmed.Substring(0, $idx).Trim()
        $value = $trimmed.Substring($idx + 1).Trim()
        if (($value.StartsWith('"') -and $value.EndsWith('"')) -or ($value.StartsWith("'") -and $value.EndsWith("'"))) {
            $value = $value.Substring(1, $value.Length - 2)
        }
        $values[$name] = $value
    }

    return $values
}

function Get-Setting {
    param(
        [hashtable]$Values,
        [string]$Name,
        [string]$Default = ""
    )

    $envValue = [Environment]::GetEnvironmentVariable($Name)
    if (-not [string]::IsNullOrWhiteSpace($envValue)) {
        return $envValue
    }
    if ($Values.ContainsKey($Name) -and -not [string]::IsNullOrWhiteSpace($Values[$Name])) {
        return $Values[$Name]
    }
    return $Default
}

function New-K6EnvArgs {
    param([hashtable]$Values)

    $args = @()
    foreach ($key in ($Values.Keys | Sort-Object)) {
        $args += @("-e", "$key=$($Values[$key])")
    }
    return $args
}

function Get-UrlHost {
    param([string]$Value)

    if ([string]::IsNullOrWhiteSpace($Value)) {
        return ""
    }
    try {
        return ([uri]$Value).Host.ToLowerInvariant()
    } catch {
        throw "Invalid URL: $Value"
    }
}

function Get-MetricValue {
    param(
        [object]$Summary,
        [string]$Metric,
        [string]$ValueName,
        [object]$Default = $null
    )

    if ($null -eq $Summary -or $null -eq $Summary.metrics) {
        return $Default
    }

    $metricProperty = $Summary.metrics.PSObject.Properties[$Metric]
    if ($null -eq $metricProperty -or $null -eq $metricProperty.Value.values) {
        return $Default
    }

    $valueProperty = $metricProperty.Value.values.PSObject.Properties[$ValueName]
    if ($null -eq $valueProperty) {
        return $Default
    }
    return $valueProperty.Value
}

function Read-JsonFile {
    param([string]$Path)

    if (-not (Test-Path $Path)) {
        return $null
    }
    return Get-Content $Path -Raw | ConvertFrom-Json
}

function Capture-DockerStats {
    param([string]$OutputFile)

    if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
        Set-Content -Path $OutputFile -Value "docker command not found"
        return
    }

    $stats = & docker stats --no-stream --format "table {{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.NetIO}}\t{{.BlockIO}}" 2>&1
    Set-Content -Path $OutputFile -Value $stats
}

function Invoke-PostgresDiagnostics {
    param(
        [string]$Stage,
        [string]$OutputFile
    )

    $args = @(
        "-NoProfile",
        "-ExecutionPolicy", "Bypass",
        "-File", (Join-Path $PSScriptRoot "postgres-diagnostics.ps1"),
        "-EnvFile", $EnvFile,
        "-OutputFile", $OutputFile
    )
    if ($UseDockerCompose) {
        $args += "-UseDockerCompose"
    }

    Write-Host "Running PostgreSQL diagnostics ($Stage)..."
    & powershell @args
    return $LASTEXITCODE
}

function Invoke-RedisDiagnostics {
    param(
        [string]$OutputFile,
        [string]$SnapshotFile
    )

    $args = @(
        "-NoProfile",
        "-ExecutionPolicy", "Bypass",
        "-File", (Join-Path $PSScriptRoot "redis-diagnostics.ps1"),
        "-EnvFile", $EnvFile,
        "-OutputFile", $OutputFile,
        "-SnapshotFile", $SnapshotFile
    )
    if ($UseDockerCompose) {
        $args += "-UseDockerCompose"
    }

    Write-Host "Running Redis diagnostics..."
    & powershell @args
    return $LASTEXITCODE
}

function New-BillingReport {
    param(
        [object]$Summary,
        [object]$RedisSnapshot,
        [string]$OutputFile,
        [string]$BaseUrlValue,
        [string]$DurationValue,
        [int]$RateValue,
        [string]$ProductCodeValue,
        [array]$Failures
    )

    $httpFailedRate = Get-MetricValue $Summary "http_req_failed" "rate" ""
    $httpP95 = Get-MetricValue $Summary "http_req_duration" "p(95)" ""
    $httpP99 = Get-MetricValue $Summary "http_req_duration" "p(99)" ""
    $journeyP95 = Get-MetricValue $Summary "payment_mock_journey_duration" "p(95)" ""
    $journeyP99 = Get-MetricValue $Summary "payment_mock_journey_duration" "p(99)" ""
    $intentCount = Get-MetricValue $Summary "payment_intent_created_total" "count" ""
    $historyRate = Get-MetricValue $Summary "payment_history_ok" "rate" ""
    $topupRate = Get-MetricValue $Summary "payment_mock_topup_ok" "rate" ""
    $refundRate = Get-MetricValue $Summary "payment_refund_ok" "rate" ""
    $idempotencyRate = Get-MetricValue $Summary "payment_idempotency_ok" "rate" ""

    $redisQueueDepth = ""
    $redisDLQDepth = ""
    if ($null -ne $RedisSnapshot) {
        $redisQueueDepth = $RedisSnapshot.queue_depth
        $redisDLQDepth = $RedisSnapshot.dlq_depth
    }

    $lines = @()
    $lines += "# Billing Mock Load Report"
    $lines += ""
    $lines += "- Generated: $(Get-Date -Format o)"
    $lines += ('- Base URL: `{0}`' -f $BaseUrlValue)
    $lines += ('- Duration: `{0}`' -f $DurationValue)
    $lines += ('- Rate: `{0}/s`' -f $RateValue)
    $lines += ('- Product: `{0}`' -f $ProductCodeValue)
    $lines += ""
    $lines += "## k6 Summary"
    $lines += ""
    $lines += "| Metric | Value |"
    $lines += "|---|---:|"
    $lines += "| HTTP failed rate | $httpFailedRate |"
    $lines += "| HTTP p95 ms | $httpP95 |"
    $lines += "| HTTP p99 ms | $httpP99 |"
    $lines += "| Payment journey p95 ms | $journeyP95 |"
    $lines += "| Payment journey p99 ms | $journeyP99 |"
    $lines += "| Intents created | $intentCount |"
    $lines += "| History OK rate | $historyRate |"
    $lines += "| Mock top-up OK rate | $topupRate |"
    $lines += "| Refund OK rate | $refundRate |"
    $lines += "| Idempotency OK rate | $idempotencyRate |"
    $lines += ""
    $lines += "## Redis Snapshot"
    $lines += ""
    $lines += "| Metric | Value |"
    $lines += "|---|---:|"
    $lines += "| Queue depth | $redisQueueDepth |"
    $lines += "| DLQ depth | $redisDLQDepth |"
    $lines += ""
    $lines += "## Artifacts"
    $lines += ""
    $lines += '- `k6-summary.json`'
    $lines += '- `k6-output.log`'
    $lines += '- `postgres-before.md`'
    $lines += '- `postgres-after.md`'
    $lines += '- `redis-diagnostics.md`'
    $lines += '- `docker-stats.txt`'
    $lines += ""
    $lines += "## Ledger And Idempotency Criteria"
    $lines += ""
    $lines += '- `payment_mock_topup_ok` should stay above the k6 threshold.'
    $lines += '- `payment_refund_ok` should stay above the k6 threshold.'
    $lines += '- `payment_idempotency_ok` should stay above the k6 threshold.'
    $lines += '- Redis DLQ should remain zero unless `-AllowDLQ` is explicitly used.'
    $lines += "- PostgreSQL diagnostics should not show unexpected lock buildup or slow ledger/payment queries."
    $lines += ""
    if ($Failures.Count -gt 0) {
        $lines += "## Failures"
        $lines += ""
        foreach ($failure in $Failures) {
            $lines += "- $failure"
        }
    } else {
        $lines += "## Failures"
        $lines += ""
        $lines += "None."
    }

    Set-Content -Path $OutputFile -Value $lines -Encoding UTF8
}

$envValues = Read-EnvFile $EnvFile

if ($BaseUrl -eq "") {
    $BaseUrl = Get-Setting $envValues "K6_BASE_URL" "http://127.0.0.1:8080"
}
if ($WebhookBaseUrl -eq "") {
    $WebhookBaseUrl = Get-Setting $envValues "K6_PAYMENT_WEBHOOK_BASE_URL" ""
}
if ($Duration -eq "") {
    $Duration = Get-Setting $envValues "K6_BILLING_LOAD_DURATION" (Get-Setting $envValues "K6_PAYMENT_DURATION" "1m")
}
if ($Rate -le 0) {
    $Rate = [int](Get-Setting $envValues "K6_BILLING_LOAD_RATE" (Get-Setting $envValues "K6_PAYMENT_RATE" "1"))
}
if ($ProductCode -eq "") {
    $ProductCode = Get-Setting $envValues "K6_PAYMENT_PRODUCT_CODE" "crystals_99"
}

$blockedHosts = @("vk.neiirohub.ru", "app.neiirohub.ru", "neiirohub.ru")
foreach ($hostValue in @((Get-UrlHost $BaseUrl), (Get-UrlHost $WebhookBaseUrl))) {
    if ($blockedHosts -contains $hostValue) {
        throw "Refusing to run billing load test against production host: $hostValue"
    }
}

$preflightScript = Join-Path $PSScriptRoot "loadtest-preflight.ps1"
if (Test-Path $preflightScript) {
    & powershell -NoProfile -ExecutionPolicy Bypass -File $preflightScript -EnvFile $EnvFile
    if ($LASTEXITCODE -ne 0) {
        throw "loadtest preflight failed"
    }
}

if ($ReportDir -eq "") {
    $timestamp = Get-Date -Format "yyyyMMdd-HHmmss"
    $ReportDir = Join-Path "artifacts/loadtest" "billing-load-$timestamp"
}
New-Item -ItemType Directory -Force -Path $ReportDir | Out-Null

if (-not (Get-Command k6 -ErrorAction SilentlyContinue)) {
    throw "k6 is not installed or not available in PATH"
}

$script = "tests/k6/billing-payments.js"
if (-not (Test-Path $script)) {
    throw "Missing k6 script: $script"
}

$failures = @()

if (-not $SkipPostgres) {
    $exit = Invoke-PostgresDiagnostics -Stage "before" -OutputFile (Join-Path $ReportDir "postgres-before.md")
    if ($exit -ne 0) {
        $failures += "postgres diagnostics before load failed"
    }
}

$reservedK6ConfigKeys = @("K6_DURATION", "K6_ITERATIONS", "K6_SCENARIOS", "K6_STAGES", "K6_VUS")
$k6Env = @{}
foreach ($key in ($envValues.Keys | Sort-Object)) {
    if ($key -like "K6_*" -and $reservedK6ConfigKeys -notcontains $key) {
        $k6Env[$key] = $envValues[$key]
    }
}
$k6Env["K6_BASE_URL"] = $BaseUrl
$k6Env["K6_RUN"] = "1"
$k6Env["K6_ALLOW_PRODUCTION_LIVE_SMOKE"] = "false"
$k6Env["K6_PAYMENT_DURATION"] = $Duration
$k6Env["K6_PAYMENT_RATE"] = "$Rate"
$k6Env["K6_PAYMENT_PRODUCT_CODE"] = $ProductCode
if ($WebhookBaseUrl -ne "") {
    $k6Env["K6_PAYMENT_WEBHOOK_BASE_URL"] = $WebhookBaseUrl
}

$summaryPath = Join-Path $ReportDir "k6-summary.json"
$outputPath = Join-Path $ReportDir "k6-output.log"

Write-Host "Running billing mock load for $Duration at $Rate/s..."
$k6Args = @("run") + (New-K6EnvArgs $k6Env) + @("--summary-export", $summaryPath, $script)
$k6Output = & k6 @k6Args 2>&1
$k6ExitCode = $LASTEXITCODE
Set-Content -Path $outputPath -Value $k6Output

if ($k6ExitCode -ne 0) {
    $failures += "billing k6 failed with exit code $k6ExitCode"
}

if (-not $SkipPostgres) {
    $exit = Invoke-PostgresDiagnostics -Stage "after" -OutputFile (Join-Path $ReportDir "postgres-after.md")
    if ($exit -ne 0) {
        $failures += "postgres diagnostics after load failed"
    }
}

$redisSnapshot = $null
if (-not $SkipRedis) {
    $redisSnapshotPath = Join-Path $ReportDir "redis-diagnostics.snapshot.json"
    $exit = Invoke-RedisDiagnostics -OutputFile (Join-Path $ReportDir "redis-diagnostics.md") -SnapshotFile $redisSnapshotPath
    if ($exit -ne 0) {
        $failures += "redis diagnostics failed"
    } else {
        $redisSnapshot = Read-JsonFile $redisSnapshotPath
        if (-not $AllowDLQ -and $null -ne $redisSnapshot -and $redisSnapshot.dlq_depth -gt 0) {
            $failures += "Redis DLQ is not empty: $($redisSnapshot.dlq_depth)"
        }
    }
}

if (-not $SkipDockerStats) {
    Capture-DockerStats (Join-Path $ReportDir "docker-stats.txt")
}

$summary = Read-JsonFile $summaryPath
$reportPath = Join-Path $ReportDir "billing-load-report.md"
New-BillingReport -Summary $summary -RedisSnapshot $redisSnapshot -OutputFile $reportPath -BaseUrlValue $BaseUrl -DurationValue $Duration -RateValue $Rate -ProductCodeValue $ProductCode -Failures $failures

$summaryOutput = [pscustomobject]@{
    generated_at = (Get-Date -Format o)
    base_url = $BaseUrl
    webhook_base_url = $WebhookBaseUrl
    duration = $Duration
    rate = $Rate
    product_code = $ProductCode
    k6_exit_code = $k6ExitCode
    failures = $failures
    metrics = [pscustomobject]@{
        http_failed_rate = Get-MetricValue $summary "http_req_failed" "rate" $null
        http_p95_ms = Get-MetricValue $summary "http_req_duration" "p(95)" $null
        http_p99_ms = Get-MetricValue $summary "http_req_duration" "p(99)" $null
        journey_p95_ms = Get-MetricValue $summary "payment_mock_journey_duration" "p(95)" $null
        journey_p99_ms = Get-MetricValue $summary "payment_mock_journey_duration" "p(99)" $null
        intents_created = Get-MetricValue $summary "payment_intent_created_total" "count" $null
        history_ok_rate = Get-MetricValue $summary "payment_history_ok" "rate" $null
        topup_ok_rate = Get-MetricValue $summary "payment_mock_topup_ok" "rate" $null
        refund_ok_rate = Get-MetricValue $summary "payment_refund_ok" "rate" $null
        idempotency_ok_rate = Get-MetricValue $summary "payment_idempotency_ok" "rate" $null
    }
    redis = $redisSnapshot
}
$summaryOutput | ConvertTo-Json -Depth 8 | Set-Content -Path (Join-Path $ReportDir "billing-load-summary.json")

Write-Host "Billing load report: $reportPath"

if ($failures.Count -gt 0 -and -not $ContinueOnFailure) {
    throw "Billing load run failed: $($failures -join '; ')"
}
