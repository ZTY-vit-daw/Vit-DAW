#requires -Version 5.1
<#
    One-click build for Vit DAW v0.9 Windows installer.

    v0.9 layout:
      Vit DAW.exe              launcher entry
      ui\                      Godot UI export + UI DLLs
      kernel\                  VitApp.exe
      agent\                   VitAgent.exe
      bridge\                  Python bridge fallback scripts/config
      python_embed\            embedded Python runtime fallback

    Usage:
      powershell -ExecutionPolicy Bypass -File .\scripts\build_installer_v0.9.ps1
      powershell -ExecutionPolicy Bypass -File .\scripts\build_installer_v0.9.ps1 -SkipInnoCompile
#>
[CmdletBinding()]
param(
    [string]$UiExe = "D:\Vit_DAW\Export\Vit DAW v0.9.exe",
    [string]$ReleaseDir = "",
    [string]$KernelExe = "D:\Vit_DAW\VitApp\build_release\VitApp_artefacts\Release\VitApp.exe",
    [string]$AgentExe = "D:\Vit_DAW\agent\bin\VitAgent.exe",
    [string]$LauncherExe = "",
    [string]$PythonEmbedDir = "",
    [string]$UiDependencyDir = "D:\Godot\project\vit-daw-frontend",
    [string]$InnoCompiler = "",
    [switch]$SkipInnoCompile
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$RepoRoot = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
if ([string]::IsNullOrWhiteSpace($ReleaseDir)) {
    $ReleaseDir = Join-Path $RepoRoot "Export\_build\Vit_DAW_v0.9_release"
}
if ([string]::IsNullOrWhiteSpace($PythonEmbedDir)) {
    $PythonEmbedDir = Join-Path $RepoRoot "Export\staging\python_embed"
}
if ([string]::IsNullOrWhiteSpace($LauncherExe)) {
    $LauncherExe = Join-Path $RepoRoot "Export\build_launcher\Release\Vit_DAW_Launcher.exe"
}

$AssembleScript = Join-Path $RepoRoot "scripts\assemble_windows_release.ps1"
$IssPath = Join-Path $RepoRoot "Export\installer\Vit_DAW_setup_v0.9.iss"

if (-not (Test-Path -LiteralPath $AssembleScript)) { throw "Missing script: $AssembleScript" }
if (-not (Test-Path -LiteralPath $IssPath)) { throw "Missing Inno script: $IssPath" }
if (-not (Test-Path -LiteralPath $UiExe)) { throw "UI exe not found: $UiExe" }
if (-not (Test-Path -LiteralPath $KernelExe)) { throw "Kernel exe not found: $KernelExe" }
if (-not (Test-Path -LiteralPath $AgentExe)) { throw "Agent exe not found: $AgentExe. Build D:\Vit_DAW\agent first." }
if (-not (Test-Path -LiteralPath $LauncherExe)) { throw "Launcher exe not found: $LauncherExe. Build D:\Vit_DAW\Export with CMake Release first." }
if (-not [string]::IsNullOrWhiteSpace($UiDependencyDir) -and -not (Test-Path -LiteralPath $UiDependencyDir)) {
    throw "UI dependency dir not found: $UiDependencyDir"
}

Write-Host "[1/2] Assembling v0.9 layered release folder ..."
$assembleArgs = @{
    GodotUiExe           = $UiExe
    OutDir               = $ReleaseDir
    PythonEmbedDir       = $PythonEmbedDir
    KernelExe            = $KernelExe
    AgentExe             = $AgentExe
    LauncherExe          = $LauncherExe
    GodotUiDependencyDir = $UiDependencyDir
    MinimalPayloadOnly   = $true
    LayeredPayload       = $true
}
& $AssembleScript @assembleArgs

if ($SkipInnoCompile) {
    Write-Host "[2/2] Skipped Inno compile by request."
    Write-Host "Release folder ready: $ReleaseDir"
    exit 0
}

if ([string]::IsNullOrWhiteSpace($InnoCompiler)) {
    $innoCandidates = @(
        (Join-Path ${env:ProgramFiles(x86)} "Inno Setup 6\ISCC.exe"),
        (Join-Path $env:ProgramFiles "Inno Setup 6\ISCC.exe")
    )
    $InnoCompiler = $innoCandidates | Where-Object { Test-Path -LiteralPath $_ } | Select-Object -First 1
}
if ([string]::IsNullOrWhiteSpace($InnoCompiler) -or -not (Test-Path -LiteralPath $InnoCompiler)) {
    throw "ISCC.exe not found. Install Inno Setup 6, or pass -InnoCompiler <path-to-ISCC.exe>."
}

Write-Host "[2/2] Compiling installer with Inno Setup ..."
$releaseDefineArg = "/DCustomReleaseDir=$ReleaseDir"
& $InnoCompiler $releaseDefineArg $IssPath
if ($LASTEXITCODE -ne 0) { throw "Inno compile failed with exit code $LASTEXITCODE" }

$InstallerPath = Join-Path $RepoRoot "Export\installer\Vit_DAW_0.9_Setup.exe"
if (Test-Path -LiteralPath $InstallerPath) {
    Write-Host "Done. Installer created: $InstallerPath"
}
else {
    Write-Host "Compile finished. Please check output under: $(Join-Path $RepoRoot "Export\installer")"
}
