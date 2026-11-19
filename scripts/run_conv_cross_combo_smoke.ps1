#Requires -Version 5.1
<#
CONV-CROSS-COMBO-1 (422): B2b cross-worktree real-stack combo smoke executor.

STATUS: apparatus only. The default invocation prints the runbook summary and
exits 3. Nothing is started, no port is opened and no stack is reused unless
the caller explicitly passes -StackAuthorized after the supervisor has
registered the stack berth in coord/resources/PC-RUNTIME-STACK.md and given
written authorization.

The script only uses the existing agent HTTP/tool surface:
  GET  /health
  GET  /agent/runtime/status
  GET  /agent/state
  GET  /agent/ui/state
  POST /agent/invoke  {tool, args, confirmed, source}

Deterministic / zero LLM. Project A is created with project.new + project.save_as
under a fresh RunRoot. The second worktree B is created with version.worktree_create.
Because a real /agent/chat turn would require the model, the target conversation
node is the product-generated worktree seed checkpoint node plus a product-
generated version.branch_create branch_marker node (from_node_id=seed). The
target branch is intentionally non-main so version.conversation_open exercises
the branch-checkout (non-detached) path.

Exit codes:
  0  PASS (all coverable assertions green; documented coverage gaps are listed)
  1  assertion / product failure
  2  environment / usage / contract failure (including reload-count unverifiable
     and recents/authority restore failure)
  3  unauthorized default invocation (runbook summary printed)
#>
[CmdletBinding()]
param(
    [string]$RepoRoot = "D:/Vit_DAW",
    [string]$AgentBaseUrl = "http://127.0.0.1:7878",
    [string]$ProjectA = "",
    [string]$RuntimeOwner = "",
    [int]$TimeoutSeconds = 180,
    [string]$RunRoot = "",
    [string]$KernelLogPath = "",
    [switch]$StackAuthorized,
    [switch]$DryRun
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$script:Card = "CONV-CROSS-COMBO-1"
$script:Schema = "vit_conv_cross_combo_smoke.v1"
$script:Assertions = New-Object System.Collections.ArrayList
$script:Warnings = New-Object System.Collections.ArrayList
$script:CurrentStep = "S0"
$script:RequestsDir = ""
$script:ResponsesDir = ""
$script:SummaryPath = ""
$script:RecentsPath = ""
$script:RecentsExisted = $false
$script:RecentsBeforeHash = ""
$script:RecentsSnapshot = ""
$script:WorktreeB = ""
$script:WorktreeC = ""
$script:WorktreeName = "cross-b"
$script:BranchName = "cross-combo"
$script:SeedNodeId = ""
$script:SeedCommit = ""
$script:BranchNodeId = ""
$script:Summary = $null

function Write-Step {
    param([string]$Message)
    Write-Host ""
    Write-Host ("== " + $Message) -ForegroundColor Cyan
}
function Write-Ok {
    param([string]$Message)
    Write-Host ("ok: " + $Message) -ForegroundColor Green
}
function Write-Warn {
    param([string]$Message)
    Write-Host ("warn: " + $Message) -ForegroundColor Yellow
}
function Write-Bad {
    param([string]$Message)
    Write-Host ("FAIL: " + $Message) -ForegroundColor Red
}

function Show-RunbookSummary {
    Write-Host "CONV-CROSS-COMBO-1 (422) real-stack combo smoke: RUNBOOK ONLY, NOT AUTHORIZED." -ForegroundColor Yellow
    Write-Host "  默认不启动/不复用任何栈，不占端口；无 -StackAuthorized 时 exit 3。" -ForegroundColor Yellow
    Write-Host ""
    Write-Host "前置（缺一即停）："
    Write-Host "  1) 主管在 coord/resources/PC-RUNTIME-STACK.md 登记栈位并书面授权；单 owner、单正式轮。"
    Write-Host "  2) 真实栈已由 owner 按产品入口启好（agent 7878 + kernel 5555/5556 + hub 8787）；本脚本不启停栈。"
    Write-Host "  3) 栈位为隔离 berth：无用户工程在编；RunRoot 下创建 A/B，不写用户工程；建议设置 VIT_HISTORY_DRAFT_ROOT。"
    Write-Host "  4) 记录 -RuntimeOwner；recents 与 Settings.xml/default_project.xml 启动前快照、finally 还原并哈希亲证。"
    Write-Host "  5) kernel 日志可读（默认 <RepoRoot>/VitApp/Workspace/Logs/VitHeadlessServer*.log；可 -KernelLogPath 指定）。"
    Write-Host ""
    Write-Host "步骤："
    Write-Host "  S0 preflight：health/runtime/state；栈空闲检查；recents/authority/kernel-log 快照；静态单 reload 锚点。"
    Write-Host "  S1 project.new + project.save_as 在 RunRoot 创建真实工程 A；version.status A 初始化核对。"
    Write-Host "  S2 version.worktree_create 创建第二工作树 B；version.branch_create 在 B 的目标节点上建非 main 分支与 branch_marker 节点；project.open 回 A。"
    Write-Host "  S3 version.status {project_path:B} 只读浏览：断言 B 路径/分支/节点/HEAD，且内核工程仍为 A、零 open_project、A/B 文件哈希不变。"
    Write-Host "  S4 version.conversation_open {project_file_path:B,branch,node_id}：断言 target_materialized=true、kernel_reloaded=true、"
    Write-Host "     目标工程路径/分支/节点/HEAD active、非 detached、单次 Command received open_project、DAW 投影一致。"
    Write-Host "  S5 失败语义：未知 worktree、失效 worktree 文件、未知 node/branch：target_materialized=false、零 reload、当前工程不变。"
    Write-Host "  S6 teardown：recents/authority 还原哈希亲证；装置零进程/零端口、只写 RunRoot；真栈由 owner 停栈后核对全零。"
    Write-Host ""
    Write-Host "授权后示例："
    Write-Host "  powershell -NoProfile -ExecutionPolicy Bypass -File .\\scripts\\run_conv_cross_combo_smoke.ps1 \\"
    Write-Host "    -RepoRoot D:/Vit_DAW -AgentBaseUrl http://127.0.0.1:7878 \\"
    Write-Host "    -RuntimeOwner '<registered owner>' -RunRoot '<fresh empty run dir>' -StackAuthorized"
    Write-Host ""
    Write-Host "干跑（零栈、零 HTTP，exit 0）："
    Write-Host "  powershell -NoProfile -ExecutionPolicy Bypass -File .\\scripts\\run_conv_cross_combo_smoke.ps1 -DryRun"
    Write-Host ""
    Write-Host "完整 runbook：coord/runs/CONV-CROSS-COMBO-1/RUNBOOK.md"
}

function Show-DryRunPlan {
    $plannedRunRoot = $RunRoot
    if ([string]::IsNullOrWhiteSpace($plannedRunRoot)) {
        $plannedRunRoot = Join-Path $RepoRoot ("coord/runs/CONV-CROSS-COMBO-1/run_<yyyyMMdd_HHmmss>_<8hex>")
    }
    $plannedA = $ProjectA
    if ([string]::IsNullOrWhiteSpace($plannedA)) {
        $plannedA = Join-Path $plannedRunRoot "workspace/ProjectA.vit"
    }
    Write-Host "CONV-CROSS-COMBO-1 (422) DRY RUN: no stack, no HTTP, no port, no filesystem writes." -ForegroundColor Cyan
    Write-Host ("  RepoRoot       : " + $RepoRoot)
    Write-Host ("  AgentBaseUrl   : " + $AgentBaseUrl)
    Write-Host ("  RuntimeOwner   : " + $RuntimeOwner)
    Write-Host ("  RunRoot (plan) : " + $plannedRunRoot)
    Write-Host ("  ProjectA (plan): " + $plannedA)
    Write-Host ("  KernelLogPath  : " + $(if ([string]::IsNullOrWhiteSpace($KernelLogPath)) { "<auto: RepoRoot/VitApp/Workspace/Logs/VitHeadlessServer*.log>" } else { $KernelLogPath }))
    Write-Host ("  TimeoutSeconds : " + $TimeoutSeconds)
    Write-Host "  Plan: S0 preflight -> S1 create/save A -> S2 worktree B + branch/node -> S3 read-only status(B) -> S4 conversation_open(B) -> S5 failure semantics -> S6 recents/authority teardown."
    Write-Host "  Exit: 0 PASS (coverable), 1 assertion/product, 2 environment/usage/contract, 3 unauthorized."
    Write-Host "  Authorization required for a real leg: -StackAuthorized -RuntimeOwner '<owner>'."
    exit 0
}

function Throw-Classified {
    param([string]$Class, [string]$Code, [string]$Message)
    $ex = New-Object System.Exception -ArgumentList $Message
    $ex.Data["vit_class"] = $Class
    $ex.Data["vit_code"] = $Code
    throw $ex
}

function Assert-That {
    param([string]$Id, [bool]$Condition, [string]$Detail, [string]$Code = "assertion")
    $row = [ordered]@{
        id = $Id
        ok = $Condition
        detail = $Detail
        step = $script:CurrentStep
        at = (Get-Date).ToString("o")
    }
    [void]$script:Assertions.Add($row)
    if (-not $Condition) {
        Throw-Classified "assertion_failure" $Code ("assertion failed: " + $Id + " :: " + $Detail)
    }
}

function Add-AssertionSafe {
    param([string]$Id, [bool]$Condition, [string]$Detail)
    $row = [ordered]@{
        id = $Id
        ok = $Condition
        detail = $Detail
        step = $script:CurrentStep
        at = (Get-Date).ToString("o")
    }
    [void]$script:Assertions.Add($row)
    return $Condition
}

function Add-Warning {
    param([string]$Message)
    [void]$script:Warnings.Add($Message)
    Write-Warn $Message
}

function Get-FullPath {
    param([string]$Path)
    if ([string]::IsNullOrWhiteSpace($Path)) { return "" }
    return [System.IO.Path]::GetFullPath($Path)
}

function Test-SamePath {
    param([string]$A, [string]$B)
    if ([string]::IsNullOrWhiteSpace($A) -or [string]::IsNullOrWhiteSpace($B)) { return $false }
    $a = (Get-FullPath $A).TrimEnd([char]92).TrimEnd([char]47)
    $b = (Get-FullPath $B).TrimEnd([char]92).TrimEnd([char]47)
    return $a.Equals($b, [System.StringComparison]::OrdinalIgnoreCase)
}

function Test-PathUnder {
    param([string]$Path, [string]$Root)
    if ([string]::IsNullOrWhiteSpace($Path) -or [string]::IsNullOrWhiteSpace($Root)) { return $false }
    $p = (Get-FullPath $Path).TrimEnd([char]92).TrimEnd([char]47)
    $r = (Get-FullPath $Root).TrimEnd([char]92).TrimEnd([char]47)
    if ($p.Equals($r, [System.StringComparison]::OrdinalIgnoreCase)) { return $true }
    $prefix = $r + [string][System.IO.Path]::DirectorySeparatorChar
    return $p.StartsWith($prefix, [System.StringComparison]::OrdinalIgnoreCase)
}

function Get-Sha256 {
    param([string]$Path)
    if ([string]::IsNullOrWhiteSpace($Path) -or -not (Test-Path -LiteralPath $Path -PathType Leaf)) { return "" }
    return (Get-FileHash -Algorithm SHA256 -LiteralPath $Path).Hash
}

function Write-TextFile {
    param([string]$Path, [string]$Text)
    $dir = Split-Path -Parent $Path
    if (-not [string]::IsNullOrWhiteSpace($dir) -and -not (Test-Path -LiteralPath $dir -PathType Container)) {
        New-Item -ItemType Directory -Path $dir -Force | Out-Null
    }
    $encoding = New-Object System.Text.UTF8Encoding -ArgumentList $false
    [System.IO.File]::WriteAllText($Path, $Text, $encoding)
}

function Write-JsonFile {
    param([string]$Path, $Object)
    $json = ConvertTo-Json -InputObject $Object -Depth 40
    Write-TextFile -Path $Path -Text $json
}

function Read-JsonFile {
    param([string]$Path)
    return (Get-Content -LiteralPath $Path -Raw -Encoding UTF8 | ConvertFrom-Json)
}

function Copy-FileWithRetry {
    param([string]$Source, [string]$Destination)
    $last = $null
    for ($i = 0; $i -lt 6; $i++) {
        try {
            Copy-Item -LiteralPath $Source -Destination $Destination -Force
            return
        } catch {
            $last = $_
            Start-Sleep -Milliseconds 250
        }
    }
    throw $last
}

function Get-Prop {
    param($Object, [string]$Name)
    if ($null -eq $Object) { return $null }
    $prop = $Object.PSObject.Properties[$Name]
    if ($null -eq $prop) { return $null }
    return $prop.Value
}

function Test-HasProp {
    param($Object, [string]$Name)
    if ($null -eq $Object) { return $false }
    return ($null -ne $Object.PSObject.Properties[$Name])
}

function Get-Str {
    param($Object, [string]$Name)
    $value = Get-Prop $Object $Name
    if ($null -eq $value) { return "" }
    return [string]$value
}

function Get-Bool {
    param($Object, [string]$Name, [bool]$Default)
    $value = Get-Prop $Object $Name
    if ($null -eq $value) { return $Default }
    if ($value -is [bool]) { return $value }
    $text = ([string]$value).Trim().ToLowerInvariant()
    if ($text -eq "true" -or $text -eq "1") { return $true }
    if ($text -eq "false" -or $text -eq "0") { return $false }
    return $Default
}

function Get-Array {
    param($Object, [string]$Name)
    $value = Get-Prop $Object $Name
    if ($null -eq $value) { return @() }
    return @($value)
}

function Invoke-AgentHttp {
    param(
        [string]$Name,
        [string]$Method,
        [string]$Path,
        $Body,
        [bool]$SaveArtifacts = $true
    )
    $uri = $AgentBaseUrl.TrimEnd([char]47) + $Path
    $json = $null
    if ($null -ne $Body) {
        $json = ConvertTo-Json -InputObject $Body -Depth 40 -Compress
        if ($SaveArtifacts) {
            Write-TextFile -Path (Join-Path $script:RequestsDir ($Name + ".json")) -Text $json
        }
    }
    $request = [System.Net.HttpWebRequest]::Create($uri)
    $request.Method = $Method
    $request.Timeout = $TimeoutSeconds * 1000
    $request.ReadWriteTimeout = $TimeoutSeconds * 1000
    $request.KeepAlive = $false
    $request.ServicePoint.Expect100Continue = $false
    if ($null -ne $json) {
        $bytes = [System.Text.Encoding]::UTF8.GetBytes($json)
        $request.ContentType = "application/json; charset=utf-8"
        $request.ContentLength = $bytes.Length
        $stream = $request.GetRequestStream()
        $stream.Write($bytes, 0, $bytes.Length)
        $stream.Close()
    }
    $status = 0
    $text = ""
    try {
        $response = $request.GetResponse()
        $status = [int]$response.StatusCode
        $reader = [System.IO.StreamReader]::new($response.GetResponseStream(), [System.Text.Encoding]::UTF8)
        $text = $reader.ReadToEnd()
        $reader.Close()
        $response.Close()
    } catch [System.Net.WebException] {
        $web = $_.Exception
        if ($null -ne $web.Response) {
            $response = $web.Response
            $status = [int]$response.StatusCode
            $reader = [System.IO.StreamReader]::new($response.GetResponseStream(), [System.Text.Encoding]::UTF8)
            $text = $reader.ReadToEnd()
            $reader.Close()
            $response.Close()
        } else {
            Throw-Classified "environment_failure" "agent_unreachable" ("agent HTTP " + $Method + " " + $uri + " failed: " + $web.Message)
        }
    } catch {
        Throw-Classified "environment_failure" "agent_request_failed" ("agent HTTP " + $Method + " " + $uri + " failed: " + $_.Exception.Message)
    }
    if ($SaveArtifacts) {
        Write-TextFile -Path (Join-Path $script:ResponsesDir ($Name + ".json")) -Text $text
    }
    $parsed = $null
    if (-not [string]::IsNullOrWhiteSpace($text)) {
        try {
            $parsed = $text | ConvertFrom-Json
        } catch {
            $snippet = $text
            if ($snippet.Length -gt 300) { $snippet = $snippet.Substring(0, 300) }
            Throw-Classified "environment_failure" "agent_response_not_json" ("non-JSON response from " + $uri + ": " + $snippet)
        }
    }
    return [pscustomobject]@{
        HttpStatus = $status
        BodyText = $text
        Json = $parsed
        Uri = $uri
        Name = $Name
    }
}

function Invoke-Tool {
    param(
        [string]$Name,
        [string]$Tool,
        $ToolArgs,
        [bool]$Confirmed = $true,
        [bool]$ExpectOk = $true
    )
    $body = [ordered]@{
        tool = $Tool
        args = $ToolArgs
        confirmed = $Confirmed
        source = "conv_cross_combo_smoke"
    }
    $resp = Invoke-AgentHttp -Name $Name -Method "Post" -Path "/agent/invoke" -Body $body
    if ($ExpectOk) {
        if ($resp.HttpStatus -ne 200) {
            $detail = ""
            if ($null -ne $resp.Json) {
                $detail = Get-Str $resp.Json "error"
                if ([string]::IsNullOrWhiteSpace($detail)) {
                    $detail = Get-Str (Get-Prop $resp.Json "result") "error"
                }
            }
            if ([string]::IsNullOrWhiteSpace($detail)) { $detail = $resp.BodyText }
            Throw-Classified "product_failure" "invoke_http_error" ($Tool + " HTTP " + $resp.HttpStatus + ": " + $detail)
        }
        if ($null -eq $resp.Json -or [string]$resp.Json.status -ne "ok") {
            $detail = ""
            if ($null -ne $resp.Json) { $detail = Get-Str $resp.Json "error" }
            if ([string]::IsNullOrWhiteSpace($detail)) { $detail = $resp.BodyText }
            Throw-Classified "product_failure" "invoke_status_error" ($Tool + " status != ok: " + $detail)
        }
    }
    return $resp
}

function Get-AgentState {
    param([string]$Name, [bool]$SaveArtifacts = $true)
    $resp = Invoke-AgentHttp -Name $Name -Method "Get" -Path "/agent/state" -Body $null -SaveArtifacts $SaveArtifacts
    if ($resp.HttpStatus -ne 200 -or $null -eq $resp.Json) {
        Throw-Classified "environment_failure" "agent_state_unreachable" ("/agent/state HTTP " + $resp.HttpStatus)
    }
    return $resp.Json
}

function Get-UIState {
    param([string]$Name, [bool]$SaveArtifacts = $true)
    $resp = Invoke-AgentHttp -Name $Name -Method "Get" -Path "/agent/ui/state" -Body $null -SaveArtifacts $SaveArtifacts
    if ($resp.HttpStatus -ne 200 -or $null -eq $resp.Json) {
        Throw-Classified "environment_failure" "agent_ui_state_unreachable" ("/agent/ui/state HTTP " + $resp.HttpStatus)
    }
    return $resp.Json
}

function Get-ActiveProjectPath {
    param($State)
    if ($null -eq $State) { return "" }
    $current = Get-Prop $State "current_project"
    $path = Get-Str $current "path"
    if (-not [string]::IsNullOrWhiteSpace($path)) { return $path }
    $history = Get-Prop $State "project_history"
    $path = Get-Str $history "project_path"
    if (-not [string]::IsNullOrWhiteSpace($path)) { return $path }
    $shadow = Get-Prop $State "shadow"
    return (Get-Str $shadow "project_path")
}

function Get-ActiveProjectUUID {
    param($State)
    if ($null -eq $State) { return "" }
    $current = Get-Prop $State "current_project"
    $uuid = Get-Str $current "uuid"
    if (-not [string]::IsNullOrWhiteSpace($uuid)) { return $uuid }
    $history = Get-Prop $State "project_history"
    $uuid = Get-Str $history "project_uuid"
    if (-not [string]::IsNullOrWhiteSpace($uuid)) { return $uuid }
    $shadow = Get-Prop $State "shadow"
    return (Get-Str $shadow "project_uuid")
}

function Get-HistoryStatus {
    param([string]$Name, [string]$ProjectPath)
    $resp = Invoke-Tool -Name $Name -Tool "version.status" -ToolArgs ([ordered]@{ project_path = $ProjectPath }) -Confirmed $false
    $result = Get-Prop $resp.Json "result"
    Assert-That ($Name + "-result") ($null -ne $result) ($Name + " result is null") "version_status_result_missing"
    return $result
}

function Get-BranchHead {
    param($History, [string]$Branch)
    $branches = Get-Prop $History "branches"
    return [string](Get-Prop $branches $Branch)
}

function Find-GraphNode {
    param($History, [string]$NodeId, [string]$Branch, [string]$CommitId)
    $nodes = @()
    $graph = Get-Prop $History "conversation_graph"
    if ($null -ne $graph) { $nodes = Get-Array $graph "nodes" }
    foreach ($node in $nodes) {
        if (-not [string]::IsNullOrWhiteSpace($NodeId) -and (Get-Str $node "id") -ne $NodeId) { continue }
        if (-not [string]::IsNullOrWhiteSpace($Branch) -and (Get-Str $node "branch") -ne $Branch) { continue }
        if (-not [string]::IsNullOrWhiteSpace($CommitId) -and (Get-Str $node "commit_id") -ne $CommitId) { continue }
        return $node
    }
    return $null
}

function Find-WorktreeRow {
    param($History, [string]$ProjectFilePath)
    foreach ($row in (Get-Array $History "worktrees")) {
        if (Test-SamePath (Get-Str $row "project_file_path") $ProjectFilePath) { return $row }
    }
    return $null
}

function Wait-ProjectPath {
    param([string]$Expected, [int]$Seconds)
    $deadline = (Get-Date).AddSeconds($Seconds)
    $last = ""
    while ((Get-Date) -lt $deadline) {
        try {
            $state = Get-AgentState -Name "poll_state" -SaveArtifacts $false
            $last = Get-ActiveProjectPath $state
            if (Test-SamePath $last $Expected) { return $state }
        } catch {
            $last = $_.Exception.Message
        }
        Start-Sleep -Milliseconds 500
    }
    return $null
}

function Wait-UIProjectPath {
    param([string]$Expected, [int]$Seconds)
    $deadline = (Get-Date).AddSeconds($Seconds)
    $last = ""
    while ((Get-Date) -lt $deadline) {
        try {
            $ui = Get-UIState -Name "poll_ui_state" -SaveArtifacts $false
            $project = Get-Prop $ui "project"
            $last = Get-Str $project "project_path"
            if (Test-SamePath $last $Expected) { return $ui }
        } catch {
            $last = $_.Exception.Message
        }
        Start-Sleep -Milliseconds 500
    }
    return $null
}

function Get-KernelLogPath {
    if (-not [string]::IsNullOrWhiteSpace($KernelLogPath)) {
        $path = Get-FullPath $KernelLogPath
        if (-not (Test-Path -LiteralPath $path -PathType Leaf)) {
            Throw-Classified "environment_failure" "kernel_log_unavailable" ("-KernelLogPath does not exist: " + $path)
        }
        return $path
    }
    $dir = Join-Path (Join-Path $RepoRoot "VitApp/Workspace") "Logs"
    $files = @(Get-ChildItem -LiteralPath $dir -Filter "VitHeadlessServer*.log" -File -ErrorAction SilentlyContinue | Sort-Object LastWriteTime -Descending)
    if ($files.Count -eq 0) {
        Throw-Classified "environment_failure" "kernel_log_unavailable" ("no VitHeadlessServer*.log under " + $dir + "; pass -KernelLogPath (single open_project reload count cannot be verified without it)")
    }
    return $files[0].FullName
}

function Get-LogLength {
    param([string]$Path)
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) { return -1 }
    return [long](Get-Item -LiteralPath $Path).Length
}

function Read-LogAppend {
    param([string]$Path, [long]$Offset)
    $stream = [System.IO.File]::Open($Path, [System.IO.FileMode]::Open, [System.IO.FileAccess]::Read, [System.IO.FileShare]::ReadWrite)
    try {
        if ($stream.Length -lt $Offset) {
            Throw-Classified "environment_failure" "kernel_log_rotated" ("kernel log shrank below snapshot offset: " + $Path)
        }
        [void]$stream.Seek($Offset, [System.IO.SeekOrigin]::Begin)
        $reader = [System.IO.StreamReader]::new($stream, [System.Text.Encoding]::UTF8)
        $text = $reader.ReadToEnd()
        return [pscustomobject]@{ Text = $text; Length = [long]$stream.Length }
    } finally {
        $stream.Close()
    }
}

function Count-OpenProject {
    param([string]$Text)
    return ([regex]::Matches($Text, '(?m)^Command received:[^\r\n]*"cmd"\s*:\s*"open_project"')).Count
}

function Count-CommandReceived {
    param([string]$Text)
    return ([regex]::Matches($Text, '(?m)^Command received:')).Count
}

function Test-TextHasPath {
    param([string]$Text, [string]$Path)
    if ([string]::IsNullOrWhiteSpace($Path)) { return $false }
    $escaped = $Path.Replace([string][char]92, [string][char]92 + [string][char]92)
    if ($Text.Contains($escaped)) { return $true }
    $leaf = Split-Path -Leaf $Path
    return $Text.Contains($leaf)
}

function Wait-OpenProjectCount {
    param([string]$Path, [long]$Offset, [int]$Expected, [int]$Seconds)
    $deadline = (Get-Date).AddSeconds($Seconds)
    $last = $null
    while ((Get-Date) -lt $deadline) {
        $last = Read-LogAppend -Path $Path -Offset $Offset
        $count = Count-OpenProject $last.Text
        if ($count -ge $Expected) {
            Start-Sleep -Milliseconds 1500
            $settled = Read-LogAppend -Path $Path -Offset $Offset
            return [pscustomobject]@{ Text = $settled.Text; Count = (Count-OpenProject $settled.Text); Length = $settled.Length }
        }
        Start-Sleep -Milliseconds 250
    }
    if ($null -eq $last) { $last = Read-LogAppend -Path $Path -Offset $Offset }
    return [pscustomobject]@{ Text = $last.Text; Count = (Count-OpenProject $last.Text); Length = $last.Length }
}

function Assert-NoOpenProjectSettled {
    param([string]$Id, [string]$Path, [long]$Offset, [string]$Context)
    Start-Sleep -Milliseconds 800
    $read = Read-LogAppend -Path $Path -Offset $Offset
    $count = Count-OpenProject $read.Text
    $commands = Count-CommandReceived $read.Text
    Assert-That $Id ($count -eq 0) ($Context + ": expected 0 open_project, got " + $count + " (command_received=" + $commands + ")") "unexpected_open_project"
    return $read
}

function Get-StaticReloadAnchor {
    $conv = Join-Path $RepoRoot "agent/internal/harness/conversation_open.go"
    $harness = Join-Path $RepoRoot "agent/internal/harness/harness.go"
    $test = Join-Path $RepoRoot "agent/internal/harness/conversation_open_test.go"
    $out = [ordered]@{
        available = $false
        conversation_open_with_reload = 0
        with_reload_open_project = 0
        reload_failure_test = $false
        note = ""
    }
    if (-not (Test-Path -LiteralPath $conv -PathType Leaf) -or -not (Test-Path -LiteralPath $harness -PathType Leaf)) {
        $out.note = "source files not found under RepoRoot"
        return [pscustomobject]$out
    }
    $out.available = $true
    $convText = Get-Content -LiteralPath $conv -Raw -Encoding UTF8
    $out.conversation_open_with_reload = ([regex]::Matches($convText, 'withProjectReload\(ctx,\s*"version_conversation_open"')).Count
    $harnessText = Get-Content -LiteralPath $harness -Raw -Encoding UTF8
    $match = [regex]::Match($harnessText, '(?s)func \(h \*Harness\) withProjectReload\(.*?\r?\n\}')
    if ($match.Success) {
        $out.with_reload_open_project = ([regex]::Matches($match.Value, '"cmd"\s*:\s*"open_project"')).Count
    }
    if (Test-Path -LiteralPath $test -PathType Leaf) {
        $out.reload_failure_test = [bool](Select-String -LiteralPath $test -Pattern "TestConversationOpenReloadFailureIsExplicit" -SimpleMatch -Quiet)
    }
    return [pscustomobject]$out
}

function Get-RecentsPath {
    if (-not [string]::IsNullOrWhiteSpace($env:VIT_DAW_USER_CONFIG_DIR)) {
        return (Join-Path $env:VIT_DAW_USER_CONFIG_DIR "global_project_config.json")
    }
    if ([string]::IsNullOrWhiteSpace($env:APPDATA)) { return "" }
    return (Join-Path (Join-Path $env:APPDATA "Vit-DAW") "global_project_config.json")
}

function Initialize-RecentsSnapshot {
    $script:RecentsPath = Get-RecentsPath
    if ([string]::IsNullOrWhiteSpace($script:RecentsPath)) {
        Throw-Classified "environment_failure" "recents_path_unavailable" "APPDATA and VIT_DAW_USER_CONFIG_DIR are both empty; cannot snapshot recents"
    }
    $script:RecentsExisted = Test-Path -LiteralPath $script:RecentsPath -PathType Leaf
    $script:RecentsBeforeHash = Get-Sha256 $script:RecentsPath
    if ($script:RecentsExisted) {
        $script:RecentsSnapshot = Join-Path $script:RunRoot "recents.before.json"
        Copy-FileWithRetry -Source $script:RecentsPath -Destination $script:RecentsSnapshot
        $copyHash = Get-Sha256 $script:RecentsSnapshot
        if ($copyHash -ne $script:RecentsBeforeHash) {
            Throw-Classified "environment_failure" "recents_snapshot_hash_mismatch" "recents snapshot copy hash differs from source"
        }
    } else {
        $script:RecentsSnapshot = ""
        Write-JsonFile -Path (Join-Path $script:RunRoot "recents.before.absent.json") -Object ([ordered]@{ path = $script:RecentsPath; existed = $false })
    }
    $script:Summary.recents = [ordered]@{
        path = $script:RecentsPath
        existed_before = $script:RecentsExisted
        hash_before = $script:RecentsBeforeHash
        hash_after_run = ""
        hash_after_restore = ""
        restored = $false
        error = ""
    }
}

function Restore-RecentsSnapshot {
    $result = [ordered]@{
        path = $script:RecentsPath
        existed_before = $script:RecentsExisted
        hash_before = $script:RecentsBeforeHash
        hash_after_run = (Get-Sha256 $script:RecentsPath)
        hash_after_restore = ""
        restored = $false
        error = ""
    }
    try {
        if ([string]::IsNullOrWhiteSpace($script:RecentsPath)) {
            throw "recents path is unavailable; snapshot was never initialized"
        }
        if ($script:RecentsExisted -and -not (Test-Path -LiteralPath $script:RecentsSnapshot -PathType Leaf)) {
            throw "recents snapshot file is missing"
        }
        if ($script:RecentsExisted) {
            $parent = Split-Path -Parent $script:RecentsPath
            if (-not [string]::IsNullOrWhiteSpace($parent) -and -not (Test-Path -LiteralPath $parent -PathType Container)) {
                New-Item -ItemType Directory -Path $parent -Force | Out-Null
            }
            Copy-FileWithRetry -Source $script:RecentsSnapshot -Destination $script:RecentsPath
            $afterCopy = Join-Path $script:RunRoot "recents.after_restore.json"
            Copy-FileWithRetry -Source $script:RecentsPath -Destination $afterCopy
        } else {
            if (Test-Path -LiteralPath $script:RecentsPath -PathType Leaf) {
                Remove-Item -LiteralPath $script:RecentsPath -Force
            }
        }
        $result.hash_after_restore = Get-Sha256 $script:RecentsPath
        $result.restored = ($result.hash_after_restore -eq $script:RecentsBeforeHash)
        if (-not $result.restored) {
            $result.error = "recents hash mismatch after restore"
        }
    } catch {
        $result.error = $_.Exception.Message
    }
    $script:Summary.recents = $result
    return [pscustomobject]$result
}

function Get-AuthoritySpecs {
    return @(
        [pscustomobject]@{ name = "settings"; path = (Join-Path $RepoRoot "VitApp/Workspace/Settings/Settings.xml") },
        [pscustomobject]@{ name = "default_project"; path = (Join-Path $RepoRoot "VitApp/Workspace/default_project.xml") }
    )
}

function Initialize-AuthoritySnapshots {
    $rows = New-Object System.Collections.ArrayList
    foreach ($spec in (Get-AuthoritySpecs)) {
        $row = [ordered]@{
            name = $spec.name
            path = (Get-FullPath $spec.path)
            existed = $false
            hash_before = ""
            snapshot = ""
            hash_after = ""
            restored = $false
            error = ""
        }
        if (Test-Path -LiteralPath $row.path -PathType Leaf) {
            $row.existed = $true
            $row.hash_before = Get-Sha256 $row.path
            $snapshot = Join-Path (Join-Path $script:RunRoot "authority") ($spec.name + ".before")
            Copy-FileWithRetry -Source $row.path -Destination $snapshot
            if ((Get-Sha256 $snapshot) -ne $row.hash_before) {
                Throw-Classified "environment_failure" "authority_snapshot_hash_mismatch" ("authority snapshot hash mismatch: " + $row.path)
            }
            $row.snapshot = $snapshot
        }
        [void]$rows.Add([pscustomobject]$row)
    }
    $script:Summary.authority = [ordered]@{ files = @($rows); restored = $false; error = "" }
}

function Restore-AuthoritySnapshots {
    $rows = @()
    if ($null -ne $script:Summary -and $null -ne $script:Summary.authority) {
        $rows = @($script:Summary.authority.files)
    }
    if ($rows.Count -eq 0) {
        return [pscustomobject]@{ ok = $false; files = @(); error = "authority snapshot was never initialized" }
    }
    $ok = $true
    $errors = New-Object System.Collections.ArrayList
    foreach ($row in $rows) {
        try {
            $current = Get-Sha256 $row.path
            if ($row.existed) {
                if ($current -ne $row.hash_before) {
                    Copy-FileWithRetry -Source $row.snapshot -Destination $row.path
                }
            } else {
                if (Test-Path -LiteralPath $row.path -PathType Leaf) {
                    Remove-Item -LiteralPath $row.path -Force
                }
            }
            $row.hash_after = Get-Sha256 $row.path
            $row.restored = ($row.hash_after -eq $row.hash_before)
            if (-not $row.restored) {
                $ok = $false
                $row.error = "authority hash mismatch after restore"
                [void]$errors.Add($row.path + ": " + $row.error)
            }
        } catch {
            $ok = $false
            $row.restored = $false
            $row.error = $_.Exception.Message
            [void]$errors.Add($row.path + ": " + $_.Exception.Message)
        }
    }
    if ($null -ne $script:Summary -and $null -ne $script:Summary.authority) {
        $script:Summary.authority.files = @($rows)
        $script:Summary.authority.restored = $ok
        $script:Summary.authority.error = ($errors -join "; ")
    }
    return [pscustomobject]@{ ok = $ok; files = @($rows); error = ($errors -join "; ") }
}

function New-RunRoot {
    $full = ""
    if ([string]::IsNullOrWhiteSpace($RunRoot)) {
        $stamp = Get-Date -Format "yyyyMMdd_HHmmss"
        $suffix = [guid]::NewGuid().ToString("N").Substring(0, 8)
        $full = Join-Path $RepoRoot ("coord/runs/CONV-CROSS-COMBO-1/run_" + $stamp + "_" + $suffix)
    } else {
        $full = Get-FullPath $RunRoot
    }
    if (Test-Path -LiteralPath $full) {
        if (-not (Test-Path -LiteralPath $full -PathType Container)) {
            Throw-Classified "usage" "runroot_is_file" ("RunRoot is a file: " + $full)
        }
        $children = @(Get-ChildItem -LiteralPath $full -Force -ErrorAction SilentlyContinue)
        if ($children.Count -gt 0) {
            Throw-Classified "usage" "runroot_not_empty" ("RunRoot must be fresh/empty; found " + $children.Count + " entries: " + $full)
        }
    } else {
        New-Item -ItemType Directory -Path $full -Force | Out-Null
    }
    $script:RunRoot = (Get-FullPath $full)
    $script:RequestsDir = Join-Path $script:RunRoot "requests"
    $script:ResponsesDir = Join-Path $script:RunRoot "responses"
    foreach ($dir in @($script:RequestsDir, $script:ResponsesDir, (Join-Path $script:RunRoot "authority"), (Join-Path $script:RunRoot "workspace"), (Join-Path $script:RunRoot "logs"))) {
        if (-not (Test-Path -LiteralPath $dir -PathType Container)) {
            New-Item -ItemType Directory -Path $dir -Force | Out-Null
        }
    }
}

function Get-ProcessEvidence {
    param([string]$Name)
    $proc = $null
    try {
        $proc = Get-Process -Name $Name -ErrorAction SilentlyContinue | Select-Object -First 1
    } catch {
        $proc = $null
    }
    if ($null -eq $proc) {
        return [ordered]@{ name = $Name; present = $false; pid = 0; path = ""; sha256 = "" }
    }
    $path = ""
    try { $path = [string]$proc.Path } catch { $path = "" }
    $hash = ""
    if (-not [string]::IsNullOrWhiteSpace($path)) { $hash = Get-Sha256 $path }
    return [ordered]@{ name = $Name; present = $true; pid = [int]$proc.Id; path = $path; sha256 = $hash }
}

function Initialize-Summary {
    $gitHead = ""
    $gitStatus = ""
    try {
        $gitHead = [string](& git -C $RepoRoot rev-parse HEAD 2>$null)
        $gitStatus = [string](& git -C $RepoRoot status --short 2>$null)
    } catch {
        $gitHead = ""
        $gitStatus = ""
    }
    $script:Summary = [ordered]@{
        schema_version = $script:Schema
        card = $script:Card
        status = "running"
        runtime_owner = $RuntimeOwner
        created_at = (Get-Date).ToString("o")
        completed_at = ""
        run_root = $script:RunRoot
        repo_root = (Get-FullPath $RepoRoot)
        agent_base_url = $AgentBaseUrl
        timeout_seconds = $TimeoutSeconds
        script_path = $PSCommandPath
        script_sha256 = (Get-Sha256 $PSCommandPath)
        environment = [ordered]@{
            ps_version = $PSVersionTable.PSVersion.ToString()
            os = [System.Environment]::OSVersion.VersionString
            computer = $env:COMPUTERNAME
            user = $env:USERNAME
            git_head = $gitHead.Trim()
            git_status = $gitStatus.Trim()
            godot_process_count = @(Get-Process -Name Godot, Godot_console -ErrorAction SilentlyContinue).Count
            processes = [ordered]@{
                agent = Get-ProcessEvidence "VitAgent"
                kernel = Get-ProcessEvidence "VitApp"
                vsp_hub = Get-ProcessEvidence "VspHub"
            }
        }
        isolation = [ordered]@{
            run_root = $script:RunRoot
            project_a = ""
            project_a_uuid = ""
            worktree_b = ""
            worktree_b_name = $script:WorktreeName
            branch_name = $script:BranchName
            seed_node_id = ""
            seed_commit = ""
            branch_node_id = ""
            draft_path = ""
            draft_under_runroot = $false
            stack_idle_at_start = $false
            initial_kernel_project = ""
            initial_kernel_uuid = ""
        }
        kernel_log = [ordered]@{
            path = ""
            offset = -1
            single_reload_count = -1
            verified = $false
        }
        static_anchor = $null
        steps = [ordered]@{
            s0 = $false
            s1 = $false
            s2 = $false
            s3 = $false
            s4 = $false
            s5 = $false
            s6 = $false
        }
        reload = [ordered]@{
            conversation_open_open_project = -1
            branch_create_open_project = -1
            project_open_a_open_project = -1
            worktree_create_open_project = -1
            target_path_seen = $false
        }
        recents = $null
        authority = $null
        assertions = @()
        warnings = @()
        coverage = [ordered]@{
            real_stack_verified = @()
            unit_test_only = @("reload failure after target materialization is covered only by the 419 fake-kernel unit test")
            not_exercised = @("target_materialized=unknown", "reload failure after materialization on a real kernel", "Godot/VitShell window pixels", "human /agent/chat conversation node")
            gaps = @("real-stack reload-failure injection is not available through the existing agent HTTP/tool surface")
        }
        failure = $null
        teardown = $null
    }
    $metaPath = Join-Path $script:RunRoot "run_meta.json"
    Write-JsonFile -Path $metaPath -Object $script:Summary
    Write-TextFile -Path (Join-Path $script:RunRoot "head.txt") -Text ($gitHead.Trim() + [Environment]::NewLine)
    Write-TextFile -Path (Join-Path $script:RunRoot "git_status.txt") -Text ($gitStatus.Trim() + [Environment]::NewLine)
}

function Resolve-ProjectA {
    $path = $ProjectA
    if ([string]::IsNullOrWhiteSpace($path)) {
        $path = Join-Path (Join-Path $script:RunRoot "workspace") "ProjectA.vit"
    } else {
        $path = Get-FullPath $path
    }
    if (-not (Test-PathUnder $path $script:RunRoot)) {
        Throw-Classified "usage" "project_a_outside_runroot" ("ProjectA must live under the fresh RunRoot (never a user project): " + $path)
    }
    if ([System.IO.Path]::GetExtension($path) -ine ".vit") {
        Throw-Classified "usage" "project_a_not_vit" ("ProjectA must end in .vit: " + $path)
    }
    if (Test-Path -LiteralPath $path) {
        Throw-Classified "usage" "project_a_exists" ("ProjectA already exists; use a fresh RunRoot: " + $path)
    }
    $parent = Split-Path -Parent $path
    if (-not (Test-Path -LiteralPath $parent -PathType Container)) {
        New-Item -ItemType Directory -Path $parent -Force | Out-Null
    }
    $script:ProjectA = $path
    $script:Summary.isolation.project_a = $path
}

function Assert-KernelProject {
    param([string]$Id, [string]$ExpectedPath, [string]$Context)
    $state = Get-AgentState -Name ($Id + "_state")
    $actual = Get-ActiveProjectPath $state
    Assert-That $Id (Test-SamePath $actual $ExpectedPath) ($Context + ": kernel project path=" + $actual + " expected=" + $ExpectedPath) "kernel_project_path_mismatch"
    return $state
}

function Assert-ConversationOpenFailure {
    param(
        [string]$Name,
        $ToolArgs,
        [string]$ExpectedText,
        [string]$CurrentExpectedPath
    )
    $offset = Get-LogLength $script:KernelLogPath
    $resp = Invoke-Tool -Name $Name -Tool "version.conversation_open" -ToolArgs $ToolArgs -Confirmed $true -ExpectOk $false
    Assert-That ($Name + "-http") (($resp.HttpStatus -eq 400) -or ($resp.HttpStatus -eq 409)) ($Name + ": HTTP " + $resp.HttpStatus + " body=" + $resp.BodyText) "conversation_open_failure_http"
    Assert-That ($Name + "-status") ($null -ne $resp.Json -and [string]$resp.Json.status -eq "error") ($Name + ": expected InvokeResponse.status=error") "conversation_open_failure_status"
    $result = Get-Prop $resp.Json "result"
    Assert-That ($Name + "-result") ($null -ne $result) ($Name + ": result map missing") "conversation_open_failure_result"
    Assert-That ($Name + "-target-disclosed") (Test-HasProp $result "target_materialized") ($Name + ": target_materialized field not disclosed") "target_materialized_not_disclosed"
    Assert-That ($Name + "-target-not-materialized") ((Get-Bool $result "target_materialized" $true) -eq $false) ($Name + ": target_materialized must be false before materialization") "target_materialized_unexpected"
    Assert-That ($Name + "-kernel-not-reloaded") ((Get-Bool $result "kernel_reloaded" $true) -eq $false) ($Name + ": kernel_reloaded must be false") "kernel_reloaded_unexpected"
    $errorText = Get-Str $resp.Json "error"
    if ([string]::IsNullOrWhiteSpace($errorText)) { $errorText = Get-Str $result "error" }
    $errorOk = ($errorText.IndexOf($ExpectedText, [System.StringComparison]::OrdinalIgnoreCase) -ge 0)
    Assert-That ($Name + "-error-text") $errorOk ($Name + ": error=" + $errorText + " expected to contain=" + $ExpectedText) "conversation_open_failure_text"
    $read = Assert-NoOpenProjectSettled -Id ($Name + "-no-open-project") -Path $script:KernelLogPath -Offset $offset -Context $Name
    Assert-That ($Name + "-kernel-current") (Test-SamePath (Get-ActiveProjectPath (Get-AgentState -Name ($Name + "_state"))) $CurrentExpectedPath) ($Name + ": current kernel project changed") "conversation_open_failure_kernel_changed"
    return $resp
}

# ============================================================================
# Entry gates: dry run -> unauthorized -> authorized real leg
# ============================================================================
$script:StartedProcesses = 0
$script:StartedPorts = 0

if ($DryRun) {
    Show-DryRunPlan
}

if (-not $StackAuthorized) {
    Show-RunbookSummary
    exit 3
}

$exitCode = 1
try {
    $RepoRoot = Get-FullPath $RepoRoot
    if (-not (Test-Path -LiteralPath $RepoRoot -PathType Container)) {
        Throw-Classified "usage" "repo_root_missing" ("RepoRoot does not exist: " + $RepoRoot)
    }
    if ([string]::IsNullOrWhiteSpace($RuntimeOwner)) {
        Throw-Classified "usage" "runtime_owner_missing" "authorized runs must record -RuntimeOwner (stack berth owner)"
    }
    if ($TimeoutSeconds -lt 10 -or $TimeoutSeconds -gt 1800) {
        Throw-Classified "usage" "timeout_out_of_range" ("TimeoutSeconds must be 10..1800; got " + $TimeoutSeconds)
    }
    if ($AgentBaseUrl -notmatch '^https?://') {
        Throw-Classified "usage" "agent_base_url_invalid" ("AgentBaseUrl must be an http(s) URL: " + $AgentBaseUrl)
    }

    New-RunRoot
    Initialize-Summary
    Resolve-ProjectA
    Initialize-RecentsSnapshot
    Initialize-AuthoritySnapshots

    # ------------------------------------------------------------------
    # S0 preflight
    # ------------------------------------------------------------------
    $script:CurrentStep = "S0"
    Write-Step "S0 preflight (agent HTTP, idle stack, recents/authority/kernel-log snapshots)"

    $health = Invoke-AgentHttp -Name "s0_health" -Method "Get" -Path "/health" -Body $null
    Assert-That "S0-agent-health" ($health.HttpStatus -eq 200 -and $null -ne $health.Json -and [string]$health.Json.status -eq "ok") ("/health HTTP " + $health.HttpStatus) "agent_health_failed"

    $runtime = Invoke-AgentHttp -Name "s0_runtime_status" -Method "Get" -Path "/agent/runtime/status" -Body $null
    Assert-That "S0-runtime-status" ($runtime.HttpStatus -eq 200 -and $null -ne $runtime.Json) ("/agent/runtime/status HTTP " + $runtime.HttpStatus) "agent_runtime_status_failed"
    $kernelStatus = Get-Prop $runtime.Json "kernel"
    Assert-That "S0-kernel-online" (Get-Bool $kernelStatus "connected" $false) ("kernel not connected: " + (Get-Str $kernelStatus "status") + " " + (Get-Str $kernelStatus "error")) "kernel_offline"
    Assert-That "S0-no-active-turn" (-not (Get-Bool $runtime.Json "checkout_blocked" $false)) "agent reports checkout_blocked=true; stop the active turn first" "checkout_blocked"

    $initialState = Get-AgentState -Name "s0_state"
    $initialProject = Get-ActiveProjectPath $initialState
    $initialUuid = Get-ActiveProjectUUID $initialState
    $script:Summary.isolation.initial_kernel_project = $initialProject
    $script:Summary.isolation.initial_kernel_uuid = $initialUuid
    $initialIsDraft = ($initialProject -match '(?i)[\\/]ProjectHistory[\\/]drafts[\\/]')
    $initialUnderRunRoot = $false
    if (-not [string]::IsNullOrWhiteSpace($initialProject)) {
        $initialUnderRunRoot = Test-PathUnder $initialProject $script:RunRoot
    }
    $stackIdle = ([string]::IsNullOrWhiteSpace($initialProject) -or $initialIsDraft -or $initialUnderRunRoot)
    Assert-That "S0-stack-idle" $stackIdle ("a real project is already open; this leg would overwrite workspace state: " + $initialProject) "stack_not_idle"
    $script:Summary.isolation.stack_idle_at_start = $stackIdle
    if ($initialIsDraft -and -not (Test-PathUnder $initialProject $script:RunRoot)) {
        Add-Warning ("initial kernel draft is outside RunRoot: " + $initialProject + " (set VIT_HISTORY_DRAFT_ROOT for a strict berth)")
    }

    $script:KernelLogPath = Get-KernelLogPath
    $script:KernelLogOffset = Get-LogLength $script:KernelLogPath
    Assert-That "S0-kernel-log" ($script:KernelLogOffset -ge 0) ("kernel log unavailable: " + $script:KernelLogPath) "kernel_log_unavailable"
    $script:Summary.kernel_log.path = $script:KernelLogPath
    $script:Summary.kernel_log.offset = $script:KernelLogOffset
    Write-Ok ("kernel log: " + $script:KernelLogPath + " offset=" + $script:KernelLogOffset)

    $anchor = Get-StaticReloadAnchor
    $script:Summary.static_anchor = $anchor
    if ($anchor.available) {
        $anchorOk = (($anchor.conversation_open_with_reload -eq 1) -and ($anchor.with_reload_open_project -eq 1))
        Assert-That "S0-static-reload-anchor" $anchorOk ("conversation_open.go withProjectReload=" + $anchor.conversation_open_with_reload + " withProjectReload open_project=" + $anchor.with_reload_open_project) "static_reload_anchor_mismatch"
        if (-not $anchor.reload_failure_test) {
            Add-Warning "419 reload-failure unit test anchor not found (TestConversationOpenReloadFailureIsExplicit)"
        }
    } else {
        Add-Warning ("static reload anchor unavailable: " + $anchor.note)
    }

    Assert-That "S0-recents-snapshot" (-not [string]::IsNullOrWhiteSpace($script:RecentsPath)) ("recents path=" + $script:RecentsPath) "recents_snapshot_missing"
    Assert-That "S0-authority-snapshot" (@($script:Summary.authority.files).Count -eq 2) "authority snapshot count != 2" "authority_snapshot_missing"
    $script:Summary.steps.s0 = $true

    # ------------------------------------------------------------------
    # S1 real project creation + save
    # ------------------------------------------------------------------
    $script:CurrentStep = "S1"
    Write-Step ("S1 create/save real project A: " + $script:ProjectA)
    $newResp = Invoke-Tool -Name "s1_project_new" -Tool "project.new" -ToolArgs ([ordered]@{}) -Confirmed $true
    $newResult = Get-Prop $newResp.Json "result"
    Assert-That "S1-project-new-ok" ($null -ne $newResult -and [string]$newResult.status -eq "ok") "project.new did not return status ok" "project_new_failed"
    Assert-That "S1-project-new-lifecycle" ((Get-Str $newResult "project_lifecycle") -eq "new") ("project_lifecycle=" + (Get-Str $newResult "project_lifecycle")) "project_new_lifecycle"
    $draftUuid = Get-Str $newResult "project_uuid"
    if ([string]::IsNullOrWhiteSpace($draftUuid)) { $draftUuid = Get-Str $newResult "project_id" }
    Assert-That "S1-project-new-uuid" (-not [string]::IsNullOrWhiteSpace($draftUuid)) "project.new returned no project uuid" "project_new_uuid"
    $newWorkspace = Get-Prop $newResult "project_workspace"
    $draftPath = ""
    if ($null -ne $newWorkspace) { $draftPath = Get-Str $newWorkspace "project_path" }
    $script:Summary.isolation.draft_path = $draftPath
    $script:Summary.isolation.draft_under_runroot = (Test-PathUnder $draftPath $script:RunRoot)
    if (-not [string]::IsNullOrWhiteSpace($draftPath) -and -not $script:Summary.isolation.draft_under_runroot) {
        Add-Warning ("draft materialized outside RunRoot: " + $draftPath)
    }

    $saveResp = Invoke-Tool -Name "s1_project_save_as" -Tool "project.save_as" -ToolArgs ([ordered]@{ file_path = $script:ProjectA }) -Confirmed $true
    $saveResult = Get-Prop $saveResp.Json "result"
    Assert-That "S1-project-save-ok" ($null -ne $saveResult -and [string]$saveResult.status -eq "ok") "project.save_as did not return status ok" "project_save_failed"
    $savedPath = Get-Str $saveResult "project_path"
    if ([string]::IsNullOrWhiteSpace($savedPath)) { $savedPath = Get-Str $saveResult "current_project_path" }
    Assert-That "S1-project-save-path" (Test-SamePath $savedPath $script:ProjectA) ("save_as project_path=" + $savedPath + " expected=" + $script:ProjectA) "project_save_path_mismatch"
    $uuidA = Get-Str $saveResult "project_uuid"
    if ([string]::IsNullOrWhiteSpace($uuidA)) { $uuidA = Get-Str $saveResult "project_id" }
    Assert-That "S1-project-save-uuid" (-not [string]::IsNullOrWhiteSpace($uuidA)) "save_as returned no project uuid" "project_save_uuid"
    Assert-That "S1-project-save-workspace" ($null -ne (Get-Prop $saveResult "project_workspace")) "save_as lifecycle did not return project_workspace" "project_save_workspace"
    Assert-That "S1-project-save-no-warning" ([string]::IsNullOrWhiteSpace((Get-Str $saveResult "project_workspace_warning"))) ("project_workspace_warning=" + (Get-Str $saveResult "project_workspace_warning")) "project_save_workspace_warning"
    Assert-That "S1-project-file-exists" (Test-Path -LiteralPath $script:ProjectA -PathType Leaf) ("project file missing: " + $script:ProjectA) "project_file_missing"
    $script:Summary.isolation.project_a_uuid = $uuidA

    $statusA = Get-HistoryStatus -Name "s1_status_a" -ProjectPath $script:ProjectA
    Assert-That "S1-history-path" (Test-SamePath (Get-Str $statusA "project_path") $script:ProjectA) ("version.status project_path=" + (Get-Str $statusA "project_path")) "history_path_mismatch"
    Assert-That "S1-history-initialized" ((Get-Bool $statusA "initialized" $false) -eq $true) "version.status A initialized=false" "history_not_initialized"

    $stateA = Assert-KernelProject -Id "S1-kernel-project-a" -ExpectedPath $script:ProjectA -Context "S1 save_as"
    $activeUuidA = Get-ActiveProjectUUID $stateA
    Assert-That "S1-kernel-uuid-a" (([string]::IsNullOrWhiteSpace($activeUuidA)) -or ($activeUuidA -eq $uuidA)) ("kernel uuid=" + $activeUuidA + " save_as uuid=" + $uuidA) "kernel_uuid_mismatch"
    $script:Summary.steps.s1 = $true
    Write-Ok ("A created: " + $script:ProjectA + " uuid=" + $uuidA)

    # ------------------------------------------------------------------
    # S2 second worktree + target branch/node, then back to A
    # ------------------------------------------------------------------
    $script:CurrentStep = "S2"
    Write-Step ("S2 create worktree B and target branch/node: " + $script:WorktreeName + "/" + $script:BranchName)

    $offset = Get-LogLength $script:KernelLogPath
    $wtResp = Invoke-Tool -Name "s2_worktree_create" -Tool "version.worktree_create" -ToolArgs ([ordered]@{ name = $script:WorktreeName }) -Confirmed $true
    $wtResult = Get-Prop $wtResp.Json "result"
    Assert-That "S2-worktree-create-ok" ($null -ne $wtResult -and [string]$wtResult.status -eq "ok") "version.worktree_create did not return status ok" "worktree_create_failed"
    Assert-That "S2-worktree-name" ((Get-Str $wtResult "name") -eq $script:WorktreeName) ("worktree name=" + (Get-Str $wtResult "name")) "worktree_name_mismatch"
    $worktreeB = Get-Str $wtResult "project_file_path"
    Assert-That "S2-worktree-path" (-not [string]::IsNullOrWhiteSpace($worktreeB)) "worktree_create returned no project_file_path" "worktree_path_missing"
    $script:WorktreeB = Get-FullPath $worktreeB
    Assert-That "S2-worktree-under-runroot" (Test-PathUnder $script:WorktreeB $script:RunRoot) ("worktree B outside RunRoot: " + $script:WorktreeB) "worktree_outside_runroot"
    Assert-That "S2-worktree-file" (Test-Path -LiteralPath $script:WorktreeB -PathType Leaf) ("worktree project file missing: " + $script:WorktreeB) "worktree_file_missing"
    Assert-That "S2-worktree-source-checkpoint" (-not [string]::IsNullOrWhiteSpace((Get-Str $wtResult "source_checkpoint_id"))) "worktree_create returned no source_checkpoint_id (source autosave failed)" "worktree_source_checkpoint_missing"
    Assert-That "S2-worktree-source-warning" ([string]::IsNullOrWhiteSpace((Get-Str $wtResult "source_checkpoint_warning"))) ("source_checkpoint_warning=" + (Get-Str $wtResult "source_checkpoint_warning")) "worktree_source_autosave_warning"
    $read = Assert-NoOpenProjectSettled -Id "S2-worktree-create-no-reload" -Path $script:KernelLogPath -Offset $offset -Context "version.worktree_create"
    $script:Summary.reload.worktree_create_open_project = Count-OpenProject $read.Text
    $script:Summary.isolation.worktree_b = $script:WorktreeB

    $bSeedStatus = Get-HistoryStatus -Name "s2_status_b_seed" -ProjectPath $script:WorktreeB
    Assert-That "S2-status-b-path" (Test-SamePath (Get-Str $bSeedStatus "project_path") $script:WorktreeB) ("version.status B project_path=" + (Get-Str $bSeedStatus "project_path")) "worktree_b_status_path"
    Assert-That "S2-status-b-initialized" ((Get-Bool $bSeedStatus "initialized" $false) -eq $true) "version.status B initialized=false" "worktree_b_history_unavailable"
    Assert-That "S2-status-b-worktree-count" (@(Get-Array $bSeedStatus "worktrees").Count -ge 2) ("worktree count=" + @(Get-Array $bSeedStatus "worktrees").Count) "worktree_count"
    Assert-That "S2-status-b-active-branch" ((Get-Str $bSeedStatus "active_branch") -eq "main") ("B active_branch=" + (Get-Str $bSeedStatus "active_branch")) "worktree_b_branch"
    $seedHead = Get-Str $bSeedStatus "head"
    Assert-That "S2-status-b-head" (-not [string]::IsNullOrWhiteSpace($seedHead)) "B head is empty" "worktree_b_head"
    Assert-That "S2-status-b-branch-head" ((Get-BranchHead $bSeedStatus "main") -eq $seedHead) ("branches.main=" + (Get-BranchHead $bSeedStatus "main") + " head=" + $seedHead) "worktree_b_branch_head"
    $seedNodeId = Get-Str $bSeedStatus "active_node_id"
    Assert-That "S2-status-b-active-node" (-not [string]::IsNullOrWhiteSpace($seedNodeId)) "B active_node_id is empty" "worktree_b_node"
    $seedNode = Find-GraphNode -History $bSeedStatus -NodeId $seedNodeId -Branch "" -CommitId ""
    Assert-That "S2-status-b-node-row" ($null -ne $seedNode) ("node not found: " + $seedNodeId) "worktree_b_node_missing"
    Assert-That "S2-status-b-node-commit" ((Get-Str $seedNode "commit_id") -eq $seedHead) ("node commit=" + (Get-Str $seedNode "commit_id") + " head=" + $seedHead) "worktree_b_node_commit"
    $script:SeedNodeId = $seedNodeId
    $script:SeedCommit = $seedHead
    $script:Summary.isolation.seed_node_id = $seedNodeId
    $script:Summary.isolation.seed_commit = $seedHead
    Write-Ok ("B seeded: " + $script:WorktreeB + " head=" + $seedHead + " node=" + $seedNodeId)

    $offset = Get-LogLength $script:KernelLogPath
    $branchResp = Invoke-Tool -Name "s2_branch_create" -Tool "version.branch_create" -ToolArgs ([ordered]@{
        project_path = $script:WorktreeB
        name = $script:BranchName
        commit_id = $script:SeedCommit
        from_node_id = $script:SeedNodeId
    }) -Confirmed $true
    $branchResult = Get-Prop $branchResp.Json "result"
    Assert-That "S2-branch-create-ok" ($null -ne $branchResult -and [string]$branchResult.status -eq "ok") "version.branch_create did not return status ok" "branch_create_failed"
    Assert-That "S2-branch-name" ((Get-Str $branchResult "branch") -eq $script:BranchName) ("branch=" + (Get-Str $branchResult "branch")) "branch_name_mismatch"
    Assert-That "S2-branch-commit" ((Get-Str $branchResult "commit_id") -eq $script:SeedCommit) ("branch commit=" + (Get-Str $branchResult "commit_id")) "branch_commit_mismatch"
    $branchRefresh = Get-Prop $branchResult "refresh"
    Assert-That "S2-branch-reload" (Get-Bool $branchRefresh "kernel_reloaded" $false) "branch_create did not report kernel_reloaded=true" "branch_create_reload"
    $read = Wait-OpenProjectCount -Path $script:KernelLogPath -Offset $offset -Expected 1 -Seconds 15
    $script:Summary.reload.branch_create_open_project = $read.Count
    Assert-That "S2-branch-open-project-count" ($read.Count -eq 1) ("branch_create open_project count=" + $read.Count + " (expected exactly 1 setup reload)") "branch_create_reload_count"
    Assert-That "S2-branch-open-project-target" (Test-TextHasPath $read.Text $script:WorktreeB) "branch_create open_project line does not contain B path" "branch_create_reload_target"
    $null = Assert-KernelProject -Id "S2-kernel-b-after-branch" -ExpectedPath $script:WorktreeB -Context "S2 branch_create"
    $branchUi = Wait-UIProjectPath -Expected $script:WorktreeB -Seconds 30
    Assert-That "S2-branch-ui-project-b" ($null -ne $branchUi) "ui project_path did not settle on B after branch_create" "branch_create_ui_settle"

    $bBranchStatus = Get-HistoryStatus -Name "s2_status_b_branch" -ProjectPath $script:WorktreeB
    Assert-That "S2-branch-active" ((Get-Str $bBranchStatus "active_branch") -eq $script:BranchName) ("B active_branch=" + (Get-Str $bBranchStatus "active_branch")) "branch_not_active"
    Assert-That "S2-branch-head" ((Get-BranchHead $bBranchStatus $script:BranchName) -eq $script:SeedCommit) ("branches." + $script:BranchName + "=" + (Get-BranchHead $bBranchStatus $script:BranchName)) "branch_head"
    Assert-That "S2-branch-head-field" ((Get-Str $bBranchStatus "head") -eq $script:SeedCommit) ("B head=" + (Get-Str $bBranchStatus "head")) "branch_head_field"
    $branchNodeId = Get-Str $bBranchStatus "active_node_id"
    Assert-That "S2-branch-node-active" (-not [string]::IsNullOrWhiteSpace($branchNodeId)) "B active_node_id is empty after branch_create" "branch_node_missing"
    $branchNode = Find-GraphNode -History $bBranchStatus -NodeId $branchNodeId -Branch "" -CommitId ""
    Assert-That "S2-branch-node-row" ($null -ne $branchNode) ("branch node not found: " + $branchNodeId) "branch_node_row"
    Assert-That "S2-branch-node-branch" ((Get-Str $branchNode "branch") -eq $script:BranchName) ("node branch=" + (Get-Str $branchNode "branch")) "branch_node_branch"
    Assert-That "S2-branch-node-commit" ((Get-Str $branchNode "commit_id") -eq $script:SeedCommit) ("node commit=" + (Get-Str $branchNode "commit_id")) "branch_node_commit"
    $script:BranchNodeId = $branchNodeId
    $script:Summary.isolation.branch_node_id = $branchNodeId
    Write-Ok ("target branch/node: " + $script:BranchName + "/" + $branchNodeId)

    $offset = Get-LogLength $script:KernelLogPath
    $openAResp = Invoke-Tool -Name "s2_project_open_a" -Tool "project.open" -ToolArgs ([ordered]@{ file_path = $script:ProjectA }) -Confirmed $true
    $openAResult = Get-Prop $openAResp.Json "result"
    Assert-That "S2-open-a-ok" ($null -ne $openAResult -and [string]$openAResult.status -eq "ok") "project.open A did not return status ok" "project_open_a_failed"
    $read = Wait-OpenProjectCount -Path $script:KernelLogPath -Offset $offset -Expected 1 -Seconds 20
    $script:Summary.reload.project_open_a_open_project = $read.Count
    Assert-That "S2-open-a-count" ($read.Count -eq 1) ("project.open A count=" + $read.Count) "project_open_a_count"
    Assert-That "S2-open-a-target" (Test-TextHasPath $read.Text $script:ProjectA) "project.open A line does not contain A path" "project_open_a_target"
    $null = Assert-KernelProject -Id "S2-kernel-back-a" -ExpectedPath $script:ProjectA -Context "S2 project.open A"
    $backToA = Wait-UIProjectPath -Expected $script:ProjectA -Seconds 30
    Assert-That "S2-back-to-a-ui" ($null -ne $backToA) "ui project_path did not settle back on A" "project_open_a_ui_settle"
    $script:Summary.steps.s2 = $true

    # ------------------------------------------------------------------
    # S3 read-only browse of B through version.status
    # ------------------------------------------------------------------
    $script:CurrentStep = "S3"
    Write-Step "S3 read-only browse B via version.status (must not switch kernel/DAW)"
    $offset = Get-LogLength $script:KernelLogPath
    $bHashBefore = Get-Sha256 $script:WorktreeB
    $aHashBefore = Get-Sha256 $script:ProjectA
    Assert-That "S3-a-file-present" (-not [string]::IsNullOrWhiteSpace($aHashBefore)) "A project file missing before browse" "project_file_missing"
    Assert-That "S3-b-file-present" (-not [string]::IsNullOrWhiteSpace($bHashBefore)) "B project file missing before browse" "worktree_file_missing"

    $s3StateBefore = Assert-KernelProject -Id "S3-kernel-before" -ExpectedPath $script:ProjectA -Context "S3 before status"
    $s3UiBefore = Get-UIState -Name "s3_ui_before"
    $s3UiProjectBefore = Get-Prop $s3UiBefore "project"
    Assert-That "S3-ui-before" (Test-SamePath (Get-Str $s3UiProjectBefore "project_path") $script:ProjectA) ("ui project_path=" + (Get-Str $s3UiProjectBefore "project_path")) "ui_project_before"

    $bBrowseStatus = Get-HistoryStatus -Name "s3_status_b" -ProjectPath $script:WorktreeB
    Assert-That "S3-status-b-path" (Test-SamePath (Get-Str $bBrowseStatus "project_path") $script:WorktreeB) ("status B project_path=" + (Get-Str $bBrowseStatus "project_path")) "browse_status_path"
    Assert-That "S3-status-b-initialized" ((Get-Bool $bBrowseStatus "initialized" $false) -eq $true) "status B initialized=false" "browse_status_available"
    Assert-That "S3-status-b-branch" ((Get-Str $bBrowseStatus "active_branch") -eq $script:BranchName) ("status B active_branch=" + (Get-Str $bBrowseStatus "active_branch")) "browse_status_branch"
    Assert-That "S3-status-b-node" ((Get-Str $bBrowseStatus "active_node_id") -eq $script:BranchNodeId) ("status B active_node_id=" + (Get-Str $bBrowseStatus "active_node_id") + " expected=" + $script:BranchNodeId) "browse_status_node"
    Assert-That "S3-status-b-head" ((Get-Str $bBrowseStatus "head") -eq $script:SeedCommit) ("status B head=" + (Get-Str $bBrowseStatus "head")) "browse_status_head"
    Assert-That "S3-status-b-branch-head" ((Get-BranchHead $bBrowseStatus $script:BranchName) -eq $script:SeedCommit) ("branches." + $script:BranchName + "=" + (Get-BranchHead $bBrowseStatus $script:BranchName)) "browse_status_branch_head"

    Assert-That "S3-b-file-unchanged" ((Get-Sha256 $script:WorktreeB) -eq $bHashBefore) "B project file changed during read-only status" "browse_wrote_b"
    Assert-That "S3-a-file-unchanged" ((Get-Sha256 $script:ProjectA) -eq $aHashBefore) "A project file changed during read-only status" "browse_wrote_a"
    $null = Assert-KernelProject -Id "S3-kernel-after" -ExpectedPath $script:ProjectA -Context "S3 after status"
    $s3UiAfter = Get-UIState -Name "s3_ui_after"
    $s3UiProjectAfter = Get-Prop $s3UiAfter "project"
    Assert-That "S3-ui-after" (Test-SamePath (Get-Str $s3UiProjectAfter "project_path") $script:ProjectA) ("ui project_path=" + (Get-Str $s3UiProjectAfter "project_path")) "ui_project_after"
    $read = Assert-NoOpenProjectSettled -Id "S3-no-open-project" -Path $script:KernelLogPath -Offset $offset -Context "version.status B read-only browse"
    $script:Summary.reload.browse_open_project = Count-OpenProject $read.Text
    $script:Summary.steps.s3 = $true
    Write-Ok "read-only browse did not switch kernel/DAW"

    # ------------------------------------------------------------------
    # S4 version.conversation_open: exactly one open_project reload
    # ------------------------------------------------------------------
    $script:CurrentStep = "S4"
    Write-Step "S4 version.conversation_open: open B and continue at target branch/node"
    $offset = Get-LogLength $script:KernelLogPath
    $aHashBeforeOpen = Get-Sha256 $script:ProjectA
    $s4StateBefore = Assert-KernelProject -Id "S4-kernel-before" -ExpectedPath $script:ProjectA -Context "S4 before conversation_open"

    $convResp = Invoke-Tool -Name "s4_conversation_open" -Tool "version.conversation_open" -ToolArgs ([ordered]@{
        project_file_path = $script:WorktreeB
        branch = $script:BranchName
        node_id = $script:BranchNodeId
    }) -Confirmed $true
    $conv = Get-Prop $convResp.Json "result"
    Assert-That "S4-result" ($null -ne $conv) "conversation_open result is null" "conversation_open_result"
    Assert-That "S4-status" ([string]$conv.status -eq "ok") ("conversation_open result status=" + (Get-Str $conv "status")) "conversation_open_status"
    Assert-That "S4-project-file-path" (Test-SamePath (Get-Str $conv "project_file_path") $script:WorktreeB) ("result project_file_path=" + (Get-Str $conv "project_file_path")) "conversation_open_target_path"
    $convWorktree = Get-Prop $conv "worktree"
    Assert-That "S4-worktree-name" ((Get-Str $convWorktree "name") -eq $script:WorktreeName) ("worktree.name=" + (Get-Str $convWorktree "name")) "conversation_open_worktree"
    Assert-That "S4-worktree-not-root" (-not (Get-Bool $convWorktree "is_root" $true)) "target worktree reported is_root=true" "conversation_open_worktree_root"
    Assert-That "S4-same-worktree-false" (-not (Get-Bool $conv "same_worktree" $true)) "same_worktree must be false for a cross-worktree open" "conversation_open_same_worktree"
    Assert-That "S4-target-materialized" (Get-Bool $conv "target_materialized" $false) "target_materialized=false after successful materialization" "conversation_open_materialized"
    Assert-That "S4-kernel-reloaded" (Get-Bool $conv "kernel_reloaded" $false) "kernel_reloaded=false" "conversation_open_reloaded"
    $refresh = Get-Prop $conv "refresh"
    Assert-That "S4-refresh-reloaded" (Get-Bool $refresh "kernel_reloaded" $false) "result.refresh.kernel_reloaded=false" "conversation_open_refresh"
    Assert-That "S4-refresh-reason" ((Get-Str $refresh "reason") -eq "version_conversation_open") ("refresh.reason=" + (Get-Str $refresh "reason")) "conversation_open_refresh_reason"
    Assert-That "S4-refresh-shadow" (Get-Bool $refresh "shadow_refreshed" $false) "result.refresh.shadow_refreshed=false" "conversation_open_shadow"
    Assert-That "S4-mode" ((Get-Str $conv "mode") -eq "branch_checkout") ("mode=" + (Get-Str $conv "mode")) "conversation_open_mode"
    Assert-That "S4-branch" ((Get-Str $conv "branch") -eq $script:BranchName) ("branch=" + (Get-Str $conv "branch")) "conversation_open_branch"
    Assert-That "S4-detached" (-not (Get-Bool $conv "detached" $true)) "detached=true; target branch was not activated" "conversation_open_detached"
    Assert-That "S4-head" ((Get-Str $conv "head") -eq $script:SeedCommit) ("head=" + (Get-Str $conv "head")) "conversation_open_head"
    Assert-That "S4-target-commit" ((Get-Str $conv "target_commit_id") -eq $script:SeedCommit) ("target_commit_id=" + (Get-Str $conv "target_commit_id")) "conversation_open_target_commit"
    Assert-That "S4-active-branch" ((Get-Str $conv "active_branch") -eq $script:BranchName) ("active_branch=" + (Get-Str $conv "active_branch")) "conversation_open_active_branch"
    Assert-That "S4-active-node" ((Get-Str $conv "active_node_id") -eq $script:BranchNodeId) ("active_node_id=" + (Get-Str $conv "active_node_id") + " expected=" + $script:BranchNodeId) "conversation_open_active_node"
    Assert-That "S4-active-worktree" ((Get-Str $conv "active_worktree") -eq $script:WorktreeName) ("active_worktree=" + (Get-Str $conv "active_worktree")) "conversation_open_active_worktree"
    Assert-That "S4-branch-warning" ([string]::IsNullOrWhiteSpace((Get-Str $conv "branch_warning"))) ("unexpected branch_warning=" + (Get-Str $conv "branch_warning")) "conversation_open_branch_warning"

    $autosaveId = Get-Str $conv "switch_autosave_commit_id"
    $autosaveWarning = Get-Str $conv "switch_autosave_warning"
    Assert-That "S4-switch-autosave" ((-not [string]::IsNullOrWhiteSpace($autosaveId)) -or (-not [string]::IsNullOrWhiteSpace($autosaveWarning))) "conversation_open disclosed neither switch_autosave_commit_id nor switch_autosave_warning" "conversation_open_autosave"
    if (-not [string]::IsNullOrWhiteSpace($autosaveWarning)) { Add-Warning ("switch autosave warning: " + $autosaveWarning) }

    $read = Wait-OpenProjectCount -Path $script:KernelLogPath -Offset $offset -Expected 1 -Seconds 20
    $script:Summary.reload.conversation_open_open_project = $read.Count
    $script:Summary.reload.target_path_seen = (Test-TextHasPath $read.Text $script:WorktreeB)
    $script:Summary.kernel_log.single_reload_count = $read.Count
    Assert-That "S4-single-open-project" ($read.Count -eq 1) ("conversation_open open_project count=" + $read.Count + " (expected exactly 1)") "conversation_open_reload_count"
    Assert-That "S4-open-project-target" $script:Summary.reload.target_path_seen "single open_project line does not contain target B path" "conversation_open_reload_target"

    $stateAfter = Wait-ProjectPath -Expected $script:WorktreeB -Seconds 30
    Assert-That "S4-kernel-project-b" ($null -ne $stateAfter) "kernel project path did not become B within 30s" "kernel_project_not_b"
    $uiAfter = Wait-UIProjectPath -Expected $script:WorktreeB -Seconds 30
    Assert-That "S4-ui-project-b" ($null -ne $uiAfter) "ui project_path did not become B within 30s" "ui_project_not_b"
    Start-Sleep -Milliseconds 1000

    $stateAfter = Get-AgentState -Name "s4_state_after"
    $statePath = Get-ActiveProjectPath $stateAfter
    Assert-That "S4-state-path-b" (Test-SamePath $statePath $script:WorktreeB) ("state current_project.path=" + $statePath) "state_project_path"
    $uiAfter = Get-UIState -Name "s4_ui_after"
    $uiProject = Get-Prop $uiAfter "project"
    $uiPath = Get-Str $uiProject "project_path"
    Assert-That "S4-ui-path-b" (Test-SamePath $uiPath $script:WorktreeB) ("ui project.project_path=" + $uiPath) "ui_project_path"
    $shadow = Get-Prop $stateAfter "shadow"
    $uiTracks = Get-Prop $uiProject "track_count"
    $shadowTracks = Get-Prop $shadow "track_count"
    Assert-That "S4-daw-track-count" ((Test-HasProp $uiProject "track_count") -and (Test-HasProp $shadow "track_count") -and ([string]$uiTracks -eq [string]$shadowTracks)) ("ui track_count=" + [string]$uiTracks + " shadow track_count=" + [string]$shadowTracks) "daw_track_count"
    $uiRev = Get-Prop $uiProject "graph_revision"
    $shadowRev = Get-Prop $shadow "graph_revision"
    Assert-That "S4-daw-graph-revision" ([string]$uiRev -eq [string]$shadowRev) ("ui graph_revision=" + [string]$uiRev + " shadow graph_revision=" + [string]$shadowRev) "daw_graph_revision"
    Assert-That "S4-daw-initialized" (Get-Bool $uiProject "initialized" $false) "ui project.initialized=false" "daw_initialized"

    $bStatusAfter = Get-HistoryStatus -Name "s4_status_b_after" -ProjectPath $script:WorktreeB
    Assert-That "S4-history-path" (Test-SamePath (Get-Str $bStatusAfter "project_path") $script:WorktreeB) ("post-open status B project_path=" + (Get-Str $bStatusAfter "project_path")) "post_open_status_path"
    Assert-That "S4-history-branch" ((Get-Str $bStatusAfter "active_branch") -eq $script:BranchName) ("post-open active_branch=" + (Get-Str $bStatusAfter "active_branch")) "post_open_branch"
    Assert-That "S4-history-node" ((Get-Str $bStatusAfter "active_node_id") -eq $script:BranchNodeId) ("post-open active_node_id=" + (Get-Str $bStatusAfter "active_node_id")) "post_open_node"
    Assert-That "S4-history-head" ((Get-Str $bStatusAfter "head") -eq $script:SeedCommit) ("post-open head=" + (Get-Str $bStatusAfter "head")) "post_open_head"
    Assert-That "S4-history-worktree" ((Get-Str $bStatusAfter "active_worktree") -eq $script:WorktreeName) ("post-open active_worktree=" + (Get-Str $bStatusAfter "active_worktree")) "post_open_worktree"
    Assert-That "S4-a-file-unchanged" ((Get-Sha256 $script:ProjectA) -eq $aHashBeforeOpen) "A project file changed during conversation_open" "conversation_open_wrote_a"
    $script:Summary.steps.s4 = $true
    Write-Ok ("conversation_open PASS: B active branch=" + $script:BranchName + " node=" + $script:BranchNodeId + " open_project=" + $read.Count)

    # ------------------------------------------------------------------
    # S5 failure semantics
    # ------------------------------------------------------------------
    $script:CurrentStep = "S5"
    Write-Step "S5 failure semantics: unknown/expired worktree, unknown node/branch"

    $missingName = "cross-missing-" + [guid]::NewGuid().ToString("N").Substring(0, 8)
    $null = Assert-ConversationOpenFailure -Name "s5_unknown_worktree" -ToolArgs ([ordered]@{ name = $missingName; node_id = $script:BranchNodeId }) -ExpectedText "worktree not found" -CurrentExpectedPath $script:WorktreeB

    $offset = Get-LogLength $script:KernelLogPath
    $expiredCreate = Invoke-Tool -Name "s5_worktree_create_expired" -Tool "version.worktree_create" -ToolArgs ([ordered]@{ name = "cross-expired" }) -Confirmed $true
    $expiredResult = Get-Prop $expiredCreate.Json "result"
    Assert-That "S5-expired-create" ($null -ne $expiredResult -and [string]$expiredResult.status -eq "ok") "version.worktree_create for expired fixture failed" "expired_worktree_create"
    $worktreeC = Get-Str $expiredResult "project_file_path"
    Assert-That "S5-expired-path" (-not [string]::IsNullOrWhiteSpace($worktreeC)) "expired fixture worktree has no project_file_path" "expired_worktree_path"
    $script:WorktreeC = Get-FullPath $worktreeC
    Assert-That "S5-expired-under-runroot" (Test-PathUnder $script:WorktreeC $script:RunRoot) ("expired fixture outside RunRoot: " + $script:WorktreeC) "expired_worktree_outside_runroot"
    $read = Assert-NoOpenProjectSettled -Id "S5-expired-create-no-reload" -Path $script:KernelLogPath -Offset $offset -Context "expired fixture worktree_create"
    $expiredStatus = Get-HistoryStatus -Name "s5_status_expired" -ProjectPath $script:WorktreeC
    $expiredNode = Get-Str $expiredStatus "active_node_id"
    if ([string]::IsNullOrWhiteSpace($expiredNode)) { $expiredNode = $script:BranchNodeId }
    Remove-Item -LiteralPath $script:WorktreeC -Force
    Assert-That "S5-expired-file-removed" (-not (Test-Path -LiteralPath $script:WorktreeC -PathType Leaf)) ("failed to remove expired fixture project file: " + $script:WorktreeC) "expired_fixture_remove"
    $null = Assert-ConversationOpenFailure -Name "s5_expired_worktree" -ToolArgs ([ordered]@{ project_file_path = $script:WorktreeC; node_id = $expiredNode }) -ExpectedText "worktree project file does not exist" -CurrentExpectedPath $script:WorktreeB

    $missingNode = "n_missing_" + [guid]::NewGuid().ToString("N").Substring(0, 8)
    $null = Assert-ConversationOpenFailure -Name "s5_unknown_node" -ToolArgs ([ordered]@{ project_file_path = $script:WorktreeB; node_id = $missingNode }) -ExpectedText "conversation node not found" -CurrentExpectedPath $script:WorktreeB

    $missingBranch = "missing-branch-" + [guid]::NewGuid().ToString("N").Substring(0, 8)
    $null = Assert-ConversationOpenFailure -Name "s5_unknown_branch" -ToolArgs ([ordered]@{ project_file_path = $script:WorktreeB; branch = $missingBranch; node_id = $script:BranchNodeId }) -ExpectedText "branch not found" -CurrentExpectedPath $script:WorktreeB

    $null = Assert-KernelProject -Id "S5-kernel-still-b" -ExpectedPath $script:WorktreeB -Context "S5 failures"
    $script:Summary.steps.s5 = $true
    Write-Ok "failure semantics: target_materialized=false and zero reload on all coverable failures"

    $script:Summary.coverage.real_stack_verified = @(
        "S1 project.new + project.save_as under fresh RunRoot",
        "S2 version.worktree_create + version.branch_create target branch/node; project.open back to A",
        "S3 version.status {project_path:B} read-only; kernel stays A; zero open_project; A/B file hashes unchanged",
        "S4 version.conversation_open cross-worktree branch checkout; target path/branch/node/HEAD active; exactly one open_project; DAW projection consistency",
        "S5 unknown/expired worktree + unknown node/branch: HTTP error, target_materialized=false, kernel_reloaded=false, zero reload, current project unchanged"
    )
    $script:Summary.coverage.unit_test_only = @(
        "reload failure after target materialization (419 TestConversationOpenReloadFailureIsExplicit uses a fake kernel; not injectable through the public real-stack surface)"
    )
    $script:Summary.coverage.not_exercised = @(
        "target_materialized=unknown (partial materialization error path)",
        "reload failure after materialization on a real kernel",
        "Godot/VitShell window pixels (script asserts agent-visible DAW projection, not GUI rendering)",
        "human /agent/chat conversation node (zero-LLM leg uses product-generated worktree seed + branch_marker nodes)"
    )
    $script:Summary.coverage.gaps = @(
        "Single open_project reload is verified at runtime from the kernel Command received log; if the kernel log is unavailable the script fails honest instead of claiming a count.",
        "A deterministic real-stack reload failure cannot be injected through the existing agent HTTP/tool surface without racing the filesystem or killing the kernel; 419 unit test is the only coverage and is not claimed as real-stack.",
        "The target conversation node is a product-generated branch_marker node, not a model-authored chat turn, because this leg is zero-LLM."
    )

    $exitCode = 0
}
catch {
    $ex = $_.Exception
    $class = "assertion_failure"
    $code = "unhandled"
    if ($null -ne $ex.Data -and $ex.Data.Contains("vit_class")) {
        $class = [string]$ex.Data["vit_class"]
        $code = [string]$ex.Data["vit_code"]
    }
    if (($class -eq "environment_failure") -or ($class -eq "usage") -or ($class -eq "contract_failure")) {
        $exitCode = 2
    } else {
        $exitCode = 1
    }
    Write-Bad ($class + "/" + $code + ": " + $ex.Message)
    if ($null -ne $script:Summary) {
        $script:Summary.status = "failed"
        $script:Summary.failure = [ordered]@{
            class = $class
            code = $code
            message = $ex.Message
            step = $script:CurrentStep
        }
    }
}
finally {
    if ($null -ne $script:Summary) {
        $teardownOk = $true
        $script:CurrentStep = "S6"
        $recentsResult = $null
        $authorityResult = $null
        try {
            $recentsResult = Restore-RecentsSnapshot
            $authorityResult = Restore-AuthoritySnapshots
        } catch {
            $teardownOk = $false
            Write-Bad ("teardown restore failed: " + $_.Exception.Message)
        }
        $recentsOk = ($null -ne $recentsResult -and $recentsResult.restored)
        $authorityOk = ($null -ne $authorityResult -and $authorityResult.ok)
        $apparatusOk = (($script:StartedProcesses -eq 0) -and ($script:StartedPorts -eq 0))
        $pathsOk = $true
        foreach ($path in @($script:ProjectA, $script:WorktreeB, $script:WorktreeC)) {
            if (-not [string]::IsNullOrWhiteSpace($path) -and -not (Test-PathUnder $path $script:RunRoot)) { $pathsOk = $false }
        }
        if (-not $recentsOk) { $teardownOk = $false }
        if (-not $authorityOk) { $teardownOk = $false }
        if (-not $apparatusOk) { $teardownOk = $false }

        $null = Add-AssertionSafe -Id "S6-recents-restored" -Condition $recentsOk -Detail ("recents before=" + $script:RecentsBeforeHash + " after=" + (Get-Sha256 $script:RecentsPath))
        $null = Add-AssertionSafe -Id "S6-authority-restored" -Condition $authorityOk -Detail ("authority restore error=" + $(if ($null -ne $authorityResult) { $authorityResult.error } else { "restore did not run" }))
        $null = Add-AssertionSafe -Id "S6-apparatus-zero" -Condition $apparatusOk -Detail ("script processes=" + $script:StartedProcesses + " ports=" + $script:StartedPorts)
        $null = Add-AssertionSafe -Id "S6-runroot-only" -Condition $pathsOk -Detail ("A/B/C must stay under " + $script:RunRoot)
        $null = Add-AssertionSafe -Id "S6-teardown-ok" -Condition $teardownOk -Detail "recents/authority restore and apparatus zero"

        if (-not $teardownOk) {
            $exitCode = 2
            if ($null -eq $script:Summary.failure) {
                $script:Summary.failure = [ordered]@{
                    class = "environment_failure"
                    code = "teardown_restore_failed"
                    message = "recents/authority restore or apparatus-zero check failed"
                    step = "S6"
                }
            } else {
                $script:Summary.failure["teardown_error"] = "recents/authority restore or apparatus-zero check failed"
            }
        }
        $script:Summary.steps.s6 = $teardownOk
        $script:Summary.status = $(if ($exitCode -eq 0) { "passed" } else { "failed" })
        $script:Summary.completed_at = (Get-Date).ToString("o")
        $script:Summary.assertions = @($script:Assertions)
        $script:Summary.warnings = @($script:Warnings)
        $script:Summary.assertion_count = @($script:Assertions).Count
        $script:Summary.assertions_passed = @($script:Assertions | Where-Object { $_.ok }).Count
        $script:Summary.kernel_log.single_reload_count = $script:Summary.reload.conversation_open_open_project
        $script:Summary.kernel_log.verified = ($script:Summary.reload.conversation_open_open_project -eq 1)
        $script:Summary.reload.verified = ($script:Summary.reload.conversation_open_open_project -eq 1)
        $script:Summary.teardown = [ordered]@{
            apparatus_processes_started = $script:StartedProcesses
            apparatus_ports_bound = $script:StartedPorts
            stack_processes_started = 0
            stack_processes_stopped = 0
            stack_owner = $RuntimeOwner
            kernel_left_running = $true
            zero_scope = "apparatus-only; the real stack is owned and stopped by RuntimeOwner"
            run_root_only = $pathsOk
            recents_restored = $recentsOk
            recents_hash_before = $script:RecentsBeforeHash
            recents_hash_after_restore = (Get-Sha256 $script:RecentsPath)
            authority_restored = $authorityOk
            authority_error = $(if ($null -ne $authorityResult) { $authorityResult.error } else { "" })
        }
        $script:SummaryPath = Join-Path $script:RunRoot "summary.json"
        try {
            Write-JsonFile -Path $script:SummaryPath -Object $script:Summary
            Write-JsonFile -Path (Join-Path $script:RunRoot "teardown.json") -Object $script:Summary.teardown
        } catch {
            Write-Bad ("failed to write summary artifact: " + $_.Exception.Message)
            $exitCode = 2
        }
        if ($exitCode -eq 0) {
            Write-Ok ("PASS: " + @($script:Assertions).Count + " assertions; single open_project=" + $script:Summary.kernel_log.single_reload_count + "; recents restored=" + $recentsOk)
        } else {
            Write-Bad ("FAILED: exit=" + $exitCode + "; see " + $script:SummaryPath)
        }
        Write-Host ("artifact: " + $script:SummaryPath)
    }
}

exit $exitCode
