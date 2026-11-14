[CmdletBinding()]
param(
    [string]$RepoRoot = "",
    [string]$AgentHttp = "http://127.0.0.1:7878",
    [string]$ZmqReqPort = "5555",
    [string]$VspHubPort = "8787",
    [string]$KernelExe = "",
    [string]$AgentBinary = "",
    [string]$GodotExe = "D:\Godot\Godot_v4.6.1-stable_win64.exe",
    # 被测前端 = UI-OS-1 的前端 worktree（port/ui-os-1 + 重编 extension）。
    [string]$GodotProjectRoot = "D:\Godot\vit-daw-frontend_wt_uios1",
    [string]$ProjectSource = "D:\Godot\project\vit-daw-frontend\912.vit",
    [string]$RunRoot = "",
    [int]$WaitSeconds = 60,
    [int]$SceneSettleSeconds = 40,
    [switch]$SkipBuild,
    [switch]$SkipLegacyLeg
)

# UI-OS-1 端侧烟测（343 ui_host_smoke.ps1 同模式新脚本；授权=卡面主管域扩展声明）。
# 断言面：
#   A) autoload 腿（343 继承）：真实三件套（VitApp 内核 + Godot 前端 worktree + go build
#      VitAgent），VIT_AGENT_WINDOW_MODE=autoload；boot→page_loaded→auto_open.done→
#      scene_added(VitDockRoot) 同一 HWND + navigate_count==1（343 不变量不被本卡破坏）。
#   B) OS 行为矩阵（UI-OS-1 新增，VIT_SMOKE_OS_NOTIFY=1 钩子协作）：
#      B1 任务栏区分：chat 窗 title=="Vit"+类图标在；同 PID 的 DAW 主窗 title≠"Vit"
#         （任务栏两条目可区分：图标/标题）；无重复窗（EnumWindows 恰 1）。
#      B2 通知前台门控 N1：page_loaded 即刻 notify（foreground=true → issued=false，
#         os_event notification_skipped）——「打开在前时不通知」。
#      B3 最小化→任务栏：SW_MINIMIZE → IsIconic + topmost 撤销 + os_event minimized；
#         SW_RESTORE（召回）→ 同 HWND 可见 + topmost 复立（always-on-top 仅打开态）。
#      B4 后台真实通知 N2：最小化态 notify → issued=true + os_event notification_shown
#         （Shell_NotifyIconW NIM_ADD/NIM_MODIFY 真实触发）；notification_visible
#         （NIN_BALLOONSHOW，OS 确认上屏）为软断言（DND 环境可能缺）。
#      B5 通知召回：钩子合成召回（生产 _recall_from_notification 路径）→ 同 HWND
#         恢复可见。（OS toast 实点=人工目检项，脚本以同一生产路径合成驱动。）
#      B6 关闭≠最小化：PostMessageW(WM_CLOSE) → DestroyWindow → IsWindow=false +
#         枚举 0 + os_event closed（真关，不是 SW_HIDE 收拢）。
#      B7 关闭后重建：show_window(false) → 新 HWND（≠旧）+ boot_index=2 +
#         navigate_count==2（新窗页面重载）+ 前后前台不变（SW_SHOWNA 不抢焦点）+
#         topmost 复立。
#   C) legacy 腿（-SkipLegacyLeg 跳过）：VIT_AGENT_WINDOW_MODE=legacy 无 boot 行。
# 通过标准：退出码 0（AGENTS §5 烟测门槛）。

$ErrorActionPreference = "Stop"
if ([string]::IsNullOrWhiteSpace($RepoRoot)) { $RepoRoot = "D:\Vit_DAW_wt_uios1" }

function Write-Step0 { param([string]$Msg) Write-Host ("--- " + $Msg) }
if (-not (Test-Path -LiteralPath $RepoRoot)) { throw ("RepoRoot not found: " + $RepoRoot) }
if ([string]::IsNullOrWhiteSpace($RunRoot)) {
    $RunRoot = Join-Path $RepoRoot ("coord\runs\UI-OS-1\run_" + (Get-Date -Format "yyyyMMdd_HHmmss"))
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

function Get-JsonField {
    param([string]$Line, [string]$Field)
    $m = ($Line | Select-String -Pattern ('"' + $Field + '":\s*(-?\d+)') -AllMatches).Matches
    if ($m.Count -gt 0) { return [long]$m[0].Groups[1].Value }
    return 0
}

function Get-JsonBool {
    param([string]$Line, [string]$Field)
    $m = ($Line | Select-String -Pattern ('"' + $Field + '":\s*(true|false)') -AllMatches).Matches
    if ($m.Count -gt 0) { return ($m[0].Groups[1].Value -eq "true") }
    return $false
}

function Get-JsonString {
    param([string]$Line, [string]$Field)
    $m = ($Line | Select-String -Pattern ('"' + $Field + '":\s*"([^"]*)"') -AllMatches).Matches
    if ($m.Count -gt 0) { return $m[0].Groups[1].Value }
    return ""
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

Write-Host "=== UI-OS-1 smoke; run dir: $RunRoot"
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
$ProjectCopy = Join-Path $FixtureDir ("ui_os_" + [System.IO.Path]::GetFileName($ProjectSource))
Copy-Item -LiteralPath $ProjectSource -Destination $ProjectCopy -Force
$fixtureHashBefore = (Get-FileHash -Algorithm SHA256 -LiteralPath $ProjectSource).Hash
$assert["fixture_copy"] = $ProjectCopy
$assert["fixture_source_sha256"] = $fixtureHashBefore

# kernel workspace 隔离。
$KernelWorkspace = Join-Path $RunRoot "kernel_workspace"
New-Item -ItemType Directory -Force -Path (Join-Path $KernelWorkspace "Settings") | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $KernelWorkspace "Logs") | Out-Null
$RepoDefaultProject = Join-Path $RepoRoot "VitApp\Workspace\default_project.xml"
if (Test-Path -LiteralPath $RepoDefaultProject) {
    Copy-Item -LiteralPath $RepoDefaultProject -Destination (Join-Path $KernelWorkspace "default_project.xml") -Force
}

# agent 构建（隔离二进制放 run 目录）。
if ([string]::IsNullOrWhiteSpace($AgentBinary)) { $AgentBinary = Join-Path $RunRoot "bin\VitAgent.ui_os.exe" }
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
    Write-Step0 "Import pass (rebuild .godot cache in a fresh worktree)"
    $importArgs = @("--headless", "--path", $GodotProjectRoot, "--import")
    $importProc = Start-Process -FilePath $GodotExe -ArgumentList $importArgs -WorkingDirectory $GodotProjectRoot -PassThru -Wait -WindowStyle Hidden
    if (-not (Test-Path -LiteralPath (Join-Path $GodotProjectRoot ".godot\extension_list.cfg"))) {
        throw ("Godot import pass did not produce .godot/extension_list.cfg")
    }
    $assert["godot_import_pass"] = $true

    Write-Step0 "Start Godot (autoload + smoke open + OS notify sequence hooks)"
    $env:VIT_AGENT_WINDOW_MODE = "autoload"
    $env:VIT_SMOKE_AUTO_OPEN_PROJECT = $ProjectCopy
    $env:VIT_SMOKE_OS_NOTIFY = "1"
    $env:VIT_SKIP_DEV_AUTOSTART = "1"
    $env:VIT_DAW_DEV_ROOT = $RepoRoot
    try {
        $godotProc = Start-Process -FilePath $GodotExe -ArgumentList @("--path", $GodotProjectRoot, "--log-file", $godotAutoloadLog) -WorkingDirectory $GodotProjectRoot -PassThru
    }
    finally {
        Remove-Item Env:VIT_AGENT_WINDOW_MODE -ErrorAction SilentlyContinue
        Remove-Item Env:VIT_SMOKE_AUTO_OPEN_PROJECT -ErrorAction SilentlyContinue
        Remove-Item Env:VIT_SMOKE_OS_NOTIFY -ErrorAction SilentlyContinue
        Remove-Item Env:VIT_SKIP_DEV_AUTOSTART -ErrorAction SilentlyContinue
    }
    $godotAutoloadPid = $godotProc.Id
    $assert["godot_autoload_pid"] = $godotAutoloadPid

    # boot 行（main_hwnd/boot_index 留证）。
    $bootHit = Test-LogLine -LogPath $godotAutoloadLog -Pattern '\[AgentChatWindow\] boot ' -TimeoutSeconds $WaitSeconds
    if ($null -eq $bootHit) { throw ("AgentChatWindow boot line not seen in " + $godotAutoloadLog) }
    $assert["boot_line"] = ($bootHit.Line -replace '^.*\[AgentChatWindow\] boot ', '').Trim()
    $bootIndex1 = Get-JsonField -Line $bootHit.Line -Field "boot_index"
    $assert["boot_index_first"] = $bootIndex1

    # visible 行（child_hwnd 留证——343 同源：boot 行无 child_hwnd，visible 行才有）。
    $visHit = Test-LogLine -LogPath $godotAutoloadLog -Pattern '\[AgentChatWindow\] visible ' -TimeoutSeconds 30
    if ($null -eq $visHit) { throw ("AgentChatWindow visible line not seen") }
    $assert["visible_line"] = ($visHit.Line -replace '^.*\[AgentChatWindow\] visible ', '').Trim()
    $childHwnd = Get-JsonField -Line $visHit.Line -Field "child_hwnd"
    if ($childHwnd -le 0) { throw ("visible line missing child_hwnd") }
    $assert["child_hwnd"] = $childHwnd

    # page_loaded 行（React 页面加载完成）。
    $pageHit = Test-LogLine -LogPath $godotAutoloadLog -Pattern '\[AgentChatWindow\] page_loaded ' -TimeoutSeconds 90
    if ($null -eq $pageHit) { throw ("AgentChatWindow page_loaded line not seen (React did not load)") }
    $assert["page_loaded_line"] = ($pageHit.Line -replace '^.*\[AgentChatWindow\] page_loaded ', '').Trim()

    # N1：前台门控（page_loaded 即刻 notify → foreground=true → issued=false）。
    Write-Step0 "N1: foreground gating (notify while chat window is in the foreground)"
    $n1Hit = Test-LogLine -LogPath $godotAutoloadLog -Pattern '\[AgentChatWindow\] notify\.request .*"state":\s*"done"' -TimeoutSeconds 30
    if ($null -eq $n1Hit) { throw ("N1 notify.request (done) not seen") }
    $n1Foreground = Get-JsonBool -Line $n1Hit.Line -Field "foreground"
    $n1Issued = Get-JsonBool -Line $n1Hit.Line -Field "issued"
    $assert["n1_notify_line"] = ($n1Hit.Line -replace '^.*\[AgentChatWindow\] notify\.request ', '').Trim()
    $assert["n1_foreground"] = $n1Foreground
    $assert["n1_issued"] = $n1Issued
    $assert["n1_gating_holds"] = ($n1Foreground -and -not $n1Issued)
    $n1Skip = Test-LogLine -LogPath $godotAutoloadLog -Pattern 'os_event.*notification_skipped' -TimeoutSeconds 10
    $assert["n1_skip_event_logged"] = ($null -ne $n1Skip)
    if (-not $assert["n1_gating_holds"]) { throw ("N1 foreground gating violated: foreground=$n1Foreground issued=$n1Issued") }

    # open 链 + 场景切换（343 不变量）。
    $doneHit = Test-LogLine -LogPath $godotAutoloadLog -Pattern '\[AgentChatWindow\] smoke\.auto_open\.done ' -TimeoutSeconds $SceneSettleSeconds
    if ($null -eq $doneHit) { throw ("smoke.auto_open.done not seen within ${SceneSettleSeconds}s (open chain failed)") }
    $assert["auto_open_done_line"] = ($doneHit.Line -replace '^.*\[AgentChatWindow\] smoke\.auto_open\.done ', '').Trim()
    $sceneHit = Test-LogLine -LogPath $godotAutoloadLog -Pattern 'scene_added.*VitDockRoot' -TimeoutSeconds $SceneSettleSeconds
    if ($null -eq $sceneHit) { throw ("scene_added VitDockRoot not seen (scene switch did not happen)") }
    $assert["scene_added_line"] = $sceneHit.Line
    $sceneHwnd = Get-JsonField -Line $sceneHit.Line -Field "child_hwnd"
    $doneHwnd = Get-JsonField -Line $doneHit.Line -Field "child_hwnd"
    $doneNavCount = Get-JsonField -Line $doneHit.Line -Field "navigate_count"
    $doneUrl = Get-JsonString -Line $doneHit.Line -Field "page_url"
    $assert["same_hwnd_across_scene_switch"] = ($childHwnd -eq $doneHwnd -and $doneHwnd -eq $sceneHwnd -and $doneHwnd -gt 0)
    $assert["navigate_count_is_1"] = ($doneNavCount -eq 1)
    $assert["page_url_after_open"] = $doneUrl
    $assert["page_url_unchanged"] = ($doneUrl -like "http://127.0.0.1:7878/app/*")
    if (-not $assert["same_hwnd_across_scene_switch"]) { throw ("HWND changed across scene switch: child=$childHwnd done=$doneHwnd scene=$sceneHwnd") }
    if (-not $assert["navigate_count_is_1"]) { throw ("page reloaded: navigate_count=$doneNavCount") }
    if (-not $assert["page_url_unchanged"]) { throw ("page URL changed: $doneUrl") }

    # ---------------------------------------------------------- OS baseline
    Write-Step0 "Win32 face: enum/title/icon/DPI + taskbar differentiation"
    Add-Type -Namespace UiOsSmoke -Name Win32 -MemberDefinition @'
[DllImport("user32.dll")] public static extern bool EnumWindows(EnumProc cb, IntPtr lp);
public delegate bool EnumProc(IntPtr hWnd, IntPtr lp);
[DllImport("user32.dll", CharSet=CharSet.Unicode)] public static extern int GetClassNameW(IntPtr hWnd, System.Text.StringBuilder text, int count);
[DllImport("user32.dll", CharSet=CharSet.Unicode)] public static extern int GetWindowTextW(IntPtr hWnd, System.Text.StringBuilder text, int count);
[DllImport("user32.dll")] public static extern bool IsWindowVisible(IntPtr hWnd);
[DllImport("user32.dll")] public static extern bool IsIconic(IntPtr hWnd);
[DllImport("user32.dll")] public static extern bool IsWindow(IntPtr hWnd);
[DllImport("user32.dll")] public static extern bool ShowWindow(IntPtr hWnd, int nCmdShow);
[DllImport("user32.dll")] public static extern IntPtr GetForegroundWindow();
[DllImport("user32.dll")] public static extern IntPtr GetWindowLongPtrW(IntPtr hWnd, int nIndex);
[DllImport("user32.dll")] public static extern IntPtr GetClassLongPtrW(IntPtr hWnd, int nIndex);
[DllImport("user32.dll")] public static extern bool PostMessageW(IntPtr hWnd, uint Msg, IntPtr wParam, IntPtr lParam);
[DllImport("user32.dll")] public static extern uint GetWindowThreadProcessId(IntPtr hWnd, ref uint pid);
[DllImport("user32.dll")] public static extern uint GetDpiForWindow(IntPtr hWnd);
'@
    $allWindows = New-Object System.Collections.ArrayList
    $enumCb = [UiOsSmoke.Win32+EnumProc]{ param($h, $lp)
        $cn = New-Object System.Text.StringBuilder 256
        [void][UiOsSmoke.Win32]::GetClassNameW($h, $cn, 256)
        [void]$allWindows.Add([pscustomobject]@{ Hwnd = $h; Class = $cn.ToString() })
        return $true
    }
    [void][UiOsSmoke.Win32]::EnumWindows($enumCb, [IntPtr]::Zero)
    $chatWindows = @($allWindows | Where-Object { $_.Class -eq "VitWebViewHostChildWindow" })
    $assert["chat_window_count"] = $chatWindows.Count
    $assert["no_duplicate_window"] = ($chatWindows.Count -eq 1)
    if ($chatWindows.Count -ne 1) { throw ("expected exactly 1 VitWebViewHostChildWindow top-level window, got $($chatWindows.Count)") }
    $chatHwnd = $chatWindows[0].Hwnd
    $assert["chat_hwnd_win32"] = ([long]$chatHwnd)
    if (([long]$chatHwnd) -ne $childHwnd) { throw ("Win32 enum hwnd $chatHwnd != log child_hwnd $childHwnd") }
    $assert["win32_hwnd_matches_log"] = $true

    $sb = New-Object System.Text.StringBuilder 256
    [void][UiOsSmoke.Win32]::GetWindowTextW($chatHwnd, $sb, 256)
    $title = $sb.ToString()
    $assert["window_title"] = $title
    $assert["title_is_vit"] = ($title -eq "Vit")
    if ($title -ne "Vit") { throw ("window title is '$title', expected 'Vit'") }
    $iconPtr = [UiOsSmoke.Win32]::GetClassLongPtrW($chatHwnd, -14)  # GCLP_HICON
    $assert["taskbar_icon_present"] = ($iconPtr -ne [IntPtr]::Zero)
    $chatPid = [uint32]0
    [void][UiOsSmoke.Win32]::GetWindowThreadProcessId($chatHwnd, [ref]$chatPid)
    $assert["chat_window_pid"] = ([int]$chatPid)
    $chatDpi = [UiOsSmoke.Win32]::GetDpiForWindow($chatHwnd)
    $assert["chat_window_dpi"] = ([int]$chatDpi)
    $assert["dpi_record_only"] = ("chat_window_dpi recorded; 125%/150% visual scaling = manual observation item")

    # 任务栏条目可区分：同 PID 的另一个可见顶层窗（DAW Godot 主窗）title ≠ "Vit"。
    $dawWin = $null
    foreach ($w in $allWindows) {
        if ($w.Hwnd -eq $chatHwnd) { continue }
        if ($w.Class -eq "VitWebViewHostChildWindow") { continue }
        $wpid = [uint32]0
        [void][UiOsSmoke.Win32]::GetWindowThreadProcessId($w.Hwnd, [ref]$wpid)
        if ($wpid -ne $chatPid) { continue }
        if (-not [bool][UiOsSmoke.Win32]::IsWindowVisible($w.Hwnd)) { continue }
        $sb2 = New-Object System.Text.StringBuilder 256
        [void][UiOsSmoke.Win32]::GetWindowTextW($w.Hwnd, $sb2, 256)
        if ($sb2.ToString().Length -gt 0) { $dawWin = [pscustomobject]@{ Hwnd = $w.Hwnd; Class = $w.Class; Title = $sb2.ToString() }; break }
    }
    if ($null -ne $dawWin) {
        $assert["daw_window_class"] = $dawWin.Class
        $assert["daw_window_title"] = $dawWin.Title
        $assert["taskbar_entries_distinguishable"] = ($dawWin.Title -ne "Vit")
    } else {
        $assert["daw_window_title"] = ""
        $assert["taskbar_entries_distinguishable"] = $false
    }

    # ---------------------------------------------------------- N2 + minimize
    Write-Step0 "Minimize -> taskbar (topmost dropped) -> N2 background real notify"
    [void][UiOsSmoke.Win32]::ShowWindow($chatHwnd, 6)  # SW_MINIMIZE
    Start-Sleep -Milliseconds 900
    $minimized = [bool][UiOsSmoke.Win32]::IsIconic($chatHwnd)
    $assert["minimized_ok"] = $minimized
    if (-not $minimized) { throw ("SW_MINIMIZE did not iconic the chat window") }
    $exStyleMin = [long][UiOsSmoke.Win32]::GetWindowLongPtrW($chatHwnd, -20)  # GWL_EXSTYLE
    $topmostAfterMinimize = (($exStyleMin -band 0x00000008) -ne 0)  # WS_EX_TOPMOST
    $assert["topmost_removed_on_minimize"] = (-not $topmostAfterMinimize)
    $minEventHit = Test-LogLine -LogPath $godotAutoloadLog -Pattern 'os_event.*"kind":\s*"minimized"' -TimeoutSeconds 10
    $assert["minimized_event_logged"] = ($null -ne $minEventHit)
    if ($topmostAfterMinimize) { throw ("topmost survived minimize (always-on-top must hold only in the open state)") }

    $n2Hit = Test-LogLine -LogPath $godotAutoloadLog -Pattern '\[AgentChatWindow\] notify\.request .*"state":\s*"confirm"' -TimeoutSeconds 30
    if ($null -eq $n2Hit) { throw ("N2 notify.request (confirm) not seen") }
    $n2Foreground = Get-JsonBool -Line $n2Hit.Line -Field "foreground"
    $n2Issued = Get-JsonBool -Line $n2Hit.Line -Field "issued"
    $assert["n2_notify_line"] = ($n2Hit.Line -replace '^.*\[AgentChatWindow\] notify\.request ', '').Trim()
    $assert["n2_foreground"] = $n2Foreground
    $assert["n2_issued"] = $n2Issued
    $assert["n2_background_gating_holds"] = ((-not $n2Foreground) -and $n2Issued)
    if (-not $assert["n2_background_gating_holds"]) { throw ("N2 background notify failed: foreground=$n2Foreground issued=$n2Issued") }
    $shownHit = Test-LogLine -LogPath $godotAutoloadLog -Pattern 'os_event.*"kind":\s*"notification_shown"' -TimeoutSeconds 10
    $assert["n2_shown_event_logged"] = ($null -ne $shownHit)
    if (-not $assert["n2_shown_event_logged"]) { throw ("notification_shown os_event not seen (Shell_NotifyIcon failed)") }
    $visibleHit = Test-LogLine -LogPath $godotAutoloadLog -Pattern 'os_event.*"kind":\s*"notification_visible"' -TimeoutSeconds 10
    $assert["n2_balloon_on_screen_os_confirmed"] = ($null -ne $visibleHit)   # soft: DND 环境可能缺

    # ------------------------------------------------- recall (same HWND)
    Write-Step0 "Notification recall path -> same HWND restored"
    $recallHit = Test-LogLine -LogPath $godotAutoloadLog -Pattern '\[AgentChatWindow\] notify\.recall\.done ' -TimeoutSeconds 30
    if ($null -eq $recallHit) { throw ("notify.recall.done not seen (synthetic recall path failed)") }
    $assert["recall_line"] = ($recallHit.Line -replace '^.*\[AgentChatWindow\] notify\.recall\.done ', '').Trim()
    $recallHwnd = Get-JsonField -Line $recallHit.Line -Field "child_hwnd"
    $assert["recall_same_hwnd"] = ($recallHwnd -eq $childHwnd)
    Start-Sleep -Milliseconds 900
    $recallVisible = [bool][UiOsSmoke.Win32]::IsWindowVisible($chatHwnd) -and -not [bool][UiOsSmoke.Win32]::IsIconic($chatHwnd)
    $assert["recall_visible_ok"] = $recallVisible
    $exStyleRecall = [long][UiOsSmoke.Win32]::GetWindowLongPtrW($chatHwnd, -20)
    $assert["topmost_reapplied_on_restore"] = ((($exStyleRecall -band 0x00000008) -ne 0))
    if (-not $recallVisible) { throw ("recall did not bring the SAME window back visible") }
    if ($recallHwnd -ne $childHwnd) { throw ("recall hwnd $recallHwnd != original $childHwnd") }

    # ------------------------------------------------- close != minimize
    Write-Step0 "Close (WM_CLOSE) = real destroy, not collapse"
    [void][UiOsSmoke.Win32]::PostMessageW($chatHwnd, 0x0010, [IntPtr]::Zero, [IntPtr]::Zero)  # WM_CLOSE
    $closeDeadline = (Get-Date).AddSeconds(10)
    $windowGone = $false
    while ((Get-Date) -lt $closeDeadline) {
        if (-not [bool][UiOsSmoke.Win32]::IsWindow($chatHwnd)) { $windowGone = $true; break }
        Start-Sleep -Milliseconds 300
    }
    $assert["close_is_destroy_iswindow_false"] = $windowGone
    $closedEventHit = Test-LogLine -LogPath $godotAutoloadLog -Pattern 'os_event.*"kind":\s*"closed"' -TimeoutSeconds 10
    $assert["closed_event_logged"] = ($null -ne $closedEventHit)
    $enumAfterClose = New-Object System.Collections.ArrayList
    $enumCb2 = [UiOsSmoke.Win32+EnumProc]{ param($h, $lp)
        $cn = New-Object System.Text.StringBuilder 256
        [void][UiOsSmoke.Win32]::GetClassNameW($h, $cn, 256)
        if ($cn.ToString() -eq "VitWebViewHostChildWindow") { [void]$enumAfterClose.Add($h) }
        return $true
    }
    [void][UiOsSmoke.Win32]::EnumWindows($enumCb2, [IntPtr]::Zero)
    $assert["chat_window_count_after_close"] = $enumAfterClose.Count
    if (-not $windowGone) { throw ("WM_CLOSE did not destroy the window (close collapsed into hide?)") }
    if ($enumAfterClose.Count -ne 0) { throw ("chat window still enumerable after close") }

    # ------------------------------------------------- recreate (new HWND)
    Write-Step0 "Recreate after close: new HWND, page reload, no focus steal"
    $recHit = Test-LogLine -LogPath $godotAutoloadLog -Pattern '\[AgentChatWindow\] smoke\.os_notify\.recreated ' -TimeoutSeconds 40
    if ($null -eq $recHit) { throw ("smoke.os_notify.recreated not seen (recreate path failed)") }
    $recLine = ($recHit.Line -replace '^.*\[AgentChatWindow\] smoke\.os_notify\.recreated ', '').Trim()
    $assert["recreated_line"] = $recLine
    $newHwnd = Get-JsonField -Line $recHit.Line -Field "child_hwnd"
    $recBootIndex = Get-JsonField -Line $recHit.Line -Field "boot_index"
    $recNavCount = Get-JsonField -Line $recHit.Line -Field "navigate_count"
    $recPageLoaded = Get-JsonBool -Line $recHit.Line -Field "page_loaded"
    $recFgBefore = Get-JsonBool -Line $recHit.Line -Field "foreground_before"
    $recFgAfter = Get-JsonBool -Line $recHit.Line -Field "foreground_after"
    $recUrl = Get-JsonString -Line $recHit.Line -Field "page_url"
    $assert["recreated_new_hwnd"] = $newHwnd
    $assert["recreated_hwnd_differs"] = ($newHwnd -gt 0 -and $newHwnd -ne $childHwnd)
    $assert["recreated_boot_index_2"] = ($recBootIndex -eq 2)
    $assert["recreated_navigate_count_2"] = ($recNavCount -eq 2)
    $assert["recreated_page_loaded"] = $recPageLoaded
    $assert["recreated_url_ok"] = ($recUrl -like "http://127.0.0.1:7878/app/*")
    $assert["recreate_no_focus_steal"] = ($recFgBefore -eq $recFgAfter)
    $enumAfterRecreate = New-Object System.Collections.ArrayList
    $enumCb3 = [UiOsSmoke.Win32+EnumProc]{ param($h, $lp)
        $cn = New-Object System.Text.StringBuilder 256
        [void][UiOsSmoke.Win32]::GetClassNameW($h, $cn, 256)
        if ($cn.ToString() -eq "VitWebViewHostChildWindow") { [void]$enumAfterRecreate.Add($h) }
        return $true
    }
    [void][UiOsSmoke.Win32]::EnumWindows($enumCb3, [IntPtr]::Zero)
    $assert["chat_window_count_after_recreate"] = $enumAfterRecreate.Count
    if (-not $assert["recreated_hwnd_differs"]) { throw ("recreate reused the old HWND or failed: old=$childHwnd new=$newHwnd") }
    if ($enumAfterRecreate.Count -ne 1) { throw ("expected exactly 1 chat window after recreate, got $($enumAfterRecreate.Count)") }
    $newChatHwnd = $enumAfterRecreate[0]
    if (([long]$newChatHwnd) -ne $newHwnd) { throw ("recreated enum hwnd $newChatHwnd != log $newHwnd") }
    $assert["recreated_visible"] = [bool][UiOsSmoke.Win32]::IsWindowVisible($newChatHwnd)
    $exStyleRec = [long][UiOsSmoke.Win32]::GetWindowLongPtrW($newChatHwnd, -20)
    $assert["recreated_topmost"] = ((($exStyleRec -band 0x00000008) -ne 0))

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
        $report.legacy_leg["no_boot_line"] = ($null -eq $legacyBoot)
        $legacyProcAlive = $true
        try { $legacyProcAlive = (-not $godotLegacyProc.HasExited) } catch { $legacyProcAlive = $false }
        $report.legacy_leg["process_alive"] = $legacyProcAlive
        if (-not $report.legacy_leg["no_boot_line"]) { throw ("legacy mode unexpectedly created the chat window") }
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
    $reportPath = Join-Path $ArtDir "ui_os_smoke_report.json"
    $report | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $reportPath -Encoding UTF8
    Write-Host ("report: " + $reportPath)
    Copy-Item -LiteralPath $godotAutoloadLog -Destination $ArtDir -Force -ErrorAction SilentlyContinue
    Copy-Item -LiteralPath $godotLegacyLog -Destination $ArtDir -Force -ErrorAction SilentlyContinue
    Copy-Item -LiteralPath $agentLog -Destination $ArtDir -Force -ErrorAction SilentlyContinue
}

if ($report.exit_code -eq 0) { Write-Host "UI-OS-1 smoke: PASS" ; exit 0 }
else { Write-Host "UI-OS-1 smoke: FAIL" ; exit 1 }
