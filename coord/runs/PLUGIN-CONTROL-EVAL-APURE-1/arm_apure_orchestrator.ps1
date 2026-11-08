# PLUGIN-CONTROL-EVAL-APURE-1 (card 310) A-pure arm stack orchestrator.
#
# Same-pattern new script per AGENTS section 5 (card 310 file domain =
# artifact dir + card face; the 300 runner py trio is reused byte-frozen
# from runner_frozen/ -- blob-identical to the 307 runner_frozen snapshot --
# and is NOT modified). This wrapper replicates the frozen 300/307 ps1's
# stack phases (clear-field preflight -> env freeze -> bring-up via
# dev_agent_smoke -> driver legs -> owner-verified teardown) and adds the
# A-pure specifics:
#   - VIT_PLUGIN_CONTROL_EVAL_SEAM=1 set in the process env BEFORE any
#     process start (agent inherits via Start-Process) -- seam set =
#     305 S1-S4 + 308 S5 (tool-catalog trim) + S2b (render-surface strip),
#     all merged under the single env;
#   - device-verification probe gate (driver round T1 repeat 0 +
#     seam_verify_apure with S2b negative anchor + content-form
#     control_shell backstop + S5 catalog face + executed-typed gate)
#     BEFORE the 18 paid task rounds;
#   - 18 task rounds = frozen case order T1..T6 x repeat 1..3 via the frozen
#     driver `round` face (per-round fresh conversation + fixture + budget
#     counters), segment caps checked between rounds (plan segment_caps
#     A_star, caps carried over verbatim);
#   - consolidation via consolidate_apure.py (A-pure vs A(307) vs P
#     three-way comparison + typed_adapter device gate).
#
# Authoritative section 8 declaration: coord/runs/PLUGIN-CONTROL-EVAL-APURE-1/
# RUNBOOK.md (frozen before the run). Exit codes: 0 evaluated-complete /
# 1 wrapper or driver failure / 2 stack occupied or environment interrupt /
# 3 segment budget or stop-loss / 4 device verification failure (seam leak).
#
# Usage (from this card's worktree root):
#   powershell -NoProfile -ExecutionPolicy Bypass -File .\coord\runs\PLUGIN-CONTROL-EVAL-APURE-1\arm_apure_orchestrator.ps1

[CmdletBinding()]
param(
    [string]$RepoRoot = "",
    [string]$AgentHttp = "http://127.0.0.1:7878",
    [int]$WaitSeconds = 90,
    [string]$KernelExe = "D:\Vit_DAW_wt_228\VitApp\build_release\VitApp_artefacts\Release\VitApp.exe",
    [string]$RunIdSuffix = ""
)

$ErrorActionPreference = "Stop"
if ([string]::IsNullOrWhiteSpace($RepoRoot)) { $RepoRoot = (Get-Location).Path }
$RepoRoot = (Resolve-Path -LiteralPath $RepoRoot).Path
$ArtifactRoot = Join-Path $RepoRoot "coord\runs\PLUGIN-CONTROL-EVAL-APURE-1"
$RunnerFrozen = Join-Path $ArtifactRoot "runner_frozen"
$RunStamp = Get-Date -Format "yyyyMMdd_HHmmss"
if ($RunIdSuffix -ne "") { $RunStamp = $RunStamp + "_" + $RunIdSuffix }
$RunRoot = Join-Path $ArtifactRoot ("arm_apure_" + $RunStamp)
New-Item -ItemType Directory -Force -Path $RunRoot | Out-Null
$LogsDir = Join-Path $RunRoot "logs"
New-Item -ItemType Directory -Force -Path $LogsDir | Out-Null

$sec8Decl = "A-pure arm (seam open, VIT_PLUGIN_CONTROL_EVAL_SEAM=1 = 305 S1-S4 + 308 S5 catalog trim + S2b render strip): s0 refreeze (0 LLM) + dryround (0 LLM) + 1 device-verification probe round (T1 repeat 0, paid, not scored) + 18 paid task rounds (T1..T6 x repeat 1..3, new conversation + fresh fixture each); per-round caps (12 calls / 40k in / 8k out per call / 30 tool calls / 20 min) frozen; segment caps 216 calls / 8.64M in / 1.728M out checked between rounds (caps verbatim from plan A_star); probe gate must prove S2/S2b/S3/S4/S5/CS faces + zero executed typed calls before task rounds; failure taxonomy per plan + seam_leakage; full declaration in RUNBOOK.md (frozen pre-run)"
$report = [ordered]@{
    card            = "PLUGIN-CONTROL-EVAL-APURE-1"
    leg             = "arm_apure"
    run_id          = $RunStamp
    repo_head       = (git -C $RepoRoot rev-parse HEAD 2>$null | Out-String).Trim()
    branch          = (git -C $RepoRoot rev-parse --abbrev-ref HEAD 2>$null | Out-String).Trim()
    started_at      = (Get-Date).ToString("o")
    sec8_decl       = $sec8Decl
    agent_http      = $AgentHttp
    seam_env        = "VIT_PLUGIN_CONTROL_EVAL_SEAM=1 (set in process env before any process start; = 305 S1-S4 + 308 S5 + S2b)"
}

function Write-Step([string]$m) { Write-Host ("[apure] " + $m) -ForegroundColor Cyan }
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
    Write-Step "Phase 1: env freeze (isolation + telemetry + seam + model-face record)"
    if (-not (Test-Path -LiteralPath $KernelExe)) { throw ("kernel exe missing: " + $KernelExe) }
    $report.kernel_exe = $KernelExe
    $report.kernel_sha256 = (Get-FileHash -LiteralPath $KernelExe -Algorithm SHA256).Hash
    # git-clean project isolation (byte-exact via cmd redirect; same as 300/307 ps1)
    $ProjectCopy = Join-Path $RunRoot "default_project.xml"
    & cmd /c ('git -C "' + $RepoRoot + '" show HEAD:VitApp/Workspace/default_project.xml > "' + $ProjectCopy + '"')
    if ($LASTEXITCODE -ne 0 -or -not (Test-Path -LiteralPath $ProjectCopy) -or
        ((Get-Item -LiteralPath $ProjectCopy).Length -le 0)) {
        throw "git-clean default_project.xml copy failed"
    }
    try { if ($null -eq ((Get-Content -LiteralPath $ProjectCopy -Raw) -as [xml])) { throw "project copy not parseable XML" } } catch { throw }
    Set-Item -LiteralPath "env:VIT_PROJECT_XML" -Value $ProjectCopy
    # per-run telemetry + prompt render + payload capture (env-before-agent-start)
    Set-Item -LiteralPath "env:VIT_AGENT_LLM_TELEMETRY_PATH" -Value (Join-Path $LogsDir "agent_llm_telemetry.jsonl")
    Set-Item -LiteralPath "env:VIT_AGENT_PROMPT_RENDER_PATH" -Value (Join-Path $LogsDir "agent_prompt_render.jsonl")
    Set-Item -LiteralPath "env:VIT_AGENT_LLM_PAYLOAD_CAPTURE" -Value $LogsDir
    # A-pure seam: exact "1" only (anything else leaves production behavior)
    Set-Item -LiteralPath "env:VIT_PLUGIN_CONTROL_EVAL_SEAM" -Value "1"
    # model face record: A-pure arm runs the product default config; env
    # model/base overrides would change the model under test -> refuse.
    $modelEnv = @{}
    foreach ($k in "VIT_AGENT_LLM_BASE_URL", "VIT_AGENT_LLM_MODEL", "OPENAI_BASE_URL", "OPENAI_MODEL") {
        $v = [System.Environment]::GetEnvironmentVariable($k, "Process")
        $modelEnv[$k] = $(if ([string]::IsNullOrWhiteSpace($v)) { "<unset>" } else { $v })
    }
    foreach ($k in "VIT_AGENT_LLM_API_KEY", "OPENAI_API_KEY") {
        $v = [System.Environment]::GetEnvironmentVariable($k, "Process")
        $modelEnv[$k] = $(if ([string]::IsNullOrWhiteSpace($v)) { "<unset>" } else { "<set,masked>" })
    }
    $report.model_env = $modelEnv
    if ($modelEnv["VIT_AGENT_LLM_MODEL"] -ne "<unset>" -or $modelEnv["OPENAI_MODEL"] -ne "<unset>") {
        throw ("arm_apure refuses to run with an env model override in force: " + ($modelEnv | ConvertTo-Json -Compress))
    }
    Write-Ok ("VIT_PROJECT_XML=" + $ProjectCopy)
    Write-Ok ("telemetry=" + $env:VIT_AGENT_LLM_TELEMETRY_PATH + " payload_capture=" + $env:VIT_AGENT_LLM_PAYLOAD_CAPTURE)
    Write-Ok ("seam env VIT_PLUGIN_CONTROL_EVAL_SEAM=" + $env:VIT_PLUGIN_CONTROL_EVAL_SEAM)

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

    # agent health + S5 live catalog face (tool_count recorded; catalog-face
    # assertion itself is proven from the probe payload by seam_verify S5)
    $state = $null
    $healthDeadline = (Get-Date).AddSeconds(45)
    while ((Get-Date) -lt $healthDeadline -and ($null -eq $state -or [string]$state.status -ne "ok")) {
        try { $state = Invoke-RestMethod -Method GET -Uri ($AgentHttp.TrimEnd("/") + "/agent/state") -TimeoutSec 10 } catch { Start-Sleep -Seconds 3 }
        if ($null -eq $state) { Start-Sleep -Seconds 3 }
    }
    if ($null -eq $state -or [string]$state.status -ne "ok") { throw "agent health check failed" }
    $report.agent_tool_count = [string]$state.tool_count
    Write-Ok ("agent ok tool_count=" + [string]$state.tool_count)

    # ---------------- phase 3: driver env (shared module import path)
    $env:PYTHONPATH = (Join-Path $RepoRoot "scripts")
    $Driver = Join-Path $RunnerFrozen "plugin_control_eval_driver.py"
    $Verify = Join-Path $ArtifactRoot "seam_verify_apure.py"
    $Consolidate = Join-Path $ArtifactRoot "consolidate_apure.py"
    $AgentLog = Join-Path $RepoRoot "VitApp\Workspace\Logs\agent_last.log"
    $TelemetryPath = Join-Path $LogsDir "agent_llm_telemetry.jsonl"

    # ---------------- phase 4: S0 refreeze (zero LLM)
    Write-Step "Phase 4: driver s0 refreeze (zero LLM)"
    $S0Dir = Join-Path $RunRoot ("s0_" + $RunStamp)
    $s0Log = Join-Path $RunRoot "driver_s0_console.log"
    & python $Driver "s0" "--agent-http" $AgentHttp "--out-dir" $S0Dir 2>&1 | Tee-Object -FilePath $s0Log | Out-Null
    $s0Exit = $LASTEXITCODE
    $report.s0_exit = $s0Exit
    if ($s0Exit -ne 0) { throw ("s0 refreeze failed exit=" + $s0Exit + " (fixture_failure per RUNBOOK; forensics in s0 dir)") }
    $freezePath = Join-Path $S0Dir "case_freeze.json"
    if (-not (Test-Path -LiteralPath $freezePath)) { throw "s0 completed but case_freeze.json missing" }
    $freeze = Get-Content -LiteralPath $freezePath -Raw -Encoding UTF8 | ConvertFrom-Json
    if (-not $freeze.s0_all_verified) { throw "s0_all_verified=false -- refusing model rounds (plan gate)" }
    $allowanceArr = @($freeze.allowance_set.T4)
    $Allowance = ($allowanceArr -join ",")
    $report.s0_all_verified = $true
    $report.t4_allowance = $Allowance
    Write-Ok ("s0 verified; T4 allowance={" + $Allowance + "}")
    if (Test-Path -LiteralPath $TelemetryPath) {
        $telLines = @(Get-Content -LiteralPath $TelemetryPath | Where-Object { $_.Trim() -ne "" })
        if ($telLines.Count -gt 0) { Write-Warn2 ("s0 telemetry non-empty: " + $telLines.Count + " lines (classified per RUNBOOK)") }
        else { Write-Ok "s0 zero-LLM telemetry proof: 0 lines" }
    } else { Write-Ok "s0 zero-LLM telemetry proof: file absent" }

    # ---------------- phase 5: dryround with seam open (zero LLM)
    Write-Step "Phase 5: driver dryround (seam open, zero LLM)"
    $DryDir = Join-Path $RunRoot ("dryround_" + $RunStamp)
    $dryLog = Join-Path $RunRoot "driver_dryround_console.log"
    & python $Driver "dryround" "--agent-http" $AgentHttp "--out-dir" $DryDir 2>&1 | Tee-Object -FilePath $dryLog | Out-Null
    $dryExit = $LASTEXITCODE
    $report.dryround_exit = $dryExit
    if ($dryExit -ne 0) { throw ("dryround failed exit=" + $dryExit) }
    if (Test-Path -LiteralPath $TelemetryPath) {
        $telLines = @(Get-Content -LiteralPath $TelemetryPath | Where-Object { $_.Trim() -ne "" })
        if ($telLines.Count -gt 0) { throw ("dryround telemetry non-empty (" + $telLines.Count + " lines) -- zero-LLM leg violated; environment class per RUNBOOK") }
    }
    Write-Ok "dryround pass with seam open; zero-LLM telemetry proof holds"

    # ---------------- phase 6: device-verification probe (paid, T1 repeat 0)
    Write-Step "Phase 6: seam device-verification probe (T1 repeat 0, paid, not scored)"
    $ProbeDir = Join-Path $RunRoot "probe"
    $probeLog = Join-Path $RunRoot "driver_probe_console.log"
    & python $Driver "round" "--case" "T1" "--repeat" "0" "--agent-http" $AgentHttp `
        "--rounds-dir" $ProbeDir "--telemetry-path" $TelemetryPath "--agent-log" $AgentLog `
        "--allowance" $Allowance 2>&1 | Tee-Object -FilePath $probeLog | Out-Null
    $probeExit = $LASTEXITCODE
    $report.probe_exit = $probeExit
    if ($probeExit -ne 0) { throw ("probe round driver failed exit=" + $probeExit) }
    Write-Step "Phase 6b: seam_verify_apure gate (S2/S2b/S3/S4/S5/CS + typed)"
    $verifyLog = Join-Path $RunRoot "seam_verify_console.log"
    & python $Verify "--a-capture" (Join-Path $LogsDir "agent_llm_payload_capture.jsonl") `
        "--probe-dir" (Join-Path $ProbeDir "T1_r0") `
        "--p-capture" (Join-Path $ArtifactRoot "p_baseline\agent_llm_payload_capture.jsonl") `
        "--t3r1-capture" (Join-Path $ArtifactRoot "a307_reference\t3_r1_contaminated_capture.jsonl") `
        "--out" (Join-Path $RunRoot "seam_verify_probe.json") 2>&1 | Tee-Object -FilePath $verifyLog | Out-Null
    $verifyExit = $LASTEXITCODE
    $report.seam_verify_exit = $verifyExit
    if ($verifyExit -ne 0) {
        $report.verdict = "device_invalid_seam_leakage"
        $report.finished_at = (Get-Date).ToString("o"); Save-Json $report "run_report.json"
        Write-Fail "seam verification FAILED -- task rounds NOT started (card stop condition); exit 4"
        $driverExit = 0; $finalExit = 4
        throw ("seam_verify exit=" + $verifyExit + " -- see seam_verify_probe.json")
    }
    Write-Ok "seam verification passed (S2/S2b/S3/S4/S5/CS + zero executed typed) -- task rounds released"

    # ---------------- phase 7: 18 task rounds (paid)
    Write-Step "Phase 7: 18 task rounds (T1..T6 x repeat 1..3, segment caps between rounds)"
    $RoundsDir = Join-Path $RunRoot "rounds"
    New-Item -ItemType Directory -Force -Path $RoundsDir | Out-Null
    $capsA = @{ rounds = 18; max_llm_calls = 216; max_input_tokens = 8640000; max_output_tokens = 1728000 }
    $totals = @{ llm_calls = 0; input_tokens = 0; output_tokens = 0 }
    $roundsIndex = @()
    $consecEnvInterrupt = 0
    $segmentStop = ""
    $caseOrder = @("T1", "T2", "T3", "T4", "T5", "T6")
    :outer foreach ($case_id in $caseOrder) {
        foreach ($repeat in 1..3) {
            if ($segmentStop -ne "") {
                $roundsIndex += [ordered]@{ case_id = $case_id; repeat = $repeat; status = "not_run_" + $segmentStop }
                continue
            }
            if ($totals["llm_calls"] -ge $capsA["max_llm_calls"] -or
                $totals["input_tokens"] -ge $capsA["max_input_tokens"] -or
                $totals["output_tokens"] -ge $capsA["max_output_tokens"]) {
                $segmentStop = "segment_cap_reached"
                $roundsIndex += [ordered]@{ case_id = $case_id; repeat = $repeat; status = "not_run_segment_cap" }
                continue
            }
            Write-Step ("round " + $case_id + " r" + $repeat + " ...")
            $rLog = Join-Path $RunRoot ("driver_round_" + $case_id + "_r" + $repeat + "_console.log")
            & python $Driver "round" "--case" $case_id "--repeat" $repeat "--agent-http" $AgentHttp `
                "--rounds-dir" $RoundsDir "--telemetry-path" $TelemetryPath "--agent-log" $AgentLog `
                "--allowance" $Allowance 2>&1 | Tee-Object -FilePath $rLog | Out-Null
            $rExit = $LASTEXITCODE
            $roundDirR = Join-Path $RoundsDir ($case_id + "_r" + $repeat)
            $recordPath = Join-Path $roundDirR "round_record.json"
            if ($rExit -eq 0 -and (Test-Path -LiteralPath $recordPath)) {
                $rec = Get-Content -LiteralPath $recordPath -Raw -Encoding UTF8 | ConvertFrom-Json
                $u = $rec.usage
                $totals["llm_calls"] += [int]($u.llm_calls)
                $totals["input_tokens"] += [int]($u.input_tokens)
                $totals["output_tokens"] += [int]($u.output_tokens)
                $roundsIndex += [ordered]@{
                    case_id = $case_id; repeat = $repeat; exit = 0
                    conversation_id = $rec.conversation_id
                    first_action = ($rec.first_action | ConvertTo-Json -Compress -Depth 6)
                    final_verdict = $rec.verdict.final_verdict
                    failure_tags = ($rec.verdict.failure_tags -join ",")
                    behavior_tags = ($rec.behavior_tags -join ",")
                    mutation_count = $rec.verdict.mutation_count
                    llm_calls = [int]($u.llm_calls); input_tokens = [int]($u.input_tokens)
                    output_tokens = [int]($u.output_tokens)
                    cache_read_tokens = [int]($u.cache_read_tokens)
                    stop_reason = $rec.stop_reason
                    round_dir = $roundDirR
                }
                $consecEnvInterrupt = 0
                Write-Ok ("verdict=" + $rec.verdict.final_verdict + " stop=" + $rec.stop_reason + " calls=" + $u.llm_calls)
            } else {
                $errTail = ""
                if (Test-Path -LiteralPath $rLog) { $errTail = ((Get-Content -LiteralPath $rLog -Tail 6) -join " | ") }
                $isHarness = $errTail -match "RuntimeError"
                $class = $(if ($isHarness) { "harness_invalid" } else { "environment_interruption" })
                $roundsIndex += [ordered]@{ case_id = $case_id; repeat = $repeat; exit = $rExit;
                                            status = $class; error_tail = $errTail }
                if ($class -eq "environment_interruption") {
                    $consecEnvInterrupt += 1
                    Write-Fail ("environment_interruption on " + $case_id + " r" + $repeat + " (consecutive=" + $consecEnvInterrupt + ")")
                    if ($consecEnvInterrupt -ge 2) {
                        $segmentStop = "stop_loss_two_consecutive_environment"
                        Write-Fail "stop-loss: two consecutive environment failures -- segment halted (remaining rounds recorded not_run)"
                    }
                } else {
                    Write-Warn2 ("harness_invalid on " + $case_id + " r" + $repeat + " -- recorded, segment continues (frozen driver policy)")
                }
            }
        }
    }
    $report.task_rounds = $roundsIndex
    $report.segment_budget = @{ totals = $totals; caps = $capsA;
        within_caps = ($totals["llm_calls"] -le $capsA["max_llm_calls"] -and
                       $totals["input_tokens"] -le $capsA["max_input_tokens"] -and
                       $totals["output_tokens"] -le $capsA["max_output_tokens"]) }
    if ($segmentStop -ne "") { $report.segment_stop = $segmentStop }
    $evaluated = @($roundsIndex | Where-Object { $_.final_verdict }).Count
    Write-Step ("task rounds evaluated: " + $evaluated + "/18; budget=" + ($totals | ConvertTo-Json -Compress))

    # ---------------- phase 8b: post-run corpus seam check FIRST (typed/S5/CS
    # over all rounds is a device-gate face; if it fails the segment result is
    # quarantined for supervisor adjudication per card stop conditions)
    Write-Step "Phase 8b: post-run seam corpus check (device face, all rounds)"
    $verifyLog2 = Join-Path $RunRoot "seam_verify_corpus_console.log"
    & python $Verify "--a-capture" (Join-Path $LogsDir "agent_llm_payload_capture.jsonl") `
        "--probe-dir" (Join-Path $ProbeDir "T1_r0") `
        "--p-capture" (Join-Path $ArtifactRoot "p_baseline\agent_llm_payload_capture.jsonl") `
        "--t3r1-capture" (Join-Path $ArtifactRoot "a307_reference\t3_r1_contaminated_capture.jsonl") `
        "--out" (Join-Path $RunRoot "seam_verify_corpus.json") 2>&1 | Tee-Object -FilePath $verifyLog2 | Out-Null
    $corpusExit = $LASTEXITCODE
    $report.seam_verify_corpus_exit = $corpusExit
    if ($corpusExit -ne 0) { Write-Fail "post-run corpus seam check FAILED -- evidence preserved, supervisor adjudicates (card stop condition met post-hoc)" }

    # ---------------- phase 8: consolidation (offline; three-way compare)
    Write-Step "Phase 8: consolidation"
    $consLog = Join-Path $RunRoot "consolidate_console.log"
    & python $Consolidate "--run-dir" $RunRoot `
        "--p-summary" (Join-Path $ArtifactRoot "p_baseline\segment_p_summary.json") `
        "--a307-summary" (Join-Path $ArtifactRoot "a307_reference\arm_a_summary_307.json") `
        "--seam-corpus" (Join-Path $RunRoot "seam_verify_corpus.json") `
        "--out" (Join-Path $RunRoot "arm_apure_summary.json") 2>&1 | Tee-Object -FilePath $consLog | Out-Null
    $consExit = $LASTEXITCODE
    $report.consolidate_exit = $consExit
    if ($consExit -ne 0) { Write-Warn2 ("consolidation exit=" + $consExit + " (raw round records remain authoritative)") }

    $driverExit = 0
    if ($evaluated -eq 18 -and $segmentStop -eq "" -and $consExit -eq 0 -and $corpusExit -eq 0) { $finalExit = 0 }
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
    # copy agent log + telemetry evidence into the run dir (per-run copies)
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
    Write-Host ("[apure] run_root: " + $RunRoot)
    Write-Host ("[apure] verdict: " + $report.verdict + " final_exit=" + $finalExit)
}
exit $finalExit
