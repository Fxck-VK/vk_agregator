<#
.SYNOPSIS
Read APIMart metadata for explicitly selected model IDs. No generation calls.
.DESCRIPTION
Uses APIMART_API_KEY and the optional APIMART_BASE_URL from the process
environment. Never loads .env files or writes runtime settings or tariffs.
ExpectedPricingPath is local reference evidence, not proof of the key's price.
Requires the repository's Go toolchain. stdout contains only a safe JSON report.
#>
[CmdletBinding()]
param(
    [string[]]$ModelId = @(),
    [string]$ExpectedPricingPath = ''
)

$ErrorActionPreference = 'Stop'
$PSNativeCommandUseErrorActionPreference = $false

function Stop-Preflight {
    param([string]$Code)
    @{ version = 1; status = 'blocked'; b0_complete = $false; errors = @($Code) } | ConvertTo-Json -Compress
    exit 1
}

try {
    $selected = @($ModelId | ForEach-Object { $_.Split(',') } | ForEach-Object { $_.Trim() })
    if ($selected.Count -eq 0 -or @($selected | Where-Object { $_ -eq '' }).Count -gt 0) {
        Stop-Preflight 'explicit_model_ids_required'
    }
    $goCommand = Get-Command go -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($null -eq $goCommand) { Stop-Preflight 'go_toolchain_required' }
    $arguments = @('run', (Join-Path $PSScriptRoot 'preflight/main.go'), '--models', ($selected -join ','))
    if ($ExpectedPricingPath -ne '') { $arguments += @('--expected-pricing', $ExpectedPricingPath) }
    # Never pass credentials as command-line arguments. Suppress compiler/native
    # diagnostics; API failures are normalized inside the Go reader.
    $result = & $goCommand.Source @arguments 2>$null
    $code = $LASTEXITCODE
    if ($null -eq $result -or [string]::IsNullOrWhiteSpace(($result -join "`n"))) {
        Stop-Preflight 'preflight_runner_failed'
    }
    $result | Write-Output
    exit $code
}
catch {
    Stop-Preflight 'preflight_runner_failed'
}
