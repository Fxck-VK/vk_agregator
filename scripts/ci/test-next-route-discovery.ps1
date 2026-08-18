[CmdletBinding()]
param()

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$modulePath = Join-Path $PSScriptRoot "NextRouteDiscovery.psm1"
Import-Module $modulePath -Force

function Assert-Equal {
    param(
        [Parameter(Mandatory = $true)]$Expected,
        [Parameter(Mandatory = $true)]$Actual,
        [Parameter(Mandatory = $true)][string]$Description
    )

    if ($Expected -ne $Actual) {
        throw "$Description expected=[$Expected] actual=[$Actual]"
    }
}

function Assert-ThrowsLike {
    param(
        [Parameter(Mandatory = $true)][scriptblock]$Action,
        [Parameter(Mandatory = $true)][string]$Pattern,
        [Parameter(Mandatory = $true)][string]$Description
    )

    try {
        & $Action
    } catch {
        if ($_.Exception.Message -notlike $Pattern) {
            throw "$Description threw an unexpected error: $($_.Exception.Message)"
        }
        return
    }
    throw "$Description did not throw"
}

$tempRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("next-route-discovery-" + [guid]::NewGuid().ToString("N"))
$appRoot = Join-Path $tempRoot "app"

try {
    $publicRoot = Join-Path $appRoot "(public)"
    New-Item -ItemType Directory -Path $publicRoot -Force | Out-Null
    Set-Content -LiteralPath (Join-Path $publicRoot "page.tsx") -Value "export default function Page() {}"

    $resolved = Resolve-NextAppRoutePage -AppRoot $appRoot -Route "/"
    Assert-Equal (Join-Path $publicRoot "page.tsx") $resolved "route group root"

    Remove-Item -LiteralPath $publicRoot -Recurse -Force
    $relocatedRoot = Join-Path $appRoot "(marketing)\(home)"
    New-Item -ItemType Directory -Path $relocatedRoot -Force | Out-Null
    Set-Content -LiteralPath (Join-Path $relocatedRoot "page.tsx") -Value "export default function Page() {}"
    $resolved = Resolve-NextAppRoutePage -AppRoot $appRoot -Route "/"
    Assert-Equal (Join-Path $relocatedRoot "page.tsx") $resolved "relocated route group root"

    $modelsRoot = Join-Path $appRoot "(catalog)\models"
    New-Item -ItemType Directory -Path $modelsRoot -Force | Out-Null
    Set-Content -LiteralPath (Join-Path $modelsRoot "page.tsx") -Value "export default function Page() {}"
    $resolved = Resolve-NextAppRoutePage -AppRoot $appRoot -Route "/models/"
    Assert-Equal (Join-Path $modelsRoot "page.tsx") $resolved "nested route"

    $parallelRoot = Join-Path $appRoot "@modal"
    New-Item -ItemType Directory -Path $parallelRoot -Force | Out-Null
    Set-Content -LiteralPath (Join-Path $parallelRoot "page.tsx") -Value "export default function ModalPage() {}"
    $resolved = Resolve-NextAppRoutePage -AppRoot $appRoot -Route "/"
    Assert-Equal (Join-Path $relocatedRoot "page.tsx") $resolved "parallel route is not the canonical owner"

    $privateRoot = Join-Path $appRoot "_components"
    New-Item -ItemType Directory -Path $privateRoot -Force | Out-Null
    Set-Content -LiteralPath (Join-Path $privateRoot "page.tsx") -Value "export default function PrivatePage() {}"
    $resolved = Resolve-NextAppRoutePage -AppRoot $appRoot -Route "/"
    Assert-Equal (Join-Path $relocatedRoot "page.tsx") $resolved "private folder is not routable"

    Assert-ThrowsLike {
        Resolve-NextAppRoutePage -AppRoot $appRoot -Route "/pricing"
    } "*No Next.js page owns route /pricing*" "missing route"

    Set-Content -LiteralPath (Join-Path $appRoot "page.tsx") -Value "export default function Duplicate() {}"
    Assert-ThrowsLike {
        Resolve-NextAppRoutePage -AppRoot $appRoot -Route "/"
    } "*Multiple Next.js pages own route /*" "duplicate route"
} finally {
    if (Test-Path -LiteralPath $tempRoot) {
        Remove-Item -LiteralPath $tempRoot -Recurse -Force
    }
}

Write-Host "Next.js route discovery tests passed"
