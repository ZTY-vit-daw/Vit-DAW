# FIX-TEST-DRAFT-ISOLATION-1 (140) helper: attribute draft-root pollution to the
# exact running test by streaming `go test -v` output and polling the real
# ProjectHistory draft root for newly created draft dirs.
#
# Usage:
#   powershell -NoProfile -File map_polluting_tests.ps1 -Package ./internal/chat `
#     -AgentDir D:\Vit_DAW_wt_testiso1\agent `
#     -RealRoot C:\Users\timoz\AppData\Local\Vit\ProjectHistory\drafts `
#     -OutJson <run>\evidence\chat_pollution_map.json

param(
    [Parameter(Mandatory = $true)][string]$Package,
    [Parameter(Mandatory = $true)][string]$AgentDir,
    [Parameter(Mandatory = $true)][string]$RealRoot,
    [Parameter(Mandatory = $true)][string]$OutJson,
    [string]$GoExe = "C:\Program Files\Go\bin\go.exe"
)

$ErrorActionPreference = "Stop"
$outDir = Split-Path -Parent $OutJson
New-Item -ItemType Directory -Force -Path $outDir | Out-Null
$stdoutFile = Join-Path $outDir ("go_test_verbose_" + [System.IO.Path]::GetFileNameWithoutExtension($OutJson) + ".log")
$stderrFile = Join-Path $outDir ("go_test_verbose_" + [System.IO.Path]::GetFileNameWithoutExtension($OutJson) + ".err.log")

$known = @{}
foreach ($d in @(Get-ChildItem -LiteralPath $RealRoot -Directory -ErrorAction SilentlyContinue)) { $known[$d.Name] = $true }
$events = New-Object System.Collections.Generic.List[object]
$currentTest = "<before-first-test>"
$offset = 0
$reader = $null

$proc = Start-Process -FilePath $GoExe -ArgumentList @("test", "-v", "-count=1", $Package) -WorkingDirectory $AgentDir -PassThru -WindowStyle Hidden -RedirectStandardOutput $stdoutFile -RedirectStandardError $stderrFile
try {
    while (-not $proc.HasExited) {
        if (Test-Path -LiteralPath $stdoutFile) {
            if ($null -eq $reader) {
                $stream = [System.IO.File]::Open($stdoutFile, [System.IO.FileMode]::Open, [System.IO.FileAccess]::Read, [System.IO.FileShare]::ReadWrite)
                $reader = New-Object System.IO.StreamReader($stream)
            }
            $chunk = $reader.ReadToEnd()
            if ($chunk) {
                foreach ($line in ($chunk -split "`r?`n")) {
                    if ($line -match '^\s*=== RUN\s+(\S+)') { $currentTest = $Matches[1] }
                }
            }
        }
        foreach ($d in @(Get-ChildItem -LiteralPath $RealRoot -Directory -ErrorAction SilentlyContinue)) {
            if (-not $known.ContainsKey($d.Name)) {
                $known[$d.Name] = $true
                $events.Add([pscustomobject]@{ dir = $d.Name; attributed_test = $currentTest; detected_at = (Get-Date).ToString("o") })
                Write-Host ("[pollution] " + $d.Name + " <- " + $currentTest)
            }
        }
        Start-Sleep -Milliseconds 80
    }
}
finally {
    if ($null -ne $reader) { $reader.Dispose() }
    $exitCode = $proc.ExitCode
    $result = [ordered]@{
        schema_version      = "fix_test_draft_isolation_pollution_map.v1"
        package             = $Package
        agent_dir           = $AgentDir
        real_draft_root     = $RealRoot
        go_test_exit_code   = $exitCode
        events              = @($events)
        verbose_log         = $stdoutFile
        stderr_log          = $stderrFile
        captured_at         = (Get-Date).ToString("o")
    }
    $result | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $OutJson -Encoding UTF8
    Write-Host ("[map] package=" + $Package + " exit=" + $exitCode + " events=" + $events.Count + " -> " + $OutJson)
}
