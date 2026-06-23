[CmdletBinding()]
param(
    [string]$EnvFile = ".env.loadtest",
    [string]$RedisAddr = "",
    [string]$RedisPassword = "",
    [int]$RedisDb = -1,
    [string[]]$Streams = @(),
    [string]$ConsumerGroup = "",
    [int]$PendingIdleMs = -1,
    [int]$PendingLimit = -1,
    [string[]]$KeyPatterns = @(),
    [int]$ScanCount = -1,
    [int]$KeyPatternScanLimit = -1,
    [string]$OutputFile = "",
    [string]$SnapshotFile = "",
    [string]$ComposeProjectName = "",
    [switch]$UseDockerCompose,
    [switch]$AllowProduction
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$RepoRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..")
Set-Location $RepoRoot

function Read-EnvFile {
    param([string]$Path)

    $values = @{}
    if (-not (Test-Path $Path)) {
        return $values
    }

    foreach ($line in Get-Content -LiteralPath $Path) {
        $trimmed = $line.Trim()
        if ($trimmed.Length -eq 0 -or $trimmed.StartsWith("#")) {
            continue
        }

        $idx = $trimmed.IndexOf("=")
        if ($idx -lt 1) {
            continue
        }

        $key = $trimmed.Substring(0, $idx).Trim()
        $value = $trimmed.Substring($idx + 1).Trim()
        if (($value.StartsWith('"') -and $value.EndsWith('"')) -or ($value.StartsWith("'") -and $value.EndsWith("'"))) {
            $value = $value.Substring(1, $value.Length - 2)
        }

        $values[$key] = $value
    }

    return $values
}

$EnvValues = Read-EnvFile -Path $EnvFile

function Get-Setting {
    param(
        [string]$Name,
        [string]$Default = ""
    )

    if ($EnvValues.ContainsKey($Name) -and $EnvValues[$Name] -ne "") {
        return $EnvValues[$Name]
    }

    $fromProcess = [Environment]::GetEnvironmentVariable($Name)
    if (-not [string]::IsNullOrWhiteSpace($fromProcess)) {
        return $fromProcess
    }

    return $Default
}

function Split-SettingList {
    param(
        [string[]]$Explicit,
        [string]$EnvName,
        [string[]]$Default
    )

    if ($Explicit.Count -gt 0) {
        return $Explicit | ForEach-Object { $_.Trim() } | Where-Object { $_ -ne "" }
    }

    $fromEnv = Get-Setting -Name $EnvName
    if (-not [string]::IsNullOrWhiteSpace($fromEnv)) {
        return $fromEnv.Split(",") | ForEach-Object { $_.Trim() } | Where-Object { $_ -ne "" }
    }

    return $Default
}

function Resolve-RedisEndpoint {
    param([string]$Address)

    $endpoint = @{
        Host = "127.0.0.1"
        Port = "6379"
    }

    if ([string]::IsNullOrWhiteSpace($Address)) {
        return $endpoint
    }

    if ($Address -match "^[a-z]+://") {
        $uri = [Uri]$Address
        $endpoint.Host = $uri.Host
        if ($uri.Port -gt 0) {
            $endpoint.Port = [string]$uri.Port
        }
        return $endpoint
    }

    if ($Address -match "^(?<host>[^:]+):(?<port>\d+)$") {
        $endpoint.Host = $Matches["host"]
        $endpoint.Port = $Matches["port"]
        return $endpoint
    }

    $endpoint.Host = $Address
    return $endpoint
}

function Add-Line {
    param([string]$Line)
    [void]$script:Output.Add($Line)
}

function Add-Section {
    param([string]$Title)
    Add-Line ""
    Add-Line "## $Title"
}

function Add-CommandOutput {
    param([object[]]$Lines)

    foreach ($line in $Lines) {
        if ($null -eq $line) {
            continue
        }
        Add-Line ([string]$line)
    }
}

function Invoke-RedisCli {
    param(
        [string[]]$RedisArguments,
        [switch]$AllowFailure
    )

    if ($UseDockerCompose) {
        $composeArgs = @("compose", "-p", $ComposeProjectName)
        if (Test-Path $EnvFile) {
            $composeArgs += @("--env-file", $EnvFile)
        }
        $composeArgs += @("-f", "docker-compose.data.yml", "exec", "-T")
        if (-not [string]::IsNullOrWhiteSpace($RedisPassword)) {
            $composeArgs += @("-e", "REDISCLI_AUTH=$RedisPassword")
        }
        $composeArgs += @("redis", "redis-cli", "--raw", "-n", "$RedisDb")
        $composeArgs += $RedisArguments

        $result = & docker @composeArgs 2>&1
        $exitCode = $LASTEXITCODE
    } else {
        $endpoint = Resolve-RedisEndpoint -Address $RedisAddr
        $redisArgs = @("--raw", "-h", $endpoint.Host, "-p", $endpoint.Port, "-n", "$RedisDb")
        $redisArgs += $RedisArguments

        $previousAuth = [Environment]::GetEnvironmentVariable("REDISCLI_AUTH")
        try {
            if (-not [string]::IsNullOrWhiteSpace($RedisPassword)) {
                $env:REDISCLI_AUTH = $RedisPassword
            }
            $result = & redis-cli @redisArgs 2>&1
            $exitCode = $LASTEXITCODE
        } finally {
            if ($null -eq $previousAuth) {
                Remove-Item Env:\REDISCLI_AUTH -ErrorAction SilentlyContinue
            } else {
                $env:REDISCLI_AUTH = $previousAuth
            }
        }
    }

    if ($exitCode -ne 0) {
        $display = "redis-cli " + ($RedisArguments -join " ")
        if ($AllowFailure) {
            return @("WARN: $display failed with exit code $exitCode") + $result
        }
        throw "$display failed with exit code $exitCode. $result"
    }

    return $result
}

function Get-FirstLine {
    param([object[]]$Lines)

    foreach ($line in $Lines) {
        if ($null -ne $line -and [string]$line -ne "") {
            return [string]$line
        }
    }

    return ""
}

function Get-RedisKeyType {
    param([string]$Key)

    return Get-FirstLine -Lines (Invoke-RedisCli -RedisArguments @("TYPE", $Key) -AllowFailure)
}

function Convert-ToInt64OrZero {
    param([object]$Value)

    $parsed = [int64]0
    if ($null -ne $Value -and [int64]::TryParse(([string]$Value).Trim(), [ref]$parsed)) {
        return $parsed
    }

    return [int64]0
}

function Parse-RedisInfo {
    param([object[]]$Lines)

    $info = @{}
    foreach ($line in $Lines) {
        $text = [string]$line
        if ($text.StartsWith("#") -or $text.Trim() -eq "") {
            continue
        }

        $idx = $text.IndexOf(":")
        if ($idx -gt 0) {
            $info[$text.Substring(0, $idx)] = $text.Substring($idx + 1).Trim()
        }
    }

    return $info
}

function Get-RedisNumber {
    param([string[]]$RedisArguments)

    $lines = @(Invoke-RedisCli -RedisArguments $RedisArguments -AllowFailure)
    if ($lines.Count -eq 0 -or ([string]$lines[0]).StartsWith("WARN:")) {
        return [int64]0
    }

    return Convert-ToInt64OrZero -Value $lines[0]
}

function Get-StreamDiagnostics {
    param(
        [string]$Stream,
        [string]$Group
    )

    $streamType = Get-FirstLine -Lines (Invoke-RedisCli -RedisArguments @("TYPE", $Stream) -AllowFailure)
    if ($streamType -eq "none" -or $streamType.StartsWith("WARN:")) {
        return [PSCustomObject]@{
            Stream    = $Stream
            Type      = $streamType
            Length    = 0
            Groups    = 0
            Consumers = 0
            Pending   = 0
            Lag       = 0
            Backlog   = 0
            IsDLQ     = ($Stream -like "*dlq*")
        }
    }

    $length = Get-RedisNumber -RedisArguments @("XLEN", $Stream)
    $groups = 0
    $consumers = 0
    $pending = 0
    $lag = 0

    $groupInfo = @(Invoke-RedisCli -RedisArguments @("XINFO", "GROUPS", $Stream) -AllowFailure)
    if ($groupInfo.Count -gt 0 -and -not ([string]$groupInfo[0]).StartsWith("WARN:")) {
        for ($i = 0; $i -lt $groupInfo.Count - 1; $i += 1) {
            $key = ([string]$groupInfo[$i]).Trim()
            $value = ([string]$groupInfo[$i + 1]).Trim()
            switch ($key) {
                "name" {
                    $groups += 1
                }
                "consumers" {
                    $consumers += [int](Convert-ToInt64OrZero -Value $value)
                }
                "pending" {
                    $pending += [int](Convert-ToInt64OrZero -Value $value)
                }
                "lag" {
                    $lag += [int](Convert-ToInt64OrZero -Value $value)
                }
            }
        }
    }

    return [PSCustomObject]@{
        Stream    = $Stream
        Type      = $streamType
        Length    = $length
        Groups    = $groups
        Consumers = $consumers
        Pending   = $pending
        Lag       = $lag
        Backlog   = $pending + $lag
        IsDLQ     = ($Stream -like "*dlq*")
    }
}

function Scan-RedisKeys {
    param(
        [string]$Pattern,
        [int]$Limit
    )

    $cursor = "0"
    $keys = New-Object System.Collections.Generic.List[string]
    $iterations = 0

    do {
        $rows = @(Invoke-RedisCli -RedisArguments @("SCAN", $cursor, "MATCH", $Pattern, "COUNT", "100") -AllowFailure)
        if ($rows.Count -eq 0 -or ([string]$rows[0]).StartsWith("WARN:")) {
            return @()
        }

        $cursor = [string]$rows[0]
        if ($rows.Count -gt 1) {
            for ($i = 1; $i -lt $rows.Count; $i++) {
                $key = ([string]$rows[$i]).Trim()
                if ($key -ne "") {
                    [void]$keys.Add($key)
                    if ($keys.Count -ge $Limit) {
                        break
                    }
                }
            }
        }

        $iterations++
    } while ($cursor -ne "0" -and $keys.Count -lt $Limit -and $iterations -lt 100)

    return $keys | Select-Object -Unique -First $Limit
}

function Get-KeyPatternDiagnostics {
    param(
        [string]$Pattern,
        [int]$Limit
    )

    $keys = @(Scan-RedisKeys -Pattern $Pattern -Limit $Limit)
    $expiring = 0
    $persistent = 0
    $missing = 0
    $unknown = 0
    $memoryBytes = [int64]0

    foreach ($key in $keys) {
        $ttl = Get-RedisNumber -RedisArguments @("TTL", $key)
        if ($ttl -gt -1) {
            $expiring += 1
        } elseif ($ttl -eq -1) {
            $persistent += 1
        } elseif ($ttl -eq -2) {
            $missing += 1
        } else {
            $unknown += 1
        }

        $memoryBytes += Get-RedisNumber -RedisArguments @("MEMORY", "USAGE", $key)
    }

    return [PSCustomObject]@{
        Pattern        = $Pattern
        MatchedUpTo    = $keys.Count
        ScanLimit      = $Limit
        Capped         = ($keys.Count -ge $Limit)
        ExpiringTTL    = $expiring
        PersistentTTL  = $persistent
        MissingTTL     = $missing
        UnknownTTL     = $unknown
        SampleMemoryB  = $memoryBytes
    }
}

$appEnv = Get-Setting -Name "APP_ENV"
$allowProdFromEnv = (Get-Setting -Name "ALLOW_PRODUCTION_LOADTEST_REDIS_DIAGNOSTICS" -Default "false").ToLowerInvariant() -eq "true"
if ($appEnv -eq "production" -and -not $AllowProduction -and -not $allowProdFromEnv) {
    throw "Refusing to run Redis load diagnostics against APP_ENV=production. Pass -AllowProduction only for an approved production audit."
}

if ([string]::IsNullOrWhiteSpace($RedisAddr)) {
    $RedisAddr = Get-Setting -Name "REDIS_ADDR" -Default "localhost:6379"
}
if ([string]::IsNullOrWhiteSpace($RedisPassword)) {
    $RedisPassword = Get-Setting -Name "REDIS_PASSWORD"
}
if ($RedisDb -lt 0) {
    $RedisDb = [int](Get-Setting -Name "REDIS_DB" -Default "0")
}
if ([string]::IsNullOrWhiteSpace($ConsumerGroup)) {
    $ConsumerGroup = Get-Setting -Name "LOADTEST_REDIS_CONSUMER_GROUP" -Default "workers"
}
if ($PendingIdleMs -lt 0) {
    $PendingIdleMs = [int](Get-Setting -Name "LOADTEST_REDIS_PENDING_IDLE_MS" -Default "30000")
}
if ($PendingLimit -lt 0) {
    $PendingLimit = [int](Get-Setting -Name "LOADTEST_REDIS_PENDING_LIMIT" -Default "20")
}
if ($ScanCount -lt 0) {
    $ScanCount = [int](Get-Setting -Name "LOADTEST_REDIS_SCAN_COUNT" -Default "100")
}
if ($KeyPatternScanLimit -lt 0) {
    $KeyPatternScanLimit = [int](Get-Setting -Name "LOADTEST_REDIS_KEY_PATTERN_SCAN_LIMIT" -Default "1000")
}

$Streams = Split-SettingList `
    -Explicit $Streams `
    -EnvName "LOADTEST_REDIS_STREAMS" `
    -Default @(
        "stream:jobs:text",
        "stream:jobs:image",
        "stream:jobs:video",
        "stream:jobs:delivery",
        "stream:jobs:provider_poll",
        "stream:jobs:dlq"
    )

$KeyPatterns = Split-SettingList `
    -Explicit $KeyPatterns `
    -EnvName "LOADTEST_REDIS_KEY_PATTERNS" `
    -Default @(
        "rate:vk:user:*",
        "spam:vk:user:*",
        "block:vk:user:*",
        "cooldown:vk:user:*",
        "vk:peer:*:dialog_mode",
        "stream:jobs:*"
    )
foreach ($requiredPattern in @("cooldown:vk:user:*")) {
    if ($KeyPatterns -notcontains $requiredPattern) {
        $KeyPatterns += $requiredPattern
    }
}

if (-not $UseDockerCompose -and -not (Get-Command redis-cli -ErrorAction SilentlyContinue)) {
    Write-Warning "redis-cli was not found in PATH; falling back to docker compose exec redis."
    $UseDockerCompose = $true
}

if ($UseDockerCompose) {
    if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
        throw "Docker is required for -UseDockerCompose fallback."
    }

    if ([string]::IsNullOrWhiteSpace($ComposeProjectName)) {
        $ComposeProjectName = Get-Setting -Name "COMPOSE_PROJECT_NAME"
    }
    if ([string]::IsNullOrWhiteSpace($ComposeProjectName)) {
        $ComposeProjectName = Get-Setting -Name "COMPOSE_NETWORK_NAME" -Default "vk-ai-aggregator-loadtest"
    }
}

$script:Output = New-Object System.Collections.Generic.List[string]

Add-Line "# Redis Load Diagnostics"
Add-Line "Generated: $(Get-Date -Format o)"
Add-Line "Environment: $appEnv"
Add-Line "Mode: $(if ($UseDockerCompose) { "docker-compose service redis" } else { "redis-cli $RedisAddr" })"
Add-Line "Redis DB: $RedisDb"
Add-Line "Consumer group: $ConsumerGroup"
Add-Line "Safety: read-only commands only; key values are not fetched."

$serverInfoLines = @(Invoke-RedisCli -RedisArguments @("INFO", "server") -AllowFailure)
$memoryInfoLines = @(Invoke-RedisCli -RedisArguments @("INFO", "memory") -AllowFailure)
$clientsInfoLines = @(Invoke-RedisCli -RedisArguments @("INFO", "clients") -AllowFailure)
$statsInfoLines = @(Invoke-RedisCli -RedisArguments @("INFO", "stats") -AllowFailure)
$memoryInfo = Parse-RedisInfo -Lines $memoryInfoLines
$statsInfo = Parse-RedisInfo -Lines $statsInfoLines
$clientsInfo = Parse-RedisInfo -Lines $clientsInfoLines
$streamDiagnostics = @($Streams | ForEach-Object { Get-StreamDiagnostics -Stream $_ -Group $ConsumerGroup })
$keyPatternDiagnostics = @($KeyPatterns | ForEach-Object { Get-KeyPatternDiagnostics -Pattern $_ -Limit $KeyPatternScanLimit })
$totalStreamLength = ($streamDiagnostics | Measure-Object -Property Length -Sum).Sum
$totalBacklog = ($streamDiagnostics | Measure-Object -Property Backlog -Sum).Sum
$dlqDepth = ($streamDiagnostics | Where-Object { $_.IsDLQ } | Measure-Object -Property Length -Sum).Sum
if ($null -eq $totalStreamLength) { $totalStreamLength = 0 }
if ($null -eq $totalBacklog) { $totalBacklog = 0 }
if ($null -eq $dlqDepth) { $dlqDepth = 0 }

Add-Section "Connection"
Add-CommandOutput -Lines (Invoke-RedisCli -RedisArguments @("PING"))

Add-Section "Summary"
Add-Line "- Used memory: $($memoryInfo["used_memory_human"])"
Add-Line "- Peak memory: $($memoryInfo["used_memory_peak_human"])"
Add-Line "- Memory fragmentation ratio: $($memoryInfo["mem_fragmentation_ratio"])"
Add-Line "- Connected clients: $($clientsInfo["connected_clients"])"
Add-Line "- Instantaneous ops/sec: $($statsInfo["instantaneous_ops_per_sec"])"
Add-Line "- Total commands processed: $($statsInfo["total_commands_processed"])"
Add-Line "- Rejected connections: $($statsInfo["rejected_connections"])"
Add-Line "- Expired keys: $($statsInfo["expired_keys"])"
Add-Line "- Evicted keys: $($statsInfo["evicted_keys"])"
Add-Line "- Total stream length: $totalStreamLength"
Add-Line "- Total backlog pending+lag: $totalBacklog"
Add-Line "- DLQ depth: $dlqDepth"

Add-Line ""
Add-Line "| Stream | Length | Groups | Consumers | Pending | Lag | Backlog |"
Add-Line "| --- | ---: | ---: | ---: | ---: | ---: | ---: |"
foreach ($row in $streamDiagnostics) {
    Add-Line "| $($row.Stream) | $($row.Length) | $($row.Groups) | $($row.Consumers) | $($row.Pending) | $($row.Lag) | $($row.Backlog) |"
}

Add-Line ""
Add-Line "| Key pattern | Matched up to | Scan limit | Capped | Expiring TTL | Persistent TTL | Sample memory bytes |"
Add-Line "| --- | ---: | ---: | --- | ---: | ---: | ---: |"
foreach ($row in $keyPatternDiagnostics) {
    Add-Line "| $($row.Pattern) | $($row.MatchedUpTo) | $($row.ScanLimit) | $($row.Capped) | $($row.ExpiringTTL) | $($row.PersistentTTL) | $($row.SampleMemoryB) |"
}

Add-Section "Server"
Add-CommandOutput -Lines $serverInfoLines

Add-Section "Memory"
Add-CommandOutput -Lines $memoryInfoLines

Add-Section "Clients"
Add-CommandOutput -Lines $clientsInfoLines

Add-Section "Stats"
Add-CommandOutput -Lines $statsInfoLines

Add-Section "Slowlog"
Add-Line "SLOWLOG LEN"
Add-CommandOutput -Lines (Invoke-RedisCli -RedisArguments @("SLOWLOG", "LEN") -AllowFailure)
Add-Line ""
Add-Line "SLOWLOG GET 10"
Add-CommandOutput -Lines (Invoke-RedisCli -RedisArguments @("SLOWLOG", "GET", "10") -AllowFailure)

Add-Section "Streams And Pending Entries"
foreach ($stream in $Streams) {
    Add-Line ""
    Add-Line "### $stream"
    Add-Line "TYPE"
    $streamType = Get-FirstLine -Lines (Invoke-RedisCli -RedisArguments @("TYPE", $stream) -AllowFailure)
    Add-Line $streamType
    if ($streamType -eq "none" -or $streamType.StartsWith("WARN:")) {
        Add-Line "Stream is absent; skipping XINFO/XPENDING."
        continue
    }
    Add-Line "XLEN"
    Add-CommandOutput -Lines (Invoke-RedisCli -RedisArguments @("XLEN", $stream) -AllowFailure)
    Add-Line "XINFO STREAM"
    Add-CommandOutput -Lines (Invoke-RedisCli -RedisArguments @("XINFO", "STREAM", $stream) -AllowFailure)
    Add-Line "XINFO GROUPS"
    Add-CommandOutput -Lines (Invoke-RedisCli -RedisArguments @("XINFO", "GROUPS", $stream) -AllowFailure)
    if ($stream -ne "stream:jobs:dlq") {
        Add-Line "XPENDING summary for group '$ConsumerGroup'"
        Add-CommandOutput -Lines (Invoke-RedisCli -RedisArguments @("XPENDING", $stream, $ConsumerGroup) -AllowFailure)
        Add-Line "XPENDING idle >= ${PendingIdleMs}ms, limit $PendingLimit"
        Add-CommandOutput -Lines (Invoke-RedisCli -RedisArguments @("XPENDING", $stream, $ConsumerGroup, "IDLE", "$PendingIdleMs", "-", "+", "$PendingLimit") -AllowFailure)
    }
}

Add-Section "Key Samples"
Add-Line "SCAN is used instead of KEYS. Values are not read."
foreach ($pattern in $KeyPatterns) {
    Add-Line ""
    Add-Line "### Pattern: $pattern"
    $keys = @(Scan-RedisKeys -Pattern $pattern -Limit $ScanCount)
    Add-Line "Matched sample count: $($keys.Count) (limit $ScanCount)"
    foreach ($key in $keys) {
        $type = Get-RedisKeyType -Key $key
        Add-Line ""
        Add-Line "- key: $key"
        Add-Line "  type: $type"
        Add-Line "  ttl:"
        Add-CommandOutput -Lines (Invoke-RedisCli -RedisArguments @("TTL", $key) -AllowFailure)
        Add-Line "  memory_usage:"
        Add-CommandOutput -Lines (Invoke-RedisCli -RedisArguments @("MEMORY", "USAGE", $key) -AllowFailure)

        switch ($type) {
            "stream" {
                Add-Line "  xlen:"
                Add-CommandOutput -Lines (Invoke-RedisCli -RedisArguments @("XLEN", $key) -AllowFailure)
            }
            "list" {
                Add-Line "  llen:"
                Add-CommandOutput -Lines (Invoke-RedisCli -RedisArguments @("LLEN", $key) -AllowFailure)
            }
            "set" {
                Add-Line "  scard:"
                Add-CommandOutput -Lines (Invoke-RedisCli -RedisArguments @("SCARD", $key) -AllowFailure)
            }
            "zset" {
                Add-Line "  zcard:"
                Add-CommandOutput -Lines (Invoke-RedisCli -RedisArguments @("ZCARD", $key) -AllowFailure)
            }
            "hash" {
                Add-Line "  hlen:"
                Add-CommandOutput -Lines (Invoke-RedisCli -RedisArguments @("HLEN", $key) -AllowFailure)
            }
            "string" {
                Add-Line "  strlen:"
                Add-CommandOutput -Lines (Invoke-RedisCli -RedisArguments @("STRLEN", $key) -AllowFailure)
            }
        }
    }
}

Add-Section "Backpressure Review Checklist"
Add-Line "- stream XLEN grows continuously while workers are healthy -> producer rate exceeds worker throughput."
Add-Line "- XPENDING grows or idle entries exceed the threshold -> workers crash, hang or cannot ack."
Add-Line "- stream:jobs:dlq grows during mock-provider load -> retry/terminal failure path needs inspection."
Add-Line "- rate:* keys lack TTL under bursts -> anti-spam windows may leak keys."
Add-Line "- cooldown:* keys grow without expiration -> temporary cooldown state may leak keys."
Add-Line "- vk:peer:*:dialog_mode keys lack TTL -> dialog state may survive longer than configured."
Add-Line "- one operation type grows while others drain -> split worker concurrency/pools before adding traffic."

$snapshot = [PSCustomObject]@{
    generated_at              = (Get-Date -Format o)
    app_env                   = $appEnv
    redis_db                  = $RedisDb
    consumer_group            = $ConsumerGroup
    used_memory_human         = $memoryInfo["used_memory_human"]
    used_memory_peak_human    = $memoryInfo["used_memory_peak_human"]
    mem_fragmentation_ratio   = $memoryInfo["mem_fragmentation_ratio"]
    connected_clients         = $clientsInfo["connected_clients"]
    instantaneous_ops_per_sec = $statsInfo["instantaneous_ops_per_sec"]
    total_commands_processed  = $statsInfo["total_commands_processed"]
    rejected_connections      = $statsInfo["rejected_connections"]
    expired_keys              = $statsInfo["expired_keys"]
    evicted_keys              = $statsInfo["evicted_keys"]
    total_stream_length       = $totalStreamLength
    total_backlog             = $totalBacklog
    dlq_depth                 = $dlqDepth
    streams                   = @($streamDiagnostics)
    key_patterns              = @($keyPatternDiagnostics)
}

if (-not [string]::IsNullOrWhiteSpace($OutputFile)) {
    $outputPath = Split-Path -Parent $OutputFile
    if (-not [string]::IsNullOrWhiteSpace($outputPath) -and -not (Test-Path $outputPath)) {
        New-Item -ItemType Directory -Force -Path $outputPath | Out-Null
    }
    $script:Output | Set-Content -LiteralPath $OutputFile -Encoding UTF8
    Write-Host "Redis diagnostics written to $OutputFile"
} else {
    $script:Output | ForEach-Object { Write-Host $_ }
}

if ([string]::IsNullOrWhiteSpace($SnapshotFile) -and -not [string]::IsNullOrWhiteSpace($OutputFile)) {
    $SnapshotFile = [IO.Path]::ChangeExtension($OutputFile, ".snapshot.json")
}
if (-not [string]::IsNullOrWhiteSpace($SnapshotFile)) {
    $snapshotPath = Split-Path -Parent $SnapshotFile
    if (-not [string]::IsNullOrWhiteSpace($snapshotPath) -and -not (Test-Path $snapshotPath)) {
        New-Item -ItemType Directory -Force -Path $snapshotPath | Out-Null
    }
    $snapshot | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $SnapshotFile -Encoding UTF8
    Write-Host "Redis diagnostics snapshot written to $SnapshotFile"
}

Write-Host "Redis diagnostics completed."
