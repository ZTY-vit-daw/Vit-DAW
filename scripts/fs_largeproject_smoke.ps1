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
        batch / dynamic_parameter_batch / c2_dynamic_plugin_selection /
        c2.dynamic_plugin_load.governed single-track)
        have ZERO hits across the chat/hop response JSONs, the event
        streams and the agent log (fastpath preflight is harness plumbing
        and is NOT in the marker set), AND the telemetry LLM source face
        carries ZERO fixed-orchestration planner sources (b4/c1/c2
        project_*_planner tags; ordinary_agent_* / pullharness are the
        model-driven path and expected).

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
    [switch]$KeepStack,
    # MIXLAYER-M1-2 queue roundtrip phase (off = legacy flow unchanged):
    # post-round queue view read + node annotation write-back + re-read, with
    # the same fixed-orchestration marker discipline applied to the queue
    # response surfaces.
    [switch]$M1QueueRoundtrip,
    # MIXLAYER-M1-3 C3 read-only observation phase (off = legacy flow
    # unchanged): a free-state chat message with an explicit C3 marker must
    # reach the space/depth observation Session and return the read-only
    # report (schema line + no-write line), with the fixed-orchestration
    # marker discipline applied to the response surface.
    [switch]$C3Observation,
    # MIXLAYER-M1-4 D1-D3 pre-master read-only observation phase (off =
    # legacy flow unchanged): a free-state chat message with an explicit D
    # marker must reach the pre_master.observation.v0 read-only pack and
    # return the observation report (schema line + three face lines + D3
    # reference-track gap registration + no-write line), with the fixed-
    # orchestration marker discipline applied to the response surface.
    [switch]$D1D3Observation,
    # MIXLAYER-M1-5 F1 three-view phase (off = legacy flow unchanged): a
    # deterministic /agent/invoke mix_report call whose response must carry
    # the latest settle conclusion line and the queue node-state line. The
    # E1-deferred needle depends on the M1Q q2 annotation, so this switch
    # requires -M1QueueRoundtrip (misuse fails the run up front).
    [switch]$M1FiveThreeView
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

# MIXLAYER-M1-5 misuse guard: the three-view leg's E1-deferred needle rides
# the M1Q q2 annotation write-back, so the switch is only meaningful together
# with -M1QueueRoundtrip. Fail fast, before the stack starts.
if ($M1FiveThreeView -and (-not $M1QueueRoundtrip)) {
    Write-FailLine "-M1FiveThreeView requires -M1QueueRoundtrip (the E1-deferred needle rides the M1Q q2 annotation)"
    exit 1
}

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

# Numeric exit code recorded into run_report (review D gap 2: only the
# verdict was recorded, the actual process exit code was not). Stamped into
# the report by Write-Report; every exit site sets it before reporting.
$script:PlannedExitCode = 1
# Stack-ownership guard for the pre-net catch below: a throw before the
# clear-field check (kernel/source path resolution) must NOT tear a stack
# down - the field has not been verified empty yet, so a live stack at that
# point belongs to someone else. Set only after clear-field passed and the
# bring-up of OUR stack is starting.
$script:StackOwnershipClaimed = $false

# Review D gap 1: $MyInvocation.Line is empty under powershell -File, which
# left command_line="" and the full invocation unrestorable from artifacts.
# Fallback: rebuild the effective command line from the script path plus the
# bound parameters (only explicitly passed ones are recorded).
$invocationLine = ""
try {
    if (-not [string]::IsNullOrWhiteSpace([string]$MyInvocation.Line)) {
        $invocationLine = [string]$MyInvocation.Line
    }
    else {
        $invocationLine = $PSCommandPath
        foreach ($boundParam in $PSBoundParameters.GetEnumerator()) {
            if ($boundParam.Value -is [switch]) { $invocationLine += (" -" + $boundParam.Key) }
            elseif ((($boundParam.Value -as [string]) -ne $null) -and ([string]$boundParam.Value).Contains(" ")) { $invocationLine += (" -" + $boundParam.Key + ' "' + [string]$boundParam.Value + '"') }
            else { $invocationLine += (" -" + $boundParam.Key + " " + [string]$boundParam.Value) }
        }
    }
}
catch { $invocationLine = "unavailable: " + $_.Exception.Message }

$report = [ordered]@{
    run_id = "fs_largeproject_smoke_" + $RunStamp
    card = "FS-LARGEPROJECT-SMOKE-1"
    started_at = (Get-Date).ToUniversalTime().ToString("o")
    command_line = $invocationLine
    repo_head = ""
    worktree = $RepoRoot
    branch = ""
    kernel_exe = ""
    kernel_sha256 = ""
    # Review D gap 3: this used to be a permanently-empty dead string field.
    # Populated after stack bring-up with the binary the running agent really
    # executes (path + sha256 + last_write_utc; AGENTS section 9 SkipBuild rule).
    agent_binary = $null
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
    exit_code = $null
    teardown = $null
}
try { $report.repo_head = (git -C $RepoRoot rev-parse HEAD 2>$null | Out-String).Trim() } catch { $report.repo_head = "unavailable" }
try { $report.branch = (git -C $RepoRoot rev-parse --abbrev-ref HEAD 2>$null | Out-String).Trim() } catch { $report.branch = "unavailable" }
function Write-Report {
    $report.finished_at = (Get-Date).ToUniversalTime().ToString("o")
    $report.exit_code = $script:PlannedExitCode
    try {
        $report | ConvertTo-Json -Depth 14 | Out-File -FilePath (Join-Path $RunRoot "run_report.json") -Encoding utf8
    }
    catch {
        # 184500 lesson, hardened: a crash (or an unserializable $report
        # field) must never leave the run without any report artifact. Land a
        # minimal fallback next to the full report and re-raise so the
        # caller's error path keeps its evidence.
        try {
            @{
                run_id = [string]$report.run_id
                card = [string]$report.card
                verdict = "FAIL"
                classification = "script_exception"
                exit_code = $script:PlannedExitCode
                report_serialization_error = $_.Exception.Message
                note = "full run_report.json serialization failed; minimal fallback written by Write-Report"
            } | ConvertTo-Json -Depth 4 | Out-File -FilePath (Join-Path $RunRoot "run_report.min.json") -Encoding utf8
        }
        catch { }
        throw
    }
}
(git -C $RepoRoot rev-parse HEAD 2>$null | Out-String).Trim() | Set-Content -LiteralPath (Join-Path $RunRoot "head.txt") -Encoding UTF8
(git -C $RepoRoot status --short 2>$null | Out-String) | Set-Content -LiteralPath (Join-Path $RunRoot "git_status.txt") -Encoding UTF8

# Pre-net safety wrap (184500 lesson generalized: a tooling crash must never
# leave the run without run_report.json). Phases 0-1 run before the big
# post-health safety net below; an uncaught throw here used to kill the
# script with no report and no teardown record.
try {

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
    $script:PlannedExitCode = 2
    Write-Report
    exit 2
}
Write-Ok "clear field: ports 7878/5555/5556 free, no VitAgent/VitApp/Godot-frontend processes"
$script:StackOwnershipClaimed = $true

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
    $script:PlannedExitCode = 2
    Write-Report
    if (-not $KeepStack) { Clear-OwnedStack }
    exit 2
}
Write-Ok ("stack up (bringup " + [string]$report.bringup_seconds + "s, agent inherits VIT_DAW_HARNESS=pull)")

# agent_binary face (review D gap 3: dead field): record the binary the
# running agent actually executes, with hash + build time (AGENTS section 9
# records the tested binary for -SkipBuild runs). Primary source = the
# running VitAgent process path; fallback = dev_agent_smoke's agent\bin copy.
$agentBinaryFace = @{ path = ""; sha256 = ""; last_write_utc = "" }
try {
    $agentProc = @(Get-CimInstance Win32_Process -Filter "Name LIKE 'VitAgent%'" -ErrorAction SilentlyContinue | Select-Object -First 1)
    $agentBinPath = ""
    if (@($agentProc).Count -ge 1 -and -not [string]::IsNullOrWhiteSpace([string]$agentProc[0].ExecutablePath)) {
        $agentBinPath = [string]$agentProc[0].ExecutablePath
    }
    if ([string]::IsNullOrWhiteSpace($agentBinPath)) { $agentBinPath = Join-Path $RepoRoot "agent\bin\VitAgent.exe" }
    if (Test-Path -LiteralPath $agentBinPath) {
        $agentBinaryFace.path = $agentBinPath
        $agentBinaryFace.sha256 = (Get-FileHash -LiteralPath $agentBinPath -Algorithm SHA256).Hash
        $agentBinaryFace.last_write_utc = (Get-Item -LiteralPath $agentBinPath).LastWriteTimeUtc.ToString("o")
        Write-Ok ("agent binary: " + $agentBinPath)
    }
    else { Write-WarnLine "agent binary not resolvable (report field left empty)" }
}
catch { Write-WarnLine ("agent binary face unresolved: " + $_.Exception.Message) }
$report.agent_binary = $agentBinaryFace

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
    $script:PlannedExitCode = 2
    Write-Report
    if (-not $KeepStack) { Clear-OwnedStack }
    exit 2
}
Write-Ok ("agent ok tool_count=" + [string](Get-OptionalProperty -Object $state -Name "tool_count"))

}
catch {
    # Pre-net crash mirror: record, report, tear down, exit functional
    # failure (classification script_exception - tooling, not an LLM class).
    Write-FailLine ("pre-safety-net script exception: " + $_.Exception.Message)
    try { $_ | Out-String | Set-Content -LiteralPath (Join-Path $RunRoot "script_exception_prenet.txt") -Encoding UTF8 } catch { }
    $failureReasons.Add("pre-safety-net script exception: " + $_.Exception.Message)
    $report.classification = "script_exception"
    $report.verdict = "FAIL"
    $report.failures = @($failureReasons.ToArray())
    # Tear down only what this run owns: before clear-field passed, a live
    # stack belongs to someone else (pre-net throws can precede Phase 0).
    if (-not $KeepStack -and $script:StackOwnershipClaimed) {
        Clear-OwnedStack
        $teardownDeadlinePrenet = (Get-Date).AddSeconds(30)
        do { Start-Sleep -Seconds 3 } while (((@(Test-StackGone)).Count -gt 0) -and ((Get-Date) -lt $teardownDeadlinePrenet))
        $remainingPrenet = @(Test-StackGone)
        $report.teardown = @{
            torn_down = ($remainingPrenet.Count -eq 0)
            remaining = @($remainingPrenet)
        }
    }
    $script:PlannedExitCode = 1
    Write-Report
    exit 1
}

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
    $script:PlannedExitCode = 1
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
    $script:PlannedExitCode = 2
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
    $script:PlannedExitCode = 2
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
    $script:PlannedExitCode = 1
    Write-Report
    if (-not $KeepStack) { Clear-OwnedStack }
    exit 1
}
if (-not (Test-Path -LiteralPath $IsolatedProjectPath)) {
    Add-Failure ("isolated project file missing after successful save_as: " + $IsolatedProjectPath)
    $report.verdict = "save_as_failed"
    $script:PlannedExitCode = 1
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
    $script:PlannedExitCode = 1
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
# Fixed-orchestration planner LLM source tags (review A note 2): the fixed
# C1/C2/B4 orchestrators drive their own LLM calls under dedicated source
# tags, so a telemetry record carrying one of these proves a fixed
# orchestration planner ran - independent of the text-marker scan faces.
# Anchors: b4_eq_planner.go:43/124/180, c1_frequency_cleanup_planner.go:41/108,
# c2_dynamic_control_runtime.go:214/308, c2_project_treatment.go:114.
# ordinary_agent_* / pullharness are the model-driven path and expected.
$fixedOrchestrationLlmSources = @(
    "b4_project_eq_planner",
    "b4_project_eq_instance_selection",
    "b4_project_treatment_planner",
    "c1_project_eq_planner",
    "c1_project_treatment_planner",
    "c2_project_candidate_planner",
    "c2_project_target_planner",
    "c2_project_treatment_planner"
)
$telemetrySourceHits = @()
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
    $telemetrySourceHits = @($records | Where-Object { $fixedOrchestrationLlmSources -contains [string]$_.source } | ForEach-Object { @{ source = [string]$_.source; created_at = [string]$_.created_at } })
}
$report.telemetry_census = $telemetryCensus

Write-Step "Phase 8: fixed-orchestration zero-hit audit (assertion 4 evidence)"
# Marker families: the governed command names + workflow ids of the C1/B4/C2
# fixed orchestrations, including the C2 single-track governed load command
# (review A note 1: c2.dynamic_plugin_load.governed is not covered by the
# _batch substring form; anchor c2_dynamic_plugin_load.go:20, dispatched at
# c2_dynamic_control_runtime.go:520). Surfaces = what the runtime actually
# did (chat/hop response JSONs, both event snapshots, agent log) PLUS the
# telemetry LLM source face (planner tags above); capability advertisement
# (tool catalogs, prompt assembly) and fastpath preflight are not in scope.
$fixedMarkers = @(
    "semantic_eq_batch",
    "eq_plugin_load_batch",
    "dynamic_plugin_load_batch",
    "dynamic_parameter_batch",
    "c2_dynamic_plugin_selection",
    "c2.dynamic_plugin_load.governed"
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
# Telemetry LLM source face joins the same zero-hit assertion: a planner
# source tag on any telemetry record is fixed-orchestration evidence.
foreach ($srcHit in @($telemetrySourceHits)) {
    $markerHits.Add([pscustomobject]@{ marker = [string]$srcHit.source; surface = ("telemetry:" + $TelemetryFile); kind = "telemetry_llm_source" })
}
$fixedOrchestrationAudit = @{
    markers = $fixedMarkers
    telemetry_llm_source_markers = $fixedOrchestrationLlmSources
    telemetry_llm_source_hits = @($telemetrySourceHits)
    surfaces_scanned = @($auditSurfaces.ToArray() | ForEach-Object { $_.surface })
    hits = @($markerHits.ToArray())
    zero_hits = ($markerHits.Count -eq 0)
    scope_note = "fastpath preflight (FastPathRouter) is harness plumbing and not in the marker set; tool catalogs / prompt advertisement not scanned (capability ads are not runtime activation); telemetry LLM source face scanned against the fixed-orchestration planner tags"
}
$fixedOrchestrationAudit | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath (Join-Path $RunRoot "fixed_orchestration_audit.json") -Encoding UTF8

# ------------------- phase 8.5: M1-2 mix workflow queue roundtrip (opt-in)
# MIXLAYER-M1-2: in-conversation queue read -> annotation write-back ->
# re-read, on the same conversation the free-state round ran in. All three
# messages ride the deterministic queue preflights (no LLM dependence), so
# the gates below are deterministic. Chinese text uses the script-wide B64
# discipline (encoding verified before commit).
$m1q = @{
    enabled = [bool]$M1QueueRoundtrip
    q1_read_schema = $false
    q1_read_datasource = $false
    q2_write_ack = $false
    q3_reread_deferred = $false
    zero_fixed_orchestration_markers = $false
    responses = @()
    marker_hits = @()
}
if ($M1QueueRoundtrip) {
    Write-Step "Phase 8.5: M1-2 queue roundtrip (read -> annotate -> re-read)"
    $m1qReadText = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String("5p+l55yL5re36Z+z5bel5L2c6Zif5YiX54q25oCB"))
    $m1qWriteText = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String("6Zif5YiX5qCH6K6wIEUxIOW7tuacnyDng5/mtYvlhpnlm57pqozor4E="))
    $m1qNeedleSchema = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String("bWl4X3dvcmtmbG93X3F1ZXVlLnYw"))
    $m1qNeedleDataSource = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String("5pWw5o2u5p2l5rqQ"))
    $m1qNeedleE1Deferred = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String("RTEg5q+N5bimIEVRW+W7tuacn10="))
    $m1qNeedleWriteAck = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String("5bey5Zue5YaZ6IqC54K55qCH5rOo77yaRTEg5q+N5bimIEVRW+W7tuacn10="))
    $m1qSequence = @(
        @{ name = "q1_read"; message = $m1qReadText },
        @{ name = "q2_write"; message = $m1qWriteText },
        @{ name = "q3_reread"; message = $m1qReadText }
    )
    $m1qReplies = @{}
    foreach ($step in $m1qSequence) {
        $m1qResponse = $null
        try {
            $m1qResponse = Invoke-Json -Method POST -Uri $ChatUri -Body @{
                # Fresh conversation (M1-1 cross-identity-axis precedent): the
                # queue is project-keyed blackboard state, and a fresh
                # conversation cannot be captured by any pending-confirmation
                # intercept left mounted by the round.
                conversation_id = ($conversationID + "_m1q")
                message = $step.message
                context = @{ agent_mode = "chat" }
            } -TimeoutSec $TurnSeconds
        }
        catch {
            $errText = ""
            if ($null -ne $_.ErrorDetails -and -not [string]::IsNullOrWhiteSpace($_.ErrorDetails.Message)) { $errText = $_.ErrorDetails.Message }
            else { $errText = $_.Exception.Message }
            Add-Failure ("M1Q transport error on " + $step.name + ": " + $errText)
        }
        if ($null -ne $m1qResponse) {
            $m1qFile = Join-Path $RunRoot ("m1q_" + $step.name + ".json")
            $m1qResponse | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath $m1qFile -Encoding UTF8
            $m1q.responses += $m1qFile
            $m1qReplies[$step.name] = [string](Get-OptionalProperty -Object $m1qResponse -Name "reply")
        }
    }
    if ($null -ne $m1qReplies["q1_read"]) {
        $m1q.q1_read_schema = $m1qReplies["q1_read"].Contains($m1qNeedleSchema)
        $m1q.q1_read_datasource = $m1qReplies["q1_read"].Contains($m1qNeedleDataSource)
    }
    if ($null -ne $m1qReplies["q2_write"]) {
        $m1q.q2_write_ack = $m1qReplies["q2_write"].Contains($m1qNeedleWriteAck)
    }
    if ($null -ne $m1qReplies["q3_reread"]) {
        $m1q.q3_reread_deferred = $m1qReplies["q3_reread"].Contains($m1qNeedleE1Deferred)
    }
    # A4 discipline extension: the queue surfaces must stay clean of the same
    # fixed-orchestration marker families.
    $m1qHits = New-Object System.Collections.Generic.List[object]
    foreach ($m1qSurface in @($m1q.responses)) {
        $m1qText = ""
        try { $m1qText = [string](Get-Content -LiteralPath $m1qSurface -Raw -Encoding UTF8) } catch { $m1qText = "" }
        if ([string]::IsNullOrWhiteSpace($m1qText)) { continue }
        foreach ($marker in $fixedMarkers) {
            if ($m1qText.Contains($marker)) { $m1qHits.Add([pscustomobject]@{ marker = $marker; surface = $m1qSurface }) }
        }
    }
    $m1q.marker_hits = @($m1qHits.ToArray())
    $m1q.zero_fixed_orchestration_markers = ($m1qHits.Count -eq 0)
    $m1q | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath (Join-Path $RunRoot "m1_queue_roundtrip.json") -Encoding UTF8
    if ($m1q.q1_read_schema -and $m1q.q1_read_datasource) { Write-Ok "M1Q q1 queue view read renders schema + data-source line" }
    else { Add-Failure "M1Q FAIL: queue view read did not render (schema/data-source needle missing)" }
    if ($m1q.q2_write_ack) { Write-Ok "M1Q q2 node annotation write-back acknowledged (E1 deferred)" }
    else { Add-Failure "M1Q FAIL: node annotation write-back not acknowledged" }
    if ($m1q.q3_reread_deferred) { Write-Ok "M1Q q3 re-read shows E1 deferred (write-back roundtrip)" }
    else { Add-Failure "M1Q FAIL: re-read does not show E1 deferred (roundtrip broken)" }
    if ($m1q.zero_fixed_orchestration_markers) { Write-Ok "M1Q queue surfaces clean of fixed-orchestration markers" }
    else {
        foreach ($hit in @($m1q.marker_hits)) { Add-Failure ("M1Q FAIL: marker '" + $hit.marker + "' hit on " + $hit.surface) }
    }
}
$report.m1_queue_roundtrip = $m1q

# ------------------- phase 8.6: M1-3 C3 space/depth read-only observation (opt-in)
# MIXLAYER-M1-3: one free-state chat message with an explicit C3 marker. The
# canary seam routes it to the fine_mix.space_depth.v0 read-only observation
# Session; the gates assert the observation report (schema line + read-only
# line) came back and the response surface carries no fixed-orchestration
# markers. The report is deterministic (no LLM turn), so the gates are
# deterministic. Chinese text uses the script-wide B64 discipline.
$c3obs = @{
    enabled = [bool]$C3Observation
    observation_reply_schema = $false
    observation_readonly_line = $false
    zero_fixed_orchestration_markers = $false
    response = ""
    marker_hits = @()
}
if ($C3Observation) {
    Write-Step "Phase 8.6: M1-3 C3 space/depth read-only observation"
    $c3Message = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String("YzMg6KeC5a+f56m66Ze05LiO5rex5bqm"))
    $c3NeedleSchema = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String("YzMuc3BhY2VfZGVwdGgub2JzZXJ2YXRpb24udjA="))
    $c3NeedleReadOnly = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String("5pyq5rOo5YaM5Lu75L2V5YaZ5ZG95Luk77yM5pyq5L+u5pS55bel56iL"))
    $c3Response = $null
    try {
        $c3Response = Invoke-Json -Method POST -Uri $ChatUri -Body @{
            # Fresh conversation (M1Q precedent): a read-only observation must
            # not be captured by any pending-confirmation intercept left
            # mounted by the free-state round.
            conversation_id = ($conversationID + "_c3obs")
            message = $c3Message
            context = @{ agent_mode = "chat" }
        } -TimeoutSec $TurnSeconds
    }
    catch {
        $errText = ""
        if ($null -ne $_.ErrorDetails -and -not [string]::IsNullOrWhiteSpace($_.ErrorDetails.Message)) { $errText = $_.ErrorDetails.Message }
        else { $errText = $_.Exception.Message }
        Add-Failure ("C3OBS transport error: " + $errText)
    }
    if ($null -ne $c3Response) {
        $c3File = Join-Path $RunRoot "c3_observation_response.json"
        $c3Response | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath $c3File -Encoding UTF8
        $c3obs.response = $c3File
        $c3Reply = [string](Get-OptionalProperty -Object $c3Response -Name "reply")
        $c3obs.observation_reply_schema = $c3Reply.Contains($c3NeedleSchema)
        $c3obs.observation_readonly_line = $c3Reply.Contains($c3NeedleReadOnly)
        # A4 discipline extension: the C3 response surface must stay clean of
        # the fixed-orchestration marker families (C3 is an observation-only
        # Session; no governed batch command may appear on this surface).
        $c3Hits = New-Object System.Collections.Generic.List[object]
        $c3Text = ""
        try { $c3Text = [string](Get-Content -LiteralPath $c3File -Raw -Encoding UTF8) } catch { $c3Text = "" }
        if (-not [string]::IsNullOrWhiteSpace($c3Text)) {
            foreach ($marker in $fixedMarkers) {
                if ($c3Text.Contains($marker)) { $c3Hits.Add([pscustomobject]@{ marker = $marker; surface = $c3File }) }
            }
        }
        $c3obs.marker_hits = @($c3Hits.ToArray())
        $c3obs.zero_fixed_orchestration_markers = ($c3Hits.Count -eq 0)
    }
    $c3obs | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath (Join-Path $RunRoot "c3_observation.json") -Encoding UTF8
    if ($c3obs.observation_reply_schema) { Write-Ok "C3OBS observation report renders schema line (c3.space_depth.observation.v0)" }
    else { Add-Failure "C3OBS FAIL: C3 observation report schema line missing (seam dispatch or report rendering broken)" }
    if ($c3obs.observation_readonly_line) { Write-Ok "C3OBS observation report carries the no-write declaration" }
    else { Add-Failure "C3OBS FAIL: read-only declaration line missing" }
    if ($c3obs.zero_fixed_orchestration_markers) { Write-Ok "C3OBS response surface clean of fixed-orchestration markers" }
    else {
        foreach ($hit in @($c3obs.marker_hits)) { Add-Failure ("C3OBS FAIL: marker '" + $hit.marker + "' hit on " + $hit.surface) }
    }
}
$report.c3_observation = $c3obs
# ------------------- phase 8.7: M1-4 D1-D3 pre-master read-only observation (opt-in)
# MIXLAYER-M1-4: one free-state chat message with an explicit D marker. The
# canary seam routes it to the pre_master.observation.v0 read-only observation
# pack; the gates assert the observation report (schema line + D1/D2/D3 face
# lines + D3 reference-track gap registration + read-only line) came back and
# the response surface carries no fixed-orchestration markers. The report is
# deterministic (no LLM turn), so the gates are deterministic. Chinese text
# uses the script-wide B64 discipline (encoding verified before commit).
$d1d3obs = @{
    enabled = [bool]$D1D3Observation
    observation_reply_schema = $false
    observation_face_lines = $false
    observation_gap_line = $false
    observation_readonly_line = $false
    zero_fixed_orchestration_markers = $false
    response = ""
    marker_hits = @()
}
if ($D1D3Observation) {
    Write-Step "Phase 8.7: M1-4 D1-D3 pre-master read-only observation"
    $d1d3Message = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String("ZDEg5q+N5bim5YmN5ZON5bqm5bOw5YC86KeC5a+f"))
    $d1d3NeedleSchema = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String("ZDFkMy5wcmVtYXN0ZXIub2JzZXJ2YXRpb24udjA="))
    $d1d3NeedleReadOnly = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String("5pys5qyh6KeC5a+f5pyq5Yib5bu6IFByb3Bvc2Fs77yM5pyq5rOo5YaM5Lu75L2V5YaZ5ZG95Luk77yM5pyq5L+u5pS55bel56iL"))
    $d1d3NeedleGap = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String("5Y+C6ICD5puy5a+85YWlL+WvueavlOWRveS7pOe8uuWPow=="))
    $d1d3NeedleFace1 = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String("RDEg5ZON5bqm5bOw5YC86Z2i"))
    $d1d3NeedleFace2 = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String("RDIg57+76K+R5qOA5p+l6Z2i"))
    $d1d3NeedleFace3 = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String("RDMg5Y+C6ICD5puy5a+55q+U6Z2i"))
    $d1d3Response = $null
    try {
        $d1d3Response = Invoke-Json -Method POST -Uri $ChatUri -Body @{
            # Fresh conversation (C3OBS/M1Q precedent): a read-only observation
            # must not be captured by any pending-confirmation intercept left
            # mounted by the free-state round.
            conversation_id = ($conversationID + "_d1d3obs")
            message = $d1d3Message
            context = @{ agent_mode = "chat" }
        } -TimeoutSec $TurnSeconds
    }
    catch {
        $errText = ""
        if ($null -ne $_.ErrorDetails -and -not [string]::IsNullOrWhiteSpace($_.ErrorDetails.Message)) { $errText = $_.ErrorDetails.Message }
        else { $errText = $_.Exception.Message }
        Add-Failure ("D1D3OBS transport error: " + $errText)
    }
    if ($null -ne $d1d3Response) {
        $d1d3File = Join-Path $RunRoot "d1d3_observation_response.json"
        $d1d3Response | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath $d1d3File -Encoding UTF8
        $d1d3obs.response = $d1d3File
        $d1d3Reply = [string](Get-OptionalProperty -Object $d1d3Response -Name "reply")
        $d1d3obs.observation_reply_schema = $d1d3Reply.Contains($d1d3NeedleSchema)
        $d1d3obs.observation_face_lines = ($d1d3Reply.Contains($d1d3NeedleFace1) -and $d1d3Reply.Contains($d1d3NeedleFace2) -and $d1d3Reply.Contains($d1d3NeedleFace3))
        $d1d3obs.observation_gap_line = $d1d3Reply.Contains($d1d3NeedleGap)
        $d1d3obs.observation_readonly_line = $d1d3Reply.Contains($d1d3NeedleReadOnly)
        # A4 discipline extension: the D1-D3 response surface must stay clean of
        # the fixed-orchestration marker families (the pack is observation-only;
        # no governed batch command may appear on this surface).
        $d1d3Hits = New-Object System.Collections.Generic.List[object]
        $d1d3Text = ""
        try { $d1d3Text = [string](Get-Content -LiteralPath $d1d3File -Raw -Encoding UTF8) } catch { $d1d3Text = "" }
        if (-not [string]::IsNullOrWhiteSpace($d1d3Text)) {
            foreach ($marker in $fixedMarkers) {
                if ($d1d3Text.Contains($marker)) { $d1d3Hits.Add([pscustomobject]@{ marker = $marker; surface = $d1d3File }) }
            }
        }
        $d1d3obs.marker_hits = @($d1d3Hits.ToArray())
        $d1d3obs.zero_fixed_orchestration_markers = ($d1d3Hits.Count -eq 0)
    }
    $d1d3obs | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath (Join-Path $RunRoot "d1d3_observation.json") -Encoding UTF8
    if ($d1d3obs.observation_reply_schema) { Write-Ok "D1D3OBS observation report renders schema line (d1d3.premaster.observation.v0)" }
    else { Add-Failure "D1D3OBS FAIL: D1-D3 observation report schema line missing (seam dispatch or report rendering broken)" }
    if ($d1d3obs.observation_face_lines) { Write-Ok "D1D3OBS observation report renders D1/D2/D3 face lines" }
    else { Add-Failure "D1D3OBS FAIL: D1/D2/D3 face lines missing (three-face report incomplete)" }
    if ($d1d3obs.observation_gap_line) { Write-Ok "D1D3OBS observation report registers the D3 reference-track command gap" }
    else { Add-Failure "D1D3OBS FAIL: D3 reference-track gap registration line missing" }
    if ($d1d3obs.observation_readonly_line) { Write-Ok "D1D3OBS observation report carries the no-write declaration" }
    else { Add-Failure "D1D3OBS FAIL: read-only declaration line missing" }
    if ($d1d3obs.zero_fixed_orchestration_markers) { Write-Ok "D1D3OBS response surface clean of fixed-orchestration markers" }
    else {
        foreach ($hit in @($d1d3obs.marker_hits)) { Add-Failure ("D1D3OBS FAIL: marker '" + $hit.marker + "' hit on " + $hit.surface) }
    }
}
$report.d1d3_observation = $d1d3obs
# ------------------- phase 8.6: M1-5 mix.report three-view (opt-in)
# MIXLAYER-M1-5: one deterministic /agent/invoke mix_report call after the
# M1Q legs. The response must carry (1) the latest settle conclusion line
# (mixboard fold over the M1-1 persistence layer), (2) the queue node-state
# line (chat-side fold over the M1-2 queue view), and the two folds must
# agree on the same free-state conclusion board. Chinese text uses the
# script-wide B64 discipline (encoding verified before commit).
$m15 = @{
    enabled       = [bool]$M1FiveThreeView
    invoke_ok     = $false
    report_schema = $false
    settle_line   = $false
    queue_line    = $false
    e1_deferred   = $false
    fold_agree    = $false
    settle_truth  = $false
    a4_zero_markers = $false
    timeline_count = -1
    queue_sources_conclusions = -1
    latest_conclusion_id = ""
    response_file = ""
    marker_hits   = @()
}
if ($M1FiveThreeView) {
    Write-Step "Phase 8.6: M1-5 mix.report three-view (settle line + queue node states)"
    $m15NeedleSettleLine = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String("5pyA6L+RIHNldHRsZSDnu5PorrrvvJo="))
    $m15NeedleQueueLine = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String("cXVldWUg6IqC54K554q25oCB77ya"))
    $m15NeedleNodeCount = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String("MjMg6IqC54K5"))
    $m15NeedleE1Deferred = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String("RTEg5q+N5bimIEVRW+W7tuacn10="))
    $m15NeedleNoRecord = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String("5peg6K6w5b2V"))
    $m15Response = $null
    try {
        $m15Response = Invoke-ToolJson -Uri $InvokeUri -Body @{ tool = "mix_report" } -TimeoutSec 120
    }
    catch {
        Add-Failure ("M15 transport error on mix_report invoke: " + $_.Exception.Message)
    }
    if ($null -ne $m15Response) {
        $m15File = Join-Path $RunRoot "m1_5_three_view_report.json"
        $m15Response | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath $m15File -Encoding UTF8
        $m15.response_file = $m15File
        $m15Result = Get-OptionalProperty -Object $m15Response -Name "result"
        $m15Status = [string](Get-OptionalProperty -Object $m15Response -Name "status")
        $m15Message = [string](Get-OptionalProperty -Object $m15Result -Name "message")
        $m15.invoke_ok = ($m15Status -eq "ok" -and $null -ne $m15Result)
        if ($m15.invoke_ok) {
            $m15.report_schema = (([string](Get-OptionalProperty -Object $m15Result -Name "schema_version")) -eq "mix_report.v1")
            $m15.settle_line = $m15Message.Contains($m15NeedleSettleLine)
            $m15.queue_line = ($m15Message.Contains($m15NeedleQueueLine) -and $m15Message.Contains($m15NeedleNodeCount))
            $m15.e1_deferred = $m15Message.Contains($m15NeedleE1Deferred)
            # Fold agreement: the report's conclusion timeline and the queue
            # view's source count read the SAME M1-1 board - disagreement
            # means one fold lied.
            $m15Timeline = Get-OptionalProperty -Object $m15Result -Name "free_state_conclusion_timeline"
            $m15Queue = Get-OptionalProperty -Object $m15Result -Name "mix_workflow_queue"
            $m15QueueSources = Get-OptionalProperty -Object $m15Queue -Name "sources"
            # PS 5.1: an empty JSON array decodes to $null and @($null).Count
            # is 1, so the null case is pinned to 0 explicitly.
            if ($null -eq $m15Timeline) { $m15.timeline_count = 0 }
            else { $m15.timeline_count = @($m15Timeline).Count }
            $m15QSC = Get-OptionalProperty -Object $m15QueueSources -Name "free_state_conclusions"
            if ($null -eq $m15QSC) { $m15.queue_sources_conclusions = -1 }
            else { $m15.queue_sources_conclusions = [int]$m15QSC }
            $m15.fold_agree = ($m15.timeline_count -eq $m15.queue_sources_conclusions)
            $m15Latest = Get-OptionalProperty -Object $m15Result -Name "latest_settle_conclusion"
            $m15.latest_conclusion_id = [string](Get-FirstPropertyValue -Object $m15Latest -Names @("conclusion_id"))
            if ($m15.timeline_count -ge 1) {
                $m15.settle_truth = (($null -ne $m15Latest) -and (-not $m15Message.Contains($m15NeedleNoRecord)))
            }
            else {
                # No conclusions this round: the honest no-record default IS
                # the correct line content; the fold is still proven by the
                # agreement gate above.
                $m15.settle_truth = $m15Message.Contains($m15NeedleNoRecord)
            }
            # A4 discipline: the invoke response surface must stay clean of
            # the fixed-orchestration marker families.
            $m15Hits = New-Object System.Collections.Generic.List[object]
            $m15Text = ""
            try { $m15Text = [string](Get-Content -LiteralPath $m15File -Raw -Encoding UTF8) } catch { $m15Text = "" }
            foreach ($marker in $fixedMarkers) {
                if ($m15Text.Contains($marker)) { $m15Hits.Add([pscustomobject]@{ marker = $marker; surface = $m15File }) }
            }
            $m15.marker_hits = @($m15Hits.ToArray())
            $m15.a4_zero_markers = ($m15Hits.Count -eq 0)
        }
        $extract = @{
            status = $m15Status
            schema_version = [string](Get-OptionalProperty -Object $m15Result -Name "schema_version")
            message = $m15Message
            timeline_count = $m15.timeline_count
            queue_sources_conclusions = $m15.queue_sources_conclusions
            latest_conclusion_id = $m15.latest_conclusion_id
        }
        $extract | ConvertTo-Json -Depth 6 | Set-Content -LiteralPath (Join-Path $RunRoot "m1_5_three_view_extract.json") -Encoding UTF8
    }
    if ($m15.invoke_ok) { Write-Ok "M15 mix_report invoke ok (mix_report.v1)" } else { Add-Failure "M15 FAIL: mix_report invoke did not return ok" }
    if ($m15.report_schema) { Write-Ok "M15 report schema_version = mix_report.v1" } else { Add-Failure "M15 FAIL: report schema_version mismatch" }
    if ($m15.settle_line) { Write-Ok "M15 latest settle conclusion line rendered" } else { Add-Failure "M15 FAIL: latest settle conclusion line missing" }
    if ($m15.queue_line) { Write-Ok "M15 queue node-state line rendered (23 nodes)" } else { Add-Failure "M15 FAIL: queue node-state line missing" }
    if ($m15.e1_deferred) { Write-Ok "M15 non-default node E1 deferred visible (rides M1Q q2 annotation)" } else { Add-Failure "M15 FAIL: E1 deferred not visible in the queue line" }
    if ($m15.fold_agree) { Write-Ok ("M15 fold agreement: report timeline=" + [string]$m15.timeline_count + " = queue sources=" + [string]$m15.queue_sources_conclusions) } else { Add-Failure ("M15 FAIL: folds disagree (report timeline=" + [string]$m15.timeline_count + " queue sources=" + [string]$m15.queue_sources_conclusions + ")") }
    if ($m15.settle_truth) { Write-Ok ("M15 settle line truthful (latest=" + $m15.latest_conclusion_id + ")") } else { Add-Failure "M15 FAIL: settle line content does not match the conclusion board state" }
    if ($m15.a4_zero_markers) { Write-Ok "M15 invoke surface clean of fixed-orchestration markers" } else {
        foreach ($hit in @($m15.marker_hits)) { Add-Failure ("M15 FAIL: marker '" + $hit.marker + "' hit on " + $hit.surface) }
    }
}
$report.m1_five_three_view = $m15

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
# Under -M1QueueRoundtrip the exit gate is the MIXLAYER-M1-2 surface (queue
# roundtrip + A1 + A4): the free-state round's A2/A3 outcome is recorded as
# round-status evidence, not an exit gate - that battle belongs to
# FS-LARGEPROJECT-SMOKE-1's rework line (4 runs 2026-10-10, best A1/A4 pass).
$roundStatusSoft = [bool]($M1QueueRoundtrip -or $C3Observation -or $D1D3Observation)
if ($a2) { Write-Ok ("A2 free-state station map: rounds=" + [string]$censusAfter.trajectory_round_started + " observations=" + [string]$censusAfter.trajectory_observation_recorded + " proposals/mix_ticks=" + [string]($censusAfter.trajectory_hypothesis_proposed + $censusAfter.mix_tick_pending)) }
elseif ($roundStatusSoft) { Write-WarnLine ("A2 not holding (recorded as round status, not an M1-2 exit gate): rounds=" + [string]$censusAfter.trajectory_round_started + " observations=" + [string]$censusAfter.trajectory_observation_recorded) }
else { Add-Failure ("A2 FAIL: station map not recognizable in event stream (rounds=" + [string]$censusAfter.trajectory_round_started + " observations=" + [string]$censusAfter.trajectory_observation_recorded + " proposals=" + [string]$censusAfter.trajectory_hypothesis_proposed + " mix_tick=" + [string]$censusAfter.mix_tick_pending + ")") }
if ($a3) { Write-Ok ("A3 AB audition round trip: card mounted, approve answered, audition events=" + [string]$censusAfter.audition_events + ", mix_tick_applied_reobserved+completed observed") }
elseif ($roundStatusSoft) { Write-WarnLine ("A3 not holding (recorded as round status, not an M1-2 exit gate): card_mounted=" + [string]$cardMounted + " approve_answered=" + [string]$approveAnswered) }
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
        if (-not $roundStatusSoft) {
            Add-Failure ("budget_exhausted stop with station-map/A3 not yet holding (partial evidence: A1=" + [string]$a1 + " A2=" + [string]$a2 + " A3=" + [string]$a3 + " A4=" + [string]$a4 + ")")
        }
    }
}
elseif (@($nlStopReasons.ToArray()) -contains "llm_error" -or @($nlStopReasons.ToArray()) -contains "model_protocol_failure") { $classification = "llm_error" }
elseif (@($nlStopReasons.ToArray()) -contains "capability_blocked") { $classification = "capability_blocked" }
# Review B follow-up (run 185519 lesson): the top-level stop reasons can read
# "done" while the free-state loop itself recorded an admission receipt - the
# receipt status is the machine-readable admission verdict and must feed the
# ladder, otherwise a receipt-blocked run classifies as "other" and diverges
# from the human classification. needs_experiment / improvement_proposal are
# continuation states, not failure classes, so they stay unconsumed.
elseif ($fsLoopAdmissionStatus -eq "capability_blocked" -or $fsLoopAdmissionStatus -eq "blocked") { $classification = "capability_blocked" }
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

$m1qGatesHold = ($m1q.q1_read_schema -and $m1q.q1_read_datasource -and $m1q.q2_write_ack -and $m1q.q3_reread_deferred -and $m1q.zero_fixed_orchestration_markers)
$m15GatesHold = ($m15.invoke_ok -and $m15.report_schema -and $m15.settle_line -and $m15.queue_line -and $m15.e1_deferred -and $m15.fold_agree -and $m15.settle_truth -and $m15.a4_zero_markers)
$c3GatesHold = ($c3obs.observation_reply_schema -and $c3obs.observation_readonly_line -and $c3obs.zero_fixed_orchestration_markers)
$d1d3GatesHold = ($d1d3obs.observation_reply_schema -and $d1d3obs.observation_face_lines -and $d1d3obs.observation_gap_line -and $d1d3obs.observation_readonly_line -and $d1d3obs.zero_fixed_orchestration_markers)
if ($roundStatusSoft) {
    # Exit-gate composition per opt-in surface: each opt-in phase contributes
    # its own gate set on top of the invariant A1+A4; A2/A3 stay round-status
    # evidence (FS-LARGEPROJECT-SMOKE-1 rework line owns them).
    $surfaceGatesHold = $true
    $surfaceGateNames = @()
    if ($M1QueueRoundtrip) {
        $surfaceGatesHold = ($surfaceGatesHold -and $m1qGatesHold)
        $surfaceGateNames += "M1Q"
    }
    if ($C3Observation) {
        $surfaceGatesHold = ($surfaceGatesHold -and $c3GatesHold)
        $surfaceGateNames += "C3OBS"
    }
    if ($D1D3Observation) {
        $surfaceGatesHold = ($surfaceGatesHold -and $d1d3GatesHold)
        $surfaceGateNames += "D1D3OBS"
    }
    if ($M1FiveThreeView) {
        $surfaceGatesHold = ($surfaceGatesHold -and $m15GatesHold)
        $surfaceGateNames += "M15"
    }
    $report.exit_gate_semantics = "surface gate: " + ($surfaceGateNames -join " + ") + " + A1 + A4 (A2/A3 round status recorded, not gated - FS-LARGEPROJECT-SMOKE-1 rework line owns them)"
    if ($failureReasons.Count -eq 0 -and $a1 -and $a4 -and $surfaceGatesHold) {
        $report.verdict = "PASS"
        $script:PlannedExitCode = 0
        Write-Report
        Write-Step "PASS"
        Write-Ok ("FS large-project smoke passed (" + ($surfaceGateNames -join " + ") + " + A1 + A4). run_root=" + $RunRoot)
        exit 0
    }
}
elseif ($failureReasons.Count -eq 0 -and $a1 -and $a2 -and $a3 -and $a4) {
    $report.verdict = "PASS"
    $script:PlannedExitCode = 0
    Write-Report
    Write-Step "PASS"
    Write-Ok ("FS large-project free-state smoke passed (four assertion groups). run_root=" + $RunRoot)
    exit 0
}
$report.verdict = "FAIL"
$report.failures = @($failureReasons.ToArray())
$script:PlannedExitCode = 1
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
    $script:PlannedExitCode = 1
    Write-Report
    exit 1
}
