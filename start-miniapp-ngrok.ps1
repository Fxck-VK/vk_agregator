param(
  [int]$ApiPort = 8080,
  [int]$MiniAppPort = 5173,
  [int]$WorkerMetricsPort = 9090,
  [int]$VkUserID = 777,
  [string]$EnvFile = ".env",
  [switch]$Migrate,
  [switch]$NoNgrok,
  [switch]$CheckOnly,
  [switch]$NoWait,
  [switch]$OpenBrowser,
  [switch]$StopOnly
)

$ErrorActionPreference = "Stop"

$Root = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $Root

function Write-Step($Text) {
  Write-Host ""
  Write-Host "==> $Text" -ForegroundColor Cyan
}

function Load-DotEnv($Path) {
  if (-not (Test-Path $Path)) {
    throw "Env file not found: $Path"
  }

  foreach ($line in Get-Content $Path) {
    $trim = $line.Trim()
    if ($trim.Length -eq 0 -or $trim.StartsWith("#") -or -not $trim.Contains("=")) {
      continue
    }
    $idx = $trim.IndexOf("=")
    $key = $trim.Substring(0, $idx).Trim()
    $value = $trim.Substring($idx + 1).Trim()
    if ($value.StartsWith('"') -and $value.EndsWith('"') -and $value.Length -ge 2) {
      $value = $value.Substring(1, $value.Length - 2)
    }
    if ($value.StartsWith("'") -and $value.EndsWith("'") -and $value.Length -ge 2) {
      $value = $value.Substring(1, $value.Length - 2)
    }
    if ($key -match "^[A-Za-z_][A-Za-z0-9_]*$") {
      Set-Item -Path ("Env:\" + $key) -Value $value
    }
  }
}

function Require-Command($Name) {
  if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
    throw "Required command not found: $Name"
  }
}

function Env-State($Name) {
  if (-not (Test-Path ("Env:\" + $Name))) {
    return "missing"
  }
  $value = (Get-Item ("Env:\" + $Name)).Value
  if ([string]::IsNullOrWhiteSpace($value)) {
    return "empty"
  }
  return "present"
}

function Stop-Port($Port) {
  $conns = Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue
  foreach ($conn in $conns) {
    $proc = Get-Process -Id $conn.OwningProcess -ErrorAction SilentlyContinue
    if ($proc -and $proc.ProcessName -notin @("postgres", "redis-server", "minio")) {
      Write-Host "Stopping $($proc.ProcessName) on port $Port (pid $($proc.Id))"
      Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue
    }
  }
}

function Stop-ViteDevServer($Port) {
  $needle = "--port $Port"
  Get-CimInstance Win32_Process -ErrorAction SilentlyContinue |
    Where-Object {
      $_.Name -eq "node.exe" -and
      $_.CommandLine -and
      $_.CommandLine.Contains($needle) -and
      ($_.CommandLine.Contains("vite") -or $_.CommandLine.Contains("npm-cli.js"))
    } |
    ForEach-Object {
      Write-Host "Stopping Vite node process on port $Port (pid $($_.ProcessId))"
      Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue
    }
}

function Wait-Http($Url, $Name, $Seconds = 30) {
  $deadline = (Get-Date).AddSeconds($Seconds)
  while ((Get-Date) -lt $deadline) {
    try {
      Invoke-WebRequest -Uri $Url -UseBasicParsing -TimeoutSec 2 | Out-Null
      return
    } catch {
      Start-Sleep -Milliseconds 500
    }
  }
  throw "$Name did not become ready: $Url"
}

function New-LaunchParams($UserID) {
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

function Public-NgrokUrl($LogPath) {
  for ($i = 0; $i -lt 90; $i++) {
    try {
      $data = Invoke-RestMethod -Uri "http://127.0.0.1:4040/api/tunnels" -TimeoutSec 2
      $url = ($data.tunnels | Where-Object { $_.proto -eq "https" } | Select-Object -First 1).public_url
      if ($url) {
        return $url
      }
    } catch {
      # ngrok's local API appears a moment after the process starts.
    }
    if (Test-Path $LogPath) {
      $match = Select-String -Path $LogPath -Pattern "url=https://\S+" | Select-Object -Last 1
      if ($match -and $match.Matches.Count -gt 0) {
        return $match.Matches[0].Value.Substring(4)
      }
    }
    Start-Sleep -Milliseconds 500
  }
  throw "ngrok tunnel URL was not reported on http://127.0.0.1:4040"
}

if ($StopOnly) {
  Write-Step "Stopping local Mini App processes"
  Stop-Port $ApiPort
  Stop-Port $MiniAppPort
  Stop-ViteDevServer $MiniAppPort
  Stop-Port $WorkerMetricsPort
  if (-not $NoNgrok) {
    Stop-Port 4040
  }
  exit 0
}

Write-Step "Loading env from $EnvFile"
Load-DotEnv (Join-Path $Root $EnvFile)

$env:APP_ENV = "development"
$env:HTTP_ADDR = ":$ApiPort"
$env:PROVIDER = "deepinfra"
$env:PROVIDER_CHAIN = "deepinfra"
$env:VK_DELIVERY_MODE = "mock"
$env:MODERATION_PROVIDER = "keyword"
$env:ARTIFACT_SCANNER = "none"
$env:WORKER_METRICS_ADDR = ":$WorkerMetricsPort"

Write-Host "DEEPINFRA_API_KEY: $(Env-State 'DEEPINFRA_API_KEY')"
Write-Host "DEEPINFRA_TEXT_MODEL: $(Env-State 'DEEPINFRA_TEXT_MODEL')"
Write-Host "DATABASE_URL: $(Env-State 'DATABASE_URL')"
Write-Host "REDIS_ADDR: $(Env-State 'REDIS_ADDR')"
Write-Host "S3_ENDPOINT: $(Env-State 'S3_ENDPOINT')"
Write-Host "S3_BUCKET: $(Env-State 'S3_BUCKET')"

if ((Env-State "DEEPINFRA_API_KEY") -ne "present") {
  throw "DEEPINFRA_API_KEY is required for real Mini App chat responses"
}

Require-Command "go"
Require-Command "npm.cmd"
if (-not $NoNgrok) {
  Require-Command "ngrok"
}

if ($CheckOnly) {
  Write-Host "CheckOnly completed."
  exit 0
}

Write-Step "Stopping local app processes on known ports"
Stop-Port $ApiPort
Stop-Port $MiniAppPort
Stop-ViteDevServer $MiniAppPort
Stop-Port $WorkerMetricsPort
if (-not $NoNgrok) {
  Stop-Port 4040
}

Write-Step "Database status"
if ($Migrate) {
  go run ./cmd/migrate up
} else {
  go run ./cmd/migrate status
}

$RunDir = Join-Path $env:TEMP "vkagg-miniapp-ngrok"
New-Item -ItemType Directory -Force -Path $RunDir | Out-Null
$ApiExe = Join-Path $RunDir "api.exe"
$WorkerExe = Join-Path $RunDir "worker.exe"
$ApiOut = Join-Path $RunDir "api.out.log"
$ApiErr = Join-Path $RunDir "api.err.log"
$WorkerOut = Join-Path $RunDir "worker.out.log"
$WorkerErr = Join-Path $RunDir "worker.err.log"
$ViteOut = Join-Path $RunDir "vite.out.log"
$ViteErr = Join-Path $RunDir "vite.err.log"
$NgrokOut = Join-Path $RunDir "ngrok.out.log"
$NgrokErr = Join-Path $RunDir "ngrok.err.log"
Remove-Item -ErrorAction SilentlyContinue $ApiOut,$ApiErr,$WorkerOut,$WorkerErr,$ViteOut,$ViteErr,$NgrokOut,$NgrokErr

$api = $null
$worker = $null
$vite = $null
$ngrok = $null
$keepRunning = $false

try {
  Write-Step "Building local API and worker binaries"
  go build -o $ApiExe ./cmd/api
  go build -o $WorkerExe ./cmd/worker

  Write-Step "Starting API, worker and Mini App dev server"
  $api = Start-Process $ApiExe -WorkingDirectory $Root -RedirectStandardOutput $ApiOut -RedirectStandardError $ApiErr -PassThru -WindowStyle Hidden
  $worker = Start-Process $WorkerExe -WorkingDirectory $Root -RedirectStandardOutput $WorkerOut -RedirectStandardError $WorkerErr -PassThru -WindowStyle Hidden
  $devLaunchParams = New-LaunchParams $VkUserID
  $env:VITE_DEV_LAUNCH_PARAMS = $devLaunchParams
  $vite = Start-Process npm.cmd -ArgumentList @("run", "dev", "--", "--host", "127.0.0.1", "--port", [string]$MiniAppPort) -WorkingDirectory (Join-Path $Root "web/miniapp") -RedirectStandardOutput $ViteOut -RedirectStandardError $ViteErr -PassThru -WindowStyle Hidden
  Remove-Item Env:\VITE_DEV_LAUNCH_PARAMS -ErrorAction SilentlyContinue

  Wait-Http "http://127.0.0.1:$ApiPort/health" "API"
  Wait-Http "http://127.0.0.1:$MiniAppPort" "Vite"

  $publicUrl = "http://127.0.0.1:$MiniAppPort"
  if (-not $NoNgrok) {
    Write-Step "Starting ngrok HTTPS tunnel"
    $ngrok = Start-Process ngrok -ArgumentList @("http", "http://127.0.0.1:$MiniAppPort", "--log=stdout") -WorkingDirectory $Root -RedirectStandardOutput $NgrokOut -RedirectStandardError $NgrokErr -PassThru -WindowStyle Hidden
    $publicUrl = Public-NgrokUrl $NgrokOut
  }

  $launchUrl = "$publicUrl/?$devLaunchParams"
  $LaunchShortcut = Join-Path $RunDir "miniapp-launch.url"
  Set-Content -Path $LaunchShortcut -Encoding ASCII -Value "[InternetShortcut]`r`nURL=$launchUrl`r`n"

  Write-Step "Mini App is ready"
  Write-Host "Frontend URL: $publicUrl"
  Write-Host "Local browser test shortcut: $LaunchShortcut"
  Write-Host ""
  Write-Host "For VK Mini App settings, use the Frontend URL above."
  Write-Host "The shortcut includes launch params so /miniapp/estimate passes auth in a plain browser."
  if ($OpenBrowser) {
    Start-Process $launchUrl
  }
  Write-Host ""
  Write-Host "Logs:"
  Write-Host "  API:    $ApiOut / $ApiErr"
  Write-Host "  Worker: $WorkerOut / $WorkerErr"
  Write-Host "  Vite:   $ViteOut / $ViteErr"
  if (-not $NoNgrok) {
    Write-Host "  ngrok:  $NgrokOut / $NgrokErr"
  }
  Write-Host ""

  if ($NoWait) {
    $keepRunning = $true
    [Console]::Out.WriteLine("")
    [Console]::Out.WriteLine("Mini App background run:")
    [Console]::Out.WriteLine("  Frontend URL: $publicUrl")
    [Console]::Out.WriteLine("  Local browser test shortcut: $LaunchShortcut")
    [Console]::Out.WriteLine("  API pid:    $($api.Id)")
    [Console]::Out.WriteLine("  Worker pid: $($worker.Id)")
    [Console]::Out.WriteLine("  Vite pid:   $($vite.Id)")
    if ($ngrok) {
      [Console]::Out.WriteLine("  ngrok pid:  $($ngrok.Id)")
    }
    [Console]::Out.WriteLine("")
    [Console]::Out.WriteLine("Stop them with:")
    [Console]::Out.WriteLine("  powershell -ExecutionPolicy Bypass -File .\start-miniapp-ngrok.ps1 -StopOnly")
    [Console]::Out.Flush()
    Start-Sleep -Milliseconds 100
    exit 0
  }

  Read-Host "Press Enter to stop API, worker, Vite and ngrok"
} finally {
  if (-not $keepRunning) {
    Write-Step "Stopping local processes"
    foreach ($proc in @($api, $worker, $vite, $ngrok)) {
      if ($proc -and -not $proc.HasExited) {
        Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue
      }
    }
    Stop-Port $ApiPort
    Stop-Port $MiniAppPort
    Stop-ViteDevServer $MiniAppPort
    Stop-Port $WorkerMetricsPort
    if (-not $NoNgrok) {
      Stop-Port 4040
    }
  }
}
