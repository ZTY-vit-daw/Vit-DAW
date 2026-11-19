[CmdletBinding()]
param(
    [string]$ScriptPath = "",
    [string]$A1cEvidencePath = ""
)

# W02-A2a work-question evidence helper self-test (zero stack, zero LLM).
# It extracts the evidence helpers from the real executor via the PowerShell
# AST, feeds synthetic /agent/events, prompt-render and kernel-log shapes, and
# asserts positive hits plus false-positive negatives. It never starts a stack.
$ErrorActionPreference = "Stop"
$repoRoot = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot "../../../..")).Path
if ([string]::IsNullOrWhiteSpace($ScriptPath)) { $ScriptPath = Join-Path $repoRoot "scripts/run_w02_a1c_smoke.ps1" }
if ([string]::IsNullOrWhiteSpace($A1cEvidencePath)) { $A1cEvidencePath = Join-Path $repoRoot "coord/runs/W02-A1c/real-stack/run_20261119_2039/evidence/s4_prompt_render.jsonl" }
if (-not (Test-Path -LiteralPath $ScriptPath)) { throw ("executor not found: " + $ScriptPath) }

$tokens = $null
$parseErrors = $null
$ast = [System.Management.Automation.Language.Parser]::ParseFile($ScriptPath, [ref]$tokens, [ref]$parseErrors)
if ($parseErrors.Count -gt 0) { throw ("executor parse errors: " + $parseErrors.Count) }

$names = @(
    "Get-Prop", "Get-PropOr", "Read-AppendedText", "Test-NotesReadToolName",
    "Find-ClipIdInObject", "Get-ToolCallRowsFromEvents", "Find-PromptRenderToolResults",
    "Find-KernelNotesRead"
)
$defs = @()
$defs += '$script:NotesReadToolNames = @("get_midi_clip_notes", "midi.read_notes", "midi.read_clip_notes", "get_midi_clip_data", "midi.read_clip_data")'
foreach ($fn in $ast.FindAll({ param($n) ($n -is [System.Management.Automation.Language.FunctionDefinitionAst]) -and ($names -contains $n.Name) }, $true)) {
    $defs += $fn.Extent.Text
}
if ($defs.Count -lt ($names.Count + 1)) { throw ("helper definitions missing: " + $defs.Count) }
Invoke-Expression ($defs -join ([string][char]10))

# 1) /agent/events parsing: item.started + item.completed for the selected clip.
$events = [pscustomobject]@{
    status = "ok"
    events = @(
        [pscustomobject]@{ seq = 1; type = "item.started"; status = "running"; item_id = "t1"; payload = [pscustomobject]@{ tool = "midi.read_notes"; command_name = ""; args = [pscustomobject]@{ clip_id = "1014" }; result = $null; error = "" } },
        [pscustomobject]@{ seq = 2; type = "item.completed"; status = "completed"; item_id = "t1"; payload = [pscustomobject]@{ tool = "midi.read_notes"; command_name = "get_midi_clip_notes"; args = [pscustomobject]@{ clip_id = "1014" }; result = [pscustomobject]@{ notes = @(@{ pitch = 67 }, @{ pitch = 71 }, @{ pitch = 74 }) }; error = "" } }
    )
}
$rows = @(Get-ToolCallRowsFromEvents $events)
if ($rows.Count -ne 2) { throw ("tool rows=" + $rows.Count) }
$hit = @($rows | Where-Object { ($_.type -eq "item.completed") -and ($_.clip_id -eq "1014") -and (Test-NotesReadToolName $_.tool) })
if ($hit.Count -ne 1) { throw ("notes event rows=" + $hit.Count) }
if ($hit[0].notes_count -ne 3) { throw ("notes count=" + $hit[0].notes_count) }
if (@($rows | Where-Object { Test-NotesReadToolName $_.command_name }).Count -ne 1) { throw "command_name match" }

# Nested legacy command payload shape must also yield the clip id.
$nested = [pscustomobject]@{ seq = 3; type = "item.completed"; status = "completed"; item_id = "t2"; payload = [pscustomobject]@{ tool = "midi.read_notes"; command_name = "get_midi_clip_notes"; args = $null; command = $null; result = $null; error = ""; command_raw = [pscustomobject]@{ legacy = [pscustomobject]@{ args = [pscustomobject]@{ clip_id = "1014" } } } } }
$nestedRows = @(Get-ToolCallRowsFromEvents ([pscustomobject]@{ status = "ok"; events = @($nested) }))
if ($nestedRows.Count -ne 1 -or $nestedRows[0].clip_id -ne "1014") { throw ("nested clip id=" + $nestedRows[0].clip_id) }

# 2) prompt render <tool_result> for the selected clip (same field).
$snapObj = @{ recent_turns = @(@{ role = "user"; content = '<tool_result>{"result_preview":{"clip_id":"1014","notes":[{"pitch":67}]},"tool":"midi.read_notes"}</tool_result>' }) }
$row = @{ conversation_id = "c1"; turn = 2; tag = "message_loop_system"; model_snapshot = ($snapObj | ConvertTo-Json -Depth 6 -Compress); system_prompt = "tool catalog lists midi.read_notes" }
$pr = ($row | ConvertTo-Json -Depth 8 -Compress)
$found = @(Find-PromptRenderToolResults -Text $pr -ConversationId "c1" -ClipId "1014")
if ($found.Count -ne 1) { throw ("prompt hit=" + $found.Count) }
if (-not $found[0].has_notes_key) { throw "prompt notes key" }

# 3) no <tool_result> (tool catalog only) must not count.
$row2 = @{ conversation_id = "c1"; turn = 1; tag = "x"; model_snapshot = '{"lesson_context":{}}'; system_prompt = "midi.read_notes catalog 1014" }
$pr2 = ($row2 | ConvertTo-Json -Depth 5 -Compress)
if (@(Find-PromptRenderToolResults -Text $pr2 -ConversationId "c1" -ClipId "1014").Count -ne 0) { throw "prompt false positive" }

# 4) tool result for another clip must not count.
$row3 = @{ conversation_id = "c1"; turn = 2; tag = "x"; model_snapshot = '<tool_result>{"result_preview":{"clip_id":"1013"},"tool":"midi.read_notes"}</tool_result>' }
$pr3 = ($row3 | ConvertTo-Json -Depth 5 -Compress)
if (@(Find-PromptRenderToolResults -Text $pr3 -ConversationId "c1" -ClipId "1014").Count -ne 0) { throw "prompt clip false positive" }

# 5) kernel log receipt for the selected clip (compact JSON) and negatives.
$k = 'ZmqGateway: {"payload":{"legacy":{"args":{"agent_action_id":"a","clip_id":"1014","tool_call_id":"t"},"cmd":"get_midi_clip_notes"}}}'
$kh = Find-KernelNotesRead -Text $k -ClipId "1014"
if ($kh.received -ne 1) { throw ("kernel received=" + $kh.received) }
if ($kh.tool_call_id_hits -ne 1) { throw ("kernel tool_call_id=" + $kh.tool_call_id_hits) }
if ((Find-KernelNotesRead -Text $k -ClipId "1013").received -ne 0) { throw "kernel clip false positive" }
$k2 = 'Command executed: get_midi_clip_notes, result: {'
$kh2 = Find-KernelNotesRead -Text $k2 -ClipId "1014"
if ($kh2.received -ne 0 -or $kh2.executed_command_lines -ne 1) { throw "kernel executed count" }

# 6) the real A1c prompt render row (no tool result) must not count.
if (Test-Path -LiteralPath $A1cEvidencePath) {
    $text = [System.IO.File]::ReadAllText($A1cEvidencePath)
    if (@(Find-PromptRenderToolResults -Text $text -ConversationId "w02-a1c-20261119_203850" -ClipId "1014").Count -ne 0) { throw "real prompt row false positive" }
}

"HELPER_TESTS_OK"
