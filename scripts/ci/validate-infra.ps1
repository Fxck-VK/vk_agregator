[CmdletBinding()]
param(
    [switch]$SkipPromtool
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

function Get-TrackedFiles {
    $files = git ls-files
    if ($LASTEXITCODE -ne 0) {
        throw "git ls-files failed"
    }
    return @($files)
}

function Assert-Migrations {
    $migrationDir = Join-Path $repoRoot "migrations"
    if (-not (Test-Path -LiteralPath $migrationDir)) {
        throw "migrations directory is missing"
    }

    $files = @(Get-ChildItem -LiteralPath $migrationDir -File -Filter "*.sql" | Sort-Object Name)
    if ($files.Count -eq 0) {
        throw "migrations directory has no sql files"
    }

    $pattern = "^(?<id>\d{6})_(?<slug>[a-z0-9_]+)\.(?<direction>up|down)\.sql$"
    $parsed = @()
    foreach ($file in $files) {
        if ($file.Name -notmatch $pattern) {
            throw "invalid migration name: $($file.Name)"
        }
        $parsed += [pscustomobject]@{
            ID = $Matches.id
            Slug = $Matches.slug
            Direction = $Matches.direction
            Name = $file.Name
        }
    }

    $duplicateDirections = @(
        $parsed |
            Group-Object ID, Direction |
            Where-Object { $_.Count -gt 1 } |
            ForEach-Object { $_.Name }
    )
    if ($duplicateDirections.Count -gt 0) {
        throw "duplicate migration directions: $($duplicateDirections -join ', ')"
    }

    $byID = $parsed | Group-Object ID | Sort-Object Name
    for ($index = 0; $index -lt $byID.Count; $index++) {
        $expectedID = "{0:D6}" -f ($index + 1)
        $group = $byID[$index]
        if ($group.Name -ne $expectedID) {
            throw "migration id gap or order mismatch: expected $expectedID, got $($group.Name)"
        }

        $directions = @($group.Group | ForEach-Object { $_.Direction })
        if ($directions -notcontains "up" -or $directions -notcontains "down") {
            throw "migration $($group.Name) must have both up and down files"
        }

        $slugs = @($group.Group | Select-Object -ExpandProperty Slug -Unique)
        if ($slugs.Count -ne 1) {
            throw "migration $($group.Name) up/down slugs differ"
        }
    }

    Write-Host "migrations OK: $($byID.Count) pairs"
}

function Assert-NoTrackedEnvFiles {
    $tracked = Get-TrackedFiles
    $allowedTrackedEnv = @(
        ".env.example",
        ".env.prod.example"
    )
    $bad = @(
        $tracked | Where-Object {
            $leaf = Split-Path $_ -Leaf
            ($leaf -eq ".env" -or $leaf -like ".env.*") -and $allowedTrackedEnv -notcontains $leaf
        }
    )

    if ($bad.Count -gt 0) {
        throw "tracked env files are forbidden: $($bad -join ', ')"
    }

    Write-Host "tracked env files OK"
}

function Assert-CloudflareConfigHasNoSecrets {
    $tracked = Get-TrackedFiles
    $candidates = @(
        $tracked | Where-Object {
            $_ -match "(?i)(cloudflare|cloudflared|tunnel)"
        }
    )

    $secretPatterns = @(
        [pscustomobject]@{
            Name = "dashboard tunnel token"
            Pattern = "(?i)(TUNNEL_TOKEN|tunnel_token|cloudflare[_-]?tunnel[_-]?token)\s*[:=]\s*['""]?eyJ[A-Za-z0-9_-]+"
        },
        [pscustomobject]@{
            Name = "cloudflared command token"
            Pattern = "(?i)cloudflared(?:\.exe)?\s+(?:service\s+install|tunnel\s+run)\s+eyJ[A-Za-z0-9_-]+"
        },
        [pscustomobject]@{
            Name = "cloudflare tunnel credentials json"
            Pattern = '(?i)"TunnelSecret"\s*:'
        },
        [pscustomobject]@{
            Name = "cloudflare jwt-like token"
            Pattern = "eyJhIjoi[A-Za-z0-9_-]{20,}"
        }
    )

    foreach ($file in $candidates) {
        if (-not (Test-Path -LiteralPath $file)) {
            continue
        }
        $content = Get-Content -LiteralPath $file -Raw
        foreach ($secretPattern in $secretPatterns) {
            if ($content -match $secretPattern.Pattern) {
                throw "possible Cloudflare secret in $file ($($secretPattern.Name))"
            }
        }
    }

    Write-Host "Cloudflare tracked config/script secret check OK: $($candidates.Count) files"
}

function Assert-ReverseProxyConfig {
    $path = Join-Path $repoRoot "deployments\nginx\nginx.prod.conf"
    if (-not (Test-Path -LiteralPath $path)) {
        Write-Host "no production nginx reverse proxy config found; skipping"
        return
    }

    $content = Get-Content -LiteralPath $path -Raw
    $requiredSnippets = @(
        "server_name vk.neiirohub.ru;",
        "server_name app.neiirohub.ru;",
        "server_name neiirohub.ru;",
        "location = /webhooks/vk",
        "location = /billing/webhooks/yookassa",
        "location ^~ /miniapp/",
        "proxy_pass http://api;",
        "proxy_pass http://provider_webhook;",
        "proxy_pass http://miniapp_frontend;",
        "X-Forwarded-Proto",
        "proxy_set_header X-Forwarded-Proto https;",
        'proxy_set_header Forwarded "proto=https;host=$host";',
        "/(admin|metrics|debug"
    )

    foreach ($snippet in $requiredSnippets) {
        if (-not $content.Contains($snippet)) {
            throw "reverse proxy config is missing required snippet: $snippet"
        }
    }

    if ($content -match '\$request(?!_)') {
        throw "reverse proxy access log must not use `$request because it includes query strings"
    }

    Write-Host "reverse proxy config OK"
}

function Assert-CloudflareDeploymentConfig {
    $path = Join-Path $repoRoot "deployments\cloudflare\cloudflared.prod.example.yml"
    if (-not (Test-Path -LiteralPath $path)) {
        Write-Host "no production cloudflared config example found; skipping"
        return
    }

    $content = Get-Content -LiteralPath $path -Raw
    $forbiddenPatterns = @(
        "eyJhIjoi[A-Za-z0-9_-]{20,}",
        '(?i)"TunnelSecret"\s*:'
    )
    foreach ($pattern in $forbiddenPatterns) {
        if ($content -match $pattern) {
            throw "cloudflared config example contains a value that looks like a real tunnel credential"
        }
    }

    $requiredSnippets = @(
        "hostname: vk.neiirohub.ru",
        "hostname: app.neiirohub.ru",
        "hostname: neiirohub.ru",
        "service: http://127.0.0.1:8088",
        "service: http_status:404"
    )

    foreach ($snippet in $requiredSnippets) {
        if (-not $content.Contains($snippet)) {
            throw "cloudflared config example is missing required snippet: $snippet"
        }
    }

    Write-Host "cloudflared production example OK"
}

function Assert-ProductionDataServices {
    $path = Join-Path $repoRoot "docker-compose.prod.yml"
    if (-not (Test-Path -LiteralPath $path)) {
        Write-Host "no production compose file found; skipping data-service checks"
        return
    }

    $requiredFiles = @(
        "Dockerfile.migrate",
        "Dockerfile.backup",
        "scripts\backup\backup-postgres.sh",
        "scripts\backup\backup-minio.sh"
    )
    foreach ($requiredFile in $requiredFiles) {
        $fullPath = Join-Path $repoRoot $requiredFile
        if (-not (Test-Path -LiteralPath $fullPath)) {
            throw "production data-service support file is missing: $requiredFile"
        }
    }

    $content = Get-Content -LiteralPath $path -Raw
    $requiredSnippets = @(
        "postgres_data:/var/lib/postgresql/data",
        "redis_data:/data",
        "minio_data:/data",
        "migrate:",
        "Dockerfile.migrate",
        "condition: service_completed_successfully",
        "backup-postgres:",
        "backup-minio:",
        "Dockerfile.backup",
        "backup_data:/backups",
        "backup_metrics:/backup-metrics",
        "postgres_data:",
        "redis_data:",
        "minio_data:",
        "backup_data:",
        "backup_metrics:"
    )

    foreach ($snippet in $requiredSnippets) {
        if (-not $content.Contains($snippet)) {
            throw "production compose data-service config is missing required snippet: $snippet"
        }
    }

    Write-Host "production data services config OK"
}

function Assert-DeployScripts {
    $scripts = @(
        [pscustomobject]@{
            Path = "scripts\deploy\deploy-prod.ps1"
            Required = @(
                "git pull --ff-only origin",
                "docker-compose.prod.yml",
                "IMAGE_TAG",
                "up --no-deps --exit-code-from migrate migrate",
                "api", "worker", "provider-webhook", "miniapp", "reverse-proxy",
                "Wait-Http"
            )
        },
        [pscustomobject]@{
            Path = "scripts\deploy\deploy-prod.sh"
            Required = @(
                "git pull --ff-only origin",
                "docker-compose.prod.yml",
                "IMAGE_TAG",
                "up --no-deps --exit-code-from migrate migrate",
                "api worker provider-webhook miniapp reverse-proxy",
                "wait_http"
            )
        },
        [pscustomobject]@{
            Path = "scripts\deploy\rollback-prod.ps1"
            Required = @(
                "ImageTag",
                "backup postgres before rollback",
                "backup minio before rollback",
                "does not run migrate down",
                "up -d --no-build --no-deps",
                "Wait-Http"
            )
        },
        [pscustomobject]@{
            Path = "scripts\deploy\rollback-prod.sh"
            Required = @(
                "--image-tag",
                "backup-postgres",
                "backup-minio",
                "does not run migrate down",
                "up -d --no-build --no-deps",
                "wait_http"
            )
        },
        [pscustomobject]@{
            Path = "scripts\deploy\smoke-prod.ps1"
            Required = @(
                "/health",
                "/miniapp/balance",
                "/billing/webhooks/yookassa",
                "PaymentWebhookOnly",
                "must use https",
                "/billing/payment-intents",
                "/admin/jobs",
                "/metrics",
                "VK /start",
                "YooKassa payment.succeeded",
                "artifact delivery"
            )
        },
        [pscustomobject]@{
            Path = "scripts\deploy\smoke-prod.sh"
            Required = @(
                "/health",
                "/miniapp/balance",
                "/billing/webhooks/yookassa",
                "--payment-webhook-only",
                "must use https",
                "/billing/payment-intents",
                "/admin/jobs",
                "/metrics",
                "VK /start",
                "YooKassa payment.succeeded",
                "artifact delivery"
            )
        }
    )

    foreach ($script in $scripts) {
        $fullPath = Join-Path $repoRoot $script.Path
        if (-not (Test-Path -LiteralPath $fullPath)) {
            throw "deploy script is missing: $($script.Path)"
        }
        $content = Get-Content -LiteralPath $fullPath -Raw
        foreach ($snippet in $script.Required) {
            if (-not $content.Contains($snippet)) {
                throw "deploy script $($script.Path) is missing required snippet: $snippet"
            }
        }
        if ($content -match "docker compose down -v|reset --hard|push --force|--force-with-lease") {
            throw "deploy script $($script.Path) contains a forbidden destructive operation"
        }
        if ($script.Path -match "rollback-prod" -and $content -match "(?m)(go\s+run\s+\./cmd/migrate\s+down|docker\s+compose[^\r\n]*migrate\s+down|Invoke-DockerCompose[^\r\n]*migrate\s+down)") {
            throw "rollback script $($script.Path) must not run migrate down automatically"
        }
    }

    Write-Host "deploy scripts OK"
}

function Assert-RollbackConfig {
    $composePath = Join-Path $repoRoot "docker-compose.prod.yml"
    if (-not (Test-Path -LiteralPath $composePath)) {
        Write-Host "no production compose file found; skipping rollback checks"
        return
    }

    $content = Get-Content -LiteralPath $composePath -Raw
    $requiredSnippets = @(
        'vk-ai-aggregator-api:${IMAGE_TAG:-prod}',
        'vk-ai-aggregator-worker:${IMAGE_TAG:-prod}',
        'vk-ai-aggregator-provider-webhook:${IMAGE_TAG:-prod}',
        'vk-ai-aggregator-miniapp:${IMAGE_TAG:-prod}',
        'vk-ai-aggregator-migrate:${IMAGE_TAG:-prod}',
        'vk-ai-aggregator-backup:${BACKUP_IMAGE_TAG:-prod}'
    )

    foreach ($snippet in $requiredSnippets) {
        if (-not $content.Contains($snippet)) {
            throw "production rollback image tag config is missing required snippet: $snippet"
        }
    }

    Write-Host "production rollback config OK"
}

function Assert-ObservabilityConfig {
    $composePath = Join-Path $repoRoot "docker-compose.observability.yml"
    $prodComposePath = Join-Path $repoRoot "docker-compose.prod.yml"
    $prometheusPath = Join-Path $repoRoot "observability\prometheus\prometheus.yml"
    $alertsPath = Join-Path $repoRoot "observability\prometheus\rules\product-alerts.yml"
    $observeScripts = @(
        "scripts\deploy\observe-prod.ps1",
        "scripts\deploy\observe-prod.sh"
    )

    foreach ($path in @($composePath, $prodComposePath, $prometheusPath, $alertsPath)) {
        if (-not (Test-Path -LiteralPath $path)) {
            throw "observability required file is missing: $path"
        }
    }

    foreach ($script in $observeScripts) {
        $fullPath = Join-Path $repoRoot $script
        if (-not (Test-Path -LiteralPath $fullPath)) {
            throw "production observe script is missing: $script"
        }
    }

    $prodCompose = Get-Content -LiteralPath $prodComposePath -Raw
    $observabilityCompose = Get-Content -LiteralPath $composePath -Raw
    $prometheus = Get-Content -LiteralPath $prometheusPath -Raw
    $alerts = Get-Content -LiteralPath $alertsPath -Raw

    $requiredProdComposeSnippets = @(
        'name: ${COMPOSE_NETWORK_NAME:-vk-ai-aggregator-prod}'
    )
    foreach ($snippet in $requiredProdComposeSnippets) {
        if (-not $prodCompose.Contains($snippet)) {
            throw "production compose observability network is missing snippet: $snippet"
        }
    }

    $requiredObservabilityComposeSnippets = @(
        "prometheus:",
        "grafana:",
        "loki:",
        "alertmanager:",
        "blackbox-exporter:",
        "postgres-exporter:",
        "redis-exporter:",
        "cadvisor:",
        "external: true",
        'name: ${COMPOSE_NETWORK_NAME:-vk-ai-aggregator-prod}'
    )
    foreach ($snippet in $requiredObservabilityComposeSnippets) {
        if (-not $observabilityCompose.Contains($snippet)) {
            throw "observability compose is missing required snippet: $snippet"
        }
    }

    $requiredPrometheusSnippets = @(
        "api:8080",
        "worker:9090",
        "provider-webhook:8082",
        "miniapp:80",
        "reverse-proxy/proxy-health",
        "payment_webhook_oldest_unprocessed_age_seconds",
        "vkagg_queue_oldest_age_seconds",
        "vkagg_dlq_routed_total",
        "blackbox-public-metrics"
    )
    foreach ($snippet in $requiredPrometheusSnippets) {
        if (-not $prometheus.Contains($snippet) -and -not $alerts.Contains($snippet)) {
            throw "Prometheus observability config/rules missing required snippet: $snippet"
        }
    }

    $requiredAlerts = @(
        "WorkerDown",
        "WorkerReadinessDegraded",
        "ProviderWebhookDown",
        "ProviderWebhookReadinessDegraded",
        "ApiReadinessDegraded",
        "ReverseProxyHealthDown",
        "PaymentWebhookBacklog",
        "QueueOldestAgeHigh",
        "DLQNotEmpty",
        "PublicMetricsExposed",
        "PostgresExporterDown",
        "RedisExporterDown"
    )
    foreach ($alert in $requiredAlerts) {
        if (-not $alerts.Contains($alert)) {
            throw "required observability alert is missing: $alert"
        }
    }

    Write-Host "production observability config OK"
}

function Invoke-Promtool {
    param(
        [Parameter(Mandatory = $true)][string[]]$Arguments
    )

    $promtool = Get-Command promtool -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($null -ne $promtool) {
        & $promtool.Source @Arguments
        return
    }

    $docker = Get-Command docker -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($null -eq $docker) {
        throw "promtool is not installed and docker is unavailable"
    }

    $previousErrorActionPreference = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    & $docker.Source info *> $null
    $dockerInfoExitCode = $LASTEXITCODE
    $ErrorActionPreference = $previousErrorActionPreference

    if ($dockerInfoExitCode -ne 0) {
        if ($env:CI -eq "true") {
            throw "promtool is not installed and docker daemon is unavailable in CI"
        }
        Write-Warning "promtool is not installed and docker daemon is unavailable; skipping local promtool check"
        $global:LASTEXITCODE = 0
        return
    }

    $promDir = (Resolve-Path (Join-Path $repoRoot "observability\prometheus")).Path.Replace("\", "/")
    $mount = "${promDir}:/etc/prometheus:ro"
    $prometheusImage = if ([string]::IsNullOrWhiteSpace($env:PROMETHEUS_IMAGE)) {
        "prom/prometheus:latest"
    } else {
        $env:PROMETHEUS_IMAGE
    }
    & $docker.Source run --rm -v $mount --entrypoint=promtool $prometheusImage @Arguments
}

function Assert-PrometheusConfig {
    if ($SkipPromtool) {
        Write-Host "promtool checks skipped by parameter"
        return
    }

    $promConfig = Join-Path $repoRoot "observability\prometheus\prometheus.yml"
    $rulesDir = Join-Path $repoRoot "observability\prometheus\rules"

    if (-not (Test-Path -LiteralPath $promConfig) -and -not (Test-Path -LiteralPath $rulesDir)) {
        Write-Host "no Prometheus config/rules found; skipping promtool"
        return
    }

    if (Test-Path -LiteralPath $promConfig) {
        Invoke-Step "promtool check config" {
            Invoke-Promtool -Arguments @("check", "config", "/etc/prometheus/prometheus.yml")
        }
    }

    if (Test-Path -LiteralPath $rulesDir) {
        $ruleFiles = @(Get-ChildItem -LiteralPath $rulesDir -File -Include "*.yml", "*.yaml" | Sort-Object Name)
        foreach ($ruleFile in $ruleFiles) {
            $containerPath = "/etc/prometheus/rules/$($ruleFile.Name)"
            Invoke-Step "promtool check rules $($ruleFile.Name)" {
                Invoke-Promtool -Arguments @("check", "rules", $containerPath)
            }
        }
    }
}

Invoke-Step "docker compose config" {
    docker compose --project-name vk-ai-aggregator -f docker-compose.yml config | Out-Null
}

if (Test-Path -LiteralPath "docker-compose.observability.yml") {
    Invoke-Step "docker compose observability config" {
        docker compose --project-name vk-ai-aggregator-observability -f docker-compose.observability.yml config | Out-Null
    }
}

if (Test-Path -LiteralPath "docker-compose.prod.yml") {
    Invoke-Step "docker compose prod config" {
        $previousAppEnvFile = $env:APP_ENV_FILE
        $prodEnvTemplate = if (Test-Path -LiteralPath ".env.prod.example") { ".env.prod.example" } else { ".env.example" }
        try {
            $env:APP_ENV_FILE = $prodEnvTemplate
            docker compose --project-name vk-ai-aggregator-prod --env-file $prodEnvTemplate -f docker-compose.prod.yml config | Out-Null
        } finally {
            if ($null -eq $previousAppEnvFile) {
                Remove-Item Env:\APP_ENV_FILE -ErrorAction SilentlyContinue
            } else {
                $env:APP_ENV_FILE = $previousAppEnvFile
            }
        }
    }
}

Assert-Migrations
Assert-NoTrackedEnvFiles
Assert-CloudflareConfigHasNoSecrets
Assert-CloudflareDeploymentConfig
Assert-ReverseProxyConfig
Assert-ProductionDataServices
Assert-DeployScripts
Assert-RollbackConfig
Assert-ObservabilityConfig
Assert-PrometheusConfig

Write-Host "infrastructure validation OK"
