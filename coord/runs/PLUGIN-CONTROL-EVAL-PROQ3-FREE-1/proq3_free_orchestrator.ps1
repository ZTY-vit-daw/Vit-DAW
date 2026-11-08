# PLUGIN-CONTROL-EVAL-PROQ3-FREE-1 (card 320) T7 free-control orchestrator.
#
# Same-pattern new script per AGENTS section 5 (card 320 file domain = artifact
# dir + card face + placement-script extension + the NEW T7 single-case driver
# outside the frozen six-case system; agent source ZERO change -- both seams
# live in main). Stack shape replicates the 318 BAREFREE orchestrator with the
# T7 specifics:
#   - VIT_PLUGIN_CONTROL_EVAL_SEAM=1 AND
#     VIT_PLUGIN_CONTROL_EVAL_SEAM_BARE_WRITE=1 both set in the process env
#     BEFORE any process start (BAREFREE configuration fully on; no
#     B_RETRIEVAL seam, no BR1 prompt seam);
#   - device leg (zero LLM): Pro-Q 3 registry resolve (card stop-condition
#     gate) + persistent placement of THREE independent project copies
#     (实验·ProQ3 裸控 r1/r2/r3.vit in %APPDATA%\Vit-DAW\projects, card-311
#     placement precedent: one real-audio track + Pro-Q 3 mounted, start-page
#     recent face) + initial-state equivalence + known-op bare write (must
#     land + read back changed, then restore) + save/open persistence gate;
#     ANY failure = exit 4, NO paid rounds;
#   - 3 paid T7 rounds (one per project copy, new conversation each, frozen
#     per-round caps, confirm-approve policy) with before/after/reopen
#     readbacks and the final reply captured verbatim as the self-report;
#     final states are PRESERVED for the user GUI review (no teardown);
#   - consolidation (offline): three rounds side by side + mechanical claim
#     extraction + honesty notes + corpus flags + reconciliation draft.
#
# Authoritative section 8 declaration: coord/runs/PLUGIN-CONTROL-EVAL-PROQ3-FREE-1/
# RUNBOOK.md (frozen before the run). Exit codes: 0 evaluated-complete /
# 1 wrapper or driver failure / 2 stack occupied or environment interrupt /
# 3 stop-loss / 4 device verification failure (incl. Pro-Q 3 unloadable).
#
# Usage (from this card's worktree root):
#   powershell -NoProfile -ExecutionPolicy Bypass -File .\coord\runs\PLUGIN-CONTROL-EVAL-PROQ3-FREE-1\proq3_free_orchestrator.ps1

[CmdletBinding()]
param(
    [string]$RepoRoot = "",
    [string]$AgentHttp = "http://127.0.0.1:7878",
    [int]$WaitSeconds = 90,
    [string]$KernelExe = "D:\Vit_DAW_wt_321persist\VitApp\build_release\VitApp_artefacts\Release\VitApp.exe",
    [string]$RunIdSuffix = ""
)

$ErrorActionPreference = "Stop"
if ([string]::IsNullOrWhiteSpace($RepoRoot)) { $RepoRoot = (Get-Location).Path }
$RepoRoot = (Resolve-Path -LiteralPath $RepoRoot).Path
$ArtifactRoot = Join-Path $RepoRoot "coord\runs\PLUGIN-CONTROL-EVAL-PROQ3-FREE-1"
$RunStamp = Get-Date -Format "yyyyMMdd_HHmmss"
if ($RunIdSuffix -ne "") { $RunStamp = $RunStamp + "_" + $RunIdSuffix }
$RunRoot = Join-Path $ArtifactRoot ("proq3_free_" + $RunStamp)
New-Item -ItemType Directory -Force -Path $RunRoot | Out-Null
$LogsDir = Join-Path $RunRoot "logs"
New-Item -ItemType Directory -Force -Path $LogsDir | Out-Null
$ProjectsDir = Join-Path $env:APPDATA "Vit-DAW\projects"

$sec8Decl = "T7 free-control Pro-Q 3 FINAL BATCH (kernel WITH 321 rack-param-persistence fix 950cf07b..30f5; BAREFREE face VIT_PLUGIN_CONTROL_EVAL_SEAM=1 PLUS VIT_PLUGIN_CONTROL_EVAL_SEAM_BARE_WRITE=1 both set before any process start AND psutil-verified on the live agent process pre-paid; NO retrieval seam, NO prompt seam): zero-LLM device gates (registry resolve + placement of three persistent project copies with one real-audio track + Pro-Q 3 + start-page recent + initial-state equivalence + known-op bare write must land + save/open persistence -- now expected PASS under the 321 fix) then 3 paid rounds (one per copy, new conversation, frozen message with forced self-report, per-round caps 12 calls / 40k in / 8k out per call / 30 tools / 20 min, segment caps 36 calls / 1.44M in / 288k out); final states PRESERVED for the user's GUI review; failure taxonomy = frozen plan set + plugin_load_failure + guard_blocked_write + seam_leakage + self_report_mismatch (honesty observation) + persistence_failure; single run, no rerun; device/env failure after paid rounds = forensics hand-over; full declaration in RUNBOOK.md final-batch section (frozen pre-run)"
$report = [ordered]@{
    card            = "PLUGIN-CONTROL-EVAL-PROQ3-FREE-1"
    leg             = "proq3_free"
    run_id          = $RunStamp
    repo_head       = (git -C $RepoRoot rev-parse HEAD 2>$null | Out-String).Trim()
    branch          = (git -C $RepoRoot rev-parse --abbrev-ref HEAD 2>$null | Out-String).Trim()
    started_at      = (Get-Date).ToString("o")
    sec8_decl       = $sec8Decl
    agent_http      = $AgentHttp
    projects_dir    = $ProjectsDir
    seam_env        = "VIT_PLUGIN_CONTROL_EVAL_SEAM=1 + VIT_PLUGIN_CONTROL_EVAL_SEAM_BARE_WRITE=1 (both set in process env before any process start; no B_RETRIEVAL, no BR1_PROMPT)"
}

function Write-Step([string]$m) { Write-Host ("[proq3] " + $m) -ForegroundColor Cyan }
function Write-Ok([string]$m)   { Write-Host ("  ok: " + $m) -ForegroundColor Green }
function Write-Warn2([string]$m){ Write-Host ("  warn: " + $m) -ForegroundColor Yellow }
function Write-Fail([string]$m) { Write-Host ("  FAIL: " + $m) -ForegroundColor Red }
function Save-Json($o, [string]$name) {
    $o | ConvertTo-Json -Depth 80 | Set-Content -LiteralPath (Join-Path $RunRoot $name) -Encoding UTF8
}
function Test-StackGone {
    $out = @()
    foreach ($p in 7878, 5555, 5556, 5557, 8787, 18787) {
        if (Get-NetTCPConnection -LocalPort $p -State Listen -ErrorAction SilentlyContinue) { $out += "port $p listening" }
    }
    foreach ($p in 4444, 4445) {
        if (Get-NetUDPEndpoint -LocalPort $p -ErrorAction SilentlyContinue) { $out += "udp port $p bound" }
    }
    foreach ($n in "VitApp", "VitAgent", "vsphub", "godot", "python") {
        $procs = Get-Process -Name $n -ErrorAction SilentlyContinue
        if ($procs) { $out += ("process " + $n + " pid=" + ($procs.Id -join ",")) }
    }
    return $out
}
function Get-StackPids {
    $pids = @{}
    foreach ($n in "VitApp", "VitAgent") {
        $procs = Get-Process -Name $n -ErrorAction SilentlyContinue
        if ($procs) { $pids[$n] = @($procs.Id) }
    }
    return $pids
}
function Wait-AgentOk([int]$Seconds = 45) {
    $state = $null
    $deadline = (Get-Date).AddSeconds($Seconds)
    while ((Get-Date) -lt $deadline -and ($null -eq $state -or [string]$state.status -ne "ok")) {
        try { $state = Invoke-RestMethod -Method GET -Uri ($AgentHttp.TrimEnd("/") + "/agent/state") -TimeoutSec 10 } catch { Start-Sleep -Seconds 3 }
        if ($null -eq $state) { Start-Sleep -Seconds 3 }
    }
    return $state
}

$driverExit = -1
$finalExit = 1
try {
    # ---------------- phase 0: clear field (immediately before start)
    Write-Step "Phase 0: clear-field preflight"
    $residual = @(Test-StackGone)
    if ($residual.Count -gt 0) {
        foreach ($r in $residual) { Write-Fail $r }
        $report.verdict = "stack_occupied"; $report.environment_interrupt = ($residual -join "; ")
        $report.finished_at = (Get-Date).ToString("o"); Save-Json $report "run_report.json"; exit 2
    }
    Write-Ok "six TCP ports + two UDP ports free, no stack processes"

    # ---------------- phase 1: environment freeze (before ANY process start)
    Write-Step "Phase 1: env freeze (isolation + telemetry + seams + model-face record)"
    if (-not (Test-Path -LiteralPath $KernelExe)) { throw ("kernel exe missing: " + $KernelExe) }
    $report.kernel_exe = $KernelExe
    $report.kernel_sha256 = (Get-FileHash -LiteralPath $KernelExe -Algorithm SHA256).Hash
    # final batch: kernel WITH the 321 rack-param-persistence fix (supervisor
    # authorization message; the wt_228 F9E3DA0A baseline is the UNFIXED build)
    if ($report.kernel_sha256 -ne "950CF07B7B735749C2DEF5E9447B7260F1272694DC5B83A865790B8E24CD30F5") {
        throw ("kernel binary hash is not the 321-fixed build 950cf07b..30f5: " + $report.kernel_sha256)
    }
    $ProjectCopy = Join-Path $RunRoot "default_project.xml"
    & cmd /c ('git -C "' + $RepoRoot + '" show HEAD:VitApp/Workspace/default_project.xml > "' + $ProjectCopy + '"')
    if ($LASTEXITCODE -ne 0 -or -not (Test-Path -LiteralPath $ProjectCopy) -or
        ((Get-Item -LiteralPath $ProjectCopy).Length -le 0)) {
        throw "git-clean default_project.xml copy failed"
    }
    try { if ($null -eq ((Get-Content -LiteralPath $ProjectCopy -Raw) -as [xml])) { throw "project copy not parseable XML" } } catch { throw }
    Set-Item -LiteralPath "env:VIT_PROJECT_XML" -Value $ProjectCopy
    Set-Item -LiteralPath "env:VIT_AGENT_LLM_TELEMETRY_PATH" -Value (Join-Path $LogsDir "agent_llm_telemetry.jsonl")
    Set-Item -LiteralPath "env:VIT_AGENT_PROMPT_RENDER_PATH" -Value (Join-Path $LogsDir "agent_prompt_render.jsonl")
    Set-Item -LiteralPath "env:VIT_AGENT_LLM_PAYLOAD_CAPTURE" -Value $LogsDir
    Set-Item -LiteralPath "env:VIT_PLUGIN_CONTROL_EVAL_SEAM" -Value "1"
    Set-Item -LiteralPath "env:VIT_PLUGIN_CONTROL_EVAL_SEAM_BARE_WRITE" -Value "1"
    $modelEnv = @{}
    foreach ($k in "VIT_AGENT_LLM_BASE_URL", "VIT_AGENT_LLM_MODEL", "OPENAI_BASE_URL", "OPENAI_MODEL") {
        $v = [System.Environment]::GetEnvironmentVariable($k, "Process")
        $modelEnv[$k] = $(if ([string]::IsNullOrWhiteSpace($v)) { "<unset>" } else { $v })
    }
    foreach ($k in "VIT_AGENT_LLM_API_KEY", "OPENAI_API_KEY") {
        $v = [System.Environment]::GetEnvironmentVariable($k, "Process")
        $modelEnv[$k] = $(if ([string]::IsNullOrWhiteSpace($v)) { "<set,masked>" } else { "<unset>" })
    }
    $report.model_env = $modelEnv
    if ($modelEnv["VIT_AGENT_LLM_MODEL"] -ne "<unset>" -or $modelEnv["OPENAI_MODEL"] -ne "<unset>") {
        throw ("proq3_free refuses to run with an env model override in force: " + ($modelEnv | ConvertTo-Json -Compress))
    }
    foreach ($r in 1..3) {
        $existing = Join-Path $ProjectsDir ("实验·ProQ3 裸控 r" + $r + ".vit")
        if (Test-Path -LiteralPath $existing) {
            throw ("project copy already present (rerun hygiene, placement must not overwrite): " + $existing)
        }
    }
    Write-Ok ("VIT_PROJECT_XML=" + $ProjectCopy)
    Write-Ok ("seam env VIT_PLUGIN_CONTROL_EVAL_SEAM=" + $env:VIT_PLUGIN_CONTROL_EVAL_SEAM + " + VIT_PLUGIN_CONTROL_EVAL_SEAM_BARE_WRITE=" + $env:VIT_PLUGIN_CONTROL_EVAL_SEAM_BARE_WRITE)

    # ---------------- phase 2: stack bring-up via dev_agent_smoke (no UI)
    Write-Step "Phase 2: stack bring-up (dev_agent_smoke -StartKernel -RestartAgent, no UI)"
    $devSmoke = Join-Path $RepoRoot "scripts\dev_agent_smoke.ps1"
    $devParams = @{ StartKernel = $true; RestartAgent = $true; NoChatSmoke = $true;
                    NoStripSilenceSmoke = $true; Strict = $true; WaitSeconds = $WaitSeconds;
                    RepoRoot = $RepoRoot; KernelExe = $KernelExe; AgentHttp = $AgentHttp }
    $devConsoleLog = Join-Path $RunRoot "dev_agent_smoke_console.log"
    & $devSmoke @devParams 2>&1 | Tee-Object -FilePath $devConsoleLog | Out-Null
    if ($LASTEXITCODE -ne 0) { throw ("dev_agent_smoke bring-up failed exit=" + $LASTEXITCODE + " (see dev_agent_smoke_console.log)") }
    $stackPids = Get-StackPids
    $report.stack_pids = $stackPids
    try { $report.agent_sha256 = (Get-FileHash -LiteralPath (Join-Path $RepoRoot "agent\bin\VitAgent.exe") -Algorithm SHA256).Hash } catch { $report.agent_sha256 = "unavailable" }
    Write-Ok ("stack up pids=" + (($stackPids.GetEnumerator() | ForEach-Object { $_.Key + "=" + ($_.Value -join ",") }) -join " "))

    $state = Wait-AgentOk
    if ($null -eq $state -or [string]$state.status -ne "ok") { throw "agent health check failed" }
    $report.agent_tool_count = [string]$state.tool_count
    Write-Ok ("agent ok tool_count=" + [string]$state.tool_count)

    # ---------------- phase 2b: agent-process env liveness (paid-round guard)
    # 321 out-of-domain record: the -RestartAgent chain only carries env the
    # agent INHERITS at start. This orchestrator sets both seam envs BEFORE
    # any process start (run3 known-op landing is the empirical proof), and
    # this leg re-proves it on the live agent PROCESS before any paid round
    # (321's psutil precedent). Missing env = device verification failure.
    Write-Step "Phase 2b: agent-process seam env liveness (psutil, pre-paid guard)"
    $envProbe = @"
import json, sys
try:
    import psutil
except ImportError:
    print(json.dumps({"ok": False, "error": "psutil unavailable"})); sys.exit(0)
rows = []
for p in psutil.process_iter(['name']):
    try:
        if (p.info['name'] or '').lower() == 'vitagent.exe':
            env = p.environ()
            rows.append({
                'pid': p.pid,
                'VIT_PLUGIN_CONTROL_EVAL_SEAM': env.get('VIT_PLUGIN_CONTROL_EVAL_SEAM'),
                'VIT_PLUGIN_CONTROL_EVAL_SEAM_BARE_WRITE': env.get('VIT_PLUGIN_CONTROL_EVAL_SEAM_BARE_WRITE'),
            })
    except Exception:
        continue
ok = len(rows) > 0 and all(r['VIT_PLUGIN_CONTROL_EVAL_SEAM'] == '1' and
                           r['VIT_PLUGIN_CONTROL_EVAL_SEAM_BARE_WRITE'] == '1' for r in rows)
print(json.dumps({'ok': ok, 'agents': rows}, ensure_ascii=False))
"@
    $envOut = $envProbe | python -
    $report.agent_env_liveness = $envOut
    if ($envOut -notmatch '"ok": true') {
        $report.verdict = "device_invalid_agent_env_not_live"
        $report.finished_at = (Get-Date).ToString("o"); Save-Json $report "run_report.json"
        Write-Fail "agent process is NOT carrying both seam envs -- exit 4, no paid rounds"
        $driverExit = 0; $finalExit = 4
        throw ("agent env liveness failed: " + $envOut)
    }
    Write-Ok "agent process carries both seam envs (psutil-verified)"

    # ---------------- phase 3: driver env
    $env:PYTHONPATH = (Join-Path $RepoRoot "scripts")
    $Driver = Join-Path $ArtifactRoot "proq3_free_driver.py"
    $AgentLog = Join-Path $RepoRoot "VitApp\Workspace\Logs\agent_last.log"
    $TelemetryPath = Join-Path $LogsDir "agent_llm_telemetry.jsonl"

    # ---------------- phase 4: device leg (zero LLM, stop-condition gates)
    Write-Step "Phase 4: device leg (registry resolve + placement r1-r3 + known-op + persistence; zero LLM)"
    $DeviceDir = Join-Path $RunRoot "device"
    $devLog = Join-Path $RunRoot "driver_device_console.log"
    & python $Driver "device" "--agent-http" $AgentHttp "--out-dir" $DeviceDir `
        "--projects-dir" $ProjectsDir 2>&1 | Tee-Object -FilePath $devLog | Out-Null
    $deviceExit = $LASTEXITCODE
    $report.device_exit = $deviceExit
    if ($deviceExit -ne 0) {
        $report.verdict = "device_verification_failed"
        $report.finished_at = (Get-Date).ToString("o"); Save-Json $report "run_report.json"
        Write-Fail "device leg FAILED (Pro-Q 3 unloadable / seam not live / persistence broken -- card stop condition); exit 4, no paid rounds"
        $driverExit = 0; $finalExit = 4
        throw ("device leg exit=" + $deviceExit)
    }
    Write-Ok "device gates green: registry + placement r1/r2/r3 + initial-state equivalence + known-op + persistence"

    # ---------------- phase 5: 3 paid T7 rounds (one per project copy)
    Write-Step "Phase 5: 3 paid T7 rounds (segment caps between rounds)"
    $RoundsDir = Join-Path $RunRoot "rounds"
    New-Item -ItemType Directory -Force -Path $RoundsDir | Out-Null
    $caps = @{ rounds = 3; max_llm_calls = 36; max_input_tokens = 1440000; max_output_tokens = 288000 }
    $totals = @{ llm_calls = 0; input_tokens = 0; output_tokens = 0 }
    $roundsIndex = @()
    $consecEnvInterrupt = 0
    $segmentStop = ""
    :outer foreach ($r in 1..3) {
        if ($segmentStop -ne "") {
            $roundsIndex += [ordered]@{ round = $r; status = "not_run_" + $segmentStop }
            continue
        }
        if ($totals["llm_calls"] -ge $caps["max_llm_calls"] -or
            $totals["input_tokens"] -ge $caps["max_input_tokens"] -or
            $totals["output_tokens"] -ge $caps["max_output_tokens"]) {
            $segmentStop = "segment_cap_reached"
            $roundsIndex += [ordered]@{ round = $r; status = "not_run_segment_cap" }
            continue
        }
        Write-Step ("round r" + $r + " ...")
        $projectPath = Join-Path $ProjectsDir ("实验·ProQ3 裸控 r" + $r + ".vit")
        $rLog = Join-Path $RunRoot ("driver_round_r" + $r + "_console.log")
        & python $Driver "round" "--repeat" $r "--project" $projectPath `
            "--agent-http" $AgentHttp "--rounds-dir" $RoundsDir `
            "--telemetry-path" $TelemetryPath "--agent-log" $AgentLog 2>&1 | Tee-Object -FilePath $rLog | Out-Null
        $rExit = $LASTEXITCODE
        $recordPath = Join-Path $RoundsDir ("r" + $r + "\round_record.json")
        if ($rExit -eq 0 -and (Test-Path -LiteralPath $recordPath)) {
            $rec = Get-Content -LiteralPath $recordPath -Raw -Encoding UTF8 | ConvertFrom-Json
            $u = $rec.usage
            $totals["llm_calls"] += [int]($u.llm_calls)
            $totals["input_tokens"] += [int]($u.input_tokens)
            $totals["output_tokens"] += [int]($u.output_tokens)
            $roundsIndex += [ordered]@{
                round = $r; exit = 0
                conversation_id = $rec.conversation_id
                stop_reason = $rec.stop_reason
                changed_params = @($rec.mutation_audit).Count
                final_state_persisted = $rec.final_state_preservation.final_state_persisted
                llm_calls = [int]($u.llm_calls); input_tokens = [int]($u.input_tokens)
                output_tokens = [int]($u.output_tokens)
                round_dir = (Join-Path $RoundsDir ("r" + $r))
            }
            $consecEnvInterrupt = 0
            Write-Ok ("r" + $r + " evaluated stop=" + $rec.stop_reason + " changed=" + @($rec.mutation_audit).Count + " persisted=" + $rec.final_state_preservation.final_state_persisted)
        }
        else {
            $errTail = ""
            if (Test-Path -LiteralPath $rLog) { $errTail = ((Get-Content -LiteralPath $rLog -Tail 6) -join " | ") }
            $isHarness = $errTail -match "RuntimeError|DeviceFailure|Traceback"
            $class = $(if ($isHarness) { "harness_invalid" } else { "environment_interruption" })
            $roundsIndex += [ordered]@{ round = $r; exit = $rExit; status = $class; error_tail = $errTail }
            if ($class -eq "environment_interruption") {
                $consecEnvInterrupt += 1
                Write-Fail ("environment_interruption on r" + $r + " (consecutive=" + $consecEnvInterrupt + ")")
                if ($consecEnvInterrupt -ge 2) {
                    $segmentStop = "stop_loss_two_consecutive_environment"
                    Write-Fail "stop-loss: two consecutive environment failures -- segment halted"
                }
            }
            else {
                Write-Warn2 ("harness_invalid on r" + $r + " -- recorded, segment continues")
            }
        }
    }
    $report.task_rounds = $roundsIndex
    $report.segment_budget = @{ totals = $totals; caps = $caps;
        within_caps = ($totals["llm_calls"] -le $caps["max_llm_calls"] -and
                       $totals["input_tokens"] -le $caps["max_input_tokens"] -and
                       $totals["output_tokens"] -le $caps["max_output_tokens"]) }
    if ($segmentStop -ne "") { $report.segment_stop = $segmentStop }
    $evaluated = @($roundsIndex | Where-Object { $_.exit -eq 0 }).Count
    Write-Step ("task rounds evaluated: " + $evaluated + "/3; budget=" + ($totals | ConvertTo-Json -Compress))

    # ---------------- phase 6: consolidation (offline)
    Write-Step "Phase 6: consolidation (three rounds side by side + reconciliation draft)"
    $consLog = Join-Path $RunRoot "consolidate_console.log"
    & python $Driver "consolidate" "--run-dir" $RunRoot 2>&1 | Tee-Object -FilePath $consLog | Out-Null
    $consExit = $LASTEXITCODE
    $report.consolidate_exit = $consExit
    if ($consExit -ne 0) { Write-Warn2 ("consolidation exit=" + $consExit + " (raw round records remain authoritative)") }

    $driverExit = 0
    if ($evaluated -eq 3 -and $segmentStop -eq "" -and $consExit -eq 0) { $finalExit = 0 }
    elseif ($segmentStop -ne "") { $finalExit = 3 }
    else { $finalExit = 1 }
    $report.verdict = "driver_complete"
}
catch {
    if (-not $report.verdict) { $report.verdict = "wrapper_fail" }
    $report.error = $_.Exception.Message
    if ($finalExit -eq 1) { $finalExit = 1 }
    try { $_ | Out-String | Set-Content -LiteralPath (Join-Path $RunRoot "script_exception.txt") -Encoding UTF8 } catch { }
}
finally {
    # ---------------- teardown (owner-verified, never silent reuse)
    try {
        $pids = Get-StackPids
        foreach ($name in $pids.Keys) {
            foreach ($p in $pids[$name]) { try { Stop-Process -Id $p -Force -ErrorAction Stop } catch { } }
        }
        Start-Sleep -Seconds 3
        $remain = @(Test-StackGone)
        $report.teardown = @{ remaining = $remain; torn_down = ($remain.Count -eq 0) }
        if ($remain.Count -eq 0) { Write-Ok "stack torn down (TEARDOWN_ALL_ZERO: ports free, processes gone)" }
        else { Write-Warn2 ("teardown residue: " + ($remain -join "; ")) }
    } catch { }
    try {
        $AgentLogCopy = Join-Path $RepoRoot "VitApp\Workspace\Logs\agent_last.log"
        foreach ($src in @($AgentLogCopy, (Join-Path $LogsDir "agent_llm_telemetry.jsonl"),
                           (Join-Path $LogsDir "agent_llm_payload_capture.jsonl"),
                           (Join-Path $LogsDir "agent_prompt_render.jsonl"))) {
            if ($src -and (Test-Path -LiteralPath $src)) {
                Copy-Item -LiteralPath $src -Destination (Join-Path $LogsDir ("copy_" + (Split-Path -Leaf $src))) -Force
            }
        }
    } catch { }
    $report.finished_at = (Get-Date).ToString("o")
    Save-Json $report "run_report.json"
    Write-Host ("[proq3] run_root: " + $RunRoot)
    Write-Host ("[proq3] verdict: " + $report.verdict + " final_exit=" + $finalExit)
}
exit $finalExit
