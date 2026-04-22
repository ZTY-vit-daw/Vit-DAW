Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$Root = Split-Path -Parent $MyInvocation.MyCommand.Path
$Runtime = Join-Path $Root "runtime"
$KernelExe = Join-Path $Runtime "VitApp.exe"
$BridgeProdPy = Join-Path $Runtime "bridge_prod.py"
$BridgeCorePy = Join-Path $Runtime "bridge_core.py"
$BridgeConfig = Join-Path $Runtime "bridge_prod.config.json"
$UiExe = Join-Path $Root "Vit_DAW.exe"
if (-not (Test-Path -LiteralPath $UiExe)) {
    $verHit = Get-ChildItem -Path $Root -File -Filter "Vit DAW v*.exe" -ErrorAction SilentlyContinue |
        Sort-Object { $_.Name.Length } -Descending | Select-Object -First 1
    if ($null -ne $verHit) {
        $UiExe = $verHit.FullName
    }
    else {
        $alt = Get-ChildItem -Path $Root -File -Filter "Vit DAW*.exe" -ErrorAction SilentlyContinue |
            Where-Object { $_.Name -ne "Vit DAW.exe" } |
            Sort-Object { $_.Name.Length } -Descending | Select-Object -First 1
        if ($null -ne $alt) {
            $UiExe = $alt.FullName
        }
    }
}

$AppLogRoot = Join-Path $Root "logs"
$UserLogRoot = if ([string]::IsNullOrWhiteSpace($env:LOCALAPPDATA)) {
    Join-Path $env:TEMP "Vit DAW\logs"
} else {
    Join-Path $env:LOCALAPPDATA "Vit DAW\logs"
}

function Resolve-BridgeLogRoot {
    param(
        [string]$PrimaryPath,
        [string]$FallbackPath
    )
    foreach ($candidate in @($PrimaryPath, $FallbackPath)) {
        try {
            New-Item -ItemType Directory -Path $candidate -Force | Out-Null
            $probe = Join-Path $candidate "__vit_write_test.tmp"
            Set-Content -LiteralPath $probe -Value "ok" -Encoding ASCII
            Remove-Item -LiteralPath $probe -Force -ErrorAction SilentlyContinue
            return $candidate
        }
        catch {
            continue
        }
    }
    throw "Cannot create writable log directory. Tried: $PrimaryPath ; $FallbackPath"
}

$BridgeLogRoot = Resolve-BridgeLogRoot -PrimaryPath $AppLogRoot -FallbackPath $UserLogRoot
$BridgeLog = Join-Path $BridgeLogRoot "bridge_last.log"

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
$BridgeStdOut = Join-Path $BridgeLogRoot "bridge_stdout.log"
$BridgeStdErr = Join-Path $BridgeLogRoot "bridge_stderr.log"
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

Write-Host "Starting UI: $UiExe"
Start-Process -FilePath $UiExe -WorkingDirectory $Root | Out-Null

Write-Host "All components started."
