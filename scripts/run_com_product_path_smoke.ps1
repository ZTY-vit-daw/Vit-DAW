#Requires -Version 5.1
[CmdletBinding()]
param(
    [string]$RepoRoot = "D:\Vit_DAW",
    [string]$GodotExe = "D:\Godot\Godot_v4.6.1-stable_win64.exe",
    [string]$GodotProject = "D:\Godot\project\vit-daw-frontend",
    [string]$TrackId = "1007",
    [string]$PluginId = "1021",
    [int]$TimeoutSeconds = 180,
    [switch]$KeepGodot
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
function Fail([string]$Message) { throw $Message }
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
function Rebase-AudioClipSources([string]$SourceProject, [string]$ProjectCopy) {
    $sourceDirectory = Split-Path -Parent $SourceProject
    $content = [IO.File]::ReadAllText($ProjectCopy)
    $pattern = '(<AUDIOCLIP\b[^>]*?\bsource=")([^"]+)(")'
    $rewritten = [regex]::Replace($content, $pattern, {
        param($match)
        $source = $match.Groups[2].Value
        if ([string]::IsNullOrWhiteSpace($source) -or [IO.Path]::IsPathRooted($source)) { return $match.Value }
        $resolved = [IO.Path]::GetFullPath((Join-Path $sourceDirectory $source))
        if (-not (Test-Path -LiteralPath $resolved -PathType Leaf)) {
            Fail "Copied project audio source does not exist: $resolved"
        }
        return $match.Groups[1].Value + $resolved + $match.Groups[3].Value
    }, [Text.RegularExpressions.RegexOptions]::Singleline)
    [IO.File]::WriteAllText($ProjectCopy, $rewritten, [Text.UTF8Encoding]::new($false))
}

$RepoRoot = (Resolve-Path -LiteralPath $RepoRoot).Path
$GodotExe = (Resolve-Path -LiteralPath $GodotExe).Path
$GodotProject = (Resolve-Path -LiteralPath $GodotProject).Path
foreach ($port in @(5555, 5556, 7878, 8787)) {
    if (Get-NetTCPConnection -State Listen -LocalPort $port -ErrorAction SilentlyContinue) {
        Fail "Port $port is already in use; refusing to disturb an existing product session"
    }
}

$stamp = Get-Date -Format "yyyyMMdd_HHmmss"
$runDir = Join-Path $RepoRoot ("artifacts\com_product_path\" + $stamp)
$null = New-Item -ItemType Directory -Force -Path $runDir
$projectCopy = Join-Path $runDir "com5_project.xml"
$sourceProject = Join-Path $RepoRoot "VitApp\Workspace\default_project.xml"
Copy-Item -LiteralPath $sourceProject -Destination $projectCopy
Rebase-AudioClipSources $sourceProject $projectCopy
$com4Report = Join-Path $runDir "com4_prerequisite_report.json"
$com5Report = Join-Path $runDir "com5_product_path_report.json"
$godotStdout = Join-Path $runDir "godot.stdout.log"
$godotStderr = Join-Path $runDir "godot.stderr.log"
$priorEnv = @{}
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
        if ($LASTEXITCODE -ne 0) { Fail "VitAgent build failed" }
    } finally { Pop-Location }
    foreach ($key in $overrides.Keys) {
        $priorEnv[$key] = [Environment]::GetEnvironmentVariable($key, "Process")
        [Environment]::SetEnvironmentVariable($key, [string]$overrides[$key], "Process")
    }
    $godot = Start-Process -FilePath $GodotExe -ArgumentList @("--path", $GodotProject) `
        -WorkingDirectory $GodotProject -WindowStyle Hidden -RedirectStandardOutput $godotStdout `
        -RedirectStandardError $godotStderr -PassThru
    if (-not (Wait-Port 5555 60)) { Fail "Godot-launched kernel command port 5555 did not become ready" }
    if (-not (Wait-Port 5556 60)) { Fail "Godot-launched kernel event port 5556 did not become ready" }
    if (-not (Wait-HttpReady "http://127.0.0.1:7878/health" 60)) { Fail "Godot-launched VitAgent did not become ready" }
    if (-not (Wait-Port 8787 60)) { Fail "Godot-launched VspHub port 8787 did not become ready" }

    & python (Join-Path $RepoRoot "scripts\compressor_change_delta_smoke.py") `
        --repo-root $RepoRoot --track-id $TrackId --plugin-id $PluginId `
        --material (Join-Path $RepoRoot "PluginProbe\assets\probe_audio_v1\transient_burst_48k_10s.wav") `
        --workspace (Join-Path $RepoRoot "VitApp\Workspace") --timeout-sec $TimeoutSeconds --output $com4Report
    if ($LASTEXITCODE -ne 0) { Fail "COM-4 prerequisite capture failed; inspect $com4Report" }

    & python (Join-Path $RepoRoot "scripts\com_product_path_smoke.py") `
        --com4-report $com4Report --output $com5Report --timeout-sec $TimeoutSeconds
    if ($LASTEXITCODE -ne 0) { Fail "COM-5 product path smoke failed; inspect $com5Report" }
}
finally {
    foreach ($key in $overrides.Keys) { [Environment]::SetEnvironmentVariable($key, $priorEnv[$key], "Process") }
    if ($null -ne $godot -and -not $KeepGodot) {
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
