[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string]$ReleaseEnvFile,
    [string]$ExpectedCommit = "",
    [string]$ExpectedRepository = "fxck-vk/vk_agregator"
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

if ($ExpectedRepository -notmatch '^[a-z0-9][a-z0-9._-]*/[a-z0-9][a-z0-9._-]*$') {
    throw "Expected repository is invalid."
}
if ($ExpectedCommit.Length -gt 0 -and $ExpectedCommit -notmatch '^[0-9a-f]{40}$') {
    throw "Expected commit must be a full lowercase SHA."
}
if (-not (Test-Path -LiteralPath $ReleaseEnvFile -PathType Leaf)) {
    throw "Verified release env file is missing: $ReleaseEnvFile"
}
$item = Get-Item -LiteralPath $ReleaseEnvFile -Force
if (-not [string]::IsNullOrEmpty($item.LinkType)) {
    throw "Verified release env must not be a symbolic link."
}

$expectedImages = [ordered]@{
    API_IMAGE = "api"
    WORKER_IMAGE = "worker"
    PROVIDER_WEBHOOK_IMAGE = "provider-webhook"
    PROVIDER_BALANCE_BOT_IMAGE = "provider-balance-bot"
    MINIAPP_IMAGE = "miniapp"
    MIGRATE_IMAGE = "migrate"
    BACKUP_IMAGE = "backup"
}
$allowed = @($expectedImages.Keys) + @("RELEASE_COMMIT_SHA", "RELEASE_MANIFEST_SHA256", "RELEASE_WORKFLOW_IDENTITY")
$values = @{}

foreach ($line in Get-Content -LiteralPath $ReleaseEnvFile) {
    if ($line -notmatch '^(?<key>[A-Z][A-Z0-9_]*)=(?<value>.+)$') {
        throw "Verified release env contains an invalid line."
    }
    $key = $Matches.key
    $value = $Matches.value
    if ($allowed -notcontains $key) {
        throw "Verified release env contains unexpected key $key."
    }
    if ($values.ContainsKey($key)) {
        throw "Verified release env contains duplicate key $key."
    }
    $values[$key] = $value
}

if ($values.Count -ne 10) {
    throw "Verified release env must contain exactly ten entries."
}
foreach ($entry in $expectedImages.GetEnumerator()) {
    $expected = '^ghcr\.io/' + [regex]::Escape($ExpectedRepository) + '/' + [regex]::Escape($entry.Value) + '@sha256:[0-9a-f]{64}$'
    if (-not $values.ContainsKey($entry.Key) -or $values[$entry.Key] -notmatch $expected) {
        throw "$($entry.Key) is not the expected digest-only image reference."
    }
}
if (-not $values.ContainsKey("RELEASE_COMMIT_SHA") -or $values.RELEASE_COMMIT_SHA -notmatch '^[0-9a-f]{40}$') {
    throw "RELEASE_COMMIT_SHA is invalid."
}
if ($ExpectedCommit.Length -gt 0 -and $values.RELEASE_COMMIT_SHA -ne $ExpectedCommit) {
    throw "Verified release commit does not match checked-out source."
}
if (-not $values.ContainsKey("RELEASE_MANIFEST_SHA256") -or $values.RELEASE_MANIFEST_SHA256 -notmatch '^[0-9a-f]{64}$') {
    throw "RELEASE_MANIFEST_SHA256 is invalid."
}
if (-not $values.ContainsKey("RELEASE_WORKFLOW_IDENTITY") -or $values.RELEASE_WORKFLOW_IDENTITY -notmatch '^https://github\.com/[A-Za-z0-9][A-Za-z0-9_.-]*/[A-Za-z0-9][A-Za-z0-9_.-]*/\.github/workflows/docker-images\.yml@refs/heads/[A-Za-z0-9][A-Za-z0-9._/-]*$') {
    throw "RELEASE_WORKFLOW_IDENTITY is invalid."
}
$raw = Get-Content -LiteralPath $ReleaseEnvFile -Raw
if ($raw -match 'IMAGE_TAG|:sha-|:latest') {
    throw "Verified release env contains mutable tag material."
}

Write-Host "Verified release env passed digest-only validation for seven images."
