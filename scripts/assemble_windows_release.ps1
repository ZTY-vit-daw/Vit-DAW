#requires -Version 5.1
<#
    Assemble a portable Windows release folder for Inno / zip (kernel + embed Python + Godot UI + launcher).

    Example:
      powershell -ExecutionPolicy Bypass -File .\scripts\assemble_windows_release.ps1 `
        -GodotUiExe "D:\Godot\project\vit-daw-frontend\Vit DAW v0.4.exe" `
        -OutDir "D:\Vit_DAW\Export\Vit DAW v0.4" `
        -TemplateDir "D:\Vit_DAW\Export\Vit DAW v0.3.5"
#>
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$GodotUiExe,
    [Parameter(Mandatory = $true)]
    [string]$OutDir,
    [string]$TemplateDir = "",
    [string]$KernelExe = "",
    [string]$LauncherExe = "",
    [string]$PythonEmbedDir = "",
    [switch]$MinimalPayloadOnly
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$RepoRoot = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
if ([string]::IsNullOrWhiteSpace($KernelExe)) {
    $KernelExe = Join-Path $RepoRoot "VitApp\build\VitApp_artefacts\Release\VitApp.exe"
}
if ([string]::IsNullOrWhiteSpace($LauncherExe)) {
    $LauncherExe = Join-Path $RepoRoot "Export\build_launcher\Release\Vit_DAW_Launcher.exe"
}
if ([string]::IsNullOrWhiteSpace($TemplateDir)) {
    $TemplateDir = Join-Path $RepoRoot "Export\Vit DAW v0.3.5"
}
if ([string]::IsNullOrWhiteSpace($PythonEmbedDir)) {
    $PythonEmbedDir = Join-Path $RepoRoot "Export\staging\python_embed"
}

if (-not (Test-Path -LiteralPath $GodotUiExe)) { throw "Godot UI exe not found: $GodotUiExe" }
if (-not (Test-Path -LiteralPath $KernelExe)) { throw "Kernel exe not found: $KernelExe" }
if (-not (Test-Path -LiteralPath $PythonEmbedDir)) { throw "Python embed dir not found: $PythonEmbedDir" }
if (-not $MinimalPayloadOnly) {
    if (-not (Test-Path -LiteralPath $LauncherExe)) { throw "Launcher not found: $LauncherExe (build Export\ with CMake Release)" }
    if (-not (Test-Path -LiteralPath $TemplateDir)) { throw "Template release folder not found: $TemplateDir" }
}

$RuntimeDir = Join-Path $OutDir "runtime"
if (Test-Path -LiteralPath $OutDir) {
    Remove-Item -LiteralPath $OutDir -Recurse -Force
}
New-Item -ItemType Directory -Path $OutDir -Force | Out-Null

if (-not $MinimalPayloadOnly) {
    Write-Host "Copying payload from template: $TemplateDir"
    Copy-Item -Path (Join-Path $TemplateDir "*") -Destination $OutDir -Recurse -Force
}

Write-Host "Updating kernel + bridge scripts"
New-Item -ItemType Directory -Path $RuntimeDir -Force | Out-Null
Copy-Item -LiteralPath $KernelExe -Destination (Join-Path $RuntimeDir "VitApp.exe") -Force
Copy-Item -LiteralPath (Join-Path $RepoRoot "scripts\bridge_core.py") -Destination (Join-Path $RuntimeDir "bridge_core.py") -Force
if (Test-Path -LiteralPath (Join-Path $RepoRoot "scripts\bridge_prod.py")) {
    Copy-Item -LiteralPath (Join-Path $RepoRoot "scripts\bridge_prod.py") -Destination (Join-Path $RuntimeDir "bridge_prod.py") -Force
}
if (Test-Path -LiteralPath (Join-Path $RepoRoot "scripts\bridge_prod.config.json")) {
    Copy-Item -LiteralPath (Join-Path $RepoRoot "scripts\bridge_prod.config.json") -Destination (Join-Path $RuntimeDir "bridge_prod.config.json") -Force
}

$destUi = Join-Path $OutDir (Split-Path -Leaf $GodotUiExe)
Copy-Item -LiteralPath $GodotUiExe -Destination $destUi -Force

$pythonDst = Join-Path $OutDir "python_embed"
New-Item -ItemType Directory -Path $pythonDst -Force | Out-Null
Copy-Item -Path (Join-Path $PythonEmbedDir "*") -Destination $pythonDst -Recurse -Force

if (-not $MinimalPayloadOnly) {
    $legacyUi = Join-Path $OutDir "Vit_DAW.exe"
    $godotLeaf = Split-Path -Leaf $GodotUiExe
    if ($godotLeaf -ine "Vit_DAW.exe" -and (Test-Path -LiteralPath $legacyUi)) {
        Remove-Item -LiteralPath $legacyUi -Force
    }
    $rogueLauncher = Join-Path $OutDir "Vit_DAW_Launcher.exe"
    if (Test-Path -LiteralPath $rogueLauncher) {
        Remove-Item -LiteralPath $rogueLauncher -Force
    }

    $mainLauncher = Join-Path $OutDir "Vit DAW.exe"
    Copy-Item -LiteralPath $LauncherExe -Destination $mainLauncher -Force

    $startPs1 = Join-Path $RepoRoot "Export\staging\start_all.ps1"
    $startBat = Join-Path $RepoRoot "Export\staging\start_all.bat"
    if ((Test-Path -LiteralPath $startPs1) -and (Test-Path -LiteralPath $startBat)) {
        Copy-Item -LiteralPath $startPs1 -Destination (Join-Path $OutDir "start_all.ps1") -Force
        Copy-Item -LiteralPath $startBat -Destination (Join-Path $OutDir "start_all.bat") -Force
    }

    $hc = Join-Path $TemplateDir "health_check.ps1"
    if (Test-Path -LiteralPath $hc) {
        Copy-Item -LiteralPath $hc -Destination (Join-Path $OutDir "health_check.ps1") -Force
    }
    Write-Host "Done. Release folder: $OutDir"
    Write-Host "Main entry for users: $mainLauncher"
}
else {
    Write-Host "Done. Minimal release folder: $OutDir"
    Write-Host "Included: $(Split-Path -Leaf $GodotUiExe), runtime\\VitApp.exe, runtime\\bridge_core.py/bridge_prod.py, python_embed\\*"
}
