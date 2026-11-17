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
    [switch]$SkipBuild,
    [switch]$Win32RestoreForensics,
    [switch]$GodotRestoreForensics,
    [string]$ForensicsGodotExe = 'D:/Godot/Godot_v4.6.1-stable_win64.exe'
)

$ErrorActionPreference = "Stop"
$MainRoot = Split-Path -Parent $PSScriptRoot   # main-repo worktree root (scripts/..)

# Card 403: explicitly selected, independent owned-window experiment. Return
# before any product build/start/HTTP/project code below. No product code links
# to this probe, and the normal smoke path is unchanged when this switch is off.
if ($Win32RestoreForensics -or $GodotRestoreForensics) {
    if ($Win32RestoreForensics -and $GodotRestoreForensics) { throw 'Select only one 403 phase' }
    if ($SkipBuild) { throw "403 requires a fresh isolated probe build" }
    if ([string]::IsNullOrWhiteSpace($RunRoot)) { throw "403 requires a new explicit -RunRoot" }
    if (Test-Path -LiteralPath $RunRoot) { throw "403 refuses to overwrite an existing run directory" }
    $null = New-Item -ItemType Directory -Path $RunRoot
    $RunRoot = (Resolve-Path -LiteralPath $RunRoot).Path
    $probeResults = @()
    $probeExit = 1
    $probeProcess = $null
    $probeStarted = Get-Date
    $recentsPath = Join-Path $env:APPDATA 'Vit-DAW/global_project_config.json'
    function Get-403RecentsHash {
        if (Test-Path -LiteralPath $recentsPath) { return (Get-FileHash -LiteralPath $recentsPath -Algorithm SHA256).Hash }
        return 'absent'
    }
    function Invoke-403Native {
        param([string]$Exe, [string[]]$NativeArgs, [string]$LogPath)
        # Windows PowerShell 5 represents redirected native stderr as error
        # records. A successful git fetch prints there; inspect the native exit
        # code instead of treating any stderr text as a terminating failure.
        $ErrorActionPreference = 'Continue'
        & $Exe @NativeArgs 2>&1 | Out-File -LiteralPath $LogPath -Encoding utf8
        return $LASTEXITCODE
    }
    function Assert-403Idle {
        param([string]$Label)
        $fetchCode=Invoke-403Native -Exe git -NativeArgs @('-C',$MainRoot,'fetch','origin','main') -LogPath (Join-Path $RunRoot "$Label.fetch.log")
        "git -C `"$MainRoot`" fetch origin main; exit=$fetchCode" | Add-Content (Join-Path $RunRoot 'commands.txt')
        if ($fetchCode -ne 0) { throw '403 resource fetch failed' }
        $resource = @(& git -C $MainRoot show origin/main:coord/resources/PC-RUNTIME-STACK.md)
        if ($LASTEXITCODE -ne 0) { throw '403 resource read failed' }
        $resource | Select-Object -First 5 | Out-File (Join-Path $RunRoot "$Label.resource.txt") -Encoding utf8
        if ($resource[3] -notmatch '403.*owner=Codex `/root/restore403`') { throw '403 resource owner changed' }
        $processes = @(Get-CimInstance Win32_Process | Where-Object { $_.Name -match '^(VitShell|VitAgent|VitApp|VspHub|Godot|godot|win32_noactivate_restore_probe).*\.exe$' })
        $tcp = @(Get-NetTCPConnection -State Listen -ErrorAction SilentlyContinue | Where-Object { $_.LocalPort -in @(5555,5556,7878,7879,8787,9333) })
        $udp = @(Get-NetUDPEndpoint -LocalPort 4445 -ErrorAction SilentlyContinue)
        [ordered]@{ at_utc=(Get-Date).ToUniversalTime().ToString('o'); processes=$processes; tcp=$tcp; udp=$udp } |
            ConvertTo-Json -Depth 5 | Out-File (Join-Path $RunRoot "$Label.idle.json") -Encoding utf8
        if ($processes.Count -or $tcp.Count -or $udp.Count) { throw '403 resource conflict: no processes stopped or reused' }
    }
    $recentsBefore = Get-403RecentsHash
    $recentsBefore | Out-File (Join-Path $RunRoot 'recents.before.sha256') -Encoding utf8
    if ($GodotRestoreForensics -and (Test-Path -LiteralPath $recentsPath)) { Copy-Item -LiteralPath $recentsPath -Destination (Join-Path $RunRoot 'recents.before.json') }
    try {
        foreach ($entry in @(@('main',$MainRoot),@('frontend',$FrontendWorktree))) {
            $prefix=$entry[0]; $tree=$entry[1]
            & git -C $tree rev-parse HEAD | Out-File (Join-Path $RunRoot "$prefix.head.txt") -Encoding utf8
            & git -C $tree status --short | Out-File (Join-Path $RunRoot "$prefix.status.txt") -Encoding utf8
            & git -C $tree diff --stat | Out-File (Join-Path $RunRoot "$prefix.diffstat.txt") -Encoding utf8
            & git -C $tree diff | Out-File (Join-Path $RunRoot "$prefix.diff.txt") -Encoding utf8
        }
        Get-FileHash -Algorithm SHA256 -LiteralPath $PSCommandPath,(Join-Path $FrontendWorktree 'VitShell/tests/CMakeLists.txt'),(Join-Path $FrontendWorktree 'VitShell/tests/win32_noactivate_restore_probe.cpp') |
            ConvertTo-Json | Out-File (Join-Path $RunRoot 'source_hashes.json') -Encoding utf8
        if ($GodotRestoreForensics) {
            if ($ForensicsGodotExe -match '(?i)console' -or -not (Test-Path -LiteralPath $ForensicsGodotExe)) { throw '403B requires the direct existing GUI Godot executable' }
            Get-FileHash -LiteralPath $ForensicsGodotExe -Algorithm SHA256 | ConvertTo-Json | Out-File (Join-Path $RunRoot 'godot_binary_hash.json') -Encoding utf8
            (Get-Item -LiteralPath $ForensicsGodotExe).VersionInfo | Select-Object FileVersion,ProductVersion,OriginalFilename | ConvertTo-Json | Out-File (Join-Path $RunRoot 'godot_version_resource.json') -Encoding utf8
        }
        Assert-403Idle 'before_build'
        $testSource=Join-Path $FrontendWorktree 'VitShell/tests'
        $testBuild=Join-Path $RunRoot 'build'
        "cmake -S `"$testSource`" -B `"$testBuild`" -G `"Visual Studio 18 2026`" -A x64" |
            Out-File (Join-Path $RunRoot 'commands.txt') -Encoding utf8
        $configureExit=Invoke-403Native -Exe cmake -NativeArgs @('-S',$testSource,'-B',$testBuild,'-G','Visual Studio 18 2026','-A','x64') -LogPath (Join-Path $RunRoot 'configure.log')
        "configure_exit=$configureExit" | Add-Content (Join-Path $RunRoot 'commands.txt')
        if ($configureExit -ne 0) { throw '403 configure failed; no scenario started' }
        "cmake --build `"$testBuild`" --config Release" | Add-Content (Join-Path $RunRoot 'commands.txt')
        $buildExit=Invoke-403Native -Exe cmake -NativeArgs @('--build',$testBuild,'--config','Release') -LogPath (Join-Path $RunRoot 'build.log')
        "build_exit=$buildExit" | Add-Content (Join-Path $RunRoot 'commands.txt')
        if ($buildExit -ne 0) { throw '403 build failed; no scenario started' }
        $probeExe=Join-Path $testBuild 'Release/win32_noactivate_restore_probe.exe'
        Get-FileHash -Algorithm SHA256 -LiteralPath $probeExe | ConvertTo-Json |
            Out-File (Join-Path $RunRoot 'binary_hash.json') -Encoding utf8
        $failCount=0
        foreach ($condition in @('synthetic','external')) {
            $variants=@('restore','shownoactivate','placement')
            if ($GodotRestoreForensics) { $variants=@('godot') }
            foreach ($variant in $variants) {
                if (((Get-Date)-$probeStarted).TotalMinutes -ge 30) { throw '403 30-minute stop line' }
                if ($failCount -ge 2) { throw '403 two candidate failure stop line; no replacement runs' }
                $scene="$condition.$variant"
                if ($GodotRestoreForensics) {
                    $project=Join-Path $RunRoot "$scene.project"
                    $null=New-Item -ItemType Directory -Path (Join-Path $project isolated_user) -Force
                    $nonce=[guid]::NewGuid().ToString('N')
                    $engineObservation=Join-Path $project 'observation.jsonl'
                    $projectText=@'
config_version=5
[application]
config/name="403 isolated Godot observation"
run/main_scene="res://main.tscn"
[display]
window/size/viewport_width=720
window/size/viewport_height=480
window/size/mode=1
window/size/always_on_top=false
window/size/no_focus=false
[rendering]
renderer/rendering_method="gl_compatibility"
renderer/rendering_method.mobile="gl_compatibility"
'@
                    [System.IO.File]::WriteAllText((Join-Path $project project.godot),$projectText,(New-Object System.Text.UTF8Encoding($false)))
                    $sceneText=@'
[gd_scene load_steps=2 format=3]
[ext_resource type="Script" path="res://observer.gd" id="1"]
[node name="Observer403" type="Node"]
script = ExtResource("1")
'@
                    [System.IO.File]::WriteAllText((Join-Path $project main.tscn),$sceneText,(New-Object System.Text.UTF8Encoding($false)))
                    $observationPath=$engineObservation.Replace('\','/')
                    $observerText=@"
extends Node
var output: FileAccess
var sequence: int = 0
var ready_seen: bool = false
var elapsed: float = 0.0
func emit_observation(kind: String) -> void:
	if output == null:
		return
	sequence += 1
	var record = {"kind":kind,"seq":sequence,"nonce":"$nonce","pid":OS.get_process_id(),"unix_ms":int(Time.get_unix_time_from_system()*1000.0),"ticks_msec":Time.get_ticks_msec(),"ready":int(ready_seen),"window_mode":get_window().mode,"display_mode":DisplayServer.window_get_mode(),"focused":int(get_window().has_focus()),"hwnd":DisplayServer.window_get_native_handle(DisplayServer.WINDOW_HANDLE),"width":get_window().size.x,"height":get_window().size.y,"version":Engine.get_version_info()}
	output.store_line(JSON.stringify(record))
	output.flush()
func _ready() -> void:
	output = FileAccess.open("$observationPath",FileAccess.WRITE)
	get_window().focus_entered.connect(func(): emit_observation("focus_entered"))
	get_window().focus_exited.connect(func(): emit_observation("focus_exited"))
	get_window().size_changed.connect(func(): emit_observation("size_changed"))
	emit_observation("scene_ready")
func _process(delta: float) -> void:
	if not ready_seen:
		ready_seen = true
		emit_observation("first_process_ready")
	elapsed += delta
	if elapsed >= 0.02:
		elapsed = 0.0
		emit_observation("sample")
"@
                    [System.IO.File]::WriteAllText((Join-Path $project observer.gd),$observerText,(New-Object System.Text.UTF8Encoding($false)))
                    [ordered]@{nonce=$nonce; project=$project; command=('"'+$ForensicsGodotExe+'" --path "'+$project+'" --log-file "'+$project+'/godot_engine.log"'); startup_flags=1; startup_show=0; creation_flags='CREATE_SUSPENDED|CREATE_NO_WINDOW|CREATE_UNICODE_ENVIRONMENT'; user_environment='Child-only APPDATA/LOCALAPPDATA/USERPROFILE/TEMP/TMP redirected to run project isolated_user; VIT_* removed'; no_autoload=$true; no_plugins=$true; no_extensions=$true} | ConvertTo-Json | Out-File (Join-Path $RunRoot "$scene.project_manifest.json") -Encoding utf8
                }
                Assert-403Idle "before_$scene"
                $raw=Join-Path $RunRoot "$scene.jsonl"
                if (-not $GodotRestoreForensics) { "`"$probeExe`" $variant $condition `"$raw`" (Start-Process -WindowStyle Hidden; new process)" |
                    Add-Content (Join-Path $RunRoot 'commands.txt') }
                $probeArgs=@($variant,$condition,('"'+$raw+'"'))
                if ($GodotRestoreForensics) { $probeArgs=@('--godot',$condition,('"'+$raw+'"'),('"'+$ForensicsGodotExe+'"'),('"'+$project+'"'),('"'+$engineObservation+'"'),$nonce); ('"'+$probeExe+'" '+($probeArgs -join ' ')) | Add-Content (Join-Path $RunRoot commands.txt) }
                $probeProcess=Start-Process -FilePath $probeExe -ArgumentList $probeArgs -WindowStyle Hidden -PassThru
                $ownedPID=$probeProcess.Id
                $null=$probeProcess.Handle # retain the exact created process handle before waiting
                [ordered]@{ pid=$ownedPID; started_utc=$probeProcess.StartTime.ToUniversalTime().ToString('o'); exe=$probeExe; variant=$variant; condition=$condition } |
                    ConvertTo-Json | Out-File (Join-Path $RunRoot "$scene.process.json") -Encoding utf8
                $probeTimeout=15000
                if ($GodotRestoreForensics) { $probeTimeout=20000 }
                if (-not $probeProcess.WaitForExit($probeTimeout)) { throw "403 timeout in owned process $ownedPID" }
                $code=$probeProcess.ExitCode
                $probeProcess.Dispose()
                $probeProcess=$null
                "scenario=$scene pid=$ownedPID exit=$code" | Add-Content (Join-Path $RunRoot 'commands.txt')
                $records=@(Get-Content -LiteralPath $raw | ForEach-Object { $_ | ConvertFrom-Json })
                $result=@($records | Where-Object { $_.kind -eq 'result' })
                if ($result.Count -ne 1 -or $records[-1].kind -ne 'hook_removed') { throw "403 incomplete raw interval/teardown in $scene" }
                $probeResults += [ordered]@{ scene=$scene; pid=$ownedPID; exit=$code; classification=$result[0].classification; raw=$raw }
                if ($GodotRestoreForensics -and $code -notin @(0,10)) { throw "403B first anomaly in $scene; remaining scene stopped, no rerun" }
                if ($code -eq 40 -and $variant -ne 'restore') { ++$failCount }
                if ($code -in @(20,30,50) -or ($variant -eq 'restore' -and $code -notin @(0,10))) { throw "403 observation/probe/environment failure in $scene; no rerun" }
            }
        }
        $probeExit=0
        if (@($probeResults | Where-Object { $_.exit -eq 40 }).Count) { $probeExit=1 }
        elseif (@($probeResults | Where-Object { $_.exit -eq 10 }).Count) { $probeExit=2 }
    } catch {
        $_ | Out-String | Out-File (Join-Path $RunRoot 'failure.txt') -Encoding utf8
        $probeExit=1
    } finally {
        # Never enumerate-and-kill: the only process eligible is our exact handle.
        if ($null -ne $probeProcess -and -not $probeProcess.HasExited) { $probeProcess.Kill(); $probeProcess.WaitForExit() }
        $recentsAfter=Get-403RecentsHash
        $recentsAfter | Out-File (Join-Path $RunRoot 'recents.after.sha256') -Encoding utf8
        if ($recentsAfter -ne $recentsBefore) { $probeExit=1; 'Unexpected recents change: no restore/write attempted (no kernel launched)' | Out-File (Join-Path $RunRoot 'recents.failure.txt') }
        try { Assert-403Idle 'after_teardown' } catch { $probeExit=1; $_ | Out-String | Out-File (Join-Path $RunRoot 'teardown.failure.txt') }
        $boundary='Owned synthetic Win32 windows only; no VitShell/Godot/product conclusions'
        if ($GodotRestoreForensics) { $boundary='Isolated minimal Godot only; raw child WinEvent plus engine observations, no child WndProc or product acceptance' }
        [ordered]@{ card='403'; run_id=(Split-Path -Leaf $RunRoot); started=$probeStarted.ToUniversalTime().ToString('o'); finished=(Get-Date).ToUniversalTime().ToString('o'); exit=$probeExit; recents_unchanged=($recentsBefore -eq $recentsAfter); scenarios=$probeResults; boundary=$boundary } |
            ConvertTo-Json -Depth 8 | Out-File (Join-Path $RunRoot 'report.json') -Encoding utf8
    }
    exit $probeExit
}

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
