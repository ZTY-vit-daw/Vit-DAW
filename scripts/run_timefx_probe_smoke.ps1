#Requires -Version 5.1
<#
M2-TIMEFX-2 reverb/delay paired probe + FXM differential smoke.

Starts the real stack the same way run_com_product_path_smoke.ps1 does
(Godot-launched kernel + agent + hub), but with VIT_DAW_DEV_ROOT pointed at
the worktree under test so the Godot dev autostart launches the worktree
kernel/agent builds. Then scripts/timefx_probe_smoke.py drives:

  A. probe track + probe audio + one family rack node (kernel ZMQ),
  B. agent semantic apply -> automatic FXM differential assertions,
  C. direct timefx_dual_tap_probe roundtrip + artifact integrity gates.

Exit 0 = PASS. Artifacts under artifacts\timefx_probe\<stamp>.
#>
[CmdletBinding()]
param(
    [string]$RepoRoot = "D:\Vit_DAW_wt_m2_timefx2",
    [string]$GodotExe = "D:\Godot\Godot_v4.6.1-stable_win64.exe",
    [string]$GodotProject = "D:\Godot\project\vit-daw-frontend",
    [ValidateSet("reverb", "delay")]
    [string]$Family = "reverb",
    [string]$PluginPath = "",
    [string]$Material = "D:\Vit_DAW\PluginProbe\assets\probe_audio_v1\transient_burst_48k_10s.wav",
    [int]$TimeoutSeconds = 300,
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
if ([string]::IsNullOrWhiteSpace($PluginPath)) {
    $PluginPath = if ($Family -eq "reverb") { "C:\Program Files\Common Files\VST3\FabFilter\FabFilter Pro-R.vst3" }
                  else { "C:\Program Files\Common Files\VST3\FabFilter\FabFilter Timeless 3.vst3" }
}
$PluginPath = (Resolve-Path -LiteralPath $PluginPath).Path
$Material = (Resolve-Path -LiteralPath $Material).Path
if (-not (Test-Path -LiteralPath (Join-Path $RepoRoot "VitApp\build\VitApp_artefacts\Release\VitApp.exe") -PathType Leaf)) {
    Fail "Worktree kernel build missing: run cmake --build VitApp\build --config Release first"
}
foreach ($port in @(5555, 5556, 7878, 8787)) {
    if (Get-NetTCPConnection -State Listen -LocalPort $port -ErrorAction SilentlyContinue) {
        Fail "Port $port is already in use; refusing to disturb an existing product session"
    }
}

$stamp = Get-Date -Format "yyyyMMdd_HHmmss"
$runDir = Join-Path $RepoRoot ("artifacts\timefx_probe\" + $stamp)
# Card 144 (FIX-STACK-DRAFT-ROOT-ISOLATION-1): redirect the agent draft root
# away from the user's real %LOCALAPPDATA%\Vit\ProjectHistory\drafts before the
# stack start; external VIT_HISTORY_DRAFT_ROOT wins. Default drafts live under
# this leg's run dir.
. (Join-Path $RepoRoot "scripts\vit_stack_draft_isolation.ps1")
$draftInfo = Enable-VitDraftRootIsolation -Label "run_timefx_probe_smoke" -RunDir $runDir
Assert-VitDraftRootIsolation -Info $draftInfo
$null = New-Item -ItemType Directory -Force -Path $runDir
$projectCopy = Join-Path $runDir "timefx_project.xml"
$sourceProject = Join-Path $RepoRoot "VitApp\Workspace\default_project.xml"
Copy-Item -LiteralPath $sourceProject -Destination $projectCopy
Rebase-AudioClipSources $sourceProject $projectCopy
$report = Join-Path $runDir ("timefx_{0}_report.json" -f $Family)
$godotStdout = Join-Path $runDir "godot.stdout.log"
$godotStderr = Join-Path $runDir "godot.stderr.log"
$priorEnv = @{}
$overrides = @{
    VIT_DAW_DEV_ROOT = $RepoRoot
    VIT_DEV_ROOT = $RepoRoot
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

    & python (Join-Path $RepoRoot "scripts\timefx_probe_smoke.py") `
        --repo-root $RepoRoot --family $Family --plugin-path $PluginPath `
        --material $Material --workspace (Join-Path $RepoRoot "VitApp\Workspace") `
        --timeout-sec $TimeoutSeconds --output $report
    if ($LASTEXITCODE -ne 0) { Fail "timefx probe smoke failed; inspect $report" }
    Write-Host ("timefx probe smoke PASS: " + $report)
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
