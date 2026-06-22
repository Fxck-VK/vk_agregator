[CmdletBinding()]
param(
    [string]$EnvFile = "dev.env",
    [string]$ProjectName = "vk-ai-aggregator-dev",
    [int]$TimeoutSeconds = 5,
    [switch]$Strict
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

Add-Type -AssemblyName System.Net.Http

$script:RepoRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..")
$script:RuntimeDir = Join-Path $script:RepoRoot ".runtime\dev-stack"
$script:StatusFailures = 0
$script:StatusWarnings = 0
Set-Location $script:RepoRoot

function Write-StatusLine {
    param(
        [Parameter(Mandatory = $true)][string]$State,
        [Parameter(Mandatory = $true)][string]$Name,
        [Parameter(Mandatory = $true)][string]$Detail
    )

    Write-Host ("{0,-5} {1}: {2}" -f $State, $Name, $Detail)
    if ($State -eq "FAIL") {
        $script:StatusFailures++
    } elseif ($State -eq "WARN") {
        $script:StatusWarnings++
    }
}

function Read-EnvFile {
    param([Parameter(Mandatory = $true)][string]$Path)

    $values = @{}
    foreach ($line in Get-Content -LiteralPath $Path) {
        $trimmed = $line.Trim()
        if ($trimmed.Length -eq 0 -or $trimmed.StartsWith("#")) {
            continue
        }
        $idx = $trimmed.IndexOf("=")
        if ($idx -le 0) {
            continue
        }
        $key = $trimmed.Substring(0, $idx).Trim()
        $value = $trimmed.Substring($idx + 1).Trim().Trim('"').Trim("'")
        $values[$key] = $value
    }
    return $values
}

function Get-EnvValue {
    param(
        [Parameter(Mandatory = $true)][hashtable]$Values,
        [Parameter(Mandatory = $true)][string]$Name,
        [string]$Default = ""
    )

    if ($Values.ContainsKey($Name)) {
        return [string]$Values[$Name]
    }
    return $Default
}

function Get-BoolEnv {
    param(
        [Parameter(Mandatory = $true)][hashtable]$Values,
        [Parameter(Mandatory = $true)][string]$Name,
        [bool]$Default = $false
    )

    $raw = (Get-EnvValue -Values $Values -Name $Name).Trim().ToLowerInvariant()
    if ([string]::IsNullOrWhiteSpace($raw)) {
        return $Default
    }
    return $raw -in @("1", "true", "yes", "on", "enabled")
}

function Test-PlaceholderValue {
    param([string]$Value)

    if ([string]::IsNullOrWhiteSpace($Value)) {
        return $true
    }
    return $Value -match "CHANGE_ME|placeholder|example"
}

function Assert-DevStatusSecurity {
    param(
        [Parameter(Mandatory = $true)][hashtable]$Values,
        [Parameter(Mandatory = $true)]
        [AllowEmptyCollection()]
        [System.Collections.Generic.List[string]]$Problems
    )

    $productionGroupID = "239332376"
    $vkGroupID = Get-EnvValue -Values $Values -Name "VK_GROUP_ID"
    if ($vkGroupID -eq $productionGroupID) {
        [void]$Problems.Add("VK_GROUP_ID must not be the production group id")
    }

    $referralBase = Get-EnvValue -Values $Values -Name "VK_REFERRAL_LINK_BASE"
    if ($referralBase.Contains($productionGroupID)) {
        [void]$Problems.Add("VK_REFERRAL_LINK_BASE must not point to the production community")
    }

    $expectedTunnelName = Get-EnvValue -Values $Values -Name "DEV_EXPECTED_TUNNEL_NAME" -Default "neiirohub-vk-dev"
    $expectedTunnelHost = Get-EnvValue -Values $Values -Name "DEV_EXPECTED_TUNNEL_HOSTNAME" -Default "dev-vk.neiirohub.ru"
    if ((Get-EnvValue -Values $Values -Name "VK_BOT_TUNNEL_NAME") -ne $expectedTunnelName) {
        [void]$Problems.Add("VK_BOT_TUNNEL_NAME must be DEV tunnel $expectedTunnelName")
    }
    if ((Get-EnvValue -Values $Values -Name "VK_BOT_TUNNEL_HOSTNAME") -ne $expectedTunnelHost) {
        [void]$Problems.Add("VK_BOT_TUNNEL_HOSTNAME must be DEV hostname $expectedTunnelHost")
    }

    $allowRealAI = Get-BoolEnv -Values $Values -Name "DEV_ALLOW_REAL_AI_PROVIDERS" -Default $false
    if (-not $allowRealAI) {
        foreach ($name in @("PROVIDER", "PROVIDER_CHAIN", "IMAGE_PROVIDER", "VIDEO_PROVIDER")) {
            $value = (Get-EnvValue -Values $Values -Name $name).ToLowerInvariant()
            if ($value -ne "mock") {
                [void]$Problems.Add("$name must be mock unless DEV_ALLOW_REAL_AI_PROVIDERS=true")
            }
        }
        foreach ($name in @("RUNWAY_PROVIDER_ENABLED", "POYO_PROVIDER_ENABLED", "APIMART_PROVIDER_ENABLED")) {
            if (Get-BoolEnv -Values $Values -Name $name -Default $false) {
                [void]$Problems.Add("$name must be false unless DEV_ALLOW_REAL_AI_PROVIDERS=true")
            }
        }
        foreach ($name in @("FAL_API_KEY", "FAL_KEY", "RUNWAYML_API_SECRET", "POYO_API_KEY", "APIMART_API_KEY", "DEEPINFRA_API_KEY", "OPENAI_API_KEY")) {
            if (-not [string]::IsNullOrWhiteSpace((Get-EnvValue -Values $Values -Name $name))) {
                [void]$Problems.Add("$name must be empty unless DEV_ALLOW_REAL_AI_PROVIDERS=true")
            }
        }
    }

    $allowRealPayments = Get-BoolEnv -Values $Values -Name "DEV_ALLOW_REAL_PAYMENTS" -Default $false
    $paymentProvider = (Get-EnvValue -Values $Values -Name "PAYMENT_PROVIDER").ToLowerInvariant()
    if (-not $allowRealPayments) {
        if ($paymentProvider -ne "mock") {
            [void]$Problems.Add("PAYMENT_PROVIDER must be mock unless DEV_ALLOW_REAL_PAYMENTS=true")
        }
        foreach ($name in @("YOOKASSA_SHOP_ID", "YOOKASSA_SECRET_KEY")) {
            if (-not [string]::IsNullOrWhiteSpace((Get-EnvValue -Values $Values -Name $name))) {
                [void]$Problems.Add("$name must be empty unless DEV_ALLOW_REAL_PAYMENTS=true")
            }
        }
    } elseif ($paymentProvider -eq "yookassa") {
        if ((Get-EnvValue -Values $Values -Name "YOOKASSA_SECRET_KEY") -notmatch "^test_") {
            [void]$Problems.Add("YOOKASSA_SECRET_KEY must be a YooKassa test key in DEV")
        }
    }
}

function Assert-DevStatusEnv {
    param(
        [Parameter(Mandatory = $true)][hashtable]$Values,
        [Parameter(Mandatory = $true)][string]$ResolvedEnvFile
    )

    $problems = [System.Collections.Generic.List[string]]::new()
    $appEnv = (Get-EnvValue -Values $Values -Name "APP_ENV").ToLowerInvariant()
    if ($appEnv -ne "development" -and $appEnv -ne "dev") {
        [void]$problems.Add("APP_ENV must be development/dev for DEV status")
    }
    if ($ResolvedEnvFile -match "\.env\.prod") {
        [void]$problems.Add("EnvFile must not be a production template/file")
    }

    $expectedUrls = @{
        PUBLIC_VK_BASE_URL = "https://dev-vk.neiirohub.ru"
        PUBLIC_APP_BASE_URL = "https://dev-app.neiirohub.ru"
        PUBLIC_PAYMENT_WEBHOOK_URL = "https://dev.neiirohub.ru/billing/webhooks/yookassa"
    }
    foreach ($entry in $expectedUrls.GetEnumerator()) {
        $value = Get-EnvValue -Values $Values -Name $entry.Key
        if ($value -ne $entry.Value) {
            [void]$problems.Add("$($entry.Key) must be $($entry.Value)")
        }
    }

    $prodMarkers = @(
        "https://vk.neiirohub.ru",
        "https://app.neiirohub.ru",
        "https://neiirohub.ru/billing/webhooks/yookassa",
        "vk-ai-aggregator-prod"
    )
    foreach ($marker in $prodMarkers) {
        foreach ($value in $Values.Values) {
            if (([string]$value).Contains($marker)) {
                [void]$problems.Add("DEV status env contains production marker: $marker")
            }
        }
    }

    Assert-DevStatusSecurity -Values $Values -Problems $problems

    if ($problems.Count -gt 0) {
        foreach ($problem in $problems) {
            Write-StatusLine -State "FAIL" -Name "DEV env" -Detail $problem
        }
        exit 1
    }
}

function Set-ProcessEnvFromValues {
    param([Parameter(Mandatory = $true)][hashtable]$Values)

    $previous = @{}
    foreach ($key in $Values.Keys) {
        $previous[$key] = [Environment]::GetEnvironmentVariable($key, "Process")
        [Environment]::SetEnvironmentVariable($key, [string]$Values[$key], "Process")
    }
    return $previous
}

function Restore-ProcessEnv {
    param([Parameter(Mandatory = $true)][hashtable]$Previous)

    foreach ($key in $Previous.Keys) {
        [Environment]::SetEnvironmentVariable($key, $Previous[$key], "Process")
    }
}

function Test-TcpPort {
    param(
        [Parameter(Mandatory = $true)][int]$Port,
        [int]$TimeoutMs = 700
    )

    $client = [System.Net.Sockets.TcpClient]::new()
    try {
        $task = $client.ConnectAsync("127.0.0.1", $Port)
        if (-not $task.Wait($TimeoutMs)) {
            return $false
        }
        return $client.Connected
    } catch {
        return $false
    } finally {
        $client.Dispose()
    }
}

function Invoke-RawHttp {
    param(
        [Parameter(Mandatory = $true)][string]$Url,
        [string]$Method = "GET",
        [string]$HostHeader = "",
        [string]$Body = "",
        [int]$TimeoutSec = 5
    )

    $client = [System.Net.Http.HttpClient]::new()
    $client.Timeout = [TimeSpan]::FromSeconds($TimeoutSec)
    $request = $null
    $response = $null

    try {
        $request = [System.Net.Http.HttpRequestMessage]::new([System.Net.Http.HttpMethod]::new($Method), [Uri]$Url)
        if (-not [string]::IsNullOrWhiteSpace($HostHeader)) {
            $request.Headers.Host = $HostHeader
        }
        if ($Body.Length -gt 0) {
            $request.Content = [System.Net.Http.StringContent]::new($Body, [System.Text.Encoding]::UTF8, "application/json")
        }

        $response = $client.SendAsync($request).GetAwaiter().GetResult()
        $responseBody = $response.Content.ReadAsStringAsync().GetAwaiter().GetResult()
        return [pscustomobject]@{
            Status = [int]$response.StatusCode
            Body = $responseBody
            Error = ""
        }
    } catch {
        return [pscustomobject]@{
            Status = 0
            Body = ""
            Error = $_.Exception.Message
        }
    } finally {
        if ($null -ne $response) {
            $response.Dispose()
        }
        if ($null -ne $request) {
            $request.Dispose()
        }
        $client.Dispose()
    }
}

function Test-HttpStatus {
    param(
        [Parameter(Mandatory = $true)][string]$Name,
        [Parameter(Mandatory = $true)][string]$Url,
        [Parameter(Mandatory = $true)][int[]]$ExpectedStatuses,
        [string]$Method = "GET",
        [string]$HostHeader = "",
        [string]$Body = ""
    )

    $result = Invoke-RawHttp -Url $Url -Method $Method -HostHeader $HostHeader -Body $Body -TimeoutSec $TimeoutSeconds
    if ($ExpectedStatuses -contains $result.Status) {
        Write-StatusLine -State "OK" -Name $Name -Detail "HTTP $($result.Status)"
        return $true
    }

    if ($result.Status -eq 0) {
        Write-StatusLine -State "FAIL" -Name $Name -Detail $result.Error
    } else {
        Write-StatusLine -State "FAIL" -Name $Name -Detail "HTTP $($result.Status), expected $($ExpectedStatuses -join '/')"
    }
    return $false
}

function Show-DockerComposeStatus {
    if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
        Write-StatusLine -State "WARN" -Name "Docker" -Detail "docker CLI is not in PATH"
        return
    }

    Write-Host ""
    Write-Host "== Docker Compose services"
    $previousErrorActionPreference = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    try {
        $output = @(& docker @script:ComposeArgs ps 2>&1)
        $exitCode = $LASTEXITCODE
    } finally {
        $ErrorActionPreference = $previousErrorActionPreference
    }

    if ($exitCode -ne 0) {
        $detail = (($output | Select-Object -First 1 | ForEach-Object { $_.ToString() }) -join "").Trim()
        $detail = (($detail -split "`r?`n") | Select-Object -First 1).Trim()
        $detail = $detail -replace '^docker\.exe\s*:\s*', ''
        if ([string]::IsNullOrWhiteSpace($detail)) {
            $detail = "exit code $exitCode"
        }
        Write-StatusLine -State "FAIL" -Name "docker compose ps" -Detail $detail
        return
    }

    foreach ($line in $output) {
        Write-Host $line
    }
}

function Show-PortStatus {
    param([Parameter(Mandatory = $true)][int]$ReverseProxyPort)

    Write-Host ""
    Write-Host "== Local ports"
    $ports = @(
        [pscustomobject]@{ Name = "reverse-proxy"; Port = $ReverseProxyPort },
        [pscustomobject]@{ Name = "api"; Port = 8080 },
        [pscustomobject]@{ Name = "provider-webhook"; Port = 8082 },
        [pscustomobject]@{ Name = "worker"; Port = 9090 },
        [pscustomobject]@{ Name = "miniapp"; Port = 5173 }
    )

    foreach ($item in $ports) {
        if (Test-TcpPort -Port $item.Port) {
            Write-StatusLine -State "OK" -Name "$($item.Name) port" -Detail "127.0.0.1:$($item.Port) is open"
        } else {
            Write-StatusLine -State "FAIL" -Name "$($item.Name) port" -Detail "127.0.0.1:$($item.Port) is closed"
        }
    }
}

function Show-CloudflaredStatus {
    $pidFile = Join-Path $script:RuntimeDir "cloudflared.pid"
    if (-not (Test-Path -LiteralPath $pidFile)) {
        Write-StatusLine -State "WARN" -Name "Tunnel" -Detail "cloudflared pid file is missing"
        return $false
    }

    $raw = Get-Content -LiteralPath $pidFile -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($raw -notmatch '^\d+$') {
        Write-StatusLine -State "WARN" -Name "Tunnel" -Detail "cloudflared pid file is invalid"
        return $false
    }

    $proc = Get-Process -Id ([int]$raw) -ErrorAction SilentlyContinue
    if ($null -eq $proc) {
        Write-StatusLine -State "WARN" -Name "Tunnel" -Detail "cloudflared process is not running"
        return $false
    }

    Write-StatusLine -State "OK" -Name "Tunnel" -Detail "cloudflared pid=$($proc.Id)"
    return $true
}

function Test-VKCallbackConfirmation {
    param(
        [Parameter(Mandatory = $true)][hashtable]$Values,
        [Parameter(Mandatory = $true)][int]$ReverseProxyPort
    )

    $token = Get-EnvValue -Values $Values -Name "VK_CONFIRMATION_TOKEN"
    $groupID = Get-EnvValue -Values $Values -Name "VK_GROUP_ID"
    $secret = Get-EnvValue -Values $Values -Name "VK_SECRET"
    if ((Test-PlaceholderValue $token) -or (Test-PlaceholderValue $groupID) -or ($groupID -notmatch '^-?\d+$')) {
        Write-StatusLine -State "WARN" -Name "VK callback" -Detail "confirmation check skipped; fill DEV VK_CONFIRMATION_TOKEN and VK_GROUP_ID"
        return
    }

    $payload = @{
        type = "confirmation"
        group_id = [int64]$groupID
    }
    if (-not (Test-PlaceholderValue $secret)) {
        $payload.secret = $secret
    }
    $body = $payload | ConvertTo-Json -Compress
    $result = Invoke-RawHttp `
        -Url "http://127.0.0.1:$ReverseProxyPort/webhooks/vk" `
        -Method "POST" `
        -HostHeader "dev-vk.neiirohub.ru" `
        -Body $body `
        -TimeoutSec $TimeoutSeconds

    $ok = ($result.Status -eq 200 -and $result.Body.Trim() -eq $token)
    if ($ok) {
        Write-StatusLine -State "OK" -Name "VK callback" -Detail "confirmation body matched without printing token"
    } elseif ($result.Status -eq 0) {
        Write-StatusLine -State "FAIL" -Name "VK callback" -Detail $result.Error
    } else {
        Write-StatusLine -State "FAIL" -Name "VK callback" -Detail "HTTP $($result.Status) or confirmation body mismatch"
    }
}

$resolvedEnvFile = if ([System.IO.Path]::IsPathRooted($EnvFile)) {
    $EnvFile
} else {
    Join-Path $script:RepoRoot $EnvFile
}

if (-not (Test-Path -LiteralPath $resolvedEnvFile)) {
    Write-StatusLine -State "FAIL" -Name "Env" -Detail "not found: $resolvedEnvFile"
    exit 1
}

$envValues = Read-EnvFile -Path $resolvedEnvFile
Assert-DevStatusEnv -Values $envValues -ResolvedEnvFile $resolvedEnvFile
$reverseProxyPortRaw = Get-EnvValue -Values $envValues -Name "REVERSE_PROXY_HTTP_PORT" -Default "8088"
if ($reverseProxyPortRaw -notmatch '^\d+$') {
    Write-StatusLine -State "FAIL" -Name "REVERSE_PROXY_HTTP_PORT" -Detail "must be numeric"
    exit 1
}
$reverseProxyPort = [int]$reverseProxyPortRaw

$previousEnv = Set-ProcessEnvFromValues -Values $envValues
$previousAppEnvFile = [Environment]::GetEnvironmentVariable("APP_ENV_FILE", "Process")
[Environment]::SetEnvironmentVariable("APP_ENV_FILE", $resolvedEnvFile, "Process")

try {
    $script:ComposeArgs = @(
        "compose",
        "--project-name", $ProjectName,
        "--env-file", $resolvedEnvFile,
        "-f", "docker-compose.prod.yml"
    )

    Write-Host "== DEV stack status"
    Write-Host "Repo:        $script:RepoRoot"
    Write-Host "Env file:    $resolvedEnvFile"
    Write-Host "Project:     $ProjectName"
    Write-Host "APP_ENV:     $(Get-EnvValue -Values $envValues -Name 'APP_ENV' -Default '<missing>')"
    Write-Host "Proxy port:  $reverseProxyPort"

    Show-DockerComposeStatus
    Show-PortStatus -ReverseProxyPort $reverseProxyPort

    Write-Host ""
    Write-Host "== Local health"
    Test-HttpStatus -Name "API health" -Url "http://127.0.0.1:8080/health" -ExpectedStatuses @(200) | Out-Null
    Test-HttpStatus -Name "Worker health" -Url "http://127.0.0.1:9090/healthz" -ExpectedStatuses @(200) | Out-Null
    Test-HttpStatus -Name "Provider webhook health" -Url "http://127.0.0.1:8082/health" -ExpectedStatuses @(200) | Out-Null
    Test-HttpStatus -Name "Mini App frontend" -Url "http://127.0.0.1:5173/" -ExpectedStatuses @(200) | Out-Null
    Test-HttpStatus -Name "Reverse proxy health" -Url "http://127.0.0.1:$reverseProxyPort/proxy-health" -ExpectedStatuses @(200) | Out-Null

    Write-Host ""
    Write-Host "== DEV reverse proxy routes"
    Test-HttpStatus -Name "DEV VK health" -Url "http://127.0.0.1:$reverseProxyPort/health" -HostHeader "dev-vk.neiirohub.ru" -ExpectedStatuses @(200) | Out-Null
    Test-VKCallbackConfirmation -Values $envValues -ReverseProxyPort $reverseProxyPort
    Test-HttpStatus -Name "DEV Mini App" -Url "http://127.0.0.1:$reverseProxyPort/" -HostHeader "dev-app.neiirohub.ru" -ExpectedStatuses @(200) | Out-Null
    Test-HttpStatus -Name "DEV Mini App BFF" -Url "http://127.0.0.1:$reverseProxyPort/miniapp/balance" -HostHeader "dev-app.neiirohub.ru" -ExpectedStatuses @(400, 401, 403) | Out-Null
    Test-HttpStatus -Name "DEV YooKassa webhook" -Url "http://127.0.0.1:$reverseProxyPort/billing/webhooks/yookassa" -HostHeader "dev.neiirohub.ru" -Method "POST" -Body "{}" -ExpectedStatuses @(400, 401, 403) | Out-Null
    Test-HttpStatus -Name "DEV metrics blocked" -Url "http://127.0.0.1:$reverseProxyPort/metrics" -HostHeader "dev-app.neiirohub.ru" -ExpectedStatuses @(404) | Out-Null
    Test-HttpStatus -Name "DEV admin blocked" -Url "http://127.0.0.1:$reverseProxyPort/admin/jobs" -HostHeader "dev-vk.neiirohub.ru" -ExpectedStatuses @(404) | Out-Null

    Write-Host ""
    Write-Host "== DEV tunnel"
    $tunnelRunning = Show-CloudflaredStatus
    if ($tunnelRunning) {
        $publicVK = "$(Get-EnvValue -Values $envValues -Name 'PUBLIC_VK_BASE_URL')/health"
        $publicApp = "$(Get-EnvValue -Values $envValues -Name 'PUBLIC_APP_BASE_URL')/"
        $publicWebhook = Get-EnvValue -Values $envValues -Name "PUBLIC_PAYMENT_WEBHOOK_URL"
        Test-HttpStatus -Name "Public DEV VK health" -Url $publicVK -ExpectedStatuses @(200) | Out-Null
        Test-HttpStatus -Name "Public DEV Mini App" -Url $publicApp -ExpectedStatuses @(200) | Out-Null
        Test-HttpStatus -Name "Public DEV YooKassa webhook" -Url $publicWebhook -Method "POST" -Body "{}" -ExpectedStatuses @(400, 401, 403) | Out-Null
    } else {
        Write-StatusLine -State "WARN" -Name "Public DEV smoke" -Detail "skipped because tunnel is not running"
    }

    Write-Host ""
    if ($script:StatusFailures -eq 0) {
        Write-Host "DEV status summary: OK with $script:StatusWarnings warning(s)"
    } else {
        Write-Host "DEV status summary: $script:StatusFailures failure(s), $script:StatusWarnings warning(s)"
    }

    if ($Strict -and $script:StatusFailures -gt 0) {
        exit 1
    }
} finally {
    [Environment]::SetEnvironmentVariable("APP_ENV_FILE", $previousAppEnvFile, "Process")
    Restore-ProcessEnv -Previous $previousEnv
}
