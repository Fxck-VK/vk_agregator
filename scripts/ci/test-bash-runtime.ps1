[CmdletBinding()]
param()

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest
$modulePath = Join-Path $PSScriptRoot "BashRuntime.psm1"
if (-not (Test-Path -LiteralPath $modulePath -PathType Leaf)) {
    throw "The explicit CI Bash runtime is missing."
}
Import-Module $modulePath -Force

function Assert-Equal($Expected, $Actual, [string]$Description) {
    if ($Expected -cne $Actual) { throw "$Description expected=[$Expected] actual=[$Actual]" }
}

$resolvedExecutable = & (Get-Module BashRuntime) { Get-CIBashExecutable }
Assert-Equal 1 @($resolvedExecutable).Count "exactly one Bash runtime is selected"
if (-not [IO.Path]::IsPathRooted($resolvedExecutable) -or -not (Test-Path -LiteralPath $resolvedExecutable -PathType Leaf)) {
    throw "The selected Bash runtime must be one existing absolute executable path."
}

$tempParent = [IO.Path]::GetFullPath([IO.Path]::GetTempPath()).TrimEnd([IO.Path]::DirectorySeparatorChar) + [IO.Path]::DirectorySeparatorChar
$tempRoot = Join-Path $tempParent ("bash-runtime-" + [guid]::NewGuid().ToString("N"))
$savedRuntimePath = $env:PATH
try {
    $fixtureRoot = Join-Path $tempRoot "directory with spaces/кириллица"
    New-Item -ItemType Directory -Path $fixtureRoot -Force | Out-Null
    $scriptPath = Join-Path $fixtureRoot "probe with spaces.sh"
    $probe = @'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$1"
if [[ ! -f "$3" ]]; then
  echo "Converted file argument was not accessible" >&2
  exit 12
fi
exit "$2"
'@
    [IO.File]::WriteAllText($scriptPath, $probe.Replace("`r`n", "`n") + "`n", [Text.UTF8Encoding]::new($false))
    $fixtureFile = Join-Path $fixtureRoot "input with spaces.txt"
    [IO.File]::WriteAllText($fixtureFile, "fixture`n")
    $bashFixturePath = ConvertTo-CIBashPath -Path $fixtureFile

    if ($IsWindows) {
        $fakeBash = Join-Path $tempRoot "bash.cmd"
        [IO.File]::WriteAllText($fakeBash, "@echo WRONG_BASH_SELECTED`r`n@exit /b 88`r`n")
        $env:PATH = $tempRoot + [IO.Path]::PathSeparator + $savedRuntimePath
    }
    Assert-CIBashEnvironment -RequiredCommands @("bash", "curl", "jq")
    $payload = 'argument with spaces, apostrophe '' and $ literal; text'
    $passed = Invoke-CIBashScript -RepositoryRoot $fixtureRoot -ScriptPath $scriptPath -ScriptArguments @($payload, "0", $bashFixturePath)
    Assert-Equal 0 $passed.ExitCode "successful script exit code"
    Assert-Equal $payload ($passed.Output -join "`n") "literal arguments and converted paths"

    $failed = Invoke-CIBashScript -RepositoryRoot $fixtureRoot -ScriptPath $scriptPath -ScriptArguments @($payload, "9", $bashFixturePath)
    Assert-Equal 9 $failed.ExitCode "failed Bash script exit code is preserved"

    $missingCommandReported = $false
    try { Assert-CIBashEnvironment -RequiredCommands @("ci-bash-prerequisite-that-does-not-exist") } catch {
        if ($_.Exception.Message -notlike "*Missing Bash prerequisite: ci-bash-prerequisite-that-does-not-exist*") { throw }
        $missingCommandReported = $true
    }
    Assert-Equal $true $missingCommandReported "missing prerequisites fail clearly"

    $outsideScript = Join-Path $tempRoot "outside.sh"
    [IO.File]::WriteAllText($outsideScript, "exit 0`n")
    $outsideScriptRejected = $false
    try {
        Invoke-CIBashScript -RepositoryRoot $fixtureRoot -ScriptPath $outsideScript | Out-Null
    } catch {
        if ($_.Exception.Message -notlike "*must belong to the specified repository root*") { throw }
        $outsideScriptRejected = $true
    }
    Assert-Equal $true $outsideScriptRejected "scripts outside the specified root are rejected"
} finally {
    $env:PATH = $savedRuntimePath
    $resolvedTemp = [IO.Path]::GetFullPath($tempRoot)
    if (-not $resolvedTemp.StartsWith($tempParent, [StringComparison]::OrdinalIgnoreCase) -or
        [IO.Path]::GetFileName($resolvedTemp) -notlike "bash-runtime-*") {
        throw "Refusing fixture cleanup outside the temporary Bash runtime directory."
    }
    if (Test-Path -LiteralPath $resolvedTemp) {
        if ((Get-Item -LiteralPath $resolvedTemp).Attributes -band [IO.FileAttributes]::ReparsePoint) {
            throw "Refusing to delete a reparse-point fixture root."
        }
        Remove-Item -LiteralPath $resolvedTemp -Recurse -Force
    }
}

Write-Host "CI Bash runtime tests passed."
# Expected native failures above must not become the GitHub pwsh wrapper's exit.
exit 0
