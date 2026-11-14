# FIX-RESTARTAGENT-ENV-1 zero-stack forensics experiments (E1/E2/E3).
# Purpose: establish empirically, on THIS machine's PowerShell, the environment
# inheritance mechanics of the dev_agent_smoke.ps1 -RestartAgent chain:
#   E1 = boundary B (script process -> Start-Process child)   [anchor: dev_agent_smoke.ps1:2012]
#   E2 = boundary A+B (caller PS -> powershell.exe -File -> Start-Process grandchild)
#   E3 = reuse-path loss face  [anchor: dev_agent_smoke.ps1:2024-2037] and restart pickup
# Zero kernel / zero agent / zero UI processes touched: surrogates are cmd.exe
# and powershell.exe children only. Evidence files land in $OutDir.
param([Parameter(Mandatory = $true)][string]$OutDir)
$ErrorActionPreference = "Stop"
New-Item -ItemType Directory -Path $OutDir -Force | Out-Null

$results = [ordered]@{}
$results.ps_version = $PSVersionTable.PSVersion.ToString()
$results.ps_edition = [string]$PSVersionTable.PSEdition
$results.out_dir = $OutDir

# ---------- E1: boundary B — set env in THIS PS process, Start-Process child ----------
$probeFile1 = Join-Path $OutDir "e1_child_env.txt"
Remove-Item -LiteralPath $probeFile1 -ErrorAction SilentlyContinue
$env:VIT_ENV_PROBE_E1 = "e1_passthrough_check"
try {
    Start-Process -FilePath $env:ComSpec -ArgumentList "/c", ("set > """ + $probeFile1 + """") -WindowStyle Hidden -Wait
    $e1 = Get-Content -LiteralPath $probeFile1 -Raw
    $results.E1_startProcess_child_has_set_env = [bool]($e1 -match "VIT_ENV_PROBE_E1=e1_passthrough_check")
    $results.E1_evidence_file = $probeFile1
}
finally {
    Remove-Item -LiteralPath "env:VIT_ENV_PROBE_E1" -ErrorAction SilentlyContinue
}

# ---------- E2: boundary A + B — outer PS sets env, powershell.exe -File inner (no env writes), grandchild captures ----------
$innerScript = Join-Path $OutDir "e2_inner.ps1"
Set-Content -LiteralPath $innerScript -Encoding UTF8 -Value @(
    'param([string]$OutDir2)',
    '$p = Join-Path $OutDir2 "e2_grandchild_env.txt"',
    'Remove-Item -LiteralPath $p -ErrorAction SilentlyContinue',
    '# inner script deliberately does NOT touch any env var',
    'Start-Process -FilePath $env:ComSpec -ArgumentList "/c", ("set > """ + $p + """") -WindowStyle Hidden -Wait'
)
Remove-Item -LiteralPath (Join-Path $OutDir "e2_grandchild_env.txt") -ErrorAction SilentlyContinue
$env:VIT_ENV_PROBE_E2 = "e2_chain_check"
try {
    # boundary A: this PS process -> powershell.exe child (dev_agent_smoke.ps1 is invoked exactly this way per AGENTS.md section 1/6)
    & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $innerScript -OutDir2 $OutDir
    $e2 = Get-Content -LiteralPath (Join-Path $OutDir "e2_grandchild_env.txt") -Raw
    $results.E2_powershellChild_chain_carries_outer_env = [bool]($e2 -match "VIT_ENV_PROBE_E2=e2_chain_check")
    $results.E2_evidence_file = (Join-Path $OutDir "e2_grandchild_env.txt")
}
finally {
    Remove-Item -LiteralPath "env:VIT_ENV_PROBE_E2" -ErrorAction SilentlyContinue
}

# ---------- E3: reuse-path loss face + restart pickup ----------
# Surrogate "agent": snapshots its env to a file, then stays alive (like a listening VitAgent).
$surrogateScript = Join-Path $OutDir "e3_surrogate.ps1"
Set-Content -LiteralPath $surrogateScript -Encoding UTF8 -Value @(
    'param([string]$OutPath, [int]$HoldSeconds = 10)',
    '$rows = Get-ChildItem env: | ForEach-Object { "$($_.Name)=$($_.Value)" }',
    '$rows | Set-Content -LiteralPath $OutPath -Encoding UTF8',
    'Start-Sleep -Seconds $HoldSeconds'
)
$env:VIT_ENV_PROBE_ORIG = "orig_value"
$snap1 = Join-Path $OutDir "e3_surrogate_env_snapshot1.txt"
Remove-Item -LiteralPath $snap1 -ErrorAction SilentlyContinue
$surrogate = Start-Process -FilePath "powershell.exe" -ArgumentList @("-NoProfile", "-ExecutionPolicy", "Bypass", "-File", $surrogateScript, "-OutPath", $snap1, "-HoldSeconds", "10") -WindowStyle Hidden -PassThru
Start-Sleep -Seconds 3   # surrogate has snapshotted by now and is "listening"

# The caller session now sets a NEW env var and REUSES the running surrogate
# (models: env set in session; agent already up; script run WITHOUT -RestartAgent
#  -> dev_agent_smoke.ps1 :2024-2037 keeps the existing agent).
$env:VIT_ENV_PROBE_NEWVAR = "new_var_after_start"
try {
    Wait-Process -Id $surrogate.Id -Timeout 30
    $snap1Content = Get-Content -LiteralPath $snap1 -Raw
    $results.E3_reused_running_process_has_orig_env = [bool]($snap1Content -match "VIT_ENV_PROBE_ORIG=orig_value")
    $results.E3_reused_running_process_LACKS_new_env = (-not [bool]($snap1Content -match "VIT_ENV_PROBE_NEWVAR=new_var_after_start"))
    $results.E3_evidence_file = $snap1

    # E3b: same session STOPS nothing needed (surrogate exited) and RESTARTS:
    # a fresh Start-Process from the env-carrying session picks up the new var
    # (models dev_agent_smoke.ps1 -RestartAgent: stop :1811-1837, start :2012).
    $snap2 = Join-Path $OutDir "e3_surrogate_env_snapshot2.txt"
    Remove-Item -LiteralPath $snap2 -ErrorAction SilentlyContinue
    $surrogate2 = Start-Process -FilePath "powershell.exe" -ArgumentList @("-NoProfile", "-ExecutionPolicy", "Bypass", "-File", $surrogateScript, "-OutPath", $snap2, "-HoldSeconds", "3") -WindowStyle Hidden -PassThru
    Wait-Process -Id $surrogate2.Id -Timeout 30
    $snap2Content = Get-Content -LiteralPath $snap2 -Raw
    $results.E3b_restarted_process_picks_up_session_env = [bool]($snap2Content -match "VIT_ENV_PROBE_NEWVAR=new_var_after_start")
    $results.E3b_restarted_process_still_has_orig_env = [bool]($snap2Content -match "VIT_ENV_PROBE_ORIG=orig_value")
    $results.E3b_evidence_file = $snap2
}
finally {
    Remove-Item -LiteralPath "env:VIT_ENV_PROBE_ORIG" -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath "env:VIT_ENV_PROBE_NEWVAR" -ErrorAction SilentlyContinue
}

# ---------- verdict ----------
$expected = @(
    @{ k = "E1_startProcess_child_has_set_env"; v = $true },
    @{ k = "E2_powershellChild_chain_carries_outer_env"; v = $true },
    @{ k = "E3_reused_running_process_has_orig_env"; v = $true },
    @{ k = "E3_reused_running_process_LACKS_new_env"; v = $true },
    @{ k = "E3b_restarted_process_picks_up_session_env"; v = $true },
    @{ k = "E3b_restarted_process_still_has_orig_env"; v = $true }
)
$failed = @()
foreach ($e in $expected) {
    if ($results[$e.k] -ne $e.v) { $failed += $e.k }
}
$results.failed_assertions = $failed
$results.verdict = if ($failed.Count -eq 0) { "ALL_PASS" } else { "FAIL" }

$outJson = Join-Path $OutDir "env_inheritance_experiments_result.json"
$results | ConvertTo-Json | Set-Content -LiteralPath $outJson -Encoding UTF8
Write-Host ("VERDICT: " + $results.verdict)
Write-Host ("result json: " + $outJson)
if ($results.verdict -ne "ALL_PASS") { exit 1 }
exit 0
