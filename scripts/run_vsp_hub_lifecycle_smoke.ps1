#requires -Version 5.1

[CmdletBinding()]
param(
    [string]$RepoRoot = "D:\Vit_DAW",
    [string]$GodotExe = "",
    [string]$GodotProjectRoot = "",
    [string]$KernelExe = "",
    [string]$AgentExe = "",
    [string]$VspHubExe = "",
    [switch]$ReuseGodot,
    [switch]$ReuseAgent,
    [switch]$ReuseHub,
    [switch]$ReuseKernel,
    [switch]$SkipBuild,
    [switch]$KeepProcesses,
    [switch]$SkipHubHttpAssetSmoke,
    [switch]$Run60TrackPlaybackSmoke,
    [int]$PlaybackTrackCount = 60,
    [switch]$KeepPlaybackImportedTracksForGuiProbe,
    [string]$CleanupPlaybackImportSummaryPath = "",
    [switch]$RunGuiHealthPlaybackProbe,
    [int]$GuiHealthPlaybackSeconds = 15,
    [int]$GuiHealthMinTrackCount = -1,
    [switch]$GuiHealthInputAudit,
    [int]$GuiHealthInputAuditIntervalSeconds = 3,
    [int]$GuiHealthInputAuditMinClicks = 1,
    [string]$GuiHealthInputAuditActions = "stop_return_to_start,loop,return_to_zero",
    [string]$GuiHealthPlaybackLayout = "default",
    [string]$GuiHealthPlaybackDynamicMode = "steady",
    [switch]$RunGuiArchitectureAuditProbe,
    [int]$GuiArchitectureAuditSeconds = 8,
    [int]$GuiArchitectureAuditMinTrackCount = -1,
    [string]$GuiArchitectureAuditLayout = "full_daw",
    [string]$GuiArchitectureAuditDynamicMode = "steady",
    [switch]$GuiArchitectureAuditInputAudit,
    [int]$GuiArchitectureAuditInputAuditIntervalSeconds = 3,
    [int]$GuiArchitectureAuditInputAuditMinClicks = 1,
    [string]$GuiArchitectureAuditInputAuditActions = "stop_return_to_start,loop,return_to_zero",
    [string]$GuiArchitectureAuditProfiles = "",
    [string]$GuiArchitectureAuditRackFixture = "",
    [int]$GuiArchitectureAuditRackFixturePluginCount = 48,
    [int]$GuiArchitectureAuditRackFixtureConnectionCount = 72,
    [int]$TimeoutSeconds = 60
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$AgentHttp = "http://127.0.0.1:7878"
$AgentHttpPort = 7878
$VspHubHttp = "http://127.0.0.1:8787"
$VspHubPort = 8787
$ZmqReqPort = 5555
$ZmqSubPort = 5556

function Write-Step {
    param([string]$Message)
    Write-Host ""
    Write-Host ("== " + $Message) -ForegroundColor Cyan
}

function Write-Ok {
    param([string]$Message)
    Write-Host ("ok: " + $Message) -ForegroundColor Green
}

function Write-WarnLine {
    param([string]$Message)
    Write-Host ("warn: " + $Message) -ForegroundColor Yellow
}

function Fail {
    param([string]$Message)
    throw $Message
}

function ConvertTo-JsonFile {
    param(
        [object]$Value,
        [string]$Path,
        [int]$Depth = 24
    )
    $json = $Value | ConvertTo-Json -Depth $Depth
    Set-Content -LiteralPath $Path -Value $json -Encoding UTF8
}

function Invoke-Json {
    param(
        [ValidateSet("GET", "POST")]
        [string]$Method,
        [string]$Uri,
        [object]$Body = $null,
        [int]$TimeoutSec = 30
    )
    if ($Method -eq "GET") {
        $resp = Invoke-WebRequest -UseBasicParsing -Method GET -Uri $Uri -TimeoutSec $TimeoutSec
    }
    else {
        $json = $Body | ConvertTo-Json -Depth 24 -Compress
        $bytes = [System.Text.Encoding]::UTF8.GetBytes($json)
        $resp = Invoke-WebRequest -UseBasicParsing -Method POST -Uri $Uri -Body $bytes -ContentType "application/json; charset=utf-8" -TimeoutSec $TimeoutSec
    }
    if ([string]::IsNullOrWhiteSpace($resp.Content)) {
        return $null
    }
    return $resp.Content | ConvertFrom-Json
}

function New-VspSmokeId {
    param([string]$Prefix)
    return ($Prefix + "_" + ([guid]::NewGuid().ToString("N")))
}

function Invoke-VspLifecycleCommandRequest {
    param(
        [string]$HubUrl,
        [string]$SessionId,
        [string]$Command,
        [object]$CommandArgs = @{},
        [object]$Legacy = $null,
        [int]$TimeoutSeconds,
        [int]$TimeoutMs = 120000
    )
    $payload = [ordered]@{
        command = $Command
    }
    if ($Command -eq "legacy.command") {
        $payload["legacy"] = $Legacy
    }
    else {
        $payload["args"] = $CommandArgs
    }
    $body = [ordered]@{
        vsp_version = "1.0"
        schema = "vsp.command.request.v1"
        message_id = New-VspSmokeId -Prefix "msg"
        session_id = $SessionId
        client_id = "smoke.vsp.lifecycle_cleanup"
        role = "gui"
        channel = "command"
        type = "command.request"
        created_at = (Get-Date).ToUniversalTime().ToString("o")
        trace_id = New-VspSmokeId -Prefix "trace"
        request_id = New-VspSmokeId -Prefix "req"
        transaction_id = New-VspSmokeId -Prefix "tx"
        command_timeout_ms = $TimeoutMs
        payload = $payload
    }
    $reply = Invoke-Json -Method POST -Uri $HubUrl -Body $body -TimeoutSec $TimeoutSeconds
    if ($null -eq $reply -or [string]$reply.type -ne "command.response") {
        Fail ("VSP cleanup command returned unexpected reply for " + $Command + ": " + ($reply | ConvertTo-Json -Depth 12 -Compress))
    }
    $payload = $reply.payload
    $legacyReply = $null
    if ($null -ne $payload) {
        $legacyReply = $payload.legacy_reply
    }
    $legacyStatus = ""
    if ($null -ne $legacyReply) {
        $legacyStatus = [string]$legacyReply.status
    }
    if (-not [string]::IsNullOrWhiteSpace($legacyStatus) -and $legacyStatus -notin @("ok", "success")) {
        Fail ("VSP cleanup command failed for " + $Command + ": " + ($reply | ConvertTo-Json -Depth 12 -Compress))
    }
    return $reply
}

function Test-VspTransientCleanupError {
    param([string]$Message)
    $lower = ([string]$Message).ToLowerInvariant()
    return $lower.Contains("502") `
        -or $lower.Contains("bad gateway") `
        -or $lower.Contains("timeout") `
        -or $lower.Contains("kernel gateway") `
        -or $lower.Contains("kernel_unavailable")
}

function Invoke-VspLifecycleCommandRequestWithRetry {
    param(
        [string]$HubUrl,
        [string]$SessionId,
        [string]$Command,
        [object]$CommandArgs = @{},
        [object]$Legacy = $null,
        [int]$TimeoutSeconds,
        [int]$TimeoutMs = 120000,
        [int]$RetryCount = 3
    )
    $attempt = 0
    while ($true) {
        try {
            return Invoke-VspLifecycleCommandRequest -HubUrl $HubUrl -SessionId $SessionId -Command $Command -CommandArgs $CommandArgs -Legacy $Legacy -TimeoutSeconds $TimeoutSeconds -TimeoutMs $TimeoutMs
        }
        catch {
            $attempt += 1
            $message = [string]$_.Exception.Message
            if ($attempt -gt $RetryCount -or -not (Test-VspTransientCleanupError -Message $message)) {
                throw
            }
            Start-Sleep -Milliseconds ([Math]::Min(2500, 350 * $attempt))
        }
    }
}

function Split-List {
    param(
        [object[]]$Items,
        [int]$Size
    )
    $chunks = @()
    if ($null -eq $Items -or $Items.Count -eq 0) {
        return $chunks
    }
    $chunkSize = [Math]::Max(1, $Size)
    for ($i = 0; $i -lt $Items.Count; $i += $chunkSize) {
        $end = [Math]::Min($Items.Count - 1, $i + $chunkSize - 1)
        $chunks += ,@($Items[$i..$end])
    }
    return $chunks
}

function Get-SafeFileNameSegment {
    param([string]$Value)
    $safe = ([string]$Value).Trim().ToLowerInvariant()
    if ([string]::IsNullOrWhiteSpace($safe)) {
        $safe = "profile"
    }
    $safe = [regex]::Replace($safe, "[^a-z0-9._-]+", "_")
    $safe = $safe.Trim("._-")
    if ([string]::IsNullOrWhiteSpace($safe)) {
        $safe = "profile"
    }
    return $safe
}

function New-GuiArchitectureAuditProfile {
    param(
        [string]$Name,
        [string]$Layout,
        [string]$DynamicMode,
        [int]$DurationSeconds,
        [int]$MinTrackCount,
        [bool]$InputAudit = $false,
        [int]$InputAuditIntervalSeconds = 3,
        [int]$InputAuditMinClicks = 1,
        [string]$InputAuditActions = "stop_return_to_start,loop,return_to_zero",
        [string]$RackFixture = "",
        [int]$RackFixturePluginCount = 0,
        [int]$RackFixtureConnectionCount = 0
    )
    $cleanLayout = ([string]$Layout).Trim()
    $cleanDynamicMode = ([string]$DynamicMode).Trim()
    $cleanName = ([string]$Name).Trim()
    if ([string]::IsNullOrWhiteSpace($cleanName)) {
        $cleanName = $cleanLayout + "_" + $cleanDynamicMode
    }
    return [ordered]@{
        name = $cleanName
        safe_name = Get-SafeFileNameSegment -Value $cleanName
        layout = $cleanLayout
        dynamic_mode = $cleanDynamicMode
        duration_seconds = [Math]::Max(1, $DurationSeconds)
        min_track_count = $MinTrackCount
        input_audit = [bool]$InputAudit
        input_audit_interval_seconds = [Math]::Max(1, $InputAuditIntervalSeconds)
        input_audit_min_clicks = [Math]::Max(0, $InputAuditMinClicks)
        input_audit_actions = ([string]$InputAuditActions).Trim()
        rack_fixture = ([string]$RackFixture).Trim()
        rack_fixture_plugin_count = [Math]::Max(0, $RackFixturePluginCount)
        rack_fixture_connection_count = [Math]::Max(0, $RackFixtureConnectionCount)
    }
}

function Get-GuiArchitectureAuditPresetProfile {
    param(
        [string]$Name,
        [int]$DefaultDurationSeconds,
        [int]$DefaultMinTrackCount,
        [bool]$DefaultInputAudit,
        [int]$DefaultInputAuditIntervalSeconds,
        [int]$DefaultInputAuditMinClicks,
        [string]$DefaultInputAuditActions,
        [string]$DefaultRackFixture,
        [int]$DefaultRackFixturePluginCount,
        [int]$DefaultRackFixtureConnectionCount
    )
    $clean = ([string]$Name).Trim().ToLowerInvariant()
    switch ($clean) {
        "full_daw_steady" {
            return New-GuiArchitectureAuditProfile -Name $clean -Layout "full_daw" -DynamicMode "steady" -DurationSeconds $DefaultDurationSeconds -MinTrackCount $DefaultMinTrackCount -InputAudit $DefaultInputAudit -InputAuditIntervalSeconds $DefaultInputAuditIntervalSeconds -InputAuditMinClicks $DefaultInputAuditMinClicks -InputAuditActions $DefaultInputAuditActions
        }
        "full_daw_scroll_zoom" {
            return New-GuiArchitectureAuditProfile -Name $clean -Layout "full_daw" -DynamicMode "timeline_scroll_zoom" -DurationSeconds $DefaultDurationSeconds -MinTrackCount $DefaultMinTrackCount -InputAudit $DefaultInputAudit -InputAuditIntervalSeconds $DefaultInputAuditIntervalSeconds -InputAuditMinClicks $DefaultInputAuditMinClicks -InputAuditActions $DefaultInputAuditActions
        }
        "full_daw_transport_input_audit" {
            return New-GuiArchitectureAuditProfile -Name $clean -Layout "full_daw" -DynamicMode "timeline_scroll_zoom" -DurationSeconds ([Math]::Max($DefaultDurationSeconds, 20)) -MinTrackCount $DefaultMinTrackCount -InputAudit $true -InputAuditIntervalSeconds $DefaultInputAuditIntervalSeconds -InputAuditMinClicks ([Math]::Max($DefaultInputAuditMinClicks, 3)) -InputAuditActions $DefaultInputAuditActions
        }
        "full_daw_time_scroll_zoom" {
            return New-GuiArchitectureAuditProfile -Name $clean -Layout "full_daw" -DynamicMode "timeline_time_scroll_zoom" -DurationSeconds $DefaultDurationSeconds -MinTrackCount $DefaultMinTrackCount -InputAudit $DefaultInputAudit -InputAuditIntervalSeconds $DefaultInputAuditIntervalSeconds -InputAuditMinClicks $DefaultInputAuditMinClicks -InputAuditActions $DefaultInputAuditActions
        }
        "mixer_detail_steady" {
            return New-GuiArchitectureAuditProfile -Name $clean -Layout "mixer_detail" -DynamicMode "steady" -DurationSeconds $DefaultDurationSeconds -MinTrackCount $DefaultMinTrackCount -InputAudit $DefaultInputAudit -InputAuditIntervalSeconds $DefaultInputAuditIntervalSeconds -InputAuditMinClicks $DefaultInputAuditMinClicks -InputAuditActions $DefaultInputAuditActions
        }
        "mixer_detail_scroll_zoom" {
            return New-GuiArchitectureAuditProfile -Name $clean -Layout "mixer_detail" -DynamicMode "timeline_scroll_zoom" -DurationSeconds $DefaultDurationSeconds -MinTrackCount $DefaultMinTrackCount -InputAudit $DefaultInputAudit -InputAuditIntervalSeconds $DefaultInputAuditIntervalSeconds -InputAuditMinClicks $DefaultInputAuditMinClicks -InputAuditActions $DefaultInputAuditActions
        }
        "graph_rack_pan_zoom" {
            return New-GuiArchitectureAuditProfile -Name $clean -Layout "full_daw" -DynamicMode "rack_pan_zoom" -DurationSeconds $DefaultDurationSeconds -MinTrackCount $DefaultMinTrackCount -InputAudit $DefaultInputAudit -InputAuditIntervalSeconds $DefaultInputAuditIntervalSeconds -InputAuditMinClicks $DefaultInputAuditMinClicks -InputAuditActions $DefaultInputAuditActions -RackFixture $DefaultRackFixture -RackFixturePluginCount $DefaultRackFixturePluginCount -RackFixtureConnectionCount $DefaultRackFixtureConnectionCount
        }
        "graph_rack_heavy_pan_zoom" {
            return New-GuiArchitectureAuditProfile -Name $clean -Layout "full_daw" -DynamicMode "rack_pan_zoom" -DurationSeconds $DefaultDurationSeconds -MinTrackCount $DefaultMinTrackCount -InputAudit $DefaultInputAudit -InputAuditIntervalSeconds $DefaultInputAuditIntervalSeconds -InputAuditMinClicks $DefaultInputAuditMinClicks -InputAuditActions $DefaultInputAuditActions -RackFixture "synthetic_heavy" -RackFixturePluginCount ([Math]::Max($DefaultRackFixturePluginCount, 72)) -RackFixtureConnectionCount ([Math]::Max($DefaultRackFixtureConnectionCount, 120))
        }
        "long_graph_rack_heavy_pan_zoom" {
            return New-GuiArchitectureAuditProfile -Name $clean -Layout "full_daw" -DynamicMode "rack_pan_zoom" -DurationSeconds ([Math]::Max($DefaultDurationSeconds, 20)) -MinTrackCount $DefaultMinTrackCount -InputAudit $DefaultInputAudit -InputAuditIntervalSeconds $DefaultInputAuditIntervalSeconds -InputAuditMinClicks $DefaultInputAuditMinClicks -InputAuditActions $DefaultInputAuditActions -RackFixture "synthetic_heavy" -RackFixturePluginCount ([Math]::Max($DefaultRackFixturePluginCount, 96)) -RackFixtureConnectionCount ([Math]::Max($DefaultRackFixtureConnectionCount, 180))
        }
        "long_full_daw_scroll_zoom" {
            return New-GuiArchitectureAuditProfile -Name $clean -Layout "full_daw" -DynamicMode "timeline_scroll_zoom" -DurationSeconds ([Math]::Max($DefaultDurationSeconds, 20)) -MinTrackCount $DefaultMinTrackCount -InputAudit $DefaultInputAudit -InputAuditIntervalSeconds $DefaultInputAuditIntervalSeconds -InputAuditMinClicks $DefaultInputAuditMinClicks -InputAuditActions $DefaultInputAuditActions
        }
    }
    return $null
}

function ConvertFrom-GuiArchitectureAuditProfiles {
    param(
        [string]$ProfilesText,
        [int]$DefaultDurationSeconds,
        [int]$DefaultMinTrackCount,
        [string]$DefaultLayout,
        [string]$DefaultDynamicMode,
        [bool]$DefaultInputAudit,
        [int]$DefaultInputAuditIntervalSeconds,
        [int]$DefaultInputAuditMinClicks,
        [string]$DefaultInputAuditActions,
        [string]$DefaultRackFixture,
        [int]$DefaultRackFixturePluginCount,
        [int]$DefaultRackFixtureConnectionCount
    )
    $profiles = @()
    if ([string]::IsNullOrWhiteSpace($ProfilesText)) {
        $profiles += New-GuiArchitectureAuditProfile -Name ($DefaultLayout + "_" + $DefaultDynamicMode) -Layout $DefaultLayout -DynamicMode $DefaultDynamicMode -DurationSeconds $DefaultDurationSeconds -MinTrackCount $DefaultMinTrackCount -InputAudit $DefaultInputAudit -InputAuditIntervalSeconds $DefaultInputAuditIntervalSeconds -InputAuditMinClicks $DefaultInputAuditMinClicks -InputAuditActions $DefaultInputAuditActions -RackFixture $DefaultRackFixture -RackFixturePluginCount $DefaultRackFixturePluginCount -RackFixtureConnectionCount $DefaultRackFixtureConnectionCount
        return $profiles
    }

    $items = @($ProfilesText -split "[,;]" | ForEach-Object { ([string]$_).Trim() } | Where-Object { -not [string]::IsNullOrWhiteSpace($_) })
    if ($items.Count -eq 0) {
        Fail "GuiArchitectureAuditProfiles was supplied but did not contain any profile names"
    }
    foreach ($item in $items) {
        $preset = Get-GuiArchitectureAuditPresetProfile -Name $item -DefaultDurationSeconds $DefaultDurationSeconds -DefaultMinTrackCount $DefaultMinTrackCount -DefaultInputAudit $DefaultInputAudit -DefaultInputAuditIntervalSeconds $DefaultInputAuditIntervalSeconds -DefaultInputAuditMinClicks $DefaultInputAuditMinClicks -DefaultInputAuditActions $DefaultInputAuditActions -DefaultRackFixture $DefaultRackFixture -DefaultRackFixturePluginCount $DefaultRackFixturePluginCount -DefaultRackFixtureConnectionCount $DefaultRackFixtureConnectionCount
        if ($null -ne $preset) {
            $profiles += $preset
            continue
        }

        $parts = @()
        if ($item.Contains("|")) {
            $parts = @($item -split "\|" | ForEach-Object { ([string]$_).Trim() })
        }
        else {
            $parts = @($item -split ":" | ForEach-Object { ([string]$_).Trim() })
        }
        if ($parts.Count -lt 2) {
            Fail ("Invalid GUI architecture audit profile '" + $item + "'. Use a preset name or layout:dynamic[:seconds[:min_tracks]] / name:layout:dynamic[:seconds[:min_tracks]].")
        }

        $name = ""
        $layout = ""
        $dynamic = ""
        $duration = $DefaultDurationSeconds
        $minTracks = $DefaultMinTrackCount
        $rackFixture = $DefaultRackFixture
        $rackFixturePluginCount = $DefaultRackFixturePluginCount
        $rackFixtureConnectionCount = $DefaultRackFixtureConnectionCount
        $parsedDuration = 0
        if ($parts.Count -eq 2) {
            $layout = [string]$parts[0]
            $dynamic = [string]$parts[1]
            $name = $layout + "_" + $dynamic
        }
        elseif ($parts.Count -ge 3 -and [int]::TryParse([string]$parts[2], [ref]$parsedDuration)) {
            $layout = [string]$parts[0]
            $dynamic = [string]$parts[1]
            $name = $layout + "_" + $dynamic
            $duration = $parsedDuration
            if ($parts.Count -ge 4 -and -not [string]::IsNullOrWhiteSpace([string]$parts[3])) {
                $minTracks = [int]([string]$parts[3])
            }
            if ($parts.Count -ge 5 -and -not [string]::IsNullOrWhiteSpace([string]$parts[4])) {
                $rackFixture = [string]$parts[4]
            }
            if ($parts.Count -ge 6 -and -not [string]::IsNullOrWhiteSpace([string]$parts[5])) {
                $rackFixturePluginCount = [int]([string]$parts[5])
            }
            if ($parts.Count -ge 7 -and -not [string]::IsNullOrWhiteSpace([string]$parts[6])) {
                $rackFixtureConnectionCount = [int]([string]$parts[6])
            }
        }
        else {
            $name = [string]$parts[0]
            $layout = [string]$parts[1]
            $dynamic = [string]$parts[2]
            if ($parts.Count -ge 4 -and -not [string]::IsNullOrWhiteSpace([string]$parts[3])) {
                $duration = [int]([string]$parts[3])
            }
            if ($parts.Count -ge 5 -and -not [string]::IsNullOrWhiteSpace([string]$parts[4])) {
                $minTracks = [int]([string]$parts[4])
            }
            if ($parts.Count -ge 6 -and -not [string]::IsNullOrWhiteSpace([string]$parts[5])) {
                $rackFixture = [string]$parts[5]
            }
            if ($parts.Count -ge 7 -and -not [string]::IsNullOrWhiteSpace([string]$parts[6])) {
                $rackFixturePluginCount = [int]([string]$parts[6])
            }
            if ($parts.Count -ge 8 -and -not [string]::IsNullOrWhiteSpace([string]$parts[7])) {
                $rackFixtureConnectionCount = [int]([string]$parts[7])
            }
        }
        $profiles += New-GuiArchitectureAuditProfile -Name $name -Layout $layout -DynamicMode $dynamic -DurationSeconds $duration -MinTrackCount $minTracks -InputAudit $DefaultInputAudit -InputAuditIntervalSeconds $DefaultInputAuditIntervalSeconds -InputAuditMinClicks $DefaultInputAuditMinClicks -InputAuditActions $DefaultInputAuditActions -RackFixture $rackFixture -RackFixturePluginCount $rackFixturePluginCount -RackFixtureConnectionCount $rackFixtureConnectionCount
    }

    $seen = @{}
    foreach ($profile in $profiles) {
        $baseSafeName = [string]$profile.safe_name
        $count = 1
        if ($seen.ContainsKey($baseSafeName)) {
            $count = [int]$seen[$baseSafeName] + 1
        }
        $seen[$baseSafeName] = $count
        if ($count -gt 1) {
            $profile["safe_name"] = $baseSafeName + "_" + [string]$count
        }
    }
    return $profiles
}

function Invoke-VspPlaybackImportCleanup {
    param(
        [string]$HubUrl,
        [object]$PlaybackSummary,
        [string]$ArtifactDir,
        [int]$TimeoutSeconds,
        [switch]$Strict
    )
    $result = [ordered]@{
        attempted = $false
        status = "skipped"
        session_id = ""
        clip_count = 0
        track_count = 0
        tracks_deleted = 0
        tracks_missing = 0
        clips_removed = $false
        clips_missing = $false
        error = ""
    }
    if ($null -eq $PlaybackSummary) {
        $result.error = "missing_playback_summary"
        return $result
    }
    $sessionId = ""
    if ($PlaybackSummary.PSObject.Properties.Name -contains "session_id") {
        $sessionId = [string]$PlaybackSummary.session_id
    }
    $trackValues = @()
    $clipValues = @()
    if ($PlaybackSummary.PSObject.Properties.Name -contains "created_track_ids") {
        $trackValues = @($PlaybackSummary.created_track_ids)
    }
    if ($PlaybackSummary.PSObject.Properties.Name -contains "created_clip_ids") {
        $clipValues = @($PlaybackSummary.created_clip_ids)
    }
    $trackIds = @($trackValues | ForEach-Object { [string]$_ } | Where-Object { -not [string]::IsNullOrWhiteSpace($_) })
    $clipIds = @($clipValues | ForEach-Object { [string]$_ } | Where-Object { -not [string]::IsNullOrWhiteSpace($_) })
    $result.session_id = $sessionId
    $result.track_count = [int]$trackIds.Count
    $result.clip_count = [int]$clipIds.Count
    if ([string]::IsNullOrWhiteSpace($sessionId) -or $trackIds.Count -eq 0) {
        $result.error = "missing_session_or_track_ids"
        return $result
    }

    $result.attempted = $true
    $cleanupPath = Join-Path $ArtifactDir "vsp_hub_playback_import_cleanup.json"
    try {
        $timeoutMs = [Math]::Max(60000, $TimeoutSeconds * 1000)
        if ($clipIds.Count -gt 0) {
            try {
                foreach ($clipBatch in (Split-List -Items $clipIds -Size 32)) {
                    [void](Invoke-VspLifecycleCommandRequestWithRetry -HubUrl $HubUrl -SessionId $sessionId -Command "legacy.command" -Legacy ([ordered]@{
                        cmd = "remove_clips"
                        args = [ordered]@{ clip_ids = @($clipBatch) }
                    }) -TimeoutSeconds $TimeoutSeconds -TimeoutMs $timeoutMs -RetryCount 5)
                    Start-Sleep -Milliseconds 50
                }
                $result.clips_removed = $true
            }
            catch {
                $clipCleanupError = [string]$_.Exception.Message
                if ($clipCleanupError.ToLowerInvariant().Contains("not found") -or $clipCleanupError.ToLowerInvariant().Contains("missing")) {
                    $result.clips_missing = $true
                }
                else {
                    throw
                }
            }
        }
        foreach ($trackId in $trackIds) {
            try {
                [void](Invoke-VspLifecycleCommandRequestWithRetry -HubUrl $HubUrl -SessionId $sessionId -Command "track.delete" -CommandArgs ([ordered]@{
                    track_id = $trackId
                }) -TimeoutSeconds $TimeoutSeconds -TimeoutMs $timeoutMs -RetryCount 5)
                $result.tracks_deleted = [int]$result.tracks_deleted + 1
                Start-Sleep -Milliseconds 35
            }
            catch {
                $trackCleanupError = [string]$_.Exception.Message
                if ($trackCleanupError.ToLowerInvariant().Contains("not found") -or $trackCleanupError.ToLowerInvariant().Contains("missing")) {
                    $result.tracks_missing = [int]$result.tracks_missing + 1
                    continue
                }
                throw
            }
        }
        $result.status = "ok"
        ConvertTo-JsonFile -Value $result -Path $cleanupPath -Depth 16
        return $result
    }
    catch {
        $result.status = "failed"
        $result.error = [string]$_.Exception.Message
        ConvertTo-JsonFile -Value $result -Path $cleanupPath -Depth 16
        if ($Strict) {
            Fail ("Failed to cleanup retained playback imports: " + [string]$_.Exception.Message)
        }
        Write-WarnLine ("cleanup retained playback imports failed: " + [string]$_.Exception.Message)
        return $result
    }
}

function Get-TcpListener {
    param([int]$Port)
    foreach ($line in @(& netstat -ano -p tcp 2>$null)) {
        $text = ([string]$line).Trim()
        if (-not $text.StartsWith("TCP", [System.StringComparison]::OrdinalIgnoreCase)) {
            continue
        }
        $parts = @($text -split "\s+" | Where-Object { -not [string]::IsNullOrWhiteSpace($_) })
        if ($parts.Count -lt 5) {
            continue
        }
        $local = [string]$parts[1]
        $state = [string]$parts[3]
        if ($state -ine "LISTENING") {
            continue
        }
        $colon = $local.LastIndexOf(":")
        if ($colon -lt 0 -or $colon -ge ($local.Length - 1)) {
            continue
        }
        $portText = $local.Substring($colon + 1)
        $parsedPort = 0
        if (-not [int]::TryParse($portText, [ref]$parsedPort)) {
            continue
        }
        if ($parsedPort -eq $Port) {
            $address = $local.Substring(0, $colon).Trim("[", "]")
            return [pscustomobject]@{
                LocalAddress = $address
                LocalPort = $parsedPort
                OwningProcess = [int]$parts[4]
            }
        }
    }
    return $null
}

function Normalize-ComparablePath {
    param([string]$Path)
    if ([string]::IsNullOrWhiteSpace($Path)) {
        return ""
    }
    $normalized = $Path.Trim().Trim('"')
    try {
        $normalized = [System.IO.Path]::GetFullPath($normalized)
    }
    catch {
    }
    return $normalized.Replace("/", "\").TrimEnd("\").ToLowerInvariant()
}

function Resolve-FirstExistingPath {
    param(
        [string[]]$Candidates,
        [string]$Label
    )
    foreach ($candidate in $Candidates) {
        if (-not [string]::IsNullOrWhiteSpace($candidate) -and (Test-Path -LiteralPath $candidate)) {
            return (Resolve-Path -LiteralPath $candidate).Path
        }
    }
    Fail ("Could not find " + $Label + ". Tried: " + ($Candidates -join "; "))
}

function Get-CommandLinePathArg {
    param(
        [string]$CommandLine,
        [string]$Name
    )
    if ([string]::IsNullOrWhiteSpace($CommandLine) -or [string]::IsNullOrWhiteSpace($Name)) {
        return ""
    }
    $pattern = "(?i)(?:^|\s)" + [regex]::Escape($Name) + "\s+(""([^""]+)""|'([^']+)'|([^\s]+))"
    $match = [regex]::Match($CommandLine, $pattern)
    if (-not $match.Success) {
        return ""
    }
    foreach ($index in @(2, 3, 4)) {
        if ($match.Groups[$index].Success -and -not [string]::IsNullOrWhiteSpace($match.Groups[$index].Value)) {
            return $match.Groups[$index].Value
        }
    }
    return ""
}

function Find-GodotProjectProcesses {
    param(
        [string]$ProjectRoot,
        [bool]$RuntimeOnly = $false
    )
    $projectKey = Normalize-ComparablePath -Path $ProjectRoot
    $rows = @()
    $processes = Get-CimInstance Win32_Process -ErrorAction SilentlyContinue | Where-Object {
        $_.Name -like "Godot*.exe"
    }
    foreach ($proc in @($processes)) {
        $cmd = [string]$proc.CommandLine
        if ([string]::IsNullOrWhiteSpace($cmd)) {
            continue
        }
        $pathArg = Get-CommandLinePathArg -CommandLine $cmd -Name "--path"
        if ([string]::IsNullOrWhiteSpace($pathArg)) {
            continue
        }
        if ((Normalize-ComparablePath -Path $pathArg) -ne $projectKey) {
            continue
        }
        $isEditor = $cmd -match "(?i)(?:^|\s)--editor(?:\s|$)"
        if ($RuntimeOnly -and $isEditor) {
            continue
        }
        $rows += [pscustomobject]@{
            pid = [int]$proc.ProcessId
            parent_pid = [int]$proc.ParentProcessId
            name = [string]$proc.Name
            executable_path = [string]$proc.ExecutablePath
            command_line = $cmd
            project_path = $pathArg
            is_editor = [bool]$isEditor
        }
    }
    return $rows
}

function Group-GodotRuntimeProcesses {
    param([object[]]$Processes)
    $byPid = @{}
    foreach ($proc in @($Processes)) {
        $byPid[[int]$proc.pid] = $proc
    }
    $groups = @{}
    foreach ($proc in @($Processes)) {
        $root = $proc
        $seen = @{}
        while ($null -ne $root -and $byPid.ContainsKey([int]$root.parent_pid) -and -not $seen.ContainsKey([int]$root.pid)) {
            $seen[[int]$root.pid] = $true
            $root = $byPid[[int]$root.parent_pid]
        }
        if ($null -eq $root) {
            $root = $proc
        }
        $key = [string]([int]$root.pid)
        if (-not $groups.ContainsKey($key)) {
            $groups[$key] = [System.Collections.ArrayList]::new()
        }
        [void]$groups[$key].Add($proc)
    }
    $out = @()
    foreach ($key in @($groups.Keys)) {
        $members = @($groups[$key])
        $root = $members | Where-Object { [int]$_.pid -eq [int]$key } | Select-Object -First 1
        if ($null -eq $root) {
            $root = $members | Select-Object -First 1
        }
        $out += [pscustomobject]@{
            root_pid = [int]$root.pid
            processes = $members
        }
    }
    return $out
}

function Resolve-GodotProjectRoot {
    param([string]$RequestedProjectRoot)
    if (-not [string]::IsNullOrWhiteSpace($RequestedProjectRoot)) {
        return (Resolve-Path -LiteralPath $RequestedProjectRoot -ErrorAction Stop).Path
    }
    foreach ($proc in @(Get-CimInstance Win32_Process -ErrorAction SilentlyContinue | Where-Object { $_.Name -like "Godot*.exe" })) {
        $pathArg = Get-CommandLinePathArg -CommandLine ([string]$proc.CommandLine) -Name "--path"
        if (-not [string]::IsNullOrWhiteSpace($pathArg) -and (Test-Path -LiteralPath (Join-Path $pathArg "project.godot"))) {
            return (Resolve-Path -LiteralPath $pathArg).Path
        }
    }
    $fallback = "D:\Godot\project\vit-daw-frontend"
    if (Test-Path -LiteralPath (Join-Path $fallback "project.godot")) {
        return (Resolve-Path -LiteralPath $fallback).Path
    }
    Fail "Godot project root was not supplied and no running Godot --path project.godot was found"
}

function Resolve-GodotExecutable {
    param(
        [string]$RequestedGodotExe,
        [string]$ProjectRoot
    )
    $candidates = @()
    if (-not [string]::IsNullOrWhiteSpace($RequestedGodotExe)) {
        $candidates += $RequestedGodotExe.Trim().Trim('"')
    }
    foreach ($proc in @(Find-GodotProjectProcesses -ProjectRoot $ProjectRoot -RuntimeOnly $false)) {
        if (-not [string]::IsNullOrWhiteSpace($proc.executable_path)) {
            $candidates += [string]$proc.executable_path
        }
    }
    if (-not [string]::IsNullOrWhiteSpace($env:GODOT)) {
        $candidates += $env:GODOT.Trim().Trim('"')
    }
    $candidates += @(
        "D:\Godot\Godot_v4.6.1-stable_win64_console.exe",
        "D:\Godot\Godot_v4.6.1-stable_win64.exe",
        "${env:ProgramFiles}\Godot\Godot.exe",
        "${env:LocalAppData}\Programs\Godot\Godot.exe"
    )
    return Resolve-FirstExistingPath -Candidates $candidates -Label "Godot executable"
}

function Resolve-GodotLaunchExecutable {
    param([string]$GodotExe)
    $resolved = (Resolve-Path -LiteralPath $GodotExe).Path
    if ($resolved -match "(?i)_console\.exe$") {
        return $resolved
    }
    $dir = Split-Path -Parent $resolved
    $name = [System.IO.Path]::GetFileName($resolved)
    $consoleName = $name -replace "\.exe$", "_console.exe"
    $consolePath = Join-Path $dir $consoleName
    if (Test-Path -LiteralPath $consolePath) {
        return (Resolve-Path -LiteralPath $consolePath).Path
    }
    $consoleSibling = Get-ChildItem -LiteralPath $dir -Filter "Godot*console*.exe" -ErrorAction SilentlyContinue |
        Select-Object -First 1
    if ($null -ne $consoleSibling) {
        return $consoleSibling.FullName
    }
    return $resolved
}

function Stop-ProcessByID {
    param([int]$ProcessID)
    if ($ProcessID -le 0) {
        return
    }
    $proc = Get-Process -Id $ProcessID -ErrorAction SilentlyContinue
    if ($null -ne $proc) {
        Stop-Process -Id $ProcessID -Force
    }
}

function Stop-GodotRuntimeIfNeeded {
    param(
        [string]$ProjectRoot,
        [bool]$Reuse,
        [bool]$Quiet = $false
    )
    $runtimeProcesses = @(Find-GodotProjectProcesses -ProjectRoot $ProjectRoot -RuntimeOnly $true)
    if ($runtimeProcesses.Count -eq 0) {
        return
    }
    if ($Reuse) {
        Write-WarnLine ("reusing existing Godot runtime pid=" + $runtimeProcesses[0].pid)
        return
    }
    foreach ($group in @(Group-GodotRuntimeProcesses -Processes $runtimeProcesses)) {
        $pids = @($group.processes | ForEach-Object { [int]$_.pid } | Sort-Object -Unique)
        $childPids = @($pids | Where-Object { $_ -ne [int]$group.root_pid })
        $message = "stopping existing Godot runtime pid=" + [string]$group.root_pid
        if ($childPids.Count -gt 0) {
            $message += " child_pids=" + ($childPids -join ",")
        }
        if (-not $Quiet) {
            Write-WarnLine $message
        }
        foreach ($processID in $pids) {
            Stop-ProcessByID -ProcessID ([int]$processID)
        }
    }
    Start-Sleep -Milliseconds 500
}

function Stop-PortOwnerIfNeeded {
    param(
        [int]$Port,
        [string]$Label,
        [bool]$Reuse
    )
    $listener = Get-TcpListener -Port $Port
    if ($null -eq $listener) {
        return
    }
    if ($Reuse) {
        Write-WarnLine ("reusing existing " + $Label + " pid=" + $listener.OwningProcess)
        return
    }
    Write-WarnLine ("stopping existing " + $Label + " pid=" + $listener.OwningProcess)
    Stop-ProcessByID -ProcessID ([int]$listener.OwningProcess)
    Start-Sleep -Milliseconds 500
}

function Build-AgentAndHubIfNeeded {
    param(
        [string]$RepoRoot,
        [string]$AgentPath,
        [string]$VspHubPath,
        [bool]$SkipAgentBuild,
        [bool]$SkipHubBuild
    )
    if ($SkipAgentBuild -and -not (Test-Path -LiteralPath $AgentPath)) {
        Fail ("Missing agent exe while agent build is skipped: " + $AgentPath)
    }
    if ($SkipHubBuild -and -not (Test-Path -LiteralPath $VspHubPath)) {
        Fail ("Missing VSP Hub exe while hub build is skipped: " + $VspHubPath)
    }
    if ($SkipAgentBuild -and $SkipHubBuild) {
        return
    }

    $agentDir = Join-Path $RepoRoot "agent"
    $buildDir = Join-Path $agentDir "bin"
    $agentBuildExe = Join-Path $buildDir "VitAgent.lifecycle-smoke.exe"
    $hubBuildExe = Join-Path $buildDir "VspHub.lifecycle-smoke.exe"
    New-Item -ItemType Directory -Path $buildDir -Force | Out-Null

    Push-Location $agentDir
    try {
        if (-not $SkipAgentBuild) {
            & go build -o $agentBuildExe .\cmd\vitagent
            if ($LASTEXITCODE -ne 0) {
                Fail ("VitAgent go build failed with exit code " + $LASTEXITCODE)
            }
        }
        if (-not $SkipHubBuild) {
            & go build -o $hubBuildExe .\cmd\vsphub
            if ($LASTEXITCODE -ne 0) {
                Fail ("VspHub go build failed with exit code " + $LASTEXITCODE)
            }
        }
    }
    finally {
        Pop-Location
    }

    if (-not $SkipAgentBuild) {
        New-Item -ItemType Directory -Path (Split-Path -Parent $AgentPath) -Force | Out-Null
        Copy-Item -LiteralPath $agentBuildExe -Destination $AgentPath -Force
    }
    if (-not $SkipHubBuild) {
        New-Item -ItemType Directory -Path (Split-Path -Parent $VspHubPath) -Force | Out-Null
        Copy-Item -LiteralPath $hubBuildExe -Destination $VspHubPath -Force
    }
}

function Get-ProcessPathByID {
    param([int]$ProcessID)
    $proc = Get-Process -Id $ProcessID -ErrorAction SilentlyContinue
    if ($null -eq $proc) {
        return ""
    }
    return [string]$proc.Path
}

function Get-ExecutableEvidence {
    param([string]$Path)
    if ([string]::IsNullOrWhiteSpace($Path) -or -not (Test-Path -LiteralPath $Path)) {
        return @{
            path = $Path
            exists = $false
        }
    }
    $item = Get-Item -LiteralPath $Path
    $hash = Get-FileHash -LiteralPath $Path -Algorithm SHA256
    return @{
        path = $item.FullName
        exists = $true
        length = [int64]$item.Length
        last_write_time_utc = $item.LastWriteTimeUtc.ToString("o")
        sha256 = $hash.Hash
    }
}

function Get-ListenersSnapshot {
    $rows = @()
    foreach ($port in @($AgentHttpPort, $VspHubPort, $ZmqReqPort, $ZmqSubPort)) {
        $listener = Get-TcpListener -Port $port
        if ($null -eq $listener) {
            $rows += [pscustomobject]@{
                port = $port
                listening = $false
                pid = $null
                process = $null
                path = $null
            }
            continue
        }
        $proc = Get-Process -Id $listener.OwningProcess -ErrorAction SilentlyContinue
        $rows += [pscustomobject]@{
            port = $port
            listening = $true
            pid = [int]$listener.OwningProcess
            process = if ($null -ne $proc) { $proc.ProcessName } else { $null }
            path = if ($null -ne $proc) { [string]$proc.Path } else { $null }
        }
    }
    return $rows
}

function Read-LogText {
    param([string]$Path)
    if ([string]::IsNullOrWhiteSpace($Path) -or -not (Test-Path -LiteralPath $Path)) {
        return ""
    }
    try {
        $stream = [System.IO.File]::Open($Path, [System.IO.FileMode]::Open, [System.IO.FileAccess]::Read, [System.IO.FileShare]::ReadWrite)
        try {
            $reader = [System.IO.StreamReader]::new($stream, [System.Text.Encoding]::UTF8, $true)
            try {
                return $reader.ReadToEnd()
            }
            finally {
                $reader.Dispose()
            }
        }
        finally {
            $stream.Dispose()
        }
    }
    catch {
        return ""
    }
}

function Test-LogContains {
    param(
        [string[]]$LogPaths,
        [string]$Pattern
    )
    foreach ($logPath in $LogPaths) {
        $text = Read-LogText -Path $logPath
        if (-not [string]::IsNullOrWhiteSpace($text) -and $text.Contains($Pattern)) {
            return $true
        }
    }
    return $false
}

function Get-GodotAutostartEvidence {
    param([string]$LogPath)
    $rows = @()
    $text = Read-LogText -Path $LogPath
    if ([string]::IsNullOrWhiteSpace($text)) {
        return $rows
    }
    foreach ($line in @($text -split "\r?\n")) {
        $match = [regex]::Match($line, "VitIpcClient:\s+registered autostart child\s+(\w+)\s+pid=(\d+)\s+path=(.+)$")
        if (-not $match.Success) {
            continue
        }
        $rows += [pscustomobject]@{
            role = $match.Groups[1].Value
            pid = [int]$match.Groups[2].Value
            path = $match.Groups[3].Value.Trim()
            line = $line
        }
    }
    return $rows
}

function Wait-GodotLifecycleEvidence {
    param(
        [string]$StdoutLog,
        [string]$AlternateLog,
        [int]$TimeoutSeconds
    )
    $logPaths = @($StdoutLog, $AlternateLog)
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    do {
        $evidence = @()
        foreach ($logPath in $logPaths) {
            $evidence += @(Get-GodotAutostartEvidence -LogPath $logPath)
        }
        $hasKernel = $false
        $hasHub = $false
        $hasAgent = $false
        foreach ($row in $evidence) {
            if ([string]$row.role -eq "kernel") { $hasKernel = $true }
            if ([string]$row.role -eq "hub") { $hasHub = $true }
            if ([string]$row.role -eq "agent") { $hasAgent = $true }
        }
        $hubReadyLog = Test-LogContains -LogPaths $logPaths -Pattern "start_page: VSP Hub ready."
        $agentReadyLog = Test-LogContains -LogPaths $logPaths -Pattern "start_page: Agent v0.5 tools ready."
        $agentListener = Get-TcpListener -Port $AgentHttpPort
        $hubListener = Get-TcpListener -Port $VspHubPort
        $kernelReqListener = Get-TcpListener -Port $ZmqReqPort
        $kernelSubListener = Get-TcpListener -Port $ZmqSubPort
        $portsReady = ($null -ne $agentListener -and $null -ne $hubListener -and $null -ne $kernelReqListener -and $null -ne $kernelSubListener)
        if ($hasKernel -and $hasHub -and $hasAgent -and $portsReady -and $hubReadyLog -and $agentReadyLog) {
            return @{
                mode = "godot_autostart_log"
                autostart = $evidence
                vsp_hub_ready_log = $true
                agent_tools_ready_log = $true
            }
        }
        if ($portsReady -and $hubReadyLog -and $agentReadyLog) {
            return @{
                mode = "godot_runtime_self_check_ports"
                autostart = $evidence
                vsp_hub_ready_log = $true
                agent_tools_ready_log = $true
            }
        }
        Start-Sleep -Milliseconds 250
    } while ((Get-Date) -lt $deadline)

    $finalEvidence = @()
    foreach ($logPath in $logPaths) {
        $finalEvidence += @(Get-GodotAutostartEvidence -LogPath $logPath)
    }
    return @{
        mode = "missing_godot_lifecycle_evidence"
        autostart = $finalEvidence
        vsp_hub_ready_log = Test-LogContains -LogPaths $logPaths -Pattern "start_page: VSP Hub ready."
        agent_tools_ready_log = Test-LogContains -LogPaths $logPaths -Pattern "start_page: Agent v0.5 tools ready."
    }
}

function Assert-PortOwnerExpectedPath {
    param(
        [int]$Port,
        [string]$Label,
        [string]$ExpectedPath,
        [bool]$AllowExisting
    )
    $listener = Get-TcpListener -Port $Port
    if ($null -eq $listener) {
        Fail ($Label + " port is not listening: " + $Port)
    }
    $ownerPath = Get-ProcessPathByID -ProcessID ([int]$listener.OwningProcess)
    if ((Normalize-ComparablePath -Path $ownerPath) -ne (Normalize-ComparablePath -Path $ExpectedPath)) {
        Fail ($Label + " owner path mismatch. expected=" + $ExpectedPath + " actual=" + $ownerPath + " pid=" + $listener.OwningProcess)
    }
    return @{
        pid = [int]$listener.OwningProcess
        path = $ownerPath
        expected_path = $ExpectedPath
        reused = [bool]$AllowExisting
        started = (-not [bool]$AllowExisting)
    }
}

function Wait-VspHubAgentSession {
    param([int]$TimeoutSeconds)
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    $lastStatus = $null
    do {
        try {
            $status = Invoke-Json -Method GET -Uri ($VspHubHttp.TrimEnd("/") + "/vsp/status") -TimeoutSec 5
            $lastStatus = $status
            foreach ($session in @($status.sessions)) {
                if ([string]$session.role -eq "agent" -and [string]$session.client_id -eq "vit.agent.official") {
                    return $status
                }
            }
        }
        catch {
        }
        Start-Sleep -Milliseconds 500
    } while ((Get-Date) -lt $deadline)
    $detail = if ($null -ne $lastStatus) { $lastStatus | ConvertTo-Json -Depth 12 -Compress } else { "<no status>" }
    Fail ("VSP Hub did not report VitAgent session before timeout. last_status=" + $detail)
}

function Start-Or-Reuse-GodotProject {
    param(
        [string]$GodotLaunchExe,
        [string]$ProjectRoot,
        [string]$StdoutLog,
        [string]$StderrLog,
        [bool]$Reuse,
        [int]$TimeoutSeconds
    )
    $runtimeProcesses = @(Find-GodotProjectProcesses -ProjectRoot $ProjectRoot -RuntimeOnly $true)
    if ($runtimeProcesses.Count -gt 0 -and $Reuse) {
        return @{
            pid = [int]$runtimeProcesses[0].pid
            started = $false
            reused = $true
            exe = $runtimeProcesses[0].executable_path
            launch_exe = $GodotLaunchExe
            project_root = $ProjectRoot
            command_line = $runtimeProcesses[0].command_line
            stdout_log = $StdoutLog
            stderr_log = $StderrLog
        }
    }

    Set-Content -LiteralPath $StderrLog -Value "" -Encoding UTF8
    $args = @("--path", $ProjectRoot, "--log-file", $StdoutLog)
    $proc = Start-Process -FilePath $GodotLaunchExe -ArgumentList $args -WorkingDirectory $ProjectRoot -PassThru
    Start-Sleep -Milliseconds 500
    return @{
        pid = [int]$proc.Id
        started = $true
        reused = $false
        exe = $GodotLaunchExe
        launch_exe = $GodotLaunchExe
        project_root = $ProjectRoot
        command_line = ("`"" + $GodotLaunchExe + "`" --path " + $ProjectRoot)
        stdout_log = $StdoutLog
        stderr_log = $StderrLog
        lifecycle_timeout_seconds = $TimeoutSeconds
        log_capture = "godot_log_file"
    }
}

function Invoke-GodotVspRealtimeWebSocketProbe {
    param(
        [string]$GodotLaunchExe,
        [string]$ProjectRoot,
        [string]$HubUrl,
        [string]$ArtifactDir,
        [int]$TimeoutSeconds
    )
    $scriptPath = "res://tools/diagnostics/vsp_realtime_ws_live_probe.gd"
    $stdoutPath = Join-Path $ArtifactDir "godot_vsp_realtime_ws_probe.log"
    $rawStdoutPath = Join-Path $ArtifactDir "godot_vsp_realtime_ws_probe_stdout.log"
    $rawStderrPath = Join-Path $ArtifactDir "godot_vsp_realtime_ws_probe_stderr.log"
    $summaryPath = Join-Path $ArtifactDir "godot_vsp_realtime_ws_probe.json"
    $oldHttpUrl = $env:VIT_GUI_VSP_HTTP_URL
    $oldWsUrl = $env:VIT_GUI_VSP_REALTIME_WS_URL
    $oldWsDisable = $env:VIT_GUI_VSP_REALTIME_WS_DISABLE
    $oldRealtimeDisable = $env:VIT_GUI_VSP_REALTIME_DISABLE
    $oldVspDisable = $env:VIT_GUI_VSP_DISABLE
    $oldProbeTimeout = $env:VIT_WS_LIVE_PROBE_TIMEOUT_SEC
    $oldLegacyUdpDisable = $env:VIT_GUI_LEGACY_UDP_TELEMETRY_DISABLE
    try {
        $env:VIT_GUI_VSP_HTTP_URL = $HubUrl.TrimEnd("/")
        $env:VIT_GUI_VSP_REALTIME_WS_URL = ($HubUrl.TrimEnd("/") -replace "^http://", "ws://") -replace "^https://", "wss://"
        if (-not $env:VIT_GUI_VSP_REALTIME_WS_URL.EndsWith("/stream")) {
            $env:VIT_GUI_VSP_REALTIME_WS_URL = $env:VIT_GUI_VSP_REALTIME_WS_URL.TrimEnd("/") + "/stream"
        }
        $env:VIT_GUI_VSP_REALTIME_WS_DISABLE = "0"
        $env:VIT_GUI_VSP_REALTIME_DISABLE = "0"
        $env:VIT_GUI_VSP_DISABLE = "0"
        $env:VIT_WS_LIVE_PROBE_TIMEOUT_SEC = [string]([Math]::Max($TimeoutSeconds, 20))
        $env:VIT_GUI_LEGACY_UDP_TELEMETRY_DISABLE = "1"

        $args = @("--headless", "--path", $ProjectRoot, "--script", $scriptPath)
        $proc = Start-Process -FilePath $GodotLaunchExe -ArgumentList $args -WorkingDirectory $ProjectRoot -RedirectStandardOutput $rawStdoutPath -RedirectStandardError $rawStderrPath -PassThru -Wait -WindowStyle Hidden
        $exitCode = [int]$proc.ExitCode
        $output = @()
        if (Test-Path -LiteralPath $rawStdoutPath) {
            $output += @(Get-Content -LiteralPath $rawStdoutPath -ErrorAction SilentlyContinue | ForEach-Object { [string]$_ })
        }
        if (Test-Path -LiteralPath $rawStderrPath) {
            $output += @(Get-Content -LiteralPath $rawStderrPath -ErrorAction SilentlyContinue | ForEach-Object { [string]$_ })
        }
        Set-Content -LiteralPath $stdoutPath -Value $output -Encoding UTF8
        if ($exitCode -ne 0) {
            Fail ("Godot VSP realtime WebSocket probe failed with exit code " + $exitCode + ". log=" + $stdoutPath)
        }
        $jsonLine = ""
        foreach ($line in $output) {
            $trimmed = ([string]$line).Trim()
            if ($trimmed.StartsWith("{") -and $trimmed.Contains('"schema_version":"vsp_realtime_ws_live_probe.v1"')) {
                $jsonLine = $trimmed
            }
        }
        if ([string]::IsNullOrWhiteSpace($jsonLine)) {
            Fail ("Godot VSP realtime WebSocket probe did not emit JSON summary. log=" + $stdoutPath)
        }
        $probeSummary = $jsonLine | ConvertFrom-Json
        if ([string]$probeSummary.status -ne "ok") {
            Fail ("Godot VSP realtime WebSocket probe status was not ok: " + ($probeSummary | ConvertTo-Json -Depth 12 -Compress))
        }
        ConvertTo-JsonFile -Value $probeSummary -Path $summaryPath
        return @{
            status = "ok"
            script = $scriptPath
            log = $stdoutPath
            summary = $summaryPath
            checks = @($probeSummary.checks)
            subscription_transport = [string]$probeSummary.subscription_transport
            visible_track_count = [int]$probeSummary.visible_track_count
        }
    }
    finally {
        $env:VIT_GUI_VSP_HTTP_URL = $oldHttpUrl
        $env:VIT_GUI_VSP_REALTIME_WS_URL = $oldWsUrl
        $env:VIT_GUI_VSP_REALTIME_WS_DISABLE = $oldWsDisable
        $env:VIT_GUI_VSP_REALTIME_DISABLE = $oldRealtimeDisable
        $env:VIT_GUI_VSP_DISABLE = $oldVspDisable
        $env:VIT_WS_LIVE_PROBE_TIMEOUT_SEC = $oldProbeTimeout
        $env:VIT_GUI_LEGACY_UDP_TELEMETRY_DISABLE = $oldLegacyUdpDisable
    }
}

function Invoke-GodotGuiHealthPlaybackProbe {
    param(
        [string]$GodotLaunchExe,
        [string]$ProjectRoot,
        [string]$HubUrl,
        [string]$ArtifactDir,
        [int]$TimeoutSeconds,
        [int]$DurationSeconds,
        [int]$MinTrackCount,
        [bool]$InputAudit,
        [int]$InputAuditIntervalSeconds,
        [int]$InputAuditMinClicks,
        [string]$InputAuditActions,
        [string]$Layout,
        [string]$DynamicMode
    )
    $scriptPath = "res://tools/diagnostics/gui_health_playback_probe.gd"
    $stdoutPath = Join-Path $ArtifactDir "godot_gui_health_playback_probe.log"
    $rawStdoutPath = Join-Path $ArtifactDir "godot_gui_health_playback_probe_stdout.log"
    $rawStderrPath = Join-Path $ArtifactDir "godot_gui_health_playback_probe_stderr.log"
    $summaryPath = Join-Path $ArtifactDir "godot_gui_health_playback_probe.json"
    $oldSkipAutostart = $env:VIT_SKIP_DEV_AUTOSTART
    $oldHttpUrl = $env:VIT_GUI_VSP_HTTP_URL
    $oldWsUrl = $env:VIT_GUI_VSP_REALTIME_WS_URL
    $oldVspDisable = $env:VIT_GUI_VSP_DISABLE
    $oldRealtimeDisable = $env:VIT_GUI_VSP_REALTIME_DISABLE
    $oldWsDisable = $env:VIT_GUI_VSP_REALTIME_WS_DISABLE
    $oldHealthMonitor = $env:VIT_GUI_HEALTH_MONITOR
    $oldHealthInput = $env:VIT_GUI_HEALTH_INPUT_LOG
    $oldHealthSummary = $env:VIT_GUI_HEALTH_SUMMARY_LOG
    $oldHealthLongFrame = $env:VIT_GUI_HEALTH_LONG_FRAME_MS
    $oldProbeDuration = $env:VIT_GUI_HEALTH_PLAYBACK_PROBE_DURATION_SEC
    $oldProbeTimeout = $env:VIT_GUI_HEALTH_PLAYBACK_PROBE_TIMEOUT_SEC
    $oldProbeMinTracks = $env:VIT_GUI_HEALTH_PLAYBACK_PROBE_MIN_TRACKS
    $oldProbeInputAudit = $env:VIT_GUI_HEALTH_PLAYBACK_PROBE_INPUT_AUDIT
    $oldProbeInputInterval = $env:VIT_GUI_HEALTH_PLAYBACK_PROBE_INPUT_INTERVAL_SEC
    $oldProbeInputMinClicks = $env:VIT_GUI_HEALTH_PLAYBACK_PROBE_INPUT_MIN_CLICKS
    $oldProbeInputActions = $env:VIT_GUI_HEALTH_PLAYBACK_PROBE_INPUT_ACTIONS
    $oldProbeLayout = $env:VIT_GUI_HEALTH_PLAYBACK_PROBE_LAYOUT
    $oldProbeDynamicMode = $env:VIT_GUI_HEALTH_PLAYBACK_PROBE_DYNAMIC_MODE
    $oldLegacyUdpDisable = $env:VIT_GUI_LEGACY_UDP_TELEMETRY_DISABLE
    try {
        $probeTimeout = [Math]::Max($TimeoutSeconds, $DurationSeconds + 30)
        $env:VIT_SKIP_DEV_AUTOSTART = "1"
        $env:VIT_GUI_VSP_HTTP_URL = $HubUrl.TrimEnd("/")
        $env:VIT_GUI_VSP_REALTIME_WS_URL = ($HubUrl.TrimEnd("/") -replace "^http://", "ws://") -replace "^https://", "wss://"
        if (-not $env:VIT_GUI_VSP_REALTIME_WS_URL.EndsWith("/stream")) {
            $env:VIT_GUI_VSP_REALTIME_WS_URL = $env:VIT_GUI_VSP_REALTIME_WS_URL.TrimEnd("/") + "/stream"
        }
        $env:VIT_GUI_VSP_DISABLE = "0"
        $env:VIT_GUI_VSP_REALTIME_DISABLE = "0"
        $env:VIT_GUI_VSP_REALTIME_WS_DISABLE = "0"
        $env:VIT_GUI_HEALTH_MONITOR = "1"
        $env:VIT_GUI_HEALTH_INPUT_LOG = "0"
        $env:VIT_GUI_HEALTH_SUMMARY_LOG = "0"
        $env:VIT_GUI_HEALTH_LONG_FRAME_MS = "80"
        $env:VIT_GUI_HEALTH_PLAYBACK_PROBE_DURATION_SEC = [string]([Math]::Max(1, $DurationSeconds))
        $env:VIT_GUI_HEALTH_PLAYBACK_PROBE_TIMEOUT_SEC = [string]([Math]::Max(5, $probeTimeout))
        $env:VIT_GUI_HEALTH_PLAYBACK_PROBE_MIN_TRACKS = [string]([Math]::Max(0, $MinTrackCount))
        $env:VIT_GUI_HEALTH_PLAYBACK_PROBE_INPUT_AUDIT = if ($InputAudit) { "1" } else { "0" }
        $env:VIT_GUI_HEALTH_PLAYBACK_PROBE_INPUT_INTERVAL_SEC = [string]([Math]::Max(1, $InputAuditIntervalSeconds))
        $env:VIT_GUI_HEALTH_PLAYBACK_PROBE_INPUT_MIN_CLICKS = [string]([Math]::Max(0, $InputAuditMinClicks))
        $env:VIT_GUI_HEALTH_PLAYBACK_PROBE_INPUT_ACTIONS = $InputAuditActions
        $env:VIT_GUI_HEALTH_PLAYBACK_PROBE_LAYOUT = $Layout
        $env:VIT_GUI_HEALTH_PLAYBACK_PROBE_DYNAMIC_MODE = $DynamicMode
        $env:VIT_GUI_LEGACY_UDP_TELEMETRY_DISABLE = "1"

        $args = @("--headless", "--path", $ProjectRoot, "--script", $scriptPath)
        $proc = Start-Process -FilePath $GodotLaunchExe -ArgumentList $args -WorkingDirectory $ProjectRoot -RedirectStandardOutput $rawStdoutPath -RedirectStandardError $rawStderrPath -PassThru -Wait -WindowStyle Hidden
        $exitCode = [int]$proc.ExitCode
        $output = @()
        if (Test-Path -LiteralPath $rawStdoutPath) {
            $output += @(Get-Content -LiteralPath $rawStdoutPath -ErrorAction SilentlyContinue | ForEach-Object { [string]$_ })
        }
        if (Test-Path -LiteralPath $rawStderrPath) {
            $output += @(Get-Content -LiteralPath $rawStderrPath -ErrorAction SilentlyContinue | ForEach-Object { [string]$_ })
        }
        Set-Content -LiteralPath $stdoutPath -Value $output -Encoding UTF8
        if ($exitCode -ne 0) {
            Fail ("Godot GUI health playback probe failed with exit code " + $exitCode + ". log=" + $stdoutPath)
        }
        $jsonLine = ""
        foreach ($line in $output) {
            $trimmed = ([string]$line).Trim()
            if ($trimmed.StartsWith("{") -and $trimmed.Contains('"schema_version":"gui_health_playback_probe.v1"')) {
                $jsonLine = $trimmed
            }
        }
        if ([string]::IsNullOrWhiteSpace($jsonLine)) {
            Fail ("Godot GUI health playback probe did not emit JSON summary. log=" + $stdoutPath)
        }
        $probeSummary = $jsonLine | ConvertFrom-Json
        if ([string]$probeSummary.status -ne "ok") {
            Fail ("Godot GUI health playback probe status was not ok: " + ($probeSummary | ConvertTo-Json -Depth 18 -Compress))
        }
        ConvertTo-JsonFile -Value $probeSummary -Path $summaryPath -Depth 32
        return @{
            status = "ok"
            script = $scriptPath
            log = $stdoutPath
            summary = $summaryPath
            checks = @($probeSummary.checks)
            track_count_user_visible = [int]$probeSummary.track_count_user_visible
            sample_count = [int]$probeSummary.analysis.sample_count
            max_frame_ms = [double]$probeSummary.analysis.max_frame_ms
            slow_frames = [int]$probeSummary.analysis.slow_frames
            budget_60hz_frames = [int]$probeSummary.analysis.budget_60hz_frames
            budget_30hz_frames = [int]$probeSummary.analysis.budget_30hz_frames
            frame_budget_max_frame_ms = [double]$probeSummary.playback_frame_budget.max_frame_ms
            max_realtime_pending = [int]$probeSummary.analysis.max_realtime_pending
            transport_client_ok = [int]$probeSummary.final.queues.transport_client.ok_count
            transport_client_fallback = [int]$probeSummary.final.queues.transport_client.fallback_count
            input_audit_enabled = [bool]$probeSummary.input_audit.enabled
            input_audit_clicks = [int]$probeSummary.input_audit.clicks
            input_audit_failures = [int]$probeSummary.input_audit.failures
            input_click_failures = [int]$probeSummary.analysis.clicks.failures
            input_click_max_pressed_latency_ms = [int]$probeSummary.analysis.clicks.max_pressed_latency_ms
            layout = [string]$probeSummary.config.layout
            dynamic_mode = [string]$probeSummary.config.dynamic_mode
        }
    }
    finally {
        $env:VIT_SKIP_DEV_AUTOSTART = $oldSkipAutostart
        $env:VIT_GUI_VSP_HTTP_URL = $oldHttpUrl
        $env:VIT_GUI_VSP_REALTIME_WS_URL = $oldWsUrl
        $env:VIT_GUI_VSP_DISABLE = $oldVspDisable
        $env:VIT_GUI_VSP_REALTIME_DISABLE = $oldRealtimeDisable
        $env:VIT_GUI_VSP_REALTIME_WS_DISABLE = $oldWsDisable
        $env:VIT_GUI_HEALTH_MONITOR = $oldHealthMonitor
        $env:VIT_GUI_HEALTH_INPUT_LOG = $oldHealthInput
        $env:VIT_GUI_HEALTH_SUMMARY_LOG = $oldHealthSummary
        $env:VIT_GUI_HEALTH_LONG_FRAME_MS = $oldHealthLongFrame
        $env:VIT_GUI_HEALTH_PLAYBACK_PROBE_DURATION_SEC = $oldProbeDuration
        $env:VIT_GUI_HEALTH_PLAYBACK_PROBE_TIMEOUT_SEC = $oldProbeTimeout
        $env:VIT_GUI_HEALTH_PLAYBACK_PROBE_MIN_TRACKS = $oldProbeMinTracks
        $env:VIT_GUI_HEALTH_PLAYBACK_PROBE_INPUT_AUDIT = $oldProbeInputAudit
        $env:VIT_GUI_HEALTH_PLAYBACK_PROBE_INPUT_INTERVAL_SEC = $oldProbeInputInterval
        $env:VIT_GUI_HEALTH_PLAYBACK_PROBE_INPUT_MIN_CLICKS = $oldProbeInputMinClicks
        $env:VIT_GUI_HEALTH_PLAYBACK_PROBE_INPUT_ACTIONS = $oldProbeInputActions
        $env:VIT_GUI_HEALTH_PLAYBACK_PROBE_LAYOUT = $oldProbeLayout
        $env:VIT_GUI_HEALTH_PLAYBACK_PROBE_DYNAMIC_MODE = $oldProbeDynamicMode
        $env:VIT_GUI_LEGACY_UDP_TELEMETRY_DISABLE = $oldLegacyUdpDisable
    }
}

function Invoke-GodotGuiArchitectureAuditProbe {
    param(
        [string]$GodotLaunchExe,
        [string]$ProjectRoot,
        [string]$HubUrl,
        [string]$ArtifactDir,
        [int]$TimeoutSeconds,
        [int]$DurationSeconds,
        [int]$MinTrackCount,
        [string]$Layout,
        [string]$DynamicMode,
        [bool]$InputAudit = $false,
        [int]$InputAuditIntervalSeconds = 3,
        [int]$InputAuditMinClicks = 1,
        [string]$InputAuditActions = "stop_return_to_start,loop,return_to_zero",
        [string]$RackFixture = "",
        [int]$RackFixturePluginCount = 0,
        [int]$RackFixtureConnectionCount = 0
    )
    $scriptPath = "res://tools/diagnostics/gui_architecture_audit_probe.gd"
    $stdoutPath = Join-Path $ArtifactDir "godot_gui_architecture_audit_probe.log"
    $rawStdoutPath = Join-Path $ArtifactDir "godot_gui_architecture_audit_probe_stdout.log"
    $rawStderrPath = Join-Path $ArtifactDir "godot_gui_architecture_audit_probe_stderr.log"
    $summaryPath = Join-Path $ArtifactDir "godot_gui_architecture_audit_probe.json"
    $oldSkipAutostart = $env:VIT_SKIP_DEV_AUTOSTART
    $oldHttpUrl = $env:VIT_GUI_VSP_HTTP_URL
    $oldWsUrl = $env:VIT_GUI_VSP_REALTIME_WS_URL
    $oldVspDisable = $env:VIT_GUI_VSP_DISABLE
    $oldRealtimeDisable = $env:VIT_GUI_VSP_REALTIME_DISABLE
    $oldWsDisable = $env:VIT_GUI_VSP_REALTIME_WS_DISABLE
    $oldHealthMonitor = $env:VIT_GUI_HEALTH_MONITOR
    $oldHealthInput = $env:VIT_GUI_HEALTH_INPUT_LOG
    $oldHealthSummary = $env:VIT_GUI_HEALTH_SUMMARY_LOG
    $oldHealthLongFrame = $env:VIT_GUI_HEALTH_LONG_FRAME_MS
    $oldAuditSample = $env:VIT_GUI_ARCH_AUDIT_SAMPLE_SEC
    $oldAuditTimeout = $env:VIT_GUI_ARCH_AUDIT_TIMEOUT_SEC
    $oldAuditMinTracks = $env:VIT_GUI_ARCH_AUDIT_MIN_TRACKS
    $oldAuditPlayback = $env:VIT_GUI_ARCH_AUDIT_PLAYBACK
    $oldAuditLayout = $env:VIT_GUI_ARCH_AUDIT_LAYOUT
    $oldAuditDynamicMode = $env:VIT_GUI_ARCH_AUDIT_DYNAMIC_MODE
    $oldAuditInputAudit = $env:VIT_GUI_ARCH_AUDIT_INPUT_AUDIT
    $oldAuditInputInterval = $env:VIT_GUI_ARCH_AUDIT_INPUT_INTERVAL_SEC
    $oldAuditInputMinClicks = $env:VIT_GUI_ARCH_AUDIT_INPUT_MIN_CLICKS
    $oldAuditInputActions = $env:VIT_GUI_ARCH_AUDIT_INPUT_ACTIONS
    $oldAuditRackFixture = $env:VIT_GUI_ARCH_AUDIT_RACK_FIXTURE
    $oldAuditRackFixturePluginCount = $env:VIT_GUI_ARCH_AUDIT_RACK_FIXTURE_PLUGIN_COUNT
    $oldAuditRackFixtureConnectionCount = $env:VIT_GUI_ARCH_AUDIT_RACK_FIXTURE_CONNECTION_COUNT
    $oldLegacyUdpDisable = $env:VIT_GUI_LEGACY_UDP_TELEMETRY_DISABLE
    try {
        $probeTimeout = [Math]::Max($TimeoutSeconds, $DurationSeconds + 30)
        $env:VIT_SKIP_DEV_AUTOSTART = "1"
        $env:VIT_GUI_VSP_HTTP_URL = $HubUrl.TrimEnd("/")
        $env:VIT_GUI_VSP_REALTIME_WS_URL = ($HubUrl.TrimEnd("/") -replace "^http://", "ws://") -replace "^https://", "wss://"
        if (-not $env:VIT_GUI_VSP_REALTIME_WS_URL.EndsWith("/stream")) {
            $env:VIT_GUI_VSP_REALTIME_WS_URL = $env:VIT_GUI_VSP_REALTIME_WS_URL.TrimEnd("/") + "/stream"
        }
        $env:VIT_GUI_VSP_DISABLE = "0"
        $env:VIT_GUI_VSP_REALTIME_DISABLE = "0"
        $env:VIT_GUI_VSP_REALTIME_WS_DISABLE = "0"
        $env:VIT_GUI_HEALTH_MONITOR = "1"
        $env:VIT_GUI_HEALTH_INPUT_LOG = "0"
        $env:VIT_GUI_HEALTH_SUMMARY_LOG = "0"
        $env:VIT_GUI_HEALTH_LONG_FRAME_MS = "80"
        $env:VIT_GUI_ARCH_AUDIT_SAMPLE_SEC = [string]([Math]::Max(1, $DurationSeconds))
        $env:VIT_GUI_ARCH_AUDIT_TIMEOUT_SEC = [string]([Math]::Max(5, $probeTimeout))
        $env:VIT_GUI_ARCH_AUDIT_MIN_TRACKS = [string]([Math]::Max(0, $MinTrackCount))
        $env:VIT_GUI_ARCH_AUDIT_PLAYBACK = "1"
        $env:VIT_GUI_ARCH_AUDIT_LAYOUT = $Layout
        $env:VIT_GUI_ARCH_AUDIT_DYNAMIC_MODE = $DynamicMode
        $env:VIT_GUI_ARCH_AUDIT_INPUT_AUDIT = if ($InputAudit) { "1" } else { "0" }
        $env:VIT_GUI_ARCH_AUDIT_INPUT_INTERVAL_SEC = [string]([Math]::Max(1, $InputAuditIntervalSeconds))
        $env:VIT_GUI_ARCH_AUDIT_INPUT_MIN_CLICKS = [string]([Math]::Max(0, $InputAuditMinClicks))
        $env:VIT_GUI_ARCH_AUDIT_INPUT_ACTIONS = $InputAuditActions
        $env:VIT_GUI_ARCH_AUDIT_RACK_FIXTURE = ([string]$RackFixture).Trim()
        $env:VIT_GUI_ARCH_AUDIT_RACK_FIXTURE_PLUGIN_COUNT = [string]([Math]::Max(0, $RackFixturePluginCount))
        $env:VIT_GUI_ARCH_AUDIT_RACK_FIXTURE_CONNECTION_COUNT = [string]([Math]::Max(0, $RackFixtureConnectionCount))
        $env:VIT_GUI_LEGACY_UDP_TELEMETRY_DISABLE = "1"

        $args = @("--headless", "--path", $ProjectRoot, "--script", $scriptPath)
        $proc = Start-Process -FilePath $GodotLaunchExe -ArgumentList $args -WorkingDirectory $ProjectRoot -RedirectStandardOutput $rawStdoutPath -RedirectStandardError $rawStderrPath -PassThru -Wait -WindowStyle Hidden
        $exitCode = [int]$proc.ExitCode
        $output = @()
        if (Test-Path -LiteralPath $rawStdoutPath) {
            $output += @(Get-Content -LiteralPath $rawStdoutPath -ErrorAction SilentlyContinue | ForEach-Object { [string]$_ })
        }
        if (Test-Path -LiteralPath $rawStderrPath) {
            $output += @(Get-Content -LiteralPath $rawStderrPath -ErrorAction SilentlyContinue | ForEach-Object { [string]$_ })
        }
        Set-Content -LiteralPath $stdoutPath -Value $output -Encoding UTF8
        if ($exitCode -ne 0) {
            Fail ("Godot GUI architecture audit probe failed with exit code " + $exitCode + ". log=" + $stdoutPath)
        }
        $jsonLine = ""
        foreach ($line in $output) {
            $trimmed = ([string]$line).Trim()
            if ($trimmed.StartsWith("{") -and $trimmed.Contains('"schema_version":"gui_architecture_audit_probe.v1"')) {
                $jsonLine = $trimmed
            }
        }
        if ([string]::IsNullOrWhiteSpace($jsonLine)) {
            Fail ("Godot GUI architecture audit probe did not emit JSON summary. log=" + $stdoutPath)
        }
        $probeSummary = $jsonLine | ConvertFrom-Json
        if ([string]$probeSummary.status -ne "ok") {
            Fail ("Godot GUI architecture audit probe status was not ok: " + ($probeSummary | ConvertTo-Json -Depth 18 -Compress))
        }
        ConvertTo-JsonFile -Value $probeSummary -Path $summaryPath -Depth 36
        return @{
            status = "ok"
            script = $scriptPath
            log = $stdoutPath
            summary = $summaryPath
            checks = @($probeSummary.checks)
            layout = [string]$probeSummary.config.layout
            layout_applied = [bool]$probeSummary.layout.applied
            dynamic_mode = [string]$probeSummary.config.dynamic_mode
            input_audit_enabled = [bool]$probeSummary.input_audit.enabled
            input_audit_clicks = [int]$probeSummary.input_audit.clicks
            input_audit_successes = [int]$probeSummary.input_audit.successes
            input_audit_failures = [int]$probeSummary.input_audit.failures
            rack_fixture = [string]$probeSummary.config.rack_fixture.mode
            rack_fixture_applied = [bool]$probeSummary.rack_fixture.applied
            rack_fixture_target_track_id = [string]$probeSummary.rack_fixture.target_track_id
            rack_fixture_node_count = [int]$probeSummary.rack_fixture.node_count
            rack_fixture_edge_count = [int]$probeSummary.rack_fixture.edge_count
            repository_tracks = [int]$probeSummary.after.project.repository_tracks
            repository_clips = [int]$probeSummary.after.project.repository_clips
            virtual_active = [bool]$probeSummary.after.timeline.virtual.active
            virtual_total_tracks = [int]$probeSummary.after.timeline.virtual.total_tracks
            virtual_pool_size = [int]$probeSummary.after.timeline.virtual.pool_size
            virtual_visible_rows = [int]$probeSummary.after.timeline.virtual.visible_rows
            track_rows = [int]$probeSummary.after.timeline.track_rows.total
            right3d_wrappers = [int]$probeSummary.after.runtime_hotspots.right3d_wrappers_total
            right3d_processing = [int]$probeSummary.after.runtime_hotspots.right3d_wrappers_processing
            rack_count = [int]$probeSummary.after.rack.rack_count
            rack_graph_nodes = [int]$probeSummary.after.rack.graph_node_total
            rack_plugin_nodes = [int]$probeSummary.after.rack.plugin_node_total
            rack_connections = [int]$probeSummary.after.rack.connection_total
            rack_graph_edit_processing = [int]$probeSummary.after.rack.graph_edit_processing
            rack_pending_edges = ([int]$probeSummary.after.rack.pending_add_edges + [int]$probeSummary.after.rack.pending_remove_edges)
            rack_pending_param_surface_requests = [int]$probeSummary.after.rack.pending_param_surface_requests
            rack_process_count = [int]$probeSummary.after.rack.process_count
            rack_process_queue_redraw_count = [int]$probeSummary.after.rack.process_queue_redraw_count
            rack_process_anchor_count = [int]$probeSummary.after.rack.process_anchor_count
            rack_draw_count = [int]$probeSummary.after.rack.draw_count
            rack_draw_max_ms = [double]$probeSummary.after.rack.draw_max_ms
            rack_draw_avg_ms = [double]$probeSummary.after.rack.draw_avg_ms
            rack_draw_connection_total = [int]$probeSummary.after.rack.draw_connection_total
            rack_draw_last_connection_count = [int]$probeSummary.after.rack.draw_last_connection_count
            rack_draw_badge_total = [int]$probeSummary.after.rack.draw_badge_total
            rack_scrollbar_hide_count = [int]$probeSummary.after.rack.scrollbar_hide_count
            rack_scrollbar_hide_max_ms = [double]$probeSummary.after.rack.scrollbar_hide_max_ms
            processing_nodes = [int]$probeSummary.after.process.processing_count
            hidden_processing_nodes = [int]$probeSummary.after.process.hidden_processing_count
            active_timers = [int]$probeSummary.after.timers.active_count
            fast_active_timers = [int]$probeSummary.after.timers.fast_active_count
            sample_max_frame_ms = [double]$probeSummary.sample.max_frame_ms
            sample_slow_frames = [int]$probeSummary.sample.slow_frames
            sample_budget_60hz_frames = [int]$probeSummary.sample.budget_60hz_frames
            sample_budget_30hz_frames = [int]$probeSummary.sample.budget_30hz_frames
            hotspot_count = [int]@($probeSummary.hotspot_candidates).Count
            hotspot_names = @($probeSummary.hotspot_candidates | ForEach-Object { [string]$_.name })
        }
    }
    finally {
        $env:VIT_SKIP_DEV_AUTOSTART = $oldSkipAutostart
        $env:VIT_GUI_VSP_HTTP_URL = $oldHttpUrl
        $env:VIT_GUI_VSP_REALTIME_WS_URL = $oldWsUrl
        $env:VIT_GUI_VSP_DISABLE = $oldVspDisable
        $env:VIT_GUI_VSP_REALTIME_DISABLE = $oldRealtimeDisable
        $env:VIT_GUI_VSP_REALTIME_WS_DISABLE = $oldWsDisable
        $env:VIT_GUI_HEALTH_MONITOR = $oldHealthMonitor
        $env:VIT_GUI_HEALTH_INPUT_LOG = $oldHealthInput
        $env:VIT_GUI_HEALTH_SUMMARY_LOG = $oldHealthSummary
        $env:VIT_GUI_HEALTH_LONG_FRAME_MS = $oldHealthLongFrame
        $env:VIT_GUI_ARCH_AUDIT_SAMPLE_SEC = $oldAuditSample
        $env:VIT_GUI_ARCH_AUDIT_TIMEOUT_SEC = $oldAuditTimeout
        $env:VIT_GUI_ARCH_AUDIT_MIN_TRACKS = $oldAuditMinTracks
        $env:VIT_GUI_ARCH_AUDIT_PLAYBACK = $oldAuditPlayback
        $env:VIT_GUI_ARCH_AUDIT_LAYOUT = $oldAuditLayout
        $env:VIT_GUI_ARCH_AUDIT_DYNAMIC_MODE = $oldAuditDynamicMode
        $env:VIT_GUI_ARCH_AUDIT_INPUT_AUDIT = $oldAuditInputAudit
        $env:VIT_GUI_ARCH_AUDIT_INPUT_INTERVAL_SEC = $oldAuditInputInterval
        $env:VIT_GUI_ARCH_AUDIT_INPUT_MIN_CLICKS = $oldAuditInputMinClicks
        $env:VIT_GUI_ARCH_AUDIT_INPUT_ACTIONS = $oldAuditInputActions
        $env:VIT_GUI_ARCH_AUDIT_RACK_FIXTURE = $oldAuditRackFixture
        $env:VIT_GUI_ARCH_AUDIT_RACK_FIXTURE_PLUGIN_COUNT = $oldAuditRackFixturePluginCount
        $env:VIT_GUI_ARCH_AUDIT_RACK_FIXTURE_CONNECTION_COUNT = $oldAuditRackFixtureConnectionCount
        $env:VIT_GUI_LEGACY_UDP_TELEMETRY_DISABLE = $oldLegacyUdpDisable
    }
}

function Invoke-GodotGuiArchitectureAuditProfileSet {
    param(
        [string]$GodotLaunchExe,
        [string]$ProjectRoot,
        [string]$HubUrl,
        [string]$ArtifactDir,
        [int]$TimeoutSeconds,
        [int]$DefaultDurationSeconds,
        [int]$DefaultMinTrackCount,
        [string]$DefaultLayout,
        [string]$DefaultDynamicMode,
        [string]$ProfilesText,
        [bool]$DefaultInputAudit,
        [int]$DefaultInputAuditIntervalSeconds,
        [int]$DefaultInputAuditMinClicks,
        [string]$DefaultInputAuditActions,
        [string]$DefaultRackFixture,
        [int]$DefaultRackFixturePluginCount,
        [int]$DefaultRackFixtureConnectionCount
    )
    $profiles = @(ConvertFrom-GuiArchitectureAuditProfiles -ProfilesText $ProfilesText -DefaultDurationSeconds $DefaultDurationSeconds -DefaultMinTrackCount $DefaultMinTrackCount -DefaultLayout $DefaultLayout -DefaultDynamicMode $DefaultDynamicMode -DefaultInputAudit $DefaultInputAudit -DefaultInputAuditIntervalSeconds $DefaultInputAuditIntervalSeconds -DefaultInputAuditMinClicks $DefaultInputAuditMinClicks -DefaultInputAuditActions $DefaultInputAuditActions -DefaultRackFixture $DefaultRackFixture -DefaultRackFixturePluginCount $DefaultRackFixturePluginCount -DefaultRackFixtureConnectionCount $DefaultRackFixtureConnectionCount)
    $profileRoot = Join-Path $ArtifactDir "gui_architecture_audit_profiles"
    New-Item -ItemType Directory -Path $profileRoot -Force | Out-Null
    $results = @()
    foreach ($profile in $profiles) {
        $profileName = [string]$profile.name
        $safeName = [string]$profile.safe_name
        $profileArtifactDir = Join-Path $profileRoot $safeName
        New-Item -ItemType Directory -Path $profileArtifactDir -Force | Out-Null
        Write-Step ("Audit Godot GUI architecture profile " + $profileName)
        $result = Invoke-GodotGuiArchitectureAuditProbe `
            -GodotLaunchExe $GodotLaunchExe `
            -ProjectRoot $ProjectRoot `
            -HubUrl $HubUrl `
            -ArtifactDir $profileArtifactDir `
            -TimeoutSeconds $TimeoutSeconds `
            -DurationSeconds ([int]$profile.duration_seconds) `
            -MinTrackCount ([int]$profile.min_track_count) `
            -Layout ([string]$profile.layout) `
            -DynamicMode ([string]$profile.dynamic_mode) `
            -InputAudit ([bool]$profile.input_audit) `
            -InputAuditIntervalSeconds ([int]$profile.input_audit_interval_seconds) `
            -InputAuditMinClicks ([int]$profile.input_audit_min_clicks) `
            -InputAuditActions ([string]$profile.input_audit_actions) `
            -RackFixture ([string]$profile.rack_fixture) `
            -RackFixturePluginCount ([int]$profile.rack_fixture_plugin_count) `
            -RackFixtureConnectionCount ([int]$profile.rack_fixture_connection_count)
        $result["profile_name"] = $profileName
        $result["profile_safe_name"] = $safeName
        $result["duration_seconds"] = [int]$profile.duration_seconds
        $result["min_track_count"] = [int]$profile.min_track_count
        $result["input_audit_requested"] = [bool]$profile.input_audit
        $result["input_audit_interval_seconds"] = [int]$profile.input_audit_interval_seconds
        $result["input_audit_min_clicks"] = [int]$profile.input_audit_min_clicks
        $result["input_audit_actions"] = [string]$profile.input_audit_actions
        $result["rack_fixture_requested"] = [string]$profile.rack_fixture
        $result["rack_fixture_plugin_count_requested"] = [int]$profile.rack_fixture_plugin_count
        $result["rack_fixture_connection_count_requested"] = [int]$profile.rack_fixture_connection_count
        $result["artifact_dir"] = $profileArtifactDir
        $results += $result
        Write-Ok ("Godot GUI architecture profile passed name=" + $profileName + " layout=" + [string]$result.layout + " mode=" + [string]$result.dynamic_mode + " fixture=" + [string]$result.rack_fixture + " rack_nodes=" + [string]$result.rack_graph_nodes + " max_frame_ms=" + [string]$result.sample_max_frame_ms + " input_clicks=" + [string]$result.input_audit_clicks + " input_failures=" + [string]$result.input_audit_failures + " hotspots=" + [string]$result.hotspot_count)
    }
    $matrix = [ordered]@{
        status = "ok"
        schema_version = "gui_architecture_audit_profile_set.v1"
        profile_count = $results.Count
        profiles = $results
    }
    ConvertTo-JsonFile -Value $matrix -Path (Join-Path $profileRoot "profiles_summary.json") -Depth 36
    return $matrix
}

function Invoke-VspHubHttpAssetSmoke {
    param(
        [string]$RepoRoot,
        [string]$HubUrl,
        [string]$ArtifactDir,
        [int]$TimeoutSeconds
    )
    $scriptPath = Join-Path $RepoRoot "scripts\vsp_phase4_hub_http_asset_smoke.py"
    if (-not (Test-Path -LiteralPath $scriptPath)) {
        Fail ("Missing VSP Hub HTTP asset smoke script: " + $scriptPath)
    }
    $stdoutPath = Join-Path $ArtifactDir "vsp_hub_http_asset_smoke_stdout.log"
    $stderrPath = Join-Path $ArtifactDir "vsp_hub_http_asset_smoke_stderr.log"
    $summaryPath = Join-Path $ArtifactDir "vsp_hub_http_asset_smoke.json"
    $timeoutSec = [Math]::Max(30, $TimeoutSeconds)
    $timeoutMs = [Math]::Max(60000, $timeoutSec * 1000)
    $args = @(
        $scriptPath,
        "--hub-url", $HubUrl,
        "--timeout-ms", [string]$timeoutMs,
        "--timeout-sec", [string]$timeoutSec
    )
    $proc = Start-Process -FilePath "python" -ArgumentList $args -WorkingDirectory $RepoRoot -RedirectStandardOutput $stdoutPath -RedirectStandardError $stderrPath -PassThru -Wait -WindowStyle Hidden
    if ($proc.ExitCode -ne 0) {
        $err = ""
        if (Test-Path -LiteralPath $stderrPath) {
            $err = Get-Content -LiteralPath $stderrPath -Raw -ErrorAction SilentlyContinue
        }
        Fail ("VSP Hub HTTP asset smoke failed with exit code " + [string]$proc.ExitCode + ". stderr=" + $err)
    }
    $raw = Get-Content -LiteralPath $stdoutPath -Raw -ErrorAction SilentlyContinue
    if ([string]::IsNullOrWhiteSpace($raw)) {
        Fail ("VSP Hub HTTP asset smoke did not emit a JSON summary. log=" + $stdoutPath)
    }
    try {
        $assetSummary = $raw | ConvertFrom-Json
    }
    catch {
        Fail ("VSP Hub HTTP asset smoke summary was not JSON. log=" + $stdoutPath + " error=" + [string]$_.Exception.Message)
    }
    ConvertTo-JsonFile -Value $assetSummary -Path $summaryPath
    return $assetSummary
}

function Invoke-VspHub60TrackPlaybackSmoke {
    param(
        [string]$RepoRoot,
        [string]$HubUrl,
        [string]$ArtifactDir,
        [int]$TimeoutSeconds,
        [int]$TrackCount,
        [switch]$KeepImportedTracks
    )
    $scriptPath = Join-Path $RepoRoot "scripts\vsp_hub_60_track_playback_smoke.py"
    if (-not (Test-Path -LiteralPath $scriptPath)) {
        Fail ("Missing VSP Hub 60-track playback smoke script: " + $scriptPath)
    }
    $stdoutPath = Join-Path $ArtifactDir "vsp_hub_60_track_playback_smoke_stdout.log"
    $stderrPath = Join-Path $ArtifactDir "vsp_hub_60_track_playback_smoke_stderr.log"
    $summaryPath = Join-Path $ArtifactDir "vsp_hub_60_track_playback_smoke.json"
    $timeoutSec = [Math]::Max(120, $TimeoutSeconds)
    $timeoutMs = [Math]::Max(180000, $timeoutSec * 1000)
    $args = @(
        $scriptPath,
        "--hub-url", $HubUrl,
        "--track-count", [string]$TrackCount,
        "--timeout-ms", [string]$timeoutMs,
        "--timeout-sec", [string]$timeoutSec
    )
    if ($KeepImportedTracks) {
        $mediaDir = Join-Path $ArtifactDir "vsp_hub_60_track_playback_media"
        New-Item -ItemType Directory -Path $mediaDir -Force | Out-Null
        $args += @("--keep-imported-tracks", "--media-dir", $mediaDir)
    }
    $proc = Start-Process -FilePath "python" -ArgumentList $args -WorkingDirectory $RepoRoot -RedirectStandardOutput $stdoutPath -RedirectStandardError $stderrPath -PassThru -Wait -WindowStyle Hidden
    if ($proc.ExitCode -ne 0) {
        $err = ""
        if (Test-Path -LiteralPath $stderrPath) {
            $err = Get-Content -LiteralPath $stderrPath -Raw -ErrorAction SilentlyContinue
        }
        $out = ""
        if (Test-Path -LiteralPath $stdoutPath) {
            $out = Get-Content -LiteralPath $stdoutPath -Raw -ErrorAction SilentlyContinue
        }
        Fail ("VSP Hub 60-track playback smoke failed with exit code " + [string]$proc.ExitCode + ". stdout=" + $out + " stderr=" + $err)
    }
    $raw = Get-Content -LiteralPath $stdoutPath -Raw -ErrorAction SilentlyContinue
    if ([string]::IsNullOrWhiteSpace($raw)) {
        Fail ("VSP Hub 60-track playback smoke did not emit a JSON summary. log=" + $stdoutPath)
    }
    try {
        $playbackSummary = $raw | ConvertFrom-Json
    }
    catch {
        Fail ("VSP Hub 60-track playback smoke summary was not JSON. log=" + $stdoutPath + " error=" + [string]$_.Exception.Message)
    }
    if ([string]$playbackSummary.status -ne "ok") {
        Fail ("VSP Hub 60-track playback smoke status was not ok: " + ($playbackSummary | ConvertTo-Json -Depth 12 -Compress))
    }
    ConvertTo-JsonFile -Value $playbackSummary -Path $summaryPath
    return $playbackSummary
}

$RepoRoot = (Resolve-Path -LiteralPath $RepoRoot -ErrorAction Stop).Path
$GodotProjectRoot = Resolve-GodotProjectRoot -RequestedProjectRoot $GodotProjectRoot
$GodotExe = Resolve-GodotExecutable -RequestedGodotExe $GodotExe -ProjectRoot $GodotProjectRoot
$GodotLaunchExe = Resolve-GodotLaunchExecutable -GodotExe $GodotExe
if ([string]::IsNullOrWhiteSpace($KernelExe)) {
    $KernelExe = Resolve-FirstExistingPath -Candidates @(
        (Join-Path $RepoRoot "VitApp\build\VitApp_artefacts\Release\VitApp.exe"),
        (Join-Path $RepoRoot "VitApp\build_release\VitApp_artefacts\Release\VitApp.exe"),
        (Join-Path $RepoRoot "VitApp\build_release\VitApp.exe")
    ) -Label "VitApp kernel"
}
else {
    $KernelExe = [System.IO.Path]::GetFullPath($KernelExe)
}
if ([string]::IsNullOrWhiteSpace($AgentExe)) {
    $AgentExe = Join-Path $RepoRoot "agent\bin\VitAgent.exe"
}
$AgentExe = [System.IO.Path]::GetFullPath($AgentExe)
if ([string]::IsNullOrWhiteSpace($VspHubExe)) {
    $VspHubExe = Join-Path $RepoRoot "agent\bin\VspHub.exe"
}
$VspHubExe = [System.IO.Path]::GetFullPath($VspHubExe)

$stamp = Get-Date -Format "yyyyMMdd_HHmmss"
$ArtifactDir = Join-Path $RepoRoot ("VitApp\Workspace\Artifacts\smoke\vsp_hub_lifecycle_" + $stamp)
New-Item -ItemType Directory -Path $ArtifactDir -Force | Out-Null
$GodotRuntimeStdout = Join-Path $ArtifactDir "godot_runtime_stdout.log"
$GodotRuntimeStderr = Join-Path $ArtifactDir "godot_runtime_stderr.log"
$AgentLog = Join-Path $ArtifactDir "agent_last.log"
$HubLog = Join-Path $ArtifactDir "vsp_hub_last.log"

$summary = [ordered]@{
    schema_version = "vsp_hub_lifecycle_smoke.v1"
    created_at = (Get-Date).ToString("o")
    repo_root = $RepoRoot
    artifact_dir = $ArtifactDir
    godot_project_root = $GodotProjectRoot
    godot_exe = $GodotExe
    godot_launch_exe = $GodotLaunchExe
    kernel_exe = $KernelExe
    vsp_hub_exe = $VspHubExe
    agent_exe = $AgentExe
    godot_runtime_stdout = $GodotRuntimeStdout
    godot_runtime_stderr = $GodotRuntimeStderr
    godot_lifecycle_evidence = $null
    playback_import_handoff = $null
    playback_import_cleanup = $null
    processes = @{}
    binary_evidence = @{}
    ports = @()
    status = "running"
}

$started = @{
    godot = $null
    kernel = $null
    hub = $null
    agent = $null
}
$playbackProbe = $null
$playbackImportsRetainedForGuiProbe = $false
$playbackImportsCleaned = $false
$oldDevRoot = $env:VIT_DAW_DEV_ROOT
$oldRoot = $env:VIT_DAW_ROOT
$oldHubLog = $env:VIT_VSP_HUB_LAST_LOG_PATH
$oldAgentHubUrl = $env:VIT_AGENT_VSP_HUB_URL
$oldAgentLog = $env:VIT_AGENT_LAST_LOG_PATH
$oldAgentKeepLogLines = $env:VIT_AGENT_KEEP_LAST_LOG_LINES
$oldLegacyTelemetryDiag = $env:VIT_BRIDGE_LEGACY_TELEMETRY_DIAG
$oldKeepLegacyRealtimeUdp = $env:VIT_BRIDGE_KEEP_LEGACY_REALTIME_UDP
$oldSkipDevAutostart = $env:VIT_SKIP_DEV_AUTOSTART

try {
    Write-Step "VSP Hub product lifecycle smoke"
    Write-Host ("repo: " + $RepoRoot)
    Write-Host ("artifact_dir: " + $ArtifactDir)
    Write-Host ("godot_project: " + $GodotProjectRoot)
    Write-Host ("kernel: " + $KernelExe)
    Write-Host ("hub: " + $VspHubExe)
    Write-Host ("agent: " + $AgentExe)

    Write-Step "Prepare binaries"
    if (-not $SkipBuild) {
        Stop-PortOwnerIfNeeded -Port $AgentHttpPort -Label "agent HTTP" -Reuse ([bool]$ReuseAgent)
        Stop-PortOwnerIfNeeded -Port $VspHubPort -Label "VSP Hub HTTP" -Reuse ([bool]$ReuseHub)
    }
    Build-AgentAndHubIfNeeded -RepoRoot $RepoRoot -AgentPath $AgentExe -VspHubPath $VspHubExe -SkipAgentBuild ([bool]($SkipBuild -or $ReuseAgent)) -SkipHubBuild ([bool]($SkipBuild -or $ReuseHub))
    $summary["binary_evidence"]["agent_expected"] = Get-ExecutableEvidence -Path $AgentExe
    $summary["binary_evidence"]["hub_expected"] = Get-ExecutableEvidence -Path $VspHubExe
    ConvertTo-JsonFile -Value ($summary["binary_evidence"]["agent_expected"]) -Path (Join-Path $ArtifactDir "agent_expected_binary.json")
    ConvertTo-JsonFile -Value ($summary["binary_evidence"]["hub_expected"]) -Path (Join-Path $ArtifactDir "hub_expected_binary.json")

    Write-Step "Start Godot lifecycle"
    Stop-GodotRuntimeIfNeeded -ProjectRoot $GodotProjectRoot -Reuse ([bool]$ReuseGodot)
    Stop-PortOwnerIfNeeded -Port $AgentHttpPort -Label "agent HTTP" -Reuse ([bool]$ReuseAgent)
    Stop-PortOwnerIfNeeded -Port $VspHubPort -Label "VSP Hub HTTP" -Reuse ([bool]$ReuseHub)
    Stop-PortOwnerIfNeeded -Port $ZmqReqPort -Label "kernel command" -Reuse ([bool]$ReuseKernel)
    Stop-PortOwnerIfNeeded -Port $ZmqSubPort -Label "kernel event" -Reuse ([bool]$ReuseKernel)

    $env:VIT_DAW_DEV_ROOT = $RepoRoot
    $env:VIT_DAW_ROOT = $RepoRoot
    $env:VIT_VSP_HUB_LAST_LOG_PATH = $HubLog
    $env:VIT_AGENT_VSP_HUB_URL = ($VspHubHttp.TrimEnd("/") + "/vsp")
    $env:VIT_AGENT_LAST_LOG_PATH = $AgentLog
    $env:VIT_AGENT_KEEP_LAST_LOG_LINES = "1200"
    $env:VIT_BRIDGE_LEGACY_TELEMETRY_DIAG = "1"
    $env:VIT_BRIDGE_KEEP_LEGACY_REALTIME_UDP = "0"
    $env:VIT_SKIP_DEV_AUTOSTART = "0"

    $projectRuntimeLog = Join-Path $GodotProjectRoot "godot_runtime.log"
    if (Test-Path -LiteralPath $projectRuntimeLog) {
        Copy-Item -LiteralPath $projectRuntimeLog -Destination (Join-Path $ArtifactDir "godot_runtime_previous.log") -Force
        Remove-Item -LiteralPath $projectRuntimeLog -Force
    }

    $started.godot = Start-Or-Reuse-GodotProject -GodotLaunchExe $GodotLaunchExe -ProjectRoot $GodotProjectRoot -StdoutLog $GodotRuntimeStdout -StderrLog $GodotRuntimeStderr -Reuse ([bool]$ReuseGodot) -TimeoutSeconds $TimeoutSeconds
    $summary["processes"]["godot_runtime"] = $started.godot

    $lifecycleEvidence = Wait-GodotLifecycleEvidence -StdoutLog $GodotRuntimeStdout -AlternateLog $projectRuntimeLog -TimeoutSeconds $TimeoutSeconds
    if ([string]$lifecycleEvidence.mode -eq "missing_godot_lifecycle_evidence") {
        Fail "Missing Godot lifecycle evidence: no Hub/Agent self-check with live Kernel/Hub/Agent ports"
    }
    $summary["godot_lifecycle_evidence"] = @{
        mode = [string]$lifecycleEvidence.mode
        vsp_hub_ready_log = [bool]$lifecycleEvidence.vsp_hub_ready_log
        agent_tools_ready_log = [bool]$lifecycleEvidence.agent_tools_ready_log
        autostart = @($lifecycleEvidence.autostart)
    }
    ConvertTo-JsonFile -Value ($summary["godot_lifecycle_evidence"]) -Path (Join-Path $ArtifactDir "godot_lifecycle_evidence.json")

    Write-Step "Verify process ownership"
    $started.hub = Assert-PortOwnerExpectedPath -Port $VspHubPort -Label "VSP Hub HTTP" -ExpectedPath $VspHubExe -AllowExisting ([bool]$ReuseHub)
    $started.agent = Assert-PortOwnerExpectedPath -Port $AgentHttpPort -Label "agent HTTP" -ExpectedPath $AgentExe -AllowExisting ([bool]$ReuseAgent)
    $started.kernel = Assert-PortOwnerExpectedPath -Port $ZmqReqPort -Label "kernel command" -ExpectedPath $KernelExe -AllowExisting ([bool]$ReuseKernel)
    $kernelEventOwner = Assert-PortOwnerExpectedPath -Port $ZmqSubPort -Label "kernel event" -ExpectedPath $KernelExe -AllowExisting ([bool]$ReuseKernel)
    $started.kernel["event_port_pid"] = [int]$kernelEventOwner.pid
    $summary["processes"]["hub"] = $started.hub
    $summary["processes"]["agent"] = $started.agent
    $summary["processes"]["kernel"] = $started.kernel

    $summary["binary_evidence"]["hub_running"] = Get-ExecutableEvidence -Path ([string]$started.hub.path)
    $summary["binary_evidence"]["agent_running"] = Get-ExecutableEvidence -Path ([string]$started.agent.path)
    if ([string]$summary["binary_evidence"]["hub_expected"].sha256 -ne [string]$summary["binary_evidence"]["hub_running"].sha256) {
        Fail "Running VSP Hub binary hash does not match expected Hub binary"
    }
    if ([string]$summary["binary_evidence"]["agent_expected"].sha256 -ne [string]$summary["binary_evidence"]["agent_running"].sha256) {
        Fail "Running agent binary hash does not match expected agent binary"
    }
    Write-Ok ("VSP Hub binary verified sha256=" + [string]$summary["binary_evidence"]["hub_running"].sha256)
    Write-Ok ("VitAgent binary verified sha256=" + [string]$summary["binary_evidence"]["agent_running"].sha256)

    $summary["ports"] = @(Get-ListenersSnapshot)
    ConvertTo-JsonFile -Value ($summary["ports"]) -Path (Join-Path $ArtifactDir "ports.json")
    ConvertTo-JsonFile -Value ($summary["processes"]) -Path (Join-Path $ArtifactDir "processes.json")

    Write-Step "Verify Hub health and status"
    $hubHealth = Invoke-Json -Method GET -Uri ($VspHubHttp.TrimEnd("/") + "/health") -TimeoutSec 10
    ConvertTo-JsonFile -Value $hubHealth -Path (Join-Path $ArtifactDir "vsp_hub_health.json")
    if ($null -eq $hubHealth -or [string]$hubHealth.status -ne "ok" -or [string]$hubHealth.service -ne "VspHub") {
        Fail "GET /health did not return VspHub ok"
    }
    $hubStatus = Wait-VspHubAgentSession -TimeoutSeconds 20
    ConvertTo-JsonFile -Value $hubStatus -Path (Join-Path $ArtifactDir "vsp_hub_status.json")
    $transports = @($hubStatus.transports)
    if (-not ($transports -contains "vsp.hub.http")) {
        Fail "VSP Hub status did not advertise vsp.hub.http"
    }
    $summary["hub_health"] = $hubHealth
    $summary["hub_status_session_count"] = [int]$hubStatus.session_count
    Write-Ok "VSP Hub health/status passed with VitAgent session"

    Write-Step "Verify Godot realtime WebSocket consumption"
    $wsProbe = Invoke-GodotVspRealtimeWebSocketProbe -GodotLaunchExe $GodotLaunchExe -ProjectRoot $GodotProjectRoot -HubUrl ($VspHubHttp.TrimEnd("/") + "/vsp") -ArtifactDir $ArtifactDir -TimeoutSeconds $TimeoutSeconds
    $summary["godot_vsp_realtime_ws_probe"] = $wsProbe
    Write-Ok ("Godot realtime WebSocket probe passed transport=" + [string]$wsProbe.subscription_transport)

    if (-not $SkipHubHttpAssetSmoke) {
        Write-Step "Verify Hub HTTP asset manifest consumption"
        $assetProbe = Invoke-VspHubHttpAssetSmoke -RepoRoot $RepoRoot -HubUrl ($VspHubHttp.TrimEnd("/") + "/vsp") -ArtifactDir $ArtifactDir -TimeoutSeconds $TimeoutSeconds
        $summary["vsp_hub_http_asset_smoke"] = $assetProbe
        Write-Ok ("VSP Hub HTTP asset smoke passed checks=" + [string](@($assetProbe.checks).Count))
    }
    else {
        $summary["vsp_hub_http_asset_smoke"] = [ordered]@{ status = "skipped"; reason = "explicit_skip" }
        Write-Ok "VSP Hub HTTP asset smoke skipped by request"
    }

    if (-not [string]::IsNullOrWhiteSpace($CleanupPlaybackImportSummaryPath)) {
        Write-Step "Cleanup retained playback imports from summary"
        $cleanupSummaryPath = (Resolve-Path -LiteralPath $CleanupPlaybackImportSummaryPath -ErrorAction Stop).Path
        $playbackProbe = Get-Content -LiteralPath $cleanupSummaryPath -Raw -ErrorAction Stop | ConvertFrom-Json
        $cleanup = Invoke-VspPlaybackImportCleanup -HubUrl ($VspHubHttp.TrimEnd("/") + "/vsp") -PlaybackSummary $playbackProbe -ArtifactDir $ArtifactDir -TimeoutSeconds $TimeoutSeconds -Strict
        $summary["playback_import_cleanup"] = $cleanup
        $playbackImportsCleaned = ([string]$cleanup.status -eq "ok")
        Write-Ok ("Retained playback import cleanup passed tracks_deleted=" + [string]$cleanup.tracks_deleted + " clips_removed=" + [string]$cleanup.clips_removed)
    }

    if ($Run60TrackPlaybackSmoke) {
        Write-Step "Verify 60-track VSP Hub playback acceptance"
        $playbackImportsRetainedForGuiProbe = [bool]$KeepPlaybackImportedTracksForGuiProbe
        if (-not $playbackImportsRetainedForGuiProbe -and $PlaybackTrackCount -ge 60 -and ($RunGuiHealthPlaybackProbe -or $RunGuiArchitectureAuditProbe)) {
            $playbackImportsRetainedForGuiProbe = $true
        }
        $playbackHandoffReason = "playback_smoke_self_cleanup"
        if ($playbackImportsRetainedForGuiProbe) {
            $playbackHandoffReason = "gui_probe_requires_imported_project_state"
        }
        $summary["playback_import_handoff"] = [ordered]@{
            retained_for_gui_probe = $playbackImportsRetainedForGuiProbe
            requested_track_count = $PlaybackTrackCount
            reason = $playbackHandoffReason
        }
        $playbackProbe = Invoke-VspHub60TrackPlaybackSmoke -RepoRoot $RepoRoot -HubUrl ($VspHubHttp.TrimEnd("/") + "/vsp") -ArtifactDir $ArtifactDir -TimeoutSeconds $TimeoutSeconds -TrackCount $PlaybackTrackCount -KeepImportedTracks:$playbackImportsRetainedForGuiProbe
        $summary["vsp_hub_60_track_playback_smoke"] = $playbackProbe
        Write-Ok ("VSP Hub 60-track playback smoke passed tracks=" + [string]$playbackProbe.created_track_count + " checks=" + [string](@($playbackProbe.checks).Count))
    }

    if ($RunGuiHealthPlaybackProbe) {
        Write-Step "Verify Godot GUI health during VSP playback"
        $requiredGuiTracks = $GuiHealthMinTrackCount
        if ($requiredGuiTracks -lt 0) {
            if ($Run60TrackPlaybackSmoke) {
                $requiredGuiTracks = $PlaybackTrackCount
            }
            else {
                $requiredGuiTracks = 0
            }
        }
        $guiHealthProbe = Invoke-GodotGuiHealthPlaybackProbe -GodotLaunchExe $GodotLaunchExe -ProjectRoot $GodotProjectRoot -HubUrl ($VspHubHttp.TrimEnd("/") + "/vsp") -ArtifactDir $ArtifactDir -TimeoutSeconds $TimeoutSeconds -DurationSeconds $GuiHealthPlaybackSeconds -MinTrackCount $requiredGuiTracks -InputAudit ([bool]$GuiHealthInputAudit) -InputAuditIntervalSeconds $GuiHealthInputAuditIntervalSeconds -InputAuditMinClicks $GuiHealthInputAuditMinClicks -InputAuditActions $GuiHealthInputAuditActions -Layout $GuiHealthPlaybackLayout -DynamicMode $GuiHealthPlaybackDynamicMode
        $summary["godot_gui_health_playback_probe"] = $guiHealthProbe
        Write-Ok ("Godot GUI health playback probe passed tracks=" + [string]$guiHealthProbe.track_count_user_visible + " max_frame_ms=" + [string]$guiHealthProbe.max_frame_ms + " slow_frames=" + [string]$guiHealthProbe.slow_frames + " transport_ok=" + [string]$guiHealthProbe.transport_client_ok + " input_clicks=" + [string]$guiHealthProbe.input_audit_clicks)
    }

    if ($RunGuiArchitectureAuditProbe) {
        Write-Step "Audit Godot GUI architecture hotspots"
        $requiredAuditTracks = $GuiArchitectureAuditMinTrackCount
        if ($requiredAuditTracks -lt 0) {
            if ($Run60TrackPlaybackSmoke) {
                $requiredAuditTracks = $PlaybackTrackCount
            }
            else {
                $requiredAuditTracks = 0
            }
        }
        if ([string]::IsNullOrWhiteSpace($GuiArchitectureAuditProfiles)) {
            $guiArchProbe = Invoke-GodotGuiArchitectureAuditProbe -GodotLaunchExe $GodotLaunchExe -ProjectRoot $GodotProjectRoot -HubUrl ($VspHubHttp.TrimEnd("/") + "/vsp") -ArtifactDir $ArtifactDir -TimeoutSeconds $TimeoutSeconds -DurationSeconds $GuiArchitectureAuditSeconds -MinTrackCount $requiredAuditTracks -Layout $GuiArchitectureAuditLayout -DynamicMode $GuiArchitectureAuditDynamicMode -InputAudit ([bool]$GuiArchitectureAuditInputAudit) -InputAuditIntervalSeconds $GuiArchitectureAuditInputAuditIntervalSeconds -InputAuditMinClicks $GuiArchitectureAuditInputAuditMinClicks -InputAuditActions $GuiArchitectureAuditInputAuditActions -RackFixture $GuiArchitectureAuditRackFixture -RackFixturePluginCount $GuiArchitectureAuditRackFixturePluginCount -RackFixtureConnectionCount $GuiArchitectureAuditRackFixtureConnectionCount
            $summary["godot_gui_architecture_audit_probe"] = $guiArchProbe
            Write-Ok ("Godot GUI architecture audit passed layout=" + [string]$guiArchProbe.layout + " mode=" + [string]$guiArchProbe.dynamic_mode + " repo_tracks=" + [string]$guiArchProbe.repository_tracks + " virtual=" + [string]$guiArchProbe.virtual_active + " rows=" + [string]$guiArchProbe.track_rows + " processing=" + [string]$guiArchProbe.processing_nodes + " fast_timers=" + [string]$guiArchProbe.fast_active_timers + " max_frame_ms=" + [string]$guiArchProbe.sample_max_frame_ms + " input_clicks=" + [string]$guiArchProbe.input_audit_clicks + " input_failures=" + [string]$guiArchProbe.input_audit_failures + " hotspots=" + [string]$guiArchProbe.hotspot_count)
        }
        else {
            $guiArchProfiles = Invoke-GodotGuiArchitectureAuditProfileSet -GodotLaunchExe $GodotLaunchExe -ProjectRoot $GodotProjectRoot -HubUrl ($VspHubHttp.TrimEnd("/") + "/vsp") -ArtifactDir $ArtifactDir -TimeoutSeconds $TimeoutSeconds -DefaultDurationSeconds $GuiArchitectureAuditSeconds -DefaultMinTrackCount $requiredAuditTracks -DefaultLayout $GuiArchitectureAuditLayout -DefaultDynamicMode $GuiArchitectureAuditDynamicMode -ProfilesText $GuiArchitectureAuditProfiles -DefaultInputAudit ([bool]$GuiArchitectureAuditInputAudit) -DefaultInputAuditIntervalSeconds $GuiArchitectureAuditInputAuditIntervalSeconds -DefaultInputAuditMinClicks $GuiArchitectureAuditInputAuditMinClicks -DefaultInputAuditActions $GuiArchitectureAuditInputAuditActions -DefaultRackFixture $GuiArchitectureAuditRackFixture -DefaultRackFixturePluginCount $GuiArchitectureAuditRackFixturePluginCount -DefaultRackFixtureConnectionCount $GuiArchitectureAuditRackFixtureConnectionCount
            $summary["godot_gui_architecture_audit_profiles"] = $guiArchProfiles
            Write-Ok ("Godot GUI architecture audit profile set passed profiles=" + [string]$guiArchProfiles.profile_count)
        }
    }

    if ($playbackImportsRetainedForGuiProbe -and -not $playbackImportsCleaned) {
        Write-Step "Cleanup playback imports retained for GUI probe"
        $cleanup = Invoke-VspPlaybackImportCleanup -HubUrl ($VspHubHttp.TrimEnd("/") + "/vsp") -PlaybackSummary $playbackProbe -ArtifactDir $ArtifactDir -TimeoutSeconds $TimeoutSeconds -Strict
        $summary["playback_import_cleanup"] = $cleanup
        $playbackImportsCleaned = ([string]$cleanup.status -eq "ok")
        Write-Ok ("Playback import cleanup passed tracks_deleted=" + [string]$cleanup.tracks_deleted + " clips_removed=" + [string]$cleanup.clips_removed)
    }

    $summary["status"] = "ok"
    ConvertTo-JsonFile -Value $summary -Path (Join-Path $ArtifactDir "summary.json")
    Write-Ok ("VSP Hub lifecycle smoke passed. artifact_dir=" + $ArtifactDir)
}
catch {
    if ($playbackImportsRetainedForGuiProbe -and -not $playbackImportsCleaned) {
        try {
            $cleanup = Invoke-VspPlaybackImportCleanup -HubUrl ($VspHubHttp.TrimEnd("/") + "/vsp") -PlaybackSummary $playbackProbe -ArtifactDir $ArtifactDir -TimeoutSeconds $TimeoutSeconds
            $summary["playback_import_cleanup"] = $cleanup
            $playbackImportsCleaned = ([string]$cleanup.status -eq "ok")
        }
        catch {
            Write-WarnLine ("failed-path playback import cleanup threw: " + [string]$_.Exception.Message)
        }
    }
    $summary["status"] = "failed"
    $summary["error"] = [string]$_.Exception.Message
    ConvertTo-JsonFile -Value $summary -Path (Join-Path $ArtifactDir "summary.json")
    Write-WarnLine ("failed: " + [string]$_.Exception.Message)
    throw
}
finally {
    if (-not $KeepProcesses) {
        if ($null -ne $started.godot -and [bool]$started.godot.started) {
            Stop-GodotRuntimeIfNeeded -ProjectRoot $GodotProjectRoot -Reuse $false -Quiet $true
            Stop-ProcessByID -ProcessID ([int]$started.godot.pid)
        }
        if (-not $ReuseAgent) {
            $listener = Get-TcpListener -Port $AgentHttpPort
            if ($null -ne $listener) {
                $ownerPath = Get-ProcessPathByID -ProcessID ([int]$listener.OwningProcess)
                if ((Normalize-ComparablePath -Path $ownerPath) -eq (Normalize-ComparablePath -Path $AgentExe)) {
                    Stop-ProcessByID -ProcessID ([int]$listener.OwningProcess)
                }
            }
        }
        if (-not $ReuseHub) {
            $listener = Get-TcpListener -Port $VspHubPort
            if ($null -ne $listener) {
                $ownerPath = Get-ProcessPathByID -ProcessID ([int]$listener.OwningProcess)
                if ((Normalize-ComparablePath -Path $ownerPath) -eq (Normalize-ComparablePath -Path $VspHubExe)) {
                    Stop-ProcessByID -ProcessID ([int]$listener.OwningProcess)
                }
            }
        }
        if (-not $ReuseKernel) {
            $listener = Get-TcpListener -Port $ZmqReqPort
            if ($null -ne $listener) {
                $ownerPath = Get-ProcessPathByID -ProcessID ([int]$listener.OwningProcess)
                if ((Normalize-ComparablePath -Path $ownerPath) -eq (Normalize-ComparablePath -Path $KernelExe)) {
                    Stop-ProcessByID -ProcessID ([int]$listener.OwningProcess)
                }
            }
        }
    }
    $env:VIT_DAW_DEV_ROOT = $oldDevRoot
    $env:VIT_DAW_ROOT = $oldRoot
    $env:VIT_VSP_HUB_LAST_LOG_PATH = $oldHubLog
    $env:VIT_AGENT_VSP_HUB_URL = $oldAgentHubUrl
    $env:VIT_AGENT_LAST_LOG_PATH = $oldAgentLog
    $env:VIT_AGENT_KEEP_LAST_LOG_LINES = $oldAgentKeepLogLines
    $env:VIT_BRIDGE_LEGACY_TELEMETRY_DIAG = $oldLegacyTelemetryDiag
    $env:VIT_BRIDGE_KEEP_LEGACY_REALTIME_UDP = $oldKeepLegacyRealtimeUdp
    $env:VIT_SKIP_DEV_AUTOSTART = $oldSkipDevAutostart
}
