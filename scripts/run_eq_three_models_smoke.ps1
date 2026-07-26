#Requires -Version 5.1
<##
.SYNOPSIS
    Live smoke test for all three Plugin Grabber EQ operation models.

    Creates isolated tracks and loads TDR Nova, Voxengo Marvel GEQ, and
    FabFilter Pro-Q 3, then runs eq_three_models_smoke.py.
##>
param(
    [string]$RepoRoot    = "D:\Vit_DAW",
    [string]$AgentHttp   = "http://127.0.0.1:7878",
    [string]$ZmqReqPort  = "5555",
    [string]$TdrNovaPath = "C:\Program Files\Common Files\VST3\TDR Nova.vst3",
    [string]$MarvelPath  = "C:\Program Files\Common Files\VST3\Marvel GEQ.vst3",
    [string]$ProQPath    = "C:\Program Files\Common Files\VST3\FabFilter\FabFilter Pro-Q 3.vst3",
    [int]$TimeoutSec     = 180,
    [int]$WaitSeconds    = 45,
    [switch]$SkipBuild,
    [switch]$ReuseKernel,
    [switch]$ReuseAgent
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$RepoRoot = (Resolve-Path -LiteralPath $RepoRoot).Path

function Fail([string]$Message) { throw $Message }
function Step([string]$Message) { Write-Host "`n== $Message" -ForegroundColor Cyan }
function Ok([string]$Message) { Write-Host "ok: $Message" -ForegroundColor Green }
function Prop([object]$Object, [string]$Name) {
    if ($null -eq $Object) { return $null }
    $p = $Object.PSObject.Properties[$Name]
    if ($null -eq $p) { return $null }
    return $p.Value
}
function Invoke-Json([string]$Method, [string]$Uri, [object]$Body = $null, [int]$Timeout = 120) {
    if ($Method -eq "GET") {
        $r = Invoke-WebRequest -UseBasicParsing -Method GET -Uri $Uri -TimeoutSec $Timeout
    } else {
        $bytes = [Text.Encoding]::UTF8.GetBytes(($Body | ConvertTo-Json -Depth 30 -Compress))
        $r = Invoke-WebRequest -UseBasicParsing -Method POST -Uri $Uri -Body $bytes -ContentType "application/json; charset=utf-8" -TimeoutSec $Timeout
    }
    if ([string]::IsNullOrWhiteSpace($r.Content)) { return $null }
    return $r.Content | ConvertFrom-Json
}
function Invoke-Tool([string]$Tool, [hashtable]$ToolArgs = @{}, [bool]$Confirmed = $true) {
    return Invoke-Json POST ($AgentHttp.TrimEnd("/") + "/agent/invoke") @{ tool = $Tool; args = $ToolArgs; confirmed = $Confirmed; source = "eq_three_models_smoke" } 240
}
function Assert-Ok([object]$Response, [string]$Label) {
    $status = [string](Prop $Response "status")
    if ($status -notin @("ok", "success", "completed")) {
        Fail "$Label failed: status=$status error=$([string](Prop $Response 'error'))"
    }
}
function Wait-Port([int]$Port, [int]$Seconds) {
    $deadline = (Get-Date).AddSeconds($Seconds)
    do {
        $listener = Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue | Select-Object -First 1
        if ($null -ne $listener) { return $listener }
        Start-Sleep -Milliseconds 500
    } while ((Get-Date) -lt $deadline)
    return $null
}

$agentDir = Join-Path $RepoRoot "agent"
$agentExe = Join-Path $agentDir "bin\VitAgent.exe"
$kernelExe = @(
    (Join-Path $RepoRoot "VitApp\build_release\VitApp_artefacts\Release\VitApp.exe"),
    (Join-Path $RepoRoot "VitApp\build\VitApp_artefacts\Release\VitApp.exe")
) | Where-Object { Test-Path -LiteralPath $_ } | Select-Object -First 1
if ([string]::IsNullOrWhiteSpace($kernelExe)) { Fail "VitApp.exe not found" }
foreach ($path in @($TdrNovaPath, $MarvelPath, $ProQPath)) {
    if (-not (Test-Path -LiteralPath $path)) { Fail "Plugin not found: $path" }
}

Step "Build VitAgent"
Push-Location $agentDir
try {
    if (-not $SkipBuild) {
        & go build -o bin/VitAgent.exe ./cmd/vitagent 2>&1 | ForEach-Object { Write-Host $_ }
        if ($LASTEXITCODE -ne 0) { Fail "go build failed: $LASTEXITCODE" }
        Ok "VitAgent built"
    } else { Ok "using existing VitAgent" }
} finally { Pop-Location }
if (-not (Test-Path -LiteralPath $agentExe)) { Fail "VitAgent.exe not found: $agentExe" }

Step "Prepare VitApp"
$port = [int]$ZmqReqPort
$listener = Get-NetTCPConnection -LocalPort $port -State Listen -ErrorAction SilentlyContinue | Select-Object -First 1
if ($null -eq $listener) {
    Start-Process -FilePath $kernelExe -WorkingDirectory (Split-Path -Parent $kernelExe) -WindowStyle Hidden | Out-Null
    if ($null -eq (Wait-Port $port $WaitSeconds)) { Fail "VitApp did not listen on $port" }
    Ok "VitApp started"
} elseif ($ReuseKernel) {
    Ok "reusing VitApp on port $port"
} else {
    Ok "VitApp already listening on port $port; reusing it"
}

Step "Prepare VitAgent"
$ready = $false
try { $h = Invoke-Json GET ($AgentHttp.TrimEnd("/") + "/health") $null 5; $ready = [string](Prop $h "status") -in @("ok", "ready") } catch { }
if (-not $ready) {
    if ($ReuseAgent) { Fail "-ReuseAgent set but Agent is not ready" }
    Start-Process -FilePath $agentExe -WorkingDirectory (Split-Path -Parent $agentExe) -WindowStyle Hidden | Out-Null
    $deadline = (Get-Date).AddSeconds($WaitSeconds)
    do {
        Start-Sleep -Milliseconds 600
        try { $h = Invoke-Json GET ($AgentHttp.TrimEnd("/") + "/health") $null 4; $ready = [string](Prop $h "status") -in @("ok", "ready") } catch { }
    } while (-not $ready -and (Get-Date) -lt $deadline)
    if (-not $ready) { Fail "VitAgent did not become ready" }
    Ok "VitAgent started"
} else { Ok "reusing VitAgent" }

Step "Scan VST3 index"
$scanArgs = @{ paths = @((Split-Path -Parent $TdrNovaPath)) }
$scan = Invoke-Tool -Tool "plugin.scan" -ToolArgs $scanArgs -Confirmed $true
Assert-Ok $scan "plugin.scan"
Ok "VST3 scan completed"

Step "Create isolated fixture tracks"
$newProject = Invoke-Tool "project.new" @{} $true
Assert-Ok $newProject "project.new"
Start-Sleep -Milliseconds 500

$fixtures = @(
    @{ label = "TDR Nova"; name = "EQ Model TDR"; path = $TdrNovaPath },
    @{ label = "Marvel GEQ"; name = "EQ Model Marvel"; path = $MarvelPath },
    @{ label = "Pro-Q 3"; name = "EQ Model ProQ"; path = $ProQPath }
)
$loaded = @()
foreach ($fixture in $fixtures) {
    $trackResp = Invoke-Tool "track.add_audio" @{ name = $fixture.name } $true
    Assert-Ok $trackResp ("track.add_audio " + $fixture.label)
    $trackResult = Prop $trackResp "result"
    $trackId = [string](Prop $trackResult "track_id")
    if ([string]::IsNullOrWhiteSpace($trackId)) { $trackId = [string](Prop $trackResult "id") }
    if ([string]::IsNullOrWhiteSpace($trackId)) { Fail "No track_id for $($fixture.label)" }

    $rackResp = Invoke-Tool "plugin.load_to_rack" @{ track_id = $trackId; plugin_path = $fixture.path } $true
    Assert-Ok $rackResp ("plugin.load_to_rack " + $fixture.label)
    $rackResult = Prop $rackResp "result"
    $pluginId = [string](Prop $rackResult "plugin_id")
    if ([string]::IsNullOrWhiteSpace($pluginId)) { $pluginId = [string](Prop $rackResult "node_id") }
    if ([string]::IsNullOrWhiteSpace($pluginId)) { Fail "No plugin_id for $($fixture.label)" }
    $loaded += [pscustomobject]@{ label = $fixture.label; track_id = $trackId; plugin_id = $pluginId }
    Ok "$($fixture.label): track=$trackId plugin=$pluginId"
    Start-Sleep -Milliseconds 600
}

Step "Run three-model Python smoke"
$tdr = $loaded | Where-Object label -eq "TDR Nova"
$marvel = $loaded | Where-Object label -eq "Marvel GEQ"
$proq = $loaded | Where-Object label -eq "Pro-Q 3"
& python (Join-Path $RepoRoot "scripts\eq_three_models_smoke.py") `
    --agent-http $AgentHttp `
    --tdr-track-id $tdr.track_id --tdr-plugin-id $tdr.plugin_id `
    --marvel-track-id $marvel.track_id --marvel-plugin-id $marvel.plugin_id `
    --proq-track-id $proq.track_id --proq-plugin-id $proq.plugin_id `
    --timeout-sec $TimeoutSec
if ($LASTEXITCODE -ne 0) { Fail "three-model smoke failed: exit $LASTEXITCODE" }
Ok "three-model smoke passed"
