[CmdletBinding()]
param()

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$modulePath = Join-Path $PSScriptRoot "TrivyFilesystemScope.psm1"
if (-not (Test-Path -LiteralPath $modulePath -PathType Leaf)) {
    throw "Trivy filesystem exclusion guard is missing."
}
Import-Module $modulePath -Force
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "../..")).Path
$configPath = Join-Path $repoRoot "trivy-filesystem.yaml"

function Assert-ThrowsLike {
    param([scriptblock]$Action, [string]$Pattern, [string]$Description)
    try { & $Action | Out-Null } catch {
        if ($_.Exception.Message -notlike $Pattern) {
            throw "$Description threw an unexpected error: $($_.Exception.Message)"
        }
        return
    }
    throw "$Description did not fail closed."
}

$safeTrackedFiles = @(
    "go.mod", "go.sum", "Dockerfile.api", "Dockerfile.platform",
    "docker-compose.dev.yml", "deployments/nginx/dev-web.conf",
    "internal/service/example.go", "web/platform/src/app/page.tsx",
    "web/platform/package-lock.json", "web/admin/package-lock.json",
    "web/admin/package-lock.json", "scripts/ci/example.ps1",
    ".cache-not-generated/source.go", "web/platform/.next-not-generated/source.ts",
    "docs/node_modules-guide.md", "web/admin/distribution/source.ts",
    "references/README.md", "references/not-local/source.go"
)
Assert-TrivyFilesystemScanScope -RepositoryRoot $repoRoot -ConfigPath $configPath -TrackedFiles $safeTrackedFiles

foreach ($hiddenSource in @(
    ".runtime/source.go", ".cache/package-lock.json", "node_modules/private-package/index.js",
    "web/platform/node_modules/private-package/package.json",
    "web/platform/.next/source.ts", "web/admin/dist/source.ts",
    "web/admin/dist/config.yaml", "coverage/Dockerfile",
    "web/platform/coverage/source.ts", "web/admin/coverage/source.ts",
    "web/admin/coverage/source.ts", "web/platform/playwright-report/source.ts",
    "web/platform/test-results/source.ts",
    "references/magnific-html/source.js", "references/magnific-html-js/source.js",
    "references/magnific-login-ru/source.js", "references/magnific-reference-assets/source.js",
    "references/magnific-states/source.js", "references/magnific-html-24-pages.zip"
)) {
    Assert-ThrowsLike {
        Assert-TrivyFilesystemScanScope -RepositoryRoot $repoRoot -ConfigPath $configPath -TrackedFiles @($hiddenSource)
    } "*would hide tracked files*" "tracked file $hiddenSource"
}

$tempRoot = Join-Path ([IO.Path]::GetTempPath()) ("trivy-scope-" + [guid]::NewGuid().ToString("N"))
try {
    New-Item -ItemType Directory -Path $tempRoot | Out-Null
    $invalidConfigPath = Join-Path $tempRoot "invalid.yaml"
    foreach ($invalidConfig in @(
        "scan:`n  skip-dirs:`n    - internal",
        "scan:`n  skip-dirs:`n    - `"**/dist`"",
        "scan:`n  skip-dirs:`n    - ../outside",
        "scan:`n  skip-dirs:`n    - /src/internal",
        "scan:`n  skip-dirs:`n    - .cache`n    - .cache",
        "scan:`n  skip-dirs:`n    - .cache`nignore-unfixed: true",
        "scan:`n  skip-files:`n    - go.mod",
        "scan:`n  skip-dirs:`n    - .cache`n  skip-files:`n    - go.mod",
        "scan:`n  skip-dirs:`n    - .cache`n  skip-files:`n    - `"references/*`"",
        "scan:`n  skip-dirs:`n    - .cache`n  skip-files:`n    - `"references/*.zip`"`n    - `"references/*.zip`"",
        "scan:`n  skip-dirs:`n    - .cache`n  skip-files:",
        "scan:`n  skip-dirs:`n"
    )) {
        [IO.File]::WriteAllText($invalidConfigPath, $invalidConfig, [Text.UTF8Encoding]::new($false))
        Assert-ThrowsLike {
            Assert-TrivyFilesystemScanScope -RepositoryRoot $repoRoot -ConfigPath $invalidConfigPath -TrackedFiles $safeTrackedFiles
        } "*Invalid Trivy filesystem exclusion config*" "unsafe configuration"
    }

    # Exercise git ls-files itself, including newly staged files, without
    # changing the user's checkout or committing a fixture repository.
    $fixtureCache = Join-Path $tempRoot ".cache"
    New-Item -ItemType Directory -Path $fixtureCache | Out-Null
    [IO.File]::WriteAllText((Join-Path $fixtureCache "tracked.go"), "package fixture`n")
    & git -C $tempRoot -c init.defaultBranch=scope-test init --quiet
    if ($LASTEXITCODE -ne 0) { throw "Could not initialize the tracked-file fixture." }
    & git -C $tempRoot -c core.autocrlf=false add --force .cache/tracked.go
    if ($LASTEXITCODE -ne 0) { throw "Could not stage the excluded-file fixture." }
    Assert-ThrowsLike {
        Assert-TrivyFilesystemScanScope -RepositoryRoot $tempRoot -ConfigPath $configPath
    } "*would hide tracked files*" "real staged Git file"
} finally {
    $resolvedTemp = [IO.Path]::GetFullPath($tempRoot)
    $tempParent = [IO.Path]::GetFullPath([IO.Path]::GetTempPath()).TrimEnd([IO.Path]::DirectorySeparatorChar) + [IO.Path]::DirectorySeparatorChar
    if (-not $resolvedTemp.StartsWith($tempParent, [StringComparison]::OrdinalIgnoreCase) -or
        [IO.Path]::GetFileName($resolvedTemp) -notlike "trivy-scope-*") {
        throw "Refusing to remove a fixture directory outside its temporary root."
    }
    if (Test-Path -LiteralPath $resolvedTemp) {
        if ((Get-Item -LiteralPath $resolvedTemp).Attributes -band [IO.FileAttributes]::ReparsePoint) {
            throw "Refusing to remove a reparse-point fixture directory."
        }
        Remove-Item -LiteralPath $resolvedTemp -Recurse -Force
    }
}

Assert-TrivyFilesystemScanScope -RepositoryRoot $repoRoot -ConfigPath $configPath
Write-Host "Trivy filesystem exclusion scope tests passed."
