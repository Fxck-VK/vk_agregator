[CmdletBinding()]
param(
    [string]$EnvFile = ".env.loadtest"
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$RepoRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..")
Set-Location $RepoRoot

function Read-EnvFile {
    param([string]$Path)

    $values = @{}
    if (-not (Test-Path -LiteralPath $Path)) {
        throw "Env file not found."
    }

    foreach ($line in Get-Content -LiteralPath $Path) {
        $trimmed = $line.Trim()
        if ($trimmed.Length -eq 0 -or $trimmed.StartsWith("#")) {
            continue
        }

        $idx = $trimmed.IndexOf("=")
        if ($idx -lt 1) {
            continue
        }

        $key = $trimmed.Substring(0, $idx).Trim()
        $value = $trimmed.Substring($idx + 1).Trim()
        if (($value.StartsWith('"') -and $value.EndsWith('"')) -or ($value.StartsWith("'") -and $value.EndsWith("'"))) {
            $value = $value.Substring(1, $value.Length - 2)
        }

        $values[$key] = $value
    }

    return $values
}

function Get-EnvValue {
    param(
        [hashtable]$Values,
        [string]$Name,
        [string]$Default = ""
    )

    if ($Values.ContainsKey($Name)) {
        return [string]$Values[$Name]
    }

    $fromProcess = [Environment]::GetEnvironmentVariable($Name)
    if (-not [string]::IsNullOrWhiteSpace($fromProcess)) {
        return $fromProcess
    }

    return $Default
}

function Get-BoolEnv {
    param(
        [hashtable]$Values,
        [string]$Name,
        [bool]$Default = $false
    )

    $value = Get-EnvValue -Values $Values -Name $Name
    if ([string]::IsNullOrWhiteSpace($value)) {
        return $Default
    }

    return @("1", "true", "yes", "on") -contains $value.Trim().ToLowerInvariant()
}

function Test-TokenEquals {
    param(
        [string]$Value,
        [string]$Expected
    )

    return $Value.Trim().ToLowerInvariant() -eq $Expected.Trim().ToLowerInvariant()
}

function Test-OptionalTokenEquals {
    param(
        [string]$Value,
        [string]$Expected
    )

    return [string]::IsNullOrWhiteSpace($Value) -or (Test-TokenEquals -Value $Value -Expected $Expected)
}

function Test-ProviderChainMockOnly {
    param([string]$Value)

    if ([string]::IsNullOrWhiteSpace($Value)) {
        return $true
    }

    foreach ($part in $Value.Split(",")) {
        if ($part.Trim().Length -eq 0) {
            continue
        }
        if (-not (Test-TokenEquals -Value $part -Expected "mock")) {
            return $false
        }
    }

    return $true
}

function Get-UrlHost {
    param([string]$Value)

    if ([string]::IsNullOrWhiteSpace($Value)) {
        return ""
    }

    try {
        $uri = [Uri]$Value
        if (-not [string]::IsNullOrWhiteSpace($uri.Host)) {
            return $uri.Host.ToLowerInvariant()
        }
    } catch {
        return ""
    }

    return ""
}

$values = Read-EnvFile -Path $EnvFile
$problems = New-Object System.Collections.Generic.List[string]

$appEnv = Get-EnvValue -Values $values -Name "APP_ENV"
if (-not (Test-TokenEquals -Value $appEnv -Expected "loadtest")) {
    $problems.Add("APP_ENV must be loadtest")
}

if (-not (Test-TokenEquals -Value (Get-EnvValue -Values $values -Name "PROVIDER") -Expected "mock")) {
    $problems.Add("PROVIDER must be mock")
}
if (-not (Test-ProviderChainMockOnly -Value (Get-EnvValue -Values $values -Name "PROVIDER_CHAIN"))) {
    $problems.Add("PROVIDER_CHAIN must be empty or mock only")
}
if (-not (Test-OptionalTokenEquals -Value (Get-EnvValue -Values $values -Name "IMAGE_PROVIDER") -Expected "mock")) {
    $problems.Add("IMAGE_PROVIDER must be empty or mock")
}
if (-not (Test-OptionalTokenEquals -Value (Get-EnvValue -Values $values -Name "VIDEO_PROVIDER") -Expected "mock")) {
    $problems.Add("VIDEO_PROVIDER must be empty or mock")
}
if (-not (Test-TokenEquals -Value (Get-EnvValue -Values $values -Name "PAYMENT_PROVIDER") -Expected "mock")) {
    $problems.Add("PAYMENT_PROVIDER must be mock")
}
if (-not (Test-TokenEquals -Value (Get-EnvValue -Values $values -Name "VK_DELIVERY_MODE") -Expected "mock")) {
    $problems.Add("VK_DELIVERY_MODE must be mock")
}
if (-not (Test-OptionalTokenEquals -Value (Get-EnvValue -Values $values -Name "MODERATION_PROVIDER") -Expected "keyword")) {
    $problems.Add("MODERATION_PROVIDER must be empty or keyword")
}
if (-not (Test-TokenEquals -Value (Get-EnvValue -Values $values -Name "ARTIFACT_SCANNER") -Expected "none")) {
    $problems.Add("ARTIFACT_SCANNER must be none")
}

if (-not (Get-BoolEnv -Values $values -Name "LOADTEST_REQUIRE_MOCK_CONTOUR" -Default $true)) {
    $problems.Add("LOADTEST_REQUIRE_MOCK_CONTOUR must stay true for generic capacity tests")
}
if (-not (Get-BoolEnv -Values $values -Name "LOADTEST_BLOCK_PRODUCTION_HOSTS" -Default $true)) {
    $problems.Add("LOADTEST_BLOCK_PRODUCTION_HOSTS must stay true for generic capacity tests")
}
if (Get-BoolEnv -Values $values -Name "K6_ALLOW_PRODUCTION_LIVE_SMOKE" -Default $false) {
    $problems.Add("K6_ALLOW_PRODUCTION_LIVE_SMOKE must be false for generic capacity tests")
}
if (Get-BoolEnv -Values $values -Name "ALLOW_PRODUCTION_LOADTEST_REPORT" -Default $false) {
    $problems.Add("ALLOW_PRODUCTION_LOADTEST_REPORT must be false for generic capacity tests")
}
if (Get-BoolEnv -Values $values -Name "ALLOW_PRODUCTION_LOADTEST_DIAGNOSTICS" -Default $false) {
    $problems.Add("ALLOW_PRODUCTION_LOADTEST_DIAGNOSTICS must be false for generic capacity tests")
}
if (Get-BoolEnv -Values $values -Name "ALLOW_PRODUCTION_LOADTEST_REDIS_DIAGNOSTICS" -Default $false) {
    $problems.Add("ALLOW_PRODUCTION_LOADTEST_REDIS_DIAGNOSTICS must be false for generic capacity tests")
}

$realSecretKeys = @(
    "VK_ACCESS_TOKEN",
    "VK_APP_SECRET",
    "OPENAI_API_KEY",
    "DEEPINFRA_API_KEY",
    "FAL_API_KEY",
    "FAL_KEY",
    "RUNWAYML_API_SECRET",
    "POYO_API_KEY",
    "APIMART_API_KEY",
    "YOOKASSA_SHOP_ID",
    "YOOKASSA_SECRET_KEY"
)
foreach ($key in $realSecretKeys) {
    if (-not [string]::IsNullOrWhiteSpace((Get-EnvValue -Values $values -Name $key))) {
        $problems.Add("$key must be empty in generic loadtest env")
    }
}

$prodHosts = @("vk.neiirohub.ru", "app.neiirohub.ru", "neiirohub.ru")
foreach ($key in ($values.Keys | Sort-Object)) {
    $value = [string]$values[$key]
    if ([string]::IsNullOrWhiteSpace($value)) {
        continue
    }

    $urlHost = Get-UrlHost -Value $value
    if ($urlHost -ne "" -and $prodHosts -contains $urlHost) {
        $problems.Add("$key points to a production host")
        continue
    }

    $lowerValue = $value.Trim().ToLowerInvariant()
    if ($prodHosts -contains $lowerValue) {
        $problems.Add("$key points to a production host")
    }
}

if ($problems.Count -gt 0) {
    Write-Error ("Loadtest preflight failed:`n- " + ($problems -join "`n- "))
    exit 1
}

Write-Host "Loadtest preflight OK: APP_ENV=loadtest, providers/payments/VK delivery are mock, and production hosts are blocked."
