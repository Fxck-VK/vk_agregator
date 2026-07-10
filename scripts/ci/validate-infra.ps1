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
    $bad = @(
        $tracked | Where-Object {
            $path = Join-Path $repoRoot $_
            $leaf = Split-Path $_ -Leaf
            (Test-Path -LiteralPath $path) -and
                ($leaf -eq ".env" -or $leaf -like ".env.*")
        }
    )

    if ($bad.Count -gt 0) {
        throw "tracked env files are forbidden: $($bad -join ', ')"
    }

    Write-Host "tracked env files OK"
}

function Assert-NoActiveEnvExampleReferences {
    $tracked = Get-TrackedFiles
    $activeFiles = @(
        $tracked | Where-Object {
            $_ -notmatch '^(docs/archive/|docs/superpowers/plans/)' -and
            $_ -notmatch '^\.agents/logs/' -and
            $_ -notin @(".gitignore", ".gitleaksignore")
        }
    )
    $bad = @()
    foreach ($file in $activeFiles) {
        if (-not (Test-Path -LiteralPath $file)) {
            continue
        }
        $content = Get-Content -LiteralPath $file -Raw -ErrorAction SilentlyContinue
        if ($null -ne $content -and $content -match '\.env\.(dev|prod|staging|loadtest)?\.?example') {
            $bad += $file
        }
    }
    if ($bad.Count -gt 0) {
        throw "active files still reference env example files: $($bad -join ', ')"
    }

    Write-Host "active env example references OK"
}

function New-ComposeValidationEnvFile {
    $path = Join-Path ([System.IO.Path]::GetTempPath()) ("vkagg-compose-validate-{0}.env" -f ([guid]::NewGuid().ToString("N")))
    $appEnvFile = $path.Replace("\", "/")
    $lines = @(
        "APP_ENV_FILE=$appEnvFile",
        "APP_ENV=production",
        "APP_IMAGE_REGISTRY=ghcr.io/fxck-vk/vk_agregator",
        "IMAGE_TAG=infra-validate",
        "BACKUP_IMAGE_TAG=infra-validate",
        "DATABASE_URL=postgres://vk_ai_aggregator:vk_ai_aggregator@postgres:5432/vk_ai_aggregator?sslmode=disable",
        "REDIS_ADDR=redis:6379",
        "S3_ENDPOINT=minio:9000",
        "S3_ACCESS_KEY=compose_validate_access",
        "S3_SECRET_KEY=compose_validate_secret",
        "S3_BUCKET=artifacts",
        "S3_USE_SSL=false",
        "S3_REGION=us-east-1",
        "S3_ADDRESSING_STYLE=path",
        "CLOUDFLARED_TUNNEL_TOKEN=compose-validate-token",
        "COMPOSE_NETWORK_NAME=vk-ai-aggregator-prod"
    )
    [IO.File]::WriteAllLines($path, $lines, [Text.UTF8Encoding]::new($false))
    return $path
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
        "vk.neiirohub.ru",
        "app.neiirohub.ru",
        "neiirohub.ru",
        "dev-vk.neiirohub.ru",
        "dev-app.neiirohub.ru",
        "dev.neiirohub.ru",
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

function Assert-DevReverseProxySmokeScript {
    $path = Join-Path $repoRoot "scripts\dev\check-dev-reverse-proxy.ps1"
    if (-not (Test-Path -LiteralPath $path)) {
        throw "DEV reverse proxy smoke script is missing: scripts/dev/check-dev-reverse-proxy.ps1"
    }

    $content = Get-Content -LiteralPath $path -Raw
    $requiredSnippets = @(
        "http://127.0.0.1:8088",
        "dev-vk.neiirohub.ru",
        "dev-app.neiirohub.ru",
        "dev.neiirohub.ru",
        "/health",
        "/miniapp/balance",
        "/billing/webhooks/yookassa",
        "/metrics",
        "/admin/jobs",
        "ForbiddenStatuses",
        "DEV reverse proxy smoke OK"
    )

    foreach ($snippet in $requiredSnippets) {
        if (-not $content.Contains($snippet)) {
            throw "DEV reverse proxy smoke script is missing required snippet: $snippet"
        }
    }

    Write-Host "DEV reverse proxy smoke script OK"
}

function Assert-DevStartStackScript {
    $path = Join-Path $repoRoot "scripts\dev\start-dev-stack.ps1"
    if (-not (Test-Path -LiteralPath $path)) {
        throw "DEV stack start script is missing: scripts/dev/start-dev-stack.ps1"
    }

    $content = Get-Content -LiteralPath $path -Raw
    $requiredSnippets = @(
        "WithCloudflare",
        "APP_ENV must be development/dev",
        "docker-compose.prod.yml",
        "start Postgres/Redis/MinIO",
        "run migrations",
        "api",
        "worker",
        "provider-webhook",
        "miniapp",
        "reverse-proxy",
        "cloudflared DEV tunnel",
        "DEV_ALLOW_REAL_AI_PROVIDERS",
        "DEV_ALLOW_REAL_PAYMENTS",
        "DEV_ALLOW_REMOTE_IMAGES",
        "DEV runtime mode: local-build from current working tree",
        "-SkipBuild would run prebuilt Docker images",
        "COMPOSE_BAKE",
        "VK_GROUP_ID must not be the production group id",
        "YOOKASSA_SECRET_KEY must be a YooKassa test key in DEV",
        "check-dev-reverse-proxy.ps1",
        "https://dev-vk.neiirohub.ru/health",
        "https://dev-app.neiirohub.ru/",
        "https://dev.neiirohub.ru/billing/webhooks/yookassa",
        "DEV stack is running."
    )

    foreach ($snippet in $requiredSnippets) {
        if (-not $content.Contains($snippet)) {
            throw "DEV stack start script is missing required snippet: $snippet"
        }
    }

    if ($content -match "docker compose down -v|reset --hard|push --force|--force-with-lease") {
        throw "DEV stack start script contains a forbidden destructive operation"
    }

    Write-Host "DEV stack start script OK"
}

function Assert-DevStopStatusScripts {
    $scripts = @(
        [pscustomobject]@{
            Path = "scripts\dev\stop-dev-stack.ps1"
            Required = @(
                "start-dev-stack.ps1",
                "StopOnly",
                "EnvFile",
                "ProjectName"
            )
        },
        [pscustomobject]@{
            Path = "scripts\dev\status-dev-stack.ps1"
            Required = @(
                "Test-TcpPort",
                "Invoke-RawHttp",
                "APP_ENV must be development/dev",
                "DEV_ALLOW_REAL_AI_PROVIDERS",
                "DEV_ALLOW_REAL_PAYMENTS",
                "VK_GROUP_ID must not be the production group id",
                "YOOKASSA_SECRET_KEY must be a YooKassa test key in DEV",
                "cloudflared.pid",
                "dev-vk.neiirohub.ru",
                "dev-app.neiirohub.ru",
                "dev.neiirohub.ru",
                "/webhooks/vk",
                "/miniapp/balance",
                "/billing/webhooks/yookassa",
                "VK callback",
                "Mini App",
                "Tunnel",
                "Public DEV smoke"
            )
        }
    )

    foreach ($script in $scripts) {
        $fullPath = Join-Path $repoRoot $script.Path
        if (-not (Test-Path -LiteralPath $fullPath)) {
            throw "DEV helper script is missing: $($script.Path)"
        }
        $content = Get-Content -LiteralPath $fullPath -Raw
        foreach ($snippet in $script.Required) {
            if (-not $content.Contains($snippet)) {
                throw "DEV helper script $($script.Path) is missing required snippet: $snippet"
            }
        }
        if ($content -match "docker compose down -v|reset --hard|push --force|--force-with-lease") {
            throw "DEV helper script $($script.Path) contains a forbidden destructive operation"
        }
    }

    Write-Host "DEV stop/status scripts OK"
}

function Assert-DevPublicSmokeScript {
    $path = Join-Path $repoRoot "scripts\dev\smoke-dev.ps1"
    if (-not (Test-Path -LiteralPath $path)) {
        throw "DEV public smoke script is missing: scripts/dev/smoke-dev.ps1"
    }

    $content = Get-Content -LiteralPath $path -Raw
    $requiredSnippets = @(
        "https://dev-vk.neiirohub.ru",
        "https://dev-app.neiirohub.ru",
        "https://dev.neiirohub.ru",
        "must use HTTPS",
        "/health",
        "/webhooks/vk",
        "/miniapp/balance",
        "/billing/webhooks/yookassa",
        "/admin/jobs",
        "/metrics",
        "ForbiddenStatuses",
        "DEV public smoke OK"
    )

    foreach ($snippet in $requiredSnippets) {
        if (-not $content.Contains($snippet)) {
            throw "DEV public smoke script is missing required snippet: $snippet"
        }
    }

    $forbiddenSnippets = @(
        "https://vk.neiirohub.ru",
        "https://app.neiirohub.ru",
        "https://neiirohub.ru/billing/webhooks/yookassa"
    )

    foreach ($snippet in $forbiddenSnippets) {
        if ($content.Contains($snippet)) {
            throw "DEV public smoke script must not target production URL: $snippet"
        }
    }

    if ($content -match "VK_ACCESS_TOKEN|VK_SECRET|YOOKASSA_SECRET|DEEPINFRA_API_KEY|OPENAI_API_KEY|CLOUDFLARED_TUNNEL_TOKEN") {
        throw "DEV public smoke script must not reference secrets"
    }

    Write-Host "DEV public smoke script OK"
}

function Assert-CloudflareDeploymentConfig {
    $path = Join-Path $repoRoot "deployments\cloudflare\cloudflared.prod.example.yml"
    $readmePath = Join-Path $repoRoot "deployments\cloudflare\README.md"
    if (-not (Test-Path -LiteralPath $path)) {
        Write-Host "no production cloudflared config example found; skipping"
        return
    }
    if (-not (Test-Path -LiteralPath $readmePath)) {
        throw "Cloudflare deployment README is missing: deployments/cloudflare/README.md"
    }

    $content = Get-Content -LiteralPath $path -Raw
    $readme = Get-Content -LiteralPath $readmePath -Raw
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

    $requiredReadmeSnippets = @(
        "vk.neiirohub.ru",
        "app.neiirohub.ru",
        "https://neiirohub.ru/billing/webhooks/yookassa",
        "dev-vk.neiirohub.ru",
        "dev-app.neiirohub.ru",
        "https://dev.neiirohub.ru/billing/webhooks/yookassa",
        "CLOUDFLARED_TUNNEL_TOKEN",
        "PUBLIC_PAYMENT_WEBHOOK_URL",
        'Do not route broad `/billing/*`'
    )

    foreach ($snippet in $requiredReadmeSnippets) {
        if (-not $readme.Contains($snippet)) {
            throw "Cloudflare deployment README is missing required snippet: $snippet"
        }
    }

    Write-Host "cloudflared production example OK"
}

function Assert-ProductionDataServices {
    $prodPath = Join-Path $repoRoot "docker-compose.prod.yml"
    $dataPath = Join-Path $repoRoot "docker-compose.data.yml"
    if (-not (Test-Path -LiteralPath $prodPath)) {
        Write-Host "no production compose file found; skipping data-service checks"
        return
    }
    if (-not (Test-Path -LiteralPath $dataPath)) {
        throw "production data compose file is missing: docker-compose.data.yml"
    }

    $requiredFiles = @(
        "docs/DATA_SERVICES_CONTRACT.md",
        "docker-compose.data.yml",
        "Dockerfile.migrate",
        "Dockerfile.backup",
        "scripts\backup\backup-postgres.sh",
        "scripts\backup\backup-minio.sh",
        "scripts\backup\restore-postgres.sh",
        "scripts\backup\restore-minio.sh",
        "scripts\deploy\check-migrations-safe.ps1",
        "scripts\deploy\check-migrations-safe.sh"
    )
    foreach ($requiredFile in $requiredFiles) {
        $fullPath = Join-Path $repoRoot $requiredFile
        if (-not (Test-Path -LiteralPath $fullPath)) {
            throw "production data-service support file is missing: $requiredFile"
        }
    }

    $prodContent = Get-Content -LiteralPath $prodPath -Raw
    $dataContent = Get-Content -LiteralPath $dataPath -Raw
    $requiredDataSnippets = @(
        "postgres:",
        "postgres_data:/var/lib/postgresql/data",
        "redis:",
        "redis_data:/data",
        "minio:",
        "minio_data:/data",
        "local-postgres-disabled",
        "local-minio-disabled",
        "postgres_data:",
        "redis_data:",
        "minio_data:",
        'name: ${COMPOSE_NETWORK_NAME:-vk-ai-aggregator-prod}'
    )
    $requiredProdSnippets = @(
        "migrate:",
        "Dockerfile.migrate",
        "backup-postgres:",
        "backup-minio:",
        "restore-postgres:",
        "restore-minio:",
        "Dockerfile.backup",
        "RESTORE_ALLOW_DESTRUCTIVE",
        "backup_data:/backups",
        "backup_metrics:/backup-metrics",
        "backup_data:",
        "backup_metrics:"
    )

    foreach ($snippet in $requiredDataSnippets) {
        if (-not $dataContent.Contains($snippet)) {
            throw "production data compose config is missing required snippet: $snippet"
        }
    }
    foreach ($snippet in $requiredProdSnippets) {
        if (-not $prodContent.Contains($snippet)) {
            throw "production app compose config is missing required snippet: $snippet"
        }
    }
    if ($prodContent -match "(?m)^\s{2}(postgres|redis|minio):\s*$") {
        throw "docker-compose.prod.yml must not define Postgres/Redis/MinIO services; use docker-compose.data.yml"
    }

    $modeAwareScripts = @(
        @{ Path = "scripts\deploy\deploy-prod.sh"; Snippet = "docker-compose.data.yml" },
        @{ Path = "scripts\deploy\deploy-prod.sh"; Snippet = "check_external_data_services" },
        @{ Path = "scripts\deploy\rollback-prod.sh"; Snippet = "docker-compose.data.yml" },
        @{ Path = "scripts\deploy\deploy-prod.ps1"; Snippet = "docker-compose.data.yml" },
        @{ Path = "scripts\deploy\deploy-prod.ps1"; Snippet = "Invoke-ExternalDataServiceChecks" },
        @{ Path = "scripts\deploy\rollback-prod.ps1"; Snippet = "docker-compose.data.yml" }
    )
    foreach ($script in $modeAwareScripts) {
        $scriptPath = Join-Path $repoRoot $script.Path
        $scriptContent = Get-Content -LiteralPath $scriptPath -Raw
        if (-not $scriptContent.Contains($script.Snippet)) {
            throw "production deploy script is not data-service-mode aware: $($script.Path)"
        }
    }

    Write-Host "production data services config OK"
}

function Assert-VersionDigestImageReference {
    param(
        [Parameter(Mandatory = $true)][string]$Reference,
        [Parameter(Mandatory = $true)][string]$Context
    )

    if ($Reference -match ':latest(?:@|$)' -or $Reference -notmatch '^[^@\s]+:[^@\s]+@sha256:[0-9a-f]{64}$') {
        throw "external container image must use a readable non-latest tag and sha256 digest: ${Context}: $Reference"
    }
}

function Assert-ExternalContainerPinsAndHelperSecrets {
    $dockerfiles = @(Get-ChildItem -LiteralPath $repoRoot -File -Filter "Dockerfile.*" | Sort-Object Name)
    if ($dockerfiles.Count -ne 7) {
        throw "expected seven application Dockerfiles, found $($dockerfiles.Count)"
    }

    $dockerfileImageCount = 0
    foreach ($dockerfile in $dockerfiles) {
        $lineNumber = 0
        foreach ($line in Get-Content -LiteralPath $dockerfile.FullName) {
            $lineNumber++
            if ($line -match '^\s*#\s*syntax=(?<image>\S+)\s*$') {
                Assert-VersionDigestImageReference -Reference $Matches.image -Context "$($dockerfile.Name):${lineNumber} syntax"
                $dockerfileImageCount++
                continue
            }
            if ($line -match '^\s*FROM\s+(?<image>\S+)') {
                Assert-VersionDigestImageReference -Reference $Matches.image -Context "$($dockerfile.Name):${lineNumber} FROM"
                $dockerfileImageCount++
            }
        }
    }
    if ($dockerfileImageCount -eq 0) {
        throw "Dockerfile pin validation found no external image inputs"
    }

    $composePaths = @(
        "docker-compose.prod.yml",
        "docker-compose.data.yml",
        "docker-compose.observability.yml"
    )
    $externalComposeCount = 0
    foreach ($relativePath in $composePaths) {
        $path = Join-Path $repoRoot $relativePath
        $lineNumber = 0
        foreach ($line in Get-Content -LiteralPath $path) {
            $lineNumber++
            if ($line -notmatch '^\s*image:\s*(?<image>\S+)\s*$') {
                continue
            }

            $image = $Matches.image
            if ($image.Contains('APP_IMAGE_REGISTRY')) {
                if ($image.Contains(':-main') -or ($image -notmatch '\$\{(?:IMAGE_TAG|BACKUP_IMAGE_TAG):\?')) {
                    throw "internal application image must require the immutable SHA tag without main fallback: ${relativePath}:${lineNumber}"
                }
                continue
            }

            Assert-VersionDigestImageReference -Reference $image -Context "${relativePath}:${lineNumber} image"
            $externalComposeCount++
        }
    }
    if ($externalComposeCount -eq 0) {
        throw "Compose pin validation found no external production images"
    }

    $productionCompose = Get-Content -LiteralPath (Join-Path $repoRoot "docker-compose.prod.yml") -Raw
    $internalProxyPattern = '(?m)^  reverse-proxy:\r?\n    image: \$\{APP_IMAGE_REGISTRY:-ghcr\.io/fxck-vk/vk_agregator\}/miniapp:\$\{IMAGE_TAG:\?IMAGE_TAG is required\}\s*$'
    if ($productionCompose -notmatch $internalProxyPattern) {
        throw "production reverse proxy must reuse the scanned internal miniapp SHA image"
    }

    $helperScripts = @(
        [pscustomobject]@{
            Path = "scripts\deploy\deploy-prod.sh"
            Required = @("POSTGRES_HELPER_IMAGE", "REDIS_HELPER_IMAGE", "S3_HELPER_IMAGE", "--env-file", "chmod 600", "cleanup_helper_env_files", "rm -f")
            ForbiddenPattern = '(?m)(docker run[^\r\n]*\s-e\s|^\s*-e\s+)'
        },
        [pscustomobject]@{
            Path = "scripts\deploy\deploy-prod.ps1"
            Required = @("PostgresHelperImage", "RedisHelperImage", "S3HelperImage", "--env-file", "SetUnixFileMode", "finally", "Remove-Item -LiteralPath")
            ForbiddenPattern = '(?m)(docker run[^\r\n]*\s-e\s|^\s*-e\s+)'
        }
    )
    foreach ($helperScript in $helperScripts) {
        $path = Join-Path $repoRoot $helperScript.Path
        $content = Get-Content -LiteralPath $path -Raw
        foreach ($snippet in $helperScript.Required) {
            if (-not $content.Contains($snippet)) {
                throw "deploy helper script $($helperScript.Path) is missing secure helper snippet: $snippet"
            }
        }
        if ($content -match $helperScript.ForbiddenPattern) {
            throw "deploy helper script $($helperScript.Path) passes helper environment values in docker CLI arguments"
        }
        foreach ($helperName in @("Postgres", "Redis", "S3")) {
            if (-not $content.Contains($helperName)) {
                throw "deploy helper script $($helperScript.Path) is missing $helperName helper path"
            }
        }
    }

    $dependabotPath = Join-Path $repoRoot ".github\dependabot.yml"
    $dependabot = Get-Content -LiteralPath $dependabotPath -Raw
    if (-not $dependabot.Contains('package-ecosystem: "docker"')) {
        throw "Dependabot config is missing Docker ecosystem updates"
    }

    Write-Host "external container pins OK: $dockerfileImageCount Dockerfile inputs and $externalComposeCount production Compose images; helper secrets use scoped env files"
}

function Assert-HelperEnvFileBehavior {
    $bashPath = Join-Path $repoRoot "scripts\deploy\deploy-prod.sh"
    $bashContent = Get-Content -LiteralPath $bashPath -Raw
    $bashStart = $bashContent.IndexOf("cleanup_helper_env_files() {")
    $bashEnd = $bashContent.IndexOf("check_docker() {")
    if ($bashStart -lt 0 -or $bashEnd -le $bashStart) {
        throw "could not isolate Bash helper env-file functions"
    }

    $bashFunctions = $bashContent.Substring($bashStart, $bashEnd - $bashStart)
    $bashTest = @'
set -euo pipefail
helper_env_files=()
__HELPER_FUNCTIONS__
helper_env_file=""
create_helper_env_file helper_env_file DATABASE_URL "sentinel-db" REDISCLI_AUTH "sentinel-redis"
[[ -n "$helper_env_file" && -f "$helper_env_file" ]]
mode="$(stat -f '%Lp' "$helper_env_file" 2>/dev/null || stat -c '%a' "$helper_env_file")"
[[ "$mode" == "600" ]]
expected=$'DATABASE_URL=sentinel-db\nREDISCLI_AUTH=sentinel-redis'
[[ "$(<"$helper_env_file")" == "$expected" ]]
remove_helper_env_file "$helper_env_file"
[[ ! -e "$helper_env_file" ]]
invalid_file=""
if create_helper_env_file invalid_file TOKEN $'bad\nvalue' 2>/dev/null; then
  exit 1
fi
'@.Replace("__HELPER_FUNCTIONS__", $bashFunctions)

    $bashTempDir = Join-Path ([System.IO.Path]::GetTempPath()) "vkagg-helper-test-$([guid]::NewGuid().ToString('N'))"
    New-Item -ItemType Directory -Path $bashTempDir | Out-Null
    $previousTmpDir = $env:TMPDIR
    try {
        $env:TMPDIR = $bashTempDir
        & bash -c $bashTest
        if ($LASTEXITCODE -ne 0) {
            throw "Bash helper env-file behavior test failed with exit code $LASTEXITCODE"
        }
        if (@(Get-ChildItem -LiteralPath $bashTempDir -Force).Count -ne 0) {
            throw "Bash helper env-file behavior test left temporary files behind"
        }
    } finally {
        if ($null -eq $previousTmpDir) {
            Remove-Item Env:\TMPDIR -ErrorAction SilentlyContinue
        } else {
            $env:TMPDIR = $previousTmpDir
        }
        Remove-Item -LiteralPath $bashTempDir -Recurse -Force -ErrorAction SilentlyContinue
    }

    $powerShellPath = Join-Path $repoRoot "scripts\deploy\deploy-prod.ps1"
    $tokens = $null
    $parseErrors = $null
    $ast = [System.Management.Automation.Language.Parser]::ParseFile($powerShellPath, [ref]$tokens, [ref]$parseErrors)
    if ($parseErrors.Count -gt 0) {
        throw "PowerShell deploy script has parser errors"
    }
    $helperFunction = $ast.Find({
        param($node)
        $node -is [System.Management.Automation.Language.FunctionDefinitionAst] -and $node.Name -eq "New-HelperEnvFile"
    }, $true)
    if ($null -eq $helperFunction) {
        throw "could not isolate PowerShell helper env-file function"
    }
    . ([scriptblock]::Create($helperFunction.Extent.Text))

    $powerShellEnvFile = New-HelperEnvFile -Values @{
        S3_ACCESS_KEY = "sentinel-access"
        S3_SECRET_KEY = "sentinel-secret"
    }
    try {
        if (-not (Test-Path -LiteralPath $powerShellEnvFile -PathType Leaf)) {
            throw "PowerShell helper env-file was not created"
        }
        if ($IsWindows) {
            $identity = [System.Security.Principal.WindowsIdentity]::GetCurrent().Name
            $unexpectedRules = @(
                (Get-Acl -LiteralPath $powerShellEnvFile).Access |
                    Where-Object {
                        $_.AccessControlType -eq [System.Security.AccessControl.AccessControlType]::Allow -and
                        $_.IdentityReference.Value -ne $identity
                    }
            )
            if ($unexpectedRules.Count -ne 0) {
                throw "PowerShell helper env-file grants access beyond the current Windows identity"
            }
        } else {
            $expectedMode = [System.IO.UnixFileMode]::UserRead -bor [System.IO.UnixFileMode]::UserWrite
            if ([System.IO.File]::GetUnixFileMode($powerShellEnvFile) -ne $expectedMode) {
                throw "PowerShell helper env-file mode is not 0600"
            }
        }
        $actualLines = @(Get-Content -LiteralPath $powerShellEnvFile)
        $expectedLines = @("S3_ACCESS_KEY=sentinel-access", "S3_SECRET_KEY=sentinel-secret")
        if (($actualLines -join "`n") -ne ($expectedLines -join "`n")) {
            throw "PowerShell helper env-file content is invalid"
        }
    } finally {
        Remove-Item -LiteralPath $powerShellEnvFile -Force -ErrorAction SilentlyContinue
    }

    $rejected = $false
    try {
        $unexpectedFile = New-HelperEnvFile -Values @{ TOKEN = "bad`nvalue" }
        Remove-Item -LiteralPath $unexpectedFile -Force -ErrorAction SilentlyContinue
    } catch {
        $rejected = $true
    }
    if (-not $rejected) {
        throw "PowerShell helper env-file accepted a newline-bearing value"
    }

    Write-Host "deploy helper env-file behavior OK: Bash and PowerShell mode, content, injection rejection, cleanup"
}

function Assert-CloudflaredComposeConfig {
    $path = Join-Path $repoRoot "docker-compose.prod.yml"
    if (-not (Test-Path -LiteralPath $path)) {
        Write-Host "no production compose file found; skipping cloudflared compose checks"
        return
    }

    $content = Get-Content -LiteralPath $path -Raw
    $requiredSnippets = @(
        "cloudflared:",
        "profiles:",
        "- cloudflare",
        "TUNNEL_TOKEN:",
        "CLOUDFLARED_TUNNEL_TOKEN",
        "network_mode: host",
        "--metrics",
        '127.0.0.1:${CLOUDFLARED_METRICS_PORT:-2000}'
    )

    foreach ($snippet in $requiredSnippets) {
        if (-not $content.Contains($snippet)) {
            throw "production cloudflared compose config is missing required snippet: $snippet"
        }
    }

    if ($content -match "(?m)^\s*-\s*--token\s*$") {
        throw "cloudflared compose config must use TUNNEL_TOKEN env instead of command-line --token"
    }

    if ($content -match [regex]::Escape('127.0.0.1:${CLOUDFLARED_METRICS_PORT:-2000}:2000')) {
        throw "cloudflared metrics must not publish a Docker port when host networking is enabled"
    }

    Write-Host "production cloudflared compose config OK"
}

function Assert-DeployScripts {
    $scripts = @(
        [pscustomobject]@{
            Path = "scripts\deploy\deploy-prod.ps1"
            Required = @(
                "check Docker",
                "docker info",
                "check-prod-env.ps1",
                "git pull --ff-only origin",
                "docker-compose.prod.yml",
                "docker compose pull",
                "BuildOnVPS",
                "--no-build",
                "SkipPublicSmoke",
                "Invoke-ExternalDataServiceChecks",
                "PostgresHelperImage",
                "RedisHelperImage",
                "S3HelperImage",
                "smoke-prod.ps1",
                "check-migrations-safe.ps1",
                "MIGRATION_BACKUP_CONFIRMED",
                "backup postgres before migration",
                "-EnvFile",
                "PUBLIC_PAYMENT_WEBHOOK_URL",
                "IMAGE_TAG",
                "migrateArgs",
                "exit-code-from",
                "api", "worker", "provider-webhook", "miniapp", "reverse-proxy",
                "provider-balance",
                "provider-balance-bot",
                "Provider balance bot:",
                "Wait-Http",
                "/readyz",
                "Production deploy completed.",
                "skipped; pulled registry images",
                "Health checks:"
            )
        },
        [pscustomobject]@{
            Path = "scripts\deploy\deploy-prod.sh"
            Required = @(
                "check Docker",
                "docker info",
                "check-prod-env.sh",
                "git pull --ff-only origin",
                "docker-compose.prod.yml",
                "image_pull_services",
                "--build-on-vps",
                "--no-build",
                "--skip-public-smoke",
                "check_external_data_services",
                "POSTGRES_HELPER_IMAGE",
                "REDIS_HELPER_IMAGE",
                "S3_HELPER_IMAGE",
                "smoke-prod.sh",
                "check-migrations-safe.sh",
                "MIGRATION_BACKUP_CONFIRMED",
                "Migration backup:",
                "--env-file",
                "PUBLIC_PAYMENT_WEBHOOK_URL",
                "IMAGE_TAG",
                "migrate_args",
                "exit-code-from",
                "api worker maintenance-worker provider-webhook miniapp reverse-proxy",
                "provider-balance",
                "provider-balance-bot",
                "Provider balance bot:",
                "wait_http",
                "/readyz",
                "Production deploy completed.",
                "skipped; pulled registry images",
                "Health checks:"
            )
        },
        [pscustomobject]@{
            Path = "scripts\deploy\check-prod-env.ps1"
            Required = @(
                "APP_IMAGE_REGISTRY",
                "IMAGE_TAG",
                "APP_ENV",
                "DATA_SERVICES_MODE",
                "POSTGRES_MODE",
                "REDIS_MODE",
                "S3_MODE",
                "S3_ENDPOINT",
                "S3_ADDRESSING_STYLE",
                "REDIS_ADDR",
                "staging",
                "CHANGE_ME",
                "PAYMENT_PROVIDER",
                "ARTIFACT_SCANNER",
                "CLOUDFLARED_TUNNEL_TOKEN",
                "PUBLIC_VK_BASE_URL",
                "PUBLIC_APP_BASE_URL",
                "PUBLIC_PAYMENT_WEBHOOK_URL",
                "MIGRATION_ALLOW_DESTRUCTIVE",
                "MIGRATION_BACKUP_CONFIRMED",
                "RESTORE_ALLOW_DESTRUCTIVE",
                "PROVIDER_BALANCE_BOT_ENABLED",
                "TELEGRAM_ADMIN_CHAT_ID",
                "APIMART_API_KEY"
            )
        },
        [pscustomobject]@{
            Path = "scripts\deploy\check-prod-env.sh"
            Required = @(
                "APP_IMAGE_REGISTRY",
                "IMAGE_TAG",
                "APP_ENV",
                "DATA_SERVICES_MODE",
                "POSTGRES_MODE",
                "REDIS_MODE",
                "S3_MODE",
                "S3_ENDPOINT",
                "S3_ADDRESSING_STYLE",
                "REDIS_ADDR",
                "staging",
                "CHANGE_ME",
                "PAYMENT_PROVIDER",
                "ARTIFACT_SCANNER",
                "CLOUDFLARED_TUNNEL_TOKEN",
                "PUBLIC_VK_BASE_URL",
                "PUBLIC_APP_BASE_URL",
                "PUBLIC_PAYMENT_WEBHOOK_URL",
                "MIGRATION_ALLOW_DESTRUCTIVE",
                "MIGRATION_BACKUP_CONFIRMED",
                "RESTORE_ALLOW_DESTRUCTIVE",
                "PROVIDER_BALANCE_BOT_ENABLED",
                "TELEGRAM_ADMIN_CHAT_ID",
                "APIMART_API_KEY"
            )
        },
        [pscustomobject]@{
            Path = "scripts\deploy\rollback-prod.ps1"
            Required = @(
                "ImageTag",
                "check-prod-env.ps1",
                "check Docker",
                "docker info",
                "pull backup images",
                "pull rollback images",
                "backup postgres before rollback",
                "backup minio before rollback",
                "does not run migrate down",
                "up -d --no-build --no-deps",
                "provider-balance-bot",
                "Wait-Http"
            )
        },
        [pscustomobject]@{
            Path = "scripts\deploy\rollback-prod.sh"
            Required = @(
                "--image-tag",
                "check-prod-env.sh",
                "check_docker",
                "docker info",
                "pull backup-postgres backup-minio",
                'pull "${rollback_services[@]}"',
                "backup-postgres",
                "backup-minio",
                "does not run migrate down",
                "up -d --no-build --no-deps",
                "provider-balance-bot",
                "wait_http"
            )
        },
        [pscustomobject]@{
            Path = "scripts\deploy\smoke-prod.ps1"
            Required = @(
                "EnvFile",
                "/health",
                "/readyz",
                "/miniapp/balance",
                "/billing/webhooks/yookassa",
                "PaymentWebhookOnly",
                "SkipLocalHealth",
                "PUBLIC_PAYMENT_WEBHOOK_URL",
                "WORKER_METRICS_ADDR",
                "PAYMENT_WEBHOOK_ADDR",
                "REVERSE_PROXY_HTTP_PORT",
                "must use https",
                "VK webhook route",
                "Worker local health",
                "Provider webhook local health",
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
                "--env-file",
                "/health",
                "/readyz",
                "/miniapp/balance",
                "/billing/webhooks/yookassa",
                "--payment-webhook-only",
                "--skip-local-health",
                "PUBLIC_PAYMENT_WEBHOOK_URL",
                "WORKER_METRICS_ADDR",
                "PAYMENT_WEBHOOK_ADDR",
                "REVERSE_PROXY_HTTP_PORT",
                "must use https",
                "VK webhook route",
                "Worker local health",
                "Provider webhook local health",
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

function Assert-CIWorkflowCoverage {
    $path = Join-Path $repoRoot ".github\workflows\ci.yml"
    if (-not (Test-Path -LiteralPath $path)) {
        throw "CI workflow is missing: .github/workflows/ci.yml"
    }

    $content = Get-Content -LiteralPath $path -Raw
    $permissionsIndex = $content.IndexOf("permissions:", [StringComparison]::Ordinal)
    if ($permissionsIndex -lt 0) {
        throw "CI workflow permissions block is missing"
    }
    $triggerSection = $content.Substring(0, $permissionsIndex)

    foreach ($branch in @("main", "dev-deploy", "fastlife_dev")) {
        $pattern = "(?m)^\s+-\s+$([regex]::Escape($branch))\s*$"
        $triggerCount = [regex]::Matches($triggerSection, $pattern).Count
        if ($triggerCount -lt 2) {
            throw "CI workflow must cover both pull_request and push for branch: $branch"
        }
    }

    foreach ($jobName in @("Backend", "Secret Scan", "Mini App", "Infrastructure")) {
        if (-not $content.Contains("name: $jobName")) {
            throw "CI workflow is missing required quality job: $jobName"
        }
    }
    if ($content.Contains("continue-on-error:") -or $content.Contains('secrets.')) {
        throw "CI quality jobs must fail closed and must not consume repository secrets in pull_request context"
    }
    if ($content.Contains("pull_request_target:") -or $content.Contains("write-all") -or $content.Contains("contents: write")) {
        throw "CI workflow must not grant broad write access or use pull_request_target"
    }

    Write-Host "CI workflow coverage OK: main, dev-deploy, fastlife_dev; quality failures block and PR jobs are secret-free"
}

function Assert-GitHubActionPins {
    $workflowDir = Join-Path $repoRoot ".github\workflows"
    if (-not (Test-Path -LiteralPath $workflowDir)) {
        throw "GitHub workflow directory is missing: .github/workflows"
    }

    $workflowFiles = @(Get-ChildItem -LiteralPath $workflowDir -File -Include "*.yml", "*.yaml" | Sort-Object Name)
    if ($workflowFiles.Count -eq 0) {
        throw "no GitHub workflow files found for action pin validation"
    }

    $externalCount = 0
    $actions = [System.Collections.Generic.HashSet[string]]::new([StringComparer]::Ordinal)
    foreach ($workflowFile in $workflowFiles) {
        $lineNumber = 0
        foreach ($line in Get-Content -LiteralPath $workflowFile.FullName) {
            $lineNumber++
            if ($line -notmatch '^\s*uses:\s*(?<value>[^\s#]+)(?:\s+#.*)?$') {
                continue
            }

            $value = $Matches.value
            if ($value.StartsWith("./", [StringComparison]::Ordinal)) {
                continue
            }
            if ($value -notmatch '^(?<action>[^@]+)@(?<ref>[0-9a-fA-F]{40})$') {
                throw "external GitHub Action must use a full 40-character commit SHA: $($workflowFile.Name):${lineNumber}: $value"
            }

            $externalCount++
            $null = $actions.Add($Matches.action)
        }
    }

    if ($externalCount -eq 0 -or $actions.Count -eq 0) {
        throw "GitHub Action pin validation found no external uses entries"
    }

    $dependabotPath = Join-Path $repoRoot ".github\dependabot.yml"
    if (-not (Test-Path -LiteralPath $dependabotPath)) {
        throw "GitHub Actions Dependabot config is missing: .github/dependabot.yml"
    }
    $dependabot = Get-Content -LiteralPath $dependabotPath -Raw
    foreach ($snippet in @('package-ecosystem: "github-actions"', 'directory: "/"')) {
        if (-not $dependabot.Contains($snippet)) {
            throw "GitHub Actions Dependabot config is missing required snippet: $snippet"
        }
    }

    Write-Host "GitHub Action pins OK: $externalCount external uses across $($actions.Count) repositories; all refs are full commit SHAs"
}

function Assert-DockerImageWorkflow {
    $path = Join-Path $repoRoot ".github\workflows\docker-images.yml"
    if (-not (Test-Path -LiteralPath $path)) {
        throw "Docker image build workflow is missing: .github/workflows/docker-images.yml"
    }

    $content = Get-Content -LiteralPath $path -Raw
    $requiredSnippets = @(
        "name: Docker Images",
        "actions: read",
        "packages: write",
        "ghcr.io/",
        "CI_WORKFLOW: ci.yml",
        'commit_sha: ${{ steps.source.outputs.commit_sha }}',
        'ref: ${{ needs.validate_source.outputs.commit_sha }}',
        'value=sha-${{ needs.validate_source.outputs.commit_sha }}',
        'head_sha=${SOURCE_SHA}',
        'actions/workflows/${CI_WORKFLOW}/runs',
        "event=push",
        "workflow_dispatch)",
        '.head_sha == $sha',
        '.conclusion == "success"',
        "failed_runs",
        "active_runs",
        "Manual Docker Images dispatch requires prior successful CI",
        "needs: validate_source",
        "push: true",
        "^[0-9a-f]{40}$",
        "docker/setup-buildx-action",
        "docker/login-action",
        "docker/metadata-action",
        "docker/build-push-action",
        "Dockerfile.api",
        "Dockerfile.worker",
        "Dockerfile.provider-webhook",
        "Dockerfile.provider-balance-bot",
        "Dockerfile.miniapp",
        "Dockerfile.migrate",
        "service: api",
        "service: worker",
        "service: provider-webhook",
        "service: provider-balance-bot",
        "service: miniapp",
        "service: migrate"
    )

    foreach ($snippet in $requiredSnippets) {
        if (-not $content.Contains($snippet)) {
            throw "Docker image workflow is missing required snippet: $snippet"
        }
    }

    $permissionsIndex = $content.IndexOf("permissions:", [StringComparison]::Ordinal)
    $buildIndex = $content.IndexOf("`n  build:", [StringComparison]::Ordinal)
    $packagesWriteIndex = $content.IndexOf("packages: write", [StringComparison]::Ordinal)
    if ($permissionsIndex -lt 0 -or $buildIndex -lt 0 -or $packagesWriteIndex -lt $buildIndex) {
        throw "Docker image packages:write permission must be scoped to the gated build job"
    }
    $triggerSection = $content.Substring(0, $permissionsIndex)
    if ($triggerSection.Contains("pull_request:") -or $triggerSection.Contains("pull_request_target:")) {
        throw "Docker image publish workflow must not run in pull request context"
    }
    if ($content.Contains("format=short") -or $content.Contains("deploy_short_sha")) {
        throw "Docker image workflow must not use short SHA as a release identity"
    }
    if ($content.Contains("write-all") -or $content.Contains("contents: write") -or $content.Contains("id-token: write")) {
        throw "Docker image workflow contains an unnecessary broad write permission"
    }

    $validateIndex = $content.IndexOf("`n  validate_source:", [StringComparison]::Ordinal)
    $validateSection = $content.Substring($validateIndex, $buildIndex - $validateIndex)
    if ($validateSection.Contains("actions/checkout") -or $validateSection.Contains('secrets.') -or $validateSection.Contains("packages: write")) {
        throw "Docker image validation must run without checkout, repository secrets, or package write access"
    }

    Write-Host "Docker image workflow exact-SHA CI gate OK: Backend/Secret Scan failure and PR/manual bypass paths fail closed"
}

function Assert-DeployWorkflowTrustChain {
    param(
        [Parameter(Mandatory = $true)][string]$Path,
        [Parameter(Mandatory = $true)][string]$TargetBranch,
        [Parameter(Mandatory = $true)][string]$EnvironmentName,
        [Parameter(Mandatory = $true)][ValidateSet("push", "workflow_run")][string]$AutomaticEvent
    )

    $fullPath = Join-Path $repoRoot $Path
    if (-not (Test-Path -LiteralPath $fullPath)) {
        throw "deploy workflow is missing: $Path"
    }

    $content = Get-Content -LiteralPath $fullPath -Raw
    $requiredSnippets = @(
        "validate_source:",
        "actions: read",
        "CI_WORKFLOW: ci.yml",
        "IMAGE_WORKFLOW: docker-images.yml",
        'commit_sha: ${{ steps.source.outputs.commit_sha }}',
        'ref: ${{ needs.validate_source.outputs.commit_sha }}',
        'head_sha=${SOURCE_SHA}',
        'actions/workflows/${CI_WORKFLOW}/runs',
        'actions/workflows/${IMAGE_WORKFLOW}/runs',
        "workflow_dispatch)",
        '.head_sha == $sha',
        '.conclusion == "success"',
        "ci_success",
        "image_success",
        "refs/heads/$TargetBranch",
        "environment: $EnvironmentName",
        "needs: validate_source",
        "sha-`${deploy_sha}",
        "^sha-[0-9a-f]{40}$",
        "^[0-9a-f]{40}$"
    )
    foreach ($snippet in $requiredSnippets) {
        if (-not $content.Contains($snippet)) {
            throw "deploy workflow $Path is missing exact-SHA trust snippet: $snippet"
        }
    }

    if ($AutomaticEvent -eq "workflow_run") {
        foreach ($snippet in @(
            "workflow_run:",
            'WORKFLOW_RUN_ID: ${{ github.event.workflow_run.id }}',
            'actions/runs/${WORKFLOW_RUN_ID}'
        )) {
            if (-not $content.Contains($snippet)) {
                throw "deploy workflow $Path is missing workflow_run trust snippet: $snippet"
            }
        }
    } else {
        foreach ($snippet in @(
            "push:",
            'SOURCE_SHA="${GITHUB_SHA}"',
            "SOURCE_BRANCH=`"$TargetBranch`""
        )) {
            if (-not $content.Contains($snippet)) {
                throw "deploy workflow $Path is missing protected push trust snippet: $snippet"
            }
        }
    }

    $permissionsIndex = $content.IndexOf("permissions:", [StringComparison]::Ordinal)
    if ($permissionsIndex -lt 0) {
        throw "deploy workflow $Path permissions block is missing"
    }
    $triggerSection = $content.Substring(0, $permissionsIndex)
    if ($triggerSection.Contains("pull_request:") -or $triggerSection.Contains("pull_request_target:")) {
        throw "deploy workflow $Path must not run in pull request context"
    }
    if ($content.Contains("write-all") -or $content.Contains("contents: write") -or $content.Contains("packages: write") -or $content.Contains("id-token: write")) {
        throw "deploy workflow $Path contains an unnecessary write permission"
    }

    $validateIndex = $content.IndexOf("`n  validate_source:", [StringComparison]::Ordinal)
    $deployIndex = $content.IndexOf("`n  deploy:", [StringComparison]::Ordinal)
    $firstSecretIndex = $content.IndexOf('secrets.', [StringComparison]::Ordinal)
    if ($validateIndex -lt 0 -or $deployIndex -lt 0 -or $deployIndex -le $validateIndex) {
        throw "deploy workflow $Path must gate deploy behind validate_source"
    }
    if ($firstSecretIndex -ge 0 -and $firstSecretIndex -lt $deployIndex) {
        throw "deploy workflow $Path exposes secrets before exact-SHA validation"
    }

    $validateSection = $content.Substring($validateIndex, $deployIndex - $validateIndex)
    if ($validateSection.Contains('secrets.')) {
        throw "deploy workflow $Path validate_source must not access repository secrets"
    }
    if ($content.Contains("deploy_short_sha") -or $content.Contains("git_short") -or $content.Contains("--short=7")) {
        throw "deploy workflow $Path must not use short SHA as release identity"
    }

    Write-Host "deploy workflow trust OK: $Path -> $AutomaticEvent/$TargetBranch -> $EnvironmentName -> exact CI/image SHA before secrets"
}

function Assert-RollbackConfig {
    $composePath = Join-Path $repoRoot "docker-compose.prod.yml"
    if (-not (Test-Path -LiteralPath $composePath)) {
        Write-Host "no production compose file found; skipping rollback checks"
        return
    }

    $content = Get-Content -LiteralPath $composePath -Raw
    $requiredSnippets = @(
        '${APP_IMAGE_REGISTRY:-ghcr.io/fxck-vk/vk_agregator}/api:${IMAGE_TAG:?IMAGE_TAG is required}',
        '${APP_IMAGE_REGISTRY:-ghcr.io/fxck-vk/vk_agregator}/worker:${IMAGE_TAG:?IMAGE_TAG is required}',
        '${APP_IMAGE_REGISTRY:-ghcr.io/fxck-vk/vk_agregator}/provider-webhook:${IMAGE_TAG:?IMAGE_TAG is required}',
        '${APP_IMAGE_REGISTRY:-ghcr.io/fxck-vk/vk_agregator}/provider-balance-bot:${IMAGE_TAG:?IMAGE_TAG is required}',
        '${APP_IMAGE_REGISTRY:-ghcr.io/fxck-vk/vk_agregator}/miniapp:${IMAGE_TAG:?IMAGE_TAG is required}',
        '${APP_IMAGE_REGISTRY:-ghcr.io/fxck-vk/vk_agregator}/migrate:${IMAGE_TAG:?IMAGE_TAG is required}',
        '${APP_IMAGE_REGISTRY:-ghcr.io/fxck-vk/vk_agregator}/backup:${BACKUP_IMAGE_TAG:?BACKUP_IMAGE_TAG is required}'
    )

    foreach ($snippet in $requiredSnippets) {
        if (-not $content.Contains($snippet)) {
            throw "production rollback image tag config is missing required snippet: $snippet"
        }
    }

    Write-Host "production rollback config OK"
}

function Get-ComposeServiceBlock {
    param(
        [Parameter(Mandatory = $true)][string]$Content,
        [Parameter(Mandatory = $true)][string]$Service
    )

    $pattern = "(?ms)^  $([regex]::Escape($Service)):\r?\n(?<block>.*?)(?=^  [A-Za-z0-9][A-Za-z0-9_-]*:\r?\n|\z)"
    $match = [regex]::Match($Content, $pattern)
    if (-not $match.Success) {
        throw "compose service is missing: $Service"
    }
    return $match.Value
}

function Assert-HardenedServiceBlock {
    param(
        [Parameter(Mandatory = $true)][string]$Block,
        [Parameter(Mandatory = $true)][string]$Service,
        [switch]$RequireHealthcheck,
        [switch]$RequireTmpfs
    )

    $required = @(
        "read_only: true",
        "no-new-privileges:true",
        "cap_drop:",
        "- ALL",
        "pids_limit:",
        "mem_limit:",
        "cpus:"
    )
    if ($RequireHealthcheck) {
        $required += "healthcheck:"
    }
    if ($RequireTmpfs) {
        $required += "tmpfs:"
    }
    foreach ($snippet in $required) {
        if (-not $Block.Contains($snippet)) {
            throw "service $Service is missing hardening control: $snippet"
        }
    }
    if ($Block.Contains("privileged: true")) {
        throw "service $Service must not be privileged"
    }
}

function Assert-ContainerPrivilegeHardening {
    $observabilityPath = Join-Path $repoRoot "docker-compose.observability.yml"
    $productionPath = Join-Path $repoRoot "docker-compose.prod.yml"
    $alloyPath = Join-Path $repoRoot "observability\alloy\config.alloy"
    $socketProxyConfigPath = Join-Path $repoRoot "observability\docker-socket-proxy\haproxy.cfg"
    $alloyVexPath = Join-Path $repoRoot "security\trivy\alloy.openvex.json"
    $backupDockerfilePath = Join-Path $repoRoot "Dockerfile.backup"
    $miniappDockerfilePath = Join-Path $repoRoot "Dockerfile.miniapp"

    if (-not (Test-Path -LiteralPath $socketProxyConfigPath)) {
        throw "docker-socket-proxy deny-by-default HAProxy config is missing"
    }
    if (-not (Test-Path -LiteralPath $alloyVexPath)) {
        throw "Alloy Docker client OpenVEX is missing"
    }

    $observability = Get-Content -LiteralPath $observabilityPath -Raw
    $production = Get-Content -LiteralPath $productionPath -Raw
    $alloy = Get-Content -LiteralPath $alloyPath -Raw
    $socketProxyConfig = Get-Content -LiteralPath $socketProxyConfigPath -Raw
    $alloyVex = Get-Content -LiteralPath $alloyVexPath -Raw | ConvertFrom-Json -Depth 20
    $backupDockerfile = Get-Content -LiteralPath $backupDockerfilePath -Raw
    $miniappDockerfile = Get-Content -LiteralPath $miniappDockerfilePath -Raw

    $socketProxy = Get-ComposeServiceBlock -Content $observability -Service "docker-socket-proxy"
    if (-not $socketProxy.Contains("/var/run/docker.sock:/var/run/docker.sock:ro")) {
        throw "docker-socket-proxy must be the only service mounting docker.sock"
    }
    $observabilityWithoutProxy = $observability.Replace($socketProxy, "")
    if ($observabilityWithoutProxy.Contains("/var/run/docker.sock") -or $alloy.Contains("unix:///var/run/docker.sock")) {
        throw "observability services must not access docker.sock directly"
    }

    foreach ($snippet in @(
        'image: haproxy:3.2.21-alpine@sha256:66e25cc9a8332635f4e897f7f4b1e5622c25f09f0ee23cddc6ce9bdb3a24772a',
        './observability/docker-socket-proxy/haproxy.cfg:/usr/local/etc/haproxy/haproxy.cfg:ro'
    )) {
        if (-not $socketProxy.Contains($snippet)) {
            throw "docker-socket-proxy is missing pinned proxy control: $snippet"
        }
    }

    foreach ($snippet in @(
        'acl read_method method GET HEAD',
        'acl allowed_path path_reg',
        '/networks$',
        'maxconn 256',
        'http-request deny deny_status 403 unless read_method allowed_path',
        'server docker unix@/var/run/docker.sock'
    )) {
        if (-not $socketProxyConfig.Contains($snippet)) {
            throw "docker-socket-proxy HAProxy policy is missing deny-by-default control: $snippet"
        }
    }
    foreach ($forbiddenPath in @("/auth", "/build", "/commit", "/configs", "/exec", "/images", "/plugins", "/secrets", "/services", "/swarm", "/tasks", "/volumes")) {
        if ($socketProxyConfig.Contains($forbiddenPath)) {
            throw "docker-socket-proxy HAProxy allowlist includes forbidden Docker API path: $forbiddenPath"
        }
    }

    $expectedAlloyCves = @("CVE-2026-34040", "CVE-2026-41567", "CVE-2026-42306")
    $alloyVexStatements = @($alloyVex.statements)
    if ($alloyVex.'@context' -ne "https://openvex.dev/ns/v0.2.0" -or $alloyVexStatements.Count -ne $expectedAlloyCves.Count) {
        throw "Alloy OpenVEX must use OpenVEX 0.2 and contain exactly the reviewed Docker client findings"
    }
    foreach ($cve in $expectedAlloyCves) {
        $statement = @($alloyVexStatements | Where-Object { $_.vulnerability.name -eq $cve })
        if ($statement.Count -ne 1) {
            throw "Alloy OpenVEX must contain exactly one statement for $cve"
        }
        $product = @($statement[0].products)[0]
        $subcomponent = @($product.subcomponents)[0]
        if ($statement[0].status -ne "not_affected" -or
            $statement[0].justification -ne "vulnerable_code_not_in_execute_path" -or
            $product.'@id' -ne "pkg:golang/github.com/grafana/alloy/otel_engine" -or
            $subcomponent.'@id' -ne "pkg:golang/github.com/docker/docker@v28.5.2%2Bincompatible" -or
            -not $statement[0].impact_statement.Contains("Docker daemon") -or
            -not $statement[0].impact_statement.Contains("HAProxy")) {
            throw "Alloy OpenVEX statement is not scoped to the reviewed daemon-only $cve attack path"
        }
    }
    if ($socketProxy -match '(?m)^\s+ports:\s*$') {
        throw "docker-socket-proxy must not publish a host port"
    }
    foreach ($capacityControl in @("pids_limit: 128", "mem_limit: 128m", "cpus: 0.50")) {
        if (-not $socketProxy.Contains($capacityControl)) {
            throw "docker-socket-proxy is missing tested streaming capacity control: $capacityControl"
        }
    }
    Assert-HardenedServiceBlock -Block $socketProxy -Service "docker-socket-proxy" -RequireHealthcheck -RequireTmpfs

    if (-not $alloy.Contains('host = "http://docker-socket-proxy:2375"')) {
        throw "Alloy Docker discovery and log source must use docker-socket-proxy"
    }
    $alloyService = Get-ComposeServiceBlock -Content $observability -Service "alloy"
    Assert-HardenedServiceBlock -Block $alloyService -Service "alloy" -RequireHealthcheck -RequireTmpfs

    $cadvisor = Get-ComposeServiceBlock -Content $observability -Service "cadvisor"
    foreach ($forbidden in @("privileged: true", "- /var/run:/var/run", "/dev/kmsg", "/dev/disk", "devices:", "DOCKER_HOST:", "/:/rootfs", "/var/lib/docker")) {
        if ($cadvisor.Contains($forbidden)) {
            throw "cAdvisor retains forbidden host-control access: $forbidden"
        }
    }
    if (-not $cadvisor.Contains("- /sys:/sys:ro")) {
        throw "cAdvisor must retain only the proven read-only cgroup mount"
    }
    Assert-HardenedServiceBlock -Block $cadvisor -Service "cadvisor" -RequireHealthcheck -RequireTmpfs

    if (-not $backupDockerfile.Contains("USER 10001:10001")) {
        throw "backup image must run as non-root UID/GID 10001"
    }
    if (-not $miniappDockerfile.Contains("USER 101:101")) {
        throw "Mini App image must run as the nginx non-root UID/GID"
    }
    if (-not $miniappDockerfile.Contains("EXPOSE 80") -or -not (Get-Content -LiteralPath (Join-Path $repoRoot "deployments\nginx\miniapp.static.conf") -Raw).Contains("listen 80;")) {
        throw "Mini App non-root hardening must preserve the Nginx bind port"
    }

    foreach ($service in @("miniapp", "reverse-proxy")) {
        $block = Get-ComposeServiceBlock -Content $production -Service $service
        Assert-HardenedServiceBlock -Block $block -Service $service -RequireHealthcheck -RequireTmpfs
        foreach ($snippet in @('user: "101:101"', "cap_add:", "- NET_BIND_SERVICE")) {
            if (-not $block.Contains($snippet)) {
                throw "service $service is missing non-root Nginx control: $snippet"
            }
        }
    }

    foreach ($service in @("backup-postgres", "backup-minio", "restore-postgres", "restore-minio")) {
        $block = Get-ComposeServiceBlock -Content $production -Service $service
        Assert-HardenedServiceBlock -Block $block -Service $service -RequireTmpfs
        foreach ($snippet in @('user: "10001:10001"', "HOME: /tmp")) {
            if (-not $block.Contains($snippet)) {
                throw "service $service is missing non-root backup control: $snippet"
            }
        }
        if ($block.Contains("cap_add:")) {
            throw "service $service must not add Linux capabilities"
        }
    }

    foreach ($restoreScript in @("scripts\backup\restore-postgres.sh", "scripts\backup\restore-minio.sh")) {
        $restore = Get-Content -LiteralPath (Join-Path $repoRoot $restoreScript) -Raw
        if (-not $restore.Contains('RESTORE_ALLOW_DESTRUCTIVE:-false') -or -not $restore.Contains('I_UNDERSTAND_RESTORE_OVERWRITES_DATA')) {
            throw "restore script must remain fail-closed: $restoreScript"
        }
    }

    Write-Host "container privilege hardening OK: Docker API proxy, cAdvisor, Mini App, backup and restore"
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
        $prometheusRoot = (Resolve-Path (Join-Path $repoRoot "observability\prometheus")).Path
        $localArguments = @(
            $Arguments | ForEach-Object {
                if ($_ -eq "/etc/prometheus") {
                    $prometheusRoot
                } elseif ($_.StartsWith("/etc/prometheus/")) {
                    $relativePath = $_.Substring("/etc/prometheus/".Length).Replace("/", [string][System.IO.Path]::DirectorySeparatorChar)
                    Join-Path $prometheusRoot $relativePath
                } else {
                    $_
                }
            }
        )
        & $promtool.Source @localArguments
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
    $prometheusImage = "prom/prometheus:v3.13.1@sha256:3c42b892cf723fa54d2f262c37a0e1f80aa8c8ddb1da7b9b0df9455a35a7f893"
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
    $composeEnvFile = New-ComposeValidationEnvFile
    Invoke-Step "docker compose prod app config" {
        $previousAppEnvFile = $env:APP_ENV_FILE
        try {
            $env:APP_ENV_FILE = $composeEnvFile
            docker compose --project-name vk-ai-aggregator-prod --env-file $composeEnvFile -f docker-compose.prod.yml config | Out-Null
        } finally {
            if ($null -eq $previousAppEnvFile) {
                Remove-Item Env:\APP_ENV_FILE -ErrorAction SilentlyContinue
            } else {
                $env:APP_ENV_FILE = $previousAppEnvFile
            }
        }
    }
    if (Test-Path -LiteralPath "docker-compose.data.yml") {
        Invoke-Step "docker compose prod app+data config" {
            $previousAppEnvFile = $env:APP_ENV_FILE
            try {
                $env:APP_ENV_FILE = $composeEnvFile
                docker compose --project-name vk-ai-aggregator-prod --env-file $composeEnvFile -f docker-compose.prod.yml -f docker-compose.data.yml config | Out-Null
            } finally {
                if ($null -eq $previousAppEnvFile) {
                    Remove-Item Env:\APP_ENV_FILE -ErrorAction SilentlyContinue
                } else {
                    $env:APP_ENV_FILE = $previousAppEnvFile
                }
            }
        }
    }
    Remove-Item -LiteralPath $composeEnvFile -Force -ErrorAction SilentlyContinue
}

Assert-Migrations
Assert-NoTrackedEnvFiles
Assert-NoActiveEnvExampleReferences
Assert-CloudflareConfigHasNoSecrets
Assert-CloudflareDeploymentConfig
Assert-ReverseProxyConfig
Assert-DevReverseProxySmokeScript
Assert-DevStartStackScript
Assert-DevStopStatusScripts
Assert-DevPublicSmokeScript
Assert-ProductionDataServices
Assert-ExternalContainerPinsAndHelperSecrets
Assert-HelperEnvFileBehavior
Assert-CloudflaredComposeConfig
Assert-DeployScripts
Assert-CIWorkflowCoverage
Assert-GitHubActionPins
Assert-DockerImageWorkflow
Assert-DeployWorkflowTrustChain -Path ".github\workflows\deploy-dev.yml" -TargetBranch "dev-deploy" -EnvironmentName "development" -AutomaticEvent "push"
Assert-DeployWorkflowTrustChain -Path ".github\workflows\deploy-prod.yml" -TargetBranch "main" -EnvironmentName "production" -AutomaticEvent "workflow_run"
Assert-RollbackConfig
Assert-ContainerPrivilegeHardening
Assert-ObservabilityConfig
Assert-PrometheusConfig

Write-Host "infrastructure validation OK"
