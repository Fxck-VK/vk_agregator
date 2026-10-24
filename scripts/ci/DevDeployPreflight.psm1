Set-StrictMode -Version Latest

function Invoke-DevDeployPreflight {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)][string]$GitDirectory,
        [Parameter(Mandatory = $true)][string]$CommitSha,
        [Parameter(Mandatory = $true)][string]$PolicyVersion,
        [Parameter(Mandatory = $true)][string[]]$StageNames,
        [Parameter(Mandatory = $true)][scriptblock]$RunStage,
        [string]$InputFingerprint = "",
        [ValidateRange(1, 168)][int]$SecurityCacheHours = 24,
        [ValidateRange(0, 3600)][int]$LockTimeoutSeconds = 300,
        [switch]$Force
    )

    if ($CommitSha -notmatch '^[0-9a-f]{40}$') {
        throw "CommitSha must be a lowercase 40-hex Git commit."
    }
    if ($PolicyVersion -notmatch '^[A-Za-z0-9._-]+$') {
        throw "PolicyVersion contains unsupported characters."
    }
    if ($StageNames.Count -eq 0) {
        throw "At least one preflight stage is required."
    }
    if ($InputFingerprint -ne "" -and $InputFingerprint -notmatch '^[0-9a-f]{64}$') { throw "InputFingerprint must be a SHA-256 digest." }
    if (@($StageNames | Select-Object -Unique).Count -ne $StageNames.Count) { throw "Duplicate preflight stages are not allowed." }
    foreach ($stageName in $StageNames) {
        if ($stageName -notmatch '^[A-Za-z0-9._-]+$') {
            throw "Stage name contains unsupported characters: $stageName"
        }
    }

    New-Item -ItemType Directory -Path $GitDirectory -Force | Out-Null
    $resolvedGitDirectory = (Resolve-Path -LiteralPath $GitDirectory).Path
    $lockPath = Join-Path $resolvedGitDirectory "dev-deploy-preflight.lock"
    $cacheBase = "dev-deploy-preflight-$PolicyVersion-$CommitSha"
    if ($InputFingerprint -ne "") { $cacheBase += "-" + $InputFingerprint.Substring(0, 16) }
    $cachePath = Join-Path $resolvedGitDirectory "$cacheBase.ok"
    $timingPath = "$cachePath.timings.json"
    $timings = [System.Collections.Generic.List[object]]::new()
    $started = [DateTime]::UtcNow
    $totalWatch = [Diagnostics.Stopwatch]::StartNew()
    $currentStage = ""
    $stageWatch = $null
    $status = "failed"

    function Test-StageReceipt([string]$Path, [string]$Stage) {
        if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) { return $false }
        $values = @{}
        foreach ($line in [IO.File]::ReadAllLines($Path)) {
            $pair = $line.Split('=', 2)
            if ($pair.Count -ne 2 -or $values.ContainsKey($pair[0])) { return $false }
            $values[$pair[0]] = $pair[1]
        }
        if ($values.commit -ne $CommitSha -or $values.policy -ne $PolicyVersion -or
            $values.stage -ne $Stage -or $values.fingerprint -ne $InputFingerprint) { return $false }
        $completed = [DateTimeOffset]::MinValue
        if (-not [DateTimeOffset]::TryParse($values.completed_at, [ref]$completed)) { return $false }
        $age = [DateTimeOffset]::UtcNow - $completed
        if ($age.TotalMinutes -lt -5) { return $false }
        if ($Stage -in @("audit", "govulncheck", "trivy") -and $age.TotalHours -gt $SecurityCacheHours) { return $false }
        return $true
    }
    $deadline = [DateTime]::UtcNow.AddSeconds($LockTimeoutSeconds)
    $lockHandle = $null

    while ($null -eq $lockHandle) {
        try {
            $lockHandle = [System.IO.File]::Open(
                $lockPath,
                [System.IO.FileMode]::OpenOrCreate,
                [System.IO.FileAccess]::ReadWrite,
                [System.IO.FileShare]::None
            )
        } catch [System.IO.IOException] {
            if ([DateTime]::UtcNow -ge $deadline) {
                throw "Another DEV preflight is already running for this worktree: $lockPath"
            }
            Start-Sleep -Milliseconds 250
        }
    }

    try {
        $completedStages = [System.Collections.Generic.List[string]]::new()
        foreach ($stage in $StageNames) {
            $currentStage = $stage
            $stageCachePath = Join-Path $resolvedGitDirectory "$cacheBase-$stage.ok"
            if (-not $Force -and (Test-StageReceipt $stageCachePath $stage)) {
                Write-Host "==> DEV preflight: $stage (cached for exact commit)"
                $timings.Add([pscustomobject]@{ stage = $stage; cached = $true; seconds = 0; result = "passed" })
                continue
            }

            Write-Host "==> DEV preflight: $stage"
            # A failed forced/expired refresh must never reuse an older success.
            if (Test-Path -LiteralPath $stageCachePath) { Remove-Item -LiteralPath $stageCachePath -Force }
            if (Test-Path -LiteralPath $cachePath) { Remove-Item -LiteralPath $cachePath -Force }
            $stageWatch = [Diagnostics.Stopwatch]::StartNew()
            & $RunStage $stage | Out-Host
            $seconds = [Math]::Round($stageWatch.Elapsed.TotalSeconds, 2)
            $timings.Add([pscustomobject]@{ stage = $stage; cached = $false; seconds = $seconds; result = "passed" })
            Write-Host ("DEV preflight {0}: {1:N1}s" -f $stage, $seconds)
            $completedStages.Add($stage)

            $stageTempPath = "$stageCachePath.$PID.tmp"
            $stageCacheContent = "commit=$CommitSha`npolicy=$PolicyVersion`nstage=$stage`nfingerprint=$InputFingerprint`ncompleted_at=$([DateTime]::UtcNow.ToString('O'))`n"
            [System.IO.File]::WriteAllText($stageTempPath, $stageCacheContent, [System.Text.UTF8Encoding]::new($false))
            Move-Item -LiteralPath $stageTempPath -Destination $stageCachePath -Force
        }

        $tempCachePath = "$cachePath.$PID.tmp"
        $cacheContent = "commit=$CommitSha`npolicy=$PolicyVersion`ncompleted_at=$([DateTime]::UtcNow.ToString('O'))`n"
        [System.IO.File]::WriteAllText($tempCachePath, $cacheContent, [System.Text.UTF8Encoding]::new($false))
        Move-Item -LiteralPath $tempCachePath -Destination $cachePath -Force

        $status = "passed"
        return [pscustomobject]@{
            Cached = ($completedStages.Count -eq 0)
            CachePath = $cachePath
            StagesRun = $completedStages.ToArray()
            TimingPath = $timingPath
        }
    } catch {
        if ($null -ne $stageWatch -and $currentStage -ne "") {
            $timings.Add([pscustomobject]@{ stage = $currentStage; cached = $false; seconds = [Math]::Round($stageWatch.Elapsed.TotalSeconds, 2); result = "failed" })
        }
        throw
    } finally {
        $receipt = [ordered]@{
            commit = $CommitSha; policy = $PolicyVersion; fingerprint = $InputFingerprint
            started_at = $started.ToString('O'); completed_at = [DateTime]::UtcNow.ToString('O')
            status = $status; total_seconds = [Math]::Round($totalWatch.Elapsed.TotalSeconds, 2)
            stages = $timings.ToArray()
        }
        [IO.File]::WriteAllText($timingPath, ($receipt | ConvertTo-Json -Depth 5), [Text.UTF8Encoding]::new($false))
        if ($null -ne $lockHandle) {
            $lockHandle.Dispose()
        }
    }
}

Export-ModuleMember -Function Invoke-DevDeployPreflight
