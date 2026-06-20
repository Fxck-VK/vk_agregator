[CmdletBinding()]
param(
    [string]$EnvFile = "dev.env",
    [string]$ProjectName = "vk-ai-aggregator-dev",
    [switch]$WithCloudflare,
    [switch]$SkipBuild,
    [switch]$SkipPublicSmoke,
    [switch]$StatusOnly,
    [switch]$StopOnly,
    [int]$TimeoutSeconds = 180
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$script:RepoRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..")
$script:RuntimeDir = Join-Path $script:RepoRoot ".runtime\dev-stack"
Set-Location $script:RepoRoot

function Invoke-Step {
    param(
        [Parameter(Mandatory = $true)][string]$Name,
        [Parameter(Mandatory = $true)][scriptblock]$Command
    )

    Write-Host "==> $Name"
    $global:LASTEXITCODE = 0
    & $Command
    if ($LASTEXITCODE -ne 0) {
        throw "$Name failed with exit code $LASTEXITCODE"
    }
    $global:LASTEXITCODE = 0
}

function Read-EnvFile {
    param([Parameter(Mandatory = $true)][string]$Path)

    $values = @{}
    foreach ($line in Get-Content -LiteralPath $Path) {
        $trimmed = $line.Trim()
        if ($trimmed.Length -eq 0 -or $trimmed.StartsWith("#")) {
            continue
        }
        $idx = $trimmed.IndexOf("=")
        if ($idx -le 0) {
            continue
        }
        $key = $trimmed.Substring(0, $idx).Trim()
        $value = $trimmed.Substring($idx + 1).Trim().Trim('"').Trim("'")
        $values[$key] = $value
    }
    return $values
}

function Get-EnvValue {
    param(
        [Parameter(Mandatory = $true)][hashtable]$Values,
        [Parameter(Mandatory = $true)][string]$Name,
        [string]$Default = ""
    )

    if ($Values.ContainsKey($Name)) {
        return [string]$Values[$Name]
    }
    return $Default
}

function Get-BoolEnv {
    param(
        [Parameter(Mandatory = $true)][hashtable]$Values,
        [Parameter(Mandatory = $true)][string]$Name,
        [bool]$Default = $false
    )

    $raw = (Get-EnvValue -Values $Values -Name $Name).Trim().ToLowerInvariant()
    if ([string]::IsNullOrWhiteSpace($raw)) {
        return $Default
    }
    return $raw -in @("1", "true", "yes", "on", "enabled")
}

function Test-PlaceholderValue {
    param([string]$Value)

    if ([string]::IsNullOrWhiteSpace($Value)) {
        return $true
    }
    return $Value -match "CHANGE_ME|placeholder|example"
}

function Assert-DevRuntimeMode {
    param(
        [Parameter(Mandatory = $true)][hashtable]$Values,
        [Parameter(Mandatory = $true)][bool]$SkipBuildRequested
    )

    if (-not $SkipBuildRequested) {
        Write-Host "DEV runtime mode: local-build from current working tree"
        Write-Host "DEV code source:   $script:RepoRoot"
        return
    }

    $allowRemoteImages = Get-BoolEnv -Values $Values -Name "DEV_ALLOW_REMOTE_IMAGES" -Default $false
    if (-not $allowRemoteImages) {
        throw "-SkipBuild would run prebuilt Docker images instead of the current working tree. Run without -SkipBuild, or set DEV_ALLOW_REMOTE_IMAGES=true deliberately for image-tag smoke."
    }

    $registry = Get-EnvValue -Values $Values -Name "APP_IMAGE_REGISTRY" -Default "ghcr.io/fxck-vk/vk_agregator"
    $imageTag = Get-EnvValue -Values $Values -Name "IMAGE_TAG" -Default "main"
    Write-Warning "DEV runtime mode: remote-image smoke"
    Write-Warning "DEV image source:  $registry/*:$imageTag"
    Write-Warning "This mode can differ from local code. Use only when testing an already-built image tag."
}

function Assert-DevSecurity {
    param(
        [Parameter(Mandatory = $true)][hashtable]$Values,
        [Parameter(Mandatory = $true)]
        [AllowEmptyCollection()]
        [System.Collections.Generic.List[string]]$Problems
    )

    $productionGroupID = "239332376"
    $vkGroupID = Get-EnvValue -Values $Values -Name "VK_GROUP_ID"
    if ($vkGroupID -eq $productionGroupID) {
        [void]$Problems.Add("VK_GROUP_ID must not be the production group id")
    }

    $referralBase = Get-EnvValue -Values $Values -Name "VK_REFERRAL_LINK_BASE"
    if ($referralBase.Contains($productionGroupID)) {
        [void]$Problems.Add("VK_REFERRAL_LINK_BASE must not point to the production community")
    }

    $expectedTunnelName = Get-EnvValue -Values $Values -Name "DEV_EXPECTED_TUNNEL_NAME" -Default "neiirohub-vk-dev"
    $expectedTunnelHost = Get-EnvValue -Values $Values -Name "DEV_EXPECTED_TUNNEL_HOSTNAME" -Default "dev-vk.neiirohub.ru"
    $actualTunnelName = Get-EnvValue -Values $Values -Name "VK_BOT_TUNNEL_NAME"
    $actualTunnelHost = Get-EnvValue -Values $Values -Name "VK_BOT_TUNNEL_HOSTNAME"
    if ($actualTunnelName -ne $expectedTunnelName) {
        [void]$Problems.Add("VK_BOT_TUNNEL_NAME must be DEV tunnel $expectedTunnelName")
    }
    if ($actualTunnelHost -ne $expectedTunnelHost) {
        [void]$Problems.Add("VK_BOT_TUNNEL_HOSTNAME must be DEV hostname $expectedTunnelHost")
    }

    $allowRealAI = Get-BoolEnv -Values $Values -Name "DEV_ALLOW_REAL_AI_PROVIDERS" -Default $false
    if (-not $allowRealAI) {
        foreach ($name in @("PROVIDER", "PROVIDER_CHAIN", "IMAGE_PROVIDER", "VIDEO_PROVIDER")) {
            $value = (Get-EnvValue -Values $Values -Name $name).ToLowerInvariant()
            if ($value -ne "mock") {
                [void]$Problems.Add("$name must be mock unless DEV_ALLOW_REAL_AI_PROVIDERS=true")
            }
        }

        foreach ($name in @("RUNWAY_PROVIDER_ENABLED", "POYO_PROVIDER_ENABLED", "APIMART_PROVIDER_ENABLED")) {
            $enabled = Get-BoolEnv -Values $Values -Name $name -Default $false
            if ($enabled) {
                [void]$Problems.Add("$name must be false unless DEV_ALLOW_REAL_AI_PROVIDERS=true")
            }
        }

        foreach ($name in @("FAL_API_KEY", "FAL_KEY", "RUNWAYML_API_SECRET", "POYO_API_KEY", "APIMART_API_KEY", "DEEPINFRA_API_KEY", "OPENAI_API_KEY")) {
            $value = Get-EnvValue -Values $Values -Name $name
            if (-not [string]::IsNullOrWhiteSpace($value)) {
                [void]$Problems.Add("$name must be empty unless DEV_ALLOW_REAL_AI_PROVIDERS=true")
            }
        }
    }

    $allowRealPayments = Get-BoolEnv -Values $Values -Name "DEV_ALLOW_REAL_PAYMENTS" -Default $false
    $paymentProvider = (Get-EnvValue -Values $Values -Name "PAYMENT_PROVIDER").ToLowerInvariant()
    if (-not $allowRealPayments) {
        if ($paymentProvider -ne "mock") {
            [void]$Problems.Add("PAYMENT_PROVIDER must be mock unless DEV_ALLOW_REAL_PAYMENTS=true")
        }
        foreach ($name in @("YOOKASSA_SHOP_ID", "YOOKASSA_SECRET_KEY")) {
            $value = Get-EnvValue -Values $Values -Name $name
            if (-not [string]::IsNullOrWhiteSpace($value)) {
                [void]$Problems.Add("$name must be empty unless DEV_ALLOW_REAL_PAYMENTS=true")
            }
        }
    } elseif ($paymentProvider -eq "yookassa") {
        $secretKey = Get-EnvValue -Values $Values -Name "YOOKASSA_SECRET_KEY"
        if ($secretKey -notmatch "^test_") {
            [void]$Problems.Add("YOOKASSA_SECRET_KEY must be a YooKassa test key in DEV")
        }
        if (Test-PlaceholderValue (Get-EnvValue -Values $Values -Name "YOOKASSA_SHOP_ID")) {
            [void]$Problems.Add("YOOKASSA_SHOP_ID must be filled for DEV YooKassa test mode")
        }
    }
}

function Assert-DevEnv {
    param(
        [Parameter(Mandatory = $true)][hashtable]$Values,
        [Parameter(Mandatory = $true)][string]$ResolvedEnvFile
    )

    $problems = [System.Collections.Generic.List[string]]::new()
    $appEnv = (Get-EnvValue -Values $Values -Name "APP_ENV").ToLowerInvariant()
    if ($appEnv -ne "development" -and $appEnv -ne "dev") {
        [void]$problems.Add("APP_ENV must be development/dev for DEV stack")
    }

    if ($ResolvedEnvFile -match "\.env\.prod") {
        [void]$problems.Add("EnvFile must not be a production template/file")
    }

    $network = Get-EnvValue -Values $Values -Name "COMPOSE_NETWORK_NAME"
    if ($network -notmatch "dev") {
        [void]$problems.Add("COMPOSE_NETWORK_NAME must be DEV-specific")
    }

    $expectedUrls = @{
        PUBLIC_VK_BASE_URL = "https://dev-vk.neiirohub.ru"
        PUBLIC_APP_BASE_URL = "https://dev-app.neiirohub.ru"
        PUBLIC_PAYMENT_WEBHOOK_URL = "https://dev.neiirohub.ru/billing/webhooks/yookassa"
    }
    foreach ($entry in $expectedUrls.GetEnumerator()) {
        $value = Get-EnvValue -Values $Values -Name $entry.Key
        if ($value -ne $entry.Value) {
            [void]$problems.Add("$($entry.Key) must be $($entry.Value)")
        }
    }

    foreach ($name in @("POSTGRES_PASSWORD", "DATABASE_URL", "MINIO_ROOT_USER", "MINIO_ROOT_PASSWORD", "S3_ACCESS_KEY", "S3_SECRET_KEY", "ADMIN_TOKEN")) {
        if (Test-PlaceholderValue (Get-EnvValue -Values $Values -Name $name)) {
            [void]$problems.Add("$name must be filled for local DEV runtime")
        }
    }

    if ($WithCloudflare) {
        $token = Get-EnvValue -Values $Values -Name "CLOUDFLARED_TUNNEL_TOKEN"
        if (Test-PlaceholderValue $token) {
            [void]$problems.Add("CLOUDFLARED_TUNNEL_TOKEN must be filled when -WithCloudflare is used")
        }
    }

    $prodMarkers = @(
        "https://vk.neiirohub.ru",
        "https://app.neiirohub.ru",
        "https://neiirohub.ru/billing/webhooks/yookassa",
        "vk-ai-aggregator-prod"
    )
    foreach ($marker in $prodMarkers) {
        foreach ($value in $Values.Values) {
            if (([string]$value).Contains($marker)) {
                [void]$problems.Add("DEV env contains production marker: $marker")
            }
        }
    }

    Assert-DevSecurity -Values $Values -Problems $problems

    if ($problems.Count -gt 0) {
        Write-Host "DEV env check failed for $ResolvedEnvFile"
        foreach ($problem in $problems) {
            Write-Host " - $problem"
        }
        exit 1
    }

    Write-Host "DEV env check OK: $ResolvedEnvFile"
}

function Set-ProcessEnvFromValues {
    param([Parameter(Mandatory = $true)][hashtable]$Values)

    $previous = @{}
    foreach ($key in $Values.Keys) {
        $previous[$key] = [Environment]::GetEnvironmentVariable($key, "Process")
        [Environment]::SetEnvironmentVariable($key, [string]$Values[$key], "Process")
    }
    return $previous
}

function Restore-ProcessEnv {
    param([Parameter(Mandatory = $true)][hashtable]$Previous)

    foreach ($key in $Previous.Keys) {
        [Environment]::SetEnvironmentVariable($key, $Previous[$key], "Process")
    }
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
}

function Invoke-DockerCompose {
    param([Parameter(ValueFromRemainingArguments = $true)][object[]]$Arguments)

    $flatArgs = @()
    $flatArgs += $script:ComposeArgs
    foreach ($argument in $Arguments) {
        if ($argument -is [System.Array]) {
            $flatArgs += $argument
            continue
        }
        $flatArgs += [string]$argument
    }

    & docker @flatArgs
    if ($LASTEXITCODE -ne 0) {
        throw "docker $($flatArgs -join ' ') failed with exit code $LASTEXITCODE"
    }
}

function Wait-ComposeServiceHealthy {
    param(
        [Parameter(Mandatory = $true)][string]$Service,
        [Parameter(Mandatory = $true)][int]$TimeoutSeconds
    )

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    $lastState = "unknown"
    while ((Get-Date) -lt $deadline) {
        $containerID = (& docker @script:ComposeArgs ps -q $Service 2>$null | Select-Object -First 1)
        if (-not [string]::IsNullOrWhiteSpace($containerID)) {
            $state = (& docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' $containerID 2>$null)
            if (-not [string]::IsNullOrWhiteSpace($state)) {
                $lastState = $state.Trim()
            }
            if ($lastState -eq "healthy" -or $lastState -eq "running") {
                Write-Host "$Service healthy"
                return
            }
            if ($lastState -eq "unhealthy" -or $lastState -eq "exited" -or $lastState -eq "dead") {
                throw "$Service is $lastState"
            }
        }
        Start-Sleep -Seconds 2
    }
    throw "$Service did not become healthy within $TimeoutSeconds seconds (last state: $lastState)"
}

function Wait-Http {
    param(
        [Parameter(Mandatory = $true)][string]$Name,
        [Parameter(Mandatory = $true)][string]$Url,
        [Parameter(Mandatory = $true)][int[]]$ExpectedStatuses,
        [Parameter(Mandatory = $true)][int]$TimeoutSeconds,
        [string]$Method = "GET",
        [string]$Body = ""
    )

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    $lastError = $null
    while ((Get-Date) -lt $deadline) {
        try {
            $params = @{
                Uri = $Url
                Method = $Method
                TimeoutSec = 5
                UseBasicParsing = $true
                ErrorAction = "Stop"
            }
            if ($Body.Length -gt 0) {
                $params.Body = $Body
                $params.ContentType = "application/json"
            }
            $response = Invoke-WebRequest @params
            $status = [int]$response.StatusCode
        } catch {
            $response = $_.Exception.Response
            if ($null -ne $response) {
                $status = [int]$response.StatusCode
            } else {
                $lastError = $_.Exception.Message
                Start-Sleep -Seconds 2
                continue
            }
        }

        if ($ExpectedStatuses -contains $status) {
            Write-Host "$Name OK: HTTP $status"
            return
        }
        $lastError = "HTTP $status"
        Start-Sleep -Seconds 2
    }
    throw "$Name check timed out at $Url ($lastError)"
}

function Stop-Cloudflared {
    $pidFile = Join-Path $script:RuntimeDir "cloudflared.pid"
    if (-not (Test-Path -LiteralPath $pidFile)) {
        return
    }
    $raw = Get-Content -LiteralPath $pidFile -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($raw -match '^\d+$') {
        Stop-Process -Id ([int]$raw) -Force -ErrorAction SilentlyContinue
    }
    Remove-Item -LiteralPath $pidFile -Force -ErrorAction SilentlyContinue
}

function Wait-CloudflaredReady {
    param(
        [Parameter(Mandatory = $true)][string]$StdoutPath,
        [Parameter(Mandatory = $true)][string]$StderrPath,
        [int]$TimeoutSeconds = 60
    )

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    do {
        foreach ($path in @($StdoutPath, $StderrPath)) {
            if (-not (Test-Path -LiteralPath $path)) {
                continue
            }
            $text = Get-Content -LiteralPath $path -Raw -ErrorAction SilentlyContinue
            if ($text -match "Registered tunnel connection|Connection .* registered|Connected to") {
                return $true
            }
            if ($text -match "(?i)invalid token|unauthorized|failed to authenticate") {
                return $false
            }
        }
        Start-Sleep -Milliseconds 500
    } while ((Get-Date) -lt $deadline)

    return $false
}

function Start-Cloudflared {
    param(
        [Parameter(Mandatory = $true)][string]$Token,
        [string]$Protocol = "http2",
        [string]$EdgeIPVersion = "4"
    )

    $cloudflared = Get-Command cloudflared -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($null -eq $cloudflared) {
        throw "cloudflared is not installed or not available in PATH"
    }

    Stop-Cloudflared
    New-Item -ItemType Directory -Force -Path $script:RuntimeDir | Out-Null
    $stdout = Join-Path $script:RuntimeDir "cloudflared-live.log"
    $stderr = Join-Path $script:RuntimeDir "cloudflared-live.err"
    Remove-Item -LiteralPath $stdout, $stderr -Force -ErrorAction SilentlyContinue

    $oldTunnelToken = [Environment]::GetEnvironmentVariable("TUNNEL_TOKEN", "Process")
    $oldCloudflaredTunnelToken = [Environment]::GetEnvironmentVariable("CLOUDFLARED_TUNNEL_TOKEN", "Process")
    [Environment]::SetEnvironmentVariable("TUNNEL_TOKEN", $Token, "Process")
    [Environment]::SetEnvironmentVariable("CLOUDFLARED_TUNNEL_TOKEN", $null, "Process")
    try {
        $cloudflaredPath = $cloudflared.Source
        $filePath = $cloudflaredPath
        $argumentList = @()
        if (-not [string]::IsNullOrWhiteSpace($EdgeIPVersion)) {
            $argumentList += @("--edge-ip-version", $EdgeIPVersion)
        }
        $argumentList += @("tunnel", "--no-autoupdate", "--loglevel", "info")
        if (-not [string]::IsNullOrWhiteSpace($Protocol)) {
            $argumentList += @("--protocol", $Protocol)
        }
        $argumentList += "run"
        $extension = [System.IO.Path]::GetExtension($cloudflaredPath)
        if ($extension -ieq ".ps1" -or $extension -ieq ".cmd") {
            $npmDir = Split-Path -Parent $cloudflaredPath
            $directExe = Join-Path $npmDir "node_modules\cloudflared\bin\cloudflared.exe"
            if (Test-Path -LiteralPath $directExe) {
                $filePath = $directExe
            } else {
                $filePath = $cloudflaredPath
            }
        }

        if ([System.IO.Path]::GetExtension($filePath) -ieq ".ps1") {
            $filePath = (Get-Command powershell.exe -ErrorAction Stop).Source
            $argumentList = @(
                "-NoProfile",
                "-ExecutionPolicy",
                "Bypass",
                "-File",
                $cloudflaredPath
            ) + $argumentList
        }

        $proc = Start-Process -FilePath $filePath `
            -ArgumentList $argumentList `
            -WorkingDirectory $script:RepoRoot `
            -WindowStyle Hidden `
            -RedirectStandardOutput $stdout `
            -RedirectStandardError $stderr `
            -PassThru
    } finally {
        [Environment]::SetEnvironmentVariable("TUNNEL_TOKEN", $oldTunnelToken, "Process")
        [Environment]::SetEnvironmentVariable("CLOUDFLARED_TUNNEL_TOKEN", $oldCloudflaredTunnelToken, "Process")
    }

    Set-Content -Path (Join-Path $script:RuntimeDir "cloudflared.pid") -Value $proc.Id -Encoding ASCII
    $connected = Wait-CloudflaredReady -StdoutPath $stdout -StderrPath $stderr -TimeoutSeconds 60
    if (-not $connected) {
        Write-Warning "cloudflared started but readiness was not confirmed. Check $stderr"
    }
    return $proc.Id
}

$resolvedEnvFile = if ([System.IO.Path]::IsPathRooted($EnvFile)) {
    $EnvFile
} else {
    Join-Path $script:RepoRoot $EnvFile
}

if (-not (Test-Path -LiteralPath $resolvedEnvFile)) {
    throw "DEV env file not found: $EnvFile. Copy .env.dev.example to dev.env and fill DEV-only values."
}

$envValues = Read-EnvFile -Path $resolvedEnvFile
Assert-DevEnv -Values $envValues -ResolvedEnvFile $resolvedEnvFile
Assert-DevRuntimeMode -Values $envValues -SkipBuildRequested ([bool]$SkipBuild)

$previousEnv = Set-ProcessEnvFromValues -Values $envValues
$previousAppEnvFile = [Environment]::GetEnvironmentVariable("APP_ENV_FILE", "Process")
$previousComposeBake = [Environment]::GetEnvironmentVariable("COMPOSE_BAKE", "Process")
[Environment]::SetEnvironmentVariable("APP_ENV_FILE", $resolvedEnvFile, "Process")
[Environment]::SetEnvironmentVariable("COMPOSE_BAKE", "false", "Process")
try {
    $script:ComposeArgs = @(
        "compose",
        "--project-name", $ProjectName,
        "--env-file", $resolvedEnvFile,
        "-f", "docker-compose.prod.yml"
    )

    if ($StatusOnly) {
        Invoke-DockerCompose ps
        $pidFile = Join-Path $script:RuntimeDir "cloudflared.pid"
        if (Test-Path -LiteralPath $pidFile) {
            Write-Host "cloudflared pid: $(Get-Content -LiteralPath $pidFile | Select-Object -First 1)"
        } else {
            Write-Host "cloudflared pid: not running"
        }
        exit 0
    }

    if ($StopOnly) {
        Stop-Cloudflared
        Invoke-DockerCompose down
        Write-Host "DEV stack stopped. Docker volumes were preserved."
        exit 0
    }

    Invoke-Step "check Docker" {
        Test-DockerRuntime
    }

    Invoke-Step "docker compose config" {
        Invoke-DockerCompose config | Out-Null
    }

    Invoke-Step "start Postgres/Redis/MinIO" {
        Invoke-DockerCompose @("up", "-d", "postgres", "redis", "minio")
    }
    foreach ($service in @("postgres", "redis", "minio")) {
        Wait-ComposeServiceHealthy -Service $service -TimeoutSeconds $TimeoutSeconds
    }

    Invoke-Step "remove old migrate container" {
        & docker @script:ComposeArgs rm -f -s migrate | Out-Null
        $global:LASTEXITCODE = 0
    }

    Invoke-Step "run migrations" {
        $args = @("up")
        if (-not $SkipBuild) {
            $args += "--build"
        }
        $args += @("--no-deps", "--exit-code-from", "migrate", "migrate")
        Invoke-DockerCompose @args
    }

    $runtimeServices = @("api", "worker", "provider-webhook", "miniapp", "reverse-proxy")
    Invoke-Step "start API/worker/provider-webhook/miniapp/reverse-proxy" {
        $args = @("up", "-d")
        if ($SkipBuild) {
            $args += "--no-build"
        } else {
            $args += "--build"
        }
        $args += $runtimeServices
        Invoke-DockerCompose @args
    }

    foreach ($service in $runtimeServices) {
        Wait-ComposeServiceHealthy -Service $service -TimeoutSeconds $TimeoutSeconds
    }

    $reverseProxyPort = Get-EnvValue -Values $envValues -Name "REVERSE_PROXY_HTTP_PORT" -Default "8088"
    Wait-Http -Name "API health" -Url "http://127.0.0.1:8080/health" -ExpectedStatuses @(200) -TimeoutSeconds $TimeoutSeconds
    Wait-Http -Name "Worker health" -Url "http://127.0.0.1:9090/healthz" -ExpectedStatuses @(200) -TimeoutSeconds $TimeoutSeconds
    Wait-Http -Name "Provider webhook health" -Url "http://127.0.0.1:8082/health" -ExpectedStatuses @(200) -TimeoutSeconds $TimeoutSeconds
    Wait-Http -Name "Mini App health" -Url "http://127.0.0.1:5173/" -ExpectedStatuses @(200) -TimeoutSeconds $TimeoutSeconds
    Wait-Http -Name "Reverse proxy health" -Url "http://127.0.0.1:$reverseProxyPort/proxy-health" -ExpectedStatuses @(200) -TimeoutSeconds $TimeoutSeconds

    Invoke-Step "DEV reverse proxy smoke" {
        & (Join-Path $PSScriptRoot "check-dev-reverse-proxy.ps1") -BaseUrl "http://127.0.0.1:$reverseProxyPort" -TimeoutSec 10
    }

    if ($WithCloudflare) {
        Invoke-Step "start cloudflared DEV tunnel" {
            $token = Get-EnvValue -Values $envValues -Name "CLOUDFLARED_TUNNEL_TOKEN"
            $protocol = Get-EnvValue -Values $envValues -Name "CLOUDFLARED_PROTOCOL" -Default "http2"
            $edgeIPVersion = Get-EnvValue -Values $envValues -Name "CLOUDFLARED_EDGE_IP_VERSION" -Default "4"
            $cloudflaredPid = Start-Cloudflared -Token $token -Protocol $protocol -EdgeIPVersion $edgeIPVersion
            Write-Host "cloudflared started pid=$cloudflaredPid"
        }

        if (-not $SkipPublicSmoke) {
            Wait-Http -Name "DEV public VK health" -Url "https://dev-vk.neiirohub.ru/health" -ExpectedStatuses @(200) -TimeoutSeconds $TimeoutSeconds
            Wait-Http -Name "DEV public Mini App" -Url "https://dev-app.neiirohub.ru/" -ExpectedStatuses @(200) -TimeoutSeconds $TimeoutSeconds
            Wait-Http -Name "DEV public YooKassa webhook route" -Url "https://dev.neiirohub.ru/billing/webhooks/yookassa" -Method "POST" -Body "{}" -ExpectedStatuses @(400, 401, 403) -TimeoutSeconds $TimeoutSeconds
        }
    }

    Invoke-Step "docker compose ps" {
        Invoke-DockerCompose ps
    }

    Write-Host ""
    Write-Host "DEV stack is running."
    Write-Host "Reverse proxy:      http://127.0.0.1:$reverseProxyPort"
    Write-Host "DEV VK callback:    https://dev-vk.neiirohub.ru/webhooks/vk"
    Write-Host "DEV Mini App:       https://dev-app.neiirohub.ru"
    Write-Host "DEV YooKassa hook:  https://dev.neiirohub.ru/billing/webhooks/yookassa"
    Write-Host "Status:             .\scripts\dev\start-dev-stack.ps1 -StatusOnly"
    Write-Host "Stop:               .\scripts\dev\start-dev-stack.ps1 -StopOnly"
    Write-Host "Runtime logs:       $script:RuntimeDir"
} finally {
    [Environment]::SetEnvironmentVariable("APP_ENV_FILE", $previousAppEnvFile, "Process")
    [Environment]::SetEnvironmentVariable("COMPOSE_BAKE", $previousComposeBake, "Process")
    Restore-ProcessEnv -Previous $previousEnv
}
