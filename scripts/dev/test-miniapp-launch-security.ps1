[CmdletBinding()]
param()

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..")
. (Join-Path $PSScriptRoot "_miniapp-common.ps1")

$signedParams = "vk_user_id=777&vk_ts=1&sign=TEST_PLACEHOLDER_SIGNATURE"
$script:openedTarget = ""
$probeDir = Join-Path ([IO.Path]::GetTempPath()) ("vkagg-miniapp-launch-probe-{0}" -f [guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $probeDir -Force | Out-Null

try {
    $output = (& {
        Open-MiniAppBrowserWithoutDisclosure `
            -PublicUrl "https://dev-app.example.test" `
            -LaunchParams $signedParams `
            -BrowserLauncher {
                param([string]$Target)
                $script:openedTarget = $Target
            }
    } *>&1 | Out-String)

    if ($output.Contains($signedParams)) {
        throw "browser helper exposed signed launch params in command output"
    }
    if (-not $script:openedTarget.Contains($signedParams)) {
        throw "browser helper did not pass launch params to the browser launcher"
    }

    foreach ($file in @(Get-ChildItem -LiteralPath $probeDir -File -Recurse -ErrorAction SilentlyContinue)) {
        $content = Get-Content -LiteralPath $file.FullName -Raw -ErrorAction SilentlyContinue
        if ($null -ne $content -and $content.Contains($signedParams)) {
            throw "browser helper persisted signed launch params"
        }
    }

    $dashboardScript = Get-Content -LiteralPath (Join-Path $PSScriptRoot "start-miniapp-cloudflare-dashboard.ps1") -Raw
    if ($dashboardScript -match '(?i)Set-Content[^\r\n]*miniapp-cloudflare-launch\.url' -or $dashboardScript.Contains('Write-Host "Launch URL:')) {
        throw "dashboard helper still persists or prints a signed launch URL"
    }

    Write-Host "Mini App launch disclosure tests OK"
} finally {
    Remove-Item -LiteralPath $probeDir -Recurse -Force -ErrorAction SilentlyContinue
}
