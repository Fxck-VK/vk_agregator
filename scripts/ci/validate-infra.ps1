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
            $leaf = Split-Path $_ -Leaf
            ($leaf -eq ".env" -or $leaf -like ".env.*") -and $leaf -ne ".env.example"
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
    & $docker.Source run --rm -v $mount $prometheusImage @Arguments
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

Assert-Migrations
Assert-NoTrackedEnvFiles
Assert-CloudflareConfigHasNoSecrets
Assert-PrometheusConfig

Write-Host "infrastructure validation OK"
