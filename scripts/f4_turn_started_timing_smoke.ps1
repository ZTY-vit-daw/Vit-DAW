<#
AGENT-F4 acceptance probe: first turn.started arrival latency after a sent
message, plus the free-state experiment stage-node burst offsets.

Reconnaissance context (queue/reports/2026-09-12-AGENT-F4-emission-timing.md):
the transport turn.started is emitted by handleChat right after the goal/run
identity is bound and before the semantic-entry classifier runs, so its arrival
latency is the agent entry cost only (plus whatever the server is already busy
with). The free-state experiment stage nodes are published in one synchronous
burst at the admission decision point.

Method notes (both learned the hard way on this host):
  * The chat POST is written over a raw TCP socket. HttpWebRequest /
    HttpClient hide the send behind a blocking call that only returns once the
    whole turn is answered, which silently destroys the concurrent event
    polling this probe depends on.
  * The probe waits for a quiet agent first: a scheduler-driven continuation
    chain from an earlier conversation serializes with new chat turns and would
    otherwise be charged to the new turn's entry latency.

This file is deliberately pure ASCII: Windows PowerShell 5.1 reads .ps1 without
a BOM using the system ANSI code page, so literal CJK text corrupts the parse
(same reason dev_agent_smoke.ps1 builds its Chinese strings from [char] codes).

Requires the real stack (agent on the AgentHttp endpoint). PASS = exit code 0.
#>
[CmdletBinding()]
param(
    [string]$AgentHttp = "http://127.0.0.1:7878",
    [string]$ConversationId = "",
    [string]$Message = "",
    [int]$BudgetMs = 500,
    [int]$PollIntervalMs = 25,
    [int]$TimeoutSeconds = 120,
    [int]$QuietSeconds = 3,
    [int]$QuietWaitSeconds = 180,
    [int]$SettleMs = 2500,
    [string]$AgentLogPath = "",
    [string]$ArtifactDir = ""
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

function Fail {
    param([string]$Message)
    Write-Host ("FAIL: " + $Message) -ForegroundColor Red
    exit 1
}

# "帮我混一下当前轨道" - built from code points, never from literal CJK bytes.
$defaultMessage = -join @(
    [char]0x5E2E, [char]0x6211, [char]0x6DF7, [char]0x4E00, [char]0x4E0B,
    [char]0x5F53, [char]0x524D, [char]0x8F68, [char]0x9053
)

$repoRoot = if ($PSScriptRoot) { Split-Path -Parent $PSScriptRoot } else { (Get-Location).Path }
if ([string]::IsNullOrWhiteSpace($Message)) { $Message = $defaultMessage }
if ([string]::IsNullOrWhiteSpace($ConversationId)) {
    $ConversationId = "f4_timing_" + (Get-Date -Format "yyyyMMdd_HHmmss")
}
if ([string]::IsNullOrWhiteSpace($AgentLogPath)) {
    $AgentLogPath = Join-Path $repoRoot "VitApp\Workspace\Logs\agent_last.log"
}
if ([string]::IsNullOrWhiteSpace($ArtifactDir)) {
    $ArtifactDir = Join-Path $repoRoot ("artifacts\f4_emission_timing\" + (Get-Date -Format "yyyyMMdd_HHmmss"))
}
New-Item -ItemType Directory -Force -Path $ArtifactDir | Out-Null

$base = $AgentHttp.TrimEnd("/")
$uri = [uri]$base
$hostName = $uri.Host
$port = $uri.Port
Write-Host "== AGENT-F4 turn.started arrival probe"
Write-Host ("agent http: " + $base)
Write-Host ("conversation: " + $ConversationId)
Write-Host ("budget_ms: " + [string]$BudgetMs + " poll_interval_ms: " + [string]$PollIntervalMs)

Write-Step "Agent reachability (read-only GET)"
# /agent/runtime/status is the cheap liveness read. /agent/state is NOT used
# here: with an unreachable kernel the shadow stays uninitialized and every
# /agent/state call triggers a full harness.refresh_shadow (~500 ms of failing
# kernel round trips on this host), which would be charged to the measurement.
try {
    $runtimeStatus = Invoke-RestMethod -Method GET -Uri ($base + "/agent/runtime/status") -TimeoutSec 10
} catch {
    Fail ("agent /agent/runtime/status unreachable: " + $_.Exception.Message)
}
Write-Ok "agent runtime status readable"

$eventsUri = $base + "/agent/events?conversation_id=" + [uri]::EscapeDataString($ConversationId) + "&since=0&limit=200"
# Warm the HTTP client stack so no first-call cost lands inside the measurement.
[void](Invoke-RestMethod -Method GET -Uri $eventsUri -TimeoutSec 10)

Write-Step "Wait for a quiet agent"
# The gate reads /agent/state rather than the log mtime: a 30 s model call
# writes nothing to the log while it runs, so an mtime gate would happily
# "pass" in the middle of a scheduler slice and charge that slice's remaining
# time to the new turn.
$quietDeadline = (Get-Date).AddSeconds($QuietWaitSeconds)
$quiet = $false
$lastSeen = ""
while ((Get-Date) -lt $quietDeadline) {
    $busy = $false
    try {
        $probeState = Invoke-RestMethod -Method GET -Uri ($base + "/agent/state") -TimeoutSec 10
        $active = $probeState.active_goal
        if ($active -ne $null) {
            $status = ""
            if ($active.PSObject.Properties.Name -contains "goal" -and $active.goal -ne $null) { $status = [string]$active.goal.status }
            $lastSeen = $status
            if ($status -eq "running" -or $status -eq "waiting_continue" -or $status -eq "waiting_confirmation") { $busy = $true }
        }
    } catch { $busy = $true }
    if (-not $busy) { $quiet = $true; break }
    Start-Sleep -Milliseconds 500
}
if ($quiet) {
    Write-Ok ("agent idle (last goal status was '" + $lastSeen + "')")
} else {
    Write-Host ("warn: agent never went idle within " + [string]$QuietWaitSeconds + "s; measuring under load")
}
# Let any work the idle gate itself kicked off drain before the clock starts.
Start-Sleep -Milliseconds $SettleMs

Write-Step "Send the message and measure first turn.started arrival"
$payload = @{
    conversation_id = $ConversationId
    message         = $Message
    context         = @{ agent_mode = "chat" }
} | ConvertTo-Json -Depth 6 -Compress
$bodyBytes = [System.Text.Encoding]::UTF8.GetBytes($payload)
$head = "POST /agent/chat HTTP/1.1" + [char]13 + [char]10 +
        "Host: " + $hostName + ":" + [string]$port + [char]13 + [char]10 +
        "Content-Type: application/json" + [char]13 + [char]10 +
        "Content-Length: " + [string]$bodyBytes.Length + [char]13 + [char]10 +
        "Connection: close" + [char]13 + [char]10 + [char]13 + [char]10
$headBytes = [System.Text.Encoding]::ASCII.GetBytes($head)

$tcp = New-Object System.Net.Sockets.TcpClient
try {
    $tcp.Connect($hostName, $port)
} catch {
    Fail ("cannot open TCP connection to " + $hostName + ":" + [string]$port + ": " + $_.Exception.Message)
}
$netStream = $tcp.GetStream()

$sw = [System.Diagnostics.Stopwatch]::StartNew()
$sentAt = (Get-Date).ToString("yyyy-MM-ddTHH:mm:ss.fffzzz")
try {
    $netStream.Write($headBytes, 0, $headBytes.Length)
    $netStream.Write($bodyBytes, 0, $bodyBytes.Length)
    $netStream.Flush()
} catch {
    Fail ("chat request could not be written: " + $_.Exception.Message)
}
Write-Host ("sent_at=" + $sentAt)

$turnStartedMs = -1.0
$trajectoryStartedMs = -1.0
$turnStartedWall = ""
$observedEvents = @()
$deadline = (Get-Date).AddSeconds($TimeoutSeconds)
while ((Get-Date) -lt $deadline) {
    $rows = @()
    try {
        $body = Invoke-RestMethod -Method GET -Uri $eventsUri -TimeoutSec 10
        if ($body -and ($body.PSObject.Properties.Name -contains "events") -and $body.events) { $rows = @($body.events) }
    } catch {
        Start-Sleep -Milliseconds $PollIntervalMs
        continue
    }
    if ($rows.Count -gt 0) {
        $observedEvents = $rows
        foreach ($row in $rows) {
            if ($row.type -eq "turn.started" -and $turnStartedMs -lt 0) {
                $turnStartedMs = $sw.Elapsed.TotalMilliseconds
                $turnStartedWall = [string]$row.created_at
            }
            if ($row.type -eq "trajectory.turn.started" -and $trajectoryStartedMs -lt 0) {
                $trajectoryStartedMs = $sw.Elapsed.TotalMilliseconds
            }
        }
    }
    if ($turnStartedMs -ge 0 -and $trajectoryStartedMs -ge 0) { break }
    Start-Sleep -Milliseconds $PollIntervalMs
}

# Drain the response so the turn is not left half-read, then take a final read.
try {
    $buffer = New-Object byte[] 8192
    while ($netStream.Read($buffer, 0, $buffer.Length) -gt 0) { }
} catch { }
try { $tcp.Close() } catch { }
Start-Sleep -Milliseconds 250
try {
    $final = Invoke-RestMethod -Method GET -Uri $eventsUri -TimeoutSec 10
    if ($final -and ($final.PSObject.Properties.Name -contains "events") -and $final.events) { $observedEvents = @($final.events) }
} catch { }

$eventsPath = Join-Path $ArtifactDir "turn_started_timing_events.json"
$observedEvents | ConvertTo-Json -Depth 12 | Set-Content -Path $eventsPath -Encoding UTF8
Write-Host ("artifact: " + $eventsPath)

if ($turnStartedMs -lt 0) {
    Fail ("no turn.started observed within " + [string]$TimeoutSeconds + "s for conversation " + $ConversationId)
}

Write-Host ("turn.started arrival_ms=" + [string][math]::Round($turnStartedMs, 1) + " created_at=" + $turnStartedWall)
if ($trajectoryStartedMs -ge 0) {
    Write-Host ("trajectory.turn.started arrival_ms=" + [string][math]::Round($trajectoryStartedMs, 1))
}

Write-Step "Free-state experiment stage-node burst (informational)"
$experimentBurst = @()
foreach ($row in $observedEvents) {
    if ($row.payload -and ($row.payload.PSObject.Properties.Name -contains "turn_id")) {
        $turnId = [string]$row.payload.turn_id
        if ($turnId.StartsWith("turn:free_state_")) {
            $experimentBurst += ($row.type + "@" + [string]$row.created_at)
        }
    }
}
if ($experimentBurst.Count -gt 0) {
    Write-Host ("experiment stage nodes: " + ($experimentBurst -join ", "))
} else {
    Write-Host "experiment stage nodes: none in this turn (no free-state experiment admission)"
}

if ($turnStartedMs -lt [double]$BudgetMs) {
    Write-Ok ("first turn.started arrived in " + [string][math]::Round($turnStartedMs, 1) + "ms (budget " + [string]$BudgetMs + "ms)")
    Write-Host ""
    Write-Host "== Summary"
    Write-Ok "AGENT-F4 turn.started arrival probe completed"
    exit 0
}
Fail ("first turn.started arrived in " + [string][math]::Round($turnStartedMs, 1) + "ms, budget " + [string]$BudgetMs + "ms")
