<#
TUTOR-SIM-1（池序 435）真栈烟测腿 —— RUNBOOK + 待授权执行器。

目标（全部满足 -> exit 0）：
  S1 已打开工程旁存在 .vit_agent/tutor_sim.yaml，agent 当前工程路径与 -ProjectPath 一致；
  S2 POST /agent/tutor/start 成功：status=ok、第 1/3 步、LLM 开场非空；
  S3 错误输入 "0" 不推进：workflow=tutor_sim、passed=false、step_index=1；
  S4 输入 7 -> 第 2 步；输入 42 -> 第 3 步；输入 123 -> completed；
  S5 GET /agent/tutor/status 与上述步骤一致，完成后 completed=true；
  S6 完成后继续输入不再推进、不再需要 LLM，stop_reason=tutor_sim_already_completed；
  S7 所有用户可见 reply 不含命令词（开始乐局 / 写好了 / 乐局状态 / 乐局事件），
     chat 回执不含 goal_id（未落普通 agent loop）。

退出码：0=PASS；1=断言/实验装置失败；2=环境/上游 LLM 失败；3=未授权（默认）。
本脚本只走 agent HTTP，不启动、不重启、不停止任何栈进程。

默认（无 -StackAuthorized）只打印 runbook 并 exit 3。
-DryRun 只打印执行计划，不发任何 HTTP，exit 0。
授权后示例（单行）：
  powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\run_tutor_sim_smoke.ps1 -StackAuthorized -RuntimeOwner "<主管登记 owner>" -ProjectPath "<工程.vit>"
#>
[CmdletBinding()]
param(
    [string]$RepoRoot = "",
    [string]$AgentBaseUrl = "http://127.0.0.1:7878",
    [string]$ProjectPath = "",
    [string]$ConversationID = "",
    [string]$RuntimeOwner = "",
    [string]$RunRoot = "",
    [int]$TimeoutSeconds = 120,
    [switch]$StackAuthorized,
    [switch]$DryRun
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Write-Step { param([string]$Message) Write-Host ""; Write-Host ("== " + $Message) -ForegroundColor Cyan }
function Write-Ok { param([string]$Message) Write-Host ("ok: " + $Message) -ForegroundColor Green }
function Write-WarnLine { param([string]$Message) Write-Host ("warn: " + $Message) -ForegroundColor Yellow }
function Fail { param([string]$Message) throw $Message }

function Get-JsonMember {
    param($Object, [string]$Name)
    if ($null -eq $Object) { return $null }
    $property = $Object.PSObject.Properties[$Name]
    if ($null -eq $property) { return $null }
    return $property.Value
}

function Get-NormalizedPath {
    param([string]$Path)
    if ([string]::IsNullOrWhiteSpace($Path)) { return "" }
    $full = [System.IO.Path]::GetFullPath($Path)
    return $full.TrimEnd([System.IO.Path]::DirectorySeparatorChar, [System.IO.Path]::AltDirectorySeparatorChar)
}

function Test-SamePath {
    param([string]$Left, [string]$Right)
    return [string]::Equals((Get-NormalizedPath $Left), (Get-NormalizedPath $Right), [System.StringComparison]::OrdinalIgnoreCase)
}

function Get-ActiveProjectPath {
    param($State)
    $current = Get-JsonMember $State "current_project"
    $path = Get-JsonMember $current "path"
    if (-not [string]::IsNullOrWhiteSpace([string]$path)) { return [string]$path }
    $history = Get-JsonMember $State "project_history"
    $path = Get-JsonMember $history "project_path"
    if (-not [string]::IsNullOrWhiteSpace([string]$path)) { return [string]$path }
    $shadow = Get-JsonMember $State "shadow"
    $path = Get-JsonMember $shadow "project_path"
    if (-not [string]::IsNullOrWhiteSpace([string]$path)) { return [string]$path }
    return ""
}

function Get-SidecarPath {
    param([string]$Project)
    if ([string]::IsNullOrWhiteSpace($Project)) { return "" }
    $leaf = Get-Item -LiteralPath $Project -ErrorAction SilentlyContinue
    if ($null -ne $leaf -and $leaf.PSIsContainer) {
        $dir = $leaf.FullName
    } else {
        $dir = Split-Path -Parent $Project
    }
    return (Join-Path (Join-Path $dir ".vit_agent") "tutor_sim.yaml")
}

function Find-CommandWords {
    param([string]$Text)
    $found = @()
    foreach ($word in @("开始乐局", "写好了", "乐局状态", "乐局事件")) {
        if (-not [string]::IsNullOrWhiteSpace($Text) -and $Text.Contains($word)) { $found += $word }
    }
    return $found
}

function Assert-True {
    param([bool]$Condition, [string]$Message)
    if (-not $Condition) { throw $Message }
}

if ([string]::IsNullOrWhiteSpace($RepoRoot)) {
    $RepoRoot = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
} else {
    $RepoRoot = (Resolve-Path -LiteralPath $RepoRoot).Path
}

if (-not $StackAuthorized -and -not $DryRun) {
    Write-Host "TUTOR-SIM-1（池序 435）真栈烟测腿：RUNBOOK 已就位，未授权不运行。" -ForegroundColor Yellow
    Write-Host "  前置：主管登记 coord/resources/PC-RUNTIME-STACK.md 栈位并书面授权；" -ForegroundColor Yellow
    Write-Host "        工程已由产品入口打开，且 <工程目录>/.vit_agent/tutor_sim.yaml 已放置；" -ForegroundColor Yellow
    Write-Host "        实验文件模板见 coord/experiments/tutor-sim-1/tutor_sim.yaml 与 README.md。" -ForegroundColor Yellow
    Write-Host "  授权后追加：-StackAuthorized -RuntimeOwner '<owner>' -ProjectPath '<工程.vit>'。" -ForegroundColor Yellow
    Write-Host "  本脚本不启动/重启/停止栈进程（只走 agent HTTP）。" -ForegroundColor Yellow
    exit 3
}

$resolvedProject = ""
$sidecarPath = ""
if (-not [string]::IsNullOrWhiteSpace($ProjectPath)) {
    if (Test-Path -LiteralPath $ProjectPath) {
        $resolvedProject = (Resolve-Path -LiteralPath $ProjectPath).Path
    } else {
        $resolvedProject = [System.IO.Path]::GetFullPath($ProjectPath)
    }
    $sidecarPath = Get-SidecarPath $resolvedProject
}

if ($DryRun) {
    $plan = [ordered]@{
        card = "TUTOR-SIM-1"
        mode = "dry_run"
        repo_root = $RepoRoot
        agent_base_url = $AgentBaseUrl
        project_path = $resolvedProject
        sidecar_path = $sidecarPath
        sidecar_exists = ($sidecarPath -ne "" -and (Test-Path -LiteralPath $sidecarPath))
        runtime_owner = $RuntimeOwner
        planned_steps = @(
            "GET /health 与 /agent/state（真栈时）",
            "POST /agent/tutor/start {conversation_id, project_path}",
            "POST /agent/chat 错输入 0 -> 断言不推进",
            "POST /agent/chat 7 -> 第 2 步",
            "POST /agent/chat 42 -> 第 3 步",
            "POST /agent/chat 123 -> completed",
            "GET /agent/tutor/status 断言 completed",
            "完成后追加输入 -> 断言 already_completed"
        )
    }
    $plan | ConvertTo-Json -Depth 16
    exit 0
}

if ([string]::IsNullOrWhiteSpace($RuntimeOwner)) {
    Fail "真栈执行必须在 -RuntimeOwner 记录主管登记的栈位 owner（防未登记占用）"
}
if ([string]::IsNullOrWhiteSpace($resolvedProject)) {
    Fail "真栈执行必须用 -ProjectPath 指定已打开的实验工程"
}
if (-not (Test-Path -LiteralPath $resolvedProject)) {
    Fail "工程不存在: $resolvedProject"
}
if ([string]::IsNullOrWhiteSpace($sidecarPath) -or -not (Test-Path -LiteralPath $sidecarPath)) {
    Fail ("工程旁缺少实验 sidecar: " + $sidecarPath + "（模板见 coord/experiments/tutor-sim-1/）")
}

$stamp = Get-Date -Format "yyyyMMdd_HHmmss"
if ([string]::IsNullOrWhiteSpace($RunRoot)) {
    $RunRoot = Join-Path (Join-Path (Join-Path (Join-Path $RepoRoot "VitApp") "Workspace") "Artifacts") ("smoke/tutor_sim_" + $stamp)
}
$RunRoot = [System.IO.Path]::GetFullPath($RunRoot)
New-Item -ItemType Directory -Path $RunRoot -Force | Out-Null
New-Item -ItemType Directory -Path (Join-Path $RunRoot "requests") -Force | Out-Null
New-Item -ItemType Directory -Path (Join-Path $RunRoot "responses") -Force | Out-Null

if ([string]::IsNullOrWhiteSpace($ConversationID)) {
    $ConversationID = "tutor-smoke-" + [guid]::NewGuid().ToString("N").Substring(0, 10)
}

$summary = [ordered]@{
    schema_version = "vit_tutor_sim_smoke.v1"
    card = "TUTOR-SIM-1"
    status = "running"
    runtime_owner = $RuntimeOwner
    agent_base_url = $AgentBaseUrl
    project_path = $resolvedProject
    sidecar_path = $sidecarPath
    conversation_id = $ConversationID
    agent_state_path = ""
    start = $null
    turns = @()
    final_status = $null
    command_words_found = @()
    answer_leak_warnings = @()
    created_at = (Get-Date).ToString("o")
    finished_at = ""
    error = ""
}
$turnRecords = @()
$commandWordsFound = @()
$answerLeakWarnings = @()

function Save-Summary {
    param([string]$Status, [string]$ErrorText)
    $summary.status = $Status
    $summary.error = $ErrorText
    $summary.turns = @($turnRecords)
    $summary.command_words_found = @($commandWordsFound)
    $summary.answer_leak_warnings = @($answerLeakWarnings)
    $summary.finished_at = (Get-Date).ToString("o")
    $summary | ConvertTo-Json -Depth 32 | Set-Content -LiteralPath (Join-Path $RunRoot "summary.json") -Encoding UTF8
}

function Stop-Environment {
    param([string]$Message)
    Save-Summary "environment_failure" $Message
    Write-Host ("ENV: " + $Message) -ForegroundColor Red
    exit 2
}

function Stop-Upstream {
    param([string]$Message)
    Save-Summary "upstream_failure" $Message
    Write-Host ("UPSTREAM: " + $Message) -ForegroundColor Red
    exit 2
}

function Invoke-AgentJson {
    param([string]$Name, [string]$Method, [string]$Path, $Body)
    $uri = ($AgentBaseUrl.TrimEnd("/") + $Path)
    $requestFile = Join-Path (Join-Path $RunRoot "requests") ($Name + ".json")
    $responseFile = Join-Path (Join-Path $RunRoot "responses") ($Name + ".json")
    try {
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
    } catch {
        $statusCode = 0
        $bodyText = ""
        if ($null -ne $_.Exception.Response) {
            $statusCode = [int]$_.Exception.Response.StatusCode
            try {
                $stream = $_.Exception.Response.GetResponseStream()
                if ($null -ne $stream) {
                    $reader = New-Object System.IO.StreamReader($stream)
                    $bodyText = $reader.ReadToEnd()
                    $reader.Close()
                }
            } catch { }
        }
        if ([string]::IsNullOrWhiteSpace($bodyText) -and $null -ne $_.ErrorDetails) {
            $bodyText = [string]$_.ErrorDetails.Message
        }
        $failure = [ordered]@{ http_status = $statusCode; message = $_.Exception.Message; body = $bodyText }
        $failure | ConvertTo-Json -Depth 16 | Set-Content -LiteralPath $responseFile -Encoding UTF8
        return [pscustomobject]@{ __smoke_error = $true; http_status = $statusCode; message = $_.Exception.Message; body = $bodyText }
    }
}

function Test-SmokeError {
    param($Response)
    return [bool](Get-JsonMember $Response "__smoke_error")
}

function Get-FailureClass {
    param($Response)
    $bodyText = [string](Get-JsonMember $Response "body")
    if ([string]::IsNullOrWhiteSpace($bodyText)) { return "" }
    try {
        $parsed = $bodyText | ConvertFrom-Json
        return [string](Get-JsonMember $parsed "failure_class")
    } catch {
        return ""
    }
}

function Record-Reply {
    param([string]$Name, [string]$Reply)
    $found = @(Find-CommandWords $Reply)
    if ($found.Count -gt 0) {
        $script:commandWordsFound = @($script:commandWordsFound) + $found
        throw ("reply " + $Name + " 含命令词: " + ($found -join ","))
    }
}

function Get-TutorStatus {
    param([string]$Name)
    $response = Invoke-AgentJson -Name $Name -Method "Get" -Path ("/agent/tutor/status?conversation_id=" + [System.Uri]::EscapeDataString($ConversationID)) -Body $null
    if (Test-SmokeError $response) {
        Stop-Environment ("status 请求失败: " + [string](Get-JsonMember $response "message"))
    }
    return $response
}

function Send-TutorChat {
    param([string]$Name, [string]$Message)
    $response = Invoke-AgentJson -Name $Name -Method "Post" -Path "/agent/chat" -Body ([ordered]@{
        conversation_id = $ConversationID
        message = $Message
        context = @{}
    })
    if (Test-SmokeError $response) {
        $class = Get-FailureClass $response
        if ($class -ne "") { Stop-Upstream ("chat " + $Name + " HTTP " + [string](Get-JsonMember $response "http_status") + " failure_class=" + $class) }
        throw ("chat " + $Name + " 请求失败: " + [string](Get-JsonMember $response "message"))
    }
    $failureClass = [string](Get-JsonMember (Get-JsonMember $response "workflow_data") "failure_class")
    if (-not [string]::IsNullOrWhiteSpace($failureClass) -or -not [string]::IsNullOrWhiteSpace([string](Get-JsonMember $response "error"))) {
        Stop-Upstream ("chat " + $Name + " LLM failure_class=" + $failureClass + " error=" + [string](Get-JsonMember $response "error"))
    }
    Assert-True ([string](Get-JsonMember $response "workflow") -eq "tutor_sim") ("chat " + $Name + " workflow=" + [string](Get-JsonMember $response "workflow"))
    Assert-True ([string]::IsNullOrWhiteSpace([string](Get-JsonMember $response "goal_id"))) ("chat " + $Name + " 落入普通 agent loop（goal_id 非空）")
    $reply = [string](Get-JsonMember $response "reply")
    Assert-True (-not [string]::IsNullOrWhiteSpace($reply)) ("chat " + $Name + " reply 为空")
    Record-Reply $Name $reply
    return $response
}

function Assert-Turn {
    param($Response, [string]$Name, [bool]$Passed, [bool]$Completed, [int]$StepIndex, [string]$StepID, [string]$StopReason)
    $data = Get-JsonMember $Response "workflow_data"
    Assert-True ($null -ne $data) ($Name + " 缺 workflow_data")
    Assert-True ([bool](Get-JsonMember $data "passed") -eq $Passed) ($Name + " passed=" + [string](Get-JsonMember $data "passed") + "，期望 " + $Passed)
    Assert-True ([bool](Get-JsonMember $data "completed") -eq $Completed) ($Name + " completed=" + [string](Get-JsonMember $data "completed") + "，期望 " + $Completed)
    Assert-True ([int](Get-JsonMember $data "step_index") -eq $StepIndex) ($Name + " step_index=" + [string](Get-JsonMember $data "step_index") + "，期望 " + $StepIndex)
    Assert-True ([int](Get-JsonMember $data "step_total") -eq 3) ($Name + " step_total=" + [string](Get-JsonMember $data "step_total") + "，期望 3")
    if (-not [string]::IsNullOrWhiteSpace($StepID)) {
        Assert-True ([string](Get-JsonMember $data "step_id") -eq $StepID) ($Name + " step_id=" + [string](Get-JsonMember $data "step_id") + "，期望 " + $StepID)
    }
    if (-not [string]::IsNullOrWhiteSpace($StopReason)) {
        Assert-True ([string](Get-JsonMember $Response "stop_reason") -eq $StopReason) ($Name + " stop_reason=" + [string](Get-JsonMember $Response "stop_reason") + "，期望 " + $StopReason)
    }
    $turnRecords = $script:turnRecords
    $script:turnRecords = @($turnRecords) + @([ordered]@{
        name = $Name
        step_index = [int](Get-JsonMember $data "step_index")
        step_total = [int](Get-JsonMember $data "step_total")
        step_id = [string](Get-JsonMember $data "step_id")
        passed = [bool](Get-JsonMember $data "passed")
        completed = [bool](Get-JsonMember $data "completed")
        stop_reason = [string](Get-JsonMember $Response "stop_reason")
        reply = [string](Get-JsonMember $Response "reply")
    })
}

try {
    Write-Step "S0 preflight（agent HTTP + 栈身份；本脚本不启停栈）"
    $health = Invoke-AgentJson -Name "health" -Method "Get" -Path "/health" -Body $null
    if (Test-SmokeError $health) {
        Stop-Environment ("agent /health 不可达: " + $AgentBaseUrl)
    }
    Assert-True ([string](Get-JsonMember $health "status") -eq "ok") "agent /health 未返回 status=ok"

    $state = Invoke-AgentJson -Name "state" -Method "Get" -Path "/agent/state" -Body $null
    if (Test-SmokeError $state) {
        Stop-Environment ("agent 面不可达（/agent/state）: " + [string](Get-JsonMember $state "message"))
    }
    $activePath = Get-ActiveProjectPath $state
    $summary.agent_state_path = $activePath
    Assert-True (-not [string]::IsNullOrWhiteSpace($activePath)) "agent 当前没有已打开工程；请先用产品入口打开 -ProjectPath 指向的工程"
    Assert-True (Test-SamePath $activePath $resolvedProject) ("agent 当前工程路径与 -ProjectPath 不一致: " + $activePath + " != " + $resolvedProject)
    Write-Ok "agent 可见且当前工程匹配"

    Write-Step "S1 start：无命令词、LLM 人设开场、第 1/3 步"
    $start = Invoke-AgentJson -Name "start" -Method "Post" -Path "/agent/tutor/start" -Body ([ordered]@{
        conversation_id = $ConversationID
        project_path = $resolvedProject
    })
    if (Test-SmokeError $start) {
        $httpStatus = [int](Get-JsonMember $start "http_status")
        $class = Get-FailureClass $start
        if ($httpStatus -ge 500 -or $class -ne "") {
            Stop-Upstream ("start HTTP " + $httpStatus + " failure_class=" + $class + " message=" + [string](Get-JsonMember $start "message"))
        }
        throw ("start 请求失败: HTTP " + $httpStatus + " " + [string](Get-JsonMember $start "message"))
    }
    Assert-True ([string](Get-JsonMember $start "status") -eq "ok") ("start status=" + [string](Get-JsonMember $start "status") + " reason=" + [string](Get-JsonMember $start "reason"))
    Assert-True ([int](Get-JsonMember $start "step_index") -eq 1) "start step_index 不是 1"
    Assert-True ([int](Get-JsonMember $start "step_total") -eq 3) "start step_total 不是 3"
    Assert-True (-not [string]::IsNullOrWhiteSpace([string](Get-JsonMember $start "step_id"))) "start step_id 为空"
    Assert-True ([string](Get-JsonMember $start "conversation_id") -eq $ConversationID) "start conversation_id 与请求不一致"
    Assert-True ([string](Get-JsonMember $start "schema_version") -eq "vit.tutor_sim.v0") "start schema_version 不是 vit.tutor_sim.v0"
    $startReply = [string](Get-JsonMember $start "reply")
    Assert-True (-not [string]::IsNullOrWhiteSpace($startReply)) "start reply 为空（LLM 未生成开场）"
    Record-Reply "start" $startReply
    $summary.start = [ordered]@{
        conversation_id = [string](Get-JsonMember $start "conversation_id")
        experiment_id = [string](Get-JsonMember $start "experiment_id")
        step_index = [int](Get-JsonMember $start "step_index")
        step_total = [int](Get-JsonMember $start "step_total")
        step_id = [string](Get-JsonMember $start "step_id")
        reply = $startReply
    }
    foreach ($expectValue in @("7", "42", "123")) {
        if ($startReply.Contains($expectValue)) {
            $script:answerLeakWarnings = @($script:answerLeakWarnings) + @("start reply 含 expect 值 " + $expectValue)
        }
    }
    Write-Ok "开场已生成"

    Write-Step "S2 错输入 0：判定失败、不推进"
    $wrong = Send-TutorChat -Name "wrong_0" -Message "0"
    Assert-Turn -Response $wrong -Name "wrong_0" -Passed $false -Completed $false -StepIndex 1 -StepID ([string](Get-JsonMember $start "step_id")) -StopReason "tutor_sim_retry"
    $wrongStatus = Get-TutorStatus "status_after_wrong"
    Assert-True ([int](Get-JsonMember $wrongStatus "step_index") -eq 1) "错误输入后 status step_index 不是 1"
    Assert-True (-not [bool](Get-JsonMember $wrongStatus "completed")) "错误输入后 status completed=true"
    Assert-True ([int](Get-JsonMember $wrongStatus "attempts") -eq 1) "错误输入后 attempts 不是 1"
    Assert-True ([int](Get-JsonMember $wrongStatus "failures") -eq 1) "错误输入后 failures 不是 1"
    Write-Ok "错误输入未推进"

    Write-Step "S3 输入 7：推进到第 2 步"
    $step2 = Send-TutorChat -Name "answer_7" -Message "7"
    Assert-Turn -Response $step2 -Name "answer_7" -Passed $true -Completed $false -StepIndex 2 -StepID "" -StopReason "tutor_sim_step_advanced"
    $status2 = Get-TutorStatus "status_after_7"
    Assert-True ([int](Get-JsonMember $status2 "step_index") -eq 2) "7 之后 status step_index 不是 2"
    Assert-True (-not [bool](Get-JsonMember $status2 "completed")) "7 之后 status completed=true"
    Write-Ok "第 2 步"

    Write-Step "S4 输入 42：推进到第 3 步"
    $step3 = Send-TutorChat -Name "answer_42" -Message "42"
    Assert-Turn -Response $step3 -Name "answer_42" -Passed $true -Completed $false -StepIndex 3 -StepID "" -StopReason "tutor_sim_step_advanced"
    $status3 = Get-TutorStatus "status_after_42"
    Assert-True ([int](Get-JsonMember $status3 "step_index") -eq 3) "42 之后 status step_index 不是 3"
    Assert-True (-not [bool](Get-JsonMember $status3 "completed")) "42 之后 status completed=true"
    Write-Ok "第 3 步"

    Write-Step "S5 输入 123：完成"
    $completed = Send-TutorChat -Name "answer_123" -Message "123"
    Assert-Turn -Response $completed -Name "answer_123" -Passed $true -Completed $true -StepIndex 3 -StepID "" -StopReason "tutor_sim_completed"
    Assert-True ([string](Get-JsonMember $completed "goal_status") -eq "completed") "完成回执 goal_status 不是 completed"
    $finalStatus = Get-TutorStatus "status_completed"
    Assert-True ([bool](Get-JsonMember $finalStatus "completed")) "完成后 status completed=false"
    Assert-True ([string](Get-JsonMember $finalStatus "session_status") -eq "completed") "完成后 session_status 不是 completed"
    Assert-True ([int](Get-JsonMember $finalStatus "step_index") -eq 3) "完成后 status step_index 不是 3"
    $summary.final_status = [ordered]@{
        step_index = [int](Get-JsonMember $finalStatus "step_index")
        step_total = [int](Get-JsonMember $finalStatus "step_total")
        step_id = [string](Get-JsonMember $finalStatus "step_id")
        session_status = [string](Get-JsonMember $finalStatus "session_status")
        completed = [bool](Get-JsonMember $finalStatus "completed")
        attempts = [int](Get-JsonMember $finalStatus "attempts")
        failures = [int](Get-JsonMember $finalStatus "failures")
    }
    Write-Ok "实验 completed"

    Write-Step "S6 完成后追加输入：不推进、不进入普通 loop"
    $after = Send-TutorChat -Name "after_completed" -Message "7"
    $afterData = Get-JsonMember $after "workflow_data"
    Assert-True ([string](Get-JsonMember $after "stop_reason") -eq "tutor_sim_already_completed") ("完成后 stop_reason=" + [string](Get-JsonMember $after "stop_reason"))
    Assert-True ([bool](Get-JsonMember $afterData "completed")) "完成后追加输入 completed=false"
    Assert-True (-not [bool](Get-JsonMember $afterData "passed")) "完成后追加输入 passed=true"
    $afterStatus = Get-TutorStatus "status_after_completed"
    Assert-True ([int](Get-JsonMember $afterStatus "step_index") -eq 3) "完成后追加输入改变了 step_index"
    Assert-True ([int](Get-JsonMember $afterStatus "attempts") -eq [int](Get-JsonMember $finalStatus "attempts")) "完成后追加输入改变了 attempts"
    Write-Ok "完成后已停止"

    Save-Summary "passed" ""
    Write-Host ""
    Write-Host ("PASS TUTOR-SIM-1；工件: " + $RunRoot) -ForegroundColor Green
    exit 0
} catch {
    $message = $_.Exception.Message
    Save-Summary "failed" $message
    Write-Host ""
    Write-Host ("FAIL: " + $message) -ForegroundColor Red
    Write-Host ("工件: " + $RunRoot) -ForegroundColor Yellow
    exit 1
}
