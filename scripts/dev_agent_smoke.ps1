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
        & $assertRejected "ref.diff depth=content" (& $invokeExpectingError "ref.diff" @{ base_revision = "current"; depth = "content" }) "not implemented"
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
