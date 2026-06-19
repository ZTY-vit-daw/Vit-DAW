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
        plugin_set_parameter = Count-ExecutedToolGroup -Rows $Rows -Aliases @("plugin.set_parameter", "plugin_set_parameter", "set_plugin_param")
        plugin_grabber_apply_control = Count-ExecutedToolGroup -Rows $Rows -Aliases @("plugin_grabber.apply_control", "plugin_grabber_apply_control")
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
    foreach ($key in @("operation", "track_id", "track_name", "delta_db", "delta_pan", "target_pan", "before_db", "after_db", "before_pan", "after_pan", "tick_id", "kernel_command", "requires_confirmation", "requires_refresh", "observation_id")) {
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
}

function Assert-PluginLearningPreparationPlan {
    param([object]$Response)
    $pluginLearning = Get-OptionalProperty -Object $Response -Name "plugin_learning"
    if ($null -eq $pluginLearning) {
        Fail "plugin preparation response did not include plugin_learning payload"
    }
    $plan = Get-OptionalProperty -Object $pluginLearning -Name "mix_treatment_preparation_plan"
    if ($null -eq $plan) {
        Fail "plugin_learning payload did not include mix_treatment_preparation_plan"
    }
    $schema = [string](Get-OptionalProperty -Object $plan -Name "schema_version")
    if ($schema -ne "mix_treatment_preparation.v0") {
        Fail ("unexpected plugin_learning preparation plan schema=" + $schema)
    }
    $interactions = @((Get-OptionalProperty -Object $Response -Name "interaction_requests"))
    if ($interactions.Count -lt 1) {
        Fail "plugin preparation response did not include interaction_requests"
    }
    $payload = Get-OptionalProperty -Object $interactions[0] -Name "payload"
    $interactionPlan = Get-OptionalProperty -Object $payload -Name "mix_treatment_preparation_plan"
    if ($null -eq $interactionPlan) {
        Fail "plugin learning interaction did not carry mix_treatment_preparation_plan"
    }
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

function Invoke-BlindTurn {
    param(
        [string]$ConversationID,
        [string]$Message,
        [string]$Label
    )
    $response = Invoke-AgentChat -ConversationID $ConversationID -Message $Message
    $fileName = ("chat_" + $Label + ".json")
    ConvertTo-JsonFile -Value $response -Path (Join-Path $ArtifactDir $fileName)
    $rawRows = Get-OptionalProperty -Object $response -Name "executed_kernel_reply"
    $rows = @()
    if ($null -ne $rawRows) {
        $rows = @($rawRows)
    }
    $tools = @(Tool-Names -Rows $rows)
    $observationIDs = @(Extract-ObservationIDs -Rows $rows)
    $turn = [ordered]@{
        label = $Label
        conversation_id = $ConversationID
        message = $Message
        stop_reason = [string](Get-OptionalProperty -Object $response -Name "stop_reason")
        goal_status = [string](Get-OptionalProperty -Object $response -Name "goal_status")
        reply = [string](Get-OptionalProperty -Object $response -Name "reply")
        tool_route = $tools
        tool_counts = Tool-Counts -Rows $rows
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
        response = $response
        turn = $turn
        rows = $rows
        tools = $tools
    }
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
    if ([int]$counts.daw_invoke -gt 0 -or [int]$counts.track_volume -gt 0 -or [int]$counts.track_pan -gt 0 -or [int]$counts.plugin_set_parameter -gt 0) {
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
    if ([string]$Turn.stop_reason -ne "done") {
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
    if ($LASTEXITCODE -ne 0) {
        Fail ("dev_agent_smoke failed with exit code " + $LASTEXITCODE)
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
    $lowMudMessage = Join-UnicodeChars @(0x4F4E, 0x9891, 0x6709, 0x70B9, 0x7CCA, 0xFF0C, 0x770B, 0x770B, 0x600E, 0x4E48, 0x8C03)
    $panLeftMessage = "Track 2 " + (Join-UnicodeChars @(0x58F0, 0x50CF, 0x5F80, 0x5DE6, 0x4E00, 0x70B9))
    $vocalAnswerMessage = "Track 1 " + (Join-UnicodeChars @(0x662F, 0x4E3B, 0x5531))

    Write-Step "Scenario A/B/D/E: natural request, discuss, reobserve, confirm"
    $conversationID = "blind_mix_main_" + $Stamp
    $summary["conversation_id"] = $conversationID
    $first = Invoke-BlindTurn -ConversationID $conversationID -Message $overallMessage -Label "overall_observe"
    Record-Turn -Turn $first.turn
    Assert-ObserveOnlyFirstTurn -Turn $first.turn -Label "overall observe"
    $eventsFirst = Record-Events -ConversationID $conversationID -Label "overall_after_observe"
    $firstPending = @(Extract-PendingCandidates -Events $eventsFirst)
    $firstGainTreatments = @(Extract-GainTreatmentCandidates -Events $eventsFirst)
    if ($firstPending.Count -lt 1 -and $firstGainTreatments.Count -lt 1) {
        Fail "overall observe did not produce a pending gain candidate or gain treatment"
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
    if (@(Extract-PendingCandidates -Events $pendingEvents).Count -lt 1 -and @(Extract-GainTreatmentCandidates -Events $pendingEvents).Count -lt 1) {
        Fail "confirm observe did not produce a pending gain candidate or gain treatment"
    }
    $confirmTurn = Invoke-BlindTurn -ConversationID $confirmConversationID -Message $confirmMessage -Label "confirm_execute"
    Record-Turn -Turn $confirmTurn.turn
    Assert-ConfirmationRoute -Turn $confirmTurn.turn
    [void](Record-Events -ConversationID $confirmConversationID -Label "confirm_after_execute")

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
    $panEvents = Record-Events -ConversationID $panConversationID -Label "pan_after_observe"
    if (@(Extract-PanTreatmentCandidates -Events $panEvents).Count -lt 1) {
        Fail "pan observe did not produce pan_balance treatment"
    }
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
    $lowEvents = Record-Events -ConversationID $lowConversationID -Label "low_mud_after_observe"
    $lowTreatments = @(Extract-TreatmentPendingCandidates -Events $lowEvents)
    if ($lowTreatments.Count -lt 1) {
        Fail "low mud observe did not produce mix_treatment_pending"
    }
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
        Assert-PluginLearningPreparationPlan -Response $lowConfirm.response
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
