Set-StrictMode -Version Latest

. (Join-Path $PSScriptRoot "_bot-common.ps1")

function Get-PaymentsRuntimeDir {
    param([Parameter(Mandatory = $true)][string]$Root)

    return (Join-Path $Root ".runtime\payments")
}

function Get-PaymentsBinDir {
    param([Parameter(Mandatory = $true)][string]$Root)

    return (Join-Path $Root "bin\dev")
}

function Get-PaymentWebhookHealthUrl {
    param([Parameter(Mandatory = $true)][string]$Root)

    $addr = Get-ConfigValue -Root $Root -Name "PAYMENT_WEBHOOK_ADDR" -Default ":8082"
    return Convert-ListenAddrToLocalUrl -Addr $addr -Path "/health"
}

function Get-PaymentWebhookPublicUrl {
    param(
        [Parameter(Mandatory = $true)][string]$Root,
        [AllowEmptyString()][string]$Override = ""
    )

    if (-not [string]::IsNullOrWhiteSpace($Override)) {
        return $Override
    }

    return Get-ConfigValue -Root $Root -Name "PAYMENT_WEBHOOK_PUBLIC_URL" -Default "https://vk.neiirohub.ru/billing/webhooks/yookassa"
}

function Wait-PaymentsPostgres {
    param(
        [Parameter(Mandatory = $true)][string]$Root,
        [int]$TimeoutSeconds = 120
    )

    Push-Location $Root
    try {
        $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
        do {
            try {
                docker compose exec -T postgres pg_isready -U vk_ai_aggregator -d vk_ai_aggregator *> $null
                return
            } catch {
                Start-Sleep -Seconds 2
            }
        } while ((Get-Date) -lt $deadline)
    } finally {
        Pop-Location
    }

    throw "Postgres did not become healthy in time."
}

function Test-PaymentWebhookPublicRoute {
    param([Parameter(Mandatory = $true)][string]$Url)

    try {
        $response = Invoke-WebRequest -Uri $Url -Method POST -ContentType "application/json" -Body "{}" -UseBasicParsing -TimeoutSec 10
        return ([int]$response.StatusCode -eq 400)
    } catch {
        $status = $null
        if ($_.Exception.Response -and $_.Exception.Response.StatusCode) {
            $status = [int]$_.Exception.Response.StatusCode
        }
        return ($status -eq 400)
    }
}

function Wait-PaymentWebhookPublicRoute {
    param(
        [Parameter(Mandatory = $true)][string]$Url,
        [int]$TimeoutSeconds = 60
    )

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    do {
        if (Test-PaymentWebhookPublicRoute -Url $Url) {
            return $true
        }
        Start-Sleep -Seconds 2
    } while ((Get-Date) -lt $deadline)

    throw "Timed out waiting for public payment webhook route: $Url"
}

function Get-PaymentWebhookPublicRouteStatus {
    param([Parameter(Mandatory = $true)][string]$Url)

    try {
        $response = Invoke-WebRequest -Uri $Url -Method POST -ContentType "application/json" -Body "{}" -UseBasicParsing -TimeoutSec 10
        return "reachable status=$($response.StatusCode)"
    } catch {
        if ($_.Exception.Response -and $_.Exception.Response.StatusCode) {
            $status = [int]$_.Exception.Response.StatusCode
            if ($status -eq 400) {
                return "reachable status=400 invalid webhook"
            }
            return "failed status=$status"
        }
        return "failed $($_.Exception.Message)"
    }
}

function Stop-PaymentCommandLineProcesses {
    param([Parameter(Mandatory = $true)][string]$Root)

    $escapedRoot = [regex]::Escape($Root)
    $processes = Get-CimInstance Win32_Process | Where-Object {
        $cmd = [string]$_.CommandLine
        if ($cmd -eq "") {
            return $false
        }
        $cmd -match $escapedRoot -and (
            $cmd -match 'cmd/provider-webhook' -or
            $cmd -match 'cmd\\provider-webhook' -or
            $cmd -match 'bin\\dev\\provider-webhook\.exe'
        )
    }

    foreach ($proc in $processes) {
        try {
            Stop-Process -Id $proc.ProcessId -Force
        } catch {}
    }
}
