[CmdletBinding()]
param()

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..")
Set-Location $repoRoot

function Fail {
    param([Parameter(Mandatory = $true)][string]$Message)
    throw "docs validation failed: $Message"
}

function Get-ExistingFiles {
    param([Parameter(Mandatory = $true)][string[]]$Paths)

    foreach ($path in $Paths) {
        if (Test-Path -LiteralPath $path -PathType Leaf) {
            (Resolve-Path -LiteralPath $path).Path
        }
    }
}

$routingFiles = @(
    Get-ExistingFiles -Paths @(
        "AGENTS.md",
        "README.md",
        "RUNBOOK.md",
        "TASKS.md",
        "DECISIONS.md",
        ".agents/state.json",
        "docs/INDEX.md",
        "docs/HANDOFF_CURRENT.md"
    )
)

if (Test-Path -LiteralPath "docs/runbooks" -PathType Container) {
    $routingFiles += @(
        Get-ChildItem -LiteralPath "docs/runbooks" -File -Filter "*.md" |
            ForEach-Object { $_.FullName }
    )
}

$activeDocFiles = @($routingFiles)
if (Test-Path -LiteralPath "docs" -PathType Container) {
    $activeDocFiles += @(
        Get-ChildItem -LiteralPath "docs" -File -Filter "*.md" |
            ForEach-Object { $_.FullName }
    )
}
$activeDocFiles = @($activeDocFiles | Sort-Object -Unique)

if ($activeDocFiles.Count -eq 0) {
    Fail "no active documentation files found"
}

$todoHits = @(
    Select-String -LiteralPath $activeDocFiles -Pattern "TODO.*актуализ|актуализ.*TODO" -ErrorAction SilentlyContinue
)
if ($todoHits.Count -gt 0) {
    $todoHits | ForEach-Object {
        Write-Error ("{0}:{1}: {2}" -f $_.Path, $_.LineNumber, $_.Line)
    }
    Fail "active docs contain TODO актуализировать markers"
}

if (Test-Path -LiteralPath "docs/merge" -PathType Container) {
    $mergeDocs = @(
        Get-ChildItem -LiteralPath "docs/merge" -Recurse -File -Filter "*.md" |
            ForEach-Object { $_.FullName }
    )
    if ($mergeDocs.Count -gt 0) {
        $mergeDocs | ForEach-Object { Write-Error $_ }
        Fail "handoff/merge docs must not live under active docs/merge"
    }
}

$activeHandoffFiles = @()
if (Test-Path -LiteralPath "docs" -PathType Container) {
    $activeHandoffFiles = @(
        Get-ChildItem -LiteralPath "docs" -Recurse -File -Filter "*.md" |
            Where-Object {
                $relative = (Resolve-Path -LiteralPath $_.FullName -Relative).Replace("\", "/").TrimStart(".", "/")
                $relative -notlike "docs/archive/*" -and
                $relative -ne "docs/HANDOFF_CURRENT.md" -and
                $_.Name -match "(?i)(handoff|context|merge).*\.md$"
            } |
            ForEach-Object { (Resolve-Path -LiteralPath $_.FullName -Relative).Replace("\", "/").TrimStart(".", "/") }
    )
}
if ($activeHandoffFiles.Count -gt 0) {
    $activeHandoffFiles | ForEach-Object { Write-Error $_ }
    Fail "handoff/context/merge files must be archived under docs/archive/handoffs"
}

function Get-MarkdownRefs {
    param([Parameter(Mandatory = $true)][string]$Path)

    $content = Get-Content -LiteralPath $Path -Raw
    $pattern = "((docs|\.agents)/[A-Za-z0-9._/-]+\.md|[A-Z][A-Z0-9_/-]*\.md)"
    [regex]::Matches($content, $pattern) |
        ForEach-Object { $_.Value.TrimStart("./") } |
        Sort-Object -Unique
}

$missingRefs = New-Object System.Collections.Generic.List[string]
foreach ($file in $routingFiles) {
    $relativeFile = (Resolve-Path -LiteralPath $file -Relative).Replace("\", "/").TrimStart(".", "/")
    foreach ($ref in Get-MarkdownRefs -Path $file) {
        if ([string]::IsNullOrWhiteSpace($ref) -or $ref.Contains("<") -or $ref.Contains(">")) {
            continue
        }
        if (-not (Test-Path -LiteralPath $ref)) {
            $missingRefs.Add("$relativeFile -> $ref")
        }
    }
}
if ($missingRefs.Count -gt 0) {
    $missingRefs | ForEach-Object { Write-Error $_ }
    Fail "routing docs reference missing markdown files"
}

$badAgentsRefs = @(
    Get-MarkdownRefs -Path "AGENTS.md" |
        Where-Object { $_ -match "^docs/(archive|merge)/" }
)
if ($badAgentsRefs.Count -gt 0) {
    $badAgentsRefs | ForEach-Object { Write-Error $_ }
    Fail "AGENTS.md must not reference archived or merge docs as active sources"
}

$index = Get-Content -LiteralPath "docs/INDEX.md" -Raw
if (-not $index.Contains("docs/HANDOFF_CURRENT.md")) {
    Fail "docs/INDEX.md must list docs/HANDOFF_CURRENT.md as the only active handoff slot"
}
if (-not $index.Contains("Status: archived")) {
    Fail "docs/INDEX.md must document deprecated docs status marker"
}

Write-Host "documentation validation OK"
