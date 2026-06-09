param(
    [switch]$SkipDocker,
    [switch]$SkipMigrate,
    [switch]$NoRestart,
    [switch]$SkipPublicCheck,
    [string]$PublicWebhookUrl = "",
    [int]$TimeoutSeconds = 120
)

$ErrorActionPreference = "Stop"
. (Join-Path $PSScriptRoot "_payments-common.ps1")

$root = Get-RepoRoot
$runtime = Get-PaymentsRuntimeDir -Root $root
$bin = Get-PaymentsBinDir -Root $root

Ensure-Directory -Path $runtime
Ensure-Directory -Path $bin

Push-Location $root
try {
    if (-not $NoRestart) {
        & (Join-Path $PSScriptRoot "stop-payments.ps1") -Quiet
    }

    if (-not $SkipDocker) {
        Write-Host "Starting payment dependency: postgres..."
        Ensure-DockerRunning -TimeoutSeconds $TimeoutSeconds
        docker compose up -d postgres | Out-Host
        Wait-PaymentsPostgres -Root $root -TimeoutSeconds $TimeoutSeconds
    }

    if (-not $SkipMigrate) {
        Write-Host "Applying database migrations..."
        go run ./cmd/migrate up | Out-Host
    }

    Write-Host "Building payment webhook binary..."
    $webhookExe = Join-Path $bin "provider-webhook.exe"
    go build -o $webhookExe ./cmd/provider-webhook

    Write-Host "Starting payment webhook service..."
    $webhookProc = Start-BotExecutable `
        -Root $root `
        -ExePath $webhookExe `
        -StdoutPath (Join-Path $runtime "provider-webhook-live.log") `
        -StderrPath (Join-Path $runtime "provider-webhook-live.err")
    Set-Content -Path (Join-Path $runtime "provider-webhook.pid") -Value $webhookProc.Id -Encoding ASCII

    $healthUrl = Get-PaymentWebhookHealthUrl -Root $root
    Write-Host "Waiting for payment webhook health: $healthUrl"
    Wait-Http -Url $healthUrl -TimeoutSeconds $TimeoutSeconds | Out-Null

    $publicUrl = Get-PaymentWebhookPublicUrl -Root $root -Override $PublicWebhookUrl
    if (-not $SkipPublicCheck) {
        Write-Host "Checking public YooKassa webhook route: $publicUrl"
        Wait-PaymentWebhookPublicRoute -Url $publicUrl -TimeoutSeconds $TimeoutSeconds | Out-Null
    }

    Write-Host ""
    Write-Host "Payment webhook service is running."
    Write-Host "Local health:   $healthUrl"
    Write-Host "Public webhook: $publicUrl"
    if ($SkipPublicCheck) {
        Write-Host "Public check:   skipped"
    } else {
        Write-Host "Public check:   reachable"
    }
    Write-Host "Logs:           $runtime"
} finally {
    Pop-Location
}
