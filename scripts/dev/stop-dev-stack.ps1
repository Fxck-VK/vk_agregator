[CmdletBinding()]
param(
    [string]$EnvFile = "dev.env",
    [string]$ProjectName = "vk-ai-aggregator-dev"
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$startScript = Join-Path $PSScriptRoot "start-dev-stack.ps1"
if (-not (Test-Path -LiteralPath $startScript)) {
    throw "DEV stack start script is missing: $startScript"
}

& $startScript -EnvFile $EnvFile -ProjectName $ProjectName -StopOnly
if ($LASTEXITCODE -ne 0) {
    exit $LASTEXITCODE
}
