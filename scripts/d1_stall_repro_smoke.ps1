<#
D1-STALL-1 real-stack reproduction / forensics smoke (machine walk).
The human hand test still goes through the Godot entry (AGENTS.md section 5).

Shape: under full_project_access the same-type prompt drives the free-state D1
chain (auto-apply, no confirmation card) to its post-apply evaluation step and
classifies the chain terminal:
  finished              goal completed and the task settled                 -> exit 0
  parked_owe            evaluation unfinished; goal waiting_continue and the
                        conversation flow carries the explicit owed receipt -> exit 0
  parked_owe_silent     evaluation unfinished with zero text delivery        -> exit 1
  defect_fake_complete  goal completed while task stays needs_experiment     -> exit 1
  chain_failed          honest failure terminal (not the stall shape)        -> exit 1
  timeout / env_failure                                                      -> exit 2

Also collects: PARK-1 judgment-park settle summary, TRAJ-DUAL-1 experiment turn
closure after a stand-in judgment POST.

Usage:
  powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\d1_stall_repro_smoke.ps1
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
    [int]$PollSeconds = 10
)

$ErrorActionPreference = "Stop"

function From-B64 {
    param([string]$Value)
    return [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String($Value))
}

# ASCII-safe literals (Windows PowerShell reads BOM-less .ps1 as ANSI; Chinese
# literals would mangle, so every non-empty non-ASCII string rides base64).
$PromptText = From-B64 "5a+55L2O6Z+z6L2o5YGa5LiA5qyh5re36Z+z5pS56L+b5a6e6aqM77yM5pS55a6M6K6p5oiRIEEvQiDor5XlkKzlr7nmr5TkuIDkuIs="
$NeedleContinue = From-B64 "57un57ut"
$NeedleReadback = From-B64 "5Zue6K+7"
$NeedleFinding = From-B64 "6ZKI5a+555qE5Y+R546w"
$NeedleUnfinished = From-B64 "6L+Y5rKh5YGa5a6M"
$NeedleAudition = From-B64 "QS9C"
$JudgeFreeText = From-B64 "RDEtU1RBTEwtMSBtYWNoaW5lIHdhbGs6IGJlZCBzdGFuZC1pbiBqdWRkZ21lbnQgKGh1bWFuLWVhciB2ZXJkaWN0IG91dCBvZiBzY29wZSk="

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
    $RunRoot = Join-Path $RepoRoot ("artifacts\d1_stall\" + (Get-Date -Format "yyyyMMdd_HHmmss"))
}
New-Item -ItemType Directory -Force -Path $RunRoot | Out-Null
$AgentDrafts = Join-Path $RunRoot "agent_drafts"
New-Item -ItemType Directory -Force -Path $AgentDrafts | Out-Null
$AgentBin = Join-Path $RunRoot "bin\VitAgent.d1stall.exe"
$AgentLog = Join-Path $RunRoot "agent_last.log"
$AgentDir = Join-Path $RepoRoot "agent"
$PrereqPath = Join-Path $RunRoot "prereq.txt"
$ReportPath = Join-Path $RunRoot "d1_stall_report.json"
$EventsPath = Join-Path $RunRoot "events.json"
$ChatPath = Join-Path $RunRoot "chat_response.json"
$RuntimePath = Join-Path $RunRoot "runtime_status.json"
if ([string]::IsNullOrWhiteSpace($KernelExe)) { $KernelExe = Join-Path $RepoRoot "VitApp\build\VitApp_artefacts\Release\VitApp.exe" }
$RepoDefaultProject = Join-Path $RepoRoot "VitApp\Workspace\default_project.xml"

$prereq = New-Object System.Collections.Generic.List[string]
function Add-Prereq { param([string]$Line) $script:prereq.Add($Line); Write-Host $Line }

$kernelProcId = $null
$agentProcId = $null
$outcome = "env_failure"
$detail = ""
$chatResponse = $null
$lastRuntime = $null
$lastEvents = @()
$park1 = $false
$traj = [ordered]@{}

try {
    Add-Prereq ("run_root=" + $RunRoot)
    Add-Prereq ("head=" + (& git -C $RepoRoot rev-parse HEAD))
    Add-Prereq ("git_status=" + ((& git -C $RepoRoot status --short) -join " ; "))

    Write-Step "Prereq: ports 7878/5555/5556 must be free"
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

    $conversationID = "d1stall_" + (Get-Date -Format "yyyyMMdd_HHmmss")
    Write-Step ("Drive the D1 chain prompt on " + $conversationID)
    $chatResponse = Invoke-Json -Method POST -Url ($base + "/agent/chat") -Body @{ conversation_id = $conversationID; message = $PromptText } -TimeoutSec 300
    $chatResponse | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $ChatPath -Encoding UTF8
    Add-Prereq ("chat_goal_id=" + $chatResponse.goal_id + " goal_status=" + $chatResponse.goal_status + " stop_reason=" + $chatResponse.stop_reason)
    Write-Ok ("first slice: goal_status=" + $chatResponse.goal_status + " stop=" + $chatResponse.stop_reason)

    $deadline = (Get-Date).AddSeconds($ChainBudgetSeconds)
    $verdict = ""
    while ((Get-Date) -lt $deadline) {
        Start-Sleep -Seconds $PollSeconds
        $lastRuntime = Invoke-Json -Method GET -Url ($base + "/agent/runtime/status") -Body $null -TimeoutSec 30
        $eventsResponse = Invoke-Json -Method GET -Url ($base + "/agent/events?conversation_id=" + $conversationID + "&since=0&limit=200") -Body $null -TimeoutSec 30
        $lastEvents = @($eventsResponse.events)
        $goalStatus = [string]$lastRuntime.goal.status
        $taskState = [string]$lastRuntime.task_state
        $chainResults = @($lastEvents | Where-Object { $_.item_id -eq "chain_result" })
        $body = ""
        if ($chainResults.Count -gt 0) { $body = [string]$chainResults[-1].body }
        if ($body -like ("*" + $NeedleReadback + "*") -and $body -like ("*" + $NeedleAudition + "*")) { $park1 = $true }
        $live = @($lastRuntime.continuations | Where-Object { $_.status -eq "pending" -or $_.status -eq "claimed" -or $_.status -eq "running" })
        Write-Host ("poll goal=" + $goalStatus + " task=" + $taskState + " chain_results=" + $chainResults.Count + " live_continuations=" + $live.Count)
        if ($goalStatus -eq "completed" -and $taskState -eq "needs_experiment") { $verdict = "defect_fake_complete"; break }
        if ($goalStatus -eq "completed") { $verdict = "finished"; break }
        if ($goalStatus -eq "failed" -or $goalStatus -eq "stopped" -or $goalStatus -eq "cancelled") { $verdict = "chain_" + $goalStatus; break }
        if ($goalStatus -eq "waiting_continue" -and $live.Count -eq 0 -and $chainResults.Count -gt 0) {
            if ($body -like ("*" + $NeedleContinue + "*") -or $body -like ("*" + $NeedleUnfinished + "*")) { $verdict = "parked_owe" } else { $verdict = "parked_owe_silent" }
            break
        }
    }
    if ([string]::IsNullOrWhiteSpace($verdict)) { $verdict = "timeout" }

    # TRAJ-DUAL-1 + PARK-1 evidence (best effort, no judgment submission unless a
    # complete audition session is already available).
    $sessions = @($lastEvents | Where-Object { $_.type -eq "audition.ready" })
    $turnNodes = @($lastEvents | Where-Object { $_.type -eq "turn.completed" -or $_.type -eq "turn.failed" -or $_.type -eq "turn.stopped" })
    $traj = [ordered]@{
        turn_terminal_events = $turnNodes.Count
        turn_ids           = @($turnNodes | ForEach-Object { $_.payload.turn_id } | Where-Object { $_ } | Sort-Object -Unique)
        audition_ready     = $sessions.Count
        chain_result_count = @($lastEvents | Where-Object { $_.item_id -eq "chain_result" }).Count
    }
    if ($sessions.Count -gt 0) {
        $session = $sessions[-1].payload.session
        if ($session -and $session.session_id -and $session.turn_id -and $session.round_id -and $session.project_revision) {
            $judgment = $null
            try {
                $judgment = Invoke-Json -Method POST -Url ($base + "/agent/audition/judgment") -Body @{
                    conversation_id = $conversationID; turn_id = $session.turn_id; round_id = $session.round_id;
                    audition_session_id = $session.session_id; project_revision = [string]$session.project_revision;
                    heard_difference = "yes"; preference = "a"; reason_tags = @("d1_stall_machine_walk"); free_text = $JudgeFreeText
                } -TimeoutSec 60
            }
            catch { $judgment = @{ status = "error"; error = $_.Exception.Message } }
            Start-Sleep -Seconds 20
            $postRuntime = Invoke-Json -Method GET -Url ($base + "/agent/runtime/status") -Body $null -TimeoutSec 30
            $postEvents = @((Invoke-Json -Method GET -Url ($base + "/agent/events?conversation_id=" + $conversationID + "&since=0&limit=200") -Body $null -TimeoutSec 30).events)
            $traj["judgment_http"] = $judgment.status
            $traj["judgment_payload"] = $judgment
            $traj["post_judgment_goal_status"] = $postRuntime.goal.status
            $traj["post_judgment_terminal_events"] = @($postEvents | Where-Object { $_.type -eq "turn.completed" -or $_.type -eq "turn.failed" -or $_.type -eq "turn.stopped" }).Count
            $traj["post_judgment_turn_ids"] = @($postEvents | Where-Object { $_.type -eq "turn.completed" } | ForEach-Object { $_.payload.turn_id } | Where-Object { $_ } | Sort-Object -Unique)
        }
    }

    $lastRuntime | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath $RuntimePath -Encoding UTF8
    $lastEvents | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath $EventsPath -Encoding UTF8

    $report = [ordered]@{
        schema_version   = "d1_stall_repro.v1"
        run_root         = $RunRoot
        conversation_id  = $conversationID
        verdict          = $verdict
        goal_status      = $lastRuntime.goal.status
        task_state       = $lastRuntime.task_state
        chat_goal_status = $chatResponse.goal_status
        chat_stop_reason = $chatResponse.stop_reason
        park1_settle_summary = $park1
        trajectory       = $traj
        prereq           = @($prereq)
    }
    $report | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath $ReportPath -Encoding UTF8

    Write-Step ("Verdict: " + $verdict)
    Add-Prereq ("verdict=" + $verdict)
    Add-Prereq ("park1_settle_summary=" + $park1)
    $outcome = $verdict
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
    Add-Prereq ("finished=" + (Get-Date -Format "o"))
    $prereq | Out-File -FilePath $PrereqPath -Encoding utf8
}

Write-Host ""
Write-Host ("D1_STALL_VERDICT " + $outcome)
switch ($outcome) {
    "finished" { exit 0 }
    "parked_owe" { exit 0 }
    "parked_owe_silent" { exit 1 }
    "defect_fake_complete" { exit 1 }
    "chain_failed" { exit 1 }
    "chain_stopped" { exit 1 }
    "chain_cancelled" { exit 1 }
    default { exit 2 }
}
