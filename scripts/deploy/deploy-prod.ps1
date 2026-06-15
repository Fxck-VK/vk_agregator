[CmdletBinding()]
param(
    [string]$Branch = "main",
    [string]$EnvFile = ".env",
    [string]$ProjectName = "vk-ai-aggregator-prod",
    [string]$ImageTag = "",
    [switch]$SkipPull,
    [switch]$AllowDirty,
    [switch]$BuildOnVPS,
    [switch]$SkipBuild,
    [switch]$SkipMigrate,
    [switch]$WithCloudflare,
    [switch]$BackupBeforeDeploy,
    [switch]$PullBaseImages,
    [switch]$NoHealthCheck,
    [switch]$SkipPublicSmoke,
    [int]$TimeoutSeconds = 180
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$script:RepoRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..")
Set-Location $script:RepoRoot
$deployStartedAt = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
$healthStatus = "skipped"
$publicSmokeStatus = "skipped"
$shouldBuildOnVPS = $BuildOnVPS -and -not $SkipBuild

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

function Get-PublicUrlValue {
    param(
        [Parameter(Mandatory = $true)][string]$Path,
        [Parameter(Mandatory = $true)][string]$PrimaryName,
        [Parameter(Mandatory = $true)][string]$LegacyName,
        [Parameter(Mandatory = $true)][string]$Default
    )

    $value = Get-EnvFileValue -Path $Path -Name $PrimaryName
    if (-not [string]::IsNullOrWhiteSpace($value)) {
        return $value
    }
    $value = Get-EnvFileValue -Path $Path -Name $LegacyName
    if (-not [string]::IsNullOrWhiteSpace($value)) {
        return $value
    }
    return $Default
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
        [Parameter(Mandatory = $true)][string]$Url,
        [Parameter(Mandatory = $true)][int]$TimeoutSeconds
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

if (-not (Test-Path -LiteralPath "docker-compose.prod.yml")) {
    throw "docker-compose.prod.yml not found; run from the repository root or keep this script in scripts/deploy"
}
if (-not (Test-Path -LiteralPath $EnvFile)) {
    throw "Server env file not found: $EnvFile. Copy .env.staging.example or .env.prod.example to .env on the server and fill real values there."
}

Invoke-Step "check Docker" {
    Test-DockerRuntime
}

Invoke-Step "check production env" {
    $checkArgs = @("-EnvFile", $EnvFile)
    if ($WithCloudflare) {
        $checkArgs += "-WithCloudflare"
    }
    if ($BackupBeforeDeploy) {
        $checkArgs += "-BackupBeforeDeploy"
    }
    & (Join-Path $PSScriptRoot "check-prod-env.ps1") @checkArgs
}

$script:ComposeArgs = @(
    "compose",
    "--project-name", $ProjectName,
    "--env-file", $EnvFile,
    "-f", "docker-compose.prod.yml"
)
if ($WithCloudflare) {
    $script:ComposeArgs += @("--profile", "cloudflare")
}

function Invoke-DockerCompose {
    param([Parameter(ValueFromRemainingArguments = $true)][string[]]$Arguments)

    & docker @script:ComposeArgs @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "docker compose $($Arguments -join ' ') failed with exit code $LASTEXITCODE"
    }
}

if (-not $SkipPull) {
    if (-not $AllowDirty) {
        $dirty = @(git status --porcelain --untracked-files=no)
        if ($LASTEXITCODE -ne 0) {
            throw "git status failed"
        }
        if ($dirty.Count -gt 0) {
            throw "Tracked worktree changes found. Commit/stash them or rerun with -AllowDirty. Changes: $($dirty -join '; ')"
        }
    }

    Invoke-Step "git fetch" { git fetch --prune origin }
    Invoke-Step "git checkout $Branch" { git checkout $Branch }
    Invoke-Step "git pull --ff-only origin $Branch" { git pull --ff-only origin $Branch }
}

$previousAppEnvFile = $env:APP_ENV_FILE
$previousImageTag = $env:IMAGE_TAG
try {
    $env:APP_ENV_FILE = $EnvFile
    if (-not [string]::IsNullOrWhiteSpace($ImageTag)) {
        $env:IMAGE_TAG = $ImageTag
        Write-Host "Using IMAGE_TAG=$ImageTag"
    }

    Invoke-Step "docker compose config" {
        Invoke-DockerCompose config | Out-Null
    }

    $ghcrUsername = Get-EnvFileValue -Path $EnvFile -Name "GHCR_USERNAME"
    $ghcrToken = Get-EnvFileValue -Path $EnvFile -Name "GHCR_TOKEN"
    if (-not (Test-EnvPlaceholderValue -Value $ghcrUsername) -and -not (Test-EnvPlaceholderValue -Value $ghcrToken)) {
        Invoke-Step "docker login ghcr.io" {
            $ghcrToken | docker login ghcr.io -u $ghcrUsername --password-stdin | Out-Null
            if ($LASTEXITCODE -ne 0) {
                throw "docker login ghcr.io failed with exit code $LASTEXITCODE"
            }
        }
    }

    $imagePullServices = @("postgres", "redis", "minio", "reverse-proxy")
    if (-not $shouldBuildOnVPS) {
        $imagePullServices += @("api", "worker", "provider-webhook", "miniapp", "migrate")
        if ($BackupBeforeDeploy) {
            $imagePullServices += @("backup-postgres", "backup-minio")
        }
    }
    if ($WithCloudflare) {
        $imagePullServices += "cloudflared"
    }
    Invoke-Step "docker compose pull" {
        Invoke-DockerCompose pull @imagePullServices
    }

    if ($BackupBeforeDeploy) {
        $backupArgs = @(
            "compose",
            "--project-name", $ProjectName,
            "--env-file", $EnvFile,
            "-f", "docker-compose.prod.yml",
            "--profile", "backup"
        )
        if ($WithCloudflare) {
            $backupArgs += @("--profile", "cloudflare")
        }
        Invoke-Step "backup postgres" {
            & docker @backupArgs run --rm backup-postgres
            if ($LASTEXITCODE -ne 0) { throw "backup-postgres failed with exit code $LASTEXITCODE" }
        }
        Invoke-Step "backup minio" {
            & docker @backupArgs run --rm backup-minio
            if ($LASTEXITCODE -ne 0) { throw "backup-minio failed with exit code $LASTEXITCODE" }
        }
    }

    Invoke-Step "start stateful dependencies" {
        Invoke-DockerCompose up -d --no-build postgres redis minio
    }

    if ($shouldBuildOnVPS) {
        $buildArgs = @("build")
        if ($PullBaseImages) {
            $buildArgs += "--pull"
        }
        $buildArgs += @("api", "worker", "provider-webhook", "miniapp", "migrate")
        if ($BackupBeforeDeploy) {
            $buildArgs += @("backup-postgres", "backup-minio")
        }
        Invoke-Step "docker compose build" {
            Invoke-DockerCompose @buildArgs
        }
    } else {
        Write-Host "Skipping VPS image build; using images pulled from registry."
    }

    if (-not $SkipMigrate) {
        Invoke-Step "remove old migrate container" {
            & docker @script:ComposeArgs rm -f -s migrate | Out-Null
            $global:LASTEXITCODE = 0
        }
        Invoke-Step "run migrations" {
            $migrateArgs = @("up", "--no-deps", "--exit-code-from", "migrate")
            if (-not $shouldBuildOnVPS) {
                $migrateArgs += "--no-build"
            }
            $migrateArgs += "migrate"
            Invoke-DockerCompose @migrateArgs
        }
    } else {
        Write-Warning "Skipping migrations. Runtime services still require a successful migrate service state in this compose project."
    }

    $runtimeServices = @("api", "worker", "provider-webhook", "miniapp", "reverse-proxy")
    if ($WithCloudflare) {
        $runtimeServices += "cloudflared"
    }

    Invoke-Step "start runtime services" {
        $runtimeUpArgs = @("up", "-d")
        if (-not $shouldBuildOnVPS) {
            $runtimeUpArgs += "--no-build"
        }
        $runtimeUpArgs += $runtimeServices
        Invoke-DockerCompose @runtimeUpArgs
    }

    if (-not $NoHealthCheck) {
        $reverseProxyPort = Get-EnvFileValue -Path $EnvFile -Name "REVERSE_PROXY_HTTP_PORT" -Default "8088"
        Wait-Http -Name "reverse-proxy" -Url "http://127.0.0.1:$reverseProxyPort/proxy-health" -TimeoutSeconds $TimeoutSeconds
        Wait-Http -Name "api" -Url "http://127.0.0.1:8080/health" -TimeoutSeconds $TimeoutSeconds
        Wait-Http -Name "provider-webhook" -Url "http://127.0.0.1:8082/health" -TimeoutSeconds $TimeoutSeconds
        Wait-Http -Name "worker" -Url "http://127.0.0.1:9090/healthz" -TimeoutSeconds $TimeoutSeconds
        Wait-Http -Name "miniapp" -Url "http://127.0.0.1:5173/" -TimeoutSeconds $TimeoutSeconds
        $healthStatus = "passed"

        if ($WithCloudflare -and -not $SkipPublicSmoke) {
            $publicVkUrl = Get-PublicUrlValue -Path $EnvFile -PrimaryName "PUBLIC_VK_BASE_URL" -LegacyName "VK_BASE_URL" -Default "https://vk.neiirohub.ru"
            $publicAppUrl = Get-PublicUrlValue -Path $EnvFile -PrimaryName "PUBLIC_APP_BASE_URL" -LegacyName "APP_BASE_URL" -Default "https://app.neiirohub.ru"
            $publicPaymentWebhookUrl = Get-PublicUrlValue -Path $EnvFile -PrimaryName "PUBLIC_PAYMENT_WEBHOOK_URL" -LegacyName "PAYMENT_WEBHOOK_URL" -Default "https://neiirohub.ru/billing/webhooks/yookassa"
            Invoke-Step "public Cloudflare/DNS smoke" {
                & (Join-Path $PSScriptRoot "smoke-prod.ps1") `
                    -EnvFile $EnvFile `
                    -VkBaseUrl $publicVkUrl `
                    -AppBaseUrl $publicAppUrl `
                    -PaymentWebhookUrl $publicPaymentWebhookUrl `
                    -TimeoutSeconds $TimeoutSeconds
            }
            $publicSmokeStatus = "passed"
        }
    }

    Invoke-Step "docker compose ps" {
        Invoke-DockerCompose ps
    }

    Write-Host ""
    Write-Host "Production deploy completed."
    Write-Host "Started at: $deployStartedAt"
    Write-Host "Finished at: $((Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ"))"
    Write-Host "Branch: $Branch"
    $commit = (& git rev-parse --short HEAD 2>$null)
    if ([string]::IsNullOrWhiteSpace($commit)) {
        $commit = "unknown"
    }
    Write-Host "Commit: $commit"
    Write-Host "Project: $ProjectName"
    Write-Host "Env file: $EnvFile"
    Write-Host "Runtime services: $($runtimeServices -join ', ')"
    $migrationsStatus = if ($SkipMigrate) { "skipped" } else { "applied" }
    $buildStatus = if ($shouldBuildOnVPS) { "completed on VPS" } else { "skipped; pulled registry images" }
    $cloudflareStatus = if ($WithCloudflare) { "enabled" } else { "disabled" }
    Write-Host "Migrations: $migrationsStatus"
    Write-Host "Image pull: completed"
    Write-Host "Build: $buildStatus"
    Write-Host "Health checks: $healthStatus"
    Write-Host "Public Cloudflare/DNS smoke: $publicSmokeStatus"
    Write-Host "Cloudflare tunnel profile: $cloudflareStatus"
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
