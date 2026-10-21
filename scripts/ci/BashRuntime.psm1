Set-StrictMode -Version Latest

function Get-CIBashExecutable {
    if ($IsWindows) {
        $windowsDirectory = [Environment]::GetFolderPath([Environment+SpecialFolder]::Windows)
        $wslExecutable = Join-Path $windowsDirectory "System32/wsl.exe"
        if (-not (Test-Path -LiteralPath $wslExecutable -PathType Leaf)) {
            throw "WSL Bash is required for Windows CI checks. Install WSL and prepare a Linux distribution with Bash, curl and jq."
        }
        return $wslExecutable
    }
    $bashCommand = Get-Command bash -CommandType Application -ErrorAction SilentlyContinue
    if ($null -eq $bashCommand) { throw "Bash is required for CI checks on this platform." }
    return $bashCommand.Source
}

function Invoke-CIBashCommand {
    param([string]$WorkingDirectory, [string[]]$BashArguments)

    $executable = Get-CIBashExecutable
    $nativeArguments = if ($IsWindows) {
        @("--cd", $WorkingDirectory, "--exec", "bash", "--noprofile", "--norc") + $BashArguments
    } else {
        @("--noprofile", "--norc") + $BashArguments
    }
    Push-Location -LiteralPath $WorkingDirectory
    try {
        $output = @(& $executable @nativeArguments 2>&1)
        $exitCode = $LASTEXITCODE
    } finally { Pop-Location }
    return [pscustomobject]@{
        ExitCode = $exitCode
        Output = @($output | ForEach-Object { [string]$_ })
    }
}

function Assert-CIBashEnvironment {
    [CmdletBinding()]
    param([ValidatePattern('^[A-Za-z0-9_-]+$')][string[]]$RequiredCommands = @("bash", "curl", "jq"))

    $probe = 'for required_command; do command -v "$required_command" >/dev/null 2>&1 || { printf "Missing Bash prerequisite: %s\n" "$required_command" >&2; exit 127; }; done'
    $result = Invoke-CIBashCommand -WorkingDirectory (Get-Location).Path -BashArguments (@("-c", $probe, "ci-bash-prerequisites") + $RequiredCommands)
    if ($result.ExitCode -ne 0) {
        $runtime = if ($IsWindows) { "WSL Bash in the default Linux distribution" } else { "Bash" }
        throw "$runtime is unavailable or missing prerequisites. Prepare Bash, curl and jq before running checks. $($result.Output -join ' ')"
    }
}

function ConvertTo-CIBashPath {
    [CmdletBinding()]
    param([Parameter(Mandatory = $true)][string]$Path)

    $resolved = (Resolve-Path -LiteralPath $Path -ErrorAction Stop).Path
    if (-not $IsWindows) { return $resolved }
    $wslExecutable = Get-CIBashExecutable
    $output = @(& $wslExecutable --exec wslpath -a -u $resolved 2>&1)
    if ($LASTEXITCODE -ne 0 -or $output.Count -ne 1 -or -not ([string]$output[0]).StartsWith("/")) {
        throw "Cannot convert a Windows check fixture path with WSL wslpath. Check the default Linux distribution."
    }
    return [string]$output[0]
}

function Invoke-CIBashScript {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)][string]$RepositoryRoot,
        [Parameter(Mandatory = $true)][string]$ScriptPath,
        [AllowEmptyCollection()][string[]]$ScriptArguments = @()
    )

    $root = (Resolve-Path -LiteralPath $RepositoryRoot -ErrorAction Stop).Path
    $candidate = if ([IO.Path]::IsPathRooted($ScriptPath)) { $ScriptPath } else { Join-Path $root $ScriptPath }
    $script = (Resolve-Path -LiteralPath $candidate -ErrorAction Stop).Path
    if (-not (Test-Path -LiteralPath $script -PathType Leaf)) { throw "Bash check script is not a file." }
    $relativeScript = [IO.Path]::GetRelativePath($root, $script).Replace("\", "/")
    if ([IO.Path]::IsPathRooted($relativeScript) -or $relativeScript -eq ".." -or $relativeScript.StartsWith("../")) {
        throw "Bash check scripts must belong to the specified repository root."
    }
    return Invoke-CIBashCommand -WorkingDirectory $root -BashArguments (@($relativeScript) + $ScriptArguments)
}

Export-ModuleMember -Function Assert-CIBashEnvironment, ConvertTo-CIBashPath, Invoke-CIBashScript
