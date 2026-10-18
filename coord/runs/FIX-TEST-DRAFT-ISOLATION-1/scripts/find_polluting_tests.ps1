# FIX-TEST-DRAFT-ISOLATION-1 (140): deterministically find tests that create
# drafts in the REAL ProjectHistory draft root by running each test in isolation
# from a prebuilt test binary and counting real-root directory deltas.
param(
    [Parameter(Mandatory = $true)][string]$Package,
    [Parameter(Mandatory = $true)][string]$AgentDir,
    [Parameter(Mandatory = $true)][string]$RealRoot,
    [Parameter(Mandatory = $true)][string]$OutJson,
    [string]$GoExe = "C:\Program Files\Go\bin\go.exe",
    [int]$PerTestTimeoutSec = 180
)

$ErrorActionPreference = "Continue"
$outDir = Split-Path -Parent $OutJson
New-Item -ItemType Directory -Force -Path $outDir | Out-Null
$pkgLeaf = ($Package -replace '^\./','') -replace '[\\/]','_'
$binary = Join-Path $outDir ($pkgLeaf + ".test.exe")
$pkgDir = Join-Path $AgentDir (($Package -replace '^\./','') -replace '/','\')

Push-Location $AgentDir
try {
    & $GoExe test -c -o $binary $Package 2>&1 | Out-Null
    if (-not (Test-Path -LiteralPath $binary)) { throw ("failed to build test binary for " + $Package) }
}
finally { Pop-Location }

$tests = @(& $binary "-test.list" ".*" 2>$null | Where-Object { $_ -match '^Test' })
$tests = @($tests | Sort-Object)
Write-Host ("[find] package=" + $Package + " tests=" + $tests.Count + " binary=" + $binary)

$known = @{}
foreach ($d in @(Get-ChildItem -LiteralPath $RealRoot -Directory -ErrorAction SilentlyContinue)) { $known[$d.Name] = $true }
$rows = New-Object System.Collections.ArrayList
$polluters = New-Object System.Collections.ArrayList
$logDir = Join-Path $outDir ("per_test_logs_" + $pkgLeaf)
New-Item -ItemType Directory -Force -Path $logDir | Out-Null

Push-Location $pkgDir
try {
    foreach ($test in $tests) {
        $before = @(Get-ChildItem -LiteralPath $RealRoot -Directory -ErrorAction SilentlyContinue | ForEach-Object { $_.Name })
        $started = Get-Date
        $logFile = Join-Path $logDir ($test + ".log")
        & $binary "-test.run" ("^" + $test + "$") "-test.timeout" ($PerTestTimeoutSec.ToString() + "s") *> $logFile
        $exit = $LASTEXITCODE
        $durationMs = [int]((Get-Date) - $started).TotalMilliseconds
        $after = @(Get-ChildItem -LiteralPath $RealRoot -Directory -ErrorAction SilentlyContinue | ForEach-Object { $_.Name })
        $added = @($after | Where-Object { $before -notcontains $_ })
        [void]$rows.Add([pscustomobject]@{ test = $test; exit = $exit; duration_ms = $durationMs; added_dirs = @($added); added_count = $added.Count })
        if ($added.Count -gt 0) {
            [void]$polluters.Add([pscustomobject]@{ test = $test; added = @($added); exit = $exit })
            Write-Host ("[pollution] " + $test + " added=" + ($added -join ","))
        }
    }
}
finally { Pop-Location }

$result = [ordered]@{
    schema_version = "fix_test_draft_isolation_per_test_finder.v1"
    package = $Package
    agent_dir = $AgentDir
    real_draft_root = $RealRoot
    test_count = $tests.Count
    polluters = @($polluters)
    tests = @($rows)
    captured_at = (Get-Date).ToString("o")
}
$result | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $OutJson -Encoding UTF8
Write-Host ("[find] package=" + $Package + " polluters=" + $polluters.Count + " -> " + $OutJson)
