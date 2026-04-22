#requires -Version 5.1
<#
    One-click build for Vit DAW v0.5 Windows installer.
    Steps:
      1) Assemble portable release folder
      2) Compile Inno Setup installer
    Usage:
      powershell -ExecutionPolicy Bypass -File .\scripts\build_installer_v0.5.ps1
#>
[CmdletBinding()]
param(
    [string]$UiExe = "D:\Vit DAW1\Vit DAW v0.5.exe",
    [string]$ReleaseDir = "",
    [string]$KernelExe = "",
    [string]$PythonEmbedDir = "",
    [string]$InnoCompiler = "",
    [switch]$SkipInnoCompile
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$RepoRoot = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
if ([string]::IsNullOrWhiteSpace($ReleaseDir)) {
    $ReleaseDir = Join-Path $RepoRoot "Export\_build\Vit DAW v0.5 minimal"
}
if ([string]::IsNullOrWhiteSpace($PythonEmbedDir)) {
    $PythonEmbedDir = Join-Path $RepoRoot "Export\staging\python_embed"
}

$AssembleScript = Join-Path $RepoRoot "scripts\assemble_windows_release.ps1"
$IssPath = Join-Path $RepoRoot "Export\installer\Vit_DAW_setup_v0.5.iss"

if (-not (Test-Path -LiteralPath $AssembleScript)) { throw "Missing script: $AssembleScript" }
if (-not (Test-Path -LiteralPath $IssPath)) { throw "Missing Inno script: $IssPath" }
if (-not (Test-Path -LiteralPath $UiExe)) { throw "UI exe not found: $UiExe" }

Write-Host "[1/2] Assembling release folder ..."
$assembleArgs = @{
    GodotUiExe = $UiExe
    OutDir = $ReleaseDir
    PythonEmbedDir = $PythonEmbedDir
    MinimalPayloadOnly = $true
}
if (-not [string]::IsNullOrWhiteSpace($KernelExe)) {
    $assembleArgs.KernelExe = $KernelExe
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

$InstallerPath = Join-Path $RepoRoot "Export\installer\Vit_DAW_0.5_Setup.exe"
if (Test-Path -LiteralPath $InstallerPath) {
    Write-Host "Done. Installer created: $InstallerPath"
}
else {
    Write-Host "Compile finished. Please check output under: $(Join-Path $RepoRoot "Export\installer")"
}
