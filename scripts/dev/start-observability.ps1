param(
    [switch]$StatusOnly,
    [switch]$StopOnly,
    [switch]$StopDocker,
    [switch]$WaitDocker,
    [switch]$OpenDockerDesktop,
    [switch]$KeepDockerContainers,
    [switch]$SkipDocker,
    [switch]$SkipMigrate,
    [switch]$SkipBuild,
    [switch]$SkipMiniApp,
    [switch]$SkipAdmin,
    [switch]$SkipProviderWebhook,
    [switch]$NoRestart,
    [switch]$NoWait,
    [switch]$OpenGrafana,
    [switch]$OpenMiniApp,
    [switch]$OpenAdmin,
    [switch]$NoTracing,
    [switch]$NoFrontendTelemetry,
    [switch]$MockProviders,
    [switch]$RealVkDelivery,
    [ValidateSet("mock", "yookassa")]
    [string]$PaymentProvider = "mock",
    [int]$ApiPort = 8080,
    [int]$WorkerMetricsPort = 9090,
    [int]$ProviderWebhookPort = 8082,
    [int]$MiniAppPort = 5173,
    [int]$AdminPort = 5175,
    [int]$GrafanaPort = 3000,
    [int]$PrometheusPort = 9091,
    [int]$AlertmanagerPort = 9093,
    [int]$OtelGrpcPort = 4317,
    [int]$VkUserID = 777,
    [string]$ComposeProject = "vk-ai-aggregator",
    [int]$DockerWaitSeconds = 60,
    [int]$TimeoutSeconds = 180
)

$ErrorActionPreference = "Stop"
. (Join-Path $PSScriptRoot "_miniapp-common.ps1")

function Get-ObservabilityRuntimeDir {
    param([Parameter(Mandatory = $true)][string]$Root)

    return (Join-Path $Root ".runtime\observability-dev")
}

function Get-ObservabilityBinDir {
    param([Parameter(Mandatory = $true)][string]$Root)

    return (Join-Path $Root "bin\dev\observability")
}

function Set-LocalEnv {
    param(
        [Parameter(Mandatory = $true)][string]$Name,
        [Parameter(Mandatory = $true)][AllowEmptyString()][string]$Value
    )

    Set-Item -Path ("Env:\" + $Name) -Value $Value
}

function Invoke-NativeChecked {
    param(
        [Parameter(Mandatory = $true)][string]$FilePath,
        [Parameter(Mandatory = $true)][string[]]$ArgumentList
    )

    $oldPreference = $ErrorActionPreference
    try {
        $ErrorActionPreference = "Continue"
        & $FilePath @ArgumentList
        $exitCode = $LASTEXITCODE
    } finally {
        $ErrorActionPreference = $oldPreference
    }
    if ($exitCode -ne 0) {
        throw "$FilePath $($ArgumentList -join ' ') failed with exit code $exitCode."
    }
}

function Invoke-DockerRaw {
    param([Parameter(Mandatory = $true)][string[]]$ArgumentList)

    $otelEnvNames = @(
        "OTEL_EXPORTER_OTLP_ENDPOINT",
        "OTEL_TRACES_EXPORTER",
        "OTEL_SERVICE_NAME",
        "OTEL_TRACES_SAMPLE_RATIO",
        "OTEL_TRACES_CRITICAL_SAMPLE_RATIO"
    )
    $saved = @{}
    foreach ($name in $otelEnvNames) {
        $saved[$name] = [Environment]::GetEnvironmentVariable($name, "Process")
        Remove-Item -Path ("Env:\" + $name) -ErrorAction SilentlyContinue
    }

    $oldPreference = $ErrorActionPreference
    try {
        $ErrorActionPreference = "Continue"
        $output = & docker @ArgumentList 2>&1
        $exitCode = $LASTEXITCODE
        return [pscustomobject]@{
            ExitCode = $exitCode
            Output   = (($output | ForEach-Object { [string]$_ }) -join "`n")
        }
    } catch {
        return [pscustomobject]@{
            ExitCode = 1
            Output   = $_.Exception.Message
        }
    } finally {
        $ErrorActionPreference = $oldPreference
        foreach ($name in $otelEnvNames) {
            if ($null -eq $saved[$name]) {
                Remove-Item -Path ("Env:\" + $name) -ErrorAction SilentlyContinue
            } else {
                Set-Item -Path ("Env:\" + $name) -Value $saved[$name]
            }
        }
    }
}

function Invoke-DockerChecked {
    param([Parameter(Mandatory = $true)][string[]]$ArgumentList)

    $result = Invoke-DockerRaw -ArgumentList $ArgumentList
    if (-not [string]::IsNullOrWhiteSpace($result.Output)) {
        Write-Host $result.Output
    }
    if ($result.ExitCode -ne 0) {
        throw "docker $($ArgumentList -join ' ') failed with exit code $($result.ExitCode).`n$result.Output"
    }
    return $result
}

function Get-DevContainerNames {
    return @(
        "vk-ai-aggregator-postgres",
        "vk-ai-aggregator-redis",
        "vk-ai-aggregator-minio",
        "vk-ai-aggregator-prometheus",
        "vk-ai-aggregator-alertmanager",
        "vk-ai-aggregator-grafana",
        "vk-ai-aggregator-loki",
        "vk-ai-aggregator-alloy",
        "vk-ai-aggregator-tempo",
        "vk-ai-aggregator-otel-collector",
        "vk-ai-aggregator-blackbox-exporter",
        "vk-ai-aggregator-postgres-exporter",
        "vk-ai-aggregator-redis-exporter",
        "vk-ai-aggregator-node-exporter",
        "vk-ai-aggregator-cadvisor"
    )
}

function Get-DockerNameConflicts {
    param([Parameter(Mandatory = $true)][AllowEmptyString()][string]$Output)

    $names = New-Object System.Collections.Generic.List[string]
    $matches = [regex]::Matches($Output, 'container name "/([^"]+)" is already in use')
    foreach ($match in $matches) {
        $name = $match.Groups[1].Value.Trim()
        if ($name -ne "" -and -not $names.Contains($name)) {
            $names.Add($name)
        }
    }
    return @($names)
}

function Repair-DockerNameConflicts {
    param([Parameter(Mandatory = $true)][string[]]$ContainerNames)

    $allowed = @(Get-DevContainerNames)

    foreach ($name in $ContainerNames) {
        if ($allowed -notcontains $name) {
            throw "Refusing to remove unexpected Docker container conflict: $name"
        }
        Write-Host "Removing conflicting dev container without deleting volumes: $name"
        Invoke-DockerChecked -ArgumentList @("rm", "-f", $name) | Out-Null
    }
}

function Get-ComposeProjectLabel {
    param([Parameter(Mandatory = $true)][string]$ContainerName)

    $result = Invoke-DockerRaw -ArgumentList @("inspect", "-f", '{{ index .Config.Labels "com.docker.compose.project" }}', $ContainerName)
    if ($result.ExitCode -ne 0) {
        return $null
    }
    $label = $result.Output.Trim()
    if ($label -eq "" -or $label -eq "<no value>") {
        return ""
    }
    return $label
}

function Repair-StaleDevContainers {
    param([Parameter(Mandatory = $true)][string]$DesiredProject)

    foreach ($name in Get-DevContainerNames) {
        $project = Get-ComposeProjectLabel -ContainerName $name
        if ($null -eq $project) {
            continue
        }
        if ($project -eq $DesiredProject) {
            continue
        }
        Write-Host "Removing stale dev container from compose project '$project' without deleting volumes: $name"
        Invoke-DockerChecked -ArgumentList @("rm", "-f", $name) | Out-Null
    }
}

function Reset-DevContainersForComposeUp {
    foreach ($name in Get-DevContainerNames) {
        $exists = Invoke-DockerRaw -ArgumentList @("inspect", $name)
        if ($exists.ExitCode -ne 0) {
            continue
        }
        Write-Host "Removing existing dev container before clean compose up without deleting volumes: $name"
        Invoke-DockerChecked -ArgumentList @("rm", "-f", $name) | Out-Null
    }
}

function Invoke-ComposeUpWithConflictRepair {
    param([Parameter(Mandatory = $true)][string[]]$ArgumentList)

    $maxAttempts = (Get-DevContainerNames).Count + 2
    for ($attempt = 1; $attempt -le $maxAttempts; $attempt++) {
        $result = Invoke-DockerRaw -ArgumentList $ArgumentList
        if (-not [string]::IsNullOrWhiteSpace($result.Output)) {
            Write-Host $result.Output
        }
        if ($result.ExitCode -eq 0) {
            return
        }

        $conflicts = @(Get-DockerNameConflicts -Output $result.Output)
        if ($conflicts.Count -eq 0) {
            throw "docker $($ArgumentList -join ' ') failed with exit code $($result.ExitCode).`n$result.Output"
        }

        Repair-DockerNameConflicts -ContainerNames $conflicts
        Write-Host "Retrying docker compose up after removing container name conflicts..."
    }

    throw "docker $($ArgumentList -join ' ') still has container name conflicts after $maxAttempts repair attempts."
}

function Test-DockerEngineReady {
    $result = Invoke-DockerRaw -ArgumentList @("info")
    return [pscustomobject]@{
        Ready   = ($result.ExitCode -eq 0)
        Message = $result.Output
    }
}

function Ensure-ObservabilityDockerRunning {
    param(
        [switch]$Wait,
        [switch]$OpenDesktop,
        [int]$DockerWaitSeconds = 60
    )

    Write-Host "Checking Docker Engine..."
    $state = Test-DockerEngineReady
    if ($state.Ready) {
        return
    }

    if (-not $Wait) {
        throw @"
Docker Engine is not ready.
Last docker info output:
$($state.Message)

Start Docker Desktop manually, wait until the engine is running, then verify:
  docker info

Then rerun:
  powershell.exe -ExecutionPolicy Bypass -File scripts/dev/start-observability.ps1 -NoWait -OpenGrafana

If you want this script to wait for Docker explicitly, rerun with:
  -WaitDocker

If you also want it to open Docker Desktop explicitly, rerun with:
  -WaitDocker -OpenDockerDesktop
"@
    }

    if ($OpenDesktop) {
        Write-Host "Opening Docker Desktop..."
        $dockerDesktop = "C:\Program Files\Docker\Docker\Docker Desktop.exe"
        if (Test-Path -LiteralPath $dockerDesktop) {
            Start-Process -FilePath $dockerDesktop -WindowStyle Hidden | Out-Null
        } else {
            Write-Host "Docker Desktop executable was not found at: $dockerDesktop"
        }
    }

    $deadline = (Get-Date).AddSeconds($DockerWaitSeconds)
    do {
        Start-Sleep -Seconds 3
        $state = Test-DockerEngineReady
        if ($state.Ready) {
            Write-Host "Docker Engine is ready."
            return
        }
        $remaining = [Math]::Max(0, [int]($deadline - (Get-Date)).TotalSeconds)
        Write-Host "Waiting for Docker Engine... ${remaining}s left"
    } while ((Get-Date) -lt $deadline)

    throw @"
Docker Engine is not ready.
Last docker info output:
$($state.Message)

If the error contains 'permission denied' for dockerDesktopLinuxEngine, add your Windows user to the docker-users group, sign out/sign in, then rerun this script.
"@
}

function Get-ProcessState {
    param(
        [Parameter(Mandatory = $true)][string]$RuntimeDir,
        [Parameter(Mandatory = $true)][string]$Name
    )

    $pidFile = Join-Path $RuntimeDir "$Name.pid"
    if (-not (Test-Path -LiteralPath $pidFile)) {
        return "not tracked"
    }
    $raw = Get-Content -LiteralPath $pidFile -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($raw -notmatch '^\d+$') {
        return "invalid pid"
    }
    try {
        $proc = Get-Process -Id ([int]$raw) -ErrorAction Stop
        return "running pid=$($proc.Id)"
    } catch {
        return "not running pid=$raw"
    }
}

function Get-HttpState {
    param([Parameter(Mandatory = $true)][string]$Url)

    try {
        $response = Invoke-WebRequest -Uri $Url -UseBasicParsing -TimeoutSec 5
        return "ok status=$($response.StatusCode)"
    } catch {
        return "failed $($_.Exception.Message)"
    }
}

function Stop-ObservabilityProcesses {
    param(
        [Parameter(Mandatory = $true)][string]$Root,
        [Parameter(Mandatory = $true)][string]$RuntimeDir,
        [switch]$Quiet
    )

    if (-not $Quiet) {
        Write-Host "Stopping local observability dev processes..."
    }

    Stop-PidFileProcess -PidFile (Join-Path $RuntimeDir "api.pid")
    Stop-PidFileProcess -PidFile (Join-Path $RuntimeDir "worker.pid")
    Stop-PidFileProcess -PidFile (Join-Path $RuntimeDir "provider-webhook.pid")
    Stop-PidFileProcess -PidFile (Join-Path $RuntimeDir "vite.pid")
    Stop-PidFileProcess -PidFile (Join-Path $RuntimeDir "admin-vite.pid")

    Stop-ListenerOnPort -Port $ApiPort
    Stop-ListenerOnPort -Port $WorkerMetricsPort
    Stop-ListenerOnPort -Port $ProviderWebhookPort
    Stop-ListenerOnPort -Port $MiniAppPort
    Stop-ListenerOnPort -Port $AdminPort

    $escapedRoot = [regex]::Escape($Root)
    $escapedRuntime = [regex]::Escape($RuntimeDir)
    $escapedAdmin = [regex]::Escape((Join-Path $Root "web\admin"))
    $processes = Get-CimInstance Win32_Process -ErrorAction SilentlyContinue | Where-Object {
        $cmd = [string]$_.CommandLine
        if ($cmd -eq "") {
            return $false
        }
        ($cmd -match $escapedRuntime) -or
        ($cmd -match $escapedRoot -and (
            $cmd -match 'bin\\dev\\observability\\api\.exe' -or
            $cmd -match 'bin\\dev\\observability\\worker\.exe' -or
            $cmd -match 'bin\\dev\\observability\\provider-webhook\.exe'
        )) -or
        ($cmd -match $escapedAdmin -and $cmd -match 'vite')
    }
    foreach ($proc in $processes) {
        try {
            Stop-Process -Id $proc.ProcessId -Force
        } catch {}
    }
}

function Stop-ObservabilityDocker {
    param([Parameter(Mandatory = $true)][string]$Root)

    Push-Location $Root
    try {
        Invoke-DockerChecked -ArgumentList @("compose", "-p", $ComposeProject, "-f", "docker-compose.yml", "-f", "docker-compose.observability.yml", "stop")
    } finally {
        Pop-Location
    }
}

function Show-ObservabilityStatus {
    param(
        [Parameter(Mandatory = $true)][string]$Root,
        [Parameter(Mandatory = $true)][string]$RuntimeDir
    )

    Write-Host "Product observability status"
    Write-Host "Repo:              $Root"
    Write-Host "Runtime:           $RuntimeDir"
    Write-Host "API process:       $(Get-ProcessState -RuntimeDir $RuntimeDir -Name "api")"
    Write-Host "Worker process:    $(Get-ProcessState -RuntimeDir $RuntimeDir -Name "worker")"
    Write-Host "Webhook process:   $(Get-ProcessState -RuntimeDir $RuntimeDir -Name "provider-webhook")"
    Write-Host "Mini App process:  $(Get-ProcessState -RuntimeDir $RuntimeDir -Name "vite")"
    Write-Host "Admin UI process:  $(Get-ProcessState -RuntimeDir $RuntimeDir -Name "admin-vite")"
    Write-Host "API health:        $(Get-HttpState -Url "http://127.0.0.1:$ApiPort/healthz")"
    Write-Host "Worker health:     $(Get-HttpState -Url "http://127.0.0.1:$WorkerMetricsPort/healthz")"
    Write-Host "Webhook ready:     $(Get-HttpState -Url "http://127.0.0.1:$ProviderWebhookPort/readyz")"
    Write-Host "Mini App:          $(Get-HttpState -Url "http://127.0.0.1:$MiniAppPort/")"
    Write-Host "Admin UI:          $(Get-HttpState -Url "http://127.0.0.1:$AdminPort/")"
    Write-Host "Grafana:           $(Get-HttpState -Url "http://127.0.0.1:$GrafanaPort/api/health")"
    Write-Host "Prometheus:        $(Get-HttpState -Url "http://127.0.0.1:$PrometheusPort/-/ready")"
    Write-Host "Alertmanager:      $(Get-HttpState -Url "http://127.0.0.1:$AlertmanagerPort/-/ready")"

    Write-Host ""
    Write-Host "Docker stack:"
    Push-Location $Root
    try {
        $state = Test-DockerEngineReady
        if (-not $state.Ready) {
            Write-Host "Docker Engine is not ready or access is denied."
            Write-Host $state.Message
            return
        }
        Invoke-DockerChecked -ArgumentList @("compose", "-p", $ComposeProject, "-f", "docker-compose.yml", "-f", "docker-compose.observability.yml", "ps")
    } catch {
        Write-Host "Docker status failed: $($_.Exception.Message)"
    } finally {
        Pop-Location
    }
}

function Wait-ContainerDependencies {
    param([int]$TimeoutSeconds = 180)

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    do {
        $postgresOk = $false
        $redisOk = $false
        try {
            $postgresCheck = Invoke-DockerRaw -ArgumentList @("exec", "vk-ai-aggregator-postgres", "pg_isready", "-U", "vk_ai_aggregator", "-d", "vk_ai_aggregator")
            $postgresOk = ($postgresCheck.ExitCode -eq 0)
        } catch {}
        try {
            $redisCheck = Invoke-DockerRaw -ArgumentList @("exec", "vk-ai-aggregator-redis", "redis-cli", "ping")
            $redisOk = ($redisCheck.ExitCode -eq 0 -and $redisCheck.Output.Trim() -eq "PONG")
        } catch {}
        if ($postgresOk -and $redisOk) {
            Wait-TcpPort -Port 9000 -TimeoutSeconds 30 | Out-Null
            return
        }
        Start-Sleep -Seconds 2
    } while ((Get-Date) -lt $deadline)

    throw "Docker dependencies did not become healthy in time."
}

function Start-ObservedExecutable {
    param(
        [Parameter(Mandatory = $true)][string]$Root,
        [Parameter(Mandatory = $true)][string]$RuntimeDir,
        [Parameter(Mandatory = $true)][string]$Name,
        [Parameter(Mandatory = $true)][string]$ExePath
    )

    if (-not (Test-Path -LiteralPath $ExePath)) {
        throw "Executable does not exist: $ExePath"
    }
    $stdout = Join-Path $RuntimeDir "$Name-live.log"
    $stderr = Join-Path $RuntimeDir "$Name-live.err"
    $proc = Start-Process -FilePath $ExePath `
        -WorkingDirectory $Root `
        -WindowStyle Hidden `
        -RedirectStandardOutput $stdout `
        -RedirectStandardError $stderr `
        -PassThru
    Set-Content -Path (Join-Path $RuntimeDir "$Name.pid") -Value $proc.Id -Encoding ASCII
    return $proc
}

function Start-ObservedVite {
    param(
        [Parameter(Mandatory = $true)][string]$Root,
        [Parameter(Mandatory = $true)][string]$RuntimeDir,
        [Parameter(Mandatory = $true)][string]$LaunchParams
    )

    $miniappDir = Join-Path $Root "web\miniapp"
    $stdout = Join-Path $RuntimeDir "vite-live.log"
    $stderr = Join-Path $RuntimeDir "vite-live.err"

    $oldLaunchParams = [Environment]::GetEnvironmentVariable("VITE_DEV_LAUNCH_PARAMS", "Process")
    $oldTelemetry = [Environment]::GetEnvironmentVariable("VITE_FRONTEND_TELEMETRY_ENABLED", "Process")
    try {
        Set-LocalEnv -Name "VITE_DEV_LAUNCH_PARAMS" -Value $LaunchParams
        Set-LocalEnv -Name "VITE_FRONTEND_TELEMETRY_ENABLED" -Value ($(if ($NoFrontendTelemetry) { "false" } else { "true" }))
        $proc = Start-Process npm.cmd `
            -ArgumentList @("run", "dev", "--", "--host", "127.0.0.1", "--port", [string]$MiniAppPort) `
            -WorkingDirectory $miniappDir `
            -WindowStyle Hidden `
            -RedirectStandardOutput $stdout `
            -RedirectStandardError $stderr `
            -PassThru
        Set-Content -Path (Join-Path $RuntimeDir "vite.pid") -Value $proc.Id -Encoding ASCII
        return $proc
    } finally {
        if ($null -eq $oldLaunchParams) {
            Remove-Item Env:\VITE_DEV_LAUNCH_PARAMS -ErrorAction SilentlyContinue
        } else {
            Set-LocalEnv -Name "VITE_DEV_LAUNCH_PARAMS" -Value $oldLaunchParams
        }
        if ($null -eq $oldTelemetry) {
            Remove-Item Env:\VITE_FRONTEND_TELEMETRY_ENABLED -ErrorAction SilentlyContinue
        } else {
            Set-LocalEnv -Name "VITE_FRONTEND_TELEMETRY_ENABLED" -Value $oldTelemetry
        }
    }
}

function Start-ObservedAdminVite {
    param(
        [Parameter(Mandatory = $true)][string]$Root,
        [Parameter(Mandatory = $true)][string]$RuntimeDir
    )

    $adminDir = Join-Path $Root "web\admin"
    $stdout = Join-Path $RuntimeDir "admin-vite-live.log"
    $stderr = Join-Path $RuntimeDir "admin-vite-live.err"

    $oldDevHost = [Environment]::GetEnvironmentVariable("VITE_ADMIN_DEV_HOST", "Process")
    $oldApiTarget = [Environment]::GetEnvironmentVariable("VITE_ADMIN_API_TARGET", "Process")
    try {
        Set-LocalEnv -Name "VITE_ADMIN_DEV_HOST" -Value "127.0.0.1"
        Set-LocalEnv -Name "VITE_ADMIN_API_TARGET" -Value "http://127.0.0.1:$ApiPort"
        $proc = Start-Process npm.cmd `
            -ArgumentList @("run", "dev", "--", "--host", "127.0.0.1", "--port", [string]$AdminPort) `
            -WorkingDirectory $adminDir `
            -WindowStyle Hidden `
            -RedirectStandardOutput $stdout `
            -RedirectStandardError $stderr `
            -PassThru
        Set-Content -Path (Join-Path $RuntimeDir "admin-vite.pid") -Value $proc.Id -Encoding ASCII
        return $proc
    } finally {
        if ($null -eq $oldDevHost) {
            Remove-Item Env:\VITE_ADMIN_DEV_HOST -ErrorAction SilentlyContinue
        } else {
            Set-LocalEnv -Name "VITE_ADMIN_DEV_HOST" -Value $oldDevHost
        }
        if ($null -eq $oldApiTarget) {
            Remove-Item Env:\VITE_ADMIN_API_TARGET -ErrorAction SilentlyContinue
        } else {
            Set-LocalEnv -Name "VITE_ADMIN_API_TARGET" -Value $oldApiTarget
        }
    }
}

function Assert-ObservabilityRequirements {
    param(
        [switch]$NeedMiniApp,
        [switch]$NeedAdmin
    )

    if ($null -eq (Get-Command go -ErrorAction SilentlyContinue | Select-Object -First 1)) {
        throw "go is not installed or not available in PATH."
    }
    if (($NeedMiniApp -or $NeedAdmin) -and $null -eq (Get-Command npm.cmd -ErrorAction SilentlyContinue | Select-Object -First 1)) {
        throw "npm.cmd is not installed or not available in PATH."
    }
}

function Configure-ObservabilityEnv {
    Import-MiniAppDevEnv -Root $root

    Set-LocalEnv -Name "APP_ENV" -Value "development"
    Set-LocalEnv -Name "HTTP_ADDR" -Value ":$ApiPort"
    Set-LocalEnv -Name "WORKER_METRICS_ADDR" -Value ":$WorkerMetricsPort"
    Set-LocalEnv -Name "PAYMENT_WEBHOOK_ADDR" -Value ":$ProviderWebhookPort"
    Set-LocalEnv -Name "PAYMENT_PROVIDER" -Value $PaymentProvider
    Set-LocalEnv -Name "PAYMENT_WEBHOOK_REQUIRE_HTTPS" -Value "false"

    if ($NoTracing) {
        Set-LocalEnv -Name "OTEL_TRACES_EXPORTER" -Value "none"
    } else {
        Set-LocalEnv -Name "OTEL_TRACES_EXPORTER" -Value "otlp"
        Set-LocalEnv -Name "OTEL_EXPORTER_OTLP_ENDPOINT" -Value "http://127.0.0.1:$OtelGrpcPort"
    }

    if ($NoFrontendTelemetry) {
        Set-LocalEnv -Name "FRONTEND_TELEMETRY_ENABLED" -Value "false"
    } else {
        Set-LocalEnv -Name "FRONTEND_TELEMETRY_ENABLED" -Value "true"
        if ([string]::IsNullOrWhiteSpace($env:FRONTEND_TELEMETRY_USER_HASH_SECRET)) {
            Set-LocalEnv -Name "FRONTEND_TELEMETRY_USER_HASH_SECRET" -Value "local-dev-frontend-telemetry"
        }
    }

    if ($MockProviders) {
        Set-LocalEnv -Name "PROVIDER" -Value "mock"
        Set-LocalEnv -Name "PROVIDER_CHAIN" -Value "mock"
        Set-LocalEnv -Name "IMAGE_PROVIDER" -Value "mock"
        Set-LocalEnv -Name "VIDEO_PROVIDER" -Value "mock"
    }

    if ($RealVkDelivery) {
        if ([string]::IsNullOrWhiteSpace($env:VK_DELIVERY_MODE)) {
            Set-LocalEnv -Name "VK_DELIVERY_MODE" -Value "real"
        }
    } else {
        Set-LocalEnv -Name "VK_DELIVERY_MODE" -Value "mock"
    }
}

$root = Get-RepoRoot
$runtime = Get-ObservabilityRuntimeDir -Root $root
$bin = Get-ObservabilityBinDir -Root $root

Ensure-Directory -Path $runtime
Ensure-Directory -Path $bin

Push-Location $root
try {
    if ($StatusOnly) {
        Show-ObservabilityStatus -Root $root -RuntimeDir $runtime
        exit 0
    }

    if ($StopOnly) {
        Stop-ObservabilityProcesses -Root $root -RuntimeDir $runtime
        if ($StopDocker) {
            Stop-ObservabilityDocker -Root $root
        }
        Write-Host "Product observability dev stack stopped."
        exit 0
    }

    if (-not $NoRestart) {
        Stop-ObservabilityProcesses -Root $root -RuntimeDir $runtime -Quiet
    }

    Configure-ObservabilityEnv
    Assert-ObservabilityRequirements -NeedMiniApp:(-not $SkipMiniApp) -NeedAdmin:(-not $SkipAdmin)

    if (-not $SkipDocker) {
        Write-Host "Starting Docker dependencies and observability stack..."
        Ensure-ObservabilityDockerRunning -Wait:$WaitDocker -OpenDesktop:$OpenDockerDesktop -DockerWaitSeconds $DockerWaitSeconds
        if ($KeepDockerContainers) {
            Repair-StaleDevContainers -DesiredProject $ComposeProject
        } else {
            Reset-DevContainersForComposeUp
        }
        Invoke-ComposeUpWithConflictRepair -ArgumentList @("compose", "-p", $ComposeProject, "-f", "docker-compose.yml", "-f", "docker-compose.observability.yml", "up", "-d")
        Wait-ContainerDependencies -TimeoutSeconds $TimeoutSeconds
        Wait-Http -Url "http://127.0.0.1:$PrometheusPort/-/ready" -TimeoutSeconds $TimeoutSeconds | Out-Null
        Wait-Http -Url "http://127.0.0.1:$GrafanaPort/api/health" -TimeoutSeconds $TimeoutSeconds | Out-Null
    }

    if (-not $SkipMigrate) {
        Write-Host "Applying database migrations..."
        Invoke-NativeChecked -FilePath "go" -ArgumentList @("run", "./cmd/migrate", "up")
    }

    $apiExe = Join-Path $bin "api.exe"
    $workerExe = Join-Path $bin "worker.exe"
    $providerWebhookExe = Join-Path $bin "provider-webhook.exe"

    if (-not $SkipBuild) {
        Write-Host "Building API, worker and provider-webhook binaries..."
        Invoke-NativeChecked -FilePath "go" -ArgumentList @("build", "-o", $apiExe, "./cmd/api")
        Invoke-NativeChecked -FilePath "go" -ArgumentList @("build", "-o", $workerExe, "./cmd/worker")
        if (-not $SkipProviderWebhook) {
            Invoke-NativeChecked -FilePath "go" -ArgumentList @("build", "-o", $providerWebhookExe, "./cmd/provider-webhook")
        }
    }

    Write-Host "Starting API..."
    Start-ObservedExecutable -Root $root -RuntimeDir $runtime -Name "api" -ExePath $apiExe | Out-Null
    Wait-Http -Url "http://127.0.0.1:$ApiPort/healthz" -TimeoutSeconds $TimeoutSeconds | Out-Null

    Write-Host "Starting worker..."
    Start-ObservedExecutable -Root $root -RuntimeDir $runtime -Name "worker" -ExePath $workerExe | Out-Null
    Wait-Http -Url "http://127.0.0.1:$WorkerMetricsPort/healthz" -TimeoutSeconds $TimeoutSeconds | Out-Null

    if (-not $SkipProviderWebhook) {
        Write-Host "Starting provider-webhook..."
        Start-ObservedExecutable -Root $root -RuntimeDir $runtime -Name "provider-webhook" -ExePath $providerWebhookExe | Out-Null
        Wait-Http -Url "http://127.0.0.1:$ProviderWebhookPort/readyz" -TimeoutSeconds $TimeoutSeconds | Out-Null
    }

    if (-not $SkipMiniApp) {
        Write-Host "Starting Mini App Vite dev server..."
        $launchParams = New-MiniAppLaunchParams -UserID $VkUserID
        Start-ObservedVite -Root $root -RuntimeDir $runtime -LaunchParams $launchParams | Out-Null
        Wait-Http -Url "http://127.0.0.1:$MiniAppPort/" -TimeoutSeconds $TimeoutSeconds | Out-Null
    }

    if (-not $SkipAdmin) {
        Write-Host "Starting Admin UI Vite dev server..."
        Start-ObservedAdminVite -Root $root -RuntimeDir $runtime | Out-Null
        Wait-Http -Url "http://127.0.0.1:$AdminPort/" -TimeoutSeconds $TimeoutSeconds | Out-Null
    }

    Write-Host ""
    Write-Host "Product observability dev stack is running."
    Write-Host "Grafana:           http://127.0.0.1:$GrafanaPort"
    Write-Host "Prometheus:        http://127.0.0.1:$PrometheusPort"
    Write-Host "Alertmanager:      http://127.0.0.1:$AlertmanagerPort"
    Write-Host "API health:        http://127.0.0.1:$ApiPort/healthz"
    Write-Host "Worker health:     http://127.0.0.1:$WorkerMetricsPort/healthz"
    if (-not $SkipProviderWebhook) {
        Write-Host "Webhook ready:     http://127.0.0.1:$ProviderWebhookPort/readyz"
    }
    if (-not $SkipMiniApp) {
        Write-Host "Mini App:          http://127.0.0.1:$MiniAppPort"
    }
    if (-not $SkipAdmin) {
        Write-Host "Admin UI:          http://127.0.0.1:$AdminPort"
    }
    Write-Host "Logs:              $runtime"
    Write-Host ""
    Write-Host "Status:            .\scripts\dev\start-observability.ps1 -StatusOnly"
    Write-Host "Stop apps:         .\scripts\dev\start-observability.ps1 -StopOnly"
    Write-Host "Stop everything:   .\scripts\dev\start-observability.ps1 -StopOnly -StopDocker"

    if ($OpenGrafana) {
        Start-Process "http://127.0.0.1:$GrafanaPort"
    }
    if ($OpenMiniApp -and -not $SkipMiniApp) {
        Start-Process "http://127.0.0.1:$MiniAppPort"
    }
    if ($OpenAdmin -and -not $SkipAdmin) {
        Start-Process "http://127.0.0.1:$AdminPort"
    }

    if ($NoWait) {
        exit 0
    }

    Read-Host "Press Enter to stop local app processes"
    Stop-ObservabilityProcesses -Root $root -RuntimeDir $runtime
} finally {
    Pop-Location
}
