<#
TRAJ-DUAL-1 acceptance probe: dual trajectory turn shells must collapse into a
single turn block, and every turn shell must be closed by a turn-family terminal.

Reconnaissance context (queue/reports/2026-09-13-TRAJ-DUAL-1.md):
the free-state experiment turn shell (payload.turn_id = turn:{loopID}) is opened
by experiment.Turn.StartEvents at admission, but its only turn-family terminal
route was Turn.Stop(). The normal settlement path emits trajectory.settled
(node_kind=settlement), and the webui round-scoped turn closes ONLY on
trajectory.turn.completed/failed/stopped - so a turn whose message-scope shell
was suppressed by the AGENT-F1 gate could never be closed by the stream.
chat.turn now closes the free-state shell at the same transport terminal.

Assertions (on the captured events.json, mirroring the webui consumer):
  1. single block: every trajectory event resolves to ONE turn key
     (source_turn_id -> payload.turn_id -> trajectory_turn_id -> turn_id ->
      run_id -> goal_id, the B9 unified surface).
  2. terminal closure: every turn shell (payload.node_kind = turn) that emitted
     trajectory.turn.started also has a trajectory.turn.completed/failed/stopped
     under the same payload turn_id. Suspended shells = 0.

Set -RequireFreeStateExperiment to also demand that the turn reached a
turn:free_state_* shell (the card's exact shape). Without a reachable kernel and
a configured model the free-state limb is not admitted, so that switch is off by
default and the script reports the limb as not-reached instead of failing.

Method notes (learned on this host, same as f4_turn_started_timing_smoke.ps1):
  * The chat POST is written over a raw TCP socket: HttpWebRequest/HttpClient
    block until the whole turn is answered and starve the concurrent polling.
  * This file is deliberately pure ASCII: Windows PowerShell 5.1 reads .ps1
    without a BOM using the system ANSI code page.

Requires the real stack (agent on the AgentHttp endpoint). PASS = exit code 0.
#>
[CmdletBinding()]
param(
    [string]$AgentHttp = "http://127.0.0.1:7878",
    [string]$ConversationId = "",
    [string]$Message = "",
    [int]$TimeoutSeconds = 180,
    [int]$PollIntervalMs = 100,
    [int]$TerminalGraceSeconds = 60,
    [string]$AgentLogPath = "",
    [string]$ArtifactDir = "",
    [switch]$RequireFreeStateExperiment
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

function Write-Info {
    param([string]$Message)
    Write-Host ("info: " + $Message) -ForegroundColor Gray
}

function Fail {
    param([string]$Message)
    Write-Host ("FAIL: " + $Message) -ForegroundColor Red
    exit 1
}

# "make the current mix better" - built from code points, never literal CJK bytes.
$defaultMessage = -join @(
    [char]0x628A, [char]0x5F53, [char]0x524D, [char]0x5DE5, [char]0x7A0B,
    [char]0x6DF7, [char]0x5F97, [char]0x66F4, [char]0x597D, [char]0x4E00,
    [char]0x70B9
)

# --- consumer-mirroring helpers -------------------------------------------

function Get-EventTurnKey {
    param($Event)
    # B9 unified surface: source_turn_id first, then the legacy attribution chain.
    if ($Event.source_turn_id) { return [string]$Event.source_turn_id }
    $payloadTurn = ""
    if ($Event.payload -and ($Event.payload.PSObject.Properties.Name -contains "turn_id")) { $payloadTurn = [string]$Event.payload.turn_id }
    if ($payloadTurn) { return $payloadTurn }
    if ($Event.trajectory_turn_id) { return [string]$Event.trajectory_turn_id }
    if ($Event.turn_id) { return [string]$Event.turn_id }
    if ($Event.run_id) { return [string]$Event.run_id }
    if ($Event.goal_id) { return [string]$Event.goal_id }
    return ""
}

function Get-ShellTurnId {
    param($Event)
    # A turn shell is a trajectory node with node_kind=turn; its identity is the
    # payload turn id (message-scope shells use the bare runID, free-state
    # experiment shells use turn:{loopID}).
    if ([string]$Event.type -notlike "trajectory.*") { return "" }
    if (-not $Event.payload) { return "" }
    if (-not ($Event.payload.PSObject.Properties.Name -contains "node_kind")) { return "" }
    if ([string]$Event.payload.node_kind -ne "turn") { return "" }
    if (-not ($Event.payload.PSObject.Properties.Name -contains "turn_id")) { return "" }
    return [string]$Event.payload.turn_id
}

function Test-IsShellTerminal {
    param($Event)
    $t = [string]$Event.type
    return ($t -eq "trajectory.turn.completed" -or $t -eq "trajectory.turn.failed" -or $t -eq "trajectory.turn.stopped")
}

# --- setup ----------------------------------------------------------------

$repoRoot = if ($PSScriptRoot) { Split-Path -Parent $PSScriptRoot } else { (Get-Location).Path }
if ([string]::IsNullOrWhiteSpace($Message)) { $Message = $defaultMessage }
if ([string]::IsNullOrWhiteSpace($ConversationId)) {
    $ConversationId = "traj_dual1_" + (Get-Date -Format "yyyyMMdd_HHmmss")
}
if ([string]::IsNullOrWhiteSpace($AgentLogPath)) {
    $AgentLogPath = Join-Path $repoRoot "VitApp\Workspace\Logs\agent_last.log"
}
if ([string]::IsNullOrWhiteSpace($ArtifactDir)) {
    $ArtifactDir = Join-Path $repoRoot ("artifacts\traj_dual1\" + (Get-Date -Format "yyyyMMdd_HHmmss"))
}
New-Item -ItemType Directory -Force -Path $ArtifactDir | Out-Null

$base = $AgentHttp.TrimEnd("/")
$uri = [uri]$base
$hostName = $uri.Host
$port = $uri.Port
Write-Host "== TRAJ-DUAL-1 turn shell closure probe"
Write-Host ("agent http: " + $base)
Write-Host ("conversation: " + $ConversationId)
Write-Host ("artifact dir: " + $ArtifactDir)

Write-Step "Agent reachability (read-only GET)"
try {
    [void](Invoke-RestMethod -Method GET -Uri ($base + "/agent/runtime/status") -TimeoutSec 10)
} catch {
    Fail ("agent /agent/runtime/status unreachable: " + $_.Exception.Message)
}
Write-Ok "agent runtime status readable"

$eventsUri = $base + "/agent/events?conversation_id=" + [uri]::EscapeDataString($ConversationId) + "&since=0&limit=400"
[void](Invoke-RestMethod -Method GET -Uri $eventsUri -TimeoutSec 10)

Write-Step "Send the first message of the conversation over a raw TCP socket"
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
$sentAt = (Get-Date).ToString("yyyy-MM-ddTHH:mm:ss.fffzzz")
try {
    $netStream.Write($headBytes, 0, $headBytes.Length)
    $netStream.Write($bodyBytes, 0, $bodyBytes.Length)
    $netStream.Flush()
} catch {
    Fail ("chat request could not be written: " + $_.Exception.Message)
}
Write-Host ("sent_at=" + $sentAt)

Write-Step "Poll /agent/events until the turn terminal lands"
$observedEvents = @()
$sawTurnStarted = $false
$sawTerminal = $false
$deadline = (Get-Date).AddSeconds($TimeoutSeconds)
$terminalDeadline = $null
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
            if ([string]$row.type -eq "turn.started") { $sawTurnStarted = $true }
            if (Test-IsShellTerminal $row) { $sawTerminal = $true }
        }
    }
    if ($sawTerminal) { break }
    if ($sawTurnStarted -and $null -eq $terminalDeadline) {
        $terminalDeadline = (Get-Date).AddSeconds($TerminalGraceSeconds)
    }
    if ($null -ne $terminalDeadline -and (Get-Date) -gt $terminalDeadline) { break }
    Start-Sleep -Milliseconds $PollIntervalMs
}

# Drain the response, then take a final authoritative read.
try {
    $buffer = New-Object byte[] 8192
    while ($netStream.Read($buffer, 0, $buffer.Length) -gt 0) { }
} catch { }
try { $tcp.Close() } catch { }
Start-Sleep -Milliseconds 400
try {
    $final = Invoke-RestMethod -Method GET -Uri $eventsUri -TimeoutSec 10
    if ($final -and ($final.PSObject.Properties.Name -contains "events") -and $final.events) { $observedEvents = @($final.events) }
} catch { }

$eventsPath = Join-Path $ArtifactDir "traj_dual1_shell_closure_events.json"
$observedEvents | ConvertTo-Json -Depth 14 | Set-Content -Path $eventsPath -Encoding UTF8
Write-Host ("artifact: " + $eventsPath)
Write-Host ("events: " + [string]$observedEvents.Count)

if (-not $sawTurnStarted) {
    Fail ("no turn.started observed within " + [string]$TimeoutSeconds + "s for conversation " + $ConversationId)
}

# --- assertions -----------------------------------------------------------

Write-Step "Assertion 1: single turn block (one turn key, B9 unified surface)"
$turnKeys = @()
foreach ($row in $observedEvents) {
    if ([string]$row.type -notlike "trajectory.*") { continue }
    $key = Get-EventTurnKey $row
    if ($key -and ($turnKeys -notcontains $key)) { $turnKeys += $key }
}
Write-Host ("turn keys: " + ($turnKeys -join ", "))
if ($turnKeys.Count -ne 1) {
    Fail ("trajectory events resolved to " + [string]$turnKeys.Count + " turn blocks, want exactly 1: " + ($turnKeys -join ", "))
}
Write-Ok ("one trajectory turn block: " + $turnKeys[0])

Write-Step "Assertion 2: terminal closure (zero suspended turn shells)"
$openedNodeIds = @()
$closedNodeIds = @()
$freeStateSeen = $false
foreach ($row in $observedEvents) {
    $nodeTurnId = Get-ShellTurnId $row
    if (-not $nodeTurnId) { continue }
    if ($nodeTurnId.StartsWith("turn:free_state_")) { $freeStateSeen = $true }
    if ([string]$row.type -eq "trajectory.turn.started") {
        if ($openedNodeIds -notcontains $nodeTurnId) { $openedNodeIds += $nodeTurnId }
        continue
    }
    if (Test-IsShellTerminal $row) {
        if ($closedNodeIds -notcontains $nodeTurnId) { $closedNodeIds += $nodeTurnId }
    }
}
Write-Host ("shells opened: " + ($openedNodeIds -join ", "))
Write-Host ("shells closed: " + ($closedNodeIds -join ", "))
$suspended = @()
foreach ($nodeTurnId in $openedNodeIds) { if ($closedNodeIds -notcontains $nodeTurnId) { $suspended += $nodeTurnId } }
if ($openedNodeIds.Count -eq 0) {
    Fail "no trajectory turn shell observed at all; the probe cannot judge closure"
}
if ($suspended.Count -gt 0) {
    Fail ("suspended turn shells after the turn terminal: " + ($suspended -join ", "))
}
Write-Ok ("every turn shell closed (" + [string]$openedNodeIds.Count + " shell(s), 0 suspended)")

Write-Step "Free-state experiment limb (informational)"
if ($freeStateSeen) {
    Write-Ok "free-state experiment shell turn:free_state_* was admitted AND closed in this turn"
} else {
    Write-Info "no turn:free_state_* shell in this turn (free-state experiment not admitted)"
    if ($RequireFreeStateExperiment) {
        Fail "-RequireFreeStateExperiment set, but the turn never reached the free-state experiment limb"
    }
}

Write-Host ""
Write-Host "== Summary"
Write-Ok "TRAJ-DUAL-1 turn shell closure probe completed (single block + terminal closure)"
exit 0
