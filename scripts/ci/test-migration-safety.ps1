[CmdletBinding()]
param()

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "../..")).Path
$tempParent = [IO.Path]::GetFullPath([IO.Path]::GetTempPath())
$tempRoot = Join-Path $tempParent ("migration-safety-" + [guid]::NewGuid().ToString("N"))
$pwsh = Join-Path $PSHOME $(if ($IsWindows) { "pwsh.exe" } else { "pwsh" })
$bash = if ($IsWindows -and (Test-Path 'C:/Program Files/Git/bin/bash.exe')) {
    'C:/Program Files/Git/bin/bash.exe'
} else { (Get-Command bash -ErrorAction Stop).Source }
$utf8 = [Text.UTF8Encoding]::new($false)

try {
    $migrations = Join-Path $tempRoot "migrations"
    New-Item -ItemType Directory -Path $migrations -Force | Out-Null
    $envFile = Join-Path $tempRoot "test.env"
    [IO.File]::WriteAllText($envFile, "APP_ENV=development`n", $utf8)
    $reviewedName = "000052_text_model_pricing.up.sql"
    $reviewedSql = [IO.File]::ReadAllText((Join-Path $repoRoot "migrations/$reviewedName")).Replace("`r`n", "`n")
    $cases = @(
        @{ Name = "additive"; File = "000001_test.up.sql"; SQL = "CREATE TABLE test (id integer);`n"; Exit = 0 },
        @{ Name = "reviewed constraint replacement"; File = $reviewedName; SQL = $reviewedSql; Exit = 0 },
        @{ Name = "modified reviewed migration"; File = $reviewedName; SQL = $reviewedSql + "DROP TABLE users;`n"; Exit = 1 },
        @{ Name = "review does not permit another migration"; File = $reviewedName; SQL = $reviewedSql; ExtraSQL = "DROP TABLE users;`n"; Exit = 1 },
        @{ Name = "reviewed content under another name"; File = "000053_test.up.sql"; SQL = $reviewedSql; Exit = 1 },
        @{ Name = "unreviewed constraint removal"; File = "000053_test.up.sql"; SQL = "ALTER TABLE prices DROP CONSTRAINT prices_unique;`n"; Exit = 1 },
        @{ Name = "table removal"; File = "000053_test.up.sql"; SQL = "DROP TABLE users;`n"; Exit = 1 },
        @{ Name = "column removal"; File = "000053_test.up.sql"; SQL = "ALTER TABLE users DROP COLUMN balance;`n"; Exit = 1 },
        @{ Name = "row deletion"; File = "000053_test.up.sql"; SQL = "DELETE FROM users;`n"; Exit = 1 },
        @{ Name = "truncate"; File = "000053_test.up.sql"; SQL = "TRUNCATE users;`n"; Exit = 1 }
    )
    foreach ($case in $cases) {
        $fixture = Join-Path $migrations $case.File
        [IO.File]::WriteAllText($fixture, $case.SQL, $utf8)
        $extraFixture = Join-Path $migrations "000099_unreviewed.up.sql"
        if ($case.ContainsKey('ExtraSQL')) { [IO.File]::WriteAllText($extraFixture, $case.ExtraSQL, $utf8) }
        foreach ($runner in @("PowerShell", "Bash")) {
            if ($runner -eq "PowerShell") {
                $output = & $pwsh -NoProfile -File (Join-Path $repoRoot "scripts/deploy/check-migrations-safe.ps1") -EnvFile $envFile -MigrationsDir $migrations 2>&1
            } else {
                $output = & $bash -l (Join-Path $repoRoot "scripts/deploy/check-migrations-safe.sh").Replace('\', '/') --env-file $envFile.Replace('\', '/') --migrations-dir $migrations.Replace('\', '/') 2>&1
            }
            if ($LASTEXITCODE -ne $case.Exit) {
                throw "$runner / $($case.Name): expected exit $($case.Exit), got $LASTEXITCODE`n$($output -join "`n")"
            }
            $expectedMessage = if ($case.Exit -eq 0) { 'Migration safety check OK' } else { 'Destructive migration patterns detected' }
            if (($output -join "`n") -notmatch $expectedMessage) {
                throw "$runner / $($case.Name): guard did not report the expected decision`n$($output -join "`n")"
            }
            Write-Host "PASS $runner / $($case.Name)"
        }
        Remove-Item -LiteralPath $fixture
        if (Test-Path -LiteralPath $extraFixture) { Remove-Item -LiteralPath $extraFixture }
    }
} finally {
    $resolvedTemp = [IO.Path]::GetFullPath($tempRoot)
    if (-not $resolvedTemp.StartsWith($tempParent, [StringComparison]::OrdinalIgnoreCase) -or
        [IO.Path]::GetFileName($resolvedTemp) -notlike 'migration-safety-*') {
        throw "Refusing cleanup outside the migration test directory"
    }
    if (Test-Path -LiteralPath $resolvedTemp) { Remove-Item -LiteralPath $resolvedTemp -Recurse -Force }
}

Write-Host "Migration safety regression tests passed"
