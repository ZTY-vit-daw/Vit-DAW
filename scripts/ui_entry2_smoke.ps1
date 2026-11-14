[CmdletBinding()]
param(
    [string]$RepoRoot = "",
    [string]$AgentHttp = "http://127.0.0.1:7878",
    [string]$ZmqReqPort = "5555",
    [string]$VspHubPort = "8787",
    [string]$KernelExe = "",
    [string]$GodotExe = "D:\Godot\Godot_v4.6.1-stable_win64.exe",
    # 被测前端 = UI-ENTRY-2 的前端 worktree（改动在 port/ui-entry-2 分支；extension cpp 零改动零重编）。
    [string]$GodotProjectRoot = "D:\Godot\project\vit-daw-frontend_wt_uientry2",
    [string]$ProjectSource = "D:\Godot\project\vit-daw-frontend\912.vit",
    [string]$RunRoot = "",
    [int]$WaitSeconds = 60,
    [int]$SceneSettleSeconds = 40,
    [switch]$SkipBuild,
    [switch]$SkipLegacyLeg
)

# UI-ENTRY-2 端侧烟测（ui_entry_smoke 354 同模式新脚本；启动页门面退役四腿）。
# 断言面（RUNBOOK coord/runs/UI-ENTRY-2/RUNBOOK.md 冻结）：
#   腿 1 失败注入腿（VIT_AGENT_WINDOW_MODE=autoload + 无 agent + VIT_SKIP_DEV_AUTOSTART=1）：
#     health.failed + unavailable 行 → 零 main_window.restore 行（去 Godot 化）→
#     navigate 行在场（错误态导航）→ EnumWindows：Godot 主窗不可见（启动页零出现）
#     + VitWebViewHostChildWindow 恰 1 可见（React 窗保持在场）。
#   腿 2 WebView2 不可用腿（VIT_SMOKE_WEBVIEW2_FAIL=1）：webview2.unavailable 行 +
#     零 boot 行 → #32770 原生错误对话框出现（pid 归属）→ WM_CLOSE 关闭对话框 →
#     Godot 进程退出 + 全程零 VitWebViewHostChildWindow。
#   腿 3 autoload 全链腿（VIT_SMOKE_AUTO_OPEN_PROJECT + autostart 开）：
#     354 全链（单窗启动→launcher 路由→open 链→进工程展开→同 HWND→navigate_count==1
#     →surface.ack(project)）+ 本卡核心：pid 端口归属实测后对 child hwnd 投递 WM_CLOSE
#     → Godot/agent/kernel 全部进程退出 + 端口释放（关窗=退出整个应用）。
#   腿 4 legacy 腿（flag 不设 + agent 存活）：进程存活 + 零 boot/hidden/app.exit 行。
# 通过标准：退出码 0（AGENTS §5 烟测门槛）。
# 装置纪律：UTF-8 BOM（PS5.1 中文注释）；EnumWindows delegate 内禁 $pid（用 $procId）；
# OS.alert 为阻塞模态——脚本必须显式关闭对话框，finally 兜底杀进程防悬挂。

$ErrorActionPreference = "Stop"
if ([string]::IsNullOrWhiteSpace($RepoRoot)) { $RepoRoot = "D:\Vit_DAW_wt_uientry2" }

function Write-Step0 { param([string]$Msg) Write-Host ("--- " + $Msg) }
if (-not (Test-Path -LiteralPath $RepoRoot)) { throw ("RepoRoot not found: " + $RepoRoot) }
if ([string]::IsNullOrWhiteSpace($RunRoot)) {
    $RunRoot = Join-Path $RepoRoot ("coord\runs\UI-ENTRY-2\run_" + (Get-Date -Format "yyyyMMdd_HHmmss"))
}
New-Item -ItemType Directory -Force -Path $RunRoot | Out-Null
$LogsDir   = Join-Path $RunRoot "logs"
$ArtDir    = Join-Path $RunRoot "artifacts"
$FixtureDir = Join-Path $RunRoot "fixture"
$AgentDir  = Join-Path $RepoRoot "agent"
foreach ($dir in @($LogsDir, $ArtDir, $FixtureDir)) { New-Item -ItemType Directory -Force -Path $dir | Out-Null }

function Get-TcpListener { param([int]$Port) return Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue | Select-Object -First 1 }

function Get-PortOwnerPid { param([int]$Port)
    $conn = Get-TcpListener -Port $Port
    if ($null -ne $conn) { return [int]$conn.OwningProcess }
    return 0
}

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

function Test-LogAbsent {
    param([string]$LogPath, [string]$Pattern)
    if (-not (Test-Path -LiteralPath $LogPath)) { return $true }
    $hits = Select-String -LiteralPath $LogPath -Pattern $Pattern -ErrorAction SilentlyContinue
    return ($null -eq $hits -or @($hits).Count -eq 0)
}

function Stop-GuiProcess { param([int]$ProcId)
    if ($ProcId -gt 0) { Stop-Process -Id $ProcId -Force -ErrorAction SilentlyContinue }
}

function Test-ProcessAlive { param([int]$ProcId)
    if ($ProcId -le 0) { return $false }
    return ($null -ne (Get-Process -Id $ProcId -ErrorAction SilentlyContinue))
}

Add-Type -Namespace UiEntry2Smoke -Name Win32 -MemberDefinition @'
[DllImport("user32.dll")] public static extern bool EnumWindows(EnumProc cb, IntPtr lp);
public delegate bool EnumProc(IntPtr hWnd, IntPtr lp);
[DllImport("user32.dll", CharSet=CharSet.Unicode)] public static extern int GetClassNameW(IntPtr hWnd, System.Text.StringBuilder text, int count);
[DllImport("user32.dll", CharSet=CharSet.Unicode)] public static extern int GetWindowTextW(IntPtr hWnd, System.Text.StringBuilder text, int count);
[DllImport("user32.dll")] public static extern bool IsWindowVisible(IntPtr hWnd);
[DllImport("user32.dll")] public static extern uint GetWindowThreadProcessId(IntPtr hWnd, ref uint pid);
[DllImport("user32.dll")] public static extern bool PostMessageW(IntPtr hWnd, uint Msg, IntPtr wParam, IntPtr lParam);
'@

# 枚举全部顶层窗 → (hwnd,class,title,pid,visible)。UI-HOST-1 先例：FindWindowExW 跨
# 进程不可用，EnumWindows 是本环境标准枚举面。
function Get-AllWindows {
    $found = New-Object System.Collections.ArrayList
    $enumCb = [UiEntry2Smoke.Win32+EnumProc]{ param($h, $lp)
        $cn = New-Object System.Text.StringBuilder 256
        [void][UiEntry2Smoke.Win32]::GetClassNameW($h, $cn, 256)
        $sb = New-Object System.Text.StringBuilder 256
        [void][UiEntry2Smoke.Win32]::GetWindowTextW($h, $sb, 256)
        $procId = [uint32]0
        [void][UiEntry2Smoke.Win32]::GetWindowThreadProcessId($h, [ref]$procId)
        [void]$found.Add([pscustomobject]@{
            Hwnd = [long]$h
            Class = $cn.ToString()
            Title = $sb.ToString()
            Pid = [int]$procId
            Visible = [bool][UiEntry2Smoke.Win32]::IsWindowVisible($h)
        })
        return $true
    }
    [void][UiEntry2Smoke.Win32]::EnumWindows($enumCb, [IntPtr]::Zero)
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

# 原生对话框（OS.alert → MessageBoxW；#32770 为 Win32 标准对话框类）。
function Get-ErrorDialog {
    param([int]$GodotPid)
    foreach ($w in (Get-AllWindows)) {
        if ($w.Pid -ne $GodotPid) { continue }
        if ($w.Class -eq "#32770") { return $w }
    }
    return $null
}

function Send-WmClose { param([long]$Hwnd)
    [void][UiEntry2Smoke.Win32]::PostMessageW([IntPtr]$Hwnd, 0x0010, [IntPtr]::Zero, [IntPtr]::Zero)
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
    webview2_leg = [ordered]@{}
    legacy_leg = [ordered]@{}
    exit_code = 1
}
$assert = $report.assertions

Write-Host "=== UI-ENTRY-2 smoke; run dir: $RunRoot"
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
$ProjectCopy = Join-Path $FixtureDir ("ui_entry2_" + [System.IO.Path]::GetFileName($ProjectSource))
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

# kernel workspace 隔离（VIT_PROJECT_XML 指 run 目录副本；settings/cache/logs 落
# D:/Vit_DAW/VitApp/Workspace 为 354 既有行为，经 recents 快照+VIT_PROJECT_XML 隔离）。
$KernelWorkspace = Join-Path $RunRoot "kernel_workspace"
New-Item -ItemType Directory -Force -Path (Join-Path $KernelWorkspace "Settings") | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $KernelWorkspace "Logs") | Out-Null
$RepoDefaultProject = Join-Path $RepoRoot "VitApp\Workspace\default_project.xml"
if (Test-Path -LiteralPath $RepoDefaultProject) {
    Copy-Item -LiteralPath $RepoDefaultProject -Destination (Join-Path $KernelWorkspace "default_project.xml") -Force
}

# agent 构建：worktree 隔离二进制（autostart freshness 排序自动选中 worktree 最新构建；
# agent 语义零改动，构建只为隔离与 sha256 取证）。
$AgentBinary = Join-Path $RepoRoot "agent\bin\VitAgent.exe"
if (-not $SkipBuild) {
    Write-Step0 "Build VitAgent (isolated binary in worktree agent/bin)"
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
$assert["agent_binary"] = $AgentBinary
$assert["agent_binary_sha256"] = (Get-FileHash -Algorithm SHA256 -LiteralPath $AgentBinary).Hash

$godotFailInjectPid = 0
$godotWebview2Pid = 0
$godotAutoloadPid = 0
$godotLegacyPid = 0
$scriptAgentPid = 0

function Stop-Stack {
    Write-Step0 "Finally: stop any leftover stack and confirm port release"
    Stop-GuiProcess $godotFailInjectPid
    Stop-GuiProcess $godotWebview2Pid
    Stop-GuiProcess $godotAutoloadPid
    Stop-GuiProcess $godotLegacyPid
    Stop-GuiProcess $scriptAgentPid
    Start-Sleep -Seconds 1
    $deadline = (Get-Date).AddSeconds(30)
    while ((Get-Date) -lt $deadline) {
        $busy = @()
        foreach ($port in 7878, [int]$ZmqReqPort, 5556, [int]$VspHubPort) { if (Get-TcpListener -Port $port) { $busy += $port } }
        if ($busy.Count -eq 0) { break }
        foreach ($port in 7878, [int]$ZmqReqPort, 5556, [int]$VspHubPort) {
            $owner = Get-PortOwnerPid -Port $port
            if ($owner -gt 0) { Stop-Process -Id $owner -Force -ErrorAction SilentlyContinue }
        }
        Start-Sleep -Milliseconds 500
    }
    foreach ($port in 7878, [int]$ZmqReqPort, 5556, [int]$VspHubPort) {
        if (Get-TcpListener -Port $port) { $assert["port_still_busy_$port"] = $true }
    }
}

$agentScriptLog = Join-Path $LogsDir "agent_last.log"
$godotFailInjectLog = Join-Path $LogsDir "godot_failinject.log"
$godotWebview2Log = Join-Path $LogsDir "godot_webview2.log"
$godotAutoloadLog = Join-Path $LogsDir "godot_autoload.log"
$godotLegacyLog = Join-Path $LogsDir "godot_legacy.log"

function Start-GodotLeg {
    param([string]$LogPath, [string]$WorkingDir, [hashtable]$EnvVars)
    foreach ($key in $EnvVars.Keys) { Set-Item -Path ("Env:" + $key) -Value ([string]$EnvVars[$key]) }
    try {
        $proc = Start-Process -FilePath $GodotExe -ArgumentList @("--path", $GodotProjectRoot, "--log-file", $LogPath) -WorkingDirectory $WorkingDir -PassThru
    }
    finally {
        foreach ($key in $EnvVars.Keys) { Remove-Item -Path ("Env:" + $key) -ErrorAction SilentlyContinue }
    }
    return $proc
}

try {
    # =============================================== leg 1: fail-inject (agent unreachable, de-Godot)
    Write-Step0 "Leg 1 (fail-inject): autoload + NO agent -> health fails -> React window stays with in-window error, ZERO Godot start page"
    $leg1Env = @{
        "VIT_AGENT_WINDOW_MODE"  = "autoload"
        "VIT_SKIP_DEV_AUTOSTART" = "1"
        "VIT_DAW_DEV_ROOT"       = $RepoRoot
    }
    $godotProc1 = Start-GodotLeg -LogPath $godotFailInjectLog -WorkingDir $GodotProjectRoot -EnvVars $leg1Env
    $godotFailInjectPid = $godotProc1.Id
    $report.failinject_leg["godot_pid"] = $godotFailInjectPid

    # boot 行（React 窗建成——失败点是 agent health，不是建窗）。
    $fbBoot = Test-LogLine -LogPath $godotFailInjectLog -Pattern '\[AgentChatWindow\] boot ' -TimeoutSeconds $WaitSeconds
    if ($null -eq $fbBoot) { throw ("fail-inject leg: boot line not seen (window create unexpectedly failed)") }
    $report.failinject_leg["boot_line"] = "present"

    # main_window.hidden（启动隐藏照常——失败链不影响单窗启动语义）。
    $fbHidden = Test-LogLine -LogPath $godotFailInjectLog -Pattern '\[AgentChatWindow\] main_window\.hidden ' -TimeoutSeconds 15
    if ($null -eq $fbHidden) { throw ("fail-inject leg: main_window.hidden not seen") }
    $report.failinject_leg["hidden_line"] = "present"

    # health.failed + unavailable（真链失败注入：24×0.45s 上限 + 余量）。
    $fbHealthFail = Test-LogLine -LogPath $godotFailInjectLog -Pattern '\[AgentChatWindow\] health\.failed ' -TimeoutSeconds 30
    if ($null -eq $fbHealthFail) { throw ("fail-inject leg: health.failed not seen (agent reachable when it must not be)") }
    $fbUnavailable = Test-LogLine -LogPath $godotFailInjectLog -Pattern '\[AgentChatWindow\] unavailable ' -TimeoutSeconds 10
    if ($null -eq $fbUnavailable) { throw ("fail-inject leg: unavailable line not seen") }
    $report.failinject_leg["unavailable_line"] = ($fbUnavailable.Line -replace '^.*\[AgentChatWindow\] unavailable ', '').Trim()

    # UI-ENTRY-2 核心：零 main_window.restore（354 的 unavailable→Godot 回退链已退役）。
    Start-Sleep -Seconds 2
    $noRestore = Test-LogAbsent -LogPath $godotFailInjectLog -Pattern '\[AgentChatWindow\] main_window\.restore '
    $report.failinject_leg["zero_main_window_restore"] = $noRestore
    if (-not $noRestore) { throw ("fail-inject leg: main_window.restore seen — Godot start page fallback must be retired") }

    # navigate 行在场（错误态导航已尝试——窗内错误态=WebView2 连接失败页）。
    $fbNav = Test-LogLine -LogPath $godotFailInjectLog -Pattern '\[AgentChatWindow\] navigate ' -TimeoutSeconds 10
    $report.failinject_leg["navigate_line"] = ($null -ne $fbNav)
    if ($null -eq $fbNav) { throw ("fail-inject leg: navigate line not seen (error-state navigation not attempted)") }

    # EnumWindows：Godot 主窗不可见（启动页零出现）+ React 窗恰 1 可见（保持在场）。
    $dawWin1 = Get-DawMainWindow -GodotPid $godotFailInjectPid
    $chatWins1 = @((Get-AllWindows) | Where-Object { $_.Class -eq "VitWebViewHostChildWindow" -and $_.Visible })
    $report.failinject_leg["daw_window_visible"] = ($null -ne $dawWin1 -and $dawWin1.Visible)
    $report.failinject_leg["visible_chat_count"] = $chatWins1.Count
    if ($null -ne $dawWin1 -and $dawWin1.Visible) { throw ("fail-inject leg: Godot main window visible — start page fallback must never appear") }
    if ($chatWins1.Count -ne 1) { throw ("fail-inject leg: expected exactly 1 visible chat window, got $($chatWins1.Count)") }
    $assert["failinject_leg_pass"] = $true

    Stop-GuiProcess $godotFailInjectPid
    $godotFailInjectPid = 0
    Start-Sleep -Seconds 2

    # =============================================== leg 2: WebView2 unavailable (system error box + exit)
    Write-Step0 "Leg 2 (webview2): VIT_SMOKE_WEBVIEW2_FAIL=1 -> webview2.unavailable + native error dialog + process exit"
    $leg2Env = @{
        "VIT_AGENT_WINDOW_MODE"  = "autoload"
        "VIT_SMOKE_WEBVIEW2_FAIL" = "1"
        "VIT_SKIP_DEV_AUTOSTART" = "1"
        "VIT_DAW_DEV_ROOT"       = $RepoRoot
    }
    $godotProc2 = Start-GodotLeg -LogPath $godotWebview2Log -WorkingDir $GodotProjectRoot -EnvVars $leg2Env
    $godotWebview2Pid = $godotProc2.Id
    $report.webview2_leg["godot_pid"] = $godotWebview2Pid

    $wvLine = Test-LogLine -LogPath $godotWebview2Log -Pattern '\[AgentChatWindow\] webview2\.unavailable ' -TimeoutSeconds 30
    if ($null -eq $wvLine) { throw ("webview2 leg: webview2.unavailable line not seen") }
    $report.webview2_leg["unavailable_line"] = ($wvLine.Line -replace '^.*\[AgentChatWindow\] webview2\.unavailable ', '').Trim()

    Start-Sleep -Seconds 1
    $noBoot2 = Test-LogAbsent -LogPath $godotWebview2Log -Pattern '\[AgentChatWindow\] boot '
    $report.webview2_leg["zero_boot_line"] = $noBoot2
    if (-not $noBoot2) { throw ("webview2 leg: boot line seen — create path must abort before boot evidence") }

    # 原生错误对话框（OS.alert 阻塞模态；轮询 ≤20s）。
    $dialog = $null
    $dlgDeadline = (Get-Date).AddSeconds(20)
    while ((Get-Date) -lt $dlgDeadline) {
        $dialog = Get-ErrorDialog -GodotPid $godotWebview2Pid
        if ($null -ne $dialog) { break }
        Start-Sleep -Milliseconds 300
    }
    $report.webview2_leg["error_dialog_found"] = ($null -ne $dialog)
    if ($null -eq $dialog) { throw ("webview2 leg: native error dialog (#32770) not found within 20s") }
    $report.webview2_leg["error_dialog_hwnd"] = $dialog.Hwnd

    # 对话框在位时：零 chat 子窗（建窗路径中止的直接证据）。
    $chatWins2 = @((Get-AllWindows) | Where-Object { $_.Class -eq "VitWebViewHostChildWindow" })
    $report.webview2_leg["chat_window_count"] = $chatWins2.Count
    if ($chatWins2.Count -ne 0) { throw ("webview2 leg: chat window exists although WebView2 path aborted") }

    # WM_CLOSE 关闭对话框（窗口消息自动化，354 UI-OS-1 同先例）→ OS.alert 返回 → quit。
    Send-WmClose -Hwnd $dialog.Hwnd
    $exitDeadline = (Get-Date).AddSeconds(15)
    $exited = $false
    while ((Get-Date) -lt $exitDeadline) {
        if (-not (Test-ProcessAlive -ProcId $godotWebview2Pid)) { $exited = $true; break }
        Start-Sleep -Milliseconds 300
    }
    $report.webview2_leg["process_exited"] = $exited
    if (-not $exited) { throw ("webview2 leg: Godot process did not exit after dialog dismissal") }
    $assert["webview2_leg_pass"] = $true
    $godotWebview2Pid = 0
    Start-Sleep -Seconds 2

    # =============================================== leg 3: autoload full chain + close = exit whole app
    Write-Step0 "Leg 3 (autoload): autostart full stack -> single window -> open project -> WM_CLOSE -> ALL processes exit"
    $leg3Env = @{
        "VIT_AGENT_WINDOW_MODE"     = "autoload"
        "VIT_SMOKE_AUTO_OPEN_PROJECT" = $ProjectCopy
        "VIT_DAW_DEV_ROOT"          = $RepoRoot
        "VIT_PROJECT_XML"           = (Join-Path $KernelWorkspace "default_project.xml")
        "VIT_HISTORY_DRAFT_ROOT"    = (Join-Path $RunRoot "agent_drafts")
        "VIT_AGENT_LAST_LOG_PATH"   = $agentScriptLog
    }
    New-Item -ItemType Directory -Force -Path (Join-Path $RunRoot "agent_drafts") | Out-Null
    $godotProc3 = Start-GodotLeg -LogPath $godotAutoloadLog -WorkingDir $GodotProjectRoot -EnvVars $leg3Env
    $godotAutoloadPid = $godotProc3.Id
    $assert["godot_autoload_pid"] = $godotAutoloadPid

    # boot 行。
    $bootHit = Test-LogLine -LogPath $godotAutoloadLog -Pattern '\[AgentChatWindow\] boot ' -TimeoutSeconds $WaitSeconds
    if ($null -eq $bootHit) { throw ("AgentChatWindow boot line not seen in " + $godotAutoloadLog) }
    $assert["boot_line"] = ($bootHit.Line -replace '^.*\[AgentChatWindow\] boot ', '').Trim()

    # main_window.hidden（React 窗建成 → Godot 主窗启动隐藏）。
    $hiddenHit = Test-LogLine -LogPath $godotAutoloadLog -Pattern '\[AgentChatWindow\] main_window\.hidden ' -TimeoutSeconds 15
    if ($null -eq $hiddenHit) { throw ("main_window.hidden not seen (Godot main window was not hidden at startup)") }
    $assert["main_window_hidden_line"] = ($hiddenHit.Line -replace '^.*\[AgentChatWindow\] main_window\.hidden ', '').Trim()

    # autostart 链证据（start_page 经 OS.create_process 拉起 kernel/agent 并登记）。
    $regKernel = Test-LogLine -LogPath $godotAutoloadLog -Pattern 'VitIpcClient: registered autostart child kernel ' -TimeoutSeconds 20
    $regAgent = Test-LogLine -LogPath $godotAutoloadLog -Pattern 'VitIpcClient: registered autostart child agent ' -TimeoutSeconds 20
    $assert["autostart_kernel_registered"] = ($null -ne $regKernel)
    $assert["autostart_agent_registered"] = ($null -ne $regAgent)
    if (-not $assert["autostart_kernel_registered"] -or -not $assert["autostart_agent_registered"]) {
        throw ("autostart chain evidence missing: kernel=$($assert["autostart_kernel_registered"]) agent=$($assert["autostart_agent_registered"])")
    }

    # visible 行（child_hwnd 留证）。
    $visHit = Test-LogLine -LogPath $godotAutoloadLog -Pattern '\[AgentChatWindow\] visible ' -TimeoutSeconds 30
    if ($null -eq $visHit) { throw ("AgentChatWindow visible line not seen") }
    $childHwnd = 0
    try { $childHwnd = [long](($visHit.Line | Select-String -Pattern '"child_hwnd":\s*(-?\d+)' -AllMatches).Matches[0].Groups[1].Value) } catch { $childHwnd = 0 }
    if ($childHwnd -le 0) { throw ("child_hwnd missing/invalid in visible line") }
    $assert["child_hwnd"] = $childHwnd

    # 启动态单窗断言：hidden 行后轮询枚举——静置窗口内（React 页加载中、open 链未启动）
    # 主窗必须不可见+React 启动台恰 1 个可见（354 run_20261114_233618 竞速教训：断言
    # 锚点前移到 hidden 行，不落在 page_loaded 之后）。
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

    # 断言 1：同一 HWND 跨场景切换+进工程展开。
    $assert["same_hwnd_across_entry"] = ($childHwnd -eq $doneHwnd -and $doneHwnd -eq $sceneHwnd -and $sceneHwnd -eq $presentedHwnd -and $doneHwnd -gt 0)
    # 断言 2：navigate_count == 1（surface 切换零导航，343 不变量）。
    $assert["navigate_count_is_1"] = ($doneNavCount -eq 1 -and $presentedNav -eq 1)
    # 断言 3：URL 未变。
    $assert["page_url_unchanged_after_entry"] = ($presentedUrl -like "http://127.0.0.1:7878/app/*")
    # 断言 4：Win32 枚举的 chat 窗 hwnd 与日志一致且可见。
    $chatWinAfter = @((Get-AllWindows) | Where-Object { $_.Class -eq "VitWebViewHostChildWindow" })
    $assert["chat_window_count_after_entry"] = $chatWinAfter.Count
    $assert["chat_window_visible_after_entry"] = ($chatWinAfter.Count -eq 1 -and $chatWinAfter[0].Visible -and ([long]$chatWinAfter[0].Hwnd) -eq $childHwnd)

    if (-not $assert["same_hwnd_across_entry"]) { throw ("HWND changed across entry: child=$childHwnd done=$doneHwnd scene=$sceneHwnd presented=$presentedHwnd") }
    if (-not $assert["navigate_count_is_1"]) { throw ("page reloaded on surface switch: done=$doneNavCount presented=$presentedNav") }
    if (-not $assert["page_url_unchanged_after_entry"]) { throw ("page URL changed: $presentedUrl") }
    if (-not $assert["chat_window_visible_after_entry"]) { throw ("chat window missing/hidden/inconsistent after entry (count=$($chatWinAfter.Count))") }

    # ------------------------------------------------- UI-ENTRY-2 core: close = exit whole app
    # pid 端口归属实测（不依赖 autostart 登记日志的 pid 解析——独立证据面）。
    $agentPidViaPort = Get-PortOwnerPid -Port $agentPort
    $kernelPidViaPort = Get-PortOwnerPid -Port ([int]$ZmqReqPort)
    $assert["agent_pid_via_port"] = $agentPidViaPort
    $assert["kernel_pid_via_port"] = $kernelPidViaPort
    if ($agentPidViaPort -le 0) { throw ("agent pid not found via port $agentPort ownership") }
    if ($kernelPidViaPort -le 0) { throw ("kernel pid not found via port $ZmqReqPort ownership") }
    if ($agentPidViaPort -eq $godotAutoloadPid -or $kernelPidViaPort -eq $godotAutoloadPid) { throw ("port owner pid collides with godot pid") }
    $assert["agent_alive_before_close"] = (Test-ProcessAlive -ProcId $agentPidViaPort)
    $assert["kernel_alive_before_close"] = (Test-ProcessAlive -ProcId $kernelPidViaPort)

    Write-Step0 "UI-ENTRY-2 core: PostMessage WM_CLOSE to chat window -> whole app must exit"
    Send-WmClose -Hwnd $childHwnd

    $exitDeadline3 = (Get-Date).AddSeconds(25)
    $godotExited3 = $false
    while ((Get-Date) -lt $exitDeadline3) {
        if (-not (Test-ProcessAlive -ProcId $godotAutoloadPid)) { $godotExited3 = $true; break }
        Start-Sleep -Milliseconds 300
    }
    $assert["close_godot_exited"] = $godotExited3
    if (-not $godotExited3) { throw ("close-window leg: Godot did not exit after WM_CLOSE (close must exit whole app)") }

    # Godot 退出后：agent/kernel 由 VitIpcClient._exit_tree 终止（autostart 登记面）。
    Start-Sleep -Seconds 2
    $agentExited3 = -not (Test-ProcessAlive -ProcId $agentPidViaPort)
    $kernelExited3 = -not (Test-ProcessAlive -ProcId $kernelPidViaPort)
    $assert["close_agent_exited"] = $agentExited3
    $assert["close_kernel_exited"] = $kernelExited3
    if (-not $agentExited3) { throw ("close-window leg: agent (pid=$agentPidViaPort) still alive after Godot exit") }
    if (-not $kernelExited3) { throw ("close-window leg: kernel (pid=$kernelPidViaPort) still alive after Godot exit") }

    # 端口释放终证。
    $portsBusy3 = @()
    foreach ($port in 7878, [int]$ZmqReqPort, 5556, [int]$VspHubPort) { if (Get-TcpListener -Port $port) { $portsBusy3 += $port } }
    $assert["close_ports_released"] = ($portsBusy3.Count -eq 0)
    if ($portsBusy3.Count -gt 0) { throw ("close-window leg: ports still busy after exit: $($portsBusy3 -join ',')") }
    $assert["autoload_leg_pass"] = $true
    $godotAutoloadPid = 0

    # ---------------------------------------------------------- legacy leg
    if (-not $SkipLegacyLeg) {
        Write-Step0 "Leg 4 (legacy): flag unset keeps the Godot start page, zero new orchestration lines"
        # agent 先行启动（与 354 legacy 腿环境等价：agent 在场、start_page 不 spawn 桥）。
        $env:VIT_HISTORY_DRAFT_ROOT = (Join-Path $RunRoot "agent_drafts")
        $env:VIT_DAW_DEV_ROOT = $RepoRoot
        try {
            $agentProc = Start-Process -FilePath $AgentBinary -ArgumentList @("-http", ($AgentHttp -replace '^http://',''), "-last-log-path", $agentScriptLog, "-keep-last-log-lines", "8000") -WorkingDirectory $AgentDir -WindowStyle Hidden -PassThru
        }
        finally {
            Remove-Item Env:VIT_HISTORY_DRAFT_ROOT -ErrorAction SilentlyContinue
            Remove-Item Env:VIT_DAW_DEV_ROOT -ErrorAction SilentlyContinue
        }
        $scriptAgentPid = $agentProc.Id
        $assert["legacy_agent_pid"] = $scriptAgentPid
        if (-not (Wait-HttpReady -BaseUrl $AgentHttp -TimeoutSeconds $WaitSeconds)) {
            throw ("agent HTTP did not become ready for legacy leg")
        }
        $assert["legacy_agent_http_ready"] = $true

        $leg4Env = @{
            "VIT_SKIP_DEV_AUTOSTART" = "1"
            "VIT_DAW_DEV_ROOT"       = $RepoRoot
        }
        $godotProc4 = Start-GodotLeg -LogPath $godotLegacyLog -WorkingDir $GodotProjectRoot -EnvVars $leg4Env
        $godotLegacyPid = $godotProc4.Id
        $report.legacy_leg["godot_pid"] = $godotLegacyPid
        Start-Sleep -Seconds 20
        $legacyBoot = $null
        $legacyHidden = $null
        $legacyExit = $null
        if (Test-Path -LiteralPath $godotLegacyLog) {
            $legacyBoot = Select-String -LiteralPath $godotLegacyLog -Pattern '\[AgentChatWindow\] boot ' -ErrorAction SilentlyContinue | Select-Object -First 1
            $legacyHidden = Select-String -LiteralPath $godotLegacyLog -Pattern '\[AgentChatWindow\] main_window\.hidden ' -ErrorAction SilentlyContinue | Select-Object -First 1
            $legacyExit = Select-String -LiteralPath $godotLegacyLog -Pattern '\[AgentChatWindow\] app\.exit ' -ErrorAction SilentlyContinue | Select-Object -First 1
        }
        $report.legacy_leg["no_boot_line"] = ($null -eq $legacyBoot)
        $report.legacy_leg["no_main_window_hidden_line"] = ($null -eq $legacyHidden)
        $report.legacy_leg["no_app_exit_line"] = ($null -eq $legacyExit)
        $legacyProcAlive = $true
        try { $legacyProcAlive = (-not $godotProc4.HasExited) } catch { $legacyProcAlive = $false }
        $report.legacy_leg["process_alive"] = $legacyProcAlive
        if (-not $report.legacy_leg["no_boot_line"]) { throw ("legacy mode unexpectedly created the chat window") }
        if (-not $report.legacy_leg["no_main_window_hidden_line"]) { throw ("legacy mode unexpectedly hid the Godot main window") }
        if (-not $report.legacy_leg["no_app_exit_line"]) { throw ("legacy mode unexpectedly logged app.exit") }
        if (-not $legacyProcAlive) { throw ("legacy-mode Godot exited early") }
        $assert["legacy_leg_pass"] = $true
        Stop-GuiProcess $godotLegacyPid
        $godotLegacyPid = 0
        Start-Sleep -Seconds 2
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
    $reportPath = Join-Path $ArtDir "ui_entry2_smoke_report.json"
    $report | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $reportPath -Encoding UTF8
    Write-Host ("report: " + $reportPath)
    foreach ($log in @($godotFailInjectLog, $godotWebview2Log, $godotAutoloadLog, $godotLegacyLog, $agentScriptLog)) {
        Copy-Item -LiteralPath $log -Destination $ArtDir -Force -ErrorAction SilentlyContinue
    }
}

if ($report.exit_code -eq 0) { Write-Host "UI-ENTRY-2 smoke: PASS" ; exit 0 }
else { Write-Host "UI-ENTRY-2 smoke: FAIL" ; exit 1 }
