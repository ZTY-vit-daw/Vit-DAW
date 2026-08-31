#Requires -Version 5.1
# D2-KREV1 kernel revision visibility probe (2026-08-31).
#
# Drives a real VitApp kernel over VSP (through the VspHub HTTP relay) and
# observes whether the VSP revision counter advances after external-plugin
# parameter writes. One public fixture case per run:
#   spv1_p04  FabFilter Pro-DS      (chunk-stable for param writes -> the wall)
#   spv1_p03  Vertigo VSC-2         (chunk-variable -> today's advancing class)
#   spv1_p01  Plugin Alliance bx_hybrid V2 (advance source class unknown)
#
# Measured sequence (identical in every mode):
#   settle -> S0 -> S1 -> W1(write v1) -> NOOP(write v1 again)
#          -> W2(restore v0) -> S2
# with a state.snapshot after every step. Checks:
#   locks (must hold pre- and post-fix): S1==S0, NOOP==W1, S2==W2
#   gates (post-fix expectation):        W1>S1, W2>NOOP
#
# Modes:
#   default            judge locks + gates; exit 1 on any failure
#   -RecordBaseline p  run the sequence, persist the revision sequence, judge
#                      locks only (gates recorded as data); exit 0 iff the
#                      mechanics and locks hold
#   -ReplayBaseline p  judge locks + digit-for-digit equality of the r0-anchored
#                      revision sequence against the recorded baseline; exit 4
#                      on replay mismatch (stop-card signal, no interpretive
#                      pass)
[CmdletBinding()]
param(
    [string]$RepoRoot = "D:\Vit_DAW",
    [ValidateSet("spv1_p01", "spv1_p03", "spv1_p04")]
    [string]$PublicCaseId = "spv1_p04",
    [string]$AgentHttp = "http://127.0.0.1:7878",
    [string]$HubUrl = "http://127.0.0.1:8787/vsp",
    [string]$GodotProjectRoot = "D:\Godot\project\vit-daw-frontend",
    [string]$KernelExe = "",
    [int]$TimeoutSeconds = 420,
    [switch]$SkipBuild,
    [string]$RecordBaseline = "",
    [string]$ReplayBaseline = ""
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$RepoRoot = (Resolve-Path -LiteralPath $RepoRoot).Path
if (-not [string]::IsNullOrWhiteSpace($RecordBaseline) -and -not [string]::IsNullOrWhiteSpace($ReplayBaseline)) {
    throw "record and replay baseline modes are mutually exclusive"
}

# Per-case plugin binding mirrors the local experiment whitelist
# (C:\Users\timoz\.vit\free_state_experiment_plugins.json); the probe writes
# the same first parameter the D1 treatment path writes. InsertMode mirrors
# each case's REAL E2E insertion path (kernel log 2026-08-31_20-06-22):
# p04's plugin-grabber chain inserts via rack_add_node (rack-contained ->
# param state lives in the edit-level RACKS subtree -> the revision wall),
# while p01/p03's D1 paths insert via instantiate_plugin (direct track slot
# -> param state inside the track tree -> writes advance pre-fix already).
$CaseConfig = @{
    spv1_p01 = @{ Identifier = "VST3-bx_hybrid V2-d0ef306f-c141eb4b";  ParamId = "827092295";   Label = "bx_hybrid-V2-band315-gain-ch1"; InsertMode = "direct" }
    spv1_p03 = @{ Identifier = "VST3-Vertigo VSC-2-7e4e7243-aaeea0d";  ParamId = "1416131121";  Label = "VSC-2-threshold-ch1";           InsertMode = "direct" }
    spv1_p04 = @{ Identifier = "VST3-Pro-DS-831d2251-cba24108";        ParamId = "1";           Label = "Pro-DS-threshold";              InsertMode = "rack" }
}
$case = $CaseConfig[$PublicCaseId]

function Fail {
    param([string]$Message)
    throw ("KREV1 probe [" + $PublicCaseId + "] failed: " + $Message)
}

function Wait-HttpReady {
    param([string]$Url, [int]$Seconds)
    $deadline = (Get-Date).AddSeconds($Seconds)
    while ((Get-Date) -lt $deadline) {
        try {
            Invoke-RestMethod -Uri $Url -Method Get -TimeoutSec 5 | Out-Null
            return $true
        }
        catch {
            Start-Sleep -Milliseconds 500
        }
    }
    return $false
}

function Wait-Port {
    param([int]$Port, [int]$Seconds)
    $deadline = (Get-Date).AddSeconds($Seconds)
    while ((Get-Date) -lt $deadline) {
        $listener = Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue | Select-Object -First 1
        if ($null -ne $listener) {
            return $true
        }
        Start-Sleep -Milliseconds 500
    }
    return $false
}

$script:vspMessageCounter = 0
function New-VspEnvelope {
    param([string]$SessionId, [string]$Role, [string]$Channel, [string]$Type, [hashtable]$Payload)
    $script:vspMessageCounter++
    return @{
        vsp_version = "1.0"
        schema = ("vsp." + $Type + ".v1")
        message_id = ("msg_krev1_probe_" + [Guid]::NewGuid().ToString("N"))
        session_id = $SessionId
        client_id = "krev1.kernel.probe"
        role = $Role
        channel = $Channel
        type = $Type
        created_at = [DateTime]::UtcNow.ToString("o")
        trace_id = ("trace_krev1_" + [Guid]::NewGuid().ToString("N"))
        payload = $Payload
    }
}

function Invoke-Vsp {
    param([hashtable]$Envelope)
    $body = $Envelope | ConvertTo-Json -Depth 32 -Compress
    $response = Invoke-WebRequest -UseBasicParsing -Uri $HubUrl -Method Post -ContentType "application/json" -Body $body -TimeoutSec $TimeoutSeconds
    return ($response.Content | ConvertFrom-Json)
}

function Require-ReplyType {
    param([object]$Reply, [string]$ExpectedType, [string]$Label)
    if ([string]$Reply.type -ne $ExpectedType) {
        Fail ($Label + " returned type " + [string]$Reply.type + ": " + ($Reply | ConvertTo-Json -Depth 16 -Compress))
    }
}

# --- session (established after the hub relay is up, below) ----------------
function Send-Snapshot {
    param([string]$Label)
    $envelope = New-VspEnvelope -SessionId $sessionId -Role "controller" -Channel "state" -Type "state.snapshot_request" -Payload @{
        scope = "project.timeline"
    }
    $reply = Invoke-Vsp -Envelope $envelope
    Require-ReplyType -Reply $reply -ExpectedType "state.snapshot" -Label ("state.snapshot_request(" + $Label + ")")
    if ([string]$reply.payload.status -ne "ok") {
        Fail ("state.snapshot(" + $Label + ") payload was not ok: " + ($reply | ConvertTo-Json -Depth 8 -Compress))
    }
    return [double][string]$reply.revision
}

function Send-Legacy {
    param([string]$Cmd, [hashtable]$LegacyArgs, [string]$Label)
    $envelope = New-VspEnvelope -SessionId $sessionId -Role "controller" -Channel "command" -Type "command.request" -Payload @{
        command = "legacy.command"
        legacy = @{ cmd = $Cmd; args = $LegacyArgs }
    }
    $reply = Invoke-Vsp -Envelope $envelope
    Require-ReplyType -Reply $reply -ExpectedType "command.response" -Label ("legacy.command(" + $Label + ")")
    if ([string]$reply.ack.stage -ne "completed") {
        Fail ($Label + " was rejected: " + ($reply | ConvertTo-Json -Depth 16 -Compress))
    }
    $legacyReply = $reply.payload.legacy_reply
    if ($null -eq $legacyReply -or [string]$legacyReply.status -ne "ok") {
        Fail ($Label + " legacy reply was not ok: " + ($reply.payload | ConvertTo-Json -Depth 16 -Compress))
    }
    return $legacyReply
}

function Send-SetParamsBatch {
    param([string]$PluginId, [string]$TrackId, [string]$ParamId, [double]$NormalizedValue, [string]$Label)
    $envelope = New-VspEnvelope -SessionId $sessionId -Role "controller" -Channel "command" -Type "command.request" -Payload @{
        command = "plugin.set_params_batch"
        args = @{
            plugin_id = $PluginId
            track_id = $TrackId
            parameters = @(
                @{ parameter_id = $ParamId; normalized_value = $NormalizedValue }
            )
        }
    }
    $reply = Invoke-Vsp -Envelope $envelope
    Require-ReplyType -Reply $reply -ExpectedType "command.response" -Label ("set_params_batch(" + $Label + ")")
    if ([string]$reply.ack.stage -ne "completed" -or [string]$reply.payload.status -ne "ok") {
        Fail ($Label + " write failed: " + ($reply | ConvertTo-Json -Depth 16 -Compress))
    }
}

# --- materialize fixture project -------------------------------------------
$fixtureRoot = Join-Path $RepoRoot "temp\semantic-processor-agent-project-smoke-v1\fixtures\semantic_processor_project_smoke_v1_80085263a651cf20"
$manifestPath = Join-Path $fixtureRoot "fixture_manifest.json"
if (-not (Test-Path -LiteralPath $manifestPath -PathType Leaf)) {
    Fail "fixture manifest is missing: $manifestPath"
}
$manifest = Get-Content -LiteralPath $manifestPath -Raw | ConvertFrom-Json
$manifestCase = @($manifest.cases) | Where-Object { [string]$_.public_case_id -eq $PublicCaseId } | Select-Object -First 1
if ($null -eq $manifestCase) {
    Fail ("manifest has no case " + $PublicCaseId)
}
$stamp = Get-Date -Format "yyyyMMdd_HHmmss"
$artifactDir = Join-Path $RepoRoot ("artifacts\free_state_d1_s1\krev1_probe_" + $PublicCaseId + "_" + $stamp)
New-Item -ItemType Directory -Path $artifactDir -Force | Out-Null
$workdir = Join-Path $artifactDir "project"
# mirror the D1 runner's materialize_public_case: copy the .vit file's whole
# directory (project-relative assets ride along), never the read-only fixture
$sourceProject = [string]$manifestCase.project_path
Copy-Item -LiteralPath (Split-Path -Parent $sourceProject) -Destination $workdir -Recurse
$projectPath = Join-Path $workdir (Split-Path -Leaf $sourceProject)
if (-not (Test-Path -LiteralPath $projectPath -PathType Leaf)) {
    Fail "copied fixture project is missing: $projectPath"
}

# --- boot the real stack -----------------------------------------------------
if ([string]::IsNullOrWhiteSpace($KernelExe)) {
    $KernelExe = Join-Path $RepoRoot "VitApp\build\VitApp_artefacts\Release\VitApp.exe"
}
if (-not $SkipBuild) {
    $kernelListener = Get-NetTCPConnection -LocalPort 5555 -State Listen -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($null -ne $kernelListener) {
        $kernelProcess = Get-CimInstance Win32_Process -Filter ("ProcessId=" + $kernelListener.OwningProcess) -ErrorAction SilentlyContinue
        if ($null -eq $kernelProcess -or $kernelProcess.Name -ne "VitApp.exe" -or -not ([string]$kernelProcess.ExecutablePath).StartsWith($RepoRoot, [System.StringComparison]::OrdinalIgnoreCase)) {
            throw "port 5555 is owned by a process outside the Vit-DAW workspace"
        }
        Stop-Process -Id $kernelListener.OwningProcess -Force
        Start-Sleep -Milliseconds 800
    }
    & cmake --build (Join-Path $RepoRoot "VitApp\build") --config Release --target VitApp --parallel 4
    if ($LASTEXITCODE -ne 0) {
        throw "current VitApp Release build failed"
    }
}
$KernelExe = (Resolve-Path -LiteralPath $KernelExe).Path

$hubProcess = $null
$startedListenerPids = @()
try {
    $devArgs = @(
        "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", (Join-Path $RepoRoot "scripts\dev_agent_smoke.ps1"),
        "-RepoRoot", $RepoRoot, "-AgentHttp", $AgentHttp, "-RestartAgent", "-StartKernel", "-KernelExe", $KernelExe, "-StartUI",
        "-GodotProjectRoot", $GodotProjectRoot, "-NoChatSmoke", "-NoStripSilenceSmoke", "-WaitSeconds", ([string]$TimeoutSeconds)
    )
    if ($SkipBuild) {
        $devArgs += "-SkipBuild"
    }
    & powershell @devArgs
    if ($LASTEXITCODE -ne 0) {
        Fail ("real-stack startup failed with exit code " + $LASTEXITCODE)
    }
    foreach ($port in @(5555, 5556, 7878)) {
        if (-not (Wait-Port -Port $port -Seconds 120)) {
            Fail ("expected stack port did not become ready: " + $port)
        }
    }
    foreach ($port in @(5555, 5556, 7878)) {
        $listener = Get-NetTCPConnection -LocalPort $port -State Listen -ErrorAction SilentlyContinue | Select-Object -First 1
        if ($null -ne $listener) {
            $startedListenerPids += [int]$listener.OwningProcess
        }
    }

    # --- start the VSP hub relay (connects the running kernel's ZMQ endpoints)
    $hubExe = Join-Path $RepoRoot "agent\bin\VspHub.exe"
    Push-Location (Join-Path $RepoRoot "agent")
    try {
        & go build -o $hubExe ./cmd/vsphub
        if ($LASTEXITCODE -ne 0) {
            Fail "VspHub build failed"
        }
    }
    finally {
        Pop-Location
    }
    $hubLog = Join-Path $artifactDir "vsp_hub_last.log"
    $hubProcess = Start-Process -FilePath $hubExe -ArgumentList @("-last-log-path", $hubLog) -WorkingDirectory (Join-Path $RepoRoot "agent") -WindowStyle Hidden -PassThru
    if (-not (Wait-HttpReady -Url ($HubUrl.TrimEnd("/") -replace "/vsp$", "/health") -Seconds 60)) {
        Fail "VspHub did not become ready"
    }

    # --- VSP session over the running hub ------------------------------------
    $hello = New-VspEnvelope -SessionId "session_pending" -Role "controller" -Channel "session" -Type "session.hello" -Payload @{
        client_name = "KREV1 Kernel Revision Probe"
        client_version = "krev1-probe-v1"
        protocol_min = "1.0"
        protocol_max = "1.0"
        wants = @("command.request", "state.snapshot")
        transport_bindings = @("vsp.hub.http")
    }
    $helloReply = Invoke-Vsp -Envelope $hello
    Require-ReplyType -Reply $helloReply -ExpectedType "session.hello_ack" -Label "session.hello"
    $sessionId = [string]$helloReply.session_id
    if ([string]::IsNullOrWhiteSpace($sessionId)) {
        Fail "session.hello did not return a session_id"
    }

    # --- open the fixture project -------------------------------------------
    $null = Send-Legacy -Cmd "open_project" -LegacyArgs @{ file_path = $projectPath } -Label "open_project"

    # --- discover the first audio track --------------------------------------
    $discovery = $null
    $trackId = ""
    foreach ($attempt in 1..10) {
        $envelope = New-VspEnvelope -SessionId $sessionId -Role "controller" -Channel "state" -Type "state.snapshot_request" -Payload @{ scope = "project.timeline" }
        $reply = Invoke-Vsp -Envelope $envelope
        Require-ReplyType -Reply $reply -ExpectedType "state.snapshot" -Label "state.snapshot_request(discovery)"
        $discovery = $reply
        foreach ($row in @($reply.payload.tracks)) {
            if ([string]$row.is_audio_track -eq "True" -and -not [string]::IsNullOrWhiteSpace([string]$row.track_id)) {
                $trackId = [string]$row.track_id
                break
            }
        }
        if (-not [string]::IsNullOrWhiteSpace($trackId)) {
            break
        }
        Start-Sleep -Milliseconds 500
    }
    if ([string]::IsNullOrWhiteSpace($trackId)) {
        Fail "no audio track found in the opened project"
    }

    # --- insert the target plugin through the case's E2E insertion path.
    # rack_add_node (the plugin-grabber load, p04): plugin ends up INSIDE the
    # rack type whose ValueTree lives under the edit-level RACKS node, outside
    # every track subtree track_state_revision hashes - the revision wall.
    # instantiate_plugin (the D1 paths, p01/p03): direct track slot, plugin
    # state inside the track tree. The first probe draft instantiated p04
    # directly and the write ADVANCED pre-fix (3->4->5, run 20260831_205510),
    # which is how the insertion-path differentiator was isolated.
    # Retries absorb async project load.
    $pluginId = ""
    $lastInstantiateError = ""
    foreach ($attempt in 1..10) {
        try {
            if ([string]$case.InsertMode -eq "rack") {
                $insertReply = Send-Legacy -Cmd "rack_add_node" -LegacyArgs @{
                    track_id = $trackId
                    plugin_identifier = [string]$case.Identifier
                    x = 400
                    y = 300
                } -Label ("rack_add_node-" + $attempt)
            }
            else {
                $insertReply = Send-Legacy -Cmd "instantiate_plugin" -LegacyArgs @{
                    track_id = $trackId
                    plugin_identifier = [string]$case.Identifier
                } -Label ("instantiate_plugin-" + $attempt)
            }
            $pluginId = [string]$insertReply.plugin_id
            if (-not [string]::IsNullOrWhiteSpace($pluginId)) {
                break
            }
            $lastInstantiateError = "plugin insertion did not return a plugin_id"
        }
        catch {
            $lastInstantiateError = [string]$_.Exception.Message
        }
        Start-Sleep -Seconds 1
    }
    if ([string]::IsNullOrWhiteSpace($pluginId)) {
        Fail ("plugin insertion did not succeed: " + $lastInstantiateError)
    }

    # --- read the target parameter's current normalized value ----------------
    $paramRow = $null
    $paramsReply = $null
    $lastParamsError = ""
    foreach ($attempt in 1..10) {
        try {
            $paramsReply = Send-Legacy -Cmd "get_plugin_parameters" -LegacyArgs @{
                track_id = $trackId
                plugin_id = $pluginId
            } -Label ("get_plugin_parameters-" + $attempt)
            $paramRow = @($paramsReply.parameters) | Where-Object { [string]$_.param_id -eq [string]$case.ParamId } | Select-Object -First 1
            if ($null -ne $paramRow) {
                break
            }
            $lastParamsError = ("target parameter not exposed yet: param_id=" + $case.ParamId)
        }
        catch {
            $lastParamsError = [string]$_.Exception.Message
        }
        Start-Sleep -Seconds 1
    }
    if ($null -eq $paramRow) {
        Fail ("target parameter was not exposed by the plugin: " + $lastParamsError)
    }
    $v0 = [double][string]$paramRow.normalized_value
    # v1/v2 must both stay in [0.05,0.95] and differ from v0 and each other.
    # VSC-2's threshold sits at the normalized ceiling (v0=1.0), so the two
    # distinct steps walk in ONE direction instead of straddling v0. W2 is the
    # decisive step: a second DISTINCT write distinguishes per-write
    # visibility from one-shot state (e.g. an automation-deviation blob
    # appearing once) - the E2E wall shape is "later distinct writes stop
    # advancing".
    $offset = 0.15
    $v1 = $null
    $v2 = $null
    if ($v0 - (2 * $offset) -ge 0.05) {
        $v1 = $v0 - $offset
        $v2 = $v0 - (2 * $offset)
    }
    elseif ($v0 + (2 * $offset) -le 0.95) {
        $v1 = $v0 + $offset
        $v2 = $v0 + (2 * $offset)
    }
    if ($null -eq $v1) {
        Fail ("probe could not derive distinct write values around v0=" + $v0)
    }

    # --- settle: background project load (audio analysis, asset sync, default
    # settings) must stop moving the revision before the measured sequence ---
    $settleRev = Send-Snapshot -Label "settle"
    $settled = $false
    foreach ($attempt in 1..60) {
        Start-Sleep -Seconds 1
        $next = Send-Snapshot -Label ("settle-" + $attempt)
        if ($next -eq $settleRev) {
            $settled = $true
            break
        }
        $settleRev = $next
    }
    if (-not $settled) {
        Fail ("revision never settled after project load (last=" + $settleRev + ")")
    }

    # --- measured sequence ----------------------------------------------------
    $rWarm = $settleRev
    $rS0 = Send-Snapshot -Label "S0"
    $rS1 = Send-Snapshot -Label "S1"
    Send-SetParamsBatch -PluginId $pluginId -TrackId $trackId -ParamId ([string]$case.ParamId) -NormalizedValue $v1 -Label "W1"
    $rW1 = Send-Snapshot -Label "W1"
    Send-SetParamsBatch -PluginId $pluginId -TrackId $trackId -ParamId ([string]$case.ParamId) -NormalizedValue $v1 -Label "NOOP"
    $rNoop = Send-Snapshot -Label "NOOP"
    Send-SetParamsBatch -PluginId $pluginId -TrackId $trackId -ParamId ([string]$case.ParamId) -NormalizedValue $v2 -Label "W2"
    $rW2 = Send-Snapshot -Label "W2"
    Send-SetParamsBatch -PluginId $pluginId -TrackId $trackId -ParamId ([string]$case.ParamId) -NormalizedValue $v0 -Label "W3-restore"
    $rW3 = Send-Snapshot -Label "W3"
    $rFinal = Send-Snapshot -Label "S2"

    $sequence = @($rWarm, $rS0, $rS1, $rW1, $rNoop, $rW2, $rW3, $rFinal)
    $names = @("settle", "S0", "S1", "W1_write", "NOOP_write", "W2_write_distinct", "W3_restore", "S2")
    $deltas = @()
    foreach ($value in $sequence) {
        $deltas += ($value - $sequence[1])
    }
    $locksOk = ($rS1 -eq $rS0) -and ($rNoop -eq $rW1) -and ($rFinal -eq $rW3)
    $firstWriteAdvanced = ($rW1 -gt $rS1)
    $secondWriteAdvanced = ($rW2 -gt $rNoop)
    $restoreAdvanced = ($rW3 -gt $rW2)

    $observations = @()
    for ($i = 0; $i -lt $sequence.Count; $i++) {
        $observations += @{
            step = $names[$i]
            revision = [int64]$sequence[$i]
            delta_vs_S0 = [int64]$deltas[$i]
        }
    }
    $report = @{
        schema_version = "vit.krev1_kernel_param_revision_probe.v1"
        public_case_id = $PublicCaseId
        plugin_label = [string]$case.Label
        plugin_identifier = [string]$case.Identifier
        plugin_id = $pluginId
        track_id = $trackId
        param_id = [string]$case.ParamId
        normalized_v0 = $v0
        normalized_v1 = $v1
        normalized_v2 = $v2
        sequence = $observations
        deltas_vs_s0 = @($deltas | ForEach-Object { [int64]$_ })
        locks_ok = [bool]$locksOk
        first_write_advanced = [bool]$firstWriteAdvanced
        second_write_advanced = [bool]$secondWriteAdvanced
        restore_advanced = [bool]$restoreAdvanced
        mode = if ($RecordBaseline -ne "") { "record" } elseif ($ReplayBaseline -ne "") { "replay" } else { "gate" }
        artifact_dir = $artifactDir
        recorded_at = [DateTime]::UtcNow.ToString("o")
    }
    $reportPath = Join-Path $artifactDir "probe_report.json"
    $report | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $reportPath -Encoding UTF8

    if (-not $locksOk) {
        Write-Host ($report | ConvertTo-Json -Depth 8 -Compress)
        Fail ("sequence locks failed (S1==S0, NOOP==W1, S2==W3 expected): " + ($deltas -join ","))
    }

    if ($RecordBaseline -ne "") {
        $report | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $RecordBaseline -Encoding UTF8
        Write-Host ("KREV1 BASELINE RECORDED [" + $PublicCaseId + "]: deltas=" + ($deltas -join ",") + " first_write_advanced=" + $firstWriteAdvanced + " second_write_advanced=" + $secondWriteAdvanced + " -> " + $RecordBaseline) -ForegroundColor Yellow
        exit 0
    }

    if ($ReplayBaseline -ne "") {
        $baseline = Get-Content -LiteralPath $ReplayBaseline -Raw | ConvertFrom-Json
        $baselineDeltas = @($baseline.deltas_vs_s0 | ForEach-Object { [int64]$_ })
        if ($baselineDeltas.Count -ne $deltas.Count) {
            Fail ("replay shape mismatch: baseline has " + $baselineDeltas.Count + " entries, run has " + $deltas.Count)
        }
        $mismatches = @()
        for ($i = 0; $i -lt $deltas.Count; $i++) {
            if ([int64]$baselineDeltas[$i] -ne [int64]$deltas[$i]) {
                $mismatches += ($names[$i] + ": baseline=" + $baselineDeltas[$i] + " run=" + $deltas[$i])
            }
        }
        if ($mismatches.Count -gt 0) {
            Write-Host ("KREV1 REPLAY MISMATCH [" + $PublicCaseId + "]: " + ($mismatches -join "; ")) -ForegroundColor Red
            exit 4
        }
        Write-Host ("KREV1 REPLAY PASS [" + $PublicCaseId + "]: revision sequence digit-identical to baseline") -ForegroundColor Green
        exit 0
    }

    if (-not $firstWriteAdvanced) {
        Write-Host ("KREV1 GATE RED: first parameter write did not advance the VSP revision (S1=" + $rS1 + " W1=" + $rW1 + ")") -ForegroundColor Red
        exit 1
    }
    if (-not $secondWriteAdvanced) {
        Write-Host ("KREV1 GATE RED: second distinct write did not advance the VSP revision (NOOP=" + $rNoop + " W2=" + $rW2 + ") - one-shot visibility, the E2E wall shape") -ForegroundColor Red
        exit 1
    }
    if (-not $restoreAdvanced) {
        Write-Host ("KREV1 GATE RED: restore write did not advance the VSP revision (W2=" + $rW2 + " W3=" + $rW3 + ")") -ForegroundColor Red
        exit 1
    }
    Write-Host ("KREV1 GATE PASS [" + $PublicCaseId + "]: writes advanced revision " + $rS1 + " -> " + $rW1 + " -> " + $rW2 + " -> " + $rW3 + "; locks held") -ForegroundColor Green
    exit 0
}
finally {
    if ($null -ne $hubProcess -and -not $hubProcess.HasExited) {
        Stop-Process -Id $hubProcess.Id -Force -ErrorAction SilentlyContinue
    }
    # stop the stack this probe started: kernel, agent and any UI child the
    # boot spawned inside the workspace; foreign listeners are never touched.
    foreach ($port in @(8787, 7878, 5556, 5555)) {
        $listeners = Get-NetTCPConnection -LocalPort $port -State Listen -ErrorAction SilentlyContinue
        foreach ($listener in @($listeners)) {
            $pidToStop = [int]$listener.OwningProcess
            if ($startedListenerPids -notcontains $pidToStop -and $port -ne 8787) {
                continue
            }
            $process = Get-CimInstance Win32_Process -Filter ("ProcessId=" + $pidToStop) -ErrorAction SilentlyContinue
            if ($null -ne $process -and ([string]$process.ExecutablePath).StartsWith($RepoRoot, [System.StringComparison]::OrdinalIgnoreCase)) {
                Stop-Process -Id $pidToStop -Force -ErrorAction SilentlyContinue
            }
            elseif ($null -ne $process -and $port -eq 8787 -and $process.Name -eq "VspHub.exe") {
                Stop-Process -Id $pidToStop -Force -ErrorAction SilentlyContinue
            }
        }
    }
    Write-Host ("KREV1 probe artifacts: " + $artifactDir)
}
