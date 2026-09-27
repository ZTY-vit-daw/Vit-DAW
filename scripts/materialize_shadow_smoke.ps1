<#
MATERIALIZATION shadow-mode real-stack smoke (cards 2026-09-28-MAT-D /
2026-09-29-MAT-D4 / 2026-09-29-MAT-E0 same-source reconciliation,
design docs/MATERIALIZATION_V1_DESIGN.md section 5.5 G2-D,
same pattern as tim_assert_smoke.ps1).

Proves the shadow state of the three-state materialization flag end to end on
the real three-piece stack (VitApp kernel + Godot UI + Go agent), read-only on
the metrics/log surface (shadow promises the read side never consumes):

  1. bring the stack up by reusing dev_agent_smoke.ps1 -StartKernel -StartUI
     with VIT_DAW_MATERIALIZATION=shadow set for the agent process and
     -KernelExe pointing at the pcverify1 incremental build (staging is never
     touched);
  2. fixture: temporary track + modest sine wav (-12 dBFS) imported via
     clip.import_media_to_track so the feature snapshot gains a real row;
  3. observation rounds targeting the fixture track drive the shadow
     materialization round (RecomputeLazy + fxm/com registration + dom
     reconcile) and each round appends a [materialize] shadow_round metrics
     line to the agent log;
  4. assertion groups (all must pass for exit 0):
     S1 flag=shadow started and is alive on the real stack: agent healthy AND
        a "[materialize] mode=shadow" startup line in the agent log (the off
        lock-in之外 third state);
     S2 (MAT-E0 same-source reconciliation, per-round; the G2 ruling restores
        observation reconciliation as the regression gate once the
        materialization input reads the full-state evidence blob):
        - baseline rounds: at least one shadow_round line, non-vacuous via
          reconcile_rows>0 on some line (metrics are cumulative counters);
        - change round (post set_volume observation): with the materialization
          input switched to the full-state evidence read-back (State A), the
          finalize generation and the tail-hook generation are same-source,
          so the generation-gap divergence is structurally gone: zero NEW
          divergences AND zero NEW timing exclusions AND at least one real
          compared row (reconcile_rows delta >= 1);
        - convergence round (third observation after a telemetry silence
          window of >=8s + log-line stability): identical zero-delta gates
          (G2-D closes at the converged state, not instantaneously).
     S3 dirty propagation happens on the real event stream: after one
        set_volume change event on the fixture track a "[materialize] receipt"
        line appears (event-time evidence) and a subsequent shadow_round line
        reports invalidations>0 (kind marked dirty);
     S4 exit 0 (this script's own exit code).

Usage:
  powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\materialize_shadow_smoke.ps1
  powershell ... -KernelExe <path>   # default: pcverify1 incremental build
  powershell ... -ReuseStack         # stack already running (kernel+UI+agent)
  powershell ... -SkipBuild          # reuse the installed agent binary

Exit codes: 0 = PASS, 1 = assertion/environment failure, 2 = stack bring-up
failure. Artifacts land under coord\runs\MAT-D4-1\ (new dir per run).
#>

[CmdletBinding()]
param(
    [string]$RepoRoot = "",
    [string]$AgentHttp = "http://127.0.0.1:7878",
    [string]$KernelExe = "",
    [int]$WaitSeconds = 20,
    [int]$ObserveTimeoutSeconds = 180,
    [int]$PollIntervalSeconds = 5,
    [switch]$SkipBuild,
    [switch]$ReuseStack
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

function Resolve-RepoRoot {
    param([string]$Explicit)
    if (-not [string]::IsNullOrWhiteSpace($Explicit)) {
        return (Resolve-Path -LiteralPath $Explicit).Path
    }
    return (Split-Path -Parent (Split-Path -Parent $MyInvocation.ScriptName))
}

function Write-Step { param([string]$Message) Write-Host ("== " + $Message + " ==") -ForegroundColor Cyan }
function Write-Ok { param([string]$Message) Write-Host ("   [ok] " + $Message) -ForegroundColor Green }
function Write-WarnLine { param([string]$Message) Write-Host ("   [warn] " + $Message) -ForegroundColor Yellow }
function Write-FailLine { param([string]$Message) Write-Host ("   [FAIL] " + $Message) -ForegroundColor Red }

# SMOKE-TOOLING-1 (BLIND-BOM-1 family): write artifacts as UTF-8 WITHOUT BOM.
function Write-Utf8NoBom {
    param([string]$Path, [string]$Text)
    [System.IO.File]::WriteAllText($Path, $Text, [System.Text.UTF8Encoding]::new($false))
}

function Invoke-Json {
    [CmdletBinding()]
    param([string]$Method, [string]$Uri, $Body, [int]$TimeoutSec = 60)
    if ($null -ne $Body) {
        $json = $Body | ConvertTo-Json -Depth 10 -Compress
        return Invoke-RestMethod -Method $Method -Uri $Uri -ContentType "application/json; charset=utf-8" -Body ([System.Text.Encoding]::UTF8.GetBytes($json)) -TimeoutSec $TimeoutSec
    }
    return Invoke-RestMethod -Method $Method -Uri $Uri -TimeoutSec $TimeoutSec
}

function Invoke-AgentTool {
    [CmdletBinding()]
    param([string]$Tool, [hashtable]$ToolArgs, [int]$TimeoutSec = 60, [string]$Source = "materialize_shadow_smoke", [switch]$Confirmed)
    $body = @{
        tool = $Tool
        args = $ToolArgs
        source = $Source
    }
    if ($Confirmed) { $body["confirmed"] = $true }
    return Invoke-Json -Method POST -Uri ($AgentHttp.TrimEnd("/") + "/agent/invoke") -Body $body -TimeoutSec $TimeoutSec
}

function Get-OptionalProperty {
    param($Object, [string]$Name)
    if ($null -eq $Object) { return $null }
    $adapted = $Object -as [System.Management.Automation.PSObject]
    if ($null -ne $adapted) {
        foreach ($prop in $adapted.PSObject.Properties) {
            if ($prop.Name -eq $Name) { return $prop.Value }
        }
    }
    return $null
}

function Get-FirstPropertyValue {
    param($Object, [string[]]$Names)
    foreach ($name in $Names) {
        $value = Get-OptionalProperty -Object $Object -Name $name
        if ($null -ne $value -and -not [string]::IsNullOrWhiteSpace([string]$value)) {
            return $value
        }
    }
    return $null
}

function Read-LogLines {
    param([string]$Path)
    if (-not (Test-Path -LiteralPath $Path)) { return @() }
    try {
        $stream = [System.IO.File]::Open($Path, [System.IO.FileMode]::Open, [System.IO.FileAccess]::Read, [System.IO.FileShare]::ReadWrite)
        try {
            $reader = New-Object System.IO.StreamReader($stream, [System.Text.Encoding]::UTF8)
            try {
                $lines = New-Object System.Collections.Generic.List[string]
                while ($null -ne ($line = $reader.ReadLine())) { $lines.Add($line) }
                return $lines
            }
            finally { $reader.Close() }
        }
        finally { $stream.Close() }
    }
    catch { return @() }
}

# Get-ShadowMetric 从 shadow_round metrics 行取累计计数字段（MAT-D4 分轮断言：
# metrics 是累计计数，轮间 delta 才是本轮值）。行空或字段缺失返回 -1。
function Get-ShadowMetric {
    param([string]$Line, [string]$Field)
    if ([string]::IsNullOrEmpty($Line)) { return -1 }
    $m = [regex]::Match($Line, ($Field + "=(\d+)"))
    if ($m.Success) { return [int]$m.Groups[1].Value }
    return -1
}

# Read-ShadowRoundMetrics 把一行 shadow_round 的分轮断言面字段抽成有序表。
function Read-ShadowRoundMetrics {
    param([string]$Line)
    return [ordered]@{
        line = $Line
        shadow_divergences = Get-ShadowMetric -Line $Line -Field "shadow_divergences"
        reconcile_rows = Get-ShadowMetric -Line $Line -Field "reconcile_rows"
        measurement_carried_excluded = Get-ShadowMetric -Line $Line -Field "measurement_carried_excluded"
        timing_carried_excluded = Get-ShadowMetric -Line $Line -Field "timing_carried_excluded"
    }
}

# Wait-NextShadowRoundLine 等待下一条未见过的 shadow_round 行（对账在观察请求
# 尾挂同步发生，行应在请求返回后数秒内出现）。返回新行；预算耗尽返回空串。
# 命中的新行同步补进收集列表（后续报告/取证共用）。
function Wait-NextShadowRoundLine {
    param([System.Collections.Generic.List[string]]$Collected, [int]$BudgetSeconds)
    $deadline = (Get-Date).AddSeconds($BudgetSeconds)
    while ((Get-Date) -lt $deadline) {
        Start-Sleep -Seconds 2
        $lines = @(Read-LogLines -Path $AgentLog | Where-Object { $_.Contains("[materialize] shadow_round") })
        foreach ($line in $lines) {
            if (-not $Collected.Contains($line)) {
                $Collected.Add($line)
                return $line
            }
        }
    }
    return ""
}

# Modest sine wav: 440 Hz, -12 dBFS (amplitude 0.25), mono 16-bit. The fixture
# only needs a feature snapshot row for the new track, no assertion hinges on
# the level itself (unlike tim_assert_smoke's near-full-scale fixture).
function Write-TestWav {
    param([string]$Path, [int]$SampleRate)
    $dir = Split-Path -Parent $Path
    New-Item -ItemType Directory -Path $dir -Force | Out-Null
    $durationSeconds = 2.0
    $samples = [int]($SampleRate * $durationSeconds)
    $channels = 1
    $bitsPerSample = 16
    $blockAlign = [int]($channels * $bitsPerSample / 8)
    $byteRate = [int]($SampleRate * $blockAlign)
    $dataBytes = [int]($samples * $blockAlign)
    $writer = [System.IO.BinaryWriter]::new([System.IO.File]::Open($Path, [System.IO.FileMode]::Create, [System.IO.FileAccess]::Write))
    try {
        $ascii = [System.Text.Encoding]::ASCII
        $writer.Write($ascii.GetBytes("RIFF"))
        $writer.Write([int](36 + $dataBytes))
        $writer.Write($ascii.GetBytes("WAVE"))
        $writer.Write($ascii.GetBytes("fmt "))
        $writer.Write([int]16)
        $writer.Write([int16]1)
        $writer.Write([int16]$channels)
        $writer.Write([int]$SampleRate)
        $writer.Write([int]$byteRate)
        $writer.Write([int16]$blockAlign)
        $writer.Write([int16]$bitsPerSample)
        $writer.Write($ascii.GetBytes("data"))
        $writer.Write([int]$dataBytes)
        for ($i = 0; $i -lt $samples; $i++) {
            $t = [double]$i / [double]$SampleRate
            $amp = 0.25 * [Math]::Sin(2.0 * [Math]::PI * 440.0 * $t)
            $writer.Write([int16]([Math]::Round($amp * 32767.0)))
        }
    }
    finally { $writer.Close() }
    return $Path
}

$RepoRoot = Resolve-RepoRoot -Explicit $RepoRoot
$WorkspaceDir = Join-Path $RepoRoot "VitApp\Workspace"
$AgentLog = Join-Path $WorkspaceDir "Logs\agent_last.log"
$RunStamp = Get-Date -Format "yyyyMMdd_HHmmss"
$RunRoot = Join-Path $RepoRoot ("coord\runs\MAT-D4-1\materialize_shadow_smoke_" + $RunStamp)
New-Item -ItemType Directory -Force -Path $RunRoot | Out-Null

if ([string]::IsNullOrWhiteSpace($KernelExe)) {
    # MAT-D card constraint: the real-stack kernel is the pcverify1 incremental
    # build; the staging export is never touched by this smoke.
    $KernelExe = Join-Path $RepoRoot "VitApp\cmake-build-pcverify1\VitApp_artefacts\Release\VitApp.exe"
}

$failureReasons = New-Object System.Collections.Generic.List[string]
function Add-Failure { param([string]$Message) $script:failureReasons.Add($Message); Write-FailLine $Message }

$report = [ordered]@{
    run_id = "materialize_shadow_smoke_" + $RunStamp
    card = "2026-09-29-MAT-D4"
    started_at = (Get-Date).ToUniversalTime().ToString("o")
    command_line = ($MyInvocation.Line)
    repo_head = ""
    repo_status = ""
    stack_mode = ""
    kernel = [ordered]@{}
    fixture = [ordered]@{}
    rounds = [ordered]@{}
    assertions = [ordered]@{}
    log_evidence = [ordered]@{}
    verdict = ""
}

try { $report.repo_head = (git -C $RepoRoot rev-parse HEAD 2>$null | Out-String).Trim() } catch { $report.repo_head = "unavailable" }
try { $report.repo_status = ((git -C $RepoRoot status --short 2>$null | Out-String).Trim() -replace "`r?`n", "; ") } catch { $report.repo_status = "unavailable" }

Write-Step "MATERIALIZATION shadow-mode real-stack smoke (MAT-D / G2-D)"
Write-Host ("run_root: " + $RunRoot)
Write-Host ("repo HEAD: " + $report.repo_head)
Write-Host ("kernel exe: " + $KernelExe)

# Kernel provenance (AGENTS section 9: record binary hash/time for a prebuilt
# kernel launched via -KernelExe).
if (-not (Test-Path -LiteralPath $KernelExe)) {
    Write-FailLine ("kernel exe not found: " + $KernelExe)
    $report.verdict = "kernel_exe_missing"
    Write-Utf8NoBom -Path (Join-Path $RunRoot "run_report.json") -Text ($report | ConvertTo-Json -Depth 8)
    exit 2
}
$kernelItem = Get-Item -LiteralPath $KernelExe
$kernelHash = (Get-FileHash -LiteralPath $KernelExe -Algorithm SHA256).Hash
$report.kernel.path = $KernelExe
$report.kernel.last_write_utc = $kernelItem.LastWriteTimeUtc.ToString("o")
$report.kernel.sha256 = $kernelHash
Write-Ok ("kernel sha256=" + $kernelHash + " mtime_utc=" + $report.kernel.last_write_utc)

# ---------------------------------------------------------------- stack bring-up
$createdTrackID = ""
if (-not $ReuseStack) {
    # AGENTS.md section 9 single-owner rule: refuse to build on a stack someone
    # else may own (an already-running agent would also lack this card's wiring).
    $agentListener = Get-NetTCPConnection -LocalPort 7878 -State Listen -ErrorAction SilentlyContinue | Select-Object -First 1
    $kernelListener = Get-NetTCPConnection -LocalPort 5555 -State Listen -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($null -ne $agentListener -or $null -ne $kernelListener) {
        Write-FailLine ("stack already running (agent pid=" + $(if ($agentListener) { $agentListener.OwningProcess } else { "-" }) + " kernel pid=" + $(if ($kernelListener) { $kernelListener.OwningProcess } else { "-" }) + "); clean it up or pass -ReuseStack")
        $report.verdict = "stack_occupied"
        Write-Utf8NoBom -Path (Join-Path $RunRoot "run_report.json") -Text ($report | ConvertTo-Json -Depth 8)
        exit 2
    }
    Write-Step "Bring up real stack via dev_agent_smoke (kernel via pcverify1 -KernelExe + Godot UI + agent, VIT_DAW_MATERIALIZATION=shadow)"
    $devSmoke = Join-Path $RepoRoot "scripts\dev_agent_smoke.ps1"
    $devParams = @{
        StartKernel = $true
        KernelExe = $KernelExe
        StartUI = $true
        NoChatSmoke = $true
        NoStripSilenceSmoke = $true
        Strict = $true
        WaitSeconds = $WaitSeconds
        RepoRoot = $RepoRoot
    }
    if ($SkipBuild) { $devParams["SkipBuild"] = $true }
    $devExit = 0
    # The shadow flag must reach the agent process (Start-Process inherits the
    # PowerShell process environment); restore the prior value afterwards so
    # nothing leaks into the caller's environment.
    $priorFlag = [Environment]::GetEnvironmentVariable("VIT_DAW_MATERIALIZATION", "Process")
    $env:VIT_DAW_MATERIALIZATION = "shadow"
    try {
        & $devSmoke @devParams
        if (-not $?) { $devExit = 1 }
    }
    catch {
        $devExit = 1
        Write-FailLine ("dev_agent_smoke failed: " + $_.Exception.Message)
    }
    finally {
        if ($null -eq $priorFlag) {
            Remove-Item Env:VIT_DAW_MATERIALIZATION -ErrorAction SilentlyContinue
        }
        else {
            $env:VIT_DAW_MATERIALIZATION = $priorFlag
        }
    }
    if ($devExit -ne 0) {
        Write-FailLine "dev_agent_smoke stack bring-up failed"
        $report.verdict = "stack_bringup_failed"
        Write-Utf8NoBom -Path (Join-Path $RunRoot "run_report.json") -Text ($report | ConvertTo-Json -Depth 8)
        exit 2
    }
    $report.stack_mode = "dev_agent_smoke_started"
}
else {
    $report.stack_mode = "reused_existing_stack"
}

# ---------------------------------------------------------------- S1: shadow startup
Write-Step "Agent health + S1: flag=shadow alive on the real stack"
$state = $null
$healthDeadline = (Get-Date).AddSeconds(45)
while ((Get-Date) -lt $healthDeadline) {
    try {
        $state = Invoke-Json -Method GET -Uri ($AgentHttp.TrimEnd("/") + "/agent/state") -TimeoutSec 10
        if ($null -ne $state -and [string](Get-OptionalProperty -Object $state -Name "status") -eq "ok") { break }
    }
    catch { Start-Sleep -Seconds 3 }
    if ($null -eq $state) { Start-Sleep -Seconds 3 }
}
if ($null -eq $state -or [string](Get-OptionalProperty -Object $state -Name "status") -ne "ok") {
    Add-Failure "GET /agent/state did not return ok (agent not reachable)"
}
else {
    Write-Ok ("agent ok tool_count=" + [string](Get-OptionalProperty -Object $state -Name "tool_count"))
}
# Startup evidence: the mode line is emitted at agent construction, before any
# observation noise, so grep the whole (fresh per-run) agent log for it.
$startupLines = @(Read-LogLines -Path $AgentLog | Where-Object { $_.Contains("[materialize] mode=") })
Write-Utf8NoBom -Path (Join-Path $RunRoot "startup_mode_lines.txt") -Text ($startupLines -join [Environment]::NewLine)
if ($startupLines.Count -eq 0) {
    Add-Failure "S1 no `[materialize] mode=` startup line in agent log (shadow wiring not alive; off default would emit none)"
}
elseif (-not $startupLines[0].Contains("mode=shadow")) {
    Add-Failure ("S1 startup line is not mode=shadow: " + $startupLines[0])
}
else {
    Write-Ok ("S1 pass: " + $startupLines[0])
}
$report.assertions.S1_startup_line = if ($startupLines.Count -gt 0) { $startupLines[0] } else { "" }

if ($failureReasons.Count -gt 0) {
    $report.verdict = "s1_failed"
    Write-Utf8NoBom -Path (Join-Path $RunRoot "run_report.json") -Text ($report | ConvertTo-Json -Depth 8)
    exit 1
}

# ---------------------------------------------------------------- fixture track
Write-Step "Fixture: temporary track + modest sine wav"
$trackResp = Invoke-AgentTool -Tool "track.add" -ToolArgs @{} -TimeoutSec 30 -Source "materialize_shadow_smoke.fixture_track" -Confirmed
$trackStatus = [string](Get-OptionalProperty -Object $trackResp -Name "status")
$trackResult = Get-OptionalProperty -Object $trackResp -Name "result"
$createdTrackID = [string](Get-FirstPropertyValue -Object $trackResult -Names @("track_id", "id", "item_id"))
if ($trackStatus -ne "ok" -or [string]::IsNullOrWhiteSpace($createdTrackID)) {
    Add-Failure ("track.add failed status=" + $trackStatus + " error=" + [string](Get-OptionalProperty -Object $trackResp -Name "error"))
    $report.verdict = "fixture_track_failed"
    Write-Utf8NoBom -Path (Join-Path $RunRoot "run_report.json") -Text ($report | ConvertTo-Json -Depth 8)
    exit 1
}
Write-Ok ("fixture track_id=" + $createdTrackID)
$report.fixture.track_id = $createdTrackID

$projectState = Invoke-AgentTool -Tool "project.state" -ToolArgs @{} -TimeoutSec 30
$projectResult = Get-OptionalProperty -Object $projectState -Name "result"
$audioSettings = Get-OptionalProperty -Object $projectResult -Name "audio_settings"
if ($null -eq $audioSettings) { $audioSettings = Get-OptionalProperty -Object (Get-OptionalProperty -Object $projectResult -Name "project") -Name "audio_settings" }
$projectSampleRate = 48000
$rawRate = Get-FirstPropertyValue -Object $audioSettings -Names @("sample_rate_hz", "sample_rate", "sampleRate")
if ($null -ne $rawRate) { $projectSampleRate = [int]$rawRate }
$report.fixture.project_sample_rate_hz = $projectSampleRate
Write-Ok ("project sample rate=" + $projectSampleRate)

$wavName = "mat_d_shadow_fixture_{0}_{1}.wav" -f $PID, ([DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds())
$wavPath = Write-TestWav -Path (Join-Path (Join-Path $WorkspaceDir "Artifacts\smoke") $wavName) -SampleRate $projectSampleRate
$report.fixture.wav_path = $wavPath
Write-Ok ("fixture wav written: " + $wavPath)

$importResp = Invoke-AgentTool -Tool "clip.import_media_to_track" -ToolArgs @{
    track_id = $createdTrackID
    file_path = $wavPath
    start_time = 0.0
    media_type = "audio"
    mode = "non_destructive"
} -TimeoutSec 90 -Source "materialize_shadow_smoke.fixture_import" -Confirmed
$importStatus = [string](Get-OptionalProperty -Object $importResp -Name "status")
Write-Utf8NoBom -Path (Join-Path $RunRoot "import_response.json") -Text ($importResp | ConvertTo-Json -Depth 8)
if ($importStatus -ne "ok") {
    Add-Failure ("clip.import_media_to_track failed status=" + $importStatus + " error=" + [string](Get-OptionalProperty -Object $importResp -Name "error"))
}
else {
    Write-Ok "fixture wav imported"
}

# ---------------------------------------------------------------- observation loop (baseline)
$mixSessionID = "materialize_shadow_smoke_" + $RunStamp
$report.fixture.mix_session_id = $mixSessionID

# Track-targeted observation so the dom reconcile path actually runs
# (materializeObserveRound reconciles only when the observation targets a
# track). Baseline poll succeeds once a shadow_round line shows dom rows
# upserted (the adapter materialized the fixture track).
$baselineLines = @(Read-LogLines -Path $AgentLog)
$baselineSet = New-Object 'System.Collections.Generic.HashSet[string]'
foreach ($line in $baselineLines) { [void]$baselineSet.Add($line) }
$report.log_evidence.baseline_line_count = $baselineLines.Count
Write-Ok ("agent log baseline lines=" + $baselineLines.Count)

$shadowRoundLines = New-Object System.Collections.Generic.List[string]
$baselineReady = $false
$finalObservationID = ""
$pollRounds = 0
$deadline = (Get-Date).AddSeconds($ObserveTimeoutSeconds)

function Request-ShadowObservation {
    $obsResp = Invoke-AgentTool -Tool "mix_request_observation" -ToolArgs @{
        mix_session_id = $mixSessionID
        goal_text = "materialization shadow-mode smoke observation"
        track_id = $createdTrackID
        target_ref = @{ kind = "track"; id = $createdTrackID }
    } -TimeoutSec 120 -Source "materialize_shadow_smoke.observe"
    return $obsResp
}

Write-Step ("Poll track-targeted observations until a shadow round materializes dom rows (budget " + $ObserveTimeoutSeconds + "s)")
while ((Get-Date) -lt $deadline) {
    $pollRounds++
    $obsResp = Request-ShadowObservation
    $obsStatus = [string](Get-OptionalProperty -Object $obsResp -Name "status")
    if ($obsStatus -ne "ok") {
        Write-WarnLine ("round " + $pollRounds + " observation failed status=" + $obsStatus + " error=" + [string](Get-OptionalProperty -Object $obsResp -Name "error"))
        if ($pollRounds -ge 3) {
            Add-Failure ("mix_request_observation kept failing: status=" + $obsStatus + " error=" + [string](Get-OptionalProperty -Object $obsResp -Name "error"))
            break
        }
        Start-Sleep -Seconds $PollIntervalSeconds
        continue
    }
    $obsResult = Get-OptionalProperty -Object $obsResp -Name "result"
    $finalObservationID = [string](Get-FirstPropertyValue -Object $obsResult -Names @("observation_id"))
    $report.fixture.observation_id = $finalObservationID

    # Collect new [materialize] lines since baseline (dedupe by line content).
    $afterLines = @(Read-LogLines -Path $AgentLog)
    foreach ($line in $afterLines) {
        if (-not $baselineSet.Contains($line) -and $line.Contains("[materialize] shadow_round") -and -not $shadowRoundLines.Contains($line)) {
            $shadowRoundLines.Add($line)
        }
    }
    $lastLine = if ($shadowRoundLines.Count -gt 0) { $shadowRoundLines[$shadowRoundLines.Count - 1] } else { "" }
    $domUps = 0
    if ($lastLine -ne "") {
        $m = [regex]::Match($lastLine, "dom:[^;]*\bups=(\d+)")
        if ($m.Success) { $domUps = [int]$m.Groups[1].Value }
    }
    if ($lastLine -ne "" -and $domUps -gt 0) {
        Write-Ok ("shadow round materialized dom rows after " + $pollRounds + " round(s), observation_id=" + $finalObservationID)
        Write-Host ("   " + $lastLine)
        $baselineReady = $true
        break
    }
    Write-Host ("   round " + $pollRounds + ": waiting for dom rows in shadow_round metrics line; sleeping " + $PollIntervalSeconds + "s")
    Start-Sleep -Seconds $PollIntervalSeconds
}
if (-not $baselineReady -and $failureReasons.Count -eq 0) {
    Add-Failure ("no shadow_round metrics line with dom ups>0 within budget (rounds=" + $pollRounds + ")")
}

# ---------------------------------------------------------------- S2 baseline anchor
# MAT-E0 同源口径：物化输入源切换到完整态（证据回溯 State A raw）后，观察对账
# 作为回归指标恢复（G2 ruling 附条件）——所有轮次零分歧零 timing 排除+真比过。
# 非空洞证据=dom 行确实到达对账闸门并被处置（timing/measurement 排除或真比过
# 任一面>0——同源态下正常形态是真比过 reconcile>0）。
Write-Step "S2 baseline: shadow_round lines observable + reconcile gate reached"
if ($shadowRoundLines.Count -eq 0) {
    Add-Failure "S2 no [materialize] shadow_round metrics line in agent log diff (metric not observable)"
}
else {
    $gateReached = $false
    foreach ($line in $shadowRoundLines) {
        $tim = Get-ShadowMetric -Line $line -Field "timing_carried_excluded"
        $meas = Get-ShadowMetric -Line $line -Field "measurement_carried_excluded"
        $rec = Get-ShadowMetric -Line $line -Field "reconcile_rows"
        if (($tim -gt 0) -or ($meas -gt 0) -or ($rec -gt 0)) { $gateReached = $true; break }
    }
    if (-not $gateReached) {
        Add-Failure "S2 vacuous baseline: no shadow_round line shows the dom row reaching the reconcile gate (timing/measurement/reconcile all zero)"
    }
    else {
        Write-Ok "S2 baseline non-vacuous: dom row reached the reconcile gate (disposed via timing/measurement/reconcile)"
    }
}
$baselineLastLine = if ($shadowRoundLines.Count -gt 0) { $shadowRoundLines[$shadowRoundLines.Count - 1] } else { "" }
$baselineMetrics = Read-ShadowRoundMetrics -Line $baselineLastLine
$report.rounds.baseline = $baselineMetrics
$report.log_evidence.shadow_round_line_count = $shadowRoundLines.Count
$report.log_evidence.shadow_round_lines = @($shadowRoundLines)

# ---------------------------------------------------------------- change event + S3: dirty propagation
if ($baselineReady) {
    Write-Step "Change event: set_volume on fixture track"
    $preChangeLines = @(Read-LogLines -Path $AgentLog)
    $preChangeSet = New-Object 'System.Collections.Generic.HashSet[string]'
    foreach ($line in $preChangeLines) { [void]$preChangeSet.Add($line) }

    $volumeResp = Invoke-AgentTool -Tool "set_volume" -ToolArgs @{
        track_id = $createdTrackID
        db = -7.5
    } -TimeoutSec 60 -Source "materialize_shadow_smoke.change_event" -Confirmed
    $volumeStatus = [string](Get-OptionalProperty -Object $volumeResp -Name "status")
    Write-Utf8NoBom -Path (Join-Path $RunRoot "set_volume_response.json") -Text ($volumeResp | ConvertTo-Json -Depth 8)
    if ($volumeStatus -ne "ok") {
        Add-Failure ("set_volume failed status=" + $volumeStatus + " error=" + [string](Get-OptionalProperty -Object $volumeResp -Name "error"))
    }
    else {
        Write-Ok "set_volume applied (fixture track -> -7.5 dB)"
    }

    # Event-time dirty propagation evidence: receipt line logged when the
    # shadow delta lands, before any further observation.
    $afterChangeLines = @(Read-LogLines -Path $AgentLog | Where-Object { -not $preChangeSet.Contains($_) })
    $receiptLines = @($afterChangeLines | Where-Object { $_.Contains("[materialize] receipt") })
    Write-Utf8NoBom -Path (Join-Path $RunRoot "receipt_lines.txt") -Text ($receiptLines -join [Environment]::NewLine)
    if ($receiptLines.Count -eq 0) {
        Add-Failure "S3 no [materialize] receipt line after set_volume (event-time dirty propagation not observable)"
    }
    else {
        Write-Ok ("S3 event-time evidence: " + $receiptLines.Count + " receipt line(s), e.g. " + $receiptLines[0])
    }

    # Post-change observation round (=MAT-D4 变更轮): invalidations must show up
    # in the metrics line (kind marked dirty after the change event), and the
    # finalize-vs-tail generation-gap divergence must be REGISTERED as
    # timing-carried (excluded + counted separately, never silent).
    Write-Step "Change-round observation (post set_volume)"
    $obsResp = Request-ShadowObservation
    $obsStatus = [string](Get-OptionalProperty -Object $obsResp -Name "status")
    $changeLine = ""
    if ($obsStatus -ne "ok") {
        Add-Failure ("change-round mix_request_observation failed status=" + $obsStatus + " error=" + [string](Get-OptionalProperty -Object $obsResp -Name "error"))
    }
    else {
        $obsResult = Get-OptionalProperty -Object $obsResp -Name "result"
        $report.fixture.post_change_observation_id = [string](Get-FirstPropertyValue -Object $obsResult -Names @("observation_id"))
        $changeLine = Wait-NextShadowRoundLine -Collected $shadowRoundLines -BudgetSeconds 60
        if ($changeLine -eq "") {
            Add-Failure "change-round shadow_round metrics line did not appear within 60s"
        }
    }
    $invLines = @($shadowRoundLines | Where-Object { ([regex]::Match($_, ":inv=([1-9]\d*)").Success) })
    Write-Utf8NoBom -Path (Join-Path $RunRoot "shadow_round_metrics_lines.txt") -Text ($shadowRoundLines -join [Environment]::NewLine)
    if ($invLines.Count -eq 0) {
        Add-Failure ("S3 no shadow_round line with invalidations>0 after the change event (kind dirty not observable; lines=" + $shadowRoundLines.Count + ")")
    }
    else {
        Write-Ok ("S3 pass: " + $invLines.Count + " shadow_round line(s) with invalidations>0, e.g. " + $invLines[0])
        $report.assertions.S3_invalidation_line = $invLines[0]
    }

    # ---------------- S2b (变更轮): MAT-E0 同源口径——零新增分歧+零新增 timing
    # 排除+真比过。物化输入源=证据回溯完整态（State A）后，finalize 世代与尾挂
    # 世代同源，世代差分歧结构性消失（MAT-D4 的"登记不静默"是止血口径，E0 起
    # 观察对账作为回归指标恢复——任何非零 delta 都是回归红，必须可解释或修复）。
    $changeMetrics = Read-ShadowRoundMetrics -Line $changeLine
    $report.rounds.change = $changeMetrics
    Write-Step ("S2 change round: same-source zero div/timing delta + real compare (baseline div/tim/rec=" + $baselineMetrics.shadow_divergences + "/" + $baselineMetrics.timing_carried_excluded + "/" + $baselineMetrics.reconcile_rows + " change div/tim/rec=" + $changeMetrics.shadow_divergences + "/" + $changeMetrics.timing_carried_excluded + "/" + $changeMetrics.reconcile_rows + ")")
    if ($changeLine -eq "") {
        Add-Failure "S2 change round has no shadow_round metrics line"
    }
    elseif ($changeMetrics.timing_carried_excluded -lt 0 -or $baselineMetrics.timing_carried_excluded -lt 0) {
        Add-Failure ("S2 change round metrics unreadable (timing field missing; stale agent binary?): baseline_line=" + $baselineLastLine + " change_line=" + $changeLine)
    }
    else {
        $timingDelta = $changeMetrics.timing_carried_excluded - $baselineMetrics.timing_carried_excluded
        $divDelta = $changeMetrics.shadow_divergences - $baselineMetrics.shadow_divergences
        $recDelta = $changeMetrics.reconcile_rows - $baselineMetrics.reconcile_rows
        if ($divDelta -ne 0) {
            Add-Failure ("S2 change round diverged after same-source switch (div delta=" + $divDelta + "; observation reconciliation is the restored regression gate — non-zero must be explainable or fixed)")
        }
        elseif ($timingDelta -ne 0) {
            Add-Failure ("S2 change round still timing-carried after same-source switch (delta=" + $timingDelta + "; MAT-E0 expects zero — the generation gap must be structurally gone)")
        }
        elseif ($recDelta -lt 1) {
            Add-Failure ("S2 change round vacuous: no real compared row (reconcile delta=" + $recDelta + ")")
        }
        else {
            Write-Ok ("S2 change round pass: div_delta=0 timing_delta=0 reconcile_delta=" + $recDelta + " (same-source reconciliation restored as regression gate)")
            $report.assertions.S2_change_round_same_source = @{ div_delta = $divDelta; timing_delta = $timingDelta; reconcile_delta = $recDelta }
        }
    }

    # ---------------- silence window + convergence round (MAT-D4 G2-D 收敛态闭环)
    # 静默判据：>=8s 地板 + [materialize] 事件面行数稳定（活栈有 authority/
    # chat 周期日志，总行数永不稳定——只看物化事件面）；预算 40s。
    Write-Step "Telemetry silence window before convergence round (>=8s + [materialize]-line stability, budget 40s)"
    $silenceStart = Get-Date
    $stablePolls = 0
    $lastMaterializeLines = -1
    $silenceStable = $false
    while ($true) {
        Start-Sleep -Seconds 2
        $count = @((Read-LogLines -Path $AgentLog) | Where-Object { $_.Contains("[materialize]") }).Count
        if ($count -eq $lastMaterializeLines) { $stablePolls++ } else { $stablePolls = 0; $lastMaterializeLines = $count }
        $elapsed = ((Get-Date) - $silenceStart).TotalSeconds
        if ($elapsed -ge 8 -and $stablePolls -ge 3) { $silenceStable = $true; break }
        if ($elapsed -ge 40) { break }
    }
    if ($silenceStable) {
        Write-Ok ("silence window reached: log stable for " + ($stablePolls * 2) + "s, total wait " + [int](((Get-Date) - $silenceStart).TotalSeconds) + "s")
    }
    else {
        Write-WarnLine "silence window budget exhausted (40s) without full stability; proceeding (convergence assertions decide)"
    }
    $report.rounds.silence_window = [ordered]@{
        stable = $silenceStable
        waited_seconds = [int](((Get-Date) - $silenceStart).TotalSeconds)
    }

    Write-Step "Convergence-round observation (third round, post-silence)"
    $convResp = Request-ShadowObservation
    $convStatus = [string](Get-OptionalProperty -Object $convResp -Name "status")
    $convLine = ""
    if ($convStatus -ne "ok") {
        Add-Failure ("convergence-round mix_request_observation failed status=" + $convStatus + " error=" + [string](Get-OptionalProperty -Object $convResp -Name "error"))
    }
    else {
        $convResult = Get-OptionalProperty -Object $convResp -Name "result"
        $report.fixture.convergence_observation_id = [string](Get-FirstPropertyValue -Object $convResult -Names @("observation_id"))
        $convLine = Wait-NextShadowRoundLine -Collected $shadowRoundLines -BudgetSeconds 60
        if ($convLine -eq "") {
            Add-Failure "convergence-round shadow_round metrics line did not appear within 60s"
        }
    }
    $convMetrics = Read-ShadowRoundMetrics -Line $convLine
    $report.rounds.convergence = $convMetrics

    # ---------------- S2c (收敛轮): 零分歧 + 零新排除 + 真比过
    Write-Step ("S2 convergence round: zero new divergences AND zero new timing exclusions AND real compare (change div=" + $changeMetrics.shadow_divergences + "/" + $changeMetrics.timing_carried_excluded + "/" + $changeMetrics.reconcile_rows + " conv div=" + $convMetrics.shadow_divergences + "/" + $convMetrics.timing_carried_excluded + "/" + $convMetrics.reconcile_rows + ")")
    if ($convLine -eq "") {
        Add-Failure "S2 convergence round has no shadow_round metrics line"
    }
    else {
        $convDivDelta = $convMetrics.shadow_divergences - $changeMetrics.shadow_divergences
        $convTimingDelta = $convMetrics.timing_carried_excluded - $changeMetrics.timing_carried_excluded
        $convRecDelta = $convMetrics.reconcile_rows - $changeMetrics.reconcile_rows
        if ($convDivDelta -ne 0) {
            Add-Failure ("S2 convergence round still diverging: new divergences=" + $convDivDelta + " (穿透在静默窗后仍发生=缺陷更深——卡面停止条件，取证上交) line=" + $convLine)
        }
        elseif ($convTimingDelta -ne 0) {
            Add-Failure ("S2 convergence round still timing-carried: new exclusions=" + $convTimingDelta + " (世代差窗口在静默窗后仍开着=未真收敛) line=" + $convLine)
        }
        elseif ($convRecDelta -lt 1) {
            Add-Failure ("S2 convergence zero-divergence is vacuous: no new compared row (reconcile_rows delta=" + $convRecDelta + "; dom row excluded or missing) line=" + $convLine)
        }
        else {
            Write-Ok ("S2 convergence round pass: div_delta=0 timing_delta=0 reconcile_delta=" + $convRecDelta + " (G2-D converged-state closure)")
            $report.assertions.S2_convergence_round_zero_divergence = $true
        }
    }
}

# ---------------------------------------------------------------- cleanup
if (-not [string]::IsNullOrWhiteSpace($createdTrackID)) {
    Write-Step "Cleanup: delete fixture track"
    try {
        $cleanupState = Invoke-AgentTool -Tool "project.state" -ToolArgs @{} -TimeoutSec 30 -Source "materialize_shadow_smoke.cleanup"
        $cleanupResult = Get-OptionalProperty -Object $cleanupState -Name "result"
        $userTrackCount = [int](Get-FirstPropertyValue -Object $cleanupResult -Names @("user_track_count"))
        if ($userTrackCount -le 1) {
            Write-WarnLine ("leaving fixture track (kernel refuses deleting the last audio track): " + $createdTrackID)
            $report.fixture.track_cleanup = "left_last_track"
        }
        else {
            $trackCleanup = Invoke-AgentTool -Tool "track.delete" -ToolArgs @{ track_id = $createdTrackID } -TimeoutSec 60 -Source "materialize_shadow_smoke.cleanup" -Confirmed
            $report.fixture.track_cleanup = [string](Get-OptionalProperty -Object $trackCleanup -Name "status")
            Write-Host ("   track.delete status=" + $report.fixture.track_cleanup)
        }
    }
    catch {
        Write-WarnLine ("cleanup failed: " + $_.Exception.Message)
        $report.fixture.track_cleanup = "failed"
    }
}

# Preserve the agent log for this run (agent_last.log is overwritten by the
# next stack run; the ring keeps only the last 800 lines).
try {
    Copy-Item -LiteralPath $AgentLog -Destination (Join-Path $RunRoot "agent_last.log") -ErrorAction SilentlyContinue
    $report.log_evidence.agent_log_copy = "saved"
}
catch { $report.log_evidence.agent_log_copy = "copy_failed" }

# Divergence forensics (MAT-D): collect [materialize] divergence WARN lines
# (ref + both hashes + both payloads) so a red S2 carries root-cause-grade
# evidence in the run artifacts, not just a counter.
$divergenceLines = @(Read-LogLines -Path $AgentLog | Where-Object { $_.Contains("[materialize] divergence") })
if ($divergenceLines.Count -gt 0) {
    Write-Utf8NoBom -Path (Join-Path $RunRoot "divergence_lines.txt") -Text ($divergenceLines -join [Environment]::NewLine)
    $report.log_evidence.divergence_lines = @($divergenceLines)
}

# ---------------------------------------------------------------- verdict (S4 = exit code)
$report.finished_at = (Get-Date).ToUniversalTime().ToString("o")
if ($failureReasons.Count -eq 0) {
    $report.verdict = "PASS"
    Write-Utf8NoBom -Path (Join-Path $RunRoot "run_report.json") -Text ($report | ConvertTo-Json -Depth 8)
    Write-Step "PASS"
    Write-Ok ("MATERIALIZATION shadow real-stack smoke passed (S1 startup / S2 same-source zero divergence + zero timing exclusion on change and convergence rounds / S3 dirty propagation / S4 exit 0). run_root=" + $RunRoot)
    exit 0
}
$report.verdict = "FAIL"
$report.failures = @($failureReasons)
Write-Utf8NoBom -Path (Join-Path $RunRoot "run_report.json") -Text ($report | ConvertTo-Json -Depth 8)
Write-Step "FAIL"
foreach ($reason in $failureReasons) { Write-FailLine $reason }
Write-Host ("run_root=" + $RunRoot)
exit 1
