#Requires -Version 5.1
[CmdletBinding()]
param(
    [string]$RepoRoot = "D:\Vit_DAW",
    [string]$SeedProject = "",
    [string]$AgentHttp = "http://127.0.0.1:7878",
    [string]$GodotProjectRoot = "D:\Godot\project\vit-daw-frontend",
    [string]$GodotExe = "D:\Godot\Godot_v4.6.1-stable_win64_console.exe",
    [string]$KernelExe = "",
    [int]$TimeoutSeconds = 600,
    [switch]$SkipBuild,
    [string]$TrackId = "1012",
    [string]$PluginId = "1048",
    [string]$ParamId = "827156039",
    [double]$NormalizedValue = 0.5,
    [double]$TempoBpm = 123.0
)

# B15 real-stack smoke: true manual-save semantics.
#
# Boots the real Kernel + Godot + Go agent lifecycle once per phase and drives
# the public Agent HTTP surface. Phases:
#   setup       -> stage the seed project, explicit manual save (the save point)
#   govern      -> governed VSP parameter write + a callback-bearing governance
#                  mutation; the working copy must not move, the repository must
#                  advance, and the repository must be able to give the state back
#   reopen      -> new lifecycle; file hash still == save point, unsaved write gone
#   save_verify -> explicit manual save writes the working copy; reopen brings it back
#
# The governed mutation is deterministic on purpose: the card's real-stack round
# is about the kernel write path, and the agent wraps every kernel command in a
# VSP envelope (agent/internal/kernel/vsp.go), which is exactly the discriminator
# the B15 gate keys on.

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$RepoRoot = (Resolve-Path -LiteralPath $RepoRoot).Path
if ([string]::IsNullOrWhiteSpace($SeedProject)) {
    $SeedProject = Join-Path $RepoRoot "artifacts\mantest3\20260911\project\spv1_p01.vit"
}
$SeedProject = (Resolve-Path -LiteralPath $SeedProject).Path
if ([string]::IsNullOrWhiteSpace($KernelExe)) {
    $KernelExe = Join-Path $RepoRoot "VitApp\build\VitApp_artefacts\Release\VitApp.exe"
}
$driver = Join-Path $RepoRoot "scripts\b15_manual_save_smoke.py"
if (-not (Test-Path -LiteralPath $driver -PathType Leaf)) {
    throw ("missing B15 smoke driver: " + $driver)
}

$stamp = Get-Date -Format "yyyyMMdd_HHmmss"
$runRoot = Join-Path $RepoRoot ("artifacts\b15_manual_save\" + $stamp)
$workdir = Join-Path $runRoot "project"
$phaseRoots = [ordered]@{
    setup       = Join-Path $runRoot "phase_setup"
    govern      = Join-Path $runRoot "phase_govern"
    reopen      = Join-Path $runRoot "phase_reopen"
    save_verify = Join-Path $runRoot "phase_save_verify"
}
New-Item -ItemType Directory -Path $runRoot -Force | Out-Null
foreach ($phaseRoot in $phaseRoots.Values) {
    New-Item -ItemType Directory -Path $phaseRoot -Force | Out-Null
}

$summary = [ordered]@{
    schema_version = "vit.b15_manual_save_orchestrator.v1"
    status = "running"
    created_at = (Get-Date).ToString("o")
    run_root = $runRoot
    workdir = $workdir
    seed_project = $SeedProject
    kernel_exe = $KernelExe
    kernel_exe_sha256 = ""
    kernel_exe_built_at = ""
    project_path = ""
    phases = [ordered]@{}
    verdict = $null
}

function Invoke-StackBoot {
    param([string]$Label)
    Write-Host ("== B15 stack boot (" + $Label + ")") -ForegroundColor Cyan
    $bootArgs = @(
        "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", (Join-Path $RepoRoot "scripts\dev_agent_smoke.ps1"),
        "-RepoRoot", $RepoRoot, "-AgentHttp", $AgentHttp, "-RestartAgent", "-StartKernel", "-KernelExe", $KernelExe, "-StartUI",
        "-GodotProjectRoot", $GodotProjectRoot, "-GodotExe", $GodotExe,
        "-NoChatSmoke", "-NoStripSilenceSmoke", "-WaitSeconds", ([string]$TimeoutSeconds)
    )
    $bootArgs += "-SkipBuild"
    & powershell @bootArgs
    if ($LASTEXITCODE -ne 0) {
        throw ("real-stack startup failed for phase " + $Label + " with exit code " + $LASTEXITCODE)
    }
    $kernelListener = Get-NetTCPConnection -LocalPort 5555 -State Listen -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($null -eq $kernelListener) {
        throw ("real VitApp Kernel is not listening on port 5555 for phase " + $Label)
    }
    $agentListener = Get-NetTCPConnection -LocalPort 7878 -State Listen -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($null -eq $agentListener) {
        throw ("real Go agent is not listening on port 7878 for phase " + $Label)
    }
    return [pscustomobject]@{
        kernel_pid = $kernelListener.OwningProcess
        agent_pid = $agentListener.OwningProcess
    }
}

function Stop-OwnedStack {
    $owned = @()
    foreach ($port in @(5555, 5556, 7878, 8787)) {
        $listener = Get-NetTCPConnection -LocalPort $port -State Listen -ErrorAction SilentlyContinue | Select-Object -First 1
        if ($null -ne $listener) {
            $owned += $listener.OwningProcess
        }
    }
    foreach ($pidValue in ($owned | Sort-Object -Unique)) {
        Stop-Process -Id $pidValue -Force -ErrorAction SilentlyContinue
    }
    Start-Sleep -Milliseconds 900
}

function Resolve-CanonicalProject {
    if (-not [string]::IsNullOrWhiteSpace([string]$summary.project_path)) {
        return [string]$summary.project_path
    }
    if (Test-Path -LiteralPath $workdir -PathType Container) {
        $candidate = Get-ChildItem -LiteralPath $workdir -File -Filter "*.vit" | Select-Object -First 1
        if ($null -ne $candidate) { return $candidate.FullName }
    }
    throw ("B15 cannot resolve the canonical project under " + $workdir)
}

function Invoke-Phase {
    param(
        [string]$Phase,
        [hashtable]$Extra
    )
    $artifactDir = $phaseRoots[$Phase]
    $pythonArgs = @(
        $driver,
        "--phase", $Phase,
        "--agent-http", $AgentHttp,
        "--project-workdir", $workdir,
        "--artifact-dir", $artifactDir,
        "--timeout-sec", ([string][Math]::Max(600, $TimeoutSeconds)),
        "--track-id", $TrackId,
        "--plugin-id", $PluginId,
        "--param-id", $ParamId,
        "--normalized-value", ([string]$NormalizedValue),
        "--tempo-bpm", ([string]$TempoBpm)
    )
    if ($Phase -eq "setup") {
        $pythonArgs += @("--seed-project", $SeedProject)
    }
    else {
        $pythonArgs += @("--project-path", (Resolve-CanonicalProject))
    }
    foreach ($key in $Extra.Keys) {
        $value = [string]$Extra[$key]
        if ([string]::IsNullOrWhiteSpace($value)) {
            continue
        }
        $pythonArgs += @($key, $value)
    }
    $stdoutPath = Join-Path $artifactDir "stdout.log"
    $previousErrorActionPreference = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    & python @pythonArgs 2>&1 | Tee-Object -FilePath $stdoutPath | Out-Null
    $exitCode = $LASTEXITCODE
    $ErrorActionPreference = $previousErrorActionPreference
    $summaryPath = Join-Path $artifactDir "summary.json"
    if ($exitCode -ne 0) {
        if (Test-Path -LiteralPath $summaryPath -PathType Leaf) {
            $summary.phases[$Phase] = Get-Content -LiteralPath $summaryPath -Raw -Encoding UTF8 | ConvertFrom-Json
        }
        throw ("B15 phase " + $Phase + " failed with exit code " + $exitCode + "; output=" + $stdoutPath)
    }
    if (-not (Test-Path -LiteralPath $summaryPath -PathType Leaf)) {
        throw ("B15 phase " + $Phase + " produced no summary.json")
    }
    $phaseSummary = Get-Content -LiteralPath $summaryPath -Raw -Encoding UTF8 | ConvertFrom-Json
    if ([string]$phaseSummary.status -ne "passed") {
        throw ("B15 phase " + $Phase + " summary is not passed: " + ($phaseSummary | ConvertTo-Json -Depth 24 -Compress))
    }
    $summary.phases[$Phase] = $phaseSummary
    return $phaseSummary
}

$kernelLogDir = Join-Path $RepoRoot "VitApp\Workspace\Logs"
$latestLog = $null

try {
    if (-not $SkipBuild) {
        & cmake --build (Join-Path $RepoRoot "VitApp\build") --config Release --target VitApp --parallel 4
        if ($LASTEXITCODE -ne 0) {
            throw "current VitApp Release build failed"
        }
    }
    $KernelExe = (Resolve-Path -LiteralPath $KernelExe).Path
    $summary.kernel_exe = $KernelExe
    $summary.kernel_exe_sha256 = (Get-FileHash -LiteralPath $KernelExe -Algorithm SHA256).Hash
    $summary.kernel_exe_built_at = (Get-Item -LiteralPath $KernelExe).LastWriteTime.ToString("o")

    # --- phase setup -------------------------------------------------------
    $boot = Invoke-StackBoot -Label "setup"
    $summary.phases["setup_boot"] = $boot
    $setup = Invoke-Phase -Phase "setup" -Extra @{}
    $summary.project_path = [string]$setup.project_path
    $savePoint = [string]$setup.save_point_sha256
    Start-Sleep -Milliseconds 500
    Stop-OwnedStack

    # --- phase govern ------------------------------------------------------
    $boot = Invoke-StackBoot -Label "govern"
    $summary.phases["govern_boot"] = $boot
    $latestLog = Get-ChildItem -LiteralPath $kernelLogDir -Filter "VitHeadlessServer*.log" -File |
        Sort-Object LastWriteTime -Descending | Select-Object -First 1
    $govern = Invoke-Phase -Phase "govern" -Extra @{
        "--expected-sha256" = $savePoint
        "--kernel-log" = [string]$latestLog.FullName
    }
    $governedHead = [string]$govern.version_repository.head_after
    $mutatedValue = [string]$govern.governed_mutation.revision_after
    Start-Sleep -Milliseconds 500
    Stop-OwnedStack

    # --- phase reopen ------------------------------------------------------
    $boot = Invoke-StackBoot -Label "reopen"
    $summary.phases["reopen_boot"] = $boot
    $reopen = Invoke-Phase -Phase "reopen" -Extra @{
        "--expected-sha256" = $savePoint
        "--governed-head" = $governedHead
        "--expected-mutated-value" = $mutatedValue
        "--track-id" = $TrackId
        "--plugin-id" = $PluginId
    }
    Start-Sleep -Milliseconds 500
    Stop-OwnedStack

    # --- phase save_verify -------------------------------------------------
    $boot = Invoke-StackBoot -Label "save_verify"
    $summary.phases["save_verify_boot"] = $boot
    $saved = Invoke-Phase -Phase "save_verify" -Extra @{
        "--expected-sha256" = $savePoint
        "--expected-mutated-value" = $mutatedValue
    }
    Start-Sleep -Milliseconds 500
    Stop-OwnedStack

    $summary.status = "passed"
    $summary.verdict = [ordered]@{
        governed_change_reached_live_edit = [bool]$govern.governed_mutation.reached_live_edit
        governed_mutation_left_working_copy_untouched = [bool]$govern.working_copy.unchanged
        version_repository_advanced = [bool]$govern.version_repository.advanced
        kernel_reported_deferred_working_copy_persist = [bool]($govern.kernel_deferral_log.hit_count -gt 0)
        repository_checkout_restored_recorded_state = [bool]$saved.repository_recovery.checkout_restored_recorded_state
        reopen_returned_to_save_point = [bool]$reopen.working_copy.unchanged
        reopen_dropped_unsaved_governed_change = [bool]$reopen.unsaved_governed_change_absent
        manual_save_wrote_working_copy = [bool]$saved.manual_save.wrote_working_copy
        manual_save_persisted_change_across_reopen = [bool]$saved.manual_save_persisted_change
    }
}
catch {
    $summary.status = "failed"
    $summary.error = $_.Exception.Message
    throw
}
finally {
    try { Stop-OwnedStack } catch { }
    $agentLog = Join-Path $RepoRoot "VitApp\Workspace\Logs\agent_last.log"
    if (Test-Path -LiteralPath $agentLog -PathType Leaf) {
        Copy-Item -LiteralPath $agentLog -Destination (Join-Path $runRoot "agent_last.log") -Force
    }
    if ($null -ne $latestLog -and (Test-Path -LiteralPath $latestLog.FullName -PathType Leaf)) {
        Copy-Item -LiteralPath $latestLog.FullName -Destination (Join-Path $runRoot "kernel_last.log") -Force
    }
    $summary.completed_at = (Get-Date).ToString("o")
    $summary | ConvertTo-Json -Depth 40 | Set-Content -LiteralPath (Join-Path $runRoot "summary.json") -Encoding UTF8
}

if ($summary.status -ne "passed") {
    throw ("B15 manual-save smoke did not pass; see " + (Join-Path $runRoot "summary.json"))
}

Write-Host "ok: B15 true-manual-save real-stack round passed" -ForegroundColor Green
Write-Host ("artifact: " + (Join-Path $runRoot "summary.json"))
