Set-StrictMode -Version Latest

function Get-TrivyFilesystemExclusions {
    [CmdletBinding()]
    param([Parameter(Mandatory = $true)][string]$ConfigPath)

    # Only this narrow YAML subset is allowed: no extra parser dependency and no
    # configuration keys that can weaken the existing vulnerability gate.
    $allowedDirectories = @(
        ".runtime", ".cache", "node_modules", "**/node_modules",
        "web/platform/.next", "web/admin/dist", "coverage",
        "web/platform/coverage", "web/admin/coverage",
        "web/platform/playwright-report", "web/platform/test-results",
        "references/magnific-html", "references/magnific-html-js",
        "references/magnific-login-ru", "references/magnific-reference-assets",
        "references/magnific-states"
    )
    $allowedFiles = @("references/*.zip")
    if (-not (Test-Path -LiteralPath $ConfigPath -PathType Leaf)) {
        throw "Invalid Trivy filesystem exclusion config: file is missing."
    }
    $lines = @(Get-Content -LiteralPath $ConfigPath | Where-Object {
        -not [string]::IsNullOrWhiteSpace($_) -and $_ -notmatch '^\s*#'
    })
    if ($lines.Count -lt 3 -or $lines[0] -cne "scan:" -or $lines[1] -cne "  skip-dirs:") {
        throw "Invalid Trivy filesystem exclusion config: scan.skip-dirs must be first."
    }
    $directories = [Collections.Generic.List[string]]::new()
    $files = [Collections.Generic.List[string]]::new()
    $fileSection = $false
    foreach ($line in $lines[2..($lines.Count - 1)]) {
        if ($line -ceq "  skip-files:") {
            if ($fileSection -or $directories.Count -eq 0) {
                throw "Invalid Trivy filesystem exclusion config: duplicate or empty section."
            }
            $fileSection = $true
            continue
        }
        if ($line -notmatch '^    - (?:(?:"(?<directory>[^"]+)")|(?<directory>[A-Za-z0-9._/-]+))$') {
            throw "Invalid Trivy filesystem exclusion config: unsupported YAML entry."
        }
        $directory = $Matches.directory
        if ($fileSection) {
            if ($allowedFiles -cnotcontains $directory -or $files.Contains($directory)) {
                throw "Invalid Trivy filesystem exclusion config: unexpected or duplicate file."
            }
            $files.Add($directory)
        } elseif ($allowedDirectories -cnotcontains $directory -or $directories.Contains($directory)) {
            throw "Invalid Trivy filesystem exclusion config: unexpected or duplicate directory."
        } else {
            $directories.Add($directory)
        }
    }
    if ($directories.Count -eq 0 -or ($fileSection -and $files.Count -eq 0)) {
        throw "Invalid Trivy filesystem exclusion config: empty section."
    }
    return @{ Directories = $directories.ToArray(); Files = $files.ToArray() }
}

function Get-TrivyFilesystemSkipDirectories {
    [CmdletBinding()]
    param([Parameter(Mandatory = $true)][string]$ConfigPath)
    return (Get-TrivyFilesystemExclusions -ConfigPath $ConfigPath).Directories
}

function Assert-TrivyFilesystemScanScope {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)][string]$RepositoryRoot,
        [Parameter(Mandatory = $true)][string]$ConfigPath,
        [AllowEmptyCollection()][string[]]$TrackedFiles
    )

    $exclusions = Get-TrivyFilesystemExclusions -ConfigPath $ConfigPath
    $skipDirectories = @($exclusions.Directories)
    if (-not $PSBoundParameters.ContainsKey("TrackedFiles")) {
        $gitOutput = @(& git -c core.quotepath=false -C $RepositoryRoot ls-files --cached -z)
        if ($LASTEXITCODE -ne 0) { throw "Cannot validate Trivy exclusions: git ls-files failed." }
        $TrackedFiles = @(($gitOutput -join "`n") -split "`0" | Where-Object { $_.Length -gt 0 })
    }
    $hiddenFiles = [Collections.Generic.List[string]]::new()
    foreach ($file in $TrackedFiles) {
        $path = $file.Replace("\", "/")
        if ($path -match '[\r\n]' -or $path.StartsWith("/") -or $path -match '(^|/)\.\.(/|$)') {
            throw "Cannot validate Trivy exclusions: unsupported tracked path."
        }
        foreach ($directory in $skipDirectories) {
            $hidden = if ($directory -eq "**/node_modules") {
                $path -match '(^|/)node_modules(/|$)'
            } else {
                $path.Equals($directory, [StringComparison]::OrdinalIgnoreCase) -or
                    $path.StartsWith("$directory/", [StringComparison]::OrdinalIgnoreCase)
            }
            if ($hidden) { $hiddenFiles.Add($path); break }
        }
        # Deliberately more conservative than the root-only Trivy glob: a
        # tracked ZIP anywhere below references must never disappear from scans.
        if ($exclusions.Files -contains "references/*.zip" -and
            $path.StartsWith("references/", [StringComparison]::OrdinalIgnoreCase) -and
            $path.EndsWith(".zip", [StringComparison]::OrdinalIgnoreCase)) {
            $hiddenFiles.Add($path)
        }
    }
    if ($hiddenFiles.Count -gt 0) {
        throw "Trivy filesystem exclusions would hide tracked files: $($hiddenFiles -join ', ')"
    }
}

Export-ModuleMember -Function Get-TrivyFilesystemSkipDirectories, Assert-TrivyFilesystemScanScope
