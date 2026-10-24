[CmdletBinding()]
param(
    [ValidateRange(0, 3600)][int]$LockTimeoutSeconds = 300,
    [string]$ManifestPath = ".cache/dev-release-manifest.json",
    [switch]$Force
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
Import-Module (Join-Path $PSScriptRoot "DevDeployPreflight.psm1") -Force
Import-Module (Join-Path $PSScriptRoot "TrivyFilesystemScope.psm1") -Force
Import-Module (Join-Path $PSScriptRoot "BashRuntime.psm1") -Force
Set-Location $repoRoot

function Invoke-Native {
    param(
        [Parameter(Mandatory = $true)][string]$Command,
        [Parameter(Mandatory = $true)][string[]]$Arguments
    )

    Write-Host ("+ " + $Command + " " + ($Arguments -join " "))
    & $Command @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "Command failed with exit code $LASTEXITCODE`: $Command $($Arguments -join ' ')"
    }
}

function Get-GitOutput {
    param([Parameter(Mandatory = $true)][string[]]$Arguments)
    $output = & git @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "git $($Arguments -join ' ') failed"
    }
    return ($output | Out-String).Trim()
}

function Install-NpmDependenciesIfNeeded {
    param([Parameter(Mandatory = $true)][string]$PackagePath)

    $lockfilePath = Join-Path $repoRoot "$PackagePath/package-lock.json"
    $eslintExecutable = if ($IsWindows) {
        Join-Path $repoRoot "$PackagePath/node_modules/.bin/eslint.cmd"
    } else {
        Join-Path $repoRoot "$PackagePath/node_modules/.bin/eslint"
    }
    $lockHash = (Get-FileHash -LiteralPath $lockfilePath -Algorithm SHA256).Hash.ToLowerInvariant()
    $dependencyCacheDirectory = Join-Path $gitDirectory "dev-deploy-npm"
    $packageCacheName = $PackagePath.Replace("/", "-").Replace("\\", "-")
    $dependencyMarker = Join-Path $dependencyCacheDirectory "$packageCacheName-$lockHash.ok"

    if ((Test-Path -LiteralPath $dependencyMarker -PathType Leaf) -and
        (Test-Path -LiteralPath $eslintExecutable -PathType Leaf)) {
        Write-Host "npm dependencies already match $PackagePath/package-lock.json"
        return
    }

    Invoke-Native -Command $npmCommand -Arguments @("--prefix", $PackagePath, "ci")
    New-Item -ItemType Directory -Path $dependencyCacheDirectory -Force | Out-Null
    [System.IO.File]::WriteAllText(
        $dependencyMarker,
        "lock_sha256=$lockHash`ncompleted_at=$([DateTime]::UtcNow.ToString('O'))`n",
        [System.Text.UTF8Encoding]::new($false)
    )
}

$branch = Get-GitOutput -Arguments @("rev-parse", "--abbrev-ref", "HEAD")
$upstream = ""
try {
    $upstream = Get-GitOutput -Arguments @("rev-parse", "--abbrev-ref", "@{upstream}")
} catch {
    $upstream = ""
}
if ($branch -ne "dev-deploy" -and $upstream -ne "origin/dev-deploy") {
    throw "DEV preflight is restricted to dev-deploy or a branch tracking origin/dev-deploy; branch=$branch upstream=$upstream"
}

$commitSha = Get-GitOutput -Arguments @("rev-parse", "HEAD")
# Verify the agreed inventory even when the exact-commit test cache is warm.
Invoke-Native -Command "node" -Arguments @("scripts/ci/release-manifest.mjs", "--check", $ManifestPath, "--commit", $commitSha)

$worktreeChanges = Get-GitOutput -Arguments @("status", "--porcelain", "--untracked-files=all")
if (-not [string]::IsNullOrWhiteSpace($worktreeChanges)) {
    throw "DEV preflight requires a completely clean worktree. Commit, stash, or remove local changes first."
}

# Validate before the stage cache is consulted.
Assert-TrivyFilesystemScanScope -RepositoryRoot $repoRoot -ConfigPath (Join-Path $repoRoot "trivy-filesystem.yaml")
Assert-CIBashEnvironment
foreach ($required in @("go", "node", "npm", "pwsh", "docker")) {
    if (-not (Get-Command $required -ErrorAction SilentlyContinue)) { throw "Missing preflight prerequisite: $required" }
}
$dockerVersion = & docker version --format '{{.Server.Version}}' 2>$null
if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($dockerVersion)) {
    throw "Docker engine must be running before preflight; no expensive checks have started."
}

$gitDirectory = Get-GitOutput -Arguments @("rev-parse", "--absolute-git-dir")
$npmCommand = if ($IsWindows) { "npm.cmd" } else { "npm" }
$policyVersion = "v5"
# Fail on source contracts and fresh advisories before lengthy source builds/tests.
$stageNames = @("release-contract", "audit", "govulncheck", "infrastructure", "trivy", "tests")
function Get-RuntimeVersion([string]$Command, [string[]]$Arguments) {
    $version = (& $Command @Arguments | Out-String).Trim()
    if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($version)) {
        throw "Cannot determine preflight runtime version: $Command"
    }
    return $version
}
$runtimeInputs = [ordered]@{
    go = Get-RuntimeVersion "go" @("env", "GOVERSION")
    node = Get-RuntimeVersion "node" @("--version")
    npm = Get-RuntimeVersion $npmCommand @("--version")
    docker = [string]$dockerVersion
    powershell = $PSVersionTable.PSVersion.ToString()
    os = [Environment]::OSVersion.ToString()
    settings = [ordered]@{}
}
foreach ($name in @("CI", "NODE_ENV", "NODE_OPTIONS", "GOTOOLCHAIN", "GOFLAGS", "GOWORK", "CGO_ENABLED", "VITEST_MAX_WORKERS")) {
    $runtimeInputs.settings[$name] = [Environment]::GetEnvironmentVariable($name)
}
$inputBytes = [Text.Encoding]::UTF8.GetBytes(($runtimeInputs | ConvertTo-Json -Depth 4 -Compress))
$inputFingerprint = [Convert]::ToHexString([Security.Cryptography.SHA256]::HashData($inputBytes)).ToLowerInvariant()

$runStage = {
    param([string]$Stage)

    switch ($Stage) {
        "release-contract" {
            & pwsh -NoProfile -File scripts/ci/validate-release-contract.ps1
            if ($LASTEXITCODE -ne 0) { throw "Shared release contract failed before source tests" }
        }
        "tests" {
            Invoke-Native -Command "git" -Arguments @("diff", "--check", "$commitSha^", $commitSha)
            Invoke-Native -Command "go" -Arguments @("test", "./...")
            Invoke-Native -Command "go" -Arguments @("vet", "./...")
            Invoke-Native -Command "go" -Arguments @("run", "./scripts/models/check")
            Invoke-Native -Command "go" -Arguments @("run", "./scripts/models/preview", "-check")
            foreach ($packagePath in @("web/admin", "web/platform")) {
                Install-NpmDependenciesIfNeeded -PackagePath $packagePath
                Invoke-Native -Command $npmCommand -Arguments @("--prefix", $packagePath, "run", "lint")
                Invoke-Native -Command $npmCommand -Arguments @("--prefix", $packagePath, "run", "typecheck")
                Invoke-Native -Command $npmCommand -Arguments @("--prefix", $packagePath, "run", "test")
                Invoke-Native -Command $npmCommand -Arguments @("--prefix", $packagePath, "run", "build")
            }
            Invoke-Native -Command $npmCommand -Arguments @("--prefix", "web/platform", "run", "test:packaging")
        }
        "audit" {
            foreach ($lockfile in @(
                "web/admin/package-lock.json",
                "web/platform/package-lock.json"
            )) {
                Invoke-Native -Command "node" -Arguments @("scripts/ci/validate-npm-lockfiles.mjs", $lockfile)
            }
            foreach ($packagePath in @("web/admin", "web/platform")) {
                Invoke-Native -Command $npmCommand -Arguments @("--prefix", $packagePath, "audit", "--audit-level=moderate")
            }
        }
        "govulncheck" {
            if (Get-Command govulncheck -ErrorAction SilentlyContinue) {
                Invoke-Native -Command "govulncheck" -Arguments @("./...")
            } else {
                Invoke-Native -Command "go" -Arguments @("run", "golang.org/x/vuln/cmd/govulncheck@v1.6.0", "./...")
            }
        }
        "infrastructure" {
            & pwsh -NoProfile -File scripts/ci/test-next-route-discovery.ps1
            if ($LASTEXITCODE -ne 0) { throw "Next.js route discovery tests failed" }
            & pwsh -NoProfile -File scripts/ci/test-dev-deploy-preflight.ps1
            if ($LASTEXITCODE -ne 0) { throw "DEV preflight regression tests failed" }
            & pwsh -NoProfile -File scripts/ci/test-trivy-filesystem-scope.ps1
            if ($LASTEXITCODE -ne 0) { throw "Trivy filesystem scope regression tests failed" }
            & pwsh -NoProfile -File scripts/ci/test-bash-runtime.ps1
            if ($LASTEXITCODE -ne 0) { throw "Bash runtime regression tests failed" }
            & pwsh -NoProfile -File scripts/ci/test-docs-validation.ps1
            if ($LASTEXITCODE -ne 0) { throw "Documentation regression tests failed" }
            & pwsh -NoProfile -File scripts/ci/validate-docs.ps1
            if ($LASTEXITCODE -ne 0) { throw "Documentation validation failed" }
            Invoke-Native -Command "node" -Arguments @("--test", "scripts/ci/release-manifest.test.mjs")
            foreach ($scriptPath in @("scripts/ci/test-retry-network.sh", "scripts/ci/test-validate-dev-release.sh")) {
                $scriptResult = Invoke-CIBashScript -RepositoryRoot $repoRoot -ScriptPath $scriptPath
                $scriptResult.Output | Out-Host
                if ($scriptResult.ExitCode -ne 0) { throw "Release reliability regression failed: $scriptPath" }
            }
            $bashResult = Invoke-CIBashScript -RepositoryRoot $repoRoot -ScriptPath "scripts/ci/test-wait-for-github-workflow.sh"
            $bashResult.Output | Out-Host
            if ($bashResult.ExitCode -ne 0) { throw "Bash workflow monitor tests failed with exit code $($bashResult.ExitCode)" }
            & node scripts/ci/check-retired-miniapp.mjs
            if ($LASTEXITCODE -ne 0) { throw "Mini App retirement check failed" }
            & pwsh -NoProfile -File scripts/ci/validate-infra.ps1
            if ($LASTEXITCODE -ne 0) { throw "Infrastructure validation failed" }
            & pwsh -NoProfile -File scripts/ci/validate-nightly-quality.ps1
            if ($LASTEXITCODE -ne 0) { throw "Nightly quality validation failed" }
        }
        "trivy" {
            if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
                throw "Docker is required for the pinned Trivy preflight scan."
            }
            $trivyCache = Join-Path $gitDirectory "trivy-cache"
            New-Item -ItemType Directory -Path $trivyCache -Force | Out-Null
            $repoMount = "type=bind,src=$repoRoot,dst=/src,readonly"
            $cacheMount = "type=bind,src=$trivyCache,dst=/root/.cache/trivy"
            Invoke-Native -Command "docker" -Arguments @(
                "run", "--rm",
                "--mount", $repoMount,
                "--mount", $cacheMount,
                "--workdir", "/src",
                "aquasec/trivy@sha256:cffe3f5161a47a6823fbd23d985795b3ed72a4c806da4c4df16266c02accdd6f",
                "fs",
                "--scanners", "vuln,misconfig",
                "--severity", "HIGH,CRITICAL",
                "--exit-code", "1",
                "--no-progress",
                "--config", "/src/trivy-filesystem.yaml",
                "."
            )
        }
        default {
            throw "Unknown DEV preflight stage: $Stage"
        }
    }
}

$result = Invoke-DevDeployPreflight `
    -GitDirectory $gitDirectory `
    -CommitSha $commitSha `
    -PolicyVersion $policyVersion `
    -StageNames $stageNames `
    -RunStage $runStage `
    -InputFingerprint $inputFingerprint `
    -LockTimeoutSeconds $LockTimeoutSeconds `
    -Force:$Force

if ($result.Cached) {
    Write-Host "DEV preflight reused a successful commit-scoped result."
} else {
    Write-Host "DEV preflight passed: $($result.StagesRun -join ', ')"
}
Write-Host "Measured preflight receipt: $($result.TimingPath)"
