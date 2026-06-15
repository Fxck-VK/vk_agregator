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
    throw "Server env file not found: $EnvFile. Copy .env.staging.example or .env.prod.example to .env and fill real values."
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

foreach ($required in @(
    "APP_IMAGE_REGISTRY",
    "IMAGE_TAG",
    "POSTGRES_PASSWORD",
    "DATABASE_URL",
    "S3_ACCESS_KEY",
    "S3_SECRET_KEY",
    "S3_BUCKET",
    "MINIO_ROOT_USER",
    "MINIO_ROOT_PASSWORD",
    "VK_ACCESS_TOKEN",
    "VK_SECRET",
    "VK_CONFIRMATION_TOKEN",
    "VK_APP_SECRET",
    "ADMIN_TOKEN"
)) {
    Require-Value -Values $envValues -Problems $problems -Name $required -Reason "required for server runtime"
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

$usesOpenAI = $providerValues -match "(^|,)openai(,|$)"
$moderationProvider = (Get-Value -Values $envValues -Name "MODERATION_PROVIDER" -Default "keyword").ToLowerInvariant()
$artifactScanner = (Get-Value -Values $envValues -Name "ARTIFACT_SCANNER" -Default "none").ToLowerInvariant()
if ($moderationProvider -eq "openai" -or $artifactScanner -eq "openai" -or $usesOpenAI) {
    Require-Value -Values $envValues -Problems $problems -Name "OPENAI_API_KEY" -Reason "required when OpenAI provider/moderation/scanner is configured"
}
if ($appEnv -eq "production" -and ($artifactScanner -eq "" -or $artifactScanner -eq "none") -and -not (Is-TrueValue (Get-Value -Values $envValues -Name "ALLOW_UNSCANNED_ARTIFACTS_IN_PRODUCTION"))) {
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
    if ([string]::IsNullOrWhiteSpace($email) -and [string]::IsNullOrWhiteSpace($phone)) {
        Add-Problem -Problems $problems -Name "VK_TOP_UP_RECEIPT_EMAIL/VK_TOP_UP_RECEIPT_PHONE" -Reason "one receipt contact is required when VK_MENU_TOP_UP_ENABLED=true"
    }
    if ($email -match "CHANGE_ME" -or $phone -match "CHANGE_ME") {
        Add-Problem -Problems $problems -Name "VK_TOP_UP_RECEIPT_EMAIL/VK_TOP_UP_RECEIPT_PHONE" -Reason "replace CHANGE_ME placeholder before enabling VK top-up"
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
    foreach ($required in @("GRAFANA_ADMIN_PASSWORD", "GRAFANA_SECRET_KEY", "POSTGRES_EXPORTER_DATA_SOURCE_NAME")) {
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
