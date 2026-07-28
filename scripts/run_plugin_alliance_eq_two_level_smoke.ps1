#Requires -Version 5.1
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
function Get-HashOrEmpty([string]$Path) {
    if (-not (Test-Path -LiteralPath $Path)) { return "" }
    return (Get-FileHash -Algorithm SHA256 -LiteralPath $Path).Hash
}

$RepoRoot = (Resolve-Path -LiteralPath $RepoRoot).Path
$GodotExe = (Resolve-Path -LiteralPath $GodotExe).Path
$GodotProject = (Resolve-Path -LiteralPath $GodotProject).Path
if ([string]::IsNullOrWhiteSpace($ConfigPath)) {
    $ConfigPath = Join-Path $RepoRoot "scripts\eq_plugin_alliance_second_holdout_smoke.json"
}
$ConfigPath = (Resolve-Path -LiteralPath $ConfigPath).Path
$pythonScript = Join-Path $RepoRoot "scripts\plugin_alliance_eq_two_level_smoke.py"
foreach ($port in @(5555, 5556, 7878, 8787)) {
    $listener = Get-NetTCPConnection -State Listen -LocalPort $port -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($null -ne $listener) { Fail "Port $port is occupied by PID $($listener.OwningProcess)" }
}
if (-not $SkipBuild) {
    Push-Location (Join-Path $RepoRoot "agent")
    try {
        & go build -o bin/VitAgent.exe ./cmd/vitagent
        if ($LASTEXITCODE -ne 0) { Fail "VitAgent build failed: $LASTEXITCODE" }
    } finally { Pop-Location }
}

$stamp = Get-Date -Format "yyyyMMdd_HHmmss"
$runDir = Join-Path $RepoRoot ("artifacts\plugin_alliance_eq_two_level\" + $stamp)
$null = New-Item -ItemType Directory -Force -Path $runDir
$projectXml = Join-Path $runDir "isolated_project.xml"
$protectedFiles = @(
    (Join-Path $RepoRoot "VitApp\Workspace\default_project.xml"),
    (Join-Path $RepoRoot "VitApp\Workspace\Settings\Settings.xml"),
    (Join-Path $RepoRoot "VitApp\Workspace\Logs\bridge_last_dev.log")
)
$protectedBefore = @{}
foreach ($path in $protectedFiles) { $protectedBefore[$path] = Get-HashOrEmpty $path }
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
$priorEnv = @{}
$godotProcess = $null
$runError = $null
try {
    foreach ($key in $envOverrides.Keys) {
        $priorEnv[$key] = [Environment]::GetEnvironmentVariable($key, "Process")
        [Environment]::SetEnvironmentVariable($key, [string]$envOverrides[$key], "Process")
    }
    $godotProcess = Start-Process -FilePath $GodotExe -ArgumentList @("--path", $GodotProject) `
        -WorkingDirectory $GodotProject -WindowStyle Hidden `
        -RedirectStandardOutput (Join-Path $runDir "godot.stdout.log") `
        -RedirectStandardError (Join-Path $runDir "godot.stderr.log") -PassThru
    if (-not (Wait-HttpReady "http://127.0.0.1:7878/health" $WaitSeconds)) {
        Fail "Godot-launched Agent was not ready within $WaitSeconds seconds"
    }
    & python $pythonScript --config $ConfigPath --output-dir $runDir --timeout-sec $TimeoutSec
    if ($LASTEXITCODE -ne 0) { Fail "two-level smoke failed: $LASTEXITCODE" }
} catch { $runError = $_ }
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
    $protectedAfter = @{}
    $changed = @()
    foreach ($path in $protectedFiles) {
        $protectedAfter[$path] = Get-HashOrEmpty $path
        if ([string]$protectedBefore[$path] -ne [string]$protectedAfter[$path]) { $changed += $path }
    }
    [ordered]@{
        schema_version = "plugin_alliance.eq_two_level_run_manifest.v1"
        timestamp = $stamp
        branch = (& git -C $RepoRoot branch --show-current)
        config = $ConfigPath
        protected_runtime_before = $protectedBefore
        protected_runtime_after = $protectedAfter
        protected_runtime_changed = $changed
        run_error = if ($null -eq $runError) { "" } else { [string]$runError }
    } | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath (Join-Path $runDir "run_manifest.json") -Encoding UTF8
    Write-Host "artifacts: $runDir"
}
if ($null -ne $runError) { throw $runError }
Write-Host "PASS: Plugin Alliance two-level EQ smoke" -ForegroundColor Green
