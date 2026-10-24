Set-StrictMode -Version Latest

. (Join-Path $PSScriptRoot "_bot-common.ps1")

function Import-DevDotEnvFile {
    param([Parameter(Mandatory = $true)][string]$Path)

    if (-not (Test-Path -LiteralPath $Path)) {
        return
    }

    foreach ($line in Get-Content -LiteralPath $Path) {
        $trim = $line.Trim()
        if ($trim.Length -eq 0 -or $trim.StartsWith("#") -or -not $trim.Contains("=")) {
            continue
        }
        $idx = $trim.IndexOf("=")
        $key = $trim.Substring(0, $idx).Trim()
        $value = $trim.Substring($idx + 1).Trim()
        if ($key.StartsWith('$env:')) {
            $key = $key.Substring(5)
        }
        if ($value.Length -ge 2 -and (([int][char]$value[0] -eq 34 -and [int][char]$value[$value.Length - 1] -eq 34) -or ([int][char]$value[0] -eq 39 -and [int][char]$value[$value.Length - 1] -eq 39))) {
            $value = $value.Substring(1, $value.Length - 2)
        }
        if ($key -match "^[A-Za-z_][A-Za-z0-9_]*$") {
            Set-Item -Path ("Env:\" + $key) -Value $value
        }
    }
}

function Import-DevEnv {
    param([Parameter(Mandatory = $true)][string]$Root)

    Import-DevDotEnvFile -Path (Join-Path $Root ".env")
    $overlay = Join-Path $Root ".env.ps1"
    if (Test-Path -LiteralPath $overlay) {
        Import-DevDotEnvFile -Path $overlay
    }
}
