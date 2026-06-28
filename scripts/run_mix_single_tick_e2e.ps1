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
    [int]$ChatTimeoutSec = 240
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
    return Invoke-Json -Method POST -Uri ($AgentHttp.TrimEnd("/") + "/agent/chat") -Body @{
        conversation_id = $ConversationID
        message = $Message
        context = @{
            agent_mode = "chat"
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
$executeNeedle = Join-UnicodeChars @(0x6267, 0x884C)
$continueNeedle = Join-UnicodeChars @(0x7EE7, 0x7EED)
$observe = Invoke-AgentChat -ConversationID $conversationID -Message $observeMessage
$observeStop = [string](Get-OptionalProperty -Object $observe -Name "stop_reason")
Assert-Equals -Actual $observeStop -Expected "done" -Label "observe turn stop_reason"
$observeReply = [string](Get-OptionalProperty -Object $observe -Name "reply")
$readOnlyDueToIncompleteL3 = $false
if (($observeReply -notmatch [regex]::Escape($executeNeedle)) -and ($observeReply -notmatch [regex]::Escape($continueNeedle))) {
    if (($observeReply -match "L3|深度|spectrogram") -and ($observeReply -match "building|partial|未完整|未完成|不可靠|还在构建|正在构建")) {
        $readOnlyDueToIncompleteL3 = $true
        Write-Ok "observe stayed read-only while L3 acoustic package was incomplete"
    }
    else {
        Fail ("observe reply did not ask for execution confirmation: " + $observeReply)
    }
}
if (-not $readOnlyDueToIncompleteL3) {
    $storedLine = Wait-LogPattern -LogPath $AgentLog -Pattern ("[mix.tick.pending] stored conversation=" + $conversationID) -TimeoutSeconds 10
    if (($storedLine -notmatch [regex]::Escape("track=" + $track2ID)) -and
        ($storedLine -notmatch [regex]::Escape("track " + $track2ID)) -and
        ($storedLine -notmatch [regex]::Escape("target=track:" + $track2ID))) {
        Fail ("pending candidate did not target Track 2. line=" + $storedLine)
    }
    Write-Ok ("pending candidate stored: " + $storedLine)
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
Write-Ok ("unresolved vocal asks clarification without pending: " + $vocalReply)

Write-Step "Run confirmation turn"
$confirmMessage = Join-UnicodeChars @(0x53EF, 0x4EE5, 0x6267, 0x884C)
$confirm = Invoke-AgentChat -ConversationID $conversationID -Message $confirmMessage
$confirmStop = [string](Get-OptionalProperty -Object $confirm -Name "stop_reason")
if ($readOnlyDueToIncompleteL3) {
    Assert-Equals -Actual $confirmStop -Expected "no_pending_mix_tick_candidate" -Label "confirmation stop_reason"
    $tools = @()
    Write-Ok "confirmation correctly found no pending tick after incomplete L3 read-only observe"
}
else {
    Assert-Equals -Actual $confirmStop -Expected "mix_tick_applied_reobserved" -Label "confirmation stop_reason"
    $executed = Get-OptionalProperty -Object $confirm -Name "executed_kernel_reply"
    $tools = Tool-Names -Rows $executed
    Assert-AnyToolPresent -Tools $tools -Aliases @("mix.propose_tick", "mix_propose_tick") -Label "mix.propose_tick"
    Assert-AnyToolPresent -Tools $tools -Aliases @("mix.apply_tick", "mix_apply_tick") -Label "mix.apply_tick"
    Assert-AnyToolPresent -Tools $tools -Aliases @("mix.observe", "mix_observe", "mix.request_observation", "mix_request_observation") -Label "re-observation tool"
    Assert-AnyToolAbsent -Tools $tools -Aliases @("daw.invoke", "daw_invoke") -Label "daw.invoke"
    Assert-AnyToolAbsent -Tools $tools -Aliases @("track.volume", "track_volume") -Label "track.volume"

    $routeLine = Wait-LogPattern -LogPath $AgentLog -Pattern ("[mix.tick.pending] explicit confirmation routed conversation=" + $conversationID) -TimeoutSeconds 10
    $appliedLine = Wait-LogPattern -LogPath $AgentLog -Pattern ("[mix.tick.pending] applied and reobserved conversation=" + $conversationID) -TimeoutSeconds 10
    Write-Ok ("confirmation routed: " + $routeLine)
    Write-Ok ("applied and reobserved: " + $appliedLine)
}

Write-Step "Summary"
Write-Host ("conversation: " + $conversationID)
Write-Host ("route: " + ($tools -join " -> "))
Write-Host ("observe reply: " + $observeReply)
Write-Host ("confirm reply: " + [string](Get-OptionalProperty -Object $confirm -Name "reply"))
Write-Ok "mix single tick E2E passed"
