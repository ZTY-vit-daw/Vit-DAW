[CmdletBinding()]
param(
    # 真栈腿入口（W01-C）：未授权不运行。默认仅打印 runbook 并 exit 3。
    [string]$RepoRoot = "D:/Vit_DAW",
    [string]$AgentBaseUrl = "http://127.0.0.1:7878",
    [string]$DesignSourceDir = "D:/Vit_DAW/coord/lesson-designs/music-creation-unit2",
    [string]$ProjectA = "",
    [string]$ProjectB = "",
    [string]$RuntimeOwner = "",
    [int]$TimeoutSeconds = 120,
    [switch]$StackAuthorized,
    [switch]$SkipRecentsSnapshot,
    [string]$RunRoot = ""
)

# ============================================================================
# G0-LEGAME-BIND-1（W01-C）真栈烟测腿 —— RUNBOOK + 待授权执行器
#
# 状态：准备就绪，未运行（425 零栈实现段；真栈运行必须候主管登记
#       coord/resources/PC-RUNTIME-STACK.md 栈位并书面授权后执行）。
#
# 成功条件（全部满足 → exit 0）：
#   S1 导入设计源（A）：/agent/chat 回执 stop_reason=edu_jam_legame_imported；
#      项目包内 legame/cards/<id>/<id>.compiled.level.yaml + compile manifest 存在；
#      项目包 manifest.json 校验和自洽（manifest.files 全对账）。
#   S2 binding 按 project UUID 目录隔离：A/B 各 .vit_agent/<uuid>/legame_binding.json
#      路径不同，level_id 指向本工程编译卡；父目录旧 binding 不被新流程写入。
#   S3 乐局装载：A 工程「开始乐局」回执 edu_jam_entered，回复含编译卡标题；
#      workflow_data.level_id=编译卡 id；/agent/state session.project_uuid=A uuid。
#   S4 A/B 不串：B 导入/开局后 A 的 binding 哈希不变；A/B 项目包/编译卡路径不同。
#   S5 重开不切别版：重新 project.open(A) 后装载的编译卡 manifest cache_key
#      与导入时记录一致（相对路径不变 → 四元组命中，不重编译不换版）。
#   S6 revise 源→缓存失效→新卡生效：改包内 legame/design.v1.yaml 标题后开局，
#      回复标题=修订标题；compile manifest cache_key 变化；项目包校验和仍自洽。
#   S7 recents 快照还原：启动前快照 %APPDATA%/Vit-DAW/global_project_config.json
#      （或 VIT_DAW_USER_CONFIG_DIR 同名文件），teardown 还原并哈希一致；
#      还原失败=装置非零退出（环境中断分类，不得静默）。
#
# 前置（真栈执行方逐项确认并在 -RuntimeOwner 记录）：
#   1. 主管已在 coord/resources/PC-RUNTIME-STACK.md 登记本卡占用并书面授权；
#   2. 真实栈由主管按既有产品入口统一启停；本脚本只走 agent HTTP + 读文件，
#      不启动/不重启任何栈进程；
#   3. 工程 A/B 已存在（真实栈正常入口新建/保存得到 .vit），路径经
#      -ProjectA/-ProjectB 传入（同目录两工程亦可，本卡核心断言=不串 binding）。
#
# 运行命令（授权后，单行即可，脚本不依赖行内反引号续行）：
#   pwsh -NoProfile -ExecutionPolicy Bypass -File ./scripts/run_legame_bind_smoke.ps1 -RepoRoot D:/Vit_DAW -StackAuthorized -RuntimeOwner "<主管登记 owner>" -ProjectA "<A.vit>" -ProjectB "<B.vit>"
#
# 工件（每次新 runRoot，不覆盖旧 run）：
#   VitApp/Workspace/Artifacts/smoke/legame_bind_<stamp>/
#     summary.json / requests/*.json / responses/*.json
#     binding_A.json binding_B.json manifest_A.json
#     design_A.before.yaml design_A.revised.yaml
#     recents.before.json recents.after_restore.json
#   关键结论均回指上述原始工件。
# ============================================================================

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Write-Step { param([string]$Message) Write-Host ""; Write-Host ("== " + $Message) -ForegroundColor Cyan }
function Write-Ok { param([string]$Message) Write-Host ("ok: " + $Message) -ForegroundColor Green }
function Fail { param([string]$Message) throw $Message }

if (-not $StackAuthorized) {
    Write-Host "G0-LEGAME-BIND-1（W01-C）真栈烟测腿：RUNBOOK 已就位，未授权不运行。" -ForegroundColor Yellow
    Write-Host "  须主管登记 coord/resources/PC-RUNTIME-STACK.md 栈位并书面授权后执行；" -ForegroundColor Yellow
    Write-Host "  授权后追加 -StackAuthorized -RuntimeOwner '<owner>' -ProjectA <A.vit> -ProjectB <B.vit>。" -ForegroundColor Yellow
    Write-Host "  本脚本不启动/重启任何栈进程（只走 agent HTTP + 读文件 + recents 快照还原）。" -ForegroundColor Yellow
    exit 3
}

$RepoRoot = (Resolve-Path -LiteralPath $RepoRoot).Path
$DesignSourceDir = (Resolve-Path -LiteralPath $DesignSourceDir).Path
if ([string]::IsNullOrWhiteSpace($RuntimeOwner)) {
    Fail "真栈执行必须在 -RuntimeOwner 记录主管登记的栈位 owner（防未登记占用）"
}
if ([string]::IsNullOrWhiteSpace($ProjectA) -or [string]::IsNullOrWhiteSpace($ProjectB)) {
    Fail "需要 -ProjectA/-ProjectB 两个已存在工程（真实栈正常入口创建/保存）"
}
$ProjectA = (Resolve-Path -LiteralPath $ProjectA).Path
$ProjectB = (Resolve-Path -LiteralPath $ProjectB).Path
if ([string]::IsNullOrWhiteSpace($RunRoot)) {
    $stamp = Get-Date -Format "yyyyMMdd_HHmmss"
    $RunRoot = Join-Path (Join-Path (Join-Path (Join-Path $RepoRoot "VitApp") "Workspace") "Artifacts") ("smoke/legame_bind_" + $stamp)
}
$RunRoot = [System.IO.Path]::GetFullPath($RunRoot)
New-Item -ItemType Directory -Path $RunRoot -Force | Out-Null
New-Item -ItemType Directory -Path (Join-Path $RunRoot "requests") -Force | Out-Null
New-Item -ItemType Directory -Path (Join-Path $RunRoot "responses") -Force | Out-Null

$summary = [ordered]@{
    schema_version = "vit_legame_bind_smoke.v1"
    card = "G0-LEGAME-BIND-1"
    status = "running"
    runtime_owner = $RuntimeOwner
    created_at = (Get-Date).ToString("o")
    run_root = $RunRoot
    project_a = $ProjectA
    project_b = $ProjectB
    design_source_dir = $DesignSourceDir
    import = $null
    enter = $null
    a_b_isolation = $null
    reopen = $null
    revise = $null
    recents = $null
    error = $null
}

function Get-RecentsPath {
    if (-not [string]::IsNullOrWhiteSpace($env:VIT_DAW_USER_CONFIG_DIR)) {
        return (Join-Path $env:VIT_DAW_USER_CONFIG_DIR "global_project_config.json")
    }
    if (-not [string]::IsNullOrWhiteSpace($env:APPDATA)) {
        return (Join-Path (Join-Path $env:APPDATA "Vit-DAW") "global_project_config.json")
    }
    return ""
}

function Get-FileSha256([string]$Path) {
    if ([string]::IsNullOrWhiteSpace($Path) -or -not (Test-Path -LiteralPath $Path)) { return "" }
    return (Get-FileHash -Algorithm SHA256 -LiteralPath $Path).Hash
}

$recentsPath = Get-RecentsPath
$recentsBeforeHash = ""
$recentsSnapshot = $null
if (-not $SkipRecentsSnapshot -and -not [string]::IsNullOrWhiteSpace($recentsPath)) {
    $recentsBeforeHash = Get-FileSha256 $recentsPath
    if (Test-Path -LiteralPath $recentsPath) {
        $recentsSnapshot = Join-Path $RunRoot "recents.before.json"
        Copy-Item -LiteralPath $recentsPath -Destination $recentsSnapshot -Force
    }
}

function Invoke-AgentJson {
    param([string]$Name, [string]$Method, [string]$Path, $Body)
    $uri = ($AgentBaseUrl.TrimEnd("/") + $Path)
    $requestFile = Join-Path (Join-Path $RunRoot "requests") ($Name + ".json")
    $responseFile = Join-Path (Join-Path $RunRoot "responses") ($Name + ".json")
    if ($null -ne $Body) {
        $json = ConvertTo-Json -InputObject $Body -Depth 32 -Compress
        $json | Set-Content -LiteralPath $requestFile -Encoding UTF8
        $bytes = [System.Text.Encoding]::UTF8.GetBytes($json)
        $response = Invoke-RestMethod -Method $Method -Uri $uri -ContentType "application/json; charset=utf-8" -Body $bytes -TimeoutSec $TimeoutSeconds
    } else {
        $response = Invoke-RestMethod -Method $Method -Uri $uri -TimeoutSec $TimeoutSeconds
    }
    $response | ConvertTo-Json -Depth 32 | Set-Content -LiteralPath $responseFile -Encoding UTF8
    return $response
}

function Invoke-AgentChat {
    param([string]$Name, [string]$ConversationID, [string]$Message)
    return (Invoke-AgentJson -Name $Name -Method "Post" -Path "/agent/chat" -Body ([ordered]@{
        conversation_id = $ConversationID
        message = $Message
        context = @{}
    }))
}

function Get-ProjectState {
    $suffix = [guid]::NewGuid().ToString("N").Substring(0,8)
    return (Invoke-AgentJson -Name ("state_" + $suffix) -Method "Get" -Path "/agent/state" -Body $null)
}

function Get-ActiveProjectPath([object]$State) {
    if ($null -eq $State) { return "" }
    if ($State.current_project -and -not [string]::IsNullOrWhiteSpace([string]$State.current_project.path)) { return [string]$State.current_project.path }
    if ($State.project_history -and -not [string]::IsNullOrWhiteSpace([string]$State.project_history.project_path)) { return [string]$State.project_history.project_path }
    if ($State.shadow -and -not [string]::IsNullOrWhiteSpace([string]$State.shadow.project_path)) { return [string]$State.shadow.project_path }
    return ""
}

function Get-ActiveProjectUUID([object]$State) {
    if ($null -eq $State) { return "" }
    if ($State.current_project -and -not [string]::IsNullOrWhiteSpace([string]$State.current_project.uuid)) { return [string]$State.current_project.uuid }
    if ($State.project_history -and -not [string]::IsNullOrWhiteSpace([string]$State.project_history.project_uuid)) { return [string]$State.project_history.project_uuid }
    if ($State.shadow -and -not [string]::IsNullOrWhiteSpace([string]$State.shadow.project_uuid)) { return [string]$State.shadow.project_uuid }
    return ""
}

function Get-ProjectPackage([string]$ProjectPath) {
    $dir = Split-Path -Parent $ProjectPath
    $base = [System.IO.Path]::GetFileNameWithoutExtension($ProjectPath)
    $candidates = @(
        (Join-Path $dir ".vit_project"),
        (Join-Path $dir ($base + ".vit_project"))
    )
    foreach ($candidate in $candidates) {
        $manifestPath = Join-Path $candidate "manifest.json"
        if (Test-Path -LiteralPath $manifestPath) {
            return [pscustomobject]@{ PackageDir = $candidate; ManifestPath = $manifestPath }
        }
    }
    return $null
}

function Read-JsonFile([string]$Path) {
    return (Get-Content -LiteralPath $Path -Raw -Encoding UTF8 | ConvertFrom-Json)
}

function Assert-True([bool]$Condition, [string]$Message) {
    if (-not $Condition) { Fail $Message }
}

function Assert-PackageManifest([string]$ManifestPath) {
    $manifest = Read-JsonFile $ManifestPath
    $packageDir = Split-Path -Parent $ManifestPath
    $files = @($manifest.files)
    Assert-True ($files.Count -gt 0) ("项目包 manifest.files 为空: " + $ManifestPath)
    foreach ($entry in $files) {
        $relative = ([string]$entry.path).Replace("/", [string][System.IO.Path]::DirectorySeparatorChar)
        $target = Join-Path $packageDir $relative
        Assert-True (Test-Path -LiteralPath $target) ("项目包文件缺失: " + $target)
        Assert-True ((Get-FileSha256 $target) -eq ([string]$entry.sha256)) ("项目包校验和不符: " + $target)
    }
    return $manifest
}

function Get-LegameCompiled([string]$PackageDir) {
    $legameDir = Join-Path $PackageDir "legame"
    Assert-True (Test-Path -LiteralPath $legameDir) ("项目包缺 legame 装配目录: " + $legameDir)
    $cardsRoot = Join-Path $legameDir "cards"
    $cardDirs = @(Get-ChildItem -LiteralPath $cardsRoot -Directory | Sort-Object Name | Select-Object -First 1)
    Assert-True ($cardDirs.Count -eq 1) ("legame/cards 须恰有一个卡目录: " + $cardsRoot)
    $cardPath = @(Get-ChildItem -LiteralPath $cardDirs[0].FullName -Filter "*.compiled.level.yaml")
    $manifestPath = @(Get-ChildItem -LiteralPath $cardDirs[0].FullName -Filter "*.compile.manifest.json")
    Assert-True (($cardPath.Count -eq 1) -and ($manifestPath.Count -eq 1)) ("编译卡/manifest 缺失: " + $cardDirs[0].FullName)
    return [pscustomobject]@{
        CardDir = $cardDirs[0].FullName
        CardPath = $cardPath[0].FullName
        ManifestPath = $manifestPath[0].FullName
    }
}

function Get-BindingPath([string]$ProjectPath, [string]$ProjectUUID) {
    return (Join-Path (Join-Path (Join-Path (Split-Path -Parent $ProjectPath) ".vit_agent") $ProjectUUID) "legame_binding.json")
}

function Get-DesignFilePath([string]$PackageDir) {
    return (Join-Path (Join-Path $PackageDir "legame") "design.v1.yaml")
}

function Open-Project([string]$Name, [string]$ProjectPath) {
    $null = Invoke-AgentJson -Name $Name -Method "Post" -Path "/agent/invoke" -Body ([ordered]@{
        tool = "project.open"
        args = @{ file_path = $ProjectPath }
        confirmed = $true
        source = "legame_bind_smoke"
    })
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        Start-Sleep -Milliseconds 500
        $state = Get-ProjectState
        $activePath = Get-ActiveProjectPath $state
        if ($state -and $activePath -and ([System.IO.Path]::GetFullPath($activePath) -eq $ProjectPath)) {
            return $state
        }
    }
    Fail ("project.open 未在超时内完成: " + $ProjectPath)
}

function Import-DesignSource([string]$Name, [string]$DesignSource) {
    $conv = "legame-bind-" + [guid]::NewGuid().ToString("N").Substring(0,8)
    return (Invoke-AgentChat -Name $Name -ConversationID $conv -Message ("导入乐局设计源 " + $DesignSource + " tempo=80 turns=60 calls=120"))
}

try {
    Write-Step "S0 preflight（agent HTTP + 栈身份；本脚本不启停栈）"
    $health = Invoke-AgentJson -Name "health" -Method "Get" -Path "/health" -Body $null
    Assert-True ($null -ne $health) "agent /health 不可达"
    # VspHub(8787) 也有 /health；必须证明 agent 面可达（/agent/state），
    # 否则 S1 的 /agent/chat 才会以 404 暴露端口错配。
    try { $null = Get-ProjectState } catch { Fail ("agent 面不可达（" + $AgentBaseUrl + "）：" + $_.Exception.Message) }

    Write-Step "S1 导入设计源到工程 A（编译装配进包）"
    $null = Open-Project -Name "open_a" -ProjectPath $ProjectA
    $importA = Import-DesignSource -Name "import_a" -DesignSource $DesignSourceDir
    Assert-True ($importA.stop_reason -eq "edu_jam_legame_imported") ("A 导入 stop_reason=" + $importA.stop_reason + " reply=" + $importA.reply)
    $packageA = Get-ProjectPackage $ProjectA
    Assert-True ($null -ne $packageA) "A 项目包不存在"
    $manifestA = Assert-PackageManifest $packageA.ManifestPath
    Assert-True (($null -ne $manifestA.legame) -and [bool]$manifestA.legame.enabled) "A 项目包 manifest 未启用 legame"
    $compiledA = Get-LegameCompiled $packageA.PackageDir
    $compiledManifestA = Read-JsonFile $compiledA.ManifestPath
    $summary.import = [ordered]@{
        stop_reason = $importA.stop_reason
        card_id = $importA.workflow_data.card_id
        package_dir = $packageA.PackageDir
        card_path = $compiledA.CardPath
        cache_key = $compiledManifestA.cache_key
        warnings = $importA.workflow_data.warnings
        infos = $importA.workflow_data.infos
    }
    Copy-Item -LiteralPath $compiledA.ManifestPath -Destination (Join-Path $RunRoot "manifest_A.json") -Force

    Write-Step "S2 binding 按 project UUID 目录隔离（A/B 不串）"
    $stateA = Get-ProjectState
    $uuidA = Get-ActiveProjectUUID $stateA
    Assert-True (-not [string]::IsNullOrWhiteSpace($uuidA)) "A active_project_uuid 为空"
    $bindingA = Get-BindingPath -ProjectPath $ProjectA -ProjectUUID $uuidA
    Assert-True (Test-Path -LiteralPath $bindingA) ("A UUID binding 缺失: " + $bindingA)
    Copy-Item -LiteralPath $bindingA -Destination (Join-Path $RunRoot "binding_A.json") -Force
    $bindingAJson = Read-JsonFile $bindingA
    Assert-True ([string]$bindingAJson.project_uuid -eq $uuidA) "A binding.project_uuid 与工程 uuid 不符"
    $legacyBinding = Join-Path (Join-Path (Split-Path -Parent $ProjectA) ".vit_agent") "legame_binding.json"
    Assert-True (-not (Test-Path -LiteralPath $legacyBinding)) ("新流程不得写父目录旧 binding: " + $legacyBinding)

    Write-Step "S3 乐局装载（A 工程）"
    $enterA = Invoke-AgentChat -Name "enter_a" -ConversationID "legame-bind-a-owner" -Message "开始乐局"
    Assert-True ($enterA.stop_reason -eq "edu_jam_entered") ("A 开局 stop_reason=" + $enterA.stop_reason + " reply=" + $enterA.reply)
    $summary.enter = [ordered]@{ level_id = $enterA.workflow_data.level_id; stop_reason = $enterA.stop_reason }

    Write-Step "S4 B 工程导入/开局（A/B 隔离亲证）"
    $null = Open-Project -Name "open_b" -ProjectPath $ProjectB
    $importB = Import-DesignSource -Name "import_b" -DesignSource $DesignSourceDir
    Assert-True ($importB.stop_reason -eq "edu_jam_legame_imported") ("B 导入 stop_reason=" + $importB.stop_reason)
    $stateB = Get-ProjectState
    $uuidB = Get-ActiveProjectUUID $stateB
    Assert-True ($uuidA -ne $uuidB) "A/B 工程 uuid 相同，隔离断言无意义"
    $bindingB = Get-BindingPath -ProjectPath $ProjectB -ProjectUUID $uuidB
    Assert-True (Test-Path -LiteralPath $bindingB) ("B UUID binding 缺失: " + $bindingB)
    Copy-Item -LiteralPath $bindingB -Destination (Join-Path $RunRoot "binding_B.json") -Force
    Assert-True ($bindingA -ne $bindingB) "A/B binding 路径相同（串工程）"
    $packageB = Get-ProjectPackage $ProjectB
    Assert-True ($null -ne $packageB) "B 项目包不存在"
    Assert-True ($packageA.PackageDir -ne $packageB.PackageDir) "A/B 项目包同路径"
    $compiledB = Get-LegameCompiled $packageB.PackageDir
    Assert-True ($compiledA.CardPath -ne $compiledB.CardPath) "A/B 编译卡同路径（串卡）"
    $null = Assert-PackageManifest $packageB.ManifestPath
    $bindingAAfter = Read-JsonFile $bindingA
    Assert-True ([string]$bindingAAfter.level_id -eq [string]$bindingAJson.level_id) "B 导入改写了 A 的 binding（串工程）"
    $summary.a_b_isolation = [ordered]@{
        uuid_a = $uuidA
        uuid_b = $uuidB
        binding_a = $bindingA
        binding_b = $bindingB
        card_a = $compiledA.CardPath
        card_b = $compiledB.CardPath
    }

    Write-Step "S5 重开不切别版（A 重新打开后四元组命中）"
    $null = Open-Project -Name "open_a_again" -ProjectPath $ProjectA
    # 命令词「乐局状态」按设计跳过自动开场（eduJamAutoIntroCheckpoint 只接
    # 自然语句），不能作为重开装载取证；用新会话自然语句触发 auto-intro。
    $reopenConv = "legame-bind-a-reopen-" + [guid]::NewGuid().ToString("N").Substring(0,8)
    $enterA2 = Invoke-AgentChat -Name "enter_a2" -ConversationID $reopenConv -Message "我回来了"
    Assert-True (($enterA2.stop_reason -eq "edu_jam_entered") -or ($enterA2.stop_reason -eq "edu_jam_recovered")) ("重开后装载回执 stop_reason=" + $enterA2.stop_reason + " reply=" + $enterA2.reply)
    Assert-True ([string]$enterA2.workflow_data.level_id -eq [string]$summary.import.card_id) ("重开后装载卡不是导入卡: " + [string]$enterA2.workflow_data.level_id)
    $compiledA2 = Get-LegameCompiled $packageA.PackageDir
    $manifestA2 = Read-JsonFile $compiledA2.ManifestPath
    Assert-True (([string]$manifestA2.cache_key.design_source_sha256) -eq ([string]$compiledManifestA.cache_key.design_source_sha256)) "重开后设计源哈希变化（切版）"
    $summary.reopen = [ordered]@{ conversation_id = $reopenConv; stop_reason = $enterA2.stop_reason; level_id = $enterA2.workflow_data.level_id; cache_key = $manifestA2.cache_key }

    Write-Step "S6 revise 源→缓存失效→新卡生效（A）"
    $designPathA = Get-DesignFilePath $packageA.PackageDir
    Assert-True (Test-Path -LiteralPath $designPathA) ("A 设计源副本缺失: " + $designPathA)
    Copy-Item -LiteralPath $designPathA -Destination (Join-Path $RunRoot "design_A.before.yaml") -Force
    $designText = Get-Content -LiteralPath $designPathA -Raw -Encoding UTF8
    $marker = "revise-smoke-" + (Get-Date -Format "HHmmss")
    if ($designText -notmatch "(?m)^title:") { Fail "设计源缺 title 行，无法 revise" }
    $revisedText = [regex]::Replace($designText, "(?m)^title:.*$", ("title: " + $marker), 1)
    Set-Content -LiteralPath $designPathA -Value $revisedText -Encoding UTF8 -NoNewline
    Set-Content -LiteralPath (Join-Path $RunRoot "design_A.revised.yaml") -Value $revisedText -Encoding UTF8 -NoNewline
    $exitActive = Invoke-AgentChat -Name "exit_a_before_revise" -ConversationID $reopenConv -Message "结束乐局"
    Assert-True ($exitActive.stop_reason -eq "edu_jam_exited") ("revise 前结束乐局 stop_reason=" + $exitActive.stop_reason)
    $enterA3 = Invoke-AgentChat -Name "enter_a_after_revise" -ConversationID "legame-bind-a-revise" -Message "开始乐局"
    Assert-True ($enterA3.stop_reason -eq "edu_jam_entered") ("revise 后开局 stop_reason=" + $enterA3.stop_reason)
    Assert-True ($enterA3.reply -match [regex]::Escape($marker)) "revise 后装载卡标题未更新（新卡未生效）"
    $compiledA3 = Get-LegameCompiled $packageA.PackageDir
    $manifestA3 = Read-JsonFile $compiledA3.ManifestPath
    Assert-True (([string]$manifestA3.cache_key.design_source_sha256) -ne ([string]$compiledManifestA.cache_key.design_source_sha256)) "revise 后 cache_key 未变化（缓存未失效）"
    $null = Assert-PackageManifest $packageA.ManifestPath
    $summary.revise = [ordered]@{
        revised_title = $marker
        cache_key_before = $compiledManifestA.cache_key
        cache_key_after = $manifestA3.cache_key
        enter_stop_reason = $enterA3.stop_reason
    }

    $summary.status = "passed"
}
catch {
    $summary.status = "failed"
    $summary.error = $_.Exception.Message
    throw
}
finally {
    $recentsResult = [ordered]@{ path = $recentsPath; restored = $false; hash_before = $recentsBeforeHash; hash_after = "" }
    try {
        if (-not $SkipRecentsSnapshot -and -not [string]::IsNullOrWhiteSpace($recentsPath)) {
            if ($null -ne $recentsSnapshot) {
                $parent = Split-Path -Parent $recentsPath
                if (-not [string]::IsNullOrWhiteSpace($parent)) { New-Item -ItemType Directory -Path $parent -Force | Out-Null }
                Copy-Item -LiteralPath $recentsSnapshot -Destination $recentsPath -Force
                Copy-Item -LiteralPath $recentsPath -Destination (Join-Path $RunRoot "recents.after_restore.json") -Force
            } elseif (Test-Path -LiteralPath $recentsPath) {
                Remove-Item -LiteralPath $recentsPath -Force
            }
        }
        $recentsResult.hash_after = Get-FileSha256 $recentsPath
        $recentsResult.restored = $true
        Assert-True ($recentsResult.hash_before -eq $recentsResult.hash_after) "recents 还原哈希不一致（装置污染）"
    }
    catch {
        $recentsResult.restored = $false
        $recentsResult.error = $_.Exception.Message
        if ($summary.status -eq "passed") { $summary.status = "failed" }
        $summary.error = "recents restore failed: " + $_.Exception.Message
    }
    $summary.recents = $recentsResult
    $summary.completed_at = (Get-Date).ToString("o")
    $summary | ConvertTo-Json -Depth 32 | Set-Content -LiteralPath (Join-Path $RunRoot "summary.json") -Encoding UTF8
}

if ($summary.status -ne "passed") {
    throw ("G0-LEGAME-BIND-1 real-stack leg failed/blocked; see " + (Join-Path $RunRoot "summary.json"))
}
Write-Host "ok: G0-LEGAME-BIND-1 legame binding real-stack smoke passed" -ForegroundColor Green
Write-Host ("artifact: " + (Join-Path $RunRoot "summary.json"))
