[CmdletBinding()]
param(
    [string]$RepoRoot = "",
    [string]$AgentHttp = "http://127.0.0.1:7878",
    [string]$ZmqReqPort = "5555",
    [string]$VspHubPort = "8787",
    [string]$KernelExe = "",
    [string]$AgentBinary = "",
    [string]$GodotExe = "D:\Godot\Godot_v4.6.1-stable_win64.exe",
    # 被测前端使用现行 UI-ENTRY-2 去回退语义；可显式传入待测前端 worktree。
    [string]$GodotProjectRoot = "D:\Godot\project\vit-daw-frontend",
    [string]$ProjectSource = "D:\Godot\project\vit-daw-frontend\912.vit",
    [string]$RunRoot = "",
    [int]$WaitSeconds = 60,
    [int]$SceneSettleSeconds = 40,
    [switch]$SkipBuild,
    [switch]$SkipLegacyLeg
)

# UI-ENTRY-1 端侧烟测（ui_host_smoke 同模式新脚本；单窗入口编排三腿）。
# 断言面（389 对齐现行 UI-ENTRY-2；原 UI-ENTRY-1 回退判据退役）：
#   腿 1 失败注入腿（VIT_AGENT_WINDOW_MODE=autoload + 无 agent 进程）：
#     health.failed + unavailable + navigate 行，零 main_window.restore；
#     EnumWindows：Godot 主窗隐藏，恰一个 React 窗保持可见并呈报窗内错误。
#   腿 2 autoload 腿（agent 在 + VIT_SMOKE_AUTO_OPEN_PROJECT 驱动真实 open 链）：
#     main_window.hidden 行 → page_loaded(url=?surface=launcher) → EnumWindows：
#     可见顶层窗恰 1 个 React 启动台（chat 可见）且 Godot 主窗不可见 →
#     scene_added(VitDockRoot) + project_stage.presented + surface.ack(surface=project)
#     → EnumWindows：Godot 主窗可见 + 同一 child_hwnd + navigate_count==1 +
#     page_url 不变（343 不变量：进工程切 surface 零导航零重载）。
#   腿 3 legacy 腿（flag 不设）：进程存活 + 零 boot 行 + 零 main_window.hidden 行。
# 通过标准：退出码 0（AGENTS §5 烟测门槛）。

$ErrorActionPreference = "Stop"
if ([string]::IsNullOrWhiteSpace($RepoRoot)) { $RepoRoot = (Split-Path -Parent $PSScriptRoot) }

function Write-Step0 { param([string]$Msg) Write-Host ("--- " + $Msg) }
if (-not (Test-Path -LiteralPath $RepoRoot)) { throw ("RepoRoot not found: " + $RepoRoot) }
if ([string]::IsNullOrWhiteSpace($RunRoot)) {
    $RunRoot = Join-Path $RepoRoot ("coord\runs\UI-ENTRY-1\run_" + (Get-Date -Format "yyyyMMdd_HHmmss"))
}
New-Item -ItemType Directory -Force -Path $RunRoot | Out-Null
$LogsDir   = Join-Path $RunRoot "logs"
$ArtDir    = Join-Path $RunRoot "artifacts"
$FixtureDir = Join-Path $RunRoot "fixture"
$AgentDir  = Join-Path $RepoRoot "agent"
foreach ($dir in @($LogsDir, $ArtDir, $FixtureDir)) { New-Item -ItemType Directory -Force -Path $dir | Out-Null }

function Get-TcpListener { param([int]$Port) return Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue | Select-Object -First 1 }

function Wait-PortListen {
    param([int]$Port, [int]$TimeoutSeconds)
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        if (Get-TcpListener -Port $Port) { return $true }
        Start-Sleep -Milliseconds 500
    }
    return $false
}

function Wait-HttpReady {
    param([string]$BaseUrl, [int]$TimeoutSeconds)
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        try {
            $resp = Invoke-WebRequest -UseBasicParsing -Uri ($BaseUrl.TrimEnd("/") + "/health") -TimeoutSec 2
            if ($resp.StatusCode -eq 200) { return $true }
        }
        catch { }
        Start-Sleep -Milliseconds 500
    }
    return $false
}

function Test-LogLine {
    param([string]$LogPath, [string]$Pattern, [int]$TimeoutSeconds)
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        if (Test-Path -LiteralPath $LogPath) {
            $hit = Select-String -LiteralPath $LogPath -Pattern $Pattern -ErrorAction SilentlyContinue | Select-Object -Last 1
            if ($null -ne $hit) { return $hit }
        }
        Start-Sleep -Milliseconds 500
    }
    return $null
}

function Stop-GuiProcess { param([int]$ProcId)
    if ($ProcId -gt 0) { Stop-Process -Id $ProcId -Force -ErrorAction SilentlyContinue }
}

Add-Type -Namespace UiEntrySmoke -Name Win32 -MemberDefinition @'
[DllImport("user32.dll")] public static extern bool EnumWindows(EnumProc cb, IntPtr lp);
public delegate bool EnumProc(IntPtr hWnd, IntPtr lp);
[DllImport("user32.dll", CharSet=CharSet.Unicode)] public static extern int GetClassNameW(IntPtr hWnd, System.Text.StringBuilder text, int count);
[DllImport("user32.dll", CharSet=CharSet.Unicode)] public static extern int GetWindowTextW(IntPtr hWnd, System.Text.StringBuilder text, int count);
[DllImport("user32.dll")] public static extern bool IsWindowVisible(IntPtr hWnd);
[DllImport("user32.dll")] public static extern uint GetWindowThreadProcessId(IntPtr hWnd, ref uint pid);
'@

# 枚举全部顶层窗 → (hwnd,class,title,pid,visible)。UI-HOST-1 先例：FindWindowExW 跨
# 进程不可用，EnumWindows 是本环境标准枚举面。
function Get-AllWindows {
    $found = New-Object System.Collections.ArrayList
    $enumCb = [UiEntrySmoke.Win32+EnumProc]{ param($h, $lp)
        $cn = New-Object System.Text.StringBuilder 256
        [void][UiEntrySmoke.Win32]::GetClassNameW($h, $cn, 256)
        $sb = New-Object System.Text.StringBuilder 256
        [void][UiEntrySmoke.Win32]::GetWindowTextW($h, $sb, 256)
        $procId = [uint32]0
        [void][UiEntrySmoke.Win32]::GetWindowThreadProcessId($h, [ref]$procId)
        [void]$found.Add([pscustomobject]@{
            Hwnd = [long]$h
            Class = $cn.ToString()
            Title = $sb.ToString()
            Pid = [int]$procId
            Visible = [bool][UiEntrySmoke.Win32]::IsWindowVisible($h)
        })
        return $true
    }
    [void][UiEntrySmoke.Win32]::EnumWindows($enumCb, [IntPtr]::Zero)
    return $found
}

# Godot 主窗（本 Godot 进程 + title 含 Vit_DAW_Frontend）。
function Get-DawMainWindow {
    param([int]$GodotPid)
    foreach ($w in (Get-AllWindows)) {
        if ($w.Pid -ne $GodotPid) { continue }
        if ($w.Class -eq "VitWebViewHostChildWindow") { continue }
        if ($w.Title -like "*Vit_DAW_Frontend*") { return $w }
    }
    return $null
}

# ---------------------------------------------------------------- prereq
$report = [ordered]@{
    run_root = $RunRoot
    started_utc = (Get-Date).ToUniversalTime().ToString("o")
    head = (& git -C $RepoRoot rev-parse HEAD)
    frontend_head = (& git -C $GodotProjectRoot rev-parse HEAD)
    godot_project_root = $GodotProjectRoot
    project_source = $ProjectSource
    assertions = [ordered]@{}
    failinject_leg = [ordered]@{}
    legacy_leg = [ordered]@{}
    exit_code = 1
}
$assert = $report.assertions

Write-Host "=== UI-ENTRY-1 smoke; run dir: $RunRoot"
Write-Host ("repo HEAD=" + $report.head + "  frontend HEAD=" + $report.frontend_head)

# §2.2 复用禁令：任一端口已监听即停（栈占用以 coord/resources/PC-RUNTIME-STACK.md 为准，
# 由调用者保证登记推送；脚本自身按 AGENTS §9 先净空后启栈）。
Write-Step0 "Prereq: ports must be free (7878/5555/5556/8787)"
[int]$agentPort = 7878
if ($AgentHttp -match ':(\d+)\s*$') { $agentPort = [int]$Matches[1] }
foreach ($port in $agentPort, [int]$ZmqReqPort, 5556, [int]$VspHubPort) {
    if (Get-TcpListener -Port $port) {
        throw ("port $port is already listening; NOT reusing a foreign stack (PROTOCOL 2.2 / AGENTS 9). Abort before any start.")
    }
    $assert["port_free_$port"] = $true
}

if (-not (Test-Path -LiteralPath $GodotProjectRoot)) { throw ("Godot project root not found: " + $GodotProjectRoot) }
if (-not (Test-Path -LiteralPath $GodotExe -PathType Leaf)) { throw ("Godot exe not found: " + $GodotExe) }
if (-not (Test-Path -LiteralPath $ProjectSource -PathType Leaf)) { throw ("project fixture not found: " + $ProjectSource) }
if ([string]::IsNullOrWhiteSpace($KernelExe)) {
    foreach ($candidate in @(
        (Join-Path $RepoRoot "VitApp\build\VitApp_artefacts\Release\VitApp.exe"),
        (Join-Path "D:\Vit_DAW" "VitApp\build\VitApp_artefacts\Release\VitApp.exe"),
        (Join-Path "D:\Vit_DAW" "VitApp\build_release\VitApp_artefacts\Release\VitApp.exe"),
        (Join-Path "D:\Vit_DAW" "Export\staging\runtime\VitApp.exe"))) {
        if (Test-Path -LiteralPath $candidate) { $KernelExe = $candidate; break }
    }
}
if (-not (Test-Path -LiteralPath $KernelExe -PathType Leaf)) { throw ("kernel exe not found: " + $KernelExe) }
$assert["kernel_exe"] = $KernelExe

# fixture 隔离：复制到 run 目录（不触碰样例/用户工程；AGENTS §10）。
$ProjectCopy = Join-Path $FixtureDir ("ui_entry_" + [System.IO.Path]::GetFileName($ProjectSource))
Copy-Item -LiteralPath $ProjectSource -Destination $ProjectCopy -Force
$fixtureHashBefore = (Get-FileHash -Algorithm SHA256 -LiteralPath $ProjectSource).Hash
$assert["fixture_copy"] = $ProjectCopy
$assert["fixture_source_sha256"] = $fixtureHashBefore

# recents 快照（autoload 腿 open 链会写内核 recents；finally 快照回写，净差零；AGENTS §10）。
$recentsConfig = Join-Path $env:APPDATA "Vit-DAW\global_project_config.json"
$recentsBackup = $null
if (Test-Path -LiteralPath $recentsConfig) {
    $recentsBackup = Join-Path $ArtDir "global_project_config.before.json"
    Copy-Item -LiteralPath $recentsConfig -Destination $recentsBackup -Force
    $assert["recents_backup"] = $recentsBackup
}

# kernel workspace 隔离（VIT_PROJECT_XML 指 run 目录副本）。
$KernelWorkspace = Join-Path $RunRoot "kernel_workspace"
New-Item -ItemType Directory -Force -Path (Join-Path $KernelWorkspace "Settings") | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $KernelWorkspace "Logs") | Out-Null
$RepoDefaultProject = Join-Path $RepoRoot "VitApp\Workspace\default_project.xml"
if (Test-Path -LiteralPath $RepoDefaultProject) {
    Copy-Item -LiteralPath $RepoDefaultProject -Destination (Join-Path $KernelWorkspace "default_project.xml") -Force
}

# agent 构建（隔离二进制放 run 目录）。
if ([string]::IsNullOrWhiteSpace($AgentBinary)) { $AgentBinary = Join-Path $RunRoot "bin\VitAgent.ui_entry.exe" }
if (-not $SkipBuild) {
    Write-Step0 "Build VitAgent (isolated binary in run dir)"
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $AgentBinary) | Out-Null
    Push-Location $AgentDir
    try {
        & go build -o $AgentBinary .\cmd\vitagent
        if ($LASTEXITCODE -ne 0) { throw ("go build failed with exit code " + $LASTEXITCODE) }
    }
    finally { Pop-Location }
} elseif (-not (Test-Path -LiteralPath $AgentBinary -PathType Leaf)) {
    throw "-SkipBuild given but the isolated agent binary is missing"
}
$assert["agent_binary_sha256"] = (Get-FileHash -Algorithm SHA256 -LiteralPath $AgentBinary).Hash

$kernelProcId = 0
$agentProcId = 0
$godotAutoloadPid = 0
$godotLegacyPid = 0

function Stop-Stack {
    Write-Step0 "Finally: stop stack and confirm port release"
    Stop-GuiProcess $godotAutoloadPid
    Stop-GuiProcess $godotLegacyPid
    if ($agentProcId -gt 0) { Stop-Process -Id $agentProcId -Force -ErrorAction SilentlyContinue }
    Start-Sleep -Seconds 1
    if ($kernelProcId -gt 0) { Stop-Process -Id $kernelProcId -Force -ErrorAction SilentlyContinue }
    $deadline = (Get-Date).AddSeconds(30)
    while ((Get-Date) -lt $deadline) {
        $busy = @()
        foreach ($port in 7878, [int]$ZmqReqPort, 5556, [int]$VspHubPort) { if (Get-TcpListener -Port $port) { $busy += $port } }
        if ($busy.Count -eq 0) { break }
        Start-Sleep -Milliseconds 500
    }
    foreach ($port in 7878, [int]$ZmqReqPort, 5556, [int]$VspHubPort) {
        if (Get-TcpListener -Port $port) { $assert["port_still_busy_$port"] = $true }
    }
}

$agentLog = Join-Path $LogsDir "agent_last.log"
$godotAutoloadLog = Join-Path $LogsDir "godot_autoload.log"
$godotFailInjectLog = Join-Path $LogsDir "godot_failinject.log"
$godotLegacyLog = Join-Path $LogsDir "godot_legacy.log"

function Start-GodotLeg {
    param([string]$LogPath, [switch]$WithAutoOpen, [switch]$IsLegacy)
    if (-not $IsLegacy) { $env:VIT_AGENT_WINDOW_MODE = "autoload" }
    if ($WithAutoOpen) { $env:VIT_SMOKE_AUTO_OPEN_PROJECT = $ProjectCopy }
    $env:VIT_SKIP_DEV_AUTOSTART = "1"
    $env:VIT_DAW_DEV_ROOT = $RepoRoot
    try {
        $proc = Start-Process -FilePath $GodotExe -ArgumentList @("--path", $GodotProjectRoot, "--log-file", $LogPath) -WorkingDirectory $GodotProjectRoot -PassThru
    }
    finally {
        Remove-Item Env:VIT_AGENT_WINDOW_MODE -ErrorAction SilentlyContinue
        Remove-Item Env:VIT_SMOKE_AUTO_OPEN_PROJECT -ErrorAction SilentlyContinue
        Remove-Item Env:VIT_SKIP_DEV_AUTOSTART -ErrorAction SilentlyContinue
        Remove-Item Env:VIT_DAW_DEV_ROOT -ErrorAction SilentlyContinue
    }
    return $proc
}

try {
    # ---------------------------------------------------------- start kernel
    Write-Step0 "Start VitApp kernel (isolated workspace)"
    $env:VIT_PROJECT_XML = (Join-Path $KernelWorkspace "default_project.xml")
    $env:VIT_DAW_DEV_ROOT = $RepoRoot
    try {
        $kernelProc = Start-Process -FilePath $KernelExe -WorkingDirectory $KernelWorkspace -WindowStyle Hidden -PassThru
    }
    finally {
        Remove-Item Env:VIT_PROJECT_XML -ErrorAction SilentlyContinue
        Remove-Item Env:VIT_DAW_DEV_ROOT -ErrorAction SilentlyContinue
    }
    $kernelProcId = $kernelProc.Id
    $assert["kernel_pid"] = $kernelProcId
    if (-not (Wait-PortListen -Port ([int]$ZmqReqPort) -TimeoutSeconds $WaitSeconds)) {
        throw ("kernel ZMQ REQ port $ZmqReqPort did not listen")
    }
    $assert["kernel_zmq_listening"] = $true

    # ------------------------------------------------- import pass (fresh worktree cache)
    Write-Step0 "Import pass (rebuild .godot cache: extension_list + global class cache)"
    $importProc = Start-Process -FilePath $GodotExe -ArgumentList @("--headless", "--path", $GodotProjectRoot, "--import") -WorkingDirectory $GodotProjectRoot -PassThru -Wait -WindowStyle Hidden
    if (-not (Test-Path -LiteralPath (Join-Path $GodotProjectRoot ".godot\extension_list.cfg"))) {
        throw ("Godot import pass did not produce .godot/extension_list.cfg")
    }
    $assert["godot_import_pass"] = $true

    # =============================================== leg 1: fail-inject (no agent, UI-ENTRY-2)
    Write-Step0 "Leg 1 (fail-inject): NO agent -> health fails -> React error window remains, ZERO Godot start page"
    $env:VIT_AGENT_WINDOW_MODE = "autoload"
    $env:VIT_SKIP_DEV_AUTOSTART = "1"
    $env:VIT_DAW_DEV_ROOT = $RepoRoot
    try {
        $godotFailInjectProc = Start-Process -FilePath $GodotExe -ArgumentList @("--path", $GodotProjectRoot, "--log-file", $godotFailInjectLog) -WorkingDirectory $GodotProjectRoot -PassThru
    }
    finally {
        Remove-Item Env:VIT_AGENT_WINDOW_MODE -ErrorAction SilentlyContinue
        Remove-Item Env:VIT_SKIP_DEV_AUTOSTART -ErrorAction SilentlyContinue
        Remove-Item Env:VIT_DAW_DEV_ROOT -ErrorAction SilentlyContinue
    }
    $godotAutoloadPid = $godotFailInjectProc.Id
    $report.failinject_leg["godot_pid"] = $godotAutoloadPid

    # boot 行（React 窗建成——fail-inject 腿的失败点是 agent health，不是建窗）。
    $fbBoot = Test-LogLine -LogPath $godotFailInjectLog -Pattern '\[AgentChatWindow\] boot ' -TimeoutSeconds $WaitSeconds
    if ($null -eq $fbBoot) { throw ("fail-inject leg: boot line not seen (window create unexpectedly failed)") }
    $report.failinject_leg["boot_line"] = "present"

    # health.failed + unavailable（真链失败注入：24×0.45s 上限 + 余量）。
    $fbHealthFail = Test-LogLine -LogPath $godotFailInjectLog -Pattern '\[AgentChatWindow\] health\.failed ' -TimeoutSeconds 30
    if ($null -eq $fbHealthFail) { throw ("fail-inject leg: health.failed not seen (agent reachable when it must not be)") }
    $fbUnavailable = Test-LogLine -LogPath $godotFailInjectLog -Pattern '\[AgentChatWindow\] unavailable ' -TimeoutSeconds 10
    if ($null -eq $fbUnavailable) { throw ("fail-inject leg: unavailable line not seen") }
    $report.failinject_leg["unavailable_line"] = ($fbUnavailable.Line -replace '^.*\[AgentChatWindow\] unavailable ', '').Trim()

    # UI-ENTRY-2：失败留在 React 窗内，禁止 unavailable → Godot 启动页回退。
    $fbHidden = Test-LogLine -LogPath $godotFailInjectLog -Pattern '\[AgentChatWindow\] main_window\.hidden ' -TimeoutSeconds 15
    if ($null -eq $fbHidden) { throw ("fail-inject leg: main_window.hidden not seen") }
    $report.failinject_leg["hidden_line"] = "present"
    $fbNav = Test-LogLine -LogPath $godotFailInjectLog -Pattern '\[AgentChatWindow\] navigate ' -TimeoutSeconds 10
    $report.failinject_leg["navigate_line"] = ($null -ne $fbNav)
    if ($null -eq $fbNav) { throw ("fail-inject leg: navigate line not seen (error-state navigation not attempted)") }
    Start-Sleep -Seconds 2
    $restoreLines = @(Select-String -LiteralPath $godotFailInjectLog -Pattern '\[AgentChatWindow\] main_window\.restore ' -ErrorAction Stop)
    $report.failinject_leg["zero_main_window_restore"] = ($restoreLines.Count -eq 0)
    if ($restoreLines.Count -ne 0) { throw ("fail-inject leg: main_window.restore seen — Godot start page fallback must be retired") }

    $dawWin = Get-DawMainWindow -GodotPid $godotAutoloadPid
    $report.failinject_leg["daw_window_found"] = ($null -ne $dawWin)
    if ($null -eq $dawWin) { throw ("fail-inject leg: Godot main window not found via EnumWindows") }
    $report.failinject_leg["daw_window_visible"] = $dawWin.Visible
    if ($dawWin.Visible) { throw ("fail-inject leg: Godot main window visible — start page fallback must never appear") }
    $chatWins = @((Get-AllWindows) | Where-Object { $_.Class -eq "VitWebViewHostChildWindow" -and $_.Visible })
    $report.failinject_leg["visible_chat_count"] = $chatWins.Count
    if ($chatWins.Count -ne 1) { throw ("fail-inject leg: expected exactly 1 visible React window, got $($chatWins.Count)") }
    $assert["failinject_leg_pass"] = $true

    Stop-GuiProcess $godotAutoloadPid
    $godotAutoloadPid = 0
    Start-Sleep -Seconds 2

    # ---------------------------------------------------------- start agent
    Write-Step0 "Start VitAgent (UDP 4445 bridge face + HTTP 7878)"
    $env:VIT_HISTORY_DRAFT_ROOT = (Join-Path $RunRoot "agent_drafts")
    New-Item -ItemType Directory -Force -Path $env:VIT_HISTORY_DRAFT_ROOT | Out-Null
    $env:VIT_DAW_DEV_ROOT = $RepoRoot
    try {
        $agentProc = Start-Process -FilePath $AgentBinary -ArgumentList @("-http", ($AgentHttp -replace '^http://',''), "-last-log-path", $agentLog, "-keep-last-log-lines", "8000") -WorkingDirectory $AgentDir -WindowStyle Hidden -PassThru
    }
    finally {
        Remove-Item Env:VIT_HISTORY_DRAFT_ROOT -ErrorAction SilentlyContinue
        Remove-Item Env:VIT_DAW_DEV_ROOT -ErrorAction SilentlyContinue
    }
    $agentProcId = $agentProc.Id
    $assert["agent_pid"] = $agentProcId
    if (-not (Wait-HttpReady -BaseUrl $AgentHttp -TimeoutSeconds $WaitSeconds)) {
        throw ("agent HTTP did not become ready")
    }
    $assert["agent_http_ready"] = $true

    # =============================================== leg 2: autoload (single-window entry)
    Write-Step0 "Leg 2 (autoload): single-window entry -> open project -> main window restored + same-HWND surface switch"
    $godotProc = Start-GodotLeg -LogPath $godotAutoloadLog -WithAutoOpen
    $godotAutoloadPid = $godotProc.Id
    $assert["godot_autoload_pid"] = $godotAutoloadPid

    # boot 行。
    $bootHit = Test-LogLine -LogPath $godotAutoloadLog -Pattern '\[AgentChatWindow\] boot ' -TimeoutSeconds $WaitSeconds
    if ($null -eq $bootHit) { throw ("AgentChatWindow boot line not seen in " + $godotAutoloadLog) }
    $assert["boot_line"] = ($bootHit.Line -replace '^.*\[AgentChatWindow\] boot ', '').Trim()

    # main_window.hidden（React 窗建成 → Godot 主窗启动隐藏）。
    $hiddenHit = Test-LogLine -LogPath $godotAutoloadLog -Pattern '\[AgentChatWindow\] main_window\.hidden ' -TimeoutSeconds 15
    if ($null -eq $hiddenHit) { throw ("main_window.hidden not seen (Godot main window was not hidden at startup)") }
    $assert["main_window_hidden_line"] = ($hiddenHit.Line -replace '^.*\[AgentChatWindow\] main_window\.hidden ', '').Trim()

    # visible 行（child_hwnd 留证）。
    $visHit = Test-LogLine -LogPath $godotAutoloadLog -Pattern '\[AgentChatWindow\] visible ' -TimeoutSeconds 30
    if ($null -eq $visHit) { throw ("AgentChatWindow visible line not seen") }
    $childHwnd = 0
    try { $childHwnd = [long](($visHit.Line | Select-String -Pattern '"child_hwnd":\s*(-?\d+)' -AllMatches).Matches[0].Groups[1].Value) } catch { $childHwnd = 0 }
    if ($childHwnd -le 0) { throw ("child_hwnd missing/invalid in visible line") }
    $assert["child_hwnd"] = $childHwnd

    # page_loaded（launcher URL 路由生效）——断言在单窗轮询之后（轮询须在 open 链
    # 启动前的静置窗口内完成，page_loaded 锚点先等待会吃掉静置窗口）。
    # 启动态单窗断言：hidden 行（SW_HIDE 生效）后轮询枚举——静置窗口内（React 页
    # 加载中、open 链未启动）主窗必须不可见+React 启动台恰 1 个可见。轮询捕捉首个
    # 成立点；主窗静置期提前可见=立即失败；超时=主窗从未隐藏=真失败。
    # （run_20261114_233618 教训：page_loaded 后 sleep 2 枚举与 auto_open 竞速——
    # open 链可在 2s 内完成并 restore 主窗，断言窗口必须前移到 hidden 锚点。）
    $singleWindowOK = $false
    $pollDeadline = (Get-Date).AddSeconds(20)
    while ((Get-Date) -lt $pollDeadline) {
        $chatVis = @((Get-AllWindows) | Where-Object { $_.Class -eq "VitWebViewHostChildWindow" -and $_.Visible })
        $dawNow = Get-DawMainWindow -GodotPid $godotAutoloadPid
        $dawVisNow = ($null -ne $dawNow -and $dawNow.Visible)
        if ($chatVis.Count -eq 1 -and (-not $dawVisNow) -and ([long]$chatVis[0].Hwnd) -eq $childHwnd) {
            $singleWindowOK = $true
            break
        }
        if ($dawVisNow) { break }
        Start-Sleep -Milliseconds 200
    }
    $assert["startup_single_visible_window"] = $singleWindowOK
    if (-not $singleWindowOK) {
        $dawFinal = Get-DawMainWindow -GodotPid $godotAutoloadPid
        $chatFinal = @((Get-AllWindows) | Where-Object { $_.Class -eq "VitWebViewHostChildWindow" -and $_.Visible })
        throw ("startup is not single-window within polling window: daw_visible=$($null -ne $dawFinal -and $dawFinal.Visible) visible_chat_count=$($chatFinal.Count) child_hwnd=$childHwnd")
    }

    # page_loaded（launcher URL 路由生效）。
    $pageHit = Test-LogLine -LogPath $godotAutoloadLog -Pattern '\[AgentChatWindow\] page_loaded ' -TimeoutSeconds 90
    if ($null -eq $pageHit) { throw ("AgentChatWindow page_loaded line not seen (React did not load)") }
    $pageLoadedUrl = ""
    try { $pageLoadedUrl = (($pageHit.Line | Select-String -Pattern '"url":\s*"([^"]*)"' -AllMatches).Matches[0].Groups[1].Value) } catch { $pageLoadedUrl = "" }
    $assert["page_loaded_url"] = $pageLoadedUrl
    $assert["page_url_is_launcher"] = ($pageLoadedUrl -like "http://127.0.0.1:7878/app/?surface=launcher*")
    if (-not $assert["page_url_is_launcher"]) { throw ("autoload startup did not route to launcher surface: $pageLoadedUrl") }

    # 驱动钩子完成（start_page open 链 → _enter_main_scene → dock）。
    $doneHit = Test-LogLine -LogPath $godotAutoloadLog -Pattern '\[AgentChatWindow\] smoke\.auto_open\.done ' -TimeoutSeconds $SceneSettleSeconds
    if ($null -eq $doneHit) { throw ("smoke.auto_open.done not seen within ${SceneSettleSeconds}s (open chain failed)") }
    $assert["auto_open_done_line"] = ($doneHit.Line -replace '^.*\[AgentChatWindow\] smoke\.auto_open\.done ', '').Trim()

    # scene_added(VitDockRoot)。
    $sceneHit = Test-LogLine -LogPath $godotAutoloadLog -Pattern 'scene_added.*VitDockRoot' -TimeoutSeconds $SceneSettleSeconds
    if ($null -eq $sceneHit) { throw ("scene_added VitDockRoot not seen (scene switch did not happen)") }
    $assert["scene_added_line"] = $sceneHit.Line

    # project_stage.presented（dock _ready → 进工程展开接线）。
    $presentedHit = Test-LogLine -LogPath $godotAutoloadLog -Pattern '\[AgentChatWindow\] project_stage\.presented ' -TimeoutSeconds 15
    if ($null -eq $presentedHit) { throw ("project_stage.presented not seen (dock entry wiring did not fire)") }
    $presentedHwnd = 0
    $presentedNav = -1
    try { $presentedHwnd = [long](($presentedHit.Line | Select-String -Pattern '"child_hwnd":\s*(-?\d+)' -AllMatches).Matches[0].Groups[1].Value) } catch { $presentedHwnd = 0 }
    try { $presentedNav = [int](($presentedHit.Line | Select-String -Pattern '"navigate_count":\s*(\d+)' -AllMatches).Matches[0].Groups[1].Value) } catch { $presentedNav = -1 }
    $presentedUrl = ""
    try { $presentedUrl = (($presentedHit.Line | Select-String -Pattern '"page_url":\s*"([^"]*)"' -AllMatches).Matches[0].Groups[1].Value) } catch { $presentedUrl = "" }
    $assert["project_stage_presented_line"] = ($presentedHit.Line -replace '^.*\[AgentChatWindow\] project_stage\.presented ', '').Trim()

    # surface.ack（React 真收 vit_surface 的回执——project surface 在场证明）。
    $ackHit = Test-LogLine -LogPath $godotAutoloadLog -Pattern '\[AgentChatWindow\] surface\.ack .*"surface":\s*"project"' -TimeoutSeconds 30
    if ($null -eq $ackHit) { throw ("surface.ack(surface=project) not seen (React did not confirm the surface switch)") }
    $assert["surface_ack_line"] = ($ackHit.Line -replace '^.*\[AgentChatWindow\] surface\.ack ', '').Trim()

    # 进工程后：Godot 主窗恢复可见 + 同 HWND。
    Start-Sleep -Seconds 2
    $dawWinAfter = Get-DawMainWindow -GodotPid $godotAutoloadPid
    $assert["project_daw_window_found"] = ($null -ne $dawWinAfter)
    $assert["project_daw_window_visible"] = ($null -ne $dawWinAfter -and $dawWinAfter.Visible)
    if (-not $assert["project_daw_window_visible"]) { throw ("Godot main window not visible after project entry (DAW expansion failed)") }

    $sceneHwnd = 0
    $sceneHwndMatch = ($sceneHit.Line | Select-String -Pattern '"child_hwnd":\s*(-?\d+)' -AllMatches).Matches
    if ($sceneHwndMatch.Count -gt 0) { $sceneHwnd = [long]$sceneHwndMatch[0].Groups[1].Value }
    $doneHwnd = 0
    $doneHwndMatch = ($doneHit.Line | Select-String -Pattern '"child_hwnd":\s*(-?\d+)' -AllMatches).Matches
    if ($doneHwndMatch.Count -gt 0) { $doneHwnd = [long]$doneHwndMatch[0].Groups[1].Value }
    $doneNavCount = -1
    $navMatch = ($doneHit.Line | Select-String -Pattern '"navigate_count":\s*(\d+)' -AllMatches).Matches
    if ($navMatch.Count -gt 0) { $doneNavCount = [int]$navMatch[0].Groups[1].Value }

    # 断言 1：同一 HWND 跨场景切换+进工程展开（boot child_hwnd == done == scene == presented）。
    $assert["same_hwnd_across_entry"] = ($childHwnd -eq $doneHwnd -and $doneHwnd -eq $sceneHwnd -and $sceneHwnd -eq $presentedHwnd -and $doneHwnd -gt 0)
    # 断言 2：navigate_count == 1（surface 切换零导航，343 不变量）。
    $assert["navigate_count_is_1"] = ($doneNavCount -eq 1 -and $presentedNav -eq 1)
    # 断言 3：URL 未变（launcher 页面同窗在用）。
    $assert["page_url_unchanged_after_entry"] = ($presentedUrl -like "http://127.0.0.1:7878/app/*")
    # 断言 4：Win32 枚举的 chat 窗 hwnd 与日志一致且可见。
    $chatWinAfter = @((Get-AllWindows) | Where-Object { $_.Class -eq "VitWebViewHostChildWindow" })
    $assert["chat_window_count_after_entry"] = $chatWinAfter.Count
    $assert["chat_window_visible_after_entry"] = ($chatWinAfter.Count -eq 1 -and $chatWinAfter[0].Visible -and ([long]$chatWinAfter[0].Hwnd) -eq $childHwnd)

    if (-not $assert["same_hwnd_across_entry"]) { throw ("HWND changed across entry: child=$childHwnd done=$doneHwnd scene=$sceneHwnd presented=$presentedHwnd") }
    if (-not $assert["navigate_count_is_1"]) { throw ("page reloaded on surface switch: done=$doneNavCount presented=$presentedNav") }
    if (-not $assert["page_url_unchanged_after_entry"]) { throw ("page URL changed: $presentedUrl") }
    if (-not $assert["chat_window_visible_after_entry"]) { throw ("chat window missing/hidden/inconsistent after entry (count=$($chatWinAfter.Count))") }
    $assert["autoload_leg_pass"] = $true

    # ---------------------------------------------------------- legacy leg
    if (-not $SkipLegacyLeg) {
        Write-Step0 "Leg 3 (legacy): flag unset keeps the Godot start page, zero new orchestration lines"
        Stop-GuiProcess $godotAutoloadPid
        $godotAutoloadPid = 0
        Start-Sleep -Seconds 2
        $godotLegacyProc = Start-GodotLeg -LogPath $godotLegacyLog -IsLegacy
        $godotLegacyPid = $godotLegacyProc.Id
        $report.legacy_leg["godot_pid"] = $godotLegacyPid
        Start-Sleep -Seconds 12
        $legacyBoot = $null
        $legacyHidden = $null
        if (Test-Path -LiteralPath $godotLegacyLog) {
            $legacyBoot = Select-String -LiteralPath $godotLegacyLog -Pattern '\[AgentChatWindow\] boot ' -ErrorAction SilentlyContinue | Select-Object -First 1
            $legacyHidden = Select-String -LiteralPath $godotLegacyLog -Pattern '\[AgentChatWindow\] main_window\.hidden ' -ErrorAction SilentlyContinue | Select-Object -First 1
        }
        $report.legacy_leg["no_boot_line"] = ($null -eq $legacyBoot)
        $report.legacy_leg["no_main_window_hidden_line"] = ($null -eq $legacyHidden)
        $legacyProcAlive = $true
        try { $legacyProcAlive = (-not $godotLegacyProc.HasExited) } catch { $legacyProcAlive = $false }
        $report.legacy_leg["process_alive"] = $legacyProcAlive
        if (-not $report.legacy_leg["no_boot_line"]) { throw ("legacy mode unexpectedly created the chat window") }
        if (-not $report.legacy_leg["no_main_window_hidden_line"]) { throw ("legacy mode unexpectedly hid the Godot main window") }
        if (-not $legacyProcAlive) { throw ("legacy-mode Godot exited early") }
        $assert["legacy_leg_pass"] = $true
    }

    $report.exit_code = 0
}
catch {
    $report["error"] = $_.Exception.Message
    $report["error_type"] = $_.Exception.GetType().Name
    Write-Host ("FAIL: " + $_.Exception.Message)
}
finally {
    Stop-Stack
    $report["finished_utc"] = (Get-Date).ToUniversalTime().ToString("o")
    # recents 还原（快照回写，净差零；AGENTS §10）+ fixture 源哈希核对。
    if ($null -ne $recentsBackup -and (Test-Path -LiteralPath $recentsBackup)) {
        Copy-Item -LiteralPath $recentsBackup -Destination $recentsConfig -Force
        Write-Host ("recents restored from snapshot: " + $recentsBackup)
    }
    $fixtureHashAfter = (Get-FileHash -Algorithm SHA256 -LiteralPath $ProjectSource).Hash
    $report["fixture_source_sha256_after"] = $fixtureHashAfter
    $report["fixture_source_unchanged"] = ($fixtureHashAfter -eq $fixtureHashBefore)
    if (-not $report["fixture_source_unchanged"]) { Write-Host ("WARN: fixture source hash changed during run") }
    $reportPath = Join-Path $ArtDir "ui_entry_smoke_report.json"
    $report | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $reportPath -Encoding UTF8
    Write-Host ("report: " + $reportPath)
    foreach ($log in @($godotAutoloadLog, $godotFailInjectLog, $godotLegacyLog, $agentLog)) {
        Copy-Item -LiteralPath $log -Destination $ArtDir -Force -ErrorAction SilentlyContinue
    }
}

if ($report.exit_code -eq 0) { Write-Host "UI-ENTRY-1 smoke: PASS" ; exit 0 }
else { Write-Host "UI-ENTRY-1 smoke: FAIL" ; exit 1 }
