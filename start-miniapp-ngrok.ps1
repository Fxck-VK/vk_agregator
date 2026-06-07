# Backward-compatible entrypoint for VK Mini App local dev.
# Canonical scripts live under scripts/dev/ (start-miniapp.ps1, stop-miniapp.ps1, status-miniapp.ps1).
param(
  [int]$ApiPort = 8080,
  [int]$MiniAppPort = 5173,
  [int]$WorkerMetricsPort = 9090,
  [int]$VkUserID = 777,
  [string]$EnvFile = ".env",
  [switch]$Migrate,
  [Alias("NoTunnel")]
  [switch]$NoNgrok,
  [switch]$CheckOnly,
  [switch]$NoWait,
  [switch]$OpenBrowser,
  [switch]$StopOnly
)

$ErrorActionPreference = "Stop"
$scriptDir = Join-Path $PSScriptRoot "scripts\dev"

if ($StopOnly) {
  & (Join-Path $scriptDir "stop-miniapp.ps1")
  exit $LASTEXITCODE
}

if ($CheckOnly) {
  . (Join-Path $scriptDir "_miniapp-common.ps1")
  $root = Get-RepoRoot
  Import-MiniAppDevEnv -Root $root
  Set-MiniAppDevDefaults -ApiPort $ApiPort -WorkerMetricsPort $WorkerMetricsPort
  Assert-MiniAppDevRequirements -RequireTunnel:(-not $NoNgrok)
  Write-Host "CheckOnly completed."
  exit 0
}

if ($EnvFile -ne ".env") {
  Write-Warning "EnvFile is deprecated. Load env via .env and optional .env.ps1 in the repo root."
}

$params = @{
  ApiPort = $ApiPort
  MiniAppPort = $MiniAppPort
  WorkerMetricsPort = $WorkerMetricsPort
  VkUserID = $VkUserID
  TimeoutSeconds = 120
}
if ($NoNgrok) { $params.NoTunnel = $true }
if ($NoWait) { $params.NoWait = $true }
if ($OpenBrowser) { $params.OpenBrowser = $true }
if (-not $Migrate) {
  # Legacy -Migrate flag is kept for compatibility; migrations run by default like start-bot.ps1.
}

& (Join-Path $scriptDir "start-miniapp.ps1") @params
exit $LASTEXITCODE
