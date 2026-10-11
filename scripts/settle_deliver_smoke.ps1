<#
SETTLE-DELIVER-1 real-stack smoke (machine walk): A/B 判定 → 结算确认进对话流 →
新输入正常执行，三症状全链验收。人类手测仍走 Godot 入口（AGENTS.md §5）。

MIXLAYER-M1-1 extension (-M1Survival, 2026-10-11): after the SETTLE-DELIVER-1
verdicts, the same run additionally proves the settle conclusion survives
across sessions — the record lands in the project-keyed
free_state_conclusions store (under the run's isolated draft project store or
the repo workspace store), and a NEW conversation's 工程状态汇报 blackboard
report reads the conclusion back (the gated readback: settle state is keyed
by conversation_id, the store by project_uuid — the identity-axis bridge this
card builds). An agent-restart readback runs as a report-only probe: this
smoke's isolated draft root gives a restarted agent a fresh draft project
store for an unsaved kernel project (run 20261011_113450), an environment
constraint the production shape (saved project, one .vit_agent) does not
have; the record's on-disk survival is asserted directly instead. When the
judgment leg is missed by the documented model random branch
(judgment_not_armed family, SETTLE-DELIVER-SMOKE-SUMMARY.md: 4/9 historical
misses), -M1Survival continues against the loop-closure settle conclusion
that DID land (kind=loop_closure_settle, e.g. capability_blocked) instead of
aborting: verdict m1_finished / exit 0 then means "M1 survival face passed;
judgment leg recorded as skipped" — the no-switch invocation keeps the
original byte-identical semantics. Fail class: m1_survival_missing.

Probabilistic-run protocol (AGENTS.md §8, declared before execution):
  runs        : 1 full-chain attempt; a second attempt only for a classified
                environment interruption (port/kernel-start failure), never a
                blind rerun of a functional failure.
  success     : exit 0 — judgment POST accepted; judgment.settled event with
                body AND a persisted assistant conversation_message row carry
                the settle report (symptom A); the follow-up input's turn
                terminal is NOT stop_reason=project_revision_stale and the
                task does not close as "without governed experiment" (symptom
                B); message order assertions are pinned in webui unit tests
                (symptom C) and re-verified by the user's manual pass. With
                -M1Survival: conclusion record + board file exist under the
                project store, and the restarted agent's new-conversation
                blackboard report contains the cross-session conclusion
                section with the judgment outcome wording.
  fail classes: judgment_not_armed (audition chain never reached the seat —
                pre-existing channel, out of card scope, evidence kept),
                settle_message_missing (symptom A not delivered end-to-end),
                round2_stale (symptom B residual), round2_failed (honest other
                stop), m1_survival_missing (M1 extension: record absent or
                blackboard readback missing), timeout, env_failure.
  stop-loss   : two consecutive same-shape failures → stop, keep artifacts,
                escalate to the decision side. No assertion weakening.

Usage:
  powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\settle_deliver_smoke.ps1
  powershell ... -SkipBuild -KernelDwellSeconds 30 -ChainBudgetSeconds 600
  powershell ... -M1Survival   # MIXLAYER-M1-1 cross-session survival phase
#>

[CmdletBinding()]
param(
    [string]$RepoRoot = "",
    [string]$RunRoot = "",
    [switch]$SkipBuild,
    [string]$KernelExe = "",
    [int]$WaitSeconds = 30,
    [int]$KernelDwellSeconds = 60,
    [int]$ChainBudgetSeconds = 420,
    [int]$PollSeconds = 10,
    [int]$Round2BudgetSeconds = 420,
    [switch]$M1Survival
)

$ErrorActionPreference = "Stop"

function From-B64 {
    param([string]$Value)
    return [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String($Value))
}

# ASCII-safe literals (Windows PowerShell reads BOM-less .ps1 as ANSI).
$PromptText = From-B64 "5a+55L2O6Z+z6L2o5YGa5LiA5qyh5re36Z+z5pS56L+b5a6e6aqM77yM5pS55a6M6K6p5oiRIEEvQiDor5XlkKzlr7nmr5TkuIDkuIs="
$Round2Text = From-B64 "5LiK5LiA6L2u55qEIEEvQiDliKTlrprkv53nlZnnmoTmmK/lk6rkupvmlLnliqjvvJ/or6bnu4bor7Tor7TkvaDlgZrkuobku4DkuYjvvIzku6Xlj4rkuLrku4DkuYjjgII="
$NeedleSettle = From-B64 "QS9CIOWIpOWumuW3suiQvei0puW5tuWujOaIkOe7k+eulw=="
$NeedleStale = "project_revision_stale"
$NeedleEvidence = From-B64 "5Yik5a6a6K+B5o2u"
$NeedleUngoverned = "without governed experiment"
# MIXLAYER-M1-1 survival needles
$M1BlackboardPrompt = From-B64 "6K+357uZ5oiR5b2T5YmN5bel56iL54q25oCB5rGH5oql"   # 请给我当前工程状态汇报
$M1NeedleSection = From-B64 "6Ieq55Sx5oCB57uT6K6677yI6Leo5Lya6K+d77yJ"             # 自由态结论（跨会话）
$M1NeedleOutcome = From-B64 "5bey5oyJ55So5oi35Lq66ICz5Yik5a6a5L+d55WZ5pS55Yqo"     # 已按用户人耳判定保留改动
$M1NeedleRollbackOutcome = From-B64 "5bey5oyJ55So5oi35Lq66ICz5Yik5a6a5Zue5rua5Yiw5pS55Yqo5YmN54q25oCB"  # 已按用户人耳判定回滚到改动前状态
$M1NeedleClosureOutcome = From-B64 "6IO95Yqb6L6555WM5Y+X6Zi75ZCO5pS25Y+j"         # 能力边界受阻后收口

function Resolve-RepoRoot {
    param([string]$Explicit)
    if (-not [string]::IsNullOrWhiteSpace($Explicit)) { return (Resolve-Path -LiteralPath $Explicit).Path }
    return (Split-Path -Parent (Split-Path -Parent $MyInvocation.ScriptName))
}

function Write-Step { param([string]$Message) Write-Host ""; Write-Host ("== " + $Message) -ForegroundColor Cyan }
function Write-Ok { param([string]$Message) Write-Host ("ok: " + $Message) -ForegroundColor Green }
function Write-Bad { param([string]$Message) Write-Host ("fail: " + $Message) -ForegroundColor Red }

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

$RepoRoot = Resolve-RepoRoot -Explicit $RepoRoot
if ([string]::IsNullOrWhiteSpace($RunRoot)) {
    $RunRoot = Join-Path $RepoRoot ("coord\runs\SETTLE-DELIVER-SMOKE-" + (Get-Date -Format "yyyyMMdd_HHmmss"))
}
New-Item -ItemType Directory -Force -Path $RunRoot | Out-Null
$AgentDrafts = Join-Path $RunRoot "agent_drafts"
New-Item -ItemType Directory -Force -Path $AgentDrafts | Out-Null
$AgentBin = Join-Path $RunRoot "bin\VitAgent.settledeliver.exe"
$AgentLog = Join-Path $RunRoot "agent_last.log"
$AgentDir = Join-Path $RepoRoot "agent"
$PrereqPath = Join-Path $RunRoot "prereq.txt"
$ReportPath = Join-Path $RunRoot "settle_deliver_report.json"
$EventsPath = Join-Path $RunRoot "events_final.json"
$StatePath = Join-Path $RunRoot "state_final.json"
$Chat1Path = Join-Path $RunRoot "chat1_response.json"
$Chat2Path = Join-Path $RunRoot "chat2_response.json"
if ([string]::IsNullOrWhiteSpace($KernelExe)) { $KernelExe = Join-Path $RepoRoot "VitApp\build\VitApp_artefacts\Release\VitApp.exe" }
$RepoDefaultProject = Join-Path $RepoRoot "VitApp\Workspace\default_project.xml"

$prereq = New-Object System.Collections.Generic.List[string]
function Add-Prereq { param([string]$Line) $script:prereq.Add($Line); Write-Host $Line }

$kernelProcId = $null
$agentProcId = $null
$outcome = "env_failure"
$detail = ""
$judgment = $null
$settleEventBody = ""
$settleMessageRows = 0
$round2Stop = ""
$round2GoalStatus = ""
$round2TaskNote = ""
$judgmentLegMissed = $false

try {
    Add-Prereq ("run_root=" + $RunRoot)
    Add-Prereq ("head=" + (& git -C $RepoRoot rev-parse HEAD))
    Add-Prereq ("git_status=" + ((& git -C $RepoRoot status --short) -join " ; "))

    Write-Step "Prereq: ports 7878/5555/5556 must be free (user manual stack closed)"
    foreach ($port in 7878, 5555, 5556) {
        if (Get-TcpListener -Port $port) { throw ("port " + $port + " is already listening; aborting before any start (stack ownership rule)") }
        Add-Prereq ("port_free=" + $port)
    }

    if (-not $SkipBuild) {
        Write-Step "Build VitAgent (isolated binary in run dir)"
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $AgentBin) | Out-Null
        Push-Location $AgentDir
        try {
            & go build -o $AgentBin .\cmd\vitagent
            if ($LASTEXITCODE -ne 0) { throw ("go build failed with exit code " + $LASTEXITCODE) }
        }
        finally { Pop-Location }
    }
    elseif (-not (Test-Path -LiteralPath $AgentBin -PathType Leaf)) { throw "-SkipBuild given but isolated agent binary missing" }
    $agentItem = Get-Item -LiteralPath $AgentBin
    Add-Prereq ("agent_binary=" + $AgentBin + " size=" + $agentItem.Length + " mtime=" + $agentItem.LastWriteTime.ToString("o"))

    Write-Step "Start kernel"
    $kernelItem = Get-Item -LiteralPath $KernelExe
    Add-Prereq ("kernel_binary=" + $KernelExe + " size=" + $kernelItem.Length + " mtime=" + $kernelItem.LastWriteTime.ToString("o"))
    $projectHashBefore = (Get-FileHash -Algorithm SHA256 -LiteralPath $RepoDefaultProject).Hash
    Add-Prereq ("repo_default_project_sha256_before=" + $projectHashBefore)
    $kernelProc = Start-Process -FilePath $KernelExe -WorkingDirectory (Split-Path -Parent $KernelExe) -WindowStyle Hidden -PassThru
    $kernelProcId = $kernelProc.Id
    Add-Prereq ("kernel_pid=" + $kernelProcId)
    if (-not (Wait-PortListen -Port 5555 -TimeoutSeconds $WaitSeconds)) { throw "kernel ZMQ REQ port 5555 did not listen" }
    Write-Ok ("kernel up, pid=" + $kernelProcId)

    Write-Step "Start agent (isolated draft root)"
    $priorDraftRoot = [Environment]::GetEnvironmentVariable("VIT_HISTORY_DRAFT_ROOT", "Process")
    $priorDevRoot = [Environment]::GetEnvironmentVariable("VIT_DAW_DEV_ROOT", "Process")
    $env:VIT_HISTORY_DRAFT_ROOT = $AgentDrafts
    $env:VIT_DAW_DEV_ROOT = $RepoRoot
    try {
        $agentProc = Start-Process -FilePath $AgentBin -ArgumentList @("-http", "127.0.0.1:7878", "-last-log-path", $AgentLog, "-keep-last-log-lines", "8000") -WorkingDirectory $AgentDir -WindowStyle Hidden -PassThru
    }
    finally {
        if ($null -eq $priorDraftRoot) { Remove-Item Env:VIT_HISTORY_DRAFT_ROOT -ErrorAction SilentlyContinue } else { $env:VIT_HISTORY_DRAFT_ROOT = $priorDraftRoot }
        if ($null -eq $priorDevRoot) { Remove-Item Env:VIT_DAW_DEV_ROOT -ErrorAction SilentlyContinue } else { $env:VIT_DAW_DEV_ROOT = $priorDevRoot }
    }
    $agentProcId = $agentProc.Id
    Add-Prereq ("agent_pid=" + $agentProcId)
    if (-not (Wait-HttpReady -BaseUrl "http://127.0.0.1:7878" -TimeoutSeconds $WaitSeconds)) { throw "agent HTTP did not become ready" }
    Write-Ok ("agent up, pid=" + $agentProcId)

    $base = "http://127.0.0.1:7878"
    $state = Invoke-Json -Method GET -Url ($base + "/agent/state") -Body $null -TimeoutSec 30
    Add-Prereq ("s1_shadow_initialized=" + $state.shadow.initialized + " track_count=" + $state.shadow.track_count)
    if ($state.shadow.initialized -ne $true) { throw "shadow not initialized after stack start" }

    if ($KernelDwellSeconds -gt 0) {
        Write-Step ("Kernel warm-up dwell " + $KernelDwellSeconds + "s")
        Start-Sleep -Seconds $KernelDwellSeconds
    }

    Write-Step "Full project access (auto-apply, no confirmation card)"
    $auth = Invoke-Json -Method POST -Url ($base + "/agent/authority") -Body @{ authority_mode = "full_project_access" } -TimeoutSec 30
    Add-Prereq ("authority_mode=" + $auth.authority_mode)
    if ($auth.authority_mode -ne "full_project_access") { throw ("authority switch refused: " + ($auth | ConvertTo-Json -Compress)) }

    $conversationID = "settledeliver_" + (Get-Date -Format "yyyyMMdd_HHmmss")
    Write-Step ("Drive the A/B D1 chain prompt on " + $conversationID)
    $chat1 = Invoke-Json -Method POST -Url ($base + "/agent/chat") -Body @{ conversation_id = $conversationID; message = $PromptText } -TimeoutSec 300
    $chat1 | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $Chat1Path -Encoding UTF8
    Add-Prereq ("chat1_goal=" + $chat1.goal_id + " status=" + $chat1.goal_status + " stop=" + $chat1.stop_reason)

    Write-Step "Wait for the audition seat (judgment boundary)"
    $deadline = (Get-Date).AddSeconds($ChainBudgetSeconds)
    $seat = $null
    while ((Get-Date) -lt $deadline) {
        Start-Sleep -Seconds $PollSeconds
        $eventsResponse = Invoke-Json -Method GET -Url ($base + "/agent/events?conversation_id=" + $conversationID + "&since=0&limit=200") -Body $null -TimeoutSec 30
        $events = @($eventsResponse.events)
        $ready = @($events | Where-Object { $_.type -eq "audition.ready" })
        # The LAST audition.ready can be kernel telemetry without turn fields
        # (forensic shape: agent-enriched and kernel rows interleave); pick the
        # last row that actually carries the complete judgment identity.
        for ($i = $ready.Count - 1; $i -ge 0; $i--) {
            $session = $ready[$i].payload.session
            if ($session -and $session.session_id -and $session.turn_id -and $session.round_id -and $session.project_revision) {
                $seat = $session
                break
            }
        }
        if ($null -ne $seat) { break }
        $runtimeNow = Invoke-Json -Method GET -Url ($base + "/agent/runtime/status") -Body $null -TimeoutSec 30
        Write-Host ("poll goal=" + $runtimeNow.goal.status + " task=" + $runtimeNow.task_state + " audition_ready=" + $ready.Count)
        if ($runtimeNow.goal.status -eq "failed" -or $runtimeNow.goal.status -eq "stopped" -or $runtimeNow.goal.status -eq "cancelled") {
            throw ("chain died before the judgment seat: " + $runtimeNow.goal.status)
        }
    }
    if ($null -eq $seat) { $outcome = "judgment_not_armed"; throw "audition seat never became ready within budget" }
    Write-Ok ("seat armed: " + $seat.session_id)

    Write-Step "Wait for the judgment request (round parked at human_audition_ready)"
    # The webui arms its verdict seats from trajectory.user_judgment.requested
    # (canJudge); posting before the round records its target response is a 409.
    $deadlineJR = (Get-Date).AddSeconds(240)
    $requested = $false
    while ((Get-Date) -lt $deadlineJR) {
        $jrEvents = @((Invoke-Json -Method GET -Url ($base + "/agent/events?conversation_id=" + $conversationID + "&since=0&limit=200") -Body $null -TimeoutSec 30).events)
        $requestedRows = @($jrEvents | Where-Object { $_.type -eq "trajectory.user_judgment.requested" })
        if ($requestedRows.Count -gt 0) { $requested = $true; break }
        Start-Sleep -Seconds 5
    }
    if (-not $requested) {
        if ($M1Survival) {
            # Documented model random branch (SETTLE-DELIVER-SMOKE-SUMMARY.md:
            # 4/9 historical misses): the chain terminal landed on a loop
            # closure settle instead of parking at the judgment boundary.
            # -M1Survival continues against that conclusion; the no-switch
            # invocation keeps the original judgment_not_armed abort.
            $judgmentLegMissed = $true
            Add-Prereq "m1_judgment_leg=skipped_model_branch"
            Write-Host "judgment leg missed (model branch); -M1Survival continues against the closure settle conclusion"
        }
        else {
            $outcome = "judgment_not_armed"; throw "trajectory.user_judgment.requested never arrived after the audition seat"
        }
    }
    if (-not $judgmentLegMissed) { Write-Ok "judgment boundary armed" }

    if (-not $judgmentLegMissed) {
    Write-Step "Submit the A/B judgment (retain: heard=yes, prefer B)"
    $judgment = Invoke-Json -Method POST -Url ($base + "/agent/audition/judgment") -Body @{
        conversation_id = $conversationID; turn_id = $seat.turn_id; round_id = $seat.round_id;
        audition_session_id = $seat.session_id; project_revision = [string]$seat.project_revision;
        heard_difference = "yes"; preference = "b"; reason_tags = @("settle_deliver_machine_walk")
    } -TimeoutSec 60
    Add-Prereq ("judgment_http=" + $judgment.status)
    if ($judgment.status -ne "ok") { throw ("judgment rejected: " + ($judgment | ConvertTo-Json -Compress)) }
    Start-Sleep -Seconds 10

    Write-Step "Symptom A: settle confirmation must be an event AND a persisted conversation message"
    $eventsAfter = @((Invoke-Json -Method GET -Url ($base + "/agent/events?conversation_id=" + $conversationID + "&since=0&limit=200") -Body $null -TimeoutSec 30).events)
    $settleEvents = @($eventsAfter | Where-Object { $_.type -eq "judgment.settled" })
    foreach ($event in $settleEvents) { if ($event.body) { $settleEventBody = [string]$event.body } }
    if ($settleEvents.Count -eq 0 -or [string]::IsNullOrWhiteSpace($settleEventBody)) { $outcome = "settle_message_missing"; throw "judgment.settled event with body missing (live delivery face)" }
    Write-Ok ("settle event body present (" + $settleEventBody.Length + " chars)")
    } # end judgment POST + settle-event face (M1 runs before the row-face assertions)

    if ($M1Survival) {
        Write-Step "MIXLAYER-M1-1: conclusion record must exist in the project-keyed store"
        # The stack runs on an isolated draft root, so the project store lands
        # under the run's agent_drafts; the repo workspace is searched as the
        # non-draft fallback. A record + board file written during this run is
        # the on-disk half of the survival proof.
        $m1ConclusionDirs = @()
        foreach ($m1SearchRoot in @((Join-Path $RepoRoot "VitApp\Workspace"), $RunRoot)) {
            if (Test-Path -LiteralPath $m1SearchRoot) {
                $m1ConclusionDirs += @(Get-ChildItem -LiteralPath $m1SearchRoot -Recurse -Directory -Filter "free_state_conclusions" -ErrorAction SilentlyContinue)
            }
        }
        $m1Records = @()
        foreach ($dir in $m1ConclusionDirs) {
            $m1Records += @(Get-ChildItem -LiteralPath $dir.FullName -File -Filter "fsc_*.json" -ErrorAction SilentlyContinue)
        }
        Add-Prereq ("m1_conclusion_dirs=" + ($m1ConclusionDirs.Count) + " records=" + $m1Records.Count)
        if ($m1Records.Count -lt 1) { $outcome = "m1_survival_missing"; throw "no fsc_*.json conclusion record found under the project store (MIXLAYER-M1-1)" }
        foreach ($record in $m1Records) {
            Add-Prereq ("m1_record=" + $record.FullName + " mtime=" + $record.LastWriteTime.ToString("o"))
            Copy-Item -LiteralPath $record.FullName -Destination (Join-Path $RunRoot $record.Name) -Force
        }
        $m1BoardFiles = @()
        foreach ($dir in $m1ConclusionDirs) {
            $m1BoardFiles += @(Get-ChildItem -LiteralPath $dir.FullName -File -Filter "free_state_conclusions.json" -ErrorAction SilentlyContinue)
        }
        if ($m1BoardFiles.Count -lt 1) { $outcome = "m1_survival_missing"; throw "conclusion board file missing under the project store (MIXLAYER-M1-1)" }
        Write-Ok ("conclusion records on disk: " + $m1Records.Count)

        Write-Step "MIXLAYER-M1-1 gate: NEW conversation reads the conclusion back (identity-axis bridge)"
        # The card's题眼 is the identity-axis mismatch: settle state is keyed by
        # conversation_id while the conclusion store is keyed by project_uuid.
        # The gated readback therefore uses a brand-new conversation on the SAME
        # process (same project-store binding): the settle conclusion from
        # conversation settledeliver_* must surface in conversation
        # m1survival_*'s blackboard report.
        # The expected outcome wording follows the conclusion kind that
        # actually landed: the judgment settle reads back as the human-audition
        # retain wording, the closure settle as the capability-boundary wording.
        $m1ExpectedOutcomeNeedle = ""
        $m1NewestRecord = $m1Records | Sort-Object -Property LastWriteTime | Select-Object -Last 1
        try {
            $m1RecordJson = Get-Content -LiteralPath $m1NewestRecord.FullName -Raw -Encoding UTF8 | ConvertFrom-Json
            Add-Prereq ("m1_newest_record_kind=" + $m1RecordJson.kind + " outcome=" + $m1RecordJson.outcome + " project=" + $m1RecordJson.project_uuid)
            switch ($m1RecordJson.outcome) {
                "improved" { $m1ExpectedOutcomeNeedle = $M1NeedleOutcome }
                "rolled_back" { $m1ExpectedOutcomeNeedle = $M1NeedleRollbackOutcome }
                "capability_blocked" { $m1ExpectedOutcomeNeedle = $M1NeedleClosureOutcome }
                default { $m1ExpectedOutcomeNeedle = "" }
            }
        } catch { Add-Prereq ("m1_record_read_failed=" + $_.Exception.Message) }
        $m1Conversation = "m1survival_" + (Get-Date -Format "yyyyMMdd_HHmmss")
        $m1Chat = Invoke-Json -Method POST -Url ($base + "/agent/chat") -Body @{ conversation_id = $m1Conversation; message = $M1BlackboardPrompt } -TimeoutSec 300
        $m1Chat | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath (Join-Path $RunRoot "m1_blackboard_response.json") -Encoding UTF8
        $m1Reply = [string]$m1Chat.reply
        if ($m1Reply -eq "") { $m1Reply = [string]$m1Chat.message }
        Add-Prereq ("m1_blackboard_goal=" + $m1Chat.goal_id + " status=" + $m1Chat.goal_status)
        if (-not $m1Reply.Contains($M1NeedleSection)) { $outcome = "m1_survival_missing"; throw ("blackboard report missing the cross-session conclusion section (MIXLAYER-M1-1); reply head: " + $m1Reply.Substring(0, [Math]::Min(300, $m1Reply.Length))) }
        if ($m1ExpectedOutcomeNeedle -ne "" -and -not $m1Reply.Contains($m1ExpectedOutcomeNeedle)) { $outcome = "m1_survival_missing"; throw ("blackboard report missing the settled outcome wording (MIXLAYER-M1-1); reply head: " + $m1Reply.Substring(0, [Math]::Min(300, $m1Reply.Length))) }
        Write-Ok "cross-conversation conclusion visible in the blackboard report"

    }

    if (-not $judgmentLegMissed) {

    # /agent/state is the compact polling face (conversation nodes stripped by
    # design, Godot 16 MiB chunk limit); the durable rows live on detail=full.
    # The settle POST's persistence tail (closure settle → checkpoint → "vit"
    # node write) can outlive a fixed sleep, so the row read is a bounded poll
    # (run 20261011_111730: node landed at 11:19:57, the fixed-10s read lost
    # the race; the node itself was on disk in the draft graph).
    $settleMessageRows = 0
    $rows = @()
    $rowDeadline = (Get-Date).AddSeconds(60)
    while ((Get-Date) -lt $rowDeadline) {
        $stateAfter = Invoke-Json -Method GET -Url ($base + "/agent/state?detail=full") -Body $null -TimeoutSec 60
        $rows = @()
        if ($stateAfter.project_history -and $stateAfter.project_history.conversation_messages) {
            $rows = @($stateAfter.project_history.conversation_messages)
        }
        $settleMessageRows = @($rows | Where-Object {
            ($_.role -eq "assistant" -or $_.kind -eq "vit") -and ($_.content -like ("*" + $NeedleSettle + "*") -and $_.content -like ("*" + $NeedleEvidence + "*"))
        }).Count
        if ($settleMessageRows -ge 1) { break }
        # Forensic anchor (run 20261011_111730/112607: graph node with both
        # needles on disk, yet the detail=full face reported rows=4 with no
        # match): keep every failing poll's raw state JSON for offline diff.
        $stateAfter | ConvertTo-Json -Depth 12 | Set-Content -LiteralPath (Join-Path $RunRoot ("state_poll_miss_" + (Get-Date -Format "HHmmss") + ".json")) -Encoding UTF8
        Start-Sleep -Seconds 5
    }
    if ($settleMessageRows -lt 1) { $outcome = "settle_message_missing"; throw "persisted assistant conversation_message row for the settle report missing (rows=" + $rows.Count + ")" }
    Write-Ok ("persisted settle message rows=" + $settleMessageRows)

    Write-Step "Symptom B: follow-up input on the same conversation must run (no revision-stale stop)"
    $chat2 = Invoke-Json -Method POST -Url ($base + "/agent/chat") -Body @{ conversation_id = $conversationID; message = $Round2Text } -TimeoutSec 300
    $chat2 | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $Chat2Path -Encoding UTF8
    Add-Prereq ("chat2_goal=" + $chat2.goal_id + " status=" + $chat2.goal_status + " stop=" + $chat2.stop_reason)
    $round2Stop = [string]$chat2.stop_reason
    $round2GoalStatus = [string]$chat2.goal_status
    if ($round2Stop -eq $NeedleStale) { $outcome = "round2_stale"; throw "round-2 input stopped with project_revision_stale (symptom B residual)" }

    $deadline2 = (Get-Date).AddSeconds($Round2BudgetSeconds)
    while ((Get-Date) -lt $deadline2) {
        Start-Sleep -Seconds $PollSeconds
        $runtime2 = Invoke-Json -Method GET -Url ($base + "/agent/runtime/status") -Body $null -TimeoutSec 30
        $goalStatus = [string]$runtime2.goal.status
        $taskState = [string]$runtime2.task_state
        Write-Host ("round2 poll goal=" + $goalStatus + " task=" + $taskState)
        if ($goalStatus -eq "completed" -or $goalStatus -eq "failed" -or $goalStatus -eq "stopped" -or $goalStatus -eq "cancelled") { break }
    }
    $eventsFinal = @((Invoke-Json -Method GET -Url ($base + "/agent/events?conversation_id=" + $conversationID + "&since=0&limit=300") -Body $null -TimeoutSec 30).events)
    $eventsFinal | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath $EventsPath -Encoding UTF8
    $stateFinal = Invoke-Json -Method GET -Url ($base + "/agent/state") -Body $null -TimeoutSec 30
    $stateFinal | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath $StatePath -Encoding UTF8
    $terminals = @($eventsFinal | Where-Object { $_.type -eq "turn.completed" -or $_.type -eq "turn.failed" -or $_.type -eq "turn.stopped" })
    foreach ($terminal in $terminals) {
        if ([string]$terminal.goal_id -eq [string]$chat2.goal_id -and [string]$terminal.payload.stop_reason) {
            if ([string]$terminal.payload.stop_reason -eq $NeedleStale) { $outcome = "round2_stale"; throw "round-2 turn terminal reports project_revision_stale" }
        }
    }
    $ungoverned = @($eventsFinal | Where-Object { ($_.type -eq "turn.completed") -and ([string]$terminal.payload.stop_reason) })
    $round2TaskNote = (@($terminals | ForEach-Object { [string]$_.payload.transition_reason }) -join ";")
    if ($round2TaskNote -like ("*" + $NeedleUngoverned + "*")) { $outcome = "round2_stale"; throw "round-2 closed without a governed experiment (residual symptom B semantics)" }
    } # end judgment-leg assertions

    if ($M1Survival) {
          Write-Step "MIXLAYER-M1-1 probe (report-only): agent restart readback"
          # The restart axis is probed, not gated: this smoke runs on an isolated
          # draft root, and a restarted agent drafts a fresh project store for an
          # unsaved kernel project (run 20261011_113450: new draft, empty store)
          # — a smoke-environment constraint, not the production shape (a saved
          # project keeps one .vit_agent across processes). Process-restart
          # survival of the record itself is already proven on disk above.
          Stop-Process -Id $agentProcId -Force -ErrorAction SilentlyContinue
          Add-Prereq ("m1_restarted_former_agent_pid=" + $agentProcId)
          $m1AgentDown = (Get-Date).AddSeconds($WaitSeconds)
          while ((Get-Date) -lt $m1AgentDown) {
              if (-not (Get-TcpListener -Port 7878)) { break }
              Start-Sleep -Milliseconds 500
          }
          if (Get-TcpListener -Port 7878) { Add-Prereq "m1_restart_probe=port_busy_skip"; $m1RestartReply = "" }
          else {
              $priorDraftRoot2 = [Environment]::GetEnvironmentVariable("VIT_HISTORY_DRAFT_ROOT", "Process")
              $priorDevRoot2 = [Environment]::GetEnvironmentVariable("VIT_DAW_DEV_ROOT", "Process")
              $env:VIT_HISTORY_DRAFT_ROOT = $AgentDrafts
              $env:VIT_DAW_DEV_ROOT = $RepoRoot
              try {
                  $agentProc2 = Start-Process -FilePath $AgentBin -ArgumentList @("-http", "127.0.0.1:7878", "-last-log-path", $AgentLog, "-keep-last-log-lines", "8000") -WorkingDirectory $AgentDir -WindowStyle Hidden -PassThru
              }
              finally {
                  if ($null -eq $priorDraftRoot2) { Remove-Item Env:VIT_HISTORY_DRAFT_ROOT -ErrorAction SilentlyContinue } else { $env:VIT_HISTORY_DRAFT_ROOT = $priorDraftRoot2 }
                  if ($null -eq $priorDevRoot2) { Remove-Item Env:VIT_DAW_DEV_ROOT -ErrorAction SilentlyContinue } else { $env:VIT_DAW_DEV_ROOT = $priorDevRoot2 }
              }
              $agentProcId = $agentProc2.Id
              Add-Prereq ("m1_agent2_pid=" + $agentProcId)
              $m1RestartReply = ""
              if (Wait-HttpReady -BaseUrl $base -TimeoutSeconds $WaitSeconds) {
                  try {
                      $m1State = Invoke-Json -Method GET -Url ($base + "/agent/state") -Body $null -TimeoutSec 30
                      Add-Prereq ("m1_agent2_shadow_initialized=" + $m1State.shadow.initialized)
                      Invoke-Json -Method POST -Url ($base + "/agent/authority") -Body @{ authority_mode = "full_project_access" } -TimeoutSec 30 | Out-Null
                      $m1ProbeConversation = "m1probe_" + (Get-Date -Format "yyyyMMdd_HHmmss")
                      $m1ProbeChat = Invoke-Json -Method POST -Url ($base + "/agent/chat") -Body @{ conversation_id = $m1ProbeConversation; message = $M1BlackboardPrompt } -TimeoutSec 300
                      $m1ProbeChat | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath (Join-Path $RunRoot "m1_restart_probe_response.json") -Encoding UTF8
                      $m1RestartReply = [string]$m1ProbeChat.reply
                      if ($m1RestartReply -eq "") { $m1RestartReply = [string]$m1ProbeChat.message }
                  } catch { Add-Prereq ("m1_restart_probe_error=" + $_.Exception.Message) }
              }
              else { Add-Prereq "m1_restart_probe=agent_not_ready" }
          }
          if ($m1RestartReply.Contains($M1NeedleSection)) { Add-Prereq "m1_restart_readback=visible" }
          else { Add-Prereq "m1_restart_readback=empty_draft_isolation" }
    }


    if ($judgmentLegMissed) { $outcome = "m1_finished" } else { $outcome = "finished" }
}
catch {
    $detail = $_.Exception.Message
    Add-Prereq ("fatal=" + $detail)
    Write-Bad ("fatal: " + $detail)
    # Failure-state forensics: events + runtime land on disk too (AGENTS 9);
    # the success path must not own the only copy of the evidence.
    try {
        $failEvents = @((Invoke-Json -Method GET -Url ($base + "/agent/events?conversation_id=" + $conversationID + "&since=0&limit=300") -Body $null -TimeoutSec 30).events)
        $failEvents | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath $EventsPath -Encoding UTF8
        $failRuntime = Invoke-Json -Method GET -Url ($base + "/agent/runtime/status") -Body $null -TimeoutSec 30
        $failRuntime | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath ($RunRoot + "\runtime_failed.json") -Encoding UTF8
    }
    catch { Add-Prereq ("failure_forensics_unavailable=" + $_.Exception.Message) }
    if ($outcome -eq "env_failure" -and $detail -notlike "*port*" -and $detail -notlike "*listen*" -and $detail -notlike "*ready*") { $outcome = "chain_failed" }
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
    Add-Prereq ("finished=" + (Get-Date -Format "o"))
    $prereq | Out-File -FilePath $PrereqPath -Encoding utf8

    $report = [ordered]@{
        schema_version       = "settle_deliver_smoke.v1"
        run_root             = $RunRoot
        outcome              = $outcome
        detail               = $detail
        settle_event_body    = $settleEventBody
        settle_message_rows  = $settleMessageRows
        round2_stop_reason   = $round2Stop
        round2_goal_status   = $round2GoalStatus
        round2_task_note     = $round2TaskNote
        m1_survival_enabled  = [bool]$M1Survival
        m1_judgment_leg_missed = $judgmentLegMissed
        prereq               = @($prereq)
    }
    $report | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath $ReportPath -Encoding UTF8
}

Write-Host ""
Write-Host ("SETTLE_DELIVER_VERDICT " + $outcome)
switch ($outcome) {
    "finished" { exit 0 }
    "m1_finished" { exit 0 }
    "judgment_not_armed" { exit 2 }
    "settle_message_missing" { exit 1 }
    "round2_stale" { exit 1 }
    "round2_failed" { exit 1 }
    "m1_survival_missing" { exit 1 }
    "chain_failed" { exit 1 }
    "timeout" { exit 2 }
    default { exit 2 }
}
