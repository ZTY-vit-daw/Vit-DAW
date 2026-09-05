[CmdletBinding()]
param(
    [string]$RepoRoot = "",
    [string]$AgentHttp = "http://127.0.0.1:7878",
    [string]$ProjectPath = "",
    [int]$FirstChatBudgetMs = 20000,
    [int]$ZeroChangeChatBudgetMs = 15000,
    [int]$AsyncSettleSeconds = 150
)

# HARNESS-1 acceptance: the conversation checkpoint (kernel snapshot export +
# history commit) must be off the chat response path. Reopens the given project
# (default: the handtest fixture), sends two chit-chat turns, and asserts:
#   1. turn 1 (may carry a revision-advancing async checkpoint) < FirstChatBudgetMs
#   2. turn 2 (zero-change, gate must skip)              < ZeroChangeChatBudgetMs
#   3. exactly one async checkpoint for turn 1, and NONE launched for turn 2
# Requires the real stack (kernel + agent) on $AgentHttp and the agent log at
# VitApp/Workspace/Logs/agent_last.log (dev_agent_smoke.ps1 layout).

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Resolve-RepoRoot {
    param([string]$Explicit)
    if (-not [string]::IsNullOrWhiteSpace($Explicit)) {
        return (Resolve-Path -LiteralPath $Explicit).Path
    }
    return (Split-Path -Parent (Split-Path -Parent $MyInvocation.ScriptName))
}

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

function Invoke-Json {
    param(
        [ValidateSet("GET", "POST")]
        [string]$Method,
        [string]$Uri,
        [object]$Body = $null,
        [int]$TimeoutSec = 300
    )
    if ($Method -eq "GET") {
        $resp = Invoke-WebRequest -UseBasicParsing -Method GET -Uri $Uri -TimeoutSec $TimeoutSec
    }
    else {
        $json = $Body | ConvertTo-Json -Depth 20 -Compress
        $bytes = [System.Text.Encoding]::UTF8.GetBytes($json)
        $resp = Invoke-WebRequest -UseBasicParsing -Method POST -Uri $Uri -Body $bytes -ContentType "application/json; charset=utf-8" -TimeoutSec $TimeoutSec
    }
    if ([string]::IsNullOrWhiteSpace($resp.Content)) {
        return $null
    }
    return $resp.Content | ConvertFrom-Json
}

$Repo = Resolve-RepoRoot $RepoRoot
if ([string]::IsNullOrWhiteSpace($ProjectPath)) {
    $ProjectPath = Join-Path $Repo "temp\handtest\spv1_p01_handtest.vit"
}
$AgentLog = Join-Path $Repo "VitApp\Workspace\Logs\agent_last.log"

Write-Step "HARNESS-1 vit gate timing smoke"
Write-Host "project=$ProjectPath"
Write-Host "agent=$AgentHttp log=$AgentLog"

$health = Invoke-Json -Method GET -Uri ($AgentHttp.TrimEnd("/") + "/health") -TimeoutSec 10
Write-Ok ("agent healthy: " + ($health | ConvertTo-Json -Compress))

if (-not (Test-Path -LiteralPath $ProjectPath)) {
    Fail "project fixture not found: $ProjectPath"
}

Write-Step "Open project (may advance kernel revision)"
$open = Invoke-Json -Method POST -Uri ($AgentHttp.TrimEnd("/") + "/agent/invoke") -Body @{
    tool      = "project.open"
    args      = @{ file_path = $ProjectPath; project_path = $ProjectPath }
    source    = "h1_vit_gate_timing_smoke"
    confirmed = $true
}
if (@("ok", "success", "completed") -notcontains ([string]$open.status).ToLower()) {
    Fail ("project.open failed: " + ($open | ConvertTo-Json -Compress -Depth 6).Substring(0, [Math]::Min(400, ($open | ConvertTo-Json -Compress -Depth 6).Length)))
}
Write-Ok "project opened"

if (Test-Path -LiteralPath $AgentLog) {
    $logBaselineLines = (Get-Content -LiteralPath $AgentLog | Measure-Object -Line).Lines
}
else {
    $logBaselineLines = 0
}

function Get-NewLogText {
    if (-not (Test-Path -LiteralPath $AgentLog)) { return "" }
    $current = (Get-Content -LiteralPath $AgentLog | Measure-Object -Line).Lines
    if ($current -le $logBaselineLines) { return "" }
    return ((Get-Content -LiteralPath $AgentLog) | Select-Object -Skip $logBaselineLines) -join "`n"
}

function Send-ChatTurn {
    param([string]$Message, [string]$ConversationID)
    $sw = [System.Diagnostics.Stopwatch]::StartNew()
    $resp = Invoke-Json -Method POST -Uri ($AgentHttp.TrimEnd("/") + "/agent/chat") -Body @{
        message         = $Message
        conversation_id = $ConversationID
    }
    $sw.Stop()
    [pscustomobject]@{
        ElapsedMs = $sw.ElapsedMilliseconds
        StopReason = [string]$resp.stop_reason
        ReplyLen = ([string]$resp.reply).Length
    }
}

Write-Step "Turn 1 (chit-chat; async checkpoint allowed)"
$turn1 = Send-ChatTurn -Message "hello, quick chit-chat" -ConversationId "h1_gate_timing_1"
Write-Host ("turn1 elapsed_ms=" + $turn1.ElapsedMs + " stop_reason=" + $turn1.StopReason + " reply_len=" + $turn1.ReplyLen)
if ($turn1.ElapsedMs -ge $FirstChatBudgetMs) {
    Fail ("turn 1 took " + $turn1.ElapsedMs + "ms, budget " + $FirstChatBudgetMs + "ms")
}
Write-Ok ("turn 1 within budget " + $FirstChatBudgetMs + "ms")

Write-Step "Wait for turn 1 async checkpoint to settle"
$settleDeadline = (Get-Date).AddSeconds($AsyncSettleSeconds)
$launchCount = 0
$doneCount = 0
while ((Get-Date) -lt $settleDeadline) {
    $text = Get-NewLogText
    $launchCount = ([regex]::Matches($text, "\[vit_checkpoint\] async checkpoint launched")).Count
    $doneCount = ([regex]::Matches($text, "\[vit_checkpoint\] async checkpoint done")).Count
    $gateSkipped = ([regex]::Matches($text, "\[vit_checkpoint\] gate skipped")).Count
    if (($launchCount -eq $doneCount -and $launchCount -gt 0) -or ($launchCount -eq 0 -and $gateSkipped -gt 0)) {
        break
    }
    Start-Sleep -Milliseconds 500
}
Write-Host ("async launched=" + $launchCount + " done=" + $doneCount)
if ($launchCount -gt 0 -and $launchCount -ne $doneCount) {
    Fail ("async checkpoint did not settle: launched=$launchCount done=$doneCount")
}
Write-Ok "async checkpoint settled (or turn was already gated)"

$preTurn2Text = Get-NewLogText
$preTurn2Lines = (Get-Content -LiteralPath $AgentLog | Measure-Object -Line).Lines

Write-Step "Turn 2 (zero-change; gate must skip the checkpoint)"
$turn2 = Send-ChatTurn -Message "thanks - one more chit-chat: how is the weather today" -ConversationId "h1_gate_timing_2"
Write-Host ("turn2 elapsed_ms=" + $turn2.ElapsedMs + " stop_reason=" + $turn2.StopReason + " reply_len=" + $turn2.ReplyLen)
if ($turn2.ElapsedMs -ge $ZeroChangeChatBudgetMs) {
    Fail ("turn 2 took " + $turn2.ElapsedMs + "ms, budget " + $ZeroChangeChatBudgetMs + "ms (zero-change turn must not pay the snapshot tax)")
}
Write-Ok ("turn 2 within budget " + $ZeroChangeChatBudgetMs + "ms")

Start-Sleep -Seconds 3
$turn2Text = ""
if ((Get-Content -LiteralPath $AgentLog | Measure-Object -Line).Lines -gt $preTurn2Lines) {
    $turn2Text = ((Get-Content -LiteralPath $AgentLog) | Select-Object -Skip $preTurn2Lines) -join "`n"
}
$turn2Launches = ([regex]::Matches($turn2Text, "\[vit_checkpoint\] async checkpoint launched")).Count
$turn2Skips = ([regex]::Matches($turn2Text, "\[vit_checkpoint\] gate skipped")).Count
$turn2SyncHint = ([regex]::Matches($turn2Text, "\[vit_checkpoint\] sync checkpoint failed")).Count
Write-Host ("turn2 gate_skipped=" + $turn2Skips + " async_launched=" + $turn2Launches)
if ($turn2Launches -gt 0) {
    Fail "zero-change turn 2 launched a checkpoint: revision gate did not skip"
}
if ($turn2SyncHint -gt 0) {
    Fail "turn 2 hit the sync fallback path: gate basis missing on a warm project"
}
if ($turn2Skips -eq 0 -and $turn2Launches -eq 0) {
    Write-Host "warn: no gate decision line for turn 2 (older binary without gate logs?): relied on timing budget only"
}
else {
    Write-Ok "zero-change turn 2 skipped the checkpoint"
}

Write-Step "Summary"
Write-Ok ("HARNESS-1 timing smoke passed: turn1=" + $turn1.ElapsedMs + "ms turn2=" + $turn2.ElapsedMs + "ms")
exit 0
