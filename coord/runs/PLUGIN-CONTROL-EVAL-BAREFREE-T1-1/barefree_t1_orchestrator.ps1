# barefree_t1_orchestrator.ps1 -- PLUGIN-CONTROL-EVAL-BAREFREE-T1-1 (card 319)
#
# T1-completion leg of the plugin-control-eval barefree line. Single stack
# window, sequential phases, mirroring the 318 barefree orchestrator with
# the 319 specifics:
#   - same env seam set as 318 (VIT_PLUGIN_CONTROL_EVAL_SEAM=1 A-pure face
#     + VIT_PLUGIN_CONTROL_EVAL_SEAM_BARE_WRITE=1) where the barewrite key
#     now ALSO covers the third layer (harness broadMixObserveFirstWriteGuard
#     exempts plugin parameter writes; agent built from THIS worktree);
#   - GW live probes in BOTH flavors (zero LLM): the 318 T3 typed-guard
#     victim probe (reused verbatim from the 318 artifact root) AND the new
#     T1 third-layer probe (Pro-R param 6 write WITH the natural-mix task
#     prompt in context -- the exact 318-blocked shape): open must land +
#     readback change; off-after-restart must be guard-rejected again;
#     failure = device invalid -> exit 4, NO paid rounds;
#   - dryround with all seams open (zero LLM);
#   - device-verification probe gate (driver round T1 repeat 0, paid, not
#     scored) + seam_verify_t1 twelve faces (310 six + W0 + P0 + GW + GWH
#     advisory + GW-LIVE + GW-LIVE-T1) BEFORE the 3 paid task rounds;
#   - 3 task rounds = frozen case T1 x repeat 1..3 via the frozen driver
#     (same trio, blob-identical, imported from the 318 artifact root),
#     segment caps scaled 3/18 of plan A_star (36 calls / 1.44M in / 288k
#     out) checked between rounds;
#   - consolidation (offline): consolidate_t1.py seven-way T1 comparison +
#     barewrite ledger + guard_blocked_write classification maintained;
#   - owner teardown in finally (TEARDOWN_ALL_ZERO), per AGENTS section 9.
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
$ArtifactRoot = Join-Path $RepoRoot "coord\runs\PLUGIN-CONTROL-EVAL-BAREFREE-T1-1"
$BarefreeRoot = Join-Path $RepoRoot "coord\runs\PLUGIN-CONTROL-EVAL-BAREFREE-1"
$RunStamp = Get-Date -Format "yyyyMMdd_HHmmss"
if ($RunIdSuffix -ne "") { $RunStamp = $RunStamp + "_" + $RunIdSuffix }
$RunRoot = Join-Path $ArtifactRoot ("arm_barefree_t1_" + $RunStamp)
New-Item -ItemType Directory -Force -Path $RunRoot | Out-Null
$LogsDir = Join-Path $RunRoot "logs"
New-Item -ItemType Directory -Force -Path $LogsDir | Out-Null

$sec8Decl = "BAREFREE-T1 leg (A-pure face VIT_PLUGIN_CONTROL_EVAL_SEAM=1 PLUS guard-permission seam VIT_PLUGIN_CONTROL_EVAL_SEAM_BARE_WRITE=1 now covering all three guard layers incl. harness broadMixObserveFirstWriteGuard): GW live probes open+off BOTH flavors (zero LLM: 318 T3 victim Plates/Damper param 1 value 0.7 + NEW T1 third-layer victim Pro-R param 6 value 0.35 WITH natural-mix task prompt context; open must land + readback change; off-after-restart must be guard-rejected again) + s0 refreeze (0 LLM) + dryround (0 LLM) + 1 device-verification probe round (T1 repeat 0, paid, not scored) + 3 paid task rounds (T1 x repeat 1..3, new conversation + fresh fixture each); per-round caps (12 calls / 40k in / 8k out per call / 30 tool calls / 20 min) frozen; segment caps 36 calls / 1.44M in / 288k out (plan A_star scaled 3/18, frozen in RUNBOOK) checked between rounds; probe gate = seam_verify_t1 twelve faces (S2/S2b/S3/S4/S5/CS/S1 + W0 + P0 + GW + GWH advisory + GW-LIVE + GW-LIVE-T1) before task rounds; failure taxonomy per plan + seam_leakage + guard-permission probes; full declaration in RUNBOOK.md (frozen pre-run)"
$report = [ordered]@{
    card            = "PLUGIN-CONTROL-EVAL-BAREFREE-T1-1"
    leg             = "arm_barefree_t1"
    run_id          = $RunStamp
    repo_head       = (git -C $RepoRoot rev-parse HEAD 2>$null | Out-String).Trim()
    branch          = (git -C $RepoRoot rev-parse --abbrev-ref HEAD 2>$null | Out-String).Trim()
    started_at      = (Get-Date).ToString("o")
    sec8_decl       = $sec8Decl
    agent_http      = $AgentHttp
    seam_env        = "VIT_PLUGIN_CONTROL_EVAL_SEAM=1 + VIT_PLUGIN_CONTROL_EVAL_SEAM_BARE_WRITE=1 (both set in process env before any process start; same key set as 318, the key now also gates the harness third-layer seam built in this worktree)"
}

function Write-Step([string]$m) { Write-Host ("[barefree-t1] " + $m) -ForegroundColor Cyan }
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
        throw ("stack NOT clear at start: " + ($residual -join "; ") + " -- environment_interruption class, hand to stack owner")
    }
    Write-Ok "stack clear (six TCP + two UDP + processes all zero)"

    # ---------------- phase 1: environment freeze (before ANY process start)
    Write-Step "Phase 1: env freeze (isolation + telemetry + seams + model-face record)"
    if (-not (Test-Path -LiteralPath $KernelExe)) { throw ("kernel exe missing: " + $KernelExe) }
    $report.kernel_exe = $KernelExe
    $report.kernel_sha256 = (Get-FileHash -LiteralPath $KernelExe -Algorithm SHA256).Hash
    # git-clean project isolation (byte-exact via cmd redirect; same as 300/307/310/312/317/318)
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
    # A-pure seam + barewrite seam (three layers under the same key)
    Set-Item -LiteralPath "env:VIT_PLUGIN_CONTROL_EVAL_SEAM" -Value "1"
    Set-Item -LiteralPath "env:VIT_PLUGIN_CONTROL_EVAL_SEAM_BARE_WRITE" -Value "1"
    # model face record: same refusal as 318 (product default config only)
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
        throw ("arm_barefree_t1 refuses to run with an env model override in force: " + ($modelEnv | ConvertTo-Json -Compress))
    }
    Write-Ok ("VIT_PROJECT_XML=" + $ProjectCopy)
    Write-Ok ("seam env VIT_PLUGIN_CONTROL_EVAL_SEAM=" + $env:VIT_PLUGIN_CONTROL_EVAL_SEAM + " + VIT_PLUGIN_CONTROL_EVAL_SEAM_BARE_WRITE=" + $env:VIT_PLUGIN_CONTROL_EVAL_SEAM_BARE_WRITE)

    # ---------------- phase 2: stack bring-up via dev_agent_smoke (no UI)
    Write-Step "Phase 2: stack bring-up (dev_agent_smoke -StartKernel -RestartAgent, no UI; agent built from THIS worktree with the 319 harness seam)"
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

    # ---------------- phase 3: driver env (frozen trio + 318 probes reused; 319 tools local)
    $env:PYTHONPATH = (Join-Path $RepoRoot "scripts")
    $Driver = Join-Path $BarefreeRoot "runner_frozen\plugin_control_eval_driver.py"
    $GwProbe = Join-Path $BarefreeRoot "gw_probe.py"
    $GwProbeT1 = Join-Path $ArtifactRoot "gw_probe_t1.py"
    $Verify = Join-Path $ArtifactRoot "seam_verify_t1.py"
    $Consolidate = Join-Path $ArtifactRoot "consolidate_t1.py"
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

    # ---------------- phase 5: GW-OPEN live probes, both flavors (zero LLM)
    Write-Step "Phase 5a: GW live probe seam-OPEN, 318 T3 victim (zero LLM; Plates/Damper param 1 -> 0.7)"
    $GwOpenJson = Join-Path $RunRoot "gw_open.json"
    & python $GwProbe "--mode" "open" "--agent-http" $AgentHttp "--out" $GwOpenJson 2>&1 | Tee-Object -FilePath (Join-Path $RunRoot "gw_open_console.log") | Out-Null
    $gwOpenExit = $LASTEXITCODE
    $report.gw_open_exit = $gwOpenExit
    if ($gwOpenExit -ne 0) {
        $report.verdict = "device_invalid_guard_seam_not_live"
        $report.finished_at = (Get-Date).ToString("o"); Save-Json $report "run_report.json"
        Write-Fail "GW open (T3 flavor) FAILED -- exit 4"
        $driverExit = 0; $finalExit = 4
        throw ("gw_probe open exit=" + $gwOpenExit)
    }
    Write-Ok "GW open (T3): guarded-family bare write LANDED with changed independent readback"

    Write-Step "Phase 5b: GW live probe seam-OPEN, T1 third-layer victim (zero LLM; Pro-R param 6 -> 0.35 WITH natural-mix prompt context)"
    $GwT1OpenJson = Join-Path $RunRoot "gw_t1_open.json"
    & python $GwProbeT1 "--mode" "open" "--agent-http" $AgentHttp "--out" $GwT1OpenJson 2>&1 | Tee-Object -FilePath (Join-Path $RunRoot "gw_t1_open_console.log") | Out-Null
    $gwT1OpenExit = $LASTEXITCODE
    $report.gw_t1_open_exit = $gwT1OpenExit
    if ($gwT1OpenExit -ne 0) {
        $report.verdict = "device_invalid_third_layer_seam_not_live"
        $report.finished_at = (Get-Date).ToString("o"); Save-Json $report "run_report.json"
        Write-Fail "GW open (T1 flavor) FAILED: the 318-blocked write shape did not land (see gw_t1_open.json) -- exit 4"
        $driverExit = 0; $finalExit = 4
        throw ("gw_probe_t1 open exit=" + $gwT1OpenExit)
    }
    Write-Ok "GW open (T1): third-layer victim write LANDED with changed independent readback"

    # ---------------- phase 5c: GW-OFF regression probes (unset + agent restart)
    Write-Step "Phase 5c: GW regression probes seam-OFF (unset env + agent restart -> guards must reject again)"
    $gwDevLog = Join-Path $RunRoot "dev_agent_restart_off_console.log"
    Remove-Item -LiteralPath "env:VIT_PLUGIN_CONTROL_EVAL_SEAM_BARE_WRITE" -ErrorAction SilentlyContinue
    & $devSmoke -RestartAgent -NoChatSmoke -NoStripSilenceSmoke -SkipBuild -Strict -WaitSeconds $WaitSeconds `
        -RepoRoot $RepoRoot -AgentHttp $AgentHttp 2>&1 | Tee-Object -FilePath $gwDevLog | Out-Null
    if ($LASTEXITCODE -ne 0) { throw ("agent restart (seam-off) failed exit=" + $LASTEXITCODE) }
    $state2 = Wait-AgentOk
    if ($null -eq $state2 -or [string]$state2.status -ne "ok") { throw "agent health check failed after seam-off restart" }

    $GwOffJson = Join-Path $RunRoot "gw_off.json"
    & python $GwProbe "--mode" "off" "--agent-http" $AgentHttp "--out" $GwOffJson 2>&1 | Tee-Object -FilePath (Join-Path $RunRoot "gw_off_console.log") | Out-Null
    $gwOffExit = $LASTEXITCODE
    $report.gw_off_exit = $gwOffExit
    if ($gwOffExit -ne 0) {
        $report.verdict = "device_invalid_guard_not_restored"
        $report.finished_at = (Get-Date).ToString("o"); Save-Json $report "run_report.json"
        Write-Fail "GW off (T3 flavor) FAILED: guard rejection NOT restored -- exit 4"
        $driverExit = 0; $finalExit = 4
        Set-Item -LiteralPath "env:VIT_PLUGIN_CONTROL_EVAL_SEAM_BARE_WRITE" -Value "1"
        throw ("gw_probe off exit=" + $gwOffExit)
    }
    Write-Ok "GW off (T3): same write guard-rejected again (typed interception restored)"

    $GwT1OffJson = Join-Path $RunRoot "gw_t1_off.json"
    & python $GwProbeT1 "--mode" "off" "--agent-http" $AgentHttp "--out" $GwT1OffJson 2>&1 | Tee-Object -FilePath (Join-Path $RunRoot "gw_t1_off_console.log") | Out-Null
    $gwT1OffExit = $LASTEXITCODE
    $report.gw_t1_off_exit = $gwT1OffExit
    if ($gwT1OffExit -ne 0) {
        $report.verdict = "device_invalid_third_layer_not_restored"
        $report.finished_at = (Get-Date).ToString("o"); Save-Json $report "run_report.json"
        Write-Fail "GW off (T1 flavor) FAILED: harness-guard rejection NOT restored after unset (see gw_t1_off.json) -- exit 4"
        $driverExit = 0; $finalExit = 4
        Set-Item -LiteralPath "env:VIT_PLUGIN_CONTROL_EVAL_SEAM_BARE_WRITE" -Value "1"
        throw ("gw_probe_t1 off exit=" + $gwT1OffExit)
    }
    Write-Ok "GW off (T1): same write guard-rejected again (interception restored)"

    # restore the seam-open env + agent for the task rounds (binary already built; -SkipBuild + recorded hash)
    Set-Item -LiteralPath "env:VIT_PLUGIN_CONTROL_EVAL_SEAM_BARE_WRITE" -Value "1"
    $gwDevLog2 = Join-Path $RunRoot "dev_agent_restart_open_console.log"
    & $devSmoke -RestartAgent -NoChatSmoke -NoStripSilenceSmoke -SkipBuild -Strict -WaitSeconds $WaitSeconds `
        -RepoRoot $RepoRoot -AgentHttp $AgentHttp 2>&1 | Tee-Object -FilePath $gwDevLog2 | Out-Null
    if ($LASTEXITCODE -ne 0) { throw ("agent restart (seam-open restore) failed exit=" + $LASTEXITCODE) }
    $state3 = Wait-AgentOk
    if ($null -eq $state3 -or [string]$state3.status -ne "ok") { throw "agent health check failed after seam-open restore" }
    try { $report.agent_sha256_after_restarts = (Get-FileHash -LiteralPath (Join-Path $RepoRoot "agent\bin\VitAgent.exe") -Algorithm SHA256).Hash } catch { $report.agent_sha256_after_restarts = "unavailable" }
    if ($report.agent_sha256_after_restarts -ne $report.agent_sha256) {
        throw ("agent binary changed across restarts: " + $report.agent_sha256 + " -> " + $report.agent_sha256_after_restarts)
    }
    # merge the GW probes into the seam-verify input artifacts
    $gwOpen = Get-Content -LiteralPath $GwOpenJson -Raw -Encoding UTF8 | ConvertFrom-Json
    $gwOff = Get-Content -LiteralPath $GwOffJson -Raw -Encoding UTF8 | ConvertFrom-Json
    $guardLive = [ordered]@{ probed_at = (Get-Date).ToString("o");
                             barewrite_probe = $gwOpen; gw_off_probe = $gwOff }
    Save-Json $guardLive "guard_live_probe.json"
    $gwT1Open = Get-Content -LiteralPath $GwT1OpenJson -Raw -Encoding UTF8 | ConvertFrom-Json
    $gwT1Off = Get-Content -LiteralPath $GwT1OffJson -Raw -Encoding UTF8 | ConvertFrom-Json
    $guardLiveT1 = [ordered]@{ probed_at = (Get-Date).ToString("o");
                               t1_open_probe = $gwT1Open; t1_off_probe = $gwT1Off }
    Save-Json $guardLiveT1 "guard_live_probe_t1.json"
    Write-Ok "guard_live_probe.json + guard_live_probe_t1.json written (GW-LIVE / GW-LIVE-T1 face inputs)"

    # ---------------- phase 6: dryround with all seams open (zero LLM)
    Write-Step "Phase 6: driver dryround (seams open, zero LLM)"
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
    Write-Ok "dryround pass with seams open; zero-LLM telemetry proof holds"

    # ---------------- phase 7: device-verification probe (paid, T1 repeat 0)
    Write-Step "Phase 7: seam device-verification probe (T1 repeat 0, paid, not scored -- the 319 case as carrier)"
    $ProbeDir = Join-Path $RunRoot "probe"
    $probeLog = Join-Path $RunRoot "driver_probe_console.log"
    & python $Driver "round" "--case" "T1" "--repeat" "0" "--agent-http" $AgentHttp `
        "--rounds-dir" $ProbeDir "--telemetry-path" $TelemetryPath "--agent-log" $AgentLog `
        "--allowance" $Allowance 2>&1 | Tee-Object -FilePath $probeLog | Out-Null
    $probeExit = $LASTEXITCODE
    $report.probe_exit = $probeExit
    if ($probeExit -ne 0) { throw ("probe round driver failed exit=" + $probeExit) }
    Write-Step "Phase 7b: seam_verify_t1 gate (S2/S2b/S3/S4/S5/CS/S1 + W0 + P0 + GW + GWH advisory + GW-LIVE + GW-LIVE-T1)"
    $verifyLog = Join-Path $RunRoot "seam_verify_console.log"
    & python $Verify "--a-capture" (Join-Path $LogsDir "agent_llm_payload_capture.jsonl") `
        "--probe-dir" (Join-Path $ProbeDir "T1_r0") `
        "--p-capture" (Join-Path $BarefreeRoot "p_baseline\agent_llm_payload_capture.jsonl") `
        "--t3r1-capture" (Join-Path $BarefreeRoot "a307_reference\t3_r1_contaminated_capture.jsonl") `
        "--gw-live-probe" (Join-Path $RunRoot "guard_live_probe.json") `
        "--gw-live-t1-probe" (Join-Path $RunRoot "guard_live_probe_t1.json") `
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
    Write-Ok "seam verification passed (310 six faces + W0 + P0 + GW + GWH advisory + GW-LIVE + GW-LIVE-T1) -- task rounds released"

    # ---------------- phase 8: 3 task rounds (paid, T1 x repeat 1..3)
    Write-Step "Phase 8: 3 task rounds (T1 x repeat 1..3, segment caps between rounds; scaled 3/18 of plan A_star)"
    $RoundsDir = Join-Path $RunRoot "rounds"
    New-Item -ItemType Directory -Force -Path $RoundsDir | Out-Null
    $capsA = @{ rounds = 3; max_llm_calls = 36; max_input_tokens = 1440000; max_output_tokens = 288000 }
    $totals = @{ llm_calls = 0; input_tokens = 0; output_tokens = 0 }
    $roundsIndex = @()
    $consecEnvInterrupt = 0
    $segmentStop = ""
    $caseOrder = @("T1")
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
                    stop_reason = $rec.stop_reason
                    llm_calls = [int]($u.llm_calls); input_tokens = [int]($u.input_tokens)
                    output_tokens = [int]($u.output_tokens)
                    cache_read_tokens = [int]($u.cache_read_tokens)
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
    Write-Step ("task rounds evaluated: " + $evaluated + "/3; budget=" + ($totals | ConvertTo-Json -Compress))

    # ---------------- phase 8b: post-run corpus seam check (device face)
    Write-Step "Phase 8b: post-run seam corpus check (device face, all rounds)"
    $verifyLog2 = Join-Path $RunRoot "seam_verify_corpus_console.log"
    & python $Verify "--a-capture" (Join-Path $LogsDir "agent_llm_payload_capture.jsonl") `
        "--probe-dir" (Join-Path $ProbeDir "T1_r0") `
        "--p-capture" (Join-Path $BarefreeRoot "p_baseline\agent_llm_payload_capture.jsonl") `
        "--t3r1-capture" (Join-Path $BarefreeRoot "a307_reference\t3_r1_contaminated_capture.jsonl") `
        "--gw-live-probe" (Join-Path $RunRoot "guard_live_probe.json") `
        "--gw-live-t1-probe" (Join-Path $RunRoot "guard_live_probe_t1.json") `
        "--out" (Join-Path $RunRoot "seam_verify_corpus.json") 2>&1 | Tee-Object -FilePath $verifyLog2 | Out-Null
    $corpusExit = $LASTEXITCODE
    $report.seam_verify_corpus_exit = $corpusExit
    if ($corpusExit -ne 0) { Write-Fail "post-run corpus seam check FAILED -- evidence preserved, supervisor adjudicates (card stop condition met post-hoc)" }

    # ---------------- phase 8c: consolidation (offline; seven-way T1 compare)
    Write-Step "Phase 8c: consolidation (BAREFREE-T1 vs BAREFREE-318 vs BR1 vs B312 vs A-pure vs A307 vs P + barewrite ledger + guard_blocked_write)"
    $consLog = Join-Path $RunRoot "consolidate_console.log"
    & python $Consolidate "--run-dir" $RunRoot `
        "--p-summary" (Join-Path $BarefreeRoot "p_baseline\segment_p_summary.json") `
        "--a307-summary" (Join-Path $BarefreeRoot "a307_reference\arm_a_summary_307.json") `
        "--apure-summary" (Join-Path $BarefreeRoot "apure_reference\arm_apure_summary.json") `
        "--b312-summary" (Join-Path $BarefreeRoot "b_reference\arm_b_summary_312.json") `
        "--br1-summary" (Join-Path $BarefreeRoot "br1_reference\arm_br1_summary.json") `
        "--barefree318-summary" (Join-Path $BarefreeRoot "arm_barefree_20261108_221634\arm_barefree_summary.json") `
        "--capture" (Join-Path $LogsDir "agent_llm_payload_capture.jsonl") `
        "--gw-live-probe" (Join-Path $RunRoot "guard_live_probe.json") `
        "--gw-live-t1-probe" (Join-Path $RunRoot "guard_live_probe_t1.json") `
        "--out" (Join-Path $RunRoot "arm_barefree_t1_summary.json") 2>&1 | Tee-Object -FilePath $consLog | Out-Null
    $consExit = $LASTEXITCODE
    $report.consolidate_exit = $consExit
    if ($consExit -ne 0) { Write-Warn2 ("consolidation exit=" + $consExit + " (raw round records remain authoritative)") }

    $driverExit = 0
    if ($evaluated -eq 3 -and $segmentStop -eq "" -and $consExit -eq 0 -and $corpusExit -eq 0) { $finalExit = 0 }
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
    Write-Host ("[barefree-t1] run_root: " + $RunRoot)
    Write-Host ("[barefree-t1] verdict: " + $report.verdict + " final_exit=" + $finalExit)
}
exit $finalExit
