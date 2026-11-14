# restart_agent_env_passthrough_check.ps1
# FIX-RESTARTAGENT-ENV-1 (326): standing assertion tool for the agent-start
# environment contract codified in dev_agent_smoke.ps1 (FIX-RESTARTAGENT-ENV-1
# block above the main agent start) and restart_agent.ps1.
#
# What it proves:
#   Phase 1 (passthrough): probe env vars set in the session that invokes this
#     script survive the full restart chain (this process -> powershell.exe
#     dev_agent_smoke.ps1 -SkipBuild -RestartAgent -> Start-Process VitAgent.exe)
#     and are readable back from the LIVE agent process env block (psutil).
#   Phase 2 (regression): with the probe vars absent from the session, the same
#     restart produces an agent WITHOUT them and dev_agent_smoke.ps1 exits 0
#     exactly as before -- no-var behavior zero change.
#
# Safety / discipline:
#   - Foreign-stack guard: refuses to run (exit 2) unless the runtime stack is
#     idle (ports 7878/5555/5556/8787/18787 free, no VitApp/VitAgent/VspHub/
#     Godot process) -- never reuses or kills a foreign stack (SMOKE-STACK-
#     GUARD-1 discipline; the CALLER is responsible for the coord/resources
#     occupancy registration before invoking this tool).
#   - AGENTS.md section 10 hygiene: snapshots + restores
#     %APPDATA%\Vit-DAW\global_project_config.json with sha256 before/after.
#   - Zero agent source / zero kernel source involvement: the agent binary is
#     only started/stopped (its sha256 is recorded for the receipt).
#   - All run artifacts go to -ArtifactsDir (default under $env:TEMP).
#
# Exit codes: 0 all pass | 1 usage/error | 2 foreign stack present | 3 phase 1
# assertion failed | 4 phase 2 regression failed | 5 dev_agent_smoke exit != 0 |
# 6 teardown or config restore failed.
param(
    [string]$RepoRoot = "",
    [int]$AgentPort = 7878,
    [string[]]$ProbeVars = @("VIT_PLUGIN_CONTROL_EVAL_SEAM", "VIT_PLUGIN_CONTROL_EVAL_SEAM_BARE_WRITE"),
    [string]$ProbeValue = "1",
    [string]$ArtifactsDir = "",
    # Fallback: pass -StartKernel through to dev_agent_smoke.ps1 if the agent
    # /agent/state gate requires the kernel in the current build.
    [switch]$StartKernel
)
Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($RepoRoot)) {
    $RepoRoot = Split-Path -Parent (Split-Path -Parent $PSCommandPath)
}
if ([string]::IsNullOrWhiteSpace($ArtifactsDir)) {
    $ArtifactsDir = Join-Path $env:TEMP ("restart_agent_env_check_" + (Get-Date -Format "yyyyMMdd_HHmmss"))
}
New-Item -ItemType Directory -Path $ArtifactsDir -Force | Out-Null
$SmokeScript = Join-Path $RepoRoot "scripts\dev_agent_smoke.ps1"
$AgentExe = Join-Path $RepoRoot "agent\bin\VitAgent.exe"

function Write-Step([string]$m) { Write-Host ("[restart-env-check] " + $m) }
function Write-Fail([string]$m) { Write-Host ("[restart-env-check][FAIL] " + $m) }

# ---------- caller-session hygiene: probe vars must be absent to start ----------
foreach ($v in $ProbeVars) {
    $prior = [Environment]::GetEnvironmentVariable($v, "Process")
    if (-not [string]::IsNullOrWhiteSpace($prior)) {
        Write-Fail ("caller session already carries " + $v + "=" + $prior + " -- run this check from a session without the probe vars (phase 2 needs a clean session)")
        exit 1
    }
}

# ---------- foreign-stack guard (never touch a foreign stack) ----------
$guardPorts = @(7878, 5555, 5556, 8787, 18787)
foreach ($p in $guardPorts) {
    $l = Get-NetTCPConnection -LocalPort $p -State Listen -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($null -ne $l) {
        Write-Fail ("foreign stack: port " + $p + " is listening (pid=" + $l.OwningProcess + ") -- idle stack required; do the coord/resources occupancy dance first and clear the stack")
        exit 2
    }
}
$guardProcNames = @("VitApp", "VitAgent", "VspHub", "Godot", "Godot_console")
foreach ($n in $guardProcNames) {
    $procs = Get-Process -Name $n -ErrorAction SilentlyContinue
    if ($procs) {
        Write-Fail ("foreign stack: process " + $n + " running (pid=" + ($procs | Select-Object -First 1 -ExpandProperty Id) + ") -- idle stack required")
        exit 2
    }
}
Write-Step "guard: runtime stack idle (5 ports + 5 process names all clear)"

# ---------- AGENTS section 10 hygiene: snapshot global_project_config.json ----------
$globalConfigPath = Join-Path $env:APPDATA "Vit-DAW\global_project_config.json"
$globalConfigSnapshot = $null
$globalConfigHashBefore = "absent"
if (Test-Path -LiteralPath $globalConfigPath) {
    $globalConfigSnapshot = Join-Path $ArtifactsDir "global_project_config.before.json"
    Copy-Item -LiteralPath $globalConfigPath -Destination $globalConfigSnapshot -Force
    $globalConfigHashBefore = (Get-FileHash -LiteralPath $globalConfigPath -Algorithm SHA256).Hash
}
Write-Step ("global_project_config.json snapshot: hash_before=" + $globalConfigHashBefore)

$agentSha = "absent"
if (Test-Path -LiteralPath $AgentExe) {
    $agentSha = (Get-FileHash -LiteralPath $AgentExe -Algorithm SHA256).Hash
}
Write-Step ("agent binary sha256=" + $agentSha + " (-SkipBuild; binary is not under test, the script chain is)")

# ---------- psutil env reader (python child; same pattern as 322 phase 2b) ----------
$readerPy = Join-Path $ArtifactsDir "read_agent_env.py"
Set-Content -LiteralPath $readerPy -Encoding ASCII -Value @'
import json, sys
import psutil
names = sys.argv[1:]
rows = []
for p in psutil.process_iter(["pid", "name"]):
    try:
        nm = (p.info.get("name") or "").lower()
    except Exception:
        continue
    if nm == "vitagent.exe":
        try:
            env = p.environ()
            rows.append({"pid": p.pid, "vars": dict((n, env.get(n)) for n in names)})
        except Exception as e:
            rows.append({"pid": p.pid, "error": repr(e)})
print(json.dumps(rows))
'@

function Read-AgentEnv {
    param([string]$OutFile)
    & python $readerPy @ProbeVars 1> $OutFile 2> ($OutFile + ".err")
    if ($LASTEXITCODE -ne 0) {
        throw ("psutil env reader failed with exit " + $LASTEXITCODE + " (see " + $OutFile + ".err)")
    }
    return Get-Content -LiteralPath $OutFile -Raw | ConvertFrom-Json
}

# StrictMode-safe property access: the psutil JSON rows only carry "error"
# when the env read failed, and PSCustomObject property misses throw under
# Set-StrictMode -Version Latest (device round 1 failure, 2026-11-14).
function Get-JsonProp {
    param($Object, [string]$Name)
    $p = $Object.PSObject.Properties[$Name]
    if ($null -eq $p) { return $null }
    return $p.Value
}

function Invoke-AgentRestart {
    # One dev_agent_smoke.ps1 restart through a REAL powershell.exe child
    # (boundary A) whose own Start-Process is boundary B.
    $smokeArgs = @("-NoProfile", "-ExecutionPolicy", "Bypass", "-File", $SmokeScript, "-SkipBuild", "-RestartAgent", "-NoChatSmoke")
    if ($StartKernel) { $smokeArgs += "-StartKernel" }
    $smokeOut = Join-Path $ArtifactsDir ("dev_agent_smoke_" + $args[0] + ".log")
    & powershell.exe @smokeArgs *> $smokeOut
    return @{
        ExitCode = $LASTEXITCODE
        Output   = $smokeOut
    }
}

$teardownDone = $false
function Stop-StackStartedByThisCheck {
    # finally-style teardown: stop the agent (and kernel if we started one),
    # then confirm the ports are actually released.
    foreach ($n in @("VitAgent", "VitApp")) {
        Get-Process -Name $n -ErrorAction SilentlyContinue | ForEach-Object {
            Write-Step ("teardown: stopping " + $n + " pid=" + $_.Id)
            Stop-Process -Id $_.Id -Force -ErrorAction SilentlyContinue
        }
    }
    $deadline = (Get-Date).AddSeconds(15)
    while ((Get-Date) -lt $deadline) {
        $remaining = @()
        foreach ($p in @(7878, 5555, 5556)) {
            if (Get-NetTCPConnection -LocalPort $p -State Listen -ErrorAction SilentlyContinue) { $remaining += $p }
        }
        if ($remaining.Count -eq 0) { return $true }
        Start-Sleep -Milliseconds 500
    }
    return $false
}

try {
    # ================= Phase 1: passthrough assertion =================
    Write-Step "phase 1: set probe vars in this session, restart agent, psutil readback"
    foreach ($v in $ProbeVars) {
        Set-Item -LiteralPath ("env:" + $v) -Value $ProbeValue
        Write-Step ("  session env: " + $v + "=" + $ProbeValue)
    }
    $r1 = Invoke-AgentRestart "phase1"
    if ($r1.ExitCode -ne 0) {
        Write-Fail ("dev_agent_smoke.ps1 exited " + $r1.ExitCode + " in phase 1 (output: " + $r1.Output + ")")
        if (-not (Stop-StackStartedByThisCheck)) { Write-Fail "teardown could not confirm port release" }
        $teardownDone = $true
        exit 5
    }
    Write-Step ("dev_agent_smoke.ps1 exit 0 (phase 1 output: " + $r1.Output + ")")

    $rows1 = Read-AgentEnv (Join-Path $ArtifactsDir "agent_env_phase1.json")
    if ($rows1.Count -ne 1) {
        Write-Fail ("expected exactly 1 live VitAgent.exe process, saw " + $rows1.Count)
        if (-not (Stop-StackStartedByThisCheck)) { Write-Fail "teardown could not confirm port release" }
        $teardownDone = $true
        exit 3
    }
    $phase1Failures = @()
    $row1Error = Get-JsonProp $rows1[0] "error"
    $row1Vars = Get-JsonProp $rows1[0] "vars"
    if ($null -eq $row1Vars) {
        $phase1Failures += ("psutil row has no vars object" + $(if ($row1Error) { (": " + $row1Error) } else { "" }))
    }
    else {
        foreach ($v in $ProbeVars) {
            $actual = Get-JsonProp $row1Vars $v
            if ($actual -ne $ProbeValue) {
                $phase1Failures += ($v + " expected '" + $ProbeValue + "' got '" + $actual + "'")
            }
        }
    }
    if ($row1Error) { $phase1Failures += ("env read error: " + $row1Error) }
    if ($phase1Failures.Count -gt 0) {
        foreach ($f in $phase1Failures) { Write-Fail ("phase 1 passthrough mismatch: " + $f) }
        Write-Host ("[restart-env-check] agent pid=" + $rows1[0].pid + " phase1 readback json: " + (Join-Path $ArtifactsDir "agent_env_phase1.json"))
        if (-not (Stop-StackStartedByThisCheck)) { Write-Fail "teardown could not confirm port release" }
        $teardownDone = $true
        exit 3
    }
    Write-Step ("phase 1 PASS: live agent pid=" + $rows1[0].pid + " carries all probe vars = '" + $ProbeValue + "' through the restart chain")

    # ================= Phase 2: no-var regression (zero change) =================
    Write-Step "phase 2: clear probe vars, restart agent again, assert absence + smoke exit 0"
    foreach ($v in $ProbeVars) {
        Remove-Item -LiteralPath ("env:" + $v) -ErrorAction SilentlyContinue
    }
    $r2 = Invoke-AgentRestart "phase2"
    if ($r2.ExitCode -ne 0) {
        Write-Fail ("dev_agent_smoke.ps1 exited " + $r2.ExitCode + " in phase 2 (output: " + $r2.Output + ")")
        if (-not (Stop-StackStartedByThisCheck)) { Write-Fail "teardown could not confirm port release" }
        $teardownDone = $true
        exit 5
    }
    $rows2 = Read-AgentEnv (Join-Path $ArtifactsDir "agent_env_phase2.json")
    if ($rows2.Count -ne 1) {
        Write-Fail ("phase 2: expected exactly 1 live VitAgent.exe process, saw " + $rows2.Count)
        if (-not (Stop-StackStartedByThisCheck)) { Write-Fail "teardown could not confirm port release" }
        $teardownDone = $true
        exit 4
    }
    $phase2Failures = @()
    $row2Error = Get-JsonProp $rows2[0] "error"
    $row2Vars = Get-JsonProp $rows2[0] "vars"
    if ($null -eq $row2Vars) {
        $phase2Failures += ("psutil row has no vars object" + $(if ($row2Error) { (": " + $row2Error) } else { "" }))
    }
    else {
        foreach ($v in $ProbeVars) {
            $actual = Get-JsonProp $row2Vars $v
            if ($null -ne $actual) {
                $phase2Failures += ($v + " expected absent, got '" + $actual + "'")
            }
        }
    }
    if ($row2Error) { $phase2Failures += ("env read error: " + $row2Error) }
    if ($phase2Failures.Count -gt 0) {
        foreach ($f in $phase2Failures) { Write-Fail ("phase 2 regression mismatch: " + $f) }
        if (-not (Stop-StackStartedByThisCheck)) { Write-Fail "teardown could not confirm port release" }
        $teardownDone = $true
        exit 4
    }
    Write-Step ("phase 2 PASS: fresh agent pid=" + $rows2[0].pid + " carries no probe vars; dev_agent_smoke.ps1 exit 0 identical to no-var baseline")
}
finally {
    if (-not $teardownDone) {
        if (Stop-StackStartedByThisCheck) {
            Write-Step "teardown: ports 7878/5555/5556 confirmed released"
        }
        else {
            Write-Fail "teardown could not confirm port release (agent/kernel may remain; record as 待处置)"
        }
    }
    # restore global_project_config.json + hash compare
    if ($null -ne $globalConfigSnapshot) {
        Copy-Item -LiteralPath $globalConfigSnapshot -Destination $globalConfigPath -Force
        $hashAfter = (Get-FileHash -LiteralPath $globalConfigPath -Algorithm SHA256).Hash
        if ($hashAfter -ne $globalConfigHashBefore) {
            Write-Fail ("global_project_config.json hash changed: before=" + $globalConfigHashBefore + " after=" + $hashAfter)
            exit 6
        }
        Write-Step ("global_project_config.json restored, hash verified: " + $hashAfter)
    }
    else {
        Write-Step "global_project_config.json absent before the run -- nothing to restore"
    }
}

Write-Host "[restart-env-check] VERDICT: ALL_PASS (phase 1 passthrough + phase 2 no-var regression)"
exit 0
