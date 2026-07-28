#Requires -Version 5.1
<#
.SYNOPSIS
    Runs the read-only Waves EQ topology census through Godot -> Kernel -> Agent.

.DESCRIPTION
    The run uses an isolated project XML and timestamped artifact directory.
    Existing workspace settings, project, and bridge log are hashed before and
    after. Production EQ source files are also hashed and must remain identical.
#>
param(
    [string]$RepoRoot = "D:\Vit_DAW",
    [string]$GodotExe = "D:\Godot\Godot_v4.6.1-stable_win64.exe",
    [string]$GodotProject = "D:\Godot\project\vit-daw-frontend",
    [string]$ConfigPath = "",
    [string]$Only = "",
    [int]$WaitSeconds = 90,
    [int]$TimeoutSec = 240,
    [switch]$SkipBuild,
    [switch]$KeepGodot
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Fail([string]$Message) { throw $Message }
function Step([string]$Message) { Write-Host ""; Write-Host "== $Message" -ForegroundColor Cyan }
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
function Get-HashMap([string[]]$Paths) {
    $out = [ordered]@{}
    foreach ($path in $Paths) { $out[$path] = Get-HashOrEmpty $path }
    return $out
}
function Compare-HashMaps($Before, $After) {
    $changed = @()
    foreach ($key in $Before.Keys) {
        if ([string]$Before[$key] -ne [string]$After[$key]) { $changed += $key }
    }
    return $changed
}

$RepoRoot = (Resolve-Path -LiteralPath $RepoRoot).Path
$GodotProject = (Resolve-Path -LiteralPath $GodotProject).Path
$GodotExe = (Resolve-Path -LiteralPath $GodotExe).Path
if ([string]::IsNullOrWhiteSpace($ConfigPath)) {
    $ConfigPath = Join-Path $RepoRoot "scripts\waves_eq_topology_census.json"
}
$ConfigPath = (Resolve-Path -LiteralPath $ConfigPath).Path
$pythonScript = Join-Path $RepoRoot "scripts\waves_eq_topology_census.py"
if (-not (Test-Path -LiteralPath $pythonScript)) { Fail "Missing census runner: $pythonScript" }

$stamp = Get-Date -Format "yyyyMMdd_HHmmss"
$runDir = Join-Path $RepoRoot ("artifacts\waves_eq_topology_census\" + $stamp)
$captureDir = Join-Path $runDir "raw"
$null = New-Item -ItemType Directory -Force -Path $captureDir
$projectXml = Join-Path $runDir "waves_eq_topology_project.xml"
$godotStdout = Join-Path $runDir "godot.stdout.log"
$godotStderr = Join-Path $runDir "godot.stderr.log"

$protectedRuntimeFiles = @(
    (Join-Path $RepoRoot "VitApp\Workspace\default_project.xml"),
    (Join-Path $RepoRoot "VitApp\Workspace\Settings\Settings.xml"),
    (Join-Path $RepoRoot "VitApp\Workspace\Logs\bridge_last_dev.log")
)
$productionEQFiles = @(
    (Join-Path $RepoRoot "agent\internal\workflows\plugingrabber\eq_structural_matrix.go"),
    (Join-Path $RepoRoot "agent\internal\workflows\plugingrabber\eq_band_summary.go"),
    (Join-Path $RepoRoot "agent\internal\chat\plugin_eq_control.go")
)
$protectedBefore = Get-HashMap $protectedRuntimeFiles
$productionBefore = Get-HashMap $productionEQFiles
$gitStatusBefore = (& git -C $RepoRoot status --porcelain=v1) -join [Environment]::NewLine

$kernelLogDir = Join-Path $RepoRoot "VitApp\Workspace\Logs"
$kernelLogsBefore = @{}
if (Test-Path -LiteralPath $kernelLogDir) {
    Get-ChildItem -LiteralPath $kernelLogDir -File -Filter "VitHeadlessServer*.log" | ForEach-Object {
        $kernelLogsBefore[$_.FullName] = $true
    }
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

$runError = $null
try {
    Step "Preflight isolated real runtime"
    Assert-PortsFree @(5555, 5556, 7878, 8787)
    Ok "kernel, Agent, and hub ports are free"

    Step "Build the VitAgent binary used by Godot"
    if (-not $SkipBuild) {
        Push-Location (Join-Path $RepoRoot "agent")
        try {
            & go build -o bin/VitAgent.exe ./cmd/vitagent
            if ($LASTEXITCODE -ne 0) { Fail "go build failed with exit code $LASTEXITCODE" }
        } finally { Pop-Location }
    }
    $agentExe = Join-Path $RepoRoot "agent\bin\VitAgent.exe"
    if (-not (Test-Path -LiteralPath $agentExe)) { Fail "VitAgent binary is missing: $agentExe" }
    Ok "VitAgent is ready"

    foreach ($key in $envOverrides.Keys) {
        $priorEnv[$key] = [Environment]::GetEnvironmentVariable($key, "Process")
        [Environment]::SetEnvironmentVariable($key, [string]$envOverrides[$key], "Process")
    }

    Step "Launch Godot -> Kernel -> VspHub -> VitAgent"
    $godotProcess = Start-Process -FilePath $GodotExe -ArgumentList @("--path", $GodotProject) -WorkingDirectory $GodotProject -WindowStyle Hidden -RedirectStandardOutput $godotStdout -RedirectStandardError $godotStderr -PassThru
    if (-not (Wait-HttpReady "http://127.0.0.1:7878/health" $WaitSeconds)) {
        Fail "VitAgent did not become ready through Godot within $WaitSeconds seconds"
    }
    Ok "Godot-launched VitAgent is ready"

    Step "Capture complete paged Waves parameter surfaces"
    $pythonArgs = @(
        $pythonScript,
        "--config", $ConfigPath,
        "--output-dir", $captureDir,
        "--timeout-sec", [string]$TimeoutSec
    )
    if (-not [string]::IsNullOrWhiteSpace($Only)) { $pythonArgs += @("--only", $Only) }
    & python @pythonArgs
    if ($LASTEXITCODE -ne 0) { Fail "census runner failed with exit code $LASTEXITCODE" }
    Ok "read-only Waves census completed"
}
catch {
    $runError = $_
}
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
        Start-Sleep -Milliseconds 800
    }

    if (Test-Path -LiteralPath $kernelLogDir) {
        $newLogs = @(Get-ChildItem -LiteralPath $kernelLogDir -File -Filter "VitHeadlessServer*.log" | Where-Object {
            -not $kernelLogsBefore.ContainsKey($_.FullName)
        })
        if ($newLogs.Count -gt 0) {
            $kernelArtifactDir = Join-Path $runDir "kernel_logs"
            $null = New-Item -ItemType Directory -Force -Path $kernelArtifactDir
            $resolvedRun = [System.IO.Path]::GetFullPath($runDir)
            $resolvedKernelArtifacts = [System.IO.Path]::GetFullPath($kernelArtifactDir)
            if (-not $resolvedKernelArtifacts.StartsWith($resolvedRun, [System.StringComparison]::OrdinalIgnoreCase)) {
                Fail "kernel log artifact target escaped the run directory"
            }
            foreach ($log in $newLogs) {
                Move-Item -LiteralPath $log.FullName -Destination $kernelArtifactDir -Force
            }
        }
    }

    $protectedAfter = Get-HashMap $protectedRuntimeFiles
    $productionAfter = Get-HashMap $productionEQFiles
    $protectedChanged = @(Compare-HashMaps $protectedBefore $protectedAfter)
    $productionChanged = @(Compare-HashMaps $productionBefore $productionAfter)
    $gitStatusAfter = (& git -C $RepoRoot status --porcelain=v1) -join [Environment]::NewLine
    $manifest = [ordered]@{
        schema_version = "waves.eq_topology_run_manifest.v1"
        timestamp = $stamp
        repo_root = $RepoRoot
        branch = (& git -C $RepoRoot branch --show-current)
        head = (& git -C $RepoRoot rev-parse HEAD)
        config_path = $ConfigPath
        only = $Only
        protected_runtime_before = $protectedBefore
        protected_runtime_after = $protectedAfter
        protected_runtime_changed = $protectedChanged
        production_eq_before = $productionBefore
        production_eq_after = $productionAfter
        production_eq_changed = $productionChanged
        git_status_before = $gitStatusBefore
        git_status_after = $gitStatusAfter
        run_error = if ($null -eq $runError) { "" } else { [string]$runError }
    }
    $manifest | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath (Join-Path $runDir "run_manifest.json") -Encoding UTF8

    if ($protectedChanged.Count -gt 0) {
        if ($null -eq $runError) { $runError = "Protected runtime files changed: $($protectedChanged -join ', ')" }
    }
    if ($productionChanged.Count -gt 0) {
        if ($null -eq $runError) { $runError = "Production EQ files changed: $($productionChanged -join ', ')" }
    }
    Write-Host "artifacts: $runDir"
}

if ($null -ne $runError) { throw $runError }
Ok "protected runtime and production EQ files remained byte-identical"
