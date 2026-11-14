# build_all.ps1 - SHELL-3 M1 environment prep: agent + webui + VitShell + godot import.
# Logs land in coord/runs/SHELL-3/build/. Idempotent; each step logs its own exit code.
$ErrorActionPreference = "Continue"
$MainRoot = "D:/Vit_DAW_wt_shell3"
$Frontend = "D:/Godot/project/vit-daw-frontend_wt_shell3"
$Webview2SdkDir = "D:/Godot/project/vit-daw-frontend/extension/third_party/webview2"
$GodotExe = "D:/Godot/Godot_v4.6.1-stable_win64_console.exe"
$logDir = Join-Path $MainRoot "coord/runs/SHELL-3/build"
New-Item -ItemType Directory -Path $logDir -Force | Out-Null

function Step {
    param([string]$Name, [scriptblock]$Cmd, [string]$Log)
    Write-Host "=== $Name ==="
    & $Cmd 2>&1 | Out-File -FilePath $Log -Encoding utf8
    Write-Host ("exit=" + $LASTEXITCODE + " log=" + $Log)
}

Step "agent go build" { pushd (Join-Path $MainRoot "agent"); go build -o ".\bin\VitAgent.exe" .\cmd\vitagent; popd } (Join-Path $logDir "agent_build.log")

$env:PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD = "1"
if (-not (Test-Path (Join-Path $MainRoot "agent/webui/node_modules"))) {
    Step "webui npm install" { pushd (Join-Path $MainRoot "agent/webui"); npm install; popd } (Join-Path $logDir "webui_install.log")
} else {
    Write-Host "=== webui node_modules present, skip install ==="
}
Step "webui npm run build" { pushd (Join-Path $MainRoot "agent/webui"); npm run build; popd } (Join-Path $logDir "webui_build.log")

Step "VitShell cmake configure" { pushd $Frontend; cmake -S VitShell -B VitShell/build -G "Visual Studio 18 2026" -A x64 "-DVITSHELL_WEBVIEW2_DIR=$Webview2SdkDir"; popd } (Join-Path $logDir "vitshell_cmake_configure.log")
Step "VitShell cmake build" { pushd $Frontend; cmake --build VitShell/build --config Release; popd } (Join-Path $logDir "vitshell_cmake_build.log")

Step "godot import (worktree caches)" { pushd $Frontend; & $GodotExe --headless --import --path .; popd } (Join-Path $logDir "godot_import.log")

Write-Host "ALL_BUILD_STEPS_DONE"
