param(
    [string]$ApiBaseUrl = "http://127.0.0.1:8080",
    [string]$ProviderWebhookBaseUrl = "http://127.0.0.1:8082",
    [string]$MiniAppBaseUrl = "http://127.0.0.1:5173",
    [string]$MiniAppLaunchParams = $env:VITE_DEV_LAUNCH_PARAMS,
    [switch]$AllowMockEstimate,
    [switch]$Strict
)

$ErrorActionPreference = "Stop"

function Invoke-SyntheticHttp {
    param(
        [string]$Name,
        [string]$Method = "GET",
        [string]$Url,
        [hashtable]$Headers = @{},
        [string]$Body = "",
        [int[]]$ExpectedStatus = @(200)
    )

    $started = Get-Date
    try {
        $args = @{
            Method = $Method
            Uri = $Url
            TimeoutSec = 5
            UseBasicParsing = $true
            Headers = $Headers
        }
        if ($Body -ne "") {
            $args.Body = $Body
            $args.ContentType = "application/json"
        }
        $response = Invoke-WebRequest @args
        $status = [int]$response.StatusCode
    } catch {
        if ($_.Exception.Response -and $_.Exception.Response.StatusCode) {
            $status = [int]$_.Exception.Response.StatusCode
        } else {
            $status = 0
        }
    }

    $durationMs = [int]((Get-Date) - $started).TotalMilliseconds
    $ok = $ExpectedStatus -contains $status
    [pscustomobject]@{
        name = $Name
        ok = $ok
        status = $status
        duration_ms = $durationMs
    }
}

$results = @()
$results += Invoke-SyntheticHttp -Name "api_healthz" -Url "$ApiBaseUrl/healthz" -ExpectedStatus @(200, 503)
$results += Invoke-SyntheticHttp -Name "provider_webhook_readyz" -Url "$ProviderWebhookBaseUrl/readyz" -ExpectedStatus @(200, 503)
$results += Invoke-SyntheticHttp -Name "miniapp_root" -Url "$MiniAppBaseUrl/" -ExpectedStatus @(200)

if ($AllowMockEstimate) {
    if ([string]::IsNullOrWhiteSpace($MiniAppLaunchParams)) {
        $results += [pscustomobject]@{
            name = "miniapp_estimate_mock"
            ok = $false
            status = 0
            duration_ms = 0
            skipped_reason = "missing_launch_params"
        }
    } else {
        $headers = @{ "X-Launch-Params" = $MiniAppLaunchParams }
        $body = '{"operation":"text_generate","prompt":"synthetic health check"}'
        $results += Invoke-SyntheticHttp -Name "miniapp_estimate_mock" -Method "POST" -Url "$ApiBaseUrl/miniapp/estimate" -Headers $headers -Body $body -ExpectedStatus @(200, 400, 401)
    }
}

$results | ConvertTo-Json -Depth 4

if ($Strict -and ($results | Where-Object { -not $_.ok })) {
    exit 1
}
