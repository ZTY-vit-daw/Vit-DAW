# M2-EXP-TIMEDYN-GAP-1 修复腿真栈验证（worktree 专用，随 port 分支）
#
# 场景=复用取证 A 轮：project.new + import_folder_as_stems 6 轨（用户同源
# stems，只读）→ 观察链 → M2 同款话术。在修复（①a 视图组装按观察请求目标轨
# 绑定证据行）之上断言三件事：
#   (1) time_dynamics 视图无 omission 且 CCB audit receipt=executed（确定性
#       invoke 腿，零 LLM）；
#   (2) stereo 证据身份正确：guitar 视图键下的 stereo 素材 track_id=目标轨
#       自身，非最后填充轨 vocals（确定性 invoke 腿）；
#   (3) M2 话术（LLM 腿）不再死于编排器入口 partial 审计——事件面无
#       "semantic progressive disclosure observation audit is not successful"
#       拒收签名，无该因的 semantic_treatment_capability_boundary 终态。
#
# AGENTS §8 声明（运行前写定，不得事后改）：
#   LLM 轮次：1 次 NL chat + ≤2 次 nudge（预计 ≤8 次模型调用：语义入口+
#   pull harness+消息环）；确定性腿零 LLM。
#   成功条件：上述 (1)(2)(3) 全部成立 + stems 源前后哈希一致 + exit 0。
#   失败分类：llm_timeout(transient，可重试不占止损)/assertion_fail(修复未
#   生效或回归)/capability_blocked(诚实终态但非本卡缺陷面)/environment_
#   interrupt(栈/端口故障，不计有效轮)。
#   止损线：同签名 assertion_fail 连续 2 次 → 停止上交，禁止原样重跑。
#
# 用法（仓库根=修复 worktree D:\Vit_DAW_wt_m2tg2）：
#   powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\m2_timedyn_fixverify.ps1
#   可选 -SkipBuild（须先确认 agent\bin\VitAgent.exe 含修复改动）

[CmdletBinding()]
param(
    [string]$RepoRoot = "",
    [string]$AgentHttp = "http://127.0.0.1:7878",
    [string]$StemsFolder = "D:\Vit_DAW\temp\semantic-processor-agent-project-smoke-v1\fixtures\semantic_processor_project_smoke_v1_02be6193fbc17bed\cases\spv1_p06\stems",
    [int]$ChatSettleSeconds = 600,
    [int]$MaxNudges = 2,
    [int]$EventsLimit = 500,
    [int]$WaitSeconds = 90,
    [switch]$SkipBuild,
    [switch]$DeterministicOnly
)

$ErrorActionPreference = "Stop"
if ([string]::IsNullOrWhiteSpace($RepoRoot)) { $RepoRoot = (Get-Location).Path }
$RepoRoot = (Resolve-Path -LiteralPath $RepoRoot).Path
$RunStamp = Get-Date -Format "yyyyMMdd_HHmmss"
$RunRoot = Join-Path $RepoRoot ("coord\runs\M2-EXP-TIMEDYN-GAP-1\m2tg2_" + $RunStamp)
New-Item -ItemType Directory -Force -Path $RunRoot | Out-Null
$TelemetryFile = Join-Path $RunRoot "agent_llm_telemetry.jsonl"
$report = [ordered]@{
    card = "M2-EXP-TIMEDYN-GAP-1"
    leg = "fix-verify"
    run_id = $RunStamp
    repo_head = (git -C $RepoRoot rev-parse HEAD 2>$null | Out-String).Trim()
    branch = (git -C $RepoRoot rev-parse --abbrev-ref HEAD 2>$null | Out-String).Trim()
    stems_folder = $StemsFolder
    started_at = (Get-Date).ToString("o")
    sec8_decl = "1 NL chat + <=2 nudges (<=8 LLM calls expected); deterministic legs zero LLM; success=(1) no omission+receipt executed (2) stereo identity=own track (3) no entry-audit partial death; failure classes llm_timeout/assertion_fail/capability_blocked/environment_interrupt; stop-loss 2 same-signature assertion_fail"
}
$assertFailures = New-Object System.Collections.Generic.List[string]

function Write-Step([string]$m) { Write-Host ("[m2tg2] " + $m) -ForegroundColor Cyan }
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
        tool = $Tool; args = $ToolArgs; confirmed = $true; source = "m2_timedyn_fixverify"
    } -TimeoutSec $TimeoutSec
}
function Assert-True([bool]$cond, [string]$label, [string]$detail) {
    if ($cond) { Write-Ok ($label + " :: " + $detail); return $true }
    Write-Fail ($label + " :: " + $detail); $assertFailures.Add($label + " :: " + $detail); return $false
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
Write-Step "Phase 1: stack bring-up (dev_agent_smoke -StartKernel, no UI, worktree build)"
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

# ---------------- phase A: import user stems (read-only source)
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
Start-Sleep -Seconds 30   # L3 background warm window (6x20s stems), same as forensics A round

# resolve guitar track (the user-journey target) from project.state
$pstate = Invoke-Tool "project.state" @{} 60
Save-Json $pstate "a_project_state.json"
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
$guitar = $null
foreach ($t in $trackRows) {
    $name = (@([string]$t.name, [string]$t.track_name) -join "")
    if ($name -match "guitar") { $guitar = $t; break }
}
if ($null -eq $guitar) { throw "guitar track not resolvable from project.state" }
$gTrackId = [string]$guitar.track_id
if ([string]::IsNullOrWhiteSpace($gTrackId)) { $gTrackId = [string]$guitar.id }
$report.guitar_track_id = $gTrackId
Write-Ok ("guitar target track_id=" + $gTrackId)

# ---------------- assertion leg 1+2: deterministic CCB observation (zero LLM)
Write-Step "Phase D: deterministic ccb_observation_request on guitar (zero LLM)"
$dObs = Invoke-Tool "ccb_observation_request" @{
    target_ref = @{ id = $gTrackId; kind = "track" }
    view_ids = @("track.stereo_space", "track.time_dynamics")
} 180
Save-Json $dObs "d_ccb_obs_track.json"
$dBundle = $dObs.result.bundle
if ($null -eq $dBundle) { $dBundle = $dObs.result.result.bundle }
$dSum = @{
    bundle_status = [string]$dBundle.status
    omissions = $dBundle.omissions
    omission_reasons = $dBundle.omission_reasons
    limitations = $dBundle.limitations
    audit_receipt_status = [string]$dBundle.audit_receipt.status
    requested_views = $dBundle.requested_views
}
try { $dSum.time_dynamics_view_status = [string]$dBundle.views."track.time_dynamics".status } catch { }
try { $dSum.stereo_space_view_status = [string]$dBundle.views."track.stereo_space".status } catch { }
try {
    $stereoFact = $dBundle.views."track.stereo_space".facts.("track." + $gTrackId + ".slow.stereo.summary")
    if ($null -ne $stereoFact) {
        $dSum.stereo_fact_track_id = [string]$stereoFact.track_id
        $dSum.stereo_fact_clip_id = [string]$stereoFact.clip_id
    }
} catch { }
try {
    $teFact = $dBundle.views."track.time_dynamics".facts.("track." + $gTrackId + ".slow.time_energy.summary")
    if ($null -ne $teFact) { $dSum.time_energy_rows = @($teFact.rows).Count }
} catch { }
Save-Json $dSum "d_ccb_summary.json"
Write-Ok ("D obs bundle=" + $dSum.bundle_status + " audit=" + $dSum.audit_receipt_status + " time_dynamics=" + $dSum.time_dynamics_view_status)

# (1) time_dynamics 无 omission + receipt executed
$omittedTimeDyn = ($null -ne $dSum.omissions -and $null -ne $dSum.omissions."track.time_dynamics")
[void](Assert-True (-not $omittedTimeDyn) "(1a) time_dynamics not omitted" ("omissions=" + ($dSum.omission_reasons -join "; ")))
[void](Assert-True ($dSum.time_dynamics_view_status -in @("ready", "partial")) "(1b) time_dynamics view present" ("status=" + $dSum.time_dynamics_view_status))
[void](Assert-True ($dSum.audit_receipt_status -eq "executed") "(1c) audit receipt executed" ("status=" + $dSum.audit_receipt_status))
[void](Assert-True (([int]$dSum.time_energy_rows) -gt 0) "(1d) time_energy rows disclosed" ("rows=" + $dSum.time_energy_rows))

# (2) stereo 身份=目标轨自身
[void](Assert-True ($dSum.stereo_fact_track_id -eq $gTrackId) "(2a) stereo evidence identity is own track" ("fact track_id=" + $dSum.stereo_fact_track_id + " want=" + $gTrackId))
[void](Assert-True ($dSum.stereo_fact_track_id -ne "") "(2b) stereo fact present" ("track_id field=" + $dSum.stereo_fact_track_id))

if (-not $DeterministicOnly) {
# ---------------- assertion leg 3: M2 prompt (LLM round, declared above)
Write-Step "Phase L: M2 prompt (LLM round, declared 1 chat + <=2 nudges)"
$nlPrompt = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String("5pyJ5Yeg6L2o5ZCs6LW35p2l5q+U6L6D5bmy77yM5oOz57uZ5YW25Lit5LiA5p2h5ZCI6YCC55qE6L2o5Yqg5LiA54K55re35ZON77yI6KGw5YeP55+t5Lqb77yJ77yM5LiN6KaB55So5pW05L2T5aKe55uK5oiWIEVRIOadpeino+WGs+OAguivt+WFiOinguWvn+W3peeoi++8jOWGjee7meS4gOS4quacieeVjOeahOWwj+atpeaUuei/m+W7uuiuruOAgg=="))
$nlNudge = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String("5Y+v5Lul5omn6KGM"))
$nlPrompt | Set-Content -LiteralPath (Join-Path $RunRoot "a_nl_prompt.txt") -Encoding UTF8
$conversationID = "m2tg2_" + $RunStamp
$ChatUri = $AgentHttp.TrimEnd("/") + "/agent/chat"
$EventsUri = $AgentHttp.TrimEnd("/") + "/agent/events"

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
$report.round_l_chat = @{
    conversation_id = $conversationID
    turns_sent = $turnsSent; nudges_sent = $nudgesSent
    last_stop_reason = $lastStop; last_goal_status = $lastGoal; settle_reason = $settleReason
    timeline = @($chatTimeline)
}
Write-Ok ("chat settled: " + $settleReason + " stop=" + $lastStop + " goal=" + $lastGoal)

# events: scan for the entry-audit death signature (must be absent) and for
# model-side track view bundles (each must be omission-free for time_dynamics)
Start-Sleep -Seconds 3
$events = Invoke-Json -Method GET -Uri ($EventsUri + "?conversation_id=" + [uri]::EscapeDataString($conversationID) + "&since=0&limit=" + [string]$EventsLimit) -TimeoutSec 120
Save-Json $events "a_nl_events.json"
$eventsRaw = ($events | ConvertTo-Json -Depth 40)
$entryAuditDeath = $eventsRaw.Contains("semantic progressive disclosure observation audit is not successful")
$report.entry_audit_death_signature_present = $entryAuditDeath
# every model-side time_dynamics observation in the trajectory must be omission-free
$modelTimeDynBundles = 0; $modelTimeDynOmitted = 0
foreach ($ev in @($events)) {
    $evJson = ($ev | ConvertTo-Json -Depth 40)
    if ($evJson -notmatch "track\.time_dynamics") { continue }
    try {
        $bundles = @()
        if ($null -ne $ev.payload.result.bundle) { $bundles += $ev.payload.result.bundle }
        if ($null -ne $ev.payload.result.result.bundle) { $bundles += $ev.payload.result.result.bundle }
        foreach ($b in $bundles) {
            if ($null -eq $b.requested_views -or (@($b.requested_views) -notcontains "track.time_dynamics")) { continue }
            $modelTimeDynBundles++
            if ($null -ne $b.omissions -and $null -ne $b.omissions."track.time_dynamics") { $modelTimeDynOmitted++ }
        }
    } catch { }
}
$report.model_time_dynamics_bundles = $modelTimeDynBundles
$report.model_time_dynamics_omitted = $modelTimeDynOmitted
Write-Ok ("model-side time_dynamics bundles=" + $modelTimeDynBundles + " omitted=" + $modelTimeDynOmitted)

# (3) 编排器入口门放行
[void](Assert-True (-not $entryAuditDeath) "(3a) no entry-audit partial death signature in events" ("present=" + $entryAuditDeath))
[void](Assert-True ($modelTimeDynOmitted -eq 0) "(3b) model-side time_dynamics observations omission-free" ("omitted=" + $modelTimeDynOmitted + "/" + $modelTimeDynBundles))

try {
    $rt = Invoke-Json -Method GET -Uri ($AgentHttp.TrimEnd("/") + "/agent/runtime/status") -TimeoutSec 60
    Save-Json $rt "a_runtime_status.json"
} catch { Write-Warn2 ("runtime status failed: " + $_.Exception.Message) }
try {
    $st = Invoke-Json -Method GET -Uri ($AgentHttp.TrimEnd("/") + "/agent/state") -TimeoutSec 30
    Save-Json $st "a_agent_state.json"
} catch { }
}   # end LLM leg

# stems read-only verification
$stemsManifestAfter = @(Get-ChildItem -LiteralPath $StemsFolder -File | Sort-Object Name | ForEach-Object {
    @{ name = $_.Name; len = $_.Length; sha256 = (Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash }
})
Save-Json $stemsManifestAfter "a_stems_manifest_after.json"
$stemsClean = (($stemsManifestBefore | ConvertTo-Json -Depth 4) -eq ($stemsManifestAfter | ConvertTo-Json -Depth 4))
$report.stems_readonly_intact = $stemsClean
if (-not $stemsClean) { Write-Fail "stems source manifest changed (pollution!)" } else { Write-Ok "stems source intact (read-only verified)" }

$report.assert_failures = @($assertFailures)
$report.verdict = if ($assertFailures.Count -eq 0 -and $stemsClean) { "fix_verify_pass" } else { "fix_verify_fail" }
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
    try {
        $draftEvidence = Get-ChildItem -LiteralPath $AgentDraftsDir -Recurse -File -ErrorAction SilentlyContinue |
            Where-Object { $_.Name -match "l3_feature_log|mixboard_feature_snapshot|Unsaved" } |
            Select-Object FullName, Length, LastWriteTime
        Save-Json $draftEvidence "agent_drafts_evidence_index.json"
    } catch { }
    Write-Host ("[m2tg2] run_root: " + $RunRoot)
    Write-Host ("[m2tg2] verdict: " + $report.verdict)
}
if ($report.verdict -eq "fix_verify_pass") { exit 0 } else { exit 1 }
