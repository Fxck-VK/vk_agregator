[CmdletBinding()]
param()

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "../..")).Path
$fixtureParent = [IO.Path]::GetFullPath((Join-Path $repoRoot ".cache"))
$fixtureRoot = Join-Path $fixtureParent ("docs-validation-" + [guid]::NewGuid().ToString("N"))

function Write-FixtureFile {
    param([string]$Path, [string]$Content)

    $target = Join-Path $fixtureRoot $Path
    New-Item -ItemType Directory -Path (Split-Path $target -Parent) -Force | Out-Null
    Set-Content -LiteralPath $target -Value $Content -Encoding utf8
}

function Assert-Validation {
    param([string]$Name, [string]$Reference, [string]$ExpectedError = "")

    Write-FixtureFile "AGENTS.md" $Reference
    $failure = ""
    Push-Location $fixtureRoot
    try {
        & (Join-Path $fixtureRoot "scripts/ci/validate-docs.ps1") *> $null
    }
    catch {
        $failure = $_.Exception.Message
    }
    finally {
        Pop-Location
    }

    if ($ExpectedError -eq "" -and $failure -ne "") {
        throw "${Name}: expected success, got $failure"
    }
    if ($ExpectedError -ne "" -and -not $failure.Contains($ExpectedError)) {
        throw "${Name}: expected '$ExpectedError', got '$failure'"
    }
    Write-Host "PASS: $Name"
}

try {
    Write-FixtureFile "scripts/ci/validate-docs.ps1" (Get-Content -LiteralPath (Join-Path $PSScriptRoot "validate-docs.ps1") -Raw)
    Write-FixtureFile "docs/INDEX.md" 'docs/HANDOFF_CURRENT.md; Status: archived'
    Write-FixtureFile "docs/HANDOFF_CURRENT.md" 'Current handoff'
    Write-FixtureFile "README.md" 'Project readme'
    Write-FixtureFile "web/platform/AGENTS.md" 'Local rules'
    Write-FixtureFile "web/platform/docs/ui-index.md" 'UI index'
    Write-FixtureFile ".agents/RULES.md" 'Agent rules'
    Write-FixtureFile "docs/archive/old.md" 'Archived document'

    Assert-Validation "nested UI index keeps its full path" '`web/platform/docs/ui-index.md`'
    Assert-Validation "nested local rules keep their full path" '[local rules](web/platform/AGENTS.md)'
    Assert-Validation "root documentation references still work" '`README.md` and `docs/INDEX.md`'
    Assert-Validation "hidden directory names keep their leading dot" '`.agents/RULES.md`'
    Assert-Validation "missing nested file cannot match a root file" '`web/platform/README.md`' 'web/platform/README.md'
    Assert-Validation "missing nested UI document is rejected" '`web/platform/docs/missing.md`' 'web/platform/docs/missing.md'
    Assert-Validation "missing root document is rejected" '`docs/missing.md`' 'docs/missing.md'
    Assert-Validation "archived documents remain forbidden in agent routing" '`docs/archive/old.md`' 'docs/archive/old.md'
}
finally {
    $resolvedFixture = [IO.Path]::GetFullPath($fixtureRoot)
    if ((Split-Path $resolvedFixture -Parent) -ne $fixtureParent -or
        -not (Split-Path $resolvedFixture -Leaf).StartsWith("docs-validation-")) {
        throw "Refusing to clean a directory outside the documentation test fixtures."
    }
    if (Test-Path -LiteralPath $resolvedFixture) {
        Remove-Item -LiteralPath $resolvedFixture -Recurse -Force
    }
}

Write-Host "documentation validation tests OK"
