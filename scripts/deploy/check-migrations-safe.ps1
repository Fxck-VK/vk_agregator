[CmdletBinding()]
param(
    [string]$EnvFile = ".env",
    [string]$MigrationsDir = "migrations"
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..")
Set-Location $repoRoot

if (-not (Test-Path -LiteralPath $EnvFile)) {
    throw "Env file not found: $EnvFile"
}
if (-not (Test-Path -LiteralPath $MigrationsDir -PathType Container)) {
    throw "Migrations directory not found: $MigrationsDir"
}

$envValues = @{}
foreach ($line in Get-Content -LiteralPath $EnvFile) {
    $trimmed = $line.Trim()
    if ($trimmed.Length -eq 0 -or $trimmed.StartsWith("#") -or -not $trimmed.Contains("=")) {
        continue
    }
    $key, $value = $trimmed.Split("=", 2)
    $envValues[$key.Trim()] = $value.Trim().Trim('"').Trim("'")
}

function Get-EnvValue {
    param(
        [Parameter(Mandatory = $true)][string]$Name,
        [string]$Default = ""
    )

    if ($envValues.ContainsKey($Name)) {
        return [string]$envValues[$Name]
    }
    return $Default
}

function Test-TrueValue {
    param([string]$Value)

    $normalized = $Value.Trim().ToLowerInvariant()
    return $normalized -in @("1", "true", "yes", "on")
}

$destructiveRegex = "(?i)(^|[\s;])(DROP\s+(TABLE|DATABASE|SCHEMA|TYPE)|TRUNCATE\s+|DELETE\s+FROM|ALTER\s+TABLE[\s\S]*?\sDROP\s+(COLUMN|CONSTRAINT))"
$reviewedMigrations = @{}
$reviewFile = Join-Path $PSScriptRoot "migration-safety.sha256"
if (Test-Path -LiteralPath $reviewFile) {
    foreach ($line in Get-Content -LiteralPath $reviewFile) {
        if ([string]::IsNullOrWhiteSpace($line) -or $line.TrimStart().StartsWith('#')) { continue }
        $entry = [regex]::Match($line, '^([0-9a-f]{64})\s+([0-9]{6}_[a-z0-9_]+\.up\.sql)\s*$')
        if (-not $entry.Success -or $reviewedMigrations.ContainsKey($entry.Groups[2].Value)) {
            throw "Invalid or duplicate migration review entry"
        }
        $reviewedMigrations[$entry.Groups[2].Value] = $entry.Groups[1].Value
    }
}

$destructiveMatches = @()
foreach ($file in Get-ChildItem -LiteralPath $MigrationsDir -File -Filter "*.up.sql" | Sort-Object Name) {
    $fileMatches = @()
    $lineNo = 0
    foreach ($line in Get-Content -LiteralPath $file.FullName) {
        $lineNo++
        if ($line -match $destructiveRegex) {
            $fileMatches += "$($file.FullName):${lineNo}:$line"
        }
    }
    if ($fileMatches.Count -eq 0) { continue }
    if ($reviewedMigrations.ContainsKey($file.Name) -and
        (Get-FileHash -LiteralPath $file.FullName -Algorithm SHA256).Hash.ToLowerInvariant() -eq $reviewedMigrations[$file.Name]) {
        Write-Host "Reviewed constraint migration matched SHA256: $($file.Name)"
        continue
    }
    $destructiveMatches += $fileMatches
}

if ($destructiveMatches.Count -gt 0) {
    $allow = Test-TrueValue -Value (Get-EnvValue -Name "MIGRATION_ALLOW_DESTRUCTIVE" -Default "false")
    $confirmed = (Get-EnvValue -Name "MIGRATION_DESTRUCTIVE_CONFIRM") -eq "I_UNDERSTAND_DESTRUCTIVE_MIGRATIONS"
    if ($allow -and $confirmed) {
        Write-Warning "Destructive migration patterns allowed by explicit confirmation."
        $destructiveMatches | ForEach-Object { Write-Warning $_ }
    } else {
        Write-Error "Destructive migration patterns detected in *.up.sql files:`n$($destructiveMatches -join "`n")`nSet MIGRATION_ALLOW_DESTRUCTIVE=true and MIGRATION_DESTRUCTIVE_CONFIRM=I_UNDERSTAND_DESTRUCTIVE_MIGRATIONS only after manual review and backup."
        exit 1
    }
}

Write-Host "Migration safety check OK: $MigrationsDir"
