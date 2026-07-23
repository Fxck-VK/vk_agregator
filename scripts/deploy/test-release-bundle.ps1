[CmdletBinding()]
param()

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..")
$tempRoot = Join-Path ([IO.Path]::GetTempPath()) ("vkagg-release-bundle-test-{0}" -f ([guid]::NewGuid().ToString("N")))
$bundleDir = Join-Path $tempRoot "bundle"
$signedPredicateDir = Join-Path $tempRoot "signed-predicates"
$outputEnv = Join-Path $tempRoot "verified.env"
$cosignLog = Join-Path $tempRoot "cosign.log"
$releaseManifestLog = Join-Path $tempRoot "release-manifest.log"
$commit = "0123456789abcdef0123456789abcdef01234567"
$digest = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
$identity = "https://github.com/Fxck-VK/vk_agregator/.github/workflows/docker-images.yml@refs/heads/main"
$services = @("api", "worker", "provider-webhook", "provider-balance-bot", "miniapp", "migrate", "backup")

function Assert-True {
    param(
        [Parameter(Mandatory = $true)][bool]$Condition,
        [Parameter(Mandatory = $true)][string]$Message
    )

    if (-not $Condition) {
        throw $Message
    }
}

function Write-TestManifest {
    param([Parameter(Mandatory = $true)][string[]]$OrderedServices)

    $images = @(
        foreach ($service in $OrderedServices) {
            [ordered]@{
                service = $service
                repository = "ghcr.io/fxck-vk/vk_agregator/$service"
                digest = "sha256:$digest"
            }
        }
    )
    $manifest = [ordered]@{
        schema_version = 1
        repository = "fxck-vk/vk_agregator"
        commit_sha = $commit
        source_branch = "main"
        workflow_identity = $identity
        images = $images
    }
    [IO.File]::WriteAllText(
        (Join-Path $bundleDir "release-manifest.json"),
        ($manifest | ConvertTo-Json -Depth 8),
        [Text.UTF8Encoding]::new($false)
    )
    [IO.File]::WriteAllText(
        (Join-Path $bundleDir "release-manifest.sigstore.json"),
        "{}",
        [Text.UTF8Encoding]::new($false)
    )

    foreach ($service in $services) {
        $bundleServiceDir = Join-Path $bundleDir $service
        $signedServiceDir = Join-Path $signedPredicateDir $service
        New-Item -ItemType Directory -Path $bundleServiceDir -Force | Out-Null
        New-Item -ItemType Directory -Path $signedServiceDir -Force | Out-Null
        $predicates = [ordered]@{
            "runtime.cdx.json" = "{`"kind`":`"runtime-cdx`",`"service`":`"$service`"}"
            "runtime.spdx.json" = "{`"kind`":`"runtime-spdx`",`"service`":`"$service`"}"
            "provenance.json" = "{`"kind`":`"provenance`",`"service`":`"$service`"}"
        }
        if ($service -eq "miniapp") {
            $predicates["source.cdx.json"] = "{`"kind`":`"source-cdx`",`"service`":`"$service`"}"
            $predicates["source.spdx.json"] = "{`"kind`":`"source-spdx`",`"service`":`"$service`"}"
        }
        foreach ($entry in $predicates.GetEnumerator()) {
            [IO.File]::WriteAllText((Join-Path $bundleServiceDir $entry.Key), $entry.Value, [Text.UTF8Encoding]::new($false))
            [IO.File]::WriteAllText((Join-Path $signedServiceDir $entry.Key), $entry.Value, [Text.UTF8Encoding]::new($false))
        }
    }
}

function Get-ScriptParameterNames {
    param([Parameter(Mandatory = $true)][string]$Path)

    $tokens = $null
    $errors = $null
    $ast = [System.Management.Automation.Language.Parser]::ParseFile($Path, [ref]$tokens, [ref]$errors)
    if ($errors.Count -gt 0) {
        throw "PowerShell parser rejected ${Path}: $($errors[0].Message)"
    }
    return @($ast.ParamBlock.Parameters | ForEach-Object { $_.Name.VariablePath.UserPath })
}

New-Item -ItemType Directory -Path $bundleDir -Force | Out-Null
$env:FAKE_COSIGN_LOG = $cosignLog
$env:FAKE_RELEASE_MANIFEST_LOG = $releaseManifestLog
$env:FAKE_SIGNED_PREDICATE_DIR = $signedPredicateDir

try {
    Write-TestManifest -OrderedServices $services

    $fakeCosign = Join-Path $tempRoot "fake-cosign.ps1"
    $fakeCosignContent = @'
param([Parameter(ValueFromRemainingArguments = $true)][string[]]$ToolArguments)
Add-Content -LiteralPath $env:FAKE_COSIGN_LOG -Value ($ToolArguments -join "`t")
if ($env:FAKE_COSIGN_FAIL_COMMAND -eq $ToolArguments[0]) { throw "requested fake Cosign failure" }
if ($env:FAKE_REQUIRE_GHCR_LOGIN -eq "true" -and $ToolArguments[0] -in @("verify", "verify-attestation") -and -not (Test-Path -LiteralPath $env:FAKE_GHCR_LOGIN_MARKER)) {
    throw "registry verification ran before GHCR login"
}
if ($ToolArguments[0] -eq "verify-attestation") {
    $typeIndex = [Array]::IndexOf($ToolArguments, "--type")
    if ($typeIndex -lt 0) { throw "fake Cosign missing --type" }
    $attestationType = $ToolArguments[$typeIndex + 1]
    $imageReference = $ToolArguments[-1]
    $imageName, $digestValue = $imageReference.Split("@")
    $service = $imageName.Substring($imageName.LastIndexOf("/") + 1)
    $digestValue = $digestValue.Substring("sha256:".Length)
    $predicateNames = @(switch ($attestationType) {
        "cyclonedx" { @("runtime.cdx.json") }
        "spdx" { @("runtime.spdx.json") }
        "slsaprovenance" { @("provenance.json") }
        default { throw "unexpected attestation type" }
    })
    if ($service -eq "miniapp" -and $attestationType -eq "cyclonedx") { $predicateNames += "source.cdx.json" }
    if ($service -eq "miniapp" -and $attestationType -eq "spdx") { $predicateNames += "source.spdx.json" }

    $envelopes = [System.Collections.Generic.List[object]]::new()
    foreach ($predicateName in $predicateNames) {
        $predicate = Get-Content -LiteralPath (Join-Path $env:FAKE_SIGNED_PREDICATE_DIR "$service\$predicateName") -Raw | ConvertFrom-Json
        $statement = [ordered]@{
            _type = "https://in-toto.io/Statement/v1"
            subject = @([ordered]@{ name = $service; digest = [ordered]@{ sha256 = $digestValue } })
            predicateType = "https://example.test/$attestationType"
            predicate = $predicate
        }
        $payloadJSON = $statement | ConvertTo-Json -Compress -Depth 12
        $payload = [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($payloadJSON))
        $envelopes.Add([ordered]@{
            payloadType = "application/vnd.in-toto+json"
            payload = $payload
            signatures = @([ordered]@{ sig = "verified" })
        })
    }
    ConvertTo-Json -InputObject $envelopes.ToArray() -Compress -Depth 12
}
'@
    [IO.File]::WriteAllText($fakeCosign, $fakeCosignContent, [Text.UTF8Encoding]::new($false))

    $fakeReleaseManifest = Join-Path $tempRoot "fake-release-manifest.ps1"
    $fakeReleaseManifestContent = @'
param([Parameter(ValueFromRemainingArguments = $true)][string[]]$ToolArguments)
Add-Content -LiteralPath $env:FAKE_RELEASE_MANIFEST_LOG -Value ($ToolArguments -join "`t")
if ($ToolArguments[0] -eq "verify-attestation") {
    $verificationIndex = [Array]::IndexOf($ToolArguments, "--verification-output")
    $predicateIndex = [Array]::IndexOf($ToolArguments, "--predicate")
    $imageIndex = [Array]::IndexOf($ToolArguments, "--image-ref")
    if ($verificationIndex -lt 0 -or $predicateIndex -lt 0 -or $imageIndex -lt 0) { throw "missing attestation argument" }
    $verification = Get-Content -LiteralPath $ToolArguments[$verificationIndex + 1] -Raw | ConvertFrom-Json
    $expectedPredicate = Get-Content -LiteralPath $ToolArguments[$predicateIndex + 1] -Raw | ConvertFrom-Json
    $expectedPredicateJSON = $expectedPredicate | ConvertTo-Json -Compress -Depth 12
    $expectedDigest = ($ToolArguments[$imageIndex + 1].Split("@")[-1]).Substring("sha256:".Length)
    foreach ($envelope in @($verification)) {
        $payloadJSON = [Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($envelope.payload))
        $statement = $payloadJSON | ConvertFrom-Json
        $digestMatches = @($statement.subject | Where-Object { $_.digest.sha256 -eq $expectedDigest }).Count -gt 0
        $predicateJSON = $statement.predicate | ConvertTo-Json -Compress -Depth 12
        if ($digestMatches -and $predicateJSON -eq $expectedPredicateJSON) { return }
    }
    throw "attestation predicate mismatch"
}
if ($ToolArguments[0] -ne "verify") { throw "unexpected release-manifest command" }
$outputIndex = [Array]::IndexOf($ToolArguments, "--output-env")
if ($outputIndex -lt 0 -or $outputIndex + 1 -ge $ToolArguments.Count) { throw "missing --output-env" }
$output = $ToolArguments[$outputIndex + 1]
$commitIndex = [Array]::IndexOf($ToolArguments, "--expected-commit")
$repositoryIndex = [Array]::IndexOf($ToolArguments, "--expected-repository")
$identityIndex = [Array]::IndexOf($ToolArguments, "--expected-workflow-identity")
if ($commitIndex -lt 0 -or $repositoryIndex -lt 0 -or $identityIndex -lt 0) { throw "missing trust expectation" }
$expectedCommit = $ToolArguments[$commitIndex + 1]
$expectedRepository = $ToolArguments[$repositoryIndex + 1]
$expectedIdentity = $ToolArguments[$identityIndex + 1]
$digest = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
$lines = foreach ($mapping in @(
    "API_IMAGE|api", "WORKER_IMAGE|worker", "PROVIDER_WEBHOOK_IMAGE|provider-webhook",
    "PROVIDER_BALANCE_BOT_IMAGE|provider-balance-bot", "MINIAPP_IMAGE|miniapp",
    "MIGRATE_IMAGE|migrate", "BACKUP_IMAGE|backup"
)) {
    $parts = $mapping.Split("|")
    "$($parts[0])=ghcr.io/$expectedRepository/$($parts[1])@sha256:$digest"
}
$lines += "RELEASE_COMMIT_SHA=$expectedCommit"
$lines += "RELEASE_MANIFEST_SHA256=$digest"
$lines += "RELEASE_WORKFLOW_IDENTITY=$expectedIdentity"
if ($env:FAKE_RELEASE_MANIFEST_INVALID -eq "true") {
    $lines[0] = "API_IMAGE=ghcr.io/$expectedRepository/api:latest"
}
[IO.File]::WriteAllLines($output, $lines, [Text.UTF8Encoding]::new($false))
'@
    [IO.File]::WriteAllText($fakeReleaseManifest, $fakeReleaseManifestContent, [Text.UTF8Encoding]::new($false))

    $verifier = Join-Path $repoRoot "scripts\release\verify-release-bundle.ps1"
    & $verifier `
        -ReleaseBundleDir $bundleDir `
        -WorkflowIdentity $identity `
        -ExpectedCommit $commit `
        -OutputEnvFile $outputEnv `
        -CosignPath $fakeCosign `
        -ReleaseManifestPath $fakeReleaseManifest

    $cosignCalls = @(Get-Content -LiteralPath $cosignLog)
    Assert-True ($cosignCalls.Count -eq 31) "expected 31 Cosign checks, got $($cosignCalls.Count)"
    Assert-True (@($cosignCalls | Where-Object { $_ -match '^verify-blob\t' }).Count -eq 1) "manifest blob verification is missing"
    Assert-True (@($cosignCalls | Where-Object { $_ -match '^verify\t' }).Count -eq 7) "seven image signature verifications are required"
    Assert-True (@($cosignCalls | Where-Object { $_ -match '^verify-attestation\t--type\tcyclonedx\t' }).Count -eq 8) "runtime and Mini App source CycloneDX attestations are required"
    Assert-True (@($cosignCalls | Where-Object { $_ -match '^verify-attestation\t--type\tspdx\t' }).Count -eq 8) "runtime and Mini App source SPDX attestations are required"
    Assert-True (@($cosignCalls | Where-Object { $_ -match '^verify-attestation\t--type\tslsaprovenance\t' }).Count -eq 7) "seven provenance attestations are required"
    Assert-True (@($cosignCalls | Where-Object { $_ -notmatch [regex]::Escape("--certificate-identity`t$identity") }).Count -eq 0) "every Cosign check must pin the exact workflow identity"

    $releaseCall = Get-Content -LiteralPath $releaseManifestLog -Raw
    Assert-True ($releaseCall.Contains("--expected-commit`t$commit")) "release manifest verifier must bind the full commit"
    Assert-True ($releaseCall.Contains("--expected-repository`tfxck-vk/vk_agregator")) "release manifest verifier must bind the workflow repository"
    Assert-True (@(Get-Content -LiteralPath $releaseManifestLog | Where-Object { $_ -match '^verify-attestation\t' }).Count -eq 23) "every expected predicate must be compared"
    Assert-True ((Get-Content -LiteralPath $outputEnv).Count -eq 10) "verified env must contain exactly ten generated entries"

    [IO.File]::WriteAllText(
        (Join-Path $bundleDir "api\runtime.cdx.json"),
        '{"kind":"tampered","service":"api"}',
        [Text.UTF8Encoding]::new($false)
    )
    $rejected = $false
    try {
        & $verifier `
            -ReleaseBundleDir $bundleDir `
            -WorkflowIdentity $identity `
            -ExpectedCommit $commit `
            -OutputEnvFile $outputEnv `
            -CosignPath $fakeCosign `
            -ReleaseManifestPath $fakeReleaseManifest
    } catch {
        $rejected = $true
    }
    Assert-True $rejected "tampered attestation predicate must fail closed"
    Assert-True (-not (Test-Path -LiteralPath $outputEnv)) "tampered predicate must not leave verifier output"

    Write-TestManifest -OrderedServices @("worker", "api", "provider-webhook", "provider-balance-bot", "miniapp", "migrate", "backup")
    $rejected = $false
    try {
        & $verifier `
            -ReleaseBundleDir $bundleDir `
            -WorkflowIdentity $identity `
            -ExpectedCommit $commit `
            -OutputEnvFile $outputEnv `
            -CosignPath $fakeCosign `
            -ReleaseManifestPath $fakeReleaseManifest
    } catch {
        $rejected = $true
    }
    Assert-True $rejected "wrong service order must fail closed"

    Write-TestManifest -OrderedServices $services
    [IO.File]::WriteAllText($outputEnv, "API_IMAGE=ghcr.io/fxck-vk/vk_agregator/api:latest", [Text.UTF8Encoding]::new($false))
    $env:FAKE_COSIGN_FAIL_COMMAND = "verify-attestation"
    $rejected = $false
    try {
        & $verifier `
            -ReleaseBundleDir $bundleDir `
            -WorkflowIdentity $identity `
            -ExpectedCommit $commit `
            -OutputEnvFile $outputEnv `
            -CosignPath $fakeCosign `
            -ReleaseManifestPath $fakeReleaseManifest
    } catch {
        $rejected = $true
    } finally {
        Remove-Item Env:\FAKE_COSIGN_FAIL_COMMAND -ErrorAction SilentlyContinue
    }
    Assert-True $rejected "failed Cosign verification must fail closed"
    Assert-True (-not (Test-Path -LiteralPath $outputEnv)) "failed verification must remove stale verifier output"

    $env:FAKE_RELEASE_MANIFEST_INVALID = "true"
    $rejected = $false
    try {
        & $verifier `
            -ReleaseBundleDir $bundleDir `
            -WorkflowIdentity $identity `
            -ExpectedCommit $commit `
            -OutputEnvFile $outputEnv `
            -CosignPath $fakeCosign `
            -ReleaseManifestPath $fakeReleaseManifest
    } catch {
        $rejected = $true
    } finally {
        Remove-Item Env:\FAKE_RELEASE_MANIFEST_INVALID -ErrorAction SilentlyContinue
    }
    Assert-True $rejected "invalid generated release environment must fail closed"
    Assert-True (-not (Test-Path -LiteralPath $outputEnv)) "invalid generated release environment must be removed"

    foreach ($script in @("deploy-prod.ps1", "rollback-prod.ps1")) {
        $path = Join-Path $PSScriptRoot $script
        $parameters = Get-ScriptParameterNames -Path $path
        foreach ($required in @("ReleaseBundleDir", "WorkflowIdentity", "CosignPath", "ReleaseManifestPath", "DryRun")) {
            Assert-True ($parameters -contains $required) "$script is missing -$required"
        }
        Assert-True ($parameters -notcontains "ReleaseEnvFile") "$script must not accept unverified -ReleaseEnvFile"
        $content = Get-Content -LiteralPath $path -Raw
        Assert-True ($content.Contains("verify-release-bundle.ps1")) "$script must verify the signed release bundle"
    }

    $dockerLog = Join-Path $tempRoot "docker.log"
    $fakeDocker = Join-Path $tempRoot "docker.cmd"
    $fakeDockerLines = @(
        "@echo off",
        ">>`"%FAKE_DOCKER_LOG%`" echo ARGS=%* API_IMAGE=%API_IMAGE%",
        "if `"%1`"==`"version`" (",
        "  if `"%2`"==`"--format`" echo 29.0.0",
        "  exit /b 0",
        ")",
        "if `"%1`"==`"info`" exit /b 0",
        "if `"%1`"==`"login`" (",
        "  type nul > `"%FAKE_GHCR_LOGIN_MARKER%`"",
        "  exit /b 0",
        ")",
        "if `"%1`"==`"compose`" (",
        "  if `"%2`"==`"version`" echo Docker Compose version v2.0.0",
        "  exit /b 0",
        ")",
        "exit /b 1"
    )
    [IO.File]::WriteAllText($fakeDocker, ($fakeDockerLines -join "`r`n"), [Text.Encoding]::ASCII)

    $runtimeEnv = Join-Path $tempRoot "runtime.env"
    # Keep credential-shaped fixtures short and clearly synthetic so repository-wide secret scans do not flag them as live credentials.
    $runtimeLines = @(
        "APP_ENV=staging",
        "DATA_SERVICES_MODE=managed",
        "DATABASE_URL=postgres://runtime:runtime@db.internal:5432/runtime?sslmode=require",
        "REDIS_ADDR=redis.internal:6379",
        "S3_ENDPOINT=https://s3.internal",
        "S3_ACCESS_KEY=runtime-access",
        "S3_SECRET_KEY=runtime-secret",
        "S3_BUCKET=artifacts",
        "VK_ACCESS_TOKEN=vk-live-token",
        "VK_SECRET=vk-live-secret",
        "VK_CONFIRMATION_TOKEN=ci-confirm",
        "VK_APP_SECRET=vk-live-app-secret",
        "ADMIN_TOKEN=ci-admin",
        "PAYMENT_PROVIDER=yookassa",
        "YOOKASSA_SHOP_ID=live-shop",
        "YOOKASSA_SECRET_KEY=live-secret",
        "YOOKASSA_RETURN_URL=https://staging.invalid/return",
        "ARTIFACT_SCANNER=none",
        "CLOUDFLARED_TUNNEL_TOKEN=unused-dry-run-token",
        "GHCR_USERNAME=release-reader",
        "GHCR_TOKEN=release-read-token"
    )
    [IO.File]::WriteAllLines($runtimeEnv, $runtimeLines, [Text.UTF8Encoding]::new($false))

    $commit = (& git -C $repoRoot rev-parse HEAD).Trim()
    Write-TestManifest -OrderedServices $services
    $oldPath = $env:PATH
    $oldDockerLog = $env:FAKE_DOCKER_LOG
    $oldApiImage = $env:API_IMAGE
    $loginMarker = Join-Path $tempRoot "ghcr-login.marker"
    try {
        $env:PATH = "$tempRoot;$oldPath"
        $env:FAKE_DOCKER_LOG = $dockerLog
        $env:FAKE_GHCR_LOGIN_MARKER = $loginMarker
        $env:FAKE_REQUIRE_GHCR_LOGIN = "true"
        $env:API_IMAGE = "ghcr.io/fxck-vk/vk_agregator/api:latest"

        & (Join-Path $PSScriptRoot "deploy-prod.ps1") `
            -ReleaseBundleDir $bundleDir `
            -WorkflowIdentity $identity `
            -EnvFile $runtimeEnv `
            -CosignPath $fakeCosign `
            -ReleaseManifestPath $fakeReleaseManifest `
            -SkipPull `
            -DryRun
        Assert-True ($env:API_IMAGE -eq "ghcr.io/fxck-vk/vk_agregator/api:latest") "deploy dry-run must restore the caller environment"

        Remove-Item -LiteralPath $loginMarker -Force -ErrorAction SilentlyContinue
        & (Join-Path $PSScriptRoot "rollback-prod.ps1") `
            -ReleaseBundleDir $bundleDir `
            -WorkflowIdentity $identity `
            -EnvFile $runtimeEnv `
            -CosignPath $fakeCosign `
            -ReleaseManifestPath $fakeReleaseManifest `
            -SkipBackup `
            -DryRun
        Assert-True ($env:API_IMAGE -eq "ghcr.io/fxck-vk/vk_agregator/api:latest") "rollback dry-run must restore the caller environment"
    } finally {
        $env:PATH = $oldPath
        if ($null -eq $oldDockerLog) { Remove-Item Env:\FAKE_DOCKER_LOG -ErrorAction SilentlyContinue } else { $env:FAKE_DOCKER_LOG = $oldDockerLog }
        if ($null -eq $oldApiImage) { Remove-Item Env:\API_IMAGE -ErrorAction SilentlyContinue } else { $env:API_IMAGE = $oldApiImage }
        Remove-Item Env:\FAKE_GHCR_LOGIN_MARKER -ErrorAction SilentlyContinue
        Remove-Item Env:\FAKE_REQUIRE_GHCR_LOGIN -ErrorAction SilentlyContinue
    }

    $configCalls = @(Get-Content -LiteralPath $dockerLog | Where-Object { $_ -match 'ARGS=compose .* config ' -or $_ -match 'ARGS=compose .* config API_IMAGE=' })
    Assert-True ($configCalls.Count -eq 2) "deploy and rollback must each run docker compose config exactly once"
    Assert-True (@($configCalls | Where-Object { $_ -notmatch 'API_IMAGE=ghcr\.io/fxck-vk/vk_agregator/api@sha256:[0-9a-f]{64}$' }).Count -eq 0) "verified digest must override inherited API_IMAGE during compose config"
    Assert-True (-not ((Get-Content -LiteralPath $dockerLog -Raw) -match 'ARGS=compose .*\s(pull|up|run)\s')) "dry-run must not pull or mutate containers"

    Write-Host "PowerShell signed release bundle tests passed."
} finally {
    Remove-Item Env:\FAKE_COSIGN_LOG -ErrorAction SilentlyContinue
    Remove-Item Env:\FAKE_RELEASE_MANIFEST_LOG -ErrorAction SilentlyContinue
    Remove-Item Env:\FAKE_SIGNED_PREDICATE_DIR -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath $tempRoot -Recurse -Force -ErrorAction SilentlyContinue
}
