# vitshell_smoke.ps1 - SHELL-1 acceptance smoke (card pool #356).
#
# Real-stack light leg: VitShell.exe (WebView2 main window host) -> agent tree
# (isolated worktree binary) -> React launcher surface -> greeting real data
# -> close -> pid-level tree teardown + port release. Zero Godot processes at
# any point (product ruling: Godot is not in the startup chain).
#
# Usage (repo root or anywhere):
#   powershell -ExecutionPolicy Bypass -File scripts\vitshell_smoke.ps1
#   -FrontendWorktree <path>  frontend repo worktree holding VitShell/ (default
#                             D:/Godot/project/vit-daw-frontend_wt_shell1)
#   -Webview2SdkDir <path>    dir with include/WebView2.h + WebView2Loader.dll
#                             (default: main frontend checkout, loader is
#                             locally excluded from frontend worktrees)
#   -StatusPort 7879 -AgentPort 7878 -AgentReadyTimeoutSec 120
#   -RunRoot <path>           default coord/runs/SHELL-1/run_<stamp>
#   -SkipBuild                reuse built binaries (record hashes in artifacts)
#
# PASS = exit 0. Artifacts land under RunRoot (meta.json, status snapshots,
# greeting response, vitshell stderr log, agent last-log tail).

param(
    [string]$FrontendWorktree = "D:/Godot/project/vit-daw-frontend_wt_shell1",
    [string]$Webview2SdkDir = "D:/Godot/project/vit-daw-frontend/extension/third_party/webview2",
    [int]$StatusPort = 7879,
    [int]$AgentPort = 7878,
    [int]$AgentReadyTimeoutSec = 120,
    [string]$RunRoot = "",
    [switch]$SkipBuild
)

$ErrorActionPreference = "Stop"
$MainRoot = Split-Path -Parent $PSScriptRoot   # main-repo worktree root (scripts/..)

# ---------- helpers ----------

function Write-Step { param([string]$Message) Write-Host ""; Write-Host ">>> $Message" }
function Write-Ok { param([string]$Message) Write-Host "  [OK] $Message" -ForegroundColor Green }
function Write-Bad { param([string]$Message) Write-Host "  [FAIL] $Message" -ForegroundColor Red }

function Get-TcpListenerPid {
    param([int]$Port)
    $conn = Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($null -eq $conn) { return $null }
    return [int]$conn.OwningProcess
}

function Get-VitProcessCount {
    param([string[]]$Names)
    $count = 0
    foreach ($n in $Names) {
        $count += @(Get-Process -Name $n -ErrorAction SilentlyContinue).Count
    }
    return $count
}

function Get-HttpJson {
    param([string]$Url, [int]$TimeoutSec = 5)
    try {
        $resp = Invoke-WebRequest -Uri $Url -UseBasicParsing -TimeoutSec $TimeoutSec
        return @{ ok = $true; status = [int]$resp.StatusCode; body = $resp.Content }
    } catch {
        return @{ ok = $false; status = 0; body = $_.Exception.Message }
    }
}

$VitProcessNames = @("VitShell", "VitAgent", "VitApp", "VspHub", "Godot", "godot")

# ---------- preflight (S2.2 guard: never reuse a live stack) ----------

Write-Step "Preflight: stack must be idle (no reuse - PROTOCOL S2.2)"
$preProcs = Get-VitProcessCount -Names $VitProcessNames
if ($preProcs -ne 0) {
    throw ("preflight: Vit-family processes already running (count=" + $preProcs + ") - resolve ownership before running this smoke (no silent reuse)")
}
foreach ($port in @($AgentPort, $StatusPort)) {
    $owner = Get-TcpListenerPid -Port $port
    if ($null -ne $owner) {
        throw ("preflight: port " + $port + " already listening (pid=" + $owner + ") - no silent reuse")
    }
}
Write-Ok ("processes=0, ports " + $AgentPort + "/" + $StatusPort + " free")

# ---------- artifacts scaffold ----------

if ([string]::IsNullOrWhiteSpace($RunRoot)) {
    $stamp = Get-Date -Format "yyyyMMdd_HHmmss"
    $RunRoot = Join-Path $MainRoot "coord/runs/SHELL-1/run_$stamp"
}
New-Item -ItemType Directory -Path $RunRoot -Force | Out-Null
Write-Step "Artifacts: $RunRoot"

$runMeta = [ordered]@{
    run_id = (Split-Path -Leaf $RunRoot)
    started_at_utc = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
    card = "SHELL-1"
    main_worktree = $MainRoot
    frontend_worktree = $FrontendWorktree
    webview2_sdk_dir = $Webview2SdkDir
    status_port = $StatusPort
    agent_port = $AgentPort
    skip_build = [bool]$SkipBuild
}

# Record tested-tree state (S9: HEAD + dirty state + binary hashes).
pushd $MainRoot
git rev-parse HEAD 2>$null | Out-File -FilePath (Join-Path $RunRoot "main_head.txt") -Encoding utf8
git status --short 2>$null | Out-File -FilePath (Join-Path $RunRoot "main_status.txt") -Encoding utf8
popd

# ---------- build VitShell ----------

$VitShellExe = Join-Path $FrontendWorktree "VitShell/build/Release/VitShell.exe"
if (-not $SkipBuild) {
    Write-Step "Build VitShell.exe (cmake, VS x64 Release)"
    pushd $FrontendWorktree
    cmake -S VitShell -B VitShell/build -G "Visual Studio 18 2026" -A x64 "-DVITSHELL_WEBVIEW2_DIR=$Webview2SdkDir" 2>&1 |
        Out-File -FilePath (Join-Path $RunRoot "vitshell_cmake_configure.log") -Encoding utf8
    if ($LASTEXITCODE -ne 0) { popd; throw "cmake configure failed (see vitshell_cmake_configure.log)" }
    cmake --build VitShell/build --config Release 2>&1 |
        Out-File -FilePath (Join-Path $RunRoot "vitshell_cmake_build.log") -Encoding utf8
    $buildCode = $LASTEXITCODE
    popd
    if ($buildCode -ne 0) { throw "cmake build failed (see vitshell_cmake_build.log)" }
    Write-Ok "VitShell build exit 0"
} else {
    if (-not (Test-Path $VitShellExe)) { throw "-SkipBuild: VitShell.exe missing at $VitShellExe" }
    Write-Ok ("-SkipBuild: reusing " + $VitShellExe)
}
$runMeta.vitshell_exe = $VitShellExe
$runMeta.vitshell_exe_sha256 = (Get-FileHash $VitShellExe -Algorithm SHA256).Hash
$runMeta.vitshell_exe_mtime = (Get-Item $VitShellExe).LastWriteTimeUtc.ToString("o")

# ---------- build isolated agent binary ----------

$AgentExe = Join-Path $MainRoot "agent/bin/VitAgent.exe"
if (-not $SkipBuild) {
    Write-Step "Build isolated VitAgent.exe (worktree agent source, zero agent changes)"
    pushd (Join-Path $MainRoot "agent")
    go build -o ".\bin\VitAgent.exe" .\cmd\vitagent 2>&1 |
        Out-File -FilePath (Join-Path $RunRoot "agent_build.log") -Encoding utf8
    $goCode = $LASTEXITCODE
    popd
    if ($goCode -ne 0) { throw "go build agent failed (see agent_build.log)" }
    Write-Ok "agent build exit 0"
} else {
    if (-not (Test-Path $AgentExe)) { throw "-SkipBuild: agent binary missing at $AgentExe" }
    Write-Ok ("-SkipBuild: reusing " + $AgentExe)
}
$runMeta.agent_exe = $AgentExe
$runMeta.agent_exe_sha256 = (Get-FileHash $AgentExe -Algorithm SHA256).Hash

# ---------- webui dist provenance (zero webui changes; source-faithful copy) ----------
# agent/webui/dist is git-ignored, so a fresh worktree lacks it. The main
# checkout carries a build of the same source tree (asserted below); copy it
# with hash evidence instead of rebuilding (npm ci would not change fidelity).

Write-Step "webui dist provenance (built fresh from THIS run tree - zero webui changes by this card)"
# dist/ is git-ignored: build it in the run tree so provenance is airtight
# (agent serves agent/webui/dist resolved by walking up from cwd).
$runWebui = Join-Path $MainRoot "agent/webui"
$runDist = Join-Path $runWebui "dist"
if (-not $SkipBuild -or -not (Test-Path (Join-Path $runDist "index.html"))) {
    if (-not (Test-Path (Join-Path $runWebui "node_modules"))) {
        pushd $runWebui
        npm ci --no-audit --no-fund 2>&1 | Out-File -FilePath (Join-Path $RunRoot "webui_npm_ci.log") -Encoding utf8
        $npmCode = $LASTEXITCODE
        popd
        if ($npmCode -ne 0) { throw "npm ci failed (see webui_npm_ci.log)" }
        Write-Ok "npm ci exit 0"
    }
    pushd $runWebui
    npm run build 2>&1 | Out-File -FilePath (Join-Path $RunRoot "webui_build.log") -Encoding utf8
    $uiCode = $LASTEXITCODE
    popd
    if ($uiCode -ne 0) { throw "npm run build failed (see webui_build.log)" }
    Write-Ok "npm run build exit 0 (tsc --noEmit + vite build)"
} else {
    Write-Ok "-SkipBuild: reusing existing run-tree dist"
}
$distIndexHash = (Get-FileHash (Join-Path $runDist "index.html") -Algorithm SHA256).Hash
$runMeta.webui_dist_index_sha256 = $distIndexHash
Write-Ok ("dist/index.html sha256=" + $distIndexHash.Substring(0, 16) + "...")

# ---------- preflight again immediately before stack start (<=60s recheck) ----------

Write-Step "Immediate recheck (PROTOCOL S2.2: within 60s of stack start)"
$reProcs = Get-VitProcessCount -Names $VitProcessNames
if ($reProcs -ne 0) { throw "recheck: Vit-family processes appeared (count=$reProcs)" }
foreach ($port in @($AgentPort, $StatusPort)) {
    if ($null -ne (Get-TcpListenerPid -Port $port)) { throw "recheck: port $port got occupied" }
}
Write-Ok "clean (processes=0, ports free)"

# ---------- launch ----------

$LauncherUrl = "http://127.0.0.1:$AgentPort/app/?surface=launcher"
$AgentUrl = "http://127.0.0.1:$AgentPort"
$AgentHttpAddr = "127.0.0.1:$AgentPort"
$vitLog = Join-Path $RunRoot "vitshell_stderr.log"

Write-Step "Launch VitShell (main window first, agent tree inside job object)"
$proc = Start-Process -FilePath $VitShellExe `
    -ArgumentList @(
        "--agent-exe", $AgentExe,
        "--dev-root", $MainRoot,
        "--agent-url", $AgentUrl,
        "--agent-http-addr", $AgentHttpAddr,
        "--url", $LauncherUrl,
        "--status-port", "$StatusPort"
    ) `
    -RedirectStandardError $vitLog -PassThru -WindowStyle Hidden
$runMeta.vitshell_pid = $proc.Id
Write-Ok ("VitShell pid=" + $proc.Id)

try {
    # ---- leg A: main window present (independent enumeration by title) ----
    Write-Step "Leg A: main window present (process enumeration, title 'Vit')"
    $deadline = (Get-Date).AddSeconds(20)
    $windowTitle = ""
    while ((Get-Date) -lt $deadline) {
        Start-Sleep -Milliseconds 400
        $vitProc = Get-Process -Id $proc.Id -ErrorAction SilentlyContinue
        if ($null -eq $vitProc) { throw "VitShell exited during startup (see vitshell_stderr.log)" }
        if (-not [string]::IsNullOrWhiteSpace($vitProc.MainWindowTitle)) {
            $windowTitle = $vitProc.MainWindowTitle
            break
        }
    }
    if ($windowTitle -ne "Vit") {
        throw ("main window title mismatch: got '" + $windowTitle + "' expected 'Vit'")
    }
    Write-Ok ("main window enumerated: title='Vit'")

    # ---- leg B: zero Godot processes during startup (product ruling) ----
    Write-Step "Leg B: zero Godot processes in the startup chain"
    $godotCount = @(Get-Process -Name "Godot", "godot" -ErrorAction SilentlyContinue).Count
    if ($godotCount -ne 0) { throw "Godot processes detected in startup chain (count=$godotCount)" }
    Write-Ok "Godot processes = 0"

    # ---- leg C: loading -> agent ready -> launcher surface (real data) ----
    Write-Step "Leg C: agent ready -> React launcher surface navigated (real data)"
    $statusUrl = "http://127.0.0.1:$StatusPort/status"
    $deadline = (Get-Date).AddSeconds($AgentReadyTimeoutSec)
    $status = $null
    while ((Get-Date) -lt $deadline) {
        Start-Sleep -Milliseconds 500
        if (Get-Process -Id $proc.Id -ErrorAction SilentlyContinue) {
            $status = $null
            $r = Get-HttpJson -Url $statusUrl -TimeoutSec 3
            if ($r.ok) {
                try { $status = $r.body | ConvertFrom-Json } catch { $status = $null }
            }
            if ($null -ne $status -and $status.phase -eq "ready") { break }
            if ($null -ne $status -and $status.phase -eq "error") {
                throw ("VitShell reports error phase: " + $status.error)
            }
        }
    }
    if ($null -eq $status -or $status.phase -ne "ready") {
        $lastPhase = "unreachable"
        if ($null -ne $status) { $lastPhase = $status.phase }
        throw ("agent ready timeout after ${AgentReadyTimeoutSec}s (last phase=" + $lastPhase + ")")
    }

    # Loading gate ends only at document complete (async React chunks load
    # after navigation; VitShell keeps re-probing readyState on the status face).
    $deadline = (Get-Date).AddSeconds(30)
    while ((Get-Date) -lt $deadline) {
        if ($null -ne $status -and $status.document_ready -eq "complete") { break }
        Start-Sleep -Milliseconds 500
        $r2 = Get-HttpJson -Url $statusUrl -TimeoutSec 3
        if ($r2.ok) { try { $status = $r2.body | ConvertFrom-Json } catch {} }
    }
    $r2.body | Out-File -FilePath (Join-Path $RunRoot "status_ready.json") -Encoding utf8
    Write-Ok ("phase=ready agent_pid=" + $status.agent_pid)

    if ($status.current_url -ne $LauncherUrl) {
        throw ("current_url mismatch: got '" + $status.current_url + "' expected '" + $LauncherUrl + "'")
    }
    if ($status.document_ready -ne "complete") {
        throw ("document.readyState not complete within 30s: '" + $status.document_ready + "'")
    }
    Write-Ok "React launcher surface navigated + document complete (loading state gone)"

    # React face is agent-served: /app/ must return the built bundle (not a 404 proxy).
    $appResp = Get-HttpJson -Url $LauncherUrl -TimeoutSec 5
    if (-not $appResp.ok -or $appResp.status -ne 200) {
        throw ("/app/ not served by agent: status=" + $appResp.status)
    }
    if ($appResp.body -notmatch 'id=.root.') {
        throw "/app/ response does not look like the webui index (no root mount)"
    }
    $appResp.body | Out-File -FilePath (Join-Path $RunRoot "app_index.html") -Encoding utf8
    Write-Ok "/app/ served 200 with webui index"

    # ---- leg D: greeting real data ----
    Write-Step "Leg D: /agent/launcher/greeting real data"
    $greeting = Get-HttpJson -Url "$AgentUrl/agent/launcher/greeting" -TimeoutSec 5
    if (-not $greeting.ok -or $greeting.status -ne 200) {
        throw ("greeting HTTP failed: " + $greeting.body)
    }
    $greeting.body | Out-File -FilePath (Join-Path $RunRoot "greeting.json") -Encoding utf8
    $gj = $greeting.body | ConvertFrom-Json
    if ($gj.status -ne "ok") { throw "greeting status != ok" }
    if ([string]::IsNullOrWhiteSpace([string]$gj.reply)) { throw "greeting reply empty (not real data)" }
    Write-Ok ("greeting reply: " + ([string]$gj.reply).Substring(0, [Math]::Min(60, ([string]$gj.reply).Length)) + "...")

    # ---- leg E: pid-level port ownership (the 7878 listener IS the managed agent) ----
    Write-Step "Leg E: port 7878 listener pid == managed agent pid"
    $listenerPid = Get-TcpListenerPid -Port $AgentPort
    if ($null -eq $listenerPid) { throw "no listener on port $AgentPort before close" }
    if ($listenerPid -ne [int]$status.agent_pid) {
        throw ("port $AgentPort owned by pid=$listenerPid, expected agent pid=" + $status.agent_pid)
    }
    Write-Ok ("port $AgentPort owned by agent pid=" + $listenerPid)

    # ---- leg F: close main window -> pid-level tree teardown + port release ----
    Write-Step "Leg F: close main window -> agent tree exits, ports release"
    $closeStatus = 0
    try {
        $cr = Invoke-WebRequest -Uri "http://127.0.0.1:$StatusPort/close" -Method POST -UseBasicParsing -TimeoutSec 5
        $closeStatus = [int]$cr.StatusCode
    } catch {
        $closeStatus = 0
    }
    if ($closeStatus -ne 202) { throw "POST /close failed (status=$closeStatus)" }

    $deadline = (Get-Date).AddSeconds(15)
    $vitAlive = $true
    while ((Get-Date) -lt $deadline) {
        Start-Sleep -Milliseconds 400
        if ($null -eq (Get-Process -Id $proc.Id -ErrorAction SilentlyContinue)) { $vitAlive = $false; break }
    }
    if ($vitAlive) { throw "VitShell still alive 15s after close" }
    Write-Ok "VitShell exited"

    $agentGone = $null -eq (Get-Process -Id ([int]$status.agent_pid) -ErrorAction SilentlyContinue)
    if (-not $agentGone) { throw ("agent pid " + $status.agent_pid + " still alive after window close") }
    Write-Ok ("agent pid " + $status.agent_pid + " gone")

    foreach ($port in @($AgentPort, $StatusPort)) {
        $deadline2 = (Get-Date).AddSeconds(8)
        $freed = $false
        while ((Get-Date) -lt $deadline2) {
            if ($null -eq (Get-TcpListenerPid -Port $port)) { $freed = $true; break }
            Start-Sleep -Milliseconds 400
        }
        if (-not $freed) { throw "port $port still listening after close" }
        Write-Ok "port $port released"
    }

    $finalProcs = Get-VitProcessCount -Names $VitProcessNames
    if ($finalProcs -ne 0) { throw "Vit-family processes remain after close (count=$finalProcs)" }
    Write-Ok "zero Vit-family processes remain (incl. Godot)"

    $runMeta.result = "PASS"
} catch {
    $runMeta.result = "FAIL"
    $runMeta.error = $_.Exception.Message
    $runMeta | ConvertTo-Json -Depth 4 | Out-File -FilePath (Join-Path $RunRoot "meta.json") -Encoding utf8
    # Best-effort teardown so a failed leg does not leave the stack occupied.
    Write-Bad ("FAIL: " + $_.Exception.Message + " - best-effort teardown")
    try {
        Invoke-WebRequest -Uri "http://127.0.0.1:$StatusPort/close" -Method POST -UseBasicParsing -TimeoutSec 3 | Out-Null
    } catch {}
    Start-Sleep -Seconds 3
    foreach ($n in @("VitShell", "VitAgent")) {
        Get-Process -Name $n -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
    }
    $runMeta.teardown = "best-effort executed (verify manually; PROTOCOL S2.2 residue rules apply)"
    $runMeta | ConvertTo-Json -Depth 4 | Out-File -FilePath (Join-Path $RunRoot "meta.json") -Encoding utf8
    throw
}

$runMeta.finished_at_utc = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
$runMeta | ConvertTo-Json -Depth 4 | Out-File -FilePath (Join-Path $RunRoot "meta.json") -Encoding utf8

Write-Host ""
Write-Host ("PASS: vitshell_smoke " + $runMeta.run_id + " (exit 0)") -ForegroundColor Green
Write-Host ("artifacts: " + $RunRoot)
exit 0
