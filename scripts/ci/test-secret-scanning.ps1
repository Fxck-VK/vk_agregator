[CmdletBinding()]
param(
    [string]$GitleaksPath = "gitleaks"
)

$ErrorActionPreference = "Stop"

function Assert-Contains {
    param(
        [string]$Content,
        [string]$Pattern,
        [string]$Message
    )

    if ($Content -notmatch $Pattern) {
        throw $Message
    }
}

function Assert-NotContains {
    param(
        [string]$Content,
        [string]$Pattern,
        [string]$Message
    )

    if ($Content -match $Pattern) {
        throw $Message
    }
}

function Invoke-Quiet {
    param(
        [string]$Executable,
        [string[]]$Arguments,
        [string]$WorkingDirectory
    )

    Push-Location $WorkingDirectory
    try {
        $previousErrorActionPreference = $ErrorActionPreference
        $ErrorActionPreference = "Continue"
        $output = & $Executable @Arguments 2>&1
        $exitCode = $LASTEXITCODE
        $ErrorActionPreference = $previousErrorActionPreference
        return [pscustomobject]@{
            ExitCode = $exitCode
            Output = $output
        }
    }
    finally {
        Pop-Location
    }
}

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
$workflowPath = Join-Path $repoRoot ".github\workflows\ci.yml"
$ignorePath = Join-Path $repoRoot ".gitleaksignore"
$configPath = Join-Path $repoRoot ".gitleaks.toml"
$workflow = Get-Content -Raw $workflowPath
$config = Get-Content -Raw $configPath

Assert-Contains $workflow 'fetch-depth:\s*0' `
    "Secret Scan checkout must fetch full history."
Assert-Contains $workflow 'GITLEAKS_VERSION:\s*["'']?8\.30\.1["'']?' `
    "Secret Scan must pin Gitleaks v8.30.1."
Assert-Contains $workflow '551f6fc83ea457d62a0d98237cbad105af8d557003051f41f3e7ca7b3f2470eb' `
    "Secret Scan must verify the pinned Gitleaks Linux archive checksum."
Assert-Contains $workflow '(?m)^\s*gitleaks\s+dir\s+' `
    "Secret Scan must run a current-tree Gitleaks dir scan."
Assert-Contains $workflow '(?m)^\s*gitleaks\s+git\s+' `
    "Secret Scan must run a git-history Gitleaks scan."
Assert-Contains $workflow '--log-opts=' `
    "Secret Scan must scan the pull-request commit range."
Assert-Contains $workflow 'github\.event\.pull_request\.base\.sha' `
    "Secret Scan must derive the pull-request range from the base SHA."
Assert-Contains $workflow 'github\.event\.pull_request\.head\.sha' `
    "Secret Scan must derive the pull-request range from the head SHA."
Assert-Contains $workflow 'test-secret-scanning\.ps1' `
    "Secret Scan must run the committed-history canary."
Assert-Contains $workflow '--redact' `
    "Every Gitleaks invocation must redact findings."
Assert-NotContains $workflow '(?i)(?:@|:)latest\b' `
    "Secret Scan must not use mutable latest scanner versions."

$activeIgnores = @(
    Get-Content $ignorePath |
        Where-Object { $_.Trim() -ne "" -and -not $_.TrimStart().StartsWith("#") }
)
if ($activeIgnores.Count -ne 0) {
    throw ".gitleaksignore must not contain fingerprint, commit, path, or line ignores."
}

Assert-NotContains $config '(?i)\(\?:change\[-_\]\?me\|example\|placeholder\|loadtest\|test\|dev\|mock' `
    "Gitleaks allowlists must not accept broad placeholder keywords."
Assert-NotContains $config '(?i)authorization\|bearer\|token\|secret\|key\|credential\|auth\|header' `
    "Gitleaks allowlists must not suppress generic authentication field names."

$policyCanaryRoot = Join-Path ([IO.Path]::GetTempPath()) ("vk-aggregator-allowlist-canary-" + [guid]::NewGuid().ToString("N"))
$policyCanaryReport = Join-Path $policyCanaryRoot "policy-canary-report.json"
New-Item -ItemType Directory -Path (Join-Path $policyCanaryRoot "internal\adapter\inbound\admin") -Force | Out-Null

try {
    $policyCanaryPrefix = "SECURITY_SCAN_" + "CANARY_"
    Set-Content -LiteralPath (Join-Path $policyCanaryRoot "RUNBOOK.md") -Value @(
        "ADMIN_TOKEN=" + $policyCanaryPrefix + "0123456789ABCDEF",
        "APIMART_API_KEY=" + $policyCanaryPrefix + "1123456789ABCDEF",
        "POYO_API_KEY=" + $policyCanaryPrefix + "2123456789ABCDEF",
        "RUNWAYML_API_SECRET=" + $policyCanaryPrefix + "3123456789ABCDEF",
        "GHCR_TOKEN=" + $policyCanaryPrefix + "4123456789ABCDEF",
        "CLOUDFLARED_TUNNEL_TOKEN=" + $policyCanaryPrefix + "5123456789ABCDEF",
        "ACCOUNT_EMAIL_SMTP_PASSWORD=" + $policyCanaryPrefix + "6123456789ABCDEF",
        "S3_SECRET_KEY=" + $policyCanaryPrefix + "7123456789ABCDEF",
        "POSTGRES_PASSWORD=" + $policyCanaryPrefix + "8123456789ABCDEF"
    ) -Encoding ascii
    Set-Content -LiteralPath (Join-Path $policyCanaryRoot "internal\adapter\inbound\admin\handler_test.go") `
        -Value ("ADMIN_TOKEN=" + $policyCanaryPrefix + "FEDCBA9876543210") -Encoding ascii

    $policyResult = Invoke-Quiet -Executable $GitleaksPath -WorkingDirectory $policyCanaryRoot -Arguments @(
        "dir",
        "--config", $configPath,
        "--redact",
        "--no-banner",
        "--no-color",
        "--report-format", "json",
        "--report-path", $policyCanaryReport,
        "."
    )
    if ($policyResult.ExitCode -ne 1) {
        throw "Synthetic secrets in documentation and test paths must not be suppressed by allowlists."
    }

    $policyFindings = Get-Content -Raw $policyCanaryReport | ConvertFrom-Json
    if ($policyFindings.Count -ne 10) {
        throw "Allowlist policy canary must return exactly ten synthetic findings; got $($policyFindings.Count)."
    }
    foreach ($finding in $policyFindings) {
        if ($finding.RuleID -ne "project-secret-assignment") {
            throw "Allowlist policy canary returned an unexpected rule classification."
        }
    }

    Remove-Item -LiteralPath (Join-Path $policyCanaryRoot "RUNBOOK.md") -Force
    Remove-Item -LiteralPath (Join-Path $policyCanaryRoot "internal") -Recurse -Force
    $placeholderEnvName = ".env.prod" + ".example"
    Set-Content -LiteralPath (Join-Path $policyCanaryRoot $placeholderEnvName) -Value @(
        'ADMIN_TOKEN=<ADMIN_TOKEN>',
        'VK_ACCESS_TOKEN=<VK_ACCESS_TOKEN>',
        'YOOKASSA_SECRET_KEY=<YOOKASSA_SECRET_KEY>'
    ) -Encoding ascii

    $placeholderResult = Invoke-Quiet -Executable $GitleaksPath -WorkingDirectory $policyCanaryRoot -Arguments @(
        "dir",
        "--config", $configPath,
        "--redact",
        "--no-banner",
        "--no-color",
        "."
    )
    if ($placeholderResult.ExitCode -ne 0) {
        throw "Exact committed environment placeholders must remain clean."
    }

    Write-Output "Allowlist policy canary detected all ten synthetic findings; exact placeholders remained clean."
}
finally {
    if (Test-Path $policyCanaryRoot) {
        Remove-Item -LiteralPath $policyCanaryRoot -Recurse -Force
    }
}

$idempotencyCanaryRoot = Join-Path ([IO.Path]::GetTempPath()) ("vk-aggregator-idempotency-canary-" + [guid]::NewGuid().ToString("N"))
$idempotencyCanaryRelativePaths = @(
    "web\platform\src\features\workspace\WorkspacePrompt\WorkspacePrompt.test.tsx",
    "web\platform\src\features\conversations\NewConversationButton\NewConversationButton.test.tsx",
    "web\platform\src\lib\web-api\proxy.test.ts",
    "web\platform\src\features\conversations\pending-conversation-bootstrap.test.ts",
    "web\platform\src\features\conversations\PendingConversationBootstrap\PendingConversationBootstrap.test.tsx"
)
$idempotencyCanaryPaths = @(
    $idempotencyCanaryRelativePaths | ForEach-Object { Join-Path $idempotencyCanaryRoot $_ }
)
$idempotencyCanaryReport = Join-Path $idempotencyCanaryRoot "idempotency-canary-report.json"
foreach ($idempotencyCanaryPath in $idempotencyCanaryPaths) {
    New-Item -ItemType Directory -Path (Split-Path -Parent $idempotencyCanaryPath) -Force | Out-Null
}

try {
    $idempotencyUuid = "9f4a6c2e" + "-7b81-4d35-a9c6-" + "2e8f1b7d5a30"
    foreach ($idempotencyCanaryPath in $idempotencyCanaryPaths) {
        Set-Content -LiteralPath $idempotencyCanaryPath `
            -Value ('const headers = { "X-Idempotency-Key": "' + $idempotencyUuid + '" };') `
            -Encoding ascii
    }

    $idempotencyResult = Invoke-Quiet -Executable $GitleaksPath -WorkingDirectory $idempotencyCanaryRoot -Arguments @(
        "dir",
        "--config", $configPath,
        "--redact",
        "--no-banner",
        "--no-color",
        "."
    )
    if ($idempotencyResult.ExitCode -ne 0) {
        throw "UUID request identifiers in the exact historical web test paths must not be classified as API keys."
    }

    $genericApiCanary = "9f4a6c2e" + "7b814d35" + "a9c62e8f" + "1b7d5a30"
    Set-Content -LiteralPath $idempotencyCanaryPaths[-1] `
        -Value ('const apiKey = "' + $genericApiCanary + '";') `
        -Encoding ascii

    $genericResult = Invoke-Quiet -Executable $GitleaksPath -WorkingDirectory $idempotencyCanaryRoot -Arguments @(
        "dir",
        "--config", $configPath,
        "--redact",
        "--no-banner",
        "--no-color",
        "--report-format", "json",
        "--report-path", $idempotencyCanaryReport,
        "."
    )
    if ($genericResult.ExitCode -ne 1) {
        throw "The exact historical test-path allowlist must continue detecting non-UUID generic API keys."
    }

    $genericFindings = @(Get-Content -Raw $idempotencyCanaryReport | ConvertFrom-Json)
    if ($genericFindings.Count -ne 1 -or $genericFindings[0].RuleID -ne "generic-api-key") {
        throw "The idempotency allowlist canary returned an unexpected finding classification."
    }

    Write-Output "Exact historical test-path UUID identifiers remained clean; a non-UUID generic API key was still detected."
}
finally {
    if (Test-Path $idempotencyCanaryRoot) {
        $resolvedIdempotencyCanaryRoot = [IO.Path]::GetFullPath($idempotencyCanaryRoot)
        $systemTemp = [IO.Path]::GetFullPath([IO.Path]::GetTempPath())
        if ($resolvedIdempotencyCanaryRoot.StartsWith($systemTemp, [StringComparison]::OrdinalIgnoreCase)) {
            Remove-Item -LiteralPath $resolvedIdempotencyCanaryRoot -Recurse -Force
        }
    }
}

$tempRoot = Join-Path ([IO.Path]::GetTempPath()) ("vk-aggregator-secret-canary-" + [guid]::NewGuid().ToString("N"))
$canaryConfig = Join-Path $tempRoot ".gitleaks.toml"
$canaryReport = Join-Path $tempRoot "canary-report.json"

New-Item -ItemType Directory -Path $tempRoot | Out-Null

try {
    $canaryConfigContent = @'
title = "Synthetic deleted-secret history canary"

[[rules]]
id = "secret-scan-history-canary"
description = "Synthetic marker used only to verify deleted history coverage"
regex = '''SECRET_SCAN_HISTORY_CANARY_[A-F0-9]{24}'''
'@
    [IO.File]::WriteAllText($canaryConfig, $canaryConfigContent, [Text.UTF8Encoding]::new($false))

    & git -C $tempRoot init --quiet
    & git -C $tempRoot config user.email "secret-scan-canary@invalid.example"
    & git -C $tempRoot config user.name "Secret Scan Canary"

    $canaryPath = Join-Path $tempRoot "deleted-canary.txt"
    Set-Content -LiteralPath $canaryPath -Value "SECRET_SCAN_HISTORY_CANARY_0123456789ABCDEF01234567" -Encoding ascii
    & git -C $tempRoot add deleted-canary.txt
    & git -C $tempRoot commit --quiet -m "add synthetic history canary"
    Remove-Item -LiteralPath $canaryPath -Force
    & git -C $tempRoot add -u
    & git -C $tempRoot commit --quiet -m "delete synthetic history canary"

    $treeResult = Invoke-Quiet -Executable $GitleaksPath -WorkingDirectory $tempRoot -Arguments @(
        "dir",
        "--config", $canaryConfig,
        "--redact",
        "--no-banner",
        "--no-color",
        "."
    )
    if ($treeResult.ExitCode -ne 0) {
        throw "Synthetic repository current tree must be clean."
    }

    $historyResult = Invoke-Quiet -Executable $GitleaksPath -WorkingDirectory $tempRoot -Arguments @(
        "git",
        "--config", $canaryConfig,
        "--redact",
        "--no-banner",
        "--no-color",
        "--report-format", "json",
        "--report-path", $canaryReport,
        "."
    )
    if ($historyResult.ExitCode -ne 1) {
        throw "Synthetic add-then-delete canary must be detected in git history."
    }
    if (-not (Test-Path $canaryReport)) {
        throw "Synthetic history scan did not produce its temporary report."
    }

    $findings = Get-Content -Raw $canaryReport | ConvertFrom-Json
    if ($findings.Count -ne 1) {
        throw "Synthetic history scan must return exactly one canary finding; got $($findings.Count)."
    }

    $finding = $findings[0]
    if ($finding.RuleID -ne "secret-scan-history-canary" -or $finding.File -ne "deleted-canary.txt") {
        throw "Synthetic history scan returned an unexpected safe finding classification."
    }

    Write-Output ("Canary detected: rule={0} path={1} line={2} classification=synthetic-deleted-history-canary" -f `
        $finding.RuleID, $finding.File, $finding.StartLine)
    Write-Output "Secret scanning policy and canary checks passed."
}
finally {
    if (Test-Path $tempRoot) {
        $resolvedTemp = [IO.Path]::GetFullPath($tempRoot)
        $systemTemp = [IO.Path]::GetFullPath([IO.Path]::GetTempPath())
        if ($resolvedTemp.StartsWith($systemTemp, [StringComparison]::OrdinalIgnoreCase)) {
            Remove-Item -LiteralPath $resolvedTemp -Recurse -Force
        }
    }
}

# Expected scanner detections use exit code 1; do not leak that native status
# as this policy test's final process exit code after every assertion passed.
$global:LASTEXITCODE = 0
