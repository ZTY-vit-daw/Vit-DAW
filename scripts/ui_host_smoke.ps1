[CmdletBinding()]
param(
    [string]$RepoRoot = "",
    [string]$AgentHttp = "http://127.0.0.1:7878",
    [string]$ZmqReqPort = "5555",
    [string]$VspHubPort = "8787",
    [string]$KernelExe = "",
    [string]$AgentBinary = "",
    [string]$GodotExe = "D:\Godot\Godot_v4.6.1-stable_win64.exe",
    # 被测前端 = UI-HOST-1 的前端 worktree（改动在 port/ui-host-1 分支 + 重编 extension）。
    [string]$GodotProjectRoot = "D:\Godot\vit-daw-frontend_wt_uihost1",
    [string]$ProjectSource = "D:\Godot\project\vit-daw-frontend\912.vit",
    [string]$RunRoot = "",
    [int]$WaitSeconds = 60,
    [int]$SceneSettleSeconds = 40,
    [switch]$SkipBuild,
    [switch]$SkipLegacyLeg
)

# UI-HOST-1 端侧烟测（同模式新脚本；模式来源=journey1_demo_journey_smoke.ps1）。
# 断言面：
#   A) autoload 腿：真实三件套（VitApp 内核 + Godot 前端 worktree + go build VitAgent），
#      VIT_AGENT_WINDOW_MODE=autoload 下 React 顶层窗先出（boot 行含 child_hwnd）；
#      VIT_SMOKE_AUTO_OPEN_PROJECT 驱动 start_page 真实 open 链进入工程；
#      scene_added(VitDockRoot) 后 child_hwnd 不变 + navigate_count==1（同一 HWND/同一页面未重载）。
#   B) OS 基线：窗口标题=="Vit"、类名 VitWebViewHostChildWindow 恰好 1 个（无重复窗）、
#      最小化(SW_MINIMIZE)/召回(SW_RESTORE) 经 Win32 断言 + 句柄留证。
#   C) legacy 腿（-SkipLegacyLeg 跳过）：VIT_AGENT_WINDOW_MODE=legacy 时无 boot 行，
#      启动页进程存活（fallback 可用）。
# 通过标准：退出码 0（AGENTS §5 烟测门槛）。

$ErrorActionPreference = "Stop"
if ([string]::IsNullOrWhiteSpace($RepoRoot)) { $RepoRoot = "D:\Vit_DAW_wt_uihost1" }

function Write-Step0 { param([string]$Msg) Write-Host ("--- " + $Msg) }
if (-not (Test-Path -LiteralPath $RepoRoot)) { throw ("RepoRoot not found: " + $RepoRoot) }
if ([string]::IsNullOrWhiteSpace($RunRoot)) {
    $RunRoot = Join-Path $RepoRoot ("coord\runs\UI-HOST-1\run_" + (Get-Date -Format "yyyyMMdd_HHmmss"))
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

# ---------------------------------------------------------------- prereq
$report = [ordered]@{
    run_root = $RunRoot
    started_utc = (Get-Date).ToUniversalTime().ToString("o")
    head = (& git -C $RepoRoot rev-parse HEAD)
    frontend_head = (& git -C $GodotProjectRoot rev-parse HEAD)
    godot_project_root = $GodotProjectRoot
    project_source = $ProjectSource
    assertions = [ordered]@{}
    legacy_leg = [ordered]@{}
    exit_code = 1
}
$assert = $report.assertions

Write-Host "=== UI-HOST-1 smoke; run dir: $RunRoot"
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
$ProjectCopy = Join-Path $FixtureDir ("ui_host_" + [System.IO.Path]::GetFileName($ProjectSource))
Copy-Item -LiteralPath $ProjectSource -Destination $ProjectCopy -Force
$fixtureHashBefore = (Get-FileHash -Algorithm SHA256 -LiteralPath $ProjectSource).Hash
$assert["fixture_copy"] = $ProjectCopy
$assert["fixture_source_sha256"] = $fixtureHashBefore

# kernel workspace 隔离（VIT_PROJECT_XML 指 run 目录副本）。
$KernelWorkspace = Join-Path $RunRoot "kernel_workspace"
New-Item -ItemType Directory -Force -Path (Join-Path $KernelWorkspace "Settings") | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $KernelWorkspace "Logs") | Out-Null
$RepoDefaultProject = Join-Path $RepoRoot "VitApp\Workspace\default_project.xml"
if (Test-Path -LiteralPath $RepoDefaultProject) {
    Copy-Item -LiteralPath $RepoDefaultProject -Destination (Join-Path $KernelWorkspace "default_project.xml") -Force
}

# agent 构建（隔离二进制放 run 目录）。
if ([string]::IsNullOrWhiteSpace($AgentBinary)) { $AgentBinary = Join-Path $RunRoot "bin\VitAgent.ui_host.exe" }
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
$godotLegacyLog = Join-Path $LogsDir "godot_legacy.log"

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
    }
    $kernelProcId = $kernelProc.Id
    $assert["kernel_pid"] = $kernelProcId
    if (-not (Wait-PortListen -Port ([int]$ZmqReqPort) -TimeoutSeconds $WaitSeconds)) {
        throw ("kernel ZMQ REQ port $ZmqReqPort did not listen")
    }
    $assert["kernel_zmq_listening"] = $true

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
    }
    $agentProcId = $agentProc.Id
    $assert["agent_pid"] = $agentProcId
    if (-not (Wait-HttpReady -BaseUrl $AgentHttp -TimeoutSeconds $WaitSeconds)) {
        throw ("agent HTTP did not become ready")
    }
    $assert["agent_http_ready"] = $true

    # ------------------------------------------------- start Godot (autoload)
    Write-Step0 "Import pass (rebuild .godot cache in a fresh worktree: extension_list + global class cache)"
    $importArgs = @("--headless", "--path", $GodotProjectRoot, "--import")
    $importProc = Start-Process -FilePath $GodotExe -ArgumentList $importArgs -WorkingDirectory $GodotProjectRoot -PassThru -Wait -WindowStyle Hidden
    if (-not (Test-Path -LiteralPath (Join-Path $GodotProjectRoot ".godot\extension_list.cfg"))) {
        throw ("Godot import pass did not produce .godot/extension_list.cfg")
    }
    $assert["godot_import_pass"] = $true

    Write-Step0 "Start Godot (VIT_AGENT_WINDOW_MODE=autoload + smoke open driver)"
    $env:VIT_AGENT_WINDOW_MODE = "autoload"
    $env:VIT_SMOKE_AUTO_OPEN_PROJECT = $ProjectCopy
    $env:VIT_SKIP_DEV_AUTOSTART = "1"
    $env:VIT_DAW_DEV_ROOT = $RepoRoot
    try {
        $godotProc = Start-Process -FilePath $GodotExe -ArgumentList @("--path", $GodotProjectRoot, "--log-file", $godotAutoloadLog) -WorkingDirectory $GodotProjectRoot -PassThru
    }
    finally {
        Remove-Item Env:VIT_AGENT_WINDOW_MODE -ErrorAction SilentlyContinue
        Remove-Item Env:VIT_SMOKE_AUTO_OPEN_PROJECT -ErrorAction SilentlyContinue
        Remove-Item Env:VIT_SKIP_DEV_AUTOSTART -ErrorAction SilentlyContinue
    }
    $godotAutoloadPid = $godotProc.Id
    $assert["godot_autoload_pid"] = $godotAutoloadPid

    # boot 行：React 顶层窗先出（child_hwnd 留证）。
    $bootHit = Test-LogLine -LogPath $godotAutoloadLog -Pattern '\[AgentChatWindow\] boot ' -TimeoutSeconds $WaitSeconds
    if ($null -eq $bootHit) { throw ("AgentChatWindow boot line not seen in " + $godotAutoloadLog) }
    $bootJson = ($bootHit.Line -replace '^.*\[AgentChatWindow\] boot ', '').Trim()
    $assert["boot_line"] = $bootJson
    $hwndA = 0
    try { $hwndA = [long](($bootHit.Line | Select-String -Pattern '"main_hwnd":\s*(-?\d+)' -AllMatches).Matches[0].Groups[1].Value) } catch { $hwndA = 0 }
    $assert["boot_main_hwnd"] = $hwndA

    # visible 行（set_visible 后窗口可见）。
    $visHit = Test-LogLine -LogPath $godotAutoloadLog -Pattern '\[AgentChatWindow\] visible ' -TimeoutSeconds 30
    if ($null -eq $visHit) { throw ("AgentChatWindow visible line not seen") }
    $assert["visible_line"] = ($visHit.Line -replace '^.*\[AgentChatWindow\] visible ', '').Trim()
    $childHwnd = 0
    try { $childHwnd = [long](($visHit.Line | Select-String -Pattern '"child_hwnd":\s*(-?\d+)' -AllMatches).Matches[0].Groups[1].Value) } catch { $childHwnd = 0 }
    if ($childHwnd -le 0) { throw ("child_hwnd missing/invalid in visible line") }
    $assert["child_hwnd"] = $childHwnd

    # page_loaded 行（React 页面在 autoload 窗内加载完成；navigate 只发生 1 次）。
    $pageHit = Test-LogLine -LogPath $godotAutoloadLog -Pattern '\[AgentChatWindow\] page_loaded ' -TimeoutSeconds 90
    if ($null -eq $pageHit) { throw ("AgentChatWindow page_loaded line not seen (React did not load)") }
    $assert["page_loaded_line"] = ($pageHit.Line -replace '^.*\[AgentChatWindow\] page_loaded ', '').Trim()

    # 驱动钩子完成（start_page open 链 → _enter_main_scene → dock）。
    $doneHit = Test-LogLine -LogPath $godotAutoloadLog -Pattern '\[AgentChatWindow\] smoke\.auto_open\.done ' -TimeoutSeconds $SceneSettleSeconds
    if ($null -eq $doneHit) { throw ("smoke.auto_open.done not seen within ${SceneSettleSeconds}s (open chain failed)") }
    $assert["auto_open_done_line"] = ($doneHit.Line -replace '^.*\[AgentChatWindow\] smoke\.auto_open\.done ', '').Trim()

    # scene_added(VitDockRoot) 行（场景切换后 HWND/URL 取证）。
    $sceneHit = Test-LogLine -LogPath $godotAutoloadLog -Pattern 'scene_added.*VitDockRoot' -TimeoutSeconds $SceneSettleSeconds
    if ($null -eq $sceneHit) { throw ("scene_added VitDockRoot not seen (scene switch did not happen)") }
    $assert["scene_added_line"] = $sceneHit.Line

    $sceneHwnd = 0
    $sceneHwndMatch = ($sceneHit.Line | Select-String -Pattern '"child_hwnd":\s*(-?\d+)' -AllMatches).Matches
    if ($sceneHwndMatch.Count -gt 0) { $sceneHwnd = [long]$sceneHwndMatch[0].Groups[1].Value }
    $doneHwnd = 0
    $doneHwndMatch = ($doneHit.Line | Select-String -Pattern '"child_hwnd":\s*(-?\d+)' -AllMatches).Matches
    if ($doneHwndMatch.Count -gt 0) { $doneHwnd = [long]$doneHwndMatch[0].Groups[1].Value }
    $doneNavCount = -1
    $navMatch = ($doneHit.Line | Select-String -Pattern '"navigate_count":\s*(\d+)' -AllMatches).Matches
    if ($navMatch.Count -gt 0) { $doneNavCount = [int]$navMatch[0].Groups[1].Value }

    # 断言 1：同一 HWND 跨场景切换（boot child_hwnd == done == scene_added）。
    $assert["same_hwnd_across_scene_switch"] = ($childHwnd -eq $doneHwnd -and $doneHwnd -eq $sceneHwnd -and $doneHwnd -gt 0)
    # 断言 2：navigate_count == 1（页面未重载）。
    $assert["navigate_count_is_1"] = ($doneNavCount -eq 1)
    # 断言 3：URL 未变（done 证据行 page_url == /app/ 页）。
    $doneUrl = ""
    $urlMatch = ($doneHit.Line | Select-String -Pattern '"page_url":\s*"([^"]*)"' -AllMatches).Matches
    if ($urlMatch.Count -gt 0) { $doneUrl = $urlMatch[0].Groups[1].Value }
    $assert["page_url_after_open"] = $doneUrl
    $assert["page_url_unchanged"] = ($doneUrl -like "http://127.0.0.1:7878/app/*")

    if (-not $assert["same_hwnd_across_scene_switch"]) { throw ("HWND changed across scene switch: child=$childHwnd done=$doneHwnd scene=$sceneHwnd") }
    if (-not $assert["navigate_count_is_1"]) { throw ("page reloaded: navigate_count=$doneNavCount") }
    if (-not $assert["page_url_unchanged"]) { throw ("page URL changed: $doneUrl") }

    # ---------------------------------------------------------- OS baseline
    Write-Step0 "OS baseline: title/class/count/minimize/recall via Win32"
    Add-Type -Namespace UiHostSmoke -Name Win32 -MemberDefinition @'
[DllImport("user32.dll")] public static extern bool EnumWindows(EnumProc cb, IntPtr lp);
public delegate bool EnumProc(IntPtr hWnd, IntPtr lp);
[DllImport("user32.dll", CharSet=CharSet.Unicode)] public static extern int GetClassNameW(IntPtr hWnd, System.Text.StringBuilder text, int count);
[DllImport("user32.dll", CharSet=CharSet.Unicode)] public static extern int GetWindowTextW(IntPtr hWnd, System.Text.StringBuilder text, int count);
[DllImport("user32.dll")] public static extern bool IsWindowVisible(IntPtr hWnd);
[DllImport("user32.dll")] public static extern bool IsIconic(IntPtr hWnd);
[DllImport("user32.dll")] public static extern bool ShowWindow(IntPtr hWnd, int nCmdShow);
[DllImport("user32.dll")] public static extern IntPtr GetForegroundWindow();
'@
    # 类名枚举（EnumWindows+GetClassName 过滤——FindWindowExW 在本环境对跨进程
    # 顶层窗返回 0 未命中，EnumWindows 为标准枚举面）：VitWebViewHostChildWindow
    # 顶层窗恰好 1 个（无重复窗）。
    $found = New-Object System.Collections.ArrayList
    $enumCb = [UiHostSmoke.Win32+EnumProc]{ param($h, $lp)
        $cn = New-Object System.Text.StringBuilder 256
        [void][UiHostSmoke.Win32]::GetClassNameW($h, $cn, 256)
        if ($cn.ToString() -eq "VitWebViewHostChildWindow") { [void]$found.Add($h) }
        return $true
    }
    [void][UiHostSmoke.Win32]::EnumWindows($enumCb, [IntPtr]::Zero)
    $assert["chat_window_count"] = $found.Count
    $assert["no_duplicate_window"] = ($found.Count -eq 1)
    if ($found.Count -ne 1) { throw ("expected exactly 1 VitWebViewHostChildWindow top-level window, got $($found.Count)") }
    $chatHwnd = $found[0]
    $assert["chat_hwnd_win32"] = ([long]$chatHwnd)
    if (([long]$chatHwnd) -ne $childHwnd) { throw ("Win32 enum hwnd $chatHwnd != log child_hwnd $childHwnd") }
    $assert["win32_hwnd_matches_log"] = $true

    $sb = New-Object System.Text.StringBuilder 256
    [void][UiHostSmoke.Win32]::GetWindowTextW($chatHwnd, $sb, 256)
    $title = $sb.ToString()
    $assert["window_title"] = $title
    $assert["title_is_vit"] = ($title -eq "Vit")
    if ($title -ne "Vit") { throw ("window title is '$title', expected 'Vit'") }
    $assert["window_visible_win32"] = [bool][UiHostSmoke.Win32]::IsWindowVisible($chatHwnd)

    # 最小化/召回（对烟测自启的被测窗 ShowWindow——Start-Process 同性质的自动化面）。
    [void][UiHostSmoke.Win32]::ShowWindow($chatHwnd, 6)  # SW_MINIMIZE
    Start-Sleep -Milliseconds 800
    $minimized = [bool][UiHostSmoke.Win32]::IsIconic($chatHwnd)
    $assert["minimized_ok"] = $minimized
    [void][UiHostSmoke.Win32]::ShowWindow($chatHwnd, 9)  # SW_RESTORE
    Start-Sleep -Milliseconds 800
    $restoredVisible = [bool][UiHostSmoke.Win32]::IsWindowVisible($chatHwnd) -and -not [bool][UiHostSmoke.Win32]::IsIconic($chatHwnd)
    $assert["recall_visible_ok"] = $restoredVisible
    if (-not $minimized) { throw ("SW_MINIMIZE did not iconic the chat window") }
    if (-not $restoredVisible) { throw ("SW_RESTORE did not bring the chat window back visible") }

    # 初始前台探针（soft）：启动后窗口若仍可见即作前台/层序可达证据；前台锁受限时记录 missed。
    $fg = [UiHostSmoke.Win32]::GetForegroundWindow()
    $assert["foreground_probe_hwnd"] = ([long]$fg)
    $assert["foreground_probe_is_chat"] = ($fg -eq $chatHwnd)

    # ---------------------------------------------------------- legacy leg
    if (-not $SkipLegacyLeg) {
        Write-Step0 "Legacy leg: VIT_AGENT_WINDOW_MODE=legacy keeps the Godot start page"
        Stop-GuiProcess $godotAutoloadPid
        $godotAutoloadPid = 0
        Start-Sleep -Seconds 2
        $env:VIT_AGENT_WINDOW_MODE = "legacy"
        $env:VIT_SKIP_DEV_AUTOSTART = "1"
        try {
            $godotLegacyProc = Start-Process -FilePath $GodotExe -ArgumentList @("--path", $GodotProjectRoot, "--log-file", $godotLegacyLog) -WorkingDirectory $GodotProjectRoot -PassThru
        }
        finally {
            Remove-Item Env:VIT_AGENT_WINDOW_MODE -ErrorAction SilentlyContinue
            Remove-Item Env:VIT_SKIP_DEV_AUTOSTART -ErrorAction SilentlyContinue
        }
        $godotLegacyPid = $godotLegacyProc.Id
        $assert["godot_legacy_pid"] = $godotLegacyPid
        Start-Sleep -Seconds 12
        $legacyBoot = $null
        if (Test-Path -LiteralPath $godotLegacyLog) {
            $legacyBoot = Select-String -LiteralPath $godotLegacyLog -Pattern '\[AgentChatWindow\] boot ' -ErrorAction SilentlyContinue | Select-Object -First 1
        }
        $assert["legacy_no_boot_line"] = ($null -eq $legacyBoot)
        $legacyProcAlive = $true
        try { $legacyProcAlive = (-not $godotLegacyProc.HasExited) } catch { $legacyProcAlive = $false }
        $assert["legacy_process_alive"] = $legacyProcAlive
        if (-not $assert["legacy_no_boot_line"]) { throw ("legacy mode unexpectedly created the chat window") }
        if (-not $legacyProcAlive) { throw ("legacy-mode Godot exited early") }
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
    $reportPath = Join-Path $ArtDir "ui_host_smoke_report.json"
    $report | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $reportPath -Encoding UTF8
    Write-Host ("report: " + $reportPath)
    Copy-Item -LiteralPath $godotAutoloadLog -Destination $ArtDir -Force -ErrorAction SilentlyContinue
    Copy-Item -LiteralPath $godotLegacyLog -Destination $ArtDir -Force -ErrorAction SilentlyContinue
    Copy-Item -LiteralPath $agentLog -Destination $ArtDir -Force -ErrorAction SilentlyContinue
}

if ($report.exit_code -eq 0) { Write-Host "UI-HOST-1 smoke: PASS" ; exit 0 }
else { Write-Host "UI-HOST-1 smoke: FAIL" ; exit 1 }
