[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string]$ReleaseBundleDir,
    [Parameter(Mandatory = $true)][string]$WorkflowIdentity,
    [string]$EnvFile = ".env",
    [string]$ProjectName = "vk-ai-aggregator-prod",
    [string]$CosignPath = "cosign",
    [string]$ReleaseManifestPath = "",
    [switch]$WithCloudflare,
    [switch]$SkipBackup,
    [switch]$NoHealthCheck,
    [switch]$DryRun,
    [int]$TimeoutSeconds = 180
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..")
Set-Location $repoRoot
$rollbackStartedAt = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
$backupStatus = "skipped"
$healthStatus = "skipped"
$script:ReleaseEnvironmentNames = @(
    "API_IMAGE",
    "WORKER_IMAGE",
    "PROVIDER_WEBHOOK_IMAGE",
    "PROVIDER_BALANCE_BOT_IMAGE",
    "MINIAPP_IMAGE",
    "MIGRATE_IMAGE",
    "BACKUP_IMAGE",
    "RELEASE_COMMIT_SHA",
    "RELEASE_MANIFEST_SHA256",
    "RELEASE_WORKFLOW_IDENTITY"
)

function Invoke-Step {
    param(
        [Parameter(Mandatory = $true)][string]$Name,
        [Parameter(Mandatory = $true)][scriptblock]$Command
    )

    Write-Host "==> $Name"
    & $Command
    if ($LASTEXITCODE -ne 0) {
        throw "$Name failed with exit code $LASTEXITCODE"
    }
}

function Get-EnvFileValue {
    param(
        [Parameter(Mandatory = $true)][string]$Path,
        [Parameter(Mandatory = $true)][string]$Name,
        [string]$Default = ""
    )

    if (-not (Test-Path -LiteralPath $Path)) {
        return $Default
    }

    foreach ($line in Get-Content -LiteralPath $Path) {
        $trimmed = $line.Trim()
        if ($trimmed.Length -eq 0 -or $trimmed.StartsWith("#")) {
            continue
        }
        if ($trimmed.StartsWith("$Name=")) {
            $value = $trimmed.Substring($Name.Length + 1).Trim()
            return $value.Trim('"').Trim("'")
        }
    }
    return $Default
}

function Test-EnvPlaceholderValue {
    param([string]$Value)

    if ([string]::IsNullOrWhiteSpace($Value)) {
        return $true
    }
    $lower = $Value.ToLowerInvariant()
    return $lower.Contains("change_me") -or $lower.Contains("placeholder") -or $lower.Contains("example")
}

function Test-TrueValue {
    param([string]$Value)

    if ([string]::IsNullOrWhiteSpace($Value)) {
        return $false
    }
    return @("1", "true", "yes", "on") -contains $Value.Trim().ToLowerInvariant()
}

function Normalize-DataServiceMode {
    param([string]$Value)

    if ([string]::IsNullOrWhiteSpace($Value)) {
        return "local"
    }
    return $Value.Trim().ToLowerInvariant()
}

function Get-DataServiceMode {
    param(
        [Parameter(Mandatory = $true)][string]$Path,
        [Parameter(Mandatory = $true)][string]$Name
    )

    $defaultMode = Normalize-DataServiceMode -Value (Get-EnvFileValue -Path $Path -Name "DATA_SERVICES_MODE" -Default "local")
    return Normalize-DataServiceMode -Value (Get-EnvFileValue -Path $Path -Name $Name -Default $defaultMode)
}

function Get-LocalStatefulServices {
    param([Parameter(Mandatory = $true)][string]$Path)

    $services = @()
    if ((Get-DataServiceMode -Path $Path -Name "POSTGRES_MODE") -eq "local") {
        $services += "postgres"
    }
    if ((Get-DataServiceMode -Path $Path -Name "REDIS_MODE") -eq "local") {
        $services += "redis"
    }
    if ((Get-DataServiceMode -Path $Path -Name "S3_MODE") -eq "local") {
        $services += "minio"
    }
    return $services
}

function Test-DockerRuntime {
    if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
        throw "Docker CLI is not installed or not in PATH"
    }
    & docker version | Out-Null
    if ($LASTEXITCODE -ne 0) {
        throw "docker version failed with exit code $LASTEXITCODE"
    }
    & docker compose version | Out-Null
    if ($LASTEXITCODE -ne 0) {
        throw "docker compose version failed with exit code $LASTEXITCODE"
    }
    & docker info | Out-Null
    if ($LASTEXITCODE -ne 0) {
        throw "docker info failed with exit code $LASTEXITCODE"
    }
    $dockerVersion = (& docker version --format '{{.Server.Version}}').Trim()
    $composeVersion = (& docker compose version --short 2>$null)
    if ([string]::IsNullOrWhiteSpace($composeVersion)) {
        $composeVersion = (& docker compose version).Trim()
    }
    Write-Host "Docker OK: $dockerVersion"
    Write-Host "Docker Compose OK: $composeVersion"
}

function Wait-Http {
    param(
        [Parameter(Mandatory = $true)][string]$Name,
        [Parameter(Mandatory = $true)][string]$Url
    )

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    $lastError = $null
    while ((Get-Date) -lt $deadline) {
        try {
            $response = Invoke-WebRequest -UseBasicParsing -Uri $Url -TimeoutSec 5
            if ($response.StatusCode -ge 200 -and $response.StatusCode -lt 300) {
                Write-Host "$Name OK: $Url"
                return
            }
            $lastError = "HTTP $($response.StatusCode)"
        } catch {
            $lastError = $_.Exception.Message
        }
        Start-Sleep -Seconds 2
    }
    throw "$Name health check timed out at $Url ($lastError)"
}

if (-not (Test-Path -LiteralPath $ReleaseBundleDir -PathType Container)) {
    throw "ReleaseBundleDir is required. Use the previous signed release bundle."
}
if (-not (Test-Path -LiteralPath "docker-compose.prod.yml")) {
    throw "docker-compose.prod.yml not found"
}
if (-not (Test-Path -LiteralPath $EnvFile)) {
    throw "Production env file not found: $EnvFile"
}

function Save-ReleaseProcessEnvironment {
    $saved = @{}
    foreach ($name in $script:ReleaseEnvironmentNames) {
        $saved[$name] = [pscustomobject]@{
            Exists = Test-Path -LiteralPath "Env:$name"
            Value = [Environment]::GetEnvironmentVariable($name, [EnvironmentVariableTarget]::Process)
        }
    }
    return $saved
}

function Import-VerifiedReleaseEnvironment {
    param([Parameter(Mandatory = $true)][string]$Path)

    $values = @{}
    foreach ($line in Get-Content -LiteralPath $Path) {
        if ($line -notmatch '^(?<key>[A-Z][A-Z0-9_]*)=(?<value>.+)$') {
            throw "Verifier-generated release environment contains an invalid line."
        }
        if ($script:ReleaseEnvironmentNames -notcontains $Matches.key -or $values.ContainsKey($Matches.key)) {
            throw "Verifier-generated release environment contains an unexpected or duplicate key."
        }
        $values[$Matches.key] = $Matches.value
    }
    if ($values.Count -ne $script:ReleaseEnvironmentNames.Count) {
        throw "Verifier-generated release environment must contain exactly ten entries."
    }
    foreach ($name in $script:ReleaseEnvironmentNames) {
        if (-not $values.ContainsKey($name)) {
            throw "Verifier-generated release environment is missing $name."
        }
        [Environment]::SetEnvironmentVariable($name, $values[$name], [EnvironmentVariableTarget]::Process)
    }
}

function Restore-ReleaseProcessEnvironment {
    param([Parameter(Mandatory = $true)][hashtable]$Saved)

    foreach ($name in $script:ReleaseEnvironmentNames) {
        if ($Saved[$name].Exists) {
            [Environment]::SetEnvironmentVariable($name, $Saved[$name].Value, [EnvironmentVariableTarget]::Process)
        } else {
            [Environment]::SetEnvironmentVariable($name, $null, [EnvironmentVariableTarget]::Process)
        }
    }
}

$releaseEnvFile = Join-Path ([IO.Path]::GetTempPath()) ("vk-ai-aggregator-rollback-release-{0}.env" -f ([guid]::NewGuid().ToString("N")))
$savedReleaseEnvironment = Save-ReleaseProcessEnvironment
$previousAppEnvFile = $env:APP_ENV_FILE
try {
    Invoke-Step "check Docker" {
        Test-DockerRuntime
    }
    Invoke-Step "check production env" {
        $checkArgs = @{ EnvFile = $EnvFile }
        if ($WithCloudflare) {
            $checkArgs.WithCloudflare = $true
        }
        if (-not $SkipBackup) {
            $checkArgs.BackupBeforeDeploy = $true
        }
        & (Join-Path $PSScriptRoot "check-prod-env.ps1") @checkArgs
    }
    $ghcrUsername = Get-EnvFileValue -Path $EnvFile -Name "GHCR_USERNAME"
    $ghcrToken = Get-EnvFileValue -Path $EnvFile -Name "GHCR_TOKEN"
    if (-not (Test-EnvPlaceholderValue -Value $ghcrUsername) -and -not (Test-EnvPlaceholderValue -Value $ghcrToken)) {
        Invoke-Step "docker login ghcr.io for release verification" {
            $ghcrToken | docker login ghcr.io -u $ghcrUsername --password-stdin | Out-Null
            if ($LASTEXITCODE -ne 0) {
                throw "docker login ghcr.io failed with exit code $LASTEXITCODE"
            }
        }
    }
    Invoke-Step "verify previous signed release bundle" {
        $verifyArgs = @{
            ReleaseBundleDir = $ReleaseBundleDir
            WorkflowIdentity = $WorkflowIdentity
            OutputEnvFile = $releaseEnvFile
            CosignPath = $CosignPath
        }
        if (-not [string]::IsNullOrWhiteSpace($ReleaseManifestPath)) {
            $verifyArgs.ReleaseManifestPath = $ReleaseManifestPath
        }
        & (Join-Path $repoRoot "scripts\release\verify-release-bundle.ps1") @verifyArgs
    }
    Import-VerifiedReleaseEnvironment -Path $releaseEnvFile
    $releaseCommit = Get-EnvFileValue -Path $releaseEnvFile -Name "RELEASE_COMMIT_SHA"
    Write-Host "Rollback target verified release commit $releaseCommit"
    Write-Warning "This script does not run migrate down. Schema rollback must be a separate reviewed operation after a verified backup."

$statefulServices = @(Get-LocalStatefulServices -Path $EnvFile)
$providerBalanceBotEnabled = Test-TrueValue -Value (Get-EnvFileValue -Path $EnvFile -Name "PROVIDER_BALANCE_BOT_ENABLED" -Default "false")
$composeArgs = @(
    "compose",
    "--project-name", $ProjectName,
    "--env-file", $EnvFile,
    "--env-file", $releaseEnvFile,
    "-f", "docker-compose.prod.yml"
)
if ($statefulServices.Count -gt 0) {
    $composeArgs += @("-f", "docker-compose.data.yml")
}
if ($WithCloudflare) {
    $composeArgs += @("--profile", "cloudflare")
}
$composeArgs += @("--profile", "provider-balance")

function Invoke-DockerCompose {
    param([Parameter(ValueFromRemainingArguments = $true)][string[]]$Arguments)

    & docker @composeArgs @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "docker compose $($Arguments -join ' ') failed with exit code $LASTEXITCODE"
    }
}

    $env:APP_ENV_FILE = $EnvFile

    Invoke-Step "docker compose config" {
        Invoke-DockerCompose config | Out-Null
    }
    if ($DryRun) {
        Write-Host "Production rollback dry-run completed after signed bundle verification and docker compose config."
        return
    }

    if ($statefulServices.Count -gt 0) {
        Invoke-Step "ensure local stateful dependencies are running" {
            Invoke-DockerCompose pull @statefulServices
            Invoke-DockerCompose up -d --no-build --wait --wait-timeout $TimeoutSeconds @statefulServices
        }
    } else {
        Write-Host "Skipping local stateful containers; DATA_SERVICES_MODE/POSTGRES_MODE/REDIS_MODE/S3_MODE point to external or managed services."
    }

    $backupArgs = @(
        "compose",
        "--project-name", $ProjectName,
        "--env-file", $EnvFile,
        "--env-file", $releaseEnvFile,
        "-f", "docker-compose.prod.yml"
    )
    if ($statefulServices.Count -gt 0) {
        $backupArgs += @("-f", "docker-compose.data.yml")
    }
    $backupArgs += @("--profile", "backup")

    if (-not $SkipBackup) {
        Invoke-Step "pull backup images" {
            & docker @backupArgs pull backup-postgres backup-minio
            if ($LASTEXITCODE -ne 0) { throw "docker compose pull backup services failed with exit code $LASTEXITCODE" }
        }
        Invoke-Step "backup postgres before rollback" {
            & docker @backupArgs run --rm backup-postgres
            if ($LASTEXITCODE -ne 0) { throw "backup-postgres failed with exit code $LASTEXITCODE" }
        }
        Invoke-Step "backup minio before rollback" {
            & docker @backupArgs run --rm backup-minio
            if ($LASTEXITCODE -ne 0) { throw "backup-minio failed with exit code $LASTEXITCODE" }
        }
        $backupStatus = "completed"
    } else {
        Write-Warning "Skipping backup before rollback. Use only if a fresh verified backup already exists."
        $backupStatus = "skipped by operator"
    }

    $runtimeServices = @("api", "worker", "maintenance-worker", "provider-webhook", "miniapp", "reverse-proxy")
    if ($providerBalanceBotEnabled) {
        $runtimeServices += "provider-balance-bot"
    } else {
        Invoke-Step "remove disabled provider balance bot" {
            & docker @composeArgs rm -f -s provider-balance-bot | Out-Null
            $global:LASTEXITCODE = 0
        }
    }
    if ($WithCloudflare) {
        $runtimeServices += "cloudflared"
    }

    Invoke-Step "pull rollback images" {
        Invoke-DockerCompose pull @runtimeServices
    }

    Invoke-Step "rollback runtime services without migrations" {
        Invoke-DockerCompose up -d --no-build --no-deps @runtimeServices
    }
    Invoke-Step "recreate reverse proxy to refresh upstream DNS" {
        Invoke-DockerCompose up -d --no-build --force-recreate --no-deps reverse-proxy
    }

    if (-not $NoHealthCheck) {
        $reverseProxyPort = Get-EnvFileValue -Path $EnvFile -Name "REVERSE_PROXY_HTTP_PORT" -Default "8088"
        Wait-Http -Name "reverse-proxy" -Url "http://127.0.0.1:$reverseProxyPort/proxy-health"
        Wait-Http -Name "api" -Url "http://127.0.0.1:8080/readyz"
        Wait-Http -Name "provider-webhook" -Url "http://127.0.0.1:8082/readyz"
        Wait-Http -Name "worker" -Url "http://127.0.0.1:9090/readyz"
        Wait-Http -Name "maintenance-worker" -Url "http://127.0.0.1:9091/readyz"
        Wait-Http -Name "miniapp" -Url "http://127.0.0.1:5173/"
        $healthStatus = "passed"
    }

    Invoke-Step "docker compose ps" {
        Invoke-DockerCompose ps
    }

    Write-Host ""
    Write-Host "Production rollback completed."
    Write-Host "Started at: $rollbackStartedAt"
    Write-Host "Finished at: $((Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ"))"
    Write-Host "Project: $ProjectName"
    Write-Host "Env file: $EnvFile"
    Write-Host "Rollback verified release bundle: $ReleaseBundleDir"
    Write-Host "Rollback release commit: $releaseCommit"
    Write-Host "Backup before rollback: $backupStatus"
    Write-Host "Migrations: not run; migrate down is intentionally forbidden"
    Write-Host "Runtime services: $($runtimeServices -join ', ')"
    Write-Host "Health checks: $healthStatus"
    Write-Host "Provider balance bot: $providerBalanceBotEnabled"
    Write-Host "Verify payment/referral/job smoke manually before considering the incident closed."
} finally {
    Restore-ReleaseProcessEnvironment -Saved $savedReleaseEnvironment
    Remove-Item -LiteralPath $releaseEnvFile -Force -ErrorAction SilentlyContinue
    if ($null -eq $previousAppEnvFile) {
        Remove-Item Env:\APP_ENV_FILE -ErrorAction SilentlyContinue
    } else {
        $env:APP_ENV_FILE = $previousAppEnvFile
    }
}
