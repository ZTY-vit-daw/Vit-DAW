[CmdletBinding()]
param(
    [string]$RepoRoot = "D:\Vit_DAW",
    [string]$StemsFolder = "E:\BaiduNetdiskDownload\yingge - sattelites tracks out",
    [string]$GodotProjectRoot = "D:\Godot\project\vit-daw-frontend",
    [string]$GodotExe = "D:\Godot\Godot_v4.6.1-stable_win64_console.exe",
    [int]$TimeoutSeconds = 900
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$RepoRoot = (Resolve-Path -LiteralPath $RepoRoot).Path
$StemsFolder = (Resolve-Path -LiteralPath $StemsFolder).Path
$GodotProjectRoot = (Resolve-Path -LiteralPath $GodotProjectRoot).Path
$GodotExe = (Resolve-Path -LiteralPath $GodotExe).Path
$artifactRoot = Join-Path $RepoRoot "VitApp\Workspace\Artifacts"
$smokeRoot = Join-Path $artifactRoot "smoke"
$stamp = Get-Date -Format "yyyyMMdd_HHmmss"
$runRoot = Join-Path $smokeRoot ("project_package_reopen_" + $stamp)
$projectFolder = Join-Path $runRoot "portable_project\C1 Portable Reopen"
$projectPath = Join-Path $projectFolder "C1 Portable Reopen.vit"
$createArtifacts = Join-Path $runRoot "phase_create"
$reopenArtifacts = Join-Path $runRoot "phase_reopen"
$quarantineRoot = Join-Path $runRoot "global_artifacts_quarantine"
$restoredRuntimeRoot = Join-Path $runRoot "restored_runtime_evidence"
$productScript = Join-Path $RepoRoot "scripts\run_vit_product_path_smoke.ps1"

New-Item -ItemType Directory -Path $runRoot -Force | Out-Null
New-Item -ItemType Directory -Path $quarantineRoot -Force | Out-Null
New-Item -ItemType Directory -Path $restoredRuntimeRoot -Force | Out-Null

$globalTargets = @(
    [pscustomobject]@{ Name = "acoustic_package_status.json"; Path = (Join-Path $artifactRoot "acoustic_package_status.json") },
    [pscustomobject]@{ Name = "mixboard_feature_snapshot.json"; Path = (Join-Path $artifactRoot "mixboard_feature_snapshot.json") },
    [pscustomobject]@{ Name = "mixboard"; Path = (Join-Path $artifactRoot "mixboard") }
)

function Assert-OwnedArtifactPath {
    param([string]$Path)
    $full = [System.IO.Path]::GetFullPath($Path)
    $root = [System.IO.Path]::GetFullPath($artifactRoot).TrimEnd('\') + '\'
    if (-not $full.StartsWith($root, [System.StringComparison]::OrdinalIgnoreCase)) {
        throw "Refusing artifact move outside workspace artifact root: $full"
    }
}

function Move-GlobalArtifactsTo {
    param([string]$DestinationRoot)
    New-Item -ItemType Directory -Path $DestinationRoot -Force | Out-Null

    $moves = @()
    foreach ($target in $globalTargets) {
        Assert-OwnedArtifactPath -Path $target.Path
        if (-not (Test-Path -LiteralPath $target.Path)) {
            continue
        }
        $destination = Join-Path $DestinationRoot $target.Name
        if (Test-Path -LiteralPath $destination) {
            throw "Artifact quarantine target already exists: $destination"
        }
        $moves += [pscustomobject]@{ Source = $target.Path; Destination = $destination }
    }

    $completed = @()
    try {
        foreach ($move in $moves) {
            Move-Item -LiteralPath $move.Source -Destination $move.Destination
            $completed += $move
        }
    }
    catch {
        for ($index = $completed.Count - 1; $index -ge 0; $index--) {
            $move = $completed[$index]
            if ((Test-Path -LiteralPath $move.Destination) -and -not (Test-Path -LiteralPath $move.Source)) {
                Move-Item -LiteralPath $move.Destination -Destination $move.Source
            }
        }
        throw
    }
    return $completed.Count
}

function Restore-GlobalArtifacts {
    foreach ($target in $globalTargets) {
        $source = Join-Path $quarantineRoot $target.Name
        if (-not (Test-Path -LiteralPath $source)) {
            continue
        }
        Assert-OwnedArtifactPath -Path $target.Path
        if (Test-Path -LiteralPath $target.Path) {
            throw "Cannot restore quarantined artifact over existing path: $($target.Path)"
        }
        Move-Item -LiteralPath $source -Destination $target.Path
    }
}

$summary = [ordered]@{
    schema_version = "vit_project_package_reopen_orchestrator.v1"
    status = "running"
    created_at = (Get-Date).ToString("o")
    run_root = $runRoot
    project_path = $projectPath
    stems_folder = $StemsFolder
    create_summary = $null
    reopen_summary = $null
    global_artifacts_restored = $false
}
$quarantineTaken = $false

try {
    & powershell -NoProfile -ExecutionPolicy Bypass -File $productScript `
        -RepoRoot $RepoRoot `
        -GodotExe $GodotExe `
        -GodotProjectRoot $GodotProjectRoot `
        -ProjectPackagePhase create `
        -ProjectPackageProjectPath $projectPath `
        -ProjectPackageStemsFolder $StemsFolder `
        -ProjectPackageArtifactDir $createArtifacts `
        -TimeoutSeconds $TimeoutSeconds
    if ($LASTEXITCODE -ne 0) {
        throw "Project-package create lifecycle failed with exit code $LASTEXITCODE"
    }
    $summary.create_summary = Get-Content -LiteralPath (Join-Path $createArtifacts "summary.json") -Raw -Encoding UTF8 | ConvertFrom-Json

    $quarantinedCount = Move-GlobalArtifactsTo -DestinationRoot $quarantineRoot
    $quarantineTaken = $quarantinedCount -gt 0

    & powershell -NoProfile -ExecutionPolicy Bypass -File $productScript `
        -RepoRoot $RepoRoot `
        -GodotExe $GodotExe `
        -GodotProjectRoot $GodotProjectRoot `
        -SkipBuild `
        -ProjectPackagePhase reopen `
        -ProjectPackageProjectPath $projectPath `
        -ProjectPackageArtifactDir $reopenArtifacts `
        -TimeoutSeconds $TimeoutSeconds
    if ($LASTEXITCODE -ne 0) {
        throw "Project-package reopen lifecycle failed with exit code $LASTEXITCODE"
    }
    $summary.reopen_summary = Get-Content -LiteralPath (Join-Path $reopenArtifacts "summary.json") -Raw -Encoding UTF8 | ConvertFrom-Json
    $summary.status = "passed"
}
catch {
    $summary.status = "failed"
    $summary.error = $_.Exception.Message
    throw
}
finally {
    try {
        if ($quarantineTaken) {
            Move-GlobalArtifactsTo -DestinationRoot $restoredRuntimeRoot | Out-Null
            Restore-GlobalArtifacts
        }
        $summary.global_artifacts_restored = $true
    }
    catch {
        $summary.global_artifacts_restore_error = $_.Exception.Message
        if ($summary.status -eq "passed") {
            $summary.status = "failed"
        }
    }
    $summary.completed_at = (Get-Date).ToString("o")
    $summary | ConvertTo-Json -Depth 32 | Set-Content -LiteralPath (Join-Path $runRoot "summary.json") -Encoding UTF8
}

if ($summary.status -ne "passed" -or -not $summary.global_artifacts_restored) {
    throw "Project-package reopen C1 smoke did not complete safely; see $runRoot\summary.json"
}

Write-Host "ok: project-package save-close-clear-reopen-C1 smoke passed" -ForegroundColor Green
Write-Host ("artifact: " + (Join-Path $runRoot "summary.json"))
