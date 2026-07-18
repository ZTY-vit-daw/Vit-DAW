#requires -Version 5.1

[CmdletBinding()]
param(
    [string]$RepoRoot = "D:\Vit_DAW",
    [string]$GodotProjectRoot = "D:\Godot\project\vit-daw-frontend",
    [string]$GodotExe = "D:\Godot\Godot_v4.6.1-stable_win64_console.exe",
    [string]$KernelExe = "",
    [string]$MaterialPath = "",
    [string]$PythonExe = "python",
    [switch]$SkipGodotHeadless,
    [switch]$SkipGoTests,
    [switch]$SkipKernelSmokes,
    [switch]$SkipProductPath,
    [switch]$SkipBuild,
    [switch]$ReuseGodot,
    [switch]$ReuseAgent,
    [switch]$ReuseKernel,
    [int]$TimeoutSeconds = 60
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Resolve-RequiredPath {
    param(
        [string]$Path,
        [string]$Label,
        [switch]$Leaf
    )
    if ([string]::IsNullOrWhiteSpace($Path)) {
        throw ($Label + " was not supplied")
    }
    if ($Leaf) {
        if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
            throw ($Label + " not found: " + $Path)
        }
    }
    elseif (-not (Test-Path -LiteralPath $Path)) {
        throw ($Label + " not found: " + $Path)
    }
    return (Resolve-Path -LiteralPath $Path).Path
}

function Write-Summary {
    $script:Summary["updated_at"] = (Get-Date).ToString("o")
    $script:Summary | ConvertTo-Json -Depth 24 | Set-Content -LiteralPath $script:SummaryPath -Encoding UTF8
}

function Write-StepLine {
    param([string]$Message)
    Write-Host ""
    Write-Host ("== " + $Message) -ForegroundColor Cyan
}

function Invoke-LoggedStep {
    param(
        [string]$Name,
        [scriptblock]$Action
    )
    Write-StepLine $Name
    $started = Get-Date
    $record = [ordered]@{
        name = $Name
        status = "running"
        started_at = $started.ToString("o")
    }
    $script:Summary["steps"] += @($record)
    $script:CurrentStep = $record
    Write-Summary
    try {
        & $Action
        $record["status"] = "passed"
    }
    catch {
        $record["status"] = "failed"
        $record["error"] = $_.Exception.Message
        throw
    }
    finally {
        $ended = Get-Date
        $record["ended_at"] = $ended.ToString("o")
        $record["duration_seconds"] = [math]::Round(($ended - $started).TotalSeconds, 3)
        Write-Summary
        $script:CurrentStep = $null
    }
}

function Invoke-NativeLogged {
    param(
        [string]$LogPath,
        [scriptblock]$Action
    )
    New-Item -ItemType Directory -Path (Split-Path -Parent $LogPath) -Force | Out-Null
    $oldErrorActionPreference = $ErrorActionPreference
    try {
        $ErrorActionPreference = "Continue"
        & $Action *>&1 | Tee-Object -FilePath $LogPath
        $exitCode = $LASTEXITCODE
    }
    finally {
        $ErrorActionPreference = $oldErrorActionPreference
    }
    if ($null -ne $script:CurrentStep) {
        $script:CurrentStep["log_path"] = $LogPath
        $script:CurrentStep["exit_code"] = $exitCode
    }
    if ($exitCode -ne 0) {
        throw ("command failed with exit code " + $exitCode + "; log=" + $LogPath)
    }
}

function Get-LatestChildPath {
    param(
        [string]$Root,
        [string]$Filter,
        [switch]$Recurse
    )
    if (-not (Test-Path -LiteralPath $Root)) {
        return ""
    }
    $item = Get-ChildItem -LiteralPath $Root -Filter $Filter -Recurse:$Recurse -ErrorAction SilentlyContinue |
        Sort-Object LastWriteTime -Descending |
        Select-Object -First 1
    if ($null -eq $item) {
        return ""
    }
    return $item.FullName
}

function Validate-L2RenderProbeSummary {
    param([string]$SummaryPath)
    if ([string]::IsNullOrWhiteSpace($SummaryPath) -or -not (Test-Path -LiteralPath $SummaryPath -PathType Leaf)) {
        throw "L2 render probe summary was not produced"
    }
    $report = Get-Content -LiteralPath $SummaryPath -Raw | ConvertFrom-Json
    $row = @($report.features | Where-Object { [string]$_.feature_type -eq "l2_render_probe" } | Select-Object -First 1)
    if ($row.Count -eq 0) {
        throw ("l2_render_probe feature report missing: " + $SummaryPath)
    }
    $status = [string]$row[0].status
    if ($status -notin @("ready", "suspect")) {
        throw ("l2_render_probe status is not ready/suspect: " + $status)
    }
    $leaks = @($row[0].raw_leak_keys)
    if ($leaks.Count -ne 0) {
        throw ("l2_render_probe raw payload leaked: " + ($leaks -join ","))
    }
}

$RepoRoot = Resolve-RequiredPath -Path $RepoRoot -Label "repo root"
$GodotProjectRoot = Resolve-RequiredPath -Path $GodotProjectRoot -Label "Godot project root"
if (-not $SkipGodotHeadless -or -not $SkipProductPath) {
    $GodotExe = Resolve-RequiredPath -Path $GodotExe -Label "Godot executable" -Leaf
}
if (-not [string]::IsNullOrWhiteSpace($KernelExe)) {
    $KernelExe = Resolve-RequiredPath -Path $KernelExe -Label "kernel executable" -Leaf
}
if (-not [string]::IsNullOrWhiteSpace($MaterialPath)) {
    $MaterialPath = Resolve-RequiredPath -Path $MaterialPath -Label "material path" -Leaf
}

$AgentRoot = Join-Path $RepoRoot "agent"
if (-not (Test-Path -LiteralPath (Join-Path $AgentRoot "go.mod") -PathType Leaf)) {
    throw ("agent go.mod not found: " + $AgentRoot)
}

$Stamp = Get-Date -Format "yyyyMMdd_HHmmss"
$ArtifactDir = Join-Path $RepoRoot ("VitApp\Workspace\Artifacts\smoke\observation_v1_acceptance_" + $Stamp)
New-Item -ItemType Directory -Path $ArtifactDir -Force | Out-Null
$SummaryPath = Join-Path $ArtifactDir "summary.json"
$SmokeRoot = Join-Path $RepoRoot "VitApp\Workspace\Artifacts\smoke"
$DadProbeRoot = Join-Path $RepoRoot "VitApp\Workspace\Artifacts\dad_probe"

$script:CurrentStep = $null
$script:SummaryPath = $SummaryPath
$script:Summary = [ordered]@{
    schema_version = "observation_v1_acceptance_smoke.v1"
    status = "running"
    started_at = (Get-Date).ToString("o")
    repo_root = $RepoRoot
    godot_project_root = $GodotProjectRoot
    godot_exe = $GodotExe
    kernel_exe = $KernelExe
    material_path = $MaterialPath
    artifact_dir = $ArtifactDir
    steps = @()
    artifacts = [ordered]@{}
}
Write-Summary

try {
    if (-not $SkipGodotHeadless) {
        Invoke-LoggedStep "Godot headless parse" {
            $log = Join-Path $ArtifactDir "godot_headless_parse.log"
            Invoke-NativeLogged -LogPath $log -Action {
                & $GodotExe --headless --path $GodotProjectRoot --quit
            }
        }
    }

    if (-not $SkipGoTests) {
        Invoke-LoggedStep "Observation Go regression set" {
            $log = Join-Path $ArtifactDir "go_observation_regression.log"
            Invoke-NativeLogged -LogPath $log -Action {
                Push-Location $AgentRoot
                try {
                    & go test ./internal/acousticpackage ./internal/mom ./internal/tim ./internal/mixboard ./internal/agentloop ./internal/chat -count=1
                }
                finally {
                    Pop-Location
                }
            }
        }
    }

    if (-not $SkipKernelSmokes) {
        Invoke-LoggedStep "DAD L3 package smoke" {
            $log = Join-Path $ArtifactDir "dad_l3_package_smoke.log"
            $scriptPath = Join-Path $RepoRoot "scripts\run_dad_l3_package_smoke.ps1"
            $stepArgs = @{
                RepoRoot = $RepoRoot
                PythonExe = $PythonExe
                WaitSeconds = $TimeoutSeconds
            }
            if (-not [string]::IsNullOrWhiteSpace($KernelExe)) { $stepArgs["KernelExe"] = $KernelExe }
            if (-not [string]::IsNullOrWhiteSpace($MaterialPath)) { $stepArgs["MaterialPath"] = $MaterialPath }
            if ($ReuseKernel) { $stepArgs["ReuseKernel"] = $true }
            Invoke-NativeLogged -LogPath $log -Action {
                & $scriptPath @stepArgs
            }
            $latest = Get-LatestChildPath -Root $DadProbeRoot -Filter "dad_probe_summary.json" -Recurse
            $script:Summary["artifacts"]["dad_l3_summary"] = $latest
            if ($null -ne $script:CurrentStep) { $script:CurrentStep["summary_path"] = $latest }
        }

        Invoke-LoggedStep "L2 render probe smoke" {
            $log = Join-Path $ArtifactDir "l2_render_probe_smoke.log"
            $scriptPath = Join-Path $RepoRoot "scripts\run_kernel_dad_probe_smoke.ps1"
            $stepArgs = @{
                RepoRoot = $RepoRoot
                PythonExe = $PythonExe
                Features = "waveform_envelope,spectral_field,l2_render_probe"
                WaitSeconds = $TimeoutSeconds
            }
            if (-not [string]::IsNullOrWhiteSpace($KernelExe)) { $stepArgs["KernelExe"] = $KernelExe }
            if (-not [string]::IsNullOrWhiteSpace($MaterialPath)) { $stepArgs["MaterialPath"] = $MaterialPath }
            if ($ReuseKernel) { $stepArgs["ReuseKernel"] = $true }
            Invoke-NativeLogged -LogPath $log -Action {
                & $scriptPath @stepArgs
            }
            $latest = Get-LatestChildPath -Root $DadProbeRoot -Filter "dad_probe_summary.json" -Recurse
            Validate-L2RenderProbeSummary -SummaryPath $latest
            $script:Summary["artifacts"]["l2_render_probe_summary"] = $latest
            if ($null -ne $script:CurrentStep) { $script:CurrentStep["summary_path"] = $latest }
        }

        Invoke-LoggedStep "L2 realtime observation smoke" {
            $log = Join-Path $ArtifactDir "l2_realtime_observation_smoke.log"
            $scriptPath = Join-Path $RepoRoot "scripts\run_l2_realtime_observation_smoke.ps1"
            $stepArgs = @{
                RepoRoot = $RepoRoot
                GodotProjectRoot = $GodotProjectRoot
                GodotExe = $GodotExe
            }
            if (-not [string]::IsNullOrWhiteSpace($MaterialPath)) { $stepArgs["MaterialPath"] = $MaterialPath }
            Invoke-NativeLogged -LogPath $log -Action {
                & $scriptPath @stepArgs
            }
            $latest = Get-LatestChildPath -Root $SmokeRoot -Filter "l2_realtime_observation_*"
            $script:Summary["artifacts"]["l2_realtime_observation_dir"] = $latest
            if ($null -ne $script:CurrentStep) { $script:CurrentStep["artifact_path"] = $latest }
        }
    }

    Invoke-LoggedStep "AB result smoke" {
        $log = Join-Path $ArtifactDir "ab_result_smoke.log"
        $scriptPath = Join-Path $RepoRoot "scripts\run_ab_result_smoke.ps1"
        Invoke-NativeLogged -LogPath $log -Action {
            & $scriptPath -RepoRoot $RepoRoot
        }
    }

    if (-not $SkipProductPath) {
        Invoke-LoggedStep "Godot product-path lifecycle smoke" {
            $log = Join-Path $ArtifactDir "product_path_smoke.log"
            $scriptPath = Join-Path $RepoRoot "scripts\run_vit_product_path_smoke.ps1"
            $agentExe = Join-Path $RepoRoot "agent\bin\VitAgent.exe"
            $stepArgs = @{
                RepoRoot = $RepoRoot
                GodotProjectRoot = $GodotProjectRoot
                GodotExe = $GodotExe
                AgentExe = $agentExe
                TimeoutSeconds = $TimeoutSeconds
            }
            if (-not [string]::IsNullOrWhiteSpace($KernelExe)) { $stepArgs["KernelExe"] = $KernelExe }
            if ($SkipBuild) { $stepArgs["SkipBuild"] = $true }
            if ($ReuseGodot) { $stepArgs["ReuseGodot"] = $true }
            if ($ReuseAgent) { $stepArgs["ReuseAgent"] = $true }
            if ($ReuseKernel) { $stepArgs["ReuseKernel"] = $true }
            Invoke-NativeLogged -LogPath $log -Action {
                & $scriptPath @stepArgs
            }
            $latest = Get-LatestChildPath -Root $SmokeRoot -Filter "product_path_*"
            $script:Summary["artifacts"]["product_path_dir"] = $latest
            if ($null -ne $script:CurrentStep) { $script:CurrentStep["artifact_path"] = $latest }
        }
    }

    $script:Summary["status"] = "passed"
    $script:Summary["ended_at"] = (Get-Date).ToString("o")
    Write-Summary
    Write-Host ("ok: Observation v1 acceptance smoke passed; summary=" + $SummaryPath) -ForegroundColor Green
}
catch {
    $script:Summary["status"] = "failed"
    $script:Summary["ended_at"] = (Get-Date).ToString("o")
    $script:Summary["error"] = $_.Exception.Message
    Write-Summary
    throw
}
