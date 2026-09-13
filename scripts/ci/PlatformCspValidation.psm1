Set-StrictMode -Version Latest

function Assert-PlatformCspSource {
    param([Parameter(Mandatory = $true)][string]$Source)

    # Accept only the existing, explicitly development-gated Next.js support
    # and the CSS attribute directive used by React layouts. All other uses
    # remain forbidden; proxy.test.ts also checks the emitted production CSP.
    $allowedPatterns = @(
        '(?m)^\s*const developmentScriptSource\s*=\s*process\.env\.NODE_ENV === "development" \? " ''unsafe-eval''" : "";\s*$',
        '(?m)^\s*const developmentStyleElements\s*=\s*process\.env\.NODE_ENV === "development" \? "style-src-elem ''self'' ''unsafe-inline''" : null;\s*$',
        '(?m)^\s*"style-src-attr ''unsafe-inline''",\s*$'
    )
    $remainingSource = $Source
    foreach ($pattern in $allowedPatterns) {
        $remainingSource = [regex]::Replace($remainingSource, $pattern, "")
    }
    if ($remainingSource -match "unsafe-inline|unsafe-eval") {
        throw "platform nonce proxy must not allow unsafe inline/eval execution"
    }
}

Export-ModuleMember -Function Assert-PlatformCspSource
