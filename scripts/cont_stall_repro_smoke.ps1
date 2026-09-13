<#
CONT-STALL-1 real-stack reproduction smoke (machine walk).
The human hand test still goes through the Godot entry (AGENTS.md section 5).

Shape under test (2026-09-12 22:53 real stack, conversation webui_mtyi4sjw,
goal_5b9cb1a9e48ace5b): a same-type prompt ends its slice at
waiting_continue + stop=limit_reached, and NOTHING follows -- the durable
continuation is never claimed, the goal parks, and the user only learns about
it 203 s later by pressing stop. The turn's own reply claims the opposite
("我还在继续处理这个任务，完成后再向你汇报。").

This script measures the ARMING LATENCY between the HTTP slice's end and the
next scheduler claim, using the CONT-STALL-1 observability lines:
  [continuation.arm]   the slice boundary's arming decision
  [continuation.wake]  whether the scheduler was woken
  [continuation.claim] the claim itself
  [continuation.stall] a drivable record the scheduler could not take

Verdicts:
  armed              next slice claimed within -ArmBudgetSeconds        -> exit 0
  parked_reported    no next slice, but an explicit residency receipt
                     reached the conversation flow                       -> exit 0
  dead_park          no next slice, no receipt, goal still waiting      -> exit 1
  finished           chain settled completed                            -> exit 0
  chain_failed/stopped/cancelled  honest terminal, not the stall shape  -> exit 1
  timeout / env_failure                                                 -> exit 2

Usage:
  powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\cont_stall_repro_smoke.ps1
  powershell ... -AgentBinary <path> -RunRoot <dir> -ArmBudgetSeconds 45
#>

[CmdletBinding()]
param(
    [string]$RepoRoot = "",
    [string]$RunRoot = "",
    [string]$Affix = "cont_stall",
    [switch]$SkipBuild,
    [string]$AgentBinary = "",
    [string]$KernelExe = "",
    [int]$WaitSeconds = 30,
    [int]$KernelDwellSeconds = 20,
    [int]$ArmBudgetSeconds = 45,
    [int]$ChainBudgetSeconds = 300,
    [int]$PollSeconds = 3,
    # The card's literal prompt is the default. A same-type alternative can be
    # supplied base64-encoded (ASCII-safe for this script's encoding contract):
    # the slice boundary under test is reached only when the model picks the
    # work route, and the literal prompt's branch is model variance
    # (2026-09-13: 1 of 9 rounds reached it).
    [string]$PromptBase64 = "",
    # A same-type prompt can legitimately park at an answerable clarification
    # first (2026-09-13 09:00 baseline: "确认把第 1 轨（bass…）提升 1 dB 吗？").
    # That boundary is NOT the defect under test; these nudges drive the chain
    # on to the limit_reached slice boundary the card names.
    [int]$MaxNudges = 3
)

$ErrorActionPreference = "Stop"

function From-B64 {
    param([string]$Value)
    return [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String($Value))
}

# ASCII-safe literals (Windows PowerShell reads BOM-less .ps1 as ANSI; Chinese
# literals would mangle, so every non-empty non-ASCII string rides base64).
$PromptText = From-B64 "5oqK5L2O6Z+z6L2oL2Jhc3Mg5o+QIDFkQg=="
if (-not [string]::IsNullOrWhiteSpace($PromptBase64)) { $PromptText = From-B64 $PromptBase64 }
$NeedleContinue = From-B64 "57un57ut"
$NeedleWait = From-B64 "562J5b6F57ut6LeR"
$NeedleUnfinished = From-B64 "6L+Y5rKh5YGa5a6M"
$NeedleStall = From-B64 "6am755WZ"
# The same-type prompt frequently parks at an answerable clarification first
# ("确认把第 1 轨（bass…）提升 1 dB 吗？"). A user answers it by approving, not by
# saying "继续" -- and the approval is what drives the chain on into the work
# slices where the limit_reached boundary under test actually occurs.
$NudgeText = From-B64 "56Gu6K6k5omn6KGM"

function Resolve-RepoRoot {
    param([string]$Explicit)
    if (-not [string]::IsNullOrWhiteSpace($Explicit)) { return (Resolve-Path -LiteralPath $Explicit).Path }
    return (Split-Path -Parent (Split-Path -Parent $MyInvocation.ScriptName))
}

function firstNonEmptyLocal { param([string]$A, [string]$B) if (-not [string]::IsNullOrWhiteSpace($A)) { return $A } return $B }

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

# Read-LogLines returns the agent log's UTF-8 lines, tolerating a file that is
# still being appended to.
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

# LogStamp parses the leading yyyy-MM-ddTHH:mm:ss of a log line.
function LogStamp {
    param([string]$Line)
    if ($Line -match '^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2})') {
        try { return [datetime]::ParseExact($Matches[1], "yyyy-MM-dd'T'HH:mm:ss", $null) } catch { return $null }
    }
    return $null
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
$ReportPath = Join-Path $RunRoot "cont_stall_report.json"
$EventsPath = Join-Path $RunRoot "events.json"
$ChatPath = Join-Path $RunRoot "chat_response.json"
$RuntimePath = Join-Path $RunRoot "runtime_status.json"
if ([string]::IsNullOrWhiteSpace($KernelExe)) { $KernelExe = Join-Path $RepoRoot "VitApp\build\VitApp_artefacts\Release\VitApp.exe" }
$RepoDefaultProject = Join-Path $RepoRoot "VitApp\Workspace\default_project.xml"
# The user's own project must never be touched; it is fingerprinted before and
# after purely as evidence that this run stayed on its own draft copy.
$UserProjectDir = "D:\Godot\project\vit-daw-frontend\912.vit"

$prereq = New-Object System.Collections.Generic.List[string]
function Add-Prereq { param([string]$Line) $script:prereq.Add($Line); Write-Host $Line }

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

$kernelProcId = $null
$agentProcId = $null
$outcome = "env_failure"
$detail = ""
$chatResponse = $null
$lastRuntime = $null
$lastEvents = @()
$armLatencyMs = $null
$workMs = $null
$parkMs = $null
$timeline = [ordered]@{}

try {
    Add-Prereq ("run_root=" + $RunRoot)
    Add-Prereq ("affix=" + $Affix)
    Add-Prereq ("head=" + (& git -C $RepoRoot rev-parse HEAD))
    Add-Prereq ("git_status=" + ((& git -C $RepoRoot status --short) -join " ; "))

    Write-Step "Prereq: ports 7878/5555/5556 must be free"
    foreach ($port in 7878, 5555, 5556) {
        if (Get-TcpListener -Port $port) { throw ("port " + $port + " is already listening; aborting before any start (stack ownership rule)") }
        Add-Prereq ("port_free=" + $port)
    }

    if ([string]::IsNullOrWhiteSpace($AgentBinary)) { $AgentBinary = Join-Path $RunRoot "bin\VitAgent.contstall.exe" }
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

    $conversationID = "contstall_" + (Get-Date -Format "yyyyMMdd_HHmmss")
    Write-Step ("Drive the CONT-STALL-1 same-type prompt on " + $conversationID)
    $slices = New-Object System.Collections.Generic.List[object]
    $claimLine = ""
    $verdict = ""
    $sliceIndex = 0
    $armLatencyMs = $null
    while ($sliceIndex -le $MaxNudges) {
        $message = if ($sliceIndex -eq 0) { $PromptText } else { $NudgeText }
        $sliceStarted = Get-Date
        $chatResponse = Invoke-Json -Method POST -Url ($base + "/agent/chat") -Body @{ conversation_id = $conversationID; message = $message } -TimeoutSec 300
        $sliceEnded = Get-Date
        # Every line the HTTP slice itself writes (including its own
        # agent_loop_chat) exists by now, so any later line is scheduler work.
        # Indexing beats timestamp comparison: log stamps have 1 s resolution
        # and a sub-second claim would otherwise collide with the turn's own
        # line (the first baseline attempt read arm_latency_ms=-366 that way).
        $logIndexAtEnd = @(Read-LogLines -Path $AgentLog).Count
        $chatResponse | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath ($ChatPath + "." + $sliceIndex) -Encoding UTF8
        if ($sliceIndex -eq 0) { $chatGoalID = [string]$chatResponse.goal_id } else { $chatGoalID = firstNonEmptyLocal $chatGoalID ([string]$chatResponse.goal_id) }
        $sliceStatus = [string]$chatResponse.goal_status
        $sliceStop = [string]$chatResponse.stop_reason
        Add-Prereq ("slice_" + $sliceIndex + "_goal=" + $chatGoalID + " status=" + $sliceStatus + " stop=" + $sliceStop + " ms=" + [int](($sliceEnded - $sliceStarted).TotalMilliseconds))
        Write-Ok ("slice " + $sliceIndex + ": goal_status=" + $sliceStatus + " stop=" + $sliceStop)
        $slices.Add([ordered]@{
            index = $sliceIndex; status = $sliceStatus; stop_reason = $sliceStop
            started_at = $sliceStarted.ToString("o"); ended_at = $sliceEnded.ToString("o")
            work_ms = [int](($sliceEnded - $sliceStarted).TotalMilliseconds)
            reply = [string]$chatResponse.reply
        })
        if ($sliceIndex -eq 0) {
            $timeline["turn_started_at"] = $sliceStarted.ToString("o")
            $timeline["turn_ended_at"] = $sliceEnded.ToString("o")
            $timeline["http_slice_ms"] = [int](($sliceEnded - $sliceStarted).TotalMilliseconds)
        }

        if ($sliceStatus -eq "waiting_continue") {
            Write-Step ("Arming window " + $ArmBudgetSeconds + "s after slice " + $sliceIndex)
            $armDeadline = (Get-Date).AddSeconds($ArmBudgetSeconds)
            while ((Get-Date) -lt $armDeadline) {
                Start-Sleep -Seconds $PollSeconds
                $lines = @(Read-LogLines -Path $AgentLog)
                for ($i = $logIndexAtEnd; $i -lt $lines.Count; $i++) {
                    $line = $lines[$i]
                    $isClaim = $line -like "*[continuation.claim] claimed*"
                    # Baseline (pre-observability) builds have no
                    # [continuation.claim] line, so a scheduler slice's own logs
                    # are the arming evidence there: an agent_loop_chat for this
                    # goal, or the chain-end gate line. Both are emitted only by
                    # a scheduler-driven slice.
                    $isSlice = ($line -like "*agent_loop_chat*" -and $line -like ("*goal=" + $chatGoalID + "*")) -or
                               ($line -like "*[f6.gate]*" -and $line -like ("*goal=" + $chatGoalID + "*"))
                    if ($isClaim -or $isSlice) { $claimLine = $line; $claimStamp = LogStamp -Line $line; break }
                }
                if ($claimLine -ne "") { break }
                try {
                    $lastRuntime = Invoke-Json -Method GET -Url ($base + "/agent/runtime/status") -Body $null -TimeoutSec 30
                    $goalStatus = [string]$lastRuntime.goal.status
                    if ($goalStatus -eq "completed") { $verdict = "finished"; break }
                    if ($goalStatus -eq "failed" -or $goalStatus -eq "stopped" -or $goalStatus -eq "cancelled") { $verdict = "chain_" + $goalStatus; break }
                }
                catch { }
            }
            if ($claimLine -ne "") {
                if ($null -ne $claimStamp) {
                    # Log stamps have 1 s resolution, so a sub-second claim reads
                    # as a small negative delta against the response timestamp.
                    # The claim itself is proven by log ORDER (only lines written
                    # after the response are scanned), never by the stamp.
                    $raw = [int](($claimStamp - $sliceEnded).TotalMilliseconds)
                    $armLatencyMs = if ($raw -lt 0) { 0 } else { $raw }
                    $timeline["arm_latency_raw_ms"] = $raw
                    $timeline["arm_latency_resolution"] = "log timestamps are second-resolution; a same-second claim reports 0"
                    $timeline["next_slice_claimed_at"] = $claimStamp.ToString("o")
                }
                $verdict = "armed"
            } elseif ([string]::IsNullOrWhiteSpace($verdict)) {
                try {
                    $eventsResponse = Invoke-Json -Method GET -Url ($base + "/agent/events?conversation_id=" + $conversationID + "&since=0&limit=200") -Body $null -TimeoutSec 30
                    $lastEvents = @($eventsResponse.events)
                }
                catch { $lastEvents = @() }
                $chainResults = @($lastEvents | Where-Object { $_.item_id -eq "chain_result" })
                $body = ""
                if ($chainResults.Count -gt 0) { $body = [string]$chainResults[-1].body }
                if (($body -like ("*" + $NeedleContinue + "*")) -or ($body -like ("*" + $NeedleWait + "*")) -or ($body -like ("*" + $NeedleUnfinished + "*")) -or ($body -like ("*" + $NeedleStall + "*"))) {
                    $verdict = "parked_reported"
                } else {
                    $verdict = "dead_park"
                }
                $timeline["residency_receipt_count"] = $chainResults.Count
                $timeline["residency_receipt_body"] = $body
            }
            $timeline["arm_latency_ms"] = $armLatencyMs
            $timeline["claim_line"] = $claimLine
            $timeline["parked_slice_index"] = $sliceIndex
            break
        }
        if ($sliceStatus -eq "completed") { $verdict = "finished"; break }
        if ($sliceStatus -eq "failed" -or $sliceStatus -eq "stopped" -or $sliceStatus -eq "cancelled") { $verdict = "chain_" + $sliceStatus; break }
        if ($sliceStatus -eq "waiting_clarification" -or $sliceStatus -eq "waiting_confirmation") { $sliceIndex++; continue }
        break
    }
    if ([string]::IsNullOrWhiteSpace($verdict)) { $verdict = "timeout" }
    # NOTE (2026-09-13 09:05/09:06 runs): neither an OrderedDictionary indexer
    # assignment nor "@()" over a generic List[object] is safe on this host --
    # both raise "Argument types do not match". ToArray() is the portable form.
    $sliceRows = $slices.ToArray()

    if ($verdict -eq "armed") {
        Write-Step ("Chain budget " + $ChainBudgetSeconds + "s")
        $deadline = (Get-Date).AddSeconds($ChainBudgetSeconds)
        while ((Get-Date) -lt $deadline) {
            Start-Sleep -Seconds $PollSeconds
            try {
                $lastRuntime = Invoke-Json -Method GET -Url ($base + "/agent/runtime/status") -Body $null -TimeoutSec 30
                $eventsResponse = Invoke-Json -Method GET -Url ($base + "/agent/events?conversation_id=" + $conversationID + "&since=0&limit=200") -Body $null -TimeoutSec 30
                $lastEvents = @($eventsResponse.events)
            }
            catch { continue }
            $goalStatus = [string]$lastRuntime.goal.status
            $taskState = [string]$lastRuntime.task_state
            $live = @($lastRuntime.continuations | Where-Object { $_.status -eq "pending" -or $_.status -eq "claimed" -or $_.status -eq "running" })
            Write-Host ("poll goal=" + $goalStatus + " task=" + $taskState + " live_continuations=" + $live.Count)
            if ($goalStatus -eq "completed") { $verdict = "finished"; break }
            if ($goalStatus -eq "failed" -or $goalStatus -eq "stopped" -or $goalStatus -eq "cancelled") { $verdict = "chain_" + $goalStatus; break }
        }
    }

    if ($null -eq $lastRuntime) {
        try { $lastRuntime = Invoke-Json -Method GET -Url ($base + "/agent/runtime/status") -Body $null -TimeoutSec 30 } catch { }
    }
    if ($lastEvents.Count -eq 0) {
        try {
            $eventsResponse = Invoke-Json -Method GET -Url ($base + "/agent/events?conversation_id=" + $conversationID + "&since=0&limit=200") -Body $null -TimeoutSec 30
            $lastEvents = @($eventsResponse.events)
        }
        catch { }
    }

    $turnTerminals = @($lastEvents | Where-Object { $_.type -eq "turn.completed" -or $_.type -eq "turn.failed" })
    $turnStops = @($lastEvents | Where-Object { $_.type -eq "trajectory.turn.stopped" -or $_.type -eq "turn.stopped" })
    $workNodes = @($lastEvents | Where-Object { $_.type -eq "turn.started" })
    if ($workNodes.Count -gt 0 -and $turnTerminals.Count -gt 0) {
        $first = [datetime]$workNodes[0].created_at
        $lastWork = [datetime]$turnTerminals[-1].created_at
        $workMs = [int](($lastWork - $first).TotalMilliseconds)
    }
    if ($turnTerminals.Count -gt 0 -and $turnStops.Count -gt 0) {
        $lastWork = [datetime]$turnTerminals[-1].created_at
        $stop = [datetime]$turnStops[-1].created_at
        $parkMs = [int](($stop - $lastWork).TotalMilliseconds)
    }
    $timeline["work_ms_from_events"] = $workMs
    $timeline["park_ms_from_events"] = $parkMs
    $timeline["turn_terminal_events"] = $turnTerminals.Count
    $timeline["turn_stopped_events"] = $turnStops.Count

    $obs = @(Read-LogLines -Path $AgentLog | Where-Object { $_ -like "*[continuation.arm]*" -or $_ -like "*[continuation.wake]*" -or $_ -like "*[continuation.claim]*" -or $_ -like "*[continuation.stall]*" })
    $obs | Set-Content -LiteralPath (Join-Path $RunRoot "continuation_log.txt") -Encoding UTF8
    $timeline["continuation_log_lines"] = $obs.Count

    $lastRuntime | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath $RuntimePath -Encoding UTF8
    $lastEvents | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath $EventsPath -Encoding UTF8

    $report = [ordered]@{
        schema_version   = "cont_stall_repro.v1"
        run_root         = $RunRoot
        conversation_id  = $conversationID
        verdict          = $verdict
        goal_id          = $chatGoalID
        goal_status      = if ($null -ne $lastRuntime) { $lastRuntime.goal.status } else { "" }
        task_state       = if ($null -ne $lastRuntime) { $lastRuntime.task_state } else { "" }
        chat_goal_status = $chatResponse.goal_status
        chat_stop_reason = $chatResponse.stop_reason
        arm_latency_ms   = $armLatencyMs
        slices           = $sliceRows
        timeline         = $timeline
        prereq           = @($prereq)
    }
    $report | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath $ReportPath -Encoding UTF8

    Write-Step ("Verdict: " + $verdict)
    Add-Prereq ("verdict=" + $verdict)
    Add-Prereq ("arm_latency_ms=" + $armLatencyMs)
    $outcome = $verdict
}
catch {
    $detail = $_.Exception.Message
    $where = ""
    if ($null -ne $_.InvocationInfo) { $where = [string]$_.InvocationInfo.PositionMessage }
    $stack = [string]$_.ScriptStackTrace
    Add-Prereq ("fatal=" + $detail)
    Add-Prereq ("fatal_type=" + $_.Exception.GetType().FullName)
    Add-Prereq ("fatal_where=" + ($where -replace "?
", " | "))
    Add-Prereq ("fatal_stack=" + ($stack -replace "?
", " | "))
    Write-Bad ("fatal: " + $detail)
    Write-Bad ("at: " + ($where -replace "?
", " | "))
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
}

Write-Host ""
Write-Host ("CONT_STALL_VERDICT " + $outcome)
switch ($outcome) {
    "armed" { exit 0 }
    "parked_reported" { exit 0 }
    "finished" { exit 0 }
    "dead_park" { exit 1 }
    "chain_failed" { exit 1 }
    "chain_stopped" { exit 1 }
    "chain_cancelled" { exit 1 }
    default { exit 2 }
}
