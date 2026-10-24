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

    $retryStages = [System.Collections.Generic.List[string]]::new()
    $retryResult = Invoke-DevDeployPreflight `
        -GitDirectory $tempRoot `
        -CommitSha $failedSha `
        -PolicyVersion "v1" `
        -StageNames $stages `
        -RunStage { param($stage) $retryStages.Add($stage) | Out-Null } `
        -LockTimeoutSeconds 1
    Assert-Equal $false $retryResult.Cached "partial retry cache state"
    Assert-Equal "audit,govulncheck,infrastructure,trivy" ($retryStages -join ",") "partial retry resumes after successful stage"
    if (-not (Test-Path -LiteralPath $retryResult.CachePath -PathType Leaf)) {
        throw "successful partial retry must write the complete cache marker"
    }

    $changedStages = Invoke-DevDeployPreflight -GitDirectory $tempRoot -CommitSha $sha -PolicyVersion v1 `
        -StageNames (@("release-contract") + $stages) -RunStage { param($stage) }
    Assert-Equal "release-contract" ($changedStages.StagesRun -join ",") "new stage cannot hide behind complete cache"

    $runtimeSha = "d" * 40
    $runtimeA = Invoke-DevDeployPreflight -GitDirectory $tempRoot -CommitSha $runtimeSha -PolicyVersion v1 `
        -InputFingerprint ("a" * 64) -StageNames $stages -RunStage { param($stage) }
    $runtimeB = Invoke-DevDeployPreflight -GitDirectory $tempRoot -CommitSha $runtimeSha -PolicyVersion v1 `
        -InputFingerprint ("b" * 64) -StageNames $stages -RunStage { param($stage) }
    Assert-Equal ($stages -join ",") ($runtimeB.StagesRun -join ",") "changed toolchain/settings invalidate cache"
    if ($runtimeA.CachePath -eq $runtimeB.CachePath) { throw "Runtime fingerprints must isolate cache receipts" }

    # A marker's existence is insufficient: it must belong to this input/stage.
    $corruptMarker = Join-Path $tempRoot "dev-deploy-preflight-v1-$sha-tests.ok"
    [IO.File]::WriteAllText($corruptMarker, "untrusted or incomplete marker")
    $repaired = Invoke-DevDeployPreflight -GitDirectory $tempRoot -CommitSha $sha -PolicyVersion v1 `
        -StageNames $stages -RunStage { param($stage) }
    Assert-Equal "tests" ($repaired.StagesRun -join ",") "corrupt stage receipt reruns only that stage"

    foreach ($stage in @("audit", "govulncheck", "trivy")) {
        $marker = Join-Path $tempRoot "dev-deploy-preflight-v1-$sha-$stage.ok"
        $content = [IO.File]::ReadAllText($marker)
        $content = [regex]::Replace($content, 'completed_at=[^\r\n]+', "completed_at=$([DateTime]::UtcNow.AddHours(-25).ToString('O'))")
        [IO.File]::WriteAllText($marker, $content)
    }
    $freshSecurity = Invoke-DevDeployPreflight -GitDirectory $tempRoot -CommitSha $sha -PolicyVersion v1 `
        -StageNames $stages -RunStage { param($stage) }
    Assert-Equal "audit,govulncheck,trivy" ($freshSecurity.StagesRun -join ",") "expired security checks refresh without source-test rerun"
    if (-not (Test-Path -LiteralPath $freshSecurity.TimingPath)) { throw "Preflight must write measured stage timing receipt" }
    $timing = Get-Content -LiteralPath $freshSecurity.TimingPath -Raw | ConvertFrom-Json
    Assert-Equal "passed" $timing.status "timing receipt result"
    Assert-Equal $stages.Count $timing.stages.Count "timings include reused and executed stages"

    Assert-ThrowsLike {
        Invoke-DevDeployPreflight -GitDirectory $tempRoot -CommitSha $sha -PolicyVersion v1 `
            -StageNames $stages -Force -RunStage { throw "forced recheck failed" }
    } "*forced recheck failed*" "forced refresh failure"
    $retryForced = Invoke-DevDeployPreflight -GitDirectory $tempRoot -CommitSha $sha -PolicyVersion v1 `
        -StageNames $stages -RunStage { param($stage) }
    Assert-Equal "tests" ($retryForced.StagesRun -join ",") "failed forced refresh discards stale success"

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
        $absolute = [IO.Path]::GetFullPath($tempRoot)
        $parent = [IO.Path]::GetFullPath([IO.Path]::GetTempPath()).TrimEnd([IO.Path]::DirectorySeparatorChar) + [IO.Path]::DirectorySeparatorChar
        if (-not $absolute.StartsWith($parent, [StringComparison]::OrdinalIgnoreCase) -or [IO.Path]::GetFileName($absolute) -notlike "dev-preflight-*") { throw "Unsafe fixture cleanup path" }
        Remove-Item -LiteralPath $absolute -Recurse -Force
    }
}

Write-Host "DEV deploy preflight tests passed"
