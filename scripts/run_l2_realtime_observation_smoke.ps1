param(
    [string]$RepoRoot = "D:\Vit_DAW",
    [string]$GodotProjectRoot = "D:\Godot\project\vit-daw-frontend",
    [string]$GodotExe = "D:\Godot\Godot_v4.6.1-stable_win64_console.exe",
    [string]$MaterialPath = "D:\Vit_DAW\test_100hz_10s.wav"
)

$ErrorActionPreference = "Stop"

function Fail {
    param([string]$Message)
    throw ("L2 realtime observation smoke failed: " + $Message)
}

function Get-Prop {
    param(
        [object]$Object,
        [string]$Name
    )
    if ($null -eq $Object) {
        return $null
    }
    $prop = $Object.PSObject.Properties[$Name]
    if ($null -eq $prop) {
        return $null
    }
    return $prop.Value
}

function Number-Value {
    param([object]$Value)
    if ($null -eq $Value) {
        return $null
    }
    try {
        return [double]$Value
    }
    catch {
        return $null
    }
}

$RepoRoot = (Resolve-Path -LiteralPath $RepoRoot).Path
$GodotProjectRoot = (Resolve-Path -LiteralPath $GodotProjectRoot).Path
if (-not (Test-Path -LiteralPath $GodotExe -PathType Leaf)) {
    Fail ("Godot executable not found: " + $GodotExe)
}

$smokeRoot = Join-Path $RepoRoot "VitApp\Workspace\Artifacts\smoke"
$stamp = Get-Date -Format "yyyyMMdd_HHmmss"
$artifactDir = Join-Path $smokeRoot ("l2_realtime_observation_" + $stamp)
$devRoot = Join-Path $artifactDir "dev_root"
$snapshotDir = Join-Path $devRoot "VitApp\Workspace\Artifacts"
$snapshotPath = Join-Path $snapshotDir "mixboard_feature_snapshot.json"
$readySnapshotPath = Join-Path $artifactDir "l2_ready_snapshot.json"
$probeScript = Join-Path $RepoRoot "scripts\l2_realtime_observation_probe.gd"
$godotLog = Join-Path $artifactDir "godot_l2_probe.log"

New-Item -ItemType Directory -Path $snapshotDir -Force | Out-Null

$seed = [ordered]@{
    schema_version = "mixboard_feature_snapshot.v1"
    updated_at = "2026-06-23T00:00:00Z"
    latest_request = [ordered]@{
        schema_version = "mixboard_feature_request.v1"
        request_id = "l2_probe_req"
        resolved_target = [ordered]@{
            track_id = "track_1"
            clip_id = "clip_1"
            source_path = $MaterialPath
            file_path = $MaterialPath
            source_revision = "rev_prepared"
            clip_revision = "clip_rev_prepared"
            duration_seconds = 10.0
        }
        requested_features = @(
            [ordered]@{ feature_type = "waveform_envelope"; request_id = "l2_probe_req"; track_id = "track_1"; clip_id = "clip_1" },
            [ordered]@{ feature_type = "spectral_field"; request_id = "l2_probe_req"; track_id = "track_1"; clip_id = "clip_1" }
        )
        updated_at = "2026-06-23T00:00:00Z"
    }
    waveform_envelope = [ordered]@{
        status = "ready"
        track_id = "track_1"
        clip_id = "clip_1"
        request_id = "l2_probe_req"
        source = "kernel_audio_feature_data_ready"
        source_revision = "rev_prepared"
        clip_revision = "clip_rev_prepared"
        rms = 0.2
        peak_abs = 0.7
        float_count = 128
    }
    spectrogram_tiles = [ordered]@{
        status = "ready"
        feature_type = "spectral_field"
        track_id = "track_1"
        clip_id = "clip_1"
        request_id = "l2_probe_req"
        source = "kernel_prepared_telemetry"
        source_revision = "rev_prepared"
        clip_revision = "clip_rev_prepared"
        tile_count_seen = 2
        tile_count_expected = 2
        tile_count_parsed = 2
        coverage_seconds = 10.0
    }
    spectrogram_tile_rows = @(
        [ordered]@{
            status = "ready"
            feature_type = "spectral_field"
            track_id = "track_1"
            clip_id = "clip_1"
            request_id = "l2_probe_req"
            source = "kernel_prepared_telemetry"
            source_revision = "rev_prepared"
            tile_count_seen = 2
            tile_count_expected = 2
            coverage_seconds = 10.0
        }
    )
    band_energy_summary = [ordered]@{
        status = "ready"
        feature_type = "band_energy_summary"
        track_id = "track_1"
        clip_id = "clip_1"
        request_id = "l2_probe_req"
        source = "spectral_tile_derived"
        materialized_by = "kernel_prepared_telemetry"
        source_revision = "rev_prepared"
        clip_revision = "clip_rev_prepared"
        tile_count_parsed = 2
        coverage_seconds = 10.0
        bands = [ordered]@{
            bass = [ordered]@{ unit_energy = 0.25; energy_db = -12.041; min_hz = 60; max_hz = 160 }
            air = [ordered]@{ unit_energy = 0.02; energy_db = -33.979; min_hz = 6000; max_hz = 16000 }
        }
    }
    band_energy_summaries = @(
        [ordered]@{
            status = "ready"
            feature_type = "band_energy_summary"
            track_id = "track_1"
            clip_id = "clip_1"
            request_id = "l2_probe_req"
            source = "spectral_tile_derived"
            materialized_by = "kernel_prepared_telemetry"
            source_revision = "rev_prepared"
            tile_count_parsed = 2
            coverage_seconds = 10.0
            bands = [ordered]@{ bass = [ordered]@{ unit_energy = 0.25; energy_db = -12.041 } }
        }
    )
    stereo_relation_summary = [ordered]@{
        status = "ready"
        feature_type = "stereo_relation_summary"
        track_id = "track_1"
        clip_id = "clip_1"
        request_id = "l2_probe_req"
        source = "spectral_tile_derived"
        materialized_by = "kernel_prepared_telemetry"
        source_revision = "rev_prepared"
        clip_revision = "clip_rev_prepared"
        tile_count_parsed = 2
        coverage_seconds = 10.0
        balance_db = 0.1
        balance_state = "centered"
        correlation_estimate = 0.82
        correlation_state = "stable"
    }
    stereo_relation_summaries = @(
        [ordered]@{
            status = "ready"
            feature_type = "stereo_relation_summary"
            track_id = "track_1"
            clip_id = "clip_1"
            request_id = "l2_probe_req"
            source = "spectral_tile_derived"
            materialized_by = "kernel_prepared_telemetry"
            source_revision = "rev_prepared"
            tile_count_parsed = 2
            coverage_seconds = 10.0
            balance_state = "centered"
            correlation_estimate = 0.82
        }
    )
}

$seed | ConvertTo-Json -Depth 16 | Set-Content -LiteralPath $snapshotPath -Encoding UTF8

$oldDevRoot = $env:VIT_DAW_DEV_ROOT
$oldSource = $env:VIT_DAW_L2_PROBE_SOURCE
$oldReadySnapshot = $env:VIT_DAW_L2_READY_SNAPSHOT_PATH
try {
    $env:VIT_DAW_DEV_ROOT = $devRoot
    $env:VIT_DAW_L2_PROBE_SOURCE = $MaterialPath
    $env:VIT_DAW_L2_READY_SNAPSHOT_PATH = $readySnapshotPath
    $cmdLine = '"' + $GodotExe + '" --headless --path "' + $GodotProjectRoot + '" --script "' + $probeScript + '" 2>&1'
    & cmd.exe /d /c $cmdLine | Tee-Object -FilePath $godotLog
    $godotExitCode = $LASTEXITCODE
    if ($godotExitCode -ne 0) {
        Fail ("Godot probe exited with code " + $godotExitCode + ". Log: " + $godotLog)
    }
}
finally {
    $env:VIT_DAW_DEV_ROOT = $oldDevRoot
    $env:VIT_DAW_L2_PROBE_SOURCE = $oldSource
    $env:VIT_DAW_L2_READY_SNAPSHOT_PATH = $oldReadySnapshot
}

if (-not (Test-Path -LiteralPath $snapshotPath -PathType Leaf)) {
    Fail ("snapshot was not written: " + $snapshotPath)
}
if (-not (Test-Path -LiteralPath $readySnapshotPath -PathType Leaf)) {
    Fail ("ready matrix snapshot was not written: " + $readySnapshotPath)
}

$snapshot = Get-Content -LiteralPath $snapshotPath -Raw | ConvertFrom-Json
$readySnapshot = Get-Content -LiteralPath $readySnapshotPath -Raw | ConvertFrom-Json
$band = Get-Prop $snapshot "band_energy_summary"
$stereo = Get-Prop $snapshot "stereo_relation_summary"
$readyL3Band = Get-Prop $readySnapshot "band_energy_summary"
$readyL3Stereo = Get-Prop $readySnapshot "stereo_relation_summary"
$liveBand = Get-Prop $snapshot "realtime_band_energy_summary"
$liveStereo = Get-Prop $snapshot "realtime_stereo_relation_summary"

if ([string](Get-Prop $readyL3Band "source") -ne "spectral_tile_derived") {
    Fail ("L3 band summary was overwritten during ready phase: " + ($readyL3Band | ConvertTo-Json -Depth 10 -Compress))
}
if ([string](Get-Prop $readyL3Stereo "source") -ne "spectral_tile_derived") {
    Fail ("L3 stereo summary was overwritten during ready phase: " + ($readyL3Stereo | ConvertTo-Json -Depth 10 -Compress))
}
if ([string](Get-Prop $readyL3Band "source_revision") -ne "rev_prepared") {
    Fail "L3 band source_revision was not preserved"
}

$liveBandRows = @((Get-Prop $readySnapshot "realtime_band_energy_summaries"))
$liveStereoRows = @((Get-Prop $readySnapshot "realtime_stereo_relation_summaries"))
$readyBand = $liveBandRows | Where-Object {
    [string](Get-Prop $_ "request_id") -eq "l2_probe_req" -and
    [string](Get-Prop $_ "status") -eq "ready"
} | Select-Object -First 1
$readyStereo = $liveStereoRows | Where-Object {
    [string](Get-Prop $_ "request_id") -eq "l2_probe_req" -and
    [string](Get-Prop $_ "status") -eq "ready"
} | Select-Object -First 1

if ($null -eq $readyBand -or [string](Get-Prop $readyBand "source") -ne "live_level_meter_spectrum") {
    Fail ("L2 realtime ready band summary missing from matrix rows: " + ($liveBandRows | ConvertTo-Json -Depth 10 -Compress))
}
if ($null -eq $readyStereo -or [string](Get-Prop $readyStereo "source") -ne "live_level_meter_stereo") {
    Fail ("L2 realtime ready stereo summary missing from matrix rows: " + ($liveStereoRows | ConvertTo-Json -Depth 10 -Compress))
}
foreach ($row in @($readyBand, $readyStereo)) {
    if ([string](Get-Prop $row "capture_mode") -ne "realtime_playback") {
        Fail ("L2 realtime row missing capture_mode=realtime_playback: " + ($row | ConvertTo-Json -Depth 10 -Compress))
    }
    if ([string](Get-Prop $row "tap_point") -ne "unknown_live_meter") {
        Fail ("L2 realtime row must not claim post_fx/post_fader tap point: " + ($row | ConvertTo-Json -Depth 10 -Compress))
    }
    $qe = Get-Prop $row "quality_evidence"
    if ($null -eq $qe -or [string](Get-Prop $qe "post_fx_verified") -ne "False" -or [string](Get-Prop $qe "post_fader_verified") -ne "False") {
        Fail ("L2 realtime row missing negative post-FX/post-fader evidence: " + ($row | ConvertTo-Json -Depth 10 -Compress))
    }
}

$liveBands = Get-Prop $readyBand "bands"
$liveBass = Number-Value (Get-Prop (Get-Prop $liveBands "bass") "unit_energy")
$liveAir = Number-Value (Get-Prop (Get-Prop $liveBands "air") "unit_energy")
if ($null -eq $liveBass -or $null -eq $liveAir -or $liveBass -le $liveAir) {
    Fail ("L2 band values do not show the injected bass energy. bass=" + $liveBass + " air=" + $liveAir)
}

if ([string](Get-Prop $liveBand "status") -eq "ready" -or [string](Get-Prop $liveStereo "status") -eq "ready") {
    Fail ("L2 realtime top-level rows stayed ready after playback stopped/source changed: band=" + ($liveBand | ConvertTo-Json -Depth 10 -Compress) + " stereo=" + ($liveStereo | ConvertTo-Json -Depth 10 -Compress))
}
if ([string](Get-Prop $band "status") -eq "ready" -or [string](Get-Prop $stereo "status") -eq "ready") {
    Fail ("L3 top-level rows stayed ready after source changed without new prepared capture: band=" + ($band | ConvertTo-Json -Depth 10 -Compress) + " stereo=" + ($stereo | ConvertTo-Json -Depth 10 -Compress))
}
foreach ($row in @($liveBand, $liveStereo)) {
    if ([string](Get-Prop $row "status") -ne "deferred") {
        Fail ("L2 realtime top-level row should be deferred after stopped playback: " + ($row | ConvertTo-Json -Depth 10 -Compress))
    }
    if ([string](Get-Prop $row "source_revision") -ne "rev_second_material") {
        Fail ("L2 realtime top-level row did not follow new source identity: " + ($row | ConvertTo-Json -Depth 10 -Compress))
    }
    if ([string](Get-Prop $row "tap_point") -ne "unknown_live_meter") {
        Fail ("deferred L2 row missing explicit tap_point: " + ($row | ConvertTo-Json -Depth 10 -Compress))
    }
}

foreach ($name in @("spectrogram_tile_rows", "band_energy_summaries", "stereo_relation_summaries", "realtime_band_energy_summaries", "realtime_stereo_relation_summaries")) {
    $rows = Get-Prop $snapshot $name
    if ($null -eq $rows -or @($rows).Count -lt 1) {
        Fail ($name + " was not preserved/written")
    }
}

Write-Host ("L2 realtime observation smoke passed. snapshot=" + $snapshotPath + " ready_snapshot=" + $readySnapshotPath)
