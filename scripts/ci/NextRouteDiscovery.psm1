Set-StrictMode -Version Latest

function ConvertTo-NormalizedNextRoute {
    param([Parameter(Mandatory = $true)][string]$Route)

    $normalized = $Route.Trim()
    if ([string]::IsNullOrWhiteSpace($normalized)) {
        $normalized = "/"
    }
    if (-not $normalized.StartsWith("/", [StringComparison]::Ordinal)) {
        $normalized = "/$normalized"
    }
    if ($normalized.Length -gt 1) {
        $normalized = $normalized.TrimEnd("/")
    }
    return $normalized
}

function Get-NextRouteForPage {
    param(
        [Parameter(Mandatory = $true)][string]$AppRoot,
        [Parameter(Mandatory = $true)][System.IO.FileInfo]$Page
    )

    $relativeDirectory = [System.IO.Path]::GetRelativePath($AppRoot, $Page.DirectoryName)
    if ($relativeDirectory -eq ".") {
        return "/"
    }

    $routeSegments = @(
        $relativeDirectory -split '[\\/]' |
            Where-Object {
                -not [string]::IsNullOrWhiteSpace($_) -and
                $_ -notmatch '^\(.*\)$' -and
                $_ -notmatch '^@' -and
                $_ -notmatch '^_'
            }
    )

    if ($routeSegments.Count -eq 0) {
        return "/"
    }
    return "/" + ($routeSegments -join "/")
}

function Test-IsCanonicalNextPage {
    param(
        [Parameter(Mandatory = $true)][string]$AppRoot,
        [Parameter(Mandatory = $true)][System.IO.FileInfo]$Page
    )

    $relativeDirectory = [System.IO.Path]::GetRelativePath($AppRoot, $Page.DirectoryName)
    if ($relativeDirectory -eq ".") {
        return $true
    }

    foreach ($segment in ($relativeDirectory -split '[\\/]')) {
        if ($segment -match '^@' -or $segment -match '^_' -or $segment -match '^\(\.') {
            return $false
        }
    }
    return $true
}

function Resolve-NextAppRoutePage {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)][string]$AppRoot,
        [Parameter(Mandatory = $true)][string]$Route
    )

    if (-not (Test-Path -LiteralPath $AppRoot -PathType Container)) {
        throw "Next.js app root is missing: $AppRoot"
    }

    $resolvedAppRoot = (Resolve-Path -LiteralPath $AppRoot).Path
    $normalizedRoute = ConvertTo-NormalizedNextRoute -Route $Route
    $owners = @(
        Get-ChildItem -LiteralPath $resolvedAppRoot -Recurse -File |
            Where-Object { $_.Name -match '^page\.(?:js|jsx|ts|tsx)$' } |
            Where-Object { Test-IsCanonicalNextPage -AppRoot $resolvedAppRoot -Page $_ } |
            Where-Object {
                (Get-NextRouteForPage -AppRoot $resolvedAppRoot -Page $_) -eq $normalizedRoute
            }
    )

    if ($owners.Count -eq 0) {
        throw "No Next.js page owns route $normalizedRoute under $resolvedAppRoot"
    }
    if ($owners.Count -gt 1) {
        $ownerList = ($owners.FullName | Sort-Object) -join ", "
        throw "Multiple Next.js pages own route $normalizedRoute`: $ownerList"
    }

    return $owners[0].FullName
}

Export-ModuleMember -Function Resolve-NextAppRoutePage
