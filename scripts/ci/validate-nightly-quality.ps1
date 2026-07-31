[CmdletBinding()]
param()

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..")
$nightlyPath = Join-Path $repoRoot ".github\workflows\nightly-quality.yml"
$ciPath = Join-Path $repoRoot ".github\workflows\ci.yml"
$dockerImagesPath = Join-Path $repoRoot ".github\workflows\docker-images.yml"
$deployProdPath = Join-Path $repoRoot ".github\workflows\deploy-prod.yml"
$deployDevPath = Join-Path $repoRoot ".github\workflows\deploy-dev.yml"
$deployProdScriptPath = Join-Path $repoRoot "scripts\deploy\deploy-prod.sh"
$deployDevScriptPath = Join-Path $repoRoot "scripts\deploy\deploy-dev.sh"
$rollbackProdScriptPath = Join-Path $repoRoot "scripts\deploy\rollback-prod.sh"
$composePullRetryPath = Join-Path $repoRoot "scripts\deploy\compose-pull-retry.sh"
$dependabotPath = Join-Path $repoRoot ".github\dependabot.yml"
$codeownersPath = Join-Path $repoRoot ".github\CODEOWNERS"
$releaseVerifierTestPath = Join-Path $repoRoot "scripts\deploy\test-verify-release-images.sh"
$cosignInstallerPath = Join-Path $repoRoot "scripts\ci\install-cosign.sh"
$trivyInstallerPath = Join-Path $repoRoot "scripts\ci\install-trivy.sh"
$npmLockValidatorPath = Join-Path $repoRoot "scripts\ci\validate-npm-lockfiles.mjs"

$expectedDockerfiles = @(
    "Dockerfile.api",
    "Dockerfile.backup",
    "Dockerfile.migrate",
    "Dockerfile.miniapp",
    "Dockerfile.platform",
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
        throw "$Description must contain exactly eight production Dockerfiles; missing=[$($missing -join ', ')], unexpected=[$($unexpected -join ', ')]"
    }
}

foreach ($path in @($nightlyPath, $ciPath, $dockerImagesPath)) {
    if (-not (Test-Path -LiteralPath $path)) {
        throw "required workflow is missing: $path"
    }
}

foreach ($path in @($deployProdPath, $deployDevPath, $deployProdScriptPath, $deployDevScriptPath, $rollbackProdScriptPath, $composePullRetryPath, $dependabotPath, $codeownersPath, $releaseVerifierTestPath, $cosignInstallerPath, $trivyInstallerPath, $npmLockValidatorPath)) {
    if (-not (Test-Path -LiteralPath $path -PathType Leaf)) {
        throw "required supply-chain policy file is missing: $path"
    }
}

$nightly = Get-Content -LiteralPath $nightlyPath -Raw
$ci = Get-Content -LiteralPath $ciPath -Raw
$dockerImages = Get-Content -LiteralPath $dockerImagesPath -Raw
$deployProd = Get-Content -LiteralPath $deployProdPath -Raw
$deployDev = Get-Content -LiteralPath $deployDevPath -Raw
$deployProdScript = Get-Content -LiteralPath $deployProdScriptPath -Raw
$deployDevScript = Get-Content -LiteralPath $deployDevScriptPath -Raw
$rollbackProdScript = Get-Content -LiteralPath $rollbackProdScriptPath -Raw
$composePullRetry = Get-Content -LiteralPath $composePullRetryPath -Raw
$dependabot = Get-Content -LiteralPath $dependabotPath -Raw
$codeowners = Get-Content -LiteralPath $codeownersPath -Raw
$cosignInstaller = Get-Content -LiteralPath $cosignInstallerPath -Raw
$trivyInstaller = Get-Content -LiteralPath $trivyInstallerPath -Raw

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
Assert-Contains $nightly 'path: web/platform' 'Nightly Quality frontend matrix'
Assert-Contains $nightly 'npm --prefix "${{ matrix.path }}" audit --audit-level=moderate' 'Nightly Quality'
Assert-Contains $nightly 'node scripts/ci/validate-npm-lockfiles.mjs "${{ matrix.lockfile }}"' 'Nightly Quality lockfile integrity'
if ([regex]::Matches($ci, 'node scripts/ci/validate-npm-lockfiles\.mjs web/(?:miniapp|admin|platform)/package-lock\.json').Count -ne 3) {
    throw 'CI must validate immutable source metadata for all three frontend lockfiles.'
}
Assert-Contains $nightly 'trivy-filesystem:' 'Nightly Quality'
Assert-Contains $nightly 'trivy-images:' 'Nightly Quality'
Assert-Contains $nightly 'bash scripts/ci/install-trivy.sh' 'Nightly Quality'
if ([regex]::Matches($nightly, 'bash scripts/ci/install-trivy\.sh').Count -ne 2) {
    throw 'Nightly Quality must install pinned Trivy independently in filesystem and image jobs.'
}
Assert-Contains $nightly 'trivy fs' 'Nightly Quality filesystem scan'
Assert-Contains $nightly 'trivy image' 'Nightly Quality image scan'
Assert-Contains $nightly '--severity HIGH,CRITICAL' 'Nightly Quality'
Assert-Contains $nightly '--exit-code 1' 'Nightly Quality'
Assert-NotMatch $nightly 'uses:\s*aquasecurity/trivy-action@' 'Nightly Quality'
Assert-Contains $nightly 'if [ ! -f "${{ matrix.dockerfile }}" ]; then' 'Nightly Quality'
Assert-Contains $nightly 'grafana/k6:2.1.0@sha256:65c920dc067d5e2e00befbf982af6ad6ad0117034e8b1c65817c7975c52d4669' 'Nightly Quality'
Assert-Contains $nightly '--network none' 'Nightly Quality k6 validation'

Assert-NotMatch $nightly '(?m)(@latest|:latest(?:\s|$))' 'Nightly Quality'
Assert-NotMatch $nightly '(?m)^\s*continue-on-error:\s*true\s*$' 'Nightly Quality'
Assert-NotMatch $nightly '(?i)(No Dockerfiles found|skipping container image scan)' 'Nightly Quality'
Assert-NotMatch $nightly '(?i)(ignore-unfixed|ignore-policy|severity:\s*UNKNOWN)' 'Nightly Quality'

Assert-Contains $trivyInstaller 'readonly TRIVY_VERSION="0.72.0"' 'Pinned Trivy installer'
Assert-Contains $trivyInstaller 'readonly TRIVY_LINUX_AMD64_ARCHIVE_SHA256="bbb64b9695866ce4a7a8f5c9592002c5961cab378577fa3f8a040df362b9b2ea"' 'Pinned Trivy installer'
Assert-Contains $trivyInstaller 'https://github.com/aquasecurity/trivy/releases/download/v${TRIVY_VERSION}/trivy_${TRIVY_VERSION}_Linux-64bit.tar.gz' 'Pinned Trivy installer'
Assert-Contains $trivyInstaller 'sha256sum --check --strict' 'Pinned Trivy installer'
Assert-NotMatch $trivyInstaller '(?i)(?:@|/)latest(?:\b|/)' 'Pinned Trivy installer'

$nightlyInventory = Get-DockerfileInventory $nightly
$dockerImagesInventory = Get-DockerfileInventory $dockerImages
Assert-ExactInventory $nightlyInventory 'Nightly Quality image matrix'
Assert-ExactInventory $dockerImagesInventory 'Docker Images build matrix'

Assert-NotMatch $dockerImages '(?m)^  packages:\s*write\s*$' 'Docker Images top-level permissions'
Assert-Contains $dockerImages 'pull-request-build:' 'Docker Images'
Assert-Contains $dockerImages 'quality-gate:' 'Docker Images same-SHA quality gate'
Assert-Contains $dockerImages 'publish:' 'Docker Images'
Assert-Contains $dockerImages 'actions: read' 'Docker Images same-SHA quality gate permissions'
Assert-Contains $dockerImages 'head_sha=${GITHUB_SHA}' 'Docker Images same-SHA quality gate'
Assert-Contains $dockerImages 'conclusion == "success"' 'Docker Images same-SHA quality gate'
Assert-Contains $dockerImages 'needs: [validate_source, quality-gate]' 'Docker Images publish dependencies'
Assert-Contains $dockerImages "github.ref == 'refs/heads/main' || github.ref == 'refs/heads/dev-deploy'" 'Docker Images release publication gate'
Assert-Contains $dockerImages 'packages: write' 'Docker Images publish permissions'
Assert-Contains $dockerImages 'id-token: write' 'Docker Images publish permissions'
Assert-Contains $dockerImages 'sbom: true' 'Docker Images publish build'
Assert-Contains $dockerImages 'provenance: mode=max' 'Docker Images publish build'
Assert-Contains $dockerImages 'platforms: linux/amd64' 'Docker Images attestation platform'
Assert-Contains $dockerImages 'json .SBOM.SPDX' 'Docker Images SBOM assertion'
Assert-Contains $dockerImages 'json .Provenance.SLSA' 'Docker Images provenance assertion'
Assert-Contains $dockerImages '.buildDefinition.buildType' 'Docker Images SLSA v1 assertion'
Assert-Contains $dockerImages '.runDetails.builder.id' 'Docker Images SLSA v1 assertion'
Assert-Contains $dockerImages 'context: https://github.com/${{ github.repository }}.git#${{ github.sha }}' 'Docker Images immutable Git context'
Assert-Contains $dockerImages 'github-token: ${{ github.token }}' 'Docker Images private Git context authentication'
Assert-Contains $dockerImages 'cosign sign --yes' 'Docker Images signing'
Assert-Contains $dockerImages 'bash scripts/ci/install-trivy.sh' 'Docker Images pre-sign vulnerability gate'
Assert-Contains $dockerImages 'trivy image --scanners vuln --severity HIGH,CRITICAL --exit-code 1' 'Docker Images pre-sign vulnerability gate'
$trivyGateOffset = $dockerImages.IndexOf('trivy image --scanners vuln --severity HIGH,CRITICAL --exit-code 1', [StringComparison]::Ordinal)
$signOffset = $dockerImages.IndexOf('cosign sign --yes', [StringComparison]::Ordinal)
if ($trivyGateOffset -lt 0 -or $signOffset -lt 0 -or $trivyGateOffset -ge $signOffset) {
    throw 'Docker Images must scan immutable digests before signing them.'
}
Assert-NotMatch $dockerImages '(?i)type=raw,value=latest' 'Docker Images tags'
Assert-NotMatch $dockerImages '(?m)^\s*type=ref,event=branch\s*$' 'Docker Images mutable branch tags'
Assert-NotMatch $dockerImages '(?m)^\s*type=sha,prefix=sha-,format=short\s*$' 'Docker Images short SHA tags'
Assert-Contains $dockerImages 'type=sha,prefix=sha-,format=long' 'Docker Images immutable full-SHA tag'

Assert-Contains $cosignInstaller 'readonly COSIGN_VERSION="3.0.2"' 'Pinned Cosign installer'
Assert-Contains $cosignInstaller 'readonly COSIGN_LINUX_AMD64_SHA256="46dbdcb5467a3dfec2526923d0b3365e40c8d9dc00ec23d5aca3437449e8cbfd"' 'Pinned Cosign installer'
Assert-Contains $cosignInstaller 'https://github.com/sigstore/cosign/releases/download/v${COSIGN_VERSION}/cosign-linux-amd64' 'Pinned Cosign installer'
Assert-Contains $cosignInstaller 'sha256sum --check --strict' 'Pinned Cosign installer'
Assert-NotMatch $cosignInstaller '(?i)(?:@|/)latest(?:\b|/)' 'Pinned Cosign installer'

foreach ($workflow in @($dockerImages, $deployProd, $deployDev)) {
    Assert-Contains $workflow 'bash scripts/ci/install-cosign.sh' 'Pinned Cosign installation'
    Assert-NotMatch $workflow 'uses:\s*sigstore/cosign-installer@' 'Pinned Cosign installation'
}

foreach ($deploy in @($deployProd, $deployDev)) {
    Assert-Contains $deploy 'packages: read' 'Deployment GHCR permissions'
    Assert-Contains $deploy 'GHCR_USERNAME: ${{ github.actor }}' 'Deployment ephemeral GHCR identity'
    Assert-Contains $deploy 'GHCR_TOKEN: ${{ secrets.GITHUB_TOKEN }}' 'Deployment ephemeral GHCR token'
    Assert-NotMatch $deploy 'GHCR_USERNAME:\s*\$\{\{\s*secrets\.GHCR_USERNAME\s*\}\}' 'Deployment long-lived GHCR identity'
    Assert-NotMatch $deploy 'GHCR_TOKEN:\s*\$\{\{\s*secrets\.GHCR_TOKEN\s*\}\}' 'Deployment long-lived GHCR token'
    Assert-Contains $deploy 'scripts/deploy/verify-release-images.sh' 'Deployment image verification'
    Assert-Contains $deploy 'EXPECTED_SOURCE_REPOSITORY' 'Deployment image verification'
    Assert-Contains $deploy 'EXPECTED_SOURCE_REVISION' 'Deployment image verification'
    Assert-Contains $deploy 'EXPECTED_WORKFLOW_REF' 'Deployment image verification'
    Assert-Contains $deploy 'github.event.workflow_run.head_sha || github.sha' 'Deployment immutable checkout'
    Assert-Contains $deploy 'previous_revision="${PREVIOUS_IMAGE_TAG#sha-}"' 'Rollback immutable revision derivation'
    Assert-Contains $deploy 'verify-release-images.sh' 'Rollback image verification'
    Assert-Contains $deploy '--revision "${previous_revision}"' 'Rollback image verification'
}

Assert-Contains $deployDev 'environment: development' 'DEV protected environment'
Assert-Contains $deployDev 'Manual DEV deploy must be dispatched from refs/heads/dev-deploy.' 'DEV immutable deployment'
Assert-Contains $deployDev 'deploy_image_tag="sha-${deploy_sha}"' 'DEV immutable deployment'
Assert-Contains $deployDev 'git checkout --detach "${deploy_sha}"' 'DEV immutable deployment'
Assert-Contains $deployDev 'DEV VPS repository does not match the immutable deploy commit.' 'DEV immutable deployment'
Assert-Contains $deployDev '--skip-pull --with-cloudflare' 'DEV immutable deployment'
Assert-Contains $deployDev 'if [[ "${env_tag}" =~ ^sha-[0-9a-f]{40}$ ]]; then' 'DEV previous release capture'
Assert-Contains $deployDev 'tag="${env_tag}"' 'DEV previous release capture'
Assert-NotMatch $deployDev 'Current IMAGE_TAG does not match the checked-out DEV revision\.' 'DEV recoverable checkout drift'
Assert-Contains $deployDev 'name: Ensure DEV VPS disk headroom' 'DEV disk preflight'
Assert-Contains $deployDev 'required_kb=$((2 * 1024 * 1024))' 'DEV disk preflight'
Assert-Contains $deployDev 'docker system prune --all --force' 'DEV disk preflight'
Assert-NotMatch $deployDev '(?m)docker\s+volume\s+prune|docker\s+system\s+prune[^\r\n]*--volumes' 'DEV disk preflight volume safety'

Assert-Contains $deployProd 'if [[ "${env_tag}" =~ ^sha-[0-9a-f]{40}$ ]]; then' 'Production previous release capture'
Assert-Contains $deployProd 'tag="${env_tag}"' 'Production previous release capture'
Assert-NotMatch $deployProd 'Current IMAGE_TAG does not match the checked-out production revision\.' 'Production recoverable checkout drift'
Assert-Contains $deployProd 'git checkout --detach "${previous_revision}"' 'Production rollback source alignment'
Assert-Contains $deployProd 'scripts/deploy/docker-login-retry.sh' 'Production GHCR login retry'
Assert-Contains $ci 'bash scripts/deploy/test-docker-login-retry.sh' 'GHCR login retry regression test'
Assert-Contains $deployProdScript 'compose-pull-retry.sh' 'Production image pull retry'
Assert-Contains $deployDevScript 'compose-pull-retry.sh' 'DEV image pull retry'
Assert-Contains $rollbackProdScript 'compose-pull-retry.sh' 'Production rollback image pull retry'
Assert-Contains $composePullRetry '--kill-after=' 'Bounded image pull timeout'
Assert-Contains $ci 'bash scripts/deploy/test-compose-pull-retry.sh' 'Image pull retry regression test'

$releaseVerifierTest = Get-Content -LiteralPath $releaseVerifierTestPath -Raw
Assert-Contains $releaseVerifierTest 'mismatched image tag' 'Release verifier negative tests'
Assert-Contains $releaseVerifierTest 'wrong signed revision' 'Release verifier negative tests'
Assert-Contains $releaseVerifierTest 'wrong signed repository' 'Release verifier negative tests'

Assert-Contains $dependabot 'package-ecosystem: "github-actions"' 'Dependabot GitHub Actions updates'
Assert-Contains $dependabot 'package-ecosystem: "docker"' 'Dependabot Docker updates'
Assert-Contains $dependabot 'package-ecosystem: "gomod"' 'Dependabot Go updates'
Assert-Contains $dependabot 'package-ecosystem: "npm"' 'Dependabot npm updates'
if ([regex]::Matches($dependabot, 'package-ecosystem:\s*"npm"').Count -ne 3) {
    throw 'Dependabot must audit Mini App, Admin, and Platform npm lockfiles.'
}
Assert-Contains $codeowners '.github/workflows/' 'CODEOWNERS workflow protection'
Assert-Contains $codeowners 'scripts/deploy/**' 'CODEOWNERS deploy script protection'
Assert-Contains $codeowners 'scripts/ci/install-*.sh' 'CODEOWNERS scanner installer protection'

foreach ($dockerfile in $expectedDockerfiles) {
    $dockerfilePath = Join-Path $repoRoot $dockerfile
    if (-not (Test-Path -LiteralPath $dockerfilePath -PathType Leaf)) {
        throw "expected production Dockerfile is missing: $dockerfile"
    }
    $dockerfileContent = Get-Content -LiteralPath $dockerfilePath -Raw
    if ($dockerfileContent -match '(?im)^\s*RUN\s+.*\bapk\s+upgrade\b') {
        throw "$dockerfile must not run apk upgrade during an otherwise pinned image build"
    }
}

Write-Host "Production Dockerfile inventory ($($expectedDockerfiles.Count)):"
foreach ($dockerfile in $expectedDockerfiles) {
    Write-Host $dockerfile
}
Write-Host "Nightly Quality workflow policy OK"
