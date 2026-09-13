<#
CONFIRM-CHAT-1 real-stack smoke: the natural affirmative answer of a pending
confirmation ("需要" answering "…需要我直接做吗？").

Defect under test (goal_20c9933c, project 912.vit, 2026-09-12 22:4x): the user
answered the assistant's own confirmation question with the verb the question
used, and the turn fell out of the confirmation surface entirely -- it reached
semantic entry, was classified as an unclassifiable new request, and the model's
English diagnostic string was pasted straight into the user-facing reply.

This script starts the real stack (kernel + agent, isolated draft root, same
shape as scripts/cont_stall_repro_smoke.ps1 / run_b6_mixtick_terminal_smoke.ps1),
drives a chat turn to an answerable boundary, and answers it with the natural
affirmative, asserting:

  P1  the answer's user-facing reply carries no English diagnostic from the
      semantic-entry fallback family (the defect's user-visible face);
  P2  the turn is not classified as an invalid/unclassifiable request
      (stop_reason semantic_entry_invalid is a failure);
  P3  when a pending mix-tick confirmation was in force at the boundary, the
      answer is consumed as confirmation execution (the accept branch logs
      "[mix.tick.pending] explicit confirmation routed") and is not held as an
      ambiguous answer.

Verdicts:
  pass                  P1..P3 hold                                     -> exit 0
  assertion_failure     any of P1..P3 fails                             -> exit 1
  no_boundary           no answerable confirmation boundary was reached
                        (model variance, not a functional verdict)       -> exit 2
  env_failure           stack/port/startup problem                      -> exit 2

Usage:
  powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\run_confirm_chat_1_natural_affirmative_smoke.ps1
#>
[CmdletBinding()]
param(
    [string]$RepoRoot = "",
    [string]$RunRoot = "",
    [switch]$SkipBuild,
    [string]$AgentBinary = "",
    [string]$KernelExe = "",
    [int]$WaitSeconds = 30,
    [int]$KernelDwellSeconds = 20,
    [int]$BoundaryBudgetSeconds = 420,
    [int]$MaxProbes = 2,
    [int]$PollSeconds = 5,
    [int]$LogFlushSeconds = 20,
    [int]$MaxDriveRounds = 3
)

$ErrorActionPreference = "Stop"

function From-B64 {
    param([string]$Value)
    return [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String($Value))
}

# ASCII-safe literals: Windows PowerShell reads BOM-less .ps1 as ANSI, so every
# non-empty non-ASCII string rides base64 (same contract as cont_stall_repro_smoke).
$PromptShort = From-B64 "5oqK5L2O6Z+z6L2oL2Jhc3Mg5o+QIDFkQg=="
$PromptLong = From-B64 "6K+35LuO5re36Z+z6KeS5bqm5qOA5p+l5b2T5YmN5bel56iL5pyJ5LuA5LmI5Y+v5Lul5pS55ZaE55qE5Zyw5pa577yM5bm25YGa5LiA5Liq5bCP5q2l5bCd6K+V"
$AnswerText = From-B64 "6ZyA6KaB"
$NudgeText = From-B64 "57un57ut"
# The explicit phrase the plan-confirmation surface already accepts (used only to
# drive the chain on to the mix-tick card; the assertion always uses the natural
# affirmative).
$PlanApprovalText = From-B64 "5Y+v5Lul5omn6KGM"

# English diagnostic family of the semantic-entry fallback (the defect's
# user-visible face). Any of these in a user-facing reply is a failure.
$DiagnosticNeedles = @(
    "underspecified",
    "The user request",
    "could not classify this request safely",
    "Please clarify whether you want discussion"
)
# NOTE: log needles are matched with [string]::Contains, never -like: PowerShell's
# -like treats "[" "]" as character-class wildcards, so "*[mix.tick.pending]*" can
# never match the literal line (rounds 1-6 all reported routed_lines=0 for exactly
# that reason while the line sat in the saved window file).
$RoutedLogNeedle = "[mix.tick.pending] explicit confirmation routed"
$SemanticEntryLogNeedle = "[semantic.entry] unresolved fallback"

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

function Test-HasCJK {
    param([string]$Text)
    if ([string]::IsNullOrEmpty($Text)) { return $false }
    return ($Text -match "[\u4e00-\u9fff]")
}

function Get-DiagnosticHits {
    param([string]$Text)
    $hits = New-Object System.Collections.Generic.List[string]
    foreach ($needle in $DiagnosticNeedles) {
        if ($Text.Contains($needle)) { $hits.Add($needle) }
    }
    return $hits.ToArray()
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

$RepoRoot = Resolve-RepoRoot -Explicit $RepoRoot
$AgentDir = Join-Path $RepoRoot "agent"
if ([string]::IsNullOrWhiteSpace($RunRoot)) {
    $RunRoot = Join-Path $RepoRoot ("artifacts\confirm_chat_1\" + (Get-Date -Format "yyyyMMdd_HHmmss"))
}
if (Test-Path -LiteralPath $RunRoot) { throw ("run root already exists; each run needs a fresh artifact directory: " + $RunRoot) }
New-Item -ItemType Directory -Force -Path $RunRoot | Out-Null
$AgentDrafts = Join-Path $RunRoot "agent_drafts"
New-Item -ItemType Directory -Force -Path $AgentDrafts | Out-Null
$AgentLog = Join-Path $RunRoot "agent_last.log"
$PrereqPath = Join-Path $RunRoot "prereq.txt"
$ReportPath = Join-Path $RunRoot "confirm_chat_1_report.json"
$EventsPath = Join-Path $RunRoot "events.json"
$RuntimePath = Join-Path $RunRoot "runtime_status.json"
if ([string]::IsNullOrWhiteSpace($AgentBinary)) { $AgentBinary = Join-Path $RunRoot "bin\VitAgent.confirmchat.exe" }
if ([string]::IsNullOrWhiteSpace($KernelExe)) { $KernelExe = Join-Path $RepoRoot "VitApp\build\VitApp_artefacts\Release\VitApp.exe" }
$RepoDefaultProject = Join-Path $RepoRoot "VitApp\Workspace\default_project.xml"
$UserProjectDir = "D:\Godot\project\vit-daw-frontend\912.vit"

$prereq = New-Object System.Collections.Generic.List[string]
function Add-Prereq { param([string]$Line) $script:prereq.Add($Line); Write-Host $Line }

$kernelProcId = 0
$agentProcId = 0
$verdict = "env_failure"
$detail = ""
$boundaryBranch = "none"
$boundaryResponse = $null
$answerResponse = $null
$lastRuntime = $null
$lastEvents = @()
$routedLines = @()
$semanticEntryLines = @()
$diagnosticHits = @()

try {
    Write-Step ("CONFIRM-CHAT-1 run root: " + $RunRoot)
    Add-Prereq ("run_root=" + $RunRoot)
    Add-Prereq ("started=" + (Get-Date -Format o))
    Add-Prereq ("head=" + (& git -C $RepoRoot rev-parse HEAD))
    Add-Prereq ("git_status=" + ((& git -C $RepoRoot status --short) -join " ; "))

    Write-Step "Prereq: ports 7878/5555/5556 must be free"
    foreach ($port in 7878, 5555, 5556) {
        if (Get-TcpListener -Port $port) { throw ("port " + $port + " is already listening; aborting before any start (stack ownership rule)") }
        Add-Prereq ("port_free=" + $port)
    }

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
    $kernelItem = Get-Item -LiteralPath $KernelExe
    Add-Prereq ("kernel_binary=" + $KernelExe + " size=" + $kernelItem.Length + " mtime=" + $kernelItem.LastWriteTime.ToString("o"))
    $projectHashBefore = (Get-FileHash -Algorithm SHA256 -LiteralPath $RepoDefaultProject).Hash
    Add-Prereq ("repo_default_project_sha256_before=" + $projectHashBefore)
    $userProjectBefore = Get-ProjectFingerprint -Path $UserProjectDir
    Add-Prereq ("user_project_fingerprint_before=" + $userProjectBefore)
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
        $agentProc = Start-Process -FilePath $AgentBinary -ArgumentList @("-http", "127.0.0.1:7878", "-last-log-path", $AgentLog, "-keep-last-log-lines", "8000") -WorkingDirectory $AgentDir -WindowStyle Hidden -PassThru
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
    Add-Prereq ("shadow_initialized=" + $state.shadow.initialized + " track_count=" + $state.shadow.track_count)
    if ($state.shadow.initialized -ne $true) { throw "shadow not initialized after stack start" }

    if ($KernelDwellSeconds -gt 0) {
        Write-Step ("Kernel warm-up dwell " + $KernelDwellSeconds + "s")
        Start-Sleep -Seconds $KernelDwellSeconds
    }

    $authority = Invoke-Json -Method GET -Url ($base + "/agent/authority") -Body $null -TimeoutSec 30
    Add-Prereq ("authority_mode=" + $authority.authority_mode + " (the confirmation surface requires the default manual mode)")

    $conversationID = "confirmchat_" + (Get-Date -Format "yyyyMMdd_HHmmss")
    Write-Step ("Drive to an answerable confirmation boundary on " + $conversationID)
    $probes = New-Object System.Collections.Generic.List[object]
    $boundaryFound = $false
    $probeIndex = 0
    $budgetDeadline = (Get-Date).AddSeconds($BoundaryBudgetSeconds)
    while ($probeIndex -lt $MaxProbes -and -not $boundaryFound -and (Get-Date) -lt $budgetDeadline) {
        $message = if ($probeIndex -eq 0) { $PromptShort } else { $PromptLong }
        $probeStarted = Get-Date
        $response = Invoke-Json -Method POST -Url ($base + "/agent/chat") -Body @{ conversation_id = $conversationID; message = $message } -TimeoutSec 300
        $probeEnded = Get-Date
        $response | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath (Join-Path $RunRoot ("probe_" + $probeIndex + "_response.json")) -Encoding UTF8
        $status = [string]$response.goal_status
        $stop = [string]$response.stop_reason
        $cardKinds = @($response.interaction_requests | ForEach-Object { [string]$_.kind })
        Add-Prereq ("probe_" + $probeIndex + "_status=" + $status + " stop=" + $stop + " cards=" + ($cardKinds -join ",") + " ms=" + [int](($probeEnded - $probeStarted).TotalMilliseconds))
        Write-Ok ("probe " + $probeIndex + ": goal_status=" + $status + " stop=" + $stop + " cards=" + ($cardKinds -join ","))
        $probes.Add([ordered]@{
            index = $probeIndex; status = $status; stop_reason = $stop; card_kinds = $cardKinds
            reply = [string]$response.reply; started_at = $probeStarted.ToString("o"); ended_at = $probeEnded.ToString("o")
        })
        if ($status -eq "waiting_confirmation" -or $status -eq "waiting_clarification") {
            $boundaryFound = $true
            $boundaryResponse = $response
        }
        $probeIndex++
    }

    if (-not $boundaryFound) {
        $verdict = "no_boundary"
        $detail = "no answerable confirmation/clarification boundary was reached in " + $probes.Count + " probe(s); this is model-branch variance, not a functional verdict"
        Write-Bad $detail
    }
    else {
        $boundaryStatus = [string]$boundaryResponse.goal_status
        $boundaryStop = [string]$boundaryResponse.stop_reason
        $boundaryCards = @($boundaryResponse.interaction_requests | ForEach-Object { [string]$_.kind })
        try {
            $eventsResponse = Invoke-Json -Method GET -Url ($base + "/agent/events?conversation_id=" + $conversationID + "&since=0&limit=200") -Body $null -TimeoutSec 30
            $lastEvents = @($eventsResponse.events)
        }
        catch { $lastEvents = @() }
        $pendingEvents = @($lastEvents | Where-Object { $_.type -eq "mix_tick.pending" })
        $storedLines = @(Read-LogLines -Path $AgentLog | Where-Object { $_.Contains("[mix.tick.pending] stored") })
        $candidateInForce = ($boundaryCards -contains "mix_tick_confirmation") -or ($pendingEvents.Count -gt 0) -or ($storedLines.Count -gt 0)
        if ($candidateInForce) { $boundaryBranch = "pending_mix_tick_candidate" } else { $boundaryBranch = "no_pending_candidate" }
        Add-Prereq ("boundary_status=" + $boundaryStatus + " stop=" + $boundaryStop + " branch=" + $boundaryBranch +
            " pending_events=" + $pendingEvents.Count + " stored_log_lines=" + $storedLines.Count)

        Write-Step ("Answer the boundary with the natural affirmative (base64 answer, " + $boundaryBranch + ")")
        $logIndexBefore = @(Read-LogLines -Path $AgentLog).Count
        $answerResponse = Invoke-Json -Method POST -Url ($base + "/agent/chat") -Body @{ conversation_id = $conversationID; message = $AnswerText } -TimeoutSec 300
        $answerResponse | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath (Join-Path $RunRoot "answer_response.json") -Encoding UTF8
        Write-Ok ("answer: goal_status=" + [string]$answerResponse.goal_status + " stop=" + [string]$answerResponse.stop_reason)

        # The agent's last-log writer flushes on its own cadence, so the window is
        # polled until the accept-branch line shows up (round 2, 09:39: the line
        # was on disk at 09:40:51 while an immediate read after the 09:40:52
        # response still saw the pre-flush file).
        $newLines = @()
        $routedLines = @()
        $semanticEntryLines = @()
        $logDeadline = (Get-Date).AddSeconds($LogFlushSeconds)
        while ($true) {
            $answerLines = @(Read-LogLines -Path $AgentLog)
            $newLines = @()
            for ($i = $logIndexBefore; $i -lt $answerLines.Count; $i++) { $newLines += $answerLines[$i] }
            $routedLines = @($newLines | Where-Object { $_.Contains($RoutedLogNeedle) })
            $semanticEntryLines = @($newLines | Where-Object { $_.Contains($SemanticEntryLogNeedle) })
            if ($routedLines.Count -gt 0 -or $semanticEntryLines.Count -gt 0 -or (Get-Date) -ge $logDeadline) { break }
            Start-Sleep -Milliseconds 500
        }
        $newLines | Set-Content -LiteralPath (Join-Path $RunRoot "answer_log_window.txt") -Encoding UTF8
        $diagnosticHits = Get-DiagnosticHits -Text ([string]$answerResponse.reply)

        Add-Prereq ("answer_stop=" + [string]$answerResponse.stop_reason + " routed_lines=" + $routedLines.Count +
            " semantic_entry_fallback_lines=" + $semanticEntryLines.Count + " diagnostic_needles=" + ($diagnosticHits -join ","))

        # CONFIRM-CHAT-1 追加证据（同会话 8/9 轮的形态）：短直令先落在 clarification /
        # plan 确认（「确认后我就执行…吗？」），答一句肯定才把链推向 mix_tick 待确认卡。
        # 因此第一轮作答后按同一个脚本继续驱动，直到 mix_tick 候选真的在场，再用自然
        # 肯定作答——那才是卡面验收要的「候选在场时被消费为确认执行」。只认真正的
        # mix_tick 卡：轮 3（09:41）证明更松的判据会把 plan 确认（卡 kind=confirmation，
        # 回「当前操作仍在等待确认…」）误当成 mix_tick 卡。
        $acceptFamily = @("expired_pending_mix_tick_candidate", "mix_tick_confirmation_failed", "mix_tick_applied_reobserved", "mix_tick_rejected")
        $driveRound = 0
        $branchAEvaluated = $false
        while ($driveRound -lt $MaxDriveRounds -and -not $branchAEvaluated -and (Get-Date) -lt $budgetDeadline) {
            $driveCards = @($answerResponse.interaction_requests | ForEach-Object { [string]$_.kind })
            $driveStop = [string]$answerResponse.stop_reason
            $mixTickInForce = ($driveCards -contains "mix_tick_confirmation")
            $otherBoundary = ($driveCards -contains "confirmation") -or [bool]$answerResponse.needs_confirmation -or
                ($driveStop -eq "needs_confirmation") -or ($driveStop -eq "needs_clarification")
            if ($mixTickInForce) {
                Write-Step ("Drive round " + $driveRound + ": a mix-tick candidate is in force, answer the card with the natural affirmative")
                $driveLogIndexBefore = @(Read-LogLines -Path $AgentLog).Count
                $driveResponse = Invoke-Json -Method POST -Url ($base + "/agent/chat") -Body @{ conversation_id = $conversationID; message = $AnswerText } -TimeoutSec 300
                $driveResponse | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath (Join-Path $RunRoot ("drive_" + $driveRound + "_natural_answer.json")) -Encoding UTF8
                $driveWindow = @()
                $driveRouted = @()
                $driveSemantic = @()
                $driveDeadline = (Get-Date).AddSeconds($LogFlushSeconds)
                while ($true) {
                    $driveLines = @(Read-LogLines -Path $AgentLog)
                    $driveWindow = @()
                    for ($i = $driveLogIndexBefore; $i -lt $driveLines.Count; $i++) { $driveWindow += $driveLines[$i] }
                    $driveRouted = @($driveWindow | Where-Object { $_.Contains($RoutedLogNeedle) })
                    $driveSemantic = @($driveWindow | Where-Object { $_.Contains($SemanticEntryLogNeedle) })
                    if ($driveRouted.Count -gt 0 -or $driveSemantic.Count -gt 0 -or (Get-Date) -ge $driveDeadline) { break }
                    Start-Sleep -Milliseconds 500
                }
                $driveWindow | Set-Content -LiteralPath (Join-Path $RunRoot ("drive_" + $driveRound + "_log_window.txt")) -Encoding UTF8
                $driveStopAfter = [string]$driveResponse.stop_reason
                $driveHits = Get-DiagnosticHits -Text ([string]$driveResponse.reply)
                # 必然是分支之后的任一停止原因都只可能由「确认被执行面消费」产生。
                $consumed = ($driveRouted.Count -gt 0) -or ($acceptFamily -contains $driveStopAfter)
                $boundaryBranch = "pending_mix_tick_candidate"
                $answerResponse = $driveResponse
                $routedLines = $driveRouted
                $semanticEntryLines = $driveSemantic
                $diagnosticHits = $driveHits
                Add-Prereq ("drive_" + $driveRound + "_stop=" + $driveStopAfter + " routed_lines=" + $driveRouted.Count +
                    " consumed=" + $consumed + " diagnostic_needles=" + ($driveHits -join ","))
                if ($driveHits.Count -gt 0) {
                    $verdict = "assertion_failure"
                    $detail = "P1(branch A): the answer to the live card carried the semantic-entry diagnostic: " + ($driveHits -join ",")
                    Write-Bad $detail
                }
                elseif (-not $consumed) {
                    $verdict = "assertion_failure"
                    $detail = "P3: a pending mix-tick card was in force but the natural affirmative was not consumed as a confirmation (stop=" + $driveStopAfter + ")"
                    Write-Bad $detail
                }
                else {
                    $verdict = "pass"
                    $detail = "branch=pending_mix_tick_candidate consumed=true stop=" + $driveStopAfter + " routed=" + $driveRouted.Count +
                        " semantic_entry_fallback=" + $driveSemantic.Count
                    Write-Ok $detail
                }
                $branchAEvaluated = $true
            }
            elseif ($otherBoundary -and $driveRound -lt ($MaxDriveRounds - 1)) {
                Write-Step ("Drive round " + $driveRound + ": boundary is " + $driveStop + " with cards [" + ($driveCards -join ",") + "], approve with the explicit phrase to keep the chain going")
                $driveResponse = Invoke-Json -Method POST -Url ($base + "/agent/chat") -Body @{ conversation_id = $conversationID; message = $PlanApprovalText } -TimeoutSec 300
                $driveResponse | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath (Join-Path $RunRoot ("drive_" + $driveRound + "_approval.json")) -Encoding UTF8
                $answerResponse = $driveResponse
                Add-Prereq ("drive_" + $driveRound + "_approval_stop=" + [string]$driveResponse.stop_reason +
                    " cards=" + (@($driveResponse.interaction_requests | ForEach-Object { [string]$_.kind }) -join ","))
                Write-Ok ("drive " + $driveRound + ": approved, now goal_status=" + [string]$driveResponse.goal_status + " stop=" + [string]$driveResponse.stop_reason)
            }
            else {
                break
            }
            $driveRound++
        }
        if (-not $branchAEvaluated -and $diagnosticHits.Count -gt 0) {
            $verdict = "assertion_failure"
            $detail = "P1: the answer's user-facing reply carried the semantic-entry diagnostic: " + ($diagnosticHits -join ",")
            Write-Bad $detail
        }
        elseif ([string]$answerResponse.stop_reason -eq "semantic_entry_invalid") {
            $verdict = "assertion_failure"
            $detail = "P2: the natural affirmative was classified as an invalid request"
            Write-Bad $detail
        }
        elseif ($boundaryBranch -eq "pending_mix_tick_candidate" -and $routedLines.Count -eq 0 -and
            (@("expired_pending_mix_tick_candidate", "mix_tick_confirmation_failed", "mix_tick_applied_reobserved", "mix_tick_rejected") -notcontains [string]$answerResponse.stop_reason)) {
            $verdict = "assertion_failure"
            $detail = "P3: a pending mix-tick confirmation was in force but the natural affirmative was not consumed as confirmation (stop=" + [string]$answerResponse.stop_reason + ")"
            Write-Bad $detail
        }
        elseif ($boundaryBranch -eq "pending_mix_tick_candidate" -and [string]$answerResponse.stop_reason -eq "ambiguous_mix_tick_confirmation") {
            $verdict = "assertion_failure"
            $detail = "P3: the natural affirmative was held as an ambiguous answer while a confirmation was in force"
            Write-Bad $detail
        }
        elseif (-not (Test-HasCJK -Text ([string]$answerResponse.reply))) {
            $verdict = "assertion_failure"
            $detail = "P1: the answer's user-facing reply carried no Chinese at all: " + [string]$answerResponse.reply
            Write-Bad $detail
        }
        else {
            $verdict = "pass"
            $detail = "boundary=" + $boundaryBranch + " answer_stop=" + [string]$answerResponse.stop_reason +
                " routed=" + $routedLines.Count + " semantic_entry_fallback=" + $semanticEntryLines.Count
            Write-Ok $detail
        }
    }

    try { $lastRuntime = Invoke-Json -Method GET -Url ($base + "/agent/runtime/status") -Body $null -TimeoutSec 30 } catch { }
    if ($lastEvents.Count -eq 0) {
        try {
            $eventsResponse = Invoke-Json -Method GET -Url ($base + "/agent/events?conversation_id=" + $conversationID + "&since=0&limit=200") -Body $null -TimeoutSec 30
            $lastEvents = @($eventsResponse.events)
        }
        catch { }
    }
}
catch {
    $verdict = "env_failure"
    $detail = $_.Exception.Message
    Write-Bad ("env: " + $detail)
}
finally {
    Write-Step "Teardown"
    foreach ($pair in @(@("agent", $agentProcId), @("kernel", $kernelProcId))) {
        $name = $pair[0]
        $procId = [int]$pair[1]
        if ($procId -gt 0) {
            try {
                Stop-Process -Id $procId -Force -ErrorAction Stop
                Wait-Process -Id $procId -Timeout 20 -ErrorAction SilentlyContinue
                Add-Prereq ("stopped_" + $name + "_pid=" + $procId)
            }
            catch { Add-Prereq ("stop_failed_" + $name + "_pid=" + $procId + " " + $_.Exception.Message) }
        }
    }
    Start-Sleep -Seconds 1
    $portsLeft = New-Object System.Collections.Generic.List[string]
    foreach ($port in 7878, 5555, 5556) {
        $listener = Get-TcpListener -Port $port
        if ($listener) { $portsLeft.Add([string]$port); Add-Prereq ("PORT_STILL_BUSY=" + $port + " pid=" + $listener.OwningProcess) }
        else { Add-Prereq ("port_released=" + $port) }
    }
    $projectHashAfter = (Get-FileHash -Algorithm SHA256 -LiteralPath $RepoDefaultProject).Hash
    $userProjectAfter = Get-ProjectFingerprint -Path $UserProjectDir
    Add-Prereq ("repo_default_project_sha256_after=" + $projectHashAfter)
    Add-Prereq ("user_project_fingerprint_after=" + $userProjectAfter)
    if ($projectHashBefore -ne $projectHashAfter) { Add-Prereq "note=repo default_project.xml mutated by the kernel during the run (whitelisted runtime increment)" }
    if ($userProjectBefore -ne $userProjectAfter) { Add-Prereq "USER_PROJECT_MUTATED=1" }
    $agentLogCopy = Join-Path $RunRoot "agent_log_final.txt"
    try { Copy-Item -LiteralPath $AgentLog -Destination $agentLogCopy -ErrorAction Stop } catch { }
    $lastRuntime | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath $RuntimePath -Encoding UTF8
    $lastEvents | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath $EventsPath -Encoding UTF8
    Add-Prereq ("finished=" + (Get-Date -Format o))
    $prereq | Out-File -FilePath $PrereqPath -Encoding utf8

    # Windows PowerShell 5.1 has no "if" expression, so every conditional field
    # is materialised before the report literal.
    $boundaryStatusOut = ""
    $boundaryStopOut = ""
    $boundaryCardsOut = @()
    if ($null -ne $boundaryResponse) {
        $boundaryStatusOut = [string]$boundaryResponse.goal_status
        $boundaryStopOut = [string]$boundaryResponse.stop_reason
        $boundaryCardsOut = @($boundaryResponse.interaction_requests | ForEach-Object { [string]$_.kind })
    }
    $answerStatusOut = ""
    $answerStopOut = ""
    $answerReplyOut = ""
    $answerCardsOut = @()
    if ($null -ne $answerResponse) {
        $answerStatusOut = [string]$answerResponse.goal_status
        $answerStopOut = [string]$answerResponse.stop_reason
        $answerReplyOut = [string]$answerResponse.reply
        $answerCardsOut = @($answerResponse.interaction_requests | ForEach-Object { [string]$_.kind })
    }
    $report = [ordered]@{
        schema_version     = "confirm_chat_1_smoke.v1"
        run_root           = $RunRoot
        verdict            = $verdict
        detail             = $detail
        boundary_branch    = $boundaryBranch
        boundary_status    = $boundaryStatusOut
        boundary_stop      = $boundaryStopOut
        boundary_cards     = $boundaryCardsOut
        answer_status      = $answerStatusOut
        answer_stop        = $answerStopOut
        answer_reply       = $answerReplyOut
        answer_cards       = $answerCardsOut
        diagnostic_hits    = $diagnosticHits
        routed_log_lines   = @($routedLines).Count
        semantic_entry_log = @($semanticEntryLines).Count
        probes             = $probes.ToArray()
        ports_left_busy    = $portsLeft.ToArray()
    }
    $report | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath $ReportPath -Encoding UTF8
    Write-Host ("summary: " + ($report | ConvertTo-Json -Depth 4 -Compress))
}

if ($verdict -eq "pass") { Write-Host "CONFIRM_CHAT_1_EXIT 0 (pass)" -ForegroundColor Green; exit 0 }
if ($verdict -eq "no_boundary" -or $verdict -eq "env_failure") { Write-Host ("CONFIRM_CHAT_1_EXIT 2 (" + $verdict + ")") -ForegroundColor Yellow; exit 2 }
Write-Host ("CONFIRM_CHAT_1_EXIT 1 (" + $verdict + ")") -ForegroundColor Red
exit 1
