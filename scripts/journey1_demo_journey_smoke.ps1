#Requires -Version 5.1
<#
JOURNEY-1 demo-journey automation driver (2026-09-13).

One script, one demo journey, four assertions:

  copy project (912.vit shape: 6 stem tracks, bass = empty track without EQ)
    -> load it (isolated workspace)
    -> authority = full access
    -> send "help the bass track with a balance experiment and let me audition"
    -> assert:
       A1 no direction asking   first reply carries no A./B./C. option menu
       A2 load path works       no pca_load_gate dead-end; the plugin really lands
       A3 experiment chain      proposal -> apply -> readback -> A/B card mounted
       A4 session stays clean   reopening the project does NOT resume the old
                                agent conversation and leaves no stall WARN scan

Load-path reconnaissance (card gate; see the receipt for the full three-way
comparison). The chosen path is the kernel open_project command driven through
the agent tool face:

  POST /agent/invoke {tool:"open_project", command:{cmd:"open_project", ...}}

which runs agent/internal/harness/harness.go:12260 (kernel SendCommand
{"cmd":"open_project"}) and then applyProjectLifecycle(lifecycle="open")
(harness.go:1279-1326) -- the same agent-side session activation
(BindProjectIdentity -> OpenWorkingSessionAsGeneration -> ActivateProjectStore)
that the Godot host notification version.project_opened performs
(harness.go:2457-2510, sent by app/startup/start_page.gd:919-925). No GUI needed.

Isolation contract (AGENTS.md section 10):
  * the user project (912.vit) and its .vit_history are opened READ-ONLY and
    fingerprinted before/after; only a byte copy inside the run dir is operated on
  * the kernel default project XML is redirected with VIT_PROJECT_XML to a copy
    inside the run dir, so VitApp/Workspace/default_project.xml is never written
  * agent history is redirected with VIT_HISTORY_DRAFT_ROOT
  * every run uses a fresh artifact directory

Exit codes:
  0  four assertions green (the post-fix target; the journey gate)
  1  journey ran, at least one assertion is RED (the pre-fix baseline shape)
  2  environment / load-path failure (no assertion verdict is claimed)

Usage:
  powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\journey1_demo_journey_smoke.ps1
  powershell ... -RunRoot <fresh dir> -TurnBudgetSeconds 600 -MaxNudges 4
#>

[CmdletBinding()]
param(
    [string]$RepoRoot = "",
    [string]$RunRoot = "",
    [string]$Affix = "journey1",
    [switch]$SkipBuild,
    [string]$AgentBinary = "",
    [string]$KernelExe = "",
    [string]$ProjectSource = "D:\Godot\project\vit-daw-frontend\912.vit",
    [string]$PromptBase64 = "",
    [string]$EqPluginIdentifier = "VST3-bx_hybrid V2-d0ef306f-c141eb4b",
    [int]$WaitSeconds = 40,
    [int]$KernelDwellSeconds = 15,
    [int]$TurnBudgetSeconds = 480,
    [int]$PollSeconds = 5,
    [int]$MaxNudges = 4,
    [int]$ReopenDwellSeconds = 20,
    [switch]$SkipReopen
)

$ErrorActionPreference = "Stop"

function Write-Step { param([string]$Message) Write-Host ""; Write-Host ("== " + $Message) -ForegroundColor Cyan }
function Write-Ok { param([string]$Message) Write-Host ("ok: " + $Message) -ForegroundColor Green }
function Write-Bad { param([string]$Message) Write-Host ("RED: " + $Message) -ForegroundColor Red }
function Write-Info { param([string]$Message) Write-Host ("info: " + $Message) -ForegroundColor Gray }

function From-B64 { param([string]$Value) return [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String($Value)) }

# ASCII-safe literals: Windows PowerShell 5.1 reads a BOM-less .ps1 using the
# system ANSI code page, so every non-ASCII string rides base64 (same rule as
# scripts/b13b_frontier_masking_smoke.ps1).
# prompt = "help the bass track with a balance experiment and let me audition"
$PromptText = From-B64 "5biu5L2O6Z+z6L2o5YGa5Liq5Z2H6KGh5a6e6aqM77yM54S25ZCO6K6p5oiR6K+V5ZCs"
if (-not [string]::IsNullOrWhiteSpace($PromptBase64)) { $PromptText = From-B64 $PromptBase64 }
# nudge = "go ahead and execute"
$NudgeText = From-B64 "5Y+v5Lul5omn6KGM"
# A1 vocabulary, built from code points so the file stays pure ASCII.
$AskWordWhich   = -join @([char]0x54EA, [char]0x4E2A)   # which
$AskWordDir     = -join @([char]0x65B9, [char]0x5411)   # direction
$AskWordChoose  = -join @([char]0x9009, [char]0x62E9)   # choose
$AskWordPlease  = -join @([char]0x8BF7, [char]0x95EE)   # may I ask
$AskWordYouWant = -join @([char]0x4F60, [char]0x5E0C, [char]0x671B) # you want
$AskWordPrefer  = -join @([char]0x503E, [char]0x5411)   # preference
$AskWordOr       = -join @([char]0x8FD8, [char]0x662F)  # or
$FullWidthQuestion = [string][char]0xFF1F
# placeholder = "still working on this task; I will report when it is done"
$PlaceholderText = -join @(
    [char]0x6211, [char]0x8FD8, [char]0x5728, [char]0x7EE7, [char]0x7EED, [char]0x5904, [char]0x7406,
    [char]0x8FD9, [char]0x4E2A, [char]0x4EFB, [char]0x52A1, [char]0xFF0C, [char]0x5B8C, [char]0x6210,
    [char]0x540E, [char]0x518D, [char]0x5411, [char]0x4F60, [char]0x6C47, [char]0x62A5, [char]0x3002
)
$AskWords = @($AskWordWhich, $AskWordDir, $AskWordChoose, $AskWordPlease, $AskWordYouWant, $AskWordPrefer)
# Bass track name in the 912 shape is ASCII, so no base64 dance is needed.
$BassTrackName = "bass"

# ---------------------------------------------------------------- utilities

function Resolve-RepoRoot {
    param([string]$Explicit)
    if (-not [string]::IsNullOrWhiteSpace($Explicit)) { return (Resolve-Path -LiteralPath $Explicit).Path }
    return (Split-Path -Parent (Split-Path -Parent $MyInvocation.ScriptName))
}

function Get-TcpListener { param([int]$Port) return Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue | Select-Object -First 1 }

function Wait-PortListen {
    param([int]$Port, [int]$TimeoutSeconds)
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        if (Get-TcpListener -Port $Port) { return $true }
        Start-Sleep -Milliseconds 500
    }
    return $false
}

function Wait-HttpReady {
    param([string]$BaseUrl, [int]$TimeoutSeconds)
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        try {
            $resp = Invoke-WebRequest -UseBasicParsing -Uri ($BaseUrl.TrimEnd("/") + "/health") -TimeoutSec 2
            if ($resp.StatusCode -eq 200) { return $true }
        }
        catch { }
        Start-Sleep -Milliseconds 500
    }
    return $false
}

function Invoke-JsonGet {
    param([string]$Url, [int]$TimeoutSec = 60)
    return Invoke-RestMethod -Method GET -Uri $Url -TimeoutSec $TimeoutSec
}

# Windows PowerShell 5.1 encodes a string -Body as Latin-1, which destroys the
# Chinese prompt before it reaches the agent. Every JSON body therefore rides
# explicit UTF-8 bytes (same fix as b13b / cont_stall_repro_smoke).
function Invoke-JsonUtf8 {
    param([string]$Method, [string]$Url, $Body, [int]$TimeoutSec = 60)
    if ($null -ne $Body) {
        $json = $Body | ConvertTo-Json -Depth 12 -Compress
        return Invoke-RestMethod -Method $Method -Uri $Url -ContentType "application/json; charset=utf-8" -Body ([System.Text.Encoding]::UTF8.GetBytes($json)) -TimeoutSec $TimeoutSec
    }
    return Invoke-RestMethod -Method $Method -Uri $Url -TimeoutSec $TimeoutSec
}

function Save-Json {
    param($Value, [string]$Path)
    if ($null -eq $Value) { return }
    $Value | ConvertTo-Json -Depth 14 | Set-Content -LiteralPath $Path -Encoding UTF8
}

function Read-TextFileUtf8 {
    param([string]$Path)
    if ([string]::IsNullOrWhiteSpace($Path)) { return "" }
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) { return "" }
    try { return [System.IO.File]::ReadAllText($Path, [System.Text.Encoding]::UTF8) } catch { return "" }
}

function Read-JsonText {
    param([string]$Path)
    $text = Read-TextFileUtf8 -Path $Path
    if ([string]::IsNullOrWhiteSpace($text)) { return $null }
    try { return ($text | ConvertFrom-Json -ErrorAction Stop) } catch { return $null }
}

function Get-NamedChildren {
    param($Node)
    $out = New-Object System.Collections.Generic.List[object]
    if ($null -eq $Node) { return $out }
    if ($Node -is [System.Management.Automation.PSCustomObject]) {
        foreach ($prop in $Node.PSObject.Properties) { $out.Add([pscustomobject]@{ name = $prop.Name; value = $prop.Value }) }
        return $out
    }
    if ($Node -is [System.Collections.IEnumerable] -and -not ($Node -is [string])) {
        $index = 0
        foreach ($item in $Node) { $out.Add([pscustomobject]@{ name = [string]$index; value = $item }); $index++ }
    }
    return $out
}

function Get-RowCount {
    param($Node)
    if ($null -eq $Node) { return 0 }
    if ($Node -is [string]) { return 0 }
    $n = 0
    foreach ($item in $Node) { $n++ }
    return $n
}

# File-tree fingerprint: proves the user project and its history were never
# written by this run (AGENTS.md section 10).
function Get-TreeFingerprint {
    param([string]$Path)
    if (-not (Test-Path -LiteralPath $Path)) { return "absent" }
    if (Test-Path -LiteralPath $Path -PathType Leaf) { return (Get-FileHash -Algorithm SHA256 -LiteralPath $Path).Hash }
    $files = Get-ChildItem -LiteralPath $Path -Recurse -File -ErrorAction SilentlyContinue | Sort-Object FullName
    $sb = New-Object System.Text.StringBuilder
    foreach ($file in $files) {
        [void]$sb.Append($file.FullName.Substring($Path.Length)); [void]$sb.Append(":")
        [void]$sb.Append($file.Length); [void]$sb.Append(":")
        [void]$sb.Append($file.LastWriteTimeUtc.ToString("o")); [void]$sb.Append(";")
    }
    $bytes = [System.Text.Encoding]::UTF8.GetBytes($sb.ToString())
    $sha = [System.Security.Cryptography.SHA256]::Create()
    try { return ([System.BitConverter]::ToString($sha.ComputeHash($bytes)) -replace "-", "") }
    finally { $sha.Dispose() }
}

# ------------------------------------------------------- domain readers

# Working sessions of a project live beside the project file:
#   <projectDir>/.vit_history/.sessions/<projectUUID>/session_<stamp>_<id>/
function Get-ProjectSessionDirs {
    param([string]$ProjectFile)
    $root = Join-Path (Split-Path -Parent $ProjectFile) ".vit_history\.sessions"
    $rows = New-Object System.Collections.Generic.List[object]
    if (-not (Test-Path -LiteralPath $root)) { return $rows }
    foreach ($uuidDir in @(Get-ChildItem -LiteralPath $root -Directory -ErrorAction SilentlyContinue)) {
        foreach ($sessionDir in @(Get-ChildItem -LiteralPath $uuidDir.FullName -Directory -ErrorAction SilentlyContinue)) {
            # A directory mtime does NOT move when a file inside it is rewritten,
            # and the reopen defect rewrites session.json in place -- so the
            # freshness signal has to be the newest write anywhere in the tree.
            $newest = $sessionDir.LastWriteTimeUtc
            $fileCount = 0
            foreach ($file in @(Get-ChildItem -LiteralPath $sessionDir.FullName -Recurse -File -ErrorAction SilentlyContinue)) {
                $fileCount++
                if ($file.LastWriteTimeUtc -gt $newest) { $newest = $file.LastWriteTimeUtc }
            }
            $rows.Add([pscustomobject]@{
                uuid_dir        = $uuidDir.Name
                session         = $sessionDir.Name
                path            = $sessionDir.FullName
                last_write_utc  = $newest.ToString("o")
                file_count      = $fileCount
                session_json    = (Test-Path -LiteralPath (Join-Path $sessionDir.FullName "session.json"))
                has_workspace   = (Test-Path -LiteralPath (Join-Path $sessionDir.FullName "workspace"))
            })
        }
    }
    return $rows
}

# Newest agent_runtime_state.json under the run's draft root or the project dir.
function Find-RuntimeState {
    param([string]$DraftRoot, [string]$ProjectFile)
    $roots = New-Object System.Collections.Generic.List[string]
    if (-not [string]::IsNullOrWhiteSpace($DraftRoot)) { $roots.Add($DraftRoot) }
    if (-not [string]::IsNullOrWhiteSpace($ProjectFile)) { $roots.Add((Split-Path -Parent $ProjectFile)) }
    $found = New-Object System.Collections.Generic.List[object]
    foreach ($root in $roots) {
        if (-not (Test-Path -LiteralPath $root)) { continue }
        foreach ($item in @(Get-ChildItem -LiteralPath $root -Recurse -Filter "agent_runtime_state.json" -File -ErrorAction SilentlyContinue)) { $found.Add($item) }
    }
    if ($found.Count -eq 0) { return $null }
    return ($found | Sort-Object LastWriteTimeUtc -Descending | Select-Object -First 1)
}

# Processor rows of one /agent/ui/state track. The measured shape (probe
# 2026-09-13, run artifacts/journey1_probe/20260913_184918) is:
#   track.plugin_count : int   track.plugins : array|null   track.rack : object|null
# plus the response-level plugin_rack { plugins, rack, track }.
function Get-TrackProcessorRows {
    param($Track)
    $rows = New-Object System.Collections.Generic.List[object]
    if ($null -eq $Track) { return $rows }
    foreach ($name in @("plugins", "processors", "nodes", "rack_nodes", "effects")) {
        $prop = $Track.PSObject.Properties[$name]
        if ($null -eq $prop -or $null -eq $prop.Value) { continue }
        foreach ($child in (Get-NamedChildren $prop.Value)) { $rows.Add($child.value) }
    }
    return $rows
}

function Get-TrackPluginCount {
    param($Track)
    if ($null -eq $Track) { return 0 }
    $prop = $Track.PSObject.Properties["plugin_count"]
    if ($null -ne $prop -and $null -ne $prop.Value) { return [int]$prop.Value }
    return @(Get-TrackProcessorRows -Track $Track).Count
}

function Get-TrackFromUIState {
    param($UIState, [string]$Name)
    if ($null -eq $UIState) { return $null }
    foreach ($row in (Get-NamedChildren $UIState.tracks)) {
        $track = $row.value
        if ($null -eq $track) { continue }
        $trackName = [string]$track.name
        if ([string]::IsNullOrWhiteSpace($trackName)) { $trackName = [string]$track.track_name }
        if ($trackName -eq $Name) { return $track }
    }
    return $null
}

function Get-ProjectTrackNames {
    param($UIState)
    $names = New-Object System.Collections.Generic.List[string]
    foreach ($row in (Get-NamedChildren $UIState.tracks)) {
        $track = $row.value
        if ($null -eq $track) { continue }
        $trackName = [string]$track.name
        if ([string]::IsNullOrWhiteSpace($trackName)) { $trackName = [string]$track.track_name }
        if (-not [string]::IsNullOrWhiteSpace($trackName)) { $names.Add($trackName) }
    }
    return $names
}

# Every event type string seen in an /agent/events payload.
function Get-EventTypes {
    param($Events)
    $types = New-Object System.Collections.Generic.List[string]
    if ($null -eq $Events) { return $types }
    foreach ($row in (Get-NamedChildren $Events.events)) {
        $eventType = [string]$row.value.type
        if (-not [string]::IsNullOrWhiteSpace($eventType)) { $types.Add($eventType) }
    }
    return $types
}

function Get-LogLineCount {
    param([string]$Path, [string]$Pattern)
    $text = Read-TextFileUtf8 -Path $Path
    if ([string]::IsNullOrWhiteSpace($text)) { return 0 }
    return ([regex]::Matches($text, $Pattern)).Count
}

# ------------------------------------------------------------- main body

$RepoRoot = Resolve-RepoRoot -Explicit $RepoRoot
if ([string]::IsNullOrWhiteSpace($RunRoot)) {
    $RunRoot = Join-Path $RepoRoot ("artifacts\" + $Affix + "\" + (Get-Date -Format "yyyyMMdd_HHmmss"))
}
if (Test-Path -LiteralPath $RunRoot) { throw ("run root already exists; each run needs a fresh artifact directory: " + $RunRoot) }
New-Item -ItemType Directory -Force -Path $RunRoot | Out-Null

$ProjectDir      = Join-Path $RunRoot "project"
$AgentDrafts     = Join-Path $RunRoot "agent_drafts"
$KernelWorkspace = Join-Path $RunRoot "kernel_workspace"
$PhaseADir       = Join-Path $RunRoot "phase_a"
$PhaseBDir       = Join-Path $RunRoot "phase_b"
foreach ($dir in @($ProjectDir, $AgentDrafts, $KernelWorkspace, $PhaseADir, $PhaseBDir)) { New-Item -ItemType Directory -Force -Path $dir | Out-Null }

$CopyProject     = Join-Path $ProjectDir ("journey1_" + [System.IO.Path]::GetFileName($ProjectSource))
$AgentLog        = Join-Path $RunRoot "agent_last.log"
$AgentDir        = Join-Path $RepoRoot "agent"
$PrereqPath      = Join-Path $RunRoot "prereq.txt"
$ReportPath      = Join-Path $RunRoot "journey1_report.json"
$RepoDefaultProject = Join-Path $RepoRoot "VitApp\Workspace\default_project.xml"
$RepoSettingsXml    = Join-Path $RepoRoot "VitApp\Workspace\Settings\Settings.xml"
$UserHistoryDir  = Join-Path (Split-Path -Parent $ProjectSource) ".vit_history"

if ([string]::IsNullOrWhiteSpace($KernelExe)) {
    foreach ($candidate in @(
        (Join-Path $RepoRoot "VitApp\build\VitApp_artefacts\Release\VitApp.exe"),
        (Join-Path $RepoRoot "VitApp\build_release\VitApp_artefacts\Release\VitApp.exe"),
        (Join-Path $RepoRoot "Export\staging\runtime\VitApp.exe"))) {
        if (Test-Path -LiteralPath $candidate) { $KernelExe = $candidate; break }
    }
}
if ([string]::IsNullOrWhiteSpace($KernelExe) -or -not (Test-Path -LiteralPath $KernelExe)) {
    throw "kernel executable not found; pass -KernelExe explicitly"
}
if (-not (Test-Path -LiteralPath $ProjectSource -PathType Leaf)) { throw ("project source not found: " + $ProjectSource) }

$prereq = New-Object System.Collections.Generic.List[string]
function Add-Prereq { param([string]$Line) $script:prereq.Add($Line); Write-Host $Line }

$kernelProcId = $null
$agentProcId = $null
$outcome = "env_failure"
$assertions = [ordered]@{}
$phaseA = [ordered]@{}
$phaseB = [ordered]@{}

function Start-Stack {
    param([string]$PhaseTag, [string]$PhaseDir, [string]$AgentBinaryPath, [string]$KernelWorkspaceDir)
    $result = [ordered]@{ kernel_pid = 0; agent_pid = 0; started_utc = (Get-Date).ToUniversalTime().ToString("o") }
    $env:VIT_PROJECT_XML = (Join-Path $KernelWorkspaceDir "default_project.xml")
    $env:VIT_DAW_DEV_ROOT = $RepoRoot
    try {
        $kernelProc = Start-Process -FilePath $KernelExe -WorkingDirectory $KernelWorkspaceDir -WindowStyle Hidden -PassThru
    }
    finally {
        Remove-Item Env:VIT_PROJECT_XML -ErrorAction SilentlyContinue
    }
    $result.kernel_pid = $kernelProc.Id
    if (-not (Wait-PortListen -Port 5555 -TimeoutSeconds $WaitSeconds)) { throw ("kernel ZMQ REQ port 5555 did not listen (phase " + $PhaseTag + ")") }

    $env:VIT_HISTORY_DRAFT_ROOT = $AgentDrafts
    $env:VIT_DAW_DEV_ROOT = $RepoRoot
    try {
        $agentProc = Start-Process -FilePath $AgentBinaryPath -ArgumentList @("-http", "127.0.0.1:7878", "-last-log-path", (Join-Path $PhaseDir "agent_last.log"), "-keep-last-log-lines", "8000") -WorkingDirectory $AgentDir -WindowStyle Hidden -PassThru
    }
    finally {
        Remove-Item Env:VIT_HISTORY_DRAFT_ROOT -ErrorAction SilentlyContinue
        Remove-Item Env:VIT_DAW_DEV_ROOT -ErrorAction SilentlyContinue
    }
    $result.agent_pid = $agentProc.Id
    if (-not (Wait-HttpReady -BaseUrl "http://127.0.0.1:7878" -TimeoutSeconds $WaitSeconds)) { throw ("agent HTTP did not become ready (phase " + $PhaseTag + ")") }
    return $result
}

function Stop-Stack {
    param([int]$KernelPid, [int]$AgentPid, [string]$PhaseTag)
    if ($AgentPid) { Stop-Process -Id $AgentPid -Force -ErrorAction SilentlyContinue }
    Start-Sleep -Seconds 1
    if ($KernelPid) { Stop-Process -Id $KernelPid -Force -ErrorAction SilentlyContinue }
    $releaseDeadline = (Get-Date).AddSeconds(30)
    while ((Get-Date) -lt $releaseDeadline) {
        $busy = @()
        foreach ($port in 7878, 5555, 5556) { if (Get-TcpListener -Port $port) { $busy += $port } }
        if ($busy.Count -eq 0) { break }
        Start-Sleep -Milliseconds 500
    }
    foreach ($port in 7878, 5555, 5556) {
        if (Get-TcpListener -Port $port) { Add-Prereq ("port_still_busy_after_" + $PhaseTag + "=" + $port) }
    }
}

# The card's load-path gate: drive the kernel open_project command through the
# agent tool face and report exactly what came back.
function Invoke-ProjectLoad {
    param([string]$Base, [string]$ProjectFile, [string]$PhaseDir)
    $body = @{
        tool     = "open_project"
        command  = @{ cmd = "open_project"; file_path = $ProjectFile }
        source   = "journey1_driver"
        confirmed = $true
    }
    $response = Invoke-JsonUtf8 -Method POST -Url ($Base + "/agent/invoke") -Body $body -TimeoutSec 180
    Save-Json -Value $response -Path (Join-Path $PhaseDir "load_invoke_response.json")
    $status = [string]$response.status
    $requiresConfirmation = $false
    if ($null -ne $response.requires_confirmation) { $requiresConfirmation = [bool]$response.requires_confirmation }
    if ($requiresConfirmation) {
        $body["confirmed"] = $true
        $response = Invoke-JsonUtf8 -Method POST -Url ($Base + "/agent/invoke") -Body $body -TimeoutSec 180
        Save-Json -Value $response -Path (Join-Path $PhaseDir "load_invoke_response_confirmed.json")
        $status = [string]$response.status
    }
    return $response
}

try {
    Add-Prereq ("run_root=" + $RunRoot)
    Add-Prereq ("head=" + (& git -C $RepoRoot rev-parse HEAD))
    Add-Prereq ("git_status=" + ((& git -C $RepoRoot status --short) -join " ; "))
    Add-Prereq ("project_source=" + $ProjectSource)
    Add-Prereq ("user_project_sha256_before=" + (Get-FileHash -Algorithm SHA256 -LiteralPath $ProjectSource).Hash)
    Add-Prereq ("user_history_fingerprint_before=" + (Get-TreeFingerprint -Path $UserHistoryDir))
    Add-Prereq ("repo_default_project_sha256_before=" + (Get-FileHash -Algorithm SHA256 -LiteralPath $RepoDefaultProject).Hash)
    Add-Prereq ("repo_settings_sha256_before=" + (Get-FileHash -Algorithm SHA256 -LiteralPath $RepoSettingsXml).Hash)

    Write-Step "Prereq: ports 7878/5555/5556 must be free (stack ownership)"
    foreach ($port in 7878, 5555, 5556) {
        if (Get-TcpListener -Port $port) { throw ("port " + $port + " is already listening; aborting before any start (AGENTS.md section 9)") }
        Add-Prereq ("port_free=" + $port)
    }

    Write-Step "Isolate: copy the project and the kernel default XML into the run dir"
    Copy-Item -LiteralPath $ProjectSource -Destination $CopyProject -Force
    Add-Prereq ("copy_project=" + $CopyProject)
    Add-Prereq ("copy_project_sha256=" + (Get-FileHash -Algorithm SHA256 -LiteralPath $CopyProject).Hash)
    Copy-Item -LiteralPath $RepoDefaultProject -Destination (Join-Path $KernelWorkspace "default_project.xml") -Force
    New-Item -ItemType Directory -Force -Path (Join-Path $KernelWorkspace "Settings") | Out-Null
    New-Item -ItemType Directory -Force -Path (Join-Path $KernelWorkspace "Logs") | Out-Null

    if ([string]::IsNullOrWhiteSpace($AgentBinary)) { $AgentBinary = Join-Path $RunRoot "bin\VitAgent.journey1.exe" }
    if (-not $SkipBuild) {
        Write-Step "Build VitAgent (isolated binary in the run dir)"
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $AgentBinary) | Out-Null
        Push-Location $AgentDir
        try {
            & go build -o $AgentBinary .\cmd\vitagent
            if ($LASTEXITCODE -ne 0) { throw ("go build failed with exit code " + $LASTEXITCODE) }
        }
        finally { Pop-Location }
    }
    elseif (-not (Test-Path -LiteralPath $AgentBinary -PathType Leaf)) { throw "-SkipBuild given but the isolated agent binary is missing" }
    $agentItem = Get-Item -LiteralPath $AgentBinary
    Add-Prereq ("agent_binary=" + $AgentBinary + " size=" + $agentItem.Length + " mtime=" + $agentItem.LastWriteTime.ToString("o") + " sha256=" + (Get-FileHash -Algorithm SHA256 -LiteralPath $AgentBinary).Hash)
    Add-Prereq ("kernel_binary=" + $KernelExe + " mtime=" + (Get-Item -LiteralPath $KernelExe).LastWriteTime.ToString("o"))

    # =========================================================== PHASE A
    Write-Step "PHASE A: start the real stack (kernel + agent)"
    $stackA = Start-Stack -PhaseTag "A" -PhaseDir $PhaseADir -AgentBinaryPath $AgentBinary -KernelWorkspaceDir $KernelWorkspace
    $kernelProcId = $stackA.kernel_pid
    $agentProcId = $stackA.agent_pid
    Add-Prereq ("phase_a_kernel_pid=" + $kernelProcId + " agent_pid=" + $agentProcId)
    $base = "http://127.0.0.1:7878"

    $bootstrap = Invoke-JsonGet -Url ($base + "/agent/state") -TimeoutSec 30
    Save-Json -Value $bootstrap -Path (Join-Path $PhaseADir "state_bootstrap.json")
    Add-Prereq ("phase_a_shadow_initialized=" + $bootstrap.shadow.initialized + " track_count=" + $bootstrap.shadow.track_count)

    if ($KernelDwellSeconds -gt 0) { Write-Step ("Kernel warm-up dwell " + $KernelDwellSeconds + "s"); Start-Sleep -Seconds $KernelDwellSeconds }

    Write-Step "PHASE A step 1: load the copy project (kernel open_project via agent tool face)"
    $phaseA["load_started_utc"] = (Get-Date).ToUniversalTime().ToString("o")
    $loadResponse = Invoke-ProjectLoad -Base $base -ProjectFile $CopyProject -PhaseDir $PhaseADir
    $phaseA["load_status"] = [string]$loadResponse.status
    $phaseA["load_command"] = [string]$loadResponse.command_name
    $phaseA["load_error"] = [string]$loadResponse.error
    Add-Prereq ("phase_a_load_status=" + $phaseA["load_status"] + " command=" + $phaseA["load_command"] + " error=" + $phaseA["load_error"])
    if ($phaseA["load_status"] -ne "ok") {
        throw ("project load path failed: status=" + $phaseA["load_status"] + " error=" + $phaseA["load_error"] + " (see load_invoke_response.json)")
    }

    # Give the session activation time to land, then read the loaded project shape.
    Start-Sleep -Seconds 6
    $uiBefore = Invoke-JsonGet -Url ($base + "/agent/ui/state") -TimeoutSec 60
    Save-Json -Value $uiBefore -Path (Join-Path $PhaseADir "ui_state_after_load.json")
    $tracksBefore = @(Get-ProjectTrackNames -UIState $uiBefore)
    $bassBefore = Get-TrackFromUIState -UIState $uiBefore -Name $BassTrackName
    $bassPluginsBefore = Get-TrackPluginCount -Track $bassBefore
    $phaseA["track_names"] = @($tracksBefore)
    $phaseA["track_count"] = $tracksBefore.Count
    $phaseA["bass_plugin_count_before"] = $bassPluginsBefore
    $phaseA["bass_present"] = ($null -ne $bassBefore)
    $phaseA["project_uuid"] = [string]$uiBefore.project.project_uuid
    if ([string]::IsNullOrWhiteSpace($phaseA["project_uuid"])) { $phaseA["project_uuid"] = [string]$uiBefore.project_history.project_uuid }
    $phaseA["project_path"] = [string]$uiBefore.project.project_path
    Add-Prereq ("phase_a_tracks=" + ($tracksBefore -join ","))
    Add-Prereq ("phase_a_bass_present=" + $phaseA["bass_present"] + " bass_plugin_count_before=" + $bassPluginsBefore + " project_uuid=" + $phaseA["project_uuid"])

    $sessionsAfterLoad = @(Get-ProjectSessionDirs -ProjectFile $CopyProject)
    $phaseA["sessions_after_load"] = @($sessionsAfterLoad)
    Add-Prereq ("phase_a_sessions_after_load=" + ((@($sessionsAfterLoad | ForEach-Object { $_.session })) -join ","))

    Write-Step "PHASE A step 2: authority = full project access"
    $auth = Invoke-JsonUtf8 -Method POST -Url ($base + "/agent/authority") -Body @{ authority_mode = "full_project_access" } -TimeoutSec 30
    Save-Json -Value $auth -Path (Join-Path $PhaseADir "authority_response.json")
    $phaseA["authority_mode"] = [string]$auth.authority_mode
    Add-Prereq ("phase_a_authority_mode=" + $phaseA["authority_mode"])
    if ($phaseA["authority_mode"] -ne "full_project_access") { throw ("authority switch refused: " + ($auth | ConvertTo-Json -Compress)) }

    Write-Step "PHASE A step 3: send the demo prompt and let the chain run"
    $conversationA = "journey1_a_" + (Get-Date -Format "yyyyMMdd_HHmmss")
    $phaseA["conversation_id"] = $conversationA
    $phaseA["prompt"] = $PromptText
    $slices = New-Object System.Collections.Generic.List[object]
    $deadline = (Get-Date).AddSeconds($TurnBudgetSeconds)
    $sliceIndex = 0
    $chatResponse = $null
    while ($sliceIndex -le $MaxNudges -and (Get-Date) -lt $deadline) {
        $message = if ($sliceIndex -eq 0) { $PromptText } else { $NudgeText }
        $sliceStarted = Get-Date
        $chatResponse = Invoke-JsonUtf8 -Method POST -Url ($base + "/agent/chat") -Body @{ conversation_id = $conversationA; message = $message } -TimeoutSec 300
        $sliceEnded = Get-Date
        Save-Json -Value $chatResponse -Path (Join-Path $PhaseADir ("chat_response_" + $sliceIndex + ".json"))
        $sliceStatus = [string]$chatResponse.goal_status
        $slices.Add([pscustomobject]@{
            index = $sliceIndex
            status = $sliceStatus
            stop_reason = [string]$chatResponse.stop_reason
            ms = [int](($sliceEnded - $sliceStarted).TotalMilliseconds)
            reply = [string]$chatResponse.reply
        })
        Add-Prereq ("phase_a_slice_" + $sliceIndex + "_status=" + $sliceStatus + " stop=" + [string]$chatResponse.stop_reason)
        Write-Ok ("slice " + $sliceIndex + ": goal_status=" + $sliceStatus + " stop=" + [string]$chatResponse.stop_reason)
        if ($sliceIndex -eq 0 -and [string]::IsNullOrWhiteSpace([string]$chatResponse.reply)) { Write-Bad "first slice returned an empty reply" }

        if ($sliceStatus -eq "waiting_continue") {
            $pollDeadline = (Get-Date).AddSeconds([Math]::Min(120, $TurnBudgetSeconds))
            while ((Get-Date) -lt $pollDeadline -and (Get-Date) -lt $deadline) {
                Start-Sleep -Seconds $PollSeconds
                try {
                    $runtime = Invoke-JsonGet -Url ($base + "/agent/runtime/status") -TimeoutSec 30
                    $goalStatus = [string]$runtime.goal.status
                    Write-Info ("poll goal=" + $goalStatus)
                    if ($goalStatus -eq "completed" -or $goalStatus -eq "failed" -or $goalStatus -eq "stopped" -or $goalStatus -eq "cancelled") { break }
                }
                catch { }
            }
        }
        if ($sliceStatus -eq "waiting_clarification" -or $sliceStatus -eq "waiting_confirmation") { $sliceIndex++; continue }
        break
    }
    $phaseA["slices"] = @($slices.ToArray())

    if ($null -ne $chatResponse) { Save-Json -Value $chatResponse -Path (Join-Path $PhaseADir "chat_response_final.json") }
    Start-Sleep -Seconds 5
    $runtimeA = $null
    try { $runtimeA = Invoke-JsonGet -Url ($base + "/agent/runtime/status") -TimeoutSec 30 } catch { }
    Save-Json -Value $runtimeA -Path (Join-Path $PhaseADir "runtime_status.json")

    Write-Step "PHASE A evidence capture (ui state / events / actions / audition / durable state)"
    $uiAfter = $null
    $eventsA = $null
    $actionsA = $null
    try { $uiAfter = Invoke-JsonGet -Url ($base + "/agent/ui/state") -TimeoutSec 60 } catch { }
    try { $eventsA = Invoke-JsonGet -Url ($base + "/agent/events?conversation_id=" + $conversationA + "&since=0&limit=500") -TimeoutSec 60 } catch { }
    try { $actionsA = Invoke-JsonGet -Url ($base + "/agent/actions") -TimeoutSec 60 } catch { }
    Save-Json -Value $uiAfter -Path (Join-Path $PhaseADir "ui_state_after_turn.json")
    Save-Json -Value $eventsA -Path (Join-Path $PhaseADir "events.json")
    Save-Json -Value $actionsA -Path (Join-Path $PhaseADir "actions.json")

    $stateFileA = Find-RuntimeState -DraftRoot $AgentDrafts -ProjectFile $CopyProject
    if ($null -ne $stateFileA) {
        $phaseA["runtime_state_path"] = $stateFileA.FullName
        Copy-Item -LiteralPath $stateFileA.FullName -Destination (Join-Path $PhaseADir "agent_runtime_state_snapshot.json") -Force
    }

    Copy-Item -LiteralPath (Join-Path $PhaseADir "agent_last.log") -Destination (Join-Path $PhaseADir "agent_last.phase_a.log") -Force -ErrorAction SilentlyContinue

    # ---- A1: no direction asking in the text the user actually receives.
    # The HTTP /agent/chat reply is only the per-slice transport reply: when the
    # chain keeps running it is a "still working" placeholder, not the answer.
    # The user-visible content therefore also rides the /agent/events stream,
    # where each delivered item carries title/body. Both surfaces are measured.
    $firstReply = ""
    if ($slices.Count -gt 0) { $firstReply = [string]$slices[0].reply }
    $deliveredTexts = New-Object System.Collections.Generic.List[object]
    foreach ($slice in $slices) {
        $text = [string]$slice.reply
        if ([string]::IsNullOrWhiteSpace($text)) { continue }
        $deliveredTexts.Add([pscustomobject]@{ source = ("chat_slice_" + $slice.index); text = $text; placeholder = $text.Contains($PlaceholderText) })
    }
    foreach ($row in (Get-NamedChildren $eventsA.events)) {
        $event = $row.value
        if ($null -eq $event) { continue }
        $parts = New-Object System.Collections.Generic.List[string]
        foreach ($name in @("title", "body", "summary")) {
            $prop = $event.PSObject.Properties[$name]
            if ($null -ne $prop -and $null -ne $prop.Value -and -not [string]::IsNullOrWhiteSpace([string]$prop.Value)) { $parts.Add([string]$prop.Value) }
        }
        if ($parts.Count -eq 0) { continue }
        $deliveredTexts.Add([pscustomobject]@{ source = ("event_" + [string]$event.type + "_" + [string]$event.seq); text = ($parts -join " | "); placeholder = $false })
    }
    $substantive = @($deliveredTexts | Where-Object { -not $_.placeholder })
    $optionMarkerCount = 0
    $askHits = New-Object System.Collections.Generic.List[string]
    $hasQuestionMark = $false
    foreach ($row in $substantive) {
        $text = [string]$row.text
        $optionMarkerCount += ([regex]::Matches($text, '(?:^|[\s;:.,()\[\]])([A-D])[\.\u3001\uFF0E\)\uFF09]')).Count
        foreach ($word in $AskWords) { if ($text.Contains($word) -and -not $askHits.Contains($word)) { $askHits.Add($word) } }
        if ($text.Contains($FullWidthQuestion) -or $text.Contains("?")) { $hasQuestionMark = $true }
    }
    $askOrHit = $false
    foreach ($row in $substantive) { if (([string]$row.text).Contains($AskWordOr)) { $askOrHit = $true } }
    $a1Red = ($optionMarkerCount -ge 2) -or ($hasQuestionMark -and ($askHits.Count -ge 1 -or $askOrHit))
    if ($a1Red) { $a1State = "red" }
    elseif ($substantive.Count -eq 0) { $a1State = "not_observable" }
    else { $a1State = "green" }
    $phaseA["a1_first_reply"] = $firstReply
    $phaseA["a1_first_reply_is_placeholder"] = $firstReply.Contains($PlaceholderText)
    # NOTE: PS 5.1 rejects @(<List[object]>) as an OrderedDictionary value
    # ("Argument types do not match"); .ToArray() is the working form.
    $phaseA["a1_delivered_texts"] = $deliveredTexts.ToArray()
    $phaseA["a1_substantive_text_count"] = $substantive.Count
    $phaseA["a1_option_marker_count"] = $optionMarkerCount
    $phaseA["a1_ask_word_hits"] = @($askHits.ToArray())
    $phaseA["a1_question_mark"] = $hasQuestionMark
    $phaseA["a1_state"] = $a1State
    Add-Prereq ("a1_state=" + $a1State + " option_marker_count=" + $optionMarkerCount + " ask_words=" + (($askHits.ToArray()) -join "|") + " question_mark=" + $hasQuestionMark + " substantive_texts=" + $substantive.Count + " first_reply_placeholder=" + $phaseA["a1_first_reply_is_placeholder"])

    # ---- A2: load path works
    $gateDenials = Get-LogLineCount -Path (Join-Path $PhaseADir "agent_last.log") -Pattern "stage=pca_processor_load_gate"
    $bassAfter = Get-TrackFromUIState -UIState $uiAfter -Name $BassTrackName
    $bassPluginsAfter = Get-TrackPluginCount -Track $bassAfter
    $rackPluginsAfter = 0
    $rackRowsAfter = 0
    if ($null -ne $uiAfter -and $null -ne $uiAfter.plugin_rack) {
        $rackPluginsAfter = @(Get-NamedChildren $uiAfter.plugin_rack.plugins).Count
        $rackRowsAfter = @(Get-NamedChildren $uiAfter.plugin_rack.rack).Count
    }
    $phaseA["a2_pca_load_gate_denials"] = $gateDenials
    $phaseA["a2_bass_plugin_count_after"] = $bassPluginsAfter
    $phaseA["a2_plugin_rack_plugin_rows"] = $rackPluginsAfter
    $phaseA["a2_plugin_rack_rows"] = $rackRowsAfter
    $a2JourneyLanded = ($bassPluginsAfter -gt $bassPluginsBefore) -or ($rackPluginsAfter -gt 0)
    $phaseA["a2_journey_turn_plugin_landed"] = $a2JourneyLanded
    $phaseA["a2_plugin_landed"] = $a2JourneyLanded

    # ---- A2b: deterministic load-authorization probe.
    # The journey itself is a model-branch-dependent path (a run can end on a
    # mix_tick dose and never attempt a plugin load at all), so the plugin load
    # authorization is additionally probed directly: one rack.add_node for the
    # whitelisted static EQ on the bass track, under full project access. This
    # is the mechanical, repeatable half of assertion 2 -- it answers "is there
    # any way to load a plugin in this build" without depending on the model.
    $bassTrackID = [string]$bassAfter.track_id
    if ([string]::IsNullOrWhiteSpace($bassTrackID)) { $bassTrackID = [string]$bassAfter.id }
    $probeBefore = Get-LogLineCount -Path (Join-Path $PhaseADir "agent_last.log") -Pattern "stage=pca_processor_load_gate"
    $probeResponse = $null
    $probeErrorBody = ""
    $probeDenied = $false
    $probeStatus = ""
    $probeError = ""
    if (-not [string]::IsNullOrWhiteSpace($bassTrackID)) {
        $probeBody = @{
            tool      = "rack_add_node"
            # zone_id is a Kernel signal-zone whitelist (Z1/Z2/Z3); the earlier
            # "journey1_zone" value never reached that validator because the PCA
            # load gate rejected the probe first (PCA-FULLACCESS-1 receipt).
            command   = @{ cmd = "rack_add_node"; plugin_identifier = $EqPluginIdentifier; track_id = $bassTrackID; x = 0; y = 0; zone_id = "Z3" }
            source    = "journey1_driver"
            confirmed = $true
        }
        # A rejected invoke answers with a non-2xx status, so PowerShell throws and
        # the useful payload lives in ErrorDetails.Message -- capture both.
        $probeErrorBody = ""
        try { $probeResponse = Invoke-JsonUtf8 -Method POST -Url ($base + "/agent/invoke") -Body $probeBody -TimeoutSec 180 }
        catch {
            $probeResponse = $null
            if ($null -ne $_.ErrorDetails -and -not [string]::IsNullOrWhiteSpace($_.ErrorDetails.Message)) { $probeErrorBody = $_.ErrorDetails.Message }
            else { $probeErrorBody = $_.Exception.Message }
        }
        Save-Json -Value $probeResponse -Path (Join-Path $PhaseADir "a2_direct_rack_add_node_response.json")
        if (-not [string]::IsNullOrWhiteSpace($probeErrorBody)) { $probeErrorBody | Set-Content -LiteralPath (Join-Path $PhaseADir "a2_direct_rack_add_node_error.json") -Encoding UTF8 }
    }
    $probeAfter = Get-LogLineCount -Path (Join-Path $PhaseADir "agent_last.log") -Pattern "stage=pca_processor_load_gate"
    if ($null -ne $probeResponse) { $probeStatus = [string]$probeResponse.status; $probeError = [string]$probeResponse.error }
    $probeDenied = ($probeAfter -gt $probeBefore) -or ($probeStatus -eq "error")
    # Landing evidence comes from the request whose reply IS the Kernel's own
    # answer: a rack_add_node that returns a plugin instance id with
    # plugin_instance_ready=true has landed. /agent/ui/state is deliberately NOT
    # used here: the VSP execution path does not run afterKernelReply, so the
    # Agent shadow (and therefore the UI projection) is not refreshed by an
    # out-of-band probe and still reports plugin_count=0 (PCA-FULLACCESS-1
    # receipt, run ...20260913_pca_fullaccess_4). That projection gap is a
    # separate observation and is not what A2 asserts.
    $probePluginID = ""
    $probeInstanceReady = $false
    $probeGraphDiff = ""
    if ($null -ne $probeResponse) {
        $probeResult = $probeResponse.result
        if ($null -ne $probeResult) {
            $idProp = $probeResult.PSObject.Properties["plugin_id"]
            if ($null -ne $idProp -and $null -ne $idProp.Value) { $probePluginID = [string]$idProp.Value }
            $readyProp = $probeResult.PSObject.Properties["plugin_instance_ready"]
            if ($null -ne $readyProp -and $null -ne $readyProp.Value) { $probeInstanceReady = [bool]$readyProp.Value }
            foreach ($name in @("graph_last_diff_kind", "graph_last_diff_summary")) {
                $diffProp = $probeResult.PSObject.Properties[$name]
                if ($null -ne $diffProp -and -not [string]::IsNullOrWhiteSpace([string]$diffProp.Value)) {
                    if (-not [string]::IsNullOrWhiteSpace($probeGraphDiff)) { $probeGraphDiff = $probeGraphDiff + "|" }
                    $probeGraphDiff = $probeGraphDiff + [string]$diffProp.Value
                }
            }
        }
    }
    $probePluginLanded = (-not [string]::IsNullOrWhiteSpace($probePluginID)) -and $probeInstanceReady
    $phaseA["a2_direct_probe_plugin_id"] = $probePluginID
    $phaseA["a2_direct_probe_plugin_instance_ready"] = $probeInstanceReady
    $phaseA["a2_direct_probe_graph_diff"] = $probeGraphDiff
    $phaseA["a2_direct_probe_plugin_landed"] = $probePluginLanded
    Add-Prereq ("a2_direct_probe_landed=" + $probePluginLanded + " plugin_id=" + $probePluginID + " instance_ready=" + $probeInstanceReady + " graph_diff=" + $probeGraphDiff)

    # A2 verdict (evaluated after the probe, which is what carries it): the
    # journey turn is model-branch-dependent (a run can end on a mix_tick dose
    # without ever attempting a load), so the assertion holds when the
    # deterministic probe was denied zero times AND the Kernel's own reply to
    # that probe shows the plugin loaded. The journey-turn half stays reported.
    $a2Red = $probeDenied -or (-not $probePluginLanded)
    $phaseA["a2_direct_probe_denied"] = $probeDenied
    $phaseA["a2_red"] = $a2Red
    Add-Prereq ("a2_state=" + $(if ($a2Red) { "RED" } else { "GREEN" }) + " pca_load_gate_denials=" + $gateDenials + " probe_denied=" + $probeDenied + " probe_landed=" + $probePluginLanded + " journey_turn_landed=" + $a2JourneyLanded)
    $phaseA["a2_direct_probe_track_id"] = $bassTrackID
    $phaseA["a2_direct_probe_plugin"] = $EqPluginIdentifier
    $phaseA["a2_direct_probe_status"] = $probeStatus
    $phaseA["a2_direct_probe_error"] = $probeError
    $phaseA["a2_direct_probe_gate_denial_delta"] = ($probeAfter - $probeBefore)
    $phaseA["a2_direct_probe_denied"] = $probeDenied
    $phaseA["a2_direct_probe_error_body"] = $probeErrorBody
    $gateLines = @(Select-String -LiteralPath (Join-Path $PhaseADir "agent_last.log") -Pattern "stage=pca_processor_load_gate" -ErrorAction SilentlyContinue | ForEach-Object { $_.Line.Trim() })
    $phaseA["a2_pca_load_gate_log_lines"] = @($gateLines)
    Add-Prereq ("a2_direct_probe track=" + $bassTrackID + " plugin=" + $EqPluginIdentifier + " status=" + $probeStatus + " gate_denial_delta=" + ($probeAfter - $probeBefore) + " error=" + $probeError)

    # ---- A3: experiment chain reaches an A/B card
    $eventTypes = @(Get-EventTypes -Events $eventsA)
    $phaseA["a3_event_types"] = @($eventTypes)
    $auditionEvents = @($eventTypes | Where-Object { $_.StartsWith("audition.") })
    $mixTickEvents = @($eventTypes | Where-Object { $_ -like "*mix_tick*" })
    # The A/B card also shows up as an audition session id carried by the
    # delivered card, so the raw payload is scanned as well as the type list.
    $eventsText = ""
    if ($null -ne $eventsA) { $eventsText = ($eventsA | ConvertTo-Json -Depth 14 -Compress) }
    $actionsText = ""
    if ($null -ne $actionsA) { $actionsText = ($actionsA | ConvertTo-Json -Depth 14 -Compress) }
    $auditionSessionSeen = ($eventsText.Contains("audition_session_id")) -or ($actionsText.Contains("audition_session_id"))
    $proposalApplied = @($eventTypes | Where-Object { $_ -like "*intervention*" -or $_ -like "*applied*" }).Count -gt 0
    $auditionCardMounted = ($auditionEvents.Count -gt 0) -or $auditionSessionSeen -or ($mixTickEvents.Count -gt 0)
    $a3Red = -not $auditionCardMounted
    $phaseA["a3_audition_events"] = @($auditionEvents)
    $phaseA["a3_mix_tick_events"] = @($mixTickEvents)
    $phaseA["a3_audition_session_seen"] = $auditionSessionSeen
    $phaseA["a3_audition_card_mounted"] = $auditionCardMounted
    $phaseA["a3_proposal_applied_signal"] = $proposalApplied
    $phaseA["a3_blocked_by_a2"] = $a2Red
    $a3State = $(if ($a3Red) { "red" } else { "green" })
    $phaseA["a3_state"] = $a3State
    Add-Prereq ("a3_audition_events=" + (($auditionEvents -join ",")) + " mix_tick_events=" + (($mixTickEvents -join ",")) + " audition_session_seen=" + $auditionSessionSeen + " card_mounted=" + $auditionCardMounted + " proposal_applied=" + $proposalApplied + " A3=" + $(if ($a3Red) { "RED" } else { "GREEN" }))

    # ---- Save point (the assertion-4 contract is "no history from before the
    # SAVE POINT", so the journey has to contain a real manual save). Host-faithful
    # order, mirroring the Godot manual save: freeze the agent working session,
    # let the kernel write the .vit, then adopt the saved generation.
    Write-Step "PHASE A step 4: manual save (creates the save point for assertion 4)"
    $savePrepare = $null
    $saveResult = $null
    $saveFinal = $null
    $savePrepareID = ""
    $saveGenerationID = ""
    try {
        $savePrepare = Invoke-JsonUtf8 -Method POST -Url ($base + "/agent/invoke") -Body @{
            tool = "version_project_save_prepare"
            command = @{ cmd = "version_project_save_prepare"; project_path = $CopyProject; project_uuid = [string]$phaseA["project_uuid"]; save_kind = "manual" }
            source = "journey1_driver"
            confirmed = $true
        } -TimeoutSec 120
    } catch { $savePrepare = $null }
    Save-Json -Value $savePrepare -Path (Join-Path $PhaseADir "save_prepare_response.json")
    if ($null -ne $savePrepare -and $null -ne $savePrepare.result) {
        $savePrepareID = [string]$savePrepare.result.prepare_id
        $saveGenerationID = [string]$savePrepare.result.agent_history_generation
    }
    Add-Prereq ("phase_a_save_prepare status=" + $(if ($null -ne $savePrepare) { [string]$savePrepare.status } else { "none" }) + " prepare_id=" + $savePrepareID + " generation=" + $saveGenerationID)
    if (-not [string]::IsNullOrWhiteSpace($savePrepareID)) {
        try {
            $saveResult = Invoke-JsonUtf8 -Method POST -Url ($base + "/agent/invoke") -Body @{
                tool = "save_project"
                command = @{ cmd = "save_project"; file_path = $CopyProject; history_prepare_id = $savePrepareID; agent_history_generation = $saveGenerationID }
                source = "journey1_driver"
                confirmed = $true
            } -TimeoutSec 300
        } catch { $saveResult = $null }
        Save-Json -Value $saveResult -Path (Join-Path $PhaseADir "save_project_response.json")
        Add-Prereq ("phase_a_save_project status=" + $(if ($null -ne $saveResult) { [string]$saveResult.status } else { "none" }) + " error=" + $(if ($null -ne $saveResult) { [string]$saveResult.error } else { "" }))
        try {
            $saveFinal = Invoke-JsonUtf8 -Method POST -Url ($base + "/agent/invoke") -Body @{
                tool = "version_project_saved"
                command = @{ cmd = "version_project_saved"; project_path = $CopyProject; project_uuid = [string]$phaseA["project_uuid"]; history_prepare_id = $savePrepareID; agent_history_generation = $saveGenerationID; save_kind = "manual" }
                source = "journey1_driver"
                confirmed = $true
            } -TimeoutSec 120
        } catch { $saveFinal = $null }
        Save-Json -Value $saveFinal -Path (Join-Path $PhaseADir "version_project_saved_response.json")
    }
    $phaseA["save_prepare_status"] = $(if ($null -ne $savePrepare) { [string]$savePrepare.status } else { "none" })
    $phaseA["save_prepare_id"] = $savePrepareID
    $phaseA["save_generation_id"] = $saveGenerationID
    $phaseA["save_project_status"] = $(if ($null -ne $saveResult) { [string]$saveResult.status } else { "none" })
    $phaseA["save_project_error"] = $(if ($null -ne $saveResult) { [string]$saveResult.error } else { "" })
    $phaseA["version_project_saved_status"] = $(if ($null -ne $saveFinal) { [string]$saveFinal.status } else { "none" })
    $phaseA["copy_project_sha256_after_save"] = (Get-FileHash -Algorithm SHA256 -LiteralPath $CopyProject).Hash
    Add-Prereq ("phase_a_copy_project_sha256_after_save=" + $phaseA["copy_project_sha256_after_save"])

    $phaseA["sessions_after_turn"] = @(Get-ProjectSessionDirs -ProjectFile $CopyProject)

    # =========================================================== PHASE B
    if ($SkipReopen) {
        $phaseB["skipped"] = $true
        $phaseB["a4_state"] = "not_observable"
    }
    else {
        Write-Step "PHASE B: teardown, then reopen the same copy project (session-cleanliness control)"
        Stop-Stack -KernelPid $kernelProcId -AgentPid $agentProcId -PhaseTag "A"
        $kernelProcId = $null
        $agentProcId = $null
        $phaseB["reopen_started_utc"] = (Get-Date).ToUniversalTime().ToString("o")
        $phaseB["sessions_before_reopen"] = @(Get-ProjectSessionDirs -ProjectFile $CopyProject)

        $stackB = Start-Stack -PhaseTag "B" -PhaseDir $PhaseBDir -AgentBinaryPath $AgentBinary -KernelWorkspaceDir $KernelWorkspace
        $kernelProcId = $stackB.kernel_pid
        $agentProcId = $stackB.agent_pid
        Add-Prereq ("phase_b_kernel_pid=" + $kernelProcId + " agent_pid=" + $agentProcId)

        $loadResponseB = Invoke-ProjectLoad -Base $base -ProjectFile $CopyProject -PhaseDir $PhaseBDir
        $phaseB["load_status"] = [string]$loadResponseB.status
        $phaseB["load_error"] = [string]$loadResponseB.error
        Add-Prereq ("phase_b_load_status=" + $phaseB["load_status"] + " error=" + $phaseB["load_error"])
        if ($phaseB["load_status"] -ne "ok") { throw ("phase B project load failed: " + $phaseB["load_error"]) }

        Write-Step ("PHASE B dwell " + $ReopenDwellSeconds + "s so the reopen path can restore or not restore")
        Start-Sleep -Seconds $ReopenDwellSeconds

        try { $uiB = Invoke-JsonGet -Url ($base + "/agent/ui/state") -TimeoutSec 60 } catch { $uiB = $null }
        Save-Json -Value $uiB -Path (Join-Path $PhaseBDir "ui_state_after_reopen.json")
        try { $stateB = Invoke-JsonGet -Url ($base + "/agent/state?detail=full") -TimeoutSec 60 } catch { $stateB = $null }
        Save-Json -Value $stateB -Path (Join-Path $PhaseBDir "state_after_reopen_full.json")
        $eventsB = $null
        try { $eventsB = Invoke-JsonGet -Url ($base + "/agent/events?conversation_id=" + $conversationA + "&since=0&limit=500") -TimeoutSec 60 } catch { $eventsB = $null }
        Save-Json -Value $eventsB -Path (Join-Path $PhaseBDir "events_for_phase_a_conversation.json")

        $sessionsAfterB = @(Get-ProjectSessionDirs -ProjectFile $CopyProject)
        $phaseB["sessions_after_reopen"] = @($sessionsAfterB)
        $reopenStart = [datetime]::Parse($phaseB["reopen_started_utc"]).ToUniversalTime()
        $phaseASessionNames = @()
        foreach ($row in @($phaseB["sessions_before_reopen"])) { if (-not [string]::IsNullOrWhiteSpace($row.session)) { $phaseASessionNames += $row.session } }
        $resumedSessions = New-Object System.Collections.Generic.List[string]
        $freshSessions = New-Object System.Collections.Generic.List[string]
        foreach ($row in $sessionsAfterB) {
            $written = $false
            try { $written = ([datetime]::Parse($row.last_write_utc).ToUniversalTime() -ge $reopenStart) } catch { }
            if (-not $written) { continue }
            if ($phaseASessionNames -contains $row.session) { $resumedSessions.Add($row.session) } else { $freshSessions.Add($row.session) }
        }
        $stallWarnB = Get-LogLineCount -Path (Join-Path $PhaseBDir "agent_last.log") -Pattern "\[continuation\.stall\]"
        $phaseAResidueInLiveContext = $false
        $phaseAConversationIdInState = $false
        if ($null -ne $stateB) {
            $stateText = $stateB | ConvertTo-Json -Depth 14 -Compress
            $phaseAConversationIdInState = $stateText.Contains($conversationA)
        }
        if ($null -ne $eventsB) {
            $phaseAResidueInLiveContext = (@(Get-EventTypes -Events $eventsB)).Count -gt 0
        }
        # The contract is "the conversation after reopening must not carry history
        # from before the save point", so the decisive observable is the LIVE
        # conversation of the reopened session (history_dir + conversation graph
        # + messages), read from the same payload the GUI hydrates from.
        $liveHistoryDirB = ""
        $liveGraphNodesB = 0
        $liveMessagesB = 0
        $liveCommitCountB = 0
        if ($null -ne $stateB -and $null -ne $stateB.project_history) {
            $liveHistoryDirB = [string]$stateB.project_history.history_dir
            $liveGraphNodesB = Get-RowCount $stateB.project_history.conversation_graph.nodes
            $liveMessagesB = Get-RowCount $stateB.project_history.conversation_messages
            if ($null -ne $stateB.project_history.commit_count) { $liveCommitCountB = [int]$stateB.project_history.commit_count }
        }
        $liveSessionNameB = Split-Path -Leaf (Split-Path -Parent $liveHistoryDirB)
        $phaseAConversationCarried = ($liveGraphNodesB -gt 0) -or ($liveMessagesB -gt 0)
        $a4Red = $phaseAConversationCarried -or ($stallWarnB -gt 0)
        # File-level detail: exactly which files inside a phase A session dir the
        # reopen rewrote. This is the raw evidence behind the verdict, not just
        # the directory names.
        $rewrittenFiles = New-Object System.Collections.Generic.List[object]
        foreach ($sessRow in $sessionsAfterB) {
            if (-not ($phaseASessionNames -contains $sessRow.session)) { continue }
            foreach ($file in @(Get-ChildItem -LiteralPath $sessRow.path -Recurse -File -ErrorAction SilentlyContinue)) {
                if ($file.LastWriteTimeUtc -ge $reopenStart) {
                    $rewrittenFiles.Add([pscustomobject]@{
                        session = $sessRow.session
                        file    = $file.FullName.Substring($sessRow.path.Length)
                        bytes   = $file.Length
                        written = $file.LastWriteTimeUtc.ToString("o")
                    })
                }
            }
        }
        $phaseB["resumed_session_dirs"] = @($resumedSessions.ToArray())
        $phaseB["fresh_session_dirs"] = @($freshSessions.ToArray())
        $phaseB["phase_a_session_files_rewritten_on_reopen"] = @($rewrittenFiles.ToArray())
        $phaseB["a4_stall_warn_count"] = $stallWarnB
        $phaseB["a4_phase_a_conversation_visible"] = $phaseAResidueInLiveContext
        $phaseB["a4_phase_a_conversation_id_in_state"] = $phaseAConversationIdInState
        $phaseB["a4_live_history_dir"] = $liveHistoryDirB
        $phaseB["a4_live_session_name"] = $liveSessionNameB
        $phaseB["a4_live_conversation_graph_nodes"] = $liveGraphNodesB
        $phaseB["a4_live_conversation_messages"] = $liveMessagesB
        $phaseB["a4_live_commit_count"] = $liveCommitCountB
        $phaseB["a4_pre_savepoint_history_carried"] = $phaseAConversationCarried
        $phaseB["a4_previous_session_still_on_disk"] = @($resumedSessions.ToArray())
        $phaseB["a4_state"] = $(if ($a4Red) { "red" } else { "green" })
        Add-Prereq ("a4_state=" + $phaseB["a4_state"] + " live_session=" + $liveSessionNameB + " graph_nodes=" + $liveGraphNodesB + " messages=" + $liveMessagesB + " commit_count=" + $liveCommitCountB + " pre_savepoint_carried=" + $phaseAConversationCarried + " resumed_session_dirs=" + (@($resumedSessions.ToArray()) -join ",") + " fresh_session_dirs=" + (@($freshSessions.ToArray()) -join ",") + " rewritten_files=" + $rewrittenFiles.Count + " stall_warn=" + $stallWarnB)
    }

    if ($SkipReopen) { $a4State = "not_observable" } else { $a4State = [string]$phaseB["a4_state"] }
    $assertions = [ordered]@{
        a1_no_direction_asking = [ordered]@{ state = $a1State; basis = "delivered text (chat slices + event title/body): option-marker count / ask vocabulary" }
        a2_load_path_works     = [ordered]@{ state = $(if ($a2Red) { "red" } else { "green" }); basis = "deterministic half: the direct rack.add_node probe is not denied by the PCA load gate AND the Kernel's own reply carries a ready plugin instance (plugin_id + plugin_instance_ready); journey-turn half (bass plugin_count growth) reported separately in a2_journey_turn_plugin_landed" }
        a3_experiment_chain    = [ordered]@{ state = $a3State; basis = "proposal -> applied -> readback -> mix_tick/audition card on the conversation event stream" }
        a4_session_clean       = [ordered]@{ state = $a4State; basis = "phase B reopen after a phase A save point: does the live reopened conversation carry pre-save-point history (graph nodes / messages), and did a stall WARN scan run" }
    }
    $redCount = 0
    $notObservableCount = 0
    foreach ($key in $assertions.Keys) {
        if ($assertions[$key].state -eq "red") { $redCount++ }
        elseif ($assertions[$key].state -eq "not_observable") { $notObservableCount++ }
    }
    if ($redCount -eq 0 -and $notObservableCount -eq 0) { $outcome = "all_green" }
    elseif ($redCount -eq 0) { $outcome = "journey_inconclusive" }
    else { $outcome = "assertions_red" }
}
catch {
    $detail = $_.Exception.Message
    Add-Prereq ("fatal=" + $detail)
    Add-Prereq ("fatal_type=" + $_.Exception.GetType().FullName)
    Write-Bad ("fatal: " + $detail)
    $outcome = "env_failure"
}
finally {
    Write-Step "Teardown"
    Stop-Stack -KernelPid $kernelProcId -AgentPid $agentProcId -PhaseTag "final"
    if (Test-Path -LiteralPath $RepoDefaultProject) { Add-Prereq ("repo_default_project_sha256_after=" + (Get-FileHash -Algorithm SHA256 -LiteralPath $RepoDefaultProject).Hash) }
    if (Test-Path -LiteralPath $RepoSettingsXml) { Add-Prereq ("repo_settings_sha256_after=" + (Get-FileHash -Algorithm SHA256 -LiteralPath $RepoSettingsXml).Hash) }
    Add-Prereq ("user_project_sha256_after=" + (Get-FileHash -Algorithm SHA256 -LiteralPath $ProjectSource).Hash)
    Add-Prereq ("user_history_fingerprint_after=" + (Get-TreeFingerprint -Path $UserHistoryDir))
    Add-Prereq ("finished=" + (Get-Date -Format "o"))

    $report = [ordered]@{
        schema_version         = "vit_demo_journey_driver.v1"
        card                   = "JOURNEY-1"
        run_root               = $RunRoot
        project_source         = $ProjectSource
        copy_project           = $CopyProject
        kernel_exe             = $KernelExe
        agent_binary           = $AgentBinary
        prompt                 = $PromptText
        verdict                = $outcome
        assertions             = $assertions
        phase_a                = $phaseA
        phase_b                = $phaseB
        prereq                 = @($prereq)
    }
    try { Save-Json -Value $report -Path $ReportPath } catch { Write-Bad ("report write failed: " + $_.Exception.Message) }
    try { $prereq | Out-File -FilePath $PrereqPath -Encoding utf8 } catch { }
}

Write-Host ""
Write-Host ("JOURNEY1_VERDICT " + $outcome)
switch ($outcome) {
    "all_green"            { exit 0 }
    "assertions_red"       { exit 1 }
    "journey_inconclusive" { exit 1 }
    default                { exit 2 }
}
