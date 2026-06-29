[CmdletBinding()]
param(
    [string]$EnvFile = ".env.loadtest",
    [string]$BaseUrl = "",
    [string]$ReportDir = "",
    [int[]]$Steps = @(),
    [string]$Duration = "",
    [switch]$UseDockerCompose,
    [switch]$SkipPostgres,
    [switch]$SkipRedis,
    [switch]$SkipDockerStats,
    [switch]$ContinueOnFailure
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

function Get-Setting {
    param(
        [hashtable]$Values,
        [string]$Name,
        [string]$Default = ""
    )

    if ($Values.ContainsKey($Name) -and $Values[$Name] -ne "") {
        return $Values[$Name]
    }

    $fromProcess = [Environment]::GetEnvironmentVariable($Name)
    if (-not [string]::IsNullOrWhiteSpace($fromProcess)) {
        return $fromProcess
    }

    return $Default
}

function Split-IntList {
    param([string]$Raw)

    $result = New-Object System.Collections.Generic.List[int]
    foreach ($item in ($Raw -split ",")) {
        $trimmed = $item.Trim()
        if ($trimmed -eq "") {
            continue
        }
        $parsed = 0
        if (-not [int]::TryParse($trimmed, [ref]$parsed) -or $parsed -le 0) {
            throw "Invalid RPS ramp step: $trimmed"
        }
        $result.Add($parsed)
    }
    return [int[]]$result
}

function New-K6EnvArgs {
    param(
        [hashtable]$Values,
        [hashtable]$Overrides
    )

    $args = @()
    $reservedK6ConfigKeys = @("K6_DURATION", "K6_ITERATIONS", "K6_SCENARIOS", "K6_STAGES", "K6_VUS")
    $keys = New-Object System.Collections.Generic.HashSet[string]

    foreach ($key in $Values.Keys) {
        [void]$keys.Add([string]$key)
    }
    foreach ($key in $Overrides.Keys) {
        [void]$keys.Add([string]$key)
    }

    foreach ($key in ($keys | Sort-Object)) {
        if ($reservedK6ConfigKeys -contains $key) {
            continue
        }

        $value = $null
        if ($Overrides.ContainsKey($key)) {
            $value = $Overrides[$key]
        } elseif ($Values.ContainsKey($key)) {
            $value = $Values[$key]
        }

        if ($null -eq $value -or [string]::IsNullOrWhiteSpace([string]$value)) {
            continue
        }

        $args += @("-e", "$key=$value")
    }

    return $args
}

function Get-MetricValue {
    param(
        [object]$Metrics,
        [string]$Metric,
        [string]$Value
    )

    if ($null -eq $Metrics.$Metric -or $null -eq $Metrics.$Metric.values) {
        return $null
    }
    $property = $Metrics.$Metric.values.PSObject.Properties[$Value]
    if ($null -eq $property) {
        return $null
    }
    return $property.Value
}

function Read-K6SummaryRow {
    param(
        [string]$Step,
        [string]$SummaryFile
    )

    $json = Get-Content -LiteralPath $SummaryFile -Raw | ConvertFrom-Json
    $metrics = $json.metrics
    return [PSCustomObject]@{
        Step       = $Step
        Requests   = Get-MetricValue -Metrics $metrics -Metric "http_reqs" -Value "count"
        RPS        = Get-MetricValue -Metrics $metrics -Metric "http_reqs" -Value "rate"
        ErrorRate  = Get-MetricValue -Metrics $metrics -Metric "http_req_failed" -Value "rate"
        P95Ms      = Get-MetricValue -Metrics $metrics -Metric "http_req_duration" -Value "p(95)"
        P99Ms      = Get-MetricValue -Metrics $metrics -Metric "http_req_duration" -Value "p(99)"
        JobCreates = Get-MetricValue -Metrics $metrics -Metric "ramp_job_created_total" -Value "count"
        Summary    = $SummaryFile
    }
}

function Capture-DockerStats {
    param([string]$OutputFile)

    if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
        "docker not found" | Set-Content -LiteralPath $OutputFile -Encoding UTF8
        return
    }
    $rows = & docker stats --no-stream --format "{{json .}}" 2>&1
    $rows | Set-Content -LiteralPath $OutputFile -Encoding UTF8
}

$preflightArgs = @("-NoProfile", "-ExecutionPolicy", "Bypass", "-File", "scripts/loadtest/loadtest-preflight.ps1", "-EnvFile", $EnvFile)
& powershell @preflightArgs
if ($LASTEXITCODE -ne 0) {
    throw "Loadtest preflight failed. Fix the loadtest env before running RPS ramp."
}

if (-not (Get-Command k6 -ErrorAction SilentlyContinue)) {
    throw "k6 is not installed or not available in PATH. Install k6 before running RPS ramp."
}

$envValues = Read-EnvFile -Path $EnvFile
if ([string]::IsNullOrWhiteSpace($BaseUrl)) {
    $BaseUrl = Get-Setting -Values $envValues -Name "K6_BASE_URL" -Default ""
}
if ([string]::IsNullOrWhiteSpace($BaseUrl)) {
    throw "K6_BASE_URL is empty. Set K6_BASE_URL in $EnvFile or pass -BaseUrl."
}

if ($Steps.Count -eq 0) {
    $Steps = Split-IntList -Raw (Get-Setting -Values $envValues -Name "K6_CAPACITY_RAMP_STEPS" -Default "10,25,50,100")
}
if ([string]::IsNullOrWhiteSpace($Duration)) {
    $Duration = Get-Setting -Values $envValues -Name "K6_CAPACITY_SUSTAIN_DURATION" -Default "2m"
}
if ([string]::IsNullOrWhiteSpace($ReportDir)) {
    $stamp = Get-Date -Format "yyyyMMdd-HHmmss"
    $ReportDir = Join-Path "reports/loadtest" "rps-ramp-$stamp"
}
New-Item -ItemType Directory -Force -Path $ReportDir | Out-Null

$rows = New-Object System.Collections.Generic.List[object]
$failures = New-Object System.Collections.Generic.List[string]

foreach ($step in $Steps) {
    $stepName = "{0}rps" -f $step
    $stepDir = Join-Path $ReportDir $stepName
    New-Item -ItemType Directory -Force -Path $stepDir | Out-Null

    $summaryFile = Join-Path $stepDir "api-ramp.summary.json"
    $logFile = Join-Path $stepDir "api-ramp.k6.log"
    $preAllocated = [Math]::Max(10, $step * 2)
    $maxVUs = [Math]::Max($preAllocated, $step * 6)

    $overrides = @{
        K6_BASE_URL = $BaseUrl
        K6_RUN = "1"
        K6_ALLOW_PRODUCTION_LIVE_SMOKE = "false"
        K6_RAMP_RPS = "$step"
        K6_RAMP_DURATION = $Duration
        K6_RAMP_PREALLOCATED_VUS = "$preAllocated"
        K6_RAMP_MAX_VUS = "$maxVUs"
    }

    Write-Host "Running RPS ramp step: $stepName for $Duration"
    $k6Args = @("run") + (New-K6EnvArgs -Values $envValues -Overrides $overrides) + @("--summary-export", $summaryFile, "tests/k6/api-ramp.js")
    $output = & k6 @k6Args 2>&1
    $exitCode = $LASTEXITCODE
    $output | Set-Content -LiteralPath $logFile -Encoding UTF8

    $status = "ok"
    if ($exitCode -ne 0) {
        $status = "failed"
        $failures.Add("k6 RPS step $stepName failed with exit code $exitCode. See $logFile")
    }

    $redisDepth = $null
    $dlqDepth = $null
    $redisFile = ""
    if (-not $SkipRedis) {
        $redisFile = Join-Path $stepDir "redis-diagnostics.md"
        $redisArgs = @("-NoProfile", "-ExecutionPolicy", "Bypass", "-File", "scripts/loadtest/redis-diagnostics.ps1", "-EnvFile", $EnvFile, "-OutputFile", $redisFile)
        if ($UseDockerCompose) {
            $redisArgs += "-UseDockerCompose"
        }
        & powershell @redisArgs | Tee-Object -FilePath (Join-Path $stepDir "redis-diagnostics.log") | Out-Null
        if ($LASTEXITCODE -eq 0) {
            $redisSnapshot = [IO.Path]::ChangeExtension($redisFile, ".snapshot.json")
            if (Test-Path $redisSnapshot) {
                $redisJson = Get-Content -LiteralPath $redisSnapshot -Raw | ConvertFrom-Json
                $redisDepth = $redisJson.total_backlog
                $dlqDepth = $redisJson.dlq_depth
            }
        } else {
            $failures.Add("Redis diagnostics failed for step $stepName.")
        }
    }

    $postgresFile = ""
    if (-not $SkipPostgres) {
        $postgresFile = Join-Path $stepDir "postgres-diagnostics.md"
        $pgArgs = @("-NoProfile", "-ExecutionPolicy", "Bypass", "-File", "scripts/loadtest/postgres-diagnostics.ps1", "-EnvFile", $EnvFile, "-OutputFile", $postgresFile)
        if ($UseDockerCompose) {
            $pgArgs += "-UseDockerCompose"
        }
        & powershell @pgArgs | Tee-Object -FilePath (Join-Path $stepDir "postgres-diagnostics.log") | Out-Null
        if ($LASTEXITCODE -ne 0) {
            $failures.Add("PostgreSQL diagnostics failed for step $stepName.")
        }
    }

    $dockerStatsFile = ""
    if (-not $SkipDockerStats) {
        $dockerStatsFile = Join-Path $stepDir "docker-stats.jsonl"
        Capture-DockerStats -OutputFile $dockerStatsFile
    }

    if (Test-Path $summaryFile) {
        $summary = Read-K6SummaryRow -Step $stepName -SummaryFile $summaryFile
    } else {
        $summary = [PSCustomObject]@{
            Step       = $stepName
            Requests   = $null
            RPS        = $null
            ErrorRate  = $null
            P95Ms      = $null
            P99Ms      = $null
            JobCreates = $null
            Summary    = $summaryFile
        }
    }

    $rows.Add([PSCustomObject]@{
        Step            = $summary.Step
        Status          = $status
        Requests        = $summary.Requests
        RPS             = $summary.RPS
        ErrorRate       = $summary.ErrorRate
        P95Ms           = $summary.P95Ms
        P99Ms           = $summary.P99Ms
        JobCreates      = $summary.JobCreates
        RedisBacklog    = $redisDepth
        DLQDepth        = $dlqDepth
        SummaryFile     = $summary.Summary
        RedisFile       = $redisFile
        PostgresFile    = $postgresFile
        DockerStatsFile = $dockerStatsFile
    })

    if ($status -ne "ok" -and -not $ContinueOnFailure) {
        break
    }
}

$report = New-Object System.Collections.Generic.List[string]
$report.Add("# RPS Ramp Report")
$report.Add("")
$report.Add("- Generated at: $(Get-Date -Format o)")
$report.Add("- Env file: $EnvFile")
$report.Add("- Target: $BaseUrl")
$report.Add("- Duration per step: $Duration")
$report.Add("- Steps: $($Steps -join ', ')")
$report.Add("")
$report.Add("| Step | Status | HTTP reqs | RPS | p95 ms | p99 ms | Error rate | Jobs created | Redis backlog | DLQ |")
$report.Add("|---|---|---:|---:|---:|---:|---:|---:|---:|---:|")
foreach ($row in $rows) {
    $report.Add("| $($row.Step) | $($row.Status) | $($row.Requests) | $($row.RPS) | $($row.P95Ms) | $($row.P99Ms) | $($row.ErrorRate) | $($row.JobCreates) | $($row.RedisBacklog) | $($row.DLQDepth) |")
}
$report.Add("")
$report.Add("## Diagnostic Files")
$report.Add("")
foreach ($row in $rows) {
    $report.Add("- $($row.Step): k6=$($row.SummaryFile); redis=$($row.RedisFile); postgres=$($row.PostgresFile); docker=$($row.DockerStatsFile)")
}
if ($failures.Count -gt 0) {
    $report.Add("")
    $report.Add("## Failures")
    $report.Add("")
    foreach ($failure in $failures) {
        $report.Add("- $failure")
    }
}

$reportPath = Join-Path $ReportDir "rps-ramp-report.md"
$report | Set-Content -LiteralPath $reportPath -Encoding UTF8
$rows | ConvertTo-Json -Depth 6 | Set-Content -LiteralPath (Join-Path $ReportDir "rps-ramp-summary.json") -Encoding UTF8

Write-Host "RPS ramp report written to $reportPath"
if ($failures.Count -gt 0) {
    throw "RPS ramp completed with failures. See $reportPath"
}
