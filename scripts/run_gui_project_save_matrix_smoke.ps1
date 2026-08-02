[CmdletBinding()]
param(
    [string]$RepoRoot = "D:\Vit_DAW",
    [string]$GodotProjectRoot = "D:\Godot\project\vit-daw-frontend",
    [string]$GodotExe = "D:\Godot\Godot_v4.6.1-stable_win64_console.exe",
    [string]$SourceProject = "D:\Vit_DAW\VitApp\Workspace\Artifacts\smoke\gui_project_save_matrix_20260731_225237\Regular Save.vit",
    [int]$TimeoutSeconds = 900
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$RepoRoot = (Resolve-Path -LiteralPath $RepoRoot).Path
$GodotProjectRoot = (Resolve-Path -LiteralPath $GodotProjectRoot).Path
$GodotExe = (Resolve-Path -LiteralPath $GodotExe).Path
$SourceProject = (Resolve-Path -LiteralPath $SourceProject).Path
$stamp = Get-Date -Format "yyyyMMdd_HHmmss"
$runRoot = Join-Path $RepoRoot ("VitApp\Workspace\Artifacts\smoke\gui_project_save_matrix_" + $stamp)
New-Item -ItemType Directory -Path $runRoot -Force | Out-Null

function Get-LifecycleProcesses {
    return @(Get-CimInstance Win32_Process | Where-Object {
        $_.Name -in @(
            "Godot_v4.6.1-stable_win64.exe",
            "Godot_v4.6.1-stable_win64_console.exe",
            "VitApp.exe",
            "VspHub.exe",
            "VitAgent.exe"
        )
    })
}

$baselinePids = @{}
foreach ($process in Get-LifecycleProcesses) {
    $baselinePids[[int]$process.ProcessId] = $true
}

function Stop-OwnedLifecycleProcesses {
    foreach ($process in Get-LifecycleProcesses) {
        $pidValue = [int]$process.ProcessId
        if ($baselinePids.ContainsKey($pidValue)) {
            continue
        }
        Stop-Process -Id $pidValue -Force -ErrorAction SilentlyContinue
    }
}

function Invoke-SavePhase {
    param([string]$Phase)

    $stdoutPath = Join-Path $runRoot ("godot_" + $Phase + ".stdout.log")
    $stderrPath = Join-Path $runRoot ("godot_" + $Phase + ".stderr.log")
    $env:VIT_DAW_DEV_ROOT = $RepoRoot
    $env:VIT_SKIP_DEV_AUTOSTART = "0"
    $env:VIT_GUI_SAVE_MATRIX_PHASE = $Phase
    $env:VIT_GUI_SAVE_MATRIX_ROOT = $runRoot
    $env:VIT_GUI_SAVE_MATRIX_SOURCE = $SourceProject
    $process = Start-Process `
        -FilePath $GodotExe `
        -ArgumentList @(
            "--path", $GodotProjectRoot,
            "--script", "res://tools/diagnostics/gui_project_save_matrix_probe.gd"
        ) `
        -WorkingDirectory $GodotProjectRoot `
        -WindowStyle Hidden `
        -RedirectStandardOutput $stdoutPath `
        -RedirectStandardError $stderrPath `
        -PassThru
    if (-not $process.WaitForExit($TimeoutSeconds * 1000)) {
        Stop-Process -Id $process.Id -Force -ErrorAction SilentlyContinue
        throw "GUI save phase timed out: $Phase"
    }
    $process.Refresh()
    $exitCode = $process.ExitCode
    if ($null -ne $exitCode -and [int]$exitCode -ne 0) {
        $stderr = Get-Content -LiteralPath $stderrPath -Raw -ErrorAction SilentlyContinue
        $stdout = Get-Content -LiteralPath $stdoutPath -Raw -ErrorAction SilentlyContinue
        throw "GUI save phase failed: $Phase exit=$exitCode`n$stderr`n$stdout"
    }
    $summaryPath = Join-Path $runRoot ("summary_" + $Phase + ".json")
    if (-not (Test-Path -LiteralPath $summaryPath -PathType Leaf)) {
        throw "GUI save phase produced no summary: $Phase"
    }
    $summary = Get-Content -LiteralPath $summaryPath -Raw -Encoding UTF8 | ConvertFrom-Json
    if ([string]$summary.status -ne "passed") {
        throw "GUI save phase did not pass: $Phase"
    }
    return $summary
}

$orchestrator = [ordered]@{
    schema_version = "vit_gui_project_save_matrix_orchestrator.v2"
    status = "running"
    created_at = (Get-Date).ToString("o")
    run_root = $runRoot
    source_project = $SourceProject
    phases = [ordered]@{}
}

try {
    foreach ($phase in @("create", "reopen_regular", "reopen_folder")) {
        $orchestrator.phases[$phase] = Invoke-SavePhase -Phase $phase
        Stop-OwnedLifecycleProcesses
        Start-Sleep -Milliseconds 800
    }
    $orchestrator.status = "passed"
}
catch {
    $orchestrator.status = "failed"
    $orchestrator.error = $_.Exception.Message
    throw
}
finally {
    Stop-OwnedLifecycleProcesses
    Remove-Item Env:\VIT_GUI_SAVE_MATRIX_PHASE -ErrorAction SilentlyContinue
    Remove-Item Env:\VIT_GUI_SAVE_MATRIX_ROOT -ErrorAction SilentlyContinue
    Remove-Item Env:\VIT_GUI_SAVE_MATRIX_SOURCE -ErrorAction SilentlyContinue
    $orchestrator.completed_at = (Get-Date).ToString("o")
    $orchestrator | ConvertTo-Json -Depth 32 | Set-Content -LiteralPath (Join-Path $runRoot "summary.json") -Encoding UTF8
}

Write-Host "ok: real menu + FileDialog project save matrix passed" -ForegroundColor Green
Write-Host ("artifact: " + (Join-Path $runRoot "summary.json"))
