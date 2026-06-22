[CmdletBinding()]
param(
    [string]$EnvFile = ".env.loadtest",
    [string]$DatabaseUrl = "",
    [string]$SqlFile = "scripts/loadtest/postgres-diagnostics.sql",
    [int]$Limit = 20,
    [int]$LongQuerySeconds = 30,
    [int]$MinTableMB = 1,
    [string]$OutputFile = "",
    [string]$ComposeProjectName = "",
    [switch]$UseDockerCompose,
    [switch]$AllowProduction
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$RepoRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..")
Set-Location $RepoRoot

function Read-EnvFile {
    param([string]$Path)

    $values = @{}
    if (-not (Test-Path $Path)) {
        return $values
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

$EnvValues = Read-EnvFile -Path $EnvFile

function Get-Setting {
    param(
        [string]$Name,
        [string]$Default = ""
    )

    if ($EnvValues.ContainsKey($Name) -and $EnvValues[$Name] -ne "") {
        return $EnvValues[$Name]
    }

    $fromProcess = [Environment]::GetEnvironmentVariable($Name)
    if (-not [string]::IsNullOrWhiteSpace($fromProcess)) {
        return $fromProcess
    }

    return $Default
}

$appEnv = Get-Setting -Name "APP_ENV"
$allowProdFromEnv = (Get-Setting -Name "ALLOW_PRODUCTION_LOADTEST_DIAGNOSTICS" -Default "false").ToLowerInvariant() -eq "true"
if ($appEnv -eq "production" -and -not $AllowProduction -and -not $allowProdFromEnv) {
    throw "Refusing to run load diagnostics against APP_ENV=production. Pass -AllowProduction only for an approved production audit."
}

if ([string]::IsNullOrWhiteSpace($DatabaseUrl)) {
    $DatabaseUrl = Get-Setting -Name "DATABASE_URL"
}

if ([string]::IsNullOrWhiteSpace($DatabaseUrl)) {
    throw "DATABASE_URL is required. Set it in $EnvFile or pass -DatabaseUrl."
}

$resolvedSqlFile = Resolve-Path $SqlFile
$psqlArgs = @(
    $DatabaseUrl,
    "-X",
    "-v", "limit=$Limit",
    "-v", "long_query_seconds=$LongQuerySeconds",
    "-v", "min_table_mb=$MinTableMB"
)

if (-not $UseDockerCompose -and -not (Get-Command psql -ErrorAction SilentlyContinue)) {
    Write-Warning "psql was not found in PATH; falling back to docker compose exec postgres."
    $UseDockerCompose = $true
}

Write-Host "Running PostgreSQL load diagnostics..."
Write-Host "Environment: $appEnv"
Write-Host "SQL file: $resolvedSqlFile"
Write-Host "Output: read-only diagnostics; DATABASE_URL is intentionally not printed."

if ($UseDockerCompose) {
    if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
        throw "Docker is required for -UseDockerCompose fallback."
    }

    if ([string]::IsNullOrWhiteSpace($ComposeProjectName)) {
        $ComposeProjectName = Get-Setting -Name "COMPOSE_PROJECT_NAME"
    }
    if ([string]::IsNullOrWhiteSpace($ComposeProjectName)) {
        $ComposeProjectName = Get-Setting -Name "COMPOSE_NETWORK_NAME" -Default "vk-ai-aggregator-loadtest"
    }

    $sql = Get-Content -LiteralPath $resolvedSqlFile -Raw
    $composeArgs = @("compose", "-p", $ComposeProjectName)
    if (Test-Path $EnvFile) {
        $composeArgs += @("--env-file", $EnvFile)
    }
    $composeArgs += @("-f", "docker-compose.data.yml", "exec", "-T", "postgres", "psql")
    $composeArgs += $psqlArgs

    $output = $sql | & docker @composeArgs 2>&1
    $exitCode = $LASTEXITCODE
} else {
    $output = & psql @psqlArgs "-f" $resolvedSqlFile 2>&1
    $exitCode = $LASTEXITCODE
}

if (-not [string]::IsNullOrWhiteSpace($OutputFile)) {
    $outputPath = Split-Path -Parent $OutputFile
    if (-not [string]::IsNullOrWhiteSpace($outputPath) -and -not (Test-Path $outputPath)) {
        New-Item -ItemType Directory -Force -Path $outputPath | Out-Null
    }
    $output | Set-Content -LiteralPath $OutputFile -Encoding UTF8
    Write-Host "Diagnostics written to $OutputFile"
} else {
    $output | ForEach-Object { Write-Host $_ }
}

if ($exitCode -ne 0) {
    throw "PostgreSQL diagnostics failed with exit code $exitCode."
}

Write-Host "PostgreSQL diagnostics completed."
