#Requires -Version 5.1
<#
.SYNOPSIS
    Runs the bounded Waves static-EQ production smoke through Godot -> Kernel -> Agent.
#>
param(
    [string]$RepoRoot = "D:\Vit_DAW",
    [string]$GodotExe = "D:\Godot\Godot_v4.6.1-stable_win64.exe",
    [string]$GodotProject = "D:\Godot\project\vit-daw-frontend",
    [string]$ConfigPath = "",
    [int]$WaitSeconds = 90,
    [int]$TimeoutSec = 240,
    [switch]$SkipBuild,
    [switch]$KeepGodot
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Fail([string]$Message) { throw $Message }
function Step([string]$Message) { Write-Host ""; Write-Host "== $Message" -ForegroundColor Cyan }
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
        if ($null -ne $listener) { Fail "Port $port is owned by PID $($listener.OwningProcess); refusing to disturb it" }
    }
}

$RepoRoot = (Resolve-Path -LiteralPath $RepoRoot).Path
$GodotExe = (Resolve-Path -LiteralPath $GodotExe).Path
$GodotProject = (Resolve-Path -LiteralPath $GodotProject).Path
if ([string]::IsNullOrWhiteSpace($ConfigPath)) { $ConfigPath = Join-Path $RepoRoot "scripts\waves_eq_topology_census.json" }
$ConfigPath = (Resolve-Path -LiteralPath $ConfigPath).Path
$pythonScript = Join-Path $RepoRoot "scripts\waves_static_eq_phase3_smoke.py"
if (-not (Test-Path -LiteralPath $pythonScript)) { Fail "Missing smoke runner: $pythonScript" }

$stamp = Get-Date -Format "yyyyMMdd_HHmmss"
$phase3Dir = Join-Path $RepoRoot "artifacts\waves_eq_topology_census\20260727_201627\analysis\static_eq_phase3"
$runDir = Join-Path $phase3Dir ("live_smoke\" + $stamp)
$null = New-Item -ItemType Directory -Force -Path $runDir
$projectXml = Join-Path $runDir "isolated_project.xml"
$godotStdout = Join-Path $runDir "godot.stdout.log"
$godotStderr = Join-Path $runDir "godot.stderr.log"
$protectedFiles = @(
    (Join-Path $RepoRoot "VitApp\Workspace\default_project.xml"),
    (Join-Path $RepoRoot "VitApp\Workspace\Settings\Settings.xml"),
    (Join-Path $RepoRoot "VitApp\Workspace\Logs\bridge_last_dev.log")
)
$protectedBefore = [ordered]@{}
foreach ($path in $protectedFiles) { $protectedBefore[$path] = Get-HashOrEmpty $path }
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
$runError = $null
try {
    Step "Preflight isolated bounded live smoke"
    Assert-PortsFree @(5555, 5556, 7878, 8787)
    if (-not $SkipBuild) {
        Step "Build current VitAgent"
        Push-Location (Join-Path $RepoRoot "agent")
        try {
            & go build -o bin/VitAgent.exe ./cmd/vitagent
            if ($LASTEXITCODE -ne 0) { Fail "VitAgent build failed with exit code $LASTEXITCODE" }
        } finally { Pop-Location }
    }
    foreach ($key in $envOverrides.Keys) {
        $priorEnv[$key] = [Environment]::GetEnvironmentVariable($key, "Process")
        [Environment]::SetEnvironmentVariable($key, [string]$envOverrides[$key], "Process")
    }
    Step "Launch Godot -> Kernel -> Agent"
    $godotProcess = Start-Process -FilePath $GodotExe -ArgumentList @("--path", $GodotProject) `
        -WorkingDirectory $GodotProject -WindowStyle Hidden -RedirectStandardOutput $godotStdout `
        -RedirectStandardError $godotStderr -PassThru
    if (-not (Wait-HttpReady "http://127.0.0.1:7878/health" $WaitSeconds)) {
        Fail "Godot-launched VitAgent did not become ready within $WaitSeconds seconds"
    }
    Step "Run five positive and three read-only rejection instances"
    & python $pythonScript --config $ConfigPath --output-dir $runDir --timeout-sec $TimeoutSec
    if ($LASTEXITCODE -ne 0) { Fail "Phase-3 smoke failed with exit code $LASTEXITCODE" }
}
catch { $runError = $_ }
finally {
    foreach ($key in $envOverrides.Keys) {
        [Environment]::SetEnvironmentVariable($key, $priorEnv[$key], "Process")
    }
    if ($null -ne $godotProcess -and -not $KeepGodot) {
        try { $null = $godotProcess.CloseMainWindow() } catch { }
        try {
            if (-not $godotProcess.WaitForExit(12000)) {
                Stop-Process -Id $godotProcess.Id -Force -ErrorAction SilentlyContinue
            }
        } catch { }
    }
    $protectedAfter = [ordered]@{}
    $protectedChanged = @()
    foreach ($path in $protectedFiles) {
        $protectedAfter[$path] = Get-HashOrEmpty $path
        if ([string]$protectedBefore[$path] -ne [string]$protectedAfter[$path]) { $protectedChanged += $path }
    }
    $manifest = [ordered]@{
        schema_version = "waves.static_eq.phase3_live_smoke_manifest.v1"
        timestamp = $stamp
        branch = (& git -C $RepoRoot branch --show-current)
        head = (& git -C $RepoRoot rev-parse HEAD)
        positive_instance_count = 5
        read_only_rejection_instance_count = 3
        protected_runtime_before = $protectedBefore
        protected_runtime_after = $protectedAfter
        protected_runtime_changed = $protectedChanged
        run_error = if ($null -eq $runError) { "" } else { [string]$runError }
    }
    $manifest | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath (Join-Path $runDir "run_manifest.json") -Encoding UTF8
    if ($protectedChanged.Count -gt 0 -and $null -eq $runError) {
        $runError = "Protected runtime files changed: $($protectedChanged -join ', ')"
    }
    Write-Host "artifacts: $runDir"
}
if ($null -ne $runError) { throw $runError }
Write-Host "PASS: bounded Waves static-EQ live smoke" -ForegroundColor Green
