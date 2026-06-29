[CmdletBinding()]
param(
    [string]$EnvFile = ".env.loadtest",
    [string]$ReportDir = "",
    [string]$BaseUrl = "",
    [string]$ArtifactsRoot = "",
    [string]$RpsRampSummary = "",
    [string]$WorkerCapacitySummary = "",
    [string]$BillingLoadSummary = "",
    [string[]]$K6Scripts = @(),
    [string[]]$K6SummaryFiles = @(),
    [switch]$RunK6,
    [switch]$SkipPostgres,
    [switch]$SkipRedis,
    [switch]$SkipDockerStats,
    [switch]$UseDockerCompose,
    [switch]$AllowProduction,
    [switch]$SkipPreflight
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

if (-not $SkipPreflight) {
    $preflightArgs = @(
        "-NoProfile",
        "-ExecutionPolicy",
        "Bypass",
        "-File",
        "scripts/loadtest/loadtest-preflight.ps1",
        "-EnvFile",
        $EnvFile
    )
    & powershell @preflightArgs
    if ($LASTEXITCODE -ne 0) {
        throw "Loadtest preflight failed. Fix the loadtest env before running capacity reports."
    }
}

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

function Get-ObjectProperty {
    param(
        [object]$Object,
        [string]$Name,
        [object]$Default = $null
    )

    if ($null -eq $Object) {
        return $Default
    }

    $prop = $Object.PSObject.Properties[$Name]
    if ($null -eq $prop) {
        return $Default
    }

    return $prop.Value
}

function Get-DoubleProperty {
    param(
        [object]$Object,
        [string]$Name,
        [object]$Default = $null
    )

    $value = Get-ObjectProperty -Object $Object -Name $Name -Default $Default
    if ($null -eq $value -or $value -eq "") {
        return $Default
    }

    try {
        return [double]$value
    } catch {
        return $Default
    }
}

function Read-JsonFile {
    param([string]$Path)

    if ([string]::IsNullOrWhiteSpace($Path) -or -not (Test-Path $Path)) {
        return $null
    }

    return Get-Content -LiteralPath $Path -Raw | ConvertFrom-Json
}

function Find-LatestArtifactFile {
    param(
        [string]$Filter,
        [string[]]$Roots
    )

    $candidates = @()
    foreach ($root in ($Roots | Where-Object { -not [string]::IsNullOrWhiteSpace($_) } | Select-Object -Unique)) {
        if (-not (Test-Path $root)) {
            continue
        }

        $candidates += Get-ChildItem -LiteralPath $root -Recurse -File -Filter $Filter -ErrorAction SilentlyContinue
    }

    if ($candidates.Count -eq 0) {
        return ""
    }

    return ($candidates | Sort-Object LastWriteTimeUtc -Descending | Select-Object -First 1).FullName
}

function Resolve-RunnerSummaryFile {
    param(
        [string]$ExplicitPath,
        [string]$Filter,
        [string[]]$Roots
    )

    if (-not [string]::IsNullOrWhiteSpace($ExplicitPath)) {
        if (Test-Path $ExplicitPath) {
            return (Resolve-Path $ExplicitPath).Path
        }
        return $ExplicitPath
    }

    return Find-LatestArtifactFile -Filter $Filter -Roots $Roots
}

function Convert-PercentString {
    param([string]$Value)

    if ([string]::IsNullOrWhiteSpace($Value)) {
        return $null
    }

    $raw = $Value.Trim().TrimEnd("%").Replace(",", ".")
    try {
        return [double]$raw
    } catch {
        return $null
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
    "tests/k6/billing-payments.js",
    "tests/k6/admin-actions.js"
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
$maxCpuPercent = [double](Get-Setting -Name "LOADTEST_REPORT_MAX_CPU_PERCENT" -Default "85")

$artifactRoots = @()
if (-not [string]::IsNullOrWhiteSpace($ArtifactsRoot)) {
    $artifactRoots += $ArtifactsRoot
}
$artifactRoots += @("reports/loadtest", "artifacts/loadtest")

$rpsRampSummaryPath = Resolve-RunnerSummaryFile -ExplicitPath $RpsRampSummary -Filter "rps-ramp-summary.json" -Roots $artifactRoots
$workerCapacitySummaryPath = Resolve-RunnerSummaryFile -ExplicitPath $WorkerCapacitySummary -Filter "worker-capacity-summary.json" -Roots $artifactRoots
$billingLoadSummaryPath = Resolve-RunnerSummaryFile -ExplicitPath $BillingLoadSummary -Filter "billing-load-summary.json" -Roots $artifactRoots

$rpsRampRows = @()
$workerCapacityData = $null
$workerWorkloads = @()
$billingLoadData = $null

if (-not [string]::IsNullOrWhiteSpace($rpsRampSummaryPath)) {
    if (Test-Path $rpsRampSummaryPath) {
        $rpsRampData = Read-JsonFile -Path $rpsRampSummaryPath
        $rpsRampRows = @($rpsRampData)
    } elseif (-not [string]::IsNullOrWhiteSpace($RpsRampSummary)) {
        $failures.Add("RPS ramp summary file not found: $rpsRampSummaryPath")
    }
}

if (-not [string]::IsNullOrWhiteSpace($workerCapacitySummaryPath)) {
    if (Test-Path $workerCapacitySummaryPath) {
        $workerCapacityData = Read-JsonFile -Path $workerCapacitySummaryPath
        $workerWorkloads = @(Get-ObjectProperty -Object $workerCapacityData -Name "workloads" -Default @())
    } elseif (-not [string]::IsNullOrWhiteSpace($WorkerCapacitySummary)) {
        $failures.Add("Worker capacity summary file not found: $workerCapacitySummaryPath")
    }
}

if (-not [string]::IsNullOrWhiteSpace($billingLoadSummaryPath)) {
    if (Test-Path $billingLoadSummaryPath) {
        $billingLoadData = Read-JsonFile -Path $billingLoadSummaryPath
    } elseif (-not [string]::IsNullOrWhiteSpace($BillingLoadSummary)) {
        $failures.Add("Billing load summary file not found: $billingLoadSummaryPath")
    }
}

$maxStableRps = $null
$maxStableRpsStep = ""
$maxRampRps = $null
$maxRampP95Ms = $null
$maxRampP99Ms = $null
$maxRampErrorRate = $null
$rpsFailures = New-Object System.Collections.Generic.List[string]

foreach ($row in $rpsRampRows) {
    $rowStatus = [string](Get-ObjectProperty -Object $row -Name "Status" -Default "")
    $rowRps = Get-DoubleProperty -Object $row -Name "RPS" -Default $null
    $rowStep = [string](Get-ObjectProperty -Object $row -Name "Step" -Default "")
    $rowP95 = Get-DoubleProperty -Object $row -Name "P95Ms" -Default $null
    $rowP99 = Get-DoubleProperty -Object $row -Name "P99Ms" -Default $null
    $rowErrorRate = Get-DoubleProperty -Object $row -Name "ErrorRate" -Default $null
    $rowRedisBacklog = Get-DoubleProperty -Object $row -Name "RedisBacklog" -Default 0
    $rowDlq = Get-DoubleProperty -Object $row -Name "DLQDepth" -Default 0

    if ($null -ne $rowRps -and ($null -eq $maxRampRps -or $rowRps -gt $maxRampRps)) {
        $maxRampRps = $rowRps
    }
    if ($null -ne $rowP95 -and ($null -eq $maxRampP95Ms -or $rowP95 -gt $maxRampP95Ms)) {
        $maxRampP95Ms = $rowP95
    }
    if ($null -ne $rowP99 -and ($null -eq $maxRampP99Ms -or $rowP99 -gt $maxRampP99Ms)) {
        $maxRampP99Ms = $rowP99
    }
    if ($null -ne $rowErrorRate -and ($null -eq $maxRampErrorRate -or $rowErrorRate -gt $maxRampErrorRate)) {
        $maxRampErrorRate = $rowErrorRate
    }

    $stable = ($rowStatus -eq "ok")
    if ($null -ne $rowErrorRate -and $rowErrorRate -gt $maxErrorRate) {
        $stable = $false
        $rpsFailures.Add("RPS step $rowStep exceeded HTTP error threshold: $(Format-NullableNumber $rowErrorRate 4).")
    }
    if ($null -ne $rowP95 -and $rowP95 -gt $maxP95Ms) {
        $stable = $false
        $rpsFailures.Add("RPS step $rowStep exceeded p95 threshold: $(Format-NullableNumber $rowP95 2) ms.")
    }
    if ($rowRedisBacklog -gt $maxQueueDepth) {
        $stable = $false
        $rpsFailures.Add("RPS step $rowStep exceeded Redis backlog threshold: $rowRedisBacklog.")
    }
    if ($rowDlq -gt $maxDlqDepth) {
        $stable = $false
        $rpsFailures.Add("RPS step $rowStep exceeded DLQ threshold: $rowDlq.")
    }

    if ($stable -and $null -ne $rowRps -and ($null -eq $maxStableRps -or $rowRps -gt $maxStableRps)) {
        $maxStableRps = $rowRps
        $maxStableRpsStep = $rowStep
    }
}

$primaryWorkerRow = $null
foreach ($row in $workerWorkloads) {
    if ([string](Get-ObjectProperty -Object $row -Name "workload" -Default "") -eq "mixed") {
        $primaryWorkerRow = $row
        break
    }
}
if ($null -eq $primaryWorkerRow) {
    foreach ($row in $workerWorkloads) {
        $throughput = Get-DoubleProperty -Object $row -Name "worker_throughput_per_sec" -Default 0
        if ($null -eq $primaryWorkerRow -or $throughput -gt (Get-DoubleProperty -Object $primaryWorkerRow -Name "worker_throughput_per_sec" -Default 0)) {
            $primaryWorkerRow = $row
        }
    }
}

$jobsAcceptedPerSec = Get-DoubleProperty -Object $primaryWorkerRow -Name "jobs_created_per_sec" -Default $null
$jobsCompletedPerSec = Get-DoubleProperty -Object $primaryWorkerRow -Name "worker_throughput_per_sec" -Default $null
$workerDlq = Get-DoubleProperty -Object $primaryWorkerRow -Name "redis_dlq" -Default $null
$workerBacklog = Get-DoubleProperty -Object $primaryWorkerRow -Name "redis_backlog" -Default $null
$workerWorkloadName = [string](Get-ObjectProperty -Object $primaryWorkerRow -Name "workload" -Default "")

$billingMetrics = Get-ObjectProperty -Object $billingLoadData -Name "metrics" -Default $null
$billingFailures = @(Get-ObjectProperty -Object $billingLoadData -Name "failures" -Default @())

$maxCpuObserved = $null
foreach ($row in $dockerStats) {
    $cpu = Convert-PercentString -Value ([string]$row.CPU)
    if ($null -ne $cpu -and ($null -eq $maxCpuObserved -or $cpu -gt $maxCpuObserved)) {
        $maxCpuObserved = $cpu
    }
}

$findings = New-Object System.Collections.Generic.List[string]
foreach ($failure in $rpsFailures) {
    $findings.Add($failure)
}
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
foreach ($failure in @(Get-ObjectProperty -Object $workerCapacityData -Name "failures" -Default @())) {
    if (-not [string]::IsNullOrWhiteSpace([string]$failure)) {
        $findings.Add("Worker capacity finding: $failure")
    }
}
foreach ($failure in $billingFailures) {
    if (-not [string]::IsNullOrWhiteSpace([string]$failure)) {
        $findings.Add("Billing load finding: $failure")
    }
}
if ($null -ne $maxCpuObserved -and $maxCpuObserved -gt $maxCpuPercent) {
    $findings.Add("CPU usage exceeded threshold: $(Format-NullableNumber $maxCpuObserved 2)%.")
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

$postgresBottleneck = "not evaluated"
if (-not $SkipPostgres -and (Test-Path $postgresOutputFile)) {
    $postgresDiagnosticsTextForSummary = Get-Content -LiteralPath $postgresOutputFile -Raw
    if ($postgresDiagnosticsTextForSummary -match "(?s)== Blocking locks ==.*?\(0 rows\)" -and $postgresDiagnosticsTextForSummary -match "(?s)== Long-running queries ==.*?\(0 rows\)") {
        $postgresBottleneck = "no blocking locks or long-running queries in collected diagnostics"
    } else {
        $postgresBottleneck = "inspect diagnostics for locks/long queries before scaling API"
    }
} elseif ($SkipPostgres) {
    $postgresBottleneck = "skipped"
}

$redisBottleneck = "not evaluated"
if ($null -ne $redisSnapshot) {
    if ($redisSnapshot.DlqDepth -gt $maxDlqDepth) {
        $redisBottleneck = "DLQ depth above threshold"
    } elseif ($redisSnapshot.TotalQueueDepth -gt $maxQueueDepth) {
        $redisBottleneck = "queue depth above threshold"
    } else {
        $redisBottleneck = "no queue/DLQ bottleneck in collected snapshot"
    }
} elseif ($null -ne $workerBacklog -or $null -ne $workerDlq) {
    if ($workerDlq -gt $maxDlqDepth) {
        $redisBottleneck = "worker run observed DLQ entries"
    } elseif ($workerBacklog -gt $maxQueueDepth) {
        $redisBottleneck = "worker run observed Redis backlog"
    } else {
        $redisBottleneck = "worker run did not show Redis backlog/DLQ"
    }
}

$cpuRamLimit = "not evaluated"
if ($dockerStats.Count -gt 0) {
    if ($null -ne $maxCpuObserved -and $maxCpuObserved -gt $maxCpuPercent) {
        $cpuRamLimit = "CPU above threshold: $(Format-NullableNumber $maxCpuObserved 2)%"
    } elseif ($null -ne $maxCpuObserved) {
        $cpuRamLimit = "CPU below threshold in snapshot: $(Format-NullableNumber $maxCpuObserved 2)%"
    } else {
        $cpuRamLimit = "docker stats captured; inspect memory column"
    }
}

$capacitySummary = [ordered]@{
    max_stable_rps = $maxStableRps
    max_stable_rps_step = $maxStableRpsStep
    max_observed_rps = $maxRampRps
    max_observed_p95_ms = $maxRampP95Ms
    max_observed_p99_ms = $maxRampP99Ms
    max_observed_error_rate = $maxRampErrorRate
    jobs_per_sec_accepted = $jobsAcceptedPerSec
    jobs_per_sec_completed = $jobsCompletedPerSec
    worker_reference_workload = $workerWorkloadName
    redis_bottleneck = $redisBottleneck
    postgres_bottleneck = $postgresBottleneck
    cpu_ram_limit = $cpuRamLimit
    dlq_count = if ($null -ne $redisSnapshot) { $redisSnapshot.DlqDepth } elseif ($null -ne $workerDlq) { $workerDlq } else { $null }
    retry_count = $null
    first_bottleneck = $firstBottleneck
    sources = [ordered]@{
        rps_ramp_summary = $rpsRampSummaryPath
        worker_capacity_summary = $workerCapacitySummaryPath
        billing_load_summary = $billingLoadSummaryPath
    }
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

if ($null -ne $maxStableRps) {
    Add-ScalingDecision -Area "rate limits" -Decision ("cap public traffic below measured stable RPS {0} until next run" -f (Format-NullableNumber $maxStableRps 2)) -Reason "Use the highest stable RPS step as the upper bound for initial rollout, then keep user-facing limits below it with safety margin."
} else {
    Add-ScalingDecision -Area "rate limits" -Decision "do not raise public limits yet" -Reason "No stable RPS step was proven by the ramp artifacts."
}

if ($null -ne $jobsAcceptedPerSec -and $null -ne $jobsCompletedPerSec) {
    if ($jobsCompletedPerSec -lt ($jobsAcceptedPerSec * 0.8)) {
        Add-ScalingDecision -Area "worker count" -Decision "increase worker count or reduce accepted job rate" -Reason ("Reference workload {0}: accepted {1}/s, completed {2}/s." -f $workerWorkloadName, (Format-NullableNumber $jobsAcceptedPerSec 2), (Format-NullableNumber $jobsCompletedPerSec 2))
    } else {
        Add-ScalingDecision -Area "worker count" -Decision "current worker count is acceptable for measured mock workload" -Reason ("Reference workload {0}: accepted {1}/s, completed {2}/s." -f $workerWorkloadName, (Format-NullableNumber $jobsAcceptedPerSec 2), (Format-NullableNumber $jobsCompletedPerSec 2))
    }
} else {
    Add-ScalingDecision -Area "worker count" -Decision "run worker-capacity.ps1 before sizing workers" -Reason "No worker capacity summary was found."
}

if ($postgresBottleneck -like "inspect*") {
    Add-ScalingDecision -Area "indexes" -Decision "do not add broad indexes blindly; inspect slow SQL first" -Reason $postgresBottleneck
} else {
    Add-ScalingDecision -Area "indexes" -Decision "no index change from this report alone" -Reason $postgresBottleneck
}

if ($redisBottleneck -like "*above threshold*" -or $redisBottleneck -like "*backlog*") {
    Add-ScalingDecision -Area "separate queues" -Decision "consider separate text/image/video worker pools before scaling traffic" -Reason $redisBottleneck
} else {
    Add-ScalingDecision -Area "separate queues" -Decision "keep queue topology for now" -Reason $redisBottleneck
}

if ($cpuRamLimit -like "*above threshold*") {
    Add-ScalingDecision -Area "minimum VPS" -Decision "current VPS is below target for this load step" -Reason $cpuRamLimit
} elseif ($dockerStats.Count -gt 0) {
    Add-ScalingDecision -Area "minimum VPS" -Decision "current VPS can be used for the measured mock baseline only" -Reason $cpuRamLimit
} else {
    Add-ScalingDecision -Area "minimum VPS" -Decision "not enough runtime data" -Reason "Docker CPU/RAM snapshot was not collected."
}

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
Add-Line "## Capacity Summary"
Add-Line ""
Add-TableRow @("Metric", "Value")
Add-TableRow @("---", "---:")
Add-TableRow @("Maximum stable RPS", (Format-NullableNumber $maxStableRps 2))
Add-TableRow @("Stable RPS step", $(if ([string]::IsNullOrWhiteSpace($maxStableRpsStep)) { "-" } else { $maxStableRpsStep }))
Add-TableRow @("Maximum observed RPS", (Format-NullableNumber $maxRampRps 2))
Add-TableRow @("Worst observed p95 ms", (Format-NullableNumber $maxRampP95Ms 2))
Add-TableRow @("Worst observed p99 ms", (Format-NullableNumber $maxRampP99Ms 2))
Add-TableRow @("Worst observed error rate", (Format-NullableNumber $maxRampErrorRate 4))
Add-TableRow @("Jobs/sec accepted", (Format-NullableNumber $jobsAcceptedPerSec 2))
Add-TableRow @("Jobs/sec completed", (Format-NullableNumber $jobsCompletedPerSec 2))
Add-TableRow @("Worker reference workload", $(if ([string]::IsNullOrWhiteSpace($workerWorkloadName)) { "-" } else { $workerWorkloadName }))
Add-TableRow @("Redis bottleneck", $redisBottleneck)
Add-TableRow @("Postgres bottleneck", $postgresBottleneck)
Add-TableRow @("CPU/RAM limit", $cpuRamLimit)
Add-TableRow @("DLQ count", $(if ($null -ne $capacitySummary["dlq_count"]) { [string]$capacitySummary["dlq_count"] } else { "-" }))
Add-TableRow @("Retry count", "not collected")
Add-Line ""
Add-Line "Source summaries:"
Add-Line ""
Add-Line "- RPS ramp: $(if ([string]::IsNullOrWhiteSpace($rpsRampSummaryPath)) { 'not found' } else { $rpsRampSummaryPath })"
Add-Line "- Worker capacity: $(if ([string]::IsNullOrWhiteSpace($workerCapacitySummaryPath)) { 'not found' } else { $workerCapacitySummaryPath })"
Add-Line "- Billing load: $(if ([string]::IsNullOrWhiteSpace($billingLoadSummaryPath)) { 'not found' } else { $billingLoadSummaryPath })"

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

$decisionPath = Join-Path $ReportDir "loadtest-decisions.md"
$decisionLines = New-Object System.Collections.Generic.List[string]
$decisionLines.Add("# Load Test Decisions")
$decisionLines.Add("")
$decisionLines.Add("- Generated: $(Get-Date -Format o)")
$decisionLines.Add("- Report: $reportPath")
$decisionLines.Add("- Environment: $appEnv")
$decisionLines.Add("- Safety: decisions are based on mock/loadtest artifacts only; no real provider capacity is inferred.")
$decisionLines.Add("")
$decisionLines.Add("## Capacity Result")
$decisionLines.Add("")
$decisionLines.Add("| Metric | Value |")
$decisionLines.Add("|---|---:|")
$decisionLines.Add("| Maximum stable RPS | $(Format-NullableNumber $maxStableRps 2) |")
$decisionLines.Add("| Jobs/sec accepted | $(Format-NullableNumber $jobsAcceptedPerSec 2) |")
$decisionLines.Add("| Jobs/sec completed | $(Format-NullableNumber $jobsCompletedPerSec 2) |")
$decisionLines.Add("| First bottleneck | $firstBottleneck |")
$decisionLines.Add("| Redis bottleneck | $redisBottleneck |")
$decisionLines.Add("| Postgres bottleneck | $postgresBottleneck |")
$decisionLines.Add("| CPU/RAM limit | $cpuRamLimit |")
$decisionLines.Add("")
$decisionLines.Add("## Decisions")
$decisionLines.Add("")
$decisionLines.Add("| Area | Decision | Reason |")
$decisionLines.Add("|---|---|---|")
foreach ($decision in $script:ScalingDecisions) {
    $area = ([string]$decision.area) -replace "\|", "\\|"
    $decisionText = ([string]$decision.decision) -replace "\|", "\\|"
    $reason = ([string]$decision.reason) -replace "\|", "\\|"
    $decisionLines.Add("| $area | $decisionText | $reason |")
}
$decisionLines.Add("")
$decisionLines.Add("## Required Follow-Up")
$decisionLines.Add("")
$decisionLines.Add("- Rerun with longer sustained steps before promising production limits.")
$decisionLines.Add("- Do not use this mock report to infer real AI, VK delivery or YooKassa provider quotas.")
$decisionLines.Add("- If a bottleneck appears, fix that component and rerun the same scenario before changing unrelated infrastructure.")
$decisionLines | Set-Content -LiteralPath $decisionPath -Encoding UTF8

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
$summaryObject["decision_path"] = $decisionPath
$summaryObject["first_bottleneck_candidate"] = $firstBottleneck
$summaryObject["capacity"] = $capacitySummary
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
