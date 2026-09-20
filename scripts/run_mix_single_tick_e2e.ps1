[CmdletBinding()]
param(
    [string]$RepoRoot = "",
    [string]$AgentHttp = "http://127.0.0.1:7878",
    [string]$AgentHttpAddr = "127.0.0.1:7878",
    [string]$ZmqReqPort = "5555",
    [string]$ZmqSubPort = "5556",
    [string]$KernelExe = "",
    [string]$Track1Path = "",
    [string]$Track2Path = "",
    [switch]$SkipBuild,
    [switch]$ReuseAgent,
    [switch]$ReuseKernel,
    [switch]$StartKernel,
    [switch]$NoStartKernel,
    [switch]$StartUI,
    [int]$WaitSeconds = 30,
    [int]$ChatTimeoutSec = 240,
    # 720s default (PORT-PS1-SYNC-2): reasoning-model turns occasionally run a
    # heavy reply past 300s; measured stable at 720s (2026-09-20 flash A/B
    # experiment — pro showed no capability premium, only ~6x latency, so the
    # settle window grows while the engine stays flash).
    [int]$ChatSettleSeconds = 720
)

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
    $prop = $Object.PSObject.Properties[$Name]
    if ($null -eq $prop) {
        return $null
    }
    return $prop.Value
}

function Get-TcpListener {
    param([int]$Port)
    return Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue |
        Select-Object -First 1
}

function Resolve-KernelExe {
    param(
        [string]$RepoRoot,
        [string]$Explicit
    )
    if (-not [string]::IsNullOrWhiteSpace($Explicit)) {
        return (Resolve-Path -LiteralPath $Explicit).Path
    }
    $candidates = @(
        (Join-Path $RepoRoot "VitApp\build_release\VitApp_artefacts\Release\VitApp.exe"),
        (Join-Path $RepoRoot "VitApp\build\VitApp_artefacts\Release\VitApp.exe"),
        (Join-Path $RepoRoot "VitApp\build_release\VitApp.exe"),
        (Join-Path $RepoRoot "Export\_build\Vit_DAW_v0.9_release\kernel\VitApp.exe"),
        (Join-Path $RepoRoot "Export\staging\runtime\VitApp.exe")
    )
    foreach ($candidate in $candidates) {
        if (Test-Path -LiteralPath $candidate) {
            return (Resolve-Path -LiteralPath $candidate).Path
        }
    }
    Fail "Could not find a VitApp kernel executable. Pass -KernelExe."
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

function Start-TestKernel {
    param(
        [string]$KernelPath,
        [int]$ReqPort,
        [int]$SubPort,
        [int]$TimeoutSeconds,
        [bool]$ReuseExisting
    )
    Write-Step "Prepare kernel"
    $desiredPath = [System.IO.Path]::GetFullPath($KernelPath)
    $listener = Get-TcpListener -Port $ReqPort
    if ($null -ne $listener) {
        $proc = Get-Process -Id $listener.OwningProcess -ErrorAction SilentlyContinue
        $runningPath = ""
        if ($null -ne $proc) {
            $runningPath = [string]$proc.Path
        }
        if ($ReuseExisting) {
            Write-Ok ("reusing existing kernel command port: " + $ReqPort + " pid=" + $listener.OwningProcess)
            return
        }
        if (-not [string]::IsNullOrWhiteSpace($runningPath) -and
            [System.IO.Path]::GetFullPath($runningPath).Equals($desiredPath, [System.StringComparison]::OrdinalIgnoreCase)) {
            Write-Ok ("desired kernel already listening: " + $ReqPort + " pid=" + $listener.OwningProcess)
            return
        }
        Write-WarnLine ("stopping existing kernel pid=" + $listener.OwningProcess + " path=" + $runningPath)
        Stop-Process -Id $listener.OwningProcess -Force
        Start-Sleep -Milliseconds 500
    }

    if (-not (Test-Path -LiteralPath $desiredPath)) {
        Fail ("Missing kernel exe: " + $desiredPath)
    }
    Start-Process -FilePath $desiredPath -WorkingDirectory (Split-Path -Parent $desiredPath) -WindowStyle Hidden | Out-Null
    $ready = Wait-TcpListener -Port $ReqPort -TimeoutSeconds $TimeoutSeconds
    if ($null -eq $ready) {
        Fail ("Kernel command port did not become ready: " + $ReqPort + " using " + $desiredPath)
    }
    Write-Ok ("started kernel: " + $desiredPath + " pid=" + $ready.OwningProcess)
    $sub = Wait-TcpListener -Port $SubPort -TimeoutSeconds 5
    if ($null -eq $sub) {
        Write-WarnLine ("kernel telemetry port not listening yet: " + $SubPort)
    }
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
        source = "mix_single_tick_e2e"
    } -TimeoutSec 120
}

function Invoke-AgentChat {
    param(
        [string]$ConversationID,
        [string]$Message
    )
    $rawResponse = Invoke-Json -Method POST -Uri ($AgentHttp.TrimEnd("/") + "/agent/chat") -Body @{
        conversation_id = $ConversationID
        message = $Message
        context = @{
            agent_mode = "chat"
        }
    } -TimeoutSec $ChatTimeoutSec
    return Wait-ChatTurnSettled -Raw $rawResponse -ConversationID $ConversationID -SettleSeconds $ChatSettleSeconds
}

function Wait-ChatTurnSettled {
    # chat_settle anchor (PORT-PS1-SYNC-2, ported from
    # run_vit_product_path_smoke.ps1 SETTLE-1 / the mac agent_chat_settled
    # twin): when a turn is sliced out (goal_status=waiting_continue /
    # stop_reason=limit_reached), poll the durable continuation via
    # /agent/runtime/status until goal.status reaches a terminal value, then
    # synthesize the settled response from /agent/events. Assertions read the
    # returned object; the raw sliced response is preserved on it as raw_*.
    # Trigger accepts either slice marker: run 1 of the 2026-09-20 PC rerun
    # surfaced a raw stop_reason=limit_reached observe turn.
    param(
        [object]$Raw,
        [string]$ConversationID,
        [int]$SettleSeconds
    )
    $rawStopReasonEarly = [string](Get-OptionalProperty -Object $Raw -Name "stop_reason")
    $rawGoalStatus = [string](Get-OptionalProperty -Object $Raw -Name "goal_status")
    # Double-hop refinement (PORT-PS1-SYNC-3): a genuine agent-loop slice always
    # carries stop_reason=limit_reached (runner.pause). A synchronous terminal
    # park can also arrive with goal_status=waiting_continue plus a meaningful
    # stop_reason — the D1 applied turn (d1_post_action_evaluation_required,
    # workflow free_state_d1_s1) is exactly that shape. Such a response is
    # complete, not sliced: polling for a terminal goal would hang until the
    # continuation scheduler's evaluation slices finish (or remap the stop
    # reason through an audition judgment), so return it as-is.
    $genuinelySliced = ($rawStopReasonEarly -eq "limit_reached") -or
        (($rawGoalStatus -eq "waiting_continue") -and [string]::IsNullOrWhiteSpace($rawStopReasonEarly))
    if (-not $genuinelySliced) {
        return $Raw
    }
    Write-WarnLine ("chat turn sliced out (waiting_continue/limit_reached) - waiting for the durable continuation to settle (budget " + $SettleSeconds + "s, conversation " + $ConversationID + ")")
    $terminalGoals = @("completed", "failed", "stopped", "cancelled", "waiting_confirmation", "waiting_clarification")
    $settledGoal = ""
    $deadline = (Get-Date).AddSeconds($SettleSeconds)
    while ((Get-Date) -lt $deadline) {
        Start-Sleep -Seconds 5
        try {
            $poll = Invoke-Json -Method GET -Uri ($AgentHttp.TrimEnd("/") + "/agent/runtime/status") -TimeoutSec 30
            $pollGoal = Get-OptionalProperty -Object $poll -Name "goal"
            $polledStatus = [string](Get-OptionalProperty -Object $pollGoal -Name "status")
            if ($polledStatus -ne $settledGoal) {
                $settledGoal = $polledStatus
                Write-WarnLine ("settle poll: goal=" + $settledGoal)
            }
        }
        catch {
            continue
        }
        if ($terminalGoals -contains $settledGoal) {
            break
        }
    }
    $events = $null
    try {
        $encodedConversationID = [System.Uri]::EscapeDataString($ConversationID)
        $events = Invoke-Json -Method GET -Uri ($AgentHttp.TrimEnd("/") + "/agent/events?conversation_id=" + $encodedConversationID + "&since=0&limit=500") -TimeoutSec 60
    }
    catch { }
    # Code-point literal (file has no UTF-8 BOM; raw CJK would be misread
    # under the PS 5.1 ANSI code page — same convention as the turn messages).
    $placeholder = Join-UnicodeChars @(0x6211, 0x8FD8, 0x5728, 0x7EE7, 0x7EED, 0x5904, 0x7406, 0x8FD9, 0x4E2A, 0x4EFB, 0x52A1, 0xFF0C, 0x5B8C, 0x6210, 0x540E, 0x518D, 0x5411, 0x4F60, 0x6C47, 0x62A5, 0x3002)
    $delivered = @()
    $turnCompletedTexts = @()
    $toolRows = @()
    foreach ($agentEvent in @((Get-OptionalProperty -Object $events -Name "events"))) {
        if ($null -eq $agentEvent) {
            continue
        }
        $parts = @()
        foreach ($fieldName in @("title", "body", "summary")) {
            $fieldValue = [string](Get-OptionalProperty -Object $agentEvent -Name $fieldName)
            if (-not [string]::IsNullOrWhiteSpace($fieldValue)) {
                $parts += $fieldValue
            }
        }
        if ($parts.Count -gt 0) {
            $delivered += ($parts -join " | ")
        }
        if ([string](Get-OptionalProperty -Object $agentEvent -Name "type") -eq "turn.completed") {
            $completedText = @()
            foreach ($fieldName in @("title", "body")) {
                $fieldValue = [string](Get-OptionalProperty -Object $agentEvent -Name $fieldName)
                if (-not [string]::IsNullOrWhiteSpace($fieldValue)) {
                    $completedText += $fieldValue
                }
            }
            if ($completedText.Count -gt 0) {
                $turnCompletedTexts += ($completedText -join " ")
            }
        }
        $sources = @($agentEvent)
        $payload = Get-OptionalProperty -Object $agentEvent -Name "payload"
        if ($null -ne $payload) {
            $sources += $payload
        }
        foreach ($source in $sources) {
            $toolName = [string](Get-OptionalProperty -Object $source -Name "tool")
            if ([string]::IsNullOrWhiteSpace($toolName)) {
                $toolName = [string](Get-OptionalProperty -Object $source -Name "command_name")
            }
            if ([string]::IsNullOrWhiteSpace($toolName)) {
                $toolName = [string](Get-OptionalProperty -Object $source -Name "command")
            }
            if (-not [string]::IsNullOrWhiteSpace($toolName)) {
                $alreadyListed = $false
                foreach ($row in $toolRows) {
                    if ([string]$row["tool"] -eq $toolName) {
                        $alreadyListed = $true
                        break
                    }
                }
                if (-not $alreadyListed) {
                    $toolRows += @{ tool = $toolName }
                }
            }
        }
    }
    $replyCandidates = @()
    foreach ($text in ($delivered + $turnCompletedTexts)) {
        if (-not $text.Contains($placeholder)) {
            $replyCandidates += $text
        }
    }
    $settledReply = [string](Get-OptionalProperty -Object $Raw -Name "reply")
    if ($replyCandidates.Count -gt 0) {
        $settledReply = $replyCandidates[$replyCandidates.Count - 1]
    }
    $stopReasonMap = @{
        waiting_confirmation = "needs_confirmation"
        waiting_clarification = "needs_clarification"
        completed = "done"
    }
    $settledStopReason = [string](Get-OptionalProperty -Object $Raw -Name "stop_reason")
    if ($stopReasonMap.Contains($settledGoal)) {
        $settledStopReason = $stopReasonMap[$settledGoal]
    }
    $settledGoalStatus = $settledGoal
    if ([string]::IsNullOrWhiteSpace($settledGoalStatus)) {
        $settledGoalStatus = "settle_timeout"
    }
    # Faithful completion (PORT-PS1-SYNC-2 run-2 fix, same as the product-path
    # twin): a waiting_confirmation settled turn carries needs_confirmation=true
    # and its pending candidate as a typed event in the unsliced response.
    $settledNeedsConfirmation = [bool](Get-OptionalProperty -Object $Raw -Name "needs_confirmation")
    if ($settledGoal -eq "waiting_confirmation") {
        $settledNeedsConfirmation = $true
    }
    $rawStopReasonValue = [string](Get-OptionalProperty -Object $Raw -Name "stop_reason")
    $rawReplyValue = [string](Get-OptionalProperty -Object $Raw -Name "reply")
    $settled = $Raw
    $settled | Add-Member -Force -MemberType NoteProperty -Name "stop_reason" -Value $settledStopReason
    $settled | Add-Member -Force -MemberType NoteProperty -Name "goal_status" -Value $settledGoalStatus
    $settled | Add-Member -Force -MemberType NoteProperty -Name "reply" -Value $settledReply
    $settled | Add-Member -Force -MemberType NoteProperty -Name "settled_from" -Value "continuation+events (pc chat_settle anchor, mac-aligned)"
    $settled | Add-Member -Force -MemberType NoteProperty -Name "raw_stop_reason" -Value $rawStopReasonValue
    $settled | Add-Member -Force -MemberType NoteProperty -Name "raw_reply" -Value $rawReplyValue
    $settled | Add-Member -Force -MemberType NoteProperty -Name "delivered_event_count" -Value $delivered.Count
    if ($settledGoal -eq "waiting_confirmation") {
        $settled | Add-Member -Force -MemberType NoteProperty -Name "needs_confirmation" -Value $settledNeedsConfirmation
        # mac parity (run_vit_product_path_smoke_mac.sh settle anchor): the
        # raw sliced reply keeps needs_confirmation=false while the durable
        # continuation holds the real pending interaction. Derive the
        # confirmation surface from /agent/runtime/status continuations so
        # the settled object stays self-consistent with the mapped
        # stop_reason, exactly like the mac twin's pending_interaction walk.
        try {
            $statusSnapshot = Invoke-Json -Method GET -Uri ($AgentHttp.TrimEnd("/") + "/agent/runtime/status") -TimeoutSec 30
            $pendingInteraction = $null
            foreach ($continuationRow in @(Get-OptionalProperty -Object $statusSnapshot -Name "continuations")) {
                $continuationConversationID = [string](Get-OptionalProperty -Object $continuationRow -Name "conversation_id")
                if (-not [string]::IsNullOrWhiteSpace($ConversationID) -and $continuationConversationID -ne $ConversationID) {
                    continue
                }
                $interaction = Get-OptionalProperty -Object $continuationRow -Name "pending_interaction"
                if ($null -ne $interaction -and (([string](Get-OptionalProperty -Object $interaction -Name "kind")).Contains("confirmation"))) {
                    $pendingInteraction = $interaction
                    break
                }
            }
            if ($null -ne $pendingInteraction) {
                # null-filter the base list: @($null) in PowerShell is a
                # one-element array holding null (run-3 lesson), and the
                # typed-event assertions dot into every row; the mac twin
                # filters with isinstance(e, dict) for the same reason.
                $typedEventRows = @(Get-OptionalProperty -Object $settled -Name "typed_events" | Where-Object { $null -ne $_ })
                # pscustomobject, not [ordered]: PS 5.1 StrictMode throws
                # PropertyNotFoundStrict on dot access of OrderedDictionary
                # keys, and the assertions read $typedEvent.event_type.
                $pendingEvent = [pscustomobject]@{
                    event_type = "PendingCandidate"
                    source = "settled_pending_interaction"
                    interaction_id = [string](Get-OptionalProperty -Object $pendingInteraction -Name "interaction_id")
                    kind = [string](Get-OptionalProperty -Object $pendingInteraction -Name "kind")
                }
                $typedEventRows += $pendingEvent
                $settled | Add-Member -Force -MemberType NoteProperty -Name "typed_events" -Value $typedEventRows
                $settledPlanID = ([string](Get-OptionalProperty -Object $pendingInteraction -Name "plan_id")).Trim()
                $settledProposalID = ([string](Get-OptionalProperty -Object $pendingInteraction -Name "proposal_id")).Trim()
                if (-not [string]::IsNullOrWhiteSpace($settledPlanID)) {
                    $settled | Add-Member -Force -MemberType NoteProperty -Name "plan_id" -Value $settledPlanID
                }
                elseif (-not [string]::IsNullOrWhiteSpace($settledProposalID)) {
                    $settled | Add-Member -Force -MemberType NoteProperty -Name "plan_id" -Value $settledProposalID
                }
            }
            else {
                Write-WarnLine ("settle: no pending confirmation interaction in /agent/runtime/status for conversation " + $ConversationID)
            }
        }
        catch {
            Write-WarnLine ("settle: pending-interaction derivation failed: " + $_.Exception.Message)
        }
    }
    $rawToolRows = @(Get-OptionalProperty -Object $Raw -Name "executed_kernel_reply" | Where-Object { $null -ne $_ })
    if ($rawToolRows.Count -eq 0 -and $toolRows.Count -gt 0) {
        $settled | Add-Member -Force -MemberType NoteProperty -Name "executed_kernel_reply" -Value $toolRows
    }
    if ($null -eq (Get-OptionalProperty -Object $Raw -Name "typed_events")) {
        # mac parity: assertions treat a settled response without typed_events
        # as an empty list.
        $settled | Add-Member -Force -MemberType NoteProperty -Name "typed_events" -Value @()
    }
    return $settled
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
    param([object]$Track)
    $trackID = [string](Get-OptionalProperty -Object $Track -Name "track_id")
    if ([string]::IsNullOrWhiteSpace($trackID)) {
        $trackID = [string](Get-OptionalProperty -Object $Track -Name "id")
    }
    return $trackID
}

function Resolve-TrackIndex {
    param([object]$Track)
    $trackIndex = [string](Get-OptionalProperty -Object $Track -Name "user_track_index")
    if ([string]::IsNullOrWhiteSpace($trackIndex)) {
        $trackIndex = [string](Get-OptionalProperty -Object $Track -Name "track_index")
    }
    return $trackIndex
}

function Project-State {
    return Invoke-AgentTool -Tool "project.state" -ToolArgs @{} -Confirmed $false
}

function Reset-FixtureProject {
    Write-Step "Reset fixture project"
    $newProject = Invoke-AgentTool -Tool "project.new" -ToolArgs @{} -Confirmed $true
    $newProjectStatus = [string](Get-OptionalProperty -Object $newProject -Name "status")
    if ($newProjectStatus -eq "ok") {
        Write-Ok "project.new reset completed"
        Start-Sleep -Milliseconds 500
    }
    else {
        $newProjectError = [string](Get-OptionalProperty -Object $newProject -Name "error")
        Write-WarnLine ("project.new unavailable; falling back to project.clear/delete. error=" + $newProjectError)
    }

    $clear = Invoke-AgentTool -Tool "project.clear" -ToolArgs @{} -Confirmed $true
    Assert-StatusOk -Response $clear -Label "project.clear before fixture reset"

    $state = Project-State
    Assert-StatusOk -Response $state -Label "project.state before fixture reset"

    $tracks = @()
    if ($null -ne $state.result -and $null -ne $state.result.tracks) {
        $tracks = @($state.result.tracks)
    }

    $audioTracks = @()
    foreach ($track in $tracks) {
        $trackType = [string](Get-OptionalProperty -Object $track -Name "type")
        if ($trackType -eq "audio") {
            $audioTracks += $track
        }
    }

    $deleted = 0
    $toDelete = @()
    if ($audioTracks.Count -gt 1) {
        $toDelete = @($audioTracks |
            Sort-Object { [int](Resolve-TrackIndex -Track $_) } -Descending |
            Select-Object -First ($audioTracks.Count - 1))
    }
    foreach ($track in $toDelete) {
        $trackID = Resolve-TrackID -Track $track
        $trackType = [string](Get-OptionalProperty -Object $track -Name "type")
        if ([string]::IsNullOrWhiteSpace($trackID) -or $trackType -ne "audio") {
            continue
        }
        $deleteArgs = @{ track_id = $trackID }
        $trackIndex = Resolve-TrackIndex -Track $track
        if (-not [string]::IsNullOrWhiteSpace($trackIndex)) {
            $deleteArgs["track_index"] = [int]$trackIndex
        }
        $delete = Invoke-AgentTool -Tool "track.delete" -ToolArgs $deleteArgs -Confirmed $true
        Assert-StatusOk -Response $delete -Label ("track.delete " + $trackID)
        $deleted++
    }

    if ($deleted -gt 0) {
        Write-Ok ("deleted existing audio tracks: " + $deleted)
    }
    else {
        Write-Ok "no existing audio tracks to delete"
    }

    $after = Project-State
    Assert-StatusOk -Response $after -Label "project.state after fixture reset"
    $remaining = @()
    if ($null -ne $after.result -and $null -ne $after.result.tracks) {
        foreach ($track in @($after.result.tracks)) {
            $trackType = [string](Get-OptionalProperty -Object $track -Name "type")
            if ($trackType -eq "audio") {
                $remaining += $track
            }
        }
    }
    return @($remaining | Sort-Object { [int](Resolve-TrackIndex -Track $_) })
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
    $preferredStatus = [string](Get-OptionalProperty -Object $preferred -Name "status")
    if ($preferredStatus -eq "ok") {
        return $preferred
    }

    $preferredError = [string](Get-OptionalProperty -Object $preferred -Name "error")
    if ($preferredError -match "Unknown command: import_media_to_track") {
        Write-WarnLine ($Label + ": kernel does not support import_media_to_track; falling back to clip.import_audio")
        return Invoke-AgentTool -Tool "clip.import_audio" -ToolArgs @{
            track_id = $TrackID
            file_path = $FilePath
            offset_time = 0
        } -Confirmed $true
    }

    return $preferred
}

function Assert-Equals {
    param(
        [string]$Actual,
        [string]$Expected,
        [string]$Label
    )
    if ($Actual -ne $Expected) {
        Fail ($Label + ": got '" + $Actual + "', want '" + $Expected + "'")
    }
}

function Assert-InSet {
    param(
        [string]$Actual,
        [string[]]$Expected,
        [string]$Label
    )
    if ($Expected -notcontains $Actual) {
        Fail ($Label + ": got '" + $Actual + "', want one of [" + ($Expected -join ", ") + "]")
    }
}

function Wait-LogPattern {
    param(
        [string]$LogPath,
        [string]$Pattern,
        [int]$TimeoutSeconds
    )
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    do {
        if (Test-Path -LiteralPath $LogPath) {
            $hit = Select-String -Path $LogPath -Pattern $Pattern -SimpleMatch -ErrorAction SilentlyContinue | Select-Object -First 1
            if ($null -ne $hit) {
                return $hit.Line
            }
        }
        Start-Sleep -Milliseconds 250
    } while ((Get-Date) -lt $deadline)
    Fail ("Timed out waiting for log pattern: " + $Pattern)
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
        $out = Add-ToolName -Names $out -Name ([string](Get-OptionalProperty -Object $row -Name "tool"))
        $out = Add-ToolName -Names $out -Name ([string](Get-OptionalProperty -Object $row -Name "command_name"))
        $out = Add-ToolName -Names $out -Name ([string](Get-OptionalProperty -Object $row -Name "command"))
    }
    return $out
}

function Assert-AnyToolPresent {
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
    Fail ("Expected " + $Label + " in executed route. route=" + ($Tools -join ", "))
}

function Assert-AnyToolAbsent {
    param(
        [string[]]$Tools,
        [string[]]$Aliases,
        [string]$Label
    )
    foreach ($alias in $Aliases) {
        if ($Tools -contains $alias) {
            Fail ("Unexpected " + $Label + " in executed route. route=" + ($Tools -join ", "))
        }
    }
}

function Get-ConfirmationFaceKinds {
    # Double-hop layering (PORT-PS1-SYNC-3): collect the confirmation-face
    # kinds a response exposes. An unsliced parking response carries them on
    # interaction_requests[].kind; a settle-synthesized response rebuilds the
    # pending interaction as a typed_events row whose kind is the interaction
    # kind. The generic typed-candidate marker ("PendingCandidate") is not a
    # face kind and is skipped.
    param([object]$Response)
    $kinds = @()
    foreach ($request in @(Get-OptionalProperty -Object $Response -Name "interaction_requests")) {
        $kind = [string](Get-OptionalProperty -Object $request -Name "kind")
        if (-not [string]::IsNullOrWhiteSpace($kind)) {
            $kinds += $kind
        }
    }
    foreach ($typedEvent in @(Get-OptionalProperty -Object $Response -Name "typed_events")) {
        $kind = [string](Get-OptionalProperty -Object $typedEvent -Name "kind")
        if (-not [string]::IsNullOrWhiteSpace($kind) -and $kind -ne "PendingCandidate") {
            $kinds += $kind
        }
    }
    return $kinds
}

function Assert-BoundedMixTickPayload {
    # Hop-1 parks a concrete bounded tick (agent side: pendingMixTickEventPayload
    # + validatePendingMixTickCandidate): a non-empty track target, a native
    # operation, and for the numeric domains a non-zero bounded delta
    # (|delta_db| <= 2, |delta_pan| <= 0.15, -1 <= target_pan <= 1). The EQ /
    # broadband-compression domains carry their bounds in the admission, not in
    # this payload, so only the target and operation are checked there.
    param(
        [object]$Response,
        [string]$Label
    )
    $payload = $null
    foreach ($request in @(Get-OptionalProperty -Object $Response -Name "interaction_requests")) {
        $row = Get-OptionalProperty -Object $request -Name "payload"
        if ($null -ne $row) {
            $payload = $row
            break
        }
    }
    if ($null -eq $payload) {
        Fail ($Label + ": no interaction payload on the response")
    }
    $trackID = [string](Get-OptionalProperty -Object $payload -Name "track_id")
    if ([string]::IsNullOrWhiteSpace($trackID)) {
        Fail ($Label + ": payload track_id is empty")
    }
    $operation = [string](Get-OptionalProperty -Object $payload -Name "operation")
    Assert-InSet -Actual $operation -Expected @("track_gain_adjust", "track_pan_adjust", "track_pan_set", "static_eq_band_adjust", "broadband_threshold_adjust") -Label ($Label + " operation")
    if ($operation -eq "track_gain_adjust") {
        $deltaDB = [double](Get-OptionalProperty -Object $payload -Name "delta_db")
        if ($deltaDB -eq 0 -or [math]::Abs($deltaDB) -gt 2) {
            Fail ($Label + ": delta_db out of the bounded non-zero +/-2 range: " + $deltaDB)
        }
    }
    elseif ($operation -eq "track_pan_adjust") {
        $deltaPan = [double](Get-OptionalProperty -Object $payload -Name "delta_pan")
        if ($deltaPan -eq 0 -or [math]::Abs($deltaPan) -gt 0.15) {
            Fail ($Label + ": delta_pan out of the bounded non-zero +/-0.15 range: " + $deltaPan)
        }
    }
    elseif ($operation -eq "track_pan_set") {
        $targetPan = [double](Get-OptionalProperty -Object $payload -Name "target_pan")
        if ($targetPan -lt -1 -or $targetPan -gt 1) {
            Fail ($Label + ": target_pan out of the -1..+1 range: " + $targetPan)
        }
    }
    return $trackID
}

$RepoRoot = Resolve-RepoRoot -Explicit $RepoRoot
$ScriptsDir = Join-Path $RepoRoot "scripts"
$WorkspaceDir = Join-Path $RepoRoot "VitApp\Workspace"
$LogsDir = Join-Path $WorkspaceDir "Logs"
$AgentLog = Join-Path $LogsDir "agent_last.log"
$DevSmoke = Join-Path $ScriptsDir "dev_agent_smoke.ps1"
$KernelExe = Resolve-KernelExe -RepoRoot $RepoRoot -Explicit $KernelExe

if ([string]::IsNullOrWhiteSpace($Track1Path)) {
    $Track1Path = Join-Path $RepoRoot "test_100hz_10s.wav"
}
if ([string]::IsNullOrWhiteSpace($Track2Path)) {
    $Track2Path = Join-Path $RepoRoot "test_target_3s.wav"
}
$Track1Path = (Resolve-Path -LiteralPath $Track1Path).Path
$Track2Path = (Resolve-Path -LiteralPath $Track2Path).Path
$shouldStartKernel = -not $NoStartKernel
if ($StartKernel) {
    $shouldStartKernel = $true
}

Write-Step "Mix single tick E2E"
Write-Host ("repo: " + $RepoRoot)
Write-Host ("agent http: " + $AgentHttp)
Write-Host ("start kernel: " + [string]$shouldStartKernel)
Write-Host ("kernel exe: " + $KernelExe)
Write-Host ("track 1: " + $Track1Path)
Write-Host ("track 2: " + $Track2Path)
Write-Host "warning: this test creates a fresh DAW project and imports fixture audio."

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
    WaitSeconds = $WaitSeconds
}
if ($SkipBuild) {
    $smokeArgs["SkipBuild"] = $true
}
if (-not $ReuseAgent) {
    $smokeArgs["RestartAgent"] = $true
}
if ($StartUI) {
    $smokeArgs["StartUI"] = $true
}
& $DevSmoke @smokeArgs
if (-not $?) {
    Fail "dev_agent_smoke failed"
}

if ($shouldStartKernel) {
    Start-TestKernel -KernelPath $KernelExe -ReqPort ([int]$ZmqReqPort) -SubPort ([int]$ZmqSubPort) -TimeoutSeconds $WaitSeconds -ReuseExisting ([bool]$ReuseKernel)
}

$req = Get-TcpListener -Port ([int]$ZmqReqPort)
if ($null -eq $req) {
    Fail ("Kernel command port is not listening: " + $ZmqReqPort + ". Run with the default kernel start behavior, or open the DAW/kernel first.")
}
Write-Ok ("kernel command port listening: " + $ZmqReqPort + " pid=" + $req.OwningProcess)

Write-Step "Create two-track fixture"
$remainingTracks = @(Reset-FixtureProject)
if ($remainingTracks.Count -gt 1) {
    Fail ("fixture reset left more than one audio track: " + $remainingTracks.Count)
}

if ($remainingTracks.Count -eq 1) {
    $track1ID = Resolve-TrackID -Track $remainingTracks[0]
    if ([string]::IsNullOrWhiteSpace($track1ID)) {
        Fail "fixture reset left one audio track, but its track ID could not be resolved"
    }
    Write-Ok ("reusing remaining audio track as Track 1: " + $track1ID)
}
else {
    $track1 = Invoke-AgentTool -Tool "track.add_audio" -ToolArgs @{ name = "Track 1" } -Confirmed $true
    Assert-StatusOk -Response $track1 -Label "track.add_audio Track 1"
    $track1ID = [string](Get-OptionalProperty -Object $track1.result -Name "track_id")
    if ([string]::IsNullOrWhiteSpace($track1ID)) {
        $track1ID = [string](Get-OptionalProperty -Object $track1.result -Name "id")
    }
}
$track2 = Invoke-AgentTool -Tool "track.add_audio" -ToolArgs @{ name = "Track 2" } -Confirmed $true
Assert-StatusOk -Response $track2 -Label "track.add_audio Track 2"

$track2ID = [string](Get-OptionalProperty -Object $track2.result -Name "track_id")
if ([string]::IsNullOrWhiteSpace($track2ID)) {
    $track2ID = [string](Get-OptionalProperty -Object $track2.result -Name "id")
}
if ([string]::IsNullOrWhiteSpace($track1ID) -or [string]::IsNullOrWhiteSpace($track2ID)) {
    Fail ("Could not resolve fixture track IDs: track1=" + $track1ID + " track2=" + $track2ID)
}

$import1 = Import-AudioFixture -TrackID $track1ID -FilePath $Track1Path -Label "Track 1 import"
Assert-StatusOk -Response $import1 -Label "import Track 1 audio"

$import2 = Import-AudioFixture -TrackID $track2ID -FilePath $Track2Path -Label "Track 2 import"
Assert-StatusOk -Response $import2 -Label "import Track 2 audio"

Write-Ok ("fixture ready: Track 1=" + $track1ID + " Track 2=" + $track2ID)
Start-Sleep -Milliseconds 750

Write-Step "Run chat observe turn"
$conversationID = "mix_single_tick_e2e_" + (Get-Date -Format "yyyyMMdd_HHmmss")
$observeMessage = Join-UnicodeChars @(0x5E2E, 0x6211, 0x770B, 0x6574, 0x4F53, 0x6DF7, 0x97F3, 0xFF0C, 0x53EA, 0x5EFA, 0x8BAE, 0x4E00, 0x4E2A, 0x5C0F, 0x5E45, 0x97F3, 0x91CF, 0x8C03, 0x6574, 0xFF0C, 0x5148, 0x7B49, 0x6211, 0x786E, 0x8BA4, 0xFF0C, 0x4E0D, 0x8981, 0x7528, 0x63D2, 0x4EF6)
# Confirmation-request wording needle group (PORT-PS1-SYNC-2): the flash
# engine has been observed asking with plain "confirm" (0x786E 0x8BA4, as in
# "wait for my confirmation / please confirm / pending confirmation") without
# ever writing the execute/continue pair, so the semantic group is
# {execute (0x6267 0x884C), continue (0x7EE7 0x7EED), confirm (0x786E 0x8BA4)}.
$executeNeedle = Join-UnicodeChars @(0x6267, 0x884C)
$continueNeedle = Join-UnicodeChars @(0x7EE7, 0x7EED)
$confirmNeedle = Join-UnicodeChars @(0x786E, 0x8BA4)
$observe = Invoke-AgentChat -ConversationID $conversationID -Message $observeMessage
$observeStop = [string](Get-OptionalProperty -Object $observe -Name "stop_reason")
# Double-hop contract (PORT-PS1-SYNC-3, user ruling 2026-09-20): the observe
# turn parks at the improvement-PROPOSAL confirmation face (hop 1), never at a
# directly executable mix tick. stop_reason is the direct form
# (improvement_proposal_confirmation_required, workflow improvement_proposal)
# or the settle-mapped form (needs_confirmation from goal waiting_confirmation).
# The legacy "done" form belonged to the single-hop flow and is no longer a
# pass outcome; the classic "[mix.tick.pending] stored" wait moved to hop 1
# because the bounded tick is only stored once the proposal is confirmed.
Assert-InSet -Actual $observeStop -Expected @("needs_confirmation", "improvement_proposal_confirmation_required") -Label "observe turn stop_reason"
$observeReply = [string](Get-OptionalProperty -Object $observe -Name "reply")
$readOnlyDueToIncompleteL3 = $false
if (($observeReply -notmatch [regex]::Escape($executeNeedle)) -and ($observeReply -notmatch [regex]::Escape($continueNeedle)) -and ($observeReply -notmatch [regex]::Escape($confirmNeedle))) {
    if (($observeReply -match "L3|深度|spectrogram") -and ($observeReply -match "building|partial|未完整|未完成|不可靠|还在构建|正在构建")) {
        $readOnlyDueToIncompleteL3 = $true
        Write-Ok "observe stayed read-only while L3 acoustic package was incomplete"
    }
    else {
        Fail ("observe reply did not ask for execution confirmation: " + $observeReply)
    }
}
if (-not $readOnlyDueToIncompleteL3) {
    if (-not [bool](Get-OptionalProperty -Object $observe -Name "needs_confirmation")) {
        Fail "observe parked without needs_confirmation=true on the proposal face"
    }
    $observeFaceKinds = Get-ConfirmationFaceKinds -Response $observe
    if ($observeFaceKinds -notcontains "improvement_proposal_confirmation") {
        Fail ("observe confirmation face: got [" + ($observeFaceKinds -join ", ") + "], want improvement_proposal_confirmation (hop 1, proposal face)")
    }
    Write-Ok ("observe parked on the improvement-proposal face (hop 1): " + $observeStop)
}

Write-Step "Run unresolved vocal clarification guard"
$vocalConversationID = "mix_single_tick_vocal_clarify_" + (Get-Date -Format "yyyyMMdd_HHmmss")
$vocalMessage = Join-UnicodeChars @(0x8BA9, 0x4E3B, 0x5531, 0x66F4, 0x9760, 0x524D)
$vocalClarify = Invoke-AgentChat -ConversationID $vocalConversationID -Message $vocalMessage
$vocalStop = [string](Get-OptionalProperty -Object $vocalClarify -Name "stop_reason")
Assert-Equals -Actual $vocalStop -Expected "needs_clarification" -Label "unresolved vocal stop_reason"
$vocalReply = [string](Get-OptionalProperty -Object $vocalClarify -Name "reply")
$whichTrackText = Join-UnicodeChars @(0x54EA, 0x6761)
$whichOneMeasureText = Join-UnicodeChars @(0x54EA, 0x4E00, 0x6761)
$whichOneTrackText = Join-UnicodeChars @(0x54EA, 0x4E00, 0x8F68)
$whichTrackEnglish = "which track"
if (($vocalReply -notmatch $whichTrackText) -and ($vocalReply -notmatch $whichOneMeasureText) -and ($vocalReply -notmatch $whichOneTrackText) -and ($vocalReply.ToLowerInvariant() -notmatch $whichTrackEnglish)) {
    Fail ("unresolved vocal reply did not ask which track is vocal: " + $vocalReply)
}
$unexpectedVocalPending = Select-String -Path $AgentLog -Pattern ("[mix.tick.pending] stored conversation=" + $vocalConversationID) -SimpleMatch -ErrorAction SilentlyContinue | Select-Object -First 1
if ($null -ne $unexpectedVocalPending) {
    Fail ("unresolved vocal clarification stored pending unexpectedly: " + $unexpectedVocalPending.Line)
}
# Still valid under the double-hop workflow (PORT-PS1-SYNC-3): a clarification
# answer produces no candidate at all, so no mix tick may be stored for this
# conversation — the bounded tick only exists after a confirmed improvement
# proposal, which this unresolved vocal ask never reaches.
Write-Ok ("unresolved vocal asks clarification without pending: " + $vocalReply)

Write-Step "Run double-hop confirmation chain (proposal, then tool application)"
$confirmMessage = Join-UnicodeChars @(0x53EF, 0x4EE5, 0x6267, 0x884C)
$confirm2Stop = ""
$hop2Reply = ""
$confirm = Invoke-AgentChat -ConversationID $conversationID -Message $confirmMessage
$confirmStop = [string](Get-OptionalProperty -Object $confirm -Name "stop_reason")
if ($readOnlyDueToIncompleteL3) {
    Assert-Equals -Actual $confirmStop -Expected "no_pending_mix_tick_candidate" -Label "confirmation stop_reason"
    Write-Ok "confirmation correctly found no pending tick after incomplete L3 read-only observe"
}
else {
    # Hop 1 — the user confirms the improvement PROPOSAL. The agent then issues
    # the tool application (workflow mix_tick) and asks for its own execution
    # confirmation; nothing has been applied yet (product ruling 2026-09-20:
    # the two confirmations have different semantics, each on its own face).
    Assert-Equals -Actual $confirmStop -Expected "improvement_proposal_native_tool_confirmation_required" -Label "hop-1 confirmation stop_reason"
    Assert-Equals -Actual ([string](Get-OptionalProperty -Object $confirm -Name "workflow")) -Expected "mix_tick" -Label "hop-1 workflow"
    if (-not [bool](Get-OptionalProperty -Object $confirm -Name "needs_confirmation")) {
        Fail "hop-1 response did not carry needs_confirmation=true on the tool face"
    }
    $hop1FaceKinds = Get-ConfirmationFaceKinds -Response $confirm
    if ($hop1FaceKinds -notcontains "mix_tick_confirmation") {
        Fail ("hop-1 confirmation face: got [" + ($hop1FaceKinds -join ", ") + "], want mix_tick_confirmation (hop 2, tool face)")
    }
    $hop1TrackID = Assert-BoundedMixTickPayload -Response $confirm -Label "hop-1 pending tick"
    $storedLine = Wait-LogPattern -LogPath $AgentLog -Pattern ("[mix.tick.pending] stored conversation=" + $conversationID) -TimeoutSeconds 10
    Write-Ok ("hop-1 confirmed the proposal; bounded tick parked on the tool face (track " + $hop1TrackID + "): " + $storedLine)

    # Hop 2 — the user confirms the TOOL APPLICATION. The D1 chain applies the
    # bounded move, readback-verifies it, and parks the round at its
    # post-action evaluation boundary (workflow free_state_d1_s1). The legacy
    # single-hop anchors (mix_tick_applied_reobserved stop, the
    # propose/apply/reobserve tool route, and the
    # "[mix.tick.pending] applied and reobserved" log wait) belonged to the
    # pre-double-hop execution path and are gone by design: the D1 dispatch
    # returns before that log line is ever reached.
    $confirm2 = Invoke-AgentChat -ConversationID $conversationID -Message $confirmMessage
    $confirm2Stop = [string](Get-OptionalProperty -Object $confirm2 -Name "stop_reason")
    Assert-Equals -Actual $confirm2Stop -Expected "d1_post_action_evaluation_required" -Label "hop-2 confirmation stop_reason"
    Assert-Equals -Actual ([string](Get-OptionalProperty -Object $confirm2 -Name "workflow")) -Expected "free_state_d1_s1" -Label "hop-2 workflow"
    $hop2Data = Get-OptionalProperty -Object $confirm2 -Name "workflow_data"
    if ($null -eq $hop2Data) {
        Fail "hop-2 response carried no workflow_data"
    }
    if (([bool](Get-OptionalProperty -Object $hop2Data -Name "mutation_performed")) -ne $true) {
        Fail "hop-2 did not perform the mutation (workflow_data.mutation_performed != true)"
    }
    if (([bool](Get-OptionalProperty -Object $hop2Data -Name "readback_verified")) -ne $true) {
        Fail "hop-2 did not verify the applied value by readback (workflow_data.readback_verified != true)"
    }
    $hop2Reply = [string](Get-OptionalProperty -Object $confirm2 -Name "reply")
    # Raw CJK literal is safe: this file carries a UTF-8 BOM (PS1-SYNC-2).
    if (-not $hop2Reply.Contains("已应用并回读验证")) {
        Fail ("hop-2 reply did not report the applied+readback-verified outcome: " + $hop2Reply)
    }
    $routeLine = Wait-LogPattern -LogPath $AgentLog -Pattern ("[mix.tick.pending] explicit confirmation routed conversation=" + $conversationID) -TimeoutSeconds 10
    Write-Ok ("hop-2 confirmation routed: " + $routeLine)
    Write-Ok ("hop-2 applied and readback-verified, parked at the d1 terminal: " + $hop2Reply)
}

Write-Step "Summary"
Write-Host ("conversation: " + $conversationID)
Write-Host ("observe stop_reason: " + $observeStop + " (proposal face)")
Write-Host ("hop-1 stop_reason: " + $confirmStop + " (tool face)")
if (-not $readOnlyDueToIncompleteL3) {
    Write-Host ("hop-2 stop_reason: " + $confirm2Stop + " (workflow free_state_d1_s1)")
}
Write-Host ("observe reply: " + $observeReply)
Write-Host ("hop-1 reply: " + [string](Get-OptionalProperty -Object $confirm -Name "reply"))
if (-not $readOnlyDueToIncompleteL3) {
    Write-Host ("hop-2 reply: " + $hop2Reply)
}
Write-Ok "mix single tick E2E passed (double-hop: proposal confirmation -> tool application confirmation)"
