[CmdletBinding()]
param(
    [ValidateRange(0, 3600)][int]$LockTimeoutSeconds = 300,
    [switch]$Force
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
Import-Module (Join-Path $PSScriptRoot "DevDeployPreflight.psm1") -Force
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

$worktreeChanges = Get-GitOutput -Arguments @("status", "--porcelain", "--untracked-files=all")
if (-not [string]::IsNullOrWhiteSpace($worktreeChanges)) {
    throw "DEV preflight requires a completely clean worktree. Commit, stash, or remove local changes first."
}

$commitSha = Get-GitOutput -Arguments @("rev-parse", "HEAD")
$gitDirectory = Get-GitOutput -Arguments @("rev-parse", "--absolute-git-dir")
$npmCommand = if ($IsWindows) { "npm.cmd" } else { "npm" }
$policyVersion = "v1"
$stageNames = @("tests", "audit", "govulncheck", "infrastructure", "trivy")

$runStage = {
    param([string]$Stage)

    switch ($Stage) {
        "tests" {
            Invoke-Native -Command "git" -Arguments @("diff", "--check", "$commitSha^", $commitSha)
            Invoke-Native -Command "go" -Arguments @("test", "./...")
            Invoke-Native -Command "go" -Arguments @("vet", "./...")
            foreach ($packagePath in @("web/miniapp", "web/admin", "web/platform")) {
                Invoke-Native -Command $npmCommand -Arguments @("--prefix", $packagePath, "run", "lint")
                Invoke-Native -Command $npmCommand -Arguments @("--prefix", $packagePath, "run", "typecheck")
                Invoke-Native -Command $npmCommand -Arguments @("--prefix", $packagePath, "run", "test")
            }
            Invoke-Native -Command $npmCommand -Arguments @("--prefix", "web/platform", "run", "test:packaging")
        }
        "audit" {
            foreach ($lockfile in @(
                "web/miniapp/package-lock.json",
                "web/admin/package-lock.json",
                "web/platform/package-lock.json"
            )) {
                Invoke-Native -Command "node" -Arguments @("scripts/ci/validate-npm-lockfiles.mjs", $lockfile)
            }
            foreach ($packagePath in @("web/miniapp", "web/admin", "web/platform")) {
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
            Invoke-Native -Command "bash" -Arguments @("scripts/ci/test-wait-for-github-workflow.sh")
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
    -LockTimeoutSeconds $LockTimeoutSeconds `
    -Force:$Force

if ($result.Cached) {
    Write-Host "DEV preflight reused a successful commit-scoped result."
} else {
    Write-Host "DEV preflight passed: $($result.StagesRun -join ', ')"
}
