#Requires -Version 5.1
[CmdletBinding()]
param(
    [ValidateSet("Matrix", "Blind", "All")]
    [string]$Phase = "Matrix",
    [string]$RepoRoot = "D:\Vit_DAW",
    [string]$GodotExe = "D:\Godot\Godot_v4.6.1-stable_win64.exe",
    [string]$GodotProject = "D:\Godot\project\vit-daw-frontend",
    [int]$TimeoutSeconds = 240,
    [switch]$KeepRuntime
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Wait-HttpReady([string]$Uri, [int]$Seconds) {
    $deadline = (Get-Date).AddSeconds($Seconds)
    do {
        try { if ((Invoke-WebRequest -UseBasicParsing -Uri $Uri -TimeoutSec 3).StatusCode -eq 200) { return $true } } catch { }
        Start-Sleep -Milliseconds 400
    } while ((Get-Date) -lt $deadline)
    return $false
}

function Wait-Port([int]$Port, [int]$Seconds) {
    $deadline = (Get-Date).AddSeconds($Seconds)
    do {
        if (Get-NetTCPConnection -State Listen -LocalPort $Port -ErrorAction SilentlyContinue) { return $true }
        Start-Sleep -Milliseconds 400
    } while ((Get-Date) -lt $deadline)
    return $false
}

$RepoRoot = (Resolve-Path -LiteralPath $RepoRoot).Path
$GodotExe = (Resolve-Path -LiteralPath $GodotExe).Path
$GodotProject = (Resolve-Path -LiteralPath $GodotProject).Path
foreach ($port in @(5555, 5556, 7878, 8787)) {
    if (Get-NetTCPConnection -State Listen -LocalPort $port -ErrorAction SilentlyContinue) {
        throw "Port $port is already in use; refusing to disturb an existing session"
    }
}

$stamp = Get-Date -Format "yyyyMMdd_HHmmss"
$runDir = Join-Path $RepoRoot ("VitApp\Workspace\Artifacts\smoke\limiter_control\" + $stamp)
$null = New-Item -ItemType Directory -Force -Path $runDir
$projectCopy = Join-Path $runDir "disposable_project.xml"
Copy-Item -LiteralPath (Join-Path $RepoRoot "VitApp\Workspace\default_project.xml") -Destination $projectCopy
$prior = @{}
$overrides = @{
    VIT_DEV_ROOT = $RepoRoot
    VIT_DAW_DEV_ROOT = $RepoRoot
    VIT_ROOT = $RepoRoot
    VIT_PROJECT_XML = $projectCopy
    VIT_MIXBOARD_ROOT = (Join-Path $runDir "mixboard")
    VIT_AGENT_LAST_LOG_PATH = (Join-Path $runDir "agent_last.log")
    VIT_AGENT_JOURNAL_PATH = (Join-Path $runDir "agent_journal.json")
    VIT_VSP_HUB_LAST_LOG_PATH = (Join-Path $runDir "vsp_hub_last.log")
}
$godot = $null
try {
    Push-Location (Join-Path $RepoRoot "agent")
    try {
        & go build -o bin/VitAgent.exe ./cmd/vitagent
        if ($LASTEXITCODE -ne 0) { throw "VitAgent build failed" }
        & go build -o bin/VspHub.exe ./cmd/vsphub
        if ($LASTEXITCODE -ne 0) { throw "VspHub build failed" }
    } finally { Pop-Location }
    foreach ($key in $overrides.Keys) {
        $prior[$key] = [Environment]::GetEnvironmentVariable($key, "Process")
        [Environment]::SetEnvironmentVariable($key, [string]$overrides[$key], "Process")
    }
    $godot = Start-Process -FilePath $GodotExe -ArgumentList @("--path", $GodotProject) `
        -WorkingDirectory $GodotProject -WindowStyle Hidden `
        -RedirectStandardOutput (Join-Path $runDir "godot.stdout.log") `
        -RedirectStandardError (Join-Path $runDir "godot.stderr.log") -PassThru
    if (-not (Wait-Port 5555 60)) { throw "Kernel command port 5555 did not become ready" }
    if (-not (Wait-Port 5556 60)) { throw "Kernel event port 5556 did not become ready" }
    if (-not (Wait-HttpReady "http://127.0.0.1:7878/health" 60)) { throw "VitAgent did not become ready" }
    if (-not (Wait-Port 8787 60)) { throw "VspHub port 8787 did not become ready" }

    if ($Phase -in @("Matrix", "All")) {
        & python (Join-Path $RepoRoot "scripts\limiter_matrix_smoke.py") `
            --output-dir (Join-Path $runDir "matrix") --timeout-sec $TimeoutSeconds
        if ($LASTEXITCODE -ne 0) { throw "Limiter training/regression matrix failed" }
        & python (Join-Path $RepoRoot "scripts\limiter_known_apply_smoke.py") `
            --output-dir (Join-Path $runDir "known_apply") --timeout-sec $TimeoutSeconds
        if ($LASTEXITCODE -ne 0) { throw "Limiter known-sample typed apply/restore failed" }
    }
    if ($Phase -in @("Blind", "All")) {
        $freeze = Join-Path $runDir "blind_freeze.json"
        & python (Join-Path $RepoRoot "scripts\limiter_blind_smoke.py") freeze `
            --output $freeze --timeout-sec $TimeoutSeconds
        if ($LASTEXITCODE -ne 0) { throw "Limiter blind freeze failed" }
        & python (Join-Path $RepoRoot "scripts\limiter_blind_smoke.py") execute `
            --freeze $freeze --output-dir (Join-Path $runDir "blind") --timeout-sec $TimeoutSeconds
        if ($LASTEXITCODE -ne 0) { throw "Limiter sealed blind execution failed" }
    }
}
finally {
    foreach ($key in $overrides.Keys) { [Environment]::SetEnvironmentVariable($key, $prior[$key], "Process") }
    if ($null -ne $godot -and -not $KeepRuntime) {
        try { $null = $godot.CloseMainWindow() } catch { }
        try { if (-not $godot.WaitForExit(12000)) { Stop-Process -Id $godot.Id -Force -ErrorAction SilentlyContinue } } catch { }
        Start-Sleep -Milliseconds 800
        foreach ($port in @(5555, 5556, 7878, 8787)) {
            $listener = Get-NetTCPConnection -State Listen -LocalPort $port -ErrorAction SilentlyContinue | Select-Object -First 1
            if ($null -eq $listener) { continue }
            $process = Get-CimInstance Win32_Process -Filter ("ProcessId=" + $listener.OwningProcess) -ErrorAction SilentlyContinue
            if ($null -ne $process -and $process.ExecutablePath -like ($RepoRoot + "*")) {
                Stop-Process -Id $listener.OwningProcess -Force -ErrorAction SilentlyContinue
            }
        }
    }
    Write-Host ("artifacts: " + $runDir)
}
