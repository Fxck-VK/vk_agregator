param(
    [string]$EnvFile = "",
    [string]$VkBaseUrl = "",
    [string]$AppBaseUrl = "",
    [string]$PaymentWebhookUrl = "",
    [string]$ApiHealthUrl = "",
    [string]$WorkerHealthUrl = "",
    [string]$MaintenanceWorkerHealthUrl = "",
    [string]$ProviderWebhookHealthUrl = "",
    [string]$MiniAppHealthUrl = "",
    [string]$ReverseProxyHealthUrl = "",
    [int]$TimeoutSeconds = 10,
    [switch]$PaymentWebhookOnly,
    [switch]$SkipLocalHealth,
    [switch]$AllowInsecureHttp
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$script:EnvValues = @{}

function Import-SmokeEnvFile {
    param([string]$Path)

    if ([string]::IsNullOrWhiteSpace($Path)) {
        return
    }
    if (-not (Test-Path -LiteralPath $Path)) {
        throw "[FAIL] env file not found: $Path"
    }

    foreach ($line in Get-Content -LiteralPath $Path) {
        $trimmed = $line.Trim()
        if ($trimmed.Length -eq 0 -or $trimmed.StartsWith("#") -or -not $trimmed.Contains("=")) {
            continue
        }

        $parts = $trimmed.Split("=", 2)
        $key = $parts[0].Trim()
        if ($key -notmatch '^[A-Za-z_][A-Za-z0-9_]*$') {
            continue
        }
        $value = $parts[1].Trim().Trim('"').Trim("'")
        $script:EnvValues[$key] = $value
    }
}

function Get-SmokeEnvValue {
    param(
        [Parameter(Mandatory = $true)][string]$Name,
        [string]$Default = ""
    )

    $processValue = [Environment]::GetEnvironmentVariable($Name)
    if (-not [string]::IsNullOrWhiteSpace($processValue)) {
        return $processValue
    }
    if ($script:EnvValues.ContainsKey($Name)) {
        return $script:EnvValues[$Name]
    }
    return $Default
}

function Get-SmokeUrlFromListenAddr {
    param(
        [Parameter(Mandatory = $true)][string]$Address,
        [Parameter(Mandatory = $true)][string]$Path
    )

    if ($Address.StartsWith("http://") -or $Address.StartsWith("https://")) {
        return "$($Address.TrimEnd('/'))$Path"
    }
    if ($Address.StartsWith(":")) {
        return "http://127.0.0.1$Address$Path"
    }
    if ($Address -match '^0\.0\.0\.0:(?<port>\d+)$' -or $Address -match '^\[::\]:(?<port>\d+)$') {
        return "http://127.0.0.1:$($Matches.port)$Path"
    }
    return "http://$Address$Path"
}

function Normalize-BaseUrl {
    param([Parameter(Mandatory = $true)][string]$Value)
    return $Value.TrimEnd("/")
}

function Assert-HttpsUrl {
    param(
        [Parameter(Mandatory = $true)][string]$Name,
        [Parameter(Mandatory = $true)][string]$Url
    )
    if ($AllowInsecureHttp) {
        return
    }
    try {
        $uri = [Uri]$Url
    } catch {
        throw "$Name is not a valid URL: $Url"
    }
    if ($uri.Scheme -ne "https") {
        throw "$Name must use https in production smoke checks: $Url"
    }
}

function Invoke-SmokeRequest {
    param(
        [Parameter(Mandatory = $true)][string]$Name,
        [Parameter(Mandatory = $true)][string]$Method,
        [Parameter(Mandatory = $true)][string]$Url,
        [string]$Body
    )

    try {
        $parameters = @{
            Method          = $Method
            Uri             = $Url
            TimeoutSec      = $TimeoutSeconds
            UseBasicParsing = $true
            ErrorAction     = "Stop"
        }
        if ($PSBoundParameters.ContainsKey("Body")) {
            $parameters["Body"] = $Body
            $parameters["ContentType"] = "application/json"
        }

        $response = Invoke-WebRequest @parameters
        return [int]$response.StatusCode
    } catch {
        if ($_.Exception.Response -and $_.Exception.Response.StatusCode) {
            return [int]$_.Exception.Response.StatusCode
        }
        Write-Host "[WARN] $Name returned no HTTP status: $($_.Exception.Message)"
        return 0
    }
}

function Assert-2xx {
    param([string]$Name, [int]$Status)
    if ($Status -lt 200 -or $Status -ge 300) {
        throw "$Name expected 2xx, got $Status"
    }
    Write-Host "[OK] $Name -> $Status"
}

function Assert-Blocked {
    param([string]$Name, [int]$Status)
    if ($Status -ge 200 -and $Status -lt 300) {
        throw "$Name is publicly exposed with $Status"
    }
    if ($Status -ge 500 -or $Status -eq 0) {
        throw "$Name expected blocked non-2xx status, got $Status"
    }
    Write-Host "[OK] $Name blocked -> $Status"
}

function Assert-AuthRequired {
    param([string]$Name, [int]$Status)
    if ($Status -ge 200 -and $Status -lt 300) {
        throw "$Name is public without Mini App auth, got $Status"
    }
    if ($Status -eq 404 -or $Status -ge 500 -or $Status -eq 0) {
        throw "$Name expected auth/client rejection, got $Status"
    }
    Write-Host "[OK] $Name requires auth -> $Status"
}

function Assert-ControlledRouteResponse {
    param([string]$Name, [int]$Status)
    if ($Status -in @(521, 522, 523, 530)) {
        throw "$Name hit Cloudflare/origin error $Status; check tunnel connector and reverse proxy origin"
    }
    if ($Status -eq 404 -or $Status -eq 405 -or $Status -ge 500 -or $Status -eq 0) {
        throw "$Name did not reach the expected handler cleanly, got $Status"
    }
    Write-Host "[OK] $Name reached handler safely -> $Status"
}

function Assert-ControlledWebhookReject {
    param([string]$Name, [int]$Status)
    if ($Status -ge 200 -and $Status -lt 300) {
        throw "$Name accepted an invalid webhook body with $Status"
    }
    Assert-ControlledRouteResponse -Name $Name -Status $Status
}

Import-SmokeEnvFile -Path $EnvFile

if ([string]::IsNullOrWhiteSpace($VkBaseUrl)) {
    $VkBaseUrl = Get-SmokeEnvValue -Name "PUBLIC_VK_BASE_URL" -Default (Get-SmokeEnvValue -Name "VK_BASE_URL" -Default "https://vk.neiirohub.ru")
}
if ([string]::IsNullOrWhiteSpace($AppBaseUrl)) {
    $AppBaseUrl = Get-SmokeEnvValue -Name "PUBLIC_APP_BASE_URL" -Default (Get-SmokeEnvValue -Name "APP_BASE_URL" -Default "https://app.neiirohub.ru")
}
if ([string]::IsNullOrWhiteSpace($PaymentWebhookUrl)) {
    $PaymentWebhookUrl = Get-SmokeEnvValue -Name "PUBLIC_PAYMENT_WEBHOOK_URL" -Default (Get-SmokeEnvValue -Name "PAYMENT_WEBHOOK_URL" -Default "https://neiirohub.ru/billing/webhooks/yookassa")
}
if ([string]::IsNullOrWhiteSpace($ApiHealthUrl)) {
    $ApiHealthUrl = Get-SmokeUrlFromListenAddr -Address (Get-SmokeEnvValue -Name "HTTP_ADDR" -Default ":8080") -Path "/readyz"
}
if ([string]::IsNullOrWhiteSpace($WorkerHealthUrl)) {
    $WorkerHealthUrl = Get-SmokeUrlFromListenAddr -Address (Get-SmokeEnvValue -Name "WORKER_METRICS_ADDR" -Default ":9090") -Path "/readyz"
}
if ([string]::IsNullOrWhiteSpace($MaintenanceWorkerHealthUrl)) {
    $MaintenanceWorkerHealthUrl = "http://127.0.0.1:9091/readyz"
}
if ([string]::IsNullOrWhiteSpace($ProviderWebhookHealthUrl)) {
    $ProviderWebhookHealthUrl = Get-SmokeUrlFromListenAddr -Address (Get-SmokeEnvValue -Name "PAYMENT_WEBHOOK_ADDR" -Default ":8082") -Path "/readyz"
}
if ([string]::IsNullOrWhiteSpace($MiniAppHealthUrl)) {
    $MiniAppHealthUrl = "http://127.0.0.1:5173/"
}
if ([string]::IsNullOrWhiteSpace($ReverseProxyHealthUrl)) {
    $ReverseProxyHealthUrl = "http://127.0.0.1:$(Get-SmokeEnvValue -Name "REVERSE_PROXY_HTTP_PORT" -Default "8088")/proxy-health"
}

$VkBaseUrl = Normalize-BaseUrl $VkBaseUrl
$AppBaseUrl = Normalize-BaseUrl $AppBaseUrl

Assert-HttpsUrl -Name "VK base URL" -Url $VkBaseUrl
Assert-HttpsUrl -Name "Mini App base URL" -Url $AppBaseUrl
Assert-HttpsUrl -Name "YooKassa webhook URL" -Url $PaymentWebhookUrl

Write-Host "Running safe production smoke checks"
if (-not [string]::IsNullOrWhiteSpace($EnvFile)) {
    Write-Host "Env file: $EnvFile"
}
Write-Host "VK base: $VkBaseUrl"
Write-Host "Mini App base: $AppBaseUrl"
Write-Host "Payment webhook: $PaymentWebhookUrl"

if (-not $PaymentWebhookOnly -and -not $SkipLocalHealth) {
    $status = Invoke-SmokeRequest -Name "API local health" -Method "GET" -Url $ApiHealthUrl
    Assert-2xx -Name "API local health" -Status $status

    $status = Invoke-SmokeRequest -Name "Worker local health" -Method "GET" -Url $WorkerHealthUrl
    Assert-2xx -Name "Worker local health" -Status $status

    $status = Invoke-SmokeRequest -Name "Maintenance worker local health" -Method "GET" -Url $MaintenanceWorkerHealthUrl
    Assert-2xx -Name "Maintenance worker local health" -Status $status

    $status = Invoke-SmokeRequest -Name "Provider webhook local health" -Method "GET" -Url $ProviderWebhookHealthUrl
    Assert-2xx -Name "Provider webhook local health" -Status $status

    $status = Invoke-SmokeRequest -Name "Mini App local health" -Method "GET" -Url $MiniAppHealthUrl
    Assert-2xx -Name "Mini App local health" -Status $status

    $status = Invoke-SmokeRequest -Name "Reverse proxy local health" -Method "GET" -Url $ReverseProxyHealthUrl
    Assert-2xx -Name "Reverse proxy local health" -Status $status
}

if (-not $PaymentWebhookOnly) {
    $status = Invoke-SmokeRequest -Name "Public API health" -Method "GET" -Url "$VkBaseUrl/health"
    Assert-2xx -Name "Public API health" -Status $status

    $status = Invoke-SmokeRequest -Name "VK webhook route" -Method "POST" -Url "$VkBaseUrl/webhooks/vk" -Body "{}"
    Assert-ControlledRouteResponse -Name "VK webhook route" -Status $status

    $status = Invoke-SmokeRequest -Name "Public Mini App open" -Method "GET" -Url "$AppBaseUrl/"
    Assert-2xx -Name "Public Mini App open" -Status $status

    $status = Invoke-SmokeRequest -Name "Mini App /miniapp/balance" -Method "GET" -Url "$AppBaseUrl/miniapp/balance"
    Assert-AuthRequired -Name "Mini App /miniapp/balance" -Status $status
}

$status = Invoke-SmokeRequest -Name "YooKassa webhook route" -Method "POST" -Url $PaymentWebhookUrl -Body "{}"
Assert-ControlledWebhookReject -Name "YooKassa webhook route" -Status $status

$blockedUrls = @(
    "$VkBaseUrl/admin/jobs",
    "$VkBaseUrl/metrics",
    "$VkBaseUrl/billing/payment-intents",
    "$VkBaseUrl/billing/payment-events/unprocessed"
)

if (-not $PaymentWebhookOnly) {
    $blockedUrls += @(
        "$AppBaseUrl/admin/jobs",
        "$AppBaseUrl/metrics",
        "$AppBaseUrl/billing/payment-intents",
        "$AppBaseUrl/billing/webhooks/yookassa"
    )
}

foreach ($blockedUrl in $blockedUrls) {
    $status = Invoke-SmokeRequest -Name $blockedUrl -Method "GET" -Url $blockedUrl
    Assert-Blocked -Name $blockedUrl -Status $status
}

Write-Host ""
Write-Host "Manual live smoke still required:"
Write-Host "- VK /start"
Write-Host "- VK ask NeuroHub"
Write-Host "- VK photo"
Write-Host "- VK video"
Write-Host "- Mini App authenticated /miniapp/balance"
Write-Host "- YooKassa payment.succeeded real checkout webhook"
Write-Host "- worker job completion"
Write-Host "- artifact delivery"
Write-Host "- admin endpoints closed"
Write-Host "- metrics are not public"
Write-Host ""
Write-Host "safe production smoke checks OK"
