# PLUGIN-CONTROL-EVAL-RUNNER-1 (card 300) stack wrapper.
#
# Real-stack wrapper for the plugin-control evaluation runner (zero-LLM S0
# known-op leg / zero-LLM dryround / paid S1-P product-baseline segment).
# Precedents: scripts/pgtdc_compat_verify.ps1 (stack bring-up + teardown
# shape) + scripts/dev_agent_smoke.ps1 AutomationLLMRouteSmoke leg (per-run
# telemetry/payload env-before-agent-start window + git-clean project
# isolation via VIT_PROJECT_XML).
#
# AGENTS section 8 declaration (written into run_report.json BEFORE the run;
# never adjusted afterwards):
#   mode s0/dryround: LLM calls 0 (deterministic /agent/invoke + kernel ZMQ
#     faces only; success = all six cases verified + exit 0; failure classes
#     fixture_failure (assertion face; forensics, no model rounds) /
#     environment_interrupt (stack/transport; raw evidence retained);
#     stop-loss = same-signature fixture failure on two runs -> forensic card,
#     no identical rerun).
#   mode segment-p: 6 paid rounds, deepseek-flash (product default config
#     file; env overrides refused), per-round caps and segment caps from the
#     frozen plan (PER_ROUND_CAPS / SEGMENT_P_CAPS in
#     plugin_control_eval_cases.py); overrun stops the round/segment;
#     verdicts come from the independent evaluator; exit 0 = evaluation
#     complete (never "all tasks succeeded").
#
# Usage (from this card's worktree root):
#   powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\plugin_control_eval.ps1 -Mode s0
#   powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\plugin_control_eval.ps1 -Mode dryround
#   powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\plugin_control_eval.ps1 -Mode segment-p
#   Optional: -KernelExe <path> (default: the wt_228 F9E3DA0A kernel, C++ face
#   unchanged vs origin/main; hash recorded), -SkipBuild (agent binary must
#   already contain this card's scripts -- script-only card, agent code is
#   read-only, so the agent build equals origin/main's).

[CmdletBinding()]
param(
    [string]$RepoRoot = "",
    [string]$Mode = "s0",
    [string]$AgentHttp = "http://127.0.0.1:7878",
    [int]$WaitSeconds = 90,
    [string]$KernelExe = "D:\Vit_DAW_wt_228\VitApp\build_release\VitApp_artefacts\Release\VitApp.exe",
    [switch]$SkipBuild,
    [string]$RunIdSuffix = ""
)

$ErrorActionPreference = "Stop"
if ([string]::IsNullOrWhiteSpace($RepoRoot)) { $RepoRoot = (Get-Location).Path }
$RepoRoot = (Resolve-Path -LiteralPath $RepoRoot).Path
$RunStamp = Get-Date -Format "yyyyMMdd_HHmmss"
if ($RunIdSuffix -ne "") { $RunStamp = $RunStamp + "_" + $RunIdSuffix }
$RunRoot = Join-Path $RepoRoot ("coord\runs\PLUGIN-CONTROL-EVAL-RUNNER-1\" + $Mode + "_" + $RunStamp)
New-Item -ItemType Directory -Force -Path $RunRoot | Out-Null
$LogsDir = Join-Path $RunRoot "logs"
New-Item -ItemType Directory -Force -Path $LogsDir | Out-Null

$sec8Decl = switch ($Mode) {
    "s0" { "zero LLM; success = six cases verified (known-op reachable, enum/unit interpretation correct, readback+restore valid, T4 allowance frozen, T6 unsatisfiable re-confirmed); failure classes fixture_failure / environment_interrupt; stop-loss = same-signature fixture failure twice -> forensic card" }
    "dryround" { "zero LLM; scripted known-op through the full round pipeline; success = pipeline verdict pass + exit 0" }
    "segment-p" { "6 paid rounds deepseek-flash (product default; env model overrides refused); per-round/segment caps frozen in plugin_control_eval_cases.py; overrun stops round/segment; independent evaluator verdicts; exit 0 = evaluation complete (not task success)" }
    default { throw ("unknown mode " + $Mode) }
}

$report = [ordered]@{
    card            = "PLUGIN-CONTROL-EVAL-RUNNER-1"
    leg             = $Mode
    run_id          = $RunStamp
    repo_head       = (git -C $RepoRoot rev-parse HEAD 2>$null | Out-String).Trim()
    branch          = (git -C $RepoRoot rev-parse --abbrev-ref HEAD 2>$null | Out-String).Trim()
    started_at      = (Get-Date).ToString("o")
    sec8_decl       = $sec8Decl
    agent_http      = $AgentHttp
}

function Write-Step([string]$m) { Write-Host ("[pce] " + $m) -ForegroundColor Cyan }
function Write-Ok([string]$m)   { Write-Host ("  ok: " + $m) -ForegroundColor Green }
function Write-Warn2([string]$m){ Write-Host ("  warn: " + $m) -ForegroundColor Yellow }
function Write-Fail([string]$m) { Write-Host ("  FAIL: " + $m) -ForegroundColor Red }
function Save-Json($o, [string]$name) {
    $o | ConvertTo-Json -Depth 60 | Set-Content -LiteralPath (Join-Path $RunRoot $name) -Encoding UTF8
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
    Write-Step "Phase 1: env freeze (isolation + telemetry + model-face record)"
    if (-not (Test-Path -LiteralPath $KernelExe)) { throw ("kernel exe missing: " + $KernelExe) }
    $report.kernel_exe = $KernelExe
    $report.kernel_sha256 = (Get-FileHash -LiteralPath $KernelExe -Algorithm SHA256).Hash
    # git-clean project isolation (217/218/220 lesson; byte-exact via cmd redirect)
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
    # model face record: the P arm must run on the product default config; an
    # env model/base override would change the model under test -> refuse.
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
    if ($Mode -eq "segment-p") {
        if ($modelEnv["VIT_AGENT_LLM_MODEL"] -ne "<unset>" -or $modelEnv["OPENAI_MODEL"] -ne "<unset>") {
            throw ("segment-p refuses to run with an env model override in force: " + ($modelEnv | ConvertTo-Json -Compress))
        }
    }
    Write-Ok ("VIT_PROJECT_XML=" + $ProjectCopy)
    Write-Ok ("telemetry=" + $env:VIT_AGENT_LLM_TELEMETRY_PATH + " payload_capture=" + $env:VIT_AGENT_LLM_PAYLOAD_CAPTURE)

    # ---------------- phase 2: stack bring-up via dev_agent_smoke (no UI)
    Write-Step "Phase 2: stack bring-up (dev_agent_smoke -StartKernel -RestartAgent, no UI)"
    $devSmoke = Join-Path $RepoRoot "scripts\dev_agent_smoke.ps1"
    $devParams = @{ StartKernel = $true; RestartAgent = $true; NoChatSmoke = $true;
                    NoStripSilenceSmoke = $true; Strict = $true; WaitSeconds = $WaitSeconds;
                    RepoRoot = $RepoRoot; KernelExe = $KernelExe; AgentHttp = $AgentHttp }
    if ($SkipBuild) { $devParams["SkipBuild"] = $true }
    $devConsoleLog = Join-Path $RunRoot "dev_agent_smoke_console.log"
    & $devSmoke @devParams 2>&1 | Tee-Object -FilePath $devConsoleLog | Out-Null
    if ($LASTEXITCODE -ne 0) { throw ("dev_agent_smoke bring-up failed exit=" + $LASTEXITCODE + " (see dev_agent_smoke_console.log)") }
    $stackPids = Get-StackPids
    $report.stack_pids = $stackPids
    try { $report.agent_sha256 = (Get-FileHash -LiteralPath (Join-Path $RepoRoot "agent\bin\VitAgent.exe") -Algorithm SHA256).Hash } catch { $report.agent_sha256 = "unavailable" }
    Write-Ok ("stack up pids=" + (($stackPids.GetEnumerator() | ForEach-Object { $_.Key + "=" + ($_.Value -join ",") }) -join " "))

    # agent health
    $state = $null
    $healthDeadline = (Get-Date).AddSeconds(45)
    while ((Get-Date) -lt $healthDeadline -and ($null -eq $state -or [string]$state.status -ne "ok")) {
        try { $state = Invoke-RestMethod -Method GET -Uri ($AgentHttp.TrimEnd("/") + "/agent/state") -TimeoutSec 10 } catch { Start-Sleep -Seconds 3 }
        if ($null -eq $state) { Start-Sleep -Seconds 3 }
    }
    if ($null -eq $state -or [string]$state.status -ne "ok") { throw "agent health check failed" }
    Write-Ok ("agent ok tool_count=" + [string]$state.tool_count)

    # ---------------- phase 3: driver leg
    Write-Step ("Phase 3: driver leg (" + $Mode + ")")
    $Driver = Join-Path $RepoRoot "scripts\plugin_control_eval_driver.py"
    $AgentLog = Join-Path $RepoRoot "VitApp\Workspace\Logs\agent_last.log"
    $driverArgs = @( $Driver )
    switch ($Mode) {
        "s0"        { $driverArgs += @("s0", "--agent-http", $AgentHttp, "--out-dir", $RunRoot) }
        "dryround"  { $driverArgs += @("dryround", "--agent-http", $AgentHttp, "--out-dir", $RunRoot) }
        "segment-p" { $driverArgs += @("segment-p", "--agent-http", $AgentHttp, "--run-dir", $RunRoot,
                                       "--telemetry-path", $env:VIT_AGENT_LLM_TELEMETRY_PATH,
                                       "--agent-log", $AgentLog) }
    }
    $driverLog = Join-Path $RunRoot ("driver_" + $Mode + "_console.log")
    & python @driverArgs 2>&1 | Tee-Object -FilePath $driverLog | Out-Null
    $driverExit = $LASTEXITCODE
    $report.driver_exit = $driverExit
    $report.run_root = $RunRoot
    $report.verdict = switch ($driverExit) {
        0 { "driver_complete" }
        2 { "environment_interrupt" }
        3 { "segment_stop_budget" }
        default { "driver_failed" }
    }
    # copy agent log + telemetry evidence into the run dir (independent of the
    # workspace files; per-run copies, never overwriting older runs)
    foreach ($src in @($AgentLog, (Join-Path $LogsDir "agent_llm_telemetry.jsonl"),
                       (Join-Path $LogsDir "agent_llm_payload_capture.jsonl"))) {
        if (Test-Path -LiteralPath $src) {
            Copy-Item -LiteralPath $src -Destination (Join-Path $LogsDir ("copy_" + (Split-Path -Leaf $src))) -Force
        }
    }
}
catch {
    $report.verdict = "wrapper_fail"
    $report.error = $_.Exception.Message
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
    $report.finished_at = (Get-Date).ToString("o")
    Save-Json $report "run_report.json"
    Write-Host ("[pce] run_root: " + $RunRoot)
    Write-Host ("[pce] verdict: " + $report.verdict + " driver_exit=" + $driverExit)
}
if ($report.verdict -eq "driver_complete" -and $driverExit -eq 0) { exit 0 }
if ($report.verdict -eq "environment_interrupt" -or $report.verdict -eq "stack_occupied") { exit 2 }
if ($report.verdict -eq "segment_stop_budget") { exit 3 }
exit 1
