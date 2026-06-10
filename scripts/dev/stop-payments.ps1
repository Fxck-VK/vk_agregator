param(
    [switch]$StopDocker,
    [switch]$Quiet
)

$ErrorActionPreference = "Stop"
. (Join-Path $PSScriptRoot "_payments-common.ps1")

$root = Get-RepoRoot
$runtime = Get-PaymentsRuntimeDir -Root $root

if (-not $Quiet) {
    Write-Host "Stopping local payment webhook service..."
}

Stop-PidFileProcess -PidFile (Join-Path $runtime "provider-webhook.pid")

$webhookAddr = Get-ConfigValue -Root $root -Name "PAYMENT_WEBHOOK_ADDR" -Default ":8082"
$webhookPort = Get-PortFromListenAddr -Addr $webhookAddr
if ($null -ne $webhookPort) {
    Stop-ListenerOnPort -Port $webhookPort
}

Stop-PaymentCommandLineProcesses -Root $root

if ($StopDocker) {
    if (-not $Quiet) {
        Write-Host "Stopping payment dependency: postgres..."
    }
    Push-Location $root
    try {
        docker compose stop postgres | Out-Host
    } finally {
        Pop-Location
    }
}

if (-not $Quiet) {
    Write-Host "Payment webhook service stopped."
}
