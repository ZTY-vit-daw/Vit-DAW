<#
REPLY-GEN-TOOLGATE-1 real-stack smoke (machine walk): A/B 判定 → 结算 → 同会话
观察问句 → 收到正常观察回复。钉的是 2026-10-03 19:37 现场缺陷（error 节点
n_20261003T113752「前面的工具操作已完成，但最终回复生成失败：未知或不允许的
工具：」冒号后为空）修复后的全链。人类手测仍走 Godot 入口（AGENTS.md §5）。
脚本栈模式与 settle_deliver_smoke.ps1 同族（判定/结算链断言复用其形态）。

Probabilistic-run protocol (AGENTS.md §8, declared before execution):
  runs        : 1 full-chain attempt; a second attempt only for a classified
                environment interruption (port/kernel-start failure), never a
                blind rerun of a functional failure.
  success     : exit 0 — judgment POST accepted; judgment.settled event with
                body AND a persisted assistant conversation_message row carry
                the settle report; the follow-up OBSERVATION question's turn
                reaches terminal completed with a non-empty visible reply that
                contains neither「未知或不允许的工具」nor「最终回复生成失败」.
  fail classes: judgment_not_armed (pre-chain channel, out of card scope),
                settle_message_missing (settle delivery face),
                round2_toolgate_error (the live defect class: admission error
                text or the reply-generation-failure wrapper resurfacing),
                round2_no_reply (terminal completed but no visible reply row),
                round2_failed (honest other stop), timeout, env_failure.
  stop-loss   : two consecutive same-shape failures → stop, keep artifacts,
                escalate to the decision side. No assertion weakening.

Usage:
  powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\reply_gen_toolgate_smoke.ps1
  powershell ... -SkipBuild -KernelDwellSeconds 30 -ChainBudgetSeconds 600
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
    [int]$Round2BudgetSeconds = 420
)

$ErrorActionPreference = "Stop"

function From-B64 {
    param([string]$Value)
    return [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String($Value))
}

# ASCII-safe literals (Windows PowerShell reads BOM-less .ps1 as ANSI).
$PromptText = From-B64 "5a+55L2O6Z+z6L2o5YGa5LiA5qyh5re36Z+z5pS56L+b5a6e6aqM77yM5pS55a6M6K6p5oiRIEEvQiDor5XlkKzlr7nmr5TkuIDkuIs="
$Round2Text = From-B64 "5YaN5biu5oiR55yL55yLYmFzc+i9qOmBk+eahOS9jumikeacieayoeacieS7gOS5iOmXrumimA=="
$NeedleSettle = From-B64 "QS9CIOW+p+i55YO55bey6LW05Lus5YWl5o2i5Y6/5LqG5Y2H"
$NeedleEvidence = From-B64 "5Yik5a6a6K+B5o2u"
$NeedleBadTool = From-B64 "5pyq55+l5oiW5LiN5YWB6K6455qE5bel5YW3"
$NeedleBadReply = From-B64 "5pyA57uI5Zue5aSN55Sf5oiQ5aSx6LSl"

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
    $RunRoot = Join-Path $RepoRoot ("coord\runs\REPLY-GEN-TOOLGATE-SMOKE-" + (Get-Date -Format "yyyyMMdd_HHmmss"))
}
New-Item -ItemType Directory -Force -Path $RunRoot | Out-Null
$AgentDrafts = Join-Path $RunRoot "agent_drafts"
New-Item -ItemType Directory -Force -Path $AgentDrafts | Out-Null
$AgentBin = Join-Path $RunRoot "bin\VitAgent.replygentoolgate.exe"
$AgentLog = Join-Path $RunRoot "agent_last.log"
$AgentDir = Join-Path $RepoRoot "agent"
$PrereqPath = Join-Path $RunRoot "prereq.txt"
$ReportPath = Join-Path $RunRoot "reply_gen_toolgate_report.json"
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
$round2Reply = ""
$round2ReplyRows = 0

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
        Write-Step "Build VitAgent (isolated binary in run dir; must contain the REPLY-GEN-TOOLGATE-1 fix)"
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

    $conversationID = "replygentoolgate_" + (Get-Date -Format "yyyyMMdd_HHmmss")
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
        # The LAST audition.ready can be kernel telemetry without turn fields;
        # pick the last row that actually carries the complete judgment identity.
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
    $deadlineJR = (Get-Date).AddSeconds(240)
    $requested = $false
    while ((Get-Date) -lt $deadlineJR) {
        $jrEvents = @((Invoke-Json -Method GET -Url ($base + "/agent/events?conversation_id=" + $conversationID + "&since=0&limit=200") -Body $null -TimeoutSec 30).events)
        $requestedRows = @($jrEvents | Where-Object { $_.type -eq "trajectory.user_judgment.requested" })
        if ($requestedRows.Count -gt 0) { $requested = $true; break }
        Start-Sleep -Seconds 5
    }
    if (-not $requested) { $outcome = "judgment_not_armed"; throw "trajectory.user_judgment.requested never arrived after the audition seat" }
    Write-Ok "judgment boundary armed"

    Write-Step "Submit the A/B judgment (retain: heard=yes, prefer B)"
    $judgment = Invoke-Json -Method POST -Url ($base + "/agent/audition/judgment") -Body @{
        conversation_id = $conversationID; turn_id = $seat.turn_id; round_id = $seat.round_id;
        audition_session_id = $seat.session_id; project_revision = [string]$seat.project_revision;
        heard_difference = "yes"; preference = "b"; reason_tags = @("reply_gen_toolgate_machine_walk")
    } -TimeoutSec 60
    Add-Prereq ("judgment_http=" + $judgment.status)
    if ($judgment.status -ne "ok") { throw ("judgment rejected: " + ($judgment | ConvertTo-Json -Compress)) }
    Start-Sleep -Seconds 10

    Write-Step "Settle confirmation must be an event AND a persisted conversation-graph node"
    $eventsAfter = @((Invoke-Json -Method GET -Url ($base + "/agent/events?conversation_id=" + $conversationID + "&since=0&limit=200") -Body $null -TimeoutSec 30).events)
    $settleEvents = @($eventsAfter | Where-Object { $_.type -eq "judgment.settled" })
    foreach ($event in $settleEvents) { if ($event.body) { $settleEventBody = [string]$event.body } }
    if ($settleEvents.Count -eq 0 -or [string]::IsNullOrWhiteSpace($settleEventBody)) { $outcome = "settle_message_missing"; throw "judgment.settled event with body missing" }
    Write-Ok ("settle event body present (" + $settleEventBody.Length + " chars)")

    # Persistence face = the isolated draft's conversation graph (the SAME face
    # the live defect corrupted and the face SETTLE-DELIVER's accepted evidence
    # used; /agent/state's conversation rows do not carry async settle rows).
    $graphPath = Get-ChildItem -LiteralPath $AgentDrafts -Recurse -Filter "conversation_graph.json" -ErrorAction SilentlyContinue |
        Sort-Object LastWriteTime -Descending | Select-Object -First 1
    if ($null -eq $graphPath) { $outcome = "settle_message_missing"; throw "no conversation_graph.json under the isolated draft root" }
    Add-Prereq ("graph=" + $graphPath.FullName)
    $graph = Get-Content -LiteralPath $graphPath.FullName -Raw -Encoding UTF8 | ConvertFrom-Json
    $graphNodes = @($graph.nodes)
    $settleRows = @($graphNodes | Where-Object {
        ($_.logical_message_id -like "judgment_settle:*") -and ($_.text -like ("*" + $NeedleSettle + "*")) -and ($_.text -like ("*" + $NeedleEvidence + "*"))
    })
    $settleMessageRows = $settleRows.Count
    if ($settleMessageRows -lt 1) { $outcome = "settle_message_missing"; throw "conversation-graph node for the settle report missing (nodes=" + $graphNodes.Count + ")" }
    Write-Ok ("persisted settle graph node = " + $settleRows[0].id)

    Write-Step "Live-defect face: observation question on the SAME conversation must reach a normal reply"
    $round2AskedAt = Get-Date
    $chat2 = Invoke-Json -Method POST -Url ($base + "/agent/chat") -Body @{ conversation_id = $conversationID; message = $Round2Text } -TimeoutSec 300
    $chat2 | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $Chat2Path -Encoding UTF8
    Add-Prereq ("chat2_goal=" + $chat2.goal_id + " status=" + $chat2.goal_status + " stop=" + $chat2.stop_reason)
    $round2Stop = [string]$chat2.stop_reason
    $round2GoalStatus = [string]$chat2.goal_status
    if ($round2Stop -eq "project_revision_stale") { $outcome = "round2_failed"; throw "round-2 input stopped with project_revision_stale" }
    if ([string]$chat2.reply) { $round2Reply = [string]$chat2.reply }

    $deadline2 = (Get-Date).AddSeconds($Round2BudgetSeconds)
    $goalStatus2 = ""
    while ((Get-Date) -lt $deadline2) {
        Start-Sleep -Seconds $PollSeconds
        $runtime2 = Invoke-Json -Method GET -Url ($base + "/agent/runtime/status") -Body $null -TimeoutSec 30
        $goalStatus2 = [string]$runtime2.goal.status
        Write-Host ("round2 poll goal=" + $goalStatus2 + " task=" + $runtime2.task_state)
        if ($goalStatus2 -eq "completed" -or $goalStatus2 -eq "failed" -or $goalStatus2 -eq "stopped" -or $goalStatus2 -eq "cancelled") { break }
    }
    if ($goalStatus2 -eq "") { $goalStatus2 = $round2GoalStatus }
    if ($goalStatus2 -ne "completed") { $outcome = "round2_failed"; throw ("round-2 goal terminal status=" + $goalStatus2 + " stop=" + $round2Stop) }

    $eventsFinal = @((Invoke-Json -Method GET -Url ($base + "/agent/events?conversation_id=" + $conversationID + "&since=0&limit=300") -Body $null -TimeoutSec 30).events)
    $eventsFinal | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath $EventsPath -Encoding UTF8
    $stateFinal = Invoke-Json -Method GET -Url ($base + "/agent/state") -Body $null -TimeoutSec 30
    $stateFinal | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath $StatePath -Encoding UTF8

    # The visible reply face = the draft conversation graph (the live defect
    # wrote message_kind=error node n_20261003T113752 here): after the round-2
    # ask there must be a non-empty assistant node, no error node, and neither
    # toolgate needle may appear in any post-ask node.
    $graph2 = Get-Content -LiteralPath $graphPath.FullName -Raw -Encoding UTF8 | ConvertFrom-Json
    $graph2Nodes = @($graph2.nodes)
    $askCutoff = $round2AskedAt.ToUniversalTime()
    $postAsk = @($graph2Nodes | Where-Object {
        $_.created_at -and ([datetime]$_.created_at).ToUniversalTime() -ge $askCutoff
    })
    $postAskReplies = @($postAsk | Where-Object { $_.kind -ne "ask" -and $_.message_kind -ne "error" -and $_.text })
    $postAskErrors = @($postAsk | Where-Object { $_.message_kind -eq "error" })
    $round2ReplyRows = $postAskReplies.Count
    $lastReply = ""
    if ($postAskReplies.Count -gt 0) { $lastReply = [string]$postAskReplies[$postAskReplies.Count - 1].text }
    if ([string]::IsNullOrWhiteSpace($lastReply) -and -not [string]::IsNullOrWhiteSpace($round2Reply)) { $lastReply = $round2Reply }

    foreach ($node in $postAsk) {
        $nodeText = [string]$node.text
        if ($nodeText -like ("*" + $NeedleBadTool + "*") -or $nodeText -like ("*" + $NeedleBadReply + "*")) {
            $outcome = "round2_toolgate_error"; throw ("post-ask graph node " + $node.id + " carries the toolgate failure face: " + $nodeText)
        }
    }
    if ($postAskErrors.Count -gt 0) {
        $outcome = "round2_toolgate_error"; throw ("post-ask graph carries an error node: " + $postAskErrors[0].id + " " + [string]$postAskErrors[0].text)
    }
    if ([string]::IsNullOrWhiteSpace($lastReply)) { $outcome = "round2_no_reply"; throw "round-2 completed but no visible assistant node landed after the ask" }
    if (-not [string]::IsNullOrWhiteSpace($round2Reply) -and ($round2Reply -like ("*" + $NeedleBadTool + "*") -or $round2Reply -like ("*" + $NeedleBadReply + "*"))) {
        $outcome = "round2_toolgate_error"; throw "round-2 HTTP reply carries the toolgate failure face"
    }
    Write-Ok ("round-2 normal reply delivered (" + $lastReply.Length + " chars)")
    Add-Prereq ("round2_reply_head=" + $lastReply.Substring(0, [Math]::Min(80, $lastReply.Length)))

    $outcome = "finished"
}
catch {
    $detail = $_.Exception.Message
    Add-Prereq ("fatal=" + $detail)
    Write-Bad ("fatal: " + $detail)
    # Failure-state forensics: events + runtime land on disk too (AGENTS 9).
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
        schema_version       = "reply_gen_toolgate_smoke.v1"
        run_root             = $RunRoot
        outcome              = $outcome
        detail               = $detail
        settle_event_body    = $settleEventBody
        settle_message_rows  = $settleMessageRows
        round2_stop_reason   = $round2Stop
        round2_goal_status   = $round2GoalStatus
        round2_reply_rows    = $round2ReplyRows
        round2_reply_head    = $round2Reply
        prereq               = @($prereq)
    }
    $report | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath $ReportPath -Encoding UTF8
}

Write-Host ""
Write-Host ("REPLY_GEN_TOOLGATE_VERDICT " + $outcome)
switch ($outcome) {
    "finished" { exit 0 }
    "judgment_not_armed" { exit 2 }
    "settle_message_missing" { exit 1 }
    "round2_toolgate_error" { exit 1 }
    "round2_no_reply" { exit 1 }
    "round2_failed" { exit 1 }
    "chain_failed" { exit 1 }
    "timeout" { exit 2 }
    default { exit 2 }
}
