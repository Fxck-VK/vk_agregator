param(
    [string]$VkBaseUrl = "https://vk.neiirohub.ru",
    [string]$AppBaseUrl = "https://app.neiirohub.ru",
    [string]$PaymentWebhookUrl = "https://vk.neiirohub.ru/billing/webhooks/yookassa",
    [int]$TimeoutSeconds = 10
)

$ErrorActionPreference = "Stop"

function Normalize-BaseUrl {
    param([Parameter(Mandatory = $true)][string]$Value)
    return $Value.TrimEnd("/")
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
        throw "$Name failed before HTTP status was returned: $($_.Exception.Message)"
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

function Assert-ControlledWebhookReject {
    param([string]$Name, [int]$Status)
    if ($Status -ge 200 -and $Status -lt 300) {
        throw "$Name accepted an invalid webhook body with $Status"
    }
    if ($Status -eq 404 -or $Status -eq 405 -or $Status -ge 500 -or $Status -eq 0) {
        throw "$Name did not reach provider-webhook cleanly, got $Status"
    }
    Write-Host "[OK] $Name rejects invalid webhook safely -> $Status"
}

$VkBaseUrl = Normalize-BaseUrl $VkBaseUrl
$AppBaseUrl = Normalize-BaseUrl $AppBaseUrl

Write-Host "Running safe production smoke checks"
Write-Host "VK base: $VkBaseUrl"
Write-Host "Mini App base: $AppBaseUrl"
Write-Host "Payment webhook: $PaymentWebhookUrl"

$status = Invoke-SmokeRequest -Name "VK health" -Method "GET" -Url "$VkBaseUrl/health"
Assert-2xx -Name "VK health" -Status $status

$status = Invoke-SmokeRequest -Name "Mini App open" -Method "GET" -Url "$AppBaseUrl/"
Assert-2xx -Name "Mini App open" -Status $status

$status = Invoke-SmokeRequest -Name "Mini App /miniapp/balance" -Method "GET" -Url "$AppBaseUrl/miniapp/balance"
Assert-AuthRequired -Name "Mini App /miniapp/balance" -Status $status

$status = Invoke-SmokeRequest -Name "YooKassa payment.succeeded webhook route" -Method "POST" -Url $PaymentWebhookUrl -Body "{}"
Assert-ControlledWebhookReject -Name "YooKassa payment.succeeded webhook route" -Status $status

foreach ($blockedUrl in @(
    "$VkBaseUrl/admin/jobs",
    "$VkBaseUrl/metrics",
    "$AppBaseUrl/admin/jobs",
    "$AppBaseUrl/metrics"
)) {
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
