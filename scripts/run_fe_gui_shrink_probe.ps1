[CmdletBinding()]
param(
    [string]$RepoRoot = "D:\Vit_DAW",
    [string]$GodotProjectRoot = "D:\Godot\project\vit-daw-frontend",
    [string]$GodotExe = "D:\Godot\Godot_v4.6.1-stable_win64_console.exe",
    [string]$BaseCommit = "924318dea1f55f5c9591d573b174a1a2307398e6",
    [string]$ArtifactRoot = ""
)

# FE-GUI-SHRINK-IMPL-1 Phase A headless probe runner (dev_agent_smoke.ps1 pattern: exit 0 = PASS).
# Two gates:
#   1) Red-line git gate: frontend repo BaseCommit..worktree diff must not touch card red-line
#      files (telemetry writer chain / left library / rack+track context menus / plugin drop
#      dual landing surfaces).
#   2) Godot headless probe: tools/probe_fe_gui_shrink_phase_a.gd 69 checks (static retirement
#      contract + full-tree assembly smoke + Master virtual-path / mixboard_ id red-line pins),
#      exit 0 required.
# Artifacts: probe raw log under coord/runs/FE-GUI-SHRINK-IMPL-1/<timestamp>/.
# NOTE: keep this file ASCII-only -- PowerShell 5.1 reads BOM-less ps1 as ANSI and CJK comments
# can corrupt parsing.

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Write-Step {
    param([string]$Message)
    Write-Host ""
    Write-Host ("== " + $Message) -ForegroundColor Cyan
}

function Write-Ok {
    param([string]$Message)
    Write-Host ("ok: " + $Message) -ForegroundColor Green
}

function Fail {
    param([string]$Message)
    throw $Message
}

$RepoRoot = (Resolve-Path -LiteralPath $RepoRoot).Path
$GodotProjectRoot = (Resolve-Path -LiteralPath $GodotProjectRoot).Path

if (-not (Test-Path -LiteralPath $GodotExe)) {
    Fail ("Godot exe not found: " + $GodotExe)
}

$redlineFiles = @(
    "app/kernel/autoloads/telemetry_manager.gd",
    "app/kernel/clients/mix_client.gd",
    "app/shell/LLM_Chat_Controller.gd",
    "app/kernel/autoloads/vsp_asset_adapter.gd",
    "app/browser/left_library_dock.gd",
    "app/rack/Vit_Graph_Rack.gd",
    "app/shell/Top_Transport_Bar.tscn",
    "app/shell/top_transport_bar.gd",
    "app/tracks/track_scene/vit_track_header_meters.gd"
)

Write-Step "Red-line git gate (frontend repo $BaseCommit..worktree)"
Push-Location $GodotProjectRoot
try {
    $changed = @(& git diff --name-only $BaseCommit)
    if ($LASTEXITCODE -ne 0) {
        Fail ("git diff failed with exit code " + $LASTEXITCODE)
    }
    foreach ($file in $redlineFiles) {
        if ($changed -contains $file) {
            Fail ("red-line file touched by diff: " + $file)
        }
        $status = [string](@(& git status --porcelain -- $file) -join "")
        if ($status.StartsWith("??") -or $status.StartsWith(" M") -or $status.StartsWith("M")) {
            Fail ("red-line file present as untracked/modified in worktree: " + $file)
        }
    }
    Write-Ok ("red-line files untouched (" + $redlineFiles.Count + " checked; changed files vs base: " + $changed.Count + ")")
}
finally {
    Pop-Location
}

if ([string]::IsNullOrEmpty($ArtifactRoot)) {
    $stamp = Get-Date -Format "yyyyMMdd_HHmmss"
    $ArtifactRoot = Join-Path $RepoRoot ("coord\runs\FE-GUI-SHRINK-IMPL-1\" + $stamp)
}
New-Item -ItemType Directory -Force -Path $ArtifactRoot | Out-Null

Write-Step "Godot headless probe (69 checks)"
$probeLog = Join-Path $ArtifactRoot "probe_fe_gui_shrink_phase_a.log"
# cmd /c + file redirect (run_l2_realtime_observation_smoke.ps1 precedent): Godot stderr lines
# become NativeCommandError under ErrorActionPreference=Stop, so avoid the PS 2>&1 pipeline.
$probeCmd = '"' + $GodotExe + '" --headless --path "' + $GodotProjectRoot + '" -s tools/probe_fe_gui_shrink_phase_a.gd'
cmd /c ($probeCmd + ' > "' + $probeLog + '" 2>&1')
$probeExit = $LASTEXITCODE
Get-Content -LiteralPath $probeLog | Select-Object -Last 12
if ($probeExit -ne 0) {
    Fail ("headless probe failed with exit code " + $probeExit + " (log: " + $probeLog + ")")
}
$verdict = Select-String -LiteralPath $probeLog -Pattern "PROBE PASS"
if (-not $verdict) {
    Fail ("probe log missing PROBE PASS verdict (log: " + $probeLog + ")")
}

Write-Ok ("FE-GUI-SHRINK Phase A probe passed; artifacts: " + $ArtifactRoot)
exit 0
