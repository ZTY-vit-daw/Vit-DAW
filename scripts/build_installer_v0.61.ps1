#requires -Version 5.1
<#
    One-click build for Vit DAW v0.61 Windows installer (minimal payload: embed Python, kernel, bridge, Godot UI).
    Usage:
      powershell -ExecutionPolicy Bypass -File .\scripts\build_installer_v0.61.ps1
#>
[CmdletBinding()]
param(
    [string]$UiExe = "D:\Godot\project\vit-daw-frontend\Vit DAW v0.61.exe",
    [string]$ReleaseDir = "",
    [string]$KernelExe = "D:\Vit_DAW\VitApp\build_release\VitApp_artefacts\Release\VitApp.exe",
    [string]$PythonEmbedDir = "",
    [string]$InnoCompiler = "",
    [switch]$SkipInnoCompile
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$RepoRoot = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
if ([string]::IsNullOrWhiteSpace($ReleaseDir)) {
    $ReleaseDir = Join-Path $RepoRoot "Export\_build\Vit_DAW_v0.61_release"
}
if ([string]::IsNullOrWhiteSpace($PythonEmbedDir)) {
    $PythonEmbedDir = Join-Path $RepoRoot "Export\staging\python_embed"
}

$AssembleScript = Join-Path $RepoRoot "scripts\assemble_windows_release.ps1"
$IssPath = Join-Path $RepoRoot "Export\installer\Vit_DAW_setup_v0.61.iss"

if (-not (Test-Path -LiteralPath $AssembleScript)) { throw "Missing script: $AssembleScript" }
if (-not (Test-Path -LiteralPath $IssPath)) { throw "Missing Inno script: $IssPath" }
if (-not (Test-Path -LiteralPath $UiExe)) { throw "UI exe not found: $UiExe" }
if (-not (Test-Path -LiteralPath $KernelExe)) { throw "Kernel exe not found: $KernelExe" }

Write-Host "[1/2] Assembling release folder ..."
$assembleArgs = @{
    GodotUiExe         = $UiExe
    OutDir             = $ReleaseDir
    PythonEmbedDir     = $PythonEmbedDir
    KernelExe          = $KernelExe
    MinimalPayloadOnly = $true
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

$InstallerPath = Join-Path $RepoRoot "Export\installer\Vit_DAW_0.61_Setup.exe"
if (Test-Path -LiteralPath $InstallerPath) {
    Write-Host "Done. Installer created: $InstallerPath"
}
else {
    Write-Host "Compile finished. Please check output under: $(Join-Path $RepoRoot "Export\installer")"
}
