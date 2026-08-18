Set-StrictMode -Version Latest

function Invoke-DevDeployPreflight {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)][string]$GitDirectory,
        [Parameter(Mandatory = $true)][string]$CommitSha,
        [Parameter(Mandatory = $true)][string]$PolicyVersion,
        [Parameter(Mandatory = $true)][string[]]$StageNames,
        [Parameter(Mandatory = $true)][scriptblock]$RunStage,
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

    New-Item -ItemType Directory -Path $GitDirectory -Force | Out-Null
    $resolvedGitDirectory = (Resolve-Path -LiteralPath $GitDirectory).Path
    $lockPath = Join-Path $resolvedGitDirectory "dev-deploy-preflight.lock"
    $cachePath = Join-Path $resolvedGitDirectory "dev-deploy-preflight-$PolicyVersion-$CommitSha.ok"
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
        if (-not $Force -and (Test-Path -LiteralPath $cachePath -PathType Leaf)) {
            Write-Host "DEV preflight already passed for $CommitSha (policy $PolicyVersion)."
            return [pscustomobject]@{
                Cached = $true
                CachePath = $cachePath
                StagesRun = @()
            }
        }

        $completedStages = [System.Collections.Generic.List[string]]::new()
        foreach ($stage in $StageNames) {
            Write-Host "==> DEV preflight: $stage"
            & $RunStage $stage | Out-Host
            $completedStages.Add($stage)
        }

        $tempCachePath = "$cachePath.$PID.tmp"
        $cacheContent = "commit=$CommitSha`npolicy=$PolicyVersion`ncompleted_at=$([DateTime]::UtcNow.ToString('O'))`n"
        [System.IO.File]::WriteAllText($tempCachePath, $cacheContent, [System.Text.UTF8Encoding]::new($false))
        Move-Item -LiteralPath $tempCachePath -Destination $cachePath -Force

        return [pscustomobject]@{
            Cached = $false
            CachePath = $cachePath
            StagesRun = $completedStages.ToArray()
        }
    } finally {
        if ($null -ne $lockHandle) {
            $lockHandle.Dispose()
        }
    }
}

Export-ModuleMember -Function Invoke-DevDeployPreflight
