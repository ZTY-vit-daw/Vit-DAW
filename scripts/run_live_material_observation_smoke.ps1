[CmdletBinding()]
param(
    [string]$RepoRoot = "D:\Vit_DAW",
    [string]$AgentHttp = "http://127.0.0.1:7878",
    [string]$AgentHttpAddr = "127.0.0.1:7878",
    [string]$ZmqReqPort = "5555",
    [string]$ZmqSubPort = "5556",
    [string]$KernelExe = "",
    [string[]]$MaterialPaths = @(),
    [switch]$SkipBuild,
    [switch]$ReuseAgent,
    [switch]$ReuseKernel,
    [switch]$KeepProcesses,
    [int]$WaitSeconds = 45,
    [int]$ObservationTimeoutSec = 180
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

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

function Get-OptionalProperty {
    param(
        [object]$Object,
        [string]$Name
    )
    if ($null -eq $Object -or [string]::IsNullOrWhiteSpace($Name)) {
        return $null
    }
    if ($Object -is [System.Collections.IDictionary] -and $Object.Contains($Name)) {
        return $Object[$Name]
    }
    $prop = $Object.PSObject.Properties[$Name]
    if ($null -eq $prop) {
        return $null
    }
    return $prop.Value
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

function Get-TcpListener {
    param([int]$Port)
    return Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue |
        Select-Object -First 1
}

function Wait-TcpListener {
    param(
        [int]$Port,
        [int]$TimeoutSeconds
    )
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    do {
        $listener = Get-TcpListener -Port $Port
        if ($null -ne $listener) {
            return $listener
        }
        Start-Sleep -Milliseconds 500
    } while ((Get-Date) -lt $deadline)
    return $null
}

function Wait-HttpReady {
    param(
        [string]$BaseUrl,
        [int]$TimeoutSeconds
    )
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    do {
        try {
            $resp = Invoke-WebRequest -UseBasicParsing -Uri ($BaseUrl.TrimEnd("/") + "/health") -TimeoutSec 2
            if ($resp.StatusCode -ge 200 -and $resp.StatusCode -lt 500) {
                return $true
            }
        }
        catch {
            Start-Sleep -Milliseconds 500
        }
    } while ((Get-Date) -lt $deadline)
    return $false
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

function Invoke-AgentTool {
    param(
        [string]$Tool,
        [object]$ToolArgs = $null,
        [bool]$Confirmed = $true,
        [int]$TimeoutSec = 120
    )
    if ($null -eq $ToolArgs) {
        $ToolArgs = @{}
    }
    return Invoke-Json -Method POST -Uri ($AgentHttp.TrimEnd("/") + "/agent/invoke") -Body @{
        tool = $Tool
        args = $ToolArgs
        confirmed = $Confirmed
        source = "live_material_observation_smoke"
    } -TimeoutSec $TimeoutSec
}

function Assert-StatusOk {
    param(
        [object]$Response,
        [string]$Label
    )
    $status = [string](Get-OptionalProperty -Object $Response -Name "status")
    if ($status -ne "ok") {
        $errorText = [string](Get-OptionalProperty -Object $Response -Name "error")
        Fail ($Label + " failed: status=" + $status + " error=" + $errorText)
    }
}

function Test-TruthLike {
    param([object]$Value)
    if ($null -eq $Value) {
        return $false
    }
    if ($Value -is [bool]) {
        return [bool]$Value
    }
    $text = ([string]$Value).Trim().ToLowerInvariant()
    return ($text -in @("true", "1", "yes", "y"))
}

function Test-TypedEventsContainForbiddenConfirmation {
    param([object[]]$Events)
    foreach ($event in @($Events)) {
        $eventType = [string](Get-OptionalProperty -Object $event -Name "event_type")
        $stateKind = [string](Get-OptionalProperty -Object $event -Name "state_kind")
        $kind = [string](Get-OptionalProperty -Object $event -Name "kind")
        if ($eventType -in @("PendingCandidate", "ApprovalRequest") -or
            $stateKind -in @("PendingCandidate", "ApprovalRequest") -or
            $kind -in @("PendingCandidate", "ApprovalRequest")) {
            return $true
        }
    }
    return $false
}

function Assert-NoPendingOrConfirmation {
    param(
        [object]$Response,
        [string]$Label
    )
    $result = Get-OptionalProperty -Object $Response -Name "result"
    foreach ($container in @($Response, $result)) {
        if ($null -eq $container) {
            continue
        }
        if (Test-TruthLike -Value (Get-OptionalProperty -Object $container -Name "needs_confirmation")) {
            Fail ($Label + " unexpectedly set needs_confirmation")
        }
        if (Test-TruthLike -Value (Get-OptionalProperty -Object $container -Name "requires_confirmation")) {
            Fail ($Label + " unexpectedly set requires_confirmation")
        }
        $interactionRequests = Get-OptionalProperty -Object $container -Name "interaction_requests"
        if ($null -ne $interactionRequests -and @($interactionRequests).Count -gt 0) {
            Fail ($Label + " unexpectedly returned interaction_requests")
        }
    }
    $events = @()
    $events += @(Get-OptionalProperty -Object $Response -Name "typed_events")
    $events += @(Get-OptionalProperty -Object $result -Name "typed_events")
    if (Test-TypedEventsContainForbiddenConfirmation -Events $events) {
        Fail ($Label + " unexpectedly emitted PendingCandidate/ApprovalRequest typed event")
    }
}

function Assert-MOMV13MultitrackProjection {
    param(
        [object]$Observation,
        [string]$Label
    )
    $projection = Get-OptionalProperty -Object $Observation -Name "mom_projection"
    if ($null -eq $projection) {
        Fail ($Label + " missing mom_projection")
    }
    if ([string](Get-OptionalProperty -Object $projection -Name "mom_version") -ne "v1.5") {
        Fail ($Label + " unexpected mom_version")
    }
    if ([string](Get-OptionalProperty -Object $projection -Name "intent") -ne "project_multitrack_relation_observation") {
        Fail ($Label + " unexpected MOM intent")
    }
    $profile = Get-OptionalProperty -Object $projection -Name "project_mix_profile"
    $relation = Get-OptionalProperty -Object $projection -Name "multitrack_relation"
    if ($null -eq $profile -or $null -eq $relation) {
        Fail ($Label + " missing project_mix_profile or multitrack_relation")
    }
    if ([int](Get-OptionalProperty -Object $profile -Name "track_count") -lt 2) {
        Fail ($Label + " project_mix_profile track_count < 2")
    }
    if ([int](Get-OptionalProperty -Object $relation -Name "track_count") -lt 2) {
        Fail ($Label + " multitrack_relation track_count < 2")
    }
    $llmContext = Get-OptionalProperty -Object $projection -Name "llm_context"
    if ($null -eq $llmContext) {
        Fail ($Label + " missing llm_context")
    }
    if (-not (Test-TruthLike -Value (Get-OptionalProperty -Object $llmContext -Name "do_not_include_raw_package"))) {
        Fail ($Label + " llm_context.do_not_include_raw_package was not true")
    }
    $compact = $llmContext | ConvertTo-Json -Depth 24 -Compress
    foreach ($forbidden in @('"time_segments":', '"track_waveform_envelopes":', '"spectrogram_tile_rows":')) {
        if ($compact.Contains($forbidden)) {
            Fail ($Label + " leaked raw field " + $forbidden)
        }
    }
}

function Get-AcousticPackageStatusFromResponse {
    param([object]$Response)
    $result = Get-OptionalProperty -Object $Response -Name "result"
    $status = Get-OptionalProperty -Object $result -Name "acoustic_package_status"
    if ($null -eq $status) {
        $status = Get-OptionalProperty -Object $Response -Name "acoustic_package_status"
    }
    return $status
}

function Get-AcousticPackageStatusPathFromResponse {
    param([object]$Response)
    $result = Get-OptionalProperty -Object $Response -Name "result"
    $path = [string](Get-OptionalProperty -Object $result -Name "acoustic_package_status_path")
    if ([string]::IsNullOrWhiteSpace($path)) {
        $path = [string](Get-OptionalProperty -Object $Response -Name "acoustic_package_status_path")
    }
    return $path
}

function Get-AcousticPackageLayer {
    param(
        [object]$Status,
        [string]$LayerName
    )
    $layers = Get-OptionalProperty -Object $Status -Name "package_layers"
    return Get-OptionalProperty -Object $layers -Name $LayerName
}

function Get-AcousticPackageFeature {
    param(
        [object]$Status,
        [string]$LayerName,
        [string]$FeatureName
    )
    $layer = Get-AcousticPackageLayer -Status $Status -LayerName $LayerName
    $features = Get-OptionalProperty -Object $layer -Name "features"
    return Get-OptionalProperty -Object $features -Name $FeatureName
}

function Compact-AcousticFeatureStatus {
    param([object]$Feature)
    if ($null -eq $Feature) {
        return $null
    }
    $progress = Get-OptionalProperty -Object $Feature -Name "progress"
    $row = [ordered]@{
        status = [string](Get-OptionalProperty -Object $Feature -Name "status")
        source = [string](Get-OptionalProperty -Object $Feature -Name "source")
        reason = [string](Get-OptionalProperty -Object $Feature -Name "reason")
        progress = [ordered]@{
            tile_count_seen = Get-OptionalProperty -Object $progress -Name "tile_count_seen"
            tile_count_expected = Get-OptionalProperty -Object $progress -Name "tile_count_expected"
            coverage_seconds = Get-OptionalProperty -Object $progress -Name "coverage_seconds"
            coverage_ratio = Get-OptionalProperty -Object $progress -Name "coverage_ratio"
            reason = [string](Get-OptionalProperty -Object $progress -Name "reason")
        }
    }
    return $row
}

function Compact-AcousticPackageStatus {
    param([object]$Status)
    if ($null -eq $Status) {
        return $null
    }
    $out = [ordered]@{
        schema_version = [string](Get-OptionalProperty -Object $Status -Name "schema_version")
        status = [string](Get-OptionalProperty -Object $Status -Name "status")
        project_id = [string](Get-OptionalProperty -Object $Status -Name "project_id")
        track_id = [string](Get-OptionalProperty -Object $Status -Name "track_id")
        clip_id = [string](Get-OptionalProperty -Object $Status -Name "clip_id")
        source_revision = [string](Get-OptionalProperty -Object $Status -Name "source_revision")
        layers = [ordered]@{}
    }
    $featuresByLayer = [ordered]@{
        l1_static = @("waveform_envelope", "peak_rms_summary", "time_energy")
        l2_realtime = @("live_meter", "realtime_spectrum", "post_fx_meter", "realtime_stereo_correlation")
        l3_deep = @("spectrogram_tiles", "band_energy_summary", "stereo_relation_summary", "lufs_analysis", "masking_analysis", "reference_match")
    }
    foreach ($layerName in $featuresByLayer.Keys) {
        $layer = Get-AcousticPackageLayer -Status $Status -LayerName $layerName
        $layerRow = [ordered]@{
            status = [string](Get-OptionalProperty -Object $layer -Name "status")
            features = [ordered]@{}
        }
        foreach ($featureName in @($featuresByLayer[$layerName])) {
            $layerRow.features[$featureName] = Compact-AcousticFeatureStatus -Feature (Get-AcousticPackageFeature -Status $Status -LayerName $layerName -FeatureName $featureName)
        }
        $out.layers[$layerName] = $layerRow
    }
    return $out
}

function Assert-AcousticPackageStatusV0 {
    param(
        [object]$Status,
        [string]$Label
    )
    if ($null -eq $Status) {
        Fail ($Label + " missing acoustic_package_status")
    }
    if ([string](Get-OptionalProperty -Object $Status -Name "schema_version") -ne "acoustic_package_status.v0") {
        Fail ($Label + " schema_version is not acoustic_package_status.v0: " + ($Status | ConvertTo-Json -Depth 12 -Compress))
    }
    $allowed = @("ready", "partial", "building", "stale", "missing", "deferred", "failed")
    $featuresByLayer = @{
        l1_static = @("waveform_envelope", "peak_rms_summary", "time_energy")
        l2_realtime = @("live_meter", "realtime_spectrum", "post_fx_meter", "realtime_stereo_correlation")
        l3_deep = @("spectrogram_tiles", "band_energy_summary", "stereo_relation_summary", "lufs_analysis", "masking_analysis", "reference_match")
    }
    foreach ($layerName in $featuresByLayer.Keys) {
        $layer = Get-AcousticPackageLayer -Status $Status -LayerName $layerName
        if ($null -eq $layer) {
            Fail ($Label + " missing package layer " + $layerName)
        }
        $layerStatus = [string](Get-OptionalProperty -Object $layer -Name "status")
        if ($allowed -notcontains $layerStatus) {
            Fail ($Label + " layer " + $layerName + " has invalid status " + $layerStatus)
        }
        foreach ($featureName in @($featuresByLayer[$layerName])) {
            $feature = Get-AcousticPackageFeature -Status $Status -LayerName $layerName -FeatureName $featureName
            if ($null -eq $feature) {
                Fail ($Label + " missing feature " + $layerName + "." + $featureName)
            }
            $featureStatus = [string](Get-OptionalProperty -Object $feature -Name "status")
            if ($allowed -notcontains $featureStatus) {
                Fail ($Label + " feature " + $featureName + " has invalid status " + $featureStatus)
            }
        }
    }
    foreach ($future in @("lufs_analysis", "masking_analysis", "reference_match")) {
        $feature = Get-AcousticPackageFeature -Status $Status -LayerName "l3_deep" -FeatureName $future
        if ([string](Get-OptionalProperty -Object $feature -Name "status") -ne "deferred") {
            Fail ($Label + " future feature " + $future + " is not deferred")
        }
    }
}

function Assert-ImmediateAcousticPackageLifecycle {
    param(
        [object]$Status,
        [string]$Label
    )
    Assert-AcousticPackageStatusV0 -Status $Status -Label $Label
    foreach ($featureName in @("spectrogram_tiles", "band_energy_summary", "stereo_relation_summary")) {
        $feature = Get-AcousticPackageFeature -Status $Status -LayerName "l3_deep" -FeatureName $featureName
        $featureStatus = [string](Get-OptionalProperty -Object $feature -Name "status")
        if ($featureStatus -notin @("ready", "partial", "missing", "stale", "deferred")) {
            Fail ($Label + " l3_deep." + $featureName + " should report a read-model status without observe-time build/request, got " + $featureStatus)
        }
        if ($featureStatus -in @("partial", "missing", "stale")) {
            $progress = Get-OptionalProperty -Object $feature -Name "progress"
            $reason = [string](Get-OptionalProperty -Object $feature -Name "reason")
            $progressReason = [string](Get-OptionalProperty -Object $progress -Name "reason")
            $seen = Get-OptionalProperty -Object $progress -Name "tile_count_seen"
            $expected = Get-OptionalProperty -Object $progress -Name "tile_count_expected"
            $coverage = Get-OptionalProperty -Object $progress -Name "coverage_seconds"
            if ([string]::IsNullOrWhiteSpace($reason) -and [string]::IsNullOrWhiteSpace($progressReason) -and
                -not (Number-Present -Value $seen) -and -not (Number-Present -Value $expected) -and -not (Number-Present -Value $coverage)) {
                Fail ($Label + " l3_deep." + $featureName + " " + $featureStatus + " missing progress/reason")
            }
        }
    }
}

function Assert-AcousticPackageArtifact {
    param(
        [object]$Response,
        [string]$Label
    )
    $path = Get-AcousticPackageStatusPathFromResponse -Response $Response
    if ([string]::IsNullOrWhiteSpace($path)) {
        $path = Join-Path $RepoRoot "VitApp\Workspace\Artifacts\acoustic_package_status.json"
    }
    if (-not (Test-Path -LiteralPath $path)) {
        Fail ($Label + " acoustic package artifact missing: " + $path)
    }
    $json = Get-Content -LiteralPath $path -Raw | ConvertFrom-Json
    if ([string](Get-OptionalProperty -Object $json -Name "schema_version") -ne "acoustic_package_status.v0") {
        Fail ($Label + " artifact schema mismatch: " + $path)
    }
    return $path
}

function Get-L3CoverageRatio {
    param([object]$Status)
    $feature = Get-AcousticPackageFeature -Status $Status -LayerName "l3_deep" -FeatureName "spectrogram_tiles"
    $progress = Get-OptionalProperty -Object $feature -Name "progress"
    $ratio = Get-OptionalProperty -Object $progress -Name "coverage_ratio"
    if (Number-Present -Value $ratio) {
        return [double]::Parse(([string]$ratio).Trim(), [System.Globalization.CultureInfo]::InvariantCulture)
    }
    $seen = Get-OptionalProperty -Object $progress -Name "tile_count_seen"
    $expected = Get-OptionalProperty -Object $progress -Name "tile_count_expected"
    if ((Number-Present -Value $seen) -and (Number-Present -Value $expected) -and ([double]$expected) -gt 0) {
        return ([double]$seen) / ([double]$expected)
    }
    return $null
}

function Assert-AcousticCoverageNotRegressed {
    param(
        [object]$Before,
        [object]$After,
        [string]$Label
    )
    $beforeRatio = Get-L3CoverageRatio -Status $Before
    $afterRatio = Get-L3CoverageRatio -Status $After
    if ($null -ne $beforeRatio -and $null -ne $afterRatio -and $afterRatio -lt ($beforeRatio - 0.0001)) {
        Fail ($Label + " l3 coverage regressed: before=" + $beforeRatio + " after=" + $afterRatio)
    }
}

function Resolve-FirstExistingPath {
    param(
        [string[]]$Candidates,
        [string]$Label,
        [switch]$Optional
    )
    foreach ($candidate in $Candidates) {
        if (-not [string]::IsNullOrWhiteSpace($candidate) -and (Test-Path -LiteralPath $candidate)) {
            return (Resolve-Path -LiteralPath $candidate).Path
        }
    }
    if ($Optional) {
        return ""
    }
    Fail ("Could not find " + $Label + ". Tried: " + ($Candidates -join "; "))
}

function Resolve-KernelExe {
    param(
        [string]$RepoRoot,
        [string]$Explicit
    )
    if (-not [string]::IsNullOrWhiteSpace($Explicit)) {
        return (Resolve-Path -LiteralPath $Explicit).Path
    }
    return Resolve-FirstExistingPath -Label "kernel exe" -Candidates @(
        (Join-Path $RepoRoot "VitApp\build_release\VitApp_artefacts\Release\VitApp.exe"),
        (Join-Path $RepoRoot "VitApp\build\VitApp_artefacts\Release\VitApp.exe"),
        (Join-Path $RepoRoot "VitApp\build_release\VitApp.exe"),
        (Join-Path $RepoRoot "Export\_build\Vit_DAW_v0.9_release\kernel\VitApp.exe"),
        (Join-Path $RepoRoot "Export\staging\runtime\VitApp.exe")
    )
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
        Write-Ok ("reusing " + $Label + " port " + $Port + " pid=" + $listener.OwningProcess)
        return
    }
    Write-WarnLine ("stopping existing " + $Label + " pid=" + $listener.OwningProcess)
    Stop-Process -Id $listener.OwningProcess -Force
    Start-Sleep -Milliseconds 500
}

function Start-TestKernel {
    param(
        [string]$KernelPath,
        [int]$ReqPort,
        [int]$SubPort,
        [int]$TimeoutSeconds,
        [bool]$ReuseExisting
    )
    $desiredPath = [System.IO.Path]::GetFullPath($KernelPath)
    $listener = Get-TcpListener -Port $ReqPort
    if ($null -ne $listener) {
        if ($ReuseExisting) {
            Write-Ok ("reusing existing kernel command port: " + $ReqPort + " pid=" + $listener.OwningProcess)
            return @{
                started = $false
                pid = [int]$listener.OwningProcess
                path = $desiredPath
                reused = $true
            }
        }
        Write-WarnLine ("stopping existing kernel pid=" + $listener.OwningProcess)
        Stop-Process -Id $listener.OwningProcess -Force
        Start-Sleep -Milliseconds 500
    }

    Start-Process -FilePath $desiredPath -WorkingDirectory (Split-Path -Parent $desiredPath) -WindowStyle Hidden | Out-Null
    $ready = Wait-TcpListener -Port $ReqPort -TimeoutSeconds $TimeoutSeconds
    if ($null -eq $ready) {
        Fail ("Kernel command port did not become ready: " + $ReqPort + " using " + $desiredPath)
    }
    $sub = Wait-TcpListener -Port $SubPort -TimeoutSeconds 5
    if ($null -eq $sub) {
        Write-WarnLine ("kernel telemetry port not listening yet: " + $SubPort)
    }
    Write-Ok ("started kernel: " + $desiredPath + " pid=" + $ready.OwningProcess)
    return @{
        started = $true
        pid = [int]$ready.OwningProcess
        path = $desiredPath
        reused = $false
    }
}

function Resolve-TrackID {
    param([object]$Response)
    $result = Get-OptionalProperty -Object $Response -Name "result"
    $trackID = [string](Get-OptionalProperty -Object $result -Name "track_id")
    if ([string]::IsNullOrWhiteSpace($trackID)) {
        $trackID = [string](Get-OptionalProperty -Object $result -Name "id")
    }
    return $trackID
}

function Import-AudioFixture {
    param(
        [string]$TrackID,
        [string]$FilePath
    )
    $import = Invoke-AgentTool -Tool "clip.import_media_to_track" -ToolArgs @{
        track_id = $TrackID
        file_path = $FilePath
        start_time = 0
        media_type = "audio"
        mode = "non_destructive"
    } -Confirmed $true
    if ([string](Get-OptionalProperty -Object $import -Name "status") -eq "ok") {
        return $import
    }
    return Invoke-AgentTool -Tool "clip.import_audio" -ToolArgs @{
        track_id = $TrackID
        file_path = $FilePath
        offset_time = 0
    } -Confirmed $true
}

function Audio-MaterialName {
    param([string]$Path)
    $name = [System.IO.Path]::GetFileNameWithoutExtension($Path)
    $safe = $name -replace '[^\p{L}\p{Nd}_ -]+', ''
    $safe = $safe.Trim()
    if ([string]::IsNullOrWhiteSpace($safe)) {
        return "Audio Material"
    }
    if ($safe.Length -gt 40) {
        return $safe.Substring(0, 40)
    }
    return $safe
}

function Material-Label {
    param([string]$Path)
    $ext = [System.IO.Path]::GetExtension($Path).TrimStart(".").ToLowerInvariant()
    $name = [System.IO.Path]::GetFileNameWithoutExtension($Path)
    if ($name -eq "test_100hz_10s") {
        return "fixture_100hz_wav"
    }
    if ($ext -eq "mp3") {
        return "live_mp3_" + ($name -replace '[^A-Za-z0-9]+', '_').Trim("_")
    }
    return "live_" + $ext + "_" + ($name -replace '[^A-Za-z0-9]+', '_').Trim("_")
}

function Resolve-Materials {
    param(
        [string]$RepoRoot,
        [string[]]$ExplicitPaths
    )
    $rows = New-Object System.Collections.Generic.List[object]
    $skipped = New-Object System.Collections.Generic.List[object]
    $seen = @{}
    $addPath = {
        param([string]$Path, [string]$SourceLabel)
        if ([string]::IsNullOrWhiteSpace($Path)) {
            return
        }
        $resolved = (Resolve-Path -LiteralPath $Path).Path
        $key = $resolved.ToLowerInvariant()
        if ($seen.ContainsKey($key)) {
            return
        }
        $seen[$key] = $true
        $rows.Add([ordered]@{
            label = $SourceLabel
            path = $resolved
            file_name = [System.IO.Path]::GetFileName($resolved)
            extension = [System.IO.Path]::GetExtension($resolved).TrimStart(".").ToLowerInvariant()
        }) | Out-Null
    }

    if ($ExplicitPaths.Count -gt 0) {
        foreach ($path in $ExplicitPaths) {
            if (Test-Path -LiteralPath $path) {
                & $addPath $path (Material-Label -Path $path)
            }
            else {
                $skipped.Add([ordered]@{ path = $path; reason = "material_path_missing" }) | Out-Null
            }
        }
        return [ordered]@{ materials = @($rows.ToArray()); skipped = @($skipped.ToArray()) }
    }

    $defaults = @(
        @{ label = "fixture_100hz_wav"; candidates = @((Join-Path $RepoRoot "test_100hz_10s.wav")) },
        @{ label = "aigc_song_mp3_paper_crown"; candidates = @((Join-Path $RepoRoot "Paper Crown.mp3")) },
        @{ label = "repo_demo_ogg"; candidates = @((Join-Path $RepoRoot "tracktion_engine\examples\DemoRunner\resources\edm_song.ogg")) }
    )
    foreach ($item in $defaults) {
        $path = Resolve-FirstExistingPath -Label ([string]$item.label) -Candidates ([string[]]$item.candidates) -Optional
        if ([string]::IsNullOrWhiteSpace($path)) {
            $skipped.Add([ordered]@{ label = [string]$item.label; reason = "default_material_missing" }) | Out-Null
            continue
        }
        & $addPath $path ([string]$item.label)
    }

    $rootMP3s = @(Get-ChildItem -LiteralPath $RepoRoot -File -Filter *.mp3 -ErrorAction SilentlyContinue |
        Sort-Object Length -Descending)
    foreach ($mp3 in $rootMP3s) {
        & $addPath $mp3.FullName (Material-Label -Path $mp3.FullName)
    }

    return [ordered]@{ materials = @($rows.ToArray()); skipped = @($skipped.ToArray()) }
}

function Number-Present {
    param([object]$Value)
    if ($null -eq $Value) {
        return $false
    }
    $text = ([string]$Value).Trim()
    if ([string]::IsNullOrWhiteSpace($text) -or $text -eq "<nil>") {
        return $false
    }
    $dummy = 0.0
    return [double]::TryParse($text, [System.Globalization.NumberStyles]::Float, [System.Globalization.CultureInfo]::InvariantCulture, [ref]$dummy)
}

function Track-AcousticRow {
    param([object]$Track)
    $acoustic = Get-OptionalProperty -Object $Track -Name "acoustic"
    if ($null -eq $acoustic) {
        return $null
    }
    return $acoustic
}

function Compact-TrackAcoustic {
    param([object]$Track)
    $acoustic = Track-AcousticRow -Track $Track
    $row = [ordered]@{
        track_id = [string](Get-OptionalProperty -Object $Track -Name "track_id")
        track_name = [string](Get-OptionalProperty -Object $Track -Name "track_name")
        active_state = [string](Get-OptionalProperty -Object $Track -Name "active_state")
        acoustic_status = [string](Get-OptionalProperty -Object $acoustic -Name "status")
    }
    foreach ($key in @("primary_clip_id", "primary_clip_name", "source_path", "peak_dbfs", "rms_dbfs", "headroom_db", "crest_db", "time_energy_status", "reason")) {
        $value = Get-OptionalProperty -Object $acoustic -Name $key
        if ($null -ne $value -and -not [string]::IsNullOrWhiteSpace([string]$value)) {
            $row[$key] = $value
        }
    }
    return $row
}

function Assert-TrackAcousticsReady {
    param(
        [object[]]$Tracks,
        [string[]]$ExpectedTrackIDs
    )
    $expected = @{}
    foreach ($trackID in $ExpectedTrackIDs) {
        if (-not [string]::IsNullOrWhiteSpace($trackID)) {
            $expected[$trackID] = $false
        }
    }
    foreach ($track in @($Tracks)) {
        $trackID = [string](Get-OptionalProperty -Object $track -Name "track_id")
        if (-not $expected.ContainsKey($trackID)) {
            continue
        }
        $expected[$trackID] = $true
        $acoustic = Track-AcousticRow -Track $track
        if ([string](Get-OptionalProperty -Object $acoustic -Name "status") -ne "ready") {
            Fail ("track " + $trackID + " acoustic status is not ready: " + [string](Get-OptionalProperty -Object $acoustic -Name "status"))
        }
        foreach ($key in @("peak_dbfs", "rms_dbfs", "headroom_db", "crest_db")) {
            if (-not (Number-Present -Value (Get-OptionalProperty -Object $acoustic -Name $key))) {
                Fail ("track " + $trackID + " missing acoustic metric " + $key)
            }
        }
        foreach ($key in @("primary_clip_id", "primary_clip_name", "source_path")) {
            $value = [string](Get-OptionalProperty -Object $acoustic -Name $key)
            if ([string]::IsNullOrWhiteSpace($value)) {
                Fail ("track " + $trackID + " missing acoustic source field " + $key)
            }
        }
    }
    foreach ($trackID in $ExpectedTrackIDs) {
        if (-not $expected.ContainsKey($trackID) -or -not [bool]$expected[$trackID]) {
            Fail ("imported track " + $trackID + " was not present in observed project tracks")
        }
    }
}

function Get-TrackAcousticsReadinessIssues {
    param(
        [object[]]$Tracks,
        [string[]]$ExpectedTrackIDs
    )
    $issues = @()
    $expected = @{}
    foreach ($trackID in $ExpectedTrackIDs) {
        if (-not [string]::IsNullOrWhiteSpace($trackID)) {
            $expected[$trackID] = $false
        }
    }
    foreach ($track in @($Tracks)) {
        $trackID = [string](Get-OptionalProperty -Object $track -Name "track_id")
        if (-not $expected.ContainsKey($trackID)) {
            continue
        }
        $expected[$trackID] = $true
        $acoustic = Track-AcousticRow -Track $track
        $status = [string](Get-OptionalProperty -Object $acoustic -Name "status")
        if ($status -ne "ready") {
            $issues += ("track " + $trackID + " acoustic status=" + $status)
            continue
        }
        foreach ($key in @("peak_dbfs", "rms_dbfs", "headroom_db", "crest_db")) {
            if (-not (Number-Present -Value (Get-OptionalProperty -Object $acoustic -Name $key))) {
                $issues += ("track " + $trackID + " missing acoustic metric " + $key)
            }
        }
        foreach ($key in @("primary_clip_id", "primary_clip_name", "source_path")) {
            $value = [string](Get-OptionalProperty -Object $acoustic -Name $key)
            if ([string]::IsNullOrWhiteSpace($value)) {
                $issues += ("track " + $trackID + " missing acoustic source field " + $key)
            }
        }
    }
    foreach ($trackID in $ExpectedTrackIDs) {
        if (-not $expected.ContainsKey($trackID) -or -not [bool]$expected[$trackID]) {
            $issues += ("imported track " + $trackID + " was not present in observed project tracks")
        }
    }
    return $issues
}

function Build-AcousticReadiness {
    param([object]$Observation)
    $global = Get-OptionalProperty -Object $Observation -Name "global_summary"
    $snapshot = Get-OptionalProperty -Object $global -Name "feature_snapshot"
    $mixPackage = Get-OptionalProperty -Object $Observation -Name "mix_package"
    $deepPackage = Get-OptionalProperty -Object $Observation -Name "deep_package"
    $projectPackage = Get-OptionalProperty -Object $Observation -Name "project_package"
    $sourceCaps = Get-OptionalProperty -Object $Observation -Name "source_capabilities"
    $currentMetrics = Get-OptionalProperty -Object $mixPackage -Name "current_metrics"
    $deepCaps = Get-OptionalProperty -Object $deepPackage -Name "source_capabilities"
    $rows = [ordered]@{
        source_capabilities = $sourceCaps
        mix_source_capabilities = Get-OptionalProperty -Object $mixPackage -Name "source_capabilities"
        deep_source_capabilities = $deepCaps
        project_limitations = @(Get-OptionalProperty -Object $projectPackage -Name "limitations")
        feature_snapshot = $snapshot
        current_metrics = [ordered]@{
            band_energy = Get-OptionalProperty -Object $currentMetrics -Name "band_energy"
            stereo_relation = Get-OptionalProperty -Object $currentMetrics -Name "stereo_relation"
        }
    }
	return $rows
}

function Number-Value {
	param([object]$Value)
	if (-not (Number-Present -Value $Value)) {
		return $null
	}
	return [double]::Parse(([string]$Value).Trim(), [System.Globalization.CultureInfo]::InvariantCulture)
}

function Assert-RequestedFeature {
	param(
		[object]$LatestRequest,
		[string]$FeatureType
	)
	$found = $false
	foreach ($row in @((Get-OptionalProperty -Object $LatestRequest -Name "requested_features"))) {
		if ([string](Get-OptionalProperty -Object $row -Name "feature_type") -eq $FeatureType) {
			$found = $true
			break
		}
	}
	if (-not $found) {
		Fail ("latest_request.requested_features missing " + $FeatureType)
	}
}

function Test-KernelPreparedLatestRequest {
	param([object]$LatestRequest)
	$status = [string](Get-OptionalProperty -Object $LatestRequest -Name "status")
	$lifecycle = [string](Get-OptionalProperty -Object $LatestRequest -Name "lifecycle")
	$sourceKind = [string](Get-OptionalProperty -Object $LatestRequest -Name "source_kind")
	$requestID = [string](Get-OptionalProperty -Object $LatestRequest -Name "request_id")
	return ($status -eq "materialized" -or
		$lifecycle -eq "kernel_prepared_materializer" -or
		$sourceKind -eq "kernel_prepared_telemetry" -or
		$requestID.StartsWith("kernel_prepared_"))
}

function Assert-KernelPreparedTrackWaveformRows {
	param([object]$Snapshot)
	$rows = @((Get-OptionalProperty -Object $Snapshot -Name "track_waveform_envelopes"))
	if ($rows.Count -lt 1) {
		Fail "kernel-prepared feature_snapshot missing track_waveform_envelopes"
	}
	foreach ($row in $rows) {
		$status = [string](Get-OptionalProperty -Object $row -Name "status")
		if ($status -ne "ready") {
			Fail ("kernel-prepared track waveform row is not ready: " + ($row | ConvertTo-Json -Depth 8 -Compress))
		}
		foreach ($key in @("track_id", "clip_id", "request_id", "source_revision")) {
			if ([string]::IsNullOrWhiteSpace([string](Get-OptionalProperty -Object $row -Name $key))) {
				Fail ("kernel-prepared track waveform row missing " + $key + ": " + ($row | ConvertTo-Json -Depth 8 -Compress))
			}
		}
	}
}

function Assert-BridgeSnapshotRow {
	param(
		[object]$Snapshot,
		[string]$Name,
		[string]$ExpectedRequestID
	)
	$row = Get-OptionalProperty -Object $Snapshot -Name $Name
	$status = [string](Get-OptionalProperty -Object $row -Name "status")
	if ([string]::IsNullOrWhiteSpace($status)) {
		Fail ("acoustic feature " + $Name + " missing status")
	}
	$rowRequestID = [string](Get-OptionalProperty -Object $row -Name "request_id")
	if (-not [string]::IsNullOrWhiteSpace($ExpectedRequestID) -and -not [string]::IsNullOrWhiteSpace($rowRequestID) -and $rowRequestID -ne $ExpectedRequestID) {
		$rowSourceRevision = [string](Get-OptionalProperty -Object $row -Name "source_revision")
		$rowSourceHash = [string](Get-OptionalProperty -Object $row -Name "source_hash")
		if ([string]::IsNullOrWhiteSpace($rowSourceRevision) -and [string]::IsNullOrWhiteSpace($rowSourceHash)) {
			Fail ("acoustic feature " + $Name + " request_id mismatch without source revision/hash: got=" + $rowRequestID + " expected=" + $ExpectedRequestID)
		}
	}
	if ($status -in @("ready", "partial")) {
		if ([string]::IsNullOrWhiteSpace($rowRequestID)) {
			Fail ("ready acoustic feature " + $Name + " missing request_id")
		}
		$source = [string](Get-OptionalProperty -Object $row -Name "source")
		if ([string]::IsNullOrWhiteSpace($source)) {
			Fail ("ready acoustic feature " + $Name + " missing source")
		}
		if ($Name -eq "spectrogram_tiles") {
			if ([string](Get-OptionalProperty -Object $row -Name "feature_type") -ne "spectral_field") {
				Fail ("spectrogram_tiles feature_type is not spectral_field: " + ($row | ConvertTo-Json -Depth 8 -Compress))
			}
			if ($source -notin @("kernel_tile_ready_direct_collector", "kernel_tile_ready_godot_bridge", "kernel_prepared_telemetry")) {
				Fail ("spectrogram_tiles source is not an accepted bridge source: " + $source)
			}
			$seen = Number-Value -Value (Get-OptionalProperty -Object $row -Name "tile_count_seen")
			$expected = Number-Value -Value (Get-OptionalProperty -Object $row -Name "tile_count_expected")
			if ($null -eq $seen -or $seen -lt 1) {
				Fail ("spectrogram_tiles ready/partial row missing tile_count_seen: " + ($row | ConvertTo-Json -Depth 8 -Compress))
			}
			if ($null -eq $expected) {
				Fail ("spectrogram_tiles ready/partial row missing tile_count_expected: " + ($row | ConvertTo-Json -Depth 8 -Compress))
			}
		}
		return
	}
	if ($status -in @("requested", "building")) {
		return
	}
	if ($status -in @("missing", "stale", "blocked", "unavailable", "invalid")) {
		$reason = [string](Get-OptionalProperty -Object $row -Name "reason")
		if ([string]::IsNullOrWhiteSpace($reason)) {
			Fail ("acoustic feature " + $Name + " status " + $status + " missing explicit reason")
		}
		$allowed = @("stale_feature_snapshot_for_current_request", "incomplete_source_identity")
		if ($Name -eq "spectrogram_tiles") {
			$allowed += @("no_spectral_tile_ready_received")
		}
		elseif ($Name -eq "band_energy_summary") {
			$allowed += @("spectral_field_missing: no_spectral_tile_ready_received", "no_live_spectrum_payload_for_current_request", "no_live_spectrum_payload", "empty_band_energy_summary", "no_target_match_in_live_levels")
		}
		elseif ($Name -eq "stereo_relation_summary") {
			$allowed += @("spectral_field_missing: no_spectral_tile_ready_received", "no_live_stereo_payload_for_current_request", "no_live_stereo_payload", "no_live_spectrum_payload", "no_target_match_in_live_levels")
		}
		if ($allowed -notcontains $reason -and -not (Test-DadQualityGateReason -Reason $reason)) {
			Fail ("acoustic feature " + $Name + " has unexpected missing reason " + $reason)
		}
		return
	}
	Fail ("acoustic feature " + $Name + " has unexpected status " + $status)
}

function Test-DadQualityGateReason {
	param([string]$Reason)
	$reasonText = $Reason.Trim().ToLowerInvariant()
	if ([string]::IsNullOrWhiteSpace($reasonText)) {
		return $false
	}
	foreach ($token in @(
		"reader_all_zero",
		"fft_input_all_zero",
		"fft_output_all_zero",
		"tile_all_zero_before_write",
		"shared_memory_all_zero_after_write",
		"nan_or_inf_detected",
		"empty_coverage",
		"no_active_frames_above_gate"
	)) {
		if ($reasonText.Contains($token)) {
			return $true
		}
	}
	return $false
}

function Assert-AcousticReadinessReasons {
	param([object]$Readiness)
	$snapshot = Get-OptionalProperty -Object $Readiness -Name "feature_snapshot"
	$latest = Get-OptionalProperty -Object $snapshot -Name "latest_request"
	$requestID = [string](Get-OptionalProperty -Object $latest -Name "request_id")
	if ([string]::IsNullOrWhiteSpace($requestID)) {
		Fail "feature_snapshot.latest_request.request_id missing"
	}
	if (Test-KernelPreparedLatestRequest -LatestRequest $latest) {
		Assert-KernelPreparedTrackWaveformRows -Snapshot $snapshot
	}
	else {
		Assert-RequestedFeature -LatestRequest $latest -FeatureType "waveform_envelope"
		Assert-RequestedFeature -LatestRequest $latest -FeatureType "spectral_field"
	}
	foreach ($name in @("spectrogram_tiles", "band_energy_summary", "stereo_relation_summary")) {
		Assert-BridgeSnapshotRow -Snapshot $snapshot -Name $name -ExpectedRequestID $requestID
	}
	$currentMetrics = Get-OptionalProperty -Object $Readiness -Name "current_metrics"
	foreach ($name in @("band_energy", "stereo_relation")) {
		$row = Get-OptionalProperty -Object $currentMetrics -Name $name
		$status = [string](Get-OptionalProperty -Object $row -Name "status")
		if ($status -eq "missing" -and [string]::IsNullOrWhiteSpace([string](Get-OptionalProperty -Object $row -Name "reason"))) {
			Fail ("current_metrics." + $name + " missing explicit reason")
		}
	}
	$deepCaps = Get-OptionalProperty -Object $Readiness -Name "deep_source_capabilities"
    if ([string](Get-OptionalProperty -Object $deepCaps -Name "masking_analysis") -eq "" -or [string](Get-OptionalProperty -Object $deepCaps -Name "reference_match") -eq "" -or [string](Get-OptionalProperty -Object $deepCaps -Name "lufs_analysis") -eq "") {
        Fail ("deep acoustic deferred capabilities missing from readiness: " + ($deepCaps | ConvertTo-Json -Depth 8 -Compress))
    }
    $limits = @((Get-OptionalProperty -Object $Readiness -Name "project_limitations"))
    foreach ($required in @("lufs_analysis_deferred_phase_5", "masking_analysis_not_ready_on_current_project_cut", "reference_match_deferred_phase_5", "post_fx_probe_unavailable_phase_4_1")) {
        if ($limits -notcontains $required) {
            Fail ("project limitations missing " + $required + ". limitations=" + ($limits -join ","))
        }
    }
}

$RepoRoot = (Resolve-Path -LiteralPath $RepoRoot).Path
$ScriptsDir = Join-Path $RepoRoot "scripts"
$WorkspaceDir = Join-Path $RepoRoot "VitApp\Workspace"
$LogsDir = Join-Path $WorkspaceDir "Logs"
$SmokeRoot = Join-Path $WorkspaceDir "Artifacts\smoke"
$Stamp = Get-Date -Format "yyyyMMdd_HHmmss"
$ArtifactDir = Join-Path $SmokeRoot ("live_material_observation_" + $Stamp)
$DevSmoke = Join-Path $ScriptsDir "dev_agent_smoke.ps1"
$KernelExe = Resolve-KernelExe -RepoRoot $RepoRoot -Explicit $KernelExe
$AgentLog = Join-Path $LogsDir "agent_last.log"

New-Item -ItemType Directory -Path $ArtifactDir -Force | Out-Null
New-Item -ItemType Directory -Path $LogsDir -Force | Out-Null

$materialResult = Resolve-Materials -RepoRoot $RepoRoot -ExplicitPaths $MaterialPaths
$materials = @($materialResult["materials"])
$skippedMaterials = @($materialResult["skipped"])
if ($materials.Count -lt 2) {
    Fail ("Need at least two available materials for live observation smoke; found " + $materials.Count)
}

$startedAgentPID = $null
$startedKernel = $null
$summary = [ordered]@{
    schema_version = "vit_live_material_observation_smoke.v1"
    created_at = (Get-Date).ToString("o")
    repo_root = $RepoRoot
    artifact_dir = $ArtifactDir
    agent_http = $AgentHttp
    kernel_exe = $KernelExe
    materials = $materials
    skipped_materials = $skippedMaterials
    imports = @()
    observation_id = $null
    active_acoustic_track_count = 0
    track_acoustics = @()
    rankings = @{}
    acoustic_package_status_path = $null
    immediate_acoustic_package_status = $null
    after_wait_acoustic_package_status = $null
    status = "running"
}

try {
    Write-Step "Live material observation smoke"
    Write-Host ("repo: " + $RepoRoot)
    Write-Host ("artifact_dir: " + $ArtifactDir)
    Write-Host ("kernel: " + $KernelExe)
    foreach ($material in $materials) {
        Write-Host ("material: " + [string]$material.label + " -> " + [string]$material.path)
    }

    Write-Step "Prepare agent"
    if (-not (Test-Path -LiteralPath $DevSmoke)) {
        Fail ("Missing dev smoke script: " + $DevSmoke)
    }
    $beforeAgent = Get-TcpListener -Port ([int](($AgentHttpAddr -split ":")[-1]))
    $smokeArgs = @{
        RepoRoot = $RepoRoot
        AgentHttp = $AgentHttp
        AgentHttpAddr = $AgentHttpAddr
        ZmqReqPort = $ZmqReqPort
        ZmqSubPort = $ZmqSubPort
        NoChatSmoke = $true
        WaitSeconds = $WaitSeconds
    }
    if ($SkipBuild) {
        $smokeArgs["SkipBuild"] = $true
    }
    if (-not $ReuseAgent) {
        $smokeArgs["RestartAgent"] = $true
    }
    & $DevSmoke @smokeArgs
    if (-not $?) {
        Fail "dev_agent_smoke failed"
    }
    $afterAgent = Get-TcpListener -Port ([int](($AgentHttpAddr -split ":")[-1]))
    if ($null -ne $afterAgent -and ($null -eq $beforeAgent -or $beforeAgent.OwningProcess -ne $afterAgent.OwningProcess) -and -not $ReuseAgent) {
        $startedAgentPID = [int]$afterAgent.OwningProcess
    }
    if (-not (Wait-HttpReady -BaseUrl $AgentHttp -TimeoutSeconds $WaitSeconds)) {
        Fail ("Agent HTTP did not become ready at " + $AgentHttp)
    }

    Write-Step "Prepare kernel"
    Stop-PortOwnerIfNeeded -Port ([int]$ZmqReqPort) -Label "kernel command" -Reuse ([bool]$ReuseKernel)
    Stop-PortOwnerIfNeeded -Port ([int]$ZmqSubPort) -Label "kernel event" -Reuse ([bool]$ReuseKernel)
    $startedKernel = Start-TestKernel -KernelPath $KernelExe -ReqPort ([int]$ZmqReqPort) -SubPort ([int]$ZmqSubPort) -TimeoutSeconds $WaitSeconds -ReuseExisting ([bool]$ReuseKernel)

    Write-Step "Create material project"
    $newProject = Invoke-AgentTool -Tool "project.new" -ToolArgs @{} -Confirmed $true
    if ([string](Get-OptionalProperty -Object $newProject -Name "status") -eq "ok") {
        Write-Ok "project.new reset completed"
        Start-Sleep -Milliseconds 500
    }
    else {
        Write-WarnLine ("project.new unavailable: " + [string](Get-OptionalProperty -Object $newProject -Name "error"))
    }
    Assert-StatusOk -Response (Invoke-AgentTool -Tool "project.clear" -ToolArgs @{} -Confirmed $true) -Label "project.clear"

    $trackRows = @()
    foreach ($material in $materials) {
        $trackName = Audio-MaterialName -Path ([string]$material.path)
        $track = Invoke-AgentTool -Tool "track.add_audio" -ToolArgs @{ name = $trackName } -Confirmed $true
        Assert-StatusOk -Response $track -Label ("track.add_audio " + $trackName)
        $trackID = Resolve-TrackID -Response $track
        if ([string]::IsNullOrWhiteSpace($trackID)) {
            Fail ("Could not resolve track ID for " + $trackName)
        }
        $import = Import-AudioFixture -TrackID $trackID -FilePath ([string]$material.path)
        Assert-StatusOk -Response $import -Label ("import " + [string]$material.file_name)
        $importResult = Get-OptionalProperty -Object $import -Name "result"
        $clipID = [string](Get-OptionalProperty -Object $importResult -Name "clip_id")
        $row = [ordered]@{
            label = [string]$material.label
            track_id = $trackID
            track_name = $trackName
            file_path = [string]$material.path
            file_name = [string]$material.file_name
            extension = [string]$material.extension
            clip_id = $clipID
            import_status = [string](Get-OptionalProperty -Object $import -Name "status")
            import_tool = [string](Get-OptionalProperty -Object $import -Name "tool")
            import_command_name = [string](Get-OptionalProperty -Object $import -Name "command_name")
        }
        $trackRows += $row
        $summary["imports"] = @($trackRows)
        Write-Ok ("imported " + [string]$material.file_name + " on track " + $trackID + " clip " + $clipID)
    }
    ConvertTo-JsonFile -Value $trackRows -Path (Join-Path $ArtifactDir "imports.json")

    Write-Step "Observe acoustic package lifecycle immediately after import"
    $readOnlyBandStereoPrompt = -join ([int[]]@(0x89c2, 0x5bdf, 0x4e00, 0x4e0b, 0x5f53, 0x524d, 0x5de5, 0x7a0b, 0x7684, 0x9891, 0x6bb5, 0x548c, 0x58f0, 0x50cf, 0x72b6, 0x6001, 0xff0c, 0x4e0d, 0x8981, 0x6267, 0x884c, 0x4efb, 0x4f55, 0x4fee, 0x6539, 0x3002) | ForEach-Object { [char]$_ })
    $immediateObserve = Invoke-AgentTool -Tool "mix.observe" -ToolArgs @{
        mix_session_id = "acoustic_package_lifecycle_immediate_" + $Stamp
        scope = "full_project"
        goal_text = $readOnlyBandStereoPrompt
        mixboard_request_id = "acoustic_package_lifecycle_immediate_" + $Stamp
    } -Confirmed $true -TimeoutSec $ObservationTimeoutSec
    ConvertTo-JsonFile -Value $immediateObserve -Path (Join-Path $ArtifactDir "acoustic_package_lifecycle_immediate_observe.json")
    Assert-StatusOk -Response $immediateObserve -Label "mix.observe immediate acoustic package lifecycle"
    Assert-NoPendingOrConfirmation -Response $immediateObserve -Label "mix.observe immediate acoustic package lifecycle"
    $immediateResult = Get-OptionalProperty -Object $immediateObserve -Name "result"
    $immediateStatus = Get-AcousticPackageStatusFromResponse -Response $immediateObserve
    Assert-ImmediateAcousticPackageLifecycle -Status $immediateStatus -Label "mix.observe immediate acoustic package lifecycle"
    $statusPath = Assert-AcousticPackageArtifact -Response $immediateObserve -Label "mix.observe immediate acoustic package lifecycle"
    $summary["acoustic_package_status_path"] = $statusPath
    $summary["immediate_observation_id"] = [string](Get-OptionalProperty -Object $immediateResult -Name "observation_id")
    $summary["immediate_acoustic_package_status"] = Compact-AcousticPackageStatus -Status $immediateStatus
    ConvertTo-JsonFile -Value $immediateStatus -Path (Join-Path $ArtifactDir "acoustic_package_status_immediate.json")
    Write-Ok ("immediate acoustic_package_status captured at " + $statusPath)

    Write-Step "Observe full project acoustics"
    $expectedTrackIDs = [string[]]@($trackRows | ForEach-Object { [string]$_["track_id"] })
    $observe = $null
    $result = $null
    $obs = $null
    $project = $null
    $tracks = @()
    $trackIssues = @("full project observation not attempted")
    $attempt = 0
    $deadline = (Get-Date).AddSeconds([Math]::Max(10, [Math]::Min($ObservationTimeoutSec, 120)))
    do {
        $attempt++
        if ($attempt -eq 1) {
            Start-Sleep -Milliseconds 750
        }
        else {
            Start-Sleep -Seconds 2
        }
        $observe = Invoke-AgentTool -Tool "mix.observe" -ToolArgs @{
            mix_session_id = "live_material_observation_" + $Stamp
            scope = "full_project"
            goal_text = "compare the overall mix, multitrack band distribution, and stereo layout. do not modify."
            mixboard_request_id = "live_material_observation_" + $Stamp
        } -Confirmed $true -TimeoutSec $ObservationTimeoutSec
        ConvertTo-JsonFile -Value $observe -Path (Join-Path $ArtifactDir ("mix_observe_full_project_attempt_" + $attempt + ".json"))
        Assert-StatusOk -Response $observe -Label ("mix.observe full_project attempt " + $attempt)
        Assert-NoPendingOrConfirmation -Response $observe -Label ("mix.observe full_project attempt " + $attempt)
        $result = Get-OptionalProperty -Object $observe -Name "result"
        $obs = Get-OptionalProperty -Object $result -Name "observation"
        $project = Get-OptionalProperty -Object $obs -Name "project_package"
        $tracks = @(Get-OptionalProperty -Object $project -Name "tracks")
        $trackIssues = @(Get-TrackAcousticsReadinessIssues -Tracks $tracks -ExpectedTrackIDs $expectedTrackIDs)
        if ($trackIssues.Count -eq 0) {
            break
        }
        Write-WarnLine ("full_project acoustic readiness pending attempt " + $attempt + ": " + ($trackIssues -join "; "))
    } while ((Get-Date) -lt $deadline)
    ConvertTo-JsonFile -Value $observe -Path (Join-Path $ArtifactDir "mix_observe_full_project.json")
    $summary["observation_id"] = [string](Get-OptionalProperty -Object $result -Name "observation_id")
    $summary["active_acoustic_track_count"] = [int](Get-OptionalProperty -Object $project -Name "active_acoustic_track_count")
    if ($summary["active_acoustic_track_count"] -lt $materials.Count) {
        Fail ("active_acoustic_track_count " + $summary["active_acoustic_track_count"] + " < imported material count " + $materials.Count + ". issues=" + ($trackIssues -join "; "))
    }
    if ($tracks.Count -lt $materials.Count) {
        Fail ("observed track count " + $tracks.Count + " < imported material count " + $materials.Count)
    }
    if ($trackIssues.Count -gt 0) {
        Fail ("full_project acoustic readiness did not complete after " + $attempt + " attempts: " + ($trackIssues -join "; "))
    }
    Assert-MOMV13MultitrackProjection -Observation $obs -Label "mix.observe full_project"
    Assert-TrackAcousticsReady -Tracks $tracks -ExpectedTrackIDs $expectedTrackIDs
    $readiness = Build-AcousticReadiness -Observation $obs
    Assert-AcousticReadinessReasons -Readiness $readiness
    $summary["acoustic_readiness"] = $readiness
    ConvertTo-JsonFile -Value $readiness -Path (Join-Path $ArtifactDir "acoustic_readiness.json")
    $afterWaitStatus = Get-AcousticPackageStatusFromResponse -Response $observe
    Assert-AcousticPackageStatusV0 -Status $afterWaitStatus -Label "mix.observe after wait acoustic package lifecycle"
    Assert-AcousticCoverageNotRegressed -Before $immediateStatus -After $afterWaitStatus -Label "mix.observe after wait acoustic package lifecycle"
    $summary["after_wait_acoustic_package_status"] = Compact-AcousticPackageStatus -Status $afterWaitStatus
    ConvertTo-JsonFile -Value $afterWaitStatus -Path (Join-Path $ArtifactDir "acoustic_package_status_after_wait.json")
    $featureSnapshot = Get-OptionalProperty -Object $readiness -Name "feature_snapshot"
    if ($null -ne $featureSnapshot) {
        ConvertTo-JsonFile -Value $featureSnapshot -Path (Join-Path $ArtifactDir "feature_snapshot.json")
    }

    $compactTracks = @()
    foreach ($track in $tracks) {
        $compactTracks += Compact-TrackAcoustic -Track $track
    }
    $summary["track_acoustics"] = $compactTracks
    $summary["rankings"] = [ordered]@{
        loudness = @(Get-OptionalProperty -Object $project -Name "loudness_ranking")
        peak = @(Get-OptionalProperty -Object $project -Name "peak_ranking")
        headroom_risk = @(Get-OptionalProperty -Object $project -Name "headroom_risk")
    }
    foreach ($key in @("loudness", "peak", "headroom_risk")) {
        if (@($summary["rankings"][$key]).Count -lt $materials.Count) {
            Fail ("ranking " + $key + " has fewer rows than imported materials")
        }
    }
    ConvertTo-JsonFile -Value $compactTracks -Path (Join-Path $ArtifactDir "track_acoustics.json")
    ConvertTo-JsonFile -Value $summary["rankings"] -Path (Join-Path $ArtifactDir "rankings.json")

    $summary["status"] = "passed"
    Write-Ok "live material observation smoke passed"
}
catch {
    $summary["status"] = "failed"
    $summary["error"] = $_.Exception.Message
    Write-WarnLine ("failed: " + $_.Exception.Message)
    throw
}
finally {
    if (Test-Path -LiteralPath $AgentLog) {
        Get-Content -LiteralPath $AgentLog -Tail 160 -ErrorAction SilentlyContinue |
            Set-Content -LiteralPath (Join-Path $ArtifactDir "agent_log_tail.txt") -Encoding UTF8
    }
    ConvertTo-JsonFile -Value $summary -Path (Join-Path $ArtifactDir "summary.json")
    if (-not $KeepProcesses) {
        if ($null -ne $startedKernel -and [bool]$startedKernel.started) {
            Stop-Process -Id ([int]$startedKernel.pid) -Force -ErrorAction SilentlyContinue
        }
        if ($null -ne $startedAgentPID) {
            Stop-Process -Id ([int]$startedAgentPID) -Force -ErrorAction SilentlyContinue
        }
    }
}

Write-Step "Summary"
Write-Host ("artifact_dir: " + $ArtifactDir)
Write-Host ("observation_id: " + [string]($summary["observation_id"]))
Write-Host ("active_acoustic_track_count: " + [string]($summary["active_acoustic_track_count"]))
Write-Host ("materials: " + ($materials.Count))
