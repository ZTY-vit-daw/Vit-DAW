[CmdletBinding()]
param(
    [string]$RepoRoot = "D:\Vit_DAW",
    [string]$AgentHttp = "http://127.0.0.1:7878",
    [Parameter(Mandatory = $true)][string]$ArtifactDir,
    [string]$TDRNovaPath = "C:\Program Files\Common Files\VST3\TDR Nova.vst3",
    [int]$TimeoutSeconds = 300
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Fail {
    param([string]$Message)
    throw $Message
}

function Get-OptionalProperty {
    param([object]$Object, [string]$Name)
    if ($null -eq $Object -or [string]::IsNullOrWhiteSpace($Name)) {
        return $null
    }
    if ($Object -is [System.Collections.IDictionary] -and $Object.Contains($Name)) {
        return $Object[$Name]
    }
    $property = $Object.PSObject.Properties[$Name]
    if ($null -eq $property) {
        return $null
    }
    return $property.Value
}

function Write-JsonArtifact {
    param([string]$Name, [object]$Value)
    $path = Join-Path $ArtifactDir $Name
    $Value | ConvertTo-Json -Depth 32 | Set-Content -LiteralPath $path -Encoding UTF8
    return $path
}

function Invoke-Json {
    param(
        [ValidateSet("GET", "POST")][string]$Method,
        [string]$Uri,
        [object]$Body = $null,
        [int]$TimeoutSec = 120
    )
    if ($Method -eq "GET") {
        $response = Invoke-WebRequest -UseBasicParsing -Method GET -Uri $Uri -TimeoutSec $TimeoutSec
    }
    else {
        $json = $Body | ConvertTo-Json -Depth 32 -Compress
        $bytes = [System.Text.Encoding]::UTF8.GetBytes($json)
        $response = Invoke-WebRequest -UseBasicParsing -Method POST -Uri $Uri -Body $bytes -ContentType "application/json; charset=utf-8" -TimeoutSec $TimeoutSec
    }
    if ([string]::IsNullOrWhiteSpace($response.Content)) {
        return $null
    }
    return $response.Content | ConvertFrom-Json
}

function Invoke-AgentTool {
    param(
        [string]$Tool,
        [hashtable]$ToolArgs = @{},
        [bool]$Confirmed = $true
    )
    return Invoke-Json -Method POST -Uri ($AgentHttp.TrimEnd("/") + "/agent/invoke") -Body @{
        tool = $Tool
        args = $ToolArgs
        confirmed = $Confirmed
        source = "spal_reference_eq_agent_smoke"
    } -TimeoutSec $TimeoutSeconds
}

function Invoke-AgentChat {
    param(
        [string]$ConversationID,
        [string]$Message,
        [hashtable]$Context = @{}
    )
    $fullContext = @{
        agent_mode = "chat"
        interaction_path = "spal_reference_eq_after_godot_lifecycle"
        product_path_smoke = $true
        product_lifecycle = "godot_project"
        spal_reference_eq_smoke = $true
    }
    foreach ($key in $Context.Keys) {
        $fullContext[$key] = $Context[$key]
    }
    return Invoke-Json -Method POST -Uri ($AgentHttp.TrimEnd("/") + "/agent/chat") -Body @{
        conversation_id = $ConversationID
        message = $Message
        context = $fullContext
    } -TimeoutSec $TimeoutSeconds
}

# Plugin Learning is a user-facing, staged workflow.  The smoke must use the
# same interaction endpoint as the Godot UI rather than making a second tool
# call with invented arguments.  This keeps the artifact trail useful when a
# new learning card is introduced.
function Invoke-AgentInteraction {
    param(
        [Parameter(Mandatory = $true)][string]$InteractionID,
        [Parameter(Mandatory = $true)][string]$Decision,
        [hashtable]$Payload = @{}
    )
    return Invoke-Json -Method POST -Uri ($AgentHttp.TrimEnd("/") + "/agent/interaction/respond") -Body @{
        interaction_id = $InteractionID
        decision = $Decision
        action_id = $Decision
        payload = $Payload
    } -TimeoutSec $TimeoutSeconds
}

function Assert-StatusOk {
    param([object]$Response, [string]$Label)
    if ([string](Get-OptionalProperty -Object $Response -Name "status") -ne "ok") {
        Fail ($Label + " failed: " + ($Response | ConvertTo-Json -Depth 24 -Compress))
    }
}

function Resolve-ResultID {
    param([object]$Response, [string[]]$Keys)
    $result = Get-OptionalProperty -Object $Response -Name "result"
    foreach ($key in $Keys) {
        $value = [string](Get-OptionalProperty -Object $result -Name $key)
        if (-not [string]::IsNullOrWhiteSpace($value)) {
            return $value
        }
    }
    foreach ($key in $Keys) {
        $value = [string](Get-OptionalProperty -Object $Response -Name $key)
        if (-not [string]::IsNullOrWhiteSpace($value)) {
            return $value
        }
    }
    return ""
}

function Resolve-TrackID {
    param([object]$Response)
    return Resolve-ResultID -Response $Response -Keys @("track_id", "id", "item_id")
}

function Resolve-ClipID {
    param([object]$Response)
    return Resolve-ResultID -Response $Response -Keys @("clip_id", "id", "item_id")
}

function Resolve-PluginID {
    param([object]$Response)
    return Resolve-ResultID -Response $Response -Keys @("plugin_id", "plugin_item_id", "node_id", "item_id", "id")
}

function WorkflowValue {
    param([object]$Response, [string]$Name)
    $workflow = Get-OptionalProperty -Object $Response -Name "workflow_data"
    return Get-OptionalProperty -Object $workflow -Name $Name
}

function ResponseValue {
    param([object]$Response, [string]$Name)
    $value = Get-OptionalProperty -Object $Response -Name $Name
    if ($null -ne $value) {
        return $value
    }
    $result = Get-OptionalProperty -Object $Response -Name "result"
    return Get-OptionalProperty -Object $result -Name $Name
}

function InteractionRequestsFromResponse {
    param([object]$Response)
    # Most interaction responses expose the cards at the ChatResponse level.
    # An Agent Loop turn, however, preserves the kernel receipt verbatim and
    # therefore keeps its cards under executed_kernel_reply[].result.  Treat
    # both envelopes as the same user-facing interaction stream so the smoke
    # follows precisely the cards a Godot client would render.
    $requests = [System.Collections.ArrayList]::new()
    $seenIDs = @{}

    function Add-InteractionRequestRows {
        param([object]$Rows)
        foreach ($row in @($Rows)) {
            if ($null -eq $row) {
                continue
            }
            $id = [string](Get-OptionalProperty -Object $row -Name "id")
            if (-not [string]::IsNullOrWhiteSpace($id)) {
                if ($seenIDs.ContainsKey($id)) {
                    continue
                }
                $seenIDs[$id] = $true
            }
            [void]$requests.Add($row)
        }
    }

    Add-InteractionRequestRows -Rows (ResponseValue -Response $Response -Name "interaction_requests")
    foreach ($executed in @(Get-OptionalProperty -Object $Response -Name "executed_kernel_reply")) {
        Add-InteractionRequestRows -Rows (Get-OptionalProperty -Object $executed -Name "interaction_requests")
        $result = Get-OptionalProperty -Object $executed -Name "result"
        Add-InteractionRequestRows -Rows (Get-OptionalProperty -Object $result -Name "interaction_requests")
    }
    return $requests.ToArray()
}

function Assert-InteractionAction {
    param([object]$Interaction, [string]$ActionID, [string]$Label)
    foreach ($action in @(Get-OptionalProperty -Object $Interaction -Name "actions")) {
        if ([string](Get-OptionalProperty -Object $action -Name "id") -eq $ActionID) {
            return
        }
    }
    Fail ($Label + " did not offer action '" + $ActionID + "': " + ($Interaction | ConvertTo-Json -Depth 24 -Compress))
}

function Default-PluginLearningDisplayDomainFields {
    param([object]$Interaction)
    $fields = @{}
    foreach ($field in @(Get-OptionalProperty -Object $Interaction -Name "fields")) {
        $fieldID = [string](Get-OptionalProperty -Object $field -Name "id")
        $fieldValue = [string](Get-OptionalProperty -Object $field -Name "value")
        if (-not [string]::IsNullOrWhiteSpace($fieldID) -and -not [string]::IsNullOrWhiteSpace($fieldValue)) {
            # The value is supplied by the Plugin Learning review card from
            # observed/probed evidence.  The smoke accepts that displayed
            # proposal through the normal form action; it never manufactures
            # an arbitrary range or parameter mapping.
            $fields[$fieldID] = $fieldValue
        }
    }
    return $fields
}

function Assert-NeedsConfirmation {
    param([object]$Response, [string]$Label)
    if (-not [bool](Get-OptionalProperty -Object $Response -Name "needs_confirmation")) {
        Fail ($Label + " did not create a confirmation Proposal: " + ($Response | ConvertTo-Json -Depth 28 -Compress))
    }
}

function Invoke-PluginLearningIfRequired {
    param(
        [string]$TrackID,
        [string]$PluginID,
        [string]$ConversationID,
        [hashtable]$Context
    )
    # This must enter through the same natural-language chat route as Godot's
    # Ask Vit UI. The smoke is allowed to accept the resulting formal learning
    # cards, but it never invokes the learning tool directly.
    $learningMessage = "学习当前选中的 TDR Nova，并建立 VPS v3 静态 Bell EQ Credential。"
    $response = Invoke-AgentChat -ConversationID $ConversationID -Message $learningMessage -Context $Context
    Write-JsonArtifact -Name "plugin_learning_natural_language_start.json" -Value $response | Out-Null
    $interactionIndex = 0
    while ($true) {
        $requests = @(InteractionRequestsFromResponse -Response $response)
        if ($requests.Count -eq 0) {
            break
        }
        if ($requests.Count -ne 1) {
            Fail ("Plugin Learning returned an ambiguous interaction set: " + ($response | ConvertTo-Json -Depth 28 -Compress))
        }
        $interaction = $requests[0]
        $interactionID = [string](Get-OptionalProperty -Object $interaction -Name "id")
        $interactionType = [string](Get-OptionalProperty -Object $interaction -Name "type")
        if ([string]::IsNullOrWhiteSpace($interactionID) -or [string]::IsNullOrWhiteSpace($interactionType)) {
            Fail ("Plugin Learning returned an invalid interaction: " + ($interaction | ConvertTo-Json -Depth 28 -Compress))
        }
        $interactionIndex++
        $decision = ""
        $payload = @{}
        switch -Regex ($interactionType) {
            "plugin_learning_ui_reference_request" {
                Assert-InteractionAction -Interaction $interaction -ActionID "skip_ui_reference" -Label "Plugin Learning UI reference"
                $decision = "skip_ui_reference"
            }
            "plugin_learning_candidate_review" {
                # No human visual observation is available to a product smoke.
                # Use the card's explicit skip path rather than claiming that
                # a temporary parameter experiment was observed.
                Assert-InteractionAction -Interaction $interaction -ActionID "skip_experiments" -Label "Plugin Learning candidate review"
                $decision = "skip_experiments"
            }
            "plugin_learning_display_domain_form" {
                Assert-InteractionAction -Interaction $interaction -ActionID "submit" -Label "Plugin Learning display-domain review"
                $payload = @{ fields = (Default-PluginLearningDisplayDomainFields -Interaction $interaction) }
                $decision = "submit"
            }
            "plugin_learning_final_review" {
                Assert-InteractionAction -Interaction $interaction -ActionID "approve" -Label "Plugin Learning final review"
                $decision = "approve"
            }
            "plugin_learning_completion" {
                # This is a terminal UI boundary, not another learning write.
                break
            }
            default {
                Fail ("Plugin Learning requires an unsupported interactive step '" + $interactionType + "'; smoke refuses to fabricate an answer: " + ($interaction | ConvertTo-Json -Depth 28 -Compress))
            }
        }
        if ([string]::IsNullOrWhiteSpace($decision)) {
            break
        }
        $response = Invoke-AgentInteraction -InteractionID $interactionID -Decision $decision -Payload $payload
        $artifactName = "plugin_learning_interaction_{0:D2}_{1}.json" -f $interactionIndex, ($interactionType -replace "[^a-zA-Z0-9]+", "_")
        Write-JsonArtifact -Name $artifactName -Value $response | Out-Null
    }

    $needsConfirmation = [bool](ResponseValue -Response $response -Name "requires_confirmation") -or [bool](ResponseValue -Response $response -Name "needs_confirmation")
    if ($needsConfirmation) {
        Fail ("Plugin Learning stopped with an unhandled confirmation instead of a completed formal interaction: " + ($response | ConvertTo-Json -Depth 28 -Compress))
    }
    $vpsV3 = WorkflowValue -Response $response -Name "vps_v3"
    if ($null -eq $vpsV3) {
        $vpsV3 = Get-OptionalProperty -Object $response -Name "vps_v3"
    }
    if ($null -eq $vpsV3 -or [string](Get-OptionalProperty -Object $vpsV3 -Name "credential_status") -ne "verified" -or -not [bool](Get-OptionalProperty -Object $vpsV3 -Name "catalog_visible")) {
        Fail ("Natural-language Plugin Learning did not issue a visible VPS v3 Credential: " + ($response | ConvertTo-Json -Depth 28 -Compress))
    }
    if ([string]::IsNullOrWhiteSpace([string](Get-OptionalProperty -Object $vpsV3 -Name "credential_id"))) {
        Fail ("VPS v3 learning result omitted credential_id: " + ($vpsV3 | ConvertTo-Json -Depth 20 -Compress))
    }
    return $vpsV3
}

function Assert-StructuralPass {
    param([object]$Response, [string]$Label)
    $structural = [string](WorkflowValue -Response $Response -Name "structural_verification")
    if ($structural -ne "pass") {
        Fail ($Label + " did not report structural pass: " + ($Response | ConvertTo-Json -Depth 28 -Compress))
    }
}

function Parameter-DisplayText {
    param([object]$Parameter)
    $text = [string](Get-OptionalProperty -Object $Parameter -Name "value_text")
    if (-not [string]::IsNullOrWhiteSpace($text)) {
        return $text.Trim()
    }
    $probe = Get-OptionalProperty -Object $Parameter -Name "display_probe"
    return ([string](Get-OptionalProperty -Object $probe -Name "current_text")).Trim()
}

function Resolve-VPSCredentialFromSmokeLibrary {
    param([string]$LibraryPath, [string]$VPSID, [string]$CredentialID)
    if (-not (Test-Path -LiteralPath $LibraryPath)) {
        Fail ("VPS Library was not persisted at " + $LibraryPath)
    }
    $library = Get-Content -LiteralPath $LibraryPath -Raw -Encoding UTF8 | ConvertFrom-Json
    $document = @($library.documents | Where-Object { [string]$_.id -eq $VPSID }) | Select-Object -First 1
    if ($null -eq $document) {
        Fail ("Smoke VPS Library did not contain learned VPS " + $VPSID)
    }
    $credential = @($document.provider_credentials | Where-Object { [string]$_.id -eq $CredentialID }) | Select-Object -First 1
    if ($null -eq $credential) {
        Fail ("Smoke VPS Library did not contain learned Credential " + $CredentialID)
    }
    return $credential
}

function Assert-VPSCredentialStillVerified {
    param([string]$LibraryPath, [string]$VPSID, [string]$CredentialID, [string]$Label)
    $credential = Resolve-VPSCredentialFromSmokeLibrary -LibraryPath $LibraryPath -VPSID $VPSID -CredentialID $CredentialID
    if ([string]$credential.status -ne "verified") {
        Fail ($Label + " unexpectedly changed Credential state: " + ($credential | ConvertTo-Json -Depth 24 -Compress))
    }
    if ($null -ne (Get-OptionalProperty -Object $credential -Name "invalidated_at")) {
        Fail ($Label + " unexpectedly invalidated Credential: " + ($credential | ConvertTo-Json -Depth 24 -Compress))
    }
}

New-Item -ItemType Directory -Path $ArtifactDir -Force | Out-Null
$startedAt = (Get-Date).ToUniversalTime().ToString("o")
$summary = [ordered]@{
    schema_version = "spal_reference_eq_agent_smoke.v4"
    status = "failed"
    started_at = $startedAt
    agent_http = $AgentHttp
    product_lifecycle = "godot_project"
    assertions = [ordered]@{}
}
$referenceProfileKey = "plugin_f9788adda4203df8"
$referenceGlobalProfilePath = Join-Path $RepoRoot ("VitApp\Workspace\plugin_grabber_profiles\" + $referenceProfileKey + ".json")
$referenceGlobalProfileReset = $false
$referenceGlobalProfileBeforeLearning = "not_checked"

try {
    if (-not (Test-Path -LiteralPath $TDRNovaPath)) {
        Fail ("TDR Nova reference fixture is not installed at " + $TDRNovaPath)
    }
    $health = Invoke-Json -Method GET -Uri ($AgentHttp.TrimEnd("/") + "/health") -TimeoutSec 15
    Write-JsonArtifact -Name "agent_health.json" -Value $health | Out-Null
    if ($null -eq $health) {
        Fail "VitAgent health endpoint did not return a response"
    }

    $newProject = Invoke-AgentTool -Tool "project.new" -Confirmed $true
    Write-JsonArtifact -Name "project_new.json" -Value $newProject | Out-Null
    $clearProject = Invoke-AgentTool -Tool "project.clear" -Confirmed $true
    Write-JsonArtifact -Name "project_clear.json" -Value $clearProject | Out-Null
    Assert-StatusOk -Response $clearProject -Label "project.clear"

    $track = Invoke-AgentTool -Tool "track.add_audio" -ToolArgs @{ name = "SPAL Reference EQ Smoke Bass" } -Confirmed $true
    Write-JsonArtifact -Name "track_add.json" -Value $track | Out-Null
    Assert-StatusOk -Response $track -Label "track.add_audio"
    $trackID = Resolve-TrackID -Response $track
    if ([string]::IsNullOrWhiteSpace($trackID)) {
        Fail "Could not resolve SPAL smoke track ID"
    }
    $fixtureAudio = Join-Path $RepoRoot "test_100hz_10s.wav"
    if (-not (Test-Path -LiteralPath $fixtureAudio)) {
        Fail ("Missing SPAL smoke audio fixture: " + $fixtureAudio)
    }
    $import = Invoke-AgentTool -Tool "clip.import_media_to_track" -ToolArgs @{
        track_id = $trackID
        file_path = $fixtureAudio
        start_time = 0
        media_type = "audio"
        mode = "non_destructive"
    } -Confirmed $true
    if ([string](Get-OptionalProperty -Object $import -Name "status") -ne "ok") {
        $import = Invoke-AgentTool -Tool "clip.import_audio" -ToolArgs @{ track_id = $trackID; file_path = $fixtureAudio; offset_time = 0 } -Confirmed $true
    }
    Write-JsonArtifact -Name "clip_import.json" -Value $import | Out-Null
    Assert-StatusOk -Response $import -Label "clip import"
    $clipID = Resolve-ClipID -Response $import
    if ([string]::IsNullOrWhiteSpace($clipID)) {
        Fail "Could not resolve SPAL smoke clip ID"
    }

    # Fixture staging is deliberately outside SPAL. The product registration
    # flow below must only observe this already loaded instance, never load it.
    $load = Invoke-AgentTool -Tool "rack.add_node" -ToolArgs @{
        track_id = $trackID
        plugin_path = $TDRNovaPath
        zone_id = "Z3"
        x = 360
        y = 180
    } -Confirmed $true
    Write-JsonArtifact -Name "tdr_nova_fixture_load.json" -Value $load | Out-Null
    Assert-StatusOk -Response $load -Label "load TDR Nova fixture"
    $pluginID = Resolve-PluginID -Response $load
    if ([string]::IsNullOrWhiteSpace($pluginID)) {
        Fail ("Could not resolve TDR Nova fixture plugin ID: " + ($load | ConvertTo-Json -Depth 24 -Compress))
    }

    # This is a clean-room requalification: the old experimental TDR Skill
    # must not influence the natural-language learning flow and is deliberately
    # not restored after the smoke.
    if (Test-Path -LiteralPath $referenceGlobalProfilePath) {
        Remove-Item -LiteralPath $referenceGlobalProfilePath -Force
        $referenceGlobalProfileBeforeLearning = "removed"
    }
    else {
        $referenceGlobalProfileBeforeLearning = "absent"
    }
    if (Test-Path -LiteralPath $referenceGlobalProfilePath) {
        Fail ("Could not remove the experimental TDR Nova Skill document before natural-language learning: " + $referenceGlobalProfilePath)
    }
    $referenceGlobalProfileReset = $true
    $profileReset = Invoke-AgentTool -Tool "plugin_grabber.remove_project_profile" -ToolArgs @{ track_id = $trackID; plugin_id = $pluginID } -Confirmed $true
    Write-JsonArtifact -Name "tdr_nova_profile_reset_for_smoke.json" -Value $profileReset | Out-Null

    $uiContext = @{ selected_track_id = $trackID; selected_plugin_track_id = $trackID; selected_plugin_id = $pluginID; selected_clip_id = $clipID }
    $uiContextResponse = Invoke-Json -Method POST -Uri ($AgentHttp.TrimEnd("/") + "/agent/ui/context") -Body $uiContext -TimeoutSec 20
    Write-JsonArtifact -Name "ui_context.json" -Value $uiContextResponse | Out-Null
    if ([string](Get-OptionalProperty -Object $uiContextResponse -Name "status") -ne "ok") {
        Fail ("Could not publish Godot-style selection context: " + ($uiContextResponse | ConvertTo-Json -Depth 20 -Compress))
    }

    $catalogBefore = Invoke-Json -Method GET -Uri ($AgentHttp.TrimEnd("/") + "/agent/vps/catalog") -TimeoutSec 30
    Write-JsonArtifact -Name "vps_catalog_before_learning.json" -Value $catalogBefore | Out-Null
    if ([string](Get-OptionalProperty -Object $catalogBefore -Name "status") -ne "ok") {
        Fail ("VPS Catalog endpoint was unavailable before learning: " + ($catalogBefore | ConvertTo-Json -Depth 20 -Compress))
    }
    $catalogBeforeEntries = @((Get-OptionalProperty -Object (Get-OptionalProperty -Object $catalogBefore -Name "catalog") -Name "entries"))
    if ($catalogBeforeEntries.Count -ne 0) {
        Fail ("VPS smoke requires a clean Credential Catalog before natural-language learning: " + ($catalogBefore | ConvertTo-Json -Depth 20 -Compress))
    }

    $conversationID = "spal_reference_eq_vps_v3_smoke_" + (Get-Date -Format "yyyyMMdd_HHmmss")
    $vpsLearning = Invoke-PluginLearningIfRequired -TrackID $trackID -PluginID $pluginID -ConversationID $conversationID -Context $uiContext
    Write-JsonArtifact -Name "vps_v3_learning_result.json" -Value $vpsLearning | Out-Null
    $credentialID = [string](Get-OptionalProperty -Object $vpsLearning -Name "credential_id")
    $vpsID = [string](Get-OptionalProperty -Object $vpsLearning -Name "vps_id")

    $catalogAfter = Invoke-Json -Method GET -Uri ($AgentHttp.TrimEnd("/") + "/agent/vps/catalog") -TimeoutSec 30
    Write-JsonArtifact -Name "vps_catalog_after_learning.json" -Value $catalogAfter | Out-Null
    $catalogAfterEntries = @((Get-OptionalProperty -Object (Get-OptionalProperty -Object $catalogAfter -Name "catalog") -Name "entries"))
    $catalogCredential = @($catalogAfterEntries | Where-Object { [string]$_.credential_id -eq $credentialID }) | Select-Object -First 1
    if ($null -eq $catalogCredential -or [string]$catalogCredential.vps_id -ne $vpsID -or [string]$catalogCredential.capability_id -ne "spectral.static_eq.v0") {
        Fail ("Natural-language learning Credential was not derived into the verified VPS Catalog: " + ($catalogAfter | ConvertTo-Json -Depth 24 -Compress))
    }

    # Regression: changing an EQ band's current shape is a runtime
    # precondition, not a change to the plug-in's control surface.  This
    # exercises the original field failure: Bell -> Low S must leave the
    # newly issued Credential visible and recoverable in this same chat.
    $smokeLibraryPath = Join-Path $ArtifactDir "vps_library_v3.json"
    $learnedCredential = Resolve-VPSCredentialFromSmokeLibrary -LibraryPath $smokeLibraryPath -VPSID $vpsID -CredentialID $credentialID
    $staticEQBinding = Get-OptionalProperty -Object (Get-OptionalProperty -Object $learnedCredential -Name "conformance") -Name "static_eq_binding"
    $filterTypeParameterID = [string](Get-OptionalProperty -Object $staticEQBinding -Name "filter_type_parameter_id")
    if ([string]::IsNullOrWhiteSpace($filterTypeParameterID)) {
        Fail ("Learned Credential omitted its conformed filter-type binding: " + ($learnedCredential | ConvertTo-Json -Depth 24 -Compress))
    }
    $typeBefore = Invoke-AgentTool -Tool "plugin.get_parameters" -ToolArgs @{ track_id = $trackID; plugin_id = $pluginID; include_vps_v3_surface = $true } -Confirmed $false
    Write-JsonArtifact -Name "filter_type_before_shape_regression.json" -Value $typeBefore | Out-Null
    Assert-StatusOk -Response $typeBefore -Label "read conformed filter type before shape regression"
    $typeParameter = @(ResponseValue -Response $typeBefore -Name "parameters" | Where-Object { [string](Get-OptionalProperty -Object $_ -Name "id") -eq $filterTypeParameterID }) | Select-Object -First 1
    if ($null -eq $typeParameter) {
        Fail ("Fresh parameter surface did not expose the conformed filter type " + $filterTypeParameterID)
    }
    $bellText = Parameter-DisplayText -Parameter $typeParameter
    $bellNormalizedValue = Get-OptionalProperty -Object $typeParameter -Name "normalized_value"
    if ($bellText -notmatch "(?i)bell" -or $null -eq $bellNormalizedValue) {
        Fail ("Learning did not leave the conformed filter in Bell with a normalized readback: " + ($typeParameter | ConvertTo-Json -Depth 20 -Compress))
    }
    $typeProbe = Get-OptionalProperty -Object $typeParameter -Name "display_probe"
    $nonBellSample = @((Get-OptionalProperty -Object $typeProbe -Name "samples") | Where-Object {
            $sampleText = [string](Get-OptionalProperty -Object $_ -Name "text")
            -not [string]::IsNullOrWhiteSpace($sampleText) -and $sampleText -notmatch "(?i)bell" -and $null -ne (Get-OptionalProperty -Object $_ -Name "normalized_value")
        }) | Select-Object -First 1
    if ($null -eq $nonBellSample) {
        Fail ("TDR Nova filter-type display probe did not expose a non-Bell runtime shape sample")
    }
    $nonBellText = [string](Get-OptionalProperty -Object $nonBellSample -Name "text")
    $nonBellNormalizedValue = Get-OptionalProperty -Object $nonBellSample -Name "normalized_value"
    $setNonBell = Invoke-AgentTool -Tool "plugin.set_parameter" -ToolArgs @{
        track_id = $trackID
        plugin_id = $pluginID
        param_id = $filterTypeParameterID
        normalized_value = $nonBellNormalizedValue
    } -Confirmed $true
    Write-JsonArtifact -Name "filter_type_set_non_bell.json" -Value $setNonBell | Out-Null
    Assert-StatusOk -Response $setNonBell -Label "set TDR Nova conformed filter type to non-Bell"
    $typeNonBell = Invoke-AgentTool -Tool "plugin.get_parameters" -ToolArgs @{ track_id = $trackID; plugin_id = $pluginID; include_vps_v3_surface = $true } -Confirmed $false
    Write-JsonArtifact -Name "filter_type_after_non_bell.json" -Value $typeNonBell | Out-Null
    Assert-StatusOk -Response $typeNonBell -Label "read TDR Nova after non-Bell transition"
    $typeNonBellParameter = @(ResponseValue -Response $typeNonBell -Name "parameters" | Where-Object { [string](Get-OptionalProperty -Object $_ -Name "id") -eq $filterTypeParameterID }) | Select-Object -First 1
    if ($null -eq $typeNonBellParameter -or (Parameter-DisplayText -Parameter $typeNonBellParameter) -match "(?i)bell") {
        Fail ("TDR Nova non-Bell shape write was not confirmed by fresh host readback: " + ($typeNonBellParameter | ConvertTo-Json -Depth 20 -Compress))
    }

    $eqContext = @{
        selected_track_id = $trackID
        selected_plugin_track_id = $trackID
        selected_plugin_id = $pluginID
        selected_clip_id = $clipID
		spal_reference_eq = @{
            target_ref = "track:" + $trackID
			provider_credential_id = $credentialID
            frequency_hz = 92
            gain_db = -2.5
            q = 1.2
            clip_id = $clipID
        }
    }
    $eqMessage = "SPAL Reference EQ test: frequency: 92Hz, gain: -2.5dB, Q: 1.2."
    $eqBlocked = Invoke-AgentChat -ConversationID $conversationID -Message $eqMessage -Context $eqContext
    Write-JsonArtifact -Name "reference_eq_non_bell_blocked.json" -Value $eqBlocked | Out-Null
    if ([string](WorkflowValue -Response $eqBlocked -Name "canary_stage") -ne "vps_static_bell_not_ready" -or [bool](WorkflowValue -Response $eqBlocked -Name "plugin_learning_required")) {
        Fail ("Non-Bell runtime state did not return the recoverable VPS precondition response: " + ($eqBlocked | ConvertTo-Json -Depth 28 -Compress))
    }
    Assert-VPSCredentialStillVerified -LibraryPath $smokeLibraryPath -VPSID $vpsID -CredentialID $credentialID -Label "non-Bell VPS regression"
    $catalogDuringNonBell = Invoke-Json -Method GET -Uri ($AgentHttp.TrimEnd("/") + "/agent/vps/catalog") -TimeoutSec 30
    Write-JsonArtifact -Name "vps_catalog_during_non_bell.json" -Value $catalogDuringNonBell | Out-Null
    $catalogDuringNonBellCredential = @((Get-OptionalProperty -Object (Get-OptionalProperty -Object $catalogDuringNonBell -Name "catalog") -Name "entries") | Where-Object { [string]$_.credential_id -eq $credentialID }) | Select-Object -First 1
    if ($null -eq $catalogDuringNonBellCredential) {
        Fail ("Changing only the current filter shape removed the new Credential from the VPS Catalog: " + ($catalogDuringNonBell | ConvertTo-Json -Depth 24 -Compress))
    }
    $restoreBell = Invoke-AgentTool -Tool "plugin.set_parameter" -ToolArgs @{
        track_id = $trackID
        plugin_id = $pluginID
        param_id = $filterTypeParameterID
        normalized_value = $bellNormalizedValue
    } -Confirmed $true
    Write-JsonArtifact -Name "filter_type_restore_bell.json" -Value $restoreBell | Out-Null
    Assert-StatusOk -Response $restoreBell -Label "restore TDR Nova conformed filter type to Bell"
    $typeRestored = Invoke-AgentTool -Tool "plugin.get_parameters" -ToolArgs @{ track_id = $trackID; plugin_id = $pluginID; include_vps_v3_surface = $true } -Confirmed $false
    Write-JsonArtifact -Name "filter_type_after_bell_restore.json" -Value $typeRestored | Out-Null
    Assert-StatusOk -Response $typeRestored -Label "read TDR Nova after Bell restore"
    $typeRestoredParameter = @(ResponseValue -Response $typeRestored -Name "parameters" | Where-Object { [string](Get-OptionalProperty -Object $_ -Name "id") -eq $filterTypeParameterID }) | Select-Object -First 1
    if ($null -eq $typeRestoredParameter -or (Parameter-DisplayText -Parameter $typeRestoredParameter) -notmatch "(?i)bell") {
        Fail ("Bell restore was not confirmed by fresh host readback: " + ($typeRestoredParameter | ConvertTo-Json -Depth 20 -Compress))
    }
    Assert-VPSCredentialStillVerified -LibraryPath $smokeLibraryPath -VPSID $vpsID -CredentialID $credentialID -Label "Bell-restored VPS regression"
    $eqProposal = Invoke-AgentChat -ConversationID $conversationID -Message $eqMessage -Context $eqContext
    Write-JsonArtifact -Name "reference_eq_proposal.json" -Value $eqProposal | Out-Null
    Assert-NeedsConfirmation -Response $eqProposal -Label "Reference EQ"
    if ([string](WorkflowValue -Response $eqProposal -Name "capability_id") -ne "spal.reference_eq_test.v0") {
        Fail ("Reference EQ did not route through SPAL capability: " + ($eqProposal | ConvertTo-Json -Depth 28 -Compress))
    }
	if ([string](WorkflowValue -Response $eqProposal -Name "provider_source") -ne "vps_v3_catalog" -or [string](WorkflowValue -Response $eqProposal -Name "provider_credential_id") -ne $credentialID) {
		Fail ("Reference EQ proposal was not bound from the new VPS v3 Credential: " + ($eqProposal | ConvertTo-Json -Depth 28 -Compress))
	}
    $eqConfirm = Invoke-AgentChat -ConversationID $conversationID -Message "Execute this proposal." -Context $eqContext
    Write-JsonArtifact -Name "reference_eq_execute.json" -Value $eqConfirm | Out-Null
    Assert-StructuralPass -Response $eqConfirm -Label "Reference EQ execution"
    if ([string](WorkflowValue -Response $eqConfirm -Name "user_acceptance") -ne "unknown") {
        Fail "Reference EQ execution claimed a musical/user result"
    }
    if (-not [bool](WorkflowValue -Response $eqConfirm -Name "rollback_available")) {
        Fail "Reference EQ execution did not preserve a rollback path"
    }

    $rollbackContext = @{ capability_id = "spal.reference_eq_test.v0"; spal_operation = "rollback" }
    $rollback = Invoke-AgentChat -ConversationID $conversationID -Message "Rollback SPAL Reference EQ test." -Context $rollbackContext
    Write-JsonArtifact -Name "reference_eq_rollback_proposal.json" -Value $rollback | Out-Null
    Assert-NeedsConfirmation -Response $rollback -Label "Reference EQ rollback"
    $rollbackConfirm = Invoke-AgentChat -ConversationID $conversationID -Message "Execute this proposal." -Context $rollbackContext
    Write-JsonArtifact -Name "reference_eq_rollback_execute.json" -Value $rollbackConfirm | Out-Null
    Assert-StructuralPass -Response $rollbackConfirm -Label "Reference EQ rollback execution"

    $summary.status = "passed"
    $summary.conversation_id = $conversationID
    $summary.track_id = $trackID
    $summary.clip_id = $clipID
    $summary.plugin_id = $pluginID
	$summary.vps_id = $vpsID
	$summary.provider_credential_id = $credentialID
    $summary.assertions.natural_language_learning = "credential_verified"
    $summary.assertions.catalog = "verified_credential_visible"
    $summary.assertions.provider_binding = "vps_v3_catalog"
	$summary.assertions.mutable_filter_shape = ("{0} -> {1} -> Bell without Credential invalidation" -f $bellText, $nonBellText)
	$summary.assertions.experimental_tdr_skill_document_before_learning = $referenceGlobalProfileBeforeLearning
    $summary.assertions.reference_eq = "structural_pass"
    $summary.assertions.reference_eq_user_acceptance = "unknown"
    $summary.assertions.rollback = "structural_pass"
}
catch {
    $summary.error = $_.Exception.Message
    throw
}
finally {
    if ($referenceGlobalProfileReset) {
        $summary.global_profile_reset = $referenceGlobalProfileBeforeLearning
        $summary.global_profile_after_learning = if (Test-Path -LiteralPath $referenceGlobalProfilePath) { "recreated_by_natural_language_learning" } else { "absent_after_learning" }
    }
    $summary.finished_at = (Get-Date).ToUniversalTime().ToString("o")
    Write-JsonArtifact -Name "spal_reference_eq_summary.json" -Value $summary | Out-Null
}

Write-Output (Join-Path $ArtifactDir "spal_reference_eq_summary.json")
