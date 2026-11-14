# vitshell_open_chain_smoke.ps1 - SHELL-2 acceptance smoke (card pool #357).
#
# Full-journey real-stack leg: VitShell (WebView2 main window) -> React launcher
# -> drawer open button -> host native file-dialog (smoke bypass) -> governance
# (agent/state; page-level 3-key card on the governed leg) -> agent open chain
# -> Godot direct-launch (VIT_SHELL_* contract, no start_page render) ->
# launch-result -> vit_surface project; close DAW -> vit_surface launcher
# (same window, no reload); second open under stale project state -> governance
# card -> switch; final close -> whole tree teardown (job containment).
#
# PASS = exit 0. Artifacts under coord/runs/SHELL-2/run_<stamp>/.
# Disciplines: S2.2 stack occupancy + <=60s recheck; S10 recents snapshot and
# restore (%APPDATA%/Vit-DAW/global_project_config.json); repo fixtures are
# copied into the run dir (source hash before/after); no LLM billing.

param(
    [string]$FrontendWorktree = "D:/Godot/project/vit-daw-frontend_wt_shell2",
    [string]$Webview2SdkDir = "D:/Godot/project/vit-daw-frontend/extension/third_party/webview2",
    [string]$GodotExe = "D:/Godot/Godot_v4.6.1-stable_win64_console.exe",
    [string]$FixtureSource = "D:/Vit_DAW/temp/f3_empty_project.vit",
    [int]$StatusPort = 7879,
    [int]$AgentPort = 7878,
    [int]$CdpPort = 9333,
    [int]$AgentReadyTimeoutSec = 120,
    [int]$OpenChainTimeoutSec = 240,
    [string]$RunRoot = "",
    [switch]$SkipBuild
)

$ErrorActionPreference = "Stop"
$MainRoot = Split-Path -Parent $PSScriptRoot

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
    foreach ($n in $Names) { $count += @(Get-Process -Name $n -ErrorAction SilentlyContinue).Count }
    return $count
}

function Invoke-NativeLogged {
    # PS5.1: under $ErrorActionPreference=Stop, native stderr lines become
    # terminating NativeCommandErrors. Native tools legitimately write stderr
    # (cmake/go/node/godot warnings) - run them with Continue locally, keep the
    # exit code honest.
    param([scriptblock]$Command, [string]$LogPath)
    $prev = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    try {
        & $Command 2>&1 | Out-File -FilePath $LogPath -Encoding utf8
        return $LASTEXITCODE
    } finally {
        $ErrorActionPreference = $prev
    }
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

function Get-HttpPost {
    param([string]$Url, [int]$TimeoutSec = 5)
    try {
        $resp = Invoke-WebRequest -Uri $Url -Method POST -UseBasicParsing -TimeoutSec $TimeoutSec -ContentType "application/json" -Body "{}"
        return @{ ok = $true; status = [int]$resp.StatusCode; body = $resp.Content }
    } catch {
        return @{ ok = $false; status = 0; body = $_.Exception.Message }
    }
}

function Wait-StatusField {
    param([string]$StatusUrl, [string]$Field, [string]$Expect, [int]$TimeoutSec, [string]$ArtifactPath)
    $deadline = (Get-Date).AddSeconds($TimeoutSec)
    $last = $null
    while ((Get-Date) -lt $deadline) {
        Start-Sleep -Milliseconds 500
        $r = Get-HttpJson -Url $StatusUrl -TimeoutSec 3
        if ($r.ok) {
            try { $last = $r.body | ConvertFrom-Json } catch { $last = $null }
            if ($null -ne $last) {
                $value = [string]$last.$Field
                if ($value -eq $Expect) {
                    if ($ArtifactPath) { $r.body | Out-File -FilePath $ArtifactPath -Encoding utf8 }
                    return $last
                }
                if ($Field -eq "phase" -and $value -eq "error") {
                    throw ("VitShell phase=error: " + $last.error)
                }
            }
        }
    }
    throw ("timeout waiting status.$Field=$Expect (last=" + $(if ($null -ne $last) { $last.$Field } else { "unreachable" }) + ")")
}

function Wait-StatusLaunchCount {
    param([string]$StatusUrl, [int]$MinExpect, [int]$TimeoutSec, [string]$ArtifactPath)
    # chain_state stays "opened" after the first chain; the SECOND open must be
    # awaited via the monotonic launch counter, not a state equality.
    $deadline = (Get-Date).AddSeconds($TimeoutSec)
    $last = $null
    while ((Get-Date) -lt $deadline) {
        Start-Sleep -Milliseconds 500
        $r = Get-HttpJson -Url $StatusUrl -TimeoutSec 3
        if ($r.ok) {
            try { $last = $r.body | ConvertFrom-Json } catch { $last = $null }
            if ($null -ne $last -and [int]$last.launch_count -ge $MinExpect) {
                if ($ArtifactPath) { $r.body | Out-File -FilePath $ArtifactPath -Encoding utf8 }
                return $last
            }
        }
    }
    throw ("timeout waiting launch_count>=$MinExpect (last=" + $(if ($null -ne $last) { $last.launch_count } else { "unreachable" }) + ")")
}

function Invoke-CdpPhase {
    param([string]$Phase, [string]$OutFile)
    $webuiDir = Join-Path $MainRoot "agent/webui"
    $WebuiDir = $webuiDir
    $out = Join-Path $RunRoot $OutFile
    pushd $webuiDir
    $code = Invoke-NativeLogged -Command { node "$MainRoot/scripts/vitshell_open_chain_cdp.mjs" --cdp-port $CdpPort --phase $Phase --out $out --webui-dir "$WebuiDir" } -LogPath ($out + ".log")
    popd
    if ($code -ne 0) { throw ("CDP phase $Phase failed (exit $code, see $OutFile.log)") }
    Write-Ok "CDP $Phase pass"
    return (Get-Content $out -Raw -Encoding UTF8 | ConvertFrom-Json)
}

$VitProcessNames = @("VitShell", "VitAgent", "VitApp", "VspHub", "Godot", "godot")

# ---------- preflight (S2.2: no reuse of a live stack) ----------

Write-Step "Preflight: stack idle + occupancy re-read"
$preProcs = Get-VitProcessCount -Names $VitProcessNames
if ($preProcs -ne 0) { throw "preflight: Vit-family processes already running (count=$preProcs) - no silent reuse" }
foreach ($port in @($AgentPort, $StatusPort)) {
    $owner = Get-TcpListenerPid -Port $port
    if ($null -ne $owner) { throw "preflight: port $port already listening (pid=$owner) - no silent reuse" }
}
$occupancyHead = (Get-Content (Join-Path $MainRoot "coord/resources/PC-RUNTIME-STACK.md") -TotalCount 6) -join "`n"
if ($occupancyHead -notmatch "SHELL-2") { throw "occupancy record head does not name SHELL-2 - register before running" }
Write-Ok "stack clean (processes=0, ports free), occupancy names SHELL-2"

# ---------- artifacts scaffold + S10 snapshots ----------

if ([string]::IsNullOrWhiteSpace($RunRoot)) {
    $stamp = Get-Date -Format "yyyyMMdd_HHmmss"
    $RunRoot = Join-Path $MainRoot "coord/runs/SHELL-2/run_$stamp"
}
New-Item -ItemType Directory -Path $RunRoot -Force | Out-Null
$fixturesDir = Join-Path $RunRoot "fixtures"
New-Item -ItemType Directory -Path $fixturesDir -Force | Out-Null
Write-Step "Artifacts: $RunRoot"

$runMeta = [ordered]@{
    run_id = (Split-Path -Leaf $RunRoot)
    started_at_utc = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
    card = "SHELL-2"
    main_worktree = $MainRoot
    frontend_worktree = $FrontendWorktree
    godot_exe = $GodotExe
    status_port = $StatusPort
    agent_port = $AgentPort
    cdp_port = $CdpPort
    skip_build = [bool]$SkipBuild
}

# recents snapshot (S10: kernel recents live in %APPDATA%/Vit-DAW).
$recentsFile = Join-Path $env:APPDATA "Vit-DAW/global_project_config.json"
$recentsSnap = Join-Path $RunRoot "recents_snapshot.json"
if (Test-Path $recentsFile) {
    Copy-Item $recentsFile $recentsSnap -Force
    $runMeta.recents_snapshot_sha256 = (Get-FileHash $recentsSnap -Algorithm SHA256).Hash
    Write-Ok ("recents snapshot taken (" + $runMeta.recents_snapshot_sha256.Substring(0, 16) + "...)")
} else {
    $runMeta.recents_snapshot = "absent-before-run"
    Write-Ok "recents file absent before run (fresh face)"
}

# worktree runtime-state snapshot (kernel writes live in the worktree Workspace).
$runtimeSnaps = @{}
foreach ($rel in @("VitApp/Workspace/Settings/Settings.xml", "VitApp/Workspace/default_project.xml")) {
    $src = Join-Path $MainRoot $rel
    if (Test-Path $src) {
        $dst = Join-Path $RunRoot ("snap_" + ($rel -replace "/", "_"))
        Copy-Item $src $dst -Force
        $runtimeSnaps[$rel] = $dst
    }
}

# fixtures: copy repo fixture into the isolated run dir (S10: repo samples untouched).
if (-not (Test-Path $FixtureSource)) { throw "fixture source missing: $FixtureSource" }
$fixtureSourceHash = (Get-FileHash $FixtureSource -Algorithm SHA256).Hash
$runMeta.fixture_source = $FixtureSource
$runMeta.fixture_source_sha256 = $fixtureSourceHash
$fixtureA = Join-Path $fixturesDir "fixture_a.vit"
$fixtureB = Join-Path $fixturesDir "fixture_b.vit"
Copy-Item $FixtureSource $fixtureA -Force
Copy-Item $FixtureSource $fixtureB -Force
Write-Ok ("fixtures copied (source sha256=" + $fixtureSourceHash.Substring(0, 16) + "...)")
$runMeta.fixture_a = $fixtureA
$runMeta.fixture_b = $fixtureB

pushd $MainRoot
git rev-parse HEAD 2>$null | Out-File -FilePath (Join-Path $RunRoot "main_head.txt") -Encoding utf8
git status --short 2>$null | Out-File -FilePath (Join-Path $RunRoot "main_status.txt") -Encoding utf8
popd
pushd $FrontendWorktree
git rev-parse HEAD 2>$null | Out-File -FilePath (Join-Path $RunRoot "frontend_head.txt") -Encoding utf8
git status --short 2>$null | Out-File -FilePath (Join-Path $RunRoot "frontend_status.txt") -Encoding utf8
popd

# ---------- build (VitShell / agent / webui dist) ----------

$VitShellExe = Join-Path $FrontendWorktree "VitShell/build/Release/VitShell.exe"
if (-not $SkipBuild) {
    Write-Step "Build VitShell.exe (cmake VS x64 Release)"
    pushd $FrontendWorktree
    $code = Invoke-NativeLogged -Command { cmake -S VitShell -B VitShell/build -G "Visual Studio 18 2026" -A x64 "-DVITSHELL_WEBVIEW2_DIR=$Webview2SdkDir" } -LogPath (Join-Path $RunRoot "vitshell_cmake_configure.log")
    if ($code -ne 0) { popd; throw "cmake configure failed" }
    $code = Invoke-NativeLogged -Command { cmake --build VitShell/build --config Release } -LogPath (Join-Path $RunRoot "vitshell_cmake_build.log")
    popd
    if ($code -ne 0) { throw "cmake build failed" }
    Write-Ok "VitShell build exit 0"
} else {
    if (-not (Test-Path $VitShellExe)) { throw "-SkipBuild: VitShell.exe missing" }
    Write-Ok "-SkipBuild: reusing VitShell.exe"
}
$runMeta.vitshell_exe = $VitShellExe
$runMeta.vitshell_exe_sha256 = (Get-FileHash $VitShellExe -Algorithm SHA256).Hash

$AgentExe = Join-Path $MainRoot "agent/bin/VitAgent.exe"
if (-not $SkipBuild) {
    Write-Step "Build isolated VitAgent.exe (worktree source)"
    pushd (Join-Path $MainRoot "agent")
    $code = Invoke-NativeLogged -Command { go build -o ".\bin\VitAgent.exe" .\cmd\vitagent } -LogPath (Join-Path $RunRoot "agent_build.log")
    popd
    if ($code -ne 0) { throw "go build agent failed" }
    Write-Ok "agent build exit 0"
} else {
    if (-not (Test-Path $AgentExe)) { throw "-SkipBuild: agent binary missing" }
}
$runMeta.agent_exe = $AgentExe
$runMeta.agent_exe_sha256 = (Get-FileHash $AgentExe -Algorithm SHA256).Hash

# webui dist: always build fresh (React face under test IS the deliverable).
Write-Step "webui dist build (worktree source: SHELL-2 launcher face)"
$runWebui = Join-Path $MainRoot "agent/webui"
pushd $runWebui
$code = Invoke-NativeLogged -Command { npm run build } -LogPath (Join-Path $RunRoot "webui_build.log")
popd
if ($code -ne 0) { throw "webui build failed" }
$runMeta.webui_dist_index_sha256 = (Get-FileHash (Join-Path $runWebui "dist/index.html") -Algorithm SHA256).Hash
Write-Ok "webui build exit 0"

# kernel/hub: borrow the main-checkout binaries (C++ zero diff in this card;
# G3-ATTRIB-1 reuse precedent) into the ISOLATED worktree paths so all kernel
# writes stay inside the worktree workspace.
Write-Step "Kernel/hub binaries into isolated worktree paths (main-checkout build, zero C++ diff)"
$mainKernel = "D:/Vit_DAW/VitApp/build/VitApp_artefacts/Release/VitApp.exe"
$mainHub = "D:/Vit_DAW/agent/bin/VspHub.exe"
foreach ($pair in @(@($mainKernel, (Join-Path $MainRoot "VitApp/build/VitApp_artefacts/Release/VitApp.exe")),
                    @($mainHub, (Join-Path $MainRoot "agent/bin/VspHub.exe")))) {
    if (-not (Test-Path $pair[0])) { throw "borrowed binary missing: $($pair[0])" }
    New-Item -ItemType Directory -Path (Split-Path -Parent $pair[1]) -Force | Out-Null
    Copy-Item $pair[0] $pair[1] -Force
}
$runMeta.kernel_exe = Join-Path $MainRoot "VitApp/build/VitApp_artefacts/Release/VitApp.exe"
$runMeta.kernel_exe_sha256 = (Get-FileHash $mainKernel -Algorithm SHA256).Hash
$runMeta.hub_exe = Join-Path $MainRoot "agent/bin/VspHub.exe"
$runMeta.hub_exe_sha256 = (Get-FileHash $mainHub -Algorithm SHA256).Hash
Write-Ok ("kernel/hub staged (sha256 " + $runMeta.kernel_exe_sha256.Substring(0, 12) + ".../" + $runMeta.hub_exe_sha256.Substring(0, 12) + "...)")

# Godot project import (worktree needs .godot caches; idempotent).
Write-Step "Godot --import pre-run (worktree project caches)"
if (-not $SkipBuild -or -not (Test-Path (Join-Path $FrontendWorktree ".godot"))) {
    pushd $FrontendWorktree
    $code = Invoke-NativeLogged -Command { & $GodotExe --headless --import --path . } -LogPath (Join-Path $RunRoot "godot_import.log")
    popd
    if ($code -ne 0) { throw "godot import failed (see godot_import.log)" }
}
Write-Ok "godot import ok"

# ---------- immediate recheck (S2.2 <=60s) ----------

Write-Step "Immediate recheck (<=60s before stack start)"
$reProcs = Get-VitProcessCount -Names $VitProcessNames
if ($reProcs -ne 0) { throw "recheck: Vit-family processes appeared (count=$reProcs)" }
foreach ($port in @($AgentPort, $StatusPort)) {
    if ($null -ne (Get-TcpListenerPid -Port $port)) { throw "recheck: port $port got occupied" }
}
Write-Ok "clean"

# ---------- launch VitShell ----------

$LauncherUrl = "http://127.0.0.1:$AgentPort/app/?surface=launcher"
$vitLog = Join-Path $RunRoot "vitshell_stderr.log"
$bridgeLog = Join-Path $RunRoot "bridge.log"

Write-Step "Launch VitShell (main window + agent tree + CDP debug port)"
$env:WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS = "--remote-debugging-port=$CdpPort"
$proc = Start-Process -FilePath $VitShellExe `
    -ArgumentList @(
        "--agent-exe", $AgentExe,
        "--dev-root", $MainRoot,
        "--agent-url", "http://127.0.0.1:$AgentPort",
        "--agent-http-addr", "127.0.0.1:$AgentPort",
        "--url", $LauncherUrl,
        "--status-port", "$StatusPort",
        "--godot-exe", $GodotExe,
        "--godot-project-dir", $FrontendWorktree,
        "--smoke-file-dialog", $fixtureA,
        "--smoke-file-dialog-b", $fixtureB,
        "--bridge-log", $bridgeLog
    ) `
    -RedirectStandardError $vitLog -PassThru -WindowStyle Hidden
Remove-Item Env:WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS
$runMeta.vitshell_pid = $proc.Id
Write-Ok ("VitShell pid=" + $proc.Id)

$StatusUrl = "http://127.0.0.1:$StatusPort/status"

try {
    # ---- L2: ready + launcher DOM + drawer buttons ----
    Write-Step "L2: phase=ready + launcher surface DOM (drawer buttons)"
    Wait-StatusField -StatusUrl $StatusUrl -Field "phase" -Expect "ready" -TimeoutSec $AgentReadyTimeoutSec -ArtifactPath (Join-Path $RunRoot "status_ready.json")
    $cdp = Invoke-CdpPhase -Phase "probe-launcher" -OutFile "cdp_probe_launcher.json"
    if ($cdp.drawer_new -ne "+新建" -and $cdp.drawer_new -notmatch "新建") { throw "drawer new button text unexpected: $($cdp.drawer_new)" }
    if ($cdp.sentinel -le 0) { throw "no-reload sentinel not set" }
    $runMeta.no_reload_sentinel = $cdp.sentinel

    # ---- L3: first open (zero friction expected) -> direct-launch -> project surface ----
    Write-Step "L3: drawer open -> choose_file bypass (fixture A) -> agent open chain -> Godot direct-launch"
    $null = Invoke-CdpPhase -Phase "click-open" -OutFile "cdp_click_open_1.json"
    $status = Wait-StatusLaunchCount -StatusUrl $StatusUrl -MinExpect 1 -TimeoutSec $OpenChainTimeoutSec -ArtifactPath (Join-Path $RunRoot "status_opened_1.json")
    if (-not $status.godot_running) { throw "godot not running after first open" }
    if ([int]$status.launch_count -ne 1) { throw "launch_count expected 1, got $($status.launch_count)" }
    $runMeta.godot_pid_1 = $status.godot_pid
    Write-Ok ("first open: godot pid=" + $status.godot_pid + " chain_state=opened")
    $govCount1 = (Select-String -Path $bridgeLog -Pattern "governance.required" -ErrorAction SilentlyContinue | Measure-Object).Count
    $runMeta.first_open_governance = if ($govCount1 -gt 0) { "required" } else { "none" }
    if ($govCount1 -gt 0) {
        Write-Bad "governance appeared on FIRST open (stale persisted state) - driving switch via CDP (recorded, not zero-friction)"
        $null = Invoke-CdpPhase -Phase "click-governance-switch" -OutFile "cdp_gov_click_1.json"
        $status = Wait-StatusLaunchCount -StatusUrl $StatusUrl -MinExpect 1 -TimeoutSec $OpenChainTimeoutSec -ArtifactPath (Join-Path $RunRoot "status_opened_1b.json")
    }

    $null = Invoke-CdpPhase -Phase "probe-project" -OutFile "cdp_probe_project.json"
    $projProbe = Get-Content (Join-Path $RunRoot "cdp_probe_project.json") -Raw -Encoding UTF8 | ConvertFrom-Json
    if ($projProbe.sentinel -ne $runMeta.no_reload_sentinel) { throw "no-reload sentinel changed on project surface (reload!)" }

    # ---- L4: direct-launch quality ----
    Write-Step "L4: direct-launch assertions (agent state / hub / kernel / godot log)"
    $state = Get-HttpJson -Url "http://127.0.0.1:$AgentPort/agent/state" -TimeoutSec 5
    if (-not $state.ok) { throw "/agent/state unreachable" }
    $stateJson = $state.body | ConvertFrom-Json
    $agentProjectPath = [string]$stateJson.current_project.path
    $runMeta.agent_current_project_after_open1 = $agentProjectPath
    if ($agentProjectPath -notmatch "fixture_a") { throw "/agent/state current_project expected fixture_a, got: $agentProjectPath" }
    Write-Ok "/agent/state current_project=fixture_a (version.project_opened arrived)"

    if ($null -eq (Get-TcpListenerPid -Port 8787)) { throw "VspHub 8787 not listening (kernel chain not booted)" }
    Write-Ok "VspHub 8787 listening"
    $godotLog = Join-Path $env:APPDATA "Godot/app_userdata/Vit_DAW_Frontend/logs/godot.log"
    if (Test-Path $godotLog) {
        Copy-Item $godotLog (Join-Path $RunRoot "godot_user_log.txt") -Force
        $logText = Get-Content $godotLog -Raw -ErrorAction SilentlyContinue
        if ($logText -notmatch "\[VitShellLaunch\] direct.begin") { throw "godot log missing direct.begin marker" }
        if ($logText -notmatch "\[VitShellLaunch\] direct.enter_ok") { throw "godot log missing direct.enter_ok marker" }
        if ($logText -notmatch "VitDockRoot") { throw "godot log does not show dock scene entry" }
        Write-Ok "godot user log: direct.begin + direct.enter_ok + VitDockRoot"
    } else {
        Write-Bad "godot user log not found (markers unassertable) - recording boundary"
        $runMeta.godot_log = "missing"
    }

    # ---- L5: close DAW -> back to launcher (same window, no reload) ----
    Write-Step "L5: POST /vitshell/close-daw -> Godot exits -> React back to launcher"
    $closeDaw = Get-HttpPost -Url "http://127.0.0.1:$StatusPort/vitshell/close-daw" -TimeoutSec 5
    if (-not $closeDaw.ok) { throw "POST close-daw failed: $($closeDaw.body)" }
    Wait-StatusField -StatusUrl $StatusUrl -Field "godot_running" -Expect "False" -TimeoutSec 30 -ArtifactPath (Join-Path $RunRoot "status_daw_closed.json")
    $null = Invoke-CdpPhase -Phase "probe-launcher-back" -OutFile "cdp_probe_launcher_back.json"
    $back = Get-Content (Join-Path $RunRoot "cdp_probe_launcher_back.json") -Raw -Encoding UTF8 | ConvertFrom-Json
    if ($back.sentinel -ne $runMeta.no_reload_sentinel) { throw "sentinel changed after back-to-launcher (reload!)" }
    Write-Ok "DAW closed; launcher surface back in the SAME window (sentinel intact)"

    # ---- L6: second open under stale project state -> governance card -> switch ----
    Write-Step "L6: second open -> page governance card (3 keys) -> switch -> direct-launch fixture B"
    # The bypass takes fixture B for this (second) chain (VitShell
    # --smoke-file-dialog-b): open chain runs against a different project file.
    $null = Invoke-CdpPhase -Phase "click-open" -OutFile "cdp_click_open_2.json"
    $null = Invoke-CdpPhase -Phase "assert-governance" -OutFile "cdp_assert_governance.json"
    $gov = Get-Content (Join-Path $RunRoot "cdp_assert_governance.json") -Raw -Encoding UTF8 | ConvertFrom-Json
    if ($gov.save -ne "保存并切换" -or $gov.switch -ne "直接切换" -or $gov.cancel -ne "取消") {
        throw ("governance keys unexpected: save=$($gov.save) switch=$($gov.switch) cancel=$($gov.cancel)")
    }
    $null = Invoke-CdpPhase -Phase "click-governance-switch" -OutFile "cdp_gov_click_2.json"
    $status2 = Wait-StatusLaunchCount -StatusUrl $StatusUrl -MinExpect 2 -TimeoutSec $OpenChainTimeoutSec -ArtifactPath (Join-Path $RunRoot "status_opened_2.json")
    $null = Invoke-CdpPhase -Phase "probe-governance-done" -OutFile "cdp_gov_done.json"
    $govDone = Get-Content (Join-Path $RunRoot "cdp_gov_done.json") -Raw -Encoding UTF8 | ConvertFrom-Json
    if ($govDone.sentinel -ne $runMeta.no_reload_sentinel) { throw "sentinel changed after governed open (reload!)" }
    Write-Ok ("governed open done: launch_count=" + $status2.launch_count + " godot pid=" + $status2.godot_pid)

    # ---- L7: close main window -> whole tree teardown ----
    Write-Step "L7: POST /close -> VitShell + agent tree + Godot tree (kernel/hub) all gone"
    $agentPid = [int]$status.agent_pid
    $null = Get-HttpPost -Url "http://127.0.0.1:$StatusPort/close" -TimeoutSec 5
    $deadline = (Get-Date).AddSeconds(20)
    while ((Get-Date) -lt $deadline) {
        Start-Sleep -Milliseconds 400
        if ($null -eq (Get-Process -Id $proc.Id -ErrorAction SilentlyContinue)) { break }
    }
    if ($null -ne (Get-Process -Id $proc.Id -ErrorAction SilentlyContinue)) { throw "VitShell still alive 20s after close" }
    Write-Ok "VitShell exited"
    foreach ($name in @("VitShell", "VitAgent", "VitApp", "VspHub", "Godot", "godot")) {
        Get-Process -Name $name -ErrorAction SilentlyContinue | Wait-Process -Timeout 10 -ErrorAction SilentlyContinue
    }
    Start-Sleep -Seconds 2
    $finalProcs = Get-VitProcessCount -Names $VitProcessNames
    if ($finalProcs -ne 0) {
        Get-Process -Name $VitProcessNames -ErrorAction SilentlyContinue |
            Select-Object Name, Id | Out-File -FilePath (Join-Path $RunRoot "residue_processes.txt") -Encoding utf8
        throw "Vit-family processes remain after close (count=$finalProcs, see residue_processes.txt)"
    }
    Write-Ok "zero Vit-family processes (incl. Godot/kernel/hub)"
    foreach ($port in @($AgentPort, $StatusPort)) {
        $deadline2 = (Get-Date).AddSeconds(10)
        $freed = $false
        while ((Get-Date) -lt $deadline2) {
            if ($null -eq (Get-TcpListenerPid -Port $port)) { $freed = $true; break }
            Start-Sleep -Milliseconds 400
        }
        if (-not $freed) { throw "port $port still listening after teardown" }
        Write-Ok "port $port released"
    }

    # ---- S10: recents restore + fixture source integrity ----
    if (Test-Path $recentsSnap) {
        Copy-Item $recentsSnap $recentsFile -Force
        Write-Ok "recents restored from snapshot (S10)"
    }
    $postFixtureHash = (Get-FileHash $FixtureSource -Algorithm SHA256).Hash
    if ($postFixtureHash -ne $fixtureSourceHash) { throw "fixture source hash changed (pollution!)" }
    Write-Ok "fixture source untouched (hash consistent)"

    $runMeta.result = "PASS"
} catch {
    $runMeta.result = "FAIL"
    $runMeta.error = $_.Exception.Message
    $runMeta | ConvertTo-Json -Depth 5 | Out-File -FilePath (Join-Path $RunRoot "meta.json") -Encoding utf8
    Write-Bad ("FAIL: " + $_.Exception.Message + " - best-effort teardown")
    try { $null = Get-HttpPost -Url "http://127.0.0.1:$StatusPort/close" -TimeoutSec 3 } catch {}
    Start-Sleep -Seconds 3
    foreach ($n in @("VitShell", "VitAgent", "VitApp", "VspHub", "Godot", "godot")) {
        Get-Process -Name $n -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
    }
    if (Test-Path $recentsSnap) { Copy-Item $recentsSnap $recentsFile -Force }
    $runMeta.teardown = "best-effort executed (verify manually; S2.2 residue rules apply)"
    $runMeta | ConvertTo-Json -Depth 5 | Out-File -FilePath (Join-Path $RunRoot "meta.json") -Encoding utf8
    throw
}

# S10 runtime-state restore in the worktree (kernel writes during the run).
foreach ($rel in $runtimeSnaps.Keys) {
    Copy-Item $runtimeSnaps[$rel] (Join-Path $MainRoot $rel) -Force
}

$runMeta.finished_at_utc = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
$runMeta | ConvertTo-Json -Depth 5 | Out-File -FilePath (Join-Path $RunRoot "meta.json") -Encoding utf8

Write-Host ""
Write-Host ("PASS: vitshell_open_chain_smoke " + $runMeta.run_id + " (exit 0)") -ForegroundColor Green
Write-Host ("artifacts: " + $RunRoot)
exit 0
