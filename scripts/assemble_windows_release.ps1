#requires -Version 5.1
<#
    Assemble a portable Windows release folder for Inno / zip.

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
    [string]$AgentExe = "",
    [string]$LauncherExe = "",
    [string]$PythonEmbedDir = "",
    [string]$GodotUiDependencyDir = "",
    [switch]$LayeredPayload,
    [switch]$MinimalPayloadOnly,
    [switch]$NoPythonBridge,
    [switch]$RequireAgent
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$RepoRoot = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
if ([string]::IsNullOrWhiteSpace($KernelExe)) {
    $KernelExe = Join-Path $RepoRoot "VitApp\build\VitApp_artefacts\Release\VitApp.exe"
}
if ([string]::IsNullOrWhiteSpace($AgentExe)) {
    $AgentExe = Join-Path $RepoRoot "agent\bin\VitAgent.exe"
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
if (-not $NoPythonBridge -and -not (Test-Path -LiteralPath $PythonEmbedDir)) { throw "Python embed dir not found: $PythonEmbedDir" }
if (-not $MinimalPayloadOnly) {
    if (-not (Test-Path -LiteralPath $LauncherExe)) { throw "Launcher not found: $LauncherExe (build Export\ with CMake Release)" }
    if (-not (Test-Path -LiteralPath $TemplateDir)) { throw "Template release folder not found: $TemplateDir" }
}

$RuntimeDir = if ($LayeredPayload) { Join-Path $OutDir "kernel" } else { Join-Path $OutDir "runtime" }
$BridgeDir = if ($LayeredPayload) { Join-Path $OutDir "bridge" } else { $RuntimeDir }
$AgentDir = if ($LayeredPayload) { Join-Path $OutDir "agent" } else { $RuntimeDir }
$UiDir = if ($LayeredPayload) { Join-Path $OutDir "ui" } else { $OutDir }
if (Test-Path -LiteralPath $OutDir) {
    Remove-Item -LiteralPath $OutDir -Recurse -Force
}
New-Item -ItemType Directory -Path $OutDir -Force | Out-Null

if (-not $MinimalPayloadOnly) {
    Write-Host "Copying payload from template: $TemplateDir"
    Copy-Item -Path (Join-Path $TemplateDir "*") -Destination $OutDir -Recurse -Force
}

Write-Host "Updating kernel + agent payload"
New-Item -ItemType Directory -Path $RuntimeDir -Force | Out-Null
if (-not $NoPythonBridge) {
    New-Item -ItemType Directory -Path $BridgeDir -Force | Out-Null
}
New-Item -ItemType Directory -Path $AgentDir -Force | Out-Null
New-Item -ItemType Directory -Path $UiDir -Force | Out-Null
Copy-Item -LiteralPath $KernelExe -Destination (Join-Path $RuntimeDir "VitApp.exe") -Force
if (Test-Path -LiteralPath $AgentExe) {
    Copy-Item -LiteralPath $AgentExe -Destination (Join-Path $AgentDir "VitAgent.exe") -Force
    Write-Host "Included VitAgent.exe: $AgentExe"
    $agentRoot = Split-Path -Parent (Split-Path -Parent $AgentExe)
    $webUIDist = Join-Path $agentRoot "webui\dist"
    if (Test-Path -LiteralPath $webUIDist) {
        $webUIDest = Join-Path $AgentDir "webui\dist"
        New-Item -ItemType Directory -Path $webUIDest -Force | Out-Null
        Copy-Item -Path (Join-Path $webUIDist "*") -Destination $webUIDest -Recurse -Force
        Write-Host "Included Ask Vit WebUI dist: $webUIDist"
    }
    else {
        Write-Warning "Ask Vit WebUI dist not found at $webUIDist; /app will show the not-built placeholder."
    }
}
else {
    if ($RequireAgent) {
        throw "VitAgent.exe not found at $AgentExe. Build D:\Vit_DAW\agent first."
    }
    Write-Warning "VitAgent.exe not found at $AgentExe; release will fall back to Python bridge."
}
if (-not $NoPythonBridge) {
    Copy-Item -LiteralPath (Join-Path $RepoRoot "scripts\bridge_core.py") -Destination (Join-Path $BridgeDir "bridge_core.py") -Force
    if (Test-Path -LiteralPath (Join-Path $RepoRoot "scripts\bridge_prod.py")) {
        Copy-Item -LiteralPath (Join-Path $RepoRoot "scripts\bridge_prod.py") -Destination (Join-Path $BridgeDir "bridge_prod.py") -Force
    }
    if (Test-Path -LiteralPath (Join-Path $RepoRoot "scripts\bridge_prod.config.json")) {
        Copy-Item -LiteralPath (Join-Path $RepoRoot "scripts\bridge_prod.config.json") -Destination (Join-Path $BridgeDir "bridge_prod.config.json") -Force
    }
}

$destUi = Join-Path $UiDir (Split-Path -Leaf $GodotUiExe)
Copy-Item -LiteralPath $GodotUiExe -Destination $destUi -Force

# Godot puts native libraries beside the exported .exe (GDExtension, D3D12 Agility, etc.).
# The installer previously copied only the .exe, so VitWaveformReader never loaded and tiles/SHM waveforms stayed empty.
# PS 5.1: Split-Path -LiteralPath cannot be combined with -Parent (ambiguous parameter set).
$godotExportDir = Split-Path -Parent $GodotUiExe
$godotExeLeaf = Split-Path -Leaf $GodotUiExe
$godotDependencyDirs = New-Object System.Collections.Generic.List[string]
$godotDependencyDirs.Add($godotExportDir)
if (-not [string]::IsNullOrWhiteSpace($GodotUiDependencyDir)) {
    if (-not (Test-Path -LiteralPath $GodotUiDependencyDir)) {
        throw "Godot UI dependency dir not found: $GodotUiDependencyDir"
    }
    $resolvedDependencyDir = (Resolve-Path -LiteralPath $GodotUiDependencyDir).Path
    if (-not ($godotDependencyDirs -contains $resolvedDependencyDir)) {
        $godotDependencyDirs.Add($resolvedDependencyDir)
    }
}
foreach ($dependencyDir in $godotDependencyDirs) {
    Write-Host "Copying Godot export runtime sidecars from: $dependencyDir"
    # Avoid -File with -LiteralPath on Windows PowerShell 5.1 (ambiguous parameter set).
    Get-ChildItem -LiteralPath $dependencyDir -ErrorAction SilentlyContinue | Where-Object { -not $_.PSIsContainer } | ForEach-Object {
        if ($_.Name -ieq $godotExeLeaf) {
            return
        }
        $nameLower = $_.Name.ToLowerInvariant()
        $extLower = $_.Extension.ToLowerInvariant()
        $isRuntimeSidecar =
            $extLower -in @(".dll", ".pak", ".dat", ".bin", ".json") -or
            $nameLower -in @("bootstrap.exe", "bootstrapc.exe", "gdcef_helper.exe", "crashpad_handler.exe")
        if (-not $isRuntimeSidecar) {
            return
        }
        Copy-Item -LiteralPath $_.FullName -Destination (Join-Path $UiDir $_.Name) -Force
    }

    # CEF also needs runtime directories such as locales\. The previous installer
    # copied only top-level files, which could leave the embedded Agent WebUI blank.
    $runtimeSidecarDirs = @("locales", "swiftshader", "WidevineCdm")
    Get-ChildItem -LiteralPath $dependencyDir -ErrorAction SilentlyContinue | Where-Object { $_.PSIsContainer } | ForEach-Object {
        if (-not ($runtimeSidecarDirs -contains $_.Name)) {
            return
        }
        $destDir = Join-Path $UiDir $_.Name
        if (Test-Path -LiteralPath $destDir) {
            Remove-Item -LiteralPath $destDir -Recurse -Force
        }
        Copy-Item -LiteralPath $_.FullName -Destination $destDir -Recurse -Force
        Write-Host "Included Godot/CEF runtime directory: $($_.Name)"
    }
}

if (-not $NoPythonBridge) {
    $pythonDst = Join-Path $OutDir "python_embed"
    New-Item -ItemType Directory -Path $pythonDst -Force | Out-Null
    Copy-Item -Path (Join-Path $PythonEmbedDir "*") -Destination $pythonDst -Recurse -Force
}

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
    if ($LayeredPayload) {
        $mainLauncher = Join-Path $OutDir "Vit DAW.exe"
        Copy-Item -LiteralPath $LauncherExe -Destination $mainLauncher -Force
        Write-Host "Done. Layered release folder: $OutDir"
        Write-Host "Main entry for users: $mainLauncher"
        if ($NoPythonBridge) {
            Write-Host "Included: ui\\$(Split-Path -Leaf $GodotUiExe), kernel\\VitApp.exe, agent\\VitAgent.exe"
        }
        else {
            Write-Host "Included: ui\\$(Split-Path -Leaf $GodotUiExe), kernel\\VitApp.exe, agent\\VitAgent.exe when built, bridge\\bridge_core.py/bridge_prod.py fallback, python_embed\\*"
        }
        return
    }
    Write-Host "Done. Minimal release folder: $OutDir"
    if ($NoPythonBridge) {
        Write-Host "Included: $(Split-Path -Leaf $GodotUiExe), runtime\\VitApp.exe, runtime\\VitAgent.exe"
    }
    else {
        Write-Host "Included: $(Split-Path -Leaf $GodotUiExe), runtime\\VitApp.exe, runtime\\VitAgent.exe when built, runtime\\bridge_core.py/bridge_prod.py fallback, python_embed\\*"
    }
}
