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
    [switch]$Strict,
    [int]$WaitSeconds = 20
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
        Start-Process -FilePath $KernelExe -WorkingDirectory (Split-Path -Parent $KernelExe) -WindowStyle Hidden | Out-Null
        Write-Ok ("started kernel: " + $KernelExe)
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
        $uiKernelDeadline = (Get-Date).AddSeconds($WaitSeconds)
        while ((Get-Date) -lt $uiKernelDeadline -and -not (Get-TcpListener -Port ([int]$ZmqReqPort))) {
            Start-Sleep -Milliseconds 500
        }
        if (Get-TcpListener -Port ([int]$ZmqReqPort)) {
            Write-Ok ("UI-launched kernel command port is listening: " + $ZmqReqPort)
        }
        else {
            Fail-Or-Warn ("UI did not expose kernel command port within " + [string]$WaitSeconds + "s")
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

Write-Step "Summary"
Write-Ok "dev agent smoke completed"
