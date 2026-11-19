[CmdletBinding()]
param(
    # W02-A1c real-stack executor. Zero-stack by default: no stack start, no port.
    [string]$RepoRoot = "D:/Vit_DAW",
    [string]$AgentBaseUrl = "http://127.0.0.1:7878",
    [string]$RuntimeOwner = "",
    [int]$TimeoutSeconds = 180,
    [string]$RunRoot = "",
    [switch]$StackAuthorized,
    [switch]$DryRun
)

# ============================================================================
# W02-A1c real-stack journey executor (S0-S6)
#
# Card: coord/cards/doing/2026-11-19-W02-A1-LESSON-CTX-CLIP-1.md (A1c)
# Scope: selected-MIDI-clip binding + bounded lesson_context + one real LLM turn
#        + honest failure semantics + reopen/recovery evidence.
#
# Exit codes:
#   0 = PASS (all asserted stages passed; teardown restored)
#   1 = deterministic assertion failure (product/contract)
#   2 = preflight/environment/teardown failure (stack, HTTP, restore)
#   3 = not authorized (runbook summary only; no HTTP, no writes)
#   4 = real-LLM failure class (timeout/empty/stop_reason/telemetry error)
#   5 = evidence/API-coverage gap (no fake PASS)
#
# The script NEVER starts, restarts or stops a stack and never listens on a
# port. It only uses the already-running agent HTTP face and reads log files.
# Raw run artifacts are written under -RunRoot only; the three authoritative
# user files (recents / Settings.xml / default_project.xml) are snapshotted and
# restored byte-identically, and /agent/authority is snapshotted and restored.
# ============================================================================

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$script:exitCode = 0
$script:failureClass = ""
$script:failureMessage = ""
$script:statePoll = 0
$script:requestSeq = 0

function Write-Step { param([string]$Message) Write-Host ""; Write-Host ("== " + $Message) -ForegroundColor Cyan }
function Write-Ok { param([string]$Message) Write-Host ("ok: " + $Message) -ForegroundColor Green }
function Write-WarnLine { param([string]$Message) Write-Host ("warn: " + $Message) -ForegroundColor Yellow }

function Fail {
    param([string]$Class, [string]$Message)
    throw ("W02A1C|" + $Class + "|" + $Message)
}

function Get-Prop {
    param($Object, [string]$Name)
    if ($null -eq $Object) { return $null }
    if ($Object -is [System.Collections.IDictionary]) {
        if ($Object.Contains($Name)) { return $Object[$Name] }
        return $null
    }
    $prop = $Object.PSObject.Properties[$Name]
    if ($null -eq $prop) { return $null }
    return $prop.Value
}

function Get-PropOr {
    param($Object, [string]$Name, $Default)
    $value = Get-Prop $Object $Name
    if ($null -eq $value) { return $Default }
    return $value
}

# Extract the JSON object value of one named property from a text blob. The
# prompt render may embed the model context snapshot inside system_prompt or
# turn_directives as pretty-printed JSON; brace matching must respect strings.
function Get-JsonObjectForProperty {
    param([string]$Text, [string]$Property)
    if ([string]::IsNullOrEmpty($Text)) { return $null }
    $needle = '"' + $Property + '"'
    $searchFrom = 0
    while ($searchFrom -lt $Text.Length) {
        $idx = $Text.IndexOf($needle, $searchFrom)
        if ($idx -lt 0) { return $null }
        $colon = $Text.IndexOf([char]58, $idx + $needle.Length)
        if ($colon -ge 0) {
            $afterColon = $Text.Substring($colon + 1, [Math]::Min(80, $Text.Length - $colon - 1)).TrimStart()
            if ($afterColon.StartsWith("{") -or $afterColon.StartsWith("[")) {
                $start = $Text.IndexOf([char]123, $colon + 1)
                if ($start -ge 0) {
                    $depth = 0
                    $inString = $false
                    $escape = $false
                    for ($i = $start; $i -lt $Text.Length; $i++) {
                        $ch = $Text[$i]
                        if ($inString) {
                            if ($escape) { $escape = $false }
                            elseif ($ch -eq [char]92) { $escape = $true }
                            elseif ($ch -eq [char]34) { $inString = $false }
                        }
                        else {
                            if ($ch -eq [char]34) { $inString = $true }
                            elseif ($ch -eq [char]123) { $depth++ }
                            elseif ($ch -eq [char]125) {
                                $depth--
                                if ($depth -eq 0) { return $Text.Substring($start, $i - $start + 1) }
                            }
                        }
                    }
                }
            }
        }
        $searchFrom = $idx + $needle.Length
    }
    return $null
}

# Search an already-parsed JSON object for a non-null lesson_context property,
# including nested context_snapshot/model_snapshot objects.
function Find-LessonContextInObject {
    param($Object, [int]$Depth = 0)
    if ($null -eq $Object -or $Depth -gt 6) { return $null }
    $direct = Get-Prop $Object "lesson_context"
    if ($null -ne $direct) { return $direct }
    if ($Object -is [System.Collections.IDictionary]) {
        foreach ($key in $Object.Keys) {
            $value = $Object[$key]
            if ($value -is [System.Collections.IDictionary] -or $value -is [System.Management.Automation.PSCustomObject]) {
                $found = Find-LessonContextInObject -Object $value -Depth ($Depth + 1)
                if ($null -ne $found) { return $found }
            }
        }
        return $null
    }
    if ($Object -is [System.Management.Automation.PSCustomObject]) {
        foreach ($prop in $Object.PSObject.Properties) {
            if ($prop.Name -eq "lesson_context" -and $null -ne $prop.Value) { return $prop.Value }
            $value = $prop.Value
            if ($value -is [System.Collections.IDictionary] -or $value -is [System.Management.Automation.PSCustomObject]) {
                $found = Find-LessonContextInObject -Object $value -Depth ($Depth + 1)
                if ($null -ne $found) { return $found }
            }
        }
    }
    return $null
}

# Parse one agent_prompt_render.jsonl row and return the bounded lesson context
# if the row (or any embedded prompt string) carries it. Returns $null when the
# row exists but does not contain the model snapshot lesson overlay.
function Get-LessonContextFromPromptRenderRow {
    param([string]$Line)
    if ([string]::IsNullOrWhiteSpace($Line)) { return $null }
    $row = $null
    try { $row = $Line | ConvertFrom-Json } catch { return $null }
    $direct = Get-Prop $row "lesson_context"
    if ($null -ne $direct) {
        return [pscustomobject]@{ lesson = $direct; field = "lesson_context" }
    }
    foreach ($prop in $row.PSObject.Properties) {
        $value = $prop.Value
        if ($value -is [string]) {
            $json = Get-JsonObjectForProperty -Text $value -Property "lesson_context"
            if (-not [string]::IsNullOrWhiteSpace($json)) {
                try {
                    $lesson = $json | ConvertFrom-Json
                    return [pscustomobject]@{ lesson = $lesson; field = $prop.Name }
                }
                catch { }
            }
        }
        elseif ($value -is [System.Collections.IDictionary] -or $value -is [System.Management.Automation.PSCustomObject]) {
            $found = Find-LessonContextInObject -Object $value
            if ($null -ne $found) {
                return [pscustomobject]@{ lesson = $found; field = $prop.Name }
            }
        }
    }
    return $null
}

# Read only newly appended prompt-render lines and return the last row for this
# conversation that actually carries lesson_context. The file is huge, so the
# shared FileStream reader is used; it never loads the whole file.
function Find-PromptRenderLesson {
    param([string]$Path, [long]$Offset, [string]$ConversationId)
    $text = Read-AppendedText -Path $Path -Offset $Offset -MaxBytes 16777216
    if ($null -eq $text) { return $null }
    $match = $null
    $lineCount = 0
    $lastConversationLine = ""
    foreach ($line in [regex]::Split($text, '\r?\n')) {
        $trimmed = $line.TrimStart([char]0xFEFF).Trim()
        if ($trimmed -eq "") { continue }
        try { $row = $trimmed | ConvertFrom-Json } catch { continue }
        if ([string](Get-Prop $row "conversation_id") -ne $ConversationId) { continue }
        $lineCount++
        $lastConversationLine = $trimmed
        $found = Get-LessonContextFromPromptRenderRow -Line $trimmed
        if ($null -ne $found) {
            $match = [pscustomobject]@{
                line = $trimmed
                row = $row
                lesson = $found.lesson
                field = $found.field
                appended = $text
                conversation_lines = $lineCount
            }
        }
    }
    if ($null -eq $match) {
        return [pscustomobject]@{ line = $lastConversationLine; row = $null; lesson = $null; field = ""; appended = $text; conversation_lines = $lineCount }
    }
    return $match
}

function Get-Bool {
    param($Value)
    if ($null -eq $Value) { return $false }
    if ($Value -is [bool]) { return $Value }
    $text = ([string]$Value).Trim().ToLowerInvariant()
    return ($text -eq "true" -or $text -eq "1" -or $text -eq "yes")
}

function Test-InactiveGoalStatus {
    param([string]$Status)
    # Mirror authorityModeChangeBlocked's non-active set: these states do not
    # represent an Agent Turn that could race this journey.
    return (@("idle", "completed", "failed", "cancelled", "stopped", "stable") -contains $Status)
}

function Assert-True {
    param([bool]$Condition, [string]$Message)
    if (-not $Condition) { Fail "assert" $Message }
}

function Format-Result($Response) {
    if ($null -eq $Response) { return "<null>" }
    try { return ($Response | ConvertTo-Json -Depth 12 -Compress) } catch { return [string]$Response }
}

function Get-FileSha256 {
    param([string]$Path)
    if ([string]::IsNullOrWhiteSpace($Path) -or -not (Test-Path -LiteralPath $Path)) { return "" }
    return (Get-FileHash -Algorithm SHA256 -LiteralPath $Path).Hash
}

function Get-FileLength {
    param([string]$Path)
    if ([string]::IsNullOrWhiteSpace($Path) -or -not (Test-Path -LiteralPath $Path)) { return [long]0 }
    return [long](Get-Item -LiteralPath $Path).Length
}

function Read-AppendedText {
    param([string]$Path, [long]$Offset, [int]$MaxBytes = 2097152)
    if ([string]::IsNullOrWhiteSpace($Path) -or -not (Test-Path -LiteralPath $Path)) { return $null }
    $fs = $null
    try {
        $fs = [System.IO.File]::Open($Path, [System.IO.FileMode]::Open, [System.IO.FileAccess]::Read, [System.IO.FileShare]::ReadWrite)
        $length = $fs.Length
        if ($length -le $Offset) { return "" }
        $toRead = [int][Math]::Min([double]$MaxBytes, [double]($length - $Offset))
        if ($toRead -le 0) { return "" }
        [void]$fs.Seek($Offset, [System.IO.SeekOrigin]::Begin)
        $buffer = New-Object byte[] $toRead
        $read = $fs.Read($buffer, 0, $toRead)
        return [System.Text.Encoding]::UTF8.GetString($buffer, 0, $read)
    }
    finally {
        if ($null -ne $fs) { $fs.Dispose() }
    }
}

function Read-JsonFile {
    param([string]$Path)
    $raw = Get-Content -LiteralPath $Path -Raw -Encoding UTF8
    return ($raw | ConvertFrom-Json)
}

function Get-RecentsPath {
    if (-not [string]::IsNullOrWhiteSpace($env:VIT_DAW_USER_CONFIG_DIR)) {
        return (Join-Path $env:VIT_DAW_USER_CONFIG_DIR "global_project_config.json")
    }
    if (-not [string]::IsNullOrWhiteSpace($env:APPDATA)) {
        return (Join-Path (Join-Path $env:APPDATA "Vit-DAW") "global_project_config.json")
    }
    return ""
}

function Get-AgentLogPath {
    param([string]$EnvName, [string]$FileName)
    $override = [Environment]::GetEnvironmentVariable($EnvName)
    if (-not [string]::IsNullOrWhiteSpace($override)) {
        if ($override -eq "off" -or $override -eq "disabled") { return "" }
        return $override
    }
    return (Join-Path $RepoRoot ("VitApp/Workspace/Logs/" + $FileName))
}

function Get-KernelLogInfo {
    $logsDir = Join-Path $RepoRoot "VitApp/Workspace/Logs"
    if (-not (Test-Path -LiteralPath $logsDir)) { return $null }
    $log = Get-ChildItem -LiteralPath $logsDir -Filter "VitHeadlessServer*.log" -File -ErrorAction SilentlyContinue |
        Sort-Object LastWriteTime -Descending | Select-Object -First 1
    if ($null -eq $log) { return $null }
    return [pscustomobject]@{ path = $log.FullName; length = [long]$log.Length; last_write = $log.LastWriteTime.ToString("o") }
}

function Snapshot-File {
    param([string]$Path, [string]$Name)
    $info = [ordered]@{
        name = $Name
        path = $Path
        existed = $false
        sha256_before = ""
        snapshot = ""
    }
    if ([string]::IsNullOrWhiteSpace($Path)) { return $info }
    if (Test-Path -LiteralPath $Path) {
        try {
            $info.existed = $true
            $info.sha256_before = Get-FileSha256 $Path
            $dest = Join-Path $snapshotDir ($Name + ".before")
            Copy-Item -LiteralPath $Path -Destination $dest -Force
            $info.snapshot = $dest
        }
        catch {
            Fail "preflight" ("could not snapshot " + $Path + ": " + $_.Exception.Message)
        }
    }
    return $info
}

function Restore-File {
    param([object]$Info)
    $result = [ordered]@{
        name = [string](Get-Prop $Info "name")
        path = [string](Get-Prop $Info "path")
        restored = $false
        hash_before = [string](Get-Prop $Info "sha256_before")
        hash_after = ""
        error = ""
    }
    try {
        if ($result.path -eq "") { $result.restored = $true; return $result }
        if (Get-Bool (Get-Prop $Info "existed")) {
            $snapshot = [string](Get-Prop $Info "snapshot")
            if (-not (Test-Path -LiteralPath $snapshot)) { throw ("snapshot missing: " + $snapshot) }
            $parent = Split-Path -Parent $result.path
            if (-not [string]::IsNullOrWhiteSpace($parent)) { New-Item -ItemType Directory -Path $parent -Force | Out-Null }
            Copy-Item -LiteralPath $snapshot -Destination $result.path -Force
        }
        else {
            if (Test-Path -LiteralPath $result.path) { Remove-Item -LiteralPath $result.path -Force }
        }
        $result.hash_after = Get-FileSha256 $result.path
        if (Get-Bool (Get-Prop $Info "existed")) {
            if ($result.hash_after -ne $result.hash_before) { throw ("hash mismatch after restore: " + $result.path) }
        }
        else {
            if (Test-Path -LiteralPath $result.path) { throw ("file was recreated but existed=false: " + $result.path) }
        }
        $result.restored = $true
    }
    catch {
        $result.error = $_.Exception.Message
    }
    return $result
}

function Invoke-Api {
    param([string]$Name, [string]$Method, [string]$Path, $Body, [int]$TimeoutSec = 0)
    $uri = ($AgentBaseUrl.TrimEnd("/") + $Path)
    if ($TimeoutSec -le 0) { $TimeoutSec = [Math]::Max(1, $TimeoutSeconds) }
    $script:requestSeq++
    $safeName = ("{0:D3}_{1}" -f $script:requestSeq, $Name)
    $requestFile = Join-Path (Join-Path $RunRoot "requests") ($safeName + ".json")
    $responseFile = Join-Path (Join-Path $RunRoot "responses") ($safeName + ".json")
    try {
        if ($null -ne $Body) {
            $json = ConvertTo-Json -InputObject $Body -Depth 40 -Compress
            $utf8 = New-Object System.Text.UTF8Encoding($false)
            [System.IO.File]::WriteAllText($requestFile, $json, $utf8)
            $bytes = [System.Text.Encoding]::UTF8.GetBytes($json)
            $response = Invoke-RestMethod -Method $Method -Uri $uri -ContentType "application/json; charset=utf-8" -Body $bytes -TimeoutSec $TimeoutSec
        }
        else {
            $response = Invoke-RestMethod -Method $Method -Uri $uri -TimeoutSec $TimeoutSec
        }
    }
    catch {
        $message = $_.Exception.Message
        Fail "http" ($Name + " " + $Method + " " + $Path + " failed: " + $message)
    }
    try {
        $responseJson = ConvertTo-Json -InputObject $response -Depth 40
        Set-Content -LiteralPath $responseFile -Value $responseJson -Encoding UTF8
    }
    catch {
        Write-WarnLine ("could not persist response for " + $Name + ": " + $_.Exception.Message)
    }
    return $response
}

function Invoke-AgentChat {
    param([string]$Name, [string]$ConversationID, [string]$Message, $Context, [int]$TimeoutSec = 0)
    if ($null -eq $Context) { $Context = @{} }
    $body = [ordered]@{
        conversation_id = $ConversationID
        message = $Message
        context = $Context
    }
    return (Invoke-Api -Name $Name -Method "Post" -Path "/agent/chat" -Body $body -TimeoutSec $TimeoutSec)
}

function Invoke-LlmChat {
    param([string]$Name, [string]$ConversationID, [string]$Message, $Context, [int]$TimeoutSec = 0)
    try {
        return (Invoke-AgentChat -Name $Name -ConversationID $ConversationID -Message $Message -Context $Context -TimeoutSec $TimeoutSec)
    }
    catch {
        $messageText = $_.Exception.Message
        if ($messageText -match "(?i)timed? ?out|timeout|TaskCanceled|operation has timed out") {
            Fail "llm" ("LLM call timed out: " + $messageText)
        }
        throw
    }
}

function Invoke-Tool {
    param([string]$Name, [string]$Tool, $ToolArgs, [bool]$Confirmed = $true, [int]$TimeoutSec = 0)
    if ($null -eq $ToolArgs) { $ToolArgs = @{} }
    $body = [ordered]@{
        tool = $Tool
        args = $ToolArgs
        source = "w02_a1c_smoke"
    }
    if ($Confirmed) { $body["confirmed"] = $true }
    $response = Invoke-Api -Name $Name -Method "Post" -Path "/agent/invoke" -Body $body -TimeoutSec $TimeoutSec
    $status = [string](Get-Prop $response "status")
    if ($status -eq "needs_confirmation") {
        Fail "api" ($Tool + " returned needs_confirmation even though confirmed=true: " + (Format-Result $response))
    }
    if ($status -ne "ok") {
        $errorText = [string](Get-Prop $response "error")
        $result = Get-Prop $response "result"
        $resultError = [string](Get-Prop $result "error")
        Fail "api" ($Tool + " status=" + $status + " error=" + $errorText + " result_error=" + $resultError + " response=" + (Format-Result $response))
    }
    return $response
}

function Get-Result {
    param($Response)
    $result = Get-Prop $Response "result"
    if ($null -eq $result) { return $Response }
    return $result
}

function Get-WorkflowData {
    param($Response)
    return (Get-Prop $Response "workflow_data")
}

function Get-CurrentNode {
    param($Response)
    return [string](Get-Prop (Get-WorkflowData $Response) "current_node")
}

function Get-ClipIdForSlot {
    param($WorkflowData, [string]$Slot)
    $ids = Get-Prop $WorkflowData "clip_ids"
    if ($null -eq $ids) { return "" }
    $value = Get-Prop $ids $Slot
    if ($null -eq $value) { return "" }
    return [string]$value
}

function Wait-ActiveProject {
    param([string]$Path, [int]$Seconds)
    $deadline = (Get-Date).AddSeconds($Seconds)
    $wanted = [System.IO.Path]::GetFullPath($Path).ToLowerInvariant()
    while ((Get-Date) -lt $deadline) {
        Start-Sleep -Milliseconds 500
        $script:statePoll++
        $state = Invoke-Api -Name ("state_wait_" + $script:statePoll) -Method "Get" -Path "/agent/state" -Body $null
        $current = Get-Prop $state "current_project"
        $active = [string](Get-Prop $current "path")
        if ([string]::IsNullOrWhiteSpace($active)) {
            $history = Get-Prop $state "project_history"
            $active = [string](Get-Prop $history "project_path")
        }
        if ([string]::IsNullOrWhiteSpace($active)) {
            $shadowState = Get-Prop $state "shadow"
            $active = [string](Get-Prop $shadowState "project_path")
        }
        if (-not [string]::IsNullOrWhiteSpace($active)) {
            try {
                if ([System.IO.Path]::GetFullPath($active).ToLowerInvariant() -eq $wanted) { return $state }
            }
            catch { }
        }
    }
    Fail "preflight" ("project did not become active within " + $Seconds + "s: " + $Path)
}

function Find-ProjectPackage {
    param([string]$ProjectPath)
    $dir = Split-Path -Parent $ProjectPath
    $base = [System.IO.Path]::GetFileNameWithoutExtension($ProjectPath)
    $candidates = @(
        (Join-Path $dir ".vit_project"),
        (Join-Path $dir ($base + ".vit_project"))
    )
    foreach ($candidate in $candidates) {
        if (Test-Path -LiteralPath (Join-Path $candidate "manifest.json")) { return $candidate }
    }
    return ""
}

function Read-CompiledCard {
    param([string]$Path)
    $lines = @(Get-Content -LiteralPath $Path -Encoding UTF8)
    $slots = @{}
    $nodes = @{}
    $section = ""
    $slotName = ""
    $currentNode = ""
    $inDoneWhen = $false
    foreach ($line in $lines) {
        $m = [regex]::Match($line, '^([A-Za-z0-9_\-]+):\s*$')
        if ($m.Success) {
            $section = $m.Groups[1].Value
            $slotName = ""
            $currentNode = ""
            $inDoneWhen = $false
            continue
        }
        if ($section -eq "runtime_slots") {
            $m = [regex]::Match($line, '^    ([A-Za-z0-9_\-]+):\s*$')
            if ($m.Success) {
                $slotName = $m.Groups[1].Value
                $slots[$slotName] = @{ track_index = 0; kind = "" }
                continue
            }
            if ($slotName -ne "") {
                $m = [regex]::Match($line, '^        track_index:\s*(\d+)')
                if ($m.Success) { $slots[$slotName]["track_index"] = [int]$m.Groups[1].Value; continue }
                $m = [regex]::Match($line, '^        kind:\s*(\S+)')
                if ($m.Success) { $slots[$slotName]["kind"] = $m.Groups[1].Value; continue }
            }
            continue
        }
        if ($section -eq "nodes") {
            $m = [regex]::Match($line, '^    - id:\s*(\S+)')
            if ($m.Success) {
                $currentNode = $m.Groups[1].Value
                $nodes[$currentNode] = @{ event = ""; clip = ""; track = ""; structure = ""; unsupported = $false }
                $inDoneWhen = $false
                continue
            }
            if ($currentNode -eq "") { continue }
            if (-not $inDoneWhen) {
                $m = [regex]::Match($line, '^      done_when:\s*$')
                if ($m.Success) { $inDoneWhen = $true }
                continue
            }
            $m = [regex]::Match($line, '^        (event|clip|track|structure):\s*(\S+)')
            if ($m.Success) {
                $nodes[$currentNode][$m.Groups[1].Value] = $m.Groups[2].Value
                continue
            }
            $m = [regex]::Match($line, '^        (all|any):')
            if ($m.Success) { $nodes[$currentNode]["unsupported"] = $true; continue }
            $m = [regex]::Match($line, '^      [A-Za-z_]+:')
            if ($m.Success) { $inDoneWhen = $false; continue }
        }
    }
    return @{ slots = $slots; nodes = $nodes }
}

function Test-UserTrack {
    param($Track)
    $serviceKinds = @("master", "arranger", "chord", "marker", "tempo")
    $kind = ([string](Get-Prop $Track "track_type")).Trim().ToLowerInvariant()
    if ($kind -eq "") { $kind = ([string](Get-Prop $Track "type")).Trim().ToLowerInvariant() }
    if ($serviceKinds -contains $kind) { return $false }
    $name = ([string](Get-Prop $Track "track_name")).Trim().ToLowerInvariant()
    if ($name -eq "") { $name = ([string](Get-Prop $Track "name")).Trim().ToLowerInvariant() }
    if ($serviceKinds -contains $name) { return $false }
    $isAudio = (Get-Bool (Get-Prop $Track "is_audio_track")) -or (Get-Bool (Get-Prop $Track "is_audio"))
    if ($isAudio) { return $true }
    $vitType = ([string](Get-Prop $Track "vit_type")).Trim().ToLowerInvariant()
    return (@("audio", "midi", "bus", "ghost") -contains $vitType)
}

function Get-TracksFromState {
    param($ProjectState)
    $result = Get-Result $ProjectState
    $tracks = Get-Prop $result "tracks"
    if ($null -eq $tracks) {
        $project = Get-Prop $result "project"
        $tracks = Get-Prop $project "tracks"
    }
    return @($tracks)
}

function Get-UserTracks {
    param($ProjectState)
    $out = @()
    foreach ($track in @(Get-TracksFromState $ProjectState)) {
        if ($null -ne $track -and (Test-UserTrack $track)) { $out += $track }
    }
    return @($out)
}

function Get-TrackId {
    param($Track)
    foreach ($key in @("track_id", "id", "item_id")) {
        $value = [string](Get-Prop $Track $key)
        if (-not [string]::IsNullOrWhiteSpace($value)) { return $value }
    }
    return ""
}

function Get-ClipId {
    param($Clip)
    foreach ($key in @("clip_id", "id", "item_id")) {
        $value = [string](Get-Prop $Clip $key)
        if (-not [string]::IsNullOrWhiteSpace($value)) { return $value }
    }
    return ""
}

function Get-NoteLength {
    param($Note)
    foreach ($key in @("length", "duration", "length_beats")) {
        $value = Get-Prop $Note $key
        if ($null -ne $value) { return [double]$value }
    }
    return 0.0
}

function New-MidiClip {
    param([string]$Name, [string]$TrackId, [double]$StartBeats, [double]$LengthBeats)
    $response = Invoke-Tool -Name $Name -Tool "create_midi_clip" -ToolArgs @{
        track_id = $TrackId
        start = $StartBeats
        length = $LengthBeats
        time_unit = "beats"
    }
    $result = Get-Result $response
    $clipId = [string](Get-Prop $result "clip_id")
    if ([string]::IsNullOrWhiteSpace($clipId)) { $clipId = [string](Get-Prop $result "new_clip_id") }
    Assert-True (-not [string]::IsNullOrWhiteSpace($clipId)) ("create_midi_clip returned no clip_id: " + (Format-Result $response))
    return $clipId
}

function Add-MidiNotes {
    param([string]$Name, [string]$ClipId, $Notes)
    $noteList = @($Notes)
    $response = Invoke-Tool -Name $Name -Tool "add_midi_notes_bulk" -ToolArgs @{
        clip_id = $ClipId
        time_unit = "beats"
        notes = $noteList
    }
    $result = Get-Result $response
    $added = Get-Prop $result "added_count"
    if ($null -eq $added) { $added = Get-Prop $result "inserted_count" }
    Assert-True ([int]$added -eq $noteList.Count) ("add_midi_notes_bulk added_count=" + [string]$added + " expected=" + [string]$noteList.Count + " response=" + (Format-Result $response))
    return $response
}

function Read-ClipNotes {
    param([string]$Name, [string]$ClipId)
    $response = Invoke-Tool -Name $Name -Tool "get_midi_clip_notes" -ToolArgs @{ clip_id = $ClipId } -Confirmed $false
    return @(Get-Prop (Get-Result $response) "notes")
}

function Get-NoteSignature {
    param($Notes)
    $parts = @()
    foreach ($note in @($Notes)) {
        $parts += ([string][int](Get-Prop $note "pitch") + "@" + [string][double](Get-Prop $note "start") + "/" + [string][double](Get-NoteLength $note))
    }
    return ($parts -join ",")
}

function Assert-Note {
    param($Notes, [int]$Pitch, [double]$Start, [double]$Length, [string]$Label)
    $hits = 0
    foreach ($note in @($Notes)) {
        $notePitch = [int](Get-Prop $note "pitch")
        $noteStart = [double](Get-Prop $note "start")
        $noteLength = [double](Get-NoteLength $note)
        if ($notePitch -eq $Pitch -and [Math]::Abs($noteStart - $Start) -le 0.001 -and [Math]::Abs($noteLength - $Length) -le 0.001) {
            $hits++
        }
    }
    Assert-True ($hits -eq 1) ($Label + ": expected exactly one note pitch=" + [string]$Pitch + " start=" + [string]$Start + " length=" + [string]$Length + ", hits=" + [string]$hits + " notes=" + (Get-NoteSignature $Notes))
}

function New-SelectionContext {
    param([string]$ClipId, [string]$TrackId)
    # Bind every selection key the agent chat contract reads. The piano-roll
    # focus key has the highest priority in eduJamRequestSelectedClipID; setting
    # it explicitly prevents a cached UI focus from silently winning.
    return @{
        selected_clip_id = $ClipId
        selected_clip_ids = @($ClipId)
        selected_clip_track_id = $TrackId
        piano_roll_focus_clip_id = $ClipId
        piano_roll_focus_track_id = $TrackId
        current_selection = @{
            selected_clip_id = $ClipId
            selected_clip_ids = @($ClipId)
            selected_clip_track_id = $TrackId
            piano_roll_focus_clip_id = $ClipId
            piano_roll_focus_track_id = $TrackId
        }
    }
}

function Print-RunbookSummary {
    Write-Host "W02-A1c real-stack journey executor - RUNBOOK summary" -ForegroundColor Yellow
    Write-Host "  S0 preflight: /health, /agent/runtime/status, /agent/state, authority," -ForegroundColor Yellow
    Write-Host "     recents + Settings.xml + default_project.xml snapshots, log offsets." -ForegroundColor Yellow
    Write-Host "  S1 isolated RunRoot: project.new + project.save_as A; import W01 design" -ForegroundColor Yellow
    Write-Host "     source (tempo=80 turns=60 calls=120) and 开始乐局; assert imported/entered." -ForegroundColor Yellow
    Write-Host "  S2 target track from compiled runtime_slots/clip slot; create two" -ForegroundColor Yellow
    Write-Host "     distinguishable MIDI clips (create_midi_clip + add_midi_notes_bulk)." -ForegroundColor Yellow
    Write-Host "  S3 写好了 with selected_clip_id=second: assert clip_scope=selected and" -ForegroundColor Yellow
    Write-Host "     clip_ids[target]=second; first clip must not impersonate." -ForegroundColor Yellow
    Write-Host "  S4 one real-LLM natural question: stop_reason=done, non-empty reply," -ForegroundColor Yellow
    Write-Host "     new agent_prompt_render line with lesson_context card/node/clip." -ForegroundColor Yellow
    Write-Host "  S5 no-selection control (scope=first) + cross-slot selection" -ForegroundColor Yellow
    Write-Host "     (scope=selection_unmatched); never counted as selected pass." -ForegroundColor Yellow
    Write-Host "  S6 结束乐局 -> project.open A -> natural sentence: auto-intro/recovery" -ForegroundColor Yellow
    Write-Host "     evidence; teardown restores recents/Settings/default_project/authority." -ForegroundColor Yellow
    Write-Host ""
    Write-Host "Not authorized. The script does not start/restart/reuse a stack; it only" -ForegroundColor Yellow
    Write-Host "uses an already-running stack. Ask the lead to register the berth in" -ForegroundColor Yellow
    Write-Host "coord/resources/PC-RUNTIME-STACK.md and run with -StackAuthorized:" -ForegroundColor Yellow
    Write-Host "  powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\run_w02_a1c_smoke.ps1 -RepoRoot D:/Vit_DAW -AgentBaseUrl http://127.0.0.1:7878 -StackAuthorized -RuntimeOwner '<owner>' -TimeoutSeconds 180" -ForegroundColor Yellow
    Write-Host "Dry-run (no stack, exit 0): add -DryRun." -ForegroundColor Yellow
}

function Print-DryRunPlan {
    param([string]$ResolvedRepoRoot, [string]$DesignSource)
    Write-Host "W02-A1c real-stack journey executor - DRY RUN (no HTTP, no stack, no port)" -ForegroundColor Cyan
    Write-Host ("  RepoRoot       : " + $ResolvedRepoRoot)
    Write-Host ("  AgentBaseUrl   : " + $AgentBaseUrl)
    Write-Host ("  Design source  : " + $DesignSource)
    Write-Host ("  RunRoot        : " + $(if ([string]::IsNullOrWhiteSpace($RunRoot)) { "<temp>/vit_w02_a1c/run_<stamp>" } else { $RunRoot }))
    Write-Host ("  RuntimeOwner   : " + $(if ([string]::IsNullOrWhiteSpace($RuntimeOwner)) { "<required with -StackAuthorized>" } else { $RuntimeOwner }))
    Write-Host ("  TimeoutSeconds : " + [string]$TimeoutSeconds)
    Write-Host "  Plan:"
    Write-Host "    S0 preflight + snapshots (recents/Settings.xml/default_project.xml) + log offsets"
    Write-Host "    S1 project.new -> project.save_as A -> import W01 design -> 开始乐局"
    Write-Host "    S1.5 record card event nodes until the first clip_exists/track_non_empty node"
    Write-Host "    S2 create required user tracks by runtime_slots track_index; create two MIDI clips; select_clip(second)"
    Write-Host "    S3 写好了 with selected_clip_id=second; assert clip_scope=selected, clip_ids[target]=second, first not used"
    Write-Host "    S4 one real-LLM natural question; assert stop_reason=done, reply non-empty, lesson_context in new agent_prompt_render.jsonl row"
    Write-Host "    S5 no-selection scope=first control; cross-slot selection scope=selection_unmatched (never selected pass)"
    Write-Host "    S6 结束乐局 -> project.open A -> natural sentence -> auto-intro/recovery; teardown restores files/authority"
    Write-Host "  Exit codes: 0 PASS, 1 assertion, 2 environment/teardown, 3 unauthorized, 4 LLM class, 5 evidence/API gap"
    Write-Host "  Not executed. No stack process was started and no port was opened." -ForegroundColor Green
}

# ---------------------------------------------------------------------------
# Zero-stack entry points: dry-run first, then authorization gate.
# ---------------------------------------------------------------------------
if ($DryRun) {
    $resolved = $RepoRoot
    if (Test-Path -LiteralPath $RepoRoot) { $resolved = (Resolve-Path -LiteralPath $RepoRoot).Path }
    $design = Join-Path $RepoRoot "coord/lesson-designs/music-creation-unit2"
    Print-DryRunPlan -ResolvedRepoRoot $resolved -DesignSource $design
    exit 0
}

if (-not $StackAuthorized) {
    Print-RunbookSummary
    exit 3
}

# ---------------------------------------------------------------------------
# Authorized path. Resolve inputs; never start/stop a stack.
# ---------------------------------------------------------------------------
if ([string]::IsNullOrWhiteSpace($RuntimeOwner)) {
    Write-Host "error: -RuntimeOwner is required with -StackAuthorized (record the berth owner)." -ForegroundColor Red
    exit 2
}
if (-not (Test-Path -LiteralPath $RepoRoot)) {
    Write-Host ("error: RepoRoot does not exist: " + $RepoRoot) -ForegroundColor Red
    exit 2
}
$RepoRoot = (Resolve-Path -LiteralPath $RepoRoot).Path
$designSource = Join-Path $RepoRoot "coord/lesson-designs/music-creation-unit2"
if (-not (Test-Path -LiteralPath (Join-Path $designSource "design.v1.yaml"))) {
    Write-Host ("error: W01 design source missing: " + $designSource) -ForegroundColor Red
    exit 2
}

$stamp = Get-Date -Format "yyyyMMdd_HHmmss"
if ([string]::IsNullOrWhiteSpace($RunRoot)) {
    $tempRoot = [System.IO.Path]::GetTempPath()
    $RunRoot = Join-Path (Join-Path $tempRoot "vit_w02_a1c") ("run_" + $stamp)
}
$RunRoot = [System.IO.Path]::GetFullPath($RunRoot)
$snapshotDir = Join-Path $RunRoot "snapshots"
$evidenceDir = Join-Path $RunRoot "evidence"
New-Item -ItemType Directory -Path $RunRoot -Force | Out-Null
New-Item -ItemType Directory -Path (Join-Path $RunRoot "requests") -Force | Out-Null
New-Item -ItemType Directory -Path (Join-Path $RunRoot "responses") -Force | Out-Null
New-Item -ItemType Directory -Path $snapshotDir -Force | Out-Null
New-Item -ItemType Directory -Path $evidenceDir -Force | Out-Null

$conversationId = "w02-a1c-" + $stamp
$projectA = Join-Path $RunRoot ("W02-A1c-A-" + $stamp + ".vit")
$recentsPath = Get-RecentsPath
$settingsPath = Join-Path $RepoRoot "VitApp/Workspace/Settings/Settings.xml"
$defaultProjectPath = Join-Path $RepoRoot "VitApp/Workspace/default_project.xml"
$contextSnapshotPath = Get-AgentLogPath -EnvName "VIT_CONTEXT_SNAPSHOT_PATH" -FileName "agent_context_snapshots.jsonl"
$promptRenderPath = Get-AgentLogPath -EnvName "VIT_AGENT_PROMPT_RENDER_PATH" -FileName "agent_prompt_render.jsonl"
$telemetryPath = Get-AgentLogPath -EnvName "VIT_AGENT_LLM_TELEMETRY_PATH" -FileName "agent_llm_telemetry.jsonl"

$summary = [ordered]@{
    schema_version = "vit_w02_a1c_smoke.v1"
    card = "W02-A1C"
    status = "running"
    runtime_owner = $RuntimeOwner
    created_at = (Get-Date).ToString("o")
    completed_at = ""
    run_root = $RunRoot
    agent_base_url = $AgentBaseUrl
    repo_root = $RepoRoot
    git_head = ""
    design_source = $designSource
    conversation_id = $conversationId
    project_a = $projectA
    project_uuid = ""
    card_id = ""
    compiled_card = ""
    binding_path = ""
    preflight = [ordered]@{}
    s1 = [ordered]@{}
    s2 = [ordered]@{}
    s3 = [ordered]@{}
    s4 = [ordered]@{}
    s5a = [ordered]@{}
    s5b = [ordered]@{}
    s6 = [ordered]@{}
    teardown = [ordered]@{}
    failure_class = ""
    error = ""
}

$git = Get-Command git -ErrorAction SilentlyContinue
if ($null -ne $git) {
    try {
        $head = (& git -C $RepoRoot rev-parse HEAD 2>$null) -join ""
        $summary.git_head = $head.Trim()
    }
    catch { }
}

$targetSlot = ""
$targetTrackId = ""
$firstTrackId = ""
$firstClipId = ""
$secondClipId = ""
$crossClipId = ""
$currentNode = ""
$preflight = $summary.preflight
$s1 = $summary.s1
$s2 = $summary.s2
$s3 = $summary.s3
$s4 = $summary.s4
$s5a = $summary.s5a
$s5b = $summary.s5b
$s6 = $summary.s6
$teardown = $summary.teardown

$recentsSnapshot = $null
$settingsSnapshot = $null
$defaultProjectSnapshot = $null
$authorityBefore = ""
$kernelLogInfo = $null
$contextOffset = [long]0
$promptOffset = [long]0
$telemetryOffset = [long]0
$s6ConversationId = ""
$s6CloseConversationId = ""
$s6SessionActive = $false
$mainSessionActive = $false

try {
    Write-Step "S0 preflight (read-only; no stack start, no port)"
    $health = Invoke-Api -Name "s0_health" -Method "Get" -Path "/health" -Body $null
    Assert-True (([string](Get-Prop $health "status")) -eq "ok") ("/health status=" + [string](Get-Prop $health "status"))
    Assert-True (([string](Get-Prop $health "service")) -eq "VitAgent") ("/health service=" + [string](Get-Prop $health "service"))

    $runtime = Invoke-Api -Name "s0_runtime_status" -Method "Get" -Path "/agent/runtime/status" -Body $null
    Assert-True (([string](Get-Prop $runtime "status")) -eq "ok") ("/agent/runtime/status status=" + [string](Get-Prop $runtime "status"))
    $kernel = Get-Prop $runtime "kernel"
    Assert-True (Get-Bool (Get-Prop $kernel "connected")) ("kernel not connected: " + (Format-Result $kernel))
    Assert-True (([string](Get-Prop $kernel "reply_status")) -eq "ok") ("kernel ping reply_status=" + [string](Get-Prop $kernel "reply_status"))
    Assert-True (-not (Get-Bool (Get-Prop $runtime "checkout_blocked"))) "runtime reports checkout_blocked=true (an active execution owns the project)"
    $goal = Get-Prop $runtime "goal"
    $goalStatus = [string](Get-Prop $goal "status")
    if ([string]::IsNullOrWhiteSpace($goalStatus)) { $goalStatus = "idle" }
    Assert-True (Test-InactiveGoalStatus $goalStatus) ("stack goal.status is not in the inactive set (" + $goalStatus + "); refusing to reuse a busy stack")
    $preflight.runtime = [ordered]@{
        goal_status = $goalStatus
        inactive = (Test-InactiveGoalStatus $goalStatus)
        kernel_connected = $true
        kernel_reply_status = "ok"
        kernel_latency_ms = Get-Prop $kernel "latency_ms"
    }

    $state = Invoke-Api -Name "s0_state" -Method "Get" -Path "/agent/state" -Body $null
    Assert-True (([string](Get-Prop $state "status")) -eq "ok") ("/agent/state status=" + [string](Get-Prop $state "status"))
    $currentProject = Get-Prop $state "current_project"
    $projectHistory = Get-Prop $state "project_history"
    $shadow = Get-Prop $state "shadow"
    $currentPath = [string](Get-Prop $currentProject "path")
    if ([string]::IsNullOrWhiteSpace($currentPath)) { $currentPath = [string](Get-Prop $projectHistory "project_path") }
    if ([string]::IsNullOrWhiteSpace($currentPath)) { $currentPath = [string](Get-Prop $shadow "project_path") }
    $currentUuid = [string](Get-Prop $currentProject "uuid")
    if ([string]::IsNullOrWhiteSpace($currentUuid)) { $currentUuid = [string](Get-Prop $projectHistory "project_uuid") }
    $unsaved = (Get-Bool (Get-Prop $projectHistory "unsaved")) -or (Get-Bool (Get-Prop $shadow "unsaved"))
    $draft = (Get-Bool (Get-Prop $projectHistory "draft")) -or (Get-Bool (Get-Prop $shadow "draft"))
    if (-not [string]::IsNullOrWhiteSpace($currentPath) -and ($unsaved -or $draft)) {
        Fail "preflight" ("stack has an unsaved/dirty project (" + $currentPath + ", unsaved=" + [string]$unsaved + ", draft=" + [string]$draft + "); refusing to replace it")
    }
    $preflight.state = [ordered]@{
        current_project_path = $currentPath
        current_project_uuid = $currentUuid
        unsaved = $unsaved
        draft = $draft
    }

    # Authoritative files are snapshotted before the authority handler runs:
    # /agent/authority performs activate+persist and can touch workspace state.
    $recentsSnapshot = Snapshot-File -Path $recentsPath -Name "recents"
    $settingsSnapshot = Snapshot-File -Path $settingsPath -Name "Settings.xml"
    $defaultProjectSnapshot = Snapshot-File -Path $defaultProjectPath -Name "default_project.xml"
    $preflight.snapshots = [ordered]@{
        recents = $recentsSnapshot
        settings = $settingsSnapshot
        default_project = $defaultProjectSnapshot
    }

    $authority = Invoke-Api -Name "s0_authority" -Method "Get" -Path "/agent/authority" -Body $null
    Assert-True (([string](Get-Prop $authority "status")) -eq "ok") ("/agent/authority status=" + [string](Get-Prop $authority "status"))
    $authorityBefore = [string](Get-Prop $authority "authority_mode")
    Assert-True (-not [string]::IsNullOrWhiteSpace($authorityBefore)) "authority_mode is empty"
    $preflight.authority = [ordered]@{ before = $authorityBefore }

    $kernelLogInfo = Get-KernelLogInfo
    $contextOffset = Get-FileLength $contextSnapshotPath
    $promptOffset = Get-FileLength $promptRenderPath
    $telemetryOffset = Get-FileLength $telemetryPath
    $preflight.logs = [ordered]@{
        kernel = $kernelLogInfo
        context_snapshot = [ordered]@{ path = $contextSnapshotPath; offset = $contextOffset }
        prompt_render = [ordered]@{ path = $promptRenderPath; offset = $promptOffset }
        telemetry = [ordered]@{ path = $telemetryPath; offset = $telemetryOffset }
    }
    Write-Ok ("preflight: goal idle, kernel connected, authority=" + $authorityBefore)
    Write-Ok ("snapshots: recents=" + [string]$recentsSnapshot.existed + " Settings=" + [string]$settingsSnapshot.existed + " default_project=" + [string]$defaultProjectSnapshot.existed)

    Write-Step "S1 isolated project + W01 import + enter"
    $newProject = Invoke-Tool -Name "s1_project_new" -Tool "project.new" -ToolArgs @{}
    $newResult = Get-Result $newProject
    $newKernelStatus = [string](Get-Prop $newResult "status")
    if (-not [string]::IsNullOrWhiteSpace($newKernelStatus) -and $newKernelStatus -ne "ok") {
        Fail "assert" ("project.new kernel status=" + $newKernelStatus + " result=" + (Format-Result $newResult))
    }
    $s1.project_new = [ordered]@{ status = [string](Get-Prop $newProject "status"); result = $newResult }
    $saveProject = Invoke-Tool -Name "s1_project_save_as" -Tool "project.save_as" -ToolArgs @{ file_path = $projectA }
    $saveResult = Get-Result $saveProject
    $saveKernelStatus = [string](Get-Prop $saveResult "status")
    if (-not [string]::IsNullOrWhiteSpace($saveKernelStatus) -and $saveKernelStatus -ne "ok") {
        Fail "assert" ("project.save_as kernel status=" + $saveKernelStatus + " result=" + (Format-Result $saveResult))
    }
    $s1.project_save_as = [ordered]@{ status = [string](Get-Prop $saveProject "status"); path = $projectA; result = $saveResult }
    $savedState = Wait-ActiveProject -Path $projectA -Seconds ([Math]::Max(30, [Math]::Min($TimeoutSeconds, 120)))
    $savedCurrent = Get-Prop $savedState "current_project"
    $projectUuid = [string](Get-Prop $savedCurrent "uuid")
    if ([string]::IsNullOrWhiteSpace($projectUuid)) {
        $savedHistory = Get-Prop $savedState "project_history"
        $projectUuid = [string](Get-Prop $savedHistory "project_uuid")
    }
    if ([string]::IsNullOrWhiteSpace($projectUuid)) {
        $savedShadow = Get-Prop $savedState "shadow"
        $projectUuid = [string](Get-Prop $savedShadow "project_uuid")
    }
    Assert-True (-not [string]::IsNullOrWhiteSpace($projectUuid)) "active project uuid is empty after save_as"
    Assert-True (Test-Path -LiteralPath $projectA) ("project file not written: " + $projectA)
    $summary.project_uuid = $projectUuid
    Write-Ok ("project A: " + $projectA + " uuid=" + $projectUuid)

    $importMessage = '导入乐局设计源 ' + $designSource + ' tempo=80 turns=60 calls=120'
    $import = Invoke-AgentChat -Name "s1_import" -ConversationID $conversationId -Message $importMessage -Context @{}
    $importStop = [string](Get-Prop $import "stop_reason")
    Assert-True ($importStop -eq "edu_jam_legame_imported") ("import stop_reason=" + $importStop + " reply=" + [string](Get-Prop $import "reply"))
    $importWorkflow = Get-WorkflowData $import
    $cardId = [string](Get-Prop $importWorkflow "card_id")
    Assert-True (-not [string]::IsNullOrWhiteSpace($cardId)) "import workflow_data.card_id is empty"
    $summary.card_id = $cardId
    $s1.import = [ordered]@{
        stop_reason = $importStop
        card_id = $cardId
        warnings = Get-Prop $importWorkflow "warnings"
        infos = Get-Prop $importWorkflow "infos"
    }

    $packageDir = Find-ProjectPackage -ProjectPath $projectA
    Assert-True (-not [string]::IsNullOrWhiteSpace($packageDir)) ("project package not found next to " + $projectA)
    $cardRel = [string](Get-Prop $importWorkflow "card")
    $cardPath = ""
    if (-not [string]::IsNullOrWhiteSpace($cardRel)) {
        $cardPath = Join-Path $packageDir ($cardRel.Replace("/", [System.IO.Path]::DirectorySeparatorChar))
    }
    if ([string]::IsNullOrWhiteSpace($cardPath) -or -not (Test-Path -LiteralPath $cardPath)) {
        $found = @(Get-ChildItem -LiteralPath (Join-Path $packageDir "legame/cards") -Recurse -Filter "*.compiled.level.yaml" -File -ErrorAction SilentlyContinue)
        Assert-True ($found.Count -eq 1) ("compiled card not found/unambiguous: " + $cardPath + " found=" + [string]$found.Count)
        $cardPath = $found[0].FullName
    }
    Assert-True (Test-Path -LiteralPath $cardPath) ("compiled card missing: " + $cardPath)
    $summary.compiled_card = $cardPath
    $cardInfo = Read-CompiledCard -Path $cardPath
    Assert-True ($cardInfo.slots.Count -gt 0) ("compiled card runtime_slots empty: " + $cardPath)
    Assert-True ($cardInfo.nodes.Count -gt 0) ("compiled card nodes empty: " + $cardPath)
    $bindingPath = [string](Get-Prop $importWorkflow "binding")
    if ([string]::IsNullOrWhiteSpace($bindingPath) -or -not (Test-Path -LiteralPath $bindingPath)) {
        Fail "api" ("legame binding missing after import: " + $bindingPath)
    }
    $summary.binding_path = $bindingPath
    $s1.package = [ordered]@{ package_dir = $packageDir; compiled_card = $cardPath; binding = $bindingPath }

    $enter = Invoke-AgentChat -Name "s1_enter" -ConversationID $conversationId -Message "开始乐局" -Context @{}
    $enterStop = [string](Get-Prop $enter "stop_reason")
    Assert-True ($enterStop -eq "edu_jam_entered") ("enter stop_reason=" + $enterStop + " reply=" + [string](Get-Prop $enter "reply"))
    $enterWorkflow = Get-WorkflowData $enter
    $enterLevel = [string](Get-Prop $enterWorkflow "level_id")
    Assert-True ($enterLevel -eq $cardId) ("enter level_id=" + $enterLevel + " expected=" + $cardId)
    $currentNode = Get-CurrentNode $enter
    Assert-True (-not [string]::IsNullOrWhiteSpace($currentNode)) "enter workflow_data.current_node is empty"
    $s1.enter = [ordered]@{
        stop_reason = $enterStop
        level_id = $enterLevel
        current_node = $currentNode
        reply = [string](Get-Prop $enter "reply")
    }
    $mainSessionActive = $true
    Write-Ok ("enter: level=" + $enterLevel + " node=" + $currentNode)

    Write-Step "S1.5 advance deterministic event nodes to the first clip/track node"
    $advanceCount = 0
    for ($i = 0; $i -lt 10; $i++) {
        $nodeInfo = $cardInfo.nodes[$currentNode]
        if ($null -eq $nodeInfo) { Fail "api" ("card node not found: " + $currentNode) }
        if (Get-Bool $nodeInfo["unsupported"]) { Fail "api" ("node done_when uses all/any combinator; this executor supports direct leaves only: " + $currentNode) }
        if (-not [string]::IsNullOrWhiteSpace([string]$nodeInfo["clip"])) {
            $targetSlot = [string]$nodeInfo["clip"]
            $s1.advance = [ordered]@{ stopped_at_node = $currentNode; slot = $targetSlot; kind = "clip_exists"; event_nodes = $advanceCount }
            break
        }
        if (-not [string]::IsNullOrWhiteSpace([string]$nodeInfo["track"])) {
            $targetSlot = [string]$nodeInfo["track"]
            $s1.advance = [ordered]@{ stopped_at_node = $currentNode; slot = $targetSlot; kind = "track_non_empty"; event_nodes = $advanceCount }
            break
        }
        $eventName = [string]$nodeInfo["event"]
        if ([string]::IsNullOrWhiteSpace($eventName)) {
            Fail "api" ("node has no event/clip/track leaf: " + $currentNode + " info=" + (Format-Result $nodeInfo))
        }
        $eventResponse = Invoke-AgentChat -Name ("s1_event_" + $i) -ConversationID $conversationId -Message ("乐局事件 " + $eventName) -Context @{}
        $eventStop = [string](Get-Prop $eventResponse "stop_reason")
        Assert-True ($eventStop -eq "edu_jam_event_recorded") ("event " + $eventName + " stop_reason=" + $eventStop)
        $advanceResponse = Invoke-AgentChat -Name ("s1_advance_" + $i) -ConversationID $conversationId -Message "写好了" -Context @{}
        $advanceStop = [string](Get-Prop $advanceResponse "stop_reason")
        Assert-True ($advanceStop -eq "edu_jam_node_done") ("advance after event " + $eventName + " stop_reason=" + $advanceStop + " reply=" + [string](Get-Prop $advanceResponse "reply"))
        $advanceWorkflow = Get-WorkflowData $advanceResponse
        $nodeDone = [string](Get-Prop $advanceWorkflow "node_done")
        Assert-True ($nodeDone -eq $currentNode) ("advance node_done=" + $nodeDone + " expected=" + $currentNode)
        $nextNode = [string](Get-Prop $advanceWorkflow "current_node")
        Assert-True (-not [string]::IsNullOrWhiteSpace($nextNode)) "advance current_node is empty"
        $currentNode = $nextNode
        $advanceCount++
    }
    Assert-True (-not [string]::IsNullOrWhiteSpace($targetSlot)) "did not reach a clip/track node within 10 advances"
    Assert-True ($cardInfo.slots.ContainsKey($targetSlot)) ("target slot not in runtime_slots: " + $targetSlot)
    $targetTrackIndex = [int]$cardInfo.slots[$targetSlot]["track_index"]
    Assert-True ($targetTrackIndex -ge 1) ("target slot track_index invalid: " + [string]$targetTrackIndex)
    $s1.advance.current_node = $currentNode
    $s1.advance.target_slot = $targetSlot
    $s1.advance.track_index = $targetTrackIndex
    Write-Ok ("clip node: " + $currentNode + " slot=" + $targetSlot + " track_index=" + [string]$targetTrackIndex)

    Write-Step "S2 create required user tracks + two distinguishable MIDI clips"
    $projectState = Invoke-Tool -Name "s2_project_state_0" -Tool "project.state" -ToolArgs @{} -Confirmed $false
    $userTracks = @(Get-UserTracks $projectState)
    $addCount = 0
    while ($userTracks.Count -lt $targetTrackIndex) {
        $addResponse = Invoke-Tool -Name ("s2_track_add_" + $addCount) -Tool "track.add" -ToolArgs @{}
        $addCount++
        Start-Sleep -Milliseconds 300
        $projectState = Invoke-Tool -Name ("s2_project_state_" + $addCount) -Tool "project.state" -ToolArgs @{} -Confirmed $false
        $userTracks = @(Get-UserTracks $projectState)
        if ($addCount -gt 8) { Fail "api" ("track.add did not reach runtime_slots track_index " + [string]$targetTrackIndex) }
    }
    Assert-True ($userTracks.Count -ge $targetTrackIndex) ("user tracks=" + [string]$userTracks.Count + " target_index=" + [string]$targetTrackIndex)
    $targetTrack = $userTracks[$targetTrackIndex - 1]
    $targetTrackId = Get-TrackId $targetTrack
    Assert-True (-not [string]::IsNullOrWhiteSpace($targetTrackId)) "target track id is empty"
    $firstTrack = $userTracks[0]
    $firstTrackId = Get-TrackId $firstTrack
    Assert-True (-not [string]::IsNullOrWhiteSpace($firstTrackId)) "first user track id is empty"
    $trackRows = @()
    foreach ($track in $userTracks) {
        $trackRows += [ordered]@{
            track_id = Get-TrackId $track
            track_name = [string](Get-Prop $track "track_name")
            track_type = [string](Get-Prop $track "track_type")
            is_audio = Get-Bool (Get-Prop $track "is_audio")
            user_track_index = Get-Prop $track "user_track_index"
        }
    }
    $targetUserIndex = Get-Prop $targetTrack "user_track_index"
    if ($null -ne $targetUserIndex -and [int]$targetUserIndex -ne $targetTrackIndex) {
        Fail "api" ("target row user_track_index=" + [string]$targetUserIndex + " does not match runtime_slots track_index=" + [string]$targetTrackIndex)
    }
    $s2.tracks = [ordered]@{
        target_slot = $targetSlot
        target_index = $targetTrackIndex
        target_track_id = $targetTrackId
        created_count = $addCount
        rows = $trackRows
    }
    Write-Ok ("target track: " + $targetTrackId + " (index " + [string]$targetTrackIndex + ", added " + [string]$addCount + ")")

    $firstClipId = New-MidiClip -Name "s2_create_first" -TrackId $targetTrackId -StartBeats 0 -LengthBeats 4
    $firstNotes = @(
        @{ pitch = 60; start = 0; length = 1; velocity = 90 },
        @{ pitch = 64; start = 1; length = 1; velocity = 80 },
        @{ pitch = 67; start = 2; length = 1; velocity = 70 }
    )
    $null = Add-MidiNotes -Name "s2_notes_first" -ClipId $firstClipId -Notes $firstNotes
    $firstRead = @(Read-ClipNotes -Name "s2_read_first" -ClipId $firstClipId)
    $firstSignature = Get-NoteSignature $firstRead
    Assert-True ($firstRead.Count -eq $firstNotes.Count) ("first clip notes=" + [string]$firstRead.Count + " expected=" + [string]$firstNotes.Count)
    Assert-Note -Notes $firstRead -Pitch 60 -Start 0 -Length 1 -Label "first clip note 60"
    Assert-Note -Notes $firstRead -Pitch 64 -Start 1 -Length 1 -Label "first clip note 64"
    Assert-Note -Notes $firstRead -Pitch 67 -Start 2 -Length 1 -Label "first clip note 67"

    $secondClipId = New-MidiClip -Name "s2_create_second" -TrackId $targetTrackId -StartBeats 8 -LengthBeats 4
    $secondNotes = @(
        @{ pitch = 67; start = 0; length = 2; velocity = 100 },
        @{ pitch = 71; start = 2; length = 0.5; velocity = 70 },
        @{ pitch = 74; start = 2.5; length = 1.5; velocity = 60 }
    )
    $null = Add-MidiNotes -Name "s2_notes_second" -ClipId $secondClipId -Notes $secondNotes
    $secondRead = @(Read-ClipNotes -Name "s2_read_second" -ClipId $secondClipId)
    $secondSignature = Get-NoteSignature $secondRead
    Assert-True ($secondRead.Count -eq $secondNotes.Count) ("second clip notes=" + [string]$secondRead.Count + " expected=" + [string]$secondNotes.Count)
    Assert-Note -Notes $secondRead -Pitch 67 -Start 0 -Length 2 -Label "second clip note 67"
    Assert-Note -Notes $secondRead -Pitch 71 -Start 2 -Length 0.5 -Label "second clip note 71"
    Assert-Note -Notes $secondRead -Pitch 74 -Start 2.5 -Length 1.5 -Label "second clip note 74"
    Assert-True ($firstSignature -ne $secondSignature) "first/second clip note signatures are identical"
    Assert-True ($firstClipId -ne $secondClipId) "first/second clip ids are identical"

    $selectSecondResult = [ordered]@{}
    $selectSecondError = ""
    try {
        $selectSecond = Invoke-Tool -Name "s2_select_second" -Tool "select_clip" -ToolArgs @{ clip_id = $secondClipId; track_id = $targetTrackId } -Confirmed $false
        $selectSecondResult = Get-Result $selectSecond
        if (([string](Get-Prop $selectSecondResult "clip_id")) -ne $secondClipId) {
            $selectSecondError = "select_clip returned clip_id=" + [string](Get-Prop $selectSecondResult "clip_id") + " expected=" + $secondClipId
        }
        elseif (([string](Get-Prop $selectSecondResult "ui_action")) -ne "select_clip") {
            $selectSecondError = "select_clip ui_action=" + [string](Get-Prop $selectSecondResult "ui_action")
        }
    }
    catch {
        $selectSecondError = $_.Exception.Message
    }
    if (-not [string]::IsNullOrWhiteSpace($selectSecondError)) {
        Write-WarnLine ("select_clip(second) is UI-only/unavailable; continuing with the chat-context binding: " + $selectSecondError)
    }
    $s2.clips = [ordered]@{
        first_clip_id = $firstClipId
        first_notes = $firstSignature
        second_clip_id = $secondClipId
        second_notes = $secondSignature
        selection_binding = "chat_context.selected_clip_id"
        select_clip_ui_action = [string](Get-Prop $selectSecondResult "ui_action")
        select_clip_returns_selection_state = $false
        select_clip_error = $selectSecondError
    }
    Write-Ok ("clips: first=" + $firstClipId + " second=" + $secondClipId + " (select_clip is UI-only; context is the binding)")

    Write-Step "S3 selected-clip advance (写好了 with selected_clip_id=second)"
    $selectionContext = New-SelectionContext -ClipId $secondClipId -TrackId $targetTrackId
    $s3Response = Invoke-AgentChat -Name "s3_check_selected" -ConversationID $conversationId -Message "写好了" -Context $selectionContext
    $s3Stop = [string](Get-Prop $s3Response "stop_reason")
    Assert-True ($s3Stop -eq "edu_jam_node_done") ("S3 stop_reason=" + $s3Stop + " reply=" + [string](Get-Prop $s3Response "reply") + " workflow=" + (Format-Result (Get-WorkflowData $s3Response)))
    $s3Workflow = Get-WorkflowData $s3Response
    $s3Scope = [string](Get-Prop $s3Workflow "clip_scope")
    Assert-True ($s3Scope -eq "selected") ("S3 clip_scope=" + $s3Scope + " expected=selected")
    $s3ClipId = Get-ClipIdForSlot -WorkflowData $s3Workflow -Slot $targetSlot
    Assert-True ($s3ClipId -eq $secondClipId) ("S3 clip_ids[" + $targetSlot + "]=" + $s3ClipId + " expected second=" + $secondClipId)
    Assert-True ($s3ClipId -ne $firstClipId) ("S3 first clip impersonated the selection: " + $firstClipId)
    $s3Selected = [string](Get-Prop $s3Workflow "selected_clip_id")
    Assert-True ($s3Selected -eq $secondClipId) ("S3 selected_clip_id=" + $s3Selected)
    $s3NodeDone = [string](Get-Prop $s3Workflow "node_done")
    Assert-True ($s3NodeDone -eq $currentNode) ("S3 node_done=" + $s3NodeDone + " expected=" + $currentNode)
    $s3NextNode = [string](Get-Prop $s3Workflow "current_node")
    Assert-True (-not [string]::IsNullOrWhiteSpace($s3NextNode) -and $s3NextNode -ne $currentNode) ("S3 current_node did not advance: " + $s3NextNode)
    $s3.node_done = $s3NodeDone
    $s3.next_node = $s3NextNode
    $s3.stop_reason = $s3Stop
    $s3.clip_scope = $s3Scope
    $s3.clip_id = $s3ClipId
    $s3.selected_clip_id = $s3Selected
    $s3.first_clip_id = $firstClipId
    $s3.first_clip_used = $false
    $s3.differential_note = "compiled card clip nodes are clip_exists-only; no notes-content judge leaf exists, so pass/fail cannot discriminate first vs second. Selection is proven by clip_ids/scope and distinct read-back notes."
    $currentNode = $s3NextNode
    Write-Ok ("S3: scope=selected clip=" + $s3ClipId + " node=" + $s3NodeDone + " -> " + $s3NextNode)

    Write-Step "S4 one real-LLM natural question"
    $contextOffset = Get-FileLength $contextSnapshotPath
    $promptOffset = Get-FileLength $promptRenderPath
    $telemetryOffset = Get-FileLength $telemetryPath
    $question = "我们现在第几关？这一关目标是什么？请一句话回答"
    $probe = Invoke-LlmChat -Name "s4_question_probe" -ConversationID $conversationId -Message $question -Context $selectionContext
    $probeStop = [string](Get-Prop $probe "stop_reason")
    $s4Probe = ""
    $s4Response = $null
    if ($probeStop -eq "edu_jam_recovery_broadcast") {
        $s4Probe = "recovery_broadcast"
        $s4Response = Invoke-LlmChat -Name "s4_question" -ConversationID $conversationId -Message $question -Context $selectionContext
    }
    elseif ($probeStop -eq "done") {
        $s4Probe = "direct_llm"
        $s4Response = $probe
    }
    else {
        Fail "llm" ("S4 probe stop_reason=" + $probeStop + " (expected recovery_broadcast or done); reply=" + [string](Get-Prop $probe "reply"))
    }
    $s4Stop = [string](Get-Prop $s4Response "stop_reason")
    if ($s4Stop -ne "done") {
        Fail "llm" ("S4 stop_reason=" + $s4Stop + " goal_status=" + [string](Get-Prop $s4Response "goal_status") + " error=" + [string](Get-Prop $s4Response "error") + " reply=" + [string](Get-Prop $s4Response "reply"))
    }
    $reply = [string](Get-Prop $s4Response "reply")
    if ([string]::IsNullOrWhiteSpace($reply)) { Fail "llm" "S4 reply is empty" }
    $keywordHits = @()
    foreach ($keyword in @("第", "关", "目标", "旋律", "走向", "两小节", "曲线", "平直", "平缓", "跌宕", "节奏")) {
        if ($reply.Contains($keyword)) { $keywordHits += $keyword }
    }
    $s4.probe = $s4Probe
    $s4.stop_reason = $s4Stop
    $s4.goal_status = [string](Get-Prop $s4Response "goal_status")
    $s4.reply = $reply
    $s4.reply_keyword_hits = $keywordHits
    if ($keywordHits.Count -eq 0) {
        Write-WarnLine "S4 reply had no tolerant keyword hit; recorded for lead review, not a hard fail"
    }
    $s4.question = $question
    $s4.current_node = $currentNode
    $s4.prompt_render_path = $promptRenderPath
    $s4.prompt_offset = $promptOffset
    $s4.legacy_context_snapshot_path = $contextSnapshotPath
    $s4.legacy_context_offset = $contextOffset

    Start-Sleep -Milliseconds 500
    $promptMatch = Find-PromptRenderLesson -Path $promptRenderPath -Offset $promptOffset -ConversationId $conversationId
    if ($null -eq $promptMatch) { Fail "evidence" ("prompt render file not found/readable: " + $promptRenderPath) }
    $promptHasLesson = ($null -ne $promptMatch.lesson)
    $s4.prompt_render_has_lesson_context = $promptHasLesson
    $s4.prompt_render_conversation_lines = $promptMatch.conversation_lines
    if (-not [string]::IsNullOrWhiteSpace($promptMatch.line)) {
        Set-Content -LiteralPath (Join-Path $evidenceDir "s4_prompt_render.jsonl") -Value $promptMatch.line -Encoding UTF8
    }
    if ($null -ne $promptMatch.appended -and $promptMatch.appended.Length -gt 0) {
        Set-Content -LiteralPath (Join-Path $evidenceDir "s4_prompt_render_appended.jsonl") -Value $promptMatch.appended -Encoding UTF8
    }

    $lessonContext = $null
    $matchedLine = ""
    $evidenceSource = ""
    $evidenceField = ""
    if ($promptHasLesson) {
        $lessonContext = $promptMatch.lesson
        $matchedLine = $promptMatch.line
        $evidenceSource = "agent_prompt_render.jsonl"
        $evidenceField = $promptMatch.field
    }
    else {
        # Legacy fallback only: agent_context_snapshots.jsonl is NOT required.
        $fallbackAppended = Read-AppendedText -Path $contextSnapshotPath -Offset $contextOffset -MaxBytes 4194304
        if ($null -ne $fallbackAppended -and $fallbackAppended.Length -gt 0) {
            Set-Content -LiteralPath (Join-Path $evidenceDir "s4_context_snapshots_appended.jsonl") -Value $fallbackAppended -Encoding UTF8
            foreach ($line in [regex]::Split($fallbackAppended, '\r?\n')) {
                $trimmed = $line.TrimStart([char]0xFEFF).Trim()
                if ($trimmed -eq "") { continue }
                try { $snapshot = $trimmed | ConvertFrom-Json } catch { continue }
                if ([string](Get-Prop $snapshot "conversation_id") -ne $conversationId) { continue }
                $candidateLesson = Get-Prop $snapshot "lesson_context"
                if ($null -eq $candidateLesson) { continue }
                $lessonContext = $candidateLesson
                $matchedLine = $trimmed
                $evidenceSource = "agent_context_snapshots.jsonl"
                $evidenceField = "lesson_context"
                break
            }
        }
    }

    if ($null -eq $lessonContext) {
        $diagnostic = "S4 model-input evidence missing for conversation " + $conversationId + ": prompt_render_lines=" + [string]$promptMatch.conversation_lines + " prompt_has_lesson_context=" + [string]$promptHasLesson + " fallback_context_snapshot=" + $contextSnapshotPath + ". The current agent_prompt_render diagnostic logs only the system message (agentloop/message_loop.go:4864-4910); lesson_context is in the user model snapshot (planner/planner.go:223-224, agentloop/message_loop.go:3852-3854). Product diagnostic must log the user/model snapshot or the message loop must append a context snapshot."
        Fail "evidence" $diagnostic
    }
    $s4.lesson_context_source = $evidenceSource
    $s4.lesson_context_field = $evidenceField

    $lessonCard = Get-Prop $lessonContext "card"
    $lessonNode = Get-Prop $lessonContext "node"
    $lessonClip = Get-Prop $lessonContext "selected_clip"
    $lessonSchema = [string](Get-Prop $lessonContext "schema_version")
    $lessonCardId = [string](Get-Prop $lessonCard "id")
    $lessonNodeId = [string](Get-Prop $lessonNode "id")
    $lessonClipId = [string](Get-Prop $lessonClip "clip_id")
    $lessonClipTrack = [string](Get-Prop $lessonClip "track_id")
    Assert-True ($lessonSchema -eq "vit.edu_jam.lesson_context.v1") ("lesson_context schema=" + $lessonSchema)
    Assert-True ($lessonCardId -eq $cardId) ("lesson_context card.id=" + $lessonCardId + " expected=" + $cardId)
    Assert-True ($lessonNodeId -eq $currentNode) ("lesson_context node.id=" + $lessonNodeId + " expected current=" + $currentNode)
    Assert-True ($lessonClipId -eq $secondClipId) ("lesson_context selected_clip.clip_id=" + $lessonClipId + " expected=" + $secondClipId)
    if (-not [string]::IsNullOrWhiteSpace($lessonClipTrack)) {
        Assert-True ($lessonClipTrack -eq $targetTrackId) ("lesson_context selected_clip.track_id=" + $lessonClipTrack + " expected=" + $targetTrackId)
    }
    $lessonJson = $lessonContext | ConvertTo-Json -Depth 20
    if ($evidenceSource -eq "agent_prompt_render.jsonl") {
        Set-Content -LiteralPath (Join-Path $evidenceDir "s4_lesson_context.json") -Value $lessonJson -Encoding UTF8
    }
    else {
        Set-Content -LiteralPath (Join-Path $evidenceDir "s4_context_snapshot.json") -Value $matchedLine -Encoding UTF8
        Set-Content -LiteralPath (Join-Path $evidenceDir "s4_lesson_context.json") -Value $lessonJson -Encoding UTF8
    }
    $s4.lesson_context = [ordered]@{
        schema_version = $lessonSchema
        card_id = $lessonCardId
        node_id = $lessonNodeId
        selected_clip_id = $lessonClipId
        selected_clip_track_id = $lessonClipTrack
    }

    $telemetryAppended = Read-AppendedText -Path $telemetryPath -Offset $telemetryOffset -MaxBytes 2097152
    $telemetryMatched = $null
    if ($null -ne $telemetryAppended -and $telemetryAppended.Length -gt 0) {
        foreach ($line in [regex]::Split($telemetryAppended, '\r?\n')) {
            $trimmed = $line.Trim()
            if ($trimmed -eq "") { continue }
            try { $record = $trimmed | ConvertFrom-Json } catch { continue }
            if ([string](Get-Prop $record "conversation_id") -ne $conversationId) { continue }
            $telemetryMatched = $record
            break
        }
    }
    if ($null -ne $telemetryMatched) {
        $telemetryError = [string](Get-Prop $telemetryMatched "error")
        $s4.telemetry = [ordered]@{
            source = [string](Get-Prop $telemetryMatched "source")
            model = [string](Get-Prop $telemetryMatched "model")
            message_count = Get-Prop $telemetryMatched "message_count"
            total_ms = Get-Prop $telemetryMatched "total_ms"
            usage = Get-Prop $telemetryMatched "usage"
            error = $telemetryError
        }
    }
    else {
        $s4.telemetry = [ordered]@{ found = $false; note = "no new telemetry row for this conversation; prompt render + done is the primary evidence" }
    }
    Write-Ok ("S4: stop=done reply_len=" + [string]$reply.Length + " lesson_source=" + $evidenceSource + " card=" + $lessonCardId + " node=" + $lessonNodeId + " clip=" + $lessonClipId)
    Write-Ok ("S4: reply keywords=" + ($keywordHits -join ","))
    Write-Step "S5a no-selection control (scope=first, not a selected pass)"
    $s5aResponse = Invoke-AgentChat -Name "s5a_no_selection" -ConversationID $conversationId -Message "写好了" -Context @{}
    $s5aStop = [string](Get-Prop $s5aResponse "stop_reason")
    Assert-True ($s5aStop -eq "edu_jam_node_done") ("S5a stop_reason=" + $s5aStop + " reply=" + [string](Get-Prop $s5aResponse "reply"))
    $s5aWorkflow = Get-WorkflowData $s5aResponse
    $s5aScope = [string](Get-Prop $s5aWorkflow "clip_scope")
    Assert-True ($s5aScope -eq "first") ("S5a clip_scope=" + $s5aScope + " expected=first")
    $s5aClipId = Get-ClipIdForSlot -WorkflowData $s5aWorkflow -Slot $targetSlot
    Assert-True ($s5aClipId -eq $firstClipId) ("S5a clip_ids[" + $targetSlot + "]=" + $s5aClipId + " expected first=" + $firstClipId)
    $s5aSelected = [string](Get-Prop $s5aWorkflow "selected_clip_id")
    Assert-True ([string]::IsNullOrWhiteSpace($s5aSelected)) ("S5a selected_clip_id should be absent, got " + $s5aSelected)
    $s5aNodeDone = [string](Get-Prop $s5aWorkflow "node_done")
    Assert-True ($s5aNodeDone -eq $currentNode) ("S5a node_done=" + $s5aNodeDone + " expected=" + $currentNode)
    $s5aNext = [string](Get-Prop $s5aWorkflow "current_node")
    $s5a.node_done = [string](Get-Prop $s5aWorkflow "node_done")
    $s5a.stop_reason = $s5aStop
    $s5a.clip_scope = $s5aScope
    $s5a.clip_id = $s5aClipId
    $s5a.selected_clip_id = ""
    $s5a.selected_used = $false
    $s5a.pass_basis = "first_clip_of_target_slot"
    $currentNode = $s5aNext
    Write-Ok ("S5a: scope=first clip=" + $s5aClipId + " (selected_used=false)")

    Write-Step "S5b cross-slot selection (scope=selection_unmatched, not a selected pass)"
    $crossClipId = New-MidiClip -Name "s5b_create_cross" -TrackId $firstTrackId -StartBeats 16 -LengthBeats 4
    $crossNotes = @(
        @{ pitch = 48; start = 0; length = 0.5; velocity = 50 },
        @{ pitch = 50; start = 0.5; length = 2; velocity = 55 }
    )
    $null = Add-MidiNotes -Name "s5b_notes_cross" -ClipId $crossClipId -Notes $crossNotes
    $crossRead = @(Read-ClipNotes -Name "s5b_read_cross" -ClipId $crossClipId)
    $crossSignature = Get-NoteSignature $crossRead
    Assert-True ($crossRead.Count -eq 2) ("S5b cross clip notes=" + [string]$crossRead.Count)
    $selectCrossResult = [ordered]@{}
    $selectCrossError = ""
    try {
        $selectCross = Invoke-Tool -Name "s5b_select_cross" -Tool "select_clip" -ToolArgs @{ clip_id = $crossClipId; track_id = $firstTrackId } -Confirmed $false
        $selectCrossResult = Get-Result $selectCross
        if (([string](Get-Prop $selectCrossResult "clip_id")) -ne $crossClipId) {
            $selectCrossError = "select_clip returned clip_id=" + [string](Get-Prop $selectCrossResult "clip_id") + " expected=" + $crossClipId
        }
    }
    catch {
        $selectCrossError = $_.Exception.Message
    }
    if (-not [string]::IsNullOrWhiteSpace($selectCrossError)) {
        Write-WarnLine ("select_clip(cross-slot) is UI-only/unavailable; continuing with the chat-context binding: " + $selectCrossError)
    }
    $crossContext = New-SelectionContext -ClipId $crossClipId -TrackId $firstTrackId
    $s5bResponse = Invoke-AgentChat -Name "s5b_check_cross_slot" -ConversationID $conversationId -Message "写好了" -Context $crossContext
    $s5bStop = [string](Get-Prop $s5bResponse "stop_reason")
    Assert-True ($s5bStop -eq "edu_jam_node_done") ("S5b stop_reason=" + $s5bStop + " reply=" + [string](Get-Prop $s5bResponse "reply"))
    $s5bWorkflow = Get-WorkflowData $s5bResponse
    $s5bScope = [string](Get-Prop $s5bWorkflow "clip_scope")
    Assert-True ($s5bScope -eq "selection_unmatched") ("S5b clip_scope=" + $s5bScope + " expected=selection_unmatched")
    $s5bSelected = [string](Get-Prop $s5bWorkflow "selected_clip_id")
    Assert-True ($s5bSelected -eq $crossClipId) ("S5b selected_clip_id=" + $s5bSelected + " expected=" + $crossClipId)
    $s5bClipId = Get-ClipIdForSlot -WorkflowData $s5bWorkflow -Slot $targetSlot
    Assert-True ($s5bClipId -ne $crossClipId) ("S5b cross-slot clip impersonated the judgement clip: " + $crossClipId)
    Assert-True (($s5bClipId -eq $firstClipId) -or ($s5bClipId -eq $secondClipId)) ("S5b target slot clip=" + $s5bClipId + " expected first or second")
    $s5bNodeDone = [string](Get-Prop $s5bWorkflow "node_done")
    Assert-True ($s5bNodeDone -eq $currentNode) ("S5b node_done=" + $s5bNodeDone + " expected=" + $currentNode)
    $s5bNext = [string](Get-Prop $s5bWorkflow "current_node")
    $s5b.node_done = [string](Get-Prop $s5bWorkflow "node_done")
    $s5b.stop_reason = $s5bStop
    $s5b.clip_scope = $s5bScope
    $s5b.selected_clip_id = $s5bSelected
    $s5b.clip_id = $s5bClipId
    $s5b.cross_clip_id = $crossClipId
    $s5b.cross_notes = $crossSignature
    $s5b.select_clip_ui_action = [string](Get-Prop $selectCrossResult "ui_action")
    $s5b.select_clip_error = $selectCrossError
    $s5b.selected_used = $false
    $s5b.pass_basis = "first_clip_of_target_slot"
    $s5b.differential_note = "node passed by the target slot's first clip; it is NOT a selected-clip pass. selection_unmatched is disclosed."
    $currentNode = $s5bNext
    Write-Ok ("S5b: scope=selection_unmatched selected=" + $crossClipId + " used_clip=" + $s5bClipId + " (selected_used=false)")

    Write-Step "S6 exit -> reopen A -> natural sentence -> auto-intro/recovery"
    $exitResponse = Invoke-AgentChat -Name "s6_exit" -ConversationID $conversationId -Message "结束乐局" -Context @{}
    $exitStop = [string](Get-Prop $exitResponse "stop_reason")
    Assert-True ($exitStop -eq "edu_jam_exited") ("S6 exit stop_reason=" + $exitStop + " reply=" + [string](Get-Prop $exitResponse "reply"))
    $s6.exit = [ordered]@{ stop_reason = $exitStop; status = [string](Get-Prop (Get-WorkflowData $exitResponse) "status") }
    $mainSessionActive = $false

    $openA = Invoke-Tool -Name "s6_project_open_a" -Tool "project.open" -ToolArgs @{ file_path = $projectA }
    $reopenState = Wait-ActiveProject -Path $projectA -Seconds ([Math]::Max(30, [Math]::Min($TimeoutSeconds, 120)))
    $reopenCurrent = Get-Prop $reopenState "current_project"
    $reopenUuid = [string](Get-Prop $reopenCurrent "uuid")
    if ([string]::IsNullOrWhiteSpace($reopenUuid)) {
        $reopenHistory = Get-Prop $reopenState "project_history"
        $reopenUuid = [string](Get-Prop $reopenHistory "project_uuid")
    }
    if ([string]::IsNullOrWhiteSpace($reopenUuid)) {
        $reopenShadow = Get-Prop $reopenState "shadow"
        $reopenUuid = [string](Get-Prop $reopenShadow "project_uuid")
    }
    Assert-True ($reopenUuid -eq $projectUuid) ("reopen project uuid=" + $reopenUuid + " expected=" + $projectUuid)

    if (-not (Test-Path -LiteralPath $bindingPath)) { Fail "s6" ("binding file missing after reopen: " + $bindingPath) }
    $binding = Read-JsonFile -Path $bindingPath
    $openedBefore = [string](Get-Prop $binding "opened_at")
    $s6.opened_at_before = $openedBefore
    if (-not [string]::IsNullOrWhiteSpace($openedBefore)) {
        Fail "s6" ("binding.opened_at is already set (" + $openedBefore + "); a natural sentence would be silent auto-intro and fall into a second LLM turn. S6 not exercised.")
    }
    $s6ConversationId = $conversationId + "-reopen"
    $reopenResponse = Invoke-AgentChat -Name "s6_reopen_natural" -ConversationID $s6ConversationId -Message "我回来了" -Context @{}
    $reopenStop = [string](Get-Prop $reopenResponse "stop_reason")
    if (($reopenStop -ne "edu_jam_entered") -and ($reopenStop -ne "edu_jam_recovery_broadcast")) {
        Fail "s6" ("S6 reopen natural stop_reason=" + $reopenStop + " (expected auto-intro entered/recovery_broadcast); reply=" + [string](Get-Prop $reopenResponse "reply"))
    }
    $reopenWorkflow = Get-WorkflowData $reopenResponse
    $reopenLevel = [string](Get-Prop $reopenWorkflow "level_id")
    Assert-True ($reopenLevel -eq $cardId) ("S6 reopen level_id=" + $reopenLevel + " expected=" + $cardId)
    $reopenNode = [string](Get-Prop $reopenWorkflow "current_node")
    Assert-True (-not [string]::IsNullOrWhiteSpace($reopenNode)) "S6 reopen current_node is empty"
    $bindingAfter = Read-JsonFile -Path $bindingPath
    $openedAfter = [string](Get-Prop $bindingAfter "opened_at")
    $s6CloseConversationId = $s6ConversationId
    if ($reopenStop -eq "edu_jam_recovery_broadcast") {
        $ownerConversation = [string](Get-Prop $reopenWorkflow "owner_conversation_id")
        if (-not [string]::IsNullOrWhiteSpace($ownerConversation)) { $s6CloseConversationId = $ownerConversation }
    }
    $s6.stop_reason = $reopenStop
    $s6.conversation_id = $s6ConversationId
    $s6.close_conversation_id = $s6CloseConversationId
    $s6.level_id = $reopenLevel
    $s6.current_node = $reopenNode
    $s6.reply = [string](Get-Prop $reopenResponse "reply")
    $s6.opened_at_after = $openedAfter
    $s6.writeback_observed = (-not [string]::IsNullOrWhiteSpace($openedAfter))
    $s6SessionActive = $true
    Write-Ok ("S6: " + $reopenStop + " level=" + $reopenLevel + " node=" + $reopenNode + " opened_at_after=" + $openedAfter)

    $summary.status = "passed"
}
catch {
    $raw = $_.Exception.Message
    if ($raw -like "W02A1C|*") {
        $parts = $raw.Split([char]'|', 3)
        if ($parts.Count -ge 3) {
            $script:failureClass = $parts[1]
            $script:failureMessage = $parts[2]
        }
        else {
            $script:failureClass = "harness"
            $script:failureMessage = $raw
        }
    }
    else {
        $script:failureClass = "harness"
        $position = ""
        try { $position = $_.InvocationInfo.PositionMessage } catch { }
        $script:failureMessage = $raw + " " + $position
    }
    $summary.status = "failed"
    $summary.failure_class = $script:failureClass
    $summary.error = $script:failureMessage
    Write-Host ("FAIL [" + $script:failureClass + "]: " + $script:failureMessage) -ForegroundColor Red
}
finally {
    Write-Step "Teardown (restore authoritative files/authority; no stack stop)"
    $teardownStatus = "ok"
    $teardownErrors = @()

    if ($s6SessionActive -and -not [string]::IsNullOrWhiteSpace($s6CloseConversationId)) {
        try {
            $closeResponse = Invoke-AgentChat -Name "teardown_close_s6_session" -ConversationID $s6CloseConversationId -Message "结束乐局" -Context @{}
            $closeStop = [string](Get-Prop $closeResponse "stop_reason")
            $teardown.s6_session_close = [ordered]@{ stop_reason = $closeStop; accepted = (@("edu_jam_exited", "edu_jam_terminal") -contains $closeStop) }
            if (-not ($teardown.s6_session_close.accepted)) { $teardownErrors += ("S6 session close stop_reason=" + $closeStop) }
            $s6SessionActive = $false
        }
        catch {
            $teardown.s6_session_close = [ordered]@{ error = $_.Exception.Message }
            $teardownErrors += ("S6 session close failed: " + $_.Exception.Message)
        }
    }

    if ($mainSessionActive) {
        try {
            $mainClose = Invoke-AgentChat -Name "teardown_close_main_session" -ConversationID $conversationId -Message "结束乐局" -Context @{}
            $mainCloseStop = [string](Get-Prop $mainClose "stop_reason")
            $teardown.main_session_close = [ordered]@{ stop_reason = $mainCloseStop; accepted = (@("edu_jam_exited", "edu_jam_terminal") -contains $mainCloseStop) }
            if (-not ($teardown.main_session_close.accepted)) { $teardownErrors += ("main session close stop_reason=" + $mainCloseStop) }
            $mainSessionActive = $false
        }
        catch {
            $teardown.main_session_close = [ordered]@{ error = $_.Exception.Message }
            $teardownErrors += ("main session close failed: " + $_.Exception.Message)
        }
    }

    try {
        $teardown.recents = Restore-File -Info $recentsSnapshot
        if (-not (Get-Bool (Get-Prop $teardown.recents "restored"))) { $teardownErrors += ("recents restore failed: " + [string](Get-Prop $teardown.recents "error")) }
        $teardown.settings = Restore-File -Info $settingsSnapshot
        if (-not (Get-Bool (Get-Prop $teardown.settings "restored"))) { $teardownErrors += ("Settings.xml restore failed: " + [string](Get-Prop $teardown.settings "error")) }
        $teardown.default_project = Restore-File -Info $defaultProjectSnapshot
        if (-not (Get-Bool (Get-Prop $teardown.default_project "restored"))) { $teardownErrors += ("default_project.xml restore failed: " + [string](Get-Prop $teardown.default_project "error")) }
    }
    catch {
        $teardownErrors += ("file restore exception: " + $_.Exception.Message)
    }

    try {
        $authorityNow = Invoke-Api -Name "teardown_authority_get" -Method "Get" -Path "/agent/authority" -Body $null
        $authorityCurrent = [string](Get-Prop $authorityNow "authority_mode")
        $teardown.authority = [ordered]@{ before = $authorityBefore; current = $authorityCurrent; restored = $false }
        if ($authorityCurrent -ne $authorityBefore -and -not [string]::IsNullOrWhiteSpace($authorityBefore)) {
            $authorityPost = Invoke-Api -Name "teardown_authority_restore" -Method "Post" -Path "/agent/authority" -Body @{ authority_mode = $authorityBefore }
            $authorityAfter = [string](Get-Prop $authorityPost "authority_mode")
            $teardown.authority.restored = ($authorityAfter -eq $authorityBefore)
            $teardown.authority.after = $authorityAfter
            if (-not $teardown.authority.restored) { $teardownErrors += ("authority restore mismatch: " + $authorityAfter + " expected=" + $authorityBefore) }
        }
        else {
            $teardown.authority.restored = $true
        }
    }
    catch {
        $teardown.authority = [ordered]@{ before = $authorityBefore; error = $_.Exception.Message; restored = $false }
        $teardownErrors += ("authority teardown failed: " + $_.Exception.Message)
    }

    try {
        $finalRuntime = Invoke-Api -Name "teardown_runtime_status" -Method "Get" -Path "/agent/runtime/status" -Body $null
        $finalGoal = Get-Prop $finalRuntime "goal"
        $finalGoalStatus = [string](Get-Prop $finalGoal "status")
        if ([string]::IsNullOrWhiteSpace($finalGoalStatus)) { $finalGoalStatus = "idle" }
        $inactive = Test-InactiveGoalStatus $finalGoalStatus
        $teardown.runtime = [ordered]@{ goal_status = $finalGoalStatus; inactive = $inactive }
        if (-not $inactive) { $teardownErrors += ("agent runtime still has an active turn at teardown: " + $finalGoalStatus) }
    }
    catch {
        $teardown.runtime = [ordered]@{ error = $_.Exception.Message; idle = $false }
        $teardownErrors += ("runtime status teardown failed: " + $_.Exception.Message)
    }

    try {
        if ($null -ne $kernelLogInfo) {
            $append = Read-AppendedText -Path ([string]$kernelLogInfo.path) -Offset ([long]$kernelLogInfo.length) -MaxBytes 2097152
            if ($null -ne $append -and $append.Length -gt 0) {
                Set-Content -LiteralPath (Join-Path $evidenceDir "kernel_log_appended.txt") -Value $append -Encoding UTF8
                $teardown.kernel_log_appended_bytes = $append.Length
            }
            else {
                $teardown.kernel_log_appended_bytes = 0
            }
        }
        else {
            $teardown.kernel_log_appended_bytes = 0
            $teardown.kernel_log = "not found (VitHeadlessServer*.log)"
        }
    }
    catch {
        $teardown.kernel_log_error = $_.Exception.Message
    }

    $teardown.status = if ($teardownErrors.Count -eq 0) { "ok" } else { "failed" }
    $teardown.errors = $teardownErrors
    $teardown.completed_at = (Get-Date).ToString("o")
    $teardown.process_tcp_udp_note = "This script never starts/stops the stack; process/TCP/UDP zero must be confirmed by the berth owner after stopping the stack."
    $summary.completed_at = (Get-Date).ToString("o")

    if ($teardownErrors.Count -gt 0 -and $summary.status -eq "passed") {
        $summary.status = "failed"
        $summary.failure_class = "teardown"
        $summary.error = ($teardownErrors -join " | ")
        $script:failureClass = "teardown"
        $script:failureMessage = $summary.error
    }

    try {
        $summary | ConvertTo-Json -Depth 40 | Set-Content -LiteralPath (Join-Path $RunRoot "summary.json") -Encoding UTF8
    }
    catch {
        Write-Host ("error: could not write summary.json: " + $_.Exception.Message) -ForegroundColor Red
        $summary.status = "failed"
        $summary.failure_class = "teardown"
        $summary.error = "could not write summary.json: " + $_.Exception.Message
        $script:failureClass = "teardown"
        $script:failureMessage = $summary.error
    }
}

if ($summary.status -eq "passed") {
    $script:exitCode = 0
    Write-Host ""
    Write-Host "ok: W02-A1c real-stack journey PASSED" -ForegroundColor Green
    Write-Host ("artifact: " + (Join-Path $RunRoot "summary.json"))
    Write-Host ("run_root: " + $RunRoot)
}
else {
    $script:exitCode = switch ($script:failureClass) {
        "assert" { 1 }
        "preflight" { 2 }
        "http" { 2 }
        "teardown" { 2 }
        "llm" { 4 }
        "evidence" { 5 }
        "api" { 5 }
        "s6" { 5 }
        default { 1 }
    }
    Write-Host ""
    Write-Host ("FAILED [" + $script:failureClass + "] exit=" + [string]$script:exitCode) -ForegroundColor Red
    Write-Host ("artifact: " + (Join-Path $RunRoot "summary.json"))
}
exit $script:exitCode
