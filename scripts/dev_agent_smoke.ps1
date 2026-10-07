[CmdletBinding()]
param(
    [string]$RepoRoot = "",
    [string]$AgentHttp = "http://127.0.0.1:7878",
    [string]$AgentHttpAddr = "127.0.0.1:7878",
    [string]$ZmqReqPort = "5555",
    [string]$ZmqSubPort = "5556",
    [switch]$SkipBuild,
    [switch]$RestartAgent,
    [switch]$StartKernel,
    [string]$KernelExe = "",
    [switch]$StartUI,
    [string]$GodotProjectRoot = "D:\Godot\project\vit-daw-frontend",
    [string]$GodotExe = "",
    [switch]$UseExportedUI,
    [switch]$NoChatSmoke,
    [switch]$NoStripSilenceSmoke,
    [switch]$MixSmoke,
    [switch]$AuthoritySmoke,
    # L1-3-IMPL-C：查询引擎工具面只读场景（ref.query/ref.diff 经 /agent/invoke；
    # 全确定性断言：CommandSpec 广告 / 五元数据 / T10 反例 / T11 降级标注 /
    # ref.diff identity+content 边界）。
    [switch]$RefQuerySmoke,
    # VITNOTE-IMPL-4：工程级写租约并发场景（双 chat 各触发一次 B2 能力执行，
    # 双 approve 并发入栈——断言两执行段租约区间不重叠且后来者真实排队；
    # 证据=VIT_WRITE_LEASE_EVENTS_PATH JSONL 工件时间戳）。
    [switch]$WriteLeaseSmoke,
    [switch]$Strict,
    [int]$WaitSeconds = 20,
    # SMOKE-SCEN-RANGE-1 (2026-10-04): region-line re-verification scenarios,
    # each its own pass/fail group: "note_time" (note payload v3.1 time
    # dimension injection, asserted on the assembly/telemetry surfaces, never
    # the LLM reply text) and "range_split" (selected_clip_ranges boxed-range
    # split proposals: two clip.split, end cut first, plus the no-ranges
    # counter-case). "ref_diff_content" (L1-3-IMPL-D): ref.diff content-depth
    # delegation on real observation tickets. "midi_register"
    # (MIDI-CMD-REGISTER-1): kernel registration of import_midi_to_track +
    # apply_midi_note_patch, driven through the agent tool chain with a
    # self-generated SMF fixture and mixed-op patch read-back.
    # "render_freeze" (KERNEL-RENDER-FREEZE-FIX-1): render ABBA deadlock fix
    # reversal -- MIDI-only sync rejection (leg B), cancelled-render
    # render_failed telemetry + live command surface (leg A), idle/live
    # render.cancel, and an empty-range healthy render_done regression.
    # "all" runs
    # them. Scenario mode runs an isolated berth:
    # it refuses an already-listening stack (AGENTS.md section 9), starts the
    # kernel+agent itself, and tears both down when the scenarios finish.
    # "journey_first" (JOURNEY-1): demo journey gate leg 1-3 skeleton --
    # project open -> authority grant -> one deterministic B2 capability
    # experiment round (propose/approve/execute-verify). Zero LLM in the
    # gate; the NL free-state face is the J3 journey card (see
    # coord/runs/JOURNEY-1/JOURNEYS.md). Journeys are deliberately NOT part
    # of "all" (heavier execution legs, scheduled per journey).
    [string]$Scenario = "",
    # Scenario run artifacts root; defaults to
    # coord\runs\SMOKE-SCEN-RANGE-1\<timestamp> (journey_first:
    # coord\runs\JOURNEY-1\<timestamp>) under the repo.
    [string]$RunArtifactsDir = "",
    # SMOKE-TOOLING-1: the UI-launched kernel command port can take far longer
    # than the generic wait budget when the plugin table is cold (three
    # same-shape environment failures: ~994-entry cold load blew the fixed
    # 20s). Independent knob, default preserves the previous behaviour.
    [int]$UiPortWaitSeconds = 20
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

function Fail-Or-Warn {
    param([string]$Message)
    if ($Strict) {
        throw $Message
    }
    Write-WarnLine $Message
}

function Get-TcpListener {
    param([int]$Port)
    return Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue |
        Select-Object -First 1
}

function Wait-HttpReady {
    param(
        [string]$BaseUrl,
        [int]$TimeoutSeconds
    )
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    do {
        try {
            $resp = Invoke-WebRequest -UseBasicParsing -Uri ($BaseUrl.TrimEnd("/") + "/health") -TimeoutSec 2
            if ($resp.StatusCode -ge 200 -and $resp.StatusCode -lt 500) {
                return $true
            }
        }
        catch {
            Start-Sleep -Milliseconds 500
        }
    } while ((Get-Date) -lt $deadline)
    return $false
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
        $json = $Body | ConvertTo-Json -Depth 20 -Compress
        $bytes = [System.Text.Encoding]::UTF8.GetBytes($json)
        $resp = Invoke-WebRequest -UseBasicParsing -Method POST -Uri $Uri -Body $bytes -ContentType "application/json; charset=utf-8" -TimeoutSec $TimeoutSec
    }
    if ([string]::IsNullOrWhiteSpace($resp.Content)) {
        return $null
    }
    return $resp.Content | ConvertFrom-Json
}

# render_freeze (KERNEL-RENDER-FREEZE-FIX-1): /agent/invoke replies with a
# non-2xx status when a tool execution fails, but the JSON body still carries
# the failure semantics the poll asserts on (render.profile.bind distinguishes
# "no telemetry cached" from a terminal failed status). Windows PowerShell
# 5.1 drains the error response stream before user catch blocks can read it
# (both Response.GetResponseStream and ErrorDetails come back empty), so this
# helper goes through System.Net.Http.HttpClient, which never throws on
# non-2xx and hands the body back for parsing.
function Invoke-JsonTolerant {
    param(
        [string]$Uri,
        [object]$Body = $null,
        [int]$TimeoutSec = 30
    )
    Add-Type -AssemblyName System.Net.Http | Out-Null
    $client = New-Object System.Net.Http.HttpClient
    $client.Timeout = [TimeSpan]::FromSeconds($TimeoutSec)
    try {
        $json = $Body | ConvertTo-Json -Depth 20 -Compress
        $content = New-Object System.Net.Http.StringContent ($json, [System.Text.Encoding]::UTF8, "application/json")
        $post = $client.PostAsync($Uri, $content).GetAwaiter().GetResult()
        $text = $post.Content.ReadAsStringAsync().GetAwaiter().GetResult()
        if ([string]::IsNullOrWhiteSpace($text)) {
            return $null
        }
        return $text | ConvertFrom-Json
    }
    finally {
        $client.Dispose()
    }
}

function Get-OptionalProperty {
    param(
        [object]$Object,
        [string]$Name
    )
    if ($null -eq $Object -or [string]::IsNullOrWhiteSpace($Name)) {
        return ""
    }
    $prop = $Object.PSObject.Properties[$Name]
    if ($null -eq $prop) {
        return ""
    }
    return $prop.Value
}

function Get-FirstPropertyValue {
    param(
        [object]$Object,
        [string[]]$Names
    )
    foreach ($name in $Names) {
        $value = Get-OptionalProperty -Object $Object -Name $name
        if ($null -ne $value -and -not [string]::IsNullOrWhiteSpace([string]$value)) {
            return $value
        }
    }
    return ""
}

function Get-FirstVisibleTrackId {
    param(
        [object]$ProjectState,
        [object]$AgentState
    )
    $trackSets = @()
    if ($null -ne $ProjectState) {
        $result = Get-OptionalProperty -Object $ProjectState -Name "result"
        if ($null -ne $result) {
            $trackSets += ,@(Get-OptionalProperty -Object $result -Name "tracks")
            $project = Get-OptionalProperty -Object $result -Name "project"
            if ($null -ne $project) {
                $trackSets += ,@(Get-OptionalProperty -Object $project -Name "tracks")
            }
        }
    }
    if ($null -ne $AgentState) {
        $shadow = Get-OptionalProperty -Object $AgentState -Name "shadow"
        if ($null -ne $shadow) {
            $trackSets += ,@(Get-OptionalProperty -Object $shadow -Name "tracks")
        }
    }
    foreach ($tracks in $trackSets) {
        foreach ($track in @($tracks)) {
            $trackID = [string](Get-FirstPropertyValue -Object $track -Names @("track_id", "id", "item_id"))
            if (-not [string]::IsNullOrWhiteSpace($trackID)) {
                return $trackID
            }
        }
    }
    return ""
}

function Get-ImportedClipId {
    param(
        [object]$ImportResponse,
        [object]$ProjectState,
        [string]$FilePath
    )
    $result = Get-OptionalProperty -Object $ImportResponse -Name "result"
    $direct = [string](Get-FirstPropertyValue -Object $result -Names @("clip_id", "new_clip_id", "last_created_clip_id", "id", "item_id"))
    if (-not [string]::IsNullOrWhiteSpace($direct)) {
        return $direct
    }
    if ($null -eq $ProjectState) {
        return ""
    }
    $stateResult = Get-OptionalProperty -Object $ProjectState -Name "result"
    $clips = Get-OptionalProperty -Object $stateResult -Name "clips"
    if ($null -eq $clips) {
        return ""
    }
    $target = [System.IO.Path]::GetFullPath($FilePath).ToLowerInvariant()
    $clipRows = @()
    if ($clips -is [System.Array]) {
        $clipRows = @($clips)
    }
    else {
        foreach ($prop in $clips.PSObject.Properties) {
            $row = $prop.Value
            if ($null -ne $row) {
                if ([string]::IsNullOrWhiteSpace([string](Get-OptionalProperty -Object $row -Name "clip_id"))) {
                    try {
                        $row | Add-Member -NotePropertyName "clip_id" -NotePropertyValue ([string]$prop.Name) -Force
                    }
                    catch {
                    }
                }
                $clipRows += $row
            }
        }
    }
    foreach ($clip in $clipRows) {
        $path = [string](Get-FirstPropertyValue -Object $clip -Names @("file_path", "current_source_path", "source_path", "media_path"))
        if (-not [string]::IsNullOrWhiteSpace($path)) {
            try {
                if ([System.IO.Path]::GetFullPath($path).ToLowerInvariant() -eq $target) {
                    $cid = [string](Get-FirstPropertyValue -Object $clip -Names @("clip_id", "id", "item_id"))
                    return $cid
                }
            }
            catch {
            }
        }
    }
    return ""
}

function Write-StripSilenceTestWav {
    param([string]$Path)
    $dir = Split-Path -Parent $Path
    New-Item -ItemType Directory -Path $dir -Force | Out-Null
    $sampleRate = 44100
    $durationSeconds = 2.2
    $samples = [int]($sampleRate * $durationSeconds)
    $channels = 1
    $bitsPerSample = 16
    $blockAlign = [int]($channels * $bitsPerSample / 8)
    $byteRate = [int]($sampleRate * $blockAlign)
    $dataBytes = [int]($samples * $blockAlign)
    $writer = [System.IO.BinaryWriter]::new([System.IO.File]::Open($Path, [System.IO.FileMode]::Create, [System.IO.FileAccess]::Write))
    try {
        $ascii = [System.Text.Encoding]::ASCII
        $writer.Write($ascii.GetBytes("RIFF"))
        $writer.Write([int](36 + $dataBytes))
        $writer.Write($ascii.GetBytes("WAVE"))
        $writer.Write($ascii.GetBytes("fmt "))
        $writer.Write([int]16)
        $writer.Write([int16]1)
        $writer.Write([int16]$channels)
        $writer.Write([int]$sampleRate)
        $writer.Write([int]$byteRate)
        $writer.Write([int16]$blockAlign)
        $writer.Write([int16]$bitsPerSample)
        $writer.Write($ascii.GetBytes("data"))
        $writer.Write([int]$dataBytes)
        for ($i = 0; $i -lt $samples; $i++) {
            $t = [double]$i / [double]$sampleRate
            $amp = 0.0
            if (($t -ge 0.35 -and $t -lt 0.70) -or ($t -ge 1.25 -and $t -lt 1.55)) {
                $amp = 0.45 * [Math]::Sin(2.0 * [Math]::PI * 440.0 * $t)
            }
            $writer.Write([int16]([Math]::Round($amp * 32767.0)))
        }
    }
    finally {
        $writer.Close()
    }
    return $Path
}

# VITNOTE-IMPL-4: deterministic stem for the write-lease smoke fixture. Two
# stems with deliberately different levels/frequencies give the B2 shadow
# planner a real static-balance problem (non-empty ActionSet) and DAD
# analysable signal. Same WAV shape as Write-StripSilenceTestWav.
function Write-LeaseSmokeStemWav {
    param([string]$Path, [double]$Amplitude, [double]$Frequency)
    $dir = Split-Path -Parent $Path
    New-Item -ItemType Directory -Path $dir -Force | Out-Null
    $sampleRate = 44100
    $durationSeconds = 2.2
    $samples = [int]($sampleRate * $durationSeconds)
    $channels = 1
    $bitsPerSample = 16
    $blockAlign = [int]($channels * $bitsPerSample / 8)
    $byteRate = [int]($sampleRate * $blockAlign)
    $dataBytes = [int]($samples * $blockAlign)
    $writer = [System.IO.BinaryWriter]::new([System.IO.File]::Open($Path, [System.IO.FileMode]::Create, [System.IO.FileAccess]::Write))
    try {
        $ascii = [System.Text.Encoding]::ASCII
        $writer.Write($ascii.GetBytes("RIFF"))
        $writer.Write([int](36 + $dataBytes))
        $writer.Write($ascii.GetBytes("WAVE"))
        $writer.Write($ascii.GetBytes("fmt "))
        $writer.Write([int]16)
        $writer.Write([int16]1)
        $writer.Write([int16]$channels)
        $writer.Write([int]$sampleRate)
        $writer.Write([int]$byteRate)
        $writer.Write([int16]$blockAlign)
        $writer.Write([int16]$bitsPerSample)
        $writer.Write($ascii.GetBytes("data"))
        $writer.Write([int]$dataBytes)
        for ($i = 0; $i -lt $samples; $i++) {
            $t = [double]$i / [double]$sampleRate
            $amp = 0.0
            # Continuous tone (unlike the strip-silence bursts): DAD features
            # need sustained signal across the whole stem.
            $amp = $Amplitude * [Math]::Sin(2.0 * [Math]::PI * $Frequency * $t)
            $writer.Write([int16]([Math]::Round($amp * 32767.0)))
        }
    }
    finally {
        $writer.Close()
    }
    return $Path
}

function Count-MixPlannerChatEvents {
    param([string]$LogPath)
    if (-not (Test-Path -LiteralPath $LogPath)) {
        return 0
    }
    return (Select-String -Path $LogPath -Pattern '"event":"mix_planner_chat"' | Measure-Object).Count
}

function Get-AgentProcessesForPaths {
    param([string[]]$AgentPaths)
    $targets = @{}
    foreach ($agentPath in $AgentPaths) {
        if ([string]::IsNullOrWhiteSpace($agentPath)) {
            continue
        }
        $targets[[System.IO.Path]::GetFullPath($agentPath).ToLowerInvariant()] = $true
    }
    return Get-Process -ErrorAction SilentlyContinue | Where-Object {
        $_.ProcessName -like "*VitAgent*" -or $_.ProcessName -like "*vitagent*"
    } | Where-Object {
        try {
            if ([string]::IsNullOrWhiteSpace($_.Path)) {
                return $false
            }
            $path = [System.IO.Path]::GetFullPath($_.Path).ToLowerInvariant()
            return $targets.ContainsKey($path)
        }
        catch {
            $false
        }
    }
}

function Resolve-GodotExecutable {
    param([string]$RequestedGodotExe)
    $candidates = @()
    if (-not [string]::IsNullOrWhiteSpace($RequestedGodotExe)) {
        $candidates += $RequestedGodotExe.Trim().Trim('"')
    }
    if (-not [string]::IsNullOrWhiteSpace($env:GODOT)) {
        $candidates += $env:GODOT.Trim().Trim('"')
    }
    $candidates += @(
        "D:\Godot\Godot_v4.6.1-stable_win64_console.exe",
        "D:\Godot\Godot_v4.6.1-stable_win64.exe",
        "${env:ProgramFiles}\Godot\Godot.exe",
        "${env:LocalAppData}\Programs\Godot\Godot.exe"
    )
    foreach ($candidate in $candidates) {
        if (-not [string]::IsNullOrWhiteSpace($candidate) -and (Test-Path -LiteralPath $candidate -PathType Leaf)) {
            return (Resolve-Path -LiteralPath $candidate).Path
        }
    }
    return ""
}

function Resolve-GodotProjectRoot {
    param([string]$RequestedProjectRoot)
    if ([string]::IsNullOrWhiteSpace($RequestedProjectRoot)) {
        return ""
    }
    $candidate = $RequestedProjectRoot.Trim().Trim('"')
    if (Test-Path -LiteralPath (Join-Path $candidate "project.godot") -PathType Leaf) {
        return (Resolve-Path -LiteralPath $candidate).Path
    }
    return ""
}

function Stop-AgentForPaths {
    param(
        [string[]]$AgentPaths,
        [int]$TimeoutSeconds = 20
    )
    $procs = Get-AgentProcessesForPaths -AgentPaths $AgentPaths
    foreach ($proc in $procs) {
        Write-WarnLine ("stopping existing agent pid=" + $proc.Id)
        Stop-Process -Id $proc.Id -Force
    }
    foreach ($proc in $procs) {
        try {
            Wait-Process -Id $proc.Id -Timeout ([Math]::Max(5, $TimeoutSeconds)) -ErrorAction Stop
        }
        catch {
            if ($null -ne (Get-Process -Id $proc.Id -ErrorAction SilentlyContinue)) {
                throw ("agent process did not exit within timeout; pid=" + $proc.Id)
            }
        }
    }
}

function Copy-AgentBinary {
    param(
        [string]$Source,
        [string]$Destination,
        [string]$Label
    )
    $running = @(Get-AgentProcessesForPaths -AgentPaths @($Destination))
    if ($running.Count -gt 0) {
        Fail-Or-Warn ($Label + " is currently running; not overwriting " + $Destination)
        return
    }
    Copy-Item -LiteralPath $Source -Destination $Destination -Force
    Write-Ok ("installed " + $Label + ": " + $Destination)
}

$RepoRoot = Resolve-RepoRoot -Explicit $RepoRoot
$AgentDir = Join-Path $RepoRoot "agent"
$ScriptsDir = Join-Path $RepoRoot "scripts"
$WorkspaceDir = Join-Path $RepoRoot "VitApp\Workspace"
$LogsDir = Join-Path $WorkspaceDir "Logs"
$AgentExe = Join-Path $AgentDir "bin\VitAgent.exe"
$RootAgentExe = Join-Path $AgentDir "vitagent.exe"
$SmokeBuildExe = Join-Path $AgentDir "bin\VitAgent.dev-smoke.exe"
if ([string]::IsNullOrWhiteSpace($KernelExe)) {
    $KernelExe = Join-Path $RepoRoot "Export\staging\runtime\VitApp.exe"
}
else {
    $KernelExe = (Resolve-Path -LiteralPath $KernelExe).Path
}
$UiExe = Join-Path $RepoRoot "Export\staging\Vit_DAW.exe"
$ResolvedGodotProjectRoot = Resolve-GodotProjectRoot -RequestedProjectRoot $GodotProjectRoot
$ResolvedGodotExe = Resolve-GodotExecutable -RequestedGodotExe $GodotExe
$MixDiagLog = Join-Path $LogsDir "mixboard_interaction_diag.jsonl"
$AgentLog = Join-Path $LogsDir "agent_last.log"
$AgentPaths = @($AgentExe, $RootAgentExe, $SmokeBuildExe)
$httpPort = [int](($AgentHttpAddr -split ":")[-1])
$listener = Get-TcpListener -Port $httpPort

# SMOKE-SCEN-RANGE-1 berth preflight: scenario mode owns its own stack. It
# refuses to reuse or disturb anything already listening (AGENTS.md section 9:
# one stack owner at a time), then starts kernel+agent below and tears them
# down after the selected scenarios finish.
$ScenarioMode = -not [string]::IsNullOrWhiteSpace($Scenario)
$ScenarioKernelProcId = $null
$ScenarioAgentProcId = $null
$ScenarioRunDir = ""
if ($ScenarioMode) {
    if ($Scenario -notin @("note_time", "range_split", "ref_diff_content", "midi_register", "render_freeze", "journey_first", "all")) {
        throw ("unknown -Scenario value '" + $Scenario + "'; expected note_time, range_split, ref_diff_content, midi_register, render_freeze, journey_first, or all")
    }
    if ($StartUI) {
        throw "-Scenario berth mode never starts the Godot UI (card constraint: webui/Godot untouched)"
    }
    if ($Scenario -eq "journey_first" -and -not $StartKernel) {
        throw "-Scenario journey_first owns the full stack and must be run with -StartKernel (the kernel is part of the journey berth)"
    }
    if ($null -ne $listener) {
        throw ("scenario berth requires a free agent HTTP port; " + $httpPort + " is already listening (another session's stack, AGENTS.md section 9)")
    }
    if (Get-TcpListener -Port ([int]$ZmqReqPort)) {
        throw ("scenario berth requires a free kernel command port; " + $ZmqReqPort + " is already listening (kernel ports are fixed)")
    }
    if ([string]::IsNullOrWhiteSpace($RunArtifactsDir)) {
        $scenarioRunsRoot = "coord\runs\SMOKE-SCEN-RANGE-1"
        if ($Scenario -eq "journey_first") {
            $scenarioRunsRoot = "coord\runs\JOURNEY-1"
        }
        $RunArtifactsDir = Join-Path $RepoRoot ($scenarioRunsRoot + "\" + (Get-Date -Format "yyyyMMdd_HHmmss"))
    }
    if (Test-Path -LiteralPath $RunArtifactsDir) {
        throw ("scenario run artifacts directory already exists; each run needs a fresh directory: " + $RunArtifactsDir)
    }
    New-Item -ItemType Directory -Path $RunArtifactsDir -Force | Out-Null
    $ScenarioRunDir = $RunArtifactsDir
    (& git -C $RepoRoot rev-parse HEAD) | Set-Content -LiteralPath (Join-Path $ScenarioRunDir "head.txt") -Encoding UTF8
    (& git -C $RepoRoot status --short) | Set-Content -LiteralPath (Join-Path $ScenarioRunDir "git_status.txt") -Encoding UTF8
    if ($Scenario -eq "journey_first") {
        # JOURNEY-1 berth isolation (journey1 contract): the kernel runs in a
        # run-dir workspace with its default project redirected there, so
        # nothing under VitApp\Workspace is written by this run even though the
        # journey approves real executions; the agent's conversation draft
        # root is redirected into the run dir too. A caller-provided
        # VIT_HISTORY_DRAFT_ROOT wins (same precedence as the COM roots).
        $JourneyKernelWorkspace = Join-Path $ScenarioRunDir "kernel_workspace"
        $JourneyProjectDir = Join-Path $ScenarioRunDir "project"
        $JourneyStemsDir = Join-Path $ScenarioRunDir "fixture_stems"
        $JourneyAgentDrafts = Join-Path $ScenarioRunDir "agent_drafts"
        foreach ($journeyDir in @(
            $JourneyKernelWorkspace,
            (Join-Path $JourneyKernelWorkspace "Settings"),
            (Join-Path $JourneyKernelWorkspace "Logs"),
            $JourneyProjectDir,
            $JourneyStemsDir,
            $JourneyAgentDrafts
        )) {
            New-Item -ItemType Directory -Path $journeyDir -Force | Out-Null
        }
        Copy-Item -LiteralPath (Join-Path $RepoRoot "VitApp\Workspace\default_project.xml") -Destination (Join-Path $JourneyKernelWorkspace "default_project.xml") -Force
        if ([string]::IsNullOrWhiteSpace([System.Environment]::GetEnvironmentVariable("VIT_HISTORY_DRAFT_ROOT"))) {
            Set-Item -LiteralPath "env:VIT_HISTORY_DRAFT_ROOT" -Value $JourneyAgentDrafts
        }
    }
}

Write-Step "VitAgent dev smoke"
Write-Host ("repo: " + $RepoRoot)
Write-Host ("agent http: " + $AgentHttp)

if (-not $SkipBuild) {
    Write-Step "Build VitAgent"
    New-Item -ItemType Directory -Path (Split-Path -Parent $SmokeBuildExe) -Force | Out-Null
    Push-Location $AgentDir
    try {
        & go build -o $SmokeBuildExe .\cmd\vitagent
        if ($LASTEXITCODE -ne 0) {
            throw "go build failed with exit code $LASTEXITCODE"
        }
        Write-Ok ("build passed: " + $SmokeBuildExe)
    }
    finally {
        Pop-Location
    }

    if ($RestartAgent) {
        Stop-AgentForPaths -AgentPaths $AgentPaths -TimeoutSeconds $WaitSeconds
        $listener = Get-TcpListener -Port $httpPort
        if ($null -ne $listener) {
            Fail-Or-Warn ("port " + $httpPort + " is still listening after restart request; pid=" + $listener.OwningProcess)
        }
    }

    if ($RestartAgent -or $null -eq $listener) {
        Copy-AgentBinary -Source $SmokeBuildExe -Destination $AgentExe -Label "agent bin"
        Copy-AgentBinary -Source $SmokeBuildExe -Destination $RootAgentExe -Label "agent root compatibility exe"
    }
    else {
        Write-WarnLine "existing agent left running; use -RestartAgent to install and run the new build"
    }
}
elseif ($null -eq $listener -and -not (Test-Path -LiteralPath $AgentExe)) {
    throw "Missing agent binary: $AgentExe. Run without -SkipBuild first."
}
elseif ($SkipBuild -and $RestartAgent) {
    Write-Step "Restart existing VitAgent"
    Stop-AgentForPaths -AgentPaths $AgentPaths -TimeoutSeconds $WaitSeconds
    $listener = Get-TcpListener -Port $httpPort
    if ($null -ne $listener) {
        Fail-Or-Warn ("port " + $httpPort + " is still listening after restart request; pid=" + $listener.OwningProcess)
    }
}

if ($StartKernel) {
    Write-Step "Start kernel"
    if (-not (Test-Path -LiteralPath $KernelExe)) {
        Fail-Or-Warn ("kernel exe not found: " + $KernelExe)
    }
    elseif (-not (Get-TcpListener -Port ([int]$ZmqReqPort))) {
        # JOURNEY-1: the journey berth kernel runs inside the run-dir workspace
        # with VIT_PROJECT_XML redirected to the run-dir default-project copy
        # (journey1 isolation contract); every other scenario keeps the
        # deployed-kernel working directory.
        $journeyKernelStart = ($ScenarioMode -and $Scenario -eq "journey_first")
        $kernelWorkingDir = Split-Path -Parent $KernelExe
        if ($journeyKernelStart) {
            $kernelWorkingDir = $JourneyKernelWorkspace
        }
        $startedKernelProc = $null
        if ($journeyKernelStart) {
            $priorProjectXml = [System.Environment]::GetEnvironmentVariable("VIT_PROJECT_XML", "Process")
            $env:VIT_PROJECT_XML = (Join-Path $JourneyKernelWorkspace "default_project.xml")
            try {
                $startedKernelProc = Start-Process -FilePath $KernelExe -WorkingDirectory $kernelWorkingDir -WindowStyle Hidden -PassThru
            }
            finally {
                if ([string]::IsNullOrWhiteSpace($priorProjectXml)) {
                    Remove-Item Env:VIT_PROJECT_XML -ErrorAction SilentlyContinue
                }
                else {
                    $env:VIT_PROJECT_XML = $priorProjectXml
                }
            }
        }
        else {
            $startedKernelProc = Start-Process -FilePath $KernelExe -WorkingDirectory $kernelWorkingDir -WindowStyle Hidden -PassThru
        }
        if ($null -ne $startedKernelProc) {
            $ScenarioKernelProcId = $startedKernelProc.Id
        }
        Write-Ok ("started kernel: " + $KernelExe + " pid=" + $ScenarioKernelProcId + " workdir=" + $kernelWorkingDir)
        Start-Sleep -Seconds 2
    }
    else {
        Write-Ok ("kernel command port already listening: " + $ZmqReqPort)
    }
}

if ($StartUI) {
    Write-Step "Start UI"
    if (Get-TcpListener -Port ([int]$ZmqReqPort)) {
        $existingReq = Get-TcpListener -Port ([int]$ZmqReqPort)
        Write-Ok ("kernel command port already listening before UI start: " + $ZmqReqPort + " pid=" + $existingReq.OwningProcess)
    }
    elseif (-not $UseExportedUI -and -not [string]::IsNullOrWhiteSpace($ResolvedGodotProjectRoot) -and -not [string]::IsNullOrWhiteSpace($ResolvedGodotExe)) {
        $godotLog = Join-Path $LogsDir "godot_dev_agent_smoke.log"
        New-Item -ItemType Directory -Path $LogsDir -Force | Out-Null
        $godotArgs = @("--path", $ResolvedGodotProjectRoot, "--log-file", $godotLog)
        # D1 smoke owns Kernel/agent startup.  Prevent the Godot start page from
        # spawning a competing dev runtime (and taking ownership of its lifetime).
        # Set this only on the Godot child; do not leak it into the agent or the
        # caller's PowerShell environment.
        $priorSkipDevAutostart = [Environment]::GetEnvironmentVariable("VIT_SKIP_DEV_AUTOSTART", "Process")
        $env:VIT_SKIP_DEV_AUTOSTART = "1"
        try {
            Start-Process -FilePath $ResolvedGodotExe -ArgumentList $godotArgs -WorkingDirectory $ResolvedGodotProjectRoot | Out-Null
        }
        finally {
            if ($null -eq $priorSkipDevAutostart) {
                Remove-Item Env:VIT_SKIP_DEV_AUTOSTART -ErrorAction SilentlyContinue
            }
            else {
                $env:VIT_SKIP_DEV_AUTOSTART = $priorSkipDevAutostart
            }
        }
        Write-Ok ("started Godot project UI: " + $ResolvedGodotExe + " --path " + $ResolvedGodotProjectRoot)
        Write-Host ("godot log: " + $godotLog)
    }
    elseif (-not (Test-Path -LiteralPath $UiExe)) {
        Fail-Or-Warn ("UI exe not found: " + $UiExe)
    }
    else {
        Start-Process -FilePath $UiExe -WorkingDirectory (Split-Path -Parent $UiExe) | Out-Null
        Write-Ok ("started UI: " + $UiExe)
    }
    if ($UseExportedUI -or (-not [string]::IsNullOrWhiteSpace($ResolvedGodotProjectRoot)) -or (Test-Path -LiteralPath $UiExe) -or (Get-TcpListener -Port ([int]$ZmqReqPort))) {
        $uiKernelDeadline = (Get-Date).AddSeconds($UiPortWaitSeconds)
        while ((Get-Date) -lt $uiKernelDeadline -and -not (Get-TcpListener -Port ([int]$ZmqReqPort))) {
            Start-Sleep -Milliseconds 500
        }
        if (Get-TcpListener -Port ([int]$ZmqReqPort)) {
            Write-Ok ("UI-launched kernel command port is listening: " + $ZmqReqPort)
        }
        else {
            Fail-Or-Warn ("UI did not expose kernel command port within " + [string]$UiPortWaitSeconds + "s")
        }
    }
}

Write-Step "Start or reuse agent"
# MAT-1 (2026-09-07): the kernel writes COM dual-tap evidence under
# <VitAppRoot>\Workspace\Artifacts\com_evidence\<pair>\ (VitPaths.h climbs to
# the VitApp root), while the agent locates and allowlists that artifact only
# through VIT_DAW_DEV_ROOT / VIT_DEV_ROOT / VIT_ROOT (harness
# com_observation.go comEvidenceArtifactPath + mixboard
# com_projection.go comArtifactPathAllowed). Without them a healthy paired
# probe still fails with "COM evidence workspace root is unavailable" and the
# semantic workflow falls back to source_only. Supply the repo root for the
# launched agent unless the caller already chose a root.
foreach ($comRootVariable in @("VIT_DAW_DEV_ROOT", "VIT_DEV_ROOT", "VIT_ROOT")) {
    if ([string]::IsNullOrWhiteSpace([System.Environment]::GetEnvironmentVariable($comRootVariable))) {
        Set-Item -LiteralPath ("env:" + $comRootVariable) -Value $RepoRoot
    }
}
# VITNOTE-IMPL-4: per-project write lease transitions append to this JSONL
# file (executionruntime writelease.go). Must be set before agent start so the
# script-started agent inherits it; a reused agent will not have it and the
# -WriteLeaseSmoke scenario reports that explicitly. A caller-provided path
# wins, matching the comRoot pattern above.
$WriteLeaseEventsFile = [System.Environment]::GetEnvironmentVariable("VIT_WRITE_LEASE_EVENTS_PATH")
if ([string]::IsNullOrWhiteSpace($WriteLeaseEventsFile)) {
    $WriteLeaseEventsFile = Join-Path $LogsDir ("write_lease_events_" + (Get-Date -Format "yyyyMMdd_HHmmss") + ".jsonl")
    Set-Item -LiteralPath "env:VIT_WRITE_LEASE_EVENTS_PATH" -Value $WriteLeaseEventsFile
}
# SMOKE-SCEN-RANGE-1: scenario note_time reads the LLM telemetry JSONL
# (section_stats char_count) as the deterministic note-assembly surface. Must
# be set before agent start so the berth agent inherits it; like the lease
# path above, a caller-provided value wins.
$ScenarioTelemetryFile = [System.Environment]::GetEnvironmentVariable("VIT_AGENT_LLM_TELEMETRY_PATH")
if ($ScenarioMode -and ($Scenario -eq "note_time" -or $Scenario -eq "all")) {
    if ([string]::IsNullOrWhiteSpace($ScenarioTelemetryFile)) {
        $ScenarioTelemetryFile = Join-Path $ScenarioRunDir "agent_llm_telemetry.jsonl"
        Set-Item -LiteralPath "env:VIT_AGENT_LLM_TELEMETRY_PATH" -Value $ScenarioTelemetryFile
    }
}
if ($RestartAgent) {
    $listener = Get-TcpListener -Port $httpPort
}

if ($null -eq $listener) {
    New-Item -ItemType Directory -Path $LogsDir -Force | Out-Null
    $args = @(
        "-http", $AgentHttpAddr,
        "-last-log-path", $AgentLog,
        "-keep-last-log-lines", "800"
    )
    Start-Process -FilePath $AgentExe -ArgumentList $args -WorkingDirectory $AgentDir -WindowStyle Hidden | Out-Null
    if (-not (Wait-HttpReady -BaseUrl $AgentHttp -TimeoutSeconds $WaitSeconds)) {
        throw "VitAgent HTTP did not become ready at $AgentHttp"
    }
    if ($ScenarioMode) {
        $scenarioAgentListener = Get-TcpListener -Port $httpPort
        if ($null -ne $scenarioAgentListener) {
            $ScenarioAgentProcId = $scenarioAgentListener.OwningProcess
        }
    }
    Write-Ok ("started agent: " + $AgentExe)
}
else {
    Write-Ok ("agent HTTP port already listening: " + $httpPort + " pid=" + $listener.OwningProcess)
    if (-not (Wait-HttpReady -BaseUrl $AgentHttp -TimeoutSeconds 3)) {
        Fail-Or-Warn ("port " + $httpPort + " is listening but /health did not respond")
    }
}

Write-Step "HTTP state"
$state = Invoke-Json -Method GET -Uri ($AgentHttp.TrimEnd("/") + "/agent/state") -TimeoutSec 10
if ($null -eq $state -or $state.status -ne "ok") {
    throw "GET /agent/state did not return ok"
}
Write-Ok ("tool_count=" + [string]$state.tool_count)
Write-Host ("shadow.initialized=" + [string]$state.shadow.initialized + " track_count=" + [string]$state.shadow.track_count)

# ============================================================
# SMOKE-SCEN-RANGE-1 scenario groups (REGION-TIME-1 / INTENT-WIRE-1 gates).
# Scenario mode replaces the default smoke flow: it runs only the selected
# groups on its own berth and tears the stack down afterwards. Every
# assertion is deterministic per AGENTS.md section 8: scenario A asserts the
# assembly/telemetry surfaces (never the LLM reply text); scenario B asserts
# the proposal surface of the local intent layer, with model-envelope
# preemption pre-declared as the only retryable failure class (3 attempts).
# ============================================================
if ($ScenarioMode) {
    $chatUri = $AgentHttp.TrimEnd("/") + "/agent/chat"
    $scenarioPassed = $false
    try {
        # Berth readiness: the agent binds a project identity shortly after
        # start (shadow project_uuid; project_path may legitimately stay empty
        # for an untitled kernel project — the identity then resolves to the
        # process-local default draft, which is stable for the agent's
        # lifetime). Note persistence and the note_sessions projection resolve
        # the same identity, so wait until the shadow reports a stable
        # non-empty project_uuid (two consecutive identical readings).
        $identityDeadline = (Get-Date).AddSeconds(45)
        $priorIdentityUUID = ""
        $stableIdentityUUID = ""
        while ((Get-Date) -lt $identityDeadline) {
            $statePoll = Invoke-Json -Method GET -Uri ($AgentHttp.TrimEnd("/") + "/agent/state") -TimeoutSec 10
            $identityUUID = [string](Get-OptionalProperty -Object (Get-OptionalProperty -Object $statePoll -Name "shadow") -Name "project_uuid")
            if (-not [string]::IsNullOrWhiteSpace($identityUUID) -and $identityUUID -eq $priorIdentityUUID) {
                $stableIdentityUUID = $identityUUID
                break
            }
            $priorIdentityUUID = $identityUUID
            Start-Sleep -Milliseconds 800
        }
        if ([string]::IsNullOrWhiteSpace($stableIdentityUUID)) {
            throw ("scenario berth never reported a stable active project uuid (last=" + $priorIdentityUUID + "); note-session assertions would race the agent's project recovery")
        }
        Write-Ok ("scenario berth project identity stable: " + $stableIdentityUUID)

        if ($Scenario -eq "note_time" -or $Scenario -eq "all") {
            Write-Step "Scenario note_time: note payload v3.1 time dimension injection"
            if ([string]::IsNullOrWhiteSpace($ScenarioTelemetryFile)) {
                throw "scenario note_time requires VIT_AGENT_LLM_TELEMETRY_PATH; the agent must be started by this run"
            }
        $noteStamp = Get-Date -Format "yyyyMMdd_HHmmss"
        $noteQuestion = "What does this circled range contain? Include the time span."
        # v3.1 payload: timeline face with domain.range_time_span plus per-entry
        # range_clip_start/end (same shapes as the implementation unit fixtures),
        # plus the top-level range_time_span the v3.1 panel lifts.
        $noteFacesV31 = @(
            @{
                face_kind = "timeline"
                label = "timeline"
                selection_share = 0.72
                domain = @{
                    range_time_span = @{ start_s = 3.2; end_s = 8.5 }
                    entries = @(
                        @{ clip_id = "smoke_clip_full"; track_id = "smoke_track_1"; start_seconds = 3.2; end_seconds = 7.8; range_clip_start = 3.2; range_clip_end = 7.8 },
                        @{ clip_id = "smoke_clip_part"; track_id = "smoke_track_1"; start_seconds = 5.0; end_seconds = 12.0; range_clip_start = 5.0; range_clip_end = 8.5 }
                    )
                }
            }
        )
        # Legacy v3 payload: same clips, no time keys anywhere. The digest
        # degrades to identity + full-length lines (fail-open), so the
        # char_count delta isolates exactly the injected time dimension.
        $noteFacesLegacy = @(
            @{
                face_kind = "timeline"
                label = "timeline"
                selection_share = 0.72
                domain = @{
                    entries = @(
                        @{ clip_id = "smoke_clip_full"; track_id = "smoke_track_1"; start_seconds = 3.2; end_seconds = 7.8 },
                        @{ clip_id = "smoke_clip_part"; track_id = "smoke_track_1"; start_seconds = 5.0; end_seconds = 12.0 }
                    )
                }
            }
        )
        $noteTurns = @(
            @{ key = "v31"; faces = $noteFacesV31; rangeTimeSpan = @{ start_s = 3.2; end_s = 8.5 } },
            @{ key = "legacy"; faces = $noteFacesLegacy; rangeTimeSpan = $null }
        )
        $noteConversations = @{}
        foreach ($turn in $noteTurns) {
            $noteSessionKey = "smoke_" + $turn.key + "_" + $noteStamp
            $notePayload = @{
                note_id = "note_smoke_" + $turn.key
                session = $noteSessionKey
                faces = $turn.faces
            }
            if ($null -ne $turn.rangeTimeSpan) {
                $notePayload.range_time_span = $turn.rangeTimeSpan
            }
            $notePayload | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath (Join-Path $ScenarioRunDir ("note_payload_" + $turn.key + ".json")) -Encoding UTF8
            $noteResp = Invoke-Json -Method POST -Uri $chatUri -Body @{
                conversation_id = "note_" + $noteSessionKey
                message = $noteQuestion
                note = $notePayload
                context = @{ agent_mode = "chat" }
            } -TimeoutSec 120
            $noteResp | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath (Join-Path $ScenarioRunDir ("note_response_" + $turn.key + ".json")) -Encoding UTF8
            $noteError = [string](Get-OptionalProperty -Object $noteResp -Name "error")
            if (-not [string]::IsNullOrWhiteSpace($noteError)) {
                throw ("note turn (" + $turn.key + ") returned error: " + $noteError)
            }
            $noteReply = [string](Get-OptionalProperty -Object $noteResp -Name "reply")
            if ([string]::IsNullOrWhiteSpace($noteReply)) {
                throw ("note turn (" + $turn.key + ") returned an empty reply")
            }
            $wantedConv = "note_" + $noteSessionKey
            # note_sessions is projected by /agent/runtime/status (handleRuntimeStatus),
            # not by /agent/state — the latter is the compact GUI polling shape.
            $stateAfterNote = Invoke-Json -Method GET -Uri ($AgentHttp.TrimEnd("/") + "/agent/runtime/status") -TimeoutSec 15
            $noteRows = @(Get-OptionalProperty -Object $stateAfterNote -Name "note_sessions")
            ($stateAfterNote | ConvertTo-Json -Depth 12) | Set-Content -LiteralPath (Join-Path $ScenarioRunDir ("agent_runtime_status_note_projection_" + $turn.key + ".json")) -Encoding UTF8
            $matchedRow = $null
            foreach ($row in $noteRows) {
                if ([string](Get-OptionalProperty -Object $row -Name "conversation_id") -eq $wantedConv) {
                    $matchedRow = $row
                    break
                }
            }
            if ($null -eq $matchedRow) {
                throw ("note session " + $wantedConv + " missing from the /agent/state note_sessions projection")
            }
            $matchedMessages = @(Get-OptionalProperty -Object $matchedRow -Name "messages")
            if ($matchedMessages.Count -lt 2) {
                throw ("note session " + $wantedConv + " persisted " + $matchedMessages.Count + " messages; expected the user/assistant exchange")
            }
            $noteConversations[$turn.key] = $wantedConv
            Write-Ok ("note session persisted (" + $turn.key + "): " + $wantedConv + " messages=" + $matchedMessages.Count)
            # v3.1 only: the stored faces must round-trip the per-entry time keys.
            if ($turn.key -eq "v31") {
                $noteProjectPath = [string](Get-OptionalProperty -Object $matchedRow -Name "project_path")
                $noteProjectUUID = [string](Get-OptionalProperty -Object $matchedRow -Name "project_uuid")
                if ([string]::IsNullOrWhiteSpace($noteProjectPath) -or [string]::IsNullOrWhiteSpace($noteProjectUUID)) {
                    throw "note_sessions projection row is missing the project identity"
                }
                $noteStorePath = Join-Path (Join-Path (Split-Path -Parent $noteProjectPath) ".vit_derived" ) (Join-Path $noteProjectUUID "note_sessions.json")
                if (-not (Test-Path -LiteralPath $noteStorePath)) {
                    throw ("note session store file not found: " + $noteStorePath)
                }
                $rawStore = Get-Content -LiteralPath $noteStorePath -Raw -Encoding UTF8
                if (-not $rawStore.Contains($wantedConv) -or -not $rawStore.Contains("range_clip_start")) {
                    throw ("note session store does not carry the v3.1 time keys for " + $wantedConv + ": " + $noteStorePath)
                }
                $rawStore | Set-Content -LiteralPath (Join-Path $ScenarioRunDir "note_sessions_store_snapshot.json") -Encoding UTF8
                Write-Ok "v3.1 time keys survive note session persistence"
            }
        }
        # Deterministic injection surface: the two turns share the system
        # section and the question, so the snapshot section differs only by the
        # time-dimension digest (span in the header + per-clip intersection
        # lines = 36 chars for this fixture; the threshold keeps slack for
        # read-only state drift between the back-to-back turns).
        $telemetryRecords = @()
        $telemetryDeadline = (Get-Date).AddSeconds(10)
        while ((Get-Date) -lt $telemetryDeadline) {
            if (Test-Path -LiteralPath $ScenarioTelemetryFile) {
                $telemetryRecords = @(Get-Content -LiteralPath $ScenarioTelemetryFile -Encoding UTF8 | Where-Object { $_ -like '*"source":"vitnote_chat"*' } | ForEach-Object { $_ | ConvertFrom-Json })
                $v31Telemetry = @($telemetryRecords | Where-Object { [string]$_.conversation_id -eq $noteConversations["v31"] })
                $legacyTelemetry = @($telemetryRecords | Where-Object { [string]$_.conversation_id -eq $noteConversations["legacy"] })
                if ($v31Telemetry.Count -ge 1 -and $legacyTelemetry.Count -ge 1) {
                    break
                }
            }
            Start-Sleep -Milliseconds 500
        }
        $v31Telemetry = @($telemetryRecords | Where-Object { [string]$_.conversation_id -eq $noteConversations["v31"] })
        $legacyTelemetry = @($telemetryRecords | Where-Object { [string]$_.conversation_id -eq $noteConversations["legacy"] })
        if ($v31Telemetry.Count -lt 1 -or $legacyTelemetry.Count -lt 1) {
            throw ("llm telemetry has no vitnote_chat records for the two note turns (v31=" + $v31Telemetry.Count + " legacy=" + $legacyTelemetry.Count + "); file=" + $ScenarioTelemetryFile)
        }
        $v31Chars = [int]$v31Telemetry[0].section_stats.char_count
        $legacyChars = [int]$legacyTelemetry[0].section_stats.char_count
        $timeDeltaChars = $v31Chars - $legacyChars
        Write-Host ("note assembly char_count: v31=" + $v31Chars + " legacy=" + $legacyChars + " delta=" + $timeDeltaChars)
        if ($timeDeltaChars -lt 25) {
            throw ("time_digest injection not visible on the assembly telemetry surface: v31 char_count=" + $v31Chars + " legacy=" + $legacyChars + " (expected delta >= 25)")
        }
        Copy-Item -LiteralPath $ScenarioTelemetryFile -Destination (Join-Path $ScenarioRunDir "agent_llm_telemetry_snapshot.jsonl") -Force
        Write-Ok "time_digest injection visible on the deterministic assembly/telemetry surface"
    }

    if ($Scenario -eq "range_split" -or $Scenario -eq "all") {
        Write-Step "Scenario range_split: boxed-range split proposals"
        $splitStamp = Get-Date -Format "yyyyMMdd_HHmmss"
        $splitRange = @{
            range_id = "smoke_range_1"
            clip_id = "smoke_clip_1"
            track_id = "smoke_track_1"
            start_seconds = 2.0
            end_seconds = 3.5
            duration_seconds = 1.5
            clip_start_seconds = 0.0
            clip_end_seconds = 6.0
            clip_local_start_seconds = 2.0
            clip_local_end_seconds = 3.5
        }
        # "把这段拆出来" -- matches the range-split intent gates (这段 + 拆出)
        # and carries no spoken seconds or playhead reference.
        $splitMessage = -join @([char]0x628A, [char]0x8FD9, [char]0x6BB5, [char]0x62C6, [char]0x51FA, [char]0x6765)

        $getDecisionCommands = {
            param([object]$Resp)
            $rows = @()
            foreach ($decisionRow in @(Get-OptionalProperty -Object $Resp -Name "commands")) {
                if ($null -ne $decisionRow) {
                    $rows += $decisionRow
                }
            }
            return $rows
        }
        # Split decisions arrive in two wire forms: the local intent layer emits
        # tool-form {"tool":"clip.split","args":{...}} while the model envelope
        # (when it answers with its own plan) emits the prompt-taught cmd-form
        # {"cmd":"split_clip",...} with flat params. The user-visible contract
        # (two cuts at the range boundaries, end cut first) is form-agnostic,
        # so both forms are parsed and the form is recorded as the proposal
        # source attribution.
        $getSplitDecisionInfo = {
            param([object]$DecisionRow)
            $commandMap = Get-OptionalProperty -Object $DecisionRow -Name "command"
            if ($null -eq $commandMap) {
                return $null
            }
            $toolName = [string](Get-OptionalProperty -Object $commandMap -Name "tool")
            $cmdName = [string](Get-OptionalProperty -Object $commandMap -Name "cmd")
            $argsMap = $commandMap
            $form = "cmd_form_split_clip"
            if (-not [string]::IsNullOrWhiteSpace($toolName)) {
                $argsMap = Get-OptionalProperty -Object $commandMap -Name "args"
                $form = "tool_form_clip_split"
                if ($null -eq $argsMap) {
                    return $null
                }
            }
            if ($toolName -ne "clip.split" -and $cmdName -ne "split_clip") {
                return $null
            }
            $splitTime = 0.0
            if (-not [double]::TryParse([string](Get-OptionalProperty -Object $argsMap -Name "split_time"), [System.Globalization.NumberStyles]::Float, [System.Globalization.CultureInfo]::InvariantCulture, [ref]$splitTime)) {
                return $null
            }
            return @{
                form = $form
                split_time = $splitTime
                clip_id = [string](Get-OptionalProperty -Object $argsMap -Name "clip_id")
                track_id = [string](Get-OptionalProperty -Object $argsMap -Name "track_id")
            }
        }
        $isExpectedRangeCut = {
            param([object]$SplitInfo, [double]$ExpectedSplitTime)
            return ($null -ne $SplitInfo -and
                [math]::Abs([double]$SplitInfo.split_time - $ExpectedSplitTime) -lt 0.0001 -and
                [string]$SplitInfo.clip_id -eq "smoke_clip_1" -and
                [string]$SplitInfo.track_id -eq "smoke_track_1")
        }

        # Positive group: ranges + boxed-range phrase -> exactly two split
        # proposals, end cut (3.5) before start cut (2.0), needs_confirmation
        # with a plan. disable_agent_loop routes the turn through the legacy
        # command envelope path where the local intent layer lives (the agent
        # loop has no deterministic split intent; the range cut plan is
        # implemented in that layer only). Pre-declared failure classes
        # (AGENTS.md section 8): the model envelope may answer with its OWN
        # split plan and preempt the local intent layer -- when that plan is
        # correct the user-visible contract holds and the form is recorded;
        # a wrong plan is retried as model randomness. Zero commands with
        # ranges present is deterministic (the local synthesis must return the
        # two range cuts), so it fails immediately without retry.
        Write-Step "range_split positive group: two split proposals, end cut first"
        $positivePassed = $false
        $positiveAttempt = 0
        $proposalSource = ""
        while (-not $positivePassed -and $positiveAttempt -lt 3) {
            $positiveAttempt++
            $positiveConv = "dev_range_split_pos_" + $splitStamp + "_" + $positiveAttempt
            $positiveResp = Invoke-Json -Method POST -Uri $chatUri -Body @{
                conversation_id = $positiveConv
                message = $splitMessage
                context = @{
                    agent_mode = "chat"
                    disable_agent_loop = $true
                    selected_clip_id = "smoke_clip_1"
                    selected_clip_track_id = "smoke_track_1"
                    selected_clip_ranges = @($splitRange)
                }
            } -TimeoutSec 120
            $positiveResp | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath (Join-Path $ScenarioRunDir ("range_split_positive_attempt_" + $positiveAttempt + ".json")) -Encoding UTF8
            # @() guard: a scriptblock returning a single-element collection is
            # unrolled to a scalar by the PowerShell pipeline, and .Count on a
            # scalar throws under StrictMode.
            $positiveCommands = @(& $getDecisionCommands $positiveResp)
            $positiveConfirmed = [bool](Get-OptionalProperty -Object $positiveResp -Name "needs_confirmation")
            $positiveSplitInfos = @()
            foreach ($decisionRow in $positiveCommands) {
                $splitInfo = & $getSplitDecisionInfo $decisionRow
                if ($null -ne $splitInfo) {
                    $positiveSplitInfos += $splitInfo
                }
            }
            if ($positiveCommands.Count -eq 2 -and $positiveSplitInfos.Count -eq 2 -and $positiveConfirmed -and
                (& $isExpectedRangeCut $positiveSplitInfos[0] 3.5) -and (& $isExpectedRangeCut $positiveSplitInfos[1] 2.0)) {
                $positivePassed = $true
                $proposalSource = [string]$positiveSplitInfos[0].form
                break
            }
            if ($positiveCommands.Count -gt 0) {
                # Pre-declared retry class (AGENTS.md section 8): the model
                # envelope answered with its own (here: wrong or unparseable)
                # plan, preempting the local intent layer. Model randomness,
                # not a code finding.
                Write-WarnLine ("positive attempt " + $positiveAttempt + ": model envelope plan did not match the range cut contract (" + $positiveSplitInfos.Count + " parseable splits of " + $positiveCommands.Count + " decisions); retrying")
                continue
            }
            # Zero commands with ranges present is deterministic: the local
            # synthesis must return the two range cuts. Reaching here means the
            # intent layer did not engage -- a real failure, no retry.
            throw ("range split proposal missing with ranges present: needs_confirmation=" + $positiveConfirmed + " commands=" + $positiveCommands.Count + " reply=" + [string](Get-OptionalProperty -Object $positiveResp -Name "reply") + " error=" + [string](Get-OptionalProperty -Object $positiveResp -Name "error"))
        }
        if (-not $positivePassed) {
            throw ("range split positive group did not produce the two end-first range cut proposals after " + $positiveAttempt + " attempts; artifacts: " + $ScenarioRunDir)
        }
        Write-Ok ("two split proposals returned (end cut 3.5 first, then start cut 2.0; proposal_source=" + $proposalSource + ")")

        # Counter-case group: identical fixture minus selected_clip_ranges ->
        # zero proposals. The local synthesis has no range rows to act on and
        # must leave the turn to the existing behaviour (plain reply).
        Write-Step "range_split counter-case group: no ranges -> zero proposals"
        $negativePassed = $false
        $negativeAttempt = 0
        while (-not $negativePassed -and $negativeAttempt -lt 3) {
            $negativeAttempt++
            $negativeConv = "dev_range_split_neg_" + $splitStamp + "_" + $negativeAttempt
            $negativeResp = Invoke-Json -Method POST -Uri $chatUri -Body @{
                conversation_id = $negativeConv
                message = $splitMessage
                context = @{
                    agent_mode = "chat"
                    disable_agent_loop = $true
                    selected_clip_id = "smoke_clip_1"
                    selected_clip_track_id = "smoke_track_1"
                }
            } -TimeoutSec 120
            $negativeResp | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath (Join-Path $ScenarioRunDir ("range_split_negative_attempt_" + $negativeAttempt + ".json")) -Encoding UTF8
            $negativeCommands = @(& $getDecisionCommands $negativeResp)
            $negativeConfirmed = [bool](Get-OptionalProperty -Object $negativeResp -Name "needs_confirmation")
            $negativeSplitDecisions = 0
            foreach ($decisionRow in $negativeCommands) {
                $splitInfo = & $getSplitDecisionInfo $decisionRow
                if ($null -ne $splitInfo) {
                    $negativeSplitDecisions++
                }
            }
            if (-not $negativeConfirmed -and $negativeSplitDecisions -eq 0 -and [string]::IsNullOrWhiteSpace([string](Get-OptionalProperty -Object $negativeResp -Name "plan_id"))) {
                $negativePassed = $true
                break
            }
            Write-WarnLine ("counter-case attempt " + $negativeAttempt + " still surfaced a proposal (confirmed=" + $negativeConfirmed + " split_decisions=" + $negativeSplitDecisions + "); retrying")
        }
        if (-not $negativePassed) {
            throw ("range split counter-case failed: without ranges the turn must not surface clip.split proposals, but " + $negativeAttempt + " attempts did")
        }
        Write-Ok "no-ranges counter-case surfaced zero proposals (existing behaviour preserved)"
    }

    if ($Scenario -eq "ref_diff_content" -or $Scenario -eq "all") {
        Write-Step "Scenario ref_diff_content: ref.diff content-level delegation on real observation tickets (L1-3-IMPL-D)"
        # Deterministic scenario (zero LLM): two real mix.observe invocations
        # (read-only tool; com_mode=source_only deterministically attaches a
        # com_projection to each ticket -> a bootstrap com row), then ref.diff
        # identity + content assertions. The second observation carries
        # previous_observation_id -> its ticket holds before_after_delta +
        # ab_result (the observation.before_after carrier's real product). The
        # 2s gap matters: observation ids are obs_<second-resolution timestamp
        # >_<hex>, so a later ticket always sorts after an earlier one and the
        # latest-view coordinate dedup deterministically keeps the second
        # ticket's row.
        $refDiffInvokeUri = $AgentHttp.TrimEnd("/") + "/agent/invoke"
        $observe1 = Invoke-Json -Method POST -Uri $refDiffInvokeUri -Body @{
            tool   = "mix.observe"
            args   = @{ com_mode = "source_only" }
            source = "dev_agent_smoke.ref_diff_content"
        } -TimeoutSec 120
        $observe1Result = Get-OptionalProperty -Object $observe1 -Name "result"
        if ([string]$observe1.status -ne "ok" -or [string]::IsNullOrWhiteSpace([string]$observe1Result)) {
            throw ("first mix.observe invoke failed: " + ($observe1 | ConvertTo-Json -Depth 8 -Compress))
        }
        $obs1 = [string]$observe1Result.observation_id
        $obs1Path = [string]$observe1Result.observation_path
        if ([string]::IsNullOrWhiteSpace($obs1) -or -not (Test-Path -LiteralPath $obs1Path)) {
            throw ("first mix.observe did not persist an observation ticket (id=" + $obs1 + " path=" + $obs1Path + ")")
        }
        Write-Ok ("first observation ticket persisted: " + $obs1)
        Start-Sleep -Seconds 2
        $observe2 = Invoke-Json -Method POST -Uri $refDiffInvokeUri -Body @{
            tool   = "mix.observe"
            args   = @{ com_mode = "source_only"; previous_observation_id = $obs1 }
            source = "dev_agent_smoke.ref_diff_content"
        } -TimeoutSec 120
        $observe2Result = Get-OptionalProperty -Object $observe2 -Name "result"
        if ([string]$observe2.status -ne "ok" -or [string]::IsNullOrWhiteSpace([string]$observe2Result)) {
            throw ("second mix.observe invoke failed: " + ($observe2 | ConvertTo-Json -Depth 8 -Compress))
        }
        $obs2 = [string]$observe2Result.observation_id
        $obs2Path = [string]$observe2Result.observation_path
        if ([string]::IsNullOrWhiteSpace($obs2) -or -not (Test-Path -LiteralPath $obs2Path)) {
            throw ("second mix.observe did not persist an observation ticket (id=" + $obs2 + " path=" + $obs2Path + ")")
        }
        Write-Ok ("second observation ticket persisted: " + $obs2)
        $observe1 | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath (Join-Path $ScenarioRunDir "ref_diff_observe_1.json") -Encoding UTF8
        $observe2 | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath (Join-Path $ScenarioRunDir "ref_diff_observe_2.json") -Encoding UTF8

        # Carrier-product precheck (the real observe flow's product surface; a
        # missing key is a real defect to escalate, not a reason to weaken the
        # assertion below).
        $obs2Ticket = Get-Content -LiteralPath $obs2Path -Raw -Encoding UTF8 | ConvertFrom-Json
        $obs2Metrics = Get-OptionalProperty -Object (Get-OptionalProperty -Object $obs2Ticket -Name "mix_package") -Name "current_metrics"
        if (-not (Get-OptionalProperty -Object $obs2Metrics -Name "before_after_delta") -or -not (Get-OptionalProperty -Object $obs2Metrics -Name "ab_result")) {
            throw ("second observation ticket lacks before_after_delta/ab_result (carrier product missing): " + $obs2Path)
        }
        Write-Ok "second ticket carries before_after_delta + ab_result (real carrier product)"

        # identity depth (IMPL-C surface regression).
        $identityResp = Invoke-Json -Method POST -Uri $refDiffInvokeUri -Body @{
            tool   = "ref.diff"
            args   = @{ base_observation_id = $obs1; depth = "identity" }
            source = "dev_agent_smoke.ref_diff_content"
        } -TimeoutSec ([Math]::Max(30, $WaitSeconds))
        $identityResult = Get-OptionalProperty -Object $identityResp -Name "result"
        if ([string]$identityResp.status -ne "ok" -or [string]$identityResult.cost_class -ne "index" -or $null -eq $identityResult.PSObject.Properties["unchanged_count"]) {
            throw ("ref.diff identity path failed on real tickets: " + ($identityResp | ConvertTo-Json -Depth 8 -Compress))
        }
        Write-Ok ("ref.diff identity ok on real tickets (unchanged_count=" + [string]$identityResult.unchanged_count + ")")
        $identityResp | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath (Join-Path $ScenarioRunDir "ref_diff_identity.json") -Encoding UTF8

        # content depth: delegation map + real before_after evidence.
        $contentResp = Invoke-Json -Method POST -Uri $refDiffInvokeUri -Body @{
            tool   = "ref.diff"
            args   = @{ base_observation_id = $obs1; depth = "content" }
            source = "dev_agent_smoke.ref_diff_content"
        } -TimeoutSec ([Math]::Max(30, $WaitSeconds))
        $contentResult = Get-OptionalProperty -Object $contentResp -Name "result"
        if ([string]$contentResp.status -ne "ok" -or [string]$contentResult.cost_class -ne "compile" -or [string]$contentResult.degraded -ne "") {
            throw ("ref.diff content path contract failed (want ok+compile+empty degraded): " + ($contentResp | ConvertTo-Json -Depth 8 -Compress))
        }
        $delegatedMap = Get-OptionalProperty -Object $contentResult -Name "delegated"
        if (-not $delegatedMap -or $null -eq $contentResult.PSObject.Properties["unrouted_kinds"]) {
            throw ("ref.diff content response missing delegated/unrouted_kinds surface")
        }
        $comChanged = @($contentResult.changed | Where-Object { [string]$_.kind -eq "com" })
        if ($comChanged.Count -lt 1) {
            throw ("content diff must classify the com row as changed (obs1->obs2 instance hashes differ); changed=" + ($contentResult.changed | ConvertTo-Json -Depth 6 -Compress))
        }
        foreach ($entry in $comChanged) {
            if ([string]$entry.hash_semantics -ne "instance_identity") {
                throw ("com changed entry missing instance_identity annotation: " + ($entry | ConvertTo-Json -Depth 4 -Compress))
            }
            if (-not ([string]$entry.base).Contains("@" + $obs1 + "#") -or -not ([string]$entry.head).Contains("@" + $obs2 + "#")) {
                throw ("com changed pair must span @obs1 -> @obs2 snapshots: " + ($entry | ConvertTo-Json -Depth 4 -Compress))
            }
        }
        Write-Ok ("com row classified changed with instance_identity annotation (entries=" + [string]$comChanged.Count + ")")
        $comCarrier = Get-OptionalProperty -Object $delegatedMap -Name "com.change_delta"
        if (-not $comCarrier) {
            throw ("delegated map missing com.change_delta carrier: " + ($delegatedMap | ConvertTo-Json -Depth 8 -Compress))
        }
        $comCarrierHit = @($comCarrier.pairs | Where-Object { [string]$_.base_handle -eq $obs1Path -and [string]$_.head_handle -eq $obs2Path })
        if ($comCarrierHit.Count -lt 1) {
            throw ("com.change_delta pairs must carry base_handle=obs1 ticket and head_handle=obs2 ticket: " + ($comCarrier | ConvertTo-Json -Depth 8 -Compress))
        }
        Write-Ok "com.change_delta carrier maps both ticket handles (delegation wiring live)"
        $baCarrier = Get-OptionalProperty -Object $delegatedMap -Name "observation.before_after"
        if (-not $baCarrier) {
            throw ("delegated map missing observation.before_after carrier despite on-disk ticket evidence")
        }
        $baMatched = @($baCarrier.tickets | Where-Object { [string]$_.handle -eq $obs2Path -and [bool]$_.matches_base })
        if ($baMatched.Count -lt 1) {
            throw ("before_after carrier must list the second ticket with matches_base=true: " + ($baCarrier | ConvertTo-Json -Depth 8 -Compress))
        }
        foreach ($ticketEntry in @($baCarrier.tickets)) {
            if (-not (Test-Path -LiteralPath ([string]$ticketEntry.handle))) {
                throw ("before_after carrier handle does not exist on disk: " + [string]$ticketEntry.handle)
            }
        }
        Write-Ok ("observation.before_after carrier lists real ticket evidence (matches_base=true, tickets=" + [string]@($baCarrier.tickets).Count + ")")
        $contentResp | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath (Join-Path $ScenarioRunDir "ref_diff_content.json") -Encoding UTF8

        # T10 counter-case: unknown depth rejected fail-closed (non-2xx body).
        $refDiffInvokeError = {
            param([object]$ToolArgs)
            $body = @{ tool = "ref.diff"; args = $ToolArgs; source = "dev_agent_smoke.ref_diff_content" }
            $json = $body | ConvertTo-Json -Depth 20 -Compress
            $bytes = [System.Text.Encoding]::UTF8.GetBytes($json)
            try {
                $resp = Invoke-WebRequest -UseBasicParsing -Method POST -Uri $refDiffInvokeUri -Body $bytes -ContentType "application/json; charset=utf-8" -TimeoutSec ([Math]::Max(30, $WaitSeconds))
                return $resp.Content | ConvertFrom-Json
            }
            catch {
                $content = [string]$_.ErrorDetails.Message
                if (-not [string]::IsNullOrWhiteSpace($content)) {
                    return $content | ConvertFrom-Json
                }
                throw
            }
        }
        $badDepthResp = & $refDiffInvokeError @{ base_observation_id = $obs1; depth = "contents" }
        if ([string]$badDepthResp.status -eq "ok") {
            throw "ref.diff unknown depth must be rejected fail-closed, got ok"
        }
        $badDepthError = [string](Get-OptionalProperty -Object $badDepthResp -Name "error")
        if (-not $badDepthError.Contains("rejected")) {
            throw ("ref.diff unknown depth rejection reason mismatch: " + $badDepthError)
        }
        Write-Ok "ref.diff unknown depth rejected as expected (T10)"
    }

    if ($Scenario -eq "midi_register" -or $Scenario -eq "all") {
        Write-Step "Scenario midi_register: kernel import_midi_to_track + apply_midi_note_patch registration (MIDI-CMD-REGISTER-1)"
        # Deterministic scenario (zero LLM). MIDI-RECON-1 proved both kernel
        # commands were unreachable ("Unknown command" from the dispatcher
        # registration gap); this scenario drives the repaired registration
        # through the real agent tool chain: track.add -> midi.import_file
        # (confirmation card then confirmed execute under a restricted
        # authority, or immediate execute under the berth's default
        # full-project-access authority; either way the VSP legacy command
        # must be import_midi_to_track, stage completed) -> read-back of the
        # imported notes -> mixed-op midi.write_clip_notes (insert +
        # transpose + delete, so the pure-insert add_midi_notes translation
        # cannot apply and apply_midi_note_patch itself is dispatched) ->
        # final read-back. Writes travel the same VSP legacy.command escape
        # hatch the recon probes used.
        $midiInvokeUri = $AgentHttp.TrimEnd("/") + "/agent/invoke"

        # Self-generated SMF type-0 fixture (PPQ 480): notes 60/64/67 at
        # beats 0/1/2, length 1 beat, velocities 90/80/70. Generated bytes
        # keep the run hermetic -- no dependency on untracked probe
        # artifacts. The kernel process reads the file itself, so the path
        # handed over is the absolute run-dir path.
        $fixturePath = Join-Path $ScenarioRunDir "midi_register_fixture.mid"
        [byte[]]$fixtureBytes = @(
            0x4D,0x54,0x68,0x64, 0x00,0x00,0x00,0x06, 0x00,0x00, 0x00,0x01, 0x01,0xE0,
            0x4D,0x54,0x72,0x6B, 0x00,0x00,0x00,0x1F,
            0x00,0x90,0x3C,0x5A,
            0x83,0x60,0x80,0x3C,0x00,
            0x00,0x90,0x40,0x50,
            0x83,0x60,0x80,0x40,0x00,
            0x00,0x90,0x43,0x46,
            0x83,0x60,0x80,0x43,0x00,
            0x00,0xFF,0x2F,0x00
        )
        [System.IO.File]::WriteAllBytes($fixturePath, $fixtureBytes)
        if (-not (Test-Path -LiteralPath $fixturePath)) {
            throw ("midi fixture write failed: " + $fixturePath)
        }
        Write-Ok ("midi fixture written: " + $fixturePath + " (" + [string]$fixtureBytes.Count + " bytes)")

        # Host track for the import.
        $midiTrackResp = Invoke-Json -Method POST -Uri $midiInvokeUri -Body @{
            tool   = "track.add"
            args   = @{}
            source = "dev_agent_smoke.midi_register"
        } -TimeoutSec 60
        $midiTrackResult = Get-OptionalProperty -Object $midiTrackResp -Name "result"
        $midiTrackId = [string](Get-OptionalProperty -Object $midiTrackResult -Name "track_id")
        if ([string]$midiTrackResp.status -ne "ok" -or [string]::IsNullOrWhiteSpace($midiTrackId)) {
            throw ("track.add failed: " + ($midiTrackResp | ConvertTo-Json -Depth 8 -Compress))
        }
        Write-Ok ("host track added: " + $midiTrackId)
        $midiTrackResp | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath (Join-Path $ScenarioRunDir "midi_register_track_add.json") -Encoding UTF8

        # midi.import_file routes to the kernel command import_midi_to_track.
        # The scenario berth's agent runs under full-project-access authority
        # by default, so a confirm-level write executes immediately; under a
        # restricted authority it first surfaces a confirmation card. Both
        # forms are accepted -- the surface this card repairs is the kernel
        # dispatch, asserted on the executed reply either way.
        $importResp = Invoke-Json -Method POST -Uri $midiInvokeUri -Body @{
            tool   = "midi.import_file"
            args   = @{ track_id = $midiTrackId; file_path = $fixturePath; start_time_beats = 0 }
            source = "dev_agent_smoke.midi_register"
        } -TimeoutSec 120
        if ([string]$importResp.status -eq "needs_confirmation") {
            if ([string](Get-OptionalProperty -Object $importResp -Name "command_name") -ne "import_midi_to_track") {
                throw ("midi.import_file confirmation card routed elsewhere: " + ($importResp | ConvertTo-Json -Depth 8 -Compress))
            }
            Write-Ok ("midi.import_file card routed to import_midi_to_track (action " + [string](Get-OptionalProperty -Object $importResp -Name "agent_action_id") + ")")
            $importResp | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath (Join-Path $ScenarioRunDir "midi_register_import_card.json") -Encoding UTF8
            $importResp = Invoke-Json -Method POST -Uri $midiInvokeUri -Body @{
                tool      = "midi.import_file"
                args      = @{ track_id = $midiTrackId; file_path = $fixturePath; start_time_beats = 0 }
                source    = "dev_agent_smoke.midi_register"
                confirmed = $true
            } -TimeoutSec 120
        }
        $importResult = Get-OptionalProperty -Object $importResp -Name "result"
        if ([string]$importResp.status -ne "ok") {
            throw ("midi.import_file confirmed execute failed (was the rebuilt kernel deployed to Export/staging/runtime?): " + ($importResp | ConvertTo-Json -Depth 8 -Compress))
        }
        $midiClipId = [string](Get-OptionalProperty -Object $importResult -Name "clip_id")
        if ([string]::IsNullOrWhiteSpace($midiClipId) -or [string](Get-OptionalProperty -Object $importResult -Name "note_count") -ne "3") {
            throw ("midi.import_file result missing clip_id or note_count=3: " + ($importResp | ConvertTo-Json -Depth 8 -Compress))
        }
        $importAck = Get-OptionalProperty -Object (Get-OptionalProperty -Object $importResult -Name "vsp") -Name "command_ack"
        $importLegacy = [string](Get-OptionalProperty -Object $importAck -Name "legacy_command")
        $importStage = [string](Get-OptionalProperty -Object $importAck -Name "stage")
        if ($importLegacy -ne "import_midi_to_track" -or $importStage -ne "completed") {
            throw ("midi.import_file must complete via legacy command import_midi_to_track (got " + $importLegacy + "/" + $importStage + "): " + ($importResp | ConvertTo-Json -Depth 8 -Compress))
        }
        Write-Ok ("midi.import_file executed via import_midi_to_track: clip=" + $midiClipId + " notes=3")
        $importResp | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath (Join-Path $ScenarioRunDir "midi_register_import_exec.json") -Encoding UTF8

        # Read-back 1: the imported clip must hold exactly the fixture notes.
        $readOneResp = Invoke-Json -Method POST -Uri $midiInvokeUri -Body @{
            tool   = "midi.read_clip_notes"
            args   = @{ clip_id = $midiClipId }
            source = "dev_agent_smoke.midi_register"
        } -TimeoutSec 60
        $readOneResult = Get-OptionalProperty -Object $readOneResp -Name "result"
        if ([string]$readOneResp.status -ne "ok") {
            throw ("midi.read_clip_notes after import failed: " + ($readOneResp | ConvertTo-Json -Depth 8 -Compress))
        }
        $readOneNotes = @(Get-OptionalProperty -Object $readOneResult -Name "notes")
        if ($readOneNotes.Count -ne 3) {
            throw ("imported clip must hold 3 notes, got " + [string]$readOneNotes.Count + ": " + ($readOneResp | ConvertTo-Json -Depth 8 -Compress))
        }
        foreach ($expectedNote in @(
            @{ pitch = 60; start = 0.0; length = 1.0; velocity = 90 },
            @{ pitch = 64; start = 1.0; length = 1.0; velocity = 80 },
            @{ pitch = 67; start = 2.0; length = 1.0; velocity = 70 }
        )) {
            $hits = @($readOneNotes | Where-Object { [int]$_.pitch -eq [int]$expectedNote.pitch })
            if ($hits.Count -ne 1) {
                throw ("imported pitch " + [string]$expectedNote.pitch + " expected exactly once, got " + [string]$hits.Count)
            }
            if ([Math]::Abs([double]$hits[0].start - [double]$expectedNote.start) -gt 0.001 -or [Math]::Abs([double]$hits[0].length - [double]$expectedNote.length) -gt 0.001 -or [int]$hits[0].velocity -ne [int]$expectedNote.velocity) {
                throw ("imported pitch " + [string]$expectedNote.pitch + " read-back mismatch: " + ($hits[0] | ConvertTo-Json -Depth 4 -Compress))
            }
        }
        Write-Ok ("imported notes read back consistent (60@0/64@1/67@2, len 1, vel 90/80/70): " + $midiClipId)
        $readOneResp | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath (Join-Path $ScenarioRunDir "midi_register_readback_import.json") -Encoding UTF8

        # Mixed-op patch: insert + transpose + delete. With a non-insert op
        # present the pure-insert translation to add_midi_notes cannot apply,
        # so the kernel command apply_midi_note_patch itself is dispatched --
        # the second registration this card repairs.
        $patchOperations = @(
            @{ op = "insert_note"; pitch = 72; start = 3.0; length = 1.0; velocity = 64 },
            @{ op = "transpose_note"; semitones = 5; start = 0.0; length = 1.0 },
            @{ op = "delete_note"; start = 1.0; length = 1.0 }
        )
        # Same dual-form handling as the import: full-access berth executes
        # immediately, restricted authority surfaces a card first.
        $patchResp = Invoke-Json -Method POST -Uri $midiInvokeUri -Body @{
            tool   = "midi.write_clip_notes"
            args   = @{ clip_id = $midiClipId; time_unit = "beats"; operations = $patchOperations }
            source = "dev_agent_smoke.midi_register"
        } -TimeoutSec 120
        if ([string]$patchResp.status -eq "needs_confirmation") {
            if ([string](Get-OptionalProperty -Object $patchResp -Name "command_name") -ne "apply_midi_note_patch") {
                throw ("midi.write_clip_notes confirmation card routed elsewhere: " + ($patchResp | ConvertTo-Json -Depth 8 -Compress))
            }
            Write-Ok ("midi.write_clip_notes card routed to apply_midi_note_patch (mixed ops, no add_midi_notes fallback possible)")
            $patchResp | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath (Join-Path $ScenarioRunDir "midi_register_patch_card.json") -Encoding UTF8
            $patchResp = Invoke-Json -Method POST -Uri $midiInvokeUri -Body @{
                tool      = "midi.write_clip_notes"
                args      = @{ clip_id = $midiClipId; time_unit = "beats"; operations = $patchOperations }
                source    = "dev_agent_smoke.midi_register"
                confirmed = $true
            } -TimeoutSec 120
        }
        $patchResult = Get-OptionalProperty -Object $patchResp -Name "result"
        if ([string]$patchResp.status -ne "ok") {
            throw ("midi.write_clip_notes mixed-op execute failed: " + ($patchResp | ConvertTo-Json -Depth 8 -Compress))
        }
        $patchAck = Get-OptionalProperty -Object (Get-OptionalProperty -Object $patchResult -Name "vsp") -Name "command_ack"
        $patchLegacy = [string](Get-OptionalProperty -Object $patchAck -Name "legacy_command")
        $patchStage = [string](Get-OptionalProperty -Object $patchAck -Name "stage")
        if ($patchLegacy -ne "apply_midi_note_patch" -or $patchStage -ne "completed") {
            throw ("mixed-op patch must complete via legacy command apply_midi_note_patch (got " + $patchLegacy + "/" + $patchStage + "): " + ($patchResp | ConvertTo-Json -Depth 8 -Compress))
        }
        Write-Ok ("midi.write_clip_notes executed via apply_midi_note_patch (insert+transpose+delete)")
        $patchResp | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath (Join-Path $ScenarioRunDir "midi_register_patch_exec.json") -Encoding UTF8

        # Read-back 2: 60 transposed to 65, 64 deleted, 67 untouched,
        # 72 inserted -- the exact mixed-op post-state.
        $readTwoResp = Invoke-Json -Method POST -Uri $midiInvokeUri -Body @{
            tool   = "midi.read_clip_notes"
            args   = @{ clip_id = $midiClipId }
            source = "dev_agent_smoke.midi_register"
        } -TimeoutSec 60
        $readTwoResult = Get-OptionalProperty -Object $readTwoResp -Name "result"
        if ([string]$readTwoResp.status -ne "ok") {
            throw ("midi.read_clip_notes after patch failed: " + ($readTwoResp | ConvertTo-Json -Depth 8 -Compress))
        }
        $readTwoNotes = @(Get-OptionalProperty -Object $readTwoResult -Name "notes")
        if ($readTwoNotes.Count -ne 3) {
            throw ("patched clip must hold 3 notes (65/67/72), got " + [string]$readTwoNotes.Count + ": " + ($readTwoResp | ConvertTo-Json -Depth 8 -Compress))
        }
        foreach ($expectedNote in @(
            @{ pitch = 65; start = 0.0; length = 1.0; velocity = 90 },
            @{ pitch = 67; start = 2.0; length = 1.0; velocity = 70 },
            @{ pitch = 72; start = 3.0; length = 1.0; velocity = 64 }
        )) {
            $hits = @($readTwoNotes | Where-Object { [int]$_.pitch -eq [int]$expectedNote.pitch })
            if ($hits.Count -ne 1) {
                throw ("patched pitch " + [string]$expectedNote.pitch + " expected exactly once, got " + [string]$hits.Count + ": " + ($readTwoResp | ConvertTo-Json -Depth 8 -Compress))
            }
            if ([Math]::Abs([double]$hits[0].start - [double]$expectedNote.start) -gt 0.001 -or [Math]::Abs([double]$hits[0].length - [double]$expectedNote.length) -gt 0.001 -or [int]$hits[0].velocity -ne [int]$expectedNote.velocity) {
                throw ("patched pitch " + [string]$expectedNote.pitch + " read-back mismatch: " + ($hits[0] | ConvertTo-Json -Depth 4 -Compress))
            }
        }
        Write-Ok ("patched notes read back consistent (65@0/67@2/72@3, len 1, vel 90/70/64): " + $midiClipId)
        $readTwoResp | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath (Join-Path $ScenarioRunDir "midi_register_readback_patch.json") -Encoding UTF8
    }

    if ($Scenario -eq "render_freeze" -or $Scenario -eq "all") {
        Write-Step "Scenario render_freeze: render ABBA deadlock fix reversal (KERNEL-RENDER-FREEZE-FIX-1)"
        # Deterministic scenario (zero LLM). KERNEL-RENDER-FREEZE-1 proved that
        # any render which fails after startup freezes the JUCE message thread
        # (completion callback renderHandle.reset() joins the render thread
        # which is itself blocked in ~NodeRenderContext's callBlocking waiting
        # for the message thread). KERNEL-RENDER-FREEZE-FIX-1 reverses that
        # sequence to green on three legs, verified on the real stack before
        # this scenario was written (coord/runs/KERNEL-RENDER-FREEZE-FIX-1/
        # 20261006_verify1):
        #   leg B  - MIDI-only edit: render.start must return a synchronous
        #            status=error reason=no_renderable_audio_content reply
        #            (new reply semantics declared on the card) with no job_id,
        #            and the command surface must stay alive afterwards.
        #   leg A  - a render that starts and then ends with a failure result
        #            (here: cancel mid-render -> "Cancelled") must deliver its
        #            render_failed telemetry through the kernel PUB socket and
        #            leave get_project_state responsive -- the exact completion
        #            path that used to deadlock.
        #   cancel - render.cancel must reply ok both idle and against a live
        #            render job.
        # The render_failed/render_done telemetry arrival is asserted through
        # render.profile.bind: its error distinguishes "unknown render (no
        # telemetry cached)" from "is not ready (status \"failed\")", so a
        # terminal-status reply proves the agent ingested the kernel render
        # telemetry for that job id.
        $renderInvokeUri = $AgentHttp.TrimEnd("/") + "/agent/invoke"

        # Deterministic slate: kernel clear_project removes every clip (the
        # dispatcher refuses to delete the last audio track, so track.delete
        # cannot empty the edit on its own). Leftover clips from earlier berth
        # runs would otherwise turn a MIDI-only edit into a renderable one --
        # the staging kernel persists its default project across runs. The
        # clear_project invocation resolves through the command-catalog
        # fallback (LookupCommand), routing to the kernel command verbatim.
        $renderClearResp = Invoke-Json -Method POST -Uri $renderInvokeUri -Body @{
            tool   = "clear_project"
            args   = @{}
            source = "dev_agent_smoke.render_freeze"
        } -TimeoutSec 60
        if ([string]$renderClearResp.status -eq "needs_confirmation") {
            $renderClearResp = Invoke-Json -Method POST -Uri $renderInvokeUri -Body @{
                tool      = "clear_project"
                args      = @{}
                source    = "dev_agent_smoke.render_freeze"
                confirmed = $true
            } -TimeoutSec 60
        }
        if ([string]$renderClearResp.status -ne "ok") {
            throw ("render_freeze slate clear_project failed: " + ($renderClearResp | ConvertTo-Json -Depth 8 -Compress))
        }
        Write-Ok "render_freeze slate swept: clear_project removed all clips"

        # ---- leg B: MIDI-only edit must be rejected synchronously -----------
        $renderTrackResp = Invoke-Json -Method POST -Uri $renderInvokeUri -Body @{
            tool   = "track.add"
            args   = @{}
            source = "dev_agent_smoke.render_freeze"
        } -TimeoutSec 60
        $renderTrackResult = Get-OptionalProperty -Object $renderTrackResp -Name "result"
        $renderTrackId = [string](Get-OptionalProperty -Object $renderTrackResult -Name "track_id")
        if ([string]$renderTrackResp.status -ne "ok" -or [string]::IsNullOrWhiteSpace($renderTrackId)) {
            throw ("render_freeze track.add failed: " + ($renderTrackResp | ConvertTo-Json -Depth 8 -Compress))
        }
        Write-Ok ("host track added: " + $renderTrackId)

        $renderFixturePath = Join-Path $ScenarioRunDir "render_freeze_fixture.mid"
        [byte[]]$renderFixtureBytes = @(
            0x4D,0x54,0x68,0x64, 0x00,0x00,0x00,0x06, 0x00,0x00, 0x00,0x01, 0x01,0xE0,
            0x4D,0x54,0x72,0x6B, 0x00,0x00,0x00,0x1F,
            0x00,0x90,0x3C,0x5A,
            0x83,0x60,0x80,0x3C,0x00,
            0x00,0x90,0x40,0x50,
            0x83,0x60,0x80,0x40,0x00,
            0x00,0x90,0x43,0x46,
            0x83,0x60,0x80,0x43,0x00,
            0x00,0xFF,0x2F,0x00
        )
        [System.IO.File]::WriteAllBytes($renderFixturePath, $renderFixtureBytes)

        $renderImportResp = Invoke-Json -Method POST -Uri $renderInvokeUri -Body @{
            tool   = "midi.import_file"
            args   = @{ track_id = $renderTrackId; file_path = $renderFixturePath; start_time_beats = 0 }
            source = "dev_agent_smoke.render_freeze"
        } -TimeoutSec 120
        if ([string]$renderImportResp.status -eq "needs_confirmation") {
            $renderImportResp = Invoke-Json -Method POST -Uri $renderInvokeUri -Body @{
                tool      = "midi.import_file"
                args      = @{ track_id = $renderTrackId; file_path = $renderFixturePath; start_time_beats = 0 }
                source    = "dev_agent_smoke.render_freeze"
                confirmed = $true
            } -TimeoutSec 120
        }
        if ([string]$renderImportResp.status -ne "ok") {
            throw ("render_freeze midi.import_file execute failed: " + ($renderImportResp | ConvertTo-Json -Depth 8 -Compress))
        }
        Write-Ok ("MIDI-only edit assembled (3 notes, no instrument): track=" + $renderTrackId)

        $midiOnlyWav = Join-Path $ScenarioRunDir "render_freeze_midi_only.wav"
        $legBResp = Invoke-Json -Method POST -Uri $renderInvokeUri -Body @{
            tool   = "render.start"
            args   = @{ file_path = $midiOnlyWav }
            source = "dev_agent_smoke.render_freeze"
        } -TimeoutSec 60
        if ([string]$legBResp.status -eq "needs_confirmation") {
            $legBResp = Invoke-Json -Method POST -Uri $renderInvokeUri -Body @{
                tool      = "render.start"
                args      = @{ file_path = $midiOnlyWav }
                source    = "dev_agent_smoke.render_freeze"
                confirmed = $true
            } -TimeoutSec 60
        }
        $legBResp | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath (Join-Path $ScenarioRunDir "render_freeze_legB_reply.json") -Encoding UTF8
        $legBResult = Get-OptionalProperty -Object $legBResp -Name "result"
        $legBStatus = [string](Get-OptionalProperty -Object $legBResult -Name "status")
        if ([string]::IsNullOrWhiteSpace($legBStatus)) {
            $legBStatus = [string]$legBResp.status
        }
        $legBReason = [string](Get-OptionalProperty -Object $legBResult -Name "reason")
        $legBMessage = [string](Get-OptionalProperty -Object $legBResult -Name "message")
        $legBJobId = [string](Get-OptionalProperty -Object $legBResult -Name "job_id")
        if ($legBStatus -ne "error" -or $legBReason -ne "no_renderable_audio_content" -or -not $legBMessage.Contains("no renderable audio content") -or -not [string]::IsNullOrWhiteSpace($legBJobId)) {
            throw ("render_freeze leg B: MIDI-only render.start must reply status=error reason=no_renderable_audio_content with no job_id (got status=" + $legBStatus + " reason=" + $legBReason + " job_id=" + $legBJobId + "): " + ($legBResp | ConvertTo-Json -Depth 12 -Compress))
        }
        Write-Ok ("leg B: MIDI-only render.start rejected synchronously (reason=no_renderable_audio_content, no job started)")

        # Command surface must stay alive after the rejection (leg B) and the
        # idle cancel path must reply.
        $legBAliveResp = Invoke-Json -Method POST -Uri $renderInvokeUri -Body @{
            tool   = "project.state"
            args   = @{}
            source = "dev_agent_smoke.render_freeze"
        } -TimeoutSec 20
        if ([string]$legBAliveResp.status -ne "ok") {
            throw ("render_freeze leg B: project.state after sync rejection failed (command surface frozen?): " + ($legBAliveResp | ConvertTo-Json -Depth 8 -Compress))
        }
        Write-Ok "leg B: command surface alive after rejection"

        $legBCancelResp = Invoke-Json -Method POST -Uri $renderInvokeUri -Body @{
            tool   = "render.cancel"
            args   = @{}
            source = "dev_agent_smoke.render_freeze"
        } -TimeoutSec 20
        if ([string]$legBCancelResp.status -ne "ok") {
            throw ("render_freeze leg B: idle render.cancel failed: " + ($legBCancelResp | ConvertTo-Json -Depth 8 -Compress))
        }
        Write-Ok "leg B: idle render.cancel replied ok"

        # Give the edit renderable audio content (sine stem), so later render
        # groups pass the leg-B pre-check and reach the engine.
        $renderSineWav = Write-LeaseSmokeStemWav -Path (Join-Path $ScenarioRunDir "render_freeze_sine.wav") -Amplitude 0.5 -Frequency 440.0
        $renderAudioTrackResp = Invoke-Json -Method POST -Uri $renderInvokeUri -Body @{
            tool   = "track.add"
            args   = @{}
            source = "dev_agent_smoke.render_freeze"
        } -TimeoutSec 60
        $renderAudioTrackId = [string](Get-OptionalProperty -Object (Get-OptionalProperty -Object $renderAudioTrackResp -Name "result") -Name "track_id")
        if ([string]$renderAudioTrackResp.status -ne "ok" -or [string]::IsNullOrWhiteSpace($renderAudioTrackId)) {
            throw ("render_freeze audio track.add failed: " + ($renderAudioTrackResp | ConvertTo-Json -Depth 8 -Compress))
        }
        $renderImportAudioResp = Invoke-Json -Method POST -Uri $renderInvokeUri -Body @{
            tool   = "clip.import_audio"
            args   = @{ track_id = $renderAudioTrackId; file_path = $renderSineWav }
            source = "dev_agent_smoke.render_freeze"
        } -TimeoutSec 180
        if ([string]$renderImportAudioResp.status -eq "needs_confirmation") {
            $renderImportAudioResp = Invoke-Json -Method POST -Uri $renderInvokeUri -Body @{
                tool      = "clip.import_audio"
                args      = @{ track_id = $renderAudioTrackId; file_path = $renderSineWav }
                source    = "dev_agent_smoke.render_freeze"
                confirmed = $true
            } -TimeoutSec 180
        }
        if ([string]$renderImportAudioResp.status -ne "ok") {
            throw ("render_freeze clip.import_audio execute failed: " + ($renderImportAudioResp | ConvertTo-Json -Depth 8 -Compress))
        }
        Write-Ok ("sine stem imported on track " + $renderAudioTrackId + " (edit now has audio content)")

        # ---- leg A: deterministic failed-result completion (unwritable -----
        # ---- destination) -- the deadlock shape, reversed.               -----
        # startOfflineRender ignores the createDirectory() result (pre-existing
        # behavior), so a destination on a non-existent drive passes the sync
        # checks, replies "Render started", then fails inside
        # NodeRenderContext at writer open ("Couldn't write to target file").
        # That is a completion callback on the message thread with a FAILED
        # result while the render thread unwinds a live nodePlayer -- the
        # exact ABBA shape that froze the message thread before the fix. No
        # timing race: the failure is content-determined.
        $renderDeadDrive = $null
        foreach ($renderDriveLetter in @("B:", "A:", "Y:", "Z:", "X:")) {
            if (-not [System.IO.Directory]::Exists(($renderDriveLetter + "\"))) {
                $renderDeadDrive = $renderDriveLetter
                break
            }
        }
        if ([string]::IsNullOrWhiteSpace($renderDeadDrive)) {
            throw "render_freeze leg A: no absent drive letter found for the unwritable-destination probe"
        }
        $badWav = $renderDeadDrive + "\vit_fix1_probe\render_freeze_unwritable.wav"
        $legAStartResp = Invoke-Json -Method POST -Uri $renderInvokeUri -Body @{
            tool   = "render.start"
            args   = @{ file_path = $badWav; range = @(0.0, 2.0) }
            source = "dev_agent_smoke.render_freeze"
        } -TimeoutSec 60
        if ([string]$legAStartResp.status -eq "needs_confirmation") {
            $legAStartResp = Invoke-Json -Method POST -Uri $renderInvokeUri -Body @{
                tool      = "render.start"
                args      = @{ file_path = $badWav; range = @(0.0, 2.0) }
                source    = "dev_agent_smoke.render_freeze"
                confirmed = $true
            } -TimeoutSec 60
        }
        $legAStartResult = Get-OptionalProperty -Object $legAStartResp -Name "result"
        $badJobId = [string](Get-OptionalProperty -Object $legAStartResult -Name "job_id")
        $legAStartStatus = [string](Get-OptionalProperty -Object $legAStartResult -Name "status")
        if ([string]::IsNullOrWhiteSpace($legAStartStatus)) {
            $legAStartStatus = [string]$legAStartResp.status
        }
        if ($legAStartStatus -ne "ok" -or [string]::IsNullOrWhiteSpace($badJobId)) {
            throw ("render_freeze leg A: unwritable-destination render.start must be accepted synchronously (got status=" + $legAStartStatus + "): " + ($legAStartResp | ConvertTo-Json -Depth 12 -Compress))
        }
        $legAStartResp | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath (Join-Path $ScenarioRunDir "render_freeze_legA_start.json") -Encoding UTF8
        Write-Ok ("doomed render started on unwritable destination: job=" + $badJobId + " (" + $renderDeadDrive + " absent)")

        # render_failed telemetry arrival: poll render.profile.bind until its
        # error no longer says "no render telemetry cached" and carries the
        # terminal failed status for this job id.
        $legABindDeadline = (Get-Date).AddSeconds(45)
        $legABindTerminal = $false
        $legABindLastJson = ""
        while ((Get-Date) -lt $legABindDeadline) {
            $legABindResp = Invoke-JsonTolerant -Uri $renderInvokeUri -Body @{
                tool   = "render.profile.bind"
                args   = @{ render_id = $badJobId; profile_id = "builtin:apple_music" }
                source = "dev_agent_smoke.render_freeze"
            } -TimeoutSec 30
            $legABindLastJson = ($legABindResp | ConvertTo-Json -Depth 12 -Compress)
            if ($legABindLastJson.Contains("no render telemetry cached")) {
                Start-Sleep -Milliseconds 500
                continue
            }
            if ($legABindLastJson.Contains("is not ready") -and $legABindLastJson.Contains("failed")) {
                $legABindTerminal = $true
                break
            }
            throw ("render_freeze leg A: render.profile.bind produced an unexpected reply for the doomed job (expected terminal failed status): " + $legABindLastJson)
        }
        if (-not $legABindTerminal) {
            throw ("render_freeze leg A: render_failed telemetry for job " + $badJobId + " never reached the agent within 45s (last bind reply: " + $legABindLastJson + ")")
        }
        $legABindLastJson | Set-Content -LiteralPath (Join-Path $ScenarioRunDir "render_freeze_legA_bind_terminal.json") -Encoding UTF8
        Write-Ok ("leg A: render_failed telemetry reached the agent for doomed job " + $badJobId)

        # THE reversal assertion: after a failed-result render completion, the
        # kernel command surface must still respond (previously this exact
        # sequence froze the message thread and every command timed out).
        $legAAliveResp = Invoke-Json -Method POST -Uri $renderInvokeUri -Body @{
            tool   = "project.state"
            args   = @{}
            source = "dev_agent_smoke.render_freeze"
        } -TimeoutSec 20
        if ([string]$legAAliveResp.status -ne "ok") {
            throw ("render_freeze leg A: project.state after failed-result render failed (message thread frozen?): " + ($legAAliveResp | ConvertTo-Json -Depth 8 -Compress))
        }
        Write-Ok "leg A: command surface alive after failed-result render completion (freeze reversed)"

        # ---- live-cancel path availability: start a live render, cancel it, --
        # ---- require a terminal telemetry of EITHER kind plus a live surface.--
        # Whether the cancel lands mid-render (render_failed "Cancelled") or
        # the render wins the race (render_done) depends on machine speed and
        # is deliberately NOT gated; the deadlock-reversal assertion lives in
        # the deterministic unwritable-destination group above.
        $liveWav = Join-Path $ScenarioRunDir "render_freeze_live_cancel.wav"
        $liveStartResp = Invoke-Json -Method POST -Uri $renderInvokeUri -Body @{
            tool   = "render.start"
            args   = @{ file_path = $liveWav; range = @(0.0, 30.0) }
            source = "dev_agent_smoke.render_freeze"
        } -TimeoutSec 60
        if ([string]$liveStartResp.status -eq "needs_confirmation") {
            $liveStartResp = Invoke-Json -Method POST -Uri $renderInvokeUri -Body @{
                tool      = "render.start"
                args      = @{ file_path = $liveWav; range = @(0.0, 30.0) }
                source    = "dev_agent_smoke.render_freeze"
                confirmed = $true
            } -TimeoutSec 60
        }
        $liveStartResult = Get-OptionalProperty -Object $liveStartResp -Name "result"
        $liveJobId = [string](Get-OptionalProperty -Object $liveStartResult -Name "job_id")
        $liveStartStatus = [string](Get-OptionalProperty -Object $liveStartResult -Name "status")
        if ([string]::IsNullOrWhiteSpace($liveStartStatus)) {
            $liveStartStatus = [string]$liveStartResp.status
        }
        if ($liveStartStatus -ne "ok" -or [string]::IsNullOrWhiteSpace($liveJobId)) {
            throw ("render_freeze live-cancel: render.start must succeed with a job_id (got status=" + $liveStartStatus + "): " + ($liveStartResp | ConvertTo-Json -Depth 12 -Compress))
        }
        $liveStartResp | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath (Join-Path $ScenarioRunDir "render_freeze_live_start.json") -Encoding UTF8
        Write-Ok ("live render started: job=" + $liveJobId)

        # Cancel immediately. If the cancel lands mid-render the completion
        # callback carries a FAILED result ("Cancelled"); if the render wins
        # the race it completes normally. Either way the cancel command must
        # reply and the job must reach a terminal telemetry state.
        $liveCancelResp = Invoke-Json -Method POST -Uri $renderInvokeUri -Body @{
            tool   = "render.cancel"
            args   = @{}
            source = "dev_agent_smoke.render_freeze"
        } -TimeoutSec 20
        if ([string]$liveCancelResp.status -ne "ok") {
            throw ("render_freeze live-cancel: render.cancel against live job failed: " + ($liveCancelResp | ConvertTo-Json -Depth 8 -Compress))
        }
        $liveCancelResp | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath (Join-Path $ScenarioRunDir "render_freeze_live_cancel.json") -Encoding UTF8
        Write-Ok "render.cancel against live job replied ok"

        # Terminal telemetry of EITHER kind proves the cancel path interplays
        # with a live job without burying the engine.
        $liveBindDeadline = (Get-Date).AddSeconds(45)
        $liveTerminal = ""
        $liveLastJson = ""
        while ((Get-Date) -lt $liveBindDeadline) {
            $liveBindResp = Invoke-JsonTolerant -Uri $renderInvokeUri -Body @{
                tool   = "render.profile.bind"
                args   = @{ render_id = $liveJobId; profile_id = "builtin:apple_music" }
                source = "dev_agent_smoke.render_freeze"
            } -TimeoutSec 30
            $liveLastJson = ($liveBindResp | ConvertTo-Json -Depth 12 -Compress)
            if ($liveLastJson.Contains("no render telemetry cached")) {
                Start-Sleep -Milliseconds 500
                continue
            }
            if ([string]$liveBindResp.status -eq "ok") {
                $liveTerminal = "ready"
                break
            }
            if ($liveLastJson.Contains("is not ready") -and $liveLastJson.Contains("failed")) {
                $liveTerminal = "failed"
                break
            }
            throw ("render_freeze live-cancel: unexpected bind reply for job " + $liveJobId + ": " + $liveLastJson)
        }
        if ([string]::IsNullOrWhiteSpace($liveTerminal)) {
            throw ("render_freeze live-cancel: terminal telemetry for job " + $liveJobId + " never reached the agent within 45s (last bind reply: " + $liveLastJson + ")")
        }
        $liveLastJson | Set-Content -LiteralPath (Join-Path $ScenarioRunDir "render_freeze_live_bind_terminal.json") -Encoding UTF8
        Write-Ok ("live-cancel: job " + $liveJobId + " reached terminal telemetry (status=" + $liveTerminal + "; outcome intentionally not gated)")

        $liveAliveResp = Invoke-Json -Method POST -Uri $renderInvokeUri -Body @{
            tool   = "project.state"
            args   = @{}
            source = "dev_agent_smoke.render_freeze"
        } -TimeoutSec 20
        if ([string]$liveAliveResp.status -ne "ok") {
            throw ("render_freeze live-cancel: project.state after live-cancel sequence failed: " + ($liveAliveResp | ConvertTo-Json -Depth 8 -Compress))
        }
        Write-Ok "live-cancel: command surface alive"
        if (Test-Path -LiteralPath $liveWav) {
            Remove-Item -LiteralPath $liveWav -Force -ErrorAction SilentlyContinue
        }

        # ---- healthy-render regression: an empty-range render over an edit ---
        # ---- with audio succeeds (render_done) and renders a file.          ---
        $emptyWav = Join-Path $ScenarioRunDir "render_freeze_empty_range.wav"
        $legCStartResp = Invoke-Json -Method POST -Uri $renderInvokeUri -Body @{
            tool   = "render.start"
            args   = @{ file_path = $emptyWav; range = @(10.0, 12.0) }
            source = "dev_agent_smoke.render_freeze"
        } -TimeoutSec 60
        if ([string]$legCStartResp.status -eq "needs_confirmation") {
            $legCStartResp = Invoke-Json -Method POST -Uri $renderInvokeUri -Body @{
                tool      = "render.start"
                args      = @{ file_path = $emptyWav; range = @(10.0, 12.0) }
                source    = "dev_agent_smoke.render_freeze"
                confirmed = $true
            } -TimeoutSec 60
        }
        $legCStartResult = Get-OptionalProperty -Object $legCStartResp -Name "result"
        $emptyJobId = [string](Get-OptionalProperty -Object $legCStartResult -Name "job_id")
        $legCStartStatus = [string](Get-OptionalProperty -Object $legCStartResult -Name "status")
        if ([string]::IsNullOrWhiteSpace($legCStartStatus)) {
            $legCStartStatus = [string]$legCStartResp.status
        }
        if ($legCStartStatus -ne "ok" -or [string]::IsNullOrWhiteSpace($emptyJobId)) {
            throw ("render_freeze regression: empty-range render.start must be accepted (edit has audio): " + ($legCStartResp | ConvertTo-Json -Depth 12 -Compress))
        }

        $legCBindDeadline = (Get-Date).AddSeconds(45)
        $legCReady = $false
        $legCLastJson = ""
        while ((Get-Date) -lt $legCBindDeadline) {
            $legCBindResp = Invoke-JsonTolerant -Uri $renderInvokeUri -Body @{
                tool   = "render.profile.bind"
                args   = @{ render_id = $emptyJobId; profile_id = "builtin:apple_music" }
                source = "dev_agent_smoke.render_freeze"
            } -TimeoutSec 30
            $legCLastJson = ($legCBindResp | ConvertTo-Json -Depth 12 -Compress)
            if ($legCLastJson.Contains("no render telemetry cached")) {
                Start-Sleep -Milliseconds 500
                continue
            }
            if ([string]$legCBindResp.status -eq "ok") {
                $legCReady = $true
                break
            }
            throw ("render_freeze regression: empty-range render did not reach ready status: " + $legCLastJson)
        }
        if (-not $legCReady) {
            throw ("render_freeze regression: render_done telemetry for job " + $emptyJobId + " never reached the agent within 45s (last bind reply: " + $legCLastJson + ")")
        }
        if (-not (Test-Path -LiteralPath $emptyWav) -or ((Get-Item -LiteralPath $emptyWav).Length -le 0)) {
            throw ("render_freeze regression: empty-range render reported done but produced no file: " + $emptyWav)
        }
        Write-Ok ("regression: empty-range render completed with render_done telemetry and a file (job " + $emptyJobId + ")")
    }

    if ($Scenario -eq "journey_first") {
        # JOURNEY-1 first demo journey gate: project open -> authority grant ->
        # one deterministic experiment round. Every assertion sits on a
        # server/kernel-owned surface (AGENTS.md section 8): invoke status,
        # authority mode + the /smoke authority input-chain probe, the B2
        # capability canary chain (proposal card contract + executed stage),
        # the UI projection, and the filesystem session evidence. Zero LLM in
        # the gate; the natural-language free-state face is journey J3 (see
        # coord/runs/JOURNEY-1/JOURNEYS.md).
        Write-Step "Scenario journey_first: demo journey -- project open -> authority -> one experiment round"
        $journeyInvokeUri = $AgentHttp.TrimEnd("/") + "/agent/invoke"
        $journeyChatUri = $AgentHttp.TrimEnd("/") + "/agent/chat"
        $journeyAuthorityUri = $AgentHttp.TrimEnd("/") + "/agent/authority"
        $journeyStateUri = $AgentHttp.TrimEnd("/") + "/agent/state"
        $journeyUiStateUri = $AgentHttp.TrimEnd("/") + "/agent/ui/state"
        $journeyRuntimeUri = $AgentHttp.TrimEnd("/") + "/agent/runtime/status"
        $journeyRespondUri = $AgentHttp.TrimEnd("/") + "/agent/interaction/respond"
        $journeyStamp = Get-Date -Format "yyyyMMdd_HHmmss"
        $journeyInvoke = {
            param([string]$Tool, [object]$ToolArgs, [int]$TimeoutSec)
            Invoke-Json -Method POST -Uri $journeyInvokeUri -Body @{
                tool = $Tool
                args = $ToolArgs
                confirmed = $true
                source = "dev_agent_smoke.journey_first"
            } -TimeoutSec $TimeoutSec
        }

        # ---------------- fixture: self-contained two-stem project ----------
        # WriteLease recipe (VITNOTE-IMPL-4): the two stem names feed the
        # staticbalance name-inference vocabulary and the sustained tones
        # carry the DAD L1 features. Everything lives inside the run dir.
        Write-Step "Journey fixture: build the two-stem project, bake DAD analysis, persist"
        Write-LeaseSmokeStemWav -Path (Join-Path $JourneyStemsDir "Lead Vocal.wav") -Amplitude 0.5 -Frequency 440.0 | Out-Null
        Write-LeaseSmokeStemWav -Path (Join-Path $JourneyStemsDir "Bass.wav") -Amplitude 0.35 -Frequency 110.0 | Out-Null
        $journeyProjectPath = Join-Path $JourneyProjectDir "journey_fixture.vit"
        $null = & $journeyInvoke "project.new" @{} 60
        $journeyImport = & $journeyInvoke "project.import_folder_as_stems" @{
            folder_path = $JourneyStemsDir
            recursive = $false
            target_policy = "create_tracks"
            start_time_seconds = 0.0
            skip_unreadable = $false
            command_timeout_ms = 60000
        } 120
        if ([string]$journeyImport.status -ne "ok") {
            throw ("journey fixture stems import failed: " + ($journeyImport | ConvertTo-Json -Depth 8 -Compress))
        }
        $journeyImportResult = Get-OptionalProperty -Object $journeyImport -Name "result"
        $journeyJobId = [string](Get-FirstPropertyValue -Object $journeyImportResult -Names @("analysis_job_id"))
        if ([string]::IsNullOrWhiteSpace($journeyJobId)) {
            $journeyImportJob = Get-OptionalProperty -Object $journeyImportResult -Name "analysis_job"
            $journeyJobId = [string](Get-FirstPropertyValue -Object $journeyImportJob -Names @("analysis_job_id", "job_id"))
        }
        if ([string]::IsNullOrWhiteSpace($journeyJobId)) {
            throw ("journey fixture import returned no analysis job id: " + ($journeyImport | ConvertTo-Json -Depth 8 -Compress))
        }
        $null = & $journeyInvoke "project.audio_analysis_start" @{ analysis_job_id = $journeyJobId; interval_ms = 10 } 60
        $journeyDadDeadline = (Get-Date).AddSeconds(240)
        $journeyDadReady = $false
        $journeyDadTotal = 0
        while ((Get-Date) -lt $journeyDadDeadline) {
            $journeyDad = & $journeyInvoke "project.audio_analysis_status" @{ analysis_job_id = $journeyJobId; latest = $true } 60
            $journeyDadResult = Get-OptionalProperty -Object $journeyDad -Name "result"
            $journeyDadJob = Get-OptionalProperty -Object $journeyDadResult -Name "analysis_job"
            $journeyDadTotal = [int](Get-FirstPropertyValue -Object $journeyDadJob -Names @("dad_fact_total_count", "dad_fact_total"))
            $journeyDadReadyCount = [int](Get-FirstPropertyValue -Object $journeyDadJob -Names @("dad_fact_ready_count"))
            $journeyDadStatus = [string](Get-FirstPropertyValue -Object $journeyDadJob -Names @("dad_fact_status"))
            $journeyDadWaveforms = @(Get-OptionalProperty -Object $journeyDadJob -Name "track_waveform_envelopes")
            if ($journeyDadTotal -gt 0 -and $journeyDadReadyCount -ge $journeyDadTotal -and $journeyDadStatus.ToLower() -eq "ready" -and $journeyDadWaveforms.Count -ge $journeyDadTotal) {
                $journeyDadReady = $true
                break
            }
            Start-Sleep -Seconds 1
        }
        if (-not $journeyDadReady) {
            throw "journey fixture DAD analysis did not become ready within 240s"
        }
        Write-Ok ("fixture DAD ready (tracks=" + [string]$journeyDadTotal + ")")
        # Persist AFTER the analysis bake: the analysis manifest travels with
        # the edit state (vit_analysis_manifest_json), so the reopen below
        # restores stems and facts together.
        $journeySave = & $journeyInvoke "project.save_as" @{ file_path = $journeyProjectPath } 60
        if ([string]$journeySave.status -ne "ok") {
            throw ("journey fixture save_as failed: " + ($journeySave | ConvertTo-Json -Depth 8 -Compress))
        }
        if (-not (Test-Path -LiteralPath $journeyProjectPath)) {
            throw ("journey fixture project file missing after save_as: " + $journeyProjectPath)
        }
        # Blank untitled project = the demo start-page state the open journey
        # departs from.
        $null = & $journeyInvoke "project.new" @{} 60
        Write-Ok ("fixture persisted: " + $journeyProjectPath)

        # ---------------- LEG 1: project open -------------------------------
        # Same agent-side face the Godot start page drives (journey1 ruling
        # #1): POST /agent/invoke {tool: project.open} -> kernel open_project
        # + applyProjectLifecycle("open") + OpenWorkingSessionAsGeneration.
        Write-Step "LEG 1 (project open): open_project through the agent tool face"
        $journeyOpen = & $journeyInvoke "project.open" @{ file_path = $journeyProjectPath } 180
        $journeyOpen | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath (Join-Path $ScenarioRunDir "journey_open_response.json") -Encoding UTF8
        if ([string]$journeyOpen.status -ne "ok") {
            throw ("journey project.open failed: " + ($journeyOpen | ConvertTo-Json -Depth 8 -Compress))
        }
        Start-Sleep -Seconds 4
        $journeyUiState = Invoke-Json -Method GET -Uri $journeyUiStateUri -TimeoutSec 60
        $journeyUiState | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath (Join-Path $ScenarioRunDir "journey_ui_state_after_open.json") -Encoding UTF8
        $journeyTrackRows = @()
        $journeyTracksProp = Get-OptionalProperty -Object $journeyUiState -Name "tracks"
        if ($null -ne $journeyTracksProp) {
            if ($journeyTracksProp -is [System.Management.Automation.PSCustomObject]) {
                foreach ($journeyTrackProp in $journeyTracksProp.PSObject.Properties) {
                    $journeyTrackRows += $journeyTrackProp.Value
                }
            }
            else {
                $journeyTrackRows = @($journeyTracksProp)
            }
        }
        $journeyTrackNames = New-Object System.Collections.Generic.List[string]
        foreach ($journeyTrackRow in $journeyTrackRows) {
            if ($null -eq $journeyTrackRow) { continue }
            $journeyTrackName = [string](Get-OptionalProperty -Object $journeyTrackRow -Name "name")
            if ([string]::IsNullOrWhiteSpace($journeyTrackName)) {
                $journeyTrackName = [string](Get-OptionalProperty -Object $journeyTrackRow -Name "track_name")
            }
            if (-not [string]::IsNullOrWhiteSpace($journeyTrackName)) { $journeyTrackNames.Add($journeyTrackName) }
        }
        if ($journeyTrackNames.Count -ne 2) {
            throw ("journey open did not restore the two-stem project: tracks=[" + ($journeyTrackNames.ToArray() -join ",") + "]")
        }
        $journeyTracksJoined = $journeyTrackNames.ToArray() -join ","
        if (-not ($journeyTracksJoined.Contains("Lead Vocal")) -or -not ($journeyTracksJoined.Contains("Bass"))) {
            throw ("journey open restored unexpected track names: [" + $journeyTracksJoined + "]")
        }
        Write-Ok ("open restored both stems: [" + $journeyTracksJoined + "]")
        # Project identity: non-empty and stable across two consecutive reads.
        $journeyIdentityA = [string](Get-OptionalProperty -Object (Get-OptionalProperty -Object (Invoke-Json -Method GET -Uri $journeyStateUri -TimeoutSec 10) -Name "shadow") -Name "project_uuid")
        Start-Sleep -Milliseconds 800
        $journeyIdentityB = [string](Get-OptionalProperty -Object (Get-OptionalProperty -Object (Invoke-Json -Method GET -Uri $journeyStateUri -TimeoutSec 10) -Name "shadow") -Name "project_uuid")
        if ([string]::IsNullOrWhiteSpace($journeyIdentityA) -or [string]::IsNullOrWhiteSpace($journeyIdentityB) -or $journeyIdentityA -ne $journeyIdentityB) {
            throw ("journey open left no stable project identity: first=" + $journeyIdentityA + " second=" + $journeyIdentityB)
        }
        Write-Ok ("project identity stable after open: " + $journeyIdentityB)
        # Session evidence: OpenWorkingSessionAsGeneration materializes a
        # session directory beside the project file.
        $journeySessionsRoot = Join-Path $JourneyProjectDir ".vit_history\.sessions"
        $journeySessionDeadline = (Get-Date).AddSeconds(20)
        $journeySessionNames = New-Object System.Collections.Generic.List[string]
        while ((Get-Date) -lt $journeySessionDeadline -and $journeySessionNames.Count -lt 1) {
            if (Test-Path -LiteralPath $journeySessionsRoot) {
                foreach ($journeyUuidDir in @(Get-ChildItem -LiteralPath $journeySessionsRoot -Directory -ErrorAction SilentlyContinue)) {
                    foreach ($journeySessionDir in @(Get-ChildItem -LiteralPath $journeyUuidDir.FullName -Directory -ErrorAction SilentlyContinue)) {
                        $journeySessionNames.Add($journeySessionDir.Name)
                    }
                }
            }
            if ($journeySessionNames.Count -lt 1) { Start-Sleep -Milliseconds 800 }
        }
        if ($journeySessionNames.Count -lt 1) {
            throw ("journey open produced no working-session directory under " + $journeySessionsRoot)
        }
        Write-Ok ("working session materialized: " + ($journeySessionNames.ToArray() -join ","))

        # ---------------- LEG 2: authority grant ----------------------------
        Write-Step "LEG 2 (authority grant): full project access switches and binds the input chain"
        $journeyAuthoritySwitch = Invoke-Json -Method POST -Uri $journeyAuthorityUri -Body @{ authority_mode = "full_project_access" } -TimeoutSec 30
        $journeyAuthoritySwitch | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath (Join-Path $ScenarioRunDir "journey_authority_switch.json") -Encoding UTF8
        if ([string]$journeyAuthoritySwitch.status -ne "ok" -or [string](Get-OptionalProperty -Object $journeyAuthoritySwitch -Name "authority_mode") -ne "full_project_access") {
            throw ("journey authority switch refused: " + ($journeyAuthoritySwitch | ConvertTo-Json -Depth 6 -Compress))
        }
        $journeyHoldDeadline = (Get-Date).AddSeconds(8)
        while ((Get-Date) -lt $journeyHoldDeadline) {
            $null = Invoke-Json -Method GET -Uri $journeyRuntimeUri -TimeoutSec 60
            Start-Sleep -Milliseconds 800
        }
        $journeyAuthorityHeld = Invoke-Json -Method GET -Uri $journeyAuthorityUri -TimeoutSec 60
        if ([string](Get-OptionalProperty -Object $journeyAuthorityHeld -Name "authority_mode") -ne "full_project_access") {
            throw ("journey authority mode flipped after the hold window: " + ($journeyAuthorityHeld | ConvertTo-Json -Depth 6 -Compress))
        }
        Write-Ok "authority held full_project_access across the activation window"
        # Input-chain probe (AUTHORITY-LOST-1 face): server-owned, no LLM.
        $journeyAuthorityProbe = Invoke-Json -Method POST -Uri $journeyChatUri -Body @{
            conversation_id = ("dev_journey_first_authority_" + $journeyStamp)
            message = "/smoke authority"
            context = @{
                agent_mode = "chat"
            }
        } -TimeoutSec ([Math]::Max(30, $WaitSeconds))
        $journeyAuthorityProbe | ConvertTo-Json -Depth 12 | Set-Content -LiteralPath (Join-Path $ScenarioRunDir "journey_authority_probe.json") -Encoding UTF8
        if ([string](Get-OptionalProperty -Object $journeyAuthorityProbe -Name "stop_reason") -ne "authority_smoke_ok" -or -not (([string](Get-OptionalProperty -Object $journeyAuthorityProbe -Name "reply")).Contains("bound=full_project_access"))) {
            throw ("journey authority input-chain probe did not bind full access: " + ($journeyAuthorityProbe | ConvertTo-Json -Depth 8 -Compress))
        }
        if ([bool](Get-OptionalProperty -Object $journeyAuthorityProbe -Name "needs_confirmation")) {
            throw "journey authority probe raised a confirmation card under full access"
        }
        Write-Ok "input chain binds full_project_access with no confirmation card"

        # ---------------- LEG 3: one experiment round -----------------------
        # B2 capability chain (static_mix.static_balance.v0): propose ->
        # approval card -> approve -> execute inside the project write lease
        # -> executed_verified. Entirely server-owned; readiness lag is the
        # only declared retryable class.
        Write-Step "LEG 3 (experiment round): B2 static balance propose -> approve -> execute-verify"
        $journeyMixSession = "dev_journey_first_" + $journeyStamp
        $null = & $journeyInvoke "mix.observe" @{
            scope = "full_project"
            project_context = $true
            observation_only = $true
            observation_ready_gate = $true
            disclosure = "digest_catalog"
            mom_intent = "action_preflight_observation"
            mix_session_id = $journeyMixSession
            goal_text = "journey first fixture L3 preflight"
        } 240
        $journeyMomDeadline = (Get-Date).AddSeconds(240)
        $journeyMomReady = $false
        while ((Get-Date) -lt $journeyMomDeadline) {
            $journeyMom = & $journeyInvoke "mix.observe" @{
                scope = "full_project"
                project_context = $true
                observation_only = $true
                disclosure = "digest_catalog"
                mom_intent = "project_multitrack_relation_observation"
                mix_session_id = $journeyMixSession
                goal_text = "journey first fixture readiness"
            } 240
            $journeyMomResult = Get-OptionalProperty -Object $journeyMom -Name "result"
            $journeyMomProjection = Get-OptionalProperty -Object $journeyMomResult -Name "mom_projection"
            $journeyMomRelation = Get-OptionalProperty -Object $journeyMomProjection -Name "multitrack_relation"
            $journeyMomStatus = [string](Get-OptionalProperty -Object $journeyMomRelation -Name "status")
            $journeyMomStatic = Get-OptionalProperty -Object $journeyMomProjection -Name "static_level_relationship"
            $journeyMomStaticStatus = [string](Get-OptionalProperty -Object $journeyMomStatic -Name "status")
            if ($journeyMomStatus.ToLower() -eq "ready" -and $journeyMomStaticStatus.ToLower() -eq "ready") {
                $journeyMomReady = $true
                break
            }
            Start-Sleep -Seconds 1
        }
        if (-not $journeyMomReady) {
            throw "journey fixture MOM multitrack/static_level relations did not become ready within 240s"
        }
        Write-Ok "fixture MOM multitrack + static_level relations ready"

        # ASCII-safe Chinese propose sentence (PS 5.1 BOM-less rule):
        # "please generate and execute one B2 static balance for the current
        #  isolated test project"
        $journeyProposeMessage = -join @(
            [char]0x8BF7, [char]0x4E3A, [char]0x5F53, [char]0x524D, [char]0x9694, [char]0x79BB,
            [char]0x6D4B, [char]0x8BD5, [char]0x5DE5, [char]0x7A0B, [char]0x751F, [char]0x6210,
            [char]0x5E76, [char]0x6267, [char]0x884C, ' ', 'B', '2', ' ',
            [char]0x9759, [char]0x6001, [char]0x5E73, [char]0x8861, [char]0x3002
        )
        $journeyProposeContext = @{
            agent_mode = "chat"
            capability_id = "static_mix.static_balance.v0"
            interaction_mode = "propose"
            capability_runtime_v1 = $true
        }
        $journeyProposal = $null
        $journeyProbeCount = 0
        $journeyReadinessReady = $false
        while (-not $journeyReadinessReady -and $journeyProbeCount -lt 10) {
            $journeyProbeCount++
            $journeyProposal = Invoke-Json -Method POST -Uri $journeyChatUri -Body @{
                conversation_id = ("dev_journey_first_probe_" + $journeyStamp + "_" + [string]$journeyProbeCount)
                message = $journeyProposeMessage
                context = $journeyProposeContext
            } -TimeoutSec 240
            $journeyProbeData = Get-OptionalProperty -Object $journeyProposal -Name "workflow_data"
            $journeyProbeStage = [string](Get-OptionalProperty -Object $journeyProbeData -Name "canary_stage")
            if ($journeyProbeStage -eq "proposal") {
                $journeyReadinessReady = $true
                break
            }
            if ($journeyProbeStage -ne "readiness_blocked") {
                throw ("journey propose hit unexpected canary_stage " + $journeyProbeStage + ": " + ($journeyProposal | ConvertTo-Json -Depth 8 -Compress))
            }
            Write-WarnLine ("journey readiness probe " + [string]$journeyProbeCount + " still blocked: " + ((Get-OptionalProperty -Object $journeyProbeData -Name "blockers") -join ","))
            Start-Sleep -Seconds 3
        }
        if (-not $journeyReadinessReady) {
            throw ("journey B2 readiness did not unlock after " + [string]$journeyProbeCount + " probes")
        }
        $journeyProposal | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath (Join-Path $ScenarioRunDir "journey_propose_response.json") -Encoding UTF8
        $journeyProposalData = Get-OptionalProperty -Object $journeyProposal -Name "workflow_data"
        $journeySessionID = [string](Get-OptionalProperty -Object $journeyProposalData -Name "session_id")
        $journeyInteractionID = ""
        foreach ($journeyInteractionRow in @(Get-OptionalProperty -Object $journeyProposal -Name "interaction_requests")) {
            $journeyRowKind = [string](Get-OptionalProperty -Object $journeyInteractionRow -Name "kind")
            $journeyRowWorkflow = [string](Get-OptionalProperty -Object $journeyInteractionRow -Name "workflow")
            if (($journeyRowKind.ToLower() -in @("proposal_approval", "confirmation")) -and $journeyRowWorkflow.ToLower() -eq "capability_runtime_v1") {
                $journeyInteractionID = [string](Get-OptionalProperty -Object $journeyInteractionRow -Name "id")
                break
            }
        }
        if (-not [bool](Get-OptionalProperty -Object $journeyProposal -Name "needs_confirmation") -or [string](Get-OptionalProperty -Object $journeyProposal -Name "workflow") -ne "capability_runtime_v1" -or [string](Get-OptionalProperty -Object $journeyProposalData -Name "canary_stage") -ne "proposal" -or [string]::IsNullOrWhiteSpace($journeySessionID) -or [string]::IsNullOrWhiteSpace($journeyInteractionID)) {
            throw ("journey proposal card contract failed: " + ($journeyProposal | ConvertTo-Json -Depth 8 -Compress))
        }
        Write-Ok ("B2 proposal card mounted (session=" + $journeySessionID + ", stage=proposal)")

        $journeyApprove = Invoke-Json -Method POST -Uri $journeyRespondUri -Body @{
            interaction_id = $journeyInteractionID
            action_id = "approve"
            decision = "approve"
            payload = @{}
        } -TimeoutSec 300
        $journeyApprove | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath (Join-Path $ScenarioRunDir "journey_approve_response.json") -Encoding UTF8
        $journeyExecutedStages = @("executed_verified", "executed_needs_review")
        $journeyApproveStage = [string](Get-OptionalProperty -Object (Get-OptionalProperty -Object $journeyApprove -Name "workflow_data") -Name "canary_stage")
        if (-not ($journeyExecutedStages -contains $journeyApproveStage)) {
            throw ("journey B2 execution did not reach an executed stage: canary_stage=" + $journeyApproveStage + " response=" + ($journeyApprove | ConvertTo-Json -Depth 8 -Compress))
        }
        Write-Ok ("B2 experiment round executed: canary_stage=" + $journeyApproveStage)

        # Readback face: a post-execution observation must answer ok (the
        # chain's re-observation leg).
        $journeyReadback = & $journeyInvoke "mix.observe" @{
            scope = "full_project"
            project_context = $true
            observation_only = $true
            disclosure = "digest_catalog"
            mom_intent = "project_multitrack_relation_observation"
            mix_session_id = $journeyMixSession
            goal_text = "journey first post-execution readback"
        } 240
        $journeyReadback | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath (Join-Path $ScenarioRunDir "journey_readback_observe.json") -Encoding UTF8
        if ([string]$journeyReadback.status -ne "ok") {
            throw ("journey post-execution observation failed: " + ($journeyReadback | ConvertTo-Json -Depth 8 -Compress))
        }
        Write-Ok "post-execution observation readback ok"

        $journeyFinalState = Invoke-Json -Method GET -Uri $journeyStateUri -TimeoutSec 10
        if ($null -eq $journeyFinalState -or [string]$journeyFinalState.status -ne "ok") {
            throw "journey final /agent/state did not return ok"
        }
        Write-Ok ("stack healthy after the journey (tool_count=" + [string]$journeyFinalState.tool_count + ")")

        $journeySummary = @{
            journey = "journey_first"
            project_path = $journeyProjectPath
            track_names = @($journeyTrackNames.ToArray())
            project_uuid = $journeyIdentityB
            session_dirs = @($journeySessionNames.ToArray())
            authority_probe = [string](Get-OptionalProperty -Object $journeyAuthorityProbe -Name "stop_reason")
            propose_session = $journeySessionID
            propose_probes = $journeyProbeCount
            approve_stage = $journeyApproveStage
            finished_at = (Get-Date).ToString("o")
        }
        $journeySummary | ConvertTo-Json -Depth 6 | Set-Content -LiteralPath (Join-Path $ScenarioRunDir "journey_summary.json") -Encoding UTF8
    }

        $scenarioPassed = $true
        $scenarioSummary = @{
            scenario = $Scenario
            outcome = "pass"
            finished_at = (Get-Date).ToString("o")
            artifacts_dir = $ScenarioRunDir
        }
        # range_split_proposal_source is range-split specific; guard the
        # reference so non-range scenarios do not trip StrictMode on an
        # undefined variable.
        if (Test-Path Variable:proposalSource) {
            $scenarioSummary.range_split_proposal_source = $proposalSource
        }
        $scenarioSummary | ConvertTo-Json -Depth 6 | Set-Content -LiteralPath (Join-Path $ScenarioRunDir "summary.json") -Encoding UTF8
    }
    finally {
        # Berth teardown: the stack this run started is torn down, best effort,
        # on every path (pass, assertion failure, environment failure). A
        # teardown failure is reported loudly but does not flip a passing gate.
        Write-Step "Scenario berth teardown"
        foreach ($teardownTarget in @(
            @{ name = "agent"; procId = $ScenarioAgentProcId },
            @{ name = "kernel"; procId = $ScenarioKernelProcId }
        )) {
            if ($null -ne $teardownTarget.procId -and $teardownTarget.procId -gt 0) {
                try {
                    Stop-Process -Id $teardownTarget.procId -Force -ErrorAction Stop
                    Write-Ok ("stopped scenario " + $teardownTarget.name + " pid=" + $teardownTarget.procId)
                }
                catch {
                    Write-WarnLine ("could not stop scenario " + $teardownTarget.name + " pid=" + $teardownTarget.procId + ": " + $_.Exception.Message)
                }
            }
        }
    }

    Write-Step "Summary"
    Write-Ok ("dev agent smoke completed (scenario=" + $Scenario + ", artifacts=" + $ScenarioRunDir + ")")
    return
}

if ($AuthoritySmoke) {
    # AUTHORITY-LOST-1: switch full access, hold it across activation/reload
    # surfaces, then prove the input chain still binds full access with no
    # confirmation card. The deterministic probe is the /smoke authority chat
    # command (server-owned authority, no LLM randomness in the assertion).
    Write-Step "Authority lifecycle smoke"
    $authorityBefore = Invoke-Json -Method GET -Uri ($AgentHttp.TrimEnd("/") + "/agent/authority") -TimeoutSec 60
    Write-Host ("authority before: " + [string]$authorityBefore.authority_mode)
    if ($null -eq $authorityBefore -or $authorityBefore.status -ne "ok") {
        throw "GET /agent/authority did not return ok"
    }
    $authoritySwitch = Invoke-Json -Method POST -Uri ($AgentHttp.TrimEnd("/") + "/agent/authority") -Body @{
        authority_mode = "full_project_access"
    } -TimeoutSec 15
    if ($null -eq $authoritySwitch -or $authoritySwitch.status -ne "ok" -or [string]$authoritySwitch.authority_mode -ne "full_project_access") {
        throw ("authority switch failed: " + ($authoritySwitch | ConvertTo-Json -Depth 6 -Compress))
    }
    Write-Ok "authority switched to full_project_access"
    # Hold window: activation requests plus a scheduler-tick-sized wait must
    # not flip the in-memory mode back to a stale persisted value.
    $holdDeadline = (Get-Date).AddSeconds(8)
    while ((Get-Date) -lt $holdDeadline) {
        $null = Invoke-Json -Method GET -Uri ($AgentHttp.TrimEnd("/") + "/agent/runtime/status") -TimeoutSec 60
        Start-Sleep -Milliseconds 800
    }
    $authorityHeld = Invoke-Json -Method GET -Uri ($AgentHttp.TrimEnd("/") + "/agent/authority") -TimeoutSec 60
    if ([string]$authorityHeld.authority_mode -ne "full_project_access") {
        throw ("authority mode flipped after hold window: " + [string]$authorityHeld.authority_mode)
    }
    Write-Ok "authority held full_project_access across activation window"
    $authorityConversation = "dev_authority_smoke_" + (Get-Date -Format "yyyyMMdd_HHmmss")
    $authorityChat = Invoke-Json -Method POST -Uri ($AgentHttp.TrimEnd("/") + "/agent/chat") -Body @{
        conversation_id = $authorityConversation
        message = "/smoke authority"
        context = @{
            agent_mode = "chat"
        }
    } -TimeoutSec ([Math]::Max(30, $WaitSeconds))
    Write-Host ("authority chat stop_reason=" + [string]$authorityChat.stop_reason + " reply=" + [string]$authorityChat.reply)
    if ([string]$authorityChat.stop_reason -ne "authority_smoke_ok" -or -not (([string]$authorityChat.reply).Contains("bound=full_project_access"))) {
        throw ("authority input-chain probe did not bind full access: " + ($authorityChat | ConvertTo-Json -Depth 8 -Compress))
    }
    if ([bool]$authorityChat.needs_confirmation) {
        throw "authority smoke turn raised a confirmation card under full access"
    }
    Write-Ok "input chain binds full_project_access with no confirmation card"
    if (-not [string]::IsNullOrWhiteSpace($AgentLog) -and (Test-Path -LiteralPath $AgentLog)) {
        $authorityLogLines = @(Select-String -LiteralPath $AgentLog -Pattern "\[authority\] switch accepted" -ErrorAction SilentlyContinue)
        if ($authorityLogLines.Count -lt 1) {
            Fail-Or-Warn "authority switch log anchor missing from agent_last.log (log may have rotated)"
        }
        else {
            Write-Ok ("authority switch log anchor present (" + [string]$authorityLogLines.Count + " lines)")
        }
    }
    # Restore the pre-smoke mode so the shared workspace keeps its prior state.
    $authorityRestore = Invoke-Json -Method POST -Uri ($AgentHttp.TrimEnd("/") + "/agent/authority") -Body @{
        authority_mode = [string]$authorityBefore.authority_mode
    } -TimeoutSec 60
    if ($null -eq $authorityRestore -or $authorityRestore.status -ne "ok") {
        throw "failed to restore authority mode after smoke"
    }
    Write-Ok ("authority restored to " + [string]$authorityRestore.authority_mode)
}

Write-Step "Strip Silence tool catalog"
$toolsResp = Invoke-Json -Method GET -Uri ($AgentHttp.TrimEnd("/") + "/agent/tools") -TimeoutSec 10
if ($null -eq $toolsResp -or [string]$toolsResp.status -ne "ok") {
    throw "GET /agent/tools did not return ok"
}
$toolNames = @()
foreach ($tool in @($toolsResp.tools)) {
    $name = [string](Get-OptionalProperty -Object $tool -Name "name")
    if (-not [string]::IsNullOrWhiteSpace($name)) {
        $toolNames += $name
    }
}
foreach ($wantTool in @("clip.strip_silence.analyze", "clip.strip_silence.suggest", "clip.strip_silence.apply", "clip.strip_silence.apply_batch")) {
    if (-not ($toolNames -contains $wantTool)) {
        throw ("missing Strip Silence tool in /agent/tools: " + $wantTool)
    }
}
Write-Ok "Strip Silence tools are advertised"

$priorUIContextResp = Invoke-Json -Method GET -Uri ($AgentHttp.TrimEnd("/") + "/agent/ui/context") -TimeoutSec 10
$priorUIContext = Get-OptionalProperty -Object $priorUIContextResp -Name "context"
if ($null -eq $priorUIContext) {
    $priorUIContext = @{}
}
try {
    Write-Step "Range context smoke"
    $rangeContext = @{
    selected_clip_ranges = @(
        @{
            range_id = "dev_smoke_range_1"
            clip_id = "dev_smoke_clip_1"
            track_id = "dev_smoke_track_1"
            start_seconds = 2.0
            end_seconds = 3.5
            duration_seconds = 1.5
            clip_local_start_seconds = 0.5
            clip_local_end_seconds = 2.0
        },
        @{
            range_id = "dev_smoke_range_2"
            clip_id = "dev_smoke_clip_2"
            track_id = "dev_smoke_track_1"
            start_seconds = 8.0
            end_seconds = 9.25
            duration_seconds = 1.25
            clip_local_start_seconds = 0.0
            clip_local_end_seconds = 1.25
        }
    )
    selected_clip_range_count = 2
}
    $rangeContextResp = Invoke-Json -Method POST -Uri ($AgentHttp.TrimEnd("/") + "/agent/ui/context") -Body $rangeContext -TimeoutSec 10
    if ($null -eq $rangeContextResp -or [string]$rangeContextResp.status -ne "ok") {
        throw "POST /agent/ui/context failed for range context smoke"
    }
    $rangeContextRows = @($rangeContextResp.context.selected_clip_ranges)
    if ($rangeContextRows.Count -ne 2) {
        throw ("range context was not preserved by /agent/ui/context; count=" + [string]$rangeContextRows.Count)
    }
    $rangeChat = Invoke-Json -Method POST -Uri ($AgentHttp.TrimEnd("/") + "/agent/chat") -Body @{
    conversation_id = "dev_range_context_" + (Get-Date -Format "yyyyMMdd_HHmmss")
    message = "/smoke range_context"
    context = @{
        agent_mode = "chat"
    }
    } -TimeoutSec ([Math]::Max(30, $WaitSeconds))
    Write-Host ("range_context.stop_reason=" + [string]$rangeChat.stop_reason)
    Write-Host ("range_context.reply=" + [string]$rangeChat.reply)
    if ([string]$rangeChat.stop_reason -ne "range_context_smoke_ok") {
        throw ("range context chat smoke failed: " + ($rangeChat | ConvertTo-Json -Depth 12 -Compress))
    }
    Write-Ok "range context survives UI context and chat current_selection merge"
}
finally {
    $restoreUIContextResp = Invoke-Json -Method POST -Uri ($AgentHttp.TrimEnd("/") + "/agent/ui/context") -Body $priorUIContext -TimeoutSec 10
    if ($null -eq $restoreUIContextResp -or [string]$restoreUIContextResp.status -ne "ok") {
        throw "failed to restore UI context after range context smoke"
    }
}

Write-Step "Kernel ports"
$req = Get-TcpListener -Port ([int]$ZmqReqPort)
$sub = Get-TcpListener -Port ([int]$ZmqSubPort)
if ($null -eq $req) {
    Fail-Or-Warn ("ZMQ REQ port not listening: " + $ZmqReqPort)
}
else {
    Write-Ok ("ZMQ REQ listening: " + $ZmqReqPort + " pid=" + $req.OwningProcess)
}
if ($null -eq $sub) {
    Fail-Or-Warn ("ZMQ SUB port not listening: " + $ZmqSubPort)
}
else {
    Write-Ok ("ZMQ SUB listening: " + $ZmqSubPort + " pid=" + $sub.OwningProcess)
}

if ($null -ne $req) {
    Write-Step "Project state invoke"
    $projectState = Invoke-Json -Method POST -Uri ($AgentHttp.TrimEnd("/") + "/agent/invoke") -Body @{
        tool = "project.state"
        args = @{}
        source = "dev_agent_smoke"
    } -TimeoutSec ([Math]::Max(20, $WaitSeconds))
    $projectStateStatus = [string](Get-OptionalProperty -Object $projectState -Name "status")
    $projectStateError = [string](Get-OptionalProperty -Object $projectState -Name "error")
    Write-Host ("project.state status=" + $projectStateStatus + " error=" + $projectStateError)
    if ($projectStateStatus -ne "ok") {
        Fail-Or-Warn ("project.state failed")
    }

    if (-not $NoStripSilenceSmoke) {
        Write-Step "Strip Silence agent invoke smoke"
        if ($projectStateStatus -ne "ok") {
            Fail-Or-Warn "skipping Strip Silence invoke smoke because project.state failed"
        }
        else {
        $trackID = Get-FirstVisibleTrackId -ProjectState $projectState -AgentState $state
        $createdSmokeTrackID = ""
        if ([string]::IsNullOrWhiteSpace($trackID)) {
            Write-WarnLine "no visible track found; creating a temporary smoke track"
            $trackResp = Invoke-Json -Method POST -Uri ($AgentHttp.TrimEnd("/") + "/agent/invoke") -Body @{
                tool = "track.add"
                args = @{}
                confirmed = $true
                source = "dev_agent_smoke.strip_silence_track"
            } -TimeoutSec 30
            $trackStatus = [string](Get-OptionalProperty -Object $trackResp -Name "status")
            $trackResult = Get-OptionalProperty -Object $trackResp -Name "result"
            $trackID = [string](Get-FirstPropertyValue -Object $trackResult -Names @("track_id", "id", "item_id"))
            Write-Host ("strip.track.add status=" + $trackStatus + " track_id=" + $trackID + " error=" + [string](Get-OptionalProperty -Object $trackResp -Name "error"))
            if ($trackStatus -ne "ok" -or [string]::IsNullOrWhiteSpace($trackID)) {
                Fail-Or-Warn "Strip Silence smoke could not create a temporary track"
            }
            else {
                $createdSmokeTrackID = $trackID
            }
        }
        if (-not [string]::IsNullOrWhiteSpace($trackID)) {
            $stripWavName = "strip_silence_agent_smoke_{0}_{1}.wav" -f $PID, ([DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds())
            $stripWav = Write-StripSilenceTestWav -Path (Join-Path (Join-Path $WorkspaceDir "Artifacts\smoke") $stripWavName)
            $importResp = Invoke-Json -Method POST -Uri ($AgentHttp.TrimEnd("/") + "/agent/invoke") -Body @{
                tool = "clip.import_media_to_track"
                args = @{
                    track_id = $trackID
                    file_path = $stripWav
                    start_time = 0.0
                    media_type = "audio"
                    mode = "non_destructive"
                }
                confirmed = $true
                source = "dev_agent_smoke.strip_silence"
            } -TimeoutSec 60
            $importStatus = [string](Get-OptionalProperty -Object $importResp -Name "status")
            Write-Host ("strip.import status=" + $importStatus + " error=" + [string](Get-OptionalProperty -Object $importResp -Name "error"))
            if ($importStatus -ne "ok") {
                Fail-Or-Warn "Strip Silence smoke import failed"
            }
            else {
                Start-Sleep -Milliseconds 500
                $afterImportState = Invoke-Json -Method POST -Uri ($AgentHttp.TrimEnd("/") + "/agent/invoke") -Body @{
                    tool = "project.state"
                    args = @{}
                    source = "dev_agent_smoke.strip_silence"
                } -TimeoutSec 20
                $clipID = Get-ImportedClipId -ImportResponse $importResp -ProjectState $afterImportState -FilePath $stripWav
                if ([string]::IsNullOrWhiteSpace($clipID)) {
                    Fail-Or-Warn "Strip Silence smoke could not resolve imported clip id"
                }
                else {
                    Write-Host ("strip.clip_id=" + $clipID + " track_id=" + $trackID)
                    $stripChatBefore = Count-MixPlannerChatEvents -LogPath $MixDiagLog
                    $stripChat = Invoke-Json -Method POST -Uri ($AgentHttp.TrimEnd("/") + "/agent/chat") -Body @{
                        conversation_id = "dev_strip_silence_chat_" + (Get-Date -Format "yyyyMMdd_HHmmss")
                        message = "suggest strip silence cleanup parameters for the current selected audio clip and estimate threshold"
                        context = @{
                            agent_mode = "chat"
                            current_selection = @{
                                selected_clip_id = $clipID
                                selected_clip_track_id = $trackID
                            }
                        }
                    } -TimeoutSec 90
                    $stripChatAfter = Count-MixPlannerChatEvents -LogPath $MixDiagLog
                    $stripChatReply = [string](Get-OptionalProperty -Object $stripChat -Name "reply")
                    $stripChatStopReason = [string](Get-OptionalProperty -Object $stripChat -Name "stop_reason")
                    $stripChatNeedsConfirmation = [string](Get-OptionalProperty -Object $stripChat -Name "needs_confirmation")
                    $stripChatConfirmed = $false
                    Write-Host ("strip.chat.stop_reason=" + $stripChatStopReason)
                    Write-Host ("strip.chat.reply=" + $stripChatReply)
                    if ($stripChatAfter -ne $stripChatBefore) {
                        Fail-Or-Warn ("Strip Silence chat route touched mix_planner_chat log count " + $stripChatBefore + " -> " + $stripChatAfter)
                    }
                    elseif (($stripChatReply -notmatch "dBFS") -and ($stripChatStopReason -ne "needs_confirmation") -and ($stripChatNeedsConfirmation -ne "True")) {
                        Fail-Or-Warn "Strip Silence chat route did not return a parameter suggestion or pending confirmation"
                    }
                    else {
                        Write-Ok "Strip Silence chat route used deterministic suggest without legacy mix routing"
                    }
                    $stripChatPlanID = [string](Get-OptionalProperty -Object $stripChat -Name "plan_id")
                    if ((($stripChatStopReason -eq "needs_confirmation") -or ($stripChatNeedsConfirmation -eq "True")) -and -not [string]::IsNullOrWhiteSpace($stripChatPlanID)) {
                        $stripConfirmResp = Invoke-Json -Method POST -Uri ($AgentHttp.TrimEnd("/") + "/agent/confirm") -Body @{
                            plan_id = $stripChatPlanID
                            decision = "approve"
                        } -TimeoutSec 90
                        $stripConfirmStatus = [string](Get-OptionalProperty -Object $stripConfirmResp -Name "status")
                        $stripConfirmGoalStatus = [string](Get-OptionalProperty -Object $stripConfirmResp -Name "goal_status")
                        $stripConfirmMessage = [string](Get-OptionalProperty -Object $stripConfirmResp -Name "message")
                        Write-Host ("strip.chat.confirm status=" + $stripConfirmStatus + " goal_status=" + $stripConfirmGoalStatus + " message=" + $stripConfirmMessage)
                        if ($stripConfirmStatus -ne "ok" -or $stripConfirmGoalStatus -ne "completed") {
                            Fail-Or-Warn "Strip Silence chat confirmation did not complete"
                        }
                        else {
                            $stripChatConfirmed = $true
                            Write-Ok "Strip Silence chat confirmation executed apply"
                            $cleanupIds = @()
                            $cleanupIds += $clipID
                            foreach ($reply in @(Get-OptionalProperty -Object $stripConfirmResp -Name "executed_kernel_reply")) {
                                $result = Get-OptionalProperty -Object $reply -Name "result"
                                foreach ($key in @("clip_id", "created_clip_ids", "affected_clip_ids")) {
                                    $vals = @(Get-OptionalProperty -Object $result -Name $key)
                                    foreach ($val in $vals) {
                                        if (-not [string]::IsNullOrWhiteSpace([string]$val)) {
                                            $cleanupIds += [string]$val
                                        }
                                    }
                                }
                            }
                            $cleanupIds = @($cleanupIds | Select-Object -Unique)
                            if ($cleanupIds.Count -gt 0) {
                                $cleanupResp = Invoke-Json -Method POST -Uri ($AgentHttp.TrimEnd("/") + "/agent/invoke") -Body @{
                                    tool = "clip.remove"
                                    args = @{
                                        clip_ids = $cleanupIds
                                    }
                                    confirmed = $true
                                    source = "dev_agent_smoke.strip_silence_chat_cleanup"
                                } -TimeoutSec 60
                                Write-Host ("strip.chat.cleanup status=" + [string](Get-OptionalProperty -Object $cleanupResp -Name "status") + " ids=" + ($cleanupIds -join ","))
                            }
                            if (-not [string]::IsNullOrWhiteSpace($createdSmokeTrackID)) {
                                $cleanupState = Invoke-Json -Method POST -Uri ($AgentHttp.TrimEnd("/") + "/agent/invoke") -Body @{
                                    tool = "project.state"
                                    args = @{}
                                    source = "dev_agent_smoke.strip_silence_track_cleanup"
                                } -TimeoutSec 20
                                $cleanupResult = Get-OptionalProperty -Object $cleanupState -Name "result"
                                $userTrackCount = [int](Get-OptionalProperty -Object $cleanupResult -Name "user_track_count")
                                if ($userTrackCount -le 1) {
                                    Write-WarnLine ("leaving temporary smoke track because kernel refuses deleting the last audio track: " + $createdSmokeTrackID)
                                }
                                else {
                                    $trackCleanupResp = Invoke-Json -Method POST -Uri ($AgentHttp.TrimEnd("/") + "/agent/invoke") -Body @{
                                        tool = "track.delete"
                                        args = @{
                                            track_id = $createdSmokeTrackID
                                        }
                                        confirmed = $true
                                        source = "dev_agent_smoke.strip_silence_track_cleanup"
                                    } -TimeoutSec 60
                                    Write-Host ("strip.track.cleanup status=" + [string](Get-OptionalProperty -Object $trackCleanupResp -Name "status") + " track_id=" + $createdSmokeTrackID)
                                }
                            }
                        }
                    }
                    if (-not $stripChatConfirmed) {
                    $suggestResp = Invoke-Json -Method POST -Uri ($AgentHttp.TrimEnd("/") + "/agent/invoke") -Body @{
                        tool = "clip.strip_silence.suggest"
                        args = @{
                            clip_id = $clipID
                            track_id = $trackID
                            candidate_thresholds_dbfs = @(-60.0, -54.0, -48.0, -42.0)
                            min_silence_ms = 120.0
                            clip_start_pad_ms = 5.0
                            clip_end_pad_ms = 5.0
                            scope = "whole_clip"
                        }
                        source = "dev_agent_smoke.strip_silence"
                    } -TimeoutSec 60
                    $suggestStatus = [string](Get-OptionalProperty -Object $suggestResp -Name "status")
                    $suggestResult = Get-OptionalProperty -Object $suggestResp -Name "result"
                    $suggestAnalysis = Get-OptionalProperty -Object $suggestResult -Name "analysis"
                    $stripCount = [int](Get-OptionalProperty -Object $suggestAnalysis -Name "strip_region_count")
                    $pendingAction = Get-OptionalProperty -Object $suggestResult -Name "pending_action"
                    if ($null -eq $pendingAction -or [string]::IsNullOrWhiteSpace([string]$pendingAction)) {
                        $pendingActions = @(Get-OptionalProperty -Object $suggestResult -Name "pending_actions")
                        if ($pendingActions.Count -gt 0) {
                            $pendingAction = $pendingActions[0]
                        }
                    }
                    $pendingArgs = Get-OptionalProperty -Object $pendingAction -Name "args"
                    $stripRegions = @(Get-OptionalProperty -Object $pendingArgs -Name "strip_regions")
                    if ($stripRegions.Count -eq 1 -and [string]::IsNullOrWhiteSpace([string]$stripRegions[0])) {
                        $stripRegions = @()
                    }
                    Write-Host ("strip.suggest status=" + $suggestStatus + " regions=" + [string]$stripCount + " pending=" + [string](Get-OptionalProperty -Object $suggestResult -Name "pending_action_count") + " error=" + [string](Get-OptionalProperty -Object $suggestResp -Name "error"))
                    if ($suggestStatus -ne "ok" -or $stripCount -lt 1 -or $stripRegions.Count -lt 1) {
                        Fail-Or-Warn "Strip Silence suggest did not produce expected pending apply regions"
                    }
                    else {
                        $applyResp = Invoke-Json -Method POST -Uri ($AgentHttp.TrimEnd("/") + "/agent/invoke") -Body @{
                            tool = "clip.strip_silence.apply"
                            args = @{
                                clip_id = [string](Get-OptionalProperty -Object $pendingArgs -Name "clip_id")
                                track_id = [string](Get-OptionalProperty -Object $pendingArgs -Name "track_id")
                                analysis_id = [string](Get-OptionalProperty -Object $pendingArgs -Name "analysis_id")
                                strip_regions = @($stripRegions)
                                allow_remove_entire_clip = $false
                            }
                            confirmed = $true
                            source = "dev_agent_smoke.strip_silence"
                        } -TimeoutSec 60
                        $applyStatus = [string](Get-OptionalProperty -Object $applyResp -Name "status")
                        $applyResult = Get-OptionalProperty -Object $applyResp -Name "result"
                        Write-Host ("strip.apply status=" + $applyStatus + " applied=" + [string](Get-OptionalProperty -Object $applyResult -Name "applied_region_count") + " error=" + [string](Get-OptionalProperty -Object $applyResp -Name "error"))
                        if ($applyStatus -ne "ok") {
                            Fail-Or-Warn "Strip Silence apply failed"
                        }
                        else {
                            Write-Ok "Strip Silence suggest/apply passed through Agent invoke"
                            $cleanupIds = @()
                            $cleanupIds += $clipID
                            foreach ($key in @("created_clip_ids", "affected_clip_ids")) {
                                $vals = @(Get-OptionalProperty -Object $applyResult -Name $key)
                                foreach ($val in $vals) {
                                    if (-not [string]::IsNullOrWhiteSpace([string]$val)) {
                                        $cleanupIds += [string]$val
                                    }
                                }
                            }
                            $cleanupIds = @($cleanupIds | Select-Object -Unique)
                            if ($cleanupIds.Count -gt 0) {
                                $cleanupResp = Invoke-Json -Method POST -Uri ($AgentHttp.TrimEnd("/") + "/agent/invoke") -Body @{
                                    tool = "clip.remove"
                                    args = @{
                                        clip_ids = $cleanupIds
                                    }
                                    confirmed = $true
                                    source = "dev_agent_smoke.strip_silence_cleanup"
                                } -TimeoutSec 60
                                Write-Host ("strip.cleanup status=" + [string](Get-OptionalProperty -Object $cleanupResp -Name "status") + " ids=" + ($cleanupIds -join ","))
                            }
                            if (-not [string]::IsNullOrWhiteSpace($createdSmokeTrackID)) {
                                $cleanupState = Invoke-Json -Method POST -Uri ($AgentHttp.TrimEnd("/") + "/agent/invoke") -Body @{
                                    tool = "project.state"
                                    args = @{}
                                    source = "dev_agent_smoke.strip_silence_track_cleanup"
                                } -TimeoutSec 20
                                $cleanupResult = Get-OptionalProperty -Object $cleanupState -Name "result"
                                $userTrackCount = [int](Get-OptionalProperty -Object $cleanupResult -Name "user_track_count")
                                if ($userTrackCount -le 1) {
                                    Write-WarnLine ("leaving temporary smoke track because kernel refuses deleting the last audio track: " + $createdSmokeTrackID)
                                }
                                else {
                                    $trackCleanupResp = Invoke-Json -Method POST -Uri ($AgentHttp.TrimEnd("/") + "/agent/invoke") -Body @{
                                        tool = "track.delete"
                                        args = @{
                                            track_id = $createdSmokeTrackID
                                        }
                                        confirmed = $true
                                        source = "dev_agent_smoke.strip_silence_track_cleanup"
                                    } -TimeoutSec 60
                                    Write-Host ("strip.track.cleanup status=" + [string](Get-OptionalProperty -Object $trackCleanupResp -Name "status") + " track_id=" + $createdSmokeTrackID)
                                }
                            }
                        }
                    }
                    }
                }
            }
        }
    }
    }

    if ($RefQuerySmoke) {
        Write-Step "ref.query read-only smoke (L1-3-IMPL-C)"
        # 查询引擎工具面只读场景：全部断言确定性（无 LLM 参与）。场景覆盖
        # CommandSpec 广告面 / 正常路径五元数据 / T10 三组反例 fail-closed /
        # T11 降级路径成本标注 / ref.diff identity 与 content 诚实边界。
        foreach ($wantTool in @("ref.query", "ref.diff")) {
            if (-not ($toolNames -contains $wantTool)) {
                throw ("missing query-engine tool in /agent/tools: " + $wantTool)
            }
        }
        Write-Ok "ref.query/ref.diff are advertised"

        $invokeUri = $AgentHttp.TrimEnd("/") + "/agent/invoke"
        $queryArgs = @{
            kinds = @("dom", "fxm", "com", "acp")
            limit = 50
        }
        $queryResp = Invoke-Json -Method POST -Uri $invokeUri -Body @{
            tool   = "ref.query"
            args   = $queryArgs
            source = "dev_agent_smoke.ref_query"
        } -TimeoutSec ([Math]::Max(30, $WaitSeconds))
        if ([string]$queryResp.status -ne "ok") {
            throw ("ref.query happy path failed: " + ($queryResp | ConvertTo-Json -Depth 8 -Compress))
        }
        $queryResult = Get-OptionalProperty -Object $queryResp -Name "result"
        foreach ($metaKey in @("cost_class", "degraded", "snapshot", "next_cursor", "total_matches")) {
            if ($null -eq $queryResult.PSObject.Properties[$metaKey]) {
                throw ("ref.query response missing required metadata: " + $metaKey)
            }
        }
        if ($null -ne $queryResult.rows -and @($queryResult.rows).Count -gt 0) {
            Write-Ok ("ref.query returned rows: " + [string]@($queryResult.rows).Count + " (cost_class=" + [string]$queryResult.cost_class + ")")
        }
        else {
            Write-Ok ("ref.query returned an empty row set with full metadata (cost_class=" + [string]$queryResult.cost_class + "; bootstrap scans only materialized artifacts on disk)")
        }

        $degradedResp = Invoke-Json -Method POST -Uri $invokeUri -Body @{
            tool   = "ref.query"
            args   = @{
                kinds               = @("dom")
                payload_conditions = @(
                    @{ field = "dom.peak_structure.readiness"; op = "eq"; str = "ready" }
                )
                limit               = 10
            }
            source = "dev_agent_smoke.ref_query"
        } -TimeoutSec ([Math]::Max(30, $WaitSeconds))
        $degradedResult = Get-OptionalProperty -Object $degradedResp -Name "result"
        if ([string]$degradedResp.status -ne "ok" -or [string]$degradedResult.cost_class -ne "compile" -or [string]$degradedResult.degraded -ne "payload_index_unavailable") {
            throw ("ref.query degraded path must surface cost_class=compile + degraded=payload_index_unavailable: " + ($degradedResp | ConvertTo-Json -Depth 8 -Compress))
        }
        Write-Ok "ref.query degraded path surfaces compile + payload_index_unavailable (T11)"

        # /agent/invoke 对校验拒绝回非 2xx（错误体即断言对象）——读异常响应体，
        # 与正常路径共用同一响应解析。
        $invokeExpectingError = {
            param([string]$Tool, [object]$ToolArgs)
            $body = @{ tool = $Tool; args = $ToolArgs; source = "dev_agent_smoke.ref_query" }
            $json = $body | ConvertTo-Json -Depth 20 -Compress
            $bytes = [System.Text.Encoding]::UTF8.GetBytes($json)
            try {
                $resp = Invoke-WebRequest -UseBasicParsing -Method POST -Uri $invokeUri -Body $bytes -ContentType "application/json; charset=utf-8" -TimeoutSec ([Math]::Max(30, $WaitSeconds))
                return $resp.Content | ConvertFrom-Json
            }
            catch {
                # PS 5.1：非 2xx 响应体在 $_.ErrorDetails.Message（Response 流
                # 已被错误格式化消费，不再读 GetResponseStream）。
                $content = [string]$_.ErrorDetails.Message
                if (-not [string]::IsNullOrWhiteSpace($content)) {
                    return $content | ConvertFrom-Json
                }
                throw
            }
        }
        $assertRejected = {
            param([string]$Label, [object]$Resp, [string]$ErrorPattern)
            if ([string]$Resp.status -eq "ok") {
                throw ($Label + " must be rejected fail-closed, got ok")
            }
            $errorText = [string](Get-OptionalProperty -Object $Resp -Name "error")
            if (-not $errorText.Contains($ErrorPattern)) {
                throw ($Label + " rejection reason mismatch: expected [" + $ErrorPattern + "] got [" + $errorText + "]")
            }
            Write-Ok ($Label + " rejected as expected")
        }
        & $assertRejected "ref.query empty predicate" (& $invokeExpectingError "ref.query" @{ limit = 10 }) "empty predicate"
        & $assertRejected "ref.query limit 501" (& $invokeExpectingError "ref.query" @{ kinds = @("dom"); limit = 501 }) "limit"
        & $assertRejected "ref.query unknown field" (& $invokeExpectingError "ref.query" @{ kinds = @("dom"); kind = @("dom") }) "unknown field"

        $diffResp = Invoke-Json -Method POST -Uri $invokeUri -Body @{
            tool   = "ref.diff"
            args   = @{ base_revision = "current" }
            source = "dev_agent_smoke.ref_query"
        } -TimeoutSec ([Math]::Max(30, $WaitSeconds))
        $diffResult = Get-OptionalProperty -Object $diffResp -Name "result"
        if ([string]$diffResp.status -ne "ok" -or [string]$diffResult.cost_class -ne "index" -or $null -eq $diffResult.PSObject.Properties["unchanged_count"]) {
            throw ("ref.diff identity path failed: " + ($diffResp | ConvertTo-Json -Depth 8 -Compress))
        }
        Write-Ok ("ref.diff identity path ok (unchanged_count=" + [string]$diffResult.unchanged_count + ")")

        # depth=content 正例（IMPL-D 委托面：compile 成本级 + delegated/unrouted
        # 表面；空盘面=合法形态——delegated 空 map 如实）。深度 Berth 场景
        # （-Scenario ref_diff_content）以真实观察票断言承载者映射。
        $contentResp = Invoke-Json -Method POST -Uri $invokeUri -Body @{
            tool   = "ref.diff"
            args   = @{ base_revision = "current"; depth = "content" }
            source = "dev_agent_smoke.ref_query"
        } -TimeoutSec ([Math]::Max(30, $WaitSeconds))
        $contentResult = Get-OptionalProperty -Object $contentResp -Name "result"
        if ([string]$contentResp.status -ne "ok" -or [string]$contentResult.cost_class -ne "compile" -or [string]$contentResult.degraded -ne "" -or $null -eq $contentResult.PSObject.Properties["delegated"] -or $null -eq $contentResult.PSObject.Properties["unrouted_kinds"]) {
            throw ("ref.diff content path contract failed (want ok+compile+degraded empty+delegated/unrouted surface): " + ($contentResp | ConvertTo-Json -Depth 8 -Compress))
        }
        Write-Ok "ref.diff content path surfaces compile + delegated/unrouted surface (T11 口径)"
        & $assertRejected "ref.diff unknown depth" (& $invokeExpectingError "ref.diff" @{ base_revision = "current"; depth = "contents" }) "rejected"
    }
}

if (-not $NoChatSmoke) {
    Write-Step "Chat route smoke"
    $before = Count-MixPlannerChatEvents -LogPath $MixDiagLog
    $conv = "dev_smoke_" + (Get-Date -Format "yyyyMMdd_HHmmss")
    $chatMessage = -join @(
        [char]0x5E2E,
        [char]0x6211,
        [char]0x6DF7,
        [char]0x4E00,
        [char]0x4E0B,
        [char]0x5F53,
        [char]0x524D,
        [char]0x8F68,
        [char]0x9053
    )
    $chat = Invoke-Json -Method POST -Uri ($AgentHttp.TrimEnd("/") + "/agent/chat") -Body @{
        conversation_id = $conv
        message = $chatMessage
        context = @{
            agent_mode = "chat"
        }
    } -TimeoutSec 90
    $after = Count-MixPlannerChatEvents -LogPath $MixDiagLog
    Write-Host ("chat.stop_reason=" + [string]$chat.stop_reason)
    Write-Host ("chat.reply=" + [string]$chat.reply)
    if ($after -ne $before) {
        Fail-Or-Warn ("mix_planner_chat log count changed from " + $before + " to " + $after)
    }
    else {
        Write-Ok "natural mix chat did not route through legacy mix_planner_chat"
    }

    Write-Step "No-pending mix confirmation smoke"
    $confirmMessage = -join @(
        [char]0x53EF,
        [char]0x4EE5,
        [char]0x6267,
        [char]0x884C
    )
    $confirm = Invoke-Json -Method POST -Uri ($AgentHttp.TrimEnd("/") + "/agent/chat") -Body @{
        conversation_id = "dev_no_pending_" + (Get-Date -Format "yyyyMMdd_HHmmss")
        message = $confirmMessage
        context = @{
            agent_mode = "chat"
        }
    } -TimeoutSec 30
    Write-Host ("confirm.stop_reason=" + [string]$confirm.stop_reason)
    Write-Host ("confirm.reply=" + [string]$confirm.reply)
    if ([string]$confirm.stop_reason -ne "no_pending_mix_tick_candidate") {
        Fail-Or-Warn ("expected no_pending_mix_tick_candidate for bare confirmation, got " + [string]$confirm.stop_reason)
    }
    else {
        Write-Ok "bare mix confirmation did not execute without a pending candidate"
    }
}

if ($MixSmoke) {
    Write-Step "Mix observe smoke"
    $fresh = Invoke-Json -Method GET -Uri ($AgentHttp.TrimEnd("/") + "/agent/state") -TimeoutSec 10
    $tracks = @()
    if ($null -ne $fresh.shadow.tracks) {
        $tracks = @($fresh.shadow.tracks)
    }
    if ($tracks.Count -eq 0) {
        Fail-Or-Warn "skipping mix.observe smoke because no visible tracks are available"
    }
    else {
        $track = $tracks[0]
        $trackID = [string]$track.track_id
        if ([string]::IsNullOrWhiteSpace($trackID)) {
            $trackID = [string]$track.id
        }
        $mix = Invoke-Json -Method POST -Uri ($AgentHttp.TrimEnd("/") + "/agent/invoke") -Body @{
            tool = "mix.observe"
            args = @{
                scope = "selected_track"
                track_id = $trackID
                project_context = $true
                observation_only = $true
                disclosure = "digest_catalog"
                goal_text = "dev smoke observe"
                mix_session_id = "dev_smoke_" + (Get-Date -Format "yyyyMMdd_HHmmss")
            }
            source = "dev_agent_smoke"
        } -TimeoutSec 120
        Write-Host ("mix.observe status=" + [string]$mix.status + " error=" + [string]$mix.error)
        if ($mix.status -ne "ok") {
            Fail-Or-Warn "mix.observe failed"
        }
        else {
            Write-Ok ("mix.observe observation_id=" + [string]$mix.result.observation_id)
        }
    }
}

if ($WriteLeaseSmoke) {
    # VITNOTE-IMPL-4 工程级写租约（VITNOTE_V1_DESIGN §7.3）：
    # 同工程两条 B2 能力执行并发入栈（双 conversation 各 propose 后，
    # 双 approve 以 HttpClient 双任务并发发出），断言——
    #   ①两执行段的租约 [acquired, released] 区间不重叠；
    #   ②后来者真实排队（wait_started 早于先行者 released）；
    #   ③先行者执行完成、后来者在租约内被 stale_project_cut 诚实拒绝
    #     （预冻结 cut 与先行者变异后的 revision 不匹配——分层安全语义）。
    # 证据 = VIT_WRITE_LEASE_EVENTS_PATH JSONL 事件（纳秒时间戳工件）。
    # 已声明的概率面：双 approve 同时发出后，第二请求的 revision 快照
    # 若晚于先行者首个变异落盘，会在 chat 层被拒（不进 Coordinator、无
    # 租约段）——该结局重试（上限 3 次，每次全新 conversation/proposal），
    # 不计入通过。
    Write-Step "Write lease concurrent capability execution smoke (VITNOTE-IMPL-4)"
    $leaseInvokeUri = $AgentHttp.TrimEnd("/") + "/agent/invoke"
    $leaseChatUri = $AgentHttp.TrimEnd("/") + "/agent/chat"
    $leaseRespondUri = $AgentHttp.TrimEnd("/") + "/agent/interaction/respond"
    $leaseStamp = Get-Date -Format "yyyyMMdd_HHmmss"
    $leaseArtifactDir = Join-Path (Join-Path $WorkspaceDir "Artifacts\smoke") ("write_lease_" + $leaseStamp)
    New-Item -ItemType Directory -Path $leaseArtifactDir -Force | Out-Null

    $leaseInvokeTool = {
        param([string]$Tool, [object]$ToolArgs, [int]$TimeoutSec)
        Invoke-Json -Method POST -Uri $leaseInvokeUri -Body @{
            tool = $Tool
            args = $ToolArgs
            confirmed = $true
            source = "dev_agent_smoke.write_lease"
        } -TimeoutSec $TimeoutSec
    }

    # 记住当前打开的工程，场景结束后尽力恢复（共享栈礼仪）。
    $leasePriorState = Invoke-Json -Method POST -Uri $leaseInvokeUri -Body @{
        tool = "project.state"
        args = @{}
        source = "dev_agent_smoke.write_lease.prior_state"
    } -TimeoutSec ([Math]::Max(30, $WaitSeconds))
    $leasePriorResult = Get-OptionalProperty -Object $leasePriorState -Name "result"
    $leasePriorProjectPath = [string](Get-FirstPropertyValue -Object $leasePriorResult -Names @("project_path", "project_file", "file_path"))
    $leaseProjectSwitched = $false

    try {
        # --- fixture：隔离工程 + 两条电平失衡 stem，DAD/MOM ready ---
        # 文件名走 staticbalance 名字推断词表（lead vocal / bass）——TOM 角色
        # 与功能多样性证据由轨道名喂给 B2 就绪模型；合成音频只承载 DAD L1。
        Write-LeaseSmokeStemWav -Path (Join-Path $leaseArtifactDir "Lead Vocal.wav") -Amplitude 0.5 -Frequency 440.0 | Out-Null
        Write-LeaseSmokeStemWav -Path (Join-Path $leaseArtifactDir "Bass.wav") -Amplitude 0.35 -Frequency 110.0 | Out-Null
        $leaseProjectPath = Join-Path $leaseArtifactDir "write_lease_fixture.vit"
        $null = & $leaseInvokeTool "project.new" @{} 60
        $leaseProjectSwitched = $true
        $null = & $leaseInvokeTool "project.save_as" @{ file_path = $leaseProjectPath } 60
        $leaseImport = & $leaseInvokeTool "project.import_folder_as_stems" @{
            folder_path = $leaseArtifactDir
            recursive = $false
            target_policy = "create_tracks"
            start_time_seconds = 0.0
            skip_unreadable = $false
            command_timeout_ms = 60000
        } 120
        if ([string]$leaseImport.status -ne "ok") {
            throw ("write lease fixture stems import failed: " + ($leaseImport | ConvertTo-Json -Depth 8 -Compress))
        }
        $leaseImportResult = Get-OptionalProperty -Object $leaseImport -Name "result"
        $leaseImportJob = Get-OptionalProperty -Object $leaseImportResult -Name "analysis_job"
        $leaseJobId = [string](Get-FirstPropertyValue -Object $leaseImportResult -Names @("analysis_job_id"))
        if ([string]::IsNullOrWhiteSpace($leaseJobId)) {
            $leaseJobId = [string](Get-FirstPropertyValue -Object $leaseImportJob -Names @("analysis_job_id", "job_id"))
        }
        if ([string]::IsNullOrWhiteSpace($leaseJobId)) {
            throw ("write lease fixture import returned no analysis job id: " + ($leaseImport | ConvertTo-Json -Depth 8 -Compress))
        }
        $null = & $leaseInvokeTool "project.audio_analysis_start" @{ analysis_job_id = $leaseJobId; interval_ms = 10 } 60

        $leaseDadDeadline = (Get-Date).AddSeconds(240)
        $leaseDadReady = $false
        while ((Get-Date) -lt $leaseDadDeadline) {
            $leaseDad = & $leaseInvokeTool "project.audio_analysis_status" @{ analysis_job_id = $leaseJobId; latest = $true } 60
            $leaseDadResult = Get-OptionalProperty -Object $leaseDad -Name "result"
            $leaseDadJob = Get-OptionalProperty -Object $leaseDadResult -Name "analysis_job"
            $leaseDadTotal = [int](Get-FirstPropertyValue -Object $leaseDadJob -Names @("dad_fact_total_count", "dad_fact_total"))
            $leaseDadReadyCount = [int](Get-FirstPropertyValue -Object $leaseDadJob -Names @("dad_fact_ready_count"))
            $leaseDadStatus = [string](Get-OptionalProperty -Object $leaseDadJob -Name "dad_fact_status")
            $leaseWaveformRows = @(Get-OptionalProperty -Object $leaseDadJob -Name "track_waveform_envelopes")
            if ($leaseDadTotal -gt 0 -and $leaseDadReadyCount -ge $leaseDadTotal -and $leaseDadStatus.ToLower() -eq "ready" -and $leaseWaveformRows.Count -ge $leaseDadTotal) {
                $leaseDadReady = $true
                break
            }
            Start-Sleep -Seconds 1
        }
        if (-not $leaseDadReady) {
            throw "write lease fixture DAD analysis did not become ready within 240s"
        }
        Write-Ok ("fixture DAD ready (tracks=" + [string]$leaseDadTotal + ")")

        # L3 特征显式预填 + MOM 多轨关系 ready（与 capability_runtime_v1_live_smoke 同款前置）。
        $leaseMixSession = "dev_write_lease_smoke_" + $leaseStamp
        $null = & $leaseInvokeTool "mix.observe" @{
            scope = "full_project"
            project_context = $true
            observation_only = $true
            observation_ready_gate = $true
            disclosure = "digest_catalog"
            mom_intent = "action_preflight_observation"
            mix_session_id = $leaseMixSession
            goal_text = "write lease smoke fixture L3 preflight"
        } 240
        $leaseMomDeadline = (Get-Date).AddSeconds(240)
        $leaseMomReady = $false
        $leaseMomObservationID = ""
        while ((Get-Date) -lt $leaseMomDeadline) {
            $leaseMom = & $leaseInvokeTool "mix.observe" @{
                scope = "full_project"
                project_context = $true
                observation_only = $true
                disclosure = "digest_catalog"
                mom_intent = "project_multitrack_relation_observation"
                mix_session_id = $leaseMixSession
                goal_text = "write lease smoke fixture readiness"
            } 240
            $leaseMomResult = Get-OptionalProperty -Object $leaseMom -Name "result"
            $leaseMomProjection = Get-OptionalProperty -Object $leaseMomResult -Name "mom_projection"
            $leaseMomRelation = Get-OptionalProperty -Object $leaseMomProjection -Name "multitrack_relation"
            $leaseMomStatus = [string](Get-OptionalProperty -Object $leaseMomRelation -Name "status")
            $leaseMomStatic = Get-OptionalProperty -Object $leaseMomProjection -Name "static_level_relationship"
            $leaseMomStaticStatus = [string](Get-OptionalProperty -Object $leaseMomStatic -Name "status")
            $leaseMomObservationID = [string](Get-OptionalProperty -Object $leaseMomResult -Name "observation_id")
            if (-not [string]::IsNullOrWhiteSpace($leaseMomObservationID) -and $leaseMomStatus.ToLower() -eq "ready" -and $leaseMomStaticStatus.ToLower() -eq "ready") {
                $leaseMomReady = $true
                break
            }
            Start-Sleep -Seconds 1
        }
        if (-not $leaseMomReady) {
            throw "write lease fixture MOM multitrack/static_level relations did not become ready within 240s"
        }
        Write-Ok "fixture MOM multitrack + static_level relations ready"

        # --- 双 conversation 各 propose B2（顺序发起，均不执行），再并发 approve ---
        $leaseProposeMessage = -join @(
            [char]0x8BF7, [char]0x4E3A, [char]0x5F53, [char]0x524D, [char]0x9694, [char]0x79BB,
            [char]0x6D4B, [char]0x8BD5, [char]0x5DE5, [char]0x7A0B, [char]0x751F, [char]0x6210,
            [char]0x5E76, [char]0x6267, [char]0x884C, ' ', 'B', '2', ' ',
            [char]0x9759, [char]0x6001, [char]0x5E73, [char]0x8861, [char]0x3002
        )
        Add-Type -AssemblyName System.Net.Http
        $leaseParseTime = {
            param([string]$Raw)
            if ([string]::IsNullOrWhiteSpace($Raw)) { return [datetime]::MinValue }
            # Go RFC3339Nano 可带 >7 位小数；.NET 只认 7 位，先裁剪再按不变文化解析。
            $trimmed = [regex]::Replace($Raw, '(\.\d{7})\d+', '$1')
            return [datetime]::Parse($trimmed, [System.Globalization.CultureInfo]::InvariantCulture, [System.Globalization.DateTimeStyles]::AssumeUniversal)
        }

        # 就绪探测环：B2 就绪证据（faders/roles/static levels）是摄入滞后敏感面
        # ——试探性 propose 各带一次新观察，反复至 readiness 解锁（canary_stage=
        # proposal）。readiness_blocked 是可重试滞后，其余阶段立即失败。
        $leaseReadinessReady = $false
        $leaseProbe = 0
        while (-not $leaseReadinessReady -and $leaseProbe -lt 10) {
            $leaseProbe++
            $probeResp = Invoke-Json -Method POST -Uri $leaseChatUri -Body @{
                conversation_id = ("dev_write_lease_probe_" + $leaseStamp + "_" + [string]$leaseProbe)
                message = $leaseProposeMessage
                context = @{
                    agent_mode = "chat"
                    capability_id = "static_mix.static_balance.v0"
                    interaction_mode = "propose"
                    capability_runtime_v1 = $true
                }
            } -TimeoutSec 240
            $probeWorkflowData = Get-OptionalProperty -Object $probeResp -Name "workflow_data"
            $probeStage = [string](Get-OptionalProperty -Object $probeWorkflowData -Name "canary_stage")
            if ($probeStage -eq "proposal") {
                $leaseReadinessReady = $true
                break
            }
            if ($probeStage -ne "readiness_blocked") {
                throw ("write lease readiness probe hit unexpected canary_stage " + $probeStage + ": " + ($probeResp | ConvertTo-Json -Depth 8 -Compress))
            }
            Write-Host ("readiness probe " + [string]$leaseProbe + " still blocked: " + ((Get-OptionalProperty -Object $probeWorkflowData -Name "blockers") -join ","))
            Start-Sleep -Seconds 3
        }
        if (-not $leaseReadinessReady) {
            throw "write lease B2 readiness did not unlock after " + [string]$leaseProbe + " probes"
        }
        Write-Ok ("B2 readiness unlocked after " + [string]$leaseProbe + " probe(s)")

        $leaseAttempts = 0
        $leasePassed = $false
        $leaseSummary = $null
        while (-not $leasePassed -and $leaseAttempts -lt 3) {
            $leaseAttempts++
            $convA = "dev_write_lease_a_" + $leaseStamp + "_" + [string]$leaseAttempts
            $convB = "dev_write_lease_b_" + $leaseStamp + "_" + [string]$leaseAttempts
            $proposeA = Invoke-Json -Method POST -Uri $leaseChatUri -Body @{
                conversation_id = $convA
                message = $leaseProposeMessage
                context = @{
                    agent_mode = "chat"
                    capability_id = "static_mix.static_balance.v0"
                    interaction_mode = "propose"
                    capability_runtime_v1 = $true
                }
            } -TimeoutSec 240
            $proposeB = Invoke-Json -Method POST -Uri $leaseChatUri -Body @{
                conversation_id = $convB
                message = $leaseProposeMessage
                context = @{
                    agent_mode = "chat"
                    capability_id = "static_mix.static_balance.v0"
                    interaction_mode = "propose"
                    capability_runtime_v1 = $true
                }
            } -TimeoutSec 240
            $proposeA | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath (Join-Path $leaseArtifactDir ("propose_a_" + [string]$leaseAttempts + ".json")) -Encoding UTF8
            $proposeB | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath (Join-Path $leaseArtifactDir ("propose_b_" + [string]$leaseAttempts + ".json")) -Encoding UTF8

            $leaseConfirmations = @()
            foreach ($proposeRow in @($proposeA, $proposeB)) {
                $rowWorkflowData = Get-OptionalProperty -Object $proposeRow -Name "workflow_data"
                $rowStage = [string](Get-OptionalProperty -Object $rowWorkflowData -Name "canary_stage")
                $rowSessionID = [string](Get-OptionalProperty -Object $rowWorkflowData -Name "session_id")
                $rowNeedsConfirmation = [bool](Get-OptionalProperty -Object $proposeRow -Name "needs_confirmation")
                $rowWorkflow = [string](Get-OptionalProperty -Object $proposeRow -Name "workflow")
                $rowConfirmation = ""
                foreach ($interactionRow in @(Get-OptionalProperty -Object $proposeRow -Name "interaction_requests")) {
                    $rowKind = [string](Get-OptionalProperty -Object $interactionRow -Name "kind")
                    $rowInteractionWorkflow = [string](Get-OptionalProperty -Object $interactionRow -Name "workflow")
                    if (($rowKind.ToLower() -in @("proposal_approval", "confirmation")) -and $rowInteractionWorkflow.ToLower() -eq "capability_runtime_v1") {
                        $rowConfirmation = [string](Get-OptionalProperty -Object $interactionRow -Name "id")
                        break
                    }
                }
                if (-not $rowNeedsConfirmation -or $rowWorkflow -ne "capability_runtime_v1" -or $rowStage -ne "proposal" -or [string]::IsNullOrWhiteSpace($rowSessionID) -or [string]::IsNullOrWhiteSpace($rowConfirmation)) {
                    throw ("write lease proposal contract failed (attempt " + [string]$leaseAttempts + "): " + ($proposeRow | ConvertTo-Json -Depth 8 -Compress))
                }
                $leaseConfirmations += @{ session_id = $rowSessionID; interaction_id = $rowConfirmation }
            }
            $sessionA = [string]$leaseConfirmations[0].session_id
            $sessionB = [string]$leaseConfirmations[1].session_id
            $confirmA = [string]$leaseConfirmations[0].interaction_id
            $confirmB = [string]$leaseConfirmations[1].interaction_id

            # 并发 approve：同一 HttpClient 两个 PostAsync 背靠背发出（毫秒级偏差）。
            $leaseClient = New-Object System.Net.Http.HttpClient
            $leaseClient.Timeout = [TimeSpan]::FromSeconds(300)
            $leasePostJson = {
                param([string]$InteractionID)
                (@{ interaction_id = $InteractionID; action_id = "approve"; decision = "approve"; payload = @{} } | ConvertTo-Json -Depth 6 -Compress)
            }
            $contentA = New-Object System.Net.Http.StringContent((& $leasePostJson $confirmA), [System.Text.Encoding]::UTF8, "application/json")
            $contentB = New-Object System.Net.Http.StringContent((& $leasePostJson $confirmB), [System.Text.Encoding]::UTF8, "application/json")
            $taskA = $leaseClient.PostAsync($leaseRespondUri, $contentA)
            $taskB = $leaseClient.PostAsync($leaseRespondUri, $contentB)
            if (-not [System.Threading.Tasks.Task]::WaitAll(@($taskA, $taskB), 300000)) {
                throw "write lease concurrent approvals did not complete within 300s"
            }
            $approveRawA = $taskA.Result.Content.ReadAsStringAsync().Result
            $approveRawB = $taskB.Result.Content.ReadAsStringAsync().Result
            $approveA = $approveRawA | ConvertFrom-Json
            $approveB = $approveRawB | ConvertFrom-Json
            $approveRawA | Set-Content -LiteralPath (Join-Path $leaseArtifactDir ("approve_a_" + [string]$leaseAttempts + ".json")) -Encoding UTF8
            $approveRawB | Set-Content -LiteralPath (Join-Path $leaseArtifactDir ("approve_b_" + [string]$leaseAttempts + ".json")) -Encoding UTF8

            # 响应面：恰好一个执行成功（先行者），另一个诚实失败。
            $stageA = [string](Get-OptionalProperty -Object (Get-OptionalProperty -Object $approveA -Name "workflow_data") -Name "canary_stage")
            $stageB = [string](Get-OptionalProperty -Object (Get-OptionalProperty -Object $approveB -Name "workflow_data") -Name "canary_stage")
            $executedStages = @("executed_verified", "executed_needs_review")
            $winnerASuccess = $executedStages -contains $stageA
            $winnerBSuccess = $executedStages -contains $stageB

            # 证据面：租约事件 JSONL（需脚本启动的 agent 继承过 env）。
            $leaseEvents = @()
            if (Test-Path -LiteralPath $WriteLeaseEventsFile) {
                foreach ($leaseEventLine in @(Get-Content -LiteralPath $WriteLeaseEventsFile)) {
                    if (-not [string]::IsNullOrWhiteSpace($leaseEventLine)) {
                        $leaseEvents += ($leaseEventLine | ConvertFrom-Json)
                    }
                }
            }
            $eventsA = @($leaseEvents | Where-Object { [string]$_.holder -eq $sessionA })
            $eventsB = @($leaseEvents | Where-Object { [string]$_.holder -eq $sessionB })
            $acquiredA = @($eventsA | Where-Object { [string]$_.event -eq "acquired" })
            $acquiredB = @($eventsB | Where-Object { [string]$_.event -eq "acquired" })
            $terminalA = @($eventsA | Where-Object { [string]$_.event -in @("released", "ttl_expired") })
            $terminalB = @($eventsB | Where-Object { [string]$_.event -in @("released", "ttl_expired") })

            $segmentsPresent = ($acquiredA.Count -eq 1 -and $acquiredB.Count -eq 1 -and $terminalA.Count -eq 1 -and $terminalB.Count -eq 1)
            if (-not $segmentsPresent) {
                # 只有一段=竞争窗口未成型（第二请求被 chat 层 revision 守卫拦截，
                # 未进 Coordinator）——预声明的重试类；其它差异=真实失败。
                if ($acquiredA.Count + $acquiredB.Count -eq 1) {
                    Write-WarnLine ("write lease attempt " + [string]$leaseAttempts + ": contention window missed (second approval was rejected before the coordinator); retrying with fresh proposals")
                    $leaseClient.Dispose()
                    Start-Sleep -Seconds 1
                    continue
                }
                throw ("write lease events missing expected segments (acquiredA=" + [string]$acquiredA.Count + " acquiredB=" + [string]$acquiredB.Count + " terminalA=" + [string]$terminalA.Count + " terminalB=" + [string]$terminalB.Count + "); if the agent was reused instead of started by this script, re-run with -RestartAgent")
            }

            $timeAcquiredA = & $leaseParseTime ([string]$acquiredA[0].time)
            $timeAcquiredB = & $leaseParseTime ([string]$acquiredB[0].time)
            $timeTerminalA = & $leaseParseTime ([string]$terminalA[0].time)
            $timeTerminalB = & $leaseParseTime ([string]$terminalB[0].time)
            if ($timeAcquiredA -le $timeAcquiredB) {
                $firstSession = $sessionA; $secondSession = $sessionB
                $firstAcquired = $timeAcquiredA; $firstTerminal = $timeTerminalA
                $secondAcquired = $timeAcquiredB; $secondTerminal = $timeTerminalB
                $firstStage = $stageA; $secondStage = $stageB
                $secondEvents = $eventsB
                $secondApprove = $approveB
            }
            else {
                $firstSession = $sessionB; $secondSession = $sessionA
                $firstAcquired = $timeAcquiredB; $firstTerminal = $timeTerminalB
                $secondAcquired = $timeAcquiredA; $secondTerminal = $timeTerminalA
                $firstStage = $stageB; $secondStage = $stageA
                $secondEvents = $eventsA
                $secondApprove = $approveA
            }
            $firstSegmentMS = [math]::Round(($firstTerminal - $firstAcquired).TotalMilliseconds)
            $secondSegmentMS = [math]::Round(($secondTerminal - $secondAcquired).TotalMilliseconds)

            # 断言①：两段租约区间不重叠（先行者释放 ≤ 后来者获得）。
            if ($firstTerminal -gt $secondAcquired) {
                throw ("write lease segments overlapped: first=[" + $firstAcquired.ToString("o") + "," + $firstTerminal.ToString("o") + "] second=[" + $secondAcquired.ToString("o") + "," + $secondTerminal.ToString("o") + "]")
            }
            # 断言②：后来者真实排队（wait_started 早于先行者释放）。
            $secondWaitStarted = @($secondEvents | Where-Object { [string]$_.event -eq "wait_started" })
            if ($secondWaitStarted.Count -lt 1) {
                throw ("write lease second holder never queued behind the first (no wait_started for " + $secondSession + "); segments were merely sequential")
            }
            $timeSecondWait = & $leaseParseTime ([string]$secondWaitStarted[0].time)
            if ($timeSecondWait -gt $firstTerminal) {
                throw ("write lease second holder queued only after the first released (wait_started=" + $timeSecondWait.ToString("o") + " first_released=" + $firstTerminal.ToString("o") + ")")
            }
            # 断言③：同工程竞争 + 先行者执行完成 + 后来者诚实拒绝。
            $firstProject = [string]$acquiredA[0].project_id
            $secondProject = [string]$acquiredB[0].project_id
            if ($firstProject -ne $secondProject) {
                throw ("write lease segments ran on different projects: " + $firstProject + " vs " + $secondProject)
            }
            if (-not ($executedStages -contains $firstStage)) {
                throw ("write lease first (lease-winning) execution did not complete: canary_stage=" + $firstStage + " approve=" + ($secondApprove | ConvertTo-Json -Depth 8 -Compress))
            }
            $secondError = [string](Get-OptionalProperty -Object $secondApprove -Name "error")
            $secondGoalStatus = [string](Get-OptionalProperty -Object $secondApprove -Name "goal_status")
            if ($secondGoalStatus.ToLower() -ne "failed" -or -not ($secondError.Contains("stale_project_cut") -or $secondError.Contains("execution preflight"))) {
                throw ("write lease second execution was not honestly refused inside its lease segment: goal_status=" + $secondGoalStatus + " error=" + $secondError)
            }
            $leaseClient.Dispose()
            $leasePassed = $true
            $leaseSummary = @{
                attempt = $leaseAttempts
                project_id = $firstProject
                first_session = $firstSession
                second_session = $secondSession
                first_stage = $firstStage
                second_stage = $secondStage
                first_segment_ms = $firstSegmentMS
                second_segment_ms = $secondSegmentMS
                second_error = $secondError
                events_file = $WriteLeaseEventsFile
            }
        }
        if (-not $leasePassed) {
            throw "write lease smoke did not materialize two concurrent execution segments after 3 attempts"
        }
        Copy-Item -LiteralPath $WriteLeaseEventsFile -Destination (Join-Path $leaseArtifactDir "lease_events.json") -Force
        $leaseSummary | ConvertTo-Json -Depth 6 | Set-Content -LiteralPath (Join-Path $leaseArtifactDir "summary.json") -Encoding UTF8
        Write-Ok ("write lease serialized concurrent execution segments (project=" + $leaseSummary.project_id + " first=" + $leaseSummary.first_session + " " + [string]$leaseSummary.first_segment_ms + "ms, second=" + $leaseSummary.second_session + " " + [string]$leaseSummary.second_segment_ms + "ms incl. honest stale_project_cut refusal)")
        Write-Host ("write lease events artifact: " + (Join-Path $leaseArtifactDir "lease_events.json"))
    }
    finally {
        # 尽力恢复场景前打开的工程（共享栈礼仪；失败仅告警不影响场景结论）。
        if ($leaseProjectSwitched -and -not [string]::IsNullOrWhiteSpace($leasePriorProjectPath) -and (Test-Path -LiteralPath $leasePriorProjectPath)) {
            try {
                $null = & $leaseInvokeTool "project.open" @{ file_path = $leasePriorProjectPath } 120
                Write-Ok ("restored prior project: " + $leasePriorProjectPath)
            }
            catch {
                Write-WarnLine ("could not restore prior project " + $leasePriorProjectPath + ": " + $_.Exception.Message)
            }
        }
    }
}

Write-Step "Summary"
Write-Ok "dev agent smoke completed"
