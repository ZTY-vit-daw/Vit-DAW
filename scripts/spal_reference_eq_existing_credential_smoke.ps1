[CmdletBinding()]
param(
    [string]$RepoRoot = "D:\Vit_DAW",
    [string]$AgentHttp = "http://127.0.0.1:7878",
    [string]$TrackID = "1007",
    [string]$PluginID = "1015",
    [string]$VPSID = "vps_8c9699daa17a3603278d",
    [string]$CredentialID = "credential_a0d52b966a1f0a2e",
    [string]$ArtifactDir = "",
    [int]$TimeoutSeconds = 180
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

# This is intentionally an existing-Credential smoke.  It never calls Plugin
# Learning, profile mutation, project creation/clearing, rack creation, or a
# direct plug-in write.  The only project mutation is the confirmation-gated
# Static Bell action, followed by its confirmation-gated rollback.

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
        [bool]$Confirmed = $false
    )
    return Invoke-Json -Method POST -Uri ($AgentHttp.TrimEnd("/") + "/agent/invoke") -Body @{
        tool = $Tool
        args = $ToolArgs
        confirmed = $Confirmed
        source = "spal_existing_credential_smoke"
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
        interaction_path = "agent_http_after_godot_project_lifecycle"
        product_path_smoke = $true
        product_lifecycle = "godot_project"
        spal_existing_credential_smoke = $true
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

function ResponseValue {
    param([object]$Response, [string]$Name)
    $value = Get-OptionalProperty -Object $Response -Name $Name
    if ($null -ne $value) {
        return $value
    }
    return Get-OptionalProperty -Object (Get-OptionalProperty -Object $Response -Name "result") -Name $Name
}

function WorkflowValue {
    param([object]$Response, [string]$Name)
    return Get-OptionalProperty -Object (Get-OptionalProperty -Object $Response -Name "workflow_data") -Name $Name
}

function Assert-StatusOk {
    param([object]$Response, [string]$Label)
    if ([string](Get-OptionalProperty -Object $Response -Name "status") -ne "ok") {
        Fail ($Label + " failed: " + ($Response | ConvertTo-Json -Depth 24 -Compress))
    }
}

function Assert-NeedsConfirmation {
    param([object]$Response, [string]$Label)
    if (-not [bool](Get-OptionalProperty -Object $Response -Name "needs_confirmation")) {
        Fail ($Label + " did not create a confirmation Proposal: " + ($Response | ConvertTo-Json -Depth 28 -Compress))
    }
}

function Assert-StructuralPass {
    param([object]$Response, [string]$Label)
    if ([string](WorkflowValue -Response $Response -Name "structural_verification") -ne "pass") {
        Fail ($Label + " did not report structural pass: " + ($Response | ConvertTo-Json -Depth 28 -Compress))
    }
}

function Number-Value {
    param([object]$Value)
    if ($null -eq $Value) {
        return $null
    }
    $text = ([string]$Value).Trim().Replace("−", "-")
    if ([string]::IsNullOrWhiteSpace($text) -or $text -eq "<nil>") {
        return $null
    }
    $number = 0.0
    if (-not [double]::TryParse($text, [System.Globalization.NumberStyles]::Float, [System.Globalization.CultureInfo]::InvariantCulture, [ref]$number)) {
        return $null
    }
    return $number
}

function Assert-NearNumber {
    param(
        [object]$Actual,
        [double]$Expected,
        [string]$Label,
        [double]$Tolerance = 0.001
    )
    $value = Number-Value -Value $Actual
    if ($null -eq $value -or [Math]::Abs(([double]$value) - $Expected) -gt $Tolerance) {
        Fail ($Label + " expected " + [string]$Expected + " got " + [string]$Actual)
    }
}

function Parameter-DisplayText {
    param([object]$Parameter)
    $text = [string](Get-OptionalProperty -Object $Parameter -Name "value_text")
    if (-not [string]::IsNullOrWhiteSpace($text)) {
        return $text.Trim()
    }
    return ([string](Get-OptionalProperty -Object (Get-OptionalProperty -Object $Parameter -Name "display_probe") -Name "current_text")).Trim()
}

function Get-ConformedParameterValues {
    param([string]$ArtifactName)
    $response = Invoke-AgentTool -Tool "plugin.get_parameters" -ToolArgs @{
        track_id = $TrackID
        plugin_id = $PluginID
        include_vps_v3_surface = $true
    }
    Write-JsonArtifact -Name $ArtifactName -Value $response | Out-Null
    Assert-StatusOk -Response $response -Label "fresh TDR Nova parameter read"
    $parameters = @(ResponseValue -Response $response -Name "parameters")
    $byID = @{}
    foreach ($parameter in $parameters) {
        $id = [string](Get-OptionalProperty -Object $parameter -Name "id")
        if (-not [string]::IsNullOrWhiteSpace($id)) {
            $byID[$id] = $parameter
        }
    }
    foreach ($id in @($script:FrequencyParameterID, $script:GainParameterID, $script:QParameterID, $script:TypeParameterID)) {
        if (-not $byID.ContainsKey($id)) {
            Fail ("fresh parameter surface omitted conformed parameter " + $id)
        }
    }
    return [pscustomobject]@{
        frequency = $byID[$script:FrequencyParameterID]
        gain = $byID[$script:GainParameterID]
        q = $byID[$script:QParameterID]
        shape = $byID[$script:TypeParameterID]
    }
}

function New-ProposalContext {
    param([hashtable]$BaseContext, [object]$Proposal)
    $context = @{}
    foreach ($key in $BaseContext.Keys) {
        $context[$key] = $BaseContext[$key]
    }
    $workflow = Get-OptionalProperty -Object $Proposal -Name "workflow_data"
    $proposalID = [string](Get-OptionalProperty -Object $Proposal -Name "plan_id")
    $revision = Get-OptionalProperty -Object $workflow -Name "proposal_revision"
    $actionSetHash = [string](Get-OptionalProperty -Object $workflow -Name "action_set_hash")
    $projectCutHash = [string](Get-OptionalProperty -Object $workflow -Name "project_cut_hash")
    if ([string]::IsNullOrWhiteSpace($proposalID) -or $null -eq $revision -or [string]::IsNullOrWhiteSpace($actionSetHash) -or [string]::IsNullOrWhiteSpace($projectCutHash)) {
        Fail ("Proposal did not expose a complete exact-confirmation binding: " + ($Proposal | ConvertTo-Json -Depth 28 -Compress))
    }
    $context["capability_id"] = "spal.reference_eq_test.v0"
    $context["expected_proposal_id"] = $proposalID
    $context["expected_proposal_revision"] = $revision
    $context["expected_action_set_hash"] = $actionSetHash
    $context["expected_project_cut_hash"] = $projectCutHash
    return $context
}

function Invoke-SafetyRollback {
    param([string]$ConversationID, [hashtable]$RollbackContext, [string]$Prefix)
    $proposal = Invoke-AgentChat -ConversationID $ConversationID -Message "回滚 SPAL Reference EQ 测试。" -Context $RollbackContext
    Write-JsonArtifact -Name ($Prefix + "_rollback_proposal.json") -Value $proposal | Out-Null
    Assert-NeedsConfirmation -Response $proposal -Label "SPAL rollback"
    $approvalContext = New-ProposalContext -BaseContext $RollbackContext -Proposal $proposal
    $executed = Invoke-AgentChat -ConversationID $ConversationID -Message "执行这个方案。" -Context $approvalContext
    Write-JsonArtifact -Name ($Prefix + "_rollback_execute.json") -Value $executed | Out-Null
    Assert-StructuralPass -Response $executed -Label "SPAL rollback execution"
    return $executed
}

if ([string]::IsNullOrWhiteSpace($ArtifactDir)) {
    $ArtifactDir = Join-Path $RepoRoot ("VitApp\Workspace\Artifacts\smoke\spal_existing_credential_" + (Get-Date -Format "yyyyMMdd_HHmmss"))
}
New-Item -ItemType Directory -Path $ArtifactDir -Force | Out-Null

$libraryPath = Join-Path $env:APPDATA "Vit\Agent\vps_library_v3.json"
$summary = [ordered]@{
    schema_version = "spal_reference_eq_existing_credential_smoke.v1"
    status = "failed"
    started_at = (Get-Date).ToUniversalTime().ToString("o")
    artifact_dir = $ArtifactDir
    track_id = $TrackID
    plugin_id = $PluginID
    vps_id = $VPSID
    provider_credential_id = $CredentialID
    assertions = [ordered]@{}
}
$executed = $false
$rolledBack = $false
$conversationID = "spal_existing_credential_" + (Get-Date -Format "yyyyMMdd_HHmmss")

try {
    if (-not (Test-Path -LiteralPath $libraryPath)) {
        Fail ("existing VPS Library was not found: " + $libraryPath)
    }
    $libraryHashBefore = (Get-FileHash -LiteralPath $libraryPath -Algorithm SHA256).Hash
    $library = Get-Content -LiteralPath $libraryPath -Raw -Encoding UTF8 | ConvertFrom-Json
    $document = @($library.documents | Where-Object { [string]$_.id -eq $VPSID }) | Select-Object -First 1
    if ($null -eq $document) {
        Fail ("existing VPS document was not found: " + $VPSID)
    }
    $credential = @($document.provider_credentials | Where-Object { [string]$_.id -eq $CredentialID }) | Select-Object -First 1
    if ($null -eq $credential -or [string]$credential.status -ne "verified") {
        Fail ("existing Credential is not verified: " + $CredentialID)
    }
    $binding = Get-OptionalProperty -Object (Get-OptionalProperty -Object $credential -Name "conformance") -Name "static_eq_binding"
    if ($null -eq $binding) {
        Fail "existing Credential has no Static EQ binding"
    }
    $script:FrequencyParameterID = [string](Get-OptionalProperty -Object $binding -Name "frequency_parameter_id")
    $script:GainParameterID = [string](Get-OptionalProperty -Object $binding -Name "gain_parameter_id")
    $script:QParameterID = [string](Get-OptionalProperty -Object $binding -Name "q_parameter_id")
    $script:TypeParameterID = [string](Get-OptionalProperty -Object $binding -Name "filter_type_parameter_id")
    if (@($script:FrequencyParameterID, $script:GainParameterID, $script:QParameterID, $script:TypeParameterID | Where-Object { [string]::IsNullOrWhiteSpace($_) }).Count -gt 0) {
        Fail "existing Credential Static EQ binding is incomplete"
    }

    $health = Invoke-Json -Method GET -Uri ($AgentHttp.TrimEnd("/") + "/health") -TimeoutSec 15
    Write-JsonArtifact -Name "agent_health.json" -Value $health | Out-Null
    if ($null -eq $health) {
        Fail "VitAgent health endpoint did not return a response; start it through the Godot lifecycle first"
    }
    $catalogBefore = Invoke-Json -Method GET -Uri ($AgentHttp.TrimEnd("/") + "/agent/vps/catalog") -TimeoutSec 30
    Write-JsonArtifact -Name "vps_catalog_before.json" -Value $catalogBefore | Out-Null
    if ([string](Get-OptionalProperty -Object $catalogBefore -Name "status") -ne "ok") {
        Fail ("VPS Catalog endpoint was unavailable: " + ($catalogBefore | ConvertTo-Json -Depth 20 -Compress))
    }
    $catalogCredential = @((Get-OptionalProperty -Object (Get-OptionalProperty -Object $catalogBefore -Name "catalog") -Name "entries") | Where-Object {
            [string]$_.credential_id -eq $CredentialID -and [string]$_.vps_id -eq $VPSID -and [string]$_.capability_id -eq "spectral.static_eq.v0"
        }) | Select-Object -First 1
    if ($null -eq $catalogCredential) {
        Fail "existing verified Credential was not visible in the VPS v3 Catalog"
    }
    $summary.assertions.catalog_before = "verified_existing_credential_visible"

    $before = Get-ConformedParameterValues -ArtifactName "parameters_before.json"
    if ((Parameter-DisplayText -Parameter $before.shape) -notmatch "(?i)bell") {
        Fail ("current conformed filter type is not Bell; smoke refuses to write it: " + (Parameter-DisplayText -Parameter $before.shape))
    }
    Write-JsonArtifact -Name "parameter_values_before.json" -Value $before | Out-Null

    # Do not provide frequency/gain/Q in context: this must prove the bare
    # natural-language tuple itself takes the VPS v3 Static Bell route.
    $baseContext = @{
        selected_track_id = $TrackID
        selected_plugin_track_id = $TrackID
        selected_plugin_id = $PluginID
        start_seconds = 0
        end_seconds = 1
        spal_reference_eq = @{
            target_ref = "track:" + $TrackID
            provider_credential_id = $CredentialID
            start_seconds = 0
            end_seconds = 1
        }
    }
    $message = "在当前 TDR Nova 上做 92Hz、-2.5dB、Q 1.2 的静态 Bell EQ。"
    $proposal = Invoke-AgentChat -ConversationID $conversationID -Message $message -Context $baseContext
    Write-JsonArtifact -Name "natural_language_proposal.json" -Value $proposal | Out-Null
    Assert-NeedsConfirmation -Response $proposal -Label "existing-Credential Static Bell request"
    if ([string](WorkflowValue -Response $proposal -Name "capability_id") -ne "spal.reference_eq_test.v0" -or
        [string](WorkflowValue -Response $proposal -Name "provider_source") -ne "vps_v3_catalog" -or
        [string](WorkflowValue -Response $proposal -Name "provider_credential_id") -ne $CredentialID) {
        Fail ("natural-language Static Bell request was not bound to the existing VPS v3 Credential: " + ($proposal | ConvertTo-Json -Depth 28 -Compress))
    }
    $summary.assertions.natural_language_route = "spal.reference_eq_test.v0 via vps_v3_catalog"

    $approvalContext = New-ProposalContext -BaseContext $baseContext -Proposal $proposal
    $execution = Invoke-AgentChat -ConversationID $conversationID -Message "执行这个方案。" -Context $approvalContext
    $executed = $true
    Write-JsonArtifact -Name "static_bell_execute.json" -Value $execution | Out-Null
    Assert-StructuralPass -Response $execution -Label "existing-Credential Static Bell execution"
    if (-not [bool](WorkflowValue -Response $execution -Name "rollback_available")) {
        Fail "Static Bell execution did not preserve a rollback path"
    }

    $afterApply = Get-ConformedParameterValues -ArtifactName "parameters_after_apply.json"
    Assert-NearNumber -Actual (Parameter-DisplayText -Parameter $afterApply.frequency) -Expected 92 -Tolerance 0.05 -Label "frequency readback"
    Assert-NearNumber -Actual (Parameter-DisplayText -Parameter $afterApply.gain) -Expected -2.5 -Tolerance 0.02 -Label "gain readback"
    Assert-NearNumber -Actual (Parameter-DisplayText -Parameter $afterApply.q) -Expected 1.2 -Tolerance 0.02 -Label "Q readback"
    if ((Parameter-DisplayText -Parameter $afterApply.shape) -notmatch "(?i)bell") {
        Fail "Static Bell execution changed the verified filter shape"
    }
    Write-JsonArtifact -Name "parameter_values_after_apply.json" -Value $afterApply | Out-Null
    $summary.assertions.apply_readback = "92 Hz / -2.5 dB / Q 1.2 / Bell"

    $rollbackContext = @{
        capability_id = "spal.reference_eq_test.v0"
        spal_operation = "rollback"
        selected_track_id = $TrackID
        selected_plugin_track_id = $TrackID
        selected_plugin_id = $PluginID
    }
    $rollback = Invoke-SafetyRollback -ConversationID $conversationID -RollbackContext $rollbackContext -Prefix "normal"
    $rolledBack = $true
    $afterRollback = Get-ConformedParameterValues -ArtifactName "parameters_after_rollback.json"
    foreach ($row in @(
            @{ name = "frequency"; before = $before.frequency; after = $afterRollback.frequency },
            @{ name = "gain"; before = $before.gain; after = $afterRollback.gain },
            @{ name = "Q"; before = $before.q; after = $afterRollback.q }
        )) {
        Assert-NearNumber -Actual (Get-OptionalProperty -Object $row.after -Name "normalized_value") -Expected ([double](Get-OptionalProperty -Object $row.before -Name "normalized_value")) -Tolerance 0.00001 -Label ("rollback " + $row.name + " normalized readback")
    }
    if ((Parameter-DisplayText -Parameter $afterRollback.shape) -ne (Parameter-DisplayText -Parameter $before.shape)) {
        Fail "rollback did not restore the original filter shape display value"
    }
    Write-JsonArtifact -Name "parameter_values_after_rollback.json" -Value $afterRollback | Out-Null
    $summary.assertions.rollback = "fresh host normalized readback restored preimage"

    $catalogAfter = Invoke-Json -Method GET -Uri ($AgentHttp.TrimEnd("/") + "/agent/vps/catalog") -TimeoutSec 30
    Write-JsonArtifact -Name "vps_catalog_after.json" -Value $catalogAfter | Out-Null
    $catalogAfterCredential = @((Get-OptionalProperty -Object (Get-OptionalProperty -Object $catalogAfter -Name "catalog") -Name "entries") | Where-Object {
            [string]$_.credential_id -eq $CredentialID -and [string]$_.vps_id -eq $VPSID
        }) | Select-Object -First 1
    if ($null -eq $catalogAfterCredential) {
        Fail "existing Credential disappeared from the Catalog during smoke"
    }
    $libraryHashAfter = (Get-FileHash -LiteralPath $libraryPath -Algorithm SHA256).Hash
    if ($libraryHashAfter -ne $libraryHashBefore) {
        Fail "existing VPS Library changed during a no-learning smoke"
    }
    $summary.assertions.library = "unchanged"
    $summary.status = "passed"
}
catch {
    $summary.error = $_.Exception.Message
    if ($executed -and -not $rolledBack) {
        try {
            $rollbackContext = @{
                capability_id = "spal.reference_eq_test.v0"
                spal_operation = "rollback"
                selected_track_id = $TrackID
                selected_plugin_track_id = $TrackID
                selected_plugin_id = $PluginID
            }
            $null = Invoke-SafetyRollback -ConversationID $conversationID -RollbackContext $rollbackContext -Prefix "emergency"
            $rolledBack = $true
            $summary.emergency_rollback = "structural_pass"
        }
        catch {
            $summary.emergency_rollback = "failed: " + $_.Exception.Message
        }
    }
    throw
}
finally {
    $summary.finished_at = (Get-Date).ToUniversalTime().ToString("o")
    $summary.conversation_id = $conversationID
    $summary.rollback_completed = $rolledBack
    Write-JsonArtifact -Name "spal_existing_credential_summary.json" -Value $summary | Out-Null
}

Write-Output (Join-Path $ArtifactDir "spal_existing_credential_summary.json")

