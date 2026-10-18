# FIX-TEST-DRAFT-ISOLATION-1 (140) helper v2: attribute draft-root pollution to
# the exact running test by streaming `go test -v` output (tail polling) and
# polling the real ProjectHistory draft root for newly created draft dirs.
param(
    [Parameter(Mandatory = $true)][string]$Package,
    [Parameter(Mandatory = $true)][string]$AgentDir,
    [Parameter(Mandatory = $true)][string]$RealRoot,
    [Parameter(Mandatory = $true)][string]$OutJson,
    [string]$GoExe = "C:\Program Files\Go\bin\go.exe"
)

$ErrorActionPreference = "Continue"
$outDir = Split-Path -Parent $OutJson
New-Item -ItemType Directory -Force -Path $outDir | Out-Null
$stdoutFile = Join-Path $outDir ("go_test_verbose_" + [System.IO.Path]::GetFileNameWithoutExtension($OutJson) + ".log")
$stderrFile = Join-Path $outDir ("go_test_verbose_" + [System.IO.Path]::GetFileNameWithoutExtension($OutJson) + ".err.log")
Remove-Item $stdoutFile, $stderrFile -Force -ErrorAction SilentlyContinue

$known = @{}
foreach ($d in @(Get-ChildItem -LiteralPath $RealRoot -Directory -ErrorAction SilentlyContinue)) { $known[$d.Name] = $true }
$events = New-Object System.Collections.ArrayList
$lastRun = "<before-first-test>"
$lastRunAt = (Get-Date).ToString("o")

$job = Start-Job -ScriptBlock {
    param($GoExe, $AgentDir, $Package, $stdoutFile, $stderrFile)
    Set-Location $AgentDir
    & $GoExe test -v -count=1 $Package *> $stdoutFile
    $code = $LASTEXITCODE
    if ($code -ne 0) { & $GoExe test -v -count=1 $Package *> $stderrFile }
    return $code
} -ArgumentList $GoExe, $AgentDir, $Package, $stdoutFile, $stderrFile

try {
    $poll = 0
    while ($job.State -eq "Running" -or $job.State -eq "NotStarted") {
        $poll++
        if (Test-Path -LiteralPath $stdoutFile) {
            $tail = @(Get-Content -LiteralPath $stdoutFile -Tail 400 -ErrorAction SilentlyContinue)
            foreach ($line in $tail) {
                if ($line -match '^\s*=== RUN\s+(\S+)') {
                    if ($Matches[1] -ne $lastRun) {
                        $lastRun = $Matches[1]
                        $lastRunAt = (Get-Date).ToString("o")
                    }
                }
            }
        }
        foreach ($d in @(Get-ChildItem -LiteralPath $RealRoot -Directory -ErrorAction SilentlyContinue)) {
            if (-not $known.ContainsKey($d.Name)) {
                $known[$d.Name] = $true
                [void]$events.Add([pscustomobject]@{ dir = $d.Name; attributed_test = $lastRun; last_run_at = $lastRunAt; detected_at = (Get-Date).ToString("o") })
                Write-Host ("[pollution] " + $d.Name + " <- " + $lastRun)
            }
        }
        Start-Sleep -Milliseconds 80
    }
    Wait-Job $job | Out-Null
    $exitCode = Receive-Job $job
    $result = [ordered]@{
        schema_version    = "fix_test_draft_isolation_pollution_map.v2"
        package           = $Package
        agent_dir         = $AgentDir
        real_draft_root   = $RealRoot
        go_test_exit_code = $exitCode
        events            = @($events)
        verbose_log       = $stdoutFile
        captured_at       = (Get-Date).ToString("o")
    }
    $result | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $OutJson -Encoding UTF8
    Write-Host ("[map] package=" + $Package + " exit=" + $exitCode + " events=" + $events.Count + " -> " + $OutJson)
}
catch {
    Write-Host ("[map][error] " + $_.Exception.Message)
}
finally {
    Remove-Job $job -Force -ErrorAction SilentlyContinue
}
