[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string]$ReleaseBundleDir,
    [Parameter(Mandatory = $true)][string]$WorkflowIdentity,
    [Parameter(Mandatory = $true)][string]$OutputEnvFile,
    [string]$ExpectedCommit = "",
    [string]$CosignPath = "cosign",
    [string]$ReleaseManifestPath = ""
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$oidcIssuer = "https://token.actions.githubusercontent.com"
$expectedServices = @("api", "worker", "provider-webhook", "provider-balance-bot", "miniapp", "migrate", "backup")
$identityPattern = [regex]::new(
    '^https://github\.com/(?<owner>[A-Za-z0-9][A-Za-z0-9_.-]*)/(?<repository>[A-Za-z0-9][A-Za-z0-9_.-]*)/\.github/workflows/docker-images\.yml@refs/heads/[A-Za-z0-9][A-Za-z0-9._/-]*$',
    [Text.RegularExpressions.RegexOptions]::CultureInvariant
)

function Resolve-ToolPath {
    param([Parameter(Mandatory = $true)][string]$Path)

    if (Test-Path -LiteralPath $Path -PathType Leaf) {
        return (Resolve-Path -LiteralPath $Path).Path
    }
    return $Path
}

function Assert-RegularFile {
    param(
        [Parameter(Mandatory = $true)][string]$Path,
        [Parameter(Mandatory = $true)][string]$Description
    )

    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
        throw "$Description is missing: $Path"
    }
    $item = Get-Item -LiteralPath $Path -Force
    if (($item.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0 -or $item.Length -le 0) {
        throw "$Description must be a non-empty regular non-symlink file."
    }
}

function Invoke-CheckedTool {
    param(
        [Parameter(Mandatory = $true)][string]$FilePath,
        [Parameter(Mandatory = $true)][string[]]$ToolArguments,
        [Parameter(Mandatory = $true)][string]$Description
    )

    $global:LASTEXITCODE = 0
    & $FilePath @ToolArguments | Out-Null
    if ($LASTEXITCODE -ne 0) {
        throw "$Description failed with exit code $LASTEXITCODE."
    }
}

function Invoke-CheckedToolCapture {
    param(
        [Parameter(Mandatory = $true)][string]$FilePath,
        [Parameter(Mandatory = $true)][string[]]$ToolArguments,
        [Parameter(Mandatory = $true)][string]$Description,
        [Parameter(Mandatory = $true)][string]$OutputPath
    )

    $global:LASTEXITCODE = 0
    $outputLines = @(& $FilePath @ToolArguments)
    if ($LASTEXITCODE -ne 0) {
        throw "$Description failed with exit code $LASTEXITCODE."
    }
    $output = ($outputLines | ForEach-Object { [string]$_ }) -join "`n"
    if ([string]::IsNullOrWhiteSpace($output)) {
        throw "$Description returned empty verification output."
    }
    [IO.File]::WriteAllText($OutputPath, $output, [Text.UTF8Encoding]::new($false))
}

$identityMatch = $identityPattern.Match($WorkflowIdentity)
if (-not $identityMatch.Success) {
    throw "WorkflowIdentity must be the exact Docker Images GitHub workflow identity."
}
$expectedRepository = (
    $identityMatch.Groups["owner"].Value + "/" + $identityMatch.Groups["repository"].Value
).ToLowerInvariant()

if (-not (Test-Path -LiteralPath $ReleaseBundleDir -PathType Container)) {
    throw "Release bundle directory is missing: $ReleaseBundleDir"
}
$bundleItem = Get-Item -LiteralPath $ReleaseBundleDir -Force
if (($bundleItem.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) {
    throw "Release bundle directory must not be a symbolic link."
}
$bundleDir = (Resolve-Path -LiteralPath $ReleaseBundleDir).Path
$manifestPath = Join-Path $bundleDir "release-manifest.json"
$signatureBundlePath = Join-Path $bundleDir "release-manifest.sigstore.json"
Assert-RegularFile -Path $manifestPath -Description "Release manifest"
Assert-RegularFile -Path $signatureBundlePath -Description "Release manifest Sigstore bundle"

$outputPath = if ([IO.Path]::IsPathRooted($OutputEnvFile)) {
    [IO.Path]::GetFullPath($OutputEnvFile)
} else {
    [IO.Path]::GetFullPath((Join-Path (Get-Location).Path $OutputEnvFile))
}
$cosign = Resolve-ToolPath -Path $CosignPath
$releaseManifest = if ([string]::IsNullOrWhiteSpace($ReleaseManifestPath)) {
    ""
} else {
    Resolve-ToolPath -Path $ReleaseManifestPath
}
$releaseManifestRepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path

function Invoke-ReleaseManifestTool {
    param(
        [Parameter(Mandatory = $true)][string[]]$ToolArguments,
        [Parameter(Mandatory = $true)][string]$Description
    )

    if ([string]::IsNullOrWhiteSpace($releaseManifest)) {
        Push-Location $releaseManifestRepoRoot
        try {
            Invoke-CheckedTool -FilePath "go" -Description $Description -ToolArguments (@("run", "./cmd/release-manifest") + $ToolArguments)
        } finally {
            Pop-Location
        }
    } else {
        Invoke-CheckedTool -FilePath $releaseManifest -Description $Description -ToolArguments $ToolArguments
    }
}
Remove-Item -LiteralPath $outputPath -Force -ErrorAction SilentlyContinue

try {
Invoke-CheckedTool -FilePath $cosign -Description "Cosign release manifest verification" -ToolArguments @(
    "verify-blob",
    "--bundle", $signatureBundlePath,
    "--certificate-identity", $WorkflowIdentity,
    "--certificate-oidc-issuer", $oidcIssuer,
    $manifestPath
)

try {
    $manifest = Get-Content -LiteralPath $manifestPath -Raw | ConvertFrom-Json
} catch {
    throw "Signed release manifest is not valid JSON."
}

if ($manifest.repository -ne $expectedRepository) {
    throw "Signed release manifest repository does not match WorkflowIdentity."
}
if ($manifest.workflow_identity -ne $WorkflowIdentity) {
    throw "Signed release manifest workflow identity mismatch."
}
if ($manifest.commit_sha -notmatch '^[0-9a-f]{40}$') {
    throw "Signed release manifest commit is invalid."
}
if ([string]::IsNullOrWhiteSpace($ExpectedCommit)) {
    $ExpectedCommit = [string]$manifest.commit_sha
} elseif ($ExpectedCommit -notmatch '^[0-9a-f]{40}$' -or $manifest.commit_sha -ne $ExpectedCommit) {
    throw "Signed release manifest commit does not match the expected source commit."
}

$images = @($manifest.images)
if ($images.Count -ne $expectedServices.Count) {
    throw "Signed release manifest must contain exactly seven images."
}
for ($index = 0; $index -lt $expectedServices.Count; $index++) {
    $service = $expectedServices[$index]
    $image = $images[$index]
    $expectedImageRepository = "ghcr.io/$expectedRepository/$service"
    if ($image.service -ne $service -or $image.repository -ne $expectedImageRepository) {
        throw "Signed release manifest service set or order is invalid."
    }
    if ($image.digest -notmatch '^sha256:[0-9a-f]{64}$') {
        throw "Signed release manifest digest is invalid for $service."
    }
}

$verifyArguments = @(
    "verify",
    "--manifest", $manifestPath,
    "--bundle-dir", $bundleDir,
    "--expected-repository", $expectedRepository,
    "--expected-commit", $ExpectedCommit,
    "--expected-workflow-identity", $WorkflowIdentity,
    "--output-env", $outputPath
)

Invoke-ReleaseManifestTool -Description "Release manifest verification" -ToolArguments $verifyArguments

Assert-RegularFile -Path $outputPath -Description "Verifier-generated release environment"
& (Join-Path $PSScriptRoot "..\deploy\validate-release-env.ps1") `
    -ReleaseEnvFile $outputPath `
    -ExpectedCommit $ExpectedCommit `
    -ExpectedRepository $expectedRepository | Out-Null

for ($index = 0; $index -lt $expectedServices.Count; $index++) {
    $service = $expectedServices[$index]
    $image = $images[$index]
    $imageReference = "$($image.repository)@$($image.digest)"
    Invoke-CheckedTool -FilePath $cosign -Description "Cosign image verification for $service" -ToolArguments @(
        "verify",
        "--certificate-identity", $WorkflowIdentity,
        "--certificate-oidc-issuer", $oidcIssuer,
        $imageReference
    )

    $predicateChecks = @(
        [pscustomobject]@{ Type = "cyclonedx"; Path = Join-Path $bundleDir "$service\runtime.cdx.json" },
        [pscustomobject]@{ Type = "spdx"; Path = Join-Path $bundleDir "$service\runtime.spdx.json" },
        [pscustomobject]@{ Type = "slsaprovenance"; Path = Join-Path $bundleDir "$service\provenance.json" }
    )
    if ($service -eq "miniapp") {
        $predicateChecks += @(
            [pscustomobject]@{ Type = "cyclonedx"; Path = Join-Path $bundleDir "$service\source.cdx.json" },
            [pscustomobject]@{ Type = "spdx"; Path = Join-Path $bundleDir "$service\source.spdx.json" }
        )
    }

    foreach ($check in $predicateChecks) {
        Assert-RegularFile -Path $check.Path -Description "$service $($check.Type) predicate"
        $verificationOutput = Join-Path ([IO.Path]::GetTempPath()) ("vk-ai-aggregator-attestation-{0}.json" -f ([guid]::NewGuid().ToString("N")))
        try {
            Invoke-CheckedToolCapture -FilePath $cosign -Description "Cosign $($check.Type) attestation verification for $service" -OutputPath $verificationOutput -ToolArguments @(
                "verify-attestation",
                "--type", $check.Type,
                "--certificate-identity", $WorkflowIdentity,
                "--certificate-oidc-issuer", $oidcIssuer,
                $imageReference
            )
            Invoke-ReleaseManifestTool -Description "$service $($check.Type) predicate verification" -ToolArguments @(
                "verify-attestation",
                "--verification-output", $verificationOutput,
                "--predicate", $check.Path,
                "--image-ref", $imageReference
            )
        } finally {
            Remove-Item -LiteralPath $verificationOutput -Force -ErrorAction SilentlyContinue
        }
    }
}

Write-Host "Signed release bundle verified for seven immutable digests."
} catch {
    Remove-Item -LiteralPath $outputPath -Force -ErrorAction SilentlyContinue
    throw
}
