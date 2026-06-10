param(
    [string]$PublicWebhookUrl = ""
)

$ErrorActionPreference = "Stop"
. (Join-Path $PSScriptRoot "_payments-common.ps1")

$root = Get-RepoRoot
$runtime = Get-PaymentsRuntimeDir -Root $root

function Test-ProcessId {
    param([string]$Name)

    $pidFile = Join-Path $runtime "$Name.pid"
    if (-not (Test-Path -LiteralPath $pidFile)) {
        return "not tracked"
    }
    $raw = Get-Content -LiteralPath $pidFile -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($raw -notmatch '^\d+$') {
        return "invalid pid"
    }
    try {
        $proc = Get-Process -Id ([int]$raw) -ErrorAction Stop
        return "running pid=$($proc.Id)"
    } catch {
        return "not running pid=$raw"
    }
}

function Test-HttpStatus {
    param([string]$Url)

    try {
        $response = Invoke-WebRequest -Uri $Url -UseBasicParsing -TimeoutSec 5
        return "ok status=$($response.StatusCode)"
    } catch {
        return "failed $($_.Exception.Message)"
    }
}

$healthUrl = Get-PaymentWebhookHealthUrl -Root $root
$publicUrl = Get-PaymentWebhookPublicUrl -Root $root -Override $PublicWebhookUrl
$provider = Get-ConfigValue -Root $root -Name "PAYMENT_PROVIDER" -Default "mock"

Write-Host "Payment webhook status"
Write-Host "Repo:           $root"
Write-Host "Runtime:        $runtime"
Write-Host "Provider:       $provider"
Write-Host "Webhook:        $(Test-ProcessId -Name "provider-webhook")"
Write-Host "Local health:   $(Test-HttpStatus -Url $healthUrl)"
Write-Host "Public webhook: $(Get-PaymentWebhookPublicRouteStatus -Url $publicUrl)"
Write-Host "Public URL:     $publicUrl"

Write-Host ""
Write-Host "Docker dependency:"
Push-Location $root
try {
    $oldPreference = $ErrorActionPreference
    $ErrorActionPreference = "SilentlyContinue"
    docker info 1>$null 2>$null
    $dockerAvailable = ($LASTEXITCODE -eq 0)
    $ErrorActionPreference = $oldPreference

    if (-not $dockerAvailable) {
        Write-Host "Docker Engine is not running."
    } else {
        docker compose ps postgres
    }
} catch {
    Write-Host "Docker status failed: $($_.Exception.Message)"
} finally {
    Pop-Location
}
