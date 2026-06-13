[CmdletBinding()]
param(
    [string]$Branch = "main",
    [string]$EnvFile = ".env",
    [string]$ProjectName = "vk-ai-aggregator-prod",
    [string]$ImageTag = "",
    [switch]$SkipPull,
    [switch]$AllowDirty,
    [switch]$SkipBuild,
    [switch]$SkipMigrate,
    [switch]$WithCloudflare,
    [switch]$BackupBeforeDeploy,
    [switch]$PullBaseImages,
    [switch]$NoHealthCheck,
    [int]$TimeoutSeconds = 180
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$script:RepoRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..")
Set-Location $script:RepoRoot

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

function Get-EnvFileValue {
    param(
        [Parameter(Mandatory = $true)][string]$Path,
        [Parameter(Mandatory = $true)][string]$Name,
        [string]$Default = ""
    )

    if (-not (Test-Path -LiteralPath $Path)) {
        return $Default
    }

    foreach ($line in Get-Content -LiteralPath $Path) {
        $trimmed = $line.Trim()
        if ($trimmed.Length -eq 0 -or $trimmed.StartsWith("#")) {
            continue
        }
        if ($trimmed.StartsWith("$Name=")) {
            $value = $trimmed.Substring($Name.Length + 1).Trim()
            return $value.Trim('"').Trim("'")
        }
    }
    return $Default
}

function Wait-Http {
    param(
        [Parameter(Mandatory = $true)][string]$Name,
        [Parameter(Mandatory = $true)][string]$Url,
        [Parameter(Mandatory = $true)][int]$TimeoutSeconds
    )

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    $lastError = $null
    while ((Get-Date) -lt $deadline) {
        try {
            $response = Invoke-WebRequest -UseBasicParsing -Uri $Url -TimeoutSec 5
            if ($response.StatusCode -ge 200 -and $response.StatusCode -lt 300) {
                Write-Host "$Name OK: $Url"
                return
            }
            $lastError = "HTTP $($response.StatusCode)"
        } catch {
            $lastError = $_.Exception.Message
        }
        Start-Sleep -Seconds 2
    }
    throw "$Name health check timed out at $Url ($lastError)"
}

if (-not (Test-Path -LiteralPath "docker-compose.prod.yml")) {
    throw "docker-compose.prod.yml not found; run from the repository root or keep this script in scripts/deploy"
}
if (-not (Test-Path -LiteralPath $EnvFile)) {
    throw "Production env file not found: $EnvFile. Copy .env.prod.example to .env on the server and fill real secrets there."
}

$script:ComposeArgs = @(
    "compose",
    "--project-name", $ProjectName,
    "--env-file", $EnvFile,
    "-f", "docker-compose.prod.yml"
)
if ($WithCloudflare) {
    $script:ComposeArgs += @("--profile", "cloudflare")
}

function Invoke-DockerCompose {
    param([Parameter(ValueFromRemainingArguments = $true)][string[]]$Arguments)

    & docker @script:ComposeArgs @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "docker compose $($Arguments -join ' ') failed with exit code $LASTEXITCODE"
    }
}

if (-not $SkipPull) {
    if (-not $AllowDirty) {
        $dirty = @(git status --porcelain --untracked-files=no)
        if ($LASTEXITCODE -ne 0) {
            throw "git status failed"
        }
        if ($dirty.Count -gt 0) {
            throw "Tracked worktree changes found. Commit/stash them or rerun with -AllowDirty. Changes: $($dirty -join '; ')"
        }
    }

    Invoke-Step "git fetch" { git fetch --prune origin }
    Invoke-Step "git checkout $Branch" { git checkout $Branch }
    Invoke-Step "git pull --ff-only origin $Branch" { git pull --ff-only origin $Branch }
}

$previousAppEnvFile = $env:APP_ENV_FILE
$previousImageTag = $env:IMAGE_TAG
try {
    $env:APP_ENV_FILE = $EnvFile
    if (-not [string]::IsNullOrWhiteSpace($ImageTag)) {
        $env:IMAGE_TAG = $ImageTag
        Write-Host "Using IMAGE_TAG=$ImageTag"
    }

    Invoke-Step "docker compose config" {
        Invoke-DockerCompose config | Out-Null
    }

    if ($BackupBeforeDeploy) {
        $backupArgs = @(
            "compose",
            "--project-name", $ProjectName,
            "--env-file", $EnvFile,
            "-f", "docker-compose.prod.yml",
            "--profile", "backup"
        )
        if ($WithCloudflare) {
            $backupArgs += @("--profile", "cloudflare")
        }
        Invoke-Step "backup postgres" {
            & docker @backupArgs run --rm backup-postgres
            if ($LASTEXITCODE -ne 0) { throw "backup-postgres failed with exit code $LASTEXITCODE" }
        }
        Invoke-Step "backup minio" {
            & docker @backupArgs run --rm backup-minio
            if ($LASTEXITCODE -ne 0) { throw "backup-minio failed with exit code $LASTEXITCODE" }
        }
    }

    Invoke-Step "start stateful dependencies" {
        Invoke-DockerCompose up -d postgres redis minio
    }

    if (-not $SkipBuild) {
        $buildArgs = @("build")
        if ($PullBaseImages) {
            $buildArgs += "--pull"
        }
        $buildArgs += @("api", "worker", "provider-webhook", "miniapp", "migrate")
        if ($BackupBeforeDeploy) {
            $buildArgs += @("backup-postgres", "backup-minio")
        }
        Invoke-Step "docker compose build" {
            Invoke-DockerCompose @buildArgs
        }
    }

    if (-not $SkipMigrate) {
        Invoke-Step "remove old migrate container" {
            & docker @script:ComposeArgs rm -f -s migrate | Out-Null
            $global:LASTEXITCODE = 0
        }
        Invoke-Step "run migrations" {
            Invoke-DockerCompose up --no-deps --exit-code-from migrate migrate
        }
    } else {
        Write-Warning "Skipping migrations. Runtime services still require a successful migrate service state in this compose project."
    }

    $runtimeServices = @("api", "worker", "provider-webhook", "miniapp", "reverse-proxy")
    if ($WithCloudflare) {
        $runtimeServices += "cloudflared"
    }

    Invoke-Step "start runtime services" {
        Invoke-DockerCompose up -d @runtimeServices
    }

    if (-not $NoHealthCheck) {
        $reverseProxyPort = Get-EnvFileValue -Path $EnvFile -Name "REVERSE_PROXY_HTTP_PORT" -Default "8088"
        Wait-Http -Name "reverse-proxy" -Url "http://127.0.0.1:$reverseProxyPort/proxy-health" -TimeoutSeconds $TimeoutSeconds
        Wait-Http -Name "api" -Url "http://127.0.0.1:8080/health" -TimeoutSeconds $TimeoutSeconds
        Wait-Http -Name "provider-webhook" -Url "http://127.0.0.1:8082/health" -TimeoutSeconds $TimeoutSeconds
        Wait-Http -Name "worker" -Url "http://127.0.0.1:9090/healthz" -TimeoutSeconds $TimeoutSeconds
        Wait-Http -Name "miniapp" -Url "http://127.0.0.1:5173/" -TimeoutSeconds $TimeoutSeconds
    }

    Invoke-Step "docker compose ps" {
        Invoke-DockerCompose ps
    }

    Write-Host ""
    Write-Host "Production deploy completed."
} finally {
    if ($null -eq $previousAppEnvFile) {
        Remove-Item Env:\APP_ENV_FILE -ErrorAction SilentlyContinue
    } else {
        $env:APP_ENV_FILE = $previousAppEnvFile
    }
    if ($null -eq $previousImageTag) {
        Remove-Item Env:\IMAGE_TAG -ErrorAction SilentlyContinue
    } else {
        $env:IMAGE_TAG = $previousImageTag
    }
}
