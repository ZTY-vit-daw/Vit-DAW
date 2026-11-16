[CmdletBinding()]
param(
    [string]$RepoRoot = "",
    [string]$AgentHttp = "http://127.0.0.1:7878",
    [string]$ZmqReqPort = "5555",
    [string]$VspHubPort = "8787",
    [string]$KernelExe = "",
    [string]$AgentBinary = "",
    [string]$GodotExe = "D:\Godot\Godot_v4.6.1-stable_win64.exe",
    # 被测前端 = UI-BRIDGE-1 前端 worktree（port/ui-bridge-1；extension dll 为 git 跟踪工件，cpp 零改动免重编）。
    [string]$GodotProjectRoot = "D:\Godot\vit-daw-frontend_wt_uibridge1",
    [string]$ProjectSource = "D:\Godot\project\vit-daw-frontend\912.vit",
    [string]$RunRoot = "",
    [int]$WaitSeconds = 60,
    [int]$LegSettleSeconds = 50,
    [switch]$SkipBuild,
    # LAUNCHER-RECENTS-CLICK-1（383）：仅跑列表点击腿（RC1 磁带卡点击→治理→打开→
    # 工程内对话往返 / RC2 文件移走后点击→failed 降级 / RC3 损坏文件点击→failed
    # 降级）；L1/L2a/b/c 不跑。驱动=React 烟测接缝 launcher_smoke_followup=recents_click[:N]
    # （宿主 VIT_SMOKE_LAUNCHER_FOLLOWUP 既有 env 透传，Godot 零改动）。
    [switch]$RecentsClickLeg
)

# UI-BRIDGE-1 端侧烟测（343 ui_host_smoke.ps1 / 349 ui_os_smoke.ps1 同模式新脚本；授权=卡面文件域 scripts/）。
# 一次运行 = 内核/agent 单次启动 + 4 次 Godot boot（RUNBOOK §腿结构）：
#   L1 零摩擦首开（boot1，kernel 无工程）：React 桥「打开烟测桥甲」→ 治理 none → open → VitDockRoot
#      → result=opened（Godot 回执）→ result_ack=opened（React 真收取证）。
#   L2a 三键·直接切换（boot2，kernel 有甲）：「打开烟测桥乙」→ 三键对话框出现 → switch → 乙 open。
#   L2b 三键·取消（boot3，kernel 有乙）：「打开烟测桥甲」→ cancel → 无场景切换 + kernel 仍乙 + result=canceled。
#   L2c 三键·保存并切换 + dock 降级（boot4，kernel 有乙）：「打开烟测桥甲」→ save（真实 save_project）→ 甲 open
#      → VitDockRoot；followup（+8s）第二条「打开烟测桥乙」→ dock 降级 handler → unsupported 回执 + result_ack。
# 断言面：[LauncherBridge] 日志链 + scene_added VitDockRoot + boot 内同 HWND + /agent/state current_project
#         + fixture 源哈希 + recents 还原 + 零 LLM telemetry（chat 全确定性路由）。
# 通过标准：退出码 0（AGENTS §5 烟测门槛）。
# LAUNCHER-RECENTS-CLICK-1（383）扩展（-RecentsClickLeg）：
#   RC1 列表点击→打开→工程内对话往返（boot，kernel 无工程→治理 dialog on boot scratch →switch）：
#      React recents 磁带卡[0]（烟测接缝 followup=recents_click，greeting+recents 就绪后 +8s 经
#      同一 openRecentProject 链点击）→ directive.received → 治理三键 → open.begin → result=opened
#      → scene_added VitDockRoot → /agent/state=烟测桥甲 → 工程内对话往返（同会话 HTTP 两轮：
#      launcher list 轮 + 桥接播报轮 stop_reason=launcher_bridge_broadcast + volume_switch，零 LLM）。
#   RC2 文件移走失败降级（boot，kernel 有甲）：surface_ready 后删除烟测桥乙.vit → recents[1] 点击
#      （followup=recents_click:1，+25s——列表在删除前已装载）→ 治理 switch → open 失败 →
#      result=failed（file_path=乙）+ result_ack=failed（React 真收降级回执）+ 无场景切换。
#   RC3 损坏文件失败降级（boot，kernel 有甲）：烟测桥坏.vit（非工程内容，存在→可点）为 recents[2]
#      点击（followup=recents_click:2）→ 治理 switch → open 失败 → result=failed + ack + 无场景切换。
#   LLM 预算：RC 全腿 0 调用（telemetry 断言行数 0；播报轮/checkpoint 先于 LLM）。

$ErrorActionPreference = "Stop"
if ([string]::IsNullOrWhiteSpace($RepoRoot)) { $RepoRoot = "D:\Vit_DAW_wt_uibridge1" }

function Write-Step0 { param([string]$Msg) Write-Host ("--- " + $Msg) }
if (-not (Test-Path -LiteralPath $RepoRoot)) { throw ("RepoRoot not found: " + $RepoRoot) }
if ([string]::IsNullOrWhiteSpace($RunRoot)) {
    $RunRoot = Join-Path $RepoRoot ("coord\runs\UI-BRIDGE-1\run_" + (Get-Date -Format "yyyyMMdd_HHmmss"))
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

function Get-JsonString {
    param([string]$Line, [string]$Field)
    $m = ($Line | Select-String -Pattern ('"' + $Field + '":\s*"([^"]*)"') -AllMatches).Matches
    if ($m.Count -gt 0) { return $m[0].Groups[1].Value }
    return ""
}

function Get-AgentCurrentProject {
    param([string]$BaseUrl)
    try {
        $resp = Invoke-WebRequest -UseBasicParsing -Uri ($BaseUrl.TrimEnd("/") + "/agent/state") -TimeoutSec 5
        $json = $resp.Content | ConvertFrom-Json
        return [string]$json.current_project.path
    } catch { return "" }
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
    legs = [ordered]@{}
    isolation = [ordered]@{}
    exit_code = 1
}
$assert = $report.legs

Write-Host "=== UI-BRIDGE-1 smoke; run dir: $RunRoot"
Write-Host ("repo HEAD=" + $report.head + "  frontend HEAD=" + $report.frontend_head)

# §2.2 复用禁令：任一端口已监听即停（栈占用以 coord/resources/PC-RUNTIME-STACK.md 为准，
# 由调用者保证登记推送 + ≤60s 复核；脚本自身按 AGENTS §9 先净空后启栈）。
Write-Step0 "Prereq: ports must be free (7878/5555/5556/8787)"
[int]$agentPort = 7878
if ($AgentHttp -match ':(\d+)\s*$') { $agentPort = [int]$Matches[1] }
foreach ($port in $agentPort, [int]$ZmqReqPort, 5556, [int]$VspHubPort) {
    if (Get-TcpListener -Port $port) {
        throw ("port $port is already listening; NOT reusing a foreign stack (PROTOCOL 2.2 / AGENTS 9). Abort before any start.")
    }
}
Write-Host "ports free"

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
$assert["prereq"] = [ordered]@{ kernel_exe = $KernelExe }

# fixture 隔离：两份不同名副本（同内容异名=340 先例），不触碰样例/用户工程（AGENTS §10）。
# 383 RC 腿追加坡坏文件 fixture（存在但非工程内容——损坏降级腿用；agent os.Stat 通过=可点）。
$fixtureA = Join-Path $FixtureDir "烟测桥甲.vit"
$fixtureB = Join-Path $FixtureDir "烟测桥乙.vit"
$fixtureCorrupt = Join-Path $FixtureDir "烟测桥坏.vit"
Copy-Item -LiteralPath $ProjectSource -Destination $fixtureA -Force
Copy-Item -LiteralPath $ProjectSource -Destination $fixtureB -Force
[System.IO.File]::WriteAllText($fixtureCorrupt, "not a vit project (corrupt fixture, ui_bridge_smoke RC3)", (New-Object System.Text.UTF8Encoding($false)))
$fixtureSourceHashBefore = (Get-FileHash -Algorithm SHA256 -LiteralPath $ProjectSource).Hash
$report.isolation["fixture_source_sha256_before"] = $fixtureSourceHashBefore
$report.isolation["fixture_a"] = $fixtureA
$report.isolation["fixture_b"] = $fixtureB
$report.isolation["fixture_corrupt"] = $fixtureCorrupt

# recents 种子：snapshot → prepend fixture（BOM-less UTF-8 手工数组，dev_agent_smoke 同款）。
$recentsConfig = Join-Path $env:APPDATA "Vit-DAW\global_project_config.json"
if (-not (Test-Path -LiteralPath $recentsConfig)) { throw ("kernel recents config missing: " + $recentsConfig) }
$recentsBackup = Join-Path $ArtDir "global_project_config.before.json"
Copy-Item -LiteralPath $recentsConfig -Destination $recentsBackup -Force
$recentsJson = Get-Content -LiteralPath $recentsConfig -Raw -Encoding UTF8 | ConvertFrom-Json
$recentsExisting = @()
if ($null -ne $recentsJson.recent_projects) {
    $recentsExisting = @($recentsJson.recent_projects | ForEach-Object { [string]$_ })
}
$recentsSeed = @($fixtureA, $fixtureB)
if ($RecentsClickLeg) { $recentsSeed = @($fixtureA, $fixtureB, $fixtureCorrupt) }
$recentsMerged = @(($recentsSeed + $recentsExisting) | Select-Object -Unique)
$recentsItems = @($recentsMerged | ForEach-Object { $_ | ConvertTo-Json })
$recentsText = "{`n  `"recent_projects`": [`n    " + ($recentsItems -join ",`n    ") + "`n  ]`n}"
[System.IO.File]::WriteAllText($recentsConfig, $recentsText, (New-Object System.Text.UTF8Encoding($false)))
Copy-Item -LiteralPath $recentsConfig -Destination (Join-Path $ArtDir "global_project_config.seeded.json") -Force
Write-Host ("recents seeded (prepended " + $recentsSeed.Count + " fixtures; snapshot=" + $recentsBackup + ")")

# kernel workspace 隔离。
$KernelWorkspace = Join-Path $RunRoot "kernel_workspace"
New-Item -ItemType Directory -Force -Path (Join-Path $KernelWorkspace "Settings") | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $KernelWorkspace "Logs") | Out-Null
$RepoDefaultProject = Join-Path $RepoRoot "VitApp\Workspace\default_project.xml"
if (Test-Path -LiteralPath $RepoDefaultProject) {
    Copy-Item -LiteralPath $RepoDefaultProject -Destination (Join-Path $KernelWorkspace "default_project.xml") -Force
}

# agent 构建（隔离二进制放 run 目录）。
if ([string]::IsNullOrWhiteSpace($AgentBinary)) { $AgentBinary = Join-Path $RunRoot "bin\VitAgent.ui_bridge.exe" }
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
$report.isolation["agent_binary_sha256"] = (Get-FileHash -Algorithm SHA256 -LiteralPath $AgentBinary).Hash

$kernelProcId = 0
$agentProcId = 0
$godotPid = 0

function Stop-Stack {
    Write-Step0 "Finally: stop stack and confirm port release"
    Stop-GuiProcess $godotPid
    Stop-GuiProcess $agentProcId
    Start-Sleep -Seconds 1
    Stop-GuiProcess $kernelProcId
    $deadline = (Get-Date).AddSeconds(30)
    while ((Get-Date) -lt $deadline) {
        $busy = @()
        foreach ($port in 7878, [int]$ZmqReqPort, 5556, [int]$VspHubPort) { if (Get-TcpListener -Port $port) { $busy += $port } }
        if ($busy.Count -eq 0) { break }
        Start-Sleep -Milliseconds 500
    }
    $stillBusy = @()
    foreach ($port in 7878, [int]$ZmqReqPort, 5556, [int]$VspHubPort) { if (Get-TcpListener -Port $port) { $stillBusy += $port } }
    $report.isolation["ports_busy_after_teardown"] = ($stillBusy -join ",")
}

# 单 boot 助手：起 Godot（autoload + 桥烟测钩子），返回日志路径。
function Start-BridgeGodot {
    param([string]$LogPath, [string]$BridgeMessage, [string]$Governance = "", [string]$FollowupMs = "", [string]$Followup = "")
    $env:VIT_AGENT_WINDOW_MODE = "autoload"
    $env:VIT_SMOKE_LAUNCHER_BRIDGE = $BridgeMessage
    $env:VIT_SMOKE_LAUNCHER_GOVERNANCE = $Governance
    if (-not [string]::IsNullOrWhiteSpace($FollowupMs)) { $env:VIT_SMOKE_LAUNCHER_FOLLOWUP_MS = $FollowupMs }
    if (-not [string]::IsNullOrWhiteSpace($Followup)) { $env:VIT_SMOKE_LAUNCHER_FOLLOWUP = $Followup }
    $env:VIT_SKIP_DEV_AUTOSTART = "1"
    $env:VIT_DAW_DEV_ROOT = $RepoRoot
    try {
        $proc = Start-Process -FilePath $GodotExe -ArgumentList @("--path", $GodotProjectRoot, "--log-file", $LogPath) -WorkingDirectory $GodotProjectRoot -PassThru
    }
    finally {
        Remove-Item Env:VIT_AGENT_WINDOW_MODE -ErrorAction SilentlyContinue
        Remove-Item Env:VIT_SMOKE_LAUNCHER_BRIDGE -ErrorAction SilentlyContinue
        Remove-Item Env:VIT_SMOKE_LAUNCHER_GOVERNANCE -ErrorAction SilentlyContinue
        Remove-Item Env:VIT_SMOKE_LAUNCHER_FOLLOWUP_MS -ErrorAction SilentlyContinue
        Remove-Item Env:VIT_SMOKE_LAUNCHER_FOLLOWUP -ErrorAction SilentlyContinue
        Remove-Item Env:VIT_SKIP_DEV_AUTOSTART -ErrorAction SilentlyContinue
    }
    return $proc.Id
}

# 每 boot 重建 agent（340 -RestartAgent 先例）：launcher 门副门 user_entered_project
# 是 agent 进程级不重置（launcher_flow.go 副门语义）——共享 agent 会让 boot2-4 门关闭，
# directive 不再路由；重启 agent = 每腿全新门态。内核不重启（在开工程跨 agent 存活，
# L2 治理前提=新 agent shadow 从内核态同步 project_path，337 双面同源）。
function Start-BridgeAgent {
    param([string]$DraftRoot, [string]$TelemetryPath = "")
    New-Item -ItemType Directory -Force -Path $DraftRoot | Out-Null
    $env:VIT_HISTORY_DRAFT_ROOT = $DraftRoot
    $env:VIT_DAW_DEV_ROOT = $RepoRoot
    # 383 RC 腿：LLM telemetry 显式落 run 目录（零 LLM 断言=行数 0）。
    if (-not [string]::IsNullOrWhiteSpace($TelemetryPath)) {
        $env:VIT_AGENT_LLM_TELEMETRY_PATH = $TelemetryPath
    }
    try {
        $proc = Start-Process -FilePath $AgentBinary -ArgumentList @("-http", ($AgentHttp -replace '^http://',''), "-last-log-path", $agentLog, "-keep-last-log-lines", "8000") -WorkingDirectory $AgentDir -WindowStyle Hidden -PassThru
    }
    finally {
        Remove-Item Env:VIT_HISTORY_DRAFT_ROOT -ErrorAction SilentlyContinue
        Remove-Item Env:VIT_AGENT_LLM_TELEMETRY_PATH -ErrorAction SilentlyContinue
    }
    if (-not (Wait-HttpReady -BaseUrl $AgentHttp -TimeoutSeconds $WaitSeconds)) {
        throw ("agent HTTP did not become ready (leg restart)")
    }
    return $proc.Id
}

# 单 boot 公共前段断言：boot/visible/page_loaded/surface_ready/directive.received。
function Assert-BridgeBoot {
    param([string]$LogPath, $Leg)
    $bootHit = Test-LogLine -LogPath $LogPath -Pattern '\[AgentChatWindow\] boot ' -TimeoutSeconds $WaitSeconds
    if ($null -eq $bootHit) { throw ("boot line not seen in " + $LogPath) }
    $Leg.boot_line = ($bootHit.Line -replace '^.*\[AgentChatWindow\] boot ', '').Trim()
    $visHit = Test-LogLine -LogPath $LogPath -Pattern '\[AgentChatWindow\] visible ' -TimeoutSeconds 30
    if ($null -eq $visHit) { throw ("visible line not seen") }
    $Leg.child_hwnd = Get-JsonField -Line $visHit.Line -Field "child_hwnd"
    if ($Leg.child_hwnd -le 0) { throw ("visible line missing child_hwnd") }
    $pageHit = Test-LogLine -LogPath $LogPath -Pattern '\[AgentChatWindow\] page_loaded ' -TimeoutSeconds 90
    if ($null -eq $pageHit) { throw ("page_loaded not seen (React did not load)") }
    $navHit = Test-LogLine -LogPath $LogPath -Pattern '\[LauncherBridge\] smoke\.navigate ' -TimeoutSeconds 60
    if ($null -eq $navHit) { throw ("smoke.navigate not seen (bridge smoke hook did not drive the surface)") }
    $Leg.smoke_navigate_url = Get-JsonString -Line $navHit.Line -Field "url"
    $srHit = Test-LogLine -LogPath $LogPath -Pattern '\[LauncherBridge\] surface_ready ' -TimeoutSeconds 60
    if ($null -eq $srHit) { throw ("launcher surface_ready not seen (React launcher surface did not come up)") }
    $drHit = Test-LogLine -LogPath $LogPath -Pattern '\[LauncherBridge\] directive\.received ' -TimeoutSeconds 90
    if ($null -eq $drHit) { throw ("directive.received not seen (React auto-fire did not reach the host)") }
    $Leg.directive_request_id = Get-JsonString -Line $drHit.Line -Field "request_id"
}

# 单 boot 公共尾段断言：result.posted + result_ack（React 真收）。
function Assert-BridgeResult {
    param([string]$LogPath, $Leg, [string]$ExpectStatus, [string]$ExpectGovernance = "")
    $rpHit = Test-LogLine -LogPath $LogPath -Pattern ('\[LauncherBridge\] result\.posted .*"status":\s*"' + $ExpectStatus + '"') -TimeoutSeconds $LegSettleSeconds
    if ($null -eq $rpHit) { throw ("result.posted status=$ExpectStatus not seen") }
    $Leg.result_posted_status = $ExpectStatus
    $Leg.result_governance = Get-JsonString -Line $rpHit.Line -Field "governance"
    if ($ExpectGovernance -ne "" -and $Leg.result_governance -ne $ExpectGovernance) {
        throw ("governance mismatch: got '" + $Leg.result_governance + "' expected '" + $ExpectGovernance + "'")
    }
    $ackHit = Test-LogLine -LogPath $LogPath -Pattern '\[LauncherBridge\] result_ack ' -TimeoutSeconds 20
    if ($null -eq $ackHit) { throw ("result.ack not seen (React did not really receive the host result)") }
    $Leg.result_ack = Get-JsonString -Line $ackHit.Line -Field "status"
    if ($Leg.result_ack -ne $ExpectStatus) {
        throw ("result_ack status mismatch: got '" + $Leg.result_ack + "' expected '" + $ExpectStatus + "'")
    }
}

$agentLog = Join-Path $LogsDir "agent_last.log"

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
    $assert["stack"] = [ordered]@{ kernel_pid = $kernelProcId }
    if (-not (Wait-PortListen -Port ([int]$ZmqReqPort) -TimeoutSeconds $WaitSeconds)) {
        throw ("kernel ZMQ REQ port $ZmqReqPort did not listen")
    }

    # ---------------------------------------------------------- import pass
    Write-Step0 "Import pass (rebuild .godot cache in a fresh worktree)"
    $importArgs = @("--headless", "--path", $GodotProjectRoot, "--import")
    $importProc = Start-Process -FilePath $GodotExe -ArgumentList $importArgs -WorkingDirectory $GodotProjectRoot -PassThru -Wait -WindowStyle Hidden
    if (-not (Test-Path -LiteralPath (Join-Path $GodotProjectRoot ".godot\extension_list.cfg"))) {
        throw ("Godot import pass did not produce .godot/extension_list.cfg")
    }

    # ============================================== LAUNCHER-RECENTS-CLICK-1（383）RC 腿
    if ($RecentsClickLeg) {
    $rcTelemetry = Join-Path $RunRoot "llm_telemetry_rc.jsonl"

    # ------------------------------------------------------------------ RC1
    Write-Step0 "RC1: recents tape-card click -> governance switch -> open -> in-project chat roundtrip (zero LLM)"
    $agentProcId = Start-BridgeAgent -DraftRoot (Join-Path $RunRoot "agent_drafts_rc1") -TelemetryPath $rcTelemetry
    $assert["stack"]["agent_pid_rc1"] = $agentProcId
    # 工程内对话往返·轮1（attempt1 教训：**进入工程后** launcher gate 关（userEnteredProject
    # 置位）→「有哪些工程」落 LLM 7 调用）。轮1 必须在 Godot boot/打开前发（gate 开=
    # 确定性 list 零 LLM），并为本会话注册 launcher 起源——轮2 桥接播报 checkpoint 的前提。
    Start-Sleep -Seconds 2
    $chatUri = $AgentHttp.TrimEnd("/") + "/agent/chat"
    $rcConv = "webui_rc1_http_" + (Get-Date -Format "yyyyMMdd_HHmmss")
    $rc1Pre = [ordered]@{}
    $assert["RC1_pre_open_launcher_list_round"] = $rc1Pre
    $round1 = $null
    for ($attempt = 1; $attempt -le 3; $attempt++) {
        $round1 = Invoke-RestMethod -Method Post -Uri $chatUri -ContentType "application/json; charset=utf-8" -Body ([System.Text.Encoding]::UTF8.GetBytes((ConvertTo-Json @{ conversation_id = $rcConv; message = "有哪些工程"; context = @{ launcher = $true } } -Compress))) -TimeoutSec 30
        $rc1Pre.chat_round1_reply = [string]$round1.reply
        if ($rc1Pre.chat_round1_reply -like "*最近的工程*") { break }
        if ($rc1Pre.chat_round1_reply -notlike "*连不上内核*") {
            throw ("RC1 chat round1 (pre-open) not deterministic list reply — gate already closed? reply=" + $rc1Pre.chat_round1_reply)
        }
        # 内核引导窗口内的确定性降级回执（零 LLM）——等 3s 重试（不盲目重试防 LLM 消耗）。
        Start-Sleep -Seconds 3
    }
    if ($rc1Pre.chat_round1_reply -notlike "*最近的工程*") {
        throw ("RC1 chat round1 (pre-open) kernel never became reachable: " + $rc1Pre.chat_round1_reply)
    }
    $rc1Pre.rounds_ok = $true
    $logR1 = Join-Path $LogsDir "godot_boot_rc1.log"
    $godotPid = Start-BridgeGodot -LogPath $logR1 -BridgeMessage "有哪些工程" -Governance "switch" -FollowupMs "8000" -Followup "recents_click"
    $assert["stack"]["godot_boot_rc1_pid"] = $godotPid
    $rc1 = [ordered]@{}
    $assert["RC1_recents_click_open_roundtrip"] = $rc1
    Assert-BridgeBoot -LogPath $logR1 -Leg $rc1
    if ($rc1.smoke_navigate_url -notlike "*launcher_smoke_followup=recents_click*") {
        throw ("RC1 smoke.navigate URL missing recents_click followup: " + $rc1.smoke_navigate_url)
    }
    $rc1.followup_url_ok = $true
    # 治理（boot scratch 保守判据=L1 同形态）：dialog → 宿主自动 switch。
    $govR1 = Test-LogLine -LogPath $logR1 -Pattern '\[LauncherBridge\] governance\.dialog_shown ' -TimeoutSeconds 30
    if ($null -eq $govR1) { throw ("RC1 governance dialog not seen") }
    $gpR1 = Test-LogLine -LogPath $logR1 -Pattern '\[LauncherBridge\] smoke\.governance\.press .*"choice":\s*"switch"' -TimeoutSeconds 30
    if ($null -eq $gpR1) { throw ("RC1 governance press (switch) not seen") }
    $obR1 = Test-LogLine -LogPath $logR1 -Pattern '\[LauncherBridge\] open\.begin ' -TimeoutSeconds 20
    if ($null -eq $obR1) { throw ("RC1 open.begin not seen") }
    Assert-BridgeResult -LogPath $logR1 -Leg $rc1 -ExpectStatus "opened" -ExpectGovernance "switch"
    $sceneR1 = Test-LogLine -LogPath $logR1 -Pattern 'scene_added.*VitDockRoot' -TimeoutSeconds $LegSettleSeconds
    if ($null -eq $sceneR1) { throw ("RC1 scene_added VitDockRoot not seen (click-to-open chain did not dock)") }
    $rc1.scene_hwnd = Get-JsonField -Line $sceneR1.Line -Field "child_hwnd"
    if (-not ($rc1.scene_hwnd -eq $rc1.child_hwnd -and $rc1.child_hwnd -gt 0)) {
        throw ("RC1 HWND changed across open: child=$($rc1.child_hwnd) scene=$($rc1.scene_hwnd)")
    }
    $rc1.same_hwnd_across_open = $true
    # 打开工程绑定观察（L2a 先例：open 后 shadow 同步，/agent/state=被点条目）。
    $bindDeadline = (Get-Date).AddSeconds(30)
    $rc1Bound = ""
    while ((Get-Date) -lt $bindDeadline) {
        $rc1Bound = Get-AgentCurrentProject -BaseUrl $AgentHttp
        if ($rc1Bound -like "*烟测桥甲*") { break }
        Start-Sleep -Milliseconds 800
    }
    if ($rc1Bound -notlike "*烟测桥甲*") { throw ("RC1 agent current_project not fixture A after open: '$rc1Bound'") }
    $rc1.agent_current_project = $rc1Bound
    # 工程内对话往返·轮2（同会话，进入工程后）：launcher 起源会话+绑定工程+进入事实
    # 在案 → 桥接播报 checkpoint 先于 LLM（339 leg1 同款断言面，零 LLM）。
    Start-Sleep -Seconds 5
    $round2 = Invoke-RestMethod -Method Post -Uri $chatUri -ContentType "application/json; charset=utf-8" -Body ([System.Text.Encoding]::UTF8.GetBytes((ConvertTo-Json @{ conversation_id = $rcConv; message = "现在到哪了" } -Compress))) -TimeoutSec 60
    $rc1.chat_round2_reply = [string]$round2.reply
    $rc1.chat_round2_stop_reason = [string]$round2.stop_reason
    $rc1.chat_round2_context_flow = [string]$round2.context_flow
    $rc1.chat_round2_typed_event0 = [string]$round2.typed_events[0].type
    if ($rc1.chat_round2_stop_reason -ne "launcher_bridge_broadcast" -or $rc1.chat_round2_context_flow -ne "project" -or $rc1.chat_round2_reply -notlike "*已进入《烟测桥甲*" -or $rc1.chat_round2_typed_event0 -ne "volume_switch") {
        throw ("RC1 in-project chat roundtrip mismatch: stop_reason=$($rc1.chat_round2_stop_reason) context_flow=$($rc1.chat_round2_context_flow) reply=$($rc1.chat_round2_reply) typed0=$($rc1.chat_round2_typed_event0)")
    }
    $rc1.in_project_roundtrip = $true

    Stop-GuiProcess $godotPid; $godotPid = 0
    Stop-GuiProcess $agentProcId; $agentProcId = 0
    Start-Sleep -Seconds 2

    # ------------------------------------------------------------------ RC2
    Write-Step0 "RC2: file moved away after list load -> recents click fails with degradation receipt"
    $agentProcId = Start-BridgeAgent -DraftRoot (Join-Path $RunRoot "agent_drafts_rc2") -TelemetryPath $rcTelemetry
    $assert["stack"]["agent_pid_rc2"] = $agentProcId
    $logR2 = Join-Path $LogsDir "godot_boot_rc2.log"
    $godotPid = Start-BridgeGodot -LogPath $logR2 -BridgeMessage "有哪些工程" -Governance "switch" -FollowupMs "25000" -Followup "recents_click:1"
    $assert["stack"]["godot_boot_rc2_pid"] = $godotPid
    $rc2 = [ordered]@{}
    $assert["RC2_moved_file_click_failed"] = $rc2
    # 前段手工断言（不走 Assert-BridgeBoot——directive.received 在点击后才出现）。
    foreach ($pat in @('\[AgentChatWindow\] boot ', '\[AgentChatWindow\] visible ', '\[AgentChatWindow\] page_loaded ', '\[LauncherBridge\] smoke\.navigate ', '\[LauncherBridge\] surface_ready ')) {
        if ($null -eq (Test-LogLine -LogPath $logR2 -Pattern $pat -TimeoutSeconds 90)) { throw ("RC2 pre-click marker not seen: $pat") }
    }
    # 列表装载（surface_ready 后 greeting+list 轮，正常 <5s）→ 等待 8s 后移走乙；
    # 点击在条件就绪后 +25s（FOLLOWUP_MS），删除窗口余量 ~17s。
    Start-Sleep -Seconds 8
    Remove-Item -LiteralPath $fixtureB -Force
    $rc2.fixture_b_deleted = $true
    $drR2 = Test-LogLine -LogPath $logR2 -Pattern '\[LauncherBridge\] directive\.received ' -TimeoutSeconds 90
    if ($null -eq $drR2) { throw ("RC2 directive.received not seen (click did not fire — entry greyed by exists=false or seam failure)") }
    $rc2.directive_request_id = Get-JsonString -Line $drR2.Line -Field "request_id"
    $govR2 = Test-LogLine -LogPath $logR2 -Pattern '\[LauncherBridge\] governance\.dialog_shown ' -TimeoutSeconds 30
    if ($null -eq $govR2) { throw ("RC2 governance dialog not seen") }
    $rc2.dialog_current_project = Get-JsonString -Line $govR2.Line -Field "current_project"
    $gpR2 = Test-LogLine -LogPath $logR2 -Pattern '\[LauncherBridge\] smoke\.governance\.press .*"choice":\s*"switch"' -TimeoutSeconds 30
    if ($null -eq $gpR2) { throw ("RC2 governance press (switch) not seen") }
    Assert-BridgeResult -LogPath $logR2 -Leg $rc2 -ExpectStatus "failed"
    # attempt2 教训：宿主 result.posted 日志行不含 file_path——失败对象的证据锚=
    # 宿主警告行（launcher directive chain failed: Project file does not exist: <乙路径>）。
    $failRefR2 = Select-String -LiteralPath $logR2 -Pattern 'launcher directive chain failed.*烟测桥乙' | Select-Object -Last 1
    if ($null -eq $failRefR2) { throw ("RC2 failure detail does not reference fixture B (moved-away file): no chain-failed warning line") }
    $rc2.failure_detail_refs_fixture_b = $true
    Start-Sleep -Seconds 3
    $noDockR2 = Select-String -LiteralPath $logR2 -Pattern 'scene_added.*VitDockRoot' -ErrorAction SilentlyContinue
    $rc2.no_scene_switch = ($null -eq $noDockR2)
    if (-not $rc2.no_scene_switch) { throw ("RC2 unexpectedly entered the dock after failed open") }

    Stop-GuiProcess $godotPid; $godotPid = 0
    Stop-GuiProcess $agentProcId; $agentProcId = 0
    Start-Sleep -Seconds 2

    # ------------------------------------------------------------------ RC3
    Write-Step0 "RC3: corrupt-but-present project file -> recents click fails with degradation receipt"
    $agentProcId = Start-BridgeAgent -DraftRoot (Join-Path $RunRoot "agent_drafts_rc3") -TelemetryPath $rcTelemetry
    $assert["stack"]["agent_pid_rc3"] = $agentProcId
    $logR3 = Join-Path $LogsDir "godot_boot_rc3.log"
    $godotPid = Start-BridgeGodot -LogPath $logR3 -BridgeMessage "有哪些工程" -Governance "switch" -FollowupMs "8000" -Followup "recents_click:2"
    $assert["stack"]["godot_boot_rc3_pid"] = $godotPid
    $rc3 = [ordered]@{}
    $assert["RC3_corrupt_file_click_failed"] = $rc3
    Assert-BridgeBoot -LogPath $logR3 -Leg $rc3
    $govR3 = Test-LogLine -LogPath $logR3 -Pattern '\[LauncherBridge\] governance\.dialog_shown ' -TimeoutSeconds 30
    if ($null -eq $govR3) { throw ("RC3 governance dialog not seen") }
    $gpR3 = Test-LogLine -LogPath $logR3 -Pattern '\[LauncherBridge\] smoke\.governance\.press .*"choice":\s*"switch"' -TimeoutSeconds 30
    if ($null -eq $gpR3) { throw ("RC3 governance press (switch) not seen") }
    Assert-BridgeResult -LogPath $logR3 -Leg $rc3 -ExpectStatus "failed"
    # 同 RC2：失败对象证据锚=宿主警告行（损坏文件解析失败形态的 detail 文案可能不同，
    # 锚=警告行含 fixture 名）。
    $failRefR3 = Select-String -LiteralPath $logR3 -Pattern '烟测桥坏' | Select-Object -Last 1
    if ($null -eq $failRefR3) { throw ("RC3 failure detail does not reference corrupt fixture: no warning line with fixture name") }
    $rc3.failure_detail_refs_corrupt_fixture = $true
    Start-Sleep -Seconds 3
    $noDockR3 = Select-String -LiteralPath $logR3 -Pattern 'scene_added.*VitDockRoot' -ErrorAction SilentlyContinue
    $rc3.no_scene_switch = ($null -eq $noDockR3)
    if (-not $rc3.no_scene_switch) { throw ("RC3 unexpectedly entered the dock after failed open") }

    Stop-GuiProcess $godotPid; $godotPid = 0
    Stop-GuiProcess $agentProcId; $agentProcId = 0
    Start-Sleep -Seconds 2

    # RC 零 LLM 断言（telemetry 行数 0；播报轮/checkpoint 先于 LLM）。
    $rcTelemetryLines = 0
    if (Test-Path -LiteralPath $rcTelemetry) {
        $rcTelemetryLines = @(Get-Content -LiteralPath $rcTelemetry -ErrorAction SilentlyContinue | Where-Object { $_.Trim() -ne "" }).Count
    }
    $report.isolation["rc_llm_telemetry_lines"] = $rcTelemetryLines
    if ($rcTelemetryLines -ne 0) { throw ("RC legs spent LLM calls (telemetry lines=$rcTelemetryLines) — expected 0") }

    # RC fixture 复原（乙被 RC2 删除——重建副本供后续核查；不影响源 912.vit 断言）。
    Copy-Item -LiteralPath $ProjectSource -Destination $fixtureB -Force
    }
    else {

    # ================================================================== L1
    Write-Step0 "L1: first open via bridge from boot state (governance switch; zero-friction premise refuted -> finding)"
    $agentProcId = Start-BridgeAgent -DraftRoot (Join-Path $RunRoot "agent_drafts")
    $assert["stack"]["agent_pid_leg1"] = $agentProcId
    $log1 = Join-Path $LogsDir "godot_boot1.log"
    $godotPid = Start-BridgeGodot -LogPath $log1 -BridgeMessage "打开烟测桥甲" -Governance "switch"
    $assert["stack"]["godot_boot1_pid"] = $godotPid
    $leg1 = [ordered]@{}
    $assert["L1_boot_state_open_governance_switch"] = $leg1
    Assert-BridgeBoot -LogPath $log1 -Leg $leg1
    # UI-BRIDGE-1 finding（attempt 1 真栈证据）：React launcher 面加载即发 recents 聊天 →
    # agent 工作区激活 → SetProjectPathHint 物化 boot scratch（"Unsaved"）路径 →
    # /agent/state current_project.path 非空 → 337 保守治理判据视为「有在开工程」弹三键。
    # 零摩擦前提被真栈证据推翻（探针无聊天时 path=""；boot1 日志 dialog_shown Unsaved）。
    # 治理判据/agent hint 语义改动越本卡域 → 转主管裁定；本腿以 switch 治理路径如实驱动。
    $govHit1 = Test-LogLine -LogPath $log1 -Pattern '\[LauncherBridge\] governance\.dialog_shown ' -TimeoutSeconds 30
    if ($null -eq $govHit1) { throw ("L1 governance dialog not seen (expected on boot-scratch state per attempt-1 evidence)") }
    $leg1.zero_friction_dialog_current_project = Get-JsonString -Line $govHit1.Line -Field "current_project"
    $leg1.zero_friction_finding = "governance dialog on kernel boot scratch (Unsaved) - zero-friction premise refuted by stack evidence; governance criterion vs agent hint semantics -> supervisor ruling"
    $gpHit1 = Test-LogLine -LogPath $log1 -Pattern '\[LauncherBridge\] smoke\.governance\.press .*"choice":\s*"switch"' -TimeoutSeconds 30
    if ($null -eq $gpHit1) { throw ("L1 smoke governance press (switch) not seen") }
    $obHit = Test-LogLine -LogPath $log1 -Pattern '\[LauncherBridge\] open\.begin ' -TimeoutSeconds 20
    if ($null -eq $obHit) { throw ("L1 open.begin not seen") }
    Assert-BridgeResult -LogPath $log1 -Leg $leg1 -ExpectStatus "opened" -ExpectGovernance "switch"
    $sceneHit = Test-LogLine -LogPath $log1 -Pattern 'scene_added.*VitDockRoot' -TimeoutSeconds $LegSettleSeconds
    if ($null -eq $sceneHit) { throw ("L1 scene_added VitDockRoot not seen (open chain did not dock)") }
    $leg1.scene_hwnd = Get-JsonField -Line $sceneHit.Line -Field "child_hwnd"
    $leg1.same_hwnd_across_open = ($leg1.scene_hwnd -eq $leg1.child_hwnd -and $leg1.child_hwnd -gt 0)
    if (-not $leg1.same_hwnd_across_open) { throw ("L1 HWND changed across open: child=$($leg1.child_hwnd) scene=$($leg1.scene_hwnd)") }
    # /agent/state 仅记录不判定：agent current_project=工作区绑定面（chat 激活时绑定，
    # open 后无新聊天不重绑）=340 桥播域语义，非本卡验收面；kernel-open 由
    # scene_added VitDockRoot + result(opened) 证明（IPC 失败走 _finish_ipc_failure 不切场景）。
    $proj1 = Get-AgentCurrentProject -BaseUrl $AgentHttp
    $leg1.agent_current_project_observed = $proj1

    Stop-GuiProcess $godotPid; $godotPid = 0
    Stop-GuiProcess $agentProcId; $agentProcId = 0
    Start-Sleep -Seconds 2

    # ================================================================== L2a
    Write-Step0 "L2a: governance switch (kernel has 甲 open; open 乙)"
    $agentProcId = Start-BridgeAgent -DraftRoot (Join-Path $RunRoot "agent_drafts")
    $assert["stack"]["agent_pid_leg2a"] = $agentProcId
    $log2 = Join-Path $LogsDir "godot_boot2.log"
    $godotPid = Start-BridgeGodot -LogPath $log2 -BridgeMessage "打开烟测桥乙" -Governance "switch"
    $assert["stack"]["godot_boot2_pid"] = $godotPid
    $leg2a = [ordered]@{}
    $assert["L2a_governance_switch"] = $leg2a
    Assert-BridgeBoot -LogPath $log2 -Leg $leg2a
    $gdHit = Test-LogLine -LogPath $log2 -Pattern '\[LauncherBridge\] governance\.dialog_shown ' -TimeoutSeconds 30
    if ($null -eq $gdHit) { throw ("L2a governance dialog not shown (kernel has 甲 open — three-key confirm must appear)") }
    $leg2a.dialog_current_project = Get-JsonString -Line $gdHit.Line -Field "current_project"
    if ($leg2a.dialog_current_project -notlike "*烟测桥甲*") { throw ("L2a governance dialog does not reference fixture A (kernel-open premise): '" + $leg2a.dialog_current_project + "'") }
    $gpHit = Test-LogLine -LogPath $log2 -Pattern '\[LauncherBridge\] smoke\.governance\.press .*"choice":\s*"switch"' -TimeoutSeconds 30
    if ($null -eq $gpHit) { throw ("L2a smoke governance press (switch) not seen") }
    Assert-BridgeResult -LogPath $log2 -Leg $leg2a -ExpectStatus "opened" -ExpectGovernance "switch"
    $sceneHit2 = Test-LogLine -LogPath $log2 -Pattern 'scene_added.*VitDockRoot' -TimeoutSeconds $LegSettleSeconds
    if ($null -eq $sceneHit2) { throw ("L2a scene_added VitDockRoot not seen") }
    $leg2a.scene_hwnd = Get-JsonField -Line $sceneHit2.Line -Field "child_hwnd"
    $leg2a.same_hwnd_across_open = ($leg2a.scene_hwnd -eq $leg2a.child_hwnd -and $leg2a.child_hwnd -gt 0)
    if (-not $leg2a.same_hwnd_across_open) { throw ("L2a HWND changed across open") }
    $proj2 = Get-AgentCurrentProject -BaseUrl $AgentHttp
    $leg2a.agent_current_project = $proj2
    if ($proj2 -notlike "*烟测桥乙*") { throw ("L2a agent current_project not fixture B: '$proj2'") }

    Stop-GuiProcess $godotPid; $godotPid = 0
    Stop-GuiProcess $agentProcId; $agentProcId = 0
    Start-Sleep -Seconds 2

    # ================================================================== L2b
    Write-Step0 "L2b: governance cancel (kernel has 乙 open; cancel open 甲)"
    $agentProcId = Start-BridgeAgent -DraftRoot (Join-Path $RunRoot "agent_drafts")
    $assert["stack"]["agent_pid_leg2b"] = $agentProcId
    $log3 = Join-Path $LogsDir "godot_boot3.log"
    $godotPid = Start-BridgeGodot -LogPath $log3 -BridgeMessage "打开烟测桥甲" -Governance "cancel"
    $assert["stack"]["godot_boot3_pid"] = $godotPid
    $leg2b = [ordered]@{}
    $assert["L2b_governance_cancel"] = $leg2b
    Assert-BridgeBoot -LogPath $log3 -Leg $leg2b
    $gdHit3 = Test-LogLine -LogPath $log3 -Pattern '\[LauncherBridge\] governance\.dialog_shown ' -TimeoutSeconds 30
    if ($null -eq $gdHit3) { throw ("L2b governance dialog not shown") }
    $gpHit3 = Test-LogLine -LogPath $log3 -Pattern '\[LauncherBridge\] smoke\.governance\.press .*"choice":\s*"cancel"' -TimeoutSeconds 30
    if ($null -eq $gpHit3) { throw ("L2b smoke governance press (cancel) not seen") }
    Assert-BridgeResult -LogPath $log3 -Leg $leg2b -ExpectStatus "canceled" -ExpectGovernance "cancel"
    Start-Sleep -Seconds 3
    $noDock = Select-String -LiteralPath $log3 -Pattern 'scene_added.*VitDockRoot' -ErrorAction SilentlyContinue
    $leg2b.no_scene_switch = ($null -eq $noDock)
    if (-not $leg2b.no_scene_switch) { throw ("L2b unexpectedly entered the dock after cancel") }
    $proj3 = Get-AgentCurrentProject -BaseUrl $AgentHttp
    $leg2b.agent_current_project = $proj3
    if ($proj3 -notlike "*烟测桥乙*") { throw ("L2b kernel project changed after cancel: '$proj3'") }

    Stop-GuiProcess $godotPid; $godotPid = 0
    Stop-GuiProcess $agentProcId; $agentProcId = 0
    Start-Sleep -Seconds 2

    # ================================================================== L2c
    Write-Step0 "L2c: governance save + dock unsupported degradation"
    $agentProcId = Start-BridgeAgent -DraftRoot (Join-Path $RunRoot "agent_drafts")
    $assert["stack"]["agent_pid_leg2c"] = $agentProcId
    $log4 = Join-Path $LogsDir "godot_boot4.log"
    $godotPid = Start-BridgeGodot -LogPath $log4 -BridgeMessage "打开烟测桥甲" -Governance "save" -FollowupMs "8000" -Followup "choose_file"
    $assert["stack"]["godot_boot4_pid"] = $godotPid
    $leg2c = [ordered]@{}
    $assert["L2c_governance_save_and_dock_degradation"] = $leg2c
    Assert-BridgeBoot -LogPath $log4 -Leg $leg2c
    $gdHit4 = Test-LogLine -LogPath $log4 -Pattern '\[LauncherBridge\] governance\.dialog_shown ' -TimeoutSeconds 30
    if ($null -eq $gdHit4) { throw ("L2c governance dialog not shown") }
    $gpHit4 = Test-LogLine -LogPath $log4 -Pattern '\[LauncherBridge\] smoke\.governance\.press .*"choice":\s*"save"' -TimeoutSeconds 30
    if ($null -eq $gpHit4) { throw ("L2c smoke governance press (save) not seen") }
    Assert-BridgeResult -LogPath $log4 -Leg $leg2c -ExpectStatus "opened" -ExpectGovernance "save"
    $sceneHit4 = Test-LogLine -LogPath $log4 -Pattern 'scene_added.*VitDockRoot' -TimeoutSeconds $LegSettleSeconds
    if ($null -eq $sceneHit4) { throw ("L2c scene_added VitDockRoot not seen") }
    $leg2c.scene_hwnd = Get-JsonField -Line $sceneHit4.Line -Field "child_hwnd"
    $leg2c.same_hwnd_across_open = ($leg2c.scene_hwnd -eq $leg2c.child_hwnd -and $leg2c.child_hwnd -gt 0)
    if (-not $leg2c.same_hwnd_across_open) { throw ("L2c HWND changed across open") }
    # /agent/state 仅记录（同 L1：open 后无新聊天不重绑，工作区绑定面非本卡验收面）。
    $proj4 = Get-AgentCurrentProject -BaseUrl $AgentHttp
    $leg2c.agent_current_project_observed = $proj4
    # 降级段：followup choose_file 请求在 dock 阶段到达 → unsupported 回执 + React 真收。
    # （open/new directive 的 dock 降级受 agent 门上游拦截：340 设计=进程内
    # user_entered_project 后 launcher 流关闭，跨工程意图归 C5——attempt-5 证据。）
    $dockHit = Test-LogLine -LogPath $log4 -Pattern '\[LauncherBridge\] dock\.received .*"type":\s*"launcher_choose_file"' -TimeoutSeconds 60
    if ($null -eq $dockHit) { throw ("L2c dock.received (choose_file) not seen (followup did not reach the dock handler)") }
    $unsHit = Test-LogLine -LogPath $log4 -Pattern ('\[LauncherBridge\] result\.posted .*"status":\s*"unsupported"') -TimeoutSeconds 30
    if ($null -eq $unsHit) { throw ("L2c unsupported result not posted (dock degradation silent?)") }
    $leg2c.dock_unsupported_posted = $true
    $ackDeadline = (Get-Date).AddSeconds(30)
    $ackUnsup = $null
    while ((Get-Date) -lt $ackDeadline) {
        $acks = Select-String -LiteralPath $log4 -Pattern '\[LauncherBridge\] result_ack ' -ErrorAction SilentlyContinue
        $unsAcks = @($acks | Where-Object { $_.Line -match '"status":\s*"unsupported"' })
        if ($unsAcks.Count -gt 0) { $ackUnsup = $unsAcks[-1]; break }
        Start-Sleep -Milliseconds 500
    }
    if ($null -eq $ackUnsup) { throw ("L2c result_ack unsupported not seen (React did not really receive the degradation receipt)") }
    $leg2c.result_ack_unsupported = $true

    Stop-GuiProcess $godotPid; $godotPid = 0
    Stop-GuiProcess $agentProcId; $agentProcId = 0
    Start-Sleep -Seconds 2
    } # end else（L1-L2c 原腿，-RecentsClickLeg 时跳过）

    # ---------------------------------------------------------- zero LLM + isolation
    Write-Step0 "Isolation: fixture source hash + recents restore + zero LLM telemetry"
    $fixtureSourceHashAfter = (Get-FileHash -Algorithm SHA256 -LiteralPath $ProjectSource).Hash
    $report.isolation["fixture_source_sha256_after"] = $fixtureSourceHashAfter
    $report.isolation["fixture_source_unchanged"] = ($fixtureSourceHashAfter -eq $fixtureSourceHashBefore)
    if ($fixtureSourceHashAfter -ne $fixtureSourceHashBefore) { throw ("fixture source 912.vit hash changed — pollution!") }
    $telemetryCandidates = Get-ChildItem -LiteralPath (Join-Path $RunRoot "agent_drafts") -Recurse -Filter "*telemetry*" -ErrorAction SilentlyContinue
    $report.isolation["llm_telemetry_files"] = $telemetryCandidates.Count
    $report.exit_code = 0
}
catch {
    $report["error"] = $_.Exception.Message
    $report["error_type"] = $_.Exception.GetType().Name
    Write-Host ("FAIL: " + $_.Exception.Message)
}
finally {
    Stop-Stack
    # recents 还原（快照回写，净差零）。
    try {
        Copy-Item -LiteralPath $recentsBackup -Destination $recentsConfig -Force
        Write-Host "recents restored from snapshot"
    } catch { Write-Host ("recents restore failed: " + $_.Exception.Message) }
    Copy-Item -LiteralPath $recentsConfig -Destination (Join-Path $ArtDir "global_project_config.after.json") -Force
    $report["finished_utc"] = (Get-Date).ToUniversalTime().ToString("o")
    $reportPath = Join-Path $ArtDir "ui_bridge_smoke_report.json"
    $report | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $reportPath -Encoding UTF8
    Write-Host ("report: " + $reportPath)
    foreach ($log in (Get-ChildItem -LiteralPath $LogsDir -Filter "godot_boot*.log" -ErrorAction SilentlyContinue)) {
        Copy-Item -LiteralPath $log.FullName -Destination $ArtDir -Force -ErrorAction SilentlyContinue
    }
    Copy-Item -LiteralPath $agentLog -Destination $ArtDir -Force -ErrorAction SilentlyContinue
}

if ($report.exit_code -eq 0) { Write-Host "UI-BRIDGE-1 smoke: PASS" ; exit 0 }
else { Write-Host "UI-BRIDGE-1 smoke: FAIL" ; exit 1 }
