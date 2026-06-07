Set-StrictMode -Version Latest

. (Join-Path $PSScriptRoot "_bot-common.ps1")

function Get-MiniAppRuntimeDir {
    param([Parameter(Mandatory = $true)][string]$Root)

    return (Join-Path $Root ".runtime\vk-miniapp")
}

function Get-MiniAppBinDir {
    param([Parameter(Mandatory = $true)][string]$Root)

    return (Join-Path $Root "bin\dev\miniapp")
}

function Import-MiniAppDotEnvFile {
    param([Parameter(Mandatory = $true)][string]$Path)

    if (-not (Test-Path -LiteralPath $Path)) {
        return
    }

    foreach ($line in Get-Content -LiteralPath $Path) {
        $trim = $line.Trim()
        if ($trim.Length -eq 0 -or $trim.StartsWith("#") -or -not $trim.Contains("=")) {
            continue
        }
        $idx = $trim.IndexOf("=")
        $key = $trim.Substring(0, $idx).Trim()
        $value = $trim.Substring($idx + 1).Trim()
        if ($key.StartsWith('$env:')) {
            $key = $key.Substring(5)
        }
        if ($value.Length -ge 2 -and (([int][char]$value[0] -eq 34 -and [int][char]$value[$value.Length - 1] -eq 34) -or ([int][char]$value[0] -eq 39 -and [int][char]$value[$value.Length - 1] -eq 39))) {
            $value = $value.Substring(1, $value.Length - 2)
        }
        if ($key -match "^[A-Za-z_][A-Za-z0-9_]*$") {
            Set-Item -Path ("Env:\" + $key) -Value $value
        }
    }
}

function Import-MiniAppDevEnv {
    param([Parameter(Mandatory = $true)][string]$Root)

    Import-MiniAppDotEnvFile -Path (Join-Path $Root ".env")
    $overlay = Join-Path $Root ".env.ps1"
    if (Test-Path -LiteralPath $overlay) {
        Import-MiniAppDotEnvFile -Path $overlay
    }
}

function Get-MiniAppEnvState {
    param([Parameter(Mandatory = $true)][string]$Name)

    if (-not (Test-Path ("Env:\" + $Name))) {
        return "missing"
    }
    $value = (Get-Item ("Env:\" + $Name)).Value
    if ([string]::IsNullOrWhiteSpace($value)) {
        return "empty"
    }
    return "present"
}

function Set-MiniAppDevDefaults {
    param(
        [Parameter(Mandatory = $true)][int]$ApiPort,
        [Parameter(Mandatory = $true)][int]$WorkerMetricsPort
    )

    $env:APP_ENV = "development"
    $env:HTTP_ADDR = ":$ApiPort"
    $env:PROVIDER = "deepinfra"
    $env:PROVIDER_CHAIN = "deepinfra"
    $env:VK_DELIVERY_MODE = "mock"
    $env:MODERATION_PROVIDER = "keyword"
    $env:ARTIFACT_SCANNER = "none"
    $env:WORKER_METRICS_ADDR = ":$WorkerMetricsPort"
}

function New-MiniAppLaunchParams {
    param([Parameter(Mandatory = $true)][int]$UserID)

    $params = [ordered]@{
        vk_user_id = [string]$UserID
        vk_ts = [string][DateTimeOffset]::UtcNow.ToUnixTimeSeconds()
        vk_platform = "desktop_web"
    }
    if (-not [string]::IsNullOrWhiteSpace($env:VK_APP_ID)) {
        $params.vk_app_id = $env:VK_APP_ID
    }

    if (-not [string]::IsNullOrWhiteSpace($env:VK_APP_SECRET)) {
        $sorted = $params.Keys | Sort-Object
        $toSign = ($sorted | ForEach-Object {
            $_ + "=" + [uri]::EscapeDataString($params[$_])
        }) -join "&"
        $keyBytes = [Text.Encoding]::UTF8.GetBytes($env:VK_APP_SECRET)
        $dataBytes = [Text.Encoding]::UTF8.GetBytes($toSign)
        $hmac = [System.Security.Cryptography.HMACSHA256]::new($keyBytes)
        $hash = $hmac.ComputeHash($dataBytes)
        $sign = [Convert]::ToBase64String($hash).TrimEnd("=").Replace("+", "-").Replace("/", "_")
        $params.sign = $sign
    }

    return (($params.GetEnumerator() | ForEach-Object {
        $_.Key + "=" + [uri]::EscapeDataString([string]$_.Value)
    }) -join "&")
}

function Stop-ViteDevServer {
    param([Parameter(Mandatory = $true)][int]$Port)

    $needle = "--port $Port"
    Get-CimInstance Win32_Process -ErrorAction SilentlyContinue |
        Where-Object {
            $_.Name -eq "node.exe" -and
            $_.CommandLine -and
            $_.CommandLine.Contains($needle) -and
            ($_.CommandLine.Contains("vite") -or $_.CommandLine.Contains("npm-cli.js"))
        } |
        ForEach-Object {
            try {
                Stop-Process -Id $_.ProcessId -Force
            } catch {}
        }
}

function Stop-LocalhostRunTunnel {
    Stop-Process -Name ngrok -Force -ErrorAction SilentlyContinue
    Get-CimInstance Win32_Process -ErrorAction SilentlyContinue |
        Where-Object {
            $_.Name -eq "ssh.exe" -and
            $_.CommandLine -and
            $_.CommandLine.Contains("localhost.run") -and
            $_.CommandLine.Contains("127.0.0.1")
        } |
        ForEach-Object {
            try {
                Stop-Process -Id $_.ProcessId -Force
            } catch {}
        }
}

function Get-LocalhostRunUrlFromLogs {
    param([Parameter(Mandatory = $true)][string]$RuntimeDir)

    $files = @(
        (Join-Path $RuntimeDir "localhostrun-live.log"),
        (Join-Path $RuntimeDir "localhostrun-live.err")
    )
    foreach ($file in $files) {
        if (-not (Test-Path -LiteralPath $file)) {
            continue
        }
        $matches = Select-String -Path $file -Pattern "https://[a-z0-9]+\.lhr\.life" -AllMatches -ErrorAction SilentlyContinue
        $urls = @()
        foreach ($match in $matches) {
            foreach ($m in $match.Matches) {
                $urls += $m.Value
            }
        }
        if ($urls.Count -gt 0) {
            return $urls[-1]
        }
    }
    return ""
}

function Wait-LocalhostRunUrl {
    param(
        [Parameter(Mandatory = $true)][string]$RuntimeDir,
        [int]$TimeoutSeconds = 90
    )

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    do {
        $url = Get-LocalhostRunUrlFromLogs -RuntimeDir $RuntimeDir
        if (-not [string]::IsNullOrWhiteSpace($url)) {
            return $url
        }
        Start-Sleep -Milliseconds 500
    } while ((Get-Date) -lt $deadline)

    throw "Timed out waiting for localhost.run tunnel URL (*.lhr.life)."
}

function Start-LocalhostRunTunnel {
    param(
        [Parameter(Mandatory = $true)][string]$RuntimeDir,
        [Parameter(Mandatory = $true)][int]$MiniAppPort
    )

    $cmd = Get-Command ssh -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($null -eq $cmd) {
        throw "ssh is not installed or not available in PATH."
    }

    $stdout = Join-Path $RuntimeDir "localhostrun-live.log"
    $stderr = Join-Path $RuntimeDir "localhostrun-live.err"
    return Start-Process -FilePath $cmd.Source `
        -ArgumentList @(
            "-o", "StrictHostKeyChecking=no",
            "-o", "ServerAliveInterval=30",
            "-R", "80:127.0.0.1:$MiniAppPort",
            "nokey@localhost.run"
        ) `
        -WindowStyle Hidden `
        -RedirectStandardOutput $stdout `
        -RedirectStandardError $stderr `
        -PassThru
}

function Start-ViteDevServer {
    param(
        [Parameter(Mandatory = $true)][string]$Root,
        [Parameter(Mandatory = $true)][string]$RuntimeDir,
        [Parameter(Mandatory = $true)][int]$MiniAppPort,
        [Parameter(Mandatory = $true)][string]$LaunchParams
    )

    $stdout = Join-Path $RuntimeDir "vite-live.log"
    $stderr = Join-Path $RuntimeDir "vite-live.err"
    $miniappDir = Join-Path $Root "web\miniapp"
    $env:VITE_DEV_LAUNCH_PARAMS = $LaunchParams
    try {
        return Start-Process npm.cmd `
            -ArgumentList @("run", "dev", "--", "--host", "127.0.0.1", "--port", [string]$MiniAppPort) `
            -WorkingDirectory $miniappDir `
            -WindowStyle Hidden `
            -RedirectStandardOutput $stdout `
            -RedirectStandardError $stderr `
            -PassThru
    } finally {
        Remove-Item Env:\VITE_DEV_LAUNCH_PARAMS -ErrorAction SilentlyContinue
    }
}

function Stop-MiniAppCommandLineProcesses {
    param(
        [Parameter(Mandatory = $true)][string]$Root,
        [Parameter(Mandatory = $true)][int]$MiniAppPort
    )

    $escapedRoot = [regex]::Escape($Root)
    $escapedRuntime = [regex]::Escape((Get-MiniAppRuntimeDir -Root $Root))
    $escapedMiniapp = [regex]::Escape((Join-Path $Root "web\miniapp"))
    $processes = Get-CimInstance Win32_Process | Where-Object {
        $cmd = [string]$_.CommandLine
        if ($cmd -eq "") {
            return $false
        }
        ($cmd -match $escapedRoot -and (
            $cmd -match 'bin\\dev\\miniapp\\api\.exe' -or
            $cmd -match 'bin\\dev\\miniapp\\worker\.exe'
        )) -or
        ($cmd -match $escapedMiniapp -and $cmd -match 'vite') -or
        ($cmd -match 'ssh\.exe' -and $cmd -match 'localhost\.run') -or
        ($cmd -match $escapedRuntime)
    }

    foreach ($proc in $processes) {
        try {
            Stop-Process -Id $proc.ProcessId -Force
        } catch {}
    }

    Stop-ViteDevServer -Port $MiniAppPort
}

function Assert-MiniAppDevRequirements {
    param([switch]$RequireTunnel)

    $go = Get-Command go -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($null -eq $go) {
        throw "go is not installed or not available in PATH."
    }
    $npm = Get-Command npm.cmd -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($null -eq $npm) {
        throw "npm.cmd is not installed or not available in PATH."
    }
    if ($RequireTunnel) {
        $ssh = Get-Command ssh -ErrorAction SilentlyContinue | Select-Object -First 1
        if ($null -eq $ssh) {
            throw "ssh is not installed or not available in PATH."
        }
    }
    if ((Get-MiniAppEnvState -Name "DEEPINFRA_API_KEY") -ne "present") {
        throw "DEEPINFRA_API_KEY is required for real Mini App chat responses."
    }
}
