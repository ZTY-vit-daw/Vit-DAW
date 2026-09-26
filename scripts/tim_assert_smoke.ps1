<#
TIM_ASSERT real-stack supervision smoke (card L2-1-TIM-SMOKE-1, design
docs/TIM_ASSERTER_V1_DESIGN.md section 4.2).

Proves the [tim.assert] warn path end to end on the real three-piece stack
(VitApp kernel + Godot UI + Go agent), including the harness logx wiring:

  1. bring the stack up by reusing dev_agent_smoke.ps1 -StartKernel -StartUI
     (builds and installs the agent binary first);
  2. fixture: temporary track + near-full-scale wav (amplitude 0.999,
     peak ~= -0.009 dBFS -> triggers AS-PEAK assert_level_ceiling_exceeded
     and AS-SIG P2 possible_clipping_or_no_headroom) imported via
     clip.import_media_to_track;
  3. trigger mix_request_observation (scope=full_project) and poll until the
     L3 acoustic evidence reached TIM (level_ceiling no longer not_evaluable);
  4. four assertion groups (all must pass for exit 0):
     A1 mix_read observation.tim_projection has a non-empty assertions[] and
        every row status is one of pass/fail/not_evaluable;
     A2 the fixture track's fail codes appear in assertions[] AND in
        risk_summary.primary_codes;
     A3 agent_last.log diff (baseline taken before the first observation)
        contains [tim.assert] WARN lines matching the expected fail code;
     A4 exit 0 (this script's own exit code).

Usage:
  powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\tim_assert_smoke.ps1
  powershell ... -ReuseStack          # stack already running (kernel+UI+agent)
  powershell ... -SkipBuild           # reuse the installed agent binary

Exit codes: 0 = PASS, 1 = assertion/environment failure, 2 = stack bring-up
failure. Artifacts land under coord\runs\L2-1-TIM-SMOKE-1\ (new dir per run).
#>

[CmdletBinding()]
param(
    [string]$RepoRoot = "",
    [string]$AgentHttp = "http://127.0.0.1:7878",
    [int]$WaitSeconds = 20,
    [int]$AcousticTimeoutSeconds = 150,
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

function Invoke-Json {
    # [CmdletBinding()] matters: without it a mistyped parameter name (e.g.
    # -Uri vs -Url) silently lands in $args and the request goes out with an
    # empty URI (a real failure mode hit during this card's bring-up).
    [CmdletBinding()]
    param([string]$Method, [string]$Uri, $Body, [int]$TimeoutSec = 60)
    if ($null -ne $Body) {
        $json = $Body | ConvertTo-Json -Depth 10 -Compress
        return Invoke-RestMethod -Method $Method -Uri $Uri -ContentType "application/json; charset=utf-8" -Body ([System.Text.Encoding]::UTF8.GetBytes($json)) -TimeoutSec $TimeoutSec
    }
    return Invoke-RestMethod -Method $Method -Uri $Uri -TimeoutSec $TimeoutSec
}

function Invoke-AgentTool {
    # Note: the args parameter is named ToolArgs because $args is a PowerShell
    # automatic variable and cannot be a declared parameter.
    [CmdletBinding()]
    param([string]$Tool, [hashtable]$ToolArgs, [int]$TimeoutSec = 60, [string]$Source = "tim_assert_smoke", [switch]$Confirmed)
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

# Near-full-scale test wav: 0.999 amplitude sine, 440 Hz, mono 16-bit.
# 20*log10(0.999) ~= -0.009 dBFS: above the -0.1 clip ceiling (AS-SIG P2 fail)
# and above the -1.0 level ceiling (AS-PEAK fail), with ~0.09 dB margin to the
# tighter threshold so L3 measurement noise cannot flip the verdict.
function Write-NearFullScaleWav {
    param([string]$Path, [int]$SampleRate)
    $dir = Split-Path -Parent $Path
    New-Item -ItemType Directory -Path $dir -Force | Out-Null
    $durationSeconds = 2.2
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
            $amp = 0.999 * [Math]::Sin(2.0 * [Math]::PI * 440.0 * $t)
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
$RunRoot = Join-Path $RepoRoot ("coord\runs\L2-1-TIM-SMOKE-1\tim_assert_smoke_" + $RunStamp)
New-Item -ItemType Directory -Force -Path $RunRoot | Out-Null

$failureReasons = New-Object System.Collections.Generic.List[string]
function Add-Failure { param([string]$Message) $script:failureReasons.Add($Message); Write-FailLine $Message }

$report = [ordered]@{
    run_id = "tim_assert_smoke_" + $RunStamp
    card = "L2-1-TIM-SMOKE-1"
    started_at = (Get-Date).ToUniversalTime().ToString("o")
    command_line = ($MyInvocation.Line)
    repo_head = ""
    repo_status = ""
    stack_mode = ""
    fixture = [ordered]@{}
    assertions_block = [ordered]@{}
    log_diff = [ordered]@{}
    verdict = ""
}

try { $report.repo_head = (git -C $RepoRoot rev-parse HEAD 2>$null | Out-String).Trim() } catch { $report.repo_head = "unavailable" }
try { $report.repo_status = ((git -C $RepoRoot status --short 2>$null | Out-String).Trim() -replace "`r?`n", "; ") } catch { $report.repo_status = "unavailable" }

Write-Step "TIM assert real-stack smoke"
Write-Host ("run_root: " + $RunRoot)
Write-Host ("repo HEAD: " + $report.repo_head)

# ---------------------------------------------------------------- stack bring-up
$createdTrackID = ""
$stackBroughtUp = $false
if (-not $ReuseStack) {
    # AGENTS.md section 9 single-owner rule: refuse to build on a stack someone
    # else may own (an already-running agent would also lack this card's wiring).
    $agentListener = Get-NetTCPConnection -LocalPort 7878 -State Listen -ErrorAction SilentlyContinue | Select-Object -First 1
    $kernelListener = Get-NetTCPConnection -LocalPort 5555 -State Listen -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($null -ne $agentListener -or $null -ne $kernelListener) {
        Write-FailLine ("stack already running (agent pid=" + $(if ($agentListener) { $agentListener.OwningProcess } else { "-" }) + " kernel pid=" + $(if ($kernelListener) { $kernelListener.OwningProcess } else { "-" }) + "); clean it up or pass -ReuseStack")
        $report.verdict = "stack_occupied"
        $report | ConvertTo-Json -Depth 8 | Out-File -FilePath (Join-Path $RunRoot "run_report.json") -Encoding utf8
        exit 2
    }
    Write-Step "Bring up real stack via dev_agent_smoke (kernel + Godot UI + agent)"
    $devSmoke = Join-Path $RepoRoot "scripts\dev_agent_smoke.ps1"
    $devParams = @{
        StartKernel = $true
        StartUI = $true
        NoChatSmoke = $true
        NoStripSilenceSmoke = $true
        Strict = $true
        WaitSeconds = $WaitSeconds
        RepoRoot = $RepoRoot
    }
    if ($SkipBuild) { $devParams["SkipBuild"] = $true }
    $devExit = 0
    try {
        & $devSmoke @devParams
        if (-not $?) { $devExit = 1 }
    }
    catch {
        $devExit = 1
        Write-FailLine ("dev_agent_smoke failed: " + $_.Exception.Message)
    }
    if ($devExit -ne 0) {
        Write-FailLine "dev_agent_smoke stack bring-up failed"
        $report.verdict = "stack_bringup_failed"
        $report | ConvertTo-Json -Depth 8 | Out-File -FilePath (Join-Path $RunRoot "run_report.json") -Encoding utf8
        exit 2
    }
    $stackBroughtUp = $true
    $report.stack_mode = "dev_agent_smoke_started"
}
else {
    $report.stack_mode = "reused_existing_stack"
}

Write-Step "Agent health"
# The stack just came up and dev_agent_smoke's closing chat smoke may leave the
# agent briefly busy; poll instead of a single 10s probe.
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
if ($failureReasons.Count -gt 0) {
    $report.verdict = "agent_unreachable"
    $report | ConvertTo-Json -Depth 8 | Out-File -FilePath (Join-Path $RunRoot "run_report.json") -Encoding utf8
    exit 1
}

# ---------------------------------------------------------------- fixture track
Write-Step "Fixture: temporary track + near-full-scale wav"
$trackResp = Invoke-AgentTool -Tool "track.add" -ToolArgs @{} -TimeoutSec 30 -Source "tim_assert_smoke.fixture_track" -Confirmed
$trackStatus = [string](Get-OptionalProperty -Object $trackResp -Name "status")
$trackResult = Get-OptionalProperty -Object $trackResp -Name "result"
$createdTrackID = [string](Get-FirstPropertyValue -Object $trackResult -Names @("track_id", "id", "item_id"))
if ($trackStatus -ne "ok" -or [string]::IsNullOrWhiteSpace($createdTrackID)) {
    Add-Failure ("track.add failed status=" + $trackStatus + " error=" + [string](Get-OptionalProperty -Object $trackResp -Name "error"))
    $report.verdict = "fixture_track_failed"
    $report | ConvertTo-Json -Depth 8 | Out-File -FilePath (Join-Path $RunRoot "run_report.json") -Encoding utf8
    exit 1
}
Write-Ok ("fixture track_id=" + $createdTrackID)
$report.fixture.track_id = $createdTrackID

# Match the wav sample rate to the project so AS-SR does not fire on purpose.
$projectState = Invoke-AgentTool -Tool "project.state" -ToolArgs @{} -TimeoutSec 30
$projectResult = Get-OptionalProperty -Object $projectState -Name "result"
$audioSettings = Get-OptionalProperty -Object $projectResult -Name "audio_settings"
if ($null -eq $audioSettings) { $audioSettings = Get-OptionalProperty -Object (Get-OptionalProperty -Object $projectResult -Name "project") -Name "audio_settings" }
$projectSampleRate = 48000
$rawRate = Get-FirstPropertyValue -Object $audioSettings -Names @("sample_rate_hz", "sample_rate", "sampleRate")
if ($null -ne $rawRate) { $projectSampleRate = [int]$rawRate }
$report.fixture.project_sample_rate_hz = $projectSampleRate
Write-Ok ("project sample rate=" + $projectSampleRate)

$wavName = "tim_assert_near_full_scale_{0}_{1}.wav" -f $PID, ([DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds())
$wavPath = Write-NearFullScaleWav -Path (Join-Path (Join-Path $WorkspaceDir "Artifacts\smoke") $wavName) -SampleRate $projectSampleRate
$report.fixture.wav_path = $wavPath
Write-Ok ("near-full-scale wav written: " + $wavPath)

$importResp = Invoke-AgentTool -Tool "clip.import_media_to_track" -ToolArgs @{
    track_id = $createdTrackID
    file_path = $wavPath
    start_time = 0.0
    media_type = "audio"
    mode = "non_destructive"
} -TimeoutSec 90 -Source "tim_assert_smoke.fixture_import" -Confirmed
$importStatus = [string](Get-OptionalProperty -Object $importResp -Name "status")
$importResp | ConvertTo-Json -Depth 8 | Out-File -FilePath (Join-Path $RunRoot "import_response.json") -Encoding utf8
if ($importStatus -ne "ok") {
    Add-Failure ("clip.import_media_to_track failed status=" + $importStatus + " error=" + [string](Get-OptionalProperty -Object $importResp -Name "error"))
}
else {
    Write-Ok "fixture wav imported"
}

# ---------------------------------------------------------------- observation loop
$mixSessionID = "tim_assert_smoke_" + $RunStamp
$report.fixture.mix_session_id = $mixSessionID

# Log baseline BEFORE the first observation (design 4.2 step 3: baseline diff).
$baselineLines = @(Read-LogLines -Path $AgentLog)
$baselineSet = New-Object 'System.Collections.Generic.HashSet[string]'
foreach ($line in $baselineLines) { [void]$baselineSet.Add($line) }
$report.log_diff.baseline_line_count = $baselineLines.Count
Write-Ok ("agent log baseline lines=" + $baselineLines.Count)

function Request-ObservationAndReadTIM {
    $obsResp = Invoke-AgentTool -Tool "mix_request_observation" -ToolArgs @{
        scope = "full_project"
        mix_session_id = $mixSessionID
        goal_text = "tim assert supervision smoke"
    } -TimeoutSec 120 -Source "tim_assert_smoke.observe"
    $obsStatus = [string](Get-OptionalProperty -Object $obsResp -Name "status")
    if ($obsStatus -ne "ok") {
        return @{ ok = $false; stage = "mix_request_observation"; status = $obsStatus; error = [string](Get-OptionalProperty -Object $obsResp -Name "error"); response = $obsResp }
    }
    $obsResult = Get-OptionalProperty -Object $obsResp -Name "result"
    $obsID = [string](Get-FirstPropertyValue -Object $obsResult -Names @("observation_id"))
    if ([string]::IsNullOrWhiteSpace($obsID)) {
        return @{ ok = $false; stage = "observation_id"; status = $obsStatus; error = "no observation_id in result"; response = $obsResp }
    }
    $readResp = Invoke-AgentTool -Tool "mix_read" -ToolArgs @{
        observation_id = $obsID
        mix_session_id = $mixSessionID
        keys = @("observation.tim_projection")
    } -TimeoutSec 60 -Source "tim_assert_smoke.read"
    $readStatus = [string](Get-OptionalProperty -Object $readResp -Name "status")
    if ($readStatus -ne "ok") {
        return @{ ok = $false; stage = "mix_read"; status = $readStatus; error = [string](Get-OptionalProperty -Object $readResp -Name "error"); response = $readResp }
    }
    $readResult = Get-OptionalProperty -Object $readResp -Name "result"
    $items = Get-OptionalProperty -Object $readResult -Name "items"
    $tim = Get-OptionalProperty -Object $items -Name "observation.tim_projection"
    if ($null -eq $tim) {
        return @{ ok = $false; stage = "tim_projection"; status = "missing"; error = "observation.tim_projection not in items"; response = $readResp }
    }
    $timStatus = [string](Get-OptionalProperty -Object $tim -Name "status")
    if ($timStatus -eq "missing") {
        return @{ ok = $false; stage = "tim_projection"; status = "missing"; error = "tim_projection status=missing"; response = $readResp }
    }
    return @{ ok = $true; observation_id = $obsID; tim = $tim; response = $readResp }
}

Write-Step ("Poll observations until acoustic evidence reaches TIM (budget " + $AcousticTimeoutSeconds + "s)")
$deadline = (Get-Date).AddSeconds($AcousticTimeoutSeconds)
$timPayload = $null
$finalObservationID = ""
$pollRounds = 0
$lastErrorStage = ""
$lastErrorDetail = ""
while ((Get-Date) -lt $deadline) {
    $pollRounds++
    $round = Request-ObservationAndReadTIM
    if (-not $round.ok) {
        $lastErrorStage = [string]$round.stage
        $lastErrorDetail = ([string]$round.status + " " + [string]$round.error)
        Write-WarnLine ("round " + $pollRounds + " not ready: " + $lastErrorStage + " " + $lastErrorDetail)
        if ($round.stage -eq "mix_request_observation" -or $round.stage -eq "mix_read") {
            # Infrastructure failure, not acoustic latency: stop polling.
            break
        }
        Start-Sleep -Seconds $PollIntervalSeconds
        continue
    }
    $timPayload = $round.tim
    $finalObservationID = [string]$round.observation_id
    $assertions = @(Get-OptionalProperty -Object $timPayload -Name "assertions")
    $ceilingReady = $false
    foreach ($row in $assertions) {
        if ([string](Get-OptionalProperty -Object $row -Name "asserter") -eq "level_ceiling" -and
            [string](Get-OptionalProperty -Object $row -Name "track_id") -eq $createdTrackID -and
            [string](Get-OptionalProperty -Object $row -Name "status") -ne "not_evaluable") {
            $ceilingReady = $true
            break
        }
    }
    if ($ceilingReady) {
        Write-Ok ("acoustic evidence reached TIM after " + $pollRounds + " round(s), observation_id=" + $finalObservationID)
        break
    }
    $timPayload | ConvertTo-Json -Depth 12 | Out-File -FilePath (Join-Path $RunRoot ("tim_projection_poll_round" + $pollRounds + ".json")) -Encoding utf8
    Write-Host ("   round " + $pollRounds + ": fixture track level_ceiling still not_evaluable; waiting " + $PollIntervalSeconds + "s")
    $timPayload = $null
    Start-Sleep -Seconds $PollIntervalSeconds
}

if ($null -eq $timPayload) {
    if ([string]::IsNullOrWhiteSpace($lastErrorStage)) { $lastErrorStage = "acoustic_timeout"; $lastErrorDetail = "fixture track acoustic never reached TIM within budget" }
    Add-Failure ("observation loop ended without usable TIM projection: stage=" + $lastErrorStage + " detail=" + $lastErrorDetail)
}
else {
    $report.fixture.observation_id = $finalObservationID
}

if ($null -ne $timPayload) {
    $timPayload | ConvertTo-Json -Depth 12 | Out-File -FilePath (Join-Path $RunRoot "tim_projection_final.json") -Encoding utf8

    # ------------------------------------------------------------ assertion group A1
    Write-Step "A1: assertions[] present and every row three-state legal"
    $assertions = @(Get-OptionalProperty -Object $timPayload -Name "assertions")
    if ($assertions.Count -eq 0) {
        Add-Failure "assertions[] missing or empty (assembly chain did not run -> per card stop condition, collect evidence)"
    }
    else {
        $legalStates = @("pass", "fail", "not_evaluable")
        $illegal = @($assertions | Where-Object { $legalStates -notcontains [string](Get-OptionalProperty -Object $_ -Name "status") })
        if ($illegal.Count -gt 0) {
            Add-Failure ("A1 illegal assertion status rows: " + ($illegal | ConvertTo-Json -Depth 4 -Compress))
        }
        else {
            Write-Ok ("A1 pass: " + $assertions.Count + " assertion rows, all three-state legal")
        }
    }
    $report.assertions_block.row_count = $assertions.Count

    # ------------------------------------------------------------ assertion group A2
    Write-Step "A2: fixture fail codes in assertions[] and risk_summary.primary_codes"
    $risk = Get-OptionalProperty -Object $timPayload -Name "risk_summary"
    $primaryCodes = @(Get-OptionalProperty -Object $risk -Name "primary_codes")
    foreach ($pair in @(
        @{ asserter = "level_ceiling"; check = "ceiling"; code = "assert_level_ceiling_exceeded"; label = "AS-PEAK" },
        @{ asserter = "signal_hygiene"; check = "clipping_headroom"; code = "possible_clipping_or_no_headroom"; label = "AS-SIG P2" }
    )) {
        $hit = @($assertions | Where-Object {
            [string](Get-OptionalProperty -Object $_ -Name "asserter") -eq $pair.asserter -and
            [string](Get-OptionalProperty -Object $_ -Name "check") -eq $pair.check -and
            [string](Get-OptionalProperty -Object $_ -Name "status") -eq "fail" -and
            [string](Get-OptionalProperty -Object $_ -Name "track_id") -eq $createdTrackID -and
            [string](Get-OptionalProperty -Object $_ -Name "code") -eq $pair.code
        })
        if ($hit.Count -eq 0) {
            Add-Failure ("A2 " + $pair.label + " fail row missing in assertions[] for track " + $createdTrackID + " (code=" + $pair.code + ")")
        }
        else {
            Write-Ok ("A2 " + $pair.label + " fail row present: " + ($hit[0] | ConvertTo-Json -Depth 4 -Compress))
        }
        if ($primaryCodes -notcontains $pair.code) {
            Add-Failure ("A2 " + $pair.label + " code " + $pair.code + " missing from risk_summary.primary_codes: " + ($primaryCodes -join ","))
        }
        else {
            Write-Ok ("A2 " + $pair.label + " code listed in risk_summary.primary_codes")
        }
    }
    $report.assertions_block.primary_codes = $primaryCodes

    # ------------------------------------------------------------ assertion group A3
    Write-Step "A3: agent log diff shows [tim.assert] WARN lines"
    $afterLines = @(Read-LogLines -Path $AgentLog)
    $newLines = @($afterLines | Where-Object { -not $baselineSet.Contains($_) })
    # Use .Contains, not -like: "[tim.assert]" is a wildcard character class
    # in -like patterns and would match nearly every line.
    $warnLines = @($newLines | Where-Object { $_.Contains("[tim.assert]") -and $_.Contains("[WARN]") })
    $report.log_diff.after_line_count = $afterLines.Count
    $report.log_diff.new_line_count = $newLines.Count
    $report.log_diff.tim_assert_warn_lines = $warnLines
    $warnLines | Out-File -FilePath (Join-Path $RunRoot "log_diff_tim_assert_lines.txt") -Encoding utf8
    if ($warnLines.Count -eq 0) {
        Add-Failure ("A3 no [tim.assert] WARN lines in agent log diff (new lines=" + $newLines.Count + ")")
    }
    else {
        $ceilingWarn = @($warnLines | Where-Object { $_ -like "*asserter=level_ceiling*" -and $_ -like "*status=fail*" -and $_ -like "*code=assert_level_ceiling_exceeded*" })
        if ($ceilingWarn.Count -eq 0) {
            Add-Failure ("A3 [tim.assert] WARN lines exist but none match level_ceiling fail: " + ($warnLines -join " | "))
        }
        else {
            Write-Ok ("A3 pass: " + $warnLines.Count + " [tim.assert] WARN line(s) in diff, level_ceiling fail matched")
            Write-Host ("   e.g. " + $ceilingWarn[0])
        }
    }
}

# ---------------------------------------------------------------- cleanup
if (-not [string]::IsNullOrWhiteSpace($createdTrackID)) {
    Write-Step "Cleanup: delete fixture track"
    try {
        $cleanupState = Invoke-AgentTool -Tool "project.state" -ToolArgs @{} -TimeoutSec 30 -Source "tim_assert_smoke.cleanup"
        $cleanupResult = Get-OptionalProperty -Object $cleanupState -Name "result"
        $userTrackCount = [int](Get-FirstPropertyValue -Object $cleanupResult -Names @("user_track_count"))
        if ($userTrackCount -le 1) {
            Write-WarnLine ("leaving fixture track (kernel refuses deleting the last audio track): " + $createdTrackID)
            $report.fixture.track_cleanup = "left_last_track"
        }
        else {
            $trackCleanup = Invoke-AgentTool -Tool "track.delete" -ToolArgs @{ track_id = $createdTrackID } -TimeoutSec 60 -Source "tim_assert_smoke.cleanup" -Confirmed
            $report.fixture.track_cleanup = [string](Get-OptionalProperty -Object $trackCleanup -Name "status")
            Write-Host ("   track.delete status=" + $report.fixture.track_cleanup)
        }
    }
    catch {
        Write-WarnLine ("cleanup failed: " + $_.Exception.Message)
        $report.fixture.track_cleanup = "failed"
    }
}

# ---------------------------------------------------------------- verdict
$report.finished_at = (Get-Date).ToUniversalTime().ToString("o")
if ($failureReasons.Count -eq 0) {
    $report.verdict = "PASS"
    $report | ConvertTo-Json -Depth 8 | Out-File -FilePath (Join-Path $RunRoot "run_report.json") -Encoding utf8
    Write-Step "PASS"
    Write-Ok ("TIM assert real-stack smoke passed (four assertion groups). run_root=" + $RunRoot)
    exit 0
}
$report.verdict = "FAIL"
$report.failures = @($failureReasons)
$report | ConvertTo-Json -Depth 8 | Out-File -FilePath (Join-Path $RunRoot "run_report.json") -Encoding utf8
Write-Step "FAIL"
foreach ($reason in $failureReasons) { Write-FailLine $reason }
Write-Host ("run_root=" + $RunRoot)
exit 1
