[CmdletBinding()]
param([string]$BaseRef = 'origin/dev-deploy')

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$watch = [Diagnostics.Stopwatch]::StartNew()
Push-Location $repoRoot
try {
    $json = & node scripts/ci/release-source-plan.mjs --base $BaseRef
    if ($LASTEXITCODE -ne 0) { throw 'Cannot select early source checks' }
    $plan = ($json | Out-String) | ConvertFrom-Json
    Write-Host "Early source checks: $($plan.changedFiles.Count) changed paths; $($plan.commands.Count) checks"
    foreach ($step in $plan.commands) {
        $command = [string]$step.command
        if ($command -eq 'npm' -and $IsWindows) { $command = 'npm.cmd' }
        if (-not (Get-Command $command -ErrorAction SilentlyContinue)) { throw "Missing early-check prerequisite: $command" }
        if ($step.cwd -like 'web/*') {
            $executable = if ($IsWindows) { 'eslint.cmd' } else { 'eslint' }
            if (-not (Test-Path -LiteralPath (Join-Path $repoRoot "$($step.cwd)/node_modules/.bin/$executable"))) {
                throw "Install dependencies first: npm --prefix $($step.cwd) ci"
            }
        }
        Write-Host ("==> source-fast [$($step.cwd)]: $command " + ($step.args -join ' '))
        Push-Location (Join-Path $repoRoot $step.cwd)
        try {
            $arguments = [string[]]$step.args
            if ($step.PSObject.Properties.Name -contains 'expectEmptyOutput' -and $step.expectEmptyOutput) {
                $output = & $command @arguments
                if ($LASTEXITCODE -ne 0) { throw "Early source check failed: $command (exit $LASTEXITCODE)" }
                if (-not [string]::IsNullOrWhiteSpace(($output | Out-String))) { throw 'Go formatting differs; run gofmt on the changed Go files' }
            } else {
                & $command @arguments
                if ($LASTEXITCODE -ne 0) { throw "Early source check failed: $command (exit $LASTEXITCODE)" }
            }
        } finally { Pop-Location }
    }
    Write-Host ("Early source checks passed in {0:N1}s; complete commit-scoped validation is still required." -f $watch.Elapsed.TotalSeconds)
} finally { Pop-Location }
