param(
    [string]$VitAppExe = "D:\Vit_DAW\VitApp\build\VitApp_artefacts\Release\VitApp.exe",
    [ValidateSet("A","B","C")]
    [string]$StartFrom = "A"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

if (-not (Test-Path $VitAppExe)) {
    Write-Host "ERROR: VitApp.exe not found: $VitAppExe" -ForegroundColor Red
    exit 1
}

function Run-Case {
    param(
        [string]$Name,
        [string]$Mode,
        [string]$Pool,
        [string]$Smooth
    )

    Write-Host ""
    Write-Host "==================================================" -ForegroundColor Cyan
    Write-Host "Running case: $Name" -ForegroundColor Cyan
    Write-Host "VIT_BAKER_MODE=$Mode | VIT_BAKER_POOL=$Pool | VIT_BAKER_SMOOTH=$Smooth"
    Write-Host "==================================================" -ForegroundColor Cyan

    $env:VIT_BAKER_DEBUG = "1"
    $env:VIT_BAKER_MODE = $Mode
    $env:VIT_BAKER_POOL = $Pool
    $env:VIT_BAKER_SMOOTH = $Smooth

    Write-Host ""
    Write-Host "Steps now:" -ForegroundColor Yellow
    Write-Host "1) Wait VitApp starts."
    Write-Host "2) In Godot, import D:/Vit_DAW/test_target_3s.wav"
    Write-Host "3) Switch to FREQ and play once."
    Write-Host "4) Close VitApp window when done."
    Write-Host ""
    Write-Host "Press Enter to launch this case..."
    [void](Read-Host)

    & $VitAppExe

    Write-Host ""
    Write-Host "Case [$Name] finished." -ForegroundColor Green
    Write-Host "Copy lines containing: BakerDiag"
    Write-Host "Press Enter to continue..."
    [void](Read-Host)
}

Write-Host "Baker diagnostic matrix runner" -ForegroundColor Green
Write-Host "VitApp: $VitAppExe"
Write-Host "StartFrom: $StartFrom"
Write-Host ""
Write-Host "This script runs 3 cases sequentially."
Write-Host "Please complete one Godot playback per case."

if ($StartFrom -eq "A") {
    Run-Case -Name "A Baseline" -Mode "5" -Pool "max" -Smooth "off"
    Run-Case -Name "B Centroid only" -Mode "5" -Pool "centroid" -Smooth "off"
    Run-Case -Name "C Practical" -Mode "4" -Pool "centroid" -Smooth "on"
}
elseif ($StartFrom -eq "B") {
    Run-Case -Name "B Centroid only" -Mode "5" -Pool "centroid" -Smooth "off"
    Run-Case -Name "C Practical" -Mode "4" -Pool "centroid" -Smooth "on"
}
else {
    Run-Case -Name "C Practical" -Mode "4" -Pool "centroid" -Smooth "on"
}

Write-Host ""
Write-Host "All cases done." -ForegroundColor Green
Write-Host "Now send me all BakerDiag lines from A/B/C."

# Optional cleanup (kept for this shell only)
Remove-Item Env:VIT_BAKER_DEBUG -ErrorAction SilentlyContinue
Remove-Item Env:VIT_BAKER_MODE -ErrorAction SilentlyContinue
Remove-Item Env:VIT_BAKER_POOL -ErrorAction SilentlyContinue
Remove-Item Env:VIT_BAKER_SMOOTH -ErrorAction SilentlyContinue
