<#
FS_LARGEPROJECT real-stack smoke (card FS-LARGEPROJECT-SMOKE-1, pool #45).

Proves the pull-mode free-state chain on the real three-piece stack
(VitApp kernel + Godot UI + Go agent) against a REAL large project:
the 61-track sattelites training folder (3.8GB, E:\ read-only source):

  0. clear-field preflight (constraint 4, BOUNDARY-PERSIST-1 observation B):
     refuse to start unless every VitAgent/VitApp/Godot-frontend process and
     every stack port (7878/5555/5556) is already gone - the push->pull
     dual-process window is the known runtime-state lease contention bed;
  1. bring the stack up via dev_agent_smoke.ps1 -StartKernel -StartUI with
     VIT_DAW_HARNESS=pull + VIT_AGENT_LLM_TELEMETRY_PATH injected BEFORE the
     call, so the agent starts in pull mode from the first process (the
     dual-mode entry wiring face; plain /agent/chat messages route through
     agentloop Start -> startPull, never the direct-chat LLM pipeline);
  2. isolated project + full 61-track import through the SAME command chain
     as L2-2-SEG-SMOKE-1 (project.new -> project.save_as <run dir> ->
     project.import_preflight -> project.import_folder_as_stems, everything
     written under the run dir; E:\ source is read-only authoritative media);
  3. bake preheat with an EXPLICIT budget: poll the import's analysis job
     until dad facts are ready (ready>=total, status=ready). Budget expiry
     or a stalled pool = environment interruption class (NOT a functional
     failure), recorded with the full poll history as raw evidence;
  4. authority grant (full_project_access), then ONE real-LLM free-state
     round through the goal entry with the B9/J3-era utterance
     ("检查一下当前工程有什么问题吗" verbatim) + bounded fixed nudges
     ("然后继续"), pull observation budget injected in the chat context
     (max_cycles, long-range by card requirement);
  5. four assertion groups (all must pass for exit 0):
     A1 mode/telemetry face: telemetry file carries >=1 source=pullharness
        record with a non-empty prompt_fingerprint (proves pull drove it);
     A2 free-state station map: trajectory.round.started >=1 (diagnostic
        round) AND trajectory.observation.recorded >=1 (CCB observation)
        AND (trajectory.hypothesis.proposed >=1 OR mix_tick.pending >=1)
        in the conversation event stream - the free-state-era output shape;
     A3 AB audition round trip: confirmation card mounted -> >=1 answered
        approve hop -> audition.* events >=1 -> a response with
        stop_reason=mix_tick_applied_reobserved and goal_status=completed
        (assertion template = G3-ATTRIB-1 harness_ab_confirm_roundtrip.json);
     A4 zero fixed orchestration: the C1/C2/B4 marker families
        (semantic_eq_batch / eq_plugin_load_batch / dynamic_plugin_load_
        batch / dynamic_parameter_batch / c2_dynamic_plugin_selection)
        have ZERO hits across the chat/hop response JSONs, the event
        streams and the agent log (fastpath preflight is harness plumbing
        and is NOT in the marker set).

Probabilistic discipline pre-written (AGENTS.md section 8, card section
"概率运行纪律"):
  runs: <=2 invocations of this script (fresh berth each run); success =
     exit 0 with all four assertion groups; failure classes recorded
     separately, never merged: no_candidate_found / capability_blocked /
     llm_error / budget_exhausted / environment_interrupt (bake backlog
     timeout, port conflict, ... - always with raw evidence).
  stop-loss: the same deterministic breakpoint failing twice in a row =
     stop and hand up, no same-shape rerun.
  budget: max_cycles is injected EXPLICITLY (long-range; 61-track whole-song
     judgement is a multi-round task); budget_exhausted is a legal terminal
     class, but assertion groups 2/3 must already hold at the stop-loss
     point to count as partial evidence (recorded as such, honestly split
     between "station map before stop-loss" and "full chain completed").

Usage:
  powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\fs_largeproject_smoke.ps1
  powershell ... -SkipBuild     # reuse the installed agent binary
  powershell ... -KeepStack     # leave the stack up after the run

Exit codes: 0 = PASS (four assertion groups), 1 = assertion/functional
failure, 2 = environment interruption (stack occupied / bring-up failed /
bake preheat budget exhausted / E:\ source integrity violation).
Artifacts land under coord\runs\FS-LARGEPROJECT-SMOKE-1\fs_largeproject_smoke_<stamp>\
(fresh dir per run, never overwritten).
#>

[CmdletBinding()]
param(
    [string]$RepoRoot = "",
    [string]$AgentHttp = "http://127.0.0.1:7878",
    [string]$AgentHttpAddr = "127.0.0.1:7878",
    [string]$ZmqReqPort = "5555",
    [string]$ZmqSubPort = "5556",
    [string]$KernelExe = "",
    # E:\ read-only authoritative source (card section 10 red line).
    [string]$TrainingFolder = "E:\BaiduNetdiskDownload\yingge - sattelites tracks out",
    [int]$ExpectedTrackCount = 61,
    # TIM-SMOKE lesson: cold kernel/plugin-table start can exceed 20s.
    [int]$WaitSeconds = 90,
    # Import of 61 x 3.8GB: header reads + track/clip creation.
    [int]$ImportTimeoutSec = 1800,
    [int]$ImportCommandTimeoutMs = 600000,
    # Bake preheat (explicit budget; expiry = environment interruption).
    [int]$BakePreheatSeconds = 9000,
    [int]$PreheatPollSeconds = 30,
    [int]$PreheatStallMinutes = 30,
    # Free-state NL round budgets (long-range by card requirement).
    [int]$TurnSeconds = 480,
    [int]$SettleSeconds = 2700,
    [int]$MaxNudges = 6,
    [int]$MaxHops = 4,
    [int]$PullMaxCycles = 40,
    # /agent/events server cap (agentEventBufferLimit): above it the query
    # resets to 120, so 500 is the real maximum.
    [int]$EventsLimit = 500,
    [switch]$SkipBuild,
    [switch]$KeepStack
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

# ---------------------------------------------------------------- helpers
function Resolve-RepoRoot {
    param([string]$Explicit)
    if (-not [string]::IsNullOrWhiteSpace($Explicit)) {
        return (Resolve-Path -LiteralPath $Explicit).Path
    }
    return (Split-Path -Parent (Split-Path -Parent $MyInvocation.ScriptName))
}

function Write-Step { param([string]$Message) Write-Host "" ; Write-Host ("== " + $Message + " ==") -ForegroundColor Cyan }
function Write-Ok { param([string]$Message) Write-Host ("   [ok] " + $Message) -ForegroundColor Green }
function Write-WarnLine { param([string]$Message) Write-Host ("   [warn] " + $Message) -ForegroundColor Yellow }
function Write-FailLine { param([string]$Message) Write-Host ("   [FAIL] " + $Message) -ForegroundColor Red }

function Get-OptionalProperty {
    param($Object, [string]$Name)
    if ($null -eq $Object) { return $null }
    $adapted = $Object -as [System.Management.Automation.PSObject]
    if ($null -ne $adapted) {
        foreach ($prop in $adapted.PSObject.Properties) {
            if ($prop.Name -eq $Name) { return $prop.Value }
        }
    }
    return $null
}

function Get-FirstPropertyValue {
    param([object]$Object, [string[]]$Names)
    foreach ($name in $Names) {
        $value = Get-OptionalProperty -Object $Object -Name $name
        if ($null -ne $value -and -not [string]::IsNullOrWhiteSpace([string]$value)) {
            return $value
        }
    }
    return ""
}

function Invoke-Json {
    param(
        [ValidateSet("GET", "POST")]
        [string]$Method,
        [string]$Uri,
        [object]$Body = $null,
        [int]$TimeoutSec = 30
    )
    if ($Method -eq "GET") {
        $resp = Invoke-WebRequest -UseBasicParsing -Method GET -Uri $Uri -TimeoutSec $TimeoutSec
    }
    else {
        $json = $Body | ConvertTo-Json -Depth 20 -Compress
        $bytes = [System.Text.Encoding]::UTF8.GetBytes($json)
        $resp = Invoke-WebRequest -UseBasicParsing -Method POST -Uri $Uri -Body $bytes -ContentType "application/json; charset=utf-8" -TimeoutSec $TimeoutSec
    }
    if ([string]::IsNullOrWhiteSpace($resp.Content)) {
        return $null
    }
    return $resp.Content | ConvertFrom-Json
}

# Tool-face variant that never throws on non-2xx: /agent/invoke replies carry
# their failure semantics in the JSON body, and Windows PowerShell 5.1 drains
# the error-response stream before catch blocks can read it (dev_agent_smoke
# Invoke-JsonTolerant precedent). Tool failures stay recorded JSON, never
# uncaught exceptions that would skip the teardown path.
function Invoke-ToolJson {
    param([string]$Uri, [object]$Body = $null, [int]$TimeoutSec = 30)
    Add-Type -AssemblyName System.Net.Http | Out-Null
    $client = New-Object System.Net.Http.HttpClient
    $client.Timeout = [TimeSpan]::FromSeconds($TimeoutSec)
    try {
        $json = $Body | ConvertTo-Json -Depth 20 -Compress
        $content = New-Object System.Net.Http.StringContent ($json, [System.Text.Encoding]::UTF8, "application/json")
        $post = $client.PostAsync($Uri, $content).GetAwaiter().GetResult()
        $text = $post.Content.ReadAsStringAsync().GetAwaiter().GetResult()
        if ([string]::IsNullOrWhiteSpace($text)) {
            return $null
        }
        return $text | ConvertFrom-Json
    }
    finally {
        $client.Dispose()
    }
}
# Read-only source manifest (name/size/mtimeUtc) for the E:\ integrity check.
function Get-SourceManifest {
    param([string]$Folder)
    $rows = New-Object System.Collections.Generic.List[object]
    foreach ($file in @(Get-ChildItem -LiteralPath $Folder -File | Sort-Object Name)) {
        $rows.Add([pscustomobject]@{
            name = $file.Name
            size = [int64]$file.Length
            mtime_utc_ticks = [int64]$file.LastWriteTimeUtc.Ticks
        })
    }
    return $rows
}

function Compare-SourceManifest {
    param($Before, $After)
    if (@($Before).Count -ne @($After).Count) {
        return ("file_count_changed before=" + @($Before).Count + " after=" + @($After).Count)
    }
    for ($i = 0; $i -lt @($Before).Count; $i++) {
        if ($Before[$i].name -ne $After[$i].name -or $Before[$i].size -ne $After[$i].size -or $Before[$i].mtime_utc_ticks -ne $After[$i].mtime_utc_ticks) {
            return ("entry_changed index=" + $i + " before=" + ($Before[$i] | ConvertTo-Json -Compress) + " after=" + ($After[$i] | ConvertTo-Json -Compress))
        }
    }
    return ""
}

# Tear down the stack this run owns (agent via HTTP port, kernel via REQ
# port, Godot frontend by project path). Best effort: never masks the
# verdict. Post-teardown verification is recorded separately.
function Clear-OwnedStack {
    foreach ($port in @(@{ port = 7878; label = "agent" }, @{ port = 5555; label = "kernel" })) {
        try {
            $listener = Get-NetTCPConnection -LocalPort $port.port -State Listen -ErrorAction SilentlyContinue | Select-Object -First 1
            if ($null -ne $listener) {
                Stop-Process -Id $listener.OwningProcess -Force -ErrorAction SilentlyContinue
                Write-Host ("   [cleanup] stopped " + $port.label + " pid=" + $listener.OwningProcess)
            }
        }
        catch { Write-WarnLine ("cleanup " + $port.label + " failed: " + $_.Exception.Message) }
    }
    try {
        $godotProcs = Get-CimInstance Win32_Process -Filter "Name LIKE 'Godot%'" -ErrorAction SilentlyContinue |
            Where-Object { $_.CommandLine -like "*vit-daw-frontend*" }
        foreach ($proc in @($godotProcs)) {
            Stop-Process -Id $proc.ProcessId -Force -ErrorAction SilentlyContinue
            Write-Host ("   [cleanup] stopped godot pid=" + $proc.ProcessId)
        }
    }
    catch { Write-WarnLine ("cleanup godot failed: " + $_.Exception.Message) }
}

function Test-StackGone {
    $remaining = @()
    foreach ($p in @(7878, 5555, 5556)) {
        $l = Get-NetTCPConnection -LocalPort $p -State Listen -ErrorAction SilentlyContinue | Select-Object -First 1
        if ($null -ne $l) { $remaining += ("port " + $p + " pid=" + $l.OwningProcess) }
    }
    foreach ($n in @("VitAgent", "VitAgent.dev-smoke", "VitApp")) {
        $procs = Get-Process -Name $n -ErrorAction SilentlyContinue
        foreach ($proc in @($procs)) { $remaining += ("process " + $n + " pid=" + $proc.Id) }
    }
    $godot = Get-CimInstance Win32_Process -Filter "Name LIKE 'Godot%'" -ErrorAction SilentlyContinue |
        Where-Object { $_.CommandLine -like "*vit-daw-frontend*" }
    foreach ($g in @($godot)) { $remaining += ("godot pid=" + $g.ProcessId) }
    return $remaining
}

# ---------------------------------------------------------------- setup
$RepoRoot = Resolve-RepoRoot -Explicit $RepoRoot
$RunStamp = Get-Date -Format "yyyyMMdd_HHmmss"
$RunRoot = Join-Path $RepoRoot ("coord\runs\FS-LARGEPROJECT-SMOKE-1\fs_largeproject_smoke_" + $RunStamp)
if (Test-Path -LiteralPath $RunRoot) {
    throw ("run artifacts directory already exists; each run needs a fresh directory: " + $RunRoot)
}
New-Item -ItemType Directory -Force -Path $RunRoot | Out-Null
$ProjectDir = Join-Path $RunRoot "project"
New-Item -ItemType Directory -Force -Path $ProjectDir | Out-Null
$IsolatedProjectPath = Join-Path $ProjectDir "sattelites_61.vit"
$TelemetryFile = Join-Path $RunRoot "telemetry_pull.jsonl"

$failureReasons = New-Object System.Collections.Generic.List[string]
function Add-Failure { param([string]$Message) $script:failureReasons.Add($Message); Write-FailLine $Message }

$report = [ordered]@{
    run_id = "fs_largeproject_smoke_" + $RunStamp
    card = "FS-LARGEPROJECT-SMOKE-1"
    started_at = (Get-Date).ToUniversalTime().ToString("o")
    command_line = ($MyInvocation.Line)
    repo_head = ""
    worktree = $RepoRoot
    branch = ""
    kernel_exe = ""
    kernel_sha256 = ""
    agent_binary = ""
    stack_mode = ""
    harness_env = "pull"
    telemetry_file = $TelemetryFile
    probabilistic_discipline = @{
        max_runs = 2
        success_condition = "exit 0 with assertion groups A1-A4 all true"
        failure_taxonomy = @("no_candidate_found", "capability_blocked", "llm_error", "budget_exhausted", "environment_interrupt")
        stop_loss = "same deterministic breakpoint failing twice in a row -> stop and hand up"
    }
    budget = @{
        bake_preheat_seconds = $BakePreheatSeconds
        pull_max_cycles = $PullMaxCycles
        turn_seconds = $TurnSeconds
        settle_seconds = $SettleSeconds
        max_nudges = $MaxNudges
        max_hops = $MaxHops
    }
    import = $null
    preheat = $null
    nl_round = $null
    confirm_roundtrip = $null
    telemetry_census = $null
    assertions_block = $null
    classification = ""
    source_integrity = ""
    environment_interrupt = ""
    verdict = ""
    teardown = $null
}
try { $report.repo_head = (git -C $RepoRoot rev-parse HEAD 2>$null | Out-String).Trim() } catch { $report.repo_head = "unavailable" }
try { $report.branch = (git -C $RepoRoot rev-parse --abbrev-ref HEAD 2>$null | Out-String).Trim() } catch { $report.branch = "unavailable" }
function Write-Report {
    $report.finished_at = (Get-Date).ToUniversalTime().ToString("o")
    $report | ConvertTo-Json -Depth 14 | Out-File -FilePath (Join-Path $RunRoot "run_report.json") -Encoding utf8
}
(git -C $RepoRoot rev-parse HEAD 2>$null | Out-String).Trim() | Set-Content -LiteralPath (Join-Path $RunRoot "head.txt") -Encoding UTF8
(git -C $RepoRoot status --short 2>$null | Out-String) | Set-Content -LiteralPath (Join-Path $RunRoot "git_status.txt") -Encoding UTF8

Write-Step "FS large-project free-state smoke (pull mode, sattelites 61 tracks)"
Write-Host ("run_root: " + $RunRoot)
Write-Host ("repo HEAD: " + $report.repo_head + " branch: " + $report.branch)

# Kernel binary: the staging runtime kernel (G3-ATTRIB-1 same-source form;
# the C++ face is untouched by this card, zero agent/Go code changes).
# Export/ is gitignored, so an execution worktree carries no kernel build:
# resolve the worktree path first, then the main checkout staging build.
if ([string]::IsNullOrWhiteSpace($KernelExe)) {
    $KernelExe = Join-Path $RepoRoot "Export\staging\runtime\VitApp.exe"
    if (-not (Test-Path -LiteralPath $KernelExe)) {
        $KernelExe = "D:\Vit_DAW\Export\staging\runtime\VitApp.exe"
    }
}
$KernelExe = (Resolve-Path -LiteralPath $KernelExe).Path
if (-not (Test-Path -LiteralPath $KernelExe)) {
    throw ("kernel exe not found: " + $KernelExe)
}
$report.kernel_exe = $KernelExe
try { $report.kernel_sha256 = (Get-FileHash -LiteralPath $KernelExe -Algorithm SHA256).Hash } catch { $report.kernel_sha256 = "unavailable" }
$TrainingFolder = (Resolve-Path -LiteralPath $TrainingFolder).Path
if (-not (Test-Path -LiteralPath $TrainingFolder -PathType Container)) {
    throw ("training folder not found: " + $TrainingFolder)
}
Write-Host ("kernel: " + $KernelExe)
Write-Host ("kernel sha256: " + $report.kernel_sha256)
Write-Host ("source folder (read-only): " + $TrainingFolder)

# ---------------------------------------------------- phase 0: clear field
# Constraint 4 (BOUNDARY-PERSIST-1 observation B): the push->pull dual
# process window is the runtime-state lease contention bed. Refuse to start
# on ANY residual agent/kernel/Godot process or stack port, no reuse.
Write-Step "Phase 0: clear-field preflight (no residual stack)"
# PS 5.1: a function returning an empty array unrolls to $null in the
# pipeline - capture with @() or StrictMode .Count throws.
$residual = @(Test-StackGone)
if ($residual.Count -gt 0) {
    foreach ($r in @($residual)) { Write-FailLine ("residual stack element: " + $r) }
    $report.verdict = "stack_occupied"
    $report.environment_interrupt = "residual stack present before bring-up: " + (@($residual) -join "; ")
    Write-Report
    exit 2
}
Write-Ok "clear field: ports 7878/5555/5556 free, no VitAgent/VitApp/Godot-frontend processes"

# E:\ source integrity baseline (read-only manifest; compared again after
# import+preheat and at the end of the run).
$sourceManifestBefore = @(Get-SourceManifest -Folder $TrainingFolder)
@($sourceManifestBefore) | ConvertTo-Json -Depth 4 | Out-File -FilePath (Join-Path $RunRoot "source_manifest_before.json") -Encoding utf8
Write-Ok ("source manifest: " + $sourceManifestBefore.Count + " files (read-only baseline recorded)")
if ($sourceManifestBefore.Count -ne $ExpectedTrackCount) {
    Add-Failure ("source folder carries " + $sourceManifestBefore.Count + " files, expected " + $ExpectedTrackCount)
}

# ------------------------------------------- phase 1: pull env + stack bring-up
Write-Step "Phase 1: bring up real stack via dev_agent_smoke (kernel + Godot UI + agent, pull env)"
Set-Item -LiteralPath env:VIT_DAW_HARNESS -Value "pull"
Set-Item -LiteralPath env:VIT_AGENT_LLM_TELEMETRY_PATH -Value $TelemetryFile
# Journey-berth isolation precedent (dev_agent_smoke journey berth): the
# agent's conversation draft root is redirected into the run dir so every
# durable write of this run lands under coord\runs\... (also avoids the
# user-AppData draft sessions of aborted runs piling up).
$AgentDraftsDir = Join-Path $RunRoot "agent_drafts"
New-Item -ItemType Directory -Force -Path $AgentDraftsDir | Out-Null
Set-Item -LiteralPath env:VIT_HISTORY_DRAFT_ROOT -Value $AgentDraftsDir
@{
    harness_env = "VIT_DAW_HARNESS=pull"
    telemetry_env = "VIT_AGENT_LLM_TELEMETRY_PATH=" + $TelemetryFile
    draft_root_env = "VIT_HISTORY_DRAFT_ROOT=" + $AgentDraftsDir
    injected_at = (Get-Date).ToString("o")
} | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath (Join-Path $RunRoot "pull_env.json") -Encoding UTF8

$devSmoke = Join-Path $RepoRoot "scripts\dev_agent_smoke.ps1"
$devParams = @{
    StartKernel = $true
    StartUI = $true
    NoChatSmoke = $true
    NoStripSilenceSmoke = $true
    Strict = $true
    WaitSeconds = $WaitSeconds
    RepoRoot = $RepoRoot
    KernelExe = $KernelExe
}
if ($SkipBuild) { $devParams["SkipBuild"] = $true }
$devConsoleLog = Join-Path $RunRoot "dev_agent_smoke_console.log"
$devExit = 0
$bringupStarted = Get-Date
try {
    & $devSmoke @devParams 2>&1 | Tee-Object -FilePath $devConsoleLog
    if (-not $?) { $devExit = 1 }
}
catch {
    $devExit = 1
    Write-FailLine ("dev_agent_smoke failed: " + $_.Exception.Message)
    $_.Exception.Message | Set-Content -LiteralPath (Join-Path $RunRoot "dev_agent_smoke_error.txt") -Encoding UTF8
}
$report.stack_mode = "dev_agent_smoke_started"
$report.bringup_seconds = [int]((Get-Date) - $bringupStarted).TotalSeconds
if ($devExit -ne 0) {
    Add-Failure "dev_agent_smoke stack bring-up failed (see dev_agent_smoke_console.log)"
    $report.verdict = "stack_bringup_failed"
    $report.environment_interrupt = "dev_agent_smoke bring-up failed; console log preserved"
    Write-Report
    if (-not $KeepStack) { Clear-OwnedStack }
    exit 2
}
Write-Ok ("stack up (bringup " + [string]$report.bringup_seconds + "s, agent inherits VIT_DAW_HARNESS=pull)")

# Agent health + pull mode sanity on the running agent.
$InvokeUri = $AgentHttp.TrimEnd("/") + "/agent/invoke"
$ChatUri = $AgentHttp.TrimEnd("/") + "/agent/chat"
$AuthorityUri = $AgentHttp.TrimEnd("/") + "/agent/authority"
$RuntimeUri = $AgentHttp.TrimEnd("/") + "/agent/runtime/status"
$EventsUri = $AgentHttp.TrimEnd("/") + "/agent/events"
$RespondUri = $AgentHttp.TrimEnd("/") + "/agent/interaction/respond"
$ConfirmUri = $AgentHttp.TrimEnd("/") + "/agent/confirm"
$invoke = {
    param([string]$Tool, [object]$ToolArgs, [int]$TimeoutSec)
    Invoke-ToolJson -Uri $InvokeUri -Body @{
        tool = $Tool
        args = $ToolArgs
        confirmed = $true
        source = "fs_largeproject_smoke"
    } -TimeoutSec $TimeoutSec
}
Write-Step "Agent health"
$state = $null
$healthDeadline = (Get-Date).AddSeconds(45)
while ((Get-Date) -lt $healthDeadline) {
    try {
        $state = Invoke-Json -Method GET -Uri ($AgentHttp.TrimEnd("/") + "/agent/state") -TimeoutSec 10
        if ($null -ne $state -and [string](Get-OptionalProperty -Object $state -Name "status") -eq "ok") { break }
    }
    catch { Start-Sleep -Seconds 3 }
    if ($null -eq $state) { Start-Sleep -Seconds 3 }
}
if ($null -eq $state -or [string](Get-OptionalProperty -Object $state -Name "status") -ne "ok") {
    Add-Failure "GET /agent/state did not return ok (agent not reachable)"
    $report.verdict = "agent_unreachable"
    $report.environment_interrupt = "agent health check failed after bring-up"
    Write-Report
    if (-not $KeepStack) { Clear-OwnedStack }
    exit 2
}
Write-Ok ("agent ok tool_count=" + [string](Get-OptionalProperty -Object $state -Name "tool_count"))

# Safety net (run-1/run-3 lesson): every uncaught throw below used to kill
# the script BEFORE the teardown path. One try around the whole post-health
# flow guarantees any exception still writes the report, tears the stack
# down and exits with the functional-failure code.
try {

# ------------------------------- phase 2: isolated project + 61-track import
# Command order on the AGENT invoke face follows the G3-ATTRIB-1 harness_ab
# fixture precedent: project.new -> import -> (bake) -> save_as. The L2-2
# ZMQ chain saved BEFORE importing, but on the agent face an immediate
# save_as races the draft session workspace materialization (run 1 evidence:
# agent_runtime_state.lock.json missing one second after project.new) - the
# journey/G3 fixtures all persist AFTER the import. The isolation essence of
# the card (isolated .vit in the run dir, E:\ read-only) is unchanged.
Write-Step "Phase 2: full 61-track stems import (L2-2 command chain, agent-face order)"
$importStarted = Get-Date
$null = & $invoke "project.new" @{} 60

$preflight = & $invoke "project.import_preflight" @{
    folder_path = $TrainingFolder
    recursive = $false
    media_kinds = @("audio")
    intended_mode = "stems_folder"
    target_policy = "create_tracks"
    start_time_seconds = 0
    command_timeout_ms = $ImportCommandTimeoutMs
} $ImportTimeoutSec
$preflight | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath (Join-Path $RunRoot "import_preflight.json") -Encoding UTF8
if ([string]$preflight.status -ne "ok") {
    Add-Failure ("project.import_preflight failed: " + ($preflight | ConvertTo-Json -Depth 6 -Compress))
}
# Agent-face reply wraps the kernel payload in "result" (run-2 evidence:
# summary lives at result.summary; top-level has only status/tool metadata).
$preflightResult = Get-OptionalProperty -Object $preflight -Name "result"
if ($null -eq $preflightResult) { $preflightResult = $preflight }
$preflightSummary = Get-OptionalProperty -Object $preflightResult -Name "summary"
if ($null -eq $preflightSummary) { $preflightSummary = Get-OptionalProperty -Object $preflight -Name "summary" }
$readableCount = [int](Get-FirstPropertyValue -Object $preflightSummary -Names @("readable_file_count"))
$tracksToCreate = [int](Get-FirstPropertyValue -Object $preflightSummary -Names @("tracks_to_create"))
if ($readableCount -ne $ExpectedTrackCount -or $tracksToCreate -ne $ExpectedTrackCount) {
    Add-Failure ("preflight counts mismatch: readable=" + $readableCount + " tracks_to_create=" + $tracksToCreate + " expected=" + $ExpectedTrackCount)
}

$imported = & $invoke "project.import_folder_as_stems" @{
    folder_path = $TrainingFolder
    recursive = $false
    target_policy = "create_tracks"
    start_time_seconds = 0.0
    confirmed = $true
    skip_unreadable = $false
    command_timeout_ms = $ImportCommandTimeoutMs
} $ImportTimeoutSec
$imported | ConvertTo-Json -Depth 12 | Set-Content -LiteralPath (Join-Path $RunRoot "import_folder_as_stems.json") -Encoding UTF8
$importResult = Get-OptionalProperty -Object $imported -Name "result"
if ($null -eq $importResult) { $importResult = $imported }
$importSummary = Get-OptionalProperty -Object $importResult -Name "summary"
$tracksCreated = [int](Get-FirstPropertyValue -Object $importResult -Names @("tracks_created"))
if ($tracksCreated -le 0) { $tracksCreated = [int](Get-FirstPropertyValue -Object $importSummary -Names @("tracks_created")) }
$clipsCreated = [int](Get-FirstPropertyValue -Object $importResult -Names @("clips_created"))
if ($clipsCreated -le 0) { $clipsCreated = [int](Get-FirstPropertyValue -Object $importSummary -Names @("clips_created")) }
$analysisJobId = [string](Get-FirstPropertyValue -Object $importResult -Names @("analysis_job_id"))
if ([string]::IsNullOrWhiteSpace($analysisJobId)) {
    $importJob = Get-OptionalProperty -Object $importResult -Name "analysis_job"
    $analysisJobId = [string](Get-FirstPropertyValue -Object $importJob -Names @("analysis_job_id", "job_id"))
}
$report.import = @{
    started_at = $importStarted.ToString("o")
    readable_file_count = $readableCount
    tracks_to_create = $tracksToCreate
    tracks_created = $tracksCreated
    clips_created = $clipsCreated
    analysis_job_id = $analysisJobId
    isolated_project = $IsolatedProjectPath
    elapsed_seconds = [int]((Get-Date) - $importStarted).TotalSeconds
}
if ([string]$imported.status -ne "ok" -or $tracksCreated -ne $ExpectedTrackCount -or $clipsCreated -ne $ExpectedTrackCount) {
    Add-Failure ("import mismatch: status=" + [string]$imported.status + " tracks=" + $tracksCreated + " clips=" + $clipsCreated + " expected=" + $ExpectedTrackCount)
}
if ([string]::IsNullOrWhiteSpace($analysisJobId)) {
    Add-Failure "import returned no analysis_job_id (bake preheat cannot proceed)"
}
if ($failureReasons.Count -gt 0) {
    $report.verdict = "import_failed"
    Write-Report
    if (-not $KeepStack) { Clear-OwnedStack }
    exit 1
}
Write-Ok ("import complete: tracks=" + $tracksCreated + " clips=" + $clipsCreated + " analysis_job=" + $analysisJobId + " (" + [string]$report.import.elapsed_seconds + "s)")

# ---------------------------------------------------- phase 3: bake preheat
Write-Step "Phase 3: bake preheat (explicit budget, L2-2 serialized L3 pool)"
$null = & $invoke "project.audio_analysis_start" @{ analysis_job_id = $analysisJobId; interval_ms = 10 } 120
$preheatStarted = Get-Date
$polls = New-Object System.Collections.Generic.List[object]
$ready = $false
$lastReadyCount = -1
$stallPolls = 0
$maxStallPolls = [int][Math]::Max(2, [int]($PreheatStallMinutes * 60 / $PreheatPollSeconds))
$preheatPollLog = Join-Path $RunRoot "preheat_polls.jsonl"
$deadline = (Get-Date).AddSeconds($BakePreheatSeconds)
while ((Get-Date) -lt $deadline) {
    Start-Sleep -Seconds $PreheatPollSeconds
    $job = $null
    try {
        $statusReply = & $invoke "project.audio_analysis_status" @{ analysis_job_id = $analysisJobId; latest = $true } 120
        $statusResult = Get-OptionalProperty -Object $statusReply -Name "result"
        $job = Get-OptionalProperty -Object $statusResult -Name "analysis_job"
        if ($null -eq $job) { $job = Get-OptionalProperty -Object $statusReply -Name "analysis_job" }
    }
    catch { $job = $null }
    $pending = [int](Get-FirstPropertyValue -Object $job -Names @("pending_clips"))
    $submittedClips = [int](Get-FirstPropertyValue -Object $job -Names @("submitted_clips"))
    $submittedFeatures = [int](Get-FirstPropertyValue -Object $job -Names @("submitted_feature_jobs"))
    $dadTotal = [int](Get-FirstPropertyValue -Object $job -Names @("dad_fact_total_count", "dad_fact_total"))
    $dadReady = [int](Get-FirstPropertyValue -Object $job -Names @("dad_fact_ready_count"))
    $dadStatus = [string](Get-FirstPropertyValue -Object $job -Names @("dad_fact_status"))
    $queueStatus = [string](Get-FirstPropertyValue -Object $job -Names @("analysis_queue_status"))
    $row = @{
        at = (Get-Date).ToUniversalTime().ToString("o")
        elapsed_s = [int]((Get-Date) - $preheatStarted).TotalSeconds
        pending_clips = $pending
        submitted_clips = $submittedClips
        submitted_feature_jobs = $submittedFeatures
        dad_fact_total = $dadTotal
        dad_fact_ready = $dadReady
        dad_fact_status = $dadStatus
        analysis_queue_status = $queueStatus
    }
    $polls.Add($row)
    ($row | ConvertTo-Json -Compress) | Add-Content -LiteralPath $preheatPollLog -Encoding UTF8
    if ($dadTotal -gt 0 -and $dadReady -ge $dadTotal -and $dadStatus.ToLower() -eq "ready") {
        $ready = $true
        break
    }
    if ($dadReady -gt $lastReadyCount -or $pending -gt 0) {
        $stallPolls = 0
    }
    else {
        $stallPolls++
        if ($stallPolls -ge $maxStallPolls) { break }
    }
    $lastReadyCount = $dadReady
}
$report.preheat = @{
    budget_seconds = $BakePreheatSeconds
    poll_interval_seconds = $PreheatPollSeconds
    elapsed_seconds = [int]((Get-Date) - $preheatStarted).TotalSeconds
    polls = @($polls.ToArray())
    dad_ready = $ready
    final = if ($polls.Count -gt 0) { $polls[$polls.Count - 1] } else { $null }
}
if (-not $ready) {
    $finalRow = if ($polls.Count -gt 0) { $polls[$polls.Count - 1] } else { $null }
    $stalled = ($stallPolls -ge $maxStallPolls)
    $report.verdict = "bake_preheat_environment_interrupt"
    $report.environment_interrupt = ("bake preheat not ready within budget: budget=" + $BakePreheatSeconds + "s elapsed=" + [string]$report.preheat.elapsed_seconds + "s stalled_pool=" + [string]$stalled + " final=" + ($finalRow | ConvertTo-Json -Compress))
    Write-Report
    if (-not $KeepStack) { Clear-OwnedStack }
    exit 2
}
Write-Ok ("bake preheat ready: " + [string]$report.preheat.elapsed_seconds + "s, final dad_fact " + [string]$report.preheat.final.dad_fact_ready + "/" + [string]$report.preheat.final.dad_fact_total)

# E:\ integrity mid-check after import+preheat.
$sourceManifestMid = @(Get-SourceManifest -Folder $TrainingFolder)
@($sourceManifestMid) | ConvertTo-Json -Depth 4 | Out-File -FilePath (Join-Path $RunRoot "source_manifest_mid.json") -Encoding utf8
$midDiff = Compare-SourceManifest -Before $sourceManifestBefore -After $sourceManifestMid
if ($midDiff -ne "") {
    $report.verdict = "source_integrity_violation"
    $report.source_integrity = "mid-check diff: " + $midDiff
    Add-Failure ("E:\ source integrity violation after import/preheat: " + $midDiff)
    Write-Report
    if (-not $KeepStack) { Clear-OwnedStack }
    exit 2
}
Write-Ok "E:\ source integrity mid-check clean"

# ------------------------------- phase 3.5: persist the isolated project
# Agent-face order (G3 harness_ab fixture): save AFTER import + bake, with a
# bounded retry absorbing any residual draft-session workspace race (run 1
# evidence class). The .vit then persists the imported 61-track state and the
# kernel autosave keeps it current for the rest of the run.
Write-Step "Phase 3.5: persist isolated project (save_as with bounded retry)"
$saveOk = $false
$saveAttempts = @()
foreach ($saveAttempt in 1..3) {
    $saveReply = & $invoke "project.save_as" @{ file_path = $IsolatedProjectPath } 300
    $saveReplyText = ""
    try { $saveReplyText = ($saveReply | ConvertTo-Json -Depth 8 -Compress) } catch { $saveReplyText = "<unserializable>" }
    $saveAttempts += $saveReplyText
    if ($null -ne $saveReply -and [string]$saveReply.status -eq "ok") {
        $saveOk = $true
        break
    }
    Write-WarnLine ("save_as attempt " + [string]$saveAttempt + " not ok; retrying after 10s: " + $saveReplyText)
    Start-Sleep -Seconds 10
}
$saveAttempts | Set-Content -LiteralPath (Join-Path $RunRoot "save_as_attempts.json") -Encoding UTF8
if (-not $saveOk) {
    Add-Failure ("project.save_as failed after 3 attempts; see save_as_attempts.json")
    $report.verdict = "save_as_failed"
    Write-Report
    if (-not $KeepStack) { Clear-OwnedStack }
    exit 1
}
if (-not (Test-Path -LiteralPath $IsolatedProjectPath)) {
    Add-Failure ("isolated project file missing after successful save_as: " + $IsolatedProjectPath)
    $report.verdict = "save_as_failed"
    Write-Report
    if (-not $KeepStack) { Clear-OwnedStack }
    exit 1
}
Write-Ok ("isolated project persisted: " + $IsolatedProjectPath)

# ------------------------------------- phase 4: authority + free-state NL round
Write-Step "Phase 4: authority grant (full_project_access)"
$authoritySwitch = $null
try {
    $authoritySwitch = Invoke-ToolJson -Uri $AuthorityUri -Body @{ authority_mode = "full_project_access" } -TimeoutSec 30
}
catch {
    $authoritySwitch = $null
}
if ($null -eq $authoritySwitch) {
    $authoritySwitch = [pscustomobject]@{ status = "unreachable" }
}
$authoritySwitch | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath (Join-Path $RunRoot "authority_switch.json") -Encoding UTF8
if ([string]$authoritySwitch.status -ne "ok" -or [string](Get-OptionalProperty -Object $authoritySwitch -Name "authority_mode") -ne "full_project_access") {
    Add-Failure ("authority switch failed: " + ($authoritySwitch | ConvertTo-Json -Depth 6 -Compress))
    $report.verdict = "authority_failed"
    Write-Report
    if (-not $KeepStack) { Clear-OwnedStack }
    exit 1
}
Write-Ok "authority = full_project_access"

Write-Step "Phase 5: free-state NL round via goal entry (pull mode, real LLM)"
# B9/J3-era verbatim utterance (the free-state-era same-source diagnostic
# question "检查一下当前工程有什么问题吗", verified against the UTF-8
# encoding before committing); nudge = J3 journey precedent base64, whose
# decoded text is "可以执行" (journey1 evidence-tool fixed nudge).
$nlPrompt = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String("5qOA5p+l5LiA5LiL5b2T5YmN5bel56iL5pyJ5LuA5LmI6Zeu6aKY5ZCX"))
$nlNudge = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String("5Y+v5Lul5omn6KGM"))
$nlPrompt | Set-Content -LiteralPath (Join-Path $RunRoot "nl_prompt.txt") -Encoding UTF8
$conversationID = "dev_fs_largeproject_" + $RunStamp
$nlChatContext = @{
    agent_mode = "chat"
    pull_observation_budget = @{ max_cycles = $PullMaxCycles }
}

$nlTimeline = New-Object System.Collections.Generic.List[string]
$nlStopReasons = New-Object System.Collections.Generic.List[string]
$nlGoalStatuses = New-Object System.Collections.Generic.List[string]
$nlResponseFiles = New-Object System.Collections.Generic.List[string]
$allResponses = New-Object System.Collections.Generic.List[object]
$turnsSent = 0
$nudgesSent = 0
$sendPending = $true
$terminal = $false
$lastStop = ""
$lastGoal = ""
$lastNeedsConfirmation = $false
$transportErrors = 0
$cardID = ""
$cardKind = ""
$cardSource = ""
$settleReason = "budget_exhausted"
$runtimeFinal = $null
$terminalGoals = @("completed", "failed", "stopped", "cancelled", "waiting_confirmation", "waiting_clarification")
$settleDeadline = (Get-Date).AddSeconds($SettleSeconds)
while ((Get-Date) -lt $settleDeadline -and -not $terminal) {
    if ($sendPending) {
        $sendPending = $false
        $turnsSent++
        $isNudge = ($turnsSent -gt 1)
        if ($isNudge) { $nudgesSent++ }
        $turnMessage = $nlPrompt
        if ($isNudge) { $turnMessage = $nlNudge }
        $turnResponse = $null
        $turnError = ""
        try {
            $turnResponse = Invoke-Json -Method POST -Uri $ChatUri -Body @{
                conversation_id = $conversationID
                message = $turnMessage
                context = $nlChatContext
            } -TimeoutSec $TurnSeconds
        }
        catch {
            $turnResponse = $null
            if ($null -ne $_.ErrorDetails -and -not [string]::IsNullOrWhiteSpace($_.ErrorDetails.Message)) { $turnError = $_.ErrorDetails.Message }
            else { $turnError = $_.Exception.Message }
        }
        if ($null -ne $turnResponse) {
            $allResponses.Add($turnResponse)
            $responseFile = Join-Path $RunRoot ("nl_chat_" + [string]$turnsSent + ".json")
            $turnResponse | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath $responseFile -Encoding UTF8
            $nlResponseFiles.Add($responseFile)
            $lastStop = [string](Get-OptionalProperty -Object $turnResponse -Name "stop_reason")
            $lastGoal = [string](Get-OptionalProperty -Object $turnResponse -Name "goal_status")
            $lastNeedsConfirmation = [bool](Get-OptionalProperty -Object $turnResponse -Name "needs_confirmation")
            if (-not [string]::IsNullOrWhiteSpace($lastStop)) { $nlStopReasons.Add($lastStop) }
            if (-not [string]::IsNullOrWhiteSpace($lastGoal)) { $nlGoalStatuses.Add($lastGoal) }
            $nlTimeline.Add(("chat_turn_" + [string]$turnsSent + $(if ($isNudge) { " (nudge)" } else { "" }) + " stop=" + $lastStop + " goal=" + $lastGoal + " needs_confirmation=" + [string]$lastNeedsConfirmation))
            # Pending-card discovery, source 1: the chat response's own live
            # card. Row objects only (PS 5.1 unrolling: a missing array reads
            # as one empty string row through the @() wrapper - J2 rack-row
            # trap; an empty row must not masquerade as a card).
            if (-not ($lastGoal -eq "waiting_continue" -and ($lastStop -eq "limit_reached" -or [string]::IsNullOrWhiteSpace($lastStop)))) {
                foreach ($cardRow in @(Get-OptionalProperty -Object $turnResponse -Name "interaction_requests")) {
                    if ($null -eq $cardRow -or -not ($cardRow -is [System.Management.Automation.PSCustomObject])) { continue }
                    $rowKind = [string](Get-OptionalProperty -Object $cardRow -Name "kind")
                    if ([string]::IsNullOrWhiteSpace($rowKind)) { $rowKind = [string](Get-OptionalProperty -Object $cardRow -Name "type") }
                    if ($rowKind.ToLower().Contains("confirmation")) {
                        $cardID = [string](Get-OptionalProperty -Object $cardRow -Name "id")
                        if ([string]::IsNullOrWhiteSpace($cardID)) { $cardID = [string](Get-OptionalProperty -Object $cardRow -Name "interaction_id") }
                        $cardKind = $rowKind
                        $cardSource = "chat_response"
                    }
                }
            }
        }
        else {
            $transportErrors++
            $nlStopReasons.Add("transport_error")
            $turnError | Set-Content -LiteralPath (Join-Path $RunRoot ("nl_chat_" + [string]$turnsSent + "_error.txt")) -Encoding UTF8
            $nlTimeline.Add(("chat_turn_" + [string]$turnsSent + " transport_error=" + $turnError))
            if ($turnsSent -eq 1) {
                $settleReason = "initial_transport_error"
                break
            }
        }
        # Boundary policy (J3/harness_ab shared vocabulary): terminal or
        # user-facing statuses settle; waiting_continue chains get bounded
        # nudges; observation_budget_exhausted is never nudged (legal stop).
        if ($lastGoal -in @("completed", "failed", "stopped", "cancelled") -or $lastStop -in @("done", "failed", "cancelled", "user_stop")) {
            $terminal = $true
            $settleReason = "goal_terminal"
        }
        elseif ($lastGoal -in @("waiting_confirmation", "waiting_clarification")) {
            $terminal = $true
            $settleReason = "awaiting_user"
        }
        elseif ($lastStop -eq "observation_budget_exhausted") {
            $terminal = $true
            $settleReason = "budget_exhausted"
        }
        elseif ($lastGoal -eq "waiting_continue" -or $lastStop -in @("limit_reached", "transient_llm_error", "interrupted", "transport_error")) {
            if ($nudgesSent -lt $MaxNudges) {
                $sendPending = $true
            }
            else {
                $terminal = $true
                $settleReason = "boundary_no_nudges_left"
            }
        }
        if ($transportErrors -ge 2) { $terminal = $true; $settleReason = "transport_error" }
    }
    else {
        Start-Sleep -Seconds 5
    }
    if ($terminal) { break }
    # ---- poll the durable continuation (pending-card discovery, source 2).
    $runtimePoll = $null
    try { $runtimePoll = Invoke-Json -Method GET -Uri $RuntimeUri -TimeoutSec 30 } catch { $runtimePoll = $null }
    if ($null -eq $runtimePoll) { continue }
    $runtimeFinal = $runtimePoll
    $goalProp = Get-OptionalProperty -Object $runtimePoll -Name "goal"
    $goalStatusPoll = [string](Get-OptionalProperty -Object $goalProp -Name "status")
    $walkedCardID = ""
    $walkedCardKind = ""
    $continuationActive = $false
    foreach ($contRow in @(Get-OptionalProperty -Object $runtimePoll -Name "continuations")) {
        if ($null -eq $contRow -or -not ($contRow -is [System.Management.Automation.PSCustomObject])) { continue }
        $contStatus = [string](Get-OptionalProperty -Object $contRow -Name "status")
        if ($contStatus -in @("pending", "claimed", "running")) { $continuationActive = $true }
        $pendingInteraction = Get-OptionalProperty -Object $contRow -Name "pending_interaction"
        if ($null -eq $pendingInteraction -or -not ($pendingInteraction -is [System.Management.Automation.PSCustomObject])) { continue }
        $pendingKind = [string](Get-OptionalProperty -Object $pendingInteraction -Name "kind")
        if (-not $pendingKind.ToLower().Contains("confirmation")) { continue }
        $walkedCardID = [string](Get-FirstPropertyValue -Object $pendingInteraction -Names @("interaction_id", "id"))
        $walkedCardKind = $pendingKind
    }
    if (-not [string]::IsNullOrWhiteSpace($walkedCardID)) {
        $cardID = $walkedCardID
        $cardKind = $walkedCardKind
        $cardSource = "runtime_status"
        $settleReason = "card_parked"
        $nlTimeline.Add(("settle card_parked kind=" + $cardKind + " goal_status=" + $goalStatusPoll))
        $terminal = $true
        break
    }
    if ($terminalGoals -contains $goalStatusPoll) {
        $settleReason = "goal_terminal"
        $nlTimeline.Add(("settle goal_terminal status=" + $goalStatusPoll))
        $terminal = $true
        break
    }
    # Conversational boundary with nothing active and no card: nudge (the
    # sendPending face above already handles the counter; poll loop only
    # detects the park state for the record).
    if ($goalStatusPoll -eq "waiting_continue" -and -not $continuationActive -and -not $sendPending -and $nudgesSent -lt $MaxNudges) {
        $nlTimeline.Add(("boundary: goal waiting_continue with no active continuation -> nudge " + [string]($nudgesSent + 1) + "/" + [string]$MaxNudges))
        $sendPending = $true
    }
}
if (-not $terminal -and $settleReason -eq "budget_exhausted") {
    $nlTimeline.Add(("settle budget_exhausted goal_status=" + $lastGoal))
}
if ($null -ne $runtimeFinal) {
    $runtimeFinal | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath (Join-Path $RunRoot "nl_runtime_status_final.json") -Encoding UTF8
}
$report.nl_round = @{
    conversation_id = $conversationID
    prompt_file = "nl_prompt.txt"
    settle_reason = $settleReason
    turns_sent = $turnsSent
    nudges_sent = $nudgesSent
    transport_errors = $transportErrors
    stop_reasons = @($nlStopReasons.ToArray())
    goal_statuses = @($nlGoalStatuses.ToArray())
    last_stop_reason = $lastStop
    last_goal_status = $lastGoal
    pending_card_id = $cardID
    pending_card_kind = $cardKind
    pending_card_source = $cardSource
    timeline = @($nlTimeline.ToArray())
}
Write-Ok ("NL round settled: reason=" + $settleReason + " turns=" + $turnsSent + " nudges=" + $nudgesSent + " card=" + $(if ($cardID) { $cardKind } else { "none" }))

# Events snapshot before hops (station map evidence, first read).
function Get-EventCensus {
    param([string]$OutFile, [string]$Label)
    $events = $null
    try {
        $events = Invoke-Json -Method GET -Uri ($EventsUri + "?conversation_id=" + [uri]::EscapeDataString($conversationID) + "&since=0&limit=" + [string]$EventsLimit) -TimeoutSec 120
    }
    catch { $events = $null }
    if ($null -ne $events) {
        $events | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath $OutFile -Encoding UTF8
    }
    $types = New-Object System.Collections.Generic.List[string]
    foreach ($eventRow in @(Get-OptionalProperty -Object $events -Name "events")) {
        if ($null -eq $eventRow -or -not ($eventRow -is [System.Management.Automation.PSCustomObject])) { continue }
        $eventType = [string](Get-OptionalProperty -Object $eventRow -Name "type")
        if (-not [string]::IsNullOrWhiteSpace($eventType)) { $types.Add($eventType) }
    }
    $typeArray = @($types.ToArray())
    $eventsText = ""
    if ($null -ne $events) { $eventsText = ($events | ConvertTo-Json -Depth 14 -Compress) }
    return @{
        label = $Label
        file = $OutFile
        total = $typeArray.Count
        type_counts = ($typeArray | Group-Object | Sort-Object Name | ForEach-Object { @{ type = $_.Name; count = $_.Count } })
        trajectory_round_started = @($typeArray | Where-Object { $_ -eq "trajectory.round.started" }).Count
        trajectory_observation_recorded = @($typeArray | Where-Object { $_ -eq "trajectory.observation.recorded" }).Count
        trajectory_hypothesis_proposed = @($typeArray | Where-Object { $_ -eq "trajectory.hypothesis.proposed" }).Count
        mix_tick_pending = @($typeArray | Where-Object { $_ -like "*mix_tick*" }).Count
        audition_events = @($typeArray | Where-Object { $_.StartsWith("audition.") }).Count
        applied_like = @($typeArray | Where-Object { $_ -like "*applied*" -or $_ -like "*intervention*" }).Count
        audition_session_id_seen = $eventsText.Contains("audition_session_id")
    }
}
$censusBefore = Get-EventCensus -OutFile (Join-Path $RunRoot "nl_events.json") -Label "before_hops"

# ------------------------------------------------ phase 6: confirmation hops
Write-Step "Phase 6: confirmation hops (approve the mounted card, follow the chain)"
$hops = New-Object System.Collections.Generic.List[object]
$hopIndex = 0
$faceExercised = $false
$sawReobservedCompleted = $false
$hopPlanID = ""
# Plan face fallback when the response carried only a plan_id (G3 phase-C
# driver face): recover the newest plan id from the recorded turns.
if ([string]::IsNullOrWhiteSpace($cardID)) {
    foreach ($resp in @($allResponses.ToArray())) {
        # $PID is a read-only automatic variable in PowerShell - any other
        # name works (run-3 crash: assigning $pid threw before teardown).
        $planFallback = [string](Get-OptionalProperty -Object $resp -Name "plan_id")
        if (-not [string]::IsNullOrWhiteSpace($planFallback)) { $hopPlanID = $planFallback }
    }
}
while ($hopIndex -lt $MaxHops -and ((-not [string]::IsNullOrWhiteSpace($cardID)) -or ((($hopIndex -eq 0)) -and (-not [string]::IsNullOrWhiteSpace($hopPlanID))))) {
    $hopIndex++
    $face = "interaction"
    $currentCard = $cardID
    if ([string]::IsNullOrWhiteSpace($currentCard) -and -not [string]::IsNullOrWhiteSpace($hopPlanID)) {
        $face = "plan"
        $currentCard = $hopPlanID
    }
    $hopResponse = $null
    $hopError = ""
    try {
        if ($face -eq "plan") {
            $hopResponse = Invoke-Json -Method POST -Uri $ConfirmUri -Body @{
                plan_id = $currentCard
                decision = "approve"
            } -TimeoutSec $TurnSeconds
        }
        else {
            $hopResponse = Invoke-Json -Method POST -Uri $RespondUri -Body @{
                interaction_id = $currentCard
                action_id = "approve"
                decision = "approve"
                payload = @{}
            } -TimeoutSec $TurnSeconds
        }
    }
    catch {
        $hopResponse = $null
        if ($null -ne $_.ErrorDetails -and -not [string]::IsNullOrWhiteSpace($_.ErrorDetails.Message)) { $hopError = $_.ErrorDetails.Message }
        else { $hopError = $_.Exception.Message }
    }
    $hopStop = ""
    $hopGoal = ""
    $hopNeedsConfirm = $false
    $hopExpiredFallback = $false
    $nextPlanID = ""
    $nextInteraction = $null
    if ($null -ne $hopResponse) {
        $faceExercised = $true
        $allResponses.Add($hopResponse)
        $hopStop = [string](Get-OptionalProperty -Object $hopResponse -Name "stop_reason")
        $hopGoal = [string](Get-OptionalProperty -Object $hopResponse -Name "goal_status")
        $hopNeedsConfirm = [bool](Get-OptionalProperty -Object $hopResponse -Name "needs_confirmation")
        if ([string]::IsNullOrWhiteSpace($hopStop) -and $hopGoal -eq "completed") { $hopExpiredFallback = $true }
        if ($hopStop -eq "mix_tick_applied_reobserved" -and $hopGoal -eq "completed") { $sawReobservedCompleted = $true }
        if (-not [string]::IsNullOrWhiteSpace($hopStop)) { $nlStopReasons.Add($hopStop) }
        $hopResponse | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath (Join-Path $RunRoot ("hop_" + [string]$hopIndex + "_response.json")) -Encoding UTF8
        if ($face -eq "plan") {
            $nextPlanID = [string](Get-FirstPropertyValue -Object $hopResponse -Names @("next_plan_id", "plan_id"))
        }
        foreach ($cardRow in @(Get-OptionalProperty -Object $hopResponse -Name "interaction_requests")) {
            if ($null -eq $cardRow -or -not ($cardRow -is [System.Management.Automation.PSCustomObject])) { continue }
            $rowKind = [string](Get-OptionalProperty -Object $cardRow -Name "kind")
            if ([string]::IsNullOrWhiteSpace($rowKind)) { $rowKind = [string](Get-OptionalProperty -Object $cardRow -Name "type") }
            if ($rowKind.ToLower().Contains("confirmation")) {
                $nextInteraction = [string](Get-OptionalProperty -Object $cardRow -Name "id")
                if ([string]::IsNullOrWhiteSpace($nextInteraction)) { $nextInteraction = [string](Get-OptionalProperty -Object $cardRow -Name "interaction_id") }
            }
        }
    }
    $hopAnswered = ($null -ne $hopResponse) -and (-not [string]::IsNullOrWhiteSpace($hopStop)) -and (-not $hopExpiredFallback)
    $hops.Add([pscustomobject]@{
        index = $hopIndex
        face = $face
        card_id = $currentCard
        answered = $hopAnswered
        stop_reason = $hopStop
        goal_status = $hopGoal
        expired_fallback = $hopExpiredFallback
        error = $hopError
    })
    $nlTimeline.Add(("hop_" + [string]$hopIndex + " face=" + $face + " answered=" + [string]$hopAnswered + " stop=" + $hopStop + " goal=" + $hopGoal))
    if ($hopGoal -in @("completed", "failed", "stopped", "cancelled")) { break }
    $cardID = ""
    $hopPlanID = ""
    if ($hopNeedsConfirm) {
        if (-not [string]::IsNullOrWhiteSpace($nextInteraction)) { $cardID = $nextInteraction }
        elseif (-not [string]::IsNullOrWhiteSpace($nextPlanID)) { $hopPlanID = $nextPlanID }
        else { break }
    }
    else {
        break
    }
}

# Also scan the settle-loop turn responses for the reobserved+completed face
# (a full-access auto-authorized apply can complete on a chat turn).
foreach ($resp in @($allResponses.ToArray())) {
    $rStop = [string](Get-OptionalProperty -Object $resp -Name "stop_reason")
    $rGoal = [string](Get-OptionalProperty -Object $resp -Name "goal_status")
    if ($rStop -eq "mix_tick_applied_reobserved" -and $rGoal -eq "completed") { $sawReobservedCompleted = $true }
}

Start-Sleep -Seconds 5
$censusAfter = Get-EventCensus -OutFile (Join-Path $RunRoot "nl_events_after_hop.json") -Label "after_hops"

# Post-confirm station classification (G3-ATTRIB-1 vocabulary).
$postConfirmStation = "not_driven_no_card"
if ($hops.Count -gt 0) {
    $lastHop = $hops[$hops.Count - 1]
    if (-not $lastHop.answered) { $postConfirmStation = "card_mounted_hop_unanswered" }
    elseif ($sawReobservedCompleted) { $postConfirmStation = "judgment_terminal" }
    elseif ([string]$lastHop.stop_reason -eq "observation_budget_exhausted") { $postConfirmStation = "budget_exhausted" }
    elseif ([string]$lastHop.goal_status -eq "failed") { $postConfirmStation = "judgment_failed" }
    elseif ([string]$lastHop.goal_status -eq "waiting_confirmation") { $postConfirmStation = "needs_user_confirmation_again" }
    elseif ([string]$lastHop.goal_status -eq "waiting_clarification") { $postConfirmStation = "needs_user_clarification" }
    elseif (@("llm_error", "model_protocol_failure") -contains [string]$lastHop.stop_reason) { $postConfirmStation = "infra_error_llm" }
    else { $postConfirmStation = "settled_other" }
}
$report.confirm_roundtrip = @{
    phase = "confirm_roundtrip"
    mode = "pull"
    conversation_id = $conversationID
    hops = @($hops.ToArray())
    hops_used = $hops.Count
    confirm_face_exercised = $faceExercised
    post_confirm_station = $postConfirmStation
    judgment_station_events = @{
        total = $censusAfter.total
        audition = $censusAfter.audition_events
        mix_tick = $censusAfter.mix_tick_pending
        applied_like = $censusAfter.applied_like
        audition_session_id_seen = $censusAfter.audition_session_id_seen
    }
    reobserved_completed_seen = $sawReobservedCompleted
}
$report.confirm_roundtrip | ConvertTo-Json -Depth 14 | Set-Content -LiteralPath (Join-Path $RunRoot "confirm_roundtrip.json") -Encoding UTF8
Write-Ok ("confirm round trip: exercised=" + [string]$faceExercised + " hops=" + [string]$hops.Count + " station=" + $postConfirmStation + " reobserved_completed=" + [string]$sawReobservedCompleted)

# ----------------------------------------------- phase 7: telemetry + audit
Write-Step "Phase 7: telemetry census (assertion 1 evidence)"
$telemetryCensus = @{
    file = $TelemetryFile
    exists = (Test-Path -LiteralPath $TelemetryFile)
    records_total = 0
    source_counts = @()
    pullharness_records = 0
    pullharness_with_fingerprint = 0
    distinct_fingerprints = 0
}
if ($telemetryCensus.exists) {
    $records = @()
    foreach ($line in @(Get-Content -LiteralPath $TelemetryFile)) {
        if (-not [string]::IsNullOrWhiteSpace($line)) {
            try { $records += ($line | ConvertFrom-Json) } catch { }
        }
    }
    $telemetryCensus.records_total = $records.Count
    $telemetryCensus.source_counts = @($records | Group-Object { [string]$_.source } | ForEach-Object { @{ source = $_.Name; count = $_.Count } })
    $pullRecords = @($records | Where-Object { [string]$_.source -eq "pullharness" })
    $telemetryCensus.pullharness_records = $pullRecords.Count
    $telemetryCensus.pullharness_with_fingerprint = @($pullRecords | Where-Object { -not [string]::IsNullOrWhiteSpace([string]$_.prompt_fingerprint) }).Count
    $fingerprints = @($pullRecords | ForEach-Object { [string]$_.prompt_fingerprint } | Where-Object { $_ -ne "" })
    $telemetryCensus.distinct_fingerprints = @($fingerprints | Sort-Object -Unique).Count
}
$report.telemetry_census = $telemetryCensus

Write-Step "Phase 8: fixed-orchestration zero-hit audit (assertion 4 evidence)"
# Marker families: the governed command names + workflow ids of the C1/B4/C2
# fixed orchestrations. Surfaces = what the runtime actually did (chat/hop
# response JSONs, both event snapshots, agent log); capability advertisement
# (tool catalogs, prompt assembly) and fastpath preflight are not in scope.
$fixedMarkers = @(
    "semantic_eq_batch",
    "eq_plugin_load_batch",
    "dynamic_plugin_load_batch",
    "dynamic_parameter_batch",
    "c2_dynamic_plugin_selection"
)
$auditSurfaces = New-Object System.Collections.Generic.List[object]
foreach ($evidenceFile in (@($nlResponseFiles.ToArray()) + @(
        (Join-Path $RunRoot "nl_events.json"),
        (Join-Path $RunRoot "nl_events_after_hop.json")))) {
    if ([string]::IsNullOrWhiteSpace($evidenceFile) -or -not (Test-Path -LiteralPath $evidenceFile)) { continue }
    $auditSurfaces.Add([pscustomobject]@{ surface = $evidenceFile; kind = "response_or_events" })
}
foreach ($hopIndex2 in 1..$MaxHops) {
    $hopFile = Join-Path $RunRoot ("hop_" + [string]$hopIndex2 + "_response.json")
    if (Test-Path -LiteralPath $hopFile) { $auditSurfaces.Add([pscustomobject]@{ surface = $hopFile; kind = "hop_response" }) }
}
$agentLogSource = Join-Path $RepoRoot "VitApp\Workspace\Logs\agent_last.log"
$agentLogCopy = Join-Path $RunRoot "agent_last.log"
if (Test-Path -LiteralPath $agentLogSource) {
    Copy-Item -LiteralPath $agentLogSource -Destination $agentLogCopy -Force
    $auditSurfaces.Add([pscustomobject]@{ surface = $agentLogCopy; kind = "agent_log" })
}
$markerHits = New-Object System.Collections.Generic.List[object]
foreach ($surface in @($auditSurfaces.ToArray())) {
    $text = ""
    try { $text = [string](Get-Content -LiteralPath $surface.surface -Raw -Encoding UTF8) } catch { $text = "" }
    if ([string]::IsNullOrWhiteSpace($text)) { continue }
    foreach ($marker in $fixedMarkers) {
        if ($text.Contains($marker)) {
            $markerHits.Add([pscustomobject]@{ marker = $marker; surface = $surface.surface; kind = $surface.kind })
        }
    }
}
$fixedOrchestrationAudit = @{
    markers = $fixedMarkers
    surfaces_scanned = @($auditSurfaces.ToArray() | ForEach-Object { $_.surface })
    hits = @($markerHits.ToArray())
    zero_hits = ($markerHits.Count -eq 0)
    scope_note = "fastpath preflight (FastPathRouter) is harness plumbing and not in the marker set; tool catalogs / prompt advertisement not scanned (capability ads are not runtime activation)"
}
$fixedOrchestrationAudit | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath (Join-Path $RunRoot "fixed_orchestration_audit.json") -Encoding UTF8

# --------------------------------------------- phase 9: assertion review
# Review must never throw past this point (L2-2 lesson: run 20260926_224515
# died inside ConvertFrom-Json and skipped both the FAIL verdict and the
# stack cleanup). Every step below is non-throwing.
Write-Step "Phase 9: assertion groups A1-A4"
$a1 = ($telemetryCensus.pullharness_with_fingerprint -ge 1)
$a2 = ($censusAfter.trajectory_round_started -ge 1 -and $censusAfter.trajectory_observation_recorded -ge 1 -and (($censusAfter.trajectory_hypothesis_proposed + $censusAfter.mix_tick_pending) -ge 1))
$cardMounted = ((-not [string]::IsNullOrWhiteSpace($cardID)) -or ($hops.Count -ge 1) -or $lastNeedsConfirmation -or ($lastGoal -eq "waiting_confirmation"))
$approveAnswered = $false
foreach ($hopRow in @($hops.ToArray())) { if ($hopRow.answered) { $approveAnswered = $true } }
$a3 = ($cardMounted -and $approveAnswered -and ($censusAfter.audition_events -ge 1) -and $sawReobservedCompleted)
$a4 = $fixedOrchestrationAudit.zero_hits

if ($a1) { Write-Ok ("A1 mode/telemetry: pullharness records with fingerprint=" + [string]$telemetryCensus.pullharness_with_fingerprint + " distinct=" + [string]$telemetryCensus.distinct_fingerprints) }
else { Add-Failure ("A1 FAIL: no pullharness telemetry record with prompt_fingerprint (pull mode wiring miss; census=" + ($telemetryCensus | ConvertTo-Json -Compress) + ")") }
if ($a2) { Write-Ok ("A2 free-state station map: rounds=" + [string]$censusAfter.trajectory_round_started + " observations=" + [string]$censusAfter.trajectory_observation_recorded + " proposals/mix_ticks=" + [string]($censusAfter.trajectory_hypothesis_proposed + $censusAfter.mix_tick_pending)) }
else { Add-Failure ("A2 FAIL: station map not recognizable in event stream (rounds=" + [string]$censusAfter.trajectory_round_started + " observations=" + [string]$censusAfter.trajectory_observation_recorded + " proposals=" + [string]$censusAfter.trajectory_hypothesis_proposed + " mix_tick=" + [string]$censusAfter.mix_tick_pending + ")") }
if ($a3) { Write-Ok ("A3 AB audition round trip: card mounted, approve answered, audition events=" + [string]$censusAfter.audition_events + ", mix_tick_applied_reobserved+completed observed") }
else {
    Add-Failure ("A3 FAIL: card_mounted=" + [string]$cardMounted + " approve_answered=" + [string]$approveAnswered + " audition_events=" + [string]$censusAfter.audition_events + " reobserved_completed=" + [string]$sawReobservedCompleted + " (post_confirm_station=" + $postConfirmStation + ")")
}
if ($a4) { Write-Ok "A4 zero fixed orchestration: no C1/C2/B4 marker hits on any scanned surface" }
else {
    foreach ($hit in @($markerHits.ToArray())) { Add-Failure ("A4 FAIL: marker '" + $hit.marker + "' hit on " + $hit.surface) }
}

# Failure classification (AGENTS section 8; card taxonomy).
$classification = "other"
# Failure-class evidence from the free-state reasoning loop itself (run-3
# lesson: the chat response surfaces stop_reason=failed while the real class
# - e.g. model_protocol_failure - rides workflow_data.free_state_reasoning_
# loop.last_error).
$fsLoopLastError = ""
$fsLoopAdmissionStatus = ""
foreach ($respForClass in @($allResponses.ToArray())) {
    $wfd = Get-OptionalProperty -Object $respForClass -Name "workflow_data"
    if ($null -eq $wfd) { continue }
    $fsLoop = Get-OptionalProperty -Object $wfd -Name "free_state_reasoning_loop"
    if ($null -eq $fsLoop) { continue }
    $errText = [string](Get-OptionalProperty -Object $fsLoop -Name "last_error")
    if (-not [string]::IsNullOrWhiteSpace($errText)) { $fsLoopLastError = $errText }
    $receipt = Get-OptionalProperty -Object $fsLoop -Name "admission_receipt"
    if ($null -ne $receipt) {
        $receiptStatus = [string](Get-OptionalProperty -Object $receipt -Name "status")
        if (-not [string]::IsNullOrWhiteSpace($receiptStatus)) { $fsLoopAdmissionStatus = $receiptStatus }
    }
}
$report.fs_loop_last_error = $fsLoopLastError
$report.fs_loop_admission_status = $fsLoopAdmissionStatus
if ($transportErrors -ge 2 -or $settleReason -eq "initial_transport_error") { $classification = "infra_error_transport" }
elseif (@($nlStopReasons.ToArray()) -contains "llm_error" -or @($nlStopReasons.ToArray()) -contains "model_protocol_failure" -or $fsLoopLastError -eq "model_protocol_failure") { $classification = "llm_error" }
elseif ($settleReason -eq "budget_exhausted") {
    $classification = "budget_exhausted"
    if (-not ($a2 -and $a3)) {
        # Honest split per card: partial evidence only counts what held.
        Add-Failure ("budget_exhausted stop with station-map/A3 not yet holding (partial evidence: A1=" + [string]$a1 + " A2=" + [string]$a2 + " A3=" + [string]$a3 + " A4=" + [string]$a4 + ")")
    }
}
elseif (@($nlStopReasons.ToArray()) -contains "llm_error" -or @($nlStopReasons.ToArray()) -contains "model_protocol_failure") { $classification = "llm_error" }
elseif (@($nlStopReasons.ToArray()) -contains "capability_blocked") { $classification = "capability_blocked" }
elseif (@($nlStopReasons.ToArray()) -contains "no_candidate_found" -or @($nlStopReasons.ToArray()) -contains "no_pending_mix_tick_candidate") { $classification = "no_candidate_found" }
elseif ($a1 -and $a2 -and $a3 -and $a4) { $classification = "judgment_terminal" }
$report.classification = $classification
$report.assertions_block = @{
    A1_mode_telemetry = $a1
    A2_free_state_station_map = $a2
    A3_ab_audition_roundtrip = $a3
    A4_zero_fixed_orchestration = $a4
    station_map_before_hops = $censusBefore
    station_map_after_hops = $censusAfter
}
$report.nl_round.timeline = @($nlTimeline.ToArray())
$report.nl_round.stop_reasons = @($nlStopReasons.ToArray())

# Final E:\ integrity check.
$sourceManifestAfter = @(Get-SourceManifest -Folder $TrainingFolder)
@($sourceManifestAfter) | ConvertTo-Json -Depth 4 | Out-File -FilePath (Join-Path $RunRoot "source_manifest_after.json") -Encoding utf8
$finalDiff = Compare-SourceManifest -Before $sourceManifestBefore -After $sourceManifestAfter
if ($finalDiff -ne "") {
    $report.source_integrity = "final diff: " + $finalDiff
    Add-Failure ("E:\ source integrity violation at run end: " + $finalDiff)
}

# ------------------------------------------------------------- teardown
if (-not $KeepStack) {
    Write-Step "Teardown: stop owned stack + verify release"
    Clear-OwnedStack
    $teardownDeadline = (Get-Date).AddSeconds(30)
    do { Start-Sleep -Seconds 3 } while (((@(Test-StackGone)).Count -gt 0) -and ((Get-Date) -lt $teardownDeadline))
    $remaining = @(Test-StackGone)
    $report.teardown = @{
        torn_down = ($remaining.Count -eq 0)
        remaining = @($remaining)
    }
    if ($remaining.Count -gt 0) {
        Write-WarnLine ("stack NOT fully released (kept owner flag): " + (@($remaining) -join "; "))
        $report.environment_interrupt = "teardown incomplete: " + (@($remaining) -join "; ")
    }
    else { Write-Ok "stack released: ports free, processes gone" }
}

Remove-Item -LiteralPath env:VIT_DAW_HARNESS -ErrorAction SilentlyContinue
Remove-Item -LiteralPath env:VIT_AGENT_LLM_TELEMETRY_PATH -ErrorAction SilentlyContinue

if ($failureReasons.Count -eq 0 -and $a1 -and $a2 -and $a3 -and $a4) {
    $report.verdict = "PASS"
    Write-Report
    Write-Step "PASS"
    Write-Ok ("FS large-project free-state smoke passed (four assertion groups). run_root=" + $RunRoot)
    exit 0
}
$report.verdict = "FAIL"
$report.failures = @($failureReasons.ToArray())
Write-Report
Write-Step "FAIL"
foreach ($reason in $failureReasons) { Write-FailLine $reason }
Write-Host ("run_root=" + $RunRoot)
exit 1
}
catch {
    # Uncaught script exception: record, report, tear down, exit functional
    # failure (classification script_exception - tooling, not an LLM class).
    Write-FailLine ("uncaught script exception: " + $_.Exception.Message)
    try { $_ | Out-String | Set-Content -LiteralPath (Join-Path $RunRoot "script_exception.txt") -Encoding UTF8 } catch { }
    $failureReasons.Add("uncaught script exception: " + $_.Exception.Message)
    $report.classification = "script_exception"
    $report.verdict = "FAIL"
    $report.failures = @($failureReasons.ToArray())
    if (-not $KeepStack) {
        Clear-OwnedStack
        $teardownDeadline2 = (Get-Date).AddSeconds(30)
        do { Start-Sleep -Seconds 3 } while (((@(Test-StackGone)).Count -gt 0) -and ((Get-Date) -lt $teardownDeadline2))
        $remaining2 = @(Test-StackGone)
        $report.teardown = @{
            torn_down = ($remaining2.Count -eq 0)
            remaining = @($remaining2)
        }
    }
    Write-Report
    exit 1
}
