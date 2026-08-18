[CmdletBinding()]
param(
    [switch]$SkipPromtool
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..")
Import-Module (Join-Path $PSScriptRoot "NextRouteDiscovery.psm1") -Force
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
        "DATABASE_URL=config-validation-placeholder",
        "POSTGRES_PASSWORD=pg-config",
        "REDIS_ADDR=redis:6379",
        "S3_ENDPOINT=minio:9000",
        "S3_ACCESS_KEY=compose_validate_access",
        "S3_SECRET_KEY=compose_validate_secret",
        "S3_BUCKET=artifacts",
        "S3_USE_SSL=false",
        "S3_REGION=us-east-1",
        "S3_ADDRESSING_STYLE=path",
        "MINIO_ROOT_USER=minio-config",
        "MINIO_ROOT_PASSWORD=minio-config",
        "CLOUDFLARED_TUNNEL_TOKEN=compose-validate-token",
        "DEV_WEB_BASIC_AUTH_HTPASSWD=compose-validation-placeholder",
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
        "include /etc/nginx/dev-web.conf;",
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
        "/(admin|metrics|debug",
        "Strict-Transport-Security",
        "Content-Security-Policy",
        "frame-ancestors 'self' https://vk.com https://*.vk.com https://vk.ru https://*.vk.ru",
        "https://*.userapi.com",
        "https://*.vkuserphoto.ru",
        "X-Content-Type-Options",
        "Referrer-Policy",
        "Permissions-Policy"
    )

    foreach ($snippet in $requiredSnippets) {
        if (-not $content.Contains($snippet)) {
            throw "reverse proxy config is missing required snippet: $snippet"
        }
    }

    if ($content -match '\$request(?!_)') {
        throw "reverse proxy access log must not use `$request because it includes query strings"
    }
    if ($content -match '(?im)^\s*add_header\s+Access-Control-Allow-Origin\s+["'']?\*') {
        throw "reverse proxy must not enable broad wildcard CORS"
    }
    if ($content -match '(?im)script-src\s+[^;]*(unsafe-inline|unsafe-eval)') {
        throw "reverse proxy CSP must not allow unsafe inline/eval scripts"
    }

    $disabledDevWebPath = Join-Path $repoRoot "deployments\nginx\dev-web.disabled.conf"
    if (-not (Test-Path -LiteralPath $disabledDevWebPath)) {
        throw "DEV web disabled nginx fragment is missing: deployments/nginx/dev-web.disabled.conf"
    }
    $disabledDevWeb = Get-Content -LiteralPath $disabledDevWebPath -Raw
    $disabledDevWebLines = @(
        $disabledDevWeb -split "`r?`n" |
            Where-Object { -not [string]::IsNullOrWhiteSpace($_) -and -not $_.TrimStart().StartsWith("#") }
    )
    if ($disabledDevWebLines.Count -gt 0 -or $disabledDevWeb -notmatch '(?m)^\s*#') {
        throw "DEV web disabled nginx fragment must contain only a comment"
    }

    $devWebPath = Join-Path $repoRoot "deployments\nginx\dev-web.conf"
    if (-not (Test-Path -LiteralPath $devWebPath)) {
        throw "DEV web nginx host fragment is missing: deployments/nginx/dev-web.conf"
    }
    $devWeb = Get-Content -LiteralPath $devWebPath -Raw
    foreach ($snippet in @(
        "upstream platform_frontend",
        "server platform:3000;",
        "keepalive 16;",
        "server_name dev-web.neiirohub.ru;",
        'auth_basic "NeiroHub development";',
        "auth_basic_user_file /tmp/dev-web.htpasswd;",
        'proxy_set_header Authorization "";',
        "proxy_set_header Host `$host;",
        "proxy_set_header X-Real-IP `$remote_addr;",
        "proxy_set_header X-Forwarded-For `$proxy_add_x_forwarded_for;",
        "proxy_set_header X-Forwarded-Proto `$forwarded_proto;",
        "proxy_set_header X-Forwarded-Host `$host;",
        'proxy_set_header Connection "";',
        "proxy_pass http://platform_frontend;",
        "Strict-Transport-Security",
        "X-Content-Type-Options",
        "Referrer-Policy",
        "Permissions-Policy",
        "X-Frame-Options",
        "admin|metrics|debug|billing|readyz|healthz",
        "return 404;"
    )) {
        if (-not $devWeb.Contains($snippet)) {
            throw "DEV web nginx host fragment is missing required snippet: $snippet"
        }
    }
    if ($devWeb -match '(?im)^\s*auth_basic\s+off\s*;') {
        throw "DEV web nginx host fragment must keep basic authentication enabled"
    }
    if ($devWeb -match '(?im)^\s*add_header\s+Access-Control-Allow-Origin\s+["'']?\*') {
        throw "DEV web nginx host fragment must not enable broad wildcard CORS"
    }
    if ($devWeb -match '(?im)^\s*(proxy_hide_header|add_header)\s+Content-Security-Policy\b') {
        throw "DEV web nginx must not hide or overwrite the platform nonce CSP"
    }

    $platformProxyPath = Join-Path $repoRoot "web\platform\src\proxy.ts"
    if (-not (Test-Path -LiteralPath $platformProxyPath)) {
        throw "platform nonce proxy is missing: web/platform/src/proxy.ts"
    }
    $platformProxy = Get-Content -LiteralPath $platformProxyPath -Raw
    foreach ($snippet in @(
        'matcher: ["/", "/login", "/app/:path*"]',
        "crypto.getRandomValues",
        'requestHeaders.set("x-nonce", nonce)',
        'requestHeaders.set("Content-Security-Policy", contentSecurityPolicy)',
        'response.headers.set("Content-Security-Policy", contentSecurityPolicy)',
        "'strict-dynamic'",
        "style-src 'self' 'nonce-",
        "media-src 'self' blob:",
        "object-src 'none'",
        "base-uri 'none'",
        "form-action 'self'",
        "frame-src 'none'",
        "frame-ancestors 'none'",
        "upgrade-insecure-requests"
    )) {
        if (-not $platformProxy.Contains($snippet)) {
            throw "platform nonce proxy is missing required snippet: $snippet"
        }
    }
    if ($platformProxy -match "unsafe-inline|unsafe-eval") {
        throw "platform nonce proxy must not allow unsafe inline/eval execution"
    }
    if ($platformProxy.Contains('response.headers.set("x-nonce", nonce)')) {
        throw "platform nonce proxy must keep the nonce in request headers and CSP, not expose a redundant response header"
    }
    if ($platformProxy.Contains('response.headers.delete("Set-Cookie")')) {
        throw "platform nonce proxy must preserve the browser-visible return-path cookie"
    }

    $platformAppRoot = Join-Path $repoRoot "web\platform\src\app"
    $platformHomePagePath = Resolve-NextAppRoutePage -AppRoot $platformAppRoot -Route "/"
    $platformHomePage = Get-Content -LiteralPath $platformHomePagePath -Raw
    foreach ($snippet in @(
        'import { connection } from "next/server";',
        "export default async function HomePage()",
        "await connection();"
    )) {
        if (-not $platformHomePage.Contains($snippet)) {
            throw "platform root page must use request-bound rendering for nonce CSP: $snippet"
        }
    }

    Write-Host "reverse proxy config OK"
}

function Assert-ImageDependencySecurityFloor {
    $goModPath = Join-Path $repoRoot "go.mod"
    if (-not (Test-Path -LiteralPath $goModPath)) {
        throw "go.mod is missing"
    }

    $goMod = Get-Content -LiteralPath $goModPath -Raw
    $match = [regex]::Match($goMod, '(?m)^\s*golang\.org/x/text\s+v(?<version>\d+\.\d+\.\d+)')
    if (-not $match.Success) {
        throw "go.mod must pin golang.org/x/text"
    }

    $installed = [version]$match.Groups['version'].Value
    $minimum = [version]"0.39.0"
    if ($installed -lt $minimum) {
        throw "golang.org/x/text must be at least v$minimum (found v$installed)"
    }

    $platformDockerfilePath = Join-Path $repoRoot "Dockerfile.platform"
    if (-not (Test-Path -LiteralPath $platformDockerfilePath)) {
        throw "Dockerfile.platform is missing"
    }

    $platformDockerfile = Get-Content -LiteralPath $platformDockerfilePath -Raw
    $runtimeStage = [regex]::Match($platformDockerfile, '(?ms)^FROM\s+node:[^\r\n]+\s+AS\s+runtime\s*\r?\n(?<body>.*)\z')
    if (-not $runtimeStage.Success) {
        throw "Dockerfile.platform must have a node runtime stage"
    }

    $runtimeBody = $runtimeStage.Groups['body'].Value
    $cleanupRun = [regex]::Match($runtimeBody, '(?ms)^RUN\s+rm\s+-rf\b(?<paths>.*?)(?=^\S|\z)')
    if (-not $cleanupRun.Success) {
        throw "Dockerfile.platform runtime must remove unused package-manager payload in a RUN instruction"
    }

    $copyMatches = [regex]::Matches($runtimeBody, '(?m)^COPY\s+')
    if ($copyMatches.Count -eq 0) {
        throw "Dockerfile.platform runtime must copy the standalone application"
    }
    if ($cleanupRun.Index -le $copyMatches[$copyMatches.Count - 1].Index) {
        throw "Dockerfile.platform runtime must remove package-manager payload after application copies"
    }

    $cleanupPaths = $cleanupRun.Groups['paths'].Value
    foreach ($path in @(
        "/usr/local/lib/node_modules/npm",
        "/usr/local/lib/node_modules/corepack",
        "/usr/local/bin/npm",
        "/usr/local/bin/npx",
        "/usr/local/bin/corepack",
        "/usr/local/bin/yarn",
        "/usr/local/bin/yarnpkg",
        "/opt/yarn-v*"
    )) {
        if (-not $cleanupPaths.Contains($path)) {
            throw "Dockerfile.platform runtime must remove unused package-manager payload: $path"
        }
    }

    Write-Host "image dependency security floor OK: golang.org/x/text v$installed; platform runtime excludes unused package managers"
}

function Assert-MiniAppStaticSecurityHeaders {
    $path = Join-Path $repoRoot "deployments\nginx\miniapp.static.conf"
    if (-not (Test-Path -LiteralPath $path)) {
        throw "Mini App static nginx config is missing"
    }

    $content = Get-Content -LiteralPath $path -Raw
    foreach ($snippet in @(
        "Strict-Transport-Security",
        "Content-Security-Policy",
        "frame-ancestors 'self' https://vk.com https://*.vk.com https://vk.ru https://*.vk.ru",
        "https://*.userapi.com",
        "https://*.vkuserphoto.ru",
        "X-Content-Type-Options",
        "Referrer-Policy",
        "Permissions-Policy"
    )) {
        if (-not $content.Contains($snippet)) {
            throw "Mini App static nginx config is missing required security policy: $snippet"
        }
    }
    if ($content -match '(?im)^\s*add_header\s+Access-Control-Allow-Origin\s+["'']?\*') {
        throw "Mini App static nginx must not enable broad wildcard CORS"
    }
    if ($content -match '(?im)script-src\s+[^;]*(unsafe-inline|unsafe-eval)') {
        throw "Mini App static CSP must not allow unsafe inline/eval scripts"
    }

    Write-Host "Mini App static browser security headers OK"
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
        "dev-web.neiirohub.ru",
        "/health",
        "/miniapp/balance",
        "/billing/webhooks/yookassa",
        "/metrics",
        "/admin/jobs",
        "DEV web gateway required",
        "ExpectedStatuses = @(401)",
        "SkipDevWebGatewayCheck",
        "if (-not `$SkipDevWebGatewayCheck)",
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
        "-SkipDevWebGatewayCheck",
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
    if ($content.Contains("docker-compose.dev-web.yml")) {
        throw "standard DEV stack start script must not include the remote DEV web Compose overlay"
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
        "https://dev-web.neiirohub.ru",
        "must use HTTPS",
        "/health",
        "/webhooks/vk",
        "/miniapp/balance",
        "/billing/webhooks/yookassa",
        "/admin/jobs",
        "/metrics",
        "DEV web gateway required",
        "-ExpectedStatuses @(401)",
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

    if ($content -match "VK_ACCESS_TOKEN|VK_SECRET|YOOKASSA_SECRET|DEEPINFRA_API_KEY|OPENAI_API_KEY|CLOUDFLARED_TUNNEL_TOKEN|DEV_WEB_BASIC_AUTH_HTPASSWD|Authorization|Get-Credential") {
        throw "DEV public smoke script must not reference secrets"
    }

    Write-Host "DEV public smoke script OK"
}

function Assert-DevDeploySmokeScript {
    $path = Join-Path $repoRoot "scripts\deploy\smoke-dev.sh"
    if (-not (Test-Path -LiteralPath $path)) {
        throw "DEV deploy smoke script is missing: scripts/deploy/smoke-dev.sh"
    }

    $content = Get-Content -LiteralPath $path -Raw
    foreach ($snippet in @(
        "https://dev-web.neiirohub.ru",
        "DEV web gateway required",
        '"401"'
    )) {
        if (-not $content.Contains($snippet)) {
            throw "DEV deploy smoke script is missing required snippet: $snippet"
        }
    }

    if ($content -match "DEV_WEB_BASIC_AUTH_HTPASSWD|Authorization|Get-Credential|--user|-u[[:space:]]") {
        throw "DEV deploy smoke script must not read or send DEV web gateway credentials"
    }

    Write-Host "DEV deploy smoke script OK"
}

function Assert-DevDeployWorkflow {
    $path = Join-Path $repoRoot ".github\workflows\deploy-dev.yml"
    if (-not (Test-Path -LiteralPath $path)) {
        throw "DEV deploy workflow is missing: .github/workflows/deploy-dev.yml"
    }

    $content = Get-Content -LiteralPath $path -Raw
    $uploadStep = [regex]::Match($content, '(?ms)^      - name: Upload DEV env\s*\r?\n(?<body>.*?)(?=^      - name:|\z)')
    if (-not $uploadStep.Success) {
        throw "DEV deploy workflow is missing the Upload DEV env step"
    }

    $uploadBody = $uploadStep.Groups['body'].Value
    foreach ($snippet in @(
        "scripts/deploy/assemble-env-parts.sh",
        "--target dev",
        "--web-origin https://dev-web.neiirohub.ru"
    )) {
        if (-not $uploadBody.Contains($snippet)) {
            throw "DEV deploy workflow Upload DEV env step is missing required snippet: $snippet"
        }
    }

    Write-Host "DEV deploy workflow WEB_ORIGIN override OK"
}

function Assert-DevWebOperatorDocs {
    $documents = @{
        "deployments/cloudflare/README.md" = @(
            "dashboard-managed",
            "dev-web.neiirohub.ru/* -> http://127.0.0.1:8088",
            "DEV-only",
            "three local hostnames",
            "remote DEV browser platform",
            "docker-compose.dev-web.yml"
        )
        "docs/DEV_CONTOUR.md" = @(
            "WEB_ORIGIN=https://dev-web.neiirohub.ru",
            "DEV_WEB_BASIC_AUTH_HTPASSWD",
            "single pre-hashed htpasswd entry",
            "never a plaintext password",
            "401 outside gate",
            "gateway credentials then password login",
            "Secure host-only Lax cookies",
            "/web/v1/me",
            "CSRF reject",
            "deep-link return",
            "three local DEV hostnames",
            "remote-only browser platform overlay",
            "-SkipDevWebGatewayCheck"
        )
        "web/platform/README.md" = @(
            "https://dev-web.neiirohub.ru",
            "WEB_ORIGIN=https://dev-web.neiirohub.ru",
            "DEV-only",
            "remote DEV deployment",
            "docker-compose.dev-web.yml",
            "clear the outer Basic Auth gate",
            "/web/v1/me -> 401",
            "protected administrative path -> 404"
        )
    }

    foreach ($relativePath in $documents.Keys) {
        $path = Join-Path $repoRoot $relativePath
        if (-not (Test-Path -LiteralPath $path)) {
            throw "DEV web operator documentation is missing: $relativePath"
        }
        $content = Get-Content -LiteralPath $path -Raw
        foreach ($snippet in $documents[$relativePath]) {
            if (-not $content.Contains($snippet)) {
                throw "DEV web operator documentation is missing required snippet in ${relativePath}: $snippet"
            }
        }
        if ($relativePath -eq "docs/DEV_CONTOUR.md" -and $content -match "all three DEV hostnames") {
            throw "DEV contour docs must not describe every DEV hostname as a three-host local route set"
        }
    }

    Write-Host "DEV web operator documentation OK"
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
        "scripts\backup\test-objectsync-wrapper.sh",
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

    $backupDockerfileContent = Get-Content -LiteralPath (Join-Path $repoRoot "Dockerfile.backup") -Raw
    foreach ($requiredSnippet in @("./cmd/objectsync", "/out/objectsync", "/app/objectsync")) {
        if (-not $backupDockerfileContent.Contains($requiredSnippet)) {
            throw "Dockerfile.backup must build and install the internal object sync binary: $requiredSnippet"
        }
    }
    if ($backupDockerfileContent -match '(?m)\b(?:aws-cli|minio-client|rclone)\b') {
        throw "Dockerfile.backup must not include an external S3 CLI with an independently vulnerable dependency graph"
    }

    foreach ($relativePath in @("scripts\backup\backup-minio.sh", "scripts\backup\restore-minio.sh")) {
        $scriptContent = Get-Content -LiteralPath (Join-Path $repoRoot $relativePath) -Raw
        if ($scriptContent -notmatch '(?m)command -v objectsync' -or $scriptContent -notmatch '(?m)\bobjectsync\b.*\b(?:backup|restore)\b') {
            throw "$relativePath must use the internal object sync binary with an explicit availability check"
        }
        if ($scriptContent -match '(?m)\b(?:aws|mc|rclone)\b') {
            throw "$relativePath must not invoke an external S3 CLI"
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
        '${POSTGRES_PASSWORD:?POSTGRES_PASSWORD is required}',
        '${MINIO_ROOT_USER:?MINIO_ROOT_USER is required}',
        '${MINIO_ROOT_PASSWORD:?MINIO_ROOT_PASSWORD is required}',
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

function Assert-CloudflaredComposeConfig {
    $path = Join-Path $repoRoot "docker-compose.prod.yml"
    if (-not (Test-Path -LiteralPath $path)) {
        Write-Host "no production compose file found; skipping cloudflared compose checks"
        return
    }

    $content = Get-Content -LiteralPath $path -Raw
    $requiredSnippets = @(
        "cloudflared:",
        'image: ${CLOUDFLARED_IMAGE:-cloudflare/cloudflared:2024.12.2@sha256:cb38f3f30910a7d51545118a179b8516eb7066eac61855d62ce6ed733c54ce70}',
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

function Assert-ProductionComposeHardening {
    $path = Join-Path $repoRoot "scripts\ci\assert-prod-compose-hardening.ps1"
    if (-not (Test-Path -LiteralPath $path)) {
        throw "production compose hardening assertion script is missing: scripts/ci/assert-prod-compose-hardening.ps1"
    }

    & $path

    $dataComposePath = Join-Path $repoRoot "docker-compose.data.yml"
    $dataCompose = Get-Content -LiteralPath $dataComposePath -Raw
    foreach ($requiredSecret in @(
        '${POSTGRES_PASSWORD:?POSTGRES_PASSWORD is required}',
        '${MINIO_ROOT_USER:?MINIO_ROOT_USER is required}',
        '${MINIO_ROOT_PASSWORD:?MINIO_ROOT_PASSWORD is required}'
    )) {
        if (-not $dataCompose.Contains($requiredSecret)) {
            throw "data compose must fail closed for missing credential: $requiredSecret"
        }
    }
    if ($dataCompose -match '(?i)local-(?:postgres|minio)-disabled') {
        throw "data compose must not provide disabled-looking credential fallbacks"
    }
    if ([regex]::Matches($dataCompose, '(?m)^\s+cap_drop:\s*$').Count -ne 3) {
        throw "all three data services must drop the default Linux capability set"
    }
    if ([regex]::Matches($dataCompose, '(?m)^\s+read_only:\s+true\s*$').Count -ne 3) {
        throw "all three data services must use a read-only root filesystem"
    }
    if ([regex]::Matches($dataCompose, '(?m)^\s+tmpfs:\s*$').Count -ne 3) {
        throw "all three data services must declare bounded writable tmpfs mounts"
    }
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
                "postgres:16-alpine@sha256:57c72fd2a128e416c7fcc499958864df5301e940bca0a56f58fddf30ffc07777",
                "redis:7-alpine@sha256:6ab0b6e7381779332f97b8ca76193e45b0756f38d4c0dcda72dbb3c32061ab99",
                "minio/mc:RELEASE.2025-08-13T08-35-41Z@sha256:a7fe349ef4bd8521fb8497f55c6042871b2ae640607cf99d9bede5e9bdf11727",
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
                "postgres:16-alpine@sha256:57c72fd2a128e416c7fcc499958864df5301e940bca0a56f58fddf30ffc07777",
                "redis:7-alpine@sha256:6ab0b6e7381779332f97b8ca76193e45b0756f38d4c0dcda72dbb3c32061ab99",
                "minio/mc:RELEASE.2025-08-13T08-35-41Z@sha256:a7fe349ef4bd8521fb8497f55c6042871b2ae640607cf99d9bede5e9bdf11727",
                "smoke-prod.sh",
                "check-migrations-safe.sh",
                "MIGRATION_BACKUP_CONFIRMED",
                "Migration backup:",
                "--env-file",
                "PUBLIC_PAYMENT_WEBHOOK_URL",
                "IMAGE_TAG",
                "migrate_args",
                "exit-code-from",
                "runtime_services=(api maintenance-worker provider-webhook miniapp reverse-proxy)",
                "--relay-only-workers-upgraded",
                "worker_up_args",
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
                "APIMART_API_KEY",
                "GRAFANA_ADMIN_USER"
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
                "APIMART_API_KEY",
                "GRAFANA_ADMIN_USER"
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

    foreach ($deployScript in @(
        @{ Path = "scripts\deploy\deploy-prod.sh"; EnvCheck = "check-prod-env.sh" },
        @{ Path = "scripts\deploy\deploy-dev.sh"; EnvCheck = "check-dev-env.sh" }
    )) {
        $content = Get-Content -LiteralPath (Join-Path $repoRoot $deployScript.Path) -Raw
        $tagValidation = $content.IndexOf('validate_image_tag "${image_tag}"', [StringComparison]::Ordinal)
        $envValidation = $content.IndexOf($deployScript.EnvCheck, [StringComparison]::Ordinal)
        if ($tagValidation -lt 0 -or $envValidation -lt 0 -or $tagValidation -ge $envValidation) {
            throw "deploy script $($deployScript.Path) must validate an explicit image tag before env validation"
        }
    }

    Write-Host "deploy scripts OK"
}

function Assert-DockerImageWorkflow {
    $path = Join-Path $repoRoot ".github\workflows\docker-images.yml"
    if (-not (Test-Path -LiteralPath $path)) {
        throw "Docker image build workflow is missing: .github/workflows/docker-images.yml"
    }

    $content = Get-Content -LiteralPath $path -Raw
    $requiredSnippets = @(
        "name: Docker Images",
        "packages: write",
        "ghcr.io/",
        "docker/setup-buildx-action",
        "docker/login-action",
        "docker/metadata-action",
        "docker/build-push-action",
        "Dockerfile.api",
        "Dockerfile.worker",
        "Dockerfile.provider-webhook",
        "Dockerfile.provider-balance-bot",
        "Dockerfile.miniapp",
        "Dockerfile.platform",
        "Dockerfile.migrate",
        "Dockerfile.backup",
        "service: api",
        "service: worker",
        "service: provider-webhook",
        "service: provider-balance-bot",
        "service: miniapp",
        "service: platform",
        "service: migrate",
        "service: backup",
        "pull-request-build:",
        "Build without registry publication",
        "push: false",
        "publish:",
        "github.ref == 'refs/heads/main' || github.ref == 'refs/heads/dev-deploy'",
        "type=sha,prefix=sha-,format=long",
        "push: true",
        "id-token: write",
        "sbom: true",
        "provenance: mode=max",
        "cosign sign --yes"
    )

    foreach ($snippet in $requiredSnippets) {
        if (-not $content.Contains($snippet)) {
            throw "Docker image workflow is missing required snippet: $snippet"
        }
    }

    if ($content -match '(?m)^\s*type=ref,event=branch\s*$' -or
        $content -match '(?m)^\s*type=sha,prefix=sha-,format=short\s*$') {
        throw "Docker image workflow must publish only immutable full-SHA tags"
    }

    Write-Host "Docker image workflow OK"
}

function Assert-NightlyQualityWorkflow {
    $path = Join-Path $repoRoot "scripts\ci\validate-nightly-quality.ps1"
    if (-not (Test-Path -LiteralPath $path)) {
        throw "Nightly Quality workflow validator is missing: scripts/ci/validate-nightly-quality.ps1"
    }

    & $path
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
        "miniapp:8080",
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
    $prometheusImage = if ([string]::IsNullOrWhiteSpace($env:PROMETHEUS_IMAGE)) {
        "prom/prometheus:v2.55.1@sha256:2659f4c2ebb718e7695cb9b25ffa7d6be64db013daba13e05c875451cf51b0d3"
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
        $previousExporterDSN = $env:POSTGRES_EXPORTER_DATA_SOURCE_NAME
        $previousGrafanaUser = $env:GRAFANA_ADMIN_USER
        $previousGrafanaPassword = $env:GRAFANA_ADMIN_PASSWORD
        $previousGrafanaSecret = $env:GRAFANA_SECRET_KEY
        try {
            if ([string]::IsNullOrWhiteSpace($previousExporterDSN)) {
                $env:POSTGRES_EXPORTER_DATA_SOURCE_NAME = "config-validation-placeholder"
            }
            if ([string]::IsNullOrWhiteSpace($previousGrafanaUser)) {
                $env:GRAFANA_ADMIN_USER = "config-validation-user"
            }
            if ([string]::IsNullOrWhiteSpace($previousGrafanaPassword)) {
                $env:GRAFANA_ADMIN_PASSWORD = ("test" + "-grafana-" + "password")
            }
            if ([string]::IsNullOrWhiteSpace($previousGrafanaSecret)) {
                $env:GRAFANA_SECRET_KEY = "config-validation-secret"
            }
            docker compose --project-name vk-ai-aggregator-observability -f docker-compose.observability.yml config | Out-Null
        } finally {
            if ($null -eq $previousExporterDSN) {
                Remove-Item Env:POSTGRES_EXPORTER_DATA_SOURCE_NAME -ErrorAction SilentlyContinue
            } else {
                $env:POSTGRES_EXPORTER_DATA_SOURCE_NAME = $previousExporterDSN
            }
            if ($null -eq $previousGrafanaUser) {
                Remove-Item Env:GRAFANA_ADMIN_USER -ErrorAction SilentlyContinue
            } else {
                $env:GRAFANA_ADMIN_USER = $previousGrafanaUser
            }
            if ($null -eq $previousGrafanaPassword) {
                Remove-Item Env:GRAFANA_ADMIN_PASSWORD -ErrorAction SilentlyContinue
            } else {
                $env:GRAFANA_ADMIN_PASSWORD = $previousGrafanaPassword
            }
            if ($null -eq $previousGrafanaSecret) {
                Remove-Item Env:GRAFANA_SECRET_KEY -ErrorAction SilentlyContinue
            } else {
                $env:GRAFANA_SECRET_KEY = $previousGrafanaSecret
            }
        }
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
    $devWebComposePath = Join-Path $repoRoot "docker-compose.dev-web.yml"
    if (-not (Test-Path -LiteralPath $devWebComposePath)) {
        throw "DEV web Compose overlay is missing: docker-compose.dev-web.yml"
    }
    $devWebCompose = Get-Content -LiteralPath $devWebComposePath -Raw
    foreach ($snippet in @(
        "platform:",
        'image: ${APP_IMAGE_REGISTRY}/platform:${IMAGE_TAG}',
        'user: "1000:1000"',
        "cap_drop:",
        "- ALL",
        "no-new-privileges:true",
        "read_only: true",
        "tmpfs:",
        "WEB_API_INTERNAL_ORIGIN: http://api:8080",
        "healthcheck:",
        "http://127.0.0.1:3000/health",
        "reverse-proxy:",
        "./deployments/nginx/dev-web.conf:/etc/nginx/dev-web.conf:ro",
        "condition: service_healthy"
    )) {
        if (-not $devWebCompose.Contains($snippet)) {
            throw "DEV web Compose overlay is missing required snippet: $snippet"
        }
    }
    if ($devWebCompose -match '(?m)^\s+ports:\s*$') {
        throw "DEV web platform service must not publish host ports"
    }

    Invoke-Step "docker compose prod app+dev-web config" {
        $previousAppEnvFile = $env:APP_ENV_FILE
        try {
            $env:APP_ENV_FILE = $composeEnvFile
            $baseRendered = docker compose --project-name vk-ai-aggregator-prod --env-file $composeEnvFile -f docker-compose.prod.yml config | Out-String
            if ($LASTEXITCODE -ne 0) {
                throw "docker compose base production config failed"
            }
            if ($baseRendered -match '(?m)^  platform:\s*$') {
                throw "base production Compose must not define the DEV web platform service"
            }

            $devWebRendered = docker compose --project-name vk-ai-aggregator-prod --env-file $composeEnvFile -f docker-compose.prod.yml -f docker-compose.dev-web.yml config | Out-String
            if ($LASTEXITCODE -ne 0) {
                throw "docker compose DEV web overlay config failed"
            }
            if ($devWebRendered -notmatch '(?m)^  platform:\s*$') {
                throw "rendered DEV web Compose config is missing the platform service"
            }

            $platformBlock = [regex]::Match($devWebRendered, '(?ms)^  platform:\s*\r?\n(?<body>.*?)(?=^  [A-Za-z0-9_-]+:|\z)').Groups['body'].Value
            if ([string]::IsNullOrWhiteSpace($platformBlock)) {
                throw "rendered DEV web Compose config has an invalid platform service"
            }
            if ($platformBlock -match '(?m)^    ports:\s*$') {
                throw "rendered DEV web platform service must not publish host ports"
            }

            $reverseProxyBlock = [regex]::Match($devWebRendered, '(?ms)^  reverse-proxy:\s*\r?\n(?<body>.*?)(?=^  [A-Za-z0-9_-]+:|\z)').Groups['body'].Value
            if ($reverseProxyBlock -notmatch '(?ms)^    depends_on:\s*\r?\n.*?^      platform:\s*\r?\n        condition: service_healthy\s*$') {
                throw "rendered DEV web reverse proxy must depend on healthy platform"
            }
        } finally {
            if ($null -eq $previousAppEnvFile) {
                Remove-Item Env:\APP_ENV_FILE -ErrorAction SilentlyContinue
            } else {
                $env:APP_ENV_FILE = $previousAppEnvFile
            }
        }
    }
    Remove-Item -LiteralPath $composeEnvFile -Force -ErrorAction SilentlyContinue
}

Assert-Migrations
Assert-ImageDependencySecurityFloor
Assert-NoTrackedEnvFiles
Assert-NoActiveEnvExampleReferences
Assert-CloudflareConfigHasNoSecrets
Assert-CloudflareDeploymentConfig
Assert-ReverseProxyConfig
Assert-MiniAppStaticSecurityHeaders
Assert-DevReverseProxySmokeScript
Assert-DevStartStackScript
Assert-DevStopStatusScripts
Assert-DevPublicSmokeScript
Assert-DevDeploySmokeScript
Assert-DevDeployWorkflow
Assert-DevWebOperatorDocs
Assert-ProductionDataServices
Assert-CloudflaredComposeConfig
Assert-ProductionComposeHardening
Assert-DeployScripts
Assert-DockerImageWorkflow
Assert-NightlyQualityWorkflow
Assert-RollbackConfig
Assert-ObservabilityConfig
Assert-PrometheusConfig

Write-Host "infrastructure validation OK"
