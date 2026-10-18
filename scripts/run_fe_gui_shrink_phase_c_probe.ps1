[CmdletBinding()]
param(
    [string]$RepoRoot = "",
    [string]$GodotProjectRoot = "D:\Godot\project\vit-daw-frontend-fe-gui-c",
    [string]$GodotExe = "D:\Godot\Godot_v4.6.1-stable_win64_console.exe",
    [string]$BaseCommit = "924318dea1f55f5c9591d573b174a1a2307398e6",
    [string]$ArtifactRoot = ""
)

# FE-GUI-C-FLOATING (card 92) Phase C headless probe runner
# (run_fe_gui_shrink_phase_b_probe.ps1 pattern: exit 0 = PASS).
# Gates:
#   1) Godot import cache warm-up for fresh worktrees (headless --import; class cache needed
#      before the probe can resolve global class names).
#   2) Red-line git gate: frontend repo BaseCommit..worktree diff must not touch the card
#      red-line 9 files (telemetry writer chain / left library / rack+track context menus /
#      plugin drop dual landing surfaces / transport bar / track header meters).
#   3) Phase C probe: tools/probe_fe_gui_shrink_phase_c.gd (static contract + assembly/injection
#      pins for spec assertion 5 second half: drawer-chip click -> floating card, floating card
#      warn/promote buttons -> real Godot Window instance, reattach/close semantics, F2 panel
#      float button, bottom status bar terminal state, window_manager three live call sites
#      retained as anchor), exit 0 required.
#   4) Regression battery: Phase B probe (119 nails) + Phase A probe (69 nails) + vitnote six
#      probes + FE-RACK plugin-search probe, all exit 0.
# Artifacts: raw logs under coord/runs/FE-GUI-C-FLOATING/<timestamp>/ by default (RepoRoot
# derived from this script location so a dedicated probe worktree stays self-contained).
# NOTE: keep this file ASCII-only -- PowerShell 5.1 reads BOM-less ps1 as ANSI and CJK
# comments can corrupt parsing.

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

if ([string]::IsNullOrEmpty($RepoRoot)) {
    $RepoRoot = Split-Path -Parent $PSScriptRoot
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

if ([string]::IsNullOrEmpty($ArtifactRoot)) {
    $stamp = Get-Date -Format "yyyyMMdd_HHmmss"
    $ArtifactRoot = Join-Path $RepoRoot ("coord\runs\FE-GUI-C-FLOATING\" + $stamp)
}
New-Item -ItemType Directory -Force -Path $ArtifactRoot | Out-Null

Write-Step "Godot import cache warm-up ($GodotProjectRoot)"
$importLog = Join-Path $ArtifactRoot "godot_import.log"
$importCmd = '"' + $GodotExe + '" --headless --path "' + $GodotProjectRoot + '" --import'
cmd /c ($importCmd + ' > "' + $importLog + '" 2>&1')
$importExit = $LASTEXITCODE
if ($importExit -ne 0) {
    Fail ("Godot import failed with exit code " + $importExit + " (log: " + $importLog + ")")
}
if (-not (Test-Path -LiteralPath (Join-Path $GodotProjectRoot ".godot\global_script_class_cache.cfg"))) {
    Fail ("Godot import did not produce global_script_class_cache.cfg (log: " + $importLog + ")")
}
Write-Ok "Godot import cache ready"

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

$probeScripts = @(
    "tools/probe_fe_gui_shrink_phase_c.gd",
    "tools/probe_fe_gui_shrink_phase_b.gd",
    "tools/probe_fe_gui_shrink_phase_a.gd",
    "tools/probe_vitnote_circle_state.gd",
    "tools/probe_vitnote_container.gd",
    "tools/probe_vitnote_face_resolve.gd",
    "tools/probe_vitnote_input.gd",
    "tools/probe_vitnote_notestream.gd",
    "tools/probe_vitnote_panel_summary.gd",
    "tools/probe_rack_plugin_search_popup.gd"
)

foreach ($probeScript in $probeScripts) {
    $probeName = [IO.Path]::GetFileNameWithoutExtension($probeScript)
    Write-Step ("Godot headless probe: " + $probeName)
    $probeLog = Join-Path $ArtifactRoot ($probeName + ".log")
    # cmd /c + file redirect (Phase A/B precedent): Godot stderr lines become NativeCommandError
    # under ErrorActionPreference=Stop, so avoid the PS 2>&1 pipeline.
    $probeCmd = '"' + $GodotExe + '" --headless --path "' + $GodotProjectRoot + '" -s ' + $probeScript
    cmd /c ($probeCmd + ' > "' + $probeLog + '" 2>&1')
    $probeExit = $LASTEXITCODE
    Get-Content -LiteralPath $probeLog | Select-Object -Last 6
    if ($probeExit -ne 0) {
        Fail ($probeName + " failed with exit code " + $probeExit + " (log: " + $probeLog + ")")
    }
    # FE-GUI probes carry an explicit "PROBE PASS" verdict line; vitnote/rack probes use
    # varying summary formats -- for those the exit code is the gate (with failed=0 sanity).
    if ($probeName -like "probe_fe_gui_shrink_*") {
        $verdict = Select-String -LiteralPath $probeLog -Pattern "PROBE PASS"
        if (-not $verdict) {
            Fail ($probeName + " log missing PROBE PASS verdict (log: " + $probeLog + ")")
        }
    } else {
        $failedLine = Select-String -LiteralPath $probeLog -Pattern "failed=([1-9][0-9]*)"
        if ($failedLine) {
            Fail ($probeName + " reported failed>0 (log: " + $probeLog + ")")
        }
    }
    Write-Ok ($probeName + " exit 0")
}

Write-Ok ("FE-GUI-C-FLOATING Phase C probe battery passed; artifacts: " + $ArtifactRoot)
exit 0
