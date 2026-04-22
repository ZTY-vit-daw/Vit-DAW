Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$Root = Split-Path -Parent $MyInvocation.MyCommand.Path
$Runtime = Join-Path $Root "runtime"
$KernelExe = Join-Path $Runtime "VitApp.exe"
$BridgeProdPy = Join-Path $Runtime "bridge_prod.py"
$BridgeCorePy = Join-Path $Runtime "bridge_core.py"
$BridgeConfig = Join-Path $Runtime "bridge_prod.config.json"
$BridgeLog = Join-Path $Runtime "bridge_last.log"
$UiExe = Join-Path $Root "Vit_DAW.exe"

if (-not (Test-Path -LiteralPath $KernelExe)) { throw "Missing kernel: $KernelExe" }
if (-not (Test-Path -LiteralPath $BridgeProdPy) -and -not (Test-Path -LiteralPath $BridgeCorePy)) { throw "Missing bridge script: bridge_prod.py / bridge_core.py" }
if (-not (Test-Path -LiteralPath $UiExe)) { throw "Missing UI executable: $UiExe" }

Write-Host "Starting kernel VitApp.exe ..."
Start-Process -FilePath $KernelExe -WorkingDirectory $Runtime | Out-Null
Start-Sleep -Milliseconds 500

$pythonCmd = Get-Command py -ErrorAction SilentlyContinue
if ($null -eq $pythonCmd) {
    $pythonCmd = Get-Command python -ErrorAction SilentlyContinue
}
if ($null -eq $pythonCmd) {
    throw "Python not found. Please install Python launcher (py) or python."
}

Write-Host "Starting bridge ..."
if (Test-Path -LiteralPath $BridgeProdPy) {
    $env:VIT_BRIDGE_LAST_LOG_PATH = $BridgeLog
    if (Test-Path -LiteralPath $BridgeConfig) {
        $env:VIT_BRIDGE_CONFIG = $BridgeConfig
    }
    $bridgeArgs = @("`"$BridgeProdPy`"")
}
else {
    $bridgeArgs = @("`"$BridgeCorePy`"", "--last-log-path", "`"$BridgeLog`"")
}
Start-Process -FilePath $pythonCmd.Source -ArgumentList $bridgeArgs -WorkingDirectory $Runtime | Out-Null
Start-Sleep -Milliseconds 700

Write-Host "Starting UI Vit_DAW.exe ..."
Start-Process -FilePath $UiExe -WorkingDirectory $Root | Out-Null

Write-Host "All components started."
