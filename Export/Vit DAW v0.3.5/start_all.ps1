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
Start-Process -FilePath $KernelExe -WorkingDirectory $Runtime -WindowStyle Hidden | Out-Null
Start-Sleep -Milliseconds 500

$BundledPython = Join-Path $Root "python_embed\python.exe"
$PythonExe = $null
if (Test-Path -LiteralPath $BundledPython) {
    $PythonExe = $BundledPython
}
else {
    $pythonCmd = Get-Command py -ErrorAction SilentlyContinue
    if ($null -eq $pythonCmd) {
        $pythonCmd = Get-Command python -ErrorAction SilentlyContinue
    }
    if ($null -eq $pythonCmd) {
        throw "Python not found. Run scripts\bundle_python_embed.ps1 to ship embedded Python, or install py/python."
    }
    $PythonExe = $pythonCmd.Source
}

Write-Host "Starting bridge ..."
$env:PYTHONPATH = $Runtime
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
$BridgeStdOut = Join-Path $Runtime "bridge_stdout.log"
$BridgeStdErr = Join-Path $Runtime "bridge_stderr.log"
if (Test-Path -LiteralPath $BridgeStdOut) { Remove-Item -LiteralPath $BridgeStdOut -Force -ErrorAction SilentlyContinue }
if (Test-Path -LiteralPath $BridgeStdErr) { Remove-Item -LiteralPath $BridgeStdErr -Force -ErrorAction SilentlyContinue }
$bridgeProc = Start-Process -FilePath $PythonExe -ArgumentList $bridgeArgs -WorkingDirectory $Runtime -PassThru -WindowStyle Hidden -RedirectStandardOutput $BridgeStdOut -RedirectStandardError $BridgeStdErr
Start-Sleep -Milliseconds 700
if ($bridgeProc.HasExited) {
    $errText = ""
    if (Test-Path -LiteralPath $BridgeStdErr) {
        $errText = (Get-Content -LiteralPath $BridgeStdErr -Raw -ErrorAction SilentlyContinue)
    }
    if ([string]::IsNullOrWhiteSpace($errText) -and (Test-Path -LiteralPath $BridgeStdOut)) {
        $errText = (Get-Content -LiteralPath $BridgeStdOut -Raw -ErrorAction SilentlyContinue)
    }
    throw "Bridge process exited immediately. Check $BridgeStdErr. Details: $errText"
}

Write-Host "Starting UI Vit_DAW.exe ..."
Start-Process -FilePath $UiExe -WorkingDirectory $Root | Out-Null

Write-Host "All components started."
