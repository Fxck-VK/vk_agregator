[CmdletBinding()]
param()
$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "../..")).Path
Import-Module (Join-Path $PSScriptRoot "BashRuntime.psm1") -Force
Push-Location $repoRoot
try {
    Assert-CIBashEnvironment
    & node --test scripts/ci/release-contract.test.mjs
    if ($LASTEXITCODE -ne 0) { throw "Release contract mutation tests failed" }
    & node scripts/ci/check-release-contract.mjs
    if ($LASTEXITCODE -ne 0) { throw "Release source contract failed" }
    foreach ($name in @(
        "verify-release-images", "dev-web-auth", "retire-miniapp-env", "browser-auth-secret",
        "title-worker-rollout", "dev-env", "dev-email", "dev-google", "dev-yandex",
        "dev-legal", "prod-env", "env-parity", "assemble-env-parts",
        "docker-login-retry", "compose-pull-retry"
    )) {
        $watch = [Diagnostics.Stopwatch]::StartNew()
        $result = Invoke-CIBashScript -RepositoryRoot $repoRoot -ScriptPath "scripts/deploy/test-$name.sh"
        $result.Output | Out-Host
        if ($result.ExitCode -ne 0) { throw "Release regression failed: $name" }
        Write-Host ("Release regression {0}: {1:N1}s" -f $name, $watch.Elapsed.TotalSeconds)
    }
} finally { Pop-Location }
