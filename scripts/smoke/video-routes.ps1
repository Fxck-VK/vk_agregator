param(
    [ValidateSet("DryRun", "Mock", "Live")]
    [string]$Mode = "DryRun",
    [string]$RepoRoot = "",
    [int]$TimeoutSeconds = 120,
    [switch]$AllowPaidProviderCalls,
    [switch]$SkipGoTests,
    [switch]$EmitMatrixJson
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

if ([string]::IsNullOrWhiteSpace($RepoRoot)) {
    $RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
} else {
    $RepoRoot = (Resolve-Path $RepoRoot).Path
}

function New-Route {
    param(
        [Parameter(Mandatory = $true)][string]$Alias,
        [Parameter(Mandatory = $true)][string]$Provider,
        [Parameter(Mandatory = $true)][string[]]$SuccessModes,
        [bool]$CancelSupported = $false
    )

    [pscustomobject]@{
        Alias           = $Alias
        Provider        = $Provider
        SuccessModes    = $SuccessModes
        CancelSupported = $CancelSupported
    }
}

function New-SmokeCase {
    param(
        [Parameter(Mandatory = $true)][pscustomobject]$Route,
        [Parameter(Mandatory = $true)][string]$Category,
        [Parameter(Mandatory = $true)][string]$Name,
        [Parameter(Mandatory = $true)][string]$Expected,
        [bool]$Paid = $false
    )

    [pscustomobject]@{
        route_alias = $Route.Alias
        provider    = $Route.Provider
        category    = $Category
        name        = $Name
        expected    = $Expected
        paid        = $Paid
    }
}

function New-RouteSmokeMatrix {
    param([Parameter(Mandatory = $true)][pscustomobject]$Route)

    $cases = New-Object System.Collections.Generic.List[object]
    for ($i = 1; $i -le 5; $i++) {
        $mode = $Route.SuccessModes[($i - 1) % $Route.SuccessModes.Count]
        $cases.Add((New-SmokeCase -Route $Route -Category "success" -Name "success_$i`_$mode" -Expected "provider_submit_poll_download_store_probe_delivery_capture" -Paid $true))
    }

    $cases.Add((New-SmokeCase -Route $Route -Category "invalid_input" -Name "invalid_unsupported_duration" -Expected "reject_before_billing_reserve"))
    $cases.Add((New-SmokeCase -Route $Route -Category "invalid_input" -Name "invalid_missing_or_too_many_references" -Expected "reject_before_provider_submit"))

    $cases.Add((New-SmokeCase -Route $Route -Category "timeout_or_cancel" -Name "submit_or_poll_timeout" -Expected "technical_failure_release_no_capture"))
    if ($Route.CancelSupported) {
        $cases.Add((New-SmokeCase -Route $Route -Category "timeout_or_cancel" -Name "provider_cancel" -Expected "cancel_is_idempotent_release_no_capture"))
    } else {
        $cases.Add((New-SmokeCase -Route $Route -Category "timeout_or_cancel" -Name "poll_timeout_retry_budget" -Expected "technical_failure_release_no_capture"))
    }

    $cases.Add((New-SmokeCase -Route $Route -Category "auth_or_balance" -Name "auth_or_insufficient_balance_stub" -Expected "normalized_provider_error_release_no_capture"))
    $cases.Add((New-SmokeCase -Route $Route -Category "storage" -Name "download_output_into_object_storage" -Expected "user_delivery_uses_our_storage_not_provider_url"))
    $cases.Add((New-SmokeCase -Route $Route -Category "media_pipeline" -Name "probe_transcode_if_enabled" -Expected "fail_closed_on_invalid_media"))
    $cases.Add((New-SmokeCase -Route $Route -Category "billing" -Name "release_on_technical_failure" -Expected "reservation_released_no_capture"))
    $cases.Add((New-SmokeCase -Route $Route -Category "billing" -Name "capture_after_storage_delivery_success" -Expected "capture_only_after_safe_output"))

    return $cases
}

function Assert-Matrix {
    param(
        [Parameter(Mandatory = $true)][pscustomobject]$Route,
        [Parameter(Mandatory = $true)][object[]]$Cases
    )

    $success = @($Cases | Where-Object { $_.category -eq "success" })
    $invalid = @($Cases | Where-Object { $_.category -eq "invalid_input" })
    $timeoutOrCancel = @($Cases | Where-Object { $_.category -eq "timeout_or_cancel" })
    $authOrBalance = @($Cases | Where-Object { $_.category -eq "auth_or_balance" })
    $storage = @($Cases | Where-Object { $_.category -eq "storage" })
    $media = @($Cases | Where-Object { $_.category -eq "media_pipeline" })
    $release = @($Cases | Where-Object { $_.name -eq "release_on_technical_failure" })
    $capture = @($Cases | Where-Object { $_.name -eq "capture_after_storage_delivery_success" })

    if ($success.Count -ne 5) { throw "$($Route.Alias): expected 5 success cases, got $($success.Count)" }
    if ($invalid.Count -ne 2) { throw "$($Route.Alias): expected 2 invalid input cases, got $($invalid.Count)" }
    if ($timeoutOrCancel.Count -ne 2) { throw "$($Route.Alias): expected 2 timeout/cancel cases, got $($timeoutOrCancel.Count)" }
    if ($authOrBalance.Count -ne 1) { throw "$($Route.Alias): expected 1 auth/balance case, got $($authOrBalance.Count)" }
    if ($storage.Count -lt 1) { throw "$($Route.Alias): missing storage download case" }
    if ($media.Count -lt 1) { throw "$($Route.Alias): missing media probe/transcode case" }
    if ($release.Count -ne 1) { throw "$($Route.Alias): missing billing release case" }
    if ($capture.Count -ne 1) { throw "$($Route.Alias): missing billing capture case" }
}

function Assert-TrackedRouteFlagsOff {
    param([Parameter(Mandatory = $true)][string]$Root)

    $envFiles = @(".env.example", ".env.staging.example", ".env.prod.example")
    foreach ($relative in $envFiles) {
        $path = Join-Path $Root $relative
        if (-not (Test-Path -LiteralPath $path)) {
            continue
        }
        $lineNo = 0
        foreach ($line in Get-Content -LiteralPath $path) {
            $lineNo++
            $trimmed = $line.Trim()
            if ($trimmed -match '^(FEATURE_VIDEO_ROUTER_ENABLED|FEATURE_VIDEO_ROUTE_[A-Z0-9_]+_ENABLED)\s*=\s*true\s*$') {
                throw "${relative}:$lineNo has a video route flag enabled. Keep routes disabled before smoke review."
            }
        }
    }
}

function Invoke-MockedGoChecks {
    param([Parameter(Mandatory = $true)][string]$Root)

    if ($SkipGoTests) {
        Write-Host "[SKIP] mocked Go checks skipped by -SkipGoTests"
        return
    }

    $goCache = Join-Path ([System.IO.Path]::GetTempPath()) "vkagg-go-build-cache"
    New-Item -ItemType Directory -Force -Path $goCache | Out-Null
    $oldGoCache = [Environment]::GetEnvironmentVariable("GOCACHE", "Process")
    [Environment]::SetEnvironmentVariable("GOCACHE", $goCache, "Process")
    try {
        $packages = @(
            "./internal/service/videorouter",
            "./internal/service/joborchestrator",
            "./internal/worker",
            "./internal/adapter/provider/apimart",
            "./internal/adapter/provider/poyo",
            "./internal/adapter/provider/runway"
        )
        Push-Location $Root
        try {
            Write-Host "[RUN] go test mocked video route packages"
            & go test @packages
            if ($LASTEXITCODE -ne 0) {
                throw "go test mocked video route packages failed with exit code $LASTEXITCODE"
            }
        } finally {
            Pop-Location
        }
    } finally {
        [Environment]::SetEnvironmentVariable("GOCACHE", $oldGoCache, "Process")
    }
}

function Assert-LiveProviderPreflight {
    if (-not $AllowPaidProviderCalls) {
        throw "Live mode may create paid provider jobs. Re-run only with -AllowPaidProviderCalls after explicit approval."
    }

    $requiredNames = @("APIMART_API_KEY", "POYO_API_KEY", "RUNWAYML_API_SECRET")
    $missing = @()
    foreach ($name in $requiredNames) {
        if ([string]::IsNullOrWhiteSpace([Environment]::GetEnvironmentVariable($name))) {
            $missing += $name
        }
    }
    if ($missing.Count -gt 0) {
        throw "Live smoke preflight missing provider env names: $($missing -join ', ')"
    }

    throw "Live paid submit/poll is intentionally not implemented in this phase. Use DryRun/Mock until a separately approved live smoke harness is added."
}

$routes = @(
    New-Route -Alias "video_hailuo_2_3_fast" -Provider "apimart" -SuccessModes @("i2v")
    New-Route -Alias "video_hailuo_2_3_standard" -Provider "apimart" -SuccessModes @("t2v", "i2v")
    New-Route -Alias "video_kling_o3_standard" -Provider "poyo" -SuccessModes @("t2v", "i2v")
    New-Route -Alias "video_seedance_2_0_fast" -Provider "poyo" -SuccessModes @("t2v", "i2v", "reference")
    New-Route -Alias "video_runway_gen4_5" -Provider "poyo" -SuccessModes @("t2v", "i2v")
    New-Route -Alias "video_runway_gen4_turbo" -Provider "runway" -SuccessModes @("i2v") -CancelSupported $true
)

Assert-TrackedRouteFlagsOff -Root $RepoRoot

$matrix = New-Object System.Collections.Generic.List[object]
foreach ($route in $routes) {
    $cases = @(New-RouteSmokeMatrix -Route $route)
    Assert-Matrix -Route $route -Cases $cases
    foreach ($case in $cases) {
        $matrix.Add($case)
    }
    Write-Host "[OK] $($route.Alias): matrix cases=$($cases.Count), provider=$($route.Provider)"
}

if ($EmitMatrixJson) {
    $matrix | ConvertTo-Json -Depth 4
}

switch ($Mode) {
    "DryRun" {
        Write-Host "[OK] dry-run only: no provider calls, no local services required"
    }
    "Mock" {
        Invoke-MockedGoChecks -Root $RepoRoot
        Write-Host "[OK] mocked smoke checks passed without provider calls"
    }
    "Live" {
        Assert-LiveProviderPreflight
    }
}

Write-Host "[OK] routes remain disabled for users; this script does not mutate env or feature flags"
