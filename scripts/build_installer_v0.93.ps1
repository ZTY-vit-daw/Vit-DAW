#requires -Version 5.1
<#
    One-click build for Vit DAW v0.93 Windows installer.

    v0.93 layout:
      Vit DAW.exe              launcher entry
      ui\                      Godot UI export + UI/CEF runtime files and dirs
      kernel\                  VitApp.exe
      agent\                   VitAgent.exe + webui\dist

    This release is Go-agent-only. It intentionally does not ship the legacy
    Python bridge or python_embed payload.

    Usage:
      powershell -ExecutionPolicy Bypass -File .\scripts\build_installer_v0.93.ps1
      powershell -ExecutionPolicy Bypass -File .\scripts\build_installer_v0.93.ps1 -SkipInnoCompile
#>
[CmdletBinding()]
param(
    [string]$UiExe = "D:\Vit_DAW\Export\Vit DAW v0.93.exe",
    [string]$ReleaseDir = "",
    [string]$KernelExe = "D:\Vit_DAW\VitApp\build_release\VitApp.exe",
    [string]$AgentExe = "D:\Vit_DAW\agent\bin\VitAgent.exe",
    [string]$LauncherExe = "",
    [string]$UiDependencyDir = "D:\Godot\project\vit-daw-frontend",
    [string]$WebUIDir = "D:\Vit_DAW\agent\webui",
    [string]$InnoCompiler = "",
    [switch]$SkipWebUIBuild,
    [switch]$SkipAgentBuild,
    [switch]$SkipLauncherBuild,
    [switch]$SkipInnoCompile
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$RepoRoot = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
if ([string]::IsNullOrWhiteSpace($ReleaseDir)) {
    $ReleaseDir = Join-Path $RepoRoot "Export\_build\Vit_DAW_v0.93_release"
}
if ([string]::IsNullOrWhiteSpace($LauncherExe)) {
    $LauncherExe = Join-Path $RepoRoot "Export\build_launcher_v0.93\Release\Vit_DAW_Launcher.exe"
}

$AssembleScript = Join-Path $RepoRoot "scripts\assemble_windows_release.ps1"
$IssPath = Join-Path $RepoRoot "Export\installer\Vit_DAW_setup_v0.93.iss"

function Assert-WebUIDist {
    param([Parameter(Mandatory = $true)][string]$DistPath)
    if (-not (Test-Path -LiteralPath (Join-Path $DistPath "index.html"))) {
        throw "Ask Vit WebUI dist is missing index.html: $DistPath"
    }
    $assetsDir = Join-Path $DistPath "assets"
    if (-not (Test-Path -LiteralPath $assetsDir)) {
        throw "Ask Vit WebUI dist is missing assets dir: $assetsDir"
    }
    $hasJs = @(Get-ChildItem -LiteralPath $assetsDir -Filter "*.js" -ErrorAction SilentlyContinue).Count -gt 0
    $hasCss = @(Get-ChildItem -LiteralPath $assetsDir -Filter "*.css" -ErrorAction SilentlyContinue).Count -gt 0
    if (-not $hasJs -or -not $hasCss) {
        throw "Ask Vit WebUI dist must contain bundled JS and CSS assets: $assetsDir"
    }
}

function Assert-UiRuntime {
    param([Parameter(Mandatory = $true)][string]$UiDir)
    foreach ($name in @("libcef.dll", "gdcef.dll", "icudtl.dat", "resources.pak")) {
        $path = Join-Path $UiDir $name
        if (-not (Test-Path -LiteralPath $path)) {
            throw "Godot/CEF runtime file missing from release UI payload: $path"
        }
    }
    $locales = Join-Path $UiDir "locales"
    if (-not (Test-Path -LiteralPath $locales)) {
        throw "Godot/CEF locales directory missing from release UI payload: $locales"
    }
    if (@(Get-ChildItem -LiteralPath $locales -Filter "*.pak" -ErrorAction SilentlyContinue).Count -eq 0) {
        throw "Godot/CEF locales directory has no .pak files: $locales"
    }
}

if (-not (Test-Path -LiteralPath $AssembleScript)) { throw "Missing script: $AssembleScript" }
if (-not (Test-Path -LiteralPath $IssPath)) { throw "Missing Inno script: $IssPath" }
if (-not (Test-Path -LiteralPath $UiExe)) { throw "UI exe not found: $UiExe" }
if (-not (Test-Path -LiteralPath $KernelExe)) { throw "Kernel exe not found: $KernelExe" }
if (-not (Test-Path -LiteralPath $WebUIDir)) { throw "WebUI dir not found: $WebUIDir" }
if (-not [string]::IsNullOrWhiteSpace($UiDependencyDir) -and -not (Test-Path -LiteralPath $UiDependencyDir)) {
    throw "UI dependency dir not found: $UiDependencyDir"
}

if (-not $SkipWebUIBuild) {
    Write-Host "[1/5] Building Ask Vit WebUI ..."
    Push-Location $WebUIDir
    try {
        npm run build
    }
    finally {
        Pop-Location
    }
    if ($LASTEXITCODE -ne 0) { throw "Ask Vit WebUI build failed with exit code $LASTEXITCODE" }
}
Assert-WebUIDist (Join-Path $WebUIDir "dist")

if (-not $SkipAgentBuild) {
    Write-Host "[2/5] Building VitAgent ..."
    Push-Location (Join-Path $RepoRoot "agent")
    try {
        go build -o $AgentExe .\cmd\vitagent
    }
    finally {
        Pop-Location
    }
    if ($LASTEXITCODE -ne 0) { throw "VitAgent build failed with exit code $LASTEXITCODE" }
}
if (-not (Test-Path -LiteralPath $AgentExe)) {
    throw "VitAgent.exe not found: $AgentExe"
}

if (-not $SkipLauncherBuild) {
    Write-Host "[3/5] Building launcher ..."
    $LauncherSourceDir = Join-Path $RepoRoot "Export"
    $LauncherBuildDir = Join-Path $RepoRoot "Export\build_launcher_v0.93"
    cmake -S $LauncherSourceDir -B $LauncherBuildDir -A x64
    if ($LASTEXITCODE -ne 0) { throw "Launcher CMake configure failed with exit code $LASTEXITCODE" }
    cmake --build $LauncherBuildDir --config Release
    if ($LASTEXITCODE -ne 0) { throw "Launcher build failed with exit code $LASTEXITCODE" }
}
if (-not (Test-Path -LiteralPath $LauncherExe)) {
    throw "Launcher exe not found: $LauncherExe"
}

Write-Host "[4/5] Assembling v0.93 Go-agent-only release folder ..."
$assembleArgs = @{
    GodotUiExe           = $UiExe
    OutDir               = $ReleaseDir
    KernelExe            = $KernelExe
    AgentExe             = $AgentExe
    LauncherExe          = $LauncherExe
    GodotUiDependencyDir = $UiDependencyDir
    MinimalPayloadOnly   = $true
    LayeredPayload       = $true
    NoPythonBridge       = $true
    RequireAgent         = $true
}
& $AssembleScript @assembleArgs

Assert-WebUIDist (Join-Path $ReleaseDir "agent\webui\dist")
Assert-UiRuntime (Join-Path $ReleaseDir "ui")
Write-Host "Verified release includes Ask Vit WebUI dist and Godot/CEF runtime directories."

if ($SkipInnoCompile) {
    Write-Host "[5/5] Skipped Inno compile by request."
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

Write-Host "[5/5] Compiling installer with Inno Setup ..."
$releaseDefineArg = "/DCustomReleaseDir=$ReleaseDir"
& $InnoCompiler $releaseDefineArg $IssPath
if ($LASTEXITCODE -ne 0) { throw "Inno compile failed with exit code $LASTEXITCODE" }

$InstallerPath = Join-Path $RepoRoot "Export\installer\Vit_DAW_0.93_Setup.exe"
if (Test-Path -LiteralPath $InstallerPath) {
    Write-Host "Done. Installer created: $InstallerPath"
}
else {
    Write-Host "Compile finished. Please check output under: $(Join-Path $RepoRoot "Export\installer")"
}
