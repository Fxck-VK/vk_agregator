[CmdletBinding()]
param(
    [string]$VkBaseUrl = "https://dev-vk.neiirohub.ru",
    [string]$AppBaseUrl = "https://dev-app.neiirohub.ru",
    [string]$DevBaseUrl = "https://dev.neiirohub.ru",
    [int]$TimeoutSeconds = 10
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

Add-Type -AssemblyName System.Net.Http

function Assert-DevUrl {
    param(
        [Parameter(Mandatory = $true)][string]$Name,
        [Parameter(Mandatory = $true)][string]$Url,
        [Parameter(Mandatory = $true)][string]$ExpectedHost
    )

    $uri = [Uri]$Url
    if ($uri.Scheme -ne "https") {
        throw "$Name must use HTTPS: $Url"
    }
    if ($uri.Host -ne $ExpectedHost) {
        throw "$Name must use $ExpectedHost, got $($uri.Host)"
    }
}

function Invoke-HttpCheck {
    param(
        [Parameter(Mandatory = $true)][string]$Name,
        [Parameter(Mandatory = $true)][string]$Url,
        [Parameter(Mandatory = $true)][int[]]$ExpectedStatuses,
        [string]$Method = "GET",
        [string]$Body = "",
        [int[]]$ForbiddenStatuses = @()
    )

    $client = [System.Net.Http.HttpClient]::new()
    $client.Timeout = [TimeSpan]::FromSeconds($TimeoutSeconds)
    $request = $null
    $response = $null

    try {
        $request = [System.Net.Http.HttpRequestMessage]::new([System.Net.Http.HttpMethod]::new($Method), [Uri]$Url)
        if ($Body.Length -gt 0) {
            $request.Content = [System.Net.Http.StringContent]::new($Body, [System.Text.Encoding]::UTF8, "application/json")
        }

        $response = $client.SendAsync($request).GetAwaiter().GetResult()
        $status = [int]$response.StatusCode
    } catch {
        throw "$Name failed before HTTP response: $($_.Exception.Message)"
    } finally {
        if ($null -ne $response) {
            $response.Dispose()
        }
        if ($null -ne $request) {
            $request.Dispose()
        }
        $client.Dispose()
    }

    if ($ForbiddenStatuses -contains $status) {
        throw "$Name returned forbidden HTTP $status"
    }
    if ($ExpectedStatuses -contains $status) {
        Write-Host "OK  $Name -> HTTP $status"
        return
    }

    throw "$Name returned HTTP $status, expected one of: $($ExpectedStatuses -join ', ')"
}

$VkBaseUrl = $VkBaseUrl.TrimEnd("/")
$AppBaseUrl = $AppBaseUrl.TrimEnd("/")
$DevBaseUrl = $DevBaseUrl.TrimEnd("/")

Assert-DevUrl -Name "VK DEV base URL" -Url $VkBaseUrl -ExpectedHost "dev-vk.neiirohub.ru"
Assert-DevUrl -Name "Mini App DEV base URL" -Url $AppBaseUrl -ExpectedHost "dev-app.neiirohub.ru"
Assert-DevUrl -Name "Shared DEV base URL" -Url $DevBaseUrl -ExpectedHost "dev.neiirohub.ru"

Write-Host "== DEV public smoke"

Invoke-HttpCheck `
    -Name "DEV VK health" `
    -Url "$VkBaseUrl/health" `
    -ExpectedStatuses @(200)

Invoke-HttpCheck `
    -Name "DEV VK callback route" `
    -Url "$VkBaseUrl/webhooks/vk" `
    -ExpectedStatuses @(400, 401, 403, 405) `
    -ForbiddenStatuses @(404, 500, 502, 503, 504)

Invoke-HttpCheck `
    -Name "DEV Mini App frontend" `
    -Url "$AppBaseUrl/" `
    -ExpectedStatuses @(200)

Invoke-HttpCheck `
    -Name "DEV Mini App BFF balance" `
    -Url "$AppBaseUrl/miniapp/balance" `
    -ExpectedStatuses @(400, 401, 403) `
    -ForbiddenStatuses @(404, 500, 502, 503, 504)

Invoke-HttpCheck `
    -Name "DEV YooKassa webhook route" `
    -Url "$DevBaseUrl/billing/webhooks/yookassa" `
    -Method "POST" `
    -Body "{}" `
    -ExpectedStatuses @(400, 401, 403) `
    -ForbiddenStatuses @(404, 500, 502, 503, 504)

$blockedPublicRoutes = @(
    [pscustomobject]@{ Name = "DEV VK admin blocked"; Url = "$VkBaseUrl/admin/jobs" },
    [pscustomobject]@{ Name = "DEV App admin blocked"; Url = "$AppBaseUrl/admin/jobs" },
    [pscustomobject]@{ Name = "DEV shared admin blocked"; Url = "$DevBaseUrl/admin/jobs" },
    [pscustomobject]@{ Name = "DEV VK metrics blocked"; Url = "$VkBaseUrl/metrics" },
    [pscustomobject]@{ Name = "DEV App metrics blocked"; Url = "$AppBaseUrl/metrics" },
    [pscustomobject]@{ Name = "DEV shared metrics blocked"; Url = "$DevBaseUrl/metrics" }
)

foreach ($route in $blockedPublicRoutes) {
    Invoke-HttpCheck `
        -Name $route.Name `
        -Url $route.Url `
        -ExpectedStatuses @(404, 401, 403) `
        -ForbiddenStatuses @(200, 500, 502, 503, 504)
}

Write-Host "DEV public smoke OK"
