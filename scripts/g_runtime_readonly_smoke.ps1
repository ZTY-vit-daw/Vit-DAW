#requires -Version 5.1

[CmdletBinding()]
param(
    [string]$AgentHttp = "http://127.0.0.1:7878",
    [int]$TimeoutSeconds = 5,
    [string]$ConversationId = "g-readonly-smoke-probe",
    [string]$OutputPath = ""
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$AllowedPaths = @(
    "/health",
    "/agent/runtime/status",
    "/agent/state",
    "/agent/events"
)

function Assert-ReadonlyHarness {
    param([string]$ScriptPath)

    $source = Get-Content -LiteralPath $ScriptPath -Raw
    # The harness must contain no non-GET request construction. Keep this
    # guard independent from the request helper so future edits fail closed.
    if ($source -match '(?im)(?:-Method\s+|Method\s*=\s*)["'']?POST\b') {
        throw "G readonly harness contains a non-GET request method"
    }
    $requestBodyToken = "-" + "Body"
    if ($source -match ('(?im)Invoke-WebRequest[^\r\n]*' + [regex]::Escape($requestBodyToken) + '\\b')) {
        throw "G readonly harness contains a request body"
    }
}

function Get-ReadonlyJson {
    param(
        [string]$BaseUrl,
        [string]$Path
    )

    if ($AllowedPaths -notcontains $Path) {
        throw "G readonly harness rejected non-whitelisted path: $Path"
    }
    $uri = $BaseUrl.TrimEnd("/") + $Path
    if ($Path -eq "/agent/events") {
        # The real agent contract (agent/internal/chat/events.go) answers HTTP
        # 400 without a conversation id; the fixture server ignores query
        # parameters. limit=200 stays within the real agent's event buffer cap.
        $uri += "?conversation_id=$ConversationId&limit=200"
    }
    $response = Invoke-WebRequest -UseBasicParsing -Method GET -Uri $uri -TimeoutSec $TimeoutSeconds
    if ($response.StatusCode -lt 200 -or $response.StatusCode -ge 300) {
        throw "GET $Path returned HTTP $($response.StatusCode)"
    }
    if ([string]::IsNullOrWhiteSpace($response.Content)) {
        return [ordered]@{}
    }
    return $response.Content | ConvertFrom-Json
}

function Get-PropertyOrEmpty {
    param([object]$Value, [string]$Name)
    if ($null -eq $Value) { return "" }
    $property = $Value.PSObject.Properties[$Name]
    if ($null -eq $property -or $null -eq $property.Value) { return "" }
    return $property.Value
}

Assert-ReadonlyHarness -ScriptPath $PSCommandPath

$health = Get-ReadonlyJson -BaseUrl $AgentHttp -Path "/health"
$runtime = Get-ReadonlyJson -BaseUrl $AgentHttp -Path "/agent/runtime/status"
$state = Get-ReadonlyJson -BaseUrl $AgentHttp -Path "/agent/state"
$events = Get-ReadonlyJson -BaseUrl $AgentHttp -Path "/agent/events"

$trajectory = Get-PropertyOrEmpty -Value $runtime -Name "task_trajectory"
$task = Get-PropertyOrEmpty -Value $trajectory -Name "task"
$run = Get-PropertyOrEmpty -Value $trajectory -Name "run"
$slices = @(Get-PropertyOrEmpty -Value $run -Name "slices")
$summary = [ordered]@{
    schema_version = "g.readonly.runtime.smoke.v1"
    endpoints = $AllowedPaths
    health_status = [string](Get-PropertyOrEmpty -Value $health -Name "status")
    state_status = [string](Get-PropertyOrEmpty -Value $state -Name "status")
    task_id = [string](Get-PropertyOrEmpty -Value $task -Name "task_id")
    goal_id = [string](Get-PropertyOrEmpty -Value $task -Name "goal_id")
    run_id = [string](Get-PropertyOrEmpty -Value $task -Name "run_id")
    original_intent = [string](Get-PropertyOrEmpty -Value $task -Name "original_intent")
    task_status = [string](Get-PropertyOrEmpty -Value $task -Name "status")
    semantic_state = [string](Get-PropertyOrEmpty -Value (Get-PropertyOrEmpty -Value $trajectory -Name "semantic") -Name "state")
    slice_count = $slices.Count
    event_count = @((Get-PropertyOrEmpty -Value $events -Name "events")).Count
    read_only = $true
}

$json = $summary | ConvertTo-Json -Depth 12
if (-not [string]::IsNullOrWhiteSpace($OutputPath)) {
    $parent = Split-Path -Parent $OutputPath
    if (-not [string]::IsNullOrWhiteSpace($parent)) {
        New-Item -ItemType Directory -Force -Path $parent | Out-Null
    }
    Set-Content -LiteralPath $OutputPath -Value $json -Encoding UTF8
}
Write-Output $json
