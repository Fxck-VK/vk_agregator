[CmdletBinding()]
param()

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..")
$nightlyPath = Join-Path $repoRoot ".github\workflows\nightly-quality.yml"
$dockerImagesPath = Join-Path $repoRoot ".github\workflows\docker-images.yml"
$deployProdPath = Join-Path $repoRoot ".github\workflows\deploy-prod.yml"
$deployDevPath = Join-Path $repoRoot ".github\workflows\deploy-dev.yml"
$dependabotPath = Join-Path $repoRoot ".github\dependabot.yml"
$codeownersPath = Join-Path $repoRoot ".github\CODEOWNERS"
$releaseVerifierTestPath = Join-Path $repoRoot "scripts\deploy\test-verify-release-images.sh"

$expectedDockerfiles = @(
    "Dockerfile.api",
    "Dockerfile.backup",
    "Dockerfile.migrate",
    "Dockerfile.miniapp",
    "Dockerfile.provider-balance-bot",
    "Dockerfile.provider-webhook",
    "Dockerfile.worker"
)

function Assert-Contains {
    param(
        [Parameter(Mandatory = $true)][string]$Content,
        [Parameter(Mandatory = $true)][string]$Snippet,
        [Parameter(Mandatory = $true)][string]$Description
    )

    if (-not $Content.Contains($Snippet)) {
        throw "$Description is missing required snippet: $Snippet"
    }
}

function Assert-NotMatch {
    param(
        [Parameter(Mandatory = $true)][string]$Content,
        [Parameter(Mandatory = $true)][string]$Pattern,
        [Parameter(Mandatory = $true)][string]$Description
    )

    if ($Content -match $Pattern) {
        throw "$Description contains forbidden pattern: $Pattern"
    }
}

function Get-DockerfileInventory {
    param([Parameter(Mandatory = $true)][string]$Content)

    return @(
        [regex]::Matches($Content, '(?m)^\s+dockerfile:\s*(Dockerfile[^\s#]+)\s*$') |
            ForEach-Object { $_.Groups[1].Value } |
            Sort-Object -Unique
    )
}

function Assert-ExactInventory {
    param(
        [Parameter(Mandatory = $true)][string[]]$Actual,
        [Parameter(Mandatory = $true)][string]$Description
    )

    $missing = @($expectedDockerfiles | Where-Object { $Actual -notcontains $_ })
    $unexpected = @($Actual | Where-Object { $expectedDockerfiles -notcontains $_ })
    if ($Actual.Count -ne $expectedDockerfiles.Count -or $missing.Count -gt 0 -or $unexpected.Count -gt 0) {
        throw "$Description must contain exactly seven production Dockerfiles; missing=[$($missing -join ', ')], unexpected=[$($unexpected -join ', ')]"
    }
}

foreach ($path in @($nightlyPath, $dockerImagesPath)) {
    if (-not (Test-Path -LiteralPath $path)) {
        throw "required workflow is missing: $path"
    }
}

foreach ($path in @($deployProdPath, $deployDevPath, $dependabotPath, $codeownersPath, $releaseVerifierTestPath)) {
    if (-not (Test-Path -LiteralPath $path -PathType Leaf)) {
        throw "required supply-chain policy file is missing: $path"
    }
}

$nightly = Get-Content -LiteralPath $nightlyPath -Raw
$dockerImages = Get-Content -LiteralPath $dockerImagesPath -Raw
$deployProd = Get-Content -LiteralPath $deployProdPath -Raw
$deployDev = Get-Content -LiteralPath $deployDevPath -Raw
$dependabot = Get-Content -LiteralPath $dependabotPath -Raw
$codeowners = Get-Content -LiteralPath $codeownersPath -Raw

$workflowFiles = Get-ChildItem -LiteralPath (Join-Path $repoRoot ".github\workflows") -Filter "*.yml" -File
foreach ($workflowFile in $workflowFiles) {
    $workflowContent = Get-Content -LiteralPath $workflowFile.FullName -Raw
    $mutableUses = @(
        [regex]::Matches($workflowContent, '(?m)^\s*uses:\s*([^\s#]+)') |
            ForEach-Object { $_.Groups[1].Value } |
            Where-Object { $_ -notmatch '^\./' -and $_ -notmatch '@[0-9a-fA-F]{40}$' }
    )
    if ($mutableUses.Count -gt 0) {
        throw "$($workflowFile.Name) contains mutable action references: $($mutableUses -join ', ')"
    }
}

Assert-Contains $nightly 'github.com/rhysd/actionlint/cmd/actionlint@v1.7.12' 'Nightly Quality'
Assert-Contains $nightly 'github.com/securego/gosec/v2/cmd/gosec@v2.28.0' 'Nightly Quality'
Assert-Contains $nightly 'golang.org/x/vuln/cmd/govulncheck@v1.6.0' 'Nightly Quality'
Assert-Contains $nightly 'path: web/miniapp' 'Nightly Quality frontend matrix'
Assert-Contains $nightly 'path: web/admin' 'Nightly Quality frontend matrix'
Assert-Contains $nightly 'npm --prefix "${{ matrix.path }}" audit --audit-level=moderate' 'Nightly Quality'
Assert-Contains $nightly 'trivy-filesystem:' 'Nightly Quality'
Assert-Contains $nightly 'trivy-images:' 'Nightly Quality'
Assert-Contains $nightly 'uses: aquasecurity/trivy-action@ed142fd0673e97e23eac54620cfb913e5ce36c25 # v0.36.0' 'Nightly Quality'
Assert-Contains $nightly 'version: v0.72.0' 'Nightly Quality'
Assert-Contains $nightly 'severity: HIGH,CRITICAL' 'Nightly Quality'
Assert-Contains $nightly 'exit-code: "1"' 'Nightly Quality'
Assert-Contains $nightly 'if [ ! -f "${{ matrix.dockerfile }}" ]; then' 'Nightly Quality'
Assert-Contains $nightly 'grafana/k6:2.1.0' 'Nightly Quality'
Assert-Contains $nightly '--network none' 'Nightly Quality k6 validation'

Assert-NotMatch $nightly '(?m)(@latest|:latest(?:\s|$))' 'Nightly Quality'
Assert-NotMatch $nightly '(?m)^\s*continue-on-error:\s*true\s*$' 'Nightly Quality'
Assert-NotMatch $nightly '(?i)(No Dockerfiles found|skipping container image scan)' 'Nightly Quality'
Assert-NotMatch $nightly '(?i)(ignore-unfixed|ignore-policy|severity:\s*UNKNOWN)' 'Nightly Quality'

$nightlyInventory = Get-DockerfileInventory $nightly
$dockerImagesInventory = Get-DockerfileInventory $dockerImages
Assert-ExactInventory $nightlyInventory 'Nightly Quality image matrix'
Assert-ExactInventory $dockerImagesInventory 'Docker Images build matrix'

Assert-NotMatch $dockerImages '(?m)^  packages:\s*write\s*$' 'Docker Images top-level permissions'
Assert-Contains $dockerImages 'pull-request-build:' 'Docker Images'
Assert-Contains $dockerImages 'publish:' 'Docker Images'
Assert-Contains $dockerImages 'packages: write' 'Docker Images publish permissions'
Assert-Contains $dockerImages 'id-token: write' 'Docker Images publish permissions'
Assert-Contains $dockerImages 'sbom: true' 'Docker Images publish build'
Assert-Contains $dockerImages 'provenance: mode=max' 'Docker Images publish build'
Assert-Contains $dockerImages 'context: https://github.com/${{ github.repository }}.git#${{ github.sha }}' 'Docker Images immutable Git context'
Assert-Contains $dockerImages 'github-token: ${{ github.token }}' 'Docker Images private Git context authentication'
Assert-Contains $dockerImages 'cosign sign --yes' 'Docker Images signing'
Assert-NotMatch $dockerImages '(?i)type=raw,value=latest' 'Docker Images tags'

foreach ($deploy in @($deployProd, $deployDev)) {
    Assert-Contains $deploy 'scripts/deploy/verify-release-images.sh' 'Deployment image verification'
    Assert-Contains $deploy 'EXPECTED_SOURCE_REPOSITORY' 'Deployment image verification'
    Assert-Contains $deploy 'EXPECTED_SOURCE_REVISION' 'Deployment image verification'
    Assert-Contains $deploy 'EXPECTED_WORKFLOW_REF' 'Deployment image verification'
    Assert-Contains $deploy 'github.event.workflow_run.head_sha || github.sha' 'Deployment immutable checkout'
    Assert-Contains $deploy 'uses: sigstore/cosign-installer@7e8b541eb2e61bf99390e1afd4be13a184e9ebc5 # v3.10.1' 'Deployment image verification'
    Assert-Contains $deploy 'cosign-release: v3.0.2' 'Deployment image verification'
}

Assert-Contains $deployDev 'Manual DEV deploy must be dispatched from refs/heads/dev-deploy.' 'DEV immutable deployment'
Assert-Contains $deployDev 'deploy_image_tag="sha-${deploy_sha}"' 'DEV immutable deployment'
Assert-Contains $deployDev 'git checkout --detach "${deploy_sha}"' 'DEV immutable deployment'
Assert-Contains $deployDev 'DEV VPS repository does not match the immutable deploy commit.' 'DEV immutable deployment'
Assert-Contains $deployDev '--skip-pull --with-cloudflare' 'DEV immutable deployment'

$releaseVerifierTest = Get-Content -LiteralPath $releaseVerifierTestPath -Raw
Assert-Contains $releaseVerifierTest 'mismatched image tag' 'Release verifier negative tests'
Assert-Contains $releaseVerifierTest 'wrong signed revision' 'Release verifier negative tests'
Assert-Contains $releaseVerifierTest 'wrong signed repository' 'Release verifier negative tests'

Assert-Contains $dependabot 'package-ecosystem: "github-actions"' 'Dependabot GitHub Actions updates'
Assert-Contains $dependabot 'package-ecosystem: "docker"' 'Dependabot Docker updates'
Assert-Contains $dependabot 'package-ecosystem: "gomod"' 'Dependabot Go updates'
Assert-Contains $dependabot 'package-ecosystem: "npm"' 'Dependabot npm updates'
if ([regex]::Matches($dependabot, 'package-ecosystem:\s*"npm"').Count -ne 2) {
    throw 'Dependabot must audit both Mini App and Admin npm lockfiles.'
}
Assert-Contains $codeowners '.github/workflows/' 'CODEOWNERS workflow protection'

foreach ($dockerfile in $expectedDockerfiles) {
    $dockerfilePath = Join-Path $repoRoot $dockerfile
    if (-not (Test-Path -LiteralPath $dockerfilePath -PathType Leaf)) {
        throw "expected production Dockerfile is missing: $dockerfile"
    }
}

Write-Host "Production Dockerfile inventory ($($expectedDockerfiles.Count)):"
foreach ($dockerfile in $expectedDockerfiles) {
    Write-Host $dockerfile
}
Write-Host "Nightly Quality workflow policy OK"
