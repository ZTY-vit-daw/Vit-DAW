<#
B13-B real-stack smoke (2026-09-13): does the masking candidate source reach the
closure hypothesis frontier?

Shape under test (FALLBACK-1 receipt section 5, 912.vit): mix.frequency_relationship
was ready with an EMPTY conflict candidate family while mix.masking_relationship
was ready with coverage.candidate_count=138 and four directional rows. The
frontier extractor had no masking arm and its facts reader only knew the conflict
key families, so the frontier stayed empty, G5_frontier_established refused, and
the closure hard-failed.

This script drives the same-type prompt on the real stack (isolated agent binary,
isolated draft root, kernel workspace project = the repository copy -- the user's
own 912.vit is fingerprinted before/after and never opened), then reads the
durable agent_runtime_state.json and reports:
  - every closure's hypothesis_frontier candidate count and candidate view ids
  - every observation-ledger view's candidate-family count, so the
    "frequency zero + masking present" live shape is visible in the same artifact

Verdicts:
  frontier_established      frontier non-empty, candidate rows attributed to a
                            declared candidate-source view                   -> exit 0
  frontier_empty            chain reached a boundary with an empty frontier -> exit 1
  chain_failed/stopped/cancelled  honest terminal, not the frontier shape   -> exit 1
  timeout / env_failure     budget or environment                           -> exit 2

Usage:
  powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\b13b_frontier_masking_smoke.ps1
  powershell ... -AgentBinary <path> -RunRoot <dir> -ChainBudgetSeconds 420
#>

[CmdletBinding()]
param(
    [string]$RepoRoot = "",
    [string]$RunRoot = "",
    [string]$Affix = "b13b_frontier",
    [switch]$SkipBuild,
    [string]$AgentBinary = "",
    [string]$KernelExe = "",
    [int]$WaitSeconds = 30,
    [int]$KernelDwellSeconds = 20,
    [int]$ChainBudgetSeconds = 420,
    [int]$PollSeconds = 5,
    [string]$PromptBase64 = "",
    [int]$MaxNudges = 4
)

$ErrorActionPreference = "Stop"

function From-B64 {
    param([string]$Value)
    return [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String($Value))
}

# ASCII-safe literals (Windows PowerShell reads BOM-less .ps1 as ANSI; Chinese
# literals would mangle, so every non-empty non-ASCII string rides base64).
# prompt = "帮低音轨做个均衡实验，然后让我试听"
$PromptText = From-B64 "5biu5L2O6Z+z6L2o5YGa5Liq5Z2H6KGh5a6e6aqM77yM54S25ZCO6K6p5oiR6K+V5ZCs"
if (-not [string]::IsNullOrWhiteSpace($PromptBase64)) { $PromptText = From-B64 $PromptBase64 }
# nudge = "确认执行"
$NudgeText = From-B64 "5Y+v5Lul5omn6KGM"

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
    $params = @{ Method = $Method; Uri = $Url; TimeoutSec = $TimeoutSec; UseBasicParsing = $true }
    if ($null -ne $Body) {
        $params["Body"] = ($Body | ConvertTo-Json -Depth 10)
        $params["ContentType"] = "application/json"
    }
    $resp = Invoke-WebRequest @params
    if ([string]::IsNullOrWhiteSpace($resp.Content)) { return $null }
    return ($resp.Content | ConvertFrom-Json)
}

# Windows PowerShell 5.1 encodes a string -Body as Latin-1, which turns Chinese
# prompts into "?" before they ever reach the agent (observed 2026-09-13: the
# entry classifier answered "User request is unreadable (only question marks)").
# Every JSON body therefore rides UTF-8 bytes with an explicit charset, exactly
# like scripts/cont_stall_repro_smoke.ps1.
function Invoke-JsonUtf8 {
    param([string]$Method, [string]$Url, $Body, [int]$TimeoutSec = 60)
    if ($null -ne $Body) {
        $json = $Body | ConvertTo-Json -Depth 10 -Compress
        return Invoke-RestMethod -Method $Method -Uri $Url -ContentType "application/json; charset=utf-8" -Body ([System.Text.Encoding]::UTF8.GetBytes($json)) -TimeoutSec $TimeoutSec
    }
    return Invoke-RestMethod -Method $Method -Uri $Url -TimeoutSec $TimeoutSec
}

# The durable runtime state lives under <workspace>/.vit_history/<uuid>/state/.
# VIT_HISTORY_DRAFT_ROOT redirects the workspace, so both the isolated draft root
# and the kernel workspace are searched; the newest file wins.
function Find-RuntimeState {
    param([string]$DraftRoot, [string]$Repo)
    $roots = New-Object System.Collections.Generic.List[string]
    if (-not [string]::IsNullOrWhiteSpace($DraftRoot)) { $roots.Add($DraftRoot) }
    $roots.Add((Join-Path $Repo "VitApp\Workspace"))
    $found = New-Object System.Collections.Generic.List[object]
    foreach ($root in $roots) {
        if (-not (Test-Path -LiteralPath $root)) { continue }
        foreach ($item in @(Get-ChildItem -LiteralPath $root -Recurse -Filter "agent_runtime_state.json" -File -ErrorAction SilentlyContinue)) {
            $found.Add($item)
        }
    }
    if ($found.Count -eq 0) { return $null }
    return ($found | Sort-Object LastWriteTimeUtc -Descending | Select-Object -First 1)
}

# The runtime state carries UTF-8 (the intent text and track names are inside
# it). Windows PowerShell 5.1 decodes a bare "Get-Content -Raw" as ANSI, which
# corrupts the multibyte text into INVALID JSON -- and ConvertFrom-Json then
# fails with a NON-terminating error that a plain try/catch does not catch
# (observed 2026-09-13: a run whose frontier really held four masking candidates
# was reported as "timeout", and the parser echoed the whole file into the log).
# The read is therefore explicit UTF-8 plus -ErrorAction Stop.
function Read-JsonState {
    param([string]$Path)
    if ([string]::IsNullOrWhiteSpace($Path)) { return $null }
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) { return $null }
    try {
        $text = [System.IO.File]::ReadAllText($Path, [System.Text.Encoding]::UTF8)
        if ([string]::IsNullOrWhiteSpace($text)) { return $null }
        return ($text | ConvertFrom-Json -ErrorAction Stop)
    }
    catch { return $null }
}

# Property walk that works for both PSCustomObject maps and arrays.
function Get-NamedChildren {
    param($Node)
    $out = New-Object System.Collections.Generic.List[object]
    if ($null -eq $Node) { return $out }
    if ($Node -is [System.Management.Automation.PSCustomObject]) {
        foreach ($prop in $Node.PSObject.Properties) { $out.Add([pscustomobject]@{ name = $prop.Name; value = $prop.Value }) }
        return $out
    }
    if ($Node -is [System.Collections.IEnumerable] -and -not ($Node -is [string])) {
        $index = 0
        foreach ($item in $Node) { $out.Add([pscustomobject]@{ name = [string]$index; value = $item }); $index++ }
    }
    return $out
}

function Get-RowCount {
    param($Node)
    if ($null -eq $Node) { return 0 }
    if ($Node -is [string]) { return 0 }
    $n = 0
    foreach ($item in $Node) { $n++ }
    return $n
}

# Closure frontier rows: candidate count + the candidate view ids.
function Get-FrontierRows {
    param($State)
    $rows = New-Object System.Collections.Generic.List[object]
    foreach ($closure in (Get-NamedChildren $State.minimal_audio_closures)) {
        $frontier = $closure.value.hypothesis_frontier
        $candidates = New-Object System.Collections.Generic.List[object]
        foreach ($candidate in (Get-NamedChildren $frontier.candidates)) { $candidates.Add($candidate.value) }
        $views = New-Object System.Collections.Generic.List[string]
        foreach ($candidate in $candidates) {
            $viewID = [string]$candidate.view_id
            if (-not [string]::IsNullOrWhiteSpace($viewID) -and -not $views.Contains($viewID)) { $views.Add($viewID) }
        }
        $rows.Add([pscustomobject]@{
            closure_id         = [string]$closure.name
            candidate_id       = [string]$frontier.candidate_id
            candidate_count    = $candidates.Count
            candidate_view_ids = @($views.ToArray())
        })
    }
    return $rows
}

# Observation-ledger candidate-family counts per view: makes the live
# "frequency zero + masking present" shape visible without re-running forensics.
function Get-LedgerViewRows {
    param($State)
    $rows = New-Object System.Collections.Generic.List[object]
    foreach ($loop in (Get-NamedChildren $State.free_state_reasoning_loops)) {
        $available = $loop.value.observation_ledger.available_views
        foreach ($view in (Get-NamedChildren $available)) {
            $facts = $view.value.conclusion.facts
            if ($null -eq $facts) { continue }
            # Report a view when its facts CARRY a candidate key family, even
            # when that family is empty: the 912.vit shape under test is exactly
            # "frequency family present but zero, masking family present and
            # full", and dropping the zero row would hide half of it.
            $factKeys = @()
            foreach ($prop in $facts.PSObject.Properties) { $factKeys += $prop.Name }
            $hasFamily = ($factKeys -contains "conflict_candidates") -or ($factKeys -contains "band_conflict_candidates") -or ($factKeys -contains "candidates")
            if (-not $hasFamily) { continue }
            $conflict = Get-RowCount $facts.conflict_candidates
            $bandConflict = Get-RowCount $facts.band_conflict_candidates
            $masking = Get-RowCount $facts.candidates
            $rows.Add([pscustomobject]@{
                loop               = [string]$loop.name
                view_key           = [string]$view.name
                view_id            = [string]$view.value.view_id
                status             = [string]$view.value.status
                conflict_rows      = $conflict
                band_conflict_rows = $bandConflict
                masking_rows       = $masking
            })
        }
    }
    return $rows
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
$ReportPath = Join-Path $RunRoot "b13b_frontier_report.json"
$ChatPath = Join-Path $RunRoot "chat_response.json"
$RuntimePath = Join-Path $RunRoot "runtime_status.json"
$StateCopyPath = Join-Path $RunRoot "agent_runtime_state_snapshot.json"
if ([string]::IsNullOrWhiteSpace($KernelExe)) { $KernelExe = Join-Path $RepoRoot "VitApp\build\VitApp_artefacts\Release\VitApp.exe" }
$RepoDefaultProject = Join-Path $RepoRoot "VitApp\Workspace\default_project.xml"
# The user's own project must never be touched; it is fingerprinted before and
# after purely as evidence that this run stayed on its own draft copy.
$UserProjectDir = "D:\Godot\project\vit-daw-frontend\912.vit"

$prereq = New-Object System.Collections.Generic.List[string]
function Add-Prereq { param([string]$Line) $script:prereq.Add($Line); Write-Host $Line }

$kernelProcId = $null
$agentProcId = $null
$outcome = "env_failure"
$chatResponse = $null
$lastRuntime = $null
$frontierRows = @()
$ledgerRows = @()
$sliceRows = @()

try {
    Add-Prereq ("run_root=" + $RunRoot)
    Add-Prereq ("head=" + (& git -C $RepoRoot rev-parse HEAD))
    Add-Prereq ("git_status=" + ((& git -C $RepoRoot status --short) -join " ; "))

    Write-Step "Prereq: ports 7878/5555/5556 must be free"
    foreach ($port in 7878, 5555, 5556) {
        if (Get-TcpListener -Port $port) { throw ("port " + $port + " is already listening; aborting before any start (stack ownership rule)") }
        Add-Prereq ("port_free=" + $port)
    }

    if ([string]::IsNullOrWhiteSpace($AgentBinary)) { $AgentBinary = Join-Path $RunRoot "bin\VitAgent.b13b.exe" }
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
    Add-Prereq ("repo_default_project_sha256_before=" + (Get-FileHash -Algorithm SHA256 -LiteralPath $RepoDefaultProject).Hash)
    Add-Prereq ("user_project_fingerprint_before=" + (Get-ProjectFingerprint -Path $UserProjectDir))
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
    $bootstrapState = Invoke-Json -Method GET -Url ($base + "/agent/state") -Body $null -TimeoutSec 30
    Add-Prereq ("s1_shadow_initialized=" + $bootstrapState.shadow.initialized + " track_count=" + $bootstrapState.shadow.track_count)
    if ($bootstrapState.shadow.initialized -ne $true) { throw "shadow not initialized after stack start" }

    if ($KernelDwellSeconds -gt 0) {
        Write-Step ("Kernel warm-up dwell " + $KernelDwellSeconds + "s")
        Start-Sleep -Seconds $KernelDwellSeconds
    }

    Write-Step "Full project access (auto-apply, no confirmation card)"
    $auth = Invoke-JsonUtf8 -Method POST -Url ($base + "/agent/authority") -Body @{ authority_mode = "full_project_access" } -TimeoutSec 30
    Add-Prereq ("authority_mode=" + $auth.authority_mode)
    if ($auth.authority_mode -ne "full_project_access") { throw ("authority switch refused: " + ($auth | ConvertTo-Json -Compress)) }

    $conversationID = "b13b_" + (Get-Date -Format "yyyyMMdd_HHmmss")
    Write-Step ("Drive the same-type prompt on " + $conversationID)
    $slices = New-Object System.Collections.Generic.List[object]
    $frontierSeen = $false
    $deadline = (Get-Date).AddSeconds($ChainBudgetSeconds)
    $sliceIndex = 0
    $sliceStatus = ""
    while ($sliceIndex -le $MaxNudges -and (Get-Date) -lt $deadline) {
        $message = if ($sliceIndex -eq 0) { $PromptText } else { $NudgeText }
        $sliceStarted = Get-Date
        $chatResponse = Invoke-JsonUtf8 -Method POST -Url ($base + "/agent/chat") -Body @{ conversation_id = $conversationID; message = $message } -TimeoutSec 300
        $sliceEnded = Get-Date
        $chatResponse | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath ($ChatPath + "." + $sliceIndex) -Encoding UTF8
        $sliceStatus = [string]$chatResponse.goal_status
        $sliceStop = [string]$chatResponse.stop_reason
        $slices.Add([pscustomobject]@{
            index = $sliceIndex; status = $sliceStatus; stop_reason = $sliceStop
            ms = [int](($sliceEnded - $sliceStarted).TotalMilliseconds)
            reply = [string]$chatResponse.reply
        })
        Add-Prereq ("slice_" + $sliceIndex + "_status=" + $sliceStatus + " stop=" + $sliceStop)
        Write-Ok ("slice " + $sliceIndex + ": goal_status=" + $sliceStatus + " stop=" + $sliceStop)

        # Only a working slice leaves the chain running under the scheduler
        # (waiting_continue). A clarification/confirmation boundary needs the
        # outer nudge immediately -- polling it would burn the whole budget
        # (observed 2026-09-13, first attempt).
        if ($sliceStatus -eq "waiting_continue") {
        while ((Get-Date) -lt $deadline) {
            Start-Sleep -Seconds $PollSeconds
            $stateFile = Find-RuntimeState -DraftRoot $AgentDrafts -Repo $RepoRoot
            if ($null -ne $stateFile) {
                $durable = Read-JsonState -Path $stateFile.FullName
                if ($null -ne $durable) {
                    $frontierRows = @(Get-FrontierRows -State $durable)
                    $ledgerRows = @(Get-LedgerViewRows -State $durable)
                    foreach ($row in $frontierRows) { if ($row.candidate_count -gt 0) { $frontierSeen = $true } }
                }
            }
            if ($frontierSeen) { break }
            try {
                $lastRuntime = Invoke-Json -Method GET -Url ($base + "/agent/runtime/status") -Body $null -TimeoutSec 30
                $goalStatus = [string]$lastRuntime.goal.status
                Write-Host ("poll goal=" + $goalStatus + " frontier=" + ((@($frontierRows | ForEach-Object { $_.candidate_count })) -join ","))
                if ($goalStatus -eq "completed" -or $goalStatus -eq "failed" -or $goalStatus -eq "stopped" -or $goalStatus -eq "cancelled") { break }
            }
            catch { }
        }
        }
        if ($frontierSeen) { break }
        if ($sliceStatus -eq "waiting_clarification" -or $sliceStatus -eq "waiting_confirmation") { $sliceIndex++; continue }
        break
    }

    $sliceRows = $slices.ToArray()

    # Final read: whatever the chain reached, the durable frontier is the fact.
    $stateFile = Find-RuntimeState -DraftRoot $AgentDrafts -Repo $RepoRoot
    if ($null -ne $stateFile) {
        Add-Prereq ("runtime_state=" + $stateFile.FullName)
        Copy-Item -LiteralPath $stateFile.FullName -Destination $StateCopyPath -Force
        $durable = Read-JsonState -Path $stateFile.FullName
        if ($null -ne $durable) {
            $frontierRows = @(Get-FrontierRows -State $durable)
            $ledgerRows = @(Get-LedgerViewRows -State $durable)
        } else {
            Add-Prereq "state_parse_error=unreadable"
        }
    } else {
        Add-Prereq "runtime_state=absent"
    }
    try { $lastRuntime = Invoke-Json -Method GET -Url ($base + "/agent/runtime/status") -Body $null -TimeoutSec 30 } catch { }
    if ($null -ne $lastRuntime) { $lastRuntime | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath $RuntimePath -Encoding UTF8 }

    $maxCandidates = 0
    $attributedViews = New-Object System.Collections.Generic.List[string]
    foreach ($row in $frontierRows) {
        if ($row.candidate_count -gt $maxCandidates) { $maxCandidates = $row.candidate_count }
        foreach ($viewID in $row.candidate_view_ids) { if (-not $attributedViews.Contains($viewID)) { $attributedViews.Add($viewID) } }
    }
    $verdict = ""
    if ($maxCandidates -gt 0) {
        $verdict = "frontier_established"
    } else {
        $goalStatus = ""
        if ($null -ne $lastRuntime) { $goalStatus = [string]$lastRuntime.goal.status }
        if ($goalStatus -eq "failed" -or $goalStatus -eq "stopped" -or $goalStatus -eq "cancelled") { $verdict = "chain_" + $goalStatus }
        elseif ($frontierRows.Count -eq 0) { $verdict = "timeout" }
        else { $verdict = "frontier_empty" }
    }

    $report = [ordered]@{
        schema_version          = "b13b_frontier_masking.v1"
        run_root                = $RunRoot
        conversation_id         = $conversationID
        verdict                 = $verdict
        goal_status             = if ($null -ne $lastRuntime) { $lastRuntime.goal.status } else { "" }
        chat_goal_status        = $chatResponse.goal_status
        chat_stop_reason        = $chatResponse.stop_reason
        frontier_max_candidates = $maxCandidates
        frontier_view_ids       = @($attributedViews.ToArray())
        frontier_rows           = @($frontierRows)
        ledger_view_rows        = @($ledgerRows)
        slices                  = @($sliceRows)
        prereq                  = @($prereq)
    }
    $report | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath $ReportPath -Encoding UTF8

    Write-Step ("Verdict: " + $verdict)
    Add-Prereq ("verdict=" + $verdict)
    Add-Prereq ("frontier_max_candidates=" + $maxCandidates)
    Add-Prereq ("frontier_view_ids=" + ((@($attributedViews.ToArray())) -join ","))
    foreach ($row in $ledgerRows) {
        Add-Prereq ("ledger_view=" + $row.view_id + " status=" + $row.status + " conflict=" + $row.conflict_rows + " band_conflict=" + $row.band_conflict_rows + " masking=" + $row.masking_rows)
    }
    $outcome = $verdict
}
catch {
    $detail = $_.Exception.Message
    Add-Prereq ("fatal=" + $detail)
    Add-Prereq ("fatal_type=" + $_.Exception.GetType().FullName)
    Write-Bad ("fatal: " + $detail)
    $outcome = "env_failure"
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
Write-Host ("B13B_FRONTIER_VERDICT " + $outcome)
switch ($outcome) {
    "frontier_established" { exit 0 }
    "frontier_empty" { exit 1 }
    "chain_failed" { exit 1 }
    "chain_stopped" { exit 1 }
    "chain_cancelled" { exit 1 }
    default { exit 2 }
}
