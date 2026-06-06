Set-StrictMode -Version Latest

function Get-RepoRoot {
    return (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
}

function Get-BotRuntimeDir {
    param([Parameter(Mandatory = $true)][string]$Root)

    return (Join-Path $Root ".runtime\vk-bot")
}

function Get-BotBinDir {
    param([Parameter(Mandatory = $true)][string]$Root)

    return (Join-Path $Root "bin\dev")
}

function Ensure-Directory {
    param([Parameter(Mandatory = $true)][string]$Path)

    if (-not (Test-Path -LiteralPath $Path)) {
        New-Item -ItemType Directory -Path $Path | Out-Null
    }
}

function Get-DotEnvMap {
    param([Parameter(Mandatory = $true)][string]$Root)

    $map = @{}
    $envPath = Join-Path $Root ".env"
    if (-not (Test-Path -LiteralPath $envPath)) {
        return $map
    }

    foreach ($line in Get-Content -LiteralPath $envPath) {
        if ($line -match '^\s*$' -or $line -match '^\s*#') {
            continue
        }
        if ($line -match '^\s*([^=]+?)\s*=\s*(.*)\s*$') {
            $name = $matches[1].Trim()
            $value = $matches[2].Trim()
            if ($value.Length -ge 2 -and (([int][char]$value[0] -eq 34 -and [int][char]$value[$value.Length - 1] -eq 34) -or ([int][char]$value[0] -eq 39 -and [int][char]$value[$value.Length - 1] -eq 39))) {
                $value = $value.Substring(1, $value.Length - 2)
            }
            $map[$name] = $value
        }
    }
    return $map
}

function Get-ConfigValue {
    param(
        [Parameter(Mandatory = $true)][string]$Root,
        [Parameter(Mandatory = $true)][string]$Name,
        [Parameter(Mandatory = $true)][string]$Default
    )

    $processValue = [Environment]::GetEnvironmentVariable($Name, "Process")
    if (-not [string]::IsNullOrWhiteSpace($processValue)) {
        return $processValue
    }

    $dotenv = Get-DotEnvMap -Root $Root
    if ($dotenv.ContainsKey($Name) -and -not [string]::IsNullOrWhiteSpace([string]$dotenv[$Name])) {
        return [string]$dotenv[$Name]
    }

    return $Default
}

function Get-PortFromListenAddr {
    param([Parameter(Mandatory = $true)][string]$Addr)

    $trimmed = $Addr.Trim()
    if ($trimmed -eq "") {
        return $null
    }
    if ($trimmed -match ':(\d+)$') {
        return [int]$matches[1]
    }
    if ($trimmed -match '^\d+$') {
        return [int]$trimmed
    }
    return $null
}

function Convert-ListenAddrToLocalUrl {
    param(
        [Parameter(Mandatory = $true)][string]$Addr,
        [Parameter(Mandatory = $true)][AllowEmptyString()][string]$Path
    )

    $port = Get-PortFromListenAddr -Addr $Addr
    if ($null -eq $port) {
        throw "Cannot determine port from listen address '$Addr'."
    }
    return "http://127.0.0.1:$port$Path"
}

function Wait-Http {
    param(
        [Parameter(Mandatory = $true)][string]$Url,
        [int]$TimeoutSeconds = 60
    )

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    do {
        try {
            $response = Invoke-WebRequest -Uri $Url -UseBasicParsing -TimeoutSec 5
            if ($response.StatusCode -ge 200 -and $response.StatusCode -lt 300) {
                return $true
            }
        } catch {
            Start-Sleep -Seconds 1
        }
    } while ((Get-Date) -lt $deadline)

    throw "Timed out waiting for $Url"
}

function Wait-TcpPort {
    param(
        [Parameter(Mandatory = $true)][int]$Port,
        [int]$TimeoutSeconds = 60
    )

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    do {
        try {
            $client = New-Object Net.Sockets.TcpClient
            $iar = $client.BeginConnect("127.0.0.1", $Port, $null, $null)
            if ($iar.AsyncWaitHandle.WaitOne(1000, $false)) {
                $client.EndConnect($iar)
                $client.Close()
                return $true
            }
            $client.Close()
        } catch {
            Start-Sleep -Seconds 1
        }
    } while ((Get-Date) -lt $deadline)

    throw "Timed out waiting for TCP port $Port"
}

function Ensure-DockerRunning {
    param([int]$TimeoutSeconds = 120)

    try {
        docker info *> $null
        return
    } catch {
        $dockerDesktop = "C:\Program Files\Docker\Docker\Docker Desktop.exe"
        if (Test-Path -LiteralPath $dockerDesktop) {
            Start-Process -FilePath $dockerDesktop -WindowStyle Hidden | Out-Null
        }
    }

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    do {
        try {
            docker info *> $null
            return
        } catch {
            Start-Sleep -Seconds 3
        }
    } while ((Get-Date) -lt $deadline)

    throw "Docker Engine is not running."
}

function Wait-BotDockerDependencies {
    param(
        [Parameter(Mandatory = $true)][string]$Root,
        [int]$TimeoutSeconds = 120
    )

    Push-Location $Root
    try {
        $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
        do {
            $postgresOk = $false
            $redisOk = $false
            try {
                docker compose exec -T postgres pg_isready -U vk_ai_aggregator -d vk_ai_aggregator *> $null
                $postgresOk = $true
            } catch {}
            try {
                $redisPing = docker compose exec -T redis redis-cli ping 2>$null
                $redisOk = (($redisPing | Select-Object -First 1) -eq "PONG")
            } catch {}
            if ($postgresOk -and $redisOk) {
                Wait-TcpPort -Port 9000 -TimeoutSeconds 30 | Out-Null
                return
            }
            Start-Sleep -Seconds 2
        } while ((Get-Date) -lt $deadline)
    } finally {
        Pop-Location
    }

    throw "Docker dependencies did not become healthy in time."
}

function Stop-PidFileProcess {
    param([Parameter(Mandatory = $true)][string]$PidFile)

    if (-not (Test-Path -LiteralPath $PidFile)) {
        return
    }

    $raw = (Get-Content -LiteralPath $PidFile -ErrorAction SilentlyContinue | Select-Object -First 1)
    if ($raw -match '^\d+$') {
        $pidValue = [int]$raw
        try {
            $proc = Get-Process -Id $pidValue -ErrorAction Stop
            Stop-Process -Id $proc.Id -Force
        } catch {}
    }
    Remove-Item -LiteralPath $PidFile -Force -ErrorAction SilentlyContinue
}

function Stop-ListenerOnPort {
    param([int]$Port)

    if ($Port -le 0) {
        return
    }
    try {
        $listeners = Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue
        foreach ($listener in $listeners) {
            try {
                Stop-Process -Id $listener.OwningProcess -Force
            } catch {}
        }
    } catch {}
}

function Stop-BotCommandLineProcesses {
    param([Parameter(Mandatory = $true)][string]$Root)

    $escapedRoot = [regex]::Escape($Root)
    $escapedRuntime = [regex]::Escape((Get-BotRuntimeDir -Root $Root))
    $processes = Get-CimInstance Win32_Process | Where-Object {
        $cmd = [string]$_.CommandLine
        if ($cmd -eq "") {
            return $false
        }
        ($cmd -match $escapedRoot -and (
            $cmd -match 'cmd/api' -or
            $cmd -match 'cmd\\api' -or
            $cmd -match 'cmd/worker' -or
            $cmd -match 'cmd\\worker' -or
            $cmd -match 'bin\\dev\\api\.exe' -or
            $cmd -match 'bin\\dev\\worker\.exe'
        )) -or
        ($cmd -match 'cloudflared' -and (
            $cmd -match '--url http://127\.0\.0\.1:' -or
            $cmd -match $escapedRuntime
        ))
    }

    foreach ($proc in $processes) {
        try {
            Stop-Process -Id $proc.ProcessId -Force
        } catch {}
    }
}

function Get-NamedTunnelConfigPath {
    param([Parameter(Mandatory = $true)][string]$RuntimeDir)

    return (Join-Path $RuntimeDir "cloudflared\config.yml")
}

function Get-NamedTunnelCallbackUrl {
    param([Parameter(Mandatory = $true)][string]$Hostname)

    return "https://$Hostname/webhooks/vk"
}

function Get-TunnelUrlFromLogs {
    param([Parameter(Mandatory = $true)][string]$RuntimeDir)

    $files = @(
        (Join-Path $RuntimeDir "cloudflared-live.log"),
        (Join-Path $RuntimeDir "cloudflared-live.err")
    )
    foreach ($file in $files) {
        if (-not (Test-Path -LiteralPath $file)) {
            continue
        }
        $matches = Select-String -Path $file -Pattern 'https://[-a-z0-9]+\.trycloudflare\.com' -AllMatches -ErrorAction SilentlyContinue
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

function Wait-TunnelUrl {
    param(
        [Parameter(Mandatory = $true)][string]$RuntimeDir,
        [int]$TimeoutSeconds = 90
    )

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    do {
        $url = Get-TunnelUrlFromLogs -RuntimeDir $RuntimeDir
        if (-not [string]::IsNullOrWhiteSpace($url)) {
            return $url
        }
        Start-Sleep -Seconds 1
    } while ((Get-Date) -lt $deadline)

    throw "Timed out waiting for Cloudflare tunnel URL."
}

function Start-CloudflaredTunnel {
    param(
        [Parameter(Mandatory = $true)][string]$RuntimeDir,
        [Parameter(Mandatory = $true)][string]$LocalUrl,
        [Parameter(Mandatory = $true)][string]$Protocol
    )

    $cmd = Get-Command cloudflared -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($null -eq $cmd) {
        throw "cloudflared is not installed or not available in PATH."
    }

    $stdout = Join-Path $RuntimeDir "cloudflared-live.log"
    $stderr = Join-Path $RuntimeDir "cloudflared-live.err"
    $source = $cmd.Source
    $ext = [IO.Path]::GetExtension($source).ToLowerInvariant()
    $tunnelArgs = @("tunnel", "--protocol", $Protocol, "--url", $LocalUrl)

    if ($ext -eq ".ps1") {
        return Start-Process -FilePath "powershell.exe" `
            -ArgumentList (@("-NoProfile", "-ExecutionPolicy", "Bypass", "-File", $source) + $tunnelArgs) `
            -WindowStyle Hidden `
            -RedirectStandardOutput $stdout `
            -RedirectStandardError $stderr `
            -PassThru
    }
    if ($ext -eq ".cmd" -or $ext -eq ".bat") {
        return Start-Process -FilePath "cmd.exe" `
            -ArgumentList (@("/c", $source) + $tunnelArgs) `
            -WindowStyle Hidden `
            -RedirectStandardOutput $stdout `
            -RedirectStandardError $stderr `
            -PassThru
    }

    return Start-Process -FilePath $source `
        -ArgumentList $tunnelArgs `
        -WindowStyle Hidden `
        -RedirectStandardOutput $stdout `
        -RedirectStandardError $stderr `
        -PassThru
}

function Start-CloudflaredNamedTunnel {
    param(
        [Parameter(Mandatory = $true)][string]$RuntimeDir,
        [Parameter(Mandatory = $true)][string]$ConfigPath,
        [Parameter(Mandatory = $true)][string]$TunnelName
    )

    $cmd = Get-Command cloudflared -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($null -eq $cmd) {
        throw "cloudflared is not installed or not available in PATH."
    }
    if (-not (Test-Path -LiteralPath $ConfigPath)) {
        throw "Named Cloudflare tunnel config does not exist: $ConfigPath. Run scripts\dev\setup-cloudflare-tunnel.ps1 first."
    }

    $stdout = Join-Path $RuntimeDir "cloudflared-live.log"
    $stderr = Join-Path $RuntimeDir "cloudflared-live.err"
    $source = $cmd.Source
    $ext = [IO.Path]::GetExtension($source).ToLowerInvariant()
    $tunnelArgs = @("tunnel", "--config", $ConfigPath, "run", $TunnelName)

    if ($ext -eq ".ps1") {
        return Start-Process -FilePath "powershell.exe" `
            -ArgumentList (@("-NoProfile", "-ExecutionPolicy", "Bypass", "-File", $source) + $tunnelArgs) `
            -WindowStyle Hidden `
            -RedirectStandardOutput $stdout `
            -RedirectStandardError $stderr `
            -PassThru
    }
    if ($ext -eq ".cmd" -or $ext -eq ".bat") {
        return Start-Process -FilePath "cmd.exe" `
            -ArgumentList (@("/c", $source) + $tunnelArgs) `
            -WindowStyle Hidden `
            -RedirectStandardOutput $stdout `
            -RedirectStandardError $stderr `
            -PassThru
    }

    return Start-Process -FilePath $source `
        -ArgumentList $tunnelArgs `
        -WindowStyle Hidden `
        -RedirectStandardOutput $stdout `
        -RedirectStandardError $stderr `
        -PassThru
}

function Start-BotExecutable {
    param(
        [Parameter(Mandatory = $true)][string]$Root,
        [Parameter(Mandatory = $true)][string]$ExePath,
        [Parameter(Mandatory = $true)][string]$StdoutPath,
        [Parameter(Mandatory = $true)][string]$StderrPath
    )

    $safeRoot = $Root.Replace("'", "''")
    $safeExe = $ExePath.Replace("'", "''")
    $runner = @"
`$ErrorActionPreference = 'Stop'
Set-Location -LiteralPath '$safeRoot'
`$envPath = Join-Path (Get-Location) '.env'
if (Test-Path -LiteralPath `$envPath) {
    foreach (`$line in Get-Content -LiteralPath `$envPath) {
        if (`$line -match '^\s*#' -or `$line -match '^\s*$') { continue }
        if (`$line -match '^\s*([^=]+?)\s*=\s*(.*)\s*$') {
            `$name = `$matches[1].Trim()
            `$value = `$matches[2].Trim()
            if (`$value.Length -ge 2 -and ((([int][char]`$value[0]) -eq 34 -and ([int][char]`$value[`$value.Length - 1]) -eq 34) -or (([int][char]`$value[0]) -eq 39 -and ([int][char]`$value[`$value.Length - 1]) -eq 39))) {
                `$value = `$value.Substring(1, `$value.Length - 2)
            }
            [Environment]::SetEnvironmentVariable(`$name, `$value, 'Process')
        }
    }
}
& '$safeExe'
"@

    return Start-Process -FilePath "powershell.exe" `
        -ArgumentList @("-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", $runner) `
        -WorkingDirectory $Root `
        -WindowStyle Hidden `
        -RedirectStandardOutput $StdoutPath `
        -RedirectStandardError $StderrPath `
        -PassThru
}
