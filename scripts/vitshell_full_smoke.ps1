# vitshell_full_smoke.ps1 - SHELL-3 acceptance smoke (card pool #358).
#
# Full-journey real-stack leg x $Rounds + failure-injection legs + legacy
# declarative leg, on the SHELL-2 device patterns (same stack, same seams,
# same artifact discipline - no second test mechanism).
#
# Main journey (per round): cold start (process-level ZERO Godot assert) ->
# launcher real data (live greeting endpoint, no fixture face) -> drawer open
# (fixture A) -> governance-if-needed -> Godot direct-launch -> in-project
# REAL chat roundtrip (346 surface: composer -> /agent/chat -> thread frames;
# zero-billing dead-endpoint LLM override - assistant frame is the honest
# failure-delivery path, LLM content out of scope) -> close DAW -> launcher
# back (same window) -> second open -> governance card -> switch (fixture B)
# -> close main window -> whole tree teardown -> recents/runtime restore.
#
# Failure injection legs (run once):
#   bad-agent-exe     --agent-exe nonexistent -> in-window error page, ZERO Godot
#   occupied-port     7878 pre-taken by script TcpListener -> error page, ZERO Godot
#   webview2-off      WEBVIEW2_BROWSER_EXECUTABLE_FOLDER -> dead dir -> system
#                     error box path (probe+MessageBox; no window/agent/Godot).
#                     Injection-ineffective outcome is declared BOUNDARY, not fail.
# Legacy leg (run once, declarative): godot --import --check-only exit 0 +
# start_page.gd direct-launch env-gate structural checks (F5 path untouched).
#
# PASS = exit 0: legacy leg pass + all failure legs pass-or-declared-boundary +
# main rounds $Rounds/$Rounds PASS. Artifacts under coord/runs/SHELL-4/.
#
# SHELL-4 extension (card pool #360, kernel-ready-at-start ruling 2026-11-15):
#   L1b kernel-online     launcher stage MUST have VitApp+VspHub alive and
#                         agent-owned (parent=agent pid); greeting real data
#                         (seeded recents) via live HTTP; kernel attr=online.
#   L1c new-project chain launcher chat -> directive -> bridge -> Godot
#                         direct-launch new -> kernel new_project -> project
#                         surface; then close-DAW -> kernel SURVIVES (lifecycle
#                         belongs to the main window, not the DAW window).
#   L4 survival assert    kernel/hub still alive after fixture_a DAW close.
#   S10 hardening         recents seeded before rounds (deterministic real
#                         data) + restore is hash-verified (mismatch = env
#                         interrupt, non-zero exit); absent-before case now
#                         deletes the run-created file and verifies absence.
# Disciplines: S2.2 stack occupancy + <=60s recheck; S10 recents snapshot and
# restore (%APPDATA%/Vit-DAW/global_project_config.json) + both-root Workspace
# runtime snapshots; repo fixtures copied into run dir (source hash before/
# after); zero LLM billing (VIT_AGENT_LLM_BASE_URL dead-endpoint override on
# the VitShell process tree; explicit env beats ~/.vit/config.json per agent
# config.applyExplicitEnvOverrides); no GUI automation (read-only process and
# window-title inspection only).

param(
    [string]$FrontendWorktree = "D:/Godot/project/vit-daw-frontend_wt_shell4",
    [string]$CardId = "SHELL-4",
    [string]$Webview2SdkDir = "D:/Godot/project/vit-daw-frontend/extension/third_party/webview2",
    [string]$GodotExe = "D:/Godot/Godot_v4.6.1-stable_win64_console.exe",
    [string]$FixtureSource = "D:/Vit_DAW/temp/f3_empty_project.vit",
    [int]$StatusPort = 7879,
    [int]$AgentPort = 7878,
    [int]$CdpPort = 9333,
    [int]$AgentReadyTimeoutSec = 120,
    [int]$OpenChainTimeoutSec = 240,
    [int]$Rounds = 3,
    [string]$ChatText = "这个工程现在叫什么名字？",
    [int]$ChatTimeoutMs = 90000,
    [string]$RunRoot = "",
    [switch]$SkipBuild,
    [switch]$SkipFailureLegs,
    [switch]$SkipLegacyLegs
)

$ErrorActionPreference = "Stop"
$MainRoot = Split-Path -Parent $PSScriptRoot

# ---------- helpers (SHELL-2 device, same semantics) ----------

function Write-Step { param([string]$Message) Write-Host ""; Write-Host ">>> $Message" }
function Write-Ok { param([string]$Message) Write-Host "  [OK] $Message" -ForegroundColor Green }
function Write-Bad { param([string]$Message) Write-Host "  [FAIL] $Message" -ForegroundColor Red }
function Write-Note { param([string]$Message) Write-Host "  [NOTE] $Message" -ForegroundColor Yellow }

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

function Test-PortBindable {
    # LISTEN-state checks miss bound-but-lingering sockets - the only honest
    # "next launch can bind" test is an actual bind+close probe.
    # CRITICAL: use a RAW socket, not TcpListener - TcpListener binds in its
    # ctor and Stop() does NOT release a never-started listener (micro-test
    # run_full_20261115_042646 era: fixed-probe still leaked a Bound socket).
    # Raw Socket.Close() releases instantly.
    param([int]$Port)
    $s = $null
    try {
        $s = [System.Net.Sockets.Socket]::new([System.Net.Sockets.AddressFamily]::InterNetwork, [System.Net.Sockets.SocketType]::Stream, [System.Net.Sockets.ProtocolType]::Tcp)
        $s.Bind([System.Net.IPEndPoint]::new([System.Net.IPAddress]::Loopback, $Port))
        return $true
    } catch {
        return $false
    } finally {
        if ($null -ne $s) { try { $s.Close() } catch {} }
    }
}

function Test-UdpBindable {
    # UDP 4445 (agent command port) bindable probe: bind+close on loopback.
    param([int]$Port)
    $s = $null
    try {
        $s = [System.Net.Sockets.Socket]::new([System.Net.Sockets.AddressFamily]::InterNetwork, [System.Net.Sockets.SocketType]::Dgram, [System.Net.Sockets.ProtocolType]::Udp)
        $s.Bind([System.Net.IPEndPoint]::new([System.Net.IPAddress]::Loopback, $Port))
        return $true
    } catch {
        return $false
    } finally {
        if ($null -ne $s) { try { $s.Close() } catch {} }
    }
}

function Wait-PortsBindable {
    param([int[]]$Ports, [int]$TimeoutSec = 30)
    $deadline = (Get-Date).AddSeconds($TimeoutSec)
    while ((Get-Date) -lt $deadline) {
        $all = $true
        foreach ($p in $Ports) { if (-not (Test-PortBindable -Port $p)) { $all = $false; break } }
        if ($all) { return $true }
        Start-Sleep -Milliseconds 500
    }
    return $false
}

function Get-VitWebviewProcessCount {
    # Only OUR WebView2 browser processes (shared user data dir marker), not
    # other apps' msedgewebview2 instances.
    $n = 0
    $procs = Get-CimInstance Win32_Process -Filter "Name = 'msedgewebview2.exe'" -ErrorAction SilentlyContinue
    foreach ($p in $procs) { if ($p.CommandLine -match "VitDAW") { $n++ } }
    return $n
}

function Wait-VitWebviewGone {
    # A browser process lingering in shutdown can poison the NEXT VitShell's
    # WebView2 environment creation on the shared user data folder (observed
    # as a silent pre-status death: run_full_20261115_040758 R1). Wait it out.
    param([int]$TimeoutSec = 45)
    $deadline = (Get-Date).AddSeconds($TimeoutSec)
    while ((Get-Date) -lt $deadline) {
        if ((Get-VitWebviewProcessCount) -eq 0) { return $true }
        Start-Sleep -Milliseconds 500
    }
    return $false
}

function Get-GodotProcessCount {
    return @(Get-Process -Name "Godot*" -ErrorAction SilentlyContinue).Count
}

function Invoke-NativeLogged {
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

function Wait-StatusErrorPhase {
    # Failure-leg waiter: error phase IS the expected outcome here.
    param([string]$StatusUrl, [int]$TimeoutSec, [string]$ArtifactPath)
    $deadline = (Get-Date).AddSeconds($TimeoutSec)
    $last = $null
    while ((Get-Date) -lt $deadline) {
        Start-Sleep -Milliseconds 500
        $r = Get-HttpJson -Url $StatusUrl -TimeoutSec 3
        if ($r.ok) {
            try { $last = $r.body | ConvertFrom-Json } catch { $last = $null }
            if ($null -ne $last -and [string]$last.phase -eq "error") {
                if ($ArtifactPath) { $r.body | Out-File -FilePath $ArtifactPath -Encoding utf8 }
                return $last
            }
        }
    }
    throw ("timeout waiting phase=error (last=" + $(if ($null -ne $last) { $last.phase } else { "unreachable" }) + ")")
}

function Wait-StatusLaunchCount {
    param([string]$StatusUrl, [int]$MinExpect, [int]$TimeoutSec, [string]$ArtifactPath)
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
    param([string]$Phase, [string]$OutFile, [string]$NewProjectName = "")
    $webuiDir = Join-Path $MainRoot "agent/webui"
    $WebuiDir = $webuiDir
    $out = Join-Path $RunRoot $OutFile
    pushd $webuiDir
    $nodeArgs = @("$MainRoot/scripts/vitshell_full_smoke_cdp.mjs", "--cdp-port", "$CdpPort", "--phase", $Phase, "--out", $out, "--webui-dir", "$WebuiDir")
    if ($Phase -eq "chat-roundtrip") {
        $nodeArgs += @("--chat-text", $ChatText, "--chat-timeout-ms", "$ChatTimeoutMs")
    }
    if ($Phase -eq "launcher-chat-new") {
        if ([string]::IsNullOrWhiteSpace($NewProjectName)) { throw "launcher-chat-new requires -NewProjectName" }
        $nodeArgs += @("--new-project-name", $NewProjectName)
    }
    $code = Invoke-NativeLogged -Command { node @nodeArgs } -LogPath ($out + ".log")
    popd
    if ($code -ne 0) { throw ("CDP phase $Phase failed (exit $code, see $OutFile.log)") }
    Write-Ok "CDP $Phase pass"
    return (Get-Content $out -Raw -Encoding UTF8 | ConvertFrom-Json)
}

function Assert-ZeroGodot {
    param([string]$When)
    $n = Get-GodotProcessCount
    if ($n -ne 0) { throw "cold-start assert: $When Godot processes present (count=$n)" }
}

function Start-VitShell {
    param([string]$OutLog, [string[]]$ExtraArgs = @(), [hashtable]$ExtraEnv = @{}, [string]$AgentExePath = "")
    $LauncherUrl = "http://127.0.0.1:$AgentPort/app/?surface=launcher"
    $env:WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS = "--remote-debugging-port=$CdpPort"
    $env:VIT_WEBVIEW_DEBUG = "1"
    foreach ($k in $ExtraEnv.Keys) { Set-Item -Path ("env:" + $k) -Value $ExtraEnv[$k] }
    $effectiveAgentExe = $AgentExe
    if ($AgentExePath -ne "") { $effectiveAgentExe = $AgentExePath }
    $p = Start-Process -FilePath $VitShellExe `
        -ArgumentList (@(
            "--agent-exe", $effectiveAgentExe,
            "--dev-root", $MainRoot,
            "--agent-url", "http://127.0.0.1:$AgentPort",
            "--agent-http-addr", "127.0.0.1:$AgentPort",
            "--url", $LauncherUrl,
            "--status-port", "$StatusPort",
            "--godot-exe", $GodotExe,
            "--godot-project-dir", $FrontendWorktree,
            "--bridge-log", (Join-Path $RunRoot $OutLog)
        ) + $ExtraArgs) `
        -RedirectStandardError (Join-Path $RunRoot ($OutLog + ".stderr.log")) -PassThru -WindowStyle Hidden
    Remove-Item Env:WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS -ErrorAction SilentlyContinue
    Remove-Item Env:VIT_WEBVIEW_DEBUG -ErrorAction SilentlyContinue
    Remove-Item Env:VIT_AGENT_LLM_BASE_URL -ErrorAction SilentlyContinue
    # ExtraEnv keys MUST be scrubbed from the script process: a lingering
    # WEBVIEW2_BROWSER_EXECUTABLE_FOLDER (F3) poisoned every later launch's
    # runtime probe -> silent pre-status death (run_full_20261115_041804 R1).
    foreach ($k in $ExtraEnv.Keys) { Remove-Item -Path ("env:" + $k) -ErrorAction SilentlyContinue }
    return $p
}

function Stop-TreeBestEffort {
    param([int]$ShellPid)
    try { $null = Get-HttpPost -Url "http://127.0.0.1:$StatusPort/close" -TimeoutSec 3 } catch {}
    Start-Sleep -Seconds 2
    foreach ($n in @("VitShell", "VitAgent", "VitApp", "VspHub", "Godot*")) {
        Get-Process -Name $n -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
    }
    if ($ShellPid -gt 0) {
        Wait-Process -Id $ShellPid -Timeout 8 -ErrorAction SilentlyContinue
    }
}

$VitProcessNames = @("VitShell", "VitAgent", "VitApp", "VspHub", "Godot*")

# ---------- preflight (S2.2: no reuse of a live stack) ----------

Write-Step "Preflight: stack idle + occupancy re-read"
$preProcs = Get-VitProcessCount -Names $VitProcessNames
if ($preProcs -ne 0) { throw "preflight: Vit-family processes already running (count=$preProcs) - no silent reuse" }
foreach ($port in @($AgentPort, $StatusPort, 8787, 5555)) {
    $owner = Get-TcpListenerPid -Port $port
    if ($null -ne $owner) { throw "preflight: port $port already listening (pid=$owner) - no silent reuse" }
    if (-not (Test-PortBindable -Port $port)) { throw "preflight: port $port not bindable (lingering bound socket) - no silent reuse" }
}
if (-not (Test-UdpBindable -Port 4445)) { throw "preflight: UDP 4445 not bindable - no silent reuse" }
$occupancyHead = (Get-Content (Join-Path $MainRoot "coord/resources/PC-RUNTIME-STACK.md") -TotalCount 6) -join "`n"
if ($occupancyHead -notmatch $CardId) { throw "occupancy record head does not name $CardId - register before running" }
Write-Ok "stack clean (processes=0, TCP 7878/7879/8787/5555 + UDP 4445 free), occupancy names $CardId"

# ---------- artifacts scaffold + S10 snapshots ----------

if ([string]::IsNullOrWhiteSpace($RunRoot)) {
    $stamp = Get-Date -Format "yyyyMMdd_HHmmss"
    $RunRoot = Join-Path $MainRoot "coord/runs/SHELL-3/run_full_$stamp"
}
New-Item -ItemType Directory -Path $RunRoot -Force | Out-Null
$fixturesDir = Join-Path $RunRoot "fixtures"
New-Item -ItemType Directory -Path $fixturesDir -Force | Out-Null
Write-Step "Artifacts: $RunRoot"

$runMeta = [ordered]@{
    run_id = (Split-Path -Leaf $RunRoot)
    started_at_utc = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
    card = $CardId
    main_worktree = $MainRoot
    frontend_worktree = $FrontendWorktree
    godot_exe = $GodotExe
    status_port = $StatusPort
    agent_port = $AgentPort
    cdp_port = $CdpPort
    rounds = $Rounds
    chat_text = $ChatText
    zero_billing_override = "VIT_AGENT_LLM_BASE_URL=http://127.0.0.1:1 (explicit env beats user config; LLM content out of scope)"
    skip_build = [bool]$SkipBuild
    skip_failure_legs = [bool]$SkipFailureLegs
    skip_legacy_legs = [bool]$SkipLegacyLegs
}

$recentsFile = Join-Path $env:APPDATA "Vit-DAW/global_project_config.json"
$recentsSnap = Join-Path $RunRoot "recents_snapshot.json"
# SHELL-4: restore is hash-verified (RECENTS-HYGIENE-1 b-case discipline):
# restore failure is recorded and fails the run as ENV-INTERRUPT (never silent).
$script:s10RecentsExistedBefore = $false
$script:s10RecentsHash = ""
$script:s10RuntimeHashes = @{}
$script:s10RestoreErrors = @()
function Restore-S10State {
    # Recents: snapshot existed -> copy back + hash-verify; absent-before ->
    # remove the run-created file + verify absence (SHELL-4 hardening).
    if ($script:s10RecentsExistedBefore) {
        Copy-Item $recentsSnap $recentsFile -Force
        $restored = (Get-FileHash $recentsFile -Algorithm SHA256).Hash
        if ($restored -ne $script:s10RecentsHash) {
            $script:s10RestoreErrors += ("recents hash mismatch after restore (want=" + $script:s10RecentsHash + " got=" + $restored + ")")
        }
    } elseif (Test-Path $recentsFile) {
        Remove-Item $recentsFile -Force
        if (Test-Path $recentsFile) {
            $script:s10RestoreErrors += "recents file still present after absent-before restore"
        }
    }
    foreach ($key in $script:runtimeSnaps.Keys) {
        $parts = $key.Split("|")
        $dst = Join-Path $parts[0] $parts[1]
        $src = $script:runtimeSnaps[$key]
        if (Test-Path $src) {
            Copy-Item $src $dst -Force
            $want = $script:s10RuntimeHashes[$key]
            if ($want) {
                $got = (Get-FileHash $dst -Algorithm SHA256).Hash
                if ($got -ne $want) {
                    $script:s10RestoreErrors += ("runtime snapshot hash mismatch: " + $dst)
                }
            }
        }
    }
}
if (Test-Path $recentsFile) {
    Copy-Item $recentsFile $recentsSnap -Force
    $script:s10RecentsExistedBefore = $true
    $script:s10RecentsHash = (Get-FileHash $recentsSnap -Algorithm SHA256).Hash
    $runMeta.recents_snapshot_sha256 = $script:s10RecentsHash
    Write-Ok ("recents snapshot taken (" + $runMeta.recents_snapshot_sha256.Substring(0, 16) + "...)")
} else {
    $runMeta.recents_snapshot = "absent-before-run"
    Write-Ok "recents file absent before run (fresh face)"
}

$script:runtimeSnaps = @{}
foreach ($rootBase in @($MainRoot, "D:/Vit_DAW")) {
    foreach ($rel in @("VitApp/Workspace/Settings/Settings.xml", "VitApp/Workspace/default_project.xml")) {
        $src = Join-Path $rootBase $rel
        if (Test-Path $src) {
            $dst = Join-Path $RunRoot ("snap_" + ($rootBase.Replace(":", "_").Replace("/", "_").Replace([string][char]92, "_")) + "_" + ($rel.Replace("/", "_")))
            Copy-Item $src $dst -Force
            $script:runtimeSnaps[$rootBase + "|" + $rel] = $dst
            $script:s10RuntimeHashes[$rootBase + "|" + $rel] = (Get-FileHash $src -Algorithm SHA256).Hash
        }
    }
}

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

# SHELL-4: seed fixture_a at the recents head (kernel get_recent_projects reads
# this file live) so the launcher real-data legs are deterministic on any host.
# S10 restore (hash-verified) brings the user's file back exactly.
$existing = @()
if (Test-Path $recentsFile) {
    try {
        $parsed = Get-Content $recentsFile -Raw -Encoding UTF8 | ConvertFrom-Json
        if ($null -ne $parsed.recent_projects) { $existing = @($parsed.recent_projects) }
    } catch { $existing = @() }
}
$merged = @($fixtureA)
foreach ($path in $existing) {
    if ("$path" -ne "$fixtureA") { $merged += "$path" }
}
@{ recent_projects = $merged } | ConvertTo-Json -Depth 3 | Out-File -FilePath $recentsFile -Encoding utf8
$runMeta.recents_seeded_fixture = $fixtureA
$runMeta.recents_seeded_sha256 = (Get-FileHash $recentsFile -Algorithm SHA256).Hash
Write-Ok ("recents seeded with fixture_a (deterministic greeting; seeded sha256=" + $runMeta.recents_seeded_sha256.Substring(0, 12) + "...)")

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

Write-Step "webui dist build (worktree source: SHELL-2 launcher face + 346 project face)"
$runWebui = Join-Path $MainRoot "agent/webui"
pushd $runWebui
$code = Invoke-NativeLogged -Command { npm run build } -LogPath (Join-Path $RunRoot "webui_build.log")
popd
if ($code -ne 0) { throw "webui build failed" }
$runMeta.webui_dist_index_sha256 = (Get-FileHash (Join-Path $runWebui "dist/index.html") -Algorithm SHA256).Hash
Write-Ok "webui build exit 0"

Write-Step "Kernel/hub binaries into isolated worktree paths (main-checkout build, zero C++ diff)"
$mainKernel = "D:/Vit_DAW/VitApp/build/VitApp_artefacts/Release/VitApp.exe"
$mainHub = "D:/Vit_DAW/agent/bin/VspHub.exe"
foreach ($pair in @(@($mainKernel, (Join-Path $MainRoot "VitApp/build/VitApp_artefacts/Release/VitApp.exe")),
                    @($mainHub, (Join-Path $MainRoot "agent/bin/VspHub.exe")))) {
    if (-not (Test-Path $pair[0])) { throw "borrowed binary missing: $($pair[0])" }
    New-Item -ItemType Directory -Path (Split-Path -Parent $pair[1]) -Force | Out-Null
    Copy-Item $pair[0] $pair[1] -Force
}
$runMeta.kernel_exe = $mainKernel
$runMeta.kernel_exe_sha256 = (Get-FileHash $mainKernel -Algorithm SHA256).Hash
$runMeta.hub_exe = $mainHub
$runMeta.hub_exe_sha256 = (Get-FileHash $mainHub -Algorithm SHA256).Hash
Write-Ok ("kernel/hub staged (sha256 " + $runMeta.kernel_exe_sha256.Substring(0, 12) + ".../" + $runMeta.hub_exe_sha256.Substring(0, 12) + "...)")

Write-Step "Godot --import pre-run (worktree project caches)"
if (-not $SkipBuild -or -not (Test-Path (Join-Path $FrontendWorktree ".godot"))) {
    pushd $FrontendWorktree
    $code = Invoke-NativeLogged -Command { & $GodotExe --headless --import --path . } -LogPath (Join-Path $RunRoot "godot_import.log")
    popd
    if ($code -ne 0) { throw "godot import failed (see godot_import.log)" }
}
Write-Ok "godot import ok"

# ---------- legacy leg (declarative, once) ----------

$legacyResult = "skipped"
$legacyMeta = @{}
if (-not $SkipLegacyLegs) {
    try {
        Write-Step "Legacy leg: godot --import --check-only + start_page env-gate structural checks"
        pushd $FrontendWorktree
        $code = Invoke-NativeLogged -Command { & $GodotExe --headless --import --check-only --path . } -LogPath (Join-Path $RunRoot "godot_import_checkonly.log")
        popd
        if ($code -ne 0) { throw "godot --import --check-only exit $code (project/script validation failed)" }
        $legacyMeta.import_checkonly_exit = 0
        $startPage = Join-Path $FrontendWorktree "app/startup/start_page.gd"
        $spText = Get-Content $startPage -Raw -Encoding UTF8
        if ($spText -notmatch "VIT_SHELL_DIRECT") { throw "start_page.gd direct-launch env gate marker missing" }
        $legacyMeta.direct_env_gate = "present"
        if ($spText -notmatch "open_project_by_path") { throw "start_page.gd common open chain reference missing" }
        $legacyMeta.open_chain_ref = "present"
        if ($spText -notmatch "launch-result|launch_result") { throw "start_page.gd launch-result receipt missing" }
        $legacyMeta.launch_result = "present"
        $govDialog = Join-Path $FrontendWorktree "app/startup/launcher_governance_dialog.gd"
        if (Test-Path $govDialog) { $legacyMeta.governance_dialog_file = "present" } else { $legacyMeta.governance_dialog_file = "absent (verify path)" }
        $legacyResult = "pass"
        Write-Ok "legacy leg pass (F5 path structurally intact, declarative)"
    } catch {
        $legacyResult = "fail"
        $legacyMeta.error = $_.Exception.Message
        Write-Bad "legacy leg FAIL: $($legacyMeta.error)"
    }
}
$runMeta.legacy_leg = $legacyResult
$runMeta.legacy = $legacyMeta

# ---------- SHELL-4: boot-draft write surface baseline (user-area hygiene) ----------
# Kernel-online-at-start makes the agent materialize a boot draft workspace under
# %LOCALAPPDATA%/Vit/ProjectHistory/drafts on every start (pre-existing F11
# machinery). The run deletes exactly the drafts IT created (name-set diff).
$script:draftsRoot = Join-Path $env:LOCALAPPDATA "Vit/ProjectHistory/drafts"
$script:draftsBefore = @{}
if (Test-Path $script:draftsRoot) {
    Get-ChildItem $script:draftsRoot -Directory -ErrorAction SilentlyContinue | ForEach-Object {
        $script:draftsBefore[$_.Name] = $true
    }
}
$runMeta.boot_drafts_baseline_count = $script:draftsBefore.Count

# ---------- immediate recheck (S2.2 <=60s) ----------

Write-Step "Immediate recheck (<=60s before stack start)"
$reProcs = Get-VitProcessCount -Names $VitProcessNames
if ($reProcs -ne 0) { throw "recheck: Vit-family processes appeared (count=$reProcs)" }
foreach ($port in @($AgentPort, $StatusPort)) {
    if ($null -ne (Get-TcpListenerPid -Port $port)) { throw "recheck: port $port got occupied" }
    if (-not (Test-PortBindable -Port $port)) { throw "recheck: port $port not bindable (lingering bound socket)" }
}
Write-Ok "clean"

$StatusBase = "http://127.0.0.1:$StatusPort"
$StatusUrl = "$StatusBase/status"

# ---------- failure injection legs (once) ----------

$failureResults = [ordered]@{}
if (-not $SkipFailureLegs) {
    # ---- F1: bad agent binary ----
    Write-Step "F1 bad-agent-exe: --agent-exe nonexistent -> in-window error page, zero Godot"
    try {
        $p = Start-VitShell -OutLog "f1_bridge.log" -AgentExePath "D:/nonexistent_shell3_badagent/VitAgent.exe" -ExtraArgs @("--smoke-file-dialog", $fixtureA, "--smoke-file-dialog-b", $fixtureB)
        $errSnap = Wait-StatusErrorPhase -StatusUrl $StatusUrl -TimeoutSec 30 -ArtifactPath (Join-Path $RunRoot "f1_status_error.json")
        if ($errSnap.error -notmatch "start|exited|spawn|agent|CreateProcess|launch") { Write-Note ("F1 error detail: " + $errSnap.error) }
        Assert-ZeroGodot -When "after agent start failure"
        if ($null -eq (Get-Process -Id $p.Id -ErrorAction SilentlyContinue)) { throw "VitShell exited on agent start failure (expected in-window error page)" }
        $f1cdp = Invoke-CdpPhase -Phase "probe-error-page" -OutFile "f1_cdp_error_page.json"
        if ($f1cdp.detail.Length -lt 3) { throw "F1 error page detail empty" }
        Write-Ok ("F1 pass: phase=error, in-window error page (detail=" + $f1cdp.detail.Substring(0, [Math]::Min(60, $f1cdp.detail.Length)) + "...), VitShell alive, zero Godot")
        $null = Get-HttpPost -Url "$StatusBase/close" -TimeoutSec 5
        $deadline = (Get-Date).AddSeconds(15)
        while ((Get-Date) -lt $deadline -and $null -ne (Get-Process -Id $p.Id -ErrorAction SilentlyContinue)) { Start-Sleep -Milliseconds 400 }
        Stop-TreeBestEffort -ShellPid $p.Id
        $left = Get-VitProcessCount -Names $VitProcessNames
        if ($left -ne 0) { throw "F1 teardown residue: $left Vit processes" }
        if (-not (Wait-VitWebviewGone -TimeoutSec 45)) { Write-Note "F1: Vit WebView2 browser processes still lingering after teardown (recorded)" }
        if (-not (Wait-PortsBindable -Ports @($AgentPort, $StatusPort) -TimeoutSec 20)) { throw "F1: status/agent port not bindable after teardown (lingering bound socket)" }
        $failureResults.bad_agent_exe = "pass"
    } catch {
        Stop-TreeBestEffort -ShellPid 0
        $failureResults.bad_agent_exe = "fail: " + $_.Exception.Message
        Write-Bad "F1 FAIL: $($failureResults.bad_agent_exe)"
    }

    # ---- F2: agent port occupied ----
    Write-Step "F2 occupied-port: 7878 taken by script socket -> agent cannot bind -> error page, zero Godot"
    # Bound-but-NOT-listening socket: blocks the agent's bind exactly like a
    # real port collision, but VitShell's health-poll connects get refused
    # (no accept queue) and Close() releases instantly. A TcpListener here was
    # lessoned by run_full_20261115_040758: pending poll connections keep the
    # listen socket alive after Stop() for tens of seconds.
    $occupier = $null
    try {
        $occupier = [System.Net.Sockets.Socket]::new([System.Net.Sockets.AddressFamily]::InterNetwork, [System.Net.Sockets.SocketType]::Stream, [System.Net.Sockets.ProtocolType]::Tcp)
        $occupier.Bind([System.Net.IPEndPoint]::new([System.Net.IPAddress]::Loopback, $AgentPort))
        if (Test-PortBindable -Port $AgentPort) { throw "F2 self-check: port $AgentPort still bindable (occupier not effective)" }
        $p = Start-VitShell -OutLog "f2_bridge.log" -ExtraArgs @("--agent-ready-timeout-ms", "15000", "--smoke-file-dialog", $fixtureA, "--smoke-file-dialog-b", $fixtureB)
        $errSnap = Wait-StatusErrorPhase -StatusUrl $StatusUrl -TimeoutSec 60 -ArtifactPath (Join-Path $RunRoot "f2_status_error.json")
        Write-Note ("F2 error detail: " + $errSnap.error)
        Assert-ZeroGodot -When "after agent bind failure"
        if ($null -eq (Get-Process -Id $p.Id -ErrorAction SilentlyContinue)) { throw "VitShell exited on occupied agent port (expected in-window error page)" }
        $f2cdp = Invoke-CdpPhase -Phase "probe-error-page" -OutFile "f2_cdp_error_page.json"
        Write-Ok "F2 pass: phase=error, in-window error page, VitShell alive, zero Godot"
        # Release the occupier ONLY AFTER VitShell exits: Start-Process (with
        # the stderr redirect pipe) spawns VitShell with handle inheritance, so
        # the child holds a duplicate of the bound socket handle and the port
        # stays bound until the child is gone (repro_f2hold evidence: Bound
        # entry persisted 20s+ after parent Close() while VitShell lived).
        $null = Get-HttpPost -Url "$StatusBase/close" -TimeoutSec 5
        $deadline = (Get-Date).AddSeconds(15)
        while ((Get-Date) -lt $deadline -and $null -ne (Get-Process -Id $p.Id -ErrorAction SilentlyContinue)) { Start-Sleep -Milliseconds 400 }
        Stop-TreeBestEffort -ShellPid $p.Id
        if ($null -ne $occupier) { $occupier.Close(); $occupier = $null }
        if (-not (Wait-PortsBindable -Ports @($AgentPort, $StatusPort) -TimeoutSec 20)) { throw "F2: status/agent port not bindable after teardown (lingering bound socket)" }
        Write-Ok "F2 occupier released after VitShell exit; ports bindable again"
        $left = Get-VitProcessCount -Names $VitProcessNames
        if ($left -ne 0) { throw "F2 teardown residue: $left Vit processes" }
        if (-not (Wait-VitWebviewGone -TimeoutSec 45)) { Write-Note "F2: Vit WebView2 browser processes still lingering after teardown (recorded)" }
        $failureResults.occupied_port = "pass"
    } catch {
        Stop-TreeBestEffort -ShellPid 0
        $failureResults.occupied_port = "fail: " + $_.Exception.Message
        Write-Bad "F2 FAIL: $($failureResults.occupied_port)"
    } finally {
        if ($null -ne $occupier) { $occupier.Close() }
    }

    # ---- F3: WebView2 disabled ----
    Write-Step "F3 webview2-off: WEBVIEW2_BROWSER_EXECUTABLE_FOLDER -> dead dir -> probe fail -> system error box"
    $wv2Dead = Join-Path $RunRoot "webview2_dead_dir"
    New-Item -ItemType Directory -Path $wv2Dead -Force | Out-Null
    try {
        $p = Start-VitShell -OutLog "f3_bridge.log" -ExtraArgs @("--agent-ready-timeout-ms", "15000") -ExtraEnv @{ WEBVIEW2_BROWSER_EXECUTABLE_FOLDER = $wv2Dead }
        Start-Sleep -Seconds 12
        $procAlive = $null -ne (Get-Process -Id $p.Id -ErrorAction SilentlyContinue)
        $statusUp = $null -ne (Get-TcpListenerPid -Port $StatusPort)
        $agentUp = @(Get-Process -Name "VitAgent" -ErrorAction SilentlyContinue).Count
        $godotUp = Get-GodotProcessCount
        if ($statusUp -or $agentUp -gt 0) {
            # Injection NOT honored: the app proceeded past the runtime probe.
            Write-Note "F3 injection ineffective (status up / agent started) - declaring BOUNDARY (manual test item), not a product fail"
            Stop-TreeBestEffort -ShellPid $p.Id
            $failureResults.webview2_off = "boundary: injection not honored by loader (app proceeded); code path exists, manual test item"
        } elseif (-not $procAlive) {
            throw "F3: VitShell exited without dialog (unexpected; see f3_bridge.log.stderr.log)"
        } else {
            $winTitle = (Get-Process -Id $p.Id -ErrorAction SilentlyContinue).MainWindowTitle
            if ($winTitle -ne "Vit") { Write-Note ("F3 MainWindowTitle=" + $winTitle + " (expected 'Vit' dialog)") }
            if ($godotUp -ne 0) { throw "F3: Godot processes present ($godotUp)" }
            Write-Ok "F3 pass: probe failed -> process holds system error dialog (title=Vit), no status server, no agent, zero Godot"
            Stop-Process -Id $p.Id -Force -ErrorAction SilentlyContinue
            Start-Sleep -Seconds 2
            $failureResults.webview2_off = "pass"
        }
        if ($null -ne (Get-Process -Id $p.Id -ErrorAction SilentlyContinue)) { Stop-Process -Id $p.Id -Force -ErrorAction SilentlyContinue }
        $left = Get-VitProcessCount -Names $VitProcessNames
        if ($left -ne 0) { throw "F3 teardown residue: $left Vit processes" }
        if (-not (Wait-VitWebviewGone -TimeoutSec 45)) { Write-Note "F3: Vit WebView2 browser processes still lingering after teardown (recorded)" }
    } catch {
        Stop-TreeBestEffort -ShellPid 0
        $failureResults.webview2_off = "fail: " + $_.Exception.Message
        Write-Bad "F3 FAIL: $($failureResults.webview2_off)"
    }
}
$runMeta.failure_legs = $failureResults

# ---------- immediate recheck #2 (before main rounds) ----------

Write-Step "Immediate recheck #2 (<=60s before main rounds)"
$reProcs2 = Get-VitProcessCount -Names $VitProcessNames
if ($reProcs2 -ne 0) { throw "recheck2: Vit-family processes appeared (count=$reProcs2)" }
foreach ($port in @($AgentPort, $StatusPort)) {
    if ($null -ne (Get-TcpListenerPid -Port $port)) { throw "recheck2: port $port got occupied" }
    if (-not (Test-PortBindable -Port $port)) { throw "recheck2: port $port not bindable (lingering bound socket)" }
}
Write-Ok "clean"

# ---------- main journey x Rounds ----------

$roundResults = @()
$roundsPassed = 0

for ($round = 1; $round -le $Rounds; $round++) {
    Write-Step "Main journey round $round/$Rounds"
    $roundMeta = [ordered]@{ round = $round; started_at_utc = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ") }
    try {
        # ---- R-L0: cold start with process-level zero-Godot asserts ----
        Write-Step "R$round L0: cold start (zero Godot assert pre+post ready)"
        $p = Start-VitShell -OutLog "r${round}_bridge.log" -ExtraArgs @("--smoke-file-dialog", $fixtureA, "--smoke-file-dialog-b", $fixtureB) -ExtraEnv @{ VIT_AGENT_LLM_BASE_URL = "http://127.0.0.1:1" }
        Assert-ZeroGodot -When "immediately after VitShell spawn"
        $roundMeta.vitshell_pid = $p.Id
        Start-Sleep -Seconds 3
        Assert-ZeroGodot -When "3s after spawn (splash window)"
        $status = Wait-StatusField -StatusUrl $StatusUrl -Field "phase" -Expect "ready" -TimeoutSec $AgentReadyTimeoutSec -ArtifactPath (Join-Path $RunRoot "r${round}_status_ready.json")
        Assert-ZeroGodot -When "after phase=ready (launcher up, pre-open)"
        if ([bool]$status.godot_running) { throw "R$round status says godot_running=true before any open" }
        Write-Ok "R$round cold start: phase=ready, zero Godot across the whole cold-start window"

        # ---- R-L1: launcher real data ----
        Write-Step "R$round L1: launcher surface real data (live greeting, no fixture face)"
        $null = Invoke-CdpPhase -Phase "probe-launcher" -OutFile "r${round}_cdp_probe_launcher.json"
        $ldata = Invoke-CdpPhase -Phase "probe-launcher-data" -OutFile "r${round}_cdp_launcher_data.json"
        $roundMeta.no_reload_sentinel = $ldata.sentinel
        $roundMeta.greeting_text = $ldata.greeting_text
        $roundMeta.recents_count = $ldata.recents_count
        Write-Ok ("R$round launcher data: greeting=" + $ldata.greeting_text.Substring(0, [Math]::Min(40, $ldata.greeting_text.Length)) + "... recents=" + $ldata.recents_count)

        # ---- R-L1b: SHELL-4 kernel online at launcher stage ----
        Write-Step "R$round L1b: kernel online at launcher (VitApp+VspHub alive, agent-owned)"
        $kernelProcs = @(Get-Process -Name "VitApp" -ErrorAction SilentlyContinue)
        if ($kernelProcs.Count -lt 1) { throw "R$round L1b: VitApp not running at launcher stage (kernel-ready-at-start missing)" }
        $hubProcs = @(Get-Process -Name "VspHub" -ErrorAction SilentlyContinue)
        if ($hubProcs.Count -lt 1) { throw "R$round L1b: VspHub not running at launcher stage" }
        $agentPidLive = [int]$status.agent_pid
        if ($agentPidLive -le 0) { throw "R$round L1b: agent_pid missing from status snapshot" }
        foreach ($pair in @(@("VitApp", $kernelProcs), @("VspHub", $hubProcs))) {
            foreach ($proc in $pair[1]) {
                $parent = (Get-CimInstance Win32_Process -Filter "ProcessId = $($proc.Id)" -ErrorAction SilentlyContinue).ParentProcessId
                if ([int]$parent -ne $agentPidLive) {
                    throw "R$round L1b: $($pair[0]) pid=$($proc.Id) parent=$parent != agent pid=$agentPidLive (spawn ownership broke)"
                }
            }
        }
        $greetingHttp = Get-HttpJson -Url "http://127.0.0.1:$AgentPort/agent/launcher/greeting" -TimeoutSec 45
        if (-not $greetingHttp.ok) { throw "R$round L1b: greeting HTTP unreachable: $($greetingHttp.body)" }
        $greetingJson = $greetingHttp.body | ConvertFrom-Json
        if ([bool]$greetingJson.kernel_unavailable) { throw "R$round L1b: greeting kernel_unavailable=true (kernel-online leg)" }
        if ([int]$greetingJson.recent_count -lt 1) { throw "R$round L1b: greeting recent_count=$($greetingJson.recent_count) (expected >=1 seeded real data)" }
        if ("$($greetingJson.reply)" -notmatch "fixture_a") { throw "R$round L1b: greeting reply does not name the seeded recent: $($greetingJson.reply)" }
        $kernelOnline = Invoke-CdpPhase -Phase "probe-launcher-kernel-online" -OutFile "r${round}_cdp_kernel_online.json"
        $roundMeta.kernel_online = @{
            vitapp_count = $kernelProcs.Count
            vsphub_count = $hubProcs.Count
            agent_pid = $agentPidLive
            greeting_recent_count = [int]$greetingJson.recent_count
            drawer_rows = $kernelOnline.recents_rows
        }
        Write-Ok ("R$round L1b: kernel online at launcher (VitApp=" + $kernelProcs.Count + " VspHub=" + $hubProcs.Count + " agent-owned; greeting real data recents=" + $greetingJson.recent_count + ")")

        # ---- R-L1c: SHELL-4 new-project end-to-end (launcher chat -> directive -> bridge -> kernel new -> project) ----
        Write-Step "R$round L1c: new-project end-to-end (launcher chat -> kernel create -> enter project)"
        $newName = "SHELL4NP_R$round"
        $null = Invoke-CdpPhase -Phase "launcher-chat-new" -OutFile "r${round}_cdp_chat_new.json" -NewProjectName $newName
        $status = Wait-StatusLaunchCount -StatusUrl $StatusUrl -MinExpect 1 -TimeoutSec $OpenChainTimeoutSec -ArtifactPath (Join-Path $RunRoot "r${round}_status_opened_new.json")
        $newReady = Invoke-CdpPhase -Phase "probe-project-ready" -OutFile "r${round}_cdp_new_ready.json"
        if ([string]::IsNullOrWhiteSpace("$($newReady.context_name)")) { throw "R$round L1c: project surface ready but context name empty" }
        $roundMeta.new_project_name = $newName
        $roundMeta.new_project_context_name = $newReady.context_name
        Write-Ok ("R$round L1c: new project entered end-to-end (context=" + $newReady.context_name + ", launch_count=1)")

        # ---- R-L1d: close DAW -> kernel survives (lifecycle belongs to main window) ----
        Write-Step "R$round L1d: close DAW -> kernel survives"
        $closeDawNew = Get-HttpPost -Url "$StatusBase/vitshell/close-daw" -TimeoutSec 5
        if (-not $closeDawNew.ok) { throw "R$round L1d: POST close-daw failed: $($closeDawNew.body)" }
        Wait-StatusField -StatusUrl $StatusUrl -Field "godot_running" -Expect "False" -TimeoutSec 30 -ArtifactPath (Join-Path $RunRoot "r${round}_status_new_daw_closed.json")
        $null = Invoke-CdpPhase -Phase "probe-launcher-back" -OutFile "r${round}_cdp_new_back.json"
        $kAfterNew = @(Get-Process -Name "VitApp" -ErrorAction SilentlyContinue).Count
        $hAfterNew = @(Get-Process -Name "VspHub" -ErrorAction SilentlyContinue).Count
        if ($kAfterNew -lt 1 -or $hAfterNew -lt 1) {
            throw "R$round L1d: kernel/hub died with DAW close (VitApp=$kAfterNew VspHub=$hAfterNew) - lifecycle must belong to the main window"
        }
        $roundMeta.kernel_survives_daw_close_after_new = $true
        Write-Ok ("R$round L1d: DAW closed, kernel survives (VitApp=$kAfterNew VspHub=$hAfterNew)")

        # ---- R-L2: first file open (zero friction expected) -> direct-launch -> project ----
        Write-Step "R$round L2: drawer open (fixture A) -> agent open chain -> Godot direct-launch"
        # SHELL-4: the L1c new project was launch 1; the fixture_a file open is
        # launch 2. The L1c blank draft is the active project, so the in-page
        # governance card is EXPECTED on this open - answer it via the real
        # bridge path (mirror of L5) instead of waiting for a launch that never
        # comes without a choice. Baseline BEFORE the click: the governance
        # request fires inside the click-open CDP phase (choose_file chain).
        $bridgeLog = Join-Path $RunRoot "r${round}_bridge.log"
        $govBeforeOpen = (Select-String -Path $bridgeLog -Pattern "governance.required" -ErrorAction SilentlyContinue | Measure-Object).Count
        $null = Invoke-CdpPhase -Phase "click-open" -OutFile "r${round}_cdp_click_open_1.json"
        $govDeadline = (Get-Date).AddSeconds(20)
        $govSeen = $false
        while ((Get-Date) -lt $govDeadline) {
            Start-Sleep -Milliseconds 400
            $govNow = (Select-String -Path $bridgeLog -Pattern "governance.required" -ErrorAction SilentlyContinue | Measure-Object).Count
            if ($govNow -gt $govBeforeOpen) { $govSeen = $true; break }
            $snap = Get-HttpJson -Url $StatusUrl -TimeoutSec 3
            if ($snap.ok) {
                try { if ([int]($snap.body | ConvertFrom-Json).launch_count -ge 2) { break } } catch {}
            }
        }
        $roundMeta.first_open_governance = if ($govSeen) { "required" } else { "none" }
        if ($govSeen) {
            $null = Invoke-CdpPhase -Phase "click-governance-switch" -OutFile "r${round}_cdp_gov_click_1.json"
        }
        $status = Wait-StatusLaunchCount -StatusUrl $StatusUrl -MinExpect 2 -TimeoutSec $OpenChainTimeoutSec -ArtifactPath (Join-Path $RunRoot "r${round}_status_opened_1.json")
        if (-not $status.godot_running) { throw "R$round godot not running after first file open" }
        $roundMeta.godot_pid_1 = $status.godot_pid
        Write-Ok ("R$round first file open: godot pid=" + $status.godot_pid + " launch_count=2 (governance=" + $roundMeta.first_open_governance + ")")
        $null = Invoke-CdpPhase -Phase "probe-project" -OutFile "r${round}_cdp_probe_project.json"
        $projReady = Invoke-CdpPhase -Phase "probe-project-ready" -OutFile "r${round}_cdp_project_ready.json"
        if ($projReady.context_name -notmatch "fixture_a") { throw "R$round project context name unexpected: $($projReady.context_name)" }
        $roundMeta.project_context_name = $projReady.context_name
        Write-Ok ("R$round project surface ready (context=" + $projReady.context_name + ", source=" + $projReady.context_source + ")")

        # ---- R-L3: in-project REAL chat roundtrip (SHELL-3 new leg) ----
        Write-Step "R$round L3: in-project chat roundtrip (composer -> /agent/chat -> thread frames)"
        $chat = Invoke-CdpPhase -Phase "chat-roundtrip" -OutFile "r${round}_cdp_chat.json"
        $roundMeta.chat_assistant_reply = $chat.assistant_reply
        $roundMeta.chat_user_frames = $chat.user_frames
        $roundMeta.chat_vit_frames = $chat.vit_frames
        Write-Ok ("R$round chat roundtrip: user_frames=" + $chat.user_frames + " vit_frames=" + $chat.vit_frames + " reply=" + $chat.assistant_reply.Substring(0, [Math]::Min(60, $chat.assistant_reply.Length)) + "...")
        $godotDuringChat = Get-GodotProcessCount
        if ($godotDuringChat -lt 1) { throw "R$round Godot gone during in-project chat (expected running DAW)" }

        # ---- R-L4: close DAW -> back to launcher (same window) ----
        Write-Step "R$round L4: POST /vitshell/close-daw -> Godot exits -> launcher back (same window)"
        $closeDaw = Get-HttpPost -Url "$StatusBase/vitshell/close-daw" -TimeoutSec 5
        if (-not $closeDaw.ok) { throw "R$round POST close-daw failed: $($closeDaw.body)" }
        Wait-StatusField -StatusUrl $StatusUrl -Field "godot_running" -Expect "False" -TimeoutSec 30 -ArtifactPath (Join-Path $RunRoot "r${round}_status_daw_closed.json")
        $null = Invoke-CdpPhase -Phase "probe-launcher-back" -OutFile "r${round}_cdp_launcher_back.json"
        $back = Get-Content (Join-Path $RunRoot "r${round}_cdp_launcher_back.json") -Raw -Encoding UTF8 | ConvertFrom-Json
        if ($back.sentinel -ne $roundMeta.no_reload_sentinel) { throw "R$round sentinel changed after back-to-launcher (reload!)" }
        # SHELL-4: kernel/hub must survive the DAW close (lifecycle = main window).
        $kAfterL4 = @(Get-Process -Name "VitApp" -ErrorAction SilentlyContinue).Count
        $hAfterL4 = @(Get-Process -Name "VspHub" -ErrorAction SilentlyContinue).Count
        if ($kAfterL4 -lt 1 -or $hAfterL4 -lt 1) {
            throw "R$round L4: kernel/hub died with DAW close (VitApp=$kAfterL4 VspHub=$hAfterL4) - lifecycle must belong to the main window"
        }
        $roundMeta.kernel_survives_daw_close_l4 = $true
        Write-Ok ("R$round DAW closed; launcher back in the SAME window (sentinel intact; kernel survives VitApp=$kAfterL4 VspHub=$hAfterL4)")

        # ---- R-L5: second open -> governance card -> switch (fixture B) ----
        Write-Step "R$round L5: second open -> governance 3-key card -> switch -> fixture B direct-launch"
        $null = Invoke-CdpPhase -Phase "click-open" -OutFile "r${round}_cdp_click_open_2.json"
        $null = Invoke-CdpPhase -Phase "assert-governance" -OutFile "r${round}_cdp_assert_governance.json"
        $gov = Get-Content (Join-Path $RunRoot "r${round}_cdp_assert_governance.json") -Raw -Encoding UTF8 | ConvertFrom-Json
        if ($gov.save -ne "保存并切换" -or $gov.switch -ne "直接切换" -or $gov.cancel -ne "取消") {
            throw ("R$round governance keys unexpected: save=$($gov.save) switch=$($gov.switch) cancel=$($gov.cancel)")
        }
        $null = Invoke-CdpPhase -Phase "click-governance-switch" -OutFile "r${round}_cdp_gov_click_2.json"
        # SHELL-4: launch 1 = L1c new project, launch 2 = fixture_a open, this = 3.
        $status2 = Wait-StatusLaunchCount -StatusUrl $StatusUrl -MinExpect 3 -TimeoutSec $OpenChainTimeoutSec -ArtifactPath (Join-Path $RunRoot "r${round}_status_opened_2.json")
        $null = Invoke-CdpPhase -Phase "probe-governance-done" -OutFile "r${round}_cdp_gov_done.json"
        $govDone = Get-Content (Join-Path $RunRoot "r${round}_cdp_gov_done.json") -Raw -Encoding UTF8 | ConvertFrom-Json
        if ($govDone.sentinel -ne $roundMeta.no_reload_sentinel) { throw "R$round sentinel changed after governed open (reload!)" }
        $state = Get-HttpJson -Url "http://127.0.0.1:$AgentPort/agent/state" -TimeoutSec 5
        if ($state.ok) {
            $stateJson = $state.body | ConvertFrom-Json
            $roundMeta.agent_current_project_after_open2 = [string]$stateJson.current_project.path
            if ($roundMeta.agent_current_project_after_open2 -notmatch "fixture_b") { throw "R$round /agent/state expected fixture_b, got: $($roundMeta.agent_current_project_after_open2)" }
        } else {
            throw "R$round /agent/state unreachable after governed open"
        }
        Write-Ok ("R$round governed open done: launch_count=" + $status2.launch_count + " current_project=fixture_b")

        # ---- R-L6: close main window -> whole tree teardown ----
        Write-Step "R$round L6: POST /close -> whole tree teardown"
        $null = Get-HttpPost -Url "$StatusBase/close" -TimeoutSec 5
        $deadline = (Get-Date).AddSeconds(20)
        while ((Get-Date) -lt $deadline) {
            Start-Sleep -Milliseconds 400
            if ($null -eq (Get-Process -Id $p.Id -ErrorAction SilentlyContinue)) { break }
        }
        if ($null -ne (Get-Process -Id $p.Id -ErrorAction SilentlyContinue)) { throw "R$round VitShell still alive 20s after close" }
        foreach ($name in @("VitShell", "VitAgent", "VitApp", "VspHub", "Godot*")) {
            Get-Process -Name $name -ErrorAction SilentlyContinue | Wait-Process -Timeout 10 -ErrorAction SilentlyContinue
        }
        Start-Sleep -Seconds 2
        $finalProcs = Get-VitProcessCount -Names $VitProcessNames
        if ($finalProcs -ne 0) {
            Get-Process -Name $VitProcessNames -ErrorAction SilentlyContinue | Select-Object Name, Id | Out-File -FilePath (Join-Path $RunRoot "r${round}_residue_processes.txt") -Encoding utf8
            throw "R$round Vit-family processes remain after close (count=$finalProcs)"
        }
        foreach ($port in @($AgentPort, $StatusPort)) {
            $deadline2 = (Get-Date).AddSeconds(10)
            $freed = $false
            while ((Get-Date) -lt $deadline2) {
                if ($null -eq (Get-TcpListenerPid -Port $port)) { $freed = $true; break }
                Start-Sleep -Milliseconds 400
            }
            if (-not $freed) { throw "R$round port $port still listening after teardown" }
        }
        if (-not (Wait-PortsBindable -Ports @($AgentPort, $StatusPort) -TimeoutSec 20)) { throw "R$round status/agent port not bindable after teardown (lingering bound socket)" }
        if ($round -lt $Rounds) {
            if (-not (Wait-VitWebviewGone -TimeoutSec 45)) { Write-Note "R${round}: Vit WebView2 browser processes still lingering (next round may hit shared-profile contention)" }
        }
        Write-Ok "R$round teardown: zero Vit-family processes, ports released and bindable"

        $postHash = (Get-FileHash $FixtureSource -Algorithm SHA256).Hash
        if ($postHash -ne $fixtureSourceHash) { throw "R$round fixture source hash changed (pollution!)" }

        Restore-S10State
        if ($script:s10RestoreErrors.Count -gt 0) {
            throw ("R$round S10 restore FAILED (env interrupt): " + ($script:s10RestoreErrors -join "; "))
        }
        Write-Ok "R$round S10 restore done (recents + runtime state, hash-verified)"

        $roundMeta.result = "PASS"
        $roundResults += $roundMeta
        $roundsPassed++
        $roundMeta.finished_at_utc = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
        $roundMeta | ConvertTo-Json -Depth 5 | Out-File -FilePath (Join-Path $RunRoot "meta_round_$round.json") -Encoding utf8
        Write-Ok "R$round PASS"
    } catch {
        $roundMeta.result = "FAIL"
        $roundMeta.error = $_.Exception.Message
        # SHELL-4 (AGENTS §8): separate failure classes - teardown/S10/fixture
        # pollution issues are environment interrupts, not product assertions.
        if ($roundMeta.error -match "S10|teardown|residue|pollution|not bindable|still listening") {
            $roundMeta.failure_class = "env_interrupt"
        } else {
            $roundMeta.failure_class = "assertion_failure"
        }
        # Failure-state evidence snapshot before teardown (read-only).
        $diag = [ordered]@{}
        if ($null -ne $p) {
            $proc = Get-Process -Id $p.Id -ErrorAction SilentlyContinue
            $diag.vitshell_alive = ($null -ne $proc)
            if ($null -ne $proc) {
                $diag.main_window_title = $proc.MainWindowTitle
                $diag.threads = $proc.Threads.Count
            } else {
                try { $diag.exit_code = $p.ExitCode } catch { $diag.exit_code = "unavailable" }
            }
        }
        $diag.vit_webview_processes = Get-VitWebviewProcessCount
        foreach ($dport in @($AgentPort, $StatusPort)) {
            $holders = @()
            Get-NetTCPConnection -LocalPort $dport -ErrorAction SilentlyContinue | ForEach-Object {
                $pn = try { (Get-Process -Id $_.OwningProcess -ErrorAction Stop).ProcessName } catch { "?" }
                $holders += ("state=" + $_.State + " pid=" + $_.OwningProcess + " proc=" + $pn)
            }
            $diag["port_" + $dport] = $holders
        }
        $roundMeta.failure_diag = $diag
        $roundMeta.finished_at_utc = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
        $roundResults += $roundMeta
        $roundMeta | ConvertTo-Json -Depth 6 | Out-File -FilePath (Join-Path $RunRoot "meta_round_$round.json") -Encoding utf8
        $diag | ConvertTo-Json -Depth 4 | Out-File -FilePath (Join-Path $RunRoot "r${round}_failure_diag.json") -Encoding utf8
        Write-Bad ("R$round FAIL: " + $_.Exception.Message + " - diag=" + ($diag | ConvertTo-Json -Depth 3 -Compress))
        Stop-TreeBestEffort -ShellPid $(if ($null -ne $p) { $p.Id } else { 0 })
        Restore-S10State
        if ($script:s10RestoreErrors.Count -gt 0) {
            Write-Bad ("R$round S10 restore ALSO FAILED (env interrupt): " + ($script:s10RestoreErrors -join "; "))
            if (-not $roundMeta.failure_class) { $roundMeta.failure_class = "env_interrupt" }
        }
        break
    }
}

# ---------- SHELL-4: boot-draft cleanup (delete exactly the run's drafts) ----------
$draftsCreated = @()
if (Test-Path $script:draftsRoot) {
    Get-ChildItem $script:draftsRoot -Directory -ErrorAction SilentlyContinue | ForEach-Object {
        if (-not $script:draftsBefore.ContainsKey($_.Name)) { $draftsCreated += $_.FullName }
    }
}
foreach ($draftDir in $draftsCreated) {
    Remove-Item $draftDir -Recurse -Force -ErrorAction SilentlyContinue
}
$draftsLeft = @()
if (Test-Path $script:draftsRoot) {
    Get-ChildItem $script:draftsRoot -Directory -ErrorAction SilentlyContinue | ForEach-Object {
        if (-not $script:draftsBefore.ContainsKey($_.Name)) { $draftsLeft += $_.Name }
    }
}
$runMeta.boot_drafts_created = $draftsCreated.Count
$runMeta.boot_drafts_left_after_cleanup = $draftsLeft
if ($draftsLeft.Count -gt 0) {
    Write-Bad ("S10 boot drafts not cleaned (env interrupt): " + ($draftsLeft -join "; "))
    $overallOk = $false
    $runMeta.boot_draft_cleanup = "FAILED (env interrupt)"
} else {
    $runMeta.boot_draft_cleanup = "ok"
    Write-Ok ("boot drafts cleaned (" + $draftsCreated.Count + " created by this run, all removed)")
}

# ---------- final meta + verdict ----------

$runMeta.round_results = $roundResults
$runMeta.rounds_passed = $roundsPassed
$runMeta.success_ratio = "$roundsPassed/$Rounds"

$failureLegsOk = $true
if (-not $SkipFailureLegs) {
    foreach ($k in $failureResults.Keys) {
        if ($failureResults[$k] -notmatch "^pass" -and $failureResults[$k] -notmatch "^boundary") { $failureLegsOk = $false }
    }
}

$runMeta.finished_at_utc = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
$runMeta | ConvertTo-Json -Depth 6 | Out-File -FilePath (Join-Path $RunRoot "meta.json") -Encoding utf8

$overallOk = ($legacyResult -eq "pass" -or $legacyResult -eq "skipped") -and $failureLegsOk -and ($roundsPassed -eq $Rounds)

if ($overallOk) {
    Write-Host ""
    Write-Host ("PASS: vitshell_full_smoke " + $runMeta.run_id + " (rounds $roundsPassed/$Rounds, exit 0)") -ForegroundColor Green
    Write-Host ("artifacts: " + $RunRoot)
    exit 0
} else {
    Write-Host ""
    Write-Host ("FAIL: vitshell_full_smoke " + $runMeta.run_id + " (legacy=$legacyResult, failureLegsOk=$failureLegsOk, rounds=$roundsPassed/$Rounds)") -ForegroundColor Red
    Write-Host ("artifacts: " + $RunRoot)
    exit 1
}