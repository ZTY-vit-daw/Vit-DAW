[CmdletBinding()]
param(
    # W02-A1c real-stack executor. Zero-stack by default: no stack start, no port.
    [string]$RepoRoot = "D:/Vit_DAW",
    [string]$AgentBaseUrl = "http://127.0.0.1:7878",
    [string]$RuntimeOwner = "",
    [int]$TimeoutSeconds = 180,
    [string]$RunRoot = "",
    # status = A1c S0-S6 journey (default, unchanged); work = S0-S3 + S4W
    # selected-clip work-feedback question (W02-A2a).
    [ValidateSet("status", "work")]
    [string]$QuestionMode = "status",
    # W02-A1-HANDTEST-V0 prep mode (see header): S0-S2 + rename/save/copy/reopen
    # verification. Requires -HandtestRoot; skips S3-S6; zero LLM.
    # BLOCKED for the hand test: project.open archives+blanks the agent
    # conversation (PROJ-OPEN-RESUME-1). Kept for clip-persistence evidence.
    [switch]$HandtestPrep,
    # W02-A1-HANDTEST-V0 same-stack prep (lead adjudication B, 2026-11-19):
    # the project is ALREADY OPEN in the user's stack; prep never calls
    # project.new/project.open. It imports the design source, enters the lesson,
    # advances to the clip node, creates the two clips, renames the tracks,
    # project.save, verifies via the LIVE project.state, and snapshots the
    # project dir to <HandtestRoot>/pristine. The hand test then runs in the
    # SAME stack/window (no close/reopen).
    [switch]$SameStackPrep,
    [string]$ProjectPath = "",
    [string]$HandtestRoot = "",
    [switch]$StackAuthorized,
    [switch]$DryRun
)

# ============================================================================
# W02-A1c real-stack journey executor (S0-S6)
#
# Cards: coord/cards/doing/2026-11-19-W02-A1-LESSON-CTX-CLIP-1.md (A1c)
#        coord/cards/doing/2026-11-19-W02-A2-FEEDBACK-CHECKER-DESIGN.md (A2a)
# Scope: selected-MIDI-clip binding + bounded lesson_context + one real LLM turn
#        + honest failure semantics + reopen/recovery evidence.
#
# -QuestionMode status (default): the A1c S0-S6 journey, behavior/assertions
#   unchanged.
# -QuestionMode work (W02-A2a): reuses S0-S3 (import / enter / two clips /
#   selected second) and replaces S4 with S4W, ONE real-LLM feedback question
#   about the selected clip's actual notes. S5/S6 are A1c status-mode coverage
#   and are not run in work mode.
#
# -SameStackPrep -HandtestRoot <dir> [-ProjectPath <open .vit>] (W02-A1-HANDTEST-V0,
#   card pool 434; lead adjudication B, 2026-11-19): THE HAND-TEST MODE. The
#   project is ALREADY OPEN in the user's running stack; prep never calls
#   project.new / project.save_as / project.open (project.open archives+blanks the
#   agent conversation, PROJ-OPEN-RESUME-1). It runs S0, then in the open
#   project: import W01 design -> 开始乐局 -> S1.5 advance to the first clip node
#   -> clear stale clips on the target/reference tracks -> two distinguishable
#   MIDI clips -> rename the tracks -> project.save -> verify the LIVE
#   project.state (two tracks / 2 clips / exact notes) -> 乐局状态 at the clip
#   node -> snapshot the project dir to <dir>/pristine. S3-S6 are skipped and no
#   LLM is consumed. The user then continues IN THE SAME WINDOW (no close/reopen);
#   reset = re-run prep. Missing/unmatching open project = exit 2 before any
#   project mutation.
#
# -HandtestPrep -HandtestRoot <dir> (reopen mode, BLOCKED for hand tests by
#   PROJ-OPEN-RESUME-1; kept for clip-persistence evidence): reuse
#   S0/S1/S1.5/S2 (import / enter / advance to the first clip node / two
#   distinguishable MIDI clips / selected second), then rename the target and
#   reference tracks, project.save (tracks+clips persistence), copy the project
#   directory (.vit/.vit_project/.vit_agent/.vit_history) to <dir>/live and
#   <dir>/pristine, reopen the live copy and assert that project.state still has
#   the two renamed user tracks and the two clips with their exact notes, and
#   that the prep conversation is still at the clip node ("乐局状态" ->
#   edu_jam_status with current_node == the clip node). S3/S4/S5/S6 are NOT run
#   and NO LLM turn is consumed: the user's hand test performs the "写好了"
#   advance itself. The prep conversation is left ACTIVE (teardown does not send
#   结束乐局) so the fixture stays at n2; recents / Settings.xml /
#   default_project.xml / authority are still restored and hashed.
#   A lost clip/track/node after reopen is exit 1; a missing persistence face,
#   a zero-LLM violation or an unreadable fixture face is exit 5.
#   Evidence: a notes-read receipt (get_midi_clip_notes / midi.read_notes or the
#   midi.read_clip_data equivalents) for the selected clip must be found on at
#   least one of: the /agent/events tool timeline, the model-input prompt render
#   (<tool_result> for the clip), or response.executed_kernel_reply. The kernel
#   log payload is recorded as supporting evidence. If the model claims note
#   content ("我看到/我听到/具体音高是...") without any receipt, the run fails
#   honestly (exit 5). If the model honestly says it needs more information, the
#   run ends status=model_boundary (exit 5, lead adjudication) - never a PASS.
#
# Exit codes:
#   0 = PASS (all asserted stages passed; teardown restored)
#   1 = deterministic assertion failure (product/contract)
#   2 = preflight/environment/teardown failure (stack, HTTP, restore)
#   3 = not authorized (runbook summary only; no HTTP, no writes)
#   4 = real-LLM failure class (timeout/empty/stop_reason/telemetry error)
#   5 = evidence/API-coverage gap (no fake PASS); in work mode also a false
#       note-content claim, or model_boundary (honest "need more info")
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

# ---------------------------------------------------------------------------
# W02-A2a work-question evidence helpers.
#
# The LLM-facing notes tool is midi.read_notes; the kernel command behind it is
# get_midi_clip_notes (agent/internal/tools/catalog.go). Both names, plus the
# midi.read_clip_data equivalents, count as a notes receipt. Evidence is read
# from independent faces: /agent/events tool items (in-memory timeline),
# agent_prompt_render.jsonl <tool_result> rows (model input), the kernel log
# command payloads (persistent receipt), and response.executed_kernel_reply.
# ---------------------------------------------------------------------------
$script:NotesReadToolNames = @(
    "get_midi_clip_notes",
    "midi.read_notes",
    "midi.read_clip_notes",
    "get_midi_clip_data",
    "midi.read_clip_data"
)

function Test-NotesReadToolName {
    param([string]$Name)
    if ([string]::IsNullOrWhiteSpace($Name)) { return $false }
    return ($script:NotesReadToolNames -contains $Name.Trim().ToLowerInvariant())
}

# Recursively find a clip_id in a nested args/command/result object. Model tool
# calls usually carry it at the top level, but legacy command payloads nest it
# under command.legacy.args; this executor must stay tolerant of both shapes.
function Find-ClipIdInObject {
    param($Object, [int]$Depth = 0)
    if ($null -eq $Object -or $Depth -gt 5) { return "" }
    $direct = Get-Prop $Object "clip_id"
    if ($null -ne $direct -and -not [string]::IsNullOrWhiteSpace([string]$direct)) { return [string]$direct }
    if ($Object -is [System.Collections.IDictionary]) {
        foreach ($key in $Object.Keys) {
            $value = $Object[$key]
            if ($value -is [System.Collections.IDictionary] -or $value -is [System.Management.Automation.PSCustomObject]) {
                $found = Find-ClipIdInObject -Object $value -Depth ($Depth + 1)
                if (-not [string]::IsNullOrWhiteSpace($found)) { return $found }
            }
        }
    }
    elseif ($Object -is [System.Management.Automation.PSCustomObject]) {
        foreach ($prop in $Object.PSObject.Properties) {
            $value = $prop.Value
            if ($value -is [System.Collections.IDictionary] -or $value -is [System.Management.Automation.PSCustomObject]) {
                $found = Find-ClipIdInObject -Object $value -Depth ($Depth + 1)
                if (-not [string]::IsNullOrWhiteSpace($found)) { return $found }
            }
        }
    }
    return ""
}

# Flatten /agent/events rows into the tool-call rows this executor inspects.
# Tool items carry payload.tool (LLM name), payload.command_name (kernel name),
# payload.args.clip_id and payload.result.notes.
function Get-ToolCallRowsFromEvents {
    param($EventsResponse)
    $rows = @()
    if ($null -eq $EventsResponse) { return @() }
    foreach ($event in @(Get-Prop $EventsResponse "events")) {
        if ($null -eq $event) { continue }
        $type = [string](Get-Prop $event "type")
        if (@("item.started", "item.completed", "approval.requested") -notcontains $type) { continue }
        $payload = Get-Prop $event "payload"
        if ($null -eq $payload) { continue }
        $args = Get-Prop $payload "args"
        $commandRaw = Get-Prop $payload "command_raw"
        $result = Get-Prop $payload "result"
        $clipId = Find-ClipIdInObject -Object $args
        if ([string]::IsNullOrWhiteSpace($clipId)) { $clipId = Find-ClipIdInObject -Object $commandRaw }
        if ([string]::IsNullOrWhiteSpace($clipId)) { $clipId = Find-ClipIdInObject -Object (Get-Prop $payload "command") }
        if ([string]::IsNullOrWhiteSpace($clipId)) { $clipId = Find-ClipIdInObject -Object $result }
        $notesCount = -1
        $notes = Get-Prop $result "notes"
        if ($null -ne $notes) { $notesCount = @($notes).Count }
        $rows += [pscustomobject]@{
            seq = (Get-Prop $event "seq")
            type = $type
            status = [string](Get-Prop $event "status")
            item_id = [string](Get-Prop $event "item_id")
            tool = [string](Get-Prop $payload "tool")
            command_name = [string](Get-Prop $payload "command_name")
            command = [string](Get-Prop $payload "command")
            clip_id = $clipId
            notes_count = $notesCount
            error = [string](Get-Prop $payload "error")
        }
    }
    return @($rows)
}

# Scan newly appended agent_prompt_render.jsonl rows for a model-input tool
# receipt: a <tool_result> block that mentions a notes tool and the selected
# clip id. The system prompt alone lists the tool catalog, so the
# <tool_result> marker is required to avoid a false positive.
function Find-PromptRenderToolResults {
    param([string]$Text, [string]$ConversationId, [string]$ClipId)
    $rows = @()
    if ([string]::IsNullOrEmpty($Text)) { return @() }
    foreach ($line in [regex]::Split($Text, '\r?\n')) {
        $trimmed = $line.TrimStart([char]0xFEFF).Trim()
        if ($trimmed -eq "") { continue }
        try { $row = $trimmed | ConvertFrom-Json } catch { continue }
        if ([string](Get-Prop $row "conversation_id") -ne $ConversationId) { continue }
        # The three markers must share one field: the system prompt lists the
        # tool catalog and the snapshot carries the selection, so joining fields
        # would manufacture a receipt that the model never received.
        foreach ($name in @("model_snapshot", "system_prompt", "turn_directives")) {
            $blob = Get-Prop $row $name
            if ($blob -isnot [string] -or $blob.Length -eq 0) { continue }
            # Go's default JSON HTML escaping can render the marker as
            # \u003ctool_result\u003e inside a doubly-encoded model_snapshot;
            # accept both forms (the current encoder writes the literal one).
            $hasToolResult = ($blob.Contains("<tool_result>") -or $blob.Contains('\u003ctool_result\u003e'))
            if (-not $hasToolResult) { continue }
            if (-not $blob.Contains($ClipId)) { continue }
            $toolHits = @()
            foreach ($tool in $script:NotesReadToolNames) {
                if ($blob.IndexOf($tool, [System.StringComparison]::OrdinalIgnoreCase) -ge 0) { $toolHits += $tool }
            }
            if ($toolHits.Count -eq 0) { continue }
            $rows += [pscustomobject]@{
                at = [string](Get-Prop $row "at")
                turn = (Get-Prop $row "turn")
                tag = [string](Get-Prop $row "tag")
                field = $name
                tool_hits = ($toolHits -join ",")
                has_notes_key = ($blob -match '\\?"notes\\?"\s*:')
                text_length = $blob.Length
            }
        }
    }
    return @($rows)
}

# Scan a kernel-log slice for a notes-read command payload targeting the
# selected clip. The kernel logs its compact JSON args, so both compact and
# spaced clip_id forms are accepted. "Command executed" lines prove the kernel
# replied, but they do not carry the clip id; keep them as a separate count.
function Find-KernelNotesRead {
    param([string]$Text, [string]$ClipId)
    if ([string]::IsNullOrEmpty($Text)) {
        return [pscustomobject]@{ received = 0; executed_command_lines = 0; tool_call_id_hits = 0; samples = @() }
    }
    $received = 0
    $executedLines = 0
    $toolCallIDHits = 0
    $samples = @()
    $cmdPattern = '"cmd"\s*:\s*"(get_midi_clip_notes|get_midi_clip_data)"'
    $clipCompact = '"clip_id":"' + $ClipId + '"'
    $clipSpaced = '"clip_id": "' + $ClipId + '"'
    foreach ($line in [regex]::Split($Text, '\r?\n')) {
        if ($line -eq "") { continue }
        if (($line -match $cmdPattern) -and ($line.Contains($clipCompact) -or $line.Contains($clipSpaced))) {
            $received++
            if ($line -match '"tool_call_id"') { $toolCallIDHits++ }
            if ($samples.Count -lt 3) { $samples += $line.Substring(0, [Math]::Min(600, $line.Length)) }
        }
        if ($line -match 'Command executed: get_midi_clip_(notes|data)') { $executedLines++ }
    }
    return [pscustomobject]@{ received = $received; executed_command_lines = $executedLines; tool_call_id_hits = $toolCallIDHits; samples = @($samples) }
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

# ---------------------------------------------------------------------------
# W02-A1-HANDTEST-V0 handtest prep helpers (prep mode only; zero LLM).
# The prep leg stops at the clip node: the user's hand test does the advance.
# ---------------------------------------------------------------------------

function Get-TreeDigest {
    # Deterministic tree digest ("<relative path>:<sha256>" lines, sorted) so a
    # live/pristine copy pair can be compared without external tools.
    param([string]$Path)
    $out = [ordered]@{ path = $Path; files = 0; bytes = [long]0; sha256 = "" }
    if ([string]::IsNullOrWhiteSpace($Path) -or -not (Test-Path -LiteralPath $Path)) { return $out }
    $root = [System.IO.Path]::GetFullPath($Path)
    $lines = @()
    foreach ($file in @(Get-ChildItem -LiteralPath $root -Recurse -File -Force | Sort-Object FullName)) {
        $rel = $file.FullName.Substring($root.Length)
        $rel = $rel.TrimStart([char[]]@([char]92, [char]47))
        $lines += ($rel + ":" + (Get-FileSha256 $file.FullName))
        $out.files++
        $out.bytes += [long]$file.Length
    }
    $sha = [System.Security.Cryptography.SHA256]::Create()
    try {
        $hash = $sha.ComputeHash([System.Text.Encoding]::UTF8.GetBytes(($lines -join ([string][char]10))))
        $out.sha256 = [System.BitConverter]::ToString($hash).Replace("-", "")
    }
    finally { $sha.Dispose() }
    return $out
}

function Get-TrackClips {
    # project.state rows expose clips[] (clip_summaries is the fallback key).
    param($Track)
    $clips = Get-Prop $Track "clips"
    if ($null -eq $clips) { $clips = Get-Prop $Track "clip_summaries" }
    if ($null -eq $clips) { return @() }
    return @($clips)
}

function Find-EduJamStateProbe {
    # Supporting cold-start probe: the saved agent runtime state under the
    # fixture should carry the prep conversation with the clip node active.
    # This is file-level evidence for the NEXT cold start; the primary check is
    # the product chat face (乐局状态). An in-process reopen alone cannot prove
    # a cold restore, so this probe is recorded and disclosed, never inflated.
    param([string]$ProjectDir, [string]$ConversationId, [string]$NodeId)
    $out = [ordered]@{
        result = "not_found"
        file = ""
        checked_files = 0
        conversation_found = $false
        node_state = ""
        note = ""
    }
    if ([string]::IsNullOrWhiteSpace($ProjectDir) -or -not (Test-Path -LiteralPath $ProjectDir)) {
        $out.note = "fixture dir missing"
        return $out
    }
    $files = @(Get-ChildItem -LiteralPath $ProjectDir -Recurse -File -Force -Filter "agent_runtime_state.json" -ErrorAction SilentlyContinue | Sort-Object LastWriteTimeUtc -Descending)
    $nodeNeedle = ('"node_id"\s*:\s*"' + [regex]::Escape($NodeId) + '"')
    foreach ($stateFile in $files) {
        $out.checked_files++
        $text = ""
        try { $text = [System.IO.File]::ReadAllText($stateFile.FullName) } catch { continue }
        if (-not $text.Contains($ConversationId)) { continue }
        $out.file = $stateFile.FullName
        $out.conversation_found = $true
        $match = [regex]::Match($text, ($nodeNeedle + '\s*,\s*"state"\s*:\s*"([A-Za-z_]+)"'))
        if (-not $match.Success) {
            $match = [regex]::Match($text, ($nodeNeedle + '[\s\S]{0,200}?"state"\s*:\s*"([A-Za-z_]+)"'))
        }
        if ($match.Success) {
            $out.node_state = $match.Groups[1].Value
            if ($out.node_state -eq "active") { $out.result = "node_active" }
            else { $out.result = "conversation_without_active_node" }
        }
        else {
            $out.result = "conversation_without_node_state"
        }
        return $out
    }
    $out.note = "no agent_runtime_state.json carrying the prep conversation under the fixture"
    return $out
}

function New-HandtestPrepNotes {
    # Prep explanation generated from the summary (no hand-written claims).
    param($Summary)
    $prep = Get-Prop $Summary "handtest_prep"
    $rename = Get-Prop $prep "rename"
    $save = Get-Prop $prep "save"
    $copies = Get-Prop $prep "copies"
    $liveState = Get-Prop $prep "live_state"
    if ($null -eq $liveState) { $liveState = Get-Prop $prep "reopen" }
    $conversation = Get-Prop $prep "conversation"
    $probe = Get-Prop $prep "restore_probe"
    $zero = Get-Prop $prep "zero_llm"
    $mode = [string](Get-Prop $prep "mode")
    $modeText = "重开（reopen，已阻塞：PROJ-OPEN-RESUME-1）"
    $verifyLabel = "重开 live"
    $conversationLabel = "重开后 prep 会话"
    $modeHint = "- 模式：-HandtestPrep（S0/S1/S1.5/S2 + rename/save/copy/reopen；不跑 S3-S6、零 LLM、不启停栈）"
    if ($mode -eq "same_stack") {
        $modeText = "同栈（same_stack，手测主用）"
        $verifyLabel = "live 验证（未重开）"
        $conversationLabel = "同栈 prep 会话"
        $modeHint = "- 模式：-SameStackPrep（工程已打开、不 project.open；import/enter/S1.5/S2 + rename/save/copy；不跑 S3-S6、零 LLM、不启停栈）"
    }
    $lines = @()
    $lines += "# W02-A1-HANDTEST-V0 prep 记录（脚本生成）"
    $lines += ""
    $lines += "- 生成时间：" + [string](Get-Prop $Summary "completed_at")
    $lines += "- 结论：" + [string](Get-Prop $Summary "status") + " / failure_class=" + [string](Get-Prop $Summary "failure_class")
    $lines += $modeHint
    $lines += "- 模式标记：" + $modeText
    if (Get-Bool (Get-Prop $prep "blocked_by_proj_open_resume")) {
        $lines += "- **阻塞标记**：blocked_by_proj_open_resume=true（保存点会话在 open 边界被归档清空；此模式不得用于手测，手测用 -SameStackPrep）"
    }
    $lines += "- 栈：" + [string](Get-Prop $Summary "agent_base_url") + "（owner=" + [string](Get-Prop $Summary "runtime_owner") + "）"
    $lines += "- RunRoot（临时工件）：" + [string](Get-Prop $Summary "run_root")
    $lines += "- git head：" + [string](Get-Prop $Summary "git_head")
    $lines += ""
    $lines += "## 预置工程"
    $lines += ""
    $lines += "- 原工程：" + [string](Get-Prop $Summary "project_a")
    $lines += "- live：" + [string](Get-Prop $prep "live_project")
    $lines += "- pristine：" + [string](Get-Prop $prep "pristine_project")
    $lines += '- prep 会话 conversation_id："' + [string](Get-Prop $prep "prep_conversation_id") + '"'
    $lines += '- 预期节点："' + [string](Get-Prop $prep "expected_node") + '"（第 3 关 / clip 节点，乐局已导入、已开局）'
    $lines += '- card_id："' + [string](Get-Prop $Summary "card_id") + '"；level 绑定：' + [string](Get-Prop $Summary "binding_path")
    $lines += ""
    $lines += "## 轨道与 clip"
    $lines += ""
    $lines += "- 目标轨：" + [string](Get-Prop (Get-Prop $rename "target") "name") + "（track_id=" + [string](Get-Prop (Get-Prop $rename "target") "track_id") + "）"
    $lines += "- 参考轨：" + [string](Get-Prop (Get-Prop $rename "reference") "name") + "（track_id=" + [string](Get-Prop (Get-Prop $rename "reference") "track_id") + "）"
    foreach ($row in @(Get-Prop $liveState "clips")) {
        $lines += "- clip_id=" + [string](Get-Prop $row "clip_id") + " start=" + [string](Get-Prop $row "start") + " notes=" + [string](Get-Prop $row "notes")
    }
    $lines += "- 第一个 clip（控制组）：" + [string](Get-Prop $liveState "first_clip_id")
    $lines += "- 第二个 clip（手测要选的那个）：" + [string](Get-Prop $liveState "second_clip_id")
    $lines += ""
    $lines += "## 保存与验证取证"
    $lines += ""
    $lines += '- 保存命令："' + [string](Get-Prop $save "command") + '"，kernel status="' + [string](Get-Prop $save "status") + '"，lifecycle="' + [string](Get-Prop $save "project_lifecycle") + '"'
    $lines += "- .vit 大小 " + [string](Get-Prop $save "size_before") + " -> " + [string](Get-Prop $save "size_after") + " 字节；sha256 " + [string](Get-Prop $save "sha256_before") + " -> " + [string](Get-Prop $save "sha256_after")
    if ($mode -eq "same_stack") {
        $lines += "- pristine 快照：文件数=" + [string](Get-Prop (Get-Prop $copies "pristine") "files") + "，.vit sha256=" + [string](Get-Prop (Get-Prop $copies "pristine") "project_sha256") + "（同栈：reset=重跑 prep，不做文件还原）"
    }
    else {
        $lines += "- 保存后 unsaved 标记：" + [string](Get-Prop $save "unsaved_after_save") + "（不单独判据；以重开取证为准）"
        $lines += "- live/pristine 文件数：" + [string](Get-Prop (Get-Prop $copies "live") "files") + " / " + [string](Get-Prop (Get-Prop $copies "pristine") "files") + "，tree sha256 相等=" + [string](Get-Prop (Get-Prop $copies "same_digest") "value")
    }
    $lines += "- " + $verifyLabel + "：用户轨 " + [string](Get-Prop $liveState "user_track_count") + "（track_count=" + [string](Get-Prop $liveState "track_count") + "），目标轨 clip 数=" + [string](Get-Prop $liveState "target_clip_count") + "，参考轨 clip 数=" + [string](Get-Prop $liveState "reference_clip_count")
    $lines += "- " + $conversationLabel + '：stop_reason="' + [string](Get-Prop $conversation "stop_reason") + '"，current_node="' + [string](Get-Prop $conversation "current_node") + '"，期望 "' + [string](Get-Prop $conversation "expected_node") + '"'
    $lines += "- 持久化状态探针（支撑证据）：" + [string](Get-Prop $probe "result") + "（file=" + [string](Get-Prop $probe "file") + "，node_state=" + [string](Get-Prop $probe "node_state") + "）"
    $lines += "- 零 LLM：telemetry 新增行=" + [string](Get-Prop $zero "llm_rows") + "（本会话），可用=" + [string](Get-Prop $zero "telemetry_available")
    $lines += ""
    $lines += "## 已知边界"
    $lines += ""
    $lines += "- 本脚本只操作**已存在的栈**，不启停进程、不占端口；prep 结束后栈仍由 lead 拥有，由 lead 停栈后核 process/TCP/UDP 零。"
    $lines += "- 重开发生在同一 agent 进程内：它证明 .vit 里有 tracks/clips，也证明 prep 会话在重开后仍报 n2；**冷启动恢复**（新进程/新栈）以手测为准，见 coord/runs/W02-A1-handtest/RUNBOOK.md。"
    $lines += "- teardown **不关闭** prep 会话（否则 fixture 不再是 n2）；recents / Settings.xml / default_project.xml / authority 仍按字节还原。"
    $lines += "- 运行工件（requests/responses/snapshots/evidence）在 RunRoot；本文件与 summary.json 是本轮的 prep 记录。"
    return ($lines -join ([Environment]::NewLine))
}

function Get-HandtestLiveEvidence {
    # Shared live-state verification for a hand-test fixture: two renamed user
    # tracks, exactly two clips on the target track, an empty reference track,
    # and both exact MIDI note signatures read back from the kernel. Used by the
    # same-stack mode (the reopen mode keeps its own inline copy, frozen behind
    # the PROJ-OPEN-RESUME-1 block).
    param(
        $ProjectState,
        [string]$TargetName,
        [string]$ReferenceName,
        [string]$FirstSignature,
        [string]$SecondSignature,
        [string]$FirstClipId,
        [string]$SecondClipId,
        [string]$TargetTrackId,
        [string]$ReferenceTrackId,
        [string]$EvidenceDir,
        [string]$EvidenceName
    )
    $evidence = [ordered]@{}
    if (-not [string]::IsNullOrWhiteSpace($EvidenceDir)) {
        try {
            Set-Content -LiteralPath (Join-Path $EvidenceDir $EvidenceName) -Value (($ProjectState | ConvertTo-Json -Depth 40)) -Encoding UTF8
        }
        catch { }
    }
    $tracks = @(Get-UserTracks $ProjectState)
    $names = @()
    foreach ($track in $tracks) { $names += [string](Get-Prop $track "track_name") }
    Assert-True ($tracks.Count -ge 2) ("live project user tracks=" + [string]$tracks.Count + " names=[" + ($names -join ",") + "]; expected the renamed target + reference tracks")
    $targetRow = $null
    $referenceRow = $null
    foreach ($track in $tracks) {
        $rowName = [string](Get-Prop $track "track_name")
        if ($rowName -eq $TargetName) { $targetRow = $track }
        elseif ($rowName -eq $ReferenceName) { $referenceRow = $track }
    }
    Assert-True ($null -ne $targetRow) ("target track name not found in the live project; names=[" + ($names -join ",") + "]")
    Assert-True ($null -ne $referenceRow) ("reference track name not found in the live project; names=[" + ($names -join ",") + "]")
    $targetClips = @(Get-TrackClips $targetRow)
    Assert-True ($targetClips.Count -eq 2) ("target track clips=" + [string]$targetClips.Count + " expected=2")
    $referenceClips = @(Get-TrackClips $referenceRow)
    Assert-True ($referenceClips.Count -eq 0) ("reference track should stay empty; clips=" + [string]$referenceClips.Count)
    $clips = @()
    $firstFound = ""
    $secondFound = ""
    foreach ($clip in $targetClips) {
        $clipId = Get-ClipId $clip
        Assert-True (-not [string]::IsNullOrWhiteSpace($clipId)) ("target clip row has no clip id: " + (Format-Result $clip))
        $clipNotes = @(Read-ClipNotes -Name ("prep_read_clip_" + $clipId) -ClipId $clipId)
        $clipSignature = Get-NoteSignature $clipNotes
        $clips += [ordered]@{
            clip_id = $clipId
            start = Get-Prop $clip "start"
            length = Get-Prop $clip "length"
            notes = $clipSignature
            notes_count = $clipNotes.Count
        }
        if ($clipSignature -eq $FirstSignature -and [string]::IsNullOrWhiteSpace($firstFound)) { $firstFound = $clipId }
        elseif ($clipSignature -eq $SecondSignature -and [string]::IsNullOrWhiteSpace($secondFound)) { $secondFound = $clipId }
    }
    $signatures = @()
    foreach ($row in $clips) { $signatures += [string](Get-Prop $row "notes") }
    Assert-True (-not [string]::IsNullOrWhiteSpace($firstFound)) ("first clip notes not found; expected=" + $FirstSignature + " actual=[" + ($signatures -join " | ") + "]")
    Assert-True (-not [string]::IsNullOrWhiteSpace($secondFound)) ("second clip notes not found; expected=" + $SecondSignature + " actual=[" + ($signatures -join " | ") + "]")
    Assert-True ($firstFound -ne $secondFound) "control/target clips resolved to the same clip id"
    $evidence.track_count = Get-Prop (Get-Result $ProjectState) "track_count"
    $evidence.user_track_count = $tracks.Count
    $evidence.track_names = $names
    $evidence.target_track_id = Get-TrackId $targetRow
    $evidence.reference_track_id = Get-TrackId $referenceRow
    $evidence.target_track_id_stable = ((Get-TrackId $targetRow) -eq $TargetTrackId)
    $evidence.reference_track_id_stable = ((Get-TrackId $referenceRow) -eq $ReferenceTrackId)
    $evidence.target_clip_count = $targetClips.Count
    $evidence.reference_clip_count = $referenceClips.Count
    $evidence.clips = $clips
    $evidence.first_clip_id = $firstFound
    $evidence.second_clip_id = $secondFound
    $evidence.first_signature = $FirstSignature
    $evidence.second_signature = $SecondSignature
    $evidence.first_clip_id_stable = ($firstFound -eq $FirstClipId)
    $evidence.second_clip_id_stable = ($secondFound -eq $SecondClipId)
    return $evidence
}

function Invoke-SameStackPrep {
    # Lead adjudication B (2026-11-19): the hand test runs in the SAME stack and
    # window where prep ran. The project is already open; this function never
    # calls project.new/project.open (project.open would archive+blank the agent
    # conversation per PROJ-OPEN-RESUME-1). Deterministic commands only.
    param(
        [object]$Summary,
        [string]$ConversationId,
        [string]$ProjectPath,
        [string]$ProjectUuid,
        [string]$CardId,
        [string]$TargetTrackId,
        [string]$ReferenceTrackId,
        [string]$ClipNode,
        [string]$TargetSlot,
        [string]$FirstClipId,
        [string]$SecondClipId,
        [string]$FirstSignature,
        [string]$SecondSignature,
        [string]$HandtestRoot,
        [string]$EvidenceDir
    )

    if ([string]::IsNullOrWhiteSpace($HandtestRoot)) {
        Fail "preflight" "-HandtestRoot is required with -SameStackPrep (the pristine snapshot root)."
    }
    if ([string]::IsNullOrWhiteSpace($ProjectPath) -or -not (Test-Path -LiteralPath $ProjectPath)) {
        Fail "preflight" ("same-stack prep: the active project path is missing or does not exist: " + $ProjectPath)
    }
    $rootFull = [System.IO.Path]::GetFullPath($HandtestRoot)
    $liveDir = Split-Path -Parent $ProjectPath
    $projectFileName = Split-Path -Leaf $ProjectPath
    $projectBase = [System.IO.Path]::GetFileNameWithoutExtension($ProjectPath)
    $pristineDir = Join-Path $rootFull "pristine"
    if (Test-Path -LiteralPath $pristineDir) {
        Fail "preflight" ("same-stack prep: HandtestRoot already contains pristine (" + $rootFull + "); choose a new root or remove it")
    }

    $prep = [ordered]@{
        schema_version = "vit_w02_a1_handtest_prep.v1"
        card = "W02-A1-HANDTEST-V0"
        mode = "same_stack"
        blocked_by_proj_open_resume = $false
        no_project_open = $true
        handtest_root = $rootFull
        live_dir = $liveDir
        pristine_dir = $pristineDir
        live_project = $ProjectPath
        pristine_project = ""
        prep_conversation_id = $ConversationId
        expected_node = $ClipNode
        target_slot = $TargetSlot
        track_names = [ordered]@{}
        rename = [ordered]@{}
        save = [ordered]@{}
        copies = [ordered]@{}
        live_state = [ordered]@{}
        conversation = [ordered]@{}
        restore_probe = [ordered]@{}
        zero_llm = [ordered]@{}
        verified = $false
    }
    if ($null -ne $Summary) { $Summary.handtest_prep = $prep }

    # SS1: human-readable names for the two user tracks (idempotent on re-runs).
    Write-Step "PREP(same-stack) rename target/reference tracks"
    $targetName = "我的旋律（选第二个 clip）"
    $referenceName = "参考（空白）"
    $renameTarget = Invoke-Tool -Name "prep_rename_target" -Tool "rename_track" -ToolArgs @{ track_id = $TargetTrackId; name = $targetName }
    $renameTargetResult = Get-Result $renameTarget
    $targetNameAfter = [string](Get-Prop $renameTargetResult "track_name")
    Assert-True ($targetNameAfter -eq $targetName) ("rename_track(target) track_name=" + $targetNameAfter + " expected=" + $targetName + " result=" + (Format-Result $renameTargetResult))
    $renameReference = Invoke-Tool -Name "prep_rename_reference" -Tool "rename_track" -ToolArgs @{ track_id = $ReferenceTrackId; name = $referenceName }
    $renameReferenceResult = Get-Result $renameReference
    $referenceNameAfter = [string](Get-Prop $renameReferenceResult "track_name")
    Assert-True ($referenceNameAfter -eq $referenceName) ("rename_track(reference) track_name=" + $referenceNameAfter + " expected=" + $referenceName + " result=" + (Format-Result $renameReferenceResult))
    $prep.rename = [ordered]@{
        target = [ordered]@{ track_id = $TargetTrackId; name = $targetName; old_name = [string](Get-Prop $renameTargetResult "old_name"); status = [string](Get-Prop $renameTargetResult "status") }
        reference = [ordered]@{ track_id = $ReferenceTrackId; name = $referenceName; old_name = [string](Get-Prop $renameReferenceResult "old_name"); status = [string](Get-Prop $renameReferenceResult "status") }
    }
    $prep.track_names = [ordered]@{ target = $targetName; reference = $referenceName }
    Write-Ok ("PREP(same-stack) rename: " + $TargetTrackId + " -> " + $targetName + " / " + $ReferenceTrackId + " -> " + $referenceName)

    # SS2: persist the kernel state (tracks/clips/notes) into the open .vit.
    Write-Step "PREP(same-stack) save project in place"
    $saveBefore = Get-FileSha256 $ProjectPath
    $saveBeforeSize = Get-FileLength $ProjectPath
    $saveResponse = Invoke-Tool -Name "prep_project_save" -Tool "project.save" -ToolArgs @{}
    $saveResult = Get-Result $saveResponse
    $saveKernelStatus = [string](Get-Prop $saveResult "status")
    if (-not [string]::IsNullOrWhiteSpace($saveKernelStatus) -and $saveKernelStatus -ne "ok") {
        Fail "assert" ("project.save kernel status=" + $saveKernelStatus + " result=" + (Format-Result $saveResult))
    }
    Start-Sleep -Milliseconds 800
    Assert-True (Test-Path -LiteralPath $ProjectPath) ("project file missing after save: " + $ProjectPath)
    $saveAfter = Get-FileSha256 $ProjectPath
    $saveAfterSize = Get-FileLength $ProjectPath
    Assert-True (-not [string]::IsNullOrWhiteSpace($saveAfter)) ("could not hash the saved project file: " + $ProjectPath)
    Assert-True ($saveAfter -ne $saveBefore) ("project.save did not rewrite the .vit (sha256 unchanged " + $saveAfter + "); tracks/clips would not persist")
    $prep.save = [ordered]@{
        command = "project.save"
        status = $saveKernelStatus
        kernel_message = [string](Get-Prop $saveResult "message")
        project_path = [string](Get-Prop $saveResult "project_path")
        project_lifecycle = [string](Get-Prop $saveResult "project_lifecycle")
        size_before = $saveBeforeSize
        size_after = $saveAfterSize
        sha256_before = $saveBefore
        sha256_after = $saveAfter
        rewritten = ($saveAfter -ne $saveBefore)
    }
    Write-Ok ("PREP(same-stack) save: " + [string]$saveAfterSize + " bytes sha256=" + $saveAfter.Substring(0, 12) + "..")

    # SS3: verify the LIVE project (no reopen): the same stack must still have
    # the same active project and the two renamed tracks / two clips / notes.
    Write-Step "PREP(same-stack) verify live project.state (no project.open)"
    $liveProjectState = $null
    $stateAttempts = 0
    while ($stateAttempts -lt 12) {
        $stateAttempts++
        $liveProjectState = Invoke-Tool -Name ("prep_live_project_state_" + $stateAttempts) -Tool "project.state" -ToolArgs @{} -Confirmed $false
        $pollTracks = @(Get-UserTracks $liveProjectState)
        $pollTarget = $null
        foreach ($pollRow in $pollTracks) {
            if ([string](Get-Prop $pollRow "track_name") -eq $targetName) { $pollTarget = $pollRow }
        }
        $pollClips = @()
        if ($null -ne $pollTarget) { $pollClips = @(Get-TrackClips $pollTarget) }
        if (($pollTracks.Count -ge 2) -and ($pollClips.Count -eq 2)) { break }
        Start-Sleep -Milliseconds 800
    }
    $liveEvidence = Get-HandtestLiveEvidence -ProjectState $liveProjectState -TargetName $targetName -ReferenceName $referenceName -FirstSignature $FirstSignature -SecondSignature $SecondSignature -FirstClipId $FirstClipId -SecondClipId $SecondClipId -TargetTrackId $TargetTrackId -ReferenceTrackId $ReferenceTrackId -EvidenceDir $EvidenceDir -EvidenceName "prep_same_stack_live_state.json"
    $liveEvidence.state_poll_attempts = $stateAttempts
    $prep.live_state = $liveEvidence

    # The no-open invariant: the active project must be the very same file/uuid.
    $afterState = Invoke-Api -Name "prep_same_stack_state_after" -Method "Get" -Path "/agent/state" -Body $null
    $afterCurrent = Get-Prop $afterState "current_project"
    $afterPath = [string](Get-Prop $afterCurrent "path")
    if ([string]::IsNullOrWhiteSpace($afterPath)) { $afterPath = [string](Get-Prop (Get-Prop $afterState "project_history") "project_path") }
    $afterUuid = [string](Get-Prop $afterCurrent "uuid")
    if ([string]::IsNullOrWhiteSpace($afterUuid)) { $afterUuid = [string](Get-Prop (Get-Prop $afterState "project_history") "project_uuid") }
    Assert-True ($afterPath -eq $ProjectPath) ("same-stack prep must not switch projects; active=" + $afterPath + " expected=" + $ProjectPath)
    if (-not [string]::IsNullOrWhiteSpace($afterUuid) -and -not [string]::IsNullOrWhiteSpace($ProjectUuid)) {
        Assert-True ($afterUuid -eq $ProjectUuid) ("same-stack prep must not switch projects; uuid=" + $afterUuid + " expected=" + $ProjectUuid)
    }
    $prep.live_state.active_project_unchanged = $true
    $prep.live_state.active_project_uuid = $afterUuid
    Write-Ok ("PREP(same-stack) live: tracks=" + [string]$liveEvidence.user_track_count + " target clips=" + [string]$liveEvidence.target_clip_count + " second=" + [string]$liveEvidence.second_clip_id)

    # SS4: the prep conversation must be at the clip node in this very stack.
    Write-Step "PREP(same-stack) verify prep conversation node (live session)"
    $statusResponse = Invoke-AgentChat -Name "prep_status_same_stack" -ConversationID $ConversationId -Message "乐局状态" -Context @{}
    $statusStop = [string](Get-Prop $statusResponse "stop_reason")
    Assert-True ($statusStop -eq "edu_jam_status") ("prep status stop_reason=" + $statusStop + " reply=" + [string](Get-Prop $statusResponse "reply"))
    $statusWorkflow = Get-WorkflowData $statusResponse
    $nodeAfter = [string](Get-Prop $statusWorkflow "current_node")
    $levelAfter = [string](Get-Prop $statusWorkflow "level_id")
    Assert-True ($levelAfter -eq $CardId) ("prep conversation level=" + $levelAfter + " expected=" + $CardId)
    Assert-True ($nodeAfter -eq $ClipNode) ("prep conversation node=" + $nodeAfter + " expected=" + $ClipNode + " (reply=" + [string](Get-Prop $statusResponse "reply") + ")")
    $prep.conversation = [ordered]@{
        conversation_id = $ConversationId
        command = "乐局状态"
        stop_reason = $statusStop
        level_id = $levelAfter
        current_node = $nodeAfter
        expected_node = $ClipNode
        live_session = $true
        reply = [string](Get-Prop $statusResponse "reply")
    }
    Write-Ok ("PREP(same-stack) conversation: " + $statusStop + " level=" + $levelAfter + " node=" + $nodeAfter)

    # SS5: pristine snapshot of the prepared project dir. The project stays OPEN
    # in this stack, so the copy is retried on transient sharing locks; it is a
    # reference snapshot (same-stack reset = re-run prep, not a file restore).
    Write-Step "PREP(same-stack) snapshot the project dir to <HandtestRoot>/pristine"
    New-Item -ItemType Directory -Path $rootFull -Force | Out-Null
    $script:handtestCopyDirs = @($pristineDir)
    $packageDir = Find-ProjectPackage -ProjectPath $ProjectPath
    Assert-True (-not [string]::IsNullOrWhiteSpace($packageDir)) ("project package not found next to the prep project: " + $ProjectPath)
    $agentDir = Join-Path $liveDir ".vit_agent"
    $historyDir = Join-Path $liveDir ".vit_history"
    Assert-True (Test-Path -LiteralPath $agentDir) ("project agent dir missing: " + $agentDir)
    Assert-True (Test-Path -LiteralPath $historyDir) ("project history dir missing: " + $historyDir)
    $copyItems = @()
    $copyItems += [ordered]@{ source = $ProjectPath; name = $projectFileName; kind = "file" }
    $copyItems += [ordered]@{ source = $packageDir; name = (Split-Path -Leaf $packageDir); kind = "dir" }
    $copyItems += [ordered]@{ source = $agentDir; name = ".vit_agent"; kind = "dir" }
    $copyItems += [ordered]@{ source = $historyDir; name = ".vit_history"; kind = "dir" }
    $mediaSource = Join-Path $liveDir ($projectBase + "_Media")
    $mediaCopied = $false
    if (Test-Path -LiteralPath $mediaSource) {
        $copyItems += [ordered]@{ source = $mediaSource; name = ($projectBase + "_Media"); kind = "dir" }
        $mediaCopied = $true
    }
    $liveShaAtCopy = Get-FileSha256 $ProjectPath
    $copyAttempts = 0
    $pristineSha = ""
    $pristineProject = Join-Path $pristineDir $projectFileName
    $pristineDigest = $null
    $lastCopyError = ""
    while ($copyAttempts -lt 3) {
        $copyAttempts++
        try {
            if (Test-Path -LiteralPath $pristineDir) { Remove-Item -LiteralPath $pristineDir -Recurse -Force }
            New-Item -ItemType Directory -Path $pristineDir -Force | Out-Null
            foreach ($item in $copyItems) {
                $dest = Join-Path $pristineDir $item.name
                if ($item.kind -eq "dir") { Copy-Item -LiteralPath $item.source -Destination $dest -Recurse -Force }
                else { Copy-Item -LiteralPath $item.source -Destination $dest -Force }
            }
            $pristineSha = Get-FileSha256 $pristineProject
            if ($pristineSha -eq $liveShaAtCopy) { break }
            $lastCopyError = "pristine .vit sha256=" + $pristineSha + " expected=" + $liveShaAtCopy
        }
        catch {
            $lastCopyError = $_.Exception.Message
        }
        Start-Sleep -Milliseconds 800
    }
    Assert-True (Test-Path -LiteralPath $pristineProject) ("pristine snapshot missing: " + $pristineProject + " last_error=" + $lastCopyError)
    Assert-True ($pristineSha -eq $liveShaAtCopy) ("pristine .vit sha256 mismatch after " + [string]$copyAttempts + " attempt(s): " + $pristineSha + " expected=" + $liveShaAtCopy + " last_error=" + $lastCopyError)
    $pristineDigest = Get-TreeDigest -Path $pristineDir
    $prep.pristine_project = $pristineProject
    $prep.copies = [ordered]@{
        mode = "same_stack_snapshot"
        items = @($copyItems | ForEach-Object { [ordered]@{ source = $_.source; name = $_.name; kind = $_.kind } })
        media_dir_copied = $mediaCopied
        attempts = $copyAttempts
        live_project_sha256 = $liveShaAtCopy
        pristine = [ordered]@{ files = $pristineDigest.files; bytes = $pristineDigest.bytes; sha256 = $pristineDigest.sha256; project_sha256 = $pristineSha }
    }
    Write-Ok ("PREP(same-stack) snapshot: " + $pristineDir + " files=" + [string]$pristineDigest.files + " sha256=" + $pristineSha.Substring(0, 12) + "..")

    # SS6: supporting disk probe (what the next cold start would find - and drop).
    $stateProbe = Find-EduJamStateProbe -ProjectDir $liveDir -ConversationId $ConversationId -NodeId $ClipNode
    $prep.restore_probe = $stateProbe
    if ([string](Get-Prop $stateProbe "result") -ne "node_active") {
        Write-WarnLine ("same-stack restore probe: " + (Format-Result $stateProbe) + " (supporting only; the live session check above is the hand-test basis)")
    }

    # SS7: zero-LLM assertion on the per-call telemetry rows for this conversation.
    $llmRows = 0
    $otherLlmRows = 0
    $telemetryText = Read-AppendedText -Path $telemetryPath -Offset $telemetryOffset -MaxBytes 2097152
    $prep.zero_llm = [ordered]@{ telemetry_available = ($null -ne $telemetryText); telemetry_path = $telemetryPath; llm_rows = 0; other_conversation_rows = 0; note = "" }
    if ($null -eq $telemetryText) {
        $prep.zero_llm.note = "telemetry file missing; the zero-LLM claim rests on the deterministic stop_reasons recorded in responses/"
        Write-WarnLine "same-stack prep zero-LLM check: telemetry unavailable; recorded as evidence gap"
    }
    else {
        foreach ($telemetryLine in [regex]::Split($telemetryText, '\r?\n')) {
            $trimmedLine = $telemetryLine.Trim()
            if ($trimmedLine -eq "") { continue }
            try { $telemetryRecord = $trimmedLine | ConvertFrom-Json } catch { continue }
            if ([string](Get-Prop $telemetryRecord "conversation_id") -eq $ConversationId) { $llmRows++ }
            else { $otherLlmRows++ }
        }
        $prep.zero_llm.llm_rows = $llmRows
        $prep.zero_llm.other_conversation_rows = $otherLlmRows
        if ($otherLlmRows -gt 0) {
            Write-WarnLine ("same-stack prep zero-LLM check: " + [string]$otherLlmRows + " LLM telemetry row(s) for other conversations appeared during this run; recorded")
        }
        Assert-True ($llmRows -eq 0) ("same-stack prep consumed " + [string]$llmRows + " LLM call(s) for conversation " + $ConversationId + "; prep must be zero-LLM")
    }
    Write-Ok ("PREP(same-stack) zero-LLM: " + [string]$llmRows + " LLM rows for the prep conversation")

    # SS8: fixture manifest for the hand test / reset instructions.
    try {
        $fixtureManifest = [ordered]@{
            schema_version = "vit_w02_a1_handtest_fixture.v1"
            card = "W02-A1-HANDTEST-V0"
            mode = "same_stack"
            created_at = (Get-Date).ToString("o")
            handtest_root = $rootFull
            live_dir = $liveDir
            pristine_dir = $pristineDir
            project_file = $projectFileName
            live_project = $ProjectPath
            prep_conversation_id = $ConversationId
            expected_node = $ClipNode
            target_track_id = $liveEvidence.target_track_id
            reference_track_id = $liveEvidence.reference_track_id
            target_track_name = $targetName
            reference_track_name = $referenceName
            first_clip_id = $liveEvidence.first_clip_id
            second_clip_id = $liveEvidence.second_clip_id
            project_sha256 = $liveShaAtCopy
            zero_llm = ($llmRows -eq 0)
            handtest_rule = "do NOT close/reopen the project; select the prepared conversation, select the second clip, say 写好了 in the same window"
        }
        $fixtureManifest | ConvertTo-Json -Depth 12 | Set-Content -LiteralPath (Join-Path $rootFull "handtest.json") -Encoding UTF8
        $prep.fixture_manifest = (Join-Path $rootFull "handtest.json")
    }
    catch {
        $prep.fixture_manifest_error = $_.Exception.Message
        Write-WarnLine ("same-stack fixture manifest could not be written: " + $_.Exception.Message)
    }

    $prep.verified = $true
    return $prep
}

function Invoke-HandtestPrep {
    # Prep leg for W02-A1-HANDTEST-V0. Runs after S2 and returns the
    # handtest_prep evidence block. Zero LLM: only deterministic commands
    # (rename_track / project.save / project.open / project.state / 乐局状态).
    param(
        [object]$Summary,
        [string]$ConversationId,
        [string]$ProjectPath,
        [string]$CardId,
        [string]$TargetTrackId,
        [string]$ReferenceTrackId,
        [string]$ClipNode,
        [string]$TargetSlot,
        [string]$FirstClipId,
        [string]$SecondClipId,
        [string]$FirstSignature,
        [string]$SecondSignature,
        [string]$HandtestRoot,
        [string]$EvidenceDir
    )

    if ([string]::IsNullOrWhiteSpace($HandtestRoot)) {
        Fail "preflight" "-HandtestRoot is required with -HandtestPrep (live/pristine fixture root)."
    }
    $rootFull = [System.IO.Path]::GetFullPath($HandtestRoot)
    $liveDir = Join-Path $rootFull "live"
    $pristineDir = Join-Path $rootFull "pristine"
    if ((Test-Path -LiteralPath $liveDir) -or (Test-Path -LiteralPath $pristineDir)) {
        Fail "preflight" ("HandtestRoot already contains live/pristine (" + $rootFull + "); choose a new root or reset it with reset_handtest_live.ps1")
    }

    $prep = [ordered]@{
        schema_version = "vit_w02_a1_handtest_prep.v1"
        card = "W02-A1-HANDTEST-V0"
        mode = "reopen"
        # Lead ruling 2026-11-19: project.open archives+blanks the agent
        # conversation (PROJ-OPEN-RESUME-1), so this mode cannot deliver the n2
        # hand test. Kept only for clip-persistence evidence; do not hand-test it.
        blocked_by_proj_open_resume = $true
        handtest_root = $rootFull
        live_dir = $liveDir
        pristine_dir = $pristineDir
        live_project = ""
        pristine_project = ""
        prep_conversation_id = $ConversationId
        expected_node = $ClipNode
        target_slot = $TargetSlot
        track_names = [ordered]@{}
        rename = [ordered]@{}
        save = [ordered]@{}
        copies = [ordered]@{}
        reopen = [ordered]@{}
        conversation = [ordered]@{}
        restore_probe = [ordered]@{}
        zero_llm = [ordered]@{}
        verified = $false
    }
    # Published before the first assertion so a failing prep round still leaves
    # the partial evidence (save/copy/reopen) in PREP-<stamp>/summary.json.
    if ($null -ne $Summary) { $Summary.handtest_prep = $prep }

    # P1: the two user tracks get human-readable names so the hand test can pick
    # the right clip slot by reading the DAW.
    Write-Step "PREP rename target/reference tracks"
    $targetName = "我的旋律（选第二个 clip）"
    $referenceName = "参考（空白）"
    $renameTarget = Invoke-Tool -Name "prep_rename_target" -Tool "rename_track" -ToolArgs @{ track_id = $TargetTrackId; name = $targetName }
    $renameTargetResult = Get-Result $renameTarget
    $targetNameAfter = [string](Get-Prop $renameTargetResult "track_name")
    Assert-True ($targetNameAfter -eq $targetName) ("rename_track(target) track_name=" + $targetNameAfter + " expected=" + $targetName + " result=" + (Format-Result $renameTargetResult))
    $renameReference = Invoke-Tool -Name "prep_rename_reference" -Tool "rename_track" -ToolArgs @{ track_id = $ReferenceTrackId; name = $referenceName }
    $renameReferenceResult = Get-Result $renameReference
    $referenceNameAfter = [string](Get-Prop $renameReferenceResult "track_name")
    Assert-True ($referenceNameAfter -eq $referenceName) ("rename_track(reference) track_name=" + $referenceNameAfter + " expected=" + $referenceName + " result=" + (Format-Result $renameReferenceResult))
    $prep.rename = [ordered]@{
        target = [ordered]@{ track_id = $TargetTrackId; name = $targetName; old_name = [string](Get-Prop $renameTargetResult "old_name"); status = [string](Get-Prop $renameTargetResult "status") }
        reference = [ordered]@{ track_id = $ReferenceTrackId; name = $referenceName; old_name = [string](Get-Prop $renameReferenceResult "old_name"); status = [string](Get-Prop $renameReferenceResult "status") }
    }
    $prep.track_names = [ordered]@{ target = $targetName; reference = $referenceName }
    Write-Ok ("PREP rename: " + $TargetTrackId + " -> " + $targetName + " / " + $ReferenceTrackId + " -> " + $referenceName)

    # P2: persist the kernel edit state (tracks/clips/notes) into the .vit.
    # project.save writes the app-bound .vit in place (catalog save_project ->
    # project.save -> ipcSaveEncryptedProjectToPath with createNewProjectIdentity
    # false); no save_as-same-path workaround was needed.
    Write-Step "PREP save project (persist tracks/clips before copy)"
    $saveBefore = Get-FileSha256 $ProjectPath
    $saveBeforeSize = Get-FileLength $ProjectPath
    $saveResponse = Invoke-Tool -Name "prep_project_save" -Tool "project.save" -ToolArgs @{}
    $saveResult = Get-Result $saveResponse
    $saveKernelStatus = [string](Get-Prop $saveResult "status")
    if (-not [string]::IsNullOrWhiteSpace($saveKernelStatus) -and $saveKernelStatus -ne "ok") {
        Fail "assert" ("project.save kernel status=" + $saveKernelStatus + " result=" + (Format-Result $saveResult))
    }
    Start-Sleep -Milliseconds 1200
    Assert-True (Test-Path -LiteralPath $ProjectPath) ("project file missing after save: " + $ProjectPath)
    $saveAfter = Get-FileSha256 $ProjectPath
    $saveAfterSize = Get-FileLength $ProjectPath
    Assert-True (-not [string]::IsNullOrWhiteSpace($saveAfter)) ("could not hash the saved project file: " + $ProjectPath)
    Assert-True ($saveAfter -ne $saveBefore) ("project.save did not rewrite the .vit (sha256 unchanged " + $saveAfter + "); tracks/clips would not persist")
    $saveState = Invoke-Api -Name "prep_state_after_save" -Method "Get" -Path "/agent/state" -Body $null
    $saveHistory = Get-Prop $saveState "project_history"
    $saveUnsaved = (Get-Bool (Get-Prop $saveHistory "unsaved")) -or (Get-Bool (Get-Prop (Get-Prop $saveState "shadow") "unsaved"))
    $prep.save = [ordered]@{
        command = "project.save"
        status = $saveKernelStatus
        kernel_message = [string](Get-Prop $saveResult "message")
        project_path = [string](Get-Prop $saveResult "project_path")
        project_lifecycle = [string](Get-Prop $saveResult "project_lifecycle")
        size_before = $saveBeforeSize
        size_after = $saveAfterSize
        sha256_before = $saveBefore
        sha256_after = $saveAfter
        rewritten = ($saveAfter -ne $saveBefore)
        unsaved_after_save = $saveUnsaved
        save_as_same_path_fallback = $false
    }
    if ($saveUnsaved) {
        Write-WarnLine "prep save: /agent/state still reports unsaved=true after project.save; recorded, reopen evidence stays authoritative"
    }
    Write-Ok ("PREP save: " + [string]$saveAfterSize + " bytes sha256=" + $saveAfter.Substring(0, 12) + ".. unsaved=" + [string]$saveUnsaved)

    # P3: copy the project directory (card list: .vit/.vit_project/.vit_agent/
    # .vit_history) to HandtestRoot/live, then live -> pristine BEFORE any
    # reopen, so pristine is the clean save-point baseline.
    Write-Step "PREP copy project directory to HandtestRoot live + pristine"
    New-Item -ItemType Directory -Path $rootFull -Force | Out-Null
    New-Item -ItemType Directory -Path $liveDir -Force | Out-Null
    $script:handtestCopyDirs = @($liveDir, $pristineDir)
    $projectFileName = Split-Path -Leaf $ProjectPath
    $projectDir = Split-Path -Parent $ProjectPath
    $projectBase = [System.IO.Path]::GetFileNameWithoutExtension($ProjectPath)
    $packageDir = Find-ProjectPackage -ProjectPath $ProjectPath
    Assert-True (-not [string]::IsNullOrWhiteSpace($packageDir)) ("project package not found next to the prep project: " + $ProjectPath)
    $agentDir = Join-Path $projectDir ".vit_agent"
    $historyDir = Join-Path $projectDir ".vit_history"
    Assert-True (Test-Path -LiteralPath $agentDir) ("project agent dir missing: " + $agentDir)
    Assert-True (Test-Path -LiteralPath $historyDir) ("project history dir missing: " + $historyDir + " (the prep conversation lives there)")
    $copyItems = @()
    $copyItems += [ordered]@{ source = $ProjectPath; name = $projectFileName; kind = "file" }
    $copyItems += [ordered]@{ source = $packageDir; name = (Split-Path -Leaf $packageDir); kind = "dir" }
    $copyItems += [ordered]@{ source = $agentDir; name = ".vit_agent"; kind = "dir" }
    $copyItems += [ordered]@{ source = $historyDir; name = ".vit_history"; kind = "dir" }
    $mediaSource = Join-Path $projectDir ($projectBase + "_Media")
    $mediaCopied = $false
    if (Test-Path -LiteralPath $mediaSource) {
        $copyItems += [ordered]@{ source = $mediaSource; name = ($projectBase + "_Media"); kind = "dir" }
        $mediaCopied = $true
    }
    foreach ($item in $copyItems) {
        $dest = Join-Path $liveDir $item.name
        if ($item.kind -eq "dir") { Copy-Item -LiteralPath $item.source -Destination $dest -Recurse -Force }
        else { Copy-Item -LiteralPath $item.source -Destination $dest -Force }
    }
    $liveProject = Join-Path $liveDir $projectFileName
    Assert-True (Test-Path -LiteralPath $liveProject) ("live project copy missing: " + $liveProject)
    $liveSha = Get-FileSha256 $liveProject
    Assert-True ($liveSha -eq $saveAfter) ("live .vit sha256 mismatch: " + $liveSha + " expected=" + $saveAfter)

    # The agent can still flush workspace/history files right after a save, so
    # the live->pristine copy is compared and re-taken once instead of failing on
    # a transient background write.
    $liveDigest = Get-TreeDigest -Path $liveDir
    $pristineDigest = $null
    $pristineSha = ""
    $copyAttempts = 0
    while ($copyAttempts -lt 2) {
        $copyAttempts++
        if (Test-Path -LiteralPath $pristineDir) { Remove-Item -LiteralPath $pristineDir -Recurse -Force }
        New-Item -ItemType Directory -Path $pristineDir -Force | Out-Null
        foreach ($entry in @(Get-ChildItem -LiteralPath $liveDir -Force)) {
            Copy-Item -LiteralPath $entry.FullName -Destination (Join-Path $pristineDir $entry.Name) -Recurse -Force
        }
        $liveDigest = Get-TreeDigest -Path $liveDir
        $pristineDigest = Get-TreeDigest -Path $pristineDir
        if (($liveDigest.files -eq $pristineDigest.files) -and ($liveDigest.sha256 -eq $pristineDigest.sha256)) { break }
        Start-Sleep -Milliseconds 500
    }
    $pristineProject = Join-Path $pristineDir $projectFileName
    Assert-True (Test-Path -LiteralPath $pristineProject) ("pristine project copy missing: " + $pristineProject)
    $pristineSha = Get-FileSha256 $pristineProject
    Assert-True ($pristineSha -eq $liveSha) ("pristine .vit sha256 mismatch: " + $pristineSha + " expected=" + $liveSha)
    Assert-True ($liveDigest.files -eq $pristineDigest.files) ("live/pristine file count mismatch: " + [string]$liveDigest.files + " / " + [string]$pristineDigest.files)
    Assert-True ($liveDigest.sha256 -eq $pristineDigest.sha256) ("live/pristine tree digest mismatch: " + $liveDigest.sha256 + " / " + $pristineDigest.sha256)
    $prep.live_project = $liveProject
    $prep.pristine_project = $pristineProject
    $prep.copies = [ordered]@{
        items = @($copyItems | ForEach-Object { [ordered]@{ source = $_.source; name = $_.name; kind = $_.kind } })
        media_dir_copied = $mediaCopied
        live = [ordered]@{ files = $liveDigest.files; bytes = $liveDigest.bytes; sha256 = $liveDigest.sha256; project_sha256 = $liveSha }
        pristine = [ordered]@{ files = $pristineDigest.files; bytes = $pristineDigest.bytes; sha256 = $pristineDigest.sha256; project_sha256 = $pristineSha }
        same_digest = [ordered]@{ value = ($liveDigest.sha256 -eq $pristineDigest.sha256); live = $liveDigest.sha256; pristine = $pristineDigest.sha256 }
    }
    Write-Ok ("PREP copy: live=" + $liveDir + " pristine=" + $pristineDir + " files=" + [string]$liveDigest.files + " digest=" + $liveDigest.sha256.Substring(0, 12) + "..")

    # P4: reopen the live copy (same .vit content, delivered path) and assert the
    # kernel still reports both renamed tracks and both clips with their notes.
    Write-Step "PREP reopen live copy and verify persisted tracks/clips/notes"
    $openLive = Invoke-Tool -Name "prep_open_live" -Tool "project.open" -ToolArgs @{ file_path = $liveProject }
    $openLiveResult = Get-Result $openLive
    $liveState = Wait-ActiveProject -Path $liveProject -Seconds ([Math]::Max(30, [Math]::Min($TimeoutSeconds, 120)))
    $liveCurrent = Get-Prop $liveState "current_project"
    $liveUuid = [string](Get-Prop $liveCurrent "uuid")
    if ([string]::IsNullOrWhiteSpace($liveUuid)) { $liveUuid = [string](Get-Prop (Get-Prop $liveState "project_history") "project_uuid") }
    if ([string]::IsNullOrWhiteSpace($liveUuid)) { $liveUuid = [string](Get-Prop (Get-Prop $liveState "shadow") "project_uuid") }
    Assert-True (-not [string]::IsNullOrWhiteSpace($liveUuid)) ("live reopen project uuid is empty: " + $liveProject)

    # The kernel can report the reopened project as active before the full track
    # graph settles, so poll project.state until the target row shows its two
    # clips (bounded; the last observation is the assertion basis).
    $liveProjectState = $null
    $stateAttempts = 0
    while ($stateAttempts -lt 12) {
        $stateAttempts++
        $liveProjectState = Invoke-Tool -Name ("prep_live_project_state_" + $stateAttempts) -Tool "project.state" -ToolArgs @{} -Confirmed $false
        $pollTracks = @(Get-UserTracks $liveProjectState)
        $pollTarget = $null
        foreach ($pollRow in $pollTracks) {
            if ([string](Get-Prop $pollRow "track_name") -eq $targetName) { $pollTarget = $pollRow }
        }
        $pollClips = @()
        if ($null -ne $pollTarget) { $pollClips = @(Get-TrackClips $pollTarget) }
        if (($pollTracks.Count -ge 2) -and ($pollClips.Count -eq 2)) { break }
        Start-Sleep -Milliseconds 800
    }
    try {
        Set-Content -LiteralPath (Join-Path $EvidenceDir "prep_live_project_state.json") -Value (($liveProjectState | ConvertTo-Json -Depth 40)) -Encoding UTF8
    }
    catch { }

    $liveTracks = @(Get-UserTracks $liveProjectState)
    $liveTrackNames = @()
    foreach ($track in $liveTracks) { $liveTrackNames += [string](Get-Prop $track "track_name") }
    Assert-True ($liveTracks.Count -ge 2) ("live reopen user tracks=" + [string]$liveTracks.Count + " names=[" + ($liveTrackNames -join ",") + "]; expected the renamed target + reference tracks")
    $targetRow = $null
    $referenceRow = $null
    foreach ($track in $liveTracks) {
        $rowName = [string](Get-Prop $track "track_name")
        if ($rowName -eq $targetName) { $targetRow = $track }
        elseif ($rowName -eq $referenceName) { $referenceRow = $track }
    }
    Assert-True ($null -ne $targetRow) ("target track name not persisted after reopen; names=[" + ($liveTrackNames -join ",") + "]")
    Assert-True ($null -ne $referenceRow) ("reference track name not persisted after reopen; names=[" + ($liveTrackNames -join ",") + "]")
    $targetClips = @(Get-TrackClips $targetRow)
    Assert-True ($targetClips.Count -eq 2) ("target track clips after reopen=" + [string]$targetClips.Count + " expected=2 (the .vit saved tracks but lost clips?)")
    $referenceClips = @(Get-TrackClips $referenceRow)
    Assert-True ($referenceClips.Count -eq 0) ("reference track should stay empty; clips after reopen=" + [string]$referenceClips.Count)
    $targetTrackIdAfter = Get-TrackId $targetRow
    $referenceTrackIdAfter = Get-TrackId $referenceRow

    $reopenClips = @()
    $firstAfter = ""
    $secondAfter = ""
    foreach ($clip in $targetClips) {
        $clipId = Get-ClipId $clip
        Assert-True (-not [string]::IsNullOrWhiteSpace($clipId)) ("target clip row has no clip id: " + (Format-Result $clip))
        $clipNotes = @(Read-ClipNotes -Name ("prep_read_clip_" + $clipId) -ClipId $clipId)
        $clipSignature = Get-NoteSignature $clipNotes
        $origin = ""
        if ($clipId -eq $FirstClipId) { $origin = "first" }
        elseif ($clipId -eq $SecondClipId) { $origin = "second" }
        $reopenClips += [ordered]@{
            clip_id = $clipId
            start = Get-Prop $clip "start"
            length = Get-Prop $clip "length"
            notes = $clipSignature
            notes_count = $clipNotes.Count
            original_clip_id = $origin
        }
        if ($clipSignature -eq $FirstSignature -and [string]::IsNullOrWhiteSpace($firstAfter)) { $firstAfter = $clipId }
        elseif ($clipSignature -eq $SecondSignature -and [string]::IsNullOrWhiteSpace($secondAfter)) { $secondAfter = $clipId }
    }
    $reopenSignatures = @()
    foreach ($row in $reopenClips) { $reopenSignatures += [string](Get-Prop $row "notes") }
    Assert-True (-not [string]::IsNullOrWhiteSpace($firstAfter)) ("first clip notes not found after reopen; expected=" + $FirstSignature + " actual=[" + ($reopenSignatures -join " | ") + "]")
    Assert-True (-not [string]::IsNullOrWhiteSpace($secondAfter)) ("second clip notes not found after reopen; expected=" + $SecondSignature + " actual=[" + ($reopenSignatures -join " | ") + "]")
    Assert-True ($firstAfter -ne $secondAfter) "reopened control/target clips resolved to the same clip id"
    $prep.reopen = [ordered]@{
        command = "project.open"
        project_path = $liveProject
        project_uuid = $liveUuid
        project_uuid_reply = [string](Get-Prop $openLiveResult "project_uuid")
        track_count = Get-Prop (Get-Result $liveProjectState) "track_count"
        user_track_count = $liveTracks.Count
        track_names = $liveTrackNames
        target_track_id = $targetTrackIdAfter
        reference_track_id = $referenceTrackIdAfter
        target_track_id_stable = ($targetTrackIdAfter -eq $TargetTrackId)
        reference_track_id_stable = ($referenceTrackIdAfter -eq $ReferenceTrackId)
        target_clip_count = $targetClips.Count
        reference_clip_count = $referenceClips.Count
        state_poll_attempts = $stateAttempts
        pristine_copy_attempts = $copyAttempts
        clips = $reopenClips
        first_clip_id = $firstAfter
        second_clip_id = $secondAfter
        first_signature = $FirstSignature
        second_signature = $SecondSignature
        first_clip_id_stable = ($firstAfter -eq $FirstClipId)
        second_clip_id_stable = ($secondAfter -eq $SecondClipId)
    }
    Write-Ok ("PREP reopen: tracks=" + [string]$liveTracks.Count + " target clips=" + [string]$targetClips.Count + " second=" + $secondAfter)

    # P5: the prep conversation must still be at the clip node. 乐局状态 is a
    # deterministic command: with no session it returns edu_jam_no_session (no
    # LLM fallback), with a session it returns edu_jam_status + current_node.
    Write-Step "PREP verify prep conversation node after reopen"
    $statusResponse = Invoke-AgentChat -Name "prep_status_after_open" -ConversationID $ConversationId -Message "乐局状态" -Context @{}
    $statusStop = [string](Get-Prop $statusResponse "stop_reason")
    if ($statusStop -eq "edu_jam_no_session") {
        $prep.conversation = [ordered]@{
            conversation_id = $ConversationId
            command = "乐局状态"
            stop_reason = $statusStop
            expected_node = $ClipNode
            restore_blocked_by = "PROJ-OPEN-RESUME-1"
            evidence = "agent/internal/history/working_session.go:216-245 (archiveReopenedAgentState at the project-open boundary) + working_session.go:441 (writeBlankAgentRuntimeState drops conversations/edu_jam_sessions) + chat/project_workspace_persistence_test.go:147 TestSameProjectReopenStartsBlankAndArchivesSavedConversation (go test PASS 2026-11-19)"
            reply = [string](Get-Prop $statusResponse "reply")
        }
        Fail "assert" ("prep conversation " + $ConversationId + " has NO lesson session after the reopen (stop_reason=edu_jam_no_session). The tracks/clips persistence part passed, but project.open intentionally archives and blanks the agent conversation/runtime state (PROJ-OPEN-RESUME-1, user ruling 2026-09-13): opening a saved project never restores its lesson session, so the hand-test premise 'prep conversation at n2 after opening the live project' is blocked by that product behavior. Reply=" + [string](Get-Prop $statusResponse "reply"))
    }
    Assert-True ($statusStop -eq "edu_jam_status") ("prep status stop_reason=" + $statusStop + " reply=" + [string](Get-Prop $statusResponse "reply"))
    $statusWorkflow = Get-WorkflowData $statusResponse
    $nodeAfter = [string](Get-Prop $statusWorkflow "current_node")
    $levelAfter = [string](Get-Prop $statusWorkflow "level_id")
    Assert-True ($levelAfter -eq $CardId) ("prep conversation level after reopen=" + $levelAfter + " expected=" + $CardId)
    Assert-True ($nodeAfter -eq $ClipNode) ("prep conversation node after reopen=" + $nodeAfter + " expected=" + $ClipNode + " (reply=" + [string](Get-Prop $statusResponse "reply") + ")")
    $prep.conversation = [ordered]@{
        conversation_id = $ConversationId
        command = "乐局状态"
        stop_reason = $statusStop
        level_id = $levelAfter
        current_node = $nodeAfter
        expected_node = $ClipNode
        reply = [string](Get-Prop $statusResponse "reply")
    }
    Write-Ok ("PREP conversation: " + $statusStop + " level=" + $levelAfter + " node=" + $nodeAfter)

    # P6: supporting cold-start probe over the copied agent state.
    $stateProbe = Find-EduJamStateProbe -ProjectDir $liveDir -ConversationId $ConversationId -NodeId $ClipNode
    $prep.restore_probe = $stateProbe
    if ([string](Get-Prop $stateProbe "result") -eq "conversation_without_active_node") {
        Fail "assert" ("persisted agent state for the live copy carries the prep conversation but node " + $ClipNode + " is not active (" + [string](Get-Prop $stateProbe "file") + "); the fixture would restore at the wrong node")
    }
    if ([string](Get-Prop $stateProbe "result") -ne "node_active") {
        Write-WarnLine ("prep restore probe inconclusive: " + (Format-Result $stateProbe) + " (cold start is proven by the hand test; in-process reopen is weaker evidence)")
    }

    # P7: zero-LLM assertion on the per-call telemetry rows for this conversation.
    $llmRows = 0
    $otherLlmRows = 0
    $telemetryText = Read-AppendedText -Path $telemetryPath -Offset $telemetryOffset -MaxBytes 2097152
    $prep.zero_llm = [ordered]@{ telemetry_available = ($null -ne $telemetryText); telemetry_path = $telemetryPath; llm_rows = 0; other_conversation_rows = 0; note = "" }
    if ($null -eq $telemetryText) {
        $prep.zero_llm.note = "telemetry file missing; the zero-LLM claim rests on the deterministic stop_reasons recorded in responses/"
        Write-WarnLine "prep zero-LLM check: telemetry file unavailable; recorded as evidence gap"
    }
    else {
        foreach ($telemetryLine in [regex]::Split($telemetryText, '\r?\n')) {
            $trimmedLine = $telemetryLine.Trim()
            if ($trimmedLine -eq "") { continue }
            try { $telemetryRecord = $trimmedLine | ConvertFrom-Json } catch { continue }
            if ([string](Get-Prop $telemetryRecord "conversation_id") -eq $ConversationId) { $llmRows++ }
            else { $otherLlmRows++ }
        }
        $prep.zero_llm.llm_rows = $llmRows
        $prep.zero_llm.other_conversation_rows = $otherLlmRows
        if ($otherLlmRows -gt 0) {
            Write-WarnLine ("prep zero-LLM check: " + [string]$otherLlmRows + " LLM telemetry row(s) for other conversations appeared during this run (concurrent stack use?); recorded")
        }
        Assert-True ($llmRows -eq 0) ("handtest prep consumed " + [string]$llmRows + " LLM call(s) for conversation " + $ConversationId + "; prep must be zero-LLM")
    }
    Write-Ok ("PREP zero-LLM: " + [string]$llmRows + " LLM rows for the prep conversation")

    # P8: fixture manifest next to live/pristine (reset + hand test read this).
    try {
        $fixtureManifest = [ordered]@{
            schema_version = "vit_w02_a1_handtest_fixture.v1"
            card = "W02-A1-HANDTEST-V0"
            created_at = (Get-Date).ToString("o")
            handtest_root = $rootFull
            live_dir = $liveDir
            pristine_dir = $pristineDir
            project_file = $projectFileName
            live_project = $liveProject
            prep_conversation_id = $ConversationId
            expected_node = $ClipNode
            target_track_id = $targetTrackIdAfter
            reference_track_id = $referenceTrackIdAfter
            target_track_name = $targetName
            reference_track_name = $referenceName
            first_clip_id = $firstAfter
            second_clip_id = $secondAfter
            project_sha256 = $liveSha
            zero_llm = ($llmRows -eq 0)
        }
        $fixtureManifest | ConvertTo-Json -Depth 12 | Set-Content -LiteralPath (Join-Path $rootFull "handtest.json") -Encoding UTF8
        $prep.fixture_manifest = (Join-Path $rootFull "handtest.json")
    }
    catch {
        $prep.fixture_manifest_error = $_.Exception.Message
        Write-WarnLine ("prep fixture manifest could not be written: " + $_.Exception.Message)
    }

    $prep.verified = $true
    return $prep
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
    Write-Host ("  Question mode : " + $QuestionMode + " (status = A1c S0-S6; work = S0-S3 + S4W)") -ForegroundColor Yellow
    Write-Host "  Same-stack prep (-SameStackPrep -HandtestRoot <dir> [-ProjectPath <open.vit>], W02-A1-HANDTEST-V0):" -ForegroundColor Yellow
    Write-Host "     requires the project ALREADY OPEN in the running stack; never project.new/project.open." -ForegroundColor Yellow
    Write-Host "     import W01 -> 开始乐局 -> S1.5 to the clip node -> two clips -> rename -> project.save," -ForegroundColor Yellow
    Write-Host "     verify the LIVE project.state (2 tracks / 2 clips / notes), snapshot <dir>/pristine." -ForegroundColor Yellow
    Write-Host "     S3-S6 are NOT run and NO LLM is consumed; the prep conversation stays at n2 for the" -ForegroundColor Yellow
    Write-Host "     hand test IN THE SAME WINDOW (do not close/reopen; reset = re-run prep)." -ForegroundColor Yellow
    Write-Host "  Reopen prep (-HandtestPrep): BLOCKED for hand tests by PROJ-OPEN-RESUME-1 (project.open" -ForegroundColor Yellow
    Write-Host "     archives+blanks the agent conversation). Kept only for clip-persistence evidence." -ForegroundColor Yellow
    Write-Host "  Work mode (A2a): S4W one real-LLM work question about the selected clip's" -ForegroundColor Yellow
    Write-Host "     actual notes; requires a notes-read receipt (agent_events / prompt-render" -ForegroundColor Yellow
    Write-Host "     tool_result / executed_kernel_reply). False note claims fail exit 5;" -ForegroundColor Yellow
    Write-Host "     an honest 'need more info' reply = model_boundary (exit 5, lead decides)." -ForegroundColor Yellow
    Write-Host ""
    Write-Host "Not authorized. The script does not start/restart/reuse a stack; it only" -ForegroundColor Yellow
    Write-Host "uses an already-running stack. Ask the lead to register the berth in" -ForegroundColor Yellow
    Write-Host "coord/resources/PC-RUNTIME-STACK.md and run with -StackAuthorized:" -ForegroundColor Yellow
    Write-Host "  powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\run_w02_a1c_smoke.ps1 -RepoRoot D:/Vit_DAW -AgentBaseUrl http://127.0.0.1:7878 -StackAuthorized -RuntimeOwner '<owner>' -TimeoutSeconds 180 [-QuestionMode work]" -ForegroundColor Yellow
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
    Write-Host ("  QuestionMode   : " + $QuestionMode + " (status|work; default status = A1c semantics)")
    Write-Host ("  HandtestPrep   : " + [string]$HandtestPrep + " (reopen mode; BLOCKED by PROJ-OPEN-RESUME-1)")
    Write-Host ("  SameStackPrep  : " + [string]$SameStackPrep)
    if ($HandtestPrep -or $SameStackPrep) {
        Write-Host ("  HandtestRoot   : " + $(if ([string]::IsNullOrWhiteSpace($HandtestRoot)) { "<required with -HandtestPrep/-SameStackPrep>" } else { $HandtestRoot }))
        Write-Host ("  ProjectPath    : " + $(if ([string]::IsNullOrWhiteSpace($ProjectPath)) { "</agent/state current_project.path>" } else { $ProjectPath }))
    }
    Write-Host "  Plan:"
    if ($SameStackPrep) {
        Write-Host "    S0 preflight + snapshots + log offsets; S1 requires the project ALREADY OPEN"
        Write-Host "    S1 no project.new/save_as: import W01 design -> 开始乐局 -> S1.5 advance to the clip node"
        Write-Host "    S2 track.add if needed; clear target/reference tracks (re-run convergence); two MIDI clips; select_clip(second)"
        Write-Host "    PREP rename_track target (我的旋律（选第二个 clip）) + reference (参考（空白）)"
        Write-Host "    PREP project.save in place (persist tracks/clips; fail if the file is not rewritten)"
        Write-Host "    PREP verify the LIVE project.state: two tracks / target 2 clips / exact notes; assert no project switch"
        Write-Host "    PREP 乐局状态 in the prep conversation: edu_jam_status + current_node == the clip node (n2-rhythm)"
        Write-Host "    PREP snapshot the project dir -> <HandtestRoot>/pristine (reference; reset = re-run prep)"
        Write-Host "    PREP record coord/runs/W02-A1-handtest/PREP-<stamp>/summary.json + PREP-NOTES.md"
        Write-Host "    S3/S4/S5/S6 are skipped; zero LLM; NEVER project.open (PROJ-OPEN-RESUME-1)"
        Write-Host "  Exit codes: 0 PASS, 1 assertion, 2 environment/precondition (project not open), 3 unauthorized, 5 evidence/API gap"
        Write-Host "  Not executed. No stack process was started and no port was opened." -ForegroundColor Green
        return
    }
    if ($HandtestPrep) {
        Write-Host "    S0 preflight + snapshots (recents/Settings.xml/default_project.xml) + log offsets"
        Write-Host "    S1 project.new -> project.save_as A -> import W01 design -> 开始乐局"
        Write-Host "    S1.5 record card event nodes until the first clip_exists/track_non_empty node"
        Write-Host "    S2 create required user tracks by runtime_slots track_index; create two MIDI clips; select_clip(second)"
        Write-Host "    PREP rename_track target (我的旋律（选第二个 clip）) + reference (参考（空白）)"
        Write-Host "    PREP project.save (persist tracks/clips into the .vit; fail if the file is not rewritten)"
        Write-Host "    PREP copy .vit/.vit_project/.vit_agent/.vit_history -> <HandtestRoot>/live; live -> pristine (digest check)"
        Write-Host "    PREP project.open <live>/<name>.vit: project.state two renamed tracks, target 2 clips, notes readback"
        Write-Host "    PREP 乐局状态 in the prep conversation: edu_jam_status + current_node == the clip node (n2-rhythm)"
        Write-Host "    PREP record coord/runs/W02-A1-handtest/PREP-<stamp>/summary.json + PREP-NOTES.md"
        Write-Host "    S3/S4/S5/S6 are skipped; zero LLM; the prep conversation stays active for the hand test"
        Write-Host "  Exit codes: 0 PASS, 1 assertion (lost clip/track/node), 2 environment/teardown, 3 unauthorized, 5 evidence/API gap"
        Write-Host "  Not executed. No stack process was started and no port was opened." -ForegroundColor Green
        return
    }
    Write-Host "    S0 preflight + snapshots (recents/Settings.xml/default_project.xml) + log offsets"
    Write-Host "    S1 project.new -> project.save_as A -> import W01 design -> 开始乐局"
    Write-Host "    S1.5 record card event nodes until the first clip_exists/track_non_empty node"
    Write-Host "    S2 create required user tracks by runtime_slots track_index; create two MIDI clips; select_clip(second)"
    Write-Host "    S3 写好了 with selected_clip_id=second; assert clip_scope=selected, clip_ids[target]=second, first not used"
    if ($QuestionMode -eq "work") {
        Write-Host "    S4W one real-LLM work question about the selected clip's actual notes;"
        Write-Host "        assert stop_reason=done, reply non-empty, lesson_context card/node/clip;"
        Write-Host "        require a notes-read receipt (agent_events / prompt-render tool_result /"
        Write-Host "        executed_kernel_reply); kernel log recorded as support; false note claim"
        Write-Host "        = exit 5; honest 'need more info' = model_boundary (exit 5, lead decides)"
        Write-Host "    S5/S6 skipped in work mode (A1c status-mode coverage only)"
    }
    else {
        Write-Host "    S4 one real-LLM natural question; assert stop_reason=done, reply non-empty, lesson_context in new agent_prompt_render.jsonl row"
        Write-Host "    S5 no-selection scope=first control; cross-slot selection scope=selection_unmatched (never selected pass)"
        Write-Host "    S6 结束乐局 -> project.open A -> natural sentence -> auto-intro/recovery; teardown restores files/authority"
    }
    Write-Host "  Exit codes: 0 PASS, 1 assertion, 2 environment/teardown, 3 unauthorized, 4 LLM class, 5 evidence/API gap (work: false claim/model_boundary)"
    Write-Host "  Not executed. No stack process was started and no port was opened." -ForegroundColor Green
}

# ---------------------------------------------------------------------------
# Mode validation (applies to the dry-run and the authorized path).
# ---------------------------------------------------------------------------
if ($HandtestPrep -and $QuestionMode -eq "work") {
    Write-Host "error: -HandtestPrep and -QuestionMode work are mutually exclusive (prep never consumes an LLM turn)." -ForegroundColor Red
    exit 2
}
if ($SameStackPrep -and $QuestionMode -eq "work") {
    Write-Host "error: -SameStackPrep and -QuestionMode work are mutually exclusive (prep never consumes an LLM turn)." -ForegroundColor Red
    exit 2
}
if ($SameStackPrep -and $HandtestPrep) {
    Write-Host "error: -SameStackPrep (hand-test mode) and -HandtestPrep (reopen mode, blocked by PROJ-OPEN-RESUME-1) are mutually exclusive." -ForegroundColor Red
    exit 2
}
if ((-not $SameStackPrep) -and (-not [string]::IsNullOrWhiteSpace($ProjectPath))) {
    Write-Host "error: -ProjectPath is only valid with -SameStackPrep (the project must already be open)." -ForegroundColor Red
    exit 2
}
$prepMode = ""
if ($HandtestPrep) { $prepMode = "reopen" }
if ($SameStackPrep) { $prepMode = "same_stack" }

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
if ($HandtestPrep -or $SameStackPrep) {
    if ([string]::IsNullOrWhiteSpace($HandtestRoot)) {
        Write-Host ("error: -HandtestRoot is required with " + $(if ($SameStackPrep) { "-SameStackPrep" } else { "-HandtestPrep" }) + " (the pristine snapshot root).") -ForegroundColor Red
        exit 2
    }
    $HandtestRoot = [System.IO.Path]::GetFullPath($HandtestRoot)
    $blocked = @()
    if (Test-Path -LiteralPath (Join-Path $HandtestRoot "pristine")) { $blocked += "pristine" }
    if ($HandtestPrep -and (Test-Path -LiteralPath (Join-Path $HandtestRoot "live"))) { $blocked += "live" }
    if ($blocked.Count -gt 0) {
        Write-Host ("error: HandtestRoot already contains " + ($blocked -join "/") + " (" + $HandtestRoot + "); choose a new root (or reset an existing fixture with reset_handtest_live.ps1).") -ForegroundColor Red
        exit 2
    }
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
if ($HandtestPrep -or $SameStackPrep) {
    $conversationId = "w02-a1ht-" + $stamp
}
if ($HandtestPrep) {
    $projectA = Join-Path $RunRoot ("W02-A1-handtest-" + $stamp + ".vit")
}
if ($SameStackPrep) {
    # Filled from /agent/state in S1; the project must already be open.
    $projectA = ""
}
$recentsPath = Get-RecentsPath
$settingsPath = Join-Path $RepoRoot "VitApp/Workspace/Settings/Settings.xml"
$defaultProjectPath = Join-Path $RepoRoot "VitApp/Workspace/default_project.xml"
$contextSnapshotPath = Get-AgentLogPath -EnvName "VIT_CONTEXT_SNAPSHOT_PATH" -FileName "agent_context_snapshots.jsonl"
$promptRenderPath = Get-AgentLogPath -EnvName "VIT_AGENT_PROMPT_RENDER_PATH" -FileName "agent_prompt_render.jsonl"
$telemetryPath = Get-AgentLogPath -EnvName "VIT_AGENT_LLM_TELEMETRY_PATH" -FileName "agent_llm_telemetry.jsonl"

$summary = [ordered]@{
    schema_version = $(if ($HandtestPrep -or $SameStackPrep) { "vit_w02_a1_handtest_prep.v1" } elseif ($QuestionMode -eq "work") { "vit_w02_a2a_smoke.v1" } else { "vit_w02_a1c_smoke.v1" })
    card = $(if ($HandtestPrep -or $SameStackPrep) { "W02-A1-HANDTEST-V0" } elseif ($QuestionMode -eq "work") { "W02-A2A" } else { "W02-A1C" })
    question_mode = $QuestionMode
    prep_mode = $prepMode
    handtest_prep = [ordered]@{}
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
    s4w = [ordered]@{}
    s5a = [ordered]@{}
    s5b = [ordered]@{}
    s6 = [ordered]@{}
    teardown = [ordered]@{}
    prep_record_dir = ""
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
$handtestRootFull = ""
$prepRecordDir = ""
if ($HandtestPrep -or $SameStackPrep) {
    $handtestRootFull = $HandtestRoot
    $script:handtestCopyDirs = @()
    $scriptRoot = Split-Path -Parent $PSCommandPath
    $checkoutRoot = Split-Path -Parent $scriptRoot
    if (-not [string]::IsNullOrWhiteSpace($checkoutRoot)) {
        $prepRecordDir = Join-Path $checkoutRoot ("coord/runs/W02-A1-handtest/PREP-" + $stamp)
    }
    $summary.prep_record_dir = $prepRecordDir
}
$preflight = $summary.preflight
$s1 = $summary.s1
$s2 = $summary.s2
$s3 = $summary.s3
$s4 = $summary.s4
$s4w = $summary.s4w
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

    if ($SameStackPrep) {
        Write-Step "S1 same-stack project + W01 import + enter (no project.new / no project.open)"
        $stackState = Invoke-Api -Name "s1_same_stack_state" -Method "Get" -Path "/agent/state" -Body $null
        $stackCurrent = Get-Prop $stackState "current_project"
        $stackPath = [string](Get-Prop $stackCurrent "path")
        if ([string]::IsNullOrWhiteSpace($stackPath)) { $stackPath = [string](Get-Prop (Get-Prop $stackState "project_history") "project_path") }
        if ([string]::IsNullOrWhiteSpace($stackPath)) { $stackPath = [string](Get-Prop (Get-Prop $stackState "shadow") "project_path") }
        if ([string]::IsNullOrWhiteSpace($stackPath)) {
            Fail "preflight" "same-stack prep requires an ALREADY OPEN saved project; /agent/state reports no active project path (open the .vit in VitShell first — prep never opens/reopens a project)"
        }
        Assert-True (Test-Path -LiteralPath $stackPath) ("same-stack prep: active project file does not exist: " + $stackPath)
        if (-not [string]::IsNullOrWhiteSpace($ProjectPath)) {
            $wantedPath = [System.IO.Path]::GetFullPath($ProjectPath)
            $activePath = [System.IO.Path]::GetFullPath($stackPath)
            if (-not [string]::Equals($wantedPath, $activePath, [System.StringComparison]::OrdinalIgnoreCase)) {
                Fail "preflight" ("same-stack prep: -ProjectPath (" + $wantedPath + ") is not the active project (" + $activePath + "); open it first (prep never opens/reopens)")
            }
        }
        $stackUnsaved = (Get-Bool (Get-Prop (Get-Prop $stackState "project_history") "unsaved")) -or (Get-Bool (Get-Prop (Get-Prop $stackState "shadow") "unsaved"))
        if ($stackUnsaved) {
            Fail "preflight" ("same-stack prep: the active project has unsaved changes (" + $stackPath + "); save or discard them first (prep will save the project as the fixture)")
        }
        $projectA = [System.IO.Path]::GetFullPath($stackPath)
        $stackUuid = [string](Get-Prop $stackCurrent "uuid")
        if ([string]::IsNullOrWhiteSpace($stackUuid)) { $stackUuid = [string](Get-Prop (Get-Prop $stackState "project_history") "project_uuid") }
        if ([string]::IsNullOrWhiteSpace($stackUuid)) { $stackUuid = [string](Get-Prop (Get-Prop $stackState "shadow") "project_uuid") }
        Assert-True (-not [string]::IsNullOrWhiteSpace($stackUuid)) "same-stack prep: active project uuid is empty"
        $projectUuid = $stackUuid
        $summary.project_uuid = $projectUuid
        $summary.project_a = $projectA
        $s1.project = [ordered]@{
            mode = "same_stack"
            project_path = $projectA
            project_uuid = $projectUuid
            opened_by_user = $true
            prep_opened_project = $false
        }
        Write-Ok ("same-stack project: " + $projectA + " uuid=" + $projectUuid + " (opened by the user; prep never opens/reopens)")
    }
    else {
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
    }

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

    $removedClipIds = @()
    if ($SameStackPrep) {
        # Re-runs converge: clear the target/reference tracks first. A previous
        # prep run or a hand-test round may have left clips there; creating two
        # more would break the "target track has exactly 2 clips" fixture.
        Write-Step "S2 same-stack cleanup (remove clips left by an earlier prep/hand test)"
        $cleanupTarget = $null
        $cleanupReference = $null
        $cleanupState = Invoke-Tool -Name "s2_cleanup_state" -Tool "project.state" -ToolArgs @{} -Confirmed $false
        foreach ($cleanupTrack in @(Get-UserTracks $cleanupState)) {
            $cleanupTrackId = Get-TrackId $cleanupTrack
            if ($cleanupTrackId -eq $targetTrackId) { $cleanupTarget = $cleanupTrack }
            elseif ($cleanupTrackId -eq $firstTrackId) { $cleanupReference = $cleanupTrack }
        }
        Assert-True ($null -ne $cleanupTarget) ("same-stack cleanup: target track " + $targetTrackId + " is missing from project.state")
        $cleanupClipIds = @()
        foreach ($cleanupTrack in @($cleanupTarget, $cleanupReference)) {
            if ($null -eq $cleanupTrack) { continue }
            foreach ($cleanupClip in @(Get-TrackClips $cleanupTrack)) {
                $cleanupClipId = Get-ClipId $cleanupClip
                if (-not [string]::IsNullOrWhiteSpace($cleanupClipId)) { $cleanupClipIds += $cleanupClipId }
            }
        }
        $cleanupClipIds = @($cleanupClipIds | Select-Object -Unique)
        $cleanupLeft = 0
        if ($cleanupClipIds.Count -gt 0) {
            $null = Invoke-Tool -Name "s2_cleanup_remove" -Tool "remove_clips" -ToolArgs @{ clip_ids = $cleanupClipIds }
            $removedClipIds = $cleanupClipIds
            $cleanupDeadline = (Get-Date).AddSeconds(30)
            $cleanupLeft = -1
            $cleanupPoll = 0
            while ((Get-Date) -lt $cleanupDeadline) {
                Start-Sleep -Milliseconds 500
                $cleanupPoll++
                $verifyState = Invoke-Tool -Name ("s2_cleanup_verify_" + $cleanupPoll) -Tool "project.state" -ToolArgs @{} -Confirmed $false
                $cleanupLeft = 0
                foreach ($verifyTrack in @(Get-UserTracks $verifyState)) {
                    $verifyTrackId = Get-TrackId $verifyTrack
                    if (($verifyTrackId -eq $targetTrackId) -or ($verifyTrackId -eq $firstTrackId)) {
                        $cleanupLeft += @(Get-TrackClips $verifyTrack).Count
                    }
                }
                if ($cleanupLeft -eq 0) { break }
            }
            Assert-True ($cleanupLeft -eq 0) ("same-stack cleanup: " + [string]$cleanupLeft + " clip(s) remain on the target/reference tracks after remove_clips; re-run prep on a clean project or restore the fixture")
        }
        $s2.cleanup = [ordered]@{ removed_clip_ids = $removedClipIds; remaining = $cleanupLeft }
        Write-Ok ("same-stack cleanup: removed " + [string]$removedClipIds.Count + " pre-existing clip(s)")
    }

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

    if ($SameStackPrep) {
        Write-Step "PREP same-stack fixture (W02-A1-HANDTEST-V0; zero LLM; session stays at the clip node)"
        $null = Invoke-SameStackPrep -Summary $summary -ConversationId $conversationId -ProjectPath $projectA -ProjectUuid $projectUuid -CardId $cardId -TargetTrackId $targetTrackId -ReferenceTrackId $firstTrackId -ClipNode $currentNode -TargetSlot $targetSlot -FirstClipId $firstClipId -SecondClipId $secondClipId -FirstSignature $firstSignature -SecondSignature $secondSignature -HandtestRoot $HandtestRoot -EvidenceDir $evidenceDir
        $summary.status = "passed"
        Write-Ok ("same-stack prep PASSED: live=" + $projectA + " node=" + $currentNode + " conversation=" + $conversationId)
    }
    elseif ($HandtestPrep) {
        Write-Step "PREP handtest fixture (W02-A1-HANDTEST-V0; reopen mode is BLOCKED by PROJ-OPEN-RESUME-1; zero LLM)"
        Write-WarnLine "reopen prep mode is blocked for the hand test (PROJ-OPEN-RESUME-1: project.open drops the agent conversation). Kept for clip-persistence evidence only; use -SameStackPrep for the hand test."
        $null = Invoke-HandtestPrep -Summary $summary -ConversationId $conversationId -ProjectPath $projectA -CardId $cardId -TargetTrackId $targetTrackId -ReferenceTrackId $firstTrackId -ClipNode $currentNode -TargetSlot $targetSlot -FirstClipId $firstClipId -SecondClipId $secondClipId -FirstSignature $firstSignature -SecondSignature $secondSignature -HandtestRoot $HandtestRoot -EvidenceDir $evidenceDir
        $summary.status = "passed"
        Write-Ok ("handtest prep PASSED: live=" + [string]$summary.handtest_prep.live_project + " node=" + $currentNode + " conversation=" + $conversationId)
    }
    else {

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

    if ($QuestionMode -eq "work") {
    # W02-A2a work mode: S4W (selected-clip work feedback) replaces S4;
    # S5/S6 remain A1c status-mode coverage and are not run here.
        Write-Step "S4W one real-LLM work-feedback question (selected clip actual notes)"
        # Work mode reuses S0-S3 (import / enter / two clips / selected second) and
        # asks ONE real-LLM natural question about the selected clip's actual notes.
        # Fail-honest by construction: empty reply, non-done stop_reason, missing
        # lesson_context, a note-content claim without a notes-read receipt, and an
        # unreadable evidence face all end in a non-zero exit. Only a notes-read
        # receipt plus done+reply+lesson_context yields status=passed.
        $contextOffset = Get-FileLength $contextSnapshotPath
        $promptOffset = Get-FileLength $promptRenderPath
        $telemetryOffset = Get-FileLength $telemetryPath
        $workKernelLogPath = ""
        $workKernelOffset = [long]0
        if ($null -ne $kernelLogInfo) {
            $workKernelLogPath = [string](Get-Prop $kernelLogInfo "path")
            $workKernelOffset = Get-FileLength $workKernelLogPath
        }
        # Watermark the in-memory agent event timeline so only this turn's tool
        # items are inspected. /agent/events is GET ?conversation_id=&since=&limit=.
        $eventsSince = [long]0
        try {
            $eventsBeforePath = "/agent/events?conversation_id=" + [uri]::EscapeDataString($conversationId) + "&since=0&limit=1"
            $eventsBefore = Invoke-Api -Name "s4w_events_before" -Method "Get" -Path $eventsBeforePath -Body $null
            $eventsSince = [long](Get-PropOr $eventsBefore "next_seq" 0)
        }
        catch {
            $s4w.events_watermark_error = $_.Exception.Message
            Write-WarnLine ("S4W agent events pre-read unavailable: " + $_.Exception.Message)
        }
        $workQuestion = "我选中了这段旋律：它的具体音高和时值走向是什么？请基于实际音符给我一个修改建议。"
        $s4w.question = $workQuestion
        $s4w.selection_clip_id = $secondClipId
        $s4w.expected_notes = $secondSignature
        $s4w.events_watermark = $eventsSince
        $s4w.kernel_log_path = $workKernelLogPath
        $s4w.kernel_log_offset = $workKernelOffset

        $workProbe = Invoke-LlmChat -Name "s4w_question_probe" -ConversationID $conversationId -Message $workQuestion -Context $selectionContext
        $workProbeStop = [string](Get-Prop $workProbe "stop_reason")
        $workProbeKind = ""
        $workResponse = $null
        if ($workProbeStop -eq "edu_jam_recovery_broadcast") {
            # Deterministic zero-LLM broadcast: the second send is the single formal
            # LLM user turn (same discipline as A1c S4).
            $workProbeKind = "recovery_broadcast"
            $workResponse = Invoke-LlmChat -Name "s4w_question" -ConversationID $conversationId -Message $workQuestion -Context $selectionContext
        }
        elseif ($workProbeStop -eq "done") {
            $workProbeKind = "direct_llm"
            $workResponse = $workProbe
        }
        else {
            Fail "llm" ("S4W probe stop_reason=" + $workProbeStop + " (expected recovery_broadcast or done); reply=" + [string](Get-Prop $workProbe "reply"))
        }
        $workStop = [string](Get-Prop $workResponse "stop_reason")
        if ($workStop -ne "done") {
            Fail "llm" ("S4W stop_reason=" + $workStop + " goal_status=" + [string](Get-Prop $workResponse "goal_status") + " error=" + [string](Get-Prop $workResponse "error") + " reply=" + [string](Get-Prop $workResponse "reply"))
        }
        $workReply = [string](Get-Prop $workResponse "reply")
        if ([string]::IsNullOrWhiteSpace($workReply)) { Fail "llm" "S4W reply is empty" }

        $workExecuted = Get-Prop $workResponse "executed_kernel_reply"
        $workExecutedRows = @()
        if ($null -ne $workExecuted) { $workExecutedRows = @($workExecuted) }
        $executedNames = @()
        foreach ($executedRow in $workExecutedRows) {
            $executedName = [string](Get-Prop $executedRow "command_name")
            if ([string]::IsNullOrWhiteSpace($executedName)) { $executedName = [string](Get-Prop $executedRow "tool") }
            if ([string]::IsNullOrWhiteSpace($executedName)) { $executedName = [string](Get-Prop $executedRow "command") }
            if (-not [string]::IsNullOrWhiteSpace($executedName)) { $executedNames += $executedName }
        }
        $s4w.probe = $workProbeKind
        $s4w.stop_reason = $workStop
        $s4w.goal_status = [string](Get-Prop $workResponse "goal_status")
        $s4w.completed_steps = Get-Prop $workResponse "completed_steps"
        $s4w.current_step = [string](Get-Prop $workResponse "current_step")
        $s4w.turn_id = [string](Get-Prop $workResponse "turn_id")
        $s4w.run_id = [string](Get-Prop $workResponse "run_id")
        $s4w.reply = $workReply
        $s4w.executed_kernel_reply_count = $workExecutedRows.Count
        $s4w.executed_kernel_reply_names = $executedNames
        $s4w.prompt_render_path = $promptRenderPath
        $s4w.prompt_offset = $promptOffset

        # Tolerant content recording only: never a PASS gate (the card asks for
        # keyword recording, not hard-coded wording).
        $workKeywordHits = @()
        foreach ($keyword in @("音高", "时值", "走向", "旋律", "节奏", "音符", "修改建议", "建议", "上行", "下行", "级进", "跳进", "长音", "短音")) {
            if ($workReply.Contains($keyword)) { $workKeywordHits += $keyword }
        }
        $noteFeatureHits = @()
        foreach ($keyword in @("67", "71", "74", "G4", "B4", "D5", "二分音符", "八分音符", "2拍", "半拍", "1.5拍")) {
            if ($workReply.Contains($keyword)) { $noteFeatureHits += $keyword }
        }
        $claimHits = @()
        foreach ($keyword in @("我听到", "我听了", "听过", "我看了", "我查看", "看过", "我读到", "我看到", "可以看到", "能看到", "从音符", "音高是", "音高为", "时值是", "时值为", "这段旋律是", "该片段是", "音符是")) {
            if ($workReply.Contains($keyword)) { $claimHits += $keyword }
        }
        $honestHits = @()
        foreach ($keyword in @("需要更多信息", "无法看到", "看不到", "没有读取", "无法读取", "无法确认", "不知道具体", "请提供", "需要先", "没有足够", "无法判断", "没有看到", "没看到", "没有看过", "没看过", "没有听过", "没听过", "无法访问", "没有权限", "无法直接", "需要读取", "我还不知道")) {
            if ($workReply.Contains($keyword)) { $honestHits += $keyword }
        }
        $s4w.reply_keyword_hits = $workKeywordHits
        $s4w.reply_note_feature_hits = $noteFeatureHits
        $s4w.claim_marker_hits = $claimHits
        $s4w.honest_marker_hits = $honestHits
        if ($workKeywordHits.Count -eq 0) {
            Write-WarnLine "S4W reply had no tolerant work keyword hit; recorded for lead review, not a hard fail"
        }

        # lesson_context must still reach the model in work mode (card/node/selected
        # clip). Work mode does not use the legacy context-snapshot fallback: the
        # prompt-render row is the required model-input evidence.
        Start-Sleep -Milliseconds 500
        $workPromptMatch = Find-PromptRenderLesson -Path $promptRenderPath -Offset $promptOffset -ConversationId $conversationId
        if ($null -eq $workPromptMatch) { Fail "evidence" ("prompt render file not found/readable: " + $promptRenderPath) }
        $workPromptHasLesson = ($null -ne $workPromptMatch.lesson)
        $s4w.prompt_render_has_lesson_context = $workPromptHasLesson
        $s4w.prompt_render_conversation_lines = $workPromptMatch.conversation_lines
        if (-not [string]::IsNullOrWhiteSpace($workPromptMatch.line)) {
            Set-Content -LiteralPath (Join-Path $evidenceDir "s4w_prompt_render.jsonl") -Value $workPromptMatch.line -Encoding UTF8
        }
        if ($null -ne $workPromptMatch.appended -and $workPromptMatch.appended.Length -gt 0) {
            Set-Content -LiteralPath (Join-Path $evidenceDir "s4w_prompt_render_appended.jsonl") -Value $workPromptMatch.appended -Encoding UTF8
        }
        if (-not $workPromptHasLesson) {
            Fail "evidence" ("S4W model-input lesson_context missing for conversation " + $conversationId + ": prompt_render_lines=" + [string]$workPromptMatch.conversation_lines + ". lesson_context is required in work mode too.")
        }
        $workLesson = $workPromptMatch.lesson
        $workLessonCard = Get-Prop $workLesson "card"
        $workLessonNode = Get-Prop $workLesson "node"
        $workLessonClip = Get-Prop $workLesson "selected_clip"
        $workLessonSchema = [string](Get-Prop $workLesson "schema_version")
        $workLessonCardId = [string](Get-Prop $workLessonCard "id")
        $workLessonNodeId = [string](Get-Prop $workLessonNode "id")
        $workLessonClipId = [string](Get-Prop $workLessonClip "clip_id")
        $workLessonClipTrack = [string](Get-Prop $workLessonClip "track_id")
        Assert-True ($workLessonSchema -eq "vit.edu_jam.lesson_context.v1") ("S4W lesson_context schema=" + $workLessonSchema)
        Assert-True ($workLessonCardId -eq $cardId) ("S4W lesson_context card.id=" + $workLessonCardId + " expected=" + $cardId)
        Assert-True ($workLessonNodeId -eq $currentNode) ("S4W lesson_context node.id=" + $workLessonNodeId + " expected current=" + $currentNode)
        Assert-True ($workLessonClipId -eq $secondClipId) ("S4W lesson_context selected_clip.clip_id=" + $workLessonClipId + " expected=" + $secondClipId)
        if (-not [string]::IsNullOrWhiteSpace($workLessonClipTrack)) {
            Assert-True ($workLessonClipTrack -eq $targetTrackId) ("S4W lesson_context selected_clip.track_id=" + $workLessonClipTrack + " expected=" + $targetTrackId)
        }
        Set-Content -LiteralPath (Join-Path $evidenceDir "s4w_lesson_context.json") -Value ($workLesson | ConvertTo-Json -Depth 20) -Encoding UTF8
        $s4w.lesson_context = [ordered]@{
            schema_version = $workLessonSchema
            card_id = $workLessonCardId
            node_id = $workLessonNodeId
            selected_clip_id = $workLessonClipId
            selected_clip_track_id = $workLessonClipTrack
        }

        # Evidence face 1: /agent/events tool items (the agent timeline). Tool
        # events carry payload.tool (LLM name, e.g. midi.read_notes),
        # payload.command_name (kernel name, e.g. get_midi_clip_notes) and args.
        Start-Sleep -Milliseconds 500
        $eventsAfter = $null
        try {
            $eventsAfterPath = "/agent/events?conversation_id=" + [uri]::EscapeDataString($conversationId) + "&since=" + [string]$eventsSince + "&limit=500"
            $eventsAfter = Invoke-Api -Name "s4w_agent_events" -Method "Get" -Path $eventsAfterPath -Body $null
        }
        catch {
            $s4w.agent_events_error = $_.Exception.Message
            Write-WarnLine ("S4W agent events timeline unavailable: " + $_.Exception.Message)
        }
        $eventRows = @()
        if ($null -ne $eventsAfter) {
            $eventRows = @(Get-Prop $eventsAfter "events")
            try {
                Set-Content -LiteralPath (Join-Path $evidenceDir "s4w_agent_events.json") -Value (ConvertTo-Json -InputObject $eventsAfter -Depth 12) -Encoding UTF8
            }
            catch { }
        }
        $toolRows = @(Get-ToolCallRowsFromEvents $eventsAfter)
        $s4w.agent_events_count = $eventRows.Count
        $s4w.tool_call_count = $toolRows.Count
        $s4w.tool_calls = @()
        foreach ($toolRow in $toolRows) {
            $s4w.tool_calls += [ordered]@{
                seq = $toolRow.seq
                type = $toolRow.type
                status = $toolRow.status
                tool = $toolRow.tool
                command_name = $toolRow.command_name
                command = $toolRow.command
                clip_id = $toolRow.clip_id
                notes_count = $toolRow.notes_count
                error = $toolRow.error
            }
        }
        $notesEventRows = @()
        foreach ($toolRow in $toolRows) {
            if ($toolRow.type -ne "item.completed") { continue }
            if ($toolRow.status -ne "completed") { continue }
            if (-not [string]::IsNullOrWhiteSpace($toolRow.error)) { continue }
            if ($toolRow.clip_id -ne $secondClipId) { continue }
            if (-not ((Test-NotesReadToolName $toolRow.tool) -or (Test-NotesReadToolName $toolRow.command_name) -or (Test-NotesReadToolName $toolRow.command))) { continue }
            $notesEventRows += $toolRow
        }
        $s4w.notes_read_event_rows = $notesEventRows

        # Evidence face 2: the model input. A model call after the tool result
        # carries <tool_result> in recent_turns; matching it against the selected
        # clip id proves the model actually received the notes.
        $promptToolRows = @()
        if ($null -ne $workPromptMatch.appended -and $workPromptMatch.appended.Length -gt 0) {
            $promptToolRows = @(Find-PromptRenderToolResults -Text $workPromptMatch.appended -ConversationId $conversationId -ClipId $secondClipId)
        }
        if ($promptToolRows.Count -gt 0) {
            Set-Content -LiteralPath (Join-Path $evidenceDir "s4w_prompt_tool_results.json") -Value (ConvertTo-Json -InputObject $promptToolRows -Depth 8) -Encoding UTF8
        }
        $s4w.prompt_tool_result_rows = $promptToolRows

        # Evidence face 3: the kernel log payload for this turn (persistent
        # receipt). Kernel-only hits are supporting evidence, not the primary pass
        # basis (deterministic observers can also read notes).
        $kernelWorkText = ""
        if (-not [string]::IsNullOrWhiteSpace($workKernelLogPath)) {
            $kernelWorkText = Read-AppendedText -Path $workKernelLogPath -Offset $workKernelOffset -MaxBytes 4194304
            if ($null -eq $kernelWorkText) { $kernelWorkText = "" }
            if ($kernelWorkText.Length -gt 0) {
                Set-Content -LiteralPath (Join-Path $evidenceDir "s4w_kernel_log_work.txt") -Value $kernelWorkText -Encoding UTF8
            }
        }
        $kernelNotes = Find-KernelNotesRead -Text $kernelWorkText -ClipId $secondClipId
        $s4w.kernel_notes_read = $kernelNotes

        # Evidence face 4: the chat response's executed-kernel receipt.
        $executedNotesHit = $false
        if ($workExecutedRows.Count -gt 0) {
            $executedJson = ConvertTo-Json -InputObject $workExecutedRows -Depth 12 -Compress
            $executedHasTool = ($executedJson -match 'get_midi_clip_notes|get_midi_clip_data|midi\.read_notes|midi\.read_clip_notes|midi\.read_clip_data')
            $executedNotesHit = ($executedHasTool -and $executedJson.Contains($secondClipId))
        }
        $s4w.executed_kernel_reply_contains_notes_read = $executedNotesHit

        $notesReadSources = @()
        if ($notesEventRows.Count -gt 0) { $notesReadSources += "agent_events" }
        if ($promptToolRows.Count -gt 0) { $notesReadSources += "prompt_render_tool_result" }
        if ($executedNotesHit) { $notesReadSources += "executed_kernel_reply" }
        $notesRead = ($notesReadSources.Count -gt 0)
        $s4w.notes_read = $notesRead
        $s4w.notes_read_sources = $notesReadSources
        $s4w.kernel_notes_read_support = ($kernelNotes.received -gt 0)

        $feedbackHonesty = ""
        if ($notesRead) {
            $feedbackHonesty = "notes_read"
        }
        elseif ($claimHits.Count -gt 0 -and $honestHits.Count -eq 0) {
            $feedbackHonesty = "claimed_without_notes_read"
        }
        elseif ($noteFeatureHits.Count -gt 0 -and $honestHits.Count -eq 0) {
            $feedbackHonesty = "claimed_without_notes_read"
        }
        else {
            $feedbackHonesty = "model_boundary"
        }
        $s4w.feedback_honesty = $feedbackHonesty

        # Optional telemetry row for this turn (never the sole conclusion).
        $telemetryAppendedWork = Read-AppendedText -Path $telemetryPath -Offset $telemetryOffset -MaxBytes 2097152
        $telemetryMatchedWork = $null
        if ($null -ne $telemetryAppendedWork -and $telemetryAppendedWork.Length -gt 0) {
            foreach ($telemetryLine in [regex]::Split($telemetryAppendedWork, '\r?\n')) {
                $telemetryTrimmed = $telemetryLine.Trim()
                if ($telemetryTrimmed -eq "") { continue }
                try { $telemetryRecord = $telemetryTrimmed | ConvertFrom-Json } catch { continue }
                if ([string](Get-Prop $telemetryRecord "conversation_id") -ne $conversationId) { continue }
                $telemetryMatchedWork = $telemetryRecord
                break
            }
        }
        if ($null -ne $telemetryMatchedWork) {
            $s4w.telemetry = [ordered]@{
                source = [string](Get-Prop $telemetryMatchedWork "source")
                model = [string](Get-Prop $telemetryMatchedWork "model")
                message_count = Get-Prop $telemetryMatchedWork "message_count"
                total_ms = Get-Prop $telemetryMatchedWork "total_ms"
                usage = Get-Prop $telemetryMatchedWork "usage"
                error = [string](Get-Prop $telemetryMatchedWork "error")
            }
        }
        else {
            $s4w.telemetry = [ordered]@{ found = $false; note = "no new telemetry row for this conversation" }
        }

        $feedbackEvidence = [ordered]@{
            schema_version = "vit_w02_a2a_work_feedback.v1"
            conversation_id = $conversationId
            question = $workQuestion
            probe = $workProbeKind
            stop_reason = $workStop
            goal_status = $s4w.goal_status
            completed_steps = $s4w.completed_steps
            executed_kernel_reply_count = $s4w.executed_kernel_reply_count
            executed_kernel_reply_names = $executedNames
            reply = $workReply
            reply_keyword_hits = $workKeywordHits
            reply_note_feature_hits = $noteFeatureHits
            claim_marker_hits = $claimHits
            honest_marker_hits = $honestHits
            feedback_honesty = $feedbackHonesty
            notes_read = $notesRead
            notes_read_sources = $notesReadSources
            kernel_notes_read = $kernelNotes
            tool_calls = $s4w.tool_calls
            prompt_tool_result_rows = $promptToolRows
            lesson_context = $s4w.lesson_context
            clip_id = $secondClipId
            clip_track_id = $targetTrackId
            clip_notes_signature = $secondSignature
            first_clip_id = $firstClipId
            first_clip_notes_signature = $firstSignature
        }
        Set-Content -LiteralPath (Join-Path $evidenceDir "s4w_work_feedback.json") -Value (ConvertTo-Json -InputObject $feedbackEvidence -Depth 20) -Encoding UTF8

        if ($feedbackHonesty -eq "claimed_without_notes_read") {
            Fail "evidence" ("S4W model claimed note content without a notes-read receipt for selected clip " + $secondClipId + "; claim_hits=[" + ($claimHits -join ",") + "] note_feature_hits=[" + ($noteFeatureHits -join ",") + "] notes_read_sources=[] reply=" + $workReply)
        }
        if (-not $notesRead) {
            $s4w.outcome = "model_boundary"
            $s4w.outcome_note = "the model did not read the selected clip notes and did not fabricate note content; whether this counts as PASS is for the lead"
            $summary.status = "model_boundary"
            $script:failureClass = "evidence"
            $script:failureMessage = ("S4W model_boundary: no notes-read receipt for selected clip " + $secondClipId + " (honest_hits=[" + ($honestHits -join ",") + "]); lead adjudication required, not a PASS")
            $summary.failure_class = $script:failureClass
            $summary.error = $script:failureMessage
            Write-WarnLine ("S4W model_boundary: no notes-read receipt for clip " + $secondClipId + "; lead adjudication required (NOT a PASS)")
        }
        else {
            $s4w.outcome = "notes_read"
            $s4w.outcome_note = "notes-read receipt(s) exist for the selected clip; done+reply+lesson_context asserted"
            $summary.status = "passed"
        }
        Write-Ok ("S4W: stop=done reply_len=" + [string]$workReply.Length + " clip=" + $secondClipId + " notes_read=" + [string]$notesRead + " sources=" + ($notesReadSources -join ",") + " honesty=" + $feedbackHonesty)
    }
    else {
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
    }
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

    if ($mainSessionActive -and ($HandtestPrep -or $SameStackPrep)) {
        # Prep fixture rule: the prep conversation must stay ACTIVE at the clip
        # node, otherwise the live copy would no longer be an n2 fixture. The
        # in-memory lesson session is discarded when the lead stops the stack;
        # the on-disk fixture was copied before this teardown.
        $teardown.main_session_close = [ordered]@{ skipped = "handtest_prep"; conversation_id = $conversationId; reason = "fixture must stay at the clip node (n2)" }
        $mainSessionActive = $false
    }
    elseif ($mainSessionActive) {
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

    if (($HandtestPrep -or $SameStackPrep) -and $summary.status -ne "passed" -and $script:handtestCopyDirs.Count -gt 0) {
        # Fail-honest: never leave a fixture behind that failed its own reopen
        # verification. The paths are recorded so the evidence chain is intact.
        $removed = @()
        foreach ($copyDir in @($script:handtestCopyDirs)) {
            if ([string]::IsNullOrWhiteSpace($copyDir)) { continue }
            if (-not (Test-Path -LiteralPath $copyDir)) { continue }
            try {
                Remove-Item -LiteralPath $copyDir -Recurse -Force
                $removed += $copyDir
            }
            catch {
                $teardownErrors += ("failed fixture cleanup could not remove " + $copyDir + ": " + $_.Exception.Message)
            }
        }
        $teardown.handtest_failed_fixture_removed = $removed
    }

    if ($HandtestPrep -or $SameStackPrep) {
        # Prep record: coord/runs/W02-A1-handtest/PREP-<stamp>/summary.json +
        # PREP-NOTES.md (the checkout is the script's own, so the file domain is
        # respected even when -RepoRoot points at another stack checkout).
        try {
            if ([string]::IsNullOrWhiteSpace($prepRecordDir)) { $prepRecordDir = Join-Path $RunRoot "prep_record" }
            New-Item -ItemType Directory -Path $prepRecordDir -Force | Out-Null
            $summary.prep_record_dir = $prepRecordDir
            $summary | ConvertTo-Json -Depth 40 | Set-Content -LiteralPath (Join-Path $prepRecordDir "summary.json") -Encoding UTF8
            Set-Content -LiteralPath (Join-Path $prepRecordDir "PREP-NOTES.md") -Value (New-HandtestPrepNotes -Summary $summary) -Encoding UTF8
            $teardown.prep_record = [ordered]@{ dir = $prepRecordDir; summary = (Join-Path $prepRecordDir "summary.json"); notes = (Join-Path $prepRecordDir "PREP-NOTES.md") }
            Write-Ok ("prep record: " + $prepRecordDir)
        }
        catch {
            $teardown.prep_record = [ordered]@{ error = $_.Exception.Message; dir = $prepRecordDir }
            $teardownErrors += ("prep record write failed: " + $_.Exception.Message)
        }
    }

    $teardown.status = if ($teardownErrors.Count -eq 0) { "ok" } else { "failed" }
    $teardown.errors = $teardownErrors
    $teardown.completed_at = (Get-Date).ToString("o")
    $teardown.process_tcp_udp_note = "This script never starts/stops the stack; process/TCP/UDP zero must be confirmed by the berth owner after stopping the stack."
    $summary.completed_at = (Get-Date).ToString("o")

    if ($teardownErrors.Count -gt 0 -and (($summary.status -eq "passed") -or ($summary.status -eq "model_boundary"))) {
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
    if ($SameStackPrep) {
        Write-Host "ok: W02-A1-HANDTEST-V0 same-stack prep PASSED (open project prepared at the n2 clip node; session alive; zero LLM)" -ForegroundColor Green
        Write-Host ("handtest_live: " + [string](Get-Prop $summary.handtest_prep "live_project") + " (keep this window open; do NOT reopen)")
        Write-Host ("prep_record: " + $summary.prep_record_dir)
    }
    elseif ($HandtestPrep) {
        Write-Host "ok: W02-A1-HANDTEST-V0 prep PASSED (n2 fixture + live/pristine copies verified by reopen; zero LLM)" -ForegroundColor Green
        Write-Host ("handtest_live: " + [string](Get-Prop $summary.handtest_prep "live_project"))
        Write-Host ("prep_record: " + $summary.prep_record_dir)
    }
    elseif ($QuestionMode -eq "work") {
        Write-Host "ok: W02-A2a work-feedback real-stack journey PASSED (notes-read receipt for the selected clip)" -ForegroundColor Green
    }
    else {
        Write-Host "ok: W02-A1c real-stack journey PASSED" -ForegroundColor Green
    }
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
    if ($summary.status -eq "model_boundary") {
        Write-Host ("MODEL_BOUNDARY [" + $script:failureClass + "] exit=" + [string]$script:exitCode + " - no notes-read receipt; lead adjudication required (NOT a PASS)") -ForegroundColor Yellow
    }
    else {
        Write-Host ("FAILED [" + $script:failureClass + "] exit=" + [string]$script:exitCode) -ForegroundColor Red
    }
    Write-Host ("artifact: " + (Join-Path $RunRoot "summary.json"))
}
exit $script:exitCode
