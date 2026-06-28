[CmdletBinding()]
param(
    [string]$RepoRoot = "D:\Vit_DAW",
    [string]$AgentHttp = "http://127.0.0.1:7878",
    [string]$AgentHttpAddr = "127.0.0.1:7878",
    [switch]$ReuseAgent,
    [switch]$SkipBuild,
    [switch]$KeepProcesses,
    [int]$StartupTimeoutSec = 45
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Write-Step { param([string]$Message) Write-Host ""; Write-Host ("== " + $Message) -ForegroundColor Cyan }
function Write-Ok { param([string]$Message) Write-Host ("ok: " + $Message) -ForegroundColor Green }
function Write-WarnLine { param([string]$Message) Write-Host ("warn: " + $Message) -ForegroundColor Yellow }
function Fail { param([string]$Message) throw $Message }

function Get-OptionalProperty {
    param([object]$Object, [string]$Name)
    if ($null -eq $Object -or [string]::IsNullOrWhiteSpace($Name)) { return $null }
    if ($Object -is [System.Collections.IDictionary] -and $Object.Contains($Name)) { return $Object[$Name] }
    $prop = $Object.PSObject.Properties[$Name]
    if ($null -eq $prop) { return $null }
    return $prop.Value
}

function Get-TcpListener {
    param([int]$Port)
    return Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue | Select-Object -First 1
}

function Wait-HttpReady {
    param([string]$BaseUrl, [int]$TimeoutSeconds)
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    do {
        try {
            $resp = Invoke-WebRequest -UseBasicParsing -Uri ($BaseUrl.TrimEnd("/") + "/health") -TimeoutSec 2
            if ($resp.StatusCode -ge 200 -and $resp.StatusCode -lt 500) { return $true }
        }
        catch {
            Start-Sleep -Milliseconds 500
        }
    } while ((Get-Date) -lt $deadline)
    return $false
}

function Invoke-Json {
    param(
        [ValidateSet("GET", "POST")] [string]$Method,
        [string]$Uri,
        [object]$Body = $null,
        [int]$TimeoutSec = 60
    )
    if ($Method -eq "GET") {
        $resp = Invoke-WebRequest -UseBasicParsing -Method GET -Uri $Uri -TimeoutSec $TimeoutSec
    }
    else {
        $json = $Body | ConvertTo-Json -Depth 30 -Compress
        $bytes = [System.Text.Encoding]::UTF8.GetBytes($json)
        $resp = Invoke-WebRequest -UseBasicParsing -Method POST -Uri $Uri -Body $bytes -ContentType "application/json; charset=utf-8" -TimeoutSec $TimeoutSec
    }
    if ([string]::IsNullOrWhiteSpace($resp.Content)) { return $null }
    return $resp.Content | ConvertFrom-Json
}

function Invoke-AgentTool {
    param([string]$Tool, [object]$ToolArgs = $null, [bool]$Confirmed = $true, [int]$TimeoutSec = 120)
    if ($null -eq $ToolArgs) { $ToolArgs = @{} }
    return Invoke-Json -Method POST -Uri ($AgentHttp.TrimEnd("/") + "/agent/invoke") -Body @{
        tool = $Tool
        args = $ToolArgs
        confirmed = $Confirmed
        source = "acoustic_source_identity_smoke"
    } -TimeoutSec $TimeoutSec
}

function ConvertTo-JsonFile {
    param([object]$Value, [string]$Path, [int]$Depth = 30)
    $json = $Value | ConvertTo-Json -Depth $Depth
    Set-Content -LiteralPath $Path -Value $json -Encoding UTF8
}

function Assert-NoPendingOrConfirmation {
    param([object]$Response, [string]$Label)
    $result = Get-OptionalProperty -Object $Response -Name "result"
    foreach ($container in @($Response, $result)) {
        if ($null -eq $container) { continue }
        foreach ($name in @("needs_confirmation", "requires_confirmation", "pending_candidate", "confirmation_card")) {
            $value = Get-OptionalProperty -Object $container -Name $name
            if ($null -ne $value -and ([string]$value).Trim() -notin @("", "False", "false", "0")) {
                Fail ($Label + " unexpectedly exposed " + $name)
            }
        }
    }
    $events = @()
    $events += @(Get-OptionalProperty -Object $Response -Name "typed_events")
    $events += @(Get-OptionalProperty -Object $result -Name "typed_events")
    foreach ($event in $events) {
        $eventType = [string](Get-OptionalProperty -Object $event -Name "event_type")
        $stateKind = [string](Get-OptionalProperty -Object $event -Name "state_kind")
        if ($eventType -in @("PendingCandidate", "ApprovalRequest") -or $stateKind -in @("PendingCandidate", "ApprovalRequest")) {
            Fail ($Label + " emitted confirmation/pending typed event")
        }
    }
}

function New-ReadOnlyPrompt {
    return -join ([int[]](0x89C2, 0x5BDF, 0x4E00, 0x4E0B, 0x5F53, 0x524D, 0x5DE5, 0x7A0B, 0x7684, 0x9891, 0x6BB5, 0x548C, 0x58F0, 0x50CF, 0x72B6, 0x6001, 0xFF0C, 0x4E0D, 0x8981, 0x6267, 0x884C, 0x4EFB, 0x4F55, 0x4FEE, 0x6539, 0x3002) | ForEach-Object { [char]$_ })
}

$agentProc = $null
$artifactDir = Join-Path $RepoRoot "VitApp\Workspace\Artifacts\smoke\acoustic_source_identity"
$featureSnapshotPath = Join-Path $artifactDir "mixboard_feature_snapshot.json"
$statusPath = Join-Path $artifactDir "acoustic_package_status.json"
$summaryPath = Join-Path $artifactDir "summary.json"
$httpPort = [int](($AgentHttpAddr -split ":")[-1])

try {
    Write-Step "Clear generated acoustic observation state"
    & (Join-Path $RepoRoot "scripts\clear_acoustic_observation_state.ps1") -RepoRoot $RepoRoot -IncludeSmokeObservationCache | Write-Host
    New-Item -ItemType Directory -Force -Path $artifactDir | Out-Null

    $agentExe = Join-Path $RepoRoot "agent\bin\VitAgent.exe"
    if (-not $SkipBuild -and -not $ReuseAgent) {
        Write-Step "Build VitAgent"
        Push-Location (Join-Path $RepoRoot "agent")
        try {
            go build -o ".\bin\VitAgent.exe" ".\cmd\vitagent"
        }
        finally {
            Pop-Location
        }
    }

    $listener = Get-TcpListener -Port $httpPort
    if ($ReuseAgent -or $null -ne $listener) {
        if ($null -ne $listener) {
            Write-Ok ("agent HTTP port already listening: " + $httpPort + " pid=" + $listener.OwningProcess)
        }
        else {
            Write-WarnLine "ReuseAgent was set but no listener was found yet"
        }
    }
    else {
        Write-Step "Start VitAgent"
        $agentLog = Join-Path $artifactDir "agent.log"
        $agentProc = Start-Process -FilePath $agentExe -ArgumentList @("-http", $AgentHttpAddr, "-last-log-path", $agentLog, "-keep-last-log-lines", "800") -WorkingDirectory (Join-Path $RepoRoot "agent") -PassThru -WindowStyle Hidden
    }
    if (-not (Wait-HttpReady -BaseUrl $AgentHttp -TimeoutSeconds $StartupTimeoutSec)) {
        Fail "agent HTTP endpoint did not become ready"
    }

    Write-Step "Seed stale 100Hz cache and current Paper Crown partial package"
    $paperPath = Join-Path $RepoRoot "Paper Crown.mp3"
    $oldPath = Join-Path $RepoRoot "test_100hz_10s.wav"
    if (-not (Test-Path -LiteralPath $paperPath)) { Fail ("missing material " + $paperPath) }
    if (-not (Test-Path -LiteralPath $oldPath)) { Fail ("missing old material " + $oldPath) }

    $featureSnapshot = @{
        schema_version = "mixboard_feature_snapshot.v1"
        updated_at = (Get-Date).ToUniversalTime().ToString("o")
        waveform_envelope = @{ status = "ready"; feature_type = "waveform_envelope"; project_id = "current"; session_id = "mix_source_identity"; track_id = "1007"; clip_id = "clip_a"; file_path = $paperPath; source_path = $paperPath; source_revision = "rev_paper"; duration_seconds = 219.0; peak_abs = 0.55; rms = 0.12; peak_dbfs = -5.19; rms_dbfs = -18.41; crest_db = 13.22 }
        track_waveform_envelopes = @(
            @{ status = "ready"; feature_type = "waveform_envelope"; project_id = "current"; session_id = "old_session"; track_id = "1007"; clip_id = "clip_a"; file_path = $oldPath; source_path = $oldPath; source_revision = "rev_100hz"; duration_seconds = 10.0; peak_dbfs = -6.02; rms_dbfs = -9.03; peak_abs = 0.5; rms = 0.35 },
            @{ status = "ready"; feature_type = "waveform_envelope"; project_id = "current"; session_id = "mix_source_identity"; track_id = "1007"; clip_id = "clip_a"; file_path = $paperPath; source_path = $paperPath; source_revision = "rev_paper"; duration_seconds = 219.0; peak_dbfs = -5.19; rms_dbfs = -18.41; peak_abs = 0.55; rms = 0.12 }
        )
        spectrogram_tiles = @{ status = "ready"; feature_type = "spectral_field"; project_id = "current"; session_id = "old_session"; track_id = "1007"; clip_id = "clip_a"; file_path = $oldPath; source_path = $oldPath; source_revision = "rev_100hz"; duration_seconds = 10.0; tile_count_seen = 2; tile_count_expected = 2; coverage_seconds = 10 }
        spectrogram_tile_rows = @(
            @{ status = "partial"; feature_type = "spectral_field"; project_id = "current"; session_id = "mix_source_identity"; track_id = "1007"; clip_id = "clip_a"; file_path = $paperPath; source_path = $paperPath; source_revision = "rev_paper"; duration_seconds = 219.0; tile_count_seen = 12; tile_count_expected = 101; coverage_seconds = 60; coverage_ratio = 0.274 }
        )
        band_energy_summary = @{ status = "ready"; project_id = "current"; session_id = "old_session"; track_id = "1007"; clip_id = "clip_a"; file_path = $oldPath; source_path = $oldPath; source_revision = "rev_100hz"; duration_seconds = 10.0; bands = @{ bass = @{ energy_db = -6.02; unit_energy = 0.5 } } }
        band_energy_summaries = @(
            @{ status = "partial"; project_id = "current"; session_id = "mix_source_identity"; track_id = "1007"; clip_id = "clip_a"; file_path = $paperPath; source_path = $paperPath; source_revision = "rev_paper"; duration_seconds = 219.0; coverage_seconds = 60; coverage_ratio = 0.274; bands = @{ bass = @{ status = "partial"; energy_db = -18.2; unit_energy = 0.12 }; mid = @{ status = "partial"; energy_db = -13.4; unit_energy = 0.21 } } }
        )
        stereo_relation_summary = @{ status = "ready"; project_id = "current"; session_id = "old_session"; track_id = "1007"; clip_id = "clip_a"; file_path = $oldPath; source_path = $oldPath; source_revision = "rev_100hz"; duration_seconds = 10.0; balance_db = 0; correlation_estimate = 1.0; correlation_state = "stable" }
        stereo_relation_summaries = @(
            @{ status = "partial"; project_id = "current"; session_id = "mix_source_identity"; track_id = "1007"; clip_id = "clip_a"; file_path = $paperPath; source_path = $paperPath; source_revision = "rev_paper"; duration_seconds = 219.0; coverage_seconds = 60; coverage_ratio = 0.274; balance_db = 0.3; balance_state = "centered"; correlation_estimate = 0.72; correlation_state = "stable" }
        )
    }
    ConvertTo-JsonFile -Value $featureSnapshot -Path $featureSnapshotPath

    Write-Step "Invoke read-only frequency/stereo observation"
    $observe = Invoke-AgentTool -Tool "mix.observe" -ToolArgs @{
        mix_session_id = "mix_source_identity"
        scope = "selected_track"
        track_id = "1007"
        clip_id = "clip_a"
        file_path = $paperPath
        source_revision = "rev_paper"
        duration_seconds = 219.0
        feature_snapshot_path = $featureSnapshotPath
        acoustic_package_status_path = $statusPath
        projection = "frequency_stereo"
        include_raw = $false
        feature_keys = @("band_energy_summary", "stereo_relation_summary", "spectrogram_tiles", "acoustic_package_status", "source_identity")
        observation_only = $true
        mutation_barrier = $true
        no_pending = $true
        goal_text = (New-ReadOnlyPrompt)
    } -TimeoutSec 180

    $status = [string](Get-OptionalProperty -Object $observe -Name "status")
    if ($status -ne "ok") { Fail ("mix.observe failed: " + ($observe | ConvertTo-Json -Depth 12 -Compress)) }
    Assert-NoPendingOrConfirmation -Response $observe -Label "source identity read-only observe"

    $result = Get-OptionalProperty -Object $observe -Name "result"
    $package = Get-OptionalProperty -Object $result -Name "acoustic_package_status"
    if ([string](Get-OptionalProperty -Object $package -Name "schema_version") -ne "acoustic_package_status.v0") {
        Fail "missing acoustic_package_status.v0"
    }
    if ([string](Get-OptionalProperty -Object $package -Name "source_revision") -ne "rev_paper") {
        Fail ("current source_revision mismatch: " + ($package | ConvertTo-Json -Depth 20 -Compress))
    }

    $json = $result | ConvertTo-Json -Depth 40 -Compress
    foreach ($forbidden in @("test_100hz_10s.wav", "rev_100hz", "-6.02", "-9.03")) {
        if ($json.Contains($forbidden)) { Fail ("old source evidence leaked into observe result: " + $forbidden) }
    }
    foreach ($required in @("acoustic_package_status.v0", "rev_paper", "band_energy_summary", "stereo_relation_summary", "spectrogram_tiles")) {
        if (-not $json.Contains($required)) { Fail ("observe result missing " + $required) }
    }
    foreach ($forbiddenRawKey in @('"time_segments":', '"track_waveform_envelopes":', '"spectrogram_tile_rows":')) {
        if ($json.Contains($forbiddenRawKey)) {
            Fail ("projection leaked raw acoustic payload: " + $forbiddenRawKey)
        }
    }

    $summary = [pscustomobject]@{
        schema_version = "acoustic_source_identity_smoke.v0"
        status = "ok"
        artifact_dir = $artifactDir
        feature_snapshot_path = $featureSnapshotPath
        acoustic_package_status_path = $statusPath
        source_revision = [string](Get-OptionalProperty -Object $package -Name "source_revision")
        package_status = [string](Get-OptionalProperty -Object $package -Name "status")
        updated_at = (Get-Date).ToUniversalTime().ToString("o")
    }
    ConvertTo-JsonFile -Value $summary -Path $summaryPath
    Write-Ok ("source identity smoke passed; summary=" + $summaryPath)
}
finally {
    if ($null -ne $agentProc -and -not $KeepProcesses) {
        try { Stop-Process -Id $agentProc.Id -Force -ErrorAction SilentlyContinue } catch {}
    }
}
