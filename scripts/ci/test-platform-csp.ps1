[CmdletBinding()]
param()

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest
Import-Module (Join-Path $PSScriptRoot "PlatformCspValidation.psm1") -Force

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "../..")).Path
$source = Get-Content -LiteralPath (Join-Path $repoRoot "web/platform/src/proxy.ts") -Raw

function Assert-Rejected {
    param([string]$Name, [string]$UnsafeSource)

    try {
        Assert-PlatformCspSource -Source $UnsafeSource
    }
    catch {
        if ($_.Exception.Message -notlike "*must not allow unsafe inline/eval execution*") {
            throw
        }
        Write-Host "PASS: $Name"
        return
    }
    throw "${Name}: unsafe CSP was accepted"
}

Assert-Rejected "eval cannot be enabled in production" ($source.Replace('NODE_ENV === "development"', 'NODE_ENV === "production"'))
Assert-Rejected "eval cannot be unconditional" ($source.Replace('process.env.NODE_ENV === "development" ?', 'true ?'))
Assert-Rejected "eval cannot be enabled in the production fallback" ($source.Replace('? " ''unsafe-eval''" : ""', '? " ''unsafe-eval''" : " ''unsafe-eval''"'))
Assert-Rejected "inline script execution is forbidden" ($source + "`nconst extra = `"script-src 'self' 'unsafe-inline'`";")
Assert-Rejected "unconditional inline style elements are forbidden" ($source + "`nconst extra = `"style-src-elem 'self' 'unsafe-inline'`";")
Assert-Rejected "style attribute exception cannot widen to scripts" ($source.Replace("style-src-attr 'unsafe-inline'", "script-src-attr 'unsafe-inline'"))

Assert-PlatformCspSource -Source $source
Write-Host "PASS: development-only runtime support and style attributes are accepted"
Write-Host "platform CSP validation tests OK"
