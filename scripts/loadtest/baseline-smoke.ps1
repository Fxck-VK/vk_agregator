[CmdletBinding()]
param(
    [string]$EnvFile = ".env.loadtest",
    [string]$BaseUrl = "",
    [string]$ReportDir = "",
    [switch]$UseDockerCompose,
    [switch]$SkipRedisDLQCheck
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

function Merge-Hashtable {
    param(
        [hashtable]$Base,
        [hashtable]$Overrides
    )

    $merged = @{}
    foreach ($key in $Base.Keys) {
        $merged[$key] = $Base[$key]
    }
    foreach ($key in $Overrides.Keys) {
        $merged[$key] = $Overrides[$key]
    }

    return $merged
}

function Invoke-K6Scenario {
    param(
        [string]$Name,
        [string]$ScriptPath,
        [hashtable]$Values,
        [hashtable]$Overrides,
        [string]$OutputDir
    )

    if (-not (Get-Command k6 -ErrorAction SilentlyContinue)) {
        throw "k6 is not installed or not available in PATH. Install k6 before running baseline smoke."
    }
    if (-not (Test-Path $ScriptPath)) {
        throw "k6 script not found: $ScriptPath"
    }

    $summaryFile = Join-Path $OutputDir "$Name.summary.json"
    $logFile = Join-Path $OutputDir "$Name.k6.log"
    $k6EnvArgs = New-K6EnvArgs -Values $Values -Overrides $Overrides
    $args = @("run") + $k6EnvArgs + @("--summary-export", $summaryFile, $ScriptPath)

    Write-Host "Running baseline smoke scenario: $Name"
    $output = & k6 @args 2>&1
    $exitCode = $LASTEXITCODE
    $output | Set-Content -LiteralPath $logFile -Encoding UTF8

    if ($exitCode -ne 0) {
        throw "k6 scenario '$Name' failed with exit code $exitCode. See $logFile"
    }
    if (-not (Test-Path $summaryFile)) {
        throw "k6 scenario '$Name' did not write summary file $summaryFile"
    }

    return [PSCustomObject]@{
        Name    = $Name
        Summary = $summaryFile
        Log     = $logFile
    }
}

$preflightArgs = @("-NoProfile", "-ExecutionPolicy", "Bypass", "-File", "scripts/loadtest/loadtest-preflight.ps1", "-EnvFile", $EnvFile)
& powershell @preflightArgs
if ($LASTEXITCODE -ne 0) {
    throw "Loadtest preflight failed. Fix the loadtest env before running baseline smoke."
}

$envValues = Read-EnvFile -Path $EnvFile
if ([string]::IsNullOrWhiteSpace($BaseUrl)) {
    $BaseUrl = Get-Setting -Values $envValues -Name "K6_BASE_URL" -Default ""
}
if ([string]::IsNullOrWhiteSpace($BaseUrl)) {
    throw "K6_BASE_URL is empty. Set K6_BASE_URL in $EnvFile or pass -BaseUrl."
}

if ([string]::IsNullOrWhiteSpace($ReportDir)) {
    $stamp = Get-Date -Format "yyyyMMdd-HHmmss"
    $ReportDir = Join-Path "reports/loadtest" "baseline-$stamp"
}
New-Item -ItemType Directory -Force -Path $ReportDir | Out-Null

$duration = Get-Setting -Values $envValues -Name "K6_BASELINE_SMOKE_DURATION" -Default "10s"
$summaries = New-Object System.Collections.Generic.List[object]

$commonOverrides = @{
    K6_BASE_URL = $BaseUrl
    K6_RUN = "1"
    K6_ALLOW_PRODUCTION_LIVE_SMOKE = "false"
    K6_SLEEP_SECONDS = "0"
}

$summaries.Add((Invoke-K6Scenario -Name "basic-api-smoke" -ScriptPath "tests/k6/basic-api.js" -Values $envValues -Overrides (Merge-Hashtable -Base $commonOverrides -Overrides @{
    K6_BASIC_DURATION = $duration
    K6_HEALTH_VUS = "1"
    K6_VK_VUS = "1"
    K6_MINIAPP_VUS = "1"
}) -OutputDir $ReportDir))

$summaries.Add((Invoke-K6Scenario -Name "vk-bot-smoke" -ScriptPath "tests/k6/vk-bot.js" -Values $envValues -Overrides (Merge-Hashtable -Base $commonOverrides -Overrides @{
    K6_VK_BOT_VUS = "1"
    K6_VK_BOT_ITERATIONS = "1"
    K6_VK_BOT_MAX_DURATION = "1m"
    K6_VK_RATE_LIMIT_VUS = "1"
    K6_VK_RATE_LIMIT_ITERATIONS = "1"
    K6_VK_RATE_LIMIT_EVENTS = "5"
    K6_VK_RATE_LIMIT_MAX_DURATION = "1m"
    K6_VK_STEP_SLEEP_SECONDS = "0"
}) -OutputDir $ReportDir))

foreach ($workload in @("text", "image", "video")) {
    $rateKey = "K6_JOB_{0}_RATE" -f $workload.ToUpperInvariant()
    $summaries.Add((Invoke-K6Scenario -Name "job-$workload-smoke" -ScriptPath "tests/k6/job-worker.js" -Values $envValues -Overrides (Merge-Hashtable -Base $commonOverrides -Overrides @{
        K6_JOB_WORKLOAD = $workload
        K6_JOB_DURATION = $duration
        $rateKey = "1"
        K6_JOB_POLL = "true"
        K6_JOB_POLL_ATTEMPTS = "10"
        K6_JOB_POLL_INTERVAL_SECONDS = "0.5"
        K6_JOB_SLEEP_SECONDS = "0"
    }) -OutputDir $ReportDir))
}

$summaries.Add((Invoke-K6Scenario -Name "billing-mock-smoke" -ScriptPath "tests/k6/billing-payments.js" -Values $envValues -Overrides (Merge-Hashtable -Base $commonOverrides -Overrides @{
    K6_PAYMENT_DURATION = $duration
    K6_PAYMENT_RATE = "1"
    K6_PAYMENT_PREALLOCATED_VUS = "2"
    K6_PAYMENT_MAX_VUS = "10"
    K6_PAYMENT_SLEEP_SECONDS = "0"
}) -OutputDir $ReportDir))

$dlqDepth = $null
if (-not $SkipRedisDLQCheck) {
    $redisOutputFile = Join-Path $ReportDir "redis-baseline-smoke.md"
    $redisArgs = @("-NoProfile", "-ExecutionPolicy", "Bypass", "-File", "scripts/loadtest/redis-diagnostics.ps1", "-EnvFile", $EnvFile, "-OutputFile", $redisOutputFile)
    if ($UseDockerCompose) {
        $redisArgs += "-UseDockerCompose"
    }
    & powershell @redisArgs | Tee-Object -FilePath (Join-Path $ReportDir "redis-baseline-smoke.log") | Out-Null
    if ($LASTEXITCODE -ne 0) {
        throw "Redis diagnostics failed after baseline smoke."
    }

    $redisSnapshot = [IO.Path]::ChangeExtension($redisOutputFile, ".snapshot.json")
    if (-not (Test-Path $redisSnapshot)) {
        throw "Redis diagnostics did not write snapshot file $redisSnapshot"
    }

    $snapshot = Get-Content -LiteralPath $redisSnapshot -Raw | ConvertFrom-Json
    $dlqDepth = [int]$snapshot.dlq_depth
    if ($dlqDepth -ne 0) {
        throw "Redis DLQ is not empty after baseline smoke: $dlqDepth"
    }
}

$summaryLines = New-Object System.Collections.Generic.List[string]
$summaryLines.Add("# Baseline Smoke")
$summaryLines.Add("")
$summaryLines.Add("- Generated at: $(Get-Date -Format o)")
$summaryLines.Add("- Env file: $EnvFile")
$summaryLines.Add("- Target: $BaseUrl")
$summaryLines.Add("- Duration per scenario: $duration")
if ($null -ne $dlqDepth) {
    $summaryLines.Add("- Redis DLQ depth: $dlqDepth")
}
$summaryLines.Add("")
$summaryLines.Add("| Scenario | Summary | Log |")
$summaryLines.Add("|---|---|---|")
foreach ($row in $summaries) {
    $summaryLines.Add("| $($row.Name) | $($row.Summary) | $($row.Log) |")
}
$summaryPath = Join-Path $ReportDir "baseline-smoke.md"
$summaryLines | Set-Content -LiteralPath $summaryPath -Encoding UTF8

Write-Host "Baseline smoke completed successfully. Report: $summaryPath"
