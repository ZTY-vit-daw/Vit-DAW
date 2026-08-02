#Requires -Version 5.1
<#
.SYNOPSIS
    Full end-to-end smoke test for ordinary-Agent semantic EQ control.
    Starts kernel + agent, creates a test project, loads TDR Nova,
    then runs eq_nl_agent_smoke.py to verify discussion-only routing plus
    typed semantic action -> frozen Proposal -> confirmation -> governed
    apply_eq_edits -> readback/verification for actionable listening goals.

.EXAMPLE
    D:\Vit_DAW\scripts\run_eq_nl_agent_smoke.ps1 -RepoRoot D:\Vit_DAW
    D:\Vit_DAW\scripts\run_eq_nl_agent_smoke.ps1 -RepoRoot D:\Vit_DAW -SkipBuild -ReuseKernel -ReuseAgent
    D:\Vit_DAW\scripts\run_eq_nl_agent_smoke.ps1 -RepoRoot D:\Vit_DAW -TrackId 1007 -PluginId 1013
#>
param(
    [string]$RepoRoot       = "D:\Vit_DAW",
    [string]$AgentHttp      = "http://127.0.0.1:7878",
    [string]$ZmqReqPort     = "5555",
    [string]$ZmqSubPort     = "5556",
    [string]$TdrNovaPath    = "C:\Program Files\Common Files\VST3\TDR Nova.vst3",
    [string]$TrackId        = "",
    [string]$PluginId       = "",
    [string]$OutputDir      = "",
    [float]$TimeoutSec      = 120,
    [int]$WaitSeconds       = 30,
    [switch]$SkipBuild,
    [switch]$ReuseKernel,
    [switch]$ReuseAgent,
    [switch]$KeepProcesses
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$RepoRoot = (Resolve-Path -LiteralPath $RepoRoot).Path

function Fail([string]$msg) { Write-Host "FAIL: $msg" -ForegroundColor Red; throw $msg }
function Write-Ok([string]$msg) { Write-Host "ok: $msg" -ForegroundColor Green }
function Write-Step([string]$msg) { Write-Host "`n== $msg" -ForegroundColor Cyan }
function Write-Warn([string]$msg) { Write-Host "warn: $msg" -ForegroundColor Yellow }

function Get-OptionalProperty { param([object]$Object, [string]$Name)
    if ($null -eq $Object) { return $null }
    $p = $Object.PSObject.Properties[$Name]; if ($null -eq $p) { return $null }; return $p.Value
}

function Get-TcpListener([int]$Port) {
    return Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue | Select-Object -First 1
}

function Wait-TcpListener([int]$Port, [int]$TimeoutSec) {
    $dl = (Get-Date).AddSeconds($TimeoutSec)
    do {
        $l = Get-TcpListener $Port; if ($null -ne $l) { return $l }
        Start-Sleep -Milliseconds 500
    } while ((Get-Date) -lt $dl)
    return $null
}

function Invoke-Json([string]$Method, [string]$Uri, [object]$Body = $null, [int]$TimeoutSec = 60) {
    if ($Method -eq "GET") {
        $r = Invoke-WebRequest -UseBasicParsing -Method GET -Uri $Uri -TimeoutSec $TimeoutSec
    } else {
        $json = $Body | ConvertTo-Json -Depth 20 -Compress
        $bytes = [System.Text.Encoding]::UTF8.GetBytes($json)
        $r = Invoke-WebRequest -UseBasicParsing -Method POST -Uri $Uri -Body $bytes `
            -ContentType "application/json; charset=utf-8" -TimeoutSec $TimeoutSec
    }
    if ([string]::IsNullOrWhiteSpace($r.Content)) { return $null }
    return $r.Content | ConvertFrom-Json
}

function Invoke-Tool([string]$Tool, [hashtable]$ToolArgs = @{}, [bool]$Confirmed = $true) {
    return Invoke-Json -Method POST -Uri ($AgentHttp.TrimEnd("/") + "/agent/invoke") -Body @{
        tool = $Tool; args = $ToolArgs; confirmed = $Confirmed; source = "eq_nl_smoke"
    } -TimeoutSec 60
}

function Assert-Ok([object]$Resp, [string]$Label) {
    $st = [string](Get-OptionalProperty -Object $Resp -Name "status")
    if ($st -notin @("ok","success","completed")) {
        $err = [string](Get-OptionalProperty -Object $Resp -Name "error")
        Fail "$Label failed: status=$st error=$err"
    }
}

# ------------------------------------------------------------------
# 1. Build VitAgent
# ------------------------------------------------------------------
$AgentDir = Join-Path $RepoRoot "agent"
$AgentExe = Join-Path $AgentDir "bin\VitAgent.exe"
$KernelExe = ""
foreach ($c in @(
    (Join-Path $RepoRoot "VitApp\build_release\VitApp_artefacts\Release\VitApp.exe"),
    (Join-Path $RepoRoot "VitApp\build\VitApp_artefacts\Release\VitApp.exe")
)) { if (Test-Path -LiteralPath $c) { $KernelExe = (Resolve-Path -LiteralPath $c).Path; break } }
if ([string]::IsNullOrWhiteSpace($KernelExe)) { Fail "Cannot find VitApp.exe; pass -KernelExe or build first" }

Write-Step "Build VitAgent"
if (-not $SkipBuild) {
    Push-Location $AgentDir
    try {
        & go build -o bin/VitAgent.exe ./cmd/vitagent 2>&1 | ForEach-Object { Write-Host $_ }
        if ($LASTEXITCODE -ne 0) { Fail "go build failed (exit $LASTEXITCODE)" }
        Write-Ok "VitAgent built"
    } finally { Pop-Location }
} else { Write-Ok "SkipBuild — using existing binary" }
if (-not (Test-Path $AgentExe)) { Fail "VitAgent.exe not found: $AgentExe" }

# ------------------------------------------------------------------
# 2. Start or reuse kernel
# ------------------------------------------------------------------
Write-Step "Prepare kernel"
$zmqPort = [int]$ZmqReqPort
$listener = Get-TcpListener -Port $zmqPort
if ($null -ne $listener) {
    if ($ReuseKernel) { Write-Ok "reusing existing kernel port $zmqPort pid=$($listener.OwningProcess)" }
    else {
        $proc = Get-Process -Id $listener.OwningProcess -ErrorAction SilentlyContinue
        $runPath = if ($null -ne $proc) { [string]$proc.Path } else { "" }
        if ($runPath -and [IO.Path]::GetFullPath($runPath) -eq [IO.Path]::GetFullPath($KernelExe)) {
            Write-Ok "desired kernel already running port $zmqPort pid=$($listener.OwningProcess)"
            $ReuseKernel = $true
        } else {
            Write-Warn "stopping existing kernel pid=$($listener.OwningProcess)"
            Stop-Process -Id $listener.OwningProcess -Force; Start-Sleep -Milliseconds 800
        }
    }
}
if (-not $ReuseKernel) {
    if (-not (Get-TcpListener -Port $zmqPort)) {
        Start-Process -FilePath $KernelExe -WorkingDirectory (Split-Path -Parent $KernelExe) -WindowStyle Hidden | Out-Null
        $ready = Wait-TcpListener -Port $zmqPort -TimeoutSec $WaitSeconds
        if ($null -eq $ready) { Fail "Kernel port $zmqPort not ready after ${WaitSeconds}s" }
        Write-Ok "kernel started pid=$($ready.OwningProcess)"
    }
}

# ------------------------------------------------------------------
# 3. Start or reuse VitAgent
# ------------------------------------------------------------------
Write-Step "Prepare VitAgent"
$agentReady = $false
try {
    $h = Invoke-Json -Method GET -Uri "$AgentHttp/health" -TimeoutSec 4
    if ([string](Get-OptionalProperty $h "status") -in @("ok","ready")) { $agentReady = $true }
} catch { }
if ($agentReady) { Write-Ok "agent already ready at $AgentHttp" }
elseif ($ReuseAgent) { Fail "Agent not running and -ReuseAgent set; nothing to reuse" }
else {
    Start-Process -FilePath $AgentExe -WindowStyle Hidden -PassThru | Out-Null
    $dl = (Get-Date).AddSeconds(30)
    while ((Get-Date) -lt $dl) {
        Start-Sleep -Milliseconds 600
        try {
            $h = Invoke-Json -Method GET -Uri "$AgentHttp/health" -TimeoutSec 3
            if ([string](Get-OptionalProperty $h "status") -in @("ok","ready")) { $agentReady = $true; break }
        } catch { }
    }
    if (-not $agentReady) { Fail "VitAgent did not become ready after 30s at $AgentHttp" }
    Write-Ok "VitAgent ready"
}

# ------------------------------------------------------------------
# 4. Create test project + import audio (if no explicit track/plugin)
# ------------------------------------------------------------------
$TargetTrackId  = $TrackId
$TargetPluginId = $PluginId

if ([string]::IsNullOrWhiteSpace($TargetTrackId) -or [string]::IsNullOrWhiteSpace($TargetPluginId)) {
    Write-Step "Create fixture project"

    # Reset to a fresh project
    $newp = Invoke-Tool "project.new" -ToolArgs @{} -Confirmed $true
    $nst  = [string](Get-OptionalProperty $newp "status")
    if ($nst -eq "ok") { Write-Ok "project.new"; Start-Sleep -Milliseconds 500 }
    else {
        $cl = Invoke-Tool "project.clear" -ToolArgs @{} -Confirmed $true
        Assert-Ok $cl "project.clear"
    }

    # Create audio track
    $ta = Invoke-Tool "track.add_audio" -ToolArgs @{ name = "EQ Smoke Track" } -Confirmed $true
    Assert-Ok $ta "track.add_audio"
    $taResult = Get-OptionalProperty $ta "result"
    $TargetTrackId = [string](Get-OptionalProperty $taResult "track_id")
    if ([string]::IsNullOrWhiteSpace($TargetTrackId)) {
        $TargetTrackId = [string](Get-OptionalProperty $taResult "id")
    }
    if ([string]::IsNullOrWhiteSpace($TargetTrackId)) { Fail "track.add_audio returned no track_id" }
    Write-Ok "created track $TargetTrackId"

    # Import audio so the kernel can observe
    $audioPath = Join-Path $RepoRoot "test_100hz_10s.wav"
    if (-not (Test-Path $audioPath)) { Fail "Fixture audio not found: $audioPath" }
    $imp = Invoke-Tool "clip.import_media_to_track" -ToolArgs @{
        track_id = $TargetTrackId; file_path = $audioPath; start_time = 0
        media_type = "audio"; mode = "non_destructive"
    } -Confirmed $true
    $impSt = [string](Get-OptionalProperty $imp "status")
    if ($impSt -ne "ok") {
        Write-Warn "clip.import_media_to_track not ok ($impSt), trying clip.import_audio fallback"
        $imp = Invoke-Tool "clip.import_audio" -ToolArgs @{
            track_id = $TargetTrackId; file_path = $audioPath; offset_time = 0
        } -Confirmed $true
        Assert-Ok $imp "clip.import_audio"
    }
    Write-Ok "imported audio on track $TargetTrackId"

    # Load TDR Nova onto the track
    Write-Step "Load TDR Nova"
    if (-not (Test-Path $TdrNovaPath)) { Fail "TDR Nova not found at $TdrNovaPath; pass -TdrNovaPath" }
    $rack = Invoke-Tool "plugin.load_to_rack" -ToolArgs @{
        track_id = $TargetTrackId; plugin_path = $TdrNovaPath
    } -Confirmed $true
    Assert-Ok $rack "plugin.load_to_rack"
    $rackResult = Get-OptionalProperty $rack "result"
    $TargetPluginId = [string](Get-OptionalProperty $rackResult "plugin_id")
    if ([string]::IsNullOrWhiteSpace($TargetPluginId)) {
        $TargetPluginId = [string](Get-OptionalProperty $rackResult "node_id")
    }
    if ([string]::IsNullOrWhiteSpace($TargetPluginId)) {
        # Fall back: query project state to find the new plugin
        Start-Sleep -Milliseconds 600
        $st2 = Invoke-Tool "project.state" -ToolArgs @{} -Confirmed $false
        Assert-Ok $st2 "project.state"
        $tracks2 = @($st2.result.tracks | Where-Object { $_ -ne $null })
        foreach ($t2 in $tracks2) {
            $tid2 = [string](Get-OptionalProperty $t2 "track_id")
            if ($tid2 -ne $TargetTrackId) { continue }
            $plugins2 = @($t2.plugins | Where-Object { $_ -ne $null })
            if ($plugins2.Count -gt 0) {
                $TargetPluginId = [string](Get-OptionalProperty $plugins2[0] "plugin_id")
                if ([string]::IsNullOrWhiteSpace($TargetPluginId)) {
                    $TargetPluginId = [string](Get-OptionalProperty $plugins2[0] "id")
                }
                break
            }
        }
    }
    if ([string]::IsNullOrWhiteSpace($TargetPluginId)) { Fail "Could not get plugin_id for TDR Nova after loading" }
    Write-Ok "TDR Nova loaded: track=$TargetTrackId plugin=$TargetPluginId"

    # Give the kernel a moment to settle
    Start-Sleep -Milliseconds 500
}

# ------------------------------------------------------------------
# 5. Run the Python ordinary-Agent semantic EQ smoke script
# ------------------------------------------------------------------
Write-Step "Run ordinary-Agent semantic EQ smoke (track=$TargetTrackId plugin=$TargetPluginId)"

$pyScript = Join-Path $RepoRoot "scripts\eq_nl_agent_smoke.py"
if (-not (Test-Path $pyScript)) { Fail "eq_nl_agent_smoke.py not found at $pyScript" }
if ([string]::IsNullOrWhiteSpace($OutputDir)) {
    $OutputDir = Join-Path ([IO.Path]::GetTempPath()) ("vit_semantic_eq_smoke_" + (Get-Date -Format "yyyyMMdd_HHmmss"))
}

& python $pyScript `
    --agent-http $AgentHttp `
    --track-id   $TargetTrackId `
    --plugin-id  $TargetPluginId `
    --plugin-name "TDR Nova" `
    --timeout-sec ([int]$TimeoutSec) `
    --output-dir $OutputDir

$exitCode = $LASTEXITCODE

Write-Host ""
if ($exitCode -eq 0) {
    Write-Ok "ordinary-Agent semantic EQ smoke PASSED. Evidence: $OutputDir"
} else {
    Fail "ordinary-Agent semantic EQ smoke FAILED (exit $exitCode). Evidence: $OutputDir"
}
