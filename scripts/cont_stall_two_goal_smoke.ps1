#Requires -Version 5.1
<#
CONT-STALL-2 two-goal real-stack smoke (machine walk; the human hand test
still goes through the Godot entry, AGENTS.md section 5).

Real-stack shape under test (2026-09-13 22:59, session webui_mtzxugg8,
project 912.vit): inside ONE conversation the first goal ("请检查当前工程有什么问题")
stores a validated observation route (minimal_audio_closure + assessment) and
leaves its free-state loop active; the second goal then runs with
classificationRequired=false, and refreshCapabilityRouteForRevision's
conversation-level fallback injected the FIRST goal's route+assessment into
the second request context. durableContinuationFromResult lifted that foreign
assessment into the second goal's durable capacity state while the route index
holds no route for the second goal's own task id, so every reload fail-closed
the checkpoint into waiting_interaction ("durable capacity state has no
validated task route") and the scheduler could never claim it:
  [continuation.stall] reason=no-claimable-record non_terminal=1 stranded=1
  every 5 s until the user pressed stop (~3.5 min).

This script drives the same-type two-request shape on an isolated stack and
asserts, for the SECOND goal:
  M1 (mechanical, any branch)  its durable continuation, if any, does NOT
      carry the first goal's capability_route_decision / capacity state, and
      the runtime state holds NO non-terminal continuation fail-closed with
      reason "durable capacity state has no validated task route";
  C1 (conditional, waiting_continue branch) after the second goal's
      [continuation.arm], a scheduler-driven slice for THAT goal runs within
      -ArmBudgetSeconds (its own agent_loop_chat / [f6.gate] line), i.e. the
      chain is claimed and continues instead of stalling.

Verdicts:
  armed              M1 green + C1 green (scheduler slice for goal 2 ran)  exit 0
  finished           M1 green + goal 2 chain reached completed            exit 0
  parked_reported    M1 green + goal 2 parked at an answerable boundary
                     (waiting_confirmation/clarification)                  exit 0
  injection_leak     M1 red: goal 2's checkpoint carries the first goal's
                     route/assessment                                      exit 1
  dead_park          M1 red: a non-terminal continuation carries the
                     fail-closed recovery reason                           exit 1
  chain_failed/stopped/cancelled  honest terminal, not the stall shape     exit 1
  first_goal_terminal  goal 1 settled before goal 2 was sent: the defect
                     trigger (active loop) was absent this round; no verdict
                     is claimed for the injection path (mechanism stays
                     pinned by unit tests)                                 exit 2
  timeout / env_failure                                                    exit 2

Usage:
  powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\cont_stall_two_goal_smoke.ps1
  powershell ... -AgentBinary <path> -RunRoot <dir> -ArmBudgetSeconds 60 -MaxNudges 3
#>

[CmdletBinding()]
param(
    [string]$RepoRoot = "",
    [string]$RunRoot = "",
    [string]$Affix = "cont_stall2",
    [switch]$SkipBuild,
    [string]$AgentBinary = "",
    [string]$KernelExe = "",
    [int]$WaitSeconds = 30,
    [int]$KernelDwellSeconds = 15,
    [int]$FirstGoalBudgetSeconds = 240,
    [int]$ArmBudgetSeconds = 60,
    [int]$ChainBudgetSeconds = 180,
    [int]$PollSeconds = 3,
    # Same nudge discipline as cont_stall_repro_smoke.ps1: an answerable
    # clarification first is legitimate; nudges drive the chain on to the
    # limit_reached slice boundary the card names.
    [int]$MaxNudges = 3,
    [string]$FirstPromptBase64 = "",
    [string]$SecondPromptBase64 = ""
)

$ErrorActionPreference = "Stop"

function From-B64 {
    param([string]$Value)
    return [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String($Value))
}

# ASCII-safe literals (Windows PowerShell reads BOM-less .ps1 as ANSI; every
# non-ASCII string rides base64).
# first  = "请检查当前工程有什么问题"  (the real-stack first goal, verbatim)
$FirstPromptText = From-B64 "6K+35qOA5p+l5b2T5YmN5bel56iL5pyJ5LuA5LmI6Zeu6aKY"
if (-not [string]::IsNullOrWhiteSpace($FirstPromptBase64)) { $FirstPromptText = From-B64 $FirstPromptBase64 }
# second = "对低音轨做一次混音改进实验，改完让我 A/B 试听对比一下"
#          (the CONT-STALL-1 acceptance same-type prompt: it reliably drives
#           work slices to the turn budget boundary)
$SecondPromptText = From-B64 "5a+55L2O6Z+z6L2o5YGa5LiA5qyh5re36Z+z5pS56L+b5a6e6aqM77yM5pS55a6M6K6p5oiRIEEvQiDor5XlkKzlr7nmr5TkuIDkuIs="
if (-not [string]::IsNullOrWhiteSpace($SecondPromptBase64)) { $SecondPromptText = From-B64 $SecondPromptBase64 }
$NudgeText = From-B64 "56Gu6K6k5omn6KGM"   # "请继续" family approval nudge (same as cont_stall)
$FailClosedReason = "durable capacity state has no validated task route"

function Resolve-RepoRoot {
    param([string]$Explicit)
    if (-not [string]::IsNullOrWhiteSpace($Explicit)) { return (Resolve-Path -LiteralPath $Explicit).Path }
    return (Split-Path -Parent (Split-Path -Parent $MyInvocation.ScriptName))
}

function firstNonEmptyLocal { param([string]$A, [string]$B) if (-not [string]::IsNullOrWhiteSpace($A)) { return $A } else { return $B } }

function Write-Step { param([string]$Message) Write-Host ""; Write-Host ("== " + $Message) -ForegroundColor Cyan }
function Write-Ok { param([string]$Message) Write-Host ("ok: " + $Message) -ForegroundColor Green }
function Write-Bad { param([string]$Message) Write-Host ("RED: " + $Message) -ForegroundColor Red }
function Write-Info { param([string]$Message) Write-Host ("info: " + $Message) -ForegroundColor Gray }

function Get-TcpListener { param([int]$Port) return Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue | Select-Object -First 1 }

function Wait-PortListen {
    param([int]$Port, [int]$TimeoutSeconds)
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        if (Get-TcpListener -Port $Port) { return $true }
        Start-Sleep -Milliseconds 500
    }
    return $false
}

function Wait-HttpReady {
    param([string]$BaseUrl, [int]$TimeoutSeconds)
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        try {
            $resp = Invoke-WebRequest -UseBasicParsing -Uri ($BaseUrl.TrimEnd("/") + "/health") -TimeoutSec 2
            if ($resp.StatusCode -eq 200) { return $true }
        }
        catch { }
        Start-Sleep -Milliseconds 500
    }
    return $false
}

function Invoke-Json {
    param([string]$Method, [string]$Url, $Body, [int]$TimeoutSec = 60)
    if ($null -ne $Body) {
        $json = $Body | ConvertTo-Json -Depth 8 -Compress
        return Invoke-RestMethod -Method $Method -Uri $Url -ContentType "application/json; charset=utf-8" -Body ([System.Text.Encoding]::UTF8.GetBytes($json)) -TimeoutSec $TimeoutSec
    }
    return Invoke-RestMethod -Method $Method -Uri $Url -TimeoutSec $TimeoutSec
}

function Read-LogLines {
    param([string]$Path)
    if (-not (Test-Path -LiteralPath $Path)) { return @() }
    try {
        $stream = [System.IO.File]::Open($Path, [System.IO.FileMode]::Open, [System.IO.FileAccess]::Read, [System.IO.FileShare]::ReadWrite)
        try {
            $reader = New-Object System.IO.StreamReader($stream, [System.Text.Encoding]::UTF8)
            $text = $reader.ReadToEnd()
            $reader.Close()
        }
        finally { $stream.Close() }
        return @($text -split "\r?\n" | Where-Object { $_ -ne "" })
    }
    catch { return @() }
}

function Get-ProjectFingerprint {
    param([string]$Path)
    if (-not (Test-Path -LiteralPath $Path)) { return "absent" }
    if (Test-Path -LiteralPath $Path -PathType Leaf) { return (Get-FileHash -Algorithm SHA256 -LiteralPath $Path).Hash }
    $files = Get-ChildItem -LiteralPath $Path -Recurse -File -ErrorAction SilentlyContinue | Sort-Object FullName
    $sb = New-Object System.Text.StringBuilder
    foreach ($file in $files) {
        [void]$sb.Append($file.FullName.Substring($Path.Length))
        [void]$sb.Append(":")
        [void]$sb.Append($file.Length)
        [void]$sb.Append(":")
        [void]$sb.Append($file.LastWriteTimeUtc.ToString("o"))
        [void]$sb.Append(";")
    }
    $bytes = [System.Text.Encoding]::UTF8.GetBytes($sb.ToString())
    $sha = [System.Security.Cryptography.SHA256]::Create()
    try { return ([System.BitConverter]::ToString($sha.ComputeHash($bytes)) -replace "-", "") }
    finally { $sha.Dispose() }
}

# Read-RuntimeStateJson finds the newest agent_runtime_state.json under the
# isolated draft root and parses it.
function Read-RuntimeStateJson {
    param([string]$DraftRoot)
    $candidates = @(Get-ChildItem -LiteralPath $DraftRoot -Recurse -Filter "agent_runtime_state.json" -File -ErrorAction SilentlyContinue | Sort-Object LastWriteTime -Descending)
    if ($candidates.Count -eq 0) { return $null }
    try { return (Get-Content -LiteralPath $candidates[0].FullName -Raw -Encoding UTF8 | ConvertFrom-Json) }
    catch { return $null }
}

$RepoRoot = Resolve-RepoRoot -Explicit $RepoRoot
if ([string]::IsNullOrWhiteSpace($RunRoot)) {
    $RunRoot = Join-Path $RepoRoot ("artifacts\" + $Affix + "\" + (Get-Date -Format "yyyyMMdd_HHmmss"))
}
if (Test-Path -LiteralPath $RunRoot) { throw ("run root already exists; each run needs a fresh artifact directory: " + $RunRoot) }
New-Item -ItemType Directory -Force -Path $RunRoot | Out-Null
$AgentDrafts = Join-Path $RunRoot "agent_drafts"
New-Item -ItemType Directory -Force -Path $AgentDrafts | Out-Null
$AgentLog = Join-Path $RunRoot "agent_last.log"
$AgentDir = Join-Path $RepoRoot "agent"
$PrereqPath = Join-Path $RunRoot "prereq.txt"
$ReportPath = Join-Path $RunRoot "two_goal_report.json"
$KernelExeDefault = Join-Path $RepoRoot "VitApp\build\VitApp_artefacts\Release\VitApp.exe"
if ([string]::IsNullOrWhiteSpace($KernelExe)) { $KernelExe = $KernelExeDefault }
$RepoDefaultProject = Join-Path $RepoRoot "VitApp\Workspace\default_project.xml"
$UserProjectDir = "D:\Godot\project\vit-daw-frontend\912.vit"

$prereq = New-Object System.Collections.Generic.List[string]
function Add-Prereq { param([string]$Line) $script:prereq.Add($Line); Write-Host $Line }

$kernelProcId = $null
$agentProcId = $null
$outcome = "env_failure"
$detail = ""
$goal1ID = ""
$goal1TaskID = ""
$goal2ID = ""
$timeline = [ordered]@{}

try {
    Add-Prereq ("run_root=" + $RunRoot)
    Add-Prereq ("head=" + (& git -C $RepoRoot rev-parse HEAD))
    Add-Prereq ("git_status=" + ((& git -C $RepoRoot status --short) -join " ; "))

    Write-Step "Prereq: ports 7878/5555/5556 must be free"
    foreach ($port in 7878, 5555, 5556) {
        if (Get-TcpListener -Port $port) { throw ("port " + $port + " is already listening; aborting before any start (stack ownership rule)") }
        Add-Prereq ("port_free=" + $port)
    }

    if ([string]::IsNullOrWhiteSpace($AgentBinary)) { $AgentBinary = Join-Path $RunRoot "bin\VitAgent.contstall2.exe" }
    if (-not $SkipBuild) {
        Write-Step "Build VitAgent (isolated binary in run dir)"
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $AgentBinary) | Out-Null
        Push-Location $AgentDir
        try {
            & go build -o $AgentBinary .\cmd\vitagent
            if ($LASTEXITCODE -ne 0) { throw ("go build failed with exit code " + $LASTEXITCODE) }
        }
        finally { Pop-Location }
    }
    elseif (-not (Test-Path -LiteralPath $AgentBinary -PathType Leaf)) { throw "-SkipBuild given but isolated agent binary missing" }
    $agentItem = Get-Item -LiteralPath $AgentBinary
    Add-Prereq ("agent_binary=" + $AgentBinary + " size=" + $agentItem.Length + " mtime=" + $agentItem.LastWriteTime.ToString("o") + " sha256=" + (Get-FileHash -Algorithm SHA256 -LiteralPath $AgentBinary).Hash)

    Write-Step "Start kernel"
    $projectHashBefore = (Get-FileHash -Algorithm SHA256 -LiteralPath $RepoDefaultProject).Hash
    Add-Prereq ("repo_default_project_sha256_before=" + $projectHashBefore)
    $userProjectBefore = Get-ProjectFingerprint -Path $UserProjectDir
    Add-Prereq ("user_project_fingerprint_before=" + $userProjectBefore)
    $kernelProc = Start-Process -FilePath $KernelExe -WorkingDirectory (Split-Path -Parent $KernelExe) -WindowStyle Hidden -PassThru
    $kernelProcId = $kernelProc.Id
    if (-not (Wait-PortListen -Port 5555 -TimeoutSeconds $WaitSeconds)) { throw "kernel ZMQ REQ port 5555 did not listen" }
    Write-Ok ("kernel up, pid=" + $kernelProcId)

    Write-Step "Start agent (isolated draft root)"
    $priorDraftRoot = [Environment]::GetEnvironmentVariable("VIT_HISTORY_DRAFT_ROOT", "Process")
    $priorDevRoot = [Environment]::GetEnvironmentVariable("VIT_DAW_DEV_ROOT", "Process")
    $env:VIT_HISTORY_DRAFT_ROOT = $AgentDrafts
    $env:VIT_DAW_DEV_ROOT = $RepoRoot
    try {
        $agentProc = Start-Process -FilePath $AgentBinary -ArgumentList @("-http", "127.0.0.1:7878", "-last-log-path", $AgentLog, "-keep-last-log-lines", "8000") -WorkingDirectory $AgentDir -WindowStyle Hidden -PassThru
    }
    finally {
        if ($null -eq $priorDraftRoot) { Remove-Item Env:VIT_HISTORY_DRAFT_ROOT -ErrorAction SilentlyContinue } else { $env:VIT_HISTORY_DRAFT_ROOT = $priorDraftRoot }
        if ($null -eq $priorDevRoot) { Remove-Item Env:VIT_DAW_DEV_ROOT -ErrorAction SilentlyContinue } else { $env:VIT_DAW_DEV_ROOT = $priorDevRoot }
    }
    $agentProcId = $agentProc.Id
    if (-not (Wait-HttpReady -BaseUrl "http://127.0.0.1:7878" -TimeoutSeconds $WaitSeconds)) { throw "agent HTTP did not become ready" }
    Write-Ok ("agent up, pid=" + $agentProcId)

    $base = "http://127.0.0.1:7878"
    $state = Invoke-Json -Method GET -Url ($base + "/agent/state") -Body $null -TimeoutSec 30
    if ($state.shadow.initialized -ne $true) { throw "shadow not initialized after stack start" }
    Add-Prereq ("s1_shadow_initialized=" + $state.shadow.initialized + " track_count=" + $state.shadow.track_count)

    if ($KernelDwellSeconds -gt 0) {
        Write-Step ("Kernel warm-up dwell " + $KernelDwellSeconds + "s")
        Start-Sleep -Seconds $KernelDwellSeconds
    }

    Write-Step "Full project access"
    $auth = Invoke-Json -Method POST -Url ($base + "/agent/authority") -Body @{ authority_mode = "full_project_access" } -TimeoutSec 30
    if ($auth.authority_mode -ne "full_project_access") { throw ("authority switch refused: " + ($auth | ConvertTo-Json -Compress)) }
    Add-Prereq ("authority_mode=" + $auth.authority_mode)

    # ---------------------------------------------------------------- goal 1
    $conversationID = "contstall2_" + (Get-Date -Format "yyyyMMdd_HHmmss")
    Write-Step ("Goal 1 (validated-route seeder) on " + $conversationID)
    $goal1Started = Get-Date
    $goal1Response = Invoke-Json -Method POST -Url ($base + "/agent/chat") -Body @{ conversation_id = $conversationID; message = $FirstPromptText } -TimeoutSec 300
    $goal1ID = [string]$goal1Response.goal_id
    $goal1TaskID = [string]$goal1Response.task_id
    $goal1Status = [string]$goal1Response.goal_status
    Add-Prereq ("goal1=" + $goal1ID + " task=" + $goal1TaskID + " status=" + $goal1Status + " stop=" + [string]$goal1Response.stop_reason)
    $timeline["goal1_status_after_http"] = $goal1Status
    $goal1Response | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath (Join-Path $RunRoot "goal1_response.json") -Encoding UTF8
    if ($goal1ID -eq "") { throw "goal 1 did not start" }

    # Goal 1 must leave an ACTIVE-but-idle boundary behind (the 22:59 real-stack
    # trigger: loop alive, no running slice). While goal 1 sits at
    # waiting_continue its auto-continuation chain keeps running, and a second
    # request then merges into the SAME goal (2026-09-14 09:35 run: goal 2
    # returned goal 1's id) -- that is a legitimate resume, not the two-goal
    # defect shape. Wait for an answerable boundary (waiting_confirmation /
    # waiting_clarification): loop still active, scheduler idle. Terminal
    # statuses mean the loop settled and the defect trigger is absent.
    $deadline1 = (Get-Date).AddSeconds($FirstGoalBudgetSeconds)
    $goal1Boundary = ""
    while ((Get-Date) -lt $deadline1) {
        try {
            $rt = Invoke-Json -Method GET -Url ($base + "/agent/runtime/status") -Body $null -TimeoutSec 30
            $goal1Status = [string]$rt.goal.status
        } catch { Start-Sleep -Seconds $PollSeconds; continue }
        if ($goal1Status -eq "waiting_confirmation" -or $goal1Status -eq "waiting_clarification") { $goal1Boundary = $goal1Status; break }
        if ($goal1Status -eq "completed" -or $goal1Status -eq "failed" -or $goal1Status -eq "stopped" -or $goal1Status -eq "cancelled") { break }
        Start-Sleep -Seconds $PollSeconds
    }
    $timeline["goal1_status_at_second_send"] = $goal1Status
    $timeline["goal1_boundary"] = $goal1Boundary
    if ($goal1Status -eq "completed" -or $goal1Status -eq "failed" -or $goal1Status -eq "stopped" -or $goal1Status -eq "cancelled") {
        # Goal 1 settled before goal 2 was sent, so the active-loop injection
        # trigger was absent and goal 2 will run its own semantic entry (the
        # healthy main-chain path). Still drive goal 2: the acceptance claim
        # "second goal arms and is claimed, no stall flood" is asserted in
        # this shape too, classified as post_terminal.
        $timeline["second_send_shape"] = "post_terminal"
        Write-Info ("goal 1 settled to " + $goal1Status + " before goal 2; driving goal 2 through the main chain (post_terminal shape)")
    }

    # ---------------------------------------------------------------- goal 2
    if ($outcome -eq "env_failure") {
        Write-Step ("Goal 2 (the defect shape) on the same conversation, goal1 status=" + $goal1Status)
        $sliceIndex = 0
        $chatGoal2Status = ""
        $goal2Response = $null
        $verdict = ""
        while ($sliceIndex -le $MaxNudges) {
            $message = if ($sliceIndex -eq 0) { $SecondPromptText } else { $NudgeText }
            $sliceStarted = Get-Date
            $goal2Response = Invoke-Json -Method POST -Url ($base + "/agent/chat") -Body @{ conversation_id = $conversationID; message = $message } -TimeoutSec 300
            $logIndexAtEnd = @(Read-LogLines -Path $AgentLog).Count
            if ($sliceIndex -eq 0) { $goal2ID = [string]$goal2Response.goal_id } else { $goal2ID = firstNonEmptyLocal $goal2ID ([string]$goal2Response.goal_id) }
            $chatGoal2Status = [string]$goal2Response.goal_status
            Add-Prereq ("goal2_slice_" + $sliceIndex + "_goal=" + $goal2ID + " status=" + $chatGoal2Status + " stop=" + [string]$goal2Response.stop_reason)
            Write-Ok ("goal 2 slice " + $sliceIndex + ": status=" + $chatGoal2Status)
            $goal2Response | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath (Join-Path $RunRoot ("goal2_response." + $sliceIndex + ".json")) -Encoding UTF8
            if ($chatGoal2Status -eq "waiting_continue") {
                Write-Step ("Arming window " + $ArmBudgetSeconds + "s for goal 2's scheduler slice")
                $armDeadline = (Get-Date).AddSeconds($ArmBudgetSeconds)
                $goal2SliceLine = ""
                while ((Get-Date) -lt $armDeadline) {
                    Start-Sleep -Seconds $PollSeconds
                    $lines = @(Read-LogLines -Path $AgentLog)
                    for ($i = $logIndexAtEnd; $i -lt $lines.Count; $i++) {
                        $line = $lines[$i]
                        # A scheduler-driven slice FOR GOAL 2 (not goal 1's chain):
                        # its own agent_loop_chat line or chain-end gate line.
                        $isGoal2Slice = (($line -like "*agent_loop_chat*" -or $line -like "*[f6.gate]*") -and $line -like ("*goal=" + $goal2ID + "*"))
                        if ($isGoal2Slice) { $goal2SliceLine = $line; break }
                    }
                    if ($goal2SliceLine -ne "") { break }
                    try {
                        $rt = Invoke-Json -Method GET -Url ($base + "/agent/runtime/status") -Body $null -TimeoutSec 30
                        $gs = [string]$rt.goal.status
                        if ($gs -eq "completed") { $verdict = "finished"; break }
                        if ($gs -eq "failed" -or $gs -eq "stopped" -or $gs -eq "cancelled") { $verdict = "chain_" + $gs; break }
                    } catch { }
                }
                if ($goal2SliceLine -ne "") {
                    $verdict = "armed"
                    $timeline["goal2_slice_line"] = $goal2SliceLine
                }
                elseif ([string]::IsNullOrWhiteSpace($verdict)) { $verdict = "dead_park_candidate" }
                break
            }
            if ($chatGoal2Status -eq "completed") { $verdict = "finished"; break }
            if ($chatGoal2Status -eq "failed" -or $chatGoal2Status -eq "stopped" -or $chatGoal2Status -eq "cancelled") { $verdict = "chain_" + $chatGoal2Status; break }
            if ($chatGoal2Status -eq "waiting_clarification" -or $chatGoal2Status -eq "waiting_confirmation") {
                $verdict = "parked_reported"
                break
            }
            $sliceIndex++
        }
        $timeline["goal2_id"] = $goal2ID
        $timeline["goal2_verdict_pre_mechanical"] = $verdict

        # ------------------------------------------------------------ M1
        Start-Sleep -Seconds 5   # let one more reload tick fail-close anything it would fail-close
        Write-Step "M1: mechanical injection / fail-closed scan of the runtime state"
        $runtimeState = Read-RuntimeStateJson -DraftRoot $AgentDrafts
        $foreignInjected = $false
        $failClosedRecords = @()
        if ($null -eq $runtimeState) {
            Write-Info "no agent_runtime_state.json found under the draft root"
        }
        else {
            $durable = @()
            if ($null -ne $runtimeState.durable_continuations) {
                if ($runtimeState.durable_continuations -is [System.Array]) { $durable = @($runtimeState.durable_continuations) }
                else {
                    # map form: unwrap property values
                    $durable = @($runtimeState.durable_continuations.PSObject.Properties | ForEach-Object { $_.Value })
                }
            }
            Add-Prereq ("durable_continuations=" + $durable.Count)
            foreach ($item in $durable) {
                $itemGoal = [string]$item.goal_id
                $itemStatus = [string]$item.status
                $pendingReason = ""
                if ($null -ne $item.pending_interaction) { $pendingReason = [string]$item.pending_interaction.reason }
                $ctx = $null
                if ($null -ne $item.continuation) { $ctx = $item.continuation.context }
                $ctxRouteTask = ""
                $ctxRouteGoal = ""
                $ctxHasAssessment = $false
                if ($null -ne $ctx -and $null -ne $ctx.capability_route_decision) {
                    $ctxRouteTask = [string]$ctx.capability_route_decision.task_id
                    $ctxRouteGoal = [string]$ctx.capability_route_decision.goal_id
                    $ctxHasAssessment = ($null -ne $ctx.capability_route_decision.capacity_assessment) -or ($null -ne $ctx.free_state_capacity_assessment)
                }
                $hasDurableAssessment = ($null -ne $item.capacity_assessment)
                Add-Prereq ("durable " + [string]$item.continuation_id + " goal=" + $itemGoal + " status=" + $itemStatus + " ctx_route_task=" + $ctxRouteTask + " ctx_assessment=" + $ctxHasAssessment + " durable_assessment=" + $hasDurableAssessment + " reason=" + $pendingReason)
                if ($itemGoal -eq $goal2ID -and $goal2ID -ne $goal1ID) {
                    # goal2 == goal1 means the second request merged into goal
                    # 1's running continuation chain (a legitimate resume); the
                    # injection criterion does not apply there. For a genuine
                    # new goal, a foreign route in the context is recorded as
                    # an observation: the CONT-STALL-2 fix keeps the owner-
                    # restoration injection (legitimate for same-controller
                    # resumes) but makes reconcile ignore capacity state that
                    # rides a foreign identity, so injection alone is not a
                    # verdict -- the dead park is judged by the fail-closed
                    # scan and the no-claimable-record scan below.
                    if (($ctxRouteTask -ne "" -and $ctxRouteTask -eq $goal1TaskID) -or ($ctxRouteGoal -ne "" -and $ctxRouteGoal -eq $goal1ID)) {
                        $foreignInjected = $true
                        $timeline["foreign_route_injected_continuation"] = [string]$item.continuation_id
                        Add-Prereq ("foreign_route_injected " + [string]$item.continuation_id + " status=" + $itemStatus)
                    }
                }
                if ($pendingReason -eq $FailClosedReason -and $itemStatus -ne "completed" -and $itemStatus -ne "cancelled" -and $itemStatus -ne "failed") {
                    $failClosedRecords += ([string]$item.continuation_id + " goal=" + $itemGoal)
                }
            }
        }
        # -match, not -like: "[continuation.stall]" in a -like pattern is a
        # character class and matches almost every line (2026-09-14 09:35 run:
        # stall_lines=108 was the whole log).
        $stallLines = @(Read-LogLines -Path $AgentLog | Where-Object { $_ -match "\[continuation\.stall\]" })
        $noClaimable = @($stallLines | Where-Object { $_ -like "*reason=no-claimable-record*" })
        $stallLines | Set-Content -LiteralPath (Join-Path $RunRoot "stall_log.txt") -Encoding UTF8
        Add-Prereq ("stall_lines=" + $stallLines.Count)
        Add-Prereq ("stall_no_claimable_record=" + $noClaimable.Count)
        if ($noClaimable.Count -gt 0 -and $outcome -ne "dead_park" -and $outcome -ne "injection_leak") {
            # Repeated no-claimable-record scans with a non-terminal goal are
            # the 22:59 screen-flood shape even when no fail-closed reason was
            # persisted yet; surface it instead of passing silently.
            $outcome = "dead_park"
            $detail = "no-claimable-record stall scan x" + $noClaimable.Count + " (the 22:59 flood shape)"
            Write-Bad $detail
        }

        if ($failClosedRecords.Count -gt 0) {
            $outcome = "dead_park"
            $detail = "non-terminal fail-closed continuations: " + ($failClosedRecords -join " ; ")
            Write-Bad $detail
        }
        else {
            Write-Ok "M1 green: no fail-closed residency (foreign_route_injected=$foreignInjected is inert by design)"
            switch ($verdict) {
                "armed" { $outcome = "armed" }
                "finished" { $outcome = "finished" }
                "parked_reported" { $outcome = "parked_reported" }
                "dead_park_candidate" {
                    # No goal-2 slice within budget AND no fail-closed record:
                    # distinguish a truly answerable park from a silent one via
                    # the runtime goal status.
                    try {
                        $rt = Invoke-Json -Method GET -Url ($base + "/agent/runtime/status") -Body $null -TimeoutSec 30
                        $gs = [string]$rt.goal.status
                        $timeline["final_goal_status"] = $gs
                        if ($gs -eq "completed") { $outcome = "finished" }
                        elseif ($gs -eq "waiting_confirmation" -or $gs -eq "waiting_clarification") { $outcome = "parked_reported" }
                        else { $outcome = "dead_park"; $detail = "goal 2 armed but no scheduler slice within " + $ArmBudgetSeconds + "s; final goal status " + $gs }
                    }
                    catch { $outcome = "dead_park"; $detail = "goal 2 armed but no scheduler slice; runtime status unreadable" }
                }
                default { $outcome = $verdict }
            }
        }
    }
}
catch {
    $detail = $_.Exception.Message
    Add-Prereq ("fatal=" + $detail)
    Add-Prereq ("fatal_where=" + [string]$_.InvocationInfo.PositionMessage)
    Write-Bad ("fatal: " + $detail)
}
finally {
    Write-Step "Teardown"
    if ($agentProcId) { Stop-Process -Id $agentProcId -Force -ErrorAction SilentlyContinue; Add-Prereq ("stopped_agent_pid=" + $agentProcId) }
    if ($kernelProcId) { Stop-Process -Id $kernelProcId -Force -ErrorAction SilentlyContinue; Add-Prereq ("stopped_kernel_pid=" + $kernelProcId) }
    Start-Sleep -Seconds 2
    foreach ($port in 7878, 5555, 5556) {
        if (Get-TcpListener -Port $port) { Add-Prereq ("port_still_busy=" + $port) } else { Add-Prereq ("port_released=" + $port) }
    }
    if (Test-Path -LiteralPath $RepoDefaultProject) { Add-Prereq ("repo_default_project_sha256_after=" + (Get-FileHash -Algorithm SHA256 -LiteralPath $RepoDefaultProject).Hash) }
    Add-Prereq ("user_project_fingerprint_after=" + (Get-ProjectFingerprint -Path $UserProjectDir))
    Add-Prereq ("finished=" + (Get-Date -Format "o"))
    $prereq | Out-File -FilePath $PrereqPath -Encoding utf8
    $report = [ordered]@{
        schema_version = "cont_stall_two_goal.v1"
        run_root       = $RunRoot
        goal1_id       = $goal1ID
        goal1_task_id  = $goal1TaskID
        goal2_id       = $goal2ID
        verdict        = $outcome
        detail         = $detail
        timeline       = $timeline
        prereq         = @($prereq)
    }
    $report | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath $ReportPath -Encoding UTF8
}

Write-Host ""
Write-Host ("CONT_STALL2_VERDICT " + $outcome)
switch ($outcome) {
    "armed" { exit 0 }
    "finished" { exit 0 }
    "parked_reported" { exit 0 }
    "injection_leak" { exit 1 }
    "dead_park" { exit 1 }
    "chain_failed" { exit 1 }
    "chain_stopped" { exit 1 }
    "chain_cancelled" { exit 1 }
    default { exit 2 }
}
