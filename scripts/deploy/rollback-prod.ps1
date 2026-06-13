[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string]$ImageTag,
    [string]$EnvFile = ".env",
    [string]$ProjectName = "vk-ai-aggregator-prod",
    [switch]$WithCloudflare,
    [switch]$SkipBackup,
    [switch]$NoHealthCheck,
    [int]$TimeoutSeconds = 180
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..")
Set-Location $repoRoot

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

if ([string]::IsNullOrWhiteSpace($ImageTag)) {
    throw "ImageTag is required. Use the previous known-good Docker image tag."
}
if (-not (Test-Path -LiteralPath "docker-compose.prod.yml")) {
    throw "docker-compose.prod.yml not found"
}
if (-not (Test-Path -LiteralPath $EnvFile)) {
    throw "Production env file not found: $EnvFile"
}

$composeArgs = @(
    "compose",
    "--project-name", $ProjectName,
    "--env-file", $EnvFile,
    "-f", "docker-compose.prod.yml"
)
if ($WithCloudflare) {
    $composeArgs += @("--profile", "cloudflare")
}

function Invoke-DockerCompose {
    param([Parameter(ValueFromRemainingArguments = $true)][string[]]$Arguments)

    & docker @composeArgs @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "docker compose $($Arguments -join ' ') failed with exit code $LASTEXITCODE"
    }
}

$previousAppEnvFile = $env:APP_ENV_FILE
$previousImageTag = $env:IMAGE_TAG
try {
    $env:APP_ENV_FILE = $EnvFile
    $env:IMAGE_TAG = $ImageTag

    Write-Host "Rollback target IMAGE_TAG=$ImageTag"
    Write-Warning "This script does not run migrate down. Schema rollback must be a separate reviewed operation after a verified backup."

    Invoke-Step "docker compose config" {
        Invoke-DockerCompose config | Out-Null
    }

    if (-not $SkipBackup) {
        $backupArgs = @(
            "compose",
            "--project-name", $ProjectName,
            "--env-file", $EnvFile,
            "-f", "docker-compose.prod.yml",
            "--profile", "backup"
        )
        Invoke-Step "backup postgres before rollback" {
            & docker @backupArgs run --rm backup-postgres
            if ($LASTEXITCODE -ne 0) { throw "backup-postgres failed with exit code $LASTEXITCODE" }
        }
        Invoke-Step "backup minio before rollback" {
            & docker @backupArgs run --rm backup-minio
            if ($LASTEXITCODE -ne 0) { throw "backup-minio failed with exit code $LASTEXITCODE" }
        }
    } else {
        Write-Warning "Skipping backup before rollback. Use only if a fresh verified backup already exists."
    }

    Invoke-Step "ensure stateful dependencies are running" {
        Invoke-DockerCompose up -d postgres redis minio
    }

    $runtimeServices = @("api", "worker", "provider-webhook", "miniapp", "reverse-proxy")
    if ($WithCloudflare) {
        $runtimeServices += "cloudflared"
    }

    Invoke-Step "rollback runtime services without migrations" {
        Invoke-DockerCompose up -d --no-build --no-deps @runtimeServices
    }

    if (-not $NoHealthCheck) {
        $reverseProxyPort = Get-EnvFileValue -Path $EnvFile -Name "REVERSE_PROXY_HTTP_PORT" -Default "8088"
        Wait-Http -Name "reverse-proxy" -Url "http://127.0.0.1:$reverseProxyPort/proxy-health"
        Wait-Http -Name "api" -Url "http://127.0.0.1:8080/health"
        Wait-Http -Name "provider-webhook" -Url "http://127.0.0.1:8082/health"
        Wait-Http -Name "worker" -Url "http://127.0.0.1:9090/healthz"
        Wait-Http -Name "miniapp" -Url "http://127.0.0.1:5173/"
    }

    Invoke-Step "docker compose ps" {
        Invoke-DockerCompose ps
    }

    Write-Host ""
    Write-Host "Production rollback completed. Verify payment/referral/job smoke manually before considering the incident closed."
} finally {
    if ($null -eq $previousAppEnvFile) {
        Remove-Item Env:\APP_ENV_FILE -ErrorAction SilentlyContinue
    } else {
        $env:APP_ENV_FILE = $previousAppEnvFile
    }
    if ($null -eq $previousImageTag) {
        Remove-Item Env:\IMAGE_TAG -ErrorAction SilentlyContinue
    } else {
        $env:IMAGE_TAG = $previousImageTag
    }
}
