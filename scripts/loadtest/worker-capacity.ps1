param(
    [string]$EnvFile = ".env.loadtest",
    [string]$BaseUrl = "",
    [string[]]$Workloads = @(),
    [string]$Duration = "",
    [string]$ReportDir = "",
    [switch]$UseDockerCompose,
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

        $parts = $trimmed -split "=", 2
        if ($parts.Count -ne 2) {
            continue
        }

        $key = $parts[0].Trim()
        $value = $parts[1].Trim()
        if (($value.StartsWith('"') -and $value.EndsWith('"')) -or ($value.StartsWith("'") -and $value.EndsWith("'"))) {
            $value = $value.Substring(1, $value.Length - 2)
        }
        $values[$key] = $value
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
    if ($Values.ContainsKey($Name) -and $Values[$Name] -ne "") {
        return $Values[$Name]
    }
    return $Default
}

function Split-SettingList {
    param([string]$Value)

    if ([string]::IsNullOrWhiteSpace($Value)) {
        return @()
    }

    return @($Value -split "," | ForEach-Object { $_.Trim().ToLowerInvariant() } | Where-Object { $_ -ne "" })
}

function New-K6EnvArgs {
    param([hashtable]$Values)

    $args = @()
    foreach ($key in ($Values.Keys | Sort-Object)) {
        if ($null -eq $Values[$key]) {
            continue
        }
        $args += @("-e", "$key=$($Values[$key])")
    }
    return $args
}

function Get-MetricValue {
    param(
        [object]$Summary,
        [string]$Metric,
        [string]$ValueName,
        [double]$Default = 0
    )

    if ($null -eq $Summary.metrics) {
        return $Default
    }

    $metricObject = $Summary.metrics.PSObject.Properties[$Metric]
    if ($null -eq $metricObject) {
        return $Default
    }

    $valueObject = $metricObject.Value.values.PSObject.Properties[$ValueName]
    if ($null -eq $valueObject -or $null -eq $valueObject.Value) {
        return $Default
    }

    return [double]$valueObject.Value
}

function Read-K6Summary {
    param([string]$Path)

    if (-not (Test-Path $Path)) {
        return $null
    }

    return Get-Content $Path -Raw | ConvertFrom-Json
}

function Read-RedisSnapshot {
    param([string]$Path)

    if (-not (Test-Path $Path)) {
        return $null
    }

    return Get-Content $Path -Raw | ConvertFrom-Json
}

function Capture-DockerStats {
    param([string]$OutputFile)

    $docker = Get-Command docker -ErrorAction SilentlyContinue
    if (-not $docker) {
        Set-Content -Path $OutputFile -Value "docker cli not found"
        return
    }

    $stats = & docker stats --no-stream --format "table {{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.NetIO}}\t{{.BlockIO}}" 2>&1
    Set-Content -Path $OutputFile -Value $stats
}

function Get-WorkloadRate {
    param(
        [hashtable]$Values,
        [string]$Workload
    )

    $upper = $Workload.ToUpperInvariant()
    $specific = Get-Setting $Values "K6_WORKER_CAPACITY_${upper}_RATE" ""
    if ($specific -ne "") {
        return $specific
    }

    return Get-Setting $Values "K6_JOB_${upper}_RATE" ""
}

function Get-UrlHost {
    param([string]$Value)

    if ([string]::IsNullOrWhiteSpace($Value)) {
        return ""
    }
    try {
        $uri = [Uri]$Value
        if (-not [string]::IsNullOrWhiteSpace($uri.Host)) {
            return $uri.Host.ToLowerInvariant()
        }
    } catch {
        return ""
    }
    return ""
}

function New-ReportRow {
    param(
        [string]$Workload,
        [string]$Status,
        [object]$K6Summary,
        [object]$RedisSnapshot,
        [string]$Dir
    )

    $httpErrorRate = 0
    $httpP95 = 0
    $httpP99 = 0
    $jobsCreated = 0
    $jobsCreatedRate = 0
    $jobsTerminal = 0
    $jobsTerminalRate = 0
    $jobFailures = 0
    $jobCompletionP95 = 0
    $jobCompletionP99 = 0

    if ($null -ne $K6Summary) {
        $httpErrorRate = Get-MetricValue $K6Summary "http_req_failed" "rate"
        $httpP95 = Get-MetricValue $K6Summary "http_req_duration" "p(95)"
        $httpP99 = Get-MetricValue $K6Summary "http_req_duration" "p(99)"
        $jobsCreated = Get-MetricValue $K6Summary "job_created_total" "count"
        $jobsCreatedRate = Get-MetricValue $K6Summary "job_created_total" "rate"
        $jobsTerminal = Get-MetricValue $K6Summary "job_terminal_total" "count"
        $jobsTerminalRate = Get-MetricValue $K6Summary "job_terminal_total" "rate"
        $jobFailures = Get-MetricValue $K6Summary "job_terminal_failure_total" "count"
        $jobCompletionP95 = Get-MetricValue $K6Summary "job_completion_duration" "p(95)"
        $jobCompletionP99 = Get-MetricValue $K6Summary "job_completion_duration" "p(99)"
    }

    $redisBacklog = 0
    $redisDLQ = 0
    $redisMemory = ""
    if ($null -ne $RedisSnapshot) {
        $redisBacklog = [int64]$RedisSnapshot.total_backlog
        $redisDLQ = [int64]$RedisSnapshot.dlq_depth
        $redisMemory = [string]$RedisSnapshot.used_memory_human
    }

    return [pscustomobject]@{
        workload = $Workload
        status = $Status
        jobs_created = [int64]$jobsCreated
        jobs_created_per_sec = [math]::Round($jobsCreatedRate, 3)
        jobs_terminal = [int64]$jobsTerminal
        worker_throughput_per_sec = [math]::Round($jobsTerminalRate, 3)
        job_failures = [int64]$jobFailures
        job_completion_p95_ms = [math]::Round($jobCompletionP95, 2)
        job_completion_p99_ms = [math]::Round($jobCompletionP99, 2)
        http_error_rate = [math]::Round($httpErrorRate, 6)
        http_p95_ms = [math]::Round($httpP95, 2)
        http_p99_ms = [math]::Round($httpP99, 2)
        redis_backlog = $redisBacklog
        redis_dlq = $redisDLQ
        redis_memory = $redisMemory
        report_dir = $Dir
    }
}

function Write-WorkerReport {
    param(
        [array]$Rows,
        [string]$OutputFile,
        [string]$BaseUrl,
        [string]$DurationValue,
        [array]$Failures
    )

    $lines = @()
    $lines += "# Worker Capacity Report"
    $lines += ""
    $lines += ('- Base URL: `{0}`' -f $BaseUrl)
    $lines += ('- Duration per workload: `{0}`' -f $DurationValue)
    $lines += "- Generated at: $(Get-Date -Format o)"
    $lines += ""
    $lines += "## Summary"
    $lines += ""
    $lines += "| Workload | Status | Created | Created/s | Terminal | Throughput/s | Job p95 ms | Job p99 ms | HTTP error rate | Redis backlog | DLQ | Redis memory |"
    $lines += "| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |"
    foreach ($row in $Rows) {
        $lines += "| $($row.workload) | $($row.status) | $($row.jobs_created) | $($row.jobs_created_per_sec) | $($row.jobs_terminal) | $($row.worker_throughput_per_sec) | $($row.job_completion_p95_ms) | $($row.job_completion_p99_ms) | $($row.http_error_rate) | $($row.redis_backlog) | $($row.redis_dlq) | $($row.redis_memory) |"
    }
    $lines += ""
    $lines += "## Diagnostics"
    $lines += ""
    foreach ($row in $Rows) {
        $relative = Split-Path -Leaf $row.report_dir
        $lines += ('- `{0}`: `{1}/k6-summary.json`, `{1}/k6-output.log`, `{1}/redis-diagnostics.md`, `{1}/docker-stats.txt`' -f $row.workload, $relative)
    }
    $lines += ""
    $lines += "## Worker Isolation Notes"
    $lines += ""
    $lines += "- Compare `text` throughput/latency with `mixed` throughput/latency. If mixed workload degrades text-like completion heavily, split queues/workers by job type."
    $lines += "- DLQ must stay 0 unless this run was intentionally allowed to tolerate DLQ entries."
    $lines += "- Redis backlog after each workload should return close to 0 for a stable worker capacity baseline."
    $lines += ""

    if ($Failures.Count -gt 0) {
        $lines += "## Failures"
        $lines += ""
        foreach ($failure in $Failures) {
            $lines += "- $failure"
        }
        $lines += ""
    }

    Set-Content -Path $OutputFile -Value $lines
}

$envValues = Read-EnvFile $EnvFile
$appEnv = Get-Setting $envValues "APP_ENV" ""
if ($appEnv -eq "production") {
    throw "Refusing to run worker capacity test with APP_ENV=production"
}

if ($BaseUrl -eq "") {
    $BaseUrl = Get-Setting $envValues "K6_BASE_URL" ""
}
if ($BaseUrl -eq "") {
    throw "K6_BASE_URL is required. Set it in $EnvFile or pass -BaseUrl."
}

$blockedHosts = @("vk.neiirohub.ru", "app.neiirohub.ru", "neiirohub.ru")
$baseUrlHost = Get-UrlHost $BaseUrl
if ($blockedHosts -contains $baseUrlHost) {
    throw "Refusing to run worker capacity test against production host: $baseUrlHost"
}

$preflightScript = Join-Path $PSScriptRoot "loadtest-preflight.ps1"
if (Test-Path $preflightScript) {
    & powershell -NoProfile -ExecutionPolicy Bypass -File $preflightScript -EnvFile $EnvFile
    if ($LASTEXITCODE -ne 0) {
        throw "loadtest preflight failed"
    }
}

if ($Workloads.Count -eq 0) {
    $Workloads = Split-SettingList (Get-Setting $envValues "K6_WORKER_CAPACITY_WORKLOADS" "text,image,video,mixed")
}
if ($Workloads.Count -eq 0) {
    throw "No workloads selected"
}

if ($Duration -eq "") {
    $Duration = Get-Setting $envValues "K6_WORKER_CAPACITY_DURATION" (Get-Setting $envValues "K6_JOB_DURATION" "1m")
}

if ($ReportDir -eq "") {
    $timestamp = Get-Date -Format "yyyyMMdd-HHmmss"
    $ReportDir = Join-Path "artifacts/loadtest" "worker-capacity-$timestamp"
}
New-Item -ItemType Directory -Force -Path $ReportDir | Out-Null

$k6 = Get-Command k6 -ErrorAction SilentlyContinue
if (-not $k6) {
    throw "k6 is not installed or not available in PATH"
}

$script = "tests/k6/job-worker.js"
if (-not (Test-Path $script)) {
    throw "Missing k6 script: $script"
}

$reservedK6ConfigKeys = @("K6_DURATION", "K6_ITERATIONS", "K6_SCENARIOS", "K6_STAGES", "K6_VUS")
$baseK6Env = @{}
foreach ($key in ($envValues.Keys | Sort-Object)) {
    if ($key -like "K6_*" -and $reservedK6ConfigKeys -notcontains $key) {
        $baseK6Env[$key] = $envValues[$key]
    }
}

$rows = @()
$failures = @()
$failOnDLQ = (Get-Setting $envValues "K6_WORKER_CAPACITY_FAIL_ON_DLQ" "true").ToLowerInvariant() -ne "false"

foreach ($rawWorkload in $Workloads) {
    $workload = $rawWorkload.Trim().ToLowerInvariant()
    if ($workload -notin @("text", "image", "video", "mixed", "all")) {
        throw "Unsupported workload '$workload'. Allowed: text,image,video,mixed,all"
    }

    $stepDir = Join-Path $ReportDir $workload
    New-Item -ItemType Directory -Force -Path $stepDir | Out-Null
    $summaryPath = Join-Path $stepDir "k6-summary.json"
    $outputPath = Join-Path $stepDir "k6-output.log"

    $k6Env = @{}
    foreach ($key in $baseK6Env.Keys) {
        $k6Env[$key] = $baseK6Env[$key]
    }

    $overrides = @{
        K6_BASE_URL = $BaseUrl
        K6_RUN = "1"
        K6_ALLOW_PRODUCTION_LIVE_SMOKE = "false"
        K6_JOB_WORKLOAD = $workload
        K6_JOB_DURATION = $Duration
        K6_JOB_POLL = "true"
        K6_JOB_SLEEP_SECONDS = "0"
    }

    $rate = Get-WorkloadRate $envValues $workload
    if ($rate -ne "") {
        $rateKey = "K6_JOB_{0}_RATE" -f $workload.ToUpperInvariant()
        $overrides[$rateKey] = $rate
    }
    foreach ($key in $overrides.Keys) {
        $k6Env[$key] = $overrides[$key]
    }

    Write-Host "Running worker capacity workload '$workload' for $Duration..."
    $k6Args = @("run") + (New-K6EnvArgs $k6Env) + @("--summary-export", $summaryPath, $script)
    $k6Output = & k6 @k6Args 2>&1
    $exitCode = $LASTEXITCODE
    Set-Content -Path $outputPath -Value $k6Output

    $redisSnapshotPath = Join-Path $stepDir "redis-diagnostics.snapshot.json"
    if (-not $SkipRedis) {
        $redisReportPath = Join-Path $stepDir "redis-diagnostics.md"
        $redisArgs = @("-NoProfile", "-ExecutionPolicy", "Bypass", "-File", (Join-Path $PSScriptRoot "redis-diagnostics.ps1"), "-EnvFile", $EnvFile, "-OutputFile", $redisReportPath, "-SnapshotFile", $redisSnapshotPath)
        if ($UseDockerCompose) {
            $redisArgs += "-UseDockerCompose"
        }
        & powershell @redisArgs
        if ($LASTEXITCODE -ne 0) {
            $failures += "$workload redis diagnostics failed"
        }
    }

    if (-not $SkipDockerStats) {
        Capture-DockerStats (Join-Path $stepDir "docker-stats.txt")
    }

    $summary = Read-K6Summary $summaryPath
    $redisSnapshot = Read-RedisSnapshot $redisSnapshotPath
    $status = if ($exitCode -eq 0) { "ok" } else { "failed" }
    $row = New-ReportRow -Workload $workload -Status $status -K6Summary $summary -RedisSnapshot $redisSnapshot -Dir $stepDir
    $rows += $row

    if ($exitCode -ne 0) {
        $failures += "$workload k6 failed with exit code $exitCode"
    }
    if (-not $AllowDLQ -and $failOnDLQ -and $row.redis_dlq -gt 0) {
        $failures += "$workload produced DLQ entries: $($row.redis_dlq)"
    }

    if ($exitCode -ne 0 -and -not $ContinueOnFailure) {
        break
    }
}

$reportPath = Join-Path $ReportDir "worker-capacity-report.md"
Write-WorkerReport -Rows $rows -OutputFile $reportPath -BaseUrl $BaseUrl -DurationValue $Duration -Failures $failures

$summaryOutput = [pscustomobject]@{
    generated_at = (Get-Date -Format o)
    base_url = $BaseUrl
    duration = $Duration
    workloads = $rows
    failures = $failures
}
$summaryOutput | ConvertTo-Json -Depth 8 | Set-Content -Path (Join-Path $ReportDir "worker-capacity-summary.json")

Write-Host "Worker capacity report: $reportPath"

if ($failures.Count -gt 0 -and -not $ContinueOnFailure) {
    throw "Worker capacity run failed: $($failures -join '; ')"
}
