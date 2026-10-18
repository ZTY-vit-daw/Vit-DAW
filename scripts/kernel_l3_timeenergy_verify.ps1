# KERNEL-L3-TIMEENERGY-1 真栈验证（worktree 专用，随 port 分支）
#
# 场景=同构 m2_timedyn_fixverify.ps1 取证 A 轮（project.new + import_folder_as_stems
# 6 轨用户同源 stems，只读）→ L3 后台 warm → 两腿断言（全确定性，零 LLM）：
#   (1) 落卷腿（卡验收①）：l3_feature_log.jsonl 中 6 轨各含 time_energy 条目，
#       且 band/stereo/loudness 基线三族同轨仍在卷（对照取证腿 18 条基线=增量不回退）；
#       time_energy 行结构完整（feature_version/method/window_ms/segments）；
#   (2) 读侧回归腿（卡验收②=①a 零回退）：ccb_observation_request（确定性 invoke）
#       time_dynamics 视图 present（ready/partial）+ 素材身份=目标轨自身 +
#       audit receipt=executed + time_energy rows disclosed>0（TrackWaveformEnvelopes
#       绑定不受本卡影响）。
#
# AGENTS §8 声明（运行前写定，不得事后改）：
#   LLM 轮次：0（全确定性腿，零 LLM）。
#   成功条件：(1)+(2) 全部成立 + stems 源前后哈希一致 + exit 0。
#   失败分类：assertion_fail(生产者/落卷门/回归缺陷)/environment_interrupt(栈端口
#   故障，不计有效轮)/stack_occupied(预检拦阻)。
#   止损线：同签名 assertion_fail 连续 2 次 → 停止上交，禁止原样重跑。
#
# 用法（仓库根=实现 worktree D:\Vit_DAW_wt_klte1）：
#   powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\kernel_l3_timeenergy_verify.ps1
#   可选 -SkipBuild（须先确认 agent\bin\VitAgent.exe 含本卡 agent 侧改动）

[CmdletBinding()]
param(
    [string]$RepoRoot = "",
    [string]$AgentHttp = "http://127.0.0.1:7878",
    [string]$StemsFolder = "D:\Vit_DAW\temp\semantic-processor-agent-project-smoke-v1\fixtures\semantic_processor_project_smoke_v1_02be6193fbc17bed\cases\spv1_p06\stems",
    [string]$KernelExe = "",
    [int]$L3WaitSeconds = 300,
    [int]$WaitSeconds = 90,
    [switch]$SkipBuild
)

$ErrorActionPreference = "Stop"
if ([string]::IsNullOrWhiteSpace($RepoRoot)) { $RepoRoot = (Get-Location).Path }
$RepoRoot = (Resolve-Path -LiteralPath $RepoRoot).Path
if ([string]::IsNullOrWhiteSpace($KernelExe)) {
    $KernelExe = Join-Path $RepoRoot "VitApp\build-klte1\VitApp_artefacts\Release\VitApp.exe"
}
$RunStamp = Get-Date -Format "yyyyMMdd_HHmmss"
$RunRoot = Join-Path $RepoRoot ("coord\runs\KERNEL-L3-TIMEENERGY-1\klte1_" + $RunStamp)
New-Item -ItemType Directory -Force -Path $RunRoot | Out-Null
$TelemetryFile = Join-Path $RunRoot "agent_llm_telemetry.jsonl"
$report = [ordered]@{
    card = "KERNEL-L3-TIMEENERGY-1"
    leg = "real-stack verify (deterministic, zero LLM)"
    run_id = $RunStamp
    repo_head = (git -C $RepoRoot rev-parse HEAD 2>$null | Out-String).Trim()
    branch = (git -C $RepoRoot rev-parse --abbrev-ref HEAD 2>$null | Out-String).Trim()
    stems_folder = $StemsFolder
    kernel_exe = $KernelExe
    started_at = (Get-Date).ToString("o")
    sec8_decl = "0 LLM rounds; success=(1) 6 tracks x 4 families in l3_feature_log with well-formed time_energy rows (2) time_dynamics present + own-track identity + receipt executed + rows>0; failure classes assertion_fail/environment_interrupt/stack_occupied; stop-loss 2 same-signature assertion_fail"
}
$assertFailures = New-Object System.Collections.Generic.List[string]

function Write-Step([string]$m) { Write-Host ("[klte1] " + $m) -ForegroundColor Cyan }
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
        tool = $Tool; args = $ToolArgs; confirmed = $true; source = "kernel_l3_timeenergy_verify"
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
function Read-L3FeatureRows([string]$draftsDir) {
    $logs = @(Get-ChildItem -LiteralPath $draftsDir -Recurse -Filter "l3_feature_log.jsonl" -File -ErrorAction SilentlyContinue |
        Sort-Object LastWriteTime -Descending)
    if ($logs.Count -eq 0) { return @{ rows = @(); log = $null } }
    $log = $logs[0]
    $rows = @()
    foreach ($line in (Get-Content -LiteralPath $log.FullName -Encoding UTF8)) {
        if ([string]::IsNullOrWhiteSpace($line)) { continue }
        try { $rows += ($line | ConvertFrom-Json) } catch { }
    }
    return @{ rows = $rows; log = $log }
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
if (-not (Test-Path -LiteralPath $KernelExe)) { throw ("kernel exe missing: " + $KernelExe) }
Write-Ok "ports free, no stack processes; kernel exe present"

# ---------------- phase 1: bring up stack via dev_agent_smoke (kernel + agent, no UI)
Write-Step "Phase 1: stack bring-up (dev_agent_smoke -StartKernel, no UI, worktree build)"
Set-Item -LiteralPath env:VIT_DAW_HARNESS -Value "pull"
Set-Item -LiteralPath env:VIT_AGENT_LLM_TELEMETRY_PATH -Value $TelemetryFile
$AgentDraftsDir = Join-Path $RunRoot "agent_drafts"
New-Item -ItemType Directory -Force -Path $AgentDraftsDir | Out-Null
Set-Item -LiteralPath env:VIT_HISTORY_DRAFT_ROOT -Value $AgentDraftsDir
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
Write-Ok ("stack up pids=" + (($stackPids.GetEnumerator() | ForEach-Object { $_.Key + "=" + ($_.Value -join ",") }) -join " "))

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

# ---------------- phase B (leg 1): poll l3_feature_log until 6 tracks x 4 families
Write-Step "Phase B: poll l3_feature_log for 6 tracks x 4 families (time_energy increment)"
$rows = @(); $logFile = $null; $familyByTrack = @{}
$deadline = (Get-Date).AddSeconds($L3WaitSeconds)
while ((Get-Date) -lt $deadline) {
    $read = Read-L3FeatureRows $AgentDraftsDir
    $rows = @($read.rows); $logFile = $read.log
    $familyByTrack = @{}
    foreach ($row in $rows) {
        $ft = [string]$row.feature_type
        if ($ft -notin @("time_energy", "band_energy_summary", "stereo_relation_summary", "loudness_summary")) { continue }
        $tid = [string]$row.track_id
        if ([string]::IsNullOrWhiteSpace($tid)) { continue }
        if (-not $familyByTrack.ContainsKey($tid)) { $familyByTrack[$tid] = @{} }
        $familyByTrack[$tid][$ft] = $true
    }
    $complete = @($familyByTrack.Keys | Where-Object { $familyByTrack[$_].Count -ge 4 })
    if ($complete.Count -ge 6) { break }
    Start-Sleep -Seconds 10
}
$report.l3_log_path = if ($null -ne $logFile) { $logFile.FullName } else { "" }
$report.l3_row_count = @($rows).Count

$teTracks = @($familyByTrack.Keys | Where-Object { $familyByTrack[$_].ContainsKey("time_energy") })
$fourFamilyTracks = @($familyByTrack.Keys | Where-Object { $familyByTrack[$_].Count -ge 4 })
$baselineTracks = @($familyByTrack.Keys | Where-Object {
    ($familyByTrack[$_].ContainsKey("band_energy_summary")) -and
    ($familyByTrack[$_].ContainsKey("stereo_relation_summary")) -and
    ($familyByTrack[$_].ContainsKey("loudness_summary")) })
Save-Json (@{
    track_families = ($familyByTrack.GetEnumerator() | ForEach-Object { @{ track_id = $_.Key; families = @($_.Value.Keys) } })
    row_count = @($rows).Count
    log_path = $report.l3_log_path
}) "b_l3_family_summary.json"

[void](Assert-True ($null -ne $logFile) "(1a) l3_feature_log present" ("path=" + $report.l3_log_path))
[void](Assert-True ($teTracks.Count -ge 6) "(1b) time_energy per track" ("tracks_with_time_energy=" + $teTracks.Count + " of " + $familyByTrack.Count))
[void](Assert-True ($fourFamilyTracks.Count -ge 6) "(1c) all four families per track" ("complete_tracks=" + $fourFamilyTracks.Count))
[void](Assert-True ($baselineTracks.Count -ge 6) "(1d) baseline three families intact" ("baseline_tracks=" + $baselineTracks.Count))

# time_energy row structure checks (first ready row)
$teRow = @($rows | Where-Object { [string]$_.feature_type -eq "time_energy" -and $_.track_id -in $teTracks } | Select-Object -First 1)
if ($teRow.Count -gt 0) {
    $r = $teRow[0]
    $segCount = 0
    try { $segCount = @($r.energy_envelope.segments).Count } catch { }
    $report.time_energy_sample = @{
        track_id = [string]$r.track_id; status = [string]$r.status
        feature_version = [string]$r.feature_version; layer = [string]$r.layer
        method = [string]$r.energy_envelope.method
        window_ms = [string]$r.energy_envelope.window_ms
        hop_ms = [string]$r.energy_envelope.hop_ms
        segment_count = $segCount
        truncated = [string]$r.energy_envelope.truncated
        integrated_rms_dbfs = [string]$r.integrated_rms_dbfs
    }
    [void](Assert-True ([string]$r.feature_version -eq "time_energy.v1") "(1e) time_energy feature_version" ("got=" + [string]$r.feature_version))
    [void](Assert-True ([string]$r.energy_envelope.method -eq "time_domain_frame_rms_envelope_v1") "(1f) envelope method key" ("got=" + [string]$r.energy_envelope.method))
    [void](Assert-True (([double]$r.energy_envelope.window_ms) -gt 0.0) "(1g) window_ms declared" ("window_ms=" + [string]$r.energy_envelope.window_ms))
    [void](Assert-True ($segCount -gt 0) "(1h) envelope segments non-empty" ("segments=" + $segCount))
} else {
    [void](Assert-True $false "(1e) time_energy row inspectable" "no time_energy row found for structural check")
}

# ---------------- phase D (leg 2): deterministic CCB observation regression (zero LLM)
Write-Step "Phase D: deterministic ccb_observation_request on guitar (zero LLM, 1a regression)"
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

$omittedTimeDyn = ($null -ne $dSum.omissions -and $null -ne $dSum.omissions."track.time_dynamics")
[void](Assert-True (-not $omittedTimeDyn) "(2a) time_dynamics not omitted" ("omissions=" + ($dSum.omission_reasons -join "; ")))
[void](Assert-True ($dSum.time_dynamics_view_status -in @("ready", "partial")) "(2b) time_dynamics view present" ("status=" + $dSum.time_dynamics_view_status))
[void](Assert-True ($dSum.audit_receipt_status -eq "executed") "(2c) audit receipt executed" ("status=" + $dSum.audit_receipt_status))
[void](Assert-True (([int]$dSum.time_energy_rows) -gt 0) "(2d) time_energy rows disclosed (1a binding intact)" ("rows=" + $dSum.time_energy_rows))
[void](Assert-True ($dSum.stereo_fact_track_id -eq $gTrackId) "(2e) stereo evidence identity is own track" ("fact track_id=" + $dSum.stereo_fact_track_id + " want=" + $gTrackId))

# stems read-only verification
$stemsManifestAfter = @(Get-ChildItem -LiteralPath $StemsFolder -File | Sort-Object Name | ForEach-Object {
    @{ name = $_.Name; len = $_.Length; sha256 = (Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash }
})
Save-Json $stemsManifestAfter "a_stems_manifest_after.json"
$stemsClean = (($stemsManifestBefore | ConvertTo-Json -Depth 4) -eq ($stemsManifestAfter | ConvertTo-Json -Depth 4))
$report.stems_readonly_intact = $stemsClean
if (-not $stemsClean) { Write-Fail "stems source manifest changed (pollution!)" } else { Write-Ok "stems source intact (read-only verified)" }

$report.assert_failures = @($assertFailures)
$report.verdict = if ($assertFailures.Count -eq 0 -and $stemsClean) { "timeenergy_verify_pass" } else { "timeenergy_verify_fail" }
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
        $logCopy = Join-Path $RunRoot "l3_feature_log_copy.jsonl"
        if ($null -ne $logFile -and (Test-Path -LiteralPath $logFile.FullName)) {
            Copy-Item -LiteralPath $logFile.FullName -Destination $logCopy -Force
        }
    } catch { }
    Write-Host ("[klte1] run_root: " + $RunRoot)
    Write-Host ("[klte1] verdict: " + $report.verdict)
}
if ($report.verdict -eq "timeenergy_verify_pass") { exit 0 } else { exit 1 }
