#Requires -Version 5.1
<#
.SYNOPSIS
    Runs the EQ compatibility matrix through the real Godot startup chain.

.DESCRIPTION
    Godot starts VitApp, VspHub, and VitAgent exactly as the development UI
    does. The real D:\Vit_DAW Workspace/Settings and installed plug-ins remain in
    use. Only VIT_PROJECT_XML and smoke log outputs are redirected so an
    automated run cannot replace the current project or existing logs.
#>
param(
    [string]$RepoRoot = "D:\Vit_DAW",
    [string]$GodotExe = "D:\Godot\Godot_v4.6.1-stable_win64.exe",
    [string]$GodotProject = "D:\Godot\project\vit-daw-frontend",
    [string]$ConfigPath = "",
    [ValidateSet("capture", "smoke")]
    [string]$Mode = "capture",
    [string]$Only = "",
    [int]$WaitSeconds = 60,
    [int]$TimeoutSec = 180,
    [switch]$SkipBuild,
    [switch]$KeepGodot
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Fail([string]$Message) { throw $Message }
function Step([string]$Message) { Write-Host "`n== $Message" -ForegroundColor Cyan }
function Ok([string]$Message) { Write-Host "ok: $Message" -ForegroundColor Green }
function Get-HashOrEmpty([string]$Path) {
    if (-not (Test-Path -LiteralPath $Path)) { return "" }
    return (Get-FileHash -Algorithm SHA256 -LiteralPath $Path).Hash
}
function Wait-HttpReady([string]$Uri, [int]$Seconds) {
    $deadline = (Get-Date).AddSeconds($Seconds)
    do {
        try {
            $response = Invoke-WebRequest -UseBasicParsing -Method GET -Uri $Uri -TimeoutSec 3
            if ($response.StatusCode -eq 200) { return $true }
        } catch { }
        Start-Sleep -Milliseconds 500
    } while ((Get-Date) -lt $deadline)
    return $false
}
function Assert-PortsFree([int[]]$Ports) {
    foreach ($port in $Ports) {
        $listener = Get-NetTCPConnection -State Listen -LocalPort $port -ErrorAction SilentlyContinue | Select-Object -First 1
        if ($null -ne $listener) {
            Fail "Port $port is already owned by PID $($listener.OwningProcess); refusing to disturb an existing session"
        }
    }
}

$RepoRoot = (Resolve-Path -LiteralPath $RepoRoot).Path
$GodotProject = (Resolve-Path -LiteralPath $GodotProject).Path
$GodotExe = (Resolve-Path -LiteralPath $GodotExe).Path
if ([string]::IsNullOrWhiteSpace($ConfigPath)) {
    $ConfigPath = Join-Path $RepoRoot "scripts\eq_compat_matrix.json"
}
$ConfigPath = (Resolve-Path -LiteralPath $ConfigPath).Path
$pythonScript = Join-Path $RepoRoot "scripts\eq_compat_matrix_smoke.py"
if (-not (Test-Path -LiteralPath $pythonScript)) { Fail "Missing Python runner: $pythonScript" }

$stamp = Get-Date -Format "yyyyMMdd_HHmmss"
$runDir = Join-Path $RepoRoot ("artifacts\eq_compat_matrix\" + $stamp)
$null = New-Item -ItemType Directory -Force -Path $runDir
$captureDir = Join-Path $runDir "capture"
$null = New-Item -ItemType Directory -Force -Path $captureDir
$projectXml = Join-Path $runDir "eq_compat_matrix_project.xml"
$godotStdout = Join-Path $runDir "godot.stdout.log"
$godotStderr = Join-Path $runDir "godot.stderr.log"

$protected = @(
    (Join-Path $RepoRoot "VitApp\Workspace\default_project.xml"),
    (Join-Path $RepoRoot "VitApp\Workspace\Settings\Settings.xml"),
    (Join-Path $RepoRoot "VitApp\Workspace\Logs\bridge_last_dev.log")
)
$protectedHashes = @{}
foreach ($path in $protected) { $protectedHashes[$path] = Get-HashOrEmpty $path }
$kernelLogsBefore = @{}
$kernelLogDir = Join-Path $RepoRoot "VitApp\Workspace\Logs"
if (Test-Path -LiteralPath $kernelLogDir) {
    Get-ChildItem -LiteralPath $kernelLogDir -File -Filter "VitHeadlessServer*.log" | ForEach-Object { $kernelLogsBefore[$_.FullName] = $true }
}

$godotProcess = $null
$priorEnv = @{}
$envOverrides = @{
    "VIT_DEV_ROOT" = $RepoRoot
    "VIT_ROOT" = $RepoRoot
    "VIT_PROJECT_XML" = $projectXml
    "VIT_AGENT_LAST_LOG_PATH" = (Join-Path $runDir "agent_last.log")
    "VIT_AGENT_JOURNAL_PATH" = (Join-Path $runDir "agent_journal.json")
    "VIT_AGENT_LLM_TELEMETRY_PATH" = (Join-Path $runDir "agent_llm_telemetry.jsonl")
    "VIT_AGENT_MESSAGE_LOOP_DEBUG_PATH" = (Join-Path $runDir "agent_message_loop_debug.jsonl")
    "VIT_VSP_HUB_LAST_LOG_PATH" = (Join-Path $runDir "vsp_hub_last.log")
    "VIT_BRIDGE_LAST_LOG_PATH" = (Join-Path $runDir "bridge_last.log")
}

try {
    Step "Preflight real Godot runtime"
    Assert-PortsFree @(5555, 5556, 7878, 8787)
    Ok "kernel, agent, and hub ports are free"

    Step "Build VitAgent used by Godot"
    if (-not $SkipBuild) {
        Push-Location (Join-Path $RepoRoot "agent")
        try {
            & go build -o bin/VitAgent.exe ./cmd/vitagent
            if ($LASTEXITCODE -ne 0) { Fail "go build failed with exit code $LASTEXITCODE" }
        } finally { Pop-Location }
    }
    if (-not (Test-Path -LiteralPath (Join-Path $RepoRoot "agent\bin\VitAgent.exe"))) {
        Fail "Godot development VitAgent binary is missing"
    }
    Ok "VitAgent is ready"

    foreach ($key in $envOverrides.Keys) {
        $priorEnv[$key] = [Environment]::GetEnvironmentVariable($key, "Process")
        [Environment]::SetEnvironmentVariable($key, [string]$envOverrides[$key], "Process")
    }

    Step "Launch real Godot -> kernel -> VspHub -> VitAgent chain"
    $godotProcess = Start-Process -FilePath $GodotExe `
        -ArgumentList @("--path", $GodotProject) `
        -WorkingDirectory $GodotProject `
        -RedirectStandardOutput $godotStdout `
        -RedirectStandardError $godotStderr `
        -PassThru
    if (-not (Wait-HttpReady "http://127.0.0.1:7878/health" $WaitSeconds)) {
        Fail "VitAgent did not become ready through Godot within $WaitSeconds seconds"
    }
    Ok "Godot-launched VitAgent is ready"

    Step "Run EQ compatibility matrix ($Mode)"
    $pythonArgs = @(
        $pythonScript,
        "--config", $ConfigPath,
        "--output-dir", $captureDir,
        "--mode", $Mode,
        "--timeout-sec", [string]$TimeoutSec
    )
    if (-not [string]::IsNullOrWhiteSpace($Only)) { $pythonArgs += @("--only", $Only) }
    & python @pythonArgs
    if ($LASTEXITCODE -ne 0) { Fail "EQ compatibility matrix failed with exit code $LASTEXITCODE" }
    Ok "EQ compatibility matrix completed"
}
finally {
    foreach ($key in $envOverrides.Keys) {
        [Environment]::SetEnvironmentVariable($key, $priorEnv[$key], "Process")
    }

    if ($null -ne $godotProcess -and -not $KeepGodot) {
        try { $null = $godotProcess.CloseMainWindow() } catch { }
        try { if (-not $godotProcess.WaitForExit(12000)) { Stop-Process -Id $godotProcess.Id -Force -ErrorAction SilentlyContinue } } catch { }
        Start-Sleep -Milliseconds 800
    }

    if (Test-Path -LiteralPath $kernelLogDir) {
        Get-ChildItem -LiteralPath $kernelLogDir -File -Filter "VitHeadlessServer*.log" | ForEach-Object {
            if (-not $kernelLogsBefore.ContainsKey($_.FullName)) {
                Remove-Item -LiteralPath $_.FullName -Force -ErrorAction SilentlyContinue
            }
        }
    }

    $changed = @()
    foreach ($path in $protected) {
        $after = Get-HashOrEmpty $path
        if ($after -ne $protectedHashes[$path]) { $changed += $path }
    }
    if ($changed.Count -gt 0) {
        Write-Error ("Protected runtime files changed: " + ($changed -join ", "))
    } else {
        Ok "protected runtime files remained byte-identical"
    }
    Write-Host "artifacts: $runDir"
}
