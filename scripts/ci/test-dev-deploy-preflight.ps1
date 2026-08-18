[CmdletBinding()]
param()

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$modulePath = Join-Path $PSScriptRoot "DevDeployPreflight.psm1"
Import-Module $modulePath -Force

function Assert-Equal {
    param($Expected, $Actual, [string]$Description)
    if ($Expected -ne $Actual) {
        throw "$Description expected=[$Expected] actual=[$Actual]"
    }
}

function Assert-ThrowsLike {
    param([scriptblock]$Action, [string]$Pattern, [string]$Description)
    try {
        & $Action
    } catch {
        if ($_.Exception.Message -notlike $Pattern) {
            throw "$Description threw an unexpected error: $($_.Exception.Message)"
        }
        return
    }
    throw "$Description did not throw"
}

$tempRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("dev-preflight-" + [guid]::NewGuid().ToString("N"))
$stages = @("tests", "audit", "govulncheck", "infrastructure", "trivy")
$sha = "a" * 40

try {
    New-Item -ItemType Directory -Path $tempRoot -Force | Out-Null
    $executed = [System.Collections.Generic.List[string]]::new()
    $result = Invoke-DevDeployPreflight `
        -GitDirectory $tempRoot `
        -CommitSha $sha `
        -PolicyVersion "v1" `
        -StageNames $stages `
        -RunStage { param($stage) $executed.Add($stage) | Out-Null } `
        -LockTimeoutSeconds 1

    Assert-Equal $false $result.Cached "first preflight cache state"
    Assert-Equal ($stages -join ",") ($executed -join ",") "stage order"
    if (-not (Test-Path -LiteralPath $result.CachePath -PathType Leaf)) {
        throw "successful preflight must write a cache marker"
    }

    $cachedResult = Invoke-DevDeployPreflight `
        -GitDirectory $tempRoot `
        -CommitSha $sha `
        -PolicyVersion "v1" `
        -StageNames $stages `
        -RunStage { throw "cached preflight must not execute stages" } `
        -LockTimeoutSeconds 1
    Assert-Equal $true $cachedResult.Cached "repeat preflight cache state"

    $v2Executed = [System.Collections.Generic.List[string]]::new()
    $v2Result = Invoke-DevDeployPreflight `
        -GitDirectory $tempRoot `
        -CommitSha $sha `
        -PolicyVersion "v2" `
        -StageNames $stages `
        -RunStage { param($stage) $v2Executed.Add($stage) | Out-Null } `
        -LockTimeoutSeconds 1
    Assert-Equal $false $v2Result.Cached "policy version invalidates cache"
    Assert-Equal ($stages -join ",") ($v2Executed -join ",") "versioned stage order"

    $failedSha = "b" * 40
    Assert-ThrowsLike {
        Invoke-DevDeployPreflight `
            -GitDirectory $tempRoot `
            -CommitSha $failedSha `
            -PolicyVersion "v1" `
            -StageNames $stages `
            -RunStage { param($stage) if ($stage -eq "audit") { throw "audit failed" } } `
            -LockTimeoutSeconds 1
    } "*audit failed*" "stage failure propagation"
    $failedMarker = Join-Path $tempRoot "dev-deploy-preflight-v1-$failedSha.ok"
    if (Test-Path -LiteralPath $failedMarker) {
        throw "failed preflight must not write a cache marker"
    }

    $lockPath = Join-Path $tempRoot "dev-deploy-preflight.lock"
    $heldLock = [System.IO.File]::Open($lockPath, [System.IO.FileMode]::OpenOrCreate, [System.IO.FileAccess]::ReadWrite, [System.IO.FileShare]::None)
    try {
        Assert-ThrowsLike {
            Invoke-DevDeployPreflight `
                -GitDirectory $tempRoot `
                -CommitSha ("c" * 40) `
                -PolicyVersion "v1" `
                -StageNames $stages `
                -RunStage { } `
                -LockTimeoutSeconds 0
        } "*Another DEV preflight is already running*" "exclusive lock"
    } finally {
        $heldLock.Dispose()
    }
} finally {
    if (Test-Path -LiteralPath $tempRoot) {
        Remove-Item -LiteralPath $tempRoot -Recurse -Force
    }
}

Write-Host "DEV deploy preflight tests passed"
