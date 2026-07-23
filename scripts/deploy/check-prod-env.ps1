[CmdletBinding()]
param(
    [string]$EnvFile = ".env",
    [switch]$WithCloudflare,
    [switch]$BackupBeforeDeploy,
    [switch]$IncludeObservability
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..")
$resolvedEnvFile = if ([System.IO.Path]::IsPathRooted($EnvFile)) {
    $EnvFile
} else {
    Join-Path $repoRoot $EnvFile
}

if (-not (Test-Path -LiteralPath $resolvedEnvFile)) {
    throw "Server env file not found: $EnvFile. Assemble it from split PROD env secrets or create it with real production values."
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
        $value = $trimmed.Substring($idx + 1).Trim()
        $value = $value.Trim('"').Trim("'")
        $values[$key] = $value
    }
    return $values
}

function Get-Value {
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

function Is-TrueValue {
    param([string]$Value)
    return @("1", "true", "yes", "on") -contains $Value.Trim().ToLowerInvariant()
}

function Require-TrueValue {
    param(
        [Parameter(Mandatory = $true)][hashtable]$Values,
        [System.Collections.Generic.List[string]]$Problems,
        [Parameter(Mandatory = $true)][string]$Name,
        [Parameter(Mandatory = $true)][string]$Reason
    )

    if (-not (Is-TrueValue (Get-Value -Values $Values -Name $Name))) {
        Add-Problem -Problems $Problems -Name $Name -Reason "$Reason; must be true"
    }
}

function Require-FalseValue {
    param(
        [Parameter(Mandatory = $true)][hashtable]$Values,
        [System.Collections.Generic.List[string]]$Problems,
        [Parameter(Mandatory = $true)][string]$Name,
        [Parameter(Mandatory = $true)][string]$Reason
    )

    $value = (Get-Value -Values $Values -Name $Name).Trim().ToLowerInvariant()
    if (@("0", "false", "no", "off") -notcontains $value) {
        Add-Problem -Problems $Problems -Name $Name -Reason "$Reason; must be false"
    }
}

function Add-Problem {
    param(
        [System.Collections.Generic.List[string]]$Problems,
        [Parameter(Mandatory = $true)][string]$Name,
        [Parameter(Mandatory = $true)][string]$Reason
    )

    [void]$Problems.Add("$Name - $Reason")
}

function Require-Value {
    param(
        [Parameter(Mandatory = $true)][hashtable]$Values,
        [System.Collections.Generic.List[string]]$Problems,
        [Parameter(Mandatory = $true)][string]$Name,
        [Parameter(Mandatory = $true)][string]$Reason
    )

    $value = Get-Value -Values $Values -Name $Name
    if ([string]::IsNullOrWhiteSpace($value)) {
        Add-Problem -Problems $Problems -Name $Name -Reason $Reason
        return
    }
    if ($value -like "CHANGE_ME*" -or $value -match "CHANGE_ME") {
        Add-Problem -Problems $Problems -Name $Name -Reason "$Reason; replace CHANGE_ME placeholder"
    }
}

function Require-HttpsUrl {
    param(
        [Parameter(Mandatory = $true)][hashtable]$Values,
        [System.Collections.Generic.List[string]]$Problems,
        [Parameter(Mandatory = $true)][string]$Name,
        [Parameter(Mandatory = $true)][string]$Reason
    )

    Require-Value -Values $Values -Problems $Problems -Name $Name -Reason $Reason
    $value = Get-Value -Values $Values -Name $Name
    if ([string]::IsNullOrWhiteSpace($value) -or $value -like "CHANGE_ME*" -or $value -match "CHANGE_ME") {
        return
    }
    if (-not $value.StartsWith("https://", [System.StringComparison]::OrdinalIgnoreCase)) {
        Add-Problem -Problems $Problems -Name $Name -Reason "$Reason; must start with https://"
    }
}

function Normalize-DataServiceMode {
    param(
        [System.Collections.Generic.List[string]]$Problems,
        [Parameter(Mandatory = $true)][string]$Name,
        [string]$Value = "local"
    )

    $mode = $Value.Trim().ToLowerInvariant()
    if ([string]::IsNullOrWhiteSpace($mode)) {
        $mode = "local"
    }
    if (@("local", "external", "managed") -notcontains $mode) {
        Add-Problem -Problems $Problems -Name $Name -Reason "must be one of local, external, managed"
    }
    return $mode
}

$envValues = Read-EnvFile -Path $resolvedEnvFile
$problems = [System.Collections.Generic.List[string]]::new()

$appEnv = (Get-Value -Values $envValues -Name "APP_ENV").ToLowerInvariant()
switch ($appEnv) {
    "production" { $appEnv = "production" }
    "prod" { $appEnv = "production" }
    "staging" { $appEnv = "staging" }
    "stage" { $appEnv = "staging" }
    default { Add-Problem -Problems $problems -Name "APP_ENV" -Reason "must be staging or production" }
}

$dataServicesMode = Normalize-DataServiceMode -Problems $problems -Name "DATA_SERVICES_MODE" -Value (Get-Value -Values $envValues -Name "DATA_SERVICES_MODE" -Default "local")
$postgresMode = Normalize-DataServiceMode -Problems $problems -Name "POSTGRES_MODE" -Value (Get-Value -Values $envValues -Name "POSTGRES_MODE" -Default $dataServicesMode)
$redisMode = Normalize-DataServiceMode -Problems $problems -Name "REDIS_MODE" -Value (Get-Value -Values $envValues -Name "REDIS_MODE" -Default $dataServicesMode)
$s3Mode = Normalize-DataServiceMode -Problems $problems -Name "S3_MODE" -Value (Get-Value -Values $envValues -Name "S3_MODE" -Default $dataServicesMode)

if ((Is-TrueValue (Get-Value -Values $envValues -Name "MIGRATION_ALLOW_DESTRUCTIVE" -Default "false")) -and (Get-Value -Values $envValues -Name "MIGRATION_DESTRUCTIVE_CONFIRM") -ne "I_UNDERSTAND_DESTRUCTIVE_MIGRATIONS") {
    Add-Problem -Problems $problems -Name "MIGRATION_DESTRUCTIVE_CONFIRM" -Reason "required when MIGRATION_ALLOW_DESTRUCTIVE=true"
}
if (Is-TrueValue (Get-Value -Values $envValues -Name "RESTORE_ALLOW_DESTRUCTIVE" -Default "false")) {
    Add-Problem -Problems $problems -Name "RESTORE_ALLOW_DESTRUCTIVE" -Reason "must be false in the persistent deploy env; set it only for a manual restore command"
}
if ($appEnv -eq "production" -and -not (Is-TrueValue (Get-Value -Values $envValues -Name "MIGRATION_BACKUP_CONFIRMED" -Default "false"))) {
    Require-Value -Values $envValues -Problems $problems -Name "BACKUP_IMAGE_TAG" -Reason "required for automatic production migration backup"
    Require-Value -Values $envValues -Problems $problems -Name "BACKUP_DIR" -Reason "required for automatic production migration backup"
    Require-Value -Values $envValues -Problems $problems -Name "BACKUP_RETENTION_DAYS" -Reason "required for automatic production migration backup"
    if ((Get-Value -Values $envValues -Name "BACKUP_POSTGRES_ENABLED" -Default "true").ToLowerInvariant() -eq "false") {
        Add-Problem -Problems $problems -Name "BACKUP_POSTGRES_ENABLED" -Reason "must not be false unless MIGRATION_BACKUP_CONFIRMED=true"
    }
}

foreach ($required in @(
    "APP_IMAGE_REGISTRY",
    "IMAGE_TAG",
    "DATABASE_URL",
    "REDIS_ADDR",
    "S3_ENDPOINT",
    "S3_ACCESS_KEY",
    "S3_SECRET_KEY",
    "S3_BUCKET",
    "VK_ACCESS_TOKEN",
    "VK_SECRET",
    "VK_CONFIRMATION_TOKEN",
    "VK_APP_SECRET",
    "ADMIN_TOKEN"
)) {
    Require-Value -Values $envValues -Problems $problems -Name $required -Reason "required for server runtime"
}

$s3AddressingStyle = (Get-Value -Values $envValues -Name "S3_ADDRESSING_STYLE" -Default "path").ToLowerInvariant()
if ($s3AddressingStyle -notin @("auto", "path", "virtual-hosted", "virtual", "dns")) {
    Add-Problem -Problems $problems -Name "S3_ADDRESSING_STYLE" -Reason "must be auto, path, or virtual-hosted"
}

if ($postgresMode -eq "local") {
    Require-Value -Values $envValues -Problems $problems -Name "POSTGRES_PASSWORD" -Reason "required when POSTGRES_MODE=local"
}

if ($s3Mode -eq "local") {
    Require-Value -Values $envValues -Problems $problems -Name "MINIO_ROOT_USER" -Reason "required when S3_MODE=local"
    Require-Value -Values $envValues -Problems $problems -Name "MINIO_ROOT_PASSWORD" -Reason "required when S3_MODE=local"
}

$imageRegistry = Get-Value -Values $envValues -Name "APP_IMAGE_REGISTRY"
if ($imageRegistry -notlike "ghcr.io/*") {
    Add-Problem -Problems $problems -Name "APP_IMAGE_REGISTRY" -Reason "must point at the GitHub Container Registry image namespace, for example ghcr.io/fxck-vk/vk_agregator"
}
$ghcrUsername = Get-Value -Values $envValues -Name "GHCR_USERNAME"
$ghcrToken = Get-Value -Values $envValues -Name "GHCR_TOKEN"
if (-not [string]::IsNullOrWhiteSpace("$ghcrUsername$ghcrToken")) {
    Require-Value -Values $envValues -Problems $problems -Name "GHCR_USERNAME" -Reason "required when GHCR_TOKEN is configured"
    Require-Value -Values $envValues -Problems $problems -Name "GHCR_TOKEN" -Reason "required when GHCR_USERNAME is configured"
}

if ((Get-Value -Values $envValues -Name "VK_CONFIRMATION_TOKEN") -eq "dev-confirmation") {
    Add-Problem -Problems $problems -Name "VK_CONFIRMATION_TOKEN" -Reason "must not be dev-confirmation in production"
}

$paymentProvider = (Get-Value -Values $envValues -Name "PAYMENT_PROVIDER" -Default "mock").ToLowerInvariant()
if ($paymentProvider -eq "mock") {
    Add-Problem -Problems $problems -Name "PAYMENT_PROVIDER" -Reason "mock is not allowed in production"
}
if ($paymentProvider -eq "yookassa") {
    foreach ($required in @("YOOKASSA_SHOP_ID", "YOOKASSA_SECRET_KEY", "YOOKASSA_RETURN_URL")) {
        Require-Value -Values $envValues -Problems $problems -Name $required -Reason "required when PAYMENT_PROVIDER=yookassa"
    }
    $yooKassaSecretKey = (Get-Value -Values $envValues -Name "YOOKASSA_SECRET_KEY").Trim().ToLowerInvariant()
    if (-not [string]::IsNullOrWhiteSpace($yooKassaSecretKey) -and -not $yooKassaSecretKey.Contains("change_me") -and -not $yooKassaSecretKey.StartsWith("live")) {
        Add-Problem -Problems $problems -Name "YOOKASSA_SECRET_KEY" -Reason "must be a live YooKassa key in production"
    }
}
if (Is-TrueValue (Get-Value -Values $envValues -Name "FEATURE_DEV_PAYMENT_TEST_PRODUCT_ENABLED" -Default "false")) {
    Add-Problem -Problems $problems -Name "FEATURE_DEV_PAYMENT_TEST_PRODUCT_ENABLED" -Reason "not allowed in staging/production"
}

$providerValues = @(
    Get-Value -Values $envValues -Name "PROVIDER"
    Get-Value -Values $envValues -Name "PROVIDER_CHAIN"
    Get-Value -Values $envValues -Name "IMAGE_PROVIDER"
    Get-Value -Values $envValues -Name "VIDEO_PROVIDER"
) -join ","
$providerValues = ($providerValues -replace "\s+", "").ToLowerInvariant()
if ($providerValues -match "(^|,)mock(,|$)") {
    Add-Problem -Problems $problems -Name "PROVIDER/PROVIDER_CHAIN" -Reason "mock provider is not allowed in production"
}
if ($providerValues -match "(^|,)deepinfra(,|$)") {
    Require-Value -Values $envValues -Problems $problems -Name "DEEPINFRA_API_KEY" -Reason "required when a DeepInfra provider is configured"
}

if ($appEnv -eq "production") {
    Require-TrueValue -Values $envValues -Problems $problems -Name "FEATURE_VIDEO_ROUTER_ENABLED" -Reason "required by the production video catalog"
    Require-FalseValue -Values $envValues -Problems $problems -Name "FEATURE_VIDEO_ROUTE_HAILUO_2_3_FAST_ENABLED" -Reason "Hailuo is excluded from the production video catalog"
    Require-FalseValue -Values $envValues -Problems $problems -Name "FEATURE_VIDEO_ROUTE_HAILUO_2_3_STANDARD_ENABLED" -Reason "Hailuo is excluded from the production video catalog"

    foreach ($routeFlag in @(
        "FEATURE_VIDEO_ROUTE_KLING_O3_STANDARD_ENABLED",
        "FEATURE_VIDEO_ROUTE_RUNWAY_GEN4_TURBO_ENABLED",
        "FEATURE_VIDEO_ROUTE_SEEDANCE_2_0_FAST_ENABLED",
        "FEATURE_VIDEO_ROUTE_RUNWAY_GEN4_5_ENABLED"
    )) {
        Require-TrueValue -Values $envValues -Problems $problems -Name $routeFlag -Reason "required by the production video catalog"
    }

    $providerChain = "," + ((Get-Value -Values $envValues -Name "PROVIDER_CHAIN") -replace "\s+", "").ToLowerInvariant() + ","
    if (-not $providerChain.Contains(",poyo,")) {
        Add-Problem -Problems $problems -Name "PROVIDER_CHAIN" -Reason "must include poyo for Kling O3, Seedance 2.0 Fast and Runway Gen-4.5"
    }
    if (-not $providerChain.Contains(",runway,")) {
        Add-Problem -Problems $problems -Name "PROVIDER_CHAIN" -Reason "must include runway for Runway Gen-4 Turbo"
    }

    Require-TrueValue -Values $envValues -Problems $problems -Name "POYO_PROVIDER_ENABLED" -Reason "required for Kling O3, Seedance 2.0 Fast and Runway Gen-4.5"
    Require-Value -Values $envValues -Problems $problems -Name "POYO_API_KEY" -Reason "required for Kling O3, Seedance 2.0 Fast and Runway Gen-4.5"
    Require-HttpsUrl -Values $envValues -Problems $problems -Name "POYO_BASE_URL" -Reason "required for Kling O3, Seedance 2.0 Fast and Runway Gen-4.5"
    Require-TrueValue -Values $envValues -Problems $problems -Name "RUNWAY_PROVIDER_ENABLED" -Reason "required for Runway Gen-4 Turbo"
    Require-Value -Values $envValues -Problems $problems -Name "RUNWAYML_API_SECRET" -Reason "required for Runway Gen-4 Turbo"
    Require-HttpsUrl -Values $envValues -Problems $problems -Name "RUNWAYML_BASE_URL" -Reason "required for Runway Gen-4 Turbo"
}

if (Is-TrueValue (Get-Value -Values $envValues -Name "PROVIDER_BALANCE_BOT_ENABLED" -Default "false")) {
    foreach ($required in @("ALERT_TELEGRAM_BOT_TOKEN", "TELEGRAM_ADMIN_CHAT_ID")) {
        Require-Value -Values $envValues -Problems $problems -Name $required -Reason "required when PROVIDER_BALANCE_BOT_ENABLED=true"
    }
    $providerBalanceCheckerConfigured = $false
    $apimartKey = Get-Value -Values $envValues -Name "APIMART_API_KEY"
    if (-not [string]::IsNullOrWhiteSpace($apimartKey) -and $apimartKey -notmatch "CHANGE_ME") {
        $providerBalanceCheckerConfigured = $true
    }
    if (Is-TrueValue (Get-Value -Values $envValues -Name "POYO_PROVIDER_ENABLED" -Default "false")) {
        $providerBalanceCheckerConfigured = $true
        foreach ($required in @("POYO_API_KEY", "POYO_BASE_URL")) {
            Require-Value -Values $envValues -Problems $problems -Name $required -Reason "required when PROVIDER_BALANCE_BOT_ENABLED=true and POYO_PROVIDER_ENABLED=true"
        }
    }
    if (Is-TrueValue (Get-Value -Values $envValues -Name "RUNWAY_PROVIDER_ENABLED" -Default "false")) {
        $providerBalanceCheckerConfigured = $true
        foreach ($required in @("RUNWAYML_API_SECRET", "RUNWAYML_BASE_URL")) {
            Require-Value -Values $envValues -Problems $problems -Name $required -Reason "required when PROVIDER_BALANCE_BOT_ENABLED=true and RUNWAY_PROVIDER_ENABLED=true"
        }
    }
    if (Is-TrueValue (Get-Value -Values $envValues -Name "DEEPINFRA_BALANCE_PROVIDER_ENABLED" -Default "false")) {
        $providerBalanceCheckerConfigured = $true
        foreach ($required in @("DEEPINFRA_API_KEY", "DEEPINFRA_BALANCE_BASE_URL")) {
            Require-Value -Values $envValues -Problems $problems -Name $required -Reason "required when PROVIDER_BALANCE_BOT_ENABLED=true and DEEPINFRA_BALANCE_PROVIDER_ENABLED=true"
        }
    }
    if (-not $providerBalanceCheckerConfigured) {
        Add-Problem -Problems $problems -Name "PROVIDER_BALANCE_BOT_ENABLED" -Reason "requires at least one provider balance checker"
    }
}

$usesOpenAI = $providerValues -match "(^|,)openai(,|$)"
$moderationProvider = (Get-Value -Values $envValues -Name "MODERATION_PROVIDER" -Default "keyword").ToLowerInvariant()
$artifactScanner = (Get-Value -Values $envValues -Name "ARTIFACT_SCANNER" -Default "none").ToLowerInvariant()
if ($moderationProvider -eq "openai" -or $artifactScanner -eq "openai" -or $usesOpenAI) {
    Require-Value -Values $envValues -Problems $problems -Name "OPENAI_API_KEY" -Reason "required when OpenAI provider/moderation/scanner is configured"
}
if ($appEnv -eq "production" -and ($artifactScanner -eq "" -or $artifactScanner -eq "none") -and
    -not (Is-TrueValue (Get-Value -Values $envValues -Name "ALLOW_UNSCANNED_ARTIFACTS_IN_PRODUCTION" -Default "false"))) {
    Add-Problem -Problems $problems -Name "ARTIFACT_SCANNER" -Reason "must be openai in production unless ALLOW_UNSCANNED_ARTIFACTS_IN_PRODUCTION=true"
} elseif ($artifactScanner -eq "" -or $artifactScanner -eq "none") {
} elseif ($artifactScanner -ne "openai") {
    Add-Problem -Problems $problems -Name "ARTIFACT_SCANNER" -Reason "must be none or openai"
}

$prices = Get-Value -Values $envValues -Name "PRICES"
if ($prices -match "(^|,)image_generate=0(,|$)") {
    Add-Problem -Problems $problems -Name "PRICES" -Reason "image_generate=0 is not allowed in production env"
}

if (Is-TrueValue (Get-Value -Values $envValues -Name "VK_MENU_TOP_UP_ENABLED")) {
    $email = Get-Value -Values $envValues -Name "VK_TOP_UP_RECEIPT_EMAIL"
    $phone = Get-Value -Values $envValues -Name "VK_TOP_UP_RECEIPT_PHONE"
    Require-HttpsUrl -Values $envValues -Problems $problems -Name "PUBLIC_VK_BASE_URL" -Reason "required for VK top-up payment links"
    if (-not (Is-TrueValue (Get-Value -Values $envValues -Name "FEATURE_VK_TOPUP_STATUS_EDIT_ENABLED" -Default "false"))) {
        Add-Problem -Problems $problems -Name "FEATURE_VK_TOPUP_STATUS_EDIT_ENABLED" -Reason "must be true when VK_MENU_TOP_UP_ENABLED=true"
    }
    if ([string]::IsNullOrWhiteSpace($email) -and [string]::IsNullOrWhiteSpace($phone)) {
        Add-Problem -Problems $problems -Name "VK_TOP_UP_RECEIPT_EMAIL/VK_TOP_UP_RECEIPT_PHONE" -Reason "one receipt contact is required when VK_MENU_TOP_UP_ENABLED=true"
    }
    $emailLower = $email.ToLowerInvariant()
    $phoneLower = $phone.ToLowerInvariant()
    if ($emailLower.Contains("change_me") -or $phoneLower.Contains("change_me") -or $emailLower.Contains("example.com") -or $phoneLower.Contains("example")) {
        Add-Problem -Problems $problems -Name "VK_TOP_UP_RECEIPT_EMAIL/VK_TOP_UP_RECEIPT_PHONE" -Reason "replace placeholder before enabling VK top-up"
    }
}

if ($WithCloudflare) {
    Require-Value -Values $envValues -Problems $problems -Name "CLOUDFLARED_TUNNEL_TOKEN" -Reason "required when deploying with Cloudflare tunnel; store it only in the server .env"
    Require-HttpsUrl -Values $envValues -Problems $problems -Name "PUBLIC_VK_BASE_URL" -Reason "required for Cloudflare deploy smoke, expected https://vk.neiirohub.ru"
    Require-HttpsUrl -Values $envValues -Problems $problems -Name "PUBLIC_APP_BASE_URL" -Reason "required for Cloudflare deploy smoke, expected https://app.neiirohub.ru"
    Require-HttpsUrl -Values $envValues -Problems $problems -Name "PUBLIC_PAYMENT_WEBHOOK_URL" -Reason "required for Cloudflare deploy smoke, expected https://neiirohub.ru/billing/webhooks/yookassa"
}

if ($BackupBeforeDeploy) {
    Require-Value -Values $envValues -Problems $problems -Name "BACKUP_IMAGE_TAG" -Reason "required when backup-before-deploy is enabled"
    Require-Value -Values $envValues -Problems $problems -Name "BACKUP_DIR" -Reason "required when backup-before-deploy is enabled"
    Require-Value -Values $envValues -Problems $problems -Name "BACKUP_RETENTION_DAYS" -Reason "required when backup-before-deploy is enabled"
}

if ($IncludeObservability) {
    foreach ($required in @("GRAFANA_ADMIN_USER", "GRAFANA_ADMIN_PASSWORD", "GRAFANA_SECRET_KEY", "POSTGRES_EXPORTER_DATA_SOURCE_NAME")) {
        Require-Value -Values $envValues -Problems $problems -Name $required -Reason "required for production observability"
    }
    if (Is-TrueValue (Get-Value -Values $envValues -Name "ALERT_TELEGRAM_ENABLED")) {
        Require-Value -Values $envValues -Problems $problems -Name "ALERT_TELEGRAM_BOT_TOKEN" -Reason "required when ALERT_TELEGRAM_ENABLED=true"
        Require-Value -Values $envValues -Problems $problems -Name "ALERT_TELEGRAM_CHAT_ID" -Reason "required when ALERT_TELEGRAM_ENABLED=true"
    }
}

if ($problems.Count -gt 0) {
    Write-Host "Server env check failed for $EnvFile"
    Write-Host "Missing/invalid variables:"
    foreach ($problem in $problems) {
        Write-Host " - $problem"
    }
    exit 1
}

Write-Host "Server env check OK: $EnvFile ($appEnv)"
