# M2-EXP-TIMEDYN-GAP-1 取证驱动（worktree 专用，不入 main）
#
# 目的：复刻用户路径（新建工程 + import_folder_as_stems 6 轨 → M2 主推话术）
# 并抓取 CCB 观察 bundle 的 omission/limitation 面，对照 fixture 打开路径的
# 只读观察单腿（ccb_observation_request，零 LLM）。
#
# 用法（仓库根 = 取证 worktree）：
#   powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\m2_timedyn_forensics.ps1
#   可选 -SkipBuild / -RoundBOnly / -KeepStack
#
# 工件落 coord\runs\M2-EXP-TIMEDYN-GAP-1\m2tg1_<stamp>\（每 run 新目录）。
# AGENTS §8 声明（运行前已写卡面）：A 轮 1 次 NL chat（LLM 参与，语义入口+
# 消息环，预计 ≤6 调用）；成功条件=任一终态（复现 partial 审计=归因确认 /
# 审计过门=记录复现边界）；失败分类=llm_timeout(transient)/capability_blocked
# (预期终态)/环境中断(栈故障)；止损=同签名 ≤2 次。B 轮只读 invoke 零 LLM。

[CmdletBinding()]
param(
    [string]$RepoRoot = "",
    [string]$AgentHttp = "http://127.0.0.1:7878",
    [string]$StemsFolder = "D:\Vit_DAW\temp\semantic-processor-agent-project-smoke-v1\fixtures\semantic_processor_project_smoke_v1_02be6193fbc17bed\cases\spv1_p06\stems",
    [string]$FixtureProjectDir = "D:\Vit_DAW\temp\semantic-processor-agent-project-smoke-v1\fixtures\semantic_processor_project_smoke_v1_80085263a651cf20\projects\spv1_p01",
    [int]$ChatSettleSeconds = 600,
    [int]$MaxNudges = 2,
    [int]$EventsLimit = 500,
    [int]$WaitSeconds = 90,
    [switch]$SkipBuild,
    [switch]$RoundBOnly,
    [switch]$KeepStack
)

$ErrorActionPreference = "Stop"
if ([string]::IsNullOrWhiteSpace($RepoRoot)) { $RepoRoot = (Get-Location).Path }
$RepoRoot = (Resolve-Path -LiteralPath $RepoRoot).Path
$RunStamp = Get-Date -Format "yyyyMMdd_HHmmss"
$RunRoot = Join-Path $RepoRoot ("coord\runs\M2-EXP-TIMEDYN-GAP-1\m2tg1_" + $RunStamp)
New-Item -ItemType Directory -Force -Path $RunRoot | Out-Null
$TelemetryFile = Join-Path $RunRoot "agent_llm_telemetry.jsonl"
$report = [ordered]@{
    card = "M2-EXP-TIMEDYN-GAP-1"
    run_id = $RunStamp
    repo_head = (git -C $RepoRoot rev-parse HEAD 2>$null | Out-String).Trim()
    branch = (git -C $RepoRoot rev-parse --abbrev-ref HEAD 2>$null | Out-String).Trim()
    stems_folder = $StemsFolder
    fixture_project_dir = $FixtureProjectDir
    started_at = (Get-Date).ToString("o")
    llm_round_a = "declared: 1 NL chat, <=6 calls expected, stop-loss 2 same-signature"
    llm_round_b = "declared: read-only invoke, zero LLM"
}

function Write-Step([string]$m) { Write-Host ("[m2tg1] " + $m) -ForegroundColor Cyan }
function Write-Ok([string]$m)   { Write-Host ("  ok: " + $m) -ForegroundColor Green }
function Write-Warn2([string]$m){ Write-Host ("  warn: " + $m) -ForegroundColor Yellow }
function Write-Fail([string]$m) { Write-Host ("  FAIL: " + $m) -ForegroundColor Red }
function Save-Json($o, [string]$name) {
    $o | ConvertTo-Json -Depth 40 | Set-Content -LiteralPath (Join-Path $RunRoot $name) -Encoding UTF8
}
function Invoke-Json([string]$Method, [string]$Uri, $Body, [int]$TimeoutSec) {
    if ($Method -eq "GET") {
        return Invoke-RestMethod -Method GET -Uri $Uri -TimeoutSec $TimeoutSec
    }
    $json = $Body | ConvertTo-Json -Depth 20
    return Invoke-RestMethod -Method POST -Uri $Uri -ContentType "application/json; charset=utf-8" -Body ([System.Text.Encoding]::UTF8.GetBytes($json)) -TimeoutSec $TimeoutSec
}
function Invoke-Tool([string]$Tool, $ToolArgs, [int]$TimeoutSec) {
    Invoke-Json -Method POST -Uri ($AgentHttp.TrimEnd("/") + "/agent/invoke") -Body @{
        tool = $Tool; args = $ToolArgs; confirmed = $true; source = "m2_timedyn_forensics"
    } -TimeoutSec $TimeoutSec
}
function Test-StackGone {
    $out = @()
    foreach ($p in 7878, 5555, 5556, 8787) {
        if (Get-NetTCPConnection -LocalPort $p -State Listen -ErrorAction SilentlyContinue) { $out += "port $p listening" }
    }
    foreach ($n in "VitApp", "VitAgent", "vsphub", "godot") {
        $procs = Get-Process -Name $n -ErrorAction SilentlyContinue
        if ($procs) { $out += ("process " + $n + " pid=" + ($procs.Id -join ",")) }
    }
    return $out
}
function Get-StackPids {
    $pids = @{}
    foreach ($n in "VitApp", "VitAgent") {
        $procs = Get-Process -Name $n -ErrorAction SilentlyContinue
        if ($procs) { $pids[$n] = @($procs.Id) }
    }
    return $pids
}

try {
# ---------------- phase 0: clear field
Write-Step "Phase 0: clear-field preflight"
$residual = @(Test-StackGone)
if ($residual.Count -gt 0) {
    foreach ($r in $residual) { Write-Fail $r }
    $report.verdict = "stack_occupied"; $report.environment_interrupt = ($residual -join "; ")
    $report.finished_at = (Get-Date).ToString("o"); Save-Json $report "run_report.json"; exit 2
}
Write-Ok "ports 7878/5555/5556/8787 free, no stack processes"

# ---------------- phase 1: bring up stack via dev_agent_smoke (kernel + agent, no UI)
Write-Step "Phase 1: stack bring-up (dev_agent_smoke -StartKernel, no UI)"
Set-Item -LiteralPath env:VIT_DAW_HARNESS -Value "pull"
Set-Item -LiteralPath env:VIT_AGENT_LLM_TELEMETRY_PATH -Value $TelemetryFile
$AgentDraftsDir = Join-Path $RunRoot "agent_drafts"
New-Item -ItemType Directory -Force -Path $AgentDraftsDir | Out-Null
Set-Item -LiteralPath env:VIT_HISTORY_DRAFT_ROOT -Value $AgentDraftsDir
$KernelExe = "D:\Vit_DAW\Export\staging\runtime\VitApp.exe"
if (-not (Test-Path -LiteralPath $KernelExe)) { throw ("kernel exe missing: " + $KernelExe) }
$report.kernel_exe = $KernelExe
$report.kernel_sha256 = (Get-FileHash -LiteralPath $KernelExe -Algorithm SHA256).Hash
$devSmoke = Join-Path $RepoRoot "scripts\dev_agent_smoke.ps1"
$devParams = @{ StartKernel = $true; NoChatSmoke = $true; NoStripSilenceSmoke = $true; Strict = $true; WaitSeconds = $WaitSeconds; RepoRoot = $RepoRoot; KernelExe = $KernelExe }
if ($SkipBuild) { $devParams["SkipBuild"] = $true }
$devConsoleLog = Join-Path $RunRoot "dev_agent_smoke_console.log"
& $devSmoke @devParams 2>&1 | Tee-Object -FilePath $devConsoleLog | Out-Null
if ($LASTEXITCODE -ne 0) { throw ("dev_agent_smoke bring-up failed exit=" + $LASTEXITCODE + " (see dev_agent_smoke_console.log)") }
$stackPids = Get-StackPids
$report.stack_pids = $stackPids
try { $report.agent_sha256 = (Get-FileHash -LiteralPath (Join-Path $RepoRoot "agent\bin\VitAgent.exe") -Algorithm SHA256).Hash } catch { $report.agent_sha256 = "unavailable" }
Write-Ok ("stack up pids=" + ($stackPids.GetEnumerator() | ForEach-Object { $_.Key + "=" + ($_.Value -join ",") }) -join " ")

# agent health
$state = $null
$healthDeadline = (Get-Date).AddSeconds(45)
while ((Get-Date) -lt $healthDeadline -and ($null -eq $state -or [string]$state.status -ne "ok")) {
    try { $state = Invoke-Json -Method GET -Uri ($AgentHttp.TrimEnd("/") + "/agent/state") -TimeoutSec 10 } catch { Start-Sleep -Seconds 3 }
    if ($null -eq $state) { Start-Sleep -Seconds 3 }
}
if ($null -eq $state -or [string]$state.status -ne "ok") { throw "agent health check failed" }
Write-Ok ("agent ok tool_count=" + [string]$state.tool_count)

# ---------------- phase B (fixture, read-only, zero LLM) — runs before A so the
# imported project from A does not need teardown; A's project.new resets anyway.
Write-Step "Phase B: fixture open + read-only CCB observation single leg (zero LLM)"
$FixtureIso = Join-Path $RunRoot "fixture_iso"
New-Item -ItemType Directory -Force -Path $FixtureIso | Out-Null
Copy-Item -LiteralPath $FixtureProjectDir -Destination $FixtureIso -Recurse
$fixtureVit = Get-ChildItem -LiteralPath (Join-Path $FixtureIso (Split-Path $FixtureProjectDir -Leaf)) -Filter "*.vit" | Where-Object { $_.Name -notlike "*.bak*" } | Select-Object -First 1
if ($null -eq $fixtureVit) { throw "fixture .vit not found after isolation copy" }
$opened = Invoke-Tool "project.open" @{ file_path = $fixtureVit.FullName; project_path = $fixtureVit.FullName } 120
Save-Json $opened "b_project_open.json"
Start-Sleep -Seconds 5
$pstate = Invoke-Tool "project.state" @{} 60
Save-Json $pstate "b_project_state.json"
# recursive scan for track rows (id + name), robust to result-shape drift
$script:trackRows = New-Object System.Collections.Generic.List[object]
function Find-TrackRows($o) {
    if ($o -is [System.Management.Automation.PSCustomObject]) {
        $props = $o.PSObject.Properties
        $hasId = $null -ne ($props | Where-Object { $_.Name -in @("track_id", "id") })
        $hasName = $null -ne ($props | Where-Object { $_.Name -in @("name", "track_name") })
        if ($hasId -and $hasName) { $script:trackRows.Add($o) }
        foreach ($p in $props) { Find-TrackRows $p.Value }
    }
    elseif ($o -is [System.Array]) { foreach ($v in $o) { Find-TrackRows $v } }
}
Find-TrackRows $pstate
$report.fixture_track_count = $trackRows.Count
Write-Ok ("fixture open ok, track rows found=" + $trackRows.Count)
$bTarget = $null
foreach ($t in $trackRows) {
    $name = (@([string]$t.name, [string]$t.track_name) -join "")
    if ($name -match "guitar") { $bTarget = $t; break }
}
if ($null -eq $bTarget -and $trackRows.Count -gt 0) { $bTarget = $trackRows[0] }
$bTrackId = [string]$bTarget.track_id
if ([string]::IsNullOrWhiteSpace($bTrackId)) { $bTrackId = [string]$bTarget.id }
$bTrackName = @([string]$bTarget.name, [string]$bTarget.track_name) -ne "" | Select-Object -First 1
if ([string]::IsNullOrWhiteSpace($bTrackId)) { throw "B round: no track id resolvable from project.state" }
Write-Ok ("B target track_id=" + $bTrackId + " name=" + $bTrackName)
# observation 1: project-level views (same as obs#1 in user journey)
$bObs1 = Invoke-Tool "ccb_observation_request" @{
    target_ref = @{ id = $bTrackId; kind = "track" }
    view_ids = @("project.structure", "mix.multitrack_relationship")
} 180
Save-Json $bObs1 "b_ccb_obs_project.json"
# observation 2: track-level views (same as obs#2 in user journey)
$bObs2 = Invoke-Tool "ccb_observation_request" @{
    target_ref = @{ id = $bTrackId; kind = "track" }
    view_ids = @("track.stereo_space", "track.time_dynamics")
} 180
Save-Json $bObs2 "b_ccb_obs_track.json"
function Summarize-CCB($o, [string]$label) {
    $r = @{ label = $label }
    try {
        $bundle = $o.result.bundle
        if ($null -eq $bundle) { $bundle = $o.result.result.bundle }
        $r.bundle_status = [string]$bundle.status
        $r.omissions = $bundle.omissions
        $r.omission_reasons = $bundle.omission_reasons
        $r.limitations = $bundle.limitations
        $r.audit_receipt_status = [string]$bundle.audit_receipt.status
        $r.view_ids = $bundle.requested_views
        if ($null -ne $bundle.views -and $null -ne $bundle.views."track.time_dynamics") {
            $r.time_dynamics_view_status = [string]$bundle.views."track.time_dynamics".status
        }
        if ($null -ne $bundle.views -and $null -ne $bundle.views."track.stereo_space") {
            $r.stereo_space_view_status = [string]$bundle.views."track.stereo_space".status
        }
        if ($null -ne $bundle.views -and $null -ne $bundle.views."project.structure") {
            $r.structure_view_status = [string]$bundle.views."project.structure".status
        }
    } catch { $r.parse_error = $_.Exception.Message }
    return $r
}
$report.round_b = @(
    (Summarize-CCB $bObs1 "fixture project views [project.structure, mix.multitrack_relationship]")
    (Summarize-CCB $bObs2 "fixture track views [track.stereo_space, track.time_dynamics] target=$bTrackId")
)
Save-Json $report.round_b "b_ccb_summary.json"
Write-Ok ("B obs1 status=" + $report.round_b[0].bundle_status + " audit=" + $report.round_b[0].audit_receipt_status)
Write-Ok ("B obs2 status=" + $report.round_b[1].bundle_status + " audit=" + $report.round_b[1].audit_receipt_status + " time_dynamics=" + $report.round_b[1].time_dynamics_view_status)

if (-not $RoundBOnly) {
# ---------------- phase A: replicate user journey (new project + import + M2 prompt)
Write-Step "Phase A: project.new + import_folder_as_stems (user stems, read-only source)"
$stemsManifestBefore = @(Get-ChildItem -LiteralPath $StemsFolder -File | Sort-Object Name | ForEach-Object {
    @{ name = $_.Name; len = $_.Length; sha256 = (Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash }
})
Save-Json $stemsManifestBefore "a_stems_manifest_before.json"
$null = Invoke-Tool "project.new" @{} 60
$preflight = Invoke-Tool "project.import_preflight" @{
    folder_path = $StemsFolder; recursive = $false; media_kinds = @("audio")
    intended_mode = "stems_folder"; target_policy = "create_tracks"; start_time_seconds = 0
} 120
Save-Json $preflight "a_import_preflight.json"
$imported = Invoke-Tool "project.import_folder_as_stems" @{
    folder_path = $StemsFolder; recursive = $false; target_policy = "create_tracks"
    start_time_seconds = 0.0; confirmed = $true; skip_unreadable = $false
} 600
Save-Json $imported "a_import_folder_as_stems.json"
Write-Ok ("import status=" + [string]$imported.status)

# wait for kernel L3 bake (background warm) and shadow track sync
$trackDeadline = (Get-Date).AddSeconds(120)
$trackCount = -1
while ((Get-Date) -lt $trackDeadline) {
    try {
        $st = Invoke-Json -Method GET -Uri ($AgentHttp.TrimEnd("/") + "/agent/state") -TimeoutSec 10
        $shadow = $st.shadow
        if ($null -ne $shadow) {
            $trackCount = [int]$shadow.engine_track_count
            if ($trackCount -lt 1) { $trackCount = [int]$shadow.internal_track_count }
            if ($trackCount -ge 6) { break }
        }
    } catch { }
    Start-Sleep -Seconds 5
}
$report.round_a_track_count = $trackCount
Write-Ok ("shadow track_count=" + $trackCount)
Start-Sleep -Seconds 30   # L3 background warm window (6x20s stems)

# M2 prompt (verbatim, same as user round 3 / run12-13 flavor)
$nlPrompt = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String("5pyJ5Yeg6L2o5ZCs6LW35p2l5q+U6L6D5bmy77yM5oOz57uZ5YW25Lit5LiA5p2h5ZCI6YCC55qE6L2o5Yqg5LiA54K55re35ZON77yI6KGw5YeP55+t5Lqb77yJ77yM5LiN6KaB55So5pW05L2T5aKe55uK5oiWIEVRIOadpeino+WGs+OAguivt+WFiOinguWvn+W3peeoi++8jOWGjee7meS4gOS4quacieeVjOeahOWwj+atpeaUuei/m+W7uuiuruOAgg=="))
$nlNudge = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String("5Y+v5Lul5omn6KGM"))
$nlPrompt | Set-Content -LiteralPath (Join-Path $RunRoot "a_nl_prompt.txt") -Encoding UTF8
$conversationID = "m2tg1_" + $RunStamp
$ChatUri = $AgentHttp.TrimEnd("/") + "/agent/chat"
$EventsUri = $AgentHttp.TrimEnd("/") + "/agent/events"

Write-Step "Phase A: sending M2 prompt (LLM round, declared 1 chat + <=2 nudges)"
$chatTimeline = New-Object System.Collections.Generic.List[string]
$turnsSent = 0; $nudgesSent = 0; $sendPending = $true; $terminal = $false
$lastStop = ""; $lastGoal = ""; $settleReason = "deadline"
$terminalGoals = @("completed", "failed", "stopped", "cancelled", "waiting_confirmation", "waiting_clarification")
$settleDeadline = (Get-Date).AddSeconds($ChatSettleSeconds)
while ((Get-Date) -lt $settleDeadline -and -not $terminal) {
    if ($sendPending) {
        $sendPending = $false
        $turnsSent++
        $isNudge = ($turnsSent -gt 1)
        if ($isNudge) { $nudgesSent++ }
        $msg = if ($isNudge) { $nlNudge } else { $nlPrompt }
        $turnResponse = $null
        try {
            $turnResponse = Invoke-Json -Method POST -Uri $ChatUri -Body @{
                conversation_id = $conversationID; message = $msg
                context = @{ agent_mode = "chat"; pull_observation_budget = @{ max_cycles = 6 } }
            } -TimeoutSec 300
        } catch {
            $chatTimeline.Add(("turn " + $turnsSent + " transport_error: " + $_.Exception.Message))
            if ($turnsSent -eq 1) { $settleReason = "initial_transport_error"; break }
        }
        if ($null -ne $turnResponse) {
            Save-Json $turnResponse ("a_nl_chat_" + $turnsSent + ".json")
            $lastStop = [string]$turnResponse.stop_reason
            $lastGoal = [string]$turnResponse.goal_status
            $chatTimeline.Add(("turn " + $turnsSent + $(if ($isNudge) { " (nudge)" }) + " stop=" + $lastStop + " goal=" + $lastGoal))
            if ($lastGoal -in @("completed", "failed", "stopped", "cancelled") -or $lastStop -in @("done", "failed", "cancelled", "user_stop")) { $terminal = $true; $settleReason = "goal_terminal" }
            elseif ($lastGoal -in @("waiting_confirmation", "waiting_clarification")) { $terminal = $true; $settleReason = "awaiting_user" }
            elseif ($lastStop -eq "observation_budget_exhausted") { $terminal = $true; $settleReason = "budget_exhausted" }
            elseif ($lastGoal -eq "waiting_continue" -or $lastStop -in @("limit_reached", "transient_llm_error")) {
                if ($nudgesSent -lt $MaxNudges) { $sendPending = $true } else { $terminal = $true; $settleReason = "boundary_no_nudges_left" }
            }
        }
    } else { Start-Sleep -Seconds 5 }
}
$report.round_a_chat = @{
    conversation_id = $conversationID
    turns_sent = $turnsSent; nudges_sent = $nudgesSent
    last_stop_reason = $lastStop; last_goal_status = $lastGoal; settle_reason = $settleReason
    timeline = @($chatTimeline)
}
Write-Ok ("chat settled: " + $settleReason + " stop=" + $lastStop + " goal=" + $lastGoal)

# events + final state
Start-Sleep -Seconds 3
try {
    $events = Invoke-Json -Method GET -Uri ($EventsUri + "?conversation_id=" + [uri]::EscapeDataString($conversationID) + "&since=0&limit=" + [string]$EventsLimit) -TimeoutSec 120
    Save-Json $events "a_nl_events.json"
} catch { Write-Warn2 ("events pull failed: " + $_.Exception.Message) }
try {
    $rt = Invoke-Json -Method GET -Uri ($AgentHttp.TrimEnd("/") + "/agent/runtime/status") -TimeoutSec 60
    Save-Json $rt "a_runtime_status.json"
} catch { Write-Warn2 ("runtime status failed: " + $_.Exception.Message) }
try {
    $st = Invoke-Json -Method GET -Uri ($AgentHttp.TrimEnd("/") + "/agent/state") -TimeoutSec 30
    Save-Json $st "a_agent_state.json"
} catch { }

# stems read-only verification
$stemsManifestAfter = @(Get-ChildItem -LiteralPath $StemsFolder -File | Sort-Object Name | ForEach-Object {
    @{ name = $_.Name; len = $_.Length; sha256 = (Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash }
})
Save-Json $stemsManifestAfter "a_stems_manifest_after.json"
$stemsClean = (($stemsManifestBefore | ConvertTo-Json -Depth 4) -eq ($stemsManifestAfter | ConvertTo-Json -Depth 4))
$report.stems_readonly_intact = $stemsClean
if (-not $stemsClean) { Write-Fail "stems source manifest changed (pollution!)" } else { Write-Ok "stems source intact (read-only verified)" }
}   # end RoundA

$report.verdict = "forensics_complete"
}
catch {
    $report.verdict = "script_exception"
    $report.error = $_.Exception.Message
    try { $_ | Out-String | Set-Content -LiteralPath (Join-Path $RunRoot "script_exception.txt") -Encoding UTF8 } catch { }
}
finally {
    # ---------------- teardown
    try {
        $pids = Get-StackPids
        foreach ($name in $pids.Keys) {
            foreach ($p in $pids[$name]) { try { Stop-Process -Id $p -Force -ErrorAction Stop } catch { } }
        }
        Start-Sleep -Seconds 3
        $remain = @(Test-StackGone)
        $report.teardown = @{ remaining = $remain; torn_down = ($remain.Count -eq 0) }
        if ($remain.Count -eq 0) { Write-Ok "stack torn down (ports free, processes gone)" }
        else { Write-Warn2 ("teardown residue: " + ($remain -join "; ")) }
    } catch { }
    Remove-Item -LiteralPath env:VIT_DAW_HARNESS -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath env:VIT_AGENT_LLM_TELEMETRY_PATH -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath env:VIT_HISTORY_DRAFT_ROOT -ErrorAction SilentlyContinue
    $report.finished_at = (Get-Date).ToString("o")
    $report.run_root = $RunRoot
    Save-Json $report "run_report.json"
    # collect draft tree evidence (l3 feature logs, mixboard snapshots)
    try {
        $draftEvidence = Get-ChildItem -LiteralPath $AgentDraftsDir -Recurse -File -ErrorAction SilentlyContinue |
            Where-Object { $_.Name -match "l3_feature_log|mixboard_feature_snapshot|Unsaved" } |
            Select-Object FullName, Length, LastWriteTime
        Save-Json $draftEvidence "agent_drafts_evidence_index.json"
    } catch { }
    Write-Host ("[m2tg1] run_root: " + $RunRoot)
}
exit 0
