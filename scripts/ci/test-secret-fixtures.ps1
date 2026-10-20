[CmdletBinding()]
param([string]$GitleaksPath = "gitleaks")

$ErrorActionPreference = "Stop"
$configPath = Join-Path (Resolve-Path (Join-Path $PSScriptRoot "../..")).Path ".gitleaks.toml"
$cases = @(
    @{ Path = '.github/workflows/ci.yml'; Key = 'POSTGRES_PASSWORD'; Value = 'session_test_fixture'; Rule = 'project-secret-assignment' },
    @{ Path = 'scripts/deploy/test-dev-email.sh'; Key = 'DEV_ACCOUNT_EMAIL_SMTP_PASSWORD'; Value = 'fixture-smtp-value'; Rule = 'project-secret-assignment' },
    @{ Path = 'scripts/deploy/test-assemble-env-parts.sh'; Key = 'ADMIN_TOKEN'; Value = 'fixture-neiirohub.ru-secret'; Rule = 'project-secret-assignment' },
    @{ Path = 'internal/service/joborchestrator/turbo_h3_test.go'; Key = 'IdempotencyKey'; Value = 'h3-priced-job'; Rule = 'generic-api-key' },
    @{ Path = 'internal/adapter/provider/apimart/kling_veo_test.go'; Key = 'IdempotencyKey'; Value = 'kling-v3-contract'; Rule = 'generic-api-key' },
    @{ Path = 'web/platform/src/features/conversations/pending-conversation-bootstrap.test.ts'; Key = 'messageKey'; Value = 'e7c979f5-24e5-4f88-924b-a592d6e5a906'; Rule = 'generic-api-key' },
    @{ Path = 'web/platform/src/features/conversations/PendingConversationBootstrap/PendingConversationBootstrap.test.tsx'; Key = 'messageKey'; Value = 'c7c979f5-24e5-4f88-924b-a592d6e5a906'; Rule = 'generic-api-key' },
    @{ Path = 'web/platform/src/features/conversations/PendingConversationBootstrap/PendingConversationBootstrap.test.tsx'; Key = 'messageKey'; Value = 'e7c979f5-24e5-4f88-924b-a592d6e5a906'; Rule = 'generic-api-key' }
)
$temporaryBase = [IO.Path]::GetFullPath([IO.Path]::GetTempPath())
$fixtureRoot = Join-Path $temporaryBase ("gitleaks-exact-fixtures-" + [guid]::NewGuid().ToString('N'))

function Format-FixtureLine {
    param($Case, [string]$Value)
    if ($Case.Rule -eq 'project-secret-assignment') { return ($Case.Key + '=' + $Value) }
    return ($Case.Key + ': "' + $Value + '"')
}

try {
    for ($index = 0; $index -lt $cases.Count; $index++) {
        $case = $cases[$index]
        $caseRoot = Join-Path $fixtureRoot ([string]$index)
        $canonical = Join-Path $caseRoot $case.Path
        New-Item -ItemType Directory -Path (Split-Path -Parent $canonical) -Force | Out-Null
        # Construct synthetic high-entropy markers without embedding credentials.
        $canary = '9f4a6c2e' + '7b814d35' + 'a9c62e8f' + '1b7d5a30'
        Set-Content -LiteralPath $canonical -Encoding ascii -Value @(
            (Format-FixtureLine $case $case.Value),
            (Format-FixtureLine $case $canary)
        )
        Set-Content -LiteralPath (Join-Path $caseRoot 'untrusted.txt') -Encoding ascii `
            -Value (Format-FixtureLine $case $case.Value)
        $report = Join-Path $caseRoot 'report.json'
        Push-Location $caseRoot
        try {
            $savedPreference = $ErrorActionPreference
            $ErrorActionPreference = 'Continue'
            $scanOutput = & $GitleaksPath dir --config $configPath --redact --no-banner --no-color `
                --report-format json --report-path $report . 2>&1
            $scanExit = $LASTEXITCODE
            $ErrorActionPreference = $savedPreference
        } finally { Pop-Location }
        if ($scanExit -ne 1 -or -not (Test-Path -LiteralPath $report -PathType Leaf)) {
            throw "Exact fixture case $index did not detect both negative controls."
        }
        $findings = @(Get-Content -LiteralPath $report -Raw | ConvertFrom-Json)
        $canonicalFindings = @($findings | Where-Object { $_.File.Replace('\', '/') -eq $case.Path })
        $relocatedFindings = @($findings | Where-Object { $_.File -eq 'untrusted.txt' })
        if ($findings.Count -ne 2 -or $canonicalFindings.Count -ne 1 -or
            $canonicalFindings[0].StartLine -ne 2 -or $relocatedFindings.Count -ne 1 -or
            @($findings | Where-Object { $_.RuleID -ne $case.Rule }).Count -ne 0) {
            throw "Exact fixture case $index must allow only its known value and exact path."
        }
    }
    Write-Output "Eight exact fixture cases passed; changed values and other paths remain detected."
} finally {
    $resolvedRoot = [IO.Path]::GetFullPath($fixtureRoot)
    $safePrefix = $temporaryBase.TrimEnd([IO.Path]::DirectorySeparatorChar, [IO.Path]::AltDirectorySeparatorChar) + [IO.Path]::DirectorySeparatorChar
    if (-not $resolvedRoot.StartsWith($safePrefix, [StringComparison]::OrdinalIgnoreCase)) {
        throw 'Fixture cleanup target escaped the temporary directory.'
    }
    if (Test-Path -LiteralPath $resolvedRoot) { Remove-Item -LiteralPath $resolvedRoot -Recurse -Force }
}
$global:LASTEXITCODE = 0
