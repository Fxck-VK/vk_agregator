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
$migrationBackupStatus = "skipped"
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

function Normalize-AppEnv {
    param([string]$Value)

    if ([string]::IsNullOrWhiteSpace($Value)) {
        return "production"
    }
    $normalized = $Value.Trim().ToLowerInvariant()
    switch ($normalized) {
        "prod" { return "production" }
        "stage" { return "staging" }
        default { return $normalized }
    }
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

function Invoke-ExternalPostgresCheck {
    param([Parameter(Mandatory = $true)][string]$Path)

    $databaseUrl = Get-EnvFileValue -Path $Path -Name "DATABASE_URL"
    & docker run --rm --network host -e "DATABASE_URL=$databaseUrl" postgres:16-alpine sh -ec 'pg_isready -d "$DATABASE_URL" >/dev/null'
    if ($LASTEXITCODE -ne 0) {
        throw "external Postgres check failed with exit code $LASTEXITCODE"
    }
}

function Split-RedisAddress {
    param([Parameter(Mandatory = $true)][string]$Address)

    if ($Address -match '^\[(?<host>.+)\]:(?<port>\d+)$') {
        return [pscustomobject]@{ Host = $Matches.host; Port = $Matches.port }
    }

    $lastColon = $Address.LastIndexOf(":")
    if ($lastColon -gt 0) {
        return [pscustomobject]@{
            Host = $Address.Substring(0, $lastColon)
            Port = $Address.Substring($lastColon + 1)
        }
    }

    return [pscustomobject]@{ Host = $Address; Port = "6379" }
}

function Invoke-ExternalRedisCheck {
    param([Parameter(Mandatory = $true)][string]$Path)

    $redisAddr = Get-EnvFileValue -Path $Path -Name "REDIS_ADDR"
    $redisPassword = Get-EnvFileValue -Path $Path -Name "REDIS_PASSWORD"
    $redisDb = Get-EnvFileValue -Path $Path -Name "REDIS_DB" -Default "0"
    $redisEndpoint = Split-RedisAddress -Address $redisAddr

    & docker run --rm --network host `
        -e "REDISCLI_AUTH=$redisPassword" `
        -e "REDIS_CHECK_HOST=$($redisEndpoint.Host)" `
        -e "REDIS_CHECK_PORT=$($redisEndpoint.Port)" `
        -e "REDIS_CHECK_DB=$redisDb" `
        redis:7-alpine `
        sh -ec 'redis-cli -h "$REDIS_CHECK_HOST" -p "$REDIS_CHECK_PORT" -n "$REDIS_CHECK_DB" ping | grep -qx PONG'
    if ($LASTEXITCODE -ne 0) {
        throw "external Redis check failed with exit code $LASTEXITCODE"
    }
}

function Invoke-ExternalS3Check {
    param([Parameter(Mandatory = $true)][string]$Path)

    $s3Endpoint = Get-EnvFileValue -Path $Path -Name "S3_ENDPOINT"
    $s3AccessKey = Get-EnvFileValue -Path $Path -Name "S3_ACCESS_KEY"
    $s3SecretKey = Get-EnvFileValue -Path $Path -Name "S3_SECRET_KEY"
    $s3Bucket = Get-EnvFileValue -Path $Path -Name "S3_BUCKET"
    $s3UseSsl = (Get-EnvFileValue -Path $Path -Name "S3_USE_SSL" -Default "false").ToLowerInvariant()

    & docker run --rm --network host `
        -e "S3_ENDPOINT=$s3Endpoint" `
        -e "S3_ACCESS_KEY=$s3AccessKey" `
        -e "S3_SECRET_KEY=$s3SecretKey" `
        -e "S3_BUCKET=$s3Bucket" `
        -e "S3_USE_SSL=$s3UseSsl" `
        minio/mc:latest `
        sh -ec 'case "$S3_ENDPOINT" in http://*|https://*) endpoint_url="$S3_ENDPOINT" ;; *) if [ "$S3_USE_SSL" = "true" ]; then endpoint_url="https://$S3_ENDPOINT"; else endpoint_url="http://$S3_ENDPOINT"; fi ;; esac; mc alias set target "$endpoint_url" "$S3_ACCESS_KEY" "$S3_SECRET_KEY" >/dev/null; mc ls "target/$S3_BUCKET" >/dev/null'
    if ($LASTEXITCODE -ne 0) {
        throw "external S3 check failed with exit code $LASTEXITCODE"
    }
}

function Invoke-ExternalDataServiceChecks {
    param([Parameter(Mandatory = $true)][string]$Path)

    if ((Get-DataServiceMode -Path $Path -Name "POSTGRES_MODE") -ne "local") {
        Invoke-Step "check external Postgres" {
            Invoke-ExternalPostgresCheck -Path $Path
        }
    }
    if ((Get-DataServiceMode -Path $Path -Name "REDIS_MODE") -ne "local") {
        Invoke-Step "check external Redis" {
            Invoke-ExternalRedisCheck -Path $Path
        }
    }
    if ((Get-DataServiceMode -Path $Path -Name "S3_MODE") -ne "local") {
        Invoke-Step "check external S3" {
            Invoke-ExternalS3Check -Path $Path
        }
    }
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

$statefulServices = @(Get-LocalStatefulServices -Path $EnvFile)
$providerBalanceBotEnabled = Test-TrueValue -Value (Get-EnvFileValue -Path $EnvFile -Name "PROVIDER_BALANCE_BOT_ENABLED" -Default "false")
$script:ComposeArgs = @(
    "compose",
    "--project-name", $ProjectName,
    "--env-file", $EnvFile,
    "-f", "docker-compose.prod.yml"
)
if ($statefulServices.Count -gt 0) {
    $script:ComposeArgs += @("-f", "docker-compose.data.yml")
}
if ($WithCloudflare) {
    $script:ComposeArgs += @("--profile", "cloudflare")
}
$script:ComposeArgs += @("--profile", "provider-balance")

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

    $imagePullServices = @($statefulServices + @("reverse-proxy"))
    if (-not $shouldBuildOnVPS) {
        $imagePullServices += @("api", "worker", "maintenance-worker", "provider-webhook", "miniapp", "migrate")
        if ($providerBalanceBotEnabled) {
            $imagePullServices += "provider-balance-bot"
        }
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

    if ($statefulServices.Count -gt 0) {
        Invoke-Step "start local stateful dependencies" {
            Invoke-DockerCompose up -d --no-build --wait --wait-timeout $TimeoutSeconds @statefulServices
        }
    } else {
        Write-Host "Skipping local stateful containers; DATA_SERVICES_MODE/POSTGRES_MODE/REDIS_MODE/S3_MODE point to external or managed services."
    }

    Invoke-ExternalDataServiceChecks -Path $EnvFile

    if ($BackupBeforeDeploy) {
        $backupArgs = @(
            "compose",
            "--project-name", $ProjectName,
            "--env-file", $EnvFile,
            "-f", "docker-compose.prod.yml"
        )
        if ($statefulServices.Count -gt 0) {
            $backupArgs += @("-f", "docker-compose.data.yml")
        }
        $backupArgs += @("--profile", "backup")
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

    if ($shouldBuildOnVPS) {
        $buildArgs = @("build")
        if ($PullBaseImages) {
            $buildArgs += "--pull"
        }
        $buildArgs += @("api", "worker", "maintenance-worker", "provider-webhook", "miniapp", "migrate")
        if ($providerBalanceBotEnabled) {
            $buildArgs += "provider-balance-bot"
        }
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
        Invoke-Step "check migrations safety" {
            & (Join-Path $PSScriptRoot "check-migrations-safe.ps1") -EnvFile $EnvFile -MigrationsDir (Get-EnvFileValue -Path $EnvFile -Name "MIGRATIONS_DIR" -Default "migrations")
            if ($LASTEXITCODE -ne 0) { throw "check-migrations-safe.ps1 failed with exit code $LASTEXITCODE" }
            $global:LASTEXITCODE = 0
        }

        $appEnvNormalized = Normalize-AppEnv -Value (Get-EnvFileValue -Path $EnvFile -Name "APP_ENV" -Default "production")
        if ($appEnvNormalized -eq "production") {
            if ((Get-EnvFileValue -Path $EnvFile -Name "MIGRATION_BACKUP_CONFIRMED" -Default "false").ToLowerInvariant() -eq "true") {
                $migrationBackupStatus = "manual-confirmed"
                Write-Host "Using manually confirmed production migration backup."
            } else {
                $backupArgs = @(
                    "compose",
                    "--project-name", $ProjectName,
                    "--env-file", $EnvFile,
                    "-f", "docker-compose.prod.yml"
                )
                if ($statefulServices.Count -gt 0) {
                    $backupArgs += @("-f", "docker-compose.data.yml")
                }
                $backupArgs += @("--profile", "backup")
                if ($WithCloudflare) {
                    $backupArgs += @("--profile", "cloudflare")
                }
                if (-not $shouldBuildOnVPS) {
                    Invoke-Step "docker compose pull backup-postgres" {
                        Invoke-DockerCompose pull backup-postgres
                    }
                }
                Invoke-Step "backup postgres before migration" {
                    & docker @backupArgs run --rm backup-postgres
                    if ($LASTEXITCODE -ne 0) { throw "backup-postgres failed with exit code $LASTEXITCODE" }
                }
                $migrationBackupStatus = "postgres-backup"
            }
        }

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

    if (-not $providerBalanceBotEnabled) {
        Invoke-Step "remove disabled provider balance bot" {
            & docker @script:ComposeArgs rm -f -s provider-balance-bot | Out-Null
            $global:LASTEXITCODE = 0
        }
    }

    $runtimeServices = @("api", "worker", "maintenance-worker", "provider-webhook", "miniapp", "reverse-proxy")
    if ($providerBalanceBotEnabled) {
        $runtimeServices += "provider-balance-bot"
    }
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
    Invoke-Step "recreate reverse proxy to refresh upstream DNS" {
        Invoke-DockerCompose up -d --no-build --force-recreate --no-deps reverse-proxy
    }

    if (-not $NoHealthCheck) {
        $reverseProxyPort = Get-EnvFileValue -Path $EnvFile -Name "REVERSE_PROXY_HTTP_PORT" -Default "8088"
        Wait-Http -Name "reverse-proxy" -Url "http://127.0.0.1:$reverseProxyPort/proxy-health" -TimeoutSeconds $TimeoutSeconds
        Wait-Http -Name "api" -Url "http://127.0.0.1:8080/readyz" -TimeoutSeconds $TimeoutSeconds
        Wait-Http -Name "provider-webhook" -Url "http://127.0.0.1:8082/readyz" -TimeoutSeconds $TimeoutSeconds
        Wait-Http -Name "worker" -Url "http://127.0.0.1:9090/readyz" -TimeoutSeconds $TimeoutSeconds
        Wait-Http -Name "maintenance-worker" -Url "http://127.0.0.1:9091/readyz" -TimeoutSeconds $TimeoutSeconds
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
    Write-Host "Migration backup: $migrationBackupStatus"
    Write-Host "Image pull: completed"
    Write-Host "Build: $buildStatus"
    Write-Host "Health checks: $healthStatus"
    Write-Host "Public Cloudflare/DNS smoke: $publicSmokeStatus"
    Write-Host "Cloudflare tunnel profile: $cloudflareStatus"
    Write-Host "Provider balance bot: $providerBalanceBotEnabled"
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
