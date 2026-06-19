[CmdletBinding()]
param(
    [string]$BaseUrl = "http://127.0.0.1:8088",
    [int]$TimeoutSec = 10
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

Add-Type -AssemblyName System.Net.Http

function Invoke-DevProxyRequest {
    param(
        [Parameter(Mandatory = $true)][string]$Name,
        [Parameter(Mandatory = $true)][string]$HostName,
        [Parameter(Mandatory = $true)][string]$Path,
        [string]$Method = "GET",
        [string]$Body = "",
        [int[]]$ExpectedStatuses,
        [int[]]$ForbiddenStatuses = @(404, 500, 502, 503, 504)
    )

    if ($ExpectedStatuses.Count -eq 0) {
        throw "$Name has no expected statuses configured"
    }

    $uri = [Uri]"$BaseUrl$Path"
    $client = [System.Net.Http.HttpClient]::new()
    $client.Timeout = [TimeSpan]::FromSeconds($TimeoutSec)
    $request = $null
    $response = $null

    try {
        $request = [System.Net.Http.HttpRequestMessage]::new([System.Net.Http.HttpMethod]::new($Method), $uri)
        $request.Headers.Host = $HostName
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

    if ($ExpectedStatuses -contains $status) {
        Write-Host "OK  $Name -> HTTP $status"
        return
    }

    if ($ForbiddenStatuses -contains $status) {
        throw "$Name returned forbidden HTTP $status"
    }

    throw "$Name returned HTTP $status, expected one of: $($ExpectedStatuses -join ', ')"
}

$checks = @(
    @{
        Name = "DEV VK health"
        HostName = "dev-vk.neiirohub.ru"
        Path = "/health"
        ExpectedStatuses = @(200)
    },
    @{
        Name = "DEV Mini App frontend"
        HostName = "dev-app.neiirohub.ru"
        Path = "/"
        ExpectedStatuses = @(200)
    },
    @{
        Name = "DEV Mini App BFF route"
        HostName = "dev-app.neiirohub.ru"
        Path = "/miniapp/balance"
        ExpectedStatuses = @(400, 401, 403)
        ForbiddenStatuses = @(404, 500, 502, 503, 504)
    },
    @{
        Name = "DEV YooKassa webhook route"
        HostName = "dev.neiirohub.ru"
        Path = "/billing/webhooks/yookassa"
        Method = "POST"
        Body = "{}"
        ExpectedStatuses = @(400, 401, 403)
        ForbiddenStatuses = @(404, 500, 502, 503, 504)
    },
    @{
        Name = "DEV Mini App metrics blocked"
        HostName = "dev-app.neiirohub.ru"
        Path = "/metrics"
        ExpectedStatuses = @(404)
        ForbiddenStatuses = @(200, 500, 502, 503, 504)
    },
    @{
        Name = "DEV VK admin blocked"
        HostName = "dev-vk.neiirohub.ru"
        Path = "/admin/jobs"
        ExpectedStatuses = @(404)
        ForbiddenStatuses = @(200, 500, 502, 503, 504)
    }
)

foreach ($check in $checks) {
    Invoke-DevProxyRequest @check
}

Write-Host "DEV reverse proxy smoke OK"
