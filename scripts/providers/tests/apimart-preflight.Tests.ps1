# No Pester dependency: run the actual CLI in isolated child processes.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$scriptPath = Join-Path $PSScriptRoot '../apimart-preflight.ps1'
if (-not (Test-Path -LiteralPath $scriptPath)) { throw 'Missing preflight CLI' }
$shellPath = (Get-Process -Id $PID).Path
$testKey = 'fixture' + '-credential-value'

function Invoke-TestCLI {
    param([string[]]$Arguments)
    $start = [Diagnostics.ProcessStartInfo]::new($shellPath)
    $start.UseShellExecute = $false
    $start.CreateNoWindow = $true
    $start.RedirectStandardOutput = $true
    $start.RedirectStandardError = $true
    foreach ($item in @('-NoProfile', '-NonInteractive', '-File', $scriptPath) + $Arguments) {
        $start.ArgumentList.Add($item)
    }
    $start.Environment['APIMART_API_KEY'] = $testKey
    # An unsafe URL must fail before any network call, even with a key present.
    $start.Environment['APIMART_BASE_URL'] = 'https://preflight-invalid.example/v1'
    $process = [Diagnostics.Process]::new()
    $process.StartInfo = $start
    try {
        [void]$process.Start()
        $stdoutTask = $process.StandardOutput.ReadToEndAsync()
        $stderrTask = $process.StandardError.ReadToEndAsync()
        if (-not $process.WaitForExit(60000)) { $process.Kill($true); throw 'CLI timeout' }
        return [pscustomobject]@{ Code = $process.ExitCode; Out = $stdoutTask.GetAwaiter().GetResult(); Err = $stderrTask.GetAwaiter().GetResult() }
    }
    finally { $process.Dispose() }
}

foreach ($case in @(
    @{ Name = 'explicit model IDs required'; Args = @() },
    @{ Name = 'unsafe endpoint blocked'; Args = @('-ModelId', 'fixture-image') },
    @{ Name = 'comma-separated models accepted then endpoint blocked'; Args = @('-ModelId', 'fixture-image,fixture-chat') }
)) {
    $result = Invoke-TestCLI -Arguments $case.Args
    if ($result.Code -eq 0) { throw ('Expected failure: ' + $case.Name) }
    if (($result.Out + $result.Err).Contains($testKey)) { throw 'Credential leaked through CLI' }
    if (($result.Out + $result.Err).Contains('preflight-invalid.example')) { throw 'Raw configuration leaked through CLI' }
    $report = $result.Out | ConvertFrom-Json
    if ($report.status -ne 'blocked' -or $report.b0_complete -ne $false) { throw 'CLI failure report missing' }
    if ($case.Args.Count -gt 0 -and $report.errors -notcontains 'official_https_base_url_required') { throw 'Model selection did not reach safe endpoint validation' }
    Write-Output ('PASS: ' + $case.Name)
}
