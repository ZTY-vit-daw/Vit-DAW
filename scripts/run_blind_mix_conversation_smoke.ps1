[CmdletBinding()]
param(
    [string]$RepoRoot = "D:\Vit_DAW",
    [string]$GodotProjectRoot = "D:\Godot\project\vit-daw-frontend",
    [string]$AgentHttp = "http://127.0.0.1:7878",
    [string]$AgentHttpAddr = "127.0.0.1:7878",
    [string]$ZmqReqPort = "5555",
    [string]$ZmqSubPort = "5556",
    [string]$Track1Path = "",
    [string]$Track2Path = "",
    [switch]$ReuseGodot,
    [switch]$ReuseAgent,
    [switch]$ReuseKernel,
    [switch]$SkipBuild,
    [switch]$StartKernel,
    [int]$TimeoutSeconds = 120,
    [int]$ChatTimeoutSec = 240
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

function Join-UnicodeChars {
    param([int[]]$CodePoints)
    $builder = New-Object System.Text.StringBuilder
    foreach ($codePoint in $CodePoints) {
        [void]$builder.Append([char]$codePoint)
    }
    return $builder.ToString()
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

function Resolve-KernelExe {
    param([string]$RepoRoot)
    return Resolve-FirstExistingPath -Label "kernel exe" -Candidates @(
        (Join-Path $RepoRoot "VitApp\build_release\VitApp_artefacts\Release\VitApp.exe"),
        (Join-Path $RepoRoot "VitApp\build\VitApp_artefacts\Release\VitApp.exe"),
        (Join-Path $RepoRoot "VitApp\build_release\VitApp.exe"),
        (Join-Path $RepoRoot "Export\staging\runtime\VitApp.exe")
    )
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
        [bool]$Confirmed = $true
    )
    if ($null -eq $ToolArgs) {
        $ToolArgs = @{}
    }
    return Invoke-Json -Method POST -Uri ($AgentHttp.TrimEnd("/") + "/agent/invoke") -Body @{
        tool = $Tool
        args = $ToolArgs
        confirmed = $Confirmed
        source = "blind_mix_conversation_smoke"
    } -TimeoutSec 120
}

function Invoke-AgentChat {
    param(
        [string]$ConversationID,
        [string]$Message
    )
    return Invoke-Json -Method POST -Uri ($AgentHttp.TrimEnd("/") + "/agent/chat") -Body @{
        conversation_id = $ConversationID
        message = $Message
        context = @{
            agent_mode = "chat"
            interaction_path = "blind_mix_conversation_smoke"
            blind_mix_conversation_smoke = $true
            product_lifecycle = "godot_project"
            godot_project_root = $GodotProjectRoot
        }
    } -TimeoutSec $ChatTimeoutSec
}

function Invoke-AgentInteractionRespond {
    param(
        [string]$InteractionID,
        [string]$Decision = "approve",
        [string]$ActionID = "approve",
        [object]$Payload = $null
    )
    if ($null -eq $Payload) {
        $Payload = @{}
    }
    return Invoke-Json -Method POST -Uri ($AgentHttp.TrimEnd("/") + "/agent/interaction/respond") -Body @{
        interaction_id = $InteractionID
        decision = $Decision
        action_id = $ActionID
        payload = $Payload
    } -TimeoutSec $ChatTimeoutSec
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
        [string]$FilePath,
        [string]$Label
    )
    $preferred = Invoke-AgentTool -Tool "clip.import_media_to_track" -ToolArgs @{
        track_id = $TrackID
        file_path = $FilePath
        start_time = 0
        media_type = "audio"
        mode = "non_destructive"
    } -Confirmed $true
    if ([string](Get-OptionalProperty -Object $preferred -Name "status") -eq "ok") {
        return $preferred
    }
    Write-WarnLine ($Label + ": clip.import_media_to_track unavailable; falling back to clip.import_audio")
    return Invoke-AgentTool -Tool "clip.import_audio" -ToolArgs @{
        track_id = $TrackID
        file_path = $FilePath
        offset_time = 0
    } -Confirmed $true
}

function Reset-FixtureProject {
    $newProject = Invoke-AgentTool -Tool "project.new" -ToolArgs @{} -Confirmed $true
    if ([string](Get-OptionalProperty -Object $newProject -Name "status") -ne "ok") {
        Write-WarnLine ("project.new unavailable: " + [string](Get-OptionalProperty -Object $newProject -Name "error"))
    }
    $clear = Invoke-AgentTool -Tool "project.clear" -ToolArgs @{} -Confirmed $true
    Assert-StatusOk -Response $clear -Label "project.clear"
}

function Add-ToolName {
    param(
        [string[]]$Names,
        [string]$Name
    )
    if (-not [string]::IsNullOrWhiteSpace($Name)) {
        return @($Names + $Name)
    }
    return $Names
}

function Tool-Names {
    param([object]$Rows)
    $out = @()
    if ($null -eq $Rows) {
        return $out
    }
    foreach ($row in @($Rows)) {
        if ($row -is [string]) {
            $out = Add-ToolName -Names $out -Name $row
            continue
        }
        foreach ($key in @("tool", "command_name", "command")) {
            $out = Add-ToolName -Names $out -Name ([string](Get-OptionalProperty -Object $row -Name $key))
        }
    }
    return $out
}

function Count-ExecutedToolGroup {
    param(
        [object]$Rows,
        [string[]]$Aliases
    )
    $count = 0
    foreach ($row in @($Rows)) {
        $matched = $false
        foreach ($key in @("tool", "command_name", "command")) {
            $name = [string](Get-OptionalProperty -Object $row -Name $key)
            if ($Aliases -contains $name) {
                $matched = $true
            }
        }
        if ($matched) {
            $count++
        }
    }
    return $count
}

function Assert-ToolPresent {
    param(
        [string[]]$Tools,
        [string[]]$Aliases,
        [string]$Label
    )
    foreach ($alias in $Aliases) {
        if ($Tools -contains $alias) {
            return
        }
    }
    Fail ("Expected " + $Label + " in route. route=" + ($Tools -join " -> "))
}

function Assert-ToolAbsent {
    param(
        [string[]]$Tools,
        [string[]]$Aliases,
        [string]$Label
    )
    foreach ($alias in $Aliases) {
        if ($Tools -contains $alias) {
            Fail ("Unexpected " + $Label + " in route. route=" + ($Tools -join " -> "))
        }
    }
}

function Tool-Counts {
    param([object]$Rows)
    return [ordered]@{
        observe = Count-ExecutedToolGroup -Rows $Rows -Aliases @("mix.observe", "mix_observe", "mix.request_observation", "mix_request_observation")
        derive = Count-ExecutedToolGroup -Rows $Rows -Aliases @("mix.derive", "mix_derive")
        propose = Count-ExecutedToolGroup -Rows $Rows -Aliases @("mix.propose_tick", "mix_propose_tick")
        apply = Count-ExecutedToolGroup -Rows $Rows -Aliases @("mix.apply_tick", "mix_apply_tick")
        daw_invoke = Count-ExecutedToolGroup -Rows $Rows -Aliases @("daw.invoke", "daw_invoke")
        track_volume = Count-ExecutedToolGroup -Rows $Rows -Aliases @("track.volume", "track_volume")
        track_pan = Count-ExecutedToolGroup -Rows $Rows -Aliases @("track.pan", "track_pan")
        rack_add_node = Count-ExecutedToolGroup -Rows $Rows -Aliases @("rack.add_node", "rack_add_node", "plugin.load_to_rack", "plugin.instantiate", "instantiate_plugin")
        plugin_get_parameters = Count-ExecutedToolGroup -Rows $Rows -Aliases @("plugin.get_parameters", "plugin_get_parameters", "get_plugin_parameters")
        plugin_grabber_load_and_get_params = Count-ExecutedToolGroup -Rows $Rows -Aliases @("plugin_grabber.load_and_get_params", "plugin_grabber_load_and_get_params")
        plugin_set_parameter = Count-ExecutedToolGroup -Rows $Rows -Aliases @("plugin.set_parameter", "plugin_set_parameter", "set_plugin_param")
        plugin_grabber_apply_control = Count-ExecutedToolGroup -Rows $Rows -Aliases @("plugin_grabber.apply_control", "plugin_grabber_apply_control")
        plugin_prep_continuation = Count-ExecutedToolGroup -Rows $Rows -Aliases @("plugin_prep_continuation")
    }
}

function Count-TypedEventGroup {
    param(
        [object]$Response,
        [string[]]$Aliases
    )
    $count = 0
    foreach ($event in @((Get-OptionalProperty -Object $Response -Name "typed_events"))) {
        $eventType = [string](Get-OptionalProperty -Object $event -Name "event_type")
        $state = Get-OptionalProperty -Object $event -Name "state"
        $stateKind = [string](Get-OptionalProperty -Object $state -Name "kind")
        if (($Aliases -contains $eventType) -or ($Aliases -contains $stateKind)) {
            $count++
        }
    }
    return $count
}

function Typed-Event-Counts {
    param([object]$Response)
    return [ordered]@{
        terminal_result = Count-TypedEventGroup -Response $Response -Aliases @("TerminalResult", "terminal_result")
        user_input_request = Count-TypedEventGroup -Response $Response -Aliases @("UserInputRequest", "user_input_request")
        pending_candidate = Count-TypedEventGroup -Response $Response -Aliases @("PendingCandidate", "pending_candidate")
        approval_request = Count-TypedEventGroup -Response $Response -Aliases @("ApprovalRequest", "approval_request")
    }
}

function Count-InteractionGroup {
    param(
        [object]$Response,
        [string[]]$Aliases
    )
    $count = 0
    foreach ($request in @((Get-OptionalProperty -Object $Response -Name "interaction_requests"))) {
        foreach ($key in @("type", "kind", "stage", "workflow")) {
            $value = [string](Get-OptionalProperty -Object $request -Name $key)
            if ($Aliases -contains $value) {
                $count++
                break
            }
        }
    }
    return $count
}

function Interaction-Counts {
    param([object]$Response)
    return [ordered]@{
        plugin_prep_continuation = Count-InteractionGroup -Response $Response -Aliases @("plugin_prep_continuation")
        plugin_parameter_treatment = Count-InteractionGroup -Response $Response -Aliases @("plugin_parameter_treatment", "plugin_prep_worker")
    }
}

function Assert-NoMutationRoute {
    param(
        [object]$Rows,
        [string]$Label
    )
    $tools = Tool-Names -Rows $Rows
    Assert-ToolAbsent -Tools $tools -Aliases @("mix.propose_tick", "mix_propose_tick") -Label ($Label + " mix.propose_tick")
    Assert-ToolAbsent -Tools $tools -Aliases @("mix.apply_tick", "mix_apply_tick") -Label ($Label + " mix.apply_tick")
    Assert-ToolAbsent -Tools $tools -Aliases @("daw.invoke", "daw_invoke") -Label ($Label + " daw.invoke")
    Assert-ToolAbsent -Tools $tools -Aliases @("track.volume", "track_volume") -Label ($Label + " track.volume")
    Assert-ToolAbsent -Tools $tools -Aliases @("track.pan", "track_pan") -Label ($Label + " track.pan")
    Assert-ToolAbsent -Tools $tools -Aliases @("plugin.set_parameter", "plugin_set_parameter", "set_plugin_param") -Label ($Label + " plugin.set_parameter")
    Assert-ToolAbsent -Tools $tools -Aliases @("plugin_grabber.apply_control", "plugin_grabber_apply_control") -Label ($Label + " plugin_grabber.apply_control")
}

function Compact-ToolResult {
    param([object]$Row)
    $result = Get-OptionalProperty -Object $Row -Name "result"
    $compact = [ordered]@{
        tool = [string](Get-OptionalProperty -Object $Row -Name "tool")
        command_name = [string](Get-OptionalProperty -Object $Row -Name "command_name")
        tool_status = [string](Get-OptionalProperty -Object $Row -Name "status")
        error = [string](Get-OptionalProperty -Object $Row -Name "error")
    }
    foreach ($key in @("operation", "track_id", "track_name", "plugin_id", "plugin_name", "parameter_count", "quick_control_count", "delta_db", "delta_pan", "target_pan", "before_db", "after_db", "before_pan", "after_pan", "tick_id", "kernel_command", "requires_confirmation", "requires_refresh", "observation_id")) {
        $value = Get-OptionalProperty -Object $result -Name $key
        if ($null -ne $value -and -not [string]::IsNullOrWhiteSpace([string]$value)) {
            $compact[$key] = $value
        }
    }
    return $compact
}

function Get-AgentEvents {
    param(
        [string]$ConversationID,
        [int64]$Since = 0,
        [int]$Limit = 80
    )
    return Invoke-Json -Method GET -Uri ($AgentHttp.TrimEnd("/") + "/agent/events?conversation_id=" + [uri]::EscapeDataString($ConversationID) + "&since=" + $Since + "&limit=" + $Limit) -TimeoutSec 10
}

function Extract-PendingCandidates {
    param([object]$Events)
    $out = @()
    foreach ($event in @((Get-OptionalProperty -Object $Events -Name "events"))) {
        if ([string](Get-OptionalProperty -Object $event -Name "type") -eq "mix_tick.pending") {
            $out += (Get-OptionalProperty -Object $event -Name "payload")
        }
    }
    return $out
}

function Extract-TreatmentPendingCandidates {
    param([object]$Events)
    $out = @()
    foreach ($event in @((Get-OptionalProperty -Object $Events -Name "events"))) {
        if ([string](Get-OptionalProperty -Object $event -Name "type") -eq "mix_treatment.pending") {
            $out += (Get-OptionalProperty -Object $event -Name "payload")
        }
    }
    return $out
}

function Extract-ResolverDecisions {
    param([object]$Events)
    $out = @()
    foreach ($event in @((Get-OptionalProperty -Object $Events -Name "events"))) {
        if ([string](Get-OptionalProperty -Object $event -Name "type") -eq "mix_treatment.resolver_decision") {
            $out += (Get-OptionalProperty -Object $event -Name "payload")
        }
    }
    return $out
}

function Collection-ContainsText {
    param(
        [object]$Values,
        [string]$Needle
    )
    foreach ($value in @($Values)) {
        if ([string]$value -like ("*" + $Needle + "*")) {
            return $true
        }
    }
    return $false
}

function Assert-DiagnosisContextPayload {
    param(
        [object]$Payload,
        [string]$Label,
        [string]$ExpectedProblemKind = "",
        [string]$ExpectedStrategy = "",
        [string[]]$ExpectedMissing = @()
    )
    $diagID = [string](Get-OptionalProperty -Object $Payload -Name "diagnosis_context_id")
    $diagnosis = Get-OptionalProperty -Object $Payload -Name "diagnosis_context"
    if ($null -eq $diagnosis) {
        $diagnosis = Get-OptionalProperty -Object $Payload -Name "mix_diagnosis_context"
    }
    if ($null -eq $diagnosis) {
        Fail ($Label + " did not include diagnosis_context")
    }
    $schema = [string](Get-OptionalProperty -Object $diagnosis -Name "schema_version")
    if ($schema -ne "mix_diagnosis_context.v0") {
        Fail ($Label + " diagnosis_context schema=" + $schema)
    }
    if ([string]::IsNullOrWhiteSpace($diagID)) {
        $diagID = [string](Get-OptionalProperty -Object $diagnosis -Name "id")
    }
    if ([string]::IsNullOrWhiteSpace($diagID)) {
        Fail ($Label + " diagnosis_context missing id")
    }
    if (-not [string]::IsNullOrWhiteSpace($ExpectedProblemKind)) {
        $problemKind = [string](Get-OptionalProperty -Object $diagnosis -Name "problem_kind")
        if ($problemKind -ne $ExpectedProblemKind) {
            Fail ($Label + " diagnosis problem_kind=" + $problemKind + " expected=" + $ExpectedProblemKind)
        }
    }
    if (-not [string]::IsNullOrWhiteSpace($ExpectedStrategy)) {
        $recommendation = Get-OptionalProperty -Object $diagnosis -Name "recommendation"
        $strategy = [string](Get-OptionalProperty -Object $recommendation -Name "strategy")
        if ($strategy -ne $ExpectedStrategy) {
            Fail ($Label + " diagnosis strategy=" + $strategy + " expected=" + $ExpectedStrategy)
        }
    }
    foreach ($missingKey in $ExpectedMissing) {
        $foundMissing = $false
        foreach ($row in @((Get-OptionalProperty -Object $diagnosis -Name "missing_evidence"))) {
            if ([string](Get-OptionalProperty -Object $row -Name "key") -eq $missingKey) {
                $foundMissing = $true
                break
            }
        }
        if (-not $foundMissing) {
            Fail ($Label + " diagnosis missing_evidence did not include " + $missingKey + ". diagnosis=" + ($diagnosis | ConvertTo-Json -Depth 12 -Compress))
        }
    }
    return $diagnosis
}

function Assert-LowMudAcousticEvidence {
    param(
        [object]$Payload,
        [string]$Label,
        [switch]$AllowReady
    )
    $diagnosis = Assert-DiagnosisContextPayload -Payload $Payload -Label $Label -ExpectedProblemKind "low_mud"
    $recommendation = Get-OptionalProperty -Object $diagnosis -Name "recommendation"
    $strategy = [string](Get-OptionalProperty -Object $recommendation -Name "strategy")
    $status = Get-OptionalProperty -Object $diagnosis -Name "evidence_status"
    $bandStatus = [string](Get-OptionalProperty -Object $status -Name "band_energy")
    if ([string]::IsNullOrWhiteSpace($bandStatus)) {
        $bandStatus = "missing"
    }
    if ($bandStatus -eq "ready") {
        if (-not $AllowReady) {
            Fail ($Label + " unexpectedly had ready band evidence")
        }
        if ($strategy -notin @("eq_cut_low_mid", "conservative_eq_cut_low_mid", "band_observed_conservative")) {
            Fail ($Label + " ready band evidence did not upgrade low_mud strategy. strategy=" + $strategy)
        }
        foreach ($row in @((Get-OptionalProperty -Object $diagnosis -Name "missing_evidence"))) {
            if ([string](Get-OptionalProperty -Object $row -Name "key") -eq "band_energy_summary") {
                Fail ($Label + " ready band evidence was still listed missing. diagnosis=" + ($diagnosis | ConvertTo-Json -Depth 12 -Compress))
            }
        }
        $refs = @((Get-OptionalProperty -Object $diagnosis -Name "evidence_refs"))
        if ((-not (Collection-ContainsText -Values $refs -Needle "band_energy_summary")) -and (-not (Collection-ContainsText -Values $refs -Needle "observed.band_energy"))) {
            Fail ($Label + " ready band evidence did not cite band_energy_summary. refs=" + ($refs -join ","))
        }
    }
    else {
        if ($strategy -ne "conservative_probe") {
            Fail ($Label + " missing band evidence did not use conservative_probe. strategy=" + $strategy)
        }
        $foundMissing = $false
        $foundReason = $false
        foreach ($row in @((Get-OptionalProperty -Object $diagnosis -Name "missing_evidence"))) {
            if ([string](Get-OptionalProperty -Object $row -Name "key") -eq "band_energy_summary") {
                $foundMissing = $true
                $reason = [string](Get-OptionalProperty -Object $row -Name "reason")
                if (-not [string]::IsNullOrWhiteSpace($reason)) {
                    $foundReason = $true
                }
            }
        }
        if (-not $foundMissing -or -not $foundReason) {
            Fail ($Label + " missing band evidence lacked explicit missing reason. diagnosis=" + ($diagnosis | ConvertTo-Json -Depth 12 -Compress))
        }
    }
	return $diagnosis
}

function Number-Value {
	param([object]$Value)
	if ($null -eq $Value) {
		return $null
	}
	$text = ([string]$Value).Trim()
	if ([string]::IsNullOrWhiteSpace($text) -or $text -eq "<nil>") {
		return $null
	}
	$dummy = 0.0
	if (-not [double]::TryParse($text, [System.Globalization.NumberStyles]::Float, [System.Globalization.CultureInfo]::InvariantCulture, [ref]$dummy)) {
		return $null
	}
	return $dummy
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

function Assert-BridgeSnapshotRow {
	param(
		[object]$Snapshot,
		[string]$Name,
		[string]$ExpectedRequestID,
		[string]$Label
	)
	$row = Get-OptionalProperty -Object $Snapshot -Name $Name
	$status = [string](Get-OptionalProperty -Object $row -Name "status")
	if ([string]::IsNullOrWhiteSpace($status)) {
		Fail ($Label + " acoustic feature " + $Name + " missing status")
	}
	$rowRequestID = [string](Get-OptionalProperty -Object $row -Name "request_id")
	if (-not [string]::IsNullOrWhiteSpace($ExpectedRequestID) -and -not [string]::IsNullOrWhiteSpace($rowRequestID) -and $rowRequestID -ne $ExpectedRequestID) {
		$rowSourceRevision = [string](Get-OptionalProperty -Object $row -Name "source_revision")
		$rowSourceHash = [string](Get-OptionalProperty -Object $row -Name "source_hash")
		if ([string]::IsNullOrWhiteSpace($rowSourceRevision) -and [string]::IsNullOrWhiteSpace($rowSourceHash)) {
			Fail ($Label + " acoustic feature " + $Name + " request_id mismatch without source revision/hash: got=" + $rowRequestID + " expected=" + $ExpectedRequestID)
		}
	}
	if ($status -in @("ready", "partial")) {
		if ([string]::IsNullOrWhiteSpace($rowRequestID)) {
			Fail ($Label + " ready acoustic feature " + $Name + " missing request_id")
		}
		$source = [string](Get-OptionalProperty -Object $row -Name "source")
		if ([string]::IsNullOrWhiteSpace($source)) {
			Fail ($Label + " ready acoustic feature " + $Name + " missing source")
		}
		if ($Name -eq "spectrogram_tiles") {
			if ([string](Get-OptionalProperty -Object $row -Name "feature_type") -ne "spectral_field") {
				Fail ($Label + " spectrogram_tiles feature_type is not spectral_field")
			}
			if ($source -notin @("kernel_tile_ready_direct_collector", "kernel_tile_ready_godot_bridge")) {
				Fail ($Label + " spectrogram_tiles source is not accepted: " + $source)
			}
			$seen = Number-Value -Value (Get-OptionalProperty -Object $row -Name "tile_count_seen")
			$expected = Number-Value -Value (Get-OptionalProperty -Object $row -Name "tile_count_expected")
			if ($null -eq $seen -or $seen -lt 1 -or $null -eq $expected) {
				Fail ($Label + " spectrogram_tiles missing tile counts: " + ($row | ConvertTo-Json -Depth 8 -Compress))
			}
		}
		return
	}
	if ($status -in @("requested", "building")) {
		return
	}
	if ($status -in @("missing", "blocked", "unavailable", "invalid")) {
		$reason = [string](Get-OptionalProperty -Object $row -Name "reason")
		if ([string]::IsNullOrWhiteSpace($reason)) {
			Fail ($Label + " acoustic feature " + $Name + " status " + $status + " missing explicit reason")
		}
		return
	}
	Fail ($Label + " acoustic feature " + $Name + " has unexpected status " + $status)
}

function Assert-ObservationAcousticBridgeReadiness {
	param(
		[object]$Observation,
		[string]$Label
	)
	$global = Get-OptionalProperty -Object $Observation -Name "global_summary"
	$snapshot = Get-OptionalProperty -Object $global -Name "feature_snapshot"
	$latest = Get-OptionalProperty -Object $snapshot -Name "latest_request"
	$requestID = [string](Get-OptionalProperty -Object $latest -Name "request_id")
	if ([string]::IsNullOrWhiteSpace($requestID)) {
		Fail ($Label + " feature_snapshot.latest_request.request_id missing")
	}
	Assert-RequestedFeature -LatestRequest $latest -FeatureType "waveform_envelope"
	Assert-RequestedFeature -LatestRequest $latest -FeatureType "spectral_field"
	foreach ($name in @("spectrogram_tiles", "band_energy_summary", "stereo_relation_summary")) {
		Assert-BridgeSnapshotRow -Snapshot $snapshot -Name $name -ExpectedRequestID $requestID -Label $Label
	}
	$mixPackage = Get-OptionalProperty -Object $Observation -Name "mix_package"
	$currentMetrics = Get-OptionalProperty -Object $mixPackage -Name "current_metrics"
	foreach ($name in @("band_energy", "stereo_relation")) {
		$row = Get-OptionalProperty -Object $currentMetrics -Name $name
		$status = [string](Get-OptionalProperty -Object $row -Name "status")
		if ($status -eq "missing" -and [string]::IsNullOrWhiteSpace([string](Get-OptionalProperty -Object $row -Name "reason"))) {
			Fail ($Label + " current_metrics." + $name + " missing explicit reason")
		}
	}
}

function Assert-AcousticPackageStatusReadiness {
	param(
		[object]$Status,
		[string]$Label
	)
	if ($null -eq $Status) {
		Fail ($Label + " acoustic_package_status missing")
	}
	$schema = [string](Get-OptionalProperty -Object $Status -Name "schema_version")
	if ($schema -ne "acoustic_package_status.v0") {
		Fail ($Label + " acoustic_package_status schema mismatch: " + $schema)
	}
	$layers = Get-OptionalProperty -Object $Status -Name "package_layers"
	foreach ($layerName in @("l1_static", "l2_realtime", "l3_deep")) {
		$layer = Get-OptionalProperty -Object $layers -Name $layerName
		if ($null -eq $layer) {
			Fail ($Label + " acoustic_package_status missing layer " + $layerName)
		}
		$layerStatus = [string](Get-OptionalProperty -Object $layer -Name "status")
		if ([string]::IsNullOrWhiteSpace($layerStatus)) {
			Fail ($Label + " acoustic_package_status layer " + $layerName + " missing status")
		}
		if ($layerStatus -notin @("ready", "partial", "building", "stale", "missing", "deferred", "failed")) {
			Fail ($Label + " acoustic_package_status layer " + $layerName + " unexpected status " + $layerStatus)
		}
	}
	$l1 = Get-OptionalProperty -Object $layers -Name "l1_static"
	$l1Features = Get-OptionalProperty -Object $l1 -Name "features"
	foreach ($name in @("waveform_envelope", "peak_rms_summary", "time_energy")) {
		$feature = Get-OptionalProperty -Object $l1Features -Name $name
		$statusText = [string](Get-OptionalProperty -Object $feature -Name "status")
		if ([string]::IsNullOrWhiteSpace($statusText)) {
			Fail ($Label + " acoustic_package_status L1 feature " + $name + " missing status")
		}
		if ($statusText -in @("missing", "stale", "failed")) {
			Fail ($Label + " acoustic_package_status L1 feature " + $name + " not ready/usable: " + $statusText)
		}
	}
	$l2 = Get-OptionalProperty -Object $layers -Name "l2_realtime"
	$l2Features = Get-OptionalProperty -Object $l2 -Name "features"
	foreach ($name in @("live_meter", "realtime_spectrum", "post_fx_meter", "realtime_stereo_correlation")) {
		$feature = Get-OptionalProperty -Object $l2Features -Name $name
		$statusText = [string](Get-OptionalProperty -Object $feature -Name "status")
		$allowed = @("deferred")
		if ($name -ne "post_fx_meter") {
			$allowed += @("ready", "partial")
		}
		if ($allowed -notcontains $statusText) {
			Fail ($Label + " acoustic_package_status L2 feature " + $name + " has unexpected status " + $statusText)
		}
	}
	$l3 = Get-OptionalProperty -Object $layers -Name "l3_deep"
	$l3Features = Get-OptionalProperty -Object $l3 -Name "features"
	foreach ($name in @("spectrogram_tiles", "band_energy_summary", "stereo_relation_summary", "lufs_analysis", "masking_analysis", "reference_match")) {
		$feature = Get-OptionalProperty -Object $l3Features -Name $name
		$statusText = [string](Get-OptionalProperty -Object $feature -Name "status")
		if ([string]::IsNullOrWhiteSpace($statusText)) {
			Fail ($Label + " acoustic_package_status L3 feature " + $name + " missing status")
		}
		if ($statusText -eq "ready") {
			continue
		}
		if ($name -in @("lufs_analysis", "masking_analysis", "reference_match")) {
			if ($statusText -ne "deferred") {
				Fail ($Label + " acoustic_package_status deferred L3 feature " + $name + " unexpected status " + $statusText)
			}
			continue
		}
		if ($statusText -notin @("partial", "building", "deferred", "failed")) {
			Fail ($Label + " acoustic_package_status L3 feature " + $name + " unexpected status " + $statusText)
		}
	}
}

function Assert-TurnAcousticBridgeReadiness {
	param(
		[object]$TurnResult,
		[string]$Label
	)
	$packageStatus = Get-AcousticPackageStatusFromTurnResult -TurnResult $TurnResult
	if ($null -ne $packageStatus) {
		Assert-AcousticPackageStatusReadiness -Status $packageStatus -Label $Label
		return
	}
	foreach ($row in @((Get-OptionalProperty -Object $TurnResult -Name "rows"))) {
		$result = Get-OptionalProperty -Object $row -Name "result"
		$observation = Get-OptionalProperty -Object $result -Name "observation"
		if ($null -ne $observation) {
			Assert-ObservationAcousticBridgeReadiness -Observation $observation -Label $Label
			return
		}
	}
	Fail ($Label + " did not expose a mix observation result")
}

function Assert-PluginPreparationPlan {
	param([object]$Decision)
    $plan = Get-OptionalProperty -Object $Decision -Name "preparation_plan"
    if ($null -eq $plan) {
        Fail "plugin resolver decision did not include preparation_plan"
    }
    $schema = [string](Get-OptionalProperty -Object $plan -Name "schema_version")
    if ($schema -ne "mix_treatment_preparation.v0") {
        Fail ("unexpected preparation_plan schema=" + $schema)
    }
    $safeRoute = @((Get-OptionalProperty -Object $plan -Name "safe_route"))
    if ($safeRoute -notcontains "plugin_grabber.apply_control") {
        Fail ("preparation_plan safe_route missing plugin_grabber.apply_control. route=" + ($safeRoute -join " -> "))
    }
    $blockedRoutes = @((Get-OptionalProperty -Object $plan -Name "blocked_routes"))
    foreach ($blocked in @("daw.invoke", "plugin.set_parameter")) {
        if ($blockedRoutes -notcontains $blocked) {
            Fail ("preparation_plan blocked_routes missing " + $blocked + ". blocked=" + ($blockedRoutes -join " -> "))
        }
    }
    $steps = @((Get-OptionalProperty -Object $plan -Name "steps"))
    if ($steps.Count -lt 1) {
        Fail "preparation_plan did not include steps"
    }
    [void](Assert-DiagnosisContextPayload -Payload $plan -Label "plugin preparation plan")
}

function Assert-PluginResponsePreparationPlan {
    param([object]$Response)
    $workflowData = Get-OptionalProperty -Object $Response -Name "workflow_data"
    $plan = Get-OptionalProperty -Object $workflowData -Name "mix_treatment_preparation_plan"
    if ($null -eq $plan) {
        $pluginLearning = Get-OptionalProperty -Object $Response -Name "plugin_learning"
        $plan = Get-OptionalProperty -Object $pluginLearning -Name "mix_treatment_preparation_plan"
    }
    if ($null -eq $plan) {
        Fail "plugin preparation response did not include mix_treatment_preparation_plan"
    }
    $schema = [string](Get-OptionalProperty -Object $plan -Name "schema_version")
    if ($schema -ne "mix_treatment_preparation.v0") {
        Fail ("unexpected plugin response preparation plan schema=" + $schema)
    }
    [void](Assert-DiagnosisContextPayload -Payload $plan -Label "plugin response preparation plan")
    $interactions = @((Get-OptionalProperty -Object $Response -Name "interaction_requests"))
    if ($interactions.Count -lt 1) {
        Fail "plugin preparation response did not include interaction_requests"
    }
    foreach ($interaction in $interactions) {
        if ([string](Get-OptionalProperty -Object $interaction -Name "workflow") -eq "plugin_grabber_load_and_get_params") {
            return
        }
        $payload = Get-OptionalProperty -Object $interaction -Name "payload"
        $interactionPlan = Get-OptionalProperty -Object $payload -Name "mix_treatment_preparation_plan"
        if ($null -ne $interactionPlan) {
            return
        }
        $data = Get-OptionalProperty -Object $interaction -Name "data"
        $interactionPlan = Get-OptionalProperty -Object $data -Name "mix_treatment_preparation_plan"
        if ($null -ne $interactionPlan) {
            return
        }
    }
    Fail "plugin preparation interaction did not expose workflow or mix_treatment_preparation_plan"
}

function Get-PluginPreparationInteractionID {
    param([object]$Response)
    foreach ($interaction in @((Get-OptionalProperty -Object $Response -Name "interaction_requests"))) {
        if ([string](Get-OptionalProperty -Object $interaction -Name "workflow") -ne "plugin_grabber_load_and_get_params") {
            continue
        }
        $id = [string](Get-OptionalProperty -Object $interaction -Name "id")
        if (-not [string]::IsNullOrWhiteSpace($id)) {
            return $id
        }
    }
    Fail "plugin preparation response did not include a plugin_grabber_load_and_get_params interaction id"
}

function Get-PluginParameterTreatmentInteractionID {
    param([object]$Response)
    foreach ($interaction in @((Get-OptionalProperty -Object $Response -Name "interaction_requests"))) {
        $type = [string](Get-OptionalProperty -Object $interaction -Name "type")
        $workflow = [string](Get-OptionalProperty -Object $interaction -Name "workflow")
        if ($type -ne "plugin_parameter_treatment" -and $workflow -ne "plugin_prep_worker") {
            continue
        }
        $id = [string](Get-OptionalProperty -Object $interaction -Name "id")
        if (-not [string]::IsNullOrWhiteSpace($id)) {
            return $id
        }
    }
    Fail "plugin prep worker response did not include a plugin_parameter_treatment interaction id"
}

function Extract-GainTreatmentCandidates {
    param([object]$Events)
    $out = @()
    foreach ($candidate in @(Extract-TreatmentPendingCandidates -Events $Events)) {
        if ([string](Get-OptionalProperty -Object $candidate -Name "action_kind") -eq "gain_balance") {
            $out += $candidate
        }
    }
    return $out
}

function Extract-PanTreatmentCandidates {
    param([object]$Events)
    $out = @()
    foreach ($candidate in @(Extract-TreatmentPendingCandidates -Events $Events)) {
        if ([string](Get-OptionalProperty -Object $candidate -Name "action_kind") -eq "pan_balance") {
            $out += $candidate
        }
    }
    return $out
}

function Extract-ObservationIDs {
    param([object]$Rows)
    $out = @()
    foreach ($row in @($Rows)) {
        $result = Get-OptionalProperty -Object $row -Name "result"
        $observationID = [string](Get-OptionalProperty -Object $result -Name "observation_id")
        if ([string]::IsNullOrWhiteSpace($observationID)) {
            $observation = Get-OptionalProperty -Object $result -Name "observation"
            $observationID = [string](Get-OptionalProperty -Object $observation -Name "observation_id")
        }
        if (-not [string]::IsNullOrWhiteSpace($observationID)) {
            $out += $observationID
        }
    }
    return $out
}

function Get-AcousticPackageStatusFromTurnResult {
    param([object]$TurnResult)
    foreach ($row in @((Get-OptionalProperty -Object $TurnResult -Name "rows"))) {
        $result = Get-OptionalProperty -Object $row -Name "result"
        $status = Get-OptionalProperty -Object $result -Name "acoustic_package_status"
        if ($null -ne $status) {
            return $status
        }
        $digest = Get-OptionalProperty -Object $result -Name "acoustic_digest"
        $status = Get-OptionalProperty -Object $digest -Name "acoustic_package_status"
        if ($null -ne $status) {
            return $status
        }
    }
    $response = Get-OptionalProperty -Object $TurnResult -Name "response"
    return Get-OptionalProperty -Object $response -Name "acoustic_package_status"
}

function Test-AcousticPackageDeepIncomplete {
    param([object]$Status)
    if ($null -eq $Status) {
        return $false
    }
    $layers = Get-OptionalProperty -Object $Status -Name "package_layers"
    $l3 = Get-OptionalProperty -Object $layers -Name "l3_deep"
    $l3Status = [string](Get-OptionalProperty -Object $l3 -Name "status")
    if (-not [string]::IsNullOrWhiteSpace($l3Status) -and $l3Status -ne "ready") {
        return $true
    }
    $features = Get-OptionalProperty -Object $l3 -Name "features"
    foreach ($name in @("spectrogram_tiles", "band_energy_summary", "stereo_relation_summary")) {
        $feature = Get-OptionalProperty -Object $features -Name $name
        $featureStatus = [string](Get-OptionalProperty -Object $feature -Name "status")
        if ($featureStatus -ne "ready") {
            return $true
        }
    }
    return $false
}

function Test-TurnResultAcousticPackageDeepIncomplete {
    param([object]$TurnResult)
    return (Test-AcousticPackageDeepIncomplete -Status (Get-AcousticPackageStatusFromTurnResult -TurnResult $TurnResult))
}

function Assert-NoExecutionQuestion {
    param(
        [object]$Turn,
        [string]$Label
    )
    $reply = [string](Get-OptionalProperty -Object $Turn -Name "reply")
    $lower = $reply.ToLowerInvariant()
    $continueExecute = Join-UnicodeChars @(0x7EE7, 0x7EED, 0x6267, 0x884C)
    $executeQuestion = Join-UnicodeChars @(0x6267, 0x884C, 0x5417)
    if ($lower.Contains("execute") -or $lower.Contains("continue") -or $reply.Contains($continueExecute) -or $reply.Contains($executeQuestion)) {
        Fail ($Label + " asked for execution despite incomplete acoustic package: " + $reply)
    }
}

function Convert-AgentResponseToTurn {
    param(
        [string]$ConversationID,
        [string]$Message,
        [string]$Label,
        [object]$Response
    )
    $fileName = ("chat_" + $Label + ".json")
    ConvertTo-JsonFile -Value $Response -Path (Join-Path $ArtifactDir $fileName)
    $rawRows = Get-OptionalProperty -Object $Response -Name "executed_kernel_reply"
    $rows = @()
    if ($null -ne $rawRows) {
        $rows = @($rawRows)
    }
    $tools = @(Tool-Names -Rows $rows)
    $observationIDs = @(Extract-ObservationIDs -Rows $rows)
    $resolvedConversationID = $ConversationID
    if ([string]::IsNullOrWhiteSpace($resolvedConversationID)) {
        $resolvedConversationID = [string](Get-OptionalProperty -Object $Response -Name "conversation_id")
    }
    $replyText = [string](Get-OptionalProperty -Object $Response -Name "reply")
    if ([string]::IsNullOrWhiteSpace($replyText)) {
        $replyText = [string](Get-OptionalProperty -Object $Response -Name "message")
    }
    $turn = [ordered]@{
        label = $Label
        conversation_id = $resolvedConversationID
        message = $Message
        stop_reason = [string](Get-OptionalProperty -Object $Response -Name "stop_reason")
        goal_status = [string](Get-OptionalProperty -Object $Response -Name "goal_status")
        reply = $replyText
        tool_route = $tools
        tool_counts = Tool-Counts -Rows $rows
        typed_event_counts = Typed-Event-Counts -Response $Response
        interaction_counts = Interaction-Counts -Response $Response
        observations = $observationIDs
        executed = @()
        response_file = $fileName
    }
    foreach ($row in $rows) {
        if ($null -ne $row) {
            $turn["executed"] += (Compact-ToolResult -Row $row)
        }
    }
    return @{
        response = $Response
        turn = $turn
        rows = $rows
        tools = $tools
    }
}

function Invoke-BlindTurn {
    param(
        [string]$ConversationID,
        [string]$Message,
        [string]$Label
    )
    $response = Invoke-AgentChat -ConversationID $ConversationID -Message $Message
    return Convert-AgentResponseToTurn -ConversationID $ConversationID -Message $Message -Label $Label -Response $response
}

function Invoke-BlindInteraction {
    param(
        [string]$ConversationID,
        [string]$InteractionID,
        [string]$Label,
        [string]$Decision = "approve",
        [string]$ActionID = "approve",
        [object]$Payload = $null
    )
    $response = Invoke-AgentInteractionRespond -InteractionID $InteractionID -Decision $Decision -ActionID $ActionID -Payload $Payload
    return Convert-AgentResponseToTurn -ConversationID $ConversationID -Message ("interaction:" + $Decision + ":" + $InteractionID) -Label $Label -Response $response
}

function Record-Turn {
    param([object]$Turn)
    $script:summary["turns"] += $Turn
    $script:summary["tool_route"] += @($Turn.tool_route)
    $script:summary["stop_reasons"][$Turn.label] = $Turn.stop_reason
    foreach ($observationID in @($Turn.observations)) {
        $script:summary["observations"] += $observationID
    }
    $counts = $Turn.tool_counts
    $allowedPluginPrepWrite = ([string]$Turn.label -eq "low_mud_parameter_candidate_confirm" -and [string]$Turn.stop_reason -eq "plugin_prep_parameter_treatment_applied_reobserved")
    if ([int]$counts.daw_invoke -gt 0 -or [int]$counts.track_volume -gt 0 -or [int]$counts.track_pan -gt 0 -or (([int]$counts.plugin_set_parameter -gt 0) -and -not $allowedPluginPrepWrite)) {
        $script:summary["blocked_mutation_attempts"] += [ordered]@{
            label = $Turn.label
            route = $Turn.tool_route
            counts = $counts
        }
    }
}

function Record-Events {
    param(
        [string]$ConversationID,
        [string]$Label,
        [int64]$Since = 0
    )
    $events = Get-AgentEvents -ConversationID $ConversationID -Since $Since -Limit 80
    ConvertTo-JsonFile -Value $events -Path (Join-Path $ArtifactDir ("events_" + $Label + ".json"))
    foreach ($candidate in @(Extract-PendingCandidates -Events $events)) {
        $script:summary["pending_candidates"] += $candidate
    }
    foreach ($candidate in @(Extract-TreatmentPendingCandidates -Events $events)) {
        $script:summary["treatment_pending_candidates"] += $candidate
    }
    foreach ($decision in @(Extract-ResolverDecisions -Events $events)) {
        $script:summary["resolver_decisions"] += $decision
    }
    return $events
}

function Assert-ObserveOnlyFirstTurn {
    param(
        [object]$Turn,
        [string]$Label
    )
    if ([string]$Turn.stop_reason -notin @("done", "needs_confirmation", "needs_clarification")) {
        Fail ($Label + " stop_reason=" + [string]$Turn.stop_reason)
    }
    $counts = $Turn.tool_counts
    if ([int]$counts.observe -lt 1) {
        Fail ($Label + " did not run mix.observe. route=" + ($Turn.tool_route -join " -> "))
    }
    Assert-NoMutationRoute -Rows $Turn.executed -Label $Label
}

function Assert-NoPendingConfirmationGuard {
    param([object]$Turn)
    if ([string]$Turn.stop_reason -ne "no_pending_mix_tick_candidate") {
        Fail ("no-pending confirmation stop_reason=" + [string]$Turn.stop_reason)
    }
    Assert-NoMutationRoute -Rows $Turn.executed -Label "no-pending confirmation"
}

function Assert-ConfirmationRoute {
    param([object]$Turn)
    if ([string]$Turn.stop_reason -notin @("mix_tick_applied_reobserved", "mix_treatment_resolved_ready_gain_tick", "mix_treatment_resolved_ready_pan_tick")) {
        Fail ("confirmation stop_reason=" + [string]$Turn.stop_reason)
    }
    Assert-ToolPresent -Tools $Turn.tool_route -Aliases @("mix.propose_tick", "mix_propose_tick") -Label "mix.propose_tick"
    Assert-ToolPresent -Tools $Turn.tool_route -Aliases @("mix.apply_tick", "mix_apply_tick") -Label "mix.apply_tick"
    Assert-ToolPresent -Tools $Turn.tool_route -Aliases @("mix.observe", "mix_observe", "mix.request_observation", "mix_request_observation") -Label "post-apply mix.observe"
    Assert-ToolAbsent -Tools $Turn.tool_route -Aliases @("daw.invoke", "daw_invoke") -Label "daw.invoke"
    Assert-ToolAbsent -Tools $Turn.tool_route -Aliases @("track.volume", "track_volume") -Label "track.volume"
    Assert-ToolAbsent -Tools $Turn.tool_route -Aliases @("track.pan", "track_pan") -Label "track.pan"
    Assert-ToolAbsent -Tools $Turn.tool_route -Aliases @("plugin.set_parameter", "plugin_set_parameter", "set_plugin_param") -Label "plugin.set_parameter"
}

function Assert-PluginPrepConfirmationGate {
    param(
        [object]$TurnResult,
        [string]$Label,
        [switch]$RequirePreparationExecution
    )
    $turn = $TurnResult.turn
    $counts = $turn.tool_counts
    $typedCounts = $turn.typed_event_counts
    $interactionCounts = $turn.interaction_counts
    if ([int]$counts.rack_add_node -gt 1) {
        Fail ($Label + " loaded plugin more than once. counts=" + ($counts | ConvertTo-Json -Compress))
    }
    if ([int]$counts.plugin_get_parameters -gt 1) {
        Fail ($Label + " fetched plugin parameters more than once. counts=" + ($counts | ConvertTo-Json -Compress))
    }
    if ([int]$counts.plugin_grabber_load_and_get_params -gt 1) {
        Fail ($Label + " ran plugin_grabber.load_and_get_params more than once. counts=" + ($counts | ConvertTo-Json -Compress))
    }
    if ($RequirePreparationExecution) {
        if (([int]$counts.rack_add_node -lt 1) -and ([int]$counts.plugin_grabber_load_and_get_params -lt 1)) {
            Fail ($Label + " did not load or instantiate the plugin during approved preparation. counts=" + ($counts | ConvertTo-Json -Compress))
        }
        if (([int]$counts.plugin_get_parameters -lt 1) -and ([int]$counts.plugin_grabber_load_and_get_params -lt 1)) {
            Fail ($Label + " did not fetch plugin parameters during approved preparation. counts=" + ($counts | ConvertTo-Json -Compress))
        }
        if ([string]$turn.stop_reason -notmatch "^plugin_prep_") {
            Fail ($Label + " approved preparation did not enter the plugin prep bridge. stop_reason=" + [string]$turn.stop_reason)
        }
        if ([int]$typedCounts.terminal_result -lt 1) {
            Fail ($Label + " approved preparation did not emit TerminalResult. typed_counts=" + ($typedCounts | ConvertTo-Json -Compress))
        }
    }
    if ([int]$counts.plugin_prep_continuation -gt 1 -or [int]$interactionCounts.plugin_prep_continuation -gt 1) {
        Fail ($Label + " produced more than one plugin prep continuation. turn=" + ($turn | ConvertTo-Json -Depth 12 -Compress))
    }
    if ([int]$counts.plugin_set_parameter -ne 0) {
        Fail ($Label + " wrote plugin parameters during preparation. counts=" + ($counts | ConvertTo-Json -Compress))
    }
    if ([int]$counts.plugin_grabber_apply_control -ne 0) {
        Fail ($Label + " applied plugin control during preparation. counts=" + ($counts | ConvertTo-Json -Compress))
    }
    if ([string]$turn.stop_reason -match "^plugin_prep_" -and ([int]$typedCounts.terminal_result -lt 1) -and ([int]$typedCounts.user_input_request -lt 1)) {
        Fail ($Label + " entered plugin prep without TerminalResult or UserInputRequest. typed_counts=" + ($typedCounts | ConvertTo-Json -Compress))
    }
    if ([string]$turn.stop_reason -eq "plugin_prep_waiting_continuation" -and ([int]$typedCounts.user_input_request -lt 1 -or [int]$interactionCounts.plugin_prep_continuation -ne 1)) {
        Fail ($Label + " did not expose exactly one continuation user input. turn=" + ($turn | ConvertTo-Json -Depth 12 -Compress))
    }
    if ([string]$turn.stop_reason -eq "plugin_prep_parameter_candidate_pending") {
        if ([int]$typedCounts.pending_candidate -lt 1) {
            Fail ($Label + " did not emit a PendingCandidate. typed_counts=" + ($typedCounts | ConvertTo-Json -Compress))
        }
        if ([int]$interactionCounts.plugin_parameter_treatment -ne 1) {
            Fail ($Label + " did not expose exactly one plugin_parameter_treatment interaction. turn=" + ($turn | ConvertTo-Json -Depth 12 -Compress))
        }
        $pluginInteraction = $null
        foreach ($interaction in @((Get-OptionalProperty -Object $TurnResult.response -Name "interaction_requests"))) {
            $type = [string](Get-OptionalProperty -Object $interaction -Name "type")
            $workflow = [string](Get-OptionalProperty -Object $interaction -Name "workflow")
            if ($type -eq "plugin_parameter_treatment" -or $workflow -eq "plugin_prep_worker") {
                $pluginInteraction = $interaction
                break
            }
        }
        if ($null -eq $pluginInteraction) {
            Fail ($Label + " did not expose a plugin_prep_worker interaction payload")
        }
        $missingEvidenceText = Join-UnicodeChars @(0x672A, 0x53D6, 0x5F97, 0x53EF, 0x9760, 0x7684, 0x9891, 0x6BB5, 0x89C2, 0x6D4B)
        $conservativeProbeText = Join-UnicodeChars @(0x4FDD, 0x5B88, 0x8BD5, 0x63A2)
        $applyLabel = Join-UnicodeChars @(0x5E94, 0x7528, 0x5019, 0x9009)
        $reviseLabel = Join-UnicodeChars @(0x8C03, 0x6574, 0x5019, 0x9009)
        $cancelLabel = Join-UnicodeChars @(0x53D6, 0x6D88)
        $body = [string](Get-OptionalProperty -Object $pluginInteraction -Name "body")
        if ($body -match "Plugin Prep Worker prepared|Apply candidate|Revise|Cancel") {
            Fail ($Label + " plugin prep worker body still contains English UX text: " + $body)
        }
        $labels = @()
        foreach ($action in @((Get-OptionalProperty -Object $pluginInteraction -Name "actions"))) {
            $labels += [string](Get-OptionalProperty -Object $action -Name "label")
        }
        foreach ($expectedLabel in @($applyLabel, $reviseLabel, $cancelLabel)) {
            if ($labels -notcontains $expectedLabel) {
                Fail ($Label + " plugin prep worker action labels missing " + $expectedLabel + ". labels=" + ($labels -join ","))
            }
        }
        $payload = Get-OptionalProperty -Object $pluginInteraction -Name "payload"
        $candidate = Get-OptionalProperty -Object $payload -Name "plugin_prep_worker"
        if ($null -eq $candidate) {
            $candidate = Get-OptionalProperty -Object $payload -Name "plugin_parameter_treatment_candidate"
        }
        if ($null -eq $candidate) {
            Fail ($Label + " plugin prep worker interaction missing candidate display payload")
        }
        $evidenceStatus = Get-OptionalProperty -Object $candidate -Name "evidence_status"
        $bandStatus = [string](Get-OptionalProperty -Object $evidenceStatus -Name "band_energy")
        $statusStrategy = [string](Get-OptionalProperty -Object $evidenceStatus -Name "strategy")
        $displaySummary = [string](Get-OptionalProperty -Object $candidate -Name "display_summary")
        if ($bandStatus -eq "ready") {
            if ($statusStrategy -notin @("eq_cut_low_mid", "conservative_eq_cut_low_mid", "band_observed_conservative")) {
                Fail ($Label + " plugin prep worker ready band evidence did not upgrade strategy. evidence_status=" + ($evidenceStatus | ConvertTo-Json -Depth 8 -Compress))
            }
            $bandSummary = Get-OptionalProperty -Object $candidate -Name "band_energy_summary"
            if ($null -eq $bandSummary) {
                Fail ($Label + " plugin prep worker ready band evidence missing band_energy_summary payload")
            }
        }
        else {
            if ($bandStatus -ne "missing") {
                Fail ($Label + " plugin prep worker evidence_status.band_energy was neither ready nor missing. evidence_status=" + ($evidenceStatus | ConvertTo-Json -Depth 8 -Compress))
            }
            if ($statusStrategy -ne "conservative_probe") {
                Fail ($Label + " plugin prep worker evidence_status.strategy was not conservative_probe. evidence_status=" + ($evidenceStatus | ConvertTo-Json -Depth 8 -Compress))
            }
            if ((-not $body.Contains($missingEvidenceText)) -and (-not $body.Contains($conservativeProbeText))) {
                Fail ($Label + " plugin prep worker body missing evidence degradation explanation: " + $body)
            }
            if ((-not $displaySummary.Contains($missingEvidenceText)) -and (-not $displaySummary.Contains($conservativeProbeText))) {
                Fail ($Label + " plugin prep worker display_summary missing evidence degradation: " + $displaySummary)
            }
        }
        $diagnosis = Assert-LowMudAcousticEvidence -Payload $candidate -Label ($Label + " plugin prep worker candidate") -AllowReady
        $diagID = [string](Get-OptionalProperty -Object $candidate -Name "diagnosis_context_id")
        if ([string]::IsNullOrWhiteSpace($diagID)) {
            $diagID = [string](Get-OptionalProperty -Object $diagnosis -Name "id")
        }
        $candidateRefs = @((Get-OptionalProperty -Object $candidate -Name "evidence_refs"))
        if (-not (Collection-ContainsText -Values $candidateRefs -Needle $diagID)) {
            Fail ($Label + " plugin prep worker evidence_refs did not cite diagnosis_context_id. refs=" + ($candidateRefs -join ",") + " diag_id=" + $diagID)
        }
        $parameterSummary = @((Get-OptionalProperty -Object $candidate -Name "parameter_change_summary"))
        if ($parameterSummary.Count -lt 1) {
            Fail ($Label + " plugin prep worker candidate missing parameter_change_summary")
        }
        if ([int]$counts.plugin_set_parameter -ne 0 -or [int]$counts.plugin_grabber_apply_control -ne 0) {
            Fail ($Label + " wrote plugin parameters before candidate confirmation. counts=" + ($counts | ConvertTo-Json -Compress))
        }
    }
    $script:summary["plugin_prep_audit"] += [ordered]@{
        label = $Label
        stop_reason = [string]$turn.stop_reason
        tool_route = $turn.tool_route
        tool_counts = $counts
        typed_event_counts = $typedCounts
        interaction_counts = $interactionCounts
        response_file = $turn.response_file
    }
}

function Assert-PluginPrepWorkerAppliedRoute {
    param(
        [object]$TurnResult,
        [string]$Label
    )
    $turn = $TurnResult.turn
    $counts = $turn.tool_counts
    $typedCounts = $turn.typed_event_counts
    if ([string]$turn.stop_reason -ne "plugin_prep_parameter_treatment_applied_reobserved") {
        Fail ($Label + " did not apply and reobserve. stop_reason=" + [string]$turn.stop_reason)
    }
    if ([int]$counts.plugin_set_parameter -lt 1) {
        Fail ($Label + " did not write any plugin parameter after confirmation. counts=" + ($counts | ConvertTo-Json -Compress))
    }
    if ([int]$counts.observe -lt 1) {
        Fail ($Label + " did not reobserve after plugin parameter write. counts=" + ($counts | ConvertTo-Json -Compress))
    }
    Assert-ToolAbsent -Tools $turn.tool_route -Aliases @("daw.invoke", "daw_invoke") -Label ($Label + " daw.invoke")
    Assert-ToolAbsent -Tools $turn.tool_route -Aliases @("track.volume", "track_volume") -Label ($Label + " track.volume")
    Assert-ToolAbsent -Tools $turn.tool_route -Aliases @("track.pan", "track_pan") -Label ($Label + " track.pan")
    Assert-ToolAbsent -Tools $turn.tool_route -Aliases @("plugin_grabber.apply_control", "plugin_grabber_apply_control") -Label ($Label + " plugin_grabber.apply_control")
    if ([int]$typedCounts.pending_candidate -lt 1 -or [int]$typedCounts.terminal_result -lt 1) {
        Fail ($Label + " missing typed PendingCandidate or TerminalResult. typed_counts=" + ($typedCounts | ConvertTo-Json -Compress))
    }
    $script:summary["plugin_prep_audit"] += [ordered]@{
        label = $Label
        stop_reason = [string]$turn.stop_reason
        tool_route = $turn.tool_route
        tool_counts = $counts
        typed_event_counts = $typedCounts
        interaction_counts = $turn.interaction_counts
        response_file = $turn.response_file
    }
}

function Start-KernelIfRequested {
    param(
        [string]$KernelExe,
        [int]$ReqPort,
        [int]$SubPort
    )
    if (-not $StartKernel) {
        return
    }
    if ($ReuseKernel -and $null -ne (Get-TcpListener -Port $ReqPort)) {
        Write-Ok ("reusing kernel command port pid=" + (Get-TcpListener -Port $ReqPort).OwningProcess)
        return
    }
    $existing = Get-TcpListener -Port $ReqPort
    if ($null -ne $existing) {
        Fail ("Kernel command port is already listening on " + $ReqPort + " pid=" + $existing.OwningProcess + ". Stop it or rerun with -ReuseKernel.")
    }
    if ($null -eq (Get-TcpListener -Port $ReqPort)) {
        Start-Process -FilePath $KernelExe -WorkingDirectory (Split-Path -Parent $KernelExe) -WindowStyle Hidden | Out-Null
        $ready = Wait-TcpListener -Port $ReqPort -TimeoutSeconds $TimeoutSeconds
        if ($null -eq $ready) {
            Fail ("Kernel command port did not become ready: " + $ReqPort)
        }
        [void](Wait-TcpListener -Port $SubPort -TimeoutSeconds 5)
        Write-Ok ("started kernel: " + $KernelExe + " pid=" + $ready.OwningProcess)
    }
}

$RepoRoot = (Resolve-Path -LiteralPath $RepoRoot).Path
$GodotProjectRoot = (Resolve-Path -LiteralPath $GodotProjectRoot).Path
$WorkspaceDir = Join-Path $RepoRoot "VitApp\Workspace"
$LogsDir = Join-Path $WorkspaceDir "Logs"
$SmokeRoot = Join-Path $WorkspaceDir "Artifacts\smoke"
$Stamp = Get-Date -Format "yyyyMMdd_HHmmss"
$ArtifactDir = Join-Path $SmokeRoot ("blind_mix_conversation_" + $Stamp)
New-Item -ItemType Directory -Path $ArtifactDir -Force | Out-Null
New-Item -ItemType Directory -Path $LogsDir -Force | Out-Null

$AgentLog = Join-Path $LogsDir "agent_last.log"
$DevSmoke = Join-Path $RepoRoot "scripts\dev_agent_smoke.ps1"
$KernelExe = Resolve-KernelExe -RepoRoot $RepoRoot
if ([string]::IsNullOrWhiteSpace($Track1Path)) {
    $Track1Path = Join-Path $RepoRoot "test_100hz_10s.wav"
}
if ([string]::IsNullOrWhiteSpace($Track2Path)) {
    $Track2Path = Join-Path $RepoRoot "test_target_3s.wav"
}
$Track1Path = (Resolve-Path -LiteralPath $Track1Path).Path
$Track2Path = (Resolve-Path -LiteralPath $Track2Path).Path

$summary = [ordered]@{
    schema_version = "blind_mix_conversation_smoke.v0"
    created_at = (Get-Date).ToString("o")
    repo_root = $RepoRoot
    godot_project_root = $GodotProjectRoot
    artifact_dir = $ArtifactDir
    conversation_id = $null
    turns = @()
    observations = @()
    pending_candidates = @()
    treatment_pending_candidates = @()
    tool_route = @()
    stop_reasons = [ordered]@{}
    blocked_mutation_attempts = @()
    resolver_decisions = @()
    plugin_prep_audit = @()
    fixture = [ordered]@{}
    status = "running"
}

try {
    Write-Step "Blind mix conversation smoke"
    Write-Host ("repo: " + $RepoRoot)
    Write-Host ("artifact_dir: " + $ArtifactDir)
    Write-Host ("godot_project: " + $GodotProjectRoot)
    Write-Host ("reuse_godot: " + [string][bool]$ReuseGodot)

    if (-not (Test-Path -LiteralPath $DevSmoke)) {
        Fail ("Missing dev smoke script: " + $DevSmoke)
    }

    Write-Step "Prepare agent and kernel"
    $smokeArgs = @{
        RepoRoot = $RepoRoot
        AgentHttp = $AgentHttp
        AgentHttpAddr = $AgentHttpAddr
        ZmqReqPort = $ZmqReqPort
        ZmqSubPort = $ZmqSubPort
        NoChatSmoke = $true
        WaitSeconds = [Math]::Min($TimeoutSeconds, 60)
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

    Start-KernelIfRequested -KernelExe $KernelExe -ReqPort ([int]$ZmqReqPort) -SubPort ([int]$ZmqSubPort)
    $req = Get-TcpListener -Port ([int]$ZmqReqPort)
    if ($null -eq $req) {
        Fail ("Kernel command port is not listening: " + $ZmqReqPort + ". Start the Godot product path or rerun with -StartKernel.")
    }
    Write-Ok ("kernel command port listening: " + $ZmqReqPort + " pid=" + $req.OwningProcess)

    $state = Invoke-Json -Method GET -Uri ($AgentHttp.TrimEnd("/") + "/agent/state") -TimeoutSec 10
    ConvertTo-JsonFile -Value $state -Path (Join-Path $ArtifactDir "agent_state.json")
    if ($null -eq $state -or [string]$state.status -ne "ok") {
        Fail "GET /agent/state did not return ok"
    }

    Write-Step "Create two-track blind fixture"
    Reset-FixtureProject
    $track1 = Invoke-AgentTool -Tool "track.add_audio" -ToolArgs @{ name = "Track 1" } -Confirmed $true
    Assert-StatusOk -Response $track1 -Label "track.add_audio Track 1"
    $track1ID = Resolve-TrackID -Response $track1
    $track2 = Invoke-AgentTool -Tool "track.add_audio" -ToolArgs @{ name = "Track 2" } -Confirmed $true
    Assert-StatusOk -Response $track2 -Label "track.add_audio Track 2"
    $track2ID = Resolve-TrackID -Response $track2
    if ([string]::IsNullOrWhiteSpace($track1ID) -or [string]::IsNullOrWhiteSpace($track2ID)) {
        Fail ("Could not resolve fixture track IDs: " + $track1ID + " / " + $track2ID)
    }
    Assert-StatusOk -Response (Import-AudioFixture -TrackID $track1ID -FilePath $Track1Path -Label "Track 1 import") -Label "import Track 1"
    Assert-StatusOk -Response (Import-AudioFixture -TrackID $track2ID -FilePath $Track2Path -Label "Track 2 import") -Label "import Track 2"
    Start-Sleep -Milliseconds 750
    $summary["fixture"] = [ordered]@{
        track1_id = $track1ID
        track2_id = $track2ID
        track1_path = $Track1Path
        track2_path = $Track2Path
    }
    ConvertTo-JsonFile -Value $summary["fixture"] -Path (Join-Path $ArtifactDir "fixture.json")

    $confirmMessage = Join-UnicodeChars @(0x53EF, 0x4EE5, 0xFF0C, 0x7EE7, 0x7EED)
    $whyMessage = Join-UnicodeChars @(0x4E3A, 0x4EC0, 0x4E48, 0x8981, 0x8FD9, 0x4E48, 0x52A8, 0xFF1F)
    $reobserveMessage = Join-UnicodeChars @(0x91CD, 0x65B0, 0x89C2, 0x5BDF, 0x4E00, 0x4E0B)
    $overallMessage = Join-UnicodeChars @(0x5E2E, 0x6211, 0x770B, 0x6574, 0x4F53, 0x6DF7, 0x97F3)
    $messyMessage = Join-UnicodeChars @(0x6574, 0x4F53, 0x542C, 0x8D77, 0x6765, 0x6709, 0x70B9, 0x4E71, 0xFF0C, 0x5E2E, 0x6211, 0x5904, 0x7406, 0x4E0B)
    $vocalForwardMessage = Join-UnicodeChars @(0x4E3B, 0x5531, 0x80FD, 0x4E0D, 0x80FD, 0x66F4, 0x9760, 0x524D)
    $lowMudMessage = "Track 1 " + (Join-UnicodeChars @(0x4F4E, 0x9891, 0x6709, 0x70B9, 0x7CCA, 0xFF0C, 0x522B, 0x76F4, 0x63A5, 0x8C03, 0x97F3, 0x91CF, 0xFF0C, 0x7528)) + " EQ " + (Join-UnicodeChars @(0x63D2, 0x4EF6, 0x505A, 0x4F4E, 0x5207, 0x548C, 0x4F4E, 0x4E2D, 0x9891, 0x5904, 0x7406, 0xFF0C, 0x5148, 0x51C6, 0x5907, 0x63D2, 0x4EF6, 0x53C2, 0x6570))
    $panLeftMessage = "Track 2 " + (Join-UnicodeChars @(0x58F0, 0x50CF, 0x5F80, 0x5DE6, 0x4E00, 0x70B9))
    $vocalAnswerMessage = "Track 1 " + (Join-UnicodeChars @(0x662F, 0x4E3B, 0x5531))

    Write-Step "Scenario A/B/D/E: natural request, discuss, reobserve, confirm"
    $conversationID = "blind_mix_main_" + $Stamp
    $summary["conversation_id"] = $conversationID
	$first = Invoke-BlindTurn -ConversationID $conversationID -Message $overallMessage -Label "overall_observe"
	Record-Turn -Turn $first.turn
	Assert-ObserveOnlyFirstTurn -Turn $first.turn -Label "overall observe"
	Assert-TurnAcousticBridgeReadiness -TurnResult $first -Label "overall observe"
	$eventsFirst = Record-Events -ConversationID $conversationID -Label "overall_after_observe"
    $firstPending = @(Extract-PendingCandidates -Events $eventsFirst)
    $firstGainTreatments = @(Extract-GainTreatmentCandidates -Events $eventsFirst)
    $firstDeepIncomplete = Test-TurnResultAcousticPackageDeepIncomplete -TurnResult $first
    if ($firstPending.Count -lt 1 -and $firstGainTreatments.Count -lt 1) {
        if ($firstDeepIncomplete) {
            Assert-NoExecutionQuestion -Turn $first.turn -Label "overall observe"
            $summary["overall_observe_pending_skipped_reason"] = "l3_deep_not_ready"
            Write-Ok "overall observe stayed read-only while L3 acoustic package was incomplete"
        }
        else {
            Fail "overall observe did not produce a pending gain candidate or gain treatment"
        }
    }

    $why = Invoke-BlindTurn -ConversationID $conversationID -Message $whyMessage -Label "why_guard"
    Record-Turn -Turn $why.turn
    Assert-NoMutationRoute -Rows $why.rows -Label "why guard"

    $reobserve = Invoke-BlindTurn -ConversationID $conversationID -Message $reobserveMessage -Label "reobserve_guard"
    Record-Turn -Turn $reobserve.turn
    Assert-NoMutationRoute -Rows $reobserve.rows -Label "reobserve guard"
    if ([int]$reobserve.turn.tool_counts.observe -lt 1) {
        Fail ("reobserve guard did not run mix.observe. route=" + ($reobserve.turn.tool_route -join " -> "))
    }
    [void](Record-Events -ConversationID $conversationID -Label "overall_after_reobserve")

    $afterReobserveConfirm = Invoke-BlindTurn -ConversationID $conversationID -Message $confirmMessage -Label "confirm_after_reobserve"
    Record-Turn -Turn $afterReobserveConfirm.turn
    if (@("mix_tick_applied_reobserved", "no_pending_mix_tick_candidate") -notcontains [string]$afterReobserveConfirm.turn.stop_reason) {
        Fail ("confirm after reobserve stop_reason=" + [string]$afterReobserveConfirm.turn.stop_reason)
    }
    if ([string]$afterReobserveConfirm.turn.stop_reason -eq "mix_tick_applied_reobserved") {
        Assert-ConfirmationRoute -Turn $afterReobserveConfirm.turn
    }
    else {
        Assert-NoPendingConfirmationGuard -Turn $afterReobserveConfirm.turn
    }

    Write-Step "Scenario B: explicit confirmation executes typed gain tick"
    $confirmConversationID = "blind_mix_confirm_" + $Stamp
    $pendingTurn = Invoke-BlindTurn -ConversationID $confirmConversationID -Message $overallMessage -Label "confirm_observe"
    Record-Turn -Turn $pendingTurn.turn
    Assert-ObserveOnlyFirstTurn -Turn $pendingTurn.turn -Label "confirm observe"
    $pendingEvents = Record-Events -ConversationID $confirmConversationID -Label "confirm_after_observe"
    $pendingDeepIncomplete = Test-TurnResultAcousticPackageDeepIncomplete -TurnResult $pendingTurn
    $pendingHasGain = (@(Extract-PendingCandidates -Events $pendingEvents).Count -ge 1 -or @(Extract-GainTreatmentCandidates -Events $pendingEvents).Count -ge 1)
    if (-not $pendingHasGain) {
        if ($pendingDeepIncomplete) {
            Assert-NoExecutionQuestion -Turn $pendingTurn.turn -Label "confirm observe"
            $summary["confirm_gain_tick_skipped_reason"] = "l3_deep_not_ready"
            Write-Ok "confirm observe stayed read-only while L3 acoustic package was incomplete"
        }
        else {
            Fail "confirm observe did not produce a pending gain candidate or gain treatment"
        }
    }
    if ($pendingHasGain) {
        $confirmTurn = Invoke-BlindTurn -ConversationID $confirmConversationID -Message $confirmMessage -Label "confirm_execute"
        Record-Turn -Turn $confirmTurn.turn
        Assert-ConfirmationRoute -Turn $confirmTurn.turn
        [void](Record-Events -ConversationID $confirmConversationID -Label "confirm_after_execute")
    }

    Write-Step "Scenario natural blind phrase stays observe-first"
    $messyConversationID = "blind_mix_messy_" + $Stamp
    $messy = Invoke-BlindTurn -ConversationID $messyConversationID -Message $messyMessage -Label "messy_observe"
    Record-Turn -Turn $messy.turn
    Assert-ObserveOnlyFirstTurn -Turn $messy.turn -Label "messy observe"
    [void](Record-Events -ConversationID $messyConversationID -Label "messy_after_observe")

    Write-Step "Scenario C: no pending confirmation"
    $noPendingConversationID = "blind_mix_no_pending_" + $Stamp
    $noPending = Invoke-BlindTurn -ConversationID $noPendingConversationID -Message $confirmMessage -Label "no_pending_confirm"
    Record-Turn -Turn $noPending.turn
    Assert-NoPendingConfirmationGuard -Turn $noPending.turn

    Write-Step "Scenario F: ambiguous vocal asks, answer creates pending"
    $vocalConversationID = "blind_mix_vocal_" + $Stamp
    $vocalAsk = Invoke-BlindTurn -ConversationID $vocalConversationID -Message $vocalForwardMessage -Label "vocal_ask"
    Record-Turn -Turn $vocalAsk.turn
    if ([string]$vocalAsk.turn.stop_reason -ne "needs_clarification") {
        Fail ("vocal ask stop_reason=" + [string]$vocalAsk.turn.stop_reason)
    }
    Assert-NoMutationRoute -Rows $vocalAsk.rows -Label "vocal ask"
    $vocalAskEvents = Record-Events -ConversationID $vocalConversationID -Label "vocal_after_ask"
    if (@(Extract-PendingCandidates -Events $vocalAskEvents).Count -ne 0) {
        Fail "vocal ask stored pending before target was clarified"
    }

    $vocalAnswer = Invoke-BlindTurn -ConversationID $vocalConversationID -Message $vocalAnswerMessage -Label "vocal_answer"
    Record-Turn -Turn $vocalAnswer.turn
    if ([string]$vocalAnswer.turn.stop_reason -ne "done") {
        Fail ("vocal answer stop_reason=" + [string]$vocalAnswer.turn.stop_reason)
    }
    $vocalAnswerEvents = Record-Events -ConversationID $vocalConversationID -Label "vocal_after_answer"
    if (@(Extract-PendingCandidates -Events $vocalAnswerEvents).Count -lt 1 -and @(Extract-GainTreatmentCandidates -Events $vocalAnswerEvents).Count -lt 1) {
        Fail "vocal answer did not produce a pending gain candidate or gain treatment"
    }
    $vocalConfirm = Invoke-BlindTurn -ConversationID $vocalConversationID -Message $confirmMessage -Label "vocal_confirm"
    Record-Turn -Turn $vocalConfirm.turn
    Assert-ConfirmationRoute -Turn $vocalConfirm.turn

    Write-Step "Scenario pan treatment confirms through typed pan tick"
    $panConversationID = "blind_mix_pan_" + $Stamp
	$panObserve = Invoke-BlindTurn -ConversationID $panConversationID -Message $panLeftMessage -Label "pan_observe"
	Record-Turn -Turn $panObserve.turn
	Assert-ObserveOnlyFirstTurn -Turn $panObserve.turn -Label "pan observe"
	Assert-TurnAcousticBridgeReadiness -TurnResult $panObserve -Label "pan observe"
	$panEvents = Record-Events -ConversationID $panConversationID -Label "pan_after_observe"
    $panTreatments = @(Extract-PanTreatmentCandidates -Events $panEvents)
    if ($panTreatments.Count -lt 1) {
        Fail "pan observe did not produce pan_balance treatment"
    }
    [void](Assert-DiagnosisContextPayload -Payload $panTreatments[-1] -Label "pan treatment pending" -ExpectedProblemKind "pan_balance" -ExpectedStrategy "small_pan_adjust")
    $panConfirm = Invoke-BlindTurn -ConversationID $panConversationID -Message $confirmMessage -Label "pan_confirm"
    Record-Turn -Turn $panConfirm.turn
    Assert-ConfirmationRoute -Turn $panConfirm.turn
    $panResolveEvents = Record-Events -ConversationID $panConversationID -Label "pan_after_confirm"
    $panDecisions = @(Extract-ResolverDecisions -Events $panResolveEvents)
    if ($panDecisions.Count -lt 1) {
        Fail "pan treatment confirmation did not emit resolver decision"
    }
    $panStatus = [string](Get-OptionalProperty -Object $panDecisions[-1] -Name "status")
    if ($panStatus -ne "ready_pan_tick") {
        Fail ("unexpected pan resolver status=" + $panStatus)
    }

    Write-Step "Scenario broad/plugin-like requests stay observe-first"
    $lowConversationID = "blind_mix_low_" + $Stamp
	$low = Invoke-BlindTurn -ConversationID $lowConversationID -Message $lowMudMessage -Label "low_mud_observe"
	Record-Turn -Turn $low.turn
	Assert-ObserveOnlyFirstTurn -Turn $low.turn -Label "low mud observe"
	Assert-TurnAcousticBridgeReadiness -TurnResult $low -Label "low mud observe"
	$lowEvents = Record-Events -ConversationID $lowConversationID -Label "low_mud_after_observe"
    $lowTreatments = @(Extract-TreatmentPendingCandidates -Events $lowEvents)
    if ($lowTreatments.Count -lt 1) {
        Fail "low mud observe did not produce mix_treatment_pending"
    }
    [void](Assert-LowMudAcousticEvidence -Payload $lowTreatments[-1] -Label "low mud treatment pending" -AllowReady)
    $lowConfirm = Invoke-BlindTurn -ConversationID $lowConversationID -Message $confirmMessage -Label "low_mud_treatment_confirm"
    Record-Turn -Turn $lowConfirm.turn
    $lowResolveEvents = Record-Events -ConversationID $lowConversationID -Label "low_mud_after_treatment_confirm"
    $lowDecisions = @(Extract-ResolverDecisions -Events $lowResolveEvents)
    if ($lowDecisions.Count -lt 1) {
        Fail "low mud treatment confirmation did not emit resolver decision"
    }
    $latestDecision = $lowDecisions[-1]
    $decisionStatus = [string](Get-OptionalProperty -Object $latestDecision -Name "status")
    if ($decisionStatus -notin @("needs_preparation", "needs_clarification", "observation_only", "ready_gain_tick", "ready_pan_tick", "ready_plugin_control")) {
        Fail ("unexpected low mud resolver status=" + $decisionStatus)
    }
    $decisionRoute = @((Get-OptionalProperty -Object $latestDecision -Name "tool_route"))
    if ($decisionStatus -in @("needs_preparation", "needs_clarification", "observation_only")) {
        Assert-NoMutationRoute -Rows $lowConfirm.rows -Label "low mud treatment resolver"
        if ([string]$lowConfirm.turn.stop_reason -notmatch "^mix_treatment_resolved_" -and [string]$lowConfirm.turn.stop_reason -ne "mix_treatment_preparation_started") {
            Fail ("low mud treatment confirm stop_reason=" + [string]$lowConfirm.turn.stop_reason)
        }
    }
    if ($decisionStatus -in @("ready_gain_tick", "ready_pan_tick")) {
        Assert-ConfirmationRoute -Turn $lowConfirm.turn
    }
    if ($decisionStatus -eq "needs_preparation" -and ($decisionRoute -notcontains "plugin_grabber.apply_control")) {
        Fail ("low mud resolver did not preserve plugin_grabber.apply_control route. route=" + ($decisionRoute -join " -> "))
    }
    if ($decisionStatus -eq "needs_preparation") {
        Assert-PluginPreparationPlan -Decision $latestDecision
    }
    if ([string]$lowConfirm.turn.stop_reason -eq "mix_treatment_preparation_started") {
        Assert-PluginResponsePreparationPlan -Response $lowConfirm.response
        Assert-PluginPrepConfirmationGate -TurnResult $lowConfirm -Label "low mud preparation card"
        $lowPrepInteractionID = Get-PluginPreparationInteractionID -Response $lowConfirm.response
        $lowPrepConfirm = Invoke-BlindInteraction -ConversationID $lowConversationID -InteractionID $lowPrepInteractionID -Label "low_mud_plugin_prep_confirm" -Decision "approve" -ActionID "approve"
        Record-Turn -Turn $lowPrepConfirm.turn
        Assert-PluginPrepConfirmationGate -TurnResult $lowPrepConfirm -Label "low mud plugin prep confirm" -RequirePreparationExecution
        $lowParameterCandidateInteractionID = Get-PluginParameterTreatmentInteractionID -Response $lowPrepConfirm.response
        $lowParameterConfirm = Invoke-BlindInteraction -ConversationID $lowConversationID -InteractionID $lowParameterCandidateInteractionID -Label "low_mud_parameter_candidate_confirm" -Decision "approve" -ActionID "approve"
        Record-Turn -Turn $lowParameterConfirm.turn
        Assert-PluginPrepWorkerAppliedRoute -TurnResult $lowParameterConfirm -Label "low mud parameter candidate confirm"
    }
    if ($decisionStatus -eq "ready_plugin_control") {
        if ([string]$lowConfirm.turn.stop_reason -ne "mix_treatment_applied_plugin_control_reobserved") {
            Fail ("ready plugin control did not reobserve after apply. stop_reason=" + [string]$lowConfirm.turn.stop_reason)
        }
        Assert-ToolPresent -Tools $lowConfirm.turn.tool_route -Aliases @("plugin_grabber.apply_control", "plugin_grabber_apply_control") -Label "plugin treatment apply_control"
        Assert-ToolPresent -Tools $lowConfirm.turn.tool_route -Aliases @("mix.observe", "mix_observe", "mix.request_observation", "mix_request_observation") -Label "plugin treatment reobserve"
    }

    $summary["status"] = "passed"
    Write-Ok "blind mix conversation smoke passed"
}
catch {
    $summary["status"] = "failed"
    $summary["error"] = $_.Exception.Message
    Write-WarnLine ("failed: " + $_.Exception.Message)
    throw
}
finally {
    if (Test-Path -LiteralPath $AgentLog) {
        Get-Content -LiteralPath $AgentLog -Tail 240 -ErrorAction SilentlyContinue |
            Set-Content -LiteralPath (Join-Path $ArtifactDir "agent_log_tail.txt") -Encoding UTF8
    }
    ConvertTo-JsonFile -Value $summary -Path (Join-Path $ArtifactDir "summary.json")
}

Write-Step "Summary"
Write-Host ("artifact_dir: " + $ArtifactDir)
Write-Host ("conversation_id: " + [string]$summary["conversation_id"])
Write-Host ("turn_count: " + [string]$summary["turns"].Count)
Write-Host ("pending_candidate_count: " + [string]$summary["pending_candidates"].Count)
Write-Host ("treatment_pending_count: " + [string]$summary["treatment_pending_candidates"].Count)
Write-Host ("resolver_decision_count: " + [string]$summary["resolver_decisions"].Count)
Write-Ok "blind mix conversation smoke finished"
