[CmdletBinding()]
param()

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..")

function Get-RequiredContent {
    param([Parameter(Mandatory = $true)][string]$RelativePath)

    $path = Join-Path $repoRoot $RelativePath
    if (-not (Test-Path -LiteralPath $path)) {
        throw "required compose file is missing: $RelativePath"
    }
    return Get-Content -LiteralPath $path -Raw
}

function Get-ServiceBlock {
    param(
        [Parameter(Mandatory = $true)][string]$Content,
        [Parameter(Mandatory = $true)][string]$ServiceName,
        [Parameter(Mandatory = $true)][string]$FileName
    )

    $escaped = [regex]::Escape($ServiceName)
    $match = [regex]::Match(
        $Content,
        "(?ms)^  $escaped\s*:\s*\r?\n(?<block>.*?)(?=^  [A-Za-z0-9_-]+\s*:\s*\r?\n|^volumes\s*:\s*\r?\n|^networks\s*:\s*\r?\n|\z)"
    )
    if (-not $match.Success) {
        throw "$FileName is missing service: $ServiceName"
    }
    return $match.Value
}

function Assert-Matches {
    param(
        [Parameter(Mandatory = $true)][string]$Text,
        [Parameter(Mandatory = $true)][string]$Pattern,
        [Parameter(Mandatory = $true)][string]$Message
    )

    if ($Text -notmatch $Pattern) {
        throw $Message
    }
}

function Assert-NoLatestImages {
    param(
        [Parameter(Mandatory = $true)][string]$Content,
        [Parameter(Mandatory = $true)][string]$FileName
    )

    $bad = @(
        $Content -split "\r?\n" |
            Where-Object { $_ -match '^\s*image:\s+' -and $_ -match '(?i):latest(\s|$|\})' }
    )
    if ($bad.Count -gt 0) {
        throw "$FileName must not use latest image tags: $($bad -join '; ')"
    }
}

function Assert-NoPrivilegedContainers {
    param(
        [Parameter(Mandatory = $true)][string]$Content,
        [Parameter(Mandatory = $true)][string]$FileName
    )

    if ($Content -match '(?im)^\s*privileged:\s*true\s*$') {
        throw "$FileName must not run privileged containers"
    }
}

function Assert-RequiredTag {
    param(
        [Parameter(Mandatory = $true)][string]$Block,
        [Parameter(Mandatory = $true)][string]$ServiceName,
        [Parameter(Mandatory = $true)][string]$VariableName
    )

    Assert-Matches `
        -Text $Block `
        -Pattern "image:\s+\S+:\$\{$([regex]::Escape($VariableName)):\?[^}]+\}" `
        -Message "$ServiceName image must require $VariableName with no fallback tag"
}

function Assert-ServiceHardening {
    param(
        [Parameter(Mandatory = $true)][string]$Block,
        [Parameter(Mandatory = $true)][string]$ServiceName
    )

    Assert-Matches `
        -Text $Block `
        -Pattern '(?ms)^\s+cap_drop:\s*(?:\r?\n\s+-\s+ALL\s*$|\[\s*"?ALL"?\s*\]\s*$)' `
        -Message "$ServiceName must drop all Linux capabilities"
    Assert-Matches `
        -Text $Block `
        -Pattern '(?ms)^\s+security_opt:\s*(?:\r?\n\s+-\s+no-new-privileges:true\s*$|\[\s*"?no-new-privileges:true"?\s*\]\s*$)' `
        -Message "$ServiceName must set no-new-privileges"
    Assert-Matches `
        -Text $Block `
        -Pattern '(?m)^\s+read_only:\s+true\s*$' `
        -Message "$ServiceName must use a read-only root filesystem"
    Assert-Matches `
        -Text $Block `
        -Pattern '(?ms)^\s+tmpfs:\s*\r?\n\s+-\s+/' `
        -Message "$ServiceName must declare tmpfs for writable runtime paths"
    Assert-Matches `
        -Text $Block `
        -Pattern '(?m)^\s+pids_limit:\s+\d+\s*$' `
        -Message "$ServiceName must set pids_limit"
    Assert-Matches `
        -Text $Block `
        -Pattern '(?m)^\s+cpus:\s+"?[0-9.]+[0-9]"?\s*$' `
        -Message "$ServiceName must set cpus"
    Assert-Matches `
        -Text $Block `
        -Pattern '(?m)^\s+mem_limit:\s+\S+\s*$' `
        -Message "$ServiceName must set mem_limit"
}

$prod = Get-RequiredContent -RelativePath "docker-compose.prod.yml"
$observability = Get-RequiredContent -RelativePath "docker-compose.observability.yml"

Assert-NoLatestImages -Content $prod -FileName "docker-compose.prod.yml"
Assert-NoLatestImages -Content $observability -FileName "docker-compose.observability.yml"
Assert-NoPrivilegedContainers -Content $prod -FileName "docker-compose.prod.yml"
Assert-NoPrivilegedContainers -Content $observability -FileName "docker-compose.observability.yml"
Assert-Matches `
    -Text $observability `
    -Pattern '(?m)^\s+DATA_SOURCE_NAME:\s+\$\{POSTGRES_EXPORTER_DATA_SOURCE_NAME:\?[^}]+\}\s*$' `
    -Message "postgres-exporter must require POSTGRES_EXPORTER_DATA_SOURCE_NAME without a credential fallback"

foreach ($service in @(
    "migrate",
    "api",
    "worker",
    "maintenance-worker",
    "provider-webhook",
    "provider-balance-bot",
    "miniapp"
)) {
    $block = Get-ServiceBlock -Content $prod -ServiceName $service -FileName "docker-compose.prod.yml"
    Assert-RequiredTag -Block $block -ServiceName $service -VariableName "IMAGE_TAG"
    Assert-ServiceHardening -Block $block -ServiceName $service
}

foreach ($service in @(
    "backup-postgres",
    "backup-minio",
    "restore-postgres",
    "restore-minio"
)) {
    $block = Get-ServiceBlock -Content $prod -ServiceName $service -FileName "docker-compose.prod.yml"
    Assert-RequiredTag -Block $block -ServiceName $service -VariableName "BACKUP_IMAGE_TAG"
    Assert-ServiceHardening -Block $block -ServiceName $service
}

foreach ($service in @("reverse-proxy", "cloudflared")) {
    $block = Get-ServiceBlock -Content $prod -ServiceName $service -FileName "docker-compose.prod.yml"
    Assert-ServiceHardening -Block $block -ServiceName $service
}

Write-Host "production compose hardening assertions OK"
