[CmdletBinding()]
param()

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "../..")).Path
Import-Module (Join-Path $PSScriptRoot "TrivyFilesystemScope.psm1") -Force
Assert-TrivyFilesystemScanScope -RepositoryRoot $repoRoot -ConfigPath (Join-Path $repoRoot "trivy-filesystem.yaml")
Write-Host "Trivy filesystem exclusions preserve all tracked files."
