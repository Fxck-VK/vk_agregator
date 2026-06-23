[CmdletBinding()]
param(
    [string]$EnvFile = ".env.loadtest",
    [string]$ReportDir = "",
    [string]$BaseUrl = "",
    [string[]]$K6Scripts = @(),
    [string[]]$K6SummaryFiles = @(),
    [switch]$RunK6,
    [switch]$SkipPostgres,
    [switch]$SkipRedis,
    [switch]$SkipDockerStats,
    [switch]$UseDockerCompose,
    [switch]$AllowProduction
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$RepoRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..")
Set-Location $RepoRoot

function Read-EnvFile {
    param([string]$Path)

    $values = @{}
    if (-not (Test-Path $Path)) {
        return $values
    }

    foreach ($line in Get-Content -LiteralPath $Path) {
        $trimmed = $line.Trim()
        if ($trimmed.Length -eq 0 -or $trimmed.StartsWith("#")) {
            continue
        }

        $idx = $trimmed.IndexOf("=")
        if ($idx -lt 1) {
            continue
        }

        $key = $trimmed.Substring(0, $idx).Trim()
        $value = $trimmed.Substring($idx + 1).Trim()
        if (($value.StartsWith('"') -and $value.EndsWith('"')) -or ($value.StartsWith("'") -and $value.EndsWith("'"))) {
            $value = $value.Substring(1, $value.Length - 2)
        }

        $values[$key] = $value
    }

    return $values
}

$EnvValues = Read-EnvFile -Path $EnvFile

function Get-Setting {
    param(
        [string]$Name,
        [string]$Default = ""
    )

    if ($EnvValues.ContainsKey($Name) -and $EnvValues[$Name] -ne "") {
        return $EnvValues[$Name]
    }

    $fromProcess = [Environment]::GetEnvironmentVariable($Name)
    if (-not [string]::IsNullOrWhiteSpace($fromProcess)) {
        return $fromProcess
    }

    return $Default
}

function Set-ProcessEnvFromFile {
    foreach ($key in $EnvValues.Keys) {
        if (-not [string]::IsNullOrWhiteSpace($key)) {
            # k6 treats K6_* process variables as its own runtime config.
            # Pass scenario variables through `k6 -e` instead so script options stay intact.
            if ($key -like "K6_*") {
                continue
            }
            [Environment]::SetEnvironmentVariable($key, [string]$EnvValues[$key], "Process")
        }
    }
}

function Split-SettingList {
    param(
        [string[]]$Explicit,
        [string]$EnvName,
        [string[]]$Default
    )

    if ($Explicit.Count -gt 0) {
        return $Explicit | ForEach-Object { $_.Trim() } | Where-Object { $_ -ne "" }
    }

    $fromEnv = Get-Setting -Name $EnvName
    if (-not [string]::IsNullOrWhiteSpace($fromEnv)) {
        return $fromEnv.Split(",") | ForEach-Object { $_.Trim() } | Where-Object { $_ -ne "" }
    }

    return $Default
}

function Get-JsonMetricValue {
    param(
        [object]$Metrics,
        [string]$Metric,
        [string]$Value,
        [object]$Default = $null
    )

    if ($null -eq $Metrics) {
        return $Default
    }
    $metricProp = $Metrics.PSObject.Properties[$Metric]
    if ($null -eq $metricProp) {
        return $Default
    }

    $metricValue = $metricProp.Value
    $valuesProp = $metricValue.PSObject.Properties["values"]
    if ($null -ne $valuesProp) {
        $metricValue = $valuesProp.Value
    }

    $valueProp = $metricValue.PSObject.Properties[$Value]
    if ($null -eq $valueProp) {
        return $Default
    }
    return $valueProp.Value
}

function Get-K6RateMetricValue {
    param(
        [object]$Metrics,
        [string]$Metric,
        [object]$Default = 0
    )

    $rate = Get-JsonMetricValue -Metrics $Metrics -Metric $Metric -Value "rate" -Default $null
    if ($null -ne $rate) {
        return $rate
    }

    return Get-JsonMetricValue -Metrics $Metrics -Metric $Metric -Value "value" -Default $Default
}

function Format-NullableNumber {
    param(
        [object]$Value,
        [int]$Digits = 2
    )

    if ($null -eq $Value) {
        return "-"
    }

    try {
        return ([double]$Value).ToString("F$Digits", [Globalization.CultureInfo]::InvariantCulture)
    } catch {
        return [string]$Value
    }
}

function Read-K6Summary {
    param([string]$Path)

    $json = Get-Content -LiteralPath $Path -Raw | ConvertFrom-Json
    $metrics = $json.metrics
    $scriptName = [IO.Path]::GetFileNameWithoutExtension($Path)
    $httpReqs = Get-JsonMetricValue -Metrics $metrics -Metric "http_reqs" -Value "count" -Default 0
    $httpRps = Get-JsonMetricValue -Metrics $metrics -Metric "http_reqs" -Value "rate" -Default 0
    $p95 = Get-JsonMetricValue -Metrics $metrics -Metric "http_req_duration" -Value "p(95)"
    $p99 = Get-JsonMetricValue -Metrics $metrics -Metric "http_req_duration" -Value "p(99)"
    $failedRate = Get-K6RateMetricValue -Metrics $metrics -Metric "http_req_failed" -Default 0
    $iterationsRate = Get-JsonMetricValue -Metrics $metrics -Metric "iterations" -Value "rate" -Default 0
    $jobTerminalRate = Get-K6RateMetricValue -Metrics $metrics -Metric "job_terminal_total" -Default $null
    $jobCreatedRate = Get-K6RateMetricValue -Metrics $metrics -Metric "job_created_total" -Default $null
    $jobSuccessRate = Get-K6RateMetricValue -Metrics $metrics -Metric "job_success_ok" -Default $null

    return [PSCustomObject]@{
        Script            = $scriptName
        File              = $Path
        HttpRequests      = [double]$httpReqs
        Rps               = [double]$httpRps
        P95Ms             = $p95
        P99Ms             = $p99
        ErrorRate         = [double]$failedRate
        IterationsPerSec  = [double]$iterationsRate
        WorkerThroughput  = $jobTerminalRate
        JobCreateRate     = $jobCreatedRate
        JobSuccessRate    = $jobSuccessRate
    }
}

function Invoke-K6Scripts {
    param(
        [string[]]$Scripts,
        [string]$OutputDir
    )

    $summaries = New-Object System.Collections.Generic.List[string]
    $failures = New-Object System.Collections.Generic.List[string]

    if (-not (Get-Command k6 -ErrorAction SilentlyContinue)) {
        $failures.Add("k6 is not installed or not in PATH.")
        return @{ Summaries = @($summaries); Failures = @($failures) }
    }

    foreach ($script in $Scripts) {
        if (-not (Test-Path $script)) {
            $failures.Add("k6 script not found: $script")
            continue
        }

        $name = [IO.Path]::GetFileNameWithoutExtension($script)
        $summaryFile = Join-Path $OutputDir "$name.summary.json"
        $logFile = Join-Path $OutputDir "$name.k6.log"
        $k6EnvArgs = @()
        $reservedK6ConfigKeys = @("K6_DURATION", "K6_ITERATIONS", "K6_SCENARIOS", "K6_STAGES", "K6_VUS")
        foreach ($key in ($EnvValues.Keys | Sort-Object)) {
            if ($key -like "K6_*") {
                if ($reservedK6ConfigKeys -contains $key) {
                    continue
                }
                $k6EnvArgs += @("-e", "$key=$($EnvValues[$key])")
            }
        }
        if (-not [string]::IsNullOrWhiteSpace($script:ResolvedK6BaseUrl) -and -not $EnvValues.ContainsKey("K6_BASE_URL")) {
            $k6EnvArgs += @("-e", "K6_BASE_URL=$script:ResolvedK6BaseUrl")
        }
        $args = @("run") + $k6EnvArgs + @("--summary-export", $summaryFile, $script)

        Write-Host "Running k6 $script..."
        $previousErrorActionPreference = $ErrorActionPreference
        $ErrorActionPreference = "Continue"
        try {
            $output = & k6 @args 2>&1 | ForEach-Object { $_.ToString() }
            $exitCode = $LASTEXITCODE
        } finally {
            $ErrorActionPreference = $previousErrorActionPreference
        }
        $output | Set-Content -LiteralPath $logFile -Encoding UTF8
        if (Test-Path $summaryFile) {
            $summaries.Add($summaryFile)
        }
        if ($exitCode -ne 0) {
            $failures.Add("k6 $script failed with exit code $exitCode. See $logFile")
        }
    }

    return @{ Summaries = @($summaries); Failures = @($failures) }
}

function Resolve-RedisEndpoint {
    param([string]$Address)

    $endpoint = @{ Host = "127.0.0.1"; Port = "6379" }
    if ([string]::IsNullOrWhiteSpace($Address)) {
        return $endpoint
    }
    if ($Address -match "^[a-z]+://") {
        $uri = [Uri]$Address
        $endpoint.Host = $uri.Host
        if ($uri.Port -gt 0) {
            $endpoint.Port = [string]$uri.Port
        }
        return $endpoint
    }
    if ($Address -match "^(?<host>[^:]+):(?<port>\d+)$") {
        $endpoint.Host = $Matches["host"]
        $endpoint.Port = $Matches["port"]
        return $endpoint
    }
    $endpoint.Host = $Address
    return $endpoint
}

function Invoke-RedisCli {
    param(
        [string[]]$RedisArguments,
        [switch]$AllowFailure
    )

    if ($script:RedisUseDockerCompose) {
        $composeArgs = @("compose", "-p", $script:ComposeProjectName)
        if (Test-Path $script:EnvFilePath) {
            $composeArgs += @("--env-file", $script:EnvFilePath)
        }
        $composeArgs += @("-f", "docker-compose.data.yml", "exec", "-T")
        if (-not [string]::IsNullOrWhiteSpace($script:RedisPassword)) {
            $composeArgs += @("-e", "REDISCLI_AUTH=$($script:RedisPassword)")
        }
        $composeArgs += @("redis", "redis-cli", "--raw", "-n", "$script:RedisDb")
        $composeArgs += $RedisArguments
        $result = & docker @composeArgs 2>&1
        $exitCode = $LASTEXITCODE
    } else {
        $endpoint = Resolve-RedisEndpoint -Address $script:RedisAddr
        $redisArgs = @("--raw", "-h", $endpoint.Host, "-p", $endpoint.Port, "-n", "$script:RedisDb")
        $redisArgs += $RedisArguments
        $previousAuth = [Environment]::GetEnvironmentVariable("REDISCLI_AUTH")
        try {
            if (-not [string]::IsNullOrWhiteSpace($script:RedisPassword)) {
                $env:REDISCLI_AUTH = $script:RedisPassword
            }
            $result = & redis-cli @redisArgs 2>&1
            $exitCode = $LASTEXITCODE
        } finally {
            if ($null -eq $previousAuth) {
                Remove-Item Env:\REDISCLI_AUTH -ErrorAction SilentlyContinue
            } else {
                $env:REDISCLI_AUTH = $previousAuth
            }
        }
    }

    if ($exitCode -ne 0) {
        if ($AllowFailure) {
            return @("WARN: redis command failed with exit code $exitCode")
        }
        throw "redis-cli command failed with exit code $exitCode"
    }
    return $result
}

function Parse-RedisInfo {
    param([object[]]$Lines)

    $info = @{}
    foreach ($line in $Lines) {
        $text = [string]$line
        if ($text.StartsWith("#") -or $text.Trim() -eq "") {
            continue
        }
        $idx = $text.IndexOf(":")
        if ($idx -gt 0) {
            $info[$text.Substring(0, $idx)] = $text.Substring($idx + 1).Trim()
        }
    }
    return $info
}

function Get-RedisSnapshot {
    param([string[]]$Streams)

    $memory = Parse-RedisInfo -Lines (Invoke-RedisCli -RedisArguments @("INFO", "memory") -AllowFailure)
    $stats = Parse-RedisInfo -Lines (Invoke-RedisCli -RedisArguments @("INFO", "stats") -AllowFailure)
    $streamRows = @()
    $totalStreamLength = 0
    $totalBacklog = 0
    $dlqDepth = 0

    foreach ($stream in $Streams) {
        $xlenRaw = @(Invoke-RedisCli -RedisArguments @("XLEN", $stream) -AllowFailure)
        $xlen = 0
        if ($xlenRaw.Count -gt 0) {
            $parsedXLen = 0
            if ([int]::TryParse(([string]$xlenRaw[0]).Trim(), [ref]$parsedXLen)) {
                $xlen = $parsedXLen
            }
        }

        $pending = 0
        $lag = 0
        $groupInfo = @(Invoke-RedisCli -RedisArguments @("XINFO", "GROUPS", $stream) -AllowFailure)
        for ($i = 0; $i -lt $groupInfo.Count - 1; $i += 1) {
            $key = ([string]$groupInfo[$i]).Trim()
            $value = ([string]$groupInfo[$i + 1]).Trim()
            $parsed = 0
            if ($key -eq "pending" -and [int]::TryParse($value, [ref]$parsed)) {
                $pending += $parsed
            }
            if ($key -eq "lag" -and [int]::TryParse($value, [ref]$parsed)) {
                $lag += $parsed
            }
        }

        $backlog = $pending + $lag
        $totalStreamLength += $xlen
        $totalBacklog += $backlog
        if ($stream -like "*dlq*") {
            $dlqDepth += $xlen
        }
        $streamRows += [PSCustomObject]@{
            Stream = $stream
            Length = $xlen
            Pending = $pending
            Lag = $lag
            Backlog = $backlog
        }
    }

    return [PSCustomObject]@{
        UsedMemoryHuman          = $memory["used_memory_human"]
        UsedMemoryPeakHuman      = $memory["used_memory_peak_human"]
        InstantaneousOpsPerSec   = $stats["instantaneous_ops_per_sec"]
        TotalCommandsProcessed   = $stats["total_commands_processed"]
        RejectedConnections      = $stats["rejected_connections"]
        ExpiredKeys              = $stats["expired_keys"]
        Streams                  = @($streamRows)
        TotalStreamLength        = $totalStreamLength
        TotalQueueDepth          = $totalBacklog
        TotalBacklog             = $totalBacklog
        DlqDepth                 = $dlqDepth
    }
}

function Get-DockerStatsSnapshot {
    if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
        return @()
    }

    $rows = & docker stats --no-stream --format "{{json .}}" 2>&1
    if ($LASTEXITCODE -ne 0) {
        return @([PSCustomObject]@{ Name = "docker stats failed"; CPU = "-"; MemUsage = ($rows -join " ") })
    }

    $stats = @()
    foreach ($row in $rows) {
        if ([string]::IsNullOrWhiteSpace($row)) {
            continue
        }
        try {
            $obj = $row | ConvertFrom-Json
            $stats += [PSCustomObject]@{
                Name     = [string]$obj.Name
                CPU      = [string]$obj.CPUPerc
                MemUsage = [string]$obj.MemUsage
                NetIO    = [string]$obj.NetIO
                BlockIO  = [string]$obj.BlockIO
            }
        } catch {
            $stats += [PSCustomObject]@{ Name = "unparsed"; CPU = "-"; MemUsage = [string]$row; NetIO = "-"; BlockIO = "-" }
        }
    }
    return @($stats)
}

function Add-Line {
    param([string]$Line = "")
    [void]$script:ReportLines.Add($Line)
}

function Add-TableRow {
    param([string[]]$Cells)
    Add-Line ("| " + (($Cells | ForEach-Object { ($_ -replace "\|", "\\|") }) -join " | ") + " |")
}

$appEnv = Get-Setting -Name "APP_ENV"
$allowProdFromEnv = (Get-Setting -Name "ALLOW_PRODUCTION_LOADTEST_REPORT" -Default "false").ToLowerInvariant() -eq "true"
if ($appEnv -eq "production" -and -not $AllowProduction -and -not $allowProdFromEnv) {
    throw "Refusing to run load-test report against APP_ENV=production. Pass -AllowProduction only for an approved production audit."
}

Set-ProcessEnvFromFile
if (-not [string]::IsNullOrWhiteSpace($BaseUrl)) {
    $script:ResolvedK6BaseUrl = $BaseUrl
} else {
    $BaseUrl = Get-Setting -Name "K6_BASE_URL" -Default (Get-Setting -Name "BASE_URL")
    $script:ResolvedK6BaseUrl = $BaseUrl
}

if ([string]::IsNullOrWhiteSpace($ReportDir)) {
    $stamp = Get-Date -Format "yyyyMMdd-HHmmss"
    $ReportDir = Join-Path "reports/loadtest" $stamp
}
New-Item -ItemType Directory -Force -Path $ReportDir | Out-Null

$defaultK6Scripts = @(
    "tests/k6/basic-api.js",
    "tests/k6/vk-bot.js",
    "tests/k6/job-worker.js",
    "tests/k6/billing-payments.js"
)
$K6Scripts = Split-SettingList -Explicit $K6Scripts -EnvName "LOADTEST_REPORT_K6_SCRIPTS" -Default $defaultK6Scripts

$script:EnvFilePath = $EnvFile
$script:RedisAddr = Get-Setting -Name "REDIS_ADDR" -Default "localhost:6379"
$script:RedisPassword = Get-Setting -Name "REDIS_PASSWORD"
$script:RedisDb = [int](Get-Setting -Name "REDIS_DB" -Default "0")
$script:ComposeProjectName = Get-Setting -Name "COMPOSE_PROJECT_NAME"
if ([string]::IsNullOrWhiteSpace($script:ComposeProjectName)) {
    $script:ComposeProjectName = Get-Setting -Name "COMPOSE_NETWORK_NAME" -Default "vk-ai-aggregator-loadtest"
}
$script:RedisUseDockerCompose = $UseDockerCompose
if (-not $script:RedisUseDockerCompose -and -not (Get-Command redis-cli -ErrorAction SilentlyContinue)) {
    $script:RedisUseDockerCompose = $true
}

$failures = New-Object System.Collections.Generic.List[string]
$summaryFiles = New-Object System.Collections.Generic.List[string]
foreach ($file in $K6SummaryFiles) {
    if (Test-Path $file) {
        $summaryFiles.Add((Resolve-Path $file).Path)
    } else {
        $failures.Add("k6 summary file not found: $file")
    }
}

if ($RunK6) {
    if ([string]::IsNullOrWhiteSpace($BaseUrl)) {
        $failures.Add("K6_BASE_URL is empty; k6 execution skipped to avoid producing a meaningless no-op report.")
    } else {
        $k6Result = Invoke-K6Scripts -Scripts $K6Scripts -OutputDir $ReportDir
        foreach ($file in $k6Result.Summaries) {
            $summaryFiles.Add($file)
        }
        foreach ($failure in $k6Result.Failures) {
            $failures.Add($failure)
        }
    }
}

$k6Rows = New-Object System.Collections.Generic.List[object]
foreach ($summaryFile in $summaryFiles) {
    try {
        $k6Rows.Add((Read-K6Summary -Path $summaryFile))
    } catch {
        $failures.Add("Failed to parse k6 summary ${summaryFile}: $($_.Exception.Message)")
    }
}

$postgresOutputFile = Join-Path $ReportDir "postgres-diagnostics.md"
if (-not $SkipPostgres) {
    try {
        $pgArgs = @("-NoProfile", "-ExecutionPolicy", "Bypass", "-File", "scripts/loadtest/postgres-diagnostics.ps1", "-EnvFile", $EnvFile, "-OutputFile", $postgresOutputFile)
        if ($UseDockerCompose) {
            $pgArgs += "-UseDockerCompose"
        }
        if ($AllowProduction) {
            $pgArgs += "-AllowProduction"
        }
        & powershell @pgArgs | Tee-Object -FilePath (Join-Path $ReportDir "postgres-diagnostics.log") | Out-Null
        if ($LASTEXITCODE -ne 0) {
            $failures.Add("PostgreSQL diagnostics failed with exit code $LASTEXITCODE.")
        }
    } catch {
        $failures.Add("PostgreSQL diagnostics failed: $($_.Exception.Message)")
    }
}

$redisOutputFile = Join-Path $ReportDir "redis-diagnostics.md"
$redisSnapshot = $null
if (-not $SkipRedis) {
    try {
        $redisArgs = @("-NoProfile", "-ExecutionPolicy", "Bypass", "-File", "scripts/loadtest/redis-diagnostics.ps1", "-EnvFile", $EnvFile, "-OutputFile", $redisOutputFile)
        if ($UseDockerCompose) {
            $redisArgs += "-UseDockerCompose"
        }
        if ($AllowProduction) {
            $redisArgs += "-AllowProduction"
        }
        & powershell @redisArgs | Tee-Object -FilePath (Join-Path $ReportDir "redis-diagnostics.log") | Out-Null
        if ($LASTEXITCODE -ne 0) {
            $failures.Add("Redis diagnostics failed with exit code $LASTEXITCODE.")
        }

        $streams = Split-SettingList -Explicit @() -EnvName "LOADTEST_REDIS_STREAMS" -Default @(
            "stream:jobs:text",
            "stream:jobs:image",
            "stream:jobs:video",
            "stream:jobs:delivery",
            "stream:jobs:provider_poll",
            "stream:jobs:dlq"
        )
        $redisSnapshot = Get-RedisSnapshot -Streams $streams
    } catch {
        $failures.Add("Redis snapshot failed: $($_.Exception.Message)")
    }
}

$dockerStats = @()
if (-not $SkipDockerStats) {
    $dockerStats = Get-DockerStatsSnapshot
    $dockerStats | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath (Join-Path $ReportDir "docker-stats.json") -Encoding UTF8
}

$maxErrorRate = [double](Get-Setting -Name "LOADTEST_REPORT_MAX_ERROR_RATE" -Default "0.05")
$maxP95Ms = [double](Get-Setting -Name "LOADTEST_REPORT_MAX_P95_MS" -Default "1500")
$maxQueueDepth = [int](Get-Setting -Name "LOADTEST_REPORT_MAX_QUEUE_DEPTH" -Default "100")
$maxDlqDepth = [int](Get-Setting -Name "LOADTEST_REPORT_MAX_DLQ" -Default "0")
$minJobSuccessRate = [double](Get-Setting -Name "LOADTEST_REPORT_MIN_JOB_SUCCESS_RATE" -Default "0.95")

$findings = New-Object System.Collections.Generic.List[string]
foreach ($row in $k6Rows) {
    if ($row.ErrorRate -gt $maxErrorRate) {
        $findings.Add("High HTTP error rate in $($row.Script): $(Format-NullableNumber $row.ErrorRate 4).")
    }
    if ($null -ne $row.P95Ms -and [double]$row.P95Ms -gt $maxP95Ms) {
        $findings.Add("High p95 latency in $($row.Script): $(Format-NullableNumber $row.P95Ms 2) ms.")
    }
    if ($null -ne $row.JobSuccessRate -and [double]$row.JobSuccessRate -lt $minJobSuccessRate) {
        $findings.Add("Low job success rate in $($row.Script): $(Format-NullableNumber $row.JobSuccessRate 4).")
    }
}
if ($null -ne $redisSnapshot) {
    if ($redisSnapshot.TotalQueueDepth -gt $maxQueueDepth) {
        $findings.Add("Redis backlog exceeds threshold: $($redisSnapshot.TotalQueueDepth).")
    }
    if ($redisSnapshot.DlqDepth -gt $maxDlqDepth) {
        $findings.Add("DLQ is not empty: $($redisSnapshot.DlqDepth).")
    }
}
foreach ($failure in $failures) {
    $findings.Add($failure)
}

$firstBottleneck = "No obvious bottleneck from collected data."
if ($findings.Count -gt 0) {
    $firstBottleneck = $findings[0]
} elseif ($k6Rows.Count -eq 0 -and $null -eq $redisSnapshot -and $dockerStats.Count -eq 0) {
    $firstBottleneck = "No measurements were collected. Run with -RunK6 and enabled diagnostics."
}

$findingsArray = @()
foreach ($finding in $findings) {
    $findingsArray += [string]$finding
}

$k6RowsArray = @()
foreach ($row in $k6Rows) {
    $k6RowsArray += $row
}

$dockerStatsArray = @()
foreach ($row in $dockerStats) {
    $dockerStatsArray += $row
}

$maxObservedRps = $null
$maxObservedP95Ms = $null
$maxObservedP99Ms = $null
$maxObservedErrorRate = $null
$totalJobCreateRate = 0.0
$totalWorkerThroughput = 0.0
$hasJobMetrics = $false
foreach ($row in $k6Rows) {
    if ($null -ne $row.Rps -and ($null -eq $maxObservedRps -or [double]$row.Rps -gt [double]$maxObservedRps)) {
        $maxObservedRps = [double]$row.Rps
    }
    if ($null -ne $row.P95Ms -and ($null -eq $maxObservedP95Ms -or [double]$row.P95Ms -gt [double]$maxObservedP95Ms)) {
        $maxObservedP95Ms = [double]$row.P95Ms
    }
    if ($null -ne $row.P99Ms -and ($null -eq $maxObservedP99Ms -or [double]$row.P99Ms -gt [double]$maxObservedP99Ms)) {
        $maxObservedP99Ms = [double]$row.P99Ms
    }
    if ($null -ne $row.ErrorRate -and ($null -eq $maxObservedErrorRate -or [double]$row.ErrorRate -gt [double]$maxObservedErrorRate)) {
        $maxObservedErrorRate = [double]$row.ErrorRate
    }
    if ($null -ne $row.JobCreateRate) {
        $totalJobCreateRate += [double]$row.JobCreateRate
        $hasJobMetrics = $true
    }
    if ($null -ne $row.WorkerThroughput) {
        $totalWorkerThroughput += [double]$row.WorkerThroughput
        $hasJobMetrics = $true
    }
}

$script:ScalingDecisions = New-Object System.Collections.Generic.List[object]
function Add-ScalingDecision {
    param(
        [string]$Area,
        [string]$Decision,
        [string]$Reason
    )

    $script:ScalingDecisions.Add([PSCustomObject]@{
        area = $Area
        decision = $Decision
        reason = $Reason
    })
}

if ($findings.Count -gt 0) {
    Add-ScalingDecision -Area "first bottleneck" -Decision "fix before scaling" -Reason $firstBottleneck
} else {
    Add-ScalingDecision -Area "first bottleneck" -Decision "not found in this run" -Reason "Collected metrics did not cross configured error, latency, queue or DLQ thresholds."
}

if ($hasJobMetrics) {
    if ($totalJobCreateRate -gt 0 -and $totalWorkerThroughput -gt 0 -and $totalWorkerThroughput -lt ($totalJobCreateRate * 0.8)) {
        Add-ScalingDecision -Area "workers" -Decision "add worker capacity" -Reason ("Worker throughput ({0}/s) is below job create rate ({1}/s)." -f (Format-NullableNumber $totalWorkerThroughput 2), (Format-NullableNumber $totalJobCreateRate 2))
    } elseif ($null -ne $redisSnapshot -and $redisSnapshot.TotalQueueDepth -eq 0 -and $redisSnapshot.DlqDepth -eq 0) {
        Add-ScalingDecision -Area "workers" -Decision "keep current worker count for this load level" -Reason "Job metrics were collected and Redis backlog/DLQ stayed empty."
    } else {
        Add-ScalingDecision -Area "workers" -Decision "review worker throughput with Redis diagnostics" -Reason "Job metrics were collected, but queue diagnostics are missing or inconclusive."
    }
} else {
    Add-ScalingDecision -Area "workers" -Decision "run job-worker scenario before sizing" -Reason "No job create/terminal metrics were collected."
}

if ($null -ne $redisSnapshot) {
    $backloggedStreams = @()
    foreach ($stream in $redisSnapshot.Streams) {
        if ([int]$stream.Backlog -gt 0) {
            $backloggedStreams += ("{0}={1}" -f $stream.Stream, $stream.Backlog)
        }
    }

    if ($backloggedStreams.Count -eq 0) {
        Add-ScalingDecision -Area "queues" -Decision "do not split text/image/video queues yet" -Reason "No per-stream backlog was observed. Split queues only after one modality consistently blocks others."
    } else {
        Add-ScalingDecision -Area "queues" -Decision "consider per-modality workers/queues" -Reason ("Backlog observed: {0}." -f ($backloggedStreams -join ", "))
    }

    if ($redisSnapshot.DlqDepth -eq 0) {
        Add-ScalingDecision -Area "DLQ/retries" -Decision "no retry storm detected" -Reason "DLQ depth is zero in the collected snapshot."
    } else {
        Add-ScalingDecision -Area "DLQ/retries" -Decision "inspect retry classification before more load" -Reason ("DLQ depth is {0}." -f $redisSnapshot.DlqDepth)
    }
} else {
    Add-ScalingDecision -Area "queues" -Decision "collect Redis diagnostics" -Reason "Queue depth and DLQ were not available."
}

if (-not $SkipPostgres -and (Test-Path $postgresOutputFile)) {
    $postgresDiagnosticsText = Get-Content -LiteralPath $postgresOutputFile -Raw
    $longQueriesClean = $postgresDiagnosticsText -match "(?s)== Long-running queries ==.*?\(0 rows\)"
    $blockingLocksClean = $postgresDiagnosticsText -match "(?s)== Blocking locks ==.*?\(0 rows\)"
    if ($longQueriesClean -and $blockingLocksClean) {
        Add-ScalingDecision -Area "Postgres" -Decision "no immediate SQL blocker in this run" -Reason "Diagnostics did not show long-running queries or blocking locks. Re-check after higher sustained RPS."
    } else {
        Add-ScalingDecision -Area "Postgres" -Decision "inspect SQL before adding API replicas" -Reason "Diagnostics did not prove long-running queries/locks are clean."
    }
    if ($postgresDiagnosticsText -match "== Sequential scan candidates ==") {
        Add-ScalingDecision -Area "indexes" -Decision "watch sequential-scan candidates under larger data volume" -Reason "The report includes a sequential scan section; add indexes only for repeated hot queries, not one-off small-table scans."
    }
} elseif ($SkipPostgres) {
    Add-ScalingDecision -Area "Postgres" -Decision "not evaluated" -Reason "PostgreSQL diagnostics were skipped."
} else {
    Add-ScalingDecision -Area "Postgres" -Decision "collect diagnostics" -Reason "PostgreSQL diagnostics file was not produced."
}

if ($null -ne $maxObservedRps) {
    Add-ScalingDecision -Area "current VPS/app shape" -Decision "treat current result as mock-load baseline, not final capacity proof" -Reason ("Max observed k6 RPS was {0}; run longer sustained steps before promising production limits." -f (Format-NullableNumber $maxObservedRps 2))
} else {
    Add-ScalingDecision -Area "current VPS/app shape" -Decision "not enough traffic data" -Reason "No k6 RPS measurements were collected."
}

Add-ScalingDecision -Area "external data services" -Decision "defer migration decision until sustained diagnostics show pressure or HA/backup requirements demand it" -Reason "Postgres/Redis/S3 separation is supported by config, but this report should prove the need before changing topology."
Add-ScalingDecision -Area "provider quotas" -Decision "do not infer real provider capacity from mock tests" -Reason "AI/VK/YooKassa providers are not called in loadtest mode; quota requests require separate credential-bound smoke/load checks."
Add-ScalingDecision -Area "user limits" -Decision "keep current anti-spam/backpressure defaults until real provider quotas are known" -Reason "Mock load can validate code paths, but paid provider budgets and VK limits determine final user-facing limits."

$script:ReportLines = New-Object System.Collections.Generic.List[string]
$targetLabel = $BaseUrl
if ([string]::IsNullOrWhiteSpace($targetLabel)) {
    $targetLabel = "(not set)"
}
Add-Line "# Load Test Report"
Add-Line ""
Add-Line "- Generated: $(Get-Date -Format o)"
Add-Line "- Environment: $appEnv"
Add-Line "- Target: $targetLabel"
Add-Line "- Report dir: $ReportDir"
Add-Line "- Safety: secrets, raw prompts, launch params and provider payloads are not included."
Add-Line ""
Add-Line "## Executive Summary"
Add-Line ""
Add-Line "**First bottleneck candidate:** $firstBottleneck"
Add-Line ""
if ($findings.Count -eq 0) {
    Add-Line "- No threshold failures were detected in collected summaries."
} else {
    foreach ($finding in $findings) {
        Add-Line "- $finding"
    }
}

Add-Line ""
Add-Line "## k6 HTTP Summary"
Add-Line ""
if ($k6Rows.Count -eq 0) {
    Add-Line "No k6 summaries were collected. Pass `-RunK6` or `-K6SummaryFiles`."
} else {
    Add-TableRow @("Script", "HTTP reqs", "RPS", "p95 ms", "p99 ms", "Error rate", "Job success", "Worker throughput/s", "Job create/s")
    Add-TableRow @("---", "---:", "---:", "---:", "---:", "---:", "---:", "---:", "---:")
    foreach ($row in $k6Rows) {
        Add-TableRow @(
            $row.Script,
            (Format-NullableNumber $row.HttpRequests 0),
            (Format-NullableNumber $row.Rps 2),
            (Format-NullableNumber $row.P95Ms 2),
            (Format-NullableNumber $row.P99Ms 2),
            (Format-NullableNumber $row.ErrorRate 4),
            (Format-NullableNumber $row.JobSuccessRate 4),
            (Format-NullableNumber $row.WorkerThroughput 2),
            (Format-NullableNumber $row.JobCreateRate 2)
        )
    }
}

Add-Line ""
Add-Line "## Redis Queue And Ops"
Add-Line ""
if ($null -eq $redisSnapshot) {
    Add-Line "Redis snapshot was skipped or failed. See findings above."
} else {
    Add-Line "- Used memory: $($redisSnapshot.UsedMemoryHuman)"
    Add-Line "- Peak memory: $($redisSnapshot.UsedMemoryPeakHuman)"
    Add-Line "- Instantaneous ops/sec: $($redisSnapshot.InstantaneousOpsPerSec)"
    Add-Line "- Total commands processed: $($redisSnapshot.TotalCommandsProcessed)"
    Add-Line "- Total stream length: $($redisSnapshot.TotalStreamLength)"
    Add-Line "- Total backlog pending+lag: $($redisSnapshot.TotalQueueDepth)"
    Add-Line "- DLQ depth: $($redisSnapshot.DlqDepth)"
    Add-Line ""
    Add-TableRow @("Stream", "Length", "Pending", "Lag", "Backlog")
    Add-TableRow @("---", "---:", "---:", "---:", "---:")
    foreach ($stream in $redisSnapshot.Streams) {
        Add-TableRow @($stream.Stream, [string]$stream.Length, [string]$stream.Pending, [string]$stream.Lag, [string]$stream.Backlog)
    }
    Add-Line ""
    Add-Line ("Full Redis diagnostics: {0}" -f $redisOutputFile)
}

Add-Line ""
Add-Line "## PostgreSQL Slow Queries And Locks"
Add-Line ""
if ($SkipPostgres) {
    Add-Line "PostgreSQL diagnostics were skipped."
} elseif (Test-Path $postgresOutputFile) {
    Add-Line ("Full PostgreSQL diagnostics: {0}" -f $postgresOutputFile)
    Add-Line "Review sections: Long-running queries, Blocking locks, pg_stat_statements slow query summary, sequential scan candidates."
} else {
    Add-Line "PostgreSQL diagnostics were not collected. See findings above."
}

Add-Line ""
Add-Line "## CPU / RAM"
Add-Line ""
if ($dockerStats.Count -eq 0) {
    Add-Line "Docker stats were skipped or unavailable."
} else {
    Add-TableRow @("Container", "CPU", "Memory", "Net I/O", "Block I/O")
    Add-TableRow @("---", "---:", "---:", "---:", "---:")
    foreach ($row in $dockerStats) {
        Add-TableRow @([string]$row.Name, [string]$row.CPU, [string]$row.MemUsage, [string]$row.NetIO, [string]$row.BlockIO)
    }
}

Add-Line ""
Add-Line "## Decision Checklist"
Add-Line ""
Add-Line "- High error rate first -> inspect API handlers, auth/idempotency and provider/mock dependencies."
Add-Line "- High p95/p99 with low queue depth -> inspect API/Postgres/Redis latency."
Add-Line "- Queue depth grows while HTTP is healthy -> increase/split workers or add backpressure."
Add-Line "- DLQ grows -> inspect retry classification and terminal provider errors."
Add-Line "- Postgres long queries or locks appear -> optimize SQL/indexes before adding API replicas."
Add-Line "- Redis memory/ops spike with flat throughput -> inspect key TTLs, streams and rate-limit pressure."
Add-Line "- CPU/RAM saturation -> split services or add runtime capacity."

Add-Line ""
Add-Line "## Scaling Decisions"
Add-Line ""
Add-TableRow @("Area", "Decision", "Reason")
Add-TableRow @("---", "---", "---")
foreach ($decision in $script:ScalingDecisions) {
    Add-TableRow @($decision.area, $decision.decision, $decision.reason)
}

$reportPath = Join-Path $ReportDir "loadtest-report.md"
$script:ReportLines | Set-Content -LiteralPath $reportPath -Encoding UTF8

$scalingDecisionsArray = @()
foreach ($decision in $script:ScalingDecisions) {
    $scalingDecisionsArray += [PSCustomObject]@{
        area = [string]$decision.area
        decision = [string]$decision.decision
        reason = [string]$decision.reason
    }
}

$summaryObject = [ordered]@{}
$summaryObject["generated_at"] = (Get-Date -Format o)
$summaryObject["app_env"] = $appEnv
$summaryObject["target"] = $BaseUrl
$summaryObject["report_path"] = $reportPath
$summaryObject["first_bottleneck_candidate"] = $firstBottleneck
$summaryObject["findings"] = [object[]]$findingsArray
$summaryObject["k6"] = [object[]]$k6RowsArray
$summaryObject["redis"] = $redisSnapshot
$summaryObject["docker_stats"] = [object[]]$dockerStatsArray
$summaryObject["scaling_decisions"] = [object[]]$scalingDecisionsArray
$summaryObject | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath (Join-Path $ReportDir "loadtest-summary.json") -Encoding UTF8

Write-Host "Load-test report written to $reportPath"
if ($findings.Count -gt 0) {
    Write-Warning "Report completed with findings. Review $reportPath"
}
