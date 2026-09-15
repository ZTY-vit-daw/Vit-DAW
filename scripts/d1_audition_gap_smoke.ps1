#Requires -Version 5.1
<#
D1-AUDITION-GAP-1 real-stack smoke (machine walk; the human hand test still
goes through the Godot entry, AGENTS.md section 5).

Real-stack shape under test (2026-09-13 22:58, session webui_mtzxugg8,
goal_c7ecb4fb, full access, prompt verbatim): the experiment chain runs
intent -> hypothesis -> observation -> mix_tick.pending -> intervention
applied -> readback -> five-element report, but ZERO audition.* events fire
and the goal completes mid-apply while the task still needs_experiment —
the user's own words: "第一次的要求执行，执行完成后没有返回AB试听".

The fix under test (two parts):
  1. recordGoalResult: a completed turn over a round that still owes its
     governed outcome restores the goal to waiting_continue (resumable);
  2. projectD1Execution: an applied D1-S1 intervention mounts its A/B
     audition card IMMEDIATELY (prepareFreeStateAudition), not only at the
     judgment boundary the stranded chain never reached.

Verdicts:
  audition_mounted    intervention applied AND the A/B card mounted
                      (audition.ready event / loop.AuditionSessionID)   exit 0
  no_audition         intervention applied but no A/B card — the defect   exit 1
  no_intervention     the model branch never applied an intervention this
                      round (answerable park / observation-only / etc.);
                      no verdict claimed for the card                     exit 2
  env_failure                                                                exit 2

Usage:
  powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\d1_audition_gap_smoke.ps1
#>

[CmdletBinding()]
param(
    [string]$RepoRoot = "",
    [string]$RunRoot = "",
    [string]$Affix = "d1_audition_gap",
    [switch]$SkipBuild,
    [string]$AgentBinary = "",
    [string]$KernelExe = "",
    [int]$WaitSeconds = 30,
    [int]$KernelDwellSeconds = 15,
    [int]$ChainBudgetSeconds = 300,
    [int]$PollSeconds = 5,
    [int]$MaxNudges = 2,
    [int]$SettleWatchSeconds = 0,
    [string]$PromptBase64 = ""
)

$ErrorActionPreference = "Stop"

function From-B64 {
    param([string]$Value)
    return [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String($Value))
}

# ASCII-safe literals (BOM-less ps1 is read as ANSI).
$PromptText = From-B64 "6K+35qOA5p+l5b2T5YmN5bel56iL5pyJ5LuA5LmI6Zeu6aKY"   # 请检查当前工程有什么问题
if (-not [string]::IsNullOrWhiteSpace($PromptBase64)) { $PromptText = From-B64 $PromptBase64 }
$NudgeText = From-B64 "56Gu6K6k5omn6KGM"   # 请继续 (drive an answerable park on)

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
if (Test-Path -LiteralPath $RunRoot) { throw ("run root already exists: " + $RunRoot) }
New-Item -ItemType Directory -Force -Path $RunRoot | Out-Null
$AgentDrafts = Join-Path $RunRoot "agent_drafts"
New-Item -ItemType Directory -Force -Path $AgentDrafts | Out-Null
$AgentLog = Join-Path $RunRoot "agent_last.log"
$AgentDir = Join-Path $RepoRoot "agent"
$PrereqPath = Join-Path $RunRoot "prereq.txt"
$ReportPath = Join-Path $RunRoot "d1_audition_gap_report.json"
if ([string]::IsNullOrWhiteSpace($KernelExe)) { $KernelExe = Join-Path $RepoRoot "VitApp\build\VitApp_artefacts\Release\VitApp.exe" }
$RepoDefaultProject = Join-Path $RepoRoot "VitApp\Workspace\default_project.xml"
$UserProjectDir = "D:\Godot\project\vit-daw-frontend\912.vit"

$prereq = New-Object System.Collections.Generic.List[string]
function Add-Prereq { param([string]$Line) $script:prereq.Add($Line); Write-Host $Line }

$kernelProcId = $null
$agentProcId = $null
$outcome = "env_failure"
$detail = ""
$timeline = [ordered]@{}

try {
    Add-Prereq ("run_root=" + $RunRoot)
    Add-Prereq ("head=" + (& git -C $RepoRoot rev-parse HEAD))
    Add-Prereq ("git_status=" + ((& git -C $RepoRoot status --short) -join " ; "))

    Write-Step "Prereq: ports 7878/5555/5556 must be free"
    foreach ($port in 7878, 5555, 5556) {
        if (Get-TcpListener -Port $port) { throw ("port " + $port + " is already listening") }
        Add-Prereq ("port_free=" + $port)
    }

    if ([string]::IsNullOrWhiteSpace($AgentBinary)) { $AgentBinary = Join-Path $RunRoot "bin\VitAgent.d1gap.exe" }
    if (-not $SkipBuild) {
        Write-Step "Build VitAgent (isolated binary in run dir)"
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $AgentBinary) | Out-Null
        Push-Location $AgentDir
        try {
            & go build -o $AgentBinary .\cmd\vitagent
            if ($LASTEXITCODE -ne 0) { throw ("go build failed: " + $LASTEXITCODE) }
        }
        finally { Pop-Location }
    }
    elseif (-not (Test-Path -LiteralPath $AgentBinary -PathType Leaf)) { throw "-SkipBuild given but binary missing" }
    $agentItem = Get-Item -LiteralPath $AgentBinary
    Add-Prereq ("agent_binary=" + $AgentBinary + " sha256=" + (Get-FileHash -Algorithm SHA256 -LiteralPath $AgentBinary).Hash)

    Write-Step "Start kernel"
    $projectHashBefore = (Get-FileHash -Algorithm SHA256 -LiteralPath $RepoDefaultProject).Hash
    Add-Prereq ("repo_default_project_sha256_before=" + $projectHashBefore)
    $userProjectBefore = Get-ProjectFingerprint -Path $UserProjectDir
    Add-Prereq ("user_project_fingerprint_before=" + $userProjectBefore)
    $kernelProc = Start-Process -FilePath $KernelExe -WorkingDirectory (Split-Path -Parent $KernelExe) -WindowStyle Hidden -PassThru
    $kernelProcId = $kernelProc.Id
    if (-not (Wait-PortListen -Port 5555 -TimeoutSeconds $WaitSeconds)) { throw "kernel ZMQ REQ port 5555 did not listen" }

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

    $base = "http://127.0.0.1:7878"
    $state = Invoke-Json -Method GET -Url ($base + "/agent/state") -Body $null -TimeoutSec 30
    if ($state.shadow.initialized -ne $true) { throw "shadow not initialized" }
    Add-Prereq ("s1_shadow_initialized=" + $state.shadow.initialized + " track_count=" + $state.shadow.track_count)

    if ($KernelDwellSeconds -gt 0) { Start-Sleep -Seconds $KernelDwellSeconds }

    Write-Step "Full project access"
    $auth = Invoke-Json -Method POST -Url ($base + "/agent/authority") -Body @{ authority_mode = "full_project_access" } -TimeoutSec 30
    if ($auth.authority_mode -ne "full_project_access") { throw ("authority refused: " + ($auth | ConvertTo-Json -Compress)) }

    $conversationID = "d1gap_" + (Get-Date -Format "yyyyMMdd_HHmmss")
    Write-Step ("Drive the D1-AUDITION-GAP-1 same-type prompt on " + $conversationID)

    $applied = $false
    $auditionMounted = $false
    $nudgeIndex = 0
    while ($nudgeIndex -le $MaxNudges) {
        $message = if ($nudgeIndex -eq 0) { $PromptText } else { $NudgeText }
        $chatResponse = Invoke-Json -Method POST -Url ($base + "/agent/chat") -Body @{ conversation_id = $conversationID; message = $message } -TimeoutSec 300
        $chatResponse | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath (Join-Path $RunRoot ("chat_response." + $nudgeIndex + ".json")) -Encoding UTF8
        $goalID = [string]$chatResponse.goal_id
        $goalStatus = [string]$chatResponse.goal_status
        Add-Prereq ("slice_" + $nudgeIndex + "_goal=" + $goalID + " status=" + $goalStatus + " stop=" + [string]$chatResponse.stop_reason)

        # Watch the chain (auto-continuation) until it settles; scan events for
        # the applied intervention and the mounted audition card.
        $deadline = (Get-Date).AddSeconds($ChainBudgetSeconds)
        while ((Get-Date) -lt $deadline) {
            Start-Sleep -Seconds $PollSeconds
            try {
                $eventsResponse = Invoke-Json -Method GET -Url ($base + "/agent/events?conversation_id=" + $conversationID + "&since=0&limit=400") -Body $null -TimeoutSec 30
                $events = @($eventsResponse.events)
            }
            catch { continue }
            $events | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath (Join-Path $RunRoot "events.json") -Encoding UTF8
            $appliedEvents = @($events | Where-Object { $_.type -eq "trajectory.intervention.applied" })
            if ($appliedEvents.Count -gt 0) { $applied = $true }
            $auditionEvents = @($events | Where-Object { $_.type -eq "audition.ready" -or $_.type -eq "audition.prepare" })
            if ($auditionEvents.Count -gt 0) { $auditionMounted = $true; $timeline["audition_event_types"] = (@($auditionEvents | ForEach-Object { $_.type }) -join ",") }
            $judgmentEvents = @($events | Where-Object { $_.type -like "*user_judgment*" })
            if ($judgmentEvents.Count -gt 0) { $timeline["judgment_events"] = $judgmentEvents.Count }

            try {
                $rt = Invoke-Json -Method GET -Url ($base + "/agent/runtime/status") -Body $null -TimeoutSec 30
                $gs = [string]$rt.goal.status
            } catch { $gs = "" }
            if ($applied -and $auditionMounted) { break }
            if ($gs -eq "completed" -or $gs -eq "failed" -or $gs -eq "stopped" -or $gs -eq "cancelled") {
                # Terminal without an applied intervention stays inside the
                # budget loop one more poll so late scheduler slices that apply
                # are still seen (the 22:58 real stack applied 30s after the
                # HTTP reply).
                if ($applied -and -not $auditionMounted) { Start-Sleep -Seconds 10; continue }
                break
            }
            if (-not $applied -and ($gs -eq "waiting_confirmation" -or $gs -eq "waiting_clarification")) { break }
        }
        if ($applied -and $auditionMounted) { break }
        if (-not $applied) {
            # one nudge drives an answerable park on toward the work route
            $nudgeIndex++
            continue
        }
        break
    }

    # TRAJ-AUTO-SETTLE-2 settle-tail watch: the D1-AUDITION-GAP verdict above is
    # decided the moment the intervention lands and the A/B card mounts, but the
    # round still owes its settlement tail (evaluation -> judgment request).
    # With -SettleWatchSeconds > 0 the stack stays up past the verdict so the
    # synthetic settle checkpoint can be observed end to end: armed at the
    # applied boundary (or the completed boundary), claimed by the scheduler,
    # the judgment request reaching the user (user_judgment_requested in the
    # persisted loop / judgment events), and zero owed continue receipts.
    # Purely additive observation; the verdict and exit codes are unchanged.
    # ASCII-only block: this BOM-less ps1 is read as ANSI (same discipline as
    # the prompts above), so the owed-receipt marker is decoded via From-B64.
    if ($SettleWatchSeconds -gt 0 -and $applied) {
        $OwedReceiptPattern = From-B64 "5L2g5Zue5LiA5Y+l"
        Write-Step ("Settle-tail watch (" + $SettleWatchSeconds + "s)")
        $settleLogPath = Join-Path $RunRoot "settle_watch_log.txt"
        $deadline = (Get-Date).AddSeconds($SettleWatchSeconds)
        while ((Get-Date) -lt $deadline) {
            Start-Sleep -Seconds $PollSeconds
            $logLines = @(Read-LogLines -Path $AgentLog)
            $logLines | Set-Content -LiteralPath $settleLogPath -Encoding UTF8
            $armedIds = @()
            foreach ($line in ($logLines | Where-Object { $_ -match "\[continuation\.settle\] armed" })) {
                if ($line -match "durable=(cont_[0-9a-f]+)") { $armedIds += $Matches[1] }
            }
            foreach ($line in ($logLines | Where-Object { $_ -match "settle_checkpoint=true" })) {
                if ($line -match "durable=(cont_[0-9a-f]+)") { $armedIds += $Matches[1] }
            }
            $claimedIds = @()
            foreach ($line in ($logLines | Where-Object { $_ -match "\[continuation\.claim\] claimed" })) {
                if ($line -match "claimed id=(cont_[0-9a-f]+)") { $claimedIds += $Matches[1] }
            }
            $settleClaimed = @($armedIds | Where-Object { $claimedIds -contains $_ } | Select-Object -Unique)
            $timeline["settle_armed_ids"] = (($armedIds | Select-Object -Unique) -join ",")
            $timeline["settle_claimed_ids"] = ($settleClaimed -join ",")
            $timeline["judgment_rejected_count"] = @($logLines | Where-Object { $_ -match "user judgment request rejected" }).Count
            $timeline["owed_continue_receipts"] = @($logLines | Where-Object { $_ -match $OwedReceiptPattern }).Count
            $runtimeState = Read-RuntimeStateJson -DraftRoot $AgentDrafts
            $judgmentRequested = $false
            $humanReady = $false
            if ($null -ne $runtimeState) {
                $loops = $runtimeState.free_state_reasoning_loops
                if ($null -ne $loops) {
                    $rows = @()
                    if ($loops -is [System.Array]) { $rows = @($loops) }
                    else { $rows = @($loops.PSObject.Properties | ForEach-Object { $_.Value }) }
                    foreach ($row in $rows) {
                        $exp = $row.experiment
                        if ($null -eq $exp) { continue }
                        foreach ($rd in @($exp.rounds)) {
                            if ($rd.user_judgment_requested -eq $true) { $judgmentRequested = $true }
                            $tr = $rd.experiment_target_response
                            if ($null -ne $tr -and [string]$tr.outcome -match "human_audition_ready") { $humanReady = $true }
                        }
                    }
                }
            }
            $timeline["judgment_requested_in_state"] = $judgmentRequested
            $timeline["human_audition_ready_in_state"] = $humanReady
            try {
                $eventsResponse = Invoke-Json -Method GET -Url ($base + "/agent/events?conversation_id=" + $conversationID + "&since=0&limit=400") -Body $null -TimeoutSec 30
                $judgmentEvents = @(@($eventsResponse.events) | Where-Object { $_.type -like "*user_judgment*" })
                $timeline["judgment_events"] = $judgmentEvents.Count
            } catch { }
            if ($settleClaimed.Count -gt 0 -and ($judgmentRequested -or $humanReady -or $timeline["judgment_events"] -gt 0)) {
                $timeline["settle_tail_closed_at"] = (Get-Date -Format "o")
                Write-Ok ("settle tail closed: claimed=" + ($settleClaimed -join ",") + " judgment_requested=" + $judgmentRequested + " human_ready=" + $humanReady)
                break
            }
        }
        if (-not $timeline.Contains("settle_tail_closed_at")) { Write-Info "settle watch ended without observing full closure; see settle_watch_log.txt and timeline" }
    }

    # Mechanical confirmation from the persisted runtime state: the loop's
    # AuditionSessionID is the durable mount witness even if the event scan
    # raced the poll.
    $runtimeState = Read-RuntimeStateJson -DraftRoot $AgentDrafts
    if ($null -ne $runtimeState) {
        $loops = $runtimeState.free_state_reasoning_loops
        if ($null -ne $loops) {
            $rows = @()
            if ($loops -is [System.Array]) { $rows = @($loops) }
            else { $rows = @($loops.PSObject.Properties | ForEach-Object { $_.Value }) }
            foreach ($row in $rows) {
                $sid = [string]$row.audition_session_id
                Add-Prereq ("loop goal=" + [string]$row.goal_id + " status=" + [string]$row.status + " phase=" + [string]$row.decision_phase + " audition_session=" + $sid)
                if ($sid -ne "") { $auditionMounted = $true; $timeline["loop_audition_session_id"] = $sid }
            }
        }
    }
    $owedLines = @(Read-LogLines -Path $AgentLog | Where-Object { $_ -match "\[continuation\.owed\]" })
    $owedLines | Set-Content -LiteralPath (Join-Path $RunRoot "owed_log.txt") -Encoding UTF8
    Add-Prereq ("owed_correction_lines=" + $owedLines.Count)
    $timeline["owed_correction_lines"] = $owedLines.Count
    $timeline["intervention_applied"] = $applied
    $timeline["audition_mounted"] = $auditionMounted

    if ($applied -and $auditionMounted) {
        $outcome = "audition_mounted"
        Write-Ok "intervention applied AND the A/B audition card mounted"
    }
    elseif ($applied) {
        $outcome = "no_audition"
        $detail = "intervention applied but no A/B card — the 22:58 defect shape"
        Write-Bad $detail
    }
    else {
        $outcome = "no_intervention"
        $detail = "the model branch never applied an intervention this round; no card verdict claimed"
        Write-Info $detail
    }
}
catch {
    $detail = $_.Exception.Message
    Add-Prereq ("fatal=" + $detail)
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
        schema_version = "d1_audition_gap.v1"
        run_root       = $RunRoot
        verdict        = $outcome
        detail         = $detail
        timeline       = $timeline
        prereq         = @($prereq)
    }
    $report | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath $ReportPath -Encoding UTF8
}

Write-Host ""
Write-Host ("D1_AUDITION_GAP_VERDICT " + $outcome)
switch ($outcome) {
    "audition_mounted" { exit 0 }
    "no_audition" { exit 1 }
    default { exit 2 }
}
