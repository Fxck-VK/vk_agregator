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

function Assert-ExternalImagesPinned {
    param(
        [Parameter(Mandatory = $true)][string]$Content,
        [Parameter(Mandatory = $true)][string]$FileName
    )

    $unpinned = @(
        $Content -split "\r?\n" |
            Where-Object { $_ -match '^\s*image:\s+' } |
            Where-Object { $_ -notmatch '\$\{(?:IMAGE_TAG|BACKUP_IMAGE_TAG):\?' } |
            Where-Object { $_ -notmatch '@sha256:[0-9a-f]{64}' }
    )
    if ($unpinned.Count -gt 0) {
        throw "$FileName contains third-party images without immutable digests: $($unpinned -join '; ')"
    }
}

function Assert-DockerfilesPinned {
    param([Parameter(Mandatory = $true)][string[]]$Paths)

    foreach ($path in $Paths) {
        $content = Get-Content -LiteralPath $path -Raw
        $unpinned = @(
            $content -split "\r?\n" |
                Where-Object { $_ -match '^\s*FROM\s+' } |
                Where-Object { $_ -notmatch '@sha256:[0-9a-f]{64}(?:\s+AS\s+\S+)?\s*$' }
        )
        if ($unpinned.Count -gt 0) {
            throw "$(Split-Path -Leaf $path) contains base images without immutable digests: $($unpinned -join '; ')"
        }
    }
}

function Assert-ServiceResourceBounds {
    param(
        [Parameter(Mandatory = $true)][string]$Block,
        [Parameter(Mandatory = $true)][string]$ServiceName
    )

    Assert-Matches -Text $Block -Pattern '(?m)^\s+pids_limit:\s+\d+\s*$' -Message "$ServiceName must set pids_limit"
    Assert-Matches -Text $Block -Pattern '(?m)^\s+cpus:\s+["'']?(?:[0-9.]+[0-9]|\$\{[A-Z0-9_]+:-[0-9.]+[0-9]\})["'']?\s*$' -Message "$ServiceName must set cpus"
    Assert-Matches -Text $Block -Pattern '(?m)^\s+mem_limit:\s+\S+\s*$' -Message "$ServiceName must set mem_limit"
}

function Assert-NoNewPrivileges {
    param(
        [Parameter(Mandatory = $true)][string]$Block,
        [Parameter(Mandatory = $true)][string]$ServiceName
    )

    Assert-Matches `
        -Text $Block `
        -Pattern '(?ms)^\s+security_opt:\s*(?:\r?\n\s+-\s+no-new-privileges:true\s*$|\[\s*"?no-new-privileges:true"?\s*\]\s*$)' `
        -Message "$ServiceName must set no-new-privileges"
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
        -Pattern '(?m)^\s+cpus:\s+["'']?(?:[0-9.]+[0-9]|\$\{[A-Z0-9_]+:-[0-9.]+[0-9]\})["'']?\s*$' `
        -Message "$ServiceName must set cpus"
    Assert-Matches `
        -Text $Block `
        -Pattern '(?m)^\s+mem_limit:\s+\S+\s*$' `
        -Message "$ServiceName must set mem_limit"
}

$prod = Get-RequiredContent -RelativePath "docker-compose.prod.yml"
$data = Get-RequiredContent -RelativePath "docker-compose.data.yml"
$observability = Get-RequiredContent -RelativePath "docker-compose.observability.yml"
$local = Get-RequiredContent -RelativePath "docker-compose.yml"

Assert-NoLatestImages -Content $prod -FileName "docker-compose.prod.yml"
Assert-NoLatestImages -Content $data -FileName "docker-compose.data.yml"
Assert-NoLatestImages -Content $observability -FileName "docker-compose.observability.yml"
Assert-NoLatestImages -Content $local -FileName "docker-compose.yml"
Assert-NoPrivilegedContainers -Content $prod -FileName "docker-compose.prod.yml"
Assert-NoPrivilegedContainers -Content $data -FileName "docker-compose.data.yml"
Assert-NoPrivilegedContainers -Content $observability -FileName "docker-compose.observability.yml"
Assert-ExternalImagesPinned -Content $prod -FileName "docker-compose.prod.yml"
Assert-ExternalImagesPinned -Content $data -FileName "docker-compose.data.yml"
Assert-ExternalImagesPinned -Content $observability -FileName "docker-compose.observability.yml"
Assert-ExternalImagesPinned -Content $local -FileName "docker-compose.yml"
Assert-DockerfilesPinned -Paths @(
    Get-ChildItem -LiteralPath $repoRoot -Filter "Dockerfile.*" -File |
        Sort-Object Name |
        Select-Object -ExpandProperty FullName
)
Assert-Matches `
    -Text $observability `
    -Pattern '(?m)^\s+GF_SECURITY_ADMIN_USER:\s+\$\{GRAFANA_ADMIN_USER:\?[^}]+\}\s*$' `
    -Message "Grafana admin user must be required without a fallback"
Assert-Matches `
    -Text $observability `
    -Pattern '(?m)^\s+GF_SECURITY_ADMIN_PASSWORD:\s+\$\{GRAFANA_ADMIN_PASSWORD:\?[^}]+\}\s*$' `
    -Message "Grafana admin password must be required without a fallback"
Assert-Matches `
    -Text $observability `
    -Pattern '(?m)^\s+GF_SECURITY_SECRET_KEY:\s+\$\{GRAFANA_SECRET_KEY:\?[^}]+\}\s*$' `
    -Message "Grafana secret key must be required without a fallback"
Assert-Matches -Text $observability -Pattern '(?m)^\s*-\s+/var/lib/docker/containers:/var/lib/docker/containers:ro\s*$' -Message "Alloy must read bounded Docker log files"
if ($observability -match '(?i)docker\.sock' -or $observability -match '(?m)^\s+pid:\s+host\s*$' -or $observability -match '(?m)^\s+devices:\s*$') {
    throw "observability compose must not expose Docker socket, host PID namespace, or host devices"
}
if ($observability -match '(?m)^\s+user:\s+["'']?0(?::0)?["'']?\s*$') {
    throw "observability compose must not explicitly run services as root"
}
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

$workerBlock = Get-ServiceBlock -Content $prod -ServiceName "worker" -FileName "docker-compose.prod.yml"
Assert-Matches `
    -Text $workerBlock `
    -Pattern '(?m)^\s+cpus:\s+["'']?\$\{WORKER_CPU_LIMIT:-2\.00\}["'']?\s*$' `
    -Message "worker CPU limit must be configurable with production default 2.00"

$miniAppBlock = Get-ServiceBlock -Content $prod -ServiceName "miniapp" -FileName "docker-compose.prod.yml"
foreach ($tmpfsPath in @("/var/cache/nginx", "/var/run")) {
    $escapedPath = [regex]::Escape($tmpfsPath)
    Assert-Matches `
        -Text $miniAppBlock `
        -Pattern "(?m)^\s+-\s+${escapedPath}:[^\r\n]*uid=101[^\r\n]*gid=101[^\r\n]*mode=0750\s*$" `
        -Message "miniapp tmpfs $tmpfsPath must be writable by the non-root nginx user"
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

$reverseProxyBlock = Get-ServiceBlock -Content $prod -ServiceName "reverse-proxy" -FileName "docker-compose.prod.yml"
Assert-Matches `
    -Text $reverseProxyBlock `
    -Pattern '(?m)^\s+user:\s+["'']?101:101["'']?\s*$' `
    -Message "reverse-proxy must run as the non-root nginx user"
foreach ($tmpfsPath in @("/var/cache/nginx", "/var/run")) {
    $escapedPath = [regex]::Escape($tmpfsPath)
    Assert-Matches `
        -Text $reverseProxyBlock `
        -Pattern "(?m)^\s+-\s+${escapedPath}:[^\r\n]*uid=101[^\r\n]*gid=101[^\r\n]*mode=0750\s*$" `
        -Message "reverse-proxy tmpfs $tmpfsPath must be writable by the non-root nginx user"
}

foreach ($service in @("postgres", "redis", "minio")) {
    $block = Get-ServiceBlock -Content $data -ServiceName $service -FileName "docker-compose.data.yml"
    Assert-NoNewPrivileges -Block $block -ServiceName $service
    Assert-ServiceResourceBounds -Block $block -ServiceName $service
}

foreach ($service in @(
    "prometheus",
    "alertmanager",
    "grafana",
    "loki",
    "alloy",
    "tempo",
    "otel-collector",
    "blackbox-exporter",
    "postgres-exporter",
    "redis-exporter",
    "node-exporter",
    "cadvisor"
)) {
    $block = Get-ServiceBlock -Content $observability -ServiceName $service -FileName "docker-compose.observability.yml"
    Assert-NoNewPrivileges -Block $block -ServiceName $service
    Assert-ServiceResourceBounds -Block $block -ServiceName $service
}

Write-Host "production compose hardening assertions OK"
